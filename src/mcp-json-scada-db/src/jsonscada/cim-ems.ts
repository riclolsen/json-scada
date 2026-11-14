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

// CIM-EMS driver (src/cim-ems) conventions used to interpret its result points
// and run records. Keep in step with cim_ems/cycle.py (status codes),
// cim_ems/contingency.py (CA codes) and cim_ems/publish.py (point addressing).

export const CIM_EMS_DRIVER = 'CIM-EMS'

// <connection>;SYS;LF;STATUS (cycle.STATUS_CODES)
export const LF_STATUS: Record<number, string> = {
  0: 'converged',
  1: 'diverged',
  2: 'rejected-snapshot',
  3: 'unsupported',
  4: 'failed',
}

// <connection>;SYS;SE;STATUS (cycle.SE_CODES)
export const SE_STATUS: Record<number, string> = {
  0: 'converged',
  1: 'not-observable',
  2: 'diverged',
  3: 'unsupported',
  4: 'failed',
  5: 'no-load-flow',
}

// <connection>;SYS;CA;STATUS (cycle._ca_sys)
export const CA_RUN_STATUS: Record<number, string> = {
  0: 'done',
  1: 'skipped',
  2: 'failed',
  3: 'no-load-flow',
}

// <connection>;CA;<contingency>;STATUS (contingency.STATUS_CODES)
export const CA_STATUS: Record<number, string> = {
  0: 'secure',
  1: 'violation',
  2: 'islanding',
  3: 'diverged',
  4: 'unsupported',
}

export const CA_INSECURE = ['violation', 'islanding', 'diverged']

export const CIM_EMS_GLOSSARY = {
  kinds: {
    LF: 'AC load flow solved from the telemetered topology (switch states) and injections',
    SE: 'Weighted least squares state estimate from the approved analog measurements',
    CA: 'N-1 contingency (security) analysis on the solved base case',
    SYS: 'Health of the last run of each study (status, convergence, duration)',
  },
  quantities: {
    V: 'bus voltage magnitude (kV)',
    VPU: 'bus voltage in per unit of the nominal voltage',
    ANG: 'bus voltage angle (degrees)',
    P: 'active power flow at a branch end (MW, positive leaving the bus into the branch)',
    Q: 'reactive power flow at a branch end (Mvar)',
    I: 'current at a branch end (A)',
    LOAD: 'branch loading: highest end current against the permanent current limit (%)',
    STATUS: 'contingency outcome code (CA) or study status code (SYS)',
    VIOL: 'number of limit violations caused by a contingency (base case violations excluded)',
    WORST: 'worst post-contingency branch loading (%)',
  },
}

export interface ParsedResultPoint {
  kind: string
  elementId: string
  side: number
  quantity: string
  name: string
}

// Result point identity from its realtimeData document: protocolSourceCommonAddress
// is the study kind and protocolSourceObjectAddress is "<mRID>|<side>|<quantity>";
// the readable name comes from the tag "<conn>;<kind>;<name>;<quantity>[;<side>]".
export function parseResultPoint(p: any): ParsedResultPoint | null {
  const addr = String(p?.protocolSourceObjectAddress ?? '')
  const parts = addr.split('|')
  if (parts.length < 3) return null
  const quantity = parts[parts.length - 1] || ''
  const side = Number(parts[parts.length - 2]) || 0
  const elementId = parts.slice(0, -2).join('|')
  const kind = String(p?.protocolSourceCommonAddress ?? '')
  const tagParts = String(p?.tag ?? '').split(';')
  const nameParts = side ? tagParts.slice(2, -2) : tagParts.slice(2, -1)
  const name = nameParts.join(';') || elementId
  return { kind, elementId, side, quantity, name }
}

export const codeText = (table: Record<number, string>, v: unknown) =>
  typeof v === 'number' && Number.isFinite(v)
    ? table[Math.round(v)] ?? `unknown(${v})`
    : null

export const round = (v: unknown, digits = 3) =>
  typeof v === 'number' && Number.isFinite(v)
    ? Math.round(v * 10 ** digits) / 10 ** digits
    : null

export function ageSeconds(t: unknown, now = Date.now()): number | null {
  return t instanceof Date ? Math.round((now - t.getTime()) / 1000) : null
}

export function compileRegex(
  value: string | undefined,
  argName: string
): RegExp | undefined {
  if (!value) return undefined
  try {
    return new RegExp(value, 'i')
  } catch {
    throw new Error(`Invalid regular expression in ${argName}: '${value}'`)
  }
}

// Quantity of an analog tag as the driver's name matcher infers it
// (cim_ems/bindings/proposal.py quantity_of): unit, else the last quantity word
// of the tag, else the description
const UNIT_QUANTITY: Record<string, string> = { kv: 'V', mw: 'P', mvar: 'Q', mvr: 'Q' }
const WORD_QUANTITY: Record<string, string> = { p: 'P', mw: 'P', q: 'Q', mvar: 'Q', mvr: 'Q', v: 'V', kv: 'V', u: 'V' }
const DESCRIPTION_QUANTITY: [string, string][] = [
  ['reactive', 'Q'],
  ['active', 'P'],
  ['voltage', 'V'],
]

const asciiLower = (s: unknown) =>
  String(s ?? '')
    .normalize('NFKD')
    .replace(/[^\x00-\x7f]/g, '')
    .toLowerCase()

export function quantityOfTag(tag: { tag?: string; unit?: string; description?: string }): string | null {
  const unit = asciiLower(tag.unit).trim()
  if (UNIT_QUANTITY[unit]) return UNIT_QUANTITY[unit]
  const words = asciiLower(tag.tag).split(/[^a-z0-9]+/).reverse()
  for (const w of words) if (WORD_QUANTITY[w]) return WORD_QUANTITY[w]
  const desc = asciiLower(tag.description)
  for (const [w, q] of DESCRIPTION_QUANTITY) if (desc.includes(w)) return q
  return null
}

// Served by the cim_ems_get_guide tool and the json-scada://cim-ems/guide
// resource: how to read and judge the CIM-EMS results
export const CIM_EMS_GUIDE = `# Interpreting CIM-EMS results

## Studies and data sources
- LF (load flow): AC power flow on the CGMES model with the live topology (switch states and tap positions from
  bound SCADA points) and the model injections. It answers "what the network does with this topology".
- SE (state estimation): weighted least squares fit of the bus voltages to the bound analog measurements (P/Q at
  branch ends, V at busbars), started from the solved load flow. It answers "what the network is doing now" and
  finds measurements that disagree with the rest (bad data).
- CA (contingency analysis): N-1 security analysis on the solved load flow: each contingency (outage of a line or
  transformer, or a user list) is simulated and post-contingency limit violations are reported.

## Health first (cim_ems_status)
- LF status: converged / diverged (no solution: check islands, extreme setpoints, topology) / rejected-snapshot
  (telemetry unusable) / unsupported / failed. Results older than resultMaxAgeSeconds turn invalid.
- topologyUncertain: a switch state was invalid or held; the topology may not match the field.
- SE status: converged / not-observable (too few measurements for some area) / diverged / unsupported / no-load-flow.
- CA status: done / skipped (no topology change and interval not elapsed: last results are kept) / failed.

## Sign and unit conventions
- P and Q at a branch end are the power entering the branch from that end's bus (load convention):
  P1 > 0 and P2 < 0 means power flows from end 1 to end 2. P1 + P2 is the branch active loss (>= 0).
- V in kV, VPU per unit of the voltage level nominal, ANG in degrees (relative to the slack).
- LOAD = highest end current / permanent current limit in %. > 100 % is an overload; 90-100 % is high.
- Run totals (cim_ems_list_runs, cim_ems_run_statistics): generation, load, losses in MW; lossesPct of generation
  (typical transmission 1-3 %, distribution 3-8 %).

## Voltage judgement
- Usual operating band 0.95-1.05 pu (some grids 0.90-1.10; EHV often run 1.0-1.1). A whole area high or low
  points to reactive power balance (shunts, generator Q, tap settings); a single bus off-band with neighbours
  normal points to a measurement, topology or model parameter issue.
- VPU 0 or "not calculated" means de-energized (isolated by open switches).

## State estimator judgement
- normalized residual r = (measured - estimated) / sigma. |r| < 3 is consistent; 3 <= |r| < badDataThreshold
  (default 4) is suspect; above it the measurement was removed as bad data (largest first, then re-estimated).
- chi-square test: chi2Ratio = statistic / threshold. <= 1 passes (measurements consistent with the model and
  sigmas); > 1 means remaining errors or sigmas too small. chi2Ratio far below 1 means sigmas are too large.
- Typical causes of large residuals: sign convention (measured ~= -estimated: set binding sign -1), wrong side
  (P1 measured as P2), wrong element (tag bound to a parallel circuit), scale/unit (kW vs MW: ratio ~1000),
  stale or frozen value, wrong topology (a switch state wrong, so a flow appears on an open branch).
- A group of residuals in one substation usually means a topology error there (switch status) rather than many
  bad meters. A consistent bias of all Q measurements suggests a sign or CT/VT scaling issue.
- Measurements dropped before the estimate (invalid, older than max_age_seconds, time skew) are listed in the
  snapshot reasons; many drops reduce redundancy and can make areas unobservable.
- Compare SE with LF (cim_ems_get_results compareWith): large differences show where the model injections
  (loads/generation) differ from reality; SE is the better picture where it is observable.

## Contingency judgement
- Outcome per contingency: secure; violation (post-contingency limit exceeded); islanding (the outage splits the
  network; check what is disconnected); diverged (no post-contingency solution: possible voltage collapse or a
  load/generation island without balance); unsupported (element not found or not simulable).
- Violations already present in the base case are not counted against a contingency; fix the base case first.
- worstLoadingPct ranks contingencies; mostViolatedElements shows the weak elements limiting the system (the same
  element overloaded by many outages).
- N-1 secure means every contingency is secure. Typical remedies to discuss: redispatch, topology changes
  (bus coupler switching), tap/reactive control, load transfer; the tools cannot simulate remedies, propose them
  for the operator.
`

// Hints for a measurement whose residual is large: compares measured and
// estimated values for typical binding errors
export function residualHints(m: { measured?: number | null; estimated?: number | null; normalized?: number | null; quantity?: string }): string[] {
  const out: string[] = []
  const a = m.measured, e = m.estimated
  if (typeof a !== 'number' || typeof e !== 'number') return out
  const mag = Math.max(Math.abs(a), Math.abs(e))
  if (mag > 1 && Math.sign(a) !== Math.sign(e) && Math.abs(a + e) < 0.2 * mag)
    out.push('sign: measured is about -estimated (check binding sign or flow direction convention)')
  if (Math.abs(e) > 1e-3 && Math.abs(a) > 1e-3) {
    const ratio = Math.abs(a / e)
    if (ratio > 500 && ratio < 2000) out.push('scale: measured ~1000x estimated (kW/kV/A units?)')
    else if (ratio > 0.0005 && ratio < 0.002) out.push('scale: measured ~1/1000 of estimated (units?)')
    else if (m.quantity === 'V' && (Math.abs(ratio - Math.sqrt(3)) < 0.05 || Math.abs(ratio - 1 / Math.sqrt(3)) < 0.03))
      out.push('scale: phase vs line voltage (sqrt 3)')
  }
  if (Math.abs(a) < 1e-6 && Math.abs(e) > 1) out.push('measured exactly 0: stale/forced value or element open in the field')
  if (Math.abs(e) < 1e-3 && Math.abs(a) > 1) out.push('estimated ~0 but measured flow: element open in the model (topology/switch status)')
  return out
}
