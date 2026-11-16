'use strict'

const test = require('node:test')
const assert = require('node:assert')
const Long = require('long')
const U = require('./sparkplug-utils')

test('topicMatchSub', () => {
  const cases = [
    ['a/b/c', 'a/b/c', true],
    ['a/b/c', 'a/+/c', true],
    ['a/b/c', 'a/+', false],
    ['a/b/c', 'a/#', true],
    ['a', 'a/#', true],
    ['a/b/c', '#', true],
    ['a/b/c', '+/+/+', true],
    ['a/b', '+/+/+', false],
    ['a/b.c', 'a/b?c', false],
    ['x/a/b', 'a/#', false],
  ]
  for (const [t, s, want] of cases)
    assert.strictEqual(U.topicMatchSub(t, s), want, t + ' vs ' + s)
})

test('jsonPathTopic', () => {
  assert.deepStrictEqual(U.jsonPathTopic(' a/b/$.x.y '), {
    topic: 'a/b',
    jsonPath: '$.x.y',
  })
  assert.deepStrictEqual(U.jsonPathTopic('a/b'), { topic: 'a/b', jsonPath: '' })
})

test('isSameOrChildPath and pathRegex do not match siblings', () => {
  assert.ok(U.isSameOrChildPath('spBv1.0/G/Node1', 'spBv1.0/G/Node1'))
  assert.ok(U.isSameOrChildPath('spBv1.0/G/Node1/Dev', 'spBv1.0/G/Node1'))
  assert.ok(!U.isSameOrChildPath('spBv1.0/G/Node10', 'spBv1.0/G/Node1'))
  const re = new RegExp(U.pathRegex('spBv1.0/G/Node1'))
  assert.ok(re.test('spBv1.0/G/Node1/metric'))
  assert.ok(!re.test('spBv1.0/G/Node10/metric'))
  assert.ok(new RegExp(U.pathRegex('a.b+c')).test('a.b+c/x'))
  assert.ok(!new RegExp(U.pathRegex('a.b+c')).test('aXb+c/x'))
})

test('parseCommandAddress', () => {
  assert.deepStrictEqual(U.parseCommandAddress('plain/topic'), {
    topic: 'plain/topic',
  })
  assert.deepStrictEqual(U.parseCommandAddress('spBv1.0/G/E/m'), {
    groupId: 'G',
    edgeNodeId: 'E',
    metricName: 'm',
  })
  assert.deepStrictEqual(
    U.parseCommandAddress('spBv1.0/G/E/Node Control/Rebirth'),
    { groupId: 'G', edgeNodeId: 'E', metricName: 'Node Control/Rebirth' }
  )
  assert.deepStrictEqual(U.parseCommandAddress('spBv1.0/G/E/D/a/b'), {
    groupId: 'G',
    edgeNodeId: 'E',
    deviceId: 'D',
    metricName: 'a/b',
  })
  assert.deepStrictEqual(U.parseCommandAddress('spBv1.0/G/NCMD/E/x/y'), {
    groupId: 'G',
    edgeNodeId: 'E',
    metricName: 'x/y',
  })
  assert.deepStrictEqual(U.parseCommandAddress('spBv1.0/G/DCMD/E/D/m'), {
    groupId: 'G',
    edgeNodeId: 'E',
    deviceId: 'D',
    metricName: 'm',
  })
  assert.strictEqual(U.parseCommandAddress('spBv1.0/G/E'), null)
  assert.strictEqual(U.parseCommandAddress('spBv1.0/G//m'), null)
  assert.strictEqual(U.parseCommandAddress(''), null)
})

test('sequence helpers', () => {
  assert.strictEqual(U.nextSeq(0), 1)
  assert.strictEqual(U.nextSeq(255), 0)
  assert.strictEqual(U.nextSeq(Long.fromNumber(7, true)), 8)
  assert.strictEqual(U.toNumber(Long.fromNumber(42, true)), 42)
  assert.strictEqual(U.toNumber('3'), 3)
})

test('topicStr', () => {
  assert.strictEqual(U.topicStr(' a/b+c#d '), 'a|b^c@d')
  assert.strictEqual(U.topicStr(null), '')
})
