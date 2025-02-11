/*
 * A realtime point data HTTP web server for JSON SCADA.
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

let AUTHENTICATION = process.env.JS_AUTHENTICATION === 'NOAUTH' ? false : true
const IP_BIND = process.env.JS_IP_BIND || '127.0.0.1'
const HTTP_PORT = process.env.JS_HTTP_PORT || 8080
const GRAFANA_SERVER = process.env.JS_GRAFANA_SERVER || 'http://127.0.0.1:3000'
const LOGIO_SERVER = process.env.JS_LOGIO_SERVER || 'http://127.0.0.1:6688'
const METABASE_SERVER =
  process.env.JS_METABASE_SERVER || 'http://127.0.0.1:3001'
const OPCAPI_AP = '/Invoke/' // mimic of webhmi from OPC reference app https://github.com/OPCFoundation/UA-.NETStandard/tree/demo/webapi/SampleApplications/Workshop/Reference
const GETFILE_AP = '/GetFile' // API Access point for requesting mongodb files (gridfs)
const QUERYJSON_AP = '/queryJSON' // API Access point for special custom queries returning JSON
const COLL_REALTIME = 'realtimeData'
const COLL_SOE = 'soeData'
const COLL_COMMANDS = 'commandsQueue'
const COLL_ACTIONS = 'userActions'
const LoadConfig = require('./load-config')
const Log = require('./simple-logger')
const express = require('express')
const fileUpload = require('express-fileupload')
const httpProxy = require('express-http-proxy')
const {
  legacyCreateProxyMiddleware: createProxyMiddleware,
} = require('http-proxy-middleware')
const path = require('path')
const cors = require('cors')
const app = express()
const cookieParser = require('cookie-parser')
const { MongoClient, ObjectId, Double, GridFSBucket } = require('mongodb')
const opc = require('./opc_codes.js')
const { Pool } = require('pg')
const UserActionsQueue = require('./userActionsQueue')
const GetQueryPostgresql = require('./customJsonQueries')
const initGQLServer = require('./graphql-server.js')

const config = require('./app/config/auth.config.js')
if (process.env.JS_JWT_SECRET) config.secret = process.env.JS_JWT_SECRET
if (process.env.JS_LDAP_ENABLED) config.ldap.enabled = process.env.JS_LDAP_ENABLED.toLowerCase() == 'true'
if (process.env.JS_LDAP_URL) config.ldap.url = process.env.JS_LDAP_URL
if (process.env.JS_LDAP_BIND_DN) config.ldap.bindDN = process.env.JS_LDAP_BIND_DN
if (process.env.JS_LDAP_BIND_CREDENTIALS) config.ldap.bindCredentials = process.env.JS_LDAP_BIND_CREDENTIALS
if (process.env.JS_LDAP_SEARCH_BASE) config.ldap.searchBase = process.env.JS_LDAP_SEARCH_BASE
if (process.env.JS_LDAP_SEARCH_FILTER) config.ldap.searchFilter = process.env.JS_LDAP_SEARCH_FILTER
if (process.env.JS_LDAP_ATTRIBUTES_USERNAME) config.ldap.attributes.username = process.env.JS_LDAP_ATTRIBUTES_USERNAME
if (process.env.JS_LDAP_ATTRIBUTES_EMAIL) config.ldap.attributes.email = process.env.JS_LDAP_ATTRIBUTES_EMAIL
if (process.env.JS_LDAP_ATTRIBUTES_DISPLAYNAME) config.ldap.attributes.displayName = process.env.JS_LDAP_ATTRIBUTES_DISPLAYNAME
if (process.env.JS_LDAP_GROUP_SEARCH_BASE) config.ldap.groupSearchBase = process.env.JS_LDAP_GROUP_SEARCH_BASE
if (process.env.JS_LDAP_TLS_REJECT_UNAUTHORIZED) config.ldap.tlsOptions.rejectUnauthorized = process.env.JS_LDAP_TLS_REJECT_UNAUTHORIZED.toLowerCase() == 'true'
if (process.env.JS_LDAP_TLS_CA) config.ldap.tlsOptions.ca = [process.env.JS_LDAP_TLS_CA]
if (process.env.JS_LDAP_TLS_CERT) config.ldap.tlsOptions.cert = process.env.JS_LDAP_TLS_CERT
if (process.env.JS_LDAP_TLS_KEY) config.ldap.tlsOptions.key = process.env.JS_LDAP_TLS_KEY
if (process.env.JS_LDAP_TLS_PASSPHRASE) config.ldap.tlsOptions.passphrase  = process.env.JS_LDAP_TLS_PASSPHRASE
if (process.env.JS_LDAP_TLS_PFX) config.ldap.tlsOptions.pfx  = process.env.JS_LDAP_TLS_PFX
if (process.env.JS_LDAP_TLS_CRL) config.ldap.tlsOptions.crl  = process.env.JS_LDAP_TLS_CRL
if (process.env.JS_LDAP_TLS_CIPHERS) config.ldap.tlsOptions.ciphers  = process.env.JS_LDAP_TLS_CIPHERS
if (process.env.JS_LDAP_TLS_SECURE_PROTOCOL) config.ldap.tlsOptions.secureProtocol  = process.env.JS_LDAP_TLS_SECURE_PROTOCOL
if (process.env.JS_LDAP_TLS_MIN_VERSION) config.ldap.tlsOptions.minVersion  = process.env.JS_LDAP_TLS_MIN_VERSION
if (process.env.JS_LDAP_TLS_MAX_VERSION) config.ldap.tlsOptions.maxVersion  = process.env.JS_LDAP_TLS_MAX_VERSION

Log.log('LDAP enabled: ' + config.ldap.enabled)
if (config.ldap.enabled && !config.ldap.url.startsWith('ldaps')) {
  Log.log('LDAP authentication - Not Encrypted!')  
}

const dbAuth = require('./app/models')
const { authJwt } = require('./app/middlewares')
const { canSendCommands } = require('./app/middlewares/authJwt.js')

process.on('uncaughtException', err => Log.log('Uncaught Exception:' + JSON.stringify(err)))

// Argument NOAUTH disables user authentication
var args = process.argv.slice(2)
if (args.length > 0) if (args[0] === 'NOAUTH') AUTHENTICATION = false

const DoInsertCommandAsSOE = true
const CommandSentAsSOESymbol = '⚙️➡️'
const opcIdTypeNumber = 0
const opcIdTypeString = 1
const beepPointKey = -1
const EventsRemoveGuardSeconds = 20

const jsConfig = LoadConfig()
let HintMongoIsConnected = true

let db = null
let clientMongo = null
let pool = null

;(async () => {
  if (AUTHENTICATION) {
    // JWT Auth Mongo Express https://bezkoder.com/node-js-mongodb-auth-jwt/
    dbAuth.mongoose
      .connect(jsConfig.mongoConnectionString, jsConfig.MongoConnectionOptions)
      .then(() => {
        Log.log('Successfully connect to MongoDB (via mongoose).')
        // initial();
      })
      .catch((err) => {
        console.error('Connection error', err)
        process.exit()
      })

    app.use(cookieParser())
    app.options(OPCAPI_AP, cors()) // enable pre-flight request
    // enable files upload
    app.use(
      fileUpload({
        createParentPath: true,
      })
    )
    app.use(express.json())
    app.use(
      express.urlencoded({
        extended: true,
      })
    )
    initGQLServer(app, dbAuth)
  } else {
    Log.log('******* DISABLED AUTHENTICATION! ********')

    // reverse proxy for grafana
    app.use('/grafana', httpProxy(GRAFANA_SERVER))
    // reverse proxy for grafana
    app.use('/metabase', httpProxy(METABASE_SERVER))
    // reverse proxy for log.io
    app.use('/log-io', httpProxy(LOGIO_SERVER))
    const wsProxy = createProxyMiddleware({
      target: LOGIO_SERVER,
      changeOrigin: true,
      ws: true, // enable websocket proxy
    })
    app.use('/socket.io', wsProxy)
    app.on('upgrade', wsProxy.upgrade)

    app.use('/svg', [authJwt.verifyToken], express.static('../../svg'))

    // production
    app.use('/', express.static('../AdminUI/dist'))
    app.use('/login', express.static('../AdminUI/dist'))
    app.use('/dashboard', express.static('../AdminUI/dist'))
    app.use('/admin', express.static('../AdminUI/dist'))

    // add charset for special sage displays
    app.use(
      '/sage-cepel-displays/',
      express.static('../AdminUI/dist/sage-cepel-displays', {
        setHeaders: function (res, path) {
          if (/.*\.html/.test(path)) {
            res.set({ 'content-type': 'text/html; charset=iso-8859-1' })
          }
        },
      })
    )
    app.options(OPCAPI_AP, cors()) // enable pre-flight request
    app.use(express.json())
    app.use(
      express.urlencoded({
        extended: true,
      })
    )
  }

  app.listen(HTTP_PORT, IP_BIND, () => {
    Log.log('listening on ' + HTTP_PORT)
  })

  // if env variables defined use them, if not set local defaults
  let pgopt = {}
  if ('PGHOST' in process.env || 'PGHOSTADDR' in process.env) pgopt = null
  else
    pgopt = {
      host: '127.0.0.1',
      database: 'json_scada',
      user: 'json_scada',
      password: 'json_scada',
      port: 5432,
    }

  if (pool == null) {
    Log.log('Postgresql - Trying to connect')
    pool = new Pool(pgopt)
    pool.on('connect', (client) => {
      Log.log('Postgresql - connection open')
    })
    pool.on('error', (err, client) => {
      Log.log('Postgresql - ' + err)
      setTimeout(() => {
        pool = null
      }, 5000)
    })
    pool.connect((err, client, done) => {
      if (err) {
        Log.log('Postgresql - connection error: ' + err)
        pool = null
        done()
      }
    })
  }

  if (AUTHENTICATION) {
    require('./app/routes/auth.routes')(app, OPCAPI_AP)
    require('./app/routes/user.routes')(
      app,
      OPCAPI_AP,
      opcApi,
      GETFILE_AP,
      getFileApi,
      GRAFANA_SERVER,
      QUERYJSON_AP,
      queryJSON,
      LOGIO_SERVER,
      METABASE_SERVER
    )
  } else {
    app.post(OPCAPI_AP, opcApi)
    app.get(GETFILE_AP, getFileApi)
    app.get(QUERYJSON_AP, queryJSON)
  }

  async function queryJSON(req, res) {
    let queryname = req.query?.query || ''
    Log.log('queryJSON ' + queryname)
    res.setHeader('Content-type', 'application/json')

    if (queryname === '') {
      res.send({ error: 'Missing "query" parameter!' })
      return
    }

    let query = GetQueryPostgresql(queryname)
    if (query === '') {
      res.send({ error: 'Unknown query name!' })
      return
    }

    // read data from postgreSQL
    pool.query(query, (err, resp) => {
      if (err) {
        res.send({ error: 'Query error!' })
        return
      }

      res.send(resp.rows)
      return
    })
  }

  // find file on mongodb gridfs and return it
  async function getFileApi(req, res) {
    let filename = req.query?.name || ''
    let bucketName = req.query?.bucket || 'fs'
    let mimeType = req.query?.mime || path.basename(filename)
    let refresh = req.query.refresh || 0

    if (filename.trim() === '') {
      res.setHeader('Content-type', 'application/json')
      res.send("{ error: 'Parameter [name] empty or not specified' }")
    }
    try {
      let gfs = new GridFSBucket(db, { bucketName: bucketName })
      let f = await gfs.find({ filename: filename }).toArray()
      if (f.length === 0) {
        Log.log('File not found ' + filename)
        res.setHeader('Content-type', 'application/json')
        res.send("{ error: 'File not found' }")
        return
      }
      let readstream = gfs.openDownloadStreamByName(filename)
      res.type(mimeType)
      res.setHeader(
        'Content-disposition',
        'inline; filename="' + filename + '"'
      )
      if (refresh) res.setHeader('Refresh', refresh)
      readstream.pipe(res)
    } catch (e) {
      Log.log('File not found: ' + filename + ' ' + e.message)
      res.setHeader('Content-type', 'application/json')
      res.send("{ error: 'File not found' }")
    }
  }

  // OPC WEB HMI API
  async function opcApi(req, res) {
    let tini = new Date().getTime()

    res.setHeader('Access-Control-Allow-Origin', '*')
    res.setHeader('Content-Type', 'application/json')

    let ServiceId,
      RequestHandle,
      Timestamp = new Date().toISOString()

    if ('ServiceId' in req.body) {
      ServiceId = req.body.ServiceId
      if ('Body' in req.body) {
        if ('RequestHeader' in req.body.Body) {
          if ('RequestHandle' in req.body.Body.RequestHeader) {
            RequestHandle = req.body.Body.RequestHeader.RequestHandle
          }
        }
      }
    }

    let OpcResp = {
      NamespaceUris: [
        'urn:opcf-apps-01:UA:Quickstarts:ReferenceServer',
        'http://opcfoundation.org/Quickstarts/ReferenceApplications',
        'http://opcfoundation.org/UA/Diagnostics',
      ],
      ServerUris: [],
      ServiceId: ServiceId,
      Body: {
        ResponseHeader: {
          RequestHandle: RequestHandle,
          Timestamp: Timestamp,
          ServiceDiagnostics: {
            LocalizedText: 0,
          },
          StringTable: [],
        },
        //, "DiagnosticInfos": []
      },
    }

    let username = 'unknown'
    let userRights = {}

    // handle auth here
    if (AUTHENTICATION) {
      let rslt = authJwt.checkToken(req)
      // Log.log(rslt)
      if (rslt === false) {
        // fail if not connected to database server
        OpcResp.ServiceId = opc.ServiceCode.ServiceFault
        OpcResp.Body.ResponseHeader.ServiceResult =
          opc.StatusCode.BadIdentityTokenRejected
        OpcResp.Body.ResponseHeader.StringTable = [
          opc.getStatusCodeName(opc.StatusCode.BadIdentityTokenRejected),
          opc.getStatusCodeText(opc.StatusCode.BadIdentityTokenRejected),
          'Access denied (absent or invalid access token)!',
        ]
        res.send(OpcResp)
        return
      } else {
        if ('username' in rslt) username = rslt.username
        if ('rights' in rslt) userRights = rslt.rights
      }
    }

    if (!clientMongo || !HintMongoIsConnected) {
      // fail if not connected to database server
      OpcResp.ServiceId = opc.ServiceCode.ServiceFault
      OpcResp.Body.ResponseHeader.ServiceResult =
        opc.StatusCode.BadServerNotConnected
      OpcResp.Body.ResponseHeader.StringTable = [
        opc.getStatusCodeName(opc.StatusCode.BadServerNotConnected),
        opc.getStatusCodeText(opc.StatusCode.BadServerNotConnected),
        'Database disconnected!',
      ]
      res.send(OpcResp)
      return
    }

    if (ServiceId === undefined) {
      // fail if no service id defined
      OpcResp.ServiceId = opc.ServiceCode.ServiceFault
      OpcResp.Body.ResponseHeader.ServiceResult =
        opc.StatusCode.BadRequestHeaderInvalid
      OpcResp.Body.ResponseHeader.StringTable = [
        opc.getStatusCodeName(opc.StatusCode.BadRequestHeaderInvalid),
        opc.getStatusCodeText(opc.StatusCode.BadRequestHeaderInvalid),
        'No ServiceID',
      ]
      res.send(OpcResp)
      return
    }

    if (RequestHandle === undefined) {
      // fail if not defined a request handle
      OpcResp.ServiceId = opc.ServiceCode.ServiceFault
      OpcResp.Body.ResponseHeader.ServiceResult =
        opc.StatusCode.BadRequestHeaderInvalid
      OpcResp.Body.ResponseHeader.StringTable = [
        opc.getStatusCodeName(opc.StatusCode.BadRequestHeaderInvalid),
        opc.getStatusCodeText(opc.StatusCode.BadRequestHeaderInvalid),
        'No RequestHandle',
      ]
      res.send(OpcResp)
      return
    }

    switch (req.body.ServiceId) {
      case opc.ServiceCode.WriteRequest: // WRITE SERVICE
        {
          OpcResp.ServiceId = opc.ServiceCode.WriteResponse
          if (!('NodesToWrite' in req.body.Body)) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.GoodNoData
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.GoodNoData),
              opc.getStatusCodeText(opc.StatusCode.GoodNoData),
              'No NodesToWrite',
            ]
            res.send(OpcResp)
            return
          }
          OpcResp.Body.ResponseHeader.ServiceResult = opc.StatusCode.Good
          OpcResp.Body.Results = []
          OpcResp.Body._CommandHandles = []
          for (let i = 0; i < req.body.Body.NodesToWrite.length; i++) {
            let node = req.body.Body.NodesToWrite[i]
            if ('AttributeId' in node) {
              if (node.AttributeId == opc.AttributeId.ExtendedAlarmEventsAck) {
                // alarm / event ack

                if (AUTHENTICATION) {
                  // test for user rights
                  if (
                    !userRights?.ackEvents &&
                    (node.Value.Body & opc.Acknowledge.RemoveAllEvents ||
                      node.Value.Body & opc.Acknowledge.RemoveOneEvent ||
                      node.Value.Body & opc.Acknowledge.RemovePointEvents ||
                      node.Value.Body & opc.Acknowledge.AckAllEvents ||
                      node.Value.Body & opc.Acknowledge.AckOneEvent ||
                      node.Value.Body & opc.Acknowledge.AckPointEvents)
                  ) {
                    // ACTION DENIED
                    Log.log(
                      `User has no right to ack/remove events! [${username}]`
                    )
                    OpcResp.Body.ResponseHeader.ServiceResult =
                      opc.StatusCode.BadUserAccessDenied
                    OpcResp.Body.ResponseHeader.StringTable = [
                      opc.getStatusCodeName(opc.StatusCode.BadUserAccessDenied),
                      opc.getStatusCodeText(opc.StatusCode.BadUserAccessDenied),
                      'User has no right to ack/remove events!',
                    ]
                    res.send(OpcResp)
                    return
                  }
                  if (
                    !userRights?.ackAlarms &&
                    (node.Value.Body & opc.Acknowledge.AckOneAlarm ||
                      node.Value.Body & opc.Acknowledge.AckAllAlarms ||
                      node.Value.Body & opc.Acknowledge.SilenceBeep)
                  ) {
                    // ACTION DENIED
                    Log.log(
                      `User has no right to ack or silence alarms! [${username}]`
                    )
                    OpcResp.Body.ResponseHeader.ServiceResult =
                      opc.StatusCode.BadUserAccessDenied
                    OpcResp.Body.ResponseHeader.StringTable = [
                      opc.getStatusCodeName(opc.StatusCode.BadUserAccessDenied),
                      opc.getStatusCodeText(opc.StatusCode.BadUserAccessDenied),
                      'User has no right to ack or silence alarms!',
                    ]
                    res.send(OpcResp)
                    return
                  }
                }

                let findPoint = null
                if (node.NodeId.IdType === opcIdTypeNumber) {
                  findPoint = {
                    _id: parseInt(node.NodeId.Id),
                    ...(!AUTHENTICATION || userRights?.group1List?.length == 0
                      ? {}
                      : { group1: { $in: userRights.group1List } }),
                  }
                } else if (node.NodeId.IdType === opcIdTypeString) {
                  findPoint = {
                    tag: node.NodeId.Id,
                    ...(!AUTHENTICATION || userRights?.group1List?.length == 0
                      ? {}
                      : { group1: { $in: userRights.group1List } }),
                  }
                }

                if (node.Value.Body & opc.Acknowledge.RemoveAllEvents) {
                  Log.log('Remove All Events')
                  let fromDate = new Date(
                    Date.now() - EventsRemoveGuardSeconds * 1000
                  )
                  let result = await db.collection(COLL_SOE).updateMany(
                    {
                      ack: { $lte: 1 },
                      timeTag: { $lte: fromDate },
                      ...(!AUTHENTICATION || userRights?.group1List?.length == 0
                        ? {}
                        : { group1: { $in: userRights.group1List } }),
                    },
                    {
                      $set: {
                        ack: 2,
                      },
                    }
                  )
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Remove All Events',
                    timeTag: fromDate,
                  })
                } else if (node.Value.Body & opc.Acknowledge.AckAllEvents) {
                  Log.log('Ack All Events')
                  let result = await db.collection(COLL_SOE).updateMany(
                    {
                      ack: 0,
                      ...(!AUTHENTICATION || userRights?.group1List?.length == 0
                        ? {}
                        : { group1: { $in: userRights.group1List } }),
                    },
                    {
                      $set: {
                        ack: 1,
                      },
                    }
                  )
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Ack All Events',
                    timeTag: new Date(),
                  })
                } else if (
                  node.Value.Body & opc.Acknowledge.RemovePointEvents
                ) {
                  Log.log('Remove Point Events: ' + node.NodeId.Id)
                  let fromDate = new Date(
                    Date.now() - EventsRemoveGuardSeconds * 1000
                  )
                  let result = await db.collection(COLL_SOE).updateMany(
                    {
                      tag: node.NodeId.Id,
                      ack: { $lte: 1 },
                      timeTag: { $lte: fromDate },
                      ...(!AUTHENTICATION || userRights?.group1List?.length == 0
                        ? {}
                        : { group1: { $in: userRights.group1List } }),
                    },
                    {
                      $set: {
                        ack: 2,
                      },
                    }
                  )
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Remove Point Events',
                    tag: node.NodeId.Id,
                    timeTag: fromDate,
                  })
                } else if (node.Value.Body & opc.Acknowledge.AckPointEvents) {
                  Log.log('Ack Point Events: ' + node.NodeId.Id)
                  let result = await db.collection(COLL_SOE).updateMany(
                    {
                      tag: node.NodeId.Id,
                      ack: 0,
                      ...(!AUTHENTICATION || userRights?.group1List?.length == 0
                        ? {}
                        : { group1: { $in: userRights.group1List } }),
                    },
                    {
                      $set: {
                        ack: 1,
                      },
                    }
                  )
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Ack Point Events',
                    tag: node.NodeId.Id,
                    timeTag: new Date(),
                  })
                } else if (node.Value.Body & opc.Acknowledge.RemoveOneEvent) {
                  Log.log('Remove One Event: ' + node.NodeId.Id)
                  let result = await db.collection(COLL_SOE).updateMany(
                    {
                      _id: new ObjectId(node._Properties.event_id),
                      ack: { $lte: 1 },
                      ...(!AUTHENTICATION || userRights?.group1List?.length == 0
                        ? {}
                        : { group1: { $in: userRights.group1List } }),
                    },
                    {
                      $set: {
                        ack: 2,
                      },
                    }
                  )
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Remove One Event',
                    tag: node.NodeId.Id,
                    eventId: new ObjectId(node._Properties.event_id),
                    timeTag: new Date(),
                  })
                } else if (node.Value.Body & opc.Acknowledge.AckOneEvent) {
                  Log.log('Ack One Event: ' + node.NodeId.Id)
                  await db.collection(COLL_SOE).updateMany(
                    {
                      _id: new ObjectId(node._Properties.event_id),
                      ack: 0,
                      ...(!AUTHENTICATION || userRights?.group1List?.length == 0
                        ? {}
                        : { group1: { $in: userRights.group1List } }),
                    },
                    {
                      $set: {
                        ack: 1,
                      },
                    }
                  )
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Ack One Event',
                    tag: node.NodeId.Id,
                    eventId: new ObjectId(node._Properties.event_id),
                    timeTag: new Date(),
                  })
                }

                if (node.Value.Body & opc.Acknowledge.AckAllAlarms) {
                  Log.log('Ack All Alarms')
                  let result = await db.collection(COLL_REALTIME).updateMany(
                    {
                      ...(!AUTHENTICATION || userRights?.group1List?.length == 0
                        ? {}
                        : { group1: { $in: userRights.group1List } }),
                    },
                    {
                      $set: {
                        alarmed: false,
                      },
                    }
                  )
                  // make digital event tags return to zero after acknowledged
                  result = await db.collection(COLL_REALTIME).updateMany(
                    {
                      $and: [
                        {
                          ...(!AUTHENTICATION ||
                          userRights?.group1List?.length == 0
                            ? {}
                            : { group1: { $in: userRights.group1List } }),
                        },
                        { type: 'digital' },
                        { isEvent: true },
                        { value: 1 },
                        {
                          $or: [
                            { origin: 'supervised' },
                            { origin: 'calculated' },
                          ],
                        },
                      ],
                    },
                    [
                      {
                        $set: {
                          value: 0,
                          valueString: '$stateTextFalse',
                          timeTagAtSource: null,
                          TimeTagAtSourceOk: null,
                        },
                      },
                    ]
                  )
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Ack All Alarms',
                    timeTag: new Date(),
                  })
                } else if (node.Value.Body & opc.Acknowledge.AckOneAlarm) {
                  Log.log('Ack alarm: ' + node.NodeId.Id)
                  let result = await db
                    .collection(COLL_REALTIME)
                    .updateOne(findPoint, {
                      $set: {
                        alarmed: false,
                      },
                    })
                  // make digital event tag return to zero after acknowledged
                  result = await db.collection(COLL_REALTIME).updateOne(
                    {
                      $and: [
                        findPoint,
                        { type: 'digital' },
                        { isEvent: true },
                        {
                          $or: [
                            { origin: 'supervised' },
                            { origin: 'calculated' },
                          ],
                        },
                      ],
                    },
                    [
                      {
                        $set: {
                          value: 0,
                          valueString: '$stateTextFalse',
                          timeTagAtSource: null,
                          TimeTagAtSourceOk: null,
                        },
                      },
                    ]
                  )
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Ack Point Alarm',
                    pointKey: node.NodeId.Id,
                    timeTag: new Date(),
                  })
                }
                if (node.Value.Body & opc.Acknowledge.SilenceBeep) {
                  Log.log('Silence Beep')
                  if (AUTHENTICATION && userRights.group1List.length > 0) {
                    // just remove groups from beep list
                    await db.collection(COLL_REALTIME).updateOne(
                      { _id: beepPointKey },
                      {
                        $pullAll: {
                          beepGroup1List: userRights.group1List,
                        },
                      }
                    )
                    // force silence when list is empty
                    await db.collection(COLL_REALTIME).updateOne(
                      { _id: beepPointKey, beepGroup1List: { $eq: [] } },
                      {
                        $set: {
                          value: new Double(0),
                          valueString: '0',
                          beepType: new Double(0),
                        },
                      }
                    )
                    UserActionsQueue.enqueue({
                      username: username,
                      action: 'Silence Beep',
                      timeTag: new Date(),
                    })
                  } else {
                    await db.collection(COLL_REALTIME).updateOne(
                      { _id: beepPointKey },
                      {
                        $set: {
                          value: new Double(0),
                          valueString: '0',
                          beepType: new Double(0),
                          beepGroup1List: [],
                        },
                      }
                    )
                    UserActionsQueue.enqueue({
                      username: username,
                      action: 'Silence Beep',
                      timeTag: new Date(),
                    })
                  }
                }

                OpcResp.ServiceId = opc.ServiceCode.WriteResponse
                OpcResp.Body.ResponseHeader.ServiceResult =
                  opc.StatusCode.GoodNoData
                OpcResp.Body.ResponseHeader.StringTable = [
                  opc.getStatusCodeName(opc.StatusCode.GoodNoData),
                  opc.getStatusCodeText(opc.StatusCode.GoodNoData),
                  'Ok, no data returned. Query time: ' +
                    (new Date().getTime() - tini) +
                    ' ms',
                ]
                res.send(OpcResp)
                Log.log(
                  'Write. Elapsed ' + (new Date().getTime() - tini) + ' ms'
                )
                return
              }

              if (node.AttributeId == opc.AttributeId.Value) {
                // Write a Value: Command

                if (AUTHENTICATION) {
                  // go check user right for command in mongodb (not just in the token for better security)
                  if (!(await canSendCommands(req))) {
                    // ACTION DENIED
                    Log.log(
                      `User has no right to issue commands! [${username}]`
                    )
                    OpcResp.Body.ResponseHeader.ServiceResult =
                      opc.StatusCode.BadUserAccessDenied
                    OpcResp.Body.ResponseHeader.StringTable = [
                      opc.getStatusCodeName(opc.StatusCode.BadUserAccessDenied),
                      opc.getStatusCodeText(opc.StatusCode.BadUserAccessDenied),
                      'User has no right to issue commands!',
                    ]
                    res.send(OpcResp)
                    return
                  } else {
                    Log.log(`User authorized to issue commands! [${username}]`)
                  }
                }

                if ('NodeId' in node)
                  if ('Id' in node.NodeId) {
                    if (
                      typeof node.Value !== 'object' ||
                      typeof node.Value.Type !== 'number' ||
                      (node.Value.Type !== opc.DataType.Double &&
                        node.Value.Type !== opc.DataType.String) || // only accepts a double or string value for the command
                      (typeof node.Value.Body !== 'number' &&
                        typeof node.Value.Body !== 'string')
                    ) {
                      OpcResp.Body.ResponseHeader.ServiceResult =
                        opc.StatusCode.BadNodeAttributesInvalid
                      OpcResp.Body.ResponseHeader.StringTable = [
                        opc.getStatusCodeName(
                          opc.StatusCode.BadNodeAttributesInvalid
                        ),
                        opc.getStatusCodeText(
                          opc.StatusCode.BadNodeAttributesInvalid
                        ),
                        'Invalid command type, malformed or missing information!',
                      ]
                      res.send(OpcResp)
                      return
                    }

                    // look for the command info in the database

                    let cmd_id = node.NodeId.Id
                    let cmd_val =
                      node.Value.Type === opc.DataType.Double
                        ? node.Value.Body
                        : 0.0
                    let cmd_val_str =
                      node.Value.Type === opc.DataType.String
                        ? node.Value.Body
                        : parseFloat(cmd_val).toString()
                    let query = { _id: parseInt(cmd_id) }
                    if (isNaN(parseInt(cmd_id))) query = { tag: cmd_id }

                    // search command in database (wait for results)
                    let data = await db.collection(COLL_REALTIME).findOne(query)

                    if (data === null || typeof data._id !== 'number') {
                      // command not found, abort
                      Log.log('Command not found!')
                      OpcResp.Body.ResponseHeader.ServiceResult =
                        opc.StatusCode.BadNodeIdUnknown
                      OpcResp.Body.ResponseHeader.StringTable = [
                        opc.getStatusCodeName(opc.StatusCode.BadNodeIdUnknown),
                        opc.getStatusCodeText(opc.StatusCode.BadNodeIdUnknown),
                        'Command point not found!',
                      ]
                      res.send(OpcResp)
                      return
                    }

                    if (AUTHENTICATION) {
                      // check if user has group1 list it can command
                      if (!(await canSendCommandTo(req, data.group1))) {
                        // ACTION DENIED
                        Log.log(
                          `User has no right to issue commands to the group1 destination! [${username}] [${data.group1}]`
                        )
                        OpcResp.Body.ResponseHeader.ServiceResult =
                          opc.StatusCode.BadUserAccessDenied
                        OpcResp.Body.ResponseHeader.StringTable = [
                          opc.getStatusCodeName(
                            opc.StatusCode.BadUserAccessDenied
                          ),
                          opc.getStatusCodeText(
                            opc.StatusCode.BadUserAccessDenied
                          ),
                          'User has no right to issue commands to the group1 destination!',
                        ]
                        res.send(OpcResp)
                        return
                      } else {
                        Log.log(
                          `User authorized to issue commands to the group1 destination! [${username}]`
                        )
                      }
                    }

                    let addressing = {}
                    if (
                      (data.protocolSourceCommonAddress != '' &&
                        isNaN(data.protocolSourceCommonAddress)) ||
                      isNaN(data.protocolSourceObjectAddress) ||
                      isNaN(data.protocolSourceASDU)
                    ) {
                      // non numerical addressing
                      addressing = {
                        protocolSourceCommonAddress:
                          data.protocolSourceCommonAddress,
                        protocolSourceObjectAddress:
                          data.protocolSourceObjectAddress,
                        protocolSourceASDU: data.protocolSourceASDU,
                      }
                    } else {
                      // numerical addressing: force data type as BSON double
                      addressing = {
                        protocolSourceCommonAddress: new Double(
                          data.protocolSourceCommonAddress
                        ),
                        protocolSourceObjectAddress: new Double(
                          data.protocolSourceObjectAddress
                        ),
                        protocolSourceASDU: new Double(data.protocolSourceASDU),
                      }
                    }

                    let result = await db.collection(COLL_COMMANDS).insertOne({
                      protocolSourceConnectionNumber: new Double(
                        data.protocolSourceConnectionNumber
                      ),
                      ...addressing,
                      protocolSourceCommandDuration: new Double(
                        data.protocolSourceCommandDuration
                      ),
                      protocolSourceCommandUseSBO:
                        data.protocolSourceCommandUseSBO,
                      pointKey: new Double(data._id),
                      tag: data.tag,
                      timeTag: new Date(),
                      value: new Double(cmd_val),
                      valueString: cmd_val_str,
                      originatorUserName: username,
                      originatorIpAddress:
                        req.headers['x-real-ip'] ||
                        req.headers['x-forwarded-for'] ||
                        req.socket.remoteAddress,
                    })

                    if (!result.acknowledged) {
                      OpcResp.Body.ResponseHeader.ServiceResult =
                        opc.StatusCode.BadUnexpectedError
                      OpcResp.Body.ResponseHeader.StringTable = [
                        opc.getStatusCodeName(
                          opc.StatusCode.BadUnexpectedError
                        ),
                        opc.getStatusCodeText(
                          opc.StatusCode.BadUnexpectedError
                        ),
                        'Could not queue command!',
                      ]
                      res.send(OpcResp)
                      return
                    }

                    // insert command action on SOE list, if desired
                    if (DoInsertCommandAsSOE) {
                      let eventText = cmd_val_str
                      if (data.type === 'digital') {
                        if (cmd_val) eventText = data.eventTextTrue
                        else eventText = data.eventTextFalse
                      }
                      db.collection(COLL_SOE).insertOne({
                        tag: data.tag,
                        pointKey: data._id,
                        description: data.description,
                        group1: data.group1,
                        eventText: eventText + CommandSentAsSOESymbol,
                        invalid: false,
                        priority: data.priority,
                        timeTag: new Date(),
                        timeTagAtSource: new Date(),
                        timeTagAtSourceOk: true,
                        ack: 1,
                      })
                    }

                    UserActionsQueue.enqueue({
                      username: username,
                      pointKey: node.NodeId.Id,
                      tag: data.tag,
                      action: 'Command',
                      properties: {
                        value: new Double(cmd_val),
                        valueString: cmd_val_str,
                      },
                      timeTag: new Date(),
                    })

                    OpcResp.Body.Results.push(opc.StatusCode.Good) // write ok
                    // a way for the client to find this inserted command
                    OpcResp.Body._CommandHandles.push(
                      result.insertedId.toString()
                    )
                  }
              } else if (node.AttributeId == opc.AttributeId.Description) {
                // Write Properties
                if ('NodeId' in node)
                  if ('Id' in node.NodeId) {
                    let findPoint = null
                    if (node.NodeId.IdType === opcIdTypeNumber) {
                      findPoint = { _id: parseInt(node.NodeId.Id) }
                    } else if (node.NodeId.IdType === opcIdTypeString) {
                      findPoint = { tag: node.NodeId.Id }
                    }
                    if (findPoint === null) {
                      OpcResp.Body.ResponseHeader.ServiceResult =
                        opc.StatusCode.BadNodeIdInvalid
                      OpcResp.Body.ResponseHeader.StringTable = [
                        opc.getStatusCodeName(opc.StatusCode.BadNodeIdInvalid),
                        opc.getStatusCodeText(opc.StatusCode.BadNodeIdInvalid),
                        'Invalid IdType!',
                      ]
                      res.send(OpcResp)
                      return
                    }

                    if (findPoint !== null) {
                      let prevData = await db
                        .collection(COLL_REALTIME)
                        .findOne(findPoint)

                      let alarmDisableNew = {}
                      if (!AUTHENTICATION || userRights?.disableAlarms)
                        if (
                          prevData?.alarmDisabled !==
                          node.Value._Properties?.alarmDisabled
                        )
                          alarmDisableNew = {
                            alarmDisabled:
                              node.Value._Properties?.alarmDisabled,
                          }

                      let annotationNew = {}
                      if (!AUTHENTICATION || userRights?.enterAnnotations)
                        if (
                          prevData?.annotation !==
                          node.Value._Properties?.annotation
                        )
                          annotationNew = {
                            annotation: node.Value._Properties?.annotation,
                          }

                      let loLimitNew = {}
                      if (!AUTHENTICATION || userRights?.enterLimits)
                        if (
                          prevData?.loLimit !== node.Value._Properties?.loLimit
                        )
                          loLimitNew = {
                            loLimit: new Double(node.Value._Properties.loLimit),
                          }

                      let hiLimitNew = {}
                      if (!AUTHENTICATION || userRights?.enterLimits)
                        if (
                          prevData?.hiLimit !== node.Value._Properties?.hiLimit
                        )
                          hiLimitNew = {
                            hiLimit: new Double(node.Value._Properties.hiLimit),
                          }

                      let hysteresisNew = {}
                      if (!AUTHENTICATION || userRights?.enterLimits)
                        if (
                          prevData?.hysteresis !==
                          node.Value._Properties?.hysteresis
                        )
                          hysteresisNew = {
                            hysteresis: new Double(
                              node.Value._Properties.hysteresis
                            ),
                          }

                      // loloLimit: node.Value._Properties.loLimit,
                      // lololoLimit: node.Value._Properties.loLimit,
                      // hihiLimit: node.Value._Properties.hiLimit,
                      // hihihiLimit: node.Value._Properties.hiLimit,

                      let notesNew = {}
                      if (!AUTHENTICATION || userRights?.enterNotes)
                        if (prevData?.notes !== node.Value._Properties?.notes)
                          notesNew = {
                            notes: node.Value._Properties.notes,
                          }

                      let valueNew = {}
                      if (
                        'substituted' in node.Value._Properties &&
                        'newValue' in node.Value._Properties
                      )
                        if (!AUTHENTICATION || userRights?.substituteValues)
                          if (
                            prevData?.value !== node.Value._Properties.newValue
                          )
                            valueNew = {
                              value: new Double(
                                node.Value._Properties.newValue
                              ),
                            }

                      let set = {
                        $set: {
                          ...alarmDisableNew,
                          ...annotationNew,
                          ...loLimitNew,
                          ...hiLimitNew,
                          ...hysteresisNew,
                          ...notesNew,
                          ...valueNew,
                        },
                      }

                      let result = await db
                        .collection(COLL_REALTIME)
                        .updateOne(findPoint, set)
                      if (result.acknowledged) {
                        // updateOne ok
                        OpcResp.Body.Results.push(opc.StatusCode.Good)
                        Log.log('update ok id: ' + node.NodeId.Id)
                        UserActionsQueue.enqueue({
                          username: username,
                          pointKey: node.NodeId.Id,
                          action: 'Update Properties',
                          properties: set['$set'],
                          timeTag: new Date(),
                        })

                        // if changed limits force an updated to recheck range
                        if (prevData.type === 'analog') {
                          if (
                            'alarmDisabled' in alarmDisableNew ||
                            'hiLimit' in hiLimitNew ||
                            'loLimit' in loLimitNew ||
                            'hysteresis' in hysteresisNew
                          ) {
                            Log.log('Update for range check: ' + node.NodeId.Id)
                            db.collection(COLL_REALTIME).updateOne(findPoint, {
                              $set: {
                                sourceDataUpdate: {
                                  ...prevData.sourceDataUpdate,
                                  rangeCheck: new Date().getTime(),
                                },
                              },
                            })
                          }
                        }

                        // insert event for changed annotation
                        if ('annotation' in annotationNew) {
                          let eventDate = new Date()
                          db.collection(COLL_SOE).insertOne({
                            tag: prevData.tag,
                            pointKey: prevData._id,
                            group1: prevData?.group1,
                            description: prevData?.description,
                            eventText:
                              annotationNew.annotation.trim() === ''
                                ? '🏷️🗑️'
                                : '🏷️🔒', // &#127991;
                            invalid: false,
                            priority: prevData?.priority,
                            timeTag: eventDate,
                            timeTagAtSource: eventDate,
                            timeTagAtSourceOk: true,
                            ack: 1,
                          })
                        }

                        // insert event for changed value
                        if ('value' in valueNew) {
                          let eventDate = new Date()
                          let eventText = ''
                          if (prevData?.type === 'digital')
                            eventText =
                              valueNew.value == 0
                                ? prevData?.eventTextFalse
                                : prevData?.eventTextTrue
                          else
                            eventText =
                              valueNew.value.toFixed(2) + ' ' + prevData?.unit
                          db.collection(COLL_SOE).insertOne({
                            tag: prevData.tag,
                            pointKey: prevData._id,
                            group1: prevData?.group1,
                            description: prevData?.description,
                            eventText: eventText,
                            invalid: false,
                            priority: prevData?.priority,
                            timeTag: eventDate,
                            timeTagAtSource: eventDate,
                            timeTagAtSourceOk: true,
                            ack: 1,
                          })
                        }
                      } else {
                        // some updateOne error
                        OpcResp.Body.Results.push(
                          opc.StatusCode.BadNodeIdUnknown
                        )
                        Log.log('update error id: ' + node.NodeId.Id)
                      }
                    }
                  } else
                    OpcResp.Body.Results.push(opc.StatusCode.BadNodeIdInvalid)
              }
            }
          }
          OpcResp.Body.ResponseHeader.ServiceResult = opc.StatusCode.Good
          res.send(OpcResp)
        }
        return
      case opc.ServiceCode.ReadRequest: // READ SERVICE
        {
          OpcResp.ServiceId = opc.ServiceCode.ReadResponse
          if (
            !('NodesToRead' in req.body.Body) &&
            !('ContentFilter' in req.body.Body)
          ) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.GoodNoData
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.GoodNoData),
              opc.getStatusCodeText(opc.StatusCode.GoodNoData),
              'No NodesToRead nor Content Filter',
            ]
            res.send(OpcResp)
            return
          }

          // if for some point Description is solicited, will respond with full info for all the points
          let points = [],
            cmdHandles = [],
            info = false,
            ack = false
          if ('NodesToRead' in req.body.Body)
            req.body.Body.NodesToRead.map((node) => {
              if ('AttributeId' in node) {
                if (node.AttributeId == opc.AttributeId.EventNotifier) {
                  if ('ClientHandle' in node) {
                    ack = true
                    cmdHandles.push(node.ClientHandle)
                  }
                }
                if (node.AttributeId == opc.AttributeId.Description) {
                  info = true
                }
              }
              if ('NodeId' in node)
                if ('Id' in node.NodeId) {
                  points.push(node.NodeId.Id)
                }
              return node
            })
          if (points.length == 0 && !('ContentFilter' in req.body.Body)) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.GoodNoData
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.GoodNoData),
              opc.getStatusCodeText(opc.StatusCode.GoodNoData),
              'No NodeId.Id found',
            ]
            res.send(OpcResp)
            return
          }

          if (ack) {
            // look for command ack
            // Log.log(cmdHandles[0])
            if (
              typeof cmdHandles[0] !== 'string' ||
              cmdHandles[0].length < 24
            ) {
              OpcResp.Body.ResponseHeader.ServiceResult =
                opc.StatusCode.BadRequestNotAllowed
              OpcResp.Body.ResponseHeader.StringTable = [
                opc.getStatusCodeName(opc.StatusCode.BadRequestNotAllowed),
                opc.getStatusCodeText(opc.StatusCode.BadRequestNotAllowed),
                'Missing or invalid ClientHandle.',
              ]
              res.send(OpcResp)
              return
            }

            let data = await db
              .collection(COLL_COMMANDS)
              .findOne({ _id: new ObjectId(cmdHandles[0]) })

            // Log.log(data);
            let status = -1,
              ackTime,
              cmdTime
            if (
              typeof data?.ack === 'boolean' ||
              typeof data?.cancelReason === 'string'
            ) {
              // ack received or cancelled
              status = opc.StatusCode.Bad
              if (data.ack === true) status = opc.StatusCode.Good
              cmdTime = data?.timeTag || new Date()
              ackTime = data?.ackTimeTag || new Date()
            } else status = opc.StatusCode.BadWaitingForResponse

            OpcResp.ServiceId = opc.ServiceCode.DataChangeNotification
            OpcResp.Body.MonitoredItems = [
              {
                ClientHandle: cmdHandles[0],
                Value: {
                  Value: data.value,
                  StatusCode: status,
                  SourceTimestamp: cmdTime,
                  ServerTimestamp: ackTime,
                },
                NodeId: {
                  IdType: opcIdTypeString,
                  Id: data.tag,
                  Namespace: opc.NamespaceMongodb,
                },
              },
            ]
            OpcResp.Body.ResponseHeader.ServiceResult = opc.StatusCode.Good
            if (typeof data?.cancelReason === 'string') {
              OpcResp.Body.ResponseHeader.StringTable = [
                data.cancelReason,
              ]
            }
            res.send(OpcResp)
            return
          }

          let tags = [],
            npts = []
          for (let i = 0; i < points.length; i++) {
            let val = parseInt(points[i])
            if (!isNaN(val)) npts.push(val)
            else tags.push(points[i])
          }

          let projection = info
            ? null
            : {
                projection: {
                  _id: 1,
                  tag: 1,
                  value: 1,
                  valueString: 1,
                  invalid: 1,
                  timeTag: 1,
                  loLimit: 1,
                  hiLimit: 1,
                  hysteresis: 1,
                  alarmed: 1,
                  alarmRange: 1,
                  alarmDisabled: 1,
                  frozen: 1,
                  type: 1,
                  annotation: 1,
                  origin: 1,
                  group1: 1,
                  beepType: 1,
                  beepGroup1List: 1,
                },
              }

          // optimize query for better performance
          // there is a cost to query empty $in lists!
          let findTags = {
            tag: {
              $in: tags,
            },
          }
          let findKeys = {
            _id: {
              $in: npts,
            },
          }
          let query = findTags
          if (npts.length > 0 && tags.length > 0)
            query = {
              // use $or only to find tags and _id keys
              $or: [findKeys, findTags],
            }
          else if (npts.length > 0) query = findKeys

          let sort = {}

          // if there is a content filter, it takes precedence over NodesToRead lists
          if ('ContentFilter' in req.body.Body) {
            sort = { group1: 1, group2: 1 }
            projection = null
            query = {}
            let grp1 = null
            let grp2 = null
            req.body.Body.ContentFilter.map((contentFilter) => {
              // supports attribute (operand) Equals (operator) to a literal value (operand)
              if (contentFilter.FilterOperator === opc.FilterOperator.Equals) {
                if (
                  contentFilter.FilterOperands[0].FilterOperand ===
                    opc.Operand.Attribute &&
                  contentFilter.FilterOperands[1].FilterOperand ===
                    opc.Operand.Literal
                ) {
                  if (contentFilter.FilterOperands[0].Value == 'group1') {
                    grp1 = { group1: contentFilter.FilterOperands[1].Value }
                  }
                  if (contentFilter.FilterOperands[0].Value == 'group2') {
                    grp2 = { group2: contentFilter.FilterOperands[1].Value }
                  }

                  if (
                    // fake attribute 'persistentAlarms' means add not normal states (digitals with alarmState=0 and value=0 or alarmState=1 and value=1)
                    contentFilter.FilterOperands[0].Value == 'persistentAlarms'
                  ) {
                    sort = { alarmed: -1, timeTagAlarm: -1 }
                    query = {
                      $or: [
                        {
                          $and: [
                            { type: 'analog' },
                            { alarmDisabled: false },
                            { alarmed: true },
                            { invalid: false },
                            { ...(grp1 !== null ? grp1 : {}) },
                            { ...(grp2 !== null ? grp2 : {}) },
                          ],
                        },
                        {
                          $and: [
                            { type: 'analog' },
                            { alarmDisabled: false },
                            { alarmRange: { $exists: true, $ne: 0 } },
                            { invalid: false },
                            { ...(grp1 !== null ? grp1 : {}) },
                            { ...(grp2 !== null ? grp2 : {}) },
                          ],
                        },
                        {
                          $and: [
                            { type: 'digital' },
                            { alarmDisabled: false },
                            { alarmState: 0 },
                            { value: 0 },
                            { invalid: false },
                            { ...(grp1 !== null ? grp1 : {}) },
                            { ...(grp2 !== null ? grp2 : {}) },
                          ],
                        },
                        {
                          $and: [
                            { type: 'digital' },
                            { alarmDisabled: false },
                            { alarmState: 1 },
                            { value: 1 },
                            { invalid: false },
                            { ...(grp1 !== null ? grp1 : {}) },
                            { ...(grp2 !== null ? grp2 : {}) },
                          ],
                        },
                        query,
                      ],
                    }
                  } else
                    query[contentFilter.FilterOperands[0].Value] =
                      contentFilter.FilterOperands[1].Value
                }
              }
              return contentFilter
            })
          }

          try {
            const results = await db
              .collection(COLL_REALTIME)
              .find(query, projection)
              .sort(sort)
              .toArray()

            if (results) {
              if (results.length == 0) {
                OpcResp.Body.ResponseHeader.ServiceResult =
                  opc.StatusCode.GoodNoData
                OpcResp.Body.ResponseHeader.StringTable = [
                  opc.getStatusCodeName(opc.StatusCode.GoodNoData),
                  opc.getStatusCodeText(opc.StatusCode.GoodNoData),
                  'No NodeId.Id requested found on realtimeDatabase',
                ]
                res.send(OpcResp)
                return
              }

              let Results = []
              if ('NodesToRead' in req.body.Body) {
                req.body.Body.NodesToRead.map((node) => {
                  let Result = {
                    // will return this if point not found or access denied
                    StatusCode: opc.StatusCode.BadNotFound,
                    NodeId: node.NodeId,
                    Value: null,
                    _Properties: null,
                  }

                  for (let i = 0; i < results.length; i++) {
                    let pointInfo = results[i]

                    if (
                      node.NodeId.Id === pointInfo.tag ||
                      node.NodeId.Id === pointInfo._id
                    ) {
                      // check for group1 list in user rights (from token)
                      if (AUTHENTICATION && userRights.group1List.length > 0) {
                        if (
                          ![-1, -2].includes(pointInfo._id) &&
                          !userRights.group1List.includes(pointInfo.group1)
                        ) {
                          // Access to data denied! (return null value and properties)
                          Result.StatusCode = opc.StatusCode.BadUserAccessDenied
                          break
                        }
                      }

                      Result.StatusCode = opc.StatusCode.Good
                      Result._Properties = {
                        _id: pointInfo._id,
                        valueString: pointInfo.valueString,
                        alarmed: pointInfo.alarmed,
                        ...(pointInfo.type === 'analog'
                          ? { alarmRange: pointInfo?.alarmRange }
                          : {}),
                        ...(pointInfo.type === 'analog'
                          ? { frozen: pointInfo?.frozen }
                          : {}),
                        alarmDisabled: pointInfo.alarmDisabled,
                        transit: pointInfo.transit,
                        annotation: pointInfo.annotation,
                        notes: pointInfo.notes,
                        origin: pointInfo.origin,
                        type: pointInfo.type,
                      }
                      if (info) {
                        Result._Properties = {
                          _id: pointInfo._id,
                          valueString: pointInfo.valueString,
                          valueDefault: pointInfo.valueDefault,
                          alarmed: pointInfo.alarmed,
                          ...(pointInfo.type === 'analog'
                            ? { alarmRange: pointInfo?.alarmRange }
                            : {}),
                          ...(pointInfo.type === 'analog'
                            ? { frozen: pointInfo?.frozen }
                            : {}),
                          alarmDisabled: pointInfo.alarmDisabled,
                          transit: pointInfo.transit,
                          group1: pointInfo.group1,
                          group2: pointInfo.group2,
                          description: pointInfo.description,
                          ungroupedDescription: pointInfo.ungroupedDescription,
                          hiLimit: pointInfo.hiLimit,
                          loLimit: pointInfo.loLimit,
                          hysteresis: pointInfo.hysteresis,
                          stateTextTrue: pointInfo.stateTextTrue,
                          stateTextFalse: pointInfo.stateTextFalse,
                          unit: pointInfo.unit,
                          annotation: pointInfo.annotation,
                          notes: pointInfo.notes,
                          commandOfSupervised: pointInfo.commandOfSupervised,
                          supervisedOfCommand: pointInfo.supervisedOfCommand,
                          origin: pointInfo.origin,
                          beepType: pointInfo?.beepType,
                          beepGroup1List: pointInfo?.beepGroup1List,
                          type: pointInfo.type,
                        }
                      }
                      if (
                        pointInfo.type === 'string' ||
                        pointInfo.type === 'json'
                      )
                        Result.Value = {
                          Type: opc.DataType.String,
                          Body: pointInfo.valueString,
                          Quality: pointInfo.invalid
                            ? opc.StatusCode.Bad
                            : opc.StatusCode.Good,
                        }
                      else
                        Result.Value = {
                          Type:
                            pointInfo.type === 'digital'
                              ? opc.DataType.Boolean
                              : opc.DataType.Double,
                          Body:
                            pointInfo.type === 'digital'
                              ? pointInfo.value !== 0
                                ? true
                                : false
                              : parseFloat(pointInfo.value),
                          Quality: pointInfo.invalid
                            ? opc.StatusCode.Bad
                            : opc.StatusCode.Good,
                        }
                      Result.NodeId = {
                        IdType: opcIdTypeString,
                        Id: pointInfo.tag,
                        Namespace: opc.NamespaceMongodb,
                      }
                      Result.SourceTimestamp = pointInfo.timeTag
                      break
                    }
                  }
                  Results.push(Result)
                })
              } else {
                // no NodesToRead so it is a filtered query
                results.map((node) => {
                  let Value = {}
                  if (node.type === 'string' || node.type === 'json')
                    Value = {
                      Type: opc.DataType.String,
                      Body: node.valueString,
                      Quality: node.invalid
                        ? opc.StatusCode.Bad
                        : opc.StatusCode.Good,
                    }
                  else
                    Value = {
                      Type:
                        node.type === 'digital'
                          ? opc.DataType.Boolean
                          : opc.DataType.Double,
                      Body:
                        node.type === 'digital'
                          ? node.value !== 0
                            ? true
                            : false
                          : node.value,
                      Quality: node.invalid
                        ? opc.StatusCode.Bad
                        : opc.StatusCode.Good,
                    }

                  let Result = {
                    StatusCode: opc.StatusCode.Good,
                    NodeId: {
                      IdType: opcIdTypeString,
                      Id: node.tag,
                    },
                    Value: Value,
                    _Properties: {
                      _id: node._id,
                      valueString: node.valueString,
                      valueDefault: node.valueDefault,
                      alarmed: node.alarmed,
                      ...(node.type === 'analog'
                        ? { alarmRange: node?.alarmRange }
                        : {}),
                      ...(node.type === 'analog'
                        ? { frozen: node?.frozen }
                        : {}),
                      alarmDisabled: node.alarmDisabled,
                      alarmState: node.alarmState,
                      isEvent: node.isEvent,
                      transit: node.transit,
                      group1: node.group1,
                      group2: node.group2,
                      description: node.description,
                      ungroupedDescription: node.ungroupedDescription,
                      hiLimit: node.hiLimit,
                      loLimit: node.loLimit,
                      hysteresis: node.hysteresis,
                      stateTextTrue: node.stateTextTrue,
                      stateTextFalse: node.stateTextFalse,
                      unit: node.unit,
                      annotation: node.annotation,
                      notes: node.notes,
                      commandOfSupervised: node.commandOfSupervised,
                      supervisedOfCommand: node.supervisedOfCommand,
                      origin: node.origin,
                      timeTagAlarm: node.timeTagAlarm,
                      priority: node.priority,
                      origin: node.origin,
                      timeTagAtSourceOk:
                        'timeTagAtSourceOk' in node
                          ? node.TimeTagAtSourceOk
                          : null,
                      type: node.type,
                    },
                  }
                  Result.ServerTimestamp = node.timeTag
                  Result.SourceTimestamp =
                    'timeTagAtSource' in node && node.timeTagAtSource !== null
                      ? node.timeTagAtSource
                      : null

                  // check for group1 list in user rights (from token)
                  if (AUTHENTICATION && userRights.group1List.length > 0) {
                    if (!userRights.group1List.includes(node.group1)) {
                      // Access to data denied!
                      return node
                    }
                  }

                  Results.push(Result)

                  return node
                })
              }

              OpcResp.Body.Results = Results
              OpcResp.Body.ResponseHeader.ServiceResult = opc.StatusCode.Good
              OpcResp.Body.ResponseHeader.StringTable = [
                opc.getStatusCodeName(opc.StatusCode.Good),
                opc.getStatusCodeText(opc.StatusCode.Good),
                'Query time: ' + (new Date().getTime() - tini) + ' ms',
              ]
              res.send(OpcResp)
              Log.log(
                'Read returned ' +
                  results.length +
                  ' values. Elapsed ' +
                  (new Date().getTime() - tini) +
                  ' ms'
              )
            }
          } catch (error) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.BadInvalidState
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.BadInvalidState),
              opc.getStatusCodeText(opc.StatusCode.BadInvalidState),
              'Error reading realtimeDatabase',
            ]
            res.send(OpcResp)
            return
          }
        }
        return
      case opc.ServiceCode.HistoryReadRequest: // HISTORY READ SERVICE
        {
          OpcResp.ServiceId = opc.ServiceCode.HistoryReadResponse
          if (
            !('HistoryReadDetails' in req.body.Body) ||
            !('ParameterTypeId' in req.body.Body.HistoryReadDetails) ||
            req.body.Body.HistoryReadDetails.ParameterTypeId !==
              opc.ServiceCode.ReadRawModifiedDetails
          ) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.BadHistoryOperationInvalid
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.BadHistoryOperationInvalid),
              opc.getStatusCodeText(opc.StatusCode.BadHistoryOperationInvalid),
              'Invalid HistoryReadDetails option',
            ]
            res.send(OpcResp)
            return
          }

          let returnServerTimestamp = true
          let returnSourceTimestamp = true
          if ('TimestampsToReturn' in req.body.Body) {
            switch (req.body.Body.TimestampsToReturn) {
              case opc.TimestampsToReturn.Source:
                returnServerTimestamp = false
                break
              case opc.TimestampsToReturn.Server:
                returnSourceTimestamp = false
                break
              case opc.TimestampsToReturn.Neither:
                returnServerTimestamp = false
                returnSourceTimestamp = false
                break
              case opc.TimestampsToReturn.Both:
              case opc.TimestampsToReturn.Invalid:
              default:
                break
            }
          }

          if (
            !('NodesToRead' in req.body.Body) &&
            !('ContentFilter' in req.body.Body)
          ) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.GoodNoData
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.GoodNoData),
              opc.getStatusCodeText(opc.StatusCode.GoodNoData),
              'No NodesToRead',
            ]
            res.send(OpcResp)
            return
          }

          let tags = []
          if ('NodesToRead' in req.body.Body) {
            req.body.Body.NodesToRead.map((node) => {
              if ('AttributeId' in node) {
                if (node.AttributeId == opc.AttributeId.Value) {
                  if ('NodeId' in node)
                    if ('IdType' in node.NodeId)
                      if (node.NodeId.IdType === opcIdTypeString)
                        if ('Id' in node.NodeId) {
                          // only string keys supported here
                          tags.push("'" + node.NodeId.Id + "'")
                        }
                }
              }
              return node
            })
            if (tags.length == 0) {
              OpcResp.Body.ResponseHeader.ServiceResult =
                opc.StatusCode.GoodNoData
              OpcResp.Body.ResponseHeader.StringTable = [
                opc.getStatusCodeName(opc.StatusCode.GoodNoData),
                opc.getStatusCodeText(opc.StatusCode.GoodNoData),
                'No valid NodeId.Ids/Attribute request found',
              ]
              res.send(OpcResp)
              return
            }
          }

          // default read is last hour
          let dt = new Date()
          let endDateTime = dt.toISOString()
          dt.setHours(dt.getHours() - 1)
          let startDateTime = dt.toISOString()
          let limitValues = 10000

          if ('ParameterData' in req.body.Body.HistoryReadDetails) {
            if ('StartTime' in req.body.Body.HistoryReadDetails.ParameterData)
              startDateTime =
                req.body.Body.HistoryReadDetails.ParameterData.StartTime
            if ('EndTime' in req.body.Body.HistoryReadDetails.ParameterData)
              endDateTime =
                req.body.Body.HistoryReadDetails.ParameterData.EndTime
            if (
              'NumValuesPerNode' in
              req.body.Body.HistoryReadDetails.ParameterData
            )
              limitValues =
                req.body.Body.HistoryReadDetails.ParameterData.NumValuesPerNode

            // req.body.Body.HistoryReadDetails.ParameterData.IsTeasModified is expected to be false!
            if (
              'IsReadModified' in
                req.body.Body.HistoryReadDetails.ParameterData &&
              req.body.Body.HistoryReadDetails.ParameterData.IsReadModified ===
                true
            ) {
              OpcResp.Body.ResponseHeader.ServiceResult =
                opc.StatusCode.BadHistoryOperationUnsupported
              OpcResp.Body.ResponseHeader.StringTable = [
                opc.getStatusCodeName(
                  opc.StatusCode.BadHistoryOperationUnsupported
                ),
                opc.getStatusCodeText(
                  opc.StatusCode.BadHistoryOperationUnsupported
                ),
                'Invalid ReadRawModifiedDetails/isReadModified option (must be false)',
              ]
              res.send(OpcResp)
              return
            }

            // Ignored parameters
            // req.body.Body.ReadRawModifiedDetails.NumValuesPerNode
            // req.body.Body.ReadRawModifiedDetails.ReturnBounds
          }

          // when Namespace is 2 (mongodb) will read from mongodb soeData
          if (
            'Namespace' in req.body.Body &&
            req.body.Body.Namespace === opc.NamespaceMongodb
          ) {
            let group1Filter = []
            let filterPriority = {}

            if (
              'ContentFilter' in req.body.Body &&
              req.body.Body.ContentFilter.length > 0
              // &&
              // req.body.Body.ContentFilter[0].FilterOperands.length > 0
            )
              req.body.Body.ContentFilter.map((node) => {
                if (node.FilterOperator === opc.FilterOperator.LessThanOrEqual)
                  // priority filter
                  filterPriority = {
                    priority: { $lte: node.FilterOperands[0] },
                  }

                if (node.FilterOperator === opc.FilterOperator.InList)
                  // group1 list filter
                  node.FilterOperands.map((element) => {
                    if (element.FilterOperand === opc.Operand.Literal)
                      if (typeof element.Value === 'string') {
                        group1Filter.push(element.Value)
                      }
                    return element
                  })

                return node
              })

            let filterDateGte = {
              timeTag: { $gte: new Date(startDateTime) },
            }
            let filterDateLte = {
              timeTag: { $lte: new Date(endDateTime) },
            }
            let sort = { timeTag: -1, timeTagAtSource: -1, tag: -1 }
            if (endDateTime !== null && startDateTime !== null)
              sort = { timeTag: 1, timeTagAtSource: 1, tag: 1 }

            if (!returnServerTimestamp) {
              sort = { timeTagAtSource: -1, timeTag: -1, tag: -1 }
              if (endDateTime !== null && startDateTime !== null)
                sort = { timeTagAtSource: 1, timeTag: 1, tag: 1 }
              filterDateGte = {
                timeTagAtSource: { $gte: new Date(startDateTime) },
              }
              filterDateLte = {
                timeTagAtSource: { $lte: new Date(endDateTime) },
              }
            }

            if (startDateTime === null) {
              filterDateGte = {}
            }
            if (endDateTime === null) {
              filterDateLte = {}
            }

            let filterGroup = {}
            if (group1Filter.length > 0)
              filterGroup = {
                $or: [
                  {
                    group1: {
                      $in: group1Filter,
                    },
                  },
                  {
                    tag: {
                      $in: tags,
                    },
                  },
                ],
              }

            // depending on aggregation do a aggregate (more expensive) or a simple find
            if (
              'AggregateFilter' in req.body.Body &&
              req.body.Body.AggregateFilter !== null &&
              'AggregateType' in req.body.Body.AggregateFilter &&
              req.body.Body.AggregateFilter.AggregateType === 'Count'
            ) {
              let results = []
              try {
                results = await db
                  .collection(COLL_SOE)
                  .aggregate([
                    {
                      $match: {
                        $and: [
                          filterDateGte,
                          filterDateLte,
                          filterGroup,
                          filterPriority,
                          { ack: { $lte: 1 } },
                        ],
                      },
                    },
                    {
                      $group: {
                        _id: '$tag',
                        tag: { $last: '$tag' },
                        pointKey: { $last: '$pointKey' },
                        group1: { $last: '$group1' },
                        description: { $last: '$description' },
                        eventText: { $last: '$eventText' },
                        description: { $last: '$description' },
                        invalid: { $last: '$invalid' },
                        priority: { $last: '$priority' },
                        timeTag: { $last: '$timeTag' },
                        timeTagAtSource: { $last: '$timeTagAtSource' },
                        timeTagAtSourceOk: { $last: '$timeTagAtSourceOk' },
                        ack: { $last: '$ack' },
                        count: { $sum: 1 },
                        event_id: { $last: '$_id' },
                      },
                    },
                  ])
                  .sort(sort)
                  .limit(limitValues)
                  .toArray()
              } catch (err) {
                Log.log(err)
                results = []
              }
              procArrayResults(results)
            } else {
              let results = []
              try {
                results = await db
                  .collection(COLL_SOE)
                  .find(
                    {
                      $and: [
                        filterDateGte,
                        filterDateLte,
                        filterGroup,
                        filterPriority,
                        { ack: { $lte: endDateTime !== null ? 2 : 1 } }, // when realtime query (endDate=null) filter out eliminated (ack=2) events
                      ],
                    },
                    {}
                  )
                  .sort(sort)
                  .limit(limitValues)
                  .toArray()
              } catch (err) {
                results = []
              }
              procArrayResults(results)
            }

            function procArrayResults(results) {
              if (results) {
                if (results.length == 0) {
                  OpcResp.Body.ResponseHeader.ServiceResult =
                    opc.StatusCode.GoodNoData
                  OpcResp.Body.ResponseHeader.StringTable = [
                    opc.getStatusCodeName(opc.StatusCode.GoodNoData),
                    opc.getStatusCodeText(opc.StatusCode.GoodNoData),
                    'No NodeId.Id requested found on soeData',
                  ]
                  res.send(OpcResp)
                  return
                }

                let Results = []
                results.map((node) => {
                  // check for group1 list in user rights (from token)
                  if (AUTHENTICATION && userRights.group1List.length > 0) {
                    if (!userRights.group1List.includes(node.group1)) {
                      // Access to data denied!
                      return node
                    }
                  }
                  let NodeId = {
                    IdType: opcIdTypeString,
                    Id: node.tag,
                    Namespace: opc.NamespaceMongodb,
                  }

                  let HistoryData = {
                    Value: {
                      Type: opc.DataType.String,
                      Body: node.eventText,
                      Quality: node.invalid
                        ? opc.StatusCode.Bad
                        : opc.StatusCode.Good,
                      Count: typeof node.count === 'number' ? node.count : 1,
                    },
                    ServerTimestamp: node.timeTag,
                    SourceTimestamp: node.timeTagAtSource,
                    SourceTimestampOk: node.timeTagAtSourceOk,
                    Acknowledge: node.ack,
                  }

                  let _Properties = {
                    group1: node.group1,
                    description: node.description,
                    priority: node.priority,
                    pointKey: node.pointKey,
                    event_id: node._id === node.tag ? node.event_id : node._id,
                  }

                  let HistoryReadResult = {
                    StatusCode: opc.StatusCode.Good,
                    HistoryData: [HistoryData],
                    // ContinuationPoint: null,
                    NodeId: NodeId,
                    _Properties: _Properties,
                  }
                  Results.push(HistoryReadResult)
                  return node
                })

                OpcResp.Body.Results = Results
                OpcResp.Body.ResponseHeader.ServiceResult = opc.StatusCode.Good
                OpcResp.Body.ResponseHeader.StringTable = [
                  opc.getStatusCodeName(opc.StatusCode.Good),
                  opc.getStatusCodeText(opc.StatusCode.Good),
                  'Query time: ' + (new Date().getTime() - tini) + ' ms',
                ]
                res.send(OpcResp)
                Log.log(
                  'HistoryRead (Events) returned ' +
                    Results.length +
                    ' values. Elapsed ' +
                    (new Date().getTime() - tini) +
                    ' ms'
                )
                return
              }
            }
            return
          }

          let query =
            'SELECT tag, value, flags, ' +
            'time_tag, ' +
            'time_tag_at_source ' +
            'FROM hist ' +
            "WHERE time_tag>='" +
            startDateTime +
            "' AND " +
            "time_tag<='" +
            endDateTime +
            "' AND " +
            'tag IN (' +
            tags.join(',') +
            ') ' +
            'ORDER BY tag asc, time_tag ASC'

          if (startDateTime === endDateTime) {
            query =
              'SELECT tag as tag, ' +
              'last(value, time_tag) as value, ' +
              'last(flags, time_tag) as flags, ' +
              'last(time_tag, time_tag) as time_tag, ' +
              'last(time_tag_at_source, time_tag) as time_tag_at_source ' +
              'from hist ' +
              'where ' +
              "time_tag<='" +
              startDateTime +
              "' and " +
              "time_tag> (TIMESTAMP '" +
              startDateTime +
              "' - INTERVAL '0.5 day') and " +
              'tag in (' +
              tags.join(',') +
              ') group by tag'
          }

          // read data from postgreSQL
          if (!pool) {
            OpcResp.ServiceId = opc.ServiceCode.ServiceFault
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.BadServerNotConnected
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.BadServerNotConnected),
              opc.getStatusCodeText(opc.StatusCode.BadServerNotConnected),
              'Database not connected!',
            ]
            res.send(OpcResp)
            return
          }
          pool.query(query, (err, resp) => {
            if (err) {
              OpcResp.ServiceId = opc.ServiceCode.ServiceFault
              OpcResp.Body.ResponseHeader.ServiceResult =
                opc.StatusCode.BadServerNotConnected
              OpcResp.Body.ResponseHeader.StringTable = [
                opc.getStatusCodeName(opc.StatusCode.BadServerNotConnected),
                opc.getStatusCodeText(opc.StatusCode.BadServerNotConnected),
                'Database error!',
              ]
              res.send(OpcResp)
              return
            }

            let Results = [] // [{ StatusCode, ContinuationPoint, HistoryData[], NodeId }]
            req.body.Body.NodesToRead.map((node) => {
              let HistoryReadResult = {
                StatusCode: opc.StatusCode.BadNotFound,
                HistoryData: [],
                ContinuationPoint: null,
                NodeId: null,
              }
              if ('AttributeId' in node) {
                if (node.AttributeId == opc.AttributeId.Value) {
                  if ('NodeId' in node)
                    if ('IdType' in node.NodeId)
                      if (node.NodeId.IdType === opcIdTypeString)
                        if ('Id' in node.NodeId) {
                          // only string keys supported here
                          HistoryReadResult.NodeId = node.NodeId
                          resp.rows.map((node) => {
                            if (node.tag === HistoryReadResult.NodeId.Id) {
                              HistoryReadResult.StatusCode = opc.StatusCode.Good
                              HistoryReadResult.HistoryData.push({
                                Value: {
                                  Type:
                                    node.flags.charAt(2) === '0'
                                      ? opc.DataType.Boolean
                                      : opc.DataType.Double,
                                  Body:
                                    node.flags.charAt(2) === '0'
                                      ? node.value === 0
                                        ? false
                                        : true
                                      : node.value,
                                  Quality:
                                    node.flags.charAt(0) === '0'
                                      ? opc.StatusCode.Good
                                      : opc.StatusCode.Bad,
                                },
                                SourceTimestamp: returnSourceTimestamp
                                  ? node.time_tag_at_source
                                  : undefined,
                                ServerTimestamp: returnServerTimestamp
                                  ? node.time_tag
                                  : undefined,
                              })
                            }
                            return node
                          })
                        }
                }
                Results.push(HistoryReadResult)
              }
              return node
            })

            OpcResp.Body.Results = Results
            OpcResp.Body.ResponseHeader.ServiceResult = opc.StatusCode.Good
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.Good),
              opc.getStatusCodeText(opc.StatusCode.Good),
              'Query time: ' + (new Date().getTime() - tini) + ' ms',
            ]
            res.send(OpcResp)
            Log.log(
              'HistoryRead returned ' +
                resp.rows.length +
                ' values. Elapsed ' +
                (new Date().getTime() - tini) +
                ' ms'
            )
          })
        }
        return
      case opc.ServiceCode.Extended_RequestUniqueAttributeValues: // list unique attribute values
        {
          OpcResp.ServiceId =
            opc.ServiceCode.Extended_ResponseUniqueAttributeValues

          if (req.body.Body.AttributeId !== opc.AttributeId.ExtendedGroup1) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.BadAttributeIdInvalid
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.BadAttributeIdInvalid),
              opc.getStatusCodeText(opc.StatusCode.BadAttributeIdInvalid),
              'Requested attribute not supported!',
            ]
            res.send(OpcResp)
            return
          }

          let results = []
          try {
            results = await db
              .collection(COLL_REALTIME)
              .aggregate([
                {
                  $group: {
                    _id: '$group1',
                    group1: { $last: '$group1' },
                    count: { $sum: 1 },
                  },
                },
              ])
              .sort({ group1: 1 })
              .toArray()
          } catch (err) {
            Log.log(err)
            results = []
          }

          let Results = []
          await results.map((node) => {
            // check for group1 list in user rights (from token)
            if (AUTHENTICATION && userRights.group1List.length > 0) {
              if (!userRights.group1List.includes(node.group1)) {
                // Access to data denied!
                return node
              }
            }
            let Value = {
              Value: {
                Type: opc.DataType.String,
                Body: node.group1,
                Count: node.count,
              },
            }
            Results.push(Value)
            return node
          })

          OpcResp.Body.Results = Results
          OpcResp.Body.ResponseHeader.ServiceResult = opc.StatusCode.Good
          OpcResp.Body.ResponseHeader.StringTable = [
            opc.getStatusCodeName(opc.StatusCode.Good),
            opc.getStatusCodeText(opc.StatusCode.Good),
            'Query time: ' + (new Date().getTime() - tini) + ' ms',
          ]
          res.send(OpcResp)
          Log.log(
            'Unique attribute values returned ' +
              Results.length +
              ' values. Elapsed ' +
              (new Date().getTime() - tini) +
              ' ms'
          )
        }
        return
      default:
        OpcResp.ServiceId = opc.ServiceCode.ServiceFault
        OpcResp.Body.ResponseHeader.ServiceResult =
          opc.StatusCode.BadServiceUnsupported
        OpcResp.Body.ResponseHeader.StringTable = [
          opc.getStatusCodeName(opc.StatusCode.BadServiceUnsupported),
          opc.getStatusCodeText(opc.StatusCode.BadServiceUnsupported),
          'Existing services: 629=ReadRequest, 671=WriteRequest, 100000001=Extended_RequestListUniqueAttributeValues',
        ]
        res.send(OpcResp)
        return
    }
  }

  for (;;) {
    try {
      if (clientMongo) {
        if (!HintMongoIsConnected) {
          // not anymore connected, will retry
          clientMongo.close()
          db = null
          clientMongo = null
        } else {
          // it is connected: process userActions fifo
          while (!UserActionsQueue.isEmpty()) {
            let ins = UserActionsQueue.peek()
            db.collection(COLL_ACTIONS).insertOne(ins)
            UserActionsQueue.dequeue()
          }
        }
      }

      if (!clientMongo) {
        // new connection
        Log.log('Connecting to ' + jsConfig.mongoConnectionString)
        await MongoClient.connect(
          jsConfig.mongoConnectionString,
          jsConfig.MongoConnectionOptions
        )
          .then(async (client) => {
            clientMongo = client
            HintMongoIsConnected = true
            db = clientMongo.db(jsConfig.mongoDatabaseName)
          })
          .catch(function (err) {
            db = null
            clientMongo = null
            Log.log(err)
            if (err.name == 'MongoParseError') process.exit(-1)
          })
      }
    } catch (e) {
      if (clientMongo) clientMongo.close()
      db = null
      clientMongo = null
      Log.log(e)
    }

    // wait 5 seconds
    await new Promise((resolve) => setTimeout(resolve, 5000))

    if (!(await checkConnectedMongo(clientMongo))) {
      clientMongo = null
    }
  }
})()

// test mongoDB connectivity
async function checkConnectedMongo(client) {
  if (!client) {
    return false
  }
  const CheckMongoConnectionTimeout = 10000
  let tr = setTimeout(() => {
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
