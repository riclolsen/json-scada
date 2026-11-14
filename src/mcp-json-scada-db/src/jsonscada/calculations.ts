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

// Formula catalog of the compiled cyclic calculations engine (src/calculations,
// calculations.go). Each entry mirrors one `case` of the engine's switch: the
// parcel count it requires, how it computes the value and how it derives the
// invalid/transient flags. Keep in step with src/calculations/formulas.go and
// its README (calculations v0.2.0 and later).

export type ParcelRule =
  | { exact: number } // engine computes only with exactly n parcels
  | { min: number } // variadic; fewer than min is an error
  | { min: number; max: number } // between min and max parcels

export interface CalcResult {
  value: number
  invalid: boolean
  transient: boolean
}

export interface FormulaDef {
  code: number
  name: string
  expression: string
  parcels: ParcelRule
  result: 'analog' | 'digital' | 'any'
  quality: string
  // site-specific formulas kept for existing projects; prefer the general ones
  legacy?: boolean
  notes?: string[]
  evaluate: (v: number[], inv: boolean[]) => CalcResult
}

// Go float64 limits used by the engine
const MAX_FLOAT = Number.MAX_VALUE

const anyInv = (inv: boolean[]) => inv.some((b) => b)
const ok = (value: number, invalid: boolean, transient = false): CalcResult => ({
  value,
  invalid,
  transient,
})

const QUALITY_ALL = 'invalid if any parcel is invalid'

// P-term text for a coefficient
function term(c: number, i: number, first: boolean): string {
  const p = `P${i + 1}`
  const abs = Math.abs(c)
  const body = abs === 1 ? p : `${abs}*${p}`
  if (first) return c < 0 ? `-${body}` : body
  return c < 0 ? ` - ${body}` : ` + ${body}`
}

// Weighted sum with a fixed parcel count: sum(coeffs[i] * Pi) + constant
function linear(
  code: number,
  coeffs: number[],
  opts: { name?: string; expression?: string; constant?: number; legacy?: boolean; notes?: string[] } = {}
): FormulaDef {
  const constant = opts.constant ?? 0
  const expression =
    opts.expression ??
    coeffs.map((c, i) => term(c, i, i === 0)).join('') +
      (constant ? (constant < 0 ? ` - ${-constant}` : ` + ${constant}`) : '')
  return {
    code,
    name: opts.name ?? 'Weighted sum',
    expression,
    parcels: { exact: coeffs.length },
    result: 'analog',
    quality: QUALITY_ALL,
    ...(opts.legacy ? { legacy: true } : {}),
    ...(opts.notes ? { notes: opts.notes } : {}),
    evaluate: (v, inv) =>
      ok(
        coeffs.reduce((s, c, i) => s + c * (v[i] ?? 0), 0) + constant,
        anyInv(inv)
      ),
  }
}

// n plus signs followed by m minus signs
const pm = (plus: number, minus: number) => [
  ...Array<number>(plus).fill(1),
  ...Array<number>(minus).fill(-1),
]

const FORMULAS: FormulaDef[] = [
  {
    code: 1,
    name: 'Current from P, Q and voltage',
    expression: '(1000/sqrt(3)) * sqrt(P1^2 + P2^2) / P3   (0 when P3 <= 0)',
    parcels: { exact: 3 },
    result: 'analog',
    quality: QUALITY_ALL,
    notes: ['P1 = active power (MW), P2 = reactive power (Mvar), P3 = line voltage (kV); result in A'],
    evaluate: (v, inv) =>
      ok(v[2]! > 0 ? (577.35027 * Math.sqrt(v[0]! * v[0]! + v[1]! * v[1]!)) / v[2]! : 0, anyInv(inv)),
  },
  {
    code: 2,
    name: 'Power factor',
    expression: 'P1 / sqrt(P1^2 + P2^2)',
    parcels: { exact: 2 },
    result: 'analog',
    quality: QUALITY_ALL,
    notes: [
      'P1 = active power, P2 = reactive power',
      'When P1 = P2 = 0 the result is undefined: 0 with the invalid flag',
    ],
    evaluate: (v, inv) => ok(v[0]! / Math.sqrt(v[0]! * v[0]! + v[1]! * v[1]!), anyInv(inv)),
  },
  {
    code: 3,
    name: 'Apparent power from P and Q',
    expression: 'sqrt(P1^2 + P2^2)',
    parcels: { exact: 2 },
    result: 'analog',
    quality: QUALITY_ALL,
    notes: ['P1 = active power (MW), P2 = reactive power (Mvar); result in MVA'],
    evaluate: (v, inv) => ok(Math.sqrt(v[0]! * v[0]! + v[1]! * v[1]!), anyInv(inv)),
  },
  {
    code: 4,
    name: 'Sum',
    expression: 'P1 + P2 + ... + Pn',
    parcels: { min: 1 },
    result: 'analog',
    quality: QUALITY_ALL,
    evaluate: (v, inv) => ok(v.reduce((s, x) => s + x, 0), anyInv(inv)),
  },
  {
    code: 5,
    name: 'Square root',
    expression: 'sqrt(P1)',
    parcels: { exact: 1 },
    result: 'analog',
    quality: QUALITY_ALL,
    notes: ['A negative P1 gives 0 with the invalid flag'],
    evaluate: (v, inv) => ok(Math.sqrt(v[0]!), anyInv(inv)),
  },
  {
    code: 6,
    name: 'Logical AND',
    expression: '1 if every parcel != 0, else 0',
    parcels: { min: 1 },
    result: 'digital',
    quality: QUALITY_ALL,
    evaluate: (v, inv) => ok(v.every((x) => x !== 0) ? 1 : 0, anyInv(inv)),
  },
  {
    code: 7,
    name: 'Logical OR',
    expression: '1 if any parcel != 0, else 0',
    parcels: { min: 1 },
    result: 'digital',
    quality: QUALITY_ALL,
    evaluate: (v, inv) => ok(v.some((x) => x !== 0) ? 1 : 0, anyInv(inv)),
  },
  {
    code: 8,
    name: 'Timer (Unix time)',
    expression: 'current Unix time in seconds',
    parcels: { min: 0 },
    result: 'analog',
    quality: 'never invalid',
    notes: ['Needs no parcels (any given are ignored); the value changes every second, so it is written on every cycle'],
    evaluate: () => ok(Math.floor(Date.now() / 1000), false),
  },
  {
    code: 9,
    name: 'Apparent power from current and voltage',
    expression: 'P1 * P2 * sqrt(3) / 1000',
    parcels: { exact: 2 },
    result: 'analog',
    quality: QUALITY_ALL,
    notes: ['P1 = current (A), P2 = line voltage (kV); result in MVA'],
    evaluate: (v, inv) => ok((v[0]! * v[1]! * Math.sqrt(3)) / 1000, anyInv(inv)),
  },
  {
    code: 10,
    name: 'Negative sum',
    expression: '-(P1 + P2 + ... + Pn)',
    parcels: { min: 1 },
    result: 'analog',
    quality: QUALITY_ALL,
    evaluate: (v, inv) => ok(v.reduce((s, x) => s - x, 0), anyInv(inv)),
  },
  linear(11, [...Array<number>(23).fill(1), 0.72], { legacy: true }),
  linear(
    13,
    [
      ...pm(8, 1), // P1..P8 - P9
      1, 1, 1, // P10..P12
      0.52, // P13
      1, 1, -1, -1, // P14 + P15 - P16 - P17
      1, 1, 1, 1, 1, // P18..P22
      -1, -1, // P23, P24
      ...Array<number>(9).fill(1), // P25..P33
      0.2, // P34
    ],
    {
      constant: -1,
      legacy: true,
    }
  ),
  linear(14, [10 / 6], { name: 'Scale', expression: '(P1 * 10) / 6' }),
  linear(15, pm(1, 1), { name: 'Difference' }),
  linear(16, pm(1, 2)),
  linear(17, pm(1, 3)),
  linear(18, pm(1, 5)),
  linear(19, pm(2, 1)),
  linear(20, pm(3, 3)),
  linear(21, pm(4, 5)),
  linear(22, pm(6, 3)),
  linear(23, pm(10, 1)),
  linear(24, pm(3, 6)),
  {
    code: 26,
    name: 'Band selector',
    expression: 'P2, but 0 when abs(P1) > 1.4 and 1 when abs(P1) <= 0.5',
    parcels: { exact: 2 },
    result: 'any',
    quality: QUALITY_ALL,
    evaluate: (v, inv) => {
      let r = v[1]!
      if (Math.abs(v[0]!) > 1.4) r = 0
      if (Math.abs(v[0]!) <= 0.5) r = 1
      return ok(r, anyInv(inv))
    },
  },
  ...[50, 51].map(
    (code): FormulaDef => ({
      code,
      name: 'Choice of valid parcel (backup sources)',
      expression: 'value of the first valid parcel in the list; P1 when none is valid',
      parcels: { min: 1 },
      result: 'any',
      quality: 'invalid only when no parcel is valid',
      notes: ['List the preferred source first, then the backups', '50 and 51 are identical'],
      evaluate: (v, inv) => {
        const i = inv.findIndex((b) => !b)
        return i >= 0 ? ok(v[i]!, false) : ok(v[0]!, true)
      },
    })
  ),
  {
    code: 52,
    name: 'Any parcel valid',
    expression: '1 if any parcel is valid, else 0',
    parcels: { min: 1 },
    result: 'digital',
    quality: 'never invalid',
    evaluate: (_v, inv) => ok(inv.some((b) => !b) ? 1 : 0, false),
  },
  {
    code: 53,
    name: 'Span (max - min)',
    expression: 'max(P1..Pn) - min(P1..Pn)',
    parcels: { min: 1 },
    result: 'analog',
    quality: QUALITY_ALL,
    notes: ['Give at least two parcels; with one the result is always 0'],
    evaluate: (v, inv) => ok(Math.max(...v) - Math.min(...v), anyInv(inv)),
  },
  {
    code: 54,
    name: 'Double point from two single points',
    expression: 'P1 != 0 and P2 == 0 -> 1 (ON); P1 == 0 and P2 != 0 -> 0 (OFF); P1 == P2 -> invalid + transient',
    parcels: { exact: 2 },
    result: 'digital',
    quality: 'invalid (and transient) when both inputs are equal, or when any parcel is invalid',
    notes: [
      'P1 = ON/closed indication, P2 = OFF/open indication',
    ],
    evaluate: (v, inv) => {
      let r = v[0]!
      let bad = false
      let tr = false
      if (v[0] === 0 && v[1] !== 0) r = 0
      if (v[0] !== 0 && v[1] === 0) r = 1
      if (v[0] === v[1]) {
        tr = true
        bad = true
      }
      return ok(r, bad || anyInv(inv), tr)
    },
  },
  {
    code: 55,
    name: 'Division',
    expression: 'P1 / P2   (P2 = 0: 0 when P1 = 0, else +/- max float)',
    parcels: { exact: 2 },
    result: 'analog',
    quality: QUALITY_ALL,
    evaluate: (v, inv) => {
      let r: number
      if (v[1] === 0) r = v[0] === 0 ? 0 : v[0]! > 0 ? MAX_FLOAT : -MAX_FLOAT
      else r = v[0]! / v[1]!
      return ok(r, anyInv(inv))
    },
  },
  linear(200, pm(1, 7)),
  linear(201, pm(8, 3)),
  linear(202, [60, 1], { name: 'Minutes and seconds', expression: '(P1 * 60) + P2' }),
  {
    code: 203,
    name: 'Selector',
    expression: 'P1 != 0 ? P3 : P2',
    parcels: { exact: 3 },
    result: 'any',
    quality: 'quality of the selected parcel (P1 quality is ignored)',
    evaluate: (v, inv) => (v[0] !== 0 ? ok(v[2]!, inv[2]!) : ok(v[1]!, inv[1]!)),
  },
  linear(204, [0.5], { name: 'Half', expression: 'P1 / 2' }),
  linear(205, pm(2, 3)),
  linear(206, pm(1, 8)),
  linear(207, pm(2, 4)),
  linear(208, pm(1, 4)),
  linear(209, pm(2, 7)),
  linear(210, pm(1, 6)),
  linear(211, pm(5, 4)),
  linear(213, pm(3, 1)),
  linear(214, pm(4, 1)),
  linear(215, pm(2, 2)),
  ...(
    [
      [216, 'P2 <= P1 <= P3', (a: number, lo: number, hi: number) => lo <= a && a <= hi],
      [218, 'P2 < P1 <= P3', (a: number, lo: number, hi: number) => lo < a && a <= hi],
      [219, 'P2 <= P1 < P3', (a: number, lo: number, hi: number) => lo <= a && a < hi],
    ] as const
  ).map(
    ([code, cond, test]): FormulaDef => ({
      code,
      name: 'Range check',
      expression: `1 if ${cond}, else 0`,
      parcels: { exact: 3 },
      result: 'digital',
      quality: 'quality of P1 only (the limits P2, P3 are not checked)',
      evaluate: (v, inv) => ok(test(v[0]!, v[1]!, v[2]!) ? 1 : 0, inv[0]!),
    })
  ),
  linear(217, pm(5, 1)),
  {
    code: 220,
    name: 'Minimum of two',
    expression: 'min(P1, P2)',
    parcels: { exact: 2 },
    result: 'analog',
    quality: QUALITY_ALL,
    evaluate: (v, inv) => ok(Math.min(v[0]!, v[1]!), anyInv(inv)),
  },
  {
    code: 221,
    name: 'Maximum of two',
    expression: 'max(P1, P2)',
    parcels: { exact: 2 },
    result: 'analog',
    quality: QUALITY_ALL,
    evaluate: (v, inv) => ok(Math.max(v[0]!, v[1]!), anyInv(inv)),
  },
  linear(222, pm(3, 4)),
  linear(223, pm(3, 5)),
  linear(224, pm(6, 1)),
  {
    code: 225,
    name: 'Constant 1',
    expression: '1',
    parcels: { min: 0, max: 1 },
    result: 'digital',
    quality: 'never invalid',
    legacy: true,
    notes: ['The parcel, if any, is ignored'],
    evaluate: () => ok(1, false),
  },
  linear(226, pm(2, 6)),
  linear(227, pm(3, 2)),
  linear(228, pm(3, 9)),
  linear(229, pm(9, 3)),
  linear(230, pm(9, 1)),
  {
    code: 231,
    name: 'Negative part',
    expression: 'P1 > 0 ? 0 : P1',
    parcels: { exact: 1 },
    result: 'analog',
    quality: QUALITY_ALL,
    evaluate: (v, inv) => ok(v[0]! > 0 ? 0 : v[0]!, anyInv(inv)),
  },
  linear(232, pm(5, 3)),
  linear(233, pm(7, 4)),
  {
    code: 500,
    name: 'Logical OR',
    expression: '1 if any parcel != 0, else 0',
    parcels: { min: 1 },
    result: 'digital',
    quality: QUALITY_ALL,
    legacy: true,
    notes: ['Same as formula 7'],
    evaluate: (v, inv) => ok(v.some((x) => x !== 0) ? 1 : 0, anyInv(inv)),
  },
  linear(680, pm(3, 2), { legacy: true }),
  linear(5000, [1, 1, 1, -1, -0.6, 1], { legacy: true }),
  linear(5022, [1, 0.65], { legacy: true }),
  linear(5041, [1, 1, -1, 1, -1], { legacy: true }),
  linear(5043, [1, 1, 1, 1, -1, -1, 1, -1, -1], { legacy: true }),
  linear(5172, [-0.72], { legacy: true }),
  linear(5173, [0.38], { legacy: true }),
  linear(5174, [0.14], { legacy: true }),
  linear(5406, [-1, 1, 1, 1, 1, 1, -1, -1, 1, 1, 1], { legacy: true }),
  linear(5711, [-1, -1, -1, -1], { legacy: true }),
  linear(7374, pm(1, 11), { legacy: true }),
  linear(8055, pm(2, 6), { legacy: true }),
  linear(25673, pm(1, 1), { legacy: true, notes: ['Same as formula 15'] }),
]

FORMULAS.sort((a, b) => a.code - b.code)

export const CALC_FORMULAS: ReadonlyMap<number, FormulaDef> = new Map(
  FORMULAS.map((f) => [f.code, f])
)

// Codes from here up are reserved for user processes (e.g. a custom change
// stream processor); the calculations engine ignores them.
export const USER_FORMULA_MIN = 100000

export const CALCULATIONS_PROCESS = 'CALCULATIONS'

// the engine reloads the calculated point definitions this often (formulas.go reloadPeriod)
export const CALC_RELOAD_SECONDS = 30

export function parcelRuleText(r: ParcelRule): string {
  if ('exact' in r) return r.exact === 1 ? 'exactly 1 parcel' : `exactly ${r.exact} parcels`
  if ('max' in r) return `${r.min} to ${r.max} parcels`
  return `${r.min} or more parcels`
}

export function parcelsFit(r: ParcelRule, n: number): boolean {
  if ('exact' in r) return n === r.exact
  return n >= r.min && (!('max' in r) || n <= r.max)
}

// Engine output for a formula (formulas.go calculate): NaN and infinite
// results become 0, invalid.
export function calculate(def: FormulaDef, v: number[], inv: boolean[]): CalcResult {
  const r = def.evaluate(v, inv)
  return Number.isFinite(r.value) ? r : { value: 0, invalid: true, transient: r.transient }
}

// Problems the engine would have with this formula / parcel count.
// errors: the engine skips the point (never computed); warnings: computed,
// but probably not what was meant.
export function checkFormula(
  formula: unknown,
  parcelCount: number
): { def?: FormulaDef; errors: string[]; warnings: string[] } {
  const errors: string[] = []
  const warnings: string[] = []
  const code = Number(formula)
  if (!Number.isInteger(code) || code <= 0) {
    errors.push(`formula must be a positive integer code (got ${JSON.stringify(formula)})`)
    return { errors, warnings }
  }
  const def = CALC_FORMULAS.get(code)
  if (!def) {
    if (code >= USER_FORMULA_MIN)
      warnings.push(
        `formula ${code} is in the user range (>= ${USER_FORMULA_MIN}): the calculations engine ignores it, ` +
          'so a custom process (e.g. cs_custom_processor) must compute this point'
      )
    else
      errors.push(
        `formula ${code} is not implemented by the calculations engine: the point would never be computed ` +
          '(see get_calculations_guide for the available codes)'
      )
    return { errors, warnings }
  }
  if (!parcelsFit(def.parcels, parcelCount))
    errors.push(
      `formula ${code} needs ${parcelRuleText(def.parcels)}, got ${parcelCount}: the engine skips the point ` +
        '(logged in calculations.log) and never computes it'
    )
  else if ((code === 8 || code === 225) && parcelCount > 0)
    warnings.push(`formula ${code} ignores its parcels`)
  if (code === 53 && parcelCount === 1) warnings.push('formula 53 with one parcel is always 0')
  if (def.legacy)
    warnings.push(
      `formula ${code} is a site-specific legacy formula; prefer a general one (e.g. 4 sum, 15 difference, ` +
        '19..24 / 200..233 signed sums) unless this exact expression is wanted'
    )
  return { def, errors, warnings }
}

// What the data processor stores for the engine output of a point: analog
// points get kconv1/kconv2; digital points with kconv1 = -1 are inverted
// (cs_data_processor, sourceDataUpdate handling; it also turns NaN into 0 + invalid).
export function processedValue(
  r: CalcResult,
  point: { type?: string; kconv1?: number; kconv2?: number }
): CalcResult {
  let { value, invalid } = r
  if (value === null || value === undefined || Number.isNaN(value)) {
    value = 0
    invalid = true
  }
  if (point.type === 'analog') value = value * (point.kconv1 ?? 1) + (point.kconv2 ?? 0)
  else if (point.type === 'digital' && point.kconv1 === -1) value = value === 0 ? 1 : 0
  return { value, invalid, transient: r.transient }
}

// One Markdown table row per formula; src/calculations/README.md uses the same rows
export function formulaTable(legacy: boolean): string {
  return FORMULAS.filter((f) => !!f.legacy === legacy)
    .map(
      (f) =>
        `| ${f.code} | ${f.name} | \`${f.expression}\` | ${parcelRuleText(f.parcels).replace(/ parcels?$/, '')} | ` +
        `${f.result} | ${f.quality}${f.notes?.length ? '. ' + f.notes.join('. ') : ''} |`
    )
    .join('\n')
}

export const CALCULATIONS_GUIDE = `# Creating calculated tags (compiled calculations engine)

The \`calculations\` process (src/calculations, Go; process name \`CALCULATIONS\`; this guide matches v0.2.0 and later)
recomputes every calculated point on a fixed cycle (default 2 s, 3rd command-line argument / \`JS_CALCULATIONS_PERIOD\`)
from predefined, compiled formulas. There is no expression language: you pick a formula **code** and list its input
points (**parcels**) in the order the formula expects.

## How the engine works

- It computes the \`realtimeData\` points with \`origin: "calculated"\` and \`formula > 0\`. It reloads them every
  ${CALC_RELOAD_SECONDS} s, so new or changed calculated points start computing within ${CALC_RELOAD_SECONDS} s, without a restart.
- Each cycle it reads \`value\` and \`invalid\` of all parcels, computes, and writes the result as \`sourceDataUpdate\`
  (\`valueAtSource\`, \`invalidAtSource\`, \`transientAtSource\`) when it differs from the last one written. The data
  processor (cs_data_processor) then finishes the update like for any acquired point: conversion (\`kconv1\`/\`kconv2\`
  for analogs, inversion with \`kconv1 = -1\` for digitals), limits and alarms, events, history. The data processor
  must be running.
- Parcels are read from their processed \`value\` (after their own conversion), so units are the engineering units
  shown on the HMI.
- Only the active node of a redundant pair computes (\`processInstances\` election).
- Latency is up to one cycle. A calculated point can be a parcel of another; each level adds up to one more cycle.
  For low latency, write a custom change stream process (cs_custom_processor) and give its points a formula
  code >= ${USER_FORMULA_MIN}, which the engine ignores.

## Point fields

| Field | Value |
|---|---|
| \`origin\` | \`"calculated"\` (required; formulas on other origins are ignored) |
| \`formula\` | integer code from the tables below; 0 = not calculated |
| \`parcels\` | array of point keys (\`_id\` numbers, **not** tag names), in formula order P1, P2, ... |
| \`type\` | \`analog\` for numeric results, \`digital\` for 0/1 results (see the result column) |
| \`kconv1\`, \`kconv2\` | normally 1 and 0; they are applied to the result |
| others | as for any point: \`tag\`, \`description\`, \`group1..3\`, \`unit\`, \`stateTextTrue/False\`, limits, \`isEvent\`, \`priority\`, \`historianDeadBand\`, ... |

No protocol fields (\`protocolSource*\`) are needed.

## Rules and pitfalls

- **The parcel count must fit the formula.** Otherwise the engine skips the point (and logs it in
  \`calculations.log\`): it is never computed.
- **Every parcel key must exist.** A missing parcel counts as 0, invalid, so the result stays invalid.
- A point must not be its own parcel, directly or through a chain of calculated points (feedback loop).
- Quality: most formulas mark the result invalid when any parcel is invalid; see the quality column for exceptions.
  The result is never flagged substituted or not-topical; old parcel values are used as they are.
- Undefined results (power factor with P = Q = 0, square root of a negative) are 0 with the invalid flag.
  Division by zero (55) gives +/- the largest float, valid, unless P1 is 0.
- Formulas 11, 13, 225, 500, 680 and 5000+ are site-specific legacy formulas. Prefer the general ones.

## Workflow with the MCP tools

1. Find the parcel points: \`search_points\` / \`get_point\` (note their \`_id\`, type, unit).
2. Pick the formula from the tables below (\`get_calculations_guide\` with \`formula\` for one entry).
3. Check the design: \`check_calculated_tag\` with \`formula\` and \`parcels\` (tag names or keys). It validates the
   count, the parcels, the types, and previews the value from the current parcel values.
4. Create or change the point: \`save_calculated_tag\` (needs the admin API), which runs the same checks and refuses
   a broken design. \`create_tag\`/\`update_tag\` with \`origin\`, \`formula\` and \`parcels\` in \`fields\` also work
   but do not check anything.
5. After about ${CALC_RELOAD_SECONDS} s plus one cycle, confirm with \`check_calculated_tag\` on the tag: the stored value
   should match the preview.
6. \`audit_calculated_tags\` lists every calculated point with problems (wrong count, missing parcels, loops,
   formulas on non-calculated points, never computed) and the calculations process status.

## General formulas

| Code | Name | Expression | Parcels | Result | Quality |
|---|---|---|---|---|---|
${formulaTable(false)}

## Legacy (site-specific) formulas

| Code | Name | Expression | Parcels | Result | Quality |
|---|---|---|---|---|---|
${formulaTable(true)}

Codes not listed (12, 25, 27..49, 56..199, 212, 234..99999 except the legacy ones) are not implemented:
points using them are never computed. Codes >= ${USER_FORMULA_MIN} are reserved for user processes.
`
