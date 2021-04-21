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

const Mongo = require('mongodb')
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')

let AutoKeyId = 0
let AutoKeyMultiplier = 100000 // should be more than estimated maximum points on a connection

function NewTag () {
  AutoKeyId++

  return {
    _id: new Mongo.Double(AutoKeyId),
    protocolSourceASDU: '',
    protocolSourceCommonAddress: '',
    protocolSourceConnectionNumber: new Mongo.Double(0),
    protocolSourceObjectAddress: '',
    alarmState: new Mongo.Double(-1.0),
    description: '',
    ungroupedDescription: '',
    group1: '',
    group2: '',
    group3: '',
    stateTextFalse: '',
    stateTextTrue: '',
    eventTextFalse: '',
    eventTextTrue: '',
    origin: 'supervised',
    tag: '',
    type: 'analog',
    value: new Mongo.Double(0),
    valueString: '',
    alarmDisabled: false,
    alerted: false,
    alarmed: false,
    alertedState: '',
    annotation: '',
    commandBlocked: false,
    commandOfSupervised: new Mongo.Double(0.0),
    commissioningRemarks: '',
    formula: new Mongo.Double(0.0),
    frozen: false,
    frozenDetectTimeout: new Mongo.Double(0.0),
    hiLimit: new Mongo.Double(Number.MAX_VALUE),
    hihiLimit: new Mongo.Double(Number.MAX_VALUE),
    hihihiLimit: new Mongo.Double(Number.MAX_VALUE),
    historianDeadBand: new Mongo.Double(0.0),
    historianPeriod: new Mongo.Double(0.0),
    hysteresis: new Mongo.Double(0.0),
    invalid: true,
    invalidDetectTimeout: new Mongo.Double(60000.0),
    isEvent: false,
    kconv1: new Mongo.Double(1.0),
    kconv2: new Mongo.Double(0.0),
    location: null,
    loLimit: new Mongo.Double(-Number.MAX_VALUE),
    loloLimit: new Mongo.Double(-Number.MAX_VALUE),
    lololoLimit: new Mongo.Double(-Number.MAX_VALUE),
    notes: '',
    overflow: false,
    parcels: null,
    priority: new Mongo.Double(0.0),
    protocolDestinations: null,
    sourceDataUpdate: null,
    supervisedOfCommand: new Mongo.Double(0.0),
    timeTag: null,
    timeTagAlarm: null,
    timeTagAtSource: null,
    timeTagAtSourceOk: false,
    transient: false,
    unit: '',
    updatesCnt: new Mongo.Double(0.0),
    valueDefault: new Mongo.Double(0.0),
    zeroDeadband: new Mongo.Double(0.0)
  }
}

// find biggest point key (_id) on range and adjust automatic key
async function GetAutoKeyInitialValue (rtCollection, configObj) {
  AutoKeyId = configObj.ConnectionNumber * AutoKeyMultiplier
  let resLastKey = await rtCollection
    .find({
      _id: {
        $gt: AutoKeyId,
        $lt: (configObj.ConnectionNumber + 1) * AutoKeyMultiplier
      }
    })
    .sort({ _id: -1 })
    .limit(1)
    .toArray()
  if (resLastKey.length > 0 && '_id' in resLastKey[0]) {
    if (parseInt(resLastKey[0]._id) >= AutoKeyId)
      AutoKeyId = parseInt(resLastKey[0]._id)
  }
  return AutoKeyId
}

let ListCreatedTags = []

async function AutoCreateTag (data, connectionNumber, rtDataCollection) {

  let tag = AppDefs.AUTOTAG_PREFIX + ':' + connectionNumber + ':' + data.protocolSourceObjectAddress

  if (!ListCreatedTags.includes(tag)) {
    // possibly not created tag, must check
    let res = await rtDataCollection
      .find({
        protocolSourceConnectionNumber: connectionNumber,
        protocolSourceObjectAddress: data.protocolSourceObjectAddress
      })
      .toArray()

    if ('length' in res && res.length === 0) {
      // not found, then create
      Log.log(
        'Auto Key - Tag not found, will create: ' + tag,
        Log.levelDetailed
      )
      let newTag = NewTag(data)
      newTag.protocolSourceConnectionNumber = new Mongo.Double(
        connectionNumber
      )

      newTag.protocolSourceObjectAddress = data.protocolSourceObjectAddress
      newTag.tag = tag
      if ('type' in data)
        newTag.type = data.type
      newTag.description = data?.description
      newTag.ungroupedDescription = data?.ungroupedDescription
      newTag.group1 = data?.group1
      newTag.group2 = data?.group2
      newTag.group3 = data?.group3
      newTag.value = new Mongo.Double(data.value)
      newTag.valueString = data?.valueString
      newTag.valueJson = data?.valueJson
      newTag.transient = data?.transient === true
      newTag.invalid = data?.invalidAtSource === false ? false : true
      newTag.timeTagAtSource = data?.timeTagAtSource
      newTag.commissioningRemarks = data?.commissioningRemarks 

      let resIns = await rtDataCollection.insertOne(newTag)
      if (resIns.insertedCount === 1) ListCreatedTags.push(tag)
    } else {
      // found (already exists, no need to create), just list as created
      ListCreatedTags.push(tag)
    }
  }
}

module.exports = {
  NewTag: NewTag,
  GetAutoKeyInitialValue: GetAutoKeyInitialValue,
  AutoCreateTag: AutoCreateTag,
}
