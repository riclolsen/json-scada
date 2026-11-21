/*
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

// Conversion of a raw protocol update into realtime, SOE and historical
// data. This is a faithful port of the change handler of
// cs_data_processor.js; the JavaScript semantics it depends on (loose
// equality against undefined, truthiness, Number formatting) are reproduced
// through the helpers in jsutil.go and rawdoc.go.
//
// Concurrency: events are dispatched to a pool of worker goroutines keyed by
// the point _id, so updates of the same point are always processed in the
// order the change stream delivered them, while different points proceed in
// parallel.

package main

import (
	"hash/maphash"
	"math"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// changeEvent is one change stream event on its way to a worker.
type changeEvent struct {
	raw bson.Raw
	// recvAt is the wall clock instant the event was read from the cursor,
	// compared against the timestamp the protocol driver wrote; recvHr is the
	// same instant on the high resolution clock, for the in-process stages.
	recvAt time.Time
	recvHr hrTime
}

// rtUpdate is a pending realtimeData modification.
type rtUpdate struct {
	id         bson.RawValue
	set        bson.D
	addToSet   bson.D
	enqueuedAt hrTime
	sourceTime time.Time
	hasSource  bool
}

// histEntry is one historical sample, for both PostgreSQL and MongoDB.
type histEntry struct {
	sql string
	obj bson.D
}

// Processor owns the channels connecting the change stream reader, the
// workers and the writers.
type Processor struct {
	cfg Config

	changeCh chan changeEvent
	rtCh     chan *rtUpdate
	histCh   chan histEntry
	sqlRtCh  chan string
	soeCh    chan bson.D
	alarmCh  chan alarmOp

	digitalUpdatesCount int64
	digitalMu           sync.Mutex

	seed maphash.Seed
	wg   sync.WaitGroup
}

// NewProcessor allocates the pipeline.
func NewProcessor(cfg Config) *Processor {
	return &Processor{
		cfg:      cfg,
		changeCh: make(chan changeEvent, cfg.ChangeQueueSize),
		rtCh:     make(chan *rtUpdate, cfg.WriteQueueSize),
		histCh:   make(chan histEntry, cfg.WriteQueueSize),
		sqlRtCh:  make(chan string, cfg.WriteQueueSize),
		soeCh:    make(chan bson.D, cfg.WriteQueueSize),
		alarmCh:  make(chan alarmOp, cfg.WriteQueueSize),
		seed:     maphash.MakeSeed(),
	}
}

// Submit hands a change event to the worker pool. It never blocks the change
// stream reader: on sustained overload the oldest pending event is dropped
// and counted, so that a slow database cannot stall the cursor.
func (p *Processor) Submit(ev changeEvent) {
	select {
	case p.changeCh <- ev:
	default:
		select {
		case <-p.changeCh:
			M.Inc(CntDropped, 1)
		default:
		}
		select {
		case p.changeCh <- ev:
		default:
			M.Inc(CntDropped, 1)
		}
	}
}

// StartWorkers launches the sharded worker pool. Each worker owns one inbox;
// the dispatcher goroutine routes events by a hash of the point _id so that
// per point ordering is preserved.
func (p *Processor) StartWorkers() {
	n := p.cfg.Workers
	inboxes := make([]chan changeEvent, n)
	for i := 0; i < n; i++ {
		inboxes[i] = make(chan changeEvent, p.cfg.ChangeQueueSize/n+1)
		p.wg.Add(1)
		go func(in chan changeEvent) {
			defer p.wg.Done()
			for ev := range in {
				M.Stage(StageQueueWait).ObserveHrSince(ev.recvHr)
				start := hrNow()
				p.handleChange(ev)
				M.Stage(StageProcessing).ObserveHrSince(start)
				M.Inc(CntChangesProcessed, 1)
			}
		}(inboxes[i])
	}
	go func() {
		var h maphash.Hash
		for ev := range p.changeCh {
			idx := 0
			if n > 1 {
				h.SetSeed(p.seed)
				h.Reset()
				if fd, err := ev.raw.LookupErr("fullDocument", "_id"); err == nil {
					h.Write(fd.Value)
				}
				idx = int(h.Sum64() % uint64(n))
			}
			inboxes[idx] <- ev
		}
	}()
}

// QueueDepths reports the current channel occupation, published as metrics
// gauges so back pressure is visible while comparing implementations.
func (p *Processor) QueueDepths() map[string]float64 {
	return map[string]float64{
		"queue_changes": float64(len(p.changeCh)),
		"queue_rtdata":  float64(len(p.rtCh)),
		"queue_hist":    float64(len(p.histCh)),
		"queue_sqlrt":   float64(len(p.sqlRtCh)),
		"queue_soe":     float64(len(p.soeCh)),
	}
}

func (p *Processor) emitRt(u *rtUpdate) {
	u.enqueuedAt = hrNow()
	select {
	case p.rtCh <- u:
		M.Inc(CntUpdatesQueued, 1)
	default:
		M.Inc(CntDropped, 1)
	}
}

func (p *Processor) emitSoe(d bson.D) {
	select {
	case p.soeCh <- d:
		M.Inc(CntSoeInserted, 1)
	default:
		M.Inc(CntDropped, 1)
	}
}

func (p *Processor) emitHist(e histEntry) {
	select {
	case p.histCh <- e:
		M.Inc(CntHistQueued, 1)
	default:
		M.Inc(CntDropped, 1)
	}
}

func (p *Processor) emitSQLRt(s string) {
	select {
	case p.sqlRtCh <- s:
	default:
		M.Inc(CntDropped, 1)
	}
}

// handleChange is the port of the changeStream 'change' handler.
func (p *Processor) handleChange(ev changeEvent) {
	defer func() {
		if r := recover(); r != nil {
			M.Inc(CntErrors, 1)
			Log(LogLevelMin, "Error processing change: %v", r)
		}
	}()

	root := rawDoc{raw: ev.raw}
	opType := root.str("operationType")
	if opType == "delete" {
		return
	}
	fd := root.doc("fullDocument")
	if !fd.valid() {
		return
	}

	tag := fd.str("tag")
	docType := fd.str("type")

	// -----------------------------------------------------------------
	// insert: just mirror the new point into the PostgreSQL realtime table
	// -----------------------------------------------------------------
	if opType == "insert" {
		fdValue, _ := fd.num("value")
		Log(LogLevelNormal, "INSERT %s %s %s", fd.idString(), tag, jsNumberToString(fdValue))
		M.Inc(CntInserts, 1)
		p.emitSQLRt("'" + sqlQuote(tag) + "'," +
			"'" + jsISODate(time.Now()) + "'," +
			"to_json('" + sqlQuote(bsonDocToJSON(fd.raw, nil, nil)) + "'::text)")
		return
	}

	// when inactive (redundancy standby), ignore changes
	if !ProcessStateIsActive() {
		M.Inc(CntIgnoredInactive, 1)
		return
	}

	updatedFields := root.doc("updateDescription").doc("updatedFields")
	sdu := updatedFields.doc("sourceDataUpdate")
	if !sdu.valid() {
		// not a Source Data Update (protocol update)
		return
	}

	// ---- latency accounting: the driver's own timestamp is the origin ----
	sourceTimeTag, hasSourceTimeTag := sdu.timeOf("timeTag")
	if hasSourceTimeTag {
		M.Stage(StageSourceToRecv).Observe(ev.recvAt.Sub(sourceTimeTag).Microseconds())
	}

	isSOE := false
	alarmRange := 0.0
	rangeEscalated := false     // to a more severe limit level
	analogRangeChanged := false // ends the active alarms of the point
	limitAlarmText := ""        // text of a new limit alarm

	// consider SOE when a digital change carries a source timestamp, or an
	// analog marked as isEvent does
	if tts, ok := sdu.timeOf("timeTagAtSource"); ok {
		if docType == "digital" || (docType == "analog" && fd.truthy("isEvent")) {
			if tts.UTC().Year() > 1899 {
				isSOE = true
			}
		}
	}

	// ---- quality bits set by the protocol driver ----
	invalid, transient, overflow, nottopical := false, false, false, false
	carry, substituted, blocked := false, false, false
	if b, ok := sdu.boolStrict("invalidAtSource"); ok {
		invalid = b
	}
	if b, ok := sdu.boolStrict("notTopicalAtSource"); ok {
		invalid = invalid || b
		nottopical = b
	}
	if b, ok := sdu.boolStrict("overflowAtSource"); ok {
		invalid = invalid || b
		overflow = b
	}
	if b, ok := sdu.boolStrict("transientAtSource"); ok {
		invalid = invalid || b
		transient = b
	}
	if b, ok := sdu.boolStrict("carryAtSource"); ok {
		carry = b
	}
	if b, ok := sdu.boolStrict("substitutedAtSource"); ok {
		substituted = b
	}
	if b, ok := sdu.boolStrict("blockedAtSource"); ok {
		blocked = b
	}

	value, valueOk := sdu.num("valueAtSource")
	valueAtSource := value
	valueString := sdu.str("valueStringAtSource")
	valueJSON := ""
	if v := sdu.lookup("valueJsonAtSource"); v.Type == bson.TypeString {
		valueJSON, _ = v.StringValueOK()
	}

	fdValue, fdValueOk := fd.num("value")
	alarmed := fd.truthy("alarmed")
	fdAlarmedIsFalse := false
	if b, ok := fd.boolStrict("alarmed"); ok && !b {
		fdAlarmedIsFalse = true
	}
	alarmDisabled := fd.truthy("alarmDisabled")
	isEventTruthy := fd.truthy("isEvent")
	isEventStrictTrue := false
	isEventStrictFalse := false
	if b, ok := fd.boolStrict("isEvent"); ok {
		isEventStrictTrue = b
		isEventStrictFalse = !b
	}
	unit := fd.str("unit")

	// avoid undefined, null or NaN values
	if !valueOk || math.IsNaN(value) {
		value = 0.0
		invalid = true
	}

	// qualifier shown appended to valueString
	var qb strings.Builder
	if invalid {
		qb.WriteString("[IV]")
	}
	if transient {
		qb.WriteString("[TR]")
	}
	if overflow {
		qb.WriteString("[OV]")
	}
	if nottopical {
		qb.WriteString("[NT]")
	}
	if carry {
		qb.WriteString("[CR]")
	}
	if substituted {
		qb.WriteString("[SB]")
	}
	if blocked {
		qb.WriteString("[BK]")
	}
	txtQualif := qb.String()

	asdu := sdu.str("asduAtSource")
	hasASDU := sdu.has("asduAtSource")

	switch docType {
	case "digital":
		// test for double point status
		if hasASDU && strings.HasPrefix(asdu, "M_DP_") {
			if value == 0 || value == 3 {
				transient = true
				invalid = true
				if !strings.Contains(txtQualif, "[IV]") {
					txtQualif += "[IV]"
				}
				if !strings.Contains(txtQualif, "[TR]") {
					txtQualif += "[TR]"
				}
				if txtQualif != "" {
					txtQualif = " " + txtQualif
				}
			}
			if int32(value)&0x01 == 0 {
				value = 1
			} else {
				value = 0
			}
		}

		// process inversions (kconv1 = -1)
		if k, ok := fd.num("kconv1"); ok && k == -1 {
			if value == 0 {
				value = 1
			} else {
				value = 0
			}
		}
		if (!fdValueOk || value != fdValue) && !alarmDisabled {
			alarmed = true
		}
		unitSuffix := ""
		if unit != "" {
			unitSuffix = " " + unit
		}
		if value != 0 {
			valueString = fd.str("stateTextTrue") + unitSuffix + txtQualif
		} else {
			valueString = fd.str("stateTextFalse") + unitSuffix + txtQualif
		}

	case "analog":
		if txtQualif != "" {
			txtQualif = " " + txtQualif
		}

		// apply conversion factors
		kconv1 := fd.numOr("kconv1", math.NaN())
		kconv2 := fd.numOr("kconv2", math.NaN())
		value = valueAtSource*kconv1 + kconv2

		if fd.has("zeroDeadband") {
			zdb := fd.numOr("zeroDeadband", 0)
			if zdb != 0 && math.Abs(value) < zdb {
				value = 0.0
			}
		}

		valueString = jsFixedString(value, 4) + " " + unit + txtQualif

		if hasASDU && strings.HasPrefix(asdu, "M_BO_") {
			// bitstring
			valueString = jsToStringRadix2(value) + " " + unit + txtQualif
		}

		hysteresis := 0.0
		if fd.truthy("hysteresis") {
			hysteresis = fd.numOr("hysteresis", 0)
		}

		// check for limits (all the configured levels, see limits.go)
		if hasLimits(fd) && !alarmDisabled {
			prevRange, prevRangeOk := fd.num("alarmRange")
			r := alarmRangeOf(fd, value, hysteresis)
			alarmRange = float64(r.rng)
			rangeEscalated = r.escalated
			if r.normal {
				alarmed = false
			}

			// SOE entry when the analog alarm condition changes.
			// A missing/null alarmRange compares unequal to any number,
			// exactly like the loose != of the JavaScript version.
			rangeChanged := !prevRangeOk || prevRange != alarmRange
			if !alarmDisabled && rangeChanged {
				if alarmRange != 0 {
					alarmed = true
				}
				eventDate := time.Now()
				arrow := ""
				if fdValueOk {
					if math.Abs(value) > math.Abs(fdValue) {
						arrow = " ⤉"
					} else if math.Abs(value) < math.Abs(fdValue) {
						arrow = " ⤈"
					}
				}
				flag := " \U0001F6A9"
				ack := 0
				if !alarmed {
					flag = " \U0001F197"
					ack = 1 // enter as acknowledged when normalized
				}
				label := ""
				if alarmRange != 0 {
					label = " " + rangeLabel(int(alarmRange))
				}
				eventText := jsFixedString(value, 3) + " " + unit + arrow + flag + label
				analogRangeChanged = true
				limitAlarmText = eventText
				p.emitSoe(bson.D{
					{Key: "tag", Value: tag},
					{Key: "pointKey", Value: rawOrNil(fd.idValue())},
					{Key: "group1", Value: rawOrNil(fd.lookup("group1"))},
					{Key: "description", Value: rawOrNil(fd.lookup("description"))},
					{Key: "eventText", Value: eventText},
					{Key: "invalid", Value: false},
					{Key: "priority", Value: rawOrNil(fd.lookup("priority"))},
					{Key: "timeTag", Value: eventDate},
					{Key: "timeTagAtSource", Value: eventDate},
					{Key: "timeTagAtSourceOk", Value: true},
					{Key: "ack", Value: ack},
				})
			}
		}

		// alarms disabled: the limit alarms of the point end
		if alarmDisabled && fd.truthy("alarmRange") {
			analogRangeChanged = true
		}

		// analogs produce SOE events when marked isEvent and the valid value
		// changed, or when they carry a source timestamp
		if !alarmDisabled {
			if (isEventStrictTrue && !invalid && (!fdValueOk || value != fdValue)) || isSOE {
				arrow := ""
				if fdValueOk {
					if math.Abs(value) > math.Abs(fdValue) {
						arrow = " ↑"
					} else if math.Abs(value) < math.Abs(fdValue) {
						arrow = " ↓"
					}
				}
				eventText := jsFixedString(value, 3) + " " + unit + arrow
				now := time.Now()
				var ttSource any = now
				var ttSourceOk any = false
				if isSOE {
					if t, ok := sdu.timeOf("timeTagAtSource"); ok {
						ttSource = t
					}
					ttSourceOk = rawOrNil(sdu.lookup("timeTagAtSourceOk"))
				}
				p.emitSoe(bson.D{
					{Key: "tag", Value: tag},
					{Key: "pointKey", Value: rawOrNil(fd.idValue())},
					{Key: "group1", Value: rawOrNil(fd.lookup("group1"))},
					{Key: "description", Value: rawOrNil(fd.lookup("description"))},
					{Key: "eventText", Value: eventText},
					{Key: "invalid", Value: false},
					{Key: "priority", Value: rawOrNil(fd.lookup("priority"))},
					{Key: "timeTag", Value: now},
					{Key: "timeTagAtSource", Value: ttSource},
					{Key: "timeTagAtSourceOk", Value: ttSourceOk},
					{Key: "ack", Value: 1}, // acknowledged, it is not an alarm
				})
			}
		}
	}

	// if changed to alarmed state, or digital change or SOE, register the
	// time of the new alarm
	var alarmTime time.Time
	hasAlarmTime := false
	if !alarmDisabled && alarmed {
		if !fd.truthy("alarmed") ||
			(docType == "digital" && (!fdValueOk || value != fdValue)) ||
			(docType == "digital" && isSOE) ||
			rangeEscalated {
			alarmTime = time.Now()
			hasAlarmTime = true
		}
	}

	// ------------------------------------------------------------------
	// decide whether realtimeData must be updated
	// ------------------------------------------------------------------
	valueChanged := !fdValueOk || value != fdValue
	changedForUpdate := isSOE ||
		sdu.truthy("rangeCheck") ||
		(valueChanged && !(!isSOE && docType == "digital" && isEventStrictTrue)) ||
		(docType == "string" && valueString != fd.str("valueString")) ||
		(docType == "json" && valueJSON != fd.str("valueJson")) ||
		sourceTimeTagAtSourceChanged(fd, sdu) ||
		fd.isNull("timeTag") ||
		invalidChanged(invalid, fd)

	isHistorical := sdu.truthy("isHistorical")

	if changedForUpdate && !isHistorical {
		dt := time.Now()

		if !alarmDisabled {
			if (alarmed && isSOE && isEventStrictTrue && docType == "digital" && value != 0) ||
				(alarmed && isEventStrictFalse && docType == "digital") ||
				(alarmed && fdAlarmedIsFalse) ||
				(alarmed && rangeEscalated) {
				// a new alarm, update the beep point
				Log(LogLevelNormal, "NEW BEEP, tag: %s", tag)
				priority, priorityOk := fd.num("priority")
				group1 := fd.lookup("group1")
				if priorityOk && priority == 0 {
					// important beep (alarm of priority zero)
					p.emitRt(&rtUpdate{
						id: doubleRawValue(beepPointKey),
						set: bson.D{
							{Key: "beepType", Value: 2.0}, // important beep
							{Key: "value", Value: 1.0},
							{Key: "valueString", Value: "Beep Active"},
							{Key: "timeTag", Value: dt},
						},
						addToSet:   bson.D{{Key: "beepGroup1List", Value: rawOrNil(group1)}},
						sourceTime: sourceTimeTag, hasSource: hasSourceTimeTag,
					})
				} else if priorityOk && priority <= lowestPriorityThatBeeps {
					p.emitRt(&rtUpdate{
						id: doubleRawValue(beepPointKey),
						set: bson.D{
							{Key: "value", Value: 1.0},
							{Key: "valueString", Value: "Beep Active"},
							{Key: "timeTag", Value: dt},
						},
						addToSet:   bson.D{{Key: "beepGroup1List", Value: rawOrNil(group1)}},
						sourceTime: sourceTimeTag, hasSource: hasSourceTimeTag,
					})
				}
			}
			if docType == "digital" {
				p.digitalMu.Lock()
				p.digitalUpdatesCount++
				cnt := p.digitalUpdatesCount
				p.digitalMu.Unlock()
				p.emitRt(&rtUpdate{
					id: doubleRawValue(cntUpdatesPointKey),
					set: bson.D{
						{Key: "value", Value: float64(cnt)},
						{Key: "valueString", Value: jsNumberToString(float64(cnt)) + " Updates"},
						{Key: "timeTag", Value: dt},
					},
					sourceTime: sourceTimeTag, hasSource: hasSourceTimeTag,
				})
			}
		}

		// alarm list (alarmsData): end the earlier alarms of the point when
		// its condition changed, register the new alarm
		{
			op := alarmOp{pointKey: rawOrNil(fd.idValue()), now: dt}
			fields := alarmFields{
				value:             value,
				valueString:       valueString,
				timeTag:           dt,
				timeTagAtSource:   rawOrNil(sdu.lookup("timeTagAtSource")),
				timeTagAtSourceOk: rawOrNil(sdu.lookup("timeTagAtSourceOk")),
			}
			switch {
			case docType == "analog":
				op.deactivate = analogRangeChanged
				if analogRangeChanged && alarmRange != 0 && !alarmDisabled {
					fields.alarmText = limitAlarmText
					fields.alarmRange = alarmRange
					fields.active = true
					op.doc = alarmDocument(fd, fields)
				}
			case docType == "digital" && !(isEventTruthy && value == 0):
				op.deactivate = !fdValueOk || value != fdValue || hasAlarmTime
				if hasAlarmTime && !alarmDisabled {
					fields.alarmText = digitalAlarmText(fd, value, valueString)
					st, stOk := fd.num("alarmState")
					fields.active = digitalAlarmActive(st, stOk, value)
					op.doc = alarmDocument(fd, fields)
				}
			}
			p.emitAlarm(op)
		}

		// historianPeriod < 0, or an update flagged as not for the historian,
		// excludes the sample from the historian
		insertIntoHistorian := true
		if fd.has("historianPeriod") {
			hp := fd.numOr("historianPeriod", 0)
			if hp < 0 || sdu.truthy("isNotForHistorical") {
				insertIntoHistorian = false
			} else if docType == "analog" && fd.has("historianDeadBand") {
				hlv, hlvOk := fd.num("historianLastValue")
				hdb := fd.numOr("historianDeadBand", 0)
				if hlvOk && !fd.isNull("historianLastValue") && hdb > 0 {
					if math.Abs(value-hlv) < math.Abs(hdb) {
						insertIntoHistorian = false
					}
				}
			}
		}

		updatesCnt := fd.numOr("updatesCnt", math.NaN()) + 1

		set := bson.D{
			{Key: "value", Value: value},
			{Key: "valueString", Value: valueString},
			{Key: "valueJson", Value: valueJSON},
		}
		if docType == "analog" && insertIntoHistorian {
			set = append(set, bson.E{Key: "historianLastValue", Value: value})
		}
		alarmedOut := alarmed
		if alarmDisabled {
			alarmedOut = false
		}
		set = append(set,
			bson.E{Key: "timeTag", Value: dt},
			bson.E{Key: "overflow", Value: overflow},
			bson.E{Key: "invalid", Value: invalid},
			bson.E{Key: "transient", Value: transient},
			bson.E{Key: "frozen", Value: false},
		)

		// update source time when available
		var updTimeTagAtSource any = nil
		var updTimeTagAtSourceOk any = nil
		hasTTAS := false
		ttasAssigned := false
		var ttasTime time.Time
		if sdu.has("timeTagAtSource") && !sdu.isNull("timeTagAtSource") {
			ttasAssigned = true
			if t, ok := sdu.timeOf("timeTagAtSource"); ok {
				updTimeTagAtSource = t
				ttasTime = t
				hasTTAS = true
			}
			if b, ok := sdu.boolStrict("timeTagAtSourceOk"); ok {
				updTimeTagAtSourceOk = b
			}
		}
		set = append(set,
			bson.E{Key: "timeTagAtSource", Value: updTimeTagAtSource},
			bson.E{Key: "timeTagAtSourceOk", Value: updTimeTagAtSourceOk},
			bson.E{Key: "updatesCnt", Value: updatesCnt},
			bson.E{Key: "alarmRange", Value: alarmRange},
			bson.E{Key: "alarmed", Value: alarmedOut},
		)
		if hasAlarmTime {
			set = append(set, bson.E{Key: "timeTagAlarm", Value: alarmTime})
		}

		// do not update protection-like events for state OFF
		if !(isEventTruthy && docType == "digital" && value == 0 && !isHistorical) {
			p.emitRt(&rtUpdate{
				id:         fd.idValue(),
				set:        set,
				sourceTime: sourceTimeTag,
				hasSource:  hasSourceTimeTag,
			})
			if logLevel() >= LogLevelDetailed {
				Log(LogLevelDetailed, "UPD %s %s %s DELAY %dms", fd.idString(), tag,
					jsNumberToString(value), msSince(sourceTimeTag, hasSourceTimeTag))
			}
		}

		// queue the sample for the PostgreSQL and MongoDB historians
		// Fields: tag, time_tag, value, value_json, time_tag_at_source, flags
		if insertIntoHistorian {
			b7 := "0" // value invalid
			if invalid {
				b7 = "1"
			}
			b6 := "1" // time tag at source invalid
			if ok, isBool := updTimeTagAtSourceOk.(bool); isBool && ok {
				b6 = "0"
			}
			b5 := "0" // analog
			if docType == "analog" {
				b5 = "1"
			}
			b4 := "0" // integrity
			if sdu.str("causeOfTransmissionAtSource") == "20" {
				b4 = "1"
			}
			const b3, b2, b1, b0 = "0", "0", "0", "0"

			vj := `""`
			if valueJSON != "" {
				vj = sqlQuote(strings.TrimSpace(valueJSON))
			}
			trimS := ""
			if len(vj) > 0 {
				if vj[0] == '{' || vj[0] == '[' {
					trimS = `"`
				} else if jsIsNaNString(vj) && vj[0] != '"' {
					vj = `"` + vj + `"`
				}
			}
			vs := ""
			if valueString != "" {
				vs = sqlQuote(valueString)
			}
			ttasSQL := "null"
			if hasTTAS {
				ttasSQL = "'" + jsISODate(ttasTime) + "'"
			}
			sourceTimeSQL := jsISODate(sourceTimeTag)

			var sb strings.Builder
			sb.WriteString("'" + sqlQuote(tag) + "',")
			sb.WriteString("'" + sourceTimeSQL + "',")
			sb.WriteString(jsNumberToString(value))
			sb.WriteString(`,('{"v":'||trim('` + trimS + `' FROM to_json('` + vj +
				`'::text)::jsonb #>> '{}')||',"s":'||to_json('` + vs + `'::text)||'}'::text)::jsonb,`)
			sb.WriteString(ttasSQL)
			sb.WriteString(",B'" + b7 + b6 + b5 + b4 + b3 + b2 + b1 + b0 + "'")

			obj := bson.D{{Key: "tag", Value: tag}, {Key: "timeTag", Value: sourceTimeTag}}
			switch docType {
			case "string":
				obj = append(obj, bson.E{Key: "value", Value: valueString})
			case "json":
				obj = append(obj, bson.E{Key: "value", Value: valueJSON})
			default:
				obj = append(obj, bson.E{Key: "value", Value: jsBSONNumber(value)})
			}
			obj = append(obj, bson.E{Key: "invalid", Value: invalid})
			if updTimeTagAtSource != nil {
				obj = append(obj, bson.E{Key: "timeTagAtSource", Value: updTimeTagAtSource})
			}
			if ttasAssigned {
				obj = append(obj, bson.E{Key: "timeTagAtSourceOk", Value: updTimeTagAtSourceOk})
			}
			if cot := sdu.str("causeOfTransmissionAtSource"); cot != "" {
				obj = append(obj, bson.E{Key: "cot", Value: cot})
			}
			p.emitHist(histEntry{sql: sb.String(), obj: obj})
		}

		// mirror the updated document into the PostgreSQL realtime table
		overrides := map[string]jsonValue{
			"value":       jvNumber(value),
			"valueString": jvString(valueString),
			"valueJson":   jvString(valueJSON),
			"timeTag":     jvDate(dt),
			"overflow":    jvBool(overflow),
			"invalid":     jvBool(invalid),
			"transient":   jvBool(transient),
			"updatesCnt":  jvNumber(updatesCnt),
			"alarmed":     jvBool(alarmed),
		}
		order := []string{"value", "valueString", "valueJson", "timeTag",
			"overflow", "invalid", "transient", "updatesCnt", "alarmed"}
		p.emitSQLRt("'" + sqlQuote(tag) + "'," +
			"'" + jsISODate(time.Now()) + "'," +
			"'" + sqlQuote(bsonDocToJSON(fd.raw, overrides, order)) + "'")
	} else {
		M.Inc(CntNotChanged, 1)
		if logLevel() >= LogLevelDetailed {
			Log(LogLevelDetailed, "Not changed %s DELAY %dms", tag,
				msSince(sourceTimeTag, hasSourceTimeTag))
		}
	}

	// ------------------------------------------------------------------
	// SOE entry for digital points
	// ------------------------------------------------------------------
	if isSOE && docType != "analog" && !alarmDisabled && !sdu.truthy("isNotForHistorical") {
		if !(value == 0 && isEventTruthy) {
			eventText := fd.str("eventTextFalse")
			if value != 0 {
				eventText = fd.str("eventTextTrue")
			}
			ttas, _ := sdu.timeOf("timeTagAtSource")
			p.emitSoe(bson.D{
				{Key: "tag", Value: tag},
				{Key: "pointKey", Value: rawOrNil(fd.idValue())},
				{Key: "group1", Value: rawOrNil(fd.lookup("group1"))},
				{Key: "description", Value: rawOrNil(fd.lookup("description"))},
				{Key: "eventText", Value: eventText},
				{Key: "invalid", Value: invalid},
				{Key: "priority", Value: rawOrNil(fd.lookup("priority"))},
				{Key: "timeTag", Value: time.Now()},
				{Key: "timeTagAtSource", Value: ttas},
				{Key: "timeTagAtSourceOk", Value: rawOrNil(sdu.lookup("timeTagAtSourceOk"))},
				{Key: "ack", Value: 0},
			})
			if logLevel() >= LogLevelDetailed {
				Log(LogLevelDetailed, "SOE %s %s %s %s", fd.idString(), tag,
					jsNumberToString(valueAtSource), ttas.UTC().Format(time.RFC3339Nano))
			}
		}
	}
}

// sourceTimeTagAtSourceChanged is the
// "sourceDataUpdate?.timeTagAtSource && fullDocument?.timeTagAtSource?.getTime() !== ..."
// term of the update condition. A missing timestamp on the stored document
// compares unequal, as undefined !== number in JavaScript.
func sourceTimeTagAtSourceChanged(fd, sdu rawDoc) bool {
	newT, ok := sdu.timeOf("timeTagAtSource")
	if !ok {
		return false
	}
	oldT, hadOld := fd.timeOf("timeTagAtSource")
	if !hadOld {
		return true
	}
	return !oldT.Equal(newT)
}

// invalidChanged is "invalid !== fullDocument.invalid": a stored value that
// is not a boolean is always different.
func invalidChanged(invalid bool, fd rawDoc) bool {
	old, ok := fd.boolStrict("invalid")
	if !ok {
		return true
	}
	return invalid != old
}

func msSince(t time.Time, valid bool) int64 {
	if !valid {
		return -1
	}
	return time.Since(t).Milliseconds()
}

// doubleRawValue builds a BSON double RawValue, the type the {json:scada}
// point keys use.
func doubleRawValue(v float64) bson.RawValue {
	t, data, err := bson.MarshalValue(v)
	if err != nil {
		return bson.RawValue{}
	}
	return bson.RawValue{Type: t, Value: data}
}
