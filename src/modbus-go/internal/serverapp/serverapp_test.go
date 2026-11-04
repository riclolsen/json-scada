/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package serverapp

import (
	"context"
	"testing"
	"time"

	"modbus-go/internal/mbcore"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Ported from src/modbus/test/unit/route-command.test.ts.
func pt(tagType string, tagK1, tagK2, destK1, destK2 float64) *Point {
	return &Point{TagType: tagType, TagKconv1: tagK1, TagKconv2: tagK2, Kconv1: destK1, Kconv2: destK2, Bit: -1}
}

func route(p *Point, raw mbcore.Value) (float64, string) { return RouteCommandValue(p, raw) }

func TestRouteCommandValue(t *testing.T) {
	n := mbcore.Num
	check := func(name string, p *Point, raw mbcore.Value, want float64, wantStr string) {
		t.Helper()
		v, s := route(p, raw)
		if v != want || (wantStr != "" && s != wantStr) {
			t.Errorf("%s: got %v %q, want %v %q", name, v, s, want, wantStr)
		}
	}
	p := pt(TagDigital, 1, 0, 1, 0)
	check("digital 1", p, n(1), 1, "1")
	check("digital 5", p, n(5), 1, "1")
	check("digital 0", p, n(0), 0, "0")
	check("digital ff00", p, n(0xff00), 1, "1")
	p = pt(TagDigital, -1, 0, 1, 0)
	check("inverted 1", p, n(1), 0, "0")
	check("inverted 42", p, n(42), 0, "0")
	check("inverted 0", p, n(0), 1, "1")
	check("analog", pt(TagAnalog, 2, 10, 1, 0), n(5), 20, "20")
	check("analog 0.1", pt(TagAnalog, 0.1, -3, 1, 0), n(100), 7, "")
	check("analog identity", pt(TagAnalog, 1, 0, 1, 0), n(1234), 1234, "")
	check("both scalings", pt(TagAnalog, 2, 1, 10, 5), n(5), 111, "")
	check("dest only", pt(TagAnalog, 1, 0, 0.5, 0), n(100), 50, "")
	check("dest -1", pt(TagDigital, 1, 0, -1, 0), n(1), 0, "")
	check("dest -1 off", pt(TagDigital, 1, 0, -1, 0), n(0), 1, "")
	check("both -1", pt(TagDigital, -1, 0, -1, 0), n(1), 1, "")
	check("both -1 off", pt(TagDigital, -1, 0, -1, 0), n(0), 0, "")
	check("string", pt(TagString, 1, 0, 1, 0), mbcore.Str("HELLO"), 0, "HELLO")
}

func tagDoc(t *testing.T, d bson.M) TagDoc {
	t.Helper()
	td, ok := ExtractTagDoc(d, 1)
	if !ok {
		t.Fatal("no destination")
	}
	return td
}

func dst(addr, asdu string, unit float64, extra ...any) bson.A {
	m := bson.M{"protocolDestinationConnectionNumber": 1.0, "protocolDestinationObjectAddress": addr,
		"protocolDestinationASDU": asdu, "protocolDestinationCommonAddress": unit}
	for i := 0; i+1 < len(extra); i += 2 {
		m[extra[i].(string)] = extra[i+1]
	}
	return bson.A{bson.M{"protocolDestinationConnectionNumber": 2.0}, m}
}

func TestNormalizeDefaults(t *testing.T) {
	c := Normalize(bson.M{"protocolConnectionNumber": 3.0, "serverUnitIds": bson.A{"5", 6.4, "x"}})
	if c.Name != "MODBUS_SERVER-3" || c.ConnectionMode != ModeTCP || c.IPAddressLocalBind != "0.0.0.0:502" ||
		c.MaxClientConnections != 8 || c.ClientIdleTimeoutMs != 60000 || c.InvalidValuePolicy != "last" ||
		len(c.ServerUnitIDs) != 2 || c.ServerUnitIDs[0] != 5 || c.ServerUnitIDs[1] != 6 {
		t.Fatalf("defaults %+v", c)
	}
	if Normalize(bson.M{}).ServerUnitIDs[0] != 1 {
		t.Fatal("default unit")
	}
	c.ConnectionMode = ModeTLS
	if c.DefaultPort() != 802 {
		t.Fatal("tls port")
	}
}

func TestPointMapServing(t *testing.T) {
	cfg := Normalize(bson.M{"protocolConnectionNumber": 1.0, "invalidValuePolicy": "zero"})
	pm := NewPointMap(cfg)
	pm.Rebuild([]TagDoc{
		tagDoc(t, bson.M{"_id": 1.0, "tag": "A", "value": 10.0, "protocolDestinations": dst("hr:0", "uint16", 1, "protocolDestinationKConv1", 2.0, "protocolDestinationKConv2", 1.0)}),
		tagDoc(t, bson.M{"_id": 2.0, "tag": "F", "value": 1.5, "protocolDestinations": dst("hr:1", "float32_cdab", 1)}),
		tagDoc(t, bson.M{"_id": 3.0, "tag": "B", "value": 1.0, "protocolDestinations": dst("hr:10.b3", "", 1)}),
		tagDoc(t, bson.M{"_id": 4.0, "tag": "C", "value": 1.0, "protocolDestinations": dst("co:0", "", 2, "protocolDestinationKConv1", -1.0)}),
		tagDoc(t, bson.M{"_id": 5.0, "tag": "S", "valueString": "AB", "protocolDestinations": dst("ir:0", "string:2", 1)}),
		tagDoc(t, bson.M{"_id": 6.0, "tag": "BAD", "protocolDestinations": dst("zz:0", "", 1)}),
	})
	if pm.Len() != 5 {
		t.Fatalf("points %d", pm.Len())
	}
	regs, ok := pm.ReadRegisters(1, mbcore.AreaHolding, 0, 3)
	// 10*2+1 = 21; float32 1.5 = 0x3FC00000, CDAB order on the wire = 0000 3FC0
	if !ok || regs[0] != 21 || regs[1] != 0x0000 || regs[2] != 0x3fc0 {
		t.Fatalf("hr %v %v", regs, ok)
	}
	if r, ok := pm.ReadRegisters(1, mbcore.AreaHolding, 10, 1); !ok || r[0] != 1<<3 {
		t.Fatalf("bit in register %v", r)
	}
	if b, ok := pm.ReadBits(2, mbcore.AreaCoil, 0, 1); !ok || b[0] {
		t.Fatalf("inverted coil %v", b)
	}
	if r, ok := pm.ReadRegisters(1, mbcore.AreaInput, 0, 1); !ok || r[0] != 0x4142 {
		t.Fatalf("string %x", r)
	}
	if _, ok := pm.ReadRegisters(1, mbcore.AreaHolding, 0, 4); ok {
		t.Fatal("unmapped address served")
	}
	if _, ok := pm.ReadRegisters(9, mbcore.AreaHolding, 0, 1); ok {
		t.Fatal("unmapped unit served")
	}
	// invalidValuePolicy zero
	pm.Update(1.0, 50, "", true)
	if r, _ := pm.ReadRegisters(1, mbcore.AreaHolding, 0, 1); r[0] != 0 {
		t.Fatalf("invalid not zeroed %v", r)
	}
	if pm.Update(99.0, 1, "", false) {
		t.Fatal("unknown id updated")
	}

	cfg.ServeUnmappedAsZero = true
	pm2 := NewPointMap(cfg)
	if r, ok := pm2.ReadRegisters(9, mbcore.AreaHolding, 0, 2); !ok || r[0] != 0 || r[1] != 0 {
		t.Fatal("serveUnmappedAsZero")
	}
}

func TestStrictUnit(t *testing.T) {
	cfg := Normalize(bson.M{"protocolConnectionNumber": 1.0, "strictUnitId": true, "serverUnitIds": bson.A{4.0}})
	pm := NewPointMap(cfg)
	pm.Rebuild([]TagDoc{tagDoc(t, bson.M{"_id": 1.0, "tag": "A", "protocolDestinations": dst("hr:0", "", 9)})})
	if !pm.AcceptsUnit(4) || !pm.AcceptsUnit(9) || pm.AcceptsUnit(5) {
		t.Fatal("strict units")
	}
}

type sinkCh chan bson.D

func (s sinkCh) Insert(_ context.Context, d bson.D) error { s <- d; return nil }

func TestWriteHandler(t *testing.T) {
	cfg := Normalize(bson.M{"protocolConnectionNumber": 1.0, "name": "SRV"})
	pm := NewPointMap(cfg)
	cmdDoc := func(id float64, tag, addr, asdu string) TagDoc {
		return tagDoc(t, bson.M{"_id": id, "tag": tag, "type": "analog", "origin": "command",
			"protocolSourceConnectionNumber": 9.0, "protocolSourceCommonAddress": 3.0,
			"protocolSourceObjectAddress": 1234.0, "protocolSourceASDU": "C_SE_NC_1",
			"protocolDestinations": dst(addr, asdu, 1)})
	}
	pm.Rebuild([]TagDoc{
		cmdDoc(10, "F", "hr:0", "float32"),
		cmdDoc(11, "U", "hr:2", "uint16"),
		tagDoc(t, bson.M{"_id": 12.0, "tag": "SUP", "protocolDestinations": dst("hr:5", "", 1)}),
		tagDoc(t, bson.M{"_id": 13.0, "tag": "BIT", "type": "digital", "origin": "command", "protocolSourceConnectionNumber": 9.0,
			"protocolDestinations": dst("hr:6.b2", "", 1)}),
	})
	sink := make(sinkCh, 4)
	wh := NewWriteHandler(cfg, sink, pm)
	next := func() map[string]any {
		select {
		case d := <-sink:
			m := map[string]any{}
			for _, e := range d {
				m[e.Key] = e.Value
			}
			return m
		case <-time.After(2 * time.Second):
			t.Fatal("no command")
		}
		return nil
	}
	// FC16 covering both command points: float32 2.5 = 0x4020 0x0000, then 7.
	if err := wh.MultiRegisterWrite(1, 0, []uint16{0x4020, 0, 7}, "1.2.3.4:5"); err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for i := 0; i < 2; i++ {
		m := next()
		got[m["tag"].(string)] = m["value"].(float64)
		if m["protocolSourceObjectAddress"] != 1234.0 || m["protocolSourceConnectionNumber"] != 9.0 ||
			m["originatorIpAddress"] != "1.2.3.4:5" || m["pointKey"] == nil {
			t.Fatalf("command doc %v", m)
		}
	}
	if got["F"] != 2.5 || got["U"] != 7 {
		t.Fatalf("values %v", got)
	}
	if err := wh.RegisterWrite(1, 5, 1, "r"); err == nil {
		t.Fatal("write to supervised accepted")
	}
	if err := wh.RegisterWrite(1, 99, 1, "r"); err == nil {
		t.Fatal("write to unmapped accepted")
	}
	// Mask write setting bit 2.
	if err := wh.MaskWrite(1, 6, ^uint16(1<<2), 1<<2, "r"); err != nil {
		t.Fatal(err)
	}
	if m := next(); m["tag"] != "BIT" || m["value"] != 1.0 {
		t.Fatalf("mask write %v", m)
	}
}
