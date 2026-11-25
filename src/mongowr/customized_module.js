/*
 * One way replication mechanism (mongofw/mongowr)
 * Receive UDP point snapshots, updates, deletions, and security events.
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

'use strict'
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')
const { BSON, Double, MongoBulkWriteError, ObjectId } = require('mongodb')
const { verifyPacket, isFresh } = require('./udp-auth')
const { setTimeout: sleep } = require('node:timers/promises')
const dgram = require('node:dgram')
const { once } = require('node:events')
const zlib = require('node:zlib')

const durable = { writeConcern: { w: 'majority' }, timeoutMS: 5000 }
const msgQueue = new Map()
let journalBatch = []
let pending = []
let cntChg = null
let cntLost = 0

module.exports.CustomProcessor = async function (
  clientMongo, jsConfig, Redundancy, MongoStatus, signal
) {
  const db = clientMongo.db(jsConfig.mongoDatabaseName)
  const realtime = db.collection('realtimeData')
  const secEvents = db.collection('secEvents')
  const receipts = db.collection('replicationUdpReceipts')
  const revisions = db.collection('replicationPointRevisions')
  await realtime.createIndex(
    { replicatedSourceId: 1 },
    { name: 'replicatedSourceId', unique: true, sparse: true }
  )
  await receipts.createIndex(
    { expiresAt: 1 },
    { name: 'expiresAt', expireAfterSeconds: 0 }
  )
  await receipts.createIndex({ pendingBatch: 1, position: 1 }, { sparse: true })
  if (signal.aborted) return

  const server = dgram.createSocket('udp4')
  const active = () =>
    !signal.aborted && MongoStatus.HintMongoIsConnected && Redundancy.ProcessStateIsActive()

  server.on('message', (msgRaw) => {
    if (!active()) return
    const packet = verifyPacket(msgRaw, jsConfig.UdpAuthKey)
    if (!packet || msgQueue.has(packet.nonce)) return
    if (msgQueue.size >= AppDefs.MAX_QUEUE) {
      msgQueue.clear()
      Log.log('Queue too large! Emptied!')
    }
    msgQueue.set(packet.nonce, packet)
  })

  const controller = new AbortController()
  const stop = () => controller.abort()
  const onError = (error) => {
    Log.log('UDP socket error: ' + error)
    stop()
  }
  server.on('error', onError)
  signal.addEventListener('abort', stop, { once: true })
  try {
    const listening = once(server, 'listening', { signal: controller.signal })
    server.bind(AppDefs.UDP_PORT, AppDefs.IP_BIND)
    await listening
    const { address, port } = server.address()
    Log.log('Server listening ' + address + ':' + port)

    let wasActive = false
    while (!controller.signal.aborted) {
      const isActive = active()
      if (isActive) {
        try {
          if (!journalBatch.length) await readBatch(receipts, active)
          if (pending.length && active()) await writeBatch(realtime, secEvents, revisions, active)
          if (!pending.length && journalBatch.length && active()) {
            await receipts.bulkWrite(journalBatch.map(({ _id, retainUntil }) => ({
              updateOne: {
                filter: { _id },
                update: {
                  $set: { expiresAt: retainUntil },
                  $unset: { payload: '', pendingBatch: '', position: '', retainUntil: '' },
                },
              },
            })), durable)
            journalBatch = []
          }
        } catch (error) {
          Log.log('UDP journal error: ' + error)
        }
      } else if (wasActive && MongoStatus.HintMongoIsConnected) {
        // A standby must not drain writes captured before it lost ownership.
        msgQueue.clear()
        journalBatch = []
        pending = []
        cntChg = null
      }
      wasActive = isActive
      await sleep(AppDefs.INTERVAL_AFTER_WRITE, undefined, { signal: controller.signal })
    }
  } finally {
    signal.removeEventListener('abort', stop)
    await new Promise((resolve) => server.close(resolve))
  }
}

async function readBatch(receipts, active) {
  const loadPending = () => receipts.find(
    { pendingBatch: { $exists: true } },
    { readConcern: { level: 'majority' }, timeoutMS: 5000 }
  ).sort({ pendingBatch: 1, position: 1 }).limit(AppDefs.MAX_MSG_SEQ).toArray()

  let entries = await loadPending()
  if (!entries.length && msgQueue.size && active()) {
    const packets = []
    for (const packet of msgQueue.values()) {
      packets.push(packet)
      if (packets.length === AppDefs.MAX_MSG_SEQ) break
    }
    const batch = new ObjectId()
    // Payload and nonce commit together. Pending receipts have no TTL and are
    // recoverable even after the original datagram's authentication window.
    const operations = packets.flatMap(({ nonce, sentAt, expiresAt, payload }, position) =>
      isFresh(sentAt) ? [{
        updateOne: {
          filter: { _id: nonce },
          update: { $setOnInsert: { pendingBatch: batch, position, retainUntil: expiresAt, payload } },
          upsert: true,
        },
      }] : []
    )
    if (operations.length && active())
      await receipts.bulkWrite(operations, { ...durable, ordered: false })
    for (const { nonce } of packets) msgQueue.delete(nonce)
    entries = await loadPending()
  }
  if (!active()) return
  pending = decodePackets(entries)
  journalBatch = entries
}

function decodePackets(packets) {
  const groups = []
  for (const { payload } of packets) {
    try {
      const buffer = zlib.inflateSync(payload.buffer, { maxOutputLength: 300000 })
      const entries = JSON.parse(buffer.toString('utf8'))
      if (!Array.isArray(entries)) continue
      for (const entry of entries) {
        if (!Number.isSafeInteger(entry?.cnt)) continue
        if (cntChg !== null && entry.cnt > cntChg + 1) {
          const lost = entry.cnt - cntChg - 1
          cntLost += lost
          Log.log('Sequence gap: ' + lost + ' (total: ' + cntLost + ')')
        }
        cntChg = entry.cnt
        const securityEvent = entry.operationType === 'secEvent'
        const operation = securityEvent ? toSecEventOperation(entry) : toWriteOperation(entry)
        if (!operation) continue
        const sourceId = entry.documentKey?._id
        const historical = entry.operationType === 'update' &&
          entry.updateDescription?.updatedFields?.sourceDataUpdate?.isHistorical === true
        if (!securityEvent && !historical) {
          if (Number.isFinite(sourceId) && typeof entry.revision === 'string' &&
              /^\d{20}\/[0-9A-Fa-f~]*(?:\/[01])?$/.test(entry.revision)) {
            groups.push({ sourceId, revision: entry.revision, operations: [operation] })
          }
          continue
        }
        let group = groups.at(-1)
        if (!group || group.revision || group.securityEvent !== securityEvent) {
          group = { securityEvent, operations: [] }
          groups.push(group)
        }
        group.operations.push(operation)
      }
    } catch (error) {
      Log.log('Error decoding replication packet: ' + error)
    }
  }
  return groups
}

function toSecEventOperation(entry) {
  if (typeof entry.eventBson !== 'string' || !entry.eventBson) return
  let event
  try {
    event = BSON.deserialize(Buffer.from(entry.eventBson, 'base64'), { promoteValues: false })
  } catch (error) {
    Log.log('Error decoding security event: ' + error)
    return
  }
  const { _id, ...fields } = event
  if (!(_id instanceof BSON.ObjectId) || !(fields.timeTag instanceof Date) ||
      !Number.isFinite(fields.timeTag.getTime()) || typeof fields.kind !== 'string') return
  return {
    updateOne: {
      filter: { _id },
      update: { $setOnInsert: fields },
      upsert: true,
    },
  }
}

function toWriteOperation(entry) {
  const { operationType, tag, documentKey, updateDescription } = entry
  const fields = updateDescription?.updatedFields
  const sourceId = documentKey?._id ?? (operationType === 'integrity' ? fields?._id : undefined)

  if (operationType === 'delete') {
    if (!Number.isFinite(sourceId)) return
    return { deleteOne: { filter: { replicatedSourceId: sourceId } } }
  }
  if (typeof tag !== 'string' || !tag || !fields ||
      typeof fields !== 'object' || Array.isArray(fields)) return

  // An established mapping survives source renames. A tag match can adopt an
  // existing point, including points with a different local MongoDB _id.
  const tagFilter = { tag, replicatedSourceId: { $exists: false } }
  const filter = Number.isFinite(sourceId)
    ? { $or: [{ replicatedSourceId: sourceId }, tagFilter] }
    : { tag }

  if (operationType === 'integrity' && Number.isFinite(sourceId)) {
    const { _id, ...point } = fields
    for (const key of ['timeTag', 'timeTagAtSource', 'timeTagAlarm', 'timeTagAlertState']) {
      if (point[key]) point[key] = new Date(point[key])
    }
    return {
      updateOne: {
        filter,
        update: {
          $set: { ...point, tag, replicatedSourceId: sourceId },
          $setOnInsert: { _id: new Double(sourceId) },
        },
        upsert: true,
      },
    }
  }

  if (operationType === 'update' && fields.sourceDataUpdate &&
      typeof fields.sourceDataUpdate === 'object' && !Array.isArray(fields.sourceDataUpdate)) {
    const sourceDataUpdate = { ...fields.sourceDataUpdate }
    for (const key of ['timeTag', 'timeTagAtSource']) {
      if (sourceDataUpdate[key]) sourceDataUpdate[key] = new Date(sourceDataUpdate[key])
    }
    const { _id, tag: updatedTag, replicatedSourceId, ...companionFields } = fields
    const update = sourceDataUpdate.isHistorical
      ? { sourceDataUpdate }
      : { ...companionFields, sourceDataUpdate }
    return { updateOne: { filter, update: { $set: update } } }
  }
}

async function applyPoint(realtime, revisions, group, active) {
  const { sourceId, revision } = group
  const operation = group.operations[0]
  const change = operation.updateOne ?? operation.deleteOne
  const identity = { _id: sourceId }
  const point = await realtime.findOne(change.filter, { projection: { replicationRevision: 1 }, timeoutMS: 5000 })
  if (!active()) return

  if (operation.updateOne && !change.upsert && !point) {
    // An update can arrive ahead of its first integrity packet. Keep the
    // newest deferred value until the writer can attach it to a point.
    await revisions.updateOne(identity, [{ $set: {
      deferred: { $cond: [
        { $gt: [revision, { $max: [
          { $ifNull: ['$revision', ''] }, { $ifNull: ['$deferred.revision', ''] },
        ] }] },
        { $literal: { revision, operation } },
        '$deferred',
      ] },
    } }], { ...durable, upsert: true })
    return
  }

  const state = await revisions.findOneAndUpdate(
    identity, { $max: { revision } },
    { ...durable, upsert: true, returnDocument: 'after' }
  )
  if (!active()) return
  const deferred = state.deferred
  if (state.revision !== revision) {
    // Recovery can revisit the integrity record after the deferred update
    // reserved its revision but before it reached realtimeData.
    if (change.upsert && point && deferred?.revision === state.revision) {
      await applyPoint(realtime, revisions, {
        sourceId, revision: deferred.revision, operations: [deferred.operation],
      }, active)
    }
    return
  }

  const versionFilter = {
    $or: [
      { replicationRevision: { $exists: false } },
      { replicationRevision: { $lt: revision } },
    ],
  }
  if (operation.deleteOne) {
    await realtime.deleteOne({ $and: [change.filter, versionFilter] }, durable)
  } else if (!point?.replicationRevision || point.replicationRevision < revision) {
    const filter = point ? { _id: point._id } : change.filter
    const update = {
      ...change.update,
      $set: { ...change.update.$set, replicationRevision: revision },
    }
    await realtime.updateOne({ $and: [filter, versionFilter] }, update,
      { ...durable, upsert: change.upsert && !point })
  }

  if (change.upsert && state.deferred?.revision > revision && active()) {
    await applyPoint(realtime, revisions, {
      sourceId, revision: deferred.revision, operations: [deferred.operation],
    }, active)
  }
  if (active() && deferred && deferred.revision <= revision)
    await revisions.updateOne({ ...identity, 'deferred.revision': deferred.revision },
      { $unset: { deferred: '' } }, durable)
}

async function writeBatch(realtime, secEvents, revisions, active) {
  let completed = 0
  try {
    for (const group of pending) {
      if (!active()) break
      const collection = group.securityEvent ? secEvents : realtime
      try {
        if (group.revision) await applyPoint(realtime, revisions, group, active)
        else await collection.bulkWrite(group.operations, { ...durable, ordered: true })
        if (!active()) break
        completed++
      } catch (error) {
        Log.log('Replication write error in ' + collection.collectionName + ': ' + error)
        const permanentCodes = [2, 9, 14, 28, 40, 52, 66, 121, 11000]
        if (!(error instanceof MongoBulkWriteError)) {
          if (group.revision && permanentCodes.includes(error.code)) completed++
          break
        }
        if (error.result.getWriteConcernError()) break

        const failure = error.result.getWriteErrorAt(0)
        if (!failure) break // An ambiguous write retains the current group.
        // MongoDB stopped at this index. Drop the acknowledged prefix and skip
        // errors that replaying the same operation cannot resolve.
        const permanentError = permanentCodes.includes(failure.code)
        group.operations = group.operations.slice(failure.index + (permanentError ? 1 : 0))
        if (!group.operations.length) completed++
        break
      }
    }
  } finally {
    pending = pending.slice(completed)
  }
}
