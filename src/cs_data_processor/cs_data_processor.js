/*
 * A process that watches for raw data updates from protocols using a MongoDB change stream.
 * Converts raw protocol values into analogs/statuses then updates realtime, soe and historical data.
 * {json:scada} - Copyright (c) 2020-2025 - Ricardo L. Olsen
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

const AppDefs = require('./app-defs')
const Log = require('./simple-logger')
const LoadConfig = require('./load-config')
const Redundancy = require('./redundancy')
const sqlFilesPath = '../../sql/'
const fs = require('fs')
const { MongoClient, Double } = require('mongodb')
const Queue = require('queue-fifo')
const { setInterval } = require('timers')

const MongoStatus = { HintMongoIsConnected: false }
const LowestPriorityThatBeeps = 1 // will beep for priorities zero and one

const args = process.argv.slice(2)
var inst = null
if (args.length > 0) inst = parseInt(args[0])

var logLevel = null
if (args.length > 1) logLevel = parseInt(args[1])
var confFile = null
if (args.length > 2) confFile = args[2]

const jsConfig = LoadConfig(confFile, logLevel, inst)

let DivideProcessingExpression = {}
if (
  AppDefs.ENV_PREFIX + 'DIVIDE_EXP' in process.env &&
  process.env[AppDefs.ENV_PREFIX + DIVIDE_EXP].trim() !== ''
) {
  try {
    DivideProcessingExpression = JSON.parse(
      process.env[AppDefs.ENV_PREFIX + 'DIVIDE_EXP']
    )
    Log.log(
      'Divide Processing Expression: ' +
        JSON.stringify(DivideProcessingExpression)
    )
  } catch (e) {
    DivideProcessingExpression = {}
    Log.log('Divide Processing Expression: ERROR!' + e)
    process.exit(1)
  }
}

const beepPointKey = -1
const cntUpdatesPointKey = -2
const invalidDetectCycle = 43000

Log.log('Connecting to MongoDB server...')

const pipeline = [
  {
    $project: { documentKey: false },
  },
  {
    $match: {
      $and: [
        { 'fullDocument.value': { $exists: true } },
        DivideProcessingExpression,
        {
          'updateDescription.updatedFields.sourceDataUpdate': { $exists: true },
        },
        {
          $or: [{ operationType: 'update' }],
        },
      ],
    },
  },
]

;(async () => {
  let collection = null
  let histCollection = null
  let sqlHistQueue = new Queue() // queue of historical values to insert on postgreSQL
  let sqlRtDataQueue = new Queue() // queue of realtime values to insert on postgreSQL
  let mongoRtDataQueue = new Queue() // queue of realtime values to insert on MongoDB
  let digitalUpdatesCount = 0

  // mark as frozen unchanged analog values greater than 1 after timeout
  setInterval(async function () {
    if (collection && MongoStatus.HintMongoIsConnected) {
      collection
        .updateMany(
          {
            $and: [
              { type: 'analog' },
              { invalid: false },
              { frozen: false },
              { frozenDetectTimeout: { $gt: 0.0 } },
              { timeTag: { $ne: null } },
              { $expr: { $gt: [{ $abs: '$value' }, 1.0] } },
              {
                $expr: {
                  $lt: [
                    '$timeTag',
                    {
                      $subtract: [
                        new Date(),
                        { $multiply: ['$frozenDetectTimeout', 1000.0] },
                      ],
                    },
                  ],
                },
              },
            ],
          },
          { $set: { frozen: true } }
        )
        .catch(function (err) {
          Log.log('Error on Mongodb query!', err)
        })
    }
  }, 17317)

  setInterval(async function () {
    if (
      !collection ||
      !MongoStatus.HintMongoIsConnected ||
      mongoRtDataQueue.isEmpty()
    )
      return
    let cnt = 0
    let updArr = []
    while (!mongoRtDataQueue.isEmpty()) {
      const upd = mongoRtDataQueue.peek()
      mongoRtDataQueue.dequeue()
      const _id = upd._id
      delete upd._id // remove _id for update
      let addToSet = null
      if ('$addToSet' in upd) {
        addToSet = upd.$addToSet
        delete upd.$addToSet
      }
      updArr.push({
        updateOne: {
          filter: { _id: _id },
          update: { $set: upd },
        },
      })
      cnt++
      if (addToSet) {
        updArr.push({
          updateOne: {
            filter: { _id: _id },
            update: { $addToSet: addToSet },
          },
        })
        cnt++
      }
    }
    const res = await collection
      .bulkWrite(updArr, {
        ordered: false,
        writeConcern: {
          w: 0,
        },
      })
      .catch(function (err) {
        Log.log('Error on Mongodb query!', err)
      })
    // Log.log(JSON.stringify(res))
    if (cnt) Log.log('Mongo Updates ' + cnt)
  }, 150)

  // write values to sql files for later insertion on postgreSQL
  setInterval(async function () {
    if (!histCollection || !MongoStatus.HintMongoIsConnected) return
    let doInsertData = false
    let sqlTransaction =
      'START TRANSACTION;\n' +
      'INSERT INTO hist (tag, time_tag, value, value_json, time_tag_at_source, flags) VALUES '

    let cntH = 0
    let insertArr = []
    while (!sqlHistQueue.isEmpty()) {
      doInsertData = true
      let entry = sqlHistQueue.peek()
      sqlHistQueue.dequeue()
      sqlTransaction = sqlTransaction + '\n(' + entry.sql + '),'
      insertArr.push(entry.obj)
      cntH++
    }
    if (cntH) Log.log('PGSQL/Mongo Hist updates ' + cntH)

    if (doInsertData) {
      histCollection
        .insertMany(insertArr, { ordered: false, writeConcern: { w: 0 } })
        .catch(function (err) {
          Log.log('Error on Mongodb query!', err)
        })
      sqlTransaction = sqlTransaction.substring(0, sqlTransaction.length - 1) // remove last comma
      sqlTransaction = sqlTransaction + ' \n'
      // this cause problems when tag/time repeated on same transaction
      // sqlTransaction = sqlTransaction + "ON CONFLICT (tag, time_tag) DO UPDATE SET value=EXCLUDED.value, value_json=EXCLUDED.value_json, time_tag_at_source=EXCLUDED.time_tag_at_source, flags=EXCLUDED.flags;\n";
      sqlTransaction =
        sqlTransaction + 'ON CONFLICT (tag, time_tag) DO NOTHING;\n'
      sqlTransaction = sqlTransaction + 'COMMIT;\n'
      fs.writeFile(
        sqlFilesPath +
          'pg_hist_' +
          new Date().getTime() +
          '_' +
          jsConfig.Instance +
          '.sql',
        sqlTransaction,
        (err) => {
          if (err) Log.log('Error writing SQL file!')
        }
      )
    }

    doInsertData = false
    sqlTransaction = ''
    let cntR = 0
    sqlTransaction =
      sqlTransaction +
      'WITH ordered_values AS (  SELECT DISTINCT ON (tag) tag, time_tag, json_data FROM (VALUES '
    while (!sqlRtDataQueue.isEmpty()) {
      doInsertData = true
      let sql = sqlRtDataQueue.peek()
      sqlRtDataQueue.dequeue()
      sqlTransaction = sqlTransaction + '\n (' + sql + '),'
      cntR++
    }
    sqlTransaction = sqlTransaction.substring(0, sqlTransaction.length - 1) // remove last comma
    sqlTransaction = sqlTransaction + ' \n'
    sqlTransaction =
      sqlTransaction +
      `) AS t(tag, time_tag, json_data)
          ORDER BY tag, time_tag DESC
        )
        INSERT INTO realtime_data (tag, time_tag, json_data)
        SELECT tag, time_tag::timestamptz, json_data::jsonb
        FROM ordered_values
        ON CONFLICT (tag) DO UPDATE 
        SET time_tag = EXCLUDED.time_tag,
            json_data = EXCLUDED.json_data;
    `
    if (cntR) Log.log('PGSQL RT updates ' + cntR)

    if (doInsertData) {
      fs.writeFile(
        sqlFilesPath +
          'pg_rtdata_' +
          new Date().getTime() +
          '_' +
          jsConfig.Instance +
          '.sql',
        sqlTransaction,
        (err) => {
          if (err) Log.log('Error writing SQL file!')
        }
      )
    }
  }, 1000)

  let clientMongo = null
  let invalidDetectIntervalHandle = null
  let latencyIntervalHandle = null
  while (true) {
    if (clientMongo === null)
      await MongoClient.connect(
        jsConfig.mongoConnectionString,
        jsConfig.MongoConnectionOptions
      )
        .then(async (client) => {
          clientMongo = client
          clientMongo.on('topologyClosed', (_) => {
            MongoStatus.HintMongoIsConnected = false
            Log.log('MongoDB server topologyClosed')
          })
          MongoStatus.HintMongoIsConnected = true
          Log.log('Connected correctly to MongoDB server')

          let latencyAccTotal = 0
          let latencyTotalCnt = 0
          let latencyAccMinute = 0
          let latencyMinuteCnt = 0
          let latencyPeak = 0
          clearInterval(latencyIntervalHandle)
          latencyIntervalHandle = setInterval(function () {
            latencyAccMinute = 0
            latencyMinuteCnt = 0
          }, 60000)

          // specify db and collections
          const db = client.db(jsConfig.mongoDatabaseName)
          collection = db.collection(jsConfig.RealtimeDataCollectionName)
          histCollection = db.collection(jsConfig.HistCollectionName)
          const changeStream = collection.watch(pipeline, {
            fullDocument: 'updateLookup',
          })

          await createSpecialTags(collection)

          Redundancy.Start(5000, clientMongo, db, jsConfig, MongoStatus)

          // periodically, mark invalid data when supervised points not updated within specified period (invalidDetectTimeout) for the point
          // check also stopped protocol driver instances
          clearInterval(invalidDetectIntervalHandle)
          invalidDetectIntervalHandle = setInterval(async function () {
            if (clientMongo !== null && MongoStatus.HintMongoIsConnected) {
              collection
                .updateMany(
                  {
                    $expr: {
                      $and: [
                        { $eq: ['$origin', 'supervised'] },
                        { $ne: ['$substituted', true] },
                        { $eq: ['$invalid', false] },
                        {
                          $lt: [
                            '$sourceDataUpdate.timeTag',
                            {
                              $subtract: [
                                new Date(),
                                { $multiply: [1000, '$invalidDetectTimeout'] },
                              ],
                            },
                          ],
                        },
                      ],
                    },
                  },
                  { $set: { invalid: true } }
                )
                .catch(function (err) {
                  Log.log('Error on Mongodb query!', err)
                })

              // look for client drivers instance not updating keep alive, if found invalidate all related data points of all its connections
              const results = await db
                .collection(jsConfig.ProtocolDriverInstancesCollectionName)
                .find({
                  $expr: {
                    $and: [
                      {
                        $in: [
                          '$protocolDriver',
                          [
                            'IEC60870-5-104',
                            'IEC60870-5-101',
                            'IEC60870-5-103',
                            'DNP3',
                            'MQTT-SPARKPLUG-B',
                            'OPC-UA',
                            'OPC-DA',
                            'TELEGRAF-LISTENER',
                            'PLCTAG',
                            'PLC4X',
                            'MODBUS',
                            'IEC61850',
                            'ICCP',
                          ],
                        ],
                      },
                      { $eq: ['$enabled', true] },
                      {
                        $lt: [
                          '$activeNodeKeepAliveTimeTag',
                          {
                            $subtract: [new Date(), { $multiply: [1000, 15] }],
                          },
                        ],
                      },
                    ],
                  },
                })
                .toArray()

              if (results && results.length > 0)
                for (let i = 0; i < results.length; i++) {
                  Log.log('PROTOCOL INSTANCE NOT RUNNING DETECTED!')
                  let instance = results[i]
                  Log.log(
                    'Driver Name: ' +
                      instance?.protocolDriver +
                      ' Instance Number: ' +
                      instance?.protocolDriverInstanceNumber
                  )
                  // find all connections related to his instance
                  const res = await db
                    .collection(jsConfig.ProtocolConnectionsCollectionName)
                    .find({
                      protocolDriver: instance?.protocolDriver,
                      protocolDriverInstanceNumber:
                        instance?.protocolDriverInstanceNumber,
                    })
                    .toArray()

                  if (res && res.length > 0)
                    for (let i = 0; i < res.length; i++) {
                      let connection = res[i]
                      Log.log(
                        'Data invalidated for connection: ' +
                          connection?.protocolConnectionNumber
                      )
                      await db
                        .collection(jsConfig.RealtimeDataCollectionName)
                        .updateMany(
                          {
                            origin: 'supervised',
                            protocolSourceConnectionNumber:
                              connection?.protocolConnectionNumber,
                            invalid: false,
                          },
                          {
                            $set: {
                              invalid: true,
                            },
                          }
                        )
                        .catch(function (err) {
                          Log.log('Error on Mongodb query!', err)
                        })
                    }
                }
            }
          }, invalidDetectCycle)

          try {
            changeStream.on('error', (change) => {
              if (clientMongo) clientMongo.close()
              clientMongo = null
              Log.log('Error on ChangeStream!')
            })
            changeStream.on('close', (change) => {
              clientMongo = null
              Log.log('Closed ChangeStream!')
            })
            changeStream.on('end', (change) => {
              if (clientMongo) clientMongo.close()
              clientMongo = null
              Log.log('Ended ChangeStream!')
            })

            // start listen to changes
            changeStream.on('change', (change) => {
              try {
                if (change.operationType === 'delete') return

                // // for older versions of mongodb
                // if (
                //   change.operationType === 'replace' &&
                //   !change?.updateDescription?.updatedFields &&
                //   change.fullDocument.sourceDataUpdate
                // ) {
                //   change['updateDescription'] = {
                //     updatedFields: {
                //       sourceDataUpdate: change.fullDocument.sourceDataUpdate,
                //     },
                //   }
                // }

                let isSOE = false
                let alarmRange = 0

                if (change.operationType === 'insert') {
                  // document inserted
                  Log.log(
                    'INSERT ' +
                      change.fullDocument._id +
                      ' ' +
                      change.fullDocument.tag +
                      ' ' +
                      value
                  )

                  sqlRtDataQueue.enqueue(
                    "'" +
                      change.fullDocument.tag +
                      "'," +
                      "'" +
                      new Date().toISOString() +
                      "'," +
                      "to_json('" +
                      JSON.stringify(change.fullDocument) +
                      "'::text)"
                  )
                }

                if (!Redundancy.ProcessStateIsActive())
                  // when inactive, ignore changes
                  return

                if (
                  !(
                    'sourceDataUpdate' in change.updateDescription.updatedFields
                  )
                )
                  // if not a Source Data Update (protocol update), return
                  return

                let delay =
                  new Date().getTime() -
                  change.updateDescription.updatedFields.sourceDataUpdate.timeTag.getTime()
                latencyAccTotal += delay
                latencyTotalCnt++
                latencyAccMinute += delay
                latencyMinuteCnt++
                if (delay > latencyPeak) latencyPeak = delay

                // consider SOE when digital changes has field timestamp
                // or analog with isEvent true
                if (
                  'timeTagAtSource' in
                  change.updateDescription.updatedFields.sourceDataUpdate
                )
                  if (
                    change.updateDescription.updatedFields.sourceDataUpdate
                      .timeTagAtSource !== null
                  )
                    if (
                      change.fullDocument.type === 'digital' ||
                      (change.fullDocument.type === 'analog' &&
                        change.fullDocument.isEvent)
                    ) {
                      if (
                        change.updateDescription.updatedFields.sourceDataUpdate.timeTagAtSource.getFullYear() >
                        1899
                      ) {
                        isSOE = true
                      }
                    }

                // check quality bits set by the protocol driver
                let invalid = false,
                  transient = false,
                  overflow = false,
                  nottopical = false,
                  carry = false,
                  substituted = false,
                  blocked = false
                if (
                  typeof change.updateDescription.updatedFields.sourceDataUpdate
                    .invalidAtSource === 'boolean'
                ) {
                  invalid =
                    change.updateDescription.updatedFields.sourceDataUpdate
                      .invalidAtSource
                }
                if (
                  typeof change.updateDescription.updatedFields.sourceDataUpdate
                    .notTopicalAtSource === 'boolean'
                ) {
                  invalid =
                    invalid ||
                    change.updateDescription.updatedFields.sourceDataUpdate
                      .notTopicalAtSource
                  nottopical =
                    change.updateDescription.updatedFields.sourceDataUpdate
                      .notTopicalAtSource
                }
                if (
                  typeof change.updateDescription.updatedFields.sourceDataUpdate
                    .overflowAtSource === 'boolean'
                ) {
                  invalid =
                    invalid ||
                    change.updateDescription.updatedFields.sourceDataUpdate
                      .overflowAtSource
                  overflow =
                    change.updateDescription.updatedFields.sourceDataUpdate
                      .overflowAtSource
                }
                if (
                  typeof change.updateDescription.updatedFields.sourceDataUpdate
                    .transientAtSource === 'boolean'
                ) {
                  invalid =
                    invalid ||
                    change.updateDescription.updatedFields.sourceDataUpdate
                      .transientAtSource
                  transient =
                    change.updateDescription.updatedFields.sourceDataUpdate
                      .transientAtSource
                }
                if (
                  typeof change.updateDescription.updatedFields.sourceDataUpdate
                    .carryAtSource === 'boolean'
                ) {
                  carry =
                    change.updateDescription.updatedFields.sourceDataUpdate
                      .carryAtSource
                }
                if (
                  typeof change.updateDescription.updatedFields.sourceDataUpdate
                    .substitutedAtSource === 'boolean'
                ) {
                  substituted =
                    change.updateDescription.updatedFields.sourceDataUpdate
                      .substitutedAtSource
                }
                if (
                  typeof change.updateDescription.updatedFields.sourceDataUpdate
                    .blockedAtSource === 'boolean'
                ) {
                  blocked =
                    change.updateDescription.updatedFields.sourceDataUpdate
                      .blockedAtSource
                }

                let value =
                  change.updateDescription.updatedFields.sourceDataUpdate
                    .valueAtSource
                let valueString =
                  change.updateDescription.updatedFields.sourceDataUpdate
                    ?.valueStringAtSource || ''
                let valueJson =
                  change.updateDescription.updatedFields.sourceDataUpdate
                    ?.valueJsonAtSource || ''
                let alarmed = change.fullDocument.alarmed

                // avoid undefined, null or NaN values
                if (value === null || value === undefined || isNaN(value)) {
                  value = 0.0
                  invalid = true
                }

                // Qualifier to be shown in valueString
                let txtQualif = ''
                txtQualif = txtQualif + (invalid ? '[IV]' : '')
                txtQualif = txtQualif + (transient ? '[TR]' : '')
                txtQualif = txtQualif + (overflow ? '[OV]' : '')
                txtQualif = txtQualif + (nottopical ? '[NT]' : '')
                txtQualif = txtQualif + (carry ? '[CR]' : '')
                txtQualif = txtQualif + (substituted ? '[SB]' : '')
                txtQualif = txtQualif + (blocked ? '[BK]' : '')

                if (change.fullDocument.type === 'digital') {
                  // test for double point status
                  if (
                    'asduAtSource' in
                    change.updateDescription.updatedFields.sourceDataUpdate
                  ) {
                    if (
                      change.updateDescription.updatedFields.sourceDataUpdate.asduAtSource.indexOf(
                        'M_DP_'
                      ) === 0
                    ) {
                      if (value === 0 || value === 3) {
                        transient = true
                        invalid = true
                        if (txtQualif.indexOf('[IV]') < 0)
                          txtQualif = txtQualif + (transient ? '[IV]' : '')
                        if (txtQualif.indexOf('[TR]') < 0)
                          txtQualif = txtQualif + (transient ? '[TR]' : '')
                        if (txtQualif !== '') txtQualif = ' ' + txtQualif
                      }
                      value = (value & 0x01) == 0 ? 1 : 0
                    }
                  }

                  // process inversions (kconv1=-1)
                  if (change.fullDocument.kconv1 === -1)
                    value = value === 0 ? 1 : 0
                  if (
                    value != change.fullDocument.value &&
                    !change.fullDocument.alarmDisabled
                  )
                    alarmed = true
                  if (value)
                    valueString =
                      change.fullDocument.stateTextTrue +
                      (change.fullDocument.unit != ''
                        ? ' ' + change.fullDocument.unit
                        : '') +
                      txtQualif
                  else
                    valueString =
                      change.fullDocument.stateTextFalse +
                      (change.fullDocument.unit != ''
                        ? ' ' + change.fullDocument.unit
                        : '') +
                      txtQualif
                } else if (change.fullDocument.type === 'analog') {
                  if (txtQualif != '') txtQualif = ' ' + txtQualif

                  // apply conversion factors
                  value =
                    change.updateDescription.updatedFields.sourceDataUpdate
                      .valueAtSource *
                      change.fullDocument.kconv1 +
                    change.fullDocument.kconv2

                  if ('zeroDeadband' in change.fullDocument)
                    if (
                      change.fullDocument.zeroDeadband !== 0 &&
                      Math.abs(value) < change.fullDocument.zeroDeadband
                    )
                      value = 0.0

                  valueString =
                    '' +
                    parseFloat(value.toFixed(4)) +
                    ' ' +
                    change.fullDocument.unit +
                    txtQualif

                  if (
                    'asduAtSource' in
                    change.updateDescription.updatedFields.sourceDataUpdate
                  )
                    if (
                      change.updateDescription.updatedFields.sourceDataUpdate.asduAtSource.indexOf(
                        'M_BO_'
                      ) === 0
                    ) {
                      // test for bitstring
                      valueString =
                        value.toString(2) +
                        ' ' +
                        change.fullDocument.unit +
                        txtQualif
                    }

                  let hysteresis = 0
                  if (change.fullDocument?.hysteresis)
                    hysteresis = parseFloat(change.fullDocument.hysteresis)

                  // check for limits
                  if (
                    // value != change.fullDocument.value &&
                    'hiLimit' in change.fullDocument &&
                    change.fullDocument.hiLimit !== null &&
                    'loLimit' in change.fullDocument &&
                    change.fullDocument.loLimit !== null &&
                    !change.fullDocument.alarmDisabled
                  ) {
                    if (value > change.fullDocument.hiLimit + hysteresis) {
                      alarmRange = 1
                    } else if (
                      value <
                      change.fullDocument.loLimit - hysteresis
                    ) {
                      alarmRange = -1
                    } else if (
                      value < change.fullDocument.hiLimit - hysteresis &&
                      value > change.fullDocument.loLimit + hysteresis
                    ) {
                      alarmed = false
                      alarmRange = 0
                    } else if (change.fullDocument?.alarmRange)
                      // keep the old range if out of range
                      alarmRange = change.fullDocument.alarmRange

                    // create a SOE entry for the limits alarm/normalization when analog alarm condition changes
                    //if (alarmed != change.fullDocument.alarmed)
                    //if (
                    //    change.fullDocument.value <= change.fullDocument.hiLimit + hysteresis &&
                    //    value > change.fullDocument.hiLimit + hysteresis
                    //    ||
                    //    change.fullDocument.value >= change.fullDocument.hiLimit - hysteresis &&
                    //    value < change.fullDocument.hiLimit - hysteresis
                    //    ||
                    //    change.fullDocument.value >= change.fullDocument.loLimit - hysteresis  &&
                    //    value < change.fullDocument.loLimit - hysteresis
                    //    ||
                    //    change.fullDocument.value <= change.fullDocument.loLimit + hysteresis  &&
                    //    value > change.fullDocument.loLimit + hysteresis
                    //      )
                    if (!change.fullDocument.alarmDisabled)
                      if (change.fullDocument?.alarmRange != alarmRange) {
                        if (alarmRange != 0) alarmed = true
                        const eventDate = new Date()
                        const eventText =
                          parseFloat(value.toFixed(3)) +
                          ' ' +
                          change.fullDocument.unit +
                          (Math.abs(value) >
                          Math.abs(change.fullDocument?.value)
                            ? ' ⤉'
                            : Math.abs(value) <
                              Math.abs(change.fullDocument?.value)
                            ? ' ⤈'
                            : '') +
                          (alarmed ? ' 🚩' : ' 🆗')
                        db.collection(jsConfig.SoeDataCollectionName)
                          .insertOne(
                            {
                              tag: change.fullDocument.tag,
                              pointKey: change.fullDocument._id,
                              group1: change.fullDocument.group1,
                              description: change.fullDocument.description,
                              eventText: eventText,
                              invalid: false,
                              priority: change.fullDocument.priority,
                              timeTag: eventDate,
                              timeTagAtSource: eventDate,
                              timeTagAtSourceOk: true,
                              ack: alarmed ? 0 : 1, // enter as acknowledged when normalized
                            },
                            {
                              writeConcern: {
                                w: 0,
                              },
                            }
                          )
                          .catch(function (err) {
                            Log.log('Error on Mongodb query!', err)
                          })
                      }
                  }

                  // analog tags can produce SOE events when marked as isEvent and valid value change, or having source timestamp
                  if (!change.fullDocument.alarmDisabled)
                    if (
                      (change.fullDocument?.isEvent === true &&
                        !invalid &&
                        value !== change.fullDocument?.value) ||
                      isSOE
                    ) {
                      const eventText =
                        parseFloat(value.toFixed(3)) +
                        ' ' +
                        change.fullDocument.unit +
                        (Math.abs(value) > Math.abs(change.fullDocument?.value)
                          ? ' ↑'
                          : Math.abs(value) <
                            Math.abs(change.fullDocument?.value)
                          ? ' ↓'
                          : '')
                      db.collection(jsConfig.SoeDataCollectionName)
                        .insertOne(
                          {
                            tag: change.fullDocument.tag,
                            pointKey: change.fullDocument._id,
                            group1: change.fullDocument.group1,
                            description: change.fullDocument.description,
                            eventText: eventText,
                            invalid: false,
                            priority: change.fullDocument.priority,
                            timeTag: new Date(),
                            timeTagAtSource: isSOE
                              ? change.updateDescription.updatedFields
                                  .sourceDataUpdate.timeTagAtSource
                              : new Date(),
                            timeTagAtSourceOk: isSOE
                              ? change.updateDescription.updatedFields
                                  .sourceDataUpdate.timeTagAtSourceOk
                              : false,
                            ack: 1, // enter as acknowledged as it is not an alarm
                          },
                          {
                            writeConcern: {
                              w: 0,
                            },
                          }
                        )
                        .catch(function (err) {
                          Log.log('Error on Mongodb query!', err)
                        })
                    }
                }

                let alarmTime = null
                // if changed to alarmed state, or digital change or soe, register new alarm tag
                if (!change.fullDocument.alarmDisabled && alarmed) {
                  if (
                    !change.fullDocument.alarmed ||
                    (change.fullDocument.type === 'digital' &&
                      value !== change.fullDocument.value) ||
                    (change.fullDocument.type === 'digital' && isSOE)
                  ) {
                    alarmTime = new Date()
                  }
                }

                // update only realtimeData if changed or for SOE, must not be historical backfill
                if (
                  (isSOE ||
                    change.updateDescription.updatedFields.sourceDataUpdate
                      ?.rangeCheck ||
                    value !== change.fullDocument.value ||
                    valueString !== change.fullDocument.valueString ||
                    invalid !== change.fullDocument.invalid) &&
                  !change.updateDescription.updatedFields.sourceDataUpdate
                    ?.isHistorical
                ) {
                  let dt = new Date()

                  if (!change.fullDocument.alarmDisabled) {
                    if (
                      (alarmed &&
                        isSOE &&
                        change.fullDocument?.isEvent === true &&
                        change.fullDocument.type === 'digital' &&
                        value != 0) ||
                      (alarmed &&
                        change.fullDocument?.isEvent === false &&
                        change.fullDocument.type === 'digital') ||
                      (alarmed && change.fullDocument?.alarmed === false)
                    ) {
                      // a new alarm, then update beep var
                      Log.log('NEW BEEP, tag: ' + change.fullDocument.tag)
                      if (change.fullDocument.priority === 0)
                        // signal an important beep (for alarm of priority 0)
                        mongoRtDataQueue.enqueue({
                          _id: beepPointKey,
                          beepType: new Double(2), // this is an important beep
                          value: new Double(1),
                          valueString: 'Beep Active',
                          timeTag: dt,
                          $addToSet: {
                            beepGroup1List: change.fullDocument.group1,
                          },
                        })
                      else if (
                        change.fullDocument.priority <= LowestPriorityThatBeeps
                      )
                        mongoRtDataQueue.enqueue({
                          _id: beepPointKey,
                          value: new Double(1),
                          valueString: 'Beep Active',
                          timeTag: dt,
                          $addToSet: {
                            beepGroup1List: change.fullDocument.group1,
                          },
                        })
                    }
                    if (change.fullDocument.type === 'digital') {
                      digitalUpdatesCount++
                      mongoRtDataQueue.enqueue({
                        _id: cntUpdatesPointKey,
                        value: new Double(digitalUpdatesCount),
                        valueString: '' + digitalUpdatesCount + ' Updates',
                        timeTag: dt,
                      })
                    }
                  }

                  // historianPeriod<0 or update is not for historical record, excludes from historian
                  let insertIntoHistorian = true
                  if ('historianPeriod' in change.fullDocument) {
                    if (
                      change.fullDocument.historianPeriod < 0 ||
                      change.updateDescription.updatedFields.sourceDataUpdate
                        ?.isNotForHistorical
                    ) {
                      insertIntoHistorian = false
                    } else {
                      // historianPeriod >= 0, will test dead band for analogs
                      if (
                        change.fullDocument?.type === 'analog' &&
                        'historianDeadBand' in change.fullDocument
                      ) {
                        if (
                          'historianLastValue' in change.fullDocument &&
                          change.fullDocument.historianLastValue !== null &&
                          change.fullDocument.historianDeadBand > 0
                        ) {
                          // test for variation less than absolute dead band
                          if (
                            Math.abs(
                              value - change.fullDocument.historianLastValue
                            ) < Math.abs(change.fullDocument.historianDeadBand)
                          ) {
                            insertIntoHistorian = false
                          }
                        }
                      }
                    }
                  }

                  let update = {
                    _id: change.fullDocument._id,
                    value: new Double(value),
                    valueString: valueString,
                    valueJson: valueJson,
                    ...(change.fullDocument?.type === 'analog' &&
                    insertIntoHistorian
                      ? { historianLastValue: new Double(value) }
                      : {}),
                    timeTag: dt,
                    overflow: overflow,
                    invalid: invalid,
                    transient: transient,
                    frozen: false,
                    timeTagAtSource: null,
                    timeTagAtSourceOk: null,
                    updatesCnt: new Double(change.fullDocument.updatesCnt + 1),
                    alarmRange: new Double(alarmRange),
                    alarmed:
                      change.fullDocument?.alarmDisabled === true
                        ? false
                        : alarmed,
                  }
                  if (alarmTime !== null) update.timeTagAlarm = alarmTime

                  // update source time when available
                  if (
                    'timeTagAtSource' in
                      change.updateDescription.updatedFields.sourceDataUpdate &&
                    change.updateDescription.updatedFields.sourceDataUpdate
                      .timeTagAtSource !== null
                  ) {
                    update.timeTagAtSource =
                      change.updateDescription.updatedFields.sourceDataUpdate.timeTagAtSource
                    update.timeTagAtSourceOk =
                      change.updateDescription.updatedFields.sourceDataUpdate.timeTagAtSourceOk
                  }

                  // do not update protection-like events for state OFF, do not update when not for historical backfill
                  if (
                    !(
                      change.fullDocument.isEvent &&
                      change.fullDocument.type === 'digital' &&
                      value === 0 &&
                      !change.updateDescription.updatedFields.sourceDataUpdate
                        ?.isHistorical
                    )
                  ) {
                    mongoRtDataQueue.enqueue(update)

                    Log.log(
                      'UPD ' +
                        change.fullDocument._id +
                        ' ' +
                        change.fullDocument.tag +
                        ' ' +
                        value +
                        ' DELAY ' +
                        (new Date().getTime() -
                          change.updateDescription.updatedFields.sourceDataUpdate.timeTag.getTime()) +
                        'ms',
                      Log.levelDetailed
                    )
                  }

                  // build sql values list for queued insert into historian
                  // Fields: tag, time_tag, value, value_json, time_tag_at_source, flags
                  if (insertIntoHistorian) {
                    // queue data change for postgresql historian
                    let b7 = invalid ? '1' : '0', // value invalid
                      b6 =
                        update.timeTagAtSourceOk !== null
                          ? update.timeTagAtSourceOk
                            ? '0'
                            : '1'
                          : '1', // time tag at source invalid
                      b5 = change.fullDocument.type === 'analog' ? '1' : '0', // analog
                      b4 =
                        change.updateDescription.updatedFields.sourceDataUpdate
                          .causeOfTransmissionAtSource === '20'
                          ? '1'
                          : '0', // integrity?
                      b3 = '0', // reserved
                      b2 = '0', // reserved
                      b1 = '0', // reserved
                      b0 = '0' // reserved
                    sqlHistQueue.enqueue({
                      sql:
                        "'" +
                        change.fullDocument.tag.replaceAll("'", "''") +
                        "'," +
                        "'" +
                        change.updateDescription.updatedFields.sourceDataUpdate.timeTag.toISOString() +
                        "'," +
                        value +
                        ',to_json(' +
                        "'{" +
                        '"v": ' +
                        JSON.stringify(valueJson).replaceAll("'", "''") +
                        ',' +
                        '"s": "' +
                        valueString.replaceAll("'", "''") +
                        '"}\'::text),' +
                        (update.timeTagAtSource !== null
                          ? "'" +
                            change.updateDescription.updatedFields.sourceDataUpdate.timeTagAtSource.toISOString() +
                            "'"
                          : 'null') +
                        ',' +
                        "B'" +
                        b7 +
                        b6 +
                        b5 +
                        b4 +
                        b3 +
                        b2 +
                        b1 +
                        b0 +
                        "'",
                      obj: {
                        tag: change.fullDocument.tag,
                        timeTag:
                          change.updateDescription.updatedFields
                            .sourceDataUpdate.timeTag,
                        value:
                          change.fullDocument.type === 'string'
                            ? valueString
                            : change.fullDocument.type === 'json'
                            ? valueJson
                            : value,
                        invalid: invalid,
                        ...(update.timeTagAtSource !== null
                          ? { timeTagAtSource: update.timeTagAtSource }
                          : {}),
                        ...(update.timeTagAtSourceOk !== null
                          ? { timeTagAtSourceOk: update.timeTagAtSourceOk }
                          : {}),
                        ...(change.updateDescription.updatedFields
                          .sourceDataUpdate?.causeOfTransmissionAtSource
                          ? {
                              cot: change.updateDescription.updatedFields
                                .sourceDataUpdate.causeOfTransmissionAtSource,
                            }
                          : {}),
                      },
                    })
                  }

                  // update change.fullDocument with new data just to stringify it and queue update for postgresql update
                  change.fullDocument.value = value
                  change.fullDocument.valueString = valueString
                  change.fullDocument.valueJson = valueJson
                  change.fullDocument.timeTag = dt
                  change.fullDocument.overflow = overflow
                  change.fullDocument.invalid = invalid
                  change.fullDocument.transient = transient
                  change.fullDocument.updatesCnt =
                    change.fullDocument.updatesCnt + 1
                  change.fullDocument.alarmed = alarmed
                  let queueStr =
                    "'" +
                    change.fullDocument.tag.replaceAll("'", "''") +
                    "'," +
                    "'" +
                    new Date().toISOString() +
                    "'," +
                    "'" +
                    JSON.stringify(change.fullDocument).replaceAll("'", "''") +
                    "'"
                  sqlRtDataQueue.enqueue(queueStr)
                } else
                  Log.log(
                    'Not changed ' +
                      change.fullDocument.tag +
                      ' DELAY ' +
                      (new Date().getTime() -
                        change.updateDescription.updatedFields.sourceDataUpdate.timeTag.getTime()) +
                      'ms',
                    Log.levelDetailed
                  )

                // prepare update to soeData collection, do not put into SOE when alarm disabled or update is not for historical record
                if (
                  isSOE &&
                  !change.fullDocument.alarmDisabled &&
                  !change.updateDescription.updatedFields.sourceDataUpdate
                    ?.isNotForHistorical
                )
                  if (!(value === 0 && change.fullDocument.isEvent)) {
                    let eventText = change.fullDocument.eventTextFalse
                    if (value !== 0) {
                      eventText = change.fullDocument.eventTextTrue
                    }

                    db.collection(jsConfig.SoeDataCollectionName)
                      .insertOne(
                        {
                          tag: change.fullDocument.tag,
                          pointKey: change.fullDocument._id,
                          group1: change.fullDocument.group1,
                          description: change.fullDocument.description,
                          eventText: eventText,
                          invalid: invalid,
                          priority: change.fullDocument.priority,
                          timeTag: new Date(),
                          timeTagAtSource:
                            change.updateDescription.updatedFields
                              .sourceDataUpdate.timeTagAtSource,
                          timeTagAtSourceOk:
                            change.updateDescription.updatedFields
                              .sourceDataUpdate.timeTagAtSourceOk,
                          ack: 0,
                        },
                        {
                          writeConcern: {
                            w: 0,
                          },
                        }
                      )
                      .catch(function (err) {
                        Log.log('Error on Mongodb query!', err)
                      })
                    Log.log(
                      'SOE ' +
                        change.fullDocument._id +
                        ' ' +
                        change.fullDocument.tag +
                        ' ' +
                        change.updateDescription.updatedFields.sourceDataUpdate
                          .valueAtSource +
                        ' ' +
                        change.updateDescription.updatedFields.sourceDataUpdate
                          .timeTagAtSource,
                      Log.levelDetailed
                    )
                  }
              } catch (e) {
                Log.log(e)
              }
            })
          } catch (e) {
            Log.log(e)
          }
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
  let tr = setTimeout(() => {
    Log.log('Mongo ping timeout error!')
    MongoStatus.HintMongoIsConnected = false
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
    MongoStatus.HintMongoIsConnected = true
    return true
  } else {
    MongoStatus.HintMongoIsConnected = false
    return false
  }
}

async function createSpecialTags(collection) {
  // insert special tags when not found
  let results = await collection.find({ _id: beepPointKey }).toArray()
  if (results && results.length == 0) {
    collection
      .insertOne({
        _id: new Double(beepPointKey),
        alarmRange: new Double(0.0),
        alarmDisabled: true,
        alarmState: new Double(1.0),
        alarmed: false,
        annotation: '',
        commandBlocked: false,
        commandOfSupervised: new Double(0.0),
        description: '_System~Status~Alarm Beep',
        eventTextFalse: 'Beep Deactivated',
        eventTextTrue: 'Beep Activated',
        formula: null,
        frozen: false,
        frozenDetectTimeout: new Double(300.0),
        group1: '_System',
        group2: 'Status',
        group3: '',
        hiLimit: null,
        hihiLimit: null,
        hihihiLimit: null,
        historianDeadBand: new Double(0.0),
        historianPeriod: new Double(0.0),
        hysteresis: new Double(0.0),
        invalid: true,
        invalidDetectTimeout: new Double(0.0),
        isEvent: false,
        kconv1: new Double(1.0),
        kconv2: new Double(0.0),
        loLimit: null,
        location: null,
        loloLimit: null,
        lololoLimit: null,
        notes: '',
        origin: 'system',
        overflow: false,
        parcels: null,
        priority: new Double(3.0),
        protocolSourceASDU: new Double(0.0),
        protocolSourceCommandDuration: null,
        protocolSourceCommandUseSBO: null,
        protocolSourceCommonAddress: new Double(0.0),
        protocolSourceConnectionNumber: new Double(0.0),
        protocolSourceObjectAddress: new Double(0.0),
        sourceDataUpdate: null,
        stateTextFalse: 'No Beep',
        stateTextTrue: 'Active Beep',
        substituted: false,
        supervisedOfCommand: new Double(0.0),
        tag: '_System.Status.AlarmBeep',
        timeTag: { $date: '2000-01-01T00:00:00.000Z' },
        transient: false,
        type: 'analog',
        ungroupedDescription: 'Alarm Beep',
        unit: 'Enum',
        updatesCnt: new Double(0.0),
        value: new Double(0.0),
        valueDefault: new Double(0.0),
        valueString: 'No Beep',
        timeTagAtSource: null,
        timeTagAtSourceOk: null,
        beepType: new Double(0.0),
        beepGroup1List: [],
      })
      .catch(function (err) {
        Log.log('Error on Mongodb query!', err)
      })
  } else {
    await collection.updateOne(
      { _id: beepPointKey, beepGroup1List: { $exists: false } },
      { $set: { beepGroup1List: [] } }
    )
  }
  results = await collection.find({ _id: cntUpdatesPointKey }).toArray()
  if (results && results.length == 0) {
    collection
      .insertOne({
        _id: new Double(cntUpdatesPointKey),
        alarmRange: new Double(0),
        alarmDisabled: true,
        alarmState: new Double(1.0),
        alarmed: false,
        annotation: '',
        commandBlocked: false,
        commandOfSupervised: new Double(0.0),
        description: '_System~Status~Digital Updates Count',
        eventTextFalse: '',
        eventTextTrue: '',
        formula: null,
        frozen: false,
        frozenDetectTimeout: new Double(300.0),
        group1: '_System',
        group2: 'Status',
        group3: '',
        hiLimit: null,
        hihiLimit: null,
        hihihiLimit: null,
        historianDeadBand: new Double(0.0),
        historianPeriod: new Double(0.0),
        hysteresis: new Double(0.0),
        invalid: true,
        invalidDetectTimeout: new Double(0.0),
        isEvent: false,
        kconv1: new Double(1.0),
        kconv2: new Double(0.0),
        loLimit: null,
        location: null,
        loloLimit: null,
        lololoLimit: null,
        notes: '',
        origin: 'system',
        overflow: false,
        parcels: null,
        priority: new Double(3.0),
        protocolSourceASDU: new Double(0.0),
        protocolSourceCommandDuration: null,
        protocolSourceCommandUseSBO: null,
        protocolSourceCommonAddress: new Double(0.0),
        protocolSourceConnectionNumber: new Double(0.0),
        protocolSourceObjectAddress: new Double(0.0),
        sourceDataUpdate: null,
        stateTextFalse: '',
        stateTextTrue: '',
        substituted: false,
        supervisedOfCommand: new Double(0.0),
        tag: '_System.Status.DigitalUpdatesCnt',
        timeTag: { $date: '2000-01-01T00:00:00.000Z' },
        transient: false,
        type: 'analog',
        ungroupedDescription: 'Digital Updates Count',
        unit: 'Updates',
        updatesCnt: new Double(0.0),
        value: new Double(0.0),
        valueDefault: new Double(0.0),
        valueString: '0 Updates',
        timeTagAtSource: null,
        timeTagAtSourceOk: null,
      })
      .catch(function (err) {
        Log.log('Error on Mongodb query!', err)
      })
  }
}
