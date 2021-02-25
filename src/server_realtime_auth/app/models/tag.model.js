const mongoose = require("mongoose");
const Double = require('@mongoosejs/double');

const Tag = mongoose.model(
  "Tag",  
  new mongoose.Schema({
    _id: {type: Number},
    tag: {type: String},
    group1: {type: String},
    group2: {type: String},
    group3: {type: String},
    description: {type: String},
    value: {type: Double}
  }),
  "realtimeData"
);

module.exports = Tag;