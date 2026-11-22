/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - shared conversions
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
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

package conv

import (
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
)

// Quality is the protocol-neutral quality descriptor used across the drivers.
type Quality struct {
	Invalid     bool
	NonTopical  bool
	Substituted bool
	Blocked     bool
	Overflow    bool
}

// ToQds converts to a go-iecp5 quality descriptor.
func (q Quality) ToQds() asdu.QualityDescriptor {
	var d asdu.QualityDescriptor
	if q.Invalid {
		d |= asdu.QDSInvalid
	}
	if q.NonTopical {
		d |= asdu.QDSNotTopical
	}
	if q.Substituted {
		d |= asdu.QDSSubstituted
	}
	if q.Blocked {
		d |= asdu.QDSBlocked
	}
	if q.Overflow {
		d |= asdu.QDSOverflow
	}
	return d
}

// QualityFromQds converts a go-iecp5 quality descriptor.
func QualityFromQds(d asdu.QualityDescriptor) Quality {
	return Quality{
		Invalid:     d&asdu.QDSInvalid != 0,
		NonTopical:  d&asdu.QDSNotTopical != 0,
		Substituted: d&asdu.QDSSubstituted != 0,
		Blocked:     d&asdu.QDSBlocked != 0,
		Overflow:    d&asdu.QDSOverflow != 0,
	}
}

// QualityFromQdp converts a protection-equipment quality descriptor
// (same field mapping as the C# drivers: overflow not represented).
func QualityFromQdp(d asdu.QualityDescriptorProtection) Quality {
	return Quality{
		Invalid:     d&asdu.QDPInvalid != 0,
		NonTopical:  d&asdu.QDPNotTopical != 0,
		Substituted: d&asdu.QDPSubstituted != 0,
		Blocked:     d&asdu.QDPBlocked != 0,
	}
}

// IecValue is one acquired value to be updated in the realtimeData collection
// (port of the C# IEC_Value struct).
type IecValue struct {
	Address int
	Asdu    asdu.TypeID
	// AsduStr, when non-empty, overrides Asdu.String() for the
	// asduAtSource field (used by the IEC 103 client whose type
	// identifications are not asdu.TypeID values).
	AsduStr            string
	IsDigital          bool
	Value              float64
	Cot                int
	ServerTimestamp    time.Time
	HasSourceTimestamp bool
	SourceTimestamp    time.Time // zero when invalid/absent
	// TimestampOk replicates the C# semantics: true only for CP56-tagged
	// types with a valid time; always false for CP24-tagged types.
	TimestampOk   bool
	Quality       Quality
	ConnNumber    int
	CommonAddress int
}

// IecCmdAck is a command confirmation to be written back to commandsQueue
// (port of the C# IEC_CmdAck struct).
type IecCmdAck struct {
	Ack           bool
	ConnNumber    int
	ObjectAddress int
	AckTimeTag    time.Time
}

// SelectConfirm signals a positive select confirmation received for an SBO
// command; the driver reacts by sending the stored execute twin.
type SelectConfirm struct {
	Ca   int
	Ioa  int
	Type asdu.TypeID // of the confirmed command
}

// InfoObject is the protocol-neutral output of BuildInfoObj: a single
// information object ready to be sent with SendInfoBatch or SendCommand.
type InfoObject struct {
	TypeID    asdu.TypeID
	Ioa       int
	Qualifier int         // QOI/QCC/QRP/QPA/qualifier for system+parameter commands
	Info      interface{} // one of the asdu.*Info structs (nil for system commands)
	Time      time.Time   // time used by time-tagged variants
}

// MapAsduToBaseType normalizes time-tagged and short TypeIDs to their
// canonical base types (port of TagsCreation.cs).
func MapAsduToBaseType(t asdu.TypeID) int {
	switch int(t) {
	case 2, 30:
		return 1 // Single Point
	case 4, 31:
		return 3 // Double Point
	case 6, 32:
		return 5 // Step Position
	case 8, 33:
		return 7 // Bitstring 32
	case 10, 21, 34:
		return 9 // Measured Normalized
	case 12, 35:
		return 11 // Measured Scaled
	case 14, 36:
		return 13 // Measured Float
	case 16, 37:
		return 15 // Integrated Totals
	default:
		return int(t)
	}
}

// TypeDescription returns a human description for a base type
// (port of TagsCreation.cs Iec10xTypeDescription).
func TypeDescription(baseType int) string {
	switch baseType {
	case 1:
		return "Single Point"
	case 3:
		return "Double Point"
	case 5:
		return "Step Position"
	case 7:
		return "Bitstring 32"
	case 9:
		return "Measured Normalized"
	case 11:
		return "Measured Scaled"
	case 13:
		return "Measured Float"
	case 15:
		return "Integrated Totals"
	case 20:
		return "Packed Single Point"
	default:
		return "ASDU " + itoa(baseType)
	}
}

func itoa(i int) string {
	// tiny local helper to avoid strconv import churn in callers
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}
