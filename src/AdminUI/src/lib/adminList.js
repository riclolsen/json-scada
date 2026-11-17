// Helpers shared by the Admin page lists (Tags, Connections, User Actions):
// JSON posts to the admin routes, paged bulk reads for exports, CSV files and
// per-browser list preferences.

import { getUserSetting, setUserSetting } from './viewerHelpers'

export async function postJson(url, body) {
  const resp = await fetch(url, {
    method: 'post',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  return resp.json()
}

// Reads every page of a paged admin list route (listTags, listUserActions:
// at most 1000 entries per page) up to `max` entries.
// Returns { items, total, truncated }.
export async function fetchAllPages(url, body, listKey, max = 50000) {
  const pageSize = 1000
  const items = []
  let total = 0
  for (let page = 1; items.length < max; page++) {
    const json = await postJson(url, { ...body, itemsPerPage: pageSize, page })
    if (json.error) throw new Error(json.error)
    const list = json[listKey] || []
    total = json.countTotal || 0
    items.push(...list)
    if (list.length < pageSize || items.length >= total) break
  }
  return { items: items.slice(0, max), total, truncated: total > max }
}

function csvCell(v) {
  if (v === undefined || v === null) return ''
  const s = typeof v === 'object' ? JSON.stringify(v) : String(v)
  return /[",\r\n;]/.test(s) ? '"' + s.replace(/"/g, '""') + '"' : s
}

// columns: [{ title, value: (row) => any }]
export function toCsv(rows, columns) {
  const lines = [columns.map((c) => csvCell(c.title)).join(',')]
  for (const r of rows) lines.push(columns.map((c) => csvCell(c.value(r))).join(','))
  return lines.join('\r\n') + '\r\n'
}

// Saves text as a file; the BOM makes spreadsheet tools read the CSV as UTF-8.
export function downloadText(filename, text, mime = 'text/csv') {
  const blob = new Blob(['﻿' + text], { type: mime + ';charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(a.href), 1000)
}

// file name stamp: 2026-11-15_14-05-09
export function fileStamp(d = new Date()) {
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}_${p(d.getHours())}-${p(d.getMinutes())}-${p(d.getSeconds())}`
}

export function escapeRegex(s) {
  return String(s).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

// Per-user preferences of the admin lists (columns, page size, view), kept in
// this browser (getUserSetting / setUserSetting); storage may be unavailable
// (private windows), so a failure falls back to the default.
const PREF_PREFIX = 'admin-pref-'
export function loadPref(key, dflt) {
  try {
    const v = getUserSetting(PREF_PREFIX + key)
    return v === null ? dflt : JSON.parse(v)
  } catch (e) {
    return dflt
  }
}
export function savePref(key, value) {
  setUserSetting(PREF_PREFIX + key, JSON.stringify(value))
}

export async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch (e) {
    return false
  }
}
