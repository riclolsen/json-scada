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

                    Log("MongoDB Update Thread Started...");

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
                                    //var list = collection.FindSync<rtData>(new BsonDocument {{"tag", tag}}).ToList();

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
                                                        "valueBsonAtSource",
                                                        BsonDocument
                                                            .Parse(iv.valueJson)
                                                    },
                                                    {
                                                        "valueAtSource",
                                                        BsonValue
                                                            .Create(iv.value)
                                                    },
                                                    {
                                                        "valueStringAtSource",
                                                        BsonValue
                                                            .Create(iv.valueString)
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
                                                        BsonValue
                                                            .Create(false)
                                                    },
                                                    {
                                                        "invalidAtSource",
                                                        BsonValue
                                                            .Create(!iv
                                                                .quality
                                                                )
                                                    },
                                                    {
                                                        "overflowAtSource",
                                                        BsonValue
                                                            .Create(false)
                                                    },
                                                    {
                                                        "blockedAtSource",
                                                        BsonValue
                                                            .Create(false)
                                                    },
                                                    {
                                                        "substitutedAtSource",
                                                        BsonValue
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
                            LogLevelDetailed);

                            listWrites
                                .Add(new UpdateOneModel<rtData>(filt
                                        .ToBsonDocument(),
                                    update));

                            if (listWrites.Count >= BulkWriteLimit)
                                break;
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

                    while (OPCDataQueue.Count > DataBufferLimit // do not let data queue grow more than a limit
                    )
                    {
                        Log("Dequeue Data", LogLevelDetailed);
                        OPC_Value iv;
                        OPCDataQueue.TryDequeue(out iv);
                    }
                }
            }
            while (true);
        }

        public static rtData newRealtimeDoc(OPC_Value iv, double _id)
        {
            if (iv.asdu == "boolean")
                return new rtData()
                {
                    _id = _id,
                    protocolSourceASDU = iv.asdu,
                    protocolSourceCommonAddress = iv.common_address,
                    protocolSourceConnectionNumber = iv.conn_number,
                    protocolSourceObjectAddress = iv.address,
                    protocolSourceCommandUseSBO = false,
                    protocolSourceCommandDuration = 0.0,
                    alarmState = 2.0,
                    description = "OPC-UA~" + iv.conn_name + "~" + iv.display_name,
                    ungroupedDescription = iv.display_name,
                    group1 = iv.conn_name,
                    group2 = iv.common_address,
                    group3 = "",
                    stateTextFalse = "FALSE",
                    stateTextTrue = "TRUE",
                    eventTextFalse = "FALSE",
                    eventTextTrue = "TRUE",
                    origin = "supervised",
                    tag = TagFromOPCParameters(iv),
                    type = "digital",
                    value = iv.value,
                    valueString = "????",
                    alarmDisabled = false,
                    alerted = false,
                    alarmed = false,
                    alertedState = "",
                    annotation = "",
                    commandBlocked = false,
                    commandOfSupervised = 0.0,
                    commissioningRemarks = "",
                    formula = 0.0,
                    frozen = false,
                    frozenDetectTimeout = 0.0,
                    hiLimit = Double.MaxValue,
                    hihiLimit = Double.MaxValue,
                    hihihiLimit = Double.MaxValue,
                    historianDeadBand = 0.0,
                    historianPeriod = 0.0,
                    hysteresis = 0.0,
                    invalid = true,
                    invalidDetectTimeout = 60000,
                    isEvent = false,
                    kconv1 = 1.0,
                    kconv2 = 0.0,
                    location = BsonNull.Value,
                    loLimit = -Double.MaxValue,
                    loloLimit = -Double.MaxValue,
                    lololoLimit = -Double.MaxValue,
                    notes = "",
                    overflow = false,
                    parcels = BsonNull.Value,
                    priority = 0.0,
                    protocolDestinations = BsonNull.Value,
                    sourceDataUpdate = BsonNull.Value,
                    supervisedOfCommand = 0.0,
                    timeTag = BsonNull.Value,
                    timeTagAlarm = BsonNull.Value,
                    timeTagAtSource = BsonNull.Value,
                    timeTagAtSourceOk = false,
                    transient = false,
                    unit = "",
                    updatesCnt = 0,
                    valueDefault = 0.0,
                    zeroDeadband = 0.0
                };
            else
            if (iv.asdu == "string")
                return new rtData()
                {
                    _id = _id,
                    protocolSourceASDU = iv.asdu,
                    protocolSourceCommonAddress = iv.common_address,
                    protocolSourceConnectionNumber = iv.conn_number,
                    protocolSourceObjectAddress = iv.address,
                    protocolSourceCommandUseSBO = false,
                    protocolSourceCommandDuration = 0.0,
                    alarmState = -1.0,
                    description = "OPC-UA~" + iv.conn_name + "~" + iv.display_name,
                    ungroupedDescription = iv.display_name,
                    group1 = iv.conn_name,
                    group2 = iv.common_address,
                    group3 = "",
                    stateTextFalse = "",
                    stateTextTrue = "",
                    eventTextFalse = "",
                    eventTextTrue = "",
                    origin = "supervised",
                    tag = TagFromOPCParameters(iv),
                    type = "string",
                    value = 0.0,
                    valueString = iv.valueString,

                    alarmDisabled = false,
                    alerted = false,
                    alarmed = false,
                    alertedState = "",
                    annotation = "",
                    commandBlocked = false,
                    commandOfSupervised = 0.0,
                    commissioningRemarks = "",
                    formula = 0.0,
                    frozen = false,
                    frozenDetectTimeout = 0.0,
                    hiLimit = Double.MaxValue,
                    hihiLimit = Double.MaxValue,
                    hihihiLimit = Double.MaxValue,
                    historianDeadBand = 0.0,
                    historianPeriod = 0.0,
                    hysteresis = 0.0,
                    invalid = true,
                    invalidDetectTimeout = 60000,
                    isEvent = false,
                    kconv1 = 1.0,
                    kconv2 = 0.0,
                    location = BsonNull.Value,
                    loLimit = -Double.MaxValue,
                    loloLimit = -Double.MaxValue,
                    lololoLimit = -Double.MaxValue,
                    notes = "",
                    overflow = false,
                    parcels = BsonNull.Value,
                    priority = 0.0,
                    protocolDestinations = BsonNull.Value,
                    sourceDataUpdate = BsonNull.Value,
                    supervisedOfCommand = 0.0,
                    timeTag = BsonNull.Value,
                    timeTagAlarm = BsonNull.Value,
                    timeTagAtSource = BsonNull.Value,
                    timeTagAtSourceOk = false,
                    transient = false,
                    unit = "",
                    updatesCnt = 0,
                    valueDefault = 0.0,
                    zeroDeadband = 0.0,
                };

            return new rtData()
            {
                _id = _id,
                protocolSourceASDU = iv.asdu,
                protocolSourceCommonAddress = iv.common_address,
                protocolSourceConnectionNumber = iv.conn_number,
                protocolSourceObjectAddress = iv.address,
                protocolSourceCommandUseSBO = false,
                protocolSourceCommandDuration = 0.0,
                alarmState = -1.0,
                description = "OPC-UA~" + iv.conn_name + "~" + iv.display_name,
                ungroupedDescription = iv.display_name,
                group1 = iv.conn_name,
                group2 = iv.common_address,
                group3 = "",
                stateTextFalse = "",
                stateTextTrue = "",
                eventTextFalse = "",
                eventTextTrue = "",
                origin = "supervised",
                tag = TagFromOPCParameters(iv),
                type = "analog",
                value = iv.value,
                valueString = "????",

                alarmDisabled = false,
                alerted = false,
                alarmed = false,
                alertedState = "",
                annotation = "",
                commandBlocked = false,
                commandOfSupervised = 0.0,
                commissioningRemarks = "",
                formula = 0.0,
                frozen = false,
                frozenDetectTimeout = 0.0,
                hiLimit = Double.MaxValue,
                hihiLimit = Double.MaxValue,
                hihihiLimit = Double.MaxValue,
                historianDeadBand = 0.0,
                historianPeriod = 0.0,
                hysteresis = 0.0,
                invalid = true,
                invalidDetectTimeout = 60000,
                isEvent = false,
                kconv1 = 1.0,
                kconv2 = 0.0,
                location = BsonNull.Value,
                loLimit = -Double.MaxValue,
                loloLimit = -Double.MaxValue,
                lololoLimit = -Double.MaxValue,
                notes = "",
                overflow = false,
                parcels = BsonNull.Value,
                priority = 0.0,
                protocolDestinations = BsonNull.Value,
                sourceDataUpdate = BsonNull.Value,
                supervisedOfCommand = 0.0,
                timeTag = BsonNull.Value,
                timeTagAlarm = BsonNull.Value,
                timeTagAtSource = BsonNull.Value,
                timeTagAtSourceOk = false,
                transient = false,
                unit = "",
                updatesCnt = 0,
                valueDefault = 0.0,
                zeroDeadband = 0.0
            };
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
