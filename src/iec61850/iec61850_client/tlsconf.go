/*
 * IEC 61850 MMS Client driver for {json:scada}, in Go.
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
// the same connection parameters the C# driver passes to libiec61850's
// TLSConfiguration. The certificate, version and cipher plumbing is shared
// with the server driver through go-common/jstls; what stays here is the
// client side of the policy.

package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jstls"
)

// buildTLS assembles the client TLS configuration of a connection.
func buildTLS(conn *Iec61850Connection) (*tls.Config, error) {
	cfg := &tls.Config{}

	if conn.LocalCertFilePath != "" && conn.PrivateKeyFilePath != "" {
		cert, err := tls.LoadX509KeyPair(conn.LocalCertFilePath, conn.PrivateKeyFilePath)
		if err != nil {
			return nil, fmt.Errorf("own certificate: %w", err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	if conn.RootCertFilePath != "" {
		pool, err := jstls.CertPool(conn.RootCertFilePath)
		if err != nil {
			return nil, fmt.Errorf("CA certificate: %w", err)
		}
		cfg.RootCAs = pool
	}

	var pinned []*x509.Certificate
	if conn.AllowOnlySpecificCertificates {
		var err error
		if pinned, err = jstls.LoadPinned(conn.PeerCertFilesPaths); err != nil {
			return nil, err
		}
	}

	// Go's own verification is bypassed whenever the connection does not
	// ask for chain validation or pins specific certificates; the checks
	// that are wanted then run in VerifyPeerCertificate.
	if !conn.ChainValidation || len(pinned) > 0 {
		cfg.InsecureSkipVerify = true
	}
	if len(pinned) > 0 || conn.ChainValidation {
		cfg.VerifyPeerCertificate = jstls.PeerVerifier(
			pinned, conn.ChainValidation, cfg.RootCAs,
			"no peer certificate presented",
			"peer certificate is not in the allowed list")
	}

	minV, maxV := jstls.VersionWindow(
		conn.AllowTLSv10, conn.AllowTLSv11, conn.AllowTLSv12, conn.AllowTLSv13)
	cfg.MinVersion, cfg.MaxVersion = minV, maxV
	jslog.Log(jslog.LevelBasic, "%s TLS versions %s..%s",
		conn.Name, jstls.VersionName(minV), jstls.VersionName(maxV))
	if (conn.AllowTLSv10 || conn.AllowTLSv11) && minV >= tls.VersionTLS12 {
		jslog.Log(jslog.LevelBasic,
			"%s TLS 1.0/1.1 requested but not available in this build; minimum is %s",
			conn.Name, jstls.VersionName(minV))
	}

	if suites := jstls.ParseCipherList(conn.CipherList); len(suites) > 0 {
		cfg.CipherSuites = suites
	}

	return cfg, nil
}
