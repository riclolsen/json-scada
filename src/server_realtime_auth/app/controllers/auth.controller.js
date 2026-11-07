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
const ProcessManager = require('../services/process-manager')
const RestartScheduler = require('../services/restart-scheduler')
const SystemSettings = require('../services/system-settings')
const User = db.user
const Role = db.role
const Tag = db.tag
const ProtocolDriverInstance = db.protocolDriverInstance
const ProtocolConnection = db.protocolConnection
const UserAction = db.userAction

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

// add header to identify json-scada user for Grafana auto-login (auth.proxy)
exports.addXWebAuthUser = (req) => {
  let ck = checkToken(req)
  if (ck !== false) req.headers['X-WEBAUTH-USER'] = ck?.username
}

exports.listUserActions = async (req, res) => {
  Log.log('listUserActions')

  let skip = 0
  if ('page' in req.body && 'itemsPerPage' in req.body)
    skip = req.body.itemsPerPage * (req.body.page - 1)
  let filter = {}
  if ('filter' in req.body) filter = req.body.filter

  let limit = req.body.itemsPerPage || 10
  let orderby = {}
  if ('sortBy' in req.body) {
    for (let i = 0; i < req.body.sortBy.length; i++)
      orderby[req.body.sortBy[i].key] =
        req.body.sortBy[i]?.order === 'desc' ? -1 : 1
    if (req.body.sortBy.length === 0) orderby = { timeTag: 1 }
  } else orderby = { timeTag: 1 }

  try {
    let count = await UserAction.countDocuments(filter)
    let userActions = await UserAction.find(filter)
      .skip(skip)
      .limit(limit)
      .sort(orderby)
      .exec()
    res.status(200).send({ userActions: userActions, countTotal: count })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.createTag = async (req, res) => {
  Log.log('createTag')
  try {
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
    res.status(200).send({ error: err })
  }
}

exports.updateTag = async (req, res) => {
  try {
    registerUserAction(req, 'updateTag')

    if ('_id' in req.body) {
      let _id = req.body._id
      delete req.body._id

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

      await Tag.findOneAndUpdate({ _id: _id }, req.body)
      res.status(200).send({ error: false })
    } else res.status(200).send({ error: 'No _id in update request.' })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.deleteTag = async (req, res) => {
  try {
    registerUserAction(req, 'deleteTag')

    if ('_id' in req.body) {
      await Tag.findOneAndDelete({ _id: req.body._id })
      res.status(200).send({ error: false })
    } else res.status(200).send({ error: 'No _id in delete request.' })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.listTags = async (req, res) => {
  Log.log('listTags')

  let skip = 0
  if ('page' in req.body && 'itemsPerPage' in req.body)
    skip = req.body.itemsPerPage * (req.body.page - 1)
  let filter = {}
  if ('filter' in req.body) filter = req.body.filter
  req.body.filter['_id'] = { $nin: [-1, -2] }

  let limit = req.body.itemsPerPage || 10
  let orderby = {}
  if ('sortBy' in req.body) {
    for (let i = 0; i < req.body.sortBy.length; i++)
      orderby[req.body.sortBy[i].key] =
        req.body.sortBy[i]?.order === 'desc' ? -1 : 1
    if (req.body.sortBy.length === 0) orderby = { tag: 1 }
  } else orderby = { tag: 1 }
  try {
    let count = await Tag.countDocuments(filter)
    let tags = await Tag.find(filter)
      .skip(skip)
      .limit(limit)
      .sort(orderby)
      .exec()
    let ret = { tags: tags, countTotal: count }
    res.status(200).send(ret)
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
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
    res.status(200).send({ error: err })
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
    registerUserAction(req, 'deleteProtocolConnection')

    if (req.body?.deleteTags === true) {
      await Tag.deleteMany({
        protocolSourceConnectionNumber: req.body.protocolConnectionNumber,
      })
    }

    const conn = await ProtocolConnection.findOneAndDelete({
      _id: req.body._id,
    })
      .lean()
      .exec()

    const restartInfo = conn
      ? await scheduleConnectionRestart(
          conn.protocolDriver,
          conn.protocolDriverInstanceNumber,
          null
        )
      : {}

    res.status(200).send({ error: false, ...restartInfo })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
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
    res.status(200).send({ error: err })
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
    res.status(200).send({ error: err })
  }
}

exports.listProtocolConnections = async (req, res) => {
  Log.log('listProtocolConnections')

  try {
    let protocolConnections = await ProtocolConnection.find({}).exec()
    res.status(200).send(protocolConnections)
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.deleteProtocolDriverInstance = async (req, res) => {
  try {
    registerUserAction(req, 'deleteProtocolDriverInstance')

    const doc = await ProtocolDriverInstance.findOne({ _id: req.body._id })
      .lean()
      .exec()
    await ProtocolDriverInstance.findOneAndDelete({ _id: req.body._id })

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
    res.status(200).send({ error: err })
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
    res.status(200).send({ error: err })
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
    res.status(200).send({ error: err })
  }
}

exports.listProtocolDriverInstances = async (req, res) => {
  Log.log('listProtocolDriverInstances')

  try {
    let driverInstances = await ProtocolDriverInstance.find({}).exec()
    res.status(200).send(driverInstances)
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.updateProtocolDriverInstance = async (req, res) => {
  try {
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
    res.status(200).send({ error: err })
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
    res.status(200).send({ error: err })
  }
}

exports.listRoles = async (req, res) => {
  Log.log('listRoles')

  try {
    let roles = await Role.find({}).exec()
    res.status(200).send(roles)
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.userAddRole = async (req, res) => {
  try {
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
    user.roles.push(role._id)
    await user.save()
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.userRemoveRole = async (req, res) => {
  try {
    if (req?.body?.username === 'admin' && req.body.role === 'admin') {
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
    user.roles.pull(role._id)
    await user.save()
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.listGroup1 = async (req, res) => {
  Log.log('listGroup1')

  try {
    let groups = await Tag.find().distinct('group1').exec()
    res.status(200).send(groups)
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.listDisplays = (req, res) => {
  Log.log('listDisplays')

  fs.readdir('../../svg', function (err, files) {
    //handling error
    if (err) {
      Log.log(err)
      res.status(200).send({ error: err })
      return
    }

    let svgFiles = files.filter(function (file) {
      return path.extname(file).toLowerCase() === '.svg'
    })

    res.status(200).send(svgFiles)
  })
}

exports.openDisplay = (req, res) => {
  Log.log('openDisplay')

  // Sanitize file path to prevent directory traversal
  const svgDir = path.resolve(__dirname, '..', '..', '..', '..', 'svg')
  const requestedPath = path.resolve(
    svgDir,
    path.basename(req.query.file || '')
  )

  if (!requestedPath.startsWith(svgDir)) {
    Log.log('openDisplay: Invalid file path attempted: ' + req.query.file)
    res.status(400).send({ error: 'Invalid file path' })
    return
  }

  fs.readFile(requestedPath, function (err, data) {
    //handling error
    if (err) {
      Log.log(err)
      res.status(200).send({ error: err })
      return
    }

    res.status(200).send(data)
  })
}

exports.saveDisplay = (req, res) => {
  Log.log('saveDisplay')

  // Sanitize file path to prevent directory traversal
  const svgDir = path.resolve(__dirname, '..', '..', '..', '..', 'svg')
  const requestedPath = path.resolve(svgDir, path.basename(req.body.file || ''))

  if (!requestedPath.startsWith(svgDir)) {
    Log.log('saveDisplay: Invalid file path attempted: ' + req.body.file)
    res.status(400).send({ error: 'Invalid file path' })
    return
  }

  fs.writeFile(requestedPath, req.body.content, function (err) {
    //handling error
    if (err) {
      Log.log(err)
      res.status(200).send({ error: err })
      return
    }

    res.status(200).send({ error: false })
  })
}

exports.updateRole = async (req, res) => {
  Log.log('Update role')
  try {
    registerUserAction(req, 'updateRole')
    await Role.findOneAndUpdate({ _id: req.body._id }, req.body)
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.updateUser = async (req, res) => {
  Log.log('Update user: ' + req?.body?.username)
  try {
    registerUserAction(req, 'updateUser')
    if (
      'password' in req.body &&
      req.body.password !== '' &&
      req.body.password !== null
    )
      req.body.password = bcrypt.hashSync(req.body.password, 8)
    else delete req.body['password']
    delete req.body['roles']
    await User.findOneAndUpdate({ _id: req.body._id }, req.body)
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.createRole = async (req, res) => {
  Log.log('Create role')
  try {
    const role = new Role(req.body)
    await role.save()
    req.body._id = role._id
    registerUserAction(req, 'createRole')
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.createUser = async (req, res) => {
  Log.log('Create user')
  try {
    if (req.body.password && req.body.password !== '')
      req.body.password = bcrypt.hashSync(req.body.password, 8)
    const user = new User(req.body)
    await user.save()
    req.body._id = user._id
    registerUserAction(req, 'createUser')
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.deleteRole = async (req, res) => {
  Log.log('Delete role')

  // do not delete a role that is attributed to a user
  let users = await User.find({ roles: req.body._id }).exec()
  if (users.length > 0)
    return res
      .status(200)
      .send({ error: 'Cannot delete role that is attributed to a user!' })

  registerUserAction(req, 'deleteRole')

  try {
    await Role.findOneAndDelete({ _id: req.body._id })
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.deleteUser = async (req, res) => {
  Log.log('Delete user')
  if (req.body.username === 'admin') {
    res.status(200).send({ error: 'Cannot delete admin user!' })
    return
  }
  registerUserAction(req, 'deleteUser')

  try {
    await User.findOneAndDelete({ _id: req.body._id })
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

// create user profile passing username, email, password and roles
exports.signup = async (req, res) => {
  const user = new User({
    username: req.body.username,
    email: req.body.email,
    password: bcrypt.hashSync(req.body.password, 8),
  })

  registerUserAction(req, 'signup')

  try {
    await user.save()

    if (req.body.roles) {
      let roles = await Role.find({
        name: { $in: req.body.roles },
      }).exec()
      user.roles = roles.map((role) => role._id)
      await user.save()
      res.send({ message: 'User was registered successfully!' })
    } else {
      let role = await Role.findOne({ name: 'user' }).exec()
      user.roles = [role._id]
      await user.save()
      res.send({ message: 'User was registered successfully!' })
    }
  } catch (err) {
    Log.log(err)
    res.status(500).send({ message: err })
    return
  }
}

// User signin request
exports.signin = async (req, res) => {
  try {
    let user = null

    // Try LDAP authentication first if enabled
    if (config.ldap.enabled) {
      user = await authenticateWithLDAP(req.body.username, req.body.password)
    }

    // Fall back to local authentication if LDAP auth failed or is disabled
    if (!user) {
      user = await User.findOne({
        username: req.body.username,
      })
        .populate('roles', '-__v')
        .exec()

      if (!user) {
        return res.status(200).send({ ok: false, message: 'User Not found.' })
      }

      const passwordIsValid = bcrypt.compareSync(
        req.body.password,
        user.password
      )

      if (!passwordIsValid) {
        return res.status(200).cookie('x-access-token', null).send({
          ok: false,
          message: 'Wrong Password!',
        })
      }
    }

    // Populate roles for LDAP user
    if (user.isLDAPUser) {
      user = await User.findById(user._id).populate('roles', '-__v').exec()
    }

    // Combines all roles rights for the user
    let authorities = []
    let rights = {
      isAdmin: false,
      changePassword: false,
      sendCommands: false,
      enterAnnotations: false,
      enterNotes: false,
      enterManuals: false,
      enterLimits: false,
      substituteValues: false,
      ackEvents: false,
      ackAlarms: false,
      disableAlarms: false,
      group1List: [],
      group1CommandList: [],
      displayList: [],
      maxSessionDays: 0.0,
    }

    for (let i = 0; i < user.roles.length; i++) {
      authorities.push(user.roles[i].name)
      if ('isAdmin' in user.roles[i])
        rights.isAdmin = rights.isAdmin || user.roles[i].isAdmin
      if ('changePassword' in user.roles[i])
        rights.changePassword =
          rights.changePassword || user.roles[i].changePassword
      if ('sendCommands' in user.roles[i])
        rights.sendCommands = rights.sendCommands || user.roles[i].sendCommands
      if ('enterAnnotations' in user.roles[i])
        rights.enterAnnotations =
          rights.enterAnnotations || user.roles[i].enterAnnotations
      if ('enterNotes' in user.roles[i])
        rights.enterNotes = rights.enterNotes || user.roles[i].enterNotes
      if ('enterManuals' in user.roles[i])
        rights.enterManuals = rights.enterManuals || user.roles[i].enterManuals
      if ('enterLimits' in user.roles[i])
        rights.enterLimits = rights.enterLimits || user.roles[i].enterLimits
      if ('substituteValues' in user.roles[i])
        rights.substituteValues =
          rights.substituteValues || user.roles[i].substituteValues
      if ('ackEvents' in user.roles[i])
        rights.ackEvents = rights.ackEvents || user.roles[i].ackEvents
      if ('ackAlarms' in user.roles[i])
        rights.ackAlarms = rights.ackAlarms || user.roles[i].ackAlarms
      if ('disableAlarms' in user.roles[i])
        rights.disableAlarms =
          rights.disableAlarms || user.roles[i].disableAlarms
      if ('group1List' in user.roles[i])
        rights.group1List =
          (
            i > 0 &&
            (rights.group1List.length === 0 ||
              user.roles[i].group1List.length === 0)
          ) ?
            []
          : rights.group1List.concat(user.roles[i].group1List)
      if ('group1CommandList' in user.roles[i])
        rights.group1CommandList = rights.group1CommandList.concat(
          user.roles[i].group1CommandList
        )
      if ('displayList' in user.roles[i])
        rights.displayList = rights.displayList.concat(
          user.roles[i].displayList
        )
      if ('maxSessionDays' in user.roles[i])
        if (user.roles[i].maxSessionDays > rights.maxSessionDays)
          rights.maxSessionDays = user.roles[i].maxSessionDays
    }

    const token = jwt.sign(
      { id: user.id, username: user.username, rights: rights },
      config.secret,
      {
        expiresIn: rights.maxSessionDays * 86400, // days*24 hours
      }
    )

    // register user action
    registerUserAction(req, 'signin')

    // return the access token in a cookie unaccessible to javascript (http only)
    // also return a cookie with plain user data accessible to the client side scripts
    res
      .status(200)
      .cookie('x-access-token', token, {
        httpOnly: true,
        secure: false,
        maxAge: (1 + rights.maxSessionDays) * 86400 * 1000,
      })
      .cookie(
        'json-scada-user',
        JSON.stringify({
          id: user._id,
          username: user.username,
          email: user.email,
          isLDAPUser: user.isLDAPUser,
          roles: authorities,
          rights: rights,
        }),
        {
          httpOnly: false,
          secure: false,
          maxAge: (1 + 2 * rights.maxSessionDays) * 86400 * 1000,
        }
      )
      .send({ ok: true, message: 'Signed In' })
  } catch (err) {
    if (err) {
      Log.log(err)
      res.status(200).send({ ok: false, message: err })
      return
    }
  }
}

// Sign out: eliminate the cookie with access token
exports.signout = (req, res) => {
  registerUserAction(req, 'signout')

  return res
    .status(200)
    .cookie('x-access-token', null, {
      httpOnly: true,
      secure: false,
      maxAge: 0,
    })
    .send({
      ok: true,
      message: 'Signed Out!',
    })
}

// check and decoded token
checkToken = (req) => {
  let res = false
  let token = req.headers['x-access-token'] || req.cookies['x-access-token']
  if (!token) {
    return res
  }
  jwt.verify(token, config.secret, (err, decoded) => {
    if (err) {
      return res
    }
    res = decoded
  })
  return res
}
// Modify password change to handle LDAP users
exports.changePassword = async (req, res) => {
  Log.log('User request for password change.')
  try {
    let user = await User.findOne({ username: req.body.username }).exec()
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

    let ck = checkToken(req)
    if (
      ck === false ||
      ck?.username !== req.body.username ||
      !ck?.rights?.changePassword
    ) {
      res.status(200).send({ error: "Can't change password!" })
      Log.log("Can't change password!")
      return
    }
    if (
      !('currentPassword' in req.body) ||
      req.body.currentPassword === '' ||
      req.body.currentPassword === null
    ) {
      res.status(200).send({ error: 'Invalid current password!' })
      Log.log('Invalid current password!')
      return
    }

    const passwordIsValid = bcrypt.compareSync(
      req.body.currentPassword,
      user.password
    )
    if (!passwordIsValid) {
      res.status(200).send({ error: 'Wrong current password!' })
      Log.log('Wrong current password!')
      return
    }

    if (
      !('newPassword' in req.body) ||
      req.body.newPassword === '' ||
      req.body.newPassword === null
    ) {
      res.status(200).send({ error: 'Invalid new password!' })
      Log.log('Invalid new password!')
      return
    }
    user.password = bcrypt.hashSync(req.body.newPassword, 8)
    await user.save()
    registerUserAction(req, 'changePassword')
    res.status(200).send({})
    Log.log('Password changed!')
    delete req.body['currentPassword']
    delete req.body['newPassword']
    registerUserAction(req, 'restartProcesses')
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
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
    const cmd = spawnPlatformScript('exportProject', [project.fileName])
    cmd.on('close', (code) => {
      Log.log(`child process exited with code ${code}`)
      // basename is a further guard even though the name is already validated.
      const filePath = path.join(
        PROJECT_TMP_DIR,
        path.basename(project.fileName)
      )
      if (!fs.existsSync(filePath) || code != 0) {
        Log.log('Project file not found! ' + filePath)
        res.status(200).send({ error: 'Project file not found! ' + filePath })
        return
      }
      registerUserAction(req, 'exportProject')
      res.download(filePath)
    })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err.message || String(err) })
  }
}

// import project file: download, extract zip project file, import collections and move some files
exports.importProject = async (req, res) => {
  Log.log('Import project')
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

    // Fixed extraction directory, not influenced by the request.
    const projectPath = PROJECT_TMP_DIR
    if (!fs.existsSync(projectPath))
      fs.mkdirSync(projectPath, { recursive: true, mode: 0o700 })

    const zipPath = path.join(projectPath, safeName)
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
    for (const entry of zip.getEntries()) {
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

    zip.extractAllTo(projectPath, true)
    Log.log('Files extracted to: ' + projectPath)

    // Fixed script, no shell, no arguments taken from the request.
    const cmd = spawnPlatformScript('importProject')
    cmd.on('close', (code) => {
      Log.log(`child process exited with code ${code}`)
      registerUserAction(req, 'importProject')
      res.status(200).send({ error: false })
      return
    })
  } catch (err) {
    Log.log(err)
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

// placeholder for future use (sanitize database, fill up missing properties, change data types, etc.)
exports.sanitizeDatabase = async function (req, res) {
  res.status(200).send({ error: false })
}

// enqueue use action for later insertion to mongodb
function registerUserAction(req, actionName) {
  let body = {}
  Object.assign(body, req.body)

  let ck = checkToken(req)
  if (ck !== false) {
    Log.log(actionName + ' - ' + ck?.username)
    delete body['password']
    // register user action
    UserActionsQueue.enqueue({
      username: ck?.username,
      properties: body,
      action: actionName,
      timeTag: new Date(),
    })
  } else {
    Log.log(actionName + ' - ' + req.body?.username)
    delete body['password']
    // register user action
    UserActionsQueue.enqueue({
      username: req.body?.username,
      properties: body,
      action: actionName,
      timeTag: new Date(),
    })
  }
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
