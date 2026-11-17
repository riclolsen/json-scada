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

// Settings of the HTTP service saved from the AdminUI (Admin > System > MCP
// server) in the systemSettings document { key: 'mcpServer' }, written and
// validated by server_realtime_auth (app/services/mcp-settings.js; keep the
// field names in step). Read once at startup in HTTP mode: a stored value
// replaces the matching JS_MCPJSDB_* environment variable before anything
// reads it. stdio servers, started by an AI client with its own environment,
// never read them.

import { MongoClient } from 'mongodb'
import Log from './logger.js'
import { IConfig } from './config.js'
import packageInfo from '../../package.json' with { type: 'json' }

const ENV_PREFIX = packageInfo.config.envPrefix || 'JS_MCPJSDB_'

interface IMcpServiceSettings {
  ipBind?: string
  httpPort?: number
  allowedHosts?: string[]
  logLevel?: number
  adminTools?: boolean
  adminUsername?: string
  adminPassword?: string
  adminUrl?: string
  cimEmsTools?: boolean
}

const ATTEMPTS = 6
const RETRY_MS = 5000

async function readSettings(cfg: IConfig): Promise<IMcpServiceSettings | null> {
  for (let i = 1; i <= ATTEMPTS; i++) {
    let client: MongoClient | null = null
    try {
      client = await MongoClient.connect(cfg.mongoConnectionString, {
        ...cfg.MongoConnectionOptions,
        serverSelectionTimeoutMS: 5000,
      })
      const doc = await client
        .db(cfg.mongoDatabaseName)
        .collection('systemSettings')
        .findOne({ key: 'mcpServer' })
      return (doc as IMcpServiceSettings | null) || null
    } catch (err) {
      Log.log(`Service settings: MongoDB read failed (attempt ${i}/${ATTEMPTS}): ${(err as Error).message}`)
      if (i < ATTEMPTS) await new Promise((r) => setTimeout(r, RETRY_MS))
    } finally {
      if (client) await client.close().catch(() => {})
    }
  }
  return null
}

// Applies the stored settings to process.env (and the log level); returns
// the names of the settings applied, for the log. Values of the wrong type
// are ignored, so the environment stays in force for them.
export async function applyServiceSettings(cfg: IConfig): Promise<string[]> {
  const s = await readSettings(cfg)
  if (!s) return []
  const applied: string[] = []
  const setEnv = (name: string, value: string, label: string) => {
    process.env[ENV_PREFIX + name] = value
    applied.push(label)
  }

  if (typeof s.ipBind === 'string' && s.ipBind !== '') setEnv('IP_BIND', s.ipBind, 'ipBind')
  if (Number.isInteger(s.httpPort)) setEnv('HTTP_PORT', String(s.httpPort), 'httpPort')
  if (Array.isArray(s.allowedHosts))
    setEnv(
      'ALLOWED_HOSTS',
      s.allowedHosts.filter((h) => typeof h === 'string').join(','),
      'allowedHosts'
    )
  if (typeof s.adminUrl === 'string' && s.adminUrl !== '') setEnv('ADMIN_URL', s.adminUrl, 'adminUrl')
  if (s.adminTools === false) {
    setEnv('ADMIN_USERNAME', '', 'adminTools')
    process.env[ENV_PREFIX + 'ADMIN_PASSWORD'] = ''
  } else {
    if (typeof s.adminUsername === 'string' && s.adminUsername !== '')
      setEnv('ADMIN_USERNAME', s.adminUsername, 'adminUsername')
    if (typeof s.adminPassword === 'string' && s.adminPassword !== '')
      setEnv('ADMIN_PASSWORD', s.adminPassword, 'adminPassword')
  }
  if (typeof s.cimEmsTools === 'boolean')
    setEnv('CIM_EMS_TOOLS', s.cimEmsTools ? 'true' : 'false', 'cimEmsTools')
  if (Number.isInteger(s.logLevel)) {
    Log.levelCurrent = s.logLevel as number
    applied.push('logLevel')
  }
  return applied
}
