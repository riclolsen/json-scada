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

// protocolDestinationASDU to static and event variation. Port of
// DefineGroupVar(), plus the family-wide defaults the C++ server applies to
// every point before it.

package serverapp

import (
	dnp3 "github.com/dscsystems/go-dnp3"
)

// family is one point type of the outstation database.
type family int

const (
	famNone family = iota
	famBinary
	famDoubleBit
	famCounter
	famFrozenCounter
	famBinaryOutputStatus
	famAnalog
	famAnalogOutputStatus
	famOctetString
	// famTimeAndInterval has no equivalent in go-dnp3's DatabaseConfig, so
	// group 50/52 destinations are dropped with a warning (deviation D6).
	famTimeAndInterval
)

// familyOf maps a protocolDestinationCommonAddress onto a point family. The
// pairs are the static and the event group of each type.
func familyOf(commonAddress int) family {
	switch commonAddress {
	case 1, 2:
		return famBinary
	case 3, 4:
		return famDoubleBit
	case 20, 22:
		return famCounter
	case 21, 23:
		return famFrozenCounter
	case 10, 11:
		return famBinaryOutputStatus
	case 40, 42:
		return famAnalogOutputStatus
	case 110, 111:
		return famOctetString
	case 50, 52:
		return famTimeAndInterval
	case 30, 32:
		return famAnalog
	default:
		// parity: the C++ server's switch defaults to the analog input branch,
		// so an unrecognised common address is distributed as an analog.
		return famAnalog
	}
}

// pointType maps a family onto the library's point type, for Database.Configure.
func (f family) pointType() (dnp3.PointType, bool) {
	switch f {
	case famBinary:
		return dnp3.TypeBinary, true
	case famDoubleBit:
		return dnp3.TypeDoubleBitBinary, true
	case famCounter:
		return dnp3.TypeCounter, true
	case famFrozenCounter:
		return dnp3.TypeFrozenCounter, true
	case famBinaryOutputStatus:
		return dnp3.TypeBinaryOutputStatus, true
	case famAnalog:
		return dnp3.TypeAnalog, true
	case famAnalogOutputStatus:
		return dnp3.TypeAnalogOutputStatus, true
	case famOctetString:
		return dnp3.TypeOctetString, true
	default:
		return dnp3.TypeUnknown, false
	}
}

// variationPair is a static and an event variation.
type variationPair struct {
	static uint8
	event  uint8
}

// defaultVariations are the family-wide settings the C++ server applies to
// every point of the database before the per-point pass.
//
// The analog defaults matter more than they look: go-dnp3's own defaults are
// the 32-bit integer variations, so an analog left unconfigured would truncate
// 123.5 to 123. g30v5 is single precision, which is what the C++ server uses.
var defaultVariations = map[family]variationPair{
	famBinary:             {static: 2, event: 2}, // g1v2, g2v2
	famDoubleBit:          {static: 2, event: 2}, // g3v2, g4v2
	famAnalog:             {static: 5, event: 7}, // g30v5, g32v7
	famCounter:            {static: 1, event: 5}, // g20v1, g22v5
	famFrozenCounter:      {static: 1, event: 5}, // g21v1, g23v5
	famBinaryOutputStatus: {static: 2, event: 2}, // g10v2, g11v2
	famAnalogOutputStatus: {static: 3, event: 7}, // g40v3, g42v7
	famOctetString:        {static: 0, event: 0}, // g110v0, g111v0
}

// defaultClasses are the event classes the C++ server assigns per family.
var defaultClasses = map[family]dnp3.Class{
	famBinary:             dnp3.Class1,
	famDoubleBit:          dnp3.Class2,
	famAnalog:             dnp3.Class2,
	famCounter:            dnp3.Class2,
	famFrozenCounter:      dnp3.Class3,
	famBinaryOutputStatus: dnp3.Class2,
	famAnalogOutputStatus: dnp3.Class2,
	famOctetString:        dnp3.Class3,
}

// variationsFor returns the static and event variation a destination's ASDU
// selects. Port of the switch tables of DefineGroupVar().
func variationsFor(f family, asdu int) variationPair {
	switch f {
	case famAnalog:
		switch asdu {
		case 1:
			return variationPair{1, 1}
		case 2:
			return variationPair{2, 2}
		case 3:
			return variationPair{3, 3}
		case 4:
			return variationPair{4, 4}
		case 6:
			return variationPair{6, 6}
		case 7:
			return variationPair{5, 7}
		case 8:
			return variationPair{6, 8}
		default: // including 5
			return variationPair{5, 5}
		}

	case famCounter:
		switch asdu {
		case 2, 4:
			return variationPair{2, 6}
		case 5, 7:
			return variationPair{5, 5}
		case 6, 8:
			return variationPair{6, 6}
		default: // including 1 and 3
			return variationPair{1, 5}
		}

	case famFrozenCounter:
		switch asdu {
		case 2, 4, 10, 12:
			return variationPair{2, 6}
		case 5, 7:
			return variationPair{5, 5}
		case 6, 8:
			return variationPair{6, 6}
		default: // including 1, 3, 9 and 11
			return variationPair{1, 5}
		}

	case famAnalogOutputStatus:
		switch asdu {
		case 1:
			return variationPair{1, 3}
		case 2:
			return variationPair{2, 4}
		case 4:
			return variationPair{4, 8}
		default: // including 3
			return variationPair{3, 7}
		}

	default:
		// Binary, double-bit, binary output status and octet string carry one
		// variation each, and the ASDU is ignored for them.
		return defaultVariations[f]
	}
}
