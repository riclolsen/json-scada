'use strict'

// Mirrors ../cs_data_processor-go/limits_test.go; the expected values must
// stay identical in both.

const test = require('node:test')
const assert = require('node:assert')
const { hasLimits, alarmRangeOf, rangeLabel } = require('./limits')

const all = {
  lololoLimit: -30,
  loloLimit: -20,
  loLimit: -10,
  hiLimit: 10,
  hihiLimit: 20,
  hihihiLimit: 30,
}

// [limits, value, hysteresis, previous alarmRange, range, normal, escalated]
const cases = [
  [all, 5, 1, 0, 0, true, false],
  [all, 11.5, 1, 0, 1, false, true],
  [all, 10.5, 1, 0, 0, false, false],
  [all, 10.5, 1, 1, 1, false, false],
  [all, 9.5, 1, 1, 1, false, false],
  [all, 8.5, 1, 1, 0, true, false],
  [all, 21.5, 1, 1, 2, false, true],
  [all, 20.5, 1, 1, 1, false, false],
  [all, 19.5, 1, 2, 2, false, false],
  [all, 18.5, 1, 2, 1, false, false],
  [all, 35, 1, 0, 3, false, true],
  [all, 35, 1, 2, 3, false, true],
  [all, -11.5, 1, 0, -1, false, true],
  [all, -21.5, 1, -1, -2, false, true],
  [all, -19.5, 1, -2, -2, false, false],
  [all, -18.5, 1, -2, -1, false, false],
  [all, -11.5, 1, 2, -1, false, true],
  [all, -10.5, 1, 1, 0, true, false],
  [all, 5, 1, -3, 0, true, false],
  [all, 35, 1, 7, 3, false, false],
  [all, 10, 0, 0, 0, false, false],
  [all, 10, 0, 1, 1, false, false],
  [{ hihiLimit: 20 }, 15, 1, 0, 0, true, false],
  [{ hihiLimit: 20 }, 25, 1, 0, 2, false, true],
  [{ hiLimit: null, loLimit: null, hihiLimit: 20 }, -100, 1, 0, 0, true, false],
  [{ hiLimit: 10, loLimit: -10 }, 50, 1, 0, 1, false, true],
  [{ hiLimit: 10, loLimit: -10 }, 50, 1, null, 1, false, true],
]

test('alarmRangeOf', () => {
  for (const [doc, value, hyst, prev, range, normal, escalated] of cases)
    assert.deepStrictEqual(
      alarmRangeOf({ ...doc, alarmRange: prev }, value, hyst, prev),
      { range, normal, escalated },
      JSON.stringify({ doc, value, hyst, prev })
    )
})

test('hasLimits', () => {
  assert.strictEqual(hasLimits({}), false)
  assert.strictEqual(hasLimits({ hiLimit: null, loLimit: null }), false)
  assert.strictEqual(hasLimits({ hiLimit: NaN }), false)
  assert.strictEqual(hasLimits({ lololoLimit: -5 }), true)
  assert.strictEqual(hasLimits({ hiLimit: Infinity }), true)
})

test('rangeLabel', () => {
  const want = { '-3': 'LOLOLO', '-2': 'LOLO', '-1': 'LO', 0: '', 1: 'HI', 2: 'HIHI', 3: 'HIHIHI' }
  for (const [r, w] of Object.entries(want)) assert.strictEqual(rangeLabel(Number(r)), w)
})
