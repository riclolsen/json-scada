package main

import (
	"net"
	"testing"
	"time"

	"github.com/riclolsen/tase2/tase2"
)

// TestServerLoopback drives the driver's model and server construction
// against a live tase2 client (no MongoDB needed): it verifies that the model
// stays current with no client attached, BLT topic isolation on discovery and
// reads, ICCP-typed value decoding, refusal of commands on a connection with
// commands disabled, and the change-stream buffering path (pendingChanges +
// flushPendingChanges) delivering DSTS change reports to a subscribed client.
func TestServerLoopback(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	isActive.Store(true)
	defer isActive.Store(false)

	// ---- Data model built the same way main() builds it ----
	tagA := rtData{ID: 1, Tag: "KAW2TR1-0MTRP", Type: "analog", Value: 55.5, Group1: "SUB_A"}
	tagB := rtData{ID: 2, Tag: "HIDDEN-POINT", Type: "analog", Value: 99.9, Group1: "SUB_B"}
	cmdA := rtData{ID: 3, Tag: "KAW2-BRK-CMD", Type: "digital", Origin: "command", Group1: "SUB_A"}

	// Connection restricted to topic SUB_A, commands disabled: the BLT must
	// hide SUB_B and operates must be refused. A second connection with
	// commands enabled makes the control point exist in the shared model.
	conn := protocolConnection{
		Name:                     "LOOPBACK",
		ProtocolConnectionNumber: 9101,
		RemoteApTitle:            "1.1.999.2",
		RemoteAeQualifier:        12,
		Topics:                   []string{"SUB_A"},
	}
	other := protocolConnection{Name: "OTHER", ProtocolConnectionNumber: 9102, CommandsEnabled: true}
	allTags := []rtData{tagA, tagB, cmdA}
	model := buildModel(allTags, []protocolConnection{conn, other}, 0)
	populateRtDataCache(allTags, model.tagByKey)

	pointA := sanitizePointName(tagA.Tag)
	pointB := sanitizePointName(tagB.Tag)
	ipA := model.ipByTag[tagA.Tag]
	if ipA == nil || model.cpRefByTag[cmdA.Tag].Item != sanitizePointName(cmdA.Tag) {
		t.Fatalf("model: ipA=%v cp=%v, want both exposed", ipA, model.cpRefByTag)
	}

	// An update with no client attached must still reach the model.
	tagA.Value = 60.5
	pendingChangesMu.Lock()
	pendingChanges[ipA] = convertToICCPValue(tagA, ipA.ICCPType)
	pendingChangesMu.Unlock()
	flushPendingChanges(model.dataModel)

	cfg := tase2.ServerConfig{
		Port: port, LocalAPTitle: "1.1.999.1", LocalAEQual: 12,
		SupportedCBBs: []tase2.ConformanceBlock{
			tase2.CBB1_BasicPeriodicData, tase2.CBB2_ExtendedDataSets, tase2.CBB5_DeviceControl,
		},
		VendorName: "JSON-SCADA", ModelName: "ICCP-Server-Test", Revision: DriverVersion,
	}

	registry := newServerRegistry()

	srvEP := tase2.NewEndpoint(tase2.EndpointPassive)
	srvEP.SetLocalAPTitle("1.1.999.1", 12)
	if err := srvEP.Listen(port); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srvEP.Disconnect()

	// Serve exactly as main() does: registry bookkeeping, the event observer
	// and statistics come from the same callbacks.
	go srvEP.ServeClients(serveClientCallbacks(registry, model.dataModel, cfg, conn, model.datasetDefs, nil, nil))

	// ---- Client (peer identity matching the BLT) ----
	cliEP := tase2.NewEndpoint(tase2.EndpointActive)
	cliEP.SetLocalAPTitle("1.1.999.2", 12)
	cliEP.SetRemoteAPTitle("1.1.999.1", 12)
	time.Sleep(50 * time.Millisecond)
	if err := cliEP.Connect("127.0.0.1", port); err != nil {
		t.Fatalf("connect: %v", err)
	}
	client := tase2.NewClient(cliEP)
	defer client.Close()

	// BLT topic isolation: only SUB_A may be discovered.
	domains, err := client.ListDomains()
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}
	if len(domains) != 1 || domains[0] != "SUB_A" {
		t.Errorf("discovered domains = %v, want [SUB_A] only (BLT topic isolation)", domains)
	}

	// Granted domain reads and decodes correctly, with the value updated
	// before the client connected.
	v, err := client.ReadDataValue("SUB_A", pointA)
	if err != nil {
		t.Fatalf("ReadDataValue SUB_A/%s: %v", pointA, err)
	}
	dp := tase2.DecodeICCP(v)
	if dp.Real == nil || *dp.Real < 60.4 || *dp.Real > 60.6 {
		t.Errorf("SUB_A value = %v, want ~60.5 (updated with no client attached)", dp.Real)
	}
	if dp.Quality == nil || dp.Quality.Validity != tase2.QualityValid {
		t.Errorf("SUB_A quality = %+v, want valid", dp.Quality)
	}

	// Non-granted domain must not be readable.
	if _, err := client.ReadDataValue("SUB_B", pointB); err == nil {
		t.Error("ReadDataValue SUB_B succeeded, want denial (BLT topic isolation)")
	}

	// Commands are disabled on this connection: the operate is refused, not
	// acknowledged and dropped.
	if err := client.Operate("SUB_A", sanitizePointName(cmdA.Tag), tase2.NewIntegerValue(1)); err == nil {
		t.Error("Operate succeeded on a connection with commands disabled, want refusal")
	}

	// ---- DSTS subscription + change-stream buffering path ----
	reportCh := make(chan tase2.Report, 8)
	client.OnDSTransferSetReport(func(report tase2.Report, finished bool) {
		if finished {
			reportCh <- report
		}
	})

	dstsCfg := tase2.DSTSConfig{
		DataSet:    tase2.ObjectRef{Domain: "SUB_A", Item: "SUB_A_DataSet"},
		BufferTime: 0,
		RBE:        true,
		Conditions: tase2.DSConditionChange,
	}
	if err := client.ConfigureDSTransferSet("SUB_A", "DSTrans", dstsCfg); err != nil {
		t.Fatalf("ConfigureDSTransferSet: %v", err)
	}
	if err := client.StartDSTransferSet("SUB_A", "DSTrans"); err != nil {
		t.Fatalf("StartDSTransferSet: %v", err)
	}

	if n := len(registry.snapshot()); n != 1 {
		t.Fatalf("registry has %d servers, want 1", n)
	}

	// Buffer an update exactly as watchRealtimeDataChanges does, then flush.
	tagA.Value = 66.25
	srcTime := time.Date(2026, 9, 26, 8, 53, 7, 123000000, time.UTC)
	tagA.TimeTagAtSource, tagA.TimeTagAtSourceOk = &srcTime, true
	pendingChangesMu.Lock()
	pendingChanges[ipA] = convertToICCPValue(tagA, ipA.ICCPType)
	pendingChangesMu.Unlock()
	flushPendingChanges(model.dataModel)

	deadline := time.After(5 * time.Second)
	for {
		var report tase2.Report
		select {
		case report = <-reportCh:
		case <-deadline:
			t.Fatal("timed out waiting for DSTS change report after flushPendingChanges")
		}
		found := false
		for _, rv := range report.Values {
			if rv.Domain == "SUB_A" && rv.Item == pointA && rv.Value != nil {
				d := tase2.DecodeICCP(rv.Value)
				if d.Real != nil && *d.Real > 66.2 && *d.Real < 66.3 {
					found = true
					if d.Type != tase2.ICCPTypeRealQTimeTagExtended {
						t.Errorf("DSTS report type = %v, want RealQTimeTagExtended", d.Type)
					}
					if got, ok := d.Time(); !ok || !got.Equal(srcTime) {
						t.Errorf("DSTS report time = %v (ok=%v), want %v (ms resolution)", got, ok, srcTime)
					}
				}
			}
		}
		if found {
			break
		}
	}

	// Live statistics: this association's requests, traffic, report and
	// the peer identity recorded by the AssociationAccepted event.
	live, clients := registry.connectionStats(conn.ProtocolConnectionNumber)
	if live.requests["read"] == 0 || live.requests["getNameList"] == 0 {
		t.Errorf("live requests = %v, want read and getNameList counted", live.requests)
	}
	if live.bytesRx == 0 || live.bytesTx == 0 {
		t.Errorf("live bytes rx/tx = %d/%d, want both > 0", live.bytesRx, live.bytesTx)
	}
	if live.reportsSent == 0 {
		t.Error("live reportsSent = 0, want the DSTS change report counted")
	}
	if len(clients) != 1 || clients[0].apTitle != "1.1.999.2" || clients[0].remoteAddress == "" {
		t.Errorf("live clients = %+v, want one peer 1.1.999.2 with a remote address", clients)
	}

	// Disconnect and verify the post-disconnect hook drains the registry.
	client.Close()
	waitRegistryEmpty(t, registry)

	// The ended association's counters are kept in the connection totals.
	closed, clients := registry.connectionStats(conn.ProtocolConnectionNumber)
	if len(clients) != 0 || closed.associations != 1 {
		t.Errorf("after disconnect: %d client(s), %d association(s); want 0 and 1", len(clients), closed.associations)
	}
	if closed.requests["read"] < live.requests["read"] || closed.bytesRx < live.bytesRx || closed.reportsSent < live.reportsSent {
		t.Errorf("after disconnect: totals %+v dropped below live %+v", closed, live)
	}
	doc := statsDocument("NODE", time.Now(), closed, clients)
	if doc["clientConnections"] != 0 || doc["associations"] != int64(1) || doc["nodeName"] != "NODE" {
		t.Errorf("stats document = %v", doc)
	}

	// An inactive node drops associations as soon as they are accepted.
	isActive.Store(false)
	cliEP2 := tase2.NewEndpoint(tase2.EndpointActive)
	cliEP2.SetLocalAPTitle("1.1.999.2", 12)
	cliEP2.SetRemoteAPTitle("1.1.999.1", 12)
	cliEP2.SetConnectTimeout(2 * time.Second)
	if err := cliEP2.Connect("127.0.0.1", port); err == nil {
		client2 := tase2.NewClient(cliEP2)
		client2.MMSClient.SetResponseTimeout(2 * time.Second)
		if _, err := client2.ListDomains(); err == nil {
			t.Error("association served on an inactive node, want it dropped")
		}
		client2.Close()
	}
	if n := len(registry.snapshot()); n != 0 {
		t.Errorf("registry has %d server(s) on an inactive node, want 0", n)
	}
}

func waitRegistryEmpty(t *testing.T, registry *serverRegistry) {
	t.Helper()
	for i := 0; i < 50; i++ {
		if len(registry.snapshot()) == 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Errorf("registry still has %d server(s) after client disconnect; post-disconnect hook did not fire", len(registry.snapshot()))
}
