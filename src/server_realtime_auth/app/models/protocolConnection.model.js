const mongoose = require("mongoose");

const ProtocolConnection = mongoose.model(
  "ProtocolConnection",
  new mongoose.Schema({
    protocolDriver: {type: String, required: true, default: "UNDEFINED"},
    protocolDriverInstanceNumber: {type: Number, required: true, default: 1.0},
    protocolConnectionNumber: {type: Number, required: true, unique: true, min: 1, default: 1.0},
    name: {type: String, required: true, default: "NEW_CONNECTION"},
    description: {type: String, required: true, default: "NEW CONNECTION"},
    enabled: {type: Boolean, required: true, default: true},
    commandsEnabled: {type: Boolean, required: true, default: true},
    
    ipAddressLocalBind: {type: String},
    ipAddresses: {type: [String], default: []},

    localLinkAddress: {type: Number, default: 1.0},
    remoteLinkAddress: {type: Number, default: 1.0},
    giInterval: {type: Number, min:0, default: 300.0},
    testCommandInterval: {type: Number, min:0, default: 0.0},
    timeSyncInterval: {type: Number, min:0, default: 0.0},
    sizeOfCOT: {type: Number, min:1, default: 2.0},
    sizeOfCA: {type: Number, min:1, default: 2.0},
    sizeOfIOA: {type: Number, min:1, default: 3.0},
    k: {type: Number, min:1, default: 12.0},
    w: {type: Number, min:1, default: 8.0},
    t0: {type: Number, min:1, default: 10.0},
    t1: {type: Number, min:1, default: 15.0},
    t2: {type: Number, min:1, default: 10.0},
    t3: {type: Number, min:1, default: 20.0},
//    serverModeMultiActive: {type: Boolean, default: true},
//    maxClientConnections: {type: Number, min:1, default: 1.0},
//    maxQueueSize: {type: Number, min:0, default: 5000.0},

  }),
  "protocolConnections"
);

module.exports = ProtocolConnection;