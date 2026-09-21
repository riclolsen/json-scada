/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 */

package main

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/youmark/pkcs8"
	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// newTestCert makes a certificate carrying an application URI SAN, the
// shape an OPC UA client certificate must have.
func newTestCert(t *testing.T) *keyPair {
	t.Helper()
	kp, _, _, err := generateSelfSigned("urn:test:opcua:client", "Test Client")
	if err != nil {
		t.Fatalf("generateSelfSigned: %v", err)
	}
	return kp
}

func TestGenerateSelfSignedCarriesAppURI(t *testing.T) {
	kp := newTestCert(t)
	if kp.AppURI != "urn:test:opcua:client" {
		t.Fatalf("AppURI = %q, want the URI SAN back", kp.AppURI)
	}
	if kp.Key == nil || kp.Cert == nil || len(kp.CertDER) == 0 {
		t.Fatal("incomplete key pair")
	}
	// A server rejects the session unless the certificate can sign and
	// encrypt; a missing key usage bit is silent until then.
	if kp.Cert.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		t.Error("certificate cannot sign")
	}
	if kp.Cert.KeyUsage&x509.KeyUsageDataEncipherment == 0 {
		t.Error("certificate cannot encrypt data")
	}
}

func TestEnsureClientCertGeneratesThenReuses(t *testing.T) {
	dir := t.TempDir()

	first, err := ensureClientCert(dir, "urn:test:opcua:client", "Test Client")
	if err != nil {
		t.Fatalf("first ensureClientCert: %v", err)
	}
	second, err := ensureClientCert(dir, "urn:test:opcua:client", "Test Client")
	if err != nil {
		t.Fatalf("second ensureClientCert: %v", err)
	}
	if first.Cert.SerialNumber.Cmp(second.Cert.SerialNumber) != 0 {
		t.Fatal("certificate was regenerated instead of reused; a new certificate " +
			"every restart means the server has to re-trust the client every restart")
	}

	// The private key must not be world readable. Windows does not apply
	// Unix permission bits, so this can only be checked elsewhere.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, "js_opcua_client_go_key.pem"))
		if err != nil {
			t.Fatalf("stat key: %v", err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Errorf("key file mode %v is too permissive", info.Mode().Perm())
		}
	}
}

func TestLoadKeyPairPKCS12(t *testing.T) {
	kp := newTestCert(t)
	pfx, err := pkcs12.Modern.Encode(kp.Key, kp.Cert, nil, "s3cret")
	if err != nil {
		t.Fatalf("encode pkcs12: %v", err)
	}
	path := filepath.Join(t.TempDir(), "client.pfx")
	if err := os.WriteFile(path, pfx, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := loadKeyPair(path, "s3cret")
	if err != nil {
		t.Fatalf("loadKeyPair: %v", err)
	}
	if got.AppURI != kp.AppURI {
		t.Errorf("AppURI = %q, want %q", got.AppURI, kp.AppURI)
	}
	if !got.Key.Equal(kp.Key) {
		t.Error("private key did not round-trip")
	}

	if _, err := loadKeyPair(path, "wrong"); err == nil {
		t.Error("a wrong passphrase must fail, not silently return a broken key")
	}
}

func TestLoadKeyPairPEMCombined(t *testing.T) {
	kp := newTestCert(t)
	buf := append(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: kp.CertDER}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(kp.Key)})...,
	)
	path := filepath.Join(t.TempDir(), "client.pem")
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := loadKeyPair(path, "")
	if err != nil {
		t.Fatalf("loadKeyPair: %v", err)
	}
	if !got.Key.Equal(kp.Key) {
		t.Error("private key did not round-trip")
	}
}

// The certificate and key may live in separate files, which is what
// platform-windows/create_client_cert.ps1 writes.
func TestLoadKeyPairPEMSiblingKeyFile(t *testing.T) {
	kp := newTestCert(t)
	dir := t.TempDir()
	certPath := filepath.Join(dir, "js_opcua_client_cert.pem")
	keyPath := filepath.Join(dir, "js_opcua_client_key.pem")

	if err := os.WriteFile(certPath,
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: kp.CertDER}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath,
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(kp.Key)}), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := loadKeyPair(certPath, "")
	if err != nil {
		t.Fatalf("loadKeyPair: %v", err)
	}
	if !got.Key.Equal(kp.Key) {
		t.Error("private key did not round-trip")
	}
}

// conf/opcua/js_opcua_client.pem in this repository holds an ENCRYPTED
// PRIVATE KEY block; the Go standard library cannot read those.
func TestLoadKeyPairPEMEncryptedPKCS8(t *testing.T) {
	kp := newTestCert(t)
	encDER, err := pkcs8.MarshalPrivateKey(kp.Key, []byte("s3cret"), nil)
	if err != nil {
		t.Fatalf("marshal encrypted pkcs8: %v", err)
	}
	buf := append(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: kp.CertDER}),
		pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: encDER})...,
	)
	path := filepath.Join(t.TempDir(), "client.pem")
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := loadKeyPair(path, "s3cret")
	if err != nil {
		t.Fatalf("loadKeyPair: %v", err)
	}
	if !got.Key.Equal(kp.Key) {
		t.Error("private key did not round-trip")
	}

	if _, err := loadKeyPair(path, "wrong"); err == nil {
		t.Error("a wrong passphrase must fail")
	}
}

func TestReadUAConfigXML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Opc.Ua.DefaultClient.Config.xml")
	xmlDoc := `<?xml version="1.0" encoding="utf-8"?>
<ApplicationConfiguration xmlns="http://opcfoundation.org/UA/SDK/Configuration.xsd">
  <ApplicationName>My Client</ApplicationName>
  <ApplicationUri>urn:example:MyClient</ApplicationUri>
  <ProductUri>http://example.org/UA/Client</ProductUri>
  <SecurityConfiguration><ApplicationCertificate><StoreType>Directory</StoreType></ApplicationCertificate></SecurityConfiguration>
</ApplicationConfiguration>`
	if err := os.WriteFile(path, []byte(xmlDoc), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := readUAConfigXML(path)
	if cfg.ApplicationName != "My Client" {
		t.Errorf("ApplicationName = %q", cfg.ApplicationName)
	}
	if cfg.ApplicationURI != "urn:example:MyClient" {
		t.Errorf("ApplicationURI = %q", cfg.ApplicationURI)
	}
	if cfg.ProductURI != "http://example.org/UA/Client" {
		t.Errorf("ProductURI = %q", cfg.ProductURI)
	}
}

// A missing file must not stop the driver: the C# stack falls back to a
// built-in configuration and so does this.
func TestReadUAConfigXMLMissingFileUsesDefaults(t *testing.T) {
	cfg := readUAConfigXML(filepath.Join(t.TempDir(), "nope.xml"))
	if cfg.ApplicationName != DefaultApplicationName || cfg.ApplicationURI != DefaultApplicationURI {
		t.Errorf("got %+v, want the built-in defaults", cfg)
	}
}

func TestValidateServerCert(t *testing.T) {
	kp := newTestCert(t)
	trusted := t.TempDir()

	if err := validateServerCert(kp.CertDER, trusted, "T"); err == nil {
		t.Error("an untrusted certificate must be refused when autoAccept is off")
	}

	if err := os.WriteFile(filepath.Join(trusted, "server.pem"),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: kp.CertDER}), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateServerCert(kp.CertDER, trusted, "T"); err != nil {
		t.Errorf("a certificate in the trust list must be accepted: %v", err)
	}

	if err := validateServerCert(nil, trusted, "T"); err == nil {
		t.Error("an absent server certificate must be refused")
	}
}

func TestPolicyShortName(t *testing.T) {
	cases := map[string]string{
		"http://opcfoundation.org/UA/SecurityPolicy#Basic256Sha256": "Basic256Sha256",
		"http://opcfoundation.org/UA/SecurityPolicy#None":           "None",
		"None": "None",
		"":     "",
	}
	for in, want := range cases {
		if got := policyShortName(in); got != want {
			t.Errorf("policyShortName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSiblingKeyPath(t *testing.T) {
	dir := t.TempDir()
	mk := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	mk("a_key.pem")
	if got := siblingKeyPath(filepath.Join(dir, "a_cert.pem")); got != filepath.Join(dir, "a_key.pem") {
		t.Errorf("_cert.pem should map to _key.pem, got %q", got)
	}
	mk("b_key.pem")
	if got := siblingKeyPath(filepath.Join(dir, "b.pem")); got != filepath.Join(dir, "b_key.pem") {
		t.Errorf("name.pem should map to name_key.pem, got %q", got)
	}
	if got := siblingKeyPath(filepath.Join(dir, "missing.pem")); got != "" {
		t.Errorf("a nonexistent sibling must yield \"\", got %q", got)
	}
}

// Guard against the encoder being swapped for one the OPC UA world cannot
// read: the driver must keep reading what it writes.
func TestPKCS12RoundTripUsesRSA(t *testing.T) {
	kp := newTestCert(t)
	pfx, err := pkcs12.Encode(rand.Reader, kp.Key, kp.Cert, nil, "pw")
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := loadPKCS12(pfx, "pw")
	if err != nil {
		t.Fatalf("loadPKCS12: %v", err)
	}
	if !got.Key.Equal(kp.Key) {
		t.Error("key mismatch")
	}
}
