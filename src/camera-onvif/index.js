/*
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
const { Cam } = require('onvif/promises')
const Stream = require('node-rtsp-stream')
const LoadConfig = require('./load-config')
const { MongoClient, GridFSBucket } = require('mongodb')
const fs = require('fs')
const path = require('path')
const fetch = require('node-fetch')

const MongoStatus = { HintMongoIsConnected: false }

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

      const changeStreamCmd = cmdCollection.watch(csCmdPipeline, {
        fullDocument: 'updateLookup',
      })
      try {
        changeStreamCmd.on('error', (change) => {
          if (clientMongo) clientMongo.close()
          clientMongo = null
          Log.log('MongoDB - Error on ChangeStream Cmd!')
        })
        changeStreamCmd.on('close', (change) => {
          Log.log('MongoDB - Closed ChangeStream Cmd!')
        })
        changeStreamCmd.on('end', (change) => {
          clientMongo = null
          Log.log('MongoDB - Ended ChangeStream Cmd!')
        })

        // start listen to changes
        changeStreamCmd.on('change', (change) => {
          if (!change.fullDocument.tag.startsWith('$$')) return // not onvif command

          // extracts connection name between $$ and $$ from tag
          const connName = change.fullDocument.tag.match(/^\$\$(.*?)\$\$/)[1]
          const conn = connectionsDocs.find((c) => c.name === connName)
          if (!conn || !conn.commandsEnabled) return

          Log.log(`${conn.name} - ChangeStream Cmd: ${change.fullDocument.tag}`)

          // count how many $$ are in the tag
          const count = (change.fullDocument.tag.match(/\$\$/g) || []).length
          if (count < 2) return // not onvif command

          // extract command from tag after $$connName$$ and before $$
          const command =
            change.fullDocument.tag.match(/^\$\$.*?\$\$(.*)\$\$/)[1]
          Log.log(`${conn.name} - Command: ${command}`)

          // extracts variable after the third $$ from tag
          let variable = ''
          if (count > 2)
            variable = change.fullDocument.tag.match(
              /^\$\$.*?\$\$.*?\$\$(.*)/
            )[1]

          Log.log(`${conn.name} - Variable: ${variable}`)

          let obj = null
          try {
            obj = JSON.parse(change.fullDocument.valueString)
          } catch {}
          switch (command) {
            case 'relativeMove':
              switch (variable) {
                case 'x':
                  conn.cam.relativeMove({
                    x: change.fullDocument.value,
                  })
                  break
                case 'y':
                  conn.cam.relativeMove({
                    y: change.fullDocument.value,
                  })
                  break
                case 'zoom':
                  conn.cam.relativeMove({
                    zoom: change.fullDocument.value,
                  })
                  break
                default:
                  conn.cam.relativeMove(obj)
              }
              break
            case 'absoluteMove':
              conn.cam.absoluteMove(obj)
              break
            case 'continuousMove':
              conn.cam.continuousMove(obj)
              break
            case 'setHomePosition':
              conn.cam.setHomePosition(obj)
              break
            case 'gotoHomePosition':
              conn.cam.gotoHomePosition(obj)
              break
            case 'removePreset':
              conn.cam.removePreset(obj)
              break
            case 'gotoPreset':
              conn.cam.gotoPreset(obj)
              break
            case 'setPreset':
              conn.cam.setPreset(obj)
              break
            case 'stop':
              conn.cam.stop(obj)
              break
            default:
              Log.log(`${conn.name} - Unknown command: ${command}`)
              break
          }
        })
      } catch (err) {
        Log.log('MongoDB - Error on ChangeStream Cmd!')
        Log.log(JSON.stringify(err, null, 2))
        process.exit(4)
      }

      for (const conn of connectionsDocs) {
        conn.cam = null
        conn.input = null
        conn.stream = null
        conn.snapshots = null

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
        // conn.cam.on('eventsError', console.error);

        try {
          Log.log(`${conn.name} - Connecting to camera: ${conn.cam.hostname}`)
          await conn.cam.connect()
          Log.log(`${conn.name} - Connected to camera: ${conn.cam.hostname}`)
          conn.input = (
            await conn.cam.getStreamUri({ protocol: 'RTSP' })
          ).uri.replace('://', `://${conn.cam.username}:${conn.cam.password}@`)
          Log.log(`${conn.name} - Connected to stream: ${conn.input}`)

          conn.snapshots = await conn.cam.getSnapshotUri()
          Log.log(`${conn.name} - Snapshots: ${conn.snapshots.uri}`)

          await saveSnapshot(conn)

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
              Log.log(
                `${conn.name} - Invalid ffmpeg options! Must be valid JSON string.`
              )
            }
          }
          Log.log(
            `${conn.name} - Streaming to: ${
              conn.input
            } on port ${port} with options: ${JSON.stringify(ffmpegOptions)}`
          )
          const stream = new Stream({
            name: conn.name,
            streamUrl: conn.input,
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

async function saveSnapshot(conn) {
  Log.log(`${conn.name} - Snapshot stream: ${conn.snapshots.uri}`)
  try {
    const response = await fetch(conn.snapshots.uri)
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`)
    }
    const buffer = await response.buffer()
    const filePath = path.join(__dirname, 'snapshot.jpg')
    fs.writeFile(filePath, buffer, (err) => {
      if (err) {
        Log.log(`${conn.name} - Error saving snapshot: ${err}`)
      } else {
        Log.log(`${conn.name} - Snapshot saved to ${filePath}`)
      }
    })
  } catch (error) {
    Log.log(`${conn.name} - Error fetching snapshot: ${error}`)
  }
}
