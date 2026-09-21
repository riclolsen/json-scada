/*
 * DNP3 Outstation Server Protocol driver for {json:scada}, in Go.
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

// A realtimeData document into an outstation database update. Port of
// ConvertValue().

package serverapp

import (
	"time"

	"dnp3-go/internal/dnp3util"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/outstation"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// timestampFor derives the DNP3 timestamp of a measurement.
//
// The source timestamp is preferred; without one the server's own is used and
// the reading is marked unsynchronised. The C++ server marks that case INVALID,
// but dnp3.TimestampInvalid discards the time entirely, and a measurement with
// no time at all is worse for a master than one honestly labelled unsynchronised
// (deviation D15).
func timestampFor(doc bson.M, dest Destination, connHoursShift float64) dnp3.Timestamp {
	shift := time.Duration((dest.HoursShift + connHoursShift) * float64(time.Hour))

	if ms := jsmongo.GetDateMs(doc, "timeTagAtSource", 0); ms != 0 {
		t := time.UnixMilli(ms).Add(shift)
		if jsmongo.GetBool(doc, "timeTagAtSourceOk", false) {
			return dnp3.Now(t)
		}
		return dnp3.Unsynchronized(t)
	}

	ms := jsmongo.GetDateMs(doc, "timeTag", 0)
	if ms == 0 {
		ms = time.Now().UnixMilli()
	}
	return dnp3.Unsynchronized(time.UnixMilli(ms).Add(shift))
}

// tagQuality reads the quality flags of a tag.
func tagQuality(doc bson.M) dnp3util.TagQuality {
	return dnp3util.TagQuality{
		Invalid:     jsmongo.GetBool(doc, "invalid", false),
		Transient:   jsmongo.GetBool(doc, "transient", false),
		Substituted: jsmongo.GetBool(doc, "substituted", false),
		Overflow:    jsmongo.GetBool(doc, "overflow", false),
	}
}

// scaled applies the destination's conversion factors.
func scaled(v float64, dest Destination) float64 {
	if dest.KConv1 != 1.0 || dest.KConv2 != 0.0 {
		return v*dest.KConv1 + dest.KConv2
	}
	return v
}

// ApplyValue writes one tag to one destination of the outstation database.
//
// It is called inside Session.Update, so every destination touched by one
// change event lands in a single atomic update and a master cannot read a torn
// set.
func ApplyValue(db *outstation.Database, doc bson.M, dest Destination, conn *Connection) {
	ts := timestampFor(doc, dest, conn.HoursShift)
	q := tagQuality(doc)
	index := uint16(dest.ObjectAddress)
	fam := familyOf(dest.CommonAddress)

	logIt := func(value any) {
		jslog.Log(jslog.LevelBasic, "Updating tag: %s %s Address: %d Group: %d Value: %v",
			jsmongo.FormatID(jsmongo.GetDouble(doc, "_id", 0)),
			jsmongo.GetString(doc, "tag", ""),
			dest.ObjectAddress, dest.CommonAddress, value)
	}

	switch fam {
	case famBinary:
		v := boolValue(doc, dest)
		db.UpdateBinary(index, dnp3.Binary{
			Value: v, Flags: q.Flags(true, 0), Time: ts,
		})
		logIt(v)

	case famDoubleBit:
		v := boolValue(doc, dest)
		state := dnp3.DoubleBitDeterminedOff
		switch {
		case q.Transient && v:
			state = dnp3.DoubleBitIndeterminate
		case q.Transient:
			state = dnp3.DoubleBitIntermediate
		case v:
			state = dnp3.DoubleBitDeterminedOn
		}
		// parity: a double-bit point expresses movement in its value, so the
		// transient flag does not make it comm-lost.
		db.UpdateDoubleBit(index, dnp3.DoubleBitBinary{
			Value: state, Flags: q.Flags(false, 0), Time: ts,
		})
		logIt(v)

	case famCounter:
		v := scaled(jsmongo.GetDouble(doc, "value", 0), dest)
		db.UpdateCounter(index, dnp3.Counter{
			Value: uint32(v), Flags: q.Flags(true, dnp3.Rollover), Time: ts,
		})
		logIt(v)

	case famFrozenCounter:
		// The C++ server falls through into the counter branch here, writing a
		// frozen counter into the counter array. The Go database keeps the two
		// families apart, so the point is written where it was sized
		// (deviation D16).
		v := scaled(jsmongo.GetDouble(doc, "value", 0), dest)
		db.UpdateFrozenCounter(index, dnp3.FrozenCounter{
			Value: uint32(v), Flags: q.Flags(true, dnp3.Rollover), Time: ts,
		})
		logIt(v)

	case famBinaryOutputStatus:
		v := boolValue(doc, dest)
		db.UpdateBinaryOutputStatus(index, dnp3.BinaryOutputStatus{
			Value: v, Flags: q.Flags(true, 0), Time: ts,
		})
		logIt(v)

	case famAnalogOutputStatus:
		v := scaled(jsmongo.GetDouble(doc, "value", 0), dest)
		db.UpdateAnalogOutputStatus(index, dnp3.AnalogOutputStatus{
			Value: v, Flags: q.Flags(true, dnp3.OverRange), Time: ts,
		})
		logIt(v)

	case famOctetString, famTimeAndInterval:
		// Not distributed. The C++ server has both branches commented out, and
		// go-dnp3's database has no time-and-interval family at all
		// (gap F1, deviation D6). Reported once per point at load, not here,
		// so a fast-changing tag cannot flood the log.

	default: // famAnalog
		v := scaled(jsmongo.GetDouble(doc, "value", 0), dest)
		db.UpdateAnalog(index, dnp3.Analog{
			Value: v, Flags: q.Flags(true, dnp3.OverRange), Time: ts,
		})
		logIt(v)
	}
}

// boolValue reads a digital tag's value, inverted when the destination asks
// for it.
func boolValue(doc bson.M, dest Destination) bool {
	v := jsmongo.GetBool(doc, "value", false)
	if dest.KConv1 == -1.0 {
		return !v
	}
	return v
}
