/*
 * valueString access for the native Display Viewer.
 *
 * A tag's `valueString` (text value: driver strings, CSV/"k=v" lists, status
 * words, ...) is reached in screen markup with the tag reference
 *
 *     !STR <tag-or-key> <selector>
 *
 * where <selector> is a chain of operations applied left to right, starting at
 * `$` (the whole valueString). It is parsed, never evaluated as code:
 *
 *     .split(sep [, limit])   array of parts (sep: 'text' or /regex/)
 *     .match(/re/flags)       match array: [0] whole match, [1].. groups
 *                             (with flag g: every match)
 *     .replace(find, with)    find: 'text' (first) or /regex/ (g = all)
 *     .slice(a [, b])         substring / sub-array (negative = from end)
 *     .trim() .upper() .lower()
 *     .kv(key [, pairSep [, kvSep]])  value of key in "a=1;b=2" (defaults ';' '=')
 *     .num()                  number (parseFloat; missing if not numeric)
 *     .is(text)  .has(text)   boolean: equals / contains (a digital state)
 *     .join(sep)              array back to text
 *     .length                 length of the text / array
 *     [n]                     element of an array / character (negative = from end)
 *
 * e.g. `!STR PLC1.status $.split(';')[2].trim()`, `!STR M1 $.match(/T=([-\d.]+)/)[1].num()`.
 * Scripts and !EVAL expressions use `$S('TAG', '$.kv("mode")')`.
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

export const STR_PREFIX = '!STR'

// Methods: name -> [min args, max args]
const METHODS = {
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

// Parse a `!STR <tag> <selector>` reference. Returns { tag, sel, ops } or null
// when `s` is not a valid string reference.
export function parseStrRef(s) {
  const str = String(s == null ? '' : s).trim()
  if (str.indexOf(STR_PREFIX) !== 0) return null
  const rest = str.substring(STR_PREFIX.length)
  if (rest !== '' && !/^\s/.test(rest)) return null // e.g. "!STRX"
  const body = rest.trim()
  const m = /\s\$/.exec(body) // the selector starts at the first whitespace-preceded '$'
  const tag = (m ? body.substring(0, m.index) : body).trim()
  const sel = m ? body.substring(m.index + 1).trim() : '$'
  if (tag === '') return null
  const ops = compileStrSelector(sel)
  if (!ops) return null
  return { tag, sel, ops }
}

// Compile a selector into ops: {op:'call',name,args} | {op:'len'} | {op:'idx',v}.
// Returns null on a syntax error.
export function compileStrSelector(sel) {
  const p = String(sel == null ? '$' : sel).trim()
  if (p.charAt(0) !== '$') return null
  const ops = []
  let i = 1
  const ws = () => {
    while (i < p.length && /\s/.test(p.charAt(i))) i++
  }
  // one argument: 'text' | "text" | number | /regex/flags
  const arg = () => {
    ws()
    const c = p.charAt(i)
    if (c === "'" || c === '"') {
      let out = ''
      i++
      while (i < p.length && p.charAt(i) !== c) {
        if (p.charAt(i) === '\\' && i + 1 < p.length) {
          const e = p.charAt(i + 1)
          out +=
            e === 'n' ? '\n'
            : e === 't' ? '\t'
            : e
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
      const fm = /^[gimsuy]*/.exec(p.substring(i))[0]
      i += fm.length
      try {
        return new RegExp(src, fm)
      } catch (e) {
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
      i++ // (
      const lim = METHODS[name]
      if (!lim) return null
      const args = []
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

const asText = (v) =>
  typeof v === 'string' ? v
  : typeof v === 'number' ? String(v)
  : undefined
const idx = (len, n) => (n < 0 ? len + n : n)

// Apply compiled ops to a valueString. Returns undefined when a step does not
// apply (missing field, no match, not numeric, ...).
export function applyStr(text, ops) {
  let cur = text
  for (const o of ops) {
    if (cur === undefined || cur === null) return undefined
    if (o.op === 'len') {
      cur =
        Array.isArray(cur) || typeof cur === 'string' ? cur.length : undefined
      continue
    }
    if (o.op === 'idx') {
      if (Array.isArray(cur) || typeof cur === 'string')
        cur = cur[idx(cur.length, o.v)]
      else return undefined
      continue
    }
    const a = o.args
    if (o.name === 'slice' && Array.isArray(cur)) {
      cur = cur.slice(a[0], a[1])
      continue
    }
    if (o.name === 'join') {
      cur =
        Array.isArray(cur) ?
          cur.join(a.length ? String(a[0]) : ',')
        : asText(cur)
      continue
    }
    const s = asText(cur)
    if (s === undefined) return undefined
    switch (o.name) {
      case 'split':
        cur = s.split(
          a[0] instanceof RegExp ? a[0] : String(a[0]),
          a.length > 1 ? a[1] : undefined
        )
        break
      case 'match': {
        const re =
          a[0] instanceof RegExp ?
            a[0]
          : new RegExp(String(a[0]).replace(/[.*+?^${}()|[\]\\]/g, '\\$&'))
        re.lastIndex = 0
        const m = s.match(re)
        cur = m ? Array.from(m) : undefined
        break
      }
      case 'replace':
        cur = s.replace(
          a[0] instanceof RegExp ? a[0] : String(a[0]),
          String(a[1])
        )
        break
      case 'slice':
        cur = s.slice(a[0], a[1])
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
