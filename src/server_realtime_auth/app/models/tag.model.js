const mongoose = require("mongoose");
const Double = require('@mongoosejs/double');

const Tag = mongoose.model(
  "Tag",  
  new mongoose.Schema({
    _id: {type: Number},
    tag: {type: String},
    group1: {type: String, default: ""},
    group2: {type: String, default: ""},
    group3: {type: String, default: ""},
    description: {type: String, default: ""},
    origin: {type: String, default: "supervised"},
    type: {type: String, default: "digital"},
    stateTextFalse: {type: String, default: ""},
    stateTextTrue: {type: String, default: ""},
    eventTextFalse: {type: String, default: ""},
    eventTextTrue: {type: String, default: ""},
    unit: {type: String, default: ""},
    commandOfSupervised: {type: Double, default: 0.0},
    supervisedOfCommand: {type: Double, default: 0.0},
    invalidDetectTimeout: {type: Double, default: 300.0},
    protocolSourceConnectionNumber: {type: Double, default: 0.0},
    kconv1: {type: Double, default: 1.0},
    kconv2: {type: Double, default: 0.0},
    protocolSourceASDU: {type: String, default: ""},
    protocolSourceCommonAddress: {type: String, default: ""},
    protocolSourceObjectAddress: {type: String, default: ""},
    protocolSourceCommandDuration: {type: String, default: ""},
    protocolSourceCommandUseSBO: {type: Boolean, default: false},
    isEvent: {type: Boolean, default: false},
  }),
  "realtimeData"
);

module.exports = Tag;