'use strict'

/*
 * A process that watches for raw data updates from protocols using a MongoDB change stream.
 * Convert raw values and update realtime values and statuses.
 * {json:scada} - Copyright (c) 2020-2021 - Ricardo L. Olsen
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

const APP_NAME = 'TELEGRAF-LISTENER'
const APP_MSG = '{json:scada} - Telegraf UDP JSON Client Driver'
const VERSION = '0.1.1'
let ProcessActive = false // for redundancy control
let jsConfigFile = '../../conf/json-scada.json'
let UdpBindPort = process.env.JS_TELEGRAF_LISTENER_BIND_PORT || 51920
let UdpBindAddress = process.env.JS_TELEGRAF_LISTENER_BIND_ADDRESS || '0.0.0.0'
let RestrictIPOrigins = []
const LogLevelMin = 0,
  LogLevelNormal = 1,
  LogLevelDetailed = 2,
  LogLevelDebug = 3

const fs = require('fs')
const mongo = require('mongodb')
const MongoClient = require('mongodb').MongoClient
const Queue = require('queue-fifo')
const { setInterval } = require('timers')
const dgram = require('dgram')
const server = dgram.createSocket('udp4')
const grpSep = '~'

let ListCreatedTags = []
let ValuesQueue = new Queue() // queue of values to update

const args = process.argv.slice(2)
var inst = null
if (args.length > 0) inst = parseInt(args[0])
const Instance = inst || process.env.JS_TELEGRAF_LISTENER_INSTANCE || 1
let ConnectionNumber = 0
let AutoCreateTags = true
const AutoKeyMultiplier = 100000 // should be more than estimated maximum points on a connection
let AutoKeyId = 0

var logLevel = null
if (args.length > 1) logLevel = parseInt(args[1])
const LogLevel = logLevel || process.env.JS_TELEGRAF_LISTENER_LOGLEVEL || 1

var confFile = null
if (args.length > 2) confFile = args[2]
jsConfigFile = confFile || process.env.JS_CONFIG_FILE || jsConfigFile

console.log(APP_MSG + ' Version ' + VERSION)
console.log('Instance: ' + Instance)
console.log('Log level: ' + LogLevel)
console.log('Config File: ' + jsConfigFile)

if (!fs.existsSync(jsConfigFile)) {
  console.log('Error: config file not found!')
  process.exit()
}

const RealtimeDataCollectionName = 'realtimeData'
const ProtocolDriverInstancesCollectionName = 'protocolDriverInstances'
const ProtocolConnectionsCollectionName = 'protocolConnections'

const notEmpty = function (obj) {
  if (typeof obj === 'string' && obj.trim().length > 0) return true
  return false
}

const addGrpIfNotEmpty = function (obj) {
  if (notEmpty(obj)) return obj + grpSep
  return ''
}

server.on('error', err => {
  console.log(`server error:\n${err.stack}`)
  server.close()
  process.exit(1)
})

server.on('listening', () => {
  const address = server.address()
  if (LogLevel > LogLevelMin)
    console.log(`server listening ${address.address}:${address.port}`)
})

server.on('message', (msg, rinfo) => {

  if (RestrictIPOrigins.length>0 && !RestrictIPOrigins.includes(rinfo.address)){
    if (LogLevel >= LogLevelDetailed)
      console.log("Message - Source of message not allowed, discarding message from " + rinfo.address)
    return
  }

  let data = {}

  try {
    data = JSON.parse(msg)
    if (LogLevel >= LogLevelDebug)
      console.log(
        'Message - ' +
          JSON.stringify(data) +
          ` - from ${rinfo.address}:${rinfo.port}`
      )
    if (ProcessActive)
      processMessageJSON(data)
  } catch (e) {
    console.log(e)
  }
})

const processMessageJSON = function (data) {
  let grouping = '',
    group1 = '',
    group2 = '',
    group3 = '',
    ungroupedDescription = ''

  // add group1 or measurement name
  if (notEmpty(data.tags?.group1)) {
    grouping += addGrpIfNotEmpty(data.tags.group1)
    group1 = data.tags.group1
  } else {
    grouping += addGrpIfNotEmpty(data?.name)
    group1 = data?.name
  }

  // add group2 or topic or and host name as group2
  if (notEmpty(data.tags?.group2)) {
    grouping += addGrpIfNotEmpty(data.tags.group2)
    group2 = data.tags.group2
  } else {
    // grouping += addGrpIfNotEmpty(data.tags?.objectname)
    if (notEmpty(data.tags?.topic)) {
      let pos = data.tags.topic.lastIndexOf('/')
      if (pos === -1) {
        group2 = data.tags.topic
        ungroupedDescription = data.tags.topic
      } else {
        group2 = data.tags.topic.substring(0, pos)
        ungroupedDescription = data.tags.topic.substring(pos + 1)
      }
      grouping += group2 + grpSep
    } else {
      if (notEmpty(data.tags?.host)) {
        grouping += addGrpIfNotEmpty(data.tags.host)
        group2 = data.tags.host
      }
    }
  }

  if (notEmpty(data.tags?.group3)) {
    group3 = data.tags.group3
  }

  // add group3 if exists
  grouping += addGrpIfNotEmpty(data.tags?.group3)

  // add other tags to grouping
  for (var [key, val] of Object.entries(data.tags)) {
    if (
      ![
        'instance',
        // 'objectname',
        'host',
        'hostname',
        'node_type',
        'rs_name',
        'topic',
        'group1',
        'group2',
        'group3'
      ].includes(key)
    )
      if (val !== '' && grouping.indexOf(val)===-1) {
        grouping += `${val}${grpSep}`
        // ungroupedDescription += `${value}${grpSep}`
      }
  }

  // add instance if exists
  if (data.tags?.instance != '') {
    grouping += addGrpIfNotEmpty(data.tags?.instance)
  }

  let isOpc = false
  let invalid = false
  if ('Quality' in data.fields) isOpc = true
  for (var [key, value] of Object.entries(data.fields)) {
    let tag
    if (key === 'Quality') {
      if (value.toString().indexOf('OK') === -1) invalid = true
    } else {
      if (key === 'value') {
        // remove the ~ at the end
        if (ungroupedDescription === '') tag = grouping.replace(/~\s*$/, '')
        else tag = grouping + ungroupedDescription
      } else {
        tag = `${grouping}${key}`
        ungroupedDescription = key
      }

      let entry = {
        tag: tag,
        value: value,
        group1: group1,
        group2: group2,
        group3: group3,
        description: tag,
        ungroupedDescription: ungroupedDescription,
        invalidAtSource: invalid,
        timeTagAtSource: new Date(1000 * data.timestamp)
      }
      if (LogLevel >= LogLevelDebug)
        console.log('Tag - ' + JSON.stringify(entry))
      ValuesQueue.enqueue(entry)
    }
  }
}

const rtData = function (measurement) {
  AutoKeyId++

  return {
    _id: new mongo.Double(AutoKeyId),
    protocolSourceASDU: '',
    protocolSourceCommonAddress: '',
    protocolSourceConnectionNumber: new mongo.Double(ConnectionNumber),
    protocolSourceObjectAddress: measurement?.tag,
    alarmState: new mongo.Double(-1.0),
    description: measurement?.description,
    ungroupedDescription: measurement?.ungroupedDescription,
    group1: measurement?.group1,
    group2: measurement?.group2,
    group3: measurement?.group3,
    stateTextFalse: '',
    stateTextTrue: '',
    eventTextFalse: '',
    eventTextTrue: '',
    origin: 'supervised',
    tag: measurement.tag,
    type: (typeof measurement.value === 'number' && !isNaN(parseFloat(measurement.value)))? 'analog':'string',
    value: new mongo.Double(measurement.value),
    valueString: measurement.value.toString(),
    alarmDisabled: false,
    alerted: false,
    alarmed: false,
    alertedState: '',
    annotation: '',
    commandBlocked: false,
    commandOfSupervised: new mongo.Double(0.0),
    commissioningRemarks: 'Auto created by ' + APP_NAME,
    formula: new mongo.Double(0.0),
    frozen: false,
    frozenDetectTimeout: new mongo.Double(0.0),
    hiLimit: new mongo.Double(Number.MAX_VALUE),
    hihiLimit: new mongo.Double(Number.MAX_VALUE),
    hihihiLimit: new mongo.Double(Number.MAX_VALUE),
    historianDeadBand: new mongo.Double(0.0),
    historianPeriod: new mongo.Double(0.0),
    hysteresis: new mongo.Double(0.0),
    invalid: measurement?.invalidAtSource ? true : false,
    invalidDetectTimeout: new mongo.Double(60000.0),
    isEvent: false,
    kconv1: new mongo.Double(1.0),
    kconv2: new mongo.Double(0.0),
    location: null,
    loLimit: new mongo.Double(-Number.MAX_VALUE),
    loloLimit: new mongo.Double(-Number.MAX_VALUE),
    lololoLimit: new mongo.Double(-Number.MAX_VALUE),
    notes: '',
    overflow: false,
    parcels: null,
    priority: new mongo.Double(0.0),
    protocolDestinations: null,
    sourceDataUpdate: null,
    supervisedOfCommand: new mongo.Double(0.0),
    timeTag: null,
    timeTagAlarm: null,
    timeTagAtSource: measurement.timeTagAtSource,
    timeTagAtSourceOk: false,
    transient: false,
    unit: '',
    updatesCnt: new mongo.Double(0.0),
    valueDefault: new mongo.Double(0.0),
    zeroDeadband: new mongo.Double(0.0)
  }
}

let rawFileContents = fs.readFileSync(jsConfigFile)
let jsConfig = JSON.parse(rawFileContents)
if (
  typeof jsConfig.mongoConnectionString != 'string' ||
  jsConfig.mongoConnectionString === ''
) {
  console.log('Error reading config file.')
  process.exit()
}

if (LogLevel > LogLevelMin) console.log('Connecting to MongoDB server...')
;(async () => {
  let collection = null

  setInterval(async function () {
    let cnt = 0
    if (clientMongo && collection)
      while (!ValuesQueue.isEmpty()) {
        let data = ValuesQueue.peek()
        // const db = clientMongo.db(jsConfig.mongoDatabaseName)

        // if not sure tag is created, try to find, if not found create it
        if (AutoCreateTags)
          if (!ListCreatedTags.includes(data.tag)) {
            // possibly not created tag, must check
            let res = await collection
              .find({
                protocolSourceConnectionNumber: ConnectionNumber,
                protocolSourceObjectAddress: data.tag
              })
              .toArray()

            if ('length' in res && res.length === 0) {
              // not found, then create
              let newTag = rtData(data)
              if (logLevel >= LogLevelDetailed)
                console.log('Tag not found, will create: ' + data.tag)
              let resIns = await collection.insertOne(newTag)
              if (resIns.insertedCount === 1) ListCreatedTags.push(data.tag)
            } else {
              // found (already exists, no need to create), just list as created
              ListCreatedTags.push(data.tag)
            }
          }

        // now update tag

        // try to parse value as JSON
        let valueJson = null
        try {
          valueJson = JSON.parse(data.value)
        } catch (e) {}

        if (LogLevel >= LogLevelDetailed)
          console.log(
            'Update - ' +
              data.timeTagAtSource +
              ' : ' +
              data.tag +
              ' : ' +
              data.value
          )

        let updTag = {
          valueAtSource: parseFloat(data.value),
          valueStringAtSource: data.value.toString(),
          valueJsonAtSource: valueJson,
          asduAtSource: '',
          causeOfTransmissionAtSource: '3',
          timeTagAtSource: data.timeTagAtSource,
          timeTagAtSourceOk: false, // signal that it is not really from field time
          timeTag: new Date(),
          originator: APP_NAME + "|" + ConnectionNumber,
          notTopicalAtSource: false,
          invalidAtSource: data.invalidAtSource,
          overflowAtSource: false,
          blockedAtSource: false,
          substitutedAtSource: false
        }
        collection.updateOne(
          {
            protocolSourceConnectionNumber: ConnectionNumber,
            protocolSourceObjectAddress: data.tag
          },
          { $set: { sourceDataUpdate: updTag } }
        )

        ValuesQueue.dequeue()
        cnt++
      }
    if (cnt)
      if (LogLevel >= LogLevelNormal) console.log('Mongo - Updates: ' + cnt)
  }, 500)

  let connOptions = {
    useNewUrlParser: true,
    useUnifiedTopology: true,
    appname: APP_NAME + ' Version:' + VERSION + ' Instance:' + Instance,
    poolSize: 20,
    readPreference: MongoClient.READ_PRIMARY
  }

  if (
    typeof jsConfig.tlsCaPemFile === 'string' &&
    jsConfig.tlsCaPemFile.trim() !== ''
  ) {
    jsConfig.tlsClientKeyPassword = jsConfig.tlsClientKeyPassword || ''
    jsConfig.tlsAllowInvalidHostnames =
      jsConfig.tlsAllowInvalidHostnames || false
    jsConfig.tlsAllowChainErrors = jsConfig.tlsAllowChainErrors || false
    jsConfig.tlsInsecure = jsConfig.tlsInsecure || false

    connOptions.tls = true
    connOptions.tlsCAFile = jsConfig.tlsCaPemFile
    connOptions.tlsCertificateKeyFile = jsConfig.tlsClientPemFile
    connOptions.tlsCertificateKeyFilePassword = jsConfig.tlsClientKeyPassword
    connOptions.tlsAllowInvalidHostnames = jsConfig.tlsAllowInvalidHostnames
    connOptions.tlsInsecure = jsConfig.tlsInsecure
  }

  let clientMongo = null
  let redundancyIntervalHandle = null
  while (true) {
    if (clientMongo === null)
      await MongoClient.connect(
        jsConfig.mongoConnectionString,
        connOptions
      ).then(async client => {
        clientMongo = client

        if (LogLevel > LogLevelMin)
          console.log('Connected correctly to MongoDB server')

        // specify db and collections
        const db = client.db(jsConfig.mongoDatabaseName)
        collection = db.collection(RealtimeDataCollectionName)

        // find the connection number, if not found abort (only one connection per instance is allowed for this protocol)
        db.collection(ProtocolConnectionsCollectionName)
          .find({
            protocolDriver: APP_NAME,
            protocolDriverInstanceNumber: Instance
          })
          .toArray(async function (err, results) {
            if (err) console.log(err)
            else if (results) {
              if (results.length == 0) {
                console.log('No protocol connection found!')
                process.exit(1)
              } else {
                if (!('protocolConnectionNumber' in results[0])) {
                  console.log('No protocol connection found on record!')
                  process.exit(2)
                }
                if (results[0].enabled === false) {
                  console.log('Connection disabled, exiting! (connection:' + results[0].protocolConnectionNumber + ")")
                  process.exit(3)
                }
                if ('autoCreateTags' in results[0]) {
                  AutoCreateTags = results[0].autoCreateTags ? true : false
                }
                if ('ipAddressLocalBind' in results[0]) {
                  if (results[0].ipAddressLocalBind.trim() !== '') {
                    let aux = results[0].ipAddressLocalBind.split(':')
                    UdpBindAddress = aux[0]
                    if (aux.length > 1 && !isNaN(parseInt(aux[1])))
                      UdpBindPort = parseInt(aux[1])
                  }
                }
                if ('ipAddresses' in results[0]) {
                  RestrictIPOrigins = results[0].ipAddresses
                }
                console.log('Binding to ' + UdpBindAddress + ':' + UdpBindPort)
                server.bind(UdpBindPort, UdpBindAddress)

                ConnectionNumber = results[0]?.protocolConnectionNumber
                console.log('Connection - ' + ConnectionNumber)

                // find biggest point key (_id) on range and ajdust automatic key
                AutoKeyId = ConnectionNumber * AutoKeyMultiplier
                let resLastKey = await collection
                  .find({
                    _id: {
                      $gt: AutoKeyId,
                      $lt: (ConnectionNumber + 1) * AutoKeyMultiplier
                    }
                  })
                  .sort({ _id: -1 })
                  .limit(1)
                  .toArray()
                if (resLastKey.length > 0 && '_id' in resLastKey[0]) {
                  if (parseInt(resLastKey[0]._id) >= AutoKeyId)
                    AutoKeyId = parseInt(resLastKey[0]._id)
                }
              }
            }
          })

        let lastActiveNodeKeepAliveTimeTag = null
        let countKeepAliveNotUpdated = 0
        let countKeepAliveUpdatesLimit = 4
        async function ProcessRedundancy () {
          if (!clientMongo) return

          if (LogLevel >= LogLevelNormal)
            console.log('Redundancy - Process Active: ' + ProcessActive)

          // look for process instance entry, if not found create a new entry
          db.collection(ProtocolDriverInstancesCollectionName)
            .find({
              protocolDriver: APP_NAME,
              protocolDriverInstanceNumber: Instance
            })
            .toArray(function (err, results) {
              if (err) console.log(err)
              else if (results) {
                if (results.length === 0) {
                  // not found, then create
                  ProcessActive = true
                  console.log(
                    'Redundancy - Instance config not found, creating one...'
                  )
                  db.collection(
                    ProtocolDriverInstancesCollectionName
                  ).insertOne({
                    protocolDriver: APP_NAME,
                    protocolDriverInstanceNumber: new mongo.Double(1),
                    enabled: true,
                    logLevel: new mongo.Double(1),
                    nodeNames: [],
                    activeNodeName: jsConfig.nodeName,
                    activeNodeKeepAliveTimeTag: new Date()
                  })
                } else {
                  // check for disabled or node not allowed
                  let instance = results[0]

                  let instKeepAliveTimeTag = null

                  if ('activeNodeKeepAliveTimeTag' in instance)
                    instKeepAliveTimeTag = instance.activeNodeKeepAliveTimeTag.toISOString()

                  if (instance?.enabled === false) {
                    console.log('Redundancy - Instance disabled, exiting...')
                    process.exit()
                  }
                  if (
                    instance?.nodeNames !== null &&
                    instance.nodeNames.length > 0
                  ) {
                    if (!instance.nodeNames.includes(jsConfig.nodeName)) {
                      console.log(
                        'Redundancy - Node name not allowed, exiting...'
                      )
                      process.exit()
                    }
                  }
                  if (instance?.activeNodeName === jsConfig.nodeName) {
                    if (!ProcessActive)
                      console.log('Redundancy - Node activated!')
                    countKeepAliveNotUpdated = 0
                    ProcessActive = true
                  } else {
                    // other node active
                    if (ProcessActive) {
                      console.log('Redundancy - Node deactivated!')
                      countKeepAliveNotUpdated = 0
                    }
                    ProcessActive = false
                    if (
                      lastActiveNodeKeepAliveTimeTag === instKeepAliveTimeTag
                    ) {
                      countKeepAliveNotUpdated++
                      console.log(
                        'Redundancy - Keep-alive from active node not updated. ' +
                          countKeepAliveNotUpdated
                      )
                    } else {
                      countKeepAliveNotUpdated = 0
                      console.log(
                        'Redundancy - Keep-alive updated by active node. Staying inactive.'
                      )
                    }
                    lastActiveNodeKeepAliveTimeTag = instKeepAliveTimeTag
                    if (countKeepAliveNotUpdated > countKeepAliveUpdatesLimit) {
                      // cnt exceeded, be active
                      countKeepAliveNotUpdated = 0
                      console.log('Redundancy - Node activated!')
                      ProcessActive = true
                    }
                  }

                  if (ProcessActive) {
                    // process active, then update keep alive
                    db.collection(
                      ProtocolDriverInstancesCollectionName
                    ).updateOne(
                      {
                        protocolDriver: APP_NAME,
                        protocolDriverInstanceNumber: new mongo.Double(Instance)
                      },
                      {
                        $set: {
                          activeNodeName: jsConfig.nodeName,
                          activeNodeKeepAliveTimeTag: new Date(),
                          softwareVersion: VERSION,
                          stats: {}
                        }
                      }
                    )
                  }
                }
              }
            })
        }

        // check and update redundancy control
        ProcessRedundancy()
        clearInterval(redundancyIntervalHandle)
        redundancyIntervalHandle = setInterval(ProcessRedundancy, 5000)
      })

    // wait 5 seconds
    await new Promise(resolve => setTimeout(resolve, 5000))

    // detect connection problems, if error will null the client to later reconnect
    if (clientMongo === undefined) {
      console.log('Disconnected Mongodb!')
      clientMongo = null
    }
    if (clientMongo)
      if (!clientMongo.isConnected()) {
        // not anymore connected, will retry
        console.log('Disconnected Mongodb!')
        clientMongo.close()
        clientMongo = null
      }
  }
})()
