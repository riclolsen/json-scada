'use strict'

/*
 * Customizable processor of mongodb changes via change streams.
 *
 * THIS FILE IS INTENDED TO BE CUSTOMIZED BY USERS TO DO SPECIAL PROCESSING
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

const Log = require('./simple-logger')
const AppDefs = require('./app-defs')
// const { Double } = require('mongodb')
const { setTimeout } = require('node:timers')
const dgram = require('node:dgram')
const Queue = require('queue-fifo')
// const zlib = require('fast-zlib')
const zlib = require('zlib')

// UDP broadcast options
const udpPort = AppDefs.UDP_PORT
const udpHostDst = AppDefs.IP_DESTINATION
// Create a UDP socket
const udpSocket = dgram.createSocket('udp4')

udpSocket.bind(udpPort, () => {
  udpSocket.setBroadcast(true)
  // udpSocket.setMulticastInterface('::%eth1');
})

const UserActionsCollectionName = 'userActions'
const RealtimeDataCollectionName = 'realtimeData'
const CommandsQueueCollectionName = 'commandsQueue'
const SoeDataCollectionName = 'soeData'
const ProcessInstancesCollectionName = 'processInstances'
const ProtocolDriverInstancesCollectionName = 'protocolDriverInstances'
const ProtocolConnectionsCollectionName = 'protocolConnections'
const BackfillDataCollectionName = 'backfillData'

const chgQueue = new Queue() // queue of changes
const backfillQueue = new Queue() // queue of backfill data

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

// this will be called by the main module when mongo is connected (or reconnected)
module.exports.CustomProcessor = async function (
  clientMongo,
  jsConfig,
  Redundancy,
  MongoStatus
) {
  if (clientMongo === null) return
  const db = clientMongo.db(jsConfig.mongoDatabaseName)

  while (!Redundancy.ProcessStateIsActive()) {
    if (!MongoStatus.HintMongoIsConnected) return // exit if mongo is not connected
    Log.log('Custom Process - Waiting for process to be active...')
    await sleep(1000)
  }

  // create time series collection for backfilling
  await db
    .createCollection(BackfillDataCollectionName, {
      timeseries: {
        timeField: 'timestamp',
        metaField: 'metadata',
        // granularity: 'minutes',
        bucketMaxSpanSeconds: 3600,
        bucketRoundingSeconds: 3600,
      },
      expireAfterSeconds: 24 * 60 * 60 * AppDefs.BACKFILL_EXPIRATION,
    })
    .then(() => {
      Log.log('Created backfillData collection.')
    })
    .catch((e) => {
      Log.log('Collection backfillData already exists or error creating.')
    })

  // queue point database sync data
  const dbSync = () => {
    pointDatabaseSync(db, Redundancy, MongoStatus)
    setTimeout(dbSync, 1000 * AppDefs.INTERVAL_INTEGRITY)
  }
  dbSync()

  const procBackfill = () => {
    backfillQueueProcess(db, Redundancy, MongoStatus)
    setTimeout(procBackfill, 1000)
  }
  procBackfill()

  // set up change streams monitoring for updates
  const changeStreamUserActions = db
    .collection(RealtimeDataCollectionName)
    .watch(
      [
        {
          $match: {
            $and: [
              // { 'fullDocument.value': { $exists: true } },
              // DivideProcessingExpression,
              {
                'updateDescription.updatedFields.sourceDataUpdate': {
                  $exists: true,
                },
              },
              { operationType: 'update' },
            ],
          },
        },
      ],
      {
        fullDocument: 'updateLookup',
      }
    )

  try {
    changeStreamUserActions.on('error', (change) => {
      if (clientMongo) clientMongo.close()
      clientMongo = null
      Log.log('Custom Process - Error on changeStreamUserActions!')
    })
    changeStreamUserActions.on('close', (change) => {
      Log.log('Custom Process - Closed changeStreamUserActions!')
    })
    changeStreamUserActions.on('end', (change) => {
      if (clientMongo) clientMongo.close()
      clientMongo = null
      Log.log('Custom Process - Ended changeStreamUserActions!')
    })

    // start listen to changes
    changeStreamUserActions.on('change', (change) => {
      // Log.log(JSON.stringify(change.fullDocument))
      if (
        !Redundancy.ProcessStateIsActive() ||
        !MongoStatus.HintMongoIsConnected
      )
        return // do nothing if process is inactive

      // will process only update data from drivers
      if (!change?.updateDescription?.updatedFields?.sourceDataUpdate) return
      if (!change?.operationType) return

      chgQueue.enqueue(change)
    })
  } catch (e) {
    Log.log('Custom Process - Error: ' + e)
  }
}

let maxSz = 0
let cntChg = 0
let pktCnt = 0

setTimeout(procQueue, 1000)
async function procQueue() {
  let cntSeq = 0
  while (!chgQueue.isEmpty()) {
    let fwArr = []
    let bfArr = []
    let strSz = 0
    while (!chgQueue.isEmpty()) {
      const change = chgQueue.peek()
      chgQueue.dequeue()

      if (
        change?.updateDescription?.updatedFields?.sourceDataUpdate
          ?.valueJsonAtSource &&
        change.updateDescription.updatedFields.sourceDataUpdate
          .valueJsonAtSource?.length &&
        change.updateDescription.updatedFields.sourceDataUpdate
          .valueJsonAtSource.length >
          AppDefs.MAX_LENGTH_JSON / 2
      ) {
        delete change.updateDescription.updatedFields.sourceDataUpdate
          .valueJsonAtSource // remove this field that can be too large to send via UDP
      }
      if (
        change?.updateDescription?.updatedFields?.sourceDataUpdate
          ?.valueBsonAtSource
      )
        delete change.updateDescription.updatedFields.sourceDataUpdate
          .valueBsonAtSource // remove this field that can be too large to send via UDP
      if (change?.updateDescription?.truncatedArrays)
        delete change.updateDescription.truncatedArrays
      if (change?.updateDescription?.removedFields)
        delete change.updateDescription.removedFields

      cntChg++
      const obj = {
        cnt: cntChg,
        tag: change?.fullDocument?.tag,
        operationType: change.operationType,
        documentKey: change.documentKey,
        updateDescription: change.updateDescription,
      }
      const chgLen = JSON.stringify(obj).length
      if (chgLen > AppDefs.MAX_LENGTH_JSON) {
        Log.log('Discarded change too large: ' + chgLen)
        cntChg--
        continue
      }
      strSz += chgLen
      fwArr.push(obj)
      if (change.operationType === 'update') bfArr.push(obj)
      if (strSz > AppDefs.PACKET_SIZE_THRESHOLD) break
    }
    if (bfArr.length > 0) backfillQueue.enqueue(bfArr)
    const opData = JSON.stringify(fwArr)
    // const deflate = new zlib.Deflate()
    // const message = deflate.process(opData)
    const message = zlib.deflateSync(opData)

    Log.log(opData.length + ' ' + message.length)
    Log.log('Objects: ' + fwArr.length)

    const buff = Buffer.from(message)
    await udpSocket.send(
      buff,
      0,
      buff.length,
      udpPort,
      udpHostDst,
      (err, bytes) => {
        if (err) {
          Log.log('UDP error:' + err)
        }
      }
    )
    if (buff.length > maxSz) maxSz = buff.length
    pktCnt++
    if (AppDefs.PACKETS_INTERVAL > 0) {
      await sleep(AppDefs.PACKETS_INTERVAL)
    }
    // Log.log('Data sent via UDP' + opData);
    Log.log('Backfill Queue Size: ' + backfillQueue.size())
    Log.log('Changes Queue Size: ' + chgQueue.size())
    Log.log('UDP Msg Size: ' + buff.length)
    Log.log('MaxMsg Size: ' + maxSz)
    Log.log('Seq count ' + cntSeq++)
    Log.log('                  Chg count ' + cntChg)
    Log.log('                  UDP count ' + pktCnt)
    if (
      cntSeq > AppDefs.MAX_SEQUENCE_OF_UDPMSGS ||
      buff.length > AppDefs.PACKET_SIZE_BREAK_SEQ
    ) {
      setTimeout(
        procQueue,
        AppDefs.INTERVAL_AFTER_UDPMSGS_SEQ + 100 * parseInt(buff.length / 1500)
      )
      return
    }
  }

  setTimeout(procQueue, AppDefs.INTERVAL_AFTER_EMPTY_QUEUE)
}

// fetch all documents from realtimeData and queue data to forward via UDP
async function pointDatabaseSync(db, Redundancy, MongoStatus) {
  if (!Redundancy.ProcessStateIsActive() || !MongoStatus.HintMongoIsConnected)
    return // do nothing if process is inactive

  const findResult = db.collection(RealtimeDataCollectionName).find({})
  for await (const doc of findResult) {
    if (doc?.sourceDataUpdate) delete doc.sourceDataUpdate
    if (doc?.protocolDestinations) delete doc.protocolDestinations

    const strDoc = JSON.stringify(doc)
    if (strDoc.length > AppDefs.MAX_LENGTH_JSON) {
      Log.log(
        'Ignored document too large on ' +
          RealtimeDataCollectionName +
          ': ' +
          strDoc
      )
      continue
    }
    chgQueue.enqueue({
      operationType: 'integrity',
      fullDocument: { tag: doc.tag },
      documentKey: { _id: doc._id },
      updateDescription: { updatedFields: { ...doc } },
    })
  }
}

// insert documents queued in backfillQueue collection
async function backfillQueueProcess(db, Redundancy, MongoStatus) {
  if (!Redundancy.ProcessStateIsActive() || !MongoStatus.HintMongoIsConnected)
    return // do nothing if process is inactive

  const backfillArr = []
  while (!backfillQueue.isEmpty()) {
    const bf = backfillQueue.peek()
    backfillQueue.dequeue()

    for (const obj of bf) {
      backfillArr.push({
        timestamp: obj.updateDescription.updatedFields.timestamp || new Date(),
        metadata: { tag: obj.tag },
        data: obj,
      })
    }
  }
  if (backfillArr.length === 0) return
  const result = await db
    .collection(BackfillDataCollectionName)
    .insertMany(backfillArr)
  Log.log('Backfill: Inserted ' + result.insertedCount + ' documents.')
}
