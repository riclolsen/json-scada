'use strict'

/*
 * A realtime point data HTTP web server for JSON SCADA.
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

let AUTHENTICATION = process.env.JS_AUTHENTICATION === 'NOAUTH' ? false : true
const IP_BIND = process.env.JS_IP_BIND || 'localhost'
const HTTP_PORT = process.env.JS_HTTP_PORT || 8080
const GRAFANA_SERVER = process.env.JS_GRAFANA_SERVER || 'http://127.0.0.1:3000'
const OPCAPI_AP = '/Invoke/' // mimic of webhmi from OPC reference app https://github.com/OPCFoundation/UA-.NETStandard/tree/demo/webapi/SampleApplications/Workshop/Reference
const API_AP = '/server_realtime'
const APP_NAME = ':' + HTTP_PORT + API_AP
const COLL_REALTIME = 'realtimeData'
const COLL_SOE = 'soeData'
const COLL_COMMANDS = 'commandsQueue'
const COLL_ACTIONS = 'userActions'
const jsConfigFile = process.env.JS_CONFIG_FILE || '../../conf/json-scada.json'
const express = require('express')
const httpProxy = require('express-http-proxy')
const path = require('path')
const cors = require('cors')
const app = express()
const bodyParser = require('body-parser')
var cookieParser = require('cookie-parser')
const fs = require('fs')
const mongo = require('mongodb')
const MongoClient = require('mongodb').MongoClient
const opc = require('./opc_codes.js')
const { Pool } = require('pg')
const UserActionsQueue = require('./userActionsQueue')

const config = require('./app/config/auth.config.js')
if (process.env.JS_JWT_SECRET) config.secret = process.env.JS_JWT_SECRET

const dbAuth = require('./app/models')
const { authJwt } = require('./app/middlewares')
const { AsyncLocalStorage } = require('async_hooks')
const { canSendCommands } = require('./app/middlewares/authJwt.js')

// Argument NOAUTH disables user authentication
var args = process.argv.slice(2)
if (args.length > 0) if (args[0] === 'NOAUTH') AUTHENTICATION = false

const opcIdTypeNumber = 0
const opcIdTypeString = 1
const beepPointKey = -1

let rawFileContents = fs.readFileSync(jsConfigFile)
let jsConfig = JSON.parse(rawFileContents)
if (
  typeof jsConfig.mongoConnectionString != 'string' ||
  jsConfig.mongoConnectionString === ''
) {
  console.log('Error reading config file.')
  process.exit(-1)
}

if (AUTHENTICATION) {
  // JWT Auth Mongo Express https://bezkoder.com/node-js-mongodb-auth-jwt/
  dbAuth.mongoose
    .connect(jsConfig.mongoConnectionString, {
      useNewUrlParser: true,
      useUnifiedTopology: true,
      useFindAndModify: false
    })
    .then(() => {
      console.log('Successfully connect to MongoDB (auth).')
      // initial();
    })
    .catch(err => {
      console.error('Connection error', err)
      process.exit()
    })

  app.use(cookieParser())
  app.options(OPCAPI_AP, cors()) // enable pre-flight request
  app.use(bodyParser.json())
  app.use(
    bodyParser.urlencoded({
      extended: true
    })
  )
} else {
  console.log('******* DISABLED AUTHENTICATION! ********')

  app.use(express.static('../htdocs')) // serve static files
  app.options(OPCAPI_AP, cors()) // enable pre-flight request
  app.use(bodyParser.json())
  app.use(
    bodyParser.urlencoded({
      extended: true
    })
  )
  // Here we serve up the index page
  app.get('/', function (req, res) {
    res.sendFile(path.join(__dirname + '../htdocs/index.html'))
  })
}

// reverse proxy for grafana
app.use('/grafana', httpProxy(GRAFANA_SERVER))

let db = null
let clientMongo = null
let pool = null

;(async () => {
  app.listen(HTTP_PORT, IP_BIND, () => {
    console.log('listening on ' + HTTP_PORT)
  })

  // if env variables defined use them, if not set local defaults
  let pgopt = {}
  if ('PGHOST' in process.env || 'PGHOSTADDR' in process.env) pgopt = null
  else
    pgopt = {
      host: '127.0.0.1',
      database: 'json_scada',
      user: 'json_scada',
      password: 'json_scada',
      port: 5432
    }

  if (pool == null) {
    pool = new Pool(pgopt)
    pool.on('error', (err, client) => {
      console.error(err)
      setTimeout(() => {
        pool = null
      }, 5000)
    })
  }

  if (AUTHENTICATION) {
    require('./app/routes/auth.routes')(app, OPCAPI_AP)
    require('./app/routes/user.routes')(app, OPCAPI_AP, opcApi)
  } else {
    app.post(OPCAPI_AP, opcApi)
  }

  // OPC WEB HMI API
  async function opcApi (req, res) {
    let tini = new Date().getTime()

    res.setHeader('Access-Control-Allow-Origin', '*')
    res.setHeader('Content-Type', 'application/json')

    let ServiceId,
      RequestHandle,
      Timestamp = new Date().toISOString()

    if ('ServiceId' in req.body) {
      ServiceId = req.body.ServiceId
      if ('Body' in req.body) {
        if ('RequestHeader' in req.body.Body) {
          if ('RequestHandle' in req.body.Body.RequestHeader) {
            RequestHandle = req.body.Body.RequestHeader.RequestHandle
          }
        }
      }
    }

    let OpcResp = {
      NamespaceUris: [
        'urn:opcf-apps-01:UA:Quickstarts:ReferenceServer',
        'http://opcfoundation.org/Quickstarts/ReferenceApplications',
        'http://opcfoundation.org/UA/Diagnostics'
      ],
      ServerUris: [],
      ServiceId: ServiceId,
      Body: {
        ResponseHeader: {
          RequestHandle: RequestHandle,
          Timestamp: Timestamp,
          ServiceDiagnostics: {
            LocalizedText: 0
          },
          StringTable: []
        }
        //, "DiagnosticInfos": []
      }
    }

    let username = 'unknown'
    let userRights = {}

    // handle auth here
    if (AUTHENTICATION) {
      let rslt = authJwt.checkToken(req)
      // console.log(rslt)
      if (rslt === false) {
        // fail if not connected to database server
        OpcResp.ServiceId = opc.ServiceCode.ServiceFault
        OpcResp.Body.ResponseHeader.ServiceResult =
          opc.StatusCode.BadIdentityTokenRejected
        OpcResp.Body.ResponseHeader.StringTable = [
          opc.getStatusCodeName(opc.StatusCode.BadIdentityTokenRejected),
          opc.getStatusCodeText(opc.StatusCode.BadIdentityTokenRejected),
          'Access denied (absent or invalid access token)!'
        ]
        res.send(OpcResp)
        return
      } else {
        if ('username' in rslt) username = rslt.username
        if ('rights' in rslt) userRights = rslt.rights
      }
    }

    if (!clientMongo || !clientMongo.isConnected()) {
      // fail if not connected to database server
      OpcResp.ServiceId = opc.ServiceCode.ServiceFault
      OpcResp.Body.ResponseHeader.ServiceResult =
        opc.StatusCode.BadServerNotConnected
      OpcResp.Body.ResponseHeader.StringTable = [
        opc.getStatusCodeName(opc.StatusCode.BadServerNotConnected),
        opc.getStatusCodeText(opc.StatusCode.BadServerNotConnected),
        'Database disconnected!'
      ]
      res.send(OpcResp)
      return
    }

    if (ServiceId === undefined) {
      // fail if no service id defined
      OpcResp.ServiceId = opc.ServiceCode.ServiceFault
      OpcResp.Body.ResponseHeader.ServiceResult =
        opc.StatusCode.BadRequestHeaderInvalid
      OpcResp.Body.ResponseHeader.StringTable = [
        opc.getStatusCodeName(opc.StatusCode.BadRequestHeaderInvalid),
        opc.getStatusCodeText(opc.StatusCode.BadRequestHeaderInvalid),
        'No ServiceID'
      ]
      res.send(OpcResp)
      return
    }

    if (RequestHandle === undefined) {
      // fail if not defined a request handle
      OpcResp.ServiceId = opc.ServiceCode.ServiceFault
      OpcResp.Body.ResponseHeader.ServiceResult =
        opc.StatusCode.BadRequestHeaderInvalid
      OpcResp.Body.ResponseHeader.StringTable = [
        opc.getStatusCodeName(opc.StatusCode.BadRequestHeaderInvalid),
        opc.getStatusCodeText(opc.StatusCode.BadRequestHeaderInvalid),
        'No RequestHandle'
      ]
      res.send(OpcResp)
      return
    }

    switch (req.body.ServiceId) {
      case opc.ServiceCode.WriteRequest: // WRITE SERVICE
        {
          OpcResp.ServiceId = opc.ServiceCode.WriteResponse
          if (!('NodesToWrite' in req.body.Body)) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.GoodNoData
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.GoodNoData),
              opc.getStatusCodeText(opc.StatusCode.GoodNoData),
              'No NodesToWrite'
            ]
            res.send(OpcResp)
            return
          }
          OpcResp.Body.ResponseHeader.ServiceResult = opc.StatusCode.Good
          OpcResp.Body.Results = []
          OpcResp.Body._CommandHandles = []
          for (let i = 0; i < req.body.Body.NodesToWrite.length; i++) {
            let node = req.body.Body.NodesToWrite[i]
            if ('AttributeId' in node) {
              if (node.AttributeId == opc.AttributeId.ExtendedAlarmEventsAck) {
                // alarm / event ack

                if (AUTHENTICATION) {
                  // test for user rights
                  if (
                    !userRights?.ackEvents &&
                    (node.Value.Body & opc.Acknowledge.RemoveAllEvents ||
                      node.Value.Body & opc.Acknowledge.RemoveOneEvent ||
                      node.Value.Body & opc.Acknowledge.RemovePointEvents ||
                      node.Value.Body & opc.Acknowledge.AckAllEvents ||
                      node.Value.Body & opc.Acknowledge.AckOneEvent ||
                      node.Value.Body & opc.Acknowledge.AckPointEvents)
                  ) {
                    // ACTION DENIED
                    console.log(
                      `User has no right to ack/remove events! [${username}]`
                    )
                    OpcResp.Body.ResponseHeader.ServiceResult =
                      opc.StatusCode.BadUserAccessDenied
                    OpcResp.Body.ResponseHeader.StringTable = [
                      opc.getStatusCodeName(opc.StatusCode.BadUserAccessDenied),
                      opc.getStatusCodeText(opc.StatusCode.BadUserAccessDenied),
                      'User has no right to ack/remove events!'
                    ]
                    res.send(OpcResp)
                    return
                  }
                  if (
                    !userRights?.ackAlarms &&
                    (node.Value.Body & opc.Acknowledge.AckOneAlarm ||
                      node.Value.Body & opc.Acknowledge.AckAllAlarms ||
                      node.Value.Body & opc.Acknowledge.SilenceBeep)
                  ) {
                    // ACTION DENIED
                    console.log(
                      `User has no right to ack or silence alarms! [${username}]`
                    )
                    OpcResp.Body.ResponseHeader.ServiceResult =
                      opc.StatusCode.BadUserAccessDenied
                    OpcResp.Body.ResponseHeader.StringTable = [
                      opc.getStatusCodeName(opc.StatusCode.BadUserAccessDenied),
                      opc.getStatusCodeText(opc.StatusCode.BadUserAccessDenied),
                      'User has no right to ack or silence alarms!'
                    ]
                    res.send(OpcResp)
                    return
                  }
                }

                let findPoint = null
                if (node.NodeId.IdType === opcIdTypeNumber) {
                  findPoint = { _id: parseInt(node.NodeId.Id) }
                } else if (node.NodeId.IdType === opcIdTypeString) {
                  findPoint = { tag: node.NodeId.Id }
                }

                if (node.Value.Body & opc.Acknowledge.RemoveAllEvents) {
                  console.log('Remove All Events')
                  let result = await db.collection(COLL_SOE).updateMany(
                    { ack: { $lte: 1 } },
                    {
                      $set: {
                        ack: 2
                      }
                    }
                  )
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Remove All Events',
                    timeTag: new Date()
                  })
                } else if (node.Value.Body & opc.Acknowledge.AckAllEvents) {
                  console.log('Ack All Events')
                  let result = await db.collection(COLL_SOE).updateMany(
                    { ack: 0 },
                    {
                      $set: {
                        ack: 1
                      }
                    }
                  )
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Ack All Events',
                    timeTag: new Date()
                  })
                } else if (
                  node.Value.Body & opc.Acknowledge.RemovePointEvents
                ) {
                  console.log('Remove Point Events: ' + node.NodeId.Id)
                  let result = await db.collection(COLL_SOE).updateMany(
                    { tag: node.NodeId.Id, ack: { $lte: 1 } },
                    {
                      $set: {
                        ack: 2
                      }
                    }
                  )
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Remove Point Events',
                    tag: node.NodeId.Id,
                    timeTag: new Date()
                  })
                } else if (node.Value.Body & opc.Acknowledge.AckPointEvents) {
                  console.log('Ack Point Events: ' + node.NodeId.Id)
                  let result = await db.collection(COLL_SOE).updateMany(
                    { tag: node.NodeId.Id, ack: 0 },
                    {
                      $set: {
                        ack: 1
                      }
                    }
                  )
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Ack Point Events',
                    tag: node.NodeId.Id,
                    timeTag: new Date()
                  })
                } else if (node.Value.Body & opc.Acknowledge.RemoveOneEvent) {
                  console.log('Remove One Event: ' + node.NodeId.Id)
                  let result = await db.collection(COLL_SOE).updateMany(
                    {
                      _id: new mongo.ObjectID(node._Properties.event_id),
                      ack: { $lte: 1 }
                    },
                    {
                      $set: {
                        ack: 2
                      }
                    }
                  )
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Remove One Event',
                    tag: node.NodeId.Id,
                    eventId: new mongo.ObjectID(node._Properties.event_id),
                    timeTag: new Date()
                  })
                } else if (node.Value.Body & opc.Acknowledge.AckOneEvent) {
                  console.log('Ack One Event: ' + node.NodeId.Id)
                  let result = await db.collection(COLL_SOE).updateMany(
                    {
                      _id: new mongo.ObjectID(node._Properties.event_id),
                      ack: 0
                    },
                    {
                      $set: {
                        ack: 1
                      }
                    }
                  )
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Ack One Event',
                    tag: node.NodeId.Id,
                    eventId: new mongo.ObjectID(node._Properties.event_id),
                    timeTag: new Date()
                  })
                }

                if (node.Value.Body & opc.Acknowledge.AckAllAlarms) {
                  console.log('Ack All Alarms')
                  let result = await db.collection(COLL_REALTIME).updateMany(
                    {},
                    {
                      $set: {
                        alarmed: false
                      }
                    }
                  )
                  result = await db
                    .collection(COLL_REALTIME)
                    .updateMany({ isEvent: true }, [
                      {
                        $set: {
                          value: 0,
                          valueString: '$stateTextFalse',
                          timeTagAtSource: null,
                          TimeTagAtSourceOk: null
                        }
                      }
                    ])
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Ack All Alarms',
                    timeTag: new Date()
                  })
                } else if (node.Value.Body & opc.Acknowledge.AckOneAlarm) {
                  console.log('Ack alarm: ' + node.NodeId.Id)
                  let result = await db
                    .collection(COLL_REALTIME)
                    .updateOne(findPoint, {
                      $set: {
                        alarmed: false
                      }
                    })
                  findPoint['isEvent'] = true
                  result = await db
                    .collection(COLL_REALTIME)
                    .updateOne(findPoint, [
                      {
                        $set: {
                          value: 0,
                          valueString: '$stateTextFalse',
                          timeTagAtSource: null,
                          TimeTagAtSourceOk: null
                        }
                      }
                    ])
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Ack Point Alarm',
                    pointKey: node.NodeId.Id,
                    timeTag: new Date()
                  })
                }
                if (node.Value.Body & opc.Acknowledge.SilenceBeep) {
                  console.log('Silence Beep')
                  let result = await db.collection(COLL_REALTIME).updateOne(
                    { _id: beepPointKey },
                    {
                      $set: {
                        value: new mongo.Double(0),
                        valueString: '0',
                        beepType: new mongo.Double(0)
                      }
                    }
                  )
                  UserActionsQueue.enqueue({
                    username: username,
                    action: 'Silence Beep',
                    timeTag: new Date()
                  })
                }

                OpcResp.ServiceId = opc.ServiceCode.WriteResponse
                OpcResp.Body.ResponseHeader.ServiceResult =
                  opc.StatusCode.GoodNoData
                OpcResp.Body.ResponseHeader.StringTable = [
                  opc.getStatusCodeName(opc.StatusCode.GoodNoData),
                  opc.getStatusCodeText(opc.StatusCode.GoodNoData),
                  'Ok, no data returned. Query time: ' +
                    (new Date().getTime() - tini) +
                    ' ms'
                ]
                res.send(OpcResp)
                console.log(
                  'Write. Elapsed ' + (new Date().getTime() - tini) + ' ms'
                )
                return
              }

              if (node.AttributeId == opc.AttributeId.Value) {
                // Write a Value: Command

                if (AUTHENTICATION) {
                  // go check user right for command in mongodb (not just in the token for better security)
                  if (!(await canSendCommands(req))) {
                    // ACTION DENIED
                    console.log(
                      `User has no right to issue commands! [${username}]`
                    )
                    OpcResp.Body.ResponseHeader.ServiceResult =
                      opc.StatusCode.BadUserAccessDenied
                    OpcResp.Body.ResponseHeader.StringTable = [
                      opc.getStatusCodeName(opc.StatusCode.BadUserAccessDenied),
                      opc.getStatusCodeText(opc.StatusCode.BadUserAccessDenied),
                      'User has no right to issue commands!'
                    ]
                    res.send(OpcResp)
                    return
                  } else {
                    console.log(
                      `User authorized to issue commands! [${username}]`
                    )
                  }
                }

                if ('NodeId' in node)
                  if ('Id' in node.NodeId) {
                    if (
                      typeof node.Value !== 'object' ||
                      typeof node.Value.Type !== 'number' ||
                      node.Value.Type !== opc.DataType.Double || // only accepts a double value for the command
                      typeof node.Value.Body !== 'number'
                    ) {
                      OpcResp.Body.ResponseHeader.ServiceResult =
                        opc.StatusCode.BadNodeAttributesInvalid
                      OpcResp.Body.ResponseHeader.StringTable = [
                        opc.getStatusCodeName(
                          opc.StatusCode.BadNodeAttributesInvalid
                        ),
                        opc.getStatusCodeText(
                          opc.StatusCode.BadNodeAttributesInvalid
                        ),
                        'Invalid command type, malformed or missing information!'
                      ]
                      res.send(OpcResp)
                      return
                    }

                    // look for the the command info in the database

                    let cmd_id = node.NodeId.Id
                    let cmd_val = node.Value.Body
                    let query = { _id: parseInt(cmd_id) }
                    if (isNaN(parseInt(cmd_id))) query = { tag: cmd_id }

                    // search command in database (wait for results)
                    let data = await db.collection(COLL_REALTIME).findOne(query)

                    if (data === null || typeof data._id !== 'number') {
                      // command not found, abort
                      console.log('Command not found!')
                      OpcResp.Body.ResponseHeader.ServiceResult =
                        opc.StatusCode.BadNodeIdUnknown
                      OpcResp.Body.ResponseHeader.StringTable = [
                        opc.getStatusCodeName(opc.StatusCode.BadNodeIdUnknown),
                        opc.getStatusCodeText(opc.StatusCode.BadNodeIdUnknown),
                        'Command point not found!'
                      ]
                      res.send(OpcResp)
                      return
                    }

                    if (AUTHENTICATION) {
                      // check if user has group1 list it can command
                      if (!(await canSendCommandTo(req, data.group1))) {
                        // ACTION DENIED
                        console.log(
                          `User has no right to issue commands to the group1 destination! [${username}] [${data.group1}]`
                        )
                        OpcResp.Body.ResponseHeader.ServiceResult =
                          opc.StatusCode.BadUserAccessDenied
                        OpcResp.Body.ResponseHeader.StringTable = [
                          opc.getStatusCodeName(
                            opc.StatusCode.BadUserAccessDenied
                          ),
                          opc.getStatusCodeText(
                            opc.StatusCode.BadUserAccessDenied
                          ),
                          'User has no right to issue commands to the group1 destination!'
                        ]
                        res.send(OpcResp)
                        return
                      } else {
                        console.log(
                          `User authorized to issue commands to the group1 destination! [${username}]`
                        )
                      }
                    }

                    let result = await db.collection(COLL_COMMANDS).insertOne({
                      protocolSourceConnectionNumber: new mongo.Double(
                        data.protocolSourceConnectionNumber
                      ),
                      protocolSourceCommonAddress: new mongo.Double(
                        data.protocolSourceCommonAddress
                      ),
                      protocolSourceObjectAddress: new mongo.Double(
                        data.protocolSourceObjectAddress
                      ),
                      protocolSourceASDU: new mongo.Double(
                        data.protocolSourceASDU
                      ),
                      protocolSourceCommandDuration: new mongo.Double(
                        data.protocolSourceCommandDuration
                      ),
                      protocolSourceCommandUseSBO:
                        data.protocolSourceCommandUseSBO,
                      pointKey: new mongo.Double(data._id),
                      tag: data.tag,
                      timeTag: new Date(),
                      value: new mongo.Double(cmd_val),
                      valueString: parseFloat(cmd_val).toString(),
                      originatorUserName: username,
                      originatorIpAddress:
                        req.headers['x-real-ip'] ||
                        req.headers['x-forwarded-for'] ||
                        req.connection.remoteAddress
                    })
                    // console.log(result);
                    if (result.insertedCount !== 1) {
                      OpcResp.Body.ResponseHeader.ServiceResult =
                        opc.StatusCode.BadUnexpectedError
                      OpcResp.Body.ResponseHeader.StringTable = [
                        opc.getStatusCodeName(
                          opc.StatusCode.BadUnexpectedError
                        ),
                        opc.getStatusCodeText(
                          opc.StatusCode.BadUnexpectedError
                        ),
                        'Could not queue command!'
                      ]
                      res.send(OpcResp)
                      return
                    }

                    OpcResp.Body.Results.push(opc.StatusCode.Good) // write ok
                    // a way for the client to find this inserted command
                    OpcResp.Body._CommandHandles.push(result.insertedId)
                  }
              } else if (node.AttributeId == opc.AttributeId.Description) {
                // Write Properties
                if ('NodeId' in node)
                  if ('Id' in node.NodeId) {
                    let findPoint = null
                    if (node.NodeId.IdType === opcIdTypeNumber) {
                      findPoint = { _id: parseInt(node.NodeId.Id) }
                    } else if (node.NodeId.IdType === opcIdTypeString) {
                      findPoint = { tag: node.NodeId.Id }
                    }
                    if (findPoint === null) {
                      OpcResp.Body.ResponseHeader.ServiceResult =
                        opc.StatusCode.BadNodeIdInvalid
                      OpcResp.Body.ResponseHeader.StringTable = [
                        opc.getStatusCodeName(opc.StatusCode.BadNodeIdInvalid),
                        opc.getStatusCodeText(opc.StatusCode.BadNodeIdInvalid),
                        'Invalid IdType!'
                      ]
                      res.send(OpcResp)
                      return
                    }

                    if (findPoint !== null) {
                      let prevData = await db
                        .collection(COLL_REALTIME)
                        .findOne(findPoint)

                      let alarmDisableNew = {}
                      if (!AUTHENTICATION || userRights?.disableAlarms)
                        if (
                          prevData?.alarmDisabled !==
                          node.Value._Properties?.alarmDisabled
                        )
                          alarmDisableNew = {
                            alarmDisabled: node.Value._Properties?.alarmDisabled
                          }

                      let annotationNew = {}
                      if (!AUTHENTICATION || userRights?.enterAnnotations)
                        if (
                          prevData?.annotation !==
                          node.Value._Properties?.annotation
                        )
                          annotationNew = {
                            annotation: node.Value._Properties?.annotation
                          }

                      let loLimitNew = {}
                      if (!AUTHENTICATION || userRights?.enterLimits)
                        if (
                          prevData?.loLimit !== node.Value._Properties?.loLimit
                        )
                          loLimitNew = {
                            loLimit: new mongo.Double(
                              node.Value._Properties.loLimit
                            )
                          }

                      let hiLimitNew = {}
                      if (!AUTHENTICATION || userRights?.enterLimits)
                        if (
                          prevData?.hiLimit !== node.Value._Properties?.hiLimit
                        )
                          hiLimitNew = {
                            hiLimit: new mongo.Double(
                              node.Value._Properties.hiLimit
                            )
                          }

                      // loloLimit: node.Value._Properties.loLimit,
                      // lololoLimit: node.Value._Properties.loLimit,
                      // hihiLimit: node.Value._Properties.hiLimit,
                      // hihihiLimit: node.Value._Properties.hiLimit,

                      let notesNew = {}
                      if (!AUTHENTICATION || userRights?.enterNotes)
                        if (prevData?.notes !== node.Value._Properties?.notes)
                          notesNew = {
                            notes: node.Value._Properties.notes
                          }

                      let valueNew = {}
                      if (
                        'substituted' in node.Value._Properties &&
                        'newValue' in node.Value._Properties
                      )
                        if (!AUTHENTICATION || userRights?.substituteValues)
                          if (
                            prevData?.value !== node.Value._Properties.newValue
                          )
                            valueNew = {
                              value: new mongo.Double(
                                node.Value._Properties.newValue
                              )
                            }

                      let set = {
                        $set: {
                          ...alarmDisableNew,
                          ...annotationNew,
                          ...loLimitNew,
                          ...hiLimitNew,
                          ...notesNew,
                          ...valueNew
                        }
                      }

                      let result = await db
                        .collection(COLL_REALTIME)
                        .updateOne(findPoint, set)
                      if (
                        typeof result.result.n === 'number' &&
                        result.result.n === 1
                      ) {
                        // updateOne ok
                        OpcResp.Body.Results.push(opc.StatusCode.Good)
                        console.log('update ok id: ' + node.NodeId.Id)
                        UserActionsQueue.enqueue({
                          username: username,
                          pointKey: node.NodeId.Id,
                          action: 'Update Properties',
                          properties: set['$set'],
                          timeTag: new Date()
                        })

                        // insert event for changed annotation
                        if ('annotation' in annotationNew) {
                          let eventDate = new Date()
                          db.collection(COLL_SOE).insertOne({
                            tag: prevData.tag,
                            pointKey: prevData._id,
                            group1: prevData?.group1,
                            description: prevData?.description,
                            eventText: (annotationNew.annotation.trim()==='')?'🏷️🗑️':'🏷️🔒', // &#127991;
                            invalid: false,
                            priority: prevData?.priority,
                            timeTag: eventDate,
                            timeTagAtSource: eventDate,
                            timeTagAtSourceOk: true,
                            ack: 1
                          })
                        }

                        // insert event for changed value
                        if ('value' in valueNew) {
                          let eventDate = new Date()
                          let eventText = ''
                          if (prevData?.type === 'digital')
                            eventText = (valueNew.value == 0)
                              ? prevData?.eventTextFalse
                              : prevData?.eventTextTrue
                          else
                            eventText =
                              valueNew.value.toFixed(2) +
                              ' ' +
                              prevData?.unit
                          db.collection(COLL_SOE).insertOne({
                            tag: prevData.tag,
                            pointKey: prevData._id,
                            group1: prevData?.group1,
                            description: prevData?.description,
                            eventText: eventText,
                            invalid: false,
                            priority: prevData?.priority,
                            timeTag: eventDate,
                            timeTagAtSource: eventDate,
                            timeTagAtSourceOk: true,
                            ack: 1
                          })
                        }
                      } else {
                        // some updateOne error
                        OpcResp.Body.Results.push(
                          opc.StatusCode.BadNodeIdUnknown
                        )
                        console.log('update error id: ' + node.NodeId.Id)
                      }
                    }
                  } else
                    OpcResp.Body.Results.push(opc.StatusCode.BadNodeIdInvalid)
              }
            }
          }
          OpcResp.Body.ResponseHeader.ServiceResult = opc.StatusCode.Good
          res.send(OpcResp)
        }
        return
      case opc.ServiceCode.ReadRequest: // READ SERVICE
        {
          OpcResp.ServiceId = opc.ServiceCode.ReadResponse
          if (
            !('NodesToRead' in req.body.Body) &&
            !('ContentFilter' in req.body.Body)
          ) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.GoodNoData
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.GoodNoData),
              opc.getStatusCodeText(opc.StatusCode.GoodNoData),
              'No NodesToRead nor Content Filter'
            ]
            res.send(OpcResp)
            return
          }

          // if for some point Description is solicited, will respond with full info for all the points
          let points = [],
            cmdHandles = [],
            info = false,
            ack = false
          if ('NodesToRead' in req.body.Body)
            req.body.Body.NodesToRead.map(node => {
              if ('AttributeId' in node) {
                if (node.AttributeId == opc.AttributeId.EventNotifier) {
                  if ('ClientHandle' in node) {
                    ack = true
                    cmdHandles.push(node.ClientHandle)
                  }
                }
                if (node.AttributeId == opc.AttributeId.Description) {
                  info = true
                }
              }
              if ('NodeId' in node)
                if ('Id' in node.NodeId) {
                  points.push(node.NodeId.Id)
                }
              return node
            })
          if (points.length == 0 && !('ContentFilter' in req.body.Body)) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.GoodNoData
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.GoodNoData),
              opc.getStatusCodeText(opc.StatusCode.GoodNoData),
              'No NodeId.Id found'
            ]
            res.send(OpcResp)
            return
          }

          if (ack) {
            // look for command ack
            // console.log(cmdHandles[0])
            if (
              typeof cmdHandles[0] !== 'string' ||
              cmdHandles[0].length < 24
            ) {
              OpcResp.Body.ResponseHeader.ServiceResult =
                opc.StatusCode.BadRequestNotAllowed
              OpcResp.Body.ResponseHeader.StringTable = [
                opc.getStatusCodeName(opc.StatusCode.BadRequestNotAllowed),
                opc.getStatusCodeText(opc.StatusCode.BadRequestNotAllowed),
                'Missing or invalid ClientHandle.'
              ]
              res.send(OpcResp)
              return
            }

            let data = await db
              .collection(COLL_COMMANDS)
              .findOne({ _id: new mongo.ObjectID(cmdHandles[0]) })

            // console.log(data);
            let status = -1,
              ackTime,
              cmdTime
            if (typeof data.ack === 'boolean') {
              // ack received
              if ((data.ack = true)) status = opc.StatusCode.Good
              else status = opc.StatusCode.Bad
              cmdTime = data.timeTag
              ackTime = data.ackTimeTag
            } else status = opc.StatusCode.BadWaitingForResponse

            OpcResp.ServiceId = opc.ServiceCode.DataChangeNotification
            OpcResp.Body.MonitoredItems = [
              {
                ClientHandle: cmdHandles[0],
                Value: {
                  Value: data.value,
                  StatusCode: status,
                  SourceTimestamp: cmdTime,
                  ServerTimestamp: ackTime
                },
                NodeId: {
                  IdType: opcIdTypeString,
                  Id: data.tag,
                  Namespace: opc.NamespaceMongodb
                }
              }
            ]
            OpcResp.Body.ResponseHeader.ServiceResult = opc.StatusCode.Good
            res.send(OpcResp)
            return
          }

          let tags = [],
            npts = []
          for (let i = 0; i < points.length; i++) {
            let val = parseInt(points[i])
            if (!isNaN(val)) npts.push(val)
            else tags.push(points[i])
          }

          let projection = info
            ? null
            : {
                projection: {
                  _id: 1,
                  tag: 1,
                  value: 1,
                  valueString: 1,
                  invalid: 1,
                  timeTag: 1,
                  alarmed: 1,
                  type: 1,
                  annotation: 1,
                  origin: 1,
                  group1: 1
                }
              }

          // optimize query for better performance
          // there is a cost to query empty $in lists!
          let findTags = {
            tag: {
              $in: tags
            }
          }
          let findKeys = {
            _id: {
              $in: npts
            }
          }
          let query = findTags
          if (npts.length > 0 && tags.length > 0)
            query = {
              // use $or only to find tags and _id keys
              $or: [findKeys, findTags]
            }
          else if (npts.length > 0) query = findKeys

          let sort = {}

          // if there is a content filter, it takes precedence over NodesToRead lists
          if ('ContentFilter' in req.body.Body) {
            sort = { group1: 1, group2: 1 }
            projection = null
            query = {}
            let grp1 = null
            let grp2 = null
            req.body.Body.ContentFilter.map(contentFilter => {
              // supports attribute (operand) Equals (operator) to a literal value (operand)
              if (contentFilter.FilterOperator === opc.FilterOperator.Equals) {
                if (
                  contentFilter.FilterOperands[0].FilterOperand ===
                    opc.Operand.Attribute &&
                  contentFilter.FilterOperands[1].FilterOperand ===
                    opc.Operand.Literal
                ) {
                  if (contentFilter.FilterOperands[0].Value == 'group1') {
                    grp1 = { group1: contentFilter.FilterOperands[1].Value }
                  }
                  if (contentFilter.FilterOperands[0].Value == 'group2') {
                    grp2 = { group2: contentFilter.FilterOperands[1].Value }
                  }

                  if (
                    // fake attribute 'persistentAlarms' means add not normal states (digitals with alarmState=0 and value=0 or alarmState=1 and value=1)
                    contentFilter.FilterOperands[0].Value == 'persistentAlarms'
                  ) {
                    sort = { alarmed: -1, timeTagAlarm: -1 }
                    query = {
                      $or: [
                        {
                          $and: [
                            { type: 'analog' },
                            { alarmed: true },
                            { invalid: false },
                            { ...(grp1 !== null ? grp1 : {}) },
                            { ...(grp2 !== null ? grp2 : {}) }
                          ]
                        },
                        {
                          $and: [
                            { type: 'digital' },
                            { alarmState: 0 },
                            { value: 0 },
                            { invalid: false },
                            { ...(grp1 !== null ? grp1 : {}) },
                            { ...(grp2 !== null ? grp2 : {}) }
                          ]
                        },
                        {
                          $and: [
                            { type: 'digital' },
                            { alarmState: 1 },
                            { value: 1 },
                            { invalid: false },
                            { ...(grp1 !== null ? grp1 : {}) },
                            { ...(grp2 !== null ? grp2 : {}) }
                          ]
                        },
                        query
                      ]
                    }
                  } else
                    query[contentFilter.FilterOperands[0].Value] =
                      contentFilter.FilterOperands[1].Value
                }
              }
              return contentFilter
            })
          }

          db.collection(COLL_REALTIME)
            .find(query, projection)
            .sort(sort)
            .toArray(function (err, results) {
              if (results) {
                if (results.length == 0) {
                  OpcResp.Body.ResponseHeader.ServiceResult =
                    opc.StatusCode.GoodNoData
                  OpcResp.Body.ResponseHeader.StringTable = [
                    opc.getStatusCodeName(opc.StatusCode.GoodNoData),
                    opc.getStatusCodeText(opc.StatusCode.GoodNoData),
                    'No NodeId.Id requested found on realtimeDatabase'
                  ]
                  res.send(OpcResp)
                  return
                }

                let Results = []
                if ('NodesToRead' in req.body.Body) {
                  req.body.Body.NodesToRead.map(node => {
                    let Result = {
                      // will return this if point not found or access denied
                      StatusCode: opc.StatusCode.BadNotFound,
                      NodeId: node.NodeId,
                      Value: null,
                      _Properties: null
                    }

                    for (let i = 0; i < results.length; i++) {
                      let pointInfo = results[i]

                      if (
                        node.NodeId.Id === pointInfo.tag ||
                        node.NodeId.Id === pointInfo._id
                      ) {
                        // check for group1 list in user rights (from token)
                        if (
                          AUTHENTICATION &&
                          userRights.group1List.length > 0
                        ) {
                          if (
                            ![-1, -2].includes(pointInfo._id) &&
                            !userRights.group1List.includes(pointInfo.group1)
                          ) {
                            // Access to data denied! (return null value and properties)
                            Result.StatusCode =
                              opc.StatusCode.BadUserAccessDenied
                            break
                          }
                        }

                        Result.StatusCode = opc.StatusCode.Good
                        Result._Properties = {
                          _id: pointInfo._id,
                          valueString: pointInfo.valueString,
                          alarmed: pointInfo.alarmed,
                          transit: pointInfo.transit,
                          annotation: pointInfo.annotation,
                          notes: pointInfo.notes,
                          origin: pointInfo.origin
                        }
                        if (info) {
                          Result._Properties = {
                            _id: pointInfo._id,
                            valueString: pointInfo.valueString,
                            valueDefault: pointInfo.valueDefault,
                            alarmed: pointInfo.alarmed,
                            alarmDisabled: pointInfo.alarmDisabled,
                            transit: pointInfo.transit,
                            group1: pointInfo.group1,
                            group2: pointInfo.group2,
                            description: pointInfo.description,
                            ungroupedDescription:
                              pointInfo.ungroupedDescription,
                            hiLimit: pointInfo.hiLimit,
                            loLimit: pointInfo.loLimit,
                            hysteresis: pointInfo.hysteresis,
                            stateTextTrue: pointInfo.stateTextTrue,
                            stateTextFalse: pointInfo.stateTextFalse,
                            unit: pointInfo.unit,
                            annotation: pointInfo.annotation,
                            notes: pointInfo.notes,
                            commandOfSupervised: pointInfo.commandOfSupervised,
                            supervisedOfCommand: pointInfo.supervisedOfCommand,
                            origin: pointInfo.origin
                          }
                        }
                        if (pointInfo.type === 'string')
                          Result.Value = {
                            Type: opc.DataType.String,
                            Body: pointInfo.valueString,
                            Quality: pointInfo.invalid
                              ? opc.StatusCode.Bad
                              : opc.StatusCode.Good
                          } 
                        else
                          Result.Value = {
                            Type:
                              pointInfo.type === 'digital'
                                ? opc.DataType.Boolean
                                : opc.DataType.Double,
                            Body:
                              pointInfo.type === 'digital'
                                ? pointInfo.value !== 0
                                  ? true
                                  : false
                                : parseFloat(pointInfo.value),
                            Quality: pointInfo.invalid
                              ? opc.StatusCode.Bad
                              : opc.StatusCode.Good
                          }
                        Result.NodeId = {
                          IdType: opcIdTypeString,
                          Id: pointInfo.tag,
                          Namespace: opc.NamespaceMongodb
                        }
                        Result.SourceTimestamp = pointInfo.timeTag
                        break
                      }
                    }
                    Results.push(Result)
                  })
                } else {
                  // no NodesToRead so it is a filtered query
                  results.map(node => {

                    let Value = {}
                    if (node.type === 'string')
                      Value = {
                        Type: opc.DataType.String,
                        Body: node.valueString,
                        Quality: node.invalid
                        ? opc.StatusCode.Bad
                        : opc.StatusCode.Good
                      }
                    else
                      Value = {
                        Type:
                          node.type === 'digital'
                            ? opc.DataType.Boolean
                            : opc.DataType.Double,
                        Body:
                          node.type === 'digital'
                            ? node.value !== 0
                              ? true
                              : false
                            : node.value,
                        Quality: node.invalid
                          ? opc.StatusCode.Bad
                          : opc.StatusCode.Good
                      }

                    let Result = {
                      StatusCode: opc.StatusCode.Good,
                      NodeId: {
                        IdType: opcIdTypeString,
                        Id: node.tag
                      },
                      Value: Value,
                      _Properties: {
                        _id: node._id,
                        valueString: node.valueString,
                        valueDefault: node.valueDefault,
                        alarmed: node.alarmed,
                        alarmDisabled: node.alarmDisabled,
                        alarmState: node.alarmState,
                        isEvent: node.isEvent,
                        transit: node.transit,
                        group1: node.group1,
                        group2: node.group2,
                        description: node.description,
                        ungroupedDescription: node.ungroupedDescription,
                        hiLimit: node.hiLimit,
                        loLimit: node.loLimit,
                        hysteresis: node.hysteresis,
                        stateTextTrue: node.stateTextTrue,
                        stateTextFalse: node.stateTextFalse,
                        unit: node.unit,
                        annotation: node.annotation,
                        notes: node.notes,
                        commandOfSupervised: node.commandOfSupervised,
                        supervisedOfCommand: node.supervisedOfCommand,
                        origin: node.origin,
                        timeTagAlarm: node.timeTagAlarm,
                        priority: node.priority,
                        origin: node.origin,
                        timeTagAtSourceOk:
                          'timeTagAtSourceOk' in node
                            ? node.TimeTagAtSourceOk
                            : null
                      }
                    }
                    Result.ServerTimestamp = node.timeTag
                    Result.SourceTimestamp =
                      'timeTagAtSource' in node && node.timeTagAtSource !== null
                        ? node.timeTagAtSource
                        : null

                    // check for group1 list in user rights (from token)
                    if (AUTHENTICATION && userRights.group1List.length > 0) {
                      if (!userRights.group1List.includes(node.group1)) {
                        // Access to data denied!
                        return node
                      }
                    }

                    Results.push(Result)

                    return node
                  })
                }

                OpcResp.Body.Results = Results
                OpcResp.Body.ResponseHeader.ServiceResult = opc.StatusCode.Good
                OpcResp.Body.ResponseHeader.StringTable = [
                  opc.getStatusCodeName(opc.StatusCode.Good),
                  opc.getStatusCodeText(opc.StatusCode.Good),
                  'Query time: ' + (new Date().getTime() - tini) + ' ms'
                ]
                res.send(OpcResp)
                console.log(
                  'Read returned ' +
                    results.length +
                    ' values. Elapsed ' +
                    (new Date().getTime() - tini) +
                    ' ms'
                )
              }
            })
        }
        return
      case opc.ServiceCode.HistoryReadRequest: // HISTORY READ SERVICE
        {
          OpcResp.ServiceId = opc.ServiceCode.HistoryReadResponse
          if (
            !('HistoryReadDetails' in req.body.Body) ||
            !('ParameterTypeId' in req.body.Body.HistoryReadDetails) ||
            req.body.Body.HistoryReadDetails.ParameterTypeId !==
              opc.ServiceCode.ReadRawModifiedDetails
          ) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.BadHistoryOperationInvalid
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.BadHistoryOperationInvalid),
              opc.getStatusCodeText(opc.StatusCode.BadHistoryOperationInvalid),
              'Invalid HistoryReadDetails option'
            ]
            res.send(OpcResp)
            return
          }

          let returnServerTimestamp = true
          let returnSourceTimestamp = true
          if ('TimestampsToReturn' in req.body.Body) {
            switch (req.body.Body.TimestampsToReturn) {
              case opc.TimestampsToReturn.Source:
                returnServerTimestamp = false
                break
              case opc.TimestampsToReturn.Server:
                returnSourceTimestamp = false
                break
              case opc.TimestampsToReturn.Neither:
                returnServerTimestamp = false
                returnSourceTimestamp = false
                break
              case opc.TimestampsToReturn.Both:
              case opc.TimestampsToReturn.Invalid:
              default:
                break
            }
          }

          if (
            !('NodesToRead' in req.body.Body) &&
            !('ContentFilter' in req.body.Body)
          ) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.GoodNoData
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.GoodNoData),
              opc.getStatusCodeText(opc.StatusCode.GoodNoData),
              'No NodesToRead'
            ]
            res.send(OpcResp)
            return
          }

          let tags = []
          if ('NodesToRead' in req.body.Body) {
            req.body.Body.NodesToRead.map(node => {
              if ('AttributeId' in node) {
                if (node.AttributeId == opc.AttributeId.Value) {
                  if ('NodeId' in node)
                    if ('IdType' in node.NodeId)
                      if (node.NodeId.IdType === opcIdTypeString)
                        if ('Id' in node.NodeId) {
                          // only string keys supported here
                          tags.push("'" + node.NodeId.Id + "'")
                        }
                }
              }
              return node
            })
            if (tags.length == 0) {
              OpcResp.Body.ResponseHeader.ServiceResult =
                opc.StatusCode.GoodNoData
              OpcResp.Body.ResponseHeader.StringTable = [
                opc.getStatusCodeName(opc.StatusCode.GoodNoData),
                opc.getStatusCodeText(opc.StatusCode.GoodNoData),
                'No valid NodeId.Ids/Attribute request found'
              ]
              res.send(OpcResp)
              return
            }
          }

          // default read is last hour
          let dt = new Date()
          let endDateTime = dt.toISOString()
          dt.setHours(dt.getHours() - 1)
          let startDateTime = dt.toISOString()
          let limitValues = 10000

          if ('ParameterData' in req.body.Body.HistoryReadDetails) {
            if ('StartTime' in req.body.Body.HistoryReadDetails.ParameterData)
              startDateTime =
                req.body.Body.HistoryReadDetails.ParameterData.StartTime
            if ('EndTime' in req.body.Body.HistoryReadDetails.ParameterData)
              endDateTime =
                req.body.Body.HistoryReadDetails.ParameterData.EndTime
            if (
              'NumValuesPerNode' in
              req.body.Body.HistoryReadDetails.ParameterData
            )
              limitValues =
                req.body.Body.HistoryReadDetails.ParameterData.NumValuesPerNode

            // req.body.Body.HistoryReadDetails.ParameterData.IsTeasModified is expected to be false!
            if (
              'IsReadModified' in
                req.body.Body.HistoryReadDetails.ParameterData &&
              req.body.Body.HistoryReadDetails.ParameterData.IsReadModified ===
                true
            ) {
              OpcResp.Body.ResponseHeader.ServiceResult =
                opc.StatusCode.BadHistoryOperationUnsupported
              OpcResp.Body.ResponseHeader.StringTable = [
                opc.getStatusCodeName(
                  opc.StatusCode.BadHistoryOperationUnsupported
                ),
                opc.getStatusCodeText(
                  opc.StatusCode.BadHistoryOperationUnsupported
                ),
                'Invalid ReadRawModifiedDetails/isReadModified option (must be false)'
              ]
              res.send(OpcResp)
              return
            }

            // Ignored parameters
            // req.body.Body.ReadRawModifiedDetails.NumValuesPerNode
            // req.body.Body.ReadRawModifiedDetails.ReturnBounds
          }

          // when Namespace is 2 (mongodb) will read from mongodb soeData
          if (
            'Namespace' in req.body.Body &&
            req.body.Body.Namespace === opc.NamespaceMongodb
          ) {
            let group1Filter = []
            let filterPriority = {}

            if (
              'ContentFilter' in req.body.Body &&
              req.body.Body.ContentFilter.length > 0
              // &&
              // req.body.Body.ContentFilter[0].FilterOperands.length > 0
            )
              req.body.Body.ContentFilter.map(node => {
                if (node.FilterOperator === opc.FilterOperator.LessThanOrEqual)
                  // priority filter
                  filterPriority = {
                    priority: { $lte: node.FilterOperands[0] }
                  }

                if (node.FilterOperator === opc.FilterOperator.InList)
                  // group1 list filter
                  node.FilterOperands.map(element => {
                    if (element.FilterOperand === opc.Operand.Literal)
                      if (typeof element.Value === 'string') {
                        group1Filter.push(element.Value)
                      }
                    return element
                  })

                return node
              })

            let filterDateGte = {
              timeTag: { $gte: new Date(startDateTime) }
            }
            let filterDateLte = {
              timeTag: { $lte: new Date(endDateTime) }
            }
            let sort = { timeTag: -1, timeTagAtSource: -1, tag: -1 }
            if (endDateTime !== null && startDateTime !== null)
              sort = { timeTag: 1, timeTagAtSource: 1, tag: 1 }

            if (!returnServerTimestamp) {
              sort = { timeTagAtSource: -1, timeTag: -1, tag: -1 }
              if (endDateTime !== null && startDateTime !== null)
                sort = { timeTagAtSource: 1, timeTag: 1, tag: 1 }
              filterDateGte = {
                timeTagAtSource: { $gte: new Date(startDateTime) }
              }
              filterDateLte = {
                timeTagAtSource: { $lte: new Date(endDateTime) }
              }
            }

            if (startDateTime === null) {
              filterDateGte = {}
            }
            if (endDateTime === null) {
              filterDateLte = {}
            }

            let filterGroup = {}
            if (group1Filter.length > 0)
              filterGroup = {
                $or: [
                  {
                    group1: {
                      $in: group1Filter
                    }
                  },
                  {
                    tag: {
                      $in: tags
                    }
                  }
                ]
              }

            // depending on aggregation do a aggregate (more expensive) or a simple find
            if (
              'AggregateFilter' in req.body.Body &&
              req.body.Body.AggregateFilter !== null &&
              'AggregateType' in req.body.Body.AggregateFilter &&
              req.body.Body.AggregateFilter.AggregateType === 'Count'
            ) {
              db.collection(COLL_SOE)
                .aggregate([
                  {
                    $match: {
                      $and: [
                        filterDateGte,
                        filterDateLte,
                        filterGroup,
                        filterPriority,
                        { ack: { $lte: 1 } }
                      ]
                    }
                  },
                  {
                    $group: {
                      _id: '$tag',
                      tag: { $last: '$tag' },
                      pointKey: { $last: '$pointKey' },
                      group1: { $last: '$group1' },
                      description: { $last: '$description' },
                      eventText: { $last: '$eventText' },
                      description: { $last: '$description' },
                      invalid: { $last: '$invalid' },
                      priority: { $last: '$priority' },
                      timeTag: { $last: '$timeTag' },
                      timeTagAtSource: { $last: '$timeTagAtSource' },
                      timeTagAtSourceOk: { $last: '$timeTagAtSourceOk' },
                      ack: { $last: '$ack' },
                      count: { $sum: 1 },
                      event_id: { $last: '$_id' }
                    }
                  }
                ])
                .sort(sort)
                .limit(limitValues)
                .toArray(procArrayResults)
            } else {
              db.collection(COLL_SOE)
                .find(
                  {
                    $and: [
                      filterDateGte,
                      filterDateLte,
                      filterGroup,
                      filterPriority,
                      { ack: { $lte: endDateTime !== null ? 2 : 1 } } // when realtime query (endDate=null) filter out eliminated (ack=2) events
                    ]
                  },
                  {}
                )
                .sort(sort)
                .limit(limitValues)
                .toArray(procArrayResults)
            }

            function procArrayResults (err, results) {
              if (results) {
                if (results.length == 0) {
                  OpcResp.Body.ResponseHeader.ServiceResult =
                    opc.StatusCode.GoodNoData
                  OpcResp.Body.ResponseHeader.StringTable = [
                    opc.getStatusCodeName(opc.StatusCode.GoodNoData),
                    opc.getStatusCodeText(opc.StatusCode.GoodNoData),
                    'No NodeId.Id requested found on soeData'
                  ]
                  res.send(OpcResp)
                  return
                }

                let Results = []
                results.map(node => {
                  let NodeId = {
                    IdType: opcIdTypeString,
                    Id: node.tag,
                    Namespace: opc.NamespaceMongodb
                  }

                  let HistoryData = {
                    Value: {
                      Type: opc.DataType.String,
                      Body: node.eventText,
                      Quality: node.invalid
                        ? opc.StatusCode.Bad
                        : opc.StatusCode.Good,
                      Count: typeof node.count === 'number' ? node.count : 1
                    },
                    ServerTimestamp: node.timeTag,
                    SourceTimestamp: node.timeTagAtSource,
                    SourceTimestampOk: node.timeTagAtSourceOk,
                    Acknowledge: node.ack
                  }

                  let _Properties = {
                    group1: node.group1,
                    description: node.description,
                    priority: node.priority,
                    pointKey: node.pointKey,
                    event_id: node._id === node.tag ? node.event_id : node._id
                  }

                  let HistoryReadResult = {
                    StatusCode: opc.StatusCode.Good,
                    HistoryData: [HistoryData],
                    // ContinuationPoint: null,
                    NodeId: NodeId,
                    _Properties: _Properties
                  }
                  Results.push(HistoryReadResult)
                  return node
                })

                OpcResp.Body.Results = Results
                OpcResp.Body.ResponseHeader.ServiceResult = opc.StatusCode.Good
                OpcResp.Body.ResponseHeader.StringTable = [
                  opc.getStatusCodeName(opc.StatusCode.Good),
                  opc.getStatusCodeText(opc.StatusCode.Good),
                  'Query time: ' + (new Date().getTime() - tini) + ' ms'
                ]
                res.send(OpcResp)
                console.log(
                  'HistoryRead (Events) returned ' +
                    Results.length +
                    ' values. Elapsed ' +
                    (new Date().getTime() - tini) +
                    ' ms'
                )
                return
              }
            }
            return
          }

          let query =
            'SELECT tag, value, flags, ' +
            'time_tag, ' +
            'time_tag_at_source ' +
            'FROM hist ' +
            "WHERE time_tag>='" +
            startDateTime +
            "' AND " +
            "time_tag<='" +
            endDateTime +
            "' AND " +
            'tag IN (' +
            tags.join(',') +
            ') ' +
            'ORDER BY tag asc, time_tag ASC'

          if (startDateTime === endDateTime) {
            query =
              'SELECT tag as tag, ' +
              'last(value, time_tag) as value, ' +
              'last(flags, time_tag) as flags, ' +
              'last(time_tag, time_tag) as time_tag, ' +
              'last(time_tag_at_source, time_tag) as time_tag_at_source ' +
              'from hist ' +
              'where ' +
              "time_tag<='" +
              startDateTime +
              "' and " +
              "time_tag> (TIMESTAMP '" +
              startDateTime +
              "' - INTERVAL '0.5 day') and " +
              'tag in (' +
              tags.join(',') +
              ') group by tag'
          }

          // read data from postgreSQL
          pool.query(query, (err, resp) => {
            if (err) {
              OpcResp.ServiceId = opc.ServiceCode.ServiceFault
              OpcResp.Body.ResponseHeader.ServiceResult =
                opc.StatusCode.BadServerNotConnected
              OpcResp.Body.ResponseHeader.StringTable = [
                opc.getStatusCodeName(opc.StatusCode.BadServerNotConnected),
                opc.getStatusCodeText(opc.StatusCode.BadServerNotConnected),
                'Database error!'
              ]
              res.send(OpcResp)
              return
            }

            let Results = [] // [{ StatusCode, ContinuationPoint, HistoryData[], NodeId }]
            req.body.Body.NodesToRead.map(node => {
              let HistoryReadResult = {
                StatusCode: opc.StatusCode.BadNotFound,
                HistoryData: [],
                ContinuationPoint: null,
                NodeId: null
              }
              if ('AttributeId' in node) {
                if (node.AttributeId == opc.AttributeId.Value) {
                  if ('NodeId' in node)
                    if ('IdType' in node.NodeId)
                      if (node.NodeId.IdType === opcIdTypeString)
                        if ('Id' in node.NodeId) {
                          // only string keys supported here
                          HistoryReadResult.NodeId = node.NodeId
                          resp.rows.map(node => {
                            if (node.tag === HistoryReadResult.NodeId.Id) {
                              HistoryReadResult.StatusCode = opc.StatusCode.Good
                              HistoryReadResult.HistoryData.push({
                                Value: {
                                  Type:
                                    node.flags.charAt(2) === '0'
                                      ? opc.DataType.Boolean
                                      : opc.DataType.Double,
                                  Body:
                                    node.flags.charAt(2) === '0'
                                      ? node.value === 0
                                        ? false
                                        : true
                                      : node.value,
                                  Quality:
                                    node.flags.charAt(0) === '0'
                                      ? opc.StatusCode.Good
                                      : opc.StatusCode.Bad
                                },
                                SourceTimestamp: returnSourceTimestamp
                                  ? node.time_tag_at_source
                                  : undefined,
                                ServerTimestamp: returnServerTimestamp
                                  ? node.time_tag
                                  : undefined
                              })
                            }
                            return node
                          })
                        }
                }
                Results.push(HistoryReadResult)
              }
              return node
            })

            OpcResp.Body.Results = Results
            OpcResp.Body.ResponseHeader.ServiceResult = opc.StatusCode.Good
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.Good),
              opc.getStatusCodeText(opc.StatusCode.Good),
              'Query time: ' + (new Date().getTime() - tini) + ' ms'
            ]
            res.send(OpcResp)
            console.log(
              'HistoryRead returned ' +
                resp.rows.length +
                ' values. Elapsed ' +
                (new Date().getTime() - tini) +
                ' ms'
            )
          })
        }
        return
      case opc.ServiceCode.Extended_RequestUniqueAttributeValues: // list unique attribute values
        {
          OpcResp.ServiceId =
            opc.ServiceCode.Extended_ResponseUniqueAttributeValues

          if (req.body.Body.AttributeId !== opc.AttributeId.ExtendedGroup1) {
            OpcResp.Body.ResponseHeader.ServiceResult =
              opc.StatusCode.BadAttributeIdInvalid
            OpcResp.Body.ResponseHeader.StringTable = [
              opc.getStatusCodeName(opc.StatusCode.BadAttributeIdInvalid),
              opc.getStatusCodeText(opc.StatusCode.BadAttributeIdInvalid),
              'Requested attribute not supported!'
            ]
            res.send(OpcResp)
            return
          }

          db.collection(COLL_REALTIME)
            .aggregate([
              {
                $group: {
                  _id: '$group1',
                  group1: { $last: '$group1' },
                  count: { $sum: 1 }
                }
              }
            ])
            .sort({ group1: 1 })
            .toArray(function (err, results) {
              let Results = []
              results.map(node => {
                let Value = {
                  Value: {
                    Type: opc.DataType.String,
                    Body: node.group1,
                    Count: node.count
                  }
                }
                Results.push(Value)
                return node
              })

              OpcResp.Body.Results = Results
              OpcResp.Body.ResponseHeader.ServiceResult = opc.StatusCode.Good
              OpcResp.Body.ResponseHeader.StringTable = [
                opc.getStatusCodeName(opc.StatusCode.Good),
                opc.getStatusCodeText(opc.StatusCode.Good),
                'Query time: ' + (new Date().getTime() - tini) + ' ms'
              ]
              res.send(OpcResp)
              console.log(
                'Unique attribute values returned ' +
                  Results.length +
                  ' values. Elapsed ' +
                  (new Date().getTime() - tini) +
                  ' ms'
              )
              return
            })
        }
        return
      default:
        OpcResp.ServiceId = opc.ServiceCode.ServiceFault
        OpcResp.Body.ResponseHeader.ServiceResult =
          opc.StatusCode.BadServiceUnsupported
        OpcResp.Body.ResponseHeader.StringTable = [
          opc.getStatusCodeName(opc.StatusCode.BadServiceUnsupported),
          opc.getStatusCodeText(opc.StatusCode.BadServiceUnsupported),
          'Existing services: 629=ReadRequest, 671=WriteRequest, 100000001=Extended_RequestListUniqueAttributeValues'
        ]
        res.send(OpcResp)
        return
    }
  }

  for (;;) {
    try {
      if (clientMongo) {
        if (!clientMongo.isConnected()) {
          // not anymore connected, will retry
          clientMongo.close()
          db = null
          clientMongo = null
        } else {
          // it is connected: process userActions fifo
          while (!UserActionsQueue.isEmpty()) {
            let ins = UserActionsQueue.peek()
            db.collection(COLL_ACTIONS).insertOne(ins)
            UserActionsQueue.dequeue()
          }
        }
      }

      if (!clientMongo) {
        let connOptions = {
          useNewUrlParser: true,
          useUnifiedTopology: true,
          appname: APP_NAME,
          poolSize: 20,
          readPreference: MongoClient.READ_PRIMARY
        }

        if (
          typeof jsConfig.tlsCaPemFile === 'string' &&
          jsConfig.tlsCaPemFile.trim() !== ''
        ) {
          jsConfig.tlsClientKeyPassword = jsConfig.tlsClientKeyPassword || ''
          jsConfig.tlsAllowInvalidHostnames =
            jsConfig.tlsAllowInvalidHostnames || false
          jsConfig.tlsAllowChainErrors = jsConfig.tlsAllowChainErrors || false
          jsConfig.tlsInsecure = jsConfig.tlsInsecure || false

          connOptions.tls = true
          connOptions.tlsCAFile = jsConfig.tlsCaPemFile
          connOptions.tlsCertificateKeyFile = jsConfig.tlsClientPemFile
          connOptions.tlsCertificateKeyFilePassword =
            jsConfig.tlsClientKeyPassword
          connOptions.tlsAllowInvalidHostnames =
            jsConfig.tlsAllowInvalidHostnames
          connOptions.tlsInsecure = jsConfig.tlsInsecure
        }

        // new connection
        console.log('Connecting to ' + jsConfig.mongoConnectionString)
        MongoClient.connect(
          jsConfig.mongoConnectionString,
          connOptions,
          async (err, client) => {
            if (err) {
              db = null
              clientMongo = null
              console.log(err)
              if (err.name == 'MongoParseError') process.exit(-1)
              return
            }
            db = client.db(jsConfig.mongoDatabaseName)
            clientMongo = client
          }
        )
      }
    } catch (e) {
      if (clientMongo) clientMongo.close()
      db = null
      clientMongo = null
      console.log(e)
    }

    // wait 5 seconds
    await new Promise(resolve => setTimeout(resolve, 5000))
  }
})()
