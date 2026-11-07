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

// Fields never returned to the MCP client (credentials/secrets)
const SENSITIVE_FIELDS = [
  'password',
  'passphrase',
  'tlsClientKeyPassword',
  'privateKeyFilePath',
  'privateKeyPassphrase',
]

// CIM-EMS network models, bindings and study results are critical infrastructure
// information: the MCP server exposes only the CIM-EMS driver status and result
// points (realtimeData), never these collections or the cimArtifacts bucket.
const CONFIDENTIAL_COLLECTIONS = [
  'cimModels',
  'cimMeasurementBindings',
  'cimRuns',
  'cimContingencyLists',
  'cimStudyRequests',
]

export function isConfidentialCollection(name: string): boolean {
  const n = String(name || '').trim()
  return (
    CONFIDENTIAL_COLLECTIONS.includes(n) || n.toLowerCase().startsWith('cimartifacts')
  )
}

export const confidentialResult = (name: string) =>
  textResult(`Collection '${name}' is confidential and not available through MCP.`, true)

export function textResult(text: string, isError = false) {
  return {
    content: [{ type: 'text' as const, text }],
    ...(isError ? { isError: true } : {}),
  }
}

export const jsonResult = (data: unknown) =>
  textResult(JSON.stringify(data, null, 2))

export function errorResult(prefix: string, e: unknown) {
  const msg = e instanceof Error ? e.message : String(e)
  return textResult(`${prefix}: ${msg}`, true)
}

export const notConnectedResult = () =>
  textResult('Database not connected.', true)

// Remove credential fields from documents before returning them to the client
export function redactSensitive<T extends Record<string, any>>(doc: T): T {
  const clone: Record<string, any> = { ...doc }
  for (const field of SENSITIVE_FIELDS) {
    if (field in clone) clone[field] = '*****'
  }
  return clone as T
}

export function redactSensitiveArray<T extends Record<string, any>>(
  docs: T[]
): T[] {
  return docs.map(redactSensitive)
}

export function parseDateArg(
  value: string | undefined,
  argName: string
): Date | undefined {
  if (value === undefined || value === '') return undefined
  const d = new Date(value)
  if (isNaN(d.getTime()))
    throw new Error(`Invalid ${argName} date: '${value}'. Use ISO 8601 format.`)
  return d
}
