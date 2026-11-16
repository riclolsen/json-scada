/*
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

// Pure helpers (no I/O) shared by the driver; covered by sparkplug-utils.test.js

'use strict'

const SparkplugNS = 'spBv1.0'

// test an MQTT topic against a subscription filter (handles + and # wildcards)
function topicMatchSub(topic, sub) {
  if (typeof topic !== 'string' || typeof sub !== 'string') return false
  if (topic === sub) return true
  const t = topic.split('/')
  const s = sub.split('/')
  for (let i = 0; i < s.length; i++) {
    if (s[i] === '#') return i === s.length - 1
    if (i >= t.length) return false
    if (s[i] !== '+' && s[i] !== t[i]) return false
  }
  return t.length === s.length
}

// extract topic and jsonPath from "topic/$.jsonPath"
// E.g. root/topicname/$.var1 -> topic=root/topicname jsonPath=$.var1
// If there is no jsonPath, return the topic and an empty jsonPath
function jsonPathTopic(jpTopic) {
  let jsonPath = ''
  let topic = jpTopic.trim()

  const pos = topic.indexOf('/$.')
  if (pos !== -1) {
    jsonPath = topic.substring(pos + 1)
    topic = topic.substring(0, pos)
  }

  return { topic: topic, jsonPath: jsonPath }
}

// replace characters that are not valid inside a topic level
function topicStr(s) {
  if (typeof s === 'string')
    return s.trim().replace(/\//g, '|').replace(/\+/g, '^').replace(/#/g, '@')
  return ''
}

// path equals base or is below base (base/...), never a sibling sharing a prefix
function isSameOrChildPath(path, base) {
  return path === base || path.startsWith(base + '/')
}

// MongoDB regex for addresses equal to or below a path
function pathRegex(base) {
  return '^' + base.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '(/|$)'
}

// Parse the protocolSourceObjectAddress of a command tag.
// Sparkplug B forms:
//   spBv1.0/group/NCMD/edge/metric          (explicit NCMD)
//   spBv1.0/group/DCMD/edge/device/metric   (explicit DCMD)
//   spBv1.0/group/edge/metric               (NCMD)
//   spBv1.0/group/edge/Node Control/...     (NCMD, standard node control)
//   spBv1.0/group/edge/device/metric        (DCMD)
// Metric names may contain slashes. Anything else is a plain MQTT topic.
function parseCommandAddress(address) {
  if (typeof address !== 'string' || address.trim() === '') return null
  const p = address.split('/')
  if (p[0] !== SparkplugNS) return { topic: address }

  const valid = (...parts) =>
    parts.every((x) => typeof x === 'string' && x !== '')
  let r = null
  if (p[2] === 'NCMD' && p.length >= 5) {
    r = { groupId: p[1], edgeNodeId: p[3], metricName: p.slice(4).join('/') }
  } else if (p[2] === 'DCMD' && p.length >= 6) {
    r = {
      groupId: p[1],
      edgeNodeId: p[3],
      deviceId: p[4],
      metricName: p.slice(5).join('/'),
    }
  } else if (p.length === 4) {
    r = { groupId: p[1], edgeNodeId: p[2], metricName: p[3] }
  } else if (p.length >= 5 && p[3] === 'Node Control') {
    r = { groupId: p[1], edgeNodeId: p[2], metricName: p.slice(3).join('/') }
  } else if (p.length >= 5) {
    r = {
      groupId: p[1],
      edgeNodeId: p[2],
      deviceId: p[3],
      metricName: p.slice(4).join('/'),
    }
  }
  if (!r || !valid(r.groupId, r.edgeNodeId, r.metricName)) return null
  if ('deviceId' in r && !valid(r.deviceId)) return null
  return r
}

// number from a number, numeric string or Long
function toNumber(v) {
  if (v !== null && typeof v === 'object' && typeof v.toNumber === 'function')
    return v.toNumber()
  return Number(v)
}

// Sparkplug sequence numbers (seq, bdSeq) wrap at 256
function nextSeq(seq) {
  return (toNumber(seq) + 1) % 256
}

module.exports = {
  SparkplugNS,
  topicMatchSub,
  jsonPathTopic,
  topicStr,
  isSameOrChildPath,
  pathRegex,
  parseCommandAddress,
  toNumber,
  nextSeq,
}
