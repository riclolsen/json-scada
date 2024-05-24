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

let maxSz = 0
let cnt = 0

const UserActionsCollectionName = 'userActions'
const RealtimeDataCollectionName = 'realtimeData'
const CommandsQueueCollectionName = 'commandsQueue'
const SoeDataCollectionName = 'soeData'
const ProcessInstancesCollectionName = 'processInstances'
const ProtocolDriverInstancesCollectionName = 'protocolDriverInstances'
const ProtocolConnectionsCollectionName = 'protocolConnections'

let CyclicIntervalHandle = null

// this will be called by the main module when mongo is connected (or reconnected)
module.exports.CustomProcessor = function (
  clientMongo,
  jsConfig,
  Redundancy,
  MongoStatus
) {
  if (clientMongo === null) return
  const db = clientMongo.db(jsConfig.mongoDatabaseName)

  // -------------------------------------------------------------------------------------------
  // EXAMPLE OF CYCLIC PROCESSING AT INTERVALS
  // BEGIN EXAMPLE

  let CyclicProcess = async function () {
    // do cyclic processing at each CyclicInterval ms

    if (!Redundancy.ProcessStateIsActive() || !MongoStatus.HintMongoIsConnected)
      return // do nothing if process is inactive

    try {
      let res = await db
        .collection(RealtimeDataCollectionName)
        .findOne({ _id: -2 }) // id of point tag with number of digital updates

      Log.log(
        'Custom Process - Checking number of digital updates: ' +
          res.valueString
      )
    } catch (err) {
      Log.log(err)
    }

    return
  }
  const CyclicInterval = 5000 // interval time in ms
  clearInterval(CyclicIntervalHandle) // clear older instances if any
  CyclicIntervalHandle = setInterval(CyclicProcess, CyclicInterval) // start a cyclic processing

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
      zlib.deflate(opData, (err, message) => {
        Log.log(opData.length + ' ' + message.length)
        if (message.length > maxSz) maxSz = message.length
        if (message.length > 60000) {
          Log.log('Message too large: ' + message.length)
          return
        }
        const buff = Buffer.from(message)
        udpSocket.send(
          buff,
          0,
          buff.length,
          udpPort,
          udpHostDst,
          (err, bytes) => {
            if (err) {
              Log.log('UDP error:' + err)
            } else {
              // Log.log('Data sent via UDP' + opData);
              Log.log('Size: ' + buff.length)
              // Log.log('Max: ' + maxSz);
            }
          }
        )
      })
    })
  } catch (e) {
    Log.log('Custom Process - Error: ' + e)
  }
}
