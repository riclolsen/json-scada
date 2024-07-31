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

const fs = require('fs')
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')
const { ReadPreference } = require('mongodb')

// load and parse config file
function LoadConfig () {
  const args = process.argv.slice(2)

  var confFileArg = null
  if (args.length > 2) confFileArg = args[2]

  let configFile =
    confFileArg || process.env.JS_CONFIG_FILE || '../../conf/json-scada.json'
  Log.log('Config - Config File: ' + configFile)

  if (!fs.existsSync(configFile)) {
    Log.log('Config - Error: config file not found!')
    process.exit()
  }

  let rawFileContents = fs.readFileSync(configFile)
  let configObj = JSON.parse(rawFileContents)
  if (
    typeof configObj.mongoConnectionString != 'string' ||
    configObj.mongoConnectionString === ''
  ) {
    Log.log('Error reading config file.')
    process.exit()
  }

  Log.levelCurrent = Log.levelNormal
  if (AppDefs.ENV_PREFIX + 'LOGLEVEL' in process.env)
    Log.levelCurrent = parseInt(process.env[AppDefs.ENV_PREFIX + 'LOGLEVEL'])
  if (args.length > 1) Log.levelCurrent = parseInt(args[1])
  configObj.LogLevel = Log.levelCurrent

  var instArg = null
  if (args.length > 0) instArg = parseInt(args[0])
  configObj.Instance =
    instArg || parseInt(process.env[AppDefs.ENV_PREFIX + 'INSTANCE']) || 1

  configObj.GridFsCollectionName = 'files'
  configObj.RealtimeDataCollectionName = 'realtimeData'
  configObj.SoeDataCollectionName = 'soeData'
  configObj.CommandsQueueCollectionName = 'commandsQueue'
  configObj.ProtocolDriverInstancesCollectionName = 'protocolDriverInstances'
  configObj.ProtocolConnectionsCollectionName = 'protocolConnections'
  configObj.GroupSep = '~'
  configObj.ConnectionNumber = 0

  Log.log('Config - ' + AppDefs.MSG + ' Version ' + AppDefs.VERSION)
  Log.log('Config - Instance: ' + configObj.Instance)
  Log.log('Config - Log level: ' + Log.levelCurrent)

  return configObj
}

// prepare mongo connection options
function getMongoConnectionOptions (configObj) {
  let connOptions = {
    // useNewUrlParser: true,
    // useUnifiedTopology: true,
    appname:
      AppDefs.NAME +
      ' Version:' +
      AppDefs.VERSION +
      ' Instance:' +
      configObj.Instance,
    maxPoolSize: 20,
    readPreference: ReadPreference.PRIMARY
  }

  if (
    typeof configObj.tlsCaPemFile === 'string' &&
    configObj.tlsCaPemFile.trim() !== ''
  ) {
    configObj.tlsClientKeyPassword = configObj.tlsClientKeyPassword || ''
    configObj.tlsAllowInvalidHostnames =
      configObj.tlsAllowInvalidHostnames || false
    configObj.tlsAllowChainErrors = configObj.tlsAllowChainErrors || false
    configObj.tlsInsecure = configObj.tlsInsecure || false

    connOptions.tls = true
    connOptions.tlsCAFile = configObj.tlsCaPemFile
    connOptions.tlsCertificateKeyFile = configObj.tlsClientPemFile
    connOptions.tlsCertificateKeyFilePassword = configObj.tlsClientKeyPassword
    connOptions.tlsAllowInvalidHostnames = configObj.tlsAllowInvalidHostnames
    connOptions.tlsInsecure = configObj.tlsInsecure
  }

  return connOptions
}

module.exports = { LoadConfig, getMongoConnectionOptions }
