/*
 * {json:scada} - Copyright (c) 2020-2024 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Outbound webhook pusher: batches notification envelopes and POSTs them to
 * one or more n8n webhook URLs, with per-URL bounded queue and exponential
 * backoff. Uses Node's built-in http/https to avoid extra dependencies.
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

'use strict'

const http = require('http')
const https = require('https')
const fs = require('fs')
const { URL } = require('url')
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')

class WebhookPusher {
  // urls: array of endpoint URL strings
  // opts: { bearerToken, timeoutMs, maxQueueSize, retryMaxMs, caFilePath, rejectUnauthorized }
  constructor(urls, opts = {}) {
    this.opts = opts
    this.timeoutMs = opts.timeoutMs || 10000
    this.maxQueueSize = opts.maxQueueSize || AppDefs.MAX_QUEUE_SIZE
    this.retryMaxMs = opts.retryMaxMs || AppDefs.RETRY_MAX_MS
    this.ca = null
    if (opts.caFilePath && opts.caFilePath.trim() !== '') {
      try {
        this.ca = fs.readFileSync(opts.caFilePath)
      } catch (e) {
        Log.log('Webhook - Could not read CA file: ' + e.message)
      }
    }
    // one independent channel per URL
    this.channels = (urls || [])
      .filter((u) => typeof u === 'string' && u.trim() !== '')
      .map((u) => ({
        url: u.trim(),
        queue: [],
        sending: false,
        backoffMs: 0,
        retryTimer: null,
      }))
    this.stats = {
      sent: 0,
      dropped: 0,
      errors: 0,
      lastError: '',
      // outcome of the last POST: true ok, false failed, null none yet
      lastOk: null,
    }
  }

  hasTargets() {
    return this.channels.length > 0
  }

  queueSizes() {
    const s = {}
    for (const ch of this.channels) s[ch.url] = ch.queue.length
    return s
  }

  // enqueue an envelope object for delivery to all channels
  push(envelope) {
    const body = JSON.stringify(envelope)
    for (const ch of this.channels) {
      if (ch.queue.length >= this.maxQueueSize) {
        // drop oldest
        ch.queue.shift()
        this.stats.dropped++
      }
      ch.queue.push(body)
      this._drain(ch)
    }
  }

  _drain(ch) {
    if (ch.sending || ch.retryTimer) return
    if (ch.queue.length === 0) return
    ch.sending = true
    const body = ch.queue[0]
    this._post(ch.url, body)
      .then(() => {
        ch.queue.shift()
        ch.sending = false
        ch.backoffMs = 0
        this.stats.sent++
        this.stats.lastOk = true
        // continue draining
        setImmediate(() => this._drain(ch))
      })
      .catch((err) => {
        ch.sending = false
        this.stats.errors++
        this.stats.lastOk = false
        this.stats.lastError = ch.url + ': ' + (err.message || String(err))
        Log.log('Webhook - POST failed ' + this.stats.lastError, Log.levelNormal)
        // exponential backoff, capped
        ch.backoffMs = Math.min(
          ch.backoffMs ? ch.backoffMs * 2 : AppDefs.RETRY_BASE_MS,
          this.retryMaxMs
        )
        ch.retryTimer = setTimeout(() => {
          ch.retryTimer = null
          this._drain(ch)
        }, ch.backoffMs)
      })
  }

  _post(urlStr, body) {
    return new Promise((resolve, reject) => {
      let u
      try {
        u = new URL(urlStr)
      } catch (e) {
        return reject(new Error('invalid URL'))
      }
      if (u.protocol !== 'http:' && u.protocol !== 'https:')
        return reject(new Error('unsupported scheme ' + u.protocol))

      const isHttps = u.protocol === 'https:'
      const lib = isHttps ? https : http
      const payload = Buffer.from(body, 'utf8')
      const headers = {
        'Content-Type': 'application/json',
        'Content-Length': payload.length,
      }
      if (this.opts.bearerToken && this.opts.bearerToken.trim() !== '')
        headers['Authorization'] = 'Bearer ' + this.opts.bearerToken.trim()

      const reqOpts = {
        method: 'POST',
        hostname: u.hostname,
        port: u.port || (isHttps ? 443 : 80),
        path: u.pathname + u.search,
        headers: headers,
        timeout: this.timeoutMs,
      }
      if (isHttps) {
        if (this.ca) reqOpts.ca = this.ca
        if (this.opts.rejectUnauthorized === false)
          reqOpts.rejectUnauthorized = false
      }

      const req = lib.request(reqOpts, (res) => {
        // drain response
        res.on('data', () => {})
        res.on('end', () => {
          if (res.statusCode >= 200 && res.statusCode < 300) resolve()
          else reject(new Error('HTTP ' + res.statusCode))
        })
      })
      req.on('error', (e) => reject(e))
      req.on('timeout', () => {
        req.destroy(new Error('timeout'))
      })
      // never follow redirects (SSRF hardening); n8n webhooks respond directly
      req.write(payload)
      req.end()
    })
  }

  stop() {
    for (const ch of this.channels) {
      if (ch.retryTimer) clearTimeout(ch.retryTimer)
      ch.retryTimer = null
      ch.queue = []
    }
  }
}

module.exports = WebhookPusher
