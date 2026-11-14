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

const IP_BIND = process.env.JS_IP_BIND || '127.0.0.1'
const HTTP_PORT = process.env.JS_HTTP_PORT || 8080
const GRAFANA_SERVER = process.env.JS_GRAFANA_SERVER || 'http://127.0.0.1:3000'
const LOGIO_SERVER = process.env.JS_LOGIO_SERVER || 'http://127.0.0.1:6688'
const METABASE_SERVER =
  process.env.JS_METABASE_SERVER || 'http://127.0.0.1:3001'
const NODERED_SERVER =
  process.env.NODERED_SERVER || 'http://127.0.0.1:1880/nodered/'
// n8n editor, proxied on /n8n: n8n must run with N8N_PATH=/n8n/
const N8N_SERVER = process.env.JS_N8N_SERVER || 'http://127.0.0.1:5678/'
// supervisord web interface: the log viewer of the linux and docker runtimes
const SUPERVISOR_SERVER =
  process.env.JS_SUPERVISOR_SERVER || 'http://admin:jsonscada@127.0.0.1:9000'
const OPCAPI_AP = '/Invoke/' // mimic of webhmi from OPC reference app https://github.com/OPCFoundation/UA-.NETStandard/tree/demo/webapi/SampleApplications/Workshop/Reference
const GETFILE_AP = '/GetFile' // API Access point for requesting mongodb files (gridfs)
const SAFE_GETFILE_MIME_RE =
  /^(image\/(png|jpe?g|gif|webp|bmp|avif)|video\/[a-z0-9.+-]+|audio\/[a-z0-9.+-]+|text\/(plain|csv)|application\/(pdf|json|octet-stream))$/
const ALLOW_FRAMING = (process.env.JS_ALLOW_FRAMING || '').toLowerCase() === 'true'
const QUERYJSON_AP = '/queryJSON' // API Access point for special custom queries returning JSON
const COLL_REALTIME = 'realtimeData'
const COLL_SOE = 'soeData'
const COLL_COMMANDS = 'commandsQueue'
const COLL_ACTIONS = 'userActions'
const GRAPHQL_AP = process.env.JS_GRAPHQL_AP || '/apollo' // same as graphql-server.js
const LoadConfig = require('./load-config')
const Log = require('./simple-logger')
const express = require('express')
const fileUpload = require('express-fileupload')
const path = require('path')
const fs = require('fs')
const os = require('os')
// temporary files of multipart uploads (removed after each request)
const UPLOAD_TMP_DIR = path.join(os.tmpdir(), 'json-scada-uploads')
const cors = require('cors')
const app = express()
const cookieParser = require('cookie-parser')
const crypto = require('crypto')
const { ObjectId, Double, GridFSBucket } = require('mongodb')
const opc = require('./opc_codes.js')
const { getPgPool } = require('./pg-pool')
const UserActionsQueue = require('./userActionsQueue')
const {
  createNoderedProxy,
  attachNoderedUpgrade,
  createN8nProxy,
  attachN8nUpgrade,
  createLogioProxy,
  attachLogioUpgrade,
  createSupervisorProxy,
} = require('./proxy-utils')
const GetQueryPostgresql = require('./customJsonQueries')
const initGQLServer = require('./graphql-server.js')

const config = require('./app/config/auth.config.js')
// set env variables for config when found
if (process.env.JS_JWT_SECRET) config.secret = process.env.JS_JWT_SECRET
// The default secret is public (it is in the repository): never sign tokens with it.
// Without a configured secret use a random one, valid until the next restart.
if (config.secret === 'json-scada-secret-key') {
  config.secret = crypto.randomBytes(48).toString('hex')
  Log.log(
    'WARNING: JS_JWT_SECRET is not set. Using a random secret: users will have to sign in again after each restart.'
  )
}
if (process.env.JS_LDAP_ENABLED)
  config.ldap.enabled = process.env.JS_LDAP_ENABLED.toLowerCase() == 'true'
if (process.env.JS_LDAP_URL) config.ldap.url = process.env.JS_LDAP_URL
if (process.env.JS_LDAP_BIND_DN)
  config.ldap.bindDN = process.env.JS_LDAP_BIND_DN
if (process.env.JS_LDAP_BIND_CREDENTIALS)
  config.ldap.bindCredentials = process.env.JS_LDAP_BIND_CREDENTIALS
if (process.env.JS_LDAP_SEARCH_BASE)
  config.ldap.searchBase = process.env.JS_LDAP_SEARCH_BASE
if (process.env.JS_LDAP_SEARCH_FILTER)
  config.ldap.searchFilter = process.env.JS_LDAP_SEARCH_FILTER
if (process.env.JS_LDAP_ATTRIBUTES_USERNAME)
  config.ldap.attributes.username = process.env.JS_LDAP_ATTRIBUTES_USERNAME
if (process.env.JS_LDAP_ATTRIBUTES_EMAIL)
  config.ldap.attributes.email = process.env.JS_LDAP_ATTRIBUTES_EMAIL
if (process.env.JS_LDAP_ATTRIBUTES_DISPLAYNAME)
  config.ldap.attributes.displayName =
    process.env.JS_LDAP_ATTRIBUTES_DISPLAYNAME
if (process.env.JS_LDAP_GROUP_SEARCH_BASE)
  config.ldap.groupSearchBase = process.env.JS_LDAP_GROUP_SEARCH_BASE
if (process.env.JS_LDAP_TLS_REJECT_UNAUTHORIZED)
  config.ldap.tlsOptions.rejectUnauthorized =
    process.env.JS_LDAP_TLS_REJECT_UNAUTHORIZED.toLowerCase() == 'true'
if (process.env.JS_LDAP_TLS_CA)
  config.ldap.tlsOptions.ca = [process.env.JS_LDAP_TLS_CA]
if (process.env.JS_LDAP_TLS_CERT)
  config.ldap.tlsOptions.cert = process.env.JS_LDAP_TLS_CERT
if (process.env.JS_LDAP_TLS_KEY)
  config.ldap.tlsOptions.key = process.env.JS_LDAP_TLS_KEY
if (process.env.JS_LDAP_TLS_PASSPHRASE)
  config.ldap.tlsOptions.passphrase = process.env.JS_LDAP_TLS_PASSPHRASE
if (process.env.JS_LDAP_TLS_PFX)
  config.ldap.tlsOptions.pfx = process.env.JS_LDAP_TLS_PFX
if (process.env.JS_LDAP_TLS_CRL)
  config.ldap.tlsOptions.crl = process.env.JS_LDAP_TLS_CRL
if (process.env.JS_LDAP_TLS_CIPHERS)
  config.ldap.tlsOptions.ciphers = process.env.JS_LDAP_TLS_CIPHERS
if (process.env.JS_LDAP_TLS_SECURE_PROTOCOL)
  config.ldap.tlsOptions.secureProtocol =
    process.env.JS_LDAP_TLS_SECURE_PROTOCOL
if (process.env.JS_LDAP_TLS_MIN_VERSION)
  config.ldap.tlsOptions.minVersion = process.env.JS_LDAP_TLS_MIN_VERSION
if (process.env.JS_LDAP_TLS_MAX_VERSION)
  config.ldap.tlsOptions.maxVersion = process.env.JS_LDAP_TLS_MAX_VERSION
if (process.env.JS_LDAP_GROUP_MAPPING) {
  try {
    config.ldap.groupMapping = JSON.parse(process.env.JS_LDAP_GROUP_MAPPING)
  } catch (e) {
    Log.log('Error parsing JS_LDAP_GROUP_MAPPING: ' + e.message)
  }
}

if (process.env.JS_LDAP_MAX_SESSION_HOURS) {
  const h = parseFloat(process.env.JS_LDAP_MAX_SESSION_HOURS)
  if (Number.isFinite(h) && h > 0) config.ldap.maxSessionHours = h
  else Log.log('Invalid JS_LDAP_MAX_SESSION_HOURS, ignored')
}

Log.log('LDAP enabled: ' + config.ldap.enabled)
if (config.ldap.enabled && !config.ldap.url.startsWith('ldaps')) {
  Log.log('LDAP authentication - Not Encrypted!')
}
// put all keys of config.ldap.groupMapping object in lower case
config.ldap.groupMapping = Object.keys(config.ldap.groupMapping).reduce(
  (acc, key) => {
    acc[key.toLowerCase()] = config.ldap.groupMapping[key]
    return acc
  },
  {}
)

const dbAuth = require('./app/models')
const { authJwt } = require('./app/middlewares')
const UserRights = require('./app/services/user-rights')
const { clientAddress } = require('./app/services/client-address')
const {
  findCommandPoint,
  commandRefusal,
  canSeeCommand,
} = require('./app/services/command-guard')

// A point id names a point key (_id) only when it is a number or a string of
// digits: '22KV_BRK' is a tag name, not key 22 (parseInt would say 22).
function asPointKey(id) {
  if (typeof id === 'number') return Number.isFinite(id) ? id : null
  if (typeof id === 'string' && /^-?\d+$/.test(id.trim())) return Number(id.trim())
  return null
}

// errors of writes nobody waits for (events, range checks) must not go unhandled
function logBgError(e) {
  Log.log('Background write error: ' + (e?.message || e))
}

// last chance to store the queued user actions when the process ends
let flushOnExit = async () => {}
let httpServerRef = null // set once listening
const ShutdownGraceMs = 5000 // time given to requests in progress on a stop
let exiting = false
async function exitAfterFlush(code, graceful = false) {
  if (exiting) return
  exiting = true
  // a stop: no new connections, the requests in progress may finish
  if (graceful && httpServerRef) {
    const closed = new Promise((resolve) => httpServerRef.close(() => resolve()))
    httpServerRef.closeIdleConnections?.()
    await Promise.race([
      closed,
      new Promise((resolve) => setTimeout(resolve, ShutdownGraceMs)),
    ])
  }
  try {
    await Promise.race([
      flushOnExit(),
      new Promise((resolve) => setTimeout(resolve, 3000)),
    ])
  } catch (e) {
    /* exiting anyway */
  }
  process.exit(code)
}

// After an uncaught exception the process state is unknown: log it and exit, the
// process manager (supervisor/nssm) restarts the server.
process.on('uncaughtException', (err) => {
  Log.log('Uncaught Exception:' + err.message + ' ' + err.stack)
  setTimeout(() => process.exit(1), 4000).unref()
  exitAfterFlush(1)
})
// a rejected promise nobody handled is an I/O error, the process state is intact
process.on('unhandledRejection', (reason) => {
  Log.log('Unhandled rejection: ' + (reason?.stack || reason))
})
for (const sig of ['SIGTERM', 'SIGINT'])
  process.on(sig, () => {
    Log.log('Received ' + sig + ', exiting')
    exitAfterFlush(0, true)
  })

const DoInsertCommandAsSOE = true
const CommandSentAsSOESymbol = '⚙️➡️'
const opcIdTypeNumber = 0
const opcIdTypeString = 1
const beepPointKey = -1
const EventsRemoveGuardSeconds = 20 // guard not to remove events recently added
const MaxHistoryRowsPerNode = 10000 // max historian rows returned per requested tag
const MaxHistoryNodes = 1000 // max NodesToRead in a historian (values) read
const MaxHistorySamples = 200000 // max samples in one historian read response
const MaxEventRows = 100000 // max rows returned by an events (soeData) query
const UserActionsFlushMs = 1000 // interval to write the queued user actions to mongodb
const UserActionsBatch = 500 // user actions written per insert
const MaxFilteredReadRows = 100000 // max points returned by a ContentFilter read
// fields of the points returned by a ContentFilter read
const FilteredReadFields = Object.fromEntries(
  ['_id', 'tag', 'type', 'value', 'valueString', 'valueDefault', 'invalid',
    'timeTag', 'timeTagAtSource', 'timeTagAtSourceOk', 'timeTagAlarm',
    'alarmed', 'alarmRange', 'alarmState', 'alarmDisabled', 'frozen', 'isEvent',
    'transit', 'group1', 'group2', 'group3', 'description',
    'ungroupedDescription', 'hiLimit', 'loLimit', 'hysteresis',
    'stateTextTrue', 'stateTextFalse', 'unit', 'annotation', 'notes',
    'commandOfSupervised', 'supervisedOfCommand', 'origin', 'priority'].map(
    (f) => [f, 1]
  )
)
// fields of a point read before a property write (the rest is not needed)
const PropertyWriteFields = Object.fromEntries(
  ['_id', 'tag', 'type', 'group1', 'value', 'description', 'unit', 'priority',
    'annotation', 'notes', 'alarmDisabled', 'loLimit', 'hiLimit', 'hysteresis',
    'eventTextTrue', 'eventTextFalse'].map((f) => [f, 1])
)

const jsConfig = LoadConfig()
let HintMongoIsConnected = true

let db = null // native db handle of the mongoose connection (set once connected)
let pool = null

// credentials of a connection string must not reach the log
function maskConnectionString(cs) {
  return String(cs).replace(/\/\/[^@/]*@/, '//***@')
}

;(async () => {
  // reverse proxy for the node-red editor (mounted on /nodered, ws upgrade below)
  const noderedProxy = createNoderedProxy(NODERED_SERVER)

  // reverse proxy for the n8n editor (mounted on /n8n, ws upgrade below)
  const n8nProxy = createN8nProxy(N8N_SERVER)

  // reverse proxy for the log.io ui socket.io (mounted on /socket.io, ws upgrade below)
  const logioProxy = createLogioProxy(LOGIO_SERVER)

  // reverse proxy for the supervisord web interface (mounted on /supervisor)
  const supervisorProxy = createSupervisorProxy(SUPERVISOR_SERVER)

  // JWT Auth Mongo Express https://bezkoder.com/node-js-mongodb-auth-jwt/
  Log.log('Connecting to ' + maskConnectionString(jsConfig.mongoConnectionString))
  dbAuth.mongoose
    .connect(jsConfig.mongoConnectionString, jsConfig.MongoConnectionOptions)
    .then(() => {
      Log.log('Successfully connect to MongoDB (via mongoose).')
      // one connection pool for everything: the native handle of mongoose
      db = dbAuth.mongoose.connection.db
      HintMongoIsConnected = true
    })
    .catch((err) => {
      console.error('Connection error', err)
      process.exit(1) // a failure: the process manager restarts the server
    })

  const PROXIED_PREFIXES = ['/grafana', '/metabase', '/nodered', '/n8n', '/log-io', '/supervisor']
  app.disable('x-powered-by')
  // security headers of the pages and data of this server (proxied apps such
  // as Grafana keep their own framing policy); JS_ALLOW_FRAMING=true lets
  // other sites embed the pages in frames
  app.use((req, res, next) => {
    res.setHeader('X-Content-Type-Options', 'nosniff')
    res.setHeader('Referrer-Policy', 'same-origin')
    if (
      !ALLOW_FRAMING &&
      !PROXIED_PREFIXES.some((p) => req.path === p || req.path.startsWith(p + '/'))
    )
      res.setHeader('X-Frame-Options', 'SAMEORIGIN')
    next()
  })
  app.use(cookieParser())
  app.options(OPCAPI_AP, cors()) // enable pre-flight request

  // Request bodies are buffered in memory, so the size limit is small by default.
  // Only the routes that really carry big bodies get a big limit, and only for
  // callers holding a valid token (checked before anything is buffered).
  const UPLOAD_ROUTES = [
    OPCAPI_AP + 'auth/importProject', // project zip
    OPCAPI_AP + 'auth/cimEms/uploadModel', // CGMES zip
  ]
  const BIG_BODY_ROUTES = [
    ...UPLOAD_ROUTES,
    OPCAPI_AP + 'auth/saveDisplay', // svg display
    OPCAPI_AP + 'auth/cimEms/request', // binding sets
  ]
  const BigBodyLimit = '100mb'
  const SmallBodyLimit = '2mb'
  const jsonSmall = express.json({ limit: SmallBodyLimit })
  const jsonBig = express.json({ limit: BigBodyLimit })
  const urlSmall = express.urlencoded({ limit: SmallBodyLimit, extended: true })
  const urlBig = express.urlencoded({ limit: BigBodyLimit, extended: true })
  const requireValidToken = (req, res, next) => {
    if (authJwt.checkToken(req) === false)
      return res
        .status(401)
        .send({ ok: false, message: 'Access not allowed. No token provided' })
    next()
  }
  // multipart uploads (the file size limit is enforced by express-fileupload).
  // Files go to temporary files, not memory (several 100 MB uploads at once
  // would use that much RAM); whatever a handler did not move away is removed
  // when the response ends.
  const removeUploadTempFiles = (req, res, next) => {
    let done = false
    const cleanup = () => {
      if (done) return
      done = true
      for (const f of Object.values(req.files || {}).flat())
        if (f?.tempFilePath)
          fs.unlink(f.tempFilePath, (e) => {
            if (e && e.code !== 'ENOENT')
              Log.log('Could not remove upload ' + f.tempFilePath + ': ' + e.message)
          })
    }
    res.on('finish', cleanup)
    res.on('close', cleanup)
    next()
  }
  app.use(
    UPLOAD_ROUTES,
    requireValidToken,
    removeUploadTempFiles,
    fileUpload({
      createParentPath: true,
      useTempFiles: true,
      tempFileDir: UPLOAD_TMP_DIR,
      limits: {
        fileSize: 100 * 1024 * 1024,
        fieldSize: 1024 * 1024,
      },
    })
  )
  app.use((req, res, next) => {
    // the GraphQL endpoint has its own body parser; the reverse proxied apps get
    // their request streams untouched
    if (
      [GRAPHQL_AP, ...PROXIED_PREFIXES].some(
        (p) => req.path === p || req.path.startsWith(p + '/')
      )
    )
      return next()
    const big = BIG_BODY_ROUTES.includes(req.path)
    if (big && authJwt.checkToken(req) === false) return requireValidToken(req, res, next)
    const json = big ? jsonBig : jsonSmall
    const url = big ? urlBig : urlSmall
    json(req, res, (err) => (err ? next(err) : url(req, res, next)))
  })
  initGQLServer(app, dbAuth, true).catch((err) =>
    Log.log('Error starting GraphQL server: ' + err.message)
  )

  const httpServer = app.listen(HTTP_PORT, IP_BIND, () => {
    Log.log('listening on ' + HTTP_PORT)
  })
  httpServerRef = httpServer

  // websocket upgrades do not pass through the express middlewares, proxy them here
  attachNoderedUpgrade(httpServer, noderedProxy)
  attachN8nUpgrade(httpServer, n8nProxy)
  attachLogioUpgrade(httpServer, logioProxy)

  // if env variables defined use them, if not set local defaults
  if (pool == null) pool = getPgPool() // shared with the GraphQL server

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
    METABASE_SERVER,
    noderedProxy,
    supervisorProxy,
    n8nProxy
  )

  // Last error handler: a JSON answer without internals (no stack traces)
  app.use((err, req, res, next) => {
    const status = Number(err?.status || err?.statusCode) || 500
    if (status >= 500) Log.log('Request error ' + req.originalUrl + ': ' + (err?.stack || err))
    if (res.headersSent) return next(err)
    res.status(status).send({
      ok: false,
      error: status >= 500 ? 'Internal server error' : err?.expose ? err.message : 'Bad request',
    })
  })

  async function queryJSON(req, res) {
    let queryname = typeof req.query?.query === 'string' ? req.query.query : ''
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

    if (!pool) {
      res.send({ error: 'Database not connected!' })
      return
    }

    // read data from postgreSQL
    pool.query(query, async (err, resp) => {
      if (err) {
        res.send({ error: 'Query error!' })
        return
      }

      // a user restricted to some group1 (stations) only gets rows of its points
      const g1 = req.rights?.group1List
      if (!Array.isArray(g1) || g1.length === 0) {
        res.send(resp.rows)
        return
      }
      try {
        const rowTags = [
          ...new Set(
            resp.rows.map((r) => r?.tag).filter((t) => typeof t === 'string')
          ),
        ]
        const allowed = new Set(
          rowTags.length === 0 || !db
            ? []
            : (
                await db
                  .collection(COLL_REALTIME)
                  .find(
                    { tag: { $in: rowTags }, group1: { $in: g1 } },
                    { projection: { tag: 1 } }
                  )
                  .toArray()
              ).map((d) => d.tag)
        )
        res.send(resp.rows.filter((r) => allowed.has(r?.tag)))
      } catch (e) {
        Log.log('queryJSON filter error: ' + e.message)
        res.send({ error: 'Query error!' })
      }
    })
  }

  // find file on mongodb gridfs and return it
  async function getFileApi(req, res) {
    const str = (v, dflt) => (typeof v === 'string' ? v : dflt)
    let filename = str(req.query?.name, '')
    let bucketName = str(req.query?.bucket, 'fs')
    // the content type comes from the file name; `mime` may only choose a
    // passive type (images, media, text, pdf): never html, svg, xml or script
    let mimeType = path.basename(filename)
    const mimeParam = str(req.query?.mime, '').trim().toLowerCase()
    if (mimeParam !== '' && SAFE_GETFILE_MIME_RE.test(mimeParam)) mimeType = mimeParam
    let refresh = str(req.query?.refresh, '')

    res.setHeader('Content-type', 'application/json')
    if (filename.trim() === '') {
      res.send({ error: 'Parameter [name] empty or not specified' })
      return
    }
    // CIM-EMS models and study results are confidential: they are served only
    // by the cimEms/artifact route, which checks the engineering right.
    if (bucketName.trim().toLowerCase().startsWith('cimartifacts')) {
      res.send({ error: 'File not found' })
      return
    }
    if (!db) {
      res.send({ error: 'Database disconnected!' })
      return
    }
    try {
      let gfs = new GridFSBucket(db, { bucketName: bucketName })
      let f = await gfs.find({ filename: filename }, { limit: 1 }).toArray()
      if (f.length === 0) {
        Log.log('File not found ' + filename)
        res.send({ error: 'File not found' })
        return
      }
      let readstream = gfs.openDownloadStreamByName(filename)
      readstream.on('error', (e) => {
        Log.log('File read error: ' + filename + ' ' + e.message)
        if (!res.headersSent) res.status(500)
        res.end()
      })
      res.type(mimeType)
      // stored files come from many sources: active content (html, svg, xml,
      // scripts) runs sandboxed, without access to the session of this site
      if (/html|svg|xml|javascript/i.test(res.get('Content-Type') || ''))
        res.setHeader('Content-Security-Policy', 'sandbox')
      res.setHeader(
        'Content-disposition',
        'inline; filename="' + filename.replace(/["\r\n\\]/g, '_') + '"'
      )
      if (refresh) res.setHeader('Refresh', refresh.replace(/[\r\n]/g, ''))
      readstream.pipe(res)
    } catch (e) {
      Log.log('File not found: ' + filename + ' ' + e.message)
      res.setHeader('Content-type', 'application/json')
      res.send({ error: 'File not found' })
    }
  }

  // OPC WEB HMI API
  async function opcApi(req, res) {
    let tini = new Date().getTime()
    if (!req.body || typeof req.body !== 'object') req.body = {}

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
      // rights as they are now in the database (verifyToken already loaded them)
      let current = req.rights || null
      if (!current) {
        try {
          current = await authJwt.getCurrentRights(rslt)
        } catch (e) {
          current = undefined // database not available
        }
      }
      if (!current) {
        OpcResp.ServiceId = opc.ServiceCode.ServiceFault
        const code =
          current === undefined
            ? opc.StatusCode.BadServerNotConnected
            : opc.StatusCode.BadIdentityTokenRejected
        OpcResp.Body.ResponseHeader.ServiceResult = code
        OpcResp.Body.ResponseHeader.StringTable = [
          opc.getStatusCodeName(code),
          opc.getStatusCodeText(code),
          current === undefined
            ? 'Database disconnected!'
            : 'Access denied (session ended or user not found)!',
        ]
        res.send(OpcResp)
        return
      }
      userRights = current
      if (!Array.isArray(userRights.group1List)) userRights.group1List = []
    }

    // rights for commands are always read afresh, once per request
    let freshRights = null
    const getFreshRights = async () => {
      if (freshRights === null) {
        try {
          freshRights =
            (await authJwt.getCurrentRights(rslt, { fresh: true })) || {}
        } catch (e) {
          freshRights = {} // database not available: no commands
        }
      }
      return freshRights
    }
    // restriction of the user to some group1 (stations)
    const group1Restriction =
      userRights.group1List.length > 0
        ? { group1: { $in: userRights.group1List } }
        : {}

    if (!db || !HintMongoIsConnected) {
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
          if (!Array.isArray(req.body.Body.NodesToWrite)) {
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
            if (!node || typeof node !== 'object') continue
            if (!node.Value || typeof node.Value !== 'object') node.Value = {}
            // point ids are numbers or strings, anything else would reach the queries
            if (
              node.NodeId &&
              typeof node.NodeId === 'object' &&
              'Id' in node.NodeId &&
              typeof node.NodeId.Id !== 'string' &&
              typeof node.NodeId.Id !== 'number'
            ) {
              OpcResp.Body.Results.push(opc.StatusCode.BadNodeIdInvalid)
              continue
            }
            if ('AttributeId' in node) {
              if (node.AttributeId == opc.AttributeId.ExtendedAlarmEventsAck) {
                // alarm / event ack

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
                  Log.log(`User has no right to ack/remove events! [${username}]`)
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

                let findPoint = null
                if (node.NodeId?.IdType === opcIdTypeNumber) {
                  findPoint = {
                    _id: asPointKey(node.NodeId.Id),
                    ...group1Restriction,
                  }
                } else if (node.NodeId?.IdType === opcIdTypeString) {
                  findPoint = {
                    tag: String(node.NodeId.Id),
                    ...group1Restriction,
                  }
                }

                // actions on one point or one event need a valid point/event id
                const ackBits = Number(node.Value.Body) || 0
                const Ack = opc.Acknowledge
                let eventObjectId = null
                if (ackBits & (Ack.RemoveOneEvent | Ack.AckOneEvent)) {
                  const ev = node._Properties?.event_id
                  if (typeof ev === 'string' && ObjectId.isValid(ev))
                    eventObjectId = new ObjectId(ev)
                }
                if (
                  (findPoint === null &&
                    ackBits &
                      (Ack.AckOneAlarm |
                        Ack.RemovePointEvents |
                        Ack.AckPointEvents |
                        Ack.RemoveOneEvent |
                        Ack.AckOneEvent)) ||
                  (eventObjectId === null &&
                    ackBits & (Ack.RemoveOneEvent | Ack.AckOneEvent))
                ) {
                  OpcResp.Body.ResponseHeader.ServiceResult =
                    opc.StatusCode.BadNodeIdInvalid
                  OpcResp.Body.ResponseHeader.StringTable = [
                    opc.getStatusCodeName(opc.StatusCode.BadNodeIdInvalid),
                    opc.getStatusCodeText(opc.StatusCode.BadNodeIdInvalid),
                    'Missing or invalid point or event id!',
                  ]
                  res.send(OpcResp)
                  return
                }

                if (node.Value.Body & opc.Acknowledge.RemoveAllEvents) {
                  Log.log('Remove All Events')
                  let fromDate = new Date(
                    Date.now() - EventsRemoveGuardSeconds * 1000
                  )
                  let afterDate = new Date(
                    Date.now() + EventsRemoveGuardSeconds * 1000
                  )
                  let result = await db.collection(COLL_SOE).updateMany(
                    {
                      ack: { $lte: 1 },
                      $or: [
                        { timeTag: { $gte: afterDate } },
                        { timeTag: { $lte: fromDate } },
                      ],
                      ...(userRights?.group1List?.length == 0
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
                      ...(userRights?.group1List?.length == 0
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
                  let afterDate = new Date(
                    Date.now() + EventsRemoveGuardSeconds * 1000
                  )
                  let result = await db.collection(COLL_SOE).updateMany(
                    {
                      tag: node.NodeId.Id,
                      ack: { $lte: 1 },
                      $or: [
                        { timeTag: { $gte: afterDate } },
                        { timeTag: { $lte: fromDate } },
                      ],
                      ...(userRights?.group1List?.length == 0
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
                      ...(userRights?.group1List?.length == 0
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
                      _id: eventObjectId,
                      ack: { $lte: 1 },
                      ...(userRights?.group1List?.length == 0
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
                    eventId: eventObjectId,
                    timeTag: new Date(),
                  })
                } else if (node.Value.Body & opc.Acknowledge.AckOneEvent) {
                  Log.log('Ack One Event: ' + node.NodeId.Id)
                  await db.collection(COLL_SOE).updateMany(
                    {
                      _id: eventObjectId,
                      ack: 0,
                      ...(userRights?.group1List?.length == 0
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
                    eventId: eventObjectId,
                    timeTag: new Date(),
                  })
                }

                if (node.Value.Body & opc.Acknowledge.AckAllAlarms) {
                  Log.log('Ack All Alarms')
                  let result = await db.collection(COLL_REALTIME).updateMany(
                    {
                      alarmed: true,
                      ...group1Restriction,
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
                          ...(userRights?.group1List?.length == 0
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
                          timeTagAtSourceOk: null,
                        },
                      },
                    ],
                    { updatePipeline: true }
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
                          timeTagAtSourceOk: null,
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
                  if (userRights.group1List.length > 0) {
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
                  'Write. Elapsed ' + (new Date().getTime() - tini) + ' ms',
              Log.levelDetailed
            )
                return
              }

              if (node.AttributeId == opc.AttributeId.Value) {
                // Write a Value: Command

                // go check user right for command in mongodb (not just in the token for better security)
                const cmdRights = await getFreshRights()
                if (!cmdRights.sendCommands) {
                  // ACTION DENIED
                  Log.log(`User has no right to issue commands! [${username}]`)
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

                if ('NodeId' in node)
                  if ('Id' in node.NodeId) {
                    if (
                      typeof node.Value !== 'object' ||
                      typeof node.Value.Type !== 'number' ||
                      (node.Value.Type !== opc.DataType.Double &&
                        node.Value.Type !== opc.DataType.String) || // only accepts a double or string value for the command
                      (typeof node.Value.Body !== 'number' &&
                        typeof node.Value.Body !== 'string') ||
                      // a numeric command carries a finite number
                      (node.Value.Type === opc.DataType.Double &&
                        (typeof node.Value.Body !== 'number' ||
                          !Number.isFinite(node.Value.Body)))
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

                    const cmd_id = node.NodeId.Id
                    const cmd_val =
                      node.Value.Type === opc.DataType.Double
                        ? node.Value.Body
                        : 0.0
                    const cmd_val_str =
                      node.Value.Type === opc.DataType.String
                        ? node.Value.Body
                        : parseFloat(cmd_val).toString()
                    const cmd_key = asPointKey(cmd_id)
                    let query = { _id: cmd_key }
                    if (cmd_key === null) {
                      query = { tag: String(cmd_id) }

                      // special command tags that starts with $$ do not need to be found in the database
                      // put it directly in the command queue. A user whose roles
                      // restrict commands to some groups needs '$$' in that list.
                      if (String(cmd_id).startsWith('$$')) {
                        if (!UserRights.canCommandGroup(cmdRights, '$$')) {
                          Log.log(
                            `User has no right to issue special ($$) commands! [${username}]`
                          )
                          OpcResp.Body.ResponseHeader.ServiceResult =
                            opc.StatusCode.BadUserAccessDenied
                          OpcResp.Body.ResponseHeader.StringTable = [
                            opc.getStatusCodeName(opc.StatusCode.BadUserAccessDenied),
                            opc.getStatusCodeText(opc.StatusCode.BadUserAccessDenied),
                            'User has no right to issue special ($$) commands!',
                          ]
                          res.send(OpcResp)
                          return
                        }
                        UserActionsQueue.enqueue({
                          username: username,
                          tag: String(cmd_id),
                          action: 'Command',
                          properties: {
                            value: new Double(cmd_val),
                            valueString: cmd_val_str,
                          },
                          timeTag: new Date(),
                        })
                        let result = await db
                          .collection(COLL_COMMANDS)
                          .insertOne({
                            protocolSourceConnectionNumber: new Double(-1),
                            protocolSourceCommonAddress: new Double(-1),
                            protocolSourceObjectAddress: new Double(-1),
                            protocolSourceASDU: new Double(-1),
                            protocolSourceCommandDuration: new Double(0),
                            protocolSourceCommandUseSBO: false,
                            pointKey: 0,
                            tag: cmd_id,
                            timeTag: new Date(),
                            value: new Double(cmd_val),
                            valueString: cmd_val_str,
                            originatorUserName: username,
                            originatorIpAddress: clientAddress(req),
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
                        OpcResp.Body.ResponseHeader.ServiceResult =
                          opc.StatusCode.Good
                        res.send(OpcResp)
                        return
                      }
                    }

                    // search command in database (primary: a fresh block counts)
                    const data = await findCommandPoint(db, query)

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

                    // check if user has group1 list it can command
                    if (!UserRights.canCommandGroup(cmdRights, data.group1)) {
                      // ACTION DENIED
                      Log.log(
                        `User has no right to issue commands to the group1 destination! [${username}] [${data.group1}]`
                      )
                      OpcResp.Body.ResponseHeader.ServiceResult =
                        opc.StatusCode.BadUserAccessDenied
                      OpcResp.Body.ResponseHeader.StringTable = [
                        opc.getStatusCodeName(opc.StatusCode.BadUserAccessDenied),
                        opc.getStatusCodeText(opc.StatusCode.BadUserAccessDenied),
                        'User has no right to issue commands to the group1 destination!',
                      ]
                      res.send(OpcResp)
                      return
                    } else {
                      Log.log(
                        `User authorized to issue commands to the group1 destination! [${username}]`
                      )
                    }

                    // only command points, and not while blocked by the operator
                    const refusal = await commandRefusal(db, data)
                    if (refusal) {
                      Log.log(`Command refused: ${refusal} [${username}] [${data.tag}]`)
                      OpcResp.Body.ResponseHeader.ServiceResult =
                        opc.StatusCode.BadNotWritable
                      OpcResp.Body.ResponseHeader.StringTable = [
                        opc.getStatusCodeName(opc.StatusCode.BadNotWritable),
                        opc.getStatusCodeText(opc.StatusCode.BadNotWritable),
                        refusal,
                      ]
                      res.send(OpcResp)
                      return
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
                      originatorIpAddress: clientAddress(req),
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
                      }).catch(logBgError)
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
                    // the group1 restriction of the user applies to property writes
                    let findPoint = null
                    if (node.NodeId.IdType === opcIdTypeNumber) {
                      findPoint = { _id: asPointKey(node.NodeId.Id), ...group1Restriction }
                    } else if (node.NodeId.IdType === opcIdTypeString) {
                      findPoint = { tag: String(node.NodeId.Id), ...group1Restriction }
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
                        .findOne(findPoint, { projection: PropertyWriteFields })

                      if (prevData === null) {
                        // not found, or outside the groups of the user
                        OpcResp.Body.Results.push(opc.StatusCode.BadNodeIdUnknown)
                        continue
                      }

                      if (!node.Value || typeof node.Value !== 'object') node.Value = {}

                      let alarmDisableNew = {}
                      if (
                        userRights?.disableAlarms &&
                        node.Value?._Properties?.alarmDisabled !== undefined
                      )
                        if (
                          prevData?.alarmDisabled !==
                          node.Value._Properties?.alarmDisabled
                        )
                          alarmDisableNew = {
                            alarmDisabled:
                              node.Value._Properties?.alarmDisabled,
                          }

                      let annotationNew = {}
                      if (
                        userRights?.enterAnnotations &&
                        node.Value._Properties?.annotation !== undefined
                      )
                        if (
                          prevData?.annotation !==
                          node.Value._Properties?.annotation
                        )
                          annotationNew = {
                            annotation: node.Value._Properties?.annotation,
                          }

                      let loLimitNew = {}
                      // absent fields are left unchanged; a null limit (an
                      // Infinity serialized as JSON) means no limit
                      const props = node.Value?._Properties || {}
                      const limitOf = (v, inf) =>
                        v === null ? inf : Number(v)
                      if (userRights?.enterLimits && props.loLimit !== undefined)
                        if (
                          prevData?.loLimit !== limitOf(props.loLimit, -Infinity)
                        )
                          loLimitNew = {
                            loLimit: new Double(
                              limitOf(props.loLimit, -Infinity)
                            ),
                          }

                      let hiLimitNew = {}
                      if (userRights?.enterLimits && props.hiLimit !== undefined)
                        if (
                          prevData?.hiLimit !== limitOf(props.hiLimit, Infinity)
                        )
                          hiLimitNew = {
                            hiLimit: new Double(limitOf(props.hiLimit, Infinity)),
                          }

                      let hysteresisNew = {}
                      if (
                        userRights?.enterLimits &&
                        props.hysteresis !== undefined
                      )
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
                      if (userRights?.enterNotes && props.notes !== undefined)
                        if (prevData?.notes !== node.Value._Properties?.notes)
                          notesNew = {
                            notes: node.Value._Properties.notes,
                          }

                      let valueNew = {}
                      if ('substituted' in props && 'newValue' in props)
                        if (userRights?.substituteValues)
                          if (prevData?.value !== props.newValue)
                            valueNew = {
                              value: new Double(props.newValue),
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
                            // merge on the server: a copy read earlier could overwrite
                            // what a driver wrote in the meantime
                            db.collection(COLL_REALTIME)
                              .updateOne({ _id: prevData._id }, [
                                {
                                  $set: {
                                    sourceDataUpdate: {
                                      $mergeObjects: [
                                        { $ifNull: ['$sourceDataUpdate', {}] },
                                        { rangeCheck: new Date().getTime() },
                                      ],
                                    },
                                  },
                                },
                              ])
                              .catch((e) =>
                                Log.log('Update for range check error: ' + e.message)
                              )
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
                          }).catch(logBgError)
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
                              Number(valueNew.value).toFixed(2) +
                              ' ' +
                              prevData?.unit
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
                          }).catch(logBgError)
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
          if (
            'NodesToRead' in req.body.Body &&
            !Array.isArray(req.body.Body.NodesToRead)
          ) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.BadNodeIdInvalid
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.BadNodeIdInvalid),
              opc.getStatusCodeText(opc.StatusCode.BadNodeIdInvalid),
              'NodesToRead must be a list',
            ]
            res.send(OpcResp)
            return
          }
          if ('NodesToRead' in req.body.Body)
            req.body.Body.NodesToRead.map((node) => {
              if (!node || typeof node !== 'object') return node
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
              if (node.NodeId && typeof node.NodeId === 'object')
                if (
                  typeof node.NodeId.Id === 'string' ||
                  typeof node.NodeId.Id === 'number'
                ) {
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
              cmdHandles[0].length !== 24 ||
              !ObjectId.isValid(cmdHandles[0])
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
            // another user's command in a station this user cannot see: unknown
            if (
              data !== null &&
              !(await canSeeCommand(db, userRights, username, data))
            )
              data = null

            if (data === null) {
              OpcResp.Body.ResponseHeader.ServiceResult =
                opc.StatusCode.BadNodeIdUnknown
              OpcResp.Body.ResponseHeader.StringTable = [
                opc.getStatusCodeName(opc.StatusCode.BadNodeIdUnknown),
                opc.getStatusCodeText(opc.StatusCode.BadNodeIdUnknown),
                'Command not found!',
              ]
              res.send(OpcResp)
              return
            }
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
              OpcResp.Body.ResponseHeader.StringTable = [data.cancelReason]
            }
            res.send(OpcResp)
            return
          }

          let tags = [],
            npts = []
          for (let i = 0; i < points.length; i++) {
            const key = asPointKey(points[i])
            if (key !== null) npts.push(key)
            else tags.push(String(points[i]))
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
                  notes: 1,
                  transit: 1,
                  origin: 1,
                  group1: 1,
                  beepType: 1,
                  beepGroup1List: 1,
                },
              }

          // JSON-SCADA extension: a NodesToRead entry with `_ValueJson: true`
          // also gets the point's valueJson in `_Properties` (display viewer)
          if (
            projection &&
            Array.isArray(req.body.Body?.NodesToRead) &&
            req.body.Body.NodesToRead.some((n) => n?._ValueJson === true)
          )
            projection.projection.valueJson = 1

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
            projection = { projection: FilteredReadFields }
            query = {}
            let grp1 = null
            let grp2 = null
            const isScalar = (v) =>
              typeof v === 'string' ||
              typeof v === 'number' ||
              typeof v === 'boolean'
            // attribute names are plain field names: no operators ($...) nor paths
            const isFieldName = (v) =>
              typeof v === 'string' && /^[A-Za-z_][A-Za-z0-9_]*$/.test(v)
            ;(Array.isArray(req.body.Body.ContentFilter)
              ? req.body.Body.ContentFilter
              : []
            ).map((contentFilter) => {
              // supports attribute (operand) Equals (operator) to a literal value (operand)
              if (
                contentFilter?.FilterOperator === opc.FilterOperator.Equals &&
                Array.isArray(contentFilter.FilterOperands) &&
                contentFilter.FilterOperands.length >= 2 &&
                isFieldName(contentFilter.FilterOperands[0]?.Value) &&
                isScalar(contentFilter.FilterOperands[1]?.Value)
              ) {
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

          // a filtered query of a user restricted to some stations reads only
          // their points (the database does the filtering, not this process)
          if (
            'ContentFilter' in req.body.Body &&
            userRights.group1List.length > 0
          )
            query = { $and: [query, { group1: { $in: userRights.group1List } }] }

          try {
            const results = await db
              .collection(COLL_REALTIME)
              .find(query, projection)
              .sort(sort)
              .limit('ContentFilter' in req.body.Body ? MaxFilteredReadRows : 0)
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
                // index the points once instead of scanning them for each node
                const byTag = new Map()
                const byId = new Map()
                for (const pt of results) {
                  byTag.set(pt.tag, pt)
                  byId.set(pt._id, pt)
                }
                req.body.Body.NodesToRead.map((node) => {
                  if (!node || typeof node !== 'object') return node
                  let Result = {
                    // will return this if point not found or access denied
                    StatusCode: opc.StatusCode.BadNotFound,
                    NodeId: node.NodeId,
                    Value: null,
                    _Properties: null,
                  }

                  do {
                    const pointInfo =
                      byTag.get(node.NodeId?.Id) ??
                      byId.get(asPointKey(node.NodeId?.Id))

                    if (pointInfo) {
                      // check for group1 list in user rights (from token)
                      if (userRights.group1List.length > 0) {
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
                          group3: pointInfo.group3,
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
                      if (node._ValueJson === true)
                        Result._Properties.valueJson =
                          pointInfo.valueJson ?? null
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
                  } while (false)
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
                      group3: node.group3,
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
                          ? node.timeTagAtSourceOk
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
                  if (userRights.group1List.length > 0) {
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
                  ' ms',
              Log.levelDetailed
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
                          // keep the raw tag name: it is passed as a query
                          // parameter, never concatenated into SQL
                          if (typeof node.NodeId.Id === 'string')
                            tags.push(node.NodeId.Id)
                        }
                }
              }
              return node
            })
            if (req.body.Body.NodesToRead.length > MaxHistoryNodes) {
              OpcResp.Body.ResponseHeader.ServiceResult =
                opc.StatusCode.BadTooManyOperations
              OpcResp.Body.ResponseHeader.StringTable = [
                opc.getStatusCodeName(opc.StatusCode.BadTooManyOperations),
                opc.getStatusCodeText(opc.StatusCode.BadTooManyOperations),
                'Too many NodesToRead (maximum ' + MaxHistoryNodes + ')',
              ]
              res.send(OpcResp)
              return
            }
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
            ) {
              // the row count is bounded: 0, negative or absurd values are not honored
              const n = parseInt(
                req.body.Body.HistoryReadDetails.ParameterData.NumValuesPerNode,
                10
              )
              if (!isNaN(n) && n > 0) limitValues = Math.min(n, MaxEventRows)
            }

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

            // restriction of the user to some group1, applied by the database
            const filterUserGroup = group1Restriction
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
                          filterUserGroup,
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
                        filterUserGroup,
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
                  if (userRights.group1List.length > 0) {
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
                    ' ms',
              Log.levelDetailed
            )
                return
              }
            }
            return
          }

          // convert the requested time range to Date objects, rejecting
          // anything that is not a valid timestamp
          let startTs = new Date(startDateTime)
          let endTs = new Date(endDateTime)
          if (
            typeof startDateTime !== 'string' ||
            typeof endDateTime !== 'string' ||
            isNaN(startTs.getTime()) ||
            isNaN(endTs.getTime())
          ) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.BadHistoryOperationInvalid
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.BadHistoryOperationInvalid),
              opc.getStatusCodeText(opc.StatusCode.BadHistoryOperationInvalid),
              'Invalid StartTime/EndTime',
            ]
            res.send(OpcResp)
            return
          }

          // each requested node gets a result, a repeated node a copy of the
          // same rows: the database reads every tag once, and the row limit
          // below is shared by all the nodes of the request
          const nodeCount = tags.length
          tags = [...new Set(tags)]

          // restrict to tags the user is allowed to see (group1 restriction)
          let allowedTags = tags
          if (userRights?.group1List?.length > 0) {
            try {
              const accessible = await db
                .collection(COLL_REALTIME)
                .find(
                  {
                    tag: { $in: tags },
                    group1: { $in: userRights.group1List },
                  },
                  { projection: { tag: 1 } }
                )
                .toArray()
              const accessibleSet = new Set(accessible.map((d) => d.tag))
              allowedTags = tags.filter((t) => accessibleSet.has(t))
            } catch (err) {
              Log.log('HistoryRead - group1 filter error: ' + err.message)
              allowedTags = []
            }
          }
          if (allowedTags.length === 0) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.GoodNoData
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.GoodNoData),
              opc.getStatusCodeText(opc.StatusCode.GoodNoData),
              'No accessible NodeId.Ids requested',
            ]
            res.send(OpcResp)
            return
          }

          // cap the number of rows returned by the historian
          let rowLimit = parseInt(limitValues, 10)
          if (isNaN(rowLimit) || rowLimit < 1) rowLimit = MaxHistoryRowsPerNode
          // a limit for each tag: one busy tag cannot take the rows of the others
          rowLimit = Math.min(rowLimit, MaxHistoryRowsPerNode)
          // and a limit for the whole response (repeated nodes count again)
          rowLimit = Math.max(
            1,
            Math.min(rowLimit, Math.floor(MaxHistorySamples / nodeCount))
          )

          // all request-supplied values below are bound as query parameters
          // ($1..$n): never build this query by string concatenation
          let query =
            'SELECT h.tag, h.value, h.flags, h.time_tag, h.time_tag_at_source ' +
            'FROM unnest($3::text[]) AS t(tag) ' +
            'CROSS JOIN LATERAL (' +
            'SELECT tag, value, flags, time_tag, time_tag_at_source ' +
            'FROM hist ' +
            'WHERE hist.tag = t.tag AND time_tag>=$1 AND time_tag<=$2 ' +
            'ORDER BY time_tag ASC LIMIT $4' +
            ') AS h ' +
            'ORDER BY h.tag ASC, h.time_tag ASC'
          let queryParams = [startTs, endTs, allowedTags, rowLimit]

          if (startTs.getTime() === endTs.getTime()) {
            query =
              'SELECT tag as tag, ' +
              'last(value, time_tag) as value, ' +
              'last(flags, time_tag) as flags, ' +
              'last(time_tag, time_tag) as time_tag, ' +
              'last(time_tag_at_source, time_tag) as time_tag_at_source ' +
              'from hist ' +
              'where ' +
              'time_tag<=$1 and ' +
              "time_tag> ($1::timestamptz - INTERVAL '0.5 day') and " +
              'tag = ANY($2) group by tag'
            queryParams = [startTs, allowedTags]
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
          pool.query(query, queryParams, (err, resp) => {
           try {
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
            // group the rows by tag once instead of scanning them for each node
            const rowsByTag = new Map()
            for (const row of resp.rows) {
              let list = rowsByTag.get(row.tag)
              if (!list) rowsByTag.set(row.tag, (list = []))
              list.push(row)
            }
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
                          ;(rowsByTag.get(HistoryReadResult.NodeId.Id) || []).map((node) => {
                            {
                              HistoryReadResult.StatusCode = opc.StatusCode.Good
                              HistoryReadResult.HistoryData.push({
                                Value: {
                                  Type:
                                    String(node.flags ?? '').charAt(2) === '0'
                                      ? opc.DataType.Boolean
                                      : opc.DataType.Double,
                                  Body:
                                    String(node.flags ?? '').charAt(2) === '0'
                                      ? node.value === 0
                                        ? false
                                        : true
                                      : node.value,
                                  Quality:
                                    String(node.flags ?? '').charAt(0) === '0'
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
                ' ms',
              Log.levelDetailed
            )
           } catch (e) {
            Log.log('HistoryRead error: ' + e.message)
            if (!res.headersSent) res.status(500).send({ error: 'HistoryRead error' })
           }
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
            if (userRights.group1List.length > 0) {
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
              ' ms',
              Log.levelDetailed
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

  // Write the queued user actions (audit trail) to mongodb. An action leaves the
  // queue only after it was stored: when the database is not available they wait.
  let flushingActions = false
  async function flushUserActions() {
    if (flushingActions || !db || !HintMongoIsConnected) return
    flushingActions = true
    try {
      while (!UserActionsQueue.isEmpty()) {
        const batch = UserActionsQueue.peekMany(UserActionsBatch)
        let stored = batch.length
        try {
          // ordered: on an error, the documents before the failing one are stored
          await db.collection(COLL_ACTIONS).insertMany(batch, { ordered: true })
        } catch (err) {
          const firstError = Array.isArray(err?.writeErrors)
            ? err.writeErrors[0]
            : err?.writeErrors
          const inserted = Number(err?.insertedCount) || 0
          // a duplicate means a previous try did store that one
          if ((firstError?.code ?? err?.code) === 11000) stored = inserted + 1
          else {
            UserActionsQueue.ack(batch.slice(0, inserted))
            Log.log('User actions not stored, will retry: ' + err.message)
            break
          }
        }
        // acknowledged by identity: an overflow during the insert may have
        // dropped some of these from the queue already
        UserActionsQueue.ack(batch.slice(0, stored))
      }
    } finally {
      flushingActions = false
    }
  }
  setInterval(flushUserActions, UserActionsFlushMs)
  flushOnExit = async () => {
    while (flushingActions) await new Promise((r) => setTimeout(r, 50))
    await flushUserActions()
  }

  // watch the mongodb connection (the driver reconnects by itself)
  for (;;) {
    await new Promise((resolve) => setTimeout(resolve, 5000))
    try {
      await checkConnectedMongo(dbAuth.mongoose.connection.getClient())
    } catch (e) {
      HintMongoIsConnected = false
      Log.log(e)
    }
  }
})()

// test mongoDB connectivity
async function checkConnectedMongo(client) {
  if (!client) {
    HintMongoIsConnected = false
    return false
  }
  const CheckMongoConnectionTimeout = 10000
  let tr = setTimeout(() => {
    Log.log('Mongo ping timeout error!')
    HintMongoIsConnected = false
    UserRights.setDatabaseHealthy(false)
  }, CheckMongoConnectionTimeout)

  let res = null
  try {
    res = await client.db('admin').command({ ping: 1 })
  } catch (e) {
    Log.log('Error on mongodb connection!')
    HintMongoIsConnected = false
    UserRights.setDatabaseHealthy(false)
    return false
  } finally {
    clearTimeout(tr)
  }
  HintMongoIsConnected = !!(res && 'ok' in res && res.ok)
  UserRights.setDatabaseHealthy(HintMongoIsConnected)
  return HintMongoIsConnected
}
