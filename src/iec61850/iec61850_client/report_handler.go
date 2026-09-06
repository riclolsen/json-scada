/*
 * IEC 61850 MMS Client driver for {json:scada}, in Go.
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

// Report reception. Port of reportHandler in the C# driver, including the
// way it derives value, quality, transient state and timestamp from a
// structured MMS value.

package main

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/dscsystems/go-iec61850/client"
	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
)

// buildIECValue turns one MMS value into the record the MongoDB writer
// consumes.
//
// parity note: for a structured value the C# driver computes value,
// quality, transient state and timestamp from the whole structure, then
// recomputes quality, transient state and timestamp once per member inside
// its logging loop — so the *last* member decides them. For the usual
// {stVal, q, t} that means the timestamp member has the last word and
// quality reads as good. That behaviour is reproduced here on purpose:
// tags fed by the two drivers must not disagree.
// The returned timestamp is the one the C# driver would have carried out of
// the same loop: it is the source timestamp of a data-change report.
func buildIECValue(conn *Iec61850Connection, entry *Iec61850Entry, value *mms.Value,
	selfPublish bool, fromReport bool, log *strings.Builder) (IECValue, uint64) {

	var (
		v         float64
		isBinary  bool
		failed    bool
		transient bool
		timestamp uint64
	)

	logging := jslog.Level() > jslog.LevelNoLog && log != nil

	if value != nil && value.Type() == mms.TypeStructure {
		childs := conn.EntryChilds(entry)
		if logging {
			log.WriteString(" Value is of complex type [")
			for _, c := range childs {
				log.WriteString(c + " ")
			}
			log.WriteString("]\n")
		}

		// When the attribute names of the object are known, the value is
		// the one the common data class names — stVal for a status or a
		// double point, mag for a measurand — rather than whichever member
		// happens to have a numeric type.
		named := namedValueAttribute(value, childs)
		if named != nil {
			v, isBinary = mmsGetDoubleVal(named)
		} else {
			v, isBinary = mmsGetNumericVal(value)
		}
		failed = mmsGetQualityFailed(value)
		transient = mmsGetQualityTransient(value)
		timestamp = mmsGetTimestamp(value)

		for i := 0; i < value.Len(); i++ {
			e := value.Index(i)
			if e == nil {
				continue
			}
			if logging {
				fmt.Fprintf(log, "  %s", mmsTypeName(e.Type()))
			}
			if e.Type() == mms.TypeStructure {
				if named == nil {
					v, isBinary = mmsGetNumericVal(e)
				}
				for j := 0; j < e.Len(); j++ {
					g := e.Index(j)
					if g == nil {
						continue
					}
					if logging {
						fmt.Fprintf(log, "  %s     -> %s\n", mmsTypeName(g.Type()), mmsToString(g))
					}
					if named == nil {
						v, isBinary = mmsGetNumericVal(g)
					}
				}
			}
			failed = mmsGetQualityFailed(e)
			transient = mmsGetQualityTransient(e)
			timestamp = mmsGetTimestamp(e)

			if logging {
				switch e.Type() {
				case mms.TypeBitString, mms.TypeUTCTime:
					fmt.Fprintf(log, "   -> %s\n", mmsToString(e))
				default:
					fmt.Fprintf(log, "   -> %s\n", formatDouble(v))
				}
			}
		}

		// A double point says more than its position: 00 is the transient
		// intermediate state and 11 is a faulty one. Those belong in the
		// quality of the tag, and they survive the loop above, which reads
		// quality from the last member of the structure.
		if pos := doublePointAttribute(value, childs); pos != nil {
			if mmsTestDoubleStateFailed(pos) {
				failed = true
			}
			if mmsTestDoubleStateTransient(pos) {
				transient = true
			}
		}
	} else {
		v, isBinary = mmsGetDoubleVal(value)
		if logging && value != nil {
			fmt.Fprintf(log, " Value is of simple type %s %s", mmsTypeName(value.Type()), formatDouble(v))
		}
		failed = mmsTestDoubleStateFailed(value)
		transient = mmsTestDoubleStateTransient(value)
	}

	valueString := formatDouble(v)
	if isBinary {
		valueString = "false"
		if v != 0 {
			valueString = "true"
		}
	}

	asdu := "MMS_DATA_ACCESS_ERROR"
	if value != nil {
		asdu = mmsTypeName(value.Type())
	}

	iv := IECValue{
		IsDigital:       isBinary,
		IsTransient:     transient,
		Value:           v,
		ValueString:     valueString,
		ValueJSON:       mmsGetStringValue(value),
		ServerTimestamp: time.Now(),
		Cot:             20,
		CommonAddress:   entry.FC.String(),
		Address:         entry.Path,
		Asdu:            asdu,
		Quality:         !failed,
		SelfPublish:     selfPublish,
		ConnName:        conn.Name,
		ConnNumber:      conn.ProtocolConnectionNumber,
		DisplayName:     entry.Path,
	}
	_ = fromReport
	return iv, timestamp
}

// reportHandler receives one decoded report. It runs on the association's
// reader goroutine, so it only formats a log line and enqueues values.
func reportHandler(conn *Iec61850Connection, st *rcbState, rep *client.Report) {
	var log strings.Builder
	logging := jslog.Level() > jslog.LevelNoLog

	if logging {
		fmt.Fprintf(&log, "%s Report RCB: %s", conn.Name, st.ref)
		fmt.Fprintf(&log, " SeqNumb:%d", rep.SeqNum)
		if rep.SubSeqNum != 0 {
			fmt.Fprintf(&log, " SubSeqNumb:%d", rep.SubSeqNum)
		}
		log.WriteString("\n")
	}

	// Reports of another RCB that advertises the same RptID reach this
	// callback too; tell them apart by their data set.
	if st.collision && rep.DataSet != "" &&
		rep.DataSet != st.dataSetRef && rep.DataSet != st.dataSetDot {
		return
	}

	if len(rep.EntryID) > 0 {
		if logging {
			fmt.Fprintf(&log, "  entryID: %s \n", entryIDString(rep.EntryID))
		}
		if st.buffered {
			if last, ok := conn.LastReportID(st.ref); ok && bytes.Equal(last, rep.EntryID) {
				if logging {
					log.WriteString("Repeated report!\n")
					jslog.Log(jslog.LevelBasic, "%s", log.String())
				}
				return
			}
			conn.SetLastReportID(st.ref, append([]byte(nil), rep.EntryID...))
		}
	}

	if logging {
		if rep.DataSet != "" {
			fmt.Fprintf(&log, "  data-set: %s\n", st.dataSetRef)
		}
		if !rep.TimeOfEntry.IsZero() {
			fmt.Fprintf(&log, "  timestamp: %s\n", rep.TimeOfEntry.UTC().Format("1/2/2006 3:04:05 PM -07:00"))
		}
		fmt.Fprintf(&log, "  report dataset contains %d elements\n", len(rep.Entries))
	}

	// A server that ignores the requested OptFlds sends no reason codes at
	// all. The inclusion bitstring has already told us these members are
	// included, so forward them rather than dropping the report, which is
	// what the C# driver's HasReasonForInclusion test would do (D12).
	hasReasons := false
	for _, e := range rep.Entries {
		if e.Reason != 0 {
			hasReasons = true
			break
		}
	}
	if !hasReasons && len(rep.Entries) > 0 {
		jslog.Log(jslog.LevelDetailed, "%s Report %s carries no reason codes; forwarding all included elements",
			conn.Name, st.ref)
	}

	for k, e := range rep.Entries {
		if hasReasons && e.Reason == 0 { // REASON_NOT_INCLUDED
			continue
		}
		if e.Ref == "" {
			fmt.Fprintf(&log, "Can't get data reference for element %d of report! Skipping element...\n", k)
			continue
		}

		key := entryKey(string(e.Ref), e.FC)
		entry := conn.Entry(key)
		if entry == nil {
			if !conn.AutoCreateTags {
				continue // no autoCreateTags: do not forward undefined tags
			}
			entry = conn.AddEntry(key, &Iec61850Entry{
				Path:        string(e.Ref),
				FC:          e.FC,
				JsTag:       conn.Name + ":" + string(e.Ref),
				AutoPublish: true,
			})
		}
		conn.SetEntryReport(entry, st.ref, st.dataSetRef)

		fmt.Fprintf(&log, "\nElement %d , path %s [%s] , js_tag %s\n", k, entry.Path, e.FC, entry.JsTag)
		if logging {
			fmt.Fprintf(&log, " Included for reason %s \n", reasonName(e.Reason))
		}

		iv, timestamp := buildIECValue(conn, entry, e.Value, conn.AutoCreateTags, true, &log)

		// Only a data-change carries a usable source timestamp, and only
		// from a structured value.
		if e.Reason&model.ReasonDataChange != 0 && e.Value != nil && e.Value.Type() == mms.TypeStructure && timestamp != 0 {
			iv.HasSourceTimestamp = true
			iv.SourceTimestamp = time.UnixMilli(int64(timestamp)).UTC()
		}
		enqueueValue(iv)
	}

	jslog.Log(jslog.LevelBasic, "%s", log.String())
}

// reasonName renders a reason-for-inclusion the way libiec61850's
// ReasonForInclusion enum does.
func reasonName(rc model.ReasonCode) string {
	switch {
	case rc&model.ReasonDataChange != 0:
		return "REASON_DATA_CHANGE"
	case rc&model.ReasonQualityChange != 0:
		return "REASON_QUALITY_CHANGE"
	case rc&model.ReasonDataUpdate != 0:
		return "REASON_DATA_UPDATE"
	case rc&model.ReasonIntegrity != 0:
		return "REASON_INTEGRITY"
	case rc&model.ReasonGI != 0:
		return "REASON_GI"
	case rc&model.ReasonApplTrigger != 0:
		return "REASON_APPL_TRIGGER"
	}
	return "REASON_NOT_INCLUDED"
}
