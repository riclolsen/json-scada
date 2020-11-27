const mongoose = require("mongoose");

const ProtocolDriverInstance = mongoose.model(
  "ProtocolDriverInstance",
  new mongoose.Schema({
    protocolDriver: {type: String, default: "UNDEFINED"},
    protocolDriverInstanceNumber: {type: Number, default: 1.0},
    enabled: {type: Boolean, default: true},
    logLevel: {type: Number, default: 1.0},
    nodeNames: {type: [String], default: ["mainNode"]},
    keepProtocolRunningWhileInactive: {type: Boolean, default: false},
    //activeNodeName: {type: String, default: ""},
    //activeNodeKeepAliveTimeTag: {type: Date, default: null},
  }),
  "protocolDriverInstances"
);

module.exports = ProtocolDriverInstance;