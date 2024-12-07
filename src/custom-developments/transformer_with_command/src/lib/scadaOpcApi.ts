// API to interact with the SCADA server
// Inspired by OPC WebHMI protocol 
// https://github.com/OPCFoundation/UA-.NETStandard/tree/demo/webapi/SampleApplications/Workshop/Reference
// https://web.archive.org/web/20211018094004/https://prototyping.opcfoundation.org/
// OPC-UA Standard Codes (and some extensions adapted to use to interact with the SCADA).
// {json:scada} - Copyright 2024 - Ricardo L. Olsen

export const OpcNamespaces = {
  Mongodb: 2,
  Postgresql: 3,
}

export const OpcAcknowledge = {
  AckOneEvent: 0x00000001,
  RemoveOneEvent: 0x00000002,
  AckPointEvents: 0x00000004,
  RemovePointEvents: 0x00000008,
  AckAllEvents: 0x00000040,
  RemoveAllEvents: 0x00000080,
  AckOneAlarm: 0x00000100,
  AckAllAlarms: 0x00000400,
  SilenceBeep: 0x00001000,
} as const

export const OpcStatusCodes = {
  Good: 0,
  GoodNoData: 0x00a50000,
  GoodMoreData: 0x00a60000,
  Uncertain: 0x40000000,
  Bad: 0x80000000,
  BadTimeout: 0x800a0000,
  BadNodeAttributesInvalid: 0x80620000,
  BadNodeIdInvalid: 0x80330000,
  BadNodeIdUnknown: 0x80340000,
  BadRequestHeaderInvalid: 0x802a0000,
  BadRequestNotAllowed: 0x80e40000,
  BadServiceUnsupported: 0x800b0000,
  BadShutdown: 0x800c0000,
  BadServerNotConnected: 0x800d0000,
  BadServerHalted: 0x800e0000,
  BadNothingToDo: 0x800f0000,
  BadUserAccessDenied: 0x801f0000,
  BadIdentityTokenInvalid: 0x80200000,
  BadIdentityTokenRejected: 0x80210000,
  BadUnexpectedError: 0x80010000,
  BadInternalError: 0x80020000,
  BadOutOfMemory: 0x80030000,
  BadResourceUnavailable: 0x80040000,
  BadCommunicationError: 0x80050000,
  BadInvalidArgument: 0x80ab0000,
  BadDisconnect: 0x80ad0000,
  BadConnectionClosed: 0x80ae0000,
  BadInvalidState: 0x80af0000,
  BadNoDataAvailable: 0x80b10000,
  BadWaitingForResponse: 0x80b20000,
} as const

export const TimestampsToReturn = {
  Source: 0,
  Server: 1,
  Both: 2,
  Neither: 3,
  Invalid: 4,
} as const

export const OpcValueTypes = {
  Null: 0,
  Boolean: 1,
  SByte: 2,
  Byte: 3,
  Int16: 4,
  UInt16: 5,
  Int32: 6,
  UInt32: 7,
  Int64: 8,
  UInt64: 9,
  Float: 10,
  Double: 11,
  String: 12,
  DateTime: 13,
  Guid: 14,
  ByteString: 15,
  XmlElement: 16,
  NodeId: 17,
  ExpandedNodeId: 18,
  StatusCode: 19,
  QualifiedName: 20,
  LocalizedText: 21,
  ExtensionObject: 22,
  DataValue: 23,
  Variant: 24,
  DiagnosticInfo: 25,
  Number: 26,
  Integer: 27,
  UInteger: 28,
  Enumeration: 29,
} as const

export const OpcKeyType = {
  Numeric: 0,
  String: 1,
} as const

export const OpcServiceCode = {
  ServiceFault: 395,
  RequestHeader: 389,
  ResponseHeader: 392,
  ReadValueId: 626,
  ReadRequest: 629,
  ReadResponse: 632,
  ReadRawModifiedDetails: 647,
  HistoryReadRequest: 662,
  HistoryReadResponse: 665,
  WriteValue: 668,
  WriteRequest: 671,
  WriteResponse: 674,
  DataChangeNotification: 809,
  StatusChangeNotification: 818,
  Extended_RequestUniqueAttributeValues: 100000001,
  Extended_ResponseUniqueAttributeValues: 100000002,
} as const

export const OpcOperand = {
  Attribute: 598,
  Literal: 595,
  Element: 592,
  SimpleAttributeOperand: 601,
} as const

export const OpcAttributeId = {
  NodeID: 1,
  NodeClass: 2,
  BrowseName: 3,
  DisplayName: 4,
  Description: 5,
  WriteMask: 6,
  UserWriteMask: 7,
  IsAbstract: 8,
  Symmetric: 9,
  InverseName: 10,
  ContainsNoLoops: 11,
  EventNotifier: 12,
  Value: 13,
  DataType: 14,
  ValueRank: 15,
  ArrayDimensions: 16,
  AccessLevel: 17,
  UserAccessLevel: 18,
  MinimumSamplingInterval: 19,
  Historizing: 20,
  Executable: 21,
  UserExecutable: 22,
  ExtendedGroup1: 100000001,
  ExtendedGroup2: 100000002,
  ExtendedGroup3: 100000003,
  ExtendedAlarmEventsAck: 100000004,
  ExtendedBlockingAnnotation: 100000005,
  ExtendedDocumentalAnnotation: 100000006,
} as const

export const OpcFilterOperator = {
  Equals: 0,
  IsNull: 1,
  GreaterThan: 2,
  LessThan: 3,
  GreaterThanOrEqual: 4,
  LessThanOrEqual: 5,
  Like: 6,
  Not: 7,
  Between: 8,
  InList: 9,
  And: 10,
  Or: 11,
  Cast: 12,
  InView: 13,
  OfType: 14,
  RelatedTo: 15,
  BitwiseAnd: 16,
  BitwiseOr: 17,
} as const

export const OpcDiagnosticInfoMask = {
  SymbolicId: 1,
  LocalizedText: 2,
  AdditionalInfo: 4,
  InnerStatusCode: 8,
  InnerDiagnosticInfo: 16,
} as const

/**
 * Represents a node (tag) identifier, which includes the type of identifier,
 * the actual identifier value, and the namespace index. The `IdType` indicates
 * how the `Id` should be interpreted, while the `Namespace` specifies the
 * namespace to which the node belongs. The `Id` can be a string or undefined.
 *  @property {typeof OpcKeyType[keyof typeof OpcKeyType]} IdType - The type of identifier, which can be 0 or 1.
 *  @property {string | number | undefined} Id - The actual identifier value, which can be a string, number, or undefined.
 *  @property {typeof OpcNamespaces[keyof typeof OpcNamespaces]} Namespace - The namespace index, which can be 2 or 3.
 */
export interface OpcNodeId {
  IdType: (typeof OpcKeyType)[keyof typeof OpcKeyType] // Only allows 0 or 1
  Id: string | number | undefined
  Namespace: (typeof OpcNamespaces)[keyof typeof OpcNamespaces] // Only allows 2 or 3
}

/**
 * Represents a node (tag) to be read. It includes the node's identifier
 * and the specific attribute of the node that needs to be read. The `NodeId` contains
 * details about the node's type, identifier, and namespace, while `AttributeId` specifies
 * the attribute of the node to be accessed.
 *  @property {OpcNodeId} NodeId - The node identifier.
 *  @property {typeof OpcAttributeId[keyof typeof OpcAttributeId]} AttributeId - The specific attribute of the node to be read.
 */
export interface OpcNodeToRead {
  NodeId: OpcNodeId
  AttributeId: (typeof OpcAttributeId)[keyof typeof OpcAttributeId]
  ClientHandle?: string
}

/**
 * Represents additional properties of an OPC value, such as alarm settings, annotation, and notes.
 *
 * @property {boolean} [alarmDisabled] - Whether the alarm is disabled.
 * @property {string} [annotation] - The (blocking) annotation of the value.
 * @property {number} [loLimit] - The low limit of the value for out of range alarms.
 * @property {number} [hiLimit] - The high limit of the value for out of range alarms.
 * @property {number} [hysteresis] - The hysteresis of the value for out of range alarms.
 * @property {string} [notes] - The documental notes of the value.
 * @property {number} [newValue] - The new value of the value.
 * @property {boolean} [substituted] - Whether the value is substituted (manually imposed).
 */
export interface ExtendedProperties {
  alarmDisabled?: boolean
  annotation?: string
  loLimit?: number
  hiLimit?: number
  hysteresis?: number
  notes?: string
  newValue?: number
  substituted?: boolean
}

/**
 * Represents an OPC value for a tag (node), which includes the type of the value and the
 * actual value content. The `Type` property specifies the type of the value,
 * which can be one of the values in `OpcValueTypes`. The `Body` property
 * contains the actual value content, which can be a number, string, or boolean.
 * The `_Properties` property is an optional object that contains additional
 * information about the value, such as alarm settings, annotation, and notes.
 * The properties in `_Properties` are specific to the type of value and are not
 * required to be present.
 * @property {typeof OpcValueTypes[keyof typeof OpcValueTypes]} Type - The type of the value, which can be one of the values in `OpcValueTypes`.
 * @property {number | string | boolean} Body - The actual value content, which can be a number, string, or boolean.
 * @property {object} _Properties - An optional object that contains additional information about the value, such as alarm settings, annotation, and notes.
 */
export interface OpcValue {
  Type?: (typeof OpcValueTypes)[keyof typeof OpcValueTypes]
  Body?: number | string | boolean
  _Properties?: ExtendedProperties
}

/**
 * Represents a node (tag) to be written. It includes the node's identifier
 * and the specific attribute of the node that needs to be written. The
 * `NodeId` contains details about the node's type, identifier, and namespace,
 * while `AttributeId` specifies the attribute of the node to be accessed.
 * The `Value` property contains the value to be written to the node.
 *
 * @property {OpcNodeId} NodeId - The node identifier.
 * @property {typeof OpcAttributeId[keyof typeof OpcAttributeId]} AttributeId - The specific attribute of the node to be written.
 * @property {OpcValue} Value - The value to be written to the node.
 */
export interface OpcNodeToWrite {
  NodeId: OpcNodeId
  AttributeId: (typeof OpcAttributeId)[keyof typeof OpcAttributeId]
  Value: OpcValue
}

/**
 * The OPC UA request header.
 *
 * The header contains the timestamp of the request, a unique request handle,
 * a timeout hint, a flag to determine what type of diagnostics to return,
 * and an authentication token.
 *
 * @property {string} Timestamp - The timestamp of the request in ISO string format.
 * @property {number} RequestHandle - A unique handle to identify the request.
 * @property {number} TimeoutHint - The timeout hint for the request in milliseconds.
 * @property {typeof OpcDiagnosticInfoMask[keyof typeof OpcDiagnosticInfoMask]} ReturnDiagnostics - A flag to determine what type of diagnostics to return.
 * @property {null} AuthenticationToken - The authentication token for the request. Currently, it is always null.
 */
export interface OpcRequestHeader {
  Timestamp: string
  RequestHandle: number
  TimeoutHint: number
  ReturnDiagnostics: (typeof OpcDiagnosticInfoMask)[keyof typeof OpcDiagnosticInfoMask]
  AuthenticationToken: null
}

/**
 * Represents an operand in a content filter. The operand can be an attribute,
 * a literal value, or an element. The `FilterOperand` indicates the type of
 * operand, while the `Value` contains the actual value. The value can be a
 * string, number, or boolean.
 *
 * @property {typeof OpcOperand[keyof typeof OpcOperand]} FilterOperand - The type of operand.
 * @property {string | number | boolean} Value - The actual value of the operand.
 */
export interface OpcFilterOperand {
  FilterOperand: (typeof OpcOperand)[keyof typeof OpcOperand]
  Value: string | number | boolean
}

/**
 * Represents a content filter in the OPC UA request body.
 *
 * The content filter is a feature to filter the data returned by the
 * server. It consists of a filter operator and an array of filter operands.
 * The filter operator defines the type of operation to be performed on the
 * filter operands, while the filter operands specify the actual values to be
 * compared.
 *
 * @property {typeof OpcFilterOperator[keyof typeof OpcFilterOperator]} FilterOperator - The type of filter operation to be performed.
 * @property {OpcFilterOperand[]} FilterOperands - An array of filter operands to be compared.
 */
export interface OpcContentFilter {
  FilterOperator: (typeof OpcFilterOperator)[keyof typeof OpcFilterOperator]
  FilterOperands: OpcFilterOperand[]
}

/**
 * The parameters for the OPC UA HistoryRead request.
 *
 * The `IsModified` flag must be set to false.
 * The `StartTime` and `EndTime` properties are ISO strings representing the start and end times
 * of the history data to be read.
 * @property {boolean} IsModified - A flag that must be set to false.
 * @property {string} StartTime - The start time of the history data to be read in ISO string format.
 * @property {string} EndTime - The end time of the history data to be read in ISO string format.
 * @property {number} NumValuesPerNode - The number of values to be read per node.
 */
export interface OpcParameterData {
  IsReadModified: boolean
  StartTime: string
  EndTime: string
  NumValuesPerNode?: number
}

/**
 * The details of the OPC UA HistoryRead request.
 *
 * The `ParameterTypeId` specifies the type of history read request.
 * The `ParameterData` contains the parameters for the history read request.
 */
export interface OpcHistoryReadDetails {
  ParameterTypeId: (typeof OpcServiceCode)[keyof typeof OpcServiceCode]
  ParameterData: OpcParameterData
}

export interface OpcAggregateFilter {
  AggregateType: string
}

/**
 * The body of an OPC request.
 *
 * The body contains the request header, the maximum age of the data to be returned,
 * the type of timestamps to return, and the nodes to read.
 *
 * @property {OpcRequestHeader} RequestHeader - The request header.
 * @property {number} MaxAge - The maximum age of the data to be returned in milliseconds.
 * @property {typeof OpcAttributeId[keyof typeof OpcAttributeId]} AttributeId - The type of attribute to read.
 * @property {typeof OpcNamespaces[keyof typeof OpcNamespaces]} Namespace - The namespace to read from.
 * @property {typeof TimestampsToReturn[keyof typeof TimestampsToReturn]} TimestampsToReturn - The type of timestamps to return.
 * @property {OpcNodeToRead[]} NodesToRead - The nodes to read.
 * @property {any} NodesToWrite - The nodes to write.
 */
export interface OpcRequestBody {
  RequestHeader: OpcRequestHeader
  MaxAge?: number
  AttributeId?: (typeof OpcAttributeId)[keyof typeof OpcAttributeId]
  Namespace?: (typeof OpcNamespaces)[keyof typeof OpcNamespaces]
  TimestampsToReturn?: (typeof TimestampsToReturn)[keyof typeof TimestampsToReturn]
  ContentFilter?: any
  HistoryReadDetails?: OpcHistoryReadDetails
  NodesToRead?: OpcNodeToRead[]
  NodesToWrite?: OpcNodeToWrite[]
  AggregateFilter?: OpcAggregateFilter
}

/**
 * An OPC-like HTTP request.
 *
 * The request contains the service ID and the request body. The service ID
 * specifies which service to call, and the request body contains the parameters
 * for the service.
 *
 * @property {(typeof OpcServiceCode)[keyof typeof OpcServiceCode]} ServiceId - The service ID for the request.
 * @property {OpcRequestBody} Body - The request body.
 */
export interface OpcRequest {
  ServiceId: (typeof OpcServiceCode)[keyof typeof OpcServiceCode]
  Body: OpcRequestBody
}

/**
 * Represents a single data point read result from a tag, containing its name, value,
 * value as a string, quality, timestamp, group1, group2, and description.
 *
 * @property {string} name - The name of the tag.
 * @property {number} value - The value of the tag.
 * @property {string} valueString - The value of the tag as a string.
 * @property {(typeof OpcStatusCodes)[keyof typeof OpcStatusCodes]} quality - The quality of the tag value.
 * @property {string} timestamp - The timestamp of the tag value.
 * @property {string} group1 - The group1 of the tag.
 * @property {string} group2 - The group2 of the tag.
 * @property {string} description - The description of the tag.
 */
export interface DataPoint {
  name: string
  value: number
  valueString: string
  quality: (typeof OpcStatusCodes)[keyof typeof OpcStatusCodes]
  timestamp: Date
  group1: string
  group2: string
  description: string
}

/**
 * Represents a single historical sample for a tag, containing its value, value as a string, quality, server timestamp, and source timestamp.
 *
 * @property {number} value - The value of the tag.
 * @property {string} valueString - The value of the tag as a string.
 * @property {(typeof OpcStatusCodes)[keyof typeof OpcStatusCodes]} quality - The quality of the tag value.
 * @property {Date} serverTimestamp - The timestamp when the value was received by the server.
 * @property {Date} sourceTimestamp - The timestamp marked by the source device of the data.
 */
export interface HistoricalData {
  value: number
  valueString: string
  quality: (typeof OpcStatusCodes)[keyof typeof OpcStatusCodes]
  serverTimestamp: Date
  sourceTimestamp: Date
}

/**
 * Represents a Sequence of Events (SOE) data object.
 * The SOE data includes details about an event such as its name, description,
 * associated station (group1), event identifier, priority level, and the value
 * of the event as a string. Additionally, it contains quality information,
 * server and source timestamps, and a flag indicating if the source timestamp is valid.
 *
 * @property {string} name - The name of the event.
 * @property {string} description - A brief description of the event.
 * @property {string} group1 - The station name associated with the event.
 * @property {string} eventId - The unique identifier for the event.
 * @property {number} priority - The priority level of the event.
 * @property {string} valueString - The value of the event as a string.
 * @property {(typeof OpcStatusCodes)[keyof typeof OpcStatusCodes]} quality - The quality of the event data.
 * @property {Date} serverTimestamp - The timestamp when the event was received by the server.
 * @property {Date} sourceTimestamp - The timestamp marked by the source device of the event.
 * @property {boolean} sourceTimestampOk - Indicates if the source timestamp is valid.
 */
export interface SoeData {
  name: string
  description: string
  group1: string
  eventId: string
  priority: number
  valueString: string
  quality: (typeof OpcStatusCodes)[keyof typeof OpcStatusCodes]
  serverTimestamp: Date
  sourceTimestamp: Date
  sourceTimestampOk: boolean
}

export interface CommandResult {
  ok: boolean
  error: string
  commandHandle: string
}

/**
 * Get the group1 (station names) list
 * @returns {Promise<string[]>} a promise that resolves to a list of group1 names
 */
export async function getGroup1List(): Promise<string[]> {
  const ServiceId = OpcServiceCode.Extended_RequestUniqueAttributeValues // read data service
  const RequestHandle = Math.floor(Math.random() * 100000000)
  const req: OpcRequest = {
    ServiceId: ServiceId,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: RequestHandle,
        TimeoutHint: 1500,
        ReturnDiagnostics: OpcDiagnosticInfoMask.LocalizedText,
        AuthenticationToken: null,
      },
      AttributeId: OpcAttributeId.ExtendedGroup1,
    },
  }

  try {
    const response = await fetch('/Invoke/', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    })
    const data = await response.json()

    if (
      !data.ServiceId ||
      !data.Body ||
      !data.Body.ResponseHeader ||
      !data.Body.ResponseHeader.RequestHandle ||
      !data.Body.Results
    ) {
      return []
    }

    // response must have same request handle and be a read response or service fault
    if (
      data.Body.ResponseHeader.RequestHandle !== RequestHandle ||
      (data.ServiceId !==
        OpcServiceCode.Extended_ResponseUniqueAttributeValues &&
        data.ServiceId !== OpcServiceCode.ServiceFault)
    ) {
      return []
    }
    if (data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good) {
      return []
    }

    let group1List: string[] = []

    if ('Results' in data.Body)
      data.Body.Results.map((elem: any) => {
        if (elem.Value.Body !== null && elem.Value.Body !== '')
          group1List.push(elem.Value.Body)
        return elem
      })
    return group1List
  } catch (error) {
    return []
  }
}

/**
 * Get the filtered tags data from the server.
 * @param {string} group1Filter - filter by group1 (station names)
 * @param {string} group2Filter - filter by group2 (bay names)
 * @param {boolean} onlyAlarms - filter by only alarmed tags
 * @returns {Promise<DataPoint[]>} a promise that resolves to an array of DataPoint objects
 */
export async function getRealtimeFilteredData(
  group1Filter: string,
  group2Filter: string,
  onlyAlarms: boolean
): Promise<DataPoint[]> {
  try {
    let ContentFilter: OpcContentFilter[] = []
    let ContFiltElem: OpcContentFilter = {
      FilterOperator: OpcFilterOperator.Equals,
      FilterOperands: [] as OpcFilterOperand[],
    }

    if (typeof group1Filter === 'string' && group1Filter.trim() !== '') {
      ContFiltElem.FilterOperands.push({
        FilterOperand: OpcOperand.Attribute,
        Value: 'group1',
      })
      ContFiltElem.FilterOperands.push({
        FilterOperand: OpcOperand.Literal,
        Value: group1Filter,
      })
      ContentFilter.push(ContFiltElem)
    }
    if (typeof group2Filter === 'string' && group2Filter.trim() !== '') {
      ContFiltElem = {
        FilterOperator: OpcFilterOperator.Equals,
        FilterOperands: [],
      }
      ContFiltElem.FilterOperands.push({
        FilterOperand: OpcOperand.Attribute,
        Value: 'group2',
      })
      ContFiltElem.FilterOperands.push({
        FilterOperand: OpcOperand.Literal,
        Value: group2Filter,
      })
      ContentFilter.push(ContFiltElem)
    }
    if (onlyAlarms) {
      ContFiltElem = {
        FilterOperator: OpcFilterOperator.Equals,
        FilterOperands: [],
      }
      ContFiltElem.FilterOperands.push({
        FilterOperand: OpcOperand.Attribute,
        Value: 'alarmed',
      })
      ContFiltElem.FilterOperands.push({
        FilterOperand: OpcOperand.Literal,
        Value: true,
      })
      ContentFilter.push(ContFiltElem)
      ContFiltElem = {
        FilterOperator: OpcFilterOperator.Equals,
        FilterOperands: [],
      }
      ContFiltElem.FilterOperands.push({
        FilterOperand: OpcOperand.Attribute,
        Value: 'invalid',
      })
      ContFiltElem.FilterOperands.push({
        FilterOperand: OpcOperand.Literal,
        Value: false,
      })
      ContentFilter.push(ContFiltElem)
      ContFiltElem = {
        FilterOperator: OpcFilterOperator.Equals,
        FilterOperands: [],
      }
      ContFiltElem.FilterOperands.push({
        FilterOperand: OpcOperand.Attribute,
        Value: 'persistentAlarms',
      })
      ContFiltElem.FilterOperands.push({
        FilterOperand: OpcOperand.Literal,
        Value: true,
      })
      ContentFilter.push(ContFiltElem)
    }

    const ServiceId = OpcServiceCode.ReadRequest // read data service
    const RequestHandle = Math.floor(Math.random() * 100000000)
    const req: OpcRequest = {
      ServiceId: ServiceId,
      Body: {
        RequestHeader: {
          Timestamp: new Date().toISOString(),
          RequestHandle: RequestHandle,
          TimeoutHint: 1500,
          ReturnDiagnostics: OpcDiagnosticInfoMask.LocalizedText,
          AuthenticationToken: null,
        },
        Namespace: OpcNamespaces.Mongodb,
        ContentFilter: ContentFilter,
        MaxAge: 0,
        TimestampsToReturn: TimestampsToReturn.Both,
      },
    }

    const response = await fetch('/Invoke/', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    })

    const data = await response.json()
    if (
      !data.ServiceId ||
      !data.Body ||
      !data.Body.ResponseHeader ||
      !data.Body.ResponseHeader.RequestHandle
    ) {
      return []
    }

    // response must have same request handle and be a read response or service fault
    if (
      data.Body.ResponseHeader.RequestHandle !== RequestHandle ||
      (data.ServiceId !== OpcServiceCode.ReadResponse &&
        data.ServiceId !== OpcServiceCode.ServiceFault)
    ) {
      return []
    }
    if (
      data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good &&
      data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.GoodNoData
    ) {
      // check access control denied, in this case go to initial page
      if (
        data.Body.ResponseHeader.ServiceResult ===
          OpcStatusCodes.BadUserAccessDenied ||
        data.Body.ResponseHeader.ServiceResult ===
          OpcStatusCodes.BadIdentityTokenInvalid ||
        data.Body.ResponseHeader.ServiceResult ===
          OpcStatusCodes.BadIdentityTokenRejected
      ) {
        window.onbeforeunload = null
        window.location.href = '/'
      }
      return []
    }

    if (!('Results' in data.Body)) return []

    return data.Body.Results.map((elem: any) => {
      if (typeof elem.StatusCode === 'number' && elem.StatusCode !== 0) {
        return // reject bad result
      }

      const basePoint = {
        name: elem.NodeId.Id,
        quality: elem.Value.Quality,
        valueString: elem._Properties.valueString,
      }

      switch (elem.Value.Type) {
        case OpcValueTypes.Boolean:
          return {
            ...basePoint,
            value: elem.Value.Body ? 1 : 0,
          }
        case OpcValueTypes.String:
          return {
            ...basePoint,
            value: parseFloat(elem.Value.Body),
            valueString: elem.Value.Body,
          }
        default:
          return {
            ...basePoint,
            value: parseFloat(elem.Value.Body),
          }
      }
    })
  } catch (error) {
    return []
  }
}

/**
 * Retrieves real-time data from the given list of tag names
 * @param {string[]} variables list of tag names
 * @returns {Promise<DataPoint[]>} a promise that resolves to an array of data points
 */
export async function getRealTimeData(
  variables: string[]
): Promise<DataPoint[]> {
  const ServiceId = OpcServiceCode.ReadRequest // read data service
  const RequestHandle = Math.floor(Math.random() * 100000000)
  const req: OpcRequest = {
    ServiceId,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle,
        TimeoutHint: 1500,
        ReturnDiagnostics: OpcDiagnosticInfoMask.LocalizedText,
        AuthenticationToken: null,
      },
      MaxAge: 0,
      TimestampsToReturn: TimestampsToReturn.Both,
      NodesToRead: variables.map((elem) => ({
        NodeId: {
          IdType: OpcKeyType.String,
          Id: elem,
          Namespace: OpcNamespaces.Mongodb,
        },
        AttributeId: OpcAttributeId.Value,
      })),
    },
  }

  try {
    const response = await fetch('/Invoke/', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    })

    const data = await response.json()

    if (
      !data.ServiceId ||
      !data.Body ||
      !data.Body.ResponseHeader ||
      !data.Body.ResponseHeader.RequestHandle
    ) {
      return []
    }

    // response must have same request handle and be a read response or service fault
    if (
      data.Body.ResponseHeader.RequestHandle !== RequestHandle ||
      (data.ServiceId !== OpcServiceCode.ReadResponse &&
        data.ServiceId !== OpcServiceCode.ServiceFault)
    ) {
      return []
    }

    if (
      data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good &&
      data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.GoodNoData
    ) {
      return []
    }

    if (!data.Body.Results) return []

    return data.Body.Results.map((elem: any) => {
      if (typeof elem.StatusCode === 'number' && elem.StatusCode !== 0) {
        return // reject bad result
      }

      const basePoint = {
        name: elem.NodeId.Id,
        quality: elem.Value.Quality,
        valueString: elem._Properties.valueString,
      }

      switch (elem.Value.Type) {
        case OpcValueTypes.Boolean:
          return {
            ...basePoint,
            value: elem.Value.Body ? 1 : 0,
          }
        case OpcValueTypes.String:
          return {
            ...basePoint,
            value: parseFloat(elem.Value.Body),
            valueString: elem.Value.Body,
          }
        default:
          return {
            ...basePoint,
            value: parseFloat(elem.Value.Body),
          }
      }
    })
  } catch (error) {
    return []
  }
}

/**
 * Retrieves historical data for a specified tag within a given time range.
 *
 * @param {string} tag - The tag name for which historical data is requested.
 * @param {Date} timeBegin - The start time for the historical data retrieval.
 * @param {Date | null | undefined} timeEnd - The end time for the historical data retrieval. If null or undefined, the current time is used.
 * @returns {Promise<HistoricalData[]>} A promise that resolves to an array of HistoricalData objects containing the retrieved data.
 */
export async function getHistoricalData(
  tag: string,
  timeBegin: Date,
  timeEnd: Date | null | undefined
): Promise<HistoricalData[]> {
  const ServiceId = OpcServiceCode.HistoryReadRequest // read historical data service
  const RequestHandle = Math.floor(Math.random() * 100000000)
  const req: OpcRequest = {
    ServiceId: ServiceId,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: RequestHandle,
        TimeoutHint: 5000,
        ReturnDiagnostics: OpcDiagnosticInfoMask.LocalizedText,
        AuthenticationToken: null,
      },
      TimestampsToReturn: TimestampsToReturn.Server,
      HistoryReadDetails: {
        ParameterTypeId: OpcServiceCode.ReadRawModifiedDetails,
        ParameterData: {
          IsReadModified: false,
          StartTime: timeBegin.toISOString(),
          EndTime: timeEnd?.toISOString() || new Date().toISOString(),
        },
      },
      NodesToRead: [
        {
          NodeId: {
            IdType: OpcKeyType.String, // string key
            Id: tag, // point string key
            Namespace: OpcNamespaces.Postgresql,
          },
          AttributeId: OpcAttributeId.Value,
        },
      ],
    },
  }

  try {
    const response = await fetch('/Invoke/', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    })

    const data = await response.json()

    if (
      !data.ServiceId ||
      !data.Body ||
      !data.Body.ResponseHeader ||
      !data.Body.ResponseHeader.RequestHandle ||
      !data.Body.Results
    ) {
      return []
    }
    // response must have same request handle and be a read response or service fault
    if (
      data.Body.ResponseHeader.RequestHandle !== RequestHandle ||
      (data.ServiceId !== OpcServiceCode.HistoryReadResponse &&
        data.ServiceId !== OpcServiceCode.ServiceFault)
    ) {
      return []
    }
    if (data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good) {
      return []
    }
    if (
      typeof data.Body.Results[0].StatusCode === 'number' &&
      data.Body.Results[0].StatusCode !== 0
    ) {
      return []
    }

    return data.Body.Results[0].HistoryData.map((elem: any) => {
      if (typeof elem.StatusCode === 'number' && elem.StatusCode !== 0) {
        return // reject bad result
      }

      const basePoint = {
        quality: elem.Value.Quality,
        serverTimestamp: new Date(elem.ServerTimestamp),
        sourceTimestamp: new Date(elem.SourceTimestamp),
      }

      switch (elem.Value.Type) {
        case OpcValueTypes.Boolean:
          return {
            ...basePoint,
            value: elem.Value.Body ? 1 : 0,
          }
        case OpcValueTypes.String:
          return {
            ...basePoint,
            value: parseFloat(elem.Value.Body),
            valueString: elem.Value.Body,
          }
        default:
          return {
            ...basePoint,
            value: parseFloat(elem.Value.Body),
          }
      }
    })
  } catch (error) {
    return []
  }
}

/**
 * Retrieves sequence of events (SOE) data.
 *
 * @param {string[]} group1Filter - An array of group1 (station names) to filter by.
 * @param {boolean} useSourceTime - If true, use source timestamp instead of server timestamp.
 * @param {number} aggregate - 0: for no aggregate, 1: to aggregate for the same tag, 2: to aggregate for the same tag and filter by priority.
 * @param {number} limit - The number of results to return.
 * @param {any} timeBegin - The start time for the query. If null or undefined, will be set to null.
 * @param {any} timeEnd - The end time for the query. If null or undefined, will be set to null.
 * @returns {Promise<SoeData[]>} A promise that resolves to an array of SoeData objects containing the retrieved data.
 */
export async function getSoeData(
  group1Filter: string[],
  useSourceTime: boolean,
  aggregate: number,
  limit: number,
  timeBegin: any,
  timeEnd: any
): Promise<SoeData[]> {
  if (timeBegin === undefined || timeBegin === null) timeBegin = null
  else if (typeof timeBegin?.getMonth === 'function')
    timeBegin = timeBegin.toISOString()

  if (timeEnd === undefined) timeEnd = null
  else if (typeof timeEnd?.getMonth === 'function')
    timeEnd = timeEnd.toISOString()

  let ContentFilter = []
  let ContFiltElem = {
    FilterOperator: OpcFilterOperator.InList as any,
    FilterOperands: [] as any,
  }

  for (let i = 0; i < group1Filter.length; i++) {
    ContFiltElem.FilterOperands.push({
      FilterOperand: OpcOperand.Literal,
      Value: group1Filter[i],
    })
  }

  if (ContFiltElem.FilterOperands.length > 0) ContentFilter.push(ContFiltElem)

  if (aggregate === 2) {
    // panic, filter priority also
    ContFiltElem = {
      FilterOperator: OpcFilterOperator.LessThanOrEqual,
      FilterOperands: [],
    }
    ContentFilter.push(ContFiltElem)
  }

  let AggregateFilter = undefined
  if (aggregate) {
    AggregateFilter = { AggregateType: 'Count' }
  }

  const ServiceId = OpcServiceCode.HistoryReadRequest // history read service
  const RequestHandle = Math.floor(Math.random() * 100000000)
  const req: OpcRequest = {
    ServiceId: ServiceId,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: RequestHandle,
        TimeoutHint: 5000,
        ReturnDiagnostics: OpcDiagnosticInfoMask.LocalizedText,
        AuthenticationToken: null,
      },
      TimestampsToReturn:
        useSourceTime ? TimestampsToReturn.Source : TimestampsToReturn.Both,
      HistoryReadDetails: {
        ParameterTypeId: OpcServiceCode.ReadRawModifiedDetails,
        ParameterData: {
          IsReadModified: false,
          StartTime: timeBegin,
          EndTime: timeEnd,
          NumValuesPerNode: limit,
        },
      },
      Namespace: OpcNamespaces.Mongodb, // directs query to mongodb instead of postgresql
      ContentFilter: ContentFilter,
      AggregateFilter: AggregateFilter,
    },
  }

  try {
    const response = await fetch('/Invoke/', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    })
    const data = await response.json()

    if (
      !data.ServiceId ||
      !data.Body ||
      !data.Body.ResponseHeader ||
      !data.Body.ResponseHeader.RequestHandle
    ) {
      return []
    }
    // response must have same request handle and be a read response or service fault
    if (
      data.Body.ResponseHeader.RequestHandle !== RequestHandle ||
      (data.ServiceId !== OpcServiceCode.HistoryReadResponse &&
        data.ServiceId !== OpcServiceCode.ServiceFault)
    ) {
      return []
    }
    if (
      data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good &&
      data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.GoodNoData
    ) {
      return []
    }
    if (
      typeof data.Body.Results !== 'object' ||
      data.Body.Results.length === 0
    ) {
      return [] // no data
    }

    return data.Body.Results.map((elem: any) => {
      if (typeof elem.StatusCode === 'number' && elem.StatusCode !== 0) {
        return // reject bad result
      }

      return {
        quality: elem.HistoryData[0].Value.Quality,
        serverTimestamp: new Date(elem.HistoryData[0].ServerTimestamp),
        sourceTimestamp: new Date(elem.HistoryData[0].SourceTimestamp),
        sourceTimestampOk: elem.HistoryData[0].SourceTimestampOk,
        valueString: elem.HistoryData[0].Value.Body,
        name: elem.NodeId.Id,
        priority: elem._Properties.priority,
        group1: elem._Properties.group1,
        description: elem._Properties.description,
        eventId: elem._Properties.eventId,
      }
    })
  } catch (error) {
    return []
  }
}

/**
 * Issue a command for a tag
 * @param {string} commandTag - tag of the command
 * @param {number} value - value of the command
 * @returns {Promise<CommandResult>} a promise that resolves to a CommandResult object
 */
export async function issueCommand(
  commandTag: string,
  value: number
): Promise<CommandResult> {
  const ServiceId = OpcServiceCode.WriteRequest // write data service
  const RequestHandle = Math.floor(Math.random() * 100000000)
  const req: OpcRequest = {
    ServiceId: ServiceId,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: RequestHandle,
        TimeoutHint: 1500,
        ReturnDiagnostics: OpcDiagnosticInfoMask.LocalizedText,
        AuthenticationToken: null,
      },
      NodesToWrite: [
        {
          NodeId: {
            IdType: OpcKeyType.String,
            Id: commandTag,
            Namespace: OpcNamespaces.Mongodb,
          },
          AttributeId: OpcAttributeId.Value,
          Value: {
            Type: OpcValueTypes.Double,
            Body: value,
          },
        },
      ],
    },
  }

  try {
    const response = await fetch('/Invoke/', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    })
    const data = await response.json()
    if (
      !data.ServiceId ||
      !data.Body ||
      !data.Body.ResponseHeader ||
      !data.Body.ResponseHeader.RequestHandle ||
      !data.Body.Results ||
      data.ServiceId !== OpcServiceCode.WriteResponse ||
      data.Body.ResponseHeader.RequestHandle !== RequestHandle ||
      data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good
    ) {
      return {
        ok: false,
        error: 'Request Error!',
        commandHandle: '',
      }
    }
    if (data.Body.Results[0] !== OpcStatusCodes.Good) {
      return {
        ok: false,
        error: 'Error Executing Command!',
        commandHandle: '',
      }
    }
    return {
      ok: true,
      error: '',
      commandHandle: data.Body._CommandHandles[0],
    }
  } catch (error) {
    return {
      ok: false,
      error: error as string,
      commandHandle: '',
    }
  }
}

/**
 * Track command acknowledgment from server/protocol, use command handle identify a specific command
 * @param {string} commandTag - the command tag name
 * @param {string} commandHandle - the command handle (in the standard this is an integer but here is MongoDB OID string)
 * @returns {Promise<(typeof OpcStatusCodes)[keyof typeof OpcStatusCodes]>} - a promise that resolves to an OpcStatusCodes enum value
 */
export async function getCommandAckStatus(
  commandTag: string,
  commandHandle: string
): Promise<(typeof OpcStatusCodes)[keyof typeof OpcStatusCodes]> {
  // track command acknowledgment from server/protocol, use CHANDLE to verify results

  const ServiceId = OpcServiceCode.ReadRequest // READ, query command ack results reading attribute 12 (EventNotifier)
  const RequestHandle = Math.floor(Math.random() * 100000000)
  const req: OpcRequest = {
    ServiceId: ServiceId,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: RequestHandle,
        TimeoutHint: 1250,
        ReturnDiagnostics: OpcDiagnosticInfoMask.LocalizedText,
        AuthenticationToken: null,
      },
      MaxAge: 0,
      NodesToRead: [
        {
          NodeId: {
            IdType: OpcKeyType.String,
            Id: commandTag,
            Namespace: OpcNamespaces.Mongodb,
          },
          AttributeId: OpcAttributeId.EventNotifier,
          ClientHandle: commandHandle,
        },
      ],
    },
  }

  try {
    const response = await fetch('/Invoke/', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    })

    const data = await response.json()

    if (
      !data.ServiceId ||
      !data.Body ||
      !data.Body.ResponseHeader ||
      !data.Body.ResponseHeader.RequestHandle
    )
      return OpcStatusCodes.BadServiceUnsupported

    // response must have same request handle and be a data change notification response or service fault
    if (
      data.Body.ResponseHeader.RequestHandle !== RequestHandle ||
      (data.ServiceId !== OpcServiceCode.DataChangeNotification &&
        data.ServiceId !== OpcServiceCode.ServiceFault) ||
      typeof data.Body.MonitoredItems !== 'object'
    ) {
      return OpcStatusCodes.BadInternalError
    }
    if (data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good) {
      return OpcStatusCodes.BadInternalError
    }
    if (data.Body.MonitoredItems.length === 0) {
      return OpcStatusCodes.BadNoDataAvailable
    }

    if (
      data.Body.MonitoredItems[0].Value.StatusCode ===
      OpcStatusCodes.BadWaitingForResponse
    )
      return OpcStatusCodes.BadWaitingForResponse
    else if (
      data.Body.MonitoredItems[0].Value.StatusCode === OpcStatusCodes.Good
    )
      return OpcStatusCodes.Good
    else if (
      data.Body.MonitoredItems[0].Value.StatusCode === OpcStatusCodes.Bad
    )
      return OpcStatusCodes.Bad

    return OpcStatusCodes.BadInvalidState
  } catch (error) {
    return OpcStatusCodes.BadInternalError
  }
}

/**
 * Write the given extended properties of a tag.
 * @param {string} tag - the tag name
 * @param {ExtendedProperties} properties - the properties to write
 * @returns {Promise<boolean>} a promise that resolves to true if the write was successful, false otherwise
 */
export async function writeTagProperties(
  tag: string,
  properties: ExtendedProperties
): Promise<boolean> {
  const ServiceId = OpcServiceCode.WriteRequest // write data service
  const RequestHandle = Math.floor(Math.random() * 100000000)
  const req: OpcRequest = {
    ServiceId: ServiceId,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: RequestHandle,
        TimeoutHint: 1250,
        ReturnDiagnostics: OpcDiagnosticInfoMask.LocalizedText,
        AuthenticationToken: null,
      },
      NodesToWrite: [
        {
          NodeId: {
            IdType: OpcKeyType.String,
            Id: tag,
            Namespace: OpcNamespaces.Mongodb,
          },
          AttributeId: OpcAttributeId.Description,
          Value: {
            _Properties: properties,
          },
        },
      ],
    },
  }

  try {
    const response = await fetch('/Invoke/', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    })
    const data = await response.json()
    if (
      !data.ServiceId ||
      !data.Body ||
      !data.Body.ResponseHeader ||
      !data.Body.ResponseHeader.RequestHandle ||
      !data.Body.Results ||
      data.ServiceId !== OpcServiceCode.WriteResponse ||
      data.Body.ResponseHeader.RequestHandle !== RequestHandle ||
      data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good
    ) {
      return false
    }
    if (data.Body.Results[0] !== OpcStatusCodes.Good) {
      return false
    }
    return true
  } catch (error) {
    return false
  }
}

/**
 * Acknowledge alarm(s)/event(s) for a given tag.
 * To acknowledge all alarms for all tags call ackAlarmsEvents('', OpcAcknowledge.AckAllAlarms)
 * To acknowledge all events for all tags call ackAlarmsEvents('', OpcAcknowledge.AckAllEvents)
 * To remove all events for all tags call ackAlarmsEvents('', OpcAcknowledge.RemoveAllEvents)
 *
 * @param {string} tag - The tag name for which alarms/events are to be acknowledged.
 * @param {number} action - A bit mask of the enum type OpcAcknowledge that specifies the acknowledgment action.
 * @returns {Promise<boolean>} A promise that resolves to true if the alarm acknowledgment is successful, false otherwise.
 */
export async function ackAlarmsEvents(
  tag: string,
  action: number // bit mask of enum type OpcAcknowledge
): Promise<boolean> {
  const ServiceId = OpcServiceCode.WriteRequest
  const RequestHandle = Math.floor(Math.random() * 100000000)
  const req = {
    ServiceId: ServiceId,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: RequestHandle,
        TimeoutHint: 1000,
        ReturnDiagnostics: OpcDiagnosticInfoMask.LocalizedText,
        AuthenticationToken: null,
      },
      NodesToWrite: [
        {
          NodeId: {
            IdType: OpcKeyType.String,
            Id: tag,
            Namespace: OpcNamespaces.Mongodb,
          },
          AttributeId: OpcAttributeId.ExtendedAlarmEventsAck,
          Value: {
            Type: OpcValueTypes.Integer,
            Body: action,
          },
        },
      ],
    },
  }

  try {
    const response = await fetch('/Invoke/', {
      method: 'POST',
      body: JSON.stringify(req),
      headers: {
        'Content-Type': 'application/json',
      },
    })

    const data = await response.json()

    if (
      !data.ServiceId ||
      !data.Body ||
      !data.Body.ResponseHeader ||
      !data.Body.ResponseHeader.RequestHandle ||
      !data.Body.Results ||
      data.ServiceId !== OpcServiceCode.WriteResponse ||
      data.Body.ResponseHeader.RequestHandle !== RequestHandle ||
      (data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good &&
        data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.GoodNoData)
    ) {
      return false
    }

    return true
  } catch (error) {
    return false
  }
}
