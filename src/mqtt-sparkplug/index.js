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

const ProtoBuf = require('protobufjs')
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

const SparkplugNS = 'spBv1.0'

const DevicesList = []
const MapAliasToObjectAddress = []

const ValuesQueue = new Queue() // queue of values to update acquisition
const SparkplugPublishQueue = new Queue() // queue of values to publish as Sparkplug-B
const MqttPublishQueue = new Queue() // queue of values to publish as standard MQTT topics
let SparkplugDeviceBirthed = false
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

  const SparkplugClientObj = { handle: null } // points to sparkplug-client object
  let collection = null
  let clientMongo = null
  let connection = null

  // process sparkplug queue
  setInterval(async function () {
    if (
      SparkplugClientObj.handle &&
      SparkplugClientObj.handle.connected &&
      connection
    ) {
      let cnt = 0,
        metrics = []
      while (!SparkplugPublishQueue.isEmpty()) {
        let data = SparkplugPublishQueue.peek()

        // publish as normal topics
        if (connection.publishTopicRoot && 'properties' in data) {
          // publish value under root/group1/group2/group3/name
          SparkplugClientObj.handle.client.publish(
            data.properties.topic.value,
            data.value.toString()
          )
          // publish under root/group1/tag: value(v), timestamp(t), quality(q)
          SparkplugClientObj.handle.client.publish(
            data.properties.topicAsTag.value + '/v',
            data.value.toString(),
            { retain: true }
          )
          if ('timestamp' in data)
            SparkplugClientObj.handle.client.publish(
              data.properties.topicAsTag.value + '/t',
              data.timestamp.toString(),
              { retain: true }
            )
          if ('qualityIsGood' in data.properties)
            SparkplugClientObj.handle.client.publish(
              data.properties.topicAsTag.value + '/q',
              data.properties.qualityIsGood.value.toString(),
              { retain: true }
            )

          // remove (static) properties to avoid publishing to sparkplug
          delete data.properties.topic
          delete data.properties.topicAsTag
        }

        // aggregate for sparkplug publishing
        metrics.push(data)
        SparkplugPublishQueue.dequeue()
        cnt++
      }

      // publish metrics as sparkplug b device data
      if (cnt) {
        let payload = {
          timestamp: new Date().getTime(),
          metrics: metrics
        }
        if (Log.logLevelCurrent >= Log.levelDetailed)
          Log.log(JSON.stringify(payload), Log.levelDetailed)
        SparkplugClientObj.handle.publishDeviceData(
          connection.deviceId,
          payload
        )
        Log.log('Sparkplug - Updates: ' + cnt, Log.levelNormal)
      }
    } else {
      if (SparkplugPublishQueue.size() > AppDefs.MAX_QUEUEDMETRICS)
        Log.log(
          'Sparkplug - Publish queue exceeded limit, discarding updates...',
          Log.levelDetailed
        )
      while (SparkplugPublishQueue.size() > AppDefs.MAX_QUEUEDMETRICS) {
        SparkplugPublishQueue.dequeue()
      }
    }
  }, AppDefs.SPARKPLUG_PUBLISH_INTERVAL)

  setInterval(async function () {
    processMongoUpdates(clientMongo, collection, jsConfig)
  }, 500)

  Log.log('MongoDB - Connecting to MongoDB server...', Log.levelMin)

  while (true) {
    // repeat every 5 seconds

    // manages MQTT connection
    await sparkplugProcess(
      SparkplugClientObj,
      connection,
      jsConfig,
      clientMongo
    )

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

        let autoKeyId = await AutoTag.GetAutoKeyInitialValue(
          collection,
          jsConfig
        )
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
            // do not queue data changes until device connected and birthed
            if (!SparkplugDeviceBirthed || !SparkplugClientObj.handle.connected)
              return

            let data = getMetricPayload(change.fullDocument, connection)
            if (data) SparkplugPublishQueue.enqueue(data)
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
        // protocolSourceObjectAddress: data.protocolSourceObjectAddress
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
  res.forEach(element => {
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
    //  timestamp = (new Date(element.timeTag)).getTime()
    //} else {
    //  timestamp = (new Date()).getTime()
    //}

    let metric = {
      name: element.tag,
      alias: element._id,
      value: value,
      type: type,
      ...(timestamp === false ? {} : { timestamp: timestamp }),

      properties: {
        description: { type: 'string', value: element.description },
        qualityIsGood: {
          type: 'boolean',
          value: element.invalid ? false : true
        },
        ...(timestamp === false
          ? {}
          : {
              timestampQualityIsGood: {
                type: 'boolean',
                value: timestampQualityGood
              }
            })
      }
    }
    metrics.push(metric)
    // Log.log(element)
    // Log.log(ret)
    return
  })

  return {
    timestamp: new Date().getTime(),
    metrics: metrics
  }
}

// Get data payload
function getMetricPayload (element, jscadaConnection) {
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
  //  timestamp = (new Date(element.timeTag)).getTime()
  //} else {
  //  timestamp = (new Date()).getTime()
  //}

  let topic = {}
  let topicAsTag = {}
  if (
    'publishTopicRoot' in jscadaConnection &&
    jscadaConnection.publishTopicRoot.trim() !== ''
  ) {
    let topicName = jscadaConnection.publishTopicRoot.trim() + '/'
    if (element.group1.trim() !== '')
      topicName += element.group1.trim().replace('/', '-') + '/'
    if (element.group2.trim() !== '')
      topicName += element.group2.trim().replace('/', '-') + '/'
    if (element.ungroupedDescription.trim() !== '')
      topicName += element.ungroupedDescription.trim().replace('/', '-')
    // topicName += '/' + element.tag
    topic = {
      topic: {
        type: 'string',
        value: topicName
      }
    }

    topicName = jscadaConnection.publishTopicRoot.trim() + '/'
    if (element.group1.trim() !== '')
      topicName += element.group1.trim().replace('/', '-') + '/'
    topicName += element.tag
    topicAsTag = {
      topicAsTag: {
        type: 'string',
        value: topicName
      }
    }
  }

  return {
    // name: element.tag,
    alias: element._id,
    value: value,
    type: type,
    ...(timestamp === false ? {} : { timestamp: timestamp }),
    properties: {
      qualityIsGood: {
        type: 'boolean',
        value: element.invalid ? false : true
      },
      ...topic,
      ...topicAsTag,
      ...(timestamp === false
        ? {}
        : {
            timestampQualityIsGood: {
              type: 'boolean',
              value: timestampQualityGood
            }
          })
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
      if (AutoCreateTags) {
        await AutoTag.AutoCreateTag(data, jsConfig.ConnectionNumber, collection)
      }

      // now update tag

      Log.log(
        'Data Update - ' +
          data.timeTagAtSource +
          ' : ' +
          data.protocolSourceObjectAddress +
          ' : ' +
          data.value,
        Log.levelDetailed
      )

      let updTag = {
        valueAtSource: parseFloat(data.value),
        valueStringAtSource: data.valueString,
        valueJsonAtSource: data?.valueJson,
        asduAtSource: data?.asduAtSource,
        causeOfTransmissionAtSource: data?.causeOfTransmissionAtSource,
        timeTagAtSource: data.timeTagAtSource,
        timeTagAtSourceOk: data.timeTagAtSourceOk,
        timeTag: new Date(),
        originator: AppDefs.NAME + '|' + jsConfig.ConnectionNumber,
        invalidAtSource: data.invalid,
        transientAtSource: data.transient,
        notTopicalAtSource: false,
        overflowAtSource: false,
        blockedAtSource: false,
        substitutedAtSource: false
      }
      collection.updateOne(
        {
          protocolSourceConnectionNumber: jsConfig.ConnectionNumber,
          protocolSourceObjectAddress: data.protocolSourceObjectAddress
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
    version: SparkplugNS,
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
        jscadaConnection.topics = [SparkplugNS + '/#']
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
        SparkplugDeviceBirthed = false
        Log.log(logMod + 'Event: Connection Closed')
      })

      spClient.handle.on('offline', function () {
        SparkplugDeviceBirthed = false
        Log.log(logMod + 'Event: Connection Offline...')
      })

      // Create 'birth' handler
      spClient.handle.on('birth', function () {
        SparkplugPublishQueue.clear() // clear old data
        MqttPublishQueue.clear() // clear old data

        // Publish SCADA HOST BIRTH certificate
        spClient.handle.publishScadaHostBirth()
        // Publish Node BIRTH certificate
        spClient.handle.publishNodeBirth(getNodeBirthPayload(configObj))

        // Publish Device BIRTH certificate
        spClient.handle.publishDeviceBirth(
          jscadaConnection.deviceId,
          sparkplugProcess.deviceBirthPayload
        )
        SparkplugDeviceBirthed = true
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
        Log.log(logMod + 'Topic: ' + topic)
        // Log.log(topicInfo);
        Log.log(logMod + JSON.stringify(payload))

        // sparkplug-b message?
        if (topic.indexOf(SparkplugNS + '/') === 0) {
          let splTopic = topic.split('/')

          let deviceLocator =
            splTopic[1] + '/' + splTopic[3] + '/' + splTopic[4]

          switch (splTopic[2]) {
            case 'DDATA':
              Log.log(
                logMod + 'Device DATA: ' + deviceLocator,
                Log.levelDetailed
              )
              ProcessDeviceBirthOrData(deviceLocator, payload, false)
              break
            case 'NBIRTH':
              Log.log(logMod + 'Node BIRTH', Log.levelDetailed)
              break
            case 'DBIRTH':
              Log.log(
                logMod + 'Device BIRTH: ' + deviceLocator,
                Log.levelDetailed
              )
              ProcessDeviceBirthOrData(deviceLocator, payload, true)
              break
            case 'DDEATH':
              Log.log(logMod + 'Device DEATH', Log.levelDetailed)
              break
            case 'NDEATH':
              Log.log(logMod + 'Node DEATH', Log.levelDetailed)
              break
          }
        }
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

function ProcessDeviceBirthOrData (deviceLocator, payload, isBirth) {
  if (!'metrics' in payload) return
  payload.metrics.forEach(metric => {
    if (metric?.is_historical === true) return

    let objectAddress = null

    if ('name' in metric && metric.name.trim() !== '')
      objectAddress = deviceLocator + '/' + metric.name
    else if ('alias' in metric) {
      let alias =
        (metric.alias.low >>> 0) + (metric.alias.high >>> 0) * Math.pow(2, 32)
      objectAddress = MapAliasToObjectAddress[alias.toString()]
      if (!objectAddress) {
        // alias not mapped
        Log.log(
          'Sparkplug - Unmapped metric alias: ' + alias,
          Log.levelDetailed
        )
        return
      }
    } else {
      Log.log(
        'Sparkplug - Invalid metric (missing name and alias)!',
        Log.levelDetailed
      )
      return
    }

    let value = null
    let valueString = ''
    let valueJson = {}
    let type = 'digital'
    let invalid = true
    let timestamp
    let timestampQualityGood = true
    if ('timestamp' in metric) {
      timestamp = metric.timestamp
    } else {
      timestamp = new Date().getTime()
      timestampQualityGood = false
    }

    if (metric?.is_transient === true) transient = true

    if (metric?.is_null === true) {
      value = 0
      valueString = 'null'
    } else {
      switch (metric.type.toLowerCase()) {
        case 'template':
          return
        case 'dataset':
          value = 0
          valueJson = metric.value
          valueString = JSON.stringify(metric.value)
          return
        case 'boolean':
          value = metric.value === false ? 0 : 1
          valueString = metric.value.toString()
          valueJson = metric
          type = 'digital'
          break
        case 'string':
          value = parseFloat(metric.value)
          if (isNaN(value)) value = 0
          valueString = metric.value
          valueJson = metric
          // try to parse value as JSON
          try {
            valueJson = JSON.parse(metric.value)
          } catch (e) {}
          type = 'string'
          break
        case 'int32':
        case 'uint32':
        case 'float':
        case 'double':
          value = metric.value
          valueString = value.toString()
          valueJson = metric
          type = 'analog'
          break
        case 'int64':
        case 'uint64':
          value =
            (metric.value.low >>> 0) +
            (metric.value.high >>> 0) * Math.pow(2, 32)
          valueString = value.toString()
          valueJson = metric
          type = 'analog'
          break
      }
    }

    if ('properties' in metric) {
      if ('qualityIsGood' in metric.properties) {
        invalid = !metric.properties.qualityIsGood.value
      }
      if ('timestampQualityIsGood' in metric.properties) {
        timestampQualityGood = !metric.properties.timestampQualityIsGood.value
      }
    }

    let catalogProperties = {}
    if (isBirth) {
      DevicesList[deviceLocator] = {
        birth: true,
        metrics: payload.metrics
      }

      // map alias to object address for later query
      if ('alias' in metric) {
        let alias =
          (metric.alias.low >>> 0) + (metric.alias.high >>> 0) * Math.pow(2, 32)
        MapAliasToObjectAddress[alias.toString()] = objectAddress
      }

      let description = ''
      let ungroupedDescription = ''
      if ('metadata' in metric) {
        if ('description' in metric.metadata) {
          description = metric.metadata.description
        }
      }
      if ('properties' in metric) {
        if ('description' in metric.properties) {
          description = metric.properties.description.value
        }
        if ('ungroupedDescription' in metric.properties) {
          ungroupedDescription = metric.properties.ungroupedDescription.value
        }
      }

      catalogProperties = {
        type: type,
        description: description,
        ungroupedDescription: ungroupedDescription,
        group1: metric?.properties?.group1?.value,
        group2: metric?.properties?.group2?.value,
        group3: metric?.properties?.group3?.value,
        commissioningRemarks:
          'Auto created by Sparkplug B driver - ' + new Date().toISOString()
      }
    }
    ValuesQueue.enqueue({
      protocolSourceObjectAddress: objectAddress,
      value: value,
      valueString: valueString,
      valueJson: valueJson,
      invalid: invalid,
      transient: metric?.is_transient === true,
      causeOfTransmissionAtSource: isBirth ? '20' : '3',
      timeTagAtSource: new Date(timestamp),
      timeTagAtSourceOk: timestampQualityGood,
      asduAtSource: metric.type,
      ...catalogProperties
    })
  })
}
