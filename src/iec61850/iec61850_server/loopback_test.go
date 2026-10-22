package main

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dscsystems/go-iec61850/client"
	"github.com/dscsystems/go-iec61850/model"
	"github.com/dscsystems/go-iec61850/server"
)

// startGateway builds a model from synthetic points and serves it on a
// loopback port, exactly as the driver does at runtime.
func startGateway(t *testing.T, conn *ServerConnection, points []*Point) *Gateway {
	t.Helper()
	conn.IPAddressLocalBind = "127.0.0.1:0"
	built := BuildModel(points, conn)
	gw, err := NewGateway(conn, built)
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}
	installControlHandlers(gw)
	gw.Start()
	if !gw.Serving() {
		t.Fatal("server did not start")
	}
	applyInitialValues(gw, points)
	t.Cleanup(func() {
		gw.Stop()
		drainCommands()
	})
	return gw
}

func drainCommands() {
	for {
		if _, ok := dequeueCommand(); !ok {
			return
		}
	}
}

func dialGateway(t *testing.T, gw *Gateway) *client.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := client.Dial(ctx, gw.Addr(), client.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("dial %s: %v", gw.Addr(), err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func loopbackPoints() []*Point {
	pts := []*Point{
		pt(1, "LB_DIG_1", "digital", "supervised", "KAW2"),
		pt(2, "LB_ANA_1", "analog", "supervised", "KAW2"),
		pt(3, "LB_STR_1", "string", "supervised", "KAW2"),
		pt(4, "LB_CMD_DIG", "digital", "command", "KAW2"),
		pt(5, "LB_CMD_ANA", "analog", "command", "KAW2"),
		// A sibling whose name would extend another data object's, to be
		// sure report member matching does not confuse them.
		pt(6, "LB_DIG_2", "digital", "supervised", "KAW2"),
	}
	for _, p := range pts {
		p.Invalid = false
	}
	return pts
}

// A client must find the logical devices, the gateway marker, the data sets
// and the report control blocks.
func TestLoopbackBrowse(t *testing.T) {
	gw := startGateway(t, testConn(), loopbackPoints())
	c := dialGateway(t, gw)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	lds, err := c.LogicalDevices(ctx)
	if err != nil {
		t.Fatalf("LogicalDevices: %v", err)
	}
	if len(lds) != 1 || lds[0] != "IEC61850SRVKAW2" {
		t.Fatalf("logical devices = %v", lds)
	}

	lns, err := c.LogicalNodes(ctx, lds[0])
	if err != nil {
		t.Fatalf("LogicalNodes: %v", err)
	}
	have := map[string]bool{}
	for _, ln := range lns {
		have[ln] = true
	}
	for _, want := range []string{"LLN0", "LPHD1", "GGIO1"} {
		if !have[want] {
			t.Errorf("missing logical node %s (have %v)", want, lns)
		}
	}

	// The IEC 61850-90-2 gateway marker must be readable and true.
	v, err := c.Read(ctx, "IEC61850SRVKAW2/LPHD1.Proxy.stVal", model.ST)
	if err != nil || v == nil || !v.Bool() {
		t.Errorf("LPHD1.Proxy.stVal = %v (%v), want true", v, err)
	}

	// Data sets and report control blocks are discoverable.
	sets, err := c.LogicalNodeDirectory(ctx, "IEC61850SRVKAW2/LLN0", client.ACSIDataSet)
	if err != nil {
		t.Fatalf("data set browse: %v", err)
	}
	if len(sets) == 0 {
		t.Error("no data sets discovered")
	}
	brcbs, _ := c.LogicalNodeDirectory(ctx, "IEC61850SRVKAW2/LLN0", client.ACSIBRCB)
	urcbs, _ := c.LogicalNodeDirectory(ctx, "IEC61850SRVKAW2/LLN0", client.ACSIURCB)
	if len(brcbs) == 0 || len(urcbs) == 0 {
		t.Errorf("report control blocks: buffered=%v unbuffered=%v", brcbs, urcbs)
	}
	// maxClientConnections=2 means two instances of each control block.
	if len(brcbs) != 4 { // brcbST0101, brcbST0102, brcbMX0101, brcbMX0102
		t.Errorf("buffered control blocks = %v, want 4 instances", brcbs)
	}
	for _, n := range brcbs {
		if !strings.HasPrefix(n, "brcb") {
			t.Errorf("unexpected buffered control block name %q", n)
		}
	}
}

// Values pushed through the update path are readable by a client, with
// quality and timestamp.
func TestLoopbackReadValues(t *testing.T) {
	points := loopbackPoints()
	gw := startGateway(t, testConn(), points)
	c := dialGateway(t, gw)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pushUpdate(gw, points[0], 1)    // LB_DIG_1
	pushUpdate(gw, points[1], 12.5) // LB_ANA_1
	points[2].ValueString = "running"
	pushUpdate(gw, points[2], 0) // LB_STR_1

	if v, err := c.Read(ctx, "IEC61850SRVKAW2/GGIO1.Ind1.stVal", model.ST); err != nil || !v.Bool() {
		t.Errorf("digital = %v (%v)", v, err)
	}
	if v, err := c.Read(ctx, "IEC61850SRVKAW2/GGIO1.AnIn1.mag.f", model.MX); err != nil || v.Float64() != 12.5 {
		t.Errorf("analog = %v (%v)", v, err)
	}
	if v, err := c.Read(ctx, "IEC61850SRVKAW2/GGIO1.Str1.stVal", model.ST); err != nil || v.Text() != "running" {
		t.Errorf("string = %v (%v)", v, err)
	}
	if v, err := c.Read(ctx, "IEC61850SRVKAW2/GGIO1.Ind1.q", model.ST); err != nil ||
		model.QualityFromValue(v).Validity() != model.ValidityGood {
		t.Errorf("quality = %v (%v)", v, err)
	}
	if v, err := c.Read(ctx, "IEC61850SRVKAW2/GGIO1.Ind1.t", model.ST); err != nil || v.Time().IsZero() {
		t.Errorf("timestamp = %v (%v)", v, err)
	}

	// A whole data object reads as its structure, which is what a
	// DO-level data set member delivers in a report.
	if v, err := c.Read(ctx, "IEC61850SRVKAW2/GGIO1.Ind1", model.ST); err != nil || v.Len() < 3 {
		t.Errorf("data object read = %v (%v)", v, err)
	}
}

// pushUpdate applies one value through the driver's normal update path.
func pushUpdate(gw *Gateway, p *Point, value float64) {
	p.Value = value
	p.Invalid = false
	p.HasTimeTagAtSource = true
	p.TimeTagAtSource = time.Now().UTC()
	p.TimeTagAtSourceOk = true
	mp := gw.built.ByTag[p.Tag]
	gw.srv.Update(func(tx *server.Tx) { applyUpdate(tx, updateFromPoint(mp, p)) })
}

// A data set read returns the configured members.
func TestLoopbackDataSet(t *testing.T) {
	gw := startGateway(t, testConn(), loopbackPoints())
	c := dialGateway(t, gw)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ds, err := c.ReadDataSet(ctx, "IEC61850SRVKAW2/LLN0.DS_ST_1")
	if err != nil {
		t.Fatalf("ReadDataSet: %v", err)
	}
	// Three status points: two digitals and one string.
	if len(ds.Members) != 3 {
		t.Fatalf("members = %d, want 3: %+v", len(ds.Members), ds.Members)
	}
	for _, m := range ds.Members {
		if m.Value == nil || m.Value.Len() < 3 {
			t.Errorf("member %s is not a structure carrying value/q/t: %v", m.Ref, m.Value)
		}
	}
}

// The DO-level data set members trigger data-change reports, and a member
// whose name is extended by a sibling is not dragged in with it.
func TestLoopbackReporting(t *testing.T) {
	points := loopbackPoints()
	gw := startGateway(t, testConn(), points)
	c := dialGateway(t, gw)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reports := make(chan *client.Report, 16)
	rcb, err := c.GetRCB(ctx, "IEC61850SRVKAW2/LLN0.RP.urcbST0101")
	if err != nil {
		t.Fatalf("GetRCB: %v", err)
	}
	sub, err := c.EnableReporting(ctx, rcb, func(r *client.Report) { reports <- r })
	if err != nil {
		t.Fatalf("EnableReporting: %v", err)
	}
	defer sub.Disable(context.Background())

	// General interrogation returns every member.
	drainReports(reports)
	if err := c.TriggerGI(ctx, rcb); err != nil {
		t.Fatalf("TriggerGI: %v", err)
	}
	gi := waitReport(t, reports, 10*time.Second)
	if len(gi.Entries) != 3 {
		t.Errorf("GI entries = %d, want 3", len(gi.Entries))
	}
	for _, e := range gi.Entries {
		if e.Reason&model.ReasonGI == 0 {
			t.Errorf("GI entry %s reason = %v", e.Ref, e.Reason)
		}
	}

	// A data change on one point reports that point only.
	drainReports(reports)
	pushUpdate(gw, points[0], 1) // LB_DIG_1 -> GGIO1.Ind1
	rep := waitReport(t, reports, 10*time.Second)
	if len(rep.Entries) != 1 {
		t.Fatalf("data-change entries = %d, want 1: %+v", len(rep.Entries), rep.Entries)
	}
	e := rep.Entries[0]
	if !strings.HasSuffix(string(e.Ref), "GGIO1.Ind1") {
		t.Errorf("reported member = %s, want GGIO1.Ind1", e.Ref)
	}
	if e.Reason&model.ReasonDataChange == 0 {
		t.Errorf("reason = %v, want data-change", e.Reason)
	}
	// The entry carries the whole data object: value, quality, timestamp.
	if e.Value == nil || e.Value.Len() < 3 {
		t.Errorf("report entry is not a structure: %v", e.Value)
	}
	if v := e.Value.Index(0); v == nil || !v.Bool() {
		t.Errorf("reported stVal = %v, want true", v)
	}

	// The other digital point must not have been included.
	drainReports(reports)
	pushUpdate(gw, points[5], 1) // LB_DIG_2 -> GGIO1.Ind2
	rep = waitReport(t, reports, 10*time.Second)
	if len(rep.Entries) != 1 || !strings.HasSuffix(string(rep.Entries[0].Ref), "GGIO1.Ind2") {
		t.Errorf("second point report = %+v", rep.Entries)
	}
}

// Every report control block instance must report an RptID of its own: the
// object reference of that instance. A client binds an incoming report to
// the control block it enabled through this identifier, and a report
// naming a control block that does not exist arrives unattached to the
// model — which is what IEDExplorer showed as a missing `mag`.
func TestLoopbackReportIDsIdentifyTheirInstance(t *testing.T) {
	points := loopbackPoints()
	gw := startGateway(t, testConn(), points) // maxClientConnections 2 -> 2 instances
	c := dialGateway(t, gw)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	seen := map[string]string{}
	for _, kind := range []struct {
		fc    string
		names []client.ACSIClass
	}{{"BR", []client.ACSIClass{client.ACSIBRCB}}, {"RP", []client.ACSIClass{client.ACSIURCB}}} {
		blocks, err := c.LogicalNodeDirectory(ctx, "IEC61850SRVKAW2/LLN0", kind.names[0])
		if err != nil {
			t.Fatalf("browse %s: %v", kind.fc, err)
		}
		if len(blocks) == 0 {
			t.Fatalf("no %s control blocks", kind.fc)
		}
		for _, name := range blocks {
			ref := model.ObjectReference("IEC61850SRVKAW2/LLN0." + kind.fc + "." + name)
			rcb, err := c.GetRCB(ctx, ref)
			if err != nil {
				t.Fatalf("GetRCB %s: %v", ref, err)
			}
			want := "IEC61850SRVKAW2/LLN0$" + kind.fc + "$" + name
			if rcb.RptID != want {
				t.Errorf("%s: RptID = %q, want %q", ref, rcb.RptID, want)
			}
			if prev, dup := seen[rcb.RptID]; dup {
				t.Errorf("%s and %s share the RptID %q", prev, ref, rcb.RptID)
			}
			seen[rcb.RptID] = string(ref)
		}
	}

	// And the identifier a report actually carries is that same value.
	ref := model.ObjectReference("IEC61850SRVKAW2/LLN0.RP.urcbST0102")
	rcb, err := c.GetRCB(ctx, ref)
	if err != nil {
		t.Fatalf("GetRCB: %v", err)
	}
	reports := make(chan *client.Report, 8)
	sub, err := c.EnableReporting(ctx, rcb, func(r *client.Report) { reports <- r })
	if err != nil {
		t.Fatalf("EnableReporting: %v", err)
	}
	defer sub.Disable(context.Background())

	if err := c.TriggerGI(ctx, rcb); err != nil {
		t.Fatalf("TriggerGI: %v", err)
	}
	rep := waitReport(t, reports, 10*time.Second)
	if rep.RptID != "IEC61850SRVKAW2/LLN0$RP$urcbST0102" {
		t.Errorf("report RptID = %q, want the instance reference", rep.RptID)
	}
}

func drainReports(ch chan *client.Report) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

func waitReport(t *testing.T, ch chan *client.Report, d time.Duration) *client.Report {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(d):
		t.Fatal("timed out waiting for a report")
		return nil
	}
}

// A direct control operation produces a commandsQueue document carrying the
// source point's routing fields and the client's address.
func TestLoopbackControlDirect(t *testing.T) {
	points := loopbackPoints()
	gw := startGateway(t, testConn(), points)
	c := dialGateway(t, gw)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	drainCommands()
	co, err := c.ControlFor(ctx, "IEC61850SRVKAW2/GGIO1.SPCSO1")
	if err != nil {
		t.Fatalf("ControlFor: %v", err)
	}
	if co.Model() != model.CtlDirectNormal {
		t.Errorf("control model = %v, want direct-normal", co.Model())
	}
	if err := co.Operate(ctx, mmsBool(true)); err != nil {
		t.Fatalf("Operate: %v", err)
	}

	doc := waitCommand(t, 5*time.Second)
	if doc["tag"] != "LB_CMD_DIG" {
		t.Errorf("tag = %v", doc["tag"])
	}
	if doc["value"] != float64(1) || doc["valueString"] != "true" {
		t.Errorf("value = %v / %v", doc["value"], doc["valueString"])
	}
	if doc["protocolSourceConnectionNumber"] != float64(91) {
		t.Errorf("source connection = %v", doc["protocolSourceConnectionNumber"])
	}
	if doc["protocolSourceObjectAddress"] != "LB_CMD_DIG_addr" {
		t.Errorf("source object address = %v", doc["protocolSourceObjectAddress"])
	}
	if doc["pointKey"] != float64(4) {
		t.Errorf("pointKey = %v", doc["pointKey"])
	}
	if s, _ := doc["originatorUserName"].(string); !strings.HasPrefix(s, "IEC61850 connection: 8001 ") {
		t.Errorf("originatorUserName = %v", doc["originatorUserName"])
	}
	if s, _ := doc["originatorIpAddress"].(string); !strings.HasPrefix(s, "127.0.0.1:") {
		t.Errorf("originatorIpAddress = %v, want the client's address", doc["originatorIpAddress"])
	}

	// The operate is reflected in the status value the server serves.
	if v, err := c.Read(ctx, "IEC61850SRVKAW2/GGIO1.SPCSO1.stVal", model.ST); err != nil || !v.Bool() {
		t.Errorf("stVal after operate = %v (%v)", v, err)
	}
}

// An analogue setpoint arrives as a number, not a boolean.
func TestLoopbackControlAnalog(t *testing.T) {
	gw := startGateway(t, testConn(), loopbackPoints())
	c := dialGateway(t, gw)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	drainCommands()
	co, err := c.ControlFor(ctx, "IEC61850SRVKAW2/GGIO1.AnOut1")
	if err != nil {
		t.Fatalf("ControlFor: %v", err)
	}
	if err := co.Operate(ctx, mmsAnalogue(37.5)); err != nil {
		t.Fatalf("Operate: %v", err)
	}
	doc := waitCommand(t, 5*time.Second)
	if doc["tag"] != "LB_CMD_ANA" {
		t.Fatalf("tag = %v", doc["tag"])
	}
	if doc["value"] != 37.5 {
		t.Errorf("value = %v, want 37.5", doc["value"])
	}
}

// A select-before-operate point requires the selection first.
func TestLoopbackControlSBO(t *testing.T) {
	points := loopbackPoints()
	points[3].SrcCommandUseSBO = true // LB_CMD_DIG
	gw := startGateway(t, testConn(), points)
	c := dialGateway(t, gw)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	drainCommands()
	co, err := c.ControlFor(ctx, "IEC61850SRVKAW2/GGIO1.SPCSO1")
	if err != nil {
		t.Fatalf("ControlFor: %v", err)
	}
	if co.Model() != model.CtlSBONormal {
		t.Fatalf("control model = %v, want SBO-normal", co.Model())
	}
	// Operate runs the select the model prescribes.
	if err := co.Operate(ctx, mmsBool(true)); err != nil {
		t.Fatalf("Operate: %v", err)
	}
	doc := waitCommand(t, 5*time.Second)
	if doc["tag"] != "LB_CMD_DIG" {
		t.Errorf("tag = %v", doc["tag"])
	}
	// The select phase must not have produced a second command.
	if _, more := dequeueCommand(); more {
		t.Error("the select phase queued a command; only the operate should")
	}
}

// When commands are disabled the control is refused and the reason reaches
// the client.
func TestLoopbackControlRefusedWhenCommandsDisabled(t *testing.T) {
	conn := testConn()
	gw := startGateway(t, conn, loopbackPoints())
	c := dialGateway(t, gw)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	drainCommands()
	// The model keeps its control objects; only the handler refuses, which is
	// what a client sees when the connection has commandsEnabled false.
	gw.conn.CommandsEnabled = false
	defer func() { gw.conn.CommandsEnabled = true }()

	co, err := c.ControlFor(ctx, "IEC61850SRVKAW2/GGIO1.SPCSO1")
	if err != nil {
		t.Fatalf("ControlFor: %v", err)
	}
	err = co.Operate(ctx, mmsBool(true))
	if err == nil {
		t.Fatal("operate succeeded while commands are disabled")
	}
	var ce *client.ControlError
	if !asControlError(err, &ce) {
		t.Fatalf("error = %v, want a ControlError", err)
	}
	if ce.AddCause != model.AddCauseBlockedByMode {
		t.Errorf("addCause = %v, want blocked-by-mode", ce.AddCause)
	}
	if _, queued := dequeueCommand(); queued {
		t.Error("a refused control must not queue a command")
	}
}

// A client that is not on the allow-list is disconnected.
func TestLoopbackAllowList(t *testing.T) {
	conn := testConn()
	conn.IPAddresses = []string{"10.0.0.1"} // not loopback
	gw := startGateway(t, conn, loopbackPoints())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := client.Dial(ctx, gw.Addr(), client.WithTimeout(5*time.Second))
	if err != nil {
		return // refused at the association: also acceptable
	}
	defer c.Close()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if c.State() != 2 /* mms.StateConnected */ {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Error("a client outside the allow-list stayed connected")
}

// The connection cap refuses the extra client.
func TestLoopbackConnectionCap(t *testing.T) {
	conn := testConn()
	conn.ServerModeMultiActive = false // caps at one client
	gw := startGateway(t, conn, loopbackPoints())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	first, err := client.Dial(ctx, gw.Addr(), client.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("first client: %v", err)
	}
	defer first.Close()

	second, err := client.Dial(ctx, gw.Addr(), client.WithTimeout(2*time.Second))
	if err == nil {
		defer second.Close()
		// The transport may accept and then drop; the association must not
		// become usable.
		if _, _, _, idErr := second.MMS().Identify(ctx); idErr == nil {
			t.Error("a second client was served despite the connection cap")
		}
	}
}

// A failed bind is retried: the 1 Hz loop in main.go calls Start until the
// port is free, and the gateway then serves normally.
func TestLoopbackStartRetriesBind(t *testing.T) {
	// Hold a port, as another process would.
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	points := loopbackPoints()
	conn := testConn()
	conn.IPAddressLocalBind = busy.Addr().String()
	gw, err := NewGateway(conn, BuildModel(points, conn))
	if err != nil {
		t.Fatalf("NewGateway: %v", err)
	}
	installControlHandlers(gw)
	t.Cleanup(gw.Stop)

	gw.Start()
	if gw.Serving() {
		t.Fatal("started on a port that is in use")
	}
	_ = busy.Close()
	gw.Start()
	if !gw.Serving() {
		t.Fatal("did not start once the port was free")
	}
	applyInitialValues(gw, points)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := client.Dial(ctx, gw.Addr(), client.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	if _, err := c.LogicalDevices(ctx); err != nil {
		t.Errorf("browse: %v", err)
	}
}

// Stop is final: the library's Close shuts the server down for good, so a
// stopped gateway must not pretend to serve again.
func TestLoopbackStopIsFinal(t *testing.T) {
	gw := startGateway(t, testConn(), loopbackPoints())
	gw.Stop()
	if gw.Serving() {
		t.Fatal("still serving after stop")
	}
	gw.Start()
	if gw.Serving() {
		t.Error("a stopped gateway started again")
	}
}
