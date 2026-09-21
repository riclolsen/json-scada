/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

// Certificates and the application configuration file.
//
// The C# driver delegates all of this to the OPC Foundation stack: an XML
// application configuration, certificates kept in the OPC Foundation store
// layout, and a validator that can be told to accept anything. gopcua has
// none of that, so the equivalent is done here from plain files —
// deviations D1 to D4 in README.md.

package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/youmark/pkcs8"
	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// Defaults used when no application configuration file can be read, taken
// from conf-templates/Opc.Ua.DefaultClient.Config.xml so both drivers
// present the same identity to a server.
const (
	DefaultApplicationName = "JSON-SCADA OPC-UA Client"
	DefaultApplicationURI  = "urn:localhost:OPCUA:JSON_SCADA_OPCUAClient"
	DefaultProductURI      = "http://json-scada.org/UA/ClientDriver"
)

// keyPair is a certificate and its private key, as gopcua wants them: the
// certificate DER-encoded, the key parsed.
type keyPair struct {
	CertDER []byte
	Cert    *x509.Certificate
	Key     *rsa.PrivateKey
	AppURI  string // first URI SAN, what the server matches against ApplicationUri
}

// uaAppConfig is the subset of Opc.Ua.DefaultClient.Config.xml this driver
// understands. Everything else in that file belongs to the OPC Foundation
// stack and is ignored — deviation D1.
type uaAppConfig struct {
	ApplicationName string
	ApplicationURI  string
	ProductURI      string
}

// readUAConfigXML mines the application configuration file for the identity
// fields. A missing or unreadable file is not an error: the defaults above
// are used, exactly as the C# driver falls back to a built-in configuration.
func readUAConfigXML(path string) uaAppConfig {
	cfg := uaAppConfig{
		ApplicationName: DefaultApplicationName,
		ApplicationURI:  DefaultApplicationURI,
		ProductURI:      DefaultProductURI,
	}
	if path == "" {
		return cfg
	}

	// Same fallback chain as the C# driver when the configured path misses.
	candidates := []string{
		path,
		filepath.Join("..", "conf", "Opc.Ua.DefaultClient.Config.xml"),
		filepath.Join(string(filepath.Separator), "json-scada", "conf", "Opc.Ua.DefaultClient.Config.xml"),
	}
	found := ""
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			found = c
			break
		}
	}
	if found == "" {
		jslog.Log(jslog.LevelDetailed, "No application configuration file found, using built-in defaults.")
		return cfg
	}

	data, err := os.ReadFile(filepath.Clean(found))
	if err != nil {
		jslog.Log(jslog.LevelBasic, "WARN: cannot read %s - %v", found, err)
		return cfg
	}

	var doc struct {
		ApplicationName string `xml:"ApplicationName"`
		ApplicationURI  string `xml:"ApplicationUri"`
		ProductURI      string `xml:"ProductUri"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		jslog.Log(jslog.LevelBasic, "WARN: cannot parse %s - %v", found, err)
		return cfg
	}

	jslog.Log(jslog.LevelBasic, "Load config from %s", found)
	if s := strings.TrimSpace(doc.ApplicationName); s != "" {
		cfg.ApplicationName = s
	}
	if s := strings.TrimSpace(doc.ApplicationURI); s != "" {
		cfg.ApplicationURI = s
	}
	if s := strings.TrimSpace(doc.ProductURI); s != "" {
		cfg.ProductURI = s
	}
	jslog.Log(jslog.LevelDetailed, "Only ApplicationName/ApplicationUri/ProductUri are honoured from that file (deviation D1).")
	return cfg
}

// loadKeyPair reads a certificate and its private key from a PKCS#12 (.pfx,
// .p12) file or from PEM. For PEM, the key may live in the same file or in a
// sibling named like the certificate with _cert replaced by _key.
func loadKeyPair(path, passphrase string) (*keyPair, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".pfx", ".p12":
		return loadPKCS12(data, passphrase)
	default:
		return loadPEM(data, path, passphrase)
	}
}

func loadPKCS12(data []byte, passphrase string) (*keyPair, error) {
	key, cert, err := pkcs12.Decode(data, passphrase)
	if err != nil {
		return nil, fmt.Errorf("cannot decode PKCS#12: %v", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("PKCS#12 holds a %T private key, only RSA is usable with OPC UA", key)
	}
	return newKeyPair(cert, rsaKey), nil
}

func loadPEM(data []byte, path, passphrase string) (*keyPair, error) {
	cert, key, err := parsePEMBlocks(data, passphrase)
	if err != nil {
		return nil, err
	}
	if cert == nil {
		return nil, fmt.Errorf("no CERTIFICATE block in %s", path)
	}
	if key == nil {
		// Try the sibling key file produced by create_client_cert.ps1.
		sibling := siblingKeyPath(path)
		if sibling == "" {
			return nil, fmt.Errorf("no private key in %s and no sibling key file", path)
		}
		kdata, kerr := os.ReadFile(filepath.Clean(sibling))
		if kerr != nil {
			return nil, fmt.Errorf("no private key in %s and cannot read %s - %v", path, sibling, kerr)
		}
		_, key, err = parsePEMBlocks(kdata, passphrase)
		if err != nil {
			return nil, err
		}
		if key == nil {
			return nil, fmt.Errorf("no private key in %s", sibling)
		}
	}
	return newKeyPair(cert, key), nil
}

// parsePEMBlocks walks every PEM block, picking up the first certificate and
// the first RSA private key. Encrypted PKCS#8 keys are decrypted with the
// passphrase — that is the form create_client_cert.ps1 writes.
func parsePEMBlocks(data []byte, passphrase string) (*x509.Certificate, *rsa.PrivateKey, error) {
	var cert *x509.Certificate
	var key *rsa.PrivateKey

	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		switch block.Type {
		case "CERTIFICATE":
			if cert != nil {
				continue // first certificate wins; the rest is the chain
			}
			c, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, nil, fmt.Errorf("cannot parse certificate: %v", err)
			}
			cert = c

		case "RSA PRIVATE KEY":
			if key != nil {
				continue
			}
			k, err := x509.ParsePKCS1PrivateKey(block.Bytes)
			if err != nil {
				return nil, nil, fmt.Errorf("cannot parse RSA private key: %v", err)
			}
			key = k

		case "PRIVATE KEY":
			if key != nil {
				continue
			}
			k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				return nil, nil, fmt.Errorf("cannot parse private key: %v", err)
			}
			rk, ok := k.(*rsa.PrivateKey)
			if !ok {
				return nil, nil, fmt.Errorf("private key is %T, only RSA is usable with OPC UA", k)
			}
			key = rk

		case "ENCRYPTED PRIVATE KEY":
			if key != nil {
				continue
			}
			k, err := pkcs8.ParsePKCS8PrivateKey(block.Bytes, []byte(passphrase))
			if err != nil {
				return nil, nil, fmt.Errorf("cannot decrypt private key (wrong passphrase?): %v", err)
			}
			rk, ok := k.(*rsa.PrivateKey)
			if !ok {
				return nil, nil, fmt.Errorf("private key is %T, only RSA is usable with OPC UA", k)
			}
			key = rk
		}
	}
	return cert, key, nil
}

// siblingKeyPath maps ".../name_cert.pem" to ".../name_key.pem", and
// ".../name.pem" to ".../name_key.pem".
func siblingKeyPath(certPath string) string {
	dir := filepath.Dir(certPath)
	base := filepath.Base(certPath)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)

	var candidate string
	switch {
	case strings.HasSuffix(stem, "_cert"):
		candidate = strings.TrimSuffix(stem, "_cert") + "_key" + ext
	default:
		candidate = stem + "_key" + ext
	}
	p := filepath.Join(dir, candidate)
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

func newKeyPair(cert *x509.Certificate, key *rsa.PrivateKey) *keyPair {
	kp := &keyPair{CertDER: cert.Raw, Cert: cert, Key: key}
	if len(cert.URIs) > 0 {
		// This is what X509Utils.GetApplicationUriFromCertificate returns,
		// and what the server checks the session's ApplicationUri against.
		kp.AppURI = cert.URIs[0].String()
	}
	return kp
}

// ensureClientCert returns the application instance certificate, generating
// a self-signed one when none exists. The C# driver does this silently
// through CheckApplicationInstanceCertificates; here it is explicit and the
// files land next to the ones create_client_cert.ps1 writes.
func ensureClientCert(dir, appURI, appName string) (*keyPair, error) {
	certPath := filepath.Join(dir, "js_opcua_client_go_cert.pem")
	keyPath := filepath.Join(dir, "js_opcua_client_go_key.pem")

	if _, err := os.Stat(certPath); err == nil {
		if _, err := os.Stat(keyPath); err == nil {
			kp, lerr := loadKeyPair(certPath, "")
			if lerr == nil {
				jslog.Log(jslog.LevelBasic, "Using generated application certificate %s", certPath)
				return kp, nil
			}
			jslog.Log(jslog.LevelBasic, "WARN: cannot reuse %s - %v, generating a new one", certPath, lerr)
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	jslog.Log(jslog.LevelBasic, "Generating a self-signed application certificate in %s", dir)
	kp, certPEM, keyPEM, err := generateSelfSigned(appURI, appName)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(certPath, certPEM, 0o644); err != nil {
		return nil, err
	}
	// The key is secret: never world-readable.
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return nil, err
	}
	return kp, nil
}

// generateSelfSigned builds an OPC UA compliant client certificate: the
// application URI must appear as a URI SAN or servers reject the session
// with BadCertificateUriInvalid.
func generateSelfSigned(appURI, appName string) (*keyPair, []byte, []byte, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, err
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, nil, nil, err
	}

	host, _ := os.Hostname()
	notBefore := time.Now().Add(-1 * time.Hour)
	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: appName, Organization: []string{"JSON-SCADA"}},
		NotBefore:    notBefore,
		NotAfter:     notBefore.AddDate(10, 0, 0),
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment |
			x509.KeyUsageKeyEncipherment | x509.KeyUsageDataEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	if u, err := url.Parse(appURI); err == nil {
		template.URIs = append(template.URIs, u)
	}
	if host != "" {
		template.DNSNames = append(template.DNSNames, host)
	}
	template.DNSNames = append(template.DNSNames, "localhost")
	template.IPAddresses = append(template.IPAddresses, net.ParseIP("127.0.0.1"))

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return newKeyPair(cert, key), certPEM, keyPEM, nil
}

// validateServerCert checks a server certificate when the connection asks
// for it. autoAcceptUntrustedCertificates is gopcua's native behaviour, so
// only the false case does work here: the certificate must chain to a system
// root or to a PEM in conf/opcua/trusted — deviation D4.
func validateServerCert(der []byte, trustedDir, connName string) error {
	if len(der) == 0 {
		return fmt.Errorf("server presented no certificate")
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return fmt.Errorf("cannot parse server certificate: %v", err)
	}

	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}

	entries, err := os.ReadDir(trustedDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext != ".pem" && ext != ".crt" && ext != ".der" {
				continue
			}
			data, rerr := os.ReadFile(filepath.Join(trustedDir, e.Name()))
			if rerr != nil {
				continue
			}
			if !roots.AppendCertsFromPEM(data) {
				// Not PEM: try raw DER.
				if c, perr := x509.ParseCertificate(data); perr == nil {
					roots.AddCert(c)
				}
			}
		}
	}

	// A self-signed server certificate that is itself in the trust list
	// verifies against a pool containing it, which is the intent.
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:     roots,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return fmt.Errorf("server certificate '%s' is not trusted: %v", cert.Subject.String(), err)
	}
	jslog.Log(jslog.LevelDetailed, "%s - Accepted server certificate: %s", connName, cert.Subject.String())
	return nil
}
