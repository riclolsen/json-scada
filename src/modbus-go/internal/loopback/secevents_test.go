/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package loopback

// Security events and peer identification of both drivers, over real
// connections. The events are read from the writer's queue instead of
// MongoDB.

import (
	"context"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"

	"modbus-go/internal/clientapp"
	"modbus-go/internal/mbsec"
	"modbus-go/internal/serverapp"

	"github.com/otfabric/go-modbus"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// events collects what a writer queues.
type events struct {
	t       *testing.T
	w       *jssecevents.Writer
	backlog []jssecevents.Event // queued, not waited for yet
}

func newEvents(t *testing.T, role string) (*mbsec.Reporter, *events) {
	w := jssecevents.NewWriter(0)
	return &mbsec.Reporter{W: w, Role: role, Origin: jssecevents.Origin{ProtocolDriver: "TEST", ProtocolConnectionNumber: 1}},
		&events{t: t, w: w}
}

// wait returns the first event of kind (and detail substring, when given)
// queued so far or within 5 s; the others stay for later waits.
func (ev *events) wait(kind, detail string) jssecevents.Event {
	ev.t.Helper()
	match := func(e jssecevents.Event) bool { return e.Kind == kind && strings.Contains(e.Detail, detail) }
	for i, e := range ev.backlog {
		if match(e) {
			ev.backlog = append(ev.backlog[:i], ev.backlog[i+1:]...)
			return e
		}
	}
	deadline := time.After(5 * time.Second)
	for {
		select {
		case e := <-ev.w.C():
			if match(e) {
				return e
			}
			ev.backlog = append(ev.backlog, e)
		case <-deadline:
			var seen []string
			for _, e := range ev.backlog {
				seen = append(seen, e.Kind+" "+e.Detail)
			}
			ev.t.Fatalf("no %s event (%q); saw %v", kind, detail, seen)
		}
	}
}

// none fails when an event of kind is queued within d.
func (ev *events) none(kind string, d time.Duration) {
	ev.t.Helper()
	end := time.After(d)
	for {
		select {
		case e := <-ev.w.C():
			if e.Kind == kind {
				ev.t.Fatalf("unexpected %s event: %+v", kind, e)
			}
		case <-end:
			return
		}
	}
}

func startSecServer(t *testing.T, doc bson.M) (*serverapp.Server, *events) {
	t.Helper()
	base := bson.M{"protocolConnectionNumber": srvConn, "name": "SRV", "ipAddressLocalBind": "127.0.0.1:0"}
	for k, v := range doc {
		base[k] = v
	}
	srv := serverapp.NewServer(serverapp.Normalize(base), cmdSink{ch: make(chan bson.D, 64)}, nil)
	rep, ev := newEvents(t, "server")
	srv.SetSecurity(rep, nil)
	srv.PM.Rebuild(serverTags())
	srv.Start()
	t.Cleanup(srv.Stop)
	waitFor(t, "server listening", 5*time.Second, func() bool { return srv.Addr() != "" })
	return srv, ev
}

func libClient(t *testing.T, addr string) *modbus.Client {
	t.Helper()
	cl, err := modbus.New(modbus.Config{URL: "tcp://" + addr, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := cl.Open(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cl.Close() })
	return cl
}

func TestServerSecurityEvents(t *testing.T) {
	srv, ev := startSecServer(t, nil)
	cl := libClient(t, srv.Addr())
	ctx := context.Background()

	if _, err := cl.ReadRegisters(ctx, 1, 0, 2, modbus.HoldingRegister); err != nil {
		t.Fatal(err)
	}
	e := ev.wait(mbsec.KindPeerIdentified, "")
	if e.PeerIP != "127.0.0.1" || e.Role != "server" || e.Source != mbsec.SourceDriver {
		t.Fatalf("peer.identified: %+v", e)
	}
	waitFor(t, "peerIdentity in the stats", 2*time.Second, func() bool {
		id := srv.PeerIdentity()
		c, _ := id["clients"].(bson.A)
		k, _ := id["known"].(bson.A)
		return len(c) == 1 && len(k) == 1
	})

	// a read of addresses without points
	if _, err := cl.ReadRegisters(ctx, 1, 900, 2, modbus.HoldingRegister); err == nil {
		t.Fatal("read of hr:900 answered")
	}
	e = ev.wait(mbsec.KindAccessObjectNonexistent, "")
	if e.Fields["address"] != 900 || e.Fields["quantity"] != 2 || e.Fields["exception"] != 2 {
		t.Fatalf("access.object_nonexistent fields: %+v", e.Fields)
	}

	// a unit neither served nor mapped (strictUnitId off: answered)
	_, _ = cl.ReadRegisters(ctx, 77, 0, 1, modbus.HoldingRegister)
	e = ev.wait(mbsec.KindAccessUnitUnknown, "")
	if e.Fields["unitId"] != 77 {
		t.Fatalf("access.unit_unknown: %+v", e)
	}

	// a write to addresses without points, and one refused
	if err := cl.WriteRegister(ctx, 1, 901, 5); err == nil {
		t.Fatal("write to hr:901 accepted")
	}
	ev.wait(mbsec.KindCmdObjectUnknown, "")
	if err := cl.WriteCoil(ctx, 1, 5, true); err == nil {
		t.Fatal("write to the supervised DI1 accepted")
	}
	e = ev.wait(mbsec.KindCmdRejected, "supervised")
	if e.Tag != "DI1" {
		t.Fatalf("cmd.rejected: %+v", e)
	}

	// diagnostics that restart the device, identification, file records
	_, _ = cl.Diagnostics(ctx, 1, modbus.DiagnosticSubFunction(0x0001), []byte{0, 0})
	e = ev.wait(mbsec.KindCtrlRestart, "restart communications")
	if e.Action != "rejected" {
		t.Fatalf("ctrl.restart: %+v", e)
	}
	_, _ = cl.ReadDeviceIdentification(ctx, 1, modbus.DeviceIDBasic, 0)
	ev.wait(mbsec.KindAccessEnumeration, "device identification")
	_ = cl.WriteFileRecords(ctx, 1, []modbus.FileRecord{{FileNumber: 1, RecordNumber: 0, Data: []uint16{1}}})
	ev.wait(mbsec.KindFileWrite, "")

	// a command flood: more than 10 commands within 10 s
	for i := 0; i < 11; i++ {
		if err := cl.WriteRegister(ctx, 1, 10, uint16(i)); err != nil {
			t.Fatal(err)
		}
	}
	ev.wait(mbsec.KindFloodControls, "")

	// a malformed MBAP header: reported by the library, naming the real master
	raw, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	frame := make([]byte, 12)
	binary.BigEndian.PutUint16(frame[0:], 1)
	binary.BigEndian.PutUint16(frame[2:], 7) // protocol id must be 0
	binary.BigEndian.PutUint16(frame[4:], 6)
	copy(frame[6:], []byte{1, 3, 0, 0, 0, 1})
	if _, err := raw.Write(frame); err != nil {
		t.Fatal(err)
	}
	e = ev.wait("proto.bad_mbap", "")
	if e.Source != mbsec.SourceLibrary || e.PeerAddress != raw.LocalAddr().String() {
		t.Fatalf("proto.bad_mbap: source %s peer %s, want %s", e.Source, e.PeerAddress, raw.LocalAddr())
	}
}

func TestServerRefusedMasters(t *testing.T) {
	srv, ev := startSecServer(t, bson.M{"ipAddresses": bson.A{"10.9.9.9"}})
	c, err := net.Dial("tcp", srv.Addr())
	if err == nil {
		_ = c.Close()
	}
	e := ev.wait(mbsec.KindConnUnexpected, "allow-list")
	if e.Action != "rejected" || e.PeerIP != "127.0.0.1" {
		t.Fatalf("conn.unexpected: %+v", e)
	}
}

func TestServerRTUBadCRC(t *testing.T) {
	srv, ev := startSecServer(t, bson.M{"connectionMode": serverapp.ModeRTUOverTCP})
	c, err := net.Dial("tcp", srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	// read holding registers of unit 1 with a wrong CRC
	if _, err := c.Write([]byte{1, 3, 0, 0, 0, 1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	ev.wait(mbsec.KindProtoBadFrame, "bad CRC")
}

func TestServerKnownPeerMACChange(t *testing.T) {
	// The stored identity says 127.0.0.1 had another certificate: the
	// loopback has no MAC, but a TLS master with another certificate is a
	// change.
	cs := makeCerts(t)
	stored := bson.M{"peerIdentity": bson.M{"known": bson.A{bson.M{"ip": "127.0.0.1", "tlsSha256": "00ff"}}}}
	srv, ev := startSecServer(t, bson.M{"connectionMode": serverapp.ModeTLS, "stats": stored,
		"localCertFilePath": cs.srvCert, "privateKeyFilePath": cs.srvKey, "peerCertFilePath": cs.ca})
	cli := clientapp.NewConnection(clientapp.Normalize(bson.M{"protocolConnectionNumber": 60.0,
		"connectionMode": clientapp.ModeTLS, "ipAddresses": bson.A{srv.Addr()}, "pollingInterval": 200.0,
		"localCertFilePath": cs.cliCert, "privateKeyFilePath": cs.cliKey, "peerCertFilePath": cs.ca}), &rtSink{})
	cli.SetBindings([]bson.M{{"_id": "a", "tag": "AN1", "protocolSourceObjectAddress": "hr:0", "protocolSourceASDU": "float32"}})
	cli.Start()
	defer cli.Stop()
	e := ev.wait(mbsec.KindTLSCertChanged, "")
	if e.Previous != "00ff" || len(e.Value) != 64 {
		t.Fatalf("tls.cert_changed: %+v", e)
	}
	ev.none(mbsec.KindPeerIdentified, 300*time.Millisecond) // the IP was known
}

func TestTLSCertificatesRefused(t *testing.T) {
	a, b := makeCerts(t), makeCerts(t)
	srvDoc := bson.M{"connectionMode": serverapp.ModeTLS,
		"localCertFilePath": a.srvCert, "privateKeyFilePath": a.srvKey, "peerCertFilePath": a.ca}

	t.Run("client refuses the server", func(t *testing.T) {
		srv, _ := startSecServer(t, srvDoc)
		cli := clientapp.NewConnection(clientapp.Normalize(bson.M{"protocolConnectionNumber": 60.0,
			"connectionMode": clientapp.ModeTLS, "ipAddresses": bson.A{srv.Addr()},
			"localCertFilePath": b.cliCert, "privateKeyFilePath": b.cliKey, "peerCertFilePath": b.ca}), &rtSink{})
		rep, ev := newEvents(t, "client")
		cli.SetSecurity(rep, nil)
		cli.Start()
		defer cli.Stop()
		e := ev.wait(mbsec.KindTLSCertRejected, "")
		tlsDoc, _ := e.Peer["tls"].(bson.M)
		if e.Role != "client" || tlsDoc["sha256"] == nil {
			t.Fatalf("tls.cert_rejected: %+v", e)
		}
	})
	t.Run("server refuses the client", func(t *testing.T) {
		srv, ev := startSecServer(t, srvDoc)
		cli := clientapp.NewConnection(clientapp.Normalize(bson.M{"protocolConnectionNumber": 60.0,
			"connectionMode": clientapp.ModeTLS, "ipAddresses": bson.A{srv.Addr()},
			"localCertFilePath": b.cliCert, "privateKeyFilePath": b.cliKey, "peerCertFilePath": a.ca}), &rtSink{})
		cli.Start()
		defer cli.Stop()
		e := ev.wait(mbsec.KindTLSCertRejected, "")
		tlsDoc, _ := e.Peer["tls"].(bson.M)
		if e.Role != "server" || e.PeerIP != "127.0.0.1" || tlsDoc["subject"] == nil {
			t.Fatalf("tls.cert_rejected: %+v", e)
		}
	})
}

// identDevice is a go-modbus server with holding registers 0..9 and a device
// identification.
type identDevice struct{ vendor string }

func (d *identDevice) HandleCoils(context.Context, *modbus.CoilsRequest) ([]bool, error) {
	return nil, modbus.ErrIllegalFunction
}
func (d *identDevice) HandleDiscreteInputs(context.Context, *modbus.DiscreteInputsRequest) ([]bool, error) {
	return nil, modbus.ErrIllegalFunction
}
func (d *identDevice) HandleHoldingRegisters(_ context.Context, r *modbus.HoldingRegistersRequest) ([]uint16, error) {
	if int(r.Addr)+int(r.Quantity) > 10 {
		return nil, modbus.ErrIllegalDataAddress
	}
	if r.IsWrite {
		if r.Addr == 5 {
			return nil, modbus.ErrIllegalDataValue
		}
		return nil, nil
	}
	return make([]uint16, r.Quantity), nil
}
func (d *identDevice) HandleInputRegisters(context.Context, *modbus.InputRegistersRequest) ([]uint16, error) {
	return nil, modbus.ErrIllegalFunction
}
func (d *identDevice) HandleDeviceIdentification(context.Context, *modbus.DeviceIdentificationRequest) (*modbus.DeviceIdentificationResponse, error) {
	return &modbus.DeviceIdentificationResponse{ConformityLevel: 0x02, Objects: []modbus.DeviceIdentificationObject{
		{ID: 0, Value: d.vendor}, {ID: 1, Value: "PLC-100"}, {ID: 2, Value: "V2.10"}, {ID: 5, Value: "M1"},
	}}, nil
}

func startDevice(t *testing.T, d *identDevice) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	srv, err := modbus.NewServer(&modbus.ServerConfig{URL: "tcp://" + addr, Timeout: 10 * time.Second, MaxClients: 4}, d)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Stop() })
	return addr
}

func TestClientIdentifiesDevice(t *testing.T) {
	addr := startDevice(t, &identDevice{vendor: "ACME"})
	// The stored identity has another vendor: the change is reported.
	stored := bson.M{"peerIdentity": bson.M{"keys": bson.A{bson.M{"k": "unit:1", "v": "OTHER | PLC-100 | V2.10 | M1"}}}}
	cli := clientapp.NewConnection(clientapp.Normalize(bson.M{"protocolConnectionNumber": 60.0,
		"ipAddresses": bson.A{addr}, "pollingInterval": 100.0, "stats": stored}), &rtSink{})
	rep, ev := newEvents(t, "client")
	cli.SetSecurity(rep, nil)
	cli.SetBindings([]bson.M{
		{"_id": "a", "tag": "R0", "protocolSourceObjectAddress": "hr:0", "protocolSourceASDU": "uint16", "protocolSourceCommonAddress": 1.0},
		{"_id": "b", "tag": "R50", "protocolSourceObjectAddress": "hr:50", "protocolSourceASDU": "uint16", "protocolSourceCommonAddress": 1.0},
	})
	cli.Start()
	defer cli.Stop()

	e := ev.wait(mbsec.KindPeerIdentityChanged, "")
	if e.Value != "ACME | PLC-100 | V2.10 | M1" || e.Previous != "OTHER | PLC-100 | V2.10 | M1" || e.ObjectRef != "unit:1" {
		t.Fatalf("peer.identity_changed: %+v", e)
	}
	id := cli.PeerIdentity()
	units, _ := id["units"].(bson.A)
	if len(units) != 1 {
		t.Fatalf("peerIdentity: %+v", id)
	}
	objs, _ := units[0].(bson.M)["objects"].(bson.M)
	if objs["VendorName"] != "ACME" || objs["ModelName"] != "M1" || units[0].(bson.M)["method"] != "FC43/14" {
		t.Fatalf("peerIdentity units: %+v", units)
	}
	// hr:50 does not exist on the device
	e = ev.wait(mbsec.KindAccessObjectNonexistent, "")
	if e.Tag != "R50" || e.Fields["exception"] != 2 {
		t.Fatalf("access.object_nonexistent: %+v", e)
	}
}

func TestClientIdentificationOff(t *testing.T) {
	addr := startDevice(t, &identDevice{vendor: "ACME"})
	cli := clientapp.NewConnection(clientapp.Normalize(bson.M{"protocolConnectionNumber": 60.0,
		"ipAddresses": bson.A{addr}, "pollingInterval": 100.0, "peerIdentification": false}), &rtSink{})
	rep, ev := newEvents(t, "client")
	cli.SetSecurity(rep, nil)
	cli.SetBindings([]bson.M{{"_id": "a", "tag": "R0", "protocolSourceObjectAddress": "hr:0", "protocolSourceASDU": "uint16"}})
	cli.Start()
	defer cli.Stop()
	waitFor(t, "connected", 5*time.Second, func() bool { return cli.Stats().Status == "connected" })
	ev.none(mbsec.KindPeerIdentified, 300*time.Millisecond)
	if cli.PeerIdentity() != nil {
		t.Fatal("peerIdentity published with peerIdentification off")
	}
}

func TestClientCommandEvents(t *testing.T) {
	addr := startDevice(t, &identDevice{vendor: "ACME"})
	cmd := func(id int, oa string, value float64, at time.Time) bson.M {
		return bson.M{"_id": id, "tag": "C", "protocolSourceObjectAddress": oa, "protocolSourceASDU": "uint16",
			"protocolSourceCommonAddress": 1.0, "value": value, "timeTag": at}
	}
	now := time.Now()
	store := &queueStore{docs: []bson.M{
		cmd(1, "hr:x", 1, now),                    // address does not parse
		cmd(2, "hr:60", 1, now),                   // the device does not have it
		cmd(3, "hr:5", 1, now),                    // the device refuses the value
		cmd(4, "hr:1", 1, now.Add(-24*time.Hour)), // expired
	}}
	for i := 0; i < 11; i++ {
		store.docs = append(store.docs, cmd(10+i, "hr:1", float64(i), now))
	}
	cli := clientapp.NewConnection(clientapp.Normalize(bson.M{"protocolConnectionNumber": 60.0,
		"ipAddresses": bson.A{addr}, "pollingInterval": 100.0, "peerIdentification": false}), &rtSink{})
	rep, ev := newEvents(t, "client")
	cli.SetSecurity(rep, nil)
	cli.SetBindings([]bson.M{{"_id": "a", "tag": "R0", "protocolSourceObjectAddress": "hr:0", "protocolSourceASDU": "uint16"}})
	waitStart := make(chan struct{})
	cli.Commands = &gatedStore{queueStore: store, start: waitStart}
	cli.Start()
	defer cli.Stop()
	waitFor(t, "connected", 5*time.Second, func() bool { return cli.Stats().Status == "connected" })
	close(waitStart)

	ev.wait(mbsec.KindCmdObjectUnknown, "")                         // hr:x
	e := ev.wait(mbsec.KindCmdObjectUnknown, "refused the command") // hr:60
	if e.Fields["exception"] != 2 {
		t.Fatalf("cmd.object_unknown: %+v", e)
	}
	e = ev.wait(mbsec.KindCmdRejected, "refused the command") // hr:5
	if e.Fields["exception"] != 3 {
		t.Fatalf("cmd.rejected: %+v", e)
	}
	ev.wait(mbsec.KindCmdStaleTime, "")
	ev.wait(mbsec.KindFloodControls, "")
}

// gatedStore delivers its commands once start is closed (the link is up).
type gatedStore struct {
	*queueStore
	start chan struct{}
}

func (g *gatedStore) Watch(ctx context.Context, n float64, onInsert func(bson.M)) {
	select {
	case <-g.start:
	case <-ctx.Done():
		return
	}
	g.queueStore.Watch(ctx, n, onInsert)
}

// A master polling one nonexistent address is not a scan; one reading many
// different ones is.
func TestServerScanCountsDistinctAddresses(t *testing.T) {
	srv, ev := startSecServer(t, nil)
	cl := libClient(t, srv.Addr())
	ctx := context.Background()
	for i := 0; i < 30; i++ {
		_, _ = cl.ReadRegisters(ctx, 1, 900, 1, modbus.HoldingRegister)
	}
	ev.none(mbsec.KindAccessScan, 300*time.Millisecond)
	for i := 0; i < 25; i++ {
		_, _ = cl.ReadRegisters(ctx, 1, uint16(1000+i), 1, modbus.HoldingRegister)
	}
	ev.wait(mbsec.KindAccessScan, "different")
}

// A device that answers no identification keeps the stored identity, so a
// later change of it is still seen.
func TestClientKeepsStoredIdentity(t *testing.T) {
	srv, _ := startSecServer(t, nil) // serves no FC43 or FC17
	stored := bson.M{"peerIdentity": bson.M{"keys": bson.A{bson.M{"k": "unit:1", "v": "ACME | PLC-100"}}}}
	cli := clientapp.NewConnection(clientapp.Normalize(bson.M{"protocolConnectionNumber": 60.0,
		"ipAddresses": bson.A{srv.Addr()}, "pollingInterval": 100.0, "stats": stored}), &rtSink{})
	rep, ev := newEvents(t, "client")
	cli.SetSecurity(rep, nil)
	cli.SetBindings([]bson.M{{"_id": "a", "tag": "AN1", "protocolSourceObjectAddress": "hr:0",
		"protocolSourceASDU": "float32", "protocolSourceCommonAddress": 1.0}})
	cli.Start()
	defer cli.Stop()
	waitFor(t, "identification", 5*time.Second, func() bool {
		_, ok := cli.PeerIdentity()["units"]
		return ok
	})
	keys, _ := cli.PeerIdentity()["keys"].(bson.A)
	if len(keys) != 1 || keys[0].(bson.M)["v"] != "ACME | PLC-100" {
		t.Fatalf("stored keys lost: %v", keys)
	}
	ev.none(mbsec.KindPeerIdentityChanged, 200*time.Millisecond)
}
