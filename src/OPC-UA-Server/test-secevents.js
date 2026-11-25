/*
 * Self-test for the security events and client identification of the
 * OPC-UA server driver (secevents.js): in-process OPC UA server and client,
 * a fake secEvents collection; no database.
 *
 * Run: node test-secevents.js   (expect "ALL PASS")
 */

'use strict'

const net = require('net')
const {
  OPCUAServer,
  OPCUAClient,
  DataType,
  StatusCodes,
  AttributeIds,
  UserTokenType,
  MessageBuilder,
  Variant,
} = require('node-opcua')
const sec = require('./secevents')

let failures = 0
function check(name, cond, extra = '') {
  console.log((cond ? 'PASS ' : 'FAIL ') + name + (cond ? '' : ' ' + extra))
  if (!cond) failures++
}
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

const PORT = 48497

async function newServer(connection, events) {
  const writer = new sec.SecEventWriter()
  writer.capture = (e) => events.push(e)
  const server = new OPCUAServer({
    port: PORT,
    allowAnonymous: true,
    userManager: { isValidUser: (u, p) => u === 'op' && p === 'right' },
    serverCapabilities: { maxSessions: 20 },
  })
  const cs = new sec.ConnSec(connection, writer, { protocolDriver: 'OPC-UA_SERVER', instance: 1, nodeName: 'test' })
  cs.attach(server)
  await server.initialize()
  const ns = server.engine.addressSpace.registerNamespace('urn:jsonscada:sectest')
  const folder = ns.addFolder('ObjectsFolder', { browseName: 'T' })
  ns.addVariable({
    componentOf: folder,
    nodeId: 's=Data',
    browseName: 'Data',
    dataType: DataType.Double,
    accessLevel: 'CurrentRead', // as index.js: data variables are read-only
    value: { get: () => new Variant({ dataType: DataType.Double, value: 1 }) },
  })
  const cmd = ns.addVariable({
    componentOf: folder,
    nodeId: 's=Cmd',
    browseName: 'Cmd',
    dataType: DataType.Boolean,
    accessLevel: 'CurrentRead | CurrentWrite',
    userAccessLevel: 'CurrentRead | CurrentWrite',
    value: {
      get: () => new Variant({ dataType: DataType.Boolean, value: false }),
      // as index.js: the writer's session context comes after the callback
      set: (variant, callback, context) => {
        if (cs.rejectNext) {
          // as sendCommand for a blocked command
          cs.rejectNext = false
          cs.commandOutcome(context, {
            tag: 'CMD1',
            nodeId: cmd.nodeId.toString(),
            status: 'BadNotWritable',
            kind: 'cmd.rejected',
            detail: 'command blocked (commandBlocked)',
          })
          callback(null, StatusCodes.BadNotWritable)
          return
        }
        cs.commandOutcome(context, {
          tag: 'CMD1',
          nodeId: cmd.nodeId.toString(),
          status: 'Good',
          value: variant.value ? 1 : 0,
          unnecessary: variant.value === true,
        })
        callback(null, StatusCodes.Good)
      },
    },
  })
  cs.commandNodes.add(cmd.nodeId.toString())
  await server.start()
  return { server, cs, writer, nsIndex: ns.index }
}

async function connect(opts = {}) {
  const client = OPCUAClient.create({ endpointMustExist: false, connectionStrategy: { maxRetry: 0 }, ...opts })
  await client.connect(`opc.tcp://127.0.0.1:${PORT}`)
  return client
}

const kinds = (events, k) => events.filter((e) => e.kind === k)

;(async () => {
  // ---- helpers
  check('ipOf strips the IPv4-mapped prefix', sec.ipOf('::ffff:10.0.0.5') === '10.0.0.5')
  check(
    '/proc/net/arp parsing',
    sec.parseProcArp(
      '10.0.0.5',
      'IP address HW type Flags HW address Mask Device\n10.0.0.5 0x1 0x2 00:1A:2b:3c:4d:5e * eth0\n'
    ) === '00:1a:2b:3c:4d:5e'
  )
  check(
    'incomplete ARP entry is unknown',
    sec.parseProcArp('10.0.0.6', '10.0.0.6 0x1 0x0 00:00:00:00:00:00 * eth0') === ''
  )
  check(
    'arp -a parsing (Windows)',
    sec.parseArpOutput('10.0.0.7', '  10.0.0.7          0-1a-2b-3c-4d-5e     dynamic') === '00:1a:2b:3c:4d:5e'
  )
  const th = new sec.Throttle(1000)
  check('throttle lets the first through', th.allow('k', 0) === 1)
  check('throttle holds a repeat', th.allow('k', 10) === 0)
  check('throttle counts the held ones', th.allow('k', 1500) === 2)
  const rl = new sec.RateLimit(2, 1000)
  rl.hit('a', 0)
  rl.hit('a', 1)
  check('rate limit fires once past the limit', rl.hit('a', 2) === 3 && rl.hit('a', 3) === 0)
  {
    // a full key table: new keys share the overflow slot, reporting goes on
    const full = new sec.Throttle(1000)
    for (let i = 0; i < 4096; i++) full.allow('k' + i, 0)
    let ok = true
    try {
      ok = full.allow('new1', 10) === 1 && full.allow('new2', 20) === 0 && full.allow('new3', 1500) === 1 && full.state.size === 1
    } catch (e) {
      ok = false
    }
    check('throttle at capacity keeps reporting', ok)
    const fullRl = new sec.RateLimit(2, 1000)
    for (let i = 0; i < 4096; i++) fullRl.hit('k' + i, 0)
    const hits = [fullRl.hit('x', 10), fullRl.hit('y', 11), fullRl.hit('z', 12)]
    check('rate limit at capacity keeps detecting', hits[2] === 3 && fullRl.state.size === 4096, JSON.stringify(hits))
  }
  const ev = sec.completeEvent({ kind: 'cmd.rejected', tag: '', fields: {} })
  check(
    'event defaults',
    ev.severity === 'warning' && ev.severityLevel === 2 && ev.category === 'command' && ev.count === 1 &&
      ev.source === 'driver' && !('tag' in ev) && !('fields' in ev)
  )
  check('duplicate-only insert errors', sec.onlyDuplicates({ writeErrors: [{ code: 11000 }] }))
  check('other insert errors', !sec.onlyDuplicates({ writeErrors: [{ code: 11000 }, { code: 2 }] }))

  // ---- writer: ids, retries, duplicates
  {
    const w = new sec.SecEventWriter()
    const stored = []
    let fail = true
    w.setCollection({
      insertMany: async (docs) => {
        if (fail) {
          fail = false
          throw new Error('down')
        }
        stored.push(...docs)
      },
    })
    w.emit(sec.completeEvent({ kind: 'conn.refused' }))
    await w.flush()
    check('a failed insert keeps the events', stored.length === 0 && w.pending.length === 1)
    const id = w.pending[0]._id
    await w.flush()
    check('the retry stores them once, with the same _id', stored.length === 1 && stored[0]._id === id)
    for (let i = 0; i < 4097; i++) w.emit({ kind: 'x' })
    await w.flush()
    check('a full queue is counted', stored.some((e) => e.kind === 'meta.events_dropped' && e.count === 1))
    await w.close()
  }
  {
    // shutdown during a flush: close waits for it and stores what came after
    const w = new sec.SecEventWriter()
    const stored = []
    w.setCollection({
      insertMany: async (docs) => {
        await sleep(100)
        stored.push(...docs)
      },
    })
    w.emit(sec.completeEvent({ kind: 'conn.refused' }))
    const inFlight = w.flush()
    await sleep(10)
    w.emit(sec.completeEvent({ kind: 'conn.unexpected' }))
    await w.close()
    await inFlight
    check('close stores the events of a flush in flight and the later ones', stored.length === 2, String(stored.length))
  }

  // ---- in-process server
  const events = []
  let { server, cs, writer, nsIndex } = await newServer(
    { protocolConnectionNumber: 71, name: 'SRV', commandsEnabled: true },
    events
  )
  const channels = []
  server.on('newChannel', (c) => channels.push(c))
  const client = await connect()
  const session = await client.createSession()
  await sleep(200)

  const ident = cs.statsDoc()
  const me = ident?.clients?.[0]
  check('the open session is in stats.peerIdentity.clients', ident?.clients?.length === 1, JSON.stringify(ident))
  check(
    'client application identified',
    !!me?.application?.applicationUri && !!me?.application?.productUri && me?.ip === '127.0.0.1',
    JSON.stringify(me)
  )
  check('session security recorded', me?.securityMode === 'None' && typeof me?.secureChannelId === 'number')
  check('user identity recorded', me?.user?.tokenType === 'anonymous', JSON.stringify(me?.user))
  check('known application', ident.known.length === 1 && ident.known[0].applicationUri === me.application.applicationUri)
  check('peer.identified on the first session', kinds(events, 'peer.identified').length === 1)
  const appUri = me.application.applicationUri

  // unknown node read
  const nope = `ns=${nsIndex};s=Nope`
  await session.read({ nodeId: nope, attributeId: AttributeIds.Value })
  await sleep(50)
  const un = kinds(events, 'access.unknown_node')
  check(
    'read of an unknown node',
    un.length === 1 && un[0].objectRef === nope && un[0].peerIp === '127.0.0.1' && un[0].fields.service === 'Read',
    JSON.stringify(un)
  )
  await session.read({ nodeId: nope, attributeId: AttributeIds.Value })
  await sleep(50)
  check('the same unknown node again is throttled', kinds(events, 'access.unknown_node').length === 1)

  // scan: many unknown nodes
  const many = []
  for (let i = 0; i < 25; i++) many.push({ nodeId: `ns=${nsIndex};s=Scan${i}`, attributeId: AttributeIds.Value })
  await session.read(many)
  await sleep(50)
  check('scan detected', kinds(events, 'access.scan').length === 1)

  // writes
  const data = `ns=${nsIndex};s=Data`
  const cmdId = `ns=${nsIndex};s=Cmd`
  await session.write({ nodeId: data, attributeId: AttributeIds.Value, value: { value: { dataType: DataType.Double, value: 5 } } })
  await sleep(50)
  check('write to a variable that is not a command', kinds(events, 'access.write_denied').length === 1)

  await session.write({ nodeId: `ns=${nsIndex};s=NoCmd`, attributeId: AttributeIds.Value, value: { value: { dataType: DataType.Double, value: 5 } } })
  await sleep(50)
  check('command to an unknown node', kinds(events, 'cmd.object_unknown').length === 1)

  await session.write({ nodeId: cmdId, attributeId: AttributeIds.Value, value: { value: { dataType: DataType.String, value: 'x' } } })
  await sleep(50)
  check('command of the wrong type', kinds(events, 'cmd.wrong_type').length === 1, JSON.stringify(events.slice(-2)))

  await session.write({ nodeId: data, attributeId: AttributeIds.DisplayName, value: { value: { dataType: DataType.LocalizedText, value: { text: 'z' } } } })
  await sleep(50)
  check('write of another attribute is a configuration change', kinds(events, 'ctrl.config_change').length === 1)

  const sc = await session.write({ nodeId: cmdId, attributeId: AttributeIds.Value, value: { value: { dataType: DataType.Boolean, value: true } } })
  await sleep(50)
  const unn = kinds(events, 'cmd.unnecessary')
  check('accepted command', sc.isGood())
  check(
    'unnecessary command, attributed to the client through the setter context',
    unn.length === 1 && unn[0].peerIp === '127.0.0.1' && unn[0].tag === 'CMD1' && unn[0].peer?.application?.applicationUri === appUri,
    JSON.stringify(unn)
  )
  check('an accepted command gives no write event of its own', kinds(events, 'cmd.rejected').length === 0)
  for (let i = 0; i < 11; i++)
    await session.write({ nodeId: cmdId, attributeId: AttributeIds.Value, value: { value: { dataType: DataType.Boolean, value: false } } })
  await sleep(50)
  check('command flooding', kinds(events, 'flood.commands').length === 1)

  cs.rejectNext = true
  await session.write({ nodeId: cmdId, attributeId: AttributeIds.Value, value: { value: { dataType: DataType.Boolean, value: true } } })
  await sleep(50)
  const rj = kinds(events, 'cmd.rejected')
  check('a command refused by the command path is reported once', rj.length === 1 && rj[0].tag === 'CMD1', JSON.stringify(rj))

  // a CreateSession for a path the server does not serve
  cs.checkEndpointUrl({ endpointUrl: `opc.tcp://10.9.9.9:${PORT}/Wrong/Path` }, { remoteAddress: '10.9.9.8', remotePort: 1 })
  cs.checkEndpointUrl({ endpointUrl: `opc.tcp://some-alias:${PORT}` }, { remoteAddress: '10.9.9.8', remotePort: 1 })
  check('endpoint path mismatch, host ignored', kinds(events, 'link.endpoint_mismatch').length === 1)

  // method calls
  await session.call({ objectId: 'ns=0;i=2253', methodId: `ns=${nsIndex};s=NoMethod`, inputArguments: [] })
  await sleep(50)
  check('call of an unknown method', kinds(events, 'cmd.object_unknown').length === 2)
  const before = events.length
  await session.call({ objectId: 'ns=0;i=2253', methodId: 'ns=0;i=11492', inputArguments: [{ dataType: DataType.UInt32, value: 1 }] })
  await sleep(50)
  check('GetMonitoredItems is routine', events.length === before, JSON.stringify(events.slice(before)))

  // the message builder hook: a sequence number error on this channel
  // the channel's real builder, with a forged channel id in the header: the
  // event names the channel the builder belongs to
  const builder = sec.builderOf(channels[0])
  check('the message builder is bound to its channel', !!builder)
  const realId = builder.channelId
  builder.channelId = 999999
  builder.emit('invalid_sequence_number', 7, 9)
  builder.channelId = realId
  const seq = kinds(events, 'seq.invalid')
  check(
    'sequence number error attributed by channel, not by header',
    seq.length === 1 && seq[0].peerIp === '127.0.0.1' && seq[0].fields.expectedSeq === 7 &&
      seq[0].fields.secureChannelId === channels[0].channelId && seq[0].fields.receivedChannelId === 999999,
    JSON.stringify(seq)
  )
  MessageBuilder.prototype.emit.call({ id: 'server999', channelId: me.secureChannelId }, 'invalid_sequence_number', 1, 2)
  check('a builder never bound to a channel is not attributed', kinds(events, 'seq.invalid').length === 1)

  // the allow-list refusal (index.js calls it)
  cs.unexpectedConnection({ remoteAddress: '::ffff:10.1.2.3', remotePort: 5555, channelId: 1 })
  const ux = kinds(events, 'conn.unexpected')
  check('unexpected connection', ux.length === 1 && ux[0].peerIp === '10.1.2.3' && ux[0].peerAddress === '10.1.2.3:5555')

  // a wrong password
  try {
    const c2 = await connect()
    try {
      await c2.createSession({ type: UserTokenType.UserName, userName: 'op', password: 'wrong' })
    } catch (e) {}
    await c2.disconnect()
  } catch (e) {}
  await sleep(100)
  const af = kinds(events, 'auth.failed')
  check('authentication failure', af.length === 1 && af[0].peer?.user?.userName === 'op' && !JSON.stringify(af).includes('wrong'), JSON.stringify(af))

  // garbage on the TCP listener
  await new Promise((resolve) => {
    const s = net.connect(PORT, '127.0.0.1', () => s.write(Buffer.from('GARBAGEGARBAGEGARBAGE')))
    s.on('close', resolve)
    s.on('error', resolve)
    setTimeout(() => {
      s.destroy()
      resolve()
    }, 3000)
  })
  await sleep(200)
  check('malformed traffic on the listener', kinds(events, 'conn.open_failed').length >= 1, JSON.stringify(events.slice(-3)))

  // every event is a complete secEvents document
  check(
    'events carry the connection and role',
    events.every((e) => e.protocolConnectionNumber === 71 && e.role === 'server' && e.protocol === 'opcua' && e.nodeName === 'test')
  )

  await session.close()
  await client.disconnect()
  await sleep(100)
  check('a closed session leaves stats.peerIdentity.clients', cs.statsDoc().clients.length === 0)

  // a session moved to another secure channel: its transport identity is the new one
  {
    const token = { toString: () => 'tok-1' }
    const desc = { applicationUri: 'urn:moved', productUri: 'urn:p', applicationName: { text: 'n' } }
    const s1 = { authenticationToken: token, clientDescription: desc, channel: { remoteAddress: '10.0.0.1', remotePort: 1, channelId: 500 } }
    cs.onCreateSession(s1)
    const first = cs.open.get('tok-1').connectedAt
    cs.onSessionActivated(s1, null)
    const s2 = { ...s1, channel: { remoteAddress: '10.0.0.2', remotePort: 2, channelId: 600 } }
    await sleep(5)
    cs.onSessionActivated(s2, null)
    const d = cs.open.get('tok-1')
    check(
      'reactivation on a new channel refreshes the address and channel',
      d.ip === '10.0.0.2' && d.secureChannelId === 600 && d.connectedAt === first && d.activatedAt instanceof Date,
      JSON.stringify(d)
    )
    cs.open.delete('tok-1')
  }

  // a command refused because the database failed
  cs.commandOutcome(null, { tag: 'T9', nodeId: 'ns=1;s=T9', status: 'BadInternalError', internal: true, error: 'db down' })
  const dbe = events.filter((e) => e.kind === 'cmd.rejected' && e.tag === 'T9')
  check('a database failure is a cmd.rejected event', dbe.length === 1 && dbe[0].error === 'db down', JSON.stringify(dbe))
  await server.shutdown()
  await writer.close()

  // ---- a changed identity across a restart (stats.peerIdentity seeds it)
  const events2 = []
  ;({ server, cs, writer } = await newServer(
    {
      protocolConnectionNumber: 72,
      name: 'SRV2',
      commandsEnabled: false,
      stats: { peerIdentity: { known: [{ applicationUri: appUri, productUri: 'urn:other:product', ip: '127.0.0.1' }] } },
    },
    events2
  ))
  const c3 = await connect()
  const s3 = await c3.createSession()
  await sleep(200)
  const ch = kinds(events2, 'peer.identity_changed')
  check('product change of a known client', ch.length === 1 && ch[0].previous === 'urn:other:product', JSON.stringify(ch))
  check('no peer.identified for a known client', kinds(events2, 'peer.identified').length === 0)
  check('a client using the IP instead of the host name is no endpoint mismatch', kinds(events2, 'link.endpoint_mismatch').length === 0)
  await s3.write({ nodeId: `ns=${nsIndex};s=Data`, attributeId: AttributeIds.Value, value: { value: { dataType: DataType.Double, value: 5 } } })
  await sleep(50)
  check('write while commands are disabled', kinds(events2, 'cmd.disabled').length === 1, JSON.stringify(events2.slice(-1)))
  await s3.close()
  await c3.disconnect()
  await server.shutdown()
  await writer.close()

  // ---- identification off: events, no identity
  const events3 = []
  ;({ server, cs, writer } = await newServer(
    { protocolConnectionNumber: 73, name: 'SRV3', commandsEnabled: true, peerIdentification: false },
    events3
  ))
  const c4 = await connect()
  const s4 = await c4.createSession()
  await s4.read({ nodeId: nope, attributeId: AttributeIds.Value })
  await sleep(200)
  check('identification off: no stats.peerIdentity', cs.statsDoc() === null)
  check('identification off: no peer events', events3.every((e) => !e.kind.startsWith('peer.')))
  check('identification off: security events still reported', kinds(events3, 'access.unknown_node').length === 1)
  // refused writes count for flooding too
  for (let i = 0; i < 11; i++)
    await s4.write({ nodeId: `ns=${nsIndex};s=Data`, attributeId: AttributeIds.Value, value: { value: { dataType: DataType.Double, value: i } } })
  await sleep(50)
  check('refused command attempts are counted for flooding', kinds(events3, 'flood.commands').length === 1)
  await s4.close()
  await c4.disconnect()
  await server.shutdown()
  await writer.close()

  console.log(failures === 0 ? 'ALL PASS' : `${failures} FAILED`)
  process.exit(failures === 0 ? 0 : 1)
})().catch((e) => {
  console.log('FAIL exception ' + (e?.stack || e))
  process.exit(1)
})
