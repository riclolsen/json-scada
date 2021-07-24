var validationLevel = 'strict'
var validationAction = 'error'
var protocolDriverNames = [
          'IEC60870-5-104',
          'IEC60870-5-104_SERVER',
          'IEC60870-5-101',
          'IEC60870-5-101_SERVER',
          'IEC60870-5-103',
          'I104M',
          'DNP3',
          'DNP3_SERVER',
          'PLCTAG',
          'OPC-UA',
          'OPC-UA_SERVER',
          'OPC-DA',
          'OPC-DA_SERVER',
          'TELEGRAF-LISTENER',
          'MODBUS',
          'MODBUS_SERVER',
          'MQTT-SPARKPLUG-B',
          'IEC61850-GOOSE',
          'IEC61850-MMS',
          'IEC61850-MMS_SERVER',
          'CIP-ETHERNET/IP',
          'S7',
          'SPA-BUS',
          'BACNET',
          'ICCP',
          'UNDEFINED'
          ]

var protocolDriverInstancesValidator = {
  $jsonSchema: {
    bsonType: 'object',
    required: [
      'protocolDriver',
      'protocolDriverInstanceNumber',
      'enabled',
      'logLevel',
      'nodeNames'
    ],
    additionalProperties: true,
    properties: {
      _id: {
        bsonType: 'objectId'
      },
      protocolDriver: {
        bsonType: 'string',
        enum: protocolDriverNames,
        description: 'Driver name. Required.'
      },
      protocolDriverInstanceNumber: {
        bsonType: ['double', 'long', 'int'],
        minimum: 1,
        description:
          'Number id for the instance. Must be an integer > 0. Must not repeat for the same driver. Required.'
      },
      enabled: {
        bsonType: 'bool',
        enum: [true, false],
        description: 'Driver instance enabled. Required.'
      },
      logLevel: {
        bsonType: ['double', 'long', 'int'],
        minimum: 0,
        maximum: 3,
        description: 'Log level. 0=min,1=basic,2=detailed,3=debug. Required.'
      },
      nodeNames: {
        bsonType: 'array',
        minItems: 0,
        uniqueItems: true,
        additionalProperties: false,
        items: {
          bsonType: 'string',
          additionalProperties: false,
          description:
            'Names of nodes (computers) part of the system. Required.'
        }
      },
      activeNodeName: {
        bsonType: 'string',
        description:
          'Name of the currently (or last) active node for the driver instance. Should be on of the listed in nodeNames property.'
      },
      activeNodeKeepAliveTimeTag: {
        bsonType: 'date',
        description:
          'Date/time of the last keep alive update for the driver instance.'
      },
      keepProtocolRunningWhileInactive: {
        bsonType: 'bool',
        enum: [true, false],
        description: 'Keep protocol connection open while not the active node.'
      },
      softwareVersion: {
        bsonType: 'string',
        description: 'Software version.'
      },
      stats: {
        bsonType: ['object', 'null'],
        description: 'Driver specific statistics.'
      }
    }
  }
}

var processInstancesValidator = {
  $jsonSchema: {
    bsonType: 'object',
    required: [
      'processName',
      'processInstanceNumber',
      'enabled',
      'logLevel',
      'nodeNames'
    ],
    additionalProperties: true,
    properties: {
      _id: {
        bsonType: 'objectId'
      },
      processName: {
        bsonType: 'string',
        enum: [
          'CALCULATIONS',
          'CS_DATA_PROCESSOR',
          'SERVER_REALTIME',
          'ALARM_BEEP',
          'DYNAMIC_CALCULATIONS',
          'ALARM_PROCESSOR'
        ],
        description: 'Process name. Required.'
      },
      processInstanceNumber: {
        bsonType: ['double', 'long', 'int'],
        minimum: 1,
        description:
          'Number id for the instance. Must be an integer > 0. Must not repeat for the same process. Required.'
      },
      enabled: {
        bsonType: 'bool',
        enum: [true, false],
        description: 'Process instance enabled. Required.'
      },
      logLevel: {
        bsonType: ['double', 'long', 'int'],
        minimum: 0,
        maximum: 3,
        description: 'Log level. 0=min,1=basic,2=detailed,3=debug. Required.'
      },
      nodeNames: {
        bsonType: 'array',
        minItems: 0,
        uniqueItems: true,
        additionalProperties: false,
        items: {
          bsonType: 'string',
          additionalProperties: false,
          description:
            'Names of nodes (computers) part of the system. Required.'
        }
      },
      activeNodeName: {
        bsonType: 'string',
        description:
          'Name of the currently (or last) active node for the process instance. Should be on of the listed in nodeNames property.'
      },
      activeNodeKeepAliveTimeTag: {
        bsonType: 'date',
        description:
          'Date/time of the last keep alive update for the process instance.'
      },
      softwareVersion: {
        bsonType: 'string',
        description: 'Software version.'
      },
      periodOfCalculation: {
        bsonType: ['double', 'long', 'int'],
        description: 'Period of cycle of calculations in seconds.'
      },
      stats: {
        bsonType: ['object', 'null'],
        additionalProperties: true,
        description: 'Process specific statistics.',
        properties: {
          latencyAvg: {
            bsonType: ['double', 'long', 'int'],
            description:
              'Average latency of change stream processing in milliseconds.'
          },
          latencyAvgMinute: {
            bsonType: ['double', 'long', 'int'],
            description: 'Latency average over a minute in milliseconds.'
          },
          latencyPeak: {
            bsonType: ['double', 'long', 'int'],
            description: 'Absolute peak of latency in milliseconds.'
          }
        }
      }
    }
  }
}

var protocolConnectionsValidator = {
  $jsonSchema: {
    bsonType: 'object',
    required: [
      'protocolDriver',
      'protocolDriverInstanceNumber',
      'protocolConnectionNumber',
      'name',
      'description',
      'enabled',
      'commandsEnabled'
    ],
    additionalProperties: true,
    properties: {
      _id: {
        bsonType: 'objectId'
      },
      protocolDriver: {
        bsonType: 'string',
        enum: protocolDriverNames,
        description: 'Protocol driver name. Required.'
      },
      protocolDriverInstanceNumber: {
        bsonType: ['double', 'long', 'int'],
        minimum: 1,
        description:
          'Number id for the driver instance. Must be an integer > 0. Should match an existing instance defined in protocolDriverInstances. Required.'
      },
      protocolConnectionNumber: {
        bsonType: ['double', 'long', 'int'],
        minimum: 1,
        description:
          'Number code for identification of the connection. Required.'
      },
      name: {
        bsonType: 'string',
        description:
          'Short name describing the connection. This name will appear in protocol logs. Required.'
      },
      description: {
        bsonType: 'string',
        description: 'Long name describing the connection. Required.'
      },
      enabled: {
        bsonType: 'bool',
        enum: [true, false],
        description: 'Process instance enabled. Required.'
      },
      commandsEnabled: {
        bsonType: 'bool',
        enum: [true, false],
        description:
          'Must be true to allow commands to be forwarded in the connection. Required.'
      },
      logLevel: {
        bsonType: ['double', 'long', 'int'],
        minimum: 0,
        maximum: 3,
        description: 'Log level. 0=min,1=basic,2=detailed,3=debug. Required.'
      },
      ipAddressLocalBind: {
        bsonType: 'string',
        description:
          'IP address and port (like in 0.0.0.0:2404) for network listening.'
      },
      ipAddresses: {
        bsonType: 'array',
        minItems: 0,
        uniqueItems: true,
        additionalProperties: false,
        items: {
          bsonType: 'string',
          additionalProperties: false,
          description:
            'IP addresses of servers to connect or clients allowed to connect.'
        }
      },
      endpointURLs: {
        bsonType: 'array',
        minItems: 0,
        uniqueItems: true,
        additionalProperties: false,
        items: {
          bsonType: 'string',
          additionalProperties: false,
          description:
            'OPC-UA server URLs.'
        }
      },
      localLinkAddress: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Local link address.'
      },
      remoteLinkAddress: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Remote link address.'
      },
      giInterval: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Time in seconds for Station General Interrogation.'
      },
      testCommandInterval: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Time in seconds for Test Command.'
      },
      timeSyncInterval: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Time in seconds for time sync operation.'
      },
      sizeOfCOT: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 1,
        minimum: 2,
        description: 'Size of cause of transmission field.'
      },
      sizeOfCA: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 1,
        minimum: 2,
        description: 'Size of cause of common address field.'
      },
      sizeOfIOA: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 1,
        minimum: 3,
        description: 'Size of cause of information object address field.'
      },
      k: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Protocol parameter.'
      },
      w: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Protocol parameter.'
      },
      t0: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Protocol parameter in seconds.'
      },
      t1: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Protocol parameter in seconds.'
      },
      t2: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Protocol parameter in seconds.'
      },
      t3: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Protocol parameter in seconds.'
      },
      serverModeMultiActive: {
        bsonType: 'bool',
        enum: [true, false],
        description: 'Protocol parameter.'
      },
      maxClientConnections: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Protocol parameter.'
      },
      maxQueueSize: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Protocol parameter.'
      },
      portName: {
        bsonType: ['string', 'null'],
        description: 'Serial port parameter.'
      },
      baudRate: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Serial port parameter.'
      },
      stopBits: {
        bsonType: ['string', 'null'],
        enum: ['One', 'One5', 'Two'],
        description: 'Serial port parameter.'
      },
      handshake: {
        bsonType: ['string', 'null'],
        enum: ['None', 'Xon', 'Rts', 'RtsXon'],
        description: 'Serial port parameter.'
      },
      timeoutForACK: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Protocol parameter.'
      },
      timeoutRepeat: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        description: 'Protocol parameter.'
      },
      useSingleCharACK: {
        bsonType: 'bool',
        enum: [true, false],
        description: 'Protocol parameter.'
      },
      sizeOfLinkAddress: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        maximum: 2,
        description: 'Protocol parameter.'
      },
      asyncOpenDelay: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        maximum: 2,
        description: 'Protocol parameter.'
      },
      class0ScanInterval: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        maximum: 2,
        description: 'Protocol parameter.'
      },
      class1ScanInterval: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        maximum: 2,
        description: 'Protocol parameter.'
      },
      class2ScanInterval: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        maximum: 2,
        description: 'Protocol parameter.'
      },
      class3ScanInterval: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        maximum: 2,
        description: 'Protocol parameter.'
      },
      timeSyncMode: {
        bsonType: ['double', 'long', 'int', 'null'],
        minimum: 0,
        maximum: 2,
        description: 'Protocol parameter.'
      },
      enableUnsolicited: {
        bsonType: ['bool', 'null'],
        enum: [true, false],
        description: 'Protocol parameter.'
      },
      rangeScans: {
        bsonType: 'array',
        minItems: 0,
        uniqueItems: true,
        additionalProperties: false,
        items: {
          bsonType: ['object', 'null'],
          additionalProperties: true,
          properties: {
            group: {
              bsonType: ['double', 'long', 'int', 'null'],
              minimum: 0
            },
            variation: {
              bsonType: ['double', 'long', 'int', 'null'],
              minimum: 0
            },
            startAddress: {
              bsonType: ['double', 'long', 'int', 'null'],
              minimum: 0
            },
            stopAddress: {
              bsonType: ['double', 'long', 'int', 'null'],
              minimum: 0
            },
            period: {
              bsonType: ['double', 'long', 'int', 'null'],
              minimum: 0
            }
          }
        }
      },
      allowTLSv10: {
        bsonType: ['bool', 'null']
      },
      allowTLSv11: {
        bsonType: ['bool', 'null']
      },
      allowTLSv12: {
        bsonType: ['bool', 'null']
      },
      allowTLSv13: {
        bsonType: ['bool', 'null']
      },
      chainValidation: {
        bsonType: ['bool', 'null']
      },
      allowOnlySpecificCertificates: {
        bsonType: ['bool', 'null']
      },
      cipherList: {
        bsonType: ['string', 'null']
      },
      localCertFilePath: {
        bsonType: ['string', 'null']
      },
      peerCertFilePath: {
        bsonType: ['string', 'null']
      },
      rootCertFilePath: {
        bsonType: ['string', 'null']
      },
      privateKeyFilePath: {
        bsonType: ['string', 'null']
      },
      autoCreateTags: {
        bsonType: ['bool', 'null']
      },
      autoCreateTagPublishingInterval: {
        bsonType: ['double', 'null']
       },
      autoCreateTagSamplingInterval: {
        bsonType: ['double', 'null']
       },
      autoCreateTagQueueSize: {
        bsonType: ['double', 'null']
       },
      configFileName: {
        bsonType: ['string', 'null']
      },
      topics: {
        bsonType: 'array',
        minItems: 0,
        uniqueItems: true,
        additionalProperties: false,
        items: {
          bsonType: 'string',
          additionalProperties: false,
          description:
            'MQTT topics'
        }
      },
      topicsAsFiles: {
        bsonType: 'array',
        minItems: 0,
        uniqueItems: true,
        additionalProperties: false,
        items: {
          bsonType: 'string',
          additionalProperties: false,
          description:
            'MQTT topics as binary files'
        }
      },
      topicsScripted: {
        bsonType: 'array',
        minItems: 0,
        uniqueItems: true,
        additionalProperties: false,
        items: {
          bsonType: ['object', 'null'],
          additionalProperties: true,
          properties: {
            topic: {
              bsonType: 'string'
            },
            script: {
              bsonType: 'string'
            }
          }
        }
      },
      clientId: {
        bsonType: ['string', 'null']
      },
      groupId: {
        bsonType: ['string', 'null']
      },
      edgeNodeId: {
        bsonType: ['string', 'null']
      },
      deviceId: {
        bsonType: ['string', 'null']
      },
      scadaHostId: {
        bsonType: ['string', 'null']
      },
      publishTopicRoot: {
        bsonType: ['string', 'null']
      },
      username: {
        bsonType: ['string', 'null']
      },
      password: {
        bsonType: ['string', 'null']
      },
      pfxFilePath: {
        bsonType: ['string', 'null']
      },
      passphrase: {
        bsonType: ['string', 'null']
      },
      stats: {
        bsonType: ['object', 'null'],
        description: 'Driver specific statistics.'
      }
    }
  }
}

// if server is >= 5.0 enable new feature set and create hist time series collection
if (db.version().charAt(0) >= 5){
  db.adminCommand( { setFeatureCompatibilityVersion: "5.0" } )
  db.createCollection(
    "hist",
    {
       timeseries: {
          timeField: "timeTag",
          metaField: "tag",
          granularity: "seconds"
       },
       expireAfterSeconds: 60*60*24*30*2
    }
  )
  db.hist.createIndex({ "tag": 1, "timeTag": 1 })  
}

db.createCollection('protocolDriverInstances', {
  validationLevel: validationLevel,
  validationAction: validationAction,
  validator: protocolDriverInstancesValidator
})
db.protocolDriverInstances.createIndex(
  { protocolDriver: 1, protocolDriverInstanceNumber: 1 },
  { name: 'protocolDriverInstancesIndex', unique: true }
)

db.createCollection('processInstances', {
  validationLevel: validationLevel,
  validationAction: validationAction,
  validator: processInstancesValidator
})
db.processInstances.createIndex(
  { processName: 1, processInstanceNumber: 1 },
  { name: 'processInstancesIndex', unique: true }
)

db.createCollection('protocolConnections', {
  validationLevel: validationLevel,
  validationAction: validationAction,
  validator: protocolConnectionsValidator
})
db.protocolConnections.createIndex(
  { protocolConnectionNumber: 1 },
  { name: 'protocolConnectionNumberIndex', unique: true }
)

db.createCollection('commandsQueue')
db.createCollection('realtimeData')
db.realtimeData.createIndex({ tag: 1 }, { name: 'tagIndex', unique: true })
db.realtimeData.createIndex({
  protocolSourceConnectionNumber: 1,
  protocolSourceCommonAddress: 1,
  protocolSourceObjectAddress: 1
})
db.realtimeData.createIndex({
  group1: 1,
  group2: 1
})
db.realtimeData.createIndex({
  alarmed: 1
})

// soeData is defined as a capped collection (limited size 2GB, circular buffer)
// remove the parameters to create as a normal collection to overcome the size restriction
db.createCollection('soeData', { capped: true, size: 2000000000 })
db.soeData.createIndex({ timeTag: 1 })
db.soeData.createIndex({ timeTagAtSource: 1 })
db.soeData.createIndex({ group1: 1 })
db.soeData.createIndex({ ack: 1 })

db.createCollection('roles')
db.roles.createIndex({ name: 1 }, { name: 'roleNameIndex', unique: true })
db.createCollection('users')
db.users.createIndex({ username: 1 }, { name: 'userNameIndex', unique: true })

db.createCollection('userActions')
db.userActions.createIndex({ timeTag: 1 }, { name: 'actionsTimeTagIndex' })
// use this to make records expire after a number of seconds
// db.userActions.createIndex( { timeTag: 1 }, { expireAfterSeconds: 2592000 } )

