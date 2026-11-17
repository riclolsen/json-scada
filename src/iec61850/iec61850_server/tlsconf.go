/*
 * IEC 61850 MMS Server driver (IEC61850-90-2 gateway) for {json:scada}, in Go.
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

// TLS configuration for secured MMS associations (IEC 62351-3), built from
// the same connection parameters the C# driver passes to libiec61850. The
// certificate, version and cipher plumbing is shared with the client driver
// through go-common/jstls; what stays here is the server side of the policy:
// a local certificate is mandatory, the CA pool verifies clients rather than
// peers, and a pinning or chain-validating connection demands a client
// certificate.

package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/youmark/pkcs8"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jstls"
)

// buildTLS assembles the server TLS configuration of a connection.
func buildTLS(conn *ServerConnection) (*tls.Config, error) {
	cfg := &tls.Config{}

	if conn.LocalCertFilePath == "" || conn.PrivateKeyFilePath == "" {
		return nil, fmt.Errorf("useSecurity requires localCertFilePath and privateKeyFilePath")
	}
	cert, err := loadKeyPair(conn.LocalCertFilePath, conn.PrivateKeyFilePath, conn.Password)
	if err != nil {
		return nil, fmt.Errorf("own certificate: %w", err)
	}
	cfg.Certificates = []tls.Certificate{cert}

	if conn.RootCertFilePath != "" {
		pool, err := jstls.CertPool(conn.RootCertFilePath)
		if err != nil {
			return nil, fmt.Errorf("CA certificate: %w", err)
		}
		cfg.ClientCAs = pool
	}

	var pinned []*x509.Certificate
	if conn.AllowOnlySpecificCertificates {
		if pinned, err = jstls.LoadPinned(conn.PeerCertFilesPaths); err != nil {
			return nil, err
		}
		// Pinning with nothing to pin to would otherwise switch every
		// check off and accept any peer: refuse the configuration instead.
		if len(pinned) == 0 {
			return nil, fmt.Errorf("allowOnlySpecificCertificates is set but peerCertFilesPaths lists no certificate")
		}
	}

	if conn.ChainValidation || len(pinned) > 0 {
		// Ask for a client certificate and check it ourselves, so pinning
		// and chain validation can be combined the way the connection asks.
		cfg.ClientAuth = tls.RequireAnyClientCert
		cfg.VerifyPeerCertificate = jstls.PeerVerifier(
			pinned, conn.ChainValidation, cfg.ClientCAs,
			"no client certificate presented",
			"client certificate is not in the allowed list")
	}

	minV, maxV := jstls.VersionWindow(
		conn.AllowTLSv10, conn.AllowTLSv11, conn.AllowTLSv12, conn.AllowTLSv13)
	cfg.MinVersion, cfg.MaxVersion = minV, maxV
	jslog.Log(jslog.LevelBasic, "TLS enabled (IEC 62351-3), versions %s..%s",
		jstls.VersionName(minV), jstls.VersionName(maxV))
	if (conn.AllowTLSv10 || conn.AllowTLSv11) && minV >= tls.VersionTLS12 {
		jslog.Log(jslog.LevelBasic,
			"TLS 1.0/1.1 requested but not available in this build; minimum is %s",
			jstls.VersionName(minV))
	}

	if suites := jstls.ParseCipherList(conn.CipherList); len(suites) > 0 {
		cfg.CipherSuites = suites
	}
	return cfg, nil
}

// loadKeyPair loads the server's certificate and private key. The key may be
// encrypted, with password as its passphrase — what the C# driver passed to
// libiec61850's TLSConfiguration.SetOwnKey. Both encrypted PKCS#8 ("ENCRYPTED
// PRIVATE KEY", OpenSSL's default) and legacy PEM encryption (Proc-Type:
// 4,ENCRYPTED) are read. An unencrypted key is read as is, whatever the
// password.
func loadKeyPair(certPath, keyPath, password string) (tls.Certificate, error) {
	if password == "" {
		return tls.LoadX509KeyPair(certPath, keyPath)
	}
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return tls.Certificate{}, err
	}
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return tls.Certificate{}, fmt.Errorf("%s: no PEM private key", keyPath)
	}

	var der []byte
	switch {
	case block.Type == "ENCRYPTED PRIVATE KEY":
		key, err := pkcs8.ParsePKCS8PrivateKey(block.Bytes, []byte(password))
		if err != nil {
			return tls.Certificate{}, fmt.Errorf("%s: decrypting the private key: %w", keyPath, err)
		}
		if der, err = x509.MarshalPKCS8PrivateKey(key); err != nil {
			return tls.Certificate{}, err
		}
		block = &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	//nolint:staticcheck // legacy PEM encryption is what older tools write
	case x509.IsEncryptedPEMBlock(block):
		//nolint:staticcheck
		if der, err = x509.DecryptPEMBlock(block, []byte(password)); err != nil {
			return tls.Certificate{}, fmt.Errorf("%s: decrypting the private key: %w", keyPath, err)
		}
		block = &pem.Block{Type: block.Type, Bytes: der}
	}
	return tls.X509KeyPair(certPEM, pem.EncodeToMemory(block))
}
