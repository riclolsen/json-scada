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

// The svg-scada agent skill (SCADAvis/OSHMI/JSON-SCADA SVG display markup) served
// to MCP clients that cannot load repository skills themselves: a tool with an
// outline and section lookup, a resource with the full text and a prompt that
// starts a display-authoring task with the skill loaded. The skill is read at
// request time from the installation, so edits to it apply without a rebuild.

import fs from 'node:fs/promises'
import path from 'node:path'
import { z } from 'zod'
import type { McpServer } from '@modelcontextprotocol/server'
import { srcDir } from '../jsonscada/drivers.js'
import { svgDir } from './displays.js'
import { errorResult, jsonResult, textResult } from './util.js'

const SKILL_REL = path.join('.agents', 'skills', 'svg-scada', 'SKILL.md')

// Committed in two identical copies: svg/.agents/... (shipped with the displays)
// and <install>/.agents/... ; JS_MCPJSDB_SVG_SKILL overrides both.
export function svgSkillCandidates(): string[] {
  const env = process.env['JS_MCPJSDB_SVG_SKILL']
  if (env) return [env]
  return [path.join(svgDir(), SKILL_REL), path.resolve(srcDir(), '..', SKILL_REL)]
}

export async function readSvgSkill(): Promise<{ file: string; text: string }> {
  const tried: string[] = []
  for (const file of svgSkillCandidates()) {
    try {
      return { file, text: await fs.readFile(file, 'utf8') }
    } catch {
      tried.push(file)
    }
  }
  throw new Error(`svg-scada skill not found (tried ${tried.join(', ')})`)
}

interface Section {
  level: number
  title: string
  start: number // line index of the heading
  end: number // exclusive
}

// Markdown headings outside fenced code blocks; a section runs to the next
// heading of the same or a higher level
export function skillSections(text: string): { lines: string[]; sections: Section[] } {
  const lines = text.split(/\r?\n/)
  const sections: Section[] = []
  let fence = ''
  lines.forEach((l, i) => {
    const f = /^\s*(```|~~~)/.exec(l)
    if (f) {
      if (!fence) fence = f[1]!
      else if (fence === f[1]) fence = ''
      return
    }
    if (fence) return
    const h = /^(#{1,6})\s+(.*?)\s*#*\s*$/.exec(l)
    if (h) sections.push({ level: h[1]!.length, title: h[2]!, start: i, end: lines.length })
  })
  for (let i = 0; i < sections.length; i++) {
    const s = sections[i]!
    const next = sections.slice(i + 1).find((n) => n.level <= s.level)
    if (next) s.end = next.start
  }
  return { lines, sections }
}

// heading text without emoji/punctuation/numbering, for lenient matching
const norm = (s: string) =>
  s
    .toLowerCase()
    .replace(/^[\s\d.]+/, '')
    .replace(/[^\p{L}\p{N}]+/gu, ' ')
    .trim()

function frontMatter(text: string): Record<string, string> {
  const m = /^---\r?\n([\s\S]*?)\r?\n---/.exec(text)
  const out: Record<string, string> = {}
  if (m) for (const l of m[1]!.split(/\r?\n/)) {
    const kv = /^(\w+):\s*(.*)$/.exec(l)
    if (kv) out[kv[1]!] = kv[2]!
  }
  return out
}

export function registerSkillTools(server: McpServer) {
  server.registerTool(
    'get_svg_scada_skill',
    {
      title: 'Get SVG-SCADA Skill',
      description:
        'Return the svg-scada skill: the rules for SCADA display markup in SVG files (SCADAvis/OSHMI/JSON-SCADA). ' +
        'Bindings are HTML-escaped JSON in inkscape:label with an "attr" animation type (get, color, bar, opacity, ' +
        'rotate, slider, tooltips, popup/open, clone faceplates, set macros such as #arc/#vega, script) and tag ' +
        'references, including "!JSON <tag> <path>" / $J() for members of a tag\'s structured valueJson (section ' +
        '"Structured Values") and "!STR <tag> <selector>" / $S() for parts of its valueString text (section ' +
        '"Text Values"). Read it before creating or editing a display (save_display) or when interpreting unusual ' +
        'markup in get_display. Without section it returns the whole skill (outline: true gives only the ' +
        'headings); with section, only the matching heading and its subsections.',
      inputSchema: z.object({
        section: z
          .string()
          .optional()
          .describe('Heading text to return, case-insensitive substring (e.g. "Color", "Set", "Script", "Example")'),
        outline: z.boolean().optional().default(false).describe('Return only the list of headings'),
      }),
      annotations: { readOnlyHint: true },
    },
    async ({ section, outline }) => {
      try {
        const { file, text } = await readSvgSkill()
        const { lines, sections } = skillSections(text)
        const headings = sections.map((s) => `${'  '.repeat(s.level - 1)}- ${s.title}`)
        if (outline) {
          const fm = frontMatter(text)
          return jsonResult({
            skill: fm['name'] || 'svg-scada',
            description: fm['description'] || '',
            file,
            lines: lines.length,
            headings,
          })
        }
        if (!section) return textResult(text)
        const q = norm(section)
        const hits = q ? sections.filter((s) => norm(s.title).includes(q)) : []
        if (!hits.length)
          return textResult(
            `No section matches "${section}". Headings:\n${headings.join('\n')}`,
            true
          )
        // exact title match wins; otherwise every match not nested in an earlier one
        const exact = hits.filter((s) => norm(s.title) === q)
        const picked = (exact.length ? exact : hits).filter(
          (s, i, arr) => !arr.slice(0, i).some((p) => s.start >= p.start && s.end <= p.end)
        )
        return textResult(picked.map((s) => lines.slice(s.start, s.end).join('\n').trimEnd()).join('\n\n'))
      } catch (e) {
        return errorResult('Cannot read the svg-scada skill', e)
      }
    }
  )

  server.registerResource(
    'svg-scada-skill',
    'json-scada://skills/svg-scada',
    {
      description: 'svg-scada skill: SCADA display markup rules for SVG files (SCADAvis/OSHMI/JSON-SCADA)',
      mimeType: 'text/markdown',
    },
    async (uri) => {
      const { text } = await readSvgSkill()
      return { contents: [{ uri: uri.href, mimeType: 'text/markdown', text }] }
    }
  )

  server.registerPrompt(
    'svg-scada-display',
    {
      title: 'Create or edit a SCADA display',
      description:
        'Start an SVG SCADA display task with the svg-scada skill loaded (markup rules and examples).',
      argsSchema: z.object({
        task: z.string().describe('What to build or change, e.g. "one-line diagram of substation KAW2 with breaker states and MW values"'),
        file: z.string().optional().describe('Display file name in the svg folder to edit or create'),
      }),
    },
    async ({ task, file }) => {
      const { text } = await readSvgSkill()
      const steps = [
        `Task: ${task}`,
        file ? `Display file: ${file}` : '',
        '',
        'Follow the svg-scada skill below. Use search_points / get_display to find existing tags and their ' +
          'types (bind only tags that exist; for a "!JSON <tag> <path>" or "!STR <tag> <selector>" reference, ' +
          'read the tag\'s valueJson / valueString with get_point to write it), keep every inkscape:label valid ' +
          'HTML-escaped JSON, and check the result with save_display dryRun before saving (no missingTags, ' +
          'jsonRefIssues or strRefIssues).',
      ].filter((l, i) => l || i > 1)
      return {
        messages: [
          { role: 'user' as const, content: { type: 'text' as const, text: `${steps.join('\n')}\n\n---\n\n${text}` } },
        ],
      }
    }
  )
}
