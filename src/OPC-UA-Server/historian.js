/*
 * OPC-UA Server Driver for JSON-SCADA - Historical Access (OPC UA Part 11)
 *
 * Backend abstraction + per-node historian that serve HistoryRead
 * (ReadRawModifiedDetails, and ReadProcessedDetails via node-opcua-aggregates)
 * from the JSON-SCADA `hist` store (MongoDB timeseries or PostgreSQL/TimescaleDB
 * hypertable). Historization itself is performed by cs_data_processor; this
 * module never writes history (read-only).
 *
 * {json:scada} - Copyright (c) 2020-2025 - Ricardo L. Olsen
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

const {
  DataType,
  DataValue,
  Variant,
  StatusCodes,
  AccessLevelFlag,
  isMinDate,
} = require('node-opcua')

// OPC UA minDate (1601-01-01T00:00:00Z) in epoch milliseconds. node-opcua uses
// it to signal an unbounded start/end time in a HistoryRead request.
const MIN_DATE_MS = -11644473600000

// true when a start/end time means "unbounded" (null/undefined or minDate)
function isUnbounded(d) {
  if (d === null || d === undefined) return true
  try {
    if (isMinDate(d)) return true
  } catch (e) {}
  return typeof d.getTime === 'function' && d.getTime() === MIN_DATE_MS
}

// parse a positive integer config value, falling back to a default
function normInt(v, def) {
  const n = parseInt(v)
  return Number.isFinite(n) && n > 0 ? n : def
}

// only two timestamp fields are allowed; never interpolate user input into a query
function validateTsField(v) {
  return v === 'timeTagAtSource' ? 'timeTagAtSource' : 'timeTag'
}

// Convert a number to an integer within [min, max]: rounds to the nearest
// integer and saturates at the limits (node-opcua rejects out-of-range
// values). NaN becomes 0.
function toIntRange(value, min, max) {
  const n = Math.round(Number(value))
  if (!Number.isFinite(n)) return Number.isNaN(n) ? 0 : n > 0 ? max : min
  return Math.min(max, Math.max(min, n))
}

// -------------------------------------------------------------------------
// Scalar value conversion shared by the live path (convertValueVariant in
// index.js delegates here) and the historical path so the two cannot drift.
// Returns { dataType, value } for a single scalar value.
// -------------------------------------------------------------------------
function convertHistValue(type, asdu, value, valueString) {
  let dataType = ''
  let outValue = null
  switch (type) {
    case 'digital':
      dataType = DataType.Boolean
      outValue = value === 0 ? false : true
      break
    case 'string':
      switch (asdu) {
        default:
        case 'string':
          dataType = DataType.String
          break
        case 'guid':
          dataType = DataType.Guid
          break
        case 'bytestring':
          dataType = DataType.ByteString
          break
        case 'xmlelement':
          dataType = DataType.XmlElement
          break
        case 'nodeid':
          dataType = DataType.NodeId
          break
        case 'expandednodeid':
          dataType = DataType.ExpandedNodeId
          break
        case 'qualifiedname':
          dataType = DataType.QualifiedName
          break
        case 'localizedtext':
          dataType = DataType.LocalizedText
          break
      }
      outValue = valueString !== undefined ? valueString : '' + value
      break
    case 'analog':
      switch (asdu) {
        default:
        case 'double':
          dataType = DataType.Double
          outValue = parseFloat(value)
          break
        case 'datavalue':
          dataType = DataType.DataValue
          outValue = parseFloat(value)
          break
        case 'statuscode':
          dataType = DataType.StatusCode
          outValue = toIntRange(value, 0, 0xffffffff)
          break
        case 'float':
          dataType = DataType.Float
          outValue = parseFloat(value)
          break
        case 'int16':
          dataType = DataType.Int16
          outValue = toIntRange(value, -32768, 32767)
          break
        case 'uint16':
          dataType = DataType.UInt16
          outValue = toIntRange(value, 0, 65535)
          break
        case 'int32':
          dataType = DataType.Int32
          outValue = toIntRange(value, -2147483648, 2147483647)
          break
        case 'uint32':
          dataType = DataType.UInt32
          outValue = toIntRange(value, 0, 4294967295)
          break
        case 'int64':
          dataType = DataType.Int64
          outValue = toIntRange(value, -9007199254740991, 9007199254740991)
          break
        case 'uint64':
          dataType = DataType.UInt64
          outValue = toIntRange(value, 0, 9007199254740991)
          break
        case 'byte':
          dataType = DataType.Byte
          outValue = toIntRange(value, 0, 255)
          break
        case 'sbyte':
          dataType = DataType.SByte
          outValue = toIntRange(value, -128, 127)
          break
        case 'boolean':
          dataType = DataType.Boolean
          outValue = value === 0 ? false : true
          break
        case 'datetime':
          dataType = DataType.DateTime
          outValue = new Date(value)
          break
      }
      break
    default:
  }
  return { dataType, value: outValue }
}

// -------------------------------------------------------------------------
// Tag eligibility for history (plan §3.2). connection.historyEnabled and the
// origin===command exclusion are also checked here for a single source of truth.
// -------------------------------------------------------------------------
function isTagEligibleForHistory(connection, element) {
  if (connection.historyEnabled !== true) return false
  if (element.origin === 'command') return false
  // tags with historianPeriod < 0 are explicitly not historized by cs_data_processor
  if (element.historianPeriod < 0) return false
  // Phase 1: only unambiguous scalar types (json/array tags skipped)
  return (
    element.type === 'analog' ||
    element.type === 'digital' ||
    element.type === 'string'
  )
}

// -------------------------------------------------------------------------
// Backend abstraction. Returns { readRange(tag, opts, cb), close() }.
//   opts = { startTime|null, endTime|null, limit, reverse }
//   rows normalized to:
//     { value, valueString, invalid, timeTag, timeTagAtSource, timeTagAtSourceOk }
// -------------------------------------------------------------------------
function createHistorianBackend(connection, jsConfig, getMongoClient, Log) {
  const opts = {
    cap: normInt(connection.historyMaxReturnDataValues, 20000),
    queryTimeoutMs: normInt(connection.historyQueryTimeoutMs, 10000),
    tsField: validateTsField(connection.historyTimestampField),
  }
  const backendName = (connection.historian || 'mongodb').toString().toLowerCase()
  if (backendName === 'postgresql' || backendName === 'postgres') {
    return createPgBackend(connection, jsConfig, Log, opts)
  }
  return createMongoBackend(connection, jsConfig, getMongoClient, Log, opts)
}

// MongoDB backend — reuses the driver's MongoClient (via getMongoClient).
function createMongoBackend(connection, jsConfig, getMongoClient, Log, opts) {
  const tsField = opts.tsField
  return {
    backendName: 'mongodb',
    async readRange(tag, range, callback) {
      try {
        const client = getMongoClient()
        if (!client) return callback(new Error('MongoDB not connected'))
        const coll = client
          .db(jsConfig.mongoDatabaseName)
          .collection(jsConfig.HistCollectionName)
        const q = { tag }
        if (range.startTime || range.endTime) {
          q[tsField] = {}
          if (range.startTime) q[tsField].$gte = range.startTime
          if (range.endTime) q[tsField].$lte = range.endTime
        }
        const rows = await coll
          .find(q)
          .sort({ [tsField]: range.reverse ? -1 : 1 })
          .limit(range.limit)
          .maxTimeMS(opts.queryTimeoutMs)
          .toArray()
        callback(
          null,
          rows.map((r) => ({
            value: r.value,
            // string tags store the string directly in `value`
            valueString:
              typeof r.value === 'string' ? r.value : r.valueString,
            invalid: !!r.invalid,
            timeTag: r.timeTag,
            timeTagAtSource: r.timeTagAtSource || null,
            timeTagAtSourceOk: r.timeTagAtSourceOk !== false,
          }))
        )
      } catch (e) {
        callback(e)
      }
    },
    close() {
      // shares the driver's MongoClient; nothing to close here
    },
  }
}

// build a pg Pool config from connection.postgresConnectionString or PG* env
// vars, falling back to the same defaults as server_realtime_auth
function buildPgPoolConfig(connection, queryTimeoutMs) {
  const cs = connection.postgresConnectionString
  const base = {
    query_timeout: queryTimeoutMs,
    statement_timeout: queryTimeoutMs,
    connectionTimeoutMillis: queryTimeoutMs,
    max: 5,
  }
  if (typeof cs === 'string' && cs.trim() !== '') {
    return { ...base, connectionString: cs.trim() }
  }
  return {
    ...base,
    host: process.env.PGHOST || '127.0.0.1',
    port: parseInt(process.env.PGPORT) || 5432,
    user: process.env.PGUSER || 'json_scada',
    password: process.env.PGPASSWORD || 'json_scada',
    database: process.env.PGDATABASE || 'json_scada',
  }
}

// PostgreSQL/TimescaleDB backend — lazy `pg` require and lazy Pool.
function createPgBackend(connection, jsConfig, Log, opts) {
  let pg = null
  try {
    pg = require('pg')
  } catch (e) {
    Log.log(
      'Historian - PostgreSQL backend selected but the "pg" package is not installed. Run "npm install" in src/OPC-UA-Server.',
      0
    )
    return {
      backendName: 'postgresql',
      readRange(tag, range, callback) {
        callback(new Error('pg package not installed'))
      },
      close() {},
    }
  }

  const tsCol =
    opts.tsField === 'timeTagAtSource' ? 'time_tag_at_source' : 'time_tag'
  let pool = null
  const getPool = () => {
    if (!pool) {
      pool = new pg.Pool(buildPgPoolConfig(connection, opts.queryTimeoutMs))
      pool.on('error', (err) =>
        Log.log('Historian - pg pool error: ' + (err.message || err), 0)
      )
    }
    return pool
  }

  return {
    backendName: 'postgresql',
    async readRange(tag, range, callback) {
      try {
        const order = range.reverse ? 'DESC' : 'ASC'
        const sql =
          'SELECT value, value_json, time_tag, time_tag_at_source, flags ' +
          'FROM hist WHERE tag = $1 ' +
          'AND ($2::timestamptz IS NULL OR ' +
          tsCol +
          ' >= $2) ' +
          'AND ($3::timestamptz IS NULL OR ' +
          tsCol +
          ' <= $3) ' +
          'ORDER BY ' +
          tsCol +
          ' ' +
          order +
          ' LIMIT $4'
        const params = [
          tag,
          range.startTime || null,
          range.endTime || null,
          range.limit,
        ]
        const r = await getPool().query(sql, params)
        callback(
          null,
          r.rows.map((row) => {
            // flags bit(8) is returned by pg as a string of '0'/'1' chars
            let flags = 0
            if (typeof row.flags === 'string' && row.flags.length > 0)
              flags = parseInt(row.flags, 2) || 0
            else if (typeof row.flags === 'number') flags = row.flags
            const vj = row.value_json
            const valueString =
              vj && typeof vj === 'object' ? vj.s : undefined
            return {
              value: row.value,
              valueString,
              invalid: (flags & 0x80) !== 0,
              timeTag: row.time_tag,
              timeTagAtSource: row.time_tag_at_source || null,
              timeTagAtSourceOk: (flags & 0x40) === 0,
            }
          })
        )
      } catch (e) {
        callback(e)
      }
    },
    close() {
      if (pool) {
        const p = pool
        pool = null
        p.end().catch(() => {})
      }
    },
  }
}

// -------------------------------------------------------------------------
// Per-node historian implementing node-opcua's IVariableHistorian.
// -------------------------------------------------------------------------
class JsonScadaVariableHistorian {
  constructor(node, element, backend, connection, Log) {
    this.node = node
    this.tag = element.tag
    this.type = element.type
    this.asdu = element.protocolSourceASDU
    this.backend = backend
    this.cap = normInt(connection.historyMaxReturnDataValues, 20000)
    this.Log = Log
  }

  // Called by node-opcua on each setValueFromSource while historizing===true.
  // No-op: history is written by cs_data_processor; the driver must not
  // double-write.
  push() {}

  extractDataValues(
    details,
    maxNumberToExtract,
    isReversed,
    reverseDataValue,
    callback
  ) {
    let startTime = details ? details.startTime : null
    let endTime = details ? details.endTime : null
    if (isUnbounded(startTime)) startTime = null
    if (isUnbounded(endTime)) endTime = null

    // node-opcua normalizes a reverse request (endTime < startTime) by swapping
    // the times and passing reverseDataValue=true (isReversed is only true when
    // there is no start time). Either way the client wants the newest values
    // first, so query newest-first: with the cap applied this keeps the newest
    // chunk of a large range, and the rows already come in the client's order
    // (no further reversal needed).
    const newestFirst = !!(isReversed || reverseDataValue)

    const limit =
      maxNumberToExtract > 0
        ? Math.min(maxNumberToExtract, this.cap)
        : this.cap
    // when the client did not bound the count (or asked for more than the cap)
    // rows beyond the cap are cut off: detect it by fetching one extra row
    const capApplies = !(maxNumberToExtract > 0 && maxNumberToExtract <= this.cap)

    this.backend.readRange(
      this.tag,
      { startTime, endTime, limit: limit + 1, reverse: newestFirst },
      (err, rows) => {
        if (err) {
          this.Log.log(
            'Historian - readRange error for tag ' +
              this.tag +
              ': ' +
              (err.message || err),
            0
          )
          return callback(err)
        }
        if (rows.length > limit) {
          rows = rows.slice(0, limit)
          if (capApplies && details) {
            // read by the wrapper installed in installTagHistory, which reports
            // GoodMoreData so the client knows the result is incomplete
            details.__jsonscadaTruncated = true
            this.Log.log(
              'Historian - result for tag ' +
                this.tag +
                ' truncated at ' +
                this.cap +
                ' values (historyMaxReturnDataValues)',
              2
            )
          }
        }
        const dataValues = []
        for (const row of rows) {
          const dv = this._rowToDataValue(row)
          if (dv) dataValues.push(dv)
        }
        callback(null, dataValues)
      }
    )
  }

  _rowToDataValue(row) {
    try {
      const { dataType, value } = convertHistValue(
        this.type,
        this.asdu,
        row.value,
        row.valueString
      )
      if (!dataType) return null
      // defensive: skip corrupt non-numeric analog rows
      if (
        this.type === 'analog' &&
        typeof value === 'number' &&
        Number.isNaN(value)
      ) {
        this.Log.log(
          'Historian - skipping NaN analog row for tag ' + this.tag,
          3
        )
        return null
      }
      return new DataValue({
        value: new Variant({ dataType, value }),
        statusCode: row.invalid ? StatusCodes.Bad : StatusCodes.Good,
        // HA clients filter/plot on sourceTimestamp
        sourceTimestamp: row.timeTagAtSource || row.timeTag,
        serverTimestamp: row.timeTag,
      })
    } catch (e) {
      this.Log.log(
        'Historian - row map error for tag ' + this.tag + ': ' + (e.message || e),
        3
      )
      return null
    }
  }
}

// -------------------------------------------------------------------------
// "Lite" install: replicate only the functional part of
// installHistoricalDataNode (no per-node "HA Configuration" object tree) to
// keep the address space small at high tag counts. The internal _historyRead*
// functions are not exported, so we capture them once from a sacrificial full
// install (they are generic, `this`-based, identical for every node).
// -------------------------------------------------------------------------
let liteTemplate = null
function getLiteTemplate(addressSpace, namespace, Log) {
  if (liteTemplate) return liteTemplate
  const tmp = namespace.addVariable({
    browseName: '__jsonscada_hist_template__',
    dataType: 'Double',
  })
  addressSpace.installHistoricalDataNode(tmp, {
    historian: {
      push() {},
      extractDataValues(d, m, i, r, cb) {
        cb(null, [])
      },
    },
  })
  liteTemplate = {
    _historyRead: tmp._historyRead,
    _historyReadRaw: tmp._historyReadRaw,
    _historyReadRawAsync: tmp._historyReadRawAsync,
    _historyReadRawModify: tmp._historyReadRawModify,
    _historyReadModify: tmp._historyReadModify,
    _historyPush: tmp._historyPush,
  }
  // remove the sacrificial node and its HA config object to keep it clean
  try {
    if (tmp.$historicalDataConfiguration)
      addressSpace.deleteNode(tmp.$historicalDataConfiguration)
  } catch (e) {}
  try {
    addressSpace.deleteNode(tmp)
  } catch (e) {}
  if (
    !liteTemplate._historyRead ||
    !liteTemplate._historyReadRaw ||
    !liteTemplate._historyReadRawAsync
  ) {
    Log.log(
      'Historian - lite install could not capture node-opcua internals; falling back to full install',
      0
    )
    liteTemplate = false // sentinel: capture failed
  }
  return liteTemplate
}

function installTagHistoryLite(addressSpace, namespace, node, historian, Log) {
  const t = getLiteTemplate(addressSpace, namespace, Log)
  if (!t) {
    // capture failed → fall back to a full install so history still works
    addressSpace.installHistoricalDataNode(node, { historian })
    return
  }
  node._historyRead = t._historyRead
  node._historyReadRaw = t._historyReadRaw
  node._historyReadRawAsync = t._historyReadRawAsync
  node._historyReadRawModify = t._historyReadRawModify
  node._historyReadModify = t._historyReadModify
  node._historyPush = t._historyPush
  node.varHistorian = historian
  node.historizing = true
  node.accessLevel =
    node.accessLevel | AccessLevelFlag.CurrentRead | AccessLevelFlag.HistoryRead
  if (node.userAccessLevel !== undefined) {
    node.userAccessLevel =
      node.userAccessLevel |
      AccessLevelFlag.CurrentRead |
      AccessLevelFlag.HistoryRead
  }
}

// Wrap the node's HistoryRead raw handler so a result cut off at the
// historyMaxReturnDataValues cap is reported as GoodMoreData (instead of Good
// with silently missing values). The historian flags the request details.
function wrapHistoryReadRaw(node) {
  const original = node._historyReadRaw
  if (typeof original !== 'function' || original.__jsonscadaWrapped) return
  const wrapped = function (context, details, indexRange, dataEncoding, cont, callback) {
    return original.call(
      this,
      context,
      details,
      indexRange,
      dataEncoding,
      cont,
      (err, result) => {
        if (
          !err &&
          result &&
          details &&
          details.__jsonscadaTruncated &&
          !result.continuationPoint &&
          result.statusCode &&
          result.statusCode.isGood()
        ) {
          result.statusCode = StatusCodes.GoodMoreData
        }
        callback(err, result)
      }
    )
  }
  wrapped.__jsonscadaWrapped = true
  node._historyReadRaw = wrapped
}

// -------------------------------------------------------------------------
// Install history on a single created UA variable node.
// -------------------------------------------------------------------------
function installTagHistory(
  namespace,
  addressSpace,
  node,
  element,
  backend,
  connection,
  Log
) {
  const historian = new JsonScadaVariableHistorian(
    node,
    element,
    backend,
    connection,
    Log
  )
  // default: full HA Configuration (spec-compliant). Lite when explicitly false.
  const fullHA = connection.historyFullHAConfiguration !== false
  if (fullHA) {
    addressSpace.installHistoricalDataNode(node, { historian })
    // Stepped: digital/string are stepped; analog is sloped-interpolated.
    // Affects raw display and aggregate math.
    try {
      const cfg = node.$historicalDataConfiguration
      if (cfg && cfg.stepped) {
        cfg.stepped.setValueFromSource({
          dataType: DataType.Boolean,
          value: element.type !== 'analog',
        })
      }
    } catch (e) {}
  } else {
    installTagHistoryLite(addressSpace, namespace, node, historian, Log)
  }
  wrapHistoryReadRaw(node)
}

module.exports = {
  convertHistValue,
  isTagEligibleForHistory,
  createHistorianBackend,
  installTagHistory,
  JsonScadaVariableHistorian,
  isUnbounded,
}
