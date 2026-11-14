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

    // IEC60870-5-104_SERVER, I104M, TELEGRAF_LISTENER, OPC-UA_SERVER, IEC61850_SERVER, ICCP_SERVER, DNP3_SERVER, ONVIF, MODBUS_SERVER
    ipAddressLocalBind: { type: String, default: '' },

    // IEC60870-5-104, IEC60870-5-104_SERVER, DNP3, DNP3_SERVER, PLCTag, I104M, TELEGRAF_LISTENER, OPC-UA_SERVER, IEC61850, IEC61850_SERVER, ICCP, ICCP_SERVER, MODBUS, MODBUS_SERVER
    ipAddresses: { type: [String], default: [] },

    // MQTT-SPARKPLUG-B, OPC-UA, OPC-UA_SERVER, IEC61850, PLC4X, OPC_DA, OPC-DA_SERVER, ICCP, ICCP_SERVER, DNP3, DNP3_SERVER, IEC60870-5-104, IEC60870-5-104_SERVER, IEC60870-5-101, IEC60870-5-101_SERVER, MODBUS
    topics: { type: [String], default: [] },

    // ICCP, ICCP_SERVER
    localAeQualifier: { type: Double, default: 12 },
    localApTitle: { type: String, default: '1.1.998.1' },

    // ICCP, ICCP_SERVER
    remoteAeQualifier: { type: Double, default: 12 },
    remoteApTitle: { type: String, default: '1.1.999.2' },

    // MQTT-SPARKPLUG-B, OPC-UA_SERVER
    groupId: { type: String, default: '' },

    // MQTT-SPARKPLUG-B, IEC60870-5-104, IEC60870-5-104_SERVER, OPC-UA
    passphrase: { type: String, default: '' },

    // MQTT-SPARKPLUG-B
    topicsAsFiles: { type: [String], default: [] },
    topicsScripted: { type: [Object], default: [] },
    clientId: { type: String, default: '' },
    edgeNodeId: { type: String, default: '' },
    deviceId: { type: String, default: '' },
    scadaHostId: { type: String, default: '' },
    publishTopicRoot: { type: String, default: '' },

    // MQTT-SPARKPLUG-B, OPC-UA
    pfxFilePath: { type: String, default: '' },

    // MQTT-SPARKPLUG-B, IEC61850, IEC61850_SERVER, OPC-DA, OPC-UA, ONVIF
    username: { type: String, default: '' },
    // MQTT-SPARKPLUG-B, IEC61850, IEC61850_SERVER, OPC-DA, OPC-UA, ONVIF, ICCP, ICCP_SERVER
    password: { type: String, default: '' },

    // OPC-UA, TELEGRAF_LISTENER, MQTT-SPARKPLUG-B, IEC61850, PLC4X, OPC-DA, ICCP, DNP3_SERVER, DNP3, IEC61850_SERVER, ICCP, IEC60870-5-104_SERVER, IEC60870-5-104, IEC60870-5-101_SERVER, IEC60870-5-101, MODBUS
    autoCreateTags: { type: Boolean, default: true },

    // OPC-UA, OPC-DA, OPC-DA_SERVER, ICCP
    autoCreateTagPublishingInterval: { type: Double, min: 0, default: 5.0 },

    // OPC-UA, OPC-DA
    autoCreateTagSamplingInterval: { type: Double, min: 0, default: 5.0 },
    autoCreateTagQueueSize: { type: Double, min: 0, default: 5.0 },

    // OPC-UA, MQTT-SPARKPLUG-B, OPC-UA_SERVER, IEC61850, IEC61850_SERVER, OPC-DA, ICCP, ICCP_SERVER
    useSecurity: { type: Boolean, default: false },

    // OPC-UA, MQTT-SPARKPLUG-B, PLC4X, OPC-DA, ONVIF
    endpointURLs: { type: [String], default: [] },

    // OPC-UA, OPC-UA_SERVER, OPC-DA, ICCP, ICCP_SERVER, ONVIF, MODBUS
    timeoutMs: { type: Double, min: 0, default: 10000 },

    // OPC-UA
    configFileName: {
      type: String,
      default: '',
    },

    // IEC60870-5-104, IEC60870-5-104_SERVER, DNP3, DNP3_SERVER, PLCTag, I104M
    localLinkAddress: { type: Double, min: 0, default: 1.0 },
    remoteLinkAddress: { type: Double, min: 0, default: 1.0 },

    // IEC60870-5-104, IEC60870-5-104_SERVER, DNP3, PLCTag, I104M, IEC61850, PLC4X, OPC-UA, OPC-DA, ICCP, ONVIF, MODBUS
    giInterval: { type: Double, min: 0, default: 300.0 },

    // OPC-DA, OPC-DA_SERVER
    deadBand: { type: Double, min: 0, default: 0.0 },

    // OPC-UA, OPC-DA, OPC-DA_SERVER, ICCP, ICCP_SERVER, DNP3_SERVER
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
    // IEC60870-5-104_SERVER, IEC61850_SERVER, MODBUS_SERVER
    maxClientConnections: { type: Double, min: 1, default: 1.0 },

    // IEC60870-5-104_SERVER,IEC60870-5-101_SERVER, IEC61850_SERVER
    maxQueueSize: { type: Double, min: 0, default: 5000.0 },

    // OPC-UA, OPC-DA, DNP3_SERVER
    serverQueueSize: { type: Double, min: 0, default: 5000.0 },

    // IEC60870-5-104, IEC60870-5-104_SERVER, DNP3, DNP3_SERVER, MQTT-SPARKPLUG-B, OPC-UA_SERVER, OPC-UA, IEC61850, IEC61850_SERVER, OPC-DA, ICCP_SERVER, ICCP, MODBUS, MODBUS_SERVER
    localCertFilePath: { type: String, default: '' },

    // IEC60870-5-104, IEC60870-5-104_SERVER, DNP3, DNP3_SERVER, IEC61850, OPC-DA, MODBUS, MODBUS_SERVER
    // For MODBUS/MODBUS_SERVER this is the CA / trust anchor used to verify the peer.
    peerCertFilePath: { type: String, default: '' },

    // IEC60870-5-104, IEC60870-5-104_SERVER, IEC61850, IEC61850_SERVER, ICCP_SERVER, ICCP
    peerCertFilesPaths: { type: [String], default: [] },

    // IEC60870-5-104, IEC60870-5-104_SERVER, MQTT-SPARKPLUG-B, IEC61850, IEC61850_SERVER, ICCP_SERVER, ICCP
    rootCertFilePath: { type: String, default: '' },
    // IEC60870-5-104, IEC60870-5-104_SERVER, MQTT-SPARKPLUG-B, IEC61850, IEC61850_SERVER, ICCP_SERVER, ICCP, MODBUS, MODBUS_SERVER
    chainValidation: { type: Boolean, default: false },

    // IEC60870-5-104, IEC60870-5-104_SERVER, IEC61850, IEC61850_SERVER, ICCP_SERVER, ICCP, MODBUS, MODBUS_SERVER
    allowOnlySpecificCertificates: { type: Boolean, default: false },

    // OPC-UA
    autoAcceptUntrustedCertificates: { type: Boolean, default: true },
    securityMode: { type: String, default: 'None' },
    securityPolicy: { type: String, default: 'None' },

    // DNP3, DNP3_SERVER, MQTT-SPARKPLUG-B, OPC-UA_SERVER, IEC61850, IEC61850_SERVER, ICCP_SERVER, ICCP, MODBUS, MODBUS_SERVER
    privateKeyFilePath: { type: String, default: '' },
    allowTLSv10: { type: Boolean, default: false },
    allowTLSv11: { type: Boolean, default: false },
    allowTLSv12: { type: Boolean, default: true },
    allowTLSv13: { type: Boolean, default: true },
    cipherList: { type: String, default: '' },

    // MODBUS, MODBUS_SERVER - passphrase protecting privateKeyFilePath
    privateKeyPassphrase: { type: String, default: '' },

    // DNP3, DNP3_SERVER, MODBUS, MODBUS_SERVER
    // MODBUS: TCP Active | TLS Active | Serial | RTU over TCP | RTU over TLS
    // MODBUS_SERVER: TCP Passive | TLS Passive | Serial | RTU over TCP Passive | RTU over TLS Passive
    connectionMode: { type: String, default: 'TCP Active' },
    asyncOpenDelay: { type: Double, min: 0.0, default: 0.0 },
    enableUnsolicited: { type: Boolean, default: true },

    // DNP3
    timeSyncMode: { type: Double, min: 0.0, max: 2.0, default: 0.0 },
    class0ScanInterval: { type: Double, min: 0.0, default: 0.0 },
    class1ScanInterval: { type: Double, min: 0.0, default: 0.0 },
    class2ScanInterval: { type: Double, min: 0.0, default: 0.0 },
    class3ScanInterval: { type: Double, min: 0.0, default: 0.0 },
    rangeScans: { type: Array, default: [] },

    // ONVIF
    options: { type: String, default: '' },

    // IEC60870-5-101, IEC60870-5-101_SERVER, DNP3, DNP3_SERVER, MODBUS, MODBUS_SERVER
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

    // MODBUS, MODBUS_SERVER - data representation for non-standard register layouts.
    // Each accepts a named alias (BE|LE|SW|SB) or an explicit byte permutation
    // string over the value's bytes (e.g. AB/BA, ABCD/DCBA/CDAB/BADC,
    // ABCDEFGH/HGFEDCBA/GHEFCDAB/BADCFEHG). Overridable per tag via the ASDU suffix.
    byteOrder16: { type: String, default: 'AB' },
    byteOrder32: { type: String, default: 'ABCD' },
    byteOrder64: { type: String, default: 'ABCDEFGH' },
    byteOrderStr: { type: String, default: 'AB' },
    stringEncoding: { type: String, default: 'latin1' },
    // interpret plain numeric addresses as 1-based Modicon refs (40001 -> hr:0)
    useModiconAddresses: { type: Boolean, default: false },

    // MODBUS - polling and request shaping
    pollingInterval: { type: Double, min: 0, default: 1000.0 },
    maxRetries: { type: Double, min: 0, default: 2.0 },
    interRequestDelayMs: { type: Double, min: 0, default: 0.0 },
    maxReadRegisters: { type: Double, min: 1, max: 125, default: 125.0 },
    maxReadCoils: { type: Double, min: 1, max: 2000, default: 2000.0 },
    maxAddressGap: { type: Double, min: 0, default: 8.0 },
    // use FC22 Mask Write for single-bit writes, else read-modify-write
    useMaskWrite: { type: Boolean, default: true },

    // MODBUS, MODBUS_SERVER - RTU inter-frame idle gap, 0 = auto (3.5 char times)
    interFrameDelayMs: { type: Double, min: 0, default: 0.0 },

    // MODBUS_SERVER
    clientIdleTimeoutMs: { type: Double, min: 0, default: 60000.0 },
    serverUnitIds: { type: [Double], default: [1.0] },
    strictUnitId: { type: Boolean, default: false },
    serveUnmappedAsZero: { type: Boolean, default: false },
    // Modbus has no quality bits: how to serve a tag flagged invalid
    invalidValuePolicy: { type: String, default: 'last' },
    allowWritesToSupervised: { type: Boolean, default: false },

    // OPC-DA_SERVER
    clsIdApp: { type: String, default: '' },
    clsIdServer: { type: String, default: '' },
    prgIdServer: { type: String, default: '' },
    prgIdCurrServer: { type: String, default: '' },

    // CIM-EMS (no schema defaults: set for CIM-EMS connections only, see updateProtocolConnection)
    cimModelId: { type: String },
    cimModelRevision: { type: Double },
    cimBindingRevision: { type: Double },
    cycleSeconds: { type: Double, min: 1 },
    resultMaxAgeSeconds: { type: Double, min: 1 },
    switchHoldSeconds: { type: Double, min: 0 },
    snapshotMaxSkewSeconds: { type: Double, min: 0 },
    loadFlowParameters: { type: mongoose.Schema.Types.Mixed },
    publishFilter: { type: mongoose.Schema.Types.Mixed },
    runStateEstimation: { type: Boolean },
    estimatorParameters: { type: mongoose.Schema.Types.Mixed },
    runContingency: { type: Boolean },
    contingencyListId: { type: String },
    contingencyIntervalSeconds: { type: Double, min: 0 },
    contingencyMinGapSeconds: { type: Double, min: 0 },
    contingencyMinKv: { type: Double, min: 0 },
    contingencyPublishLimit: { type: Double, min: 0 },
    contingencyThreads: { type: Double, min: 0 },
  }),
  'protocolConnections'
)

module.exports = ProtocolConnection
