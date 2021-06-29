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
const Mongo = require('mongodb')
const MongoClient = require('mongodb').MongoClient
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')
const { LoadConfig, getMongoConnectionOptions } = require('./load-config')
const Redundancy = require('./redundancy')

;(async () => {
  const jsConfig = LoadConfig() // load and parse config file
  Log.logLevelCurrent = jsConfig.LogLevel

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

  let metrics = []
  let rtCollection = null
  let cmdCollection = null
  let clientMongo = null
  let connsCollection = null

  // Let's create an instance of OPCUAServer
  const server = new OPCUAServer({
    port: 4334, // the port of the listening socket of the server
    resourcePath: '/UA/MyLittleServer', // this path will be added to the endpoint resource name
    buildInfo: {
      productName: 'MySampleServer1',
      buildNumber: '7658',
      buildDate: new Date(2014, 5, 2)
    }
  })
  await server.initialize()
  console.log('initialized')

  const addressSpace = server.engine.addressSpace
  const namespace = addressSpace.getOwnNamespace()

  // declare a new object
  const device = namespace.addObject({
    organizedBy: addressSpace.rootFolder.objects,
    browseName: 'JsonScadaServer'
  })

  // add some variables
  // add a variable named MyVariable1 to the newly created folder "MyDevice"
  let variable1 = 1

  // emulate variable1 changing every 500 ms
  setInterval(() => {
    variable1 += 1
  }, 500)

  namespace.addVariable({
    componentOf: device,
    browseName: 'MyVariable1',
    dataType: 'Double',
    value: {
      get: () => new Variant({ dataType: DataType.Double, value: variable1 })
    }
  })

  const os = require('os')

  /**
   * returns the percentage of free memory on the running machine
   * @return {double}
   */
  function available_memory () {
    // var value = process.memoryUsage().heapUsed / 1000000;
    const percentageMemUsed = (os.freemem() / os.totalmem()) * 100.0
    return percentageMemUsed
  }
  namespace.addVariable({
    componentOf: device,

    nodeId: 's=free_memory', // a string nodeID
    browseName: 'FreeMemory',
    dataType: 'Double',
    value: {
      get: () =>
        new Variant({ dataType: DataType.Double, value: available_memory() })
    }
  })

  server.start(function () {
    console.log('Server is now listening ... ( press CTRL+C to stop)')
    console.log('port ', server.endpoints[0].port)
    const endpointUrl = server.endpoints[0].endpointDescriptions()[0]
      .endpointUrl
    console.log(' the primary server endpoint url is ', endpointUrl)
  })

  while (true) {
    if (clientMongo === null)
      // if disconnected
      await MongoClient.connect(
        // try to (re)connect
        jsConfig.mongoConnectionString,
        getMongoConnectionOptions(jsConfig, MongoClient)
      ).then(async client => {
        // connected
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

        let res = await rtCollection
          .find(
            {
              protocolSourceConnectionNumber: {
                $ne: connection.protocolConnectionNumber
              }, // exclude data from the same connection
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
              value = element.value===0 ? false : true
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
              description: element?.description,
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
              element.invalid?StatusCodes.Bad:StatusCodes.Good,
              element.timeTagAtSource===null? new Date(1970,0,1):element.timeTagAtSource
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

        const changeStream = rtCollection.watch(csPipeline, {
          fullDocument: 'updateLookup'
        })

        try {
          changeStream.on('error', change => {
            if (clientMongo) clientMongo.close()
            clientMongo = null
            Log.log('MongoDB - Error on ChangeStream!')
          })
          changeStream.on('close', change => {
            if (clientMongo) clientMongo.close()
            clientMongo = null
            Log.log('MongoDB - Closed ChangeStream!')
          })
          changeStream.on('end', change => {
            if (clientMongo) clientMongo.close()
            clientMongo = null
            Log.log('MongoDB - Ended ChangeStream!')
          })

          // start listen to changes
          changeStream.on('change', change => {
            let m = metrics[change.fullDocument?.tag]
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
                    new Date(1970,0,1)
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

        Log.log('MongoDB - Connected correctly to MongoDB server', Log.levelMin)
      })

    // wait 5 seconds
    await new Promise(resolve => setTimeout(resolve, 5000))

    // detect connection problems, if error will null the client to later reconnect
    if (!clientMongo) {
      Log.log('MongoDB - Disconnected Mongodb!')
      clientMongo = null
      rtCollection = null
      cmdCollection = null
      connsCollection = null
    } else {
      if (!clientMongo.isConnected()) {
        // not anymore connected, will retry
        Log.log('MongoDB - Disconnected Mongodb!')
        clientMongo.close()
        clientMongo = null
        rtCollection = null
        cmdCollection = null
        connsCollection = null
      }
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
