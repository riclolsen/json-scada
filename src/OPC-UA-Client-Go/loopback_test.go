/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 */

// End-to-end acquisition and command paths against an in-process OPC UA
// server. No MongoDB and no device are needed; the server is the one built
// by startTestServer in autotag_test.go.

package main

import (
	"context"
	"testing"
	"time"

	"github.com/gopcua/opcua/server"
	"github.com/gopcua/opcua/ua"
)

// waitFor polls until cond holds or the budget runs out, which keeps the
// subscription tests from depending on a fixed sleep.
func waitFor(t *testing.T, budget time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out after %v waiting for %s", budget, what)
}

// setServerValue changes a value the way the field would, bypassing the
// access-level check that guards client writes: NodeNameSpace.SetAttribute
// refuses a read-only node, which is right for a client but wrong for
// simulating the device.
func setServerValue(t *testing.T, ns *server.NodeNameSpace, nodeID *ua.NodeID, v any) {
	t.Helper()
	n := ns.Node(nodeID)
	if n == nil {
		t.Fatalf("test server has no node %s", nodeID)
	}
	if err := n.SetAttribute(ua.AttributeIDValue, server.DataValueFromValue(v)); err != nil {
		t.Fatalf("cannot set %s: %v", nodeID, err)
	}
	ns.ChangeNotification(nodeID)
}

// The whole acquisition path: browse, autotag, subscribe, and a value
// change arriving as a queued update.
func TestLoopbackAcquisition(t *testing.T) {
	cli, objects, ns := startTestServer(t)
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
	autotagPass(ctx, cli, conn, browsed)

	// The initial read publishes every point once; clear it so what
	// follows can only have come from the subscription.
	initial := drainQueue()
	if _, ok := initial["ns=1;s=Boiler.Temp"]; !ok {
		t.Fatal("the initial read did not publish Boiler.Temp")
	}

	setupSubscriptions(ctx, cli, conn)

	// Move the value on the server and tell it to report the change.
	setServerValue(t, ns, ua.NewStringNodeID(1, "Boiler.Temp"), float64(123.75))

	var got OPCValue
	waitFor(t, 10*time.Second, "the changed value to be queued", func() bool {
		for _, ov := range drainQueue() {
			if ov.Address == "ns=1;s=Boiler.Temp" && ov.Value == 123.75 {
				got = ov
				return true
			}
		}
		return false
	})

	// A subscription update carries cause of transmission 3, unlike the
	// initial read's 20.
	if got.Cot != 3 {
		t.Errorf("cot = %d, want 3 for a subscription update", got.Cot)
	}
	if got.Asdu != "double" {
		t.Errorf("asdu = %q, want double", got.Asdu)
	}
	if !got.Quality {
		t.Error("a good status must not be reported as invalid")
	}
	if !got.SelfPublish {
		t.Error("a discovered point must stay self-publishing")
	}
	if got.ConnNumber != conn.ProtocolConnectionNumber || got.ConnName != conn.Name {
		t.Errorf("value is attributed to %d/%s", got.ConnNumber, got.ConnName)
	}
}

// The notification carries only a client handle, so the driver's own
// mapping is what puts the right address on the value. A wrong mapping
// would silently update the wrong tag.
func TestLoopbackNotificationAddressesAreCorrect(t *testing.T) {
	cli, objects, ns := startTestServer(t)
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
	autotagPass(ctx, cli, conn, browsed)
	drainQueue()
	setupSubscriptions(ctx, cli, conn)

	// Two different points, two different new values.
	setServerValue(t, ns, ua.NewStringNodeID(1, "Boiler.Drum.Level"), float64(11.5))
	setServerValue(t, ns, ua.NewStringNodeID(1, "Boiler.Text"), "stopped")

	// A subscription reports the current value of every item as soon as it
	// is created, so wait for the *changed* values rather than for the
	// addresses to merely appear.
	seen := map[string]OPCValue{}
	waitFor(t, 10*time.Second, "both changed values to be queued", func() bool {
		for addr, ov := range drainQueue() {
			seen[addr] = ov
		}
		return seen["ns=1;s=Boiler.Drum.Level"].Value == 11.5 &&
			seen["ns=1;s=Boiler.Text"].ValueString == "stopped"
	})

	if ov := seen["ns=1;s=Boiler.Drum.Level"]; ov.Value != 11.5 {
		t.Errorf("Level = %v, want 11.5 — the handle map put the wrong value on this address", ov.Value)
	}
	if ov := seen["ns=1;s=Boiler.Text"]; ov.ValueString != "stopped" {
		t.Errorf("Text = %q, want \"stopped\"", ov.ValueString)
	}
}

// A command written through the driver's own conversion must land on the
// server and come back through acquisition unchanged.
func TestLoopbackCommandRoundTrip(t *testing.T) {
	cli, _, _ := startTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	variant, reason, err := commandVariant("double", 55.25, "")
	if reason != "" || err != nil {
		t.Fatalf("reason=%q err=%v", reason, err)
	}

	nodeID := ua.NewStringNodeID(1, "Boiler.Temp")
	resp, err := cli.Write(ctx, &ua.WriteRequest{
		NodesToWrite: []*ua.WriteValue{{
			NodeID:      nodeID,
			AttributeID: ua.AttributeIDValue,
			Value:       &ua.DataValue{EncodingMask: ua.DataValueValue, Value: variant},
		}},
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if len(resp.Results) == 0 || !statusIsGood(resp.Results[0]) {
		t.Fatalf("write refused: %v", resp.Results)
	}

	// Read it back the way acquisition would.
	dvals, err := cli.Read(ctx, &ua.ReadRequest{
		TimestampsToReturn: ua.TimestampsToReturnBoth,
		NodesToRead:        []*ua.ReadValueID{{NodeID: nodeID, AttributeID: ua.AttributeIDValue}},
	})
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	asdu, value, _, _, _ := convertOPCValue(dvals.Results[0])
	if asdu != "double" || value != 55.25 {
		t.Errorf("read back (%s,%v), want (double,55.25)", asdu, value)
	}
}

// Writes carry only the value bit of the encoding mask (deviation D7);
// sending timestamps is what many servers refuse.
func TestLoopbackWriteSendsValueOnly(t *testing.T) {
	cli, _, _ := startTestServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// Boiler.Temp is the writable point; the read-only ones are refused
	// by the server, which is correct but not what this test is about.
	variant, _, _ := commandVariant("double", 3.5, "")
	wv := &ua.WriteValue{
		NodeID:      ua.NewStringNodeID(1, "Boiler.Temp"),
		AttributeID: ua.AttributeIDValue,
		Value:       &ua.DataValue{EncodingMask: ua.DataValueValue, Value: variant},
	}
	if wv.Value.EncodingMask != ua.DataValueValue {
		t.Fatalf("encoding mask = %08b, want only the value bit", wv.Value.EncodingMask)
	}

	resp, err := cli.Write(ctx, &ua.WriteRequest{NodesToWrite: []*ua.WriteValue{wv}})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if len(resp.Results) == 0 || !statusIsGood(resp.Results[0]) {
		t.Fatalf("write refused: %v", resp.Results)
	}
}

// With autoCreateTags off the driver subscribes exactly the preconfigured
// points, grouped by their publishing interval.
func TestLoopbackPreconfiguredSubscriptions(t *testing.T) {
	cli, _, ns := startTestServer(t)
	conn := testConn()
	conn.AutoCreateTags = false

	fast := &monItem{NodeID: "ns=1;s=Boiler.Temp", DisplayName: "Temp", SamplingMs: 100, QueueSize: 5}
	slow := &monItem{NodeID: "ns=1;s=Boiler.Drum.Level", DisplayName: "Level", SamplingMs: 100, QueueSize: 5}
	conn.OpcSubscriptions[0.2] = []*monItem{fast}
	conn.OpcSubscriptions[0.5] = []*monItem{slow}
	conn.SubscriptionOrder = []float64{0.2, 0.5}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	drainQueue()
	setupSubscriptions(ctx, cli, conn)

	setServerValue(t, ns, ua.NewStringNodeID(1, "Boiler.Temp"), float64(7.5))

	waitFor(t, 10*time.Second, "the preconfigured point to update", func() bool {
		for _, ov := range drainQueue() {
			if ov.Address == "ns=1;s=Boiler.Temp" && ov.Value == 7.5 {
				return true
			}
		}
		return false
	})

	// The display name of a preconfigured point comes from its tag, not
	// from the server's browse name.
	if fast.DisplayName != "Temp" {
		t.Errorf("display name = %q", fast.DisplayName)
	}
}
