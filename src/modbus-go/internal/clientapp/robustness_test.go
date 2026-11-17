/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package clientapp

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"modbus-go/internal/mbcore"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type countSink struct {
	mu  sync.Mutex
	ops []RtOp
}

func (s *countSink) Apply(_ context.Context, ops []RtOp) error {
	s.mu.Lock()
	s.ops = append(s.ops, ops...)
	s.mu.Unlock()
	return nil
}

func (s *countSink) invalidated() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, op := range s.ops {
		if op.Set["sourceDataUpdate.invalidAtSource"] == true {
			return true
		}
	}
	return false
}

// A peer that accepts the connection and never answers is a lost link: the
// connection is not reported connected, the tags are invalidated and the next
// address is tried.
func TestSilentDeviceIsLinkDown(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() { _, _ = io.Copy(io.Discard, c) }()
		}
	}()
	for _, mode := range []string{ModeTCP, ModeRTUOverTCP} {
		t.Run(mode, func(t *testing.T) {
			sink := &countSink{}
			c := NewConnection(Normalize(bson.M{
				"protocolConnectionNumber": 1.0, "connectionMode": mode,
				"ipAddresses": bson.A{ln.Addr().String(), "127.0.0.1:1"},
				"timeoutMs":   100.0, "maxRetries": 0.0, "pollingInterval": 50.0,
			}), sink)
			c.SetBindings([]bson.M{{"_id": 1.0, "tag": "A", "protocolSourceObjectAddress": "hr:0"}})
			c.Start()
			defer c.Stop()
			failedOver := false
			deadline := time.Now().Add(4 * time.Second)
			for time.Now().Before(deadline) && !failedOver {
				st := c.Stats()
				if st.Status == "connected" {
					t.Fatalf("silent peer reported connected: %+v", st)
				}
				failedOver = strings.HasSuffix(st.ConnectedTo, "127.0.0.1:1")
				time.Sleep(20 * time.Millisecond)
			}
			if !failedOver {
				t.Fatalf("no failover to the second address: %+v", c.Stats())
			}
			if !sink.invalidated() {
				t.Fatal("tags not invalidated")
			}
		})
	}
}

// Re-reading identical bindings keeps the change cache; a changed binding
// starts over.
func TestRebindKeepsChangeCache(t *testing.T) {
	c := NewConnection(Normalize(bson.M{}), &countSink{})
	docs := []bson.M{{"_id": 1.0, "tag": "A", "protocolSourceObjectAddress": "hr:0", "protocolSourceASDU": "uint16"}}
	c.SetBindings(docs)
	c.adoptBlocks(c.blocks)
	c.blocks[0].lastRaw, c.blocks[0].invalidPublished = []byte{0, 1}, true

	c.SetBindings(docs)
	c.adoptBlocks(c.blocks)
	if b := c.blocks[0]; string(b.lastRaw) != "\x00\x01" || !b.invalidPublished {
		t.Fatalf("cache lost on identical rebind: %v %v", b.lastRaw, b.invalidPublished)
	}

	docs[0]["kconv1"] = 2.0
	c.SetBindings(docs)
	c.adoptBlocks(c.blocks)
	if b := c.blocks[0]; b.lastRaw != nil {
		t.Fatalf("cache kept after the scaling changed: %v", b.lastRaw)
	}
}

func TestCommandValidation(t *testing.T) {
	c := NewConnection(Normalize(bson.M{}), &countSink{})
	for _, tc := range []struct {
		addr, asdu string
		value      any
		want       string
	}{
		{"di:3", "", 1.0, "area di is read-only"},
		{"ir:3", "uint16", 1.0, "area ir is read-only"},
		{"co:3", "", "on", "invalid value for an ON/OFF command"},
		{"hr:3.b1", "", "x", "invalid value for an ON/OFF command"},
		{"hr:3", "bool", "x", "invalid value for an ON/OFF command"},
	} {
		// the client is never reached: validation comes first
		ok, desc := c.executeCommand(context.Background(), nil, bson.M{
			"protocolSourceObjectAddress": tc.addr, "protocolSourceASDU": tc.asdu, "value": tc.value,
		})
		if ok || !strings.HasPrefix(desc, tc.want) {
			t.Errorf("%s %s %v: got %v %q, want %q", tc.addr, tc.asdu, tc.value, ok, desc, tc.want)
		}
	}
}

// A register read with ASDU bool is ON when non-zero, and a register bit is
// read even with ASDU bool.
func TestRegisterBoolDecode(t *testing.T) {
	c := NewConnection(Normalize(bson.M{}), &countSink{})
	c.SetBindings([]bson.M{
		{"_id": "w", "tag": "W", "protocolSourceObjectAddress": "hr:0", "protocolSourceASDU": "bool"},
		{"_id": "i", "tag": "I", "protocolSourceObjectAddress": "hr:1", "protocolSourceASDU": "bool", "kconv1": -1.0},
		{"_id": "b", "tag": "B", "protocolSourceObjectAddress": "hr:2.b4", "protocolSourceASDU": "bool"},
	})
	u := NewUpdater(1)
	c.decodeAndQueue(c.blocks[0], []byte{0x01, 0x00, 0x00, 0x05, 0x00, 0x10}, u, false)
	got := map[string]any{}
	for _, op := range u.ops {
		sdu, ok := op.Set["sourceDataUpdate"].(bson.M)
		if !ok {
			t.Fatalf("tag %v not decoded: %v", op.ID, op.Set)
		}
		got[op.ID.(string)] = sdu["valueAtSource"]
	}
	if got["w"] != 1.0 || got["i"] != 0.0 || got["b"] != 1.0 {
		t.Fatalf("values %v", got)
	}
	if registerWord([]byte{0x12, 0x34}, 0, mbcore.Perm{}) != 0x1234 {
		t.Fatal("register word")
	}
}
