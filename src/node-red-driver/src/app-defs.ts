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

export const AppDefs = {
  NAME: 'NODE-RED',
  ENV_PREFIX: 'JS_NODERED_',
  AUTOTAG_PREFIX: 'NODERED',
  MSG: '{json:scada} - Node-RED Integration Driver',
  VERSION: '0.1.1',
  // WebSocket JSON protocol version advertised in helloAck; the contrib package
  // checks this and refuses to run against an incompatible driver.
  PROTOCOL_VERSION: 1,

  // Default WS server bind when the connection doc omits ipAddressLocalBind.
  DEFAULT_BIND_ADDRESS: '0.0.0.0',
  DEFAULT_BIND_PORT: 51931,

  // Key allocation for auto-created tags: _id = connectionNumber*MULT + seq.
  // Same convention as telegraf-listener / DNP3 / IEC autoCreateTags.
  AUTO_KEY_MULTIPLIER: 100000,

  // Ingest (updates -> sourceDataUpdate) Mongo bulk cycle period, ms.
  INGEST_CYCLE_MS: 500,

  // Distribution (change stream -> WS) per-client batch flush.
  PUBLISH_FLUSH_MS: 200,
  PUBLISH_FLUSH_MAX: 500,

  // Per-client outbound queue cap; on overflow drop-oldest + notify + force snapshot.
  MAX_CLIENT_QUEUE: 10000,

  // App-level WS keep-alive / dead-peer reaping.
  WS_PING_INTERVAL_MS: 20000,

  // Discard commands older than this (staleness guard), ms.
  COMMAND_MAX_AGE_MS: 10000,

  // Redundancy keep-alive check period, ms.
  REDUNDANCY_INTERVAL_MS: 5000,

  // Value size caps (defensive).
  MAX_ADDRESS_LEN: 512,
  MAX_VALUE_JSON_BYTES: 16 * 1024,
} as const

export default AppDefs
