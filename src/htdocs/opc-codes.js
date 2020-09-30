// OPC-UA Standard Codes (and some extensions).
// {json:scada} - Copyright 2020 - Ricardo L. Olsen

var OpcNamespaceMongodb = 2; // first user namespace
var OpcNamespacePostgresql = 3; // second user namespace
var OpcAcknowledge = {
  AckOneEvent       : 0x00000001,
  RemoveOneEvent    : 0x00000002,
  AckPointEvents    : 0x00000004,
  RemovePointEvents : 0x00000008,
  AckAllEvents      : 0x00000040,
  RemoveAllEvents   : 0x00000080,
  AckOneAlarm       : 0x00000100,
  AckAllAlarms      : 0x00000400,
  SilenceBeep       : 0x00001000
}

var OpcStatusCodes = {
  Good: 0,
  GoodNoData: 0x00A50000,
  GoodMoreData: 0x00A60000,
  Uncertain: 0x40000000,
  Bad: 0x80000000,
  BadTimeout: 0x800A0000,
  BadNodeAttributesInvalid: 0x80620000,
  BadNodeIdInvalid: 0x80330000,
  BadNodeIdUnknown: 0x80340000,
  BadRequestHeaderInvalid: 0x802A0000,
  BadRequestNotAllowed: 0x80E40000,
  BadServiceUnsupported: 0x800B0000,
  BadShutdown: 0x800C0000,
  BadServerNotConnected: 0x800D0000,
  BadServerHalted: 0x800E0000,
  BadNothingToDo: 0x800F0000,
  BadUserAccessDenied: 0x801F0000,
  BadIdentityTokenInvalid: 0x80200000,
  BadIdentityTokenRejected: 0x80210000,
  BadUnexpectedError: 0x80010000,
  BadInternalError: 0x80020000,
  BadOutOfMemory: 0x80030000,
  BadResourceUnavailable: 0x80040000,
  BadCommunicationError: 0x80050000,
  BadInvalidArgument: 0x80AB0000,
  BadDisconnect: 0x80AD0000,
  BadConnectionClosed: 0x80AE0000,
  BadInvalidState: 0x80AF0000,
  BadNoDataAvailable: 0x80B10000,
  BadWaitingForResponse: 0x80B20000,
};
var TimestampsToReturn = {
  Source: 0,
  Server: 1,
  Both: 2,
  Neither: 3,
  Invalid: 4
};
var OpcValueTypes = {
  Null : 0,
  Boolean : 1,
  SByte : 2,
  Byte : 3,
  Int16 : 4,
  UInt16 : 5,
  Int32 : 6,
  UInt32 : 7,
  Int64 : 8,
  UInt64 : 9,
  Float : 10,
  Double : 11,
  String : 12,
  DateTime : 13,
  Guid : 14,
  ByteString : 15,
  XmlElement : 16,
  NodeId : 17,
  ExpandedNodeId : 18,
  StatusCode : 19,
  QualifiedName : 20,
  LocalizedText : 21,
  ExtensionObject : 22,
  DataValue : 23,
  Variant : 24,
  DiagnosticInfo : 25,
  Number : 26,
  Integer : 27,
  UInteger : 28,
  Enumeration : 29
};
var OpcKeyType = {
  Numeric: 0,
  String: 1
};
var OpcServiceCode = {
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
  Extended_ResponseUniqueAttributeValues: 100000002
};

let OpcOperand = {
  Attribute: 598,
  Literal: 595,
  Element: 592,
  SimpleAttributeOperand: 601
};

var OpcAttributeId = { 
  NodeID                       : 1,
  NodeClass                    : 2,
  BrowseName                   : 3,
  DisplayName                  : 4,
  Description                  : 5,
  WriteMask                    : 6,
  UserWriteMask                : 7,
  IsAbstract                   : 8,
  Symmetric                    : 9,
  InverseName                  : 10,
  ContainsNoLoops              : 11,
  EventNotifier                : 12,
  Value                        : 13,
  DataType                     : 14,
  ValueRank                    : 15,
  ArrayDimensions              : 16,
  AccessLevel                  : 17,
  UserAccessLevel              : 18,
  MinimumSamplingInterval      : 19,
  Historizing                  : 20,
  Executable                   : 21,
  UserExecutable               : 22,
  ExtendedGroup1               : 100000001,
  ExtendedGroup2               : 100000002,
  ExtendedGroup3               : 100000003,
  ExtendedAlarmEventsAck       : 100000004,
  ExtendedBlockingAnnotation   : 100000005,
  ExtendedDocumentalAnnotation : 100000006
};

var OpcFilterOperator = {
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
  BitwiseOr: 17
};
