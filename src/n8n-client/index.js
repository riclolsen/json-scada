/*
 * N8N integration driver for JSON-SCADA.
 *
 * Outbound (SCADA -> n8n): tails realtimeData (processed value changes) and
 * optionally soeData, batches notification envelopes and POSTs them to the n8n
 * webhook URLs listed in the connection's endpointURLs.
 *
 * Inbound (n8n -> SCADA): runs an HTTP listener where n8n workflows push values
 * (auto-created supervised tags) and, with double opt-in, issue commands.
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

const fs = require('fs')
const { MongoClient, Double } = require('mongodb')
const { setInterval } = require('timers')
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')
const LoadConfig = require('./load-config')
const Redundancy = require('./redundancy')
const Filters = require('./filters')
const WebhookPusher = require('./webhook-push')
const Tags = require('./tags-creation')
const { StartListener } = require('./http-listener')

process.on('uncaughtException', (err) =>
  Log.log('Uncaught Exception: ' + (err.stack || JSON.stringify(err)))
)

const configObj = LoadConfig()
Log.levelCurrent = configObj.LogLevel

// runtime state shared with the listener via ctx
let AutoKeyId = 0
let AutoCreateTags = true
let ConnectionNumber = 0
let Connection = null
let Options = {}
const ListCreatedTags = new Set()
const Stats = {
  nodeName: configObj.nodeName,
  notificationsSent: 0,
  notificationsDropped: 0,
  webhookErrors: 0,
  lastWebhookError: '',
  inboundUpdates: 0,
  inboundCommands: 0,
  queueSizes: {},
  timeTag: new Date(),
}

// outbound batching buffers
let ValueBatch = []
let ValueBatchTimer = null
let Pusher = null
let ParsedFilters = { valueRules: [], soeRules: [] }

// allocate a new auto-tag key within this connection's partition
function allocKey() {
  AutoKeyId++
  return AutoKeyId
}

// map a realtimeData document to an outbound point notification
function pointFromDoc(doc) {
  return {
    tag: doc.tag,
    pointKey: parseInt(doc._id),
    value: typeof doc.value === 'object' ? parseFloat(doc.value) : doc.value,
    valueString: doc.valueString,
    valueJson: doc.valueJson && Object.keys(doc.valueJson).length ? doc.valueJson : null,
    type: doc.type,
    invalid: doc.invalid === true,
    substituted: doc.substituted === true,
    alarmed: doc.alarmed === true,
    timeTag: doc.timeTag || null,
    timeTagAtSource: doc.timeTagAtSource || null,
    group1: doc.group1 || '',
    group2: doc.group2 || '',
    group3: doc.group3 || '',
    description: doc.description || '',
    unit: doc.unit || '',
  }
}

// build a versioned envelope
function makeEnvelope(type, extra) {
  return {
    schema: AppDefs.PAYLOAD_SCHEMA,
    type: type,
    nodeName: configObj.nodeName,
    connectionNumber: ConnectionNumber,
    connectionName: Connection?.name,
    timestamp: new Date().toISOString(),
    ...extra,
  }
}

function flushValueBatch() {
  if (ValueBatchTimer) {
    clearTimeout(ValueBatchTimer)
    ValueBatchTimer = null
  }
  if (ValueBatch.length === 0) return
  if (!Pusher || !Pusher.hasTargets()) {
    ValueBatch = []
    return
  }
  const points = ValueBatch
  ValueBatch = []
  Pusher.push(makeEnvelope('valueChange', { points }))
}

function enqueueValue(point) {
  ValueBatch.push(point)
  const batchMax = Options.batchMaxSize || AppDefs.BATCH_MAX_SIZE
  const batchWait = Options.batchWaitMs || AppDefs.BATCH_WAIT_MS
  if (ValueBatch.length >= batchMax) {
    flushValueBatch()
  } else if (!ValueBatchTimer) {
    ValueBatchTimer = setTimeout(flushValueBatch, batchWait)
  }
}

// ---- Mongo main loop ----
const MongoStatus = { HintMongoIsConnected: false }
const deadBandCache = new Map() // last-sent analog values for dead-band

;(async () => {
  let clientMongo = null
  let rtCollection = null
  let cmdCollection = null
  let soeCollection = null
  let userActionsCollection = null
  let listenerServer = null
  let changeStream = null
  let soeChangeStream = null
  let heartbeatTimer = null
  let giTimer = null
  let statsTimer = null

  // periodic stats flush
  statsTimer = setInterval(async () => {
    if (!rtCollection || !ConnectionNumber) return
    if (!Redundancy.ProcessStateIsActive()) return
    if (Pusher) {
      Stats.notificationsSent = Pusher.stats.sent
      Stats.notificationsDropped = Pusher.stats.dropped
      Stats.webhookErrors = Pusher.stats.errors
      Stats.lastWebhookError = Pusher.stats.lastError
      Stats.queueSizes = Pusher.queueSizes()
    }
    // link state for the communication status tags: the last webhook POST
    // to n8n succeeded; before any POST, the inbound listener is up
    Stats.isConnected =
      Pusher && Pusher.stats.lastOk !== null
        ? Pusher.stats.lastOk
        : listenerServer !== null
    Stats.nodeName = configObj.nodeName
    Stats.timeTag = new Date()
    try {
      await clientMongo
        .db(configObj.mongoDatabaseName)
        .collection(configObj.ProtocolConnectionsCollectionName)
        .updateOne(
          { protocolConnectionNumber: ConnectionNumber },
          { $set: { stats: Stats } }
        )
    } catch (e) {}
  }, AppDefs.STATS_INTERVAL_MS)

  while (true) {
    try {
      if (clientMongo === null) {
        Log.log('MongoDB - Connecting...')
        const client = await MongoClient.connect(
          configObj.mongoConnectionString,
          configObj.MongoConnectionOptions
        )
        clientMongo = client
        MongoStatus.HintMongoIsConnected = true
        Log.log('MongoDB - Connected')

        const db = client.db(configObj.mongoDatabaseName)
        rtCollection = db.collection(configObj.RealtimeDataCollectionName)
        cmdCollection = db.collection(configObj.CommandsQueueCollectionName)
        soeCollection = db.collection(configObj.SoeDataCollectionName)
        userActionsCollection = db.collection(
          configObj.UserActionsCollectionName
        )

        // find the connection for this driver instance (one per instance)
        const conns = await db
          .collection(configObj.ProtocolConnectionsCollectionName)
          .find({
            protocolDriver: AppDefs.NAME,
            protocolDriverInstanceNumber: configObj.Instance,
          })
          .toArray()
        if (conns.length === 0) {
          Log.log('No protocol connection found for this instance!')
          process.exit(1)
        }
        Connection = conns[0]
        if (!('protocolConnectionNumber' in Connection)) {
          Log.log('No connection number on record!')
          process.exit(2)
        }
        if (Connection.enabled === false) {
          Log.log('Connection disabled, exiting!')
          process.exit(3)
        }
        ConnectionNumber = Connection.protocolConnectionNumber
        configObj.ConnectionNumber = ConnectionNumber
        Log.log('Connection - ' + ConnectionNumber + ' (' + Connection.name + ')')

        AutoCreateTags = Connection.autoCreateTags !== false

        // parse options JSON
        Options = {}
        if (typeof Connection.options === 'string' && Connection.options.trim() !== '') {
          try {
            Options = JSON.parse(Connection.options)
          } catch (e) {
            Log.log('Options - Invalid JSON in options field: ' + e.message)
          }
        }

        ParsedFilters = Filters.ParseFilters(Connection.topics)
        Log.log(
          'Filters - value rules: ' +
            ParsedFilters.valueRules.length +
            ', soe rules: ' +
            ParsedFilters.soeRules.length
        )

        // initialize auto key
        AutoKeyId = await Tags.GetAutoKeyInitialValue(
          rtCollection,
          ConnectionNumber
        )
        Log.log('Auto Key - Initial value: ' + AutoKeyId)

        // build outbound pusher
        Pusher = new WebhookPusher(Connection.endpointURLs || [], {
          bearerToken: Connection.passphrase,
          timeoutMs: Connection.timeoutMs || 10000,
          maxQueueSize: Connection.maxQueueSize || AppDefs.MAX_QUEUE_SIZE,
          retryMaxMs: Options.retryMaxMs || AppDefs.RETRY_MAX_MS,
          caFilePath: Connection.rootCertFilePath,
          rejectUnauthorized: Connection.chainValidation === true,
        })
        if (Pusher.hasTargets())
          Log.log(
            'Webhook - ' + Pusher.channels.length + ' outbound target(s)'
          )
        else Log.log('Webhook - No outbound targets configured')

        // start redundancy control
        Redundancy.Start(5000, clientMongo, db, configObj, MongoStatus)

        // start inbound listener
        listenerServer = startInboundListener(db)

        // start outbound change streams
        startOutboundStreams(rtCollection, soeCollection)

        // heartbeat
        const hbMs =
          Options.heartbeatMs !== undefined
            ? Options.heartbeatMs
            : AppDefs.HEARTBEAT_MS
        if (hbMs > 0) {
          clearInterval(heartbeatTimer)
          heartbeatTimer = setInterval(() => {
            if (Redundancy.ProcessStateIsActive() && Pusher && Pusher.hasTargets())
              Pusher.push(makeEnvelope('heartbeat', {}))
          }, hbMs)
        }

        // integrity (full snapshot) push
        const giSec = Connection.giInterval || 0
        if (giSec > 0) {
          clearInterval(giTimer)
          giTimer = setInterval(() => {
            pushIntegritySnapshot(rtCollection)
          }, giSec * 1000)
        }
      }

      await new Promise((r) => setTimeout(r, 5000))

      if (clientMongo && !(await checkConnectedMongo(clientMongo))) {
        Log.log('MongoDB - Disconnected!')
        MongoStatus.HintMongoIsConnected = false
        try {
          if (changeStream) changeStream.close()
          if (soeChangeStream) soeChangeStream.close()
          if (listenerServer) listenerServer.close()
        } catch (e) {}
        changeStream = null
        soeChangeStream = null
        listenerServer = null
        clearInterval(heartbeatTimer)
        clearInterval(giTimer)
        try {
          clientMongo.close()
        } catch (e) {}
        clientMongo = null
      }
    } catch (e) {
      Log.log('MongoDB - Connection error: ' + e.message)
      MongoStatus.HintMongoIsConnected = false
      clientMongo = null
      await new Promise((r) => setTimeout(r, 5000))
    }
  }

  // ---- helpers bound to the current connection ----

  function startInboundListener(db) {
    // optional TLS material for the listener
    let tls = null
    if (
      Connection.localCertFilePath &&
      Connection.localCertFilePath.trim() !== '' &&
      Connection.privateKeyFilePath &&
      Connection.privateKeyFilePath.trim() !== ''
    ) {
      try {
        tls = {
          cert: fs.readFileSync(Connection.localCertFilePath),
          key: fs.readFileSync(Connection.privateKeyFilePath),
          ca:
            Connection.rootCertFilePath &&
            Connection.rootCertFilePath.trim() !== ''
              ? fs.readFileSync(Connection.rootCertFilePath)
              : undefined,
        }
      } catch (e) {
        Log.log('Listener - TLS material error, falling back to http: ' + e.message)
        tls = null
      }
    }

    // parse bind address:port
    let bindAddress = AppDefs.DEFAULT_BIND_ADDRESS
    let bindPort = AppDefs.DEFAULT_BIND_PORT
    if (
      typeof Connection.ipAddressLocalBind === 'string' &&
      Connection.ipAddressLocalBind.trim() !== ''
    ) {
      const aux = Connection.ipAddressLocalBind.split(':')
      if (aux[0].trim() !== '') bindAddress = aux[0].trim()
      if (aux.length > 1 && !isNaN(parseInt(aux[1]))) bindPort = parseInt(aux[1])
    }
    if (process.env[AppDefs.ENV_PREFIX + 'BIND_PORT'])
      bindPort = parseInt(process.env[AppDefs.ENV_PREFIX + 'BIND_PORT'])
    if (process.env[AppDefs.ENV_PREFIX + 'BIND_ADDRESS'])
      bindAddress = process.env[AppDefs.ENV_PREFIX + 'BIND_ADDRESS']

    const enableDirectCommands =
      Connection.commandsEnabled !== false && Options.enableDirectCommands === true

    return StartListener({
      configObj,
      connection: Connection,
      collections: {
        rt: rtCollection,
        cmd: cmdCollection,
        userActions: userActionsCollection,
      },
      isActive: () => Redundancy.ProcessStateIsActive(),
      allocKey,
      listCreatedTags: ListCreatedTags,
      autoCreateTags: AutoCreateTags,
      stats: Stats,
      bindAddress,
      bindPort,
      allowedIps: Connection.ipAddresses || [],
      basicUser: Connection.username || '',
      basicPass: Connection.password || '',
      enableDirectCommands,
      tls,
    })
  }

  function startOutboundStreams(rtColl, soeColl) {
    if (!Pusher || !Pusher.hasTargets()) return
    const notifyValueChanges = Options.notifyValueChanges !== false

    if (notifyValueChanges) {
      // narrow server-side when all rules are group1-based
      const group1Match = Filters.BuildChangeStreamGroup1Match(
        ParsedFilters.valueRules
      )
      const csPipeline = [
        { $project: { documentKey: false } },
        {
          $match: {
            $and: [
              {
                $or: [
                  {
                    $and: [
                      {
                        'updateDescription.updatedFields.sourceDataUpdate': {
                          $exists: false,
                        },
                      },
                      { 'fullDocument._id': { $gt: 0 } },
                      { operationType: 'update' },
                    ],
                  },
                  { operationType: 'replace' },
                ],
              },
              ...(group1Match ? [group1Match] : []),
            ],
          },
        },
      ]
      changeStream = rtColl.watch(csPipeline, { fullDocument: 'updateLookup' })
      changeStream.on('error', () => {
        Log.log('MongoDB - ChangeStream error (realtimeData)')
        if (clientMongo) {
          try {
            clientMongo.close()
          } catch (e) {}
          clientMongo = null
        }
      })
      changeStream.on('change', (change) => {
        if (!Redundancy.ProcessStateIsActive()) return
        const doc = change.fullDocument
        if (!doc || parseInt(doc._id) <= 0) return
        if (!Filters.MatchValue(doc, ParsedFilters.valueRules)) return
        // numeric dead-band for analog points
        const deadBand = Connection.deadBand || 0
        if (deadBand > 0 && doc.type === 'analog') {
          const key = doc.tag
          const last = deadBandCache.get(key)
          const v = parseFloat(doc.value)
          if (last !== undefined && Math.abs(v - last) < deadBand && !doc.invalid)
            return
          deadBandCache.set(key, v)
        }
        enqueueValue(pointFromDoc(doc))
      })
      Log.log('Outbound - watching realtimeData value changes')
    }

    if (ParsedFilters.soeRules.length > 0 && Options.notifySoe !== false) {
      const soePipeline = [
        { $match: { operationType: 'insert' } },
      ]
      soeChangeStream = soeColl.watch(soePipeline, {
        fullDocument: 'updateLookup',
      })
      soeChangeStream.on('error', () => {
        Log.log('MongoDB - ChangeStream error (soeData)')
      })
      soeChangeStream.on('change', (change) => {
        if (!Redundancy.ProcessStateIsActive()) return
        const ev = change.fullDocument
        if (!ev) return
        if (!Filters.MatchSoe(ev, ParsedFilters.soeRules)) return
        if (!Pusher || !Pusher.hasTargets()) return
        Pusher.push(
          makeEnvelope('soeEvent', {
            events: [
              {
                tag: ev.tag,
                pointKey: ev.pointKey ? parseInt(ev.pointKey) : null,
                eventText: ev.eventText,
                priority: ev.priority,
                group1: ev.group1 || '',
                description: ev.description || '',
                invalid: ev.invalid === true,
                ack: ev.ack,
                timeTag: ev.timeTag || null,
                timeTagAtSource: ev.timeTagAtSource || null,
              },
            ],
          })
        )
      })
      Log.log('Outbound - watching soeData events')
    }
  }

  async function pushIntegritySnapshot(rtColl) {
    if (!Redundancy.ProcessStateIsActive() || !Pusher || !Pusher.hasTargets())
      return
    try {
      // build a query from group1-based rules where possible, else scan all pointKey>0
      const group1Match = Filters.BuildChangeStreamGroup1Match(
        ParsedFilters.valueRules
      )
      const query = { _id: { $gt: 0 } }
      if (group1Match) query.group1 = group1Match['fullDocument.group1']
      const cursor = rtColl.find(query).project({
        tag: 1,
        value: 1,
        valueString: 1,
        valueJson: 1,
        type: 1,
        invalid: 1,
        substituted: 1,
        alarmed: 1,
        timeTag: 1,
        timeTagAtSource: 1,
        group1: 1,
        group2: 1,
        group3: 1,
        description: 1,
        unit: 1,
      })
      const batchMax = Options.batchMaxSize || AppDefs.BATCH_MAX_SIZE
      let batch = []
      for await (const doc of cursor) {
        if (!Filters.MatchValue(doc, ParsedFilters.valueRules)) continue
        batch.push(pointFromDoc(doc))
        if (batch.length >= batchMax) {
          Pusher.push(makeEnvelope('integrity', { points: batch }))
          batch = []
        }
      }
      if (batch.length > 0)
        Pusher.push(makeEnvelope('integrity', { points: batch }))
      Log.log('Outbound - integrity snapshot pushed')
    } catch (e) {
      Log.log('Outbound - integrity snapshot error: ' + e.message)
    }
  }
})()

// test mongoDB connectivity
let CheckMongoConnectionTimeout = 10000
async function checkConnectedMongo(client) {
  if (!client) return false
  let tr = setTimeout(() => {
    Log.log('MongoDB - Ping timeout!')
    MongoStatus.HintMongoIsConnected = false
  }, CheckMongoConnectionTimeout)
  let res = null
  try {
    res = await client.db('admin').command({ ping: 1 })
    clearTimeout(tr)
  } catch (e) {
    clearTimeout(tr)
    return false
  }
  if ('ok' in res && res.ok) {
    MongoStatus.HintMongoIsConnected = true
    return true
  }
  MongoStatus.HintMongoIsConnected = false
  return false
}
