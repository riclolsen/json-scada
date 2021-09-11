'use strict'

/*
 * Customizable processor of mongodb changes via change streams.
 *
 * THIS FILE IS INTENDED TO BE CUSTOMIZED BY USERS TO DO SPECIAL PROCESSING
 *
 * {json:scada} - Copyright (c) 2020-2021 - Ricardo L. Olsen
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

const { Double } = require('mongodb')
const Queue = require('queue-fifo')
const { setInterval } = require('timers')

const UserActionsCollectionName = 'userActions'
const RealtimeDataCollectionName = 'realtimeData'
const CommandsQueueCollectionName = 'commandsQueue'
const SoeDataCollectionName = 'soeData'
const ProcessInstancesCollectionName = 'processInstances'
const ProtocolDriverInstancesCollectionName = 'protocolDriverInstances'
const ProtocolConnectionsCollectionName = 'protocolConnections'

let CyclicIntervalHandle = null

// this will be called by the main module when mongo is connected (or reconnected)
module.exports.CustomProcessor = function (clientMongo, jsConfig) {
  if (clientMongo === null) return
  const db = clientMongo.db(jsConfig.mongoDatabaseName)

  /*
  // -------------------------------------------------------------------------------------------
  // EXAMPLE OF CYCLIC PROCESSING AT INTERVALS
  // BEGIN EXAMPLE

  let CyclicProcess = async function () {
    // do cyclic processing at each CyclicInterval ms
  
    if (!jsConfig.processActive) return // do nothing if process is inactive

    let res = await db
      .collection(RealtimeDataCollectionName)
      .findOne({ _id: -2 }) // id of point tag with number of digital updates

    console.log(
      'Custom Process - Checking number of digital updates: ' + res.valueString
    )

    return
  }
  const CyclicInterval = 5000 // interval time in ms
  clearInterval(CyclicIntervalHandle) // clear older instances if any
  CyclicIntervalHandle = setInterval(CyclicProcess, CyclicInterval) // start a cyclic processing

  // EXAMPLE OF CYCLIC PROCESSING AT INTERVALS
  // END EXAMPLE
  // -------------------------------------------------------------------------------------------
  */

  /*
  // -------------------------------------------------------------------------------------------
  // EXAMPLE OF CHANGE STREAM PROCESSING (MONITORING OF CHANGES IN MONGODB COLLECTIONS)
  // BEGIN EXAMPLE

  const changeStreamUserActions = db
    .collection(UserActionsCollectionName)
    .watch(
      { $match: { operationType: 'insert' } }, // will listen only for insert operations
      {
        fullDocument: 'updateLookup'
      }
    )

  try {
    changeStreamUserActions.on('error', change => {
      if (clientMongo) clientMongo.close()
      clientMongo = null
      console.log('Custom Process - Error on changeStreamUserActions!')
    })
    changeStreamUserActions.on('close', change => {
      if (clientMongo) clientMongo.close()
      clientMongo = null
      console.log('Custom Process - Closed changeStreamUserActions!')
    })
    changeStreamUserActions.on('end', change => {
      if (clientMongo) clientMongo.close()
      clientMongo = null
      console.log('Custom Process - Ended changeStreamUserActions!')
    })

    // start listen to changes
    changeStreamUserActions.on('change', change => {
      // console.log(change.fullDocument)

      if (!jsConfig.processActive) return // do nothing if process is inactive
      if (change.operationType != 'insert') return // do nothing if operation is not insert

      // when operator acks all alarms
      if (change.fullDocument?.action === 'Ack All Alarms') {
        console.log('Custom Process - Generating Interrogation Requests')

        // insert a command for requesting general interrogation on a IEC 104 connection
        db.collection(CommandsQueueCollectionName).insertOne({
          protocolSourceConnectionNumber: new Double(61), // put here number of connection (101/104 client)
          protocolSourceCommonAddress: new Double(1), // put here common address to interrogate
          protocolSourceObjectAddress: new Double(0), // should be 0 for general interrogation
          protocolSourceASDU: new Double(100), // 100 ASDU TYPE for general interrogation C_CS_NA_1
          protocolSourceCommandDuration: new Double(20), // group of interrogation (20-36), 20=general interrogation
          protocolSourceCommandUseSBO: false,
          pointKey: new Double(0),
          tag: '',
          timeTag: new Date(),
          value: new Double(20), // will not be used for interrogation, just for documentation
          valueString: 'general interrogation', // just for documentation
          originatorUserName:
            'custom processor script, action "' +
            change.fullDocument?.action +
            '" of user: ' +
            change.fullDocument?.username, // just for documentation of user action
          originatorIpAddress: '',
          delivered: false
        })
      }
    })
  } catch (e) {
    console.log('Custom Process - Error: ' + e)
  }

  // -------------------------------------------------------------------------------------------
  // EXAMPLE OF CHANGE STREAM PROCESSING (MONITORING OF CHANGES IN MONGODB COLLECTIONS)
  // END EXAMPLE
  */
}
