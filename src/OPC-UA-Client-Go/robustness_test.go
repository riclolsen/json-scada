/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 */

// Tests for the robustness behaviours: session health, rediscovery,
// typed commands, and the writer drain used before a reload.

package main

import (
	"context"
	"testing"
	"time"

	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/ua"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestSessionHealthStaleness(t *testing.T) {
	h := newSessionHealth()
	now := time.Now()

	// Without a heartbeat item silence proves nothing.
	if _, stale := h.staleFor(now.Add(time.Hour)); stale {
		t.Fatal("a session without a heartbeat must never be declared stale")
	}

	h.expected.Store(true)
	h.noteInterval(2 * time.Second)
	if _, stale := h.staleFor(now.Add(10 * time.Second)); stale {
		t.Error("10 s of silence is within the minimum budget")
	}
	if _, stale := h.staleFor(now.Add(minStaleAfter + 5*time.Second)); !stale {
		t.Error("silence beyond the budget must be stale")
	}

	// The budget grows with the publishing interval.
	h.noteInterval(60 * time.Second)
	if _, stale := h.staleFor(now.Add(minStaleAfter + 5*time.Second)); stale {
		t.Error("a 60 s publishing interval needs more than the minimum budget")
	}
	if _, stale := h.staleFor(now.Add(6 * time.Minute)); !stale {
		t.Error("silence beyond five publishing intervals must be stale")
	}

	h.touch()
	if _, stale := h.staleFor(time.Now()); stale {
		t.Error("a notification must reset the clock")
	}
}

func TestSessionHealthFailureIsKeptOnce(t *testing.T) {
	h := newSessionHealth()
	h.fail(context.Canceled)
	h.fail(context.DeadlineExceeded) // must not block
	if err := <-h.failure; err != context.Canceled {
		t.Errorf("got %v, want the first failure", err)
	}
	select {
	case err := <-h.failure:
		t.Errorf("unexpected second failure %v", err)
	default:
	}
}

// A live subscription must keep the health clock moving.
func TestLoopbackSubscriptionTouchesHealth(t *testing.T) {
	cli, objects, _ := startTestServer(t)
	conn := testConn()
	conn.AutoCreateTagPublishingInterval = 0.2
	conn.AutoCreateTagSamplingInterval = 0.1

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	drainQueue()

	browsed, err := browseFullAddressSpace(ctx, cli, conn, objects)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if err := autotagPass(ctx, cli, conn, browsed); err != nil {
		t.Fatalf("autotag: %v", err)
	}
	drainQueue()

	health := newSessionHealth()
	health.lastAlive.Store(time.Now().Add(-time.Hour).UnixNano())
	if err := setupSubscriptions(ctx, cli, conn, health); err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	waitFor(t, 10*time.Second, "a notification to reach the health clock", func() bool {
		_, stale := health.staleFor(time.Now())
		return time.Since(time.Unix(0, health.lastAlive.Load())) < 5*time.Second && !stale
	})
}

// Discovering twice over the same connection state (as a rebuilt session does
// when nothing could be inserted in between) must not register the same
// nodes for monitoring twice.
func TestAutotagDoesNotDuplicateMonitoredItems(t *testing.T) {
	cli, objects, _ := startTestServer(t)
	conn := testConn()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	drainQueue()

	run := func() {
		browsed, err := browseFullAddressSpace(ctx, cli, conn, objects)
		if err != nil {
			t.Fatalf("browse: %v", err)
		}
		if err := autotagPass(ctx, cli, conn, browsed); err != nil {
			t.Fatalf("autotag: %v", err)
		}
	}
	run()
	first := len(conn.ListMon)
	if first == 0 {
		t.Fatal("nothing was registered for monitoring")
	}
	run() // no writer ran, so no address is "inserted": the same nodes come back
	if len(conn.ListMon) != first {
		t.Errorf("ListMon grew from %d to %d on the second pass", first, len(conn.ListMon))
	}
	seen := map[string]bool{}
	for _, it := range conn.ListMon {
		if seen[it.NodeID] {
			t.Errorf("%s is monitored twice", it.NodeID)
		}
		seen[it.NodeID] = true
	}
}

func TestTypedArgument(t *testing.T) {
	cases := []struct {
		el      any
		typeID  uint32
		want    any
		wantErr bool
	}{
		{float64(7), id.Int32, int32(7), false},
		{float64(7), id.UInt16, uint16(7), false},
		{float64(7), id.Byte, uint8(7), false},
		{float64(2.5), id.Double, float64(2.5), false},
		{float64(2.5), id.Float, float32(2.5), false},
		{float64(300), id.Byte, nil, true}, // does not fit
		{true, id.Boolean, true, false},
		{float64(0), id.Boolean, false, false},
		{"hi", id.String, "hi", false},
		{float64(5), id.String, "5", false},
		{"3q2+7w==", id.ByteString, []byte{0xde, 0xad, 0xbe, 0xef}, false},
		{"not base64!", id.ByteString, nil, true},
		{"abc", id.Int32, nil, true},
	}
	for _, c := range cases {
		v, handled, err := typedArgument(c.el, c.typeID)
		if !handled {
			t.Errorf("%v as %d: not handled", c.el, c.typeID)
			continue
		}
		if c.wantErr {
			if err == nil {
				t.Errorf("%v as %d: expected an error", c.el, c.typeID)
			}
			continue
		}
		if err != nil {
			t.Errorf("%v as %d: %v", c.el, c.typeID, err)
			continue
		}
		if got := v.Value(); !equalAny(got, c.want) {
			t.Errorf("%v as %d = %v (%T), want %v (%T)", c.el, c.typeID, got, got, c.want, c.want)
		}
	}

	if _, handled, _ := typedArgument(float64(1), id.DateTime); handled {
		t.Error("types without a conversion must fall back to the ladder")
	}
}

func equalAny(a, b any) bool {
	ab, aok := a.([]byte)
	bb, bok := b.([]byte)
	if aok || bok {
		return aok && bok && string(ab) == string(bb)
	}
	return a == b
}

func TestTypedMethodArguments(t *testing.T) {
	args, err := typedMethodArguments(`[7, 2.5, "x", 9]`, []uint32{id.Int32, id.Double, 0})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 4 {
		t.Fatalf("got %d arguments", len(args))
	}
	if args[0].Value() != int32(7) {
		t.Errorf("declared Int32 sent as %T", args[0].Value())
	}
	if args[1].Value() != float64(2.5) {
		t.Errorf("declared Double sent as %T", args[1].Value())
	}
	if args[2].Value() != "x" {
		t.Errorf("undeclared string sent as %T", args[2].Value())
	}
	if args[3].Value() != int64(9) {
		t.Errorf("an argument beyond the declared ones keeps the ladder, got %T", args[3].Value())
	}

	if _, err := typedMethodArguments(`[300]`, []uint32{id.Byte}); err == nil {
		t.Error("an argument that does not fit its declared type must be refused")
	}
}

func TestStringFamilyCommandsAreEncodableAndStrict(t *testing.T) {
	for asdu, bad := range map[string]string{
		"bytestring":     "not base64!",
		"nodeid":         "i=notanumber",
		"expandednodeid": "i=notanumber",
		"guid":           "zz",
	} {
		if v, _, err := commandVariant(asdu, 0, bad); err == nil {
			t.Errorf("%s %q: expected an error, got %v", asdu, bad, v)
		}
	}
	for asdu, good := range map[string]string{
		"bytestring":     "3q2+7w==",
		"nodeid":         "ns=2;s=Foo",
		"expandednodeid": "ns=2;s=Foo",
		"guid":           "72962B91-FA75-4AE6-8D28-B404DC7DAF63",
		"localizedtext":  "Pump",
		"qualifiedname":  "Foo",
		"xmlelement":     "<a/>",
	} {
		v, _, err := commandVariant(asdu, 0, good)
		if err != nil || v == nil {
			t.Fatalf("%s: %v", asdu, err)
		}
		if _, err := v.Encode(); err != nil {
			t.Errorf("%s: cannot be encoded: %v", asdu, err)
		}
	}
	if v, _, _ := commandVariant("qualifiedname", 0, "Foo"); v.Value().(*ua.QualifiedName).NamespaceIndex != 0 {
		t.Error("a qualified name without a prefix is in namespace 0")
	}
}

func TestZeroTimeoutGetsDefault(t *testing.T) {
	for _, v := range []any{0.0, int32(0), -5.0} {
		c := connectionFromDoc(bson.M{"timeoutMs": v})
		if c.TimeoutMs != 20000 {
			t.Errorf("timeoutMs %v gave %v, want the 20000 default", v, c.TimeoutMs)
		}
	}
	if c := connectionFromDoc(bson.M{"timeoutMs": 1500.0}); c.TimeoutMs != 1500 {
		t.Errorf("an explicit timeout must be kept, got %v", c.TimeoutMs)
	}
}

func TestWaitWriterIdle(t *testing.T) {
	ctx := context.Background()
	drainQueue()
	writerBusy.Store(false)

	if !waitWriterIdle(ctx, 3*time.Second) {
		t.Error("an idle writer must be reported idle")
	}

	// A queued value or a busy flag holds the wait back until the budget ends.
	enqueueValue(OPCValue{Address: "x"})
	if waitWriterIdle(ctx, 200*time.Millisecond) {
		t.Error("a queued value means the writer is not idle")
	}
	drainQueue()

	writerBusy.Store(true)
	defer writerBusy.Store(false)
	if waitWriterIdle(ctx, 200*time.Millisecond) {
		t.Error("a busy writer is not idle")
	}
}
