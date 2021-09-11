'use strict'

/*
 * Command line tool to create backup scripts for later restore.
 *
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

const Log = require('./simple-logger.js')
const MongoClient = require('mongodb').MongoClient
const { LoadConfig, getMongoConnectionOptions } = require('./load-config')
const fs = require('fs')
const OutputDir = 'output'

;(async () => {
  const args = process.argv.slice(2)

  if (args.length > 2) {
    console.log(
      'This tool can be used to create JSON-SCADA backup scripts (javascript files for mongo/mongosh tool).'
    )
    console.log(
      'Usage: ' +
        process.argv[0] +
        ' ' +
        process.argv[1] +
        ' [config file name]'
    )
    return
  }

  let configFileArg = null
  if (args.length >= 1) configFileArg = args[0]
  
  if (!fs.existsSync(OutputDir))
    fs.mkdirSync(OutputDir)

  const jsConfig = LoadConfig(configFileArg) // load and parse config file

  // connect to mongodb
  MongoClient.connect(
    jsConfig.mongoConnectionString,
    getMongoConnectionOptions(jsConfig)
  ).then(async client => {
    // connected

    console.log('Connected to MongoDB.')

    const db = client.db(jsConfig.mongoDatabaseName)

    let usersCollection = db.collection(jsConfig.UsersCollectionName)
    let res = await usersCollection.find({}).toArray()
    if (res) {
      let fd = fs.openSync(OutputDir + '/' + jsConfig.UsersCollectionName + '_insert.js','w')
      Log.log('Collection '+jsConfig.UsersCollectionName)
      res.forEach(element => {
        Log.log(element.username)
        fs.writeSync(fd,'db.'+jsConfig.UsersCollectionName+'.insertOne(')
        fs.writeSync(fd,JSON.stringify(element))
        fs.writeSync(fd,');\n')
      })
      fs.close(fd)
    }

    let rtDataCollection = db.collection(jsConfig.RealtimeDataCollectionName)
    res = await rtDataCollection.find({}).toArray()
    if (res) {
      let fd = fs.openSync(OutputDir + '/' + jsConfig.RealtimeDataCollectionName + '_insert.js','w')
      Log.log('Collection '+jsConfig.RealtimeDataCollectionName)
      res.forEach(element => {
        Log.log(element.tag)
        fs.writeSync(fd,'db.'+jsConfig.RealtimeDataCollectionName+'.insertOne(')
        fs.writeSync(fd,JSON.stringify(element))
        fs.writeSync(fd,');\n')
      })
      fs.close(fd)
    }

    let connectionsCollection = db.collection(jsConfig.ProtocolConnectionsCollectionName)
    res = await connectionsCollection.find({}).toArray()
    if (res) {
      let fd = fs.openSync(OutputDir + '/' + jsConfig.ProtocolConnectionsCollectionName + '_insert.js','w')
      Log.log('Collection '+jsConfig.ProtocolConnectionsCollectionName)
      res.forEach(element => {
        Log.log(element.name)
        fs.writeSync(fd,'db.'+jsConfig.ProtocolConnectionsCollectionName+'.insertOne(')
        fs.writeSync(fd,JSON.stringify(element))
        fs.writeSync(fd,');\n')
      })
      fs.close(fd)
    }

    process.exit()
  })
//  process.exit()
})()
