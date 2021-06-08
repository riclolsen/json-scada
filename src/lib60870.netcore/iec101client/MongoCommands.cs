/* 
 * IEC 60870-5-101 Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020 - Ricardo L. Olsen
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
using lib60870.CS101;
using System.Threading;

namespace Iec10XDriver
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
                    var Client = ConnectMongoClient(JSConfig);
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
                                    // consider only commands for this driver
                                    {
                                        Log("MongoDB CMD CS - Looking for connection " +
                                        change
                                            .FullDocument
                                            .protocolSourceConnectionNumber +
                                        "...");
                                        var found = false;
                                        foreach (IEC10X_connection
                                            srv
                                            in
                                            IEC10Xconns
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
                                                if (
                                                    // srv.Master.IsRunning &&
                                                    srv.commandsEnabled
                                                )
                                                {
                                                    InformationObject sc =
                                                        BuildInfoObj(System
                                                            .Convert
                                                            .ToInt32(change
                                                                .FullDocument
                                                                .protocolSourceASDU),
                                                        System
                                                            .Convert
                                                            .ToInt32(change
                                                                .FullDocument
                                                                .protocolSourceObjectAddress),
                                                        System
                                                            .Convert
                                                            .ToDouble(change
                                                                .FullDocument
                                                                .value),
                                                        System
                                                            .Convert
                                                            .ToBoolean(change
                                                                .FullDocument
                                                                .protocolSourceCommandUseSBO),
                                                        System
                                                            .Convert
                                                            .ToByte(change
                                                                .FullDocument
                                                                .protocolSourceCommandDuration));
                                                    if (sc != null)
                                                    {
                                                        if (
                                                            DateTime
                                                                .Now
                                                                .ToLocalTime()
                                                                .Subtract(change
                                                                    .FullDocument
                                                                    .timeTag
                                                                    .ToLocalTime(
                                                                    ))
                                                                .Seconds <
                                                            10
                                                        )
                                                        {
                                                            // execute
                                                            srv.master.
                                                            SendControlCommand(CauseOfTransmission
                                                                    .ACTIVATION,
                                                                System
                                                                    .Convert
                                                                    .ToInt32(change
                                                                        .FullDocument
                                                                        .protocolSourceCommonAddress),
                                                                sc);
                                                            Log("MongoDB CMD CS - " +
                                                            srv.name +
                                                            " - " +
                                                            sc.ToString() +
                                                            " OA " +
                                                            change
                                                                .FullDocument
                                                                .protocolSourceObjectAddress +
                                                            " Delivered");

                                                            // update as delivered
                                                            var filter =
                                                                new BsonDocument(new BsonDocument("_id",
                                                                        change
                                                                            .FullDocument
                                                                            .id));
                                                            var update =
                                                                new BsonDocument("$set",
                                                                    new BsonDocument("delivered",
                                                                        true));
                                                            var result =
                                                                await collection
                                                                    .UpdateOneAsync(filter,
                                                                    update);
                                                        }
                                                        else
                                                        {
                                                            // update as expired
                                                            Log("MongoDB CMD CS - " +
                                                            srv.name +
                                                            " - " +
                                                            sc.ToString() +
                                                            " OA " +
                                                            change
                                                                .FullDocument
                                                                .protocolSourceObjectAddress +
                                                            " value " +
                                                            change
                                                                .FullDocument
                                                                .value +
                                                            " Expired");
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
                                                        }
                                                    }
                                                    else
                                                    {
                                                        // update as canceled (asdu not implemented)
                                                        Log("MongoDB CMD CS - " +
                                                        srv.name +
                                                        " - " +
                                                        " OA " +
                                                        change
                                                            .FullDocument
                                                            .protocolSourceObjectAddress +
                                                        " value " +
                                                        change
                                                            .FullDocument
                                                            .value +
                                                        " ASDU Not Implemented");
                                                        var filter =
                                                            new BsonDocument(new BsonDocument("_id",
                                                                    change
                                                                        .FullDocument
                                                                        .id));
                                                        var update =
                                                            new BsonDocument("$set",
                                                                new BsonDocument("cancelReason",
                                                                    "asdu not implemented"));
                                                        var result =
                                                            await collection
                                                                .UpdateOneAsync(filter,
                                                                update);
                                                    }
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
                                            // update as canceled command (not found)
                                            Log("MongoDB CMD CS - " +
                                            change
                                                .FullDocument
                                                .protocolSourceConnectionNumber
                                                .ToString() +
                                            " OA " +
                                            change
                                                .FullDocument
                                                .protocolSourceObjectAddress +
                                            " value " +
                                            change.FullDocument.value +
                                            " Connection Not Found");
                                        }
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