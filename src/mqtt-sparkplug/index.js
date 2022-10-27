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

const RJSON = require('relaxed-json')
const { JSONPath } = require('jsonpath-plus')
const { VM } = require('vm2')
const Streamifier = require('streamifier')
const SparkplugClient = require('./sparkplug-client')
const Fs = require('fs')
const { MongoClient, GridFSBucket, Double, ReadPreference } = require('mongodb')
const Grid = require('gridfs-stream')
const Queue = require('queue-fifo')
const { setInterval } = require('timers')
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')
const LoadConfig = require('./load-config')
const Redundancy = require('./redundancy')
const AutoTag = require('./auto-tag')
const { timeEnd } = require('console')
const { castSparkplugValue: castSparkplugValue } = require('./cast')

const SparkplugNS = 'spBv1.0'
const DevicesList = []
const ValuesQueue = new Queue() // queue of values to update acquisition
const SparkplugPublishQueue = new Queue() // queue of values to publish as Sparkplug-B
let SparkplugDeviceBirthed = false
let AutoCreateTags = true

;(async () => {
  const jsConfig = LoadConfig() // load and parse config file
  Log.levelCurrent = jsConfig.LogLevel
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
  let rtCollection = null
  let cmdCollection = null
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
        SparkplugPublishQueue.dequeue()

        // publish as normal topics
        if (
          connection.publishTopicRoot &&
          connection.publishTopicRoot.trim() !== '' &&
          'properties' in data
        ) {
          // publish value under root/group1/group2/group3/name
          if ('topic' in data.properties)
            SparkplugClientObj.handle.client.publish(
              data.properties.topic.value,
              data.value.toString(),
              { retain: true }
            )

          // publish under root/@jsons-scada/tags/group1/tags/value: value, timestamp, good (value quality), ...
          SparkplugClientObj.handle.client.publish(
            data.properties.topicAsTag.value,
            JSON.stringify({
              value: data.value,
              valueString: data?.properties?.valueString?.value,
              valueJson: data?.properties?.valueJson?.value,
              type: data?.type,
              ...('timestamp' in data ? { timestamp: data.timestamp } : {}),
              ...('good' in data.properties
                ? { good: data.properties.good.value }
                : {})
            }),
            { retain: true }
          )

          // remove (static) properties to avoid publishing to sparkplug
          delete data.properties.topic
          delete data.properties.topicAsTag
        }

        // aggregate for sparkplug publishing
        if (connection.groupId && connection.groupId.trim() !== '') {
          if (!('name' in data))
            // do not publish initial device metrics (they have name property)
            metrics.push(data)
        }
        cnt++
      }

      // publish metrics as sparkplug b device data
      if (metrics.length > 0) {
        let payload = {
          timestamp: new Date().getTime(),
          metrics: metrics
        }
        if (Log.levelCurrent >= Log.levelDetailed)
          Log.log(
            'Sparkplug - Publish - ' + JSON.stringify(payload),
            Log.levelDetailed
          )
        SparkplugClientObj.handle.publishDeviceData(
          connection.deviceId,
          payload,
          { compress: AppDefs.SPARKPLUG_COMPRESS_DDATA }
        )
        Log.log('Sparkplug - Publish metrics updates: ' + cnt, Log.levelNormal)
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
    processMongoUpdates(clientMongo, rtCollection, jsConfig)
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
        rtCollection = db.collection(jsConfig.RealtimeDataCollectionName)
        cmdCollection = db.collection(jsConfig.CommandsQueueCollectionName)

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
          rtCollection,
          jsConfig
        )
        Log.log('Auto Key - Initial value: ' + autoKeyId)

        Redundancy.Start(5000, clientMongo, db, jsConfig)

        // start a changestream monitor on realtimeData only if configured some MQTT publishing
        if (
          (connection.publishTopicRoot &&
            connection.publishTopicRoot.length > 0) ||
          (connection.groupId && connection.groupId.length > 0)
        ) {
          const changeStream = rtCollection.watch(csPipeline, {
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
              // do not queue data changes until device connected and sparkplug birthed
              if (
                !SparkplugClientObj?.handle?.connected ||
                (connection.groupId &&
                  connection.groupId.trim() !== '' &&
                  !SparkplugDeviceBirthed)
              )
                return

              let data = getMetricPayload(change.fullDocument, connection)
              if (data) SparkplugPublishQueue.enqueue(data)
            })
          } catch (e) {
            Log.log('MongoDB - CS Error: ' + e, Log.levelMin)
          }
        }

        if (connection.commandsEnabled) {
          const csCmdPipeline = [
            {
              $project: { documentKey: false }
            },
            {
              $match: {
                $and: [
                  {
                    'fullDocument.protocolSourceConnectionNumber': {
                      $eq: connection.protocolConnectionNumber
                    }
                  },
                  { operationType: 'insert' }
                ]
              }
            }
          ]

          const changeStreamCmd = cmdCollection.watch(csCmdPipeline, {
            fullDocument: 'updateLookup'
          })
          try {
            changeStreamCmd.on('error', change => {
              if (clientMongo) clientMongo.close()
              clientMongo = null
              Log.log('MongoDB - Error on ChangeStream Cmd!')
            })
            changeStreamCmd.on('close', change => {
              if (clientMongo) clientMongo.close()
              clientMongo = null
              Log.log('MongoDB - Closed ChangeStream Cmd!')
            })
            changeStreamCmd.on('end', change => {
              if (clientMongo) clientMongo.close()
              clientMongo = null
              Log.log('MongoDB - Ended ChangeStream Cmd!')
            })

            // start listen to changes
            changeStreamCmd.on('change', change => {
              // do not queue data changes until device connected and birthed
              if (
                !SparkplugDeviceBirthed ||
                !SparkplugClientObj.handle.connected
              )
                return

              if (
                change.fullDocument?.protocolSourceConnectionNumber !==
                connection.protocolConnectionNumber
              )
                return // not for this connection

              let data = getMetricCommandPayload(change.fullDocument)
              if (!data) return

              if (data.deviceId) {
                data.metric.timestamp = new Date(
                  change.fullDocument.timeTag
                ).getTime()
                SparkplugClientObj.handle.publishDeviceCmd(
                  data.groupId,
                  data.edgeNodeId,
                  data.deviceId,
                  {
                    timestamp: new Date(change.fullDocument.timeTag).getTime(),
                    metrics: [data.metric]
                  },
                  {},
                  err => {
                    if (!err) {
                      cmdCollection.updateOne(
                        { _id: change.fullDocument._id },
                        { $set: { ack: true, timeTag: new Date() } }
                      )
                    } else {
                      cmdCollection.updateOne(
                        { _id: change.fullDocument._id },
                        { $set: { ack: false, timeTag: new Date() } }
                      )
                      Log.log(
                        'Sparkplug Command Error, Tag: ' + change.fullDocument.tag
                      )
                    }
                  }
                )
              } else if (data.groupId) {
                data.metric.timestamp = new Date(
                  change.fullDocument.timeTag
                ).getTime()
                SparkplugClientObj.handle.publishNodeCmd(
                  data.groupId,
                  data.edgeNodeId,
                  {
                    timestamp: new Date(change.fullDocument.timeTag).getTime(),
                    metrics: [data.metric]
                  },
                  {},
                  err => {
                    if (!err) {
                      cmdCollection.updateOne(
                        { _id: change.fullDocument._id },
                        { $set: { ack: true, timeTag: new Date() } }
                      )
                    } else {
                      cmdCollection.updateOne(
                        { _id: change.fullDocument._id },
                        { $set: { ack: false, timeTag: new Date() } }
                      )
                      Log.log(
                        'Sparkplug Command Error, Tag: ' + change.fullDocument.tag
                      )
                    }
                  }
                )
              } else {
                let qos = 0,
                  retain = false
                if (!isNaN(change.fullDocument?.protocolSourceCommandDuration))
                  qos = parseInt(
                    change.fullDocument.protocolSourceCommandDuration
                  )
                if (
                  typeof change.fullDocument?.protocolSourceCommandUseSBO ===
                  'boolean'
                )
                  retain = change.fullDocument.protocolSourceCommandUseSBO
                SparkplugClientObj.handle.client.publish(
                  data.topic,
                  data.value.toString(),
                  { qos: qos, retain: retain },
                  err => {
                    if (!err) {
                      cmdCollection.updateOne(
                        { _id: change.fullDocument._id },
                        { $set: { ack: true, timeTag: new Date() } }
                      )
                    } else {
                      cmdCollection.updateOne(
                        { _id: change.fullDocument._id },
                        { $set: { ack: false, timeTag: new Date() } }
                      )
                      Log.log(
                        'MQTT Command Error, Tag: ' + change.fullDocument.tag
                      )
                    }
                  }
                )
              }
            })
          } catch (e) {
            Log.log('MongoDB - CS CMD Error: ' + e, Log.levelMin)
          }
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
        name: 'Node Control/Next Server',
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
async function getDeviceBirthPayload (
  rtCollection,
  commandsEnabled,
  connectionNumber,
  jscadaConnection
) {
  let res = await rtCollection
    .find(
      {
        protocolSourceConnectionNumber: { $ne: connectionNumber }, // exclude data from the same connection
        ...(commandsEnabled ? {} : { origin: { $ne: 'command' } }),
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
          ungroupedDescription: 1,
          group1: 1,
          group2: 1,
          group3: 1,
          origin: 1,
          protocolSourceConnectionNumber: 1
        }
      }
    )
    .toArray()

  let metrics = []
  res.forEach(element => {
    if (element._id <= 0) {
      // exclude internal system data
      return
    }

    // avoid publishing what is acquired in this same connection
    if (element.protocolSourceConnectionNumber === connectionNumber) {
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
    let timestampGood = false
    if ('timeTagAtSource' in element && element.timeTagAtSource !== null) {
      timestamp = new Date(element.timeTagAtSource).getTime()
      if ('timeTagAtSourceOk' in element)
        timestampGood = element.timeTagAtSourceOk
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

      if (element.group1 && element.group1.trim() !== '')
        topicName += topicStr(element.group1) + '/'
      if (element.group2 && element.group2.trim() !== '')
        topicName += topicStr(element.group2) + '/'
      if (element.group3 && element.group3.trim() !== '')
        topicName += topicStr(element.group3) + '/'
      if (
        element.ungroupedDescription &&
        element.ungroupedDescription.trim() !== ''
      )
        topicName += topicStr(element.ungroupedDescription)
      else topicName += topicStr(element.tag)
      topic = {
        topic: {
          type: 'string',
          value: topicName
        }
      }

      topicName = jscadaConnection.publishTopicRoot.trim() + '/'
      if (element.group1 && element.group1.trim() !== '')
        topicName +=
          AppDefs.TAGS_SUBTOPIC + '/' + topicStr(element.group1) + '/'
      topicName += topicStr(element.tag) + '/value'
      topicAsTag = {
        topicAsTag: {
          type: 'string',
          value: topicName
        }
      }
    }

    let metric = {
      name: element.tag,
      alias: element._id,
      value: value,
      type: type,
      ...(timestamp === false ? {} : { timestamp: timestamp }),

      properties: {
        ...topic,
        ...topicAsTag,
        ...(element.origin === 'command'
          ? {
              isCommand: {
                type: 'boolean',
                value: true
              }
            }
          : {}),
        description: { type: 'string', value: element.description },
        good: {
          type: 'boolean',
          value: element.invalid ? false : true
        },
        ...(timestamp === false
          ? {}
          : {
              timestampGood: {
                type: 'boolean',
                value: timestampGood
              }
            })
      }
    }
    metrics.push(metric)
    return
  })

  return {
    timestamp: new Date().getTime(),
    metrics: metrics
  }
}

// Get command payload
function getMetricCommandPayload (cmd) {
  let value = null

  if (
    typeof cmd.protocolSourceASDU !== 'string' ||
    cmd.protocolSourceASDU.trim() === ''
  )
    return null

  // int, int8, int16, int32, int64, uint8, uint16, uint32, uint64, float, double, boolean, string, datetime
  switch (cmd.protocolSourceASDU.toLowerCase()) {
    case 'int8':
      value = parseInt(cmd.value)
      if (value > Math.pow(2, 7) - 1) value = Math.pow(2, 7) - 1
      else if (value < -Math.pow(2, 7)) value = -Math.pow(2, 7)
      break
    case 'int16':
      value = parseInt(cmd.value)
      if (value > Math.pow(2, 15) - 1) value = Math.pow(2, 15) - 1
      else if (value < -Math.pow(2, 15)) value = -Math.pow(2, 15)
      break
    case 'int32':
      value = parseInt(cmd.value)
      if (value > Math.pow(2, 31) - 1) value = Math.pow(2, 31) - 1
      else if (value < -Math.pow(2, 31)) value = -Math.pow(2, 31)
      break
    case 'int':
    case 'int64':
      value = parseInt(cmd.value)
      break
    case 'uint8':
      value = parseInt(cmd.value)
      if (value > Math.pow(2, 8) - 1) value = Math.pow(2, 8) - 1
      else if (value < 0) value = 0
      break
    case 'uint16':
      value = parseInt(cmd.value)
      if (value > Math.pow(2, 16) - 1) value = Math.pow(2, 16) - 1
      else if (value < 0) value = 0
      break
    case 'uint32':
      value = parseInt(cmd.value)
      if (value > Math.pow(2, 32) - 1) value = Math.pow(2, 32) - 1
      else if (value < 0) value = 0
      break
    case 'uint64':
      value = parseInt(cmd.value)
      if (value < 0) value = 0
      break
    case 'float':
    case 'double':
      value = parseFloat(cmd.value)
      break
    case 'boolean':
      value = cmd.value !== 0
      break
    case 'text':
    case 'string':
      value = cmd?.valueString || cmd.value.toString()
      break
    case 'datetime':
      value = parseInt(Math.abs(cmd.value))
      if (value < 0) value = 0
      break
    default:
      return null
  }

  const splTopic = cmd.protocolSourceObjectAddress.split('/')
  if (splTopic.length === 0) return null

  if (splTopic[0] === SparkplugNS) {
    if (splTopic.length === 5) {
      // DCMD
      return {
        groupId: splTopic[1],
        edgeNodeId: splTopic[2],
        deviceId: splTopic[3],
        metric: {
          name: splTopic[4],
          value: value,
          type: cmd.protocolSourceASDU.toLowerCase(),
          timestamp: new Date().getTime()
        }
      }
    } else if (splTopic.length === 4) {
      // NCMD
      return {
        groupId: splTopic[1],
        edgeNodeId: splTopic[2],
        metric: {
          name: splTopic[3],
          value: value,
          type: cmd.protocolSourceASDU.toLowerCase(),
          timestamp: new Date().getTime()
        }
      }
    }
    return null
  }

  return {
    topic: cmd.protocolSourceObjectAddress,
    value: value
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
  let timestampGood = false
  if ('timeTagAtSource' in element && element.timeTagAtSource !== null) {
    timestamp = new Date(element.timeTagAtSource).getTime()
    timestampGood = element.timeTagAtSourceOk
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

    if (element.group1 && element.group1.trim() !== '')
      topicName += topicStr(element.group1) + '/'
    if (element.group2 && element.group2.trim() !== '')
      topicName += topicStr(element.group2) + '/'
    if (element.group3 && element.group3.trim() !== '')
      topicName += topicStr(element.group3) + '/'
    if (
      element.ungroupedDescription &&
      element.ungroupedDescription.trim() !== ''
    )
      topicName += topicStr(element.ungroupedDescription)
    else topicName += topicStr(element.tag)
    topic = {
      topic: {
        type: 'string',
        value: topicName
      }
    }

    topicName = jscadaConnection.publishTopicRoot.trim() + '/'
    if (element.group1 && element.group1.trim() !== '')
      topicName += AppDefs.TAGS_SUBTOPIC + '/' + topicStr(element.group1) + '/'
    topicName += topicStr(element.tag) + '/value'
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
      valueJson: {
        type: 'string',
        value: JSON.stringify(element?.valueJson || value).replace(
          /^"(.*)"$/,
          '$1'
        )
      },
      valueString: {
        type: 'string',
        value: element.valueString || value.toString()
      },
      good: {
        type: 'boolean',
        value: element.invalid ? false : true
      },
      ...topic,
      ...topicAsTag,
      ...(timestamp === false
        ? {}
        : {
            timestampGood: {
              type: 'boolean',
              value: timestampGood
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
    readPreference: ReadPreference.PRIMARY
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
      ValuesQueue.dequeue()

      // check tag is created, if not found create it
      if (AutoCreateTags) {
        let topicSplit = data.protocolSourceObjectAddress.split('/')
        if (topicSplit.length > 0) data.group2 = topicSplit[0]
        if (topicSplit.length > 1 && topicSplit[0] === SparkplugNS)
          data.group2 = topicSplit[1]
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
        valueAtSource: new Double(parseFloat(data.value)),
        valueStringAtSource: data.valueString,
        valueJsonAtSource: data?.valueJson,
        asduAtSource: data?.asduAtSource,
        causeOfTransmissionAtSource: data?.causeOfTransmissionAtSource,
        timeTagAtSource: data.timeTagAtSource,
        timeTagAtSourceOk: data.timeTagAtSourceOk,
        timeTag: new Date(),
        originator: AppDefs.NAME + '|' + jsConfig.ConnectionNumber,
        invalidAtSource: data.invalid,
        transientAtSource: false,
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
    let certOpts = {}
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
    clean: AppDefs.MQTT_CLEAN_SESSION,
    keepalive: AppDefs.MQTT_CONNECTION_KEEPALIVE,
    connectionTimeout: AppDefs.MQTT_CONNECTION_TIMEOUT,
    serverUrl: connection.endpointURLs[0],
    username: connection?.username || '',
    password: connection?.password || '',
    groupId: connection?.groupId || '',
    edgeNode: connection?.edgeNodeId || '',
    clientId: connection?.clientId || '',
    version: SparkplugNS,
    scadaHostId: connection?.scadaHostId || '', // only if a primary application
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
  sparkplugProcess.mongoClient = mongoClient

  if (jscadaConnection === null || mongoClient === null) {
    sparkplugProcess.db = null
    return
  }
  const logMod = 'MQTT Client - '
  const logModS = 'Sparkplug - '
  const connectionRetriesLimit = 20

  // poor man's local static variables
  if (typeof sparkplugProcess.currentBroker === 'undefined') {
    sparkplugProcess.currentBroker = 0
    sparkplugProcess.connectionRetries = 0
    sparkplugProcess.deviceBirthPayload = null
    sparkplugProcess.db = null
  }

  if (sparkplugProcess.db === null)
    sparkplugProcess.db = mongoClient.db(configObj.mongoDatabaseName)

  if (spClient.handle === null) {
    if (!Redundancy.ProcessStateIsActive())
      // do not connect MQTT while process not active
      return

    try {
      // Create the SparkplugClient
      Log.log(logMod + 'Creating client...')
      let config
      try {
        config = getSparkplugConfig(jscadaConnection)
      } catch (e) {
        Log.log(logMod + 'Parameter error. ' + e.message)
        process.exit(1)
      }

      config.serverUrl =
        jscadaConnection.endpointURLs[sparkplugProcess.currentBroker]
      Log.log(logMod + 'Try connecting to ' + config.serverUrl)

      //// default subscription to all topics from Sparkplug-B
      //if (
      //  !('topics' in jscadaConnection) ||
      //  jscadaConnection.topics.length === 0
      //) {
      //  jscadaConnection.topics = [SparkplugNS + '/#']
      //}

      if (
        !('deviceId' in jscadaConnection) ||
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
      spClient.handle.on('birth', async function () {
        if (
          !('groupId' in jscadaConnection) ||
          jscadaConnection.groupId.trim() === ''
        )
          return

        SparkplugPublishQueue.clear() // clear old data

        // Publish SCADA HOST BIRTH certificate (7.5.1)
        if (
          jscadaConnection.scadaHostId &&
          jscadaConnection.scadaHostId.trim() !== ''
        ) {
          spClient.handle.publishScadaHostBirth()
          Log.log(logMod + 'Publish SCADA Host Birth')
        }

        // Publish Node BIRTH certificate
        let nbc = getNodeBirthPayload(configObj)
        spClient.handle.publishNodeBirth(nbc, {
          compress: AppDefs.SPARKPLUG_COMPRESS_NBIRTH
        })
        Log.log(
          logMod + 'Publish node birth with ' + nbc.metrics.length + ' metrics'
        )

        // obtain device birth payload
        sparkplugProcess.deviceBirthPayload = await getDeviceBirthPayload(
          sparkplugProcess.db.collection(configObj.RealtimeDataCollectionName),
          jscadaConnection.commandsEnabled,
          jscadaConnection.protocolConnectionNumber,
          jscadaConnection
        )
        Log.log(
          logMod +
            'Publish device birth "' +
            jscadaConnection.deviceId +
            '" with ' +
            sparkplugProcess.deviceBirthPayload.metrics.length +
            ' metrics'
        )

        // Publish Device BIRTH certificate
        spClient.handle.publishDeviceBirth(
          jscadaConnection.deviceId,
          sparkplugProcess.deviceBirthPayload,
          { compress: AppDefs.SPARKPLUG_COMPRESS_DBIRTH }
        )
        SparkplugDeviceBirthed = true
        sparkplugProcess.deviceBirthPayload.metrics.forEach(elem => {
          SparkplugPublishQueue.enqueue(elem)
        })
      })

      spClient.handle.on('connect', function () {
        sparkplugProcess.connectionRetries = 0
        Log.log(logMod + 'Event: Connected to broker')
        // Subscribe topics
        jscadaConnection.topics
          .concat(jscadaConnection.topicsAsFiles)
          .forEach(elem => {
            let topicStr = JsonPathTopic(elem).topic

            Log.log(logMod + 'Subscribing topic: ' + topicStr)

            spClient.handle.client.subscribe(topicStr, {
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

      // test for topic matches subscription
      const topicMatchSub = t => s =>
        new RegExp(s.split`+`.join`[^/]+`.split`#`.join`.+`).test(t)

      // A VM to run scripts to extract complex payloads
      let sharedObj = {}
      const sandbox = {
        shared: sharedObj
      }
      const vm = new VM({ sandbox })

      // process non sparkplug b messages
      spClient.handle.on('nonSparkplugMessage', async function (
        topic,
        payload,
        packet
      ) {
        Log.log(
          logMod +
            'Event: Regular MQTT message, topic: ' +
            topic +
            ' size: ' +
            payload.length
        )

        let match = false
        // check for match of some topic subscription to be saved as files
        if (jscadaConnection?.topicsAsFiles instanceof Array)
          await jscadaConnection.topicsAsFiles.forEach(async tp => {
            if (topicMatchSub(topic)(tp)) {
              Log.log(logMod + 'Received topic as file ' + topic)
              match = true
              try {
                // save as file on Mongodb Gridfs
                let gfs = new GridFSBucket(sparkplugProcess.db)

                // delete older files with same name
                let f = await gfs.find({ filename: topic }).toArray()
                f.forEach(async elem => {
                  await gfs.delete(elem._id)
                })

                let writestream = gfs.openUploadStream(topic)
                Streamifier.createReadStream(payload).pipe(writestream)
              } catch (e) {
                Log.log(logMod + 'Error saving file. ' + e.message)
              }
              return
            }
          })

        if (match) return

        if (jscadaConnection?.topicsScripted instanceof Array)
          jscadaConnection.topicsScripted.forEach(elem => {
            if (elem.topic)
              if (topicMatchSub(topic)(elem.topic)) {
                match = true

                /*
              "topicsScripted": [{ 
                 "topic": "C3ET/test/jsonarr", 
                 "script": " // remove comments and put all in the same line
                            shared.dataArray = []; // array of objects to return
                            vals=JSON.parse(shared.payload.toString()); 
                            cnt = 1;
                            vals.forEach(elem => {
                              shared.dataArray.push({'id': 'scrVal'+cnt, 'value': elem, 'qualityOk': true, 'timestamp': (new Date()).getTime() });
                              cnt++;
                            })
                            ret; // return values in array of objects
                           "
                  }]
              */

                if (elem.script) {
                  // make payload (buffer) available inside VM (as info.payload)
                  sharedObj.payload = payload
                  sharedObj.dataArray = []

                  try {
                    // execute script and queue extracted values
                    vm.run(elem.script)

                    if (sharedObj?.dataArray instanceof Array)
                      sharedObj.dataArray.forEach(element => {
                        if (!element.id || !('value' in element)) return
                        let type = 'analog'
                        if (element.type) type = element.type
                        ValuesQueue.enqueue({
                          protocolSourceObjectAddress: topic + '/' + element.id,
                          value: element.value,
                          valueString: element.valueString
                            ? element.valueString
                            : element.value.toString(),
                          valueJson: element.valueJson
                            ? element.valueJson
                            : element.value,
                          invalid: element.qualityOk === false ? true : false,
                          transient: element.transient === true ? true : false,
                          causeOfTransmissionAtSource:
                            'causeOfTransmissionAtSource' in element
                              ? element.causeOfTransmissionAtSource
                              : '3',
                          timeTagAtSource: element.timestamp
                            ? new Date(element.timestamp)
                            : new Date(),
                          timeTagAtSourceOk: element.timestamp ? true : false,
                          asduAtSource: 'scripted',
                          type: type
                        })
                      })
                  } catch (e) {
                    Log.log(
                      logMod +
                        'Error on script on topic ' +
                        topic +
                        ' - ' +
                        e.message
                    )
                  }
                }
                return
              }
          })

        if (match) return

        if (jscadaConnection?.topics instanceof Array)
          jscadaConnection.topics.forEach(elem => {
            if (elem) {
              let jpt = JsonPathTopic(elem)
              if (jpt.jsonPath !== '' && topicMatchSub(topic)(jpt.topic)) {
                // extract value from payload using JSON PATH
                let JsonPayload = TryPayloadAsRJson(payload)
                const jpRes = JSONPath({
                  path: jpt.jsonPath,
                  json: JsonPayload,
                  wrap: false
                })
                EnqueueJsonValue(jpRes, elem)
                match = true
              }
            }
          })

        if (match) return

        // try to detect payload as JSON or RJSON

        if (payload.length > 10000) {
          Log.log(logMod + 'Payload too big!')
          return
        }

        let JsonValue = TryPayloadAsRJson(payload)
        EnqueueJsonValue(JsonValue, topic)
      })

      // process received node commands (for this node)
      spClient.handle.on('ncmd', function (payload) {
        Log.log(
          logModS + 'Received NCMD - ' + JSON.stringify(payload),
          Log.levelDetailed
        )
        if (payload?.metrics instanceof Array) {
          payload.metrics.forEach(async metric => {
            switch (metric?.name) {
              case 'Node Control/Rebirth':
                if (
                  !('groupId' in jscadaConnection) ||
                  jscadaConnection.groupId.trim() === ''
                )
                  return
                if (metric?.value === true) {
                  Log.log(logModS + 'Node Rebirth command received')
                  // Publish Node BIRTH certificate
                  let nbc = getNodeBirthPayload(configObj)
                  spClient.handle.publishNodeBirth(nbc, {
                    compress: AppDefs.SPARKPLUG_COMPRESS_NBIRTH
                  })
                  Log.log(
                    logMod +
                      'Publish node birth with ' +
                      nbc.metrics.length +
                      ' metrics'
                  )
                  // Publish Device BIRTH certificate
                  let dbc = await getDeviceBirthPayload(
                    sparkplugProcess.db.collection(
                      configObj.RealtimeDataCollectionName
                    ),
                    jscadaConnection.commandsEnabled,
                    jscadaConnection.protocolConnectionNumber,
                    jscadaConnection
                  )
                  Log.log(
                    logMod +
                      'Publish device Birth "' +
                      jscadaConnection.deviceId +
                      '" with ' +
                      sparkplugProcess.deviceBirthPayload.metrics.length +
                      ' metrics'
                  )
                  spClient.handle.publishDeviceBirth(
                    jscadaConnection.deviceId,
                    dbc,
                    { compress: AppDefs.SPARKPLUG_COMPRESS_DBIRTH }
                  )
                }
                break
              case 'Node Control/Reboot':
                // only accept Reboot command if not a primary application
                // if (!jscadaConnection.scadaHostId || jscadaConnection.scadaHostId == '')
                {
                  Log.log(
                    logModS + 'Node Reboot command received, exiting process...'
                  )
                  process.exit(999)
                }
                break
              case 'Node Control/Next Server':
                // only accept Next Server command if not a primary application
                //if (!jscadaConnection.scadaHostId || jscadaConnection.scadaHostId == '')
                {
                  Log.log(logMod + 'Node command Next Server received')
                  if (jscadaConnection.endpointURLs.length > 1) {
                    sparkplugProcess.currentBroker =
                      (sparkplugProcess.currentBroker + 1) %
                      jscadaConnection.endpointURLs.length
                    Log.log(
                      logMod +
                        'Will try to connect to server ' +
                        jscadaConnection.endpointURLs[
                          sparkplugProcess.currentBroker
                        ]
                    )
                    // stop and destroy current client
                    spClient.handle.stop()
                    spClient.handle = null
                    sparkplugProcess.connectionRetries = 0
                  } else Log.log(logMod + 'No alternatives servers configured!')
                }
                break
            }
          })
        }
      })

      // process received device commands (for this device)
      spClient.handle.on('dcmd', function (deviceId, payload) {
        if (!jscadaConnection.commandsEnabled) return
        Log.log(
          logModS +
            'Received DCMD - ' +
            deviceId +
            ' - ' +
            JSON.stringify(payload),
          Log.levelDetailed
        )
        // ignore command if mongo disconnected
        if (sparkplugProcess.mongoClient === null) return
        // process each metric on DCMD payload
        if (payload?.metrics instanceof Array) {
          payload.metrics.forEach(metric => {
            ProcessDeviceCommand(
              deviceId,
              metric,
              payload?.timestamp,
              sparkplugProcess.mongoClient,
              jscadaConnection,
              configObj
            )
          })
        }
      })

      // process MQTT Sparkplug B messages (coming from other devices)
      spClient.handle.on('message', function (topic, payload, topicInfo) {
        payload.metrics = payload?.metrics // null check filter
          ?.filter(
            metric => !(metric?.type === undefined || metric?.type === null)
          )
        Log.log(logModS + 'Event: Sparkplug B message on topic: ' + topic)

        if (Log.levelCurrent >= Log.levelDetailed) {
          //Log.log(logModS + JSON.stringify(topicInfo), Log.levelDetailed)
          Log.log(logModS + JSON.stringify(payload), Log.levelDetailed)
        }

        let splTopic = topic.split('/')
        if (splTopic.length < 4) {
          // invalid topic
          Log.log(logModS + 'Invalid topic')
        }
        let deviceLocator = splTopic[0] + '/' + splTopic[1] + '/' + splTopic[3]
        if (splTopic.length > 4) deviceLocator += '/' + splTopic[4]

        if (splTopic.length)
          switch (splTopic[2]) {
            case 'NCMD': // commands for other nodes
              break
            case 'DCMD': // commands for other devices
              break
            case 'NDATA':
              // edge of node data (7.2)
              break
            case 'DDATA':
              // device data, update metrics (7.4)
              Log.log(
                logModS + 'Device DATA: ' + deviceLocator,
                Log.levelDetailed
              )

              // data from not birthed device?
              if (
                !(deviceLocator in DevicesList) ||
                !DevicesList[deviceLocator].birthed
              ) {
                Log.log(
                  logModS + 'Data from not yet birthed device: ' + deviceLocator
                )
                Log.log(logModS + 'Requesting node rebirth...')
                spClient.handle.publishNodeCmd(
                  topicInfo.groupId,
                  topicInfo.edgeNodeId,
                  {
                    timestamp: new Date().getTime(),
                    metrics: [
                      {
                        name: 'Node Control/Rebirth',
                        timestamp: new Date().getTime(),
                        type: 'Boolean',
                        value: true
                      }
                    ]
                  }
                )
                return
              }
              ProcessDeviceBirthOrData(deviceLocator, payload, false)
              break
            case 'NBIRTH':
              // on node birth all associated data is invalidated (7.1.2)
              Log.log(logModS + 'Node BIRTH', Log.levelDetailed)
              if (sparkplugProcess.mongoClient)
                InvalidateDeviceTags(
                  deviceLocator,
                  sparkplugProcess.mongoClient,
                  jscadaConnection,
                  configObj
                )
              break
            case 'DBIRTH':
              // device birth, create tags and update metrics (7.3.1)
              Log.log(
                logModS + 'Device BIRTH: ' + deviceLocator,
                Log.levelDetailed
              )
              ProcessDeviceBirthOrData(deviceLocator, payload, true)
              break
            case 'NDEATH':
              // Node death, invalidate all Sparkplug metrics from this node (7.1.1)
              Log.log(logModS + 'Node DEATH', Log.levelDetailed)
              // devices from this node marked as dead
              DevicesList.forEach(function (element, key) {
                if (key.indexOf(deviceLocator) === 0)
                  DevicesList[key].birthed = false
              })
              if (sparkplugProcess.mongoClient)
                InvalidateDeviceTags(
                  deviceLocator,
                  sparkplugProcess.mongoClient,
                  jscadaConnection,
                  configObj
                )
              break
            case 'DDEATH':
              // Node death, invalidate all Sparkplug metrics from this device (7.3.2)
              Log.log(logModS + 'Device DEATH', Log.levelDetailed)
              if (deviceLocator in DevicesList)
                DevicesList[deviceLocator].birthed = false
              if (sparkplugProcess.mongoClient)
                InvalidateDeviceTags(
                  deviceLocator,
                  sparkplugProcess.mongoClient,
                  jscadaConnection,
                  configObj
                )
              break
          }
      })
    } catch (e) {
      Log.log(logModS + 'Error: ' + e.message, Log.levelMin)
    }
  } else {
    // MQTT client is already created

    if (Redundancy.ProcessStateIsActive()) {
      // test connection is established

      Log.log(
        logMod +
          (spClient.handle.connected
            ? 'Currently Connected'
            : 'Currently Disconnected'),
        Log.levelDetailed
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
}

function ProcessDeviceBirthOrData (deviceLocator, payload, isBirth) {
  if (!('metrics' in payload)) return

  if (isBirth) {
    Log.log('Sparkplug - New device: ' + deviceLocator)
    DevicesList[deviceLocator] = {
      birthed: true,
      mapAliasToObjectAddress: [],
      metrics: payload.metrics
    }
  }

  // extract metrics info and queue for tags updates on mongodb
  payload.metrics.forEach(metric => {
    queueMetric(metric, deviceLocator, isBirth)
  })
}

// obtain information from sparkplug-b decoded payload and queue for mongo tag updates
function queueMetric (metric, deviceLocator, isBirth, templateName) {
  if (metric?.isHistorical === true) return // when historical, discard
  if (metric?.isTransient === true) return // when transient, discard

  let value = 0,
    valueString = '',
    valueJson = {},
    type = 'digital',
    invalid = false,
    isNull = false,
    timestamp,
    timestampGood = true,
    catalogProperties = {},
    objectAddress = null

  //if (typeof queueMetric.MapAliasToObjectAddress === 'undefined')
  //  queueMetric.MapAliasToObjectAddress = []

  if ('name' in metric && metric.name.trim() !== '') {
    // when metric is from a template, add template name to object address
    if (templateName && templateName.trim() !== '')
      objectAddress =
        deviceLocator + '/' + topicStr(templateName) + '/' + metric.name
    else objectAddress = deviceLocator + '/' + topicStr(metric.name)

    // map alias to object address for later query
    if ('alias' in metric) {
      let alias = metric.alias
      let device = DevicesList[deviceLocator]
      if (!device) {
        // device not yet included
        return
      }
      device.mapAliasToObjectAddress['a' + alias.toString()] = objectAddress
    }
  } else if ('alias' in metric) {
    let alias = metric.alias
    let device = DevicesList[deviceLocator]
    if (device)
      objectAddress =
        DevicesList[deviceLocator].mapAliasToObjectAddress[
          'a' + alias.toString()
        ]
    if (!objectAddress) {
      // alias not mapped
      Log.log(
        'Sparkplug - Unmapped metric alias=' +
          alias.toString() +
          ' device=' +
          deviceLocator,
        Log.levelDetailed
      )
      return false
    }
  } else {
    Log.log(
      'Sparkplug - Invalid metric (missing name and alias)!',
      Log.levelDetailed
    )
    return false
  }

  if ('timestamp' in metric) {
    timestamp = metric.timestamp
  } else {
    timestamp = new Date().getTime()
    timestampGood = false
  }

  if (
    metric?.value === null ||
    metric?.isNull === true ||
    !('value' in metric)
  ) {
    // when value is absent, consider it invalid
    invalid = true
    isNull = true
  }

  switch (metric.type.toLowerCase()) {
    case 'template':
      type = 'json'
      if ('value' in metric) {
        // recurse to publish more metrics
        if ('metrics' in metric.value) {
          metric.value.metrics.forEach(m => {
            queueMetric(m, deviceLocator, isBirth, metric.name)
          })
          return
        } else {
          valueJson = metric.value
          valueString = JSON.stringify(metric.value)
        }
      }
      break
    case 'dataset':
      // transform data set in a simpler array of objects with named properties
      type = 'json'
      if ('value' in metric) {
        if ('numOfColumns' in metric.value) {
          let v = []
          for (let j = 0; j < metric.value.rows.length; j++) {
            let r = {}
            for (let i = 0; i < metric.value.numOfColumns; i++) {
              let mv = castSparkplugValue(metric.value.types[i], metric.value.rows[j][i])
              switch (metric.value.types[i].toLowerCase()) {
                case 'int64':
                case 'uint64':
                  mv = mv.toNumber() // warning number may be truncated
                  break
                default:
                  break
              }
              r[metric.value.columns[i]] = mv
            }
            v.push(r)
          }
          valueJson = v
          valueString = JSON.stringify(v)
        } else {
          valueJson = metric.value
          valueString = JSON.stringify(metric.value)
        }
      }
      break
    case 'boolean':
      type = 'digital'
      if (!('value' in metric) || metric.value === null) {
        // metric does not have a value
        value = 0
        valueString = 'false'
        valueJson = false
      } else {
        // metric does have a value
        value = metric.value === false ? 0 : 1
        valueString = metric.value.toString()
        valueJson = metric
      }
      break
    case 'string':
      type = 'string'
      if (!('value' in metric) || metric.value === null) {
        // metric does not have a value
        value = 0
        valueString = ''
        valueJson = ''
      } else {
        // metric does have a value
        value = parseFloat(metric.value)
        if (isNaN(value)) value = 0
        valueString = metric.value
        valueJson = metric
        // try to parse value as JSON
        try {
          valueJson = JSON.parse(metric.value)
        } catch (e) {}
      }
      break
    case 'int8':
    case 'uint8':
    case 'int16':
    case 'uint16':
    case 'int32':
    case 'uint32':
    case 'float':
    case 'double':
      type = 'analog'
      if (!('value' in metric) || metric.value === null) {
        // metric does not have a value
        value = 0
        valueString = '0'
        valueJson = 0
      } else {
        // metric does have a value
        value = castSparkplugValue(metric.type, metric.value)
        valueString = value.toString()
        valueJson = metric
      }
      break
    case 'int64':
    case 'uint64':
      type = 'analog'
      if (!('value' in metric) || metric.value === null) {
        // metric does not have a value
        value = 0
        valueString = '0'
        valueJson = 0
      } else {
        // metric does have a value
        value = castSparkplugValue(metric.type, metric.value)
        value = value.toNumber() // warning number may be truncated
        valueString = value.toString()
        valueJson = metric
      }
      break
    // case 'datetime': // TODO ??
  }

  if ('properties' in metric) {
    if ('good' in metric.properties) {
      invalid = !metric.properties.good.value
    }
    if ('timestampGood' in metric.properties) {
      timestampGood = !metric.properties.timestampGood.value
    }
  }

  if (isBirth) {
    catalogProperties = {
      description: '',
      ungroupedDescription: '',
      group1: '',
      group2: '',
      group3: '',
      type: type
    }

    if ('metadata' in metric && 'description' in metric.metadata)
      catalogProperties.description = metric.metadata.description

    if ('properties' in metric) {
      if ('description' in metric.properties) {
        catalogProperties.description = metric.properties.description.value
      }
      catalogProperties.ungroupedDescription =
        metric.properties?.ungroupedDescription?.value || ''
      catalogProperties.group1 = metric.properties?.group1?.value || ''
      catalogProperties.group2 = metric.properties?.group2?.value || ''
      catalogProperties.group3 = metric.properties?.group3?.value || ''
      if ('engUnit' in metric.properties)
        catalogProperties.unit = metric.properties.engUnit?.value || 'units'
    }
    catalogProperties.commissioningRemarks =
      'Auto created by Sparkplug B driver - ' + new Date().toISOString()
  }

  ValuesQueue.enqueue({
    protocolSourceObjectAddress: objectAddress,
    value: value,
    valueString: valueString,
    valueJson: valueJson,
    invalid: invalid,
    causeOfTransmissionAtSource: isBirth ? '20' : '3',
    timeTagAtSource: new Date(timestamp),
    timeTagAtSourceOk: timestampGood,
    asduAtSource: type,
    isNull: metric?.isNull === true,
    ...catalogProperties
  })
}

// Process received Sparkplug B command to possible protocol destinations (routed command)
async function ProcessDeviceCommand (
  deviceId,
  metric,
  timestamp,
  mongoClient,
  jscadaConnection,
  configObj
) {
  if (!mongoClient) return
  const db = mongoClient.db(configObj.mongoDatabaseName)
  const rtCollection = db.collection(configObj.RealtimeDataCollectionName)
  const cmdQueue = db.collection(configObj.CommandsQueueCollectionName)

  Log.log(
    'Sparkplug - Received command: ' +
      deviceId +
      '/' +
      metric.name +
      ' value:' +
      metric.value
  )

  let res = await rtCollection
    .find(
      {
        tag: metric.name,
        origin: 'command'
      },
      {
        projection: {
          _id: 1,
          tag: 1,
          type: 1,
          description: 1,
          origin: 1,
          kconv1: 1,
          kconv2: 1,
          protocolSourceConnectionNumber: 1,
          protocolSourceObjectAddress: 1,
          protocolSourceCommonAddress: 1,
          protocolSourceASDU: 1,
          protocolSourceCommandDuration: 1,
          protocolSourceCommandUseSBO: 1
        }
      }
    )
    .toArray()

  res.forEach(async element => {
    if (element.origin !== 'command' || element._id <= 0) {
      return
    }

    if (
      element.protocolSourceConnectionNumber ===
      jscadaConnection.protocolConnectionNumber
    ) {
      Log.log(
        'Sparkplug - Discarding received command on the same driver connection: ' +
          jscadaConnection.protocolConnectionNumber
      )
    }

    let value = parseFloat(metric.value)
    if (isNaN(value)) value = 0
    let valueString = ''
    let valueJson = {}

    if ('type' in metric)
      switch (metric.type.toLowerCase()) {
        case 'boolean':
          if (element.kconv1 === -1) value = metric.value ? 0 : 1
          else value = metric.value ? 1 : 0
          valueString = value ? 'true' : 'false'
          valueJson = value ? true : false
          break
        case 'string':
          valueString = metric.value
          // try to parse value as JSON
          try {
            valueJson = JSON.parse(metric.value)
          } catch (e) {}
          break
        case 'int8':
        case 'int16':
        case 'uint8':
        case 'uint16':
        case 'int32':
        case 'uint32':
        case 'float':
        case 'double':
          value = castSparkplugValue(metric.type, metric.value)
          value = element.kconv1 * value + element.kconv2
          valueString = value.toString()
          valueJson = value
          break
        case 'int64':
        case 'uint64':
          value = castSparkplugValue(metric.type, metric.value)
          value = value.mul(element.kconv1).add(element.kconv2) // safe Long object
          value = value.toNumber() // warning unsafe number
          valueString = value.toString()
          valueJson = value
          break
        // case 'datetime': 
        default:
          valueString = JSON.stringify(metric)
          valueJson = metric
          break
      }
    else {
      Log.log('Sparkplug - Invalid command!')
      return
    }

    // insert command on commandsQueue collection

    let cmd = {
      protocolSourceConnectionNumber: element.protocolSourceConnectionNumber,
      protocolSourceCommonAddress: element.protocolSourceCommonAddress,
      protocolSourceObjectAddress: element.protocolSourceObjectAddress,
      protocolSourceASDU: element.protocolSourceASDU,
      protocolSourceCommandDuration: element.protocolSourceCommandDuration,
      protocolSourceCommandUseSBO: element.protocolSourceCommandUseSBO,
      pointKey: new Double(element._id),
      tag: element.tag,
      value: new Double(value),
      valueString: valueString,
      valueJson: valueJson,
      originatorUserName: jscadaConnection.name,
      originatorIpAddress:
        jscadaConnection.endpointURLs[sparkplugProcess.currentBroker],
      timeTag: new Date()
    }

    let rIns = await cmdQueue.insertOne(cmd)
    // if (rIns.acknowledged) // change for mongo driver >= 4.0 
    if (rIns.insertedCount)
      Log.log(
        'MongoDB - Command Queued: ' + JSON.stringify(cmd),
        Log.levelDetailed
      )
  })
}

// replace invalid topic name chars
function topicStr (s) {
  if (typeof s === 'string')
    return s
      .trim()
      .replace(/\//g, '|')
      .replace(/\+/g, '^')
      .replace(/\#/g, '@')
  return ''
}

// invalidate tags from device or node based on Sparkplug B topic path
function InvalidateDeviceTags (
  deviceTopicPath,
  mongoClient,
  jscadaConnection,
  configObj
) {
  if (!mongoClient) return
  try {
    Log.log('MongoDB - Invalidate tags from ' + deviceTopicPath)
    const db = mongoClient.db(configObj.mongoDatabaseName)
    const rtCollection = db.collection(configObj.RealtimeDataCollectionName)
    rtCollection.updateMany(
      {
        protocolSourceConnectionNumber:
          jscadaConnection.protocolConnectionNumber,
        protocolSourceObjectAddress: { $regex: '^' + deviceTopicPath }
      },
      { $set: { invalid: true } }
    )
  } catch (e) {
    Log.log(
      'MongoDB - Error invalidating tags from ' +
        deviceTopicPath +
        ' - ' +
        e.message
    )
  }
}

// extract topic and jsonPath from topic/jsonPath
// the last level must begin with $
// E.g. /root/topicname/$.var1 topic=/root/topicname jsonPath=$.var1
// If do not found a jsonPath in the last level, return topic and empty string for jsonPath
function JsonPathTopic (jpTopic) {
  let jsonPath = ''
  let topic = jpTopic.trim()

  const pos = topic.indexOf('/$.')
  if (pos !== -1) {
    jsonPath = topic.substring(pos + 1, topic.length)
    topic = topic.substring(0, pos)
  }

  return {
    topic: topic,
    jsonPath: jsonPath
  }
}

// convert possible JSON value to number and string and enqueue for mongo update
function EnqueueJsonValue (JsonValue, protocolSourceObjectAddress) {
  let value = 0,
    valueString = '',
    type = 'json'
  switch (typeof JsonValue) {
    case 'boolean':
      type = 'digital'
      value = JsonValue ? 1 : 0
      valueString = JsonValue.toString()
      break
    case 'number':
      type = 'analog'
      value = JsonValue
      valueString = JsonValue.toString()
      break
    case 'string':
      type = 'string'
      value = parseFloat(JsonValue)
      valueString = JsonValue
      break
    default:
      if (JsonValue === null) JsonValue = {}
      else valueString = JSON.stringify(JsonValue)
      break
  }

  if (isNaN(value)) value = 0
  if (JsonValue === null) JsonValue = {}

  ValuesQueue.enqueue({
    protocolSourceObjectAddress: protocolSourceObjectAddress,
    value: value,
    valueString: valueString,
    valueJson: JsonValue,
    invalid: false,
    transient: false,
    causeOfTransmissionAtSource: '3',
    timeTagAtSource: new Date(),
    timeTagAtSourceOk: false,
    asduAtSource: typeof JsonValue,
    type: type
  })
}

function TryPayloadAsRJson (payload) {
  let payloadStr = ''
  let JsonValue = null
  try {
    payloadStr = payload.toString()
    // try to parse as regular JSON
    JsonValue = JSON.parse(payloadStr)
  } catch (e) {
    // NOT STRICT JSON, try RJSON
    try {
      JsonValue = RJSON.parse(payloadStr)
    } catch (e) {
      // NOT JSON NOR RJSON, consider as string
      JsonValue = payloadStr
    }
  }
  return JsonValue
}
