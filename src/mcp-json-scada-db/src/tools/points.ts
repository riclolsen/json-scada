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
import { errorResult, notConnectedResult, textResult } from './util.js'

// Compact projection to avoid flooding the model context with full documents
const SEARCH_PROJECTION = {
  _id: 1,
  tag: 1,
  description: 1,
  group1: 1,
  group2: 1,
  group3: 1,
  type: 1,
  origin: 1,
  value: 1,
  valueString: 1,
  unit: 1,
  invalid: 1,
  alarmed: 1,
  alarmDisabled: 1,
  commandBlocked: 1,
  timeTag: 1,
  timeTagAtSource: 1,
}

export function registerPointsTools(server: McpServer, mgr: ConnectionManager) {
  server.registerTool(
    'search_points',
    {
      title: 'Search Points',
      annotations: { readOnlyHint: true },
      description:
        'Search for database points (tags) by tag/description regex and other filters. ' +
        'Returns a compact summary of each matching point plus the total match count. ' +
        'Use get_point to retrieve the full document of a specific point.',
      inputSchema: z.object({
        filter: z
          .string()
          .optional()
          .describe('Case-insensitive regex matched against tag or description'),
        group1: z.string().optional().describe('Filter by group1 (station)'),
        group2: z.string().optional().describe('Filter by group2 (bay)'),
        type: z
          .enum(['digital', 'analog', 'string', 'json'])
          .optional()
          .describe('Filter by data type'),
        origin: z
          .enum(['supervised', 'calculated', 'manual', 'command'])
          .optional()
          .describe('Filter by origin'),
        alarmed: z.boolean().optional().describe('Filter by alarmed state'),
        invalid: z.boolean().optional().describe('Filter by invalid quality'),
        limit: z.number().int().min(1).max(100).optional().default(10),
        offset: z.number().int().min(0).optional().default(0),
      }),
    },
    async ({ filter, group1, group2, type, origin, alarmed, invalid, limit, offset }: any) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()

      const query: any = {}
      if (filter) {
        try {
          new RegExp(filter)
        } catch (e) {
          return errorResult(`Invalid regex filter '${filter}'`, e)
        }
        query.$or = [
          { tag: { $regex: filter, $options: 'i' } },
          { description: { $regex: filter, $options: 'i' } },
        ]
      }
      if (group1 !== undefined) query.group1 = group1
      if (group2 !== undefined) query.group2 = group2
      if (type !== undefined) query.type = type
      if (origin !== undefined) query.origin = origin
      if (alarmed !== undefined) query.alarmed = alarmed
      if (invalid !== undefined) query.invalid = invalid

      try {
        const coll = mgr.getRealtimeDataCollection()
        const total = await coll.countDocuments(query)
        const points = await coll
          .find(query)
          .project(SEARCH_PROJECTION)
          .sort({ tag: 1 })
          .skip(offset)
          .limit(limit)
          .toArray()

        return textResult(
          `Total matches: ${total} (showing ${points.length} from offset ${offset})\n` +
            JSON.stringify(points, null, 2)
        )
      } catch (e) {
        return errorResult('Error searching points', e)
      }
    }
  )

  server.registerTool(
    'get_point',
    {
      title: 'Get Point',
      annotations: { readOnlyHint: true },
      description:
        'Get the full document of a specific point by tag name or numeric point key',
      inputSchema: z.object({
        tag: z.string().optional().describe('The tag name of the point'),
        pointKey: z
          .number()
          .optional()
          .describe('The numeric point key (_id) of the point'),
      }),
    },
    async ({ tag, pointKey }: any) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()
      if (tag === undefined && pointKey === undefined)
        return textResult('Provide either tag or pointKey.', true)

      try {
        const query = tag !== undefined ? { tag } : { _id: pointKey }
        const point = await mgr.getRealtimeDataCollection().findOne(query as any)

        if (!point)
          return textResult(
            `Point '${tag !== undefined ? tag : pointKey}' not found.`,
            true
          )

        return textResult(JSON.stringify(point, null, 2))
      } catch (e) {
        return errorResult('Error retrieving point', e)
      }
    }
  )
}
