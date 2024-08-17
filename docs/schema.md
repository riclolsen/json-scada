# MongoDB Schema Documentation

The JSON-SCADA MongoDB database is comprised of the following collections.

* _realtimeData_ - This is the table that contains point information, tag names, point attributes and realtime values.
* _protocolDriverInstances_ - Configuration of protocol drivers instances. See specific protocol for documentation.
* _protocolConnections_ - Configuration of protocol connections. See specific protocol for documentation.
* _commandsQueue_ - Queue for commands.
* _soeData_ - Sequence of Events data. This is a Capped Collection, it has a limited size.
* _processInstances_ - Configuration and information about JSON-SCADA instances of processes.

Please notice that all numeric fields from the schema is recorded as BSON Doubles (64 bit floating point). However, some numeric fields are expected to contain only integer values. When numbers are updated by the Mongo Shell manually, all numeric data is converted to BSON Doubles by default. Some languages like Node.js can cause values to be stored as integers or doubles depending on the current value. It is important that values are always stored as BSON Doubles as otherwise problems may be encountered by protocol drivers, specially those programmed in C#/DotNet Core.

All string parameters are UTF-8 encoded.

Dates are stored as MongoDB BSON Dates (UTC).

## _realtimeData_ collection

Example document.

    {
        "_id": 3285,
        "alarmDisabled": false,
        "alarmRange": 0,
        "alarmState": -1,
        "alarmed": false,
        "alerted": false,
        "alertState": "ok",
        "annotation": "",
        "commandBlocked": false,
        "commandOfSupervised": 0,
        "commissioningRemarks": "",
        "description": "KAW2~FD21 13,8kV~Active power",
        "eventTextFalse": "undefined",
        "eventTextTrue": "undefined",
        "formula": 0,
        "frozen": false,
        "frozenDetectTimeout": 300,
        "group1": "KAW2",
        "group2": "FD21 13,8kV",
        "group3": "",
        "hiLimit": null,
        "hihiLimit": null,
        "hihihiLimit": null,
        "historianDeadBand": 0,
        "historianPeriod": 0,
        "historianLastValue": null,
        "hysteresis": 0,
        "invalid": false,
        "invalidDetectTimeout": 300,
        "isEvent": false,
        "kconv1": 1,
        "kconv2": 0,
        "loLimit": null,
        "location": null,
        "loloLimit": null,
        "lololoLimit": null,
        "notes": "",
        "origin": "supervised",
        "overflow": false,
        "parcels": null,
        "priority": 0,
        "protocolSourceASDU": 13,
        "protocolSourceCommandDuration": null,
        "protocolSourceCommandUseSBO": null,
        "protocolSourceCommonAddress": 1,
        "protocolSourceConnectionNumber": 61,
        "protocolSourceObjectAddress": 3285,
        "sourceDataUpdate": {
            "valueAtSource": 2.9959075450897217,
            "valueStringAtSource": "2.9959075450897217",
            "valueJsonAtSource": {},
            "valueBsonAtSource": null,
            "asduAtSource": "M_ME_NC_1",
            "causeOfTransmissionAtSource": "3",
            "timeTagAtSource": null,
            "timeTagAtSourceOk": false,
            "timeTag": { "$date": "2020-08-10T19:04:59.774Z" },
            "notTopicalAtSource": false,
            "invalidAtSource": false,
            "overflowAtSource": false,
            "blockedAtSource": false,
            "substitutedAtSource": false
        },
        "stateTextFalse": "undefined",
        "stateTextTrue": "undefined",
        "substituted": false,
        "supervisedOfCommand": 0,
        "tag": "KAW2AL-21MTWT",
        "timeTag": { "$date": "2020-08-10T19:04:59.785Z" },
        "timeTagAlarm": { "$date": "2020-07-13T20:24:54.126Z" },
        "timeTagAlertState": { "$date": "2020-08-10T19:04:60.000Z" },
        "timeTagAtSource": null,
        "timeTagAtSourceOk": null,
        "transient": false,
        "type": "analog",
        "ungroupedDescription": "Active power",
        "unit": "MW",
        "updatesCnt": 86580,
        "valueDefault": 3.1,
        "valueString": "2.9959 MW",
        "valueJson": {},
        "protocolDestinations": [
            {
            "protocolDestinationConnectionNumber": 1001,
            "protocolDestinationCommonAddress": 1,
            "protocolDestinationObjectAddress": 3285,
            "protocolDestinationASDU": 13,
            "protocolDestinationCommandDuration": 0,
            "protocolDestinationCommandUseSBO": false,
            "protocolDestinationKConv1": 1,
            "protocolDestinationKConv2": 0,
            "protocolDestinationGroup": 0,
            "protocolDestinationHoursShift": -2
            }
        ],
        "value": 2.9959075450897217,
        "zeroDeadband": 0
    }
    

### Configuration fields

* _**_id_**_ [Double] - Numeric key for the point. This is stored as a BSON Double but should only contain integer values. Must be unique for the collection. **Mandatory parameter**.
* _**_tag_**_ [String] - String key for the point. It must begin with a letter char (A-Z or a-z) or underscore. Allowed symbols are underscore, dash and dot. Do not use spaces or symbols like #/|*!^~%$. Tags beginning with "_System." are reserved for internal system data. There is no enforced limit for the size but we recommend to keep it below 30 characters to make displays more readable. Must be unique for the collection. **Mandatory parameter**.
* _**_type_**_ [String] - Data type. Can be "digital", "analog", "string" or "json". **Mandatory parameter**.
* _**_origin_**_ [String] - How the value is obtained. Can be "supervised", "calculated", "manual", or "command". **Mandatory parameter**.
* _**_description_**_ [String] - Complete textual description of the tag information. **Mandatory parameter**.
* _**_ungroupedDescription_**_ [String] - Textual description leave out grouping.
* _**_group1_**_ [String] - Main group (highest level). E.g. station or installation name. **Mandatory parameter**.
* _**_group2_**_ [String] - Secondary grouping. E.g. bay or area name. **Mandatory parameter**.
* _**_group3_**_ [String] - Lowest level grouping. E.g. device ir equipment name. **Mandatory parameter**.
* _**_valueDefault_**_ [Double] - Numeric default value. **Mandatory parameter**.
* _**_priority_**_ [String] - Alarm priority: 0=highest, 9=lowest. **Mandatory parameter**.
* _**_frozenDetectTimeout_**_ [Double] -  Time in seconds to detect frozen (not changing) analog value. Use zero to never detect. **Mandatory parameter**.
* _**_invalidDetectTimeout_**_ [Double] - Time in seconds to detect invalid/old value when not updating. All supervised values expire, do not use zero for supervised values. This timeout should be at least more than the station integrity interrogation period. **Mandatory parameter**.
* _**_historianDeadBand_**_ [Double] - Absolute dead band parameter for historian. Does not affect non analog tags. **Mandatory parameter**.
* _**_historianPeriod_**_ [Double] - Period of integrity recording on historian. Currently only values 0 and -1 are supported. Value 0.0 will not record by integrity. Value -1 will remove tag from historian. **Mandatory parameter**.
* _**_commandOfSupervised_**_ [Double] - Key (\_id) pointing to the command point related to a supervised point. Only meaningful for _origin=supervised_ points (put zero here for other origins). Put value zero for this parameter when the supervised point does not have a related command. **Mandatory parameter**.
* _**_supervisedOfCommand_**_ [Double] - Key (\_id) pointing to a supervised point related to a command point (tag where the command feedback manifests). Only meaningful for _origin=command_ points (put zero here for other origins). Put value zero for this parameter when the command point does not have a related supervised (not recommended as this is a blind command with no feedback for the user). **Mandatory parameter**.
* _**_location_**_ [GeoJSON] - Reserved for location coordinates. Currently not in use. Can be null. **Mandatory parameter**.
* _**_isEvent_**_ [Boolean] - Flag meaning that only transitions OFF->ON for _type=digital_ matters for alarms and SOE (commonly used for electrical protection events). For _type=analog_ values it means that all valid changes of values should be recorded as SOE (future use). **Mandatory parameter**.
* _**_unit_**_ [String] - Unit of measurement when _type=analog_. **Mandatory parameter**.
* _**_alarmState_**_ [Double] - Considered state for alarm (0=off=false, 1=on=true, 2=both states, 3=state OFF->ON transition, -1=no state produces alarms but alarms can be signaled by other means) when _type=digital_. **Mandatory parameter**.
* _**_stateTextTrue_**_ [String] - Text for state true (numeric value not zero) when _type=digital_. Normally expressed as present tense (e.g. "ON"). **Mandatory parameter**.
* _**_stateTextFalse_**_ [String] - Text for state false (numeric value zero) when _type=digital_. Normally expressed as present tense (e.g. "OFF").  **Mandatory parameter**.
* _**_eventTextTrue_**_ [String] - Text for state change false to true when _type=digital_. Normally expressed as past tense (e.g. "Switched ON"). **Mandatory parameter**.
* _**_eventTextFalse_**_ [String] - Text for state change true to false when _type=digital_. Normally expressed as present tense (e.g. "Switched ON").  **Mandatory parameter**.
* _**_formula_**_ [Double] - A formula code for calculation of value. See the [Calculations](../src/calculations/README.md) section for documentation. Only meaningful when _origin=calculated_. Can be null for other origins. **Mandatory parameter**.
* _**_parcels_**_ [Array of Double] - Numeric key references to parcel points for calculations. Only meaningful when _origin=calculated_. Can be null for other origins. **Mandatory parameter**.
* _**_kconv1_**_ [Double] - Conversion factor 1 (multiplier). Applied when _origin=supervised_, _origin=command_ or _origin=calculated_. Use -1 to invert states of digital values and commands. **Mandatory parameter**.
* _**_kconv2_**_ [Double] - Conversion factor 2 (adder). Applied when _origin=supervised_ or _origin=calculated_. **Mandatory parameter**.
* _**_zeroDeadband_**_ [Double] - When acquired value is below this deadband it will be zeroed. Only meaningful for _type=analog_. **Mandatory parameter**.
* _**_protocolSourceConnectionNumber_**_ [Double] - Indicates the protocol connection that can updated the point. Should contain only integer values. Only meaningful when _origin=supervised_ or _origin=command_. **Mandatory parameter**.
* _**_protocolSourceCommonAddress_**_ [Double or String] - Protocol common address (device address). Only meaningful when _origin=supervised_ or _origin=command_. See protocol documentation. **Mandatory parameter**.
* _**_protocolSourceObjectAddress_**_ [Double or String] -  Protocol object address. Only meaningful when _origin=supervised_ or _origin=command_. See protocol documentation. **Mandatory parameter**.
* _**_protocolSourceASDU_**_ [Double or String] - Protocol information ASDU TI type. Only meaningful when _origin=supervised_ or _origin=command_. See protocol documentation. **Mandatory parameter**.
* _**_protocolSourceCommandDuration_**_ [Double] - Additional command specification. Only meaningful when _origin=command_. See protocol documentation. **Mandatory parameter**.
* _**_protocolSourceCommandUseSBO_**_ [Boolean] - Use or not Select Before Operate for commands. Only meaningful when _origin=command_. See protocol documentation. **Mandatory parameter**.
* _**_protocolDestinations_**_ [Array of Objects] - List of protocol destinations for server protocol connections. Can be null or empty array when not point is not to be distributed. See protocol documentation. **Mandatory parameter**.
* _**_protocolDestinationConnectionNumber_**_ [Double] - Indicates the protocol connection that will monitor updates to the point. Should contain only integer values. **Mandatory parameter**.
* _**_protocolDestinationCommonAddress_**_ [Double or String] - Protocol common address (device address). See protocol documentation. **Mandatory parameter**.
* _**_protocolDestinationObjectAddress_**_ [Double or String] -  Protocol object address. See protocol documentation. **Mandatory parameter**.
* _**_protocolDestinationASDU_**_ [Double or String] - Protocol information ASDU TI type. See protocol documentation. **Mandatory parameter**.
* _**_protocolDestinationCommandDuration_**_ [Double] - Additional command specification. See protocol documentation. **Mandatory parameter**.
* _**_protocolDestinationCommandUseSBO_**_ [Boolean] - Use or not Select Before Operate for commands. See protocol documentation. **Mandatory parameter**.
* _**_protocolDestinationKConv1_**_ [Double] - Conversion factor 1 (multiplier). **Mandatory parameter**.
* _**_protocolDestinationKConv2_**_ [Double] - Conversion factor 2 (adder). **Mandatory parameter**.
* _**_protocolDestinationGroup_**_ [Double or String] - Group number or dataset id of points. See protocol documentation. **Mandatory parameter**.
* _**_protocolDestinationHoursShift_**_ [Double] - Number of hours to add to timestamps. **Mandatory parameter**.

### Fields that can be configured and altered by the operators.

* _**_hiLimit_**_ [Double] - High limit for out-of-range alarm. Use null, Infinity or a big value to avoid alarm. Only meaningful for _type=analog_. **Mandatory parameter**.
* _**_hihiLimit_**_ [Double] - High-high limit for out-of-range alarm. Use null,Infinity or a big value to avoid alarm. Only meaningful for _type=analog_. Currently not used. **Mandatory parameter**.
* _**_hihihiLimit_**_ [Double] - High-high-high limit for out-of-range alarm. Use null, Infinity or a big value to avoid alarm. Only meaningful for _type=analog_. Currently not used. **Mandatory parameter**.
* _**_loLimit_**_ [Double] - Low limit for out-of-range alarm. Use null, -Infinity or a big negative value to avoid alarm. Only meaningful for _type=analog_. **Mandatory parameter**.
* _**_loloLimit_**_ [Double] - Low-low limit for out-of-range alarm. Use null, -Infinity or a big negative value to avoid alarm. Only meaningful for _type=analog_. Currently not used. **Mandatory parameter**.
* _**_lololoLimit_**_ [Double] - Low-low-low limit for out-of-range alarm. Use -Infinity or a big negative value to avoid alarm. Only meaningful for _type=analog_. Currently not used. **Mandatory parameter**.
* _**_hysteresis_**_ [Double] - Hysteresis (maximum absolute value variation that will not produce out-of-range alarms) for limits verification. Only meaningful for _type=analog_. **Mandatory parameter**.
* _**_substituted_**_ [Boolean] - When true, indicates that the value is substituted locally by the operator. **Mandatory parameter**.
* _**_alarmDisabled_**_ [Boolean] - When true, indicates that alarms are disabled for the point. **Mandatory parameter**.
* _**_annotation_**_ [String] - Blocking annotation text (reason command for blocking). **Mandatory parameter**.
* _**_commandBlocked_**_ [Boolean] - When true, the command is disabled by the operator. **Mandatory parameter**.
* _**_notes_**_ [String] - Documental notes text about the point. **Mandatory parameter**.
* _**_commissioningRemarks_**_ [String] - Remarks about the point commissioning. **Mandatory parameter**.

### Fields updated by the system.

* _**_alarmed_**_ [Boolean] - When true means the point is alarmed. **Mandatory parameter**.
* _**_alarmRange_**_ [Double] - Current alarm range for analog tags. 0=normal, 1=hiLimit violated, -1=loLimit violated. **Mandatory parameter**.
* _**_alerted_**_ [Boolean] - When true means the point is alerted (Grafana alert). **Optional parameter**.
* _**_alertState_**_ [Boolean] - Grafana alert state name. **Optional parameter**.
* _**_historianLastValue_**_ [Double] - Last value sent to historian (for dead band processing). Only for analog tags. **Mandatory parameter**.
* _**_invalid_**_ [Boolean] - When true value is considered old or not trusted. **Mandatory parameter**.
* _**_overflow_**_ [Boolean] - Overflow detected for _type=analog_ value. **Mandatory parameter**.
* _**_transient_**_ [Boolean] - Flags a transient value. **Mandatory parameter**.
* _**_frozen_**_ [Boolean] - When true, value is considered frozen (not changing). **Mandatory parameter**.
* _**_value_**_ [Double] - Current value as a number. **Mandatory parameter**.
* _**_valueJson_**_ [String] - Current value as JSON. **Optional parameter**.
* _**_valueString_**_ [String] - Current value as a string. **Mandatory parameter**.
* _**_timeTag_**_ [Date] - Last update time. **Mandatory parameter**.
* _**_timeTagAlarm_**_ [Date] - Last alarm time (when alarmed). **Mandatory parameter**.
* _**_timeTagAlertState_**_ [Date] - Time of last Grafana alert state update. **Optional parameter**.
* _**_timeTagAtSource_**_ [Date] - Timestamp from the source. **Mandatory parameter**.
* _**_timeTagAtSourceOk_**_ [Boolean] - When true, the source timestamp is * considered ok. **Mandatory parameter**.
* _**_updatesCnt_**_ [Double] - Count of updates. **Mandatory parameter**.
* _**_sourceDataUpdate_**_ [Object] - Information updated by protocol driver or calculation process.

#### The sourceDataUpdate object

* _**_sourceDataUpdate.asduAtSource_**_ [String] - Protocol ASDU/TI type.
* _**_sourceDataUpdate.causeOfTransmissionAtSource_**_ [String] - Cause of transmission. E.g. For IEC60870-5-104, "3"=Spontaneous, "20"=Station Interrogation.
* _**_sourceDataUpdate.notTopicalAtSource_**_ [Boolean] - When true means old value at source.
* _**_sourceDataUpdate.invalidAtSource_**_ [Boolean] - When true means invalid (not trusted) value at source.
* _**_sourceDataUpdate.blockedAtSource_**_ [Boolean] - When true means value is blocked at source.
* _**_sourceDataUpdate.substitutedAtSource_**_ [Boolean] - When true means the value is replaced at source.
* _**_sourceDataUpdate.carryAtSource_**_ [Boolean] - Flags a counter carry at source.
* _**_sourceDataUpdate.overflowAtSource_**_ [Boolean] - Flags a overflow of value at source.
* _**_sourceDataUpdate.transientAtSource_**_ [Boolean] - Flags a transient value at source.
* _**_sourceDataUpdate.valueAtSource_**_ [Double] - Current numeric value at source.
* _**_sourceDataUpdate.valueStringAtSource_**_ [String] - Current string value at source.
* _**_sourceDataUpdate.valueJsonAtSource_**_ [String] - Current JSON value at source.
* _**_sourceDataUpdate.valueBsonAtSource_**_ [Object] - Current value at source as a Javascript object.
* _**_sourceDataUpdate.timeTagAtSource_**_ [Date] - Source timestamp.
* _**_sourceDataUpdate.timeTagAtSourceOk_**_ [Boolean] - Source timestamp ok.
* _**_sourceDataUpdate.timeTag_**_ [Date] - Local update time.

## _commandsQueue_ collection

This collection has no configuration, it is written by the system when user issue commands in the UI. Commands are inserted here to be dispatched by protocols drivers. Protocol acknowledgement information is updated here also by protocol drivers.

Custom applications can also create commands to be dispatched by protocol drivers if desired.

Commands are processed only for new insertions. Old documents are preserved and can only be manually removed.

Example document

    {
        "_id":{
            "$oid":"5f1098dfd0ea7b5d01d6045c"
        },
        "protocolSourceConnectionNumber":61,
        "protocolSourceCommonAddress":1,
        "protocolSourceObjectAddress":64139,
        "protocolSourceASDU":45,
        "protocolSourceCommandDuration":0,
        "protocolSourceCommandUseSBO":false,
        "pointKey":64139,
        "tag":"KAW2KPR21XCBR5217----K",
        "timeTag":{
            "$date":"2020-07-16T18:13:51.182Z"
        },
        "value":1,
        "valueString":"1",
        "originatorUserName":"Protocol connection: IEC104DIST",
        "originatorIpAddress":"127.0.0.1:58446 127.0.0.1:58446 ",
        "delivered":true,
        "ack":true,
        "ackTimeTag":{
            "$date":"2020-07-16T18:13:51.304Z"
        }
    }

* _**__id_**_ [ObjectId] - MongoDB document id.
* _**_protocolSourceConnectionNumber_**_ [Double] - Indicates the protocol connection that will dispatch the command. Should contain only integer values.
* _**_protocolSourceCommonAddress_**_ [Double] - Protocol common address (device address). See specific protocol documentation.
* _**_protocolSourceObjectAddress_**_ [Double or String] - Protocol object address. See specific protocol documentation.
* _**_protocolSourceASDU_**_ [Double or String] - Protocol information ASDU type. See specific protocol documentation.
* _**_protocolSourceCommandDuration_**_ [Double] - Additional command specification. See specific protocol documentation.
* _**_protocolSourceCommandUseSBO_**_ [Boolean] - When true means it is desired to use Select-Before-Operate sequence.
* _**_pointKey_**_ [Double] - Numeric key of the point (link to _id field of _realtimeData_ collection).
* _**_tag_**_ [String] - Point tag name of event.
* _**_timeTag_**_ [Date] - Timestamp for the insertion of the command document.
* _**_value_**_ [Double] - Numeric value for the command.
* _**_valueString_**_ [String] - String text for the command.
* _**_originatorUserName_**_ [String] - Name of command originator process and user name.
* _**_originatorIpAddress_**_ [String] - IP address of originator.
* _**_delivered_**_ [Boolean] - When true means the protocol driver consumed and dispatched the command.
* _**_ack_**_ [Boolean] - The value means the protocol driver received true=positive or false=negative confirmation for the dispatched command. This property is to be inserted by the consuming protocol driver.
* _**_ackTimeTag_**_ [Date] - Timestamp of the ack insertion.
* _**_cancelReason_**_ [String] - Text description of cancel reason (when the command is cancelled).
* _**_resultDescription_**_ [String] - Text description of the command result (when provided by the protocol).

## _soeData_ collection

Here are stored Sequence of Events (SOE) information for digital values with source timestamps.

This is a Capped Collection, it has a limited size. Old documents are overwritten when the maximum sized is reached. Data here is only written for digital states when the protocol provides a source timestamp (like for IEC 60870-5-104 type 30).

    {
        "_id":{
            "$oid":"5f3427575afe8a451246eb4e"
        },
        "tag":"KAW2IB1-bRPRT----CmFl",
        "pointKey":2742,
        "group1":"KAW2",
        "description":"KAW2~IB1 13,8kV~Protection-Communic.Failure",
        "eventText":"COMM FAILURE",
        "invalid":false,
        "priority":3,
        "timeTag":{
            "$date":"2020-08-12T17:31:03.702Z"
        },
        "timeTagAtSource":{
            "$date":"2020-08-12T13:31:03.499Z"
        },
        "timeTagAtSourceOk":true,
        "ack":0
    }

* _**__id_**_ [ObjectId] - MongoDB document id.
* _**_tag_**_ [String] - Point tag name of event.
* _**_pointKey_**_ [Int32] - Numeric key of the point (link to _id field of _realtimeData_ collection).
* _**_group1_**_ [String] - Highest level grouping.
* _**_description_**_ [String] - Full description of monitored information.
* _**_eventText_**_ [String] - Text related to the event status change.
* _**_invalid_**_ [Boolean] - When true means the status change is not trusted to be ok.
* _**_priority_**_ [Int32] - Priority of the point, 0 (highest) - 9 (lowest)
* _**_timeTag_**_ [Date] - Timestamp for the arrival of information.
* _**_timeTagAtSource_**_ [Date] - Timestamp for the change stamped by the source device (RTU/IED).
* _**_timeTagAtSourceOk_**_ [Boolean] - When true means the source timestamp is considered ok.
* _**_ack_**_ [Int32] - Operator acknowledgement (0=not acknowledged, 1=acknowledged, 2=eliminated from lists).

## _processInstances_ collection

This collection must be configured when some process requires more than one instance. Also it can be used to restrict nodes that can connect to the database by filling the _nodeNames_ array. For this collection, the pair processName/processInstanceNumber should not repeat (there is a unique index for those fields combined to prevent this kind of error).

### _processInstances_ entry for the _CS_DATA_PROCESSOR_ module

Example document for the _CS_DATA_PROCESSOR_ module. Currently, this process supports just one redundant instance. There is no need to configure this document for this module as it can create the entry automatically when one is not found.

    {
        "_id":{
            "$oid":"1d3427575afe8a451246eb23"
        },
        processName: "CS_DATA_PROCESSOR",
        processInstanceNumber: 1,
        enabled: true,
        logLevel: 1,
        nodeNames: [], 
        activeNodeName: "mainNode",
        activeNodeKeepAliveTimeTag: { "$date": "2020-08-11T21:04:59.678Z" },
        softwareVersion: "0.1.1",
        latencyAvg: 123.2,
        latencyAvgMinute: 89.1,
        latencyPeak: 240.12
    }

* _**__id_**_ [ObjectId] - MongoDB document id.
* _**_processName_**_ [String] - Process name ("CS_DATA_PROCESSOR" or "CALCULATIONS")
* _**_instanceNumber_**_ [Double] - Process instance number.
* _**_enabled_**_ [Boolean] - When true, this instance is enabled.
* _**_logLevel_**_ [Double] - Log level (0=min, 3=max).
* _**_nodeNames_**_ [Array of String] - Names of allowed nodes. If null or empty any node is allowed.
* _**_activeNodeName_**_ [String] - Name of the current active node for this process instance.
* _**_activeNodeKeepAliveTimeTag_**_ [Date] - Keep-alive for the active node.
* _**_softwareVersion_**_ [String] - Software version of the process.
* _**_latencyAvg_**_ [Double] - Average latency in ms (only for CS_DATA_PROCESSOR).
* _**_latencyAvgMinute_**_ [Double] - Average latency on a minute in ms (only for CS_DATA_PROCESSOR).
* _**_latencyPeak_**_ [Double] - Peak latency (only for CS_DATA_PROCESSOR).

### _processInstances_ entry for the _CALCULATIONS_ module

Example document for the _CALCULATIONS_ module. Currently, this process supports just one redundant instance. There is no need to configure this document for this module as it can create the entry automatically when one is not found.

    {
        "_id":{
            "$oid":"1d3427575afe8a451246eb24"
        },
        processName: "CALCULATIONS",
        processInstanceNumber: 1,
        enabled: true,
        logLevel: 1,
        nodeNames: [], 
        activeNodeName: "mainNode",
        activeNodeKeepAliveTimeTag: { "$date": "2020-08-11T21:04:59.678Z" },
        softwareVersion: "0.1.1",
        periodOfCalculation: 2.0
    }

* _**__id_**_ [ObjectId] - MongoDB document id.
* _**_processName_**_ [String] - Process name ("CS_DATA_PROCESSOR" or "CALCULATIONS")
* _**_instanceNumber_**_ [Double] - Process instance number.
* _**_enabled_**_ [Boolean] - When true, this instance is enabled.
* _**_logLevel_**_ [Double] - Log level (0=min, 3=max).
* _**_nodeNames_**_ [Array of String] - Names of allowed nodes. If null or empty any node is allowed.
* _**_activeNodeName_**_ [String] - Name of the current active node for this process instance.
* _**_activeNodeKeepAliveTimeTag_**_ [Date] - Keep-alive for the active node.
* _**_softwareVersion_**_ [String] - Software version of the process.
* _**_periodOfCalculation_**_ [Double] - Period in seconds to run the calculation cycle.

## Extending the Database Schema

MongoDB schemas can be extended without affecting the JSON-SCADA standard processes. New properties and collections can be added to the standard schema. However, care should be taken to avoid naming collisions with future properties of the system.

All JSON-SCADA collections and fields are named with an initial lower case letter.
So, extended collections and properties should avoid this reserved convention.

Is is recommended that custom collections and extended fields/properties be named with an initial upper case letter and also should be used a prefix that can identify the company and/or application.

# PostgreSQL/TimescaleDB Schema

## Historian table

In this table historical data is written. Local and source timestamps are recorded so that SOE events can be also extracted from this table. This table is converted to a TimescaleDB hypertable to be treated optimally as a time series table. The recommended partition is by day, but this can be changed if desired.

    CREATE TABLE hist (
        tag text not null,
        time_tag TIMESTAMPTZ(3),
        value float not null,
        value_json jsonb,
        time_tag_at_source TIMESTAMPTZ(3),
        flags bit(8) not null,
        PRIMARY KEY ( tag, time_tag )
        );
    CREATE INDEX ind_timeTag on hist ( time_tag );
    CREATE INDEX ind_tagTimeTag on hist ( tag, time_tag_at_source );
    comment on table hist is 'Historical data table';
    comment on column hist.tag is 'String key for the point';
    comment on column hist.value is 'Value as a double precision float';
    comment on column hist.time_tag is 'GMT Timestamp for the time data was received by the server';
    comment on column hist.time_tag_at_source is 'Field GMT timestamp for the event (null if not available)';
    comment on column hist.value_json is 'Structured value as JSON, can be null when do not apply. For digital point it should be the status as in {s:"OFF"}';
    comment on column hist.flags is 'Bit mask 0x80=value invalid, 0x40=Time tag at source invalid, 0x20=Analog, 0x10=value recorded by integrity (not by variation)';

    -- timescaledb hypertable, partitioned by day
    SELECT create_hypertable('hist', 'time_tag', chunk_time_interval=>86400000000);

## Realtime Data Table

This table is updated with all fields and values changed from the realtimeData collection from the MongoDB database. It is a helper for apps that can use only the PostgreSQL historian. The internal structure of the realtimeData collection is reflected in the _json_data_ field that has _bson_ type, so when this internal schema change it will be automatically updated here without changing the PostgreSQL table schema. The application should select the the data fields it needs from this JSON object structure. This table only has the latest snapshots for tags, it does not grow with time, so it does not need to be converted to be managed by TimescaleDB.

    CREATE TABLE realtime_data (
        tag text not null,
        time_tag TIMESTAMPTZ(3) not null,
        json_data jsonb,
        PRIMARY KEY ( tag )
        );

    comment on table realtime_data is 'Realtime data and catalog data';
    comment on column realtime_data.tag is 'String key for the point';
    comment on column realtime_data.time_tag is 'GMT Timestamp for the data update';
    comment on column realtime_data.json_data is 'Data image as JSON from Mongodb realtimeData collection';
    CREATE INDEX ind_tag on realtime_data ( tag );

## Helper Views for Grafana

These views are helpful to create queries in Grafana, the column names are adapted to what is best expected by the Grafana query editor.

    CREATE VIEW grafana_hist AS
        SELECT
            time_tag AS "time",
            tag AS metric,
            value as value,
            time_tag_at_source,
            value_json,
            flags
        FROM hist;

    CREATE VIEW grafana_realtime AS
        SELECT
            tag as metric, 
            time_tag as time, 
            cast(json_data->>'value' as float) as value, 
            cast(json_data->>'_id' as text) as point_key, 
            json_data->>'valueString' as value_string, 
            cast(json_data->>'invalid' as boolean) as invalid, 
            json_data->>'description' as description, 
            json_data->>'group1' as group1  
        FROM realtime_data;
