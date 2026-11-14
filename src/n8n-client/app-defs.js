/*
 * {json:scada} - Copyright (c) 2020-2024 - Ricardo L. Olsen
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

'use strict'

module.exports = {
  NAME: 'N8N',
  ENV_PREFIX: 'JS_N8N_',
  AUTOTAG_PREFIX: 'N8N',
  MSG: '{json:scada} - N8N Integration Driver',
  VERSION: '0.1.1',
  // outbound payload envelope schema tag
  PAYLOAD_SCHEMA: 'jsonscada-n8n/1',
  // key partition per connection for auto-created tags
  AUTOKEY_MULTIPLIER: 100000,
  // inbound HTTP listener defaults
  DEFAULT_BIND_ADDRESS: '0.0.0.0',
  DEFAULT_BIND_PORT: 51930,
  // outbound batching defaults (overridable via connection.options JSON)
  BATCH_MAX_SIZE: 50,
  BATCH_WAIT_MS: 500,
  // outbound retry/backoff (ms)
  RETRY_BASE_MS: 1000,
  RETRY_MAX_MS: 60000,
  // heartbeat push interval (ms), 0 = disabled by default
  HEARTBEAT_MS: 60000,
  // stats flush period (ms)
  STATS_INTERVAL_MS: 10000,
  // outbound queue hard cap (overridable via connection.maxQueueSize)
  MAX_QUEUE_SIZE: 5000,
}
