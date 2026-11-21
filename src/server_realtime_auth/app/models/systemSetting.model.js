const mongoose = require('mongoose')

// Singleton collection holding runtime-changeable global toggles. A single document
// with the fixed key 'global' is used; fields are optional and default in code.
const SystemSetting = mongoose.model(
  'SystemSetting',
  new mongoose.Schema({
    key: { type: String, default: 'global', unique: true },
    autoManageServices: { type: Boolean, default: true },
    autoRestartOnConnectionChange: { type: Boolean, default: true },
    // retention (days) of soeData, alarmsData and the hist time series; the
    // data processor (instance 1) applies them once a day (defaults in code)
    soeRetentionDays: { type: Number },
    alarmsRetentionDays: { type: Number },
    histRetentionDays: { type: Number },
  }),
  'systemSettings'
)

module.exports = SystemSetting
