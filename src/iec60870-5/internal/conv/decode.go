/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - ASDU decoding
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Port of the client AsduReceiveHandler.cs: decodes monitor-direction ASDUs
 * into IecValue entries and command confirmations into IecCmdAck entries.
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

// DecodeResult carries everything extracted from one received ASDU.
type DecodeResult struct {
	Values  []IecValue
	Acks    []IecCmdAck
	Selects []SelectConfirm
}

type timeClass int

const (
	timeNone timeClass = iota
	timeCP24
	timeCP56
)

func timeClassOf(t asdu.TypeID) timeClass {
	switch t {
	case asdu.M_SP_TA_1, asdu.M_DP_TA_1, asdu.M_ST_TA_1, asdu.M_BO_TA_1,
		asdu.M_ME_TA_1, asdu.M_ME_TB_1, asdu.M_ME_TC_1, asdu.M_IT_TA_1,
		asdu.M_EP_TA_1, asdu.M_EP_TB_1, asdu.M_EP_TC_1:
		return timeCP24
	case asdu.M_SP_TB_1, asdu.M_DP_TB_1, asdu.M_ST_TB_1, asdu.M_BO_TB_1,
		asdu.M_ME_TD_1, asdu.M_ME_TE_1, asdu.M_ME_TF_1, asdu.M_IT_TB_1,
		asdu.M_EP_TD_1, asdu.M_EP_TE_1, asdu.M_EP_TF_1:
		return timeCP56
	}
	return timeNone
}

// Decode extracts values, command acks and SBO select confirmations from a
// received ASDU, replicating the C# client AsduReceiveHandler behavior.
func Decode(pack *asdu.ASDU, connNumber int) DecodeResult {
	var res DecodeResult
	now := time.Now()
	ca := int(pack.CommonAddr)
	cot := int(pack.Coa.Cause)
	tc := timeClassOf(pack.Type)

	add := func(ioa asdu.InfoObjAddr, isDigital bool, value float64, q Quality, t time.Time) {
		iv := IecValue{
			Address:         int(ioa),
			Asdu:            pack.Type,
			IsDigital:       isDigital,
			Value:           value,
			Cot:             cot,
			ServerTimestamp: now,
			Quality:         q,
			ConnNumber:      connNumber,
			CommonAddress:   ca,
		}
		switch tc {
		case timeCP24:
			// C# parity quirk: CP24-tagged values never mark the source
			// timestamp as OK (the original driver tests the unused CP56
			// placeholder, which is always invalid).
			iv.HasSourceTimestamp = !t.IsZero()
			iv.SourceTimestamp = t
			iv.TimestampOk = false
		case timeCP56:
			iv.HasSourceTimestamp = !t.IsZero()
			iv.SourceTimestamp = t
			iv.TimestampOk = !t.IsZero()
		}
		res.Values = append(res.Values, iv)
	}

	ack := func(ioa asdu.InfoObjAddr) {
		res.Acks = append(res.Acks, IecCmdAck{
			Ack:           !pack.Coa.IsNegative,
			ConnNumber:    connNumber,
			ObjectAddress: int(ioa),
			AckTimeTag:    now,
		})
	}

	selChase := func(ioa asdu.InfoObjAddr, inSelect bool) {
		if inSelect && pack.Coa.Cause == asdu.ActivationCon && !pack.Coa.IsNegative {
			res.Selects = append(res.Selects, SelectConfirm{Ca: ca, Ioa: int(ioa), Type: pack.Type})
		}
	}

	switch pack.Type {
	case asdu.M_SP_NA_1, asdu.M_SP_TA_1, asdu.M_SP_TB_1: // 1, 2, 30
		for _, p := range pack.GetSinglePoint() {
			v := 0.0
			if p.Value {
				v = 1.0
			}
			add(p.Ioa, true, v, QualityFromQds(p.Qds), p.Time)
		}
	case asdu.M_DP_NA_1, asdu.M_DP_TA_1, asdu.M_DP_TB_1: // 3, 4, 31
		for _, p := range pack.GetDoublePoint() {
			add(p.Ioa, true, float64(p.Value), QualityFromQds(p.Qds), p.Time)
		}
	case asdu.M_ST_NA_1, asdu.M_ST_TA_1, asdu.M_ST_TB_1: // 5, 6, 32
		for _, p := range pack.GetStepPosition() {
			q := QualityFromQds(p.Qds)
			// C# merges the transient flag into invalid quality
			q.Invalid = q.Invalid || p.Value.HasTransient
			add(p.Ioa, false, float64(p.Value.Val), q, p.Time)
		}
	case asdu.M_BO_NA_1, asdu.M_BO_TA_1, asdu.M_BO_TB_1: // 7, 8, 33
		for _, p := range pack.GetBitString32() {
			// C# parity: bitstrings are stored with all-good quality
			add(p.Ioa, false, float64(p.Value), Quality{}, p.Time)
		}
	case asdu.M_ME_NA_1, asdu.M_ME_TA_1, asdu.M_ME_TD_1: // 9, 10, 34
		for _, p := range pack.GetMeasuredValueNormal() {
			add(p.Ioa, false, p.Value.Float64(), QualityFromQds(p.Qds), p.Time)
		}
	case asdu.M_ME_ND_1: // 21 (no quality)
		for _, p := range pack.GetMeasuredValueNormal() {
			add(p.Ioa, false, p.Value.Float64(), Quality{}, p.Time)
		}
	case asdu.M_ME_NB_1, asdu.M_ME_TB_1, asdu.M_ME_TE_1: // 11, 12, 35
		for _, p := range pack.GetMeasuredValueScaled() {
			add(p.Ioa, false, float64(p.Value), QualityFromQds(p.Qds), p.Time)
		}
	case asdu.M_ME_NC_1, asdu.M_ME_TC_1, asdu.M_ME_TF_1: // 13, 14, 36
		for _, p := range pack.GetMeasuredValueFloat() {
			add(p.Ioa, false, float64(p.Value), QualityFromQds(p.Qds), p.Time)
		}
	case asdu.M_IT_NA_1, asdu.M_IT_TA_1, asdu.M_IT_TB_1: // 15, 16, 37
		for _, p := range pack.GetIntegratedTotals() {
			// C# parity: integrated totals are stored with all-good quality
			add(p.Ioa, false, float64(p.Value.CounterReading), Quality{}, p.Time)
		}
	case asdu.M_EP_TA_1, asdu.M_EP_TD_1: // 17, 38
		for _, p := range pack.GetEventOfProtectionEquipment() {
			v := 0.0
			if p.Event == asdu.SEDeterminedOn {
				v = 1.0
			}
			add(p.Ioa, true, v, QualityFromQdp(p.Qdp), p.Time)
		}
	case asdu.M_EP_TB_1, asdu.M_EP_TE_1: // 18, 39
		p := pack.GetPackedStartEventsOfProtectionEquipment()
		add(p.Ioa, true, 1.0, QualityFromQdp(p.Qdp), p.Time)
	case asdu.M_EP_TC_1, asdu.M_EP_TF_1: // 19, 40
		p := pack.GetPackedOutputCircuitInfo()
		add(p.Ioa, true, 1.0, QualityFromQdp(p.Qdp), p.Time)
	case asdu.M_PS_NA_1: // 20
		for _, p := range pack.GetPackedSinglePointWithSCD() {
			// C# parity: packed single points are stored with all-good quality
			add(p.Ioa, false, float64(p.Scd), Quality{}, time.Time{})
		}

	// ---- command confirmations (control direction mirrored back) ----
	case asdu.C_SC_NA_1, asdu.C_SC_TA_1: // 45, 58
		cmd := pack.GetSingleCmd()
		selChase(cmd.Ioa, cmd.Qoc.InSelect)
		ack(cmd.Ioa)
	case asdu.C_DC_NA_1, asdu.C_DC_TA_1: // 46, 59
		cmd := pack.GetDoubleCmd()
		selChase(cmd.Ioa, cmd.Qoc.InSelect)
		ack(cmd.Ioa)
	case asdu.C_RC_NA_1, asdu.C_RC_TA_1: // 47, 60
		cmd := pack.GetStepCmd()
		selChase(cmd.Ioa, cmd.Qoc.InSelect)
		ack(cmd.Ioa)
	case asdu.C_SE_NA_1, asdu.C_SE_TA_1: // 48, 61
		ack(pack.GetSetpointNormalCmd().Ioa)
	case asdu.C_SE_NB_1, asdu.C_SE_TB_1: // 49, 62
		ack(pack.GetSetpointCmdScaled().Ioa)
	case asdu.C_SE_NC_1, asdu.C_SE_TC_1: // 50, 63
		ack(pack.GetSetpointFloatCmd().Ioa)
	case asdu.C_BO_NA_1, asdu.C_BO_TA_1: // 51, 64
		ack(pack.GetBitsString32Cmd().Ioa)
	case asdu.P_ME_NA_1: // 110
		ack(pack.GetParameterNormal().Ioa)
	case asdu.P_ME_NB_1: // 111
		ack(pack.GetParameterScaled().Ioa)
	case asdu.P_ME_NC_1: // 112
		ack(pack.GetParameterFloat().Ioa)
	case asdu.P_AC_NA_1: // 113
		ack(pack.GetParameterActivation().Ioa)

	case asdu.M_EI_NA_1: // 70 end of initialization: log only (caller)
	default:
		// other system-command confirmations (C_IC/C_CI/C_RD/C_CS/C_TS/...):
		// logged by the caller, no data extracted (C# parity)
	}
	return res
}
