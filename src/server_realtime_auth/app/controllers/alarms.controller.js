/*
 * Alarm list (alarmsData) for the Alarms Viewer: realtime and historical
 * queries (app/services/alarm-list.js). Any signed-in user; the stations the
 * user is restricted to (rights.group1List) are applied by the database.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 */

'use strict'

const Log = require('../../simple-logger')
const db = require('../models')
const AlarmList = require('../services/alarm-list')

function nativeDb() {
  const conn = db.mongoose.connection
  return conn.readyState === 1 ? conn.db : null
}

exports.listAlarms = async (req, res) => {
  try {
    const mdb = nativeDb()
    if (!mdb) {
      res.status(200).send({ error: 'Database unavailable', unavailable: true })
      return
    }
    const r = await AlarmList.listAlarms(
      mdb,
      req.body || {},
      req.rights?.group1List || [],
      { existingPointsOnly: true }
    )
    if (r.error) {
      res.status(200).send({ error: r.error })
      return
    }
    res.status(200).send({ error: false, ...r })
  } catch (err) {
    Log.log('listAlarms error: ' + err.message)
    res.status(200).send({ error: 'Could not read the alarms' })
  }
}
