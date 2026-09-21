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

// Whitelist of manageable protocol drivers. This is the ONLY source of executable
// paths and arguments for the process manager: nothing from an HTTP request ever
// reaches a spawn() command line. Keys mirror the service base names already used
// by platform-windows/create_services.bat and the platform-*/*.ini supervisor files
// so existing (instance 1) services are adopted rather than duplicated.
//
// Placeholders resolved by paths.expandPlaceholders / buildServiceSpec:
//   {bin} {src} {conf} {confFile} {node} {root}   → install paths
//   {instance} {logLevel}                          → per-instance sanitized integers

'use strict'

const nodePath = require('node:path')
const paths = require('./paths')

// Normalizes a resolved filesystem path to the platform separators (the catalog
// templates use '/', which we tidy up so nssm/supervisor store clean paths).
function normPath(p) {
  return nodePath.normalize(p)
}

// exe: a native executable under {bin}. node: a node script (cmd becomes {node}).
const CATALOG = {
  'IEC60870-5-104': {
    key: 'iec104client',
    type: 'exe',
    exe: '{bin}/iec104client',
    defaultStartMode: 'auto',
  },
  'IEC60870-5-104_SERVER': {
    key: 'iec104server',
    type: 'exe',
    exe: '{bin}/iec104server',
    defaultStartMode: 'manual',
  },
  'IEC60870-5-101': {
    key: 'iec101client',
    type: 'exe',
    exe: '{bin}/iec101client',
    defaultStartMode: 'manual',
  },
  'IEC60870-5-101_SERVER': {
    key: 'iec101server',
    type: 'exe',
    exe: '{bin}/iec101server',
    defaultStartMode: 'manual',
  },
  DNP3: {
    key: 'dnp3client',
    type: 'exe',
    exe: '{bin}/dnp3-client',
    defaultStartMode: 'manual',
  },
  DNP3_SERVER: {
    key: 'dnp3server',
    type: 'exe',
    exe: '{bin}/dnp3-server',
    defaultStartMode: 'manual',
  },
  IEC61850: {
    key: 'iec61850client',
    type: 'exe',
    exe: '{bin}/iec61850_client',
    defaultStartMode: 'manual',
  },
  IEC61850_SERVER: {
    key: 'iec61850server',
    type: 'exe',
    exe: '{bin}/iec61850_server',
    defaultStartMode: 'manual',
  },
  ICCP: {
    key: 'iccpclient',
    type: 'exe',
    exe: '{bin}/iccp-client',
    defaultStartMode: 'manual',
  },
  ICCP_SERVER: {
    key: 'iccpserver',
    type: 'exe',
    exe: '{bin}/iccp-server',
    defaultStartMode: 'manual',
  },
  'MQTT-SPARKPLUG-B': {
    key: 'mqttsparkplugclient',
    type: 'node',
    script: '{src}/mqtt-sparkplug/index.js',
    defaultStartMode: 'auto',
  },
  'OPC-UA': {
    key: 'opcuaclient',
    type: 'exe',
    exe: '{bin}/opcua-client',
    defaultStartMode: 'auto',
  },
  'OPC-UA_SERVER': {
    key: 'opcuaserver',
    type: 'node',
    script: '{src}/OPC-UA-Server/index.js',
    passConfPath: true,
    defaultStartMode: 'auto',
  },
  'OPC-DA': {
    key: 'opcdaclient',
    type: 'exe',
    exe: '{bin}/OPC-DA-Client',
    platforms: ['win32'],
    defaultStartMode: 'manual',
  },
  'OPC-DA_SERVER': {
    key: 'opcdaserver',
    type: 'exe',
    exe: '{bin}/OPC-DA-Server',
    platforms: ['win32'],
    defaultStartMode: 'manual',
  },
  PLC4X: {
    // the PLC4X driver is shipped as the java plc4j-client, installed as the
    // JSON_SCADA_plc4jclient service / plc4jclient supervisor program. The go
    // plc4x-client is an alternative executable, left disabled in both
    // create_services.bat and platform-*/plc4xclient.ini.
    key: 'plc4jclient',
    type: 'exe',
    exe:
      process.platform === 'win32'
        ? '{bin}/plc4j-client.bat'
        : '{bin}/plc4j-client.sh',
    defaultStartMode: 'manual',
  },
  'TELEGRAF-LISTENER': {
    key: 'telegraf_listener',
    type: 'node',
    script: '{src}/telegraf-listener/index.js',
    defaultStartMode: 'auto',
  },
  'NODE-RED': {
    key: 'nodered_driver',
    type: 'node',
    script: '{src}/node-red-driver/dist/main.js',
    passConfPath: true,
    defaultStartMode: 'manual',
  },
  N8N: {
    key: 'n8nclient',
    type: 'node',
    script: '{src}/n8n-client/index.js',
    passConfPath: true,
    defaultStartMode: 'manual',
  },
  I104M: {
    key: 'i104m',
    type: 'exe',
    exe: '{bin}/i104m',
    defaultStartMode: 'manual',
  },
  ONVIF: {
    key: 'onvif',
    type: 'node',
    script: '{src}/camera-onvif/index.js',
    passConfPath: true,
    defaultStartMode: 'manual',
  },
  MODBUS: {
    key: 'modbusclient',
    type: 'node',
    script: '{src}/modbus/dist/client/main.js',
    passConfPath: true,
    defaultStartMode: 'manual',
  },
  MODBUS_SERVER: {
    key: 'modbusserver',
    type: 'node',
    script: '{src}/modbus/dist/server/main.js',
    passConfPath: true,
    defaultStartMode: 'manual',
  },
}

// Drivers that exist in the AdminUI list but are intentionally not process-managed
// here (registration-based COM server, containerized-only, or no runnable binary in
// the shipped platform scripts). Reported to the UI with a reason.
const NOT_MANAGEABLE = {
  'OPC-DA_SERVER': 'COM registration based server (not a managed service)',
  PI_DATA_ARCHIVE_INJECTOR: 'not available as a managed service',
  PI_DATA_ARCHIVE_CLIENT: 'not available as a managed service',
}

function currentPlatformKey() {
  return process.platform === 'win32' ? 'win32' : 'linux'
}

function isKnownDriver(driverName) {
  return driverName in CATALOG || driverName in NOT_MANAGEABLE
}

function getCatalogEntry(driverName) {
  return CATALOG[driverName] || null
}

// Resolves a catalog entry, honoring a per-instance executable variant if the
// entry declares one. No entry currently does, so a stored
// processExecutableVariant is ignored and the entry's own executable is used.
function resolveEntry(driverName, variant) {
  const entry = CATALOG[driverName]
  if (!entry) return null
  if (entry.variants && variant && entry.variants[variant]) {
    const v = entry.variants[variant]
    const platformSpec = v[currentPlatformKey()] || v
    return { ...entry, ...platformSpec }
  }
  return entry
}

// Returns { manageable, reason } for a driver on the current platform.
function manageability(driverName) {
  if (driverName in NOT_MANAGEABLE)
    return { manageable: false, reason: NOT_MANAGEABLE[driverName] }
  const entry = CATALOG[driverName]
  if (!entry)
    return { manageable: false, reason: 'unknown protocol driver: ' + driverName }
  if (entry.platforms && !entry.platforms.includes(process.platform))
    return {
      manageable: false,
      reason: driverName + ' is not supported on ' + process.platform,
    }
  return { manageable: true, reason: '' }
}

// Base service/program name for a (driver, instance). Instance 1 keeps the legacy
// unsuffixed name; instances > 1 get a _<N> suffix.
function serviceBaseName(driverName, instanceNumber) {
  const entry = CATALOG[driverName]
  if (!entry) return null
  const n = Math.trunc(Number(instanceNumber))
  return n <= 1 ? entry.key : entry.key + '_' + n
}

// Windows service name (JSON_SCADA_ prefixed).
function winServiceName(driverName, instanceNumber) {
  const base = serviceBaseName(driverName, instanceNumber)
  return base ? 'JSON_SCADA_' + base : null
}

// Legacy Windows service name for instance 1 lookups (identical to winServiceName
// for instance 1; kept explicit for the adoption lookup path).
function legacyWinServiceName(driverName) {
  const entry = CATALOG[driverName]
  return entry ? 'JSON_SCADA_' + entry.key : null
}

// Builds the concrete launch spec for an instance document. Returns null when the
// driver is not manageable. All returned strings are fully expanded absolute paths.
function buildServiceSpec(instanceDoc) {
  const driverName = instanceDoc.protocolDriver
  const m = manageability(driverName)
  if (!m.manageable) return null

  const instanceNumber = Math.max(
    1,
    Math.trunc(Number(instanceDoc.protocolDriverInstanceNumber) || 1)
  )
  const logLevel = Math.max(
    0,
    Math.trunc(Number(instanceDoc.logLevel) || 0)
  )
  const variant = instanceDoc.processExecutableVariant || ''
  const entry = resolveEntry(driverName, variant)

  let cmd
  const args = []
  if (entry.type === 'node') {
    cmd = paths.nodePath()
    args.push(normPath(paths.expandPlaceholders(entry.script)))
  } else {
    cmd = paths.expandPlaceholders(entry.exe)
    if (process.platform === 'win32' && !cmd.toLowerCase().endsWith('.exe'))
      if (!cmd.toLowerCase().endsWith('.bat')) cmd = cmd + '.exe'
    cmd = normPath(cmd)
  }

  args.push(String(instanceNumber))
  args.push(String(logLevel))
  if (entry.passConfPath) args.push(normPath(paths.confFile()))

  const env = {}
  if (process.platform !== 'win32' && entry.linuxEnv)
    for (const [k, v] of Object.entries(entry.linuxEnv))
      env[k] = paths.expandPlaceholders(v)

  const winName = winServiceName(driverName, instanceNumber)
  const baseName = serviceBaseName(driverName, instanceNumber)

  return {
    driverName,
    key: entry.key,
    instanceNumber,
    logLevel,
    serviceName: process.platform === 'win32' ? winName : baseName,
    programName: baseName, // supervisor program name
    winServiceName: winName,
    legacyWinServiceName: legacyWinServiceName(driverName),
    cmd,
    args,
    env,
    cwd: paths.binDir(),
    logFile: require('path').join(paths.logDir(), baseName + '.log'),
    defaultStartMode: entry.defaultStartMode || 'manual',
  }
}

function listDriverNames() {
  return Object.keys(CATALOG)
}

module.exports = {
  CATALOG,
  NOT_MANAGEABLE,
  isKnownDriver,
  getCatalogEntry,
  manageability,
  serviceBaseName,
  winServiceName,
  legacyWinServiceName,
  buildServiceSpec,
  listDriverNames,
}
