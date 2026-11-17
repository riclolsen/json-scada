/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package srvapp

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/go-iecp5/cs104"
)

var testParams = asdu.Params{CauseSize: 2, CommonAddrSize: 2, InfoObjAddrSize: 3, InfoObjTimeZone: time.UTC}

// fakeLink is an asdu.Connect recording what is sent on it. The first
// fullFor calls return a full-buffer error.
type fakeLink struct {
	mu      sync.Mutex
	sent    []*asdu.ASDU
	fullFor int
	calls   int
}

func (f *fakeLink) Params() *asdu.Params     { return &testParams }
func (f *fakeLink) UnderlyingConn() net.Conn { return nil }
func (f *fakeLink) Send(a *asdu.ASDU) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls <= f.fullFor {
		return cs104.ErrBufferFulled
	}
	f.sent = append(f.sent, roundTrip(a))
	return nil
}

func (f *fakeLink) all() []*asdu.ASDU {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]*asdu.ASDU(nil), f.sent...)
}

// roundTrip encodes and decodes an ASDU, as the wire would.
func roundTrip(a *asdu.ASDU) *asdu.ASDU {
	data, err := a.MarshalBinary()
	if err != nil {
		panic(err)
	}
	b := asdu.NewEmptyASDU(&testParams)
	if err := b.UnmarshalBinary(data); err != nil {
		panic(err)
	}
	return b
}

// buildASDU returns the ASDU an asdu builder function produces.
func buildASDU(t *testing.T, build func(c asdu.Connect) error) *asdu.ASDU {
	t.Helper()
	l := &fakeLink{}
	if err := build(l); err != nil {
		t.Fatal(err)
	}
	return l.all()[0]
}
