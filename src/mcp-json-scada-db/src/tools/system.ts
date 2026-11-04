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

import { z } from 'zod'
import type { McpServer } from '@modelcontextprotocol/server'
import { AdminApiClient } from '../jsonscada/admin-api.js'
import { ConnectionManager } from '../jsonscada/connection-manager.js'
import {
  errorResult,
  jsonResult,
  notConnectedResult,
  parseDateArg,
} from './util.js'

// Read-only audit trail; works without the admin API
export function registerAuditTools(server: McpServer, mgr: ConnectionManager) {
  server.registerTool(
    'get_user_actions',
    {
      title: 'Get User Actions',
      description:
        'Read the audit log (userActions): configuration changes, commands, sign-ins and ' +
        'service actions made by users, the AdminUI and this MCP server. Newest first.',
      inputSchema: z.object({
        username: z
          .string()
          .optional()
          .describe('Exact user name (MCP command senders are prefixed "MCP:")'),
        action: z
          .string()
          .optional()
          .describe('Exact action, e.g. Command, updateProtocolConnection, createTag'),
        tag: z.string().optional(),
        from: z.string().optional().describe('Start time, ISO 8601'),
        to: z.string().optional().describe('End time, ISO 8601'),
        limit: z.number().int().min(1).max(500).optional().default(50),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ username, action, tag, from, to, limit }) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()
      try {
        const filter: Record<string, any> = {}
        if (username) filter['username'] = username
        if (action) filter['action'] = action
        if (tag) filter['tag'] = tag
        const fromDate = parseDateArg(from, 'from')
        const toDate = parseDateArg(to, 'to')
        if (fromDate || toDate)
          filter['timeTag'] = {
            ...(fromDate ? { $gte: fromDate } : {}),
            ...(toDate ? { $lte: toDate } : {}),
          }
        const docs = await mgr
          .getUserActionsCollection()
          .find(filter as any)
          .sort({ timeTag: -1 })
          .limit(limit)
          .project({ _id: 0 })
          .toArray()
        return jsonResult(docs)
      } catch (e) {
        return errorResult('Error reading user actions', e)
      }
    }
  )
}

export function registerSystemTools(server: McpServer, api: AdminApiClient) {
  server.registerTool(
    'get_system_settings',
    {
      title: 'Get System Settings',
      description:
        'Get the global process management settings: autoManageServices (create/update/remove ' +
        'OS services with driver instances), autoRestartOnConnectionChange, and whether process ' +
        'management is available on the AdminUI server node.',
      annotations: { readOnlyHint: true },
    },
    async () => {
      try {
        const { error, ...r } = await api.call('getSystemSettings')
        return jsonResult(r)
      } catch (e) {
        return errorResult('Error reading system settings', e)
      }
    }
  )

  server.registerTool(
    'update_system_settings',
    {
      title: 'Update System Settings',
      description:
        'Change global process management settings. Only the given fields change.',
      inputSchema: z.object({
        autoManageServices: z.boolean().optional(),
        autoRestartOnConnectionChange: z.boolean().optional(),
      }),
      annotations: { readOnlyHint: false, destructiveHint: false, idempotentHint: true },
    },
    async (args) => {
      try {
        const r = await api.call('updateSystemSettings', args)
        return jsonResult({
          autoManageServices: r?.settings?.autoManageServices,
          autoRestartOnConnectionChange: r?.settings?.autoRestartOnConnectionChange,
        })
      } catch (e) {
        return errorResult('Error updating system settings', e)
      }
    }
  )

  server.registerTool(
    'restart_all_protocol_drivers',
    {
      title: 'Restart All Protocol Drivers',
      description:
        'Restart every protocol driver service on the AdminUI server node (runs the ' +
        'installation restart_protocols script). Data acquisition and commands are interrupted ' +
        'briefly on all connections. Prefer control_driver_process for a single driver.',
      annotations: { readOnlyHint: false, destructiveHint: true },
    },
    async () => {
      try {
        await api.call('restartProtocols')
        return jsonResult({ started: 'restart of all protocol drivers' })
      } catch (e) {
        return errorResult('Error restarting protocol drivers', e)
      }
    }
  )

  server.registerTool(
    'restart_all_processes',
    {
      title: 'Restart All Processes',
      description:
        'Restart all JSON-SCADA services on the AdminUI server node, including the realtime ' +
        'data server, data processors and drivers (runs the installation restart script). ' +
        'The whole system is unavailable for a while and this MCP admin connection drops.',
      annotations: { readOnlyHint: false, destructiveHint: true },
    },
    async () => {
      try {
        await api.call('restartProcesses')
        return jsonResult({ started: 'restart of all JSON-SCADA processes' })
      } catch (e) {
        return errorResult('Error restarting processes', e)
      }
    }
  )
}
