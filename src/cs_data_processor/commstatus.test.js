'use strict'

// Mirrors ../cs_data_processor-go/commstatus_test.go; the expected values
// must stay identical in both.

const test = require('node:test')
const assert = require('node:assert')
const {
  CommStatus,
  Stall,
  sanitize,
  changed,
  tagDocument,
  STALL_LIMIT,
  CYCLE_MS,
} = require('./commstatus')

test('sanitize', () => {
  const cases = {
    KAW2: 'KAW2',
    ' RTU 1 / main': 'RTU_1___main',
    'IEC60870-5-104': 'IEC60870-5-104',
    'a.b': 'a_b',
    '': '_',
  }
  for (const [i, w] of Object.entries(cases)) assert.strictEqual(sanitize(i), w)
})

test('stall observe', () => {
  let now = new Date('2026-01-01T00:00:00Z')
  const s = new Stall()
  assert.strictEqual(s.observe(null, false, now)[0], false)
  let ka = new Date(now.getTime() - 2000)
  assert.deepStrictEqual(s.observe(ka, true, now), [true, 2])
  for (let i = 1; i <= STALL_LIMIT; i++) {
    now = new Date(now.getTime() + CYCLE_MS)
    assert.strictEqual(s.observe(ka, true, now)[0], true, 'reading ' + i)
  }
  now = new Date(now.getTime() + CYCLE_MS)
  assert.strictEqual(s.observe(ka, true, now)[0], false)
  ka = new Date(ka.getTime() - 3600e3) // skewed clock, still a change
  assert.deepStrictEqual(s.observe(ka, true, now), [true, 0])
})

test('stall stale at start', () => {
  const now = new Date('2026-01-01T00:00:00Z')
  const s = new Stall()
  const ka = new Date(now.getTime() - 600e3)
  assert.strictEqual(s.observe(ka, true, now)[0], false)
  assert.strictEqual(
    s.observe(ka, true, new Date(now.getTime() + CYCLE_MS))[0],
    false
  )
})

const byTag = (pts) => new Map(pts.map((p) => [p.spec.tag, p]))
const conn = (id, number, name, driver, extra) => ({
  id,
  number,
  name,
  driver,
  instNumber: 1,
  enabled: true,
  statsTime: null,
  hasStatsTime: false,
  statsNode: '',
  isConnected: null,
  numStats: {},
  ...extra,
})

test('build status', () => {
  const now = new Date('2026-01-01T00:00:00Z')
  const snap = {
    instances: [
      {
        id: 'i1',
        driver: 'DNP3',
        number: 1,
        enabled: true,
        activeNode: 'mainNode',
        version: '0.8.2',
        keepAlive: new Date(now.getTime() - 1000),
        hasKeepAlive: true,
      },
      {
        id: 'i2',
        driver: 'OPC-UA',
        number: 1,
        enabled: true,
        activeNode: '',
        version: '',
        keepAlive: new Date(now.getTime() - 3600e3),
        hasKeepAlive: true,
      },
    ],
    connections: [
      conn('c1', 10, 'RTU 1', 'DNP3', {
        isConnected: true,
        numStats: { numBytesRx: 5 },
      }),
      conn('c2', 11, 'RTU2', 'DNP3', { isConnected: false }),
      conn('c3', 12, 'PLC', 'OPC-UA'),
      conn('c4', 13, 'ALL_BAD', 'DNP3'),
      conn('c5', 14, 'NoDriver', 'MODBUS'),
    ],
    counts: new Map([
      [10, { total: 4, invalid: 1 }],
      [13, { total: 2, invalid: 2 }],
    ]),
  }
  const pts = byTag(new CommStatus(1).build(snap, true, now))
  const expect = (tag, v, invalid) => {
    const p = pts.get(tag)
    assert.ok(p, 'missing ' + tag)
    assert.deepStrictEqual([p.val.value, p.val.invalid], [v, invalid], tag)
  }
  expect('_System.Driver.DNP3.1.Alive', 1, false)
  expect('_System.Driver.OPC-UA.1.Alive', 0, false)
  expect('_System.Conn.RTU_1.Status', 1, false)
  expect('_System.Conn.RTU_1.Health', 75, false)
  expect('_System.Conn.RTU_1.LinkUp', 1, false)
  expect('_System.Conn.RTU_1.Stats.numBytesRx', 5, false)
  expect('_System.Conn.RTU2.Status', 0, false)
  expect('_System.Conn.PLC.Status', 0, false)
  expect('_System.Conn.PLC.LinkUp', 0, true)
  expect('_System.Conn.ALL_BAD.Status', 0, false)
  expect('_System.Conn.NoDriver.Status', 0, false)
  expect('_System.Conn.RTU2.Health', 0, true)
  expect('_System.Comm.DriversDown', 1, false)
  expect('_System.Comm.ConnectionsDown', 4, false)
  expect('_System.Comm.ConnectionsTotal', 5, false)

  const an = pts.get('_System.Driver.DNP3.1.ActiveNode')
  assert.strictEqual(an.val.str, 'mainNode')
  assert.strictEqual(an.spec.type, 'string')
  const st = pts.get('_System.Conn.RTU_1.Status')
  assert.strictEqual(st.spec.group3, 'RTU 1')
  assert.strictEqual(st.spec.alarm, true)

  const noMirror = byTag(new CommStatus(1).build(snap, false, now))
  assert.ok(!noMirror.has('_System.Conn.RTU_1.Stats.numBytesRx'))
})

test('build keeps counts between refreshes', () => {
  const now = new Date('2026-01-01T00:00:00Z')
  const cs = new CommStatus(1)
  const snap = {
    instances: [],
    connections: [conn('c1', 10, 'A', '')],
    counts: new Map([[10, { total: 10, invalid: 5 }]]),
  }
  cs.build(snap, false, now)
  snap.counts = null
  const p = byTag(cs.build(snap, false, new Date(now.getTime() + CYCLE_MS))).get(
    '_System.Conn.A.Health'
  )
  assert.deepStrictEqual([p.val.value, p.val.invalid], [50, false])
})

test('changed', () => {
  const a = { type: 'analog', deadband: 5 }
  assert.strictEqual(changed(a, { value: 1 }, { value: 4 }), false)
  assert.strictEqual(changed(a, { value: 1 }, { value: 6 }), true)
  assert.strictEqual(
    changed(a, { value: 1, invalid: false }, { value: 1, invalid: true }),
    true
  )
  const s = { type: 'string' }
  assert.strictEqual(changed(s, { str: 'a' }, { str: 'b' }), true)
  assert.strictEqual(changed(s, { str: 'a' }, { str: 'a' }), false)
})

test('tag document', () => {
  const doc = tagDocument(-1001, {
    kind: 'conn',
    ref: 'c1',
    point: 'Status',
    tag: '_System.Conn.A.Status',
    type: 'digital',
    group2: 'Connections',
    group3: 'A',
    text: 'Status',
    stateTrue: 'Online',
    stateFalse: 'Offline',
    unit: '',
    alarm: true,
    priority: 1,
    noHistory: false,
    deadband: 0,
  })
  assert.strictEqual(doc._id.valueOf(), -1001)
  assert.strictEqual(doc._id._bsontype, 'Double')
  assert.strictEqual(doc.tag, '_System.Conn.A.Status')
  assert.strictEqual(doc.description, '_System~Connections~A~Status')
  assert.strictEqual(doc.alarmDisabled, false)
  assert.strictEqual(doc.priority.valueOf(), 1)
  assert.strictEqual(doc.origin, 'system')
  assert.strictEqual(doc.timeTag, null)
  assert.deepStrictEqual(doc.commStatus, {
    kind: 'conn',
    ref: 'c1',
    point: 'Status',
  })
})
