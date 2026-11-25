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

// Windows backend: manages driver services through WinSW (create/alter/remove/
// start/stop/restart) and reads state through sc.exe. Same layout as
// platform-windows/winsw-services.ps1: each service is <servicesDir>/<name>.xml
// next to <name>.exe, a hard link to (or copy of) WinSW.exe. Service names match
// those of platform-windows/service-templates so instance-1 services are adopted
// in place; a service of the same name run by something else (e.g. NSSM) is
// deleted and recreated under WinSW.

'use strict'

const fs = require('node:fs')
const path = require('node:path')
const paths = require('./paths')
const { run, ProcessError } = require('./exec')

const LONG_TIMEOUT_MS = 90000 // stopwait/restart wait for the service stop timeout

function wrapperPath(spec) {
  return path.join(paths.winswServicesDir(), spec.winServiceName + '.exe')
}

function xmlPath(spec) {
  return path.join(paths.winswServicesDir(), spec.winServiceName + '.xml')
}

async function scRun(args) {
  return run('sc', args, { utf16: false })
}

async function winswRun(spec, args, opts = {}) {
  return run(wrapperPath(spec), args, { utf16: false, ...opts })
}

function errText(r) {
  return (
    (r.stderr || r.stdout || '').replace(/\s+/g, ' ').trim() || 'exit ' + r.code
  )
}

// Parses `sc query <name>` output into our state vocabulary.
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

function parseScStartType(text) {
  const m = text.match(/START_TYPE\s*:\s*\d+\s+([\w_]+)/)
  const raw = m ? m[1].toUpperCase() : ''
  if (raw.includes('DISABLED')) return 'disabled'
  if (raw.includes('DEMAND')) return 'manual'
  if (raw.includes('AUTO')) return 'auto'
  return 'unknown'
}

// Executable of the service binary path reported by `sc qc` (quoted or not).
function parseScBinary(text) {
  const m = text.match(/BINARY_PATH_NAME\s*:\s*(.+)/)
  if (!m) return ''
  const v = m[1].trim()
  if (v.startsWith('"')) return v.slice(1, v.indexOf('"', 1))
  const i = v.toLowerCase().indexOf('.exe')
  return i >= 0 ? v.slice(0, i + 4) : v.split(' ')[0]
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
    winsw: false,
  }
  if (st.installed) {
    const qc = await scRun(['qc', name])
    const text = (qc.stdout || '') + (qc.stderr || '')
    result.startMode = parseScStartType(text)
    result.winsw =
      path.resolve(parseScBinary(text)).toLowerCase() ===
        path.resolve(wrapperPath(spec)).toLowerCase() &&
      fs.existsSync(wrapperPath(spec))
    const pidm = ((q.stdout || '') + (q.stderr || '')).match(/PID\s*:\s*(\d+)/)
    if (pidm) result.pid = parseInt(pidm[1])
  }
  return result
}

// --- XML configuration -----------------------------------------------------

function xmlEscape(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&apos;')
}

// Quotes one argument for a Windows command line (CommandLineToArgvW rules).
function quoteArg(a) {
  a = String(a)
  if (a !== '' && !/[\s"]/.test(a)) return a
  let out = '"'
  let bs = 0
  for (const ch of a) {
    if (ch === '\\') {
      bs++
      continue
    }
    if (ch === '"') out += '\\'.repeat(bs * 2 + 1) + '"'
    else out += '\\'.repeat(bs) + ch
    bs = 0
  }
  return out + '\\'.repeat(bs * 2) + '"'
}

function buildXml(spec, startMode) {
  const name = xmlEscape(spec.winServiceName)
  const logName = path.basename(spec.logFile).replace(/\.log$/i, '')
  const env = { ...(spec.env || {}), JS_SPEC_HASH: spec.specHash }
  const lines = [
    '<service>',
    '  <!-- managed by the JSON-SCADA process manager (AdminUI): rewritten when the instance changes -->',
    `  <id>${name}</id>`,
    `  <name>${name}</name>`,
    `  <description>JSON-SCADA ${xmlEscape(spec.driverName || '')} instance ${xmlEscape(spec.instanceNumber || 1)}</description>`,
    `  <executable>${xmlEscape(spec.cmd)}</executable>`,
  ]
  if (spec.args && spec.args.length)
    lines.push(
      `  <arguments>${xmlEscape(spec.args.map(quoteArg).join(' '))}</arguments>`
    )
  lines.push(`  <workingdirectory>${xmlEscape(spec.cwd)}</workingdirectory>`)
  for (const [k, v] of Object.entries(env))
    lines.push(`  <env name="${xmlEscape(k)}" value="${xmlEscape(v)}"/>`)
  lines.push(
    `  <startmode>${startMode === 'auto' ? 'Automatic' : 'Manual'}</startmode>`,
    `  <logpath>${xmlEscape(path.dirname(spec.logFile))}</logpath>`,
    `  <logname>${xmlEscape(logName)}</logname>`,
    '  <log mode="roll-by-size">',
    '    <sizeThreshold>10240</sizeThreshold>',
    '    <keepFiles>1</keepFiles>',
    '  </log>',
    '  <onfailure action="restart" delay="2 sec"/>',
    '  <resetfailure>1 hour</resetfailure>',
    '  <stoptimeout>15 sec</stoptimeout>',
    '  <stopparentprocessfirst>true</stopparentprocessfirst>',
    '</service>',
    ''
  )
  return lines.join('\r\n')
}

function writeXml(spec, startMode) {
  try {
    fs.mkdirSync(paths.winswServicesDir(), { recursive: true })
    fs.writeFileSync(xmlPath(spec), buildXml(spec, startMode), 'utf8')
  } catch (err) {
    throw new ProcessError('cannot write ' + xmlPath(spec) + ': ' + err.message)
  }
}

// Reads JS_SPEC_HASH stored in the service XML to decide if a rewrite is needed.
function installedSpecHash(spec) {
  try {
    const m = fs
      .readFileSync(xmlPath(spec), 'utf8')
      .match(/<env\s+name="JS_SPEC_HASH"\s+value="([0-9a-f]+)"/i)
    return m ? m[1] : null
  } catch {
    return null
  }
}

// Creates <name>.exe as a hard link to WinSW.exe (no extra disk space), else a copy.
// An outdated wrapper is replaced (only possible while the service is stopped).
function ensureWrapper(spec) {
  const src = paths.winswPath()
  if (!fs.existsSync(src)) throw new ProcessError('WinSW not found: ' + src)
  const dst = wrapperPath(spec)
  if (fs.existsSync(dst)) {
    if (fs.statSync(dst).size === fs.statSync(src).size) return
    try {
      fs.unlinkSync(dst)
    } catch {
      return // in use: keep the running wrapper
    }
  }
  try {
    fs.linkSync(src, dst)
  } catch {
    try {
      fs.copyFileSync(src, dst)
    } catch (err) {
      throw new ProcessError('cannot create ' + dst + ': ' + err.message)
    }
  }
}

function removeFiles(spec) {
  for (const f of [wrapperPath(spec), xmlPath(spec)])
    try {
      fs.unlinkSync(f)
    } catch {
      /* absent */
    }
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

async function waitFor(spec, pred, timeoutMs) {
  const deadline = Date.now() + timeoutMs
  for (;;) {
    const st = await status(spec)
    if (pred(st)) return true
    if (Date.now() > deadline) return false
    await sleep(500)
  }
}

// Stops and deletes a service not run by our WinSW wrapper (NSSM, sc-created).
async function deleteForeign(spec) {
  const name = spec.winServiceName
  await scRun(['stop', name])
  await waitFor(spec, (s) => !s.installed || s.state === 'STOPPED', 30000)
  const st = await status(spec)
  if (st.installed && st.pid)
    await run('taskkill', ['/F', '/T', '/PID', String(st.pid)], {
      utf16: false,
    })
  const r = await scRun(['delete', name])
  if (!(await waitFor(spec, (s) => !s.installed, 30000)))
    throw new ProcessError(
      'sc delete ' + name + ' did not complete: ' + errText(r)
    )
}

async function install(spec, startMode) {
  writeXml(spec, startMode)
  ensureWrapper(spec)
  const r = await winswRun(spec, ['install'])
  const st = await status(spec)
  if (r.code !== 0 || !st.installed)
    throw new ProcessError(
      'WinSW install of ' + spec.winServiceName + ' failed: ' + errText(r)
    )
}

async function setStartMode(spec, startMode) {
  const r = await scRun([
    'config',
    spec.winServiceName,
    'start=',
    startMode === 'auto' ? 'auto' : 'demand',
  ])
  if (r.code !== 0) throw new ProcessError('sc config failed: ' + errText(r))
}

// --- backend interface -------------------------------------------------------

async function ensureService(spec, opts = {}) {
  const name = spec.winServiceName
  const st = await status(spec)
  const startMode = opts.startMode || spec.defaultStartMode

  if (!st.installed) {
    await install(spec, startMode)
    return { action: 'created', serviceName: name }
  }

  // same name but not our WinSW wrapper (e.g. a former NSSM service): replace it
  if (!st.winsw) {
    await deleteForeign(spec)
    await install(spec, startMode)
    return {
      action: 'updated',
      serviceName: name,
      wasRunning: st.state === 'RUNNING',
    }
  }

  // installed: rewrite only if the spec changed or start mode differs. WinSW reads
  // the XML at each start; the start mode is a service setting, set through sc.
  ensureWrapper(spec)
  if (installedSpecHash(spec) === spec.specHash && st.startMode === startMode)
    return { action: 'unchanged', serviceName: name }

  writeXml(spec, startMode)
  if (st.startMode !== startMode) await setStartMode(spec, startMode)
  return {
    action: 'updated',
    serviceName: name,
    wasRunning: st.state === 'RUNNING',
  }
}

async function removeService(spec) {
  const st = await status(spec)
  if (!st.installed) {
    removeFiles(spec)
    return { action: 'absent', serviceName: spec.winServiceName }
  }
  if (!st.winsw) await deleteForeign(spec)
  else {
    if (['RUNNING', 'STARTING', 'PAUSED', 'STOPPING'].includes(st.state))
      await winswRun(spec, ['stopwait'], { timeout: LONG_TIMEOUT_MS })
    const r = await winswRun(spec, ['uninstall'])
    if (!(await waitFor(spec, (s) => !s.installed, 30000)))
      throw new ProcessError('WinSW uninstall failed: ' + errText(r))
  }
  removeFiles(spec)
  return { action: 'removed', serviceName: spec.winServiceName }
}

async function start(spec) {
  const st = await status(spec)
  if (st.state === 'RUNNING') return { action: 'already-running' }
  const r =
    st.winsw ?
      await winswRun(spec, ['start'])
    : await scRun(['start', spec.winServiceName])
  if (r.code !== 0) {
    const now = await status(spec)
    if (now.state === 'RUNNING' || now.state === 'STARTING')
      return { action: 'started' }
    throw new ProcessError(
      'start of ' + spec.winServiceName + ' failed: ' + errText(r)
    )
  }
  return { action: 'started' }
}

async function stop(spec) {
  const st = await status(spec)
  if (!st.installed) return { action: 'absent' }
  if (st.state === 'STOPPED') return { action: 'already-stopped' }
  const r =
    st.winsw ?
      await winswRun(spec, ['stopwait'], { timeout: LONG_TIMEOUT_MS })
    : await scRun(['stop', spec.winServiceName])
  if (!(await waitFor(spec, (s) => s.state === 'STOPPED', 30000)))
    throw new ProcessError(
      'stop of ' + spec.winServiceName + ' failed: ' + errText(r)
    )
  return { action: 'stopped' }
}

async function restart(spec) {
  const st = await status(spec)
  if (st.winsw) {
    const r = await winswRun(spec, ['restart'], { timeout: LONG_TIMEOUT_MS })
    if (r.code !== 0)
      throw new ProcessError('WinSW restart failed: ' + errText(r))
    return { action: 'restarted' }
  }
  await stop(spec)
  await start(spec)
  return { action: 'restarted' }
}

// Sets a service to demand-start (used when disabling an instance so its exited
// driver is not started again at boot).
async function setManual(spec) {
  const st = await status(spec)
  if (!st.installed) return
  await setStartMode(spec, 'manual')
  if (st.winsw)
    try {
      const f = xmlPath(spec)
      const x = fs.readFileSync(f, 'utf8')
      const y = x.replace(
        /<startmode>[^<]*<\/startmode>/i,
        '<startmode>Manual</startmode>'
      )
      if (y !== x) fs.writeFileSync(f, y, 'utf8')
    } catch {
      /* the XML start mode is only informative after install */
    }
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
  buildXml,
  quoteArg,
  available: true,
}
