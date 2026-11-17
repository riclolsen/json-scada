/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - info object builder
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Port of Common_srv_cli.cs BuildInfoObj: builds a protocol-neutral
 * information object from a json-scada value, applying kconv scaling and
 * inversion, range clamping with overflow flagging, and the automatic
 * upgrade of monitor TypeIDs to their CP56Time2a variants when a source
 * timestamp is present.
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
	"math"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
)

// mapToWithCP56Time upgrades monitor TypeIDs to time-tagged variants
// (identical to the C# map).
var mapToWithCP56Time = map[int]int{
	1:  30, // M_SP_NA_1 -> M_SP_TB_1
	3:  31, // M_DP_NA_1 -> M_DP_TB_1
	5:  32, // M_ST_NA_1 -> M_ST_TB_1
	7:  33, // M_BO_NA_1 -> M_BO_TB_1
	9:  34, // M_ME_NA_1 -> M_ME_TD_1
	11: 35, // M_ME_NB_1 -> M_ME_TE_1
	13: 36, // M_ME_NC_1 -> M_ME_TF_1
	15: 37, // M_IT_NA_1 -> M_IT_TB_1
}

// Bits32 converts a numeric value to the 32 bits of a bitstring. Values in
// [2^31, 2^32) keep their high bit and negative values in [-2^31, 0) map to
// their two's complement (a float64 to int32 conversion of either is
// implementation-defined in Go). Out-of-range values saturate, NaN is 0.
func Bits32(v float64) uint32 {
	switch {
	case math.IsNaN(v):
		return 0
	case v >= math.MaxUint32:
		return math.MaxUint32
	case v <= math.MinInt32:
		return 1 << 31
	}
	return uint32(int64(v))
}

// clamp rounds v to the nearest integer and limits it to [lo, hi], flagging
// overflow in q when it had to be limited (NaN becomes 0, flagged invalid).
func clamp(v float64, lo, hi float64, q *Quality) float64 {
	if math.IsNaN(v) {
		if q != nil {
			q.Invalid = true
		}
		return 0
	}
	v = math.Round(v)
	if v > hi {
		if q != nil {
			q.Overflow = true
		}
		return hi
	}
	if v < lo {
		if q != nil {
			q.Overflow = true
		}
		return lo
	}
	return v
}

// BuildInfoObj builds a single information object for sending.
// Returns nil when the ASDU type is not supported (C# parity: the caller
// treats nil as "asdu not implemented").
//
// quality is taken by value and possibly extended with the overflow flag
// (matching the C# code that mutates the QualityDescriptor on clamping).
// timeTag == nil means "no source timestamp" (time-tagged command variants
// then carry time.Now(), as in C#).
func BuildInfoObj(
	asduNum int,
	addr int,
	value float64,
	sbo bool,
	cmdQualif int,
	quality Quality,
	timeTag *time.Time,
	kconv1 float64,
	kconv2 float64,
	transient bool,
) *InfoObject {
	tt := time.Now()
	if timeTag != nil {
		tt = *timeTag
		// has time tag, so change ASDU if necessary to embed a timetag
		if up, ok := mapToWithCP56Time[asduNum]; ok {
			asduNum = up
		}
	}
	ioa := asdu.InfoObjAddr(addr)
	obj := &InfoObject{TypeID: asdu.TypeID(asduNum), Ioa: addr, Qualifier: cmdQualif, Time: tt}

	boolVal := func() bool {
		b := value != 0
		if kconv1 == -1 {
			return !b
		}
		return b
	}

	switch asduNum {
	case 1, 30: // M_SP_NA_1 / M_SP_TB_1
		obj.Info = asdu.SinglePointInfo{Ioa: ioa, Value: boolVal(), Qds: quality.ToQds(), Time: tt}
	case 3, 31: // M_DP_NA_1 / M_DP_TB_1
		var dp asdu.DoublePoint
		if transient {
			dp = asdu.DPIIndeterminateOrIntermediate
		} else if kconv1 == -1 {
			if value != 0 {
				dp = asdu.DPIDeterminedOff
			} else {
				dp = asdu.DPIDeterminedOn
			}
		} else {
			if value != 0 {
				dp = asdu.DPIDeterminedOn
			} else {
				dp = asdu.DPIDeterminedOff
			}
		}
		obj.Info = asdu.DoublePointInfo{Ioa: ioa, Value: dp, Qds: quality.ToQds(), Time: tt}
	case 5, 32: // M_ST_NA_1 / M_ST_TB_1
		v := clamp(value*kconv1+kconv2, -64, 63, &quality)
		obj.Info = asdu.StepPositionInfo{Ioa: ioa,
			Value: asdu.StepPosition{Val: int(int16(v)), HasTransient: transient},
			Qds:   quality.ToQds(), Time: tt}
	case 7, 33: // M_BO_NA_1 / M_BO_TB_1
		uv := Bits32(value)
		if kconv1 == -1 {
			uv = ^uv
		}
		obj.Info = asdu.BitString32Info{Ioa: ioa, Value: uv, Qds: quality.ToQds(), Time: tt}
	case 9, 34: // M_ME_NA_1 / M_ME_TD_1 (raw normalized int16)
		v := clamp(value*kconv1+kconv2, -32768, 32767, &quality)
		obj.Info = asdu.MeasuredValueNormalInfo{Ioa: ioa, Value: asdu.Normalize(int16(v)), Qds: quality.ToQds(), Time: tt}
	case 21: // M_ME_ND_1 (no quality)
		v := clamp(value*kconv1+kconv2, -32768, 32767, &quality)
		obj.Info = asdu.MeasuredValueNormalInfo{Ioa: ioa, Value: asdu.Normalize(int16(v)), Time: tt}
	case 11, 35: // M_ME_NB_1 / M_ME_TE_1
		v := clamp(value*kconv1+kconv2, -32768, 32767, &quality)
		obj.Info = asdu.MeasuredValueScaledInfo{Ioa: ioa, Value: int16(v), Qds: quality.ToQds(), Time: tt}
	case 13, 36: // M_ME_NC_1 / M_ME_TF_1
		v := value*kconv1 + kconv2
		obj.Info = asdu.MeasuredValueFloatInfo{Ioa: ioa, Value: float32(v), Qds: quality.ToQds(), Time: tt}
	case 45, 58: // C_SC_NA_1 / C_SC_TA_1
		obj.Info = asdu.SingleCommandInfo{Ioa: ioa, Value: boolVal(),
			Qoc: asdu.QualifierOfCommand{Qual: asdu.QOCQual(cmdQualif), InSelect: sbo}, Time: tt}
	case 46, 59: // C_DC_NA_1 / C_DC_TA_1
		var dc asdu.DoubleCommand
		if kconv1 == -1 {
			if value != 0 {
				dc = asdu.DCOOff
			} else {
				dc = asdu.DCOOn
			}
		} else {
			if value != 0 {
				dc = asdu.DCOOn
			} else {
				dc = asdu.DCOOff
			}
		}
		obj.Info = asdu.DoubleCommandInfo{Ioa: ioa, Value: dc,
			Qoc: asdu.QualifierOfCommand{Qual: asdu.QOCQual(cmdQualif), InSelect: sbo}, Time: tt}
	case 47, 60: // C_RC_NA_1 / C_RC_TA_1
		var rc asdu.StepCommand
		if kconv1 == -1 {
			if value >= 1 {
				rc = asdu.SCOStepDown
			} else {
				rc = asdu.SCOStepUP
			}
		} else {
			if value >= 1 {
				rc = asdu.SCOStepUP
			} else {
				rc = asdu.SCOStepDown
			}
		}
		obj.Info = asdu.StepCommandInfo{Ioa: ioa, Value: rc,
			Qoc: asdu.QualifierOfCommand{Qual: asdu.QOCQual(cmdQualif), InSelect: sbo}, Time: tt}
	case 48, 61: // C_SE_NA_1 / C_SE_TA_1 (raw normalized int16, QL fixed 0 as in C#)
		v := clamp(value*kconv1+kconv2, -32768, 32767, nil)
		obj.Info = asdu.SetpointCommandNormalInfo{Ioa: ioa, Value: asdu.Normalize(int16(v)),
			Qos: asdu.QualifierOfSetpointCmd{Qual: 0, InSelect: sbo}, Time: tt}
	case 49, 62: // C_SE_NB_1 / C_SE_TB_1
		v := clamp(value*kconv1+kconv2, -32768, 32767, nil)
		obj.Info = asdu.SetpointCommandScaledInfo{Ioa: ioa, Value: int16(v),
			Qos: asdu.QualifierOfSetpointCmd{Qual: 0, InSelect: sbo}, Time: tt}
	case 50, 63: // C_SE_NC_1 / C_SE_TC_1
		v := value*kconv1 + kconv2
		obj.Info = asdu.SetpointCommandFloatInfo{Ioa: ioa, Value: float32(v),
			Qos: asdu.QualifierOfSetpointCmd{Qual: 0, InSelect: sbo}, Time: tt}
	case 51, 64: // C_BO_NA_1 / C_BO_TA_1
		uv := Bits32(value)
		if kconv1 == -1 {
			uv = ^uv
		}
		obj.Info = asdu.BitsString32CommandInfo{Ioa: ioa, Value: uv, Time: tt}
	case 100: // C_IC_NA_1
		// qualifier carries QOI; IOA fixed 0 (C# parity)
		obj.Ioa = 0
	case 101: // C_CI_NA_1
		obj.Ioa = 0
	case 102: // C_RD_NA_1
	case 103: // C_CS_NA_1
	case 105: // C_RP_NA_1
	case 107: // C_TS_TA_1
	case 110: // P_ME_NA_1
		v := clamp(value*kconv1+kconv2, -32768, 32767, nil)
		obj.Info = asdu.ParameterNormalInfo{Ioa: ioa, Value: asdu.Normalize(int16(v)),
			Qpm: asdu.ParseQualifierOfParamMV(byte(cmdQualif))}
	case 111: // P_ME_NB_1
		v := clamp(value*kconv1+kconv2, -32768, 32767, nil)
		obj.Info = asdu.ParameterScaledInfo{Ioa: ioa, Value: int16(v),
			Qpm: asdu.ParseQualifierOfParamMV(byte(cmdQualif))}
	case 112: // P_ME_NC_1
		v := value*kconv1 + kconv2
		obj.Info = asdu.ParameterFloatInfo{Ioa: ioa, Value: float32(v),
			Qpm: asdu.ParseQualifierOfParamMV(byte(cmdQualif))}
	case 113: // P_AC_NA_1
		obj.Info = asdu.ParameterActivationInfo{Ioa: ioa, Qpa: asdu.QualifierOfParameterAct(cmdQualif)}
	default:
		return nil // not supported (C# parity: command canceled "asdu not implemented")
	}
	return obj
}
