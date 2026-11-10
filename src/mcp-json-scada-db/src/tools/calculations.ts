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

// Calculated tags of the compiled calculations engine (src/calculations):
// the guide and formula catalog, a design check with a value preview, an
// audit of all calculated points and (admin API) a checked create/update.

import { z } from 'zod'
import type { McpServer } from '@modelcontextprotocol/server'
import type { ConnectionManager } from '../jsonscada/connection-manager.js'
import type { AdminApiClient } from '../jsonscada/admin-api.js'
import {
  CALCULATIONS_GUIDE,
  CALCULATIONS_PROCESS,
  CALC_FORMULAS,
  CALC_RELOAD_SECONDS,
  calculate,
  type FormulaDef,
  checkFormula,
  parcelRuleText,
  processedValue,
} from '../jsonscada/calculations.js'
import { aliveInfo } from './status.js'
import { buildDoc, findTag } from './tags.js'
import { RESERVED_NAME_MESSAGE, isSystemTagName } from '../jsonscada/system-tags.js'
import { errorResult, jsonResult, notConnectedResult, textResult } from './util.js'

const POINT_PROJECTION = {
  _id: 1,
  tag: 1,
  type: 1,
  origin: 1,
  formula: 1,
  parcels: 1,
  value: 1,
  invalid: 1,
  timeTag: 1,
  kconv1: 1,
  kconv2: 1,
  unit: 1,
}

const MAX_CHAIN_DEPTH = 50

const NEXT_STEP =
  `The calculations engine picks up the point within ${CALC_RELOAD_SECONDS} s (it reloads calculated points ` +
  'periodically). Then run check_calculated_tag on the tag to confirm the stored value matches the preview.'

const parcelRef = z.union([z.string().min(1), z.number().int()])
const parcelsSchema = z
  .array(parcelRef)
  .describe('Parcels in formula order (P1, P2, ...): tag names, or point keys (_id numbers)')

function formulaInfo(def: FormulaDef) {
  return {
    code: def.code,
    name: def.name,
    expression: def.expression,
    parcels: parcelRuleText(def.parcels),
    result: def.result,
    quality: def.quality,
    ...(def.legacy ? { legacy: true } : {}),
    ...(def.notes ? { notes: def.notes } : {}),
  }
}

const parcelKeysOf = (doc: any): number[] =>
  Array.isArray(doc?.parcels) ? doc.parcels.map((p: any) => Number(p)) : []

// computed by the engine: origin "calculated" and a formula code
const isCalculated = (doc: any) => Number(doc?.formula) > 0 && doc?.origin === 'calculated'

async function calculationsProcessStatus(mgr: ConnectionManager) {
  const now = Date.now()
  const docs = await mgr
    .getProcessInstancesCollection()
    .find({ processName: CALCULATIONS_PROCESS })
    .toArray()
  const instances = docs.map((d: any) => ({
    processInstanceNumber: d.processInstanceNumber,
    enabled: d.enabled !== false,
    ...aliveInfo(d, now),
    alive: aliveInfo(d, now)['alive'] === true,
  }))
  const warnings: string[] = []
  if (instances.length === 0)
    warnings.push(
      'No CALCULATIONS process instance is registered: the calculations process has never run on this database'
    )
  else if (!instances.some((i) => i.alive))
    warnings.push(
      'The calculations process is not running (stale keep-alive): calculated values are frozen'
    )
  return { instances, warnings }
}

interface ParcelRow {
  position: string
  ref: string | number
  pointKey?: number
  doc?: any
}

// Resolves tag names / point keys to realtimeData documents, keeping order
async function resolveParcels(mgr: ConnectionManager, refs: (string | number)[]): Promise<ParcelRow[]> {
  const coll = mgr.getRealtimeDataCollection()
  const keys = refs.filter((r): r is number => typeof r === 'number')
  const names = refs.filter((r): r is string => typeof r === 'string')
  const docs = await coll
    .find({ $or: [{ _id: { $in: keys } as any }, { tag: { $in: names } }] })
    .project(POINT_PROJECTION)
    .toArray()
  const byKey = new Map(docs.map((d: any) => [Number(d._id), d]))
  const byTag = new Map(docs.map((d: any) => [d.tag, d]))
  return refs.map((ref, i) => {
    const doc = typeof ref === 'number' ? byKey.get(ref) : byTag.get(ref)
    return {
      position: `P${i + 1}`,
      ref,
      ...(doc ? { pointKey: Number(doc._id), doc } : typeof ref === 'number' ? { pointKey: ref } : {}),
    }
  })
}

// Follows chains of calculated parcels from the given keys; reports whether
// targetKey is reached (feedback loop) and the longest chain length.
async function chainFrom(
  mgr: ConnectionManager,
  start: ParcelRow[],
  targetKey: number | undefined
): Promise<{ loop: boolean; depth: number }> {
  const coll = mgr.getRealtimeDataCollection()
  let frontier = start.filter((p) => p.doc && isCalculated(p.doc)).map((p) => p.doc)
  const seen = new Set<number>(frontier.map((d) => Number(d._id)))
  let depth = 0
  while (frontier.length && depth < MAX_CHAIN_DEPTH) {
    depth++
    const next = [...new Set(frontier.flatMap(parcelKeysOf))]
    if (targetKey !== undefined && next.includes(targetKey)) return { loop: true, depth }
    const docs = await coll
      .find({ _id: { $in: next.filter((k) => !seen.has(k)) } as any, formula: { $gt: 0 }, origin: 'calculated' } as any)
      .project({ _id: 1, formula: 1, parcels: 1, origin: 1 })
      .toArray()
    docs.forEach((d: any) => seen.add(Number(d._id)))
    frontier = docs
  }
  return { loop: false, depth }
}

// Checks a design (formula + resolved parcels) for a target point (existing
// document or the type it will have) and previews the result.
async function analyze(
  mgr: ConnectionManager,
  formula: number,
  parcels: ParcelRow[],
  target: { key?: number; type?: string | undefined; doc?: any }
) {
  const { def, errors, warnings } = checkFormula(formula, parcels.length)
  const notes: string[] = []

  for (const p of parcels) {
    if (!p.doc) {
      errors.push(
        `${p.position} ${JSON.stringify(p.ref)} not found in realtimeData` +
          (typeof p.ref === 'number' ? ' (the engine counts it as invalid, so the result stays invalid)' : '')
      )
      continue
    }
    if (target.key !== undefined && p.pointKey === target.key)
      errors.push(`${p.position} is the calculated point itself (feedback loop)`)
    if (p.doc.type === 'string' || p.doc.type === 'json')
      warnings.push(`${p.position} ${p.doc.tag} is a ${p.doc.type} point; the engine reads only its numeric value`)
    if (isCalculated(p.doc))
      notes.push(`${p.position} ${p.doc.tag} is itself calculated (formula ${p.doc.formula}): adds up to one cycle of delay`)
  }
  const keys = parcels.map((p) => p.pointKey).filter((k) => k !== undefined)
  if (new Set(keys).size !== keys.length) warnings.push('the same point is used more than once as a parcel')

  if (parcels.some((p) => p.doc && isCalculated(p.doc))) {
    const chain = await chainFrom(mgr, parcels, target.key)
    if (chain.loop) errors.push('a calculated parcel depends on this point: feedback loop through a chain of calculations')
    else if (chain.depth >= MAX_CHAIN_DEPTH) warnings.push(`chain of calculated parcels deeper than ${MAX_CHAIN_DEPTH}`)
    else notes.push(`longest chain of calculated parcels: ${chain.depth + 1} levels (up to ${chain.depth + 1} cycles of delay)`)
  }

  const type = target.doc?.type ?? target.type
  if (def && type) {
    if (def.result === 'digital' && type !== 'digital')
      warnings.push(`formula ${def.code} gives a 0/1 result; a digital point is the natural type (target is ${type})`)
    if (def.result === 'analog' && type === 'digital')
      warnings.push(`formula ${def.code} gives a numeric result; on a digital point any value other than 0 shows as ON`)
  }
  if (target.doc) {
    if (target.doc.origin !== 'calculated')
      errors.push(`the point has origin "${target.doc.origin}": the engine computes only points with origin "calculated"`)
    const k1 = target.doc.kconv1 ?? 1
    const k2 = target.doc.kconv2 ?? 0
    if (type === 'analog' && (k1 !== 1 || k2 !== 0))
      notes.push(`kconv1 = ${k1}, kconv2 = ${k2} are applied to the result by the data processor`)
    if (type === 'digital' && k1 === -1) notes.push('kconv1 = -1 inverts the result')
  }

  let preview: Record<string, unknown> | undefined
  if (def && errors.length === 0) {
    const vals = parcels.map((p) => Number(p.doc?.value ?? 0))
    const invs = parcels.map((p) => p.doc?.invalid === true)
    const engine = calculate(def, vals, invs)
    const stored = processedValue(engine, {
      type,
      kconv1: target.doc?.kconv1 ?? 1,
      kconv2: target.doc?.kconv2 ?? 0,
    })
    preview = { fromCurrentParcelValues: true, engineOutput: engine, storedAs: stored }
    if (target.doc && isCalculated(target.doc)) {
      const cur = Number(target.doc.value)
      const same =
        Math.abs(cur - stored.value) <= 1e-6 * Math.max(1, Math.abs(stored.value)) &&
        (target.doc.invalid === true) === stored.invalid
      preview['storedNow'] = { value: target.doc.value, invalid: target.doc.invalid, timeTag: target.doc.timeTag ?? null }
      preview['matchesStored'] = same
      if (!target.doc.timeTag)
        warnings.push(`the point has never been updated: wait ${CALC_RELOAD_SECONDS} s after creating it, and check that the calculations and data processor processes run`)
      else if (!same)
        notes.push(`the stored value differs from the preview: parcels may have changed within the last cycle, or the formula/parcels were changed less than ${CALC_RELOAD_SECONDS} s ago`)
    }
  }
  return { def, errors, warnings, notes, preview }
}

const parcelView = (p: ParcelRow) => ({
  position: p.position,
  ref: p.ref,
  ...(p.doc
    ? {
        pointKey: p.pointKey,
        tag: p.doc.tag,
        type: p.doc.type,
        origin: p.doc.origin,
        value: p.doc.value,
        invalid: p.doc.invalid,
        unit: p.doc.unit,
        ...(isCalculated(p.doc) ? { formula: p.doc.formula } : {}),
      }
    : { found: false }),
})

export function registerCalculationsReadTools(server: McpServer, mgr: ConnectionManager) {
  server.registerTool(
    'get_calculations_guide',
    {
      title: 'Calculated Tags Guide',
      description:
        'How to create calculated tags with the compiled calculations engine: how the engine works (cycle, ' +
        'periodic reload of calculated points), the point fields (origin, formula, parcels), pitfalls, and the ' +
        'table of formula codes with their parcel order and quality rules. With formula, returns that one entry. ' +
        'Read it before designing or changing calculated points.',
      inputSchema: z.object({
        formula: z.number().int().optional().describe('Formula code to describe'),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ formula }) => {
      if (formula === undefined) return textResult(CALCULATIONS_GUIDE)
      const def = CALC_FORMULAS.get(formula)
      if (!def) {
        const { errors, warnings } = checkFormula(formula, 0)
        return textResult([...errors, ...warnings].join('\n') || `Formula ${formula} not found.`, true)
      }
      return jsonResult(formulaInfo(def))
    }
  )

  server.registerTool(
    'check_calculated_tag',
    {
      title: 'Check Calculated Tag',
      description:
        'Validate a calculated point design or an existing calculated tag against the calculations engine: ' +
        'formula code, parcel count and order, parcels that exist, self references and loops, result type, and the ' +
        'calculations process status. Previews the value the engine would write from the current parcel values. ' +
        'Give tag (existing point; its formula/parcels unless overridden) and/or formula + parcels.',
      inputSchema: z.object({
        tag: z.string().optional().describe('Existing tag to check, or the tag being designed'),
        formula: z.number().int().optional().describe('Formula code (overrides the stored one)'),
        parcels: parcelsSchema.optional(),
        type: z.enum(['analog', 'digital']).optional().describe('Type of the point to create (ignored for existing tags)'),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ tag, formula, parcels, type }) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()
      try {
        const coll = mgr.getRealtimeDataCollection()
        const doc: any = tag ? await coll.findOne({ tag }, { projection: POINT_PROJECTION }) : null
        const code = formula ?? (doc ? Number(doc.formula) : undefined)
        if (code === undefined) return textResult('Give formula (and parcels), or an existing tag.', true)
        const refs: (string | number)[] = parcels ?? parcelKeysOf(doc)
        const rows = await resolveParcels(mgr, refs)
        const target = { ...(doc ? { key: Number(doc._id), doc } : {}), type }
        const r = await analyze(mgr, code, rows, target)
        const proc = await calculationsProcessStatus(mgr)
        return jsonResult({
          ...(doc
            ? {
                point: {
                  tag: doc.tag,
                  pointKey: doc._id,
                  type: doc.type,
                  origin: doc.origin,
                  formula: doc.formula,
                  parcels: doc.parcels,
                },
              }
            : tag
              ? { point: { tag, exists: false } }
              : {}),
          formula: r.def ? formulaInfo(r.def) : { code },
          parcels: rows.map(parcelView),
          ok: r.errors.length === 0,
          errors: r.errors,
          warnings: [...r.warnings, ...proc.warnings],
          notes: r.notes,
          ...(r.preview ? { preview: r.preview } : {}),
          calculationsProcess: proc.instances,
        })
      } catch (e) {
        return errorResult('Error checking calculated tag', e)
      }
    }
  )

  server.registerTool(
    'audit_calculated_tags',
    {
      title: 'Audit Calculated Tags',
      description:
        'Scan all calculated points (formula > 0 or origin "calculated") for problems the calculations engine has ' +
        'with them: unknown formula codes, wrong parcel counts (never computed), missing parcels (result stays ' +
        'invalid), self references and loops, formulas on non-calculated points (ignored), points never updated. ' +
        'Also reports the calculations process status and formula usage.',
      inputSchema: z.object({
        tagFilter: z.string().optional().describe('Case-insensitive regex on the tag name'),
        onlyProblems: z.boolean().optional().default(true),
        limit: z.number().int().min(1).max(2000).optional().default(100),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ tagFilter, onlyProblems, limit }) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()
      try {
        const coll = mgr.getRealtimeDataCollection()
        const query: any = { $or: [{ formula: { $gt: 0 } }, { origin: 'calculated' }] }
        const calcs: any[] = await coll.find(query).project(POINT_PROJECTION).toArray()
        const byKey = new Map<number, any>(calcs.map((d) => [Number(d._id), d]))
        const missing = [...new Set(calcs.flatMap(parcelKeysOf))].filter((k) => !byKey.has(k))
        const parcelDocs: any[] = missing.length
          ? await coll.find({ _id: { $in: missing } as any }).project({ _id: 1, tag: 1, type: 1 }).toArray()
          : []
        const exists = new Set<number>([...byKey.keys(), ...parcelDocs.map((d) => Number(d._id))])
        const typeOf = new Map<number, string>(
          [...calcs, ...parcelDocs].map((d) => [Number(d._id), d.type])
        )

        // loops and chain depth over the calculated points
        const state = new Map<number, 1 | 2>() // 1 visiting, 2 done
        const depth = new Map<number, number>()
        const inLoop = new Set<number>()
        const visit = (k: number, stack: number[]): number => {
          const s = state.get(k)
          if (s === 2) return depth.get(k) ?? 0
          if (s === 1) {
            stack.slice(stack.indexOf(k)).forEach((x) => inLoop.add(x))
            return 0
          }
          state.set(k, 1)
          let d = 0
          for (const p of parcelKeysOf(byKey.get(k))) {
            const pd = byKey.get(p)
            if (pd && isCalculated(pd)) d = Math.max(d, visit(p, [...stack, k]) + 1)
          }
          state.set(k, 2)
          depth.set(k, d)
          return d
        }
        calcs.filter(isCalculated).forEach((d) => visit(Number(d._id), []))
        // a depth through a loop is meaningless
        inLoop.forEach((k) => depth.delete(k))

        const re = tagFilter ? new RegExp(tagFilter, 'i') : null
        const byFormula: Record<string, number> = {}
        const rows: any[] = []
        let withProblems = 0
        for (const d of calcs) {
          const key = Number(d._id)
          const parcels = parcelKeysOf(d)
          const errors: string[] = []
          const warnings: string[] = []
          if (isCalculated(d)) {
            byFormula[String(d.formula)] = (byFormula[String(d.formula)] ?? 0) + 1
            const chk = checkFormula(d.formula, parcels.length)
            errors.push(...chk.errors)
            warnings.push(...chk.warnings.filter((w) => !w.includes('legacy formula')))
            const absent = parcels.filter((p) => !exists.has(p))
            if (absent.length) errors.push(`parcels not found (the result stays invalid): ${absent.join(', ')}`)
            if (parcels.includes(key)) errors.push('the point is its own parcel')
            else if (inLoop.has(key)) errors.push('part of a loop of calculated points')
            const odd = parcels.filter((p) => ['string', 'json'].includes(typeOf.get(p) ?? ''))
            if (odd.length) warnings.push(`string/json parcels: ${odd.join(', ')}`)
            if (!d.timeTag && chk.errors.length === 0)
              warnings.push(`never updated: are the calculations and data processor processes running? New points start within ${CALC_RELOAD_SECONDS} s`)
            if (chk.def?.result === 'digital' && d.type !== 'digital')
              warnings.push(`0/1 formula on a ${d.type} point`)
          } else if (Number(d.formula) > 0)
            warnings.push(
              `formula ${d.formula} on a point with origin "${d.origin}": ignored by the calculations engine ` +
                '(set origin "calculated", or formula 0)'
            )
          else warnings.push('origin "calculated" but no formula: not computed by the calculations engine')
          const problem = errors.length + warnings.length > 0
          if (problem) withProblems++
          if ((onlyProblems && !problem) || (re && !re.test(d.tag ?? ''))) continue
          rows.push({
            tag: d.tag,
            pointKey: key,
            type: d.type,
            origin: d.origin,
            formula: d.formula ?? 0,
            parcels,
            ...(depth.get(key) ? { chainDepth: depth.get(key) } : {}),
            value: d.value,
            invalid: d.invalid,
            ...(errors.length ? { errors } : {}),
            ...(warnings.length ? { warnings } : {}),
          })
        }
        const proc = await calculationsProcessStatus(mgr)
        return jsonResult({
          summary: {
            calculatedPoints: calcs.length,
            withProblems,
            maxChainDepth: Math.max(0, ...depth.values()),
            pointsInLoops: inLoop.size,
            byFormula,
          },
          calculationsProcess: proc.instances,
          warnings: proc.warnings,
          shown: Math.min(rows.length, limit),
          points: rows.slice(0, limit),
        })
      } catch (e) {
        return errorResult('Error auditing calculated tags', e)
      }
    }
  )
}

export function registerCalculationsAdminTools(
  server: McpServer,
  api: AdminApiClient,
  mgr: ConnectionManager
) {
  server.registerTool(
    'save_calculated_tag',
    {
      title: 'Save Calculated Tag',
      description:
        'Create a calculated tag, or change the formula/parcels and other fields of an existing one, after the same ' +
        'checks as check_calculated_tag. Refuses designs the engine cannot compute (wrong parcel count, missing ' +
        'parcels, loops, unknown formula). Sets origin "calculated". The engine picks up the change within ' +
        `${CALC_RELOAD_SECONDS} s, no restart needed. ` +
        'With dryRun, only checks.',
      inputSchema: z.object({
        tag: z.string().min(1).describe('Tag name (created when it does not exist)'),
        formula: z.number().int().positive().describe('Formula code (see get_calculations_guide)'),
        parcels: parcelsSchema,
        type: z
          .enum(['analog', 'digital'])
          .optional()
          .describe('Point type for a new tag (default from the formula result); ignored for existing tags'),
        description: z.string().optional(),
        group1: z.string().optional().describe('Station / area'),
        group2: z.string().optional().describe('Bay / equipment'),
        group3: z.string().optional(),
        unit: z.string().optional(),
        stateTextTrue: z.string().optional(),
        stateTextFalse: z.string().optional(),
        pointKey: z.number().int().positive().optional().describe('Point key for a new tag (default: next free)'),
        fields: z
          .record(z.string(), z.any())
          .optional()
          .describe('Other realtimeData configuration fields (limits, kconv1/kconv2, isEvent, priority, historianDeadBand, ...)'),
        dryRun: z.boolean().optional().default(false),
      }),
      annotations: { readOnlyHint: false, destructiveHint: false },
    },
    async ({ tag, formula, parcels, type, pointKey, fields, dryRun, ...named }) => {
      if (!mgr.status.HintMongoIsConnected) return notConnectedResult()
      try {
        const name = tag.trim()
        if (isSystemTagName(name)) return textResult(RESERVED_NAME_MESSAGE, true)
        const existing = await findTag(api, name)
        if (existing && existing.origin !== 'calculated')
          return textResult(
            `Tag '${name}' exists with origin "${existing.origin}". Refusing to turn it into a ` +
              'calculated point; use update_tag on purpose, or choose another tag name.',
            true
          )
        if (existing && pointKey && Number(existing._id) !== pointKey)
          return textResult(`Tag '${name}' exists with point key ${existing._id}, not ${pointKey}.`, true)

        const rows = await resolveParcels(mgr, parcels)
        const def = CALC_FORMULAS.get(formula)
        const newType = type ?? (def?.result === 'digital' ? 'digital' : 'analog')
        const target = existing
          ? { key: Number(existing._id), doc: existing }
          : { ...(pointKey ? { key: pointKey } : {}), type: newType }
        const r = await analyze(mgr, formula, rows, target)
        const report = {
          formula: r.def ? formulaInfo(r.def) : { code: formula },
          parcels: rows.map(parcelView),
          errors: r.errors,
          warnings: r.warnings,
          notes: r.notes,
          ...(r.preview ? { preview: r.preview } : {}),
        }
        if (r.errors.length) return { ...jsonResult({ saved: false, ...report }), isError: true }
        if (dryRun) return jsonResult({ saved: false, dryRun: true, wouldCreate: !existing, ...report })

        const doc = buildDoc({
          fields,
          ...named,
          origin: 'calculated',
          formula,
          parcels: rows.map((p) => p.pointKey!),
          ...(existing ? {} : { tag: name, type: newType }),
        })
        let saved: Record<string, unknown>
        if (existing) {
          await api.call('updateTag', { _id: existing._id, ...doc })
          saved = { updated: { tag: name, pointKey: existing._id, fields: Object.keys(doc) } }
        } else {
          const created = await api.call('createTag', { ...doc, ...(pointKey ? { _id: pointKey } : {}) })
          saved = { created: { tag: created?.tag ?? name, pointKey: created?._id } }
        }
        const proc = await calculationsProcessStatus(mgr)
        return jsonResult({
          saved: true,
          ...saved,
          ...report,
          warnings: [...r.warnings, ...proc.warnings],
          calculationsProcess: proc.instances,
          nextStep: NEXT_STEP,
        })
      } catch (e) {
        return errorResult('Error saving calculated tag', e)
      }
    }
  )
}
