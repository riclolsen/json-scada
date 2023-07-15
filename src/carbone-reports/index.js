'use strict'

/*
 * Carbone Reports Example
 *
 * {json:scada} - Copyright (c) 2020-2023 - Ricardo L. Olsen
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

const fs = require('fs')
const carbone = require('carbone')
const { MongoClient } = require('mongodb')
const Log = require('./simple-logger')
const { LoadConfig } = require('./load-config')

const jsConfig = LoadConfig() // load and parse config file
Log.levelCurrent = jsConfig.LogLevel
;(async () => {
  await MongoClient.connect(
    // try to (re)connect
    jsConfig.mongoConnectionString,
    jsConfig.MongoConnectionOptions
  )
    .then(async (client) => {
      const db = client.db(jsConfig.mongoDatabaseName)
      const rtCollection = db.collection(jsConfig.RealtimeDataCollectionName)
      let data = await rtCollection
        .find({})
        .project({ _id: true, tag: true, value: true })
        .toArray()
        .catch(function (err) {
          Log.log(err)
        })

      carbone.render('./point_list_template.ods', data, function (err, result) {
        if (err) return console.log(err)
        fs.writeFileSync('point_list.ods', result)

        console.log('End.')
        process.exit(0)
      })
    })
    .catch(function (err) {
      Log.log(err)
    })
})()
