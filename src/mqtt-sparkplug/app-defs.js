'use strict'

/*
 * {json:scada} - Copyright (c) 2020-2021 - Ricardo L. Olsen
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
  NAME: 'MQTT-SPARKPLUG-B',
  ENV_PREFIX: 'JS_MQTTSPB_',
  AUTOTAG_PREFIX: 'MQTT',
  MSG: '{json:scada} - MQTT-Sparkplug-B Client Driver',
  VERSION: '0.1.2',
  MAX_QUEUEDMETRICS: 10000,
  SPARKPLUG_PUBLISH_INTERVAL: 777,
  SPARKPLUG_COMPRESS_DBIRTH: false,
  SPARKPLUG_COMPRESS_NBIRTH: false,
  SPARKPLUG_COMPRESS_DDATA: false,
  SPARKPLUG_COMPRESS_NDATA: false,
  TAGS_SUBTOPIC: '@json-scada/tags',
  MQTT_CLEAN_SESSION: true,
  MQTT_CONNECTION_TIMEOUT: 30,
  MQTT_CONNECTION_KEEPALIVE: 15,
}
