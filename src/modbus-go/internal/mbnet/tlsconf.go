/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

// Package mbnet holds the transport plumbing around go-modbus: TLS
// configuration, the loopback bridge that lets the go-modbus client reach a
// device through a socket the driver opened itself (TLS with the connection's
// own options, a local bind address, RTU over TLS), and the server front-end
// that accepts masters, enforces the allow-list and client limit, terminates
// TLS and converts RTU framing for the go-modbus server.
package mbnet

import (
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/riclolsen/json-scada/src/go-common/jstls"
	"github.com/youmark/pkcs8"
)

// TLSOptions are the TLS fields of a protocolConnections document. The names
// mirror the Node.js driver (and the DNP3 drivers) for operator familiarity.
type TLSOptions struct {
	LocalCertFilePath             string
	PrivateKeyFilePath            string
	PrivateKeyPassphrase          string
	PeerCertFilePath              string
	ChainValidation               bool
	AllowOnlySpecificCertificates bool
	CipherList                    string
	AllowTLSv12                   bool
	AllowTLSv13                   bool
}

// versionWindow is the Node.js rule: TLS 1.0/1.1 are never offered.
func versionWindow(o TLSOptions) (uint16, uint16, error) {
	minV, maxV := uint16(tls.VersionTLS13), uint16(tls.VersionTLS12)
	if o.AllowTLSv12 {
		minV = tls.VersionTLS12
	}
	if o.AllowTLSv13 {
		maxV = tls.VersionTLS13
	}
	if minV > maxV {
		return 0, 0, errors.New("TLS: allowTLSv12 and allowTLSv13 are both false, no protocol version left")
	}
	return minV, maxV, nil
}

// openSSLNames maps OpenSSL cipher names (the form Node.js takes in
// cipherList) to the IANA names Go uses.
var openSSLNames = map[string]string{
	"ECDHE-ECDSA-AES128-GCM-SHA256": "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
	"ECDHE-RSA-AES128-GCM-SHA256":   "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
	"ECDHE-ECDSA-AES256-GCM-SHA384": "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",
	"ECDHE-RSA-AES256-GCM-SHA384":   "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
	"ECDHE-ECDSA-CHACHA20-POLY1305": "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256",
	"ECDHE-RSA-CHACHA20-POLY1305":   "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",
	"ECDHE-ECDSA-AES128-SHA256":     "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256",
	"ECDHE-RSA-AES128-SHA256":       "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256",
	"ECDHE-ECDSA-AES128-SHA":        "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA",
	"ECDHE-RSA-AES128-SHA":          "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",
	"ECDHE-ECDSA-AES256-SHA":        "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA",
	"ECDHE-RSA-AES256-SHA":          "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",
	"AES128-GCM-SHA256":             "TLS_RSA_WITH_AES_128_GCM_SHA256",
	"AES256-GCM-SHA384":             "TLS_RSA_WITH_AES_256_GCM_SHA384",
	"AES128-SHA256":                 "TLS_RSA_WITH_AES_128_CBC_SHA256",
	"AES128-SHA":                    "TLS_RSA_WITH_AES_128_CBC_SHA",
	"AES256-SHA":                    "TLS_RSA_WITH_AES_256_CBC_SHA",
	"DES-CBC3-SHA":                  "TLS_RSA_WITH_3DES_EDE_CBC_SHA",
	"ECDHE-RSA-DES-CBC3-SHA":        "TLS_ECDHE_RSA_WITH_3DES_EDE_CBC_SHA",
}

// CipherSuites resolves cipherList, which may use OpenSSL names (as the
// Node.js driver took them) or IANA names, separated by ':', ',' or spaces.
// TLS 1.3 suites are fixed by the standard and cannot be selected.
func CipherSuites(list string) []uint16 {
	list = strings.TrimSpace(list)
	if list == "" {
		return nil
	}
	var names []string
	for _, n := range strings.FieldsFunc(list, func(r rune) bool { return r == ':' || r == ',' || r == ' ' }) {
		n = strings.TrimSpace(n)
		if strings.HasPrefix(n, "!") || strings.HasPrefix(n, "-") || strings.HasPrefix(n, "+") ||
			strings.HasPrefix(n, "TLS_AES_") || strings.HasPrefix(n, "TLS_CHACHA20_") {
			continue // OpenSSL operators and TLS 1.3 suites: not selectable in Go
		}
		if iana, ok := openSSLNames[strings.ToUpper(n)]; ok {
			n = iana
		}
		names = append(names, n)
	}
	return jstls.ParseCipherList(strings.Join(names, ","))
}

// LoadKeyPair reads a certificate (chain) and its private key. The key may be
// PKCS#1, PKCS#8 or EC, encrypted (PKCS#8 or legacy PEM encryption) with the
// passphrase. When keyPath is empty the key is looked for in certPath.
func LoadKeyPair(certPath, keyPath, passphrase string) (tls.Certificate, error) {
	var out tls.Certificate
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return out, fmt.Errorf("certificate %s: %w", certPath, err)
	}
	keyPEM := certPEM
	if keyPath != "" {
		if keyPEM, err = os.ReadFile(keyPath); err != nil {
			return out, fmt.Errorf("private key %s: %w", keyPath, err)
		}
	}
	rest := certPEM
	for {
		var b *pem.Block
		b, rest = pem.Decode(rest)
		if b == nil {
			break
		}
		if b.Type == "CERTIFICATE" {
			out.Certificate = append(out.Certificate, b.Bytes)
		}
	}
	if len(out.Certificate) == 0 {
		// a DER certificate
		if c, err := x509.ParseCertificate(certPEM); err == nil {
			out.Certificate = [][]byte{c.Raw}
		} else {
			return out, fmt.Errorf("certificate %s: no certificate found", certPath)
		}
	}
	key, err := parseKey(keyPEM, passphrase)
	if err != nil {
		return out, fmt.Errorf("private key %s: %w", firstNonEmpty(keyPath, certPath), err)
	}
	out.PrivateKey = key
	if leaf, err := x509.ParseCertificate(out.Certificate[0]); err == nil {
		out.Leaf = leaf
	}
	return out, nil
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func parseKey(data []byte, passphrase string) (crypto.PrivateKey, error) {
	rest := data
	for {
		var b *pem.Block
		b, rest = pem.Decode(rest)
		if b == nil {
			break
		}
		der := b.Bytes
		switch {
		case b.Type == "ENCRYPTED PRIVATE KEY":
			return pkcs8.ParsePKCS8PrivateKey(der, []byte(passphrase))
		case strings.HasSuffix(b.Type, "PRIVATE KEY"):
			//lint:ignore SA1019 legacy PEM encryption is still what many tools write
			if x509.IsEncryptedPEMBlock(b) { //nolint:staticcheck
				var err error
				//lint:ignore SA1019 see above
				if der, err = x509.DecryptPEMBlock(b, []byte(passphrase)); err != nil { //nolint:staticcheck
					return nil, err
				}
			}
			return parseDERKey(der)
		}
	}
	return parseDERKey(data)
}

func parseDERKey(der []byte) (crypto.PrivateKey, error) {
	if k, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return k, nil
	}
	if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		return k, nil
	}
	if k, err := x509.ParseECPrivateKey(der); err == nil {
		return k, nil
	}
	return nil, errors.New("no usable private key found")
}

// ClientTLSConfig builds the client side, as the Node.js driver did:
//   - the local certificate is presented when configured (mutual TLS);
//   - peerCertFilePath is the trust anchor; without it the system roots are used;
//   - chainValidation false accepts any server certificate;
//   - allowOnlySpecificCertificates pins the certificates of peerCertFilePath
//     (exact match of the server's leaf) and skips the host name check.
func ClientTLSConfig(o TLSOptions, serverName string) (*tls.Config, error) {
	minV, maxV, err := versionWindow(o)
	if err != nil {
		return nil, err
	}
	cfg := &tls.Config{
		MinVersion:   minV,
		MaxVersion:   maxV,
		CipherSuites: CipherSuites(o.CipherList),
		ServerName:   serverName,
	}
	if o.LocalCertFilePath != "" {
		kp, err := LoadKeyPair(o.LocalCertFilePath, o.PrivateKeyFilePath, o.PrivateKeyPassphrase)
		if err != nil {
			return nil, err
		}
		cfg.Certificates = []tls.Certificate{kp}
	}
	var roots *x509.CertPool
	var peers []*x509.Certificate
	if o.PeerCertFilePath != "" {
		peers, err = jstls.LoadCerts(o.PeerCertFilePath)
		if err != nil {
			return nil, fmt.Errorf("peer certificate %s: %w", o.PeerCertFilePath, err)
		}
		roots = x509.NewCertPool()
		for _, c := range peers {
			roots.AddCert(c)
		}
		cfg.RootCAs = roots
	}
	switch {
	case o.AllowOnlySpecificCertificates && len(peers) > 0:
		cfg.InsecureSkipVerify = true // verification is done by the pinning callback
		cfg.VerifyPeerCertificate = jstls.PeerVerifier(peers, o.ChainValidation, roots,
			"TLS peer sent no certificate", "TLS peer certificate is not one of the allowed certificates")
	case !o.ChainValidation:
		cfg.InsecureSkipVerify = true
	}
	return cfg, nil
}

// ServerTLSConfig builds the server side, as the Node.js driver did:
// chainValidation true requires and verifies a client certificate against
// peerCertFilePath (or the system roots); false requests none.
// allowOnlySpecificCertificates additionally pins the client certificates.
func ServerTLSConfig(o TLSOptions) (*tls.Config, error) {
	minV, maxV, err := versionWindow(o)
	if err != nil {
		return nil, err
	}
	if o.LocalCertFilePath == "" {
		return nil, errors.New("TLS server mode needs localCertFilePath (and privateKeyFilePath)")
	}
	kp, err := LoadKeyPair(o.LocalCertFilePath, o.PrivateKeyFilePath, o.PrivateKeyPassphrase)
	if err != nil {
		return nil, err
	}
	cfg := &tls.Config{
		MinVersion:   minV,
		MaxVersion:   maxV,
		CipherSuites: CipherSuites(o.CipherList),
		Certificates: []tls.Certificate{kp},
		ClientAuth:   tls.NoClientCert,
	}
	var roots *x509.CertPool
	var peers []*x509.Certificate
	if o.PeerCertFilePath != "" {
		if peers, err = jstls.LoadCerts(o.PeerCertFilePath); err != nil {
			return nil, fmt.Errorf("peer certificate %s: %w", o.PeerCertFilePath, err)
		}
		roots = x509.NewCertPool()
		for _, c := range peers {
			roots.AddCert(c)
		}
		cfg.ClientCAs = roots
	}
	switch {
	case o.AllowOnlySpecificCertificates && len(peers) > 0:
		cfg.ClientAuth = tls.RequireAnyClientCert
		cfg.VerifyPeerCertificate = jstls.PeerVerifier(peers, o.ChainValidation, roots,
			"no client certificate received", "client certificate is not one of the allowed certificates")
	case o.ChainValidation:
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return cfg, nil
}
