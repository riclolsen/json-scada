package dnp3util

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// certSet is a CA and the two certificates a mutually authenticated DNP3
// connection needs.
type certSet struct {
	CAFile     string
	ServerCert string
	ServerKey  string
	ClientCert string
	ClientKey  string
	// OtherCAFile is a second, unrelated authority, used to check that peer
	// verification actually rejects a stranger.
	OtherCAFile     string
	OtherClientCert string
	OtherClientKey  string
}

// writeCerts generates a private CA and issues a server and a client
// certificate from it, plus a second CA with its own client certificate.
//
// The certificates carry 127.0.0.1 as an IP SAN: the library defaults a
// client's ServerName to the host it dialled, so a certificate without it is
// rejected by the client's own verification before the outstation is ever
// reached.
func writeCerts(t *testing.T) certSet {
	t.Helper()
	dir := t.TempDir()

	caCert, caKey := makeCA(t, "json-scada test CA")
	otherCert, otherKey := makeCA(t, "unrelated CA")

	set := certSet{
		CAFile:          filepath.Join(dir, "ca.crt"),
		ServerCert:      filepath.Join(dir, "server.crt"),
		ServerKey:       filepath.Join(dir, "server.key"),
		ClientCert:      filepath.Join(dir, "client.crt"),
		ClientKey:       filepath.Join(dir, "client.key"),
		OtherCAFile:     filepath.Join(dir, "other-ca.crt"),
		OtherClientCert: filepath.Join(dir, "other-client.crt"),
		OtherClientKey:  filepath.Join(dir, "other-client.key"),
	}

	writePEM(t, set.CAFile, "CERTIFICATE", caCert.Raw)
	writePEM(t, set.OtherCAFile, "CERTIFICATE", otherCert.Raw)

	issue(t, caCert, caKey, "outstation", set.ServerCert, set.ServerKey)
	issue(t, caCert, caKey, "master", set.ClientCert, set.ClientKey)
	issue(t, otherCert, otherKey, "stranger", set.OtherClientCert, set.OtherClientKey)

	return set
}

func makeCA(t *testing.T, name string) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating the CA key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating the CA certificate: %v", err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parsing the CA certificate: %v", err)
	}
	return cert, key
}

func issue(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, cn, certFile, keyFile string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating the key for %s: %v", cn, err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		// Both roles: a DNP3 TLS connection authenticates in both directions,
		// and the same certificate shape serves an active and a passive end.
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:    []string{"localhost"},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatalf("issuing the certificate for %s: %v", cn, err)
	}
	writePEM(t, certFile, "CERTIFICATE", der)

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshalling the key for %s: %v", cn, err)
	}
	writePEM(t, keyFile, "EC PRIVATE KEY", keyDER)
}

func writePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
