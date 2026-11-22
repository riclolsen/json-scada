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

'use strict'

const os = require('node:os')
const Log = require('../../simple-logger')
const LoadConfig = require('../../load-config')
const config = require('../config/auth.config')
const db = require('../models')
const fs = require('fs')
const path = require('path')
const UserActionsQueue = require('../../userActionsQueue')
const jwt = require('jsonwebtoken')
const bcrypt = require('bcryptjs')
const { spawn } = require('child_process')
const AdmZip = require('adm-zip')
// Filter.escape implements RFC 4515 escaping and DN/RDN implement RFC 4514
// escaping. Both come from ldapts itself, so they match exactly the encoders
// the client uses when it serializes a search request.
const { Client, Filter, DN } = require('ldapts')
const { checkToken } = require('../middlewares/authJwt')
const { clientAddress } = require('../services/client-address')
const { stripSessionToken } = require('../../proxy-utils')
const crypto = require('node:crypto')
const UserRights = require('../services/user-rights')
const ProcessManager = require('../services/process-manager')
const RestartScheduler = require('../services/restart-scheduler')
const SystemSettings = require('../services/system-settings')
const User = db.user
const Role = db.role
const Tag = db.tag
const ProtocolDriverInstance = db.protocolDriverInstance
const ProtocolConnection = db.protocolConnection
const UserAction = db.userAction
const BCRYPT_COST = 10
// shortest password accepted when one is set (stored passwords are not checked)
const MIN_PASSWORD_LENGTH = 8
const PASSWORD_TOO_SHORT =
  'Password too short (minimum ' + MIN_PASSWORD_LENGTH + ' characters)!'
function passwordTooShort(pw) {
  return typeof pw !== 'string' || pw.length < MIN_PASSWORD_LENGTH
}

/**
 * Generates a CLSID-style UUID (version 4)
 * Format: {xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx}
 */
const generateClsid = () => {
  const generateHex = (length) => {
    let result = ''
    for (let i = 0; i < length; i++) {
      result += Math.floor(Math.random() * 16).toString(16)
    }
    return result
  }

  return `{${generateHex(8)}-${generateHex(4)}-4${generateHex(3)}-${(Math.floor(Math.random() * 4) + 8).toString(16)}${generateHex(3)}-${generateHex(12)}}`
}

// Update documents built from request bodies. Mongoose treats top-level keys
// beginning with '$' as update operators, so a body like
// { _id, $set: { roles: [...] } } or { _id, $inc: { tokenVersion: -1 } } would
// slip past every check made on the plain fields. Bodies are reduced to plain
// fields (an allowlist where one exists) and ids must be scalars, so a filter
// like { _id: { $ne: null } } cannot select other documents.
function withoutOperators(body) {
  const out = {}
  for (const [k, v] of Object.entries(body || {}))
    if (!k.startsWith('$') && !k.includes('.')) out[k] = v
  return out
}
function pickFields(body, allowed) {
  const out = {}
  for (const k of allowed)
    if (Object.prototype.hasOwnProperty.call(body || {}, k)) out[k] = body[k]
  return out
}
function scalarId(v) {
  return typeof v === 'string' || typeof v === 'number' ? v : null
}
const USER_UPDATE_FIELDS = ['username', 'email', 'password', 'isLDAPUser']
const ROLE_UPDATE_FIELDS = [
  'name',
  'isAdmin',
  'changePassword',
  'sendCommands',
  'enterAnnotations',
  'enterNotes',
  'enterManuals',
  'enterLimits',
  'substituteValues',
  'ackEvents',
  'ackAlarms',
  'disableAlarms',
  'cimEngineering',
  'viewSecEvents',
  'editDisplays',
  'group1List',
  'group1CommandList',
  'displayList',
  'maxSessionDays',
]

// Paging and filter of the admin list routes (listTags, listUserActions).
// A page has at most MAX_LIST_PAGE entries (the largest AdminUI option). The
// filter is { field: value | { op: value } } with plain field names and a few
// comparison operators: no $where/$expr/$function (code run by the database)
// nor nested logical operators.
const MAX_LIST_PAGE = 1000
const LIST_FILTER_OPS = new Set(['$eq', '$ne', '$gt', '$gte', '$lt', '$lte', '$in', '$nin', '$regex', '$options', '$exists'])
const MAX_FILTER_REGEX = 200
function listPaging(body) {
  let limit = Math.floor(Number(body?.itemsPerPage))
  if (!Number.isFinite(limit) || limit < 1) limit = 10
  limit = Math.min(limit, MAX_LIST_PAGE)
  let page = Math.floor(Number(body?.page))
  if (!Number.isFinite(page) || page < 1) page = 1
  return { limit, skip: limit * (page - 1) }
}
function isFilterScalar(v) {
  return v === null || ['string', 'number', 'boolean'].includes(typeof v)
}
// a copy of the filter, or null when it is not acceptable
function listFilter(filter) {
  if (filter === undefined || filter === null) return {}
  if (typeof filter !== 'object' || Array.isArray(filter)) return null
  const out = {}
  for (const [k, v] of Object.entries(filter)) {
    if (!/^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z0-9_]+)*$/.test(k)) return null
    if (isFilterScalar(v)) {
      out[k] = v
      continue
    }
    if (typeof v !== 'object' || Array.isArray(v)) return null
    const cond = {}
    for (const [op, x] of Object.entries(v)) {
      if (!LIST_FILTER_OPS.has(op)) return null
      if (op === '$in' || op === '$nin') {
        if (!Array.isArray(x) || x.length > 10000 || !x.every(isFilterScalar)) return null
      } else if (op === '$regex') {
        if (typeof x !== 'string' || x.length > MAX_FILTER_REGEX) return null
      } else if (op === '$options') {
        if (typeof x !== 'string' || !/^[imsx]*$/.test(x)) return null
      } else if (!isFilterScalar(x)) return null
      cond[op] = x
    }
    out[k] = cond
  }
  return out
}
function sortOf(body, dflt) {
  const order = {}
  if (Array.isArray(body?.sortBy))
    for (const s of body.sortBy.slice(0, 10))
      if (typeof s?.key === 'string' && /^[A-Za-z_][A-Za-z0-9_.]*$/.test(s.key))
        order[s.key] = s?.order === 'desc' ? -1 : 1
  return Object.keys(order).length > 0 ? order : { [dflt]: 1 }
}

// Error text for an admin route answer: what the admin can act on (invalid
// field, duplicate name, missing file), never internals such as stack traces,
// paths of the server or database messages; the full error goes to the log.
function errorText(err) {
  if (err?.code === 11000)
    return 'Duplicate value: ' + Object.keys(err.keyValue || {}).join(', ')
  if (err?.name === 'ValidationError' || err?.name === 'CastError')
    return String(err.message).slice(0, 500)
  if (typeof err?.code === 'string' && /^E[A-Z]+$/.test(err.code))
    return 'File operation failed (' + err.code + ')'
  return 'Operation failed, see the server log.'
}

// add header to identify json-scada user for Grafana auto-login (auth.proxy)
// (node lowercases incoming header names: drop any client-sent value first, or it
// would be forwarded next to ours and could impersonate another user)
exports.addXWebAuthUser = (req) => {
  delete req.headers['x-webauth-user']
  // verifyToken already checked the token
  const ck = req.token || checkToken(req)
  if (ck) req.headers['x-webauth-user'] = ck.username
  stripSessionToken(req)
}

exports.listUserActions = async (req, res) => {
  Log.log('listUserActions')

  const { limit, skip } = listPaging(req.body)
  const filter = listFilter(req.body?.filter)
  if (filter === null) {
    res.status(200).send({ error: 'Invalid filter!' })
    return
  }
  const orderby = sortOf(req.body, 'timeTag')

  try {
    // the page and the total are independent queries
    const [count, userActions] = await Promise.all([
      UserAction.countDocuments(filter),
      UserAction.find(filter).sort(orderby).skip(skip).limit(limit).exec(),
    ])
    res.status(200).send({ userActions: userActions, countTotal: count })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

// Communication status tags (_System.Driver/Conn/Comm.*) are maintained by
// cs_data_processor (see docs/schema.md, Communication status tags). It owns
// their identity and naming: a hand change of those fields is overwritten, a
// deletion recreated, and a hand-made tag with one of their names blocks the
// generator. Other fields (priority, limits, alarm settings...) stay editable.
const commStatusTagPrefixes = ['_System.Driver.', '_System.Conn.', '_System.Comm.']
const commStatusOwnedFields = [
  'tag',
  'type',
  'origin',
  'commStatus',
  'group1',
  'group2',
  'group3',
  'description',
  'ungroupedDescription',
]
const commStatusTagError =
  'Communication status tags are maintained by the data processor: they cannot be created or deleted, and their name, type, origin, groups and descriptions cannot be changed.'
function isCommStatusTagName(name) {
  return (
    typeof name === 'string' &&
    commStatusTagPrefixes.some((p) => name.trim().startsWith(p))
  )
}
async function findCommStatusTag(_id) {
  const doc = await Tag.findOne({ _id: _id })
    .select(commStatusOwnedFields.join(' '))
    .lean()
  return doc && doc.commStatus ? doc : null
}
// true when an update would change a field the generator owns
function changesCommStatusOwnedField(stored, body) {
  return commStatusOwnedFields.some(
    (k) =>
      k in body && JSON.stringify(body[k]) !== JSON.stringify(stored[k] ?? null)
  )
}

// A tag name never begins with a number (or a signed number): a point id that
// is a number or a digit string is read as a point key (_id), anything else as
// a tag name (asPointKey in index.js and graphql-server.js).
const tagNameError =
  'Invalid tag name: it must not be empty nor begin with a number.'
function isValidTagName(name) {
  return (
    typeof name === 'string' && name.trim() !== '' && !/^[+-]?\d/.test(name.trim())
  )
}

exports.createTag = async (req, res) => {
  Log.log('createTag')
  try {
    if (isCommStatusTagName(req.body?.tag) || req.body?.commStatus) {
      res.status(200).send({ error: commStatusTagError })
      return
    }
    if (
      req.body?.tag !== undefined &&
      req.body?.tag !== null &&
      String(req.body.tag).trim() !== '' &&
      !isValidTagName(req.body.tag)
    ) {
      res.status(200).send({ error: tagNameError })
      return
    }
    if (req.body?._id && req.body?._id != 0) {
      req.body._id = Math.trunc(parseFloat(req.body._id))
      const existingTag = await Tag.findOne({ _id: req.body._id })
      if (existingTag) {
        res.status(200).send({ error: 'Tag already exists' })
        return
      }

      if (!req.body.tag || req.body.tag.trim() == '')
        req.body.tag = 'new_tag_' + req.body._id
      req.body.tag = req.body.tag.trim()
      const tag = new Tag(req.body)
      await tag.save()
      req.body = { _id: tag._id }
      registerUserAction(req, 'createTag')
      res.status(200).send(tag)
      return
    }

    // find biggest tag _id
    let biggestTagId = 0
    let resBiggest = await Tag.find({})
      .select('_id')
      .sort({ _id: -1 })
      .limit(1)
      .exec()
    if (resBiggest && resBiggest.length > 0 && '_id' in resBiggest[0])
      biggestTagId = Math.trunc(parseFloat(resBiggest[0]._id))
    if (biggestTagId < 0) biggestTagId = 0
    req.body._id = Math.trunc(parseFloat(biggestTagId + 1))
    if (!req.body.tag || req.body.tag.trim() == '')
      req.body.tag = 'new_tag_' + req.body._id
    req.body.tag = req.body.tag.trim()
    const tag = new Tag(req.body)
    await tag.save()
    req.body = tag
    registerUserAction(req, 'createTag')
    res.status(200).send(tag)
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

// fields of an analog tag that change its out-of-range alarm check
const alarmCheckFields = [
  'hiLimit', 'hihiLimit', 'hihihiLimit', 'loLimit', 'loloLimit', 'lololoLimit',
  'hysteresis', 'alarmDisabled',
]
// a null limit (an Infinity serialized as JSON) is equal to an infinite one
const limitValue = (v) => (v === null || v === undefined || !isFinite(v) ? null : Number(v))
function changesAlarmCheck(stored, body) {
  return alarmCheckFields.some(
    (k) =>
      k in body &&
      (k === 'alarmDisabled'
        ? !!body[k] !== !!stored[k]
        : limitValue(body[k]) !== limitValue(stored[k]))
  )
}
// the data processor recomputes the alarm range of the tag (as for limits
// changed by an operator); merged on the server so a driver write is kept
function requestRangeCheck(_id) {
  Tag.collection
    .updateOne({ _id: _id }, [
      {
        $set: {
          sourceDataUpdate: {
            $mergeObjects: [
              { $ifNull: ['$sourceDataUpdate', {}] },
              { rangeCheck: new Date().getTime() },
            ],
          },
        },
      },
    ])
    .catch((e) => Log.log('Update for range check error: ' + e.message))
}

exports.updateTag = async (req, res) => {
  try {
    registerUserAction(req, 'updateTag')

    req.body = withoutOperators(req.body)
    if ('_id' in req.body && scalarId(req.body._id) !== null) {
      let _id = req.body._id
      delete req.body._id
      const csTag = await findCommStatusTag(_id)
      if (
        csTag
          ? changesCommStatusOwnedField(csTag, req.body)
          : isCommStatusTagName(req.body.tag) || 'commStatus' in req.body
      ) {
        res.status(200).send({ error: commStatusTagError })
        return
      }
      if ('tag' in req.body && !isValidTagName(req.body.tag)) {
        res.status(200).send({ error: tagNameError })
        return
      }
      if ('tag' in req.body) req.body.tag = req.body.tag.trim()

      let IsNumberVal = function (value) {
        if (/^(\-|\+)?([0-9]+(\.[0-9]+)?)$/.test(value)) return true
        return false
      }

      if (IsNumberVal(req.body.protocolSourceCommonAddress))
        req.body.protocolSourceCommonAddress = parseFloat(
          req.body.protocolSourceCommonAddress
        )
      if (IsNumberVal(req.body.protocolSourceObjectAddress))
        req.body.protocolSourceObjectAddress = parseFloat(
          req.body.protocolSourceObjectAddress
        )
      if (IsNumberVal(req.body.protocolSourceASDU))
        req.body.protocolSourceASDU = parseFloat(req.body.protocolSourceASDU)

      const before = await Tag.findOneAndUpdate({ _id: _id }, req.body).lean()
      res.status(200).send({ error: false })
      if (before?.type === 'analog' && changesAlarmCheck(before, req.body))
        requestRangeCheck(before._id)
    } else res.status(200).send({ error: 'No _id in update request.' })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.deleteTag = async (req, res) => {
  try {
    const id = scalarId(req.body?._id)
    if (id === null) {
      res.status(200).send({ error: 'No valid _id in delete request.' })
      return
    }
    registerUserAction(req, 'deleteTag')
    if (await findCommStatusTag(id)) {
      res.status(200).send({ error: commStatusTagError })
      return
    }
    await Tag.findOneAndDelete({ _id: id })
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.listTags = async (req, res) => {
  Log.log('listTags')

  const { limit, skip } = listPaging(req.body)
  const filter = listFilter(req.body?.filter)
  if (filter === null) {
    res.status(200).send({ error: 'Invalid filter!' })
    return
  }
  // the beep and its internal companion point are not listed
  filter['_id'] = { $nin: [-1, -2] }
  const orderby = sortOf(req.body, 'tag')
  try {
    // the page and the total are independent queries
    const [count, tags] = await Promise.all([
      Tag.countDocuments(filter),
      Tag.find(filter).sort(orderby).skip(skip).limit(limit).exec(),
    ])
    let ret = { tags: tags, countTotal: count }
    res.status(200).send(ret)
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.updateProtocolConnection = async (req, res) => {
  registerUserAction(req, 'updateProtocolConnection')
  req.body.protocolDriverInstanceNumber = Math.floor(
    req.body.protocolDriverInstanceNumber
  )
  req.body.protocolConnectionNumber = Math.floor(
    req.body.protocolConnectionNumber
  )

  // make default bind address for some protocols
  if (
    [
      'IEC60870-5-104_SERVER',
      'IEC61850_SERVER',
      'DNP3_SERVER',
      'I104M',
      'TELEGRAF-LISTENER',
      'N8N',
      'OPC-UA_SERVER',
      'ICCP_SERVER',
      'ONVIF',
      'MODBUS_SERVER',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (
      !('ipAddressLocalBind' in req.body) ||
      req.body.ipAddressLocalBind == ''
    ) {
      req.body.ipAddressLocalBind = '0.0.0.0'
      switch (req?.body?.protocolDriver) {
        case 'OPC-UA_SERVER':
          req.body.ipAddressLocalBind = '0.0.0.0:4840'
          break
        case 'IEC60870-5-104_SERVER':
          req.body.ipAddressLocalBind = '0.0.0.0:2404'
          break
        case 'IEC61850_SERVER':
          req.body.ipAddressLocalBind = '0.0.0.0:102'
          break
        case 'DNP3_SERVER':
          req.body.ipAddressLocalBind = '0.0.0.0:20000'
          break
        case 'I104M':
          req.body.ipAddressLocalBind = '0.0.0.0:8099'
          break
        case 'TELEGRAF-LISTENER':
          req.body.ipAddressLocalBind = '0.0.0.0:51920'
          break
        case 'N8N':
          req.body.ipAddressLocalBind = '0.0.0.0:51930'
          break
        case 'ICCP_SERVER':
          req.body.ipAddressLocalBind = '0.0.0.0:102'
          break
        case 'ONVIF':
          req.body.ipAddressLocalBind = '127.0.0.1:9001'
          break
        case 'MODBUS_SERVER':
          req.body.ipAddressLocalBind = '0.0.0.0:502'
      }
    }
  }

  if (
    [
      'IEC60870-5-104',
      'IEC60870-5-104_SERVER',
      'IEC61850',
      'IEC61850_SERVER',
      'DNP3',
      'DNP3_SERVER',
      'PLCTAG',
      'I104M',
      'TELEGRAF-LISTENER',
      'N8N',
      'OPC-UA_SERVER',
      'ICCP',
      'ICCP_SERVER',
      'MODBUS',
      'MODBUS_SERVER',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('ipAddresses' in req.body)) {
      req.body.ipAddresses = []
    }
  }

  if (
    [
      'OPC-UA',
      'TELEGRAF-LISTENER',
      'N8N',
      'MQTT-SPARKPLUG-B',
      'IEC61850',
      'PLC4X',
      'OPC-DA',
      'ICCP',
      'DNP3_SERVER',
      'DNP3',
      'ICCP',
      'IEC60870-5-104_SERVER',
      'IEC60870-5-104',
      'IEC60870-5-101_SERVER',
      'IEC60870-5-101',
      'MODBUS',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('autoCreateTags' in req.body)) {
      req.body.autoCreateTags = true
    }
  }

  if (
    [
      'MQTT-SPARKPLUG-B',
      'N8N',
      'OPC-UA_SERVER',
      'IEC61850',
      'PLC4X',
      'OPC-UA',
      'OPC-DA',
      'OPC-DA_SERVER',
      'ICCP',
      'ICCP_SERVER',
      'DNP3_SERVER',
      'DNP3',
      'IEC60870-5-104_SERVER',
      'IEC60870-5-104',
      'IEC60870-5-101_SERVER',
      'IEC60870-5-101',
      'MODBUS',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('topics' in req.body)) {
      req.body.topics = []
    }
  }

  if (
    ['MQTT-SPARKPLUG-B', 'OPC-UA_SERVER'].includes(req?.body?.protocolDriver)
  ) {
    if (!('groupId' in req.body)) {
      req.body.groupId = ''
    }
  }

  if (
    [
      'MQTT-SPARKPLUG-B',
      'OPC-UA',
      'IEC60870-5-104_SERVER',
      'IEC60870-5-104',
      'IEC61850',
      'IEC61850_SERVER',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('passphrase' in req.body)) {
      req.body.passphrase = ''
    }
  }

  if (
    ['MQTT-SPARKPLUG-B', 'OPC-DA', 'OPC-UA', 'ONVIF'].includes(
      req?.body?.protocolDriver
    )
  ) {
    if (!('username' in req.body)) {
      req.body.username = ''
    }
  }
  if (
    ['MQTT-SPARKPLUG-B', 'OPC-DA', 'OPC-UA', 'ONVIF', 'ICCP', 'ICCP_SERVER'].includes(
      req?.body?.protocolDriver
    )
  ) {
    if (!('password' in req.body)) {
      req.body.password = ''
    }
  }


  if (['MQTT-SPARKPLUG-B'].includes(req?.body?.protocolDriver)) {
    if (!('topicsAsFiles' in req.body)) {
      req.body.topicsAsFiles = []
    }
    if (!('topicsScripted' in req.body)) {
      req.body.topicsScripted = []
    }
    if (!('clientId' in req.body)) {
      req.body.clientId = ''
    }
    if (!('edgeNodeId' in req.body)) {
      req.body.edgeNodeId = ''
    }
    if (!('deviceId' in req.body)) {
      req.body.deviceId = ''
    }
    if (!('scadaHostId' in req.body)) {
      req.body.scadaHostId = ''
    }
    if (!('publishTopicRoot' in req.body)) {
      req.body.publishTopicRoot = ''
    }
  }

  if (['MQTT-SPARKPLUG-B', 'OPC-UA'].includes(req?.body?.protocolDriver)) {
    if (!('pfxFilePath' in req.body)) {
      req.body.pfxFilePath = ''
    }
  }

  if (
    [
      'OPC-UA',
      'MQTT-SPARKPLUG-B',
      'OPC-UA_SERVER',
      'IEC61850',
      'IEC61850_SERVER',
      'OPC-DA',
      'ICCP',
      'ICCP_SERVER',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('useSecurity' in req.body)) {
      req.body.useSecurity = false
    }
  }

  if (
    [
      'OPC-UA',
      'OPC-UA_SERVER',
      'OPC-DA',
      'ICCP',
      'ICCP_SERVER',
      'ONVIF',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('timeoutMs' in req.body)) {
      req.body.timeoutMs = 10000.0
    }
  }

  if (['ICCP', 'ICCP_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('localAeQualifier' in req.body)) {
      req.body.localAeQualifier = 12.0
    }
    if (!('localAppTitle' in req.body)) {
      req.body.localApTitle = '1.1.998.1'
    }
  }

  if (['ICCP', 'ICCP_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('remoteAeQualifier' in req.body)) {
      req.body.remoteAeQualifier = 12.0
    }
    if (!('remoteAppTitle' in req.body)) {
      req.body.remoteApTitle = '1.1.999.2'
    }
  }

  if (['ONVIF'].includes(req?.body?.protocolDriver)) {
    if (!('endpointURLs' in req.body)) {
      req.body.endpointURLs = []
    }
  }

  if (['ONVIF'].includes(req?.body?.protocolDriver)) {
    if (!('options' in req.body)) {
      req.body.options = ''
    }
  }

  if (['OPC-UA'].includes(req?.body?.protocolDriver)) {
    if (!('configFileName' in req.body)) {
      req.body.configFileName = ''
    }
  }
  if (['ICCP'].includes(req?.body?.protocolDriver)) {
    if (!('autoCreateTagPublishingInterval' in req.body)) {
      req.body.autoCreateTagPublishingInterval = 2.5
    }
  }
  if (
    ['OPC-UA', 'OPC-DA', 'OPC-DA_SERVER'].includes(req?.body?.protocolDriver)
  ) {
    if (!('autoCreateTagPublishingInterval' in req.body)) {
      req.body.autoCreateTagPublishingInterval = 2.5
    }
    if (!('autoCreateTagSamplingInterval' in req.body)) {
      req.body.autoCreateTagSamplingInterval = 0.0
    }
    if (!('autoCreateTagQueueSize' in req.body)) {
      req.body.autoCreateTagQueueSize = 0.0
    }
  }
  if (
    ['OPC-UA', 'OPC-DA', 'OPC-DA_SERVER', 'DNP3_SERVER', 'ICCP', 'ICCP_SERVER'].includes(
      req?.body?.protocolDriver
    )
  ) {
    if (!('hoursShift' in req.body)) {
      req.body.hoursShift = 0.0
    }
  }
  if (['OPC-DA', 'OPC-DA_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('deadBand' in req.body)) {
      req.body.deadBand = 0.0
    }
  }
  if (['OPC-DA_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('clsIdApp' in req.body)) {
      req.body.clsIdApp = generateClsid()
    }
    if (!('clsIdServer' in req.body)) {
      req.body.clsIdServer = generateClsid()
    }
    if (!('prgIdServer' in req.body)) {
      req.body.prgIdServer = 'JsonScada.OpcDaServer'
    }
    if (!('prgIdCurrServer' in req.body)) {
      req.body.prgIdCurrServer =
        req.body.prgIdServer + '.' + req.body.protocolConnectionNumber
    }
  }
  if (
    [
      'IEC60870-5-101',
      'IEC60870-5-101_SERVER',
      'IEC60870-5-104',
      'IEC60870-5-104_SERVER',
      'DNP3',
      'DNP3_SERVER',
      'PLCTAG',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('localLinkAddress' in req.body)) {
      req.body.localLinkAddress = 1
    } else {
      req.body.localLinkAddress = Math.floor(req.body.localLinkAddress)
    }
    if (!('remoteLinkAddress' in req.body)) {
      req.body.remoteLinkAddress = 1
    } else {
      req.body.remoteLinkAddress = Math.floor(req.body.remoteLinkAddress)
    }
  }

  if (
    ['IEC60870-5-101', 'IEC60870-5-104'].includes(req?.body?.protocolDriver)
  ) {
    if (!('testCommandInterval' in req.body)) {
      req.body.testCommandInterval = 0
    }
  }

  if (
    [
      'IEC60870-5-101',
      'IEC60870-5-104',
      'IEC60870-5-101_SERVER',
      'IEC60870-5-104_SERVER',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('sizeOfCOT' in req.body)) {
      switch (req?.body?.protocolDriver) {
        case 'IEC60870-5-104':
        case 'IEC60870-5-104_SERVER':
          req.body.sizeOfCOT = 2
          break
        default:
          req.body.sizeOfCOT = 1
          break
      }
    } else {
      req.body.sizeOfCOT = Math.floor(req.body.sizeOfCOT)
    }
    if (!('sizeOfCA' in req.body)) {
      req.body.sizeOfCA = 2
    } else {
      req.body.sizeOfCA = Math.floor(req.body.sizeOfCA)
    }
    if (!('sizeOfIOA' in req.body)) {
      switch (req?.body?.protocolDriver) {
        case 'IEC60870-5-104':
        case 'IEC60870-5-104_SERVER':
          req.body.sizeOfIOA = 3
          break
        default:
          req.body.sizeOfIOA = 2
          break
      }
    } else {
      req.body.sizeOfIOA = Math.floor(req.body.sizeOfIOA)
    }
  }

  if (
    [
      'IEC60870-5-101',
      'IEC60870-5-104',
      'DNP3',
      'PLCTAG',
      'IEC61850',
      'PLC4X',
      'OPC-UA',
      'OPC-DA',
      'ICCP',
      'ONVIF',
      'MODBUS',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('giInterval' in req.body)) {
      req.body.giInterval = 300
      if (req.body.protocolDriver === 'ONVIF') {
        req.body.giInterval = 0
      }
    }
  }

  if (
    [
      'IEC60870-5-101',
      'IEC60870-5-104',
      'DNP3',
      'PLCTAG',
      'IEC61850',
      'PLC4X',
      'OPC-DA',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('timeSyncInterval' in req.body)) {
      req.body.timeSyncInterval = 0
    }
  }

  if (
    ['IEC60870-5-104_SERVER', 'IEC61850_SERVER', 'OPC-UA_SERVER'].includes(
      req?.body?.protocolDriver
    )
  ) {
    if (!('serverModeMultiActive' in req.body)) {
      req.body.serverModeMultiActive = true
    }
    if (!('maxClientConnections' in req.body)) {
      req.body.maxClientConnections = 1
    } else {
      req.body.maxClientConnections = Math.floor(req.body.maxClientConnections)
    }
  }

  if (
    [
      'IEC60870-5-104',
      'IEC60870-5-104_SERVER',
      'IEC61850',
      'IEC61850_SERVER',
      'DNP3',
      'DNP3_SERVER',
      'MQTT-SPARKPLUG-B',
      'OPC-UA_SERVER',
      'OPC-UA',
      'OPC-DA',
      'MODBUS',
      'MODBUS_SERVER',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('localCertFilePath' in req.body)) {
      req.body.localCertFilePath = ''
    }
  }

  if (
    [
      'IEC60870-5-104',
      'IEC60870-5-104_SERVER',
      'IEC61850',
      'IEC61850_SERVER',
      'DNP3',
      'DNP3_SERVER',
      'OPC-DA',
      'MODBUS',
      'MODBUS_SERVER',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('peerCertFilePath' in req.body)) {
      req.body.peerCertFilePath = ''
    }
  }

  if (
    [
      'IEC60870-5-104',
      'IEC60870-5-104_SERVER',
      'MQTT-SPARKPLUG-B',
      'IEC61850',
      'IEC61850_SERVER',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('rootCertFilePath' in req.body)) {
      req.body.rootCertFilePath = ''
    }
    if (!('chainValidation' in req.body)) {
      req.body.chainValidation = false
    }
  }

  if (
    [
      'IEC60870-5-104',
      'IEC60870-5-104_SERVER',
      'MODBUS',
      'MODBUS_SERVER',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('allowOnlySpecificCertificates' in req.body)) {
      req.body.allowOnlySpecificCertificates = false
    }
  }

  if (['OPC-UA'].includes(req?.body?.protocolDriver)) {
    if (!('autoAcceptUntrustedCertificates' in req.body)) {
      req.body.autoAcceptUntrustedCertificates = true
    }
    if (!('securityMode' in req.body)) {
      req.body.securityMode = 'None'
    }
    if (!('securityPolicy' in req.body)) {
      req.body.securityPolicy = 'None'
    }
  }

  if (
    [
      'DNP3',
      'DNP3_SERVER',
      'MQTT-SPARKPLUG-B',
      'OPC-UA_SERVER',
      'IEC61850',
      'IEC61850_SERVER',
      'MODBUS',
      'MODBUS_SERVER',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('privateKeyFilePath' in req.body)) {
      req.body.privateKeyFilePath = ''
    }
  }

  if (
    [
      'DNP3',
      'DNP3_SERVER',
      'MQTT-SPARKPLUG-B',
      'MODBUS',
      'MODBUS_SERVER',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('allowTLSv10' in req.body)) {
      req.body.allowTLSv10 = false
    }
    if (!('allowTLSv11' in req.body)) {
      req.body.allowTLSv11 = false
    }
    if (!('allowTLSv12' in req.body)) {
      req.body.allowTLSv12 = true
    }
    if (!('allowTLSv13' in req.body)) {
      req.body.allowTLSv13 = true
    }
    if (!('cipherList' in req.body)) {
      req.body.cipherList = ''
    }
  }

  if (['OPC-UA', 'OPC-DA', 'DNP3_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('serverQueueSize' in req.body)) {
      req.body.serverQueueSize = 5000.0
    }
  }

  if (['DNP3_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('connectionMode' in req.body)) {
      req.body.connectionMode = 'TCP Passive'
    }
    if (!('enableUnsolicited' in req.body)) {
      req.body.enableUnsolicited = true
    }
  }

  if (['DNP3'].includes(req?.body?.protocolDriver)) {
    if (!('connectionMode' in req.body)) {
      req.body.connectionMode = 'TCP Active'
    }
    if (!('asyncOpenDelay' in req.body)) {
      req.body.asyncOpenDelay = 0.0
    }
    if (!('timeSyncMode' in req.body)) {
      req.body.timeSyncMode = 0.0
    }
    if (!('class0ScanInterval' in req.body)) {
      req.body.class0ScanInterval = 0.0
    }
    if (!('class1ScanInterval' in req.body)) {
      req.body.class1ScanInterval = 0.0
    }
    if (!('class2ScanInterval' in req.body)) {
      req.body.class2ScanInterval = 0.0
    }
    if (!('class3ScanInterval' in req.body)) {
      req.body.class3ScanInterval = 0.0
    }
    if (!('enableUnsolicited' in req.body)) {
      req.body.enableUnsolicited = true
    }
    if (!('rangeScans' in req.body)) {
      req.body.rangeScans = []
    }
  }

  if (
    [
      'IEC60870-5-101_SERVER',
      'IEC60870-5-104_SERVER',
      'IEC61850_SERVER',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('maxQueueSize' in req.body)) {
      req.body.maxQueueSize = 5000
    } else {
      req.body.maxQueueSize = Math.floor(req.body.maxQueueSize)
    }
  }

  if (
    ['IEC60870-5-101', 'IEC60870-5-101_SERVER'].includes(
      req?.body?.protocolDriver
    )
  ) {
    if (!('portName' in req.body)) {
      req.body.portName = ''
    }
    if (!('baudRate ' in req.body)) {
      req.body.baudRate = 9600.0
    } else {
      req.body.baudRate = Math.floor(req.body.baudRate)
    }
    if (!('parity' in req.body)) {
      req.body.parity = 'Even'
    }
    if (!('stopBits' in req.body)) {
      req.body.stopBits = ''
    }
    if (!('handshake' in req.body)) {
      req.body.handshake = 'None'
    }
    if (!('timeoutForACK' in req.body)) {
      req.body.timeoutForACK = 1000.0
    }
    if (!('timeoutRepeat' in req.body)) {
      req.body.timeoutRepeat = 1000.0
    }
    if (!('useSingleCharACK' in req.body)) {
      req.body.useSingleCharACK = true
    }
    if (!('sizeOfLinkAddress' in req.body)) {
      req.body.sizeOfLinkAddress = 1.0
    } else {
      req.body.sizeOfLinkAddress = Math.floor(req.body.sizeOfLinkAddress)
    }
  }

  // MODBUS, MODBUS_SERVER - shared data representation and link parameters
  if (['MODBUS', 'MODBUS_SERVER'].includes(req?.body?.protocolDriver)) {
    // Byte order for non-standard multi-register layouts. Accepts a named alias
    // (BE|LE|SW|SB) or an explicit byte permutation (CDAB, BADC, GHEFCDAB, ...).
    if (!('byteOrder16' in req.body)) {
      req.body.byteOrder16 = 'AB'
    }
    if (!('byteOrder32' in req.body)) {
      req.body.byteOrder32 = 'ABCD'
    }
    if (!('byteOrder64' in req.body)) {
      req.body.byteOrder64 = 'ABCDEFGH'
    }
    if (!('byteOrderStr' in req.body)) {
      req.body.byteOrderStr = 'AB'
    }
    if (!('stringEncoding' in req.body)) {
      req.body.stringEncoding = 'latin1'
    }
    if (!('useModiconAddresses' in req.body)) {
      req.body.useModiconAddresses = false
    }
    // RTU inter-frame idle gap, 0 = auto (3.5 char times at the configured baud)
    if (!('interFrameDelayMs' in req.body)) {
      req.body.interFrameDelayMs = 0.0
    }
    if (!('privateKeyPassphrase' in req.body)) {
      req.body.privateKeyPassphrase = ''
    }
    if (!('chainValidation' in req.body)) {
      req.body.chainValidation = true
    }
    // serial parameters (connectionMode 'Serial')
    if (!('portName' in req.body)) {
      req.body.portName = ''
    }
    if (!('baudRate' in req.body)) {
      req.body.baudRate = 9600.0
    } else {
      req.body.baudRate = Math.floor(req.body.baudRate)
    }
    if (!('parity' in req.body)) {
      req.body.parity = 'Even'
    }
    if (!('stopBits' in req.body)) {
      req.body.stopBits = 'One'
    }
    if (!('handshake' in req.body)) {
      req.body.handshake = 'None'
    }
  }

  // MODBUS (client) specific
  if (['MODBUS'].includes(req?.body?.protocolDriver)) {
    if (!('connectionMode' in req.body)) {
      req.body.connectionMode = 'TCP Active'
    }
    if (!('timeoutMs' in req.body)) {
      req.body.timeoutMs = 1000.0
    }
    if (!('pollingInterval' in req.body)) {
      req.body.pollingInterval = 1000.0
    }
    if (!('maxRetries' in req.body)) {
      req.body.maxRetries = 2.0
    } else {
      req.body.maxRetries = Math.floor(req.body.maxRetries)
    }
    if (!('interRequestDelayMs' in req.body)) {
      req.body.interRequestDelayMs = 0.0
    }
    // clamp request sizes to the protocol maximums
    if (!('maxReadRegisters' in req.body)) {
      req.body.maxReadRegisters = 125.0
    } else {
      req.body.maxReadRegisters = Math.min(
        125,
        Math.max(1, Math.floor(req.body.maxReadRegisters))
      )
    }
    if (!('maxReadCoils' in req.body)) {
      req.body.maxReadCoils = 2000.0
    } else {
      req.body.maxReadCoils = Math.min(
        2000,
        Math.max(1, Math.floor(req.body.maxReadCoils))
      )
    }
    if (!('maxAddressGap' in req.body)) {
      req.body.maxAddressGap = 8.0
    } else {
      req.body.maxAddressGap = Math.max(0, Math.floor(req.body.maxAddressGap))
    }
    // use FC22 Mask Write for single-bit writes, else read-modify-write
    if (!('useMaskWrite' in req.body)) {
      req.body.useMaskWrite = true
    }
  }

  // MODBUS_SERVER specific
  if (['MODBUS_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('connectionMode' in req.body)) {
      req.body.connectionMode = 'TCP Passive'
    }
    if (!('maxClientConnections' in req.body)) {
      req.body.maxClientConnections = 8
    } else {
      req.body.maxClientConnections = Math.floor(req.body.maxClientConnections)
    }
    if (!('clientIdleTimeoutMs' in req.body)) {
      req.body.clientIdleTimeoutMs = 60000.0
    }
    // unit ids (slave addresses) served by this connection
    if (!('serverUnitIds' in req.body) || !Array.isArray(req.body.serverUnitIds)) {
      req.body.serverUnitIds = [1.0]
    } else {
      req.body.serverUnitIds = req.body.serverUnitIds
        .map((v) => Math.floor(Number(v)))
        .filter((v) => Number.isFinite(v) && v >= 0 && v <= 255)
      if (req.body.serverUnitIds.length === 0) {
        req.body.serverUnitIds = [1.0]
      }
    }
    if (!('strictUnitId' in req.body)) {
      req.body.strictUnitId = false
    }
    if (!('serveUnmappedAsZero' in req.body)) {
      req.body.serveUnmappedAsZero = false
    }
    // Modbus has no quality bits: how to serve a tag flagged invalid
    if (!('invalidValuePolicy' in req.body)) {
      req.body.invalidValuePolicy = 'last'
    }
    if (!('allowWritesToSupervised' in req.body)) {
      req.body.allowWritesToSupervised = false
    }
  }

  // CIM-EMS specific: defaults equal what the driver assumes for a missing field
  if (['CIM-EMS'].includes(req?.body?.protocolDriver)) {
    const num = (k, d, min = 0) => {
      const v = Number(req.body[k])
      req.body[k] = k in req.body && req.body[k] !== '' && Number.isFinite(v) ? Math.max(min, v) : d
    }
    if (!('cimModelId' in req.body)) req.body.cimModelId = ''
    // empty revision fields mean "active model revision" and "latest binding revision"
    for (const k of ['cimModelRevision', 'cimBindingRevision']) {
      const v = Number(req.body[k])
      req.body[k] = req.body[k] === '' || req.body[k] == null || !Number.isFinite(v) || v < 1 ? null : Math.floor(v)
    }
    num('cycleSeconds', 60.0, 1)
    num('resultMaxAgeSeconds', 180.0, 1)
    num('switchHoldSeconds', 300.0)
    num('snapshotMaxSkewSeconds', 10.0)
    req.body.loadFlowParameters = {
      distributedSlack: true,
      balanceType: 'PROPORTIONAL_TO_GENERATION_P_MAX',
      voltageInitMode: 'UNIFORM_VALUES',
      transformerVoltageControlOn: false,
      ...(req.body.loadFlowParameters || {}),
    }
    req.body.publishFilter = {
      busVoltages: true,
      branchFlows: true,
      branchLoading: true,
      minNominalKv: 0.0,
      ...(req.body.publishFilter || {}),
    }
    req.body.publishFilter.minNominalKv = Math.max(0, Number(req.body.publishFilter.minNominalKv) || 0)
    if (!('runStateEstimation' in req.body)) req.body.runStateEstimation = false
    req.body.estimatorParameters = { badDataThreshold: 4.0, ...(req.body.estimatorParameters || {}) }
    req.body.estimatorParameters.badDataThreshold = Math.max(1, Number(req.body.estimatorParameters.badDataThreshold) || 4.0)
    if (!('runContingency' in req.body)) req.body.runContingency = false
    if (!req.body.contingencyListId) req.body.contingencyListId = 'AUTO_N1'
    num('contingencyIntervalSeconds', 300.0)
    num('contingencyMinGapSeconds', 10.0)
    num('contingencyMinKv', 0.0)
    num('contingencyPublishLimit', 200.0)
    num('contingencyThreads', 1.0, 1)
    req.body.contingencyPublishLimit = Math.floor(req.body.contingencyPublishLimit)
    req.body.contingencyThreads = Math.floor(req.body.contingencyThreads)
    if (!('autoCreateTags' in req.body)) req.body.autoCreateTags = true
    req.body.commandsEnabled = false // the driver sends no commands
  }

  try {
    // findOneAndUpdate returns the pre-update document (used to detect instance moves)
    req.body = withoutOperators(req.body)
    if (scalarId(req.body._id) === null)
      return res.status(200).send({ error: 'Invalid connection id!' })
    const oldConn = await ProtocolConnection.findOneAndUpdate(
      { _id: req.body._id },
      req.body
    )
      .lean()
      .exec()
    const restartInfo = await scheduleConnectionRestart(
      req.body.protocolDriver,
      req.body.protocolDriverInstanceNumber,
      oldConn
    )
    res.status(200).send({ ...restartInfo })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

// Schedules a debounced driver restart after a connection change, honoring the
// global auto-restart setting. Returns flags the UI uses to inform the operator.
async function scheduleConnectionRestart(driver, instanceNumber, oldConn) {
  try {
    const settings = await SystemSettings.getSettings()
    const auto = settings.autoRestartOnConnectionChange
    const result = await RestartScheduler.scheduleRestart(
      driver,
      instanceNumber,
      auto
    )
    // connection moved between instances: also restart the previous owner
    if (
      oldConn &&
      (oldConn.protocolDriver !== driver ||
        Math.trunc(Number(oldConn.protocolDriverInstanceNumber)) !==
          Math.trunc(Number(instanceNumber)))
    ) {
      await RestartScheduler.scheduleRestart(
        oldConn.protocolDriver,
        oldConn.protocolDriverInstanceNumber,
        auto
      )
    }
    return {
      restartScheduled: !!result.scheduled,
      restartPending: !!result.pending,
    }
  } catch (e) {
    Log.log('scheduleConnectionRestart: ' + e.message)
    return {}
  }
}

exports.deleteProtocolConnection = async (req, res) => {
  try {
    const id = scalarId(req.body?._id)
    if (id === null)
      return res.status(200).send({ error: 'Invalid connection id!' })
    registerUserAction(req, 'deleteProtocolConnection')

    // the connection first: its tags are those of the number stored in it
    // (never a number taken from the request), and nothing is deleted when
    // the connection does not exist
    const conn = await ProtocolConnection.findOneAndDelete({ _id: id })
      .lean()
      .exec()
    if (!conn) return res.status(200).send({ error: 'Connection not found!' })

    if (req.body?.deleteTags === true) {
      const number = Number(conn.protocolConnectionNumber)
      if (Number.isFinite(number)) {
        try {
          await Tag.deleteMany({ protocolSourceConnectionNumber: number })
        } catch (err) {
          // the connection is gone: say so, the tags can be deleted again
          Log.log(err)
          return res.status(200).send({
            error:
              'Connection deleted, but its tags could not be deleted: ' +
              errorText(err),
          })
        }
      }
    }

    const restartInfo = await scheduleConnectionRestart(
      conn.protocolDriver,
      conn.protocolDriverInstanceNumber,
      null
    )

    res.status(200).send({ error: false, ...restartInfo })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.createProtocolConnection = async (req, res) => {
  try {
    // find the biggest connection number and increment for the new connection
    let protocolConnections = await ProtocolConnection.find({}).exec()
    let connNumber = 0
    protocolConnections.forEach((element) => {
      if (element.protocolConnectionNumber > connNumber)
        connNumber = element.protocolConnectionNumber
    })
    const protocolConnection = new ProtocolConnection()
    protocolConnection.protocolConnectionNumber = connNumber + 1
    protocolConnection.DriverInstanceNumber = 1
    await protocolConnection.save()
    req.body = { _id: protocolConnection._id }
    registerUserAction(req, 'createProtocolConnection')
    res.status(200).send({ _id: protocolConnection._id, error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.getProtocolConnectionModel = async (req, res) => {
  try {
    // find the biggest connection number and increment for the new connection
    let protocolConnections = await ProtocolConnection.find({}).exec()
    let connNumber = 0
    protocolConnections.forEach((element) => {
      if (element.protocolConnectionNumber > connNumber)
        connNumber = element.protocolConnectionNumber
    })
    const protocolConnection = new ProtocolConnection()
    protocolConnection.protocolConnectionNumber = connNumber + 1
    protocolConnection.DriverInstanceNumber = 1
    res
      .status(200)
      .send({ error: false, protocolConnection: protocolConnection })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.listProtocolConnections = async (req, res) => {
  Log.log('listProtocolConnections')

  try {
    let protocolConnections = await ProtocolConnection.find({}).exec()
    res.status(200).send(protocolConnections)
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.deleteProtocolDriverInstance = async (req, res) => {
  try {
    const id = scalarId(req.body?._id)
    if (id === null)
      return res.status(200).send({ error: 'Invalid driver instance id!' })
    registerUserAction(req, 'deleteProtocolDriverInstance')

    // the deleted document itself (not a second read) drives the service hook
    const doc = await ProtocolDriverInstance.findOneAndDelete({ _id: id })
      .lean()
      .exec()

    // best-effort: stop and uninstall the OS service for the removed instance
    let processWarning = undefined
    try {
      const settings = await SystemSettings.getSettings()
      if (settings.autoManageServices && doc) {
        const r = await ProcessManager.removeService(doc)
        if (r && r.error) processWarning = r.error
      }
    } catch (e) {
      Log.log('deleteProtocolDriverInstance service hook: ' + e.message)
      processWarning = e.message
    }
    res.status(200).send({ error: false, processWarning })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.listNodes = async (req, res) => {
  const cfg = LoadConfig()
  Log.log('listNodes')
  try {
    let driverInstances = await ProtocolDriverInstance.find({}).exec()
    let listNodes = []
    if (cfg.nodeName) listNodes.push(cfg.nodeName)
    driverInstances.map((element) => {
      listNodes = listNodes.concat(element.nodeNames)
    })
    res.status(200).send([...new Set(listNodes)])
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.createProtocolDriverInstance = async (req, res) => {
  try {
    const driverInstance = new ProtocolDriverInstance()
    await driverInstance.save()
    req.body = { _id: driverInstance._id }
    registerUserAction(req, 'createProtocolDriverInstance')
    res.status(200).send({ _id: driverInstance._id, error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.listProtocolDriverInstances = async (req, res) => {
  Log.log('listProtocolDriverInstances')

  try {
    let driverInstances = await ProtocolDriverInstance.find({}).exec()
    res.status(200).send(driverInstances)
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.updateProtocolDriverInstance = async (req, res) => {
  try {
    req.body = withoutOperators(req.body)
    if (scalarId(req.body._id) === null)
      return res.status(200).send({ error: 'Invalid driver instance id!' })
    req.body.protocolDriverInstanceNumber = Math.floor(
      req.body.protocolDriverInstanceNumber
    )
    req.body.logLevel = Math.floor(req.body.logLevel)
    const oldDoc = await ProtocolDriverInstance.findOne({
      _id: req.body._id,
    })
      .lean()
      .exec()
    await ProtocolDriverInstance.findOneAndUpdate(
      { _id: req.body._id },
      req.body
    )
    registerUserAction(req, 'updateProtocolDriverInstance')

    // best-effort: reconcile the OS service to the updated instance definition
    let processWarning = undefined
    try {
      const settings = await SystemSettings.getSettings()
      if (settings.autoManageServices) {
        const newDoc = await ProtocolDriverInstance.findOne({
          _id: req.body._id,
        })
          .lean()
          .exec()
        const r = await ProcessManager.applyInstanceUpdate(oldDoc, newDoc)
        if (r && r.error) processWarning = r.error
      }
    } catch (e) {
      Log.log('updateProtocolDriverInstance service hook: ' + e.message)
      processWarning = e.message
    }
    res.status(200).send({ error: false, processWarning })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.listUsers = async (req, res) => {
  Log.log('listUsers')
  try {
    let users = await User.find({}).populate('roles').exec()
    users.forEach((user) => {
      user.password = null
    })
    res.status(200).send(users)
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.listRoles = async (req, res) => {
  Log.log('listRoles')

  try {
    let roles = await Role.find({}).exec()
    res.status(200).send(roles)
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

// user and role are given by name: plain strings only (an object such as
// { $ne: null } would select some other document)
function namesOf(body) {
  return typeof body?.username === 'string' && typeof body?.role === 'string'
}

exports.userAddRole = async (req, res) => {
  try {
    if (!namesOf(req.body)) {
      res.status(200).send({ error: 'Invalid user or role name!' })
      return
    }
    registerUserAction(req, 'userAddRole')
    let role = await Role.findOne({ name: req.body.role }).exec()
    if (!role) {
      res.status(200).send({ error: 'Role not found!' })
      return
    }
    let user = await User.findOne({ username: req.body.username }).exec()
    if (!user) {
      res.status(200).send({ error: 'User not found!' })
      return
    }
    user.roles.addToSet(role._id)
    await user.save()
    UserRights.invalidate()
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.userRemoveRole = async (req, res) => {
  try {
    if (!namesOf(req.body)) {
      res.status(200).send({ error: 'Invalid user or role name!' })
      return
    }
    if (req.body.username === 'admin' && req.body.role === 'admin') {
      res
        .status(200)
        .send({ error: 'Cannot remove admin role from admin user!' })
      return
    }

    registerUserAction(req, 'userRemoveRole')

    let role = await Role.findOne({ name: req.body.role }).exec()
    if (!role) {
      res.status(200).send({ error: 'Role not found!' })
      return
    }
    let user = await User.findOne({ username: req.body.username }).exec()
    if (!user) {
      res.status(200).send({ error: 'User not found!' })
      return
    }
    if (
      role.isAdmin &&
      (await remainingAdmins({ fromUser: user._id, role: role._id })) === 0
    ) {
      res.status(200).send({ error: LAST_ADMIN_MESSAGE })
      return
    }
    user.roles.pull(role._id)
    await user.save()
    UserRights.invalidate()
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.listGroup1 = async (req, res) => {
  Log.log('listGroup1')

  try {
    let groups = await Tag.find().distinct('group1').exec()
    res.status(200).send(groups)
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

// Displays the signed-in user may open (an empty list means all of them).
exports.userDisplays = (req, res) => {
  res.status(200).send({ displayList: req.rights?.displayList || [] })
}

// The display (svg) folder of the installation, independent of the working
// directory of the process (listDisplays, openDisplay and saveDisplay).
const SVG_DIR = path.resolve(__dirname, '..', '..', '..', '..', 'svg')
// a display file name: an .svg name directly in SVG_DIR
const DISPLAY_NAME_RE = /^[^.][^/\\:]*\.svg$/i
function displayPath(name) {
  if (typeof name !== 'string') return null
  const fileName = path.basename(name)
  const p = path.resolve(SVG_DIR, fileName)
  return DISPLAY_NAME_RE.test(fileName) && path.dirname(p) === SVG_DIR
    ? { fileName, p }
    : null
}

// A display editor that is not an admin works only on the displays of its
// displayList (when the list is not empty), as the /svg viewer route allows.
function displayAllowed(rights, fileName) {
  if (!rights || rights.isAdmin) return true
  const list = Array.isArray(rights.displayList) ? rights.displayList : []
  return list.length === 0 || list.includes(fileName)
}

exports.listDisplays = (req, res) => {
  Log.log('listDisplays')

  fs.readdir(SVG_DIR, function (err, files) {
    if (err) {
      Log.log(err)
      res.status(200).send({ error: errorText(err) })
      return
    }
    res
      .status(200)
      .send(
        files.filter(
          (f) => path.extname(f).toLowerCase() === '.svg' && displayAllowed(req.rights, f)
        )
      )
  })
}

exports.openDisplay = (req, res) => {
  Log.log('openDisplay')

  // one .svg name directly in the svg folder (a repeated parameter arrives
  // as an array, other files of the folder are not displays)
  const target = displayPath(req.query?.file)
  if (!target) {
    Log.log('openDisplay: Invalid file name attempted: ' + String(req.query?.file))
    res.status(400).send({ error: 'Invalid file name (must be a .svg file)' })
    return
  }
  if (!displayAllowed(req.rights, target.fileName)) {
    res.status(403).send({ error: 'Access to display denied!' })
    return
  }

  fs.readFile(target.p, function (err, data) {
    if (err) {
      Log.log(err)
      res.status(200).send({ error: errorText(err) })
      return
    }
    // as before: a binary download, never rendered (and run) as a page
    res.status(200).send(data)
  })
}

exports.saveDisplay = (req, res) => {
  Log.log('saveDisplay')

  // only .svg files directly in the svg folder (no directory traversal)
  const target = displayPath(req.body?.file)
  if (!target) {
    Log.log('saveDisplay: Invalid file name attempted: ' + req.body?.file)
    res.status(400).send({ error: 'Invalid file name (must be a .svg file)' })
    return
  }
  if (typeof req.body.content !== 'string') {
    res.status(400).send({ error: 'Missing display content' })
    return
  }
  if (!displayAllowed(req.rights, target.fileName)) {
    res.status(403).send({ error: 'Access to display denied!' })
    return
  }

  const { fileName, p: requestedPath } = target
  fs.writeFile(requestedPath, req.body.content, function (err) {
    //handling error
    if (err) {
      Log.log(err)
      res.status(200).send({ error: errorText(err) })
      return
    }
    // the content itself is in the file, the audit keeps name and size
    registerUserAction(req, 'saveDisplay', { body: { file: fileName, size: req.body.content.length } })
    res.status(200).send({ error: false })
  })
}

// Users that would still hold a role with isAdmin after a change: without the
// admin flag of role `withoutRole`, without user `withoutUser`, or with user
// `fromUser` losing role `role`. Used to refuse changes locking every admin out.
async function remainingAdmins({ withoutRole, withoutUser, fromUser, role } = {}) {
  const adminRoleIds = (
    await Role.find({ isAdmin: true }).select('_id').read('primary').lean().exec()
  )
    .map((r) => String(r._id))
    .filter((id) => id !== String(withoutRole))
  if (adminRoleIds.length === 0) return 0
  const users = await User.find({ roles: { $in: adminRoleIds } })
    .select('_id roles')
    .read('primary')
    .lean()
    .exec()
  return users.filter((u) => {
    if (withoutUser !== undefined && String(u._id) === String(withoutUser))
      return false
    if (fromUser !== undefined && String(u._id) === String(fromUser))
      return u.roles.some(
        (r) => adminRoleIds.includes(String(r)) && String(r) !== String(role)
      )
    return true
  }).length
}
const LAST_ADMIN_MESSAGE =
  'Refused: no user would be left with administrator rights!'

exports.updateRole = async (req, res) => {
  Log.log('Update role')
  try {
    const id = scalarId(req.body?._id)
    if (id === null) {
      res.status(200).send({ error: 'Invalid role id!' })
      return
    }
    const fields = pickFields(req.body, ROLE_UPDATE_FIELDS)
    if (
      'isAdmin' in fields &&
      fields.isAdmin !== true &&
      (await remainingAdmins({ withoutRole: id })) === 0
    ) {
      res.status(200).send({ error: LAST_ADMIN_MESSAGE })
      return
    }
    registerUserAction(req, 'updateRole')
    await Role.updateOne({ _id: id }, { $set: fields })
    UserRights.invalidate()
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.updateUser = async (req, res) => {
  Log.log('Update user: ' + req?.body?.username)
  try {
    const id = scalarId(req.body?._id)
    if (id === null) {
      res.status(200).send({ error: 'Invalid user id!' })
      return
    }
    // plain allowed fields only: roles change through userAddRole/RemoveRole,
    // the token version only moves forward (and only here, with the password)
    const fields = pickFields(req.body, USER_UPDATE_FIELDS)
    if (fields.password === '' || fields.password === null) delete fields.password
    if ('password' in fields && passwordTooShort(fields.password)) {
      res.status(200).send({ error: PASSWORD_TOO_SHORT })
      return
    }
    let renamed = false
    if ('username' in fields) {
      if (typeof fields.username !== 'string' || fields.username.trim() === '') {
        res.status(200).send({ error: 'Invalid user name!' })
        return
      }
      const current = await User.findById(id).select('username').read('primary').lean().exec()
      if (current?.username === 'admin' && fields.username !== 'admin') {
        res.status(200).send({ error: 'Cannot rename admin user!' })
        return
      }
      if (await usernameTaken(fields.username, id)) {
        res.status(200).send({ error: 'User name already exists!' })
        return
      }
      renamed = !!current && current.username !== fields.username
    }
    registerUserAction(req, 'updateUser')
    const update = { $set: fields }
    if ('password' in fields)
      fields.password = await bcrypt.hash(fields.password, BCRYPT_COST)
    // a password set by an admin, or a new user name (tokens carry the name
    // used for the audit trail and the Grafana trusted header), ends the
    // sessions of that user: in the same write as the change
    if ('password' in fields || renamed) update.$inc = { tokenVersion: 1 }
    await User.updateOne({ _id: id }, update)
    UserRights.invalidate()
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

// Is the user name used by another user? (names identify users at sign-in)
async function usernameTaken(username, exceptId) {
  if (typeof username !== 'string' || username.trim() === '') return false
  const other = await User.findOne({ username: username }).select('_id').lean().exec()
  return !!other && String(other._id) !== String(exceptId)
}

exports.createRole = async (req, res) => {
  Log.log('Create role')
  try {
    const fields = pickFields(req.body, ROLE_UPDATE_FIELDS)
    if (typeof fields.name !== 'string' || fields.name.trim() === '') {
      res.status(200).send({ error: 'Invalid role name!' })
      return
    }
    const role = new Role(fields)
    await role.save()
    req.body._id = role._id
    registerUserAction(req, 'createRole')
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.createUser = async (req, res) => {
  Log.log('Create user')
  try {
    // plain allowed fields only (no id, token version or LDAP identity)
    const fields = pickFields(req.body, [...USER_UPDATE_FIELDS, 'roles'])
    if (typeof fields.username !== 'string' || fields.username.trim() === '') {
      res.status(200).send({ error: 'Invalid user name!' })
      return
    }
    if ('roles' in fields) {
      const ids = Array.isArray(fields.roles) ? fields.roles : null
      const found =
        ids && ids.every((r) => typeof r === 'string' && /^[0-9a-fA-F]{24}$/.test(r))
          ? await Role.countDocuments({ _id: { $in: ids } })
          : -1
      if (found !== new Set(ids || []).size) {
        res.status(200).send({ error: 'Invalid role list!' })
        return
      }
      fields.roles = [...new Set(ids)]
    }
    if (await usernameTaken(fields.username, null)) {
      res.status(200).send({ error: 'User name already exists!' })
      return
    }
    if (fields.password === '' || fields.password === null) delete fields.password
    if ('password' in fields) {
      if (passwordTooShort(fields.password)) {
        res.status(200).send({ error: PASSWORD_TOO_SHORT })
        return
      }
      fields.password = await bcrypt.hash(fields.password, BCRYPT_COST)
    }
    // the unique index refuses a name created meanwhile (code 11000)
    const user = new User(fields)
    try {
      await user.save()
    } catch (err) {
      if (err?.code === 11000 && 'username' in (err.keyValue || {})) {
        res.status(200).send({ error: 'User name already exists!' })
        return
      }
      throw err
    }
    req.body._id = user._id
    registerUserAction(req, 'createUser')
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.deleteRole = async (req, res) => {
  Log.log('Delete role')

  try {
    const id = scalarId(req.body?._id)
    if (id === null) return res.status(200).send({ error: 'Invalid role id!' })
    // do not delete a role that is attributed to a user
    const users = await User.find({ roles: id }).select('_id').lean().exec()
    if (users.length > 0)
      return res
        .status(200)
        .send({ error: 'Cannot delete role that is attributed to a user!' })

    registerUserAction(req, 'deleteRole')

    await Role.findOneAndDelete({ _id: id })
    UserRights.invalidate()
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

exports.deleteUser = async (req, res) => {
  Log.log('Delete user')
  try {
    // the protected user is identified by the stored document, not by the
    // username the request claims
    const id = scalarId(req.body?._id)
    if (id === null) {
      res.status(200).send({ error: 'Invalid user id!' })
      return
    }
    const target = await User.findById(id).read('primary').exec()
    if (req.body.username === 'admin' || target?.username === 'admin') {
      res.status(200).send({ error: 'Cannot delete admin user!' })
      return
    }
    if (target && (await remainingAdmins({ withoutUser: target._id })) === 0) {
      res.status(200).send({ error: LAST_ADMIN_MESSAGE })
      return
    }
    registerUserAction(req, 'deleteUser')

    await User.findOneAndDelete({ _id: id })
    UserRights.invalidate()
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: errorText(err) })
  }
}

// ---------------------------------------------------------------------------
// Sign-in throttling: after SIGNIN_MAX_FAILURES failed attempts of the same
// client address for the same user name, further attempts are refused until
// SIGNIN_LOCK_MS passed. Kept in memory (a restart clears it). The address is
// the one of the connection unless it comes from a trusted proxy (see
// services/client-address.js).
// ---------------------------------------------------------------------------
const SIGNIN_MAX_FAILURES = 5
const SIGNIN_LOCK_MS = 15 * 60 * 1000
const SIGNIN_MAX_TRACKED = 10000 // bound of the failure table
// failures of one client address, whatever the user names (password spraying)
const SIGNIN_MAX_FAILURES_PER_ADDRESS = 30
// password checks running at once (bcrypt and LDAP binds are expensive)
const SIGNIN_MAX_CONCURRENT = 8
// ...and from one client address, so one client cannot take every slot
const SIGNIN_MAX_CONCURRENT_PER_ADDRESS = 2
let signinsInProgress = 0
const signinsByAddress = new Map() // client address -> checks running
const MAX_USERNAME_LOGGED = 64 // characters of a failed user name kept in logs
const signinFailures = new Map() // key -> { count, first, lockedUntil }

function signinKey(req) {
  return (
    clientAddress(req) +
    '|' +
    String(req.body?.username).slice(0, 256).toLowerCase()
  )
}
function signinLocked(key) {
  const e = signinFailures.get(key)
  return !!e && e.lockedUntil > Date.now()
}
function purgeSigninFailures() {
  const now = Date.now()
  for (const [k, e] of signinFailures)
    if (now - e.first > SIGNIN_LOCK_MS && e.lockedUntil < now)
      signinFailures.delete(k)
}
function signinFailed(key, max = SIGNIN_MAX_FAILURES) {
  const now = Date.now()
  let e = signinFailures.get(key)
  if (!e || now - e.first > SIGNIN_LOCK_MS) e = { count: 0, first: now, lockedUntil: 0 }
  e.count++
  if (e.count >= max) e.lockedUntil = now + SIGNIN_LOCK_MS
  if (!signinFailures.has(key) && signinFailures.size >= SIGNIN_MAX_TRACKED) {
    purgeSigninFailures()
    // still full: drop the oldest entries (Map keeps insertion order)
    for (const k of signinFailures.keys()) {
      if (signinFailures.size < SIGNIN_MAX_TRACKED) break
      signinFailures.delete(k)
    }
  }
  signinFailures.set(key, e)
}
setInterval(purgeSigninFailures, 60 * 1000).unref()

const SIGNIN_FAIL_MESSAGE = 'Invalid user name or password.'
// compared against when the user does not exist, so the response time does not tell
const DUMMY_HASH = bcrypt.hashSync('json-scada-dummy-password', BCRYPT_COST)

// sessions last maxSessionDays of the user's roles, at least one hour; LDAP
// users at most ldap.maxSessionHours (default 12): the directory is only
// consulted at sign-in, so a disabled account must not keep a long session
const MIN_SESSION_SECONDS = 3600
const DEFAULT_LDAP_SESSION_HOURS = 12
function sessionSeconds(rights, user) {
  const days = Number(rights?.maxSessionDays)
  let seconds = Math.max(
    MIN_SESSION_SECONDS,
    Math.round((Number.isFinite(days) ? days : 0) * 86400)
  )
  if (user?.isLDAPUser) {
    const h = Number(config.ldap?.maxSessionHours)
    const cap = Math.round(
      (Number.isFinite(h) && h > 0 ? h : DEFAULT_LDAP_SESSION_HOURS) * 3600
    )
    seconds = Math.min(seconds, Math.max(60, cap))
  }
  return seconds
}

// cookies are sent over https when the request came that way (also behind a proxy)
function isSecureRequest(req) {
  return req.secure || req.headers['x-forwarded-proto'] === 'https'
}

// Issues a session for a user with populated roles: the access token (http only
// cookie) and a cookie with plain user data for the client side scripts.
// The token only identifies the user (rights are read from the database on each
// request), which keeps it small also for users with many stations.
const MAX_USER_COOKIE_BYTES = 3500
function issueSession(req, res, user) {
  const { rights, authorities } = UserRights.combineRights(user.roles)
  const seconds = sessionSeconds(rights, user)
  const token = jwt.sign(
    {
      id: user.id,
      username: user.username,
      tv: Number(user.tokenVersion) || 0,
      jti: crypto.randomUUID(),
    },
    config.secret,
    { expiresIn: seconds }
  )
  // the station lists are not used by the client scripts and can be long
  const { group1List, group1CommandList, ...clientRights } = rights
  const userCookie = {
    id: user._id,
    username: user.username,
    email: user.email,
    isLDAPUser: user.isLDAPUser,
    roles: authorities,
    rights: clientRights,
  }
  // browsers drop cookies over ~4 KB: a long display list is left out and the
  // client reads it from auth/userDisplays (displayListTruncated: true)
  if (encodeURIComponent(JSON.stringify(userCookie)).length > MAX_USER_COOKIE_BYTES) {
    const { displayList, ...rest } = clientRights
    userCookie.rights = { ...rest, displayListTruncated: true }
  }
  return res
    .cookie('x-access-token', token, {
      httpOnly: true,
      secure: isSecureRequest(req),
      sameSite: 'lax',
      maxAge: (seconds + 86400) * 1000,
    })
    .cookie(
      'json-scada-user',
      JSON.stringify(userCookie),
      {
        httpOnly: false,
        secure: isSecureRequest(req),
        sameSite: 'lax',
        maxAge: (2 * seconds + 86400) * 1000,
      }
    )
}

// User signin request
exports.signin = async (req, res) => {
  try {
    let user = null

    if (
      typeof req.body?.username !== 'string' ||
      typeof req.body?.password !== 'string' ||
      req.body.username === ''
    ) {
      return res.status(200).send({ ok: false, message: SIGNIN_FAIL_MESSAGE })
    }

    const attemptKey = signinKey(req)
    const addressKey = 'address|' + clientAddress(req)
    if (signinLocked(attemptKey) || signinLocked(addressKey)) {
      Log.log(
        'Sign-in locked for ' +
          req.body.username.slice(0, MAX_USERNAME_LOGGED) +
          ' from ' +
          clientAddress(req)
      )
      return res.status(200).send({
        ok: false,
        message: 'Too many failed attempts, try again later.',
      })
    }
    const address = clientAddress(req)
    if (
      signinsInProgress >= SIGNIN_MAX_CONCURRENT ||
      (signinsByAddress.get(address) || 0) >= SIGNIN_MAX_CONCURRENT_PER_ADDRESS
    )
      return res.status(503).send({
        ok: false,
        message: 'Server busy, try again in a moment.',
      })
    const failed = () => {
      signinFailed(attemptKey)
      signinFailed(addressKey, SIGNIN_MAX_FAILURES_PER_ADDRESS)
      UserActionsQueue.enqueue({
        username: req.body.username.slice(0, MAX_USERNAME_LOGGED),
        action: 'signinFailed',
        properties: { address: clientAddress(req) },
        timeTag: new Date(),
      })
      return res
        .status(200)
        .clearCookie('x-access-token', {
          httpOnly: true,
          secure: isSecureRequest(req),
          sameSite: 'lax',
        })
        .send({ ok: false, message: SIGNIN_FAIL_MESSAGE })
    }

    signinsInProgress++
    signinsByAddress.set(address, (signinsByAddress.get(address) || 0) + 1)
    try {
      // A local account is checked here only: its password never goes to the
      // directory, and an LDAP login can never take it over. An LDAP account
      // (or an unknown name, when LDAP is enabled) is checked by the
      // directory only: a password stored for it is never accepted.
      const local = await User.findOne({ username: req.body.username })
        .read('primary') // a password just changed must be the one checked
        .populate({ path: 'roles', select: '-__v', options: { readPreference: 'primary' } })
        .exec()

      if (local && !local.isLDAPUser) {
        const passwordIsValid = await bcrypt.compare(
          req.body.password,
          local.password || DUMMY_HASH
        )
        if (!local.password || !passwordIsValid) return failed()
        user = local

        // hashes made with a lower cost (older versions) are upgraded now
        if (bcrypt.getRounds(user.password) < BCRYPT_COST)
          User.updateOne(
            { _id: user._id, password: user.password },
            { $set: { password: await bcrypt.hash(req.body.password, BCRYPT_COST) } }
          )
            .exec()
            .catch((e) => Log.log('Password rehash: ' + e.message))
      } else if (config.ldap.enabled) {
        user = await authenticateWithLDAP(req.body.username, req.body.password)
        if (!user) return failed()
      } else {
        // no such user (or an LDAP user while LDAP is off): same time as a check
        await bcrypt.compare(req.body.password, DUMMY_HASH)
        return failed()
      }
    } finally {
      signinsInProgress--
      const n = (signinsByAddress.get(address) || 1) - 1
      if (n <= 0) signinsByAddress.delete(address)
      else signinsByAddress.set(address, n)
    }
    signinFailures.delete(attemptKey)

    // Populate roles for LDAP user
    if (user.isLDAPUser) {
      user = await User.findById(user._id).populate('roles', '-__v').exec()
    }

    // register user action (the user name of the account, not of the request)
    registerUserAction(req, 'signin', { username: user.username, noBody: true })

    issueSession(req, res, user)
      .status(200)
      .send({ ok: true, message: 'Signed In' })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ ok: false, message: 'Sign-in error' })
  }
}

// Sign out: the token is revoked (a kept copy stops working) and the cookie removed
exports.signout = async (req, res) => {
  // only a valid session is audited, never what the request claims
  const tok = checkToken(req)
  if (tok !== false) {
    registerUserAction(req, 'signout', { noBody: true })
    await UserRights.revokeToken(tok)
  }

  return res
    .status(200)
    .clearCookie('x-access-token', {
      httpOnly: true,
      secure: isSecureRequest(req),
      sameSite: 'lax',
    })
    .send({
      ok: true,
      message: 'Signed Out!',
    })
}

// Password change of the signed-in user (not for LDAP users). Every session of
// the user issued before it ends; this one gets a new token.
exports.changePassword = async (req, res) => {
  Log.log('User request for password change.')
  try {
    const ck = checkToken(req)
    let rights = null
    try {
      rights = ck === false ? null : await UserRights.getCurrentRights(ck, { fresh: true })
    } catch (e) {
      res.status(200).send({ error: 'Database not available, try again later!' })
      return
    }
    if (!rights || ck.username !== req.body.username || !rights.changePassword) {
      res.status(200).send({ error: "Can't change password!" })
      Log.log("Can't change password!")
      return
    }

    let user = await User.findById(ck.id).read('primary').exec()
    if (!user) {
      res.status(200).send({ error: 'User not found!' })
      Log.log('Change password user not found!')
      return
    }

    // Prevent password changes for LDAP users
    if (user.isLDAPUser) {
      res.status(200).send({ error: 'Cannot change password for LDAP users!' })
      Log.log('Cannot change password for LDAP users!')
      return
    }

    if (
      typeof req.body.currentPassword !== 'string' ||
      req.body.currentPassword === ''
    ) {
      res.status(200).send({ error: 'Invalid current password!' })
      Log.log('Invalid current password!')
      return
    }

    const passwordIsValid = await bcrypt.compare(
      req.body.currentPassword,
      user.password
    )
    if (!passwordIsValid) {
      res.status(200).send({ error: 'Wrong current password!' })
      Log.log('Wrong current password!')
      return
    }

    if (typeof req.body.newPassword !== 'string' || req.body.newPassword === '') {
      res.status(200).send({ error: 'Invalid new password!' })
      Log.log('Invalid new password!')
      return
    }
    if (passwordTooShort(req.body.newPassword)) {
      res.status(200).send({ error: PASSWORD_TOO_SHORT })
      return
    }
    const newHash = await bcrypt.hash(req.body.newPassword, BCRYPT_COST)
    // one write for the hash and the token version (older tokens are refused
    // from then on), and only over the password just checked: a concurrent
    // change (each hash has its own salt) makes this one fail, not both apply
    user = await User.findOneAndUpdate(
      { _id: user._id, password: user.password },
      { $set: { password: newHash }, $inc: { tokenVersion: 1 } },
      { new: true }
    )
      .read('primary')
      .populate({ path: 'roles', select: '-__v', options: { readPreference: 'primary' } })
      .exec()
    UserRights.invalidate()
    if (!user) {
      res.status(200).send({ error: 'Password changed meanwhile, try again!' })
      Log.log('Change password: concurrent change, refused')
      return
    }
    registerUserAction(req, 'changePassword')
    Log.log('Password changed!')

    // give this session a new token (with the new token version)
    issueSession(req, res, user).status(200).send({})
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: 'Password change error' })
  }
}

// ---------------------------------------------------------------------------
// Platform maintenance scripts (project export/import and process restarts).
//
// These endpoints run scripts shipped with the installation. Everything about
// the command line is decided here and never by the request: the program is
// picked from the fixed table below and joined to a fixed directory, and no
// shell is involved, so arguments are passed as separate argv entries instead
// of being parsed as a command line. Requests can only choose *whether* one of
// these fixed scripts runs, never *what* runs.
// ---------------------------------------------------------------------------

const IS_WINDOWS = process.platform === 'win32'

// Installation root used by the maintenance scripts. Read once at startup from
// the same JS_INSTALL_DIR override the process manager honours; it is server
// configuration, never anything a request can influence. Note that the scripts
// themselves still derive their own paths from ~/json-scada (c:\json-scada on
// Windows), so an override that does not match the real install makes them fail
// to be found rather than run against the wrong tree.
const JS_ROOT_DIR =
  process.env.JS_INSTALL_DIR && process.env.JS_INSTALL_DIR.trim() !== ''
    ? path.resolve(process.env.JS_INSTALL_DIR.trim())
    : IS_WINDOWS
      ? 'c:\\json-scada'
      : path.join(os.homedir(), 'json-scada')
const PLATFORM_SCRIPTS_DIR = path.join(
  JS_ROOT_DIR,
  IS_WINDOWS ? 'platform-windows' : 'platform-linux'
)
const PROJECT_TMP_DIR = path.join(JS_ROOT_DIR, 'tmp')
// The project import and export scripts use the fixed PROJECT_TMP_DIR (the
// export starts by emptying it): one of them at a time, from before the first
// file is written there until its last file there was used and removed.
let projectBusy = null // 'importProject' | 'exportProject' | null
function takeProjectLock(action) {
  if (projectBusy)
    throw new Error(
      'A project ' + (projectBusy === 'importProject' ? 'import' : 'export') +
        ' is already running, try again later!'
    )
  projectBusy = action
  let released = false
  return () => {
    if (!released && projectBusy === action) projectBusy = null
    released = true
  }
}

// The only scripts these endpoints are ever allowed to run.
const PLATFORM_SCRIPTS = {
  exportProject: IS_WINDOWS ? 'export_project.bat' : 'export_project.sh',
  importProject: IS_WINDOWS ? 'import_project.bat' : 'import_project.sh',
  restartProtocols: IS_WINDOWS
    ? 'restart_protocols.bat'
    : 'restart_protocols.sh',
  restartProcesses: IS_WINDOWS
    ? 'restart_services.bat'
    : 'restart_processes.sh',
}

// Why the users/roles of a project zip (mongoexport files, one JSON document
// per line) cannot be imported, or null: both files are needed and some user
// must hold a role with isAdmin.
function projectAdminProblem(zip) {
  const read = (name) => {
    const e = zip.getEntry(name)
    return e ? zip.readAsText(e) : null
  }
  const rolesTxt = read('roles.json')
  const usersTxt = read('users.json')
  if (rolesTxt === null || usersTxt === null)
    return 'Project file without users.json/roles.json: the import would remove all users!'
  const docs = (txt) =>
    txt
      .split(/\r?\n/)
      .filter((l) => l.trim() !== '')
      .map((l) => {
        try {
          return JSON.parse(l)
        } catch (e) {
          return null
        }
      })
  const oid = (v) => (v && typeof v === 'object' ? v.$oid : v)
  const adminRoles = new Set(
    docs(rolesTxt)
      .filter((r) => r && r.isAdmin === true)
      .map((r) => String(oid(r._id)))
  )
  const hasAdmin = docs(usersTxt).some(
    (u) =>
      u &&
      Array.isArray(u.roles) &&
      u.roles.some((r) => adminRoles.has(String(oid(r))))
  )
  return hasAdmin
    ? null
    : 'Project file has no user with an admin role: the import would lock everybody out!'
}

// limits of an uploaded project zip once unpacked (uploads are up to 100 MB)
const MAX_PROJECT_UNPACKED_BYTES = 1024 * 1024 * 1024
const MAX_PROJECT_ENTRIES = 10000

// Kill a script that never finishes instead of leaking child processes.
const SCRIPT_TIMEOUT_MS = 5 * 60 * 1000

// Scripts currently running, so that repeated requests cannot pile up an
// unbounded number of restarts/imports on the machine.
const scriptsInProgress = new Set()

// Spawns one of the scripts of PLATFORM_SCRIPTS. `action` is a key of that
// table written in this file - never a value taken from a request. `args` are
// handed over as argv entries with shell:false; on Windows the .bat is run by
// the command interpreter itself (cmd.exe /c <script> <args...>), which also
// receives them as separate argv entries and not as a line to re-parse.
// Returns the ChildProcess, or throws if the action is unknown, the script is
// missing or the same script is still running.
function spawnPlatformScript(action, args = []) {
  if (!Object.prototype.hasOwnProperty.call(PLATFORM_SCRIPTS, action))
    throw new Error('Unknown maintenance script: ' + action)
  const scriptName = PLATFORM_SCRIPTS[action]
  const scriptPath = path.join(PLATFORM_SCRIPTS_DIR, scriptName)

  let stat = null
  try {
    stat = fs.statSync(scriptPath)
  } catch (e) {
    stat = null
  }
  if (stat === null || !stat.isFile())
    throw new Error('Maintenance script not found: ' + scriptPath)

  if (scriptsInProgress.has(action))
    throw new Error(scriptName + ' is already running!')

  const child = IS_WINDOWS
    ? spawn(process.env.ComSpec || 'cmd.exe', ['/c', scriptPath, ...args], {
        shell: false,
        windowsHide: true,
      })
    : // absolute interpreter path: not subject to a hijacked PATH
      spawn('/bin/sh', [scriptPath, ...args], { shell: false })

  scriptsInProgress.add(action)
  Log.log('Running ' + scriptPath)

  child.stdout.on('data', (data) => Log.log(`stdout: ${data}`))
  child.stderr.on('data', (data) => Log.log(`stderr: ${data}`))
  // Without an 'error' listener a failed spawn raises an unhandled 'error'
  // event, which would take down the whole web server process.
  child.on('error', (err) =>
    Log.log(`${scriptName} could not be executed: ${err.message}`)
  )

  const timer = setTimeout(() => {
    Log.log(`${scriptName} timed out after ${SCRIPT_TIMEOUT_MS}ms, killing it`)
    try {
      child.kill()
    } catch (e) {
      /* already gone */
    }
  }, SCRIPT_TIMEOUT_MS)

  child.on('close', () => {
    clearTimeout(timer)
    scriptsInProgress.delete(action)
  })

  return child
}

// export project file: dump collections and some files to a zip file
exports.exportProject = async (req, res) => {
  Log.log('Save project')
  try {
    let project = req.body.project
    if (!project.fileName || project.fileName.trim() == '')
      project.fileName = 'new_project_' + new Date().getTime() / 1000 + '.zip'
    project.fileName = project.fileName.trim()

    // Strictly validate the file name before it ever reaches a shell or the
    // file system. Only a plain zip file name is allowed: no path separators,
    // no directory traversal and no shell metacharacters. This neutralizes
    // both command injection (spawn) and arbitrary file read (res.download)
    // through the user-supplied name.
    const safeFileName = /^[a-zA-Z0-9][a-zA-Z0-9._-]{0,250}\.zip$/
    if (!safeFileName.test(project.fileName)) {
      Log.log('Invalid project file name: ' + project.fileName)
      res.status(400).send({ error: 'Invalid project file name!' })
      return
    }

    // The file name is passed as a separate argv entry and never reaches a
    // shell, so it cannot extend the command line.
    const release = takeProjectLock('exportProject')
    let cmd
    try {
      cmd = spawnPlatformScript('exportProject', [project.fileName])
    } catch (e) {
      release()
      throw e
    }
    cmd.on('close', (code) => {
      Log.log(`child process exited with code ${code}`)
      // basename is a further guard even though the name is already validated.
      const filePath = path.join(
        PROJECT_TMP_DIR,
        path.basename(project.fileName)
      )
      if (!fs.existsSync(filePath) || code != 0) {
        release()
        Log.log('Project file not found! ' + filePath)
        res.status(200).send({ error: 'Project file not found!' })
        return
      }
      registerUserAction(req, 'exportProject')
      // the exported file holds the whole configuration: do not leave it behind
      res.download(filePath, (err) => {
        if (err) Log.log('Project download error: ' + err.message)
        fs.unlink(filePath, (e) => {
          if (e) Log.log('Could not remove ' + filePath + ': ' + e.message)
          release()
        })
      })
    })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err.message || String(err) })
  }
}

// import project file: download, extract zip project file, import collections and move some files
exports.importProject = async (req, res) => {
  Log.log('Import project')
  let zipPath = null // removed on any failure before the import starts
  let started = false
  let release = () => {}
  try {
    if (!req.files || !req.files.projectFileData)
      throw new Error('No project file uploaded!')

    // Strictly validate the uploaded file name. Strip any directory component
    // with path.basename and allow only a plain zip name, so the name can not
    // be used to write outside the tmp directory (path traversal).
    const safeName = path.basename((req.body.projectFileName || '').trim())
    const safeFileName = /^[a-zA-Z0-9][a-zA-Z0-9._-]{0,250}\.zip$/
    if (!safeFileName.test(safeName)) {
      Log.log('Invalid project file name: ' + req.body.projectFileName)
      res.status(400).send({ error: 'Invalid project file name!' })
      return
    }

    // An upload that hit the express-fileupload size limit arrives truncated:
    // do not try to unpack a partial archive.
    const upload = req.files.projectFileData
    if (upload.truncated) {
      Log.log('Project file too large: ' + safeName)
      res.status(413).send({ error: 'Project file too large!' })
      return
    }
    if (!upload.size) {
      Log.log('Empty project file: ' + safeName)
      res.status(400).send({ error: 'Empty project file!' })
      return
    }

    // taken before anything is written to the shared directory: a refused
    // request must not overwrite (nor clean up) the files of a running one
    release = takeProjectLock('importProject')

    // Fixed extraction directory, not influenced by the request.
    const projectPath = PROJECT_TMP_DIR
    if (!fs.existsSync(projectPath))
      fs.mkdirSync(projectPath, { recursive: true, mode: 0o700 })

    zipPath = path.join(projectPath, safeName)
    await upload.mv(zipPath)
    const zip = new AdmZip(zipPath)

    // Zip-Slip guard: reject archives whose entries would be written outside
    // the extraction directory via path traversal, before extracting anything.
    // The resolve/prefix test below is the check that matters; the explicit
    // rejections in front of it also stop names that are harmless on the
    // platform doing the extraction but traverse on the other one (a '\'
    // separator is a path separator on Windows and an ordinary character on
    // Linux, and adm-zip writes entry names verbatim).
    const resolvedBase = path.resolve(projectPath)
    const entries = zip.getEntries()
    // zip bomb guard: limits on the unpacked size and number of entries
    if (entries.length > MAX_PROJECT_ENTRIES)
      throw new Error('Project file has too many entries!')
    const unpacked = entries.reduce((n, e) => n + (Number(e.header?.size) || 0), 0)
    if (unpacked > MAX_PROJECT_UNPACKED_BYTES)
      throw new Error('Project file too large when unpacked!')
    for (const entry of entries) {
      const entryName = entry.entryName
      // directory entries legitimately end with '/', everything else must be a
      // plain relative path of non-empty, non-traversing segments
      const parts = entryName.replace(/\/+$/, '').split('/')
      if (
        entryName.includes('\\') ||
        entryName.startsWith('/') ||
        /^[a-zA-Z]:/.test(entryName) ||
        parts.some(
          (part) => part === '..' || part === '.' || part.trim() === ''
        )
      ) {
        throw new Error('Invalid ZIP entry name: ' + entryName)
      }
      const resolvedTarget = path.resolve(projectPath, entryName)
      if (
        resolvedTarget !== resolvedBase &&
        !resolvedTarget.startsWith(resolvedBase + path.sep)
      ) {
        throw new Error(
          'ZIP entry attempts to write outside target directory: ' + entryName
        )
      }
    }

    // the import scripts replace all users and roles: a project without a
    // user holding an admin role would lock everybody out
    const adminProblem = projectAdminProblem(zip)
    if (adminProblem) throw new Error(adminProblem)

    zip.extractAllTo(projectPath, true)
    Log.log('Files extracted to: ' + projectPath)

    // the extracted files hold the whole configuration (users, connection
    // passwords): remove them, and the zip, once the import is over
    const cleanup = () => {
      const files = [
        zipPath,
        ...entries
          .filter((e) => !e.isDirectory)
          .map((e) => path.resolve(projectPath, e.entryName)),
      ]
      for (const p of files)
        try {
          fs.unlinkSync(p)
        } catch (e) {
          if (e.code !== 'ENOENT') Log.log('Could not remove ' + p + ': ' + e.message)
        }
      release() // the directory is free again
    }

    // Fixed script, no shell, no arguments taken from the request.
    let cmd
    try {
      cmd = spawnPlatformScript('importProject')
    } catch (e) {
      cleanup()
      throw e
    }
    started = true
    let spawnError = null
    cmd.on('error', (e) => (spawnError = e))
    cmd.on('close', (code) => {
      Log.log(`child process exited with code ${code}`)
      cleanup()
      // users and roles were replaced behind the back of the rights cache
      UserRights.invalidate()
      registerUserAction(req, 'importProject', {
        body: { projectFileName: safeName, exitCode: code },
      })
      if (spawnError || code !== 0) {
        res.status(200).send({
          error:
            'Project import failed (' +
            (spawnError ? spawnError.message : 'exit code ' + code) +
            '), see the server log!',
        })
        return
      }
      res.status(200).send({ error: false })
    })
  } catch (err) {
    Log.log(err)
    if (!started) {
      if (zipPath)
        try {
          fs.unlinkSync(zipPath)
        } catch (e) {}
      release()
    }
    res.status(200).send({ error: err.message || String(err) })
  }
}

exports.restartProtocols = async (req, res) => {
  Log.log('restartProtocols')
  try {
    // No arguments and no shell: the request decides only that the fixed
    // restart_protocols script of this installation is started.
    const cmd = spawnPlatformScript('restartProtocols')
    cmd.on('close', (code) => Log.log(`child process exited with code ${code}`))

    registerUserAction(req, 'restartProtocols')
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err.message || String(err) })
  }
}

exports.restartProcesses = async (req, res) => {
  Log.log('restartProcesses')
  try {
    // No arguments and no shell: the request decides only that the fixed
    // restart_processes/restart_services script of this install is started.
    const cmd = spawnPlatformScript('restartProcesses')
    cmd.on('close', (code) => Log.log(`child process exited with code ${code}`))

    registerUserAction(req, 'restartProcesses')
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err.message || String(err) })
  }
}

// Displays shipped with the distribution (KAW2/KIK3/KNH2/KOR1 examples and the
// generated Svedala CIM-EMS demo); user displays never match
const DEMO_DISPLAY_RE =
  /^(kaw2|kik3|knh2|kor1|kor1-PergolaGrid|svedala-overview|svedala_[A-Za-z0-9_-]+)\.svg$/i
// CIM-EMS collections (models, bindings, runs, lists, requests) and artifacts bucket
const CIM_COLLECTIONS = [
  'cimModels',
  'cimMeasurementBindings',
  'cimRuns',
  'cimContingencyLists',
  'cimStudyRequests',
  'cimArtifacts.files',
  'cimArtifacts.chunks',
]
// Tags kept by a prune: reserved keys, system and communication status tags
// (the latter are removed by cs_data_processor with their connections)
const PRUNE_TAG_FILTER = {
  _id: { $nin: [-1, -2] },
  origin: { $ne: 'system' },
  commStatus: { $exists: false },
  tag: { $not: /^_System\./ },
}

// Removes the demo/example configuration so a system can start clean: every
// protocol connection, every non-system tag, all CIM-EMS models/bindings/runs/
// requests/artifacts and the shipped example displays. Driver instances, process
// instances, users, roles and system settings stay. Dry run unless confirm: true.
exports.pruneConfiguration = async (req, res) => {
  try {
    const b = req.body || {}
    const execute = b.confirm === true
    const want = (k) => b[k] !== false
    const svgDir = path.resolve(__dirname, '..', '..', '..', '..', 'svg')
    const out = { error: false, dryRun: !execute, warnings: [] }

    let conns = []
    if (want('connections')) {
      conns = await ProtocolConnection.find(
        {},
        { protocolConnectionNumber: 1, name: 1, protocolDriver: 1, protocolDriverInstanceNumber: 1 }
      )
        .lean()
        .exec()
      out.connections = {
        count: conns.length,
        names: conns.map((c) => c.name),
      }
    }
    if (want('tags'))
      out.tags = { count: await Tag.countDocuments(PRUNE_TAG_FILTER) }
    const mdb = db.mongoose.connection.db
    if (want('cimModels')) {
      out.cimModels = {}
      for (const c of CIM_COLLECTIONS)
        out.cimModels[c] = await mdb.collection(c).countDocuments({})
      out.cimModels.modelIds = await mdb.collection('cimModels').distinct('_id')
    }
    let displays = []
    if (want('displays')) {
      try {
        displays = fs
          .readdirSync(svgDir)
          .filter((f) => DEMO_DISPLAY_RE.test(f))
          .sort()
      } catch (e) {
        out.warnings.push('Cannot read the svg folder: ' + e.message)
      }
      out.displays = { count: displays.length, files: displays }
      if (displays.length && fs.existsSync(path.join(svgDir, 'screen_list.js')))
        out.warnings.push(
          'svg/screen_list.js exists: remove the pruned displays from it (or delete it to list every display).'
        )
    }

    if (!execute) {
      res.status(200).send(out)
      return
    }
    registerUserAction(req, 'pruneConfiguration')

    if (out.tags)
      out.tags.deleted = (await Tag.deleteMany(PRUNE_TAG_FILTER)).deletedCount
    if (out.connections) {
      out.connections.deleted = (
        await ProtocolConnection.deleteMany({
          _id: { $in: conns.map((c) => c._id) },
        })
      ).deletedCount
      // the tags of the connections, when tags were not pruned as a whole
      if (!out.tags)
        out.connections.tagsDeleted = (
          await Tag.deleteMany({
            protocolSourceConnectionNumber: {
              $in: conns.map((c) => c.protocolConnectionNumber),
            },
            origin: { $ne: 'system' },
            commStatus: { $exists: false },
          })
        ).deletedCount
      const seen = new Set()
      out.connections.restarts = []
      for (const c of conns) {
        const key = c.protocolDriver + '#' + c.protocolDriverInstanceNumber
        if (seen.has(key)) continue
        seen.add(key)
        out.connections.restarts.push({
          protocolDriver: c.protocolDriver,
          protocolDriverInstanceNumber: c.protocolDriverInstanceNumber,
          ...(await scheduleConnectionRestart(
            c.protocolDriver,
            c.protocolDriverInstanceNumber,
            null
          )),
        })
      }
    }
    if (out.cimModels) {
      out.cimModels.deleted = {}
      for (const c of CIM_COLLECTIONS)
        out.cimModels.deleted[c] = (
          await mdb.collection(c).deleteMany({})
        ).deletedCount
    }
    if (out.displays) {
      out.displays.deleted = []
      for (const f of displays) {
        const p = path.resolve(svgDir, path.basename(f))
        if (!p.startsWith(svgDir + path.sep)) continue
        try {
          fs.unlinkSync(p)
          out.displays.deleted.push(f)
        } catch (e) {
          out.warnings.push('Cannot delete display ' + f + ': ' + e.message)
        }
      }
    }
    res.status(200).send(out)
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err.message || String(err) })
  }
}

// placeholder for future use (sanitize database, fill up missing properties, change data types, etc.)
exports.sanitizeDatabase = async function (req, res) {
  res.status(200).send({ error: false })
}

// enqueue user action for later insertion to mongodb (UserActionsQueue removes
// secrets). Without a valid token (sign-in, sign-up) the user name is the one
// of the request: pass `username` once it is known to be genuine.
function registerUserAction(req, actionName, opts = {}) {
  const body = Object.assign({}, req.body)
  const ck = req.token || checkToken(req)
  const username =
    ck !== false && ck ? ck.username : opts.username ?? req.body?.username
  Log.log(actionName + ' - ' + String(username).substring(0, MAX_USERNAME_LOGGED))
  UserActionsQueue.enqueue({
    username: username,
    properties: opts.noBody ? {} : opts.body ?? body,
    action: actionName,
    timeTag: new Date(),
  })
}

// LDAP authentication helper
async function authenticateWithLDAP(username, password) {
  if (!config.ldap.enabled) return null

  // The credentials must be strings: anything else (a number, or an object
  // sent as JSON) has no defined meaning for a bind and must not reach it.
  if (typeof username !== 'string' || typeof password !== 'string') {
    Log.log('LDAP - Credentials of invalid type rejected')
    return null
  }
  if (username.trim() === '') {
    Log.log('LDAP - Empty username rejected')
    return null
  }
  // An empty password must never reach client.bind(). A simple bind carrying a
  // zero-length password is an *unauthenticated* bind (RFC 4513 5.1.2): the
  // server answers success without checking anything, which would let any
  // existing account be logged into with no password at all.
  if (password.length === 0) {
    Log.log('LDAP - Empty password rejected for username: ' + username)
    return null
  }

  // Everything below that comes from the request is escaped before it is put
  // into a search filter (RFC 4515) or into a DN (RFC 4514), so a username
  // like '*)(uid=*' can only ever match a user literally named that instead of
  // changing the shape of the filter.
  const filterUsername = Filter.escape(username)
  const userDNFor = (attribute) =>
    new DN().addPairRDN(attribute, username).toString() +
    ',' +
    config.ldap.searchBase

  Log.log('LDAP - Server: ' + config.ldap.url)

  let tlsOptions = null
  if (config.ldap.url.startsWith('ldaps')) {
    tlsOptions = config.ldap.tlsOptions
  }
  const client = new Client({
    url: config.ldap.url,
    timeout: 5000,
    connectTimeout: 5000,
    tlsOptions: tlsOptions,
  })

  let userDN = ''
  try {
    let userEntry = null
    try {
      // Bind with admin credentials to search for user
      await client.bind(config.ldap.bindDN, config.ldap.bindCredentials)
      Log.log('LDAP - Ok for BindDN: ' + config.ldap.bindDN)

      // Search for user (replaceAll: the configured filter may name the
      // placeholder more than once, as the Active Directory example does)
      const searchFilter = config.ldap.searchFilter.replaceAll(
        '{{username}}',
        filterUsername
      )
      const { searchEntries } = await client.search(config.ldap.searchBase, {
        filter: searchFilter,
        attributes: Object.values(config.ldap.attributes),
      })

      if (searchEntries.length === 0) {
        Log.log('LDAP - User not found: ' + username)
        return null
      }
      // An authentication decision must not be taken on an ambiguous match:
      // refuse rather than pick the first of several entries.
      if (searchEntries.length > 1) {
        Log.log(
          'LDAP - Ambiguous match (' +
            searchEntries.length +
            ' entries) for username: ' +
            username
        )
        return null
      }

      userEntry = searchEntries[0]
      // Log.log('LDAP - User found: ' + JSON.stringify(userEntry))
      Log.log('LDAP - User found: ' + username)
      userDN = userEntry.dn
    } catch (err) {
      Log.log('LDAP - Error for BindDN: ' + config.ldap.bindDN)
    }

    if (userDN === '') {
      userDN = userDNFor(config.ldap.attributes.username)
    }

    try {
      // Try to bind with user credentials to verify password
      await client.bind(userDN, password)
      Log.log('LDAP - Ok for userDN: ' + userDN)
    } catch (err) {
      Log.log('LDAP - Auth error for userDN: ' + userDN)

      userDN = userDNFor(config.ldap.attributes.displayName)

      await client.bind(userDN, password)
      Log.log('LDAP - Ok for userDN: ' + userDN)
    }

    if (!userEntry) {
      // Search for user
      const searchFilter = config.ldap.searchFilter.replaceAll(
        '{{username}}',
        filterUsername
      )
      const { searchEntries } = await client.search(config.ldap.searchBase, {
        filter: searchFilter,
        attributes: Object.values(config.ldap.attributes),
      })

      if (searchEntries.length === 0) {
        Log.log('LDAP - User not found: ' + searchFilter)
        return null
      }
      if (searchEntries.length > 1) {
        Log.log(
          'LDAP - Ambiguous match (' +
            searchEntries.length +
            ' entries) for username: ' +
            username
        )
        return null
      }

      userEntry = searchEntries[0]
      // Log.log('LDAP - User entry found: ' + JSON.stringify(userEntry))
      Log.log('LDAP - User found: ' + username)
      userDN = userEntry.dn
    }

    if (userEntry?.memberOf?.constructor === String) {
      userEntry.memberOf = [userEntry.memberOf]
    }

    if (!(config.ldap.attributes.email in userEntry)) {
      userEntry[config.ldap.attributes.email] = ''
    }
    if (userEntry[config.ldap.attributes.email].constructor === Array) {
      if (userEntry[config.ldap.attributes.email].length > 0)
        userEntry[config.ldap.attributes.email] =
          userEntry[config.ldap.attributes.email][0]
      else userEntry[config.ldap.attributes.email] = ''
    }

    // Map LDAP attributes to user object
    const userData = {
      username: userEntry[config.ldap.attributes.username],
      email: userEntry[config.ldap.attributes.email],
      isLDAPUser: true,
      ldapDN: userDN,
      lastLDAPSync: new Date(),
    }

    // Find or create user in local database
    let user = await User.findOne({ username: userData.username })
    // the directory may answer with a name of its own (case, other attribute):
    // a local account with that name is never turned into this LDAP user
    if (user && !user.isLDAPUser) {
      Log.log(
        'LDAP - Refused: a local (non LDAP) user has the name ' +
          userData.username
      )
      return null
    }

    const defaultRole = await Role.findOne({ name: config.ldap.defaultRole })

    if (!user) {
      // Create new user
      user = new User(userData)
    } else {
      // Update existing user's LDAP info
      user.lastLDAPSync = userData.lastLDAPSync
      user.email = userData.email
    }

    if (defaultRole) {
      user.roles = [defaultRole._id]
    }

    // Check LDAP groups and assign additional roles, if any
    if (userEntry.memberOf) {
      Log.log('LDAP - User groups: ' + userEntry.memberOf)
      for (const group of userEntry.memberOf) {
        const roleName = config.ldap.groupMapping[group.toLowerCase()]
        if (roleName) {
          const role = await Role.findOne({ name: roleName })
          if (role && !user.roles.includes(role._id)) {
            user.roles.push(role._id)
            Log.log('LDAP - User role: ' + roleName)
          }
          if (!role) {
            Log.log('LDAP - Role not found: ' + roleName)
          }
        } else {
          Log.log('LDAP - Group/role not mapped: ' + group)
        }
      }
    } else {
      try {
        // userDN carries the supplied username when the directory could not be
        // searched beforehand, so it is escaped here as well: role assignment
        // must not be steerable through the login name.
        const filterUserDN = Filter.escape(userDN)
        const filter =
          '(&(|(objectClass=groupOfUniqueNames)(objectClass=groupOfNames)(objectClass=group))(|(uniqueMember=' +
          filterUserDN +
          ')(member=' +
          filterUserDN +
          ')))'
        Log.log('LDAP - Search for user groups: ' + filter)
        const { searchEntries, searchReferences } = await client.search(
          config.ldap.groupSearchBase,
          {
            scope: 'sub',
            filter: filter,
          }
        )

        if (searchEntries.length > 0) {
          // Log.log('LDAP - User groups: ' + JSON.stringify(searchEntries))
          for (const group of searchEntries) {
            const roleName = config.ldap.groupMapping[group.dn.toLowerCase()]
            if (roleName) {
              const role = await Role.findOne({ name: roleName })
              if (role && !user.roles.includes(role._id)) {
                user.roles.push(role._id)
                Log.log('LDAP - User role: ' + roleName)
              }
              if (!role) {
                Log.log('LDAP - Role not found: ' + roleName)
              }
            } else {
              Log.log('LDAP - Group/role not mapped: ' + group.dn)
            }
          }
        } else {
          Log.log('LDAP - User groups not found: ' + filter)
        }
      } catch (err) {
        Log.log('LDAP - Error searching for user groups: ' + err)
      }
    }

    await user.save()

    return user
  } catch (error) {
    Log.log('LDAP - error for userDN ' + userDN + ': ' + error)
    return null
  } finally {
    // Close the connection on every path, including the rejected logins that
    // return early above, so failed attempts cannot pile up open binds.
    try {
      await client.unbind()
    } catch (e) {
      /* already closed or never connected */
    }
  }
}
