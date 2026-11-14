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

// Custom developments (src/custom-developments): web apps served by
// server_realtime_auth under /custom-developments/<folder>, built from the
// Astro + React templates, reading SCADA data through src/lib/scadaOpcApi.ts.
// The API reference is parsed from the installed library at request time,
// so it always matches the code the apps use.

import fs from 'node:fs/promises'
import path from 'node:path'
import { srcDir } from './drivers.js'

export const API_LIB_REL = path.join('src', 'lib', 'scadaOpcApi.ts')

export function customDevDir(): string {
  return process.env['JS_MCPJSDB_CUSTOM_DEV_DIR'] || path.join(srcDir(), 'custom-developments')
}

// the example templates shipped with the installation
export const TEMPLATES: Record<string, string> = {
  basic_bargraph:
    'Smallest example: a bar graph of a fixed tag list refreshed every second (getRealTimeData). Start here.',
  advanced_dashboard:
    'Dashboard with a station/bay tree to pick points (getGroup1List, getRealtimeFilteredData), bar and arc ' +
    'graphs and historical plots (getHistoricalData); reusable hooks useRealtimePoints / useHistoricalData.',
  transformer_with_command:
    'Equipment faceplate: transformer drawing with measurement boxes, tap changer raise/lower buttons with ' +
    'command acknowledgement tracking (issueCommand + getCommandAckStatus) and an events grid (getSoeData).',
}

export interface ApiFunction {
  name: string
  signature: string
  doc: string
}

export interface ApiReference {
  file: string
  functions: ApiFunction[]
  types: Record<string, string>
  statusCodes: string
  acknowledge: string
}

// the data types an app works with; the Opc* request types are internal
const API_TYPES = ['DataPoint', 'HistoricalData', 'SoeData', 'CommandResult', 'ExtendedProperties']

function cleanDoc(block: string): string {
  return block
    .replace(/^\/\*\*|\*\/$/g, '')
    .split('\n')
    .map((l) => l.replace(/^\s*\* ?/, '').trimEnd())
    .filter((l) => l.trim() !== '')
    .join('\n')
}

// Exported functions (signature + JSDoc), data interfaces and status codes
export function parseApi(text: string, file: string): ApiReference {
  const src = text.replace(/\r\n/g, '\n')
  const functions: ApiFunction[] = []
  const fnRe = /(\/\*\*(?:(?!\*\/)[\s\S])*\*\/\s*)?export\s+(?:async\s+)?function\s+(\w+)\s*\(([\s\S]*?)\)\s*:\s*([^{]+?)\s*\{/g
  for (let m; (m = fnRe.exec(src)); ) {
    const params = m[3]!
      .split('\n')
      .map((l) => l.trim().replace(/\/\/\s*(.*)$/, '/* $1 */'))
      .filter(Boolean)
      .join(' ')
      .replace(/,\s*$/, '')
    functions.push({
      name: m[2]!,
      signature: `${m[2]}(${params}): ${m[4]!.trim()}`,
      doc: m[1] ? cleanDoc(m[1].trim()) : '',
    })
  }
  const types: Record<string, string> = {}
  for (const t of API_TYPES) {
    const m = new RegExp(`export interface ${t} \\{[\\s\\S]*?\\n\\}`).exec(src)
    if (m) types[t] = m[0]
  }
  const sc = /export const OpcStatusCodes = \{[\s\S]*?\n\}/.exec(src)
  const ack = /export const OpcAcknowledge = \{[\s\S]*?\n\}/.exec(src)
  return { file, functions, types, statusCodes: sc ? sc[0] : '', acknowledge: ack ? ack[0] : '' }
}

// The library of the first template found (they are identical copies)
export async function readApiLibrary(): Promise<{ file: string; text: string }> {
  const dir = customDevDir()
  const names = [...Object.keys(TEMPLATES)]
  try {
    for (const e of await fs.readdir(dir, { withFileTypes: true }))
      if (e.isDirectory() && !names.includes(e.name)) names.push(e.name)
  } catch {
    /* reported below */
  }
  for (const n of names) {
    const file = path.join(dir, n, API_LIB_REL)
    try {
      return { file, text: await fs.readFile(file, 'utf8') }
    } catch {
      /* next */
    }
  }
  throw new Error(`scadaOpcApi.ts not found under ${dir}/<app>/${API_LIB_REL}`)
}

export function apiMarkdown(api: ApiReference): string {
  const fns = api.functions
    .map((f) => `### ${f.name}\n\n\`\`\`ts\n${f.signature}\n\`\`\`\n\n${f.doc}`)
    .join('\n\n')
  const types = Object.values(api.types)
    .map((t) => '```ts\n' + t + '\n```')
    .join('\n\n')
  return (
    `## scadaOpcApi reference (read from ${api.file.replace(/\\/g, '/')})\n\n` +
    'Import it as `import * as scadaOpcApi from \'../lib/scadaOpcApi\'` (path relative to the importing file).\n\n' +
    `${fns}\n\n### Data types\n\n${types}\n\n### Quality codes\n\n` +
    'Compare `DataPoint.quality` and command acknowledgements with these:\n\n```ts\n' +
    api.statusCodes +
    '\n```\n' +
    (api.acknowledge
      ? '\n### Acknowledge actions (ackAlarmsEvents bit mask)\n\n```ts\n' + api.acknowledge + '\n```\n'
      : '')
  )
}

export const CUSTOM_DEV_GUIDE = `# Developing custom developments

Custom developments are web apps (dashboards, equipment faceplates, special views) that run inside JSON-SCADA.
\`server_realtime_auth\` serves each folder of \`src/custom-developments/\` under \`/custom-developments/<folder>/\`,
and the AdminUI lists them under **Custom Developments**. The apps read and write SCADA data through the realtime
Web API (\`/Invoke/\`, the same OPC-style API as the HMI viewers) with the signed-in user's session, so the user's
role rights apply (commands, acknowledgements, property changes, station filters).

## How serving works

- Each folder with a built \`dist/index.html\` (or, for plain HTML apps, an \`index.html\` in the folder itself) gets a
  static route \`/custom-developments/<folder>\`. **Routes are created when \`server_realtime_auth\` starts**: after
  adding a new app, restart it (Windows \`nssm restart JSON_SCADA_server_realtime_auth\`, Linux
  \`sudo supervisorctl restart server_realtime_auth\`). Rebuilding an app that is already routed needs only a
  browser reload.
- A folder without built content is not routed and not listed; opening it answers 404 with a build hint.
- The templates' \`astro.config.mjs\` sets \`base: /custom-developments/<folder name>\` from the folder name, so the
  folder name **is** the URL: copying a template to a new folder needs no config change. Keep that line.
- Data calls are relative (\`fetch('/Invoke/')\`) and carry the login cookie, so an app works only when served by
  \`server_realtime_auth\` to a signed-in user; an expired session sends the browser to the login page.
  \`npm run dev\` (Astro dev server) has no API behind it: develop with build + reload.

## Templates

Astro 5 + React 18 + Tailwind + shadcn/ui components, TypeScript. Copy one as the starting point:

${Object.entries(TEMPLATES)
  .map(([k, v]) => `- \`${k}\`: ${v}`)
  .join('\n')}

## Workflow

1. Pick the template closest to the goal and copy its folder to a new folder name (letters, digits, \`-\`, \`_\`;
   it becomes the URL). Do not copy \`node_modules\` or \`dist\`.
2. In the copy, set \`name\` in \`package.json\`; replace the template's components, page title and texts.
3. Find the tags: \`search_points\` / \`get_point\` (names, types, units, \`origin: "command"\` for command tags), and
   \`check_system_tags\` for communication status tags. Use exact tag names.
4. Write React components in \`src/components\` and mount them in \`src/pages/index.astro\` with
   \`client:only="react"\` (the API uses the browser's \`fetch\` and \`window\`; never render API calls on the server).
5. \`npm install\` once, then \`npm run build\` (runs \`astro check\` for type errors, then writes \`dist/\`).
6. New app: restart \`server_realtime_auth\` once. Open \`/custom-developments/<folder>/\` (or AdminUI > Custom
   Developments) signed in, and reload after each rebuild.
7. Check it with \`check_custom_developments\` (build state, route, API library, referenced tags).

On an installed system, run npm with the bundled Node.js (Windows: \`c:\\json-scada\\platform-windows\\nodejs-runtime\\npm.cmd\`).

## Rules

- **Data only through \`src/lib/scadaOpcApi.ts\`.** Copy it unchanged from a template (all templates hold the same
  file); do not call MongoDB, PostgreSQL or the admin API from an app, and never put credentials in browser code.
- **Polling:** refresh realtime values every 1-5 s with \`setInterval\` (clear it on unmount); read history and events
  less often. Batch tags in one \`getRealTimeData\` call instead of one call per tag.
- **Quality:** \`DataPoint.quality\` is an OPC status code; show values whose quality is not
  \`OpcStatusCodes.Good\` as invalid (greyed, flagged). A tag missing from the result does not exist or the user may
  not see it.
- **Commands:** only on tags with \`origin: "command"\`; ask the operator to confirm; \`issueCommand\` returns
  \`{ ok, error, commandHandle }\`; then poll \`getCommandAckStatus(tag, commandHandle)\` every second for up to
  about 15 s until \`Good\` (acknowledged) or \`Bad\` (refused by the device), and report a timeout otherwise
  (\`transformer_with_command\` \`TransformerCard.tsx\` shows the full flow). The server refuses commands that are
  blocked or not allowed for the user's role.
- **Properties and alarms:** \`writeTagProperties\` changes annotation, alarm inhibition, limits, hysteresis, notes
  and substituted values; \`ackAlarmsEvents\` acknowledges alarms/events of a tag (\`OpcAcknowledge\` bit mask). Both
  work within the user's rights.
- Keep the app self-contained in its folder; shared UI pieces come from \`src/components/ui\` (shadcn).

## API

\`get_custom_development_guide\` with \`api: true\` (or the prompt \`custom-development\`) appends the reference of every
\`scadaOpcApi\` function, its data types and quality codes, read from the installed library.
`
