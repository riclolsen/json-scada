/*
 * MQTT-Sparkplug B Client Driver for JSON-SCADA
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

const zlib = require('zlib')
const RJSON = require('relaxed-json')
const { JSONPath } = require('jsonpath-plus')
const vm = require('vm')
const Streamifier = require('streamifier')
const SparkplugClient = require('./sparkplug-client')
const Fs = require('fs')
const { MongoClient, GridFSBucket, Double } = require('mongodb')
const Queue = require('queue-fifo')
const { setInterval } = require('timers')
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')
const LoadConfig = require('./load-config')
const Redundancy = require('./redundancy')
const AutoTag = require('./auto-tag')
const { castSparkplugValue: castSparkplugValue } = require('./cast')
const {
  SparkplugNS,
  topicMatchSub,
  jsonPathTopic: JsonPathTopic,
  topicStr,
  isSameOrChildPath,
  pathRegex,
  parseCommandAddress,
  toNumber,
  nextSeq,
} = require('./sparkplug-utils')

// The process is kept alive on unexpected errors (a crash/restart loop could
// be triggered remotely by a retained message); log the full stack instead.
process.on('uncaughtException', (err) =>
  Log.log('Uncaught Exception: ' + (err?.stack || err), Log.levelMin)
)
process.on('unhandledRejection', (reason) =>
  Log.log('Unhandled Rejection: ' + (reason?.stack || reason), Log.levelMin)
)

const DevicesList = {} // contains either EoN nodes or devices
const NodeStates = {} // by node locator: last received seq and NBIRTH bdSeq
const RebirthRequests = {} // by node locator: time of last rebirth request
const ValuesQueue = new Queue() // queue of values to update acquisition
const SparkplugPublishQueue = new Queue() // queue of values to publish as Sparkplug-B
const BirthAliases = new Set() // aliases declared in the last published DBIRTH
const SCRIPT_TIMEOUT_MS = 1000 // limit for each topicsScripted script run
let SparkplugDeviceBirthed = false
let PendingDeviceRebirth = false // a changed tag is missing from the DBIRTH
let PublishingBirths = false
let LastBirthTime = 0
let AutoCreateTags = true
const MongoStatus = { HintMongoIsConnected: false }
let LastNodeRebirth = 0
let BdSeq = 0 // birth/death sequence of the next MQTT session
let MongoUpdatesRunning = false

;(async () => {
  const jsConfig = LoadConfig() // load and parse config file
  Log.levelCurrent = jsConfig.LogLevel
  // The $match does not reference fullDocument, so the server applies it
  // before the updateLookup and skips the document read for raw driver writes
  // (sourceDataUpdate) and the special points; $project goes after it.
  const csPipeline = [
    {
      $match: {
        $or: [
          {
            $and: [
              {
                'updateDescription.updatedFields.sourceDataUpdate': {
                  $exists: false,
                },
              },
              { 'documentKey._id': { $nin: [-2, -1] } },
              { operationType: 'update' },
            ],
          },
          { operationType: 'replace' },
        ],
      },
    },
    {
      $project: { documentKey: false },
    },
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
              String(data.value),
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
              ...('quality' in data ? { quality: data.quality } : {}),
              ...('good' in data.properties ?
                { good: data.properties.good.value }
              : {}),
            }),
            { retain: true }
          )

          // remove (static) properties to avoid publishing to sparkplug
          delete data.properties.topic
          delete data.properties.topicAsTag
        }

        // aggregate for sparkplug publishing
        // (initial device metrics have the name property and are not republished)
        if (
          connection.groupId &&
          connection.groupId.trim() !== '' &&
          !('name' in data)
        ) {
          // DDATA may only carry aliases declared in the DBIRTH; a new tag
          // needs a rebirth first
          if (BirthAliases.has(data.alias)) metrics.push(data)
          else PendingDeviceRebirth = true
        }
        cnt++
      }

      if (
        PendingDeviceRebirth &&
        SparkplugDeviceBirthed &&
        rtCollection &&
        new Date().getTime() - LastBirthTime >
          AppDefs.SECONDS_BETWEEN_REBIRTHS * 1000
      ) {
        Log.log('Sparkplug - New tags to publish, rebirth', Log.levelNormal)
        publishBirths(
          SparkplugClientObj.handle,
          rtCollection,
          connection,
          jsConfig
        ).catch((err) => Log.log('Sparkplug - Rebirth error: ' + err))
      }

      // publish metrics as sparkplug b device data
      if (metrics.length > 0) {
        let payload = {
          timestamp: new Date().getTime(),
          metrics: metrics,
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
    processMongoUpdates(clientMongo, rtCollection, jsConfig, MongoStatus)
  }, 500)

  // connection stats: stats.isConnected tells whether the broker session is
  // up (read by the data processor's communication status tags). Written by
  // the active node only, so a standby never reports the link as down.
  setInterval(async function () {
    if (
      !clientMongo ||
      !MongoStatus.HintMongoIsConnected ||
      !connection ||
      !Redundancy.ProcessStateIsActive()
    )
      return
    try {
      await updateConnectionStats(
        clientMongo
          .db(jsConfig.mongoDatabaseName)
          .collection(jsConfig.ProtocolConnectionsCollectionName),
        connection.protocolConnectionNumber,
        {
          nodeName: jsConfig.nodeName,
          timeTag: new Date(),
          isConnected: !!SparkplugClientObj?.handle?.connected,
        }
      )
    } catch (err) {
      Log.log('MongoDB - Error updating connection stats: ' + err)
    }
  }, 5000)

  Log.log('MongoDB - Connecting to MongoDB server...', Log.levelMin)

  // Close a MongoDB client and its change streams. Events from a client that
  // was already replaced are ignored, so they can not close the current one.
  let changeStreams = []
  const dropMongo = (client, msg) => {
    if (!client || client !== clientMongo) return
    Log.log(msg)
    clientMongo = null
    const streams = changeStreams
    changeStreams = []
    streams.forEach((cs) => cs.close().catch(() => {}))
    client.close().catch(() => {})
  }

  const ackCommand = (id, ok, tag, logPrefix) => {
    if (!ok) Log.log(logPrefix + ' Command Error, Tag: ' + tag)
    if (!cmdCollection) return
    cmdCollection
      .updateOne({ _id: id }, { $set: { ack: ok, timeTag: new Date() } })
      .catch((err) => Log.log('MongoDB - Error updating command ack: ' + err))
  }

  while (true) {
    // repeat every 5 seconds

    // manages MQTT connection
    await sparkplugProcess(
      SparkplugClientObj,
      connection,
      jsConfig,
      clientMongo,
      MongoStatus
    )

    if (clientMongo === null)
      // if disconnected
      await MongoClient.connect(
        // try to (re)connect
        jsConfig.mongoConnectionString,
        jsConfig.MongoConnectionOptions
      )
        .then(async (client) => {
          // connected
          clientMongo = client
          MongoStatus.HintMongoIsConnected = true
          Log.log(
            'MongoDB - Connected correctly to MongoDB server',
            Log.levelMin
          )

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
          Log.log(
            'Connection - Connection Number: ' + jsConfig.ConnectionNumber
          )
          if ('autoCreateTags' in connection) {
            AutoCreateTags = connection.autoCreateTags ? true : false
          }

          let autoKeyId = await AutoTag.GetAutoKeyInitialValue(
            rtCollection,
            jsConfig
          )
          Log.log('Auto Key - Initial value: ' + autoKeyId)

          Redundancy.Start(5000, clientMongo, db, jsConfig, MongoStatus)

          // start a changestream monitor on realtimeData only if configured some MQTT publishing
          if (
            (connection.publishTopicRoot &&
              connection.publishTopicRoot.length > 0) ||
            (connection.groupId && connection.groupId.length > 0)
          ) {
            const changeStream = rtCollection.watch(csPipeline, {
              fullDocument: 'updateLookup',
            })
            changeStreams.push(changeStream)

            try {
              changeStream.on('error', (err) => {
                dropMongo(client, 'MongoDB - Error on ChangeStream! ' + err)
              })
              changeStream.on('close', () => {
                Log.log('MongoDB - Closed ChangeStream!')
              })
              changeStream.on('end', () => {
                dropMongo(client, 'MongoDB - Ended ChangeStream!')
              })

              // start listen to changes
              changeStream.on('change', (change) => {
                if (!change.fullDocument) return // deleted before the lookup
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
                $project: { documentKey: false },
              },
              {
                $match: {
                  $and: [
                    {
                      'fullDocument.protocolSourceConnectionNumber': {
                        $eq: connection.protocolConnectionNumber,
                      },
                    },
                    { operationType: 'insert' },
                  ],
                },
              },
            ]

            const changeStreamCmd = cmdCollection.watch(csCmdPipeline)
            changeStreams.push(changeStreamCmd)
            try {
              changeStreamCmd.on('error', (err) => {
                dropMongo(client, 'MongoDB - Error on ChangeStream Cmd! ' + err)
              })
              changeStreamCmd.on('close', () => {
                Log.log('MongoDB - Closed ChangeStream Cmd!')
              })
              changeStreamCmd.on('end', () => {
                dropMongo(client, 'MongoDB - Ended ChangeStream Cmd!')
              })

              // start listen to changes
              changeStreamCmd.on('change', (change) => {
                // do not queue data changes until device connected, and birthed (when sparkplug used)
                if (
                  (connection.groupId &&
                    connection.groupId.trim() !== '' &&
                    !SparkplugDeviceBirthed) ||
                  !SparkplugClientObj?.handle?.connected
                )
                  return

                if (
                  change.fullDocument?.protocolSourceConnectionNumber !==
                  connection.protocolConnectionNumber
                )
                  return // not for this connection

                let data = getMetricCommandPayload(change.fullDocument)
                if (!data) return

                const cmd = change.fullDocument
                const ack = (err) =>
                  ackCommand(
                    cmd._id,
                    !err,
                    cmd.tag,
                    data.groupId ? 'Sparkplug' : 'MQTT'
                  )
                try {
                  if (data.groupId) {
                    const timestamp = new Date(cmd.timeTag).getTime()
                    data.metric.timestamp = timestamp
                    const payload = {
                      timestamp: timestamp,
                      metrics: [data.metric],
                    }
                    if (data.deviceId)
                      SparkplugClientObj.handle.publishDeviceCmd(
                        data.groupId,
                        data.edgeNodeId,
                        data.deviceId,
                        payload,
                        {},
                        ack
                      )
                    else
                      SparkplugClientObj.handle.publishNodeCmd(
                        data.groupId,
                        data.edgeNodeId,
                        payload,
                        {},
                        ack
                      )
                  } else {
                    let qos = 0,
                      retain = false
                    if (!isNaN(cmd?.protocolSourceCommandDuration))
                      qos = parseInt(cmd.protocolSourceCommandDuration)
                    if (![0, 1, 2].includes(qos)) qos = 0
                    if (typeof cmd?.protocolSourceCommandUseSBO === 'boolean')
                      retain = cmd.protocolSourceCommandUseSBO
                    SparkplugClientObj.handle.client.publish(
                      data.topic,
                      data.value.toString(),
                      { qos: qos, retain: retain },
                      ack
                    )
                  }
                } catch (err) {
                  ack(err)
                }
              })
            } catch (e) {
              Log.log('MongoDB - CS CMD Error: ' + e, Log.levelMin)
            }
          }
        })
        .catch(function (err) {
          Log.log('MongoDB - ' + err)
          dropMongo(clientMongo, 'MongoDB - Closing client after error')
        })

    // wait 5 seconds
    await new Promise((resolve) => setTimeout(resolve, 5000))

    // detect connection problems, if error will close the client to later reconnect
    if (clientMongo && !(await checkConnectedMongo(clientMongo)))
      dropMongo(clientMongo, 'Disconnected Mongodb!')
  }
})()

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

// Get BIRTH payload for the edge node. Reboot and Next Server are offered
// (and honoured) only when commands are enabled for the connection.
function getNodeBirthPayload(jscadaConnection) {
  const hwVersion = 'Generic Server Hardware'
  const swVersion = 'JSON-SCADA MQTT v' + AppDefs.VERSION

  return {
    timestamp: new Date().getTime(),
    metrics: [
      {
        name: 'Node Control/Rebirth',
        type: 'boolean',
        value: false,
      },
      ...(jscadaConnection?.commandsEnabled ?
        [
          {
            name: 'Node Control/Reboot',
            type: 'boolean',
            value: false,
          },
          {
            name: 'Node Control/Next Server',
            type: 'boolean',
            value: false,
          },
        ]
      : []),
      {
        name: 'Properties/sw_version',
        type: 'string',
        value: swVersion,
      },
      {
        name: 'Properties/hw_version',
        type: 'string',
        value: hwVersion,
      },
    ],
  }
}

// Publish NBIRTH + DBIRTH. The DBIRTH is queried first, so both are published
// back-to-back with no DDATA in between. Returns the DBIRTH payload, or null.
async function publishBirths(
  spHandle,
  rtCollection,
  jscadaConnection,
  configObj
) {
  if (PublishingBirths || !spHandle) return null
  PublishingBirths = true
  try {
    const dbc = await getDeviceBirthPayload(
      rtCollection,
      jscadaConnection.commandsEnabled,
      jscadaConnection.protocolConnectionNumber,
      jscadaConnection
    )
    if (!spHandle.connected) return null

    const nbc = getNodeBirthPayload(jscadaConnection)
    spHandle.publishNodeBirth(nbc, {
      compress: AppDefs.SPARKPLUG_COMPRESS_NBIRTH,
    })
    spHandle.publishDeviceBirth(jscadaConnection.deviceId, dbc, {
      compress: AppDefs.SPARKPLUG_COMPRESS_DBIRTH,
    })
    BirthAliases.clear()
    dbc.metrics.forEach((m) => BirthAliases.add(m.alias))
    PendingDeviceRebirth = false
    LastBirthTime = new Date().getTime()
    SparkplugDeviceBirthed = true
    Log.log(
      'Sparkplug - Published node birth with ' +
        nbc.metrics.length +
        ' metrics and device birth "' +
        jscadaConnection.deviceId +
        '" with ' +
        dbc.metrics.length +
        ' metrics'
    )
    return dbc
  } finally {
    PublishingBirths = false
  }
}

// Get BIRTH payload for the device
async function getDeviceBirthPayload(
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
        _id: { $gt: 0 },
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
          protocolSourceConnectionNumber: 1,
        },
      }
    )
    .toArray()

  let metrics = []
  res.forEach((element) => {
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
        if (typeof element.valueString === 'string') value = element.valueString
        else
          value =
            element.value === null || element.value === undefined ?
              ''
            : element.value.toString()
        break
      case 'analog':
        type = 'double'
        value =
          element.value === null || element.value === undefined ?
            0
          : element.value
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
          value: topicName,
        },
      }

      topicName = jscadaConnection.publishTopicRoot.trim() + '/'
      if (element.group1 && element.group1.trim() !== '')
        topicName +=
          AppDefs.TAGS_SUBTOPIC + '/' + topicStr(element.group1) + '/'
      topicName += topicStr(element.tag) + '/value'
      topicAsTag = {
        topicAsTag: {
          type: 'string',
          value: topicName,
        },
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
        ...(element.origin === 'command' ?
          {
            isCommand: {
              type: 'boolean',
              value: true,
            },
          }
        : {}),
        description: { type: 'string', value: element.description },
        good: {
          type: 'boolean',
          value: element.invalid ? false : true,
        },
        ...(timestamp === false ?
          {}
        : {
            timestampGood: {
              type: 'boolean',
              value: timestampGood,
            },
          }),
      },
    }
    metrics.push(metric)
    return
  })

  return {
    timestamp: new Date().getTime(),
    metrics: metrics,
  }
}

// Get command payload
function getMetricCommandPayload(cmd) {
  let value = null

  if (
    typeof cmd.protocolSourceASDU !== 'string' ||
    cmd.protocolSourceASDU.trim() === '' ||
    typeof cmd.protocolSourceObjectAddress !== 'string'
  )
    return null

  // normalize type aliases to valid Sparkplug B types
  let asdu = cmd.protocolSourceASDU.toLowerCase()
  if (asdu === 'int') asdu = 'int64'
  if (asdu === 'text') asdu = 'string'

  // int, int8, int16, int32, int64, uint8, uint16, uint32, uint64, float, double, boolean, string, datetime
  switch (asdu) {
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

  const addr = parseCommandAddress(cmd.protocolSourceObjectAddress)
  if (!addr) return null
  if ('topic' in addr) return { topic: addr.topic, value: value }

  return {
    groupId: addr.groupId,
    edgeNodeId: addr.edgeNodeId,
    ...(addr.deviceId ? { deviceId: addr.deviceId } : {}),
    metric: {
      name: addr.metricName,
      value: value,
      type: asdu,
      timestamp: new Date().getTime(),
    },
  }
}

// Get data payload
function getMetricPayload(element, jscadaConnection) {
  if (
    element.origin === 'command' ||
    element._id < 1 ||
    // never republish what is acquired by this same connection
    element.protocolSourceConnectionNumber ===
      jscadaConnection.protocolConnectionNumber
  ) {
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
      if (typeof element.valueString === 'string') value = element.valueString
      else
        value =
          element.value === null || element.value === undefined ?
            ''
          : element.value.toString()
      break
    case 'analog':
      type = 'double'
      value =
        element.value === null || element.value === undefined ?
          0
        : element.value
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
        value: topicName,
      },
    }

    topicName = jscadaConnection.publishTopicRoot.trim() + '/'
    if (element.group1 && element.group1.trim() !== '')
      topicName += AppDefs.TAGS_SUBTOPIC + '/' + topicStr(element.group1) + '/'
    topicName += topicStr(element.tag) + '/value'
    topicAsTag = {
      topicAsTag: {
        type: 'string',
        value: topicName,
      },
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
        ),
      },
      valueString: {
        type: 'string',
        value: element.valueString || String(value),
      },
      good: {
        type: 'boolean',
        value: element.invalid ? false : true,
      },
      ...topic,
      ...topicAsTag,
      ...(timestamp === false ?
        {}
      : {
          timestampGood: {
            type: 'boolean',
            value: timestampGood,
          },
        }),
    },
  }
}

// find the connection number, if not found abort (only one connection per instance is allowed for this protocol)
async function getConnection(connsCollection, configObj) {
  let results = await connsCollection
    .find({
      protocolDriver: AppDefs.NAME,
      protocolDriverInstanceNumber: configObj.Instance,
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

// update queued data to mongodb
async function processMongoUpdates(
  clientMongo,
  collection,
  jsConfig,
  MongoStatus
) {
  // the timer fires every 500 ms: never run two passes at once (auto tag
  // creation would race and the writes could reach MongoDB out of order)
  if (MongoUpdatesRunning) return
  if (!clientMongo || !collection || !MongoStatus.HintMongoIsConnected) return
  MongoUpdatesRunning = true
  try {
    const ops = []
    while (!ValuesQueue.isEmpty() && ops.length < 5000) {
      try {
        let data = ValuesQueue.peek()
        ValuesQueue.dequeue()

        // check tag is created, if not found create it
        if (AutoCreateTags) {
          let topicSplit = data.protocolSourceObjectAddress.split('/')
          if (!data.group2) {
            if (topicSplit.length > 1 && topicSplit[0] === SparkplugNS)
              data.group2 = topicSplit[1]
            else if (topicSplit.length > 0) data.group2 = topicSplit[0]
          }
          if (!data.group3 && topicSplit.length > 2) data.group3 = topicSplit[2]
          await AutoTag.AutoCreateTag(
            data,
            jsConfig.ConnectionNumber,
            collection
          )
        }

        // now update tag
        Log.log(
          'Data Update - ' +
            data.protocolSourceObjectAddress +
            ' : ' +
            data.value,
          Log.levelDetailed
        )

        let numValue = parseFloat(data.value)
        if (isNaN(numValue)) numValue = 0
        let updTag = {
          valueAtSource: new Double(numValue),
          valueStringAtSource: data.valueString,
          valueJsonAtSource: data?.valueJson,
          asduAtSource: data?.asduAtSource,
          causeOfTransmissionAtSource: data?.causeOfTransmissionAtSource,
          timeTagAtSource: data.timeTagAtSource,
          timeTagAtSourceOk: data.timeTagAtSourceOk,
          timeTag: new Date(),
          originator:
            AppDefs.NAME +
            '|' +
            jsConfig.ConnectionNumber +
            (data?.msgType ? '|' + data.msgType : ''),
          invalidAtSource: data.invalid,
          transientAtSource: false,
          notTopicalAtSource: false,
          overflowAtSource: false,
          blockedAtSource: false,
          substitutedAtSource: false,
          // store-and-forward values go to the historian, not to the live value
          ...(data.isHistorical ? { isHistorical: true } : {}),
          ...(data.isNotForHistorical ? { isNotForHistorical: true } : {}),
        }
        ops.push({
          updateOne: {
            filter: {
              protocolSourceConnectionNumber: jsConfig.ConnectionNumber,
              protocolSourceObjectAddress: data.protocolSourceObjectAddress,
            },
            update: { $set: { sourceDataUpdate: updTag } },
          },
        })
      } catch (err) {
        Log.log('MongoDB - ' + err)
      }
    }

    // ordered, so updates of the same tag are applied in arrival order; on a
    // write error continue after the failed operation
    let start = 0
    while (start < ops.length) {
      try {
        await collection.bulkWrite(ops.slice(start), { ordered: true })
        break
      } catch (err) {
        Log.log('MongoDB - Error writing updates: ' + err)
        const we =
          Array.isArray(err?.writeErrors) ?
            err.writeErrors[0]
          : err?.writeErrors
        if (typeof we?.index !== 'number') break // not a per-document error
        start += we.index + 1
      }
    }
    if (ops.length) Log.log('MongoDB - Updates: ' + ops.length)
  } finally {
    MongoUpdatesRunning = false
  }
}

// sparkplug-client configuration options based on JSON-SCADA connection settings
function getSparkplugConfig(connection) {
  // TLS version range from the lowest to the highest allowed version
  const allowedVersions = []
  if (connection.allowTLSv10) allowedVersions.push('TLSv1')
  if (connection.allowTLSv11) allowedVersions.push('TLSv1.1')
  if (connection.allowTLSv12) allowedVersions.push('TLSv1.2')
  if (connection.allowTLSv13) allowedVersions.push('TLSv1.3')
  let minVersion = 'TLSv1.2',
    maxVersion = 'TLSv1.3'
  if (allowedVersions.length > 0) {
    minVersion = allowedVersions[0]
    maxVersion = allowedVersions[allowedVersions.length - 1]
  }

  let secOpts = {}
  if (connection.useSecurity) {
    let certOpts = {}
    if (
      connection.pfxFilePath !== '' &&
      Fs.existsSync(connection.pfxFilePath)
    ) {
      certOpts = {
        pfx: Fs.readFileSync(connection.pfxFilePath),
        passphrase: connection.passphrase,
      }
    } else {
      if (
        connection.rootCertFilePath !== '' &&
        Fs.existsSync(connection.rootCertFilePath)
      ) {
        certOpts.ca = Fs.readFileSync(connection.rootCertFilePath)
      }
      if (
        connection.privateKeyFilePath !== '' &&
        Fs.existsSync(connection.privateKeyFilePath)
      ) {
        certOpts.key = Fs.readFileSync(connection.privateKeyFilePath)
      }
      if (
        connection.localCertFilePath !== '' &&
        Fs.existsSync(connection.localCertFilePath)
      ) {
        certOpts.cert = Fs.readFileSync(connection.localCertFilePath)
      }
      certOpts.passphrase = connection.passphrase
    }

    secOpts = {
      rejectUnauthorized: connection.chainValidation,
      minVersion: minVersion,
      maxVersion: maxVersion,
      ciphers: connection.cipherList,
      ...certOpts,
    }
  }

  return {
    clean: AppDefs.MQTT_CLEAN_SESSION,
    keepalive: AppDefs.MQTT_CONNECTION_KEEPALIVE,
    connectTimeout: AppDefs.MQTT_CONNECTION_TIMEOUT * 1000,
    serverUrl: connection.endpointURLs[0],
    username: connection?.username || '',
    password: connection?.password || '',
    groupId: connection?.groupId || '',
    edgeNode: connection?.edgeNodeId || '',
    clientId: connection?.clientId || '',
    version: SparkplugNS,
    scadaHostId: connection?.scadaHostId || '', // only if a primary application
    publishDeath: true, // NDEATH on clean stops (the will covers abrupt losses)
    ...secOpts,
  }
}

// manage Sparkplug B Client connection, subscriptions, messages, events
async function sparkplugProcess(
  spClient,
  jscadaConnection,
  configObj,
  mongoClient,
  MongoStatus
) {
  sparkplugProcess.mongoClient = mongoClient

  if (
    jscadaConnection === null ||
    mongoClient === null ||
    !MongoStatus.HintMongoIsConnected
  ) {
    sparkplugProcess.db = null
    if (spClient.handle !== null) {
      Log.log('Sparkplug - Stopping client...')
      SparkplugDeviceBirthed = false
      spClient.handle.removeAllListeners()
      spClient.handle.stop()
      spClient.handle = null
    }
    return
  }
  const logMod = 'MQTT Client - '
  const logModS = 'Sparkplug - '
  const connectionRetriesLimit = 3 // failed sessions before trying the next broker

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

      // bdSeq must change for each new MQTT session (NBIRTH/NDEATH pairing)
      config.bdSeq = BdSeq
      BdSeq = nextSeq(BdSeq)

      const handle = SparkplugClient.newClient(config)
      spClient.handle = handle

      if (Log.levelCurrent === Log.levelDebug) handle.logger.level = 'debug'

      // Drop this client after a failure; the main loop creates a new one.
      // Consecutive failures rotate to the next broker. Events from a client
      // that was already replaced are ignored.
      const dropClient = (msg) => {
        if (spClient.handle !== handle) return
        SparkplugDeviceBirthed = false
        Log.log(logMod + msg)
        sparkplugProcess.connectionRetries++
        if (
          jscadaConnection.endpointURLs.length > 1 &&
          sparkplugProcess.connectionRetries >= connectionRetriesLimit
        ) {
          sparkplugProcess.currentBroker =
            (sparkplugProcess.currentBroker + 1) %
            jscadaConnection.endpointURLs.length
          sparkplugProcess.connectionRetries = 0
          Log.log(
            logMod +
              'Too many failures, will try broker ' +
              jscadaConnection.endpointURLs[sparkplugProcess.currentBroker]
          )
        }
        handle.stop()
        spClient.handle = null
      }

      handle.on('error', function (error) {
        dropClient("Event: Can't connect - " + error)
      })

      handle.on('close', function () {
        dropClient('Event: Connection Closed')
      })

      handle.on('offline', function () {
        dropClient('Event: Connection Offline...')
      })

      // Create 'birth' handler
      handle.on('birth', async function () {
        if (
          !('groupId' in jscadaConnection) ||
          jscadaConnection.groupId.trim() === '' ||
          sparkplugProcess.db === null ||
          !MongoStatus.HintMongoIsConnected
        )
          return

        SparkplugPublishQueue.clear() // clear old data
        try {
          const dbc = await publishBirths(
            handle,
            sparkplugProcess.db.collection(
              configObj.RealtimeDataCollectionName
            ),
            jscadaConnection,
            configObj
          )
          // also publish initial values as regular topics
          if (dbc)
            dbc.metrics.forEach((elem) => {
              SparkplugPublishQueue.enqueue(elem)
            })
        } catch (err) {
          Log.log(logModS + 'Error publishing births: ' + err)
        }
      })

      handle.on('connect', function () {
        sparkplugProcess.connectionRetries = 0
        Log.log(logMod + 'Event: Connected to broker')
        // Subscribe topics (plain, as-files and scripted)
        const topics = []
        if (jscadaConnection.topics instanceof Array)
          topics.push(...jscadaConnection.topics)
        if (jscadaConnection.topicsAsFiles instanceof Array)
          topics.push(...jscadaConnection.topicsAsFiles)
        if (jscadaConnection.topicsScripted instanceof Array)
          jscadaConnection.topicsScripted.forEach((elem) => {
            if (elem?.topic) topics.push(elem.topic)
          })
        topics.forEach((elem) => {
          if (typeof elem !== 'string' || elem.trim() === '') return
          let topicStr = JsonPathTopic(elem).topic

          Log.log(logMod + 'Subscribing topic: ' + topicStr)

          handle.client.subscribe(topicStr, { qos: 1 }, (err, granted) => {
            if (err)
              Log.log(
                logMod + 'Subscribe error on topic: ' + elem + ' : ' + err
              )
            if (granted && granted.length > 0)
              Log.log(
                logMod +
                  'Subscription granted for topic: ' +
                  granted[0].topic +
                  ' qos: ' +
                  granted[0].qos
              )
          })
        })
      })

      handle.on('reconnect', function () {
        Log.log(logMod + 'Event: Trying to reconnect to broker...')
      })

      // Context to run topicsScripted scripts. Only primitives and objects
      // created inside the context are handed to scripts (no host objects),
      // and every run is time limited. Scripts are trusted configuration.
      const scriptContext = vm.createContext(
        {},
        { microtaskMode: 'afterEvaluate' }
      )
      vm.runInContext('var shared = {}', scriptContext)

      const runScript = (script, payload) => {
        scriptContext.__payload = payload.toString('utf8')
        scriptContext.__bytes = vm.runInContext(
          'new Uint8Array(' + payload.length + ')',
          scriptContext
        )
        scriptContext.__bytes.set(payload)
        vm.runInContext(
          'shared.payload = __payload; shared.payloadBytes = __bytes; ' +
            'shared.dataArray = []; delete globalThis.__payload; delete globalThis.__bytes',
          scriptContext
        )
        vm.runInContext(script, scriptContext, { timeout: SCRIPT_TIMEOUT_MS })
        const out = vm.runInContext(
          'JSON.stringify(Array.isArray(shared.dataArray) ? shared.dataArray : [])',
          scriptContext,
          { timeout: SCRIPT_TIMEOUT_MS }
        )
        return JSON.parse(out || '[]')
      }

      // save payload as file on MongoDB GridFS, replacing older versions
      const saveAsFile = (topic, payload) => {
        const gfs = new GridFSBucket(sparkplugProcess.db)
        const writestream = gfs.openUploadStream(topic)
        const newId = writestream.id
        writestream.on('error', (e) =>
          Log.log(logMod + 'Error saving file ' + topic + '. ' + e.message)
        )
        // after the upload, delete only versions older than this one, so
        // concurrent uploads of the same topic can not delete each other
        writestream.on('finish', async () => {
          try {
            const files = await gfs.find({ filename: topic }).toArray()
            const mine = files.find((f) => f._id.equals(newId))
            if (!mine) return
            const isOlder = (f) =>
              f.uploadDate < mine.uploadDate ||
              (f.uploadDate.getTime() === mine.uploadDate.getTime() &&
                f._id.toHexString() < newId.toHexString())
            for (const f of files)
              if (!f._id.equals(newId) && isOlder(f)) await gfs.delete(f._id)
          } catch (e) {
            Log.log(logMod + 'Error deleting older file ' + topic + '. ' + e)
          }
        })
        Streamifier.createReadStream(payload).pipe(writestream)
      }

      const isUaData = (v) =>
        v !== null && typeof v === 'object' && v?.MessageType === 'ua-data'

      // process non sparkplug b messages
      handle.on('nonSparkplugMessage', function (topic, payload, packet) {
        if (sparkplugProcess.db === null || !MongoStatus.HintMongoIsConnected)
          return

        Log.log(
          logMod +
            'Event: Regular MQTT message, topic: ' +
            topic +
            ' size: ' +
            payload.length
        )

        // topics to be saved as files
        if (jscadaConnection?.topicsAsFiles instanceof Array) {
          const tp = jscadaConnection.topicsAsFiles.find((tp) =>
            topicMatchSub(topic, tp)
          )
          if (tp) {
            Log.log(logMod + 'Received topic as file ' + topic)
            try {
              saveAsFile(topic, payload)
            } catch (e) {
              Log.log(logMod + 'Error saving file. ' + e.message)
            }
            return
          }
        }

        // topics with scripts
        if (jscadaConnection?.topicsScripted instanceof Array) {
          const scripted = jscadaConnection.topicsScripted.filter(
            (elem) => elem?.topic && topicMatchSub(topic, elem.topic)
          )
          for (const elem of scripted) {
            if (typeof elem.script === 'string' && elem.script.trim() !== '') {
              try {
                runScript(elem.script, payload).forEach((element) => {
                  if (!element || !element.id || !('value' in element)) return
                  ValuesQueue.enqueue({
                    protocolSourceObjectAddress: topic + '/' + element.id,
                    value: element.value,
                    valueString:
                      element.valueString ?
                        element.valueString
                      : String(element.value),
                    valueJson:
                      element.valueJson ? element.valueJson : element.value,
                    invalid: element.qualityOk === false,
                    transient: element.transient === true,
                    causeOfTransmissionAtSource:
                      'causeOfTransmissionAtSource' in element ?
                        element.causeOfTransmissionAtSource
                      : '3',
                    timeTagAtSource:
                      element.timestamp ?
                        new Date(element.timestamp)
                      : new Date(),
                    timeTagAtSourceOk: element.timestamp ? true : false,
                    asduAtSource: 'scripted',
                    type: element.type ? element.type : 'analog',
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
          }
          if (scripted.length > 0) return
        }

        // plain topics (MQTT wildcards allowed) and JSONPathPlus topics
        if (!(jscadaConnection?.topics instanceof Array)) return
        let parsed
        const json = () => {
          if (parsed === undefined) parsed = TryPayloadAsRJson(payload)
          return parsed
        }
        jscadaConnection.topics.forEach((elem) => {
          if (typeof elem !== 'string' || elem.trim() === '') return
          const jpt = JsonPathTopic(elem)
          if (!topicMatchSub(topic, jpt.topic)) return

          if (jpt.jsonPath === '') {
            const JsonValue = json()
            if (isUaData(JsonValue)) ProcessOpcUaPubSubMessage(JsonValue, topic)
            else EnqueueJsonValue(JsonValue, topic)
            return
          }

          // the tag address uses the received topic (a wildcard subscription
          // must not collapse different topics into one tag)
          const baseAddress = topic === jpt.topic ? elem : null

          // when used '*~' at the end of JSONPathPlus explode property names
          if (jpt.jsonPath.endsWith('*~')) {
            const jpPropNames = JSONPath({
              path: jpt.jsonPath,
              json: json(),
              wrap: true,
            })
            if (!Array.isArray(jpPropNames)) return
            jpPropNames.forEach((propName) => {
              const propPath = jpt.jsonPath.replace('*~', propName)
              const jpRes = JSONPath({
                path: propPath,
                json: json(),
                wrap: false,
              })
              EnqueueJsonValue(jpRes, topic + '/' + propPath.replace('$.', ''))
            })
            return
          }

          // extract value from payload using JSON PATH
          const jpRes = JSONPath({
            path: jpt.jsonPath,
            json: json(),
            wrap: false,
          })
          EnqueueJsonValue(jpRes, baseAddress || topic + '/' + jpt.jsonPath)
        })
      })

      // process received node commands (for this node)
      handle.on('ncmd', async function (payload) {
        Log.log(
          logModS + 'Received NCMD - ' + JSON.stringify(payload),
          Log.levelDetailed
        )
        if (!(payload?.metrics instanceof Array)) return
        for (const metric of payload.metrics) {
          try {
            switch (metric?.name) {
              case 'Node Control/Rebirth':
                if (
                  !('groupId' in jscadaConnection) ||
                  jscadaConnection.groupId.trim() === '' ||
                  metric?.value !== true ||
                  sparkplugProcess.db === null
                )
                  break
                if (
                  LastNodeRebirth + AppDefs.SECONDS_BETWEEN_REBIRTHS * 1000 >
                  new Date().getTime()
                ) {
                  Log.log(
                    logModS +
                      `Node Rebirth command received, ignoring until after ${AppDefs.SECONDS_BETWEEN_REBIRTHS} seconds`
                  )
                  break
                }
                LastNodeRebirth = new Date().getTime()
                Log.log(logModS + 'Node Rebirth command received')
                await publishBirths(
                  handle,
                  sparkplugProcess.db.collection(
                    configObj.RealtimeDataCollectionName
                  ),
                  jscadaConnection,
                  configObj
                )
                break
              case 'Node Control/Reboot':
                if (
                  !jscadaConnection.commandsEnabled ||
                  metric?.value !== true
                ) {
                  Log.log(logModS + 'Node Reboot command ignored')
                  break
                }
                Log.log(
                  logModS + 'Node Reboot command received, exiting process...'
                )
                handle.stop()
                setTimeout(() => process.exit(999), 1000)
                break
              case 'Node Control/Next Server':
                if (
                  !jscadaConnection.commandsEnabled ||
                  metric?.value !== true
                ) {
                  Log.log(logModS + 'Node Next Server command ignored')
                  break
                }
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
                  if (spClient.handle === handle) {
                    SparkplugDeviceBirthed = false
                    handle.stop()
                    spClient.handle = null
                    sparkplugProcess.connectionRetries = 0
                  }
                } else Log.log(logMod + 'No alternatives servers configured!')
                break
            }
          } catch (err) {
            Log.log(logModS + 'Error processing NCMD: ' + err)
          }
        }
      })

      // process received device commands (for this device)
      handle.on('dcmd', function (deviceId, payload) {
        if (!jscadaConnection.commandsEnabled) return
        if (mongoClient === null || !MongoStatus.HintMongoIsConnected) return
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
          payload.metrics.forEach((metric) => {
            ProcessDeviceCommand(
              deviceId,
              metric,
              payload?.timestamp,
              sparkplugProcess.mongoClient,
              jscadaConnection,
              configObj,
              MongoStatus
            ).catch((err) => Log.log(logModS + 'Error processing DCMD: ' + err))
          })
        }
      })

      // process MQTT Sparkplug B messages (coming from other nodes/devices)
      handle.on('message', function (topic, payload, topicInfo) {
        try {
          processSparkplugMessage(topic, payload, topicInfo)
        } catch (err) {
          Log.log(
            logModS +
              'Error processing message on topic ' +
              topic +
              ': ' +
              (err?.stack || err)
          )
        }
      })

      // ask an edge node to rebirth, at most once per waiting period
      const requestRebirth = (topicInfo, nodeLocator, reason) => {
        const now = new Date().getTime()
        if (
          RebirthRequests[nodeLocator] &&
          now <
            RebirthRequests[nodeLocator] +
              AppDefs.SECONDS_BETWEEN_NODE_REQUESTS * 1000
        ) {
          Log.log(
            logModS +
              reason +
              ' (waiting time between birth requests): ' +
              nodeLocator,
            Log.levelDetailed
          )
          return
        }
        RebirthRequests[nodeLocator] = now
        Log.log(logModS + reason + ', requesting node rebirth: ' + nodeLocator)
        if (spClient.handle === handle && handle.connected)
          handle.publishNodeCmd(topicInfo.groupId, topicInfo.edgeNodeId, {
            timestamp: now,
            metrics: [
              {
                name: 'Node Control/Rebirth',
                timestamp: now,
                type: 'Boolean',
                value: true,
              },
            ],
          })
      }

      const invalidate = (path) => {
        if (sparkplugProcess.mongoClient)
          InvalidatePathTags(
            path,
            sparkplugProcess.mongoClient,
            jscadaConnection,
            configObj,
            MongoStatus
          )
      }

      const processSparkplugMessage = (topic, payload, topicInfo) => {
        const metricsIn = Array.isArray(payload?.metrics) ? payload.metrics : []
        payload.metrics = metricsIn.filter(
          (metric) => !(metric?.type === undefined || metric?.type === null)
        )
        Log.log(logModS + 'Event: Sparkplug B message on topic: ' + topic)
        if (payload.metrics.length < metricsIn.length)
          Log.log(
            logModS +
              'Discarded ' +
              (metricsIn.length - payload.metrics.length) +
              ' metric(s) of unknown or unsupported datatype on topic: ' +
              topic
          )

        if (Log.levelCurrent >= Log.levelDetailed) {
          Log.log(logModS + JSON.stringify(payload), Log.levelDetailed)
        }

        if (
          !topicInfo?.namespace ||
          !topicInfo?.groupId ||
          !topicInfo?.msgType ||
          !topicInfo?.edgeNodeId ||
          // device-level messages require a deviceId (5 topic levels)
          (['DDATA', 'DBIRTH', 'DDEATH'].includes(topicInfo.msgType) &&
            !topicInfo.deviceId)
        ) {
          // ignore topics with less levels than required
          Log.log(logModS + 'Ignored invalid topic: ' + topic)
          return
        }
        const nodeLocator =
          topicInfo.namespace +
          '/' +
          topicInfo.groupId +
          '/' +
          topicInfo.edgeNodeId
        const deviceLocator =
          topicInfo.deviceId ?
            nodeLocator + '/' + topicInfo.deviceId
          : nodeLocator
        const msgType = topicInfo.msgType

        // seq is shared by all messages of an edge node except NDEATH; a gap
        // means lost messages, so ask for a rebirth (data is still processed)
        if (
          ['NDATA', 'DBIRTH', 'DDATA', 'DDEATH'].includes(msgType) &&
          payload.seq !== undefined &&
          payload.seq !== null &&
          NodeStates[nodeLocator]?.seq !== undefined
        ) {
          const seq = toNumber(payload.seq)
          const expected = nextSeq(NodeStates[nodeLocator].seq)
          NodeStates[nodeLocator].seq = seq
          if (seq !== expected)
            requestRebirth(
              topicInfo,
              nodeLocator,
              'Sequence gap (expected ' + expected + ', received ' + seq + ')'
            )
        }

        switch (msgType) {
          case 'NCMD': // commands for other nodes
          case 'DCMD': // commands for other devices
            break
          case 'NDATA':
            // edge of node data (7.2)
            if (!DevicesList[nodeLocator]?.birthed) {
              requestRebirth(
                topicInfo,
                nodeLocator,
                'Data from not yet birthed node'
              )
              return
            }
            ProcessNodeBirthOrData(nodeLocator, payload, false, msgType)
            break
          case 'DDATA':
            // device data, update metrics (7.4)
            Log.log(
              logModS + 'Device DATA: ' + deviceLocator,
              Log.levelDetailed
            )
            if (!DevicesList[deviceLocator]?.birthed) {
              requestRebirth(
                topicInfo,
                nodeLocator,
                'Data from not yet birthed device ' + deviceLocator
              )
              return
            }
            ProcessDeviceBirthOrData(deviceLocator, payload, false, msgType)
            break
          case 'NBIRTH': {
            // on node birth all associated data is invalidated (7.1.2)
            Log.log(logModS + 'Node BIRTH: ' + nodeLocator, Log.levelDetailed)
            const bd = payload.metrics.find((m) => m?.name === 'bdSeq')
            NodeStates[nodeLocator] = {
              bdSeq: bd ? toNumber(bd.value) : undefined,
              seq:
                payload.seq !== undefined && payload.seq !== null ?
                  toNumber(payload.seq)
                : undefined,
            }
            delete RebirthRequests[nodeLocator]
            // devices of this node must be birthed again (DBIRTH follows)
            Object.keys(DevicesList).forEach(function (key) {
              if (key !== nodeLocator && isSameOrChildPath(key, nodeLocator))
                DevicesList[key].birthed = false
            })
            ProcessNodeBirthOrData(nodeLocator, payload, true, msgType)
            invalidate(nodeLocator)
            break
          }
          case 'DBIRTH':
            // device birth, create tags and update metrics (7.3.1)
            Log.log(
              logModS + 'Device BIRTH: ' + deviceLocator,
              Log.levelDetailed
            )
            ProcessDeviceBirthOrData(deviceLocator, payload, true, msgType)
            break
          case 'NDEATH': {
            // Node death, invalidate all Sparkplug metrics from this node (7.1.1)
            // a death whose bdSeq differs from the current NBIRTH is stale
            // (e.g. a late will of a previous session) and is ignored
            const bd = payload.metrics.find((m) => m?.name === 'bdSeq')
            const st = NodeStates[nodeLocator]
            if (
              bd &&
              st?.bdSeq !== undefined &&
              toNumber(bd.value) !== st.bdSeq
            ) {
              Log.log(
                logModS +
                  'Ignored stale Node DEATH (bdSeq ' +
                  toNumber(bd.value) +
                  ', current ' +
                  st.bdSeq +
                  '): ' +
                  nodeLocator
              )
              break
            }
            Log.log(logModS + 'Node DEATH: ' + nodeLocator, Log.levelDetailed)
            if (st) st.seq = undefined
            // the node and its devices are marked as dead
            Object.keys(DevicesList).forEach(function (key) {
              if (isSameOrChildPath(key, nodeLocator))
                DevicesList[key].birthed = false
            })
            invalidate(nodeLocator)
            break
          }
          case 'DDEATH':
            // Device death, invalidate all Sparkplug metrics from this device (7.3.2)
            Log.log(
              logModS + 'Device DEATH: ' + deviceLocator,
              Log.levelDetailed
            )
            if (deviceLocator in DevicesList)
              DevicesList[deviceLocator].birthed = false
            invalidate(deviceLocator)
            break
        }
      }
    } catch (e) {
      Log.log(logModS + 'Error: ' + e.message, Log.levelMin)
    }
  } else {
    // MQTT client is already created

    if (Redundancy.ProcessStateIsActive()) {
      // test connection is established

      Log.log(
        logMod +
          (spClient.handle.connected ?
            'Currently Connected'
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

function ProcessDeviceBirthOrData(deviceLocator, payload, isBirth, msgType) {
  if (!('metrics' in payload)) return

  if (isBirth) {
    Log.log('Sparkplug - New device: ' + deviceLocator)
    DevicesList[deviceLocator] = {
      birthed: true,
      mapAliasToObjectAddress: [],
      metrics: payload.metrics,
    }
  }

  // extract metrics info and queue for tags updates on mongodb
  payload.metrics.forEach((metric) => {
    queueMetric(metric, deviceLocator, isBirth, '', msgType)
  })
}

function ProcessNodeBirthOrData(nodeLocator, payload, isBirth, msgType) {
  if (!('metrics' in payload)) return

  if (isBirth) {
    Log.log('Sparkplug - New node: ' + nodeLocator)
    DevicesList[nodeLocator] = {
      birthed: true,
      mapAliasToObjectAddress: [],
      metrics: payload.metrics,
    }
  }

  // extract metrics info and queue for tags updates on mongodb
  payload.metrics.forEach((metric) => {
    queueMetric(metric, nodeLocator, isBirth, '', msgType)
  })
}

// obtain information from sparkplug-b decoded payload and queue for mongo tag updates
function queueMetric(metric, deviceLocator, isBirth, templateName, msgType) {
  let value = 0,
    valueString = '',
    valueJson = '',
    type = 'digital',
    invalid = false,
    isNull = false,
    timestamp,
    timestampGood = true,
    catalogProperties = {},
    objectAddress = null,
    isHistorical = null,
    isNotForHistorical = null

  //if (typeof queueMetric.MapAliasToObjectAddress === 'undefined')
  //  queueMetric.MapAliasToObjectAddress = []

  if (typeof metric?.type !== 'string') {
    Log.log('Sparkplug - Invalid metric (missing type)!', Log.levelDetailed)
    return false
  }

  if (typeof metric.name === 'string' && metric.name.trim() !== '') {
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

  if (metric?.is_historical || metric?.isHistorical) {
    isHistorical = true
  }

  if (metric?.is_transient || metric?.isTransient) {
    isNotForHistorical = true
  }

  if ('timestamp' in metric) {
    timestamp = castSparkplugValue('datetime', metric.timestamp).getTime()
  } else {
    timestamp = new Date().getTime()
    timestampGood = false
  }

  if (
    metric?.value === null ||
    metric?.is_null === true ||
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
        if (
          metric.value !== null &&
          typeof metric.value === 'object' &&
          'metrics' in metric.value
        ) {
          metric.value.metrics.forEach((m) => {
            queueMetric(m, deviceLocator, isBirth, metric.name, msgType)
          })
          return
        } else {
          valueJson = JSON.stringify(metric)
          valueString = JSON.stringify(metric.value)
        }
      }
      break
    case 'dataset':
      // transform data set in a simpler array of objects with named properties
      type = 'json'
      if ('value' in metric) {
        if (
          metric.value !== null &&
          typeof metric.value === 'object' &&
          'numOfColumns' in metric.value
        ) {
          let v = []
          for (let j = 0; j < metric.value.rows.length; j++) {
            let r = {}
            for (let i = 0; i < metric.value.numOfColumns; i++) {
              let mv = castSparkplugValue(
                metric.value.types[i],
                metric.value.rows[j][i]
              )
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
          valueJson = JSON.stringify(metric)
          valueString = JSON.stringify(v)
        } else {
          valueJson = JSON.stringify(metric)
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
        valueJson = JSON.stringify(metric)
      }
      break
    case 'uuid':
    case 'text':
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
        valueJson = JSON.stringify(metric)
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
        valueJson = JSON.stringify(metric)
      } else {
        // metric does have a value
        value = castSparkplugValue(metric.type, metric.value)
        valueString = value.toString()
        valueJson = JSON.stringify(metric)
      }
      break
    case 'int64':
    case 'uint64':
    case 'datetime':
      type = 'analog'
      if (!('value' in metric) || metric.value === null) {
        // metric does not have a value
        value = 0
        valueString = '0'
        valueJson = JSON.stringify(metric)
      } else {
        // metric does have a value
        value = castSparkplugValue(metric.type, metric.value)
        if ('toNumber' in value) {
          value = value.toNumber() // warning number may be truncated
          valueString = value.toString()
          valueJson = JSON.stringify(metric)
        } else {
          valueString = value.toString()
          valueJson = JSON.stringify(metric)
          value = new Date(value).getTime()
        }
      }
      break
    case 'bytes':
    case 'file':
      // binary content as base64 text
      type = 'string'
      if (!('value' in metric) || metric.value === null) {
        valueString = ''
      } else {
        valueString = Buffer.from(metric.value).toString('base64')
      }
      valueJson = JSON.stringify({
        ...metric,
        value: valueString,
        encoding: 'base64',
      })
      break
    default:
      // other datatypes are kept as JSON
      type = 'json'
      Log.log(
        'Sparkplug - Metric datatype ' +
          metric.type +
          ' stored as JSON: ' +
          objectAddress,
        Log.levelDetailed
      )
      valueJson = JSON.stringify(metric)
      valueString = JSON.stringify(metric.value ?? null)
      break
  }

  if ('properties' in metric) {
    if ('good' in metric.properties) {
      invalid = !metric.properties.good.value
    }
    if ('quality' in metric.properties) {
      invalid = metric.properties.quality.value !== 192
    }
    if ('Quality' in metric.properties) {
      invalid = metric.properties.Quality.value !== 192
    }
    if ('timestampGood' in metric.properties) {
      timestampGood = metric.properties.timestampGood.value ? true : false
    }
  }

  if (isBirth) {
    catalogProperties = {
      description: '',
      ungroupedDescription: '',
      group1: '',
      group2: '',
      group3: '',
      type: type,
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
    metricType: metric.type.toLowerCase(),
    msgType: msgType,
    isNull: isNull,
    isHistorical: isHistorical,
    isNotForHistorical: isNotForHistorical,
    ...catalogProperties,
  })
}

// Process received Sparkplug B command to possible protocol destinations (routed command)
async function ProcessDeviceCommand(
  deviceId,
  metric,
  timestamp,
  mongoClient,
  jscadaConnection,
  configObj,
  MongoStatus
) {
  if (!mongoClient || !MongoStatus.HintMongoIsConnected) return
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
        origin: 'command',
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
          protocolSourceCommandUseSBO: 1,
          commandBlocked: 1,
          supervisedOfCommand: 1,
        },
      }
    )
    .toArray()

  for (const element of res) {
    if (element.origin !== 'command' || element._id <= 0) {
      continue
    }

    // blocked commands (on the command or on its supervised point) are refused
    if (element.commandBlocked === true) {
      Log.log('Sparkplug - Command blocked! Tag: ' + element.tag)
      continue
    }
    if (element.supervisedOfCommand && element.supervisedOfCommand != 0) {
      const sup = await rtCollection.findOne(
        { _id: element.supervisedOfCommand },
        { projection: { commandBlocked: 1 } }
      )
      if (sup?.commandBlocked === true) {
        Log.log('Sparkplug - Command blocked (sup)! Tag: ' + element.tag)
        continue
      }
    }

    if (
      element.protocolSourceConnectionNumber ===
      jscadaConnection.protocolConnectionNumber
    ) {
      Log.log(
        'Sparkplug - Discarding received command on the same driver connection: ' +
          jscadaConnection.protocolConnectionNumber
      )
      continue
    }

    let value = parseFloat(metric.value)
    if (isNaN(value)) value = 0
    let valueString = ''
    let valueJson = ''

    if ('type' in metric)
      switch (metric.type.toLowerCase()) {
        case 'boolean':
          if (element.kconv1 === -1) value = metric.value ? 0 : 1
          else value = metric.value ? 1 : 0
          valueString = value ? 'true' : 'false'
          valueJson = JSON.stringify(metric)
          break
        case 'uuid':
        case 'text':
        case 'string':
          valueString = metric.value
          valueJson = JSON.stringify(metric)
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
          valueJson = JSON.stringify(metric)
          break
        case 'int64':
        case 'uint64':
          value = castSparkplugValue(metric.type, metric.value)
          value = value.mul(element.kconv1).add(element.kconv2) // safe Long object
          value = value.toNumber() // warning unsafe number
          valueString = value.toString()
          valueJson = JSON.stringify(metric)
          break
        // case 'datetime':
        default:
          valueString = JSON.stringify(metric)
          valueJson = JSON.stringify(metric)
          break
      }
    else {
      Log.log('Sparkplug - Invalid command!')
      continue
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
      timeTag: new Date(),
    }

    let rIns = await cmdQueue.insertOne(cmd)
    if (rIns.acknowledged)
      Log.log(
        'MongoDB - Command Queued: ' + JSON.stringify(cmd),
        Log.levelDetailed
      )
  }
}

// invalidate tags from device or node based on Sparkplug B topic path
function InvalidatePathTags(
  topicPath,
  mongoClient,
  jscadaConnection,
  configObj,
  MongoStatus
) {
  if (!mongoClient || !MongoStatus.HintMongoIsConnected) return
  try {
    Log.log('MongoDB - Invalidate tags from ' + topicPath)
    const db = mongoClient.db(configObj.mongoDatabaseName)
    const rtCollection = db.collection(configObj.RealtimeDataCollectionName)
    rtCollection
      .updateMany(
        {
          protocolSourceConnectionNumber:
            jscadaConnection.protocolConnectionNumber,
          // the path itself and below it, never siblings (Node1 vs Node10)
          protocolSourceObjectAddress: { $regex: pathRegex(topicPath) },
        },
        { $set: { invalid: true } }
      )
      .catch((e) =>
        Log.log(
          'MongoDB - Error invalidating tags from ' + topicPath + ' - ' + e
        )
      )
  } catch (e) {
    Log.log(
      'MongoDB - Error invalidating tags from ' + topicPath + ' - ' + e.message
    )
  }
}

// convert possible JSON value to number and string and enqueue for mongo update
function EnqueueJsonValue(JsonValue, protocolSourceObjectAddress) {
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

  ValuesQueue.enqueue({
    protocolSourceObjectAddress: protocolSourceObjectAddress,
    value: value,
    valueString: valueString,
    valueJson: JSON.stringify(JsonValue),
    invalid: false,
    transient: false,
    causeOfTransmissionAtSource: '3',
    timeTagAtSource: new Date(),
    timeTagAtSourceOk: false,
    asduAtSource: typeof JsonValue,
    type: type,
  })
}

function TryPayloadAsRJson(payload) {
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
      // try to decompress gzipped payload
      try {
        JsonValue = JSON.parse(zlib.gunzipSync(payload))
      } catch (e) {
        // NOT JSON NOR RJSON, consider as string
        JsonValue = payloadStr
      }
    }
  }
  return JsonValue
}

// test mongoDB connectivity
async function checkConnectedMongo(client) {
  if (!client) {
    return false
  }
  const CheckMongoConnectionTimeout = 10000
  let tr = setTimeout(() => {
    Log.log('Mongo ping timeout error!')
    MongoStatus.HintMongoIsConnected = false
  }, CheckMongoConnectionTimeout)

  let res = null
  try {
    res = await client.db('admin').command({ ping: 1 })
    clearTimeout(tr)
  } catch (e) {
    Log.log('Error on mongodb connection!')
    return false
  }
  if ('ok' in res && res.ok) {
    MongoStatus.HintMongoIsConnected = true
    return true
  } else {
    MongoStatus.HintMongoIsConnected = false
    return false
  }
}

// Detect and process OPC-UA PubSub JSON messages
// Implements OPC UA Part 14 (PubSub) JSON message mapping
function ProcessOpcUaPubSubMessage(opcMsgObj, topic) {
  // Check for OPC-UA PubSub message type
  if (opcMsgObj?.MessageType !== 'ua-data') {
    return false
  }
  Log.log(
    'OPC-UA PubSub - Detected ua-data message on topic: ' + topic,
    Log.levelDetailed
  )

  const publisherId = opcMsgObj.PublisherId || 'unknown'
  const messages = opcMsgObj.Messages

  if (!Array.isArray(messages)) {
    Log.log(
      'OPC-UA PubSub - Invalid message format: Messages is not an array',
      Log.levelDebug
    )
    return false
  }

  messages.forEach((dataSetMessage) => {
    const dataSetWriterId = dataSetMessage?.DataSetWriterId || 'unknown'
    const messageTimestamp =
      dataSetMessage?.Timestamp ?
        new Date(dataSetMessage.Timestamp)
      : new Date()
    const messageStatus = dataSetMessage?.Status
    const payload = dataSetMessage?.Payload

    // Check for message-level status indicating bad quality
    let messageInvalid = false
    if (messageStatus && !messageStatus?.Symbol?.includes('Good')) {
      // Non-zero status indicates bad quality at message level
      messageInvalid = true
    }

    if (payload && typeof payload === 'object') {
      // Explode each field in the payload
      Object.entries(payload).forEach(([fieldName, fieldValue]) => {
        const objectAddress = `${topic}/${publisherId}/${dataSetWriterId}/${fieldName}`

        let extractedValue = null
        let fieldTimestamp = messageTimestamp
        let fieldTimestampOk = !!dataSetMessage.Timestamp
        let fieldInvalid = messageInvalid
        let valueType = 'json'
        let value = 0
        let valueString = ''

        // OPC-UA can encode values as objects with Value/SourceTimestamp/StatusCode or as raw values
        if (fieldValue !== null && typeof fieldValue === 'object') {
          if ('Value' in fieldValue) {
            // DataValue format with Value, SourceTimestamp, StatusCode
            // Extract the actual value - may be in Body for complex types
            extractedValue = fieldValue.Value?.Body ?? fieldValue.Value

            // Extract source timestamp if available
            if (fieldValue.SourceTimestamp) {
              fieldTimestamp = new Date(fieldValue.SourceTimestamp)
              fieldTimestampOk = true
            } else if (fieldValue.ServerTimestamp) {
              fieldTimestamp = new Date(fieldValue.ServerTimestamp)
              fieldTimestampOk = true
            }

            // Extract quality/status - StatusCode 0 or absent means Good
            if ('StatusCode' in fieldValue) {
              const statusCode = fieldValue.StatusCode
              // StatusCode can be a number or an object with Code property
              const codeValue =
                typeof statusCode === 'object' ? statusCode.Code : statusCode
              // In OPC UA, StatusCode high bits indicate severity:
              // 0x00000000 = Good, 0x40000000 = Uncertain, 0x80000000 = Bad
              if (
                codeValue !== undefined &&
                codeValue !== null &&
                codeValue !== 0
              ) {
                // Check if it's Bad (high bit set) or Uncertain
                if ((codeValue & 0x80000000) !== 0) {
                  fieldInvalid = true
                } else if ((codeValue & 0x40000000) !== 0) {
                  // Uncertain - could mark as invalid or handle differently
                  fieldInvalid = true
                }
              }
            }
          } else if ('Body' in fieldValue) {
            // Variant format with Type and Body
            extractedValue = fieldValue.Body
          } else {
            // Direct value object (JSON structure)
            extractedValue = fieldValue
          }
        } else {
          // Raw value (number, string, boolean, null)
          extractedValue = fieldValue
        }

        // Convert extracted value to proper types for enqueuing
        switch (typeof extractedValue) {
          case 'boolean':
            valueType = 'digital'
            value = extractedValue ? 1 : 0
            valueString = extractedValue.toString()
            break
          case 'number':
            valueType = 'analog'
            value = extractedValue
            valueString = extractedValue.toString()
            break
          case 'string':
            valueType = 'string'
            value = parseFloat(extractedValue)
            if (isNaN(value)) value = 0
            valueString = extractedValue
            break
          case 'object':
            if (extractedValue === null) {
              valueType = 'json'
              value = 0
              valueString = ''
              fieldInvalid = true
            } else if (Array.isArray(extractedValue)) {
              valueType = 'json'
              value = 0
              valueString = JSON.stringify(extractedValue)
            } else {
              valueType = 'json'
              value = 0
              valueString = JSON.stringify(extractedValue)
            }
            break
          default:
            valueType = 'json'
            value = 0
            valueString = ''
            break
        }

        // Enqueue the value for MongoDB update
        ValuesQueue.enqueue({
          protocolSourceObjectAddress: objectAddress,
          value: value,
          valueString: valueString,
          valueJson: JSON.stringify(fieldValue),
          invalid: fieldInvalid,
          transient: false,
          causeOfTransmissionAtSource: '3',
          timeTagAtSource: fieldTimestamp,
          timeTagAtSourceOk: fieldTimestampOk,
          asduAtSource: 'opcua-pubsub',
          type: valueType,
        })

        Log.log(
          'OPC-UA PubSub - Enqueued: ' +
            objectAddress +
            ' = ' +
            valueString +
            (fieldInvalid ? ' [INVALID]' : ''),
          Log.levelDebug
        )
      })
    }
  })

  return true // Indicate message was processed
}
