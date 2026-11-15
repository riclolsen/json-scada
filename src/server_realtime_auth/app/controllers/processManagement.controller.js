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

// HTTP endpoints for protocol driver service management: start / stop / restart a
// single instance, bulk status for the AdminUI table, sync/reconcile, and the global
// settings toggles, and the MCP server service. All routes are admin-only (wired in auth.routes.js).

'use strict'

const Log = require('../../simple-logger')
const db = require('../models')
const UserActionsQueue = require('../../userActionsQueue')
const { checkToken } = require('../middlewares/authJwt')
const ProcessManager = require('../services/process-manager')
const RestartScheduler = require('../services/restart-scheduler')
const SystemSettings = require('../services/system-settings')
const McpSettings = require('../services/mcp-settings')

const ProtocolDriverInstance = db.protocolDriverInstance

function registerUserAction(req, actionName) {
  const body = Object.assign({}, req.body) // secrets removed by the queue
  const ck = req.token || checkToken(req)
  UserActionsQueue.enqueue({
    username: ck ? ck.username : req.body?.username,
    properties: body,
    action: actionName,
    timeTag: new Date(),
  })
}

// Answer text of an error: the messages of the service management (meant for
// the admin) as they are, anything else (database, spawn) only in the log.
function errorText(err) {
  if (err instanceof ProcessManager.ProcessError) return err.message
  if (err?.name === 'CastError') return 'Invalid instance id!'
  return 'Operation failed, see the server log.'
}

// Resolves the full instance document referenced by a request body: by its
// _id, or by driver name and instance number. Only plain values reach the
// query (an object such as { $ne: null } would select some other instance);
// null when the reference is not valid.
async function resolveInstance(req) {
  const id = req.body?._id
  if (id !== undefined && id !== null && id !== '') {
    if (typeof id !== 'string' && typeof id !== 'number') return null
    return ProtocolDriverInstance.findOne({ _id: id }).lean().exec()
  }
  const driver = req.body?.protocolDriver
  const number = Number(req.body?.protocolDriverInstanceNumber)
  if (typeof driver !== 'string' || driver === '' || !Number.isFinite(number))
    return null
  return ProtocolDriverInstance.findOne({
    protocolDriver: driver,
    protocolDriverInstanceNumber: Math.trunc(number),
  })
    .lean()
    .exec()
}

function actionHandler(actionName, opName) {
  return async (req, res) => {
    Log.log(actionName)
    try {
      const inst = await resolveInstance(req)
      if (!inst) {
        res.status(200).send({ error: 'Instance not found' })
        return
      }
      registerUserAction(req, actionName)
      const result = await ProcessManager[opName](inst)
      RestartScheduler.clearPending(
        inst.protocolDriver,
        inst.protocolDriverInstanceNumber
      )
      res.status(200).send({ error: false, result })
    } catch (err) {
      Log.log(actionName + ' error: ' + err.message)
      res.status(200).send({ error: errorText(err) })
    }
  }
}

exports.startProtocolDriverInstance = actionHandler(
  'startProtocolDriverInstance',
  'start'
)
exports.stopProtocolDriverInstance = actionHandler(
  'stopProtocolDriverInstance',
  'stop'
)
exports.restartProtocolDriverInstance = actionHandler(
  'restartProtocolDriverInstance',
  'restart'
)

// Bulk status for the instances table (cached briefly to avoid hammering sc /
// supervisorctl when several browsers poll).
let statusCache = null
let statusCacheTime = 0
let statusRefresh = null // the refresh running now, shared by concurrent polls
let statusGen = 0 // bumped by a sync: an older refresh does not fill the cache
const STATUS_TTL_MS = 3000

async function loadStatuses() {
  const instances = await ProtocolDriverInstance.find({}).lean().exec()
  const statuses = await ProcessManager.listStatuses(instances)
  const out = instances.map((inst, i) => {
    const st = statuses[i]
    return {
      protocolDriver: inst.protocolDriver,
      protocolDriverInstanceNumber: inst.protocolDriverInstanceNumber,
      serviceName: st.serviceName,
      manageable: st.manageable,
      reason: st.reason || '',
      installed: st.installed,
      state: st.state,
      startMode: st.startMode,
      node: st.node,
      restartPending: RestartScheduler.isPending(
        inst.protocolDriver,
        inst.protocolDriverInstanceNumber
      ),
    }
  })
  return {
    enabled: ProcessManager.isEnabled(),
    localNode: ProcessManager.localNodeName(),
    statuses: out,
  }
}

exports.listDriverProcessStatus = async (req, res) => {
  try {
    if (statusCache && Date.now() - statusCacheTime < STATUS_TTL_MS) {
      res.status(200).send(statusCache)
      return
    }
    if (!statusRefresh) {
      const gen = statusGen
      const p = loadStatuses()
        .then((payload) => {
          if (gen === statusGen) {
            statusCache = payload
            statusCacheTime = Date.now()
          }
          return payload
        })
        .finally(() => {
          if (statusRefresh === p) statusRefresh = null
        })
      statusRefresh = p
    }
    res.status(200).send(await statusRefresh)
  } catch (err) {
    Log.log('listDriverProcessStatus error: ' + err.message)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.syncDriverServices = async (req, res) => {
  Log.log('syncDriverServices')
  try {
    registerUserAction(req, 'syncDriverServices')
    const instances = await ProtocolDriverInstance.find({}).lean().exec()
    const result = await ProcessManager.reconcileAll(instances, {
      removeOrphans: req.body?.removeOrphans === true,
    })
    statusCache = null // force refresh
    statusGen++
    statusRefresh = null // later polls do not join a refresh from before the sync
    res.status(200).send({ error: false, result })
  } catch (err) {
    Log.log('syncDriverServices error: ' + err.message)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.getSystemSettings = async (req, res) => {
  try {
    const settings = await SystemSettings.getSettings()
    res.status(200).send({
      error: false,
      settings: {
        autoManageServices: settings.autoManageServices,
        autoRestartOnConnectionChange: settings.autoRestartOnConnectionChange,
      },
      processManagementEnabled: ProcessManager.isEnabled(),
      localNode: ProcessManager.localNodeName(),
    })
  } catch (err) {
    Log.log('getSystemSettings error: ' + err.message)
    res.status(200).send({ error: errorText(err) })
  }
}

// --- MCP server (src/mcp-json-scada-db) HTTP service ---------------------------

// Settings (no password), service state on this host and whether the
// configured address answers here.
exports.getMcpServer = async (req, res) => {
  try {
    const settings = await McpSettings.getSettings()
    const [service, listening, adminUser] = await Promise.all([
      ProcessManager.coreStatus('mcp'),
      McpSettings.probe(settings.ipBind, settings.httpPort),
      settings.adminTools
        ? McpSettings.checkAdminUser(settings.adminUsername)
        : Promise.resolve('notSet'),
    ])
    res.status(200).send({
      error: false,
      settings,
      service,
      listening,
      adminUser,
      processManagementEnabled: ProcessManager.isEnabled(),
    })
  } catch (err) {
    Log.log('getMcpServer error: ' + err.message)
    res.status(200).send({ error: errorText(err) })
  }
}

// Saves the settings; { restart: true } also restarts the service when it runs
// (the MCP server reads its settings only at startup).
exports.updateMcpServer = async (req, res) => {
  try {
    registerUserAction(req, 'updateMcpServer') // the queue masks the password
    const settings = await McpSettings.updateSettings(req.body || {})
    let restarted = false
    if (req.body?.restart === true) {
      const st = await ProcessManager.coreStatus('mcp')
      if (st.manageable && st.state === 'RUNNING') {
        await ProcessManager.coreControl('mcp', 'restart')
        restarted = true
      }
    }
    res.status(200).send({ error: false, settings, restarted })
  } catch (err) {
    Log.log('updateMcpServer error: ' + err.message)
    res.status(200).send({
      error: err instanceof McpSettings.SettingsError ? err.message : errorText(err),
    })
  }
}

// { action: 'start' | 'stop' | 'restart' }
exports.controlMcpServer = async (req, res) => {
  const action = req.body?.action
  Log.log('controlMcpServer ' + String(action).slice(0, 20))
  try {
    if (!['start', 'stop', 'restart'].includes(action)) {
      res.status(200).send({ error: 'Invalid action!' })
      return
    }
    registerUserAction(req, action + 'McpServer')
    const result = await ProcessManager.coreControl('mcp', action)
    res.status(200).send({ error: false, result })
  } catch (err) {
    Log.log('controlMcpServer error: ' + err.message)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.updateSystemSettings = async (req, res) => {
  try {
    registerUserAction(req, 'updateSystemSettings')
    const settings = await SystemSettings.updateSettings(req.body || {})
    res.status(200).send({ error: false, settings })
  } catch (err) {
    Log.log('updateSystemSettings error: ' + err.message)
    res.status(200).send({ error: errorText(err) })
  }
}
