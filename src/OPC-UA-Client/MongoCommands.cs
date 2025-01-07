﻿/* 
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
using MongoDB.Driver;
using System.Threading;
using System.Text.Json.Nodes;
using Opc.Ua;

partial class MainClass
{
    // This process watches (via change stream) for commands inserted to a commands collection
    // When the command is considered valid it is forwarded to the RTU
    static async void ProcessMongoCmd()
    {
        do
        {
            try
            {
                var Client = ConnectMongoClient(JSConfig);
                var DB = Client.GetDatabase(JSConfig.mongoDatabaseName);
                var collection = DB.GetCollection<rtCommand>(CommandsQueueCollectionName);

                BsonDocument filter, update;

                bool isMongoLive = DB.RunCommandAsync((Command<BsonDocument>)"{ping:1}").Wait(1000);
                if (!isMongoLive) throw new Exception("Error on connection " + JSConfig.mongoConnectionString);

                Log("MongoDB CMD CS - Start listening for commands via changestream...");

                var pipeline = new EmptyPipelineDefinition<ChangeStreamDocument<rtCommand>>().Match("{ operationType: 'insert' }");

                using (var cursor = await collection.WatchAsync(pipeline))
                {
                    await cursor.ForEachAsync(async change =>
                        {
                            if (!Active) return;

                            // process change event, only process inserts
                            if (change.OperationType != ChangeStreamOperationType.Insert) return;

                            Log("MongoDB CMD CS - Looking for connection " +
                                change.FullDocument.protocolSourceConnectionNumber +
                                "...", LogLevelDetailed);

                            foreach (OPCUA_connection srv in OPCUAconns)
                            {
                                if (srv.protocolConnectionNumber != change.FullDocument.protocolSourceConnectionNumber)
                                    continue;

                                // test for command expired
                                int timeDif = DateTime.Now.ToLocalTime()
                                        .Subtract(change.FullDocument.timeTag.ToLocalTime()).Seconds;
                                if (timeDif > 10)
                                {
                                    // update as expired
                                    Log("MongoDB CMD CS - " + srv.name + " - Address " +
                                    change.FullDocument.protocolSourceObjectAddress +
                                    " value " + change.FullDocument.value +
                                    " Command Timeout Expired, " + timeDif + " Seconds old");
                                    filter = new BsonDocument(new BsonDocument("_id", change.FullDocument.id));
                                    update = new BsonDocument("$set", new BsonDocument("cancelReason", "expired"));
                                    await collection.UpdateOneAsync(filter, update);
                                    break;
                                }

                                if (!srv.connection.session.Connected || !srv.commandsEnabled)
                                {
                                    // update as canceled (not connected)
                                    Log("MongoDB CMD CS - " +
                                    srv.name +
                                    " OA " +
                                    change
                                        .FullDocument
                                        .protocolSourceObjectAddress +
                                    " value " +
                                    change.FullDocument.value +
                                    (
                                    srv.commandsEnabled
                                        ? " Not Connected"
                                        : " Commands Disabled"
                                    ));
                                    filter = new BsonDocument(new BsonDocument("_id", change.FullDocument.id));
                                    update = new BsonDocument("$set", new BsonDocument("cancelReason",
                                                (
                                                srv
                                                    .commandsEnabled
                                                    ? "not connected"
                                                    : "commands disabled"
                                                )));
                                    await collection.UpdateOneAsync(filter, update);
                                }

                                // when variable, write
                                WriteValueCollection nodesToWrite = new WriteValueCollection();
                                WriteValue WriteVal = new WriteValue();
                                WriteVal.NodeId =
                                        new NodeId(System
                                        .Convert
                                        .ToString(change.FullDocument.protocolSourceObjectAddress));
                                WriteVal.AttributeId = Attributes.Value;
                                WriteVal.Value = new DataValue
                                {
                                    StatusCode = StatusCodes.Good,
                                    SourceTimestamp = DateTime.UtcNow,
                                    ServerTimestamp = DateTime.UtcNow
                                };

                                switch (change.FullDocument.protocolSourceASDU.ToString().ToLower())
                                {
                                    case "boolean":
                                        WriteVal.Value.Value = Convert.ToBoolean(Convert.ToDouble(change.FullDocument.value) != 0.0);
                                        break;
                                    case "sbyte":
                                        WriteVal.Value.Value = Convert.ToSByte(change.FullDocument.value);
                                        break;
                                    case "byte":
                                        WriteVal.Value.Value = Convert.ToByte(change.FullDocument.value);
                                        break;
                                    case "int16":
                                        WriteVal.Value.Value = Convert.ToInt16(change.FullDocument.value);
                                        break;
                                    case "uint16":
                                        WriteVal.Value.Value = Convert.ToUInt16(change.FullDocument.value);
                                        break;
                                    case "integer":
                                    case "int32":
                                        WriteVal.Value.Value = Convert.ToInt32(change.FullDocument.value);
                                        break;
                                    case "uint32":
                                        WriteVal.Value.Value = Convert.ToUInt32(change.FullDocument.value);
                                        break;
                                    case "int64":
                                        WriteVal.Value.Value = Convert.ToInt64(change.FullDocument.value);
                                        break;
                                    case "uint64":
                                        WriteVal.Value.Value = Convert.ToUInt64(change.FullDocument.value);
                                        break;
                                    case "float":
                                        WriteVal.Value.Value = Convert.ToSingle(change.FullDocument.value);
                                        break;
                                    case "double":
                                        WriteVal.Value.Value = Convert.ToDouble(change.FullDocument.value);
                                        break;
                                    case "datetime":
                                        WriteVal.Value.Value = new DateTime((long)change.FullDocument.value.AsDouble);
                                        break;
                                    case "string":
                                    case "bytestring":
                                    case "localizedtext":
                                    case "qualifiedname":
                                    case "nodeid":
                                    case "guid":
                                    case "expandednodeid":
                                    case "xmlelement":
                                        WriteVal.Value.Value = Convert.ToString(change.FullDocument.valueString);
                                        break;
                                    case "extensionobject":
                                    case "numericrange":
                                    case "variant":
                                    case "diagnosticinfo":
                                    case "datavalue":
                                        WriteVal.Value.Value = JsonNode.Parse(change.FullDocument.valueString.ToString());
                                        break;
                                }

                                // test for array type
                                if (change.FullDocument.protocolSourceASDU.ToString().Split('[').Length - 1 == 1)
                                {
                                    try
                                    {
                                        var jsonString = change.FullDocument.valueString.ToString();
                                        if (string.IsNullOrEmpty(jsonString))
                                        {
                                            Log("MongoDB CMD CS - " + srv.name + " - Empty array value string");
                                            throw new ArgumentException("Empty array value string");
                                        }

                                        var jsonNode = JsonNode.Parse(jsonString);
                                        if (jsonNode is JsonArray jsonArray)
                                        {
                                            // Get array type from ASDU string (e.g. "Float[3]" -> "Float")
                                            var arrayType = change.FullDocument.protocolSourceASDU.ToString()
                                                .Split('[')[0].Trim().ToLower();

                                            // Convert array elements based on type
                                            switch (arrayType)
                                            {
                                                case "datetime":
                                                    var datetimeArray = new DateTime[jsonArray.Count];
                                                    for (int i = 0; i < jsonArray.Count; i++)
                                                        datetimeArray[i] = jsonArray[i].GetValue<DateTime>();
                                                    WriteVal.Value.Value = new Variant(datetimeArray);
                                                    break;
                                                case "int16":
                                                    var int16Array = new short[jsonArray.Count];
                                                    for (int i = 0; i < jsonArray.Count; i++)
                                                        int16Array[i] = jsonArray[i].GetValue<short>();
                                                    WriteVal.Value.Value = new Variant(int16Array);
                                                    break;
                                                case "uint16":
                                                    var uint16Array = new ushort[jsonArray.Count];
                                                    for (int i = 0; i < jsonArray.Count; i++)
                                                        uint16Array[i] = jsonArray[i].GetValue<ushort>();
                                                    WriteVal.Value.Value = new Variant(uint16Array);
                                                    break;
                                                case "uint32":
                                                    var uint32Array = new uint[jsonArray.Count];
                                                    for (int i = 0; i < jsonArray.Count; i++)
                                                        uint32Array[i] = jsonArray[i].GetValue<uint>();
                                                    WriteVal.Value.Value = new Variant(uint32Array);
                                                    break;
                                                case "int64":
                                                    var int64Array = new long[jsonArray.Count];
                                                    for (int i = 0; i < jsonArray.Count; i++)
                                                        int64Array[i] = jsonArray[i].GetValue<long>();
                                                    WriteVal.Value.Value = new Variant(int64Array);
                                                    break;
                                                case "uint64":
                                                    var uint64Array = new ulong[jsonArray.Count];
                                                    for (int i = 0; i < jsonArray.Count; i++)
                                                        uint64Array[i] = jsonArray[i].GetValue<ulong>();
                                                    WriteVal.Value.Value = new Variant(uint64Array);
                                                    break;
                                                case "float":
                                                    var floatArray = new float[jsonArray.Count];
                                                    for (int i = 0; i < jsonArray.Count; i++)
                                                        floatArray[i] = jsonArray[i].GetValue<float>();
                                                    WriteVal.Value.Value = new Variant(floatArray);
                                                    break;
                                                case "double":
                                                    var doubleArray = new double[jsonArray.Count];
                                                    for (int i = 0; i < jsonArray.Count; i++)
                                                        doubleArray[i] = jsonArray[i].GetValue<double>();
                                                    WriteVal.Value.Value = new Variant(doubleArray);
                                                    break;
                                                case "int32":
                                                case "integer":
                                                    var intArray = new int[jsonArray.Count];
                                                    for (int i = 0; i < jsonArray.Count; i++)
                                                        intArray[i] = jsonArray[i].GetValue<int>();
                                                    WriteVal.Value.Value = new Variant(intArray);
                                                    break;
                                                case "boolean":
                                                    var boolArray = new bool[jsonArray.Count];
                                                    for (int i = 0; i < jsonArray.Count; i++)
                                                        boolArray[i] = jsonArray[i].GetValue<bool>();
                                                    WriteVal.Value.Value = new Variant(boolArray);
                                                    break;
                                                case "string":
                                                case "bytestring":
                                                case "localizedtext":
                                                case "qualifiedname":
                                                case "nodeid":
                                                case "guid":
                                                case "expandednodeid":
                                                case "xmlelement":
                                                    var strArray = new string[jsonArray.Count];
                                                    for (int i = 0; i < jsonArray.Count; i++)
                                                        strArray[i] = jsonArray[i].GetValue<string>();
                                                    WriteVal.Value.Value = new Variant(strArray);
                                                    break;
                                                default:
                                                    Log("MongoDB CMD CS - " + srv.name + " - Unsupported array type: " + arrayType);
                                                    throw new ArgumentException($"Unsupported array type: {arrayType}");
                                            }
                                        }
                                        else
                                        {
                                            Log("MongoDB CMD CS - " + srv.name + " - Invalid array JSON format");
                                            throw new ArgumentException("Invalid array JSON format");
                                        }
                                    }
                                    catch (Exception ex)
                                    {
                                        Log("MongoDB CMD CS - " + srv.name + " - Array conversion error: " + ex.Message);
                                        throw;
                                    }
                                }

                                nodesToWrite.Add(WriteVal);

                                // Write the node attributes
                                StatusCodeCollection results = null;
                                DiagnosticInfoCollection diagnosticInfos;

                                Log("MongoDB CMD CS - " + srv.name + " - Writing node...");

                                // Call Write Service
                                try
                                {
                                    // Set proper request header with timeout
                                    var requestHeader = new RequestHeader
                                    {
                                        Timestamp = DateTime.UtcNow,
                                        TimeoutHint = 10000  // 10 second timeout
                                    };

                                    // Validate node before writing
                                    var nodesToRead = new ReadValueIdCollection();
                                    nodesToRead.Add(new ReadValueId()
                                    {
                                        NodeId = WriteVal.NodeId,
                                        AttributeId = Attributes.Value
                                    });

                                    DataValueCollection readResults;
                                    DiagnosticInfoCollection readDiagnostics;
                                    srv.connection.session.Read(
                                        null,
                                        0,
                                        TimestampsToReturn.Neither,
                                        nodesToRead,
                                        out readResults,
                                        out readDiagnostics
                                    );

                                    if (readResults[0].StatusCode.Code != StatusCodes.Good)
                                    {
                                        Log("MongoDB CMD CS - " + srv.name + " - Node validation failed: " + readResults[0].StatusCode);
                                        throw new ServiceResultException(readResults[0].StatusCode);
                                    }

                                    // Perform write operation
                                    srv.connection.session.Write(
                                        requestHeader,
                                        nodesToWrite,
                                        out results,
                                        out diagnosticInfos);

                                    // Log diagnostic information if available
                                    if (diagnosticInfos != null && diagnosticInfos.Count > 0)
                                    {
                                        foreach (var diagnostic in diagnosticInfos)
                                        {
                                            if (diagnostic != null)
                                            {
                                                Log("MongoDB CMD CS - " + srv.name + " - Write diagnostic: " + diagnostic.ToString());
                                            }
                                        }
                                    }
                                }
                                catch (ServiceResultException sre)
                                {
                                    Log("MongoDB CMD CS - " + srv.name + " - Write error: " + sre.Message);
                                    Log("MongoDB CMD CS - " + srv.name + " - Status code: " + sre.StatusCode);
                                    if (sre.InnerException != null)
                                    {
                                        Log("MongoDB CMD CS - " + srv.name + " - Inner error: " + sre.InnerException.Message);
                                    }
                                    throw;
                                }
                                catch (Exception ex)
                                {
                                    Log("MongoDB CMD CS - " + srv.name + " - Unexpected error: " + ex.Message);
                                    if (ex.InnerException != null)
                                    {
                                        Log("MongoDB CMD CS - " + srv.name + " - Inner error: " + ex.InnerException.Message);
                                    }
                                    throw;
                                }

                                var okres = false;
                                var resultDescription = "";
                                if (results.Count > 0)
                                {
                                    resultDescription = results[0].ToString();
                                    if (StatusCode.IsGood(results[0])) okres = true;
                                }

                                Log("MongoDB CMD CS - " + srv.name + " - Address: " +
                                    change.FullDocument.protocolSourceObjectAddress +
                                    " value " + change.FullDocument.value +
                                    " - Command delivered - " + results[0].ToString());

                                // update as delivered
                                filter = new BsonDocument(new BsonDocument("_id", change.FullDocument.id));
                                update = new BsonDocument{ {"$set",
                                                            new BsonDocument{
                                                                { "delivered", true },
                                                                { "ack", okres },
                                                                { "ackTimeTag", new BsonDateTime(DateTime.Now) },
                                                                { "resultDescription", resultDescription }
                                                            }
                                                        } };
                                await collection.UpdateOneAsync(filter,update);
                                break;
                            }
                        });
                }
            }
            catch (Exception e)
            {
                Log("Exception MongoCmd");
                Log(e);
                Log(e
                    .ToString()
                    .Substring(0,
                    e.ToString().IndexOf(Environment.NewLine)));
                Thread.Sleep(3000);
            }
        }
        while (true);
    }
}
