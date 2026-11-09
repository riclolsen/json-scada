/*
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

// Health diagnosis: database, processes, drivers, connections, point quality,
// command and event queues and process logs, summarized into findings an
// assistant can explain. Read-only (MongoDB and the log folder).

import { z } from 'zod'
import type { McpServer } from '@modelcontextprotocol/server'
import { ConnectionManager } from '../jsonscada/connection-manager.js'
import { aliveInfo, isFresh } from './status.js'
import { listLogFiles, logDir, scanLog } from './logs.js'
import { errorResult, jsonResult, notConnectedResult, redactSensitive } from './util.js'

// Service (and log file) base name of each managed driver, instance 1; other
// instances append _<n>. Keep in step with server_realtime_auth
// app/services/process-manager/driver-catalog.js.
const DRIVER_SERVICE: Record<string, string> = {
  'IEC60870-5-104': 'iec104client',
  'IEC60870-5-104_SERVER': 'iec104server',
  'IEC60870-5-101': 'iec101client',
  'IEC60870-5-101_SERVER': 'iec101server',
  DNP3: 'dnp3client',
  DNP3_SERVER: 'dnp3server',
  IEC61850: 'iec61850client',
  IEC61850_SERVER: 'iec61850server',
  ICCP: 'iccpclient',
  ICCP_SERVER: 'iccpserver',
  'MQTT-SPARKPLUG-B': 'mqttsparkplugclient',
  'OPC-UA': 'opcuaclient',
  'OPC-UA_SERVER': 'opcuaserver',
  'OPC-DA': 'opcdaclient',
  'OPC-DA_SERVER': 'opcdaserver',
  PLC4X: 'plc4jclient',
  'TELEGRAF-LISTENER': 'telegraf_listener',
  'NODE-RED': 'nodered_driver',
  N8N: 'n8nclient',
  I104M: 'i104m',
  ONVIF: 'onvif',
  'CIM-EMS': 'cim_ems',
  MODBUS: 'modbusclient',
  MODBUS_SERVER: 'modbusserver',
}

// Log file base name of a driver instance or a process (supervisor program name)
export function serviceLogName(driverOrProcess: string, instance = 1): string {
  const key = DRIVER_SERVICE[driverOrProcess] ?? driverOrProcess.toLowerCase().replace(/[^a-z0-9_]+/g, '_')
  return instance > 1 ? `${key}_${instance}` : key
}

const PROCESS_LOG: Record<string, string> = {
  CS_DATA_PROCESSOR: 'cs_data_processor',
  CS_CUSTOM_PROCESSOR: 'cs_custom_processor',
  CALCULATIONS: 'calculations',
  SERVER_REALTIME: 'server_realtime',
  SERVER_REALTIME_AUTH: 'server_realtime_auth',
  PG_HIST: 'pg_hist',
  PG_RTDATA: 'pg_rtdata',
  MONGOFW: 'mongofw',
  MONGOWR: 'mongowr',
  ALARM_BEEP: 'alarm_beep',
}

async function logsFor(baseNames: string[], sinceMinutes: number, maxMatches: number) {
  try {
    const files = await listLogFiles()
    const since = new Date(Date.now() - sinceMinutes * 60000)
    const out = []
    for (const base of baseNames) {
      const f = files.find((x) => x.file.toLowerCase() === `${base.toLowerCase()}.log`)
      if (!f) {
        out.push({ file: `${base}.log`, found: false })
        continue
      }
      const s = await scanLog(f.file, { level: 'warn', since, maxBytes: 2 << 20, maxMatches })
      out.push({ file: f.file, found: true, modified: f.modified, ageSeconds: f.ageSeconds, recentWarnOrErrors: s.totalMatches, lastLines: s.lines })
    }
    return out
  } catch (e) {
    return [{ error: `log folder ${logDir()} not readable: ${(e as Error).message}` }]
  }
}

// "no keep-alive for 42 s" / "no keep-alive since <date>" / "never started"
function since(t: unknown, now: number): string {
  if (!(t instanceof Date)) return 'never started'
  const s = Math.round((now - t.getTime()) / 1000)
  return s < 86400 ? `no keep-alive for ${s} s` : `no keep-alive since ${t.toISOString()}`
}

const ageS = (t: unknown, now: number) => (t instanceof Date ? Math.round((now - t.getTime()) / 1000) : null)

// Point quality counts of supervised points per connection
async function pointStatsByConnection(mgr: ConnectionManager, match: Record<string, unknown> = {}) {
  const rows = await mgr
    .getRealtimeDataCollection()
    .aggregate([
      { $match: { origin: 'supervised', ...match } },
      {
        $group: {
          _id: '$protocolSourceConnectionNumber',
          points: { $sum: 1 },
          invalid: { $sum: { $cond: [{ $eq: ['$invalid', true] }, 1, 0] } },
          alarmed: { $sum: { $cond: [{ $eq: ['$alarmed', true] }, 1, 0] } },
          alarmDisabled: { $sum: { $cond: [{ $eq: ['$alarmDisabled', true] }, 1, 0] } },
          substituted: { $sum: { $cond: [{ $eq: ['$substituted', true] }, 1, 0] } },
          frozen: { $sum: { $cond: [{ $eq: ['$frozen', true] }, 1, 0] } },
          newestUpdate: { $max: '$timeTag' },
          oldestUpdate: { $min: '$timeTag' },
        },
      },
      { $sort: { _id: 1 } },
    ])
    .toArray()
  return rows.map(({ _id, ...r }) => ({ connectionNumber: _id, ...r }))
}

async function mongoHealth(mgr: ConnectionManager) {
  const out: Record<string, unknown> = {}
  const admin = mgr.db.admin()
  try {
    const s: any = await admin.command({ serverStatus: 1, repl: 1, metrics: 0, locks: 0, wiredTiger: 0 })
    out['server'] = {
      version: s.version,
      host: s.host,
      uptimeHours: Math.round((s.uptime / 3600) * 10) / 10,
      connections: s.connections ? { current: s.connections.current, available: s.connections.available } : undefined,
      memResidentMB: s.mem?.resident,
    }
  } catch (e) {
    out['server'] = { error: (e as Error).message }
  }
  try {
    const r: any = await admin.command({ replSetGetStatus: 1 })
    out['replicaSet'] = {
      set: r.set,
      members: (r.members || []).map((m: any) => ({
        name: m.name,
        state: m.stateStr,
        health: m.health,
        lagSeconds: r.members.find((p: any) => p.stateStr === 'PRIMARY')?.optimeDate && m.optimeDate
          ? Math.round((r.members.find((p: any) => p.stateStr === 'PRIMARY').optimeDate - m.optimeDate) / 1000)
          : null,
      })),
    }
    const oplog = mgr.client!.db('local').collection('oplog.rs')
    const first: any = await oplog.find({}, { projection: { ts: 1 } }).sort({ $natural: 1 }).limit(1).next()
    const last: any = await oplog.find({}, { projection: { ts: 1 } }).sort({ $natural: -1 }).limit(1).next()
    if (first && last) out['oplogWindowHours'] = Math.round(((last.ts.getHighBits() - first.ts.getHighBits()) / 3600) * 10) / 10
  } catch (e) {
    out['replicaSet'] = { error: (e as Error).message }
  }
  try {
    const d: any = await mgr.db.command({ dbStats: 1, scale: 1024 * 1024 })
    out['database'] = {
      name: d.db,
      collections: d.collections,
      dataMB: Math.round(d.dataSize),
      storageMB: Math.round(d.storageSize),
      indexMB: Math.round(d.indexSize),
      ...(d.fsTotalSize ? { fsUsedPct: Math.round((d.fsUsedSize / d.fsTotalSize) * 1000) / 10, fsFreeMB: Math.round(d.fsTotalSize - d.fsUsedSize) } : {}),
    }
  } catch (e) {
    out['database'] = { error: (e as Error).message }
  }
  return out
}

export function registerHealthTools(server: McpServer, mgr: ConnectionManager) {
  server.registerTool(
    'diagnose_system',
    {
      title: 'Diagnose System',
      description:
        'Overall health check of the JSON-SCADA installation, with a list of findings to explain or act on: ' +
        'MongoDB (version, uptime, replica set members and lag, oplog window, database and disk usage), ' +
        'processes (cs_data_processor, calculations, realtime server, ...) and protocol driver instances alive ' +
        'or not (keep-alive), connections per driver with stale statistics, point quality per connection ' +
        '(invalid, alarmed = unacknowledged alarms, alarm-disabled, substituted, frozen), command queue ' +
        '(undelivered / failed commands), unacknowledged events and recent errors in the process logs. Follow ' +
        'up on a connection with diagnose_connection and on a log with read_log.',
      inputSchema: z.object({
        logSinceMinutes: z.number().int().min(1).max(100000).optional().default(60),
        scanLogs: z.boolean().optional().default(true),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ logSinceMinutes, scanLogs }) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()
      try {
        const now = Date.now()
        const findings: string[] = []
        const mongo = await mongoHealth(mgr)
        const rs: any = mongo['replicaSet']
        if (rs?.members) {
          for (const m of rs.members) {
            if (m.health !== 1) findings.push(`MongoDB member ${m.name} unhealthy (${m.state})`)
            if (m.lagSeconds > 30) findings.push(`MongoDB member ${m.name} lags ${m.lagSeconds} s behind the primary`)
          }
          if (!rs.members.some((m: any) => m.state === 'PRIMARY')) findings.push('MongoDB replica set has no PRIMARY')
        } else if (rs?.error) findings.push(`MongoDB is not a replica set or status unavailable (${rs.error}); change streams need a replica set`)
        const db: any = mongo['database']
        if (db?.fsUsedPct > 90) findings.push(`Database disk ${db.fsUsedPct} % used`)
        if (typeof mongo['oplogWindowHours'] === 'number' && (mongo['oplogWindowHours'] as number) < 1)
          findings.push(`Oplog window only ${mongo['oplogWindowHours']} h: change streams may lose position after outages`)

        const processes = (await mgr.getProcessInstancesCollection().find().toArray()).map((p) => {
          const a = aliveInfo(p, now)
          if (p.enabled !== false && !a['alive'])
            findings.push(`Process ${p.processName} #${p.processInstanceNumber} not alive (${since(p.activeNodeKeepAliveTimeTag, now)}): see read_log ${PROCESS_LOG[String(p.processName)] ?? String(p.processName).toLowerCase()}.log`)
          return {
            processName: p.processName,
            instance: p.processInstanceNumber,
            enabled: p.enabled,
            alive: a['alive'],
            activeNodeName: a['activeNodeName'],
            keepAliveAgeSeconds: ageS(p.activeNodeKeepAliveTimeTag, now),
            ...(p['stats'] ? { stats: p['stats'] } : {}),
          }
        })
        if (!processes.some((p) => p.processName === 'CS_DATA_PROCESSOR' && p.alive))
          findings.push('cs_data_processor is not alive: incoming values are not processed into realtimeData (no alarms, no history)')
        const csdp: any = processes.find((p) => p.processName === 'CS_DATA_PROCESSOR' && p.alive)
        if (csdp?.stats?.latencyAvgMinute > 1000) findings.push(`cs_data_processor latency high (${csdp.stats.latencyAvgMinute} ms average in the last minute)`)

        const conns = await mgr
          .getProtocolConnectionsCollection()
          .find()
          .project({ protocolDriver: 1, protocolDriverInstanceNumber: 1, protocolConnectionNumber: 1, name: 1, enabled: 1, stats: 1 })
          .toArray()
        const newest = new Map<string, Date>()
        for (const c of conns) {
          const t = c['stats']?.timeTag
          const k = `${c['protocolDriver']}#${c['protocolDriverInstanceNumber']}`
          if (t instanceof Date && (!newest.has(k) || t > newest.get(k)!)) newest.set(k, t)
        }
        const driverDocs = await mgr.getProtocolDriverInstancesCollection().find().toArray()
        const drivers = driverDocs.map((d) => {
          const a = aliveInfo(d, now, newest.get(`${d.protocolDriver}#${d.protocolDriverInstanceNumber}`) ?? null)
          const nConns = conns.filter((c) => c['protocolDriver'] === d.protocolDriver && c['protocolDriverInstanceNumber'] === d.protocolDriverInstanceNumber).length
          if (d.enabled !== false && !a['alive'])
            findings.push(`Driver ${d.protocolDriver} #${d.protocolDriverInstanceNumber} enabled but not alive (process stopped, crashing or not able to reach MongoDB): see read_log ${serviceLogName(d.protocolDriver, Number(d.protocolDriverInstanceNumber))}.log`)
          if (d.enabled !== false && !nConns) findings.push(`Driver ${d.protocolDriver} #${d.protocolDriverInstanceNumber} has no connections`)
          return {
            protocolDriver: d.protocolDriver,
            instance: d.protocolDriverInstanceNumber,
            enabled: d.enabled,
            alive: a['alive'],
            aliveSource: a['aliveSource'],
            activeNodeName: a['activeNodeName'],
            nodeNames: d['nodeNames'],
            connections: nConns,
          }
        })
        const pointStats = new Map((await pointStatsByConnection(mgr)).map((r) => [r.connectionNumber, r]))
        const connections = conns.map((c) => {
          const ps: any = pointStats.get(c['protocolConnectionNumber'])
          const drv = drivers.find((d) => d.protocolDriver === c['protocolDriver'] && d.instance === c['protocolDriverInstanceNumber'])
          const row = {
            connectionNumber: c['protocolConnectionNumber'],
            name: c['name'],
            protocolDriver: c['protocolDriver'],
            driverInstance: c['protocolDriverInstanceNumber'],
            enabled: c['enabled'],
            driverAlive: drv?.alive ?? false,
            statsFresh: isFresh(c['stats']?.timeTag, now),
            points: ps?.points ?? 0,
            invalid: ps?.invalid ?? 0,
            alarmed: ps?.alarmed ?? 0,
            alarmDisabled: ps?.alarmDisabled ?? 0,
            substituted: ps?.substituted ?? 0,
            frozen: ps?.frozen ?? 0,
            lastUpdateAgeSeconds: ageS(ps?.newestUpdate, now),
          }
          if (c['enabled'] !== false && drv?.enabled !== false) {
            if (!drv) findings.push(`Connection ${row.connectionNumber} ${row.name}: no driver instance ${row.protocolDriver} #${row.driverInstance} configured`)
            if (row.points && row.invalid === row.points) findings.push(`Connection ${row.connectionNumber} ${row.name}: all ${row.points} points invalid (link down or device not responding)`)
            else if (row.points && row.invalid / row.points > 0.2) findings.push(`Connection ${row.connectionNumber} ${row.name}: ${row.invalid}/${row.points} points invalid`)
          }
          if (!row.points && c['protocolDriver'] && !/_SERVER$/.test(String(c['protocolDriver'])) && c['enabled'] !== false)
            findings.push(`Connection ${row.connectionNumber} ${row.name}: no supervised points (auto-tag creation off or nothing received yet)`)
          return row
        })

        const rtd = mgr.getRealtimeDataCollection()
        const [totalPoints, totalInvalid, totalAlarmed, totalAlarmDisabled, totalSubstituted] = await Promise.all([
          rtd.estimatedDocumentCount(),
          rtd.countDocuments({ invalid: true, origin: { $in: ['supervised', 'calculated'] } } as any),
          rtd.countDocuments({ alarmed: true } as any),
          rtd.countDocuments({ alarmDisabled: true } as any),
          rtd.countDocuments({ substituted: true } as any),
        ])
        const dayAgo = new Date(now - 86400000)
        const cq = mgr.getCommandsQueueCollection()
        const [cmds24h, undelivered, failed] = await Promise.all([
          cq.countDocuments({ timeTag: { $gte: dayAgo } }),
          cq.countDocuments({ timeTag: { $gte: dayAgo, $lte: new Date(now - 60000) }, delivered: { $ne: true } } as any),
          cq.countDocuments({ timeTag: { $gte: dayAgo }, ack: false } as any),
        ])
        if (undelivered) findings.push(`${undelivered} commands in the last 24 h never delivered to a driver (driver down or commands disabled on the connection)`)
        if (failed) findings.push(`${failed} commands in the last 24 h rejected by the device or driver (ack false)`)
        const soe = mgr.getSoeDataCollection()
        const [events24h, unackedEvents] = await Promise.all([
          soe.countDocuments({ timeTag: { $gte: dayAgo } }),
          soe.countDocuments({ ack: 0 } as any, { limit: 100000 }),
        ])

        let logs: unknown = undefined
        if (scanLogs) {
          try {
            const since = new Date(now - logSinceMinutes * 60000)
            const files = (await listLogFiles()).filter((f) => !f.rotated && f.modified >= since)
            const rows = []
            for (const f of files) {
              const s = await scanLog(f.file, { level: 'error', since, maxBytes: 1 << 20, maxMatches: 3 })
              if (s.totalMatches) rows.push({ file: f.file, errors: s.totalMatches, last: s.lines })
            }
            rows.sort((a, b) => b.errors - a.errors)
            for (const r of rows.slice(0, 5)) findings.push(`Log ${r.file}: ${r.errors} error lines in the last ${logSinceMinutes} min`)
            logs = { folder: logDir(), sinceMinutes: logSinceMinutes, filesWithErrors: rows }
          } catch (e) {
            logs = { error: `log folder ${logDir()} not readable: ${(e as Error).message}` }
          }
        }

        return jsonResult({
          findings: findings.length ? findings : ['No problems found'],
          mongodb: mongo,
          processes,
          drivers,
          connections,
          points: { total: totalPoints, invalid: totalInvalid, alarmed: totalAlarmed, alarmDisabled: totalAlarmDisabled, substituted: totalSubstituted },
          commands: { last24h: cmds24h, undelivered, failed },
          events: { last24h: events24h, unacknowledged: unackedEvents },
          ...(logs ? { logs } : {}),
        })
      } catch (e) {
        return errorResult('Error diagnosing the system', e)
      }
    }
  )

  server.registerTool(
    'diagnose_connection',
    {
      title: 'Diagnose Connection',
      description:
        'Troubleshoot one protocol connection: its configuration (endpoints, addresses, timeouts; secrets ' +
        'redacted), the driver instance (enabled, alive, active node), connection statistics freshness, point ' +
        'quality (invalid, frozen, substituted, alarmed; samples of invalid and stale points), recent commands ' +
        'with their delivery/ack results and the recent warnings/errors of the driver log and of other logs ' +
        'mentioning the connection name. Ends with findings (e.g. driver not running, link down, commands not ' +
        'delivered, configuration mismatch suspected from the log).',
      inputSchema: z.object({
        connectionNumber: z.number().int(),
        logSinceMinutes: z.number().int().min(1).max(100000).optional().default(120),
        samples: z.number().int().min(0).max(200).optional().default(15),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ connectionNumber, logSinceMinutes, samples }) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()
      try {
        const now = Date.now()
        const findings: string[] = []
        const conn: any = await mgr.getProtocolConnectionsCollection().findOne({ protocolConnectionNumber: connectionNumber } as any)
        if (!conn) return jsonResult({ connectionNumber, findings: [`Connection ${connectionNumber} not found`] })
        const { _id, stats, ...config } = conn
        const drv: any = await mgr
          .getProtocolDriverInstancesCollection()
          .findOne({ protocolDriver: conn.protocolDriver, protocolDriverInstanceNumber: conn.protocolDriverInstanceNumber } as any)
        const driver: any = drv
          ? {
              enabled: drv.enabled,
              logLevel: drv.logLevel,
              nodeNames: drv.nodeNames,
              ...aliveInfo(drv, now, stats?.timeTag instanceof Date ? stats.timeTag : null),
            }
          : null
        if (!driver) findings.push(`No driver instance ${conn.protocolDriver} #${conn.protocolDriverInstanceNumber}: create it (create_driver_instance)`)
        else {
          if (driver.enabled === false) findings.push('Driver instance disabled')
          else if (!driver['alive']) findings.push('Driver instance not alive: the process is stopped, crashing or cannot reach MongoDB (see logs below, control_driver_process)')
          if (Array.isArray(drv.nodeNames) && drv.nodeNames.length && !drv.nodeNames.includes(drv.activeNodeName) && drv.activeNodeName)
            findings.push(`Active node ${drv.activeNodeName} is not in the instance nodeNames`)
        }
        if (conn.enabled === false) findings.push('Connection disabled')
        const statsAge = ageS(stats?.timeTag, now)
        if (statsAge !== null && statsAge > 60 && driver?.['alive']) findings.push(`Connection statistics not updated for ${statsAge} s`)

        const rtd = mgr.getRealtimeDataCollection()
        const ps: any = (await pointStatsByConnection(mgr, { protocolSourceConnectionNumber: connectionNumber }))[0]
        const proj = { _id: 1, tag: 1, type: 1, value: 1, invalid: 1, timeTag: 1, timeTagAtSource: 1, protocolSourceCommonAddress: 1, protocolSourceObjectAddress: 1, protocolSourceASDU: 1, invalidDetectTimeout: 1 }
        const [invalidSample, staleSample, byType] = await Promise.all([
          rtd.find({ protocolSourceConnectionNumber: connectionNumber, invalid: true } as any, { projection: proj }).limit(samples).toArray(),
          rtd.find({ protocolSourceConnectionNumber: connectionNumber, origin: 'supervised' } as any, { projection: proj }).sort({ timeTag: 1 }).limit(Math.min(samples, 10)).toArray(),
          rtd.aggregate([{ $match: { protocolSourceConnectionNumber: connectionNumber, origin: 'supervised' } }, { $group: { _id: '$type', n: { $sum: 1 } } }]).toArray(),
        ])
        if (ps?.points) {
          if (ps.invalid === ps.points) findings.push(`All ${ps.points} points invalid: no data arriving (link down, wrong address/port, device not answering, or addresses not matching the device)`)
          else if (ps.invalid && conn.protocolDriver === 'CIM-EMS')
            findings.push(`${ps.invalid} of ${ps.points} result points invalid: de-energized elements, failed or stale studies (see cim_ems_status)`)
          else if (ps.invalid) findings.push(`${ps.invalid} of ${ps.points} points invalid: check those addresses (samples below) and the device configuration`)
          if (ps.frozen) findings.push(`${ps.frozen} analog points frozen (value not changing)`)
          if (ps.substituted) findings.push(`${ps.substituted} points substituted by operators`)
        } else if (!/_SERVER$/.test(conn.protocolDriver)) findings.push('No supervised points mapped to this connection')

        const cmds = await mgr
          .getCommandsQueueCollection()
          .find({ protocolSourceConnectionNumber: connectionNumber } as any)
          .sort({ timeTag: -1 })
          .limit(10)
          .toArray()
        const commands = cmds.map((c: any) => ({
          tag: c.tag, value: c.value, timeTag: c.timeTag, delivered: c.delivered, ack: c.ack, ackTimeTag: c.ackTimeTag,
          resultDescription: c.resultDescription, cancelled: c.cancelled, cancelReason: c.cancelReason, originatorUserName: c.originatorUserName,
        }))
        if (cmds.some((c: any) => !c.delivered && now - c.timeTag.getTime() > 60000)) findings.push('Commands pending delivery (driver down, commandsEnabled false or command not addressed to this connection)')
        if (cmds.some((c: any) => c.ack === false)) findings.push('Recent commands rejected (negative ack): check command addresses/ASDU and device permissions')
        if (cmds.length && conn.commandsEnabled === false) findings.push('commandsEnabled is false on this connection')

        const bases = [serviceLogName(conn.protocolDriver, conn.protocolDriverInstanceNumber)]
        const driverLogs = await logsFor(bases, logSinceMinutes, 25)
        let mentions: unknown[] = []
        try {
          const name = String(conn.name || '').replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
          if (name) {
            const since = new Date(now - logSinceMinutes * 60000)
            for (const f of (await listLogFiles()).filter((f) => !f.rotated && f.modified >= since && !bases.includes(f.file.replace(/\.log$/i, '')))) {
              const s = await scanLog(f.file, { pattern: new RegExp(name, 'i'), since, maxBytes: 1 << 20, maxMatches: 5 })
              if (s.totalMatches) mentions.push({ file: f.file, matches: s.totalMatches, lastLines: s.lines })
            }
          }
        } catch {
          mentions = []
        }
        const dl: any = driverLogs[0]
        if (dl?.recentWarnOrErrors) findings.push(`Driver log ${dl.file} has ${dl.recentWarnOrErrors} warning/error lines in the last ${logSinceMinutes} min (see logs.driver)`)
        if (dl && dl.found === false) findings.push(`Driver log ${dl.file} not found in ${logDir()}`)

        return jsonResult({
          connectionNumber,
          findings: findings.length ? findings : ['No problems found'],
          config: redactSensitive(config),
          stats: stats ? { ...stats, ageSeconds: statsAge } : null,
          driver,
          points: ps
            ? { ...ps, newestUpdateAgeSeconds: ageS(ps.newestUpdate, now), oldestUpdateAgeSeconds: ageS(ps.oldestUpdate, now), byType: Object.fromEntries(byType.map((t: any) => [t._id, t.n])) }
            : { points: 0 },
          invalidSample,
          leastRecentlyUpdated: staleSample,
          recentCommands: commands,
          logs: { driver: driverLogs, otherMentions: mentions },
        })
      } catch (e) {
        return errorResult(`Error diagnosing connection ${connectionNumber}`, e)
      }
    }
  )
}
