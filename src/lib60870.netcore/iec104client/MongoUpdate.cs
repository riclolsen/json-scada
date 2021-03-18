/* 
 * IEC 60870-5-104 Client Protocol driver for {json:scada}
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
using System.Threading;
using System.Collections.Generic;
using MongoDB.Bson;
using MongoDB.Driver;

namespace Iec10XDriver
{
    partial class MainClass
    {
        // This process updates acquired values in the mongodb collection for realtime data
        static async void ProcessMongo(JSONSCADAConfig jsConfig)
        {
            do
            {
                try
                {
                    var Client = ConnectMongoClient(jsConfig);
                    var DB = Client.GetDatabase(jsConfig.mongoDatabaseName);
                    var collection =
                        DB.GetCollection<rtData>(RealtimeDataCollectionName);
                    var collection_cmd =
                        DB
                            .GetCollection
                            <rtCommand>(CommandsQueueCollectionName);

                    var listWrites = new List<WriteModel<rtData>>();
                    do
                    {
                        bool isMongoLive =
                            DB
                                .RunCommandAsync((Command<BsonDocument>)
                                "{ping:1}")
                                .Wait(1000);
                        if (!isMongoLive)
                            throw new Exception("Error on MongoDB connection ");

                        IEC_CmdAck ia;
                        while (IECCmdAckQueue.TryDequeue(out ia))
                        {
                            var filter1 =
                                Builders<rtCommand>
                                    .Filter
                                    .Eq(m => m.protocolSourceConnectionNumber,
                                    ia.conn_number);
                            var filter2 =
                                Builders<rtCommand>
                                    .Filter
                                    .Eq(m => m.protocolSourceObjectAddress,
                                    ia.object_address);
                            var filter =
                                Builders<rtCommand>
                                    .Filter
                                    .And(filter1, filter2);

                            var update =
                                Builders<rtCommand>
                                    .Update
                                    .Set(m => m.ack, ia.ack)
                                    .Set(m => m.ackTimeTag, ia.ack_time_tag);

                            // sort by priority then by insert order
                            var sort =
                                Builders<rtCommand>.Sort.Descending("$natural");

                            var options =
                                new FindOneAndUpdateOptions<rtCommand, rtCommand
                                >();
                            options.IsUpsert = false;
                            options.Sort = sort;
                            await collection_cmd
                                .FindOneAndUpdateAsync(filter, update, options);
                        }

                        IEC_Value iv;
                        while (IECDataQueue.TryDequeue(out iv))
                        {
                            DateTime tt = DateTime.MinValue;
                            BsonValue bsontt = BsonNull.Value;
                            try
                            {
                                if (iv.hasSourceTimestampCP24)
                                {
                                    var dtnow = DateTime.Now;
                                    tt =
                                        new DateTime(
                                            dtnow.Year,
                                            dtnow.Month,
                                            dtnow.Day,
                                            dtnow.Hour,
                                            iv.sourceTimestampCP24.Minute,
                                            iv.sourceTimestampCP24.Second,
                                            iv.sourceTimestampCP24.Millisecond,
                                            DateTimeKind.Local);
                                    bsontt = BsonValue.Create(tt);
                                }
                                else
                                if (iv.hasSourceTimestampCP56)
                                {
                                    tt =
                                        new DateTime(iv
                                                .sourceTimestampCP56
                                                .Year +
                                            2000,
                                            iv.sourceTimestampCP56.Month,
                                            iv.sourceTimestampCP56.DayOfMonth,
                                            iv.sourceTimestampCP56.Hour,
                                            iv.sourceTimestampCP56.Minute,
                                            iv.sourceTimestampCP56.Second,
                                            iv.sourceTimestampCP56.Millisecond,
                                            DateTimeKind.Local);
                                    bsontt = BsonValue.Create(tt);
                                }
                            }
                            catch
                            {
                                tt = DateTime.MinValue;
                                bsontt = BsonNull.Value;
                            }

                            //below code will update one record of the data
                            var update =
                                new BsonDocument {
                                    {
                                        "$set",
                                        new BsonDocument {
                                            {
                                                "sourceDataUpdate",
                                                new BsonDocument {
                                                    {
                                                        "valueAtSource",
                                                        BsonValue
                                                            .Create(iv.value)
                                                    },
                                                    {
                                                        "valueStringAtSource",
                                                        BsonValue
                                                            .Create(iv.value.ToString())
                                                    },
                                                    {
                                                        "asduAtSource",
                                                        BsonValue
                                                            .Create(iv.asdu.ToString())
                                                    },
                                                    {
                                                        "causeOfTransmissionAtSource",
                                                        BsonValue.Create(iv.cot.ToString())
                                                    },
                                                    {
                                                        "timeTagAtSource",
                                                        bsontt
                                                    },
                                                    {
                                                        "timeTagAtSourceOk",
                                                        BsonValue
                                                            .Create(!iv
                                                                .sourceTimestampCP56
                                                                .Invalid)
                                                    },
                                                    {
                                                        "timeTag",
                                                        BsonValue
                                                            .Create(iv
                                                                .serverTimestamp)
                                                    },
                                                    {
                                                        "notTopicalAtSource",
                                                        BsonValue
                                                            .Create(iv
                                                                .quality
                                                                .NonTopical)
                                                    },
                                                    {
                                                        "invalidAtSource",
                                                        BsonValue
                                                            .Create(iv
                                                                .quality
                                                                .Invalid)
                                                    },
                                                    {
                                                        "overflowAtSource",
                                                        BsonValue
                                                            .Create(iv
                                                                .quality.
                                                                Overflow
                                                                )
                                                    },
                                                    {
                                                        "blockedAtSource",
                                                        BsonValue
                                                            .Create(iv
                                                                .quality
                                                                .Blocked)
                                                    },
                                                    {
                                                        "substitutedAtSource",
                                                        BsonValue
                                                            .Create(iv
                                                                .quality
                                                                .Substituted)
                                                    },
                                                    {
                                                        "originator",
                                                        BsonValue
                                                            .Create(ProtocolDriverName + "|" + iv.conn_number )
                                                    }
                                                }
                                            }
                                        }
                                    }
                                };

                            var filt =
                                new rtFilt
                                {
                                    protocolSourceConnectionNumber =
                                        iv.conn_number,
                                    protocolSourceCommonAddress =
                                        iv.common_address,
                                    protocolSourceObjectAddress = iv.address
                                };
                            Log("MongoDB - ADD " + iv.address + " " + iv.value,
                            LogLevelDetailed);

                            listWrites
                                .Add(new UpdateOneModel<rtData>(filt
                                        .ToBsonDocument(),
                                    update));
                        }

                        if (listWrites.Count > 0)
                        {
                            Log("MongoDB - Bulk write " + listWrites.Count);
                            var bulkWriteResult =
                                await collection.BulkWriteAsync(listWrites);
                            listWrites.Clear();
                        }
                        else
                        {
                            Thread.Sleep(100);
                        }
                    }
                    while (true);
                }
                catch (Exception e)
                {
                    Log("Exception Mongo");
                    Log(e);
                    Log(e
                        .ToString()
                        .Substring(0,
                        e.ToString().IndexOf(Environment.NewLine)));
                    System.Threading.Thread.Sleep(3000);

                    while (IECDataQueue.Count > DataBufferLimit // do not let data queue grow more than a limit
                    )
                    {
                        Log("Dequeue Data", LogLevelDetailed);
                        IEC_Value iv;
                        IECDataQueue.TryDequeue(out iv);
                    }
                }
            }
            while (true);
        }
    }
}