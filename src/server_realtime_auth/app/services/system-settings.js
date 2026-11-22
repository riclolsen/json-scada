/*
 * {json:scada} - Copyright (c) 2020-2025 - Ricardo L. Olsen
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

// Cached accessor for the singleton global settings document. Used by the process
// management hooks to decide whether service actions run automatically.

'use strict'

const db = require('../models')
const SystemSetting = db.systemSetting

// retention limits, same as cs_data_processor retention.js / retention.go
const RETENTION_FIELDS = [
  'soeRetentionDays',
  'alarmsRetentionDays',
  'histRetentionDays',
  'secEventsRetentionDays',
]
const MIN_RETENTION_DAYS = 1
const MAX_RETENTION_DAYS = 36500

const DEFAULTS = {
  autoManageServices: true,
  autoRestartOnConnectionChange: true,
  soeRetentionDays: 60,
  alarmsRetentionDays: 60,
  histRetentionDays: 60,
  secEventsRetentionDays: 60,
}

// a retention in whole days, or undefined when invalid
function retentionDays(v) {
  return Number.isInteger(v) && v >= MIN_RETENTION_DAYS && v <= MAX_RETENTION_DAYS
    ? v
    : undefined
}

// what is wrong with the retentions of a patch, or null
function retentionProblem(patch) {
  for (const k of RETENTION_FIELDS)
    if (patch?.[k] !== undefined && retentionDays(patch[k]) === undefined)
      return k + ' must be a whole number of days from ' + MIN_RETENTION_DAYS +
        ' to ' + MAX_RETENTION_DAYS
  return null
}

let cache = null
let cacheTime = 0
const TTL_MS = 5000

async function getSettings() {
  if (cache && Date.now() - cacheTime < TTL_MS) return cache
  try {
    const doc = await SystemSetting.findOne({ key: 'global' }).lean().exec()
    cache = { ...DEFAULTS, ...(doc || {}) }
  } catch (e) {
    cache = { ...DEFAULTS }
  }
  cacheTime = Date.now()
  return cache
}

async function updateSettings(patch) {
  const allowed = {}
  if (typeof patch.autoManageServices === 'boolean')
    allowed.autoManageServices = patch.autoManageServices
  if (typeof patch.autoRestartOnConnectionChange === 'boolean')
    allowed.autoRestartOnConnectionChange = patch.autoRestartOnConnectionChange
  for (const k of RETENTION_FIELDS)
    if (retentionDays(patch[k]) !== undefined) allowed[k] = patch[k]
  const doc = await SystemSetting.findOneAndUpdate(
    { key: 'global' },
    { $set: allowed, $setOnInsert: { key: 'global' } },
    { upsert: true, new: true }
  )
    .lean()
    .exec()
  cache = { ...DEFAULTS, ...doc }
  cacheTime = Date.now()
  return cache
}

function invalidate() {
  cache = null
}

module.exports = {
  getSettings,
  updateSettings,
  invalidate,
  retentionProblem,
  DEFAULTS,
  RETENTION_FIELDS,
  MIN_RETENTION_DAYS,
  MAX_RETENTION_DAYS,
}
