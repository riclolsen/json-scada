/*
 * Helper utilities for the native Tabular/Alarms viewer.
 * ES-module reimplementation of the few public/util.js functions the viewer needs,
 * plus flag/qualifier decoding ported from public/tabular.html.
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

import { Flags } from './opcCodes'

// --- cookie / storage --------------------------------------------------------

export function readCookie(name) {
  const nameEQ = name + '='
  const ca = document.cookie.split(';')
  for (let i = 0; i < ca.length; i++) {
    let c = ca[i]
    while (c.charAt(0) === ' ') c = c.substring(1, c.length)
    if (c.indexOf(nameEQ) === 0) return c.substring(nameEQ.length, c.length)
  }
  return null
}

export function getUserObj() {
  try {
    const ck = readCookie('json-scada-user')
    if (ck && ck !== '') return JSON.parse(decodeURIComponent(ck))
  } catch (e) {
    /* ignore */
  }
  return null
}

// Mirrors util.js userHasRight: absence of a rights object grants everything,
// a missing named right denies, otherwise returns the stored boolean.
export function userHasRight(right) {
  const obj = getUserObj()
  if (!obj || !('rights' in obj)) return true
  if (!(right in obj.rights)) return false
  return obj.rights[right]
}

export function getUserGroup1List() {
  const obj = getUserObj()
  if (obj && obj.rights && Array.isArray(obj.rights.group1List))
    return obj.rights.group1List
  return []
}

export function storageAvailable(type) {
  try {
    const storage = window[type]
    const x = '__storage_test__'
    storage.setItem(x, x)
    storage.removeItem(x)
    return true
  } catch (e) {
    return false
  }
}

// --- numbers / formatting ----------------------------------------------------

export function roundnum(value, decimals) {
  return Number(Math.round(value + 'e' + decimals) + 'e-' + decimals)
}

export function isInt(str) {
  const i = parseInt(str)
  if (isNaN(i)) return false
  return i.toString() === String(str)
}

// Format a date with optional locale/options; falls back to default toLocaleString.
// Appends milliseconds when no fractionalSecondDigits option was given (legacy behavior).
export function formatDateTime(value, locale, options) {
  if (value === null || value === undefined || value === '') return ''
  const dt = new Date(value)
  if (isNaN(dt.getTime())) return ''
  let out
  try {
    out = dt.toLocaleString(locale || undefined, options || undefined)
  } catch (e) {
    out = dt.toLocaleString()
  }
  if (!options || !('fractionalSecondDigits' in options))
    out += '.' + ('00' + dt.getUTCMilliseconds()).slice(-3)
  return out
}

// Locale date-only formatter (events viewer Date column)
export function formatDate(value, locale, options) {
  if (value === null || value === undefined || value === '') return ''
  const dt = value instanceof Date ? value : new Date(value)
  if (isNaN(dt.getTime())) return ''
  try {
    return dt.toLocaleDateString(locale || undefined, options || undefined)
  } catch (e) {
    return dt.toLocaleDateString()
  }
}

// Locale time-only formatter (events viewer Time column)
export function formatTime(value, locale, options) {
  if (value === null || value === undefined || value === '') return ''
  const dt = value instanceof Date ? value : new Date(value)
  if (isNaN(dt.getTime())) return ''
  try {
    return dt.toLocaleTimeString(locale || undefined, options || undefined)
  } catch (e) {
    return dt.toLocaleTimeString()
  }
}

// Zero-padded UTC milliseconds (events viewer ms column)
export function utcMillis(value) {
  const dt = value instanceof Date ? value : new Date(value)
  if (isNaN(dt.getTime())) return ''
  return ('00' + dt.getUTCMilliseconds()).slice(-3)
}

export function formatAlarmTime(value, locale, options) {
  if (value === null || value === undefined || value === '') return ''
  const dt = new Date(value)
  if (isNaN(dt.getTime()) || dt.getFullYear() <= 1980) return ''
  try {
    return dt.toLocaleString(locale || undefined, options || undefined)
  } catch (e) {
    return dt.toLocaleString()
  }
}

// --- qualifier decoding ------------------------------------------------------

// Decode the numeric flags bitmask into the human-readable qualifier text used
// by the point info dialog (ported from tabular.html showValsInfo2, lines 440-469).
export function decodeQuality(flags, msg) {
  let sq = ''
  if (flags & Flags.FAILED) sq += msg.failed + ' '
  if (flags & 0x10) sq += msg.substituted + ' '
  if ((flags & 0x0c) === 0x04) sq += msg.calculated + ' '
  else if ((flags & 0x0c) === 0x0c) sq += msg.manual + ' '
  else if ((flags & 0x0c) === 0x08) sq += msg.neverUpdated + ' '
  if (flags & Flags.ALARMED) sq += msg.alarmed + ' '
  if (flags & Flags.ALARM_DISABLED) sq += msg.inhibited + ' '
  if (flags & Flags.ABNORMAL) sq += msg.persistent + ' '
  if (flags & Flags.FROZEN) sq += msg.frozen + ' '
  if (sq === '') sq = msg.normal + ' '
  return sq
}

// --- clipboard ---------------------------------------------------------------

export function copyRowsToClipboard(headers, rows) {
  const head = headers.map((h) => h.title).join('\t')
  const body = rows
    .map((r) => headers.map((h) => r[h.key] ?? '').join('\t'))
    .join('\n')
  const text = head + '\n' + body
  if (navigator.clipboard && navigator.clipboard.writeText)
    return navigator.clipboard.writeText(text)
  return Promise.resolve()
}
