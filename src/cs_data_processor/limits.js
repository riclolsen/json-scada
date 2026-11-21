'use strict'

/*
 * Out-of-range alarm levels of analog tags.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 *
 * Port of ../cs_data_processor-go/limits.go: change both together and keep
 * limits.test.js and limits_test.go with the same expected values.
 *
 * alarmRange: 1, 2, 3 = above hiLimit, hihiLimit, hihihiLimit;
 * -1, -2, -3 = below loLimit, loloLimit, lololoLimit; 0 = normal.
 * A level whose limit is missing, null or not a number is not checked.
 */

const HiLimits = ['hiLimit', 'hihiLimit', 'hihihiLimit']
const LoLimits = ['loLimit', 'loloLimit', 'lololoLimit']
const RangeLabels = {
  1: 'HI',
  2: 'HIHI',
  3: 'HIHIHI',
  '-1': 'LO',
  '-2': 'LOLO',
  '-3': 'LOLOLO',
}

// a limit as a number, null when the level is not configured
function limitOf(v) {
  if (typeof v === 'number') return isNaN(v) ? null : v
  if (v && typeof v === 'object' && v._bsontype === 'Decimal128') {
    const n = parseFloat(v.toString())
    return isNaN(n) ? null : n
  }
  return null
}

// true when the document has at least one limit configured
function hasLimits(doc) {
  return [...HiLimits, ...LoLimits].some((k) => limitOf(doc?.[k]) !== null)
}

// Most severe level the value violates, the limits moved by offset (outwards
// when positive). inclusive: a value equal to a moved limit violates it.
function limitLevel(doc, value, offset, inclusive) {
  for (let i = HiLimits.length - 1; i >= 0; i--) {
    const l = limitOf(doc?.[HiLimits[i]])
    if (l === null) continue
    if (inclusive ? value >= l + offset : value > l + offset) return i + 1
  }
  for (let i = LoLimits.length - 1; i >= 0; i--) {
    const l = limitOf(doc?.[LoLimits[i]])
    if (l === null) continue
    if (inclusive ? value <= l - offset : value < l - offset) return -(i + 1)
  }
  return 0
}

// New alarm range of a value from the previous range and the hysteresis.
// A level is entered when the value passes its limit by more than the
// hysteresis and left when it comes back by more than the hysteresis, so
// a value in a hysteresis band keeps the previous range.
// Returns { range, normal, escalated }: normal when the range is 0 and the
// value left an alarm range or is clear of every hysteresis band; escalated
// when the range moved to a more severe level (or to the other side), which
// is a new alarm.
function alarmRangeOf(doc, value, hysteresis, prevRange) {
  const p = limitOf(prevRange)
  // levels go from -3 to 3
  const prev = p === null ? 0 : Math.max(-3, Math.min(3, Math.trunc(p)))
  const entry = limitLevel(doc, value, hysteresis, false)
  const hold = limitLevel(doc, value, -hysteresis, true)
  let range = 0
  let escalated = false
  if (
    entry !== 0 &&
    (prev === 0 ||
      Math.sign(entry) !== Math.sign(prev) ||
      Math.abs(entry) > Math.abs(prev))
  ) {
    range = entry
    escalated = true
  } else if (prev > 0) range = Math.min(Math.max(hold, 0), prev)
  else if (prev < 0) range = Math.max(Math.min(hold, 0), prev)
  return { range, normal: range === 0 && (hold === 0 || prev !== 0), escalated }
}

// text of an alarm range for events: HI, HIHI, ..., '' for normal
function rangeLabel(range) {
  return RangeLabels[range] || ''
}

module.exports = {
  HiLimits,
  LoLimits,
  limitOf,
  hasLimits,
  limitLevel,
  alarmRangeOf,
  rangeLabel,
}
