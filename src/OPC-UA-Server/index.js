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
} = require('node-opcua')
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
        await db
          .collection(jsConfig.ProtocolConnectionsCollectionName)
          .updateOne(
            { protocolConnectionNumber: server._connNumber },
            {
              $set: {
                'stats.nodeName': jsConfig.nodeName,
                'stats.timeTag': now,
                'stats.isConnected': (server.currentSessionCount || 0) > 0,
                'stats.sessions': server.currentSessionCount || 0,
              },
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

          connections.forEach((connection) => {
            if (connection.enabled === false) {
              return
            }

            if (!('_id' in connection)) {
              Log.log('Fatal error: malformed record for connection found!')
              return
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

            connection.historyEnabled = connection.historyEnabled ?? true

            startOpcuaServer(connection, rtCollection, cmdCollection)
          })

          // one shared change stream dispatches updates to every server
          // (replaces the previous one-stream-per-connection approach)
          setupChangeStream(rtCollection, allGroups1ToFilterCS)

          async function startOpcuaServer(
            connection,
            rtCollection,
            cmdCollection
          ) {
            async function sendCommand(tag, variant) {
              let cmdRes = await rtCollection.findOne({ tag: tag })

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
                let supRes = await rtCollection.findOne({
                  _id: cmdRes.supervisedOfCommand,
                })
                if (supRes && '_id' in supRes) {
                  if (supRes?.commandBlocked !== false) {
                    Log.log('Command blocked (sup)! Tag: ' + tag)
                    return StatusCodes.BadNotWritable
                  }
                }
              }

              let doubleVal = variant.value
              let strVal = variant.value.toString()

              switch (variant.dataType) {
                case DataType.Boolean:
                  doubleVal = variant.value ? 1 : 0
                  strVal = variant.value.toString()
                  break
                case DataType.SByte:
                case DataType.Byte:
                case DataType.Int16:
                case DataType.UInt16:
                case DataType.Int32:
                case DataType.UInt32:
                case DataType.Int64:
                case DataType.UInt64:
                case DataType.Float:
                case DataType.Double:
                  doubleVal = variant.value
                  strVal = variant.value.toString()
                  break
                case DataType.Variant:
                case DataType.StatusCode:
                case DataType.Guid:
                case DataType.QualifiedName:
                case DataType.LocalizedText:
                case DataType.DiagnosticInfo:
                case DataType.ByteString:
                case DataType.ExpandedNodeId:
                case DataType.NodeId:
                case DataType.XmlElement:
                case DataType.String:
                  doubleVal = parseFloat(variant.value)
                  strVal = variant.value.toString()
                  break
              }

              if (cmdRes?.type === 'digital') {
                doubleVal = doubleVal ? 1 : 0;
                if (cmdRes?.kconv1 === -1) 
                  doubleVal = doubleVal ? 0 : 1;
                strVal = doubleVal ? 'true' : 'false'
              }
              if (cmdRes?.type === 'analog' && 'kconv1' in cmdRes && 'kconv2' in cmdRes) {
                doubleVal = doubleVal * cmdRes?.kconv1 + cmdRes?.kconv2
                strVal = doubleVal.toString()
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
            if ('ipAddressLocalBind' in connection) {
              let ipPort = connection.ipAddressLocalBind.split(':')
              if (ipPort.length > 1) port = parseInt(ipPort[1])
            }

            let timeout = 15000
            if ('timeoutMs' in connection) {
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
            let historyProp = {}
            if (connection?.historyEnabled === true) {
              historyProp = {
                serverCapabilities: {
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
                },
              }
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
              maxSessions: 100,
              maxConnectionsPerEndpoint: 10,
              disableDiscovery: false,
              timeout: timeout,
              ...certificateProp,
              ...privateKeyProp,
              ...historyProp,
              // securityModes: [],
              // securityPolicies: [],
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

            server.start(function () {
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
            })

            // returns true when the given remote address is allowed to
            // connect. An empty/absent ipAddresses list allows any address.
            // A blank remote address is allowed (can't be matched reliably).
            const isRemoteAddressAllowed = (remoteAddress) => {
              if (
                !Array.isArray(connection.ipAddresses) ||
                connection.ipAddresses.length === 0
              )
                return true
              const addr = (remoteAddress || '').replace('::ffff:', '')
              if (addr === '') return true
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
              if (!isRemoteAddressAllowed(session?.channel?.remoteAddress)) {
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

            if ('topics' in connection && 'length' in connection.topics) {
              if (connection.topics.length > 0) {
                filterGroup1.group1 = { $in: connection.topics }
                Log.log('Filter tags: ' + JSON.stringify(filterGroup1))
              }
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
            for (let i = 0; i < res.length; i++) {
              if (
                res[i].protocolSourceBrowsePath &&
                typeof res[i].protocolSourceBrowsePath === 'string'
              ) {
                // try to find a variable with same browse name and path and use its parent as folder
                let found = false
                for (let k = 0; k < res.length; k++) {
                  if (
                    i !== k &&
                    res[k].group1 === res[i].group1 &&
                    res[k].protocolSourceBrowsePath &&
                    typeof res[k].protocolSourceBrowsePath === 'string' &&
                    res[k].protocolSourceObjectAddress.startsWith('ns=')
                  ) {
                    if (
                      res[k].protocolSourceBrowsePath +
                        '/' +
                        res[k].ungroupedDescription ===
                      res[i].protocolSourceBrowsePath
                    ) {
                      res[i]._componentOf = res[k].protocolSourceObjectAddress
                      res[i]._componentOfTag = res[k].tag
                      found = true
                      break
                    }
                  }
                }
                if (found) continue

                const browsePath = res[i].protocolSourceBrowsePath.split('/')
                let folder = 'ObjectsFolder'
                let pathKey = ''

                // when there is more than one topic, create a base folder for each topic
                if (connection.topics.length != 1) {
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
                      set: (variant) => {
                        sendCommand(element.tag, variant)
                        return StatusCodes.Good
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

                  let writeFlag = 0
                  if (
                    element.protocolSourceAccessLevel &&
                    typeof element.protocolSourceAccessLevel === 'string'
                  ) {
                    if (!isNaN(parseInt(element.protocolSourceAccessLevel)))
                      writeFlag =
                        parseInt(element.protocolSourceAccessLevel) &
                        AccessLevelFlag.CurrentWrite
                  }

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
                      (element.origin === 'command'
                        ? AccessLevelFlag.CurrentWrite
                        : writeFlag),
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
                      !('timeTagAtSource' in element) ||
                        element.timeTagAtSource === null
                        ? new Date(0)
                        : element.timeTagAtSource
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

            const csPipeline = [
              {
                $project: { documentKey: false },
              },
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
                        csFilterGroup1,
                        {
                          'fullDocument._id': {
                            $ne: -2,
                          },
                        },
                        {
                          'fullDocument._id': {
                            $ne: -1,
                          },
                        },
                        { operationType: 'update' },
                      ],
                    },
                    { operationType: 'replace' },
                  ],
                },
              },
            ]

            const changeStream = rtColl.watch(csPipeline, {
              fullDocument: 'updateLookup',
            })

            try {
              changeStream.on('error', (change) => {
                changeStream.on('change', () => {})
                if (clientMongo) clientMongo.close()
                clientMongo = null
                Log.log('MongoDB - Error on ChangeStream!')
              })
              changeStream.on('close', (change) => {
                clientMongo = null
                Log.log('MongoDB - Closed ChangeStream!')
              })
              changeStream.on('end', (change) => {
                changeStream.on('change', () => {})
                clientMongo = null
                Log.log('MongoDB - Ended ChangeStream!')
              })

              // start to listen for changes
              changeStream.on('change', (change) => {
                const v = convertValueVariant(change.fullDocument)
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
                        !('timeTagAtSource' in change.fullDocument) ||
                          change.fullDocument.timeTagAtSource === null
                          ? new Date(0)
                          : change.fullDocument.timeTagAtSource
                      )

                      if (Log.levelCurrent >= Log.levelDebug) {
                        Log.log(
                          srv._name + ' - Update: ' +
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

// find the connection number, if not found abort (only one connection per instance is allowed for this protocol)
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
