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

// Windows backend: manages driver services through NSSM (create/alter/remove/
// start/stop/restart) and reads state through sc.exe. Service names match those in
// platform-windows/create_services.bat so instance-1 services are adopted in place.

'use strict'

const paths = require('./paths')
const { run, ProcessError } = require('./exec')

const AUTO = 'SERVICE_AUTO_START'
const DEMAND = 'SERVICE_DEMAND_START'

function nssm() {
  return paths.nssmPath()
}

async function nssmRun(args, opts = {}) {
  // nssm writes UTF-16LE to its pipes.
  console.log('nssmRun', nssm(), args, opts)
  return run(nssm(), args, { utf16: true, ...opts })
}

async function scRun(args) {
  return run('sc', args, { utf16: false })
}

// Parses `sc query <name>` output into our state vocabulary; null when not installed.
function parseScState(res) {
  const text = (res.stdout || '') + (res.stderr || '')
  if (res.code === 1060 || /1060/.test(text) || /FAILED 1060/.test(text))
    return { installed: false, state: 'UNKNOWN' }
  const m = text.match(/STATE\s*:\s*\d+\s+(\w+)/)
  const raw = m ? m[1].toUpperCase() : 'UNKNOWN'
  const map = {
    RUNNING: 'RUNNING',
    STOPPED: 'STOPPED',
    START_PENDING: 'STARTING',
    STOP_PENDING: 'STOPPING',
    PAUSED: 'PAUSED',
  }
  return { installed: true, state: map[raw] || 'UNKNOWN' }
}

function parseScStartType(res) {
  const text = (res.stdout || '') + (res.stderr || '')
  const m = text.match(/START_TYPE\s*:\s*\d+\s+([\w_]+)/)
  const raw = m ? m[1].toUpperCase() : ''
  if (raw.includes('DISABLED')) return 'disabled'
  if (raw.includes('DEMAND')) return 'manual'
  if (raw.includes('AUTO')) return 'auto'
  return 'unknown'
}

async function status(spec) {
  const name = spec.winServiceName
  const q = await scRun(['query', name])
  const st = parseScState(q)
  const result = {
    serviceName: name,
    installed: st.installed,
    state: st.state,
    startMode: 'unknown',
    pid: null,
  }
  if (st.installed) {
    const qc = await scRun(['qc', name])
    result.startMode = parseScStartType(qc)
    const pidm = ((q.stdout || '') + (q.stderr || '')).match(/PID\s*:\s*(\d+)/)
    if (pidm) result.pid = parseInt(pidm[1])
  }
  return result
}

// Applies all NSSM parameters for a spec to an already-installed service.
async function applyParams(spec, startMode) {
  const name = spec.winServiceName
  const setCalls = [
    ['set', name, 'Application', spec.cmd],
    ['set', name, 'AppParameters', ...spec.args],
    ['set', name, 'AppDirectory', spec.cwd],
    ['set', name, 'AppStdout', spec.logFile],
    ['set', name, 'AppStderr', spec.logFile],
    ['set', name, 'AppRotateOnline', '1'],
    ['set', name, 'AppRotateBytes', '10000000'],
    ['set', name, 'AppRestartDelay', '2000'],
    ['set', name, 'Start', startMode === 'auto' ? AUTO : DEMAND],
    [
      'set',
      name,
      'AppEnvironmentExtra',
      'JS_SPEC_HASH=' + spec.specHash,
    ],
  ]
  for (const c of setCalls) {
    const r = await nssmRun(c)
    if (r.code !== 0)
      throw new ProcessError(
        'nssm ' + c.slice(0, 3).join(' ') + ' failed: ' + errText(r)
      )
  }
}

function errText(r) {
  return (r.stderr || r.stdout || '').replace(/\s+/g, ' ').trim() || 'exit ' + r.code
}

// Reads JS_SPEC_HASH stored in AppEnvironmentExtra to decide if a rewrite is needed.
async function installedSpecHash(spec) {
  const r = await nssmRun(['get', spec.winServiceName, 'AppEnvironmentExtra'])
  if (r.code !== 0) return null
  const m = (r.stdout || '').match(/JS_SPEC_HASH=([0-9a-f]+)/i)
  return m ? m[1] : null
}

async function ensureService(spec, opts = {}) {
  const name = spec.winServiceName
  const st = await status(spec)
  const startMode = opts.startMode || spec.defaultStartMode

  if (!st.installed) {
    const inst = await nssmRun(['install', name, spec.cmd, ...spec.args])
    if (inst.code !== 0)
      throw new ProcessError('nssm install failed: ' + errText(inst))
    await applyParams(spec, startMode)
    return { action: 'created', serviceName: name }
  }

  // installed: rewrite only if the spec changed or start mode differs
  const currentHash = await installedSpecHash(spec)
  if (currentHash === spec.specHash && st.startMode === startMode)
    return { action: 'unchanged', serviceName: name }

  await applyParams(spec, startMode)
  return { action: 'updated', serviceName: name, wasRunning: st.state === 'RUNNING' }
}

async function removeService(spec) {
  const st = await status(spec)
  if (!st.installed) return { action: 'absent', serviceName: spec.winServiceName }
  if (['RUNNING', 'STARTING', 'PAUSED', 'STOPPING'].includes(st.state))
    await nssmRun(['stop', spec.winServiceName])
  const r = await nssmRun(['remove', spec.winServiceName, 'confirm'])
  if (r.code !== 0) throw new ProcessError('nssm remove failed: ' + errText(r))
  return { action: 'removed', serviceName: spec.winServiceName }
}

async function start(spec) {
  const r = await nssmRun(['start', spec.winServiceName])
  // exit code 0 = started; nssm returns non-zero if already running — tolerate that
  if (r.code !== 0) {
    const st = await status(spec)
    if (st.state === 'RUNNING') return { action: 'already-running' }
    throw new ProcessError('nssm start failed: ' + errText(r))
  }
  return { action: 'started' }
}

async function stop(spec) {
  const r = await nssmRun(['stop', spec.winServiceName])
  if (r.code !== 0) {
    const st = await status(spec)
    if (st.state === 'STOPPED') return { action: 'already-stopped' }
    throw new ProcessError('nssm stop failed: ' + errText(r))
  }
  return { action: 'stopped' }
}

async function restart(spec) {
  const r = await nssmRun(['restart', spec.winServiceName])
  if (r.code !== 0) throw new ProcessError('nssm restart failed: ' + errText(r))
  return { action: 'restarted' }
}

// Sets a service to demand-start (used when disabling an instance so NSSM does not
// auto-restart the exited driver).
async function setManual(spec) {
  await nssmRun(['set', spec.winServiceName, 'Start', DEMAND])
}

// Lists installed JSON_SCADA_ service names (for orphan detection in reconcile).
async function listManagedServiceNames() {
  const r = await scRun(['query', 'type=', 'service', 'state=', 'all'])
  const text = (r.stdout || '') + (r.stderr || '')
  const names = []
  const re = /SERVICE_NAME:\s*(JSON_SCADA_\S+)/g
  let m
  while ((m = re.exec(text)) !== null) names.push(m[1])
  return names
}

module.exports = {
  status,
  ensureService,
  removeService,
  start,
  stop,
  restart,
  setManual,
  listManagedServiceNames,
  available: true,
}
