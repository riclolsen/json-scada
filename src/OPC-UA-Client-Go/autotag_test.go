/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 */

// Browse and autotag against an in-process OPC UA server. No MongoDB and no
// device are needed.

package main

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/server"
	"github.com/gopcua/opcua/server/attrs"
	"github.com/gopcua/opcua/ua"
)

// freePort asks the OS for a port and gives it straight back, which is the
// usual way to pick one for a test server.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot reserve a port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// startTestServer brings up a server holding this tree, and returns a
// connected client plus the Objects node of the namespace:
//
//	Objects/Direct              variable directly under Objects
//	Objects/Boiler/Temp         writable, so it gets a command twin
//	Objects/Boiler/Text
//	Objects/Boiler/Flag
//	Objects/Boiler/Reset        method
//	Objects/Boiler/Drum/Level   two levels deep
func startTestServer(t *testing.T) (*opcua.Client, *ua.NodeID, *server.NodeNameSpace) {
	t.Helper()

	port := freePort(t)
	srv := server.New(
		server.EndPoint("127.0.0.1", port),
		server.EnableSecurity("None", ua.MessageSecurityModeNone),
		server.EnableAuthMode(ua.UserTokenTypeAnonymous),
	)

	// Named "Objects" so the browse paths have the same root as a real
	// server, where browsing starts at the standard Objects folder.
	ns := server.NewNodeNameSpace(srv, "Objects")
	root := ns.Objects()
	nsid := ns.ID()

	addVar := func(parent *server.Node, id, name string, val any, access byte) {
		n := server.NewVariableNode(ua.NewStringNodeID(nsid, id), name, val)
		ns.AddNode(n)
		parent.AddRef(n, server.RefTypeIDOrganizes, true)
		n.SetAttribute(ua.AttributeIDAccessLevel, server.DataValueFromValue(access))
		n.SetAttribute(ua.AttributeIDUserAccessLevel, server.DataValueFromValue(access))
	}
	addFolder := func(parent *server.Node, id, name string) *server.Node {
		n := server.NewFolderNode(ua.NewStringNodeID(nsid, id), name)
		ns.AddNode(n)
		parent.AddRef(n, server.RefTypeIDOrganizes, true)
		return n
	}

	const readOnly, readWrite = byte(1), byte(3)

	addVar(root, "Direct", "Direct", float64(1.5), readOnly)
	boiler := addFolder(root, "Boiler", "Boiler")
	addVar(boiler, "Boiler.Temp", "Temp", float64(42.5), readWrite)
	addVar(boiler, "Boiler.Text", "Text", "running", readOnly)
	addVar(boiler, "Boiler.Flag", "Flag", true, readOnly)
	drum := addFolder(boiler, "Boiler.Drum", "Drum")
	addVar(drum, "Boiler.Drum.Level", "Level", float64(77.25), readOnly)

	method := server.NewNode(
		ua.NewStringNodeID(nsid, "Boiler.Reset"),
		map[ua.AttributeID]*ua.DataValue{
			ua.AttributeIDNodeClass:      server.DataValueFromValue(uint32(ua.NodeClassMethod)),
			ua.AttributeIDBrowseName:     server.DataValueFromValue(attrs.BrowseName("Reset")),
			ua.AttributeIDDisplayName:    server.DataValueFromValue(attrs.DisplayName("Reset", "Reset")),
			ua.AttributeIDExecutable:     server.DataValueFromValue(true),
			ua.AttributeIDUserExecutable: server.DataValueFromValue(true),
		},
		[]*ua.ReferenceDescription{},
		nil,
	)
	ns.AddNode(method)
	boiler.AddRef(method, server.RefTypeIDOrganizes, true)
	// A real server exposes hierarchical references in both directions;
	// the Call service needs the inverse one to find the owning object.
	method.AddRef(boiler, server.RefTypeIDOrganizes, false)

	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("cannot start the test server: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	endpoint := fmt.Sprintf("opc.tcp://127.0.0.1:%d", port)
	cli, err := opcua.NewClient(endpoint, opcua.SecurityMode(ua.MessageSecurityModeNone))
	if err != nil {
		t.Fatalf("cannot build a client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := cli.Connect(ctx); err != nil {
		t.Fatalf("cannot connect to the test server: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close(context.Background()) })

	return cli, ua.NewNumericNodeID(nsid, 85), ns // 85 is the Objects folder
}

// testConn is a connection with autoCreateTags on and nothing preloaded.
func testConn() *OPCUAConnection {
	return &OPCUAConnection{
		ProtocolConnectionNumber:        81,
		Name:                            "PLC1",
		CommandsEnabled:                 true,
		AutoCreateTags:                  true,
		AutoCreateTagPublishingInterval: 2.5,
		AutoCreateTagSamplingInterval:   1.0,
		AutoCreateTagQueueSize:          5.0,
		InsertedAddresses:               map[string]bool{},
		NodeIdsDetails:                  map[string]*NodeDetails{},
		OpcSubscriptions:                map[float64][]*monItem{},
		handles:                         map[uint32]*monItem{},
	}
}

// drainQueue empties the shared acquired-value queue and returns what was
// in it, keyed by address.
func drainQueue() map[string]OPCValue {
	out := map[string]OPCValue{}
	for {
		ov, ok := dequeueValue()
		if !ok {
			return out
		}
		out[ov.Address] = ov
	}
}

func TestBrowseAndAutotagAgainstServer(t *testing.T) {
	cli, objects, _ := startTestServer(t)
	conn := testConn()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	drainQueue() // start from a clean queue

	browsed, err := browseFullAddressSpace(ctx, cli, conn, objects)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	autotagPass(ctx, cli, conn, browsed)
	queued := drainQueue()

	// Every variable and the method must have been discovered.
	for _, addr := range []string{
		"ns=1;s=Direct",
		"ns=1;s=Boiler.Temp",
		"ns=1;s=Boiler.Text",
		"ns=1;s=Boiler.Flag",
		"ns=1;s=Boiler.Drum.Level",
		"ns=1;s=Boiler.Reset",
	} {
		if _, ok := queued[addr]; !ok {
			t.Errorf("%s was not discovered", addr)
		}
	}

	// Browse paths, including the top-level quirk.
	pathCases := map[string]struct{ path, parent string }{
		"ns=1;s=Direct":            {"/Objects", "Objects"},
		"ns=1;s=Boiler.Temp":       {"Boiler", "Boiler"},
		"ns=1;s=Boiler.Drum.Level": {"Boiler/Drum", "Drum"},
	}
	for addr, want := range pathCases {
		ov, ok := queued[addr]
		if !ok {
			continue
		}
		if ov.Path != want.path {
			t.Errorf("%s path = %q, want %q", addr, ov.Path, want.path)
		}
		if ov.ParentName != want.parent {
			t.Errorf("%s parent = %q, want %q", addr, ov.ParentName, want.parent)
		}
	}

	// Types and values came through the conversion ladder.
	if ov := queued["ns=1;s=Boiler.Temp"]; ov.Asdu != "double" || ov.Value != 42.5 {
		t.Errorf("Temp = (%s,%v), want (double,42.5)", ov.Asdu, ov.Value)
	}
	if ov := queued["ns=1;s=Boiler.Text"]; ov.Asdu != "string" || ov.ValueString != "running" {
		t.Errorf("Text = (%s,%q), want (string,\"running\")", ov.Asdu, ov.ValueString)
	}
	if ov := queued["ns=1;s=Boiler.Flag"]; ov.Asdu != "boolean" || ov.Value != 1 {
		t.Errorf("Flag = (%s,%v), want (boolean,1)", ov.Asdu, ov.Value)
	}

	// A variable read carries cause of transmission 20.
	if ov := queued["ns=1;s=Boiler.Temp"]; ov.Cot != 20 {
		t.Errorf("Temp cot = %d, want 20", ov.Cot)
	}

	// Only the writable variable gets a command twin.
	if ov := queued["ns=1;s=Boiler.Temp"]; !ov.CreateCommandForSupervised {
		t.Error("the writable variable must be marked for a command twin")
	}
	for _, addr := range []string{"ns=1;s=Direct", "ns=1;s=Boiler.Text", "ns=1;s=Boiler.Drum.Level"} {
		if ov := queued[addr]; ov.CreateCommandForSupervised {
			t.Errorf("%s is read only and must not get a command twin", addr)
		}
	}

	// The method becomes a command of its own, never a monitored point.
	m := queued["ns=1;s=Boiler.Reset"]
	if !m.CreateCommandForMethod {
		t.Error("the method must be marked as a method command")
	}
	if m.Asdu != "method" {
		t.Errorf("method asdu = %q, want method", m.Asdu)
	}
	if m.Cot != 0 {
		t.Errorf("method cot = %d, want 0", m.Cot)
	}
	if m.Quality {
		t.Error("a method carries no value, so it must not be flagged good")
	}

	// Monitoring holds the variables only.
	if len(conn.ListMon) != 5 {
		t.Errorf("%d monitored items, want 5 variables (the method must not be monitored)", len(conn.ListMon))
	}
	for _, it := range conn.ListMon {
		if it.NodeID == "ns=1;s=Boiler.Reset" {
			t.Error("the method must not be monitored")
		}
		if it.SamplingMs != 1000 {
			t.Errorf("%s sampling = %v ms, want autoCreateTagSamplingInterval * 1000", it.NodeID, it.SamplingMs)
		}
		if it.QueueSize != 5 {
			t.Errorf("%s queue size = %d, want autoCreateTagQueueSize", it.NodeID, it.QueueSize)
		}
	}
}

// A point that already has a tag must not be published a second time.
func TestAutotagSkipsAlreadyInsertedAddresses(t *testing.T) {
	cli, objects, _ := startTestServer(t)
	conn := testConn()
	conn.InsertedAddresses["ns=1;s=Boiler.Temp"] = true

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	drainQueue()
	browsed, err := browseFullAddressSpace(ctx, cli, conn, objects)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	autotagPass(ctx, cli, conn, browsed)
	queued := drainQueue()

	if _, found := queued["ns=1;s=Boiler.Temp"]; found {
		t.Error("an address already present in realtimeData must not be published again")
	}
	if _, found := queued["ns=1;s=Boiler.Text"]; !found {
		t.Error("the other points must still be discovered")
	}
}

// topics restrict discovery to whole path segments.
func TestAutotagTopicFilter(t *testing.T) {
	cli, objects, _ := startTestServer(t)
	conn := testConn()
	conn.Topics = []string{"Drum"}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	drainQueue()
	browsed, err := browseFullAddressSpace(ctx, cli, conn, objects)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	autotagPass(ctx, cli, conn, browsed)
	queued := drainQueue()

	if _, found := queued["ns=1;s=Boiler.Drum.Level"]; !found {
		t.Error("a point under the topic must be discovered")
	}
	if _, found := queued["ns=1;s=Boiler.Text"]; found {
		t.Error("a point outside the topic must be skipped")
	}
}

// Methods are only discovered when the connection allows commands.
func TestAutotagSkipsMethodsWhenCommandsDisabled(t *testing.T) {
	cli, objects, _ := startTestServer(t)
	conn := testConn()
	conn.CommandsEnabled = false

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	drainQueue()
	browsed, err := browseFullAddressSpace(ctx, cli, conn, objects)
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	autotagPass(ctx, cli, conn, browsed)
	queued := drainQueue()

	if _, found := queued["ns=1;s=Boiler.Reset"]; found {
		t.Error("methods must not be discovered when commands are disabled")
	}
	if ov := queued["ns=1;s=Boiler.Temp"]; ov.CreateCommandForSupervised {
		t.Error("no command twin may be created when commands are disabled")
	}
}
