const mongoose = require('mongoose')
const Double = require('./double')

const Role = mongoose.model(
  'Role',
  new mongoose.Schema({
    name: { type: String, default: 'new role' },
    isAdmin: { type: Boolean, default: false },
    changePassword: { type: Boolean, default: false },
    sendCommands: { type: Boolean, default: false },
    enterAnnotations: { type: Boolean, default: false },
    enterNotes: { type: Boolean, default: false },
    enterManuals: { type: Boolean, default: false },
    enterLimits: { type: Boolean, default: false },
    substituteValues: { type: Boolean, default: false },
    ackEvents: { type: Boolean, default: false },
    ackAlarms: { type: Boolean, default: false },
    disableAlarms: { type: Boolean, default: false },
    group1List: { type: [String], default: [] },
    group1CommandList: { type: [String], default: [] },
    displayList: { type: [String], default: [] },
    maxSessionDays: { type: Double, default: 3.0 },
  }),
  'roles'
)

module.exports = Role
