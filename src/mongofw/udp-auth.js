'use strict'

const { createHmac, randomBytes } = require('node:crypto')

// JSR1 | Unix milliseconds (uint64 BE) | 16 random bytes | zlib data | HMAC-SHA-256
const magic = Buffer.from('JSR1')

function signPacket(payload, key) {
  const header = Buffer.alloc(28)
  magic.copy(header)
  header.writeBigUInt64BE(BigInt(Date.now()), 4)
  randomBytes(16).copy(header, 12)

  const mac = createHmac('sha256', key).update(header).update(payload).digest()
  return Buffer.concat([header, payload, mac])
}

module.exports = { signPacket }
