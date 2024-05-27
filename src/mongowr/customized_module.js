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
const { Double } = require('mongodb')
const { setTimeout } = require('node:timers')
const dgram = require('node:dgram')
const Queue = require('queue-fifo')
// const zlib = require('fast-zlib')
const zlib = require('zlib')

// UDP broadcast options
const udpPort = AppDefs.UDP_PORT
const udpBind = AppDefs.IP_BIND

const UserAcinfltionsCollectionName = 'userActions'
const RealtimeDataCollectionName = 'realtimeData'
const CommandsQueueCollectionName = 'commandsQueue'
const SoeDataCollectionName = 'soeData'
const ProcessInstancesCollectionName = 'processInstances'
const ProtocolDriverInstancesCollectionName = 'protocolDriverInstances'
const ProtocolConnectionsCollectionName = 'protocolConnections'

let msgQueue = new Queue() // queue of messages
let collection = null
let pktCnt = 0

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

let cntChg = -1
let cntLost = 0

setTimeout(procQueue, 1000)
async function procQueue() {
  if (msgQueue.size() > AppDefs.MAX_QUEUE) {
    msgQueue.clear()
    Log.log('Queue too large! Emptied!')
  }

  let cntPr = 0
  const bulkOps = []
  while (!msgQueue.isEmpty()) {
    cntPr++
    if (cntPr > AppDefs.MAX_MSG_SEQ) {
      break
    }
    try {
      const msgRaw = msgQueue.peek()
      msgQueue.dequeue()

      //const inflate = new zlib.Inflate()
      //const buffer = inflate.process(msgRaw)
      const buffer = zlib.inflateSync(msgRaw)
      const msg = buffer.toString('utf8')
      const arrObj = JSON.parse(msg)

      if (arrObj.length)
        for (let i = 0; i < arrObj.length; i++) {
          const dataObj = arrObj[i]

          if (
            !('cnt' in dataObj) ||
            !('operationType' in dataObj) ||
            !('updateDescription' in dataObj)
          ) {
            Log.log('Unexpected format:' + JSON.stringify(dataObj))
            continue
          }
          if (dataObj.cnt - cntChg > 1 && cntChg != -1) {
            Log.log('Message lost # ' + (dataObj.cnt - 1))
            cntLost += dataObj.cnt - cntChg
          }
          cntChg = dataObj.cnt

          if (
            dataObj.operationType === 'integrity' &&
            dataObj?.updateDescription?.updatedFields &&
            dataObj?.updateDescription?.updatedFields?._id &&
            dataObj?.tag
          ) {
            // will process integrity of realtimeData (tag list)
            if (dataObj?.updateDescription?.updatedFields?.timeTag)
              dataObj.updateDescription.updatedFields.timeTag = new Date(
                dataObj.updateDescription.updatedFields.timeTag
              )

            if (dataObj?.updateDescription?.updatedFields?.timeTagAtSource)
              dataObj.updateDescription.updatedFields.timeTagAtSource =
                new Date(
                  dataObj.updateDescription.updatedFields.timeTagAtSource
                )

            if (dataObj?.updateDescription?.updatedFields?.timeTagAlarm)
              dataObj.updateDescription.updatedFields.timeTagAlarm = new Date(
                dataObj.updateDescription.updatedFields.timeTagAlarm
              )

            const _id = dataObj.updateDescription.updatedFields._id
            delete dataObj.updateDescription.updatedFields._id

            bulkOps.push({
              updateOne: {
                filter: { tag: dataObj.tag },
                // filter: { _id: _id },
                update: {
                  $set: dataObj.updateDescription.updatedFields,
                  $setOnInsert: {
                    _id: new Double(_id),
                    // ...dataObj.updateDescription.updatedFields,
                  },
                },
                upsert: true,
              },
            })
            // console.log(dataObj.updateDescription.updatedFields)
          } else if (
            dataObj.operationType === 'update' &&
            dataObj?.updateDescription?.updatedFields?.sourceDataUpdate
          ) {
            // will process update data from drivers
            if (
              dataObj?.updateDescription?.updatedFields?.sourceDataUpdate
                .timeTag
            )
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

            bulkOps.push({
              updateOne: {
                filter: { tag: dataObj.tag },
                update: {
                  $set: { ...dataObj.updateDescription.updatedFields },
                },
              },
            })
          }
        }
    } catch (e) {
      Log.log('Error: ' + e)
    }
    await sleep(1) // yield to allow packets to be collected and queued
  }

  if (bulkOps.length > 0) {
    try {
      const result = await collection.bulkWrite(bulkOps, {
        ordered: false, // may run ops in parallel, do all ops even when some operation fail
        writeConcern: { w: 1 }, // wait for just 1 node to complete the ops (put zero here to not wait, in this case it won't detect write errors)
      })
      // Log.log(JSON.stringify(bulkOps))
      Log.log(JSON.stringify(result))
      Log.log('Queue Size: ' + msgQueue.size())
      Log.log('Total lost: ' + cntLost)
      Log.log('                 Chg count: ' + cntChg)
      Log.log('                 UDP count: ' + pktCnt)
    } catch (e) {
      Log.log('Error: ' + e)
    }
  }

  setTimeout(procQueue, AppDefs.INTERVAL_AFTER_WRITE)
}
