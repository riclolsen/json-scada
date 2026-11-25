/*
 * Queries of the security events (secEvents) for the Security Events viewer,
 * route auth/listSecEvents (role right viewSecEvents).
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 *
 * The protocol drivers write the events (go-common/jssecevents, the IEC
 * 61850 client and server); the data processor removes those older than
 * systemSettings secEventsRetentionDays. Contract in docs/schema.md
 * (secEvents). This server only reads them.
 */

'use strict'

const MAX_ROWS = 10000
const DEFAULT_ROWS = 1000
const MAX_TEXT_FILTER = 200
const MAX_COUNT = 100000
const DAY_MS = 24 * 60 * 60 * 1000
const SEVERITIES = ['info', 'notice', 'warning', 'alert', 'critical']

const escapeRegex = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')

function dateOf(v) {
  if (typeof v !== 'string' && typeof v !== 'number') return null
  const d = new Date(v)
  return isNaN(d.getTime()) ? null : d
}

function stringList(v, max) {
  if (!Array.isArray(v)) return null
  return v.filter((s) => typeof s === 'string' && s.length <= 200).slice(0, max)
}

// The query of a request, or { error } when invalid.
// params: { timeBegin, timeEnd (ISO dates, default the last 24 hours),
// minSeverity (0 info .. 4 critical), categories: [...], kinds: [...],
// protocolDriver, connectionNumber, source ('library' | 'driver'),
// text (substring of kind, peer, object, detail, connection name), limit }
function secEventsQuery(params) {
  const p = params && typeof params === 'object' ? params : {}
  const and = []

  const end = p.timeEnd === undefined || p.timeEnd === null ? new Date() : dateOf(p.timeEnd)
  const begin =
    p.timeBegin === undefined || p.timeBegin === null
      ? new Date(Date.now() - DAY_MS)
      : dateOf(p.timeBegin)
  if (!begin || !end) return { error: 'timeBegin and timeEnd must be dates' }
  and.push({ timeTag: { $gte: begin, $lte: end } })

  if (p.minSeverity !== undefined && p.minSeverity !== null) {
    if (!Number.isInteger(p.minSeverity) || p.minSeverity < 0 || p.minSeverity > 4)
      return { error: 'minSeverity must be an integer from 0 to 4' }
    if (p.minSeverity > 0) and.push({ severityLevel: { $gte: p.minSeverity } })
  }
  const cats = stringList(p.categories, 50)
  if (cats && cats.length) and.push({ category: { $in: cats } })
  const kinds = stringList(p.kinds, 200)
  if (kinds && kinds.length) and.push({ kind: { $in: kinds } })
  if (typeof p.protocolDriver === 'string' && p.protocolDriver !== '')
    and.push({ protocolDriver: p.protocolDriver.slice(0, 100) })
  if (p.connectionNumber !== undefined && p.connectionNumber !== null) {
    if (typeof p.connectionNumber !== 'number' || !Number.isFinite(p.connectionNumber))
      return { error: 'connectionNumber must be a number' }
    and.push({ protocolConnectionNumber: p.connectionNumber })
  }
  if (p.source === 'library' || p.source === 'driver') and.push({ source: p.source })
  if (p.text !== undefined && p.text !== null && p.text !== '') {
    if (typeof p.text !== 'string' || p.text.length > MAX_TEXT_FILTER)
      return { error: 'text must be a string of up to ' + MAX_TEXT_FILTER + ' characters' }
    const re = { $regex: escapeRegex(p.text.trim()), $options: 'i' }
    and.push({
      $or: [
        { kind: re },
        { peerAddress: re },
        { peerIp: re },
        { peerMac: re },
        { objectRef: re },
        { detail: re },
        { error: re },
        { connectionName: re },
        { value: re },
      ],
    })
  }

  let limit = DEFAULT_ROWS
  if (p.limit !== undefined) {
    const n = parseInt(p.limit, 10)
    if (!isNaN(n) && n > 0) limit = Math.min(n, MAX_ROWS)
  }
  return {
    filter: and.length === 1 ? and[0] : { $and: and },
    sort: { timeTag: -1, _id: -1 },
    limit,
  }
}

// The events of a request, newest first, with the count per severity of
// everything that matches (counted up to MAX_COUNT).
async function listSecEvents(db, params) {
  const q = secEventsQuery(params)
  if (q.error) return { error: q.error }
  const coll = db.collection('secEvents')
  const [events, counts] = await Promise.all([
    coll.find(q.filter).sort(q.sort).limit(q.limit).toArray(),
    coll
      .aggregate([
        { $match: q.filter },
        { $limit: MAX_COUNT },
        { $group: { _id: '$severityLevel', n: { $sum: 1 } } },
      ])
      .toArray(),
  ])
  const bySeverity = {}
  let total = 0
  for (const s of SEVERITIES) bySeverity[s] = 0
  for (const c of counts) {
    const name = SEVERITIES[c._id] || 'info'
    bySeverity[name] += c.n
    total += c.n
  }
  return {
    events,
    total,
    totalCapped: total >= MAX_COUNT,
    truncated: events.length >= q.limit && total > events.length,
    bySeverity,
  }
}

module.exports = { secEventsQuery, listSecEvents, SEVERITIES, MAX_ROWS }
