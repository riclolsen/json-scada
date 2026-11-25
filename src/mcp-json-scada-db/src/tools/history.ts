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
import { ObjectId, type Filter } from 'mongodb'
import type { McpServer } from '@modelcontextprotocol/server'
import type { ISoeData } from '../jsonscada/types.js'
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
        'Read soeData transitions and events, newest by arrival time. See json-scada://alarms-and-soe/guide for acknowledgement, quality and timestamps.',
      inputSchema: z.object({
        filter: z
          .string().max(200)
          .optional()
          .describe(
            'Case-insensitive regex matched against tag, description or eventText'
          ),
        group1: z.string().optional().describe('Filter by group1 (station)'),
        id: z.string().regex(/^[0-9a-f]{24}$/i).optional().describe('Event ObjectId lookup without a default date window'),
        tag: z.string().min(1).max(200).optional(),
        pointKey: z.number().finite().optional(),
        ack: z.union([z.literal(0), z.literal(1), z.literal(2)]).optional()
          .describe('0 unacknowledged, 1 acknowledged, 2 eliminated from operator lists'),
        invalid: z.boolean().optional(),
        from: z
          .string()
          .optional()
          .describe('Start time, ISO 8601 (default: 24 hours ago)'),
        to: z.string().optional().describe('End time, ISO 8601 (default: now)'),
        limit: z.number().int().min(1).max(500).optional().default(50),
      }),
    },
    async ({ filter, group1, from, to, limit, id, tag, pointKey, ack, invalid }) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()

      try {
        const toDate = parseDateArg(to, 'to') ?? (id ? undefined : new Date())
        const fromDate = parseDateArg(from, 'from') ??
          (!id && toDate ? new Date(toDate.getTime() - 24 * 3600 * 1000) : undefined)
        if (fromDate && toDate && fromDate > toDate)
          return textResult('from must be at or before to.', true)

        const query: Filter<ISoeData> = {}
        if (id) query._id = new ObjectId(id)
        if (fromDate || toDate)
          query.timeTag = { ...(fromDate && { $gte: fromDate }), ...(toDate && { $lte: toDate }) }
        if (tag !== undefined) query.tag = tag
        if (pointKey !== undefined) query.pointKey = pointKey
        if (ack !== undefined) query.ack = ack
        if (invalid !== undefined) query.invalid = invalid
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
          .find(query, { maxTimeMS: 5000 })
          .project<ISoeData>({
            _id: 1,
            pointKey: 1,
            value: 1,
            valueString: 1,
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
          .sort({ timeTag: -1, _id: -1 })
          .limit(limit + 1)
          .toArray()

        const events = results.slice(0, limit).map((event) => ({
          ...event,
          interpretation: {
            acknowledgement: event.ack === 0 ? 'unacknowledged' :
              event.ack === 1 ? 'acknowledged' :
              event.ack === 2 ? 'eliminated from operator lists' : 'unknown',
            quality: event.invalid === true ? 'invalid' : event.invalid === false ? 'valid' : 'unknown',
            sourceTimeUsable: event.timeTagAtSourceOk === true && event.timeTagAtSource instanceof Date,
          },
        }))
        return textResult(
          `SOE events from ${fromDate?.toISOString() ?? 'any start'} to ${toDate?.toISOString() ?? 'any end'} ` +
            `(${events.length} entries, newest first${results.length > limit ? ', truncated' : ''}):\n` +
            JSON.stringify(events, null, 2)
        )
      } catch (e) {
        return errorResult('Error retrieving SOE events', e)
      }
    }
  )
}
