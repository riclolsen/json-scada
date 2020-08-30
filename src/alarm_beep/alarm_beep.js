'use strict'

/* 
 * A process that beeps when there are new alarms present on the system.
 * {json:scada} - Copyright (c) 2020 - Ricardo L. Olsen
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

const BEEP_PERIOD = 3000
const CHECK_PERIOD = 2000

const APP_NAME = 'ALARM_BEEP'
const APP_MSG = '{json:scada} - Alarm Beep'
const VERSION = '0.1.0'
var jsConfigFile = '../../conf/json-scada.json'
const fs = require('fs')
const mongo = require('mongodb')
const MongoClient = require('mongodb').MongoClient
let Server = require('mongodb').Server
const { setInterval } = require('timers')
const {beep}=require('a1-beep')
let sys = require('child_process')

function Beep(...args) {
  if (process.platform === "win32")
    sys.exec(`rundll32 user32.dll,MessageBeep`)
  else {
    // using beepbeep package
    beep(...args)
    // alternative method
    sys.spawn('/usr/bin/aplay -q -D default /usr/share/sounds/linuxmint-gdm.wav')
  }
}

Beep()

const args = process.argv.slice(2)
var confFile = null
if (args.length > 0)
  confFile = args[0]
jsConfigFile = confFile || process.env.JS_CONFIG_FILE || jsConfigFile;

console.log(APP_MSG + " Version " + VERSION)
console.log("Config File: " + jsConfigFile)

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

; (async () => {

  let connOptions = {
    useNewUrlParser: true,
    useUnifiedTopology: true,
    appname: APP_NAME + " Version:" + VERSION,
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

  let collection = null
  let clientMongo = null
  let doBeepIntervalHandle = null
  let checkBeepIntervalHandle = null
  let beepPresent = false
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

          clearInterval(doBeepIntervalHandle)
          doBeepIntervalHandle = setInterval(function () {
            if (clientMongo) {
              if (beepPresent) {
                console.log("Beep!")
                Beep()
              }
            }
          }, BEEP_PERIOD)

          clearInterval(checkBeepIntervalHandle)
          let enterQuery = false
          checkBeepIntervalHandle = setInterval(async function () {
            if (enterQuery)
              return
            if (clientMongo) {
              enterQuery = true
              let data = await collection.findOne({ _id: beepPointKey }, { maxTimeMS: CHECK_PERIOD / 2 })
                .catch(function (err) {
                  if (clientMongo) clientMongo.close()
                  beepPresent = false
                  clientMongo = null
                  console.log('Error on Mongodb query!')
                  enterQuery = false
                })

              if (data && typeof data._id === 'number') {
                if ("value" in data) {
                  beepPresent = data.value === 0 ? false : true;
                  console.log("Beep status " + beepPresent)
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
          console.log("Connect to MongoDB error!")
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
