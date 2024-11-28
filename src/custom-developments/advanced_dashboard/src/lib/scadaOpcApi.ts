import {
  OpcAttributeId,
  OpcFilterOperator,
  OpcKeyType,
  OpcNamespaceMongodb,
  OpcNamespacePostgresql,
  OpcOperand,
  OpcServiceCode,
  OpcStatusCodes,
  OpcValueTypes,
  TimestampsToReturn,
} from './opcCodes'

export interface OpcNodeId {
  IdType: number
  Id: string | undefined
  Namespace: number
}

export interface OpcNodeToRead {
  NodeId: OpcNodeId
  AttributeId: number
}

export interface OpcRequestHeader {
  Timestamp: string
  RequestHandle: number
  TimeoutHint: number
  ReturnDiagnostics: number
  AuthenticationToken: null
}

export interface OpcRequestBody {
  RequestHeader: OpcRequestHeader
  MaxAge: number
  TimestampsToReturn: number
  NodesToRead: OpcNodeToRead[]
}

export interface OpcRequest {
  ServiceId: number
  Body: OpcRequestBody
}

export interface DataPoint {
  name: string
  value: number
  valueString: string
  quality: number
  timestamp: string
  group1: string
  group2: string
  description: string
}

export interface HistoricalData {
  value: number
  valueString: string
  quality: number
  serverTimestamp: Date
  sourceTimestamp: Date
}

export interface SoeData {
  name: string
  description: string
  group1: string
  eventId: string
  priority: number
  valueString: string
  quality: number
  serverTimestamp: Date
  sourceTimestamp: Date
  sourceTimestampOk: boolean
}

// Get the group1 (station names) list
export async function getGroup1List (): Promise<string[]> {
  // use OPC web hmi protocol https://prototyping.opcfoundation.org/
  const ServiceId = OpcServiceCode.Extended_RequestUniqueAttributeValues // read data service
  const RequestHandle = Math.floor(Math.random() * 100000000)
  const req = {
    ServiceId: ServiceId,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: RequestHandle,
        TimeoutHint: 1500,
        ReturnDiagnostics: 2,
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
      console.log('RequestUniqueAttributeValues invalid service response!')
      return []
    }

    // response must have same request handle and be a read response or service fault
    if (
      data.Body.ResponseHeader.RequestHandle !== RequestHandle ||
      (data.ServiceId !==
        OpcServiceCode.Extended_ResponseUniqueAttributeValues &&
        data.ServiceId !== OpcServiceCode.ServiceFault)
    ) {
      console.log(
        'RequestUniqueAttributeValues invalid or unexpected service response!'
      )
      return []
    }
    if (data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good) {
      console.log('RequestUniqueAttributeValues service error!')
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
    console.log(error)
    return []
  }
}

// Get realtime filtered data
export async function getRealtimeFilteredData (
  group1Filter: string,
  group2Filter: string,
  onlyAlarms: boolean
): Promise<DataPoint[]> {
  try {
    let ContentFilter = []
    let ContFiltElem = {
      FilterOperator: OpcFilterOperator.Equals,
      FilterOperands: [] as any,
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

    // use OPC web hmi protocol https://prototyping.opcfoundation.org/
    const ServiceId = OpcServiceCode.ReadRequest // read data service
    const RequestHandle = Math.floor(Math.random() * 100000000)
    const req = {
      ServiceId: ServiceId,
      Body: {
        RequestHeader: {
          Timestamp: new Date().toISOString(),
          RequestHandle: RequestHandle,
          TimeoutHint: 1500,
          ReturnDiagnostics: 2,
          AuthenticationToken: null,
        },
        Namespace: OpcNamespaceMongodb,
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
      console.log('ReadRequest invalid service response!')
      return []
    }

    // response must have same request handle and be a read response or service fault
    if (
      data.Body.ResponseHeader.RequestHandle !== RequestHandle ||
      (data.ServiceId !== OpcServiceCode.ReadResponse &&
        data.ServiceId !== OpcServiceCode.ServiceFault)
    ) {
      console.log('ReadRequest invalid or unexpected service response!')
      return []
    }
    if (
      data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good &&
      data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.GoodNoData
    ) {
      console.log('ReadRequest service error!')
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
    console.log(error)
    return []
  }
}

// Get realtime data from tag names list
export async function readRealTimeData (
  variables: string[]
): Promise<DataPoint[]> {
  // use OPC web hmi protocol https://prototyping.opcfoundation.org/
  const ServiceId = OpcServiceCode.ReadRequest // read data service
  const RequestHandle = Math.floor(Math.random() * 100000000)
  const req: OpcRequest = {
    ServiceId,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle,
        TimeoutHint: 1500,
        ReturnDiagnostics: 2,
        AuthenticationToken: null,
      },
      MaxAge: 0,
      TimestampsToReturn: TimestampsToReturn.Both,
      NodesToRead: variables.map((elem) => ({
        NodeId: {
          IdType: OpcKeyType.String,
          Id: elem,
          Namespace: OpcNamespaceMongodb,
        },
        AttributeId: OpcAttributeId.Value,
      })),
    },
  }

  try {
    const response = await fetch('http://127.0.0.1:8080/Invoke/', {
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
      console.log('ReadRequest invalid service response!')
      return []
    }

    // response must have same request handle and be a read response or service fault
    if (
      data.Body.ResponseHeader.RequestHandle !== RequestHandle ||
      (data.ServiceId !== OpcServiceCode.ReadResponse &&
        data.ServiceId !== OpcServiceCode.ServiceFault)
    ) {
      console.log('ReadRequest invalid or unexpected service response!')
      return []
    }

    if (
      data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good &&
      data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.GoodNoData
    ) {
      console.log('ReadRequest service error!')
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
    console.error(error)
    return []
  }
}

// Get historical data for a tag
export async function getHistoricalData (
  tag: string,
  timeBegin: Date,
  timeEnd: Date | null | undefined
): Promise<HistoricalData[]> {
  // use OPC web hmi protocol https://prototyping.opcfoundation.org/
  const ServiceId = OpcServiceCode.HistoryReadRequest // read historical data service
  const RequestHandle = Math.floor(Math.random() * 100000000)
  const req = {
    ServiceId: ServiceId,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: RequestHandle,
        TimeoutHint: 5000,
        ReturnDiagnostics: 2,
        AuthenticationToken: null,
      },
      TimestampsToReturn: TimestampsToReturn.Server,
      HistoryReadDetails: {
        ParameterTypeId: OpcServiceCode.ReadRawModifiedDetails,
        ParameterData: {
          IsModified: false,
          StartTime: timeBegin.toISOString(),
          EndTime: timeEnd || new Date().toISOString(),
        },
      },
      NodesToRead: [
        {
          NodeId: {
            IdType: OpcKeyType.String, // string key
            Id: tag, // point string key
            Namespace: OpcNamespacePostgresql,
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
      console.log('Historian invalid service response!')
      return []
    }
    // response must have same request handle and be a read response or service fault
    if (
      data.Body.ResponseHeader.RequestHandle !== RequestHandle ||
      (data.ServiceId !== OpcServiceCode.HistoryReadResponse &&
        data.ServiceId !== OpcServiceCode.ServiceFault)
    ) {
      console.log('Historian invalid or unexpected service response!')
      return []
    }
    if (data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good) {
      console.log('Historian service error!')
      return []
    }
    if (
      typeof data.Body.Results[0].StatusCode === 'number' &&
      data.Body.Results[0].StatusCode !== 0
    ) {
      console.log('Historical data not found for point ' + tag + ' !')
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
    console.log(error)
    return []
  }
}

// Issue a command for a tag
export async function issueCommand (
  commandTag: string,
  value: number
): Promise<string> {
  // use OPC web hmi protocol https://prototyping.opcfoundation.org/
  const ServiceId = OpcServiceCode.WriteRequest // write data service
  const RequestHandle = Math.floor(Math.random() * 100000000)
  const req = {
    ServiceId: ServiceId,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: RequestHandle,
        TimeoutHint: 1500,
        ReturnDiagnostics: 2,
        AuthenticationToken: null,
      },
      NodesToWrite: [
        {
          NodeId: {
            IdType: OpcKeyType.String,
            Id: commandTag,
            Namespace: OpcNamespaceMongodb,
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
      return 'Request Error!'
    }
    if (data.Body.Results[0] !== OpcStatusCodes.Good) {
      return 'Error executing command!'
    }
    return ''
  } catch (error) {
    return error as string
  }
}

// Get Sequence of Events (SOE) data
export async function getSoeData (
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

  let AggregateFilter = null
  if (aggregate) {
    AggregateFilter = { AggregateType: 'Count' }
  }

  // use OPC web hmi protocol https://prototyping.opcfoundation.org/
  const ServiceId = OpcServiceCode.HistoryReadRequest // history read service
  const RequestHandle = Math.floor(Math.random() * 100000000)
  const req = {
    ServiceId: ServiceId,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: RequestHandle,
        TimeoutHint: 5000,
        ReturnDiagnostics: 2,
        AuthenticationToken: null,
      },
      TimestampsToReturn: useSourceTime
        ? TimestampsToReturn.Source
        : TimestampsToReturn.Both,
      HistoryReadDetails: {
        ParameterTypeId: OpcServiceCode.ReadRawModifiedDetails,
        ParameterData: {
          IsModified: false,
          StartTime: timeBegin,
          EndTime: timeEnd,
          NumValuesPerNode: limit,
        },
      },
      Namespace: OpcNamespaceMongodb, // directs query to mongodb instead of postgresql
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
      console.log('Historian invalid service response!')
      return []
    }
    // response must have same request handle and be a read response or service fault
    if (
      data.Body.ResponseHeader.RequestHandle !== RequestHandle ||
      (data.ServiceId !== OpcServiceCode.HistoryReadResponse &&
        data.ServiceId !== OpcServiceCode.ServiceFault)
    ) {
      console.log('Historian invalid or unexpected service response!')
      return []
    }
    if (
      data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good &&
      data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.GoodNoData
    ) {
      console.log('Historian service error!')
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
    console.log(error)
    return []
  }
}
