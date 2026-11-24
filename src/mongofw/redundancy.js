/*
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
const { setTimeout: sleep } = require('node:timers/promises')
const { Double, ObjectId } = require('mongodb')
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')

let ProcessActive = false
const owner = new ObjectId()

async function Start(interval, db, config, signal) {
  const collection = db.collection(config.ProcessInstancesCollectionName)
  const identity = {
    processName: AppDefs.NAME,
    processInstanceNumber: new Double(config.Instance),
  }
  const writeConcern = { w: 'majority' }
  ProcessActive = false
  await collection.createIndex({ processName: 1, processInstanceNumber: 1 },
    { name: 'processInstancesIndex', unique: true })
  try {
    while (!signal.aborted) {
      try {
        const instance = await collection.findOne(identity, { timeoutMS: interval })
        if (!instance) {
          const result = await collection.updateOne(identity, {
            $setOnInsert: {
              enabled: true, logLevel: new Double(1), nodeNames: [],
              activeNodeName: config.nodeName, replicationOwner: owner,
              activeNodeKeepAliveTimeTag: new Date(),
            },
          }, { upsert: true, writeConcern, timeoutMS: interval })
          ProcessActive = result.upsertedCount === 1 && !signal.aborted
        } else {
          if (instance.enabled === false ||
              (instance.nodeNames?.length && !instance.nodeNames.includes(config.nodeName))) {
            Log.log('Redundancy - Instance disabled or node not allowed')
            process.exit()
          }
          const heartbeat = instance.activeNodeKeepAliveTimeTag
          const ownsLease = instance.activeNodeName === config.nodeName &&
            (!instance.replicationOwner || owner.equals(instance.replicationOwner))
          const cutoff = 6 * interval
          const eligible = ownsLease || !(heartbeat instanceof Date) ||
            Date.now() - heartbeat.getTime() >= cutoff
          let acquired = false
          if (eligible && !signal.aborted) {
            const stale = !ownsLease && heartbeat instanceof Date
              ? { $expr: { $lte: ['$activeNodeKeepAliveTimeTag', { $subtract: ['$$NOW', cutoff] }] } }
              : {}
            const result = await collection.updateOne({
              _id: instance._id,
              activeNodeName: instance.activeNodeName ?? null,
              replicationOwner: instance.replicationOwner ?? null,
              activeNodeKeepAliveTimeTag: heartbeat ?? null,
              ...stale,
            }, {
              $currentDate: { activeNodeKeepAliveTimeTag: true },
              $set: { activeNodeName: config.nodeName, replicationOwner: owner, softwareVersion: AppDefs.VERSION, stats: {} },
            }, { writeConcern, timeoutMS: interval })
            acquired = result.matchedCount === 1
          }
          if (!signal.aborted && acquired !== ProcessActive)
            Log.log(acquired ? 'Redundancy - Node activated!' : 'Redundancy - Node deactivated!')
          ProcessActive = acquired && !signal.aborted
        }
      } catch (error) {
        ProcessActive = false
        Log.log('Redundancy - Error: ' + error)
      }
      await sleep(interval, undefined, { signal })
    }
  } finally {
    ProcessActive = false
  }
}

module.exports = { Start, ProcessStateIsActive: () => ProcessActive }
