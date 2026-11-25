'use strict'

const assert = require('node:assert/strict')
const { randomBytes } = require('node:crypto')
const test = require('node:test')
const { deflateSync, inflateSync } = require('node:zlib')
const { signPacket } = require('../mongofw/udp-auth')
const { verifyPacket } = require('./udp-auth')

test('authenticates the sender envelope and assigns a fresh nonce per datagram', () => {
  const key = randomBytes(32)
  const payload = deflateSync('[]')
  const first = verifyPacket(signPacket(payload, key), key)
  const second = verifyPacket(signPacket(payload, key), key)
  assert.equal(inflateSync(first.payload).toString(), '[]')
  assert.notEqual(first.nonce, second.nonce)
})

test('rejects unsigned, altered, truncated, and wrong-key datagrams', () => {
  const key = randomBytes(32)
  const payload = deflateSync('[]')
  const signed = signPacket(payload, key)
  assert.equal(verifyPacket(payload, key), undefined)
  assert.equal(verifyPacket(signed, randomBytes(32)), undefined)
  assert.equal(verifyPacket(signed.subarray(0, signed.length - 1), key), undefined)
  for (const offset of [0, 11, 12, 28, signed.length - 1]) {
    const changed = Buffer.from(signed)
    changed[offset] ^= 1
    assert.equal(verifyPacket(changed, key), undefined)
  }
})

test('enforces both ends of the authenticated timestamp window', (t) => {
  const key = randomBytes(32)
  const clock = t.mock.method(Date, 'now', () => 100_000)
  const signed = signPacket(deflateSync('[]'), key)
  clock.mock.mockImplementation(() => 95_000)
  assert(verifyPacket(signed, key))
  clock.mock.mockImplementation(() => 94_999)
  assert.equal(verifyPacket(signed, key), undefined)
  clock.mock.mockImplementation(() => 159_999)
  assert(verifyPacket(signed, key))
  clock.mock.mockImplementation(() => 160_000)
  assert.equal(verifyPacket(signed, key), undefined)
})
