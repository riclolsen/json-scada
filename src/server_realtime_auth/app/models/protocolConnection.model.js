const mongoose = require('mongoose')
const Double = require('./double')

const ProtocolConnection = mongoose.model(
  'ProtocolConnection',
  new mongoose.Schema({
    protocolDriver: { type: String, required: true, default: 'UNDEFINED' },
    protocolDriverInstanceNumber: {
      type: Double,
      required: true,
      default: 1.0,
    },
    protocolConnectionNumber: {
      type: Double,
      required: true,
      unique: true,
      min: 1,
      default: 1.0,
    },
    name: { type: String, required: true, default: 'NEW_CONNECTION' },
    description: { type: String, required: true, default: 'NEW CONNECTION' },
    enabled: { type: Boolean, required: true, default: true },
    commandsEnabled: { type: Boolean, required: true, default: true },
    stats: { type: Object, default: null },

    // IEC 104 Server, I104M, TELEGRAF_LISTENER, OPC-UA_SERVER, IEC61850_SERVER
    ipAddressLocalBind: { type: String },

    // IEC 104 Server and Client, DNP3 Client, PLCTag, I104M, TELEGRAF_LISTENER, OPC-UA_SERVER, IEC61850 Server and CLient
    ipAddresses: { type: [String], default: [] },

    // MQTT-SPARKPLUG-B, OPC-UA_SERVER, IEC61850 CLient
    topics: { type: [String], default: [] },
    // MQTT-SPARKPLUG-B, OPC-UA_SERVER
    groupId: { type: String },

    // MQTT-SPARKPLUG-B, IEC 104 Server, IEC 104 Client
    passphrase: { type: String, default: '' },

    // MQTT-SPARKPLUG-B
    topicsAsFiles: { type: [String], default: [] },
    topicsScripted: { type: [Object], default: [] },
    clientId: { type: String },
    edgeNodeId: { type: String },
    deviceId: { type: String },
    scadaHostId: { type: String },
    publishTopicRoot: { type: String },
    pfxFilePath: { type: String, default: '' },
    username: { type: String },

    // MQTT-SPARKPLUG-B, IEC61850 Client and Server
    password: { type: String },

    // OPC-UA Client, TELEGRAF_LISTENER, MQTT-SPARKPLUG-B, IEC61850 Client
    autoCreateTags: { type: Boolean, default: true },

    // OPC-UA Client
    autoCreateTagPublishingInterval: { type: Double, min: 0, default: 2.5 },
    autoCreateTagSamplingInterval: { type: Double, min: 0, default: 0.0 },
    autoCreateTagQueueSize: { type: Double, min: 0, default: 5.0 },

    // OPC-UA Client, MQTT-SPARKPLUG-B, OPC-UA_SERVER, IEC61850 Client and Server
    useSecurity: { type: Boolean, default: false },

    // OPC-UA Client, MQTT-SPARKPLUG-B
    endpointURLs: { type: [String], default: [] },

    // OPC-UA Client, OPC-UA_SERVER
    timeoutMs: { type: Double, min: 0, default: 20000 },

    // OPC-UA Client
    configFileName: {
      type: String,
      default: '../conf/Opc.Ua.DefaultClient.Config.xml',
    },

    // IEC 104 Server and Client, DNP3, PLCTag, I104M
    localLinkAddress: { type: Double, min: 0, default: 1.0 },
    remoteLinkAddress: { type: Double, min: 0, default: 1.0 },

    // IEC 104 Server and Client, DNP3, PLCTag, I104M, IEC61850 Client
    giInterval: { type: Double, min: 0, default: 300.0 },

    // IEC 101/104 Client
    testCommandInterval: { type: Double, min: 0, default: 0.0 },
    timeSyncInterval: { type: Double, min: 0, default: 0.0 },

    // IEC 101/104 Server and Client
    sizeOfCOT: { type: Double, min: 1, default: 2.0 },
    sizeOfCA: { type: Double, min: 1, default: 2.0 },
    sizeOfIOA: { type: Double, min: 1, default: 3.0 },

    // IEC 104 Server and Client
    k: { type: Double, min: 1, default: 12.0 },
    w: { type: Double, min: 1, default: 8.0 },
    t0: { type: Double, min: 1, default: 10.0 },
    t1: { type: Double, min: 1, default: 15.0 },
    t2: { type: Double, min: 1, default: 10.0 },
    t3: { type: Double, min: 1, default: 20.0 },

    // IEC 104 Server, IEC61850 Server
    serverModeMultiActive: { type: Boolean, default: true },
    maxClientConnections: { type: Double, min: 1, default: 1.0 },

    // IEC 101/104 Server, IEC61850 Server
    maxQueueSize: { type: Double, min: 0, default: 5000.0 },

    // IEC 104 Client/Server, DNP3 Client, MQTT-SPARKPLUG-B, OPC-UA_SERVER, IEC61850 Client and Server
    localCertFilePath: { type: String, default: '' },

    // IEC 104 Client/Server, DNP3 Client, IEC61850 Client and Server
    peerCertFilePath: { type: String, default: '' },

    // IEC 104 Server, IEC61850 Client and Server
    peerCertFilesPaths: { type: [String], default: [] },

    // IEC 104 Client/Server, MQTT-SPARKPLUG-B, IEC61850 Client and Server
    rootCertFilePath: { type: String, default: '' },
    chainValidation: { type: Boolean, default: false },

    // IEC 104 Client/Server, IEC61850 Client and Server
    allowOnlySpecificCertificates: { type: Boolean, default: false },

    // DNP3 Client, MQTT-SPARKPLUG-B, OPC-UA_SERVER, IEC61850 Client and Server
    privateKeyFilePath: { type: String, default: '' },
    allowTLSv10: { type: Boolean, default: false },
    allowTLSv11: { type: Boolean, default: false },
    allowTLSv12: { type: Boolean, default: true },
    allowTLSv13: { type: Boolean, default: true },
    cipherList: { type: String, default: '' },

    // DNP3 Client
    asyncOpenDelay: { type: Double, min: 0.0, default: 0.0 },
    timeSyncMode: { type: Double, min: 0.0, max: 2.0, default: 0.0 },
    class0ScanInterval: { type: Double, min: 0.0, default: 0.0 },
    class1ScanInterval: { type: Double, min: 0.0, default: 0.0 },
    class2ScanInterval: { type: Double, min: 0.0, default: 0.0 },
    class3ScanInterval: { type: Double, min: 0.0, default: 0.0 },
    enableUnsolicited: { type: Boolean, default: true },
    rangeScans: { type: Array, default: [] },

    // IEC 101 Server and Client, DNP3 Client
    portName: { type: String },
    baudRate: { type: Double, min: 150, default: 9600.0 },
    parity: { type: String, default: 'Even' },
    stopBits: { type: String, default: 'One' },
    handshake: { type: String, default: 'None' },

    // IEC 101 Server and Client
    timeoutForACK: { type: Double, min: 1, default: 1000 },
    timeoutRepeat: { type: Double, min: 1, default: 1000 },
    useSingleCharACK: { type: Boolean, default: true },
    sizeOfLinkAddress: { type: Double, min: 0, default: 1 },
  }),
  'protocolConnections'
)

module.exports = ProtocolConnection
