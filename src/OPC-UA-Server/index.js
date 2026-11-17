/*
 * OPC-UA Server Driver for JSON-SCADA
 *
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

const {
  OPCUAServer,
  Variant,
  DataType,
  DataValue,
  StatusCodes,
  VariantArrayType,
  AccessLevelFlag,
  NodeId,
  NodeIdType,
  MessageSecurityMode,
  SecurityPolicy,
} = require('node-opcua')
const crypto = require('crypto')
const { MongoClient, Double } = require('mongodb')
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')
const { LoadConfig, getMongoConnectionOptions } = require('./load-config')
const {
  createHistorianBackend,
  installTagHistory,
  isTagEligibleForHistory,
  convertHistValue,
} = require('./historian')
let HintMongoIsConnected = true

process.on('uncaughtException', (err) =>
  Log.log('Uncaught Exception: ' + (err?.stack || err?.message || err))
)
process.on('unhandledRejection', (reason) =>
  Log.log(
    'Unhandled Rejection: ' + (reason?.stack || reason?.message || reason)
  )
)
;(async () => {
  const jsConfig = LoadConfig() // load and parse config file
  Log.levelCurrent = jsConfig.LogLevel

  let clientMongo = null
  let servers = [] // OPC-UA server instances, tracked across (re)connections for shutdown

  // Every 5 s: the instance keep-alive and each connection's
  // stats.isConnected (at least one client session open), read by the data
  // processor's communication status tags. This driver runs no redundancy
  // arbitration (every node serves), so when several nodes run the same
  // instance they all write and activeNodeName shows the last one.
  setInterval(async () => {
    if (!clientMongo) return
    try {
      const db = clientMongo.db(jsConfig.mongoDatabaseName)
      const now = new Date()
      await db
        .collection(jsConfig.ProtocolDriverInstancesCollectionName)
        .updateOne(
          {
            protocolDriver: AppDefs.NAME,
            protocolDriverInstanceNumber: jsConfig.Instance,
          },
          {
            $set: {
              activeNodeName: jsConfig.nodeName,
              activeNodeKeepAliveTimeTag: now,
              softwareVersion: AppDefs.VERSION,
            },
          }
        )
      for (const server of servers) {
        if (server._connNumber === undefined) continue
        await updateConnectionStats(
          db.collection(jsConfig.ProtocolConnectionsCollectionName),
          server._connNumber,
          {
            nodeName: jsConfig.nodeName,
            timeTag: now,
            isConnected: (server.currentSessionCount || 0) > 0,
            sessions: server.currentSessionCount || 0,
          }
        )
      }
    } catch (err) {
      Log.log('MongoDB - Error updating keep-alive/stats: ' + err)
    }
  }, 5000)

  // gracefully shut down and forget all running OPC-UA servers
  const shutdownServers = async () => {
    if (servers.length === 0) return
    Log.log('Shutting down OPC-UA server(s)!')
    for (const srv of servers) {
      try {
        if (srv._histBackend) srv._histBackend.close()
      } catch (e) {
        Log.log('Error closing historian backend: ' + e)
      }
      try {
        await srv.shutdownChannels()
        await srv.shutdown()
      } catch (e) {
        Log.log('Error shutting down OPC-UA server: ' + e)
      }
    }
    servers.length = 0
  }

  // graceful shutdown on termination signals: drop OPC-UA clients and close
  // the Mongo client cleanly before the process exits (e.g. systemctl/nssm
  // stop, docker stop, CTRL+C)
  let isShuttingDown = false
  const gracefulShutdown = async (signal) => {
    if (isShuttingDown) return
    isShuttingDown = true
    Log.log('Received ' + signal + ', shutting down...')
    // force exit if a clean shutdown stalls (e.g. a hung endpoint)
    const forceTimer = setTimeout(() => {
      Log.log('Shutdown timed out, forcing exit.')
      process.exit(1)
    }, 10000)
    if (forceTimer.unref) forceTimer.unref()
    try {
      await shutdownServers()
    } catch (e) {
      Log.log('Error shutting down OPC-UA server(s): ' + e)
    }
    try {
      if (clientMongo) await clientMongo.close()
    } catch (e) {
      Log.log('Error closing MongoDB client: ' + e)
    }
    clearTimeout(forceTimer)
    process.exit(0)
  }
  process.on('SIGINT', () => gracefulShutdown('SIGINT'))
  process.on('SIGTERM', () => gracefulShutdown('SIGTERM'))

  while (true) {
    if (clientMongo === null) {
      // if disconnected, shut down any servers left from a previous
      // connection before trying to (re)connect (avoids leaking
      // instances and port-in-use errors on rebind)
      await shutdownServers()
      await MongoClient.connect(
        // try to (re)connect
        jsConfig.mongoConnectionString,
        getMongoConnectionOptions(jsConfig)
      )
        .then(async (client) => {
          // connected
          HintMongoIsConnected = true
          Log.log(
            'MongoDB - Connected correctly to MongoDB server',
            Log.levelMin
          )
          clientMongo = client
          let allGroups1ToFilterCS = []

          // specify db and collections
          const db = client.db(jsConfig.mongoDatabaseName)
          const rtCollection = db.collection(
            jsConfig.RealtimeDataCollectionName
          )
          const cmdCollection = db.collection(
            jsConfig.CommandsQueueCollectionName
          )
          const connsCollection = db.collection(
            jsConfig.ProtocolConnectionsCollectionName
          )

          // find the connection number, if not found abort (only one connection per instance is allowed for this protocol)
          let connections = await getConnections(connsCollection, jsConfig)

          if (connections.length == 0) {
            Log.log('Fatal error: no connections found for this driver!')
            process.exit(1)
          }

          for (const connection of connections) {
            if (connection.enabled === false) {
              continue
            }

            if (!('_id' in connection)) {
              Log.log('Fatal error: malformed record for connection found!')
              continue
            }

            // accumulate the union of group1 topics across all enabled
            // connections for the shared change-stream filter; null means
            // at least one connection serves all tags (watch everything)
            if (allGroups1ToFilterCS !== null) {
              if (
                'topics' in connection &&
                Array.isArray(connection.topics) &&
                connection.topics.length > 0
              ) {
                allGroups1ToFilterCS = allGroups1ToFilterCS.concat(
                  connection.topics
                )
              } else {
                allGroups1ToFilterCS = null
              }
            }

            // history is opt-in (documented default is false)
            connection.historyEnabled = connection.historyEnabled === true

            // awaited: a failure (e.g. port in use) throws to the outer catch,
            // which drops everything and retries on the next loop
            await startOpcuaServer(connection, rtCollection, cmdCollection)
          }

          // one shared change stream dispatches updates to every server
          // (replaces the previous one-stream-per-connection approach)
          setupChangeStream(rtCollection, allGroups1ToFilterCS)

          async function startOpcuaServer(
            connection,
            rtCollection,
            cmdCollection
          ) {
            async function sendCommand(tag, variant) {
              let cmdRes = null
              try {
                cmdRes = await rtCollection.findOne({ tag: tag })
              } catch (e) {
                Log.log('Error reading command tag! Tag: ' + tag + ' ' + e)
                return StatusCodes.BadInternalError
              }

              if (!cmdRes || !('_id' in cmdRes)) {
                Log.log('Command not found! Tag: ' + tag)
                return StatusCodes.BadNotFound
              }

              if (cmdRes?.commandBlocked !== false) {
                Log.log('Command blocked! Tag: ' + tag)
                return StatusCodes.BadNotWritable
              }

              // check the supervised point for commandBlocked
              if (cmdRes.supervisedOfCommand != 0) {
                let supRes = null
                try {
                  supRes = await rtCollection.findOne({
                    _id: cmdRes.supervisedOfCommand,
                  })
                } catch (e) {
                  Log.log('Error reading supervised tag! Tag: ' + tag + ' ' + e)
                  return StatusCodes.BadInternalError
                }
                if (supRes && '_id' in supRes) {
                  if (supRes?.commandBlocked !== false) {
                    Log.log('Command blocked (sup)! Tag: ' + tag)
                    return StatusCodes.BadNotWritable
                  }
                }
              }

              const cv = variantToCommandValue(variant)
              if (cv === null) {
                Log.log('Command with invalid value type! Tag: ' + tag)
                return StatusCodes.BadTypeMismatch
              }
              let doubleVal = cv.doubleVal
              let strVal = cv.strVal

              if (cmdRes?.type === 'digital') {
                doubleVal = doubleVal ? 1 : 0
                if (cmdRes?.kconv1 === -1) doubleVal = doubleVal ? 0 : 1
                strVal = doubleVal ? 'true' : 'false'
              }
              if (
                cmdRes?.type === 'analog' &&
                'kconv1' in cmdRes &&
                'kconv2' in cmdRes
              ) {
                doubleVal = doubleVal * cmdRes?.kconv1 + cmdRes?.kconv2
                strVal = doubleVal.toString()
              }
              if (!Number.isFinite(doubleVal)) {
                Log.log('Command with non-finite value! Tag: ' + tag)
                return StatusCodes.BadOutOfRange
              }

              // clear to send command
              Log.log(
                'Inserting command: ' +
                  cmdRes.tag +
                  ' ' +
                  doubleVal +
                  ' ' +
                  strVal
              )
              try {
                await cmdCollection.insertOne({
                  protocolSourceConnectionNumber:
                    cmdRes?.protocolSourceConnectionNumber,
                  protocolSourceCommonAddress:
                    cmdRes?.protocolSourceCommonAddress,
                  protocolSourceObjectAddress:
                    cmdRes?.protocolSourceObjectAddress,
                  protocolSourceASDU: cmdRes?.protocolSourceASDU,
                  protocolSourceCommandDuration:
                    cmdRes?.protocolSourceCommandDuration,
                  protocolSourceCommandUseSBO:
                    cmdRes?.protocolSourceCommandUseSBO,
                  pointKey: cmdRes._id,
                  tag: cmdRes.tag,
                  value: new Double(doubleVal),
                  valueString: strVal,
                  originatorUserName:
                    'Protocol connection: ' +
                    connection.protocolConnectionNumber +
                    ' ' +
                    connection.name,
                  originatorIpAddress: '',
                  timeTag: new Date(),
                })
              } catch (e) {
                Log.log('Error inserting command! Tag: ' + tag + ' ' + e)
                return StatusCodes.BadInternalError
              }

              return StatusCodes.Good
            }

            let port = 4840
            if (typeof connection.ipAddressLocalBind === 'string') {
              let ipPort = connection.ipAddressLocalBind.split(':')
              if (ipPort.length > 1 && parseInt(ipPort[1]) > 0)
                port = parseInt(ipPort[1])
            }

            let timeout = 15000
            if (parseInt(connection.timeoutMs) > 0) {
              timeout = parseInt(connection.timeoutMs)
            }

            let certificateProp = {}
            let privateKeyProp = {}
            if (connection?.useSecurity === true) {
              if (
                'localCertFilePath' in connection &&
                typeof connection.localCertFilePath === 'string'
              ) {
                if (connection.localCertFilePath.length > 0)
                  certificateProp.certificateFile = connection.localCertFilePath
              }
              if (
                'privateKeyFilePath' in connection &&
                typeof connection.privateKeyFilePath === 'string'
              ) {
                if (connection.privateKeyFilePath.length > 0)
                  privateKeyProp.privateKeyFile = connection.privateKeyFilePath
              }
            }

            // OPC UA Historical Access capabilities (published only when
            // history is enabled for this connection)
            const historyMaxReturn =
              parseInt(connection.historyMaxReturnDataValues) || 20000
            let historyCaps = {}
            if (connection?.historyEnabled === true) {
              historyCaps = {
                historyServerCapabilities: {
                  accessHistoryDataCapability: true,
                  accessHistoryEventsCapability: false,
                  maxReturnDataValues: historyMaxReturn,
                  maxReturnEventValues: 0,
                  insertDataCapability: false,
                  replaceDataCapability: false,
                  updateDataCapability: false,
                  deleteRawCapability: false,
                  deleteAtTimeCapability: false,
                  insertEventCapability: false,
                  replaceEventCapability: false,
                  updateEventCapability: false,
                  deleteEventCapability: false,
                },
              }
            }

            // Security. useSecurity=true removes the unsecured (None) endpoint:
            // only signed/encrypted channels are offered. Users (optional list
            // of {username, password}) enable username/password login and
            // disable anonymous access unless allowAnonymous is set to true.
            let securityProp = {}
            if (connection?.useSecurity === true) {
              securityProp = {
                securityModes: [
                  MessageSecurityMode.Sign,
                  MessageSecurityMode.SignAndEncrypt,
                ],
                securityPolicies: [
                  SecurityPolicy.Basic256Sha256,
                  SecurityPolicy.Aes128_Sha256_RsaOaep,
                  SecurityPolicy.Aes256_Sha256_RsaPss,
                ],
              }
            }
            const users = Array.isArray(connection.users)
              ? connection.users.filter(
                  (u) =>
                    typeof u?.username === 'string' &&
                    u.username !== '' &&
                    typeof u?.password === 'string'
                )
              : []
            const allowAnonymous =
              typeof connection.allowAnonymous === 'boolean'
                ? connection.allowAnonymous
                : users.length === 0
            const sha = (v) =>
              crypto.createHash('sha256').update(String(v)).digest()
            const userProp =
              users.length > 0
                ? {
                    userManager: {
                      isValidUser: (username, password) => {
                        // hash both sides so the comparison is constant time
                        // and independent of the secret lengths
                        let ok = false
                        for (const u of users) {
                          const a = crypto.timingSafeEqual(
                            sha(u.username),
                            sha(username)
                          )
                          const b = crypto.timingSafeEqual(
                            sha(u.password),
                            sha(password)
                          )
                          if (a && b) ok = true
                        }
                        return ok
                      },
                    },
                  }
                : {}
            if (
              connection.commandsEnabled &&
              allowAnonymous &&
              connection.useSecurity !== true &&
              (!Array.isArray(connection.ipAddresses) ||
                connection.ipAddresses.length === 0)
            ) {
              Log.log(
                'WARNING: connection ' +
                  connection.name +
                  ' accepts commands from anonymous clients over an unsecured channel from any address. Set useSecurity, users or ipAddresses.',
                Log.levelMin
              )
            }

            // Let's create an instance of OPCUAServer
            const server = new OPCUAServer({
              port: port, // the port of the listening socket of the server
              resourcePath: '/' + (connection?.groupId || 'UA/JsonScada'), // this path will be added to the endpoint resource name
              buildInfo: {
                productName: AppDefs.MSG,
                buildNumber: AppDefs.VERSION,
                softwareVersion: AppDefs.VERSION,
                manufacturerName: 'JSON-SCADA Project',
                productUri: 'https://github.com/riclolsen/json-scada/',
                // buildDate: new Date()
              },
              maxConnectionsPerEndpoint: 100,
              serverCapabilities: { maxSessions: 100, ...historyCaps },
              disableDiscovery: false,
              timeout: timeout,
              allowAnonymous: allowAnonymous,
              ...certificateProp,
              ...privateKeyProp,
              ...securityProp,
              ...userProp,
              // defaultSecureTokenLifetime: 10000000,
            })
            server._metrics = {}
            server._name = connection.name
            server._connNumber = connection.protocolConnectionNumber
            servers.push(server)

            await server.initialize()
            Log.log('OPC-UA Server initialized.')

            const addressSpace = server.engine.addressSpace
            //const namespace = addressSpace.getOwnNamespace()
            const ns = addressSpace.getOwnNamespace().namespaceUri
            const namespace = addressSpace.registerNamespace(
              ns + '_' + connection.name
            )

            // Historical Access: create the backend for this connection and
            // enable aggregate (ReadProcessed) support on the address space.
            let histBackend = null
            if (connection?.historyEnabled === true) {
              histBackend = createHistorianBackend(
                connection,
                jsConfig,
                () => clientMongo,
                Log
              )
              server._histBackend = histBackend
              Log.log(
                'History enabled (' +
                  histBackend.backendName +
                  ') for connection ' +
                  connection.name
              )
              try {
                const { addAggregateSupport } = require('node-opcua-aggregates')
                addAggregateSupport(addressSpace)
              } catch (e) {
                Log.log(
                  'Historian - aggregate (ReadProcessed) support unavailable: ' +
                    (e.message || e)
                )
              }
            }
            // we create a new folder under RootFolder
            const folderJsonScada = namespace.addFolder('ObjectsFolder', {
              browseName: 'JsonScadaServer',
            })

            // returns true when the given remote address is allowed to
            // connect. An empty/absent ipAddresses list allows any address.
            // With a list configured, a blank/unknown address is refused
            // (fail closed) unless allowBlank is set (used by the session-level
            // fallback, where the channel gate already checked the address).
            const isRemoteAddressAllowed = (remoteAddress, allowBlank = false) => {
              if (
                !Array.isArray(connection.ipAddresses) ||
                connection.ipAddresses.length === 0
              )
                return true
              const addr = (remoteAddress || '').replace('::ffff:', '')
              if (addr === '') return allowBlank
              return connection.ipAddresses.includes(addr)
            }

            server.on('newChannel', function (channel, endpoint) {
              Log.log(
                'New Channel, remote address: ' +
                  channel.remoteAddress +
                  ', endpoint: ' +
                  endpoint
              )

              // enforce the IP allow-list at the channel level, before any
              // session is created (primary gate)
              if (!isRemoteAddressAllowed(channel?.remoteAddress)) {
                Log.log(
                  'IP not authorized: closing channel! ' + channel?.remoteAddress
                )
                try {
                  channel.close()
                } catch (e) {
                  try {
                    channel.abruptlyInterrupt()
                  } catch (e2) {}
                }
              }
            })

            server.on('create_session', function (session) {
              Log.log('Creating session.')
              Log.log(
                'Client description, application URI: ' +
                  session?.parent?.clientDescription?.applicationUri
              )
              Log.log('Remote Address: ' + session?.channel?.remoteAddress)

              // fallback IP allow-list enforcement at session level (the
              // 'newChannel' handler is the primary gate)
              if (!isRemoteAddressAllowed(session?.channel?.remoteAddress, true)) {
                Log.log('IP not authorized: closing session!')
                try {
                  session.close()
                } catch (e) {}
                try {
                  session.dispose()
                } catch (e) {}
              }
            })

            // console.log(connection)

            let filterGroup1 = {}

            const topics = Array.isArray(connection.topics)
              ? connection.topics
              : []
            if (topics.length > 0) {
              filterGroup1.group1 = { $in: topics }
              Log.log('Filter tags: ' + JSON.stringify(filterGroup1))
            }

            let res = await rtCollection
              .find(
                {
                  protocolSourceConnectionNumber: {
                    $ne: connection.protocolConnectionNumber,
                  }, // exclude data from the same connection
                  ...filterGroup1,
                  ...(connection.commandsEnabled
                    ? {}
                    : { origin: { $ne: 'command' } }),
                  _id: { $gt: 0 },
                },
                {
                  projection: {
                    _id: 1,
                    tag: 1,
                    type: 1,
                    value: 1,
                    valueString: 1,
                    valueJson: 1,
                    timeTag: 1,
                    timeTagAtSource: 1,
                    timeTagAtSourceOk: 1,
                    invalid: 1,
                    isEvent: 1,
                    description: 1,
                    ungroupedDescription: 1,
                    group1: 1,
                    group2: 1,
                    group3: 1,
                    origin: 1,
                    protocolSourceConnectionNumber: 1,
                    protocolSourceBrowsePath: 1,
                    protocolSourceASDU: 1,
                    protocolSourceObjectAddress: 1,
                    protocolSourceAccessLevel: 1,
                    historianPeriod: 1,
                  },
                }
              )
              .sort({ protocolSourceConnectionNumber: 1, origin: -1 })
              .toArray()

            let group1List = {},
              group2List = {},
              group3List = {}

            // folder tree based on group1/group2/group3 properties of tags
            for (let i = 0; i < res.length; i++) {
              if (
                res[i].protocolSourceBrowsePath &&
                typeof res[i].protocolSourceBrowsePath === 'string'
              ) {
                continue
              }
              if (res[i].group1 == '') {
                if (!res[i]._componentOf) res[i]._componentOf = folderJsonScada
                continue
              }
              if (res[i].group1 in group1List) {
                res[i]._componentOf = group1List[res[i].group1]
                continue
              }
              let folder = namespace.addFolder(folderJsonScada, {
                browseName: res[i].group1,
              })
              group1List[res[i].group1] = folder
              res[i]._componentOf = folder
            }

            for (let i = 0; i < res.length; i++) {
              if (
                res[i].protocolSourceBrowsePath &&
                typeof res[i].protocolSourceBrowsePath === 'string'
              ) {
                continue
              }
              if (res[i].group1 == '' || res[i].group2 == '') {
                continue
              }
              // key by full path so equal group2 names under different
              // group1 parents don't collide into a single folder
              const g2Key = JSON.stringify([res[i].group1, res[i].group2])
              if (g2Key in group2List) {
                res[i]._componentOf = group2List[g2Key]
                continue
              }
              let folder = namespace.addFolder(res[i]._componentOf, {
                browseName: res[i].group2,
              })
              group2List[g2Key] = folder
              res[i]._componentOf = folder
            }

            for (let i = 0; i < res.length; i++) {
              if (
                res[i].protocolSourceBrowsePath &&
                typeof res[i].protocolSourceBrowsePath === 'string'
              ) {
                continue
              }
              if (
                res[i].group1 == '' ||
                res[i].group2 == '' ||
                res[i].group3 == ''
              ) {
                continue
              }
              // key by full path so equal group3 names under different
              // group1/group2 parents don't collide into a single folder
              const g3Key = JSON.stringify([
                res[i].group1,
                res[i].group2,
                res[i].group3,
              ])
              if (g3Key in group3List) {
                res[i]._componentOf = group3List[g3Key]
                continue
              }
              let folder = namespace.addFolder(res[i]._componentOf, {
                browseName: res[i].group3,
              })
              group3List[g3Key] = folder
              res[i]._componentOf = folder
            }

            // when protocolSourceBrowsePath is defined the origin of data comes from an OPC server, so we try to recreate the folder structure in this OPC-UA server
            // Avoid creating folders that already exist
            const browsePathFolders = {}
            // index of candidate parent variables (OPC-sourced tags with an
            // 'ns=' object address) by group1 + full browse path of the child,
            // built once to avoid a scan of all tags per tag
            const parentByBrowsePath = new Map()
            for (const t of res) {
              if (
                typeof t.protocolSourceBrowsePath === 'string' &&
                t.protocolSourceBrowsePath &&
                typeof t.protocolSourceObjectAddress === 'string' &&
                t.protocolSourceObjectAddress.startsWith('ns=')
              ) {
                const key = JSON.stringify([
                  t.group1,
                  t.protocolSourceBrowsePath + '/' + t.ungroupedDescription,
                ])
                if (!parentByBrowsePath.has(key)) parentByBrowsePath.set(key, t)
              }
            }
            for (let i = 0; i < res.length; i++) {
              if (
                res[i].protocolSourceBrowsePath &&
                typeof res[i].protocolSourceBrowsePath === 'string'
              ) {
                // try to find a variable with same browse name and path and use its parent as folder
                let found = false
                const parent = parentByBrowsePath.get(
                  JSON.stringify([res[i].group1, res[i].protocolSourceBrowsePath])
                )
                if (parent && parent !== res[i]) {
                  res[i]._componentOf = parent.protocolSourceObjectAddress
                  res[i]._componentOfTag = parent.tag
                  found = true
                }
                if (found) continue

                const browsePath = res[i].protocolSourceBrowsePath.split('/')
                let folder = 'ObjectsFolder'
                let pathKey = ''

                // when there is more than one topic, create a base folder for each topic
                if (topics.length != 1) {
                  // avoid duplicating existing folder for same topic
                  if (!browsePathFolders[res[i].group1]) {
                    browsePathFolders[res[i].group1] = namespace.addFolder(
                      folder,
                      {
                        browseName: res[i].group1,
                      }
                    )
                  }
                  folder = browsePathFolders[res[i].group1]
                  pathKey = res[i].group1
                }

                for (let j = 0; j < browsePath.length; j++) {
                  if (browsePath[j] === '') continue
                  pathKey += '/' + browsePath[j]
                  if (!browsePathFolders[pathKey]) {
                    let folderNew = namespace.addFolder(folder, {
                      browseName: browsePath[j],
                    })
                    browsePathFolders[pathKey] = folderNew
                  }
                  folder = browsePathFolders[pathKey]
                }
                res[i]._componentOf = folder
              }
            }

            Log.log(`Creating ${res.length} OPC UA Variables...`)
            for (let i = 0; i < res.length; i++) {
              try {
                const element = res[i]
                if (element._id <= 0) {
                  // exclude internal system data
                  continue
                }

                let cmdWriteProp = {}
                if (element.origin === 'command') {
                  let variant = {
                    dataType: DataType.Double,
                    value: element?.value,
                  }
                  if (element.type === 'string')
                    variant = {
                      dataType: DataType.String,
                      value: element?.valueString,
                    }
                  if (element.type === 'digital')
                    variant = {
                      dataType: DataType.Boolean,
                      value: element?.value == 0 ? false : true,
                    }

                  cmdWriteProp = {
                    value: {
                      get: () => new Variant(variant),
                      // async setter: the client gets the real outcome (blocked,
                      // unknown tag, bad type, insert failure) instead of Good
                      set: (variant, callback) => {
                        sendCommand(element.tag, variant)
                          .then((sc) => callback(null, sc))
                          .catch((e) => {
                            Log.log(
                              'Command error! Tag: ' + element.tag + ' ' + e
                            )
                            callback(null, StatusCodes.BadInternalError)
                          })
                      },
                    },
                  }
                }

                const v = convertValueVariant(element)
                if (v.dataType) {
                  let nodeId = null
                  // tries to keep original id
                  if (
                    element.protocolSourceObjectAddress &&
                    typeof element.protocolSourceObjectAddress === 'string' &&
                    element.protocolSourceObjectAddress.startsWith('ns=')
                  ) {
                    nodeId = element.protocolSourceObjectAddress
                    // remove namespace from nodeId
                    nodeId = nodeId.substring(nodeId.indexOf(';') + 1)
                  } else {
                    // numeric nodeId can't exceed 4294967295
                    if (element._id <= 4294967295) {
                      nodeId = 'i=' + element._id
                    }
                  }

                  // let browseName = element.ungroupedDescription || element.tag
                  // do not create new objects for commands
                  if (
                    element.origin === 'command' &&
                    nodeId &&
                    element.protocolSourceObjectAddress &&
                    typeof element.protocolSourceObjectAddress === 'string' &&
                    element.protocolSourceObjectAddress.startsWith('ns=')
                  ) {
                    const v = namespace.findNode(nodeId)
                    if (v) {
                      continue
                    }
                  }

                  // check if namespace already have this nodeId
                  if (nodeId && namespace.findNode(nodeId)) {
                    // if already exists then let a new one be auto created by NodeOPCUA
                    nodeId = null
                  }

                  Log.log(
                    server._name + ' - ' +
                    'Creating node: ' +
                      element.tag +
                      ' ' +
                      element.ungroupedDescription +
                      ' ' +
                      (nodeId || ''),
                    2
                  )

                  if (typeof element._componentOf === 'string') {
                    let pnId = element._componentOf.substring(
                      element._componentOf.indexOf(';') + 1
                    )
                    let el = namespace.findNode(pnId)
                    if (el) {
                      element._componentOf = el
                    }
                  } else if (element._componentOf === null) {
                    element._componentOf = folderJsonScada
                  }

                  // Log.log(JSON.stringify(v))

                  if (
                    element._componentOfTag &&
                    server._metrics[element._componentOfTag]?.nodeId
                  )
                    element._componentOf =
                      server._metrics[element._componentOfTag].nodeId

                  server._metrics[element.tag] = namespace.addVariable({
                    componentOf: element._componentOf,
                    ...(nodeId === null ? {} : { nodeId: nodeId }),
                    browseName: element.ungroupedDescription || element.tag,
                    displayName: element.description,
                    dataType: v.dataType,
                    description: element?.description,
                    minimumSamplingInterval: -1,
                    accessLevel:
                      AccessLevelFlag.CurrentRead |
                      // only command variables are writable: a write to any
                      // other variable would go nowhere (data comes from the
                      // database), so it is not offered
                      (element.origin === 'command'
                        ? AccessLevelFlag.CurrentWrite
                        : 0),
                    ...cmdWriteProp,
                    // timestamped_get will not be used, values will be updated by change stream
                  })
                  if (element.origin !== 'command') {
                    server._metrics[element.tag].setValueFromSource(
                      {
                        dataType: v.dataType,
                        ...(v.arrayType ? { arrayType: v.arrayType } : {}),
                        value: v.value,
                      },
                      element.invalid ? StatusCodes.Bad : StatusCodes.Good,
                      sourceTimestampOf(element)
                    )
                  }

                  // install OPC UA Historical Access on eligible tags
                  if (
                    histBackend &&
                    isTagEligibleForHistory(connection, element)
                  ) {
                    try {
                      installTagHistory(
                        namespace,
                        addressSpace,
                        server._metrics[element.tag],
                        element,
                        histBackend,
                        connection,
                        Log
                      )
                    } catch (e) {
                      Log.log(
                        'Historian - install error for tag ' +
                          element.tag +
                          ': ' +
                          (e.message || e),
                        Log.levelMin
                      )
                    }
                  }
                }
              } catch (e) {
                Log.log(
                  `Error creating OPC UA Variable for tag ${res[i].tag}`,
                  Log.levelMin
                )
                Log.log(e, Log.levelMin)
                // Log.log(JSON.stringify(res[i]))
              }
            }

            Log.log(`Finished creating OPC UA Variables.`)

            // start listening only now that the address space is complete
            await server.start()
            Log.log('Server is now listening ... (press CTRL+C to stop)')
            server.endpoints.forEach(function (endpoint) {
              endpoint.endpointDescriptions().forEach(function (desc) {
                Log.log(
                  'Server EndpointUrl: ' +
                    desc.endpointUrl +
                    ' SecurityMode: ' +
                    desc.securityMode.toString() +
                    ' SecurityPolicy: ' +
                    desc.securityPolicyUri
                )
              })
            })
          }

          // Single shared change stream feeding all servers. Using one
          // stream (instead of one per connection) avoids O(N^2) redundant
          // updates and prevents a single stream's error/close/end from
          // tearing down the Mongo client shared by every connection.
          function setupChangeStream(rtColl, groups1ToFilter) {
            let csFilterGroup1 = {
              'fullDocument.group1': {
                $exists: true,
              },
            }

            if (groups1ToFilter !== null && groups1ToFilter?.length > 0) {
              csFilterGroup1 = {
                'fullDocument.group1': { $in: groups1ToFilter },
              }
            }

            // Two $match stages with the same result as one $or: the first
            // does not reference fullDocument, so the server applies it
            // before the updateLookup and skips the document read for raw
            // driver writes (sourceDataUpdate) and the special points.
            const csPipeline = [
              {
                $match: {
                  $or: [
                    {
                      $and: [
                        {
                          'updateDescription.updatedFields.sourceDataUpdate': {
                            $exists: false,
                          },
                        },
                        { 'documentKey._id': { $nin: [-2, -1] } },
                        { operationType: 'update' },
                      ],
                    },
                    { operationType: 'replace' },
                  ],
                },
              },
              {
                $match: {
                  $or: [csFilterGroup1, { operationType: 'replace' }],
                },
              },
              {
                $project: { documentKey: false },
              },
            ]

            const changeStream = rtColl.watch(csPipeline, {
              fullDocument: 'updateLookup',
            })
            const csClient = clientMongo

            // A failed/closed stream drops the Mongo client so the main loop
            // reconnects. Events from a stream that is no longer current (a
            // late close after reconnection) must not touch the new client.
            const onStreamDown = (what) => {
              changeStream.removeAllListeners('change')
              if (clientMongo !== csClient) return
              Log.log('MongoDB - ' + what + ' ChangeStream!')
              try {
                csClient.close()
              } catch (e) {}
              clientMongo = null
            }

            try {
              changeStream.on('error', () => onStreamDown('Error on'))
              changeStream.on('close', () => onStreamDown('Closed'))
              changeStream.on('end', () => onStreamDown('Ended'))

              // start to listen for changes
              changeStream.on('change', (change) => {
                try {
                  if (!change.fullDocument) return // deleted before the lookup
                  const v = convertValueVariant(change.fullDocument)
                  if (!v.dataType) return
                  for (let i = 0; i < servers.length; i++) {
                    try {
                      const srv = servers[i]

                      const m = srv._metrics[change.fullDocument?.tag]
                      if (m !== undefined) {
                        m.setValueFromSource(
                          {
                            dataType: v.dataType,
                            ...(v.arrayType ? { arrayType: v.arrayType } : {}),
                            value: v.value,
                          },
                          change.fullDocument.invalid
                            ? StatusCodes.Bad
                            : StatusCodes.Good,
                          sourceTimestampOf(change.fullDocument)
                        )

                        if (Log.levelCurrent >= Log.levelDebug) {
                          Log.log(
                            srv._name +
                              ' - Update: ' +
                              change.fullDocument?.tag +
                              ' ' +
                              change.fullDocument?.value +
                              (change.fullDocument?.invalid ? ' bad' : ' good'),
                            Log.levelDebug
                          )
                        }
                      }
                    } catch (e) {
                      Log.log(
                        'MongoDB - CS Variable Convert Error: ' + e,
                        Log.levelMin
                      )
                    }
                  }
                } catch (e) {
                  Log.log('MongoDB - CS Change Error: ' + e, Log.levelMin)
                }
              })
            } catch (e) {
              Log.log('MongoDB - CS Error: ' + e, Log.levelMin)
            }
          }
        })
        .catch(function (err) {
          if (clientMongo) clientMongo.close()
          clientMongo = null
          Log.log(err)
        })
    }

    // wait 5 seconds
    await new Promise((resolve) => setTimeout(resolve, 5000))

    // detect connection problems, if error will null the client to later reconnect
    if (clientMongo === undefined) {
      Log.log('Disconnected Mongodb!')
      clientMongo = null
    }
    if (clientMongo)
      if (!(await checkConnectedMongo(clientMongo))) {
        // not anymore connected, will retry
        Log.log('Disconnected Mongodb!')
        if (clientMongo) clientMongo.close()
        clientMongo = null
      }
    // if disconnected from mongo, shut down the OPC-UA server(s) so clients
    // are dropped and can reconnect once the database is available again
    if (!clientMongo) await shutdownServers()
  }
})()

// $set stats.<field> on a connection document. Connection documents are
// created with stats: null, where a dotted $set fails with error 28
// (PathNotViable): replace the null with an empty document and retry once.
async function updateConnectionStats(connsCollection, connNumber, fields) {
  const set = {}
  for (const [k, v] of Object.entries(fields)) set['stats.' + k] = v
  const filter = { protocolConnectionNumber: connNumber }
  try {
    return await connsCollection.updateOne(filter, { $set: set })
  } catch (err) {
    if (err?.code !== 28) throw err
    await connsCollection.updateOne(
      { ...filter, stats: null },
      { $set: { stats: {} } }
    )
    return await connsCollection.updateOne(filter, { $set: set })
  }
}

async function getConnections(connsCollection, configObj) {
  let results = await connsCollection
    .find({
      protocolDriver: AppDefs.NAME,
      protocolDriverInstanceNumber: configObj.Instance,
      enabled: true,
    })
    .sort({ protocolConnectionNumber: 1 })
    .toArray()

  if (!results || !('length' in results) || results.length == 0) {
    Log.log('Connection - No protocol connection found!')
    process.exit(1)
  }
  return results
}

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
    clearTimeout(tr)
    Log.log('Error on mongodb connection!')
    return false
  }
  if (res && 'ok' in res && res.ok) {
    HintMongoIsConnected = true
    return true
  } else {
    HintMongoIsConnected = false
    return false
  }
}

function convertValueVariant(rtData) {
  let value = null,
    dataType = '',
    arrayType = null
  switch (rtData?.type) {
    case 'digital': {
      // delegate to the shared scalar converter (see historian.js) so live
      // and historical conversion cannot drift
      const r = convertHistValue('digital', rtData.protocolSourceASDU, rtData.value)
      dataType = r.dataType
      value = r.value
      break
    }
    case 'json':
      let obj = null
      try {
        if (rtData?.valueJson) obj = JSON.parse(rtData?.valueJson)
      } catch (e) {
        Log.log(e)
        break
      }
      switch (rtData.protocolSourceASDU) {
        case 'boolean[]':
          dataType = DataType.Boolean
          break
        case 'double[]':
          dataType = DataType.Double
          break
        case 'float[]':
          dataType = DataType.Float
          break
        case 'int16[]':
          dataType = DataType.Int16
          break
        case 'uint16[]':
          dataType = DataType.UInt16
          break
        case 'int32[]':
          dataType = DataType.Int32
          break
        case 'uint32[]':
          dataType = DataType.UInt32
          break
        case 'int64[]':
          dataType = DataType.Int64
          break
        case 'uint64[]':
          dataType = DataType.UInt64
          break
        case 'byte[]':
          dataType = DataType.Byte
          break
        case 'sbyte[]':
          dataType = DataType.SByte
          break
        case 'string[]':
          dataType = DataType.String
          break
        case 'guid[]':
          dataType = DataType.Guid
          let gArr = []
          if (obj?.length)
            for (let i = 0; i < obj.length; i++) {
              if (typeof obj[i] === 'string') gArr.push(obj[i])
              else {
                if (obj[i]?.GuidString) gArr.push(obj[i].GuidString)
              }
            }
          obj = gArr
          break
        case 'datetime[]':
          dataType = DataType.DateTime
          let dtArr = []
          if (obj?.length)
            for (let i = 0; i < obj.length; i++) {
              dtArr.push(new Date(obj[i]))
            }
          obj = dtArr
          break
        case 'bytestring[]':
          dataType = DataType.ByteString
          break
        case 'xmlelement[]':
          dataType = DataType.XmlElement
          break
        case 'nodeid':
          dataType = DataType.NodeId
          if (
            obj !== null &&
            (!('Identifier' in obj) || !('NamespaceIndex' in obj))
          ) {
            obj = new NodeId(NodeIdType.NUMERIC, 0, 0)
            break
          }
          try {
            if (!obj.IdType) obj.IdType = NodeIdType.NUMERIC
            if (typeof obj.Identifier === 'number')
              obj.IdType = NodeIdType.NUMERIC
            if (typeof obj.Identifier === 'string')
              obj.IdType = NodeIdType.STRING
            obj = new NodeId(obj.IdType, obj.Identifier, obj.NamespaceIndex)
          } catch (e) {
            obj = new NodeId(NodeIdType.NUMERIC, 0, 0)
          }
          break
        case 'nodeid[]':
          dataType = DataType.NodeId
          break
        case 'expandednodeid[]':
          dataType = DataType.ExpandedNodeId
          break
        case 'qualifiedname[]':
          dataType = DataType.QualifiedName
          break
        case 'localizedtext[]':
          dataType = DataType.LocalizedText
          break
        default:
          dataType = DataType.Double
      }

      if (Array.isArray(obj)) {
        if (rtData.protocolSourceASDU?.endsWith('[][]')) {
          arrayType = VariantArrayType.Matrix
          value = obj
        } else {
          arrayType = VariantArrayType.Array
          value = obj
          if (obj.length > 0)
            switch (typeof obj[0]) {
              case 'boolean':
                dataType = DataType.Boolean
                break
              case 'number':
              case 'bigint':
                if (!dataType) dataType = DataType.Double
                break
              case 'string':
                if (!dataType) dataType = DataType.String
                break
              case 'object':
                if (!dataType) dataType = DataType.Variant
                break
              default:
                if (!dataType) dataType = DataType.String
                break
            }
        }
      } else {
        value = obj
        if (!dataType) {
          dataType = DataType.String
          value = rtData?.valueJson
        }
      }
      break
    case 'string': {
      // delegate to the shared scalar converter (see historian.js)
      const r = convertHistValue(
        'string',
        rtData.protocolSourceASDU,
        rtData.value,
        'valueString' in rtData ? rtData.valueString : undefined
      )
      dataType = r.dataType
      value = r.value
      break
    }
    case 'analog': {
      // delegate to the shared scalar converter (see historian.js)
      const r = convertHistValue('analog', rtData.protocolSourceASDU, rtData.value)
      dataType = r.dataType
      value = r.value
      break
    }
    default:
  }

  return {
    value: value,
    dataType: dataType,
    arrayType: arrayType,
  }
}

// Source timestamp for a tag: the field time when valid, else the time the
// value was received (never the 1970 epoch, which clients show as an error).
function sourceTimestampOf(el) {
  if (el?.timeTagAtSource && el.timeTagAtSourceOk !== false)
    return el.timeTagAtSource
  return el?.timeTag || new Date()
}

// Convert the Variant of a client write into the command value. Returns
// { doubleVal, strVal } or null when the value cannot be used (null value,
// array, or a non-numeric string).
function variantToCommandValue(variant) {
  if (!variant || variant.value === null || variant.value === undefined)
    return null
  if (variant.arrayType !== undefined && variant.arrayType !== VariantArrayType.Scalar)
    return null
  let doubleVal
  let strVal
  switch (variant.dataType) {
    case DataType.Boolean:
      doubleVal = variant.value ? 1 : 0
      strVal = variant.value ? 'true' : 'false'
      break
    case DataType.Int64:
    case DataType.UInt64:
      // node-opcua represents 64-bit integers as [high, low] 32-bit words
      if (Array.isArray(variant.value) && variant.value.length === 2) {
        let hi = variant.value[0]
        if (variant.dataType === DataType.Int64 && hi >= 0x80000000)
          hi -= 0x100000000
        doubleVal = hi * 4294967296 + variant.value[1]
      } else doubleVal = Number(variant.value)
      strVal = String(doubleVal)
      break
    case DataType.SByte:
    case DataType.Byte:
    case DataType.Int16:
    case DataType.UInt16:
    case DataType.Int32:
    case DataType.UInt32:
    case DataType.Float:
    case DataType.Double:
      doubleVal = Number(variant.value)
      strVal = String(variant.value)
      break
    case DataType.String: {
      const sv = String(variant.value).trim()
      if (/^(true|false)$/i.test(sv)) doubleVal = /^true$/i.test(sv) ? 1 : 0
      else doubleVal = parseFloat(sv)
      strVal = String(variant.value)
      break
    }
    default:
      return null
  }
  if (!Number.isFinite(doubleVal)) return null
  return { doubleVal, strVal }
}
