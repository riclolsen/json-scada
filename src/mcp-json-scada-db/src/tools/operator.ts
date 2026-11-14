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

// Operator actions through the realtime Web API (/Invoke/ WriteRequest), the
// same path the HMI uses, so rights (ackAlarms, ackEvents, enterAnnotations,
// disableAlarms, enterLimits, enterNotes, substituteValues), group1 filters,
// user-action audit and annotation events all apply.

import { z } from 'zod'
import type { McpServer } from '@modelcontextprotocol/server'
import { AdminApiClient } from '../jsonscada/admin-api.js'
import { ConnectionManager } from '../jsonscada/connection-manager.js'
import { errorResult, jsonResult, notConnectedResult, textResult } from './util.js'

// server_realtime_auth opc_codes.js
const WRITE_REQUEST = 671
const ATTR_DESCRIPTION = 5
const ATTR_ALARM_EVENTS_ACK = 100000004
const ACK = {
  AckOneEvent: 0x1,
  RemoveOneEvent: 0x2,
  AckPointEvents: 0x4,
  RemovePointEvents: 0x8,
  AckAllEvents: 0x40,
  RemoveAllEvents: 0x80,
  AckOneAlarm: 0x100,
  AckAllAlarms: 0x400,
  SilenceBeep: 0x1000,
}

const nodeId = (point: { tag?: string; pointKey?: number }) =>
  point.pointKey !== undefined ? { IdType: 0, Id: point.pointKey } : { IdType: 1, Id: point.tag }

async function findPoint(mgr: ConnectionManager, tag?: string, pointKey?: number) {
  const q: any = pointKey !== undefined ? { _id: pointKey } : { tag }
  return mgr.getRealtimeDataCollection().findOne(q, {
    projection: {
      _id: 1, tag: 1, type: 1, origin: 1, description: 1, value: 1, valueString: 1, alarmed: 1, alarmDisabled: 1,
      annotation: 1, notes: 1, loLimit: 1, hiLimit: 1, hysteresis: 1, substituted: 1, invalid: 1, group1: 1,
    },
  })
}

export function registerOperatorTools(server: McpServer, api: AdminApiClient, mgr: ConnectionManager) {
  server.registerTool(
    'acknowledge_alarms_events',
    {
      title: 'Acknowledge Alarms and Events',
      description:
        'Operator acknowledgement as in the HMI alarm/event viewers. scope "point" acts on one tag: ' +
        'ackAlarm clears its alarmed flag, ackEvents acknowledges its events, removeEvents hides them from the ' +
        'event lists. scope "all" acts on every point/event visible to the MCP user (group1 filter of its ' +
        'role): ackAlarm = acknowledge all alarms, ackEvents = all events, removeEvents = remove all ' +
        'acknowledged/unacknowledged events (a few seconds guard window is kept). silenceBeep stops the ' +
        'audible alarm. Needs the ackAlarms / ackEvents rights of the MCP account. Returns the counts before ' +
        'and after.',
      inputSchema: z.object({
        scope: z.enum(['point', 'all']),
        tag: z.string().optional().describe('Point tag (scope point)'),
        ackAlarm: z.boolean().optional().default(false),
        ackEvents: z.boolean().optional().default(false),
        removeEvents: z.boolean().optional().default(false),
        silenceBeep: z.boolean().optional().default(false),
      }),
      annotations: { readOnlyHint: false, destructiveHint: false },
    },
    async ({ scope, tag, ackAlarm, ackEvents, removeEvents, silenceBeep }) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()
      try {
        if (!ackAlarm && !ackEvents && !removeEvents && !silenceBeep)
          return textResult('Choose at least one of ackAlarm, ackEvents, removeEvents, silenceBeep.', true)
        if (scope === 'point' && !tag) return textResult('scope "point" needs tag.', true)
        const rtd = mgr.getRealtimeDataCollection()
        const soe = mgr.getSoeDataCollection()
        const counts = async () => {
          const f = scope === 'point' ? { tag } : {}
          const [alarmed, unackedEvents, listedEvents] = await Promise.all([
            rtd.countDocuments({ ...f, alarmed: true } as any),
            soe.countDocuments({ ...f, ack: 0 } as any),
            soe.countDocuments({ ...f, ack: { $lte: 1 } } as any),
          ])
          return { alarmed, unackedEvents, listedEvents }
        }
        if (scope === 'point' && !(await rtd.findOne({ tag } as any, { projection: { _id: 1 } })))
          return textResult(`Point '${tag}' not found.`, true)
        const before = await counts()
        let flags = 0
        if (scope === 'point') {
          if (ackAlarm) flags |= ACK.AckOneAlarm
          if (removeEvents) flags |= ACK.RemovePointEvents
          else if (ackEvents) flags |= ACK.AckPointEvents
        } else {
          if (ackAlarm) flags |= ACK.AckAllAlarms
          if (removeEvents) flags |= ACK.RemoveAllEvents
          else if (ackEvents) flags |= ACK.AckAllEvents
        }
        if (silenceBeep) flags |= ACK.SilenceBeep
        await api.invoke(WRITE_REQUEST, {
          NodesToWrite: [
            {
              NodeId: scope === 'point' ? { IdType: 1, Id: tag } : { IdType: 1, Id: '' },
              AttributeId: ATTR_ALARM_EVENTS_ACK,
              Value: { Type: 6, Body: flags },
            },
          ],
        })
        const after = await counts()
        return jsonResult({ scope, ...(tag ? { tag } : {}), done: { ackAlarm, ackEvents, removeEvents, silenceBeep }, before, after })
      } catch (e) {
        return errorResult('Error acknowledging', e)
      }
    }
  )

  server.registerTool(
    'set_point_properties',
    {
      title: 'Set Point Properties',
      description:
        'Change operator properties of a point as the HMI point dialog does: annotation (a tag-out / blocking ' +
        'note shown on displays, empty string clears it; creates an event), alarmDisabled (inhibit alarms), ' +
        'loLimit / hiLimit / hysteresis (analog alarm limits), notes (free text documentation) and ' +
        'substituteValue (manually replace the value of a point, e.g. a failed meter). Fields not given keep ' +
        'their current value. Limits need the point to be analog. Each change needs the matching right of the MCP account (enterAnnotations, ' +
        'disableAlarms, enterLimits, enterNotes, substituteValues); changes without the right are silently ' +
        'ignored by the server, so the result re-reads the point and lists what actually changed. For ' +
        'configuration changes (descriptions, scaling, addresses) use update_tag instead.',
      inputSchema: z.object({
        tag: z.string().optional(),
        pointKey: z.number().int().optional(),
        annotation: z.string().max(2000).optional(),
        alarmDisabled: z.boolean().optional(),
        loLimit: z.number().optional(),
        hiLimit: z.number().optional(),
        hysteresis: z.number().min(0).optional(),
        notes: z.string().max(5000).optional(),
        substituteValue: z.number().optional(),
      }),
      annotations: { readOnlyHint: false, destructiveHint: false },
    },
    async (a) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()
      try {
        if (a.tag === undefined && a.pointKey === undefined) return textResult('Give tag or pointKey.', true)
        const keys = ['annotation', 'alarmDisabled', 'loLimit', 'hiLimit', 'hysteresis', 'notes', 'substituteValue'] as const
        if (!keys.some((k) => a[k] !== undefined)) return textResult('Give at least one property to change.', true)
        const prev: any = await findPoint(mgr, a.tag, a.pointKey)
        const num = (v: unknown, d: number) => (typeof v === 'number' && !Number.isNaN(v) ? v : d)
        if (!prev) return textResult(`Point ${a.tag ?? a.pointKey} not found.`, true)
        const limits = a.loLimit !== undefined || a.hiLimit !== undefined || a.hysteresis !== undefined
        if (limits && prev.type !== 'analog') return textResult(`Point ${prev.tag} is ${prev.type}: limits apply to analog points only.`, true)
        if (limits && num(a.loLimit, num(prev.loLimit, -Infinity)) > num(a.hiLimit, num(prev.hiLimit, Infinity)))
          return textResult('loLimit must not exceed hiLimit.', true)
        if (a.substituteValue !== undefined && prev.origin === 'command')
          return textResult('Command points cannot be substituted.', true)
        // only the given fields: the server leaves absent ones unchanged
        const props: Record<string, unknown> = {}
        for (const k of ['alarmDisabled', 'annotation', 'loLimit', 'hiLimit', 'hysteresis', 'notes'] as const)
          if (a[k] !== undefined) props[k] = a[k]
        if (a.substituteValue !== undefined) {
          props['substituted'] = true
          props['newValue'] = a.substituteValue
        }
        await api.invoke(WRITE_REQUEST, {
          NodesToWrite: [
            {
              NodeId: nodeId({ pointKey: prev._id }),
              AttributeId: ATTR_DESCRIPTION,
              Value: { Type: 12, Body: 'Properties', _Properties: props },
            },
          ],
        })
        const cur: any = await findPoint(mgr, undefined, prev._id)
        const requested: Record<string, unknown> = {}
        for (const k of ['annotation', 'alarmDisabled', 'loLimit', 'hiLimit', 'hysteresis', 'notes'] as const)
          if (a[k] !== undefined) requested[k] = a[k]
        if (a.substituteValue !== undefined) requested['value'] = a.substituteValue
        const applied: Record<string, unknown> = {}
        const notApplied: Record<string, unknown> = {}
        for (const [k, v] of Object.entries(requested)) {
          const now = k === 'alarmDisabled' ? !!cur?.[k] : k === 'annotation' || k === 'notes' ? cur?.[k] ?? '' : cur?.[k]
          if (now === v) applied[k] = v
          else notApplied[k] = { requested: v, current: now ?? null }
        }
        return jsonResult({
          tag: prev.tag,
          pointKey: prev._id,
          applied,
          ...(Object.keys(notApplied).length
            ? { notApplied, hint: 'Not applied: no role of the MCP account has the right for that property (enterAnnotations, disableAlarms, enterLimits, enterNotes, substituteValues).' }
            : {}),
          point: cur,
        })
      } catch (e) {
        return errorResult('Error setting point properties', e)
      }
    }
  )
}
