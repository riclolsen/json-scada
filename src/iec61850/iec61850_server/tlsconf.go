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
	"fmt"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jstls"
)

// buildTLS assembles the server TLS configuration of a connection.
func buildTLS(conn *ServerConnection) (*tls.Config, error) {
	cfg := &tls.Config{}

	if conn.LocalCertFilePath == "" || conn.PrivateKeyFilePath == "" {
		return nil, fmt.Errorf("useSecurity requires localCertFilePath and privateKeyFilePath")
	}
	cert, err := tls.LoadX509KeyPair(conn.LocalCertFilePath, conn.PrivateKeyFilePath)
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
