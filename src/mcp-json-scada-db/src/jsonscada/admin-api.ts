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

// Client for the AdminUI backend API (server_realtime_auth, /Invoke/auth/*).
// Configuration changes and OS service (process) management go through this API
// instead of writing MongoDB directly, so they get the same validation,
// protocol defaults, auto-restart scheduling, NSSM/supervisord service control
// and userActions audit trail as changes made in the AdminUI.

import Log from './logger.js'
import packageInfo from '../../package.json' with { type: 'json' }

const ENV_PREFIX = packageInfo.config.envPrefix || 'JS_MCPJSDB_'
const API_PATH = '/Invoke/auth/'

export class AdminApiClient {
  readonly baseUrl: string
  private readonly username: string
  private readonly password: string
  private token: string | null = null

  constructor() {
    this.baseUrl = (
      process.env[ENV_PREFIX + 'ADMIN_URL'] || 'http://127.0.0.1:8080'
    ).replace(/\/+$/, '')
    this.username = process.env[ENV_PREFIX + 'ADMIN_USERNAME'] || ''
    this.password = process.env[ENV_PREFIX + 'ADMIN_PASSWORD'] || ''
  }

  // Admin tools are only offered when admin credentials are configured
  get enabled(): boolean {
    return this.username !== '' && this.password !== ''
  }

  get user(): string {
    return this.username
  }

  private async signin(): Promise<string> {
    const res = await fetch(this.baseUrl + API_PATH + 'signin', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        username: this.username,
        password: this.password,
      }),
    })
    const body: any = await res.json().catch(() => ({}))
    if (!res.ok || body?.ok !== true)
      throw new Error(
        `Admin API sign-in failed for user '${this.username}': ${
          body?.message || res.status + ' ' + res.statusText
        }`
      )
    // the access token is only returned as an http-only cookie
    for (const cookie of res.headers.getSetCookie()) {
      const m = /^x-access-token=([^;]+)/.exec(cookie)
      if (m && m[1]) return decodeURIComponent(m[1])
    }
    throw new Error('Admin API sign-in returned no access token.')
  }

  // 401/403 from the token checks mean an expired or rejected token. A 403
  // carrying { error } is a missing right (e.g. CIM-EMS engineering), not a
  // token problem: it is reported as is, without signing in again.
  private static isAuthFailure(status: number, body: any): boolean {
    if (status === 401) return true
    if (status === 403) return !(body && typeof body.error === 'string')
    return (
      body?.ok === false &&
      typeof body?.message === 'string' &&
      body.message.startsWith('Access not allowed')
    )
  }

  private static apiError(endpoint: string, data: any): Error {
    return new Error(
      `Admin API ${endpoint}: ${
        typeof data.error === 'string'
          ? data.error
          : data.error.message || JSON.stringify(data.error)
      }`
    )
  }

  // Sends one request to /Invoke/auth/<endpoint>, signing in lazily and once
  // more when the token is rejected (expired). Returns the raw response.
  private async send(
    endpoint: string,
    init: { method: 'GET' | 'POST'; body?: string | FormData; json?: boolean }
  ): Promise<{ res: Response; data: any; raw: Buffer | null }> {
    if (!this.enabled)
      throw new Error(
        `Admin API not configured. Set ${ENV_PREFIX}ADMIN_USERNAME and ${ENV_PREFIX}ADMIN_PASSWORD.`
      )
    for (let attempt = 0; attempt < 2; attempt++) {
      if (!this.token) this.token = await this.signin()
      const headers: Record<string, string> = { 'x-access-token': this.token }
      if (init.json) headers['Content-Type'] = 'application/json'
      let res: Response
      try {
        res = await fetch(this.baseUrl + API_PATH + endpoint, {
          method: init.method,
          headers,
          ...(init.body !== undefined ? { body: init.body } : {}),
        })
      } catch (e) {
        throw new Error(
          `Admin API unreachable at ${this.baseUrl} (is server_realtime_auth running?): ${
            e instanceof Error ? e.message : String(e)
          }`
        )
      }
      const raw = Buffer.from(await res.arrayBuffer())
      let data: any = {}
      try {
        data = JSON.parse(raw.toString('utf8'))
      } catch {
        data = {}
      }
      if (AdminApiClient.isAuthFailure(res.status, data)) {
        this.token = null
        if (attempt === 0) continue
        throw new Error(
          `Admin API denied access for user '${this.username}': ${
            data?.message || res.status
          } (the user needs a role with isAdmin rights)`
        )
      }
      if (data && typeof data === 'object' && !Array.isArray(data) && data.error)
        throw AdminApiClient.apiError(endpoint, data)
      if (!res.ok) throw new Error(`Admin API ${endpoint}: HTTP ${res.status}`)
      Log.log(`Admin API ${endpoint} OK`, Log.levelDetailed)
      return { res, data, raw }
    }
    throw new Error(`Admin API ${endpoint}: access denied`)
  }

  // POSTs JSON to an /Invoke/auth/<endpoint> route and returns the parsed body.
  // Throws when the API reports an error ({ error: ... }).
  async call(endpoint: string, body: any = {}): Promise<any> {
    return (
      await this.send(endpoint, {
        method: 'POST',
        body: JSON.stringify(body),
        json: true,
      })
    ).data
  }

  // GETs an /Invoke/auth/<endpoint> route with query parameters (undefined
  // values are left out) and returns the parsed JSON body.
  async get(
    endpoint: string,
    query: Record<string, string | number | undefined> = {}
  ): Promise<any> {
    return (await this.send(endpoint + AdminApiClient.qs(query), { method: 'GET' }))
      .data
  }

  // GETs a file (attachment) and returns its bytes.
  async download(
    endpoint: string,
    query: Record<string, string | number | undefined> = {}
  ): Promise<Buffer> {
    const { raw } = await this.send(endpoint + AdminApiClient.qs(query), {
      method: 'GET',
    })
    return raw || Buffer.alloc(0)
  }

  // POSTs JSON to a route that answers with a file (attachment) and returns
  // its bytes.
  async postDownload(endpoint: string, body: any = {}): Promise<Buffer> {
    const { raw } = await this.send(endpoint, {
      method: 'POST',
      body: JSON.stringify(body),
      json: true,
    })
    return raw || Buffer.alloc(0)
  }

  // POSTs a multipart form (file uploads) and returns the parsed JSON body.
  async postForm(endpoint: string, form: FormData): Promise<any> {
    return (await this.send(endpoint, { method: 'POST', body: form })).data
  }

  // Calls the realtime Web API (/Invoke/, OPC UA-like JSON services used by the
  // HMI: acknowledgements, point properties, ...) with the same sign-in. The
  // user's role rights (ackAlarms, enterAnnotations, ...) apply there.
  // Returns the response Body; throws on a service fault or bad ServiceResult.
  async invoke(serviceId: number, body: Record<string, unknown>): Promise<any> {
    if (!this.enabled)
      throw new Error(
        `Admin API not configured. Set ${ENV_PREFIX}ADMIN_USERNAME and ${ENV_PREFIX}ADMIN_PASSWORD.`
      )
    for (let attempt = 0; attempt < 2; attempt++) {
      if (!this.token) this.token = await this.signin()
      const req = {
        ServiceId: serviceId,
        Body: {
          RequestHeader: {
            Timestamp: new Date().toISOString(),
            RequestHandle: Math.floor(Math.random() * 1e9),
            TimeoutHint: 5000,
            ReturnDiagnostics: 2,
            AuthenticationToken: null,
          },
          ...body,
        },
      }
      let res: Response
      try {
        res = await fetch(this.baseUrl + '/Invoke/', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', 'x-access-token': this.token },
          body: JSON.stringify(req),
        })
      } catch (e) {
        throw new Error(
          `Realtime API unreachable at ${this.baseUrl} (is server_realtime_auth running?): ${
            e instanceof Error ? e.message : String(e)
          }`
        )
      }
      const data: any = await res.json().catch(() => ({}))
      const header = data?.Body?.ResponseHeader || {}
      const text = (header.StringTable || []).join(' ')
      if (/absent or invalid access token/i.test(text)) {
        this.token = null
        if (attempt === 0) continue
      }
      if (!res.ok) throw new Error(`Realtime API: HTTP ${res.status}`)
      // OPC status severity is the top two bits: Good (0) and Uncertain (1)
      // are success (e.g. GoodNoData), Bad (2, 3) a failure
      if (header.ServiceResult && Number(header.ServiceResult) >>> 30 >= 2)
        throw new Error(`Realtime API: ${text || 'status 0x' + Number(header.ServiceResult).toString(16)}`)
      return data.Body || {}
    }
    throw new Error('Realtime API: access denied')
  }

  private static qs(query: Record<string, string | number | undefined>) {
    const p = new URLSearchParams()
    for (const [k, v] of Object.entries(query))
      if (v !== undefined && v !== '') p.set(k, String(v))
    const s = p.toString()
    return s ? '?' + s : ''
  }
}
