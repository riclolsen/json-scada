'use strict'

/*
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

const { setInterval } = require('timers')
const { Double } = require('mongodb')
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')

let ProcessActive = false // redundancy state
let redundancyIntervalHandle = null // timer handle

// start processing redundancy
function Start(interval, clientMongo, db, configObj, MongoStatus) {
  // check and update redundancy control
  ProcessRedundancy(clientMongo, db, configObj)
  clearInterval(redundancyIntervalHandle)
  redundancyIntervalHandle = setInterval(function () {
    if (!MongoStatus.HintMongoIsConnected){
      ProcessActive = false
      return
    }     
    ProcessRedundancy(clientMongo, db, configObj)
  }, interval)
}

// process JSON-SCADA redundancy state for this driver module
async function ProcessRedundancy(clientMongo, db, configObj) {
  if (!clientMongo || !db) return

  Log.levelCurrent = configObj.LogLevel

  const countKeepAliveUpdatesLimit = 4

  // poor man's local static variables
  if (typeof ProcessRedundancy.countKeepAliveNotUpdated === 'undefined') {
    ProcessRedundancy.lastActiveNodeKeepAliveTimeTag = null
    ProcessRedundancy.countKeepAliveNotUpdated = 0
  }

  Log.log('Redundancy - Process ' + (ProcessActive ? 'Active' : 'Inactive'))

  try {
    // look for process instance entry, if not found create a new entry
    const result = await db
      .collection(configObj.ProtocolDriverInstancesCollectionName)
      .findOne({
        protocolDriver: AppDefs.NAME,
        protocolDriverInstanceNumber: configObj.Instance,
      })

    if (result) {
      if (result.length === 0) {
        // not found, then create
        ProcessActive = true
        Log.log('Redundancy - Instance config not found, creating one...')
        db.collection(
          configObj.ProtocolDriverInstancesCollectionName
        ).insertOne({
          protocolDriver: AppDefs.NAME,
          protocolDriverInstanceNumber: new Double(configObj.Instance),
          enabled: true,
          logLevel: new Double(configObj.LogLevel),
          nodeNames: [],
          activeNodeName: configObj.nodeName,
          activeNodeKeepAliveTimeTag: new Date(),
        })
      } else {
        // check for disabled or node not allowed
        const instance = result

        let instKeepAliveTimeTag = null

        if ('activeNodeKeepAliveTimeTag' in instance)
          instKeepAliveTimeTag =
            instance.activeNodeKeepAliveTimeTag.toISOString()

        if (instance?.enabled === false) {
          Log.log('Redundancy - Instance disabled, exiting...')
          process.exit()
        }
        if (instance?.nodeNames !== null && instance.nodeNames.length > 0) {
          if (!instance.nodeNames.includes(configObj.nodeName)) {
            Log.log('Redundancy - Node name not allowed, exiting...')
            process.exit()
          }
        }
        if (instance?.activeNodeName === configObj.nodeName) {
          if (!ProcessActive) Log.log('Redundancy - Node activated!')
          ProcessRedundancy.countKeepAliveNotUpdated = 0
          ProcessActive = true
        } else {
          // other node active
          if (ProcessActive) {
            Log.log('Redundancy - Node deactivated!')
            ProcessRedundancy.countKeepAliveNotUpdated = 0
          }
          ProcessActive = false
          if (
            ProcessRedundancy.lastActiveNodeKeepAliveTimeTag ===
            instKeepAliveTimeTag
          ) {
            ProcessRedundancy.countKeepAliveNotUpdated++
            Log.log(
              'Redundancy - Keep-alive from active node not updated. ' +
                ProcessRedundancy.countKeepAliveNotUpdated
            )
          } else {
            ProcessRedundancy.countKeepAliveNotUpdated = 0
            Log.log(
              'Redundancy - Keep-alive updated by active node. Staying inactive.'
            )
          }
          ProcessRedundancy.lastActiveNodeKeepAliveTimeTag =
            instKeepAliveTimeTag
          if (
            ProcessRedundancy.countKeepAliveNotUpdated >
            countKeepAliveUpdatesLimit
          ) {
            // cnt exceeded, be active
            ProcessRedundancy.countKeepAliveNotUpdated = 0
            Log.log('Redundancy - Node activated!')
            ProcessActive = true
          }
        }

        if (ProcessActive) {
          // process active, then update keep alive
          db.collection(
            configObj.ProtocolDriverInstancesCollectionName
          ).updateOne(
            {
              protocolDriver: AppDefs.NAME,
              protocolDriverInstanceNumber: new Double(configObj.Instance),
            },
            {
              $set: {
                activeNodeName: configObj.nodeName,
                activeNodeKeepAliveTimeTag: new Date(),
                softwareVersion: AppDefs.VERSION,
                stats: {},
              },
            }
          )
        }
      }
    }
  } catch (err) {
    Log.log('Redundancy - Error: ' + err)
  }
}

function ProcessStateIsActive() {
  return ProcessActive
}

module.exports = {
  ProcessRedundancy: ProcessRedundancy,
  Start: Start,
  ProcessStateIsActive: ProcessStateIsActive,
}
