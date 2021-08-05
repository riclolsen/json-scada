'use strict'

/*
 * Command line tool to create and update user and password.
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

const MongoClient = require('mongodb').MongoClient
const bcrypt = require('bcryptjs')
const { LoadConfig, getMongoConnectionOptions } = require('./load-config')

;(async () => {
  const args = process.argv.slice(2)

  if (args.length < 2) {
    console.log('This tool can be used to create or update user and password.')
    console.log(
      'Usage: ' +
        process.argv[0] +
        ' ' +
        process.argv[1] +
        ' username password [email] [config file name]'
    )
    return
  }

  const username = args[0]
  const password = args[1]
  let email = ''
  if (args.length >= 3) email = args[2]
  const hashedPassword = bcrypt.hashSync(password, 8)

  let configFileArg = null
  if (args.length >= 4) configFileArg = args[3]

  const jsConfig = LoadConfig(configFileArg) // load and parse config file

  // connect to mongodb
  await MongoClient.connect(
    jsConfig.mongoConnectionString,
    getMongoConnectionOptions(jsConfig, MongoClient)
  ).then(async client => {
    // connected

    console.log('Connected to MongoDB.')

    const db = client.db(jsConfig.mongoDatabaseName)
    let usersCollection = db.collection('users')

    let res = await usersCollection.findOne({ username: username })
    if (res) {
      if (res.username == username) {
        console.log('User found, updating password...')
        res = await usersCollection.updateOne(
          { username: username },
          {
            $set: {
              password: hashedPassword,
              ...(email != '' ? { email: email } : {})
            }
          }
        )
        if (res.matchedCount === 1) console.log('User updated successfully.')
        else console.log('Error updating user!')
      } else {
        console.log('Unexpected error querying user!')
      }
    } else {
      console.log('User not found, creating user...')
      res = await usersCollection.insertOne({
        username: username,
        password: hashedPassword,
        email: email,
        roles: []
      })
      if ('insertedId' in res) {
        console.log('User created successfully.')
        console.log(
          'An administrator must assign a role to the new user so that he can possibly login!'
        )
      } else console.log('Error creating user!')
    }
  })
  process.exit()
})()

