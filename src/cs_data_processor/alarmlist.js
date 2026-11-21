'use strict'

/*
 * Alarm list (alarmsData collection): one document per alarm occurrence.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 *
 * Port of ../cs_data_processor-go/alarmlist.go: change both together and
 * keep alarmlist.test.js and alarmlist_test.go with the same expected values.
 * Contract in docs/schema.md (alarmsData).
 *
 * An alarm is "active" while its condition lasts (analog out of the range
 * that raised it, digital in its alarm state) and "unacknowledged" (ack 0)
 * until an operator acknowledges it; server_realtime_auth acknowledges.
 * A new condition of a point (range or state change) makes its earlier
 * active alarms inactive, then the new alarm (if any) is inserted, in this
 * order, through one ordered queue so the alarms of a point never swap.
 */

const { Double } = require('mongodb')
const Log = require('./simple-logger')

const MaxPending = 20000 // alarm writes waiting for the database

// A digital alarm is active while the value is in the alarm state
// (alarmState 0 or 1); other alarm states alarm on any change, never active.
function digitalAlarmActive(alarmState, value) {
  if (alarmState === 0 || alarmState === 1) return value === alarmState
  return false
}

// the alarm text of a digital point: its event text, else the value text
function digitalAlarmText(fd, value, valueString) {
  const t = value !== 0 ? fd?.eventTextTrue : fd?.eventTextFalse
  return typeof t === 'string' && t.trim() !== '' ? t : valueString
}

// alarmsData document of a new alarm
function alarmDocument(fd, a) {
  const doc = {
    tag: fd.tag,
    pointKey: fd._id,
    group1: fd.group1,
    group2: fd.group2,
    description: fd.description,
    type: fd.type,
    priority: fd.priority,
    value: new Double(a.value),
    valueString: a.valueString,
    alarmText: a.alarmText,
    timeTag: a.timeTag,
    timeTagAtSource: a.timeTagAtSource ?? null,
    timeTagAtSourceOk: a.timeTagAtSourceOk ?? null,
    active: a.active,
    ack: 0,
    timeTagAck: null,
    ackUser: null,
    timeTagNormal: a.active ? null : a.timeTag,
  }
  if (fd.type === 'analog') doc.alarmRange = new Double(a.alarmRange)
  return doc
}

// bulk write operations: deactivate the earlier active alarms of the point,
// then insert the new alarm (doc may be null)
function alarmOps(pointKey, deactivate, doc, now) {
  const ops = []
  if (deactivate)
    ops.push({
      updateMany: {
        filter: { pointKey: pointKey, active: true },
        update: { $set: { active: false, timeTagNormal: now } },
      },
    })
  if (doc) ops.push({ insertOne: { document: doc } })
  return ops
}

// Ordered writer: the operations of every change are written one change
// after the other, so the alarms of a point keep their order.
class AlarmWriter {
  constructor(getCollection) {
    this.getCollection = getCollection
    this.chain = Promise.resolve()
    this.pending = 0
  }

  write(ops) {
    if (ops.length === 0) return
    if (this.pending >= MaxPending) {
      Log.log('Alarm list - too many pending writes, alarm dropped')
      return
    }
    this.pending++
    this.chain = this.chain
      .then(async () => {
        const coll = this.getCollection()
        if (!coll) throw new Error('no database')
        await coll.bulkWrite(ops, { ordered: true })
      })
      .catch((err) => Log.log('Alarm list - error: ' + err))
      .finally(() => {
        this.pending--
      })
  }
}

module.exports = {
  digitalAlarmActive,
  digitalAlarmText,
  alarmDocument,
  alarmOps,
  AlarmWriter,
}
