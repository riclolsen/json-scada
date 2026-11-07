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
  parseDateArg,
  textResult,
} from './util.js'

export function registerHistoryTools(server: McpServer, mgr: ConnectionManager) {
  server.registerTool(
    'get_history',
    {
      title: 'Get History',
      annotations: { readOnlyHint: true },
      description:
        'Get historical values for a point from the hist collection, newest first',
      inputSchema: z.object({
        tag: z.string().describe('The tag name of the point'),
        from: z
          .string()
          .optional()
          .describe('Start time, ISO 8601 (default: 24 hours ago)'),
        to: z.string().optional().describe('End time, ISO 8601 (default: now)'),
        limit: z.number().int().min(1).max(1000).optional().default(100),
      }),
    },
    async ({ tag, from, to, limit }: any) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()

      try {
        const fromDate =
          parseDateArg(from, 'from') || new Date(Date.now() - 24 * 3600 * 1000)
        const toDate = parseDateArg(to, 'to') || new Date()

        const results = await mgr
          .getHistCollection()
          .find({ tag, timeTag: { $gte: fromDate, $lte: toDate } })
          .project({ _id: 0, tag: 1, value: 1, valueString: 1, timeTag: 1, timeTagAtSource: 1, invalid: 1 })
          .sort({ timeTag: -1 })
          .limit(limit)
          .toArray()

        return textResult(
          `History for '${tag}' from ${fromDate.toISOString()} to ${toDate.toISOString()} ` +
            `(${results.length} entries, newest first):\n` +
            JSON.stringify(results, null, 2)
        )
      } catch (e) {
        return errorResult('Error retrieving history', e)
      }
    }
  )

  server.registerTool(
    'get_soe_events',
    {
      title: 'Get SOE Events',
      annotations: { readOnlyHint: true },
      description:
        'Get Sequence of Events (SOE) records for digital points, newest first',
      inputSchema: z.object({
        filter: z
          .string()
          .optional()
          .describe(
            'Case-insensitive regex matched against tag, description or eventText'
          ),
        group1: z.string().optional().describe('Filter by group1 (station)'),
        from: z
          .string()
          .optional()
          .describe('Start time, ISO 8601 (default: 24 hours ago)'),
        to: z.string().optional().describe('End time, ISO 8601 (default: now)'),
        limit: z.number().int().min(1).max(500).optional().default(50),
      }),
    },
    async ({ filter, group1, from, to, limit }: any) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()

      try {
        const fromDate =
          parseDateArg(from, 'from') || new Date(Date.now() - 24 * 3600 * 1000)
        const toDate = parseDateArg(to, 'to') || new Date()

        const query: any = { timeTag: { $gte: fromDate, $lte: toDate } }
        if (filter) {
          try {
            new RegExp(filter)
          } catch (e) {
            return errorResult(`Invalid regex filter '${filter}'`, e)
          }
          query.$or = [
            { tag: { $regex: filter, $options: 'i' } },
            { description: { $regex: filter, $options: 'i' } },
            { eventText: { $regex: filter, $options: 'i' } },
          ]
        }
        if (group1 !== undefined) query.group1 = group1

        const results = await mgr
          .getSoeDataCollection()
          .find(query)
          .project({
            _id: 0,
            tag: 1,
            description: 1,
            eventText: 1,
            group1: 1,
            invalid: 1,
            priority: 1,
            timeTag: 1,
            timeTagAtSource: 1,
            timeTagAtSourceOk: 1,
            ack: 1,
          })
          .sort({ timeTag: -1 })
          .limit(limit)
          .toArray()

        return textResult(
          `SOE events from ${fromDate.toISOString()} to ${toDate.toISOString()} ` +
            `(${results.length} entries, newest first):\n` +
            JSON.stringify(results, null, 2)
        )
      } catch (e) {
        return errorResult('Error retrieving SOE events', e)
      }
    }
  )
}
