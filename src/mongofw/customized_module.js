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
const dgram = require('dgram')
const Queue = require('queue-fifo')
const zlib = require('zlib')

// UDP broadcast options
const udpPort = 12345
const udpHostDst = '192.168.0.255'
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

const chgQueue = new Queue() // queue of changes

// this will be called by the main module when mongo is connected (or reconnected)
module.exports.CustomProcessor = function (
  clientMongo,
  jsConfig,
  Redundancy,
  MongoStatus
) {
  if (clientMongo === null) return
  const db = clientMongo.db(jsConfig.mongoDatabaseName)

  // EXAMPLE OF CYCLIC PROCESSING AT INTERVALS
  // END EXAMPLE
  // -------------------------------------------------------------------------------------------

  const changeStreamUserActions = db
    .collection(RealtimeDataCollectionName)
    .watch(
      { $match: { operationType: 'update' } },
      { fullDocument: 'updateLookup' }
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

      // will send only update data from drivers
      if (!change?.updateDescription?.updatedFields?.sourceDataUpdate) return

      chgQueue.enqueue(change)
    })
  } catch (e) {
    Log.log('Custom Process - Error: ' + e)
  }
}

let maxSz = 0
let cnt = 0

setTimeout(procQueue, 1000)
async function procQueue() {
  let cntSeq = 0
  while (!chgQueue.isEmpty()) {
    const change = chgQueue.peek()
    chgQueue.dequeue()

    if (
      change?.updateDescription?.updatedFields?.sourceDataUpdate
        ?.valueBsonAtSource
    )
      delete change.updateDescription.updatedFields.sourceDataUpdate
        .valueBsonAtSource
    if (change?.updateDescription?.truncatedArrays)
      delete change.updateDescription.truncatedArrays

    const fwObj = {
      cnt: cnt++,
      tag: change?.fullDocument?.tag,
      operationType: change.operationType,
      documentKey: change.documentKey,
      updateDescription: change.updateDescription,
    }
    const opData = JSON.stringify(fwObj)
    const message = zlib.deflateSync(opData)

    Log.log(opData.length + ' ' + message.length)
    if (message.length > maxSz) maxSz = message.length
    if (message.length > 60000) {
      Log.log('Message too large: ' + message.length)
      cnt--
      setTimeout(procQueue, 100)
      return
    }
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
    // Log.log('Data sent via UDP' + opData);
    Log.log('Size: ' + buff.length)
    Log.log('Message count ' + fwObj.cnt)
    Log.log('Max: ' + maxSz)
    Log.log('Seq count ' + cntSeq++)
    if (cntSeq > 75 || buff.length > 6000) {
      setTimeout(procQueue, 100+100*parseInt(buff.length/1500))
      return
    }
  }

  setTimeout(procQueue, 100)
}
