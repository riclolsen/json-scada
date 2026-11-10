# MongoDB Schema Documentation

The JSON-SCADA MongoDB database is comprised of the following collections.

* _realtimeData_ - This is the table that contains point information, tag names, point attributes and realtime values.
* _protocolDriverInstances_ - Configuration of protocol drivers instances. See specific protocol for documentation.
* _protocolConnections_ - Configuration of protocol connections. See specific protocol for documentation.
* _commandsQueue_ - Queue for commands.
* _soeData_ - Sequence of Events data. This is a Capped Collection, it has a limited size.
* _processInstances_ - Configuration and information about JSON-SCADA instances of processes.
* _hist_ - MongoDB time series historian (secondary to the PostgreSQL historian). See [_hist_ collection](#hist-collection).
* _users_ - User accounts. See [_users_ collection](#users-collection).
* _roles_ - RBAC roles (rights and group filters). See [_roles_ collection](#roles-collection).
* _userActions_ - Audit log of user actions. See [_userActions_ collection](#useractions-collection).
* _cimModels_, _cimMeasurementBindings_, _cimRuns_, _cimContingencyLists_, _cimStudyRequests_ and the GridFS bucket _cimArtifacts_ - CIM-EMS network model revisions, binding revisions, study runs, contingency lists, Admin UI study requests and stored packages/results. Written by the _CIM-EMS_ driver; see the [CIM-EMS README](../src/cim-ems/README.md). Model packages and study results are confidential: _cimArtifacts_ is not served by `/GetFile`. Contingency result files (`kind: "contingency-result"`) are deleted by the driver after the _cimRuns_ retention (7 days); element catalogs (`kind: "element-catalog"`, binding review) are kept for the 5 newest per model; the state estimation residual report (`kind: "se-residuals"`) keeps only the latest one per connection.
* _systemSettings_ - Single document (`key: "global"`) with runtime toggles (`autoManageServices`, `autoRestartOnConnectionChange`), created on demand by _server_realtime_auth_; not created by the seed script.

Collections, validation and indexes are created by [mongo_seed/b_create-db.js](../mongo_seed/b_create-db.js), which is the authoritative definition when this document and the script differ:

| Collection | Type / retention | Validation | Indexes |
|---|---|---|---|
| realtimeData | normal | none | `tag` (unique, `tagIndex`); `protocolSourceConnectionNumber + protocolSourceCommonAddress + protocolSourceObjectAddress`; `group1 + group2`; `invalid + alarmed` |
| protocolDriverInstances | normal | `$jsonSchema`, `validationLevel: strict`, `validationAction: error` | `protocolDriver + protocolDriverInstanceNumber` (unique) |
| protocolConnections | normal | `$jsonSchema`, strict / error | `protocolConnectionNumber` (unique) |
| processInstances | normal | `$jsonSchema`, strict / error | `processName + processInstanceNumber` (unique) |
| commandsQueue | normal, no automatic expiry | none | none (watched by change streams) |
| soeData | capped, 2 GB circular buffer | none | `timeTag`; `timeTagAtSource`; `group1`; `ack` |
| hist | time series (`timeField: timeTag`, `metaField: tag`, 1 h buckets), `expireAfterSeconds` = 60 days | none | none created by the seed |
| users | normal | none (mongoose model in _server_realtime_auth_) | `username` (unique, `userNameIndex`) |
| roles | normal | none (mongoose model) | `name` (unique, `roleNameIndex`) |
| userActions | normal, no automatic expiry | none (mongoose model) | `timeTag` (`actionsTimeTagIndex`); for expiry replace it with a TTL index, e.g. `{ timeTag: 1 }, { expireAfterSeconds: 2592000 }` |
| cimModels | normal | none | `modelId + revision` (unique, `cimModelsIndex`); `modelId` unique for `status: "active"` (`cimModelsActiveIndex`) |
| cimMeasurementBindings | normal | none | `modelId + bindingRevision + cimMeasurementMrid` (`cimBindingsIndex`) |
| cimRuns | normal, TTL 7 days on `createdAt` (`cimRunsTtl`) | none | `connectionNumber + kind + runId` (`cimRunsIndex`) |
| cimContingencyLists | normal | none | `listId` (unique) |
| cimStudyRequests | normal, no automatic expiry | none | `status + createdAt` (`cimStudyRequestsIndex`) |

Writes that violate a validator on the strict/error collections are rejected by MongoDB, so new required fields must also be added to the validators in `b_create-db.js`.

Please notice that all numeric fields in the schema are recorded as BSON Doubles (64-bit floating point). However, some numeric fields are expected to contain only integer values. When numbers are updated manually by the Mongo Shell, all numeric data is converted to BSON Doubles by default. Some languages like Node.js can cause values to be stored as integers or doubles depending on the current value. It is important that values are always stored as BSON Doubles, otherwise problems may be encountered by protocol drivers, especially those programmed in C#/.NET Core.

All string parameters are UTF-8 encoded.

Dates are stored as MongoDB BSON Dates (UTC).

## _realtimeData_ collection

Example document.

    {
        "_id": 3285.0,
        "alarmDisabled": false,
        "alarmRange": 0.0,
        "alarmState": -1.0,
        "alarmed": false,
        "alerted": false,
        "alertState": "ok",
        "annotation": "",
        "commandBlocked": false,
        "commandOfSupervised": 0.0,
        "commissioningRemarks": "",
        "description": "KAW2~FD21 13,8kV~Active power",
        "eventTextFalse": "undefined",
        "eventTextTrue": "undefined",
        "formula": 0.0,
        "frozen": false,
        "frozenDetectTimeout": 300.0,
        "group1": "KAW2",
        "group2": "FD21 13,8kV",
        "group3": "",
        "hiLimit": null,
        "hihiLimit": null,
        "hihihiLimit": null,
        "historianDeadBand": 0.0,
        "historianPeriod": 0.0,
        "historianLastValue": null,
        "hysteresis": 0.0,
        "invalid": false,
        "invalidDetectTimeout": 300.0,
        "isEvent": false,
        "kconv1": 1.0,
        "kconv2": 0.0,
        "loLimit": null,
        "location": null,
        "loloLimit": null,
        "lololoLimit": null,
        "notes": "",
        "origin": "supervised",
        "overflow": false,
        "parcels": null,
        "priority": 0.0,
        "protocolSourceASDU": 13.0,
        "protocolSourceCommandDuration": null,
        "protocolSourceCommandUseSBO": null,
        "protocolSourceCommonAddress": 1.0,
        "protocolSourceConnectionNumber": 61.0,
        "protocolSourceObjectAddress": 3285.0,
        "protocolSourcePublishingInterval": 5.0,
        "protocolSourceSamplingInterval": 5.0,
        "protocolSourceQueueSize": 5.0,
        "protocolSourceDiscardOldest": true,
        "protocolSourceAccessLevel": 0.0,
        "sourceDataUpdate": {
            "valueAtSource": 2.9959075450897217,
            "valueStringAtSource": "2.9959075450897217",
            "valueJsonAtSource": "",
            "valueBsonAtSource": {},
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
        "supervisedOfCommand": 0.0,
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
        "updatesCnt": 86580.0,
        "valueDefault": 3.1,
        "valueString": "2.9959 MW",
        "valueJson": {},
        "protocolDestinations": [
            {
            "protocolDestinationConnectionNumber": 1001.0,
            "protocolDestinationCommonAddress": 1.0,
            "protocolDestinationObjectAddress": 3285.0,
            "protocolDestinationASDU": 13.0,
            "protocolDestinationCommandDuration": 0.0,
            "protocolDestinationCommandUseSBO": false,
            "protocolDestinationKConv1": 1.0,
            "protocolDestinationKConv2": 0.0,
            "protocolDestinationGroup": 0.0,
            "protocolDestinationHoursShift": -2.0
            }
        ],
        "value": 2.9959075450897217,
        "zeroDeadband": 0.0
    }
    

### Configuration fields

* _**_id_**_ [Double] - Numeric key for the point. This is stored as a BSON Double but should only contain integer values. Must be unique for the collection. **Mandatory parameter**.
* _**_tag_**_ [String] - String key for the point. Must be unique in the collection. See [Tag names](#tag-names) for enforced constraints, recommended conventions, driver-generated names and how displays interpret tag references. **Mandatory parameter**.
* _**_type_**_ [String] - Data type. Can be "digital", "analog", "string" or "json". **Mandatory parameter**.
* _**_origin_**_ [String] - How the value is obtained. Can be "supervised", "calculated", "manual", or "command". **Mandatory parameter**.
* _**_description_**_ [String] - Complete textual description of the tag information. **Mandatory parameter**.
* _**_ungroupedDescription_**_ [String] - Textual description leave out grouping.
* _**_group1_**_ [String] - Main group (highest level). E.g. station or installation name. **Mandatory parameter**.
* _**_group2_**_ [String] - Secondary grouping. E.g. bay or area name. **Mandatory parameter**.
* _**_group3_**_ [String] - Lowest level grouping. E.g. device ir equipment name. **Mandatory parameter**.
* _**_valueDefault_**_ [Double] - Numeric default value. **Mandatory parameter**.
* _**_priority_**_ [Double] - Alarm priority: 0=highest, 9=lowest. **Mandatory parameter**.
* _**_frozenDetectTimeout_**_ [Double] -  Time in seconds to detect frozen (not changing) analog value. Use zero to never detect. **Mandatory parameter**.
* _**_invalidDetectTimeout_**_ [Double] - Time in seconds to detect invalid/old value when not updating. All supervised values expire, do not use zero for supervised values. This timeout should be at least more than the station integrity interrogation period. **Mandatory parameter**.
* _**_historianDeadBand_**_ [Double] - Absolute dead band parameter for historian. Does not affect non analog tags. **Mandatory parameter**.
* _**_historianPeriod_**_ [Double] - Period of integrity recording on historian. Currently only values 0 and -1 are supported. Value 0.0 will not record by integrity. Value -1 will remove tag from historian. **Mandatory parameter**.
* _**_commandOfSupervised_**_ [Double] - Key (\_id) pointing to the command point related to a supervised point. Only meaningful for _origin=supervised_ points (put zero here for other origins). Put value zero for this parameter when the supervised point does not have a related command. **Mandatory parameter**.
* _**_supervisedOfCommand_**_ [Double] - Key (\_id) pointing to a supervised point related to a command point (tag where the command feedback manifests). Only meaningful for _origin=command_ points (put zero here for other origins). Put value zero for this parameter when the command point does not have a related supervised (not recommended as this is a blind command with no feedback for the user). **Mandatory parameter**.
* _**_location_**_ [GeoJSON] - Reserved for location coordinates. Currently not in use. Can be null. **Mandatory parameter**.
* _**_isEvent_**_ [Boolean] - Flag meaning that only transitions OFF->ON for _type=digital_ matters for alarms and SOE (commonly used for electrical protection events). For _type=analog_ values it means that every valid (not invalid) value change is recorded as an SOE event by _cs_data_processor_ (event text is the value plus unit and an ↑/↓ trend arrow; source timestamp is used when available); not recorded when _alarmDisabled=true_. **Mandatory parameter**.
* _**_unit_**_ [String] - Unit of measurement when _type=analog_. **Mandatory parameter**.
* _**_alarmState_**_ [Double] - Considered state for alarm (0=off=false, 1=on=true, 2=both states, 3=state OFF->ON transition, -1=no state produces alarms but alarms can be signaled by other means) when _type=digital_. **Mandatory parameter**.
* _**_stateTextTrue_**_ [String] - Text for state true (numeric value not zero) when _type=digital_. Normally expressed as present tense (e.g. "ON"). **Mandatory parameter**.
* _**_stateTextFalse_**_ [String] - Text for state false (numeric value zero) when _type=digital_. Normally expressed as present tense (e.g. "OFF").  **Mandatory parameter**.
* _**_eventTextTrue_**_ [String] - Text for state change false to true when _type=digital_. Normally expressed as past tense (e.g. "Switched ON"). **Mandatory parameter**.
* _**_eventTextFalse_**_ [String] - Text for state change true to false when _type=digital_. Normally expressed as past tense (e.g. "Switched OFF").  **Mandatory parameter**.
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
* _**_protocolSourceQueueSize_**_ [Double] - Queue size for data sampling/publishing. See protocol documentation. **Optional parameter**.
* _**_protocolSourceSamplingInterval_**_ [Double] - Sampling interval. See protocol documentation. **Optional parameter**.
* _**_protocolSourcePublishingInterval_**_ [Double] - Publishing interval. See protocol documentation. **Optional parameter**.
* _**_protocolSourceDiscardOldest_**_ [Boolean] - Discard oldest data when queue is full. See protocol documentation. **Optional parameter**.
* _**_protocolSourceAccessLevel_**_ [Double] - Access level. See protocol documentation. **Optional parameter**.
* _**_protocolDestinations_**_ [Array of Objects] - List of protocol destinations for server protocol connections. Can be null or an empty array when the point is not distributed. See protocol documentation. **Mandatory parameter**.
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

### Tag names

The rules below are deliberately separated: only the first group is enforced; the others are recommendations or describe how specific components treat names.

**Enforced constraints**

* Non-empty string, unique in _realtimeData_ (unique index `tagIndex`). Case sensitive.
* When created via the AdminUI/_server_realtime_auth_, leading/trailing spaces are trimmed and an empty name becomes `new_tag_<_id>`.
* No character set or length is validated by MongoDB, the AdminUI or the server.
* Names beginning with `_System.` are reserved for internal system tags created by the seed and processes.

**Recommended conventions for new user-defined tags**

* Begin with a letter (A-Z, a-z). A leading underscore is accepted but should be left to system tags.
* Use letters, digits, `_`, `-` and `.`; avoid spaces and symbols such as `#/|*!^~%$;:`.
* Keep names short (below about 30 characters) so displays stay readable.

These are conventions for tags you name yourself, chosen so a name works unchanged in every component (displays, scripts, SQL, URLs). Valid names outside them are not errors.

**Names generated by drivers (`autoCreateTags`)**

Drivers that create tags automatically build names from the connection and the protocol address, and these intentionally use characters outside the recommended set. They are valid tags and must not be "fixed" by renaming.

| Driver | Generated tag name |
|---|---|
| IEC 60870-5-101/104 clients, DNP3 client | `<connectionName>;<commonAddress>;<objectAddress>` |
| OPC UA client | `<connectionName>;<NodeId>` (NodeId like `ns=2;s=Demo.Value`) |
| OPC DA client | `<connectionName>.<itemId>` (item ids can contain spaces) |
| MQTT/Sparkplug B | `MQTT:<connectionNumber>:<topic path>` (contains `/`) |
| Telegraf listener | group parts and field joined by `~` |

Other drivers use names supplied in their configuration or payloads; see each driver README.

**Protocol source addresses are not tag names**

`protocolSourceObjectAddress`, `protocolSourceCommonAddress` and the `protocolDestinations` addresses hold values in the protocol's native syntax (numeric IOAs/indexes, OPC UA NodeIds, OPC DA item ids, MQTT topics, PLC addresses, ...). The tag naming conventions do not apply to them; their format is defined by each driver.

**How the SVG display renderer interprets tag references**

In SVG displays (Display Viewer and the Synoptic Editor markup) the text in a tag field is parsed before lookup:

* If it starts with a number (JavaScript `parseInt` succeeds, e.g. `21xyz` → 21) it is treated as a point key (`_id`), not as a tag name.
* A leading `!` selects a special code (`!ALM`, `!SLIM`, `!TAG`, `!EVAL ... !END`, ...), see [Special Codes](../src/svg-display-editor/README.md#special-codes).
* A leading `#` in the Set tab selects a renderer command (`#exec`, `#exec_once`, `#exec_on_update`, `#set_filter`, `#arc`, `#radar`, `#camera`, `#foreign_object`, `#copy_xsac_from`), see [Set Tab](../src/svg-display-editor/README.md#set-tab).
* `%n`, `%m`, ... anywhere in the text are faceplate indirections replaced by the group variables, see [Faceplate Tab](../src/svg-display-editor/README.md#faceplate-tab).

So a tag whose name starts with a digit, `!` or `#`, or contains `%`, cannot be referenced by name in a display; reference it by its numeric point key (`_id`) instead. Driver-generated names such as `IEC1;1;100` or `MQTT:1:group/node/metric` reference fine by name.

**Existing installations**

Do not rename existing tags to fit the conventions. Tag names are referenced by displays, scripts, dashboards and server-side mappings, and history (PostgreSQL `hist.tag`, MongoDB `hist` metaField) is stored by name, so a rename splits the history and breaks references. Apply the conventions to new tags; where a display conflict exists, use the point key.

### Fields that can be configured and altered by the operators.

* _**_hiLimit_**_ [Double] - High limit for out-of-range alarm. Use null, Infinity or a big value to avoid alarm. Only meaningful for _type=analog_. **Mandatory parameter**.
* _**_hihiLimit_**_ [Double] - High-high limit for out-of-range alarm. Use null, Infinity or a big value to avoid alarm. Only meaningful for _type=analog_. Currently not used. **Mandatory parameter**.
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
* _**_alertState_**_ [String] - Grafana alert state name. **Optional parameter**.
* _**_historianLastValue_**_ [Double] - Last value sent to historian (for dead band processing). Only for analog tags. **Mandatory parameter**.
* _**_invalid_**_ [Boolean] - When true value is considered old or not trusted. **Mandatory parameter**.
* _**_overflow_**_ [Boolean] - Overflow detected for _type=analog_ value. **Mandatory parameter**.
* _**_transient_**_ [Boolean] - Flags a transient value. **Mandatory parameter**.
* _**_frozen_**_ [Boolean] - When true, value is considered frozen (not changing). **Mandatory parameter**.
* _**_value_**_ [Double] - Current value as a number. **Mandatory parameter**.
* _**_valueJson_**_ [String] - Current value as JSON (structured value: object or array). SVG displays read its members with `!JSON <tag> <path>` markup (svg-scada skill, "Structured Values"). **Optional parameter**.
* _**_valueString_**_ [String] - Current value as a string. **Mandatory parameter**.
* _**_timeTag_**_ [Date] - Last update time. **Mandatory parameter**.
* _**_timeTagAlarm_**_ [Date] - Last alarm time (when alarmed). **Mandatory parameter**.
* _**_timeTagAlertState_**_ [Date] - Time of last Grafana alert state update. **Optional parameter**.
* _**_timeTagAtSource_**_ [Date] - Timestamp from the source. **Mandatory parameter**.
* _**_timeTagAtSourceOk_**_ [Boolean] - When true, the source timestamp is considered ok. **Mandatory parameter**.
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

### Fields only existing for the special tag "_System.Status.AlarmBeep"

* _**_beepType_**_ [Double] - Beep type. **Mandatory parameter**.
* _**_beepGroup1List_**_ [Array of String] - List of group1 alarmed. **Mandatory parameter**.

### Communication status tags

The _CS_DATA_PROCESSOR_ (both the Node.js and Go implementations; instance 1, active node) maintains tags with the health of every protocol driver instance and protocol connection, so displays, viewers, alarms and the historian can monitor communications like any other point. They have _origin="system"_, keys allocated below -1000, and a `commStatus` field identifying the source. They are created, renamed (when a connection is renamed) and removed (when the instance or connection is removed) automatically. The admin API (`/Invoke/auth/createTag`, `updateTag`, `deleteTag`) refuses to create tags under these name spaces, to delete them, or to change their `tag`, `type`, `origin`, `commStatus`, groups and descriptions; other fields (priority, limits, alarm settings) stay editable. Turn them off with `commStatusTags: false` in the [processInstances entry](#processinstances-entry-for-the-_cs_data_processor_-module).

Tag names (characters outside `A-Z a-z 0-9 _ -` in a name become `_`):

* `_System.Driver.<protocolDriver>.<protocolDriverInstanceNumber>.<Point>` - _group2_ "Drivers". Points: `Alive` (digital Running/Stopped: false after more than 4 consecutive 5 s readings of an unchanged _activeNodeKeepAliveTimeTag_, alarmed), `Enabled` (digital), `ActiveNode` and `Version` (string), `KeepAliveAge` (analog, s, not historized).
* `_System.Conn.<name>.<Point>` (the connection's `name`) - _group2_ "Connections", _group3_ the connection name. Points: `Status` (digital Online/Offline, alarmed, priority 1: on when the connection and its driver instance are enabled, the instance is alive, `stats.isConnected` is not false and not every tag of the connection is invalid), `Enabled` (digital), `LinkUp` (digital, `stats.isConnected`; invalid when the driver does not report it). Driver contract: every connection-oriented driver writes `protocolConnections.stats.isConnected` [Boolean] — clients: a protocol session to at least one endpoint is up; servers: at least one peer attached; listeners (Telegraf): data received in the last 60 s; n8n: last webhook POST succeeded, `StatsAge` (analog, s, not historized), `ActiveNode` (string), `TagCount`, `InvalidTags` (analog, supervised tags of the connection) and `Health` (analog, % of valid tags). With `commStatusMirrorStats: true`, also `Stats.<field>` for each numeric field of the connection's `stats`.
* `_System.Comm.DriversDown`, `_System.Comm.ConnectionsDown`, `_System.Comm.ConnectionsTotal` - analog summaries.

* _**_commStatus_**_ [Object] - `{ kind: "driver" | "conn" | "summary", ref: <protocolDriverInstances or protocolConnections _id as hex string>, point: <Point> }`. Only on communication status tags.

## _commandsQueue_ collection

This collection has no configuration, it is written by the system when user issue commands in the UI. Commands are inserted here to be dispatched by protocols drivers. Protocol acknowledgement information is updated here also by protocol drivers.

Custom applications can also create commands to be dispatched by protocol drivers if desired.

Commands are processed only for new insertions. Old documents are preserved and can only be manually removed.

Example document

    {
        "_id":{
            "$oid":"5f1098dfd0ea7b5d01d6045c"
        },
        "protocolSourceConnectionNumber":61.0,
        "protocolSourceCommonAddress":1.0,
        "protocolSourceObjectAddress":64139,
        "protocolSourceASDU":45.0,
        "protocolSourceCommandDuration":0.0,
        "protocolSourceCommandUseSBO":false,
        "pointKey":64139.0,
        "tag":"KAW2KPR21XCBR5217----K",
        "timeTag":{
            "$date":"2020-07-16T18:13:51.182Z"
        },
        "value":1.0,
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
        processInstanceNumber: 1.0,
        enabled: true,
        logLevel: 1.0,
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
* _**_processInstanceNumber_**_ [Double] - Process instance number.
* _**_enabled_**_ [Boolean] - When true, this instance is enabled.
* _**_logLevel_**_ [Double] - Log level (0=min, 3=max).
* _**_nodeNames_**_ [Array of String] - Names of allowed nodes. If null or empty any node is allowed.
* _**_activeNodeName_**_ [String] - Name of the current active node for this process instance.
* _**_activeNodeKeepAliveTimeTag_**_ [Date] - Keep-alive for the active node.
* _**_softwareVersion_**_ [String] - Software version of the process.
* _**_latencyAvg_**_ [Double] - Average latency in ms (only for CS_DATA_PROCESSOR).
* _**_latencyAvgMinute_**_ [Double] - Average latency on a minute in ms (only for CS_DATA_PROCESSOR).
* _**_latencyPeak_**_ [Double] - Peak latency (only for CS_DATA_PROCESSOR).
* _**_commStatusTags_**_ [Boolean] - Optional, default true. False stops maintaining the [communication status tags](#communication-status-tags) and invalidates them (instance 1 only, both CS_DATA_PROCESSOR implementations).
* _**_commStatusMirrorStats_**_ [Boolean] - Optional, default false. True also mirrors every numeric `stats` field of each protocol connection as a `_System.Conn.<name>.Stats.<field>` tag.

### _processInstances_ entry for the _CALCULATIONS_ module

Example document for the _CALCULATIONS_ module. Currently, this process supports just one redundant instance. There is no need to configure this document for this module as it can create the entry automatically when one is not found.

    {
        "_id":{
            "$oid":"1d3427575afe8a451246eb24"
        },
        processName: "CALCULATIONS",
        processInstanceNumber: 1.0,
        enabled: true,
        logLevel: 1.0,
        nodeNames: [], 
        activeNodeName: "mainNode",
        activeNodeKeepAliveTimeTag: { "$date": "2020-08-11T21:04:59.678Z" },
        softwareVersion: "0.1.1",
        periodOfCalculation: 2.0
    }

* _**__id_**_ [ObjectId] - MongoDB document id.
* _**_processName_**_ [String] - Process name ("CS_DATA_PROCESSOR" or "CALCULATIONS")
* _**_processInstanceNumber_**_ [Double] - Process instance number.
* _**_enabled_**_ [Boolean] - When true, this instance is enabled.
* _**_logLevel_**_ [Double] - Log level (0=min, 3=max).
* _**_nodeNames_**_ [Array of String] - Names of allowed nodes. If null or empty any node is allowed.
* _**_activeNodeName_**_ [String] - Name of the current active node for this process instance.
* _**_activeNodeKeepAliveTimeTag_**_ [Date] - Keep-alive for the active node.
* _**_softwareVersion_**_ [String] - Software version of the process.
* _**_periodOfCalculation_**_ [Double] - Period in seconds to run the calculation cycle.

## _hist_ collection

MongoDB time series collection written by _cs_data_processor_ (and the Go _cs_data_processor-go_) with the same records and filters (`historianPeriod`, `historianDeadBand`, `isNotForHistorical`) as the PostgreSQL historian. Inserts are unacknowledged (`w: 0`) bulk writes, so this historian is best-effort. Records older than `expireAfterSeconds` (60 days as seeded) are removed automatically; change it with `db.runCommand({ collMod: "hist", expireAfterSeconds: N })`.

    {
        tag: "KAW2AL-21MTWT",
        timeTag: ISODate("2026-10-03T12:00:00.123Z"),
        value: 123.4,
        invalid: false,
        timeTagAtSource: ISODate("2026-10-03T12:00:00.100Z"),
        timeTagAtSourceOk: true,
        cot: 3
    }

* _**_tag_**_ [String] - Tag name (time series _metaField_).
* _**_timeTag_**_ [Date] - Time the value was processed by the server (time series _timeField_).
* _**_value_**_ [Double | String | Object] - Value: Double for analog/digital, the string for _type=string_, the JSON value for _type=json_.
* _**_invalid_**_ [Boolean] - Value invalid flag.
* _**_timeTagAtSource_**_ [Date] - Field timestamp. Omitted when not available.
* _**_timeTagAtSourceOk_**_ [Boolean] - Field timestamp quality. Omitted when not available.
* _**_cot_**_ [Any] - Cause of transmission at source, as sent by the driver. Omitted when not available.

### MongoDB _hist_ versus PostgreSQL _hist_

| | MongoDB `hist` collection | PostgreSQL/TimescaleDB `hist` table |
|---|---|---|
| Role | Secondary historian, needs no extra database | Primary historian |
| Written by | _cs_data_processor_ directly (`insertMany`, `w: 0`) | _cs_data_processor_ writes SQL files, `sql/process_pg_hist.sh`/`.bat` loads them |
| Retention | `expireAfterSeconds` (60 days as seeded) | TimescaleDB `add_retention_policy('hist', INTERVAL '45 days')` in `sql/create_tables.sql` (`sql/delete_old.sql` for manual pruning) |
| Quality fields | `invalid`, `timeTagAtSource`, `timeTagAtSourceOk`, `cot` | `time_tag_at_source`, `flags` bit mask |
| Read by | _OPC-UA-Server_ HistoryRead (default backend, `historian: "mongodb"`) | _server_realtime_auth_ `/Invoke` HistoryRead and GraphQL, Grafana, Metabase, _OPC-UA-Server_ with `historian: "postgresql"` |

## _users_ collection

User accounts used by _server_realtime_auth_ (model `app/models/user.model.js`). Managed from the AdminUI; the seed creates `admin` (see `mongo_seed/users.json`).

* _**__id_**_ [ObjectId] - Document id.
* _**_username_**_ [String] - Login name. Unique (`userNameIndex`).
* _**_email_**_ [String] - E-mail address.
* _**_password_**_ [String] - bcrypt hash (cost 8), never plain text. Not used for LDAP users, who authenticate against the directory.
* _**_roles_**_ [Array of ObjectId] - References to _roles_ documents. Effective rights combine all roles (see below).
* _**_isLDAPUser_**_ [Boolean] - User is authenticated and synchronized via LDAP.
* _**_ldapDN_**_ [String] - LDAP distinguished name.
* _**_lastLDAPSync_**_ [Date] - Last LDAP synchronization, null when never.

Authentication: `POST /Invoke/auth/signin` with `{username, password}` returns the JWT in the httpOnly `x-access-token` cookie (also accepted as an `x-access-token` header on later requests) plus a readable `json-scada-user` cookie with the user's rights. The JWT carries the combined rights and expires after the largest `maxSessionDays` of the user's roles.

## _roles_ collection

RBAC roles (model `app/models/role.model.js`); seed in `mongo_seed/roles.json`. When a user has several roles, boolean rights are OR-ed, list fields are concatenated and `maxSessionDays` is the maximum. For `group1List`, an empty list in any role means no restriction.

* _**__id_**_ [ObjectId] - Document id.
* _**_name_**_ [String] - Role name. Unique (`roleNameIndex`).
* _**_isAdmin_**_ [Boolean] - Administration access (configuration, users, roles).
* _**_changePassword_**_ [Boolean] - May change own password.
* _**_sendCommands_**_ [Boolean] - May send commands.
* _**_enterAnnotations_**_, _**_enterNotes_**_, _**_enterManuals_**_, _**_enterLimits_**_ [Boolean] - May edit annotations, notes, manual values and alarm limits.
* _**_substituteValues_**_ [Boolean] - May substitute values.
* _**_ackEvents_**_, _**_ackAlarms_**_, _**_disableAlarms_**_ [Boolean] - May acknowledge events, acknowledge alarms, disable alarms.
* _**_group1List_**_ [Array of String] - _group1_ values the role can see. Empty = all.
* _**_group1CommandList_**_ [Array of String] - _group1_ values the role can command.
* _**_displayList_**_ [Array of String] - Displays the role can open.
* _**_cimEngineering_**_ [Boolean] - CIM-EMS engineering: upload and activate network models, approve binding revisions, download CIM-EMS artifacts. Checked in addition to _isAdmin_. Default false.
* _**_maxSessionDays_**_ [Double] - Session (JWT) lifetime in days. Default 3.

## _userActions_ collection

Audit log written by _server_realtime_auth_ through a queue (model `app/models/userAction.model.js`): sign-in, user/role/configuration changes, commands, acknowledgements, blocks, substitutions, limits, notes and similar operator actions. Passwords are removed from logged request bodies. No automatic expiry; see the index table above for a TTL option.

* _**__id_**_ [ObjectId] - Document id.
* _**_username_**_ [String] - User who performed the action (the login name supplied, for sign-in attempts).
* _**_action_**_ [String] - Action name (e.g. `signin`, `Remove All Events`).
* _**_pointKey_**_ [Double] - Affected point key, when the action targets a tag.
* _**_tag_**_ [String] - Affected tag name, when the action targets a tag.
* _**_properties_**_ [Object] - Action details (e.g. the request body without password, the command value).
* _**_timeTag_**_ [Date] - When the action happened. Indexed (`actionsTimeTagIndex`).

## Extending the Database Schema

MongoDB schemas can be extended without affecting the JSON-SCADA standard processes. New properties and collections can be added to the standard schema. However, care should be taken to avoid naming collisions with future properties of the system.

All JSON-SCADA collections and fields are named with an initial lower case letter.
So, extended collections and properties should avoid this reserved convention.

It is recommended that custom collections and extended fields/properties be named with an initial upper case letter and also should be used a prefix that can identify the company and/or application.

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
    -- data retention policy (older data will be deleted)
    SELECT add_retention_policy('hist', INTERVAL '45 days');

Rows are queued by _cs_data_processor_ as SQL files and loaded by `sql/process_pg_hist.sh` (or `.bat`). See [MongoDB _hist_ versus PostgreSQL _hist_](#mongodb-hist-versus-postgresql-hist) for the differences from the MongoDB time series collection. Authoritative DDL: [sql/create_tables.sql](../sql/create_tables.sql).

## Realtime Data Table

This table is updated with all fields and values changed from the realtimeData collection from the MongoDB database. It is a helper for apps that can use only the PostgreSQL historian. The internal structure of the realtimeData collection is reflected in the _json_data_ field that has _jsonb_ type (PostgreSQL binary JSON), so when this internal schema changes it is automatically updated here without changing the PostgreSQL table schema. The application should select the data fields it needs from this JSON object structure. This table only has the latest snapshots for tags, it does not grow with time, so it does not need to be converted to be managed by TimescaleDB.

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
