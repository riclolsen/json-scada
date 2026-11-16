/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package tlsutil

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"iec60870-5/internal/model"
)

type testCert struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
	der  []byte
}

func newCert(t *testing.T, cn string, parent *testCert, isCA bool, usage []x509.ExtKeyUsage) *testCert {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           usage,
		BasicConstraintsValid: true,
		IsCA:                  isCA,
	}
	signer, signerKey := tmpl, key
	if parent != nil {
		signer, signerKey = parent.cert, parent.key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, signer, &key.PublicKey, signerKey)
	if err != nil {
		t.Fatal(err)
	}
	c, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &testCert{cert: c, key: key, der: der}
}

// writePEM writes the certificate (and key, when withKey) to a file.
func writePEM(t *testing.T, c *testCert, withKey bool) string {
	t.Helper()
	out := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.der})
	if withKey {
		kb, err := x509.MarshalECPrivateKey(c.key)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb})...)
	}
	p := filepath.Join(t.TempDir(), c.cert.Subject.CommonName+".pem")
	if err := os.WriteFile(p, out, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestServerVerifiesClientAuthCertificate(t *testing.T) {
	ca := newCert(t, "ca", nil, true, nil)
	srv := newCert(t, "server", ca, false, []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	cli := newCert(t, "client", ca, false, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})

	cfg := &model.ConnCfg{
		LocalCertFilePath: writePEM(t, srv, true),
		RootCertFilePath:  writePEM(t, ca, false),
		ChainValidation:   true,
	}
	tc, err := BuildTLSConfig(cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	// a client certificate with only the clientAuth usage is accepted
	if err := tc.VerifyPeerCertificate([][]byte{cli.der}, nil); err != nil {
		t.Errorf("client certificate rejected: %v", err)
	}
	// a certificate from another CA is not
	other := newCert(t, "other", newCert(t, "otherca", nil, true, nil), false,
		[]x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	if err := tc.VerifyPeerCertificate([][]byte{other.der}, nil); err == nil {
		t.Error("certificate from an unknown CA accepted")
	}
}

// chainValidation without rootCertFilePath verifies against the system
// roots instead of accepting anything.
func TestChainValidationWithoutRootFailsClosed(t *testing.T) {
	ca := newCert(t, "ca", nil, true, nil)
	cli := newCert(t, "client", ca, false, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	peer := newCert(t, "peer", newCert(t, "peerca", nil, true, nil), false,
		[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})

	tc, err := BuildTLSConfig(&model.ConnCfg{
		LocalCertFilePath: writePEM(t, cli, true),
		ChainValidation:   true,
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := tc.VerifyPeerCertificate([][]byte{peer.der}, nil); err == nil {
		t.Error("self-issued server certificate accepted without a root")
	}

	// chainValidation false keeps accepting any certificate
	tc, err = BuildTLSConfig(&model.ConnCfg{LocalCertFilePath: writePEM(t, cli, true)}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := tc.VerifyPeerCertificate([][]byte{peer.der}, nil); err != nil {
		t.Errorf("chainValidation=false rejected: %v", err)
	}
}

func TestAllowOnlySpecificCertificates(t *testing.T) {
	ca := newCert(t, "ca", nil, true, nil)
	own := newCert(t, "own", ca, false, nil)
	allowed := newCert(t, "allowed", ca, false, nil)
	denied := newCert(t, "denied", ca, false, nil)
	tc, err := BuildTLSConfig(&model.ConnCfg{
		LocalCertFilePath:             writePEM(t, own, true),
		PeerCertFilesPaths:            []string{writePEM(t, allowed, false)},
		AllowOnlySpecificCertificates: true,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := tc.VerifyPeerCertificate([][]byte{allowed.der}, nil); err != nil {
		t.Errorf("listed certificate rejected: %v", err)
	}
	if err := tc.VerifyPeerCertificate([][]byte{denied.der}, nil); err == nil {
		t.Error("unlisted certificate accepted")
	}
}
