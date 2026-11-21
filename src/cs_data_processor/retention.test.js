'use strict'

// Mirrors ../cs_data_processor-go/retention_test.go; the expected values
// must stay identical in both.

const test = require('node:test')
const assert = require('node:assert')
const {
  retentionDays,
  retentionSettings,
  cutoff,
  alarmsCleanupFilter,
} = require('./retention')

test('retentionDays', () => {
  // [value, days]
  const cases = [
    [undefined, 60],
    [null, 60],
    ['30', 60],
    [0, 60],
    [-5, 60],
    [NaN, 60],
    [1, 1],
    [30, 30],
    [7.9, 7],
    [100000, 36500],
  ]
  for (const [v, w] of cases) assert.strictEqual(retentionDays(v), w, String(v))
})

test('retentionSettings', () => {
  assert.deepStrictEqual(retentionSettings(null), { soe: 60, alarms: 60, hist: 60 })
  assert.deepStrictEqual(
    retentionSettings({ soeRetentionDays: 10, alarmsRetentionDays: 400, histRetentionDays: 2 }),
    { soe: 10, alarms: 400, hist: 2 }
  )
})

test('cutoff', () => {
  assert.strictEqual(
    cutoff(new Date('2026-03-01T12:00:00Z'), 60).toISOString(),
    '2025-12-31T12:00:00.000Z'
  )
})

test('alarmsCleanupFilter', () => {
  const c = new Date('2026-01-01T00:00:00Z')
  assert.deepStrictEqual(alarmsCleanupFilter(c), {
    timeTag: { $lt: c },
    ack: { $ne: 0 },
    active: { $ne: true },
  })
})
