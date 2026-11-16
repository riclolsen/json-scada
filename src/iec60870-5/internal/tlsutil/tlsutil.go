/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - TLS helpers
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Builds *tls.Config from the protocolConnections certificate fields
 * (localCertFilePath/passphrase/peerCertFilesPaths/rootCertFilePath/
 * chainValidation/allowOnlySpecificCertificates), accepting PFX (as the
 * C# drivers) and PEM certificate files.
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

package tlsutil

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"strings"

	pkcs12 "software.sslmate.com/src/go-pkcs12"

	"iec60870-5/internal/model"
)

func loadOwnCertificate(path, passphrase string) (tls.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return tls.Certificate{}, err
	}
	if strings.HasSuffix(strings.ToLower(path), ".pfx") ||
		strings.HasSuffix(strings.ToLower(path), ".p12") {
		key, cert, caCerts, err := pkcs12.DecodeChain(data, passphrase)
		if err != nil {
			return tls.Certificate{}, err
		}
		tc := tls.Certificate{PrivateKey: key, Certificate: [][]byte{cert.Raw}, Leaf: cert}
		for _, ca := range caCerts {
			tc.Certificate = append(tc.Certificate, ca.Raw)
		}
		return tc, nil
	}
	// PEM: certificate and key expected in the same file
	return tls.X509KeyPair(data, data)
}

func loadPeerCertsDER(paths []string) ([][]byte, error) {
	var ders [][]byte
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		certs, err := parseCerts(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %v", p, err)
		}
		for _, c := range certs {
			ders = append(ders, c.Raw)
		}
	}
	return ders, nil
}

func parseCerts(data []byte) ([]*x509.Certificate, error) {
	if bytes.Contains(data, []byte("-----BEGIN")) {
		var certs []*x509.Certificate
		for {
			var blk *pem.Block
			blk, data = pem.Decode(data)
			if blk == nil {
				break
			}
			if blk.Type != "CERTIFICATE" {
				continue
			}
			c, err := x509.ParseCertificate(blk.Bytes)
			if err != nil {
				return nil, err
			}
			certs = append(certs, c)
		}
		return certs, nil
	}
	c, err := x509.ParseCertificate(data)
	if err != nil {
		return nil, err
	}
	return []*x509.Certificate{c}, nil
}

// BuildTLSConfig creates a *tls.Config for the given connection config;
// returns nil when no local certificate is configured (plain TCP).
// isServer selects server-side (client cert verification) semantics.
func BuildTLSConfig(cfg *model.ConnCfg, isServer bool) (*tls.Config, error) {
	if cfg.LocalCertFilePath == "" {
		return nil, nil
	}
	ownCert, err := loadOwnCertificate(cfg.LocalCertFilePath, cfg.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("error loading local certificate: %v", err)
	}
	tc := &tls.Config{
		Certificates: []tls.Certificate{ownCert},
		MinVersion:   tls.VersionTLS12,
	}

	var rootPool *x509.CertPool
	if cfg.RootCertFilePath != "" {
		data, err := os.ReadFile(cfg.RootCertFilePath)
		if err != nil {
			return nil, fmt.Errorf("error loading root certificate: %v", err)
		}
		rootPool = x509.NewCertPool()
		if !rootPool.AppendCertsFromPEM(data) {
			// try DER
			c, derr := x509.ParseCertificate(data)
			if derr != nil {
				return nil, fmt.Errorf("error parsing root certificate: %v", derr)
			}
			rootPool.AddCert(c)
		}
	}

	allowedDER, err := loadPeerCertsDER(cfg.PeerCertFilesPaths)
	if err != nil {
		return nil, fmt.Errorf("error loading peer certificates: %v", err)
	}

	peerUsage := x509.ExtKeyUsageServerAuth
	if isServer {
		peerUsage = x509.ExtKeyUsageClientAuth
	}
	verifyPeer := func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return fmt.Errorf("no peer certificate")
		}
		if cfg.AllowOnlySpecificCertificates {
			for _, der := range allowedDER {
				if bytes.Equal(der, rawCerts[0]) {
					return nil
				}
			}
			return fmt.Errorf("peer certificate not in the allowed list")
		}
		if cfg.ChainValidation {
			cert, err := x509.ParseCertificate(rawCerts[0])
			if err != nil {
				return err
			}
			inter := x509.NewCertPool()
			for _, der := range rawCerts[1:] {
				if ic, err := x509.ParseCertificate(der); err == nil {
					inter.AddCert(ic)
				}
			}
			// a server verifies a client certificate, a client a server
			// one; without rootCertFilePath (rootPool nil) the system
			// roots are used
			_, err = cert.Verify(x509.VerifyOptions{
				Roots:         rootPool,
				Intermediates: inter,
				KeyUsages:     []x509.ExtKeyUsage{peerUsage},
			})
			return err
		}
		return nil // chainValidation false: accept (C# parity)
	}

	if isServer {
		tc.ClientAuth = tls.RequireAnyClientCert
		tc.VerifyPeerCertificate = verifyPeer
	} else {
		// client side: we do our own verification (hostname checks are not
		// performed by the C# drivers either)
		tc.InsecureSkipVerify = true
		tc.VerifyPeerCertificate = verifyPeer
	}
	return tc, nil
}
