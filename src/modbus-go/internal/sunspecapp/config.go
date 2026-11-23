/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

// Package sunspecapp is the SUNSPEC client driver: it discovers the SunSpec
// model chain of each configured unit (go-sunspec over go-modbus), creates
// the tags of the discovered points, polls the models, writes
// sourceDataUpdate into realtimeData and executes commandsQueue inserts as
// SunSpec point writes.
package sunspecapp

import (
	"math"
	"strings"

	"modbus-go/internal/clientapp"
	"modbus-go/internal/drv"
	"modbus-go/internal/mbcore"

	"github.com/otfabric/go-modbus"
	"github.com/otfabric/go-sunspec"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Register types (registerType).
const (
	RegHolding = "holding"
	RegInput   = "input"
)

// Config is one normalized SUNSPEC protocolConnections document.
type Config struct {
	// Link holds the keys shared with the MODBUS client: name, transport,
	// TLS, timing, autoCreateTags, commandsEnabled, peerIdentification and
	// the stored identity. Its Modbus-only keys are not used.
	Link clientapp.Config

	UnitIDs       []int
	BaseAddresses []uint16 // nil: go-sunspec's default list
	RegisterType  string
	MaxModels     int
	// ModelIDs limits auto-created tags to these models; empty means all.
	ModelIDs []int

	AutoCreateCommandTags   bool
	AutoCreateUnimplemented bool
}

// Normalize reads a protocolConnections document permissively.
func Normalize(doc bson.M) Config {
	link := clientapp.Normalize(doc)
	if _, ok := doc["name"].(string); !ok || strings.TrimSpace(link.Name) == "" {
		link.Name = "SUNSPEC-" + mbcore.JSNumberString(link.ProtocolConnectionNumber)
	}
	c := Config{
		Link:                    link,
		RegisterType:            strings.ToLower(strings.TrimSpace(drv.Str(doc, "registerType", RegHolding))),
		MaxModels:               int(drv.Num(doc, "maxModels", 256)),
		AutoCreateCommandTags:   drv.Bool(doc, "autoCreateCommandTags", true),
		AutoCreateUnimplemented: drv.Bool(doc, "autoCreateUnimplemented", false),
	}
	for _, u := range numArr(doc, "unitIds") {
		if u >= 0 && u <= 255 && u == math.Trunc(u) {
			c.UnitIDs = appendUnique(c.UnitIDs, int(u))
		}
	}
	if len(c.UnitIDs) == 0 {
		c.UnitIDs = []int{1}
	}
	for _, a := range numArr(doc, "baseAddresses") {
		if a >= 0 && a <= 65535 && a == math.Trunc(a) {
			c.BaseAddresses = append(c.BaseAddresses, uint16(a))
		}
	}
	for _, m := range numArr(doc, "modelIds") {
		if m >= 1 && m < 65535 && m == math.Trunc(m) {
			c.ModelIDs = appendUnique(c.ModelIDs, int(m))
		}
	}
	if c.RegisterType != RegInput {
		c.RegisterType = RegHolding
	}
	if c.MaxModels < 1 {
		c.MaxModels = 256
	}
	return c
}

// Name is the connection name.
func (c Config) Name() string { return c.Link.Name }

// ConnNumber is the protocolConnectionNumber.
func (c Config) ConnNumber() float64 { return c.Link.ProtocolConnectionNumber }

// RegType is the go-modbus register type of the device map.
func (c Config) RegType() modbus.RegType {
	if c.RegisterType == RegInput {
		return modbus.InputRegister
	}
	return modbus.HoldingRegister
}

// DiscoverOptions are the go-sunspec discovery options for a unit.
func (c Config) DiscoverOptions(unit int) *sunspec.DiscoverOptions {
	return &sunspec.DiscoverOptions{
		UnitID:        uint8(unit),
		RegType:       c.RegType(),
		BaseAddresses: c.BaseAddresses,
		MaxModels:     c.MaxModels,
	}
}

// wantsModel reports whether auto creation covers a model.
func (c Config) wantsModel(id uint16) bool {
	if len(c.ModelIDs) == 0 {
		return true
	}
	for _, m := range c.ModelIDs {
		if m == int(id) {
			return true
		}
	}
	return false
}

// numArr reads an array of numbers; a single number is an array of one.
func numArr(doc bson.M, key string) []float64 {
	v, ok := doc[key]
	if !ok || v == nil {
		return nil
	}
	var items []any
	switch a := v.(type) {
	case bson.A:
		items = a
	case []any:
		items = a
	default:
		items = []any{a}
	}
	out := make([]float64, 0, len(items))
	for _, it := range items {
		if f := drv.ToNum(it, true, math.NaN()); !math.IsNaN(f) && !math.IsInf(f, 0) {
			out = append(out, f)
		}
	}
	return out
}

func appendUnique(s []int, v int) []int {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}
