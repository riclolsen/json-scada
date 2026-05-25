/*
 * OPC-DA Server Driver for {json:scada}
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * MongoDB Change Stream watcher — mirrors iec104server/MongoChangeStream.cs pattern.
 * Watches realtimeData for updates filtered by group1 in connection.topics[],
 * then calls SetItemValue() on the Technosoftware generic server cache.
 */

using System;
using System.Collections.Concurrent;
using System.Linq;
using System.Threading;
using MongoDB.Bson;
using MongoDB.Driver;

namespace ServerPlugin
{
    /// <summary>
    /// Queued update: tag name + new value + quality + timestamp
    /// </summary>
    internal class TagUpdate
    {
        public string Tag      { get; set; }
        public object Value    { get; set; }
        public short  Quality  { get; set; }
        public DateTime Time   { get; set; }
    }

    /// <summary>
    /// Runs a MongoDB change-stream loop that enqueues tag value updates.
    /// The ClassicNodeManager dequeues and applies them via SetItemValue().
    /// </summary>
    internal static class MongoChangeStream
    {
        // Queue filled by change-stream thread, drained by the apply thread
        public static readonly ConcurrentQueue<TagUpdate> UpdateQueue
            = new ConcurrentQueue<TagUpdate>();

        private static volatile bool _running = false;
        private static Thread _csThread;
        private static Thread _applyThread;

        public static void Start(
            JSONSCADAConfig cfg,
            string[] topics,
            CancellationToken ct,
            Action<string, object, short, DateTime> applyFn,   // SetItemValue wrapper
            Func<string, bool> hasTag,                         // tagHandles_.ContainsKey
            Func<string, object, object> convertFn)            // ConvertToVariant(RtDataItem)
        {
            _running = true;

            // --- Thread 1: change-stream reader ---
            _csThread = new Thread(() => ChangeStreamLoop(cfg, topics, ct, convertFn))
            {
                Name = "OpcDa-MongoCS",
                IsBackground = true
            };
            _csThread.Start();

            // --- Thread 2: queue consumer (calls SetItemValue on GenSrv thread boundary) ---
            _applyThread = new Thread(() => ApplyLoop(ct, applyFn, hasTag))
            {
                Name = "OpcDa-Apply",
                IsBackground = true
            };
            _applyThread.Start();
        }

        public static void Stop()
        {
            _running = false;
        }

        // -----------------------------------------------------------------------
        // Change-stream reader loop
        // -----------------------------------------------------------------------
        private static void ChangeStreamLoop(
            JSONSCADAConfig cfg,
            string[] topics,
            CancellationToken ct,
            Func<string, object, object> convertFn)
        {
            while (!ct.IsCancellationRequested)
            {
                try
                {
                    var client = ConfigLoader.ConnectMongo(cfg);
                    var db     = client.GetDatabase(cfg.mongoDatabaseName);
                    var col    = db.GetCollection<RtDataItem>("realtimeData");

                    // Verify connectivity
                    db.RunCommand<BsonDocument>(new BsonDocument("ping", 1));
                    Log("MongoDB ChangeStream - Connected, watching for updates...");

                    var pipeline = BuildPipeline(topics);
                    var opts = new ChangeStreamOptions
                    {
                        FullDocument = ChangeStreamFullDocumentOption.UpdateLookup
                    };

                    using (var cursor = col.Watch(pipeline, opts, ct))
                    {
                        foreach (var change in cursor.ToEnumerable(ct))
                        {
                            if (change.OperationType != ChangeStreamOperationType.Update &&
                                change.OperationType != ChangeStreamOperationType.Replace)
                                continue;

                            var doc = change.FullDocument;
                            if (doc == null) continue;

                            var newVal  = convertFn(doc.tag, doc);
                            bool isBad  = doc.invalid?.ToBoolean() == true;
                            short qual  = isBad
                                ? (short)DaQualityBits.Bad
                                : (short)DaQualityBits.Good;
                            DateTime ts = DateTime.UtcNow;
                            try
                            {
                                if (doc.timeTagAtSource != null)
                                    ts = ((DateTime)doc.timeTagAtSource).ToUniversalTime();
                            }
                            catch { }

                            UpdateQueue.Enqueue(new TagUpdate
                            {
                                Tag     = doc.tag,
                                Value   = newVal,
                                Quality = qual,
                                Time    = ts
                            });
                        }
                    }
                }
                catch (OperationCanceledException)
                {
                    break;
                }
                catch (Exception ex)
                {
                    Log("ChangeStream error: " + ex.Message);
                    if (!ct.IsCancellationRequested)
                        Thread.Sleep(3000);
                }
            }
            Log("ChangeStream loop exited.");
        }

        // -----------------------------------------------------------------------
        // Queue consumer: drain updates and call SetItemValue
        // -----------------------------------------------------------------------
        private static void ApplyLoop(
            CancellationToken ct,
            Action<string, object, short, DateTime> applyFn,
            Func<string, bool> hasTag)
        {
            while (!ct.IsCancellationRequested)
            {
                bool didWork = false;
                while (UpdateQueue.TryDequeue(out TagUpdate upd))
                {
                    didWork = true;
                    try
                    {
                        if (hasTag(upd.Tag))
                            applyFn(upd.Tag, upd.Value, upd.Quality, upd.Time);
                    }
                    catch (Exception ex)
                    {
                        Log("Apply error for tag " + upd.Tag + ": " + ex.Message);
                    }
                }
                if (!didWork)
                    Thread.Sleep(50);
            }
            Log("Apply loop exited.");
        }

        // -----------------------------------------------------------------------
        // Build change-stream pipeline — mirrors OPC-UA-Server csPipeline (lines 701-733)
        // Filters:
        //   - operationType == update  AND NOT sourceDataUpdate field  AND group1 in topics
        //   - OR operationType == replace
        // -----------------------------------------------------------------------
        private static PipelineDefinition<ChangeStreamDocument<RtDataItem>, ChangeStreamDocument<RtDataItem>>
            BuildPipeline(string[] topics)
        {
            // Build group1 $in filter clause
            string group1Filter = topics != null && topics.Length > 0
                ? "{ 'fullDocument.group1': { $in: [" +
                  string.Join(",", topics.Select(t => $"\"{EscapeJson(t)}\"")) +
                  "] } }"
                : "{ 'fullDocument.group1': { $exists: true } }";

            string filterStr = $@"{{
                $or: [
                    {{ $and: [
                        {{ 'updateDescription.updatedFields.sourceDataUpdate': {{ $exists: false }} }},
                        {group1Filter},
                        {{ 'fullDocument._id': {{ $ne: -2 }} }},
                        {{ 'fullDocument._id': {{ $ne: -1 }} }},
                        {{ operationType: 'update' }}
                    ]}},
                    {{ operationType: 'replace' }}
                ]
            }}";

            return new EmptyPipelineDefinition<ChangeStreamDocument<RtDataItem>>()
                .Match(filterStr);
        }

        private static string EscapeJson(string s)
            => s?.Replace("\\", "\\\\").Replace("\"", "\\\"") ?? "";

        private static void Log(string msg)
            => Console.WriteLine($"[{DateTime.Now:o}] {msg}");
    }
}
