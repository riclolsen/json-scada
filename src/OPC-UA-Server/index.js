'use strict'

/*
 * OPC-UA Server Driver for JSON-SCADA
 *
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

const {
  OPCUAServer,
  SessionContext,
  Variant,
  DataValue,
  DataType,
  StatusCodes,
  AttributeIds
} = require('node-opcua')
const MongoClient = require('mongodb').MongoClient
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')
const { LoadConfig, getMongoConnectionOptions } = require('./load-config')

;(async () => {
  const jsConfig = LoadConfig() // load and parse config file
  Log.levelCurrent = jsConfig.LogLevel

  let metrics = []
  let rtCollection = null
  let cmdCollection = null
  let clientMongo = null
  let connsCollection = null

  while (true) {
    if (clientMongo === null)
      // if disconnected
      await MongoClient.connect(
        // try to (re)connect
        jsConfig.mongoConnectionString,
        getMongoConnectionOptions(jsConfig)
      ).then(async client => {
        // connected
        Log.log('MongoDB - Connected correctly to MongoDB server', Log.levelMin)
        clientMongo = client
        // specify db and collections
        const db = client.db(jsConfig.mongoDatabaseName)
        rtCollection = db.collection(jsConfig.RealtimeDataCollectionName)
        cmdCollection = db.collection(jsConfig.CommandsQueueCollectionName)
        connsCollection = db.collection(
          jsConfig.ProtocolConnectionsCollectionName
        )

        // find the connection number, if not found abort (only one connection per instance is allowed for this protocol)
        let connection = await getConnection(connsCollection, jsConfig)

        if (!'_id' in connection) {
          Log.log('Fatal error: malformed record for connection found!')
          process.exit(1)
        }

        let certificateProp = {}
        if (
          'localCertFilePath' in connection &&
          'length' in connection.localCertFilePath
        ) {
          if (connection.localCertFilePath.length > 0)
            certificateProp.certificateFile = connection.localCertFilePath
        }
        let privateKeyProp = {}
        if (
          'privateKeyFilePath' in connection &&
          'length' in connection.privateKeyFilePath
        ) {
          if (connection.privateKeyFilePath.length > 0)
            privateKeyProp.privateKeyFile = connection.privateKeyFilePath
        }

        let port = 4840
        if ('ipAddressLocalBind' in connection) {
          let ipPort = connection.ipAddressLocalBind.split(':')
          if (ipPort.length > 1) port = parseInt(ipPort[1])
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
            productUri: 'https://github.com/riclolsen/json-scada/'
            // buildDate: new Date()
          },
          maxAllowedSessionNumber: 100,
          maxConnectionsPerEndpoint: 10,
          disableDiscovery: false,
          timeout: 15000,
          ...certificateProp,
          ...privateKeyProp
          // securityModes: [],
          // securityPolicies: [],
          // defaultSecureTokenLifetime: 10000000,
          // certificateFile: "", // PEM file
          // privateKeyFile: "", // PEM file
        })
        await server.initialize()
        Log.log('OPC-UA Server initialized.')

        const addressSpace = server.engine.addressSpace
        const namespace = addressSpace.getOwnNamespace()

        // declare a new object
        const device = namespace.addObject({
          organizedBy: addressSpace.rootFolder.objects,
          browseName: 'JsonScadaServer'
        })

        server.start(function () {
          Log.log('Server is now listening ... (press CTRL+C to stop)')
          const endpointUrl = server.endpoints[0].endpointDescriptions()[0]
            .endpointUrl
          Log.log('Server endpoint url is ' + endpointUrl)
        })

        server.on('newChannel', function (channel, endpoint) {
          Log.log('New Channel, remote address: ' + channel.remoteAddress)
        })

        server.on('create_session', function (session) {
          Log.log('Creating session.')
          Log.log(
            'Client description, application URI: ' +
              session?.parent?.clientDescription?.applicationUri
          )
          Log.log('Remote Address: ' + session?.channel?._remoteAddress)

          if (
            'ipAddresses' in connection &&
            'length' in connection.ipAddresses
          ) {
            if (
              connection.ipAddresses.length > 0 &&
              session?.channel?._remoteAddress != ''
            )
              if (
                !connection.ipAddresses.includes(
                  session.channel._remoteAddress.replace('::ffff:', '')
                )
              ) {
                Log.log('IP not authorized: closing session!')
                try {
                  session.close()
                } catch (e) {}
                session.dispose()
              }
          }
        })

        // console.log(connection)

        let filterGroup1 = {}
        let filterGroup1CS = {
          'fullDocument.group1': {
            $exists: true
        }}

        if ('topics' in connection && 'length' in connection.topics) {
          if (connection.topics.length > 0) {
            filterGroup1.group1 = { $in: connection.topics }
            filterGroup1CS = {
              'fullDocument.group1': { $in: connection.topics }
            }
            Log.log('Filter tags: ' + JSON.stringify(filterGroup1))
          }
        }

        let res = await rtCollection
          .find(
            {
              protocolSourceConnectionNumber: {
                $ne: connection.protocolConnectionNumber
              }, // exclude data from the same connection
              ...filterGroup1,
              ...(connection.commandsEnabled
                ? {}
                : { origin: { $ne: 'command' } }),
              _id: { $gt: 0 }
            },
            {
              projection: {
                _id: 1,
                tag: 1,
                type: 1,
                value: 1,
                valueString: 1,
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
                protocolSourceConnectionNumber: 1
              }
            }
          )
          .toArray()

        Log.log(`Creating ${res.length} OPC UA Variables...`)
        res.forEach(element => {
          if (element._id <= 0) {
            // exclude internal system data
            return
          }

          let type, value, dataType
          switch (element.type) {
            case 'digital':
              type = 'Boolean'
              dataType = DataType.Boolean
              value = element.value === 0 ? false : true
              break
            case 'string':
              type = 'String'
              dataType = DataType.String
              if ('valueString' in element) value = element.valueString
              else value = element.value.toString()
              break
            case 'analog':
              type = 'Double'
              dataType = DataType.Double
              value = element.value
              break
            default:
              return
          }
          if (type) {
            metrics[element.tag] = namespace.addVariable({
              componentOf: device,
              nodeId: 'ns=1;i=' + element._id,
              browseName: element.tag,
              dataType: type,
              description: element?.description
              // statusCode: StatusCodes.Bad,
              //value: {
              //  get:
              //  () =>
              //  new Variant({ dataType: DataType.Double,
              //                value: getValue(element.tag, rtCollection) })
              //}
            })
            metrics[element.tag].setValueFromSource(
              new Variant({
                dataType: dataType,
                value: value
              }),
              element.invalid ? StatusCodes.Bad : StatusCodes.Good,
              element.timeTagAtSource === null
                ? new Date(1970, 0, 1)
                : element.timeTagAtSource
            )
          }
        })
        Log.log(`Finished creating OPC UA Variables.`)

        //        setInterval(async () => {
        //          metrics['BRA-AM-MW'].setValueFromSource(
        //            new Variant({ dataType: DataType.Double, value: await getValue('BRA-AM-MW', rtCollection) }),
        //            StatusCodes.Good
        //          )
        //
        //
        //          //  const writeValue = {
        //          //      attributeId: AttributeIds.Value,
        //          //      dataValue: new DataValue({
        //          //          statusCode: StatusCodes.Good,
        //          //          sourceTimestamp: new Date(),
        //          //          value: new Variant({ dataType: DataType.Double, value: 3.14 })
        //          //      }),
        //          //      // nodeId
        //          //   };
        //          //   metrics['BRA-AM-MW'].writeAttribute(SessionContext.defaultContext,writeValue,(err, statusCode) => {
        //          //    if (err) { console.log("Write has failed"); return; }
        //          //    console.log("write statusCode = ",statusCode.toString());
        //          // });
        //
        //        }, 5000)

        const csPipeline = [
          {
            $project: { documentKey: false }
          },
          {
            $match: {
              $or: [
                {
                  $and: [
                    {
                      'updateDescription.updatedFields.sourceDataUpdate': {
                        $exists: false
                      }
                    },
                    filterGroup1CS,
                    {
                      'fullDocument._id': {
                        $ne: -2
                      }
                    },
                    {
                      'fullDocument._id': {
                        $ne: -1
                      }
                    },
                    { operationType: 'update' }
                  ]
                },
                { operationType: 'replace' }
              ]
            }
          }
        ]

        let changeStream = rtCollection.watch(csPipeline, {
          fullDocument: 'updateLookup'
        })

        try {
          changeStream.on('error', change => {
            changeStream.on('change', () => {})
            if (clientMongo) clientMongo.close()
            clientMongo = null
            Log.log('MongoDB - Error on ChangeStream!')
          })
          changeStream.on('close', change => {
            changeStream.on('change', () => {})
            if (clientMongo) clientMongo.close()
            clientMongo = null
            Log.log('MongoDB - Closed ChangeStream!')
          })
          changeStream.on('end', change => {
            changeStream.on('change', () => {})
            if (clientMongo) clientMongo.close()
            clientMongo = null
            Log.log('MongoDB - Ended ChangeStream!')
          })

          // start to listen for changes
          // a mongo disconnection produces a fatal error here!
          changeStream.on('change', change => {
            let m = metrics[change.fullDocument?.tag]
            Log.log(change.fullDocument?.tag + ' ' + change.fullDocument.value, Log.levelDetailed)
            if (m !== undefined)
              switch (change.fullDocument?.type) {
                case 'analog':
                  m.setValueFromSource(
                    new Variant({
                      dataType: DataType.Double,
                      value: change.fullDocument?.value
                    }),
                    change.fullDocument?.invalid
                      ? StatusCodes.Bad
                      : StatusCodes.Good,
                    new Date(1970, 0, 1)
                  )
                  break
                case 'digital':
                  m.setValueFromSource(
                    new Variant({
                      dataType: DataType.Boolean,
                      value: change.fullDocument?.value === 0 ? false : true
                    }),
                    change.fullDocument?.invalid
                      ? StatusCodes.Bad
                      : StatusCodes.Good,
                    change.fullDocument?.timeTagAtSource
                  )
                  break
                case 'string':
                  m.setValueFromSource(
                    new Variant({
                      dataType: DataType.String,
                      value: change.fullDocument?.valueString
                    }),
                    change.fullDocument?.invalid
                      ? StatusCodes.Bad
                      : StatusCodes.Good,
                    change.fullDocument?.timeTagAtSource
                  )
                  break
              }
          })
        } catch (e) {
          Log.log('MongoDB - CS Error: ' + e, Log.levelMin)
        }
      })

    // wait 5 seconds
    await new Promise(resolve => setTimeout(resolve, 5000))

    if (!(await checkConnectedMongo(clientMongo))) {
      clientMongo = null
    }

    // detect connection problems, if error will null the client to later reconnect
    if (!clientMongo) {
      Log.log('MongoDB - Disconnected Mongodb!')
      clientMongo = null
      rtCollection = null
      cmdCollection = null
      connsCollection = null
    }
  }
})()

// find the connection number, if not found abort (only one connection per instance is allowed for this protocol)
async function getConnection (connsCollection, configObj) {
  let results = await connsCollection
    .find({
      protocolDriver: AppDefs.NAME,
      protocolDriverInstanceNumber: configObj.Instance
    })
    .toArray()

  if (!results || !('length' in results) || results.length == 0) {
    Log.log('Connection - No protocol connection found!')
    process.exit(1)
  }
  const connection = results[0]
  if (!('protocolConnectionNumber' in connection)) {
    Log.log('Connection - No protocol connection found on record!')
    process.exit(2)
  }
  if (connection.enabled === false) {
    Log.log(
      'Connection - Connection disabled, exiting! (connection:' +
        connection.protocolConnectionNumber +
        ')'
    )
    process.exit(3)
  }
  return connection
}

async function getValue (tag, rtCollection) {
  if (rtCollection === null) return 0

  let results = await rtCollection
    .find(
      {
        tag: tag
      },
      {
        projection: {
          _id: 1,
          tag: 1,
          type: 1,
          value: 1,
          valueString: 1,
          timeTag: 1,
          timeTagAtSource: 1,
          timeTagAtSourceOk: 1,
          invalid: 1,
          isEvent: 1
        }
      }
    )
    .toArray()

  if (!results || !('length' in results) || results.length == 0) {
    return 0
  }

  console.log(results[0].value)
  return parseFloat(results[0].value)
}

// test mongoDB connectivity
let CheckMongoConnectionTimeout = 1000
let HintMongoIsConnected = true
async function checkConnectedMongo (client) {
  if (!client) {
    return false
  }

  let tr = setTimeout(() => {
    console.log('Mongo ping timeout error!')
    HintMongoIsConnected = false
  }, CheckMongoConnectionTimeout)

  let res = null
  try {
    res = await client.db('admin').command({ ping: 1 })
    clearTimeout(tr)
  } catch (e) {
    console.log('Error on mongodb connection!')
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
