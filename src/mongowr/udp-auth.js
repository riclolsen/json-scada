'use strict'

const { createHmac, timingSafeEqual } = require('node:crypto')

// JSR1 | Unix milliseconds (uint64 BE) | 16 random bytes | zlib data | HMAC-SHA-256
const magic = Buffer.from('JSR1')
const headerSize = 28
const macSize = 32
const maxAgeMs = 60_000
const maxFutureMs = 5_000

function isFresh(sentAt, now = Date.now()) {
  return sentAt > now - maxAgeMs && sentAt <= now + maxFutureMs
}

function verifyPacket(packet, key) {
  if (packet.length <= headerSize + macSize || packet.length > 65507 ||
      !packet.subarray(0, magic.length).equals(magic)) return

  const sentAt = Number(packet.readBigUInt64BE(4))
  if (!isFresh(sentAt)) return

  const macOffset = packet.length - macSize
  const authenticated = packet.subarray(0, macOffset)
  const expected = createHmac('sha256', key).update(authenticated).digest()
  if (!timingSafeEqual(expected, packet.subarray(macOffset))) return

  return {
    nonce: packet.toString('hex', 12, headerSize),
    sentAt,
    // Leave a second freshness window for clock differences with the MongoDB host.
    expiresAt: new Date(sentAt + 2 * maxAgeMs),
    payload: authenticated.subarray(headerSize),
  }
}

module.exports = { verifyPacket, isFresh }
