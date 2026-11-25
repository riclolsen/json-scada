package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"github.com/dscsystems/go-iec61850/client"
	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
	"github.com/dscsystems/go-iec61850/server"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// withSec enables the security events for one test.
func withSec(t *testing.T, conn *ServerConnection) *secState {
	t.Helper()
	conn.PeerIdentification = true
	sec = newSecState(conn, "test")
	t.Cleanup(func() { sec.close(); sec = nil })
	return sec
}

// waitEvent returns the first queued event of kind, failing after a while.
func waitEvent(t *testing.T, s *secState, kind string) jssecevents.Event {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case e := <-s.writer.C():
			if e.Kind == kind {
				return e
			}
		case <-deadline:
			t.Fatalf("no %s event", kind)
		}
	}
}

func dial(t *testing.T, gw *Gateway) *client.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := client.Dial(ctx, gw.Addr(), client.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// A client outside the allow-list is reported and disconnected.
func TestSecEventUnexpectedClient(t *testing.T) {
	conn := testConn()
	conn.IPAddresses = []string{"10.9.9.9"}
	s := withSec(t, conn)
	gw := startGateway(t, conn, loopbackPoints())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if c, err := client.Dial(ctx, gw.Addr(), client.WithTimeout(2*time.Second)); err == nil {
		defer c.Close()
	}
	e := waitEvent(t, s, "conn.unexpected")
	if e.PeerIP != "127.0.0.1" || e.Source != "driver" || e.Action != "disconnected" || e.Role != "server" {
		t.Errorf("event %+v", e)
	}
}

// An allowed client is identified on every association, and its identity
// is published in the statistics.
func TestSecEventPeerIdentity(t *testing.T) {
	conn := testConn()
	s := withSec(t, conn)
	gw := startGateway(t, conn, loopbackPoints())
	_ = dial(t, gw)
	e := waitEvent(t, s, "peer.identified")
	if e.PeerIP != "127.0.0.1" || e.SeverityLevel != 0 {
		t.Errorf("event %+v", e)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if d := s.statsDoc(); len(d["clients"].(bson.A)) == 1 && len(d["known"].(bson.A)) == 1 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Errorf("stats.peerIdentity = %v", s.statsDoc())
}

// A changed identity for a known address is reported; the stored one
// (from the statistics) is where detection starts.
func TestSecEventIdentityChangeFromStoredStats(t *testing.T) {
	conn := testConn()
	conn.KnownPeers = []bson.M{{"ip": "127.0.0.1", "acseIdentity": "1.3.9999.1"}}
	s := withSec(t, conn)
	gw := startGateway(t, conn, loopbackPoints())
	_ = dial(t, gw) // the library client states no calling AP title by default
	select {
	case e := <-s.writer.C():
		if e.Kind == "peer.identified" {
			t.Error("a known address reported as new")
		}
	case <-time.After(300 * time.Millisecond):
	}
	if _, ch := s.ids.Observe("127.0.0.1", "1.3.9999.2"); !ch {
		t.Error("identity change not detected against the stored identity")
	}
}

// Refused, flooding and test controls are reported.
func TestSecEventControls(t *testing.T) {
	conn := testConn()
	s := withSec(t, conn)
	built := BuildModel([]*Point{pt(4, "CMD1", "digital", "command", "T")}, conn)
	g := &Gateway{conn: conn, built: built}
	mp := built.ByTag["CMD1"]
	defer drainCommands()

	g.handleControl(mp, &server.ControlCtx{Ref: mp.ObjRef, Value: mms.NewBool(true), Test: true, Peer: "10.0.0.5:5000"})
	if e := waitEvent(t, s, "cmd.rejected"); e.Fields["test"] != true || e.ObjectRef != string(mp.ObjRef) {
		t.Errorf("event %+v", e)
	}
	for i := 0; i <= controlFloodMax; i++ {
		if c := g.handleControl(mp, &server.ControlCtx{Ref: mp.ObjRef, Value: mms.NewBool(true), Peer: "10.0.0.5:5000"}); c != model.AddCauseNone {
			t.Fatalf("control refused: %v", c)
		}
	}
	waitEvent(t, s, "flood.controls")
}

// A client certificate the pinning refuses is reported with the client's
// address.
func TestSecEventCertificateRejected(t *testing.T) {
	dir := t.TempDir()
	srvCert, srvKey := writeTestCert(t, dir)
	der, _ := x509.MarshalPKCS8PrivateKey(srvKey)
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	conn := testConn()
	conn.UseSecurity = true
	conn.LocalCertFilePath, conn.PrivateKeyFilePath = srvCert, keyPath
	conn.AllowOnlySpecificCertificates = true
	conn.PeerCertFilesPaths = []string{srvCert} // only the server's own: any client is refused
	s := withSec(t, conn)
	gw := startGateway(t, conn, loopbackPoints())

	other, otherKey := writeTestCert(t, t.TempDir())
	pemCert, _ := os.ReadFile(other)
	otherDER, _ := x509.MarshalPKCS8PrivateKey(otherKey)
	pair, err := tls.X509KeyPair(pemCert, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: otherDER}))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	//nolint:gosec // test client
	cfg := &tls.Config{Certificates: []tls.Certificate{pair}, InsecureSkipVerify: true}
	if c, err := client.Dial(ctx, gw.Addr(), client.WithTLS(cfg), client.WithTimeout(2*time.Second)); err == nil {
		_ = c.Close()
	}
	e := waitEvent(t, s, "tls.cert_rejected")
	if e.PeerIP != "127.0.0.1" || e.Error == "" || e.Peer["tls"] == nil {
		t.Errorf("event %+v", e)
	}
}
