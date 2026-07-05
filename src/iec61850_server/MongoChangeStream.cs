/*
 * IEC 61850 Server driver for {json:scada} - realtime data change stream + server update loop.
 *
 * Watches realtimeData via a MongoDB change stream (same event source as the OPC server drivers)
 * and pushes value/quality/timestamp updates into the running IEC 61850 data model. libiec61850
 * then handles report generation, buffering, integrity scans and GI natively.
 *
 * {json:scada} - Copyright (c) 2020-2024 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3. See <http://www.gnu.org/licenses/>.
 */

using System;
using System.Threading;
using MongoDB.Bson;
using MongoDB.Driver;
using IEC61850.Common;

namespace IEC61850_Server
{
    partial class MainClass
    {
        // Watch realtimeData for point updates and enqueue them for the server update loop.
        static async void ProcessMongoCS(JSONSCADAConfig jsConfig)
        {
            do
            {
                try
                {
                    var Client = ConnectMongoClient(jsConfig);
                    var DB = Client.GetDatabase(jsConfig.mongoDatabaseName);
                    var collection = DB.GetCollection<rtData>(RealtimeDataCollectionName);

                    bool isMongoLive =
                        DB.RunCommandAsync((Command<BsonDocument>)"{ping:1}").Wait(1000);
                    if (!isMongoLive)
                        throw new Exception("Error on connection " + jsConfig.mongoConnectionString);

                    Log("MongoDB CS - listening for realtime data updates...");

                    // observe updates/replaces, skip sourceDataUpdate writes (handled by cs_data_processor),
                    // and (when topics are set) restrict to the served group1 topics.
                    var filter = BuildCsFilter(srvConn.topics);
                    var pipeline = new EmptyPipelineDefinition<ChangeStreamDocument<rtData>>().Match(filter);
                    var changeStreamOptions = new ChangeStreamOptions
                    {
                        FullDocument = ChangeStreamFullDocumentOption.UpdateLookup
                    };

                    using (var cursor = await collection.WatchAsync(pipeline, changeStreamOptions))
                    {
                        await cursor.ForEachAsync(change =>
                        {
                            if (change.OperationType != ChangeStreamOperationType.Update &&
                                change.OperationType != ChangeStreamOperationType.Replace)
                                return;
                            var doc = change.FullDocument;
                            if (doc == null) return;

                            var tag = doc.tag?.ToString();
                            if (string.IsNullOrEmpty(tag)) return;
                            if (!MapByTag.TryGetValue(tag, out var mp)) return;
                            if (mp.isCommand) return; // command points are outputs, not pushed

                            var pu = new PointUpdate
                            {
                                point = mp,
                                value = doc.value?.ToDouble() ?? 0.0,
                                valueString = doc.valueString?.ToString() ?? "",
                                invalid = doc.invalid?.ToBoolean() ?? false,
                                substituted = doc.substituted?.ToBoolean() ?? false,
                                overflow = doc.overflow?.ToBoolean() ?? false,
                                transient = doc.transient?.ToBoolean() ?? false,
                                test = false,
                                hasSourceTime = false,
                                sourceTimeOk = doc.timeTagAtSourceOk?.ToBoolean() ?? false
                            };
                            if (doc.timeTagAtSource != null && !doc.timeTagAtSource.IsBsonNull)
                            {
                                try
                                {
                                    pu.sourceTime = doc.timeTagAtSource.ToUniversalTime();
                                    pu.hasSourceTime = true;
                                }
                                catch (Exception) { pu.hasSourceTime = false; }
                            }
                            UpdateQueue.Enqueue(pu);
                        });
                    }
                }
                catch (Exception e)
                {
                    Log("Exception MongoCS");
                    Log(e);
                    Thread.Sleep(3000);
                }
            }
            while (!Shutdown);
        }

        static string BuildCsFilter(string[] topics)
        {
            var baseCond =
                "{ 'updateDescription.updatedFields.sourceDataUpdate': { $exists: false } }," +
                "{ 'fullDocument._id': { $gt: 0 } }," +
                "{ operationType: 'update' }";
            if (topics != null && topics.Length > 0)
            {
                var quoted = string.Join(",", Array.ConvertAll(topics, t => "'" + t.Replace("'", "") + "'"));
                baseCond = "{ 'fullDocument.group1': { $in: [" + quoted + "] } }," + baseCond;
            }
            return "{ $or: [ { $and: [" + baseCond + "] }, { operationType: 'replace' } ] }";
        }

        // Single writer thread: drains the update queue and applies batched updates under one lock.
        static void ServerUpdateLoop()
        {
            while (!Shutdown)
            {
                if (iedServer == null || !iedServer.IsRunning())
                {
                    Thread.Sleep(100);
                    continue;
                }
                if (!UpdateQueue.TryDequeue(out var upd))
                {
                    Thread.Sleep(20);
                    continue;
                }

                iedServer.LockDataModel();
                try
                {
                    int batch = 0;
                    do
                    {
                        ApplyUpdate(upd);
                        batch++;
                    }
                    while (batch < 500 && UpdateQueue.TryDequeue(out upd));
                }
                catch (Exception e)
                {
                    Log("Update loop error: " + e.Message, LogLevelDetailed);
                }
                finally
                {
                    iedServer.UnlockDataModel();
                }
            }
        }

        static void ApplyUpdate(PointUpdate upd)
        {
            var mp = upd.point;

            // timestamp first (no trigger), then quality, then value.
            if (mp.daT != null)
            {
                var ts = new Timestamp(upd.hasSourceTime ? upd.sourceTime : DateTime.UtcNow);
                ts.SetClockNotSynchronized(upd.hasSourceTime ? !upd.sourceTimeOk : true);
                iedServer.UpdateTimestampAttributeValue(mp.daT, ts);
            }
            if (mp.daQ != null)
                iedServer.UpdateQuality(mp.daQ, MapQuality(upd));

            if (mp.daValue == null) return;
            switch (mp.kind)
            {
                case PointKind.SPS:
                case PointKind.SPC:
                    iedServer.UpdateBooleanAttributeValue(mp.daValue, upd.value != 0.0);
                    break;
                case PointKind.MV:
                case PointKind.APC:
                    iedServer.UpdateFloatAttributeValue(mp.daValue, (float)upd.value);
                    break;
                case PointKind.INS:
                case PointKind.INC:
                    iedServer.UpdateInt32AttributeValue(mp.daValue, (int)upd.value);
                    break;
                case PointKind.VSS:
                    iedServer.UpdateVisibleStringAttributeValue(mp.daValue, upd.valueString ?? "");
                    break;
            }
        }

        // Map JSON-SCADA quality flags to an IEC 61850-7-3 Quality bit-string value.
        static ushort MapQuality(PointUpdate upd)
        {
            var q = new Quality();
            if (upd.invalid)
                q.Validity = Validity.INVALID;
            else if (upd.overflow || upd.transient)
            {
                q.Validity = Validity.QUESTIONABLE;
                if (upd.overflow) q.Overflow = true;
                if (upd.transient) q.Oscillatory = true;
            }
            else
                q.Validity = Validity.GOOD;
            if (upd.substituted) q.Substituted = true;
            if (upd.test) q.Test = true;
            return q.Value;
        }
    }
}
