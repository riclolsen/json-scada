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
using System.Threading;
using System.Threading.Tasks;
using System.Collections.Generic;
using MongoDB.Bson;
using MongoDB.Driver;
using System.Diagnostics;
using MongoDB.Bson.Serialization.Serializers;
using MongoDB.Bson.Serialization;
using MongoDB.Bson.IO;

namespace OPCUAClientDriver
{
    partial class MainClass
    {
        static public int AutoKeyMultiplier = 1000000; // maximum number of points on each connection self-published (auto numbered points)

        // This process updates acquired values in the mongodb collection for realtime data
        static public async void ProcessMongo()
        {
            do
            {
                try
                {
                    var serializer = new BsonValueSerializer();
                    var Client = ConnectMongoClient(JSConfig);
                    var DB = Client.GetDatabase(JSConfig.mongoDatabaseName);
                    var collection =
                        DB.GetCollection<rtData>(RealtimeDataCollectionName);
                    var collectionId =
                        DB.GetCollection<rtDataId>(RealtimeDataCollectionName);
                    var collection_cmd =
                        DB
                            .GetCollection
                            <rtCommand>(CommandsQueueCollectionName);
                    var listWrites = new List<WriteModel<rtData>>();
                    var filt = new rtFilt
                    {
                        protocolSourceConnectionNumber = 0,
                        protocolSourceObjectAddress = "",
                        origin = "supervised"
                    };

                    Log("MongoDB Update Thread Started...");

                    do
                    {
                        Stopwatch stopWatch = new Stopwatch();
                        stopWatch.Start();

                        while (OPCDataQueue.TryDequeue(result: out var iv))
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

                            BsonValue valBSON = BsonNull.Value;
                            if (iv.valueJson != string.Empty)
                            {
                                try
                                {
                                    valBSON = serializer.Deserialize(BsonDeserializationContext.CreateRoot(new JsonReader(iv.valueJson)));
                                }
                                catch (Exception e)
                                {
                                    Log(iv.conn_name + " - " + e.Message);
                                }
                            }

                            if (iv.selfPublish)
                            {
                                // find the json-scada connection for this received value 
                                int conn_index = 0;
                                for (int index = 0; index < OPCUAconns.Count; index++)
                                {
                                    if (OPCUAconns[index].protocolConnectionNumber == iv.conn_number)
                                        conn_index = index;
                                }

                                string tag = TagFromOPCParameters(iv);
                                if (OPCUAconns[conn_index].InsertedTags.Add(tag))
                                { // added, then insert it
                                    Log(iv.conn_name + " - INSERT NEW TAG: " + tag + " - Addr:" + iv.address, LogLevelDetailed);

                                    // find a new freee _id key based on the connection number
                                    if (OPCUAconns[conn_index].LastNewKeyCreated == 0)
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
                                            OPCUAconns[conn_index].LastNewKeyCreated = results[0]._id.ToDouble() + 1;
                                        }
                                        else
                                        {
                                            OPCUAconns[conn_index].LastNewKeyCreated = AutoKeyId;
                                        }
                                    }
                                    else
                                        OPCUAconns[conn_index].LastNewKeyCreated = OPCUAconns[conn_index].LastNewKeyCreated + 1;

                                    var id = OPCUAconns[conn_index].LastNewKeyCreated;

                                    // will enqueue to insert the new tag into mongo DB
                                    var insert = newRealtimeDoc(iv, id);
                                    insert.protocolSourcePublishingInterval = OPCUAconns[conn_index].autoCreateTagPublishingInterval;
                                    insert.protocolSourceSamplingInterval = OPCUAconns[conn_index].autoCreateTagSamplingInterval;
                                    insert.protocolSourceQueueSize = OPCUAconns[conn_index].autoCreateTagQueueSize;
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
                                                        "valueBsonAtSource", valBSON
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
                            filt.protocolSourceConnectionNumber = iv.conn_number;
                            filt.protocolSourceObjectAddress = iv.address;
                            Log("MongoDB - ADD " + iv.address + " " + iv.value, LogLevelDebug);

                            listWrites
                                .Add(new UpdateOneModel<rtData>(
                                    filt.ToBsonDocument(),
                                    update));

                            if (listWrites.Count >= BulkWriteLimit)
                                break;

                            if (stopWatch.ElapsedMilliseconds > 750)
                            {
                                Log($"break ms {stopWatch.ElapsedMilliseconds}");
                                break;
                            }
                        }

                        if (listWrites.Count > 0)
                        {
                            stopWatch.Restart();
                            Log("MongoDB - Bulk writing " + listWrites.Count + ", Total enqueued data " + OPCDataQueue.Count);
                            try
                            {
                                var bulkWriteResult =
                                  collection
                                    .WithWriteConcern(WriteConcern.W1)
                                    .BulkWrite(listWrites, new BulkWriteOptions
                                    {
                                        IsOrdered = true,
                                        BypassDocumentValidation = true,
                                    });
                                Log($"MongoDB - Bulk write - Inserted:{bulkWriteResult.InsertedCount} - Updated:{bulkWriteResult.ModifiedCount}");
                                var ups = (uint)((float)listWrites.Count / ((float)stopWatch.ElapsedMilliseconds / 1000));
                                Log($"MongoDB - Bulk written in {stopWatch.ElapsedMilliseconds} ms, updates per second: {ups}");
                                listWrites.Clear();
                            }
                            catch (Exception e)
                            {
                                Log($"MongoDB - Bulk write error - " + e.Message);
                            }
                        }

                        if (OPCDataQueue.Count == 0)
                        {
                            await Task.Delay(200);
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
                }
            }
            while (true);
        }
        static string TagFromOPCParameters(OPC_Value ov)
        {
            if (ov.isCommand)
                return ov.conn_name + ";" + ov.address + "-Cmd";
            else
                return ov.conn_name + ";" + ov.address;
        }
    }
}
