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

// CIM-EMS control through the AdminUI API (server_realtime_auth
// /Invoke/auth/cimEms/*, cimEms.controller.js): model revisions, binding
// revisions, run records and study requests. The web server only queues
// cimStudyRequests; the active CIM-EMS driver node executes them
// (src/cim-ems/cim_ems/api.py). Rights are enforced by the API: every route
// needs isAdmin; upload, activate, rollback, binding proposals and saves and
// artifact downloads also need a role with cimEngineering. Every request and
// download is written to userActions under the MCP admin account.

import fs from 'node:fs/promises'
import path from 'node:path'
import { z } from 'zod'
import type { McpServer } from '@modelcontextprotocol/server'
import { AdminApiClient } from '../jsonscada/admin-api.js'
import { ConnectionManager } from '../jsonscada/connection-manager.js'
import {
  CA_INSECURE,
  CIM_EMS_DRIVER,
  compileRegex,
  parseResultPoint,
  quantityOfTag,
  residualHints,
} from '../jsonscada/cim-ems.js'
import { errorResult, jsonResult, textResult } from './util.js'

const API = 'cimEms/'
const MODEL_ID = z
  .string()
  .regex(/^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/)
  .describe('CIM-EMS model ID (cimModelId of the connections)')
const MAX_UPLOAD = 100 * 1024 * 1024 // express-fileupload limit of server_realtime_auth
const LIST_CAP = 200

const waitSchema = (def: number) =>
  z
    .number()
    .int()
    .min(0)
    .max(900)
    .optional()
    .default(def)
    .describe(
      'Seconds to wait for the CIM-EMS driver to execute the request (0 = return at once with the requestId; ' +
        'check later with cim_ems_get_request)'
    )

// Fields of a binding (cimEms.controller.js BINDING_FIELDS / cim_ems Binding)
const bindingSchema = z.object({
  kind: z.enum(['switch', 'tap', 'analog']),
  element_id: z.string().describe('mRID of the switch, transformer, branch or bus'),
  tag: z.string().describe('JSON-SCADA tag of the telemetered point'),
  quantity: z.enum(['V', 'P', 'Q', 'I']).optional().describe('analog only'),
  side: z.number().int().min(0).max(3).optional().describe('analog branch end 1/2/3, 0 for buses'),
  closed_value: z.number().optional().describe('switch: processed value meaning closed (default 1)'),
  sign: z.number().optional().describe('analog: SCADA to CGMES terminal sign (default 1)'),
  sigma: z.number().positive().optional().describe('analog: standard deviation, value unit (default 1)'),
  max_age_seconds: z.number().positive().optional().describe('analog (default 30)'),
  approved: z.boolean().optional(),
})

const BINDING_KEYS = [
  'kind',
  'element_id',
  'tag',
  'quantity',
  'side',
  'point_key',
  'closed_value',
  'sign',
  'sigma',
  'max_age_seconds',
  'approved',
  'confidence',
  'proposed_from',
]

function cleanBinding(b: any): any {
  const out: Record<string, any> = {}
  for (const k of BINDING_KEYS) if (b?.[k] !== undefined && b[k] !== null) out[k] = b[k]
  if (out['element_id'] === undefined && b?.cimMeasurementMrid) out['element_id'] = b.cimMeasurementMrid
  return out
}

// element identity of a binding (one telemetered value per element end/quantity)
const slotKey = (b: any) => `${b.kind}|${b.element_id}|${Number(b.side || 0)}|${b.quantity || ''}`
// full identity, as the AdminUI uses to carry approvals over to a new proposal
const fullKey = (b: any) => `${slotKey(b)}|${b.tag}`
// settings compared to report modified entries
const SETTING_KEYS = ['closed_value', 'sign', 'sigma', 'max_age_seconds', 'point_key']

// mRID -> { name, substation, voltageLevel } from the CIM-EMS load flow result
// points (realtimeData), to name the elements of contingency results
async function elementNames(mgr: ConnectionManager, connectionNumber?: number) {
  const names = new Map<string, { name: string; substation: string; voltageLevel: string }>()
  if (!mgr.status.HintMongoIsConnected) return names
  const q: any = { protocolSourceCommonAddress: { $in: ['LF', 'CA'] } }
  if (connectionNumber) q.protocolSourceConnectionNumber = connectionNumber
  else q.protocolSourceObjectAddress = /\|0\|(VPU|LOAD|STATUS)$/
  const pts = await mgr
    .getRealtimeDataCollection()
    .find(q, { projection: { tag: 1, group1: 1, group2: 1, protocolSourceCommonAddress: 1, protocolSourceObjectAddress: 1 } })
    .toArray()
  for (const p of pts) {
    const r = parseResultPoint(p)
    if (r && !names.has(r.elementId))
      names.set(r.elementId, { name: r.name, substation: (p as any).group1 || '', voltageLevel: (p as any).group2 || '' })
  }
  return names
}

function coverage(list: any[]) {
  const out: Record<string, { total: number; approved: number }> = {}
  for (const b of list) {
    const c = (out[b.kind] = out[b.kind] || { total: 0, approved: 0 })
    c.total++
    if (b.approved) c.approved++
  }
  return out
}

function compactRequest(r: any) {
  if (!r) return null
  const { _id, params, ...rest } = r
  const p = { ...(params || {}) }
  delete p.artifactId
  return { requestId: String(_id), ...rest, params: p }
}

function compactRun(r: any) {
  const s = r.summary || {}
  const est = s.estimation
  const ca = s.contingency
  return {
    runId: r.runId,
    connectionNumber: r.connectionNumber,
    createdAt: r.createdAt,
    trigger: r.trigger,
    nodeName: r.nodeName,
    status: r.status,
    published: r.published,
    durationMs: r.durationMs,
    modelId: r.modelId,
    modelRevision: r.modelRevision,
    bindingRevision: r.bindingRevision,
    topologyHash: r.topologyHash ? String(r.topologyHash).slice(0, 12) : '',
    snapshotTime: r.snapshotTime,
    buses: s.buses,
    islands: s.islands,
    switchesOpen: s.switchesOpen,
    iterations: s.iterations,
    loadFlowStatus: s.loadFlowStatus,
    ...(s.reason ? { reason: s.reason } : {}),
    ...(s.error ? { error: s.error } : {}),
    ...(s.totals ? { totals: s.totals } : {}),
    flags: s.flags,
    snapshotReasons: (s.reasons || []).slice(0, 5),
    ...(est
      ? {
          estimation: {
            status: est.status,
            reason: est.reason,
            measurements: est.measurements,
            degreesOfFreedom: est.degreesOfFreedom,
            chi2Passed: est.chi2Passed,
            chi2Ratio: est.chi2Ratio,
            badData: est.badData,
            largestResiduals: est.largestResiduals,
            durationMs: est.durationMs,
          },
        }
      : {}),
    ...(ca
      ? {
          contingency: {
            status: ca.status,
            error: ca.error,
            contingencies: ca.contingencies,
            insecure: ca.insecure,
            unsupported: ca.unsupported,
            baseViolations: ca.baseViolations,
            durationMs: ca.durationMs,
            artifactId: ca.artifactId,
          },
        }
      : {}),
  }
}

async function listRequests(api: AdminApiClient, limit = 200): Promise<any[]> {
  return (await api.get(API + 'listRequests', { limit }))?.requests || []
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

// Polls the request until the driver marks it done/failed or the wait elapses
async function waitRequest(api: AdminApiClient, requestId: string, waitSeconds: number) {
  const until = Date.now() + waitSeconds * 1000
  for (;;) {
    const r = (await listRequests(api)).find((x) => String(x._id) === requestId)
    if (!r) throw new Error(`Study request ${requestId} not found.`)
    if (r.status === 'done' || r.status === 'failed' || Date.now() >= until) {
      const out: any = compactRequest(r)
      if (r.status === 'pending' || r.status === 'running') {
        out.waitTimedOut = waitSeconds > 0
        out.note =
          r.status === 'pending'
            ? 'Not picked up yet: requests run on the active CIM-EMS driver node, so the driver must be running ' +
              '(cim_ems_status / control_driver_process). Check again with cim_ems_get_request.'
            : 'Still running on the driver. Check again with cim_ems_get_request.'
      }
      return out
    }
    await sleep(1000)
  }
}

async function queue(api: AdminApiClient, body: Record<string, unknown>, waitSeconds: number) {
  const { requestId } = await api.call(API + 'request', body)
  return waitRequest(api, String(requestId), waitSeconds)
}

function requestOutcome(r: any) {
  return textResult(JSON.stringify(r, null, 2), r?.status === 'failed')
}

async function downloadJson(api: AdminApiClient, artifactId: string) {
  const buf = await api.download(API + 'artifact', { id: artifactId })
  try {
    return JSON.parse(buf.toString('utf8'))
  } catch {
    throw new Error(`Artifact ${artifactId} is not a JSON document.`)
  }
}

async function currentBindings(api: AdminApiClient, modelId: string, revision?: number) {
  const r = await api.get(API + 'listBindings', { modelId, revision })
  return {
    revisions: r?.revisions || [],
    bindingRevision: r?.bindingRevision ?? null,
    bindings: ((r?.bindings || []) as any[]).map(cleanBinding),
  }
}

const artifactIdSchema = z.string().regex(/^[0-9a-fA-F]{24}$/)

// Element catalog of a model revision (listElements request, executed by the
// driver): every switch, tap changer and P/Q/V slot with names and the label
// the name matcher uses. An existing catalog artifact is reused when given.
async function elementCatalog(
  api: AdminApiClient,
  modelId: string,
  revision: number | undefined,
  artifactId: string | undefined,
  waitSeconds: number
): Promise<{ artifactId: string; modelRevision?: number; entries: any[] } | { request: any }> {
  if (!artifactId) {
    const req = await queue(api, { type: 'listElements', modelId, ...(revision ? { revision } : {}) }, waitSeconds)
    if (req.status !== 'done') return { request: req }
    artifactId = String(req.result.artifactId)
    const entries = await downloadJson(api, artifactId)
    return { artifactId, modelRevision: req.result.modelRevision, entries }
  }
  const entries = await downloadJson(api, artifactId)
  if (!Array.isArray(entries) || (entries.length && !('label' in entries[0])))
    throw new Error(`Artifact ${artifactId} is not an element catalog.`)
  return { artifactId, entries }
}

// protocolConnectionNumbers of the CIM-EMS connections: their points are
// results, never measurements
async function cimConnectionNumbers(mgr: ConnectionManager): Promise<number[]> {
  const conns = await mgr
    .getProtocolConnectionsCollection()
    .find({ protocolDriver: CIM_EMS_DRIVER } as any, { projection: { protocolConnectionNumber: 1 } })
    .toArray()
  return conns.map((c: any) => Number(c.protocolConnectionNumber))
}

const EXPECTED_UNIT: Record<string, string[]> = {
  V: ['kv'],
  P: ['mw'],
  Q: ['mvar', 'mvr', 'mvar.'],
  I: ['a'],
}

// Checks every binding against its realtimeData point: errors make the
// driver ignore the binding or feed it wrong data, warnings need a look.
// Also refreshes point_key from the point. Entries are changed in place.
async function validateBindings(
  mgr: ConnectionManager,
  list: Record<string, any>[],
  catalog: any[] | null
) {
  const errors: string[] = []
  const warnings: string[] = []
  const own = await cimConnectionNumbers(mgr)
  const tags = [...new Set(list.map((b) => String(b['tag'])))]
  const points = new Map<string, any>()
  for (let i = 0; i < tags.length; i += 5000) {
    const docs = await mgr
      .getRealtimeDataCollection()
      .find({ tag: { $in: tags.slice(i, i + 5000) } } as any, {
        projection: { _id: 1, tag: 1, type: 1, unit: 1, origin: 1, protocolSourceConnectionNumber: 1 },
      })
      .toArray()
    for (const d of docs) points.set(String(d.tag), d)
  }
  const slots = catalog ? new Set(catalog.map(slotKey)) : null
  let pointKeysSet = 0
  const owner = new Map<string, string>()
  for (const b of list) {
    const where = `${b['tag']} -> ${slotKey(b)}`
    const sev = b['approved'] ? errors : warnings
    if (slots && !slots.has(slotKey(b))) sev.push(`no such model slot (element/side/quantity): ${where}`)
    const p = points.get(String(b['tag']))
    if (!p) {
      sev.push(`tag not found: ${where}`)
      continue
    }
    if (b['point_key'] !== Number(p._id)) {
      b['point_key'] = Number(p._id)
      pointKeysSet++
    }
    if (own.includes(Number(p.protocolSourceConnectionNumber)))
      sev.push(`CIM-EMS result point, not a measurement: ${where}`)
    if (b['kind'] === 'switch' && p.type !== 'digital')
      warnings.push(`switch bound to an ${p.type} point (closed when value == closed_value ${b['closed_value'] ?? 1}): ${where}`)
    if ((b['kind'] === 'analog' || b['kind'] === 'tap') && p.type !== 'analog')
      sev.push(`${b['kind']} needs an analog point, ${p.type} given: ${where}`)
    if (b['kind'] === 'analog') {
      const unit = String(p.unit || '').trim().toLowerCase()
      const exp = EXPECTED_UNIT[b['quantity']] || []
      if (unit && !exp.includes(unit))
        warnings.push(`unit '${p.unit}' does not look like ${b['quantity']} (${exp[0]}): ${where}`)
    }
    if (p.origin && p.origin !== 'supervised') warnings.push(`point origin is ${p.origin}, not supervised: ${where}`)
    if (b['approved']) {
      const prev = owner.get(String(b['tag']))
      if (prev && prev !== slotKey(b)) errors.push(`tag bound to two model values: ${b['tag']} -> ${prev} and ${slotKey(b)}`)
      owner.set(String(b['tag']), slotKey(b))
    }
  }
  return { errors, warnings, pointKeysSet, elementsChecked: !!slots }
}

export function registerCimEmsAdminTools(
  server: McpServer,
  api: AdminApiClient,
  mgr: ConnectionManager
) {
  server.registerTool(
    'cim_ems_list_models',
    {
      title: 'CIM-EMS List Models',
      description:
        'List the CGMES model revisions (status validated / rejected / active / retired, CGMES version, ' +
        'import and activation dates, element counts, validation summary and base case load flow). ' +
        'With modelId and includeValidation, return the full validation report (errors, warnings, unsupported classes). ' +
        'canEngineer tells whether the MCP account has the CIM-EMS Engineering right needed to change models and bindings.',
      inputSchema: z.object({
        modelId: MODEL_ID.optional(),
        includeValidation: z.boolean().optional().default(false),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ modelId, includeValidation }) => {
      try {
        const r = await api.get(API + 'listModels')
        const models = ((r?.models || []) as any[])
          .filter((m) => !modelId || m.modelId === modelId)
          .map((m) => {
            const v = m.validation || {}
            return {
              modelId: m.modelId,
              revision: m.revision,
              status: m.status,
              cgmesVersion: m.cgmesVersion,
              importedAt: m.importedAt,
              importedBy: m.importedBy,
              activatedAt: m.activatedAt,
              pypowsybl: m.engine?.pypowsybl,
              counts: m.counts,
              validation: {
                ok: v.ok,
                errors: (v.errors || []).length,
                warnings: (v.warnings || []).length,
                unsupported: (v.unsupported || []).length,
                islands: v.islands,
                loadFlow: v.load_flow,
                ...(includeValidation && modelId
                  ? {
                      errorList: (v.errors || []).slice(0, LIST_CAP),
                      warningList: (v.warnings || []).slice(0, LIST_CAP),
                      unsupportedList: (v.unsupported || []).slice(0, LIST_CAP),
                    }
                  : {}),
              },
            }
          })
        return jsonResult({ canEngineer: r?.canEngineer === true, models })
      } catch (e) {
        return errorResult('Error listing CIM-EMS models', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_list_bindings',
    {
      title: 'CIM-EMS List Bindings',
      description:
        'List the measurement binding revisions of a model (which SCADA tags feed the switch states, taps and ' +
        'analog measurements of the network model) with coverage (approved/total per kind), and the bindings of ' +
        'one revision (default: the latest) filtered by kind, approval, tag or element regex. Only approved ' +
        'bindings are used by the driver.',
      inputSchema: z.object({
        modelId: MODEL_ID,
        bindingRevision: z.number().int().positive().optional(),
        kind: z.enum(['switch', 'tap', 'analog']).optional(),
        approved: z.boolean().optional(),
        tag: z.string().optional().describe('Case-insensitive regex on the tag'),
        element: z.string().optional().describe('Case-insensitive regex on the element mRID'),
        limit: z.number().int().min(0).max(2000).optional().default(50),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ modelId, bindingRevision, kind, approved, tag, element, limit }) => {
      try {
        const reTag = compileRegex(tag, 'tag')
        const reEl = compileRegex(element, 'element')
        const cur = await currentBindings(api, modelId, bindingRevision)
        const filtered = cur.bindings.filter(
          (b) =>
            (!kind || b.kind === kind) &&
            (approved === undefined || !!b.approved === approved) &&
            (!reTag || reTag.test(b.tag)) &&
            (!reEl || reEl.test(b.element_id))
        )
        return jsonResult({
          modelId,
          revisions: cur.revisions,
          bindingRevision: cur.bindingRevision,
          coverage: coverage(cur.bindings),
          matched: filtered.length,
          returned: Math.min(filtered.length, limit),
          bindings: filtered.slice(0, limit),
        })
      } catch (e) {
        return errorResult('Error listing CIM-EMS bindings', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_list_runs',
    {
      title: 'CIM-EMS List Runs',
      description:
        'List the latest study runs (cimRuns): trigger (start, cycle, topology change, request), load flow status, ' +
        'iterations, islands, open switches, snapshot flags and rejection reasons, state estimation (measurements, ' +
        'chi-square test, bad data with normalized residuals) and contingency outcome (insecure contingency ids, ' +
        'result artifact). Includes a status count over the returned runs.',
      inputSchema: z.object({
        connectionNumber: z.number().int().positive().optional(),
        status: z
          .enum(['converged', 'diverged', 'rejected-snapshot', 'unsupported', 'failed'])
          .optional(),
        limit: z.number().int().min(1).max(200).optional().default(20),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ connectionNumber, status, limit }) => {
      try {
        const r = await api.get(API + 'listRuns', {
          connectionNumber,
          limit: status ? 1000 : limit,
        })
        const all = ((r?.runs || []) as any[]).map(compactRun)
        const statusCount: Record<string, number> = {}
        for (const x of all) statusCount[x.status] = (statusCount[x.status] || 0) + 1
        const runs = all.filter((x) => !status || x.status === status).slice(0, limit)
        return jsonResult({ statusCount, runs })
      } catch (e) {
        return errorResult('Error listing CIM-EMS runs', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_list_requests',
    {
      title: 'CIM-EMS List Requests',
      description:
        'List the CIM-EMS study request queue (import, activate, rollback, proposeBindings, saveBindings, rerun) ' +
        'with status (pending, running, done, failed), result and error.',
      inputSchema: z.object({
        status: z.enum(['pending', 'running', 'done', 'failed']).optional(),
        type: z
          .enum(['import', 'activate', 'rollback', 'proposeBindings', 'saveBindings', 'rerun'])
          .optional(),
        limit: z.number().int().min(1).max(500).optional().default(20),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ status, type, limit }) => {
      try {
        const list = (await listRequests(api, status || type ? 500 : limit))
          .filter((r) => (!status || r.status === status) && (!type || r.type === type))
          .slice(0, limit)
          .map(compactRequest)
        return jsonResult(list)
      } catch (e) {
        return errorResult('Error listing CIM-EMS requests', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_get_request',
    {
      title: 'CIM-EMS Get Request',
      description: 'Get one CIM-EMS study request by id, optionally waiting until the driver has executed it.',
      inputSchema: z.object({
        requestId: artifactIdSchema,
        waitSeconds: waitSchema(0),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ requestId, waitSeconds }) => {
      try {
        return requestOutcome(await waitRequest(api, requestId, waitSeconds))
      } catch (e) {
        return errorResult('Error reading CIM-EMS request', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_run_now',
    {
      title: 'CIM-EMS Run Study Now',
      description:
        'Request an immediate study run (load flow, and state estimation / contingency analysis when enabled on the ' +
        'connection) for a CIM-EMS connection, wait for it and return the new run record. The driver must be running.',
      inputSchema: z.object({
        connectionNumber: z.number().int().positive(),
        waitSeconds: waitSchema(60),
      }),
      annotations: { readOnlyHint: false, destructiveHint: false },
    },
    async ({ connectionNumber, waitSeconds }) => {
      try {
        const started = Date.now()
        const req = await queue(api, { type: 'rerun', connectionNumber }, waitSeconds)
        if (req.status !== 'done') return requestOutcome(req)
        // the run follows the request in the same driver loop iteration
        const after = new Date(req.finishedAt || req.createdAt).getTime() - 1000
        const until = started + waitSeconds * 1000
        for (;;) {
          const runs: any[] = (await api.get(API + 'listRuns', { connectionNumber, limit: 5 }))?.runs || []
          const run = runs.find(
            (r) => r.trigger === 'request' && new Date(r.createdAt).getTime() >= after
          )
          if (run) return jsonResult({ request: req, run: compactRun(run) })
          if (Date.now() >= until)
            return jsonResult({
              request: req,
              run: null,
              note: 'The run record is not stored yet; check with cim_ems_list_runs.',
            })
          await sleep(1000)
        }
      } catch (e) {
        return errorResult('Error requesting a CIM-EMS run', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_import_model',
    {
      title: 'CIM-EMS Import Model',
      description:
        'Upload a CGMES package (.zip with EQ/SSH/TP/SV..., CGMES 2.4.15 or 3.0) from a file on the MCP server host ' +
        'as a new revision of modelId. The CIM-EMS driver imports and validates it; a rejected revision cannot be ' +
        'activated. The new revision is not used until cim_ems_activate_model. Needs CIM-EMS Engineering rights.',
      inputSchema: z.object({
        modelId: MODEL_ID,
        filePath: z.string().describe('Path of the .zip on the host running this MCP server'),
        waitSeconds: waitSchema(300),
      }),
      annotations: { readOnlyHint: false, destructiveHint: false },
    },
    async ({ modelId, filePath, waitSeconds }) => {
      try {
        const name = path.basename(filePath)
        if (!/^[a-zA-Z0-9][a-zA-Z0-9 ._()-]{0,250}\.zip$/i.test(name))
          return textResult(`Invalid package file name '${name}' (letters, digits, space ._()- and a .zip extension).`, true)
        const st = await fs.stat(filePath)
        if (!st.isFile()) return textResult(`${filePath} is not a file.`, true)
        if (st.size > MAX_UPLOAD) return textResult(`${filePath} exceeds the 100 MB upload limit.`, true)
        const data = await fs.readFile(filePath)
        if (data.length < 4 || data.readUInt32LE(0) !== 0x04034b50)
          return textResult(`${filePath} is not a zip file.`, true)
        const form = new FormData()
        form.set('modelId', modelId)
        form.set('packageFile', new Blob([new Uint8Array(data)], { type: 'application/zip' }), name)
        const { requestId } = await api.postForm(API + 'uploadModel', form)
        return requestOutcome(await waitRequest(api, String(requestId), waitSeconds))
      } catch (e) {
        return errorResult('Error importing CIM-EMS model', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_activate_model',
    {
      title: 'CIM-EMS Activate Model',
      description:
        'Make a validated model revision the active one. Every CIM-EMS connection using this modelId is rebuilt at ' +
        'once and its results then come from the new revision. Needs CIM-EMS Engineering rights.',
      inputSchema: z.object({
        modelId: MODEL_ID,
        revision: z.number().int().positive(),
        waitSeconds: waitSchema(120),
      }),
      annotations: { readOnlyHint: false, destructiveHint: true },
    },
    async ({ modelId, revision, waitSeconds }) => {
      try {
        return requestOutcome(await queue(api, { type: 'activate', modelId, revision }, waitSeconds))
      } catch (e) {
        return errorResult('Error activating CIM-EMS model', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_rollback_model',
    {
      title: 'CIM-EMS Roll Back Model',
      description:
        'Reactivate the previous retired revision of a model in place of the active one; the connections using it ' +
        'are rebuilt at once. Needs CIM-EMS Engineering rights.',
      inputSchema: z.object({ modelId: MODEL_ID, waitSeconds: waitSchema(120) }),
      annotations: { readOnlyHint: false, destructiveHint: true },
    },
    async ({ modelId, waitSeconds }) => {
      try {
        return requestOutcome(await queue(api, { type: 'rollback', modelId }, waitSeconds))
      } catch (e) {
        return errorResult('Error rolling back CIM-EMS model', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_propose_bindings',
    {
      title: 'CIM-EMS Propose Bindings',
      description:
        'Ask the driver to propose measurement bindings by matching the supervised tags to the model element names ' +
        '(switches: "<substation> <voltage level> <switch>"; analogs by quantity and branch/bus). Returns the match ' +
        'summary, unmatched and ambiguous elements, and the proposal artifactId to review with ' +
        'cim_ems_get_binding_proposal and save with cim_ems_save_bindings. Proposals are never approved ' +
        'automatically and nothing changes until saved. Needs CIM-EMS Engineering rights.',
      inputSchema: z.object({
        modelId: MODEL_ID,
        revision: z.number().int().positive().optional().describe('Model revision (default: the active one)'),
        waitSeconds: waitSchema(300),
      }),
      annotations: { readOnlyHint: false, destructiveHint: false },
    },
    async ({ modelId, revision, waitSeconds }) => {
      try {
        return requestOutcome(
          await queue(api, { type: 'proposeBindings', modelId, ...(revision ? { revision } : {}) }, waitSeconds)
        )
      } catch (e) {
        return errorResult('Error proposing CIM-EMS bindings', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_get_binding_proposal',
    {
      title: 'CIM-EMS Get Binding Proposal',
      description:
        'Review a binding proposal (artifactId from cim_ems_propose_bindings): counts per kind and confidence band, ' +
        'and the proposed bindings filtered by kind, minimum confidence, tag or element regex. ' +
        'Needs CIM-EMS Engineering rights.',
      inputSchema: z.object({
        artifactId: artifactIdSchema,
        kind: z.enum(['switch', 'tap', 'analog']).optional(),
        minConfidence: z.number().min(0).max(1).optional(),
        maxConfidence: z.number().min(0).max(1).optional(),
        tag: z.string().optional().describe('Case-insensitive regex on the tag'),
        element: z.string().optional().describe('Case-insensitive regex on the element mRID'),
        limit: z.number().int().min(0).max(2000).optional().default(50),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ artifactId, kind, minConfidence, maxConfidence, tag, element, limit }) => {
      try {
        const reTag = compileRegex(tag, 'tag')
        const reEl = compileRegex(element, 'element')
        const list = await downloadJson(api, artifactId)
        if (!Array.isArray(list)) return textResult('The artifact is not a binding proposal.', true)
        const bands: Record<string, number> = { '>=0.9': 0, '0.7-0.9': 0, '0.5-0.7': 0, '<0.5': 0 }
        for (const b of list) {
          const c = Number(b.confidence) || 0
          bands[c >= 0.9 ? '>=0.9' : c >= 0.7 ? '0.7-0.9' : c >= 0.5 ? '0.5-0.7' : '<0.5']!++
        }
        const filtered = list.map(cleanBinding).filter((b) => {
          const c = Number(b['confidence']) || 0
          return (
            (!kind || b['kind'] === kind) &&
            (minConfidence === undefined || c >= minConfidence) &&
            (maxConfidence === undefined || c <= maxConfidence) &&
            (!reTag || reTag.test(b['tag'])) &&
            (!reEl || reEl.test(b['element_id']))
          )
        })
        return jsonResult({
          total: list.length,
          perKind: coverage(list),
          confidenceBands: bands,
          matched: filtered.length,
          returned: Math.min(filtered.length, limit),
          bindings: filtered.slice(0, limit),
        })
      } catch (e) {
        return errorResult('Error reading CIM-EMS binding proposal', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_list_model_elements',
    {
      title: 'CIM-EMS List Model Elements',
      description:
        'List the model values that SCADA points can be bound to, for matching field tags to CIM objects: ' +
        'switches (breaker/disconnector state, digital point), tap changers (position, analog point) and analog slots ' +
        '(P/Q at each line or transformer end in MW/Mvar, V at a busbar in kV). Each entry has element_id, side, ' +
        'quantity, name, substation, voltage level, nominal kV, the far end of branches, the label the automatic name ' +
        'matcher compares with tag names, and its current binding (tag, approved) and, with proposalArtifactId, the ' +
        'proposed tag and confidence. Filter by status: unbound (no binding), unapproved, bound, all. ' +
        'Workflow: work substation by substation: list the unbound elements here, find the field tags with ' +
        'cim_ems_candidate_tags (same substation), match them by name, bay, voltage and quantity, check with ' +
        'cim_ems_save_bindings dryRun (upsert with approved=true, elementsArtifactId=catalogArtifactId), then save. ' +
        'The first call runs a listElements request on the driver; pass the returned catalogArtifactId to later calls ' +
        'to reuse it. Needs CIM-EMS Engineering rights.',
      inputSchema: z.object({
        modelId: MODEL_ID,
        revision: z.number().int().positive().optional().describe('Model revision (default: the active one)'),
        catalogArtifactId: artifactIdSchema.optional().describe('Reuse a catalog returned by an earlier call'),
        proposalArtifactId: artifactIdSchema.optional().describe('Show the proposed tag of each slot'),
        kind: z.enum(['switch', 'tap', 'analog']).optional(),
        quantity: z.enum(['V', 'P', 'Q']).optional(),
        status: z.enum(['unbound', 'unapproved', 'bound', 'all']).optional().default('all'),
        substation: z.string().optional().describe('Case-insensitive regex on the substation name'),
        voltageLevel: z.string().optional().describe('Case-insensitive regex on the voltage level name'),
        name: z.string().optional().describe('Case-insensitive regex on element name, label or element_id'),
        limit: z.number().int().min(0).max(2000).optional().default(100),
        offset: z.number().int().min(0).optional().default(0),
        waitSeconds: waitSchema(120),
      }),
      annotations: { readOnlyHint: false, destructiveHint: false },
    },
    async (args) => {
      try {
        const reSub = compileRegex(args.substation, 'substation')
        const reVl = compileRegex(args.voltageLevel, 'voltageLevel')
        const reName = compileRegex(args.name, 'name')
        const cat: any = await elementCatalog(api, args.modelId, args.revision, args.catalogArtifactId, args.waitSeconds)
        if (cat.request) return requestOutcome(cat.request)
        const cur = await currentBindings(api, args.modelId)
        const bound = new Map<string, any>()
        for (const b of cur.bindings) {
          const k = slotKey(b)
          if (!bound.has(k) || (b['approved'] && !bound.get(k)['approved'])) bound.set(k, b)
        }
        const proposed = new Map<string, any>()
        if (args.proposalArtifactId) {
          const prop = await downloadJson(api, args.proposalArtifactId)
          if (Array.isArray(prop)) for (const b of prop) proposed.set(slotKey(b), b)
        }
        const status = (e: any) => {
          const b = bound.get(slotKey(e))
          return !b ? 'unbound' : b.approved ? 'bound' : 'unapproved'
        }
        const summary: Record<string, Record<string, number>> = {}
        const unboundBySub: Record<string, number> = {}
        for (const e of cat.entries) {
          const st = status(e)
          const k = (summary[e.kind] = summary[e.kind] || { total: 0, bound: 0, unapproved: 0, unbound: 0 })
          k['total']!++
          k[st]!++
          if (st === 'unbound') unboundBySub[e.substation || '(none)'] = (unboundBySub[e.substation || '(none)'] || 0) + 1
        }
        const rows = cat.entries.filter(
          (e: any) =>
            (!args.kind || e.kind === args.kind) &&
            (!args.quantity || e.quantity === args.quantity) &&
            (args.status === 'all' || status(e) === args.status) &&
            (!reSub || reSub.test(e.substation)) &&
            (!reVl || reVl.test(e.voltageLevel)) &&
            (!reName || reName.test(e.name) || reName.test(e.label) || reName.test(e.element_id))
        )
        return jsonResult({
          modelId: args.modelId,
          catalogArtifactId: cat.artifactId,
          ...(cat.modelRevision ? { modelRevision: cat.modelRevision } : {}),
          bindingRevision: cur.bindingRevision,
          summary,
          unboundPerSubstation: Object.entries(unboundBySub)
            .sort((a, b) => b[1] - a[1])
            .slice(0, 40)
            .map(([substation, unbound]) => ({ substation, unbound })),
          matched: rows.length,
          offset: args.offset,
          returned: Math.max(0, Math.min(rows.length - args.offset, args.limit)),
          elements: rows.slice(args.offset, args.offset + args.limit).map((e: any) => {
            const b = bound.get(slotKey(e))
            const p = proposed.get(slotKey(e))
            return {
              ...e,
              status: status(e),
              binding: b ? { tag: b.tag, approved: !!b.approved, sigma: b.sigma, sign: b.sign, closed_value: b.closed_value } : null,
              ...(args.proposalArtifactId ? { proposal: p ? { tag: p.tag, confidence: p.confidence } : null } : {}),
            }
          }),
        })
      } catch (e) {
        return errorResult('Error listing CIM-EMS model elements', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_candidate_tags',
    {
      title: 'CIM-EMS Candidate Tags',
      description:
        'List the supervised field points that can feed CIM-EMS (analog and digital, never CIM-EMS result points), ' +
        'with description, unit, groups (group1 is usually the substation), current value and quality, the quantity ' +
        'inferred like the automatic matcher does (unit kV/MW/Mvar, else the last P/Q/V/MW/MVAR/KV word of the tag, ' +
        'else the description) and the model values each is already bound to in the current binding revision. ' +
        'Use with cim_ems_list_model_elements to match tags to model objects: digital tags to switches, analog tags ' +
        'to P/Q/V slots and tap changers. Current values help to confirm a match (a closed breaker is 1, a P ' +
        'measurement close to the load flow result of the same branch end).',
      inputSchema: z.object({
        modelId: MODEL_ID.describe('Model whose current binding revision tells which tags are bound'),
        type: z.enum(['digital', 'analog']).optional(),
        quantity: z.enum(['V', 'P', 'Q']).optional().describe('Inferred quantity (analog tags)'),
        status: z.enum(['unbound', 'bound', 'all']).optional().default('unbound'),
        tag: z.string().optional().describe('Case-insensitive regex on the tag'),
        description: z.string().optional().describe('Case-insensitive regex on the description'),
        group1: z.string().optional().describe('Case-insensitive regex on group1 (station)'),
        group2: z.string().optional().describe('Case-insensitive regex on group2 (bay / voltage)'),
        limit: z.number().int().min(0).max(2000).optional().default(100),
        offset: z.number().int().min(0).optional().default(0),
      }),
      annotations: { readOnlyHint: true },
    },
    async (args) => {
      if (!mgr.status.HintMongoIsConnected) return textResult('Database not connected.', true)
      try {
        const q: any = {
          origin: 'supervised',
          type: args.type ? args.type : { $in: ['analog', 'digital'] },
          protocolSourceConnectionNumber: { $nin: await cimConnectionNumbers(mgr) },
        }
        if (args.tag) q.tag = compileRegex(args.tag, 'tag')
        if (args.description) q.description = compileRegex(args.description, 'description')
        if (args.group1) q.group1 = compileRegex(args.group1, 'group1')
        if (args.group2) q.group2 = compileRegex(args.group2, 'group2')
        const docs = await mgr
          .getRealtimeDataCollection()
          .find(q, {
            projection: { _id: 1, tag: 1, description: 1, type: 1, unit: 1, group1: 1, group2: 1, group3: 1, value: 1, invalid: 1, timeTag: 1 },
          })
          .sort({ tag: 1 })
          .toArray()
        const cur = await currentBindings(api, args.modelId)
        const byTag = new Map<string, any[]>()
        for (const b of cur.bindings) {
          const l = byTag.get(b.tag) || []
          l.push({ kind: b.kind, element_id: b.element_id, side: b.side, quantity: b.quantity, approved: !!b.approved })
          byTag.set(b.tag, l)
        }
        const rows = docs
          .map((d: any) => ({
            tag: d.tag,
            point_key: d._id,
            description: d.description,
            type: d.type,
            unit: d.unit,
            group1: d.group1,
            group2: d.group2,
            group3: d.group3,
            value: d.value,
            invalid: d.invalid,
            inferredQuantity: d.type === 'analog' ? quantityOfTag(d) : null,
            boundTo: byTag.get(d.tag) || [],
          }))
          .filter(
            (r) =>
              (!args.quantity || r.inferredQuantity === args.quantity) &&
              (args.status === 'all' || (args.status === 'bound') === r.boundTo.length > 0)
          )
        return jsonResult({
          bindingRevision: cur.bindingRevision,
          matched: rows.length,
          offset: args.offset,
          returned: Math.max(0, Math.min(rows.length - args.offset, args.limit)),
          tags: rows.slice(args.offset, args.offset + args.limit),
        })
      } catch (e) {
        return errorResult('Error listing CIM-EMS candidate tags', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_save_bindings',
    {
      title: 'CIM-EMS Save Bindings',
      description:
        'Build and save a new binding revision of a model. Starts from the current revision (base "current", or ' +
        'bindingRevision), from a proposal (base "proposal" + proposalArtifactId; approvals of identical entries in ' +
        'the current revision are carried over, as in the Admin UI) or from an empty list; then applies approve rules, ' +
        'removals and upserts (an upsert replaces the entry with the same kind/element_id/side/quantity). ' +
        'Every entry is checked against its point (tag exists, analog/digital type, unit vs quantity, not a CIM-EMS ' +
        'result point, one model value per tag, and with elementsArtifactId that the model slot exists); errors on ' +
        'approved entries block the save unless force. ' +
        'Returns the coverage, the checks and the differences against the current revision; with dryRun nothing is saved. ' +
        'Saving rebuilds every connection using the model at once (only approved bindings are used). ' +
        'Needs CIM-EMS Engineering rights.',
      inputSchema: z.object({
        modelId: MODEL_ID,
        base: z.enum(['current', 'proposal', 'empty']).optional().default('current'),
        bindingRevision: z.number().int().positive().optional().describe('base "current": revision to start from'),
        proposalArtifactId: artifactIdSchema.optional(),
        approve: z
          .object({
            all: z.boolean().optional(),
            kinds: z.array(z.enum(['switch', 'tap', 'analog'])).optional(),
            minConfidence: z.number().min(0).max(1).optional(),
            tags: z.array(z.string()).optional().describe('Exact tags to approve'),
          })
          .optional()
          .describe(
            'Approve the entries that match every given criterion (all=true approves everything)'
          ),
        unapproveTags: z.array(z.string()).optional().describe('Exact tags to set approved=false'),
        remove: z
          .array(
            z.object({
              kind: z.enum(['switch', 'tap', 'analog']).optional(),
              element_id: z.string().optional(),
              tag: z.string().optional(),
              quantity: z.string().optional(),
              side: z.number().int().optional(),
            })
          )
          .optional()
          .describe('Remove the entries matching every given field of an item'),
        upsert: z.array(bindingSchema).optional(),
        elementsArtifactId: artifactIdSchema
          .optional()
          .describe('Element catalog (cim_ems_list_model_elements) to check that every element_id/side/quantity exists'),
        force: z
          .boolean()
          .optional()
          .default(false)
          .describe('Save although approved entries have validation errors'),
        dryRun: z.boolean().optional().default(false),
        waitSeconds: waitSchema(120),
      }),
      annotations: { readOnlyHint: false, destructiveHint: true },
    },
    async (args) => {
      try {
        const cur = await currentBindings(api, args.modelId)
        let list: Record<string, any>[]
        if (args.base === 'proposal') {
          if (!args.proposalArtifactId)
            return textResult('base "proposal" needs proposalArtifactId.', true)
          const prop = await downloadJson(api, args.proposalArtifactId)
          if (!Array.isArray(prop)) return textResult('The artifact is not a binding proposal.', true)
          const approved = new Set(cur.bindings.filter((b) => b['approved']).map(fullKey))
          list = prop.map(cleanBinding).map((b) => ({ ...b, approved: approved.has(fullKey(b)) }))
        } else if (args.base === 'current') {
          list =
            args.bindingRevision && args.bindingRevision !== cur.bindingRevision
              ? (await currentBindings(api, args.modelId, args.bindingRevision)).bindings
              : cur.bindings.map((b) => ({ ...b }))
        } else list = []

        for (const rm of args.remove || []) {
          const keys = Object.entries(rm).filter(([, v]) => v !== undefined)
          if (!keys.length) continue
          list = list.filter((b) => !keys.every(([k, v]) => (k === 'side' ? Number(b[k] || 0) === v : b[k] === v)))
        }
        const upserts = (args.upsert || []).map(cleanBinding)
        for (const u of upserts) {
          u['proposed_from'] = u['proposed_from'] || 'manual'
          const i = list.findIndex((b) => slotKey(b) === slotKey(u))
          if (i >= 0) list[i] = { ...list[i], ...u }
          else list.push(u)
        }
        const a = args.approve
        if (a && (a.all || a.kinds || a.minConfidence !== undefined || a.tags)) {
          const tags = a.tags ? new Set(a.tags) : null
          for (const b of list)
            if (
              (!a.kinds || a.kinds.includes(b['kind'])) &&
              (a.minConfidence === undefined || (Number(b['confidence']) || 0) >= a.minConfidence) &&
              (!tags || tags.has(b['tag']))
            )
              b['approved'] = true
        }
        if (args.unapproveTags?.length) {
          const tags = new Set(args.unapproveTags)
          for (const b of list) if (tags.has(b['tag'])) b['approved'] = false
        }

        const catalog = args.elementsArtifactId
          ? (await elementCatalog(api, args.modelId, undefined, args.elementsArtifactId, 0) as any).entries
          : null
        const checks = await validateBindings(mgr, list, catalog)

        const before = new Map(cur.bindings.map((b) => [fullKey(b), b]))
        const after = new Map(list.map((b) => [fullKey(b), b]))
        const added = [...after.keys()].filter((k) => !before.has(k))
        const removed = [...before.keys()].filter((k) => !after.has(k))
        const approvalChanged = [...after.keys()].filter(
          (k) => before.has(k) && !!before.get(k)!['approved'] !== !!after.get(k)!['approved']
        )
        const modified = [...after.keys()].filter(
          (k) =>
            before.has(k) &&
            SETTING_KEYS.some(
              (f) => after.get(k)![f] !== undefined && before.get(k)![f] !== after.get(k)![f]
            )
        )
        const diff = {
          comparedTo: cur.bindingRevision,
          added: added.length,
          removed: removed.length,
          approvalChanged: approvalChanged.length,
          modified: modified.length,
          examples: {
            added: added.slice(0, 10),
            removed: removed.slice(0, 10),
            approvalChanged: approvalChanged.slice(0, 10),
            modified: modified.slice(0, 10),
          },
        }
        const summary = {
          modelId: args.modelId,
          count: list.length,
          approved: list.filter((b) => b['approved']).length,
          coverage: coverage(list),
          diff,
          checks: {
            errorCount: checks.errors.length,
            warningCount: checks.warnings.length,
            errors: checks.errors.slice(0, 50),
            warnings: checks.warnings.slice(0, 50),
            pointKeysSet: checks.pointKeysSet,
            elementsChecked: checks.elementsChecked,
          },
        }
        if (args.dryRun) return jsonResult({ dryRun: true, ...summary })
        if (!list.length) return textResult('Refusing to save an empty binding revision.', true)
        if (checks.errors.length && !args.force)
          return textResult(
            JSON.stringify({ saved: false, reason: 'validation errors on approved entries (fix them, unapprove them or pass force)', ...summary }, null, 2),
            true
          )
        const req = await queue(api, { type: 'saveBindings', modelId: args.modelId, bindings: list }, args.waitSeconds)
        return textResult(JSON.stringify({ ...summary, request: req }, null, 2), req.status === 'failed')
      } catch (e) {
        return errorResult('Error saving CIM-EMS bindings', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_get_contingency_result',
    {
      title: 'CIM-EMS Get Contingency Result',
      description:
        'Full contingency analysis result of a run (every contingency, not only the published ones): base case ' +
        'status and violations, counts per outcome, and the contingencies sorted by worst loading with their ' +
        'violations (subject element with its name, limit type, side, value, limit, loading %) and disconnected ' +
        'elements, plus mostViolatedElements (elements violated by the most contingencies: the weak points of the grid). ' +
        'Give artifactId, or ' +
        'connectionNumber (+ runId, default the latest run that analysed contingencies). Needs CIM-EMS Engineering rights.',
      inputSchema: z.object({
        connectionNumber: z.number().int().positive().optional(),
        runId: z.number().int().positive().optional(),
        artifactId: artifactIdSchema.optional(),
        onlyInsecure: z.boolean().optional().default(true),
        contingency: z.string().optional().describe('Case-insensitive regex on contingency id or name'),
        limit: z.number().int().min(1).max(1000).optional().default(30),
        maxViolationsPerContingency: z.number().int().min(0).max(200).optional().default(10),
      }),
      annotations: { readOnlyHint: true },
    },
    async (args) => {
      try {
        const reCon = compileRegex(args.contingency, 'contingency')
        let artifactId = args.artifactId
        let run: any = null
        if (!artifactId) {
          if (!args.connectionNumber) return textResult('Give artifactId or connectionNumber.', true)
          const runs: any[] =
            (await api.get(API + 'listRuns', { connectionNumber: args.connectionNumber, limit: 1000 }))?.runs || []
          run = runs.find(
            (r) => (!args.runId || r.runId === args.runId) && r.summary?.contingency?.artifactId
          )
          if (!run)
            return textResult(
              args.runId
                ? `Run ${args.runId} has no contingency result (skipped, failed or expired).`
                : 'No run with a contingency result found for this connection.',
              true
            )
          artifactId = String(run.summary.contingency.artifactId)
        }
        const doc = await downloadJson(api, artifactId!)
        const outcomes: any[] = Array.isArray(doc?.outcomes) ? doc.outcomes : []
        const names = await elementNames(mgr, run?.connectionNumber)
        const named = (id: string) => {
          const n = names.get(id)
          if (!n) return {}
          return {
            subjectName: n.name,
            ...(n.substation ? { substation: n.substation } : {}),
            ...(n.voltageLevel ? { voltageLevel: n.voltageLevel } : {}),
          }
        }
        const violation = (v: any) => ({
          ...v,
          ...named(v.subject),
          ...(typeof v.value === 'number' && typeof v.limit === 'number' && v.limit
            ? { loadingPct: Math.round((1000 * v.value) / v.limit) / 10 }
            : {}),
        })
        const baseViolations: any[] = doc?.baseViolations || []
        const perStatus: Record<string, number> = {}
        for (const o of outcomes) perStatus[o.status] = (perStatus[o.status] || 0) + 1
        const weak = new Map<string, any>()
        for (const o of outcomes)
          for (const v of o.violations || []) {
            const k = `${v.subject}|${v.type}`
            const w = weak.get(k) || { subject: v.subject, type: v.type, ...named(v.subject), contingencies: 0, worstLoadingPct: null, worstContingency: null }
            w.contingencies++
            const pct = typeof v.value === 'number' && typeof v.limit === 'number' && v.limit ? (100 * v.value) / v.limit : null
            if (pct !== null && (w.worstLoadingPct === null || pct > w.worstLoadingPct)) {
              w.worstLoadingPct = Math.round(pct * 10) / 10
              w.worstContingency = o.name || o.id
            }
            weak.set(k, w)
          }
        const rows = outcomes
          .filter((o) => !args.onlyInsecure || CA_INSECURE.includes(o.status))
          .filter((o) => !reCon || reCon.test(o.id) || reCon.test(o.name || ''))
          .sort((a, b) => (b.worstLoading ?? -1) - (a.worstLoading ?? -1))
        return jsonResult({
          artifactId,
          ...(run ? { runId: run.runId, runCreatedAt: run.createdAt } : {}),
          durationMs: doc?.durationMs,
          baseStatus: doc?.baseStatus,
          baseViolationCount: baseViolations.length,
          baseViolations: baseViolations.slice(0, args.limit).map(violation),
          note: 'Violations already present in the base case are not counted against a contingency.',
          contingencies: outcomes.length,
          perStatus,
          mostViolatedElements: [...weak.values()]
            .sort((a, b) => b.contingencies - a.contingencies || (b.worstLoadingPct ?? 0) - (a.worstLoadingPct ?? 0))
            .slice(0, 15),
          matched: rows.length,
          returned: Math.min(rows.length, args.limit),
          outcomes: rows.slice(0, args.limit).map((o) => ({
            id: o.id,
            name: o.name,
            status: o.status,
            worstLoadingPct: typeof o.worstLoading === 'number' ? Math.round(o.worstLoading * 10) / 10 : null,
            violationCount: (o.violations || []).length,
            violations: (o.violations || []).slice(0, args.maxViolationsPerContingency).map(violation),
            disconnectedCount: (o.disconnected || []).length,
            disconnected: (o.disconnected || []).slice(0, 20).map((id: string) => ({ id, ...named(id) })),
          })),
        })
      } catch (e) {
        return errorResult('Error reading CIM-EMS contingency result', e)
      }
    }
  )
  server.registerTool(
    'cim_ems_get_estimation_report',
    {
      title: 'CIM-EMS State Estimation Report',
      description:
        'Measurement-level report of the latest state estimate of a connection: for every measurement used ' +
        '(and every one removed as bad data) the SCADA tag, element name, quantity, side, measured and estimated ' +
        'values, sigma and normalized residual, with hints for typical binding errors (sign, scale, open element). ' +
        'Summaries per quantity (bias = mean normalized residual, RMS, max) and per substation, the chi-square ' +
        'test, and the measurements dropped before the estimate (invalid, stale, time skew). ' +
        'Needs CIM-EMS Engineering rights.',
      inputSchema: z.object({
        connectionNumber: z.number().int().positive(),
        quantity: z.enum(['V', 'P', 'Q', 'I']).optional(),
        substation: z.string().optional().describe('Case-insensitive regex on the substation of the element'),
        tag: z.string().optional().describe('Case-insensitive regex on the tag'),
        minAbsNormalized: z.number().min(0).optional().describe('Only measurements with |normalized residual| >= this'),
        onlyBadData: z.boolean().optional().default(false),
        limit: z.number().int().min(0).max(2000).optional().default(30),
      }),
      annotations: { readOnlyHint: true },
    },
    async (args) => {
      try {
        const reSub = compileRegex(args.substation, 'substation')
        const reTag = compileRegex(args.tag, 'tag')
        const runs: any[] = (await api.get(API + 'listRuns', { connectionNumber: args.connectionNumber, limit: 50 }))?.runs || []
        const run = runs.find((r) => r.summary?.estimation?.residualsArtifactId)
        if (!run) {
          const last = runs[0]?.summary?.estimation
          return textResult(
            last
              ? `No residual report: the latest estimation is '${last.status}'${last.reason ? ` (${last.reason})` : ''}.`
              : 'No state estimation run found for this connection (runStateEstimation off or driver not running).',
            true
          )
        }
        const doc = await downloadJson(api, String(run.summary.estimation.residualsArtifactId))
        const names = await elementNames(mgr, args.connectionNumber)
        const rows = ((doc?.measurements || []) as any[]).map((m) => {
          const n = names.get(m.element) || (m.bus ? names.get(m.bus) : undefined)
          const hints = Math.abs(m.normalized ?? 0) >= 3 || m.bad ? residualHints(m) : []
          return {
            ...m,
            name: n?.name ?? null,
            substation: n?.substation ?? '',
            voltageLevel: n?.voltageLevel ?? '',
            residual: typeof m.measured === 'number' && typeof m.estimated === 'number' ? Math.round((m.measured - m.estimated) * 1e4) / 1e4 : null,
            ...(hints.length ? { hints } : {}),
          }
        })
        const thr = Number(doc?.badDataThreshold ?? 4)
        const stats = (list: any[]) => {
          const v = list.filter((x) => !x.bad && typeof x.normalized === 'number').map((x) => x.normalized as number)
          if (!v.length) return { count: list.length }
          const mean = v.reduce((a, b) => a + b, 0) / v.length
          const rms = Math.sqrt(v.reduce((a, b) => a + b * b, 0) / v.length)
          return {
            count: list.length,
            meanNormalized: Math.round(mean * 1000) / 1000,
            rmsNormalized: Math.round(rms * 1000) / 1000,
            maxAbsNormalized: Math.round(Math.max(...v.map(Math.abs)) * 1000) / 1000,
            suspect: v.filter((x) => Math.abs(x) >= 3).length,
            bad: list.filter((x) => x.bad).length,
          }
        }
        const perQuantity: Record<string, any> = {}
        for (const q of [...new Set(rows.map((r) => r.quantity))]) perQuantity[q] = stats(rows.filter((r) => r.quantity === q))
        const subs = new Map<string, any[]>()
        for (const r of rows) {
          const k = r.substation || '(unknown)'
          subs.set(k, [...(subs.get(k) || []), r])
        }
        const perSubstation = [...subs.entries()]
          .map(([substation, l]) => ({ substation, ...stats(l) }))
          .sort(
            (a: any, b: any) =>
              (b.suspect ?? 0) + (b.bad ?? 0) - (a.suspect ?? 0) - (a.bad ?? 0) ||
              (b.rmsNormalized ?? 0) * Math.sqrt(b.count) - (a.rmsNormalized ?? 0) * Math.sqrt(a.count)
          )
          .slice(0, 15)
        const dropped: Record<string, number> = {}
        for (const r of (doc?.snapshotReasons || []) as string[]) {
          const m = /analog dropped \((.*?)(?: than.*)?\)$/.exec(r)
          const k = m ? m[1]! : 'other'
          dropped[k] = (dropped[k] || 0) + 1
        }
        const findings: string[] = []
        if (doc?.chi2Passed === false) findings.push(`Chi-square test failed (statistic ${Math.round(doc.chi2Statistic)} > threshold ${Math.round(doc.chi2Threshold)}): remaining inconsistent measurements or sigmas too small.`)
        const badCount = rows.filter((r) => r.bad).length
        if (badCount) findings.push(`${badCount} measurement(s) removed as bad data (|r| > ${thr}).`)
        for (const [q, st] of Object.entries(perQuantity))
          if (typeof st.meanNormalized === 'number' && Math.abs(st.meanNormalized) > 1 && st.count >= 5)
            findings.push(`Systematic bias on ${q} measurements (mean normalized residual ${st.meanNormalized}): check sign convention, scaling or sigma.`)
        const top = perSubstation[0]
        if (top && (top.suspect ?? 0) + (top.bad ?? 0) >= 3)
          findings.push(`Residuals concentrate in ${top.substation} (${(top.suspect ?? 0) + (top.bad ?? 0)} suspect/bad): check switch states there before blaming meters.`)
        const hinted = rows.filter((r) => r.hints).length
        if (hinted) findings.push(`${hinted} suspect measurement(s) carry hints (sign, scale or topology).`)
        const droppedTotal = Object.values(dropped).reduce((a, b) => a + b, 0)
        if (droppedTotal) findings.push(`${droppedTotal} bound measurement(s) not used (${Object.entries(dropped).map(([k, n]) => `${n} ${k}`).join(', ')}).`)
        if (!findings.length) findings.push('Estimate consistent: no bad data, no suspect measurements, chi-square passed.')

        const list = rows
          .filter(
            (r) =>
              (!args.quantity || r.quantity === args.quantity) &&
              (!reSub || reSub.test(r.substation)) &&
              (!reTag || reTag.test(r.tag || '')) &&
              (!args.onlyBadData || r.bad) &&
              (args.minAbsNormalized === undefined || Math.abs(r.normalized ?? 0) >= args.minAbsNormalized)
          )
          .sort((a, b) => Number(b.bad) - Number(a.bad) || Math.abs(b.normalized ?? 0) - Math.abs(a.normalized ?? 0))
        return jsonResult({
          runId: doc?.runId,
          runCreatedAt: run.createdAt,
          snapshotTime: doc?.snapshotTime,
          chi2: {
            statistic: doc?.chi2Statistic,
            threshold: doc?.chi2Threshold,
            ratio: run.summary.estimation.chi2Ratio ?? null,
            passed: doc?.chi2Passed,
            degreesOfFreedom: doc?.degreesOfFreedom,
          },
          badDataThreshold: thr,
          findings,
          perQuantity,
          perSubstation,
          droppedBeforeEstimate: dropped,
          droppedExamples: (doc?.snapshotReasons || []).slice(0, 10),
          matched: list.length,
          returned: Math.min(list.length, args.limit),
          measurements: list.slice(0, args.limit),
        })
      } catch (e) {
        return errorResult('Error reading CIM-EMS estimation report', e)
      }
    }
  )

  server.registerTool(
    'cim_ems_run_statistics',
    {
      title: 'CIM-EMS Run Statistics',
      description:
        'Behaviour of the CIM-EMS studies over the latest runs of a connection (up to 1000, kept 7 days): load flow ' +
        'convergence rate, iterations and durations, topology changes, rejected snapshots and their reasons, system ' +
        'totals (generation, load, losses) min/max/last, state estimation convergence, chi-square pass rate and the ' +
        'measurements most often removed as bad data (chronic bad meters), contingency runs with insecure ' +
        'counts and the contingencies most often insecure. Use it to tell a one-off event from a persistent problem.',
      inputSchema: z.object({
        connectionNumber: z.number().int().positive(),
        limit: z.number().int().min(2).max(1000).optional().default(200),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ connectionNumber, limit }) => {
      try {
        const runs: any[] = (await api.get(API + 'listRuns', { connectionNumber, limit }))?.runs || []
        if (!runs.length) return textResult('No runs recorded for this connection.', true)
        const n = runs.length
        const pct = (k: number) => Math.round((1000 * k) / n) / 10
        const count = (f: (r: any) => boolean) => runs.filter(f).length
        const num = (v: number[]) => {
          if (!v.length) return null
          const s = [...v].sort((a, b) => a - b)
          const q = (p: number) => s[Math.min(s.length - 1, Math.floor(p * s.length))]
          return { min: s[0], p50: q(0.5), p95: q(0.95), max: s[s.length - 1] }
        }
        const byStatus: Record<string, number> = {}
        const byTrigger: Record<string, number> = {}
        for (const r of runs) {
          byStatus[r.status] = (byStatus[r.status] || 0) + 1
          byTrigger[r.trigger] = (byTrigger[r.trigger] || 0) + 1
        }
        let topoChanges = 0
        for (let i = 1; i < runs.length; i++)
          if (runs[i].topologyHash && runs[i - 1].topologyHash && runs[i].topologyHash !== runs[i - 1].topologyHash) topoChanges++
        const reasons: Record<string, number> = {}
        for (const r of runs)
          for (const x of (r.summary?.reasons || []) as string[]) {
            const k = x.replace(/^.*?: /, '')
            reasons[k] = (reasons[k] || 0) + 1
          }
        const totals = runs.map((r) => r.summary?.totals).filter(Boolean)
        const tstat = (k: string) => {
          const v = totals.map((t: any) => t[k]).filter((x: any) => typeof x === 'number')
          return v.length ? { last: v[0], min: Math.min(...v), max: Math.max(...v) } : null
        }
        const se = runs.map((r) => r.summary?.estimation).filter(Boolean)
        const seStatus: Record<string, number> = {}
        const badTags = new Map<string, any>()
        for (const e of se) {
          seStatus[e.status] = (seStatus[e.status] || 0) + 1
          for (const b of e.badData || []) {
            const k = `${b.tag || b.element}|${b.quantity}|${b.side}`
            const w = badTags.get(k) || { tag: b.tag || null, element: b.element, quantity: b.quantity, side: b.side, times: 0, lastNormalized: b.normalized }
            w.times++
            badTags.set(k, w)
          }
        }
        const seConv = se.filter((e: any) => e.status === 'converged')
        const ca = runs.map((r) => r.summary?.contingency).filter((c: any) => c && c.status === 'done')
        const insecureIds = new Map<string, number>()
        for (const c of ca) for (const id of c.insecure || []) insecureIds.set(id, (insecureIds.get(id) || 0) + 1)
        const names = await elementNames(mgr, connectionNumber)
        return jsonResult({
          runs: n,
          from: runs[n - 1].createdAt,
          to: runs[0].createdAt,
          loadFlow: {
            byStatus,
            convergedPct: pct(count((r) => r.status === 'converged')),
            byTrigger,
            iterations: num(runs.map((r) => r.summary?.iterations).filter((x) => typeof x === 'number')),
            durationMs: num(runs.map((r) => r.durationMs).filter((x) => typeof x === 'number')),
            topologyChanges: topoChanges,
            distinctTopologies: new Set(runs.map((r) => r.topologyHash).filter(Boolean)).size,
            topologyUncertainPct: pct(count((r) => (r.summary?.flags || []).includes('topologyUncertain'))),
            islands: num(runs.map((r) => r.summary?.islands).filter((x) => typeof x === 'number')),
            frequentSnapshotReasons: Object.entries(reasons)
              .sort((a, b) => b[1] - a[1])
              .slice(0, 10)
              .map(([reason, runs]) => ({ reason, runs })),
          },
          totals: totals.length
            ? { generationMW: tstat('generationMW'), loadMW: tstat('loadMW'), lossesMW: tstat('lossesMW'), lossesPct: tstat('lossesPct') }
            : null,
          stateEstimation: se.length
            ? {
                runs: se.length,
                byStatus: seStatus,
                chi2PassedPct: seConv.length ? Math.round((1000 * seConv.filter((e: any) => e.chi2Passed).length) / seConv.length) / 10 : null,
                chi2Ratio: num(seConv.map((e: any) => e.chi2Ratio).filter((x: any) => typeof x === 'number')),
                measurements: num(seConv.map((e: any) => e.measurements).filter((x: any) => typeof x === 'number')),
                runsWithBadData: se.filter((e: any) => (e.badData || []).length).length,
                chronicBadData: [...badTags.values()].sort((a, b) => b.times - a.times).slice(0, 15),
              }
            : null,
          contingency: ca.length
            ? {
                analyses: ca.length,
                insecure: num(ca.map((c: any) => (c.insecure || []).length)),
                lastInsecure: (ca[0].insecure || []).length,
                mostOftenInsecure: [...insecureIds.entries()]
                  .sort((a, b) => b[1] - a[1])
                  .slice(0, 15)
                  .map(([id, times]) => ({ id, name: names.get(id)?.name ?? null, times })),
                durationMs: num(ca.map((c: any) => c.durationMs).filter((x: any) => typeof x === 'number')),
              }
            : null,
        })
      } catch (e) {
        return errorResult('Error computing CIM-EMS run statistics', e)
      }
    }
  )
}
