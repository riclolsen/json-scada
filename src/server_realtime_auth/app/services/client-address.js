/*
 * Address of the client of a request.
 * {json:scada} - Copyright (c) 2020-2025 - Ricardo L. Olsen
 *
 * X-Real-IP / X-Forwarded-For are set by a reverse proxy, but anyone can send
 * them: they are only believed when the connection comes from a trusted proxy
 * (JS_TRUSTED_PROXIES, comma separated addresses; default: this host only).
 */

'use strict'

const TRUSTED_PROXIES = new Set(
  (process.env.JS_TRUSTED_PROXIES || '127.0.0.1,::1,::ffff:127.0.0.1')
    .split(',')
    .map((s) => s.trim())
    .filter((s) => s !== '')
)

function clientAddress(req) {
  const peer = req?.socket?.remoteAddress || ''
  if (!TRUSTED_PROXIES.has(peer)) return peer
  const realIp = req.headers?.['x-real-ip']
  if (typeof realIp === 'string' && realIp.trim() !== '') return realIp.trim()
  const fwd = req.headers?.['x-forwarded-for']
  if (typeof fwd === 'string' && fwd.trim() !== '') {
    // the right-most entry is the one added by the trusted proxy
    const parts = fwd.split(',').map((s) => s.trim()).filter((s) => s !== '')
    if (parts.length) return parts[parts.length - 1]
  }
  return peer
}

module.exports = { clientAddress, TRUSTED_PROXIES }
