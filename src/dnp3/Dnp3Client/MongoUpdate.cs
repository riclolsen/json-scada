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
using System.Threading;
using System.Collections.Generic;
using MongoDB.Bson;
using MongoDB.Driver;
using Automatak.DNP3.Interface;

namespace Dnp3Driver
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
                        DNP3_Value iv;
                        while (DNP3DataQueue.TryDequeue(out iv))
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

                            string valueString;
                            if (iv.valueString == "")
                                valueString = iv.value.ToString();
                            else
                                valueString = iv.valueString;

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
                                                            .Create(valueString)
                                                    },
                                                    {
                                                        "valueJSONAtSource", iv.valueBSON                                                        
                                                    },
                                                    {
                                                        "asduAtSource",
                                                        BsonValue
                                                            .Create(iv.group.ToString() + " " + iv.variation.ToString())
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
                                                            .Create(iv.timeStampQuality==TimestampQuality.SYNCHRONIZED)
                                                    },
                                                    {
                                                        "timeTag",
                                                        BsonValue
                                                            .Create(iv.serverTimestamp)
                                                    },
                                                    {
                                                        "notTopicalAtSource",
                                                        BsonValue
                                                            .Create(iv.qCommLost)
                                                    },
                                                    {
                                                        "invalidAtSource",
                                                        BsonValue
                                                            .Create(iv.qCommLost||iv.qReferenceError||!iv.qOnline)
                                                    },
                                                    {
                                                        "overflowAtSource",
                                                        BsonValue
                                                            .Create(iv.qOverrange)
                                                    },
                                                    {
                                                        "blockedAtSource",
                                                        BsonValue
                                                            .Create(!iv.qOnline)
                                                    },
                                                    {
                                                        "substitutedAtSource",
                                                        BsonValue
                                                            .Create(iv.qRemoteForced||iv.qLocalForced)
                                                    },
                                                    {
                                                        "carryAtSource",
                                                        BsonValue
                                                            .Create(iv.qRollover)
                                                    },
                                                    {
                                                        "transientAtSource",
                                                        BsonValue
                                                            .Create(iv.qTransient)
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
                                        iv.base_group,
                                    protocolSourceObjectAddress = iv.address
                                };
                            Log("MongoDB - ADD - Connection: " + iv.conn_number + 
                                " Group: " + iv.base_group + 
                                " Address: " + iv.address + 
                                " Value: " + iv.value,
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

                    while (DNP3DataQueue.Count > DataBufferLimit // do not let data queue grow more than a limit
                    )
                    {
                        Log("Dequeue Data", LogLevelDetailed);
                        DNP3_Value iv;
                        DNP3DataQueue.TryDequeue(out iv);
                    }
                }
            }
            while (true);
        }
    }
}