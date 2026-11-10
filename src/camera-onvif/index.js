/*
 * ONVIF/RTSP camera driver for JSON-SCADA.
 *
 * Connects to ONVIF cameras (or plain RTSP streams), transcodes the video
 * with ffmpeg to MPEG1 and serves it over WebSocket (JSMpeg protocol) for
 * browser viewing. PTZ commands are consumed from the commandsQueue
 * collection using tags in the form $$ConnectionName$$command$$variable.
 * Periodic JPEG snapshots can be captured via the camera snapshot service.
 *
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
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

const fs = require('fs')
const path = require('path')
const crypto = require('crypto')
const { MongoClient } = require('mongodb')
const { Cam } = require('onvif/promises')
const Stream = require('node-rtsp-stream')
const AppDefs = require('./app-defs')
const Log = require('./simple-logger')
const LoadConfig = require('./load-config')
const Redundancy = require('./redundancy')

const RETRY_DELAY_MS = 10000 // delay to retry camera/stream connection
const SUPERVISE_INTERVAL_MS = 5000 // active/inactive state check period
const STATS_INTERVAL_MS = 30000 // connection stats update period
const MAX_COMMAND_AGE_MS = 10000 // discard commands older than this
const DEFAULT_TIMEOUT_MS = 5000
const DEFAULT_FFMPEG_OPTIONS = { '-r': 30, '-s': '320x240' }

process.on('uncaughtException', (err) =>
  Log.log('Uncaught Exception: ' + (err.stack || err))
)
process.on('unhandledRejection', (reason) =>
  Log.log('Unhandled Rejection: ' + (reason?.stack || reason))
)

const jsConfig = LoadConfig()
Log.levelCurrent = jsConfig.LogLevel

const MongoStatus = { HintMongoIsConnected: false }
const FfmpegPath = resolveFfmpegPath()
let Connections = [] // protocolConnections docs decorated with runtime state
let CamerasStarted = false
let ShuttingDown = false

const csCmdPipeline = [
  {
    $project: { documentKey: false },
  },
  {
    $match: {
      operationType: 'insert',
    },
  },
]

;(async () => {
  let clientMongo = null
  let cmdCollection = null
  let connsCollection = null
  let changeStreamCmd = null
  let superviseTimer = null
  let statsTimer = null

  const shutdown = () => {
    if (ShuttingDown) return
    ShuttingDown = true
    Log.log('Process - Shutting down...')
    stopAllCameras()
    try {
      if (changeStreamCmd) changeStreamCmd.close()
    } catch (e) {}
    try {
      if (clientMongo) clientMongo.close()
    } catch (e) {}
    setTimeout(() => process.exit(0), 500)
  }
  process.on('SIGINT', shutdown)
  process.on('SIGTERM', shutdown)

  const teardownMongo = () => {
    MongoStatus.HintMongoIsConnected = false
    stopAllCameras()
    CamerasStarted = false
    clearInterval(superviseTimer)
    clearInterval(statsTimer)
    try {
      if (changeStreamCmd) changeStreamCmd.close()
    } catch (e) {}
    changeStreamCmd = null
    try {
      if (clientMongo) clientMongo.close()
    } catch (e) {}
    clientMongo = null
  }

  while (!ShuttingDown) {
    try {
      if (clientMongo === null) {
        Log.log('MongoDB - Connecting to MongoDB server...', Log.levelMin)
        clientMongo = await MongoClient.connect(
          jsConfig.mongoConnectionString,
          jsConfig.MongoConnectionOptions
        )
        MongoStatus.HintMongoIsConnected = true
        Log.log('MongoDB - Connected correctly to MongoDB server', Log.levelMin)

        const db = clientMongo.db(jsConfig.mongoDatabaseName)
        cmdCollection = db.collection(jsConfig.CommandsQueueCollectionName)
        connsCollection = db.collection(
          jsConfig.ProtocolConnectionsCollectionName
        )

        // find enabled connections for this driver instance
        Connections = await connsCollection
          .find({
            protocolDriver: AppDefs.NAME,
            protocolDriverInstanceNumber: jsConfig.Instance,
            enabled: true,
          })
          .toArray()

        if (Connections.length === 0) {
          Log.log(
            'MongoDB - No enabled connections found for this driver instance!',
            Log.levelError
          )
          process.exit(3)
        }
        for (const conn of Connections) initConnState(conn)

        // start redundancy control (also exits if instance disabled or node not allowed)
        Redundancy.Start(SUPERVISE_INTERVAL_MS, clientMongo, db, jsConfig, MongoStatus)

        // watch inserts on commandsQueue for PTZ/snapshot commands
        changeStreamCmd = cmdCollection.watch(csCmdPipeline, {
          fullDocument: 'updateLookup',
        })
        changeStreamCmd.on('error', () => {
          Log.log('MongoDB - Error on ChangeStream Cmd!')
          teardownMongo()
        })
        changeStreamCmd.on('change', (change) => {
          processCommand(change.fullDocument, cmdCollection)
        })

        superviseTimer = setInterval(superviseCameras, SUPERVISE_INTERVAL_MS)
        statsTimer = setInterval(
          () => updateStats(connsCollection),
          STATS_INTERVAL_MS
        )
      }

      await sleep(SUPERVISE_INTERVAL_MS)

      if (clientMongo && !(await checkConnectedMongo(clientMongo))) {
        Log.log('MongoDB - Disconnected!')
        teardownMongo()
      }
    } catch (e) {
      Log.log('MongoDB - Connection error: ' + (e.message || e))
      teardownMongo()
      await sleep(RETRY_DELAY_MS)
    }
  }

  // start/stop cameras following the redundancy active state
  function superviseCameras() {
    if (ShuttingDown) return
    if (Redundancy.ProcessStateIsActive() && MongoStatus.HintMongoIsConnected) {
      if (!CamerasStarted) {
        CamerasStarted = true
        for (const conn of Connections) startCamera(conn)
      }
    } else if (CamerasStarted) {
      CamerasStarted = false
      stopAllCameras()
    }
  }

  // process a commandsQueue document (tags like $$CAM001$$relativeMove$$x)
  async function processCommand(cmd, cmdColl) {
    try {
      if (ShuttingDown || !Redundancy.ProcessStateIsActive()) return
      if (typeof cmd?.tag !== 'string' || !cmd.tag.startsWith('$$')) return

      const parts = cmd.tag.split('$$') // ['', connName, command, variable?]
      if (parts.length < 3) return
      const connName = parts[1]
      const command = parts[2]
      const variable = parts.length > 3 ? parts[3] : ''

      const conn = Connections.find((c) => c.name === connName)
      if (!conn || conn.commandsEnabled === false) return

      Log.log(
        `${conn.name} - Command received: ${command}` +
          (variable !== '' ? ` variable: ${variable}` : '') +
          ` value: ${cmd.value}`
      )

      let ok = false
      let detail = ''
      if (
        cmd.timeTag &&
        new Date().getTime() - new Date(cmd.timeTag).getTime() >
          MAX_COMMAND_AGE_MS
      ) {
        detail = 'Command expired'
        Log.log(`${conn.name} - Command discarded (expired)!`)
      } else {
        try {
          ok = await execCameraCommand(conn, command, variable, cmd)
        } catch (e) {
          detail = e.message || String(e)
          Log.log(`${conn.name} - Command error: ${detail}`)
        }
      }

      await cmdColl.updateOne(
        { _id: cmd._id },
        {
          $set: {
            delivered: true,
            deliveredTimeTag: new Date(),
            ack: ok,
            ackTimeTag: new Date(),
            ...(detail !== '' ? { resultDescription: detail } : {}),
          },
        }
      )
    } catch (e) {
      Log.log('Commands - Error processing command: ' + (e.message || e))
    }
  }

  // dispatch a PTZ/snapshot command to the camera
  async function execCameraCommand(conn, command, variable, cmd) {
    if (command === 'snapshot') {
      await saveSnapshot(conn)
      return true
    }

    const ptzCommands = [
      'relativeMove',
      'absoluteMove',
      'continuousMove',
      'stop',
      'gotoHomePosition',
      'setHomePosition',
      'gotoPreset',
      'setPreset',
      'removePreset',
    ]
    if (!ptzCommands.includes(command))
      throw new Error('Unknown command: ' + command)

    if (!conn.cam)
      throw new Error(
        'Camera not connected (ONVIF endpoint required for PTZ commands)'
      )

    const value = typeof cmd.value === 'object' ? parseFloat(cmd.value) : cmd.value
    // when valueString carries a JSON object it is passed to the ONVIF call as-is
    let objArg = null
    try {
      const parsed = JSON.parse(cmd.valueString)
      if (parsed !== null && typeof parsed === 'object') objArg = parsed
    } catch (e) {}

    switch (command) {
      case 'relativeMove':
      case 'absoluteMove':
      case 'continuousMove': {
        let arg = objArg
        if (!arg) {
          if (['x', 'y', 'zoom'].includes(variable)) arg = { [variable]: value }
          else
            throw new Error(
              'Move command requires a x/y/zoom variable or a JSON object value'
            )
        }
        await conn.cam[command](arg)
        return true
      }
      case 'stop':
        await conn.cam.stop(objArg || { panTilt: true, zoom: true })
        return true
      case 'gotoHomePosition':
        await conn.cam.gotoHomePosition(objArg || {})
        return true
      case 'setHomePosition':
        await conn.cam.setHomePosition(objArg || {})
        return true
      case 'gotoPreset':
        await conn.cam.gotoPreset(objArg || { preset: String(value) })
        return true
      case 'setPreset':
        await conn.cam.setPreset(objArg || { presetName: String(value) })
        return true
      case 'removePreset':
        await conn.cam.removePreset(objArg || { presetToken: String(value) })
        return true
      default:
        throw new Error('Unknown command: ' + command)
    }
  }

  // periodic connection stats written to the connection document
  async function updateStats(connsColl) {
    if (
      ShuttingDown ||
      !MongoStatus.HintMongoIsConnected ||
      !Redundancy.ProcessStateIsActive()
    )
      return
    for (const conn of Connections) {
      const stats = {
        nodeName: jsConfig.nodeName,
        streamRunning: conn.stream !== null,
        // the stream is torn down on any camera failure, so a running stream
        // is the link state (communication status tags read isConnected)
        isConnected: conn.stream !== null,
        wsClients: conn.stream?.wsServer?.clients?.size || 0,
        ffmpegRestarts: conn.runtime.ffmpegRestarts,
        lastSnapshotTime: conn.runtime.lastSnapshotTime,
        timeTag: new Date(),
      }
      try {
        await connsColl.updateOne(
          { protocolConnectionNumber: conn.protocolConnectionNumber },
          { $set: { stats: stats } }
        )
      } catch (e) {}
    }
  }
})()

// initialize runtime state and validate configuration of a connection
function initConnState(conn) {
  conn.cam = null
  conn.input = null
  conn.stream = null
  conn.snapshotUri = null
  conn.retryTimer = null
  conn.snapshotTimer = null
  conn.stopRequested = false
  conn.invalidConfig = false
  conn.runtime = { ffmpegRestarts: 0, lastSnapshotTime: null }

  if (!Array.isArray(conn.endpointURLs) || conn.endpointURLs.length === 0) {
    Log.log(`${conn.name} - Connection has no endpoint URLs!`, Log.levelError)
    conn.invalidConfig = true
    return
  }
  // http/https URLs address ONVIF device services, any other URL scheme
  // (rtsp, rtmp, udp, srt, ...) is used as a direct ffmpeg stream input
  const url = conn.endpointURLs[0]
  if (!/^[a-z][a-z0-9+.-]*:\/\//i.test(url)) {
    Log.log(
      `${conn.name} - Invalid endpoint URL (must be an ONVIF http(s):// endpoint or a stream URL like rtsp://)!`,
      Log.levelError
    )
    conn.invalidConfig = true
    return
  }

  conn.wsPort = parseWsPort(conn)
  if (conn.wsPort === null) {
    Log.log(
      `${conn.name} - Invalid local bind address (expected ip:port): ${conn.ipAddressLocalBind}`,
      Log.levelError
    )
    conn.invalidConfig = true
    return
  }

  conn.ffmpegOptions = parseFfmpegOptions(conn)
  Log.log(`${conn.name} - URL: ${maskUrl(url)}`)
  Log.log(
    `${conn.name} - WebSocket port: ${conn.wsPort}, ffmpeg options: ${JSON.stringify(
      conn.ffmpegOptions
    )}`
  )
}

// connect to the camera (ONVIF) or direct RTSP source and start streaming
async function startCamera(conn) {
  if (ShuttingDown || conn.invalidConfig) return
  conn.stopRequested = false

  try {
    const url = new URL(conn.endpointURLs[0])

    if (url.protocol !== 'http:' && url.protocol !== 'https:') {
      // direct stream source (rtsp, rtmp, udp, ...), no ONVIF services available
      conn.input = injectCredentials(url.href, conn)
      startStream(conn)
      return
    }

    // ONVIF device service endpoint
    if (conn.cam === null) {
      let camPath = url.pathname
      if (camPath === '/') camPath = '' // let the onvif lib apply its default path
      let port = url.protocol === 'https:' ? 443 : 80
      port = url.port || port
      conn.cam = new Cam({
        path: camPath,
        username: conn.username,
        password: conn.password,
        hostname: url.hostname,
        port: port,
        autoconnect: false,
        timeout: conn.timeoutMs || DEFAULT_TIMEOUT_MS,
      })
    }

    Log.log(`${conn.name} - Connecting to camera: ${url.hostname}`)
    await conn.cam.connect()
    Log.log(`${conn.name} - Connected to camera: ${url.hostname}`)

    conn.cam.removeAllListeners('event')
    conn.cam.on('event', (camMessage) => {
      Log.log(
        `${conn.name} - Camera event message: ` + JSON.stringify(camMessage),
        Log.levelDetailed
      )
    })

    const streamUri = (await conn.cam.getStreamUri({ protocol: 'RTSP' })).uri
    conn.input = injectCredentials(streamUri, conn)
    Log.log(`${conn.name} - Stream URI: ${maskUrl(conn.input)}`)

    conn.snapshotUri = null
    try {
      conn.snapshotUri = (await conn.cam.getSnapshotUri()).uri
      Log.log(`${conn.name} - Snapshot URI: ${maskUrl(conn.snapshotUri)}`)
    } catch (e) {
      Log.log(`${conn.name} - Snapshot service not available: ${e.message || e}`)
    }

    startStream(conn)
    startSnapshotTimer(conn)
  } catch (e) {
    Log.log(`${conn.name} - Error connecting to camera: ${e.message || e}`)
    scheduleRetry(conn)
  }
}

// spawn ffmpeg and serve the MPEG1 stream over WebSocket
function startStream(conn) {
  if (ShuttingDown || conn.stopRequested) return
  Log.log(
    `${conn.name} - Streaming from ${maskUrl(conn.input)} to WebSocket port ${conn.wsPort}`
  )
  try {
    conn.stream = new Stream({
      name: conn.name,
      streamUrl: conn.input,
      wsPort: conn.wsPort,
      ffmpegOptions: conn.ffmpegOptions,
      ffmpegPath: FfmpegPath,
    })

    // restart on ffmpeg exit or failure to spawn
    conn.stream.mpeg1Muxer.stream.on('exit', () => {
      if (conn.stopRequested || ShuttingDown) return
      Log.log(`${conn.name} - ffmpeg process exited, scheduling restart...`)
      conn.runtime.ffmpegRestarts++
      scheduleRetry(conn)
    })
    conn.stream.mpeg1Muxer.stream.on('error', (e) => {
      Log.log(`${conn.name} - ffmpeg process error: ${e.message || e}`)
      if (conn.stopRequested || ShuttingDown) return
      scheduleRetry(conn)
    })
    conn.stream.wsServer.on('error', (e) => {
      Log.log(`${conn.name} - WebSocket server error: ${e.message || e}`)
      if (conn.stopRequested || ShuttingDown) return
      scheduleRetry(conn)
    })
  } catch (e) {
    Log.log(`${conn.name} - Error starting stream: ${e.message || e}`)
    scheduleRetry(conn)
  }
}

// schedule a full reconnection of the camera (stream URIs may expire)
function scheduleRetry(conn) {
  if (conn.retryTimer || ShuttingDown) return
  conn.retryTimer = setTimeout(() => {
    conn.retryTimer = null
    if (ShuttingDown || !CamerasStarted) return
    stopCamera(conn)
    startCamera(conn)
  }, RETRY_DELAY_MS)
}

function stopCamera(conn) {
  conn.stopRequested = true
  clearTimeout(conn.retryTimer)
  conn.retryTimer = null
  clearInterval(conn.snapshotTimer)
  conn.snapshotTimer = null
  if (conn.stream) {
    try {
      conn.stream.stop() // closes the ws server and kills ffmpeg
    } catch (e) {}
    conn.stream = null
  }
  if (conn.cam) {
    try {
      conn.cam.removeAllListeners('event')
    } catch (e) {}
  }
}

function stopAllCameras() {
  for (const conn of Connections) stopCamera(conn)
}

// periodic JPEG snapshots (giInterval in seconds, 0 = disabled)
function startSnapshotTimer(conn) {
  clearInterval(conn.snapshotTimer)
  conn.snapshotTimer = null
  const giSec = conn.giInterval || 0
  if (giSec <= 0 || !conn.snapshotUri) return
  saveSnapshot(conn).catch(() => {})
  conn.snapshotTimer = setInterval(() => {
    saveSnapshot(conn).catch(() => {})
  }, giSec * 1000)
}

// fetch a snapshot from the camera and save it to snapshots/<connection name>.jpg
async function saveSnapshot(conn) {
  if (!conn.snapshotUri) throw new Error('No snapshot URI available')
  try {
    const response = await fetchWithAuth(
      conn.snapshotUri,
      conn.username,
      conn.password,
      conn.timeoutMs || DEFAULT_TIMEOUT_MS
    )
    if (!response.ok) throw new Error(`HTTP error, status: ${response.status}`)
    const buffer = Buffer.from(await response.arrayBuffer())
    const dir = path.join(__dirname, 'snapshots')
    if (!fs.existsSync(dir)) fs.mkdirSync(dir, { recursive: true })
    const filePath = path.join(
      dir,
      conn.name.replace(/[^a-zA-Z0-9_.-]/g, '_') + '.jpg'
    )
    await fs.promises.writeFile(filePath, buffer)
    conn.runtime.lastSnapshotTime = new Date()
    Log.log(`${conn.name} - Snapshot saved to ${filePath}`, Log.levelDetailed)
  } catch (error) {
    Log.log(`${conn.name} - Error fetching snapshot: ${error.message || error}`)
    throw error
  }
}

// HTTP GET with support for basic and digest authentication (RFC 7616 MD5)
async function fetchWithAuth(url, username, password, timeoutMs) {
  let response = await fetch(url, { signal: AbortSignal.timeout(timeoutMs) })
  if (response.status !== 401 || !username) return response

  const wwwAuth = response.headers.get('www-authenticate') || ''
  let authHeader = null
  if (/^Digest/i.test(wwwAuth)) {
    const params = {}
    for (const m of wwwAuth
      .replace(/^Digest\s+/i, '')
      .matchAll(/(\w+)=(?:"([^"]*)"|([^,\s]*))/g))
      params[m[1].toLowerCase()] = m[2] !== undefined ? m[2] : m[3]
    const u = new URL(url)
    const uri = u.pathname + u.search
    const md5 = (s) => crypto.createHash('md5').update(s).digest('hex')
    const ha1 = md5(`${username}:${params.realm}:${password}`)
    const ha2 = md5(`GET:${uri}`)
    let resp
    let extra = ''
    if (params.qop) {
      const qop = params.qop.split(',')[0].trim()
      const nc = '00000001'
      const cnonce = crypto.randomBytes(8).toString('hex')
      resp = md5(`${ha1}:${params.nonce}:${nc}:${cnonce}:${qop}:${ha2}`)
      extra = `, qop=${qop}, nc=${nc}, cnonce="${cnonce}"`
    } else {
      resp = md5(`${ha1}:${params.nonce}:${ha2}`)
    }
    authHeader =
      `Digest username="${username}", realm="${params.realm}", ` +
      `nonce="${params.nonce}", uri="${uri}", response="${resp}"` +
      extra +
      (params.opaque ? `, opaque="${params.opaque}"` : '')
  } else if (/^Basic/i.test(wwwAuth)) {
    authHeader =
      'Basic ' + Buffer.from(`${username}:${password}`).toString('base64')
  }
  if (authHeader === null) return response

  return await fetch(url, {
    headers: { Authorization: authHeader },
    signal: AbortSignal.timeout(timeoutMs),
  })
}

// add connection credentials to a stream URL when it has none
function injectCredentials(uri, conn) {
  try {
    const u = new URL(uri)
    if (conn.username && u.username === '') {
      u.username = conn.username
      u.password = conn.password || ''
    }
    return u.href
  } catch (e) {
    return uri
  }
}

// hide credentials in URLs written to the log
function maskUrl(uri) {
  try {
    const u = new URL(uri)
    if (u.username !== '') u.username = '****'
    if (u.password !== '') u.password = '****'
    return u.href
  } catch (e) {
    return uri
  }
}

// the WebSocket server port is taken from the port part of ipAddressLocalBind
function parseWsPort(conn) {
  if (typeof conn.ipAddressLocalBind !== 'string') return null
  const parts = conn.ipAddressLocalBind.split(':')
  const port = parseInt(parts.length > 1 ? parts[parts.length - 1] : parts[0])
  if (isNaN(port) || port <= 0 || port > 65535) return null
  return port
}

// ffmpeg options come from the connection's options field as a JSON string
function parseFfmpegOptions(conn) {
  let opts = DEFAULT_FFMPEG_OPTIONS
  if (typeof conn.options === 'string' && conn.options.trim() !== '') {
    const s = conn.options.trim()
    try {
      opts = JSON.parse(s)
    } catch (e1) {
      try {
        // tolerate single-quoted pseudo-JSON from older configs
        opts = JSON.parse(s.replace(/'/g, '"'))
      } catch (e2) {
        Log.log(
          `${conn.name} - Invalid ffmpeg options (must be a valid JSON string), using defaults.`,
          Log.levelError
        )
      }
    }
  }
  return opts
}

// use the bundled ffmpeg executable when present, else rely on the PATH
function resolveFfmpegPath() {
  const envPath = process.env[AppDefs.ENV_PREFIX + 'FFMPEG_PATH']
  if (envPath && envPath.trim() !== '') return envPath.trim()
  const local = path.join(
    __dirname,
    process.platform === 'win32' ? 'ffmpeg.exe' : 'ffmpeg'
  )
  if (fs.existsSync(local)) return local
  return 'ffmpeg'
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

// test mongoDB connectivity
const CheckMongoConnectionTimeout = 10000
async function checkConnectedMongo(client) {
  if (!client) return false
  const tr = setTimeout(() => {
    Log.log('MongoDB - Ping timeout!')
    MongoStatus.HintMongoIsConnected = false
  }, CheckMongoConnectionTimeout)
  let res = null
  try {
    res = await client.db('admin').command({ ping: 1 })
    clearTimeout(tr)
  } catch (e) {
    clearTimeout(tr)
    return false
  }
  if ('ok' in res && res.ok) {
    MongoStatus.HintMongoIsConnected = true
    return true
  }
  MongoStatus.HintMongoIsConnected = false
  return false
}
