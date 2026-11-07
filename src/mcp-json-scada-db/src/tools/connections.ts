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
import { PROTOCOL_DRIVER_NAMES } from '../jsonscada/drivers.js'
import { findDriverInstance } from './drivers.js'
import {
  errorResult,
  jsonResult,
  notConnectedResult,
  redactSensitive,
  textResult,
} from './util.js'

// Identity and driver-maintained fields that settings may not overwrite
const PROTECTED_FIELDS = ['_id', '__v', 'protocolConnectionNumber', 'stats']

const settingsSchema = z
  .record(z.string(), z.any())
  .optional()
  .describe(
    'Driver-specific connection fields (e.g. ipAddresses, topics, endpointURLs, giInterval, ' +
      'useSecurity, ...). See get_protocol_driver_documentation and ' +
      'get_protocol_connection_template for the available fields.'
  )

const commonFields = {
  name: z.string().optional().describe('Unique connection name'),
  description: z.string().optional(),
  enabled: z.boolean().optional(),
  commandsEnabled: z.boolean().optional(),
}

function cleanSettings(settings: Record<string, any> | undefined) {
  const clean: Record<string, any> = { ...(settings || {}) }
  for (const f of [...PROTECTED_FIELDS, 'protocolDriver', 'protocolDriverInstanceNumber'])
    delete clean[f]
  return clean
}

function stripProtected(doc: Record<string, any>) {
  const { __v, stats, ...rest } = doc
  return rest
}

async function findConnection(api: AdminApiClient, connNumber: number) {
  const list: any[] = await api.call('listProtocolConnections')
  return list.find(
    (c) => Math.trunc(Number(c.protocolConnectionNumber)) === connNumber
  )
}

function restartNote(r: any) {
  if (r?.restartScheduled)
    return 'Driver restart scheduled to apply the change.'
  if (r?.restartPending)
    return 'Driver restart pending: automatic restart is off, use control_driver_process to restart the driver.'
  return undefined
}

// Read-only connection detail; works without the admin API
export function registerConnectionReadTools(
  server: McpServer,
  mgr: ConnectionManager
) {
  server.registerTool(
    'get_protocol_connection',
    {
      title: 'Get Protocol Connection',
      description:
        'Get the full configuration of a protocol connection (secrets redacted), including ' +
        'driver-specific fields and statistics.',
      inputSchema: z.object({
        protocolConnectionNumber: z.number().int(),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ protocolConnectionNumber }) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()
      try {
        const conn = await mgr
          .getProtocolConnectionsCollection()
          .findOne({ protocolConnectionNumber } as any)
        if (!conn)
          return textResult(
            `Protocol connection ${protocolConnectionNumber} not found.`,
            true
          )
        return jsonResult(redactSensitive(conn))
      } catch (e) {
        return errorResult('Error reading protocol connection', e)
      }
    }
  )
}

export function registerConnectionTools(server: McpServer, api: AdminApiClient) {
  server.registerTool(
    'get_protocol_connection_template',
    {
      title: 'Get Protocol Connection Template',
      description:
        'Return a new protocol connection document with every field at its default value and ' +
        'the next free protocolConnectionNumber. Nothing is created.',
      annotations: { readOnlyHint: true },
    },
    async () => {
      try {
        const r = await api.call('getProtocolConnectionModel')
        const { _id, ...tpl } = r?.protocolConnection || {}
        return jsonResult(tpl)
      } catch (e) {
        return errorResult('Error getting connection template', e)
      }
    }
  )

  server.registerTool(
    'create_protocol_connection',
    {
      title: 'Create Protocol Connection',
      description:
        'Create a protocol connection served by a driver instance. The system assigns the ' +
        'protocolConnectionNumber (used by tags in protocolSourceConnectionNumber) and fills ' +
        'protocol defaults such as bind addresses. The driver is restarted automatically when ' +
        'auto-restart on connection change is on.',
      inputSchema: z.object({
        protocolDriver: z.enum(PROTOCOL_DRIVER_NAMES),
        protocolDriverInstanceNumber: z.number().int().min(1).default(1),
        ...commonFields,
        name: z.string().describe('Unique connection name'),
        settings: settingsSchema,
      }),
      annotations: { readOnlyHint: false, destructiveHint: false },
    },
    async ({ protocolDriver, protocolDriverInstanceNumber, settings, ...common }) => {
      try {
        const list: any[] = await api.call('listProtocolConnections')
        if (list.some((c) => c.name === common.name))
          return textResult(
            `A protocol connection named '${common.name}' already exists.`,
            true
          )
        const { _id } = await api.call('createProtocolConnection')
        const created = (
          (await api.call('listProtocolConnections')) as any[]
        ).find((c) => c._id === _id)
        const doc = {
          ...stripProtected(created || {}),
          ...cleanSettings(settings),
          ...common,
          _id,
          protocolDriver,
          protocolDriverInstanceNumber,
        }
        let r: any
        try {
          r = await api.call('updateProtocolConnection', doc)
        } catch (e) {
          // do not leave an empty placeholder connection behind
          await api.call('deleteProtocolConnection', { _id }).catch(() => {})
          throw e
        }
        const instance = await findDriverInstance(
          api,
          protocolDriver,
          protocolDriverInstanceNumber
        )
        return jsonResult({
          created: {
            protocolConnectionNumber: created?.protocolConnectionNumber,
            name: common.name,
            protocolDriver,
            protocolDriverInstanceNumber,
          },
          note: restartNote(r),
          warning: instance
            ? undefined
            : `No driver instance ${protocolDriver} #${protocolDriverInstanceNumber} exists; create it with create_driver_instance.`,
        })
      } catch (e) {
        return errorResult('Error creating protocol connection', e)
      }
    }
  )

  server.registerTool(
    'update_protocol_connection',
    {
      title: 'Update Protocol Connection',
      description:
        'Change fields of an existing protocol connection. Only the given fields change. ' +
        'Changing protocolDriverInstanceNumber moves the connection to another driver instance. ' +
        'The driver is restarted automatically when auto-restart on connection change is on.',
      inputSchema: z.object({
        protocolConnectionNumber: z.number().int(),
        protocolDriverInstanceNumber: z.number().int().min(1).optional(),
        ...commonFields,
        settings: settingsSchema,
      }),
      annotations: { readOnlyHint: false, destructiveHint: false, idempotentHint: true },
    },
    async ({ protocolConnectionNumber, settings, ...common }) => {
      try {
        const conn = await findConnection(api, protocolConnectionNumber)
        if (!conn)
          return textResult(
            `Protocol connection ${protocolConnectionNumber} not found.`,
            true
          )
        const defined = Object.fromEntries(
          Object.entries(common).filter(([, v]) => v !== undefined)
        )
        const r = await api.call('updateProtocolConnection', {
          ...stripProtected(conn),
          ...cleanSettings(settings),
          ...defined,
        })
        return jsonResult({
          updated: { protocolConnectionNumber, name: defined['name'] ?? conn.name },
          note: restartNote(r),
        })
      } catch (e) {
        return errorResult('Error updating protocol connection', e)
      }
    }
  )

  server.registerTool(
    'delete_protocol_connection',
    {
      title: 'Delete Protocol Connection',
      description:
        'Delete a protocol connection. With deleteTags=true, all tags whose ' +
        'protocolSourceConnectionNumber is this connection are deleted too (irreversible).',
      inputSchema: z.object({
        protocolConnectionNumber: z.number().int(),
        deleteTags: z.boolean().optional().default(false),
      }),
      annotations: { readOnlyHint: false, destructiveHint: true },
    },
    async ({ protocolConnectionNumber, deleteTags }) => {
      try {
        const conn = await findConnection(api, protocolConnectionNumber)
        if (!conn)
          return textResult(
            `Protocol connection ${protocolConnectionNumber} not found.`,
            true
          )
        const r = await api.call('deleteProtocolConnection', {
          _id: conn._id,
          protocolConnectionNumber: conn.protocolConnectionNumber,
          deleteTags,
        })
        return jsonResult({
          deleted: { protocolConnectionNumber, name: conn.name },
          tagsDeleted: deleteTags,
          note: restartNote(r),
        })
      } catch (e) {
        return errorResult('Error deleting protocol connection', e)
      }
    }
  )
}
