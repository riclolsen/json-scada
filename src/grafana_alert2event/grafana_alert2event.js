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

const USERNAME = process.env.JS_ALERT2EVENT_USERNAME || 'grafana'
const PASSWORD = process.env.JS_ALERT2EVENT_PASSWORD || 'grafana'
const ALERTING_MSG = process.env.JS_ALERT2EVENT_ALERTING_MSG || 'alerting'
const OK_MSG = process.env.JS_ALERT2EVENT_OK_MSG || 'ok'

const IP_BIND = process.env.JS_ALERT2EVENT_IP_BIND || '127.0.0.1'
const HTTP_PORT = process.env.JS_ALERT2EVENT_HTTP_PORT || 51910
const API_URL = '/grafana_alert2event'

const APP_NAME = 'GRAFANA_ALERT2EVENT'
const APP_MSG = '{json:scada} - Grafana Alert To Event Listener'
const VERSION = '0.1.0'

const RealtimeDataCollectionName = 'realtimeData'
const SoeCollectionName = 'soeData'

var jsConfigFile = '../../conf/json-scada.json'
const Queue = require('queue-fifo')
const fs = require('fs')
const mongo = require('mongodb')
const MongoClient = require('mongodb').MongoClient
let Server = require('mongodb')
const { setInterval } = require('timers')
const express = require('express')
const app = express()
app.use(express.json())
let soeQueue = new Queue() // queue of SOE events

app.listen(HTTP_PORT, IP_BIND, () => {
  console.log('listening on ' + IP_BIND + ':' + HTTP_PORT)
})

async function basicAuth (req, res, next) {
  // make authenticate path public
  if (req.path === '/users/authenticate') {
    return next()
  }

  // check for basic auth header
  if (
    !req.headers.authorization ||
    req.headers.authorization.indexOf('Basic ') === -1
  ) {
    return res.status(401).json({ message: 'Missing Authorization Header' })
  }

  // verify auth credentials
  const base64Credentials = req.headers.authorization.split(' ')[1]
  const credentials = Buffer.from(base64Credentials, 'base64').toString('ascii')
  const [username, password] = credentials.split(':')
  const user = await validateCredentials({ username, password })
  if (!user) {
    // console.log("Invalid user credentials!")
    return res
      .status(401)
      .json({ message: 'Invalid Authentication Credentials' })
  }

  // console.log("Authorized user.")

  // attach user to request object
  req.user = user
  next()
}

async function validateCredentials (cred) {
  if (cred.username === USERNAME && cred.password === PASSWORD)
    return { username: cred.username }
  return false
}

app.use(basicAuth)

// API access point for Grafana webhook alert channel
app.post(API_URL, function (req, res) {
  res.setHeader('Access-Control-Allow-Origin', '*')
  res.setHeader('Content-Type', 'application/json')

  console.log(req.body)

  if (
    !['ok', 'alerting'].includes(req.body?.state) ||
    req.body?.tags?.event === '0'
  ) {
    console.log('Discard event.')
    res.send({ ok: true })
    return
  }

  let timeStamp = new Date()
  let tag =
    req.body?.tags?.tag || req.body?.tags?.evalMatches[0]?.metric || 'NO_TAG'
  let priority = parseFloat(req.body?.tags?.priority || 3)
  let group1 = req.body?.tags?.group1 || 'Grafana'
  let pointKey = parseFloat(req.body?.tags?.pointKey || 0)
  let description = ''
  switch (req.body?.message) {
    case 'ok':
      description = OK_MSG
      break
    case 'alerting':
      description = ALERTING_MSG
      break
  }

  let SOE_Event = {
    tag: tag,
    pointKey: pointKey,
    group1: group1,
    description: req.body?.message,
    eventText: req.body?.state,
    invalid: false,
    priority: priority,
    timeTag: timeStamp,
    timeTagAtSource: timeStamp,
    timeTagAtSourceOk: false,
    ack: 0,
    source: req.body?.ruleUrl
  }
  soeQueue.enqueue(SOE_Event)
  console.log(SOE_Event)

  res.send({ ok: true })
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
    readPreference: Server.READ_PRIMARY
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
  let checkSoeQueueIntervalHandle = null
  while (true) {
    if (clientMongo === null) {
      console.log('Try to connect to MongoDB server...')
      await MongoClient.connect(jsConfig.mongoConnectionString, connOptions)
        .then(async client => {
          clientMongo = client
          console.log('Connected correctly to MongoDB server')

          // check for event queue each 1s, insert into mongo when dequeued
          clearInterval(checkSoeQueueIntervalHandle)
          checkSoeQueueIntervalHandle = setInterval(function () {
            if (clientMongo) {
              while (!soeQueue.isEmpty()) {
                let event = soeQueue.peek()
                soeQueue.dequeue()
                const coll_soe = db.collection(SoeCollectionName)
                coll_soe.insertOne(event)
              }
            }
          }, 1000)

          // specify db and collections
          const db = client.db(jsConfig.mongoDatabaseName)
          collection = db.collection(RealtimeDataCollectionName)
        })
        .catch(function (err) {
          if (clientMongo) clientMongo.close()
          clientMongo = null
          console.log('Connect to MongoDB error!')
        })
    }

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
