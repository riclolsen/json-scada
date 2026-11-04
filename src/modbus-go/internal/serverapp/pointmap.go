/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package serverapp

// In-memory register/coil banks keyed by (unit, area). Values are pre-encoded,
// so a read is a memory copy with no Mongo access. Also owns the mapping from
// written addresses back to command tags. Ported from point-map.ts.

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"modbus-go/internal/drv"
	"modbus-go/internal/mbcore"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
)

// Tag types as realtimeData names them.
const (
	TagAnalog  = "analog"
	TagDigital = "digital"
	TagString  = "string"
)

// Point is one realtimeData tag served on this connection.
type Point struct {
	ID       any
	PointKey float64
	Tag      string
	Unit     int
	Area     mbcore.Area
	Offset   int
	Bit      int // -1 when not a bit-in-register point
	Asdu     mbcore.Asdu
	// protocolDestination scaling, applied when encoding the value out
	Kconv1, Kconv2 float64
	Span           int // registers or bits
	IsCommand      bool
	// tag-level fields, used when routing a received write as a command
	TagType              string
	TagKconv1, TagKconv2 float64
	// command routing copied from the tag's protocolSource*
	CmdConnectionNumber *float64
	CmdCommonAddress    *float64
	CmdObjectAddress    any
	CmdAsdu             any
}

// HasBit reports a bit-in-register point.
func (p *Point) HasBit() bool { return p.Bit >= 0 }

// TagDoc is the part of a realtimeData document the map needs.
type TagDoc struct {
	ID                             any
	Tag                            string
	Type                           any
	Value                          any
	HasValue                       bool
	ValueString                    any
	Invalid                        any
	Origin                         any
	Kconv1, Kconv2                 any
	HasKconv1, HasKconv2           bool
	ProtocolSourceConnectionNumber any
	ProtocolSourceCommonAddress    any
	ProtocolSourceObjectAddress    any
	ProtocolSourceASDU             any
	Dest                           map[string]any
}

type banks struct {
	hr, ir map[int]uint16
	co, di map[int]bool
}

func newBanks() *banks {
	return &banks{hr: map[int]uint16{}, ir: map[int]uint16{}, co: map[int]bool{}, di: map[int]bool{}}
}

func (b *banks) regs(a mbcore.Area) map[int]uint16 {
	if a == mbcore.AreaHolding {
		return b.hr
	}
	return b.ir
}

func (b *banks) bits(a mbcore.Area) map[int]bool {
	if a == mbcore.AreaCoil {
		return b.co
	}
	return b.di
}

// PointMap is safe for concurrent use.
type PointMap struct {
	cfg Config

	mu         sync.RWMutex
	units      map[int]*banks
	writeIndex map[string]*Point
	byID       map[string]*Point
	points     []*Point
}

// NewPointMap starts an empty map.
func NewPointMap(cfg Config) *PointMap {
	return &PointMap{cfg: cfg, units: map[int]*banks{}, writeIndex: map[string]*Point{}, byID: map[string]*Point{}}
}

func writeKey(unit int, a mbcore.Area, off int) string { return fmt.Sprintf("%d:%s:%d", unit, a, off) }

// AcceptsUnit applies strictUnitId: a unit is served when it is listed in
// serverUnitIds or has mapped points.
func (m *PointMap) AcceptsUnit(unit int) bool {
	if !m.cfg.StrictUnitID {
		return true
	}
	for _, u := range m.cfg.ServerUnitIDs {
		if u == unit {
			return true
		}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.units[unit]
	return ok
}

// Len is the number of mapped points.
func (m *PointMap) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.points)
}

func clampUnit(v any) int {
	n := mbcore.JSRound(drv.JSNumber(v, true))
	if math.IsNaN(n) || n < 0 || n > 255 {
		return 1
	}
	return int(n)
}

func numOrNil(v any) *float64 {
	if v == nil {
		return nil
	}
	if s, ok := v.(string); ok && s == "" {
		return nil
	}
	f := drv.JSNumber(v, true)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return nil
	}
	return &f
}

func normalizeType(v any) string {
	s := ""
	if v != nil {
		s = strings.ToLower(drv.AnyString(v))
	}
	switch s {
	case TagDigital, TagString:
		return s
	}
	return TagAnalog
}

// truthy is JavaScript's Boolean(x).
func truthy(x any) bool {
	switch v := x.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case float64, float32, int32, int64, int:
		f := drv.JSNumber(v, true)
		return f != 0 && !math.IsNaN(f)
	}
	return true
}

// Rebuild replaces the map with the given documents, in order: the first
// command point covering an address receives its writes.
func (m *PointMap) Rebuild(docs []TagDoc) {
	units := map[int]*banks{}
	writeIndex := map[string]*Point{}
	byID := map[string]*Point{}
	var points []*Point
	owner := map[string]string{}
	d := m.cfg.Defaults()

	for _, doc := range docs {
		p, err := m.buildPoint(doc, d)
		if err != nil {
			jslog.Log(jslog.LevelDetailed, "%s: skipping destination for tag %s: %v", m.cfg.Name, doc.Tag, err)
			continue
		}
		points = append(points, p)
		byID[drv.AnyString(p.ID)] = p
		if units[p.Unit] == nil {
			units[p.Unit] = newBanks()
		}
		for i := 0; i < p.Span; i++ {
			k := writeKey(p.Unit, p.Area, p.Offset+i)
			if p.HasBit() {
				k += fmt.Sprintf(".b%d", p.Bit)
			}
			if prev, dup := owner[k]; dup {
				jslog.Log(jslog.LevelDetailed, "%s: tag %s overlaps tag %s at unit=%d %s:%d", m.cfg.Name, p.Tag, prev, p.Unit, p.Area, p.Offset+i)
			} else {
				owner[k] = p.Tag
			}
			if p.IsCommand && mbcore.IsWritableArea(p.Area) {
				wk := writeKey(p.Unit, p.Area, p.Offset+i)
				if _, ok := writeIndex[wk]; !ok {
					writeIndex[wk] = p
				}
			}
		}
		encodeInto(m.cfg, units[p.Unit], p, drv.JSNumber(doc.Value, doc.HasValue), anyStr(doc.ValueString), truthy(doc.Invalid))
	}

	m.mu.Lock()
	m.units, m.writeIndex, m.byID, m.points = units, writeIndex, byID, points
	m.mu.Unlock()

	jslog.Log(jslog.LevelBasic, "%s: mapped %d points across %d unit(s)", m.cfg.Name, len(points), len(units))
	if jslog.Level() >= jslog.LevelDebug {
		m.logLayout(points, units)
	}
}

func anyStr(v any) string {
	if v == nil {
		return ""
	}
	return drv.AnyString(v)
}

func (m *PointMap) buildPoint(doc TagDoc, d mbcore.AsduDefaults) (*Point, error) {
	objAddr := ""
	if v, ok := doc.Dest["protocolDestinationObjectAddress"]; ok && v != nil {
		objAddr = drv.AnyString(v)
	}
	addr, err := mbcore.ParseObjectAddress(objAddr, m.cfg.UseModiconAddresses)
	if err != nil {
		return nil, err
	}
	asduStr, _ := doc.Dest["protocolDestinationASDU"].(string)
	asdu, err := mbcore.ParseAsdu(asduStr, addr.Area, d)
	if err != nil {
		return nil, err
	}
	span := 1
	if !mbcore.IsBitArea(addr.Area) && asdu.RegCount > 1 {
		span = asdu.RegCount
	}
	isCommand := doc.Origin == "command" || doc.ProtocolSourceConnectionNumber != nil
	kc1, has1 := doc.Dest["protocolDestinationKConv1"]
	kc2, has2 := doc.Dest["protocolDestinationKConv2"]
	p := &Point{
		ID:        doc.ID,
		PointKey:  drv.JSNumber(doc.ID, true),
		Tag:       doc.Tag,
		Unit:      clampUnit(doc.Dest["protocolDestinationCommonAddress"]),
		Area:      addr.Area,
		Offset:    addr.Offset,
		Bit:       addr.Bit,
		Asdu:      asdu,
		Kconv1:    drv.ToNum(kc1, has1, 1),
		Kconv2:    drv.ToNum(kc2, has2, 0),
		Span:      span,
		IsCommand: isCommand,
		TagType:   normalizeType(doc.Type),
		TagKconv1: drv.ToNum(doc.Kconv1, doc.HasKconv1, 1),
		TagKconv2: drv.ToNum(doc.Kconv2, doc.HasKconv2, 0),
	}
	if isCommand {
		// Routing fields are passed through as stored (a number stays a number),
		// so the client driver owning the point sees what a UI command carries.
		p.CmdConnectionNumber = numOrNil(doc.ProtocolSourceConnectionNumber)
		p.CmdCommonAddress = numOrNil(doc.ProtocolSourceCommonAddress)
		p.CmdObjectAddress = doc.ProtocolSourceObjectAddress
		if p.CmdObjectAddress == nil {
			p.CmdObjectAddress = ""
		}
		p.CmdAsdu = doc.ProtocolSourceASDU
	}
	return p, nil
}

func (m *PointMap) logLayout(points []*Point, units map[int]*banks) {
	for _, p := range points {
		r := fmt.Sprintf("%s:%d", p.Area, p.Offset)
		if p.Span > 1 {
			r += fmt.Sprintf("-%d", p.Offset+p.Span-1)
		}
		if p.HasBit() {
			r += fmt.Sprintf(".b%d", p.Bit)
		}
		kind := "supervised"
		if p.IsCommand {
			kind = "command"
		}
		jslog.Log(jslog.LevelDebug, "%s:   unit=%d %s %s %s tag=%s", m.cfg.Name, p.Unit, r, p.Asdu.Label(), kind, p.Tag)
	}
	ids := make([]int, 0, len(units))
	for u := range units {
		ids = append(ids, u)
	}
	sort.Ints(ids)
	for _, u := range ids {
		b := units[u]
		jslog.Log(jslog.LevelDebug, "%s:   unit=%d coverage hr=[%s] ir=[%s] co=[%s] di=[%s] serveUnmappedAsZero=%t",
			m.cfg.Name, u, summarize(keysOf(b.hr)), summarize(keysOf(b.ir)), summarize(keysOf(b.co)), summarize(keysOf(b.di)), m.cfg.ServeUnmappedAsZero)
	}
}

func keysOf[V any](m map[int]V) []int {
	ks := make([]int, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Ints(ks)
	return ks
}

func summarize(ks []int) string {
	if len(ks) == 0 {
		return "none"
	}
	return fmt.Sprintf("%d addr, range %d..%d", len(ks), ks[0], ks[len(ks)-1])
}

// Update encodes a new value of a mapped tag; false when the tag is not mapped.
func (m *PointMap) Update(id any, value float64, valueString string, invalid bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.byID[drv.AnyString(id)]
	if p == nil {
		return false
	}
	encodeInto(m.cfg, m.units[p.Unit], p, value, valueString, invalid)
	return true
}

// StoreWritten puts a value a master wrote into a supervised point
// (allowWritesToSupervised), unscaled.
func (m *PointMap) StoreWritten(p *Point, raw mbcore.Value) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if b := m.units[p.Unit]; b != nil {
		storeRaw(m.cfg, b, p, raw)
	}
}

// encodeInto writes a tag's value into its bank. Only the protocolDestination
// scaling applies: realtimeData.value already is the engineering value.
func encodeInto(cfg Config, b *banks, p *Point, value float64, valueString string, invalid bool) {
	if b == nil {
		return
	}
	if invalid && cfg.InvalidValuePolicy == "zero" {
		zeroOut(b, p)
		return
	}
	if mbcore.IsBitArea(p.Area) {
		on := value != 0
		if p.Kconv1 == -1 {
			on = !on
		}
		b.bits(p.Area)[p.Offset] = on
		return
	}
	if p.HasBit() {
		setBit(b.regs(p.Area), p, value != 0)
		return
	}
	var v mbcore.Value
	if p.Asdu.Type == mbcore.TypeString {
		v = mbcore.Str(valueString)
	} else {
		v = mbcore.Num(value*p.Kconv1 + p.Kconv2)
	}
	putWire(cfg, b, p, v)
}

// storeRaw is encodeInto for a written raw value: no scaling.
func storeRaw(cfg Config, b *banks, p *Point, raw mbcore.Value) {
	if mbcore.IsBitArea(p.Area) {
		on := raw.Float() != 0
		if p.Kconv1 == -1 {
			on = !on
		}
		b.bits(p.Area)[p.Offset] = on
		return
	}
	if p.HasBit() {
		setBit(b.regs(p.Area), p, raw.Float() != 0)
		return
	}
	v := raw
	if p.Asdu.Type == mbcore.TypeString {
		v = mbcore.Str(raw.String())
	} else if raw.Kind == mbcore.KindString || raw.Kind == mbcore.KindBool {
		v = mbcore.Num(raw.Float())
	}
	putWire(cfg, b, p, v)
}

func setBit(regs map[int]uint16, p *Point, on bool) {
	cur := regs[p.Offset]
	if on {
		cur |= 1 << uint(p.Bit)
	} else {
		cur &^= 1 << uint(p.Bit)
	}
	regs[p.Offset] = cur
}

func putWire(cfg Config, b *banks, p *Point, v mbcore.Value) {
	wire, err := mbcore.Encode(p.Asdu.Type, v, p.Asdu.Perm, p.Asdu.Str)
	if err != nil {
		jslog.Log(jslog.LevelDetailed, "%s: encode error tag %s: %v", cfg.Name, p.Tag, err)
		return
	}
	regs := b.regs(p.Area)
	for i := 0; i < p.Span && 2*i+1 < len(wire); i++ {
		regs[p.Offset+i] = uint16(wire[2*i])<<8 | uint16(wire[2*i+1])
	}
}

func zeroOut(b *banks, p *Point) {
	if mbcore.IsBitArea(p.Area) {
		b.bits(p.Area)[p.Offset] = false
		return
	}
	regs := b.regs(p.Area)
	for i := 0; i < p.Span; i++ {
		regs[p.Offset+i] = 0
	}
}

// ReadBits serves FC1/FC2; ok is false for an unmapped address (exception 02)
// unless serveUnmappedAsZero is on.
func (m *PointMap) ReadBits(unit int, a mbcore.Area, start, qty int) ([]bool, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b := m.units[unit]
	if b == nil && !m.cfg.ServeUnmappedAsZero {
		return nil, false
	}
	out := make([]bool, qty)
	for i := range out {
		var v, ok bool
		if b != nil {
			v, ok = b.bits(a)[start+i]
		}
		if !ok && !m.cfg.ServeUnmappedAsZero {
			return nil, false
		}
		out[i] = v
	}
	return out, true
}

// ReadRegisters serves FC3/FC4, as ReadBits.
func (m *PointMap) ReadRegisters(unit int, a mbcore.Area, start, qty int) ([]uint16, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b := m.units[unit]
	if b == nil && !m.cfg.ServeUnmappedAsZero {
		return nil, false
	}
	out := make([]uint16, qty)
	for i := range out {
		var v uint16
		var ok bool
		if b != nil {
			v, ok = b.regs(a)[start+i]
		}
		if !ok && !m.cfg.ServeUnmappedAsZero {
			return nil, false
		}
		out[i] = v
	}
	return out, true
}

// LookupWrite is the command point at a written address, if any.
func (m *PointMap) LookupWrite(unit int, a mbcore.Area, off int) *Point {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.writeIndex[writeKey(unit, a, off)]
}

// DecodeWrittenValue decodes the registers a master wrote into the raw value
// it meant, with the point's byte order and no scaling. A bit-in-register
// point yields 0 or 1.
func DecodeWrittenValue(p *Point, regs []uint16) (mbcore.Value, error) {
	if p.HasBit() {
		if len(regs) < 1 {
			return mbcore.Value{}, fmt.Errorf("no register written")
		}
		return mbcore.Num(float64((regs[0] >> uint(p.Bit)) & 1)), nil
	}
	wire := make([]byte, 2*len(regs))
	for i, r := range regs {
		wire[2*i], wire[2*i+1] = byte(r>>8), byte(r)
	}
	res, err := mbcore.Decode(p.Asdu.Type, wire, 0, p.Asdu.Perm, p.Asdu.Str)
	if err != nil {
		return mbcore.Value{}, err
	}
	return res.Value, nil
}

// RouteCommandValue is the value a written raw value becomes as a command.
// Both the protocolDestination and the tag-level scaling apply:
//
//	digital: forced to 1/0, inverted when exactly one of the two kconv1 is -1
//	analog:  (raw*destKconv1 + destKconv2)*tagKconv1 + tagKconv2
//	string:  the decoded string, value 0
func RouteCommandValue(p *Point, raw mbcore.Value) (float64, string) {
	switch p.TagType {
	case TagDigital:
		bit := 0.0
		if raw.Float() != 0 {
			bit = 1
		}
		if (p.Kconv1 == -1) != (p.TagKconv1 == -1) {
			bit = 1 - bit
		}
		return bit, mbcore.JSNumberString(bit)
	case TagString:
		return 0, raw.String()
	}
	dest := raw.Float()*p.Kconv1 + p.Kconv2
	v := dest*p.TagKconv1 + p.TagKconv2
	return v, mbcore.JSNumberString(v)
}
