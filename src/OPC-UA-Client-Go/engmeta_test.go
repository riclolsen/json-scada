package main

import (
	"context"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/id"
	"github.com/gopcua/opcua/server"
	"github.com/gopcua/opcua/ua"
)

// TestAutotagUnitAndRange: a variable's EngineeringUnits and EURange (or
// InstrumentRange) properties give its new tag a unit and the lololo/hihihi
// limits, and get no tag of their own.
func TestAutotagUnitAndRange(t *testing.T) {
	port := freePort(t)
	srv := server.New(
		server.EndPoint("127.0.0.1", port),
		server.EnableSecurity("None", ua.MessageSecurityModeNone),
		server.EnableAuthMode(ua.UserTokenTypeAnonymous),
	)
	ns := server.NewNodeNameSpace(srv, "Objects")
	root, nsid := ns.Objects(), ns.ID()
	addVar := func(parent *server.Node, nodeID, name string, val any, ref uint32) *server.Node {
		n := server.NewVariableNode(ua.NewStringNodeID(nsid, nodeID), name, val)
		ns.AddNode(n)
		parent.AddRef(n, server.RefType(ref), true)
		n.SetAttribute(ua.AttributeIDAccessLevel, server.DataValueFromValue(byte(1)))
		n.SetAttribute(ua.AttributeIDUserAccessLevel, server.DataValueFromValue(byte(1)))
		return n
	}
	eu := func(unit string) *ua.ExtensionObject {
		return ua.NewExtensionObject(&ua.EUInformation{NamespaceURI: "http://www.opcfoundation.org/UA/units/un/cefact",
			UnitID: 4408652, DisplayName: ua.NewLocalizedText(unit), Description: ua.NewLocalizedText("degree Celsius")})
	}
	rng := func(lo, hi float64) *ua.ExtensionObject { return ua.NewExtensionObject(&ua.Range{Low: lo, High: hi}) }

	temp := addVar(root, "Temp", "Temp", float64(42.5), id.Organizes)
	addVar(temp, "Temp.EngineeringUnits", "EngineeringUnits", eu("°C"), id.HasProperty)
	addVar(temp, "Temp.EURange", "EURange", rng(-20, 150), id.HasProperty)
	addVar(temp, "Temp.InstrumentRange", "InstrumentRange", rng(-50, 200), id.HasProperty)
	addVar(temp, "Temp.Definition", "Definition", "a temperature", id.HasProperty) // not metadata

	press := addVar(root, "Press", "Press", float64(3.2), id.Organizes)
	addVar(press, "Press.InstrumentRange", "InstrumentRange", rng(0, 10), id.HasProperty)

	flow := addVar(root, "Flow", "Flow", float64(1), id.Organizes)
	addVar(flow, "Flow.EURange", "EURange", rng(5, 5), id.HasProperty) // empty: no limits
	addVar(flow, "Flow.EngineeringUnits", "EngineeringUnits",
		ua.NewExtensionObject(&ua.EUInformation{Description: ua.NewLocalizedText("m3/h")}), id.HasProperty)

	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("cannot start the test server: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	cli, err := opcua.NewClient(fmt.Sprintf("opc.tcp://127.0.0.1:%d", port), opcua.SecurityMode(ua.MessageSecurityModeNone))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := cli.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close(context.Background()) })

	conn := testConn()
	drainQueue()
	browsed, err := browseFullAddressSpace(ctx, cli, conn, ua.NewNumericNodeID(nsid, 85))
	if err != nil {
		t.Fatalf("browse: %v", err)
	}
	if err := autotagPass(ctx, cli, conn, browsed); err != nil {
		t.Fatalf("autotag: %v", err)
	}
	queued := drainQueue()

	addr := func(s string) string { return ua.NewStringNodeID(nsid, s).String() }
	for _, p := range []string{"Temp.EngineeringUnits", "Temp.EURange", "Temp.InstrumentRange",
		"Press.InstrumentRange", "Flow.EURange", "Flow.EngineeringUnits"} {
		if _, ok := queued[addr(p)]; ok {
			t.Errorf("a tag queued for the metadata property %s", p)
		}
		for _, it := range conn.ListMon {
			if it.NodeID == addr(p) {
				t.Errorf("the metadata property %s is monitored", p)
			}
		}
	}
	if _, ok := queued[addr("Temp.Definition")]; !ok {
		t.Error("another property lost its tag")
	}

	tempV := queued[addr("Temp")]
	if tempV.Unit != "°C" || !tempV.HasRange || tempV.RangeLow != -20 || tempV.RangeHigh != 150 {
		t.Errorf("Temp unit/range = %q %v %v..%v, want °C and the EURange -20..150",
			tempV.Unit, tempV.HasRange, tempV.RangeLow, tempV.RangeHigh)
	}
	doc := newRealtimeDoc(tempV, 1, 0)
	if doc["unit"] != "°C" || doc["lololoLimit"] != -20.0 || doc["hihihiLimit"] != 150.0 {
		t.Errorf("Temp tag = unit %v lololo %v hihihi %v", doc["unit"], doc["lololoLimit"], doc["hihihiLimit"])
	}
	if p := queued[addr("Press")]; p.Unit != "" || !p.HasRange || p.RangeLow != 0 || p.RangeHigh != 10 {
		t.Errorf("Press = %+v, want the InstrumentRange 0..10 and no unit", p)
	}
	f := queued[addr("Flow")]
	if f.Unit != "m3/h" || f.HasRange {
		t.Errorf("Flow = unit %q range %v, want the description as unit and no range", f.Unit, f.HasRange)
	}
	if doc := newRealtimeDoc(f, 2, 0); doc["hihihiLimit"] != math.MaxFloat64 || doc["lololoLimit"] != -math.MaxFloat64 {
		t.Errorf("Flow limits = %v %v, want the defaults", doc["lololoLimit"], doc["hihihiLimit"])
	}
}
