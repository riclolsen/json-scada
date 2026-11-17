// Writes src/lib/calcFormulas.json, the formula catalog shown by the Tags edit
// dialog, from the MCP server's mirror of the calculations engine table
// (src/mcp-json-scada-db/src/jsonscada/calculations.ts, kept in step with
// src/calculations/formulas.go). Run after changing a formula:
//   node tools/gen-calc-formulas.mjs          (needs Node 23.6+ for .ts imports)
//   node tools/gen-calc-formulas.mjs --check  (fails when the file is stale)
import { readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { CALC_FORMULAS, USER_FORMULA_MIN } from '../../mcp-json-scada-db/src/jsonscada/calculations.ts'

const out = fileURLToPath(new URL('../src/lib/calcFormulas.json', import.meta.url))
const formulas = [...CALC_FORMULAS.values()].map((f) => ({
  code: f.code,
  name: f.name,
  expression: f.expression,
  parcels: f.parcels,
  result: f.result,
  quality: f.quality,
  ...(f.legacy ? { legacy: true } : {}),
  ...(f.notes ? { notes: f.notes } : {}),
}))
const text = JSON.stringify({ userFormulaMin: USER_FORMULA_MIN, formulas }, null, 1) + '\n'
if (process.argv.includes('--check')) {
  let cur = ''
  try {
    cur = readFileSync(out, 'utf8')
  } catch (e) {
    /* missing */
  }
  if (cur !== text) {
    console.error('calcFormulas.json is stale: run node tools/gen-calc-formulas.mjs')
    process.exit(1)
  }
  console.log('calcFormulas.json is current (' + formulas.length + ' formulas)')
} else {
  writeFileSync(out, text)
  console.log('wrote ' + out + ' (' + formulas.length + ' formulas)')
}
