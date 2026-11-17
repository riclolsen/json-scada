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

// Python environment of the CIM-EMS driver (src/cim-ems): checks whether the
// interpreter the services run has the package and the dependencies pinned in
// its pyproject.toml, and runs the component's own install script
// (install.sh / install.bat, the same one the platform builds call) as one
// background job at a time. Nothing from a request reaches the command line.

'use strict'

const fs = require('node:fs')
const path = require('node:path')
const { spawn } = require('node:child_process')
const paths = require('./process-manager/paths')
const { run } = require('./process-manager/exec')
const Log = require('../../simple-logger')

class EnvError extends Error {
  constructor(msg) {
    super(msg)
    this.name = 'EnvError'
  }
}

const CHECK_TTL_MS = 60000
const INSTALL_TIMEOUT_MS = 30 * 60 * 1000
const LOG_LINES = 300

function cimDir() {
  return path.join(paths.srcDir(), 'cim-ems')
}

// null when pypowsybl has a wheel for this host (Linux / Windows on x86_64)
function unsupportedReason() {
  if (process.platform !== 'linux' && process.platform !== 'win32')
    return 'CIM-EMS is not supported on ' + process.platform
  if (process.arch !== 'x64') return 'no pypowsybl package for ' + process.arch
  return null
}

// The interpreter the services run (same order as install.bat on Windows:
// the bundled runtime, else the component .venv; .venv on Linux).
function interpreter() {
  if (process.platform === 'win32') {
    const runtime = path.join(
      paths.installRoot(),
      'platform-windows',
      'python-runtime',
      'python.exe'
    )
    if (fs.existsSync(runtime)) return { path: runtime, exists: true }
    const venv = path.join(cimDir(), '.venv', 'Scripts', 'python.exe')
    return { path: venv, exists: fs.existsSync(venv) }
  }
  const venv = path.join(cimDir(), '.venv', 'bin', 'python')
  return { path: venv, exists: fs.existsSync(venv) }
}

// The package itself and the dependencies of pyproject.toml, with the pinned
// version when there is one: [{ name, required }]
function requiredPackages() {
  const out = []
  let text = ''
  try {
    text = fs.readFileSync(path.join(cimDir(), 'pyproject.toml'), 'utf8')
  } catch (err) {
    return out
  }
  const name = /^\s*name\s*=\s*"([^"]+)"/m.exec(text)
  const version = /^\s*version\s*=\s*"([^"]+)"/m.exec(text)
  if (name) out.push({ name: name[1], required: version ? version[1] : null })
  const deps = /^\s*dependencies\s*=\s*\[([\s\S]*?)\]/m.exec(text)
  if (deps)
    for (const m of deps[1].matchAll(/"([A-Za-z0-9_.-]+)\s*(?:==\s*([^"\s;,]+))?[^"]*"/g))
      out.push({ name: m[1], required: m[2] || null })
  return out
}

const CHECK_SCRIPT = [
  'import json, sys',
  'import importlib.metadata as m',
  'out = {}',
  'for n in json.loads(sys.argv[1]):',
  '    try:',
  '        out[n] = m.version(n)',
  '    except Exception:',
  '        out[n] = None',
  'print(json.dumps({"python": sys.version.split()[0], "packages": out}))',
].join('\n')

let checkCache = null
let checkTime = 0

// { state: 'unsupported' | 'missing' | 'outdated' | 'ok' | 'unknown', reason,
//   interpreter, python, packages: [{ name, required, installed }] }
async function check({ force = false } = {}) {
  if (!force && checkCache && Date.now() - checkTime < CHECK_TTL_MS) return checkCache
  const result = await doCheck()
  checkCache = result
  checkTime = Date.now()
  return result
}

async function doCheck() {
  const reason = unsupportedReason()
  const interp = interpreter()
  const required = requiredPackages()
  const base = {
    interpreter: interp.path,
    python: '',
    packages: required.map((p) => ({ ...p, installed: null })),
  }
  if (reason) return { ...base, state: 'unsupported', reason }
  if (!interp.exists) return { ...base, state: 'missing', reason: 'no Python environment' }
  try {
    const res = await run(
      interp.path,
      ['-c', CHECK_SCRIPT, JSON.stringify(required.map((p) => p.name))],
      { timeout: 20000, cwd: cimDir() }
    )
    if (res.code !== 0)
      return { ...base, state: 'unknown', reason: (res.stderr || res.stdout).trim().slice(-500) }
    const info = JSON.parse(res.stdout.trim().split('\n').pop())
    const packages = required.map((p) => ({ ...p, installed: info.packages[p.name] || null }))
    let state = 'ok'
    if (packages.some((p) => !p.installed)) state = 'missing'
    else if (packages.some((p) => p.required && p.installed !== p.required)) state = 'outdated'
    return { ...base, python: info.python || '', packages, state, reason: '' }
  } catch (err) {
    Log.log('CIM-EMS environment check error: ' + err.message)
    return { ...base, state: 'unknown', reason: err.message }
  }
}

// --- install job -----------------------------------------------------------------

let job = null // { state: 'running' | 'ok' | 'failed', startedAt, endedAt, exitCode, user, lines }

function jobView() {
  if (!job) return { state: 'idle', log: '' }
  return {
    state: job.state,
    startedAt: job.startedAt,
    endedAt: job.endedAt,
    exitCode: job.exitCode,
    user: job.user,
    log: job.lines.join('\n'),
  }
}

function isInstalling() {
  return job?.state === 'running'
}

function installCommand() {
  if (process.platform === 'win32')
    return {
      cmd: process.env.ComSpec || 'cmd.exe',
      args: ['/d', '/c', path.join(cimDir(), 'install.bat')],
    }
  return { cmd: '/bin/bash', args: [path.join(cimDir(), 'install.sh')] }
}

// Starts the install script; the output goes to the job log and to
// log/cim_ems_install.log. Throws EnvError when it cannot start.
function startInstall(user) {
  if (isInstalling()) throw new EnvError('An installation is already running.')
  const reason = unsupportedReason()
  if (reason) throw new EnvError(reason)
  const { cmd, args } = installCommand()
  if (!fs.existsSync(args[args.length - 1]))
    throw new EnvError('Install script not found: ' + args[args.length - 1])

  const current = {
    state: 'running',
    startedAt: new Date(),
    endedAt: null,
    exitCode: null,
    user: user || '',
    lines: [],
  }
  job = current
  let logFile = null
  try {
    logFile = fs.createWriteStream(path.join(paths.logDir(), 'cim_ems_install.log'), {
      flags: 'a',
    })
    logFile.on('error', () => {})
  } catch (err) {
    logFile = null
  }
  let partial = ''
  let timer = null
  const push = (line) => {
    current.lines.push(line)
    if (current.lines.length > LOG_LINES) current.lines.splice(0, current.lines.length - LOG_LINES)
    if (logFile) logFile.write(line + '\n')
  }
  const onData = (d) => {
    const text = partial + d.toString('utf8')
    const parts = text.split(/\r?\n/)
    partial = parts.pop()
    for (const p of parts) push(p)
  }
  const finish = (state, code, note) => {
    if (current.state !== 'running') return
    if (timer) clearTimeout(timer)
    if (partial) push(partial)
    partial = ''
    if (note) push(note)
    current.state = state
    current.exitCode = code
    current.endedAt = new Date()
    push('--- ' + state + ' (exit code ' + code + ') ' + current.endedAt.toISOString())
    if (logFile) logFile.end()
    checkCache = null
    Log.log('CIM-EMS dependency install ' + state + ', exit code ' + code)
  }

  push('--- ' + current.startedAt.toISOString() + ' ' + [cmd, ...args].join(' ') + (user ? ' (' + user + ')' : ''))
  let child
  try {
    child = spawn(cmd, args, {
      shell: false,
      windowsHide: true,
      cwd: cimDir(),
      env: { ...process.env, PYTHONUNBUFFERED: '1', PIP_NO_INPUT: '1' },
    })
  } catch (err) {
    finish('failed', null, err.message)
    throw new EnvError('Could not start the install script: ' + err.message)
  }
  timer = setTimeout(() => {
    push('--- timed out after ' + INSTALL_TIMEOUT_MS / 60000 + ' minutes')
    try {
      child.kill()
    } catch (e) {
      /* ignore */
    }
  }, INSTALL_TIMEOUT_MS)
  child.stdout.on('data', onData)
  child.stderr.on('data', onData)
  child.on('error', (err) => finish('failed', null, err.message))
  child.on('close', (code) => finish(code === 0 ? 'ok' : 'failed', code))
  Log.log('CIM-EMS dependency install started')
  return jobView()
}

module.exports = { EnvError, check, jobView, isInstalling, startInstall, unsupportedReason }
