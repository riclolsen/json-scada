'use strict'

/*
 * Data retention: once a day removes the soeData and alarmsData documents
 * older than their retention and keeps the expiry of the hist time series
 * collection equal to its retention.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 *
 * Port of ../cs_data_processor-go/retention.go: change both together and
 * keep retention.test.js and retention_test.go with the same expected
 * values. The retentions (days) are the soeRetentionDays,
 * alarmsRetentionDays and histRetentionDays fields of the systemSettings
 * document { key: 'global' }, edited in the AdminUI (Admin / System) through
 * server_realtime_auth. Contract in docs/schema.md (systemSettings).
 *
 * Instance 1 on the active node only. Also creates the alarmsData indexes
 * and turns an old capped soeData into a normal collection.
 */

const Log = require('./simple-logger')
const Redundancy = require('./redundancy')

const DAY_MS = 24 * 60 * 60 * 1000
const DEFAULT_RETENTION_DAYS = 60
const MAX_RETENTION_DAYS = 36500
const FIRST_RUN_DELAY_MS = 60 * 1000 // after the start
const CHECK_MS = 60 * 60 * 1000 // a node that became active runs within an hour
const RUN_EVERY_MS = DAY_MS

// alarmsData indexes (also in mongo_seed/b_create-db.js)
const AlarmsIndexes = [
  [{ timeTag: 1 }, { name: 'alarmsTimeTagIndex' }],
  [{ pointKey: 1, active: 1 }, { name: 'alarmsPointActiveIndex' }],
  [{ ack: 1, timeTag: -1 }, { name: 'alarmsAckIndex' }],
  [{ active: 1, timeTag: -1 }, { name: 'alarmsActiveIndex' }],
  [{ group1: 1 }, { name: 'alarmsGroup1Index' }],
]
// soeData indexes (also in mongo_seed/b_create-db.js)
const SoeIndexes = [
  [{ timeTag: 1 }, {}],
  [{ timeTagAtSource: 1 }, {}],
  [{ group1: 1 }, {}],
  [{ ack: 1 }, {}],
]

// a retention in whole days: a missing or invalid value is the default
function retentionDays(v) {
  const n = typeof v === 'number' ? v : NaN
  if (!Number.isFinite(n) || n < 1) return DEFAULT_RETENTION_DAYS
  return Math.min(Math.trunc(n), MAX_RETENTION_DAYS)
}

// the retentions of the systemSettings document (null when absent)
function retentionSettings(doc) {
  return {
    soe: retentionDays(doc?.soeRetentionDays),
    alarms: retentionDays(doc?.alarmsRetentionDays),
    hist: retentionDays(doc?.histRetentionDays),
  }
}

// documents with a timeTag older than this are removed
function cutoff(now, days) {
  return new Date(now.getTime() - days * DAY_MS)
}

// alarms removed: older than the cutoff, acknowledged and inactive
function alarmsCleanupFilter(cut) {
  return { timeTag: { $lt: cut }, ack: { $ne: 0 }, active: { $ne: true } }
}

async function collectionInfo(db, name) {
  const list = await db.listCollections({ name: name }).toArray()
  return list.length > 0 ? list[0] : null
}

async function createIndexes(db, name, indexes) {
  for (const [keys, opts] of indexes)
    await db
      .collection(name)
      .createIndex(keys, opts)
      .catch((err) => Log.log('Retention - index of ' + name + ': ' + err.message))
}

// A capped soeData (older installs) becomes a normal collection: renamed
// first, so new events create the normal collection at once, then the
// events still within the retention are copied back.
async function uncapSoe(db, soeName, keepFrom) {
  const info = await collectionInfo(db, soeName)
  if (!info || !info.options?.capped) return
  const oldName = soeName + '_capped_' + Date.now()
  Log.log('Retention - converting capped ' + soeName + ' to a normal collection')
  await db.collection(soeName).rename(oldName)
  await db.createCollection(soeName).catch((err) => {
    if (err?.code !== 48) throw err // 48: created meanwhile by a new event
  })
  await createIndexes(db, soeName, SoeIndexes)
  await db
    .collection(oldName)
    .aggregate([
      { $match: { timeTag: { $gte: keepFrom } } },
      { $merge: { into: soeName, whenMatched: 'keepExisting', whenNotMatched: 'insert' } },
    ])
    .toArray()
  await db.collection(oldName).drop()
  Log.log('Retention - ' + soeName + ' converted')
}

// keeps the hist time series expiry equal to the retention
async function histExpiry(db, histName, days) {
  const info = await collectionInfo(db, histName)
  if (!info || !info.options?.timeseries) return
  const secs = days * 24 * 60 * 60
  if (info.options.expireAfterSeconds === secs) return
  await db.command({ collMod: histName, expireAfterSeconds: secs })
  Log.log('Retention - ' + histName + ' expiry set to ' + days + ' days')
}

async function cycle(db, jsConfig, now) {
  const settings = retentionSettings(
    await db
      .collection(jsConfig.SystemSettingsCollectionName)
      .findOne({ key: 'global' })
  )
  const soeCut = cutoff(now, settings.soe)
  await createIndexes(db, jsConfig.AlarmsDataCollectionName, AlarmsIndexes)
  await uncapSoe(db, jsConfig.SoeDataCollectionName, soeCut)

  const soe = await db
    .collection(jsConfig.SoeDataCollectionName)
    .deleteMany({ timeTag: { $lt: soeCut } })
  const alm = await db
    .collection(jsConfig.AlarmsDataCollectionName)
    .deleteMany(alarmsCleanupFilter(cutoff(now, settings.alarms)))
  await histExpiry(db, jsConfig.HistCollectionName, settings.hist)
  Log.log(
    'Retention - removed ' + soe.deletedCount + ' events (' + settings.soe +
      ' days), ' + alm.deletedCount + ' alarms (' + settings.alarms +
      ' days); hist ' + settings.hist + ' days'
  )
}

// starts the daily cleanup; getDb returns the current db handle or null
function Start(jsConfig, getDb) {
  if (jsConfig.Instance !== 1) {
    Log.log('Retention - done by instance 1 only', Log.levelDetailed)
    return
  }
  let running = false
  let lastRun = 0
  const tick = async function () {
    if (running || Date.now() - lastRun < RUN_EVERY_MS) return
    const db = getDb()
    if (!db || !Redundancy.ProcessStateIsActive()) return
    running = true
    try {
      await cycle(db, jsConfig, new Date())
      lastRun = Date.now()
    } catch (err) {
      Log.log('Retention - error: ' + err)
    } finally {
      running = false
    }
  }
  setTimeout(tick, FIRST_RUN_DELAY_MS)
  setInterval(tick, CHECK_MS)
}

module.exports = {
  Start,
  // exported for tests
  retentionDays,
  retentionSettings,
  cutoff,
  alarmsCleanupFilter,
  DEFAULT_RETENTION_DAYS,
  MAX_RETENTION_DAYS,
}
