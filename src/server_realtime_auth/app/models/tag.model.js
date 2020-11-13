const mongoose = require("mongoose");

const Tag = mongoose.model(
  "Tag",  
  new mongoose.Schema({
    _id: {type: Number},
    tag: {type: String},
    group1: {type: String},
    value: {type: Number}
  }),
  "realtimeData"
);

module.exports = Tag;