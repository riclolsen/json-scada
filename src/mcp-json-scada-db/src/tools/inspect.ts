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
import { ConnectionManager } from '../jsonscada/connection-manager.js'
import {
  errorResult,
  notConnectedResult,
  redactSensitiveArray,
  textResult,
} from './util.js'

export function registerInspectTools(server: McpServer, mgr: ConnectionManager) {
  server.registerTool(
    'describe_collection',
    {
      title: 'Describe Collection',
      annotations: { readOnlyHint: true },
      description:
        'Get estimated document count and sample documents from a collection',
      inputSchema: z.object({
        collection: z.string().describe('The name of the collection to describe'),
      }),
    },
    async ({ collection }: any) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()

      try {
        // estimatedDocumentCount uses collection metadata, avoiding a full
        // scan on huge collections like hist
        const count = await mgr.db.collection(collection).estimatedDocumentCount()
        const sample = await mgr.db.collection(collection).find().limit(3).toArray()

        return textResult(
          `Collection: ${collection}\n` +
            `Estimated Documents: ${count}\n` +
            `Sample Structure (up to 3 docs):\n${JSON.stringify(redactSensitiveArray(sample), null, 2)}`
        )
      } catch (e) {
        return errorResult('Error describing collection', e)
      }
    }
  )

  server.registerTool(
    'list_database_info',
    {
      title: 'List Database Info',
      annotations: { readOnlyHint: true },
      description: 'List all collections and their estimated document counts',
    },
    async () => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()

      try {
        const collections = await mgr.db.listCollections().toArray()
        const results = []

        for (const col of collections) {
          const count = await mgr.db
            .collection(col.name)
            .estimatedDocumentCount()
          results.push({ name: col.name, count })
        }

        return textResult(
          'Collections in database (estimated counts):\n' +
            results.map((c) => `- ${c.name}: ${c.count} documents`).join('\n')
        )
      } catch (e) {
        return errorResult('Error listing database info', e)
      }
    }
  )

  server.registerTool(
    'get_collection_fields',
    {
      title: 'Get Collection Fields',
      annotations: { readOnlyHint: true },
      description: 'Inspect a collection to find all unique top-level fields',
      inputSchema: z.object({
        collection: z.string().describe('The name of the collection to inspect'),
      }),
    },
    async ({ collection }: any) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()

      try {
        const sample = await mgr.db.collection(collection).find().limit(100).toArray()
        const fields = new Set<string>()

        sample.forEach((doc) => {
          Object.keys(doc).forEach((key) => fields.add(key))
        })

        return textResult(
          `Unique top-level fields found in a sample of ${sample.length} documents from '${collection}':\n` +
            Array.from(fields).sort().join(', ')
        )
      } catch (e) {
        return errorResult('Error inspecting fields', e)
      }
    }
  )

  server.registerTool(
    'get_database_schema',
    {
      title: 'Get Database Schema',
      annotations: { readOnlyHint: true },
      description: 'Get the known schema for the JSON-SCADA database',
    },
    async () => {
      const schema = {
        realtimeData: {
          description: 'Current state of all points (tags)',
          keyFields: {
            _id: 'numeric pointKey (unique)',
            tag: 'string key (unique)',
            type: 'digital, analog, string, json',
            origin: 'supervised, calculated, manual, command',
            value: 'current numeric value',
            valueString: 'current string value',
            timeTag: 'last update time',
            invalid: 'true when value is old or not trusted',
            alarmed: 'true when the point is alarmed',
            unit: 'unit of measurement',
            group1: 'main grouping (e.g. station)',
            group2: 'secondary grouping (e.g. bay)',
            group3: 'lowest level grouping (e.g. equipment)',
          },
        },
        hist: {
          description: 'Historical data for points',
          keyFields: {
            timeTag: 'timestamp (server time)',
            timeTagAtSource: 'timestamp from the source device (may be null)',
            tag: 'point tag',
            value: 'recorded value',
            invalid: 'quality flag',
          },
        },
        soeData: {
          description: 'Sequence of Events (SOE) for digital points',
          keyFields: {
            timeTag: 'arrival timestamp',
            timeTagAtSource: 'event timestamp from the source device',
            tag: 'point tag',
            description: 'point description',
            eventText: 'text describing the event',
            ack: '0=not acknowledged, 1=acknowledged, 2=eliminated',
          },
        },
        commandsQueue: {
          description: 'Queue for commands to be dispatched',
          keyFields: {
            tag: 'point tag',
            value: 'command numeric value',
            timeTag: 'insertion time',
            delivered: 'boolean flag for delivery status',
            ack: 'boolean protocol acknowledge (when available)',
            originatorUserName: 'user who sent the command',
          },
        },
        processInstances: {
          description: 'Status and config of system processes',
          keyFields: {
            processName: 'e.g. CS_DATA_PROCESSOR, CALCULATIONS',
            enabled: 'instance enabled flag',
            activeNodeName: 'node currently running the process',
            activeNodeKeepAliveTimeTag: 'keep-alive timestamp',
          },
        },
        protocolDriverInstances: {
          description: 'Status and config of protocol driver instances',
          keyFields: {
            protocolDriver: 'driver name, e.g. IEC60870-5-104',
            protocolDriverInstanceNumber: 'instance number',
            enabled: 'instance enabled flag',
            activeNodeName: 'node currently running the driver',
          },
        },
        protocolConnections: {
          description: 'Configuration of individual communication links',
          keyFields: {
            protocolDriver: 'driver name',
            protocolConnectionNumber: 'unique connection number',
            name: 'connection name',
            enabled: 'connection enabled flag',
            commandsEnabled: 'commands allowed flag',
            stats: 'runtime statistics (driver-dependent)',
          },
        },
        userActions: {
          description: 'Audit log of actions performed by users',
          keyFields: {
            username: 'user name',
            action: 'action name, e.g. Command',
            tag: 'related point tag',
            timeTag: 'action timestamp',
          },
        },
      }

      return textResult(
        'JSON-SCADA Known Database Schema:\n' + JSON.stringify(schema, null, 2)
      )
    }
  )
}
