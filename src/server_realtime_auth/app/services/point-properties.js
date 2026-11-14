/*
 * Checks of the operator properties written to a point (annotation, notes,
 * alarm disabling, limits, substituted value) before they are stored.
 * {json:scada} - Copyright (c) 2020-2025 - Ricardo L. Olsen
 *
 * Shared by the /Invoke/ realtime API and GraphQL. The drivers, the data
 * processor and the displays read these fields: a limit must be a number (or
 * absent), an annotation a string (it blocks commands when not empty), and
 * texts are kept short enough for a point document.
 */

'use strict'

const MAX_ANNOTATION_LENGTH = 2000
const MAX_NOTES_LENGTH = 10000

const NUMBER_RE = /^\s*[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?\s*$/

// a finite number, or a string holding one (older clients); undefined otherwise
function finiteNumber(v) {
  if (typeof v === 'number') return Number.isFinite(v) ? v : undefined
  if (typeof v === 'string' && NUMBER_RE.test(v)) {
    const n = Number(v)
    return Number.isFinite(n) ? n : undefined
  }
  return undefined
}

// What is wrong with these properties, or null when they may be stored.
// Absent (undefined) fields are not checked: they are left unchanged.
function propertyProblem(props) {
  if (!props || typeof props !== 'object') return null
  if (props.alarmDisabled !== undefined && typeof props.alarmDisabled !== 'boolean')
    return 'alarmDisabled must be true or false'
  for (const [k, max] of [
    ['annotation', MAX_ANNOTATION_LENGTH],
    ['notes', MAX_NOTES_LENGTH],
  ]) {
    const v = props[k]
    if (v === undefined) continue
    if (typeof v !== 'string') return k + ' must be a string'
    if (v.length > max) return k + ' too long (maximum ' + max + ' characters)'
  }
  // a null limit (an Infinity serialized as JSON) means no limit
  for (const k of ['loLimit', 'hiLimit'])
    if (props[k] !== undefined && props[k] !== null && finiteNumber(props[k]) === undefined)
      return k + ' must be a number'
  if (props.hysteresis !== undefined) {
    const h = finiteNumber(props.hysteresis)
    if (h === undefined || h < 0) return 'hysteresis must be a number not below zero'
  }
  if (props.newValue !== undefined && finiteNumber(props.newValue) === undefined)
    return 'newValue must be a number'
  return null
}

module.exports = {
  propertyProblem,
  finiteNumber,
  MAX_ANNOTATION_LENGTH,
  MAX_NOTES_LENGTH,
}
