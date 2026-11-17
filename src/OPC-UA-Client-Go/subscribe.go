/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

// Subscriptions and the notification pump. Port of the subscription setup
// in ConsoleClient() and of OnNotification() in AsduReceiveHandler.cs.

package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
)

const (
	// monitorChunk is how many monitored items go into one
	// CreateMonitoredItems request. The C# stack chunks for the caller;
	// gopcua does not, and a large server rejects an oversized request
	// outright (deviation D8).
	monitorChunk = 1000

	// notifyBuffer sizes the channel between the client's publish
	// goroutine and the pump. The client sends to it synchronously, so it
	// has to absorb a burst rather than block acquisition.
	notifyBuffer = 4096

	// subscribeAttempts and subscribeRetryPause bound the retries of a
	// refused CreateSubscription.
	subscribeAttempts   = 3
	subscribeRetryPause = 500 * time.Millisecond

	// maxItemEventsPerSecond caps what one monitored item may deliver in a
	// second; the excess is dropped (deviation D24). A healthy item reports
	// a few values per publish at most, so this only ever trips for a node
	// that changes on every server request and would otherwise fill the
	// shared queue and starve every other tag.
	maxItemEventsPerSecond = 200

	// heartbeatHandle is the client handle of the heartbeat item, which no
	// real item can reach (handles count up from 1).
	heartbeatHandle uint32 = math.MaxUint32

	// minStaleAfter is the least silence tolerated before a session whose
	// subscriptions deliver nothing is rebuilt.
	minStaleAfter = 30 * time.Second
)

// sessionHealth tells the connection loop whether the subscriptions of the
// current session are still delivering.
//
// deviation D28: the session state alone cannot tell. gopcua pauses its
// publish loop on errors such as BadNoSubscription while the session stays
// Connected, and with its default keep-alive count (3000) a silent server is
// only noticed after hours. Each subscription therefore also monitors the
// server's CurrentTime, which changes every second, and any notification
// counts as proof of life; a session that goes quiet for longer than
// staleAfter is rebuilt.
type sessionHealth struct {
	lastAlive atomic.Int64 // unix nanoseconds of the last notification
	expected  atomic.Bool  // a heartbeat item exists, so silence means trouble
	maxIntvl  atomic.Int64 // longest publishing interval, nanoseconds
	failure   chan error   // a subscription reported an unrecoverable error
}

func newSessionHealth() *sessionHealth {
	h := &sessionHealth{failure: make(chan error, 1)}
	h.touch()
	return h
}

func (h *sessionHealth) touch() { h.lastAlive.Store(time.Now().UnixNano()) }

func (h *sessionHealth) fail(err error) {
	select {
	case h.failure <- err:
	default: // one is enough
	}
}

// staleFor returns how long the session has been silent, and whether that is
// long enough to give up on it.
func (h *sessionHealth) staleFor(now time.Time) (time.Duration, bool) {
	if !h.expected.Load() {
		return 0, false
	}
	silent := now.Sub(time.Unix(0, h.lastAlive.Load()))
	limit := max(minStaleAfter, 5*time.Duration(h.maxIntvl.Load()))
	return silent, silent > limit
}

// noteInterval records a publishing interval in use.
func (h *sessionHealth) noteInterval(d time.Duration) {
	for {
		cur := h.maxIntvl.Load()
		if int64(d) <= cur || h.maxIntvl.CompareAndSwap(cur, int64(d)) {
			return
		}
	}
}

// errSubscriptionSetup marks a session whose subscriptions could not all be
// created.
var errSubscriptionSetup = errors.New("subscription setup incomplete")

// allow reports whether the item may deliver another value now. first is
// true on the first value refused in a one-second window.
func (it *monItem) allow(now time.Time) (ok, first bool) {
	if now.Sub(it.windowStart) >= time.Second {
		it.windowStart = now
		it.windowCount = 0
	}
	it.windowCount++
	if it.windowCount <= maxItemEventsPerSecond {
		return true, false
	}
	return false, it.windowCount == maxItemEventsPerSecond+1
}

// setupSubscriptions creates the connection's subscriptions on a fresh
// session.
//
// With autoCreateTags on there is one subscription holding every monitored
// item, preconfigured and discovered alike, at the connection's
// autoCreateTagPublishingInterval. With it off there is one subscription
// per distinct publishing interval found in realtimeData.
//
// ctx is the session's context: the pumps end with it. A subscription that
// cannot be created is reported (deviation D28) so the caller can rebuild the
// session; the subscriptions before it stay in place.
func setupSubscriptions(ctx context.Context, cli *opcua.Client, conn *OPCUAConnection, health *sessionHealth) error {
	if conn.AutoCreateTags {
		return createSubscription(ctx, cli, conn, health, conn.AutoCreateTagPublishingInterval, conn.ListMon)
	}

	for _, interval := range conn.SubscriptionOrder {
		if err := createSubscription(ctx, cli, conn, health, interval, conn.OpcSubscriptions[interval]); err != nil {
			return err
		}
	}
	return nil
}

// createSubscription publishes one group of monitored items and starts the
// pump that drains its notifications into the acquired-value queue.
func createSubscription(ctx context.Context, cli *opcua.Client, conn *OPCUAConnection, health *sessionHealth, intervalSeconds float64, items []*monItem) error {
	if len(items) == 0 {
		return nil
	}

	jslog.Log(jslog.LevelNoLog, "%s - Create a subscription with publishing interval of %v seconds",
		conn.Name, intervalSeconds)

	notifyCh := make(chan *opcua.PublishNotificationData, notifyBuffer)

	// A subscription that fails to create takes every point of its
	// publishing-interval group with it until the session is rebuilt, so a
	// transient refusal is worth retrying rather than swallowing. The C#
	// driver logs and moves on; this only adds the retries.
	var sub *opcua.Subscription
	var err error
	for attempt := 1; attempt <= subscribeAttempts; attempt++ {
		sub, err = cli.Subscribe(ctx, &opcua.SubscriptionParameters{
			Interval: time.Duration(intervalSeconds * float64(time.Second)),
		}, notifyCh)
		if err == nil {
			break
		}
		jslog.Log(jslog.LevelBasic, "%s - Error creating subscription (attempt %d/%d): %v",
			conn.Name, attempt, subscribeAttempts, err)
		if attempt < subscribeAttempts && !sleepCtx(ctx, subscribeRetryPause) {
			return ctx.Err()
		}
	}
	if err != nil {
		// Loud on purpose: these points will not update at all until the
		// connection is rebuilt.
		jslog.Log(jslog.LevelNoLog,
			"%s - Error creating subscription: %v - %d monitored items with a %v s publishing interval WILL NOT UPDATE",
			conn.Name, err, len(items), intervalSeconds)
		return fmt.Errorf("%w: CreateSubscription: %v", errSubscriptionSetup, err)
	}
	health.noteInterval(time.Duration(intervalSeconds * float64(time.Second)))
	addHeartbeat(ctx, conn, sub, health)

	requests := make([]*ua.MonitoredItemCreateRequest, 0, len(items))
	for _, it := range items {
		nodeID, err := ua.ParseNodeID(it.NodeID)
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "%s - Error adding monitored item: bad node id %q - %v",
				conn.Name, it.NodeID, err)
			continue
		}

		req := opcua.NewMonitoredItemCreateRequestWithDefaults(nodeID, ua.AttributeIDValue, conn.NewHandle(it))
		req.MonitoringMode = ua.MonitoringModeReporting
		req.RequestedParameters.SamplingInterval = it.SamplingMs
		req.RequestedParameters.QueueSize = it.QueueSize
		// parity: the C# driver hardcodes DiscardOldest, ignoring the
		// protocolSourceDiscardOldest field of the tag (deviation D12).
		req.RequestedParameters.DiscardOldest = true
		req.RequestedParameters.Filter = ua.NewExtensionObject(&ua.DataChangeFilter{
			Trigger:       ua.DataChangeTriggerStatusValueTimestamp,
			DeadbandType:  uint32(ua.DeadbandTypeNone),
			DeadbandValue: 0,
		})
		requests = append(requests, req)
	}

	created := 0
	failed := false
	rejected := map[string][]string{}
	for start := 0; start < len(requests); start += monitorChunk {
		end := min(start+monitorChunk, len(requests))
		chunk := requests[start:end]

		resp, err := sub.Monitor(ctx, ua.TimestampsToReturnBoth, chunk...)
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "%s - Error creating monitored items: %v", conn.Name, err)
			failed = true
			continue
		}
		// A rejected item must not abort the subscription: the C# driver
		// logs and carries on, and one bad node id in a large namespace
		// would otherwise cost every other point.
		for i, res := range resp.Results {
			if !statusIsGood(res.StatusCode) {
				name := statusCodeName(res.StatusCode)
				rejected[name] = append(rejected[name], chunk[i].ItemToMonitor.NodeID.String())
				continue
			}
			created++
		}
	}
	logRejected(conn, rejected)

	jslog.Log(jslog.LevelNoLog, "%s - %d Monitored items", conn.Name, created)

	go notificationPump(ctx, conn, health, notifyCh)

	if failed {
		return fmt.Errorf("%w: some monitored items could not be requested", errSubscriptionSetup)
	}
	return nil
}

// addHeartbeat monitors the server's clock in a subscription so a silent
// session can be told from an idle one. A server that refuses it simply gets
// no staleness check.
func addHeartbeat(ctx context.Context, conn *OPCUAConnection, sub *opcua.Subscription, health *sessionHealth) {
	req := opcua.NewMonitoredItemCreateRequestWithDefaults(
		ua.NewNumericNodeID(0, id.Server_ServerStatus_CurrentTime), ua.AttributeIDValue, heartbeatHandle)
	req.RequestedParameters.SamplingInterval = 1000
	req.RequestedParameters.QueueSize = 1
	resp, err := sub.Monitor(ctx, ua.TimestampsToReturnNeither, req)
	if err != nil || len(resp.Results) != 1 || !statusIsGood(resp.Results[0].StatusCode) {
		jslog.Log(jslog.LevelDetailed, "%s - Heartbeat item not available: %v", conn.Name, err)
		return
	}
	health.expected.Store(true)
}

// logRejected reports the monitored items the server refused, one line per
// status code instead of one per item: a namespace of stale tags otherwise
// writes thousands of lines on every start (deviation D25). The first few
// addresses are named at the basic level, all of them at the detailed one.
func logRejected(conn *OPCUAConnection, rejected map[string][]string) {
	const sample = 3
	names := make([]string, 0, len(rejected))
	for name := range rejected {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		addrs := rejected[name]
		shown := addrs[:min(sample, len(addrs))]
		more := ""
		if len(addrs) > sample {
			more = fmt.Sprintf(" and %d more", len(addrs)-sample)
		}
		jslog.Log(jslog.LevelBasic, "%s - %d monitored items rejected: %s - e.g. %s%s",
			conn.Name, len(addrs), name, strings.Join(shown, ", "), more)
		if jslog.Level() >= jslog.LevelDetailed {
			for _, a := range addrs {
				jslog.Log(jslog.LevelDetailed, "%s - Monitored item rejected: %s - %s", conn.Name, a, name)
			}
		}
	}
}

// notificationPump turns published data changes into queued values.
func notificationPump(ctx context.Context, conn *OPCUAConnection, health *sessionHealth, notifyCh <-chan *opcua.PublishNotificationData) {
	for {
		select {
		case <-ctx.Done():
			return
		case n, ok := <-notifyCh:
			// select picks at random among ready cases: never let a value of
			// an ended session through, its handles belong to the next one.
			if !ok || ctx.Err() != nil {
				return
			}
			if n.Error != nil {
				// gopcua reports only errors it cannot recover from.
				jslog.Log(jslog.LevelBasic, "%s - subscription error: %v", conn.Name, n.Error)
				health.fail(n.Error)
				continue
			}
			dcn, isData := n.Value.(*ua.DataChangeNotification)
			if !isData {
				// Event notifications are not used by this driver; the C#
				// driver dequeues and discards them too.
				continue
			}
			health.touch()
			for _, item := range dcn.MonitoredItems {
				if item != nil && item.ClientHandle == heartbeatHandle {
					continue
				}
				handleNotification(conn, item)
			}
		}
	}
}

// handleNotification converts one reported value and queues it.
func handleNotification(conn *OPCUAConnection, item *ua.MonitoredItemNotification) {
	if item == nil {
		return
	}
	it := conn.ItemForHandle(item.ClientHandle)
	if it == nil {
		// A handle from a subscription of a previous session.
		jslog.Log(jslog.LevelDetailed, "%s - notification for unknown client handle %d",
			conn.Name, item.ClientHandle)
		return
	}
	if item.Value == nil || item.Value.Value == nil || item.Value.Value.Value() == nil {
		jslog.Log(jslog.LevelDetailed, "%s - %s %s NULL VALUE!", conn.Name, it.NodeID, it.DisplayName)
		return
	}

	CntNotificEvents.Add(1)

	// deviation D24: one item cannot take the whole queue.
	if ok, first := it.allow(time.Now()); !ok {
		it.Flooded.Add(1)
		CntFloodDropped.Add(1)
		if first {
			jslog.Log(jslog.LevelBasic,
				"%s - %s %s reports more than %d values per second; the excess is dropped",
				conn.Name, it.NodeID, it.DisplayName, maxItemEventsPerSecond)
		}
		return
	}

	tp, dbl, str, jsonStr, isArray := convertOPCValue(item.Value)

	// parity: the queue is bounded here and only here; the autotag read
	// pass enqueues unconditionally.
	if queueLen() >= DataBufferLimit {
		CntLostDataUpdates.Add(1)
		return
	}

	hasSrc := !item.Value.SourceTimestamp.IsZero()
	srcTime := item.Value.SourceTimestamp
	if hasSrc {
		srcTime = srcTime.Add(time.Duration(conn.HoursShift * float64(time.Hour)))
	}

	enqueueValue(OPCValue{
		SelfPublish:        true,
		Address:            it.NodeID,
		Asdu:               tp,
		IsArray:            isArray,
		Value:              dbl,
		ValueString:        str,
		ValueJSON:          jsonStr,
		Cot:                3,
		HasSourceTimestamp: hasSrc,
		SourceTimestamp:    srcTime,
		ServerTimestamp:    time.Now(),
		Quality:            statusIsGood(item.Value.Status),
		ConnNumber:         conn.ProtocolConnectionNumber,
		ConnName:           conn.Name,
		CommonAddress:      "",
		DisplayName:        it.DisplayName,
		// The writer fills these from what browsing recorded.
		ParentName: "",
		Path:       "",
	})
}
