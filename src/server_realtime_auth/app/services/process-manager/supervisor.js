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

// Linux backend: manages driver services as supervisord programs. Config files the
// manager owns live in <root>/conf/supervisor.d/*.ini (added to supervisord's
// [include]); control goes through supervisorctl over the inet_http_server already
// provisioned in the shipped supervisord.conf.

'use strict'

const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const paths = require('./paths')
const { run, ProcessError } = require('./exec')

let cfg = null
function configure(configObj) {
  cfg = configObj
}

function pm() {
  return (cfg && cfg.processManagement) || {}
}

// Default to the inet_http_server provided by the shipped supervisord.conf
// (127.0.0.1:9000, admin/jsonscada); avoids unix-socket permission issues for
// the jsonscada user. Override via config or JS_SUPERVISOR_* env vars.
//
// The credentials go to supervisorctl in a private config file
// ([supervisorctl] section, -c), not as -u/-p arguments: a command line is
// visible to every local user (ps, /proc/<pid>/cmdline).
let ctlConfFile = null
let ctlConfContent = null
function ctlBaseArgs() {
  const url =
    pm().supervisorUrl ||
    process.env.JS_SUPERVISOR_URL ||
    'http://127.0.0.1:9000'
  const user = pm().supervisorUser || process.env.JS_SUPERVISOR_USER || 'admin'
  const pass =
    pm().supervisorPassword || process.env.JS_SUPERVISOR_PASSWORD || 'jsonscada'
  // supervisor reads the file with configparser: no line breaks in values
  const clean = (v) => String(v).replace(/[\r\n]/g, '')
  const content =
    '[supervisorctl]\n' +
    'serverurl=' + clean(url) + '\n' +
    'username=' + clean(user) + '\n' +
    'password=' + clean(pass) + '\n'
  if (content !== ctlConfContent || !ctlConfFile || !fs.existsSync(ctlConfFile)) {
    if (!ctlConfFile || !fs.existsSync(path.dirname(ctlConfFile))) {
      const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'json-scada-ctl-'))
      fs.chmodSync(dir, 0o700)
      ctlConfFile = path.join(dir, 'supervisorctl.conf')
    }
    fs.writeFileSync(ctlConfFile, content, { mode: 0o600 })
    fs.chmodSync(ctlConfFile, 0o600)
    ctlConfContent = content
  }
  return ['-c', ctlConfFile]
}
// the file holds a password: removed when the server ends
process.once('exit', () => {
  if (!ctlConfFile) return
  try {
    fs.rmSync(path.dirname(ctlConfFile), { recursive: true, force: true })
  } catch (e) {
    /* ignore */
  }
})

async function ctl(args) {
  return run('supervisorctl', [...ctlBaseArgs(), ...args], { utf16: false })
}

function managedDir() {
  return paths.managedSupervisorDir(cfg)
}

function iniPath(spec) {
  return path.join(managedDir(), spec.programName + '.ini')
}

function isManaged(spec) {
  return fs.existsSync(iniPath(spec))
}

// Quotes a supervisor command token if it contains whitespace (supervisor uses shlex).
function quoteArg(a) {
  return /\s/.test(a) ? '"' + a + '"' : a
}

function renderIni(spec, startMode) {
  const autostart = startMode === 'auto' ? 'true' : 'false'
  const cmdLine = [spec.cmd, ...spec.args].map(quoteArg).join(' ')
  const envLine =
    spec.env && Object.keys(spec.env).length
      ? 'environment=' +
        Object.entries(spec.env)
          .map(([k, v]) => `${k}="${v}"`)
          .join(',') +
        '\n'
      : ''
  return (
    `; managed by json-scada process manager - do not edit by hand\n` +
    `; js-spec-hash: ${spec.specHash}\n` +
    `[program:${spec.programName}]\n` +
    `command=${cmdLine}\n` +
    `directory=${spec.cwd}\n` +
    `autostart=${autostart}\n` +
    `autorestart=true\n` +
    `startretries=1000\n` +
    envLine +
    `redirect_stderr=true\n` +
    `stdout_logfile=${spec.logFile}\n` +
    `stdout_logfile_maxbytes=10MB\n` +
    `stdout_logfile_backups=10\n`
  )
}

// Parses `supervisorctl status <name>` into our state vocabulary.
function parseStatus(res, programName) {
  const text = (res.stdout || '') + (res.stderr || '')
  if (/no such process/i.test(text) || /ERROR \(no such process\)/i.test(text))
    return { installed: false, state: 'UNKNOWN', pid: null }
  const line =
    text.split('\n').find((l) => l.trim().startsWith(programName)) || ''
  const m = line.match(/^\S+\s+(\w+)/)
  const raw = m ? m[1].toUpperCase() : ''
  const map = {
    RUNNING: 'RUNNING',
    STOPPED: 'STOPPED',
    STARTING: 'STARTING',
    STOPPING: 'STOPPING',
    BACKOFF: 'BACKOFF',
    FATAL: 'FATAL',
    EXITED: 'STOPPED',
    UNKNOWN: 'UNKNOWN',
  }
  if (!raw) return { installed: false, state: 'UNKNOWN', pid: null }
  const pidm = line.match(/pid\s+(\d+)/)
  return {
    installed: true,
    state: map[raw] || 'UNKNOWN',
    pid: pidm ? parseInt(pidm[1]) : null,
  }
}

async function status(spec) {
  const res = await ctl(['status', spec.programName])
  const st = parseStatus(res, spec.programName)
  let startMode = 'unknown'
  if (st.installed) startMode = isManaged(spec) ? readIniStartMode(spec) : 'unknown'
  return {
    serviceName: spec.programName,
    installed: st.installed,
    state: st.state,
    startMode,
    pid: st.pid,
    managedHere: isManaged(spec),
  }
}

function readIniStartMode(spec) {
  try {
    const txt = fs.readFileSync(iniPath(spec), 'utf8')
    return /autostart\s*=\s*true/i.test(txt) ? 'auto' : 'manual'
  } catch (e) {
    return 'unknown'
  }
}

function readIniHash(spec) {
  try {
    const txt = fs.readFileSync(iniPath(spec), 'utf8')
    const m = txt.match(/js-spec-hash:\s*([0-9a-f]+)/i)
    return m ? m[1] : null
  } catch (e) {
    return null
  }
}

function errText(r) {
  return (r.stderr || r.stdout || '').replace(/\s+/g, ' ').trim() || 'exit ' + r.code
}

async function ensureService(spec, opts = {}) {
  const startMode = opts.startMode || spec.defaultStartMode
  const st = await status(spec)

  // Program exists but is defined by a legacy root-owned file: adopt for control,
  // but refuse to rewrite it (migration script must move it into the managed dir).
  if (st.installed && !st.managedHere)
    throw new ProcessError(
      'program ' +
        spec.programName +
        ' is defined by a legacy supervisor config; run platform-linux/migrate_supervisor_inis.sh to manage it here'
    )

  if (st.installed && readIniHash(spec) === spec.specHash && st.startMode === startMode)
    return { action: 'unchanged', serviceName: spec.programName }

  fs.mkdirSync(managedDir(), { recursive: true })
  fs.writeFileSync(iniPath(spec), renderIni(spec, startMode), 'utf8')

  const rr = await ctl(['reread'])
  if (rr.code !== 0) throw new ProcessError('supervisorctl reread failed: ' + errText(rr))
  const up = await ctl(['update', spec.programName])
  if (up.code !== 0) throw new ProcessError('supervisorctl update failed: ' + errText(up))
  return {
    action: st.installed ? 'updated' : 'created',
    serviceName: spec.programName,
    wasRunning: st.state === 'RUNNING',
  }
}

async function removeService(spec) {
  const st = await status(spec)
  if (!st.installed) {
    // remove a stale managed ini if present
    if (isManaged(spec)) {
      fs.unlinkSync(iniPath(spec))
      await ctl(['reread'])
      await ctl(['update'])
    }
    return { action: 'absent', serviceName: spec.programName }
  }
  if (!st.managedHere)
    throw new ProcessError(
      'program ' +
        spec.programName +
        ' is defined by a legacy supervisor config; remove it manually or run the migration script'
    )
  await ctl(['stop', spec.programName])
  fs.unlinkSync(iniPath(spec))
  const rr = await ctl(['reread'])
  if (rr.code !== 0) throw new ProcessError('supervisorctl reread failed: ' + errText(rr))
  const up = await ctl(['update'])
  if (up.code !== 0) throw new ProcessError('supervisorctl update failed: ' + errText(up))
  return { action: 'removed', serviceName: spec.programName }
}

async function start(spec) {
  const r = await ctl(['start', spec.programName])
  const text = (r.stdout || '') + (r.stderr || '')
  if (r.code !== 0 && !/already started/i.test(text))
    throw new ProcessError('supervisorctl start failed: ' + errText(r))
  return { action: 'started' }
}

async function stop(spec) {
  const r = await ctl(['stop', spec.programName])
  const text = (r.stdout || '') + (r.stderr || '')
  if (r.code !== 0 && !/not running/i.test(text))
    throw new ProcessError('supervisorctl stop failed: ' + errText(r))
  return { action: 'stopped' }
}

async function restart(spec) {
  const r = await ctl(['restart', spec.programName])
  if (r.code !== 0) throw new ProcessError('supervisorctl restart failed: ' + errText(r))
  return { action: 'restarted' }
}

// Sets a managed program to not autostart, then rewrites+updates (used when disabling).
async function setManual(spec) {
  if (!isManaged(spec)) return
  const txt = fs.readFileSync(iniPath(spec), 'utf8').replace(
    /autostart\s*=\s*true/i,
    'autostart=false'
  )
  fs.writeFileSync(iniPath(spec), txt, 'utf8')
  await ctl(['reread'])
  await ctl(['update', spec.programName])
}

// Lists all program names known to supervisor (for orphan detection).
async function listManagedServiceNames() {
  const r = await ctl(['status'])
  const text = (r.stdout || '') + (r.stderr || '')
  return text
    .split('\n')
    .map((l) => l.trim().split(/\s+/)[0])
    .filter((n) => n && !/^error/i.test(n))
}

module.exports = {
  configure,
  status,
  ensureService,
  removeService,
  start,
  stop,
  restart,
  setManual,
  listManagedServiceNames,
  isManaged,
  available: true,
}
