/*
 * Checks of a command point before a command is queued.
 * {json:scada} - Copyright (c) 2020-2025 - Ricardo L. Olsen
 *
 * Shared by the /Invoke/ realtime API and GraphQL, so every client (AdminUI,
 * scripts, custom developments) meets the same rules as the MCP server and
 * the OPC UA server:
 *  - only points with origin "command" accept commands;
 *  - a command is refused while its point, or the supervised point it acts on
 *    (supervisedOfCommand), is blocked by the operator: commandBlocked true or
 *    a non-empty blocking annotation.
 * Points are read from the primary: a block set a moment ago must be seen.
 */

'use strict'

const { ReadPreference } = require('mongodb')

const COLL_REALTIME = 'realtimeData'
const PRIMARY = { readPreference: ReadPreference.PRIMARY }

// fields of a command point the command path uses
const COMMAND_POINT_FIELDS = {
  _id: 1,
  tag: 1,
  type: 1,
  origin: 1,
  group1: 1,
  description: 1,
  priority: 1,
  eventTextTrue: 1,
  eventTextFalse: 1,
  annotation: 1,
  commandBlocked: 1,
  supervisedOfCommand: 1,
  protocolSourceConnectionNumber: 1,
  protocolSourceCommonAddress: 1,
  protocolSourceObjectAddress: 1,
  protocolSourceASDU: 1,
  protocolSourceCommandDuration: 1,
  protocolSourceCommandUseSBO: 1,
}

const isBlocked = (p) =>
  !!p &&
  (p.commandBlocked === true ||
    (typeof p.annotation === 'string' && p.annotation.trim() !== ''))

// The command point for a query, read from the primary (null when not found).
function findCommandPoint(mdb, query) {
  return mdb
    .collection(COLL_REALTIME)
    .findOne(query, { ...PRIMARY, projection: COMMAND_POINT_FIELDS })
}

// Why a command to this point must be refused, or null when it may be sent.
async function commandRefusal(mdb, point) {
  if (!point || typeof point._id !== 'number') return 'Command point not found!'
  if (point.origin !== 'command')
    return 'Point is not a command point (origin must be "command")!'
  if (isBlocked(point)) return 'Command blocked by the operator (annotation)!'
  const sup = Number(point.supervisedOfCommand)
  if (Number.isFinite(sup) && sup !== 0) {
    const supervised = await mdb
      .collection(COLL_REALTIME)
      .findOne(
        { _id: sup },
        { ...PRIMARY, projection: { annotation: 1, commandBlocked: 1 } }
      )
    if (isBlocked(supervised))
      return 'Command blocked by the operator (annotation of the supervised point)!'
  }
  return null
}

// May this user see a queued command (its status, value and originator)?
// Its originator and admins may; others only when they can see the station
// (group1) of the command point. Special commands (no point: '$$' tags,
// pointKey 0) of others are visible only to users not restricted to some
// stations. Callers answer "not found" when refused, as for an unknown handle.
async function canSeeCommand(mdb, rights, username, cmd) {
  if (!cmd) return false
  if (!rights) return false
  if (rights.isAdmin) return true
  if (typeof cmd.originatorUserName === 'string' && cmd.originatorUserName === username)
    return true
  const g = Array.isArray(rights.group1List) ? rights.group1List : []
  if (g.length === 0) return true
  const key = Number(cmd.pointKey?.valueOf?.() ?? cmd.pointKey)
  if (!Number.isFinite(key) || key === 0) return false
  const point = await mdb
    .collection(COLL_REALTIME)
    .findOne({ _id: key }, { projection: { group1: 1 }, ...PRIMARY })
  return !!point && g.includes(point.group1)
}

module.exports = {
  findCommandPoint,
  commandRefusal,
  canSeeCommand,
  COMMAND_POINT_FIELDS,
}
