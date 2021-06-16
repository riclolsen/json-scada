'use strict'

/*
 * A process that watches for raw data updates from protocols using a MongoDB change stream.
 * Convert raw values and update realtime values and statuses.
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

const APP_NAME = 'CS_DATA_PROCESSOR'
const APP_MSG = '{json:scada} - Change Stream Data Processor'
const VERSION = '0.1.1'
const Log = require('./simple-logger')
let ProcessActive = false // for redundancy control
var jsConfigFile = '../../conf/json-scada.json'
const sqlFilesPath = '../../sql/'
const fs = require('fs')
const mongo = require('mongodb')
const MongoClient = require('mongodb').MongoClient
const Queue = require('queue-fifo')
const { setInterval } = require('timers')

const args = process.argv.slice(2)
var inst = null
if (args.length > 0) inst = parseInt(args[0])
const Instance = inst || process.env.JS_CSDATAPROC_INSTANCE || 1

var logLevel = null
if (args.length > 1) logLevel = parseInt(args[1])
Log.logLevelCurrent = logLevel || process.env.JS_CSDATAPROC_LOGLEVEL || 1

var confFile = null
if (args.length > 2) confFile = args[2]
jsConfigFile = confFile || process.env.JS_CONFIG_FILE || jsConfigFile

Log.log(APP_MSG + ' Version ' + VERSION)
Log.log('Instance: ' + Instance)
Log.log('Log level: ' + Log.logLevelCurrent)
Log.log('Config File: ' + jsConfigFile)

if (!fs.existsSync(jsConfigFile)) {
  Log.log('Error: config file not found!', Log.levelMin)
  process.exit()
}

const RealtimeDataCollectionName = 'realtimeData'
const ProcessInstancesCollectionName = 'processInstances'
const ProtocolDriverInstancesCollectionName = 'protocolDriverInstances'
const ProtocolConnectionsCollectionName = 'protocolConnections'
const beepPointKey = -1
const cntUpdatesPointKey = -2
const invalidDetectCycle = 15000

let rawFileContents = fs.readFileSync(jsConfigFile)
let jsConfig = JSON.parse(rawFileContents)
if (
  typeof jsConfig.mongoConnectionString != 'string' ||
  jsConfig.mongoConnectionString === ''
) {
  Log.log('Error reading config file.', Log.levelMin)
  process.exit()
}

Log.log('Connecting to MongoDB server...')

const pipeline = [
  {
    $project: { documentKey: false }
  },
  {
    $match: {
      $and: [
        { 'fullDocument.value': { $exists: true } },
        {
          'updateDescription.updatedFields.sourceDataUpdate': { $exists: true }
        },
        {
          $or: [{ operationType: 'update' }]
        }
      ]
    }
  }
]

;(async () => {
  let collection = null
  let sqlHistQueue = new Queue() // queue of historical values to insert on postgreSQL
  let sqlRtDataQueue = new Queue() // queue of realtime values to insert on postgreSQL
  let mongoRtDataQueue = new Queue() // queue of realtime values to insert on MongoDB
  let digitalUpdatesCount = 0

  // mark as frozen unchanged analog values greater than 1 after timeout
  setInterval(async function () {
    if (collection) {
      collection.updateMany(
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
                      { $multiply: ['$frozenDetectTimeout', 1000.0] }
                    ]
                  }
                ]
              }
            }
          ]
        },
        { $set: { frozen: true } }
      )
    }
  }, 17317)

  setInterval(async function () {
    let cnt = 0
    if (collection)
      while (!mongoRtDataQueue.isEmpty()) {
        let upd = mongoRtDataQueue.peek()
        let where = { _id: upd._id }
        delete upd._id // remove _id for update
        collection.updateOne(where, {
          $set: upd
        })
        mongoRtDataQueue.dequeue()
        cnt++
      }
    if (cnt) Log.log('Mongo Updates ' + cnt)
  }, 150)

  // write values to sql files for later insertion on postgreSQL
  setInterval(async function () {
    let doInsertData = false
    let sqlTransaction =
      'START TRANSACTION;\n' +
      'INSERT INTO hist (tag, time_tag, value, value_json, time_tag_at_source, flags) VALUES '

    let cntH = 0
    while (!sqlHistQueue.isEmpty()) {
      doInsertData = true
      let sql = sqlHistQueue.peek()
      sqlHistQueue.dequeue()
      sqlTransaction = sqlTransaction + '\n(' + sql + '),'
      cntH++
    }
    if (cntH) Log.log('PGSQL Hist updates ' + cntH)

    if (doInsertData) {
      sqlTransaction = sqlTransaction.substr(0, sqlTransaction.length - 1) // remove last comma
      sqlTransaction = sqlTransaction + ' \n'
      // this cause problems when tag/time repeated on same transaction
      // sqlTransaction = sqlTransaction + "ON CONFLICT (tag, time_tag) DO UPDATE SET value=EXCLUDED.value, value_json=EXCLUDED.value_json, time_tag_at_source=EXCLUDED.time_tag_at_source, flags=EXCLUDED.flags;\n";
      sqlTransaction =
        sqlTransaction + 'ON CONFLICT (tag, time_tag) DO NOTHING;\n'
      sqlTransaction = sqlTransaction + 'COMMIT;\n'
      fs.writeFile(
        sqlFilesPath + 'pg_hist_' + new Date().getTime() + '.sql',
        sqlTransaction,
        err => {
          if (err) Log.log('Error writing SQL file!')
        }
      )
    }

    doInsertData = false
    sqlTransaction = 'START TRANSACTION;\n'
    let cntR = 0
    while (!sqlRtDataQueue.isEmpty()) {
      doInsertData = true
      let sql = sqlRtDataQueue.peek()
      sqlRtDataQueue.dequeue()
      sqlTransaction =
        sqlTransaction +
        'INSERT INTO realtime_data (tag, time_tag, json_data) VALUES '
      sqlTransaction = sqlTransaction + ' (' + sql + ') '
      sqlTransaction =
        sqlTransaction +
        'ON CONFLICT (tag) DO UPDATE SET time_tag=EXCLUDED.time_tag, json_data=EXCLUDED.json_data;\n'
      cntR++
    }
    if (cntR) Log.log('PGSQL RT updates ' + cntR)

    if (doInsertData) {
      sqlTransaction = sqlTransaction + 'COMMIT;\n'
      fs.writeFile(
        sqlFilesPath + 'pg_rtdata_' + new Date().getTime() + '.sql',
        sqlTransaction,
        err => {
          if (err) Log.log('Error writing SQL file!')
        }
      )
    }
  }, 1000)

  let connOptions = {
    useNewUrlParser: true,
    useUnifiedTopology: true,
    appname: APP_NAME + ' Version:' + VERSION + ' Instance:' + Instance,
    poolSize: 20,
    readPreference: MongoClient.READ_PRIMARY
  }

  if (
    typeof jsConfig.tlsCaPemFile === 'string' &&
    jsConfig.tlsCaPemFile.trim() !== ''
  ) {
    jsConfig.tlsClientKeyPassword = jsConfig.tlsClientKeyPassword || ''
    jsConfig.tlsAllowInvalidHostnames =
      jsConfig.tlsAllowInvalidHostnames || false
    jsConfig.tlsAllowChainErrors = jsConfig.tlsAllowChainErrors || false
    jsConfig.tlsInsecure = jsConfig.tlsInsecure || false

    connOptions.tls = true
    connOptions.tlsCAFile = jsConfig.tlsCaPemFile
    connOptions.tlsCertificateKeyFile = jsConfig.tlsClientPemFile
    connOptions.tlsCertificateKeyFilePassword = jsConfig.tlsClientKeyPassword
    connOptions.tlsAllowInvalidHostnames = jsConfig.tlsAllowInvalidHostnames
    connOptions.tlsInsecure = jsConfig.tlsInsecure
  }

  let clientMongo = null
  let invalidDetectIntervalHandle = null
  let redundancyIntervalHandle = null
  let latencyIntervalHandle = null
  while (true) {
    if (clientMongo === null)
      await MongoClient.connect(jsConfig.mongoConnectionString, connOptions)
        .then(async client => {
          clientMongo = client
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
          collection = db.collection(RealtimeDataCollectionName)
          const changeStream = collection.watch(pipeline, {
            fullDocument: 'updateLookup'
          })

          let lastActiveNodeKeepAliveTimeTag = null
          let countKeepAliveNotUpdated = 0
          let countKeepAliveUpdatesLimit = 4
          async function ProcessRedundancy () {
            if (!clientMongo) return
            // look for process instance entry, if not found create a new entry
            db.collection(ProcessInstancesCollectionName)
              .find({
                processName: APP_NAME,
                processInstanceNumber: Instance
              })
              .toArray(function (err, results) {
                if (err) Log.log(err)
                else if (results) {
                  if (results.length == 0) {
                    // not found, then create
                    ProcessActive = true
                    Log.log('Instance config not found, creating one...')
                    db.collection(ProcessInstancesCollectionName).insertOne({
                      processName: APP_NAME,
                      processInstanceNumber: new mongo.Double(Instance),
                      enabled: true,
                      logLevel: new mongo.Double(1),
                      nodeNames: [],
                      activeNodeName: jsConfig.nodeName,
                      activeNodeKeepAliveTimeTag: new Date()
                    })
                  } else {
                    // check for disabled or node not allowed
                    let instance = results[0]
                    if (instance?.enabled === false) {
                      Log.log('Instance disabled, exiting...')
                      process.exit()
                    }
                    if (
                      instance?.nodeNames !== null &&
                      instance.nodeNames.length > 0
                    ) {
                      if (!instance.nodeNames.includes(jsConfig.nodeName)) {
                        Log.log('Node name not allowed, exiting...')
                        process.exit()
                      }
                    }
                    if (instance?.activeNodeName === jsConfig.nodeName) {
                      if (!ProcessActive) Log.log('Node activated!')
                      countKeepAliveNotUpdated = 0
                      ProcessActive = true
                    } else {
                      // other node active
                      if (ProcessActive) {
                        Log.log('Node deactivated!')
                        countKeepAliveNotUpdated = 0
                      }
                      ProcessActive = false
                      if (
                        lastActiveNodeKeepAliveTimeTag ===
                        instance.activeNodeKeepAliveTimeTag.toISOString()
                      ) {
                        countKeepAliveNotUpdated++
                        Log.log(
                          'Keep-alive from active node not updated. ' +
                            countKeepAliveNotUpdated
                        )
                      } else {
                        countKeepAliveNotUpdated = 0
                        Log.log(
                          'Keep-alive updated by active node. Staying inactive.'
                        )
                      }
                      lastActiveNodeKeepAliveTimeTag = instance.activeNodeKeepAliveTimeTag.toISOString()
                      if (
                        countKeepAliveNotUpdated > countKeepAliveUpdatesLimit
                      ) {
                        // cnt exceeded, be active
                        countKeepAliveNotUpdated = 0
                        Log.log('Node activated!')
                        ProcessActive = true
                      }
                    }

                    if (ProcessActive) {
                      // process active, then update keep alive
                      db.collection(ProcessInstancesCollectionName).updateOne(
                        {
                          processName: APP_NAME,
                          processInstanceNumber: new mongo.Double(Instance)
                        },
                        {
                          $set: {
                            activeNodeName: jsConfig.nodeName,
                            activeNodeKeepAliveTimeTag: new Date(),
                            softwareVersion: VERSION,
                            stats: {
                              latencyAvg: new mongo.Double(
                                latencyAccTotal / latencyTotalCnt
                              ),
                              latencyAvgMinute: new mongo.Double(
                                latencyAccMinute / latencyMinuteCnt
                              ),
                              latencyPeak: new mongo.Double(latencyPeak)
                            }
                          }
                        }
                      )
                    }
                  }
                }
              })
          }

          // check and update redundancy control
          ProcessRedundancy()
          clearInterval(redundancyIntervalHandle)
          redundancyIntervalHandle = setInterval(ProcessRedundancy, 5000)

          // periodically, mark invalid data when supervised points not updated within specified period (invalidDetectTimeout) for the point
          // check also stopped protocol driver instances
          clearInterval(invalidDetectIntervalHandle)
          invalidDetectIntervalHandle = setInterval(function () {
            if (clientMongo !== null) {
              collection.updateMany(
                {
                  $expr: {
                    $and: [
                      { $eq: ['$origin', 'supervised'] },
                      { $eq: ['$invalid', false] },
                      {
                        $lt: [
                          '$sourceDataUpdate.timeTag',
                          {
                            $subtract: [
                              new Date(),
                              { $multiply: [1000, '$invalidDetectTimeout'] }
                            ]
                          }
                        ]
                      }
                    ]
                  }
                },
                { $set: { invalid: true } }
              )

              // look for client drivers instance not updating keep alive, if found invalidate all related data points of all its connections
              db.collection(ProtocolDriverInstancesCollectionName)
                .find({
                  $expr: {
                    $and: [
                      { $in: ['$protocolDriver', ['IEC60870-5-104']] },
                      { $eq: ['$enabled', true] },
                      {
                        $lt: [
                          '$activeNodeKeepAliveTimeTag',
                          {
                            $subtract: [new Date(), { $multiply: [1000, 15] }]
                          }
                        ]
                      }
                    ]
                  }
                })
                .toArray(function (err, results) {
                  if (results)
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
                      db.collection(ProtocolConnectionsCollectionName)
                        .find({
                          protocolDriver: instance?.protocolDriver,
                          protocolDriverInstanceNumber:
                            instance?.protocolDriverInstanceNumber
                        })
                        .toArray(function (err, results) {
                          if (results)
                            for (let i = 0; i < results.length; i++) {
                              let connection = results[i]
                              Log.log(
                                'Data invalidated for connection: ' +
                                  connection?.protocolConnectionNumber
                              )
                              db.collection(
                                RealtimeDataCollectionName
                              ).updateMany(
                                {
                                  origin: 'supervised',
                                  protocolSourceConnectionNumber:
                                    connection?.protocolConnectionNumber,
                                  invalid: false
                                },
                                {
                                  $set: {
                                    invalid: true
                                  }
                                }
                              )
                            }
                        })
                    }
                })
            }
          }, invalidDetectCycle)

          try {
            changeStream.on('error', change => {
              if (clientMongo) clientMongo.close()
              clientMongo = null
              Log.log('Error on ChangeStream!')
            })
            changeStream.on('close', change => {
              if (clientMongo) clientMongo.close()
              clientMongo = null
              Log.log('Closed ChangeStream!')
            })
            changeStream.on('end', change => {
              if (clientMongo) clientMongo.close()
              clientMongo = null
              Log.log('Ended ChangeStream!')
            })

            // start listen to changes
            changeStream.on('change', change => {
              try {
                if (change.operationType === 'delete') return

                let isSOE = false

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
                      "'" +
                      JSON.stringify(change.fullDocument) +
                      "'"
                  )
                }

                if (!ProcessActive)
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
                if (
                  'timeTagAtSource' in
                  change.updateDescription.updatedFields.sourceDataUpdate
                )
                  if (
                    change.updateDescription.updatedFields.sourceDataUpdate
                      .timeTagAtSource !== null
                  )
                    if (change.fullDocument.type === 'digital') {
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
                    .valueStringAtSource
                let alarmed = change.fullDocument.alarmed

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

                  // check for limits
                  if (
                    value != change.fullDocument.value &&
                    !change.fullDocument.alarmDisabled
                  ) {
                    if (
                      change.fullDocument.value <=
                        change.fullDocument.hiLimit &&
                      value >
                        change.fullDocument.hiLimit +
                          change.fullDocument.hysteresis
                    )
                      alarmed = true
                    else if (
                      change.fullDocument.value >=
                        change.fullDocument.loLimit &&
                      value <
                        change.fullDocument.loLimit -
                          change.fullDocument.hysteresis
                    )
                      alarmed = true
                    else if (
                      value <
                      change.fullDocument.hiLimit -
                        change.fullDocument.hysteresis
                    )
                      alarmed = false
                    else if (
                      value >
                      change.fullDocument.loLimit +
                        change.fullDocument.hysteresis
                    )
                      alarmed = false

                    // create a SOE entry for the limits alarm/normalization when analog alarm condition changes
                    if (alarmed != change.fullDocument.alarmed) {
                      let eventDate = new Date()
                      let eventText =
                        value.toFixed(2) +
                        ' ' +
                        change.fullDocument.unit +
                        (alarmed ? ' &#x1F6A9;' : ' &#x1F197;')
                      const coll_soe = db.collection('soeData')
                      coll_soe.insertOne({
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
                        ack: alarmed ? 0 : 1 // enter as acknowledged when normalized
                      })
                    }
                  }
                }

                let alarmTime = null
                if (!change.fullDocument.alarmDisabled)
                  if (alarmed && !change.fullDocument.alarmed) {
                    // changed to alarmed state
                    alarmTime = new Date()
                  }

                // update only if changed or for SOE
                if (
                  isSOE ||
                  value !== change.fullDocument.value ||
                  valueString !== change.fullDocument.valueString ||
                  invalid !== change.fullDocument.invalid
                ) {
                  let dt = new Date()

                  if (!change.fullDocument.alarmDisabled) {
                    if (
                      isSOE ||
                      (alarmed && alarmed !== change.fullDocument.alarmed)
                    ) {
                      // a new alarm, then update beep var
                      Log.log('NEW BEEP', Log.levelDetailed)
                      if (change.fullDocument.priority === 0)
                        // signal an important beep (for alarm of priority 0)
                        mongoRtDataQueue.enqueue({
                          _id: beepPointKey,
                          beepType: new mongo.Double(2), // this is an important beep
                          value: new mongo.Double(1),
                          valueString: 'Beep Active',
                          timeTag: dt
                        })
                      else
                        mongoRtDataQueue.enqueue({
                          _id: beepPointKey,
                          value: new mongo.Double(1),
                          valueString: 'Beep Active',
                          timeTag: dt
                        })
                    }
                    if (change.fullDocument.type === 'digital') {
                      digitalUpdatesCount++
                      mongoRtDataQueue.enqueue({
                        _id: cntUpdatesPointKey,
                        value: new mongo.Double(digitalUpdatesCount),
                        valueString: '' + digitalUpdatesCount + ' Updates',
                        timeTag: dt
                      })
                    }
                  }

                  // historianPeriod<0 excludes from historian
                  let insertIntoHistorian = true
                  if ('historianPeriod' in change.fullDocument) {
                    if (change.fullDocument.historianPeriod < 0)
                      insertIntoHistorian = false
                    else { // historianPeriod >= 0, will test dead band for analogs
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
                    value: new mongo.Double(value),
                    valueString: valueString,
                    ...(change.fullDocument?.type === 'analog' &&
                    insertIntoHistorian
                      ? { historianLastValue: new mongo.Double(value) }
                      : {}),
                    timeTag: dt,
                    overflow: overflow,
                    invalid: invalid,
                    transient: transient,
                    frozen: false,
                    updatesCnt: new mongo.Double(
                      change.fullDocument.updatesCnt + 1
                    ),
                    alarmed: alarmed
                  }
                  if (alarmTime !== null) update.timeTagAlarm = alarmTime

                  // update source time when is SOE and state ON
                  if (
                    (change.fullDocument.isEvent &&
                      isSOE &&
                      change.updateDescription.updatedFields.sourceDataUpdate
                        .timeTagAtSource !== null &&
                      value === 1) ||
                    (!change.fullDocument.isEvent &&
                      isSOE &&
                      change.updateDescription.updatedFields.sourceDataUpdate
                        .timeTagAtSource !== null &&
                      change.fullDocument.value !== value)
                  ) {
                    update.timeTagAtSource =
                      change.updateDescription.updatedFields.sourceDataUpdate.timeTagAtSource
                    update.timeTagAtSourceOk =
                      change.updateDescription.updatedFields.sourceDataUpdate.timeTagAtSourceOk
                  } else {
                    update.timeTagAtSource = null
                    update.timeTagAtSourceOk = null
                  }

                  if (!(change.fullDocument.isEvent && value === 0)) {
                    // do not update protection-like events for state OFF
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
                    let dVal = 0.0
                    if (!isNaN(value))
                      dVal = value

                    // queue data change for postgresql historian
                    let b7 = invalid ? '1' : '0', // value invalid
                      b6 = isSOE
                        ? change.updateDescription.updatedFields
                            .sourceDataUpdate.timeTagAtSourceOk
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
                    sqlHistQueue.enqueue(
                      "'" +
                        change.fullDocument.tag +
                        "'," +
                        "'" +
                        change.updateDescription.updatedFields.sourceDataUpdate.timeTag.toISOString() +
                        "'," +
                        dVal +
                        ',' +
                        '\'{"s": "' +
                        valueString +
                        '"}\',' +
                        (isSOE
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
                        "'"
                    )
                  }

                  // update change.fullDocument with new data just to stringify it and queue update for postgresql update
                  change.fullDocument.value = value
                  change.fullDocument.valueString = valueString
                  change.fullDocument.timeTag = dt
                  change.fullDocument.overflow = overflow
                  change.fullDocument.invalid = invalid
                  change.fullDocument.transient = transient
                  change.fullDocument.updatesCnt =
                    change.fullDocument.updatesCnt + 1
                  change.fullDocument.alarmed = alarmed
                  let queueStr =
                    "'" +
                    change.fullDocument.tag +
                    "'," +
                    "'" +
                    new Date().toISOString() +
                    "'," +
                    "'" +
                    JSON.stringify(change.fullDocument) +
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

                // prepare update to soeData collection
                if (isSOE && !change.fullDocument.alarmDisabled)
                  if (!(value === 0 && change.fullDocument.isEvent)) {
                    let eventText = change.fullDocument.eventTextFalse
                    if (value !== 0) {
                      eventText = change.fullDocument.eventTextTrue
                    }

                    const coll_soe = db.collection('soeData')
                    coll_soe.insertOne({
                      tag: change.fullDocument.tag,
                      pointKey: change.fullDocument._id,
                      group1: change.fullDocument.group1,
                      description: change.fullDocument.description,
                      eventText: eventText,
                      invalid: invalid,
                      priority: change.fullDocument.priority,
                      timeTag: new Date(),
                      timeTagAtSource:
                        change.updateDescription.updatedFields.sourceDataUpdate
                          .timeTagAtSource,
                      timeTagAtSourceOk:
                        change.updateDescription.updatedFields.sourceDataUpdate
                          .timeTagAtSourceOk,
                      ack: 0
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
    await new Promise(resolve => setTimeout(resolve, 5000))

    // detect connection problems, if error will null the client to later reconnect
    if (clientMongo === undefined) {
      Log.log('Disconnected Mongodb!')
      clientMongo = null
    }
    if (clientMongo)
      if (!clientMongo.isConnected()) {
        // not anymore connected, will retry
        Log.log('Disconnected Mongodb!')
        clientMongo.close()
        clientMongo = null
      }
  }
})()
