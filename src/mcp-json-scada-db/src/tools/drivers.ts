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

import fs from 'node:fs/promises'
import path from 'node:path'
import { z } from 'zod'
import type { McpServer } from '@modelcontextprotocol/server'
import { AdminApiClient } from '../jsonscada/admin-api.js'
import {
  PROTOCOL_DRIVERS,
  PROTOCOL_DRIVER_NAMES,
  srcDir,
} from '../jsonscada/drivers.js'
import { errorResult, jsonResult, textResult } from './util.js'

// Runtime/status fields maintained by the running driver (redundancy keep-alive,
// stats). Never sent back on updates: a stale keep-alive could cause a failover.
const INSTANCE_RUNTIME_FIELDS = [
  '__v',
  'activeNodeName',
  'activeNodeKeepAliveTimeTag',
  'softwareVersion',
  'stats',
]

function withoutRuntimeFields(doc: Record<string, any>) {
  const clean = { ...doc }
  for (const f of INSTANCE_RUNTIME_FIELDS) delete clean[f]
  return clean
}

const instanceKey = {
  protocolDriver: z.enum(PROTOCOL_DRIVER_NAMES).describe('Protocol driver name'),
  protocolDriverInstanceNumber: z
    .number()
    .int()
    .min(1)
    .describe('Driver instance number'),
}

const instanceSettings = z.object({
  enabled: z.boolean().optional(),
  logLevel: z
    .number()
    .int()
    .min(0)
    .max(3)
    .optional()
    .describe('0=minimum, 1=basic, 2=detailed, 3=debug'),
  nodeNames: z
    .array(z.string())
    .optional()
    .describe('Computer node names allowed to run this instance (redundancy)'),
  keepProtocolRunningWhileInactive: z.boolean().optional(),
  processManagement: z
    .object({
      managed: z
        .boolean()
        .optional()
        .describe('Create/control an OS service (NSSM/supervisord) for it'),
      startMode: z.enum(['auto', 'manual']).optional(),
      autoRestartOnConfigChange: z.boolean().optional(),
    })
    .optional(),
})

export async function findDriverInstance(
  api: AdminApiClient,
  protocolDriver: string,
  instanceNumber: number
): Promise<any | undefined> {
  const list: any[] = await api.call('listProtocolDriverInstances')
  return list.find(
    (d) =>
      d.protocolDriver === protocolDriver &&
      Math.trunc(Number(d.protocolDriverInstanceNumber)) === instanceNumber
  )
}

function notFound(protocolDriver: string, n: number) {
  return textResult(
    `Driver instance ${protocolDriver} #${n} not found. Use list_driver_processes to see existing instances.`,
    true
  )
}

// Read-only driver documentation; works without the admin API
export function registerDriverDocTools(server: McpServer) {
  server.registerTool(
    'list_protocol_drivers',
    {
      title: 'List Protocol Drivers',
      description:
        'List the protocol driver names supported by JSON-SCADA (values for protocolDriver ' +
        'in driver instances and connections), with client/server role.',
      annotations: { readOnlyHint: true },
    },
    async () =>
      jsonResult(
        Object.entries(PROTOCOL_DRIVERS).map(([name, d]) => ({
          protocolDriver: name,
          role: d.role,
          documentation: !!d.doc,
        }))
      )
  )

  server.registerTool(
    'get_protocol_driver_documentation',
    {
      title: 'Get Protocol Driver Documentation',
      description:
        'Return the README of a protocol driver, documenting its connection settings ' +
        '(protocolConnections fields), tag addressing (protocolSource* fields) and command handling. ' +
        'Read this before creating or changing connections or tags for a driver.',
      inputSchema: z.object({
        protocolDriver: z.enum(PROTOCOL_DRIVER_NAMES),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ protocolDriver }) => {
      const doc = PROTOCOL_DRIVERS[protocolDriver]?.doc
      if (!doc)
        return textResult(
          `No documentation is bundled for ${protocolDriver}.`,
          true
        )
      try {
        const text = await fs.readFile(path.join(srcDir(), doc), 'utf8')
        return textResult(text)
      } catch (e) {
        return errorResult(`Cannot read documentation ${doc}`, e)
      }
    }
  )
}

export function registerDriverTools(server: McpServer, api: AdminApiClient) {
  server.registerTool(
    'list_driver_processes',
    {
      title: 'List Driver Processes',
      description:
        'List all protocol driver instances with their configuration and OS service (process) ' +
        'state: installed, state (running/stopped/...), startMode, manageable, restartPending.',
      annotations: { readOnlyHint: true },
    },
    async () => {
      try {
        const [instances, status] = await Promise.all([
          api.call('listProtocolDriverInstances'),
          api.call('listDriverProcessStatus'),
        ])
        const statuses: any[] = status?.statuses || []
        return jsonResult({
          processManagementEnabled: status?.enabled,
          localNode: status?.localNode,
          instances: (instances as any[]).map((inst) => {
            const { _id, __v, ...rest } = inst
            const st = statuses.find(
              (s) =>
                s.protocolDriver === inst.protocolDriver &&
                Number(s.protocolDriverInstanceNumber) ===
                  Number(inst.protocolDriverInstanceNumber)
            )
            return { ...rest, process: st || null }
          }),
        })
      } catch (e) {
        return errorResult('Error listing driver processes', e)
      }
    }
  )

  server.registerTool(
    'create_driver_instance',
    {
      title: 'Create Driver Instance',
      description:
        'Create a protocol driver instance (one OS process of a driver). When automatic ' +
        'service management is on, the OS service is created too. Connections are then ' +
        'attached with create_protocol_connection.',
      inputSchema: z.object({ ...instanceKey, ...instanceSettings.shape }),
      annotations: { readOnlyHint: false, destructiveHint: false },
    },
    async ({ protocolDriver, protocolDriverInstanceNumber, ...settings }) => {
      try {
        if (
          await findDriverInstance(api, protocolDriver, protocolDriverInstanceNumber)
        )
          return textResult(
            `Driver instance ${protocolDriver} #${protocolDriverInstanceNumber} already exists.`,
            true
          )
        const { _id } = await api.call('createProtocolDriverInstance')
        const created = (
          (await api.call('listProtocolDriverInstances')) as any[]
        ).find((d) => d._id === _id)
        const doc = {
          ...withoutRuntimeFields(created || {}),
          ...settings,
          processManagement: {
            ...(created?.processManagement || {}),
            ...(settings.processManagement || {}),
          },
          _id,
          protocolDriver,
          protocolDriverInstanceNumber,
        }
        try {
          const r = await api.call('updateProtocolDriverInstance', doc)
          return jsonResult({
            created: { protocolDriver, protocolDriverInstanceNumber },
            processWarning: r?.processWarning,
          })
        } catch (e) {
          // do not leave an UNDEFINED placeholder instance behind
          await api.call('deleteProtocolDriverInstance', { _id }).catch(() => {})
          throw e
        }
      } catch (e) {
        return errorResult('Error creating driver instance', e)
      }
    }
  )

  server.registerTool(
    'update_driver_instance',
    {
      title: 'Update Driver Instance',
      description:
        'Change settings of an existing protocol driver instance (enabled, logLevel, nodeNames, ' +
        'process management). Only the given fields change. The OS service is reconciled ' +
        'automatically when automatic service management is on.',
      inputSchema: z.object({ ...instanceKey, ...instanceSettings.shape }),
      annotations: { readOnlyHint: false, destructiveHint: false, idempotentHint: true },
    },
    async ({ protocolDriver, protocolDriverInstanceNumber, ...changes }) => {
      try {
        const inst = await findDriverInstance(
          api,
          protocolDriver,
          protocolDriverInstanceNumber
        )
        if (!inst) return notFound(protocolDriver, protocolDriverInstanceNumber)
        const doc = {
          ...withoutRuntimeFields(inst),
          ...changes,
          processManagement: {
            ...(inst.processManagement || {}),
            ...(changes.processManagement || {}),
          },
        }
        const r = await api.call('updateProtocolDriverInstance', doc)
        return jsonResult({
          updated: { protocolDriver, protocolDriverInstanceNumber },
          processWarning: r?.processWarning,
        })
      } catch (e) {
        return errorResult('Error updating driver instance', e)
      }
    }
  )

  server.registerTool(
    'delete_driver_instance',
    {
      title: 'Delete Driver Instance',
      description:
        'Delete a protocol driver instance. Its OS service is stopped and removed when ' +
        'automatic service management is on. Connections of the instance are NOT deleted ' +
        '(they stop being served).',
      inputSchema: z.object(instanceKey),
      annotations: { readOnlyHint: false, destructiveHint: true },
    },
    async ({ protocolDriver, protocolDriverInstanceNumber }) => {
      try {
        const inst = await findDriverInstance(
          api,
          protocolDriver,
          protocolDriverInstanceNumber
        )
        if (!inst) return notFound(protocolDriver, protocolDriverInstanceNumber)
        const r = await api.call('deleteProtocolDriverInstance', {
          _id: inst._id,
        })
        return jsonResult({
          deleted: { protocolDriver, protocolDriverInstanceNumber },
          processWarning: r?.processWarning,
        })
      } catch (e) {
        return errorResult('Error deleting driver instance', e)
      }
    }
  )

  server.registerTool(
    'control_driver_process',
    {
      title: 'Control Driver Process',
      description:
        'Start, stop or restart the OS service (process) of a protocol driver instance. ' +
        'Stopping a driver interrupts data acquisition and commands for all its connections.',
      inputSchema: z.object({
        ...instanceKey,
        action: z.enum(['start', 'stop', 'restart']),
      }),
      annotations: { readOnlyHint: false, destructiveHint: true },
    },
    async ({ protocolDriver, protocolDriverInstanceNumber, action }) => {
      const endpoint = {
        start: 'startProtocolDriverInstance',
        stop: 'stopProtocolDriverInstance',
        restart: 'restartProtocolDriverInstance',
      }[action]
      try {
        const r = await api.call(endpoint, {
          protocolDriver,
          protocolDriverInstanceNumber,
        })
        return jsonResult({ action, protocolDriver, protocolDriverInstanceNumber, result: r?.result })
      } catch (e) {
        return errorResult(`Error on ${action} of driver process`, e)
      }
    }
  )

  server.registerTool(
    'sync_driver_services',
    {
      title: 'Sync Driver Services',
      description:
        'Reconcile OS services with the configured driver instances: create missing services ' +
        'and update changed ones. With removeOrphans, also remove services of instances that ' +
        'no longer exist.',
      inputSchema: z.object({
        removeOrphans: z.boolean().optional().default(false),
      }),
      annotations: { readOnlyHint: false, destructiveHint: true, idempotentHint: true },
    },
    async ({ removeOrphans }) => {
      try {
        const r = await api.call('syncDriverServices', { removeOrphans })
        return jsonResult(r?.result ?? r)
      } catch (e) {
        return errorResult('Error syncing driver services', e)
      }
    }
  )
}
