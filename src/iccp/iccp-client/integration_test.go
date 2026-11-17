package main

import (
	"net"
	"testing"
	"time"

	"github.com/riclolsen/tase2/tase2"
)

// TestDSTSSubscriptionAndPollLoopback drives the driver's DSTS setup and poll
// paths against an in-process tase2 server (no MongoDB needed): it verifies
// dataset creation, spec-conformant DSTS activation, change-based reporting
// into dataChan, quality/timetag decoding, and the integrity poll.
func TestDSTSSubscriptionAndPollLoopback(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	// ---- Server (simulated remote control center) ----
	dm := tase2.NewDataModel()
	d := dm.AddDomain("SUB_A")
	volt := d.AddDataPoint("ICCP_RealQ_01", tase2.ICCPTypeRealQ)
	volt.UpdateValue(tase2.NewRealQ(230.5, tase2.QualityGood), tase2.QualityGood)
	d.AddDSTransferSet("DSTrans")
	dm.BuildDiscoverySnapshots()

	srvEP := tase2.NewEndpoint(tase2.EndpointPassive)
	srvEP.SetLocalAPTitle("1.1.999.1", 12)

	srv := tase2.NewServerWithConfig(dm, srvEP, tase2.ServerConfig{
		Port: port, LocalAPTitle: "1.1.999.1", LocalAEQual: 12,
		SupportedCBBs: []tase2.ConformanceBlock{
			tase2.CBB1_BasicPeriodicData, tase2.CBB2_ExtendedDataSets,
		},
		VendorName: "Loopback", ModelName: "Test", Revision: "1.0",
	})

	if err := srvEP.Listen(port); err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srvEP.Disconnect()
	go func() {
		if err := srvEP.Accept(); err == nil {
			_ = srv.Start()
		}
	}()

	// ---- Client (as the driver builds it) ----
	cliEP := tase2.NewEndpoint(tase2.EndpointActive)
	cliEP.SetLocalAPTitle("1.1.999.2", 12)
	cliEP.SetRemoteAPTitle("1.1.999.1", 12)
	time.Sleep(50 * time.Millisecond)
	if err := cliEP.Connect("127.0.0.1", port); err != nil {
		t.Fatalf("connect: %v", err)
	}
	client := tase2.NewClient(cliEP)
	defer client.Close()

	conn := protocolConnection{
		Name:                     "LOOPBACK",
		ProtocolConnectionNumber: 9001,
	}
	mappings := []tagMapping{{
		tag: rtData{
			Tag:                         "KAW2AL-27XCBR5238----K",
			ProtocolSourceObjectAddress: "SUB_A/ICCP_RealQ_01",
		},
		ref: tase2.ObjectRef{Domain: "SUB_A", Item: "ICCP_RealQ_01"},
	}}
	mappingByRef := map[string]*tagMapping{
		"SUB_A/ICCP_RealQ_01": &mappings[0],
	}
	dataChan := make(chan dataUpdate, 100)
	rt := newConnRuntime(conn, dataChan)
	isActive.Store(true)
	defer isActive.Store(false)

	domains, err := client.ListDomains()
	if err != nil {
		t.Fatalf("ListDomains: %v", err)
	}

	subs, covered := setupDSTSSubscriptions(client, rt, domains, mappings, mappingByRef, 30)
	if len(subs) != 1 {
		t.Fatalf("expected 1 DSTS subscription, got %d", len(subs))
	}
	if subs[0].domain != "SUB_A" || subs[0].slot != "DSTrans" {
		t.Errorf("subscription = %+v, want SUB_A/DSTrans", subs[0])
	}
	if !covered["SUB_A/ICCP_RealQ_01"] || len(covered) != 1 {
		t.Errorf("covered = %v, want exactly SUB_A/ICCP_RealQ_01", covered)
	}

	// Push a value change on the server; the RBE report must land in dataChan
	// with the decoded value and quality.
	time.Sleep(200 * time.Millisecond)
	dm.UpdateOnlineValue(volt, tase2.NewRealQ(231.25, tase2.QualityGood), tase2.QualityGood)

	// The initial activation snapshot (230.5) may arrive first; wait until the
	// changed value comes through.
	deadline := time.After(5 * time.Second)
	for {
		var upd dataUpdate
		select {
		case upd = <-dataChan:
		case <-deadline:
			t.Fatal("timed out waiting for DSTS change report with updated value")
		}
		if upd.invalid {
			t.Error("DSTS report: invalid = true, want false")
		}
		if upd.protocolSourceConnectionNumber != 9001 {
			t.Errorf("connection number = %v, want 9001", upd.protocolSourceConnectionNumber)
		}
		if upd.value > 231.2 && upd.value < 231.3 {
			break // change report received
		}
	}

	// Poll path: with a nil skip set everything is read.
	performPoll(client, rt, mappings, nil)
	select {
	case upd := <-dataChan:
		if upd.value < 231.2 || upd.value > 231.3 {
			t.Errorf("polled value = %v, want ~231.25", upd.value)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for polled value")
	}

	// Points covered by DSTS must be skipped by the poll.
	performPoll(client, rt, mappings, covered)
	select {
	case upd := <-dataChan:
		t.Errorf("poll of covered point produced an update: %+v", upd)
	case <-time.After(300 * time.Millisecond):
		// expected: nothing
	}

	// A mapped point the server does not have is invalidated by the poll,
	// keeping its last value.
	missing := tagMapping{
		tag: rtData{Tag: "MISSING", ProtocolSourceObjectAddress: "SUB_A/NoSuchPoint"},
		ref: tase2.ObjectRef{Domain: "SUB_A", Item: "NoSuchPoint"},
	}
	v := 12.5
	rt.seed([]rtData{{ProtocolSourceObjectAddress: "SUB_A/NoSuchPoint", SourceDataUpdate: &sourceData{ValueAtSource: &v}}})
	performPoll(client, rt, []tagMapping{missing}, nil)
	select {
	case upd := <-dataChan:
		if !upd.invalid || upd.value != 12.5 || upd.protocolSourceObjectAddress != "SUB_A/NoSuchPoint" {
			t.Errorf("missing point update = %+v, want SUB_A/NoSuchPoint invalid keeping 12.5", upd)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for the invalidation of a missing point")
	}

	// Teardown as the driver does.
	for _, s := range subs {
		if err := client.StopDSTransferSet(s.domain, s.slot); err != nil {
			t.Errorf("StopDSTransferSet: %v", err)
		}
	}
}
