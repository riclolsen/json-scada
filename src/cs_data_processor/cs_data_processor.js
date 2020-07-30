'use strict'

// A process that watches for raw data updates from protocols using a MongoDB change stream.
// Convert raw values and update realtime values and statuses.
// JSON SCADA - Copyright 2020 - Ricardo L. Olsen

const APP_NAME = 'cs_data_processor.js'
const jsConfigFile = '../../conf/json-scada.json'
const sqlFilesPath = '../../sql/'
const fs = require('fs')
const mongo = require('mongodb')
const MongoClient = require('mongodb').MongoClient
let Server = require('mongodb').Server
const Queue = require('queue-fifo')

const RealtimeDataCollectionName = 'realtimeData'
const beepPointKey = -1
const cntUpdatesPointKey = -2

let rawFileContents = fs.readFileSync(jsConfigFile)
let jsConfig = JSON.parse(rawFileContents)
if (
  typeof jsConfig.mongoConnectionString != 'string' ||
  jsConfig.mongoConnectionString === ''
) {
  console.log('Error reading config file.')
  process.exit()
}

console.log('Connecting to ' + jsConfig.mongoConnectionString)

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

/*
const pipelineRtData = [
  {
    $project: { documentKey: false }
  },
  {
    $match: {
      $and: [
        { "fullDocument.value": { $exists: true } },
        { "updateDescription.updatedFields.sourceDataUpdate": { $exists: false } },
        {
          $or: [
            { operationType: "replace" },
            { operationType: "delete" },
            { operationType: "update" },
            { operationType: "insert" }
          ]
        }
      ]
    }
  }
];
*/

;(async () => {
  let collection = null
  let sqlHistQueue = new Queue() // queue of historical values to insert on postgreSQL
  let sqlRtDataQueue = new Queue() // queue of realtime values to insert on postgreSQL
  let sqlRtDataQueueMongo = new Queue() // queue of realtime values to insert on MongoDB
  let digitalUpdatesCount = 0

  setInterval(async function () {
    let cnt = 0
    if (collection)
      while (!sqlRtDataQueueMongo.isEmpty()) {
        let upd = sqlRtDataQueueMongo.peek()
        let where = { _id: upd._id }
        delete upd._id // remove _id for update
        collection.updateOne(where, {
          $set: upd
        })
        sqlRtDataQueueMongo.dequeue()
        cnt++
      }
    if (cnt) console.log('Mongo Updates ' + cnt)
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
    if (cntH) console.log('PGSQL Hist updates ' + cntH)

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
          if (err) console.log('Error writing SQL file!')
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
    if (cntR) console.log('PGSQL RT updates ' + cntR)

    if (doInsertData) {
      sqlTransaction = sqlTransaction + 'COMMIT;\n'
      fs.writeFile(
        sqlFilesPath + 'pg_rtdata_' + new Date().getTime() + '.sql',
        sqlTransaction,
        err => {
          if (err) console.log('Error writing SQL file!')
        }
      )
    }
  }, 1000)

  let connOptions = {
    useNewUrlParser: true,
    useUnifiedTopology: true,
    appname: APP_NAME,
    poolSize: 20,
    readPreference: Server.READ_PRIMARY
  }

  if (typeof jsConfig.tlsCaPemFile === 'string' && jsConfig.tlsCaPemFile.trim() !== '') {
    jsConfig.tlsClientKeyPassword = jsConfig.tlsClientKeyPassword || ""
    jsConfig.tlsAllowInvalidHostnames = jsConfig.tlsAllowInvalidHostnames || false
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
  while (true) {
    if (clientMongo === null)
      await MongoClient.connect(jsConfig.mongoConnectionString, connOptions)
        .then(async client => {
          clientMongo = client
          console.log('Connected correctly to MongoDB server')

          // specify db and collections
          const db = client.db(jsConfig.mongoDatabaseName)
          collection = db.collection(RealtimeDataCollectionName)
          const changeStream = collection.watch(pipeline, {
            fullDocument: 'updateLookup'
          })

          try {
            changeStream.on('error', change => {
              if (clientMongo) clientMongo.close()
              clientMongo = null
              console.log('Error on ChangeStream!')
            })
            changeStream.on('close', change => {
              if (clientMongo) clientMongo.close()
              clientMongo = null
              console.log('Closed ChangeStream!')
            })
            changeStream.on('end', change => {
              if (clientMongo) clientMongo.close()
              clientMongo = null
              console.log('Ended ChangeStream!')
            })

            // start listen to changes
            changeStream.on('change', change => {
              try {
                if (change.operationType === 'delete') return

                let isSOE = false

                if (change.operationType === 'insert') {
                  // document inserted
                  console.log(
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

                if (
                  !(
                    'sourceDataUpdate' in change.updateDescription.updatedFields
                  )
                )
                  // Source Data Update (protocol update)
                  return

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

                  value =
                    change.updateDescription.updatedFields.sourceDataUpdate
                      .valueAtSource *
                      change.fullDocument.kconv1 +
                    change.fullDocument.kconv2
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
                      console.log('NEW BEEP-------------------')
                      sqlRtDataQueueMongo.enqueue({
                        _id: beepPointKey,
                        value: new mongo.Double(1),
                        valueString: 'Beep Active',
                        timeTag: dt
                      })
                    }
                    if (change.fullDocument.type === 'digital') {
                      digitalUpdatesCount++
                      sqlRtDataQueueMongo.enqueue({
                        _id: cntUpdatesPointKey,
                        value: new mongo.Double(digitalUpdatesCount),
                        valueString: '' + digitalUpdatesCount + ' Updates',
                        timeTag: dt
                      })
                    }
                  }

                  let update = {
                    _id: change.fullDocument._id,
                    value: new mongo.Double(value),
                    valueString: valueString,
                    timeTag: dt,
                    overflow: overflow,
                    invalid: invalid,
                    transient: transient,
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
                    sqlRtDataQueueMongo.enqueue(update)

                    console.log(
                      'UPD ' +
                        change.fullDocument._id +
                        ' ' +
                        change.fullDocument.tag +
                        ' ' +
                        value +
                        ' DELAY ' +
                        (new Date().getTime() -
                          change.updateDescription.updatedFields.sourceDataUpdate.timeTag.getTime()) +
                        'ms'
                    )
                  }

                  // queue data change for postgresql historian
                  let b7 = invalid ? '1' : '0', // value invalid
                    b6 = isSOE
                      ? change.updateDescription.updatedFields.sourceDataUpdate
                          .timeTagAtSourceOk
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
                      value +
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
                  console.log(
                    'Not changed ' +
                      change.fullDocument.tag +
                      ' DELAY ' +
                      (new Date().getTime() -
                        change.updateDescription.updatedFields.sourceDataUpdate.timeTag.getTime()) +
                      'ms'
                  )

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
                    console.log(
                      'SOE ' +
                        change.fullDocument._id +
                        ' ' +
                        change.fullDocument.tag +
                        ' ' +
                        change.updateDescription.updatedFields.sourceDataUpdate
                          .valueAtSource +
                        ' ' +
                        change.updateDescription.updatedFields.sourceDataUpdate
                          .timeTagAtSource
                    )
                  }
              } catch (e) {
                console.log(e)
              }
            })
          } catch (e) {
            console.log(e)
          }
        })
        .catch(function (err) {
          if (clientMongo) clientMongo.close()
          clientMongo = null
          console.log(err)
        })

    // wait 5 seconds
    await new Promise(resolve => setTimeout(resolve, 5000))

    // detect connection problems, if error will null the client to later reconnect
    if (clientMongo === undefined) {
      console.log('Disconnected Mongodb!')
      clientMongo = null
    }
    if (clientMongo)
      if (!clientMongo.isConnected()) {
        // not anymore connected, will retry
        console.log('Disconnected Mongodb!')
        clientMongo.close()
        clientMongo = null
      }
  }
})()
