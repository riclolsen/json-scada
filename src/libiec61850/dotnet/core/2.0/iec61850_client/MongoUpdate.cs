/* 
 * This software implements a IEC61850 driver for JSON SCADA.
 * Copyright - 2020-2023 - Ricardo Lastra Olsen
 * 
 * Requires libiec61850 from MZ Automation.
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

namespace IEC61850_Client
{
    partial class MainClass
    {
        static public int AutoKeyMultiplier = 1000000; // maximum number of points on each connection self-published (auto numbered points)

        // This process updates acquired values in the mongodb collection for realtime data
        static public async void ProcessMongo(JSONSCADAConfig jsConfig)
        {
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

                        IECValue iv;
                        while (!IECDataQueue.IsEmpty && IECDataQueue.TryPeek(out iv) && IECDataQueue.TryDequeue(out iv))
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
                                valJSON = BsonDocument.Parse("{a:" + iv.valueJson + "}");
                            }
                            catch (Exception e)
                            {
                                Log(iv.conn_name + " - " + e.Message);
                            }

                            if (iv.selfPublish)
                            {
                                // find the json-scada connection for this received value 
                                int conn_index = 0;
                                for (int index = 0; index < Iec61850Connections.Count; index++)
                                {
                                    if (Iec61850Connections[index].protocolConnectionNumber == iv.conn_number)
                                        conn_index = index;
                                }

                                string tag = TagFromParameters(iv);
                                if (!Iec61850Connections[conn_index].InsertedTags.Contains(tag))
                                { // tag not yet inserted
                                    // put the tag in the list of inserted, then insert it

                                    Iec61850Connections[conn_index].InsertedTags.Add(tag);

                                    Log(iv.conn_name + " - INSERT NEW TAG: " + tag + " - Addr:" + iv.address);

                                    // find a new freee _id key based on the connection number
                                    if (Iec61850Connections[conn_index].LastNewKeyCreated == 0)
                                    {
                                        Double AutoKeyId = iv.conn_number * AutoKeyMultiplier;
                                        var results = collectionId.Find<rtDataId>(new BsonDocument {
                                            { "_id", new BsonDocument{
                                                { "$gt", AutoKeyId },
                                                { "$lt", ( iv.conn_number + 1) * AutoKeyMultiplier }
                                                }
                                            }
                                            }).Sort(Builders<rtDataId>.Sort.Descending("_id"))
                                            .Limit(1)
                                            .ToList();

                                        if (results.Count > 0)
                                        {
                                            Iec61850Connections[conn_index].LastNewKeyCreated = results[0]._id.ToDouble() + 1;
                                        }
                                        else
                                        {
                                            Iec61850Connections[conn_index].LastNewKeyCreated = AutoKeyId;
                                        }
                                    }
                                    else
                                        Iec61850Connections[conn_index].LastNewKeyCreated = Iec61850Connections[conn_index].LastNewKeyCreated + 1;

                                    var id = Iec61850Connections[conn_index].LastNewKeyCreated;

                                    // will enqueue to insert the new tag into mongo DB
                                    var insert = newRealtimeDoc(iv, id);
                                    listWrites
                                        .Add(new InsertOneModel<rtData>(insert));

                                    // will imediatelly be followed by an update below (to the same tag)
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

                            // update filter, avoids updating commands that can have the same address as supervised points
                            var filt =
                                new rtFilt
                                {
                                    protocolSourceConnectionNumber =
                                        iv.conn_number,
                                    protocolSourceObjectAddress = iv.address,
                                    origin = "supervised"
                                };
                            Log("MongoDB - ADD " + iv.address + " " + iv.value,
                            LogLevelDebug);

                            listWrites
                                .Add(new UpdateOneModel<rtData>(filt
                                        .ToBsonDocument(),
                                    update));

                            if (listWrites.Count >= BulkWriteLimit)
                                break;

                            if (stopWatch.ElapsedMilliseconds > 400)
                                break;

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
                            Log("MongoDB - Bulk writing " + listWrites.Count + ", Total enqueued data " + IECDataQueue.Count);
                            var bulkWriteResult =
                                await collection.BulkWriteAsync(listWrites);
                            listWrites.Clear();

                            Log($"MongoDB - OK:{bulkWriteResult.IsAcknowledged} - Inserted:{bulkWriteResult.InsertedCount} - Updated:{bulkWriteResult.ModifiedCount}");

                            //Thread.Yield();
                            //Thread.Sleep(1);
                        }

                        if (IECDataQueue.IsEmpty)
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

                    while (IECDataQueue.Count > DataBufferLimit // do not let data queue grow more than a limit
                    )
                    {
                        Log("MongoDB - Dequeue Data", LogLevelDetailed);
                        IECValue iv;
                        IECDataQueue.TryDequeue(out iv);
                    }
                }
            }
            while (true);
        }
    }
}
