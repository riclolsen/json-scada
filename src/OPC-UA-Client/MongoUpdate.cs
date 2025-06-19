/* 
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
using System.Threading;
using System.Threading.Tasks;
using System.Collections.Generic;
using MongoDB.Bson;
using MongoDB.Driver;
using System.Diagnostics;
using MongoDB.Bson.Serialization.Serializers;
using MongoDB.Bson.Serialization;
using MongoDB.Bson.IO;

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

                    while (OPCDataQueue.TryDequeue(result: out var ov))
                    {
                        DateTime tt = DateTime.MinValue;
                        BsonValue bsontt = BsonNull.Value;
                        try
                        {
                            if (ov.hasSourceTimestamp)
                            {
                                bsontt = BsonValue.Create(ov.sourceTimestamp);
                            }
                        }
                        catch
                        {
                            tt = DateTime.MinValue;
                            bsontt = BsonNull.Value;
                        }

                        BsonValue valBSON = BsonNull.Value;
                        if (ov.valueJson != string.Empty)
                        {
                            try
                            {
                                valBSON = serializer.Deserialize(BsonDeserializationContext.CreateRoot(new JsonReader(ov.valueJson)));
                            }
                            catch (Exception e)
                            {
                                Log(ov.conn_name + " - " + e.Message);
                            }
                        }

                        if (ov.selfPublish)
                        {
                            // find the json-scada connection for this received value 
                            int conn_index = 0;
                            for (int index = 0; index < OPCUAconns.Count; index++)
                            {
                                if (OPCUAconns[index].protocolConnectionNumber == ov.conn_number)
                                {
                                    conn_index = index;
                                    break;
                                }
                            }

                            string tag = TagFromOPCParameters(ov);
                            if (OPCUAconns[conn_index].InsertedAddresses.Add(ov.address))
                            { // added, then insert it
                                Log(ov.conn_name + " - INSERT NEW TAG: " + tag + " - Addr:" + ov.address, LogLevelDetailed);

                                // find a new free _id key based on the connection number
                                if (OPCUAconns[conn_index].LastNewKeyCreated == 0)
                                {
                                    double AutoKeyId = ov.conn_number * AutoKeyMultiplier;
                                    var results = collectionId.Find<rtDataId>(new BsonDocument {
                                            { "_id", new BsonDocument{
                                                { "$gt", AutoKeyId },
                                                { "$lt", ( ov.conn_number + 1) * AutoKeyMultiplier }
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

                                // will enqueue to insert the new tag into mongo DB

                                if (OPCUAconns[conn_index].NodeIdsDetails.TryGetValue(ov.address, out var details))
                                {
                                    ov.parentName = details.ParentName;
                                    ov.path = details.Path;
                                }
                                else
                                {
                                    ov.parentName = "";
                                    ov.path = "";
                                    Log(ov.conn_name + " - NodeId not found in NodeIdsDetails: " + ov.address, LogLevelDetailed);
                                }

                                // will create a new command tag when the variable is writable
                                var commandOfSupervised = 0.0;
                                if (OPCUAconns[conn_index].commandsEnabled && ov.createCommandForSupervised)
                                {
                                    var insert_ = newRealtimeDoc(ov, OPCUAconns[conn_index].LastNewKeyCreated, commandOfSupervised);
                                    insert_.protocolSourcePublishingInterval = 0;
                                    insert_.protocolSourceSamplingInterval = 0;
                                    insert_.protocolSourceQueueSize = 0;
                                    listWrites.Add(new InsertOneModel<rtData>(insert_));

                                    commandOfSupervised = OPCUAconns[conn_index].LastNewKeyCreated;
                                    OPCUAconns[conn_index].LastNewKeyCreated++;
                                }

                                ov.createCommandForSupervised = false;
                                var insert = newRealtimeDoc(ov, OPCUAconns[conn_index].LastNewKeyCreated, commandOfSupervised);
                                insert.protocolSourcePublishingInterval = OPCUAconns[conn_index].autoCreateTagPublishingInterval;
                                insert.protocolSourceSamplingInterval = OPCUAconns[conn_index].autoCreateTagSamplingInterval;
                                insert.protocolSourceQueueSize = OPCUAconns[conn_index].autoCreateTagQueueSize;
                                listWrites.Add(new InsertOneModel<rtData>(insert));

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
                                                        "valueJsonAtSource", ov.valueJson
                                                    },
                                                    {
                                                        "valueAtSource",
                                                        BsonDouble
                                                            .Create(ov.value)
                                                    },
                                                    {
                                                        "valueStringAtSource",
                                                        BsonString
                                                            .Create(ov.valueString)
                                                    },
                                                    {
                                                        "asduAtSource",
                                                        BsonString
                                                            .Create(ov.asdu.ToString())
                                                    },
                                                    {
                                                        "causeOfTransmissionAtSource",
                                                        BsonString.Create(ov.cot.ToString())
                                                    },
                                                    {
                                                        "timeTagAtSource",
                                                        bsontt
                                                    },
                                                    {
                                                        "timeTagAtSourceOk",
                                                        BsonBoolean
                                                            .Create(ov.hasSourceTimestamp)
                                                    },
                                                    {
                                                        "timeTag",
                                                        BsonValue
                                                            .Create(ov
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
                                                            .Create(!ov
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
                        filt.protocolSourceConnectionNumber = ov.conn_number;
                        filt.protocolSourceObjectAddress = ov.address;
                        Log("MongoDB - ADD " + ov.address + " " + ov.value, LogLevelDebug);

                        var tooBig = false;
                        if (ov.valueJson.Length + ov.valueString.Length > 1000000 && update.ToBson().Length > 16000000)
                        {
                            Log("MongoDB - Too big update for " + ov.address + " - " + update.ToBson().Length + " bytes, will not be written to MongoDB", LogLevelDetailed);
                            tooBig = true;
                        }
                        if (!tooBig)
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
        return ov.conn_name + ";" + ov.address;
    }
}
