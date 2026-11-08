/*
 * Demo process simulator: drives the {json:scada} demo database with a
 * physically coherent electrical simulation and answers supervisory commands.
 *
 * The electrical behavior lives in ./power-model.js, this file only takes care
 * of the MongoDB plumbing: loading the point list, publishing sourceDataUpdate
 * documents (exactly like a protocol driver would) and consuming commandsQueue.
 *
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
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

const Log = require('./simple-logger')
const AppDefs = require('./app-defs')
const LoadConfig = require('./load-config')
const { LoadSimConfig } = require('./sim-config')
const { PowerModel } = require('./power-model')
const { MongoClient, Double } = require('mongodb')

process.on('uncaughtException', (err) =>
  Log.log('Uncaught Exception: ' + (err?.stack || err?.message || err))
)

const commandsPipeline = [
  { $project: { documentKey: false } },
  { $match: { $and: [{ $or: [{ operationType: 'insert' }] }] } },
]

const jsConfig = LoadConfig()
const simConfig = LoadSimConfig()
const model = new PowerModel(simConfig)

let HintMongoIsConnected = true
let clientMongo = null
let cntUpd = 1
let modelReady = false
let lastStepAt = Date.now()
let lastModelLoadAt = 0
let publishing = false

Log.log('Connecting to ' + jsConfig.mongoConnectionString)
Log.log(
  'Simulation - step ' +
    simConfig.STEP_PERIOD_MS +
    ' ms, load cycle ' +
    simConfig.LOAD_CYCLE_MINUTES +
    ' min, faults ' +
    (simConfig.ENABLE_FAULTS ? simConfig.FAULT_RATE_PER_HOUR + '/h' : 'off') +
    ', alarms ' +
    (simConfig.ENABLE_ALARMS ? simConfig.ALARM_RATE_PER_HOUR + '/h' : 'off')
)

// -------------------------------------------------------------- data access

const POINT_PROJECTION = {
  _id: 1,
  tag: 1,
  type: 1,
  origin: 1,
  unit: 1,
  value: 1,
  valueDefault: 1,
  group1: 1,
  group2: 1,
  description: 1,
  ungroupedDescription: 1,
  stateTextTrue: 1,
  stateTextFalse: 1,
}

// Connections whose points another writer owns: CIM-EMS results and UNDEFINED
// connections (fed by an external simulator such as the Svedala demo field).
const FOREIGN_DRIVERS = ['CIM-EMS', 'UNDEFINED']
let foreignConnections = []

async function loadForeignConnections(db) {
  const conns = await db
    .collection(jsConfig.ProtocolConnectionsCollectionName)
    .find(
      { protocolDriver: { $in: FOREIGN_DRIVERS } },
      { projection: { protocolConnectionNumber: 1 } }
    )
    .toArray()
  foreignConnections = conns.map((c) => c.protocolConnectionNumber)
  return foreignConnections
}

async function loadModel(db) {
  await loadForeignConnections(db)
  const docs = await db
    .collection(jsConfig.RealtimeDataCollectionName)
    .find(
      {
        origin: 'supervised',
        type: { $in: ['analog', 'digital'] },
        protocolSourceConnectionNumber: { $nin: foreignConnections },
      },
      { projection: POINT_PROJECTION }
    )
    .toArray()

  if (docs.length === 0) {
    Log.log('Model - no supervised points found, is the demo database loaded?')
    return false
  }

  if (modelReady) model.snapshot()
  model.build(docs)
  if (!modelReady) model.coherenceFixes()
  lastModelLoadAt = Date.now()
  modelReady = true
  return true
}

// build the sourceDataUpdate subdocument a protocol driver would write
function sourceDataUpdate(upd) {
  const now = new Date()
  const timeTagged = upd.isDigital || upd.isTap
  return {
    valueAtSource: new Double(upd.value),
    valueStringAtSource: '',
    asduAtSource:
      upd.isDigital ? 'M_SP_TB_1'
      : upd.isTap ? 'M_ST_TB_1'
      : 'M_ME_NC_1',
    // 3 = spontaneous, 20 = interrogated by group
    causeOfTransmissionAtSource: upd.changed ? '3' : '20',
    timeTag: now,
    timeTagAtSource: timeTagged ? now : null,
    timeTagAtSourceOk: timeTagged,
    invalid: false,
    invalidAtSource: false,
    substitutedAtSource: false,
    overflowAtSource: false,
    blockedAtSource: false,
    notTopicalAtSource: false,
    test: true,
    originator: AppDefs.NAME,
    CntUpd: cntUpd,
  }
}

async function publishUpdates(db, updates) {
  if (!updates.length) return 0
  const ops = updates.map((upd) => ({
    updateOne: {
      filter: { _id: upd._id },
      update: { $set: { sourceDataUpdate: sourceDataUpdate(upd) } },
    },
  }))
  cntUpd++

  // chunked to keep each bulk write comfortably below the BSON limits
  let written = 0
  const CHUNK = 500
  for (let i = 0; i < ops.length; i += CHUNK) {
    const res = await db
      .collection(jsConfig.RealtimeDataCollectionName)
      .bulkWrite(ops.slice(i, i + CHUNK), { ordered: false })
    written += res?.modifiedCount || 0
  }
  return written
}

// ------------------------------------------------------------- simulation

async function simulationStep() {
  if (clientMongo === null || !HintMongoIsConnected || !modelReady) return
  if (publishing) return
  publishing = true
  try {
    const db = clientMongo.db(jsConfig.mongoDatabaseName)
    const now = Date.now()
    const dt = Math.min(Math.max((now - lastStepAt) / 1000, 0.05), 30)
    lastStepAt = now

    model.step(dt)
    const updates = model.collectUpdates(now)
    const written = await publishUpdates(db, updates)
    Log.log(
      'Step - ' +
        updates.length +
        ' points published (' +
        written +
        ' written) | ' +
        model.summary(),
      2
    )

    if (
      simConfig.MODEL_REFRESH_MINUTES > 0 &&
      now - lastModelLoadAt > simConfig.MODEL_REFRESH_MINUTES * 60000
    )
      await loadModel(db)
  } catch (err) {
    Log.log('Step - ' + err.message)
    if (String(err.message).indexOf('ECONNREFUSED') > -1) clientMongo = null
  } finally {
    publishing = false
  }
}

// --------------------------------------------------------------- commands

async function handleCommand(db, cmdDoc) {
  if (foreignConnections.includes(cmdDoc.protocolSourceConnectionNumber)) return
  Log.log('Command - ' + cmdDoc.tag + ' = ' + cmdDoc.value)

  const cmdPoint = await db
    .collection(jsConfig.RealtimeDataCollectionName)
    .findOne({ tag: cmdDoc.tag }, { projection: { supervisedOfCommand: 1 } })

  if (!cmdPoint) {
    Log.log('Command - unknown command tag ' + cmdDoc.tag)
    return
  }

  const supervisedId = cmdPoint.supervisedOfCommand
  const value = Number(cmdDoc.value)
  let handled = model.applyCommand(supervisedId, value)

  if (!handled) {
    // point outside the simulated model (manual origin, new tag, ...):
    // mirror the commanded value like the previous simulator did
    handled = await mirrorCommand(db, supervisedId, cmdDoc, value)
  } else {
    // publish the effect quickly instead of waiting for the next step
    setTimeout(simulationStep, simConfig.BREAKER_TRAVEL_MS + 100)
  }

  if (handled)
    await db
      .collection(jsConfig.CommandsQueueCollectionName)
      .updateOne(
        { _id: cmdDoc._id },
        { $set: { ack: true, ackTimeTag: new Date() } }
      )
}

async function mirrorCommand(db, supervisedId, cmdDoc, value) {
  const supervised = await db
    .collection(jsConfig.RealtimeDataCollectionName)
    .findOne(
      { _id: supervisedId },
      { projection: { type: 1, value: 1, tag: 1 } }
    )
  if (!supervised) {
    Log.log('Command - no supervised point for ' + cmdDoc.tag)
    return false
  }
  const isTap = String(cmdDoc.tag).indexOf('YTAP') !== -1
  const newValue =
    isTap ? Number(supervised.value) + (value === 0 ? -1 : 1) : value
  const res = await db
    .collection(jsConfig.RealtimeDataCollectionName)
    .updateOne(
      { _id: supervisedId },
      {
        $set: {
          sourceDataUpdate: sourceDataUpdate({
            value: newValue,
            isDigital: supervised.type === 'digital',
            isTap: isTap,
            changed: true,
          }),
        },
      }
    )
  cntUpd++
  return (res?.matchedCount || 0) > 0
}

// ------------------------------------------------------------ housekeeping

async function housekeeping() {
  if (clientMongo === null || !HintMongoIsConnected) return
  const db = clientMongo.db(jsConfig.mongoDatabaseName)

  // fake IEC 104 driver running
  await db
    .collection(jsConfig.ProtocolDriverInstancesCollectionName)
    .updateOne({ protocolDriver: 'IEC60870-5-104' }, [
      { $set: { activeNodeKeepAliveTimeTag: '$$NOW' } },
    ])
    .catch((err) => {
      Log.log(err.message)
      if (String(err.message).indexOf('ECONNREFUSED') > -1) clientMongo = null
    })

  // revalidate points that went invalid (e.g. after a restart), never the
  // points of foreign connections (an invalid CIM-EMS result means "not calculated")
  await loadForeignConnections(db).catch((err) => Log.log(err.message))
  await db
    .collection(jsConfig.RealtimeDataCollectionName)
    .updateMany(
      {
        origin: 'supervised',
        invalid: true,
        protocolSourceConnectionNumber: { $nin: foreignConnections },
      },
      [
        {
          $set: {
            invalid: false,
            timeTag: '$$NOW',
            'sourceDataUpdate.timeTag': '$$NOW',
          },
        },
      ]
    )
    .catch((err) => {
      Log.log(err.message)
      if (String(err.message).indexOf('ECONNREFUSED') > -1) clientMongo = null
    })
}

// ------------------------------------------------------------------- main

;(async () => {
  setInterval(simulationStep, simConfig.STEP_PERIOD_MS)
  setInterval(housekeeping, 11777)

  while (true) {
    if (clientMongo === null) {
      modelReady = false
      await MongoClient.connect(
        jsConfig.mongoConnectionString,
        jsConfig.MongoConnectionOptions
      )
        .then(async (client) => {
          clientMongo = client
          HintMongoIsConnected = true
          Log.log('Connected correctly to MongoDB server')

          const db = client.db(jsConfig.mongoDatabaseName)
          await loadModel(db)
          lastStepAt = Date.now()

          const changeStream = db
            .collection(jsConfig.CommandsQueueCollectionName)
            .watch(commandsPipeline, { fullDocument: 'updateLookup' })

          changeStream
            .on('change', async (change) => {
              if (change.operationType !== 'insert') return
              try {
                await handleCommand(db, change.fullDocument)
              } catch (err) {
                Log.log('Command - ' + err.message)
              }
            })
            .on('error', (err) => {
              if (clientMongo) clientMongo.close()
              clientMongo = null
              Log.log(err.message)
            })
        })
        .catch(function (err) {
          if (clientMongo) clientMongo.close()
          clientMongo = null
          Log.log(err.message)
        })
    }

    await new Promise((resolve) => setTimeout(resolve, 5000))

    if (clientMongo === undefined) {
      Log.log('Disconnected Mongodb!')
      clientMongo = null
    }
    if (clientMongo)
      if (!(await checkConnectedMongo(clientMongo))) {
        Log.log('Disconnected Mongodb!')
        if (clientMongo) clientMongo.close()
        clientMongo = null
      }
  }
})()

// test mongoDB connectivity
async function checkConnectedMongo(client) {
  if (!client) {
    return false
  }
  const CheckMongoConnectionTimeout = 10000
  const tr = setTimeout(() => {
    Log.log('Mongo ping timeout error!')
    HintMongoIsConnected = false
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
    HintMongoIsConnected = true
    return true
  } else {
    HintMongoIsConnected = false
    return false
  }
}
