/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package serverapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"modbus-go/internal/mbcore"

	"github.com/otfabric/go-modbus"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func cmdTag(t *testing.T, id float64, tag, typ, addr, asdu string) TagDoc {
	t.Helper()
	return tagDoc(t, bson.M{"_id": id, "tag": tag, "type": typ, "origin": "command",
		"protocolSourceConnectionNumber": 9.0, "protocolDestinations": dst(addr, asdu, 1)})
}

func supTag(t *testing.T, id float64, tag, addr, asdu string, value float64) TagDoc {
	t.Helper()
	return tagDoc(t, bson.M{"_id": id, "tag": tag, "type": "analog", "value": value, "protocolDestinations": dst(addr, asdu, 1)})
}

// drain returns the tags of the commands relayed within a short wait.
func drain(sink sinkCh) map[string]float64 {
	got := map[string]float64{}
	for {
		select {
		case d := <-sink:
			m := map[string]any{}
			for _, e := range d {
				m[e.Key] = e.Value
			}
			got[m["tag"].(string)] = m["value"].(float64)
		case <-time.After(200 * time.Millisecond):
			return got
		}
	}
}

func TestFC6ToMultiRegisterPointRefused(t *testing.T) {
	cfg := Normalize(bson.M{"protocolConnectionNumber": 1.0})
	pm := NewPointMap(cfg)
	pm.Rebuild([]TagDoc{cmdTag(t, 1, "SP", "analog", "hr:10", "float32"), cmdTag(t, 2, "U", "analog", "hr:20", "uint16")})
	sink := make(sinkCh, 8)
	wh := NewWriteHandler(cfg, sink, pm)
	for _, addr := range []int{10, 11} {
		if err := wh.RegisterWrite(1, addr, 0x4248, "m"); !errors.Is(err, modbus.ErrIllegalDataAddress) {
			t.Fatalf("FC6 to hr:%d of a float32: %v", addr, err)
		}
	}
	if err := wh.RegisterWrite(1, 20, 7, "m"); err != nil {
		t.Fatal(err)
	}
	if got := drain(sink); len(got) != 1 || got["U"] != 7 {
		t.Fatalf("relayed %v", got)
	}
	// FC16 writing only the second half of the float32 relays nothing.
	if err := wh.MultiRegisterWrite(1, 11, []uint16{0}, "m"); !errors.Is(err, modbus.ErrIllegalDataAddress) {
		t.Fatalf("partial FC16: %v", err)
	}
}

func TestWritesToSupervised(t *testing.T) {
	for _, allow := range []bool{false, true} {
		cfg := Normalize(bson.M{"protocolConnectionNumber": 1.0, "allowWritesToSupervised": allow})
		pm := NewPointMap(cfg)
		pm.Rebuild([]TagDoc{
			supTag(t, 1, "S", "hr:0", "uint16", 3),
			supTag(t, 2, "C", "co:0", "", 0),
			cmdTag(t, 3, "K", "digital", "co:1", ""),
		})
		sink := make(sinkCh, 8)
		wh := NewWriteHandler(cfg, sink, pm)
		err := wh.RegisterWrite(1, 0, 99, "m")
		if !allow {
			if !errors.Is(err, modbus.ErrIllegalFunction) {
				t.Fatalf("refused write: %v", err)
			}
			// FC15 over a supervised and a command coil relays nothing.
			if err := wh.MultiCoilWrite(1, 0, []bool{true, true}, "m"); !errors.Is(err, modbus.ErrIllegalFunction) {
				t.Fatalf("FC15: %v", err)
			}
			if got := drain(sink); len(got) != 0 {
				t.Fatalf("relayed %v", got)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if r, _ := pm.ReadRegisters(1, mbcore.AreaHolding, 0, 1); r[0] != 99 {
			t.Fatalf("stored %v", r)
		}
		if err := wh.MultiCoilWrite(1, 0, []bool{true, true}, "m"); err != nil {
			t.Fatal(err)
		}
		if b, _ := pm.ReadBits(1, mbcore.AreaCoil, 0, 1); !b[0] {
			t.Fatal("coil not stored")
		}
		if got := drain(sink); len(got) != 1 || got["K"] != 1 {
			t.Fatalf("relayed %v", got)
		}
	}
}

// Several register bit command points on one register: FC22 reaches only the
// bits it changes, FC6 reaches every bit; a supervised word at the same
// address does not take the writes from them.
func TestBitPointsShareRegister(t *testing.T) {
	cfg := Normalize(bson.M{"protocolConnectionNumber": 1.0})
	pm := NewPointMap(cfg)
	pm.Rebuild([]TagDoc{
		supTag(t, 1, "STATUS", "hr:5", "uint16", 0),
		cmdTag(t, 2, "B0", "digital", "hr:5.b0", ""),
		cmdTag(t, 3, "B1", "digital", "hr:5.b1", ""),
	})
	sink := make(sinkCh, 8)
	wh := NewWriteHandler(cfg, sink, pm)
	if err := wh.MaskWrite(1, 5, ^uint16(1<<1), 1<<1, "m"); err != nil {
		t.Fatal(err)
	}
	if got := drain(sink); len(got) != 1 || got["B1"] != 1 {
		t.Fatalf("FC22 relayed %v", got)
	}
	if err := wh.RegisterWrite(1, 5, 0b01, "m"); err != nil {
		t.Fatal(err)
	}
	if got := drain(sink); len(got) != 2 || got["B0"] != 1 || got["B1"] != 0 {
		t.Fatalf("FC6 relayed %v", got)
	}
	// A mask that changes only unmapped bits.
	if err := wh.MaskWrite(1, 5, ^uint16(1<<7), 0, "m"); !errors.Is(err, modbus.ErrIllegalDataAddress) {
		t.Fatalf("FC22 to an unmapped bit: %v", err)
	}
}

func TestRegisterBoolPoint(t *testing.T) {
	cfg := Normalize(bson.M{"protocolConnectionNumber": 1.0})
	pm := NewPointMap(cfg)
	pm.Rebuild([]TagDoc{
		tagDoc(t, bson.M{"_id": 1.0, "tag": "ON", "value": 1.0, "protocolDestinations": dst("hr:0", "bool", 1)}),
		tagDoc(t, bson.M{"_id": 2.0, "tag": "CMD", "type": "digital", "origin": "command", "protocolSourceConnectionNumber": 9.0,
			"protocolDestinations": dst("hr:1", "bool", 1, "protocolDestinationKConv1", -1.0)}),
	})
	if r, _ := pm.ReadRegisters(1, mbcore.AreaHolding, 0, 1); r[0] != 1 {
		t.Fatalf("served %v", r)
	}
	sink := make(sinkCh, 8)
	wh := NewWriteHandler(cfg, sink, pm)
	if err := wh.RegisterWrite(1, 1, 5, "m"); err != nil {
		t.Fatal(err)
	}
	if got := drain(sink); len(got) != 1 || got["CMD"] != 0 { // non-zero is ON, inverted once
		t.Fatalf("relayed %v", got)
	}
}

// An update that arrives while a full reload reads realtimeData survives the
// rebuild from the older documents.
func TestReloadKeepsLiveUpdates(t *testing.T) {
	cfg := Normalize(bson.M{"protocolConnectionNumber": 1.0})
	pm := NewPointMap(cfg)
	docs := []TagDoc{supTag(t, 1, "A", "hr:0", "uint16", 10)}
	pm.Rebuild(docs)
	pm.BeginReload()
	pm.Update(1.0, 20, "", false) // arrives during the read
	pm.Rebuild(docs)              // documents read before the change
	if r, _ := pm.ReadRegisters(1, mbcore.AreaHolding, 0, 1); r[0] != 20 {
		t.Fatalf("live update lost: %v", r)
	}
	// Outside a reload nothing is kept.
	pm.Update(1.0, 30, "", false)
	pm.Rebuild(docs)
	if r, _ := pm.ReadRegisters(1, mbcore.AreaHolding, 0, 1); r[0] != 10 {
		t.Fatalf("stale pending update applied: %v", r)
	}
	pm.BeginReload()
	pm.EndReload()
	if pm.pending != nil || pm.reloading != 0 {
		t.Fatal("abandoned reload left state")
	}
}

// Requests that did not come through the front-end get no answer.
func TestBackendAnswersOnlyTheFrontEnd(t *testing.T) {
	cfg := Normalize(bson.M{"protocolConnectionNumber": 1.0})
	s := NewServer(cfg, make(sinkCh, 1), nil)
	s.PM.Rebuild([]TagDoc{supTag(t, 1, "A", "hr:0", "uint16", 10)})
	h := &Handler{cfg: s.Cfg, pm: s.PM, wh: s.wh, addrs: &s.addrs, st: &s.st}
	req := &modbus.HoldingRegistersRequest{ClientAddr: "127.0.0.1:40000", UnitID: 1, Addr: 0, Quantity: 1, FunctionCode: 3}
	if _, err := h.HandleHoldingRegisters(context.Background(), req); !errors.Is(err, modbus.ErrNoResponse) {
		t.Fatalf("stranger answered: %v", err)
	}
	s.addrs.Put("127.0.0.1:40000", "10.0.0.1:5000")
	if r, err := h.HandleHoldingRegisters(context.Background(), req); err != nil || r[0] != 10 {
		t.Fatalf("front-end connection: %v %v", r, err)
	}
}

func TestSerialClientsStat(t *testing.T) {
	cfg := Normalize(bson.M{"protocolConnectionNumber": 1.0, "connectionMode": ModeSerial, "clientIdleTimeoutMs": 1000.0})
	s := NewServer(cfg, make(sinkCh, 1), nil)
	if s.Stats().Clients != 0 {
		t.Fatal("closed port counted")
	}
	s.serialOpened = time.Now().Add(-time.Minute)
	if s.Stats().Clients != 0 {
		t.Fatal("open port without requests counted")
	}
	s.st.lastRequest.Store(time.Now().UnixNano())
	if s.Stats().Clients != 1 {
		t.Fatal("polling master not counted")
	}
	s.st.lastRequest.Store(time.Now().Add(-2 * time.Second).UnixNano())
	if s.Stats().Clients != 0 {
		t.Fatal("idle master counted")
	}
}

func TestRTUSocketGap(t *testing.T) {
	s := NewServer(Normalize(bson.M{"baudRate": 115200.0}), make(sinkCh, 1), nil)
	if g := s.socketGap(); g != socketGapMin {
		t.Fatalf("socket gap %v", g)
	}
	if g := s.idleGap(); g >= socketGapMin {
		t.Fatalf("serial gap %v", g)
	}
	s = NewServer(Normalize(bson.M{"interFrameDelayMs": 20.0}), make(sinkCh, 1), nil)
	if s.socketGap() != 20*time.Millisecond {
		t.Fatalf("explicit gap %v", s.socketGap())
	}
}
