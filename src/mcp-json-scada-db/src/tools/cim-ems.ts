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

// CIM-EMS read tools: driver/connection state and the published result points
// (realtimeData), decoded and checked against operating limits. They read only
// what the CIM-EMS driver publishes as ordinary supervised points and the
// driver/connection configuration; models, bindings and run records go
// through the AdminUI API (tools/cim-ems-admin.ts).

import { z } from 'zod'
import type { McpServer } from '@modelcontextprotocol/server'
import { ConnectionManager } from '../jsonscada/connection-manager.js'
import {
  CA_INSECURE,
  CA_RUN_STATUS,
  CA_STATUS,
  CIM_EMS_DRIVER,
  CIM_EMS_GLOSSARY,
  CIM_EMS_GUIDE,
  LF_STATUS,
  SE_STATUS,
  ageSeconds,
  codeText,
  compileRegex,
  parseResultPoint,
  round,
} from '../jsonscada/cim-ems.js'
import { errorResult, jsonResult, notConnectedResult, textResult } from './util.js'

const KEEPALIVE_TOLERANCE_MS = 30000

// CIM-EMS settings of a protocolConnections document worth showing
const CONNECTION_FIELDS = [
  'protocolConnectionNumber',
  'protocolDriverInstanceNumber',
  'name',
  'description',
  'enabled',
  'cimModelId',
  'cimBindingRevision',
  'cycleSeconds',
  'resultMaxAgeSeconds',
  'autoCreateTags',
  'runStateEstimation',
  'runContingency',
  'contingencyListId',
  'contingencyMinKv',
  'contingencyIntervalSeconds',
  'contingencyMinGapSeconds',
  'contingencyPublishLimit',
]

const RESULT_PROJECTION = {
  _id: 1,
  tag: 1,
  value: 1,
  unit: 1,
  invalid: 1,
  timeTag: 1,
  group1: 1,
  group2: 1,
  protocolSourceCommonAddress: 1,
  protocolSourceObjectAddress: 1,
}

const limitsSchema = {
  loadingLimitPct: z
    .number()
    .positive()
    .optional()
    .default(100)
    .describe('Branch loading (LOAD %, CA WORST %) above which a branch is overloaded'),
  vpuMin: z.number().positive().optional().default(0.95).describe('Lowest acceptable bus voltage (pu)'),
  vpuMax: z.number().positive().optional().default(1.05).describe('Highest acceptable bus voltage (pu)'),
}

interface Limits {
  loadingLimitPct: number
  vpuMin: number
  vpuMax: number
}

function pick(doc: any, fields: string[]) {
  const out: Record<string, unknown> = {}
  for (const f of fields) if (doc?.[f] !== undefined) out[f] = doc[f]
  return out
}

async function cimConnections(mgr: ConnectionManager, connectionNumber?: number) {
  const q: any = { protocolDriver: CIM_EMS_DRIVER }
  if (connectionNumber !== undefined) q.protocolConnectionNumber = connectionNumber
  return mgr.getProtocolConnectionsCollection().find(q).sort({ protocolConnectionNumber: 1 }).toArray()
}

// SYS points of a connection: { 'LF.STATUS': point, 'SE.CHI2': point, ... }
// plus the CA SYSTEM aggregates as 'CA.SYSTEM.<quantity>'
async function systemPoints(mgr: ConnectionManager, connectionNumber: number) {
  const pts = await mgr
    .getRealtimeDataCollection()
    .find(
      {
        protocolSourceConnectionNumber: connectionNumber,
        $or: [
          { protocolSourceCommonAddress: 'SYS' },
          { protocolSourceCommonAddress: 'CA', protocolSourceObjectAddress: /^SYSTEM\|/ },
        ],
      } as any,
      { projection: RESULT_PROJECTION }
    )
    .toArray()
  const map: Record<string, any> = {}
  for (const p of pts) {
    const r = parseResultPoint(p)
    if (!r) continue
    const key = r.kind === 'SYS' ? `${r.elementId}.${r.quantity}` : `CA.SYSTEM.${r.quantity}`
    map[key] = p
  }
  return map
}

const val = (p: any) => (p && !p.invalid ? p.value : null)

function studyHealth(sys: Record<string, any>, resultMaxAge: number, now: number) {
  const lfStatus = sys['LF.STATUS']
  const lfAge = ageSeconds(lfStatus?.timeTag, now)
  const out: Record<string, any> = {
    loadFlow: lfStatus
      ? {
          status: codeText(LF_STATUS, lfStatus.value),
          converged: sys['LF.CONVERGED']?.value === 1,
          iterations: sys['LF.ITER']?.value ?? null,
          durationMs: round(sys['LF.DURATION']?.value, 0),
          topologyUncertain: sys['LF.UNCERTAIN']?.value === 1,
          lastUpdate: lfStatus.timeTag ?? null,
          ageSeconds: lfAge,
          stale: lfStatus.invalid === true || (lfAge !== null && lfAge > resultMaxAge),
        }
      : null,
  }
  if (sys['SE.STATUS'])
    out['stateEstimation'] = {
      status: codeText(SE_STATUS, sys['SE.STATUS'].value),
      converged: sys['SE.CONVERGED']?.value === 1,
      badData: val(sys['SE.BADDATA']),
      measurementsUsed: val(sys['SE.MEAS']),
      chi2Ratio: round(val(sys['SE.CHI2'])),
      lastUpdate: sys['SE.STATUS'].timeTag ?? null,
    }
  if (sys['CA.STATUS'])
    out['contingency'] = {
      status: codeText(CA_RUN_STATUS, sys['CA.STATUS'].value),
      skippedCycles: val(sys['CA.SKIPPED']),
      durationMs: round(val(sys['CA.DURATION']), 0),
      contingencies: val(sys['CA.SYSTEM.COUNT']),
      insecure: val(sys['CA.SYSTEM.INSECURE']),
      violations: val(sys['CA.SYSTEM.VIOL']),
      worstLoadingPct: round(val(sys['CA.SYSTEM.WORST']), 1),
      lastUpdate: sys['CA.STATUS'].timeTag ?? null,
    }
  return out
}

interface ElementRow {
  element: string
  name: string
  substation: string
  voltageLevel: string
  values: Record<string, number | null>
  ends?: Record<string, Record<string, number | null>>
  invalid: boolean
  lastUpdate: Date | null
  flags: string[]
}

// Groups LF/SE result points by element (buses: V/VPU/ANG; branches: LOAD and
// P/Q/I per end) and flags limit violations
function groupElements(points: any[], limits: Limits): ElementRow[] {
  const rows = new Map<string, ElementRow>()
  for (const p of points) {
    const r = parseResultPoint(p)
    if (!r) continue
    let row = rows.get(r.elementId)
    if (!row) {
      row = {
        element: r.elementId,
        name: r.name,
        substation: p.group1 || '',
        voltageLevel: p.group2 || '',
        values: {},
        invalid: false,
        lastUpdate: null,
        flags: [],
      }
      rows.set(r.elementId, row)
    }
    const v = p.invalid ? null : round(p.value, 4)
    if (r.side) {
      row.ends = row.ends || {}
      const end = (row.ends[String(r.side)] = row.ends[String(r.side)] || {})
      end[r.quantity] = v
    } else row.values[r.quantity] = v
    if (p.invalid) row.invalid = true
    if (p.timeTag instanceof Date && (!row.lastUpdate || p.timeTag > row.lastUpdate))
      row.lastUpdate = p.timeTag
  }
  for (const row of rows.values()) {
    const p1 = row.ends?.['1']?.['P']
    const p2 = row.ends?.['2']?.['P']
    if (typeof p1 === 'number' && typeof p2 === 'number') row.values['LOSS'] = round(p1 + p2, 4)
    const load = row.values['LOAD']
    const vpu = row.values['VPU']
    if (typeof load === 'number' && load > limits.loadingLimitPct) row.flags.push('overload')
    if (typeof vpu === 'number' && vpu > 0 && vpu < limits.vpuMin) row.flags.push('undervoltage')
    if (typeof vpu === 'number' && vpu > limits.vpuMax) row.flags.push('overvoltage')
    if (typeof vpu === 'number' && vpu === 0) row.flags.push('deenergized')
    if (row.invalid) row.flags.push('notCalculatedOrStale')
  }
  return [...rows.values()]
}

function severity(row: ElementRow, limits: Limits) {
  const load = row.values['LOAD']
  const vpu = row.values['VPU']
  let s = 0
  if (typeof load === 'number') s = Math.max(s, load / limits.loadingLimitPct)
  if (typeof vpu === 'number' && vpu > 0)
    s = Math.max(s, 1 + Math.max(limits.vpuMin - vpu, vpu - limits.vpuMax, -1) * 10)
  return s
}

async function resultPoints(
  mgr: ConnectionManager,
  connectionNumber: number,
  kind: string,
  quantities?: string[]
) {
  const q: any = { protocolSourceConnectionNumber: connectionNumber, protocolSourceCommonAddress: kind }
  if (quantities?.length)
    q.protocolSourceObjectAddress = new RegExp(
      `\\|(${quantities.map((x) => x.replace(/[^A-Z]/gi, '')).join('|')})$`
    )
  return mgr.getRealtimeDataCollection().find(q, { projection: RESULT_PROJECTION }).toArray()
}

// Attaches the values of the other study (SE for LF, LF for SE) and the
// differences (this - other) to each element row; returns the largest
// difference per row in maxDelta (MW/Mvar for flows, kV for V, deg for ANG)
function attachComparison(rows: ElementRow[], other: ElementRow[], otherKind: string) {
  const byId = new Map(other.map((r) => [r.element, r]))
  for (const r of rows as any[]) {
    const o = byId.get(r.element)
    if (!o) {
      r.compare = { kind: otherKind, missing: true }
      r.maxDelta = null
      continue
    }
    const delta: Record<string, number> = {}
    let max = 0
    for (const q of ['V', 'VPU', 'ANG']) {
      const a = r.values[q], b = o.values[q]
      if (typeof a === 'number' && typeof b === 'number') {
        delta[q] = round(a - b, 4) as number
        if (q !== 'VPU') max = Math.max(max, Math.abs(a - b))
      }
    }
    for (const side of ['1', '2']) {
      for (const q of ['P', 'Q']) {
        const a = r.ends?.[side]?.[q], b = o.ends?.[side]?.[q]
        if (typeof a === 'number' && typeof b === 'number') {
          delta[`${q}${side}`] = round(a - b, 3) as number
          max = Math.max(max, Math.abs(a - b))
        }
      }
    }
    r.compare = { kind: otherKind, values: o.values, ...(o.ends ? { ends: o.ends } : {}), delta }
    r.maxDelta = round(max, 3)
  }
}

function contingencyRows(points: any[]) {
  const rows = new Map<string, any>()
  for (const p of points) {
    const r = parseResultPoint(p)
    if (!r || r.elementId === 'SYSTEM') continue
    let row = rows.get(r.elementId)
    if (!row) {
      row = { contingency: r.elementId, name: r.name, status: null, violations: null, worstLoadingPct: null, invalid: false }
      rows.set(r.elementId, row)
    }
    const v = p.invalid ? null : p.value
    if (r.quantity === 'STATUS') row.status = codeText(CA_STATUS, v)
    else if (r.quantity === 'VIOL') row.violations = v
    else if (r.quantity === 'WORST') row.worstLoadingPct = round(v, 1)
    if (p.invalid) row.invalid = true
  }
  return [...rows.values()]
}

export function registerCimEmsReadTools(server: McpServer, mgr: ConnectionManager) {
  server.registerTool(
    'cim_ems_status',
    {
      title: 'CIM-EMS Status',
      description:
        'Health and operating picture of the CIM-EMS network analysis (load flow, state estimation, ' +
        'contingency analysis): driver instances (alive), CIM-EMS connections and their study settings, ' +
        'last load flow / estimation / contingency status decoded from the SYS points, a limit check of the ' +
        'load flow results (overloaded branches, bus voltages outside vpuMin..vpuMax) and plain-language findings. ' +
        'Start here, then drill down with cim_ems_get_results (and cim_ems_list_runs when the admin tools are enabled).',
      inputSchema: z.object({
        connectionNumber: z.number().int().optional().describe('One CIM-EMS connection; default all'),
        ...limitsSchema,
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ connectionNumber, loadingLimitPct, vpuMin, vpuMax }) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()
      const limits = { loadingLimitPct, vpuMin, vpuMax }
      const now = Date.now()
      try {
        const instances = await mgr
          .getProtocolDriverInstancesCollection()
          .find({ protocolDriver: CIM_EMS_DRIVER } as any)
          .toArray()
        const drivers = instances.map((d: any) => {
          const ka = d.activeNodeKeepAliveTimeTag
          return {
            protocolDriverInstanceNumber: d.protocolDriverInstanceNumber,
            enabled: d.enabled,
            nodeNames: d.nodeNames,
            activeNodeName: d.activeNodeName || null,
            alive: ka instanceof Date && now - ka.getTime() < KEEPALIVE_TOLERANCE_MS,
            keepAliveAgeSeconds: ageSeconds(ka, now),
            softwareVersion: d.softwareVersion || null,
          }
        })
        const conns = await cimConnections(mgr, connectionNumber)
        if (connectionNumber !== undefined && !conns.length)
          return textResult(`No CIM-EMS connection number ${connectionNumber}.`, true)

        const findings: string[] = []
        if (!instances.length) findings.push('No CIM-EMS driver instance is configured.')
        const connections = []
        for (const c of conns as any[]) {
          const n = Number(c.protocolConnectionNumber)
          const inst = drivers.find(
            (d) => Number(d.protocolDriverInstanceNumber) === Number(c.protocolDriverInstanceNumber)
          )
          const resultMaxAge = Number(c.resultMaxAgeSeconds ?? 180)
          const health = studyHealth(await systemPoints(mgr, n), resultMaxAge, now)
          const label = `${c.name} (#${n})`

          // limit check of the load flow results
          const lfRows = groupElements(await resultPoints(mgr, n, 'LF', ['LOAD', 'VPU']), limits)
          const overloads = lfRows
            .filter((r) => r.flags.includes('overload'))
            .sort((a, b) => (b.values['LOAD'] ?? 0) - (a.values['LOAD'] ?? 0))
          const volt = lfRows.filter(
            (r) => r.flags.includes('undervoltage') || r.flags.includes('overvoltage')
          )
          const loads = lfRows.map((r) => r.values['LOAD']).filter((v): v is number => typeof v === 'number')
          const vpus = lfRows
            .filter((r) => typeof r.values['VPU'] === 'number' && (r.values['VPU'] as number) > 0)
            .sort((a, b) => (a.values['VPU'] as number) - (b.values['VPU'] as number))
          const brief = (r: ElementRow) => ({
            element: r.element,
            name: r.name,
            substation: r.substation,
            voltageLevel: r.voltageLevel,
            ...(r.values['LOAD'] !== undefined ? { loadPct: r.values['LOAD'] } : {}),
            ...(r.values['VPU'] !== undefined ? { vpu: r.values['VPU'] } : {}),
          })
          const limitCheck = {
            limits,
            branches: loads.length,
            maxLoadingPct: loads.length ? round(Math.max(...loads), 1) : null,
            overloaded: overloads.length,
            topOverloads: overloads.slice(0, 10).map(brief),
            buses: vpus.length,
            minVoltage: vpus.length ? brief(vpus[0]!) : null,
            maxVoltage: vpus.length ? brief(vpus[vpus.length - 1]!) : null,
            voltageViolations: volt.length,
            topVoltageViolations: volt
              .sort((a, b) => severity(b, limits) - severity(a, limits))
              .slice(0, 10)
              .map(brief),
          }

          if (!c.enabled) findings.push(`${label}: connection disabled, no studies run.`)
          if (inst && !inst.alive && c.enabled)
            findings.push(
              `${label}: CIM-EMS driver instance ${inst.protocolDriverInstanceNumber} is not running (stale keep-alive); ` +
                'results expire after resultMaxAgeSeconds and study requests are not executed.'
            )
          const lf = health['loadFlow']
          if (!lf) findings.push(`${label}: no load flow result points yet (driver never ran for this connection).`)
          else {
            if (lf.status !== 'converged') findings.push(`${label}: last load flow ${lf.status}.`)
            if (lf.stale)
              findings.push(`${label}: load flow results are stale (last update ${lf.ageSeconds ?? '?'} s ago).`)
            if (lf.topologyUncertain)
              findings.push(`${label}: topology uncertain (invalid or held switch states in the snapshot).`)
          }
          if (overloads.length)
            findings.push(
              `${label}: ${overloads.length} branch(es) above ${loadingLimitPct}% loading, worst ${overloads[0]!.name} at ${overloads[0]!.values['LOAD']}%.`
            )
          if (volt.length)
            findings.push(`${label}: ${volt.length} bus(es) outside ${vpuMin}..${vpuMax} pu.`)
          const se = health['stateEstimation']
          if (se) {
            if (se.status !== 'converged') findings.push(`${label}: state estimation ${se.status}.`)
            else if (se.badData) findings.push(`${label}: state estimation removed ${se.badData} bad measurement(s).`)
            if (typeof se.chi2Ratio === 'number' && se.chi2Ratio > 1)
              findings.push(`${label}: state estimation chi-square test failed (ratio ${se.chi2Ratio}).`)
          }
          const ca = health['contingency']
          if (ca) {
            if (ca.status === 'failed' || ca.status === 'no-load-flow')
              findings.push(`${label}: contingency analysis ${ca.status}.`)
            if (ca.insecure)
              findings.push(
                `${label}: ${ca.insecure} of ${ca.contingencies} contingencies insecure, worst post-contingency loading ${ca.worstLoadingPct}%.`
              )
          }

          connections.push({
            ...pick(c, CONNECTION_FIELDS),
            driverInstanceAlive: inst ? inst.alive : null,
            ...health,
            loadFlowLimitCheck: limitCheck,
          })
        }
        if (!conns.length) findings.push('No CIM-EMS connection is configured.')
        if (!findings.length) findings.push('All CIM-EMS studies healthy, no limit violations.')
        return jsonResult({ findings, driverInstances: drivers, connections })
      } catch (e) {
        return errorResult('Error reading CIM-EMS status', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_get_results',
    {
      title: 'CIM-EMS Get Results',
      description:
        'Read the CIM-EMS results of one connection, grouped per network element. ' +
        'kind LF (load flow) and SE (state estimation): buses with V (kV), VPU, ANG (deg); branches with LOAD (%) ' +
        'and P (MW) / Q (Mvar) / I (A) per end ("ends": {"1": ..., "2": ...}); each element is flagged ' +
        'overload / undervoltage / overvoltage / deenergized / notCalculatedOrStale. ' +
        'kind CA: one row per contingency with status (secure, violation, islanding, diverged, unsupported), ' +
        'violations and worstLoadingPct, plus the SYSTEM aggregates. ' +
        'Branches also get LOSS (P1 + P2, MW) and the summary the total losses. ' +
        'Use violationsOnly to list only the problems, ordered by severity; compareWith (SE vs LF) shows where the ' +
        'estimate departs from the model load flow. Also returns a glossary of the quantities; cim_ems_get_guide ' +
        'explains how to judge the results.',
      inputSchema: z.object({
        connectionNumber: z.number().int().describe('CIM-EMS protocolConnectionNumber'),
        kind: z.enum(['LF', 'SE', 'CA']).optional().default('LF'),
        quantities: z
          .array(z.enum(['V', 'VPU', 'ANG', 'P', 'Q', 'I', 'LOAD']))
          .optional()
          .describe('LF/SE: only these quantities (default all)'),
        substation: z.string().optional().describe('Case-insensitive regex on the substation (group1)'),
        voltageLevel: z.string().optional().describe('Case-insensitive regex on the voltage level (group2)'),
        element: z
          .string()
          .optional()
          .describe('Case-insensitive regex on the element / contingency name or mRID'),
        violationsOnly: z.boolean().optional().default(false),
        compareWith: z
          .enum(['LF', 'SE'])
          .optional()
          .describe(
            'LF/SE: add the other study values and the differences (this - other) per element, sorted by the largest difference'
          ),
        ...limitsSchema,
        limit: z.number().int().min(1).max(2000).optional().default(100),
      }),
      annotations: { readOnlyHint: true },
    },
    async (args) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()
      const { connectionNumber, kind, violationsOnly, limit } = args
      const limits = { loadingLimitPct: args.loadingLimitPct, vpuMin: args.vpuMin, vpuMax: args.vpuMax }
      try {
        const reSub = compileRegex(args.substation, 'substation')
        const reVl = compileRegex(args.voltageLevel, 'voltageLevel')
        const reEl = compileRegex(args.element, 'element')
        const conn: any = (await cimConnections(mgr, connectionNumber))[0]
        if (!conn) return textResult(`No CIM-EMS connection number ${connectionNumber}.`, true)
        const points = await resultPoints(mgr, connectionNumber, kind, kind === 'CA' ? undefined : args.quantities)

        if (kind === 'CA') {
          const sys = studyHealth(await systemPoints(mgr, connectionNumber), Number(conn.resultMaxAgeSeconds ?? 180), Date.now())
          let rows = contingencyRows(points).filter(
            (r) => !reEl || reEl.test(r.name) || reEl.test(r.contingency)
          )
          if (violationsOnly)
            rows = rows.filter(
              (r) =>
                CA_INSECURE.includes(r.status) ||
                (typeof r.worstLoadingPct === 'number' && r.worstLoadingPct > limits.loadingLimitPct)
            )
          rows.sort((a, b) => (b.worstLoadingPct ?? -1) - (a.worstLoadingPct ?? -1))
          return jsonResult({
            connection: conn.name,
            kind,
            contingencyRun: sys['contingency'] ?? null,
            total: rows.length,
            returned: Math.min(rows.length, limit),
            contingencies: rows.slice(0, limit),
            note:
              'Only the first contingencyPublishLimit contingencies are published as points; ' +
              'cim_ems_get_contingency_result (admin tools) returns the full result with each violation.',
            glossary: CIM_EMS_GLOSSARY,
          })
        }

        const filter = (r: ElementRow) =>
          (!reSub || reSub.test(r.substation)) &&
          (!reVl || reVl.test(r.voltageLevel)) &&
          (!reEl || reEl.test(r.name) || reEl.test(r.element))
        const all = groupElements(points, limits)
        const losses = all
          .map((r) => r.values['LOSS'])
          .filter((v): v is number => typeof v === 'number')
        let rows = all.filter(filter)
        if (args.compareWith && args.compareWith !== kind)
          attachComparison(
            rows,
            groupElements(await resultPoints(mgr, connectionNumber, args.compareWith, args.quantities), limits).filter(filter),
            args.compareWith
          )
        if (violationsOnly) {
          rows = rows.filter((r) =>
            r.flags.some((f) => f === 'overload' || f === 'undervoltage' || f === 'overvoltage')
          )
          rows.sort((a, b) => severity(b, limits) - severity(a, limits))
        } else if (args.compareWith && args.compareWith !== kind)
          rows.sort((a: any, b: any) => (b.maxDelta ?? -1) - (a.maxDelta ?? -1))
        else
          rows.sort(
            (a, b) =>
              a.substation.localeCompare(b.substation) ||
              a.voltageLevel.localeCompare(b.voltageLevel) ||
              a.name.localeCompare(b.name)
          )
        const flagCount: Record<string, number> = {}
        for (const r of rows) for (const f of r.flags) flagCount[f] = (flagCount[f] || 0) + 1
        return jsonResult({
          connection: conn.name,
          kind,
          limits,
          total: rows.length,
          returned: Math.min(rows.length, limit),
          flagCount,
          ...(losses.length
            ? {
                branchLosses: {
                  totalMW: round(losses.reduce((x, y) => x + y, 0), 3),
                  branches: losses.length,
                  note: 'Sum over all branches of the connection (filters not applied); branch-level only, see run totals for the system balance',
                },
              }
            : {}),
          elements: rows.slice(0, limit),
          glossary: CIM_EMS_GLOSSARY,
        })
      } catch (e) {
        return errorResult('Error reading CIM-EMS results', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_get_guide',
    {
      title: 'CIM-EMS Interpretation Guide',
      description:
        'How to read and judge CIM-EMS results: what LF, SE and CA compute, status meanings, sign and unit ' +
        'conventions, voltage and loading judgement, normalized residuals and the chi-square test, typical causes ' +
        'of bad data, and contingency outcomes. Read it before explaining results to an operator.',
      annotations: { readOnlyHint: true },
    },
    async () => textResult(CIM_EMS_GUIDE)
  )
}
