/*
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

package srvapp

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
)

// RunStatusLoop must advance the instance keep-alive and publish each
// connection's stats.isConnected from HasSession, without clobbering other
// stats fields.
func TestRunStatusLoop(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	insts := db.Collection(jsmongo.ProtocolDriverInstancesCollectionName)
	conns := db.Collection(jsmongo.ProtocolConnectionsCollectionName)
	if _, err := insts.InsertOne(ctx, bson.M{"protocolDriver": "IEC60870-5-104_SERVER",
		"protocolDriverInstanceNumber": 1.0}); err != nil {
		t.Fatal(err)
	}
	if _, err := conns.InsertMany(ctx, []any{
		bson.M{"protocolConnectionNumber": 61.0, "stats": bson.M{"openConnections": 3}},
		bson.M{"protocolConnectionNumber": 62.0},
	}); err != nil {
		t.Fatal(err)
	}

	var attached atomic.Bool
	c1 := newTestConn(61, nil)
	c1.HasSession = func() bool { return attached.Load() }
	c2 := newTestConn(62, nil) // no HasSession: never connected
	e := &Engine{Cfg: jsconfig.Config{NodeName: "nodeA"}, DriverName: "IEC60870-5-104_SERVER",
		Conns: []*Conn{c1, c2}, db: db}
	e.IsMongoLive.Store(true)
	attached.Store(true)
	go e.RunStatusLoop(1, "9.9.9")

	var inst, s1, s2 bson.M
	deadline := time.Now().Add(5 * time.Second)
	for {
		_ = insts.FindOne(ctx, bson.M{}).Decode(&inst)
		_ = conns.FindOne(ctx, bson.M{"protocolConnectionNumber": 61.0}).Decode(&s1)
		_ = conns.FindOne(ctx, bson.M{"protocolConnectionNumber": 62.0}).Decode(&s2)
		if inst["activeNodeKeepAliveTimeTag"] != nil && s1["stats"] != nil && s2["stats"] != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("status not written: inst=%v s1=%v s2=%v", inst, s1, s2)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if inst["activeNodeName"] != "nodeA" || inst["softwareVersion"] != "9.9.9" {
		t.Errorf("instance = %v", inst)
	}
	st1 := docMap(s1["stats"])
	if st1["isConnected"] != true || st1["nodeName"] != "nodeA" {
		t.Errorf("conn 61 stats = %v", st1)
	}
	if st1["openConnections"] == nil {
		t.Errorf("conn 61 lost its other stats fields: %v", st1)
	}
	if st2 := docMap(s2["stats"]); st2["isConnected"] != false {
		t.Errorf("conn 62 stats = %v", st2)
	}
}

// docMap flattens a decoded sub-document (bson.D or bson.M) into a map.
func docMap(v any) map[string]any {
	out := map[string]any{}
	switch x := v.(type) {
	case bson.D:
		for _, e := range x {
			out[e.Key] = e.Value
		}
	case bson.M:
		for k, val := range x {
			out[k] = val
		}
	}
	return out
}
