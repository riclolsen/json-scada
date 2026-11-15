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

// Settings of the MCP server's HTTP service (src/mcp-json-scada-db), kept in
// the systemSettings document { key: 'mcpServer' }. The MCP server reads them
// at startup in HTTP mode (jsonscada/service-settings.ts), where they take
// precedence over its JS_MCPJSDB_* environment; keep the field names in step.
// The sign-in password is stored for the MCP server but never sent back to a
// browser (adminPasswordSet tells whether one is stored).

'use strict'

const net = require('node:net')
const db = require('../models')

const KEY = 'mcpServer'

const DEFAULTS = {
  ipBind: '127.0.0.1',
  httpPort: 6001,
  allowedHosts: [],
  logLevel: 1,
  adminTools: true,
  adminUsername: '',
  adminUrl: 'http://127.0.0.1:8080',
  cimEmsTools: true,
}

const MAX_HOSTS = 50
const HOSTNAME_RE = /^[A-Za-z0-9]([A-Za-z0-9-]{0,62})(\.[A-Za-z0-9]([A-Za-z0-9-]{0,62}))*$/
const USERNAME_MAX = 100
const PASSWORD_MAX = 200

class SettingsError extends Error {}

function collection() {
  return db.systemSetting.collection
}

// Host name or IP address (IPv6 in brackets), without a port: the form the
// MCP SDK's Host/Origin guards compare against.
function validHost(h) {
  if (typeof h !== 'string' || h.length === 0 || h.length > 253) return false
  if (h.startsWith('[') && h.endsWith(']')) return net.isIPv6(h.slice(1, -1))
  return net.isIPv4(h) || HOSTNAME_RE.test(h)
}

// Address the HTTP server binds to: an IP address or localhost.
function validBind(a) {
  return typeof a === 'string' && (a === 'localhost' || net.isIP(a) !== 0)
}

function validUrl(u) {
  if (typeof u !== 'string' || u.length > 500) return false
  try {
    const url = new URL(u)
    return (
      (url.protocol === 'http:' || url.protocol === 'https:') &&
      url.username === '' &&
      url.password === '' &&
      url.search === '' &&
      url.hash === ''
    )
  } catch (e) {
    return false
  }
}

// The stored settings for a browser: defaults filled in, no password.
function publicView(doc) {
  const d = doc || {}
  const out = {}
  for (const k of Object.keys(DEFAULTS))
    out[k] = d[k] === undefined || d[k] === null ? DEFAULTS[k] : d[k]
  out.adminPasswordSet = typeof d.adminPassword === 'string' && d.adminPassword !== ''
  out.saved = !!doc
  return out
}

async function getStored() {
  return collection().findOne({ key: KEY })
}

async function getSettings() {
  return publicView(await getStored())
}

// Validates a change; returns the $set / $unset documents. Throws
// SettingsError with a message meant for the admin.
function buildUpdate(patch) {
  const set = {}
  const unset = {}
  const p = patch && typeof patch === 'object' ? patch : {}

  if (p.ipBind !== undefined) {
    const v = String(p.ipBind).trim()
    if (!validBind(v)) throw new SettingsError('Invalid bind address!')
    set.ipBind = v
  }
  if (p.httpPort !== undefined) {
    const v = Number(p.httpPort)
    if (!Number.isInteger(v) || v < 1 || v > 65535)
      throw new SettingsError('Invalid HTTP port!')
    set.httpPort = v
  }
  if (p.allowedHosts !== undefined) {
    if (!Array.isArray(p.allowedHosts) || p.allowedHosts.length > MAX_HOSTS)
      throw new SettingsError('Invalid allowed hosts list!')
    const hosts = []
    for (const h of p.allowedHosts) {
      const v = typeof h === 'string' ? h.trim().toLowerCase() : h
      if (!validHost(v))
        throw new SettingsError('Invalid allowed host: ' + String(h).slice(0, 80))
      if (!hosts.includes(v)) hosts.push(v)
    }
    set.allowedHosts = hosts
  }
  if (p.logLevel !== undefined) {
    const v = Number(p.logLevel)
    if (!Number.isInteger(v) || v < 0 || v > 3)
      throw new SettingsError('Invalid log level!')
    set.logLevel = v
  }
  for (const k of ['adminTools', 'cimEmsTools'])
    if (p[k] !== undefined) {
      if (typeof p[k] !== 'boolean') throw new SettingsError('Invalid ' + k + '!')
      set[k] = p[k]
    }
  if (p.adminUsername !== undefined) {
    if (typeof p.adminUsername !== 'string' || p.adminUsername.length > USERNAME_MAX)
      throw new SettingsError('Invalid user name!')
    set.adminUsername = p.adminUsername.trim()
  }
  if (p.adminUrl !== undefined) {
    const v = String(p.adminUrl).trim()
    if (!validUrl(v)) throw new SettingsError('Invalid AdminUI API address!')
    set.adminUrl = v.replace(/\/+$/, '')
  }
  // a password is only replaced when one is given, or removed on request
  if (p.clearAdminPassword === true) unset.adminPassword = ''
  else if (p.adminPassword !== undefined && p.adminPassword !== '') {
    if (typeof p.adminPassword !== 'string' || p.adminPassword.length > PASSWORD_MAX)
      throw new SettingsError('Invalid password!')
    set.adminPassword = p.adminPassword
  }
  return { set, unset }
}

async function updateSettings(patch) {
  const { set, unset } = buildUpdate(patch)
  const upd = { $set: { ...set, updatedAt: new Date() }, $setOnInsert: { key: KEY } }
  if (Object.keys(unset).length) upd.$unset = unset
  await collection().updateOne({ key: KEY }, upd, { upsert: true })
  return getSettings()
}

// Whether the sign-in user of the admin tools exists and holds an admin role:
// 'ok', 'notSet', 'notFound' or 'notAdmin'.
async function checkAdminUser(username) {
  if (!username) return 'notSet'
  const user = await db.user
    .findOne({ username: String(username) })
    .populate('roles', '-__v')
    .lean()
    .exec()
  if (!user) return 'notFound'
  const roles = Array.isArray(user.roles) ? user.roles : []
  return roles.some((r) => r && r.isAdmin === true) ? 'ok' : 'notAdmin'
}

// Whether something accepts TCP connections at the configured address on this
// host (a wildcard bind is probed on loopback).
function probe(bind, port, timeoutMs = 1000) {
  let host = bind
  if (bind === '0.0.0.0' || bind === '') host = '127.0.0.1'
  else if (bind === '::') host = '::1'
  return new Promise((resolve) => {
    const sock = net.connect({ host, port })
    const done = (ok) => {
      sock.destroy()
      resolve(ok)
    }
    sock.setTimeout(timeoutMs, () => done(false))
    sock.once('connect', () => done(true))
    sock.once('error', () => done(false))
  })
}

module.exports = {
  DEFAULTS,
  SettingsError,
  getSettings,
  updateSettings,
  checkAdminUser,
  probe,
  validHost,
}
