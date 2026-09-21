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

// Package jstags holds the parts of automatic tag creation that are the same
// in every driver: the _id partition per connection, and the block of
// realtimeData defaults that every auto-created tag carries.
//
// Scope note: the identifying half of a tag document — tag name, description,
// group1/2/3, type, alarmState, the state and event texts, and every
// protocolSource* field — is protocol-specific and stays with the driver.
// So does the digital/string/json/analog/command shape logic, which encodes
// per-driver parity quirks against the C#/C++ originals.
package jstags

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// DefaultKeyMultiplier is the size of the _id range reserved per connection
// for automatically created tags. Every driver uses this value.
const DefaultKeyMultiplier = 1000000

// BaseDoc returns the realtimeData fields that are identical in every driver's
// automatically created tag, as a fresh map for the caller to fill in and
// override.
//
// Deliberately NOT included, because the drivers disagree and the difference
// is meaningful:
//
//   - invalid / invalidDetectTimeout — a supervised point starts invalid with
//     a 60 s detect timeout; the IEC 61850 command tag starts valid with none.
//   - protocolDestinations — bson.A{} in most drivers, nil in the IEC 61850
//     command tag.
//   - value / valueString / valueJson / type / alarmState / the state and
//     event texts — set by the per-shape logic of each driver.
//
// Every field here was verified identical across OPC-UA-Client-Go, dnp3-go,
// the IEC 61850 client and iec60870-5 before being promoted.
func BaseDoc() bson.M {
	return bson.M{
		"alarmDisabled":        false,
		"alerted":              false,
		"alarmed":              false,
		"alertState":           "",
		"annotation":           "",
		"commandBlocked":       false,
		"commissioningRemarks": "",
		"formula":              0.0,
		"frozen":               false,
		"frozenDetectTimeout":  0.0,
		"hiLimit":              math.MaxFloat64,
		"hihiLimit":            math.MaxFloat64,
		"hihihiLimit":          math.MaxFloat64,
		"historianDeadBand":    0.0,
		"historianPeriod":      0.0,
		"hysteresis":           0.0,
		"isEvent":              false,
		"kconv1":               1.0,
		"kconv2":               0.0,
		"loLimit":              -math.MaxFloat64,
		"location":             nil,
		"loloLimit":            -math.MaxFloat64,
		"lololoLimit":          -math.MaxFloat64,
		"notes":                "",
		"overflow":             false,
		"parcels":              nil,
		"priority":             0.0,
		"sourceDataUpdate":     nil,
		"substituted":          false,
		"timeTag":              nil,
		"timeTagAlarm":         nil,
		"timeTagAtSource":      nil,
		"timeTagAtSourceOk":    false,
		"transient":            false,
		"unit":                 "",
		"updatesCnt":           0.0,
		"valueDefault":         0.0,
		"zeroDeadband":         0.0,
	}
}

// KeyAllocator hands out _id values inside the range reserved for one
// connection: [connNumber * Multiplier, (connNumber+1) * Multiplier).
//
// The first call queries the highest key already used in the range and
// continues from there, so tags created by an earlier run — or by the other
// node of a redundant pair — are not overwritten. Later calls just increment.
type KeyAllocator struct {
	// Multiplier defaults to DefaultKeyMultiplier when zero.
	Multiplier float64
	// Timeout bounds the lookup. Defaults to DefaultLookupTimeout.
	Timeout time.Duration

	mu   sync.Mutex
	last float64
}

// DefaultLookupTimeout is the budget every driver allowed for the lookup.
const DefaultLookupTimeout = 10 * time.Second

// Base is the first key of a connection's range.
func (k *KeyAllocator) Base(connNumber int) float64 {
	return float64(connNumber) * k.multiplier()
}

// Top is the first key past a connection's range.
func (k *KeyAllocator) Top(connNumber int) float64 {
	return float64(connNumber+1) * k.multiplier()
}

func (k *KeyAllocator) multiplier() float64 {
	if k.Multiplier == 0 {
		return DefaultKeyMultiplier
	}
	return k.Multiplier
}

// Next returns the next free _id for a connection.
//
// parity: a failed lookup starts the range at its base, which is what every
// driver does — an empty collection and an unreachable one are treated alike,
// and the insert that follows would fail anyway if the database were down.
func (k *KeyAllocator) Next(ctx context.Context, coll *mongo.Collection, connNumber int) float64 {
	k.mu.Lock()
	defer k.mu.Unlock()

	if k.last != 0 {
		k.last++
		return k.last
	}

	timeout := k.Timeout
	if timeout == 0 {
		timeout = DefaultLookupTimeout
	}
	fctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	base := k.Base(connNumber)
	k.last = base

	var doc bson.M
	err := coll.FindOne(fctx,
		bson.M{"_id": bson.M{"$gt": base, "$lt": k.Top(connNumber)}},
		options.FindOne().SetSort(bson.M{"_id": -1}),
	).Decode(&doc)
	if err == nil {
		// Permissive decode: the drivers all read the key back with the
		// same tolerance they read every other number with.
		k.last = jsmongo.GetDouble(doc, "_id", base) + 1
	}
	return k.last
}

// Reset forgets the cached key so the next call queries the collection again.
func (k *KeyAllocator) Reset() {
	k.mu.Lock()
	k.last = 0
	k.mu.Unlock()
}
