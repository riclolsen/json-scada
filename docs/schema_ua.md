# Документація схеми MongoDB

База даних JSON-SCADA MongoDB складається з наступних колекцій.

* _realtimeData_ - Це таблиця, яка містить інформацію про точки, назви тегів, атрибути точки та значення в режимі реального часу.
* _protocolDriverInstances_ - Налаштування екземплярів драйверів протоколів. Див. Конкретний протокол для документації.
* _protocolConnections_ - Налаштування протокольних з'єднань. Див. Конкретний протокол для документації.
* _commandsQueue_ - Черга для команд.
* _soeData_ - Послідовність даних про події. Це обмежена колекція, вона має обмежений розмір.
* _processInstances_ - Конфігурація та інформація про екземпляри процесів JSON-SCADA.

Зверніть увагу, що всі числові поля зі схеми записуються як подвійні BSON (64-бітна плаваюча крапка). Однак деякі числові поля повинні містити лише цілі значення. Коли номери оновлюються оболонкою Mongo вручну, усі числові дані за замовчуванням перетворюються на подвійні BSON. Деякі мови, такі як Node.js, можуть спричинити збереження значень як цілих чи подвійних, залежно від поточного значення. Важливо, щоб значення завжди зберігалися як подвійні BSON, оскільки в іншому випадку драйвери протоколів можуть зіткнутися з проблемами, особливо запрограмованими в C # / DotNet Core.

Усі параметри рядка кодуються UTF-8.

Дати зберігаються як MongoDB BSON Dates (UTC).

## _realtimeData_ колекція

Зразок документа.

    {
        "_id": 3285,
        "alarmDisabled": false,
        "alarmState": -1,
        "alarmed": false,
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
        "value": 2.9959075450897217,
        "zeroDeadband": 0
    }
    

### Поля конфігурації

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
* _**_zeroDeadband_**_ [Double] - When acquired value is below this deadband it will be zeroed. Only meaningful for _type=analog_. **Mandatory parameter**.
* _**_protocolSourceConnectionNumber_**_ [Double] - Indicates the protocol connection that can updated the point. Should contain only integer values. Only meaningful when _origin=supervised_ or _origin=command_. **Mandatory parameter**.
* _**_protocolSourceCommonAddress_**_ [Double] - Protocol common address (device address). Only meaningful when _origin=supervised_ or _origin=command_. See protocol documentation. **Mandatory parameter**.
* _**_protocolSourceObjectAddress_**_ [Double or String] -  Protocol object address. Only meaningful when _origin=supervised_ or _origin=command_. See protocol documentation. **Mandatory parameter**.
* _**_protocolSourceASDU_**_ [Double or String] - Protocol information ASDU TI type. Only meaningful when _origin=supervised_ or _origin=command_. See protocol documentation. **Mandatory parameter**.
* _**_protocolSourceCommandDuration_**_ [Double] - Additional command specification. Only meaningful when _origin=command_. See protocol documentation. **Mandatory parameter**.
* _**_protocolSourceCommandUseSBO_**_ [Boolean] - Use or not Select Before Operate for commands. Only meaningful when _origin=command_. See protocol documentation. **Mandatory parameter**.
* _**_protocolDestinations_**_ [Array of Objects] - List of protocol destinations for server protocol connections. Can be null or empty array when not point is not to be distributed. See protocol documentation. **Mandatory parameter**.

### Поля, які можуть бути налаштовані та змінені операторами.

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

### Поля, оновлені системою.

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
* _**_updatesCnt_**_ [Double] - Count of updates. **Mandatory parameter**.
* _**_sourceDataUpdate_**_ [Object] - Information updated by protocol driver or calculation process.

#### Об'єкт sourceDataUpdate

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

## _commandsQueue_ колекція

Ця колекція не має конфігурації, вона записується системою, коли користувач видає команди в інтерфейсі. Сюди вставляються команди для відправки драйверами протоколів. Інформація про підтвердження протоколу оновлюється тут також драйверами протоколів.

Спеціальні програми також можуть створювати команди, які за бажанням надсилаються драйверами протоколів.

Команди обробляються лише для нових вставок. Старі документи зберігаються і їх можна видалити лише вручну.

Приклад документа

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

Тут зберігається інформація про послідовність подій (SOE) для цифрових значень із позначками часу.

Це обмежена колекція, вона має обмежений розмір. Старі документи перезаписуються, коли досягається максимальний розмір. Дані тут записуються лише для цифрових станів, коли протокол забезпечує відмітку часу джерела (наприклад, для IEC 60870-5-104 типу 30).

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
* _**_pointKey_**_ [Double] - Numeric key of the point (link to _id field of _realtimeData_ collection).
* _**_group1_**_ [String] - Highest level grouping.
* _**_description_**_ [String] - Full description of monitored information.
* _**_eventText_**_ [String] - Text related to the event status change.
* _**_invalid_**_ [Boolean] - When true means the status change is not trusted to be ok.
* _**_priority_**_ [Double] - Priority of the point, 0 (highest) - 9 (lowest)
* _**_timeTag_**_ [Date] - Timestamp for the arrival of information.
* _**_timeTagAtSource_**_ [Date] - Timestamp for the change stamped by the source device (RTU/IED).
* _**_timeTagAtSourceOk_**_ [Boolean] - When true means the source timestamp is considered ok.
* _**_ack_**_ [Double] - Operator acknowledgement (0=not acknowledged, 1=acknowledged, 2=eliminated from lists).

## _processInstances_ collection

Цю колекцію потрібно налаштувати, коли якийсь процес вимагає більше одного екземпляра. Також його можна використовувати для обмеження вузлів, які можуть підключатися до бази даних, заповнюючи масив _nodeNames_. Для цієї колекції пара processName / processInstanceNumber не повинна повторюватися (для цих полів існує унікальний індекс, щоб запобігти подібним помилкам).

### _processInstances_ entry for the _CS_DATA_PROCESSOR_ module

Приклад документа для модуля _CS_DATA_PROCESSOR_. В даний час цей процес підтримує лише один надлишковий екземпляр. Немає необхідності налаштовувати цей документ для цього модуля, оскільки він може автоматично створити запис, коли його не знайдено.

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

Приклад документа для модуля _CALCULATIONS_. В даний час цей процес підтримує лише один надлишковий екземпляр. Немає необхідності налаштовувати цей документ для цього модуля, оскільки він може автоматично створити запис, коли його не знайдено.

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

## Розширення схеми бази даних

Схеми MongoDB можна розширити, не впливаючи на стандартні процеси JSON-SCADA. Нові властивості та колекції можуть бути додані до стандартної схеми. Однак слід подбати про те, щоб уникнути зіткнень імен із майбутніми властивостями системи.

Усі колекції та поля JSON-SCADA позначаються початковою малою літерою.
Отже, розширені колекції та властивості повинні уникати цієї зарезервованої конвенції.

Рекомендується називати власні колекції та розширені поля/властивості початковою великою літерою, а також використовувати префікс, який може ідентифікувати компанію та/або програму.

# Схема PostgreSQL/TimescaleDB

## Таблиця істоії

У цій таблиці записані історичні дані. Місцеві та вихідні мітки часу записуються, щоб події SOE також можна було витягти з цієї таблиці. Ця таблиця перетворюється на гіпертаблицю TimescaleDB для оптимального трактування як таблиці часових рядів. Рекомендований розділ - це день, але його можна змінити за бажанням.

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

## Таблиця даних реального часу

Ця таблиця оновлюється всіма полями та значеннями, зміненими з колекції realtimeData з бази даних MongoDB. Це помічник для програм, які можуть використовувати лише історичні дані PostgreSQL. Внутрішня структура колекції realtimeData відображається в полі _json_data_, яке має тип _bson_, тому при зміні цієї внутрішньої схеми воно буде автоматично оновлено тут, не змінюючи схеми таблиці PostgreSQL. Додаток повинен вибрати необхідні поля даних із цієї структури об’єкта JSON. Ця таблиця містить лише найновіші знімки тегів, вона не росте з часом, тому її не потрібно перетворювати, щоб керувати TimescaleDB.

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

## Помічні перегляди для Grafana

Ці подання корисні для створення запитів у Grafana, назви стовпців адаптовані до того, що найкраще очікується від редактора запитів Grafana.

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