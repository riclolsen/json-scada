const mongoose = require('mongoose')
const Double = require('@mongoosejs/double')

const ProtocolConnection = mongoose.model(
  'ProtocolConnection',
  new mongoose.Schema({
    protocolDriver: { type: String, required: true, default: 'UNDEFINED' },
    protocolDriverInstanceNumber: {
      type: Double,
      required: true,
      default: 1.0
    },
    protocolConnectionNumber: {
      type: Double,
      required: true,
      unique: true,
      min: 1,
      default: 1.0
    },
    name: { type: String, required: true, default: 'NEW_CONNECTION' },
    description: { type: String, required: true, default: 'NEW CONNECTION' },
    enabled: { type: Boolean, required: true, default: true },
    commandsEnabled: { type: Boolean, required: true, default: true },
    stats: { type: Object, default: null },

    // IEC 104 Server and Client, DNP3, PLCTag, I104M
    ipAddressLocalBind: { type: String },
    ipAddresses: { type: [String], default: [] },
    
    // OPC-UA Client
    endpointURLs: { type: [String], default: [] },
    configFileName: { type: String, default: "../conf/Opc.Ua.DefaultClient.Config.xml" },
    useSecurity: { type: Boolean, default: false },
    autoCreateTags: { type: Boolean, default: true },
    autoCreateTagPublishingInterval: { type: Double, min: 0, default: 2.5 },
    autoCreateTagSamplingInterval: { type: Double, min: 0, default: 0.0 },
    autoCreateTagQueueSize: { type: Double, min: 0, default: 5.0 },
    timeoutMs: { type: Double, min: 0, default: 20000 },

    // IEC 104 Server and Client, DNP3, PLCTag, I104M
    localLinkAddress: { type: Double, min: 0, default: 1.0 },
    remoteLinkAddress: { type: Double, min: 0, default: 1.0 },
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

    // IEC 104 Server
    serverModeMultiActive: { type: Boolean, default: true },
    maxClientConnections: { type: Double, min: 1, default: 1.0 },

    // IEC 101/104 Server 
    maxQueueSize: { type: Double, min: 0, default: 5000.0 },

    // IEC 104 Client, DNP3 Client
    localCertFilePath: { type: String, default: '' },
    peerCertFilePath: { type: String, default: '' }, 

    // IEC 104 Client
    rootCertFilePath: { type: String, default: '' }, 
    allowOnlySpecificCertificates: { type: Boolean, default: false },
    chainValidation: { type: Boolean, default: false }, 

    // DNP3 Client
    allowTLSv10: { type: Boolean, default: false }, 
    allowTLSv11: { type: Boolean, default: false }, 
    allowTLSv12: { type: Boolean, default: true }, 
    allowTLSv13: { type: Boolean, default: true }, 
    cipherList: { type: String, default: '' },  
    privateKeyFilePath: { type: String, default: '' }, 
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
    sizeOfLinkAddress : { type: Double, min: 0, default: 1 },

  }),
  'protocolConnections'
)

module.exports = ProtocolConnection
