/*
 * valueJson access for the native Display Viewer.
 *
 * A tag's `valueJson` (structured value from MQTT/Sparkplug, OPC UA, Node-RED,
 * N8N, ... drivers) is reached in screen markup with the tag reference
 *
 *     !JSON <tag-or-key> <path>
 *
 * where <path> is a small JSONPath subset rooted at `$`:
 *
 *     $                     the whole document
 *     .name                 object member
 *     ['any name']          object member (quoted, for names with . [ ] or spaces)
 *     [2]  [-1]             array element (negative counts from the end)
 *     [field=value]         first array element whose `field` equals value
 *                           (value may be quoted: [name='Pump 1'])
 *     .length               array / string length
 *
 * e.g. `!JSON MQTT.plant/pumps $.pumps[name='P1'].speed`.
 * Scripts and !EVAL expressions use `$J('TAG', '$.path')` instead.
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

export const JSON_PREFIX = '!JSON'

// Parse a `!JSON <tag> <path>` reference. Returns { tag, path, segs } or null
// when `s` is not a JSON reference (or its path is malformed).
export function parseJsonRef(s) {
  const str = String(s == null ? '' : s).trim()
  if (str.indexOf(JSON_PREFIX) !== 0) return null
  const rest = str.substring(JSON_PREFIX.length)
  if (rest !== '' && !/^\s/.test(rest)) return null // e.g. "!JSONX"
  const body = rest.trim()
  // the path is everything from the first whitespace-preceded '$'
  const m = /\s\$/.exec(body)
  const tag = (m ? body.substring(0, m.index) : body).trim()
  const path = m ? body.substring(m.index + 1).trim() : '$'
  if (tag === '') return null
  const segs = compileJsonPath(path)
  if (!segs) return null
  return { tag, path, segs }
}

// Compile a path into segments: {k:'key',v} | {k:'idx',v} | {k:'find',f,v}.
// Returns null on syntax error.
export function compileJsonPath(path) {
  const p = String(path == null ? '$' : path).trim()
  if (p.charAt(0) !== '$') return null
  const segs = []
  let i = 1
  const unquote = (t) => {
    const s = t.trim()
    const q = s.charAt(0)
    if (
      (q === "'" || q === '"') &&
      s.length >= 2 &&
      s.charAt(s.length - 1) === q
    )
      return s.slice(1, -1)
    return s
  }
  while (i < p.length) {
    const c = p.charAt(i)
    if (c === '.') {
      let j = i + 1
      while (j < p.length && p.charAt(j) !== '.' && p.charAt(j) !== '[') j++
      const name = p.substring(i + 1, j)
      if (name === '') return null
      segs.push({ k: 'key', v: name })
      i = j
    } else if (c === '[') {
      // find the matching ']' outside quotes
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
      if (/^-?\d+$/.test(inner)) {
        segs.push({ k: 'idx', v: parseInt(inner, 10) })
      } else if (/^(['"]).*\1$/.test(inner)) {
        segs.push({ k: 'key', v: inner.slice(1, -1) })
      } else {
        // [field=value] — split at the first '=' outside quotes
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
        const v = unquote(inner.substring(eq + 1).replace(/^=/, '')) // tolerate ==
        if (f === '') return null
        segs.push({ k: 'find', f, v })
      }
    } else if (/\s/.test(c)) {
      i++
    } else {
      return null
    }
  }
  return segs
}

const own = (o, k) =>
  o !== null &&
  typeof o === 'object' &&
  Object.prototype.hasOwnProperty.call(o, k)

// Walk compiled segments over a parsed document. Returns undefined when any
// step is missing.
export function walkJson(doc, segs) {
  let cur = doc
  for (const s of segs) {
    if (cur === undefined || cur === null) return undefined
    if (s.k === 'key') {
      if (s.v === 'length' && (Array.isArray(cur) || typeof cur === 'string'))
        cur = cur.length
      else if (Array.isArray(cur) && /^-?\d+$/.test(s.v))
        cur = cur[atIndex(cur, parseInt(s.v, 10))]
      else cur = own(cur, s.v) ? cur[s.v] : undefined
    } else if (s.k === 'idx') {
      if (Array.isArray(cur) || typeof cur === 'string')
        cur = cur[atIndex(cur, s.v)]
      else cur = own(cur, String(s.v)) ? cur[String(s.v)] : undefined
    } else {
      // find
      if (!Array.isArray(cur)) return undefined
      cur = cur.find(
        (e) => own(e, s.f) && e[s.f] !== null && String(e[s.f]) === s.v
      )
    }
  }
  return cur
}

function atIndex(arr, i) {
  return i < 0 ? arr.length + i : i
}

// Parse a tag's raw valueJson into a JS value. Strings holding JSON text are
// parsed; a string that is not JSON stays a string; objects pass through.
// Empty / missing gives undefined.
export function parseValueJson(raw) {
  if (raw === undefined || raw === null) return undefined
  if (typeof raw !== 'string') return raw
  const s = raw.trim()
  if (s === '') return undefined
  try {
    return JSON.parse(s)
  } catch (e) {
    return raw
  }
}

// Find literal tag references of `$J('TAG' ...)` / `$J(123 ...)` (fn = 'J')
// or `$V('TAG')` (fn = 'V') in screen source, so those points are read (with
// their valueJson for $J) from the first refresh.
export function scanScriptTagRefs(src, fn) {
  const out = []
  if (typeof src !== 'string' || src.indexOf('$' + fn) < 0) return out
  const re = new RegExp(
    '\\$' + fn + '\\(\\s*(?:([\'"])((?:(?!\\1).)*)\\1|(\\d+))',
    'g'
  )
  let m
  while ((m = re.exec(src))) out.push(m[2] !== undefined ? m[2] : m[3])
  return out
}
