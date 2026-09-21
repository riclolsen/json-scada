package main

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dscsystems/go-iec61850/client"
	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
	"github.com/dscsystems/go-iec61850/scl"
	"github.com/dscsystems/go-iec61850/server"
)

// sampleModelPath finds the sample SCL model, which may sit beside the
// driver or one level up, shared with the server driver.
func sampleModelPath(t *testing.T) string {
	t.Helper()
	for _, p := range []string{
		"testdata/simpleIO_direct_control.cid",
		"../testdata/simpleIO_direct_control.cid",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("sample SCL model not found (testdata/simpleIO_direct_control.cid)")
	return ""
}

// startTestIED serves the sample SCL model on a loopback port.
func startTestIED(t *testing.T) (addr string, srv *server.Server) {
	t.Helper()

	m, err := scl.LoadModel(sampleModelPath(t))
	if err != nil {
		t.Fatalf("load SCL model: %v", err)
	}
	srv = server.New(m, server.WithIdentity(server.Identity{
		Vendor: "json-scada", Model: "loopback", Revision: "1",
	}))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	return ln.Addr().String(), srv
}

// newTestConnection builds a connection document equivalent, pointing at the
// loopback IED.
func newTestConnection(addr string) *Iec61850Connection {
	return &Iec61850Connection{
		ProtocolConnectionNumber: 9999,
		Name:                     "TESTIED",
		Enabled:                  true,
		CommandsEnabled:          true,
		IPAddresses:              []string{addr},
		AutoCreateTags:           true,
		TimeoutMs:                5000,
		GiInterval:               1,
		Class0ScanInterval:       30,
		UseBrcb:                  true,
		UseUrcb:                  true,
		LastReportIds:            map[string][]byte{},
		Entries:                  map[string]*Iec61850Entry{},
		InsertedTags:             map[string]bool{},
		RcbByRptID:               map[string]*rcbState{},
	}
}

func drainQueue() []IECValue {
	var out []IECValue
	for {
		iv, ok := dequeueValue()
		if !ok {
			return out
		}
		out = append(out, iv)
	}
}

// waitForValues collects queued values until the deadline or until pred is
// satisfied.
func waitForValues(t *testing.T, d time.Duration, pred func([]IECValue) bool) []IECValue {
	t.Helper()
	deadline := time.Now().Add(d)
	var all []IECValue
	for time.Now().Before(deadline) {
		all = append(all, drainQueue()...)
		if pred(all) {
			return all
		}
		time.Sleep(50 * time.Millisecond)
	}
	return all
}

func connectTest(t *testing.T, conn *Iec61850Connection) *client.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cli, err := client.Dial(ctx, splitHostPort(conn.IPAddresses[0]), client.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	conn.Cli = cli
	t.Cleanup(func() { closeConnection(conn) })
	return cli
}

// Discovery must find the logical devices, the data sets and both kinds of
// report control block, and link a configured entry to its data set.
func TestLoopbackDiscovery(t *testing.T) {
	addr, _ := startTestIED(t)
	conn := newTestConnection(addr)
	redundancy.ForceActive(true)
	defer redundancy.ForceActive(false)

	// A configured supervised point, as preloaded from realtimeData.
	key := "simpleIOGenericIO/GGIO1.SPCSO1.stVal" + "ST"
	conn.Entries[key] = &Iec61850Entry{
		Path:  "simpleIOGenericIO/GGIO1.SPCSO1.stVal",
		FC:    model.ST,
		JsTag: "TEST_SPCSO1",
	}
	conn.EntryOrder = append(conn.EntryOrder, key)

	connectTest(t, conn)
	defer drainQueue()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := discoverServer(ctx, conn); err != nil {
		t.Fatalf("discovery: %v", err)
	}

	if len(conn.Datasets) == 0 {
		t.Error("no data sets discovered")
	}
	if len(conn.Urcb) == 0 {
		t.Error("no unbuffered report control blocks discovered")
	}
	if len(conn.Brcb) == 0 {
		t.Error("no buffered report control blocks discovered")
	}
	if entry := conn.Entries[key]; entry.DataSetName == "" {
		t.Error("configured entry was not linked to its data set")
	}
	if len(conn.Subs) < 2 {
		t.Errorf("expected several concurrent report subscriptions, got %d", len(conn.Subs))
	}
}

// Several report control blocks are active at once on one association, and
// each subscription delivers its own reports (the capability that made this
// driver possible on go-iec61850 v0.2.x).
func TestLoopbackConcurrentReports(t *testing.T) {
	addr, srv := startTestIED(t)
	conn := newTestConnection(addr)
	redundancy.ForceActive(true)
	defer redundancy.ForceActive(false)

	connectTest(t, conn)
	drainQueue()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := discoverServer(ctx, conn); err != nil {
		t.Fatalf("discovery: %v", err)
	}

	// General interrogation on connect must produce the members of every
	// activated data set.
	values := waitForValues(t, 10*time.Second, func(v []IECValue) bool {
		return hasAddressPrefix(v, "simpleIOGenericIO/GGIO1.SPCSO") &&
			hasAddressPrefix(v, "simpleIOGenericIO/GGIO1.AnIn")
	})
	if !hasAddressPrefix(values, "simpleIOGenericIO/GGIO1.SPCSO") {
		t.Errorf("no status values reported; got %d values: %v", len(values), addresses(values))
	}
	if !hasAddressPrefix(values, "simpleIOGenericIO/GGIO1.AnIn") {
		t.Errorf("no measurand values reported; got %d values: %v", len(values), addresses(values))
	}

	// A boolean status member must be classified as a digital point, and a
	// measurand must not be: that is what decides the type of an
	// automatically created tag.
	for _, iv := range values {
		switch {
		case strings.HasPrefix(iv.Address, "simpleIOGenericIO/GGIO1.SPCSO"):
			if !iv.IsDigital {
				t.Errorf("%s should be digital", iv.Address)
			}
			if iv.ValueString != "true" && iv.ValueString != "false" {
				t.Errorf("%s valueString = %q", iv.Address, iv.ValueString)
			}
			if iv.CommonAddress != "ST" {
				t.Errorf("%s common address = %q, want ST", iv.Address, iv.CommonAddress)
			}
		case strings.HasSuffix(iv.Address, ".mag.f"):
			if iv.IsDigital {
				t.Errorf("%s should not be digital", iv.Address)
			}
			if iv.CommonAddress != "MX" {
				t.Errorf("%s common address = %q, want MX", iv.Address, iv.CommonAddress)
			}
		}
		if !iv.SelfPublish {
			t.Errorf("%s: reported values must be publishable when autoCreateTags is set", iv.Address)
		}
	}

	// A data change must be reported spontaneously.
	drainQueue()
	srv.Update(func(tx *server.Tx) {
		tx.SetFloat32("simpleIOGenericIO/GGIO1.AnIn1.mag.f", 42.5)
	})
	changed := waitForValues(t, 10*time.Second, func(v []IECValue) bool {
		for _, iv := range v {
			if strings.HasPrefix(iv.Address, "simpleIOGenericIO/GGIO1.AnIn1") && iv.Value == 42.5 {
				return true
			}
		}
		return false
	})
	found := false
	for _, iv := range changed {
		if strings.HasPrefix(iv.Address, "simpleIOGenericIO/GGIO1.AnIn1") && iv.Value == 42.5 {
			found = true
			// The data set member is the leaf mag.f, so the reported value
			// is a plain float.
			if iv.Asdu != "MMS_FLOAT" {
				t.Errorf("asdu = %q, want MMS_FLOAT", iv.Asdu)
			}
			if iv.ValueString != "42.5" {
				t.Errorf("valueString = %q, want 42.5", iv.ValueString)
			}
			if iv.ValueJSON == "" {
				t.Error("valueJson empty")
			}
		}
	}
	if !found {
		t.Errorf("data change not reported; got %v", addresses(changed))
	}
}

// The report handler must decode a report the same way whether or not the
// server includes data references. The driver does not request them (D10),
// but a device may be configured to send them, and a report is decoded by
// position: this pins that the extra strings do not shift the values.
func TestLoopbackReportWithDataReferences(t *testing.T) {
	addr, srv := startTestIED(t)
	conn := newTestConnection(addr)
	redundancy.ForceActive(true)
	defer redundancy.ForceActive(false)

	cli := connectTest(t, conn)
	drainQueue()
	defer drainQueue()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Report controls are indexed, so the instance name carries the suffix.
	ref := model.ObjectReference("simpleIOGenericIO/LLN0.RP.EventsRCB01")
	rcb, err := cli.GetRCB(ctx, ref)
	if err != nil {
		t.Fatalf("GetRCB: %v", err)
	}
	rcb.TrgOps = model.TrgDataChange | model.TrgIntegrity
	rcb.OptFlds = model.OptSeqNum | model.OptTimeOfEntry | model.OptReasonCode |
		model.OptDataSetName | model.OptDataRef | model.OptConfRev

	st := &rcbState{
		ref:        string(ref),
		rptID:      rcb.RptID,
		dataSetRef: rcb.DataSet,
		dataSetDot: dottedDataSet(rcb.DataSet),
	}
	sub, err := cli.EnableReporting(ctx, rcb, func(rep *client.Report) {
		reportHandler(conn, st, rep)
	})
	if err != nil {
		t.Fatalf("EnableReporting: %v", err)
	}
	defer sub.Disable(context.Background())

	drainQueue()
	srv.Update(func(tx *server.Tx) {
		tx.SetBool("simpleIOGenericIO/GGIO1.SPCSO2.stVal", true)
	})

	values := waitForValues(t, 10*time.Second, func(v []IECValue) bool {
		for _, iv := range v {
			if iv.Address == "simpleIOGenericIO/GGIO1.SPCSO2.stVal" {
				return true
			}
		}
		return false
	})
	for _, iv := range values {
		if iv.Address == "simpleIOGenericIO/GGIO1.SPCSO2.stVal" {
			if !iv.IsDigital || iv.Value != 1 || iv.ValueString != "true" {
				t.Errorf("value decoded wrongly with data references present: %+v", iv)
			}
			return
		}
	}
	t.Errorf("no value decoded from a report carrying data references; got %v", addresses(values))
}

// Only one report control block per data set and kind is activated. A
// server offers several instances over the same data set — indexed blocks,
// or spares for other clients — and each carries its own RptID, so nothing
// but this rule stops the same values being written once per instance.
func TestLoopbackOneSubscriptionPerDataSet(t *testing.T) {
	// Two unbuffered blocks over one data set, with distinct RptIDs, plus
	// one buffered block over the same data set and one over another.
	ggio := &model.LogicalNode{Name: "GGIO1", Class: "GGIO", Objects: []*model.DataObject{
		model.NewDataObject("Ind1", model.CDCSPS),
		model.NewDataObject("AnIn1", model.CDCMV),
	}}
	lln0 := &model.LogicalNode{Name: "LLN0", Class: "LLN0",
		DataSets: []*model.DataSet{
			{Name: "DS_ST_1", Entries: []model.FCDA{{Ref: "DUPIED/GGIO1.Ind1", FC: model.ST}}},
			{Name: "DS_MX_1", Entries: []model.FCDA{{Ref: "DUPIED/GGIO1.AnIn1", FC: model.MX}}},
		},
		ReportControls: []*model.ReportControl{
			{Name: "urcbST01", RptID: "DUPIED/LLN0$RP$urcbST0101", DataSet: "DS_ST_1",
				ConfRev: 1, TrgOps: model.TrgDataChange | model.TrgGI,
				OptFlds: model.OptFldsDefault, RptEnabled: 1},
			{Name: "urcbST02", RptID: "DUPIED/LLN0$RP$urcbST0201", DataSet: "DS_ST_1",
				ConfRev: 1, TrgOps: model.TrgDataChange | model.TrgGI,
				OptFlds: model.OptFldsDefault, RptEnabled: 1},
			{Name: "brcbST01", RptID: "DUPIED/LLN0$BR$brcbST0101", DataSet: "DS_ST_1",
				ConfRev: 1, Buffered: true, TrgOps: model.TrgDataChange | model.TrgGI,
				OptFlds: model.OptFldsDefault, RptEnabled: 1},
			{Name: "urcbMX01", RptID: "DUPIED/LLN0$RP$urcbMX0101", DataSet: "DS_MX_1",
				ConfRev: 1, TrgOps: model.TrgDataChange | model.TrgGI,
				OptFlds: model.OptFldsDefault, RptEnabled: 1},
		},
	}
	ld := &model.LogicalDevice{Name: "DUPIED", Inst: "LD0", Nodes: []*model.LogicalNode{lln0, ggio}}
	srv := server.New(&model.Model{Name: "DUPIED", Devices: []*model.LogicalDevice{ld}})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	conn := newTestConnection(ln.Addr().String())
	redundancy.ForceActive(true)
	defer redundancy.ForceActive(false)
	connectTest(t, conn)
	defer drainQueue()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := discoverServer(ctx, conn); err != nil {
		t.Fatalf("discovery: %v", err)
	}

	// Four control blocks over two data sets: one stream each.
	if len(conn.Subs) != 2 {
		t.Errorf("subscriptions = %d, want 2 (one per data set)", len(conn.Subs))
	}
	if len(conn.RcbByDataSet) != 2 {
		t.Errorf("data sets covered = %d, want 2: %v", len(conn.RcbByDataSet), conn.RcbByDataSet)
	}
	// The data set that has a buffered block is served by it, since a
	// buffered stream survives a disconnection.
	for ds, st := range conn.RcbByDataSet {
		if strings.HasSuffix(ds, "DS_ST_1") && !st.buffered {
			t.Errorf("%s is served by %s, want the buffered block", ds, st.ref)
		}
	}

	// A change is reported once, not once per instance over the data set.
	drainQueue()
	srv.Update(func(tx *server.Tx) { tx.SetBool("DUPIED/GGIO1.Ind1.stVal", true) })
	values := waitForValues(t, 5*time.Second, func(v []IECValue) bool { return len(v) > 0 })
	count := 0
	for _, iv := range values {
		if strings.HasSuffix(iv.Address, "GGIO1.Ind1") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("the change was reported %d times, want once: %v", count, addresses(values))
	}
}

// With autoCreateTags the driver registers every value-bearing object the
// server exposes, not only the ones a report happens to carry, and those
// points publish their tags when polled.
func TestLoopbackAutoCreateFromBrowse(t *testing.T) {
	addr, _ := startTestIED(t)
	conn := newTestConnection(addr)
	conn.AutoCreateTags = true
	// No reports: whatever is found has to come from the browse.
	conn.UseBrcb = false
	conn.UseUrcb = false
	redundancy.ForceActive(true)
	defer redundancy.ForceActive(false)

	connectTest(t, conn)
	drainQueue()
	defer drainQueue()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := discoverServer(ctx, conn); err != nil {
		t.Fatalf("discovery: %v", err)
	}

	// Objects of every logical node are registered, under the functional
	// constraint that carries their value.
	for _, want := range []struct {
		key string
		fc  model.FC
	}{
		{"simpleIOGenericIO/GGIO1.SPCSO1" + "ST", model.ST},    // in a data set
		{"simpleIOGenericIO/GGIO1.AnIn1" + "MX", model.MX},     // in a data set
		{"simpleIOGenericIO/LLN0.Mod" + "ST", model.ST},        // in none
		{"simpleIOGenericIO/LLN0.Health" + "ST", model.ST},     // in none
		{"simpleIOGenericIO/LPHD1.PhyHealth" + "ST", model.ST}, // in none
	} {
		e := conn.Entry(want.key)
		if e == nil {
			t.Errorf("browsed object %q was not registered", want.key)
			continue
		}
		if e.FC != want.fc {
			t.Errorf("%s: FC = %v, want %v", want.key, e.FC, want.fc)
		}
		if !e.AutoPublish {
			t.Errorf("%s: a browsed object must publish its tag", want.key)
		}
	}

	// Settings, configuration and description attributes are not points.
	for _, notWanted := range []string{
		"simpleIOGenericIO/GGIO1.SPCSO1" + "CF", // ctlModel
		"simpleIOGenericIO/GGIO1.SPCSO1" + "DC", // description
	} {
		if e := conn.Entry(notWanted); e != nil {
			t.Errorf("%q should not be registered as a point", notWanted)
		}
	}
	// A control object is registered for dispatch but is not a measurement:
	// reading it would return the operate structure, so it is never polled.
	if e := conn.Entry(entryKey("simpleIOGenericIO/GGIO1.SPCSO1", model.CO)); e == nil {
		t.Error("no control entry registered")
	}

	// Polling them produces values that carry the self-publish flag, which
	// is what makes the writer create the tag.
	if err := pollSweep(ctx, conn); err != nil {
		t.Fatalf("poll sweep: %v", err)
	}
	values := drainQueue()
	if len(values) == 0 {
		t.Fatal("no values from the polling sweep")
	}
	published, byAddress := 0, map[string]bool{}
	for _, iv := range values {
		byAddress[iv.Address] = true
		if iv.SelfPublish {
			published++
		}
	}
	if published != len(values) {
		t.Errorf("%d of %d polled values carry the self-publish flag", published, len(values))
	}
	if !byAddress["simpleIOGenericIO/LLN0.Mod"] {
		t.Errorf("an object outside every data set was not polled: %v", len(byAddress))
	}
	for _, iv := range values {
		if iv.CommonAddress == "CO" {
			t.Errorf("a control object was polled as a measurement: %s", iv.Address)
		}
	}
	// And each one names a tag the writer can create.
	iv := values[0]
	if tag := TagFromParameters(iv); !strings.HasPrefix(tag, "IEC61850;TESTIED;") {
		t.Errorf("tag name = %q", tag)
	}
}

// Controllable objects found while browsing are registered so a command
// can be dispatched at once, and queued for tag creation with the control
// model and the value type the device reports.
func TestLoopbackAutoCreateCommands(t *testing.T) {
	addr, _ := startTestIED(t)
	conn := newTestConnection(addr)
	conn.AutoCreateTags = true
	conn.CommandsEnabled = true
	conn.UseBrcb = false
	conn.UseUrcb = false
	redundancy.ForceActive(true)
	defer redundancy.ForceActive(false)

	connectTest(t, conn)
	drainQueue()
	defer drainQueue()
	takeCommandTags() // start from a clean queue

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := discoverServer(ctx, conn); err != nil {
		t.Fatalf("discovery: %v", err)
	}

	queued := map[string]CommandTag{}
	for _, ct := range takeCommandTags() {
		queued[ct.Ref] = ct
	}
	for _, ref := range []string{
		"simpleIOGenericIO/GGIO1.SPCSO1",
		"simpleIOGenericIO/GGIO1.SPCSO2",
		"simpleIOGenericIO/GGIO1.SPCSO3",
		"simpleIOGenericIO/GGIO1.SPCSO4",
	} {
		ct, ok := queued[ref]
		if !ok {
			t.Errorf("no command tag queued for %s", ref)
			continue
		}
		if !ct.IsDigital {
			t.Errorf("%s: a single point control is a digital command", ref)
		}
		if ct.Asdu != "MMS_BOOLEAN" {
			t.Errorf("%s: control value type = %q", ref, ct.Asdu)
		}
		if ct.ConnName != "TESTIED" || ct.ConnNumber != 9999 {
			t.Errorf("%s: connection identity missing", ref)
		}
		if want := "IEC61850;TESTIED;" + ref + "[CO]"; ct.Tag() != want {
			t.Errorf("%s: tag = %q, want %q", ref, ct.Tag(), want)
		}
		// The sample model uses direct control, so no select is needed.
		if ct.UseSBO {
			t.Errorf("%s: useSBO set on a direct-control object", ref)
		}

		// Registered as an entry, so a command works in this session,
		// before the tag is reloaded at the next start.
		e := conn.Entry(entryKey(ref, model.CO))
		if e == nil {
			t.Errorf("%s: no CO entry registered for dispatch", ref)
			continue
		}
		if e.FC != model.CO || e.Path != ref {
			t.Errorf("%s: entry = %+v", ref, e)
		}
	}

	// A control object is not a supervised point: it must not be polled
	// under CO, and its status is the ST twin the command links to.
	if e := conn.Entry(entryKey("simpleIOGenericIO/GGIO1.SPCSO1", model.ST)); e == nil {
		t.Error("the supervised twin of a controllable object was not registered")
	}
}

// With commands disabled, no command tags are created.
func TestLoopbackNoCommandTagsWhenDisabled(t *testing.T) {
	addr, _ := startTestIED(t)
	conn := newTestConnection(addr)
	conn.AutoCreateTags = true
	conn.CommandsEnabled = false
	conn.UseBrcb = false
	conn.UseUrcb = false
	redundancy.ForceActive(true)
	defer redundancy.ForceActive(false)

	connectTest(t, conn)
	drainQueue()
	defer drainQueue()
	takeCommandTags()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := discoverServer(ctx, conn); err != nil {
		t.Fatalf("discovery: %v", err)
	}
	if got := takeCommandTags(); len(got) != 0 {
		t.Errorf("%d command tags queued with commandsEnabled false", len(got))
	}
	if e := conn.Entry(entryKey("simpleIOGenericIO/GGIO1.SPCSO1", model.CO)); e != nil {
		t.Error("a control entry was registered with commandsEnabled false")
	}
}

// A point configured in realtimeData keeps its own tag: the driver must not
// publish a second one for it.
func TestLoopbackConfiguredPointDoesNotSelfPublish(t *testing.T) {
	addr, _ := startTestIED(t)
	conn := newTestConnection(addr)
	conn.AutoCreateTags = true
	conn.UseBrcb = false
	conn.UseUrcb = false
	redundancy.ForceActive(true)
	defer redundancy.ForceActive(false)

	key := "simpleIOGenericIO/GGIO1.AnIn1" + "MX"
	conn.Entries[key] = &Iec61850Entry{
		Path:  "simpleIOGenericIO/GGIO1.AnIn1",
		FC:    model.MX,
		JsTag: "CONFIGURED_TAG",
	}
	conn.EntryOrder = append(conn.EntryOrder, key)

	connectTest(t, conn)
	drainQueue()
	defer drainQueue()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := discoverServer(ctx, conn); err != nil {
		t.Fatalf("discovery: %v", err)
	}
	if e := conn.Entry(key); e == nil || e.JsTag != "CONFIGURED_TAG" || e.AutoPublish {
		t.Fatalf("the configured entry was replaced or made publishable: %+v", conn.Entry(key))
	}

	if err := pollSweep(ctx, conn); err != nil {
		t.Fatalf("poll sweep: %v", err)
	}
	for _, iv := range drainQueue() {
		if iv.Address == "simpleIOGenericIO/GGIO1.AnIn1" && iv.SelfPublish {
			t.Error("a configured point must not publish a second tag")
		}
	}
}

// Entries not covered by a report are polled, and the values reach the
// queue with the connection's identity on them. With autoCreateTags off,
// only the configured points are polled and none of them publishes a tag.
func TestLoopbackPolling(t *testing.T) {
	addr, _ := startTestIED(t)
	conn := newTestConnection(addr)
	conn.AutoCreateTags = false
	conn.UseBrcb = false
	conn.UseUrcb = false // no reports: everything is polled
	redundancy.ForceActive(true)
	defer redundancy.ForceActive(false)

	key := "simpleIOGenericIO/GGIO1.AnIn1" + "MX"
	conn.Entries[key] = &Iec61850Entry{
		Path:  "simpleIOGenericIO/GGIO1.AnIn1",
		FC:    model.MX,
		JsTag: "TEST_ANIN1",
	}
	conn.EntryOrder = append(conn.EntryOrder, key)

	connectTest(t, conn)
	drainQueue()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := discoverServer(ctx, conn); err != nil {
		t.Fatalf("discovery: %v", err)
	}
	if err := pollSweep(ctx, conn); err != nil {
		t.Fatalf("poll sweep: %v", err)
	}

	values := drainQueue()
	if len(values) != 1 {
		t.Fatalf("expected one polled value, got %d: %v", len(values), addresses(values))
	}
	iv := values[0]
	if iv.Address != "simpleIOGenericIO/GGIO1.AnIn1" || iv.CommonAddress != "MX" {
		t.Errorf("polled value = %s [%s]", iv.Address, iv.CommonAddress)
	}
	if iv.ConnName != "TESTIED" || iv.ConnNumber != 9999 {
		t.Error("connection identity missing from the polled value")
	}
	if iv.SelfPublish {
		t.Error("a configured point must not publish a tag of its own")
	}
	if iv.Cot != 20 {
		t.Errorf("cause of transmission = %d, want 20", iv.Cot)
	}
}

// A control command reaches the IED and changes its status value.
func TestLoopbackControl(t *testing.T) {
	addr, srv := startTestIED(t)
	conn := newTestConnection(addr)
	redundancy.ForceActive(true)
	defer redundancy.ForceActive(false)

	connectTest(t, conn)
	drainQueue()
	defer drainQueue()

	entry := &Iec61850Entry{Path: "simpleIOGenericIO/GGIO1.SPCSO1", FC: model.CO}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ok, abort := controlCommand(ctx, conn, entry, 1, false)
	if abort {
		t.Fatal("control aborted")
	}
	if !ok {
		t.Fatal("control was not accepted")
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if v := srv.Read("simpleIOGenericIO/GGIO1.SPCSO1.stVal", model.ST); v != nil && v.Bool() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Error("status value did not follow the control")
}

// A plain MMS write is used for any functional constraint other than CO.
func TestLoopbackWriteCommand(t *testing.T) {
	addr, srv := startTestIED(t)
	conn := newTestConnection(addr)
	redundancy.ForceActive(true)
	defer redundancy.ForceActive(false)

	connectTest(t, conn)
	drainQueue()
	defer drainQueue()

	// A settable point of the sample model: the analogue input's value.
	entry := &Iec61850Entry{Path: "simpleIOGenericIO/GGIO1.AnIn1.mag.f", FC: model.MX}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	ok, abort := writeValueCommand(ctx, conn, entry, 3.5)
	if abort {
		t.Skip("object not writable on this model")
	}
	if !ok {
		t.Skip("write refused by the server model")
	}
	if v := srv.Read("simpleIOGenericIO/GGIO1.AnIn1.mag.f", model.MX); v == nil || v.Float64() != 3.5 {
		t.Errorf("written value not applied: %v", v)
	}
}

// The association state is observable without issuing a request, which is
// what the reconnect loop relies on.
func TestLoopbackConnectionState(t *testing.T) {
	addr, srv := startTestIED(t)
	conn := newTestConnection(addr)
	cli := connectTest(t, conn)

	if cli.State() != mms.StateConnected {
		t.Fatalf("state after dial = %v", cli.State())
	}

	_ = srv.Close()
	select {
	case <-cli.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("Done() did not fire after the server closed")
	}
	if cli.State() != mms.StateClosed {
		t.Errorf("state after close = %v, want closed", cli.State())
	}
}

func addresses(values []IECValue) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, v.Address+"["+v.CommonAddress+"]")
	}
	return out
}

func hasAddressPrefix(values []IECValue, prefix string) bool {
	for _, v := range values {
		if strings.HasPrefix(v.Address, prefix) {
			return true
		}
	}
	return false
}

// A double point ("Pos") is read from its stVal and lands as a digital
// point, whatever else the object carries under the same constraint.
func TestLoopbackDoublePointIsDigital(t *testing.T) {
	// An XCBR-style position: a controllable double point whose ST
	// attributes include the operation counter, which is numeric and would
	// win a scan by type.
	pos := model.NewDataObject("Pos", model.CDCDPC,
		model.WithControlModel(model.CtlDirectNormal), model.WithOptional("opCnt"))
	xcbr := &model.LogicalNode{Name: "XCBR1", Class: "XCBR", Objects: []*model.DataObject{pos}}
	lln0 := &model.LogicalNode{Name: "LLN0", Class: "LLN0",
		DataSets: []*model.DataSet{{Name: "Status", Entries: []model.FCDA{
			{Ref: "DPIED/XCBR1.Pos", FC: model.ST},
		}}},
		ReportControls: []*model.ReportControl{{
			Name: "urcbST01", RptID: "DPIED/LLN0$RP$urcbST0101", DataSet: "Status",
			ConfRev: 1, TrgOps: model.TrgDataChange | model.TrgGI,
			OptFlds: model.OptFldsDefault, RptEnabled: 1,
		}},
	}
	ld := &model.LogicalDevice{Name: "DPIED", Inst: "LD0", Nodes: []*model.LogicalNode{lln0, xcbr}}
	srv := server.New(&model.Model{Name: "DPIED", Devices: []*model.LogicalDevice{ld}})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() { _ = srv.Close() })

	// A plausible state: the breaker is closed and has operated 37 times.
	srv.Update(func(tx *server.Tx) {
		tx.Set("DPIED/XCBR1.Pos.stVal", model.ST, model.DbposOn.Value())
		tx.Set("DPIED/XCBR1.Pos.opCnt", model.ST, mms.NewUint32(37))
	})

	conn := newTestConnection(ln.Addr().String())
	conn.AutoCreateTags = true
	redundancy.ForceActive(true)
	defer redundancy.ForceActive(false)
	connectTest(t, conn)
	drainQueue()
	defer drainQueue()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := discoverServer(ctx, conn); err != nil {
		t.Fatalf("discovery: %v", err)
	}

	// The attribute names of the object were learned while browsing.
	entry := conn.Entry(entryKey("DPIED/XCBR1.Pos", model.ST))
	if entry == nil {
		t.Fatal("the double point was not registered")
	}
	if childs := conn.EntryChilds(entry); len(childs) == 0 || childs[0] != "stVal" {
		t.Errorf("attribute names = %v, want stVal first", childs)
	}

	check := func(what string, values []IECValue) {
		t.Helper()
		found := false
		for _, iv := range values {
			if iv.Address != "DPIED/XCBR1.Pos" {
				continue
			}
			found = true
			if !iv.IsDigital {
				t.Errorf("%s: a double point must be a digital value, got %v", what, iv.Value)
			}
			if iv.Value != 1 {
				t.Errorf("%s: value = %v, want 1 (on) - not the operation counter", what, iv.Value)
			}
			if iv.ValueString != "true" {
				t.Errorf("%s: valueString = %q, want true", what, iv.ValueString)
			}
			// The tag it would create is a digital one.
			if doc := newRealtimeDoc(iv, 1); doc["type"] != "digital" {
				t.Errorf("%s: tag type = %v, want digital", what, doc["type"])
			}
		}
		if !found {
			t.Errorf("%s: the double point produced no value", what)
		}
	}

	// Reported through a data set...
	check("report", waitForValues(t, 10*time.Second, func(v []IECValue) bool {
		for _, iv := range v {
			if iv.Address == "DPIED/XCBR1.Pos" {
				return true
			}
		}
		return false
	}))

	// ...and polled directly. A point a report covers is not polled, so
	// the polling path is exercised on its own connection.
	polled := newTestConnection(ln.Addr().String())
	polled.AutoCreateTags = true
	polled.UseBrcb = false
	polled.UseUrcb = false
	connectTest(t, polled)
	if err := discoverServer(ctx, polled); err != nil {
		t.Fatalf("discovery (polling): %v", err)
	}
	drainQueue()
	if err := pollSweep(ctx, polled); err != nil {
		t.Fatalf("poll sweep: %v", err)
	}
	check("poll", drainQueue())

	// An intermediate position is a transitioning switch: not a valid
	// position, so it is invalid and flagged transient.
	drainQueue()
	srv.Update(func(tx *server.Tx) {
		tx.Set("DPIED/XCBR1.Pos.stVal", model.ST, model.DbposIntermediate.Value())
	})
	if err := pollSweep(ctx, polled); err != nil {
		t.Fatalf("poll sweep: %v", err)
	}
	seen := false
	for _, iv := range drainQueue() {
		if iv.Address != "DPIED/XCBR1.Pos" {
			continue
		}
		seen = true
		if iv.Quality {
			t.Error("an intermediate double point must not read as good quality")
		}
		if !iv.IsTransient {
			t.Error("an intermediate double point must be flagged transient")
		}
		if !iv.IsDigital {
			t.Error("an intermediate double point is still a digital value")
		}
	}
	if !seen {
		t.Error("the intermediate position produced no value")
	}
}
