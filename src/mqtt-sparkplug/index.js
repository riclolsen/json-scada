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

const SparkplugClient = require('./sparkplug-client')
const fs = require('fs')
const mongo = require('mongodb')
const MongoClient = require('mongodb').MongoClient
const Queue = require('queue-fifo')
const { setInterval } = require('timers')
const { connect } = require('http2')

const Log = {
  // simple message logger
  levelMin: 0,
  levelNormal: 1,
  levelDetailed: 2,
  levelDebug: 3,
  levelCurrent: 1,
  dtOptions: {
    year: '2-digit',
    month: '2-digit',
    day: '2-digit',
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  },
  log: function (msg, level = 1) {
    if (level <= this.levelCurrent) {
      let dt = new Date()
      console.log(
        dt.toISOString() + 
          ' - ' +
          msg
      )
    }
  }
}

const ValuesQueue = new Queue() // queue of values to update acquisition
let AutoCreateTags = true
let AutoKeyId = 0

;(async () => {
  const jsConfig = loadConfig()
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

  const PublishQueue = new Queue() // queue of values to publish
  const sparkplugClient = { handle: null } // points to sparkplug-client object
  let collection = null
  let clientMongo = null
  let connection = null

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
      // Log.log(JSON.stringify(payload))
      Log.log('Sparkplug - Updates: ' + cnt, Log.levelNormal)
    }
  }, 1127)

  setInterval(async function () {
    processMongoUpdates(clientMongo, collection, jsConfig)
  }, 500)

  Log.log('MongoDB - Connecting to MongoDB server...', Log.levelMin)

  let redundancyIntervalHandle = null
  while (true) {
    // repeat every 5 seconds

    // manages MQTT connection
    sparkplugProcess(sparkplugClient, connection, jsConfig)

    if (clientMongo === null)
      // if disconnected
      await MongoClient.connect(
        // try to (re)connect
        jsConfig.mongoConnectionString,
        getMongoConnectionOptions(jsConfig)
      ).then(async client => {
        // connected
        clientMongo = client

        Log.log('MongoDB - Connected correctly to MongoDB server', Log.levelMin)

        // specify db and collections
        const db = client.db(jsConfig.mongoDatabaseName)
        collection = db.collection(jsConfig.RealtimeDataCollectionName)

        // find the connection number, if not found abort (only one connection per instance is allowed for this protocol)
        connection = await getConnection(
          db.collection(jsConfig.ProtocolConnectionsCollectionName),
          jsConfig
        )
        jsConfig.ConnectionNumber = connection.protocolConnectionNumber
        Log.log('Connection - Connection Number: ' + jsConfig.ConnectionNumber)
        if ('autoCreateTags' in connection) {
          AutoCreateTags = connection.autoCreateTags ? true : false
        }

        AutoKeyId = await getAutoKeyInitialValue(collection, jsConfig)
        Log.log('Auto Key - Initial value: ' + AutoKeyId)

        let redundancy = {
          lastActiveNodeKeepAliveTimeTag: null,
          countKeepAliveNotUpdated: 0,
          countKeepAliveUpdatesLimit: 4,
          clientMongo: clientMongo,
          db: db
        }

        // check and update redundancy control
        ProcessRedundancy(redundancy, jsConfig)
        clearInterval(redundancyIntervalHandle)
        redundancyIntervalHandle = setInterval(function () {
          ProcessRedundancy(redundancy, jsConfig)
        }, 5000)

        const changeStream = collection.watch(csPipeline, {
          fullDocument: 'updateLookup'
        })

        try {
          changeStream.on('error', change => {
            if (clientMongo) clientMongo.close()
            clientMongo = null
            Log.log('MongoDB - Error on ChangeStream!')
          })
          changeStream.on('close', change => {
            if (clientMongo) clientMongo.close()
            clientMongo = null
            Log.log('MongoDB - Closed ChangeStream!')
          })
          changeStream.on('end', change => {
            if (clientMongo) clientMongo.close()
            clientMongo = null
            Log.log('MongoDB - Ended ChangeStream!')
          })

          // start listen to changes
          changeStream.on('change', change => {
            let data = getMetricPayload(change.fullDocument)
            if (data !== null) PublishQueue.enqueue(data)
          })
        } catch (e) {
          Log.log('MongoDB - Error: ' + e, Log.levelMin)
        }
      })

    // wait 5 seconds
    await new Promise(resolve => setTimeout(resolve, 5000))

    // detect connection problems, if error will null the client to later reconnect
    if (clientMongo === undefined) {
      Log.log('MongoDB - Disconnected Mongodb!')
      clientMongo = null
    }
    if (clientMongo)
      if (!clientMongo.isConnected()) {
        // not anymore connected, will retry
        Log.log('MongoDB - Disconnected Mongodb!')
        clientMongo.close()
        clientMongo = null
      }
  }
})()

// Get BIRTH payload for the edge node
function getNodeBirthPayload (configObj) {
  const hwVersion = 'Generic Server Hardware'
  const swVersion = 'JSON-SCADA MQTT v' + configObj?.VERSION

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
async function getDeviceBirthPayload (rtCollection) {
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
    // Log.log(element)
    // Log.log(ret)
    return ret
  })

  return {
    timestamp: new Date().getTime(),
    metrics: res
  }
}

// Get data payload
function getMetricPayload (element) {
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
async function getConnection (connsCollection, configObj) {
  let results = await connsCollection
    .find({
      protocolDriver: configObj.APP_NAME,
      protocolDriverInstanceNumber: configObj.Instance
    })
    .toArray()

  if (!results || !('length' in results) || results.length == 0) {
    Log.log('Connection - No protocol connection found!')
    process.exit(1)
  }
  const connection = results[0]
  if (!('protocolConnectionNumber' in connection)) {
    Log.log('Connection - No protocol connection found on record!')
    process.exit(2)
  }
  if (connection.enabled === false) {
    Log.log(
      'Connection - Connection disabled, exiting! (connection:' +
        connection.protocolConnectionNumber +
        ')'
    )
    process.exit(3)
  }
  return connection
}

// find biggest point key (_id) on range and adjust automatic key
async function getAutoKeyInitialValue (rtCollection, configObj) {
  let autoKeyId = configObj.ConnectionNumber * configObj.AutoKeyMultiplier
  let resLastKey = await rtCollection
    .find({
      _id: {
        $gt: autoKeyId,
        $lt: (configObj.ConnectionNumber + 1) * configObj.AutoKeyMultiplier
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

function rtData (measurement) {
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
    commissioningRemarks: 'Auto created by Sparkplug B driver.',
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

// process JSON-SCADA redundancy state for this driver module
async function ProcessRedundancy (redundancy, configObj) {
  if (!redundancy || !redundancy.clientMongo) return

  Log.log(
    'Redundancy - Process ' + (configObj.ProcessActive ? 'Active' : 'Inactive')
  )

  // look for process instance entry, if not found create a new entry
  redundancy.db
    .collection(configObj.ProtocolDriverInstancesCollectionName)
    .find({
      protocolDriver: configObj.APP_NAME,
      protocolDriverInstanceNumber: configObj.Instance
    })
    .toArray(function (err, results) {
      if (err) Log.log('MongoDB - ' + err)
      else if (results) {
        if (results.length === 0) {
          // not found, then create
          configObj.ProcessActive = true
          Log.log('Redundancy - Instance config not found, creating one...')
          db.collection(
            configObj.ProtocolDriverInstancesCollectionName
          ).insertOne({
            protocolDriver: configObj.APP_NAME,
            protocolDriverInstanceNumber: new mongo.Double(1),
            enabled: true,
            logLevel: new mongo.Double(1),
            nodeNames: [],
            activeNodeName: configObj.nodeName,
            activeNodeKeepAliveTimeTag: new Date()
          })
        } else {
          // check for disabled or node not allowed
          let instance = results[0]

          let instKeepAliveTimeTag = null

          if ('activeNodeKeepAliveTimeTag' in instance)
            instKeepAliveTimeTag = instance.activeNodeKeepAliveTimeTag.toISOString()

          if (instance?.enabled === false) {
            Log.log('Redundancy - Instance disabled, exiting...')
            process.exit()
          }
          if (instance?.nodeNames !== null && instance.nodeNames.length > 0) {
            if (!instance.nodeNames.includes(configObj.nodeName)) {
              Log.log('Redundancy - Node name not allowed, exiting...')
              process.exit()
            }
          }
          if (instance?.activeNodeName === configObj.nodeName) {
            if (!configObj.ProcessActive)
              Log.log('Redundancy - Node activated!')
            redundancy.countKeepAliveNotUpdated = 0
            configObj.ProcessActive = true
          } else {
            // other node active
            if (configObj.ProcessActive) {
              Log.log('Redundancy - Node deactivated!')
              redundancy.countKeepAliveNotUpdated = 0
            }
            configObj.ProcessActive = false
            if (
              redundancy.lastActiveNodeKeepAliveTimeTag === instKeepAliveTimeTag
            ) {
              redundancy.countKeepAliveNotUpdated++
              Log.log(
                'Redundancy - Keep-alive from active node not updated. ' +
                  redundancy.countKeepAliveNotUpdated
              )
            } else {
              redundancy.countKeepAliveNotUpdated = 0
              Log.log(
                'Redundancy - Keep-alive updated by active node. Staying inactive.'
              )
            }
            redundancy.lastActiveNodeKeepAliveTimeTag = instKeepAliveTimeTag
            if (
              redundancy.countKeepAliveNotUpdated >
              redundancy.countKeepAliveUpdatesLimit
            ) {
              // cnt exceeded, be active
              redundancy.countKeepAliveNotUpdated = 0
              Log.log('Redundancy - Node activated!')
              configObj.ProcessActive = true
            }
          }

          if (configObj.ProcessActive) {
            // process active, then update keep alive
            redundancy.db
              .collection(configObj.ProtocolDriverInstancesCollectionName)
              .updateOne(
                {
                  protocolDriver: configObj.APP_NAME,
                  protocolDriverInstanceNumber: new mongo.Double(
                    configObj.Instance
                  )
                },
                {
                  $set: {
                    activeNodeName: configObj.nodeName,
                    activeNodeKeepAliveTimeTag: new Date(),
                    softwareVersion: configObj.VERSION,
                    stats: {}
                  }
                }
              )
          }
        }
      }
    })
}

// load and parse config file
function loadConfig () {
  const args = process.argv.slice(2)

  var logLevelArg = null
  if (args.length > 1) logLevelArg = parseInt(args[1])
  Log.levelCurrent =
    logLevelArg || process.env.JS_MQTTSPB_LISTENER_LOGLEVEL || 1

  var confFileArg = null
  if (args.length > 2) confFileArg = args[2]

  let configFile =
    confFileArg || process.env.JS_CONFIG_FILE || '../../conf/json-scada.json'
  Log.log('Config - Config File: ' + configFile)

  if (!fs.existsSync(configFile)) {
    Log.log('Config - Error: config file not found!')
    process.exit()
  }

  let rawFileContents = fs.readFileSync(configFile)
  let configObj = JSON.parse(rawFileContents)
  if (
    typeof configObj.mongoConnectionString != 'string' ||
    configObj.mongoConnectionString === ''
  ) {
    Log.log('Error reading config file.')
    process.exit()
  }

  var instArg = null
  if (args.length > 0) instArg = parseInt(args[0])
  configObj.Instance = instArg || process.env.JS_MQTTSPB_LISTENER_INSTANCE || 1

  configObj.APP_NAME = 'MQTT-SPARKPLUG-B'
  configObj.APP_MSG = '{json:scada} - MQTT-Sparkplug-B Client Driver'
  configObj.VERSION = '0.1.1'

  configObj.RealtimeDataCollectionName = 'realtimeData'
  configObj.ProtocolDriverInstancesCollectionName = 'protocolDriverInstances'
  configObj.ProtocolConnectionsCollectionName = 'protocolConnections'
  configObj.grpSep = '~'
  configObj.ProcessActive = false // for redundancy control
  configObj.AutoKeyMultiplier = 100000 // should be more than estimated maximum points on a connection
  configObj.ConnectionNumber = 0

  Log.log('Config - ' + configObj.APP_MSG + ' Version ' + configObj.VERSION)
  Log.log('Config - Instance: ' + configObj.Instance)
  Log.log('Config - Log level: ' + Log.levelCurrent)

  return configObj
}

// prepare mongo connection options
function getMongoConnectionOptions (configObj) {
  let connOptions = {
    useNewUrlParser: true,
    useUnifiedTopology: true,
    appname:
      configObj.APP_NAME +
      ' Version:' +
      configObj.VERSION +
      ' Instance:' +
      configObj.Instance,
    poolSize: 20,
    readPreference: MongoClient.READ_PRIMARY
  }

  if (
    typeof configObj.tlsCaPemFile === 'string' &&
    configObj.tlsCaPemFile.trim() !== ''
  ) {
    configObj.tlsClientKeyPassword = configObj.tlsClientKeyPassword || ''
    configObj.tlsAllowInvalidHostnames =
      configObj.tlsAllowInvalidHostnames || false
    configObj.tlsAllowChainErrors = configObj.tlsAllowChainErrors || false
    configObj.tlsInsecure = configObj.tlsInsecure || false

    connOptions.tls = true
    connOptions.tlsCAFile = configObj.tlsCaPemFile
    connOptions.tlsCertificateKeyFile = configObj.tlsClientPemFile
    connOptions.tlsCertificateKeyFilePassword = configObj.tlsClientKeyPassword
    connOptions.tlsAllowInvalidHostnames = configObj.tlsAllowInvalidHostnames
    connOptions.tlsInsecure = configObj.tlsInsecure
  }

  return connOptions
}

// update queued data to mongodb
let ListCreatedTags = []
async function processMongoUpdates (clientMongo, collection, jsConfig) {
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
              protocolSourceConnectionNumber: jsConfig.ConnectionNumber,
              protocolSourceObjectAddress: data.tag
            })
            .toArray()

          if ('length' in res && res.length === 0) {
            // not found, then create
            let newTag = rtData(data)
            Log.log(
              'Auto Key - Tag not found, will create: ' + data.tag,
              Log.levelDetailed
            )
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

      Log.log(
        'Data Update - ' +
          data.timeTagAtSource +
          ' : ' +
          data.tag +
          ' : ' +
          data.value,
        Log.levelDetailed
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
        originator: jsConfig.APP_NAME + '|' + jsConfig.ConnectionNumber,
        notTopicalAtSource: false,
        invalidAtSource: data.invalidAtSource,
        overflowAtSource: false,
        blockedAtSource: false,
        substitutedAtSource: false
      }
      collection.updateOne(
        {
          protocolSourceConnectionNumber: jsConfig.ConnectionNumber,
          protocolSourceObjectAddress: data.tag
        },
        { $set: { sourceDataUpdate: updTag } }
      )

      ValuesQueue.dequeue()
      cnt++
    }
  if (cnt) Log.log('MongoDB - Updates: ' + cnt)
}

// sparkplug-client configuration options based on JSON-SCADA connection settings
function getSparkplugConfig (connection) {
  let minVersion = 'TLSv1',
    maxVersion = 'TLSv1.3'
  if (!connection.allowTLSv10) minVersion = 'TLSv1.1'
  if (connection.allowTLSv11) maxVersion = 'TLSv1.1'
  else minVersion = 'TLSv1.2'
  if (connection.allowTLSv12) maxVersion = 'TLSv1.2'
  else minVersion = 'TLSv1.3'
  if (connection.allowTLSv13) maxVersion = 'TLSv1.3'

  let secOpts = {}
  if (connection.useSecurity) {
    certOpts = {}
    if (connection.pfxFilePath !== '') {
      certOpts = {
        pfx: Fs.readFileSync(connection.pfxFilePath),
        passphrase: connection.passphrase
      }
    } else {
      certOpts = {
        ca: Fs.readFileSync(connection.rootCertFilePath),
        key: Fs.readFileSync(connection.privateKeyFilePath),
        cert: Fs.readFileSync(connection.localCertFilePath),
        passphrase: connection.passphrase
      }
    }

    secOpts = {
      secureProtocol: 'TLSv1_2_method',
      rejectUnauthorized: connection.chainValidation,
      minVersion: minVersion,
      maxVersion: maxVersion,
      ciphers: connection.cipherList,
      ...certOpts
    }
  }

  return {
    serverUrl: connection.endpointURLs[0],
    username: connection.username,
    password: connection.password,
    groupId: connection.groupId,
    edgeNode: connection.edgeNodeId,
    clientId: 'JSON-SCADA',
    version: 'spBv1.0',
    scadaHostId: connection.scadaHostId, // only if a primary application
    ...secOpts
  }
}

// manage Sparkplug B Client connection, subscriptions, messages, events
function sparkplugProcess (spClient, jscadaConnection, configObj) {
  if (jscadaConnection === null) return
  const logMod = 'MQTT Client - '

  if (spClient.handle === null) {
    if (!configObj.ProcessActive)
      // do not connect MQTT while process not active
      return

    try {
      // Create the SparkplugClient
      Log.log(logMod + 'Creating client...')
      let config = getSparkplugConfig(jscadaConnection)

      spClient.handle = SparkplugClient.newClient(config)

      if (Log.levelCurrent === Log.levelDebug)
        spClient.handle.logger.level = 'debug';

      spClient.handle.on('error', function (error) {
        Log.log(logMod + "Event: Can't connect" + error)
      })

      spClient.handle.on('close', function () {
        Log.log(logMod + "Event: Connection Closed")
      })

      spClient.handle.on('offline', function () {
        Log.log(logMod + "Event: Connection Offline...")
      })

      spClient.handle.on('connect', function () {
        Log.log(logMod + 'Event: Connected to broker')
      })

      spClient.handle.on('reconnect', function () {
        Log.log(logMod + 'Event: Trying to reconnect to broker...')
      })

      // process MQTT messages
      spClient.handle.on('message', function (topic, payload, topicInfo) {
        Log.log(logMod + 'Event: message')
        Log.log(logMod + topic)
        // Log.log(topicInfo);
        Log.log(logMod + payload)
      })

      // default subscription to all topics from Sparkplug-B
      if (
        !('topics' in jscadaConnection) ||
        jscadaConnection.topics.length === 0
      ) {
        jscadaConnection.topics = ['spBv1.0/#']
      }

      // Subscribe topics
      jscadaConnection.topics.forEach(elem => {
        spClient.handle.client.subscribe(elem, {
          qos: 1,
          properties: { subscriptionIdentifier: 1 },
          function (err, granted) {
            Log.log(logMod + 'Subscribe Error: ' + err)
            // Log.log(granted)
            return
          }
        })
      })
    } catch (e) {
      Log.log(logMod + "Error: " + e.message, Log.levelMin)
    }
  } else {
    // MQTT client is already created

    if (configObj.ProcessActive) {

      // test connection is established

      Log.log(logMod + (spClient.handle.connected?"Currently Connected":"Currently Disconnected"))

    } else {
      // if process not active, stop mqtt
      Log.log(logMod + 'Stopping client...')
      spClient.handle.stop()
      spClient.handle = null
    }
  }

  return
  // getDeviceBirthPayload(collection)
}
