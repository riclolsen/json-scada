'use strict'

/*
 * MQTT-Sparkplug B Client Driver for JSON-SCADA
 *
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

const APP_NAME = 'MQTT-SPARKPLUG-B'
const APP_MSG = '{json:scada} - MQTT-Sparkplug-B Client Driver'
const VERSION = '0.1.1'
const hwVersion = 'Generic Server Hardware'
const swVersion = 'JSON-SCADA MQTT v' + VERSION
let ProcessActive = false // for redundancy control
let jsConfigFile = '../../conf/json-scada.json'
const LogLevelMin = 0,
  LogLevelNormal = 1,
  LogLevelDetailed = 2,
  LogLevelDebug = 3

const SparkplugClient = require('./sparkplug-client')
const fs = require('fs')
const mongo = require('mongodb')
const MongoClient = require('mongodb').MongoClient
const Queue = require('queue-fifo')
const { setInterval } = require('timers')
// const { time } = require('console')
const grpSep = '~'
const csPipeline = [
  {
    $project: { documentKey: false }
  },
  {
    $match: {
      $or: [
        {
          $and: [
            {
              'updateDescription.updatedFields.sourceDataUpdate': {
                $exists: false
              }
            },
            {
              'fullDocument._id': {
                $ne: -2
              }
            },
            {
              'fullDocument._id': {
                $ne: -1
              }
            },
            { operationType: 'update' }
          ]
        },
        { operationType: 'replace' }
      ]
    }
  }
]

let ListCreatedTags = []
let ValuesQueue = new Queue() // queue of values to update acquisition
let PublishQueue = new Queue() // queue of values to publish

const args = process.argv.slice(2)
var inst = null
if (args.length > 0) inst = parseInt(args[0])
const Instance = inst || process.env.JS_MQTTSPB_LISTENER_INSTANCE || 1
let ConnectionNumber = 0
let AutoCreateTags = true
const AutoKeyMultiplier = 100000 // should be more than estimated maximum points on a connection
let AutoKeyId = 0

var logLevel = null
if (args.length > 1) logLevel = parseInt(args[1])
const LogLevel = logLevel || process.env.JS_MQTTSPB_LISTENER_LOGLEVEL || 1

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

let rawFileContents = fs.readFileSync(jsConfigFile)
let jsConfig = JSON.parse(rawFileContents)
if (
  typeof jsConfig.mongoConnectionString != 'string' ||
  jsConfig.mongoConnectionString === ''
) {
  console.log('Error reading config file.')
  process.exit()
}

if (LogLevel > LogLevelMin) console.log('MongoDB - Connecting to MongoDB server...')
;(async () => {
  let collection = null

  setInterval(async function () {
    let cnt = 0,
      metrics = []
    if (clientMongo && collection)
      while (!PublishQueue.isEmpty()) {
        let data = PublishQueue.peek()
        metrics.push(data)
        PublishQueue.dequeue()
        cnt++
      }
    if (cnt) {
      let payload = {
        timestamp: new Date().getTime(),
        metrics: metrics
      }
      console.log(JSON.stringify(payload))
      if (LogLevel >= LogLevelNormal) console.log('Sparkplug - Updates: ' + cnt)
    }
  }, 1127)

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
                console.log('Auto Key - Tag not found, will create: ' + data.tag)
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
            'Data Update - ' +
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
          originator: APP_NAME + '|' + ConnectionNumber,
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
      if (LogLevel >= LogLevelNormal) console.log('MongoDB - Updates: ' + cnt)
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
          console.log('MongoDB - Connected correctly to MongoDB server')

        // specify db and collections
        const db = client.db(jsConfig.mongoDatabaseName)
        collection = db.collection(RealtimeDataCollectionName)

        // find the connection number, if not found abort (only one connection per instance is allowed for this protocol)
        let connection = await getConnection(
          db.collection(ProtocolConnectionsCollectionName)
        )
        ConnectionNumber = connection.protocolConnectionNumber
        console.log('Connection - ' + ConnectionNumber)
        if ('autoCreateTags' in connection) {
          AutoCreateTags = connection.autoCreateTags ? true : false
        }

        AutoKeyId = await getAutoKeyInitialValue(collection)
        console.log('Auto Key - ' + AutoKeyId)

        let redundancy = {
          lastActiveNodeKeepAliveTimeTag: null,
          countKeepAliveNotUpdated: 0,
          countKeepAliveUpdatesLimit: 0,
          clientMongo: clientMongo,
          db: db
        }

        // check and update redundancy control
        ProcessRedundancy(redundancy)
        clearInterval(redundancyIntervalHandle)
        redundancyIntervalHandle = setInterval(function(){ProcessRedundancy(redundancy)}, 5000)

        // Create the SparkplugClient
        let config = getSparkplugConfig(connection)

        const sparkplugClient = SparkplugClient.newClient(config)

        sparkplugClient.on('error', function (error) {
          console.log("Sparkplug - Can't connect" + error)
        })

        // process MQTT messages
        sparkplugClient.on('message', function (topic, payload, topicInfo) {
          console.log('message')
          console.log(topic)
          // console.log(topicInfo);
          console.log(payload)
        })

        // Subscribe topics
        sparkplugClient.client.subscribe('spBv1.0/#', {
          qos: 1,
          properties: { subscriptionIdentifier: 1 },
          function (err, granted) {
            console.log("Sparkplug - Error: " + err)
            console.log(granted)
            return
          }
        })

        // getDeviceBirthPayload(collection)
        const changeStream = collection.watch(csPipeline, {
          fullDocument: 'updateLookup'
        })

        try {
          changeStream.on('error', change => {
            if (clientMongo) clientMongo.close()
            clientMongo = null
            console.log('MongoDB - Error on ChangeStream!')
          })
          changeStream.on('close', change => {
            if (clientMongo) clientMongo.close()
            clientMongo = null
            console.log('MongoDB - Closed ChangeStream!')
          })
          changeStream.on('end', change => {
            if (clientMongo) clientMongo.close()
            clientMongo = null
            console.log('MongoDB - Ended ChangeStream!')
          })

          // start listen to changes
          changeStream.on('change', change => {
            let data = getMetricPayload(change.fullDocument)
            if (data !== null) PublishQueue.enqueue(data)
          })
        } catch (e) {
          console.log("MongoDB - Error: " + e)
        }
      })

    // wait 5 seconds
    await new Promise(resolve => setTimeout(resolve, 5000))

    // detect connection problems, if error will null the client to later reconnect
    if (clientMongo === undefined) {
      console.log('MongoDB - Disconnected Mongodb!')
      clientMongo = null
    }
    if (clientMongo)
      if (!clientMongo.isConnected()) {
        // not anymore connected, will retry
        console.log('MongoDB - Disconnected Mongodb!')
        clientMongo.close()
        clientMongo = null
      }
  }
})()

// Get BIRTH payload for the edge node
function getNodeBirthPayload() {
  return {
    timestamp: new Date().getTime(),
    metrics: [
      {
        name: 'Node Control/Rebirth',
        type: 'boolean',
        value: false
      },
      {
        name: 'Node Control/Reboot',
        type: 'boolean',
        value: false
      },
      {
        name: 'Properties/sw_version',
        type: 'string',
        value: swVersion
      },
      {
        name: 'Properties/hw_version',
        type: 'string',
        value: hwVersion
      }
    ]
  }
}

// Get BIRTH payload for the device
async function getDeviceBirthPayload(rtCollection) {
  let res = await rtCollection
    .find(
      {
        // protocolSourceConnectionNumber: ConnectionNumber,
        // protocolSourceObjectAddress: data.tag
        origin: { $ne: 'command' },
        _id: { $gt: 0 }
      },
      {
        projection: {
          _id: 1,
          tag: 1,
          type: 1,
          value: 1,
          valueString: 1,
          timeTag: 1,
          timeTagAtSource: 1,
          invalid: 1,
          isEvent: 1,
          description: 1,
          origin: 1
        }
      }
    )
    .toArray()

  res.map(function (element) {
    if (element.origin === 'command' || element._id < 1) {
      return
    }

    let type, value
    switch (element.type) {
      case 'digital':
        type = 'boolean'
        if (element.isEvent)
          // pure events steady state is false
          value = false
        else value = element.value ? true : false
        break
      case 'string':
        type = 'string'
        if ('valueString' in element) value = element.valueString
        else value = element.value.toString()
        break
      case 'analog':
        type = 'double'
        value = element.value
        break
      default:
        return
    }

    let timestamp = false
    let timestampQualityGood = false
    if ('timeTagAtSource' in element && element.timeTagAtSource !== null) {
      timestamp = new Date(element.timeTagAtSource).getTime()
      timestampQualityGood = element.timeTagAtSourceOk
    }
    //else if ("timeTag" in element && element.timeTag !== null){
    //  timestamp = new Date(element.timeTag).getTime()
    //} else {
    //  timestamp = new Date().getTime()
    //}

    let ret = {
      name: element.tag,
      alias: element._id,
      value: value,
      type: type,
      ...(timestamp === false ? {} : { timestamp: timestamp }),
      properties: {
        description: element.description,
        qualityGood: element.invalid ? false : true,
        ...(timestamp === false
          ? {}
          : { timestampQualityGood: timestampQualityGood })
      }
    }
    // console.log(element)
    // console.log(ret)
    return ret
  })

  return {
    timestamp: new Date().getTime(),
    metrics: res
  }
}

// Get data payload
function getMetricPayload(element) {
  if (element.origin === 'command' || element._id < 1) {
    return null
  }
  let type, value
  switch (element.type) {
    case 'digital':
      type = 'boolean'
      if (element.isEvent)
        // pure events steady state is false
        value = false
      else value = element.value ? true : false
      break
    case 'string':
      type = 'string'
      if ('valueString' in element) value = element.valueString
      else value = element.value.toString()
      break
    case 'analog':
      type = 'double'
      value = element.value
      break
    default:
      return null
  }

  let timestamp = false
  let timestampQualityGood = false
  if ('timeTagAtSource' in element && element.timeTagAtSource !== null) {
    timestamp = new Date(element.timeTagAtSource).getTime()
    timestampQualityGood = element.timeTagAtSourceOk
  }
  //else if ("timeTag" in element && element.timeTag !== null){
  //  timestamp = new Date(element.timeTag).getTime()
  //} else {
  //  timestamp = new Date().getTime()
  //}

  return {
    // name: element.tag,
    alias: element._id,
    value: value,
    type: type,
    ...(timestamp === false ? {} : { timestamp: timestamp }),
    properties: {
      qualityGood: element.invalid ? false : true,
      ...(timestamp === false
        ? {}
        : { timestampQualityGood: timestampQualityGood })
    }
  }
}

// find the connection number, if not found abort (only one connection per instance is allowed for this protocol)
async function getConnection (connsCollection) {
  let results = await connsCollection
    .find({
      protocolDriver: APP_NAME,
      protocolDriverInstanceNumber: Instance
    })
    .toArray()

  if (!results || !('length' in results) || results.length == 0) {
    console.log('Connection - No protocol connection found!')
    process.exit(1)
  }
  const connection = results[0]
  if (!('protocolConnectionNumber' in connection)) {
    console.log('Connection - No protocol connection found on record!')
    process.exit(2)
  }
  console.log('Connection - Connection Number:' + connection.protocolConnectionNumber)
  if (connection.enabled === false) {
    console.log(
      'Connection - Connection disabled, exiting! (connection:' +
        connection.protocolConnectionNumber +
        ')'
    )
    process.exit(3)
  }
  return connection
}

// find biggest point key (_id) on range and adjust automatic key
async function getAutoKeyInitialValue (rtCollection) {
  let autoKeyId = ConnectionNumber * AutoKeyMultiplier
  let resLastKey = await rtCollection
    .find({
      _id: {
        $gt: autoKeyId,
        $lt: (ConnectionNumber + 1) * AutoKeyMultiplier
      }
    })
    .sort({ _id: -1 })
    .limit(1)
    .toArray()
  if (resLastKey.length > 0 && '_id' in resLastKey[0]) {
    if (parseInt(resLastKey[0]._id) >= autoKeyId)
      autoKeyId = parseInt(resLastKey[0]._id)
  }
  return autoKeyId
}

function rtData(measurement) {
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
    type:
      typeof measurement.value === 'number' &&
      !isNaN(parseFloat(measurement.value))
        ? 'analog'
        : 'string',
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

function getSparkplugConfig(connection){
  return {
    serverUrl: 'mqtt://127.0.0.1:1883',
    //'serverUrl' :'mqtt://broker.hivemq.com',
    //'serverUrl' : 'mqtts://dbb220030c79405695f35740dab40eb0.s1.eu.hivemq.cloud:8883',
    username: 'admin',
    password: 'changeme',
    groupId: 'JSON-SCADA',
    edgeNode: 'mainNode',
    clientId: 'JsonScadaMainNode',
    version: 'spBv1.0',
    scadaHostId: connection.scadaHostId // only if a primary application

    //'pfx': Fs.readFileSync('certificate.pfx'),
    //'passphrase': 'test',
    //'ca': Fs.readFileSync('root.key'),
    //'key': Fs.readFileSync('client.key'),
    //'cert':  Fs.readFileSync('client.pem'),
    //'secureProtocol': 'TLSv1_2_method',
    //'rejectUnauthorized': false,
    // 'minVersion': 'TLSv1',
    // 'maxVersion': 'TLSv1.3'
    // 'ciphers': 'TLSv1.3'
  }
}

async function ProcessRedundancy (redundancy) {
  if (!redundancy || !redundancy.clientMongo) return

  if (LogLevel >= LogLevelNormal)
    console.log('Redundancy - Process Active: ' + ProcessActive)

  // look for process instance entry, if not found create a new entry
  redundancy.db.collection(ProtocolDriverInstancesCollectionName)
    .find({
      protocolDriver: APP_NAME,
      protocolDriverInstanceNumber: Instance
    })
    .toArray(function (err, results) {
      if (err) console.log("MongoDB - " + err)
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
            redundancy.countKeepAliveNotUpdated = 0
            ProcessActive = true
          } else {
            // other node active
            if (ProcessActive) {
              console.log('Redundancy - Node deactivated!')
              redundancy.countKeepAliveNotUpdated = 0
            }
            ProcessActive = false
            if (
              redundancy.lastActiveNodeKeepAliveTimeTag === instKeepAliveTimeTag
            ) {
              redundancy.countKeepAliveNotUpdated++
              console.log(
                'Redundancy - Keep-alive from active node not updated. ' +
                  redundancy.countKeepAliveNotUpdated
              )
            } else {
              redundancy.countKeepAliveNotUpdated = 0
              console.log(
                'Redundancy - Keep-alive updated by active node. Staying inactive.'
              )
            }
            redundancy.lastActiveNodeKeepAliveTimeTag = instKeepAliveTimeTag
            if (redundancy.countKeepAliveNotUpdated > redundancy.countKeepAliveUpdatesLimit) {
              // cnt exceeded, be active
              redundancy.countKeepAliveNotUpdated = 0
              console.log('Redundancy - Node activated!')
              ProcessActive = true
            }
          }

          if (ProcessActive) {
            // process active, then update keep alive
            redundancy.db.collection(
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
