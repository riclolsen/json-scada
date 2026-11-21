/*
 * Queries of the alarm list (alarmsData), shared by the /Invoke/ route
 * auth/listAlarms and GraphQL.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 *
 * The data processor (cs_data_processor, both implementations) writes one
 * document per alarm occurrence; acknowledgements are written by the
 * /Invoke/ WriteRequest (ExtendedAlarmEventsAck) and the GraphQL ackAlarms
 * mutation. Contract in docs/schema.md (alarmsData).
 *
 * Realtime: the alarms not acknowledged or still active, newest first.
 * Historical: every alarm raised in a time interval, oldest first.
 */

'use strict'

const MAX_ALARM_ROWS = 100000
const DEFAULT_REALTIME_ROWS = 5000
const DEFAULT_HISTORICAL_ROWS = 10000
const MAX_TEXT_FILTER = 200
const DAY_MS = 24 * 60 * 60 * 1000

const escapeRegex = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

function dateOf(v) {
  if (typeof v !== 'string' && typeof v !== 'number') return null
  const d = new Date(v)
  return isNaN(d.getTime()) ? null : d
}

// The query of a request, or { error } when invalid.
// params: { mode: 'realtime' | 'historical', timeBegin, timeEnd (historical,
// ISO dates), group1: [station...], priorityMax, text (substring of tag,
// description or alarm text), unackedOnly, limit }
// group1List: the stations the user is restricted to ([] = all)
function alarmListQuery(params, group1List) {
  const p = params && typeof params === 'object' ? params : {}
  const historical = p.mode === 'historical'
  const and = []

  if (Array.isArray(group1List) && group1List.length > 0)
    and.push({ group1: { $in: group1List } })
  if (Array.isArray(p.group1) && p.group1.length > 0) {
    const g = p.group1.filter((s) => typeof s === 'string').slice(0, 1000)
    and.push({ group1: { $in: g } })
  }
  if (p.priorityMax !== undefined && p.priorityMax !== null) {
    if (typeof p.priorityMax !== 'number' || !Number.isFinite(p.priorityMax))
      return { error: 'priorityMax must be a number' }
    and.push({ priority: { $lte: p.priorityMax } })
  }
  if (p.text !== undefined && p.text !== null && p.text !== '') {
    if (typeof p.text !== 'string' || p.text.length > MAX_TEXT_FILTER)
      return { error: 'text must be a string of up to ' + MAX_TEXT_FILTER + ' characters' }
    const re = { $regex: escapeRegex(p.text.trim()), $options: 'i' }
    and.push({ $or: [{ tag: re }, { description: re }, { alarmText: re }, { group1: re }] })
  }
  if (p.unackedOnly === true) and.push({ ack: 0 })

  let limit = historical ? DEFAULT_HISTORICAL_ROWS : DEFAULT_REALTIME_ROWS
  if (p.limit !== undefined) {
    const n = parseInt(p.limit, 10)
    if (!isNaN(n) && n > 0) limit = Math.min(n, MAX_ALARM_ROWS)
  }

  let sort
  if (historical) {
    const end = p.timeEnd === undefined ? new Date() : dateOf(p.timeEnd)
    const begin =
      p.timeBegin === undefined ? new Date(Date.now() - DAY_MS) : dateOf(p.timeBegin)
    if (!begin || !end) return { error: 'timeBegin and timeEnd must be dates' }
    and.push({ timeTag: { $gte: begin, $lte: end } })
    sort = { timeTag: 1, _id: 1 }
  } else {
    and.push({ $or: [{ ack: 0 }, { active: true }] })
    sort = { timeTag: -1, _id: -1 }
  }
  return { filter: and.length === 1 ? and[0] : { $and: and }, sort, limit }
}

// an alarmsData document as returned to clients
function alarmRow(d) {
  return {
    _id: String(d._id),
    tag: d.tag,
    pointKey: d.pointKey,
    group1: d.group1,
    group2: d.group2,
    description: d.description,
    type: d.type,
    priority: d.priority,
    value: d.value,
    valueString: d.valueString,
    alarmText: d.alarmText,
    alarmRange: d.alarmRange,
    timeTag: d.timeTag,
    timeTagAtSource: d.timeTagAtSource,
    timeTagAtSourceOk: d.timeTagAtSourceOk,
    active: d.active === true,
    ack: d.ack,
    timeTagAck: d.timeTagAck ?? null,
    ackUser: d.ackUser ?? null,
    timeTagNormal: d.timeTagNormal ?? null,
  }
}

// runs the query: { alarms, limited } or { error }
async function listAlarms(mdb, params, group1List) {
  const q = alarmListQuery(params, group1List)
  if (q.error) return { error: q.error }
  const docs = await mdb
    .collection('alarmsData')
    .find(q.filter)
    .sort(q.sort)
    .limit(q.limit + 1)
    .toArray()
  const limited = docs.length > q.limit
  return { alarms: docs.slice(0, q.limit).map(alarmRow), limited }
}

module.exports = {
  alarmListQuery,
  alarmRow,
  listAlarms,
  MAX_ALARM_ROWS,
}
