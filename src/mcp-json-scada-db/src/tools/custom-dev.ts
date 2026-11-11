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

// Custom developments: the development guide with the scadaOpcApi reference
// (read from the installed library), a prompt that starts a development task,
// and a read-only check of the apps (build state, route, library, tags used).
// Reads are confined to the custom developments folder; nothing is written.

import fs from 'node:fs/promises'
import path from 'node:path'
import crypto from 'node:crypto'
import { z } from 'zod'
import type { McpServer } from '@modelcontextprotocol/server'
import type { ConnectionManager } from '../jsonscada/connection-manager.js'
import {
  API_LIB_REL,
  CUSTOM_DEV_GUIDE,
  TEMPLATES,
  apiMarkdown,
  customDevDir,
  parseApi,
  readApiLibrary,
} from '../jsonscada/custom-dev.js'
import { errorResult, jsonResult, textResult } from './util.js'

const SOURCE_EXT = new Set(['.ts', '.tsx', '.js', '.jsx', '.mjs', '.astro', '.vue', '.svelte', '.html'])
const SKIP_DIRS = new Set(['node_modules', 'dist', '.astro', '.git'])
const MAX_FILES = 3000

async function exists(p: string) {
  try {
    await fs.access(p)
    return true
  } catch {
    return false
  }
}

async function mtime(p: string): Promise<number | null> {
  try {
    return (await fs.stat(p)).mtimeMs
  } catch {
    return null
  }
}

// source files of an app (src/, config files at the root), newest first
async function sourceFiles(appDir: string): Promise<{ file: string; mtime: number }[]> {
  const out: { file: string; mtime: number }[] = []
  const walk = async (dir: string) => {
    if (out.length >= MAX_FILES) return
    let entries
    try {
      entries = await fs.readdir(dir, { withFileTypes: true })
    } catch {
      return
    }
    for (const e of entries) {
      const p = path.join(dir, e.name)
      if (e.isDirectory()) {
        if (!SKIP_DIRS.has(e.name)) await walk(p)
      } else if (SOURCE_EXT.has(path.extname(e.name)) && out.length < MAX_FILES)
        out.push({ file: p, mtime: (await mtime(p)) ?? 0 })
    }
  }
  await walk(path.join(appDir, 'src'))
  for (const f of ['astro.config.mjs', 'astro.config.ts', 'package.json', 'index.html']) {
    const p = path.join(appDir, f)
    const t = await mtime(p)
    if (t !== null) out.push({ file: p, mtime: t })
  }
  return out.sort((a, b) => b.mtime - a.mtime)
}

const sha = (s: string) => crypto.createHash('sha1').update(s.replace(/\r\n/g, '\n')).digest('hex')

// a folder name passed by the client, confined to the custom developments folder
function appPath(name: string): string {
  const dir = path.resolve(customDevDir())
  const p = path.resolve(dir, path.basename(name))
  if (path.dirname(p) !== dir) throw new Error('invalid app name')
  return p
}

// Tag names referenced by the app code: string literals that are tags, plus the
// constants passed to issueCommand / getCommandAckStatus
async function referencedTags(mgr: ConnectionManager, files: string[]) {
  const literals = new Set<string>()
  const commandRefs = new Set<string>()
  const lib = path.join('src', 'lib') + path.sep
  for (const f of files) {
    if (f.includes(lib) || f.includes(path.join('components', 'ui') + path.sep)) continue
    let text: string
    try {
      text = await fs.readFile(f, 'utf8')
    } catch {
      continue
    }
    const consts = new Map<string, string>()
    for (const m of text.matchAll(/\bconst\s+(\w+)\s*=\s*(['"])([^'"\n]{1,120})\2/g)) consts.set(m[1]!, m[3]!)
    for (const m of text.matchAll(/(['"])([^'"\s\n]{2,120})\1/g)) literals.add(m[2]!)
    for (const m of text.matchAll(/\b(?:issueCommand|getCommandAckStatus)\(\s*(?:(['"])([^'"\n]+)\1|(\w+))/g)) {
      const v = m[2] ?? consts.get(m[3] ?? '')
      if (v) commandRefs.add(v)
    }
  }
  const names = [...new Set([...literals, ...commandRefs])]
  const found: any[] = mgr.status.HintMongoIsConnected
    ? await mgr
        .getRealtimeDataCollection()
        .find({ tag: { $in: names } })
        .project({ _id: 1, tag: 1, type: 1, origin: 1, unit: 1 })
        .toArray()
    : []
  const byTag = new Map(found.map((d) => [d.tag, d]))
  // literals that look like tag names (no spaces, upper case, a digit or separator, 6+ characters)
  // but are neither tags nor station (group1) names
  const looksLikeTag = (s: string) =>
    /^[A-Z0-9_][A-Za-z0-9_.\-~]{5,}$/.test(s) && /[0-9_.\-~]/.test(s) && /[A-Z]/.test(s) && !/^[A-Z_]+$/.test(s)
  const candidates = [...literals].filter((s) => !byTag.has(s) && looksLikeTag(s))
  const stations = new Set<string>(
    candidates.length && mgr.status.HintMongoIsConnected
      ? ((await mgr.getRealtimeDataCollection().distinct('group1', { group1: { $in: candidates } } as any)) as string[])
      : []
  )
  const possiblyMissing = candidates.filter((s) => !stations.has(s)).slice(0, 50)
  const commandIssues = [...commandRefs]
    .map((t) => {
      const d = byTag.get(t)
      if (!d) return `${t}: used in a command call but not found in realtimeData`
      if (d.origin !== 'command') return `${t}: used in a command call but its origin is "${d.origin}" (needs "command")`
      return ''
    })
    .filter(Boolean)
  return {
    databaseChecked: mgr.status.HintMongoIsConnected,
    tags: found
      .map((d) => ({ tag: d.tag, pointKey: d._id, type: d.type, origin: d.origin, unit: d.unit }))
      .sort((a, b) => String(a.tag).localeCompare(String(b.tag))),
    commandTags: [...commandRefs],
    commandIssues,
    possiblyMissingTags: possiblyMissing,
  }
}

export function registerCustomDevTools(server: McpServer, mgr: ConnectionManager) {
  server.registerTool(
    'get_custom_development_guide',
    {
      title: 'Custom Development Guide',
      description:
        'How to develop a custom development (a web app served by JSON-SCADA under /custom-developments/<folder>): ' +
        'how serving and routing work, the Astro + React templates and what each shows, the step-by-step workflow ' +
        '(copy a template, find tags, write components, build, restart the realtime server once, open), and the ' +
        'rules for data access, polling, quality, commands and properties. With api: true, also the reference of ' +
        'every scadaOpcApi function, data type and quality code, read from the installed library.',
      inputSchema: z.object({
        api: z.boolean().optional().default(false).describe('Append the scadaOpcApi reference'),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ api }) => {
      if (!api) return textResult(CUSTOM_DEV_GUIDE)
      try {
        const { file, text } = await readApiLibrary()
        return textResult(`${CUSTOM_DEV_GUIDE}\n${apiMarkdown(parseApi(text, file))}`)
      } catch (e) {
        return textResult(`${CUSTOM_DEV_GUIDE}\n(scadaOpcApi reference unavailable: ${e instanceof Error ? e.message : e})`)
      }
    }
  )

  server.registerTool(
    'check_custom_developments',
    {
      title: 'Check Custom Developments',
      description:
        'List the custom developments in src/custom-developments with their state: template or user app, URL, ' +
        'whether it is built and served (dist/index.html), whether the build is older than the sources, ' +
        'dependencies installed, the Astro base path, and whether its scadaOpcApi library matches the templates. ' +
        'With name, also the tags its code references (resolved in realtimeData), command calls on tags that are not ' +
        'command points, and string literals that look like tag names but do not exist.',
      inputSchema: z.object({
        name: z.string().optional().describe('App folder name for the detailed check'),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ name }) => {
      try {
        const dir = path.resolve(customDevDir())
        let folders: string[]
        try {
          folders = (await fs.readdir(dir, { withFileTypes: true })).filter((e) => e.isDirectory()).map((e) => e.name)
        } catch {
          return textResult(`Custom developments folder not found: ${dir} (set JS_MCPJSDB_CUSTOM_DEV_DIR).`, true)
        }
        if (name !== undefined && !folders.includes(path.basename(name)))
          return textResult(`Custom development '${name}' not found in ${dir}. Existing: ${folders.join(', ')}`, true)

        // the reference library: the copy most templates share
        const libHash = new Map<string, string>()
        for (const f of folders)
          try {
            libHash.set(f, sha(await fs.readFile(path.join(dir, f, API_LIB_REL), 'utf8')))
          } catch {
            /* none */
          }
        const counts = new Map<string, number>()
        for (const [f, h] of libHash) counts.set(h, (counts.get(h) ?? 0) + (f in TEMPLATES ? 10 : 1))
        const refHash = [...counts.entries()].sort((a, b) => b[1] - a[1])[0]?.[0]

        const apps: any[] = []
        for (const f of name !== undefined ? [path.basename(name)] : folders) {
          const appDir = appPath(f)
          const issues: string[] = []
          const distIndex = path.join(appDir, 'dist', 'index.html')
          const plainIndex = path.join(appDir, 'index.html')
          const distTime = await mtime(distIndex)
          const plain = distTime === null && (await exists(plainIndex))
          const served = distTime !== null || plain
          let pkg: any = null
          try {
            pkg = JSON.parse(await fs.readFile(path.join(appDir, 'package.json'), 'utf8'))
          } catch {
            /* none */
          }
          const files = await sourceFiles(appDir)
          const newest = files[0]
          let astroBase: string | null = null
          try {
            const cfg = await fs.readFile(path.join(appDir, 'astro.config.mjs'), 'utf8')
            astroBase = /base:\s*[`'"]([^`'"]*)/.exec(cfg)?.[1] ?? null
            if (!cfg.includes('/custom-developments/'))
              issues.push('astro.config.mjs does not set base /custom-developments/<folder>: assets will not load when served')
          } catch {
            /* not an Astro app */
          }
          if (!served)
            issues.push(
              pkg ? 'not built: run npm install and npm run build (the realtime server does not route it)' : 'no index.html and no package.json: nothing to serve'
            )
          if (distTime !== null && newest && newest.mtime > distTime + 1000)
            issues.push(`build is older than ${path.relative(appDir, newest.file)}: run npm run build`)
          if (pkg && !(await exists(path.join(appDir, 'node_modules')))) issues.push('dependencies not installed (npm install)')
          if (pkg?.name === 'react-shadcn-tailwind' && !(f in TEMPLATES)) issues.push('package.json name still the template name')
          const h = libHash.get(f)
          if (pkg && !h) issues.push(`no ${API_LIB_REL.replace(/\\/g, '/')}: copy it from a template`)
          else if (h && refHash && h !== refHash)
            issues.push('scadaOpcApi.ts differs from the templates\' copy: keep the library unchanged, or re-copy it')
          if (/[^A-Za-z0-9_-]/.test(f)) issues.push('folder name has characters other than letters, digits, - and _ (it is the URL)')
          const app: any = {
            name: f,
            kind: f in TEMPLATES ? 'template' : 'app',
            ...(f in TEMPLATES ? { description: TEMPLATES[f] } : {}),
            url: `/custom-developments/${f}/`,
            served,
            ...(distTime !== null ? { builtAt: new Date(distTime).toISOString() } : {}),
            ...(newest ? { sourcesChangedAt: new Date(newest.mtime).toISOString() } : {}),
            ...(pkg ? { packageName: pkg.name, framework: pkg.dependencies?.astro ? 'astro' : 'other' } : { framework: plain ? 'plain html' : 'none' }),
            ...(astroBase !== null ? { astroBase } : {}),
            issues,
          }
          if (name !== undefined) app.references = await referencedTags(mgr, files.map((x) => x.file))
          apps.push(app)
        }
        return jsonResult({
          folder: dir,
          note: 'Routes are created when server_realtime_auth starts: restart it after adding an app; rebuilding a routed app needs only a browser reload.',
          apps,
        })
      } catch (e) {
        return errorResult('Error checking custom developments', e)
      }
    }
  )

  server.registerPrompt(
    'custom-development',
    {
      title: 'Create or change a custom development',
      description: 'Start a custom development task with the development guide and the scadaOpcApi reference loaded.',
      argsSchema: z.object({
        task: z.string().describe('What to build, e.g. "faceplate of breaker 52-1 of KAW2 with open/close commands"'),
        name: z.string().optional().describe('App folder name (new or existing)'),
        template: z.string().optional().describe(`Template to start from: ${Object.keys(TEMPLATES).join(', ')}`),
      }),
    },
    async ({ task, name, template }) => {
      let api = ''
      try {
        const { file, text } = await readApiLibrary()
        api = apiMarkdown(parseApi(text, file))
      } catch (e) {
        api = `(scadaOpcApi reference unavailable: ${e instanceof Error ? e.message : e})`
      }
      const steps = [
        `Task: ${task}`,
        name ? `App folder: src/custom-developments/${name}` : '',
        template ? `Start from the template: ${template}` : '',
        '',
        'Follow the guide below. Find the exact tag names with search_points / get_point before writing code ' +
          '(command buttons only on origin "command" tags), use only scadaOpcApi for data, build with npm run build, ' +
          'and check the app with check_custom_developments (name) for build state, missing tags and command targets.',
      ].filter((l, i) => l || i > 2)
      return {
        messages: [
          {
            role: 'user' as const,
            content: { type: 'text' as const, text: `${steps.join('\n')}\n\n---\n\n${CUSTOM_DEV_GUIDE}\n${api}` },
          },
        ],
      }
    }
  )
}
