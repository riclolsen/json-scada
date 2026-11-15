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

// Process manager facade. Single entry point used by the controllers: selects the
// platform backend (NSSM on Windows, supervisord on Linux), serializes all service
// operations, checks node ownership, and computes the spec hash used for idempotency.

'use strict'

const crypto = require('node:crypto')
const catalog = require('./driver-catalog')
const paths = require('./paths')
const nssm = require('./nssm')
const supervisor = require('./supervisor')
const Log = require('../../../simple-logger')
const LoadConfig = require('../../../load-config')
const { ProcessError } = require('./exec')

let configObj = null
function cfg() {
  if (!configObj) configObj = LoadConfig()
  return configObj
}
function pmCfg() {
  return (cfg().processManagement && typeof cfg().processManagement === 'object'
    ? cfg().processManagement
    : {})
}

function isEnabled() {
  if (process.env.JS_PROCESS_MGMT_DISABLE === 'true') return false
  return pmCfg().enabled !== false
}

function localNodeName() {
  return cfg().nodeName || ''
}

// Picks the backend module for this host, honoring an explicit config override.
function backend() {
  if (!isEnabled()) return null
  const b = pmCfg().backend || 'auto'
  if (b === 'none') return null
  if (b === 'nssm') return nssm
  if (b === 'supervisor') {
    supervisor.configure(cfg())
    return supervisor
  }
  if (process.platform === 'win32') return nssm
  supervisor.configure(cfg())
  return supervisor
}

// --- serialization: NSSM and supervisorctl are not concurrency-safe per service ---
let queue = Promise.resolve()
function enqueue(fn) {
  const p = queue.then(fn, fn)
  queue = p.then(
    () => {},
    () => {}
  )
  return p
}

function specHashOf(spec) {
  const material = JSON.stringify({
    cmd: spec.cmd,
    args: spec.args,
    env: spec.env,
    cwd: spec.cwd,
    logFile: spec.logFile,
  })
  return crypto.createHash('sha1').update(material).digest('hex').slice(0, 16)
}

// Builds the launch spec + hash for an instance, or null when not manageable here.
function makeSpec(instanceDoc) {
  const spec = catalog.buildServiceSpec(instanceDoc)
  if (!spec) return null
  spec.specHash = specHashOf(spec)
  return spec
}

// Returns { manageable, reason } considering platform, catalog, config and node.
function isManageable(instanceDoc) {
  if (!isEnabled())
    return { manageable: false, reason: 'process management disabled' }
  if (!backend())
    return { manageable: false, reason: 'no process backend on this host' }
  const m = catalog.manageability(instanceDoc.protocolDriver)
  if (!m.manageable) return m
  const nodes = Array.isArray(instanceDoc.nodeNames) ? instanceDoc.nodeNames : []
  const local = localNodeName()
  if (local && nodes.length > 0 && !nodes.includes(local))
    return {
      manageable: false,
      reason: 'instance runs on node(s) ' + nodes.join(', ') + ', not ' + local,
    }
  return { manageable: true, reason: '' }
}

function startModeFor(instanceDoc, spec) {
  // explicit per-instance setting wins; else disabled instances are manual; else catalog default
  const pm = instanceDoc.processManagement || {}
  if (pm.startMode === 'auto' || pm.startMode === 'manual') return pm.startMode
  if (instanceDoc.enabled === false) return 'manual'
  return spec.defaultStartMode
}

function isManagedFlag(instanceDoc) {
  const pm = instanceDoc.processManagement || {}
  return pm.managed !== false
}

// --- public operations -----------------------------------------------------------

async function status(instanceDoc) {
  const m = isManageable(instanceDoc)
  const spec = m.manageable ? makeSpec(instanceDoc) : null
  if (!m.manageable || !spec) {
    const base =
      catalog.serviceBaseName(
        instanceDoc.protocolDriver,
        instanceDoc.protocolDriverInstanceNumber
      ) || ''
    return {
      manageable: false,
      reason: m.reason,
      serviceName:
        process.platform === 'win32'
          ? base
            ? 'JSON_SCADA_' + base
            : ''
          : base,
      installed: false,
      state: 'UNKNOWN',
      startMode: 'unknown',
      node: localNodeName(),
    }
  }
  return enqueue(async () => {
    try {
      const st = await backend().status(spec)
      return { manageable: true, reason: '', node: localNodeName(), ...st }
    } catch (err) {
      Log.log('ProcessManager status error: ' + err.message)
      return {
        manageable: true,
        reason: '',
        serviceName: spec.serviceName,
        installed: false,
        state: 'UNKNOWN',
        startMode: 'unknown',
        node: localNodeName(),
        error: err.message,
      }
    }
  })
}

// Bulk status for a list of instance docs (each queued; backend calls are cheap).
async function listStatuses(instanceDocs) {
  const out = []
  for (const doc of instanceDocs) out.push(await status(doc))
  return out
}

async function ensureService(instanceDoc) {
  const m = isManageable(instanceDoc)
  if (!m.manageable) return { skipped: true, reason: m.reason }
  if (!isManagedFlag(instanceDoc))
    return { skipped: true, reason: 'instance marked not managed' }
  const spec = makeSpec(instanceDoc)
  const startMode = startModeFor(instanceDoc, spec)
  return enqueue(() => backend().ensureService(spec, { startMode }))
}

async function removeService(instanceDoc) {
  const spec = makeSpec(instanceDoc)
  if (!spec) return { skipped: true, reason: 'not manageable' }
  if (!backend()) return { skipped: true, reason: 'no backend' }
  return enqueue(() => backend().removeService(spec))
}

async function start(instanceDoc) {
  const m = isManageable(instanceDoc)
  if (!m.manageable) throw new ProcessError(m.reason)
  const spec = makeSpec(instanceDoc)
  return enqueue(async () => {
    const st = await backend().status(spec)
    if (!st.installed) {
      await backend().ensureService(spec, {
        startMode: startModeFor(instanceDoc, spec),
      })
    }
    return backend().start(spec)
  })
}

async function stop(instanceDoc) {
  const m = isManageable(instanceDoc)
  if (!m.manageable) throw new ProcessError(m.reason)
  const spec = makeSpec(instanceDoc)
  return enqueue(() => backend().stop(spec))
}

async function restart(instanceDoc) {
  const m = isManageable(instanceDoc)
  if (!m.manageable) throw new ProcessError(m.reason)
  const spec = makeSpec(instanceDoc)
  return enqueue(async () => {
    const st = await backend().status(spec)
    if (!st.installed) {
      await backend().ensureService(spec, {
        startMode: startModeFor(instanceDoc, spec),
      })
      return backend().start(spec)
    }
    return backend().restart(spec)
  })
}

// Applies the effect of enabling/disabling an instance to its service.
async function applyEnabledState(instanceDoc) {
  const m = isManageable(instanceDoc)
  if (!m.manageable) return { skipped: true, reason: m.reason }
  if (!isManagedFlag(instanceDoc))
    return { skipped: true, reason: 'instance marked not managed' }
  const spec = makeSpec(instanceDoc)
  const b = backend()
  return enqueue(async () => {
    if (instanceDoc.enabled === false) {
      await b.setManual(spec)
      return b.stop(spec)
    }
    await b.ensureService(spec, { startMode: startModeFor(instanceDoc, spec) })
    if (startModeFor(instanceDoc, spec) === 'auto') return b.start(spec)
    return { action: 'ensured' }
  })
}

// Reconciles the service for an instance after its document changed. Handles driver
// type changes (remove old service), enable/disable transitions, and config edits
// (rewrite spec + restart when running). Best-effort; throws only on backend failure.
async function applyInstanceUpdate(oldDoc, newDoc) {
  const b = backend()
  if (!b) return { skipped: true, reason: 'no backend on this host' }

  // driver type changed: retire the old service before creating the new one
  if (
    oldDoc &&
    oldDoc.protocolDriver &&
    oldDoc.protocolDriver !== newDoc.protocolDriver
  ) {
    try {
      const oldSpec = makeSpec(oldDoc)
      if (oldSpec) await enqueue(() => b.removeService(oldSpec))
    } catch (err) {
      Log.log('applyInstanceUpdate remove-old error: ' + err.message)
    }
  }

  const m = isManageable(newDoc)
  if (!m.manageable) return { skipped: true, reason: m.reason }
  if (!isManagedFlag(newDoc))
    return { skipped: true, reason: 'instance marked not managed' }

  const spec = makeSpec(newDoc)
  const startMode = startModeFor(newDoc, spec)
  const wasEnabled = oldDoc ? oldDoc.enabled !== false : false

  return enqueue(async () => {
    const st = await b.status(spec)

    if (newDoc.enabled === false) {
      await b.ensureService(spec, { startMode: 'manual' })
      await b.setManual(spec)
      if (['RUNNING', 'STARTING', 'PAUSED', 'STOPPING'].includes(st.state))
        await b.stop(spec)
      return { action: 'disabled' }
    }

    const ens = await b.ensureService(spec, { startMode })
    if (!wasEnabled && startMode === 'auto') {
      await b.start(spec)
      return { action: 'enabled-started' }
    }
    if (ens.action === 'created' && startMode === 'auto') {
      await b.start(spec)
      return { action: 'created-started' }
    }
    if (ens.action === 'updated' && st.state === 'RUNNING') {
      await b.restart(spec)
      return { action: 'restarted' }
    }
    return { action: ens.action }
  })
}

// Ensures services for every manageable local instance; reports orphans (managed
// service names with no matching instance). Only removes orphans when asked.
async function reconcileAll(instanceDocs, opts = {}) {
  const b = backend()
  if (!b) return { skipped: true, reason: 'no backend on this host' }
  const results = { ensured: [], errors: [], orphans: [], removed: [] }
  const expected = new Set()

  for (const doc of instanceDocs) {
    const m = isManageable(doc)
    if (!m.manageable) continue
    const spec = makeSpec(doc)
    expected.add(spec.serviceName)
    if (!isManagedFlag(doc)) continue
    try {
      const r = await enqueue(() =>
        b.ensureService(spec, { startMode: startModeFor(doc, spec) })
      )
      results.ensured.push({ serviceName: spec.serviceName, ...r })
    } catch (err) {
      results.errors.push({ serviceName: spec.serviceName, error: err.message })
    }
  }

  try {
    const installed = await enqueue(() => b.listManagedServiceNames())
    for (const name of installed) {
      if (!expected.has(name)) {
        results.orphans.push(name)
      }
    }
  } catch (err) {
    Log.log('ProcessManager reconcile list error: ' + err.message)
  }

  return results
}

// --- core services -------------------------------------------------------------
// Services installed by the platform scripts (not protocol drivers), controlled
// by name only: status, start, stop, restart. Their definitions (command line,
// environment, start type) stay owned by create_services.bat / the supervisor
// .ini files, so these never create, rewrite or remove a service.
const CORE_SERVICES = {
  mcp: { winServiceName: 'JSON_SCADA_mcp_server', programName: 'mcp_server' },
  // instance 1 of the CIM-EMS driver (same names as driver-catalog.js) and its demo field simulator
  cimEms: { winServiceName: 'JSON_SCADA_cim_ems', programName: 'cim_ems' },
  cimEmsDemoField: {
    winServiceName: 'JSON_SCADA_cim_ems_demo_field',
    programName: 'cim_ems_demo_field',
  },
}

function coreSpec(key) {
  const s = CORE_SERVICES[key]
  if (!s) throw new ProcessError('Unknown service')
  return {
    ...s,
    serviceName: process.platform === 'win32' ? s.winServiceName : s.programName,
  }
}

async function coreStatus(key) {
  const spec = coreSpec(key)
  const b = backend()
  if (!b)
    return {
      manageable: false,
      reason: isEnabled() ? 'no process backend on this host' : 'process management disabled',
      serviceName: spec.serviceName,
      installed: false,
      state: 'UNKNOWN',
      node: localNodeName(),
    }
  return enqueue(async () => {
    try {
      const st = await b.status(spec)
      return {
        manageable: true,
        reason: '',
        node: localNodeName(),
        serviceName: st.serviceName,
        installed: st.installed,
        state: st.state,
        startMode: st.startMode,
      }
    } catch (err) {
      Log.log('ProcessManager core status error: ' + err.message)
      return {
        manageable: true,
        reason: '',
        node: localNodeName(),
        serviceName: spec.serviceName,
        installed: false,
        state: 'UNKNOWN',
        error: err.message,
      }
    }
  })
}

async function coreControl(key, action) {
  if (!['start', 'stop', 'restart'].includes(action))
    throw new ProcessError('Unknown action')
  const spec = coreSpec(key)
  const b = backend()
  if (!b)
    throw new ProcessError(
      isEnabled() ? 'no process backend on this host' : 'process management disabled'
    )
  return enqueue(async () => {
    const st = await b.status(spec)
    if (!st.installed)
      throw new ProcessError('service ' + spec.serviceName + ' is not installed on this host')
    if (action === 'restart' && st.state !== 'RUNNING') return b.start(spec)
    return b[action](spec)
  })
}

module.exports = {
  ProcessError,
  coreStatus,
  coreControl,
  isEnabled,
  isManageable,
  status,
  listStatuses,
  ensureService,
  removeService,
  applyEnabledState,
  start,
  stop,
  restart,
  reconcileAll,
  applyInstanceUpdate,
  localNodeName,
  makeSpec,
}
