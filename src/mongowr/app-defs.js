'use strict'

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

module.exports = {
  NAME: 'MONGOWR',
  ENV_PREFIX: 'JS_MONGOWR',
  MSG: '{json:scada} - Mongowr - write protocol data forwarded by another JSON-SCADA installation.',
  VERSION: '0.1.0',
  IP_BIND: '0.0.0.0', // IP address for binding to listen UDP messages
  UDP_PORT: 12345, // UDP port to receive packets
  MAX_QUEUE: 5000, // max size for the queue of messages, messages will be discarded after limit exceeded
  MAX_MSG_SEQ: 50, // max messages in sequence to be accumulated for a bulk write
  INTERVAL_AFTER_WRITE: 100, // ms of interval to be respected after a bulk write or empty queue
}
