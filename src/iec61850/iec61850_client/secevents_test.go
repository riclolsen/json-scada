package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"github.com/dscsystems/go-iec61850/client"
	"github.com/dscsystems/go-iec61850/model"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func withSecWriter(t *testing.T) *jssecevents.Writer {
	t.Helper()
	secWriter = jssecevents.NewWriter(0)
	t.Cleanup(func() { secWriter = nil })
	return secWriter
}

// queued returns the events queued so far.
func queued(w *jssecevents.Writer) []jssecevents.Event {
	var out []jssecevents.Event
	for {
		select {
		case e := <-w.C():
			out = append(out, e)
		default:
			return out
		}
	}
}

func kinds(evs []jssecevents.Event) map[string]int {
	m := map[string]int{}
	for _, e := range evs {
		m[e.Kind]++
	}
	return m
}

// The IED is identified on every association; a change from the identity
// stored in the statistics is reported.
func TestSecPeerIdentification(t *testing.T) {
	w := withSecWriter(t)
	addr, _ := startTestIED(t)
	conn := newTestConnection(addr)
	conn.PeerIdentification = true
	conn.seedIdentity(bson.M{"keys": bson.A{bson.M{"k": "identify", "v": "json-scada | loopback | 0"}}})
	conn.secNewAssociation(splitHostPort(addr))
	cli := connectTest(t, conn)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	v, m, r, err := cli.MMS().Identify(ctx)
	identifyPeer(ctx, conn, cli, v, m, r, err)

	id := conn.PeerIdentityDoc()
	if id["vendor"] != "json-scada" || id["model"] != "loopback" || id["ip"] != "127.0.0.1" {
		t.Errorf("identity %v", id)
	}
	devs, _ := id["devices"].(bson.A)
	plates := 0
	for _, d := range devs {
		if _, ok := d.(bson.M)["namPlt"]; ok {
			plates++
		}
	}
	if plates == 0 {
		t.Errorf("no logical device nameplate read: %v", id)
	}
	evs := queued(w)
	k := kinds(evs)
	if k["peer.identity_changed"] != 1 || k["peer.identified"] != 0 {
		t.Fatalf("events %v", k)
	}
	for _, e := range evs {
		if e.Kind == "peer.identity_changed" && (e.Value != "json-scada | loopback | 1" || e.PeerIP != "127.0.0.1" ||
			e.SeverityLevel != 2 || e.Role != "client") {
			t.Errorf("event %+v", e)
		}
	}

	// The same identity on the next association reports nothing.
	identifyPeer(ctx, conn, cli, v, m, r, err)
	if k := kinds(queued(w)); len(k) != 0 {
		t.Errorf("unchanged identity reported: %v", k)
	}
}

// A configured point the IED does not have is reported once per
// association, however many sweeps read it.
func TestSecNonexistentObjectReportedOnce(t *testing.T) {
	w := withSecWriter(t)
	addr, _ := startTestIED(t)
	conn := newTestConnection(addr)
	conn.UseBrcb, conn.UseUrcb = false, false
	redundancy.ForceActive(true)
	defer redundancy.ForceActive(false)
	key := entryKey("simpleIOGenericIO/GGIO1.AnIn99", model.MX)
	conn.Entries[key] = &Iec61850Entry{Path: "simpleIOGenericIO/GGIO1.AnIn99", FC: model.MX, JsTag: "NOPE"}
	conn.EntryOrder = []string{key}
	conn.secNewAssociation(splitHostPort(addr))
	connectTest(t, conn)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for i := 0; i < 2; i++ {
		if err := pollSweep(ctx, conn); err != nil {
			t.Fatal(err)
		}
	}
	drainQueue()
	if k := kinds(queued(w)); k["access.object_nonexistent"] != 1 {
		t.Errorf("events %v", k)
	}
}

// A report sequence number repeated or going backwards is reported; a
// wrap-around and the segments of one report are not.
func TestSecReportSequence(t *testing.T) {
	w := withSecWriter(t)
	conn := newTestConnection("127.0.0.1:1")
	st := &rcbState{ref: "LD/LLN0.RP.urcb01"}
	for _, sq := range []uint32{10, 11, 11, 5, 255, 0, 1} {
		conn.secReportSequence(st, &client.Report{SeqNum: sq})
	}
	conn.secReportSequence(st, &client.Report{SeqNum: 1, SubSeqNum: 1})
	if k := kinds(queued(w)); k["report.sequence"] != 2 {
		t.Errorf("events %v, want 2 sequence errors (11 repeated, 5 backwards)", k)
	}
}

// Commands above the flood limit are reported once per window.
func TestSecCommandFlood(t *testing.T) {
	w := withSecWriter(t)
	conn := newTestConnection("127.0.0.1:1")
	e := &Iec61850Entry{Path: "LD/GGIO1.SPCSO1", FC: model.CO}
	for i := 0; i <= commandFloodMax*2; i++ {
		conn.secCommand(context.Background(), nil, e, bson.M{}, 1)
	}
	if k := kinds(queued(w)); k["flood.controls"] != 1 {
		t.Errorf("events %v", k)
	}
}

// A certificate refused by Go's own chain verification never reaches
// VerifyPeerCertificate: the dial error reports it, once.
func TestSecCertRejectedByChainVerification(t *testing.T) {
	w := withSecWriter(t)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer srv.Close()
	conn := newTestConnection(srv.Listener.Addr().String())
	cfg := conn.withCertEvents(&tls.Config{RootCAs: x509.NewCertPool(), ServerName: "example.com"})
	c, err := tls.Dial("tcp", srv.Listener.Addr().String(), cfg)
	if err == nil {
		c.Close()
		t.Fatal("untrusted certificate accepted")
	}
	conn.secDialFailed(fmt.Errorf("mms: TLS handshake: %w", err))
	conn.secDialFailed(err)
	evs := queued(w)
	if k := kinds(evs); k["tls.cert_rejected"] != 1 || len(evs) != 1 {
		t.Fatalf("events %v", k)
	}
	if evs[0].Peer == nil {
		t.Errorf("no certificate in the event: %+v", evs[0])
	}
}

// An identification that reads less than before keeps the keys it could
// not read, so a later change of them is still detected.
func TestSecIdentityKeepsUnreadKeys(t *testing.T) {
	withSecWriter(t)
	addr, _ := startTestIED(t)
	conn := newTestConnection(addr)
	conn.PeerIdentification = true
	conn.seedIdentity(bson.M{"keys": bson.A{bson.M{"k": "phyNam:GONE", "v": "old nameplate"}}})
	conn.secNewAssociation(splitHostPort(addr))
	cli := connectTest(t, conn)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	identifyPeer(ctx, conn, cli, "", "", "", errors.New("not supported"))
	found := false
	for _, k := range jsmongo.GetDocArray(conn.PeerIdentityDoc(), "keys") {
		found = found || (k["k"] == "phyNam:GONE" && k["v"] == "old nameplate")
	}
	if !found {
		t.Errorf("unread key lost: %v", conn.PeerIdentityDoc()["keys"])
	}
}
