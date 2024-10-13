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

    // IEC60870-5-104_SERVER, I104M, TELEGRAF_LISTENER, OPC-UA_SERVER, IEC61850_SERVER, ICCP_SERVER, DNP3_SERVER
    ipAddressLocalBind: { type: String, default: '' },

    // IEC60870-5-104, IEC60870-5-104_SERVER, DNP3, PLCTag, I104M, TELEGRAF_LISTENER, OPC-UA_SERVER, IEC61850, IEC61850_SERVER, ICCP, ICCP_SERVER
    ipAddresses: { type: [String], default: [] },

    // MQTT-SPARKPLUG-B, OPC-UA_SERVER, IEC61850, PLC4X, OPC_DA, OPC-DA_SERVER, ICCP, ICCP_SERVER
    topics: { type: [String], default: [] },

    // ICCP, ICCP_SERVER
    domain : { type: String, default: '' },
    aeQualifier: { type: Double, default: 12 },
    localAppTitle: { type: String, default: '1.1.1.998' },
    localSelectors: { type: String, default: '0 0 0 2 0 2 0 2' },

    // ICCP
    remoteAppTitle: { type: String, default: '1.1.1.999' },
    remoteSelectors: { type: String, default: '0 0 0 1 0 1 0 1' },

    // MQTT-SPARKPLUG-B, OPC-UA_SERVER
    groupId: { type: String, default: '' },

    // MQTT-SPARKPLUG-B, IEC60870-5-104, IEC60870-5-104_SERVER
    passphrase: { type: String, default: '' },

    // MQTT-SPARKPLUG-B
    topicsAsFiles: { type: [String], default: [] },
    topicsScripted: { type: [Object], default: [] },
    clientId: { type: String, default: '' },
    edgeNodeId: { type: String, default: '' },
    deviceId: { type: String, default: '' },
    scadaHostId: { type: String, default: '' },
    publishTopicRoot: { type: String, default: '' },
    pfxFilePath: { type: String, default: '' },

    // MQTT-SPARKPLUG-B, IEC61850, IEC61850_SERVER, OPC-DA
    username: { type: String, default: '' },
    password: { type: String, default: '' },

    // OPC-UA, TELEGRAF_LISTENER, MQTT-SPARKPLUG-B, IEC61850, PLC4X, OPC-DA, ICCP
    autoCreateTags: { type: Boolean, default: true },

    // OPC-UA, OPC-DA, OPC-DA_SERVER, ICCP, ICCP_SERVER
    autoCreateTagPublishingInterval: { type: Double, min: 0, default: 2.5 },
    
    // OPC-UA, OPC-DA
    autoCreateTagSamplingInterval: { type: Double, min: 0, default: 0.0 },
    autoCreateTagQueueSize: { type: Double, min: 0, default: 5.0 },

    // OPC-UA, MQTT-SPARKPLUG-B, OPC-UA_SERVER, IEC61850, IEC61850_SERVER, OPC-DA
    useSecurity: { type: Boolean, default: false },

    // OPC-UA, MQTT-SPARKPLUG-B, PLC4X, OPC-DA
    endpointURLs: { type: [String], default: [] },

    // OPC-UA, OPC-UA_SERVER, OPC-DA, ICCP, ICCP_SERVER
    timeoutMs: { type: Double, min: 0, default: 10000 },

    // OPC-UA
    configFileName: {
      type: String,
      default: '../conf/Opc.Ua.DefaultClient.Config.xml',
    },

    // IEC60870-5-104, IEC60870-5-104_SERVER, DNP3, PLCTag, I104M
    localLinkAddress: { type: Double, min: 0, default: 1.0 },
    remoteLinkAddress: { type: Double, min: 0, default: 1.0 },

    // IEC60870-5-104, IEC60870-5-104_SERVER, DNP3, PLCTag, I104M, IEC61850, PLC4X, OPC-DA, ICCP
    giInterval: { type: Double, min: 0, default: 300.0 },

    // OPC-DA, OPC-DA_SERVER
    deadBand: { type: Double, min: 0, default: 0.0 },

    // OPC-DA, OPC-DA_SERVER, ICCP, ICCP_SERVER
    hoursShift: { type: Double, min: 0, default: 0.0 },

    // IEC60870-5-101, IEC60870-5-101_SERVER, IEC60870-5-104, IEC60870-5-104_SERVER
    testCommandInterval: { type: Double, min: 0, default: 0.0 },
    timeSyncInterval: { type: Double, min: 0, default: 0.0 },
    sizeOfCOT: { type: Double, min: 1, default: 2.0 },
    sizeOfCA: { type: Double, min: 1, default: 2.0 },
    sizeOfIOA: { type: Double, min: 1, default: 3.0 },

    // IEC60870-5-104, IEC60870-5-104_SERVER
    k: { type: Double, min: 1, default: 12.0 },
    w: { type: Double, min: 1, default: 8.0 },
    t0: { type: Double, min: 1, default: 10.0 },
    t1: { type: Double, min: 1, default: 15.0 },
    t2: { type: Double, min: 1, default: 10.0 },
    t3: { type: Double, min: 1, default: 20.0 },

    // IEC60870-5-104_SERVER, IEC61850_SERVER
    serverModeMultiActive: { type: Boolean, default: true },
    maxClientConnections: { type: Double, min: 1, default: 1.0 },

    // IEC60870-5-104_SERVER,IEC60870-5-101_SERVER, IEC61850_SERVER
    maxQueueSize: { type: Double, min: 0, default: 5000.0 },

    // IEC60870-5-104, IEC60870-5-104_SERVER, DNP3, MQTT-SPARKPLUG-B, OPC-UA_SERVER, IEC61850, IEC61850_SERVER, OPC-DA, ICCP_SERVER, ICCP
    localCertFilePath: { type: String, default: '' },

    // IEC60870-5-104, IEC60870-5-104_SERVER, DNP3, IEC61850, OPC-DA
    peerCertFilePath: { type: String, default: '' },

    // IEC60870-5-104, IEC60870-5-104_SERVER, IEC61850, IEC61850_SERVER, ICCP_SERVER, ICCP
    peerCertFilesPaths: { type: [String], default: [] },

    // IEC60870-5-104, IEC60870-5-104_SERVER, MQTT-SPARKPLUG-B, IEC61850, IEC61850_SERVER, ICCP_SERVER, ICCP
    rootCertFilePath: { type: String, default: '' },
    chainValidation: { type: Boolean, default: false },

    // IEC60870-5-104, IEC60870-5-104_SERVER, IEC61850, IEC61850_SERVER, ICCP_SERVER, ICCP
    allowOnlySpecificCertificates: { type: Boolean, default: false },

    // DNP3, MQTT-SPARKPLUG-B, OPC-UA_SERVER, IEC61850, IEC61850_SERVER, ICCP_SERVER, ICCP
    privateKeyFilePath: { type: String, default: '' },
    allowTLSv10: { type: Boolean, default: false },
    allowTLSv11: { type: Boolean, default: false },
    allowTLSv12: { type: Boolean, default: true },
    allowTLSv13: { type: Boolean, default: true },
    cipherList: { type: String, default: '' },

    // DNP3
    connectionMode: { type: String, default: 'TCP Active' },
    asyncOpenDelay: { type: Double, min: 0.0, default: 0.0 },
    timeSyncMode: { type: Double, min: 0.0, max: 2.0, default: 0.0 },
    class0ScanInterval: { type: Double, min: 0.0, default: 0.0 },
    class1ScanInterval: { type: Double, min: 0.0, default: 0.0 },
    class2ScanInterval: { type: Double, min: 0.0, default: 0.0 },
    class3ScanInterval: { type: Double, min: 0.0, default: 0.0 },
    enableUnsolicited: { type: Boolean, default: true },
    rangeScans: { type: Array, default: [] },

    // IEC60870-5-101, IEC60870-5-101_SERVER, DNP3
    portName: { type: String, default: '' },
    baudRate: { type: Double, min: 150, default: 9600.0 },
    parity: { type: String, default: 'Even' },
    stopBits: { type: String, default: 'One' },
    handshake: { type: String, default: 'None' },

    // IEC60870-5-101, IEC60870-5-101_SERVER
    timeoutForACK: { type: Double, min: 1, default: 1000 },
    timeoutRepeat: { type: Double, min: 1, default: 1000 },
    useSingleCharACK: { type: Boolean, default: true },
    sizeOfLinkAddress: { type: Double, min: 0, default: 1 },
  }),
  'protocolConnections'
)

module.exports = ProtocolConnection
