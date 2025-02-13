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
const { Client } = require('ldapts')
const User = db.user
const Role = db.role
const Tag = db.tag
const ProtocolDriverInstance = db.protocolDriverInstance
const ProtocolConnection = db.protocolConnection
const UserAction = db.userAction

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
      'OPC-UA_SERVER',
      'ICCP_SERVER',
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
        case 'DNP3_SERVER':
          req.body.ipAddressLocalBind = '0.0.0.0:20000'
          break
        case 'I104M':
          req.body.ipAddressLocalBind = '0.0.0.0:8099'
          break
        case 'TELEGRAF-LISTENER':
          req.body.ipAddressLocalBind = '0.0.0.0:51920'
          break
        case 'ICCP_SERVER':
          req.body.ipAddressLocalBind = '0.0.0.0:102'
          break
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
      'OPC-UA_SERVER',
      'ICCP',
      'ICCP_SERVER',
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
      'MQTT-SPARKPLUG-B',
      'IEC61850',
      'PLC4X',
      'OPC-DA',
      'ICCP',
      'DNP3_SERVER',
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('autoCreateTags' in req.body)) {
      req.body.autoCreateTags = true
    }
  }

  if (
    [
      'MQTT-SPARKPLUG-B',
      'OPC-UA_SERVER',
      'IEC61850',
      'PLC4X',
      'OPC-UA',
      'OPC-DA',
      'OPC-DA_SERVER',
      'ICCP',
      'ICCP_SERVER',
      'DNP3_SERVER',
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

  if (['MQTT-SPARKPLUG-B', 'OPC-DA'].includes(req?.body?.protocolDriver)) {
    if (!('username' in req.body)) {
      req.body.username = ''
    }
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
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('useSecurity' in req.body)) {
      req.body.useSecurity = false
    }
  }

  if (
    ['OPC-UA', 'OPC-UA_SERVER', 'OPC-DA', 'ICCP', 'ICCP_SERVER'].includes(
      req?.body?.protocolDriver
    )
  ) {
    if (!('timeoutMs' in req.body)) {
      req.body.timeoutMs = 10000.0
    }
  }

  if (['ICCP', 'ICCP_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('aeQualifier' in req.body)) {
      req.body.aeQualifier = 12.0
    }
    if (!('localAppTitle' in req.body)) {
      req.body.localAppTitle = '1.1.1.998'
    }
    if (!('localSelectors' in req.body)) {
      req.body.localSelectors = '0 0 0 2 0 2 0 2'
    }
  }

  if (['ICCP'].includes(req?.body?.protocolDriver)) {
    if (!('remoteAppTitle' in req.body)) {
      req.body.remoteAppTitle = '1.1.1.999'
    }
    if (!('remoteSelectors' in req.body)) {
      req.body.remoteSelectors = '0 0 0 1 0 1 0 1'
    }
  }

  if (['OPC-UA'].includes(req?.body?.protocolDriver)) {
    if (!('configFileName' in req.body)) {
      req.body.configFileName = '../conf/Opc.Ua.DefaultClient.Config.xml'
    }
  }
  if (['ICCP', 'ICCP_SERVER'].includes(req?.body?.protocolDriver)) {
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
    ['OPC-UA', 'OPC-DA', 'OPC-DA_SERVER', 'DNP3_SERVER'].includes(
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
  if (['ICCP', 'ICCP_SERVER'].includes(req?.body?.protocolDriver)) {
    if (!('domain' in req.body)) {
      req.body.domain = ''
    }
    if (!('hoursShift' in req.body)) {
      req.body.hoursShift = 0.0
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
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('giInterval' in req.body)) {
      req.body.giInterval = 300
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
      'OPC-DA',
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
    ['IEC60870-5-104', 'IEC60870-5-104_SERVER'].includes(
      req?.body?.protocolDriver
    )
  ) {
    if (!('allowOnlySpecificCertificates' in req.body)) {
      req.body.allowOnlySpecificCertificates = false
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
    ].includes(req?.body?.protocolDriver)
  ) {
    if (!('privateKeyFilePath' in req.body)) {
      req.body.privateKeyFilePath = ''
    }
  }

  if (
    ['DNP3', 'DNP3_SERVER', 'MQTT-SPARKPLUG-B'].includes(
      req?.body?.protocolDriver
    )
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

  try {
    await ProtocolConnection.findOneAndUpdate({ _id: req.body._id }, req.body)
    res.status(200).send({})
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
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

    await ProtocolConnection.findOneAndDelete({ _id: req.body._id })

    res.status(200).send({ error: false })
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

    await ProtocolDriverInstance.findOneAndDelete({ _id: req.body._id })
    res.status(200).send({ error: false })
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
    await ProtocolDriverInstance.findOneAndUpdate(
      { _id: req.body._id },
      req.body
    )
    registerUserAction(req, 'updateProtocolDriverInstance')
    res.status(200).send({ error: false })
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
  Log.log('Update user')
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
          i > 0 &&
          (rights.group1List.length === 0 ||
            user.roles[i].group1List.length === 0)
            ? []
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

// export project file: dump collections and some files to a zip file
exports.exportProject = async (req, res) => {
  Log.log('Save project')
  try {
    let project = req.body.project
    if (!project.fileName || project.fileName.trim() == '')
      project.fileName = 'new_project_' + new Date().getTime() / 1000 + '.zip'
    project.fileName = project.fileName.trim()

    let cmd = ''
    let dir = ''
    if (process.platform === 'win32') {
      cmd = spawn(
        'c:\\json-scada\\platform-windows\\export_project.bat',
        [project.fileName],
        { shell: true }
      )
      dir = 'c:\\json-scada\\tmp\\'
    } else {
      cmd = spawn(
        'sh',
        ['~/json-scada/platform-linux/export_project.sh', project.fileName],
        { shell: true }
      )
      dir = '~/json-scada/tmp/'
    }
    cmd.stdout.on('data', (data) => Log.log(`stdout: ${data}`))
    cmd.stderr.on('data', (data) => Log.log(`stderr: ${data}`))
    cmd.on('close', (code) => {
      Log.log(`child process exited with code ${code}`)
      if (!fs.existsSync(dir + project.fileName) || code != 0) {
        Log.log('Project file not found!')
        res.status(200).send({ error: err })
        return
      }
      registerUserAction(req, 'exportProject')
      res.download(dir + project.fileName)
    })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

// import project file: download, extract zip project file, import collections and move some files
exports.importProject = async (req, res) => {
  Log.log('Import project')
  try {
    const projectFileName = req.body.projectFileName
    if (!req.files || !req.files.projectFileData)
      throw new Error('No project file uploaded!')

    let projectPath = ''
    let importScript = ''
    if (process.platform === 'win32') {
      projectPath = 'c:\\json-scada\\tmp\\'
      importScript = 'c:\\json-scada\\platform-windows\\import_project.bat'
    } else {
      projectPath = '~/json-scada/tmp/'
      importScript = '~/json-scada/platform-linux/import_project.sh'
    }
    if (!fs.existsSync(projectPath)) fs.mkdirSync(projectPath)
    await req.files.projectFileData.mv(projectPath + projectFileName)
    const zip = new AdmZip(projectPath + projectFileName)

    //var zipEntries = zip.getEntries(); // an array of ZipEntry records - add password parameter if entries are password protected
    //zipEntries.forEach(function (zipEntry) {
    //    console.log(zipEntry.toString()); // outputs zip entries information
    //});

    zip.extractAllTo(projectPath, true)
    Log.log('Files extracted to: ' + projectPath)

    const cmd = spawn(
      importScript,
      // [project.fileName],
      { shell: true }
    )
    cmd.stdout.on('data', (data) => Log.log(`stdout: ${data}`))
    cmd.stderr.on('data', (data) => Log.log(`stderr: ${data}`))
    cmd.on('close', (code) => {
      Log.log(`child process exited with code ${code}`)
      registerUserAction(req, 'importProject')
      res.status(200).send({ error: false })
      return
    })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.restartProtocols = async (req, res) => {
  Log.log('restartProtocols')
  try {
    let cmd = ''
    if (process.platform === 'win32') {
      cmd = spawn(
        'cmd',
        ['/c', 'c:\\json-scada\\platform-windows\\restart_protocols.bat'],
        {
          shell: true,
        }
      )
    } else {
      cmd = spawn('sh', ['~/json-scada/platform-linux/restart_protocols.sh'], {
        shell: true,
      })
    }
    cmd.stdout.on('data', (data) => Log.log(`stdout: ${data}`))
    cmd.stderr.on('data', (data) => Log.log(`stderr: ${data}`))
    cmd.on('close', (code) => Log.log(`child process exited with code ${code}`))

    registerUserAction(req, 'restartProtocols')
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
  }
}

exports.restartProcesses = async (req, res) => {
  Log.log('restartProcesses')
  try {
    let cmd = ''
    if (process.platform === 'win32') {
      cmd = spawn(
        'cmd',
        ['/c', 'c:\\json-scada\\platform-windows\\restart_services.bat'],
        {
          shell: true,
        }
      )
    } else {
      cmd = spawn('sh', ['~/json-scada/platform-linux/restart_processes.sh'], {
        shell: true,
      })
    }
    cmd.stdout.on('data', (data) => Log.log(`stdout: ${data}`))
    cmd.stderr.on('data', (data) => Log.log(`stderr: ${data}`))
    cmd.on('close', (code) => Log.log(`child process exited with code ${code}`))

    registerUserAction(req, 'restartProcesses')
    res.status(200).send({ error: false })
  } catch (err) {
    Log.log(err)
    res.status(200).send({ error: err })
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

  Log.log('LDAP - Server: ' + config.ldap.url)

  const tlsOptions = null
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

      // Search for user
      const searchFilter = config.ldap.searchFilter.replace(
        '{{username}}',
        username
      )
      const { searchEntries } = await client.search(config.ldap.searchBase, {
        filter: searchFilter,
        attributes: Object.values(config.ldap.attributes),
      })

      if (searchEntries.length === 0) {
        Log.log('LDAP - User not found: ' + username)
        await client.unbind()
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
      userDN =
        config.ldap.attributes.username +
        '=' +
        username +
        ',' +
        config.ldap.searchBase
    }

    try {
      // Try to bind with user credentials to verify password
      await client.bind(userDN, password)
      Log.log('LDAP - Ok for userDN: ' + userDN)
    } catch (err) {
      Log.log('LDAP - Auth error for userDN: ' + userDN)

      userDN =
        config.ldap.attributes.displayName +
        '=' +
        username +
        ',' +
        config.ldap.searchBase

      await client.bind(userDN, password)
      Log.log('LDAP - Ok for userDN: ' + userDN)
    }

    if (!userEntry) {
      // Search for user
      const searchFilter = config.ldap.searchFilter.replaceAll(
        '{{username}}',
        username
      )
      const { searchEntries } = await client.search(config.ldap.searchBase, {
        filter: searchFilter,
        attributes: Object.values(config.ldap.attributes),
      })

      if (searchEntries.length === 0) {
        Log.log('LDAP - User not found: ' + searchFilter)
        await client.unbind()
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
        const filter =
          '(&(|(objectClass=groupOfUniqueNames)(objectClass=groupOfNames)(objectClass=group))(|(uniqueMember=' +
          userDN +
          ')(member=' +
          userDN +
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
    await client.unbind()

    return user
  } catch (error) {
    Log.log('LDAP - error for userDN ' + userDN + ': ' + error)
    return null
  }
}
