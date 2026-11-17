/*
 * MQTT-Sparkplug B Client Lib Adapter for JSON-SCADA
 *
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Derived from https://github.com/Cirrus-Link/Sparkplug/tree/master/client_libraries/javascript/sparkplug-client
 *
 */

/**
 * Copyright (c) 2016-2017 Cirrus Link Solutions
 *
 *  All rights reserved. This program and the accompanying materials
 *  are made available under the terms of the Eclipse Public License v1.0
 *  which accompanies this distribution, and is available at
 *  http://www.eclipse.org/legal/epl-v10.html
 *
 * Contributors:
 *   Cirrus Link Solutions
 */

var mqtt = require('mqtt'),
  sparkplug = require('sparkplug-payload'),
  sparkplugbpayload = sparkplug.get('spBv1.0'),
  ProtoPayload = require('sparkplug-payload/lib/sparkplugPayloadProto').org
    .eclipse.tahu.protobuf.Payload,
  events = require('events'),
  util = require('util'),
  pako = require('pako')

var compressed = 'SPBV1.0_COMPRESSED'
var STOP_TIMEOUT = 3000 // ms to wait for the death/OFFLINE ack on stop
var DATATYPE_TEMPLATE = 19

const { transports, createLogger, format } = require('winston')
const logger = createLogger({
  format: format.combine(
    format.printf(
      (info) =>
        `${new Date().toISOString()} - SparkplugClientLib - ${info.message}`
    )
  ),
  transports: [new transports.Console()],
})

logger.level = 'warn'

var getRequiredProperty = function (config, propName) {
  if (config[propName] !== undefined) {
    return config[propName]
  }
  throw new Error("Missing required configuration property '" + propName + "'")
}

var getProperty = function (config, propName, defaultValue) {
  if (config[propName] !== undefined) {
    return config[propName]
  } else {
    return defaultValue
  }
}

var hasOwn = function (obj, prop) {
  return Object.prototype.hasOwnProperty.call(obj, prop)
}

/*
 * Datatypes declared in BIRTH messages, by node/device locator.
 * Sparkplug B 3.0 senders omit the datatype (and may use only the alias) in
 * DATA and CMD messages; sparkplug-payload then decodes no type and no value.
 * Missing datatypes and names are restored from the last BIRTH before decoding.
 */
function DatatypeRegistry() {
  var entries = new Map()

  function record(entry, metrics, prefix) {
    metrics.forEach(function (m) {
      if (!m.datatype) return
      var hasName = hasOwn(m, 'name') && m.name !== ''
      var info = { datatype: m.datatype, name: hasName ? m.name : undefined }
      if (hasName) entry.byName.set(prefix + m.name, info)
      if (prefix === '' && hasOwn(m, 'alias'))
        entry.byAlias.set(String(m.alias), info)
      if (
        m.datatype === DATATYPE_TEMPLATE &&
        hasName &&
        m.templateValue &&
        Array.isArray(m.templateValue.metrics)
      )
        record(entry, m.templateValue.metrics, prefix + m.name + '/')
    })
  }

  function fill(entry, metrics, prefix) {
    var changed = false
    metrics.forEach(function (m) {
      var info
      var hasName = hasOwn(m, 'name') && m.name !== ''
      if (hasName) info = entry.byName.get(prefix + m.name)
      if (!info && prefix === '' && hasOwn(m, 'alias'))
        info = entry.byAlias.get(String(m.alias))
      if (!info) return
      if (!m.datatype) {
        m.datatype = info.datatype
        changed = true
      }
      if (!hasName && info.name !== undefined) {
        m.name = info.name
        hasName = true
        changed = true
      }
      if (
        m.datatype === DATATYPE_TEMPLATE &&
        hasName &&
        m.templateValue &&
        Array.isArray(m.templateValue.metrics)
      )
        changed =
          fill(entry, m.templateValue.metrics, prefix + m.name + '/') || changed
    })
    return changed
  }

  // a new BIRTH replaces what was known for that locator; an NBIRTH also
  // drops the devices of the node (they will be birthed again)
  this.birth = function (locator, rawPayload, isNodeBirth) {
    if (isNodeBirth)
      Array.from(entries.keys()).forEach(function (k) {
        if (k === locator || k.startsWith(locator + '/')) entries.delete(k)
      })
    var entry = { byName: new Map(), byAlias: new Map() }
    entries.set(locator, entry)
    if (Array.isArray(rawPayload.metrics)) record(entry, rawPayload.metrics, '')
  }

  // returns true when the raw payload was changed
  this.fill = function (locator, rawPayload) {
    var entry = entries.get(locator)
    if (!entry || !Array.isArray(rawPayload.metrics)) return false
    return fill(entry, rawPayload.metrics, '')
  }
}

/*
 * sparkplug-payload decodes DataSet cells only for UInt32 (long value), Int64,
 * Float, Double, Boolean and String columns and throws on any other type,
 * losing the whole message. Cells of other column types are rewritten before
 * decoding: Int8..UInt32 (int value) -> Double, UInt64 and DateTime -> Int64,
 * Text and UUID -> String. Returns true when the raw payload was changed.
 */
var DS_INT64 = 4,
  DS_DOUBLE = 10,
  DS_STRING = 12,
  DATATYPE_DATASET = 16

function isSet(v) {
  return v !== null && v !== undefined
}

function normalizeDataSet(ds) {
  if (!ds || !Array.isArray(ds.types)) return false
  var types = ds.types
  var target = types.map(function (t) {
    switch (t) {
      case 1: // Int8
      case 2: // Int16
      case 3: // Int32
      case 5: // UInt8
      case 6: // UInt16
      case 7: // UInt32
        return DS_DOUBLE
      case 8: // UInt64
      case 13: // DateTime
        return DS_INT64
      case 14: // Text
      case 15: // UUID
        return DS_STRING
      default:
        return t
    }
  })
  if (
    target.every(function (t, i) {
      return t === types[i]
    })
  )
    return false
  ;(Array.isArray(ds.rows) ? ds.rows : []).forEach(function (row) {
    var elements = Array.isArray(row?.elements) ? row.elements : []
    elements.forEach(function (el, i) {
      if (!el || target[i] !== DS_DOUBLE || types[i] === DS_DOUBLE) return
      var n = null
      if (isSet(el.intValue)) {
        n = Number(el.intValue) >>> 0
        if (types[i] === 1) n = (n << 24) >> 24
        else if (types[i] === 2) n = (n << 16) >> 16
        else if (types[i] === 3) n = n | 0
      } else if (isSet(el.longValue)) n = Number(el.longValue)
      delete el.intValue
      delete el.longValue
      if (n !== null) el.doubleValue = n
    })
  })
  ds.types = target
  return true
}

function normalizeDataSets(metrics) {
  var changed = false
  ;(Array.isArray(metrics) ? metrics : []).forEach(function (m) {
    if (!m) return
    if (m.datatype === DATATYPE_DATASET && m.datasetValue)
      changed = normalizeDataSet(m.datasetValue) || changed
    if (m.datatype === DATATYPE_TEMPLATE && m.templateValue)
      changed = normalizeDataSets(m.templateValue.metrics) || changed
  })
  return changed
}

// top level metrics with no datatype (not declared in a known BIRTH)
function countUntyped(metrics) {
  return (Array.isArray(metrics) ? metrics : []).filter(function (m) {
    return m && !m.datatype
  }).length
}

/*
 * Host Application STATE session. Each format runs on its own MQTT session,
 * so its will (OFFLINE) is independent from the edge node NDEATH will.
 *  - '3.0': topic spBv1.0/STATE/<hostId>, JSON {online, timestamp}
 *  - '2.2': topic STATE/<hostId>, plain ONLINE/OFFLINE
 */
function StateSession(format, hostId, version, baseOptions, serverUrl) {
  var timestamp = new Date().getTime(),
    topic,
    online,
    offline,
    stopping = false
  if (format === '3.0') {
    topic = version + '/STATE/' + hostId
    online = JSON.stringify({ online: true, timestamp: timestamp })
    offline = JSON.stringify({ online: false, timestamp: timestamp })
  } else {
    topic = 'STATE/' + hostId
    online = 'ONLINE'
    offline = 'OFFLINE'
  }

  var options = {
    ...baseOptions,
    clientId:
      baseOptions.clientId ?
        baseOptions.clientId + (format === '3.0' ? '_STATE' : '_STATE22')
      : '',
    reconnectPeriod: 5000,
    will: { topic: topic, payload: offline, qos: 1, retain: true },
  }
  if (options.clientId === '') delete options.clientId

  var client = mqtt.connect(serverUrl, options)

  client.on('connect', function () {
    if (stopping) {
      client.end(true)
      return
    }
    logger.info('STATE session connected (' + format + '), ' + topic)
    client.subscribe(topic, { qos: 1 })
    client.publish(topic, online, { qos: 1, retain: true })
  })
  // a host must republish ONLINE when it sees itself reported OFFLINE
  // (e.g. a late will from a previous session)
  client.on('message', function (t, message) {
    if (stopping || t !== topic) return
    var text = message.toString()
    var isOffline = text === 'OFFLINE'
    if (format === '3.0') {
      try {
        isOffline = JSON.parse(text)?.online === false
      } catch (e) {
        isOffline = false
      }
    }
    if (isOffline) {
      logger.info('Own STATE reported offline, republishing ONLINE')
      client.publish(topic, online, { qos: 1, retain: true })
    }
  })
  client.on('error', function (err) {
    logger.warn('STATE session error (' + format + '): ' + err.message)
  })

  this.publishOnline = function () {
    if (client.connected)
      client.publish(topic, online, { qos: 1, retain: true })
  }

  // the OFFLINE publish may never be acknowledged on a dead link: the session
  // is ended anyway (and never reconnects) after a short wait
  this.stop = function () {
    stopping = true
    client.options.reconnectPeriod = 0
    if (client.connected) {
      var timer = setTimeout(function () {
        client.end(true)
      }, STOP_TIMEOUT)
      client.publish(topic, offline, { qos: 1, retain: true }, function () {
        clearTimeout(timer)
        client.end()
      })
    } else client.end(true)
  }
}

/*
 * Sparkplug Client
 */
function SparkplugClient(config) {
  var versionB = 'spBv1.0',
    serverUrl = getRequiredProperty(config, 'serverUrl'),
    username = getRequiredProperty(config, 'username'),
    password = getRequiredProperty(config, 'password'),
    groupId = getRequiredProperty(config, 'groupId'),
    edgeNode = getRequiredProperty(config, 'edgeNode'),
    clientId = getRequiredProperty(config, 'clientId'),
    scadaHostId = getProperty(config, 'scadaHostId', ''),
    publishDeath = getProperty(config, 'publishDeath', false),
    version = getProperty(config, 'version', versionB),
    ca = getProperty(config, 'ca', ''),
    key = getProperty(config, 'key', ''),
    cert = getProperty(config, 'cert', ''),
    pfx = getProperty(config, 'pfx', ''),
    passphrase = getProperty(config, 'passphrase', ''),
    minVersion = getProperty(config, 'minVersion', 'TLSv1.2'),
    maxVersion = getProperty(config, 'maxVersion', 'TLSv1.3'),
    ciphers = getProperty(config, 'ciphers', ''),
    rejectUnauthorized = getProperty(config, 'rejectUnauthorized', true),
    bdSeq = getProperty(config, 'bdSeq', 0),
    seq = getProperty(config, 'seq', 0),
    clean = getProperty(config, 'clean', true),
    keepalive = getProperty(config, 'keepalive', 5),
    connectTimeout = getProperty(config, 'connectTimeout', 30 * 1000),
    // the edge node role is active only with a group and edge node id
    isEdgeNode =
      typeof groupId === 'string' &&
      groupId.trim() !== '' &&
      typeof edgeNode === 'string' &&
      edgeNode.trim() !== '',
    stateSessions = [],
    datatypes = new DatatypeRegistry(),
    client = null,
    // Increments a sequence number
    incrementSeqNum = function () {
      if (seq == 256) {
        seq = 0
      }
      return seq++
    },
    encodePayload = function (payload) {
      return sparkplugbpayload.encodePayload(payload)
    },
    decodePayload = function (payload) {
      return sparkplugbpayload.decodePayload(payload)
    },
    decodeJsonPayload = function (payload) {
      var parsed, text

      if (!Buffer.isBuffer(payload)) return null

      text = payload.toString('utf8').trim()
      if (text === '' || (text[0] !== '{' && text[0] !== '[')) return null

      try {
        parsed = JSON.parse(text)
      } catch (e) {
        return null
      }

      if (Array.isArray(parsed)) {
        return {
          timestamp: new Date().getTime(),
          metrics: parsed,
        }
      }

      if (
        parsed !== null &&
        typeof parsed === 'object' &&
        (Array.isArray(parsed.metrics) ||
          'timestamp' in parsed ||
          'seq' in parsed)
      )
        return parsed

      return null
    },
    addSeqNumber = function (payload) {
      payload.seq = incrementSeqNum()
    },
    // Get DEATH payload
    getDeathPayload = function () {
      return {
        timestamp: new Date().getTime(),
        metrics: [
          {
            name: 'bdSeq',
            value: bdSeq,
            type: 'uint64',
          },
        ],
      }
    },
    // Logs a message alert to the console
    messageAlert = function (alert, topic, payload) {
      if (logger.level !== 'debug') return
      logger.debug('Message ' + alert)
      logger.debug(' topic: ' + topic)
      logger.debug(' payload: ' + JSON.stringify(payload))
    },
    compressPayload = function (payload, options) {
      var algorithm = null,
        resultPayload = {
          uuid: compressed,
        }

      if (logger.level === 'debug')
        logger.debug('Compressing payload ' + JSON.stringify(options))

      if (options !== undefined && options !== null) {
        if (options['algorithm']) {
          algorithm = options['algorithm']
        }
      }

      if (algorithm === null || algorithm.toUpperCase() === 'DEFLATE') {
        resultPayload.body = pako.deflate(payload)
      } else if (algorithm.toUpperCase() === 'GZIP') {
        resultPayload.body = pako.gzip(payload)
      } else {
        throw new Error('Unknown or unsupported algorithm ' + algorithm)
      }

      if (algorithm !== null) {
        resultPayload.metrics = [
          {
            name: 'algorithm',
            value: algorithm.toUpperCase(),
            type: 'string',
          },
        ]
      }

      return resultPayload
    },
    decompressPayload = function (payload) {
      var metrics = payload.metrics,
        algorithm = null

      if (metrics !== undefined && metrics !== null) {
        for (var i = 0; i < metrics.length; i++) {
          if (metrics[i].name === 'algorithm') {
            algorithm = metrics[i].value
          }
        }
      }

      if (algorithm === null || algorithm.toUpperCase() === 'DEFLATE') {
        return pako.inflate(payload.body)
      } else if (algorithm.toUpperCase() === 'GZIP') {
        return pako.ungzip(payload.body)
      } else {
        throw new Error('Unknown or unsupported algorithm ' + algorithm)
      }
    },
    maybeCompressPayload = function (payload, options) {
      if (options !== undefined && options !== null && options.compress) {
        return compressPayload(encodePayload(payload), options)
      } else {
        return payload
      }
    },
    // decode a (possibly compressed) Sparkplug B protobuf payload, restoring
    // datatypes and names omitted in DATA/CMD messages from the last BIRTH
    decodeSparkplugPayload = function (message, topicInfo) {
      // the compression check uses the raw decoding: the library decoder
      // would throw on DataSets that are only fixed below
      var bytes = message
      if (ProtoPayload.decode(bytes).uuid === compressed)
        bytes = decompressPayload(decodePayload(bytes))

      var raw = ProtoPayload.decode(bytes),
        changed = false,
        locator =
          topicInfo.groupId +
          '/' +
          topicInfo.edgeNodeId +
          (topicInfo.deviceId ? '/' + topicInfo.deviceId : '')

      switch (topicInfo.msgType) {
        case 'NBIRTH':
          datatypes.birth(locator, raw, true)
          break
        case 'DBIRTH':
          datatypes.birth(locator, raw, false)
          break
        case 'NDATA':
        case 'DDATA':
        case 'NCMD':
        case 'DCMD':
          if (datatypes.fill(locator, raw)) changed = true
          // metrics still untyped refer to a BIRTH this client has not seen
          topicInfo.untypedMetrics = countUntyped(raw.metrics)
          break
      }
      if (normalizeDataSets(raw.metrics)) changed = true
      if (changed) bytes = ProtoPayload.encode(raw).finish()
      return decodePayload(bytes)
    },
    // remember datatypes of births this client publishes (for received commands)
    recordOwnBirth = function (locator, payload, isNodeBirth) {
      try {
        datatypes.birth(
          locator,
          ProtoPayload.decode(encodePayload(payload)),
          isNodeBirth
        )
      } catch (e) {
        logger.warn('Could not record birth datatypes: ' + e.message)
      }
    },
    isSparkplugPayloadMsgType = function (msgType) {
      return (
        msgType === 'NBIRTH' ||
        msgType === 'NDEATH' ||
        msgType === 'NDATA' ||
        msgType === 'NCMD' ||
        msgType === 'DBIRTH' ||
        msgType === 'DDEATH' ||
        msgType === 'DDATA' ||
        msgType === 'DCMD'
      )
    },
    parseSparkplugTopic = function (splitTopic) {
      var deviceId, msgType, msgTypeIndex

      if (splitTopic[0] !== version || splitTopic.length < 4) return null

      for (msgTypeIndex = 2; msgTypeIndex < splitTopic.length; msgTypeIndex++) {
        msgType = splitTopic[msgTypeIndex]
        if (isSparkplugPayloadMsgType(msgType)) break
      }

      if (!isSparkplugPayloadMsgType(msgType)) return null
      if (msgTypeIndex + 1 >= splitTopic.length) return null

      if (msgType[0] === 'D') {
        if (msgTypeIndex + 2 >= splitTopic.length) return null
        deviceId = splitTopic.slice(msgTypeIndex + 2).join('/')
      }

      return {
        namespace: splitTopic[0],
        groupId: splitTopic.slice(1, msgTypeIndex).join('/'),
        msgType: msgType,
        edgeNodeId: splitTopic[msgTypeIndex + 1],
        deviceId: deviceId,
      }
    }

  events.EventEmitter.call(this)

  this.client = null
  this.logger = logger
  this.connecting = false
  this.connected = false

  // Publishes Node BIRTH certificates for the edge node
  this.publishNodeBirth = function (payload, options) {
    var topic = version + '/' + groupId + '/NBIRTH/' + edgeNode
    // Reset sequence number
    seq = 0
    addSeqNumber(payload)
    // Add bdSeq number (must match the bdSeq of the NDEATH will)
    var metrics = payload.metrics
    if (metrics !== undefined && metrics !== null) {
      metrics.push({
        name: 'bdSeq',
        type: 'uint64',
        value: bdSeq,
      })
    }
    recordOwnBirth(groupId + '/' + edgeNode, payload, true)

    logger.info('Publishing Edge Node Birth')
    var p = maybeCompressPayload(payload, options)
    client.publish(topic, encodePayload(p))
    if (logger.level === 'debug') messageAlert('published', topic, p)
  }

  // Publishes Node Data messages for the edge node
  this.publishNodeData = function (payload, options) {
    var topic = version + '/' + groupId + '/NDATA/' + edgeNode
    addSeqNumber(payload)
    logger.info('Publishing NDATA')
    client.publish(topic, encodePayload(maybeCompressPayload(payload, options)))
    if (logger.level === 'debug') messageAlert('published', topic, payload)
  }

  // Publishes Node Command messages (host role: no edge node seq number)
  this.publishNodeCmd = function (group, edge, payload, options, callback) {
    if (typeof options === 'function') {
      callback = options
      options = null
    }

    var topic = version + '/' + group + '/NCMD/' + edge
    logger.info('Publishing NCMD for node ' + edge)
    client.publish(
      topic,
      encodePayload(maybeCompressPayload(payload, options)),
      { qos: 0 },
      callback
    )
    if (logger.level === 'debug') messageAlert('published', topic, payload)
  }

  // Publishes device data
  this.publishDeviceData = function (deviceId, payload, options, pubOptions) {
    var topic = version + '/' + groupId + '/DDATA/' + edgeNode + '/' + deviceId
    addSeqNumber(payload)
    logger.info('Publishing DDATA for device ' + deviceId)
    client.publish(
      topic,
      encodePayload(maybeCompressPayload(payload, options)),
      pubOptions
    )
    if (logger.level === 'debug') messageAlert('published', topic, payload)
  }

  // Publishes device command (host role: no edge node seq number)
  this.publishDeviceCmd = function (
    group,
    edge,
    device,
    payload,
    options,
    callback
  ) {
    if (typeof options === 'function') {
      callback = options
      options = null
    }

    var topic = version + '/' + group + '/DCMD/' + edge + '/' + device
    logger.info('Publishing DCMD for device ' + device)
    client.publish(
      topic,
      encodePayload(maybeCompressPayload(payload, options)),
      { qos: 0 },
      callback
    )
    if (logger.level === 'debug') messageAlert('published', topic, payload)
  }

  // Publishes device BIRTH certificates
  this.publishDeviceBirth = function (deviceId, payload, options) {
    var topic = version + '/' + groupId + '/DBIRTH/' + edgeNode + '/' + deviceId
    addSeqNumber(payload)
    recordOwnBirth(groupId + '/' + edgeNode + '/' + deviceId, payload, false)
    logger.info('Publishing DBIRTH for device ' + deviceId)
    var p = maybeCompressPayload(payload, options),
      bytes = encodePayload(p)
    client.publish(topic, bytes)
    if (logger.level === 'debug') messageAlert('published', topic, p)
    return bytes.length // encoded size, for the application to check limits
  }

  // Publishes device DEATH certificates
  this.publishDeviceDeath = function (deviceId, payload) {
    var topic =
        version + '/' + groupId + '/DDEATH/' + edgeNode + '/' + deviceId,
      options = {}
    addSeqNumber(payload)
    logger.info('Publishing DDEATH for device ' + deviceId)
    client.publish(topic, encodePayload(maybeCompressPayload(payload, options)))
    if (logger.level === 'debug') messageAlert('published', topic, payload)
  }

  // Republishes SCADA HOST STATE ONLINE (the STATE sessions also do it on connect)
  this.publishScadaHostBirth = function () {
    if (stateSessions.length === 0) {
      logger.info(
        'Can not publish SCADA Host Birth as scadaHostId is not defined.'
      )
      return
    }
    stateSessions.forEach(function (s) {
      s.publishOnline()
    })
  }

  this.stop = function () {
    var self = this
    self.connecting = false
    stateSessions.forEach(function (s) {
      s.stop()
    })
    stateSessions = []
    if (!client) return
    // a clean DISCONNECT discards the will, so publish the death explicitly
    if (publishDeath && isEdgeNode && client.connected) {
      var topic = version + '/' + groupId + '/NDEATH/' + edgeNode,
        payload = getDeathPayload()
      logger.info('Publishing Edge Node Death')
      var timer = setTimeout(function () {
        client.end(true)
      }, STOP_TIMEOUT)
      client.publish(topic, encodePayload(payload), { qos: 1 }, function () {
        clearTimeout(timer)
        client.end()
      })
      if (logger.level === 'debug') messageAlert('published', topic, payload)
    } else client.end(true)
  }

  // Configures and connects the client
  return (function (sparkplugClient) {
    var tlsOptions = {
      ...(ca !== '' ? { ca: ca } : {}),
      ...(key !== '' ? { key: key } : {}),
      ...(cert !== '' ? { cert: cert } : {}),
      ...(pfx !== '' ? { pfx: pfx } : {}),
      ...(passphrase !== '' ? { passphrase: passphrase } : {}),
      ...(ciphers !== '' ? { ciphers: ciphers } : {}),
      ...(minVersion !== '' ? { minVersion: minVersion } : {}),
      ...(maxVersion !== '' ? { maxVersion: maxVersion } : {}),
      rejectUnauthorized: rejectUnauthorized,
    }

    var clientOptions = {
      ...(clientId !== '' ? { clientId: clientId } : {}),
      clean: clean,
      keepalive: keepalive,
      reschedulePings: false,
      connectTimeout: connectTimeout,
      // the application recreates the client (and rotates brokers) on failures
      reconnectPeriod: 0,
      username: username,
      password: password,
      ...tlsOptions,
      ...(isEdgeNode ?
        {
          will: {
            topic: version + '/' + groupId + '/NDEATH/' + edgeNode,
            payload: encodePayload(getDeathPayload()),
            qos: 1,
            retain: false,
          },
        }
      : {}),
    }

    sparkplugClient.connecting = true
    if (logger.level === 'debug') {
      logger.debug('Attempting to connect: ' + serverUrl)
      logger.debug(
        '              options: ' +
          JSON.stringify({ ...clientOptions, password: '***' })
      )
    }
    client = mqtt.connect(serverUrl, clientOptions)
    sparkplugClient.client = client

    if (typeof scadaHostId === 'string' && scadaHostId.trim() !== '') {
      var stateOptions = {
        clientId: clientId,
        clean: true,
        keepalive: keepalive,
        connectTimeout: connectTimeout,
        username: username,
        password: password,
        ...tlsOptions,
      }
      stateSessions = ['3.0', '2.2'].map(function (fmt) {
        return new StateSession(
          fmt,
          scadaHostId.trim(),
          version,
          stateOptions,
          serverUrl
        )
      })
    }

    client.on('connect', function () {
      // bdSeq is NOT incremented here: it must match the bdSeq of the NDEATH
      // will message registered at connection time (Sparkplug B spec)
      logger.info('Client has connected')
      sparkplugClient.connecting = false
      sparkplugClient.connected = true
      sparkplugClient.emit('connect')

      if (isEdgeNode) {
        client.subscribe(version + '/' + groupId + '/NCMD/' + edgeNode + '/#', {
          qos: 0,
        })
        client.subscribe(version + '/' + groupId + '/DCMD/' + edgeNode + '/#', {
          qos: 0,
        })
      }

      // Emit the "birth" event to notify the application to send births
      sparkplugClient.emit('birth')
    })

    client.on('error', function (error) {
      if (
        sparkplugClient.connecting &&
        sparkplugClient.listenerCount('error') > 0
      ) {
        sparkplugClient.connecting = false
        sparkplugClient.emit('error', error)
      } else logger.warn('MQTT client error: ' + error.message)
    })

    client.on('close', function () {
      if (sparkplugClient.connected) {
        sparkplugClient.connected = false
        sparkplugClient.emit('close')
      }
    })

    client.on('reconnect', function () {
      sparkplugClient.emit('reconnect')
    })

    client.on('offline', function () {
      sparkplugClient.connected = false
      sparkplugClient.emit('offline')
    })

    function replacer(key, value) {
      if (Array.isArray(value)) {
        if (value.length > 10) return 'Array:' + value.length
      }
      if (typeof value === 'string') {
        if (value.length > 200) return value.substr(0, 200) + '...'
      }
      return value
    }

    client.on('packetsend', function (packet) {
      if (logger.level !== 'debug') return
      logger.debug('packetsend: ' + JSON.stringify(packet, replacer))
    })

    client.on('packetreceive', function (packet) {
      if (logger.level !== 'debug') return
      logger.debug('packetreceive: ' + JSON.stringify(packet, replacer))
    })

    // an exception thrown here (or by an application listener) would stop
    // mqtt.js from processing further packets of the session
    client.on('message', function (topic, message, packet) {
      try {
        onMessage(topic, message, packet)
      } catch (e) {
        logger.error(
          'Error processing message on topic ' + topic + ': ' + (e?.stack || e)
        )
      }
    })

    function onMessage(topic, message, packet) {
      var payload, topicInfo

      var splitTopic = topic.split('/')

      // discard non Sparkplug B payload messages (STATE is not protobuf)
      topicInfo = parseSparkplugTopic(splitTopic)
      if (topicInfo === null) {
        // invalid topics under the Sparkplug B namespace are discarded,
        // except STATE (host online/offline, plain payload)
        if (splitTopic[0] === version && splitTopic[1] !== 'STATE') {
          logger.debug('Discarding invalid Sparkplug B topic: ' + topic)
          return
        }
        sparkplugClient.emit('nonSparkplugMessage', topic, message, packet)
        return
      }

      try {
        payload = decodeSparkplugPayload(message, topicInfo)
      } catch (e) {
        payload = decodeJsonPayload(message)
        if (payload === null) {
          logger.debug(
            'Discarding malformed Sparkplug payload for topic: ' +
              topic +
              ' - ' +
              e.message
          )
          return
        }
      }

      if (logger.level === 'debug') messageAlert('arrived', topic, payload)

      if (
        isEdgeNode &&
        topicInfo.groupId === groupId &&
        topicInfo.msgType === 'NCMD' &&
        topicInfo.edgeNodeId === edgeNode
      ) {
        sparkplugClient.emit('ncmd', payload)
      } else if (
        isEdgeNode &&
        topicInfo.groupId === groupId &&
        topicInfo.msgType === 'DCMD' &&
        topicInfo.edgeNodeId === edgeNode
      ) {
        sparkplugClient.emit('dcmd', topicInfo.deviceId, payload)
      } else if (
        topicInfo.groupId !== groupId ||
        topicInfo.edgeNodeId !== edgeNode
      ) {
        // exclude messages from itself
        sparkplugClient.emit('message', topic, payload, topicInfo)
      }
    }

    return sparkplugClient
  })(this)
}

util.inherits(SparkplugClient, events.EventEmitter)

exports.newClient = function (config) {
  return new SparkplugClient(config)
}
