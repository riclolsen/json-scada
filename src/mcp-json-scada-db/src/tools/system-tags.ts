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

// The _System.* tags of the data processor: a guide on what they mean and how
// to use them, and a check that reads them next to the configuration and
// explains what they show (offline connections, stopped drivers) and what is
// wrong with them (missing, orphaned, name squatting, generator stopped).

import { z } from 'zod'
import type { McpServer } from '@modelcontextprotocol/server'
import type { ConnectionManager } from '../jsonscada/connection-manager.js'
import {
  ALARM_BEEP_KEY,
  CONN_POINTS,
  DATA_PROCESSOR,
  DIGITAL_UPDATES_KEY,
  DRIVER_POINTS,
  SUMMARY_POINTS,
  SYSTEM_TAGS_GUIDE,
  connTagBase,
  driverTagBase,
  isCommStatusTagName,
} from '../jsonscada/system-tags.js'
import { aliveInfo } from './status.js'
import { errorResult, jsonResult, notConnectedResult, textResult } from './util.js'

const TAG_PROJECTION = {
  _id: 1,
  tag: 1,
  type: 1,
  origin: 1,
  value: 1,
  valueString: 1,
  invalid: 1,
  alarmed: 1,
  timeTag: 1,
  commStatus: 1,
  beepType: 1,
  beepGroup1List: 1,
}

const idHex = (v: any) => (v && typeof v.toHexString === 'function' ? v.toHexString() : String(v))
const isAlive = (doc: any, now: number) => aliveInfo(doc, now)['alive'] === true

function tagView(d: any) {
  if (!d) return null
  return {
    tag: d.tag,
    value: d.type === 'string' ? d.valueString : d.value,
    ...(d.type === 'digital' ? { state: d.valueString } : {}),
    invalid: d.invalid === true,
    ...(d.alarmed ? { alarmed: true } : {}),
  }
}

export function registerSystemTagTools(server: McpServer, mgr: ConnectionManager) {
  server.registerTool(
    'get_system_tags_guide',
    {
      title: '_System Tags Guide',
      description:
        'What the _System.* tags created by the data processor are and how to deal with them: the alarm beep and ' +
        'digital update counter, the communication status tags of every driver instance and connection ' +
        '(_System.Driver/Conn/Comm.*: names, points, meaning, lifecycle), which fields can be changed, the ' +
        'commStatusTags / commStatusMirrorStats switches, how to use them in displays and alarms, and troubleshooting. ' +
        'Read it before creating, changing or interpreting tags whose names begin with "_System.".',
      inputSchema: z.object({}),
      annotations: { readOnlyHint: true },
    },
    async () => textResult(SYSTEM_TAGS_GUIDE)
  )

  server.registerTool(
    'check_system_tags',
    {
      title: 'Check _System Tags',
      description:
        'Read the _System.* tags next to the configuration: the data processor that maintains them and its switches, ' +
        'the alarm beep, the communication summaries, and per driver instance and connection the generated tags with ' +
        'their values. Explains why a connection is Offline (disabled, driver stopped, link down, all tags invalid) and ' +
        'reports missing or orphaned generated tags, user tags in the reserved _System. name space and connection names ' +
        'that collide once sanitized. Filter with connection (name or number) or driver.',
      inputSchema: z.object({
        connection: z
          .union([z.string().min(1), z.number().int()])
          .optional()
          .describe('Connection name (case-insensitive) or protocolConnectionNumber'),
        driver: z.string().optional().describe('Protocol driver name, e.g. IEC60870-5-104'),
        onlyProblems: z
          .boolean()
          .optional()
          .default(false)
          .describe('List only driver instances and connections that are down or have tag problems'),
        limit: z.number().int().min(1).max(1000).optional().default(200),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ connection, driver, onlyProblems, limit }) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()
      try {
        const now = Date.now()
        const findings: string[] = []
        const problems: string[] = []

        // the generator: data processor instance 1
        const procs = await mgr.getProcessInstancesCollection().find({ processName: DATA_PROCESSOR }).toArray()
        const proc1: any = procs.find((p: any) => Number(p.processInstanceNumber) === 1)
        const commOn = proc1?.commStatusTags !== false
        const mirror = proc1?.commStatusMirrorStats === true
        const generatorAlive = !!proc1 && isAlive(proc1, now)
        if (!proc1) findings.push(`No ${DATA_PROCESSOR} instance 1 is registered: no communication status tags are maintained.`)
        else if (!commOn) findings.push('commStatusTags is false: the communication status tags are not maintained and are invalid.')
        else if (!generatorAlive)
          findings.push(
            `${DATA_PROCESSOR} instance 1 is not running (stale keep-alive): every _System tag is frozen at its last value.`
          )

        const instances: any[] = await mgr.getProtocolDriverInstancesCollection().find().toArray()
        const conns: any[] = await mgr
          .getProtocolConnectionsCollection()
          .find()
          .project({
            name: 1,
            protocolConnectionNumber: 1,
            protocolDriver: 1,
            protocolDriverInstanceNumber: 1,
            enabled: 1,
            'stats.isConnected': 1,
          })
          .toArray()
        const sysTags: any[] = await mgr
          .getRealtimeDataCollection()
          .find({ $or: [{ tag: { $regex: '^_System\\.' } }, { commStatus: { $exists: true } }] } as any)
          .project(TAG_PROJECTION)
          .toArray()
        const byTag = new Map<string, any>(sysTags.map((d) => [d.tag, d]))
        const isGenerated = (d: any) => !!d?.commStatus

        // status tags
        const beep = sysTags.find((d) => Number(d._id) === ALARM_BEEP_KEY)
        const updates = sysTags.find((d) => Number(d._id) === DIGITAL_UPDATES_KEY)
        if (!beep) problems.push('_System.Status.AlarmBeep (key -1) is missing; the data processor recreates it at start')
        if (!updates) problems.push('_System.Status.DigitalUpdatesCnt (key -2) is missing; the data processor recreates it at start')
        if (beep && Number(beep.value) !== 0)
          findings.push(
            `Alarm beep active for stations: ${(beep.beepGroup1List || []).join(', ') || '(none listed)'}` +
              (Number(beep.beepType) === 2 ? ' (important: a priority 0 alarm)' : '')
          )

        // driver instances
        const instKey = (d: string, n: unknown) => `${d}|${Number(n)}`
        const instMap = new Map<string, any>(instances.map((i) => [instKey(i.protocolDriver, i.protocolDriverInstanceNumber), i]))
        const knownRefs = new Set<string>([...instances, ...conns].map((d) => idHex(d._id)))
        const expected = new Set<string>(SUMMARY_POINTS.map((p) => '_System.Comm.' + p))
        const wantDriver = (d: string) => !driver || d.toLowerCase() === driver.toLowerCase()

        const drivers: any[] = []
        for (const i of instances) {
          const base = driverTagBase(i.protocolDriver, i.protocolDriverInstanceNumber)
          DRIVER_POINTS.forEach((p) => expected.add(base + p))
          if (!wantDriver(i.protocolDriver) || connection !== undefined) continue
          const missing = DRIVER_POINTS.filter((p) => !isGenerated(byTag.get(base + p)))
          const aliveTag = byTag.get(base + 'Alive')
          const aliveNow = isAlive(i, now)
          const issues: string[] = []
          if (i.enabled !== false && !aliveNow) issues.push('enabled but not running (keep-alive does not advance)')
          if (missing.length) issues.push(`missing tags: ${missing.join(', ')}`)
          drivers.push({
            protocolDriver: i.protocolDriver,
            protocolDriverInstanceNumber: i.protocolDriverInstanceNumber,
            enabled: i.enabled !== false,
            runningByKeepAlive: aliveNow,
            tags: {
              Alive: tagView(aliveTag),
              KeepAliveAge: tagView(byTag.get(base + 'KeepAliveAge')),
              ActiveNode: tagView(byTag.get(base + 'ActiveNode')),
            },
            ...(issues.length ? { issues } : {}),
          })
        }

        // connections
        const bases = new Map<string, string[]>()
        const conRows: any[] = []
        for (const c of conns) {
          const base = connTagBase(c.name)
          bases.set(base, [...(bases.get(base) || []), c.name])
          CONN_POINTS.forEach((p) => expected.add(base + p))
          if (connection !== undefined) {
            const match =
              typeof connection === 'number'
                ? Number(c.protocolConnectionNumber) === connection
                : String(c.name).toLowerCase() === connection.toLowerCase()
            if (!match) continue
          }
          if (!wantDriver(c.protocolDriver)) continue
          const inst = instMap.get(instKey(c.protocolDriver, c.protocolDriverInstanceNumber))
          const t = (p: string) => byTag.get(base + p)
          const tagCount = Number(t('TagCount')?.value ?? 0)
          const invalidTags = Number(t('InvalidTags')?.value ?? 0)
          const reasons: string[] = []
          if (c.enabled === false) reasons.push('connection disabled')
          if (!inst) reasons.push(`no driver instance ${c.protocolDriver} #${c.protocolDriverInstanceNumber}`)
          else {
            if (inst.enabled === false) reasons.push('driver instance disabled')
            if (!isAlive(inst, now)) reasons.push('driver instance not running')
          }
          if (c.stats?.isConnected === false) reasons.push('link down (the driver reports no session or peer)')
          if (tagCount > 0 && invalidTags >= tagCount) reasons.push(`all ${tagCount} supervised tags invalid`)
          const missing = CONN_POINTS.filter((p) => !isGenerated(t(p)))
          const issues: string[] = []
          if (missing.length) issues.push(`missing tags: ${missing.join(', ')}`)
          const status = t('Status')
          if (status && !status.invalid && Number(status.value) === 0 && reasons.length === 0 && generatorAlive)
            issues.push('Status is Offline but no cause is visible now: it updates within 5 s')
          if (status && Number(status.value) === 1 && reasons.length > 0 && generatorAlive)
            issues.push(`Status is Online although: ${reasons.join(', ')} (it updates within 5 s)`)
          const down = c.enabled !== false && reasons.length > 0
          if (onlyProblems && !down && issues.length === 0) continue
          conRows.push({
            name: c.name,
            protocolConnectionNumber: c.protocolConnectionNumber,
            protocolDriver: c.protocolDriver,
            tagBase: base,
            tags: {
              Status: tagView(status),
              LinkUp: tagView(t('LinkUp')),
              Health: tagView(t('Health')),
              TagCount: tagCount,
              InvalidTags: invalidTags,
              StatsAge: tagView(t('StatsAge')),
            },
            ...(reasons.length ? { offlineBecause: reasons } : {}),
            ...(issues.length ? { issues } : {}),
          })
        }
        for (const [base, names] of bases)
          if (names.length > 1)
            problems.push(
              `connections ${names.map((n) => JSON.stringify(n)).join(', ')} share the tag names ${base}*: ` +
                'only one gets communication status tags; rename one connection'
            )

        // tags that should not be there
        const orphans: string[] = []
        const squatters: string[] = []
        const unexpected: string[] = []
        for (const d of sysTags) {
          const key = Number(d._id)
          if (key === ALARM_BEEP_KEY || key === DIGITAL_UPDATES_KEY) continue
          if (isGenerated(d)) {
            if (d.commStatus.kind !== 'summary' && !knownRefs.has(String(d.commStatus.ref))) orphans.push(d.tag)
            else if (!expected.has(d.tag) && !String(d.tag).includes('.Stats.')) unexpected.push(d.tag)
            continue
          }
          if (expected.has(d.tag)) squatters.push(`${d.tag} (key ${key}) blocks the generated tag of that name`)
          else if (isCommStatusTagName(d.tag)) squatters.push(`${d.tag} (key ${key}) is a user tag in a generated name space`)
          else squatters.push(`${d.tag} (key ${key}) is a user tag in the reserved _System. name space`)
        }
        if (orphans.length)
          problems.push(
            `${orphans.length} generated tags belong to removed driver instances or connections` +
              (generatorAlive && commOn ? ' (removed on the next cycle)' : ' (they stay until the data processor runs)') +
              `: ${orphans.slice(0, 20).join(', ')}`
          )
        if (unexpected.length)
          problems.push(`generated tags with outdated names (renamed on the next cycle): ${unexpected.slice(0, 20).join(', ')}`)
        if (squatters.length)
          problems.push(
            ...squatters.slice(0, 50).map((s) => s + '; rename or delete it (user tags must not begin with "_System.")')
          )
        const missingTotal = [...expected].filter((n) => !isGenerated(byTag.get(n))).length
        if (missingTotal && commOn && generatorAlive)
          problems.push(`${missingTotal} expected communication status tags are missing (they appear within 5 s of a configuration change)`)
        else if (missingTotal && commOn)
          problems.push(`${missingTotal} expected communication status tags are missing: start ${DATA_PROCESSOR} instance 1`)

        const down = conRows.filter((r) => r.offlineBecause && r.offlineBecause.length && r.offlineBecause[0] !== 'connection disabled')
        if (down.length)
          findings.push(...down.slice(0, 20).map((r) => `Connection ${r.name} is down: ${r.offlineBecause.join(', ')}.`))
        const stopped = drivers.filter((d) => d.enabled && !d.runningByKeepAlive)
        if (stopped.length)
          findings.push(
            ...stopped.map((d) => `Driver ${d.protocolDriver} #${d.protocolDriverInstanceNumber} is enabled but not running.`)
          )
        if (findings.length === 0 && problems.length === 0) findings.push('All communication status tags are present and consistent.')

        return jsonResult({
          findings,
          problems,
          dataProcessor: proc1
            ? {
                processInstanceNumber: 1,
                ...aliveInfo(proc1, now),
                commStatusTags: commOn,
                commStatusMirrorStats: mirror,
              }
            : null,
          statusTags: {
            AlarmBeep: beep
              ? { ...tagView(beep), beepType: beep.beepType ?? 0, beepGroup1List: beep.beepGroup1List ?? [] }
              : null,
            DigitalUpdatesCnt: tagView(updates),
          },
          summary: Object.fromEntries(
            SUMMARY_POINTS.map((p) => [p, byTag.get('_System.Comm.' + p)?.value ?? null])
          ),
          drivers: drivers.slice(0, limit),
          connections: conRows.slice(0, limit),
          ...(drivers.length > limit || conRows.length > limit ? { truncated: true } : {}),
        })
      } catch (e) {
        return errorResult('Error checking _System tags', e)
      }
    }
  )
}
