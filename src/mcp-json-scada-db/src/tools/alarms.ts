import type { McpServer } from '@modelcontextprotocol/server'
import { ObjectId, type Document, type Filter } from 'mongodb'
import { z } from 'zod'
import type { ConnectionManager } from '../jsonscada/connection-manager.js'
import { errorResult, jsonResult, notConnectedResult, parseDateArg, textResult } from './util.js'

const dayMs = 24 * 60 * 60 * 1000
const limitNames = new Map([
  [-3, 'LOLOLO'], [-2, 'LOLO'], [-1, 'LO'], [0, 'normal'],
  [1, 'HI'], [2, 'HIHI'], [3, 'HIHIHI'],
])

const guide = `# Interpreting alarms and Sequence of Events

- alarmsData contains one document per alarm occurrence. active means the condition is still in effect. ack tracks operator acknowledgement independently (0 unacknowledged, 1 acknowledged). An alarm may remain active after acknowledgement or remain unacknowledged after it clears. Digital alarms configured for any change can start inactive.
- Alarm timeTag is when the alarm was raised. timeTagNormal records when the condition ended, and timeTagAck and ackUser record acknowledgement. value, valueString, and alarmText describe the value or transition that raised the alarm. Analog alarmRange codes are 0 normal, 1/2/3 HI/HIHI/HIHIHI and -1/-2/-3 LO/LOLO/LOLOLO.
- soeData records reported changes and events, including analog limit transitions. ack codes are 0 unacknowledged, 1 acknowledged, and 2 eliminated from operator lists. invalid flags data quality. eventText describes the transition.
- In both lists, lower priority numbers are more urgent (0 is highest). group1 is the top-level grouping, usually the station. pointKey links to realtimeData._id. An alarm document, a point, and an SOE document have distinct identifiers. Correlate records using pointKey or tag and the time window.
- SOE timeTag is arrival time. Alarm timeTag is the raise time. timeTagAtSource is the timestamp from the source device when present. Use it for incident ordering only when timeTagAtSourceOk is true, and account for clock differences across devices. Both tools sort by timeTag, newest first.
- Realtime alarms include active or unacknowledged records of any age. Historical alarms are filtered by raise time. Check truncation and the database retention policy before drawing conclusions about coverage.
`

export function registerAlarmTools(server: McpServer, mgr: ConnectionManager) {
  server.registerResource('alarm-soe-guide', 'json-scada://alarms-and-soe/guide', {
    description: 'Interpret JSON-SCADA alarm lifecycle, SOE acknowledgement, quality and timestamps',
    mimeType: 'text/markdown',
  }, async (uri) => ({
    contents: [{ uri: uri.href, mimeType: 'text/markdown', text: guide }],
  }))

  server.registerTool('get_alarms', {
    title: 'Get Alarms',
    annotations: { readOnlyHint: true },
    description: 'Read alarmsData. Realtime mode includes active or unacknowledged alarms of any age. Historical mode filters by raise time. See json-scada://alarms-and-soe/guide for fields and interpretation.',
    inputSchema: z.object({
      mode: z.enum(['realtime', 'historical']).default('realtime'),
      id: z.string().regex(/^[0-9a-f]{24}$/i).optional().describe('Look up an alarm by ObjectId, regardless of mode'),
      tag: z.string().min(1).max(200).optional(),
      pointKey: z.number().finite().optional(),
      group1: z.string().max(200).optional().describe('Exact station or top-level group'),
      priorityMax: z.number().int().min(0).max(9).optional().describe('Largest priority number to include. Lower numbers are more urgent'),
      active: z.boolean().optional(),
      ack: z.union([z.literal(0), z.literal(1)]).optional().describe('0 unacknowledged, 1 acknowledged'),
      from: z.string().max(40).optional().describe('Inclusive start time in ISO 8601, required only to override the historical default'),
      to: z.string().max(40).optional().describe('Inclusive end time in ISO 8601, defaults to now in historical mode'),
      limit: z.number().int().min(1).max(500).default(50),
    }),
  }, async (args) => {
    if (!mgr.status.HintMongoIsConnected) return notConnectedResult()

    try {
      const historical = args.mode === 'historical' && !args.id
      const to = parseDateArg(args.to, 'to') ?? (historical ? new Date() : undefined)
      const from = parseDateArg(args.from, 'from') ??
        (historical && to ? new Date(to.getTime() - dayMs) : undefined)
      if (from && to && from > to) return textResult('from must be at or before to.', true)

      const filter: Filter<Document> = {}
      if (args.id) filter._id = new ObjectId(args.id)
      else if (!historical) filter.$or = [{ active: true }, { ack: 0 }]
      if (from || to)
        filter['timeTag'] = { ...(from && { $gte: from }), ...(to && { $lte: to }) }
      for (const key of ['tag', 'pointKey', 'group1', 'active', 'ack'] as const) {
        if (args[key] !== undefined) filter[key] = args[key]
      }
      if (args.priorityMax !== undefined) filter['priority'] = { $lte: args.priorityMax }

      const rows = await mgr.db.collection('alarmsData').find(filter, { maxTimeMS: 5000 })
        .sort({ timeTag: -1, _id: -1 }).limit(args.limit + 1).toArray()
      const alarms = rows.slice(0, args.limit).map((row) => ({
        ...row,
        interpretation: {
          condition: row['active'] === true ? 'active' : row['active'] === false ? 'inactive' : 'unknown',
          acknowledgement: row['ack'] === 0 ? 'unacknowledged' : row['ack'] === 1 ? 'acknowledged' : 'unknown',
          sourceTimeUsable: row['timeTagAtSourceOk'] === true && row['timeTagAtSource'] instanceof Date,
          ...(row['type'] === 'analog' && row['alarmRange'] != null
            ? { limit: limitNames.get(row['alarmRange']) ?? 'unknown' } : {}),
        },
      }))
      return jsonResult({
        mode: args.id ? 'lookup' : args.mode,
        from, to,
        returned: alarms.length,
        truncated: rows.length > args.limit,
        guide: 'json-scada://alarms-and-soe/guide',
        alarms,
      })
    } catch (e) {
      return errorResult('Error retrieving alarms', e)
    }
  })
}
