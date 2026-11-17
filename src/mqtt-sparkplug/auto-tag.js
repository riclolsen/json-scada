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

const Mongo = require('mongodb')
const Log = require('./simple-logger')
const AppDefs = require('./app-defs')

const SparkplugNS = 'spBv1.0'
let AutoKeyId = 0
let AutoKeyMultiplier = 1000000 // should be more than estimated maximum points on a connection

function NewTag() {
  AutoKeyId++

  return {
    _id: new Mongo.Double(AutoKeyId),
    protocolSourceASDU: '',
    protocolSourceCommonAddress: '',
    protocolSourceConnectionNumber: new Mongo.Double(0),
    protocolSourceObjectAddress: '',
    protocolSourceCommandUseSBO: false,
    protocolSourceCommandDuration: new Mongo.Double(0),
    alarmState: new Mongo.Double(-1.0),
    alarmRange: new Mongo.Double(0.0),
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
    valueJson: {},
    alarmDisabled: false,
    alerted: false,
    alarmed: false,
    alertState: '',
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
    substituted: false,
    timeTag: null,
    timeTagAlarm: null,
    timeTagAtSource: null,
    timeTagAtSourceOk: false,
    transient: false,
    unit: '',
    updatesCnt: new Mongo.Double(0.0),
    valueDefault: new Mongo.Double(0.0),
    zeroDeadband: new Mongo.Double(0.0),
  }
}

// find biggest point key (_id) on range and adjust automatic key
async function GetAutoKeyInitialValue(rtCollection, configObj) {
  return refreshAutoKey(rtCollection, configObj.ConnectionNumber)
}

// the key never goes back: another node (redundancy) may have created tags
async function refreshAutoKey(rtCollection, connectionNumber) {
  const base = connectionNumber * AutoKeyMultiplier
  let resLastKey = await rtCollection
    .find(
      {
        _id: {
          $gt: base,
          $lt: (connectionNumber + 1) * AutoKeyMultiplier,
        },
      },
      { projection: { _id: 1 } }
    )
    .sort({ _id: -1 })
    .limit(1)
    .toArray()
  let last = base
  if (resLastKey.length > 0 && '_id' in resLastKey[0])
    last = parseInt(resLastKey[0]._id)
  AutoKeyId = Math.max(AutoKeyId, base, last)
  return AutoKeyId
}

const isDuplicateId = (e) =>
  e?.code === 11000 &&
  (e?.keyPattern?._id !== undefined || /_id_/.test(String(e?.message)))

const ListCreatedTags = new Set()

async function AutoCreateTag(data, connectionNumber, rtDataCollection) {
  let splitTopic = []
  let tag = AppDefs.AUTOTAG_PREFIX + ':' + connectionNumber + ':'
  let commonAddress = ''
  if (data.protocolSourceObjectAddress.indexOf(SparkplugNS + '/') === 0) {
    commonAddress = SparkplugNS
    splitTopic = data.protocolSourceObjectAddress.split('/')
    tag += data.protocolSourceObjectAddress.replace(SparkplugNS + '/', '')
  } else {
    tag += data.protocolSourceObjectAddress
  }

  if (!ListCreatedTags.has(tag)) {
    // possibly not created tag, must check
    let res = await rtDataCollection
      .find({
        protocolSourceConnectionNumber: connectionNumber,
        protocolSourceObjectAddress: data.protocolSourceObjectAddress,
      })
      .toArray()

    if ('length' in res && res.length === 0) {
      // not found, then create
      Log.log(
        'Auto Key - Tag not found, will create: ' + tag,
        Log.levelDetailed
      )
      let newTag = NewTag(data)
      newTag.protocolSourceConnectionNumber = new Mongo.Double(connectionNumber)
      newTag.protocolSourceObjectAddress = data.protocolSourceObjectAddress
      newTag.protocolSourceASDU = data?.metricType || data?.asduAtSource || ''
      newTag.protocolSourceCommonAddress = commonAddress
      newTag.tag = tag
      if ('type' in data) newTag.type = data.type
      if (newTag.type === 'analog') {
        newTag.unit = data?.unit || 'units'
      }
      if (newTag.type === 'digital') {
        newTag.eventTextFalse = 'false'
        newTag.eventTextTrue = 'true'
        newTag.stateTextFalse = 'false'
        newTag.stateTextTrue = 'true'
      }
      newTag.ungroupedDescription =
        data?.ungroupedDescription || data.protocolSourceObjectAddress
      newTag.group1 =
        data?.group1 || AppDefs.AUTOTAG_PREFIX + ':' + connectionNumber
      newTag.group2 = data?.group2 || splitTopic[1] || ''
      newTag.group3 =
        data?.group3 ||
        (splitTopic.length > 2 ?
          splitTopic[2] + (splitTopic.length > 4 ? '/' + splitTopic[3] : '')
        : '')
      newTag.description =
        data?.description ||
        newTag.group1 + '~' + newTag.group2 + '~' + newTag.ungroupedDescription
      if (data.protocolSourceObjectAddress.indexOf(SparkplugNS + '/') === 0) {
        newTag.description = newTag.description
          .replace(SparkplugNS + '/', '')
          .replace(data.group2 + '/', '')
        newTag.ungroupedDescription = splitTopic[splitTopic.length - 1]
      }
      let numValue = parseFloat(data.value)
      if (isNaN(numValue)) numValue = 0
      newTag.value = new Mongo.Double(numValue)
      newTag.valueString = data?.valueString || ''
      newTag.valueJson = data?.valueJson || {}
      newTag.transient = data?.transient === true
      newTag.invalid = data?.invalid === false ? false : true
      newTag.timeTagAtSource = data?.timeTagAtSource || null
      newTag.commissioningRemarks = data?.commissioningRemarks || ''
      newTag.priority = new Mongo.Double(3.0)

      // console.log('>> Insert ' + tag)

      // the key may already be used by tags created by the other node of a
      // redundant pair: refresh it from the database and retry
      let resIns = null
      for (let attempt = 0; ; attempt++) {
        try {
          resIns = await rtDataCollection.insertOne(newTag)
          break
        } catch (e) {
          if (!isDuplicateId(e) || attempt >= 2) throw e
          await refreshAutoKey(rtDataCollection, connectionNumber)
          newTag._id = NewTag()._id
          Log.log('Auto Key - Key in use, retrying with ' + newTag._id)
        }
      }
      if (resIns.acknowledged)
        ListCreatedTags.add(tag) // change for mongo driver >= 4.0
      else Log.log('Auto Key - Error inserting tag : ' + tag, Log.levelDetailed)
    } else {
      // found (already exists, no need to create), just list as created
      ListCreatedTags.add(tag)
    }
  }
}

module.exports = {
  NewTag: NewTag,
  GetAutoKeyInitialValue: GetAutoKeyInitialValue,
  AutoCreateTag: AutoCreateTag,
}
