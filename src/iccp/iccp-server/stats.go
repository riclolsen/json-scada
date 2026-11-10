/*
 * ICCP/TASE.2 Server Driver for JSON-SCADA
 * {json:scada} - Copyright (c) 2020-2025 - Ricardo L. Olsen
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

package main

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/riclolsen/tase2/tase2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// statsPublishInterval is how often association statistics are written to
// protocolConnections.
const statsPublishInterval = 10 * time.Second

// connStats sums tase2.ServerStats over the associations of one connection.
type connStats struct {
	associations   uint64
	requests       map[string]uint64
	errors         uint64
	rejects        uint64
	reportsSent    uint64
	reportsDropped uint64
	imsSent        uint64
	bytesRx        uint64
	bytesTx        uint64
	eventsDropped  uint64
}

func newConnStats() *connStats {
	return &connStats{requests: make(map[string]uint64)}
}

func (c *connStats) add(s tase2.ServerStats) {
	c.associations++
	for k, v := range s.Requests {
		c.requests[k] += v
	}
	c.errors += s.Errors
	c.rejects += s.Rejects
	c.reportsSent += s.ReportsSent
	c.reportsDropped += s.ReportsDropped
	c.imsSent += s.InformationMessagesSent
	c.bytesRx += s.BytesRx
	c.bytesTx += s.BytesTx
	c.eventsDropped += s.EventsDropped
}

func (c *connStats) merge(o *connStats) {
	c.associations += o.associations
	for k, v := range o.requests {
		c.requests[k] += v
	}
	c.errors += o.errors
	c.rejects += o.rejects
	c.reportsSent += o.reportsSent
	c.reportsDropped += o.reportsDropped
	c.imsSent += o.imsSent
	c.bytesRx += o.bytesRx
	c.bytesTx += o.bytesTx
	c.eventsDropped += o.eventsDropped
}

// clientInfo describes one live association of a connection.
type clientInfo struct {
	remoteAddress string
	apTitle       string
	aeQualifier   int
	connectedAt   time.Time
}

// connectionStats returns the totals of a connection since the driver
// started (ended associations plus the current counters of live ones) and
// its live associations, oldest first.
func (r *serverRegistry) connectionStats(connNumber int) (*connStats, []clientInfo) {
	r.mu.Lock()
	defer r.mu.Unlock()
	total := newConnStats()
	if c := r.closed[connNumber]; c != nil {
		total.merge(c)
	}
	var clients []clientInfo
	for _, e := range r.entries {
		if e.connection.ProtocolConnectionNumber != connNumber {
			continue
		}
		total.add(e.server.Stats())
		clients = append(clients, clientInfo{
			remoteAddress: e.peer.RemoteAddr,
			apTitle:       e.peer.APTitle,
			aeQualifier:   e.peer.AEQualifier,
			connectedAt:   e.since,
		})
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i].connectedAt.Before(clients[j].connectedAt) })
	return total, clients
}

// statsDocument renders a connection's statistics for protocolConnections.
// Counters are cumulative since the driver started.
func statsDocument(nodeName string, now time.Time, s *connStats, clients []clientInfo) bson.M {
	requests := bson.M{}
	for k, v := range s.requests {
		requests[k] = int64(v)
	}
	clientDocs := bson.A{}
	for _, c := range clients {
		clientDocs = append(clientDocs, bson.M{
			"remoteAddress": c.remoteAddress,
			"apTitle":       c.apTitle,
			"aeQualifier":   c.aeQualifier,
			"connectedAt":   c.connectedAt,
		})
	}
	return bson.M{
		"nodeName":                nodeName,
		"timeTag":                 now,
		"clientConnections":       len(clients),
		"isConnected":             len(clients) > 0,
		"clients":                 clientDocs,
		"associations":            int64(s.associations),
		"requests":                requests,
		"errors":                  int64(s.errors),
		"rejects":                 int64(s.rejects),
		"reportsSent":             int64(s.reportsSent),
		"reportsDropped":          int64(s.reportsDropped),
		"informationMessagesSent": int64(s.imsSent),
		"bytesRx":                 int64(s.bytesRx),
		"bytesTx":                 int64(s.bytesTx),
		"eventsDropped":           int64(s.eventsDropped),
	}
}

// publishConnectionStats periodically writes the statistics of every
// listening connection to protocolConnections. It returns when its MongoDB
// client has been disconnected (the main loop reconnects and starts anew).
func publishConnectionStats(coll *mongo.Collection, registry *serverRegistry, conns []protocolConnection, nodeName string) {
	ticker := time.NewTicker(statsPublishInterval)
	defer ticker.Stop()
	for range ticker.C {
		for _, conn := range conns {
			s, clients := registry.connectionStats(conn.ProtocolConnectionNumber)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, err := coll.UpdateOne(ctx,
				bson.M{"protocolConnectionNumber": conn.ProtocolConnectionNumber},
				bson.M{"$set": bson.M{"stats": statsDocument(nodeName, time.Now(), s, clients)}})
			cancel()
			if errors.Is(err, mongo.ErrClientDisconnected) {
				return
			}
			if err != nil {
				LogMsg(LogLevelDetailed, "ICCP - Stats update error for %s: %v", conn.Name, err)
			}
		}
	}
}

// serveClientCallbacks returns the ServeClients build callback and
// post-disconnect hook for one listening connection: each association gets
// its own server, is tracked in the registry for change-stream fan-out and
// statistics, and has its server events observed.
func serveClientCallbacks(
	registry *serverRegistry,
	dataModel *tase2.DataModel,
	cfg tase2.ServerConfig,
	conn protocolConnection,
	datasetDefs []datasetDef,
	cmdCollection *mongo.Collection,
) (func(ce *tase2.Endpoint) *tase2.Server, tase2.PostDisconnectHook) {
	build := func(ce *tase2.Endpoint) *tase2.Server {
		// IP filtering is done at the ICCP bilateral-table level; IP-based
		// access control can be added in a future version.
		srv := buildServer(dataModel, ce, cfg, conn, datasetDefs, cmdCollection)
		// Registered before the association is served, so the first event
		// finds the entry.
		n := registry.add(srv, ce, conn)
		srv.SetOnEvent(func(ev tase2.ServerEvent) {
			if ev.Kind == tase2.EventAssociationAccepted {
				registry.setPeer(srv, ev.Peer)
			}
			logServerEvent(conn.Name, ev)
		})
		LogMsg(LogLevelNormal, "ICCP - Client associated! %s (%d active client(s))", conn.Name, n)
		return srv
	}
	hook := func(ce *tase2.Endpoint, srv *tase2.Server, serveErr error) {
		if serveErr != nil {
			LogMsg(LogLevelNormal, "ICCP - Server error for %s: %v", conn.Name, serveErr)
		}
		n := registry.remove(srv)
		LogMsg(LogLevelNormal, "ICCP - Client disconnected from %s (%d active client(s))", conn.Name, n)
	}
	return build, hook
}

// logServerEvent writes a server event to the driver log: association
// changes and dropped reports at the normal level, rejected requests and
// failed controls at the detailed level, everything else at debug.
func logServerEvent(connName string, ev tase2.ServerEvent) {
	level := LogLevelDebug
	switch ev.Kind {
	case tase2.EventAssociationAccepted, tase2.EventAssociationReleased,
		tase2.EventAssociationAborted, tase2.EventAssociationLost,
		tase2.EventReportDropped:
		level = LogLevelNormal
	case tase2.EventServiceError, tase2.EventReadRejected, tase2.EventWriteRejected:
		level = LogLevelDetailed
	case tase2.EventControlSelect, tase2.EventControlOperate, tase2.EventControlSetTag:
		if ev.Err != nil {
			level = LogLevelDetailed
		}
	}
	if level > currentLogLevel() {
		return
	}
	msg := "ICCP - " + connName + " event " + ev.Kind.String() + " peer " + ev.Peer.RemoteAddr
	if ev.Peer.APTitle != "" {
		msg += " " + ev.Peer.APTitle
	}
	if ev.Domain != "" || ev.Name != "" {
		msg += " object " + ev.Domain + "/" + ev.Name
	}
	if ev.Detail != "" {
		msg += " (" + ev.Detail + ")"
	}
	if ev.Err != nil {
		msg += ": " + ev.Err.Error()
	}
	LogMsg(level, "%s", msg)
}
