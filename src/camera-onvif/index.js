'use strict'

const AppDefs = require('./app-defs')
const Log = require('./simple-logger')
const { Cam } = require('onvif/promises')
const Stream = require('node-rtsp-stream')
const LoadConfig = require('./load-config')
const { MongoClient, GridFSBucket } = require('mongodb')
const MongoStatus = { HintMongoIsConnected: false }

process.on('uncaughtException', (err) =>
  console.log('Uncaught Exception:' + JSON.stringify(err))
)

;(async () => {
  const jsConfig = LoadConfig() // load and parse config file
  Log.levelCurrent = jsConfig.LogLevel

  let cmdCollection = null
  let clientMongo = null

  Log.log('MongoDB - Connecting to MongoDB server...', Log.levelMin)

  if (clientMongo === null)
    // if disconnected
    await MongoClient.connect(
      // try to (re)connect
      jsConfig.mongoConnectionString,
      jsConfig.MongoConnectionOptions
    ).then(async (client) => {
      // connected
      clientMongo = client
      MongoStatus.HintMongoIsConnected = true
      Log.log('MongoDB - Connected correctly to MongoDB server', Log.levelMin)

      // specify db and collections
      const db = client.db(jsConfig.mongoDatabaseName)
      cmdCollection = db.collection(jsConfig.CommandsQueueCollectionName)

      // find protocol driver instance in db
      const instanceDoc = await db
        .collection(jsConfig.ProtocolDriverInstancesCollectionName)
        .findOne({
          protocolDriver: AppDefs.NAME,
          protocolDriverInstanceNumber: jsConfig.Instance,
        })

      if (instanceDoc === null) {
        Log.log(
          'MongoDB - Protocol driver instance not found in database',
          Log.levelError
        )
        process.exit(1)
      }

      if (instanceDoc.enabled === false) {
        Log.log(
          'MongoDB - Protocol driver instance is disabled in database',
          Log.levelError
        )
        process.exit(2)
      }

      Log.log(
        `MongoDB - Protocol driver instance found in database: instance number ${instanceDoc.protocolDriverInstanceNumber}`
      )

      // find connections in db
      const connectionsDocs = await db
        .collection(jsConfig.ProtocolConnectionsCollectionName)
        .find({
          protocolDriver: AppDefs.NAME,
          protocolDriverInstanceNumber: jsConfig.Instance,
        })
        .toArray()

      if (connectionsDocs.length === 0) {
        Log.log('MongoDB - No connections found in database')
        process.exit(3)
      }

      for (const conn of connectionsDocs) {
        conn.cam = null
        conn.input = null
        conn.stream = null

        if (conn.endpointURLs.length === 0) {
          Log.log(`MongoDB - Connection ${conn.name} has no endpoint URLs!`)
          continue
        }
        if (
          !conn.endpointURLs[0].startsWith('rtsp://') &&
          !conn.endpointURLs[0].startsWith('http://') &&
          !conn.endpointURLs[0].startsWith('https://')
        ) {
          Log.log(`MongoDB - Connection ${conn.name} has invalid endpoint URL!`)
          continue
        }

        Log.log(`${conn.name} - URL: ${conn.endpointURLs[0]}`)

        // extract ip, port and path from url
        const url = new URL(conn.endpointURLs[0])
        let path = url.pathname
        if (path === '/') path = ''
        const ip = url.hostname

        if (url.protocol == 'rtsp:') {
          conn.input = url.href
          continue
        }

        let port = 80
        if (url.protocol == 'https:') port = 443
        port = url.port || port

        const cam = new Cam({
          path: path,
          username: conn.username,
          password: conn.password,
          hostname: ip,
          port: port,
          autoconnect: false,
          timeout: conn.timeoutMs,
          // useSecure: false,
          // useWSSecurity: false,
          // secureOpts: {},
          // preserveAddress: false,
        })
        conn.cam = cam
      }

      for (const conn of connectionsDocs) {
        if (conn.cam === null && conn.input === null) continue

        if (conn.cam === null) {
          Log.log(`${conn.name} - Connecting to stream: ${conn.input}`)
          const stream = new Stream({
            name: conn.name,
            streamUrl: conn.input,
            wsPort: url.port,
            ffmpegOptions: ffmpegOptions,
          })
          conn.stream = stream
          continue
        }

        conn.cam.on('event', function (camMessage) {
          Log.log(
            `${conn.name} - Camera event message: ` +
              JSON.stringify(camMessage),
            Log.levelDetailed
          )
        })

        try {
          Log.log(`${conn.name} - Connecting to camera: ${conn.cam.hostname}`)
          await conn.cam.connect()
          Log.log(`${conn.name} - Connected to camera: ${conn.cam.hostname}`)
          const input = (
            await conn.cam.getStreamUri({ protocol: 'RTSP' })
          ).uri.replace('://', `://${conn.cam.username}:${conn.cam.password}@`)
          conn.input = input
          Log.log(`${conn.name} - Connected to stream: ${input}`)

          // split the url into parts to get the port number
          if (conn.ipAddressLocalBind.indexOf(':') === -1) {
            Log.log(
              `${conn.name} - Invalid local bind address (does not contain port number): ${conn.ipAddressLocalBind}`
            )
            continue
          }
          const port = conn.ipAddressLocalBind.split(':')[1]

          let ffmpegOptions = { '-r': 30, '-s': '320x240' }
          if (conn.options != '') {
            try {
              ffmpegOptions = JSON.parse(conn.options)
            } catch (e) {
              Log.log(`${conn.name} - Invalid ffmpeg options! Must be valid JSON string.`)
            }
          }
          Log.log(`${conn.name} - Streaming to: ${input} on port ${port} with options: ${JSON.stringify(ffmpegOptions)}`)
          const stream = new Stream({
            name: conn.name,
            streamUrl: input,
            wsPort: port,
            ffmpegOptions: ffmpegOptions,
          })
          conn.stream = stream  
        } catch (e) {
          Log.log(`${conn.name} - ${e}`)
        }
      }
    })
})().catch(console.error)
