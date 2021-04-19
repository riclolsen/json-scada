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
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')
const LoadConfig = require('./load-config')
const Redundancy = require('./redundancy')
const AutoTag = require('./auto-tag')

const ValuesQueue = new Queue() // queue of values to update acquisition
let AutoCreateTags = true

;(async () => {
  const jsConfig = LoadConfig() // load and parse config file
  Log.logLevelCurrent = jsConfig.LogLevel
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
    await sparkplugProcess(sparkplugClient, connection, jsConfig, clientMongo)

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

        let autoKeyId = await AutoTag.GetAutoKeyInitialValue(collection, jsConfig)
        Log.log('Auto Key - Initial value: ' + autoKeyId)

        Redundancy.Start(5000, clientMongo, db, jsConfig)

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
  const swVersion = 'JSON-SCADA MQTT v' + AppDefs.VERSION

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
          timeTagAtSourceOk: 1,
          invalid: 1,
          isEvent: 1,
          description: 1,
          origin: 1
        }
      }
    )
    .toArray()

  let metrics = []
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
      if ('timeTagAtSourceOk' in element)
        timestampQualityGood = element.timeTagAtSourceOk
    }
    //else if ("timeTag" in element && element.timeTag !== null){
    //  timestamp = new Date(element.timeTag).getTime()
    //} else {
    //  timestamp = new Date().getTime()
    //}

    let metric = {
      name: element.tag,
      alias: element._id,
      value: value,
      type: type,
      ...(timestamp === false ? {} : { timestamp: timestamp }),

      properties: {
        description: { type: 'string', value: element.description },
        qualityGood: { type: 'boolean', value: element.invalid ? false : true },
        ...(timestamp === false
          ? {}
          : {
              timestampQualityGood: {
                type: 'boolean',
                value: timestampQualityGood
              }
            })
      }
    }
    metrics.push(metric)
    // Log.log(element)
    // Log.log(ret)
    return element
  })

  return {
    timestamp: new Date().getTime(),
    metrics: metrics
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
      protocolDriver: AppDefs.NAME,
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

// prepare mongo connection options
function getMongoConnectionOptions (configObj) {
  let connOptions = {
    useNewUrlParser: true,
    useUnifiedTopology: true,
    appname:
      AppDefs.NAME +
      ' Version:' +
      AppDefs.VERSION +
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
async function processMongoUpdates (clientMongo, collection, jsConfig) {
  let cnt = 0
  if (clientMongo && collection)
    while (!ValuesQueue.isEmpty()) {
      let data = ValuesQueue.peek()
      // const db = clientMongo.db(jsConfig.mongoDatabaseName)

      // check tag is created, if not found create it
      if (AutoCreateTags){
         await AutoTag.AutoCreateTag(data, jsConfig.ConnectionNumber, collection)
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
        originator: AppDefs.NAME + '|' + jsConfig.ConnectionNumber,
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
async function sparkplugProcess (
  spClient,
  jscadaConnection,
  configObj,
  mongoClient
) {
  if (jscadaConnection === null || mongoClient === null) return
  const logMod = 'MQTT Client - '
  const connectionRetriesLimit = 20

  // poor man's local static variables
  if (typeof sparkplugProcess.currentBroker === 'undefined') {
    sparkplugProcess.currentBroker = 0
    sparkplugProcess.connectionRetries = 0
    sparkplugProcess.deviceBirthPayload = null
  }

  if (spClient.handle === null) {
    if (!Redundancy.ProcessStateIsActive())
      // do not connect MQTT while process not active
      return

    try {
      // obtain device birth payload
      const db = mongoClient.db(configObj.mongoDatabaseName)
      sparkplugProcess.deviceBirthPayload = await getDeviceBirthPayload(
        db.collection(configObj.RealtimeDataCollectionName)
      )
      // Log.log(sparkplugProcess.deviceBirthPayload, Log.levelDebug)

      // Create the SparkplugClient
      Log.log(logMod + 'Creating client...')
      let config = getSparkplugConfig(jscadaConnection)

      config.serverUrl =
        jscadaConnection.endpointURLs[sparkplugProcess.currentBroker]
      Log.log(logMod + 'Try connecting to ' + config.serverUrl)

      // default subscription to all topics from Sparkplug-B
      if (
        !('topics' in jscadaConnection) ||
        jscadaConnection.topics.length === 0
      ) {
        jscadaConnection.topics = ['spBv1.0/#']
      }

      if (
        !'deviceId' in jscadaConnection ||
        jscadaConnection.deviceId.trim() === ''
      ) {
        jscadaConnection.deviceId = 'Primary Application'
      }

      spClient.handle = SparkplugClient.newClient(config)

      if (Log.levelCurrent === Log.levelDebug)
        spClient.handle.logger.level = 'debug'

      spClient.handle.on('error', function (error) {
        Log.log(logMod + "Event: Can't connect" + error)
      })

      spClient.handle.on('close', function () {
        Log.log(logMod + 'Event: Connection Closed')
      })

      spClient.handle.on('offline', function () {
        Log.log(logMod + 'Event: Connection Offline...')
      })

      // Create 'birth' handler
      spClient.handle.on('birth', function () {
        // Publish SCADA HOST BIRTH certificate
        spClient.handle.publishScadaHostBirth()
        // Publish Node BIRTH certificate
        spClient.handle.publishNodeBirth(getNodeBirthPayload(configObj))

        // Publish Device BIRTH certificate
        spClient.handle.publishDeviceBirth(
          jscadaConnection.deviceId,
          sparkplugProcess.deviceBirthPayload
        )
      })

      spClient.handle.on('connect', function () {
        sparkplugProcess.connectionRetries = 0
        Log.log(logMod + 'Event: Connected to broker')
        // Subscribe topics
        jscadaConnection.topics.forEach(elem => {
          Log.log(logMod + 'Subscribing topic: ' + elem)

          spClient.handle.client.subscribe(elem, {
            qos: 1,
            properties: { subscriptionIdentifier: 1 },
            function (err, granted) {
              if (err)
                Log.log(
                  logMod + 'Subscribe error on topic: ' + elem + ' : ' + err
                )
              if (granted)
                Log.log(
                  logMod +
                    'Subscription granted for topic: ' +
                    elem +
                    ' : ' +
                    granted
                )
              return
            }
          })
        })
      })

      spClient.handle.on('reconnect', function () {
        sparkplugProcess.connectionRetries++
        Log.log(
          logMod +
            'Event: Trying to reconnect to broker...' +
            sparkplugProcess.connectionRetries
        )
        // when retires various times it will try to recreate the client to connect other broker (if available)
        if (
          jscadaConnection.endpointURLs.length > 1 &&
          sparkplugProcess.connectionRetries >= connectionRetriesLimit
        ) {
          Log.log(logMod + 'Too many retries will try another broker...')
          sparkplugProcess.currentBroker =
            (sparkplugProcess.currentBroker + 1) %
            jscadaConnection.endpointURLs.length
          // stop and destroy current client
          spClient.handle.stop()
          spClient.handle = null
          sparkplugProcess.connectionRetries = 0
        }
      })

      // process MQTT messages
      spClient.handle.on('message', function (topic, payload, topicInfo) {
        Log.log(logMod + 'Event: message')
        Log.log(logMod + topic)
        // Log.log(topicInfo);
        Log.log(logMod + payload)
      })
    } catch (e) {
      Log.log(logMod + 'Error: ' + e.message, Log.levelMin)
    }
  } else {
    // MQTT client is already created

    if (Redundancy.ProcessStateIsActive()) {
      // test connection is established

      Log.log(
        logMod +
          (spClient.handle.connected
            ? 'Currently Connected'
            : 'Currently Disconnected')
      )
      if (spClient.handle.connected) {
        sparkplugProcess.connectionRetries = 0
      }
    } else {
      // if process not active, stop mqtt
      Log.log(logMod + 'Stopping client...')
      spClient.handle.stop()
      spClient.handle = null
    }
  }

  return
}
