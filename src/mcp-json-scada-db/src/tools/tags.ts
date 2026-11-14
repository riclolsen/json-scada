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
import { errorResult, jsonResult, textResult } from './util.js'
import { COMM_OWNED_FIELDS, RESERVED_NAME_MESSAGE, isSystemTagName } from '../jsonscada/system-tags.js'

// Live value/state fields maintained by drivers and data processors; tag
// configuration tools never write them (use send_command to act on the field).
const RUNTIME_FIELDS = [
  '_id',
  '__v',
  'value',
  'valueString',
  'valueJson',
  'timeTag',
  'timeTagAlarm',
  'timeTagAlertState',
  'timeTagAtSource',
  'timeTagAtSourceOk',
  'sourceDataUpdate',
  'alarmed',
  'alarmState',
  'alerted',
  'alertState',
  'invalid',
  'overflow',
  'transient',
  'substituted',
  'frozen',
  'updatesCnt',
  'historianLastValue',
]

const address = z.union([z.string(), z.number()])

const tagFields = {
  type: z.enum(['digital', 'analog', 'string', 'json']).optional(),
  origin: z
    .enum(['supervised', 'calculated', 'manual', 'command'])
    .optional()
    .describe('supervised = acquired by a protocol driver; command = output point'),
  description: z.string().optional(),
  ungroupedDescription: z.string().optional(),
  group1: z.string().optional().describe('Station / area'),
  group2: z.string().optional().describe('Bay / equipment'),
  group3: z.string().optional(),
  unit: z.string().optional(),
  protocolSourceConnectionNumber: z
    .number()
    .int()
    .optional()
    .describe('protocolConnectionNumber of the connection that acquires/commands the tag'),
  protocolSourceCommonAddress: address.optional(),
  protocolSourceObjectAddress: address
    .optional()
    .describe('Driver-specific point address (see get_protocol_driver_documentation)'),
  protocolSourceASDU: address.optional(),
  fields: z
    .record(z.string(), z.any())
    .optional()
    .describe(
      'Other realtimeData configuration fields, e.g. kconv1, kconv2, hiLimit, loLimit, ' +
        'stateTextTrue, stateTextFalse, eventTextTrue, eventTextFalse, isEvent, priority, ' +
        'alarmDisabled, commandOfSupervised, supervisedOfCommand, protocolSourceCommandDuration, ' +
        'protocolSourceCommandUseSBO, protocolDestinations, historianDeadBand, formula, parcels'
    ),
}

// numeric-looking addresses are stored as numbers, as the AdminUI does
function normalizeAddresses(doc: Record<string, any>) {
  for (const f of [
    'protocolSourceCommonAddress',
    'protocolSourceObjectAddress',
    'protocolSourceASDU',
  ])
    if (typeof doc[f] === 'string' && /^[-+]?[0-9]+(\.[0-9]+)?$/.test(doc[f]))
      doc[f] = parseFloat(doc[f])
  return doc
}

export function buildDoc(args: Record<string, any>) {
  const { fields, ...named } = args
  const doc: Record<string, any> = { ...(fields || {}) }
  for (const [k, v] of Object.entries(named)) if (v !== undefined) doc[k] = v
  for (const f of RUNTIME_FIELDS) delete doc[f]
  return normalizeAddresses(doc)
}

export async function findTag(api: AdminApiClient, tag: string) {
  const r = await api.call('listTags', {
    filter: { tag },
    page: 1,
    itemsPerPage: 1,
  })
  return (r?.tags || [])[0]
}

export function registerTagTools(server: McpServer, api: AdminApiClient) {
  server.registerTool(
    'create_tag',
    {
      title: 'Create Tag',
      description:
        'Create a tag (point) in realtimeData. The point key (_id) is assigned automatically ' +
        'unless pointKey is given. For acquired points set origin=supervised plus ' +
        'protocolSourceConnectionNumber and the driver-specific protocolSource* address.',
      inputSchema: z.object({
        tag: z.string().min(1).describe('Unique tag name'),
        pointKey: z.number().int().positive().optional(),
        ...tagFields,
        type: tagFields.type.unwrap(),
        origin: tagFields.origin.unwrap().default('supervised'),
      }),
      annotations: { readOnlyHint: false, destructiveHint: false },
    },
    async ({ pointKey, ...args }) => {
      try {
        const tag = args.tag.trim()
        if (isSystemTagName(tag)) return textResult(RESERVED_NAME_MESSAGE, true)
        if (await findTag(api, tag))
          return textResult(`Tag '${tag}' already exists.`, true)
        const doc = { ...buildDoc({ ...args, tag }), ...(pointKey ? { _id: pointKey } : {}) }
        const created = await api.call('createTag', doc)
        return jsonResult({ created: { tag: created?.tag, pointKey: created?._id } })
      } catch (e) {
        return errorResult('Error creating tag', e)
      }
    }
  )

  server.registerTool(
    'update_tag',
    {
      title: 'Update Tag',
      description:
        'Change configuration fields of an existing tag (description, groups, limits, protocol ' +
        'addressing, ...). Only the given fields change. Live value fields cannot be changed here.',
      inputSchema: z.object({
        tag: z.string().describe('Current tag name'),
        newTag: z.string().min(1).optional().describe('Rename the tag'),
        ...tagFields,
      }),
      annotations: { readOnlyHint: false, destructiveHint: false, idempotentHint: true },
    },
    async ({ tag, newTag, ...args }) => {
      try {
        const existing = await findTag(api, tag)
        if (!existing) return textResult(`Tag '${tag}' not found.`, true)
        if (newTag && newTag.trim() !== tag && isSystemTagName(newTag)) return textResult(RESERVED_NAME_MESSAGE, true)
        const doc = buildDoc(args)
        if (isSystemTagName(existing.tag)) {
          const owned = COMM_OWNED_FIELDS.filter((f) => f in doc || (f === 'tag' && newTag))
          if (owned.length)
            return textResult(
              `'${existing.tag}' is maintained by the data processor: ${owned.join(', ')} cannot be changed. ` +
                'Priority, alarm settings, limits, events and historian settings can (see get_system_tags_guide).',
              true
            )
        }
        if (newTag && newTag.trim() !== tag) {
          if (await findTag(api, newTag.trim()))
            return textResult(`Tag '${newTag}' already exists.`, true)
          doc['tag'] = newTag.trim()
        }
        if (Object.keys(doc).length === 0)
          return textResult('No changes given.', true)
        await api.call('updateTag', { _id: existing._id, ...doc })
        return jsonResult({ updated: { tag: doc['tag'] ?? tag, pointKey: existing._id, fields: Object.keys(doc) } })
      } catch (e) {
        return errorResult('Error updating tag', e)
      }
    }
  )

  server.registerTool(
    'delete_tag',
    {
      title: 'Delete Tag',
      description:
        'Delete a tag (point) from realtimeData. Its history and events are kept.',
      inputSchema: z.object({ tag: z.string() }),
      annotations: { readOnlyHint: false, destructiveHint: true },
    },
    async ({ tag }) => {
      try {
        const existing = await findTag(api, tag)
        if (!existing) return textResult(`Tag '${tag}' not found.`, true)
        if (isSystemTagName(existing.tag))
          return textResult(
            `'${existing.tag}' is maintained by the data processor and cannot be deleted; it is removed with its ` +
              'driver instance or connection (see get_system_tags_guide).',
            true
          )
        await api.call('deleteTag', { _id: existing._id })
        return jsonResult({ deleted: { tag, pointKey: existing._id } })
      } catch (e) {
        return errorResult('Error deleting tag', e)
      }
    }
  )
}
