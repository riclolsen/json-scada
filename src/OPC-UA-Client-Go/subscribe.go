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
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/gopcua/opcua"
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
)

// setupSubscriptions creates the connection's subscriptions on a fresh
// session.
//
// With autoCreateTags on there is one subscription holding every monitored
// item, preconfigured and discovered alike, at the connection's
// autoCreateTagPublishingInterval. With it off there is one subscription
// per distinct publishing interval found in realtimeData.
func setupSubscriptions(ctx context.Context, cli *opcua.Client, conn *OPCUAConnection) {
	if conn.AutoCreateTags {
		createSubscription(ctx, cli, conn, conn.AutoCreateTagPublishingInterval, conn.ListMon)
		return
	}

	for _, interval := range conn.SubscriptionOrder {
		createSubscription(ctx, cli, conn, interval, conn.OpcSubscriptions[interval])
	}
}

// createSubscription publishes one group of monitored items and starts the
// pump that drains its notifications into the acquired-value queue.
func createSubscription(ctx context.Context, cli *opcua.Client, conn *OPCUAConnection, intervalSeconds float64, items []*monItem) {
	if len(items) == 0 {
		return
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
			return
		}
	}
	if err != nil {
		// Loud on purpose: these points will not update at all until the
		// connection is rebuilt.
		jslog.Log(jslog.LevelNoLog,
			"%s - Error creating subscription: %v - %d monitored items with a %v s publishing interval WILL NOT UPDATE",
			conn.Name, err, len(items), intervalSeconds)
		return
	}

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
	for start := 0; start < len(requests); start += monitorChunk {
		end := min(start+monitorChunk, len(requests))
		chunk := requests[start:end]

		resp, err := sub.Monitor(ctx, ua.TimestampsToReturnBoth, chunk...)
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "%s - Error creating monitored items: %v", conn.Name, err)
			continue
		}
		// A rejected item must not abort the subscription: the C# driver
		// logs and carries on, and one bad node id in a large namespace
		// would otherwise cost every other point.
		for i, res := range resp.Results {
			if !statusIsGood(res.StatusCode) {
				jslog.Log(jslog.LevelBasic, "%s - Monitored item rejected: %s - %s",
					conn.Name, chunk[i].ItemToMonitor.NodeID, statusCodeName(res.StatusCode))
				continue
			}
			created++
		}
	}

	jslog.Log(jslog.LevelNoLog, "%s - %d Monitored items", conn.Name, created)

	go notificationPump(ctx, conn, notifyCh)
}

// notificationPump turns published data changes into queued values.
func notificationPump(ctx context.Context, conn *OPCUAConnection, notifyCh <-chan *opcua.PublishNotificationData) {
	for {
		select {
		case <-ctx.Done():
			return
		case n, ok := <-notifyCh:
			if !ok {
				return
			}
			if n.Error != nil {
				jslog.Log(jslog.LevelDetailed, "%s - subscription error: %v", conn.Name, n.Error)
				continue
			}
			dcn, isData := n.Value.(*ua.DataChangeNotification)
			if !isData {
				// Event notifications are not used by this driver; the C#
				// driver dequeues and discards them too.
				continue
			}
			for _, item := range dcn.MonitoredItems {
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

	tp, dbl, str, jsonStr, isArray := convertOPCValue(item.Value)
	CntNotificEvents.Add(1)

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
