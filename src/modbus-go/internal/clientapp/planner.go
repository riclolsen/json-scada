/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package clientapp

// The poll planner turns the tags bound to a connection into read blocks,
// coalescing adjacent addresses (within maxAddressGap) up to the request size
// limits, one plan per (unit id, area). Ported from poll-planner.ts.

import (
	"math"
	"sort"

	"modbus-go/internal/drv"
	"modbus-go/internal/mbcore"
)

// Binding is one supervised tag bound to the connection.
type Binding struct {
	ID     any // realtimeData _id
	Tag    string
	Unit   int
	Area   mbcore.Area
	Offset int
	Bit    int
	Asdu   mbcore.Asdu
	Kconv1 float64
	Kconv2 float64
	Span   int // registers or bits occupied
}

// Slice places a binding inside a block: RelStart is in registers for register
// areas and in bits for bit areas.
type Slice struct {
	B        *Binding
	RelStart int
}

// Block is one read request.
type Block struct {
	Unit   int
	Area   mbcore.Area
	FC     int
	Start  int
	Count  int
	Slices []Slice

	// poll state
	lastRaw          []byte
	invalidPublished bool
}

// Limits bound the planner.
type Limits struct {
	MaxReadRegisters float64
	MaxReadCoils     float64
	MaxAddressGap    float64
}

// RawBinding is what a realtimeData document supplies.
type RawBinding struct {
	ID            any
	Tag           string
	ObjectAddress string
	Asdu          string
	CommonAddress any
	HasCommon     bool
	Kconv1        any
	HasKconv1     bool
	Kconv2        any
	HasKconv2     bool
}

// BuildBinding parses a tag's protocolSource* fields.
func BuildBinding(r RawBinding, d mbcore.AsduDefaults, useModicon bool) (*Binding, error) {
	addr, err := mbcore.ParseObjectAddress(r.ObjectAddress, useModicon)
	if err != nil {
		return nil, err
	}
	asdu, err := mbcore.ParseAsdu(r.Asdu, addr.Area, d)
	if err != nil {
		return nil, err
	}
	span := 1
	if !mbcore.IsBitArea(addr.Area) && asdu.RegCount > 1 {
		span = asdu.RegCount
	}
	return &Binding{
		ID: r.ID, Tag: r.Tag,
		Unit: clampUnitDefault1(toNum(r.CommonAddress, r.HasCommon, 1)),
		Area: addr.Area, Offset: addr.Offset, Bit: addr.Bit, Asdu: asdu,
		Kconv1: toNum(r.Kconv1, r.HasKconv1, 1),
		Kconv2: toNum(r.Kconv2, r.HasKconv2, 0),
		Span:   span,
	}, nil
}

// PlanReads computes the ordered read plan.
func PlanReads(bindings []*Binding, l Limits) []*Block {
	type key struct {
		unit int
		area mbcore.Area
	}
	groups := map[key][]*Binding{}
	var order []key
	for _, b := range bindings {
		k := key{b.Unit, b.Area}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], b)
	}
	var blocks []*Block
	for _, k := range order {
		list := groups[k]
		sort.SliceStable(list, func(i, j int) bool { return list[i].Offset < list[j].Offset })
		first := list[0]
		maxCount := l.MaxReadRegisters
		if mbcore.IsBitArea(first.Area) {
			maxCount = l.MaxReadCoils
		}
		fc := fcForArea(first.Area)
		start := first.Offset
		end := first.Offset + first.Span
		slices := []Slice{{B: first, RelStart: 0}}
		flush := func() {
			blocks = append(blocks, &Block{Unit: first.Unit, Area: first.Area, FC: fc, Start: start, Count: end - start, Slices: slices})
		}
		for _, b := range list[1:] {
			tentativeEnd := end
			if b.Offset+b.Span > tentativeEnd {
				tentativeEnd = b.Offset + b.Span
			}
			gap := b.Offset - end
			if float64(tentativeEnd-start) <= maxCount && float64(gap) <= l.MaxAddressGap {
				end = tentativeEnd
				slices = append(slices, Slice{B: b, RelStart: b.Offset - start})
			} else {
				flush()
				start = b.Offset
				end = b.Offset + b.Span
				slices = []Slice{{B: b, RelStart: 0}}
			}
		}
		flush()
	}
	return blocks
}

func fcForArea(a mbcore.Area) int {
	switch a {
	case mbcore.AreaCoil:
		return mbcore.FCReadCoils
	case mbcore.AreaDiscrete:
		return mbcore.FCReadDiscreteInputs
	case mbcore.AreaInput:
		return mbcore.FCReadInputRegisters
	}
	return mbcore.FCReadHoldingRegisters
}

// clampUnitDefault1 rounds a unit id; anything outside 0..255 becomes 1.
func clampUnitDefault1(f float64) int {
	n := mbcore.JSRound(f)
	if math.IsNaN(n) || n < 0 || n > 255 {
		return 1
	}
	return int(n)
}

func toNum(v any, present bool, def float64) float64 { return drv.ToNum(v, present, def) }

func jsNumber(v any, present bool) float64 { return drv.JSNumber(v, present) }
