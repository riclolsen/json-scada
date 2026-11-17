package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dscsystems/go-iec61850/client"
	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
	"github.com/dscsystems/go-iec61850/server"
	"github.com/youmark/pkcs8"
)

// A test command must not reach the source device.
func TestTestControlIsRefused(t *testing.T) {
	conn := testConn()
	built := BuildModel([]*Point{pt(4, "CMD1", "digital", "command", "T")}, conn)
	g := &Gateway{conn: conn, built: built}
	drainCommands()

	mp := built.ByTag["CMD1"]
	cause := g.handleControl(mp, &server.ControlCtx{Ref: mp.ObjRef, Value: mms.NewBool(true), Test: true})
	if cause != model.AddCauseBlockedByMode {
		t.Errorf("test control answered %v, want blocked-by-mode", cause)
	}
	if _, ok := dequeueCommand(); ok {
		t.Error("a test control was queued as a real command")
	}
}

func TestVisibleString(t *testing.T) {
	cases := []struct {
		in   string
		max  int
		want string
	}{
		{"Tensão de barra", 32, "Tensao de barra"},
		{"Disjuntor\tAberto", 32, "Disjuntor Aberto"},
		{"Ação €", 32, "Acao ?"},
		{"ççççç", 3, "ccc"},
		{strings.Repeat("é", 40), 32, strings.Repeat("e", 32)},
	}
	for _, c := range cases {
		if got := visibleString(c.in, c.max); got != c.want {
			t.Errorf("visibleString(%q, %d) = %q, want %q", c.in, c.max, got, c.want)
		}
	}
}

// However many clients are allowed, the blocks of one family in a logical
// device's LLN0 stay within MaxRcbPerLLN0, including a logical device that
// holds both status and measurand points.
func TestReportControlsPerLLN0StayBounded(t *testing.T) {
	for _, clients := range []float64{1, 2, 3, 4, 10} {
		conn := testConn()
		conn.MaxClientConnections = clients
		var points []*Point
		for i := 0; i < 300; i++ {
			typ := "digital"
			if i%2 == 1 {
				typ = "analog"
			}
			points = append(points, pt(float64(i+1), "P"+itoa(i+1), typ, "supervised", "T"))
		}
		built := BuildModel(points, conn)
		for _, ld := range built.Model.Devices {
			var lln0 *model.LogicalNode
			for _, ln := range ld.Nodes {
				if ln.Name == "LLN0" {
					lln0 = ln
				}
			}
			buffered, unbuffered := 0, 0
			for _, rc := range lln0.ReportControls {
				if rc.Buffered {
					buffered += rc.RptEnabled
				} else {
					unbuffered += rc.RptEnabled
				}
			}
			if buffered > MaxRcbPerLLN0 || unbuffered > MaxRcbPerLLN0 {
				t.Errorf("%v clients: %s has %d buffered / %d unbuffered blocks, limit %d",
					clients, ld.Name, buffered, unbuffered, MaxRcbPerLLN0)
			}
		}
		if len(built.ByTag) != len(points) {
			t.Errorf("%v clients: %d points mapped, want %d", clients, len(built.ByTag), len(points))
		}
	}
}

// The listener can fail on its own (accept errors); the server must not go
// on reporting itself as serving, and Start must bring it back.
func TestLoopbackListenerIsRestartedAfterFailure(t *testing.T) {
	gw := startGateway(t, testConn(), loopbackPoints())
	_ = gw.listener.Close() // as an accept failure would end Serve

	deadline := time.Now().Add(5 * time.Second)
	for gw.Serving() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if gw.Serving() {
		t.Fatal("still reported as serving after the listener failed")
	}
	gw.Start()
	if !gw.Serving() {
		t.Fatal("did not restart")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := client.Dial(ctx, gw.Addr(), client.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("dial after restart: %v", err)
	}
	defer c.Close()
	if _, err := c.LogicalDevices(ctx); err != nil {
		t.Errorf("browse after restart: %v", err)
	}
}

// writeTestCert writes a self-signed certificate and returns its path and key.
func writeTestCert(t *testing.T, dir string) (string, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "cert.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	return path, key
}

// password is the passphrase of an encrypted private key, as in the C#
// driver; an unencrypted key ignores it.
func TestEncryptedPrivateKey(t *testing.T) {
	dir := t.TempDir()
	certPath, key := writeTestCert(t, dir)

	plainDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	encDER, err := pkcs8.MarshalPrivateKey(key, []byte("secret"), nil)
	if err != nil {
		t.Fatal(err)
	}
	ecDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	//nolint:staticcheck // legacy PEM encryption, still written by older tools
	legacy, err := x509.EncryptPEMBlock(rand.Reader, "EC PRIVATE KEY", ecDER, []byte("secret"), x509.PEMCipherAES256)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]*pem.Block{
		"plain":  {Type: "PRIVATE KEY", Bytes: plainDER},
		"pkcs8":  {Type: "ENCRYPTED PRIVATE KEY", Bytes: encDER},
		"legacy": legacy,
	}
	for name, block := range keys {
		path := filepath.Join(dir, name+".key")
		if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadKeyPair(certPath, path, "secret"); err != nil {
			t.Errorf("%s key with the right password: %v", name, err)
		}
		if name == "plain" {
			continue
		}
		if _, err := loadKeyPair(certPath, path, "wrong"); err == nil {
			t.Errorf("%s key loaded with a wrong password", name)
		}
	}
}

// Pinning with no certificate to pin to must not accept every client.
func TestPinningWithoutCertificatesIsRefused(t *testing.T) {
	dir := t.TempDir()
	certPath, key := writeTestCert(t, dir)
	der, _ := x509.MarshalPKCS8PrivateKey(key)
	keyPath := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	conn := testConn()
	conn.UseSecurity = true
	conn.LocalCertFilePath, conn.PrivateKeyFilePath = certPath, keyPath
	conn.AllowOnlySpecificCertificates = true
	conn.PeerCertFilesPaths = []string{"", " "}
	if _, err := buildTLS(conn); err == nil {
		t.Error("pinning with no certificate was accepted")
	}
	conn.PeerCertFilesPaths = []string{certPath}
	if _, err := buildTLS(conn); err != nil {
		t.Errorf("pinning with a certificate: %v", err)
	}
}
