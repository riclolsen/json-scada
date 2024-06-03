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
  NAME: 'MONGOFW',
  ENV_PREFIX: 'JS_MONGOFW',
  MSG: '{json:scada} - Mongofw - forward data from protocol drivers to another JSON-SCADA installation.',
  VERSION: '0.1.0',
  IP_DESTINATION: '255.255.255.255', // IP address for the destination of packets 
  UDP_PORT: 12345, // UDP port to send packets
  PACKETS_INTERVAL: 1, // ms of minimal interval between packets
  MAX_LENGTH_JSON: 60000, // max size of serialized JSON (before compression) that can be forwarded on a UDP message
  PACKET_SIZE_THRESHOLD: 7000, // limit of serialized JSON to break array of objects to the next UDP message
  INTERVAL_AFTER_EMPTY_QUEUE: 250, // ms of interval respected after queue of changes emptied
  MAX_SEQUENCE_OF_UDPMSGS: 50, // max number of packets in fast sequence
  INTERVAL_AFTER_UDPMSGS_SEQ: 100, // min interval after MAX_SEQUENCE_OF_UDPMSGS exceeded
  PACKET_SIZE_BREAK_SEQ: 6000, // packets large than this limit will break a fast sequence
  INTERVAL_INTEGRITY: 60*60, // interval in seconds for point database integrity 
  BACKFILL_EXPIRATION: 7, // time to preserve backfill data in days
  BACKFILL_REPLAY_INTERVAL: 3, // time to auto replay backfill data in days
  BACKFILL_DOCS_PER_SEC: 500, // max number of documents per second to replay backfill data
}
