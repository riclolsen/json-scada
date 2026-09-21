/*
 * DNP3 Client and Server Protocol drivers for {json:scada}, in Go.
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

// Quality conversion in both directions: the DNP3 flags octet as the client
// reports it into sourceDataUpdate, and the tag quality the server puts back on
// the wire.

package dnp3util

import dnp3 "github.com/dscsystems/go-dnp3"

// Quality is the set of quality bits the client extracts from a measurement.
// The upper three bits of the DNP3 flags octet mean different things per
// measurement type, so the type-specific decoding happens at the call site and
// this struct holds the result.
type Quality struct {
	Online         bool
	CommLost       bool
	RemoteForced   bool
	LocalForced    bool
	Overrange      bool
	Rollover       bool
	ReferenceError bool
	Transient      bool
}

// CommonQuality reads the five type-independent bits.
func CommonQuality(f dnp3.Flags) Quality {
	return Quality{
		Online:       f.Has(dnp3.Online),
		CommLost:     f.Has(dnp3.CommLost),
		RemoteForced: f.Has(dnp3.RemoteForced),
		LocalForced:  f.Has(dnp3.LocalForced),
	}
}

// AnalogQuality adds the bits that mean over-range and reference error on an
// analog input or analog output status.
func AnalogQuality(f dnp3.Flags) Quality {
	q := CommonQuality(f)
	q.Overrange = f.Has(dnp3.OverRange)
	q.ReferenceError = f.Has(dnp3.ReferenceErr)
	return q
}

// CounterQuality adds the bit that means rollover on a counter or frozen
// counter.
func CounterQuality(f dnp3.Flags) Quality {
	q := CommonQuality(f)
	q.Rollover = f.Has(dnp3.Rollover)
	return q
}

// The sourceDataUpdate quality fields, derived exactly as the C++ client
// derives them in processMongo().

// NotTopical reports the notTopicalAtSource flag.
func (q Quality) NotTopical() bool { return q.CommLost }

// Invalid reports the invalidAtSource flag.
func (q Quality) Invalid() bool { return q.CommLost || q.ReferenceError || !q.Online }

// Overflow reports the overflowAtSource flag.
func (q Quality) Overflow() bool { return q.Overrange }

// Blocked reports the blockedAtSource flag.
func (q Quality) Blocked() bool { return !q.Online }

// Substituted reports the substitutedAtSource flag.
func (q Quality) Substituted() bool { return q.RemoteForced || q.LocalForced }

// Carry reports the carryAtSource flag.
func (q Quality) Carry() bool { return q.Rollover }

// TagQuality is the quality of a tag as the server reads it out of
// realtimeData, on its way to a DNP3 measurement.
type TagQuality struct {
	Invalid     bool
	Transient   bool
	Substituted bool
	Overflow    bool
}

// Flags renders the tag quality as a DNP3 flags octet.
//
// Port of the flags computation repeated in every branch of ConvertValue():
// online unless the tag is invalid, comm-lost when it is, locally forced when
// substituted, plus the type-specific bit for an overflow.
//
// countsTransient distinguishes the families whose ONLINE test includes the
// transient flag (binary, counter, analog) from double-bit binary, where a
// transient value is expressed in the value itself and does not make the point
// comm-lost.
func (q TagQuality) Flags(countsTransient bool, overflowBit dnp3.Flags) dnp3.Flags {
	var f dnp3.Flags
	if q.Invalid || (countsTransient && q.Transient) {
		f = dnp3.CommLost
	} else {
		f = dnp3.Online
	}
	if q.Substituted {
		f |= dnp3.LocalForced
	}
	if q.Overflow && overflowBit != 0 {
		f |= overflowBit
	}
	return f
}
