/*
 * Shared {json:scada} driver support library, in Go.
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

// Package jstls holds the certificate, version and cipher plumbing shared by
// the drivers that secure their protocol connections (IEC 62351-3 and the
// equivalent for the other protocols).
//
// Scope: only the parts that were byte-identical across drivers. Deciding
// RootCAs vs ClientCAs, whether a client certificate is mandatory, and the
// wording of the verification errors stays in each driver, because those
// differ between a client and a server on purpose.
package jstls

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
)

// LoadCert reads a PEM or DER encoded certificate.
func LoadCert(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if block, _ := pem.Decode(data); block != nil {
		return x509.ParseCertificate(block.Bytes)
	}
	return x509.ParseCertificate(data)
}

// LoadCerts reads every certificate in a PEM bundle, or a single DER file.
func LoadCerts(path string) ([]*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []*x509.Certificate
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		c, err := x509.ParseCertificate(data)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, nil
}

// CertPool builds a pool from one CA certificate file.
func CertPool(path string) (*x509.CertPool, error) {
	ca, err := LoadCert(path)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	return pool, nil
}

// VersionWindow resolves a set of enabled-version flags to the min/max window
// Go wants: the lowest and highest version enabled.
//
// parity: the C# cascade this replaces could produce a minimum above its
// maximum for some flag combinations — see deviation D5 of the IEC 61850
// drivers. With none enabled the window is TLS 1.2..1.3. Go no longer
// negotiates TLS 1.0/1.1 by default, so a window that spans the boundary is
// raised to 1.2.
func VersionWindow(allow10, allow11, allow12, allow13 bool) (uint16, uint16) {
	var versions []uint16
	if allow10 {
		versions = append(versions, tls.VersionTLS10)
	}
	if allow11 {
		versions = append(versions, tls.VersionTLS11)
	}
	if allow12 {
		versions = append(versions, tls.VersionTLS12)
	}
	if allow13 {
		versions = append(versions, tls.VersionTLS13)
	}
	if len(versions) == 0 {
		return tls.VersionTLS12, tls.VersionTLS13
	}
	minV, maxV := versions[0], versions[len(versions)-1]
	if minV < tls.VersionTLS12 && maxV >= tls.VersionTLS12 {
		minV = tls.VersionTLS12
	}
	return minV, maxV
}

// VersionName renders a TLS version for a log line.
func VersionName(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS1.0"
	case tls.VersionTLS11:
		return "TLS1.1"
	case tls.VersionTLS12:
		return "TLS1.2"
	case tls.VersionTLS13:
		return "TLS1.3"
	}
	return "?"
}

// ParseCipherList maps IANA cipher suite names to their identifiers. Unknown
// names are ignored, with a log line. TLS 1.3 suites are fixed by the
// standard and cannot be selected here.
func ParseCipherList(list string) []uint16 {
	list = strings.TrimSpace(list)
	if list == "" {
		return nil
	}
	byName := map[string]uint16{}
	for _, s := range tls.CipherSuites() {
		byName[strings.ToUpper(s.Name)] = s.ID
	}
	for _, s := range tls.InsecureCipherSuites() {
		byName[strings.ToUpper(s.Name)] = s.ID
	}

	var out []uint16
	for _, name := range strings.FieldsFunc(list, func(r rune) bool {
		return r == ',' || r == ':' || r == ' '
	}) {
		key := strings.ToUpper(strings.TrimSpace(name))
		if id, ok := byName[key]; ok {
			out = append(out, id)
		} else {
			jslog.Log(jslog.LevelBasic, "Unknown cipher suite in cipherList: %s", name)
		}
	}
	return out
}

// LoadPinned reads the certificates a connection will accept, skipping blank
// path entries.
func LoadPinned(paths []string) ([]*x509.Certificate, error) {
	var pinned []*x509.Certificate
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		c, err := LoadCert(path)
		if err != nil {
			return nil, fmt.Errorf("peer certificate %s: %w", path, err)
		}
		pinned = append(pinned, c)
	}
	return pinned, nil
}

// PeerVerifier builds the VerifyPeerCertificate callback used when a
// connection pins certificates, validates the chain itself, or both.
//
// noPeerMsg and notAllowedMsg are the two error strings, which differ
// between the client ("peer certificate") and the server ("client
// certificate") drivers.
func PeerVerifier(pinned []*x509.Certificate, chainValidation bool,
	roots *x509.CertPool, noPeerMsg, notAllowedMsg string,
) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return fmt.Errorf("%s", noPeerMsg)
		}
		if len(pinned) > 0 {
			ok := false
			for _, p := range pinned {
				if bytes.Equal(p.Raw, rawCerts[0]) {
					ok = true
					break
				}
			}
			if !ok {
				return fmt.Errorf("%s", notAllowedMsg)
			}
		}
		if chainValidation {
			leaf, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return err
			}
			inter := x509.NewCertPool()
			for _, raw := range rawCerts[1:] {
				if c, err := x509.ParseCertificate(raw); err == nil {
					inter.AddCert(c)
				}
			}
			if _, err := leaf.Verify(x509.VerifyOptions{
				Roots:         roots,
				Intermediates: inter,
				KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
			}); err != nil {
				return err
			}
		}
		return nil
	}
}
