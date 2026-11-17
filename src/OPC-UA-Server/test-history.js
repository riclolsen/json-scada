/*
 * Self-test for the OPC-UA server driver: HistoryRead (raw, reverse, cap
 * truncation, continuation), integer conversion and asynchronous write status.
 * In-process OPC UA server + client with a mock history backend; no database.
 *
 * Run: node test-history.js   (expect "ALL PASS")
 */

'use strict'

const {
  OPCUAServer,
  OPCUAClient,
  Variant,
  DataType,
  StatusCodes,
  AttributeIds,
  TimestampsToReturn,
  ReadRawModifiedDetails,
  HistoryReadRequest,
} = require('node-opcua')
const { convertHistValue, installTagHistory } = require('./historian')

let failures = 0
function check(name, cond, extra = '') {
  console.log((cond ? 'PASS ' : 'FAIL ') + name + (cond ? '' : ' ' + extra))
  if (!cond) failures++
}

const PORT = 48499
const CAP = 20
const TOTAL = 50 // rows in the mock store, more than the cap

;(async () => {
  // ---- integer conversion must produce variants node-opcua accepts
  const cases = [
    ['int16', -1, -1],
    ['int16', 40000, 32767],
    ['sbyte', -1, -1],
    ['sbyte', 300, 127],
    ['uint16', -5, 0],
    ['int32', -5, -5],
    ['uint32', 3000000000, 3000000000],
    ['uint32', -1, 0],
    ['byte', 200, 200],
    ['int64', -7, -7],
    ['uint64', 5000000000, 5000000000],
  ]
  for (const [asdu, input, expected] of cases) {
    const r = convertHistValue('analog', asdu, input)
    let ok = r.value === expected
    try {
      new Variant({ dataType: r.dataType, value: r.value })
    } catch (e) {
      ok = false
    }
    check(`convert ${asdu}(${input}) -> ${expected}`, ok, 'got ' + r.value)
  }
  check(
    'convert NaN analog stays finite for integer types',
    convertHistValue('analog', 'int16', 'abc').value === 0
  )

  // ---- server
  const server = new OPCUAServer({
    port: PORT,
    serverCapabilities: { maxSessions: 100 },
  })
  await server.initialize()
  const as = server.engine.addressSpace
  const ns = as.registerNamespace('urn:jsonscada:test')
  const folder = ns.addFolder('ObjectsFolder', { browseName: 'T' })

  // mock backend honouring limit/reverse like the real ones
  const t0 = Date.UTC(2026, 0, 1)
  const store = []
  for (let i = 0; i < TOTAL; i++)
    store.push({
      value: i,
      invalid: false,
      timeTag: new Date(t0 + i * 1000),
      timeTagAtSource: null,
    })
  const calls = []
  const backend = {
    readRange(tag, range, cb) {
      calls.push(range)
      let rows = store.filter(
        (r) =>
          (!range.startTime || r.timeTag >= range.startTime) &&
          (!range.endTime || r.timeTag <= range.endTime)
      )
      if (range.reverse) rows = rows.slice().reverse()
      cb(null, rows.slice(0, range.limit))
    },
  }
  const hv = ns.addVariable({
    componentOf: folder,
    browseName: 'h',
    dataType: 'Double',
  })
  installTagHistory(
    ns,
    as,
    hv,
    { tag: 'h', type: 'analog' },
    backend,
    { historyMaxReturnDataValues: CAP },
    { log() {} }
  )

  // async command-style setter: the client must see the real status
  let verdict = StatusCodes.BadNotWritable
  const cmd = ns.addVariable({
    componentOf: folder,
    browseName: 'cmd',
    dataType: 'Double',
    minimumSamplingInterval: -1,
    value: {
      get: () => new Variant({ dataType: DataType.Double, value: 0 }),
      set: (variant, callback) =>
        setTimeout(() => callback(null, verdict), 5),
    },
  })

  await server.start()
  check(
    'maxSessions from serverCapabilities is applied',
    server.engine.serverCapabilities.maxSessions === 100
  )

  const client = OPCUAClient.create({ endpointMustExist: false })
  await client.connect('opc.tcp://localhost:' + PORT)
  const session = await client.createSession()

  // ---- write status
  const wr = () =>
    session.write({
      nodeId: cmd.nodeId,
      attributeId: AttributeIds.Value,
      value: { value: { dataType: DataType.Double, value: 1 } },
    })
  const bad = await wr()
  check('write returns the real (bad) status', bad.value === verdict.value, bad.toString())
  verdict = StatusCodes.Good
  check('write returns Good when accepted', (await wr()).isGood())

  // ---- history
  async function hr(start, end, n, cp) {
    calls.length = 0
    const req = new HistoryReadRequest({
      nodesToRead: [{ nodeId: hv.nodeId, continuationPoint: cp }],
      historyReadDetails: new ReadRawModifiedDetails({
        startTime: start,
        endTime: end,
        numValuesPerNode: n,
        returnBounds: false,
        isReadModified: false,
      }),
      timestampsToReturn: TimestampsToReturn.Both,
    })
    const resp = await session.historyRead(req)
    const r = resp.results[0]
    return {
      status: r.statusCode,
      values: (r.historyData?.dataValues || []).map((d) => d.value.value),
      cp: r.continuationPoint,
    }
  }
  const from = new Date(t0)
  const to = new Date(t0 + TOTAL * 1000)

  let r = await hr(from, to, 0)
  check('forward, over cap: capped at the limit', r.values.length === CAP)
  check('forward, over cap: reports GoodMoreData', r.status === StatusCodes.GoodMoreData, r.status.toString())
  check('forward, over cap: oldest values first', r.values[0] === 0)

  r = await hr(to, from, 0)
  check('reverse, over cap: newest chunk returned', r.values[0] === TOTAL - 1, 'first=' + r.values[0])
  check('reverse, over cap: descending order', r.values[1] === TOTAL - 2)
  check('reverse, over cap: reports GoodMoreData', r.status === StatusCodes.GoodMoreData)

  r = await hr(from, new Date(t0 + 9500), 0)
  check('within cap: Good and complete', r.status.isGood() && r.status !== StatusCodes.GoodMoreData && r.values.length === 10)

  r = await hr(from, to, 5)
  check('numValuesPerNode=5 gives a continuation point', r.values.length === 5 && !!r.cp)
  const r2 = await hr(from, to, 5, r.cp)
  check('continuation returns the next page', r2.values[0] === 5, 'first=' + r2.values[0])

  await session.close()
  await client.disconnect()
  await server.shutdown()

  console.log(failures === 0 ? 'ALL PASS' : failures + ' FAILURE(S)')
  process.exit(failures === 0 ? 0 : 1)
})().catch((e) => {
  console.log('ERROR', e)
  process.exit(1)
})
