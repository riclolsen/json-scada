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

// Process log files: the installation log/ folder where supervisor (Linux) and
// the Windows services write each process stdout/stderr (<name>.log, rotated
// <name>.log.1 ...). Read-only; file names are confined to that folder.

import fs from 'node:fs/promises'
import path from 'node:path'
import { z } from 'zod'
import type { McpServer } from '@modelcontextprotocol/server'
import { srcDir } from '../jsonscada/drivers.js'
import { compileRegex } from '../jsonscada/cim-ems.js'
import { errorResult, jsonResult, textResult } from './util.js'

export function logDir(): string {
  return process.env['JS_MCPJSDB_LOG_DIR'] || path.resolve(srcDir(), '..', 'log')
}

function logPath(file: string): string {
  const dir = path.resolve(logDir())
  const p = path.resolve(dir, path.basename(file))
  if (!p.startsWith(dir + path.sep)) throw new Error('Invalid log file name.')
  return p
}

export const LEVEL_RE: Record<string, RegExp> = {
  error: /\b(error|errors|exception|fatal|panic|critical|traceback|fail|failed|failure|refused|unreachable|timeout|timed out)\b/i,
  warn: /\b(warn|warning|error|errors|exception|fatal|panic|critical|traceback|fail|failed|failure|refused|timeout|timed out|disconnect(ed)?|retry(ing)?|invalid)\b/i,
}

const TS_RE = /(\d{4}-\d{2}-\d{2})[T ](\d{2}:\d{2}:\d{2})(\.\d+)?(Z|[+-]\d{2}:?\d{2})?/

// Timestamp at the start of a log line, when it has one (ISO-like formats)
export function lineTime(line: string): Date | null {
  const m = TS_RE.exec(line.slice(0, 64))
  if (!m) return null
  const d = new Date(`${m[1]}T${m[2]}${m[3] || ''}${m[4] || ''}`)
  return isNaN(d.getTime()) ? null : d
}

// Last maxBytes of a file, split in lines (first partial line dropped)
export async function tailLines(file: string, maxBytes: number): Promise<{ lines: string[]; truncated: boolean; size: number }> {
  const fh = await fs.open(file, 'r')
  try {
    const { size } = await fh.stat()
    const start = Math.max(0, size - maxBytes)
    const buf = Buffer.alloc(size - start)
    await fh.read(buf, 0, buf.length, start)
    let lines = buf.toString('utf8').split(/\r?\n/)
    if (start > 0) lines = lines.slice(1)
    if (lines.length && lines[lines.length - 1] === '') lines.pop()
    return { lines, truncated: start > 0, size }
  } finally {
    await fh.close()
  }
}

export interface LogFile {
  file: string
  sizeKB: number
  modified: Date
  ageSeconds: number
  rotated: boolean
}

export async function listLogFiles(): Promise<LogFile[]> {
  const dir = logDir()
  const now = Date.now()
  const out: LogFile[] = []
  for (const f of await fs.readdir(dir)) {
    if (!/\.(log|txt|out|err)(\.\d+)?$/i.test(f)) continue
    const st = await fs.stat(path.join(dir, f))
    if (!st.isFile()) continue
    out.push({
      file: f,
      sizeKB: Math.round(st.size / 1024),
      modified: st.mtime,
      ageSeconds: Math.round((now - st.mtimeMs) / 1000),
      rotated: /\.\d+$/.test(f),
    })
  }
  return out.sort((a, b) => a.file.localeCompare(b.file))
}

export interface ScanOptions {
  pattern?: RegExp
  level?: 'error' | 'warn' | 'any'
  since?: Date
  maxBytes: number
  maxMatches: number
}

// Matching lines from the tail of one log file; lines without their own
// timestamp inherit the previous one for the since filter
export async function scanLog(file: string, o: ScanOptions) {
  const { lines, truncated, size } = await tailLines(logPath(file), o.maxBytes)
  const level = o.level && o.level !== 'any' ? LEVEL_RE[o.level] : undefined
  const matches: string[] = []
  let total = 0
  let last: Date | null = null
  let first: Date | null = null
  for (const line of lines) {
    const t = lineTime(line)
    if (t) {
      last = t
      first ??= t
    }
    if (o.since && last && last < o.since) continue
    if (o.since && !last) continue
    if (level && !level.test(line)) continue
    if (o.pattern && !o.pattern.test(line)) continue
    total++
    matches.push(line.length > 1000 ? line.slice(0, 1000) + '…' : line)
    if (matches.length > o.maxMatches) matches.shift()
  }
  return { file, sizeKB: Math.round(size / 1024), scannedLines: lines.length, truncated, firstTime: first, lastTime: last, totalMatches: total, lines: matches }
}

const FILE = z.string().min(1).max(255).describe('Log file name as listed by list_logs (e.g. "iec104client.log")')

export function registerLogTools(server: McpServer) {
  server.registerTool(
    'list_logs',
    {
      title: 'List Log Files',
      description:
        'List the process log files of the installation (log folder: one <process>.log per service written by ' +
        'supervisor or the Windows services, rotated as .log.1, .log.2 ...) with size and last modification. A ' +
        'log not modified for long belongs to a stopped or quiet process. With scanErrors, also counts error-like ' +
        'lines in the recent tail of each current log.',
      inputSchema: z.object({
        filter: z.string().optional().describe('Case-insensitive regex on the file name'),
        scanErrors: z.boolean().optional().default(false),
        sinceMinutes: z.number().int().min(1).max(100000).optional().default(60).describe('Window for scanErrors'),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ filter, scanErrors, sinceMinutes }) => {
      try {
        const re = compileRegex(filter, 'filter')
        let files = await listLogFiles()
        if (re) files = files.filter((f) => re.test(f.file))
        if (!scanErrors) return jsonResult({ folder: logDir(), files })
        const since = new Date(Date.now() - sinceMinutes * 60000)
        const out = []
        for (const f of files) {
          if (f.rotated || f.modified < since) {
            out.push(f)
            continue
          }
          const s = await scanLog(f.file, { level: 'error', since, maxBytes: 2 << 20, maxMatches: 3 })
          out.push({ ...f, recentErrors: s.totalMatches, lastErrors: s.lines })
        }
        return jsonResult({ folder: logDir(), sinceMinutes, files: out })
      } catch (e) {
        return errorResult(`Error listing logs in ${logDir()}`, e)
      }
    }
  )

  server.registerTool(
    'read_log',
    {
      title: 'Read Log',
      description:
        'Read the tail of a process log file, optionally filtered by a regex (grep), by level (error: errors, ' +
        'exceptions, failures, timeouts; warn: also warnings, disconnections, retries, invalid) and by time ' +
        '(sinceMinutes, for lines with ISO timestamps). Returns the last `lines` matching lines and the total ' +
        'match count in the scanned tail. Use to diagnose a driver or process: connection refused/timeouts, ' +
        'configuration errors at start, database errors, crashes and restart loops.',
      inputSchema: z.object({
        file: FILE,
        lines: z.number().int().min(1).max(2000).optional().default(200),
        grep: z.string().optional().describe('Case-insensitive regex'),
        level: z.enum(['error', 'warn', 'any']).optional().default('any'),
        sinceMinutes: z.number().int().min(1).max(1000000).optional(),
        maxScanKB: z.number().int().min(16).max(65536).optional().default(4096).describe('Tail size scanned'),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ file, lines, grep, level, sinceMinutes, maxScanKB }) => {
      try {
        const r = await scanLog(file, {
          pattern: compileRegex(grep, 'grep'),
          level,
          since: sinceMinutes ? new Date(Date.now() - sinceMinutes * 60000) : undefined,
          maxBytes: maxScanKB * 1024,
          maxMatches: lines,
        })
        return jsonResult(r)
      } catch (e) {
        if ((e as NodeJS.ErrnoException).code === 'ENOENT')
          return textResult(`Log file '${file}' not found in ${logDir()} (see list_logs).`, true)
        return errorResult(`Error reading log ${file}`, e)
      }
    }
  )

  server.registerTool(
    'search_logs',
    {
      title: 'Search Logs',
      description:
        'Search all current process logs (rotated files excluded unless includeRotated) for a regex and/or a ' +
        'level, e.g. a connection name, an IP address, a tag or "error". Returns per file the match count and ' +
        'the last matches; files without matches are omitted.',
      inputSchema: z.object({
        grep: z.string().optional().describe('Case-insensitive regex'),
        level: z.enum(['error', 'warn', 'any']).optional().default('any'),
        sinceMinutes: z.number().int().min(1).max(1000000).optional(),
        fileFilter: z.string().optional().describe('Case-insensitive regex on file names'),
        includeRotated: z.boolean().optional().default(false),
        maxMatchesPerFile: z.number().int().min(1).max(200).optional().default(10),
        maxScanKB: z.number().int().min(16).max(65536).optional().default(2048),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ grep, level, sinceMinutes, fileFilter, includeRotated, maxMatchesPerFile, maxScanKB }) => {
      try {
        if (!grep && level === 'any') return textResult('Give grep and/or level.', true)
        const pattern = compileRegex(grep, 'grep')
        const fre = compileRegex(fileFilter, 'fileFilter')
        const since = sinceMinutes ? new Date(Date.now() - sinceMinutes * 60000) : undefined
        const results = []
        for (const f of await listLogFiles()) {
          if ((!includeRotated && f.rotated) || (fre && !fre.test(f.file))) continue
          if (since && f.modified < since) continue
          const r = await scanLog(f.file, { pattern, level, since, maxBytes: maxScanKB * 1024, maxMatches: maxMatchesPerFile })
          if (r.totalMatches) results.push(r)
        }
        results.sort((a, b) => b.totalMatches - a.totalMatches)
        return jsonResult({ folder: logDir(), filesWithMatches: results.length, results })
      } catch (e) {
        return errorResult('Error searching logs', e)
      }
    }
  )
}
