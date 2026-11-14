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

// SCADA displays: SVG files in the installation svg/ folder whose elements
// carry SCADAvis/OSHMI bindings as JSON in inkscape:label (see the svg-scada
// skill). Reading parses the bindings and joins the bound points' live values;
// saving goes through the AdminUI API (saveDisplay) after validation.

import fs from 'node:fs/promises'
import path from 'node:path'
import { z } from 'zod'
import type { McpServer } from '@modelcontextprotocol/server'
import { AdminApiClient } from '../jsonscada/admin-api.js'
import { ConnectionManager } from '../jsonscada/connection-manager.js'
import { srcDir } from '../jsonscada/drivers.js'
import { parseJsonRef, pointJson, walkJson } from '../jsonscada/json-path.js'
import { applyStr, parseStrRef } from '../jsonscada/str-select.js'
import { errorResult, jsonResult, textResult } from './util.js'

const DISPLAY_NAME = z
  .string()
  .regex(/^[A-Za-z0-9][A-Za-z0-9 ._()-]{0,200}\.svg$/i)
  .describe('Display file name in the svg folder (e.g. "kaw2.svg")')

export function svgDir(): string {
  return process.env['JS_MCPJSDB_SVG_DIR'] || path.resolve(srcDir(), '..', 'svg')
}

function displayPath(file: string): string {
  const dir = path.resolve(svgDir())
  const p = path.resolve(dir, path.basename(file))
  if (!p.startsWith(dir + path.sep)) throw new Error('Invalid display file name.')
  return p
}

const ENTITIES: Record<string, string> = { quot: '"', amp: '&', lt: '<', gt: '>', apos: "'", nbsp: ' ' }
export function decodeEntities(s: string): string {
  return s.replace(/&(#x[0-9a-f]+|#\d+|\w+);/gi, (m, e: string) => {
    if (e[0] === '#')
      return String.fromCodePoint(e[1] === 'x' || e[1] === 'X' ? parseInt(e.slice(2), 16) : parseInt(e.slice(1), 10))
    return ENTITIES[e.toLowerCase()] ?? m
  })
}

function attrsOf(s: string): Record<string, string> {
  const out: Record<string, string> = {}
  for (const m of s.matchAll(/([\w:.-]+)\s*=\s*("([^"]*)"|'([^']*)')/g)) out[m[1]!] = m[3] ?? m[4] ?? ''
  return out
}

function position(a: Record<string, string>): { x: number; y: number } | null {
  let x = Number(a['x'] ?? a['cx'] ?? a['x1'])
  let y = Number(a['y'] ?? a['cy'] ?? a['y1'])
  const t = /translate\(\s*([-\d.eE]+)[\s,]+([-\d.eE]+)/.exec(a['transform'] || '')
  if (t) {
    x = (Number.isFinite(x) ? x : 0) + Number(t[1])
    y = (Number.isFinite(y) ? y : 0) + Number(t[2])
  }
  return Number.isFinite(x) && Number.isFinite(y) ? { x: Math.round(x), y: Math.round(y) } : null
}

// placeholders filled by a parent clone (%n, @1), macros (#...) and special
// tags (!ALM..., !ILIM%n ...) are not database tags (a `!JSON <tag> <path>` or
// `!STR <tag> <selector>` member is reduced to its tag first)
const PLACEHOLDER = /^[#!]|%\w|@\d/
// set macros whose src is a tag list (others take element ids, scripts or names)
const SET_TAG_MACROS = /^#(arc|vega\w*|radar|spider|gauge)$/i
const POPUP_ACTIONS = /^(block|notrace)$|:/i

// Tags referenced by one SCADA label object (see the svg-scada skill)
function tagsOf(o: any): { tags: string[]; placeholders: string[]; jsonRefs: string[]; strRefs: string[] } {
  const tags = new Set<string>()
  const ph = new Set<string>()
  const refs = new Set<string>()
  const srefs = new Set<string>()
  const add = (t: unknown) => {
    let v = String(t ?? '').trim()
    if (!v) return
    // `!JSON <tag> <path>` / `!STR <tag> <selector>` read a part of the
    // tag's valueJson / valueString
    const json = parseJsonRef(v)
    const str = json ? null : parseStrRef(v)
    if (json) {
      refs.add(v)
      v = json.tag
    } else if (str) {
      srefs.add(v)
      v = str.tag
    }
    if (PLACEHOLDER.test(v)) {
      if (/%\w|@\d/.test(v)) ph.add(v)
      return
    }
    tags.add(v)
  }
  const scan = (s: unknown) => {
    for (const m of String(s ?? '').matchAll(/\$[VJS]\(\s*['"]([^'"]+)['"]/g)) add(m[1])
  }
  if (!o || typeof o !== 'object') return { tags: [], placeholders: [], jsonRefs: [], strRefs: [] }
  add(o.tag)
  scan(o.tag) // !EVAL expressions
  for (const it of Array.isArray(o.list) ? o.list : []) {
    add(it?.tag)
    scan(it?.tag)
    scan(it?.param)
  }
  if (o.attr === 'set' && typeof o.src === 'string' && SET_TAG_MACROS.test(String(o.tag || '')))
    for (const t of o.src.split(',')) add(t)
  if (o.attr === 'open' && Number(o.istag) === 1) add(o.src)
  if (o.attr === 'popup' && typeof o.src === 'string' && !POPUP_ACTIONS.test(o.src.trim())) add(o.src)
  if (o.attr === 'clone') for (const m of Array.isArray(o.map) ? o.map : []) add(String(m).split('=')[1])
  for (const p of Array.isArray(o.param) ? o.param : [o.param]) scan(p)
  return { tags: [...tags], placeholders: [...ph], jsonRefs: [...refs], strRefs: [...srefs] }
}

// The `!STR <tag> <selector>` references of a display resolved against the
// bound points' valueString (as the Display Viewer reads them); `error` flags a
// bad selector or a tag without a valueString.
function strMembers(d: ParsedDisplay, points: Map<string, any> | null): Record<string, unknown>[] {
  const refs = [...new Set(d.bindings.flatMap((b) => b.animations.flatMap((a) => a.strRefs ?? [])))]
  return refs.map((ref) => {
    const r = parseStrRef(ref)!
    const out: Record<string, unknown> = { ref, tag: r.tag, selector: r.sel }
    if (!r.ops) return { ...out, error: 'invalid selector syntax' }
    if (PLACEHOLDER.test(r.tag)) return { ...out, placeholder: true }
    if (!points) return out
    const p = points.get(r.tag)
    if (!p) return { ...out, error: 'tag not found' }
    if (typeof p.valueString !== 'string') return { ...out, error: 'tag has no valueString' }
    const v = applyStr(p.valueString, r.ops)
    if (v === undefined || v === null || (typeof v === 'number' && isNaN(v)))
      return { ...out, found: false, invalid: !!p.invalid }
    let value = v
    if (typeof v === 'object') {
      const txt = JSON.stringify(v)
      value = txt.length > MAX_MEMBER_TEXT ? txt.slice(0, MAX_MEMBER_TEXT) + '…' : v
    }
    return { ...out, found: true, value, invalid: !!p.invalid }
  })
}

const MAX_MEMBER_TEXT = 300

// The `!JSON <tag> <path>` references of a display resolved against the bound
// points (as the Display Viewer reads them). A ref whose tag is a clone
// placeholder (%n) is reported without a value; `error` flags a bad path or a
// tag without any structured value.
function jsonMembers(d: ParsedDisplay, points: Map<string, any> | null): Record<string, unknown>[] {
  const refs = [...new Set(d.bindings.flatMap((b) => b.animations.flatMap((a) => a.jsonRefs ?? [])))]
  return refs.map((ref) => {
    const r = parseJsonRef(ref)!
    const out: Record<string, unknown> = { ref, tag: r.tag, path: r.path }
    if (!r.segs) return { ...out, error: 'invalid path syntax' }
    if (PLACEHOLDER.test(r.tag)) return { ...out, placeholder: true }
    if (!points) return out
    const p = points.get(r.tag)
    if (!p) return { ...out, error: 'tag not found' }
    const doc = pointJson(p)
    if (doc === undefined) return { ...out, error: 'tag has no valueJson' }
    const v = walkJson(doc, r.segs)
    if (v === undefined || v === null) return { ...out, found: false, invalid: !!p.invalid }
    let value = v
    if (typeof v === 'object') {
      const txt = JSON.stringify(v)
      value = txt.length > MAX_MEMBER_TEXT ? txt.slice(0, MAX_MEMBER_TEXT) + '…' : v
    }
    return { ...out, found: true, value, invalid: !!p.invalid }
  })
}

export interface ParsedDisplay {
  title: string
  width: string
  height: string
  viewBox: string
  texts: { text: string; x?: number; y?: number }[]
  bindings: {
    id: string
    element: string
    position: { x: number; y: number } | null
    text?: string
    animations: {
      attr: string
      tags: string[]
      placeholders?: string[]
      /** `!JSON <tag> <path>` valueJson member references */
      jsonRefs?: string[]
      /** `!STR <tag> <selector>` valueString part references */
      strRefs?: string[]
      options: Record<string, unknown>
    }[]
  }[]
  invalidLabels: { id: string; label: string; error: string }[]
}

// Regex-based reader: SCADA SVGs are machine-written (Inkscape / generators),
// so element tags and attributes are regular enough for this purpose
export function parseDisplay(svg: string): ParsedDisplay {
  const out: ParsedDisplay = { title: '', width: '', height: '', viewBox: '', texts: [], bindings: [], invalidLabels: [] }
  const root = /<svg\b([^>]*)>/i.exec(svg)
  if (root) {
    const a = attrsOf(root[1]!)
    out.width = a['width'] || ''
    out.height = a['height'] || ''
    out.viewBox = a['viewBox'] || ''
    out.title = a['sodipodi:docname'] || ''
  }
  const title = /<title[^>]*>([\s\S]*?)<\/title>/i.exec(svg)
  if (title) out.title = decodeEntities(title[1]!.trim()) || out.title

  const re = /<([\w:-]+)\b((?:[^>"']|"[^"]*"|'[^']*')*?)(\/?)>/g
  let m: RegExpExecArray | null
  while ((m = re.exec(svg))) {
    const element = m[1]!
    if (element.startsWith('/') || element === '?xml' || element === '!--') continue
    const a = attrsOf(m[2]!)
    let text: string | undefined
    if (element === 'text' && m[3] !== '/') {
      const end = svg.indexOf('</text>', re.lastIndex)
      if (end > 0) {
        text = decodeEntities(svg.slice(re.lastIndex, end).replace(/<[^>]*>/g, ' ').replace(/\s+/g, ' ').trim())
      }
    }
    const label = a['inkscape:label']
    const decoded = label ? decodeEntities(label).trim() : ''
    if (decoded.startsWith('{')) {
      let objs: any[]
      try {
        objs = JSON.parse('[' + decoded + ']')
      } catch (e) {
        out.invalidLabels.push({ id: a['id'] || '', label: decoded.slice(0, 200), error: (e as Error).message })
        continue
      }
      out.bindings.push({
        id: a['id'] || '',
        element,
        position: position(a),
        ...(text ? { text } : {}),
        animations: objs.map((o) => {
          const { tags, placeholders, jsonRefs, strRefs } = tagsOf(o)
          const { attr, tag, list, ...options } = o || {}
          return {
            attr: String(attr || ''),
            tags,
            ...(placeholders.length ? { placeholders } : {}),
            ...(jsonRefs.length ? { jsonRefs } : {}),
            ...(strRefs.length ? { strRefs } : {}),
            options: { ...(tag && PLACEHOLDER.test(String(tag)) ? { tag } : {}), ...trimOptions(options) },
          }
        }),
      })
    } else if (text && !/^%|^@\d/.test(text)) {
      const pos = position(a)
      out.texts.push({ text, ...(pos || {}) })
    }
  }
  return out
}

function trimOptions(o: Record<string, unknown>) {
  const out: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(o)) {
    const s = typeof v === 'string' ? v : JSON.stringify(v)
    out[k] = s && s.length > 200 ? s.slice(0, 200) + '…' : v
  }
  return out
}

async function pointsByTag(mgr: ConnectionManager, tags: string[]) {
  const names = tags.filter((t) => !/^\d+$/.test(t))
  const keys = tags.filter((t) => /^\d+$/.test(t)).map(Number)
  const docs = await mgr
    .getRealtimeDataCollection()
    .find({ $or: [{ tag: { $in: names } }, { _id: { $in: keys } }] } as any, {
      projection: {
        _id: 1, tag: 1, description: 1, type: 1, origin: 1, value: 1, valueString: 1, unit: 1, invalid: 1,
        alarmed: 1, alarmDisabled: 1, substituted: 1, annotation: 1, timeTag: 1, group1: 1,
        protocolSourceConnectionNumber: 1, valueJson: 1,
      },
    })
    .toArray()
  const map = new Map<string, any>()
  for (const d of docs) {
    map.set(String(d.tag), d)
    map.set(String(d._id), d)
  }
  return map
}

async function readDisplay(file: string) {
  return fs.readFile(displayPath(file), 'utf8')
}

export function registerDisplayReadTools(server: McpServer, mgr: ConnectionManager) {
  server.registerTool(
    'list_displays',
    {
      title: 'List Displays',
      description:
        'List the SCADA synoptic displays (SVG files in the installation svg folder) with title, size, ' +
        'modification time and the number of bound points. Open one with get_display.',
      inputSchema: z.object({
        filter: z.string().optional().describe('Case-insensitive regex on the file name or title'),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ filter }) => {
      try {
        let re: RegExp | undefined
        if (filter) re = new RegExp(filter, 'i')
        const dir = svgDir()
        const files = (await fs.readdir(dir)).filter((f) => f.toLowerCase().endsWith('.svg')).sort()
        const out = []
        for (const f of files) {
          const p = path.join(dir, f)
          const st = await fs.stat(p)
          const head = st.size < 20e6 ? await fs.readFile(p, 'utf8') : ''
          const d = head ? parseDisplay(head) : null
          const row = {
            file: f,
            title: d?.title || '',
            sizeKB: Math.round(st.size / 1024),
            modified: st.mtime,
            boundElements: d?.bindings.length ?? null,
            boundTags: d ? new Set(d.bindings.flatMap((b) => b.animations.flatMap((a) => a.tags))).size : null,
          }
          if (!re || re.test(f) || re.test(row.title)) out.push(row)
        }
        return jsonResult({ folder: dir, displays: out })
      } catch (e) {
        return errorResult(`Error listing displays in ${svgDir()}`, e)
      }
    }
  )

  server.registerTool(
    'get_display',
    {
      title: 'Get Display',
      description:
        'Open a SCADA display and describe it for interpretation: title and size, the static texts (labels, ' +
        'substation/bay names) with positions, and every dynamic element with its animations (get = shows a ' +
        'value, color = color by value/state, bar, rotate, slider, open/popup, tooltips, set/script macros) and ' +
        'bound points. With includeValues (default), each bound point comes with its live value, unit, quality ' +
        '(invalid), alarm and substitution state and description, so the display can be read as an operator ' +
        'sees it, and every "!JSON <tag> <path>" reference (member of a tag\'s structured valueJson) and ' +
        '"!STR <tag> <selector>" reference (part of a tag\'s valueString text) with its current value. Reports broken bindings (tags missing from the database) and labels with invalid JSON.',
      inputSchema: z.object({
        file: DISPLAY_NAME,
        includeValues: z.boolean().optional().default(true),
        includeTexts: z.boolean().optional().default(true),
        maxElements: z.number().int().min(0).max(5000).optional().default(300),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ file, includeValues, includeTexts, maxElements }) => {
      try {
        const d = parseDisplay(await readDisplay(file))
        const tags = [...new Set(d.bindings.flatMap((b) => b.animations.flatMap((a) => a.tags)))]
        let points = new Map<string, any>()
        if (includeValues || tags.length) {
          if (!mgr.status.HintMongoIsConnected)
            return textResult('Database not connected (needed to resolve the bound points).', true)
          points = await pointsByTag(mgr, tags)
        }
        const missing = tags.filter((t) => !points.has(t))
        const pointInfo = (t: string) => {
          const p = points.get(t)
          if (!p) return { tag: t, missing: true }
          return {
            tag: p.tag,
            pointKey: p._id,
            description: p.description,
            type: p.type,
            value: p.type === 'digital' && p.valueString ? `${p.value} (${p.valueString})` : p.value,
            unit: p.unit,
            invalid: p.invalid,
            alarmed: p.alarmed,
            ...(p.alarmDisabled ? { alarmDisabled: true } : {}),
            ...(p.substituted ? { substituted: true } : {}),
            ...(p.annotation ? { annotation: p.annotation } : {}),
            timeTag: p.timeTag,
          }
        }
        const values = includeValues ? tags.map(pointInfo) : undefined
        const members = jsonMembers(d, includeValues ? points : null)
        const sMembers = strMembers(d, includeValues ? points : null)
        const summary = {
          boundElements: d.bindings.length,
          boundTags: tags.length,
          missingTags: missing.length,
          invalidPoints: tags.filter((t) => points.get(t)?.invalid).length,
          alarmedPoints: tags.filter((t) => points.get(t)?.alarmed).length,
          invalidLabels: d.invalidLabels.length,
          animationTypes: d.bindings
            .flatMap((b) => b.animations.map((a) => a.attr))
            .reduce((acc: Record<string, number>, a) => ((acc[a] = (acc[a] || 0) + 1), acc), {}),
        }
        return jsonResult({
          file,
          title: d.title,
          width: d.width,
          height: d.height,
          viewBox: d.viewBox,
          summary,
          ...(missing.length ? { missingTags: missing.slice(0, 100) } : {}),
          ...(d.invalidLabels.length ? { invalidLabels: d.invalidLabels.slice(0, 20) } : {}),
          ...(includeTexts ? { texts: d.texts.slice(0, 500) } : {}),
          elements: d.bindings.slice(0, maxElements),
          ...(values ? { points: values } : {}),
          ...(members.length ? { jsonMembers: members } : {}),
          ...(sMembers.length ? { strMembers: sMembers } : {}),
          note:
            'Positions are in SVG user units (top-left origin) from x/y and translate() only; nested group ' +
            'transforms are not applied. Placeholders (%n, @1) are filled by a parent clone/faceplate.' +
            (members.length
              ? ' jsonMembers: "!JSON <tag> <path>" references to a member of the tag\'s valueJson (shown only ' +
                'by the AdminUI Display Viewer; svg-scada skill "Structured Values"); found: false shows as failed.'
              : '') +
            (sMembers.length
              ? ' strMembers: "!STR <tag> <selector>" references to a part of the tag\'s valueString (shown only ' +
                'by the AdminUI Display Viewer; svg-scada skill "Text Values"); found: false shows as failed.'
              : ''),
        })
      } catch (e) {
        return errorResult(`Error reading display ${file}`, e)
      }
    }
  )
}

export function registerDisplayAdminTools(server: McpServer, api: AdminApiClient, mgr: ConnectionManager) {
  server.registerTool(
    'save_display',
    {
      title: 'Save Display',
      description:
        'Create or overwrite a SCADA display (SVG) in the svg folder through the AdminUI API. The content is ' +
        'checked first: it must be an <svg> document, every SCADA label must be valid JSON (HTML-escaped in ' +
        'inkscape:label; read get_svg_scada_skill for the markup rules) and bound tags should exist (reported as warnings; ' +
        '"!JSON <tag> <path>" references with a bad path or a tag without valueJson are reported as jsonRefIssues, ' +
        '"!STR <tag> <selector>" references with a bad selector or a tag without valueString as strRefIssues). ' +
        'With dryRun only the checks run. Overwriting replaces the file for every operator.',
      inputSchema: z.object({
        file: DISPLAY_NAME,
        content: z.string().optional().describe('Full SVG text'),
        fromPath: z.string().optional().describe('Or a path of an SVG file on the MCP server host'),
        overwrite: z.boolean().optional().default(false).describe('Required to replace an existing display'),
        dryRun: z.boolean().optional().default(false),
      }),
      annotations: { readOnlyHint: false, destructiveHint: true },
    },
    async ({ file, content, fromPath, overwrite, dryRun }) => {
      try {
        const svg = content ?? (fromPath ? await fs.readFile(fromPath, 'utf8') : undefined)
        if (!svg) return textResult('Give content or fromPath.', true)
        if (!/<svg\b/i.test(svg) || !/<\/svg>\s*$/i.test(svg.trim()))
          return textResult('Content is not a complete <svg> document.', true)
        const d = parseDisplay(svg)
        const tags = [...new Set(d.bindings.flatMap((b) => b.animations.flatMap((a) => a.tags)))]
        const points = mgr.status.HintMongoIsConnected ? await pointsByTag(mgr, tags) : new Map()
        const missing = mgr.status.HintMongoIsConnected ? tags.filter((t) => !points.has(t)) : []
        let exists = false
        try {
          await fs.access(displayPath(file))
          exists = true
        } catch {
          exists = false
        }
        // !JSON references: bad paths and tags without a structured value (warnings)
        const jsonIssues = jsonMembers(d, mgr.status.HintMongoIsConnected ? points : null).filter(
          (m) => m['error'] && m['error'] !== 'tag not found'
        )
        const strIssues = strMembers(d, mgr.status.HintMongoIsConnected ? points : null).filter(
          (m) => m['error'] && m['error'] !== 'tag not found'
        )
        const check = {
          file,
          exists,
          boundElements: d.bindings.length,
          boundTags: tags.length,
          invalidLabels: d.invalidLabels.slice(0, 20),
          missingTags: missing.slice(0, 100),
          ...(jsonIssues.length ? { jsonRefIssues: jsonIssues.slice(0, 50) } : {}),
          ...(strIssues.length ? { strRefIssues: strIssues.slice(0, 50) } : {}),
        }
        if (d.invalidLabels.length)
          return textResult(JSON.stringify({ saved: false, reason: 'labels with invalid JSON', ...check }, null, 2), true)
        if (dryRun) return jsonResult({ dryRun: true, ...check })
        if (exists && !overwrite)
          return textResult(JSON.stringify({ saved: false, reason: 'display exists: pass overwrite=true', ...check }, null, 2), true)
        await api.call('saveDisplay', { file, content: svg })
        return jsonResult({ saved: true, ...check })
      } catch (e) {
        return errorResult(`Error saving display ${file}`, e)
      }
    }
  )
}
