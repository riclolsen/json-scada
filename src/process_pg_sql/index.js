/*
 * Process PG SQL - loads the SQL files queued by cs_data_processor into PostgreSQL.
 *
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

'use strict'

// Usage: node index.js [hist|rtdata|all] [logLevel]
// Connection: libpq-style PG* environment variables, a pgpass file, or
// JS_PGSQL_CONNECTION_STRING. See README.md.

const fs = require('fs')
const os = require('os')
const path = require('path')
const { Client } = require('pg')

const AppName = 'process_pg_sql'
const AppVersion = '1.0.0'
const ENV_PREFIX = 'JS_PGSQL_'

const Queues = {
  hist: { prefix: 'pg_hist_' }, // hist table inserts (one transaction per file)
  rtdata: { prefix: 'pg_rtdata_' }, // realtime_data upserts (file order matters)
}

const Log = {
  levelMin: 0,
  levelNormal: 1,
  levelDetailed: 2,
  levelDebug: 3,
  levelCurrent: 1,
  log: function (msg, level = 1) {
    if (level <= this.levelCurrent)
      console.log(new Date().toISOString() + ' - ' + msg)
  },
}

function envInt(name, def) {
  const v = parseInt(process.env[ENV_PREFIX + name])
  return Number.isFinite(v) && v >= 0 ? v : def
}

function envBool(name) {
  return /^(1|true|yes|on)$/i.test(process.env[ENV_PREFIX + name] || '')
}

const queueArg = (process.argv[2] || process.env[ENV_PREFIX + 'QUEUE'] || 'all')
  .trim()
  .toLowerCase()
const queueNames = queueArg === 'all' ? Object.keys(Queues) : [queueArg]
if (!queueNames.every((q) => q in Queues)) {
  console.log(
    'Invalid queue "' +
      queueArg +
      '". Usage: node index.js [hist|rtdata|all] [logLevel]'
  )
  process.exit(1)
}

{
  const lvl = parseInt(process.argv[3] ?? process.env[ENV_PREFIX + 'LOGLEVEL'])
  if (Number.isFinite(lvl)) Log.levelCurrent = lvl
}

const Config = {
  sqlDir: findSqlDir(),
  pollIntervalMs: envInt('POLL_INTERVAL_MS', 1000), // idle wait between directory scans
  retryIntervalMs: envInt('RETRY_INTERVAL_MS', 5000), // wait after a database failure
  minFileAgeMs: envInt('MIN_FILE_AGE_MS', 500), // skip files that may still be being written
  statsIntervalMs: envInt('STATS_INTERVAL_MS', 60000),
  keepFailed: envBool('KEEP_FAILED'), // rename rejected files to *.sql.failed instead of deleting
}

// Folder where cs_data_processor writes the SQL files (same search order as cs_data_processor-go).
function findSqlDir() {
  if (process.env[ENV_PREFIX + 'DIR'])
    return path.resolve(process.env[ENV_PREFIX + 'DIR'])
  const candidates = [
    path.join(__dirname, '..', '..', 'sql'),
    path.join(process.cwd(), '..', '..', 'sql'),
    path.join(process.cwd(), '..', 'sql'),
    path.join(process.cwd(), 'sql'),
    'c:\\json-scada\\sql',
    '/sql',
  ]
  for (const c of candidates) {
    try {
      if (fs.statSync(c).isDirectory()) return path.resolve(c)
    } catch {}
  }
  return path.resolve(candidates[0])
}

// Connection options. Defaults match the former psql scripts (127.0.0.1:5432, json_scada/json_scada).
function connectionConfig(queueName) {
  const application_name = AppName + ' ' + queueName
  const common = {
    application_name,
    connectionTimeoutMillis: envInt('CONNECT_TIMEOUT_MS', 10000),
    keepAlive: true,
  }
  const connStr = process.env[ENV_PREFIX + 'CONNECTION_STRING']
  if (connStr) return { ...common, connectionString: connStr }
  const host = process.env.PGHOST || process.env.PGHOSTADDR || '127.0.0.1'
  const port = parseInt(process.env.PGPORT) || 5432
  const database = process.env.PGDATABASE || 'json_scada'
  const user = process.env.PGUSER || 'json_scada'
  return {
    ...common,
    host,
    port,
    database,
    user,
    // only called when the server asks for a password
    password: async () =>
      process.env.PGPASSWORD ?? pgpassLookup(host, port, database, user) ?? '',
  }
}

// Minimal libpq password file lookup (PGPASSFILE, ~/.pgpass or %APPDATA%\postgresql\pgpass.conf).
function pgpassLookup(host, port, database, user) {
  const file =
    process.env.PGPASSFILE ||
    (process.platform === 'win32' ?
      path.join(process.env.APPDATA || '', 'postgresql', 'pgpass.conf')
    : path.join(os.homedir(), '.pgpass'))
  let text
  try {
    text = fs.readFileSync(file, 'utf8')
  } catch {
    return undefined
  }
  const want = [host, String(port), database, user]
  for (const line of text.split(/\r?\n/)) {
    if (!line || line.startsWith('#')) continue
    const fields = []
    let cur = ''
    for (let i = 0; i < line.length; i++) {
      const ch = line[i]
      if (ch === '\\' && i + 1 < line.length) cur += line[++i]
      else if (ch === ':' && fields.length < 4) {
        fields.push(cur)
        cur = ''
      } else cur += ch
    }
    fields.push(cur)
    if (fields.length < 5) continue
    if (want.every((v, i) => fields[i] === '*' || fields[i] === v))
      return fields[4]
  }
  return undefined
}

// Errors that mean "try the same file again later": the file stays queued.
// Anything else is a problem with the file content (syntax, data) and the file
// is discarded, as psql did, so it does not block the queue forever.
function isRetryable(err) {
  const code = typeof err?.code === 'string' ? err.code : ''
  if (!/^[0-9A-Z]{5}$/.test(code)) return true // network/socket/timeout, no SQLSTATE
  return (
    code.startsWith('08') || // connection exception
    code.startsWith('28') || // invalid authorization
    code.startsWith('53') || // insufficient resources (disk full, too many connections)
    code.startsWith('57') || // operator intervention (shutdown, cancel, statement timeout)
    code.startsWith('58') || // system error (I/O)
    code.startsWith('XX') || // internal error
    [
      '3D000', // database does not exist (yet)
      '42P01', // table does not exist (yet)
      '42501', // insufficient privilege (grants not applied yet)
      '40001', // serialization failure
      '40P01', // deadlock
      '55P03', // lock not available
      '25006', // read-only transaction (standby during failover)
    ].includes(code)
  )
}

let shuttingDown = false
const sleepers = new Set()

// Waits ms milliseconds, or less when a shutdown is requested.
function sleep(ms) {
  if (shuttingDown) return Promise.resolve()
  return new Promise((resolve) => {
    const t = setTimeout(done, ms)
    function done() {
      clearTimeout(t)
      sleepers.delete(done)
      resolve()
    }
    sleepers.add(done)
  })
}

class QueueWorker {
  constructor(name) {
    this.name = name
    this.prefix = Queues[name].prefix
    this.client = null
    this.connectFailures = 0
    this.retryFailures = 0
    this.loaded = new Set() // committed files that could not be deleted yet (never re-executed)
    this.stats = { files: 0, failed: 0, bytes: 0, ms: 0 }
    this.lastStats = Date.now()
    this.pending = 0
  }

  log(msg, level) {
    Log.log('[' + this.name + '] ' + msg, level)
  }

  // Pending files in creation order: <prefix><epoch ms>[-<seq>]_<instance>.sql
  listFiles() {
    const files = []
    for (const name of fs.readdirSync(Config.sqlDir)) {
      if (!name.startsWith(this.prefix) || !name.endsWith('.sql')) continue
      const m = /^(\d+)(?:-(\d+))?_/.exec(name.substring(this.prefix.length))
      files.push({
        name,
        ms: m ? Number(m[1]) : Number.MAX_SAFE_INTEGER,
        seq: m && m[2] ? Number(m[2]) : 0,
      })
    }
    files.sort(
      (a, b) =>
        a.ms - b.ms ||
        a.seq - b.seq ||
        (a.name < b.name ? -1
        : a.name > b.name ? 1
        : 0)
    )
    return files.map((f) => f.name)
  }

  async connect() {
    if (this.client) return this.client
    const client = new Client(connectionConfig(this.name))
    client.on('error', (err) => {
      this.log('PostgreSQL connection error: ' + err.message)
      if (this.client === client) this.client = null
      client.end().catch(() => {})
    })
    try {
      await client.connect()
    } catch (err) {
      client.end().catch(() => {})
      throw err
    }
    this.client = client
    if (this.connectFailures > 0 || Log.levelCurrent >= Log.levelDetailed)
      this.log(
        'Connected to PostgreSQL ' +
          (client.host || '') +
          ':' +
          (client.port || '') +
          '/' +
          (client.database || '')
      )
    this.connectFailures = 0
    return client
  }

  async dropConnection() {
    const client = this.client
    this.client = null
    if (client) await client.end().catch(() => {})
  }

  removeLoaded(file, full) {
    try {
      fs.unlinkSync(full)
      this.loaded.delete(file)
    } catch (err) {
      if (err.code === 'ENOENT') this.loaded.delete(file)
      else {
        if (!this.loaded.has(file))
          this.log(
            'Loaded ' +
              file +
              ' but could not delete it (' +
              err.code +
              '), will retry.'
          )
        this.loaded.add(file)
      }
    }
  }

  discardFailed(file, full) {
    try {
      if (Config.keepFailed) fs.renameSync(full, full + '.failed')
      else fs.unlinkSync(full)
    } catch (err) {
      if (err.code !== 'ENOENT')
        this.log('Could not discard ' + file + ': ' + err.message)
    }
  }

  // Processes the queue once. Returns true when at least one file was handled.
  async cycle() {
    let files
    try {
      files = this.listFiles()
    } catch (err) {
      this.log('Cannot read SQL folder ' + Config.sqlDir + ': ' + err.message)
      await sleep(Config.retryIntervalMs)
      return false
    }
    this.pending = files.length
    let didWork = false

    for (const file of files) {
      if (shuttingDown) break
      const full = path.join(Config.sqlDir, file)

      if (this.loaded.has(file)) {
        this.removeLoaded(file, full)
        this.pending--
        continue
      }

      let st, sql
      try {
        st = fs.statSync(full)
        // a writer that is not atomic may still be filling the file; the
        // remaining files are newer, so stop here to keep the order
        // (a modification time in the future, after a clock step, does not block)
        const age = Date.now() - st.mtimeMs
        if (age >= 0 && age < Config.minFileAgeMs) break
        sql = fs.readFileSync(full, 'utf8')
      } catch (err) {
        if (err.code === 'ENOENT') {
          this.pending--
          continue
        }
        this.log('Cannot read ' + file + ': ' + err.message)
        break
      }

      if (sql.trim() === '') {
        this.log('Empty file discarded: ' + file, Log.levelDetailed)
        this.discardFailed(file, full)
        this.pending--
        continue
      }

      let client
      try {
        client = await this.connect()
      } catch (err) {
        this.connectFailures++
        const errKey = (err.code || '') + ' ' + err.message
        if (
          this.connectFailures === 1 ||
          this.connectFailures % 60 === 0 ||
          errKey !== this.lastConnectError
        )
          this.log(
            'Cannot connect to PostgreSQL (' +
              errKey +
              '), ' +
              files.length +
              ' file(s) queued, retrying.'
          )
        this.lastConnectError = errKey
        await sleep(Config.retryIntervalMs)
        return didWork
      }
      this.lastConnectError = ''

      const t0 = Date.now()
      try {
        await client.query(sql)
      } catch (err) {
        // leave no aborted transaction behind (files open their own transaction)
        await client.query('ROLLBACK').catch(() => {})
        if (isRetryable(err)) {
          this.retryFailures++
          if (this.retryFailures === 1 || this.retryFailures % 60 === 0)
            this.log(
              'Error loading ' +
                file +
                ' (' +
                (err.code || '') +
                ' ' +
                err.message +
                '), ' +
                this.pending +
                ' file(s) queued, retrying.'
            )
          await this.dropConnection() // reconnect on the next attempt
          await sleep(Config.retryIntervalMs)
          return didWork
        }
        this.stats.failed++
        this.log(
          'Rejected ' +
            file +
            ' (' +
            err.code +
            ' ' +
            err.message +
            ')' +
            (Config.keepFailed ? ', renamed to .failed' : ', discarded')
        )
        this.discardFailed(file, full)
        this.pending--
        didWork = true
        continue
      }

      if (this.retryFailures > 0) {
        this.log(
          'Loading resumed after ' + this.retryFailures + ' failed attempt(s).'
        )
        this.retryFailures = 0
      }
      const elapsed = Date.now() - t0
      this.stats.files++
      this.stats.bytes += st.size
      this.stats.ms += elapsed
      this.log(
        'Loaded ' + file + ' (' + st.size + ' bytes, ' + elapsed + ' ms)',
        Log.levelDebug
      )
      this.removeLoaded(file, full)
      this.pending--
      didWork = true
    }
    return didWork
  }

  reportStats(force) {
    const now = Date.now()
    if (!force && now - this.lastStats < Config.statsIntervalMs) return
    const s = this.stats
    if (s.files || s.failed || this.pending || force)
      this.log(
        'Loaded ' +
          s.files +
          ' file(s), ' +
          Math.round(s.bytes / 1024) +
          ' KiB in ' +
          s.ms +
          ' ms, rejected ' +
          s.failed +
          ', pending ' +
          Math.max(this.pending, 0) +
          ' (last ' +
          Math.round((now - this.lastStats) / 1000) +
          ' s)',
        Log.levelNormal
      )
    this.stats = { files: 0, failed: 0, bytes: 0, ms: 0 }
    this.lastStats = now
  }

  async run() {
    this.log('Watching ' + path.join(Config.sqlDir, this.prefix + '*.sql'))
    while (!shuttingDown) {
      let didWork = false
      try {
        didWork = await this.cycle()
      } catch (err) {
        this.log('Unexpected error: ' + (err.stack || err))
        await this.dropConnection()
        await sleep(Config.retryIntervalMs)
      }
      this.reportStats(false)
      if (!didWork && !shuttingDown) await sleep(Config.pollIntervalMs)
    }
    this.reportStats(true)
    await this.dropConnection()
  }
}

async function main() {
  Log.log(
    AppName + ' Version ' + AppVersion + ' - queues: ' + queueNames.join(', ')
  )
  Log.log('SQL folder: ' + Config.sqlDir)
  const workers = queueNames.map((q) => new QueueWorker(q))

  const stop = (sig) => {
    if (shuttingDown) return
    Log.log('Received ' + sig + ', finishing current file and exiting.')
    shuttingDown = true
    for (const wake of [...sleepers]) wake()
  }
  process.on('SIGINT', () => stop('SIGINT'))
  process.on('SIGTERM', () => stop('SIGTERM'))

  await Promise.all(workers.map((w) => w.run()))
  Log.log('Exiting.')
  process.exit(0)
}

main().catch((err) => {
  Log.log('Fatal: ' + (err.stack || err))
  process.exit(1)
})
