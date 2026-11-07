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
import { BSON } from 'mongodb'
import type { McpServer } from '@modelcontextprotocol/server'
import { ConnectionManager } from '../jsonscada/connection-manager.js'
import {
  errorResult,
  notConnectedResult,
  redactSensitiveArray,
  textResult,
} from './util.js'

function parseEjson(text: string, argName: string): any {
  try {
    return BSON.EJSON.parse(text, { relaxed: true })
  } catch (e) {
    throw new Error(
      `Invalid ${argName} JSON: ${e instanceof Error ? e.message : e}`
    )
  }
}

export function registerCollectionsTools(
  server: McpServer,
  mgr: ConnectionManager
) {
  server.registerTool(
    'list_collections',
    {
      title: 'List Collections',
      annotations: { readOnlyHint: true },
      description: 'List all collections in the database',
    },
    async () => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()
      try {
        const collections = await mgr.db.listCollections().toArray()
        return textResult(
          JSON.stringify(
            collections.map((c) => c.name),
            null,
            2
          )
        )
      } catch (e) {
        return errorResult('Error listing collections', e)
      }
    }
  )

  server.registerTool(
    'query_collection',
    {
      title: 'Query Collection',
      annotations: { readOnlyHint: true },
      description:
        'Run a read-only query on any collection. The query, projection and sort are ' +
        'MongoDB Extended JSON strings, so operators like {"$gte": {"$date": "..."}} are supported.',
      inputSchema: z.object({
        collection: z.string().describe('The collection name to query'),
        query: z
          .string()
          .optional()
          .describe('Extended JSON string with the MongoDB filter'),
        projection: z
          .string()
          .optional()
          .describe('Extended JSON string with the fields projection'),
        sort: z
          .string()
          .optional()
          .describe('Extended JSON string with the sort specification'),
        limit: z.number().int().min(1).max(100).optional().default(5),
        offset: z.number().int().min(0).optional().default(0),
      }),
    },
    async ({ collection, query, projection, sort, limit, offset }: any) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()

      try {
        const filter = query ? parseEjson(query, 'query') : {}
        let cursor = mgr.db.collection(collection).find(filter)
        if (projection) cursor = cursor.project(parseEjson(projection, 'projection'))
        if (sort) cursor = cursor.sort(parseEjson(sort, 'sort'))
        const results = await cursor.skip(offset).limit(limit).toArray()

        return textResult(JSON.stringify(redactSensitiveArray(results), null, 2))
      } catch (e) {
        return errorResult('Error querying collection', e)
      }
    }
  )
}
