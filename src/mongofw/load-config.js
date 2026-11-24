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
const { createSecretKey } = require('node:crypto')
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')
const { ReadPreference } = require('mongodb')

// load and parse config file
function LoadConfig(confFileArg, logLevelArg, instArg) {
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

  const env = (name) =>
    process.env[AppDefs.ENV_PREFIX + '_' + name] ??
    process.env[AppDefs.ENV_PREFIX + name]
  const envLogLevel = parseInt(env('LOGLEVEL'), 10)
  const envInstance = parseInt(env('INSTANCE'), 10)
  Log.levelCurrent = Log.levelNormal
  if (Number.isInteger(envLogLevel)) Log.levelCurrent = envLogLevel
  if (Number.isInteger(logLevelArg)) Log.levelCurrent = logLevelArg
  configObj.LogLevel = Log.levelCurrent
  configObj.Instance = instArg || envInstance || 1

  configObj.GridFsCollectionName = 'files'
  configObj.RealtimeDataCollectionName = 'realtimeData'
  configObj.UsersCollectionName = 'users'
  configObj.SoeDataCollectionName = 'soeData'
  configObj.CommandsQueueCollectionName = 'commandsQueue'
  configObj.ProtocolDriverInstancesCollectionName = 'protocolDriverInstances'
  configObj.ProtocolConnectionsCollectionName = 'protocolConnections'
  configObj.ProcessInstancesCollectionName = 'processInstances'
  configObj.GroupSep = '~'
  configObj.ConnectionNumber = 0

  Log.log('Config - ' + AppDefs.MSG + ' Version ' + AppDefs.VERSION)
  Log.log('Config - Instance: ' + configObj.Instance)
  Log.log('Config - Log level: ' + Log.levelCurrent)

  const keyPath = process.env[AppDefs.ENV_PREFIX + '_UDP_KEY_FILE'] ?? configObj.udpAuthKeyFile
  if (typeof keyPath !== 'string' || !keyPath.trim())
    throw new Error('Set udpAuthKeyFile or ' + AppDefs.ENV_PREFIX + '_UDP_KEY_FILE')
  const key = fs.readFileSync(keyPath)
  if (key.length !== 32)
    throw new Error('The UDP authentication key file must contain exactly 32 raw bytes')
  configObj.UdpAuthKey = createSecretKey(key)

  configObj.MongoConnectionOptions = getMongoConnectionOptions(configObj)
  return configObj
}

// prepare mongo connection options
function getMongoConnectionOptions(configObj) {
  let connOptions = {
    //useNewUrlParser: true,
    //useUnifiedTopology: true,
    appname:
      AppDefs.NAME +
      ' Version:' +
      AppDefs.VERSION +
      ' Instance:' +
      configObj.Instance,
    maxPoolSize: 20,
    readPreference: ReadPreference.PRIMARY,
  }

  if (
    typeof configObj.tlsCaPemFile === 'string' &&
    configObj.tlsCaPemFile.trim() !== ''
  ) {
    connOptions.tls = true
    connOptions.tlsCAFile = configObj.tlsCaPemFile
    if (configObj.tlsClientPemFile) {
      connOptions.tlsCertificateKeyFile = configObj.tlsClientPemFile
      if (configObj.tlsClientKeyPassword)
        connOptions.tlsCertificateKeyFilePassword = configObj.tlsClientKeyPassword
    }
    if (configObj.tlsInsecure === true) {
      connOptions.tlsInsecure = true
    } else {
      if (configObj.tlsAllowInvalidHostnames === true)
        connOptions.tlsAllowInvalidHostnames = true
      if (configObj.tlsAllowChainErrors === true)
        connOptions.tlsAllowInvalidCertificates = true
    }
  }

  return connOptions
}

module.exports = LoadConfig
