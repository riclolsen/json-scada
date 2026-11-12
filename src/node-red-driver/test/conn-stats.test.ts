/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Unit tests for the connection stats update (no MongoDB required).
 */

import { test } from 'node:test'
import assert from 'node:assert/strict'
import { updateConnectionStats } from '../src/conn-stats.ts'

type Doc = Record<string, unknown>

// A protocolConnections stand-in that rejects a dotted $set below a null
// stats with error 28, as MongoDB does.
function fakeColl(stats: unknown) {
  const doc: Doc = { protocolConnectionNumber: 9100, stats }
  const calls: Doc[] = []
  const coll = {
    async updateOne(filter: Doc, update: { $set: Doc }) {
      calls.push(filter)
      if ('stats' in filter && doc.stats !== null) return { matchedCount: 0 }
      for (const [k, v] of Object.entries(update.$set)) {
        if (k.startsWith('stats.')) {
          if (doc.stats === null)
            throw Object.assign(new Error("Cannot create field 'isConnected'"), { code: 28 })
          ;(doc.stats as Doc)[k.slice(6)] = v
        } else doc[k] = v
      }
      return { matchedCount: 1 }
    },
  }
  return { coll, doc, calls }
}

test('updateConnectionStats replaces a null stats and retries', async () => {
  const { coll, doc, calls } = fakeColl(null)
  await updateConnectionStats(coll as never, 9100, { isConnected: true, nodeName: 'n1' })
  assert.deepEqual(doc.stats, { isConnected: true, nodeName: 'n1' })
  assert.equal(calls.length, 3)
})

test('updateConnectionStats keeps the other stats fields', async () => {
  const { coll, doc, calls } = fakeColl({ sessions: 2 })
  await updateConnectionStats(coll as never, 9100, { isConnected: false })
  assert.deepEqual(doc.stats, { sessions: 2, isConnected: false })
  assert.equal(calls.length, 1)
})

test('updateConnectionStats rethrows other errors', async () => {
  const coll = {
    async updateOne() {
      throw Object.assign(new Error('boom'), { code: 11000 })
    },
  }
  await assert.rejects(updateConnectionStats(coll as never, 9100, { isConnected: true }), /boom/)
})
