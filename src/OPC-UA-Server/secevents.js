/*
 * Security events and client identification for the OPC-UA server driver.
 *
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
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

// Writes the secEvents documents of the Go drivers' jssecevents package
// (contract in docs/schema.md, secEvents) and keeps each connection's
// stats.peerIdentity: the client sessions open now and the last identity of
// every client application seen. The events come from node-opcua's server
// events (channels, sessions, every request and its response) and from the
// command path of index.js (sendCommand).

'use strict'

const crypto = require('crypto')
const fs = require('fs')
const net = require('net')
const { execFile } = require('child_process')
const { ObjectId } = require('mongodb')
const {
  StatusCodes,
  AttributeIds,
  MessageSecurityMode,
  ApplicationType,
  MessageBuilder,
} = require('node-opcua')

const COLLECTION = 'secEvents'
const SEVERITIES = ['info', 'notice', 'warning', 'alert', 'critical']

// writer limits, as jssecevents
const QUEUE_SIZE = 4096
const BATCH_SIZE = 200
const FLUSH_EVERY_MS = 1000
const MAX_PENDING = 4 * QUEUE_SIZE

// detectors
const COMMAND_FLOOD_MAX = 10 // commands from one client address ...
const COMMAND_FLOOD_WINDOW_MS = 10000 // ... within this window
const SCAN_MAX = 20 // requests for unknown nodes from one address ...
const SCAN_WINDOW_MS = 60000 // ... within this window
const EVENT_EVERY_MS = 10000 // the same event (kind, address, object, status)
const UNKNOWN_NODE_EVERY_MS = 10 * 60000 // the same unknown node from one address
const CERT_REJECTED_EVERY_MS = 10 * 60000 // the same refused certificate
const MAX_KNOWN = 64 // client applications kept in stats.peerIdentity.known
const MAX_THROTTLE_KEYS = 4096
const SWEEP_EVERY_MS = 1000
const PENDING_REQUEST_MS = 120000 // a request with no response after this is forgotten
const MAC_TTL_MS = 30000

// kind -> [category, severity]
const KINDS = {
  'conn.unexpected': ['connection', 'alert'],
  'conn.refused': ['resource', 'warning'],
  'conn.open_failed': ['connection', 'warning'],
  'conn.security_rejected': ['connection', 'warning'],
  'tls.cert_rejected': ['tls', 'alert'],
  'tls.cert_changed': ['tls', 'alert'],
  'peer.identified': ['peer', 'info'],
  'peer.identity_changed': ['peer', 'alert'],
  'peer.mac_changed': ['peer', 'alert'],
  'link.endpoint_mismatch': ['connection', 'notice'],
  'auth.failed': ['auth', 'alert'],
  'auth.signature_invalid': ['auth', 'alert'],
  'access.unknown_node': ['access', 'warning'],
  'access.scan': ['access', 'alert'],
  'access.write_denied': ['access', 'warning'],
  'access.session_invalid': ['access', 'notice'],
  'cmd.disabled': ['command', 'warning'],
  'cmd.object_unknown': ['command', 'warning'],
  'cmd.wrong_type': ['command', 'warning'],
  'cmd.rejected': ['command', 'warning'],
  'cmd.unnecessary': ['command', 'notice'],
  'flood.commands': ['flood', 'alert'],
  'ctrl.restart': ['control', 'notice'],
  'ctrl.config_change': ['configuration', 'notice'],
  'ctrl.method_call': ['control', 'notice'],
  'file.transfer': ['file', 'notice'],
  'proto.malformed': ['protocol', 'warning'],
  'proto.security_check_failed': ['protocol', 'alert'],
  'proto.service_unsupported': ['protocol', 'warning'],
  'proto.bad_request': ['protocol', 'warning'],
  'proto.service_fault': ['protocol', 'notice'],
  'seq.invalid': ['sequence', 'alert'],
  'resource.max_sessions': ['resource', 'warning'],
  'meta.events_dropped': ['meta', 'warning'],
}

// ---------------------------------------------------------------- helpers

function severityLevel(s) {
  const i = SEVERITIES.indexOf(s)
  return i < 0 ? 0 : i
}

function statusName(sc) {
  if (!sc) return ''
  if (typeof sc.name === 'string') return sc.name
  return String(sc)
}

function isGood(sc) {
  return !sc || (typeof sc.isGood === 'function' ? sc.isGood() : sc.value === 0)
}

function is(sc, code) {
  return !!sc && !!code && sc.value === code.value
}

// the IP of an address, without the IPv4-mapped IPv6 prefix
function ipOf(addr) {
  if (!addr || typeof addr !== 'string') return ''
  let a = addr.trim()
  if (a.startsWith('[')) a = a.slice(1, a.indexOf(']'))
  if (a.toLowerCase().startsWith('::ffff:')) a = a.slice(7)
  return net.isIP(a) ? a : ''
}

function addrOf(ip, port) {
  if (!ip) return ''
  if (!port) return ip
  return ip.includes(':') ? `[${ip}]:${port}` : `${ip}:${port}`
}

// the first certificate of a DER chain (OPC UA sends the chain concatenated)
function firstDer(buf) {
  if (!Buffer.isBuffer(buf) || buf.length < 2 || buf[0] !== 0x30) return null
  let len = buf[1]
  let off = 2
  if (len & 0x80) {
    const n = len & 0x7f
    if (n < 1 || n > 4 || buf.length < 2 + n) return null
    len = 0
    for (let i = 0; i < n; i++) len = len * 256 + buf[2 + i]
    off = 2 + n
  }
  const end = off + len
  if (end > buf.length) return null
  return buf.subarray(0, end)
}

// subject, issuer, serial, SHA-256 fingerprint and expiry of a certificate;
// null without one. No key material.
function certDoc(der) {
  const c = firstDer(der)
  if (!c) return null
  const sha256 = crypto.createHash('sha256').update(c).digest('hex')
  try {
    const x = new crypto.X509Certificate(c)
    const d = {
      subject: x.subject.replace(/\n/g, ', '),
      issuer: x.issuer.replace(/\n/g, ', '),
      serial: x.serialNumber.toLowerCase(),
      sha256,
      notAfter: new Date(x.validTo),
    }
    if (x.subjectAltName) d.subjectAltName = x.subjectAltName
    return d
  } catch (e) {
    return { sha256 }
  }
}

function enumName(e, v) {
  if (v === undefined || v === null) return ''
  return typeof e?.[v] === 'string' ? e[v] : String(v)
}

function nodeIdText(n) {
  if (n === undefined || n === null) return ''
  try {
    return n.toString()
  } catch (e) {
    return ''
  }
}

function attributeName(id) {
  for (const [k, v] of Object.entries(AttributeIds)) if (v === id) return k
  return String(id)
}

// A change of the value of a key: an empty value is unknown, not a change.
class Tracker {
  constructor() {
    this.last = new Map()
  }
  seed(key, value) {
    if (value && !this.last.has(key)) this.last.set(key, value)
  }
  observe(key, value) {
    if (!value) return null
    const prev = this.last.get(key)
    this.last.set(key, value)
    if (this.last.size > MAX_THROTTLE_KEYS) this.last.delete(this.last.keys().next().value)
    return prev !== undefined && prev !== value ? prev : null
  }
}

// Lets one event of a key through per period; the ones held back are
// counted into the next one let through.
// While the key table is full (after dropping the expired keys), new keys
// share one overflow slot kept outside the table, so reporting goes on.
class Throttle {
  constructor(everyMs) {
    this.every = everyMs
    this.state = new Map()
    this.overflow = { at: -Infinity, held: 0 }
    this.swept = -Infinity
  }
  allow(key, now = Date.now()) {
    let s = this.state.get(key)
    if (!s) {
      if (this.state.size >= MAX_THROTTLE_KEYS && now - this.swept >= SWEEP_EVERY_MS) {
        this.swept = now // at most once per second: the work per call stays bounded
        for (const [k, v] of this.state) if (now - v.at >= this.every) this.state.delete(k)
      }
      if (this.state.size >= MAX_THROTTLE_KEYS) s = this.overflow
      else {
        s = { at: -Infinity, held: 0 }
        this.state.set(key, s)
      }
    }
    if (now - s.at < this.every) {
      s.held++
      return 0
    }
    const n = s.held + 1
    s.at = now
    s.held = 0
    return n
  }
}

// More than max hits of a key within the window: true (and the count) once
// per window.
class RateLimit {
  constructor(max, windowMs) {
    this.max = max
    this.window = windowMs
    this.state = new Map()
    this.overflow = { start: -Infinity, hits: 0, reported: false } // shared by new keys while the table is full
    this.swept = -Infinity
  }
  hit(key, now = Date.now()) {
    let s = this.state.get(key)
    if (!s) {
      if (this.state.size >= MAX_THROTTLE_KEYS && now - this.swept >= SWEEP_EVERY_MS) {
        this.swept = now
        for (const [k, v] of this.state) if (now - v.start >= this.window) this.state.delete(k)
      }
      if (this.state.size >= MAX_THROTTLE_KEYS) s = this.overflow
      else {
        s = { start: now, hits: 0, reported: false }
        this.state.set(key, s)
      }
    }
    if (now - s.start >= this.window) {
      s.start = now
      s.hits = 0
      s.reported = false
    }
    s.hits++
    if (s.hits > this.max && !s.reported) {
      s.reported = true
      return s.hits
    }
    return 0
  }
}

// ----------------------------------------------------------- MAC lookup

const MAC_RE = /\b([0-9a-f]{1,2}[:-]){5}[0-9a-f]{1,2}\b/i

function normalizeMac(s) {
  const parts = String(s || '').split(/[:-]/)
  if (parts.length !== 6) return ''
  const out = []
  for (const p of parts) {
    if (!/^[0-9a-f]{1,2}$/i.test(p)) return ''
    out.push(p.padStart(2, '0').toLowerCase())
  }
  const m = out.join(':')
  return m === '00:00:00:00:00:00' ? '' : m
}

function parseProcArp(ip, text) {
  for (const line of text.split('\n')) {
    // IP address  HW type  Flags  HW address  Mask  Device
    const f = line.trim().split(/\s+/)
    if (f.length < 4 || f[0] !== ip) continue
    if (f[2] === '0x0') return '' // incomplete entry
    return normalizeMac(f[3])
  }
  return ''
}

function parseArpOutput(ip, text) {
  for (const line of text.split('\n')) {
    if (!line.split(/[\s()]+/).includes(ip)) continue
    const m = line.match(MAC_RE)
    if (m) return normalizeMac(m[0])
  }
  return ''
}

const macCache = new Map()

// The MAC address the OS neighbour table holds for ip ('' when unknown; a
// client behind a router shows the router's).
async function lookupMac(ip) {
  if (!ip || ip === '::1' || ip.startsWith('127.')) return ''
  const now = Date.now()
  const c = macCache.get(ip)
  if (c && now - c.at < MAC_TTL_MS) return c.mac
  let mac = ''
  try {
    if (process.platform === 'linux') {
      mac = parseProcArp(ip, await fs.promises.readFile('/proc/net/arp', 'utf8'))
    } else if (['win32', 'darwin', 'freebsd'].includes(process.platform)) {
      const out = await new Promise((resolve) =>
        execFile('arp', ['-a', ip], { timeout: 2000, windowsHide: true }, (err, stdout) =>
          resolve(err ? '' : String(stdout))
        )
      )
      mac = parseArpOutput(ip, out)
    }
  } catch (e) {
    mac = ''
  }
  if (macCache.size > 4096) macCache.clear()
  macCache.set(ip, { mac, at: now })
  return mac
}

// --------------------------------------------------------------- writer

// Queues events and inserts them in batches. emit never blocks; past
// QUEUE_SIZE queued events the new ones are dropped and counted, and the
// count is stored as a meta.events_dropped event. Events not stored (MongoDB
// unreachable) are kept, up to MAX_PENDING, and retried.
class SecEventWriter {
  constructor(log) {
    this.log = log
    this.queue = []
    this.pending = []
    this.dropped = 0
    this.coll = null
    this.capture = null // tests: sees every event emitted
    this.timer = setInterval(() => this.flush(), FLUSH_EVERY_MS)
    if (this.timer.unref) this.timer.unref()
  }

  setCollection(coll) {
    this.coll = coll
  }

  emit(e) {
    if (this.capture) this.capture(e)
    if (this.queue.length >= QUEUE_SIZE) {
      this.dropped++
      return
    }
    this.queue.push(e)
    if (this.queue.length >= BATCH_SIZE) setImmediate(() => this.flush())
  }

  // one flush at a time: a call during a flush waits for that one
  flush() {
    if (!this.inFlight) this.inFlight = this.flushOnce().finally(() => (this.inFlight = null))
    return this.inFlight
  }

  async flushOnce() {
    try {
      if (this.dropped > 0) {
        const n = this.dropped
        this.dropped = 0
        this.queue.push(
          completeEvent({
            kind: 'meta.events_dropped',
            count: n,
            detail: 'security events lost: queue full',
          })
        )
      }
      while (this.queue.length > 0) {
        const e = this.queue.shift()
        if (!e.peerMac && e.peerIp) {
          const mac = await lookupMac(e.peerIp)
          if (mac) e.peerMac = mac
        }
        e._id = e._id || new ObjectId()
        if (this.pending.length >= MAX_PENDING) {
          this.pending.shift()
          this.dropped++
        }
        this.pending.push(e)
      }
      while (this.pending.length > 0 && this.coll) {
        const batch = this.pending.slice(0, BATCH_SIZE)
        try {
          await this.coll.insertMany(batch, { ordered: false })
        } catch (err) {
          if (!onlyDuplicates(err)) {
            this.log?.(`secEvents - insert: ${err?.message || err}`)
            return
          }
        }
        this.pending.splice(0, batch.length)
      }
    } catch (e) {
      this.log?.(`secEvents - flush: ${e?.message || e}`)
    }
  }

  // stores what is queued: waits for a flush in flight, then flushes the
  // events queued meanwhile (shutdown, before the MongoDB client closes)
  async close() {
    clearInterval(this.timer)
    await this.flush()
    if (this.queue.length > 0 || this.dropped > 0 || (this.pending.length > 0 && this.coll)) await this.flush()
  }
}

// an insert whose only failures are duplicate keys: events of a retried
// batch already stored by an earlier attempt
function onlyDuplicates(err) {
  const we = err?.writeErrors
  if (Array.isArray(we) && we.length > 0) return we.every((w) => (w.code ?? w.err?.code) === 11000)
  return err?.code === 11000
}

// fill the fields an event leaves unset; drop the empty ones
function completeEvent(e) {
  const [category, severity] = KINDS[e.kind] || ['', 'notice']
  const out = {
    timeTag: e.timeTag || new Date(),
    kind: e.kind,
    category: e.category || category,
    severity: e.severity || severity,
  }
  out.severityLevel = severityLevel(out.severity)
  out.action = e.action || 'observed'
  out.source = e.source || 'driver'
  for (const [k, v] of Object.entries(e)) {
    if (k in out) continue
    if (v === undefined || v === null || v === '') continue
    if (typeof v === 'object' && !(v instanceof Date) && Object.keys(v).length === 0) continue
    out[k] = v
  }
  out.count = e.count || 1
  return out
}

// ------------------------------------------------- per connection state

// Methods every client may call in the normal course of a session.
const ROUTINE_METHODS = new Set([
  'GetMonitoredItems',
  'ResendData',
  'ConditionRefresh',
  'ConditionRefresh2',
  'Acknowledge',
  'Confirm',
  'AddComment',
])

// The status codes of a ServiceFault and the event each gives.
function faultKind(sc, requestName) {
  const n = statusName(sc)
  if (n === 'BadServiceUnsupported') return 'proto.service_unsupported'
  if (
    ['BadIdentityTokenInvalid', 'BadIdentityTokenRejected', 'BadUserAccessDenied'].includes(n) &&
    requestName === 'ActivateSessionRequest'
  )
    return 'auth.failed'
  if (['BadApplicationSignatureInvalid', 'BadUserSignatureInvalid', 'BadNonceInvalid'].includes(n))
    return 'auth.signature_invalid'
  if (n.startsWith('BadCertificate') || n === 'BadSecurityChecksFailed') return 'tls.cert_rejected'
  if (n === 'BadTooManySessions') return 'resource.max_sessions'
  if (n === 'BadSecurityModeRejected' || n === 'BadSecurityPolicyRejected') return 'conn.security_rejected'
  if (
    ['BadSessionIdInvalid', 'BadSessionClosed', 'BadSessionNotActivated', 'BadSecureChannelIdInvalid'].includes(n)
  )
    return 'access.session_invalid'
  if (
    [
      'BadDecodingError',
      'BadEncodingError',
      'BadEncodingLimitsExceeded',
      'BadRequestTooLarge',
      'BadTooManyOperations',
      'BadNothingToDo',
      'BadTimestampsToReturnInvalid',
      'BadRequestHeaderInvalid',
      'BadProtocolVersionUnsupported',
    ].includes(n)
  )
    return 'proto.bad_request'
  return 'proto.service_fault'
}

// requests whose faults are part of a normal session (a Publish with no
// subscription, a Republish of a message gone, a timed out request)
const ROUTINE_FAULT_REQUESTS = new Set(['PublishRequest', 'RepublishRequest'])
const ROUTINE_FAULTS = new Set(['BadTimeout', 'BadShutdown', 'BadServerHalted', 'BadNoSubscription'])

// channels by secure channel id, for the errors node-opcua's message builder
// reports (it knows the channel id from the message header only)
const channelsById = new Map()

// The security state of one served connection.
class ConnSec {
  /**
   * @param {object} connection the protocolConnections document
   * @param {SecEventWriter} writer
   * @param {object} opts { protocolDriver, instance, nodeName, log, rtCollection }
   */
  constructor(connection, writer, opts = {}) {
    this.writer = writer
    this.log = opts.log || (() => {})
    this.rt = opts.rtCollection || null
    this.ident = connection.peerIdentification !== false
    this.commandsEnabled = connection.commandsEnabled === true
    this.origin = {
      protocolDriver: opts.protocolDriver || 'OPC-UA_SERVER',
      protocolDriverInstanceNumber: opts.instance || 1,
      protocolConnectionNumber: connection.protocolConnectionNumber,
      connectionName: connection.name,
      nodeName: opts.nodeName || '',
    }
    this.server = null
    this.commandNodes = new Set() // node ids (text) of the command variables
    this.open = new Map() // session (authentication token text) -> client document
    this.known = new Map() // applicationUri -> last identity
    this.trackers = new Tracker()
    this.throttle = new Throttle(EVENT_EVERY_MS)
    this.unknownThrottle = new Throttle(UNKNOWN_NODE_EVERY_MS)
    this.certThrottle = new Throttle(CERT_REJECTED_EVERY_MS)
    this.scan = new RateLimit(SCAN_MAX, SCAN_WINDOW_MS)
    this.flood = new RateLimit(COMMAND_FLOOD_MAX, COMMAND_FLOOD_WINDOW_MS)
    this.pendingRequests = new WeakMap() // channel -> Map(requestHandle -> request)
    this.handledWrites = new Map() // `${session}|${nodeId}` -> writes reported by the command path
    this.rejectedCertChannels = new WeakSet()
    this.dirty = true
    if (this.ident) this.seed(connection?.stats?.peerIdentity)
  }

  // the identities stored in stats.peerIdentity.known: a change across a
  // driver restart is seen too
  seed(stored) {
    const known = Array.isArray(stored?.known) ? stored.known : []
    for (const k of known) {
      const uri = typeof k?.applicationUri === 'string' ? k.applicationUri : ''
      if (!uri) continue
      const entry = { ...k }
      entry.lastSeen = k.lastSeen instanceof Date ? k.lastSeen : new Date(0)
      this.known.set(uri, entry)
      this.trackers.seed('product|' + uri, k.productUri)
      this.trackers.seed('name|' + uri, k.applicationName)
      this.trackers.seed('cert|' + uri, k.tlsSha256)
      if (k.ip) this.trackers.seed('mac|' + k.ip, k.mac)
    }
  }

  emit(e) {
    const ev = completeEvent({ protocol: 'opcua', role: 'server', ...this.origin, ...e })
    this.log(
      `Security event ${ev.kind} [${ev.severity}] ${ev.peerAddress || ''} ${ev.objectRef || ''} ${ev.detail || ''}`
    )
    this.writer?.emit(ev)
  }

  // emit through the general throttle: one event per key and period, with
  // the count of the ones held back
  emitThrottled(e, key, throttle = this.throttle) {
    const n = throttle.allow(key || [e.kind, e.peerIp, e.objectRef, e.fields?.status].join('|'))
    if (n === 0) return
    this.emit({ ...e, count: n })
  }

  // peer fields of an event from a channel and, when known, its session
  peerOfChannel(channel, session) {
    const ip = ipOf(channel?.remoteAddress)
    const p = {
      peerAddress: addrOf(ip, channel?.remotePort),
      peerIp: ip,
      sessionId: Number.isFinite(channel?.channelId) ? channel.channelId : undefined,
    }
    const peer = {}
    const sess = session || null
    if (sess?.clientDescription) peer.application = appDoc(sess.clientDescription)
    const doc = this.open.get(nodeIdText(sess?.authenticationToken))
    if (doc?.user) peer.user = doc.user
    if (doc?.mac) p.peerMac = doc.mac
    if (channel?.clientCertificate) {
      const c = certDoc(channel.clientCertificate)
      if (c) peer.tls = c
    }
    if (Object.keys(peer).length > 0) p.peer = peer
    return p
  }

  sessionOf(request) {
    try {
      const token = request?.requestHeader?.authenticationToken
      if (!token || !this.server) return null
      return this.server.getSession(token) || null
    } catch (e) {
      return null
    }
  }

  // ------------------------------------------------------------ hooks

  attach(server) {
    this.server = server
    installMessageBuilderHook()
    server.on('newChannel', (channel) => this.onNewChannel(channel))
    server.on('openSecureChannelRequest', (request, channel) => bindBuilder(channel, this))
    server.on('closeChannel', (channel) => {
      if (channel && channelsById.get(channel.channelId)?.channel === channel)
        channelsById.delete(channel.channelId)
    })
    server.on('connectionRefused', (socketData) => {
      const ip = ipOf(socketData?.remoteAddress)
      this.emitThrottled({
        kind: 'conn.refused',
        action: 'rejected',
        peerAddress: addrOf(ip, socketData?.remotePort),
        peerIp: ip,
        detail: socketData?.reason || 'connection refused',
      })
    })
    server.on('openSecureChannelFailure', (socketData, channelData) => {
      const ip = ipOf(socketData?.remoteAddress)
      const ch = channelsById.get(channelData?.channelId)?.channel
      if (ch && this.rejectedCertChannels.has(ch)) return // reported as tls.cert_rejected
      this.emitThrottled({
        kind: 'conn.open_failed',
        action: 'disconnected',
        peerAddress: addrOf(ip, socketData?.remotePort),
        peerIp: ip,
        detail: socketData?.reason || 'the secure channel was not opened',
        fields: {
          secureChannelId: channelData?.channelId,
          securityPolicy: channelData?.securityPolicy,
          securityMode: enumName(MessageSecurityMode, channelData?.securityMode),
          bytesRead: socketData?.bytesRead,
        },
      })
    })
    server.on('create_session', (session) => this.onCreateSession(session))
    server.on('session_activated', (session, token) => this.onSessionActivated(session, token))
    server.on('session_closed', (session) => {
      if (this.open.delete(nodeIdText(session?.authenticationToken))) this.dirty = true
    })
    // a "request" listener turns node-opcua's lean Read/Write path off:
    // every request is decoded and seen here
    server.on('request', (request, channel) => this.onRequest(request, channel))
    server.on('response', (response, channel) => {
      try {
        this.onResponse(response, channel)
      } catch (e) {
        this.log('secEvents - response check: ' + (e?.stack || e))
      }
    })
  }

  // a channel refused by the ipAddresses allow-list (index.js closes it)
  unexpectedConnection(channel) {
    const ip = ipOf(channel?.remoteAddress)
    this.emitThrottled({
      kind: 'conn.unexpected',
      action: 'rejected',
      peerAddress: addrOf(ip, channel?.remotePort),
      peerIp: ip,
      sessionId: channel?.channelId,
      detail: 'address not in ipAddresses',
    })
  }

  onNewChannel(channel) {
    if (!channel) return
    if (Number.isFinite(channel.channelId)) channelsById.set(channel.channelId, { channel, sec: this })
    // the verdict of the certificate manager on the client's certificate
    if (typeof channel.checkCertificate === 'function' && !channel._jsSecWrapped) {
      channel._jsSecWrapped = true
      const orig = channel.checkCertificate.bind(channel)
      channel.checkCertificate = async (certificate) => {
        const sc = await orig(certificate)
        try {
          if (certificate && !isGood(sc)) {
            this.rejectedCertChannels.add(channel)
            const c = certDoc(certificate)
            const ip = ipOf(channel.remoteAddress)
            this.emitThrottled(
              {
                kind: 'tls.cert_rejected',
                action: 'rejected',
                peerAddress: addrOf(ip, channel.remotePort),
                peerIp: ip,
                sessionId: channel.channelId,
                peer: c ? { tls: c } : undefined,
                error: statusName(sc),
                detail: 'client certificate refused: ' + statusName(sc),
                fields: { status: statusName(sc) },
              },
              ['cert', ip, c?.sha256, statusName(sc)].join('|'),
              this.certThrottle
            )
          }
        } catch (e) {}
        return sc
      }
    }
  }

  // the client of a new session: its application description, certificate,
  // the security of its channel and its address
  onCreateSession(session, keep = null) {
    if (!this.ident || !session) return
    const channel = session.channel
    const ip = ipOf(channel?.remoteAddress)
    const app = appDoc(session.clientDescription)
    const doc = {
      sessionName: session.sessionName,
      address: addrOf(ip, channel?.remotePort),
      ip,
      application: app,
      endpointUrl: session.endpoint?.endpointUrl,
      securityPolicy: channel?.securityPolicy,
      securityMode: enumName(MessageSecurityMode, channel?.securityMode),
      secureChannelId: channel?.channelId,
      connectedAt: keep?.connectedAt || new Date(),
    }
    if (keep?.user) doc.user = keep.user
    const cert = channel?.clientCertificate ? certDoc(channel.clientCertificate) : null
    if (cert) doc.tls = cert
    for (const k of Object.keys(doc)) if (doc[k] === undefined || doc[k] === '') delete doc[k]
    const key = nodeIdText(session.authenticationToken)
    this.open.set(key, doc)
    this.dirty = true
    const base = { peerAddress: doc.address, peerIp: ip, sessionId: channel?.channelId }
    const peer = { application: app, ...(cert ? { tls: cert } : {}) }

    const uri = app.applicationUri || (ip ? 'ip:' + ip : '')
    if (uri) {
      const seen = this.known.has(uri)
      for (const [field, value] of [
        ['product', app.productUri],
        ['name', app.applicationName],
      ]) {
        const prev = this.trackers.observe(field + '|' + uri, value)
        if (prev !== null)
          this.emit({
            ...base,
            peer,
            kind: 'peer.identity_changed',
            objectRef: uri,
            value,
            previous: prev,
            detail: `${field === 'product' ? 'productUri' : 'applicationName'} of ${uri} changed`,
          })
      }
      const prevCert = this.trackers.observe('cert|' + uri, cert?.sha256)
      if (prevCert !== null)
        this.emit({
          ...base,
          peer,
          kind: 'tls.cert_changed',
          objectRef: uri,
          value: cert.sha256,
          previous: prevCert,
          detail: `the certificate of ${uri} changed`,
        })
      this.known.set(uri, {
        applicationUri: app.applicationUri || '',
        productUri: app.productUri || '',
        applicationName: app.applicationName || '',
        applicationType: app.applicationType || '',
        ip,
        mac: this.known.get(uri)?.ip === ip ? this.known.get(uri)?.mac || '' : '',
        tlsSha256: cert?.sha256 || '',
        tlsSubject: cert?.subject || '',
        lastSeen: new Date(),
      })
      if (this.known.size > MAX_KNOWN) {
        let oldest = null
        for (const [k, v] of this.known) if (!oldest || v.lastSeen < this.known.get(oldest).lastSeen) oldest = k
        this.known.delete(oldest)
      }
      if (!seen)
        this.emit({
          ...base,
          peer,
          kind: 'peer.identified',
          objectRef: uri,
          value: summary(app, cert),
          detail: 'first session of this client application',
        })
    }

    // the MAC address, off the request path
    if (ip)
      lookupMac(ip).then((mac) => {
        if (!mac) return
        doc.mac = mac
        doc.macSource = 'arp'
        const k = this.known.get(uri)
        if (k && k.ip === ip) k.mac = mac
        this.dirty = true
        const prev = this.trackers.observe('mac|' + ip, mac)
        if (prev !== null)
          this.emit({
            ...base,
            peerMac: mac,
            peer: { ...peer, macSource: 'arp' },
            kind: 'peer.mac_changed',
            value: mac,
            previous: prev,
            detail: `MAC address was ${prev}`,
          })
      })
  }

  onSessionActivated(session, token) {
    if (!this.ident || !session) return
    let doc = this.open.get(nodeIdText(session.authenticationToken))
    if (!doc) return
    // a session moved to another secure channel (the client reconnected):
    // its address, MAC, certificate and channel security are the new
    // channel's, and are compared again
    if (session.channel && session.channel.channelId !== doc.secureChannelId) {
      this.onCreateSession(session, doc)
      doc = this.open.get(nodeIdText(session.authenticationToken))
      if (!doc) return
    }
    doc.user = userDoc(token)
    doc.activatedAt = new Date()
    this.dirty = true
  }

  onRequest(request, channel) {
    const handle = request?.requestHeader?.requestHandle
    if (!channel || handle === undefined) return
    let m = this.pendingRequests.get(channel)
    if (!m) {
      m = new Map()
      this.pendingRequests.set(channel, m)
    }
    if (m.size > 1000) {
      const now = Date.now()
      for (const [h, v] of m) if (now - v.at > PENDING_REQUEST_MS) m.delete(h)
      if (m.size > 1000) m.clear()
    }
    m.set(handle, { request, at: Date.now() })
  }

  onResponse(response, channel) {
    const handle = response?.responseHeader?.requestHandle
    const m = this.pendingRequests.get(channel)
    const entry = m?.get(handle)
    if (!entry) return
    m.delete(handle)
    const request = entry.request
    const reqName = request?.schema?.name || request?.constructor?.name || ''
    const session = this.sessionOf(request)
    const serviceResult = response.responseHeader?.serviceResult
    const respName = response?.schema?.name || response?.constructor?.name || ''

    if (respName === 'ServiceFault' || !isGood(serviceResult)) {
      this.onFault(request, reqName, serviceResult, channel, session)
      return
    }
    switch (reqName) {
      case 'ReadRequest':
        this.checkUnknown('Read', request.nodesToRead, (x) => x?.nodeId, response.results, channel, session)
        break
      case 'BrowseRequest':
        this.checkUnknown('Browse', request.nodesToBrowse, (x) => x?.nodeId, response.results, channel, session)
        break
      case 'CreateMonitoredItemsRequest':
        this.checkUnknown(
          'CreateMonitoredItems',
          request.itemsToCreate,
          (x) => x?.itemToMonitor?.nodeId,
          response.results,
          channel,
          session
        )
        break
      case 'HistoryReadRequest':
        this.checkUnknown('HistoryRead', request.nodesToRead, (x) => x?.nodeId, response.results, channel, session)
        break
      case 'TranslateBrowsePathsToNodeIdsRequest':
        this.checkUnknown(
          'TranslateBrowsePaths',
          request.browsePaths,
          (x) => x?.startingNode,
          response.results,
          channel,
          session
        )
        break
      case 'WriteRequest':
        this.checkWrites(request, response.results, channel, session)
        break
      case 'CallRequest':
        this.checkCalls(request, response.results, channel, session)
        break
      case 'CreateSessionRequest':
        this.checkEndpointUrl(request, channel)
        break
      case 'AddNodesRequest':
      case 'AddReferencesRequest':
      case 'DeleteNodesRequest':
      case 'DeleteReferencesRequest':
      case 'HistoryUpdateRequest':
        this.emit({
          ...this.peerOfChannel(channel, session),
          kind: 'ctrl.config_change',
          action: (response.results || []).every((r) => isGood(r?.statusCode ?? r)) ? 'observed' : 'rejected',
          detail: reqName.replace(/Request$/, '') + ' request',
          fields: { service: reqName.replace(/Request$/, '') },
        })
        break
    }
  }

  onFault(request, reqName, sc, channel, session) {
    const n = statusName(sc)
    if (ROUTINE_FAULT_REQUESTS.has(reqName) || ROUTINE_FAULTS.has(n)) return
    const kind = faultKind(sc, reqName)
    const e = {
      ...this.peerOfChannel(channel, session),
      kind,
      action: 'rejected',
      error: n,
      detail: `${reqName.replace(/Request$/, '')} refused: ${n}`,
      fields: { service: reqName.replace(/Request$/, ''), status: n },
    }
    if (reqName === 'ActivateSessionRequest') {
      const u = userDoc(request?.userIdentityToken)
      if (u) e.peer = { ...(e.peer || {}), user: u }
    }
    if (reqName === 'CreateSessionRequest' && request?.clientDescription)
      e.peer = { ...(e.peer || {}), application: appDoc(request.clientDescription) }
    this.emitThrottled(e)
  }

  // requests for nodes the server does not have
  checkUnknown(service, items, nodeOf, results, channel, session) {
    if (!Array.isArray(items) || !Array.isArray(results)) return
    for (let i = 0; i < items.length && i < results.length; i++) {
      const sc = results[i]?.statusCode ?? results[i]
      if (!is(sc, StatusCodes.BadNodeIdUnknown) && !is(sc, StatusCodes.BadNodeIdInvalid)) continue
      const ref = nodeIdText(nodeOf(items[i]))
      this.noteUnknown(channel, session, {
        kind: 'access.unknown_node',
        action: 'rejected',
        objectRef: ref,
        detail: `${service} of a node the server does not have`,
        fields: { service, status: statusName(sc) },
      })
    }
  }

  // one event per unknown node and address and period, and the scan
  // detector: many requests for unknown nodes from one address
  noteUnknown(channel, session, e) {
    const p = this.peerOfChannel(channel, session)
    this.emitThrottled({ ...p, ...e }, ['u', e.kind, p.peerIp, e.objectRef].join('|'), this.unknownThrottle)
    const hits = this.scan.hit(p.peerIp || '?')
    if (hits > 0)
      this.emit({
        ...p,
        kind: 'access.scan',
        value: String(hits),
        detail: `${hits} requests for unknown nodes within ${SCAN_WINDOW_MS / 1000} s`,
        fields: { service: e.fields?.service },
      })
  }

  // the writes the command path (sendCommand) did not see: unknown nodes,
  // variables that are not commands, other attributes, values node-opcua
  // refused before the setter
  checkWrites(request, results, channel, session) {
    const items = request?.nodesToWrite
    if (!Array.isArray(items) || !Array.isArray(results)) return
    const token = nodeIdText(request?.requestHeader?.authenticationToken)
    for (let i = 0; i < items.length && i < results.length; i++) {
      const w = items[i]
      const sc = results[i]
      const ref = nodeIdText(w?.nodeId)
      const hk = token + '|' + ref
      const handled = this.handledWrites.get(hk)
      if (handled) {
        if (handled <= 1) this.handledWrites.delete(hk)
        else this.handledWrites.set(hk, handled - 1)
        continue
      }
      const status = statusName(sc)
      const good = isGood(sc)
      const base = { objectRef: ref, fields: { service: 'Write', attribute: attributeName(w?.attributeId), status } }
      const isConfig = w?.attributeId !== AttributeIds.Value || w?.nodeId?.namespace === 0
      if (!isConfig) this.noteCommandAttempt(this.peerOfChannel(channel, session), undefined, ref, base.fields)
      let e
      if (isConfig) {
        e = {
          kind: 'ctrl.config_change',
          action: good ? 'observed' : 'rejected',
          detail: `write of ${attributeName(w?.attributeId)}` + (good ? '' : ': ' + status),
        }
      } else if (is(sc, StatusCodes.BadNodeIdUnknown) || is(sc, StatusCodes.BadNodeIdInvalid)) {
        this.noteUnknown(channel, session, {
          ...base,
          kind: this.commandsEnabled ? 'cmd.object_unknown' : 'cmd.disabled',
          action: 'rejected',
          detail: this.commandsEnabled
            ? 'command to a node the server does not have'
            : 'command while commands are disabled (commandsEnabled false)',
        })
        continue
      } else if (good) {
        continue
      } else if (!this.commandsEnabled) {
        e = { kind: 'cmd.disabled', action: 'rejected', detail: 'command while commands are disabled: ' + status }
      } else if (!this.commandNodes.has(ref)) {
        e = { kind: 'access.write_denied', action: 'rejected', detail: 'write to a variable that is not a command' }
      } else if (
        is(sc, StatusCodes.BadTypeMismatch) ||
        is(sc, StatusCodes.BadOutOfRange) ||
        is(sc, StatusCodes.BadIndexRangeInvalid) ||
        is(sc, StatusCodes.BadWriteNotSupported)
      ) {
        e = { kind: 'cmd.wrong_type', action: 'rejected', detail: 'command value refused: ' + status }
      } else {
        e = { kind: 'cmd.rejected', action: 'rejected', detail: 'command refused: ' + status }
      }
      this.emitThrottled({ ...this.peerOfChannel(channel, session), ...base, ...e })
    }
  }

  // method calls: restart, configuration and file transfer methods, and
  // every other call except the routine ones; calls of unknown methods
  checkCalls(request, results, channel, session) {
    const items = request?.methodsToCall
    if (!Array.isArray(items) || !Array.isArray(results)) return
    const as = this.server?.engine?.addressSpace
    for (let i = 0; i < items.length && i < results.length; i++) {
      const c = items[i]
      const sc = results[i]?.statusCode
      const status = statusName(sc)
      const methodId = nodeIdText(c?.methodId)
      const objectId = nodeIdText(c?.objectId)
      const p = this.peerOfChannel(channel, session)
      const fields = {
        service: 'Call',
        methodId,
        objectId,
        arguments: Array.isArray(c?.inputArguments) ? c.inputArguments.length : 0,
        status,
      }
      if (
        is(sc, StatusCodes.BadMethodInvalid) ||
        is(sc, StatusCodes.BadNodeIdUnknown) ||
        is(sc, StatusCodes.BadNodeIdInvalid)
      ) {
        this.noteUnknown(channel, session, {
          kind: 'cmd.object_unknown',
          action: 'rejected',
          objectRef: methodId,
          detail: 'call of a method the server does not have',
          fields,
        })
        continue
      }
      let methodName = ''
      let objectType = ''
      try {
        methodName = as?.findNode(c.methodId)?.browseName?.name || ''
        objectType = as?.findNode(c.objectId)?.typeDefinitionObj?.browseName?.name || ''
      } catch (e) {}
      if (ROUTINE_METHODS.has(methodName) && objectType !== 'ProgramStateMachineType') continue
      fields.method = methodName
      fields.objectType = objectType
      let kind = 'ctrl.method_call'
      if (methodName === 'RequestServerStateChange' || /ProgramStateMachine/.test(objectType)) kind = 'ctrl.restart'
      else if (/^(FileType|FileDirectoryType|TrustListType)$/.test(objectType) || /File/.test(objectType))
        kind = 'file.transfer'
      else if (/ServerConfiguration|CertificateGroup|TrustList/.test(objectType + ' ' + methodName))
        kind = 'ctrl.config_change'
      if (
        kind === 'ctrl.method_call' &&
        (is(sc, StatusCodes.BadArgumentsMissing) ||
          is(sc, StatusCodes.BadTooManyArguments) ||
          is(sc, StatusCodes.BadInvalidArgument) ||
          is(sc, StatusCodes.BadTypeMismatch))
      )
        kind = 'cmd.wrong_type'
      this.emitThrottled(
        {
          ...p,
          kind,
          action: isGood(sc) ? 'observed' : 'rejected',
          objectRef: methodId,
          detail: `call of ${methodName || methodId}${objectType ? ' on a ' + objectType : ''}` + (isGood(sc) ? '' : ': ' + status),
          fields,
        },
        [kind, p.peerIp, methodId, objectId, status].join('|')
      )
    }
  }

  // a CreateSession naming an endpoint the server does not have: a port or
  // path that is not one of its endpoints' (the host is not compared:
  // clients use IP addresses, aliases and names the server does not know)
  checkEndpointUrl(request, channel) {
    const url = request?.endpointUrl
    if (!url || !Array.isArray(this.server?.endpoints)) return
    const key = endpointKey(url)
    if (!key) return
    const ours = new Set()
    try {
      for (const ep of this.server.endpoints)
        for (const d of ep.endpointDescriptions()) ours.add(endpointKey(d.endpointUrl))
    } catch (e) {
      return
    }
    if (ours.size === 0 || ours.has(key)) return
    this.emitThrottled({
      ...this.peerOfChannel(channel),
      kind: 'link.endpoint_mismatch',
      value: url,
      detail: 'CreateSession for an endpoint URL the server does not have',
      fields: { service: 'CreateSession' },
    })
  }

  // ------------------------------------------------- command path hooks

  // The context node-opcua passes to a variable's setter: the session and
  // its channel.
  peerOfContext(context) {
    const session = context?.session || null
    return this.peerOfChannel(session?.channel, session)
  }

  // sendCommand reports its outcome; the write response check then skips
  // the node. info: { tag, nodeId, value, status, kind, detail }
  commandOutcome(context, info) {
    const token = nodeIdText(context?.session?.authenticationToken)
    const hk = token + '|' + info.nodeId
    this.handledWrites.set(hk, (this.handledWrites.get(hk) || 0) + 1)
    if (this.handledWrites.size > 4096) this.handledWrites.clear()
    const p = this.peerOfContext(context)
    const fields = { service: 'Write', status: info.status }
    if (info.value !== undefined) fields.value = info.value
    if (info.asdu) fields.protocolSourceASDU = info.asdu
    this.noteCommandAttempt(p, info.tag, info.nodeId, fields)
    if (info.internal) {
      // not queued: the database failed (not the client's doing, but the
      // command was refused and the client told BadInternalError)
      this.emitThrottled({
        ...p,
        kind: 'cmd.rejected',
        action: 'rejected',
        tag: info.tag,
        objectRef: info.nodeId,
        error: info.error,
        detail: 'command not queued: database error',
        fields,
      })
      return
    }
    if (info.kind) {
      this.emitThrottled({
        ...p,
        kind: info.kind,
        action: 'rejected',
        tag: info.tag,
        objectRef: info.nodeId,
        detail: info.detail,
        fields,
      })
      return
    }
    // accepted: the commands that would change nothing
    if (info.unnecessary)
      this.emitThrottled({
        ...p,
        kind: 'cmd.unnecessary',
        tag: info.tag,
        objectRef: info.nodeId,
        value: String(info.value),
        detail: 'the supervised point already was in the commanded state',
        fields,
      })
  }

  // command flooding: every attempt counts (accepted, refused by the
  // command path, or refused by node-opcua before it)
  noteCommandAttempt(p, tag, ref, fields) {
    const hits = this.flood.hit(p.peerIp || '?')
    if (hits > 0)
      this.emit({
        ...p,
        kind: 'flood.commands',
        tag,
        objectRef: ref,
        value: String(hits),
        detail: `${hits} command attempts within ${COMMAND_FLOOD_WINDOW_MS / 1000} s`,
        fields,
      })
  }

  // stats.peerIdentity: the open sessions and the last identity of every
  // client application seen; null when identification is off
  statsDoc() {
    if (!this.ident) return null
    const clients = [...this.open.values()].sort((a, b) => String(a.address).localeCompare(String(b.address)))
    const known = [...this.known.values()].sort((a, b) =>
      String(a.applicationUri).localeCompare(String(b.applicationUri))
    )
    return { clients, known }
  }
}

// the port and path of an endpoint URL, case-insensitive, no trailing slash
function endpointKey(url) {
  const m = /^opc\.tcp:\/\/(\[[^\]]*\]|[^/:]*)(?::(\d+))?(\/.*)?$/i.exec(String(url).trim())
  if (!m) return ''
  return (m[2] || '4840') + (m[3] || '').replace(/\/+$/, '').toLowerCase()
}

function appDoc(d) {
  if (!d) return {}
  const out = {
    applicationUri: d.applicationUri || '',
    productUri: d.productUri || '',
    applicationName: d.applicationName?.text || '',
    applicationType: enumName(ApplicationType, d.applicationType),
  }
  if (d.gatewayServerUri) out.gatewayServerUri = d.gatewayServerUri
  if (d.discoveryProfileUri) out.discoveryProfileUri = d.discoveryProfileUri
  if (Array.isArray(d.discoveryUrls) && d.discoveryUrls.length > 0) out.discoveryUrls = d.discoveryUrls.slice(0, 8)
  for (const k of Object.keys(out)) if (out[k] === '') delete out[k]
  return out
}

// the user identity of a session, never a password or a token
function userDoc(token) {
  if (!token) return null
  const t = token.schema?.name || token.constructor?.name || ''
  switch (t) {
    case 'AnonymousIdentityToken':
      return { tokenType: 'anonymous' }
    case 'UserNameIdentityToken':
      return { tokenType: 'username', userName: token.userName || '' }
    case 'X509IdentityToken': {
      const c = certDoc(token.certificateData)
      return { tokenType: 'certificate', ...(c ? { certificate: c } : {}) }
    }
    case 'IssuedIdentityToken':
      return { tokenType: 'issued' }
  }
  return { tokenType: t || 'unknown' }
}

function summary(app, cert) {
  const parts = [app.applicationName, app.productUri, app.applicationUri].filter((x) => x)
  if (cert?.subject) parts.push('cert ' + cert.subject)
  return parts.join(' | ')
}

// node-opcua reports malformed messages and sequence number errors only to
// the channel's private message builder; observe them on its prototype.
// Each builder is bound to its channel when it delivers the channel's first
// OpenSecureChannel request: node-opcua forwards that request, with the
// channel, as a server event while the builder's "message" event is being
// emitted (see bindBuilder). The channel id of a message header is the
// client's claim and only recorded (receivedChannelId). Errors before that
// first request (a channel that never opens) are conn.open_failed events.
let builderHooked = false
let emittingBuilder = null // the server builder emitting "message" now
const builderChannel = new WeakMap() // builder -> { channel, sec }
const channelBuilder = new WeakMap() // channel -> builder (tests)

function installMessageBuilderHook() {
  if (builderHooked || typeof MessageBuilder?.prototype?.emit !== 'function') return
  builderHooked = true
  const orig = MessageBuilder.prototype.emit
  MessageBuilder.prototype.emit = function (name, ...args) {
    const server = typeof this.id === 'string' && this.id.startsWith('server')
    if (server && name === 'message' && !builderChannel.has(this)) {
      const prev = emittingBuilder
      emittingBuilder = this
      try {
        return orig.call(this, name, ...args)
      } finally {
        emittingBuilder = prev
      }
    }
    try {
      if (server && (name === 'error' || name === 'invalid_sequence_number' || name === 'invalid_message'))
        builderEvent(this, name, args)
    } catch (e) {}
    return orig.call(this, name, ...args)
  }
}

// the server's openSecureChannelRequest event: bind the builder delivering
// it to the channel node-opcua names
function bindBuilder(channel, sec) {
  if (!emittingBuilder || !channel || builderChannel.has(emittingBuilder)) return
  builderChannel.set(emittingBuilder, { channel, sec })
  channelBuilder.set(channel, emittingBuilder)
}

function builderEvent(builder, name, args) {
  const reg = builderChannel.get(builder)
  const sec = reg?.sec
  if (!sec) return
  const p = sec.peerOfChannel(reg.channel)
  const ids = { secureChannelId: reg.channel.channelId, receivedChannelId: builder.channelId }
  if (name === 'invalid_sequence_number') {
    builder._jsSecSeqReported = true
    sec.emitThrottled({
      ...p,
      kind: 'seq.invalid',
      action: 'disconnected',
      detail: `sequence number ${args[1]}, expected ${args[0]}`,
      fields: { expectedSeq: args[0], sequenceNumber: args[1], ...ids },
    })
    return
  }
  if (name === 'invalid_message') {
    sec.emitThrottled({
      ...p,
      kind: 'proto.malformed',
      action: 'ignored',
      detail: 'message of an unexpected type: ' + (args[0]?.constructor?.name || ''),
      fields: { ...ids },
    })
    return
  }
  // error: the channel is closed
  if (builder._jsSecSeqReported) return
  const sc = args[1]
  const n = statusName(sc)
  sec.emitThrottled({
    ...p,
    kind: n === 'BadSecurityChecksFailed' ? 'proto.security_check_failed' : 'proto.malformed',
    action: 'disconnected',
    error: n,
    detail: args[0]?.message || n,
    fields: { status: n, ...ids, requestId: args[2] ?? undefined },
  })
}

module.exports = {
  SecEventWriter,
  ConnSec,
  COLLECTION,
  // exported for the tests
  certDoc,
  parseProcArp,
  parseArpOutput,
  normalizeMac,
  ipOf,
  Throttle,
  RateLimit,
  Tracker,
  completeEvent,
  onlyDuplicates,
  builderOf: (channel) => channelBuilder.get(channel),
}
