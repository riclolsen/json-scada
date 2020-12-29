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

    ipAddressLocalBind: { type: String },
    ipAddresses: { type: [String], default: [] },

    localLinkAddress: { type: Double, default: 1.0 },
    remoteLinkAddress: { type: Double, default: 1.0 },
    giInterval: { type: Double, min: 0, default: 300.0 },
    testCommandInterval: { type: Double, min: 0, default: 0.0 },
    timeSyncInterval: { type: Double, min: 0, default: 0.0 },
    sizeOfCOT: { type: Double, min: 1, default: 2.0 },
    sizeOfCA: { type: Double, min: 1, default: 2.0 },
    sizeOfIOA: { type: Double, min: 1, default: 3.0 },
    k: { type: Double, min: 1, default: 12.0 },
    w: { type: Double, min: 1, default: 8.0 },
    t0: { type: Double, min: 1, default: 10.0 },
    t1: { type: Double, min: 1, default: 15.0 },
    t2: { type: Double, min: 1, default: 10.0 },
    t3: { type: Double, min: 1, default: 20.0 },
    serverModeMultiActive: { type: Boolean, default: true },
    maxClientConnections: { type: Double, min: 1, default: 1.0 },
    maxQueueSize: { type: Double, min: 0, default: 5000.0 },

    portName: { type: String },
    baudRate: { type: Double, min: 150, default: 9600.0 },
    parity: { type: String, default: 'Even' },
    stopBits: { type: String, default: 'One' },
    handshake: { type: String, default: 'None' },
    timeoutForACK: { type: Double, min: 1, default: 1000 },
    timeoutRepeat: { type: Double, min: 1, default: 1000 },
    useSingleCharACK: { type: Boolean, default: true },
    sizeOfLinkAddress : { type: Double, min: 0, default: 1 },

  }),
  'protocolConnections'
)

module.exports = ProtocolConnection
