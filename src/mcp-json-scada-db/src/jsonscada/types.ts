/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
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

import { Double, ObjectId } from 'mongodb'

export const CollectionNames = {
  UserActions: 'userActions',
  Users: 'users',
  Roles: 'roles',
  GridFs: 'files',
  RealtimeData: 'realtimeData',
  SoeData: 'soeData',
  CommandsQueue: 'commandsQueue',
  ProcessInstances: 'processInstances',
  ProtocolDriverInstances: 'protocolDriverInstances',
  ProtocolConnections: 'protocolConnections',
  Hist: 'hist',
} as const

export type ProtocolDriverName =
  | 'IEC60870-5-104'
  | 'IEC60870-5-104_SERVER'
  | 'IEC60870-5-101'
  | 'IEC60870-5-101_SERVER'
  | 'IEC60870-5-103'
  | 'I104M'
  | 'DNP3'
  | 'DNP3_SERVER'
  | 'PLCTAG'
  | 'PLC4X'
  | 'OPC-UA'
  | 'OPC-UA_SERVER'
  | 'OPC-DA'
  | 'OPC-DA_SERVER'
  | 'TELEGRAF-LISTENER'
  | 'MODBUS'
  | 'MODBUS_SERVER'
  | 'MQTT-SPARKPLUG-B'
  | 'IEC61850-GOOSE'
  | 'IEC61850'
  | 'IEC61850_SERVER'
  | 'CIP-ETHERNET/IP'
  | 'S7'
  | 'SPA-BUS'
  | 'BACNET'
  | 'ICCP'
  | 'ICCP_SERVER'
  | 'PI_DATA_ARCHIVE_INJECTOR'
  | 'PI_DATA_ARCHIVE_CLIENT'
  | 'INFLUXDB_INJECTOR'
  | 'INFLUXDB_CLIENT'
  | 'ONVIF'
  | 'CIM-EMS'
  | 'UNDEFINED'

export type ProcessName =
  | 'CALCULATIONS'
  | 'CS_DATA_PROCESSOR'
  | 'CS_CUSTOM_PROCESSOR'
  | 'SERVER_REALTIME'
  | 'ALARM_BEEP'
  | 'DYNAMIC_CALCULATIONS'
  | 'ALARM_PROCESSOR'
  | 'MONGOFW'
  | 'MONGOWR'

export interface IProcessInstance {
  /** MongoDB document id. */
  _id?: ObjectId
  /** Process name (e.g. "CS_DATA_PROCESSOR" or "CALCULATIONS"). */
  processName: ProcessName
  /** Process instance number. */
  processInstanceNumber: Double | number
  /** When true, this instance is enabled. */
  enabled: boolean
  /** Log level (0=min, 3=max). */
  logLevel: Double | number
  /** Names of allowed nodes. If null or empty any node is allowed. */
  nodeNames: string[]
  /** Name of the current active node for this process instance. */
  activeNodeName?: string | null
  /** Keep-alive for the active node. */
  activeNodeKeepAliveTimeTag?: Date | null
  /** Software version of the process. */
  softwareVersion?: string | null
  /** Period in seconds to run the calculation cycle. */
  periodOfCalculation?: Double | number | null
  stats?: {
    /** Average latency in ms (only for CS_DATA_PROCESSOR). */
    latencyAvg?: Double | number
    /** Average latency on a minute in ms (only for CS_DATA_PROCESSOR). */
    latencyAvgMinute?: Double | number
    /** Peak latency (only for CS_DATA_PROCESSOR). */
    latencyPeak?: Double | number
    [key: string]: any
  } | null
  [key: string]: any
}

export interface IHist {
  /** MongoDB document id. */
  _id?: ObjectId
  /** GMT Timestamp for the time data was received by the server. */
  timeTag?: Date
  /** String key for the point. */
  tag?: string
  /** Field GMT timestamp for the event (null if not available). */
  timeTagAtSource?: Date | null
  /** Time tag at source invalid flag. */
  timeTagAtSourceOk?: boolean | null
  /** Value as a double precision float, string or boolean. */
  value?: Double | number | string | boolean | null
  /** Value invalid flag. */
  invalid?: boolean | null
  /** Cause of transmission. */
  cot?: string | null
  [key: string]: any
}

export interface ICommandsQueue {
  /** MongoDB document id. */
  _id?: ObjectId
  /** Indicates the protocol connection that will dispatch the command. Should contain only integer values. */
  protocolSourceConnectionNumber: Double | number
  /** Protocol common address (device address). See specific protocol documentation. */
  protocolSourceCommonAddress: Double | number | string
  /** Protocol object address. See specific protocol documentation. */
  protocolSourceObjectAddress: Double | number | string
  /** Protocol information ASDU type. See specific protocol documentation. */
  protocolSourceASDU: Double | number | string
  /** Additional command specification. See specific protocol documentation. */
  protocolSourceCommandDuration: Double | number
  /** When true means it is desired to use Select-Before-Operate sequence. */
  protocolSourceCommandUseSBO: boolean
  /** Numeric key of the point (link to _id field of realtimeData collection). */
  pointKey: Double | number
  /** Point tag name of event. */
  tag: string
  /** Timestamp for the insertion of the command document. */
  timeTag: Date
  /** Numeric value for the command. */
  value: Double | number
  /** String text for the command. */
  valueString: string
  /** Name of command originator process and user name. */
  originatorUserName: string
  /** IP address of originator. */
  originatorIpAddress: string
  /** When true means the protocol driver consumed and dispatched the command. */
  delivered: boolean
  [key: string]: any
}

export interface ISourceDataUpdate {
  /** Protocol ASDU/TI type. */
  asduAtSource?: string
  /** Cause of transmission. E.g. For IEC60870-5-104, "3"=Spontaneous, "20"=Station Interrogation. */
  causeOfTransmissionAtSource?: string
  /** When true means old value at source. */
  notTopicalAtSource?: boolean
  /** When true means invalid (not trusted) value at source. */
  invalidAtSource?: boolean
  /** When true means value is blocked at source. */
  blockedAtSource?: boolean
  /** When true means the value is replaced at source. */
  substitutedAtSource?: boolean
  /** Flags a counter carry at source. */
  carryAtSource?: boolean
  /** Flags a overflow of value at source. */
  overflowAtSource?: boolean
  /** Flags a transient value at source. */
  transientAtSource?: boolean
  /** Current numeric value at source. */
  valueAtSource?: Double | number
  /** Current string value at source. */
  valueStringAtSource?: string
  /** Current JSON value at source. */
  valueJsonAtSource?: string
  /** Current value at source as a Javascript object. */
  valueBsonAtSource?: any
  /** Source timestamp. */
  timeTagAtSource?: Date
  /** Source timestamp ok. */
  timeTagAtSourceOk?: boolean
  /** Local update time. */
  timeTag?: Date
  [key: string]: any
}

export interface IProtocolDestination {
  /** Indicates the protocol connection that will monitor updates to the point. Should contain only integer values. */
  protocolDestinationConnectionNumber: Double | number
  /** Protocol common address. See protocol documentation. */
  protocolDestinationCommonAddress: Double | number | string
  /** Protocol object address. See protocol documentation. */
  protocolDestinationObjectAddress: Double | number | string
  /** Protocol information ASDU TI type. See protocol documentation. */
  protocolDestinationASDU: Double | number | string
  /** Additional command specification. See protocol documentation. */
  protocolDestinationCommandDuration: Double | number
  /** Use or not Select Before Operate for commands. See protocol documentation. */
  protocolDestinationCommandUseSBO: boolean
  /** Conversion factor 1 (multiplier). */
  protocolDestinationKConv1: Double | number
  /** Conversion factor 2 (adder). */
  protocolDestinationKConv2: Double | number
  /** Group number or dataset id of points. See protocol documentation. */
  protocolDestinationGroup: Double | number | string
  /** Number of hours to add to timestamps. */
  protocolDestinationHoursShift: Double | number
  [key: string]: any
}

export enum Origin {
  Supervised = 'supervised',
  Command = 'command',
  Calculated = 'calculated',
  Manual = 'manual',
}

export enum DataType {
  Digital = 'digital',
  Analog = 'analog',
  String = 'string',
  Json = 'json',
}

export const enum AlarmState {
   Off = 0,
   On = 1,
   both = 2,
   TransitionOffOn = 3,
   NoState = -1,
}

export enum AlarmRange {
  Normal = 0,
  High = 1,
  HighHigh = 2,
  HighHighHigh = 3,
  Low = -1,
  LowLow = -2,
  LowLowLow = -3,
}

export enum Priority {
  P0_HIGHEST = 0,
  P1_HIGH = 1,
  P2_MEDIUM = 2,
  P3_LOW = 3,
  P4 = 4,
  P5 = 5,
  P6 = 6,
  P7 = 7,
  P8 = 8,
  P9_LOWEST = 9,
}

export interface IRealtimeData {
  /** Numeric key for the point (pointKey). This is stored as a BSON Double but should only contain integer values. Must be unique for the collection. */
  _id?: number | Double
  /** String key for the point. It must begin with a letter char (A-Z or a-z) or underscore. */
  tag?: string
  /** Data type. Can be "digital", "analog", "string" or "json". */
  type?: DataType
  /** How the value is obtained. Can be "supervised", "calculated", "manual", or "command". */
  origin?: Origin
  /** Complete textual description of the tag information. */
  description?: string
  /** Textual description leave out grouping. */
  ungroupedDescription?: string
  /** Main group (highest level). E.g. station or installation name. */
  group1?: string
  /** Secondary grouping. E.g. bay or area name. */
  group2?: string
  /** Lowest level grouping. E.g. device ir equipment name. */
  group3?: string
  /** Numeric default value. */
  valueDefault?: Double | number
  /** Alarm priority: 0=highest, 9=lowest. */
  priority?: Priority | Double | number
  /** Time in seconds to detect frozen (not changing) analog value. Use zero to never detect. */
  frozenDetectTimeout?: Double | number
  /** Time in seconds to detect invalid/old value when not updating. */
  invalidDetectTimeout?: Double | number
  /** Absolute dead band parameter for historian. Does not affect non analog tags. */
  historianDeadBand?: Double | number
  /** Period of integrity recording on historian. Currently only values 0 and -1 are supported. */
  historianPeriod?: Double | number
  /** Key (_id) pointing to the command point related to a supervised point. Only meaningful for origin=supervised points. */
  commandOfSupervised?: Double | number
  /** Key (_id) pointing to a supervised point related to a command point (tag where the command feedback manifests). Only meaningful for origin=command points. */
  supervisedOfCommand?: Double | number
  /** Reserved for location coordinates. Currently not in use. Can be null. */
  location?: any
  /** Flag meaning that only transitions OFF->ON for type=digital matters for alarms and SOE. */
  isEvent?: boolean
  /** Unit of measurement when type=analog. */
  unit?: string
  /** Considered state for alarm (0=off=false, 1=on=true, 2=both states, 3=state OFF->ON transition, -1=no state produces alarms). */
  alarmState?: AlarmState | Double | number
  /** Text for state true (numeric value not zero) when type=digital. Normally expressed as present tense (e.g. "ON"). */
  stateTextTrue?: string
  /** Text for state false (numeric value zero) when type=digital. Normally expressed as present tense (e.g. "OFF"). */
  stateTextFalse?: string
  /** Text for state change false to true when type=digital. Normally expressed as past tense (e.g. "Switched ON"). */
  eventTextTrue?: string
  /** Text for state change true to false when type=digital. Normally expressed as present tense (e.g. "Switched ON"). */
  eventTextFalse?: string
  /** A formula code for calculation of value. Only meaningful when origin=calculated. */
  formula?: Double | number
  /** Numeric point key references to parcel points for calculations. Only meaningful when origin=calculated. */
  parcels?: (Double | number)[]
  /** Conversion factor 1 (multiplier). Applied when origin=supervised, origin=command or origin=calculated. Use -1 to invert states of digital values and commands. */
  kconv1?: Double | number
  /** Conversion factor 2 (adder). Applied when origin=supervised or origin=calculated. */
  kconv2?: Double | number
  /** When acquired value is below this deadband it will be zeroed. Only meaningful for type=analog. */
  zeroDeadband?: Double | number
  /** Current value as a number. */
  value?: Double | number
  /** Current value as JSON. */
  valueJson?: string
  /** Current value as a string. */
  valueString?: string
  /** Last update time. */
  timeTag?: Date
  /** Last alarm time (when alarmed). */
  timeTagAlarm?: Date
  /** Time of last Grafana alert state update. */
  timeTagAlertState?: Date
  /** Timestamp from the source. */
  timeTagAtSource?: Date
  /** When true, the source timestamp is considered ok. */
  timeTagAtSourceOk?: boolean
  /** Indicates the protocol connection that can updated the point. Should contain only integer values. Only meaningful when origin=supervised or origin=command. */
  protocolSourceConnectionNumber?: Double | number
  /** Protocol common address (device address). Only meaningful when origin=supervised or origin=command. */
  protocolSourceCommonAddress?: Double | number | string
  /** Protocol object address. Only meaningful when origin=supervised or origin=command. */
  protocolSourceObjectAddress?: Double | number | string
  /** Protocol information ASDU TI type. Only meaningful when origin=supervised or origin=command. */
  protocolSourceASDU?: Double | number | string
  /** Additional command specification. Only meaningful when origin=command. */
  protocolSourceCommandDuration?: Double | number
  /** Use or not Select Before Operate for commands. Only meaningful when origin=command. */
  protocolSourceCommandUseSBO?: boolean
  /** Queue size for data sampling/publishing. */
  protocolSourceQueueSize?: Double | number
  /** Sampling interval. */
  protocolSourceSamplingInterval?: Double | number
  /** Publishing interval. */
  protocolSourcePublishingInterval?: Double | number
  /** Discard oldest data when queue is full. */
  protocolSourceDiscardOldest?: boolean
  /** List of protocol destinations for server protocol connections. Can be null or empty array when not point is not to be distributed. */
  protocolDestinations?: IProtocolDestination[]
  /** High limit for out-of-range alarm. Use null, Infinity or a big value to avoid alarm. Only meaningful for type=analog. */
  hiLimit?: Double | number | null
  /** High-high limit for out-of-range alarm. Use null,Infinity or a big value to avoid alarm. Only meaningful for type=analog. */
  hihiLimit?: Double | number | null
  /** High-high-high limit for out-of-range alarm. Use null, Infinity or a big value to avoid alarm. Only meaningful for type=analog. */
  hihihiLimit?: Double | number | null
  /** Low limit for out-of-range alarm. Use null, -Infinity or a big negative value to avoid alarm. Only meaningful for type=analog. */
  loLimit?: Double | number | null
  /** Low-low limit for out-of-range alarm. Use null, -Infinity or a big negative value to avoid alarm. Only meaningful for type=analog. */
  loloLimit?: Double | number | null
  /** Low-low-low limit for out-of-range alarm. Use -Infinity or a big negative value to avoid alarm. Only meaningful for type=analog. */
  lololoLimit?: Double | number | null
  /** Hysteresis (maximum absolute value variation that will not produce out-of-range alarms) for limits verification. Only meaningful for type=analog. */
  hysteresis?: Double | number
  /** When true, indicates that the value is substituted locally by the operator. */
  substituted?: boolean
  /** When true, indicates that alarms are disabled for the point. */
  alarmDisabled?: boolean
  /** Blocking annotation text (reason command for blocking). */
  annotation?: string
  /** When true, the command is disabled by the operator. */
  commandBlocked?: boolean
  /** Documental notes text about the point. */
  notes?: string
  /** Remarks about the point commissioning. */
  commissioningRemarks?: string
  /** When true value is considered old or not trusted. */
  invalid?: boolean
  /** Overflow detected for type=analog value. */
  overflow?: boolean
  /** Flags a transient value. */
  transient?: boolean
  /** When true, value is considered frozen (not changing). */
  frozen?: boolean
  /** When true means the point is alarmed. */
  alarmed?: boolean
  /** Current alarm range for analog tags. 0=normal, 1=hiLimit violated, -1=loLimit violated. */
  alarmRange?: AlarmRange | Double | number
  /** When true means the point is alerted (Grafana alert). */
  alerted?: boolean
  /** Grafana alert state name. */
  alertState?: string
  /** Last value sent to historian (for dead band processing). Only for analog tags. */
  historianLastValue?: Double | number
  /** Count of updates. */
  updatesCnt?: Double | number
  /** Information updated by protocol driver or calculation process. */
  sourceDataUpdate?: ISourceDataUpdate
  /** Beep type. */
  beepType?: Double | number | null
  /** List of group1 alarmed. */
  beepGroup1List?: string[] | null
  [key: string]: any
}

export interface IProtocolDriverInstance {
  /** MongoDB document id. */
  _id?: ObjectId
  protocolDriver: ProtocolDriverName
  protocolDriverInstanceNumber: Double | number
  enabled: boolean
  logLevel: Double | number
  nodeNames: string[]
  activeNodeName?: string | null
  activeNodeKeepAliveTimeTag?: Date | null
  keepProtocolRunningWhileInactive?: boolean | null
  softwareVersion?: string | null
  stats?: Record<string, any> | string | null
  [key: string]: any
}

export interface IUser {
  _id?: ObjectId
  username: string
  [key: string]: any
}

export interface IRole {
  _id?: ObjectId
  name: string
  [key: string]: any
}

export interface IUserAction {
  _id?: ObjectId
  timeTag: Date
  username?: string
  action?: string
  [key: string]: any
}

export interface ISoeData {
  /** MongoDB document id. */
  _id?: ObjectId
  /** Point tag name of event. */
  tag?: string
  /** Numeric key of the point (link to _id field of realtimeData collection). */
  pointKey?: number | Double
  /** Highest level grouping. */
  group1?: string
  /** Full description of monitored information. */
  description?: string
  /** Text related to the event status change. */
  eventText?: string
  /** When true means the status change is not trusted to be ok. */
  invalid?: boolean
  /** Priority of the point, 0 (highest) - 9 (lowest). */
  priority?: number | Double
  /** Timestamp for the arrival of information. */
  timeTag?: Date
  /** Timestamp for the change stamped by the source device (RTU/IED). */
  timeTagAtSource?: Date
  /** When true means the source timestamp is considered ok. */
  timeTagAtSourceOk?: boolean
  /** Operator acknowledgement (0=not acknowledged, 1=acknowledged, 2=eliminated from lists). */
  ack?: number | Double
  value?: Double | number
  valueString?: string
  quality?: number | Double
  [key: string]: any
}

export interface IProtocolConnection {
  /** MongoDB document id. */
  _id?: ObjectId
  protocolDriver: ProtocolDriverName
  protocolDriverInstanceNumber: Double | number
  protocolConnectionNumber: Double | number
  name: string
  description: string
  enabled: boolean
  commandsEnabled: boolean
  logLevel?: Double | number | null
  ipAddressLocalBind?: string | null
  ipAddresses?: string[]
  endpointURLs?: string[]
  localLinkAddress?: Double | number | null
  remoteLinkAddress?: Double | number | null
  giInterval?: Double | number | null
  testCommandInterval?: Double | number | null
  timeSyncInterval?: Double | number | null
  sizeOfCOT?: Double | number | null
  sizeOfCA?: Double | number | null
  sizeOfIOA?: Double | number | null
  k?: Double | number | null
  w?: Double | number | null
  t0?: Double | number | null
  t1?: Double | number | null
  t2?: Double | number | null
  t3?: Double | number | null
  serverModeMultiActive?: boolean | null
  maxClientConnections?: Double | number | null
  maxQueueSize?: Double | number | null
  options?: string | null
  portName?: string | null
  baudRate?: Double | number | null
  stopBits?: 'One' | 'One5' | 'Two' | null
  handshake?: 'None' | 'Xon' | 'Rts' | 'RtsXon' | null
  timeoutForACK?: Double | number | null
  timeoutRepeat?: Double | number | null
  useSingleCharACK?: boolean | null
  sizeOfLinkAddress?: Double | number | null
  asyncOpenDelay?: Double | number | null
  class0ScanInterval?: Double | number | null
  class1ScanInterval?: Double | number | null
  class2ScanInterval?: Double | number | null
  class3ScanInterval?: Double | number | null
  timeSyncMode?: Double | number | null
  enableUnsolicited?: boolean | null
  rangeScans?: {
    group?: Double | number | null
    variation?: Double | number | null
    startAddress?: Double | number | null
    stopAddress?: Double | number | null
    period?: Double | number | null
    [key: string]: any
  }[]
  allowTLSv10?: boolean | null
  allowTLSv11?: boolean | null
  allowTLSv12?: boolean | null
  allowTLSv13?: boolean | null
  chainValidation?: boolean | null
  allowOnlySpecificCertificates?: boolean | null
  cipherList?: string | null
  localCertFilePath?: string | null
  peerCertFilePath?: string | null
  rootCertFilePath?: string | null
  privateKeyFilePath?: string | null
  deadBand?: Double | number | null
  hoursShift?: Double | number | null
  autoCreateTags?: boolean | null
  autoCreateTagPublishingInterval?: Double | number | null
  autoCreateTagSamplingInterval?: Double | number | null
  autoCreateTagQueueSize?: Double | number | null
  configFileName?: string | null
  topics?: string[]
  topicsAsFiles?: string[]
  topicsScripted?: {
    topic: string
    script: string
    [key: string]: any
  }[]
  clientId?: string | null
  groupId?: string | null
  edgeNodeId?: string | null
  deviceId?: string | null
  scadaHostId?: string | null
  publishTopicRoot?: string | null
  username?: string | null
  password?: string | null
  pfxFilePath?: string | null
  passphrase?: string | null
  stats?: Record<string, any> | null
  [key: string]: any
}
