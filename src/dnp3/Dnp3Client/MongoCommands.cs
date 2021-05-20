/* 
 * DNP3 Client Protocol driver for {json:scada}
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
using System.Linq;
using Automatak.DNP3.Interface;
using MongoDB.Bson;
using MongoDB.Driver;

namespace Dnp3Driver
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
                                    // consider only commands for this driver
                                    {
                                        Log("MongoDB CMD CS - Looking for connection " +
                                        change
                                            .FullDocument
                                            .protocolSourceConnectionNumber +
                                        "...");
                                        var found = false;
                                        foreach (DNP3_connection
                                            srv
                                            in
                                            DNP3conns
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
                                                var cs = srv.channel.GetChannelStatistics();
                                                if (
                                                        srv.isConnected &&
                                                        srv.commandsEnabled
                                                   )
                                                    {
                                                    var group = change.FullDocument.protocolSourceCommonAddress;
                                                    var variation = change.FullDocument.protocolSourceASDU;
                                                    if (group == 41 || group == 12)
                                                        {                                                            
                                                            if ( // check for command expired
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
                                                            { // can execute
                                                            System.Threading.Tasks.Task<CommandTaskResult> cmdTask = null;

                                                            if (group == 12)
                                                            {
                                                                OperationType ot = OperationType.NUL;
                                                                   
                                                                TripCloseCode tc = TripCloseCode.NUL;
                                                                switch (System.Convert.ToUInt16(change.FullDocument.protocolSourceCommandDuration))
                                                                {
                                                                    default:
                                                                    case 0:
                                                                        ot = OperationType.NUL;
                                                                        break;
                                                                    case 1:
                                                                        if (change.FullDocument.value != 0)
                                                                            ot = OperationType.PULSE_ON;
                                                                        else
                                                                            ot = OperationType.PULSE_OFF;
                                                                        break;
                                                                    case 2:
                                                                        if (change.FullDocument.value != 0)
                                                                            ot = OperationType.PULSE_OFF;
                                                                        else
                                                                            ot = OperationType.PULSE_ON;
                                                                        break;
                                                                    case 3:
                                                                        if (change.FullDocument.value != 0)
                                                                            ot = OperationType.LATCH_ON;
                                                                        else
                                                                            ot = OperationType.LATCH_OFF;
                                                                        break;
                                                                    case 4:
                                                                        if (change.FullDocument.value != 0)
                                                                            ot = OperationType.LATCH_OFF;
                                                                        else
                                                                            ot = OperationType.LATCH_ON;
                                                                        break;
                                                                    case 11:
                                                                        if (change.FullDocument.value != 0)
                                                                        {
                                                                            ot = OperationType.PULSE_ON;
                                                                            tc = TripCloseCode.CLOSE;
                                                                        }
                                                                        else
                                                                        { 
                                                                            ot = OperationType.PULSE_OFF;
                                                                            tc = TripCloseCode.TRIP;
                                                                        }
                                                                        break;
                                                                    case 13:
                                                                        if (change.FullDocument.value != 0)
                                                                        {
                                                                            ot = OperationType.LATCH_ON;
                                                                            tc = TripCloseCode.CLOSE;
                                                                        }
                                                                        else
                                                                        { 
                                                                            ot = OperationType.LATCH_OFF;
                                                                            tc = TripCloseCode.TRIP;
                                                                        }
                                                                        break;
                                                                    case 21:
                                                                        if (change.FullDocument.value != 0)
                                                                        {
                                                                            ot = OperationType.PULSE_ON;
                                                                            tc = TripCloseCode.TRIP;
                                                                        }
                                                                        else
                                                                        {
                                                                            ot = OperationType.PULSE_OFF;
                                                                            tc = TripCloseCode.CLOSE;
                                                                        }
                                                                        break;
                                                                    case 23:
                                                                        if (change.FullDocument.value != 0)
                                                                        {
                                                                            ot = OperationType.LATCH_ON;
                                                                            tc = TripCloseCode.TRIP;
                                                                        }
                                                                        else
                                                                        {
                                                                            ot = OperationType.LATCH_OFF;
                                                                            tc = TripCloseCode.CLOSE;
                                                                        }
                                                                        break;
                                                                }
                                                                ControlRelayOutputBlock crob = new ControlRelayOutputBlock(ot, tc, false, 1, 0, 0);
                                                                if (System.Convert.ToBoolean(change.FullDocument.protocolSourceCommandUseSBO))
                                                                    cmdTask = srv.master.SelectAndOperate(
                                                                        crob,
                                                                        System.Convert.ToUInt16(change.FullDocument.protocolSourceObjectAddress),
                                                                        TaskConfig.Default);
                                                                else
                                                                    cmdTask = srv.master.DirectOperate(
                                                                        crob,
                                                                        System.Convert.ToUInt16(change.FullDocument.protocolSourceObjectAddress),
                                                                        TaskConfig.Default);
                                                            }
                                                            else if (group == 41 && variation == 1)
                                                            {
                                                                if (System.Convert.ToBoolean(change.FullDocument.protocolSourceCommandUseSBO))
                                                                    cmdTask = srv.master.SelectAndOperate(
                                                                        new AnalogOutputInt32(System.Convert.ToInt32(change.FullDocument.value)),
                                                                        System.Convert.ToUInt16(change.FullDocument.protocolSourceObjectAddress),
                                                                        TaskConfig.Default);
                                                                else
                                                                    cmdTask = srv.master.DirectOperate(
                                                                        new AnalogOutputInt32(System.Convert.ToInt32(change.FullDocument.value)),
                                                                        System.Convert.ToUInt16(change.FullDocument.protocolSourceObjectAddress),
                                                                        TaskConfig.Default);
                                                            } else if (group == 41 && variation == 2)
                                                            {
                                                                if (System.Convert.ToBoolean(change.FullDocument.protocolSourceCommandUseSBO))
                                                                    cmdTask = srv.master.SelectAndOperate(
                                                                        new AnalogOutputInt16(System.Convert.ToInt16(change.FullDocument.value)),
                                                                        System.Convert.ToUInt16(change.FullDocument.protocolSourceObjectAddress),
                                                                        TaskConfig.Default);
                                                                else
                                                                    cmdTask = srv.master.DirectOperate(
                                                                        new AnalogOutputInt16(System.Convert.ToInt16(change.FullDocument.value)),
                                                                        System.Convert.ToUInt16(change.FullDocument.protocolSourceObjectAddress),
                                                                        TaskConfig.Default);
                                                            } else if (group == 41 && variation == 3)
                                                            {
                                                                if (System.Convert.ToBoolean(change.FullDocument.protocolSourceCommandUseSBO))
                                                                    cmdTask = srv.master.SelectAndOperate(
                                                                        new AnalogOutputFloat32(System.Convert.ToSingle(change.FullDocument.value)),
                                                                        System.Convert.ToUInt16(change.FullDocument.protocolSourceObjectAddress),
                                                                        TaskConfig.Default);
                                                                else
                                                                    cmdTask = srv.master.DirectOperate(
                                                                        new AnalogOutputFloat32(System.Convert.ToSingle(change.FullDocument.value)),
                                                                        System.Convert.ToUInt16(change.FullDocument.protocolSourceObjectAddress),
                                                                        TaskConfig.Default);
                                                            }
                                                            else if (group == 41 && variation == 4)
                                                            {
                                                                if (System.Convert.ToBoolean(change.FullDocument.protocolSourceCommandUseSBO))
                                                                    cmdTask = srv.master.SelectAndOperate(
                                                                        new AnalogOutputDouble64(System.Convert.ToDouble(change.FullDocument.value)),
                                                                        System.Convert.ToUInt16(change.FullDocument.protocolSourceObjectAddress),
                                                                        TaskConfig.Default);
                                                                else
                                                                    cmdTask = srv.master.DirectOperate(
                                                                        new AnalogOutputDouble64(System.Convert.ToDouble(change.FullDocument.value)),
                                                                        System.Convert.ToUInt16(change.FullDocument.protocolSourceObjectAddress),
                                                                        TaskConfig.Default);
                                                            }
                                                            else if (group == 41) // group 41, other variations defaults to float32
                                                            {
                                                                if (System.Convert.ToBoolean(change.FullDocument.protocolSourceCommandUseSBO))
                                                                    cmdTask = srv.master.SelectAndOperate(
                                                                        new AnalogOutputFloat32(System.Convert.ToSingle(change.FullDocument.value)),
                                                                        System.Convert.ToUInt16(change.FullDocument.protocolSourceObjectAddress),
                                                                        TaskConfig.Default);
                                                                else
                                                                    cmdTask = srv.master.DirectOperate(
                                                                        new AnalogOutputFloat32(System.Convert.ToSingle(change.FullDocument.value)),
                                                                        System.Convert.ToUInt16(change.FullDocument.protocolSourceObjectAddress),
                                                                        TaskConfig.Default);
                                                            }

                                                            if (cmdTask != null)
                                                                _ = cmdTask.ContinueWith((result) =>
                                                                {
                                                                    Console.WriteLine("Result: " + result.Result);
                                                                    Log("MongoDB CMD CS - " +
                                                                        srv.name +
                                                                        " - Command " +
                                                                        " TAG:" + change.FullDocument.tag +
                                                                        " GRP:" + change.FullDocument.protocolSourceCommonAddress +
                                                                        " VAR:" + change.FullDocument.protocolSourceASDU +
                                                                        " OBJ:" + change.FullDocument.protocolSourceObjectAddress +
                                                                        " Value:" + change.FullDocument.value +
                                                                        " Delivered");

                                                                    // update as delivered
                                                                    var filter =
                                                                        new BsonDocument(new BsonDocument("_id",
                                                                                change.FullDocument.id));
                                                                    var update =
                                                                        new BsonDocument("$set", new BsonDocument{
                                                                             {"delivered",  true},
                                                                             {"ack", 
                                                                                result.Result.Results.FirstOrDefault().PointState==CommandPointState.SUCCESS &&
                                                                                result.Result.Results.FirstOrDefault().Status==CommandStatus.SUCCESS },
                                                                             {"ackTimeTag", BsonValue.Create(DateTime.Now) },
                                                                             {"resultDescription", result.Result.ToString()}
                                                                            });
                                                                    var res =
                                                                        collection
                                                                            .UpdateOneAsync(filter,
                                                                            update);
                                                                });
                                                            else
                                                            {
                                                                Console.WriteLine("Command Error");
                                                            }

                                                            }
                                                            else
                                                            {
                                                                // update as expired
                                                                Log("MongoDB CMD CS - " +
                                                                srv.name +
                                                                " - Command " +
                                                                " TAG:" + change.FullDocument.tag +
                                                                " GRP:" + change.FullDocument.protocolSourceCommonAddress +
                                                                " VAR:" + change.FullDocument.protocolSourceASDU +
                                                                " OBJ:" + change.FullDocument.protocolSourceObjectAddress +
                                                                " Value:" + change.FullDocument.value +
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
                                                            " - Command " +
                                                            " TAG:" + change.FullDocument.tag +
                                                            " GRP:" + change.FullDocument.protocolSourceCommonAddress +
                                                            " VAR:" + change.FullDocument.protocolSourceASDU +
                                                            " OBJ:" + change.FullDocument.protocolSourceObjectAddress +
                                                            " Value:" + change.FullDocument.value +
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
                                                        " - Command " +
                                                        " TAG:" + change.FullDocument.tag +
                                                        " GRP:" + change.FullDocument.protocolSourceCommonAddress +
                                                        " VAR:" + change.FullDocument.protocolSourceASDU +
                                                        " OBJ:" + change.FullDocument.protocolSourceObjectAddress +
                                                        " Value:" + change.FullDocument.value +
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
                                            " - Command " +
                                            " TAG:" + change.FullDocument.tag +
                                            " GRP:" + change.FullDocument.protocolSourceCommonAddress +
                                            " VAR:" + change.FullDocument.protocolSourceASDU +
                                            " OBJ:" + change.FullDocument.protocolSourceObjectAddress +
                                            " Value:" + change.FullDocument.value +
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
                    System.Threading.Thread.Sleep(3000);
                }
            }
            while (true);
        }
    }
}