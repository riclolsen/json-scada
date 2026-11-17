/*
 * The PostgreSQL (historian) connection pool shared by the /Invoke/ API and
 * the GraphQL server.
 * {json:scada} - Copyright (c) 2020-2025 - Ricardo L. Olsen
 *
 * Libpq environment variables (PGHOST, PGUSER, PGPASSWORD...) are used when
 * PGHOST or PGHOSTADDR is set; otherwise the local default database.
 */

'use strict'

const { Pool } = require('pg')
const Log = require('./simple-logger')

let pool = null

const msFromEnv = (name, dflt) => {
  const v = parseInt(process.env[name], 10)
  return Number.isFinite(v) && v > 0 ? v : dflt
}
const CONNECT_TIMEOUT_MS = msFromEnv('JS_PG_CONNECT_TIMEOUT_MS', 5000)
const STATEMENT_TIMEOUT_MS = msFromEnv('JS_PG_STATEMENT_TIMEOUT_MS', 30000)

function getPgPool() {
  if (pool) return pool
  // fields not given here come from the libpq environment variables
  const pgopt = {
    ...('PGHOST' in process.env || 'PGHOSTADDR' in process.env
      ? {}
      : {
          host: '127.0.0.1',
          database: 'json_scada',
          user: 'json_scada',
          password: 'json_scada',
          port: 5432,
        }),
    // an unreachable server or a slow query must not hold requests forever
    connectionTimeoutMillis: CONNECT_TIMEOUT_MS,
    statement_timeout: STATEMENT_TIMEOUT_MS,
    query_timeout: STATEMENT_TIMEOUT_MS + 5000,
  }
  Log.log('Postgresql - Trying to connect')
  pool = new Pool(pgopt)
  pool.on('connect', () => Log.log('Postgresql - connection open'))
  // errors of idle clients: the pool discards them and connects again on the
  // next query, so the pool itself is kept (it also covers a server that was
  // not up yet at startup)
  pool.on('error', (err) => Log.log('Postgresql - ' + (err?.message || err)))
  return pool
}

module.exports = { getPgPool }
