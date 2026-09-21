/*
 * DNP3 Client Protocol driver for {json:scada}, in Go.
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

// The master.Handler that receives every measurement the stack decodes. Port
// of SOEHandler of the C++ client.

package clientapp

import (
	"strconv"
	"time"

	"dnp3-go/internal/dnp3util"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/master"
)

// soeHandler enqueues everything it receives. Handler methods run on the
// session goroutine, so they only convert and push; nothing here blocks.
type soeHandler struct {
	master.NopHandler
	conn  *Connection
	queue *ValueQueue
}

func newSOEHandler(conn *Connection, queue *ValueQueue) *soeHandler {
	return &soeHandler{conn: conn, queue: queue}
}

func (h *soeHandler) BeginFragment(info master.ResponseInfo) {
	kind := "solicited"
	if info.Unsolicited {
		kind = "Unsolicited"
	}
	jslog.Log(jslog.LevelDetailed, "%s - Begin Fragment: %s", h.conn.Name, kind)
}

func (h *soeHandler) EndFragment(master.ResponseInfo) {
	jslog.Log(jslog.LevelDetailed, "%s - End Fragment", h.conn.Name)
}

// push builds the queue entry. baseGroup is the common address the tag is
// configured under; group is what goes into asduAtSource. The C++ driver always
// writes variation 0 and cause of transmission 20 (quirk Q4), so this does too.
func (h *soeHandler) push(info master.HeaderInfo, baseGroup int, index uint16, value float64,
	valueString string, ts dnp3.Timestamp, q dnp3util.Quality) {

	out := Dnp3Value{
		Address:            int(index),
		BaseGroup:          baseGroup,
		Group:              baseGroup,
		Variation:          0,
		Value:              value,
		ValueString:        valueString,
		COT:                20,
		ServerTimestamp:    time.Now().UnixMilli(),
		HasSourceTimestamp: ts.IsValid(),
		TimeTagOk:          ts.Quality == dnp3.TimestampSynchronized,
		Quality:            q,
		ConnNumber:         h.conn.ProtocolConnectionNumber,
		// An event is a discrete occurrence and must not be coalesced away
		// with the next reading of the same point.
		IsEvent: info.IsEvent(),
	}
	if ts.IsValid() {
		out.SourceTimestamp = ts.Time.UnixMilli()
	}

	if jslog.Level() >= jslog.LevelDetailed {
		jslog.Log(jslog.LevelDetailed, "%s - Data Recv: addr=%d group=%d val=%s time=%d qual=%d",
			h.conn.Name, out.Address, out.Group, out.ValueString, out.SourceTimestamp, int(ts.Quality))
	}
	h.queue.Push(out)
}

func (h *soeHandler) logHeader(what string, info master.HeaderInfo, n int) {
	jslog.Log(jslog.LevelDetailed, "%s - Process %s GV=%s Count=%d",
		h.conn.Name, what, info.GV, n)
}

func (h *soeHandler) HandleBinary(info master.HeaderInfo, values []dnp3.Indexed[dnp3.Binary]) {
	h.logHeader("Binary", info, len(values))
	for _, v := range values {
		val, str := 0.0, "false"
		if v.Value.Value {
			val, str = 1.0, "true"
		}
		h.push(info, dnp3util.GroupBinaryInput, v.Index, val, str, v.Value.Time,
			dnp3util.CommonQuality(v.Value.Flags))
	}
}

func (h *soeHandler) HandleDoubleBit(info master.HeaderInfo, values []dnp3.Indexed[dnp3.DoubleBitBinary]) {
	h.logHeader("DoubleBinary", info, len(values))
	for _, v := range values {
		q := dnp3util.CommonQuality(v.Value.Flags)
		// A double-bit point is transient while it is moving, and an
		// impossible reading is treated as transient too.
		q.Transient = v.Value.Value == dnp3.DoubleBitIntermediate ||
			v.Value.Value == dnp3.DoubleBitIndeterminate
		val := 0.0
		if v.Value.Value == dnp3.DoubleBitDeterminedOn || v.Value.Value == dnp3.DoubleBitIndeterminate {
			val = 1.0
		}
		h.push(info, dnp3util.GroupDoubleBinaryInput, v.Index, val,
			strconv.Itoa(int(v.Value.Value)), v.Value.Time, q)
	}
}

func (h *soeHandler) HandleAnalog(info master.HeaderInfo, values []dnp3.Indexed[dnp3.Analog]) {
	h.logHeader("Analog", info, len(values))
	for _, v := range values {
		h.push(info, dnp3util.GroupAnalogInput, v.Index, v.Value.Value,
			formatFloat(v.Value.Value), v.Value.Time, dnp3util.AnalogQuality(v.Value.Flags))
	}
}

func (h *soeHandler) HandleCounter(info master.HeaderInfo, values []dnp3.Indexed[dnp3.Counter]) {
	h.logHeader("Counter", info, len(values))
	for _, v := range values {
		h.push(info, dnp3util.GroupCounter, v.Index, float64(v.Value.Value),
			strconv.FormatUint(uint64(v.Value.Value), 10), v.Value.Time,
			dnp3util.CounterQuality(v.Value.Flags))
	}
}

func (h *soeHandler) HandleFrozenCounter(info master.HeaderInfo, values []dnp3.Indexed[dnp3.FrozenCounter]) {
	h.logHeader("FrozenCounter", info, len(values))
	for _, v := range values {
		// parity: the C++ driver files frozen counters under common address 23,
		// the event group, not the 21 the README's table names. Reproduced so
		// that databases configured against the C++ driver keep working
		// (quirk Q1).
		h.push(info, dnp3util.GroupFrozenCounterEvent, v.Index, float64(v.Value.Value),
			strconv.FormatUint(uint64(v.Value.Value), 10), v.Value.Time,
			dnp3util.CounterQuality(v.Value.Flags))
	}
}

func (h *soeHandler) HandleBinaryOutputStatus(info master.HeaderInfo, values []dnp3.Indexed[dnp3.BinaryOutputStatus]) {
	h.logHeader("BinaryOutputStatus", info, len(values))
	for _, v := range values {
		val, str := 0.0, "false"
		if v.Value.Value {
			val, str = 1.0, "true"
		}
		h.push(info, dnp3util.GroupBinaryOutputStatus, v.Index, val, str, v.Value.Time,
			dnp3util.CommonQuality(v.Value.Flags))
	}
}

func (h *soeHandler) HandleAnalogOutputStatus(info master.HeaderInfo, values []dnp3.Indexed[dnp3.AnalogOutputStatus]) {
	h.logHeader("AnalogOutputStatus", info, len(values))
	for _, v := range values {
		h.push(info, dnp3util.GroupAnalogOutputStatus, v.Index, v.Value.Value,
			formatFloat(v.Value.Value), v.Value.Time, dnp3util.AnalogQuality(v.Value.Flags))
	}
}

// HandleOctetString discards group 110 and 111, as the C++ driver does. There
// is no common address for octet strings on the source side, so there is
// nowhere to file them (gap F1).
func (h *soeHandler) HandleOctetString(info master.HeaderInfo, values []dnp3.Indexed[dnp3.OctetString]) {
	jslog.Log(jslog.LevelDetailed, "%s - Ignoring %d octet string(s) GV=%s",
		h.conn.Name, len(values), info.GV)
}

// formatFloat renders a measurement for valueStringAtSource. std::to_string
// gives six decimals; Go's shortest round-trip form is used instead, because it
// is what every other Go driver in this repository writes.
func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
