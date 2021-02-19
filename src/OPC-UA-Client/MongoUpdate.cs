/* 
 * OPC-UA Client Protocol driver for {json:scada}
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
using System.Security.Cryptography;
using System.Text;

namespace OPCUAClientDriver
{
    partial class MainClass
    {
        static public SortedSet<string>InsertedTags = new SortedSet<string>();

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

                    Log("MongoDB Update ThreadThread Started...");

                    var listWrites = new List<WriteModel<rtData>>();
                    do
                    {
                        //if (LogLevel >= LogLevelBasic && OPCDataQueue.Count > 0)
                        //  Log("MongoDB - Data queue size: " +  OPCDataQueue.Count, LogLevelBasic);

                        bool isMongoLive =
                            DB
                                .RunCommandAsync((Command<BsonDocument>)
                                "{ping:1}")
                                .Wait(1000);
                        if (!isMongoLive)
                            throw new Exception("Error on MongoDB connection ");

                        IEC_CmdAck ia;
                        if (OPCCmdAckQueue.Count > 0)
                        while (OPCCmdAckQueue.TryDequeue(out ia))
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

                        OPC_Value iv;
                        if (OPCDataQueue.Count > 0)
                        while (OPCDataQueue.TryDequeue(out iv))
                        {
                            DateTime tt = DateTime.MinValue;
                            BsonValue bsontt = BsonNull.Value;
                            try
                            {
                                if (iv.hasSourceTimestamp)
                                {
                                    bsontt = BsonValue.Create(iv.sourceTimestamp);
                                }
                            }
                            catch
                            {
                                tt = DateTime.MinValue;
                                bsontt = BsonNull.Value;
                            }

                            BsonDocument valJSON = new BsonDocument();
                            try
                            {
                                valJSON = BsonDocument.Parse(iv.valueJson);
                            }
                            catch (Exception e)
                            {
                                Log(iv.conn_name + " - " + e.Message);
                            }                           

                            if (iv.selfPublish)
                            {
                                string tag = TagFromOPCParameters(iv);
                                if (!InsertedTags.Contains(tag))
                                {
                                    // look for the tag
                                    var task = await collection.FindAsync<rtData>(new BsonDocument {
                                        {
                                            "tag", TagFromOPCParameters(iv)
                                        }
                                    });
                                    List<rtData> list = await task.ToListAsync();
                                    Thread.Yield();
                                    Thread.Sleep(1);

                                    if (list.Count == 0)
                                    {
                                        InsertedTags.Add(tag);
                                        Log(iv.conn_name + " - INSERT - " + iv.address);
                                        // hash to create keys
                                        var id = HashStringToInt(iv.address);
                                        var insert = newRealtimeDoc(iv, id);
                                        listWrites
                                            .Add(new InsertOneModel<rtData>(insert));
                                    }
                                }
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
                                                        "valueBsonAtSource", valJSON
                                                    },
                                                    {
                                                        "valueAtSource",
                                                        BsonDouble
                                                            .Create(iv.value)
                                                    },
                                                    {
                                                        "valueStringAtSource",
                                                        BsonString
                                                            .Create(iv.valueString)
                                                    },
                                                    {
                                                        "asduAtSource",
                                                        BsonString
                                                            .Create(iv.asdu.ToString())
                                                    },
                                                    {
                                                        "causeOfTransmissionAtSource",
                                                        BsonString.Create(iv.cot.ToString())
                                                    },
                                                    {
                                                        "timeTagAtSource",
                                                        bsontt
                                                    },
                                                    {
                                                        "timeTagAtSourceOk",
                                                        BsonBoolean
                                                            .Create(iv.hasSourceTimestamp)
                                                    },
                                                    {
                                                        "timeTag",
                                                        BsonValue
                                                            .Create(iv
                                                                .serverTimestamp)
                                                    },
                                                    {
                                                        "notTopicalAtSource",
                                                        BsonBoolean
                                                            .Create(false)
                                                    },
                                                    {
                                                        "invalidAtSource",
                                                        BsonBoolean
                                                            .Create(!iv
                                                                .quality
                                                                )
                                                    },
                                                    {
                                                        "overflowAtSource",
                                                        BsonBoolean
                                                            .Create(false)
                                                    },
                                                    {
                                                        "blockedAtSource",
                                                        BsonBoolean
                                                            .Create(false)
                                                    },
                                                    {
                                                        "substitutedAtSource",
                                                        BsonBoolean
                                                            .Create(false)
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
                            LogLevelDebug);

                            listWrites
                                .Add(new UpdateOneModel<rtData>(filt
                                        .ToBsonDocument(),
                                    update));

                            if (listWrites.Count >= BulkWriteLimit)
                                break;

                            // give time to breath each 250 dequeues
                            if ((listWrites.Count % 250)==0)
                            {
                                Thread.Yield();
                                Thread.Sleep(1);
                            }
                        }

                        if (listWrites.Count > 0)
                        {
                            Log("MongoDB - Bulk write " + listWrites.Count);
                            var bulkWriteResult =
                                await collection.BulkWriteAsync(listWrites);
                            listWrites.Clear();
                            Thread.Yield();
                            if (OPCDataQueue.Count > 0)
                            {
                                Thread.Sleep(1);
                            }
                            else
                            {
                                Thread.Sleep(100);
                            }
                        }
                        else
                        {
                            // Log("MongoDB - Sleep");
                            Thread.Yield();
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
                    System.Threading.Thread.Sleep(1000);

                    while (OPCDataQueue.Count > DataBufferLimit // do not let data queue grow more than a limit
                    )
                    {
                        Log("MongoDB - Dequeue Data", LogLevelDetailed);
                        OPC_Value iv;
                        OPCDataQueue.TryDequeue(out iv);
                    }
                }
            }
            while (true);
        }

        static Int64 HashStringToInt(string str)
        {
            MD5 md5Hasher = MD5.Create();
            var hashed = md5Hasher.ComputeHash(Encoding.UTF8.GetBytes(str));
            return -1000 - Math.Abs(BitConverter.ToInt64(hashed, 0));
        }
        static string TagFromOPCParameters(OPC_Value ov)
        {
            return ov.conn_name + ";" + ov.address;
        }
    }
}
