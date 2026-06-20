/*
 * OPC Web-HMI client for the native Tabular/Alarms viewer.
 * Ports the request envelopes and response normalization from public/tabular.html
 * into a small promise-based module. Every server call goes through POST /Invoke/.
 *
 * {json:scada} - Copyright 2020-2026 - Ricardo L. Olsen
 */

import {
  OpcServiceCode,
  OpcAttributeId,
  OpcKeyType,
  OpcValueTypes,
  OpcStatusCodes,
  OpcNamespaceMongodb,
  OpcFilterOperator,
  OpcOperand,
  OpcAcknowledge,
  OpcNamespacePostgresql,
  TimestampsToReturn,
  Flags,
  BEEP_POINTKEY,
} from './opcCodes'
import {
  formatDateTime,
  formatAlarmTime,
  formatDate,
  formatTime,
  utcMillis,
} from './viewerHelpers'

// Caller (the component) sets this to redirect to /login on access-denied.
let accessDeniedHandler = () => {}
export function onAccessDenied(fn) {
  accessDeniedHandler = fn
}

function newHandle() {
  return Math.floor(Math.random() * 100000000)
}

// Low-level invoke with timeout via AbortController.
async function invoke(body, timeoutMs = 3000) {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeoutMs)
  try {
    const resp = await fetch('/Invoke/', {
      method: 'POST',
      signal: controller.signal,
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
    return await resp.json()
  } finally {
    clearTimeout(timer)
  }
}

function isValidResponse(data, handle, expectServiceIds) {
  if (
    !data ||
    !data.ServiceId ||
    !data.Body ||
    !data.Body.ResponseHeader ||
    !data.Body.ResponseHeader.RequestHandle
  )
    return false
  if (data.Body.ResponseHeader.RequestHandle !== handle) return false
  if (expectServiceIds && !expectServiceIds.includes(data.ServiceId)) return false
  return true
}

function checkAccessDenied(data) {
  const sr = data?.Body?.ResponseHeader?.ServiceResult
  if (
    sr === OpcStatusCodes.BadUserAccessDenied ||
    sr === OpcStatusCodes.BadIdentityTokenInvalid ||
    sr === OpcStatusCodes.BadIdentityTokenRejected
  ) {
    accessDeniedHandler()
    return true
  }
  return false
}

// --- distinct group1 (station) list -----------------------------------------

export async function getGroup1List() {
  const handle = newHandle()
  const req = {
    ServiceId: OpcServiceCode.Extended_RequestUniqueAttributeValues,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: handle,
        TimeoutHint: 1500,
        ReturnDiagnostics: 2,
        AuthenticationToken: null,
      },
      AttributeId: OpcAttributeId.ExtendedGroup1,
    },
  }
  const data = await invoke(req, 1500)
  if (
    !isValidResponse(data, handle, [
      OpcServiceCode.Extended_ResponseUniqueAttributeValues,
      OpcServiceCode.ServiceFault,
    ])
  )
    return ['']
  if (data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good) return ['']
  const list = ['']
  if ('Results' in data.Body)
    data.Body.Results.forEach((el) => {
      if (el.Value.Body !== null && el.Value.Body !== '') list.push(el.Value.Body)
    })
  return list
}

// --- normalization of a read result into a viewer row ------------------------

function buildContentFilter(station, bay, onlyAlarms) {
  const cf = []
  const eq = (attr, value) => ({
    FilterOperator: OpcFilterOperator.Equals,
    FilterOperands: [
      { FilterOperand: OpcOperand.Attribute, Value: attr },
      { FilterOperand: OpcOperand.Literal, Value: value },
    ],
  })
  if (typeof station === 'string' && station.trim() !== '') cf.push(eq('group1', station))
  if (typeof bay === 'string' && bay.trim() !== '') cf.push(eq('group2', bay))
  if (onlyAlarms) {
    cf.push(eq('alarmed', true))
    cf.push(eq('invalid', false))
    cf.push(eq('persistentAlarms', true))
  }
  return cf
}

// Convert one OPC read Result element + its _Properties into a normalized point.
function normalizePoint(element, cfg) {
  const prop = element._Properties || {}
  const key = prop._id
  const valType = element.Value.Type
  const body = element.Value.Body
  const quality = element.Value.Quality

  // flags bitmask (tabular.html:1842-1857)
  let flags = quality !== OpcStatusCodes.Good ? Flags.FAILED : 0
  if (valType === OpcValueTypes.Double) {
    flags |= Flags.ANALOG
    if (prop.frozen === true) flags |= Flags.FROZEN
  }
  if ('alarmRange' in prop) flags |= prop.alarmRange !== 0 ? Flags.ABNORMAL : 0
  if (prop.alarmed === true) flags |= Flags.ALARMED
  if (prop.alarmDisabled === true) flags |= Flags.ALARM_DISABLED
  if (prop.origin === 'manual') flags |= Flags.MANUAL

  // value display
  let valueStr = body
  if (valType === OpcValueTypes.Double)
    valueStr = parseFloat(body).toFixed(3) + ' ' + (prop.unit || '')
  else if (valType === OpcValueTypes.Boolean)
    valueStr = body === true ? prop.stateTextTrue : prop.stateTextFalse

  // persistent marker
  let persistent = ''
  if (
    (prop.alarmState === 1 && body === true) ||
    (prop.alarmState === 0 && body === false)
  )
    persistent = 'P'

  // qualifier string (tabular.html:1879-1888)
  const qualifier =
    '' +
    prop.priority +
    (prop.alarmed ? 'L' : '') +
    persistent +
    (prop.annotation !== '' && prop.annotation != null ? 'A' : '') +
    (prop.alarmDisabled ? 'I' : '') +
    (prop.isEvent ? 'E' : '') +
    (prop.frozen === true ? 'U' : '') +
    (prop.commandOfSupervised !== 0 ? 'K' : '') +
    (prop.origin === 'manual' ? 'M' : '') +
    (prop.timeTagAtSourceOk === 'false' ? 'T' : '') +
    (quality !== OpcStatusCodes.Good ? 'F' : '')

  const descr =
    (prop.group2 || '') +
    ' | ' +
    (prop.ungroupedDescription !== '' && prop.ungroupedDescription != null
      ? prop.ungroupedDescription
      : prop.description || '')

  return {
    key,
    tag: element.NodeId.Id,
    station: prop.group1 || '',
    bay: prop.group2 || '',
    descr,
    type: prop.type,
    value: typeof body === 'number' ? body : parseFloat(body),
    valueStr,
    unit: prop.unit || '',
    flags,
    qualifier,
    priority: prop.priority,
    alarmed: !!prop.alarmed,
    persistent: persistent === 'P',
    annotation: typeof prop.annotation === 'string' ? prop.annotation : '',
    notes: typeof prop.notes === 'string' ? prop.notes : '',
    alarmDisabled: !!prop.alarmDisabled,
    frozen: prop.frozen === true,
    manual: prop.origin === 'manual',
    isEvent: !!prop.isEvent,
    commandKey: prop.commandOfSupervised || 0,
    supervisedOfCommand: prop.supervisedOfCommand || 0,
    hiLimit:
      isNaN(prop.hiLimit) || prop.hiLimit === null ? Infinity : prop.hiLimit,
    loLimit:
      isNaN(prop.loLimit) || prop.loLimit === null ? -Infinity : prop.loLimit,
    hysteresis: prop.hysteresis,
    stateTextTrue: prop.stateTextTrue,
    stateTextFalse: prop.stateTextFalse,
    origin: prop.origin,
    group1: prop.group1 || '',
    alarmTime: formatAlarmTime(
      prop.alarmed && prop.timeTagAlarm != null ? prop.timeTagAlarm : null,
      cfg.TabularViewer_LocaleDateTime,
      cfg.TabularViewer_LocaleDateTimeOptions
    ),
    fieldTime: formatDateTime(
      element.SourceTimestamp,
      cfg.TabularViewer_LocaleDateTime,
      cfg.TabularViewer_LocaleDateTimeOptions
    ),
    quality,
    // beep pseudo-point extras
    beepType: prop.beepType,
    beepGroup1List: prop.beepGroup1List,
    valueRaw: body,
  }
}

// --- the main grid feed ------------------------------------------------------

export async function readFiltered({ station, bay, onlyAlarms }, cfg) {
  const handle = newHandle()
  const req = {
    ServiceId: OpcServiceCode.ReadRequest,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: handle,
        TimeoutHint: 3000,
        ReturnDiagnostics: 2,
        AuthenticationToken: null,
      },
      Namespace: OpcNamespaceMongodb,
      ContentFilter: buildContentFilter(station, bay, onlyAlarms),
      MaxAge: 0,
      TimestampsToReturn: TimestampsToReturn.Both,
    },
  }
  const data = await invoke(req, 3000)
  if (
    !isValidResponse(data, handle, [
      OpcServiceCode.ReadResponse,
      OpcServiceCode.ServiceFault,
    ])
  )
    return []
  if (
    data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good &&
    data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.GoodNoData
  ) {
    checkAccessDenied(data)
    return []
  }
  const rows = []
  if ('Results' in data.Body)
    data.Body.Results.forEach((el) => {
      if (typeof el.StatusCode === 'number' && el.StatusCode !== 0) return
      if (el.NodeId.IdType !== OpcKeyType.String) return
      const prop = el._Properties || {}
      // exclude command points; keep _System pseudo-points
      if (prop.origin === 'command') return
      if (!(prop._id > 0 || station === '_System')) return
      rows.push(normalizePoint(el, cfg))
    })
  return rows
}

// --- read specific points (info dialog, beep status) -------------------------

export async function readPoints(keysOrTags, askInfo, cfg) {
  if (!Array.isArray(keysOrTags) || keysOrTags.length === 0) return []
  const handle = newHandle()
  const nodesToRead = keysOrTags.map((el) => {
    const numeric = !isNaN(parseInt(el))
    return {
      NodeId: {
        IdType: numeric ? OpcKeyType.Numeric : OpcKeyType.String,
        Id: numeric ? parseInt(el) : el,
        Namespace: OpcNamespaceMongodb,
      },
      AttributeId: askInfo ? OpcAttributeId.Description : OpcAttributeId.Value,
    }
  })
  const req = {
    ServiceId: OpcServiceCode.ReadRequest,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: handle,
        TimeoutHint: 3000,
        ReturnDiagnostics: 2,
        AuthenticationToken: null,
      },
      MaxAge: 0,
      TimestampsToReturn: TimestampsToReturn.Both,
      NodesToRead: nodesToRead,
    },
  }
  const data = await invoke(req, 3000)
  if (
    !isValidResponse(data, handle, [
      OpcServiceCode.ReadResponse,
      OpcServiceCode.ServiceFault,
    ])
  )
    return []
  if (
    data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good &&
    data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.GoodNoData
  ) {
    checkAccessDenied(data)
    return []
  }
  const points = []
  if ('Results' in data.Body)
    data.Body.Results.forEach((el) => {
      if (typeof el.StatusCode === 'number' && el.StatusCode !== 0) return
      if (el.NodeId.IdType !== OpcKeyType.String) return
      points.push(normalizePoint(el, cfg))
    })
  // the server's response timestamp — the moment the data was served
  points.serverTime = data.Body.ResponseHeader && data.Body.ResponseHeader.Timestamp
  return points
}

// --- alarm acknowledge / silence beep ----------------------------------------

export async function writeAck({ pointId, action }) {
  const handle = newHandle()
  const numeric = !isNaN(parseInt(pointId))
  const req = {
    ServiceId: OpcServiceCode.WriteRequest,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: handle,
        TimeoutHint: 1500,
        ReturnDiagnostics: 2,
        AuthenticationToken: null,
      },
      NodesToWrite: [
        {
          NodeId: {
            IdType: numeric ? OpcKeyType.Numeric : OpcKeyType.String,
            Id: numeric ? parseInt(pointId) : pointId,
            Namespace: OpcNamespaceMongodb,
          },
          AttributeId: OpcAttributeId.ExtendedAlarmEventsAck,
          Value: { Type: OpcValueTypes.Integer, Body: action },
        },
      ],
    },
  }
  return invoke(req, 1500)
}

// --- command execution -------------------------------------------------------

export async function writeCommand({ pointKey, value, isString }) {
  const handle = newHandle()
  const req = {
    ServiceId: OpcServiceCode.WriteRequest,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: handle,
        TimeoutHint: 1500,
        ReturnDiagnostics: 2,
        AuthenticationToken: null,
      },
      NodesToWrite: [
        {
          NodeId: {
            IdType: OpcKeyType.Numeric,
            Id: pointKey,
            Namespace: OpcNamespaceMongodb,
          },
          AttributeId: OpcAttributeId.Value,
          Value: {
            Type: isString ? OpcValueTypes.String : OpcValueTypes.Double,
            Body: value,
          },
        },
      ],
    },
  }
  const data = await invoke(req, 1500)
  const ok =
    isValidResponse(data, handle, [OpcServiceCode.WriteResponse]) &&
    data.Body.ResponseHeader.ServiceResult === OpcStatusCodes.Good &&
    Array.isArray(data.Body.Results) &&
    data.Body.Results[0] === OpcStatusCodes.Good
  return {
    ok,
    handle: data?.Body?._CommandHandles ? data.Body._CommandHandles[0] : null,
  }
}

// --- command ack polling -----------------------------------------------------

export async function readCommandAck({ cmdKey, clientHandle }) {
  const handle = newHandle()
  const req = {
    ServiceId: OpcServiceCode.ReadRequest,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: handle,
        TimeoutHint: 1250,
        ReturnDiagnostics: 2,
        AuthenticationToken: null,
      },
      MaxAge: 0,
      NodesToRead: [
        {
          NodeId: {
            IdType: OpcKeyType.Numeric,
            Id: parseInt(cmdKey),
            Namespace: OpcNamespaceMongodb,
          },
          AttributeId: OpcAttributeId.EventNotifier,
          ClientHandle: clientHandle,
        },
      ],
    },
  }
  const data = await invoke(req, 1250)
  if (
    !data ||
    !data.ServiceId ||
    !data.Body ||
    !data.Body.ResponseHeader ||
    data.Body.ResponseHeader.RequestHandle !== handle
  )
    return { status: 'unknown' }
  if (
    (data.ServiceId !== OpcServiceCode.DataChangeNotification &&
      data.ServiceId !== OpcServiceCode.ServiceFault) ||
    typeof data.Body.MonitoredItems !== 'object'
  )
    return { status: 'unknown' }
  if (data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good)
    return { status: 'error' }
  const sc = data.Body.MonitoredItems[0].Value.StatusCode
  if (sc === OpcStatusCodes.BadWaitingForResponse) return { status: 'waiting' }
  if (sc === OpcStatusCodes.Good) return { status: 'ok' }
  if (sc === OpcStatusCodes.Bad) {
    const st = data.Body.ResponseHeader?.StringTable
    return {
      status: 'rejected',
      message: st && st.length > 0 ? st[0] : null,
    }
  }
  return { status: 'unknown' }
}

// --- write point properties (limits, annotation, notes, manual/substitution) -

export async function writeProperties({ pointKey, props }) {
  const handle = newHandle()
  const req = {
    ServiceId: OpcServiceCode.WriteRequest,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: handle,
        TimeoutHint: 1250,
        ReturnDiagnostics: 2,
        AuthenticationToken: null,
      },
      NodesToWrite: [
        {
          NodeId: {
            IdType: OpcKeyType.Numeric,
            Id: parseInt(pointKey),
            Namespace: OpcNamespaceMongodb,
          },
          AttributeId: OpcAttributeId.Description,
          Value: { _Properties: props },
        },
      ],
    },
  }
  return invoke(req, 1250)
}

// --- SOE history feed (events viewer) ----------------------------------------

// Normalize one HistoryReadResponse node into an event row.
function normalizeSoeRow(node, cfg, useSourceTime, firstDate) {
  const prop = node._Properties || {}
  const hd = node.HistoryData[0]
  const date = new Date(useSourceTime ? hd.SourceTimestamp : hd.ServerTimestamp)
  const acked = !(hd.Acknowledge === 0 || hd.Acknowledge === false)
  const failed = hd.Value.Quality !== OpcStatusCodes.Good
  const badTime = useSourceTime && hd.SourceTimestampOk !== true
  let countStr = ''
  if (typeof hd.Value.Count === 'number' && hd.Value.Count > 1)
    countStr = '+' + (hd.Value.Count - 1)
  const qualifier =
    '' + prop.priority + (acked ? '' : 'L') + (failed ? 'F' : '') + (badTime ? 'T' : '') + countStr

  let descr = prop.description || ''
  if (descr.indexOf(prop.group1 + '~') === 0) descr = descr.substring(descr.indexOf('~') + 1)

  return {
    rowId: prop.event_id ?? `${prop.pointKey}-${date.getTime()}`,
    dateStr: formatDate(date, cfg.EventsViewer_LocaleDate, cfg.EventsViewer_LocaleDateOptions),
    timeStr: formatTime(date, cfg.EventsViewer_LocaleTime, cfg.EventsViewer_LocaleTimeOptions),
    ms: utcMillis(date),
    pointKey: prop.pointKey,
    tag: node.NodeId.Id,
    station: prop.group1 || '',
    descr,
    event: hd.Value.Body,
    qualifier,
    priority: prop.priority,
    acked,
    failed,
    eventId: prop.event_id,
    timeDiffSec: Math.abs(firstDate - date) / 1000,
    date,
  }
}

// HistoryRead the SOE collection. aggregate: 0=normal, 1=count-aggregated, 2=panic.
export async function readSoe(
  { group1Filter, useSourceTime, aggregate, limit, timeBegin, timeEnd, panicPriorityLimit },
  cfg
) {
  const handle = newHandle()
  const contentFilter = []
  const inList = {
    FilterOperator: OpcFilterOperator.InList,
    FilterOperands: (group1Filter || []).map((g) => ({
      FilterOperand: OpcOperand.Literal,
      Value: g,
    })),
  }
  if (inList.FilterOperands.length > 0) contentFilter.push(inList)
  if (aggregate === 2)
    contentFilter.push({
      FilterOperator: OpcFilterOperator.LessThanOrEqual,
      FilterOperands: [panicPriorityLimit],
    })

  const toIso = (v) =>
    v == null ? null : typeof v.getMonth === 'function' ? v.toISOString() : v

  const req = {
    ServiceId: OpcServiceCode.HistoryReadRequest,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: handle,
        TimeoutHint: 5000,
        ReturnDiagnostics: 2,
        AuthenticationToken: null,
      },
      TimestampsToReturn: useSourceTime ? TimestampsToReturn.Source : TimestampsToReturn.Both,
      HistoryReadDetails: {
        ParameterTypeId: OpcServiceCode.ReadRawModifiedDetails,
        ParameterData: {
          IsModified: false,
          StartTime: toIso(timeBegin),
          EndTime: toIso(timeEnd),
          NumValuesPerNode: limit,
        },
      },
      Namespace: OpcNamespaceMongodb,
      ContentFilter: contentFilter,
      AggregateFilter: aggregate ? { AggregateType: 'Count' } : null,
    },
  }
  const data = await invoke(req, 5000)
  if (
    !isValidResponse(data, handle, [
      OpcServiceCode.HistoryReadResponse,
      OpcServiceCode.ServiceFault,
    ])
  )
    return []
  if (
    data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good &&
    data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.GoodNoData
  ) {
    checkAccessDenied(data)
    return []
  }
  if (typeof data.Body.Results !== 'object' || data.Body.Results.length === 0) return []

  const rows = []
  let firstDate = null
  data.Body.Results.forEach((node) => {
    if (!node.HistoryData || !node.HistoryData[0]) return
    const hd = node.HistoryData[0]
    const date = new Date(useSourceTime ? hd.SourceTimestamp : hd.ServerTimestamp)
    if (firstDate === null) firstDate = date
    rows.push(normalizeSoeRow(node, cfg, useSourceTime, firstDate))
  })
  return rows
}

// --- event acknowledge / removal ---------------------------------------------

// Compose the OpcAcknowledge action bitmask. almquit: 0=ack, 1=remove, -1=silence only.
export function eventAction(almquit, aggregate, pointId) {
  const S = OpcAcknowledge.SilenceBeep
  if (almquit === -1) return S
  if (pointId === 0 || pointId === '0' || pointId == null) {
    return (almquit === 1 ? OpcAcknowledge.RemoveAllEvents : OpcAcknowledge.AckAllEvents) | S
  }
  if (aggregate) {
    return (almquit === 1 ? OpcAcknowledge.RemovePointEvents : OpcAcknowledge.AckPointEvents) | S
  }
  return (almquit === 1 ? OpcAcknowledge.RemoveOneEvent : OpcAcknowledge.AckOneEvent) | S
}

export async function writeEventAck({ pointId, eventId, action }) {
  const handle = newHandle()
  const numeric = !isNaN(parseInt(pointId))
  const req = {
    ServiceId: OpcServiceCode.WriteRequest,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: handle,
        TimeoutHint: 1000,
        ReturnDiagnostics: 2,
        AuthenticationToken: null,
      },
      NodesToWrite: [
        {
          NodeId: {
            IdType: numeric ? OpcKeyType.Numeric : OpcKeyType.String,
            Id: numeric ? parseInt(pointId) : pointId,
            Namespace: OpcNamespaceMongodb,
          },
          AttributeId: OpcAttributeId.ExtendedAlarmEventsAck,
          Value: { Type: OpcValueTypes.Integer, Body: action },
          _Properties: { event_id: eventId || 0 },
        },
      ],
    },
  }
  return invoke(req, 1500)
}

// --- single-tag history (display live plots) ---------------------------------
// Reads raw historical samples for one tag from the historian (Postgresql namespace).
// Returns [{ value, time(ms) }] oldest-first.
export async function readHistory(tag, timeBegin, timeEnd) {
  const handle = newHandle()
  const req = {
    ServiceId: OpcServiceCode.HistoryReadRequest,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: handle,
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
          EndTime: timeEnd.toISOString(),
        },
      },
      NodesToRead: [
        {
          NodeId: {
            IdType: OpcKeyType.String,
            Id: tag,
            Namespace: OpcNamespacePostgresql,
          },
          AttributeId: OpcAttributeId.Value,
        },
      ],
    },
  }
  const data = await invoke(req, 5000)
  if (
    !isValidResponse(data, handle, [
      OpcServiceCode.HistoryReadResponse,
      OpcServiceCode.ServiceFault,
    ])
  )
    return []
  if (data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good) return []
  const r0 = data.Body.Results && data.Body.Results[0]
  if (!r0 || !Array.isArray(r0.HistoryData)) return []
  return r0.HistoryData.map((n) => ({
    value: n.Value.Body,
    time: Date.parse(n.ServerTimestamp),
  }))
}

// --- historical point-in-time snapshot (Time Machine) ------------------------
// Reads the value each given tag had at instant `timeSnap` (StartTime==EndTime),
// from the historian (Postgresql namespace). Returns
// [{ tag, value, type, quality, serverTime(ms) }] for tags that had a sample.
export async function readHistorySnapshot(tags, timeSnap) {
  if (!Array.isArray(tags) || tags.length === 0) return []
  const handle = newHandle()
  const iso = timeSnap.toISOString()
  const req = {
    ServiceId: OpcServiceCode.HistoryReadRequest,
    Body: {
      RequestHeader: {
        Timestamp: new Date().toISOString(),
        RequestHandle: handle,
        TimeoutHint: 8000,
        ReturnDiagnostics: 2,
        AuthenticationToken: null,
      },
      TimestampsToReturn: TimestampsToReturn.Server,
      HistoryReadDetails: {
        ParameterTypeId: OpcServiceCode.ReadRawModifiedDetails,
        ParameterData: { IsModified: false, StartTime: iso, EndTime: iso },
      },
      NodesToRead: tags.map((t) => ({
        NodeId: { IdType: OpcKeyType.String, Id: t, Namespace: OpcNamespacePostgresql },
        AttributeId: OpcAttributeId.Value,
      })),
    },
  }
  const data = await invoke(req, 8000)
  if (
    !isValidResponse(data, handle, [
      OpcServiceCode.HistoryReadResponse,
      OpcServiceCode.ServiceFault,
    ])
  )
    return []
  if (
    data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.Good &&
    data.Body.ResponseHeader.ServiceResult !== OpcStatusCodes.GoodNoData
  ) {
    checkAccessDenied(data)
    return []
  }
  const out = []
  if (Array.isArray(data.Body.Results))
    data.Body.Results.forEach((node) => {
      if (node.StatusCode !== undefined && node.StatusCode !== OpcStatusCodes.Good) return
      const hd = Array.isArray(node.HistoryData) ? node.HistoryData[0] : null
      if (!hd || !hd.Value) return
      out.push({
        tag: node.NodeId.Id,
        value: hd.Value.Body,
        type: hd.Value.Type,
        quality: hd.Value.Quality,
        serverTime: Date.parse(hd.ServerTimestamp),
      })
    })
  return out
}

export { BEEP_POINTKEY }
