/*
 * Telegraf listener driver.
 *
 * {json:scada} - Copyright (c) 2020-2024 - Ricardo L. Olsen
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

const APP_NAME = 'TELEGRAF-LISTENER'
const APP_MSG = '{json:scada} - Telegraf UDP JSON Client Driver'
const VERSION = '0.1.5'
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
const { MongoClient, ReadPreference, Double } = require('mongodb')
const Queue = require('queue-fifo')
const dgram = require('dgram')
const serverUdpSocket = dgram.createSocket({ type: 'udp4' })
let bindCount = 0
const grpSep = '~'

process.on('uncaughtException', (err) =>
  console.log('Uncaught Exception: ' + (err?.stack || err))
)

let ListCreatedTags = []
let ValuesQueue = new Queue() // queue of values to update

const args = process.argv.slice(2)
var inst = null
if (args.length > 0) inst = parseInt(args[0])
const Instance = inst || process.env.JS_TELEGRAF_LISTENER_INSTANCE || 1
let ConnectionNumber = 0
// time of the last accepted datagram; a listener counts as connected while
// data keeps arriving (stats.isConnected, for communication status tags)
let LastMessageTime = 0
const ConnectedWindowMs = 60000
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

serverUdpSocket.on('error', (err) => {
  console.log(`server error:\n${err.stack}`)
  serverUdpSocket.close()
  process.exit(1)
})

serverUdpSocket.on('listening', () => {
  const address = serverUdpSocket.address()
  if (LogLevel > LogLevelMin)
    console.log(`server listening ${address.address}:${address.port}`)
})

serverUdpSocket.on('message', (msg, rinfo) => {
  if (
    RestrictIPOrigins.length > 0 &&
    !RestrictIPOrigins.includes(rinfo.address)
  ) {
    if (LogLevel >= LogLevelDetailed)
      console.log(
        'Message - Source of message not allowed, discarding message from ' +
          rinfo.address
      )
    return
  }
  LastMessageTime = Date.now()

  let data = {}

  try {
    data = JSON.parse(msg)
    if (LogLevel >= LogLevelDebug)
      console.log(
        'Message - ' +
          JSON.stringify(data) +
          ` - from ${rinfo.address}:${rinfo.port}`
      )
    if (ProcessActive) processMessageJSON(data)
  } catch (e) {
    console.log(e)
  }
})

const processMessageJSON = function (data) {
  if (data === null || typeof data !== 'object') return
  const tags = data.tags !== null && typeof data.tags === 'object' ? data.tags : {}
  const fields =
    data.fields !== null && typeof data.fields === 'object' ? data.fields : {}

  let grouping = '',
    group1 = '',
    group2 = '',
    group3 = '',
    ungroupedDescription = ''

  // add group1 or measurement name
  if (notEmpty(tags.group1)) {
    grouping += addGrpIfNotEmpty(tags.group1)
    group1 = tags.group1
  } else {
    grouping += addGrpIfNotEmpty(data?.name)
    group1 = data?.name
  }

  // add group2 or topic or and host name as group2
  if (notEmpty(tags.group2)) {
    grouping += addGrpIfNotEmpty(tags.group2)
    group2 = tags.group2
  } else {
    // grouping += addGrpIfNotEmpty(tags.objectname)
    if (notEmpty(tags.topic)) {
      let pos = tags.topic.lastIndexOf('/')
      if (pos === -1) {
        group2 = tags.topic
        ungroupedDescription = tags.topic
      } else {
        group2 = tags.topic.substring(0, pos)
        ungroupedDescription = tags.topic.substring(pos + 1)
      }
      grouping += group2 + grpSep
    } else {
      if (notEmpty(tags.host)) {
        grouping += addGrpIfNotEmpty(tags.host)
        group2 = tags.host
      }
    }
  }

  if (notEmpty(tags.group3)) {
    group3 = tags.group3
  }

  // add group3 if exists
  grouping += addGrpIfNotEmpty(tags.group3)

  // add other tags to grouping, when group2 or group3 are not defined
  if (!tags.group2 || !tags.group3)
    for (var [key, val] of Object.entries(tags)) {
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
          'group3',
        ].includes(key)
      )
        if (val !== '' && !grouping.split(grpSep).includes(String(val))) {
          grouping += `${val}${grpSep}`
          // ungroupedDescription += `${value}${grpSep}`
        }
    }

  // add instance if exists
  grouping += addGrpIfNotEmpty(tags.instance)

  // source timestamp in seconds, fall back to current time when absent/invalid
  let timeTagAtSource = new Date(1000 * data.timestamp)
  if (isNaN(timeTagAtSource.getTime())) timeTagAtSource = new Date()

  // scan all fields for quality before enqueuing any value,
  // so the invalid flag does not depend on field order
  let invalid = false
  for (var [key, value] of Object.entries(fields)) {
    if (key === 'Quality' && value !== null && value !== undefined)
      if (value.toString().indexOf('OK') === -1) invalid = true
  }

  for (var [key, value] of Object.entries(fields)) {
    if (key === 'Quality') continue
    if (value === null || value === undefined) continue
    let tag
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
      timeTagAtSource: timeTagAtSource,
    }
    if (LogLevel >= LogLevelDebug) console.log('Tag - ' + JSON.stringify(entry))
    ValuesQueue.enqueue(entry)
  }
}

const rtData = function (measurement) {
  AutoKeyId++

  return {
    _id: new Double(AutoKeyId),
    protocolSourceASDU: '',
    protocolSourceCommonAddress: '',
    protocolSourceConnectionNumber: new Double(ConnectionNumber),
    protocolSourceObjectAddress: measurement?.tag,
    protocolSourceCommandDuration: new Double(0),
    protocolSourceCommandUseSBO: false,
    alarmState: new Double(-1.0),
    alarmRange: new Double(0.0),
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
    type:
      typeof measurement.value === 'boolean'
        ? 'digital'
        : typeof measurement.value === 'number' &&
          !isNaN(parseFloat(measurement.value))
        ? 'analog'
        : 'string',
    value: new Double(
      typeof measurement.value === 'boolean'
        ? measurement.value
          ? 1
          : 0
        : typeof measurement.value === 'number'
        ? measurement.value
        : 0
    ),
    valueString: measurement.value.toString(),
    alarmDisabled: false,
    alerted: false,
    alarmed: false,
    alertState: '',
    annotation: '',
    commandBlocked: false,
    commandOfSupervised: new Double(0.0),
    commissioningRemarks: 'Auto created by ' + APP_NAME,
    formula: new Double(0.0),
    frozen: false,
    frozenDetectTimeout: new Double(0.0),
    hiLimit: new Double(Number.MAX_VALUE),
    hihiLimit: new Double(Number.MAX_VALUE),
    hihihiLimit: new Double(Number.MAX_VALUE),
    historianDeadBand: new Double(0.0),
    historianPeriod: new Double(0.0),
    hysteresis: new Double(0.0),
    invalid: measurement?.invalidAtSource ? true : false,
    invalidDetectTimeout: new Double(60000.0),
    isEvent: false,
    kconv1: new Double(1.0),
    kconv2: new Double(0.0),
    location: null,
    loLimit: new Double(-Number.MAX_VALUE),
    loloLimit: new Double(-Number.MAX_VALUE),
    lololoLimit: new Double(-Number.MAX_VALUE),
    notes: '',
    overflow: false,
    parcels: null,
    priority: new Double(0.0),
    protocolDestinations: null,
    sourceDataUpdate: null,
    substituted: false,
    supervisedOfCommand: new Double(0.0),
    timeTag: null,
    timeTagAlarm: null,
    timeTagAtSource: measurement.timeTagAtSource,
    timeTagAtSourceOk: false,
    transient: false,
    unit: '',
    updatesCnt: new Double(0.0),
    valueDefault: new Double(0.0),
    zeroDeadband: new Double(0.0),
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

;(async () => {
  let collection = null

  let drainInProgress = false // avoids reentrant executions when a cycle takes more than the interval
  setInterval(async function () {
    if (drainInProgress) return
    drainInProgress = true
    try {
      let cnt = 0
      if (clientMongo && collection)
        while (!ValuesQueue.isEmpty()) {
          let data = ValuesQueue.peek()

          // if not sure tag is created, try to find, if not found create it
          if (AutoCreateTags)
            if (!ListCreatedTags.includes(data.tag)) {
              // possibly not created tag, must check
              let res = await collection
                .find({
                  protocolSourceConnectionNumber: ConnectionNumber,
                  protocolSourceObjectAddress: data.tag,
                })
                .toArray()

              if ('length' in res && res.length === 0) {
                // not found, then create
                let newTag = rtData(data)
                if (LogLevel >= LogLevelDetailed)
                  console.log('Tag not found, will create: ' + data.tag)
                try {
                  let resIns = await collection.insertOne(newTag)
                  if (resIns.acknowledged) ListCreatedTags.push(data.tag)
                } catch (e) {
                  console.log('Error on insert: ' + e)
                }
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
            valueAtSource:
              typeof data.value === 'boolean'
                ? data.value
                  ? 1
                  : 0
                : parseFloat(data.value),
            valueStringAtSource: data.value.toString(),
            valueJsonAtSource: valueJson,
            asduAtSource: '',
            causeOfTransmissionAtSource: '3',
            timeTagAtSource: data.timeTagAtSource,
            timeTagAtSourceOk: false, // signal that it is not really from field time
            timeTag: new Date(),
            originator: APP_NAME + '|' + ConnectionNumber,
            notTopicalAtSource: false,
            invalidAtSource: data.invalidAtSource,
            overflowAtSource: false,
            blockedAtSource: false,
            substitutedAtSource: false,
          }
          try {
            await collection.updateOne(
              {
                protocolSourceConnectionNumber: ConnectionNumber,
                protocolSourceObjectAddress: data.tag,
              },
              { $set: { sourceDataUpdate: updTag } }
            )
          } catch (e) {
            // keep entry queued, stop this cycle and retry later
            console.log('Error on update: ' + e)
            break
          }

          ValuesQueue.dequeue()
          cnt++
        }
      if (cnt)
        if (LogLevel >= LogLevelNormal) console.log('Mongo - Updates: ' + cnt)
    } finally {
      drainInProgress = false
    }
  }, 500)

  let connOptions = {
    appname: APP_NAME + ' Version:' + VERSION + ' Instance:' + Instance,
    maxPoolSize: 20,
    readPreference: ReadPreference.PRIMARY,
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
    try {
      if (clientMongo === null) {
        if (LogLevel > LogLevelMin)
          console.log('Connecting to MongoDB server...')
        await MongoClient.connect(
          jsConfig.mongoConnectionString,
          connOptions
        ).then(async (client) => {
          clientMongo = client
          if (LogLevel > LogLevelMin)
            console.log('Connected correctly to MongoDB server')

          // specify db and collections
          const db = client.db(jsConfig.mongoDatabaseName)
          collection = db.collection(RealtimeDataCollectionName)

          // find the connection number, if not found abort (only one connection per instance is allowed for this protocol)
          const results = await db
            .collection(ProtocolConnectionsCollectionName)
            .find({
              protocolDriver: APP_NAME,
              protocolDriverInstanceNumber: Instance,
            })
            .toArray()

          if (!results || results.length === 0) {
            console.log('No protocol connection found!')
            process.exit(1)
          }
          if (!('protocolConnectionNumber' in results[0])) {
            console.log('No protocol connection found on record!')
            process.exit(2)
          }
          if (results[0].enabled === false) {
            console.log(
              'Connection disabled, exiting! (connection:' +
                results[0].protocolConnectionNumber +
                ')'
            )
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
          if (Array.isArray(results[0].ipAddresses)) {
            RestrictIPOrigins = results[0].ipAddresses
          }

          if (bindCount === 0) {
            console.log('Binding to ' + UdpBindAddress + ':' + UdpBindPort)
            serverUdpSocket.bind(UdpBindPort, UdpBindAddress)
            bindCount++
          }

          ConnectionNumber = results[0]?.protocolConnectionNumber
          console.log('Connection - ' + ConnectionNumber)

          // find biggest point key (_id) on range and ajdust automatic key
          AutoKeyId = ConnectionNumber * AutoKeyMultiplier
          let resLastKey = await collection
            .find({
              _id: {
                $gt: AutoKeyId,
                $lt: (ConnectionNumber + 1) * AutoKeyMultiplier,
              },
            })
            .sort({ _id: -1 })
            .limit(1)
            .toArray()
          if (resLastKey.length > 0 && '_id' in resLastKey[0]) {
            if (parseInt(resLastKey[0]._id) >= AutoKeyId)
              AutoKeyId = parseInt(resLastKey[0]._id)
          }

          let lastActiveNodeKeepAliveTimeTag = null
          let countKeepAliveNotUpdated = 0
          let countKeepAliveUpdatesLimit = 4
          async function ProcessRedundancy() {
            if (LogLevel >= LogLevelNormal)
              console.log('Redundancy - Process Active: ' + ProcessActive)

            if (!clientMongo || !HintMongoIsConnected) {
              ProcessActive = false
              return
            }

            try {
              // look for process instance entry, if not found create a new entry
              const results = await db
                .collection(ProtocolDriverInstancesCollectionName)
                .find({
                  protocolDriver: APP_NAME,
                  protocolDriverInstanceNumber: Instance,
                })
                .toArray()

              if (!results || results.length === 0) {
                // not found, then create
                ProcessActive = true
                console.log(
                  'Redundancy - Instance config not found, creating one...'
                )
                await db
                  .collection(ProtocolDriverInstancesCollectionName)
                  .insertOne({
                    protocolDriver: APP_NAME,
                    protocolDriverInstanceNumber: new Double(Instance),
                    enabled: true,
                    logLevel: new Double(1),
                    nodeNames: [],
                    activeNodeName: jsConfig.nodeName,
                    activeNodeKeepAliveTimeTag: new Date(),
                  })
                return
              }

              // check for disabled or node not allowed
              let instance = results[0]

              let instKeepAliveTimeTag = null

              if ('activeNodeKeepAliveTimeTag' in instance)
                instKeepAliveTimeTag =
                  instance.activeNodeKeepAliveTimeTag.toISOString()

              if (instance?.enabled === false) {
                console.log('Redundancy - Instance disabled, exiting...')
                process.exit()
              }
              if (
                instance?.nodeNames !== null &&
                instance.nodeNames.length > 0
              ) {
                if (!instance.nodeNames.includes(jsConfig.nodeName)) {
                  console.log('Redundancy - Node name not allowed, exiting...')
                  process.exit()
                }
              }
              if (instance?.activeNodeName === jsConfig.nodeName) {
                if (!ProcessActive) console.log('Redundancy - Node activated!')
                countKeepAliveNotUpdated = 0
                ProcessActive = true
              } else {
                // other node active
                if (ProcessActive) {
                  console.log('Redundancy - Node deactivated!')
                  countKeepAliveNotUpdated = 0
                }
                ProcessActive = false
                if (lastActiveNodeKeepAliveTimeTag === instKeepAliveTimeTag) {
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
                await db
                  .collection(ProtocolDriverInstancesCollectionName)
                  .updateOne(
                    {
                      protocolDriver: APP_NAME,
                      protocolDriverInstanceNumber: new Double(Instance),
                    },
                    {
                      $set: {
                        activeNodeName: jsConfig.nodeName,
                        activeNodeKeepAliveTimeTag: new Date(),
                        softwareVersion: VERSION,
                        stats: {},
                      },
                    }
                  )
                if (ConnectionNumber)
                  await updateConnectionStats(
                    db.collection(ProtocolConnectionsCollectionName),
                    ConnectionNumber,
                    {
                      nodeName: jsConfig.nodeName,
                      timeTag: new Date(),
                      isConnected:
                        Date.now() - LastMessageTime < ConnectedWindowMs,
                    }
                  )
              }
            } catch (err) {
              console.log('Redundancy - Error: ' + (err?.message || err))
            }
          }

          // check and update redundancy control
          ProcessRedundancy()
          clearInterval(redundancyIntervalHandle)
          redundancyIntervalHandle = setInterval(ProcessRedundancy, 5000)
        })
      }

      // wait 5 seconds
      await new Promise((resolve) => setTimeout(resolve, 5000))

      // detect connection problems, if error will null the client to later reconnect
      if (clientMongo === undefined) {
        console.log('Disconnected Mongodb!')
        clientMongo = null
      }
      if (clientMongo) {
        if (!(await checkConnectedMongo(clientMongo))) {
          // not anymore connected, will retry
          ProcessActive = false
          console.log('Disconnected Mongodb!')
          if (clientMongo) clientMongo.close()
          clientMongo = null
        }
      }
    } catch (e) {
      console.log('Error connecting to mongodb!')
    }
  }
})()

// test mongoDB connectivity
let CheckMongoConnectionTimeout = 10000
let HintMongoIsConnected = true
// $set stats.<field> on a connection document. Connection documents are
// created with stats: null, where a dotted $set fails with error 28
// (PathNotViable): replace the null with an empty document and retry once.
async function updateConnectionStats(connsCollection, connNumber, fields) {
  const set = {}
  for (const [k, v] of Object.entries(fields)) set['stats.' + k] = v
  const filter = { protocolConnectionNumber: connNumber }
  try {
    return await connsCollection.updateOne(filter, { $set: set })
  } catch (err) {
    if (err?.code !== 28) throw err
    await connsCollection.updateOne(
      { ...filter, stats: null },
      { $set: { stats: {} } }
    )
    return await connsCollection.updateOne(filter, { $set: set })
  }
}

async function checkConnectedMongo(client) {
  if (!client) {
    return false
  }

  let tr = setTimeout(() => {
    console.log('Mongo ping timeout error!')
    ProcessActive = false
    HintMongoIsConnected = false
  }, CheckMongoConnectionTimeout)

  let res = null
  try {
    res = await client.db('admin').command({ ping: 1 })
    clearTimeout(tr)
  } catch (e) {
    console.log('Error on mongodb connection!')
    return false
  }
  if ('ok' in res && res.ok) {
    HintMongoIsConnected = true
    return true
  } else {
    HintMongoIsConnected = false
    return false
  }
}
