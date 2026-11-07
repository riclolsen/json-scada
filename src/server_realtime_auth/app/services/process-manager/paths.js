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

// Resolves the JSON-SCADA installation root and the platform tool locations used
// by the process manager. No hardcoded paths: everything derives from the module
// location or from environment overrides so the same code runs on a developer
// checkout and on a c:\json-scada / /home/jsonscada install.

'use strict'

const fs = require('fs')
const path = require('path')

// From <root>/src/server_realtime_auth/app/services/process-manager go up 5 levels.
const derivedRoot = path.resolve(__dirname, '..', '..', '..', '..', '..')

function installRoot() {
  if (process.env.JS_INSTALL_DIR && process.env.JS_INSTALL_DIR.trim() !== '')
    return path.resolve(process.env.JS_INSTALL_DIR.trim())
  return derivedRoot
}

function binDir() {
  return path.join(installRoot(), 'bin')
}

function logDir() {
  return path.join(installRoot(), 'log')
}

function srcDir() {
  return path.join(installRoot(), 'src')
}

function confDir() {
  return path.join(installRoot(), 'conf')
}

function confFile() {
  return process.env.JS_CONFIG_FILE || path.join(confDir(), 'json-scada.json')
}

// Node runtime used to launch node-based driver services (services do not inherit
// the auth server's PATH reliably, so prefer an absolute path when it exists).
function nodePath() {
  if (process.platform === 'win32') {
    const bundled = path.join(
      installRoot(),
      'platform-windows',
      'nodejs-runtime',
      'node.exe'
    )
    if (fs.existsSync(bundled)) return bundled
    return 'node'
  }
  if (fs.existsSync('/usr/bin/node')) return '/usr/bin/node'
  return 'node'
}

// Python interpreter for a python driver: the component virtual environment
// (install.sh / install.bat without a bundled runtime), else on Windows the
// bundled platform-windows\python-runtime (install.bat installs the component
// into it), else python3/python from PATH.
function pythonPath(venvDir) {
  const venv = venvDir ? expandPlaceholders(venvDir) : ''
  if (process.platform === 'win32') {
    const candidates = []
    if (venv) candidates.push(path.join(venv, 'Scripts', 'python.exe'))
    candidates.push(
      path.join(installRoot(), 'platform-windows', 'python-runtime', 'python.exe')
    )
    for (const c of candidates) if (fs.existsSync(c)) return c
    return 'python'
  }
  if (venv && fs.existsSync(path.join(venv, 'bin', 'python')))
    return path.join(venv, 'bin', 'python')
  return 'python3'
}

// nssm.exe resolution order: env override, platform-windows dir, bin dir, PATH.
function nssmPath() {
  if (process.env.JS_NSSM_PATH && process.env.JS_NSSM_PATH.trim() !== '')
    return process.env.JS_NSSM_PATH.trim()
  const candidates = [
    path.join(installRoot(), 'platform-windows', 'nssm.exe'),
    path.join(binDir(), 'nssm.exe'),
  ]
  for (const c of candidates) if (fs.existsSync(c)) return c
  return 'nssm'
}

// Directory where the process manager writes supervisor program .ini files it owns.
function managedSupervisorDir(cfg) {
  const fromCfg = cfg?.processManagement?.managedSupervisorDir
  if (typeof fromCfg === 'string' && fromCfg.trim() !== '')
    return path.resolve(fromCfg.trim())
  return path.join(confDir(), 'supervisor.d')
}

// Expands catalog placeholders ({bin}, {src}, {conf}, {node}, {root}) to real paths.
function expandPlaceholders(str) {
  if (typeof str !== 'string') return str
  return str
    .replace(/\{root\}/g, installRoot())
    .replace(/\{bin\}/g, binDir())
    .replace(/\{src\}/g, srcDir())
    .replace(/\{conf\}/g, confDir())
    .replace(/\{confFile\}/g, confFile())
    .replace(/\{node\}/g, nodePath())
}

module.exports = {
  installRoot,
  binDir,
  logDir,
  srcDir,
  confDir,
  confFile,
  nodePath,
  pythonPath,
  nssmPath,
  managedSupervisorDir,
  expandPlaceholders,
}
