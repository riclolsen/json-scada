/* 
 * OPC-DA Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020-2024 - Ricardo L. Olsen
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
using System.Threading.Tasks;
using System.Collections.Generic;
using MongoDB.Bson;
using MongoDB.Driver;
using System.Diagnostics;
using MongoDB.Bson.Serialization.Serializers;
using MongoDB.Bson.Serialization;
using MongoDB.Bson.IO;

namespace OPCDAClientDriver
{
    partial class MainClass
    {
        // This process updates acquired values in the mongodb collection for realtime data
        static public async void ProcessMongo(JSONSCADAConfig jsConfig)
        {
            var serializer = new BsonArraySerializer();
            do
            {
                try
                {
                    var Client = ConnectMongoClient(jsConfig);
                    var DB = Client.GetDatabase(jsConfig.mongoDatabaseName);
                    var collection =
                        DB.GetCollection<rtData>(RealtimeDataCollectionName);
                    var collectionId =
                        DB.GetCollection<rtDataId>(RealtimeDataCollectionName);
                    var collection_cmd =
                        DB
                            .GetCollection
                            <rtCommand>(CommandsQueueCollectionName);

                    Log("MongoDB Update Thread Started...");

                    var listWrites = new List<WriteModel<rtData>>();
                    do
                    {
                        //if (LogLevel >= LogLevelBasic && OPCDataQueue.Count > 0)
                        //  Log("MongoDB - Data queue size: " +  OPCDataQueue.Count, LogLevelBasic);

                        bool isMongoLive =
                            DB
                                .RunCommandAsync((Command<BsonDocument>)
                                "{ping:1}")
                                .Wait(2500);
                        if (!isMongoLive)
                            throw new Exception("Error on MongoDB connection ");

                        Stopwatch stopWatch = new Stopwatch();
                        stopWatch.Start();

                        OPC_Value iv;
                        while (!OPCDataQueue.IsEmpty && OPCDataQueue.TryPeek(out iv) && OPCDataQueue.TryDequeue(out iv))
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

                            BsonArray valBson = null;
                            if (iv.isArray)
                            {
                                try
                                {
                                    // valBson = BsonDocument.Parse(iv.valueJson); 
                                    valBson = serializer.Deserialize(BsonDeserializationContext.CreateRoot(new JsonReader(iv.valueJson)));
                                }
                                catch (Exception e)
                                {
                                    if (LogLevel > LogLevelDetailed) Log(iv.conn_name + " - " + e.Message, LogLevelDebug);
                                }
                            }

                            // update one existing document with received tag value (realtimeData)
                            var update =
                                new BsonDocument {
                                    {
                                        "$set",
                                        new BsonDocument {
                                            {
                                                "sourceDataUpdate",
                                                new BsonDocument {
                                                    {
                                                        "valueBsonAtSource", (valBson == null ? BsonNull.Value : valBson)
                                                    },
                                                    {
                                                        "valueJsonAtSource", iv.valueJson
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
                                                                .isGood
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
                            // update filter, avoids updating commands that can have the same address as supervised points
                            var filt =
                                new rtFilt
                                {
                                    protocolSourceConnectionNumber = iv.conn_number,
                                    protocolSourceObjectAddress = iv.address,
                                    origin = "supervised"
                                };
                            Log("MongoDB - ADD " + iv.address + " " + iv.value, LogLevelDebug);

                            listWrites
                                .Add(new UpdateOneModel<rtData>(filt
                                        .ToBsonDocument(),
                                    update));
                            var t4 = stopWatch.Elapsed;
                            if (listWrites.Count >= BulkWriteLimit)
                                break;

                            if (stopWatch.ElapsedMilliseconds > 1500)
                            {
                                break;
                            }

                            // give time to breath each 250 dequeues
                            //if ((listWrites.Count % 250)==0)
                            //{
                            //   await Task.Delay(10);
                            //Thread.Yield();
                            //Thread.Sleep(1);
                            //}
                        }

                        if (listWrites.Count > 0)
                        {
                            Log("MongoDB - Bulk writing " + listWrites.Count + ", Total enqueued data " + OPCDataQueue.Count);
                            var bulkWriteResult =
                                await collection.BulkWriteAsync(listWrites);
                            listWrites.Clear();

                            Log($"MongoDB - OK:{bulkWriteResult.IsAcknowledged} - Inserted:{bulkWriteResult.InsertedCount} - Updated:{bulkWriteResult.ModifiedCount}");

                            //Thread.Yield();
                            //Thread.Sleep(1);
                        }

                        if (OPCDataQueue.IsEmpty)
                        {
                            await Task.Delay(250);
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
                    Thread.Sleep(1000);

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
        static string TagFromOPCParameters(OPC_Value ov)
        {
            return ov.conn_name + "." + ov.address;
        }
    }
}