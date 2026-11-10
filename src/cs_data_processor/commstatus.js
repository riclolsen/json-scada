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

// Communication status tags: the health of every protocol driver instance
// and protocol connection, materialized as ordinary realtimeData tags under
// the _System.Driver., _System.Conn. and _System.Comm. name spaces.
//
// Port of ../cs_data_processor-go/commstatus.go: same tag names, keys,
// fields, number types and timing. Keep the two identical; the contract is
// documented in docs/schema.md (Communication status tags).
//
// Only processor instance 1, while active, maintains the tags. Values are
// written as sourceDataUpdate so this processor's own change stream converts
// them (alarms, SOE, beep, history).

'use strict'

const { Double } = require('mongodb')
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')
const Redundancy = require('./redundancy')

const CYCLE_MS = 5000
const COUNT_EVERY = 3 // tag counts every 3 cycles (15 s)
const STALL_LIMIT = 4 // stopped after more than 4 unchanged keep-alives
const FIRST_SEEN_GRACE_MS = 30000
const KEY_CEILING = -1000
const FIELD = 'commStatus'

const KIND_DRIVER = 'driver'
const KIND_CONN = 'conn'
const KIND_SUMMARY = 'summary'

// ---------------------------------------------------------------------
// pure helpers (exported for tests)
// ---------------------------------------------------------------------

function sanitize(s) {
  s = String(s ?? '')
    .trim()
    .replace(/[^A-Za-z0-9_-]/g, '_')
  return s === '' ? '_' : s
}

function numOk(v) {
  if (typeof v === 'number') return [v, true]
  if (v && typeof v === 'object') {
    if (v._bsontype === 'Double' || v._bsontype === 'Int32')
      return [Number(v.valueOf()), true]
    if (v._bsontype === 'Long') return [v.toNumber(), true]
    if (v._bsontype === 'Decimal128') {
      const f = parseFloat(v.toString())
      return [f, !isNaN(f)]
    }
  }
  return [0, false]
}
const num = (v) => numOk(v)[0]
const str = (v) => (typeof v === 'string' ? v : '')
const bool = (v, def) => (typeof v === 'boolean' ? v : def)
const isDate = (v) => v instanceof Date && !isNaN(v.getTime())

function idString(v) {
  if (v && typeof v === 'object' && typeof v.toHexString === 'function')
    return v.toHexString()
  if (typeof v === 'string') return v
  const [f, ok] = numOk(v)
  if (ok) return String(f)
  return String(v)
}

const round = (v) => Math.round(v)

// keep-alive stall detection, same rule as the Go csStall
class Stall {
  constructor() {
    this.seen = false
    this.last = 0
    this.unchanged = 0
    this.lastAdvance = 0
  }
  // returns [alive, ageSeconds]
  observe(v, has, now) {
    if (!has) {
      this.seen = false
      return [false, 0]
    }
    const t = v.getTime()
    const n = now.getTime()
    if (!this.seen) {
      this.seen = true
      this.last = t
      this.unchanged = 0
      this.lastAdvance = Math.min(t, n)
      const alive = n - t < FIRST_SEEN_GRACE_MS
      if (!alive) this.unchanged = STALL_LIMIT + 1
      return [alive, Math.max(0, (n - this.lastAdvance) / 1000)]
    }
    if (t === this.last) {
      this.unchanged++
    } else {
      this.unchanged = 0
      this.last = t
      this.lastAdvance = n
    }
    return [
      this.unchanged <= STALL_LIMIT,
      Math.max(0, (n - this.lastAdvance) / 1000),
    ]
  }
}

const specKey = (s) => s.kind + '|' + s.ref + '|' + s.point
const b2f = (b) => (b ? 1 : 0)

class CommStatus {
  constructor(instance) {
    this.instance = instance
    this.instStall = new Map()
    this.statsStall = new Map()
    this.counts = new Map()
    this.written = new Map()
    this.cycleCount = 0
    this.nextKey = 0
    this.disabled = false
  }

  // computes every desired point; snap = { instances, connections, counts (Map|null) }
  build(snap, mirror, now) {
    if (snap.counts) this.counts = snap.counts
    const pts = []
    const add = (spec, val) => pts.push({ spec, val })
    const blank = {
      stateTrue: '',
      stateFalse: '',
      unit: '',
      alarm: false,
      priority: 0,
      noHistory: false,
      deadband: 0,
    }

    const instByKey = new Map()
    const seenInst = new Set()
    let driversDown = 0

    for (const inst of snap.instances) {
      seenInst.add(inst.id)
      let st = this.instStall.get(inst.id)
      if (!st) {
        st = new Stall()
        this.instStall.set(inst.id, st)
      }
      const [alive, age] = st.observe(inst.keepAlive, inst.hasKeepAlive, now)
      instByKey.set(inst.driver + '|' + String(inst.number), {
        alive,
        enabled: inst.enabled,
        activeNode: inst.activeNode,
      })
      if (inst.enabled && !alive) driversDown++

      const base =
        '_System.Driver.' +
        sanitize(inst.driver) +
        '.' +
        sanitize(String(inst.number)) +
        '.'
      const g3 = inst.driver + ' #' + String(inst.number)
      const mk = (point, type, text, extra) => ({
        ...blank,
        kind: KIND_DRIVER,
        ref: inst.id,
        point,
        tag: base + point,
        type,
        group2: 'Drivers',
        group3: g3,
        text,
        ...extra,
      })
      add(
        mk('Alive', 'digital', 'Running', {
          stateTrue: 'Running',
          stateFalse: 'Stopped',
          alarm: true,
          priority: 2,
        }),
        { value: b2f(alive), str: '', invalid: !inst.hasKeepAlive }
      )
      add(
        mk('Enabled', 'digital', 'Enabled', {
          stateTrue: 'Enabled',
          stateFalse: 'Disabled',
        }),
        { value: b2f(inst.enabled), str: '', invalid: false }
      )
      add(mk('ActiveNode', 'string', 'Active Node'), {
        value: 0,
        str: inst.activeNode,
        invalid: !inst.hasKeepAlive,
      })
      add(mk('Version', 'string', 'Software Version'), {
        value: 0,
        str: inst.version,
        invalid: inst.version === '',
      })
      add(
        mk('KeepAliveAge', 'analog', 'Keep-Alive Age', {
          unit: 's',
          noHistory: true,
          deadband: 5,
        }),
        { value: round(age), str: '', invalid: !inst.hasKeepAlive }
      )
    }
    for (const id of [...this.instStall.keys()])
      if (!seenInst.has(id)) this.instStall.delete(id)

    let connsDown = 0
    const seenConn = new Set()
    for (const c of snap.connections) {
      seenConn.add(c.id)
      let st = this.statsStall.get(c.id)
      if (!st) {
        st = new Stall()
        this.statsStall.set(c.id, st)
      }
      const [, statsAge] = st.observe(c.statsTime, c.hasStatsTime, now)
      const inst = instByKey.get(c.driver + '|' + String(c.instNumber))
      const hasInst = inst !== undefined
      const hasCnt = this.counts.has(c.number)
      const cnt = hasCnt ? this.counts.get(c.number) : { total: 0, invalid: 0 }
      let health = 0
      if (hasCnt && cnt.total > 0)
        health = Math.round((1000 * (cnt.total - cnt.invalid)) / cnt.total) / 10
      const status =
        c.enabled &&
        hasInst &&
        inst.enabled &&
        inst.alive &&
        (c.isConnected === null || c.isConnected) &&
        (!hasCnt || cnt.total === 0 || health > 0)
      if (c.enabled && !status) connsDown++
      let activeNode = c.statsNode
      if (activeNode === '' && hasInst) activeNode = inst.activeNode

      const base = '_System.Conn.' + sanitize(c.name) + '.'
      const mk = (point, type, text, extra) => ({
        ...blank,
        kind: KIND_CONN,
        ref: c.id,
        point,
        tag: base + point,
        type,
        group2: 'Connections',
        group3: c.name,
        text,
        ...extra,
      })
      const countsInvalid = !hasCnt && snap.counts !== null
      add(
        mk('Status', 'digital', 'Status', {
          stateTrue: 'Online',
          stateFalse: 'Offline',
          alarm: true,
          priority: 1,
        }),
        { value: b2f(status), str: '', invalid: false }
      )
      add(
        mk('Enabled', 'digital', 'Enabled', {
          stateTrue: 'Enabled',
          stateFalse: 'Disabled',
        }),
        { value: b2f(c.enabled), str: '', invalid: false }
      )
      add(
        mk('LinkUp', 'digital', 'Link', {
          stateTrue: 'Link Up',
          stateFalse: 'Link Down',
        }),
        c.isConnected !== null
          ? { value: b2f(c.isConnected), str: '', invalid: false }
          : { value: 0, str: '', invalid: true }
      )
      add(
        mk('StatsAge', 'analog', 'Statistics Age', {
          unit: 's',
          noHistory: true,
          deadband: 5,
        }),
        { value: round(statsAge), str: '', invalid: !c.hasStatsTime }
      )
      add(mk('ActiveNode', 'string', 'Active Node'), {
        value: 0,
        str: activeNode,
        invalid: activeNode === '',
      })
      add(mk('TagCount', 'analog', 'Tag Count', { unit: 'Tags' }), {
        value: cnt.total,
        str: '',
        invalid: countsInvalid,
      })
      add(mk('InvalidTags', 'analog', 'Invalid Tags', { unit: 'Tags' }), {
        value: cnt.invalid,
        str: '',
        invalid: countsInvalid,
      })
      add(mk('Health', 'analog', 'Health', { unit: '%', deadband: 1 }), {
        value: health,
        str: '',
        invalid: !hasCnt || cnt.total === 0,
      })

      if (mirror) {
        for (const k of Object.keys(c.numStats).sort()) {
          add(
            mk('Stats.' + sanitize(k), 'analog', 'Stats ' + k, {
              noHistory: true,
            }),
            { value: c.numStats[k], str: '', invalid: false }
          )
        }
      }
    }
    for (const id of [...this.statsStall.keys()])
      if (!seenConn.has(id)) this.statsStall.delete(id)

    const sum = (point, text, unit, v) =>
      add(
        {
          ...blank,
          kind: KIND_SUMMARY,
          ref: '',
          point,
          tag: '_System.Comm.' + point,
          type: 'analog',
          group2: 'Comm',
          group3: '',
          text,
          unit,
        },
        { value: v, str: '', invalid: false }
      )
    sum('DriversDown', 'Drivers Down', 'Drivers', driversDown)
    sum('ConnectionsDown', 'Connections Down', 'Connections', connsDown)
    sum(
      'ConnectionsTotal',
      'Connections Total',
      'Connections',
      snap.connections.length
    )
    return pts
  }

  // ------------------------------------------------------------------
  // database side
  // ------------------------------------------------------------------

  async cycle(db, jsConfig) {
    const rt = db.collection(jsConfig.RealtimeDataCollectionName)
    const { enabled, mirror } = await this.readFlags(db, jsConfig)
    if (!enabled) {
      if (!this.disabled) {
        this.disabled = true
        Log.log('Comm status tags - disabled, invalidating existing tags')
        await this.invalidateAll(rt)
      }
      return
    }
    if (this.disabled) {
      Log.log('Comm status tags - enabled')
      this.disabled = false
      this.written = new Map()
    }
    const withCounts = this.cycleCount % COUNT_EVERY === 0
    this.cycleCount++
    const snap = await this.readSnapshot(db, jsConfig, rt, withCounts)
    const desired = this.build(snap, mirror, new Date())
    await this.apply(rt, desired)
  }

  async readFlags(db, jsConfig) {
    let enabled = true
    let mirror = false
    const doc = await db
      .collection(jsConfig.ProcessInstancesCollectionName)
      .findOne({
        processName: AppDefs.NAME,
        processInstanceNumber: new Double(this.instance),
      })
    if (doc) {
      if (typeof doc.commStatusTags === 'boolean') enabled = doc.commStatusTags
      if (typeof doc.commStatusMirrorStats === 'boolean')
        mirror = doc.commStatusMirrorStats
    }
    return { enabled, mirror }
  }

  async readSnapshot(db, jsConfig, rt, withCounts) {
    const snap = { instances: [], connections: [], counts: null }
    const insts = await db
      .collection(jsConfig.ProtocolDriverInstancesCollectionName)
      .find({})
      .toArray()
    for (const d of insts) {
      const ka = d.activeNodeKeepAliveTimeTag
      snap.instances.push({
        id: idString(d._id),
        driver: str(d.protocolDriver),
        number: num(d.protocolDriverInstanceNumber),
        enabled: bool(d.enabled, true),
        activeNode: str(d.activeNodeName),
        version: str(d.softwareVersion),
        keepAlive: isDate(ka) ? ka : null,
        hasKeepAlive: isDate(ka),
      })
    }
    const conns = await db
      .collection(jsConfig.ProtocolConnectionsCollectionName)
      .find(
        {},
        {
          projection: {
            protocolConnectionNumber: 1,
            protocolConnectionName: 1,
            protocolDriver: 1,
            protocolDriverInstanceNumber: 1,
            enabled: 1,
            stats: 1,
          },
        }
      )
      .toArray()
    for (const d of conns) {
      const c = {
        id: idString(d._id),
        number: num(d.protocolConnectionNumber),
        name: str(d.protocolConnectionName),
        driver: str(d.protocolDriver),
        instNumber: num(d.protocolDriverInstanceNumber),
        enabled: bool(d.enabled, true),
        statsTime: null,
        hasStatsTime: false,
        statsNode: '',
        isConnected: null,
        numStats: {},
      }
      if (c.name === '') c.name = 'Connection' + String(c.number)
      const stats = d.stats && typeof d.stats === 'object' ? d.stats : {}
      for (const [k, v] of Object.entries(stats)) {
        if (k === 'timeTag') {
          c.hasStatsTime = isDate(v)
          c.statsTime = c.hasStatsTime ? v : null
        } else if (k === 'nodeName') c.statsNode = str(v)
        else if (k === 'isConnected') {
          if (typeof v === 'boolean') c.isConnected = v
        } else {
          const [f, ok] = numOk(v)
          if (ok) c.numStats[k] = f
        }
      }
      snap.connections.push(c)
    }
    if (withCounts) {
      try {
        snap.counts = await readTagCounts(rt)
      } catch (err) {
        Log.log('Comm status tags - error counting tags: ' + err)
      }
    }
    return snap
  }

  async readExisting(rt) {
    const docs = await rt
      .find(
        { [FIELD]: { $exists: true } },
        {
          projection: {
            _id: 1,
            tag: 1,
            [FIELD]: 1,
            value: 1,
            valueString: 1,
            invalid: 1,
          },
        }
      )
      .toArray()
    const out = new Map()
    for (const d of docs) {
      const m = d[FIELD] || {}
      out.set(str(m.kind) + '|' + str(m.ref) + '|' + str(m.point), {
        id: d._id,
        tag: str(d.tag),
        value: {
          value: num(d.value),
          str: str(d.valueString),
          invalid: bool(d.invalid, false),
        },
      })
    }
    return out
  }

  async apply(rt, pts) {
    const existing = await this.readExisting(rt)
    const now = new Date()
    const wanted = new Set()
    let created = 0

    for (const p of pts) {
      const k = specKey(p.spec)
      wanted.add(k)
      let ex = existing.get(k)
      const found = ex !== undefined
      if (!found) {
        let id
        try {
          id = await this.create(rt, p.spec)
        } catch (err) {
          Log.log(
            'Comm status tags - error creating ' + p.spec.tag + ': ' + err
          )
          continue
        }
        created++
        ex = {
          id: new Double(id),
          tag: p.spec.tag,
          value: { value: 0, str: '', invalid: true },
        }
      }
      if (ex.tag !== p.spec.tag) {
        try {
          await rt.updateOne({ _id: ex.id }, { $set: namingFields(p.spec) })
          Log.log(
            'Comm status tags - renamed ' + ex.tag + ' to ' + p.spec.tag
          )
        } catch (err) {
          Log.log(
            'Comm status tags - error renaming ' +
              ex.tag +
              ' to ' +
              p.spec.tag +
              ': ' +
              err
          )
        }
      }

      // a new tag is always written once, so it leaves its creation state
      const known = this.written.has(k)
      if (found && known && !changed(p.spec, this.written.get(k), p.val))
        continue
      if (found && !known && !changed(p.spec, ex.value, p.val)) {
        this.written.set(k, ex.value)
        continue
      }
      try {
        await write(rt, ex.id, p, now)
        this.written.set(k, p.val)
      } catch (err) {
        Log.log('Comm status tags - error updating ' + p.spec.tag + ': ' + err)
      }
    }
    if (created > 0) Log.log('Comm status tags - created ' + created + ' tags')

    const stale = []
    for (const [k, ex] of existing) {
      if (!wanted.has(k)) {
        stale.push(ex.id)
        this.written.delete(k)
        Log.log('Comm status tags - removing ' + ex.tag)
      }
    }
    if (stale.length > 0) {
      try {
        await rt.deleteMany({ _id: { $in: stale } })
      } catch (err) {
        Log.log('Comm status tags - error removing tags: ' + err)
      }
    }
  }

  async create(rt, spec) {
    if (this.nextKey === 0) await this.loadNextKey(rt)
    const id = this.nextKey
    try {
      await rt.insertOne(tagDocument(id, spec))
    } catch (err) {
      // a key or tag name clash: re-read the lowest key next time
      this.nextKey = 0
      throw err
    }
    this.nextKey--
    return id
  }

  async loadNextKey(rt) {
    const doc = await rt.findOne(
      { _id: { $lte: KEY_CEILING } },
      { sort: { _id: 1 }, projection: { _id: 1 } }
    )
    let lowest = KEY_CEILING
    if (doc) {
      const [f, ok] = numOk(doc._id)
      if (ok && f < lowest) lowest = f
    }
    this.nextKey = lowest - 1
  }

  async invalidateAll(rt) {
    let existing
    try {
      existing = await this.readExisting(rt)
    } catch (err) {
      Log.log('Comm status tags - error reading tags: ' + err)
      return
    }
    const now = new Date()
    for (const ex of existing.values()) {
      const p = {
        spec: { type: 'analog', alarm: false },
        val: { value: ex.value.value, str: ex.value.str, invalid: true },
      }
      try {
        await write(rt, ex.id, p, now)
      } catch (err) {
        Log.log('Comm status tags - error invalidating ' + ex.tag + ': ' + err)
      }
    }
    this.written = new Map()
  }
}

async function readTagCounts(rt) {
  const rows = await rt
    .aggregate([
      { $match: { origin: 'supervised' } },
      {
        $group: {
          _id: '$protocolSourceConnectionNumber',
          total: { $sum: 1 },
          invalid: { $sum: { $cond: [{ $eq: ['$invalid', true] }, 1, 0] } },
        },
      },
    ])
    .toArray()
  const out = new Map()
  for (const r of rows) {
    const [n, ok] = numOk(r._id)
    if (!ok) continue
    out.set(n, { total: num(r.total), invalid: num(r.invalid) })
  }
  return out
}

// whether v must be written over last
function changed(spec, last, v) {
  if (last.invalid !== v.invalid) return true
  if (spec.type === 'string') return last.str !== v.str
  if (spec.type === 'analog' && spec.deadband > 0)
    return Math.abs(last.value - v.value) >= spec.deadband
  return last.value !== v.value
}

async function write(rt, id, p, now) {
  const sdu = {
    valueAtSource: new Double(p.val.value),
    valueStringAtSource: p.val.str,
    invalidAtSource: p.val.invalid,
    timeTag: now,
  }
  if (p.spec.type === 'digital' && p.spec.alarm) {
    // a source time tag makes the change an SOE event
    sdu.timeTagAtSource = now
    sdu.timeTagAtSourceOk = true
  } else {
    sdu.timeTagAtSource = null
    sdu.timeTagAtSourceOk = false
  }
  await rt.updateOne({ _id: id }, { $set: { sourceDataUpdate: sdu } })
}

function namingFields(s) {
  let desc = '_System~' + s.group2
  if (s.group3 !== '') desc += '~' + s.group3
  desc += '~' + s.text
  return {
    tag: s.tag,
    description: desc,
    ungroupedDescription: s.text,
    group3: s.group3,
  }
}

// a new generated tag, on the same base (and field order) as the Go
// implementation's specialTagBase
function tagDocument(id, s) {
  const D = (v) => new Double(v)
  const doc = {
    _id: D(id),
    alarmRange: D(0),
    alarmDisabled: true,
    alarmState: D(1),
    alarmed: false,
    annotation: '',
    commandBlocked: false,
    commandOfSupervised: D(0),
    formula: null,
    frozen: false,
    frozenDetectTimeout: D(300),
    group1: '_System',
    group2: 'Status',
    group3: '',
    hiLimit: null,
    hihiLimit: null,
    hihihiLimit: null,
    historianDeadBand: D(0),
    historianPeriod: D(0),
    hysteresis: D(0),
    invalid: true,
    invalidDetectTimeout: D(0),
    isEvent: false,
    kconv1: D(1),
    kconv2: D(0),
    loLimit: null,
    location: null,
    loloLimit: null,
    lololoLimit: null,
    notes: '',
    origin: 'system',
    overflow: false,
    parcels: null,
    priority: D(3),
    protocolSourceASDU: D(0),
    protocolSourceCommandDuration: null,
    protocolSourceCommandUseSBO: null,
    protocolSourceCommonAddress: D(0),
    protocolSourceConnectionNumber: D(0),
    protocolSourceObjectAddress: D(0),
    sourceDataUpdate: null,
    substituted: false,
    supervisedOfCommand: D(0),
    timeTag: new Date('2000-01-01T00:00:00.000Z'),
    transient: false,
    type: 'analog',
    updatesCnt: D(0),
    value: D(0),
    valueDefault: D(0),
    timeTagAtSource: null,
    timeTagAtSourceOk: null,
  }
  Object.assign(doc, namingFields(s))
  doc.group2 = s.group2
  doc.type = s.type
  doc.unit = s.unit
  doc.alarmDisabled = !s.alarm
  if (s.priority > 0) doc.priority = D(s.priority)
  if (s.noHistory) doc.historianPeriod = D(-1)
  if (s.type === 'analog') {
    doc.historianDeadBand = D(s.deadband)
    doc.frozenDetectTimeout = D(0)
  }
  doc.stateTextTrue = s.stateTrue
  doc.stateTextFalse = s.stateFalse
  doc.eventTextTrue = s.stateTrue
  doc.eventTextFalse = s.stateFalse
  doc.valueString = ''
  // a null timeTag makes the processor accept the first update even when
  // it carries the creation value and quality
  doc.timeTag = null
  doc[FIELD] = { kind: s.kind, ref: s.ref, point: s.point }
  return doc
}

// starts the maintenance; getDb returns the current db handle or null
function Start(jsConfig, getDb) {
  if (jsConfig.Instance !== 1) {
    Log.log('Comm status tags - maintained by instance 1 only', Log.levelDetailed)
    return
  }
  const cs = new CommStatus(jsConfig.Instance)
  let running = false
  setInterval(async function () {
    if (running) return
    const db = getDb()
    if (!db || !Redundancy.ProcessStateIsActive()) return
    running = true
    try {
      await cs.cycle(db, jsConfig)
    } catch (err) {
      Log.log('Comm status tags - error: ' + err)
    } finally {
      running = false
    }
  }, CYCLE_MS)
}

module.exports = {
  Start,
  // exported for tests
  CommStatus,
  Stall,
  sanitize,
  changed,
  tagDocument,
  STALL_LIMIT,
  CYCLE_MS,
}
