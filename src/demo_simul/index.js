/*
 * This script can simulate events, values, respond to commands combined with the default demo database.
 * Convert raw values and update realtime values and statuses.
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
const LoadConfig = require('./load-config')
const { MongoClient, Double } = require('mongodb')

const pipeline = [
  {
    $project: { documentKey: false },
  },
  {
    $match: {
      $and: [
        {
          $or: [{ operationType: 'insert' }],
        },
      ],
    },
  },
]

const jsConfig = LoadConfig()
let HintMongoIsConnected = true
Log.log('Connecting to ' + jsConfig.mongoConnectionString)
;(async () => {
  let collection = null
  let cntUpd = 1
  let clientMongo = null
  HintMongoIsConnected = true

  setInterval(async function () {
    if (clientMongo !== null && HintMongoIsConnected) {
      const db = clientMongo.db(jsConfig.mongoDatabaseName)

      // fake IEC 104 driver running
      await db
        .collection(jsConfig.ProtocolDriverInstancesCollectionName)
        .updateOne(
          {
            protocolDriver: 'IEC60870-5-104',
          },
          [
            {
              $set: {
                activeNodeKeepAliveTimeTag: '$$NOW',
              },
            },
          ]
        )
        .catch((err) => {
          Log.log(err)
          if (err.message.indexOf('ECONNREFUSED') > -1) {
            clientMongo = null
          }
        })

      // validates supervised data
      await db
        .collection(jsConfig.RealtimeDataCollectionName)
        .updateMany(
          {
            origin: 'supervised',
            invalid: true,
          },
          [
            {
              $set: {
                invalid: false,
                timeTag: '$$NOW',
                'sourceDataUpdate.timeTag': '$$NOW',
              },
            },
          ]
        )
        .catch((err) => {
          Log.log(err)
          if (err.message.indexOf('ECONNREFUSED') > -1) {
            clientMongo = null
          }
        })

      // simulate protocol writes of digital values
      let tagDocs = await db
        .collection(jsConfig.RealtimeDataCollectionName)
        .find({
          type: 'digital',
          origin: 'supervised',
          $expr: { $eq: [{ $indexOfBytes: ['$tag', 'XSWI'] }, -1] },
          _id: { $mod: [Math.floor(Math.random() * 500) + 500, 0] },
        })
        .toArray()
      for (let i = 0; i < tagDocs.length; i++) {
        const document = tagDocs[i]
        console.log(`${document._id} ${document.tag} ${document.value}`)
        let res = await db
          .collection(jsConfig.RealtimeDataCollectionName)
          .updateOne(
            {
              _id: document._id,
            },
            {
              $set: {
                sourceDataUpdate: {
                  valueAtSource: document.value == 0 ? 1 : 0,
                  valueStringAtSource: '',
                  asduAtSource: 'M_SP_NA_1',
                  causeOfTransmissionAtSource: '3',
                  timeTag: new Date(),
                  timeTagAtSource: new Date(),
                  timeTagAtSourceOk: true,
                  invalid: false,
                  invalidAtSource: false,
                  substitutedAtSource: false,
                  overflowAtSource: false,
                  blockedAtSource: false,
                  notTopicalAtSource: false,
                  test: true,
                  originator: AppDefs.NAME,
                  CntUpd: cntUpd,
                },
              },
            }
          )
      }
      /*      
      let res = await db
        .collection(jsConfig.RealtimeDataCollectionName)
        .updateMany(
          {
            type: 'digital',
            origin: 'supervised',
            $expr: { $eq: [{ $indexOfBytes: ['$tag', 'XSWI'] }, -1] },
            _id: { $mod: [Math.floor(Math.random() * 500) + 500, 0] },
          },
          [
            {
              $set: {
                sourceDataUpdate: {
                  valueAtSource: { $toDouble: { $not: '$value' } },
                  valueStringAtSource: '',
                  asduAtSource: 'M_SP_NA_1',
                  causeOfTransmissionAtSource: '3',
                  invalid: false,
                  timeTag: '$$NOW',
                  timeTagAtSource: '$$NOW',
                  timeTagAtSourceOk: true,
                  substitutedAtSource: false,
                  overflowAtSource: false,
                  blockedAtSource: false,
                  notTopicalAtSource: false,
                  test: true,
                  originator: AppDefs.NAME,
                  CntUpd: cntUpd,
                },
              },
            },
          ]
        )
        .catch((err) => {
          Log.log(err)
          if (err.message.indexOf('ECONNREFUSED') > -1) {
            clientMongo = null
          }
        })

      Log.log(
        'Digital matchedCount: ' +
          res?.matchedCount +
          ' modifiedCount: ' +
          res?.modifiedCount
      )

      await db
        .collection(jsConfig.RealtimeDataCollectionName)
        .find({
          type: 'digital',
          origin: 'supervised',
          'sourceDataUpdate.CntUpd': cntUpd,
        })
        .toArray(async function (err, resarr) {
          resarr.forEach(async (element) => {
            element.sourceDataUpdate.CntUpd = 0
            let res2 = await db
              .collection(jsConfig.RealtimeDataCollectionName)
              .updateOne(
                {
                  _id: element._id,
                },
                {
                  $set: {
                    sourceDataUpdate: element.sourceDataUpdate,
                  },
                }
              )
              .catch((err) => {
                Log.log(err)
                if (err.message.indexOf('ECONNREFUSED') > -1) {
                  clientMongo = null
                }
              })
            Log.log(
              'Digital matchedCount: ' +
                res2?.matchedCount +
                ' modifiedCount: ' +
                res2?.modifiedCount
            )
          })
        })
        .catch((err) => {
          Log.log(err)
          if (err.message.indexOf('ECONNREFUSED') > -1) {
            clientMongo = null
          }
        })
      */
      cntUpd++
    }
  }, 11777)

  setInterval(async function () {
    if (clientMongo !== null && HintMongoIsConnected) {
      const db = clientMongo.db(jsConfig.mongoDatabaseName)

      // simulate protocol writes of analog values
      let tagDocs = await db
        .collection(jsConfig.RealtimeDataCollectionName)
        .find({
          type: 'analog',
          origin: 'supervised',
          _id: {
            $mod: [
              1 +
                Math.floor(Math.random() * 50) +
                Math.floor(Math.random() * 10),
              0,
            ],
          },
        })
        .toArray()
      for (let i = 0; i < tagDocs.length; i++) {
        const document = tagDocs[i]
        console.log(`${document._id} ${document.tag} ${document.value}`)
        let res = await db
          .collection(jsConfig.RealtimeDataCollectionName)
          .updateOne(
            {
              _id: document._id,
            },
            {
              $set: {
                sourceDataUpdate: {
                  valueAtSource: document.valueDefault * (1 + 0.1 * Math.random() - 0.05),
                  valueStringAtSource: '',
                  asduAtSource: 'M_ME_NC_1',
                  causeOfTransmissionAtSource: '3',
                  timeTag: new Date(),
                  timeTagAtSource: null,
                  timeTagAtSourceOk: false,
                  invalid: false,
                  invalidAtSource: false,
                  substitutedAtSource: false,
                  overflowAtSource: false,
                  blockedAtSource: false,
                  notTopicalAtSource: false,
                  test: true,
                  originator: AppDefs.NAME,
                  CntUpd: cntUpd,
                },
              },
            }
          )
      }
/*
      let res = await db
        .collection(jsConfig.RealtimeDataCollectionName)
        .updateMany(
          {
            type: 'analog',
            origin: 'supervised',
            _id: {
              $mod: [
                1 +
                  Math.floor(Math.random() * 50) +
                  Math.floor(Math.random() * 10),
                0,
              ],
            },
          },
          [
            {
              $set: {
                sourceDataUpdate: {
                  valueAtSource: {
                    $multiply: [
                      '$valueDefault',
                      1 + 0.1 * Math.random() - 0.05,
                    ],
                  },
                  valueStringAtSource: '',
                  asduAtSource: 'M_ME_NC_1',
                  causeOfTransmissionAtSource: '3',
                  invalid: false,
                  timeTag: '$$NOW',
                  timeTagAtSource: '$$NOW',
                  timeTagAtSourceOk: true,
                  substitutedAtSource: false,
                  overflowAtSource: false,
                  blockedAtSource: false,
                  notTopicalAtSource: false,
                  test: true,
                  originator: AppDefs.NAME,
                  CntUpd: cntUpd,
                },
              },
            },
          ]
        )
        .catch((err) => {
          Log.log(err)
          if (err.message.indexOf('ECONNREFUSED') > -1) {
            clientMongo = null
          }
        })

      Log.log(
        'Analog matchedCount: ' +
          res?.matchedCount +
          ' modifiedCount: ' +
          res?.modifiedCount
      )

      await db
        .collection(jsConfig.RealtimeDataCollectionName)
        .find({
          type: 'analog',
          origin: 'supervised',
          'sourceDataUpdate.CntUpd': cntUpd,
        })
        .toArray(async function (err, resarr) {
          resarr.forEach(async (element) => {
            element.sourceDataUpdate.CntUpd = 0
            let res2 = await db
              .collection(jsConfig.RealtimeDataCollectionName)
              .updateOne(
                {
                  _id: element._id,
                },
                {
                  $set: {
                    sourceDataUpdate: element.sourceDataUpdate,
                  },
                }
              )
            Log.log(
              'Analog matchedCount: ' +
                res2.matchedCount +
                ' modifiedCount: ' +
                res2.modifiedCount
            )
          })
        })
        .catch((err) => {
          Log.log(err)
          if (err.message.indexOf('ECONNREFUSED') > -1) {
            clientMongo = null
          }
        })

*/        
      cntUpd++
    }
  }, 2333)

  while (true) {
    if (clientMongo === null)
      await MongoClient.connect(
        jsConfig.mongoConnectionString,
        jsConfig.MongoConnectionOptions
      )
        .then(async (client) => {
          clientMongo = client
          Log.log('Connected correctly to MongoDB server')

          // specify db and collections
          const db = client.db(jsConfig.mongoDatabaseName)
          collection = db.collection(jsConfig.CommandsQueueCollectionName)

          const changeStream = collection.watch(pipeline, {
            fullDocument: 'updateLookup',
          })

          // start listen to changes
          changeStream
            .on('change', async (change) => {
              if (change.operationType === 'delete') return

              if (change.operationType === 'insert') {
                // document inserted
                Log.log('INSERT ' + change.fullDocument.tag)

                let data = await db
                  .collection(jsConfig.RealtimeDataCollectionName)
                  .findOne({ tag: change.fullDocument.tag })
                Log.log('Supervised of command: ' + data.supervisedOfCommand)
                Log.log('Command value: ' + change.fullDocument.value)
                let val = new Double(change.fullDocument.value)
                if (change.fullDocument.tag.indexOf('YTAP') !== -1) {
                  let res = await db
                  .collection(jsConfig.RealtimeDataCollectionName)
                  .findOne({ _id: data.supervisedOfCommand })
                  val = res.value
                  if (change.fullDocument.value === 0)
                    val = val - 1 
                  else val = val + 1
                }

                let res = await db
                  .collection(jsConfig.RealtimeDataCollectionName)
                  .updateOne(
                    { _id: data.supervisedOfCommand },
                    {
                      $set: {
                        sourceDataUpdate: {
                          valueAtSource: val,
                          valueStringAtSource: '',
                          asduAtSource: change.fullDocument.tag.indexOf('YTAP') === -1 ? 'M_SP_NA_1' : 'M_ST_TB_1',
                          causeOfTransmissionAtSource: '3',
                          invalid: false,
                          timeTag: new Date(),
                          timeTagAtSource: new Date(),
                          timeTagAtSourceOk: true,
                          invalidAtSource: false,
                          substitutedAtSource: false,
                          overflowAtSource: false,
                          blockedAtSource: false,
                          notTopicalAtSource: false,
                          test: true,
                          originator: AppDefs.NAME + "CMD",
                          CntUpd: cntUpd,
                        },
                      },
                    }
                  )
                Log.log(res?.matchedCount)
                Log.log(res?.modifiedCount)
                if (res?.matchedCount > 0) {
                  Log.log('ACK')
                  db.collection(jsConfig.CommandsQueueCollectionName).updateOne(
                    { _id: change.fullDocument._id },
                    { $set: { ack: true, ackTimeTag: new Date() } }
                  )
                }

                await db
                  .collection(jsConfig.RealtimeDataCollectionName)
                  .find({
                    origin: 'supervised',
                    'sourceDataUpdate.CntUpd': cntUpd,
                  })
                  .toArray(async function (err, resarr) {
                    resarr.forEach(async (element) => {
                      element.sourceDataUpdate.CntUpd = 0
                      let res2 = await db
                        .collection(jsConfig.RealtimeDataCollectionName)
                        .updateOne(
                          { _id: element._id },
                          {
                            $set: {
                              sourceDataUpdate: element.sourceDataUpdate,
                            },
                          }
                        )
                      Log.log(
                        'Digital matchedCount: ' +
                          res2?.matchedCount +
                          ' modifiedCount: ' +
                          res2?.modifiedCount
                      )
                    })
                  })
                cntUpd++
              }
            })
            .on('error', (err) => {
              if (clientMongo) clientMongo.close()
              clientMongo = null
              Log.log(err)
            })
        })
        .catch(function (err) {
          if (clientMongo) clientMongo.close()
          clientMongo = null
          Log.log(err)
        })

    // wait 5 seconds
    await new Promise((resolve) => setTimeout(resolve, 5000))

    // detect connection problems, if error will null the client to later reconnect
    if (clientMongo === undefined) {
      Log.log('Disconnected Mongodb!')
      clientMongo = null
    }
    if (clientMongo)
      if (!(await checkConnectedMongo(clientMongo))) {
        // not anymore connected, will retry
        Log.log('Disconnected Mongodb!')
        if (clientMongo) clientMongo.close()
        clientMongo = null
      }
  }
})()

// test mongoDB connectivity
async function checkConnectedMongo(client) {
  if (!client) {
    return false
  }
  const CheckMongoConnectionTimeout = 1000
  const tr = setTimeout(() => {
    Log.log('Mongo ping timeout error!')
    HintMongoIsConnected = false
  }, CheckMongoConnectionTimeout)

  let res = null
  try {
    res = await client.db('admin').command({ ping: 1 })
    clearTimeout(tr)
  } catch (e) {
    Log.log('Error on mongodb connection!')
    return false
  }
  if ('ok' in res && res.ok) {
    HintMongoIsConnected = true
    return true
  } else {
    HintMongoIsConnected = false
    return false
  }
}
