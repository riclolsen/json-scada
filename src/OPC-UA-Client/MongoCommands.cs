/* 
 * OPC-UA Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020-2022 - Ricardo L. Olsen
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
using Opc.Ua;

namespace OPCUAClientDriver
{
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
                    var collection =
                        DB
                            .GetCollection
                            <rtCommand>(CommandsQueueCollectionName);

                    bool isMongoLive =
                        DB
                            .RunCommandAsync((Command<BsonDocument>)"{ping:1}")
                            .Wait(1000);
                    if (!isMongoLive)
                        throw new Exception("Error on connection " + JSConfig.mongoConnectionString);

                    Log("MongoDB CMD CS - Start listening for commands via changestream...");
                    var filter = "{ operationType: 'insert' }";

                    var pipeline =
                        new EmptyPipelineDefinition<ChangeStreamDocument<rtCommand
                            >
                        >().Match(filter);
                    using (var cursor = await collection.WatchAsync(pipeline))
                    {
                        await cursor
                            .ForEachAsync(async change =>
                            {
                                if (!Active)
                                    return;

                                // process change event, only process inserts
                                if (
                                    change.OperationType ==
                                    ChangeStreamOperationType.Insert
                                )
                                {
                                    Log("MongoDB CMD CS - Looking for connection " +
                                    change
                                        .FullDocument
                                        .protocolSourceConnectionNumber +
                                    "...");
                                    var found = false;
                                    foreach (OPCUA_connection
                                        srv
                                        in
                                        OPCUAconns
                                    )
                                    {
                                        if (
                                            srv.protocolConnectionNumber ==
                                            change
                                                .FullDocument
                                                .protocolSourceConnectionNumber
                                        )
                                        {
                                            found = true;

                                            int timeDif = DateTime
                                                    .Now
                                                    .ToLocalTime()
                                                    .Subtract(change
                                                        .FullDocument
                                                        .timeTag
                                                        .ToLocalTime(
                                                        ))
                                                    .Seconds;

                                            // test for command expired
                                            if (timeDif > 10)
                                            {
                                                // update as expired
                                                Log("MongoDB CMD CS - " +
                                                srv.name +
                                                " - " +
                                                " Address " +
                                                change
                                                    .FullDocument
                                                    .protocolSourceObjectAddress +
                                                " value " +
                                                change
                                                    .FullDocument
                                                    .value +
                                                " Expired, " + timeDif + " Seconds old");
                                                var filter =
                                                    new BsonDocument(new BsonDocument("_id",
                                                            change
                                                                .FullDocument
                                                                .id));
                                                var update =
                                                    new BsonDocument("$set",
                                                        new BsonDocument("cancelReason",
                                                            "expired"));
                                                var result =
                                                    await collection
                                                        .UpdateOneAsync(filter,
                                                        update);

                                                break;
                                            }

                                            if (
                                            srv.connection.session.Connected &&
                                            srv.commandsEnabled
                                            )
                                            {
                                                /*
                                                // when method, call
                                                VariantCollection inputArguments = new VariantCollection();
                                                CallMethodRequest request = new CallMethodRequest();
                                                request.ObjectId = new NodeId(System
                                                        .Convert
                                                        .ToString(change.FullDocument.protocolSourceObjectAddress));
                                                // request.MethodId = m_methodId;
                                                request.InputArguments = inputArguments;
                                                CallMethodRequestCollection requests = new CallMethodRequestCollection();
                                                requests.Add(request);
                                                CallMethodResultCollection results;
                                                DiagnosticInfoCollection diagnosticInfos;
                                                ResponseHeader responseHeader = srv.connection.session.Call(
                                                    null,
                                                    requests,
                                                    out results,
                                                    out diagnosticInfos);
                                                */

                                                // when variable, write
                                                WriteValueCollection nodesToWrite = new WriteValueCollection();
                                                WriteValue WriteVal = new WriteValue();
                                                WriteVal.NodeId =
                                                        new NodeId(System
                                                        .Convert
                                                        .ToString(change.FullDocument.protocolSourceObjectAddress));
                                                WriteVal.AttributeId = Attributes.Value;
                                                WriteVal.Value = new DataValue();

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
                                                        WriteVal.Value.Value = Convert.ToDateTime(change.FullDocument.value);
                                                        break;
                                                    case "string":
                                                        WriteVal.Value.Value = Convert.ToString(change.FullDocument.value);
                                                        break;
                                                }

                                                nodesToWrite.Add(WriteVal);

                                                // Write the node attributes
                                                StatusCodeCollection results = null;
                                                DiagnosticInfoCollection diagnosticInfos;

                                                Log("MongoDB CMD CS - " + srv.name + " - Writing node...");

                                                // Call Write Service
                                                srv.connection.session.Write(null,
                                                                nodesToWrite,
                                                                out results,
                                                                out diagnosticInfos);

                                                var okres = false;
                                                var resultDescription = "";
                                                if (results.Count > 0)
                                                {
                                                    resultDescription = results[0].ToString();
                                                    if (StatusCode.IsGood(results[0]))
                                                        okres = true;
                                                }

                                                Log("MongoDB CMD CS - " +
                                                srv.name +
                                                " - " +
                                                " Address: " +
                                                change
                                                    .FullDocument
                                                    .protocolSourceObjectAddress +
                                                " - Command delivered - " + results[0].ToString());

                                                // update as delivered
                                                var filter =
                                                    new BsonDocument(new BsonDocument("_id",
                                                            change
                                                                .FullDocument
                                                                .id));
                                                var update =
                                                    new BsonDocument
                                                        { {"$set",
                                                                    new BsonDocument{
                                                                        { "delivered", true },
                                                                        { "ack", okres },
                                                                        { "ackTimeTag", new BsonDateTime(DateTime.Now) },
                                                                        { "resultDescription", resultDescription }
                                                                    }
                                                                } };
                                                var result =
                                                    await collection
                                                        .UpdateOneAsync(filter,
                                                        update);
                                            }
                                            else
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
                                                var filter =
                                                    new BsonDocument(new BsonDocument("_id",
                                                            change
                                                                .FullDocument
                                                                .id));
                                                var update =
                                                    new BsonDocument("$set",
                                                        new BsonDocument("cancelReason",
                                                            (
                                                            srv
                                                                .commandsEnabled
                                                                ? "not connected"
                                                                : "commands disabled"
                                                            )));
                                                var result =
                                                    await collection
                                                        .UpdateOneAsync(filter,
                                                        update);
                                            }
                                            break;
                                        }
                                    }
                                    if (!found)
                                    {
                                        // not for a connection managed by this driver instance, just ignore
                                    }
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
}