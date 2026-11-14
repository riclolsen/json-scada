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

// Display markup `!JSON <tag> <path>`: a member of a tag's valueJson (svg-scada
// skill section 10). Mirrors src/AdminUI/src/lib/sage/sageJson.js (the Display
// Viewer engine); keep both in step.

export const JSON_PREFIX = '!JSON'

export type JsonSeg = { k: 'key'; v: string } | { k: 'idx'; v: number } | { k: 'find'; f: string; v: string }

export interface JsonRef {
  tag: string
  path: string
  /** compiled path, null when its syntax is invalid */
  segs: JsonSeg[] | null
}

const unquote = (t: string) => {
  const s = t.trim()
  const q = s.charAt(0)
  if ((q === "'" || q === '"') && s.length >= 2 && s.charAt(s.length - 1) === q) return s.slice(1, -1)
  return s
}

/** Split a `!JSON <tag> <path>` reference; null when `s` is not one (or has no tag). */
export function parseJsonRef(s: unknown): JsonRef | null {
  const str = String(s ?? '').trim()
  if (!str.startsWith(JSON_PREFIX)) return null
  const rest = str.substring(JSON_PREFIX.length)
  if (rest !== '' && !/^\s/.test(rest)) return null
  const body = rest.trim()
  const m = /\s\$/.exec(body) // the path starts at the first whitespace-preceded '$'
  const tag = (m ? body.substring(0, m.index) : body).trim()
  const path = m ? body.substring(m.index + 1).trim() : '$'
  if (!tag) return null
  return { tag, path, segs: compileJsonPath(path) }
}

/** Compile `$`, `.name`, `['name']`, `[i]`, `[-1]`, `[field=value]`; null on a syntax error. */
export function compileJsonPath(path: string): JsonSeg[] | null {
  const p = String(path ?? '$').trim()
  if (p.charAt(0) !== '$') return null
  const segs: JsonSeg[] = []
  let i = 1
  while (i < p.length) {
    const c = p.charAt(i)
    if (c === '.') {
      let j = i + 1
      while (j < p.length && p.charAt(j) !== '.' && p.charAt(j) !== '[') j++
      const name = p.substring(i + 1, j)
      if (!name) return null
      segs.push({ k: 'key', v: name })
      i = j
    } else if (c === '[') {
      let j = i + 1
      let q = ''
      while (j < p.length) {
        const ch = p.charAt(j)
        if (q) {
          if (ch === q) q = ''
        } else if (ch === "'" || ch === '"') q = ch
        else if (ch === ']') break
        j++
      }
      if (j >= p.length) return null
      const inner = p.substring(i + 1, j).trim()
      i = j + 1
      if (/^-?\d+$/.test(inner)) segs.push({ k: 'idx', v: parseInt(inner, 10) })
      else if (/^(['"]).*\1$/.test(inner)) segs.push({ k: 'key', v: inner.slice(1, -1) })
      else {
        let eq = -1
        let qq = ''
        for (let k = 0; k < inner.length; k++) {
          const ch = inner.charAt(k)
          if (qq) {
            if (ch === qq) qq = ''
          } else if (ch === "'" || ch === '"') qq = ch
          else if (ch === '=') {
            eq = k
            break
          }
        }
        if (eq <= 0) return null
        const f = unquote(inner.substring(0, eq).replace(/=$/, ''))
        const v = unquote(inner.substring(eq + 1).replace(/^=/, ''))
        if (!f) return null
        segs.push({ k: 'find', f, v })
      }
    } else if (/\s/.test(c)) i++
    else return null
  }
  return segs
}

const own = (o: any, k: string) => o !== null && typeof o === 'object' && Object.prototype.hasOwnProperty.call(o, k)
const at = (len: number, i: number) => (i < 0 ? len + i : i)

/** Walk a compiled path over a parsed value; undefined when a step is missing. */
export function walkJson(doc: any, segs: JsonSeg[]): any {
  let cur = doc
  for (const s of segs) {
    if (cur === undefined || cur === null) return undefined
    if (s.k === 'key') {
      if (s.v === 'length' && (Array.isArray(cur) || typeof cur === 'string')) cur = cur.length
      else if (Array.isArray(cur) && /^-?\d+$/.test(s.v)) cur = cur[at(cur.length, parseInt(s.v, 10))]
      else cur = own(cur, s.v) ? cur[s.v] : undefined
    } else if (s.k === 'idx') {
      if (Array.isArray(cur) || typeof cur === 'string') cur = cur[at(cur.length, s.v)]
      else cur = own(cur, String(s.v)) ? cur[String(s.v)] : undefined
    } else {
      if (!Array.isArray(cur)) return undefined
      cur = cur.find((e) => own(e, s.f) && e[s.f] !== null && String(e[s.f]) === s.v)
    }
  }
  return cur
}

/** A point's structured value as the Display Viewer sees it: valueJson parsed, else a json/string point's valueString. */
export function pointJson(p: { type?: string; valueJson?: unknown; valueString?: unknown }): any {
  let raw: unknown = p.valueJson
  if ((raw === undefined || raw === null || raw === '') && (p.type === 'json' || p.type === 'string')) raw = p.valueString
  if (raw === undefined || raw === null) return undefined
  if (typeof raw !== 'string') return raw
  const s = raw.trim()
  if (!s) return undefined
  try {
    return JSON.parse(s)
  } catch {
    return raw
  }
}
