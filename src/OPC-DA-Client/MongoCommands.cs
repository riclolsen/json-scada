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
using Technosoftware.DaAeHdaClient;
using Technosoftware.DaAeHdaClient.Da;
using static MongoDB.Libmongocrypt.CryptContext;

namespace OPCDAClientDriver
{
    partial class MainClass
    {
        // This process watches (via change stream) for commands inserted to a commands collection
        // When the command is considered valid it is forwarded to the RTU
        static async void ProcessMongoCmd(JSONSCADAConfig jsConfig)
        {
            do
            {
                try
                {
                    var Client = ConnectMongoClient(jsConfig);
                    var DB = Client.GetDatabase(jsConfig.mongoDatabaseName);
                    var collection =
                        DB
                            .GetCollection
                            <rtCommand>(CommandsQueueCollectionName);

                    bool isMongoLive =
                        DB
                            .RunCommandAsync((Command<BsonDocument>)"{ping:1}")
                            .Wait(1000);
                    if (!isMongoLive)
                        throw new Exception("Error on connection " + jsConfig.mongoConnectionString);

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
                                    foreach (OPCDA_connection
                                        srv
                                        in
                                        OPCDAconns
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
                                            srv.connection.IsConnected &&
                                            srv.commandsEnabled
                                            )
                                            {
                                                var item = new OpcItem(change.FullDocument.protocolSourceObjectAddress.ToString());
                                                var daItem = new TsCDaItem(item);
                                                var itVal = new TsCDaItemValueResult(item);
                                                switch (change.FullDocument.protocolSourceASDU.ToString().ToLower())
                                                {
                                                    case "vt_bool":
                                                    case "bool":
                                                    case "boolean":
                                                        itVal.Value = Convert.ToBoolean(Convert.ToDouble(change.FullDocument.value) != 0.0);
                                                        break;
                                                    case "vt_i1":
                                                    case "i1":
                                                    case "int1":
                                                    case "sbyte":
                                                        itVal.Value = Convert.ToSByte(change.FullDocument.value);
                                                        break;
                                                    case "vt_ui1":
                                                    case "ui1":
                                                    case "uint1":
                                                    case "byte":
                                                        itVal.Value = Convert.ToByte(change.FullDocument.value);
                                                        break;
                                                    case "vt_i2":
                                                    case "i2":
                                                    case "int2":
                                                    case "int16":
                                                        itVal.Value = Convert.ToInt16(change.FullDocument.value);
                                                        break;
                                                    case "vt_ui2":
                                                    case "ui2":
                                                    case "uint2":
                                                    case "uint16":
                                                        itVal.Value = Convert.ToUInt16(change.FullDocument.value);
                                                        break;
                                                    case "vt_i4":
                                                    case "i4":
                                                    case "int4":
                                                    case "int32":
                                                        itVal.Value = Convert.ToInt32(change.FullDocument.value);
                                                        break;
                                                    case "vt_ui4":
                                                    case "ui4":
                                                    case "uint4":
                                                    case "uint32":
                                                        itVal.Value = Convert.ToUInt32(change.FullDocument.value);
                                                        break;
                                                    case "vt_i8":
                                                    case "i8":
                                                    case "int8":
                                                    case "int64":
                                                        itVal.Value = Convert.ToInt64(change.FullDocument.value);
                                                        break;
                                                    case "vt_ui8":
                                                    case "ui8":
                                                    case "uint8":
                                                    case "uint64":
                                                        itVal.Value = Convert.ToUInt64(change.FullDocument.value);
                                                        break;
                                                    case "vt_r4":
                                                    case "r4":
                                                    case "single":
                                                    case "float":
                                                        itVal.Value = Convert.ToSingle(change.FullDocument.value);
                                                        break;
                                                    case "vt_r8":
                                                    case "r8":
                                                    case "double":
                                                        itVal.Value = Convert.ToDouble(change.FullDocument.value);
                                                        break;
                                                    case "vt_cy":
                                                    case "cy":
                                                    case "currency":
                                                    case "money":
                                                    case "decimal":
                                                        itVal.Value = Convert.ToDecimal(change.FullDocument.value);
                                                        break;
                                                    case "vt_date":
                                                    case "date":
                                                    case "time":
                                                    case "datetime":
                                                        itVal.Value = Convert.ToDateTime(change.FullDocument.value);
                                                        break;
                                                    case "vt_bstr":
                                                    case "bstr":
                                                    case "string":
                                                        itVal.Value = Convert.ToString(change.FullDocument.valueString);
                                                        break;
                                                }
                                                itVal.Quality = TsCDaQuality.Good;
                                                itVal.Timestamp = Convert.ToDateTime(change.FullDocument.timeTag);
                                                itVal.TimestampSpecified = true;
                                                Log($"MongoDB CMD CS - {srv.name} - Writing node: {itVal.ItemName} value: {itVal.Value}"  );
                                                var results = srv.connection.Write(new TsCDaItemValue[] { itVal });
                                                var okres = false;
                                                var resultDescription = "Error!";
                                                if (results != null && results.Length > 0)
                                                {
                                                    resultDescription = results[0].Result.Description();
                                                    if (results[0].Result.IsOk())
                                                        okres = true;
                                                }

                                                Log("MongoDB CMD CS - " +
                                                srv.name +
                                                " - " +
                                                " Address: " +
                                                change
                                                    .FullDocument
                                                    .protocolSourceObjectAddress +
                                                " - Command delivered - " + resultDescription);

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