/*
 * One way replication mechanism (mongofw/mongowr)
 * Watch for changes via changestream of realtimeData/sourceDataUpdate, send changes via UDP.
 * Query realtimeData for full point list sync.
 * Store changes on backfillData (timeseries collection) to keep replaying previous changes from a defined period.
 * Replay marks historical values with isHistorical=true. Integrity scans resend deletions.
 * Forward new secEvents documents and replay recent events from their source collection.
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
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')
const { BSON } = require('mongodb')
const { signPacket } = require('./udp-auth')
const { setTimeout: sleep } = require('node:timers/promises')
const dgram = require('node:dgram')
const { once } = require('node:events')
const Queue = require('queue-fifo')
const zlib = require('node:zlib')

const RealtimeDataCollectionName = 'realtimeData'
const BackfillDataCollectionName = 'backfillData'
const SecEventsCollectionName = 'secEvents'
const queueLimit = 1000
const integrityInterval = 1000 * AppDefs.INTERVAL_INTEGRITY
let cntChg = 0

// The connection loop owns this worker. Each activation gets its own stream and queues.
module.exports.CustomProcessor = async function (
  clientMongo, jsConfig, Redundancy, MongoStatus, connectionSignal
) {
  const db = clientMongo.db(jsConfig.mongoDatabaseName)
  const isActive = () =>
    !connectionSignal.aborted &&
    MongoStatus.HintMongoIsConnected &&
    Redundancy.ProcessStateIsActive()

  while (!connectionSignal.aborted) {
    if (!isActive()) {
      await sleep(250, undefined, { signal: connectionSignal })
      continue
    }
    await forward(db, jsConfig.UdpAuthKey, isActive, connectionSignal)
  }
}

async function forward(db, authKey, isActive, connectionSignal) {
  const realtime = db.collection(RealtimeDataCollectionName)
  const backfill = db.collection(BackfillDataCollectionName)
  const secEvents = db.collection(SecEventsCollectionName, { promoteValues: false })
  const knownPoints = db.collection('replicationSourcePoints')
  const live = new Queue()
  const background = new Queue()
  const archive = new Queue()
  const controller = new AbortController()
  const { signal } = controller
  const stop = () => controller.abort()
  const active = () => !signal.aborted && isActive()
  const pause = (ms) => sleep(ms, undefined, { signal })
  const activityCheck = setInterval(() => {
    if (!isActive()) stop()
  }, 250)
  connectionSignal.addEventListener('abort', stop, { once: true })

  const udpSocket = dgram.createSocket('udp4')
  const onSocketError = (error) => {
    Log.log('UDP socket error: ' + error)
    stop()
  }
  udpSocket.on('error', onSocketError)

  const streams = []
  let streamsClosing
  const closeStreams = () => {
    streamsClosing = Promise.all(streams.map((stream) =>
      stream.close().catch((error) => Log.log('Closing change stream: ' + error))
    ))
  }
  signal.addEventListener('abort', closeStreams, { once: true })
  const workers = []
  try {
    const listening = once(udpSocket, 'listening', { signal })
    udpSocket.bind(AppDefs.UDP_PORT)
    await listening
    udpSocket.setBroadcast(true)

    try {
      await db.createCollection(BackfillDataCollectionName, {
        timeseries: {
          timeField: 'timestamp',
          metaField: 'metadata',
          bucketMaxSpanSeconds: 3600,
          bucketRoundingSeconds: 3600,
        },
        expireAfterSeconds: 24 * 60 * 60 * AppDefs.BACKFILL_EXPIRATION,
      })
    } catch (error) {
      if (error.code !== 48) throw error // collection already exists
    }

    // Anchor the security event stream before its independent replay starts.
    const { operationTime } = await db.command({ ping: 1 })
    if (!operationTime) throw new Error('MongoDB did not return an operation time')
    if (!active()) return

    const start = (worker) => {
      const task = worker().catch((error) => {
        if (!signal.aborted && isActive()) throw error
      })
      workers.push(task)
      return task
    }
    const enqueue = async (queue, entry) => {
      while (active() && queue.size() >= queueLimit) await pause(25)
      if (active()) queue.enqueue(entry)
    }
    const enqueueLive = (entry) => enqueue(live, entry)
    const enqueueBackground = (entry) => enqueue(background, entry)
    const sending = start(() =>
      sendChanges(udpSocket, authKey, live, background, archive, knownPoints, active, pause))
    const storing = start(() => storeBackfill(backfill, archive, active, pause))
    const capturingEvents = start(async () => {
      const stream = secEvents.watch(
        [{ $match: { operationType: 'insert' } }],
        { startAtOperationTime: operationTime, maxAwaitTimeMS: 1000 }
      )
      streams.push(stream)
      for await (const { fullDocument } of stream) {
        if (!active()) break
        await enqueueSecEvent(fullDocument, enqueueLive)
      }
      if (active()) throw new Error('Security event change stream ended')
    })
    const replayingEvents = start(async () => {
      while (active()) {
        await replayDocuments(secEvents, 'timeTag', background, active, pause,
          (event) => enqueueSecEvent(event, enqueueBackground))
        if (active()) await pause(integrityInterval)
      }
    })
    let publishSnapshotTime
    const snapshotStarted = new Promise((resolve) => { publishSnapshotTime = resolve })
    const initialSync = start(() =>
      enqueueDbSync(db, realtime, knownPoints, enqueueBackground, active, publishSnapshotTime))
    const capturing = start(async () => {
      const snapshotTime = await Promise.race([snapshotStarted, initialSync])
      if (!active() || !snapshotTime) return
      const stream = realtime.watch(
        [{
          $match: {
            $or: [
              { operationType: { $in: ['insert', 'replace', 'delete'] } },
              {
                operationType: 'update',
                $or: [
                  { 'updateDescription.updatedFields.sourceDataUpdate': { $exists: true } },
                  { 'updateDescription.updatedFields.tag': { $exists: true } },
                ],
              },
            ],
          },
        }],
        { fullDocument: 'updateLookup', startAtOperationTime: snapshotTime, maxAwaitTimeMS: 1000 }
      )
      streams.push(stream)

      const syncing = start(async () => {
        await initialSync
        while (active()) {
          await pause(integrityInterval)
          if (!active()) break
          await enqueueDbSync(db, realtime, knownPoints, enqueueBackground, active)
        }
      })
      const replaying = start(async () => {
        await initialSync
        while (active()) {
          await replayBackfill(backfill, knownPoints, background, enqueueBackground, active, pause)
          if (active()) await pause(integrityInterval)
        }
      })
      const watching = start(async () => {
        for await (const change of stream) {
          if (!active()) break
          await enqueueChange(change, enqueueLive)
        }
        if (active()) throw new Error('Realtime change stream ended')
      })
      await Promise.race([syncing, replaying, watching])
    })
    await Promise.race([sending, storing, capturing, capturingEvents, replayingEvents])
  } finally {
    stop()
    clearInterval(activityCheck)
    connectionSignal.removeEventListener('abort', stop)
    await Promise.allSettled(workers)
    await streamsClosing
    udpSocket.close()
  }
}

async function enqueueSecEvent(event, enqueue) {
  if (!(event?._id instanceof BSON.ObjectId) || !(event.timeTag instanceof Date)) return
  const eventBson = BSON.serialize(event).toString('base64')
  await enqueue({ operationType: 'secEvent', eventBson })
}

function sourceRevision(time, event = '') {
  // MongoDB cluster time orders topology changes across source HA nodes. The
  // resume token orders entries sharing one transaction's cluster timestamp.
  return time.toString().padStart(20, '0') + '/' + event
}

async function enqueueIntegrity(doc, revision, enqueue) {
  if (!doc || typeof doc.tag !== 'string' || !doc.tag) return
  const { sourceDataUpdate, protocolDestinations, replicationRevision, replicatedSourceId, ...fields } = doc
  await enqueue({
    revision,
    tag: doc.tag,
    operationType: 'integrity',
    documentKey: { _id: doc._id },
    updateDescription: { updatedFields: fields },
  })
}

async function enqueueDbSync(db, realtime, knownPoints, enqueue, active, onSnapshotStarted) {
  Log.log('Begin enqueue of db sync data...')
  let count = 0
  const session = db.client.startSession({ snapshot: true })
  try {
    const cursor = realtime.find({}, { session })
    await cursor.hasNext()
    const time = session.snapshotTime
    if (!time) throw new Error('MongoDB did not return the point snapshot time')
    onSnapshotStarted?.(time)
    const revision = sourceRevision(time, '~')
    for await (const doc of cursor) {
      if (!active()) break
      await enqueueIntegrity(doc, revision, enqueue)
      count++
    }
    if (active()) {
      // Use the same database snapshot for point data and missing IDs.
      const missing = knownPoints.aggregate([
        { $lookup: { from: RealtimeDataCollectionName, localField: '_id', foreignField: '_id', as: 'point' } },
        { $match: { 'point.0': { $exists: false } } },
        { $project: { _id: 1 } },
      ], { session })
      for await (const { _id } of missing) {
        if (!active()) break
        await enqueue({ operationType: 'delete', documentKey: { _id }, revision, historical: true })
      }
    }
    return time
  } finally {
    await session.endSession()
    Log.log('End enqueueing of db sync data - ' + count + ' entries')
  }
}

async function enqueueChange(change, enqueue) {
  const { operationType, fullDocument, documentKey, updateDescription } = change
  const revision = sourceRevision(change.clusterTime, change._id._data)
  if (operationType === 'delete') {
    await enqueue({ operationType, documentKey, revision })
    return
  }

  const fields = updateDescription?.updatedFields
  if (operationType !== 'update' || Object.hasOwn(fields ?? {}, 'tag'))
    await enqueueIntegrity(fullDocument, revision + '/0', enqueue)
  if (!fields?.sourceDataUpdate || !fullDocument?.tag) return

  const sourceDataUpdate = { ...fields.sourceDataUpdate }
  if (sourceDataUpdate.valueJsonAtSource?.length > AppDefs.MAX_LENGTH_JSON / 2)
    delete sourceDataUpdate.valueJsonAtSource
  delete sourceDataUpdate.valueBsonAtSource
  await enqueue({
    revision: revision + '/1',
    tag: fullDocument.tag,
    operationType,
    documentKey,
    updateDescription: { updatedFields: { ...fields, sourceDataUpdate } },
  })
}

async function storeBackfill(backfill, archive, active, pause) {
  let pending = []
  while (active()) {
    while (pending.length < 1000 && !archive.isEmpty()) pending.push(archive.dequeue())
    if (pending.length) {
      try {
        await backfill.insertMany(pending, { ordered: true })
        pending = []
      } catch (error) {
        // Preserve the unacknowledged tail if MongoDB reports a per-document error.
        const failure = error.result?.getWriteErrorAt(0)
        if (failure && !error.result.getWriteConcernError())
          pending = pending.slice(failure.index + 1)
        Log.log('Backfill write error: ' + error)
      }
    }
    await pause(1013)
  }
}

async function sendChanges(socket, authKey, live, background, archive, knownPoints, active, pause) {
  let sequence = 0
  let liveBurst = 0
  while (active()) {
    if (archive.size() >= queueLimit) {
      await pause(25)
      continue
    }
    const packet = []
    let size = 0
    while (size <= AppDefs.PACKET_SIZE_THRESHOLD && (!live.isEmpty() || !background.isEmpty())) {
      const changes = !live.isEmpty() && (liveBurst < 8 || background.isEmpty()) ? live : background
      const { historical, ...change } = changes.peek()
      const obj = { ...change, cnt: cntChg + 1 }
      const length = Buffer.byteLength(JSON.stringify(obj))
      if (length > AppDefs.MAX_LENGTH_JSON) {
        changes.dequeue()
        Log.log('Discarded change too large: ' + length)
        continue
      }
      if (packet.length && size + length + packet.length + 2 > AppDefs.MAX_LENGTH_JSON) break
      changes.dequeue()
      liveBurst = changes === live ? liveBurst + 1 : 0
      cntChg++
      size += length
      packet.push(obj)
      if (
        !historical &&
        change.operationType === 'update' &&
        !change.updateDescription.updatedFields.sourceDataUpdate.isHistorical
      ) {
        archive.enqueue({
          timestamp: new Date(),
          metadata: { tag: obj.tag },
          data: obj,
        })
      }
    }

    if (!packet.length) {
      sequence = 0
      await pause(AppDefs.INTERVAL_AFTER_EMPTY_QUEUE)
      continue
    }

    const published = packet.filter((entry) => entry.operationType === 'integrity')
    if (published.length) {
      await knownPoints.bulkWrite(published.map(({ documentKey }) => ({
        updateOne: { filter: { _id: documentKey._id }, update: { $setOnInsert: { known: true } }, upsert: true },
      })), { ordered: false, writeConcern: { w: 'majority' } })
    }
    const buffer = signPacket(zlib.deflateSync(JSON.stringify(packet)), authKey)
    if (active()) {
      await new Promise((resolve) => {
        socket.send(buffer, AppDefs.UDP_PORT, AppDefs.IP_DESTINATION, (error) => {
          if (error) Log.log('UDP error: ' + error)
          resolve()
        })
      })
    }

    sequence++
    if (sequence >= AppDefs.MAX_SEQUENCE_OF_UDPMSGS || buffer.length > AppDefs.PACKET_SIZE_BREAK_SEQ) {
      sequence = 0
      await pause(AppDefs.INTERVAL_AFTER_UDPMSGS_SEQ + 100 * Math.floor(buffer.length / 1500))
    } else if (AppDefs.PACKETS_INTERVAL > 0) {
      await pause(AppDefs.PACKETS_INTERVAL)
    }
  }
}

async function replayDocuments(collection, timeField, changes, active, pause, enqueue) {
  const end = new Date()
  const begin = new Date(end)
  begin.setDate(begin.getDate() - AppDefs.BACKFILL_REPLAY_INTERVAL)
  const cursor = collection.find(
    { [timeField]: { $gt: begin, $lte: end } },
    { sort: { [timeField]: 1 }, batchSize: AppDefs.BACKFILL_DOCS_PER_SEC }
  )
  let count = 0
  let budget = changes.isEmpty() ? AppDefs.BACKFILL_DOCS_PER_SEC * 2 : AppDefs.BACKFILL_DOCS_PER_SEC
  for await (const doc of cursor) {
    if (!active()) break
    await enqueue(doc)
    if (++count === budget) {
      await pause(1000)
      count = 0
      budget = changes.isEmpty() ? AppDefs.BACKFILL_DOCS_PER_SEC * 2 : AppDefs.BACKFILL_DOCS_PER_SEC
    }
  }
}

async function replayBackfill(backfill, knownPoints, background, enqueue, active, pause) {
  await replayDocuments(backfill, 'timestamp', background, active, pause, async (doc) => {
    const data = doc.data
    const sourceDataUpdate = data?.updateDescription?.updatedFields?.sourceDataUpdate
    if (sourceDataUpdate && data.tag) {
      sourceDataUpdate.isHistorical = true
      await enqueue({ ...data, historical: true })
    } else if (data?.operationType === 'delete' && Number.isFinite(data.documentKey?._id)) {
      // Import deletion records archived by earlier forwarder versions.
      await knownPoints.updateOne(
        { _id: data.documentKey._id },
        { $setOnInsert: { known: true } },
        { upsert: true, writeConcern: { w: 'majority' } }
      )
    }
  })
}
