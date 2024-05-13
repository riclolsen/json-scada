/* 
 * This software implements a IEC61850 driver for JSON SCADA.
 * Copyright - 2020-2024 - Ricardo Lastra Olsen
 * 
 * Requires libiec61850 from MZ Automation.
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

using System;
using MongoDB.Bson;
using MongoDB.Bson.Serialization;
using MongoDB.Bson.Serialization.Attributes;

namespace IEC61850_Client
{
    partial class MainClass
    {
        [BsonIgnoreExtraElements]
        public class rtDataId
        {
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble _id;
        }
        [BsonIgnoreExtraElements]
        public class rtData
        {
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble _id;
            [BsonDefaultValue(false)]
            public BsonBoolean alarmDisabled { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble alarmState;
            [BsonDefaultValue(false)]
            public BsonBoolean alarmed { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean alerted { get; set; }
            [BsonDefaultValue("")]
            public BsonString alertState { get; set; }
            [BsonDefaultValue("")]
            public BsonString annotation { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean commandBlocked { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble commandOfSupervised;
            [BsonDefaultValue("")]
            public BsonString commissioningRemarks { get; set; }
            [BsonDefaultValue("")]
            public BsonString description { get; set; }
            [BsonDefaultValue("")]
            public BsonString eventTextFalse { get; set; }
            [BsonDefaultValue("")]
            public BsonString eventTextTrue { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble formula;
            [BsonDefaultValue(false)]
            public BsonBoolean frozen { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble frozenDetectTimeout;
            [BsonDefaultValue("")]
            public BsonString group1 { get; set; }
            [BsonDefaultValue("")]
            public BsonString group2 { get; set; }
            [BsonDefaultValue("")]
            public BsonString group3 { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(Double.MaxValue)]
            public BsonDouble hiLimit;
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(Double.MaxValue)]
            public BsonDouble hihiLimit;
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(Double.MaxValue)]
            public BsonDouble hihihiLimit;
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble historianDeadBand;
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble historianPeriod;
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble hysteresis;
            [BsonDefaultValue(true)]
            public BsonBoolean invalid { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(60000.0)]
            public BsonDouble invalidDetectTimeout;
            [BsonDefaultValue(false)]
            public BsonBoolean isEvent { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(1.0)]
            public BsonDouble kconv1;
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble kconv2;
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(-Double.MaxValue)]
            public BsonDouble loLimit;
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(-Double.MaxValue)]
            public BsonDouble loloLimit;
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(-Double.MaxValue)]
            public BsonDouble lololoLimit;
            [BsonDefaultValue(null)]
            public BsonValue location;
            [BsonDefaultValue("")]
            public BsonString notes { get; set; }
            [BsonDefaultValue("supervised")]
            public BsonString origin { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean overflow { get; set; }
            [BsonDefaultValue(null)]
            public BsonValue parcels;
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble priority;
            [BsonDefaultValue(null)]
            public BsonValue protocolDestinations;
            [BsonDefaultValue("")]
            public BsonString protocolSourceASDU { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolSourceCommandDuration;
            [BsonDefaultValue(false)]
            public BsonBoolean protocolSourceCommandUseSBO { get; set; }
            [BsonDefaultValue("")]
            public BsonString protocolSourceCommonAddress { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolSourceConnectionNumber;
            [BsonDefaultValue("")]
            public BsonString protocolSourceObjectAddress { get; set; }

            [BsonDefaultValue(null)]
            public BsonValue sourceDataUpdate { get; set; }
            [BsonDefaultValue("")]
            public BsonString stateTextFalse { get; set; }
            [BsonDefaultValue("")]
            public BsonString stateTextTrue { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble supervisedOfCommand;
            [BsonDefaultValue("")]
            public BsonString tag { get; set; }
            [BsonDefaultValue(null)]
            public BsonValue timeTag { get; set; }
            [BsonDefaultValue(null)]
            public BsonValue timeTagAlarm { get; set; }
            [BsonDefaultValue(null)]
            public BsonValue timeTagAtSource { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean timeTagAtSourceOk { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean transient { get; set; }
            [BsonDefaultValue("digital")]
            public BsonString type { get; set; }
            [BsonDefaultValue("")]
            public BsonString ungroupedDescription { get; set; }
            [BsonDefaultValue("")]
            public BsonString unit { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble updatesCnt;
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble valueDefault;
            [BsonDefaultValue("")]
            public BsonString valueString { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble value;
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble zeroDeadband;
        }
        public static rtData newRealtimeDoc(IECValue iv, double _id)
        {
            var group1 = "IEC61850";
            if (iv.asdu.ToLower() == "boolean" || iv.isDigital)
                return new rtData()
                {
                    _id = _id,
                    protocolSourceASDU = iv.asdu,
                    protocolSourceCommonAddress = iv.common_address.ToUpper(),
                    protocolSourceConnectionNumber = iv.conn_number,
                    protocolSourceObjectAddress = iv.address,
                    protocolSourceCommandUseSBO = false,
                    protocolSourceCommandDuration = 0.0,
                    alarmState = 2.0,
                    description = group1 + "~" + iv.conn_name + "~" + iv.display_name,
                    ungroupedDescription = iv.display_name,
                    group1 = group1,
                    group2 = iv.conn_name,
                    group3 = iv.common_address,
                    stateTextFalse = "FALSE",
                    stateTextTrue = "TRUE",
                    eventTextFalse = "FALSE",
                    eventTextTrue = "TRUE",
                    origin = "supervised",
                    tag = TagFromParameters(iv),
                    type = "digital",
                    value = iv.value,
                    valueString = "????",
                    alarmDisabled = false,
                    alerted = false,
                    alarmed = false,
                    alertState = "",
                    annotation = "",
                    commandBlocked = false,
                    commandOfSupervised = 0.0,
                    commissioningRemarks = "",
                    formula = 0.0,
                    frozen = false,
                    frozenDetectTimeout = 0.0,
                    hiLimit = Double.MaxValue,
                    hihiLimit = Double.MaxValue,
                    hihihiLimit = Double.MaxValue,
                    historianDeadBand = 0.0,
                    historianPeriod = 0.0,
                    hysteresis = 0.0,
                    invalid = true,
                    invalidDetectTimeout = 60000,
                    isEvent = false,
                    kconv1 = 1.0,
                    kconv2 = 0.0,
                    location = BsonNull.Value,
                    loLimit = -Double.MaxValue,
                    loloLimit = -Double.MaxValue,
                    lololoLimit = -Double.MaxValue,
                    notes = "",
                    overflow = false,
                    parcels = BsonNull.Value,
                    priority = 0.0,
                    protocolDestinations = BsonNull.Value,
                    sourceDataUpdate = BsonNull.Value,
                    supervisedOfCommand = 0.0,
                    timeTag = BsonNull.Value,
                    timeTagAlarm = BsonNull.Value,
                    timeTagAtSource = BsonNull.Value,
                    timeTagAtSourceOk = false,
                    transient = false,
                    unit = "",
                    updatesCnt = 0,
                    valueDefault = 0.0,
                    zeroDeadband = 0.0
                };
            else
            if (iv.asdu.ToLower() == "string" || iv.asdu.ToLower() == "extensionobject")
                return new rtData()
                {
                    _id = _id,
                    protocolSourceASDU = iv.asdu,
                    protocolSourceCommonAddress = iv.common_address.ToUpper(),
                    protocolSourceConnectionNumber = iv.conn_number,
                    protocolSourceObjectAddress = iv.address,
                    protocolSourceCommandUseSBO = false,
                    protocolSourceCommandDuration = 0.0,
                    alarmState = -1.0,
                    description = group1 + "~" + iv.conn_name + "~" + iv.display_name,
                    ungroupedDescription = iv.display_name,
                    group1 = group1,
                    group2 = iv.conn_name,
                    group3 = iv.common_address,
                    stateTextFalse = "",
                    stateTextTrue = "",
                    eventTextFalse = "",
                    eventTextTrue = "",
                    origin = "supervised",
                    tag = TagFromParameters(iv),
                    type = "string",
                    value = 0.0,
                    valueString = iv.valueString,

                    alarmDisabled = false,
                    alerted = false,
                    alarmed = false,
                    alertState = "",
                    annotation = "",
                    commandBlocked = false,
                    commandOfSupervised = 0.0,
                    commissioningRemarks = "",
                    formula = 0.0,
                    frozen = false,
                    frozenDetectTimeout = 0.0,
                    hiLimit = Double.MaxValue,
                    hihiLimit = Double.MaxValue,
                    hihihiLimit = Double.MaxValue,
                    historianDeadBand = 0.0,
                    historianPeriod = 0.0,
                    hysteresis = 0.0,
                    invalid = true,
                    invalidDetectTimeout = 60000,
                    isEvent = false,
                    kconv1 = 1.0,
                    kconv2 = 0.0,
                    location = BsonNull.Value,
                    loLimit = -Double.MaxValue,
                    loloLimit = -Double.MaxValue,
                    lololoLimit = -Double.MaxValue,
                    notes = "",
                    overflow = false,
                    parcels = BsonNull.Value,
                    priority = 0.0,
                    protocolDestinations = BsonNull.Value,
                    sourceDataUpdate = BsonNull.Value,
                    supervisedOfCommand = 0.0,
                    timeTag = BsonNull.Value,
                    timeTagAlarm = BsonNull.Value,
                    timeTagAtSource = BsonNull.Value,
                    timeTagAtSourceOk = false,
                    transient = false,
                    unit = "",
                    updatesCnt = 0,
                    valueDefault = 0.0,
                    zeroDeadband = 0.0,
                };

            return new rtData()
            {
                _id = _id,
                protocolSourceASDU = iv.asdu,
                protocolSourceCommonAddress = iv.common_address.ToUpper(),
                protocolSourceConnectionNumber = iv.conn_number,
                protocolSourceObjectAddress = iv.address,
                protocolSourceCommandUseSBO = false,
                protocolSourceCommandDuration = 0.0,
                alarmState = -1.0,
                description = group1 + "~" + iv.conn_name + "~" + iv.display_name,
                ungroupedDescription = iv.display_name,
                group1 = group1,
                group2 = iv.conn_name,
                group3 = iv.common_address,
                stateTextFalse = "",
                stateTextTrue = "",
                eventTextFalse = "",
                eventTextTrue = "",
                origin = "supervised",
                tag = TagFromParameters(iv),
                type = "analog",
                value = iv.value,
                valueString = "????",

                alarmDisabled = false,
                alerted = false,
                alarmed = false,
                alertState = "",
                annotation = "",
                commandBlocked = false,
                commandOfSupervised = 0.0,
                commissioningRemarks = "",
                formula = 0.0,
                frozen = false,
                frozenDetectTimeout = 0.0,
                hiLimit = Double.MaxValue,
                hihiLimit = Double.MaxValue,
                hihihiLimit = Double.MaxValue,
                historianDeadBand = 0.0,
                historianPeriod = 0.0,
                hysteresis = 0.0,
                invalid = true,
                invalidDetectTimeout = 60000,
                isEvent = false,
                kconv1 = 1.0,
                kconv2 = 0.0,
                location = BsonNull.Value,
                loLimit = -Double.MaxValue,
                loloLimit = -Double.MaxValue,
                lololoLimit = -Double.MaxValue,
                notes = "",
                overflow = false,
                parcels = BsonNull.Value,
                priority = 0.0,
                protocolDestinations = BsonNull.Value,
                sourceDataUpdate = BsonNull.Value,
                supervisedOfCommand = 0.0,
                timeTag = BsonNull.Value,
                timeTagAlarm = BsonNull.Value,
                timeTagAtSource = BsonNull.Value,
                timeTagAtSourceOk = false,
                transient = false,
                unit = "",
                updatesCnt = 0,
                valueDefault = 0.0,
                zeroDeadband = 0.0
            };
        }
        static string TagFromParameters(IECValue iv)
        {
            return "IEC61850;" + iv.conn_name + ";" + iv.address + "[" + iv.common_address + "]";
        }
    }
}