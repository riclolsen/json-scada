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

// Display markup `!STR <tag> <selector>`: a part of a tag's valueString (svg-scada
// skill "Text Values"). Mirrors src/AdminUI/src/lib/sage/sageStr.js (the Display
// Viewer engine); keep both in step. The selector is parsed, never evaluated.

export const STR_PREFIX = '!STR'

type Arg = string | number | RegExp
export type StrOp = { op: 'call'; name: string; args: Arg[] } | { op: 'len' } | { op: 'idx'; v: number }

export interface StrRef {
  tag: string
  sel: string
  /** compiled selector, null when its syntax is invalid */
  ops: StrOp[] | null
}

// name -> [min args, max args]
const METHODS: Record<string, [number, number]> = {
  split: [1, 2],
  match: [1, 1],
  replace: [2, 2],
  slice: [1, 2],
  trim: [0, 0],
  upper: [0, 0],
  lower: [0, 0],
  kv: [1, 3],
  num: [0, 0],
  is: [1, 1],
  has: [1, 1],
  join: [0, 1],
}

/** Split a `!STR <tag> <selector>` reference; null when `s` is not one (or has no tag). */
export function parseStrRef(s: unknown): StrRef | null {
  const str = String(s ?? '').trim()
  if (!str.startsWith(STR_PREFIX)) return null
  const rest = str.substring(STR_PREFIX.length)
  if (rest !== '' && !/^\s/.test(rest)) return null
  const body = rest.trim()
  const m = /\s\$/.exec(body) // the selector starts at the first whitespace-preceded '$'
  const tag = (m ? body.substring(0, m.index) : body).trim()
  const sel = m ? body.substring(m.index + 1).trim() : '$'
  if (!tag) return null
  return { tag, sel, ops: compileStrSelector(sel) }
}

/** Compile a selector (`$` then `.method(args)`, `.length`, `[n]`); null on a syntax error. */
export function compileStrSelector(sel: string): StrOp[] | null {
  const p = String(sel ?? '$').trim()
  if (p.charAt(0) !== '$') return null
  const ops: StrOp[] = []
  let i = 1
  const ws = () => {
    while (i < p.length && /\s/.test(p.charAt(i))) i++
  }
  const arg = (): Arg | undefined => {
    ws()
    const c = p.charAt(i)
    if (c === "'" || c === '"') {
      let out = ''
      i++
      while (i < p.length && p.charAt(i) !== c) {
        if (p.charAt(i) === '\\' && i + 1 < p.length) {
          const e = p.charAt(i + 1)
          out += e === 'n' ? '\n' : e === 't' ? '\t' : e
          i += 2
        } else out += p.charAt(i++)
      }
      if (i >= p.length) return undefined
      i++
      return out
    }
    if (c === '/') {
      let j = i + 1
      let cls = false
      while (j < p.length) {
        const ch = p.charAt(j)
        if (ch === '\\') j++
        else if (ch === '[') cls = true
        else if (ch === ']') cls = false
        else if (ch === '/' && !cls) break
        j++
      }
      if (j >= p.length) return undefined
      const src = p.substring(i + 1, j)
      i = j + 1
      const fm = /^[gimsuy]*/.exec(p.substring(i))![0]
      i += fm.length
      try {
        return new RegExp(src, fm)
      } catch {
        return undefined
      }
    }
    const nm = /^-?\d+(\.\d+)?/.exec(p.substring(i))
    if (nm) {
      i += nm[0].length
      return parseFloat(nm[0])
    }
    return undefined
  }
  while (i < p.length) {
    ws()
    if (i >= p.length) break
    const c = p.charAt(i)
    if (c === '.') {
      i++
      const nm = /^[A-Za-z_]\w*/.exec(p.substring(i))
      if (!nm) return null
      const name = nm[0]
      i += name.length
      ws()
      if (p.charAt(i) !== '(') {
        if (name !== 'length') return null
        ops.push({ op: 'len' })
        continue
      }
      i++
      const lim = METHODS[name]
      if (!lim) return null
      const args: Arg[] = []
      ws()
      if (p.charAt(i) !== ')') {
        for (;;) {
          const a = arg()
          if (a === undefined) return null
          args.push(a)
          ws()
          if (p.charAt(i) === ',') {
            i++
            continue
          }
          break
        }
      }
      ws()
      if (p.charAt(i) !== ')') return null
      i++
      if (args.length < lim[0] || args.length > lim[1]) return null
      ops.push({ op: 'call', name, args })
    } else if (c === '[') {
      i++
      ws()
      const nm = /^-?\d+/.exec(p.substring(i))
      if (!nm) return null
      i += nm[0].length
      ws()
      if (p.charAt(i) !== ']') return null
      i++
      ops.push({ op: 'idx', v: parseInt(nm[0], 10) })
    } else return null
  }
  return ops
}

const asText = (v: unknown) => (typeof v === 'string' ? v : typeof v === 'number' ? String(v) : undefined)
const at = (len: number, n: number) => (n < 0 ? len + n : n)

/** Apply compiled ops to a valueString; undefined when a step does not apply. */
export function applyStr(text: string, ops: StrOp[]): any {
  let cur: any = text
  for (const o of ops) {
    if (cur === undefined || cur === null) return undefined
    if (o.op === 'len') {
      cur = Array.isArray(cur) || typeof cur === 'string' ? cur.length : undefined
      continue
    }
    if (o.op === 'idx') {
      if (Array.isArray(cur) || typeof cur === 'string') cur = cur[at(cur.length, o.v)]
      else return undefined
      continue
    }
    const a = o.args
    if (o.name === 'slice' && Array.isArray(cur)) {
      cur = cur.slice(a[0] as number, a[1] as number | undefined)
      continue
    }
    if (o.name === 'join') {
      cur = Array.isArray(cur) ? cur.join(a.length ? String(a[0]) : ',') : asText(cur)
      continue
    }
    const s = asText(cur)
    if (s === undefined) return undefined
    switch (o.name) {
      case 'split':
        cur = s.split(a[0] instanceof RegExp ? a[0] : String(a[0]), a.length > 1 ? (a[1] as number) : undefined)
        break
      case 'match': {
        const re = a[0] instanceof RegExp ? a[0] : new RegExp(String(a[0]).replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
        re.lastIndex = 0
        const m = s.match(re)
        cur = m ? Array.from(m) : undefined
        break
      }
      case 'replace':
        cur = s.replace(a[0] instanceof RegExp ? a[0] : String(a[0]), String(a[1]))
        break
      case 'slice':
        cur = s.slice(a[0] as number, a[1] as number | undefined)
        break
      case 'trim':
        cur = s.trim()
        break
      case 'upper':
        cur = s.toUpperCase()
        break
      case 'lower':
        cur = s.toLowerCase()
        break
      case 'kv': {
        const key = String(a[0])
        const ps = a.length > 1 ? String(a[1]) : ';'
        const ks = a.length > 2 ? String(a[2]) : '='
        cur = undefined
        for (const part of s.split(ps)) {
          const k = part.indexOf(ks)
          if (k >= 0 && part.substring(0, k).trim() === key) {
            cur = part.substring(k + ks.length).trim()
            break
          }
        }
        break
      }
      case 'num': {
        const n = parseFloat(s)
        cur = isNaN(n) ? undefined : n
        break
      }
      case 'is':
        cur = s === String(a[0])
        break
      case 'has':
        cur = s.indexOf(String(a[0])) >= 0
        break
      default:
        return undefined
    }
  }
  return cur
}
