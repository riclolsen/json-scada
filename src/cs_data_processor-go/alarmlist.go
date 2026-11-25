package main

// Alarm list (alarmsData collection): one document per alarm occurrence.
//
// Port of ../cs_data_processor/alarmlist.js: change both together and keep
// alarmlist_test.go and alarmlist.test.js with the same expected values.
// Contract in docs/schema.md (alarmsData).
//
// An alarm is "active" while its condition lasts (analog out of the range
// that raised it, digital in its alarm state) and "unacknowledged" (ack 0)
// until an operator acknowledges it; server_realtime_auth acknowledges. A
// new condition of a point (range or state change) makes its earlier active
// alarms inactive, then the new alarm (if any) is inserted, in this order,
// through one ordered writer so the alarms of a point never swap.

import (
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// alarmMaxBatch is the most alarm changes written in one ordered bulk write.
const alarmMaxBatch = 500

// alarmOp is the alarm list change of one processed point update.
type alarmOp struct {
	pointKey   any    // the point _id
	deactivate bool   // end the earlier active alarms of the point
	doc        bson.D // the new alarm, nil when none
	now        time.Time
}

// alarmFields are the values of a new alarm.
type alarmFields struct {
	value             float64
	valueString       string
	alarmText         string
	alarmRange        float64
	active            bool
	timeTag           time.Time
	timeTagAtSource   any
	timeTagAtSourceOk any
}

// digitalAlarmActive reports whether a digital alarm is active: the value is
// in the alarm state (alarmState 0 or 1); other alarm states alarm on any
// change and are never active.
func digitalAlarmActive(alarmState float64, alarmStateOk bool, value float64) bool {
	if alarmStateOk && (alarmState == 0 || alarmState == 1) {
		return value == alarmState
	}
	return false
}

// digitalAlarmText is the alarm text of a digital point: its event text,
// else the value text.
func digitalAlarmText(fd rawDoc, value float64, valueString string) string {
	key := "eventTextFalse"
	if value != 0 {
		key = "eventTextTrue"
	}
	if v := fd.lookup(key); v.Type == bson.TypeString {
		if s, _ := v.StringValueOK(); strings.TrimSpace(s) != "" {
			return s
		}
	}
	return valueString
}

// alarmDocument is the alarmsData document of a new alarm, with the field
// order of the Node.js version.
func alarmDocument(fd rawDoc, a alarmFields) bson.D {
	var normal any
	if !a.active {
		normal = a.timeTag
	}
	d := bson.D{
		{Key: "tag", Value: rawOrNil(fd.lookup("tag"))},
		{Key: "pointKey", Value: rawOrNil(fd.idValue())},
		{Key: "group1", Value: rawOrNil(fd.lookup("group1"))},
		{Key: "group2", Value: rawOrNil(fd.lookup("group2"))},
		{Key: "description", Value: rawOrNil(fd.lookup("description"))},
		{Key: "type", Value: rawOrNil(fd.lookup("type"))},
		{Key: "priority", Value: rawOrNil(fd.lookup("priority"))},
		{Key: "value", Value: a.value},
		{Key: "valueString", Value: a.valueString},
		{Key: "alarmText", Value: a.alarmText},
		{Key: "timeTag", Value: a.timeTag},
		{Key: "timeTagAtSource", Value: a.timeTagAtSource},
		{Key: "timeTagAtSourceOk", Value: a.timeTagAtSourceOk},
		{Key: "active", Value: a.active},
		{Key: "ack", Value: int32(0)},
		{Key: "timeTagAck", Value: nil},
		{Key: "ackUser", Value: nil},
		{Key: "timeTagNormal", Value: normal},
	}
	if fd.str("type") == "analog" {
		d = append(d, bson.E{Key: "alarmRange", Value: a.alarmRange})
	}
	return d
}

// models are the bulk write operations of the change: deactivate the earlier
// active alarms of the point, then insert the new alarm.
func (op alarmOp) models() []mongo.WriteModel {
	var m []mongo.WriteModel
	if op.deactivate {
		m = append(m, mongo.NewUpdateManyModel().
			SetFilter(bson.D{{Key: "pointKey", Value: op.pointKey}, {Key: "active", Value: true}}).
			SetUpdate(bson.D{{Key: "$set", Value: bson.D{
				{Key: "active", Value: false},
				{Key: "timeTagNormal", Value: op.now},
			}}}))
	}
	if op.doc != nil {
		m = append(m, mongo.NewInsertOneModel().SetDocument(op.doc))
	}
	return m
}

func (p *Processor) emitAlarm(op alarmOp) {
	if !op.deactivate && op.doc == nil {
		return
	}
	select {
	case p.alarmCh <- op:
	default:
		M.Inc(CntDropped, 1)
		Log(LogLevelMin, "Alarm list - queue full, alarm dropped")
	}
}

// alarmWriter writes the alarm list changes in order, batching the changes
// already waiting into one ordered bulk write.
func (p *Processor) alarmWriter(ctx context.Context) {
	for {
		var batch []mongo.WriteModel
		select {
		case <-ctx.Done():
			return
		case op := <-p.alarmCh:
			batch = append(batch, op.models()...)
		}
	drain:
		for len(batch) < alarmMaxBatch {
			select {
			case op := <-p.alarmCh:
				batch = append(batch, op.models()...)
			default:
				break drain
			}
		}
		if len(batch) == 0 || !Mongo.WaitReady(ctx) {
			continue
		}
		if _, _, _, _, db, ok := Mongo.Handles(); ok && db != nil {
			_, err := db.Collection(AlarmsDataCollectionName).
				BulkWrite(ctx, batch, options.BulkWrite().SetOrdered(true))
			if err != nil && ctx.Err() == nil {
				M.Inc(CntErrors, 1)
				Log(LogLevelMin, "Alarm list - error: %v", err)
			}
		}
	}
}
