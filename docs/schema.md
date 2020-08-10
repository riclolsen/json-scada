# Schema Documentation

The JSON-SCADA MongoDB database is comprised of the following collections.

* _realtimeData_ - This is the table that contains point information, tag names, point attributes and realtime values.
* _protocolDriverInstances_ - Configuration of protocol drivers instances. See specific protocol for documentation.
* _protocolConnections_ - Configuration of protocol connections. See specific protocol for documentation.
* _commandsQueue_ - Queue for commands. This collection has no configuration, it is written by the system when user issue commands in the UI. It is consumed by protocol drivers only when new documents are inserted. Old documents are not erased automatically. Protocol acknowledgement information is updated by the protocol driver.
* _soeData_ - Sequence of Events data. This is a Capped Collection, it has a limited size. Old documents are overwritten when the maximum sized is reached. Data here is only written for digital states when the protocol provides a source timestamp (like for IEC 60870-5-104 type 30).

Please notice that all numeric fields from the schema is recorded as BSON Doubles (64 bit floating point). However, some numeric fields are expected to contain only integer values. When numbers are updated by the Mongo Shell manually, all numeric data is converted to BSON Doubles by default. Some languages like Node.js can cause values to be stored as integers or doubles depending on the current value. It is important that values are always stored as BSON Doubles as otherwise problems may be encountered by protocol drivers, specially those programmed in C#/DotNet Core.

All string parameters are UTF-8 encoded.

Dates are stored as MongoDB BSON Dates (UTC).

## _realtimeData_ collection


Example document.

    {
    "_id": 3285,
    "alarmDisabled": false,
    "alarmState": -1,
    "alarmed": false,
    "annotation": "",
    "commandBlocked": false,
    "commandOfSupervised": 0,
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
    "timeTagAtSource": null,
    "timeTagAtSourceOk": null,
    "transient": false,
    "type": "analog",
    "ungroupedDescription": "Active power",
    "unit": "MW",
    "updatesCnt": 86580,
    "valueDefault": 3.1,
    "valueString": "2.9959 MW",
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
    "value": 2.9959075450897217
    }
    

### Configuration fields

* _**_id_**_ [Double] - Numeric key for the point. This is stored as a BSON Double but should only contain integer values. Must be unique for the collection. **Mandatory parameter**.
* _**_tag_**_ [String] - String key for the point. It must begin with a letter char (A-Z or a-z). Allowed symbols are underscore, dash and dot. Do not use spaces or symbols like #*!^~%$. There is no enforced limit for the size but we recommend to keep it below 30 characters to make displays more readable. Must be unique for the collection. **Mandatory parameter**.
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
* _**_historianDeadBand_**_ [Double] - Reserved for dead band parameter for historian. Currently not in use. **Mandatory parameter**.
* _**_historianPeriod_**_ [Double] - Reserved for period of integrity recording on historian. Currently not in use. Value -1 will be used to remove tag from historian. **Mandatory parameter**.
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
* _**_protocolSourceConnectionNumber_**_ [Double] - Indicates the protocol connection that can updated the point. Should contain only integer values. Only meaningful when _origin=supervised_ or _origin=command_. **Mandatory parameter**.
* _**_protocolSourceCommonAddress_**_ [Double] - Protocol driver common address (device address). Only meaningful when _origin=supervised_ or _origin=command_. See protocol documentation. **Mandatory parameter**.
* _**_protocolSourceObjectAddress_**_ [Double or String] -  Only meaningful when _origin=supervised_ or _origin=command_. See protocol documentation. **Mandatory parameter**.
* _**_protocolSourceASDU_**_ [Double or String] -  Only meaningful when _origin=supervised_ or _origin=command_. See protocol documentation. **Mandatory parameter**.
* _**_protocolSourceCommandDuration_**_ [Double] - Additional command specification. Only meaningful when _origin=command_. See protocol documentation. **Mandatory parameter**.
* _**_protocolSourceCommandUseSBO_**_ [Boolean] - Use or not Select Before Operate for commands. Only meaningful when _origin=command_. See protocol documentation. **Mandatory parameter**.
* _**_protocolDestinations_**_ [Array of Objects] - List of protocol destinations for server protocol connections. Can be null or empty array when not point is not to be distributed. See protocol documentation. **Mandatory parameter**.

### Values that can be configured and altered by the operators.

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

### Values updated by the system.

* _**_alarmed_**_ [Boolean] - When true means the point is alarmed. **Mandatory parameter**.
* _**_invalid_**_ [Boolean] - When true value is considered old or not trusted. **Mandatory parameter**.
* _**_overflow_**_ [Boolean] - Overflow detected for _type=analog_ value. **Mandatory parameter**.
* _**_transient_**_ [Boolean] - Flags a transient value. **Mandatory parameter**.
* _**_frozen_**_ [Boolean] - When true, value is considered frozen (not changing). **Mandatory parameter**.
* _**_value_**_ [Double] - Current value as a number. **Mandatory parameter**.
* _**_valueJSON_**_ [Object] - Current value as JSON (future use). **Optional parameter**.
* _**_valueString_**_ [String] - Current value as a string. **Mandatory parameter**.
* _**_timeTag_**_ [Date] - Last update time. **Mandatory parameter**.
* _**_timeTagAlarm_**_ [Date] - Last alarm time (when alarmed). **Mandatory parameter**.
* _**_timeTagAtSource_**_ [Date] - Timestamp from the source. **Mandatory parameter**.
* _**_timeTagAtSourceOk_**_ [Boolean] - When true, the source timestamp is * considered ok. **Mandatory parameter**.
_**_updatesCnt_**_ [Double] - Count of updates. **Mandatory parameter**.
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
* _**_sourceDataUpdate.valueStringAtSource_**_ [Double] - Current string value at source.
* _**_sourceDataUpdate.valueJSONAtSource_**_ [Double] - Current JSON value at source.
* _**_sourceDataUpdate.timeTagAtSource_**_ [Date] - Source timestamp.
* _**_sourceDataUpdate.timeTagAtSourceOk_**_ [Boolean] - Source timestamp ok.
* _**_sourceDataUpdate.timeTag_**_ [Date] - Local update time.

## _commandsQueue_ collection

