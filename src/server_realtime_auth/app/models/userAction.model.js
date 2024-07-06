const mongoose = require('mongoose')
const Double = require('./double')

const UserAction = mongoose.model(
  'UserAction',
  new mongoose.Schema({
    username: { type: String },
    pointKey: { type: Double },
    tag: { type: String },
    properties: { type: Object },
    action: { type: String },
    timeTag: { type: Date },
  }),
  'userActions'
)

module.exports = UserAction
