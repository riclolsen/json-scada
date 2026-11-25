'use strict'

// Mirrors ../cs_data_processor-go/alarmlist_test.go; the expected values
// must stay identical in both.

const test = require('node:test')
const assert = require('node:assert')
const {
  digitalAlarmActive,
  digitalAlarmText,
  alarmDocument,
  alarmOps,
} = require('./alarmlist')

test('digitalAlarmActive', () => {
  // [alarmState, value, active]
  const cases = [
    [0, 0, true],
    [0, 1, false],
    [1, 1, true],
    [1, 0, false],
    [2, 1, false],
    [-1, 0, false],
    [undefined, 1, false],
  ]
  for (const [st, v, w] of cases) assert.strictEqual(digitalAlarmActive(st, v), w, JSON.stringify([st, v]))
})

test('digitalAlarmText', () => {
  const fd = { eventTextTrue: 'TRIP', eventTextFalse: '' }
  assert.strictEqual(digitalAlarmText(fd, 1, 'ON'), 'TRIP')
  assert.strictEqual(digitalAlarmText(fd, 0, 'OFF'), 'OFF')
  assert.strictEqual(digitalAlarmText({}, 0, 'OFF [IV]'), 'OFF [IV]')
})

const t0 = new Date('2026-01-01T00:00:00Z')
const fdA = {
  _id: 202,
  tag: 'KAW2~TR1~MW',
  type: 'analog',
  group1: 'KAW2',
  group2: 'TR1',
  description: 'KAW2~TR1~MW',
  priority: 2,
}

test('alarmDocument analog', () => {
  const d = alarmDocument(fdA, {
    value: 61,
    valueString: '61 MW',
    alarmText: '61 MW ⤉ 🚩 HIHI',
    alarmRange: 2,
    active: true,
    timeTag: t0,
  })
  assert.deepStrictEqual(Object.keys(d), [
    'tag', 'pointKey', 'group1', 'group2', 'description', 'type', 'priority',
    'value', 'valueString', 'alarmText', 'timeTag', 'timeTagAtSource',
    'timeTagAtSourceOk', 'active', 'ack', 'timeTagAck', 'ackUser',
    'timeTagNormal', 'alarmRange',
  ])
  assert.strictEqual(d.pointKey, 202)
  assert.strictEqual(d.value.valueOf(), 61)
  assert.strictEqual(d.alarmRange.valueOf(), 2)
  assert.strictEqual(d.ack, 0)
  assert.strictEqual(d.active, true)
  assert.strictEqual(d.timeTagNormal, null)
  assert.strictEqual(d.timeTagAtSource, null)
})

test('alarmDocument digital inactive', () => {
  const d = alarmDocument(
    { ...fdA, type: 'digital' },
    { value: 1, valueString: 'ON', alarmText: 'ON', active: false, timeTag: t0 }
  )
  assert.strictEqual('alarmRange' in d, false)
  assert.strictEqual(d.timeTagNormal, t0)
})

test('alarmOps', () => {
  assert.deepStrictEqual(alarmOps(5, false, null, t0), [])
  const ops = alarmOps(5, true, { tag: 'x' }, t0)
  assert.strictEqual(ops.length, 2)
  assert.deepStrictEqual(ops[0], {
    updateMany: {
      filter: { pointKey: 5, active: true },
      update: { $set: { active: false, timeTagNormal: t0 } },
    },
  })
  assert.deepStrictEqual(ops[1], { insertOne: { document: { tag: 'x' } } })
  assert.deepStrictEqual(Object.keys(alarmOps(5, false, { tag: 'x' }, t0)[0]), ['insertOne'])
})
