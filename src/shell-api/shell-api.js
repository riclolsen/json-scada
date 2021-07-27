'use strict'

/*
 * A process that beeps when there are new alarms present on the system.
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

const IP_BIND = process.env.JS_SHELLAPI_IP_BIND || 'localhost'
const HTTP_PORT = process.env.JS_SHELLAPI_HTTP_PORT || 51909
const API_URL = '/htdocs/shellapi.rjs'
const SCREEN_LIST_URL = '/svg/screen_list.js'
const CHECK_PERIOD = 1000

const APP_NAME = 'SHELL-API'
const APP_MSG = '{json:scada} - Shell API'
const VERSION = '0.1.1'
var jsConfigFile = '../../conf/json-scada.json'
const fs = require('fs')
const mongo = require('mongodb')
const MongoClient = require('mongodb').MongoClient
const { setInterval } = require('timers')
const express = require('express')
const app = express()
const beepPointKey = -1
const allowSilenceBeep = true
let beepPresent = false
let beepValue = 0

app.listen(HTTP_PORT, IP_BIND, () => {
  console.log('listening on ' + HTTP_PORT)
})

// Here we serve screen list file
app.get(SCREEN_LIST_URL, function (req, res) {
  res.setHeader('Access-Control-Allow-Origin', '*')
  res.setHeader('Content-Type', 'application/json')
  res.sendFile('../htdocs/svg/screen_list.js')
})

const args = process.argv.slice(2)
var confFile = null
if (args.length > 0) confFile = args[0]
jsConfigFile = confFile || process.env.JS_CONFIG_FILE || jsConfigFile

console.log(APP_MSG + ' Version ' + VERSION)
console.log('Config File: ' + jsConfigFile)

if (!fs.existsSync(jsConfigFile)) {
  console.log('Error: config file not found!')
  process.exit()
}

const RealtimeDataCollectionName = 'realtimeData'
const beepPointKey = -1

let rawFileContents = fs.readFileSync(jsConfigFile)
let jsConfig = JSON.parse(rawFileContents)
if (
  typeof jsConfig.mongoConnectionString != 'string' ||
  jsConfig.mongoConnectionString === ''
) {
  console.log('Error reading config file.')
  process.exit()
}

;(async () => {
  let connOptions = {
    useNewUrlParser: true,
    useUnifiedTopology: true,
    appname: APP_NAME + ' Version:' + VERSION,
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

  let collection = null
  let clientMongo = null
  let checkBeepIntervalHandle = null
  while (true) {
    if (clientMongo === null) {
      console.log('Try to connect to MongoDB server...')
      await MongoClient.connect(jsConfig.mongoConnectionString, connOptions)
        .then(async client => {
          clientMongo = client
          console.log('Connected correctly to MongoDB server')

          // specify db and collections
          const db = client.db(jsConfig.mongoDatabaseName)
          collection = db.collection(RealtimeDataCollectionName)

          // Here we serve beep status and 
          app.get(API_URL, function (req, res) {
            // request to silence alarm beep?
            if (allowSilenceBeep && 'Z' in req.query && req.query.Z == 1) {
              console.log("Silence beep request.")
              beepValue = 0
              collection.updateOne(
                { _id: beepPointKey },
                {
                  $set: {
                    value: new mongo.Double(0),
                    valueString: '0',
                    beepType: new mongo.Double(0)
                  }
                }
              )
            }

            res.setHeader('Access-Control-Allow-Origin', '*')
            res.setHeader('Content-Type', 'application/json')
            res.send({ beep: beepValue })
          })

          clearInterval(checkBeepIntervalHandle)
          let enterQuery = false
          checkBeepIntervalHandle = setInterval(async function () {
            if (enterQuery) return
            if (clientMongo) {
              enterQuery = true
              let data = await collection
                .findOne({ _id: beepPointKey }, { maxTimeMS: CHECK_PERIOD / 2 })
                .catch(function (err) {
                  if (clientMongo) clientMongo.close()
                  beepPresent = false
                  beepValue = 0
                  clientMongo = null
                  console.log('Error on Mongodb query! ' + err)
                  enterQuery = false
                })

              if (data && typeof data._id === 'number') {
                if ('value' in data) {
                  beepValue = data.value
                  beepPresent = data.value === 0 ? false : true
                  console.log('Beep status ' + beepPresent)
                  if (beepPresent && 'beepType' in data) {
                    if (data.beepType >= 2) beepValue = 2 // this signals a more important beep
                  }
                } else {
                  beepPresent = false
                  beepValue = 0
                }
              }
              enterQuery = false
            }
          }, CHECK_PERIOD)
        })
        .catch(function (err) {
          if (clientMongo) clientMongo.close()
          beepPresent = false
          clientMongo = null
          console.log('Connect to MongoDB error!')
        })
    }

    // wait 5 seconds
    await new Promise(resolve => setTimeout(resolve, 5000))

    // detect connection problems, if error will null the client to later reconnect
    if (clientMongo === undefined) {
      console.log('Disconnected Mongodb!')
      beepPresent = false
      clientMongo = null
    }
    if (clientMongo)
      if (!clientMongo.isConnected()) {
        // not anymore connected, will retry
        beepPresent = false
        console.log('Disconnected Mongodb!')
        clientMongo.close()
        clientMongo = null
      }
  }
})()
