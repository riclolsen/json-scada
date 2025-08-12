/* 
 * OPC-UA Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020-2025 - Ricardo L. Olsen
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

using System;
using MongoDB.Bson;
using MongoDB.Bson.Serialization;
using MongoDB.Bson.Serialization.Attributes;

partial class MainClass
{
    [BsonIgnoreExtraElements]
    public class rtDataId
    {
        [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
        public BsonDouble _id = 0.0;
    }
    [BsonIgnoreExtraElements]
    public class protocolDestination
    {
        [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
        public BsonDouble protocolDestinationConnectionNumber = 0.0;
        public BsonDouble protocolDestinationCommonAddress = 0.0;
        public BsonDouble protocolDestinationObjectAddress = 0.0;
        public BsonDouble protocolDestinationASDU = 0.0;
        public BsonDouble protocolDestinationCommandDuration = 0.0;
        public BsonBoolean protocolDestinationCommandUseSBO = false;
        public BsonDouble protocolDestinationKConv1 = 1.0;
        public BsonDouble protocolDestinationKConv2 = 0.0;
        public BsonDouble protocolDestinationGroup = 0.0;
        public BsonDouble protocolDestinationHoursShift = 0.0;

    }

    [BsonIgnoreExtraElements]
    public class rtData
    {
        [BsonDefaultValue(0.0)]
        public double _id { get; set; }

        [BsonDefaultValue(false)]
        public bool alarmDisabled { get; set; }

        [BsonDefaultValue(0.0)]
        public double alarmState { get; set; }

        [BsonDefaultValue(false)]
        public bool alarmed { get; set; }

        [BsonDefaultValue(false)]
        public bool alerted { get; set; }

        [BsonDefaultValue("")]
        public string alertState { get; set; }

        [BsonDefaultValue("")]
        public string annotation { get; set; }

        [BsonDefaultValue(false)]
        public bool commandBlocked { get; set; }

        [BsonDefaultValue(0.0)]
        public double commandOfSupervised { get; set; }

        [BsonDefaultValue("")]
        public string commissioningRemarks { get; set; }

        [BsonDefaultValue("")]
        public string description { get; set; }

        [BsonDefaultValue("")]
        public string eventTextFalse { get; set; }

        [BsonDefaultValue("")]
        public string eventTextTrue { get; set; }

        [BsonDefaultValue(0.0)]
        public double formula { get; set; }

        [BsonDefaultValue(false)]
        public bool frozen { get; set; }

        [BsonDefaultValue(0.0)]
        public double frozenDetectTimeout { get; set; }

        [BsonDefaultValue("")]
        public string group1 { get; set; }

        [BsonDefaultValue("")]
        public string group2 { get; set; }

        [BsonDefaultValue("")]
        public string group3 { get; set; }

        [BsonDefaultValue(double.MaxValue)]
        public double hiLimit { get; set; }

        [BsonDefaultValue(double.MaxValue)]
        public double hihiLimit { get; set; }

        [BsonDefaultValue(double.MaxValue)]
        public double hihihiLimit { get; set; }

        [BsonDefaultValue(0.0)]
        public double historianDeadBand { get; set; }

        [BsonDefaultValue(0.0)]
        public double historianPeriod { get; set; }

        [BsonDefaultValue(0.0)]
        public double hysteresis { get; set; }

        [BsonDefaultValue(true)]
        public bool invalid { get; set; }

        [BsonDefaultValue(60000.0)]
        public double invalidDetectTimeout { get; set; }

        [BsonDefaultValue(false)]
        public bool isEvent { get; set; }

        [BsonDefaultValue(1.0)]
        public double kconv1 { get; set; }

        [BsonDefaultValue(0.0)]
        public double kconv2 { get; set; }

        [BsonDefaultValue(-double.MaxValue)]
        public double loLimit { get; set; }

        [BsonDefaultValue(-double.MaxValue)]
        public double loloLimit { get; set; }

        [BsonDefaultValue(-double.MaxValue)]
        public double lololoLimit { get; set; }

        [BsonDefaultValue(null)]
        public BsonValue location;

        [BsonDefaultValue("")]
        public string notes { get; set; }

        [BsonDefaultValue("supervised")]
        public string origin { get; set; }

        [BsonDefaultValue(false)]
        public bool overflow { get; set; }

        [BsonDefaultValue(null)]
        public BsonValue parcels;

        [BsonDefaultValue(0.0)]
        public double priority { get; set; }

        [BsonDefaultValue(null)]
        public protocolDestination[] protocolDestinations { get; set; }

        [BsonDefaultValue("")]
        public string protocolSourceBrowsePath { get; set; }

        [BsonDefaultValue("")]
        public string protocolSourceAccessLevel { get; set; }

        [BsonDefaultValue("")]
        public string protocolSourceASDU { get; set; }

        [BsonDefaultValue(0.0)]
        public double protocolSourceCommandDuration { get; set; }

        [BsonDefaultValue(false)]
        public bool protocolSourceCommandUseSBO { get; set; }

        [BsonDefaultValue("")]
        public string protocolSourceCommonAddress { get; set; }

        [BsonDefaultValue(0.0)]
        public double protocolSourceConnectionNumber { get; set; }

        [BsonDefaultValue("")]
        public string protocolSourceObjectAddress { get; set; }

        [BsonDefaultValue(5.0)]
        public double protocolSourcePublishingInterval { get; set; }

        [BsonDefaultValue(2.0)]
        public double protocolSourceSamplingInterval { get; set; }

        [BsonDefaultValue(10.0)]
        public double protocolSourceQueueSize { get; set; }

        [BsonDefaultValue(true)]
        public bool protocolSourceDiscardOldest { get; set; }


        [BsonDefaultValue(null)]
        public BsonValue sourceDataUpdate { get; set; }

        [BsonDefaultValue("")]
        public string stateTextFalse { get; set; }

        [BsonDefaultValue("")]
        public string stateTextTrue { get; set; }

        [BsonDefaultValue(false)]
        public bool substituted { get; set; }

        [BsonDefaultValue(0.0)]
        public double supervisedOfCommand { get; set; }

        [BsonDefaultValue("")]
        public string tag { get; set; }

        [BsonDefaultValue(null)]
        public BsonValue timeTag { get; set; }
        [BsonDefaultValue(null)]
        public BsonValue timeTagAlarm { get; set; }
        [BsonDefaultValue(null)]
        public BsonValue timeTagAtSource { get; set; }

        [BsonDefaultValue(false)]
        public bool timeTagAtSourceOk { get; set; }

        [BsonDefaultValue(false)]
        public bool transient { get; set; }

        [BsonDefaultValue("digital")]
        public string type { get; set; }

        [BsonDefaultValue("")]
        public string ungroupedDescription { get; set; }

        [BsonDefaultValue("")]
        public string unit { get; set; }

        [BsonDefaultValue(0.0)]
        public double updatesCnt { get; set; }

        [BsonDefaultValue(0.0)]
        public double valueDefault { get; set; }

        [BsonDefaultValue("")]
        public string valueJson { get; set; }

        [BsonDefaultValue("")]
        public string valueString { get; set; }

        [BsonDefaultValue(0.0)]
        public double value { get; set; }

        [BsonDefaultValue(0.0)]
        public double zeroDeadband { get; set; }
    }
    public static rtData newRealtimeDoc(OPC_Value ov, double _id, double commandOfSupervised)
    {
        var type = "analog";
        switch (ov.asdu.ToLower())
        {
            case "boolean":
                type = "digital";
                break;
            case "string":
            case "bytestring":
            case "localeid":
            case "localizedtext":
            case "xmlelement":
            case "qualifiedname":
            case "guid":
                type = "string";
                break;
            case "nodeid":
            case "expandednodeid":
            case "extensionobject":
                type = "json";
                break;
        }
        if (ov.isArray)
            type = "json";

        if (ov.createCommandForMethod || ov.createCommandForSupervised)
        {
            return new rtData()
            {
                _id = _id,
                protocolSourceBrowsePath = ov.path,
                protocolSourceAccessLevel = Convert.ToString(ov.accessLevels),
                protocolSourceASDU = ov.asdu,
                protocolSourceCommonAddress = ov.common_address,
                protocolSourceConnectionNumber = ov.conn_number,
                protocolSourceObjectAddress = ov.address,
                protocolSourceCommandUseSBO = false,
                protocolSourceCommandDuration = 0.0,
                protocolSourcePublishingInterval = 0.0,
                protocolSourceSamplingInterval = 0.0,
                protocolSourceQueueSize = 0.0,
                protocolSourceDiscardOldest = true,
                alarmState = 2.0,
                description = ov.conn_name + "~" + ov.path + "~" + ov.display_name + "-Command",
                ungroupedDescription = ov.display_name,
                group1 = ov.conn_name,
                group2 = ov.path,
                group3 = "",
                stateTextFalse = type == "digital" ? "FALSE" : "",
                stateTextTrue = type == "digital" ? "TRUE" : "",
                eventTextFalse = type == "digital" ? "FALSE" : "",
                eventTextTrue = type == "digital" ? "TRUE" : "",
                origin = "command",
                tag = TagFromOPCParameters(ov) + ";cmd",
                type = type,
                value = 0.0,
                valueString = "",
                valueJson = "",
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
                protocolDestinations = new protocolDestination[] { },
                sourceDataUpdate = BsonNull.Value,
                supervisedOfCommand = ov.createCommandForSupervised ? _id + 1 : 0.0,
                substituted = false,
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
        else
        if (type == "digital")
            return new rtData()
            {
                _id = _id,
                protocolSourceBrowsePath = ov.path,
                protocolSourceAccessLevel = Convert.ToString(ov.accessLevels),
                protocolSourceASDU = ov.asdu,
                protocolSourceCommonAddress = ov.common_address,
                protocolSourceConnectionNumber = ov.conn_number,
                protocolSourceObjectAddress = ov.address,
                protocolSourceCommandUseSBO = false,
                protocolSourceCommandDuration = 0.0,
                protocolSourcePublishingInterval = 5.0,
                protocolSourceSamplingInterval = 2.0,
                protocolSourceQueueSize = 10.0,
                protocolSourceDiscardOldest = true,
                alarmState = 2.0,
                description = ov.conn_name + "~" + ov.path + "~" + ov.display_name,
                ungroupedDescription = ov.display_name,
                group1 = ov.conn_name,
                group2 = ov.path,
                group3 = "",
                stateTextFalse = "FALSE",
                stateTextTrue = "TRUE",
                eventTextFalse = "FALSE",
                eventTextTrue = "TRUE",
                origin = "supervised",
                tag = TagFromOPCParameters(ov),
                type = type,
                value = ov.value,
                valueString = "",
                valueJson = "",
                alarmDisabled = false,
                alerted = false,
                alarmed = false,
                alertState = "",
                annotation = "",
                commandBlocked = false,
                commandOfSupervised = commandOfSupervised,
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
                protocolDestinations = new protocolDestination[] { },
                sourceDataUpdate = BsonNull.Value,
                supervisedOfCommand = 0.0,
                substituted = false,
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
        if (type == "string")
            return new rtData()
            {
                _id = _id,
                protocolSourceBrowsePath = ov.path,
                protocolSourceAccessLevel = Convert.ToString(ov.accessLevels),
                protocolSourceASDU = ov.asdu,
                protocolSourceCommonAddress = ov.common_address,
                protocolSourceConnectionNumber = ov.conn_number,
                protocolSourceObjectAddress = ov.address,
                protocolSourceCommandUseSBO = false,
                protocolSourceCommandDuration = 0.0,
                protocolSourcePublishingInterval = 5.0,
                protocolSourceSamplingInterval = 2.0,
                protocolSourceQueueSize = 10.0,
                protocolSourceDiscardOldest = true,
                alarmState = -1.0,
                description = ov.conn_name + "~" + ov.path + "~" + ov.display_name,
                ungroupedDescription = ov.display_name,
                group1 = ov.conn_name,
                group2 = ov.path,
                group3 = "",
                stateTextFalse = "",
                stateTextTrue = "",
                eventTextFalse = "",
                eventTextTrue = "",
                origin = "supervised",
                tag = TagFromOPCParameters(ov),
                type = type,
                value = 0.0,
                valueString = ov.valueString,
                valueJson = "",

                alarmDisabled = false,
                alerted = false,
                alarmed = false,
                alertState = "",
                annotation = "",
                commandBlocked = false,
                commandOfSupervised = commandOfSupervised,
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
                protocolDestinations = new protocolDestination[] { },
                sourceDataUpdate = BsonNull.Value,
                supervisedOfCommand = 0.0,
                substituted = false,
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
        else
        if (type == "json")
            return new rtData()
            {
                _id = _id,
                protocolSourceBrowsePath = ov.path,
                protocolSourceAccessLevel = Convert.ToString(ov.accessLevels),
                protocolSourceASDU = ov.asdu,
                protocolSourceCommonAddress = ov.common_address,
                protocolSourceConnectionNumber = ov.conn_number,
                protocolSourceObjectAddress = ov.address,
                protocolSourceCommandUseSBO = false,
                protocolSourceCommandDuration = 0.0,
                protocolSourcePublishingInterval = 5.0,
                protocolSourceSamplingInterval = 2.0,
                protocolSourceQueueSize = 10.0,
                protocolSourceDiscardOldest = true,
                alarmState = -1.0,
                description = ov.conn_name + "~" + ov.path + "~" + ov.display_name,
                ungroupedDescription = ov.display_name,
                group1 = ov.conn_name,
                group2 = ov.path,
                group3 = "",
                stateTextFalse = "",
                stateTextTrue = "",
                eventTextFalse = "",
                eventTextTrue = "",
                origin = "supervised",
                tag = TagFromOPCParameters(ov),
                type = type,
                value = 0.0,
                valueString = ov.valueString,
                valueJson = ov.valueJson == null ? "" : ov.valueJson,

                alarmDisabled = false,
                alerted = false,
                alarmed = false,
                alertState = "",
                annotation = "",
                commandBlocked = false,
                commandOfSupervised = commandOfSupervised,
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
                protocolDestinations = new protocolDestination[] { },
                sourceDataUpdate = BsonNull.Value,
                supervisedOfCommand = 0.0,
                substituted = false,
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
            protocolSourceBrowsePath = ov.path,
            protocolSourceAccessLevel = Convert.ToString(ov.accessLevels),
            protocolSourceASDU = ov.asdu,
            protocolSourceCommonAddress = ov.common_address,
            protocolSourceConnectionNumber = ov.conn_number,
            protocolSourceObjectAddress = ov.address,
            protocolSourceCommandUseSBO = false,
            protocolSourceCommandDuration = 0.0,
            protocolSourcePublishingInterval = 5.0,
            protocolSourceSamplingInterval = 2.0,
            protocolSourceQueueSize = 10.0,
            protocolSourceDiscardOldest = true,
            alarmState = -1.0,
            description = ov.conn_name + "~" + ov.path + "~" + ov.display_name,
            ungroupedDescription = ov.display_name,
            group1 = ov.conn_name,
            group2 = ov.path,
            group3 = "",
            stateTextFalse = "",
            stateTextTrue = "",
            eventTextFalse = "",
            eventTextTrue = "",
            origin = "supervised",
            tag = TagFromOPCParameters(ov),
            type = type,
            value = ov.value,
            valueString = "",

            alarmDisabled = false,
            alerted = false,
            alarmed = false,
            alertState = "",
            annotation = "",
            commandBlocked = false,
            commandOfSupervised = commandOfSupervised,
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
            protocolDestinations = new protocolDestination[] { },
            sourceDataUpdate = BsonNull.Value,
            supervisedOfCommand = 0.0,
            substituted = false,
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
}
