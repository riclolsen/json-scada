/*
 * Security events (secEvents) for the Security Events viewer
 * (app/services/sec-events.js). Needs a role with viewSecEvents.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 */

'use strict'

const Log = require('../../simple-logger')
const db = require('../models')
const SecEvents = require('../services/sec-events')

function nativeDb() {
  const conn = db.mongoose.connection
  return conn.readyState === 1 ? conn.db : null
}

exports.listSecEvents = async (req, res) => {
  try {
    const mdb = nativeDb()
    if (!mdb) {
      res.status(200).send({ error: 'Database unavailable', unavailable: true })
      return
    }
    const r = await SecEvents.listSecEvents(mdb, req.body || {})
    if (r.error) {
      res.status(200).send({ error: r.error })
      return
    }
    res.status(200).send({ error: false, ...r })
  } catch (err) {
    Log.log('listSecEvents error: ' + err.message)
    res.status(200).send({ error: 'Could not read the security events' })
  }
}
