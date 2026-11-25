/*
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

package jssecevents

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Throttle lets an event of a key through at most once per Every; the
// occurrences held back in between are counted and handed to the next one
// let through, so a repeating condition (a noisy line, a scanning peer)
// gives one event per period with its count.
type Throttle struct {
	Every time.Duration

	mu    sync.Mutex
	last  map[string]*throttleState
	swept time.Time
}

type throttleState struct {
	at   time.Time
	held int
}

// maxThrottleKeys bounds the state. When it is full the expired keys are
// dropped (at most once per sweepEvery, so the work per call stays bounded),
// and while it stays full the new keys share overflowKey.
const (
	maxThrottleKeys = 4096
	sweepEvery      = time.Second
	overflowKey     = "\x00overflow"
)

// Allow reports whether the event of key goes out now and, when it does, how
// many occurrences it stands for (itself and the ones held back before it).
func (t *Throttle) Allow(key string, now time.Time) (bool, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.last == nil {
		t.last = map[string]*throttleState{}
	}
	s := t.last[key]
	if s != nil && now.Sub(s.at) < t.Every {
		s.held++
		return false, 0
	}
	if s == nil {
		if len(t.last) >= maxThrottleKeys && now.Sub(t.swept) >= sweepEvery {
			t.swept = now
			for k, v := range t.last {
				if now.Sub(v.at) >= t.Every {
					delete(t.last, k)
				}
			}
		}
		if len(t.last) >= maxThrottleKeys {
			key = overflowKey
			if s = t.last[key]; s != nil && now.Sub(s.at) < t.Every {
				s.held++
				return false, 0
			}
		}
		if s == nil {
			s = &throttleState{}
			t.last[key] = s
		}
	}
	n := s.held + 1
	s.at, s.held = now, 0
	return true, n
}

// Reset forgets every key: the next occurrence of each goes out.
func (t *Throttle) Reset() {
	t.mu.Lock()
	t.last = nil
	t.mu.Unlock()
}

// Fingerprint is the SHA-256 of a DER certificate, in hexadecimal.
func Fingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

// CertDoc describes a certificate for an event's peer document or a
// connection's peer identity: subject, issuer, serial, SHA-256 fingerprint
// and expiry. It holds no key material.
func CertDoc(c *x509.Certificate) bson.M {
	if c == nil {
		return bson.M{}
	}
	return bson.M{"subject": c.Subject.String(), "issuer": c.Issuer.String(),
		"serial": c.SerialNumber.Text(16), "sha256": Fingerprint(c.Raw), "notAfter": c.NotAfter}
}

// RawCertDoc is CertDoc of the first certificate of a handshake's raw chain,
// or only its fingerprint when it does not parse; nil without one.
func RawCertDoc(raw [][]byte) bson.M {
	if len(raw) == 0 {
		return nil
	}
	if c, err := x509.ParseCertificate(raw[0]); err == nil {
		return CertDoc(c)
	}
	return bson.M{"sha256": Fingerprint(raw[0])}
}

// TLSVersionName is "TLS1.2", "TLS1.3", ... ("" when unknown).
func TLSVersionName(v uint16) string {
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
	return ""
}

// TLSStateDoc describes an established TLS session: version, cipher suite
// and the peer's certificate, verified or not.
func TLSStateDoc(cs *tls.ConnectionState) bson.M {
	if cs == nil {
		return nil
	}
	m := bson.M{"version": TLSVersionName(cs.Version), "cipherSuite": tls.CipherSuiteName(cs.CipherSuite),
		"verified": len(cs.VerifiedChains) > 0}
	if len(cs.PeerCertificates) > 0 {
		for k, v := range CertDoc(cs.PeerCertificates[0]) {
			m[k] = v
		}
	}
	return m
}

// IsCertError reports whether a TLS handshake failed on the peer's
// certificate (unknown authority, invalid, wrong host name, refused by a
// verification callback) rather than on the protocol or the network.
func IsCertError(err error) bool {
	if err == nil {
		return false
	}
	var ua x509.UnknownAuthorityError
	var ci x509.CertificateInvalidError
	var he x509.HostnameError
	var ve *tls.CertificateVerificationError
	return errors.As(err, &ua) || errors.As(err, &ci) || errors.As(err, &he) || errors.As(err, &ve)
}

// SupervisedState reads the value of the point a command tag supervises
// (realtimeData supervisedOfCommand), for the check of a command that would
// not change the state it commands. Only a value last updated before
// `before` (the command's dispatch) counts: a later one may already be the
// command's own result. ok is false when the command has no supervised
// point, the point's quality is bad or its value is not older than before.
// It may run after the dispatch, so the check never delays a command.
func SupervisedState(ctx context.Context, rt *mongo.Collection, cmdKey float64, before time.Time) (value float64, ok bool, err error) {
	if rt == nil || cmdKey == 0 {
		return 0, false, nil
	}
	qctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var cmd bson.M
	if err := rt.FindOne(qctx, bson.M{"_id": cmdKey}).Decode(&cmd); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, false, nil
		}
		return 0, false, err
	}
	sup := jsmongo.GetDouble(cmd, "supervisedOfCommand", 0)
	if sup == 0 {
		return 0, false, nil
	}
	var st bson.M
	if err := rt.FindOne(qctx, bson.M{"_id": sup}).Decode(&st); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if jsmongo.GetBool(st, "invalid", true) {
		return 0, false, nil
	}
	if at := jsmongo.GetTime(st, "timeTag"); at.IsZero() || !at.Before(before) {
		return 0, false, nil
	}
	return jsmongo.GetDouble(st, "value", 0), true, nil
}
