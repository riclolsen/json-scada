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
const { Double } = require('mongodb')
const { setInterval } = require('timers')
const dgram = require('node:dgram')
const Queue = require('queue-fifo')
const zlib = require('zlib')

// UDP broadcast options
const udpPort = 12345
const udpBind = '0.0.0.0'

const UserActionsCollectionName = 'userActions'
const RealtimeDataCollectionName = 'realtimeData'
const CommandsQueueCollectionName = 'commandsQueue'
const SoeDataCollectionName = 'soeData'
const ProcessInstancesCollectionName = 'processInstances'
const ProtocolDriverInstancesCollectionName = 'protocolDriverInstances'
const ProtocolConnectionsCollectionName = 'protocolConnections'

let msgQueue = new Queue() // queue of messages
let collection = null
let pktCnt = 0

// this will be called by the main module when mongo is connected (or reconnected)
module.exports.CustomProcessor = async function (
  clientMongo,
  jsConfig,
  Redundancy,
  MongoStatus
) {
  if (clientMongo === null) return
  const db = clientMongo.db(jsConfig.mongoDatabaseName)
  collection = db.collection(RealtimeDataCollectionName)

  const server = dgram.createSocket('udp4')

  server.on('error', (err) => {
    Log.log(`Server error:\n${err.stack}`)
    server.close()
  })

  server.on('message', (msgRaw, rinfo) => {
    if (!Redundancy.ProcessStateIsActive() || !MongoStatus.HintMongoIsConnected)
      return // do nothing if process is inactive

    msgQueue.enqueue(msgRaw)
    pktCnt++
  })

  server.on('listening', () => {
    const address = server.address()
    Log.log(`server listening ${address.address}:${address.port}`)
  })

  server.bind(udpPort, udpBind)
}

let cnt = -1
let cntLost = 0

setTimeout(procQueue, 1000)
async function procQueue() {
  if (msgQueue.size() > 5000) {
    msgQueue.clear()
    Log.log('Queue too large! Emptied!')
  }

  let cntPr = 0
  const updateOps = []
  while (!msgQueue.isEmpty()) {
    cntPr++
    if (cntPr > 50) {
      break
    }
    try {
      const msgRaw = msgQueue.peek()
      msgQueue.dequeue()

      const buffer = zlib.inflateSync(msgRaw)
      const msg = buffer.toString('utf8')
      const arrObj = JSON.parse(msg)
  
      if (arrObj.length)
      for (let i = 0; i < arrObj.length; i++) {
        const dataObj = arrObj[i]
        Log.log('Queue Size: ' + msgQueue.size())

        if (!dataObj?.cnt) {
          Log.log('Unexpected format')
        }
        if (dataObj.cnt - cnt > 1 && cnt != -1) {
          Log.log('Message lost # ' + (dataObj.cnt - 1))
          cntLost += dataObj.cnt - cnt
        }
        cnt = dataObj.cnt
        Log.log('Total lost: ' + cntLost)
        Log.log('                 Chg count: ' + dataObj.cnt)
        Log.log('                 Pkt count: ' + pktCnt)

        // will process only update data from drivers
        if (!dataObj?.updateDescription?.updatedFields?.sourceDataUpdate) return

        if (dataObj?.updateDescription?.updatedFields?.sourceDataUpdate.timeTag)
          dataObj.updateDescription.updatedFields.sourceDataUpdate.timeTag =
            new Date(
              dataObj.updateDescription.updatedFields.sourceDataUpdate.timeTag
            )
        if (
          dataObj?.updateDescription?.updatedFields?.sourceDataUpdate
            .timeTagAtSource
        )
          dataObj.updateDescription.updatedFields.sourceDataUpdate.timeTagAtSource =
            new Date(
              dataObj.updateDescription.updatedFields.sourceDataUpdate.timeTagAtSource
            )

        updateOps.push({
          updateOne: {
            filter: { ...dataObj.documentKey },
            update: { $set: { ...dataObj.updateDescription.updatedFields } },
          },
        })
      }
    } catch (e) {
      Log.log('Error: ' + e)
    }
  }

  if (updateOps.length > 0) {
    const result = await collection.bulkWrite(updateOps)
    Log.log(JSON.stringify(result))
  }
  setTimeout(procQueue, 150)
}
