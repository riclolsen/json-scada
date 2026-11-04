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

import type { McpServer } from '@modelcontextprotocol/server'
import { ConnectionManager } from '../jsonscada/connection-manager.js'
import { errorResult, notConnectedResult, textResult } from './util.js'

// A process/driver instance is considered alive when its keep-alive was
// refreshed within this window. The same window applies to connection stats.
const KEEPALIVE_TOLERANCE_MS = 30000

function isFresh(t: unknown, now: number): t is Date {
  return t instanceof Date && now - t.getTime() < KEEPALIVE_TOLERANCE_MS
}

// Liveness of a process or driver instance. The instance keep-alive is the
// primary signal. Server drivers have no redundancy election and some never
// write it, so for a driver instance with a stale keep-alive the newest
// stats.timeTag among its connections is used as a fallback. aliveSource says
// which signal decided: 'keepAlive', 'connectionStats' or null (not alive).
function aliveInfo(doc: any, now: number, connStatsTime?: Date | null) {
  const keepAlive: Date | null | undefined = doc.activeNodeKeepAliveTimeTag
  let alive = false
  let aliveSource: 'keepAlive' | 'connectionStats' | null = null
  if (isFresh(keepAlive, now)) {
    alive = true
    aliveSource = 'keepAlive'
  } else if (isFresh(connStatsTime, now)) {
    alive = true
    aliveSource = 'connectionStats'
  }
  const info: Record<string, unknown> = {
    activeNodeName: doc.activeNodeName || null,
    activeNodeKeepAliveTimeTag: keepAlive || null,
    alive,
    aliveSource,
  }
  if (connStatsTime !== undefined)
    info['lastConnectionStatsTimeTag'] = connStatsTime || null
  return info
}

export function registerStatusTools(server: McpServer, mgr: ConnectionManager) {
  server.registerTool(
    'get_system_status',
    {
      title: 'Get System Status',
      annotations: { readOnlyHint: true },
      description:
        'Get a summary of system health: process instances, protocol driver instances ' +
        'and protocol connections. An instance is alive when its keep-alive is under 30 s old; ' +
        'for a driver instance with a stale keep-alive (server drivers without redundancy may not ' +
        'write one), fresh connection stats count instead. aliveSource tells which signal decided; ' +
        'statsFresh marks connections whose stats were refreshed within 30 s',
    },
    async () => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()
      const now = Date.now()

      try {
        const processes = (
          await mgr.getProcessInstancesCollection().find().toArray()
        ).map((p) => ({
          processName: p.processName,
          processInstanceNumber: p.processInstanceNumber,
          enabled: p.enabled,
          softwareVersion: p.softwareVersion || null,
          ...aliveInfo(p, now),
        }))

        const connDocs = await mgr
          .getProtocolConnectionsCollection()
          .find()
          .project({
            protocolDriver: 1,
            protocolDriverInstanceNumber: 1,
            protocolConnectionNumber: 1,
            name: 1,
            description: 1,
            enabled: 1,
            commandsEnabled: 1,
            stats: 1,
          })
          .toArray()

        // Newest stats.timeTag per driver instance.
        const instKey = (d: any) =>
          `${d.protocolDriver}#${d.protocolDriverInstanceNumber}`
        const newestStats = new Map<string, Date>()
        for (const c of connDocs) {
          const t = c['stats']?.timeTag
          if (!(t instanceof Date)) continue
          const k = instKey(c)
          const prev = newestStats.get(k)
          if (!prev || t > prev) newestStats.set(k, t)
        }

        const drivers = (
          await mgr.getProtocolDriverInstancesCollection().find().toArray()
        ).map((d) => ({
          protocolDriver: d.protocolDriver,
          protocolDriverInstanceNumber: d.protocolDriverInstanceNumber,
          enabled: d.enabled,
          softwareVersion: d.softwareVersion || null,
          ...aliveInfo(d, now, newestStats.get(instKey(d)) ?? null),
        }))

        const connections = connDocs.map((c) => {
          const { _id, ...rest } = c
          return { ...rest, statsFresh: isFresh(c['stats']?.timeTag, now) }
        })

        return textResult(
          JSON.stringify(
            {
              processInstances: processes,
              protocolDriverInstances: drivers,
              protocolConnections: connections,
            },
            null,
            2
          )
        )
      } catch (e) {
        return errorResult('Error retrieving system status', e)
      }
    }
  )
}
