/*
 * OPC-DA Server Driver for {json:scada}
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * ClassicNodeManager — the single class that the Technosoftware OpcNetDaServer.exe
 * generic server loads and calls into.  It:
 *   1. Loads json-scada.json config
 *   2. Finds its protocolConnections document (protocolDriver == "OPC-DA_SERVER")
 *   3. Queries realtimeData filtered by group1 ∈ connection.topics[]
 *   4. Registers each tag as an OPC-DA item (AddItem / AddAnalogItem)
 *   5. Starts a MongoDB change-stream thread that pushes live value updates
 *   6. Handles client writes (origin=="command") by inserting into commandsQueue
 */

using System;
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using MongoDB.Bson;
using MongoDB.Driver;

namespace ServerPlugin
{
    public class ClassicNodeManager : ClassicBaseNodeManager
    {
        // -----------------------------------------------------------------------
        // Driver identity
        // -----------------------------------------------------------------------
        public static readonly string ProtocolDriverName    = "OPC-DA_SERVER";
        public static readonly string DriverVersion         = "0.1.0";
        public static int    ProtocolDriverInstanceNumber   = 1;
        public static int    LogLevelCurrent                = 0;   // 0=none 1=normal 2=detail 3=debug

        // -----------------------------------------------------------------------
        // Static state — shared between plugin instances (Technosoftware may
        // create more than one ClassicNodeManager across sessions)
        // -----------------------------------------------------------------------

        // tag → DeviceItemHandle (for SetItemValue calls)
        private static readonly ConcurrentDictionary<string, IntPtr> tagHandles_
            = new ConcurrentDictionary<string, IntPtr>();

        // tag → RtDataItem metadata (for write-back: type, connection number, etc.)
        private static readonly ConcurrentDictionary<string, RtDataItem> tagMeta_
            = new ConcurrentDictionary<string, RtDataItem>();

        // Singleton reference used by the change-stream apply callback
        private static ClassicNodeManager instance_;

        // Loaded once in OnCreateServerItems
        private static JSONSCADAConfig     jsConfig_;
        private static OpcDaConnection     connection_;
        private static IMongoCollection<RtDataItem>    rtCollection_;
        private static IMongoCollection<BsonDocument>  cmdCollection_;

        // Cancellation for background threads
        private static CancellationTokenSource cts_ = new CancellationTokenSource();

        // Protect SetItemValue calls from concurrent threads
        private static readonly object setValueLock_ = new object();

        // -----------------------------------------------------------------------
        // Logging
        // -----------------------------------------------------------------------
        private static readonly object logLock_ = new object();
        internal static void Log(string msg, int level = 1)
        {
            if (LogLevelCurrent >= level)
            {
                lock (logLock_)
                {
                    string logLine = $"[{DateTime.Now:o}] {msg}";
                    Console.WriteLine(logLine);
                    try
                    {
                        string dir = @"c:\json-scada\log";
                        if (!System.IO.Directory.Exists(dir))
                        {
                            System.IO.Directory.CreateDirectory(dir);
                        }
                        string logPath = System.IO.Path.Combine(dir, "opcdaserver.log");
                        System.IO.File.AppendAllText(logPath, logLine + Environment.NewLine);
                    }
                    catch {}
                }
            }
        }

        // -----------------------------------------------------------------------
        // ClassicBaseNodeManager overrides — called by the generic server
        // -----------------------------------------------------------------------

        /// <inheritdoc/>
        public override int OnGetLogLevel() => (int)LogLevel.Error;

        /// <inheritdoc/>
        public override string OnGetLogPath() => @"c:\json-scada\log";

        // -----------------------------------------------------------------------
        // COM / DCOM server identity
        // Loaded from MongoDB protocolConnections document so each instance
        // can have its own CLSID / ProgID registered in the Windows registry.
        // -----------------------------------------------------------------------
        public override ClassicServerDefinition OnGetDaServerDefinition()
        {
            // connection_ may not yet be populated the very first call; use defaults
            DaServer = new ClassicServerDefinition
            {
                ClsIdApp       = connection_?.clsIdApp    ?? "{A1B2C3D4-0001-0001-0001-A1B2C3D40001}",
                ClsIdServer    = connection_?.clsIdServer ?? "{A1B2C3D4-0001-0001-0001-A1B2C3D40002}",
                PrgIdServer    = connection_?.prgIdServer    ?? "JsonScada.OpcDaServer",
                PrgIdCurrServer= connection_?.prgIdCurrServer ?? "JsonScada.OpcDaServer.1",
                ServerName     = connection_?.serverName    ?? "JSON-SCADA OPC-DA Server",
                CurrServerName = (connection_?.serverName   ?? "JSON-SCADA OPC-DA Server") + " V" + DriverVersion,
                CompanyName    = "JSON-SCADA Project",
            };
            return DaServer;
        }

        // -----------------------------------------------------------------------
        // DA server operational parameters
        // -----------------------------------------------------------------------
        public override int OnGetDaServerParameters(
            out int updatePeriod,
            out char branchDelimiter,
            out DaBrowseMode browseMode)
        {
            updatePeriod    = 1000;                   // 1 s cache refresh rate (ms)
            branchDelimiter = '.';                    // "Group1.Group2.Tag"
            browseMode      = DaBrowseMode.Generic;   // browse the internal address space
            return StatusCodes.Good;
        }

        // -----------------------------------------------------------------------
        // Main startup — build address space from MongoDB, start change stream
        // -----------------------------------------------------------------------
        public override int OnCreateServerItems()
        {
            instance_ = this;

            Log($"{{json:scada}} OPC-DA Server Driver v{DriverVersion}");
            Log($"Protocol driver name: {ProtocolDriverName}");

            // Parse instance / log-level from command-line args (same convention as iec104server)
            var args = Environment.GetCommandLineArgs();
            if (args.Length > 1 && int.TryParse(args[1], out int inst))
                ProtocolDriverInstanceNumber = inst;
            if (args.Length > 2 && int.TryParse(args[2], out int lvl))
                LogLevelCurrent = lvl;

            // --- Load JSON-SCADA config ---
            try
            {
                jsConfig_ = ConfigLoader.Load(args);
                Log("Config loaded. Node: " + jsConfig_.nodeName);
                Log("MongoDB: " + jsConfig_.mongoDatabaseName);
            }
            catch (Exception ex)
            {
                Log("FATAL: Could not load config — " + ex.Message);
                return StatusCodes.Bad;
            }

            // --- Connect MongoDB ---
            MongoClient client;
            IMongoDatabase db;
            try
            {
                client = ConfigLoader.ConnectMongo(jsConfig_);
                db     = client.GetDatabase(jsConfig_.mongoDatabaseName);
                db.RunCommand<BsonDocument>(new BsonDocument("ping", 1));
                Log("MongoDB connected.");
            }
            catch (Exception ex)
            {
                Log("FATAL: MongoDB connection failed — " + ex.ToString());
                return StatusCodes.Bad;
            }

            // --- Validate driver instance ---
            try
            {
                ValidateDriverInstance(db);
            }
            catch (Exception ex)
            {
                Log("FATAL: " + ex.Message);
                return StatusCodes.Bad;
            }

            // --- Find protocol connection ---
            try
            {
                var conCol = db.GetCollection<OpcDaConnection>("protocolConnections");
                connection_ = conCol.Find(c =>
                    c.protocolDriver              == ProtocolDriverName &&
                    c.protocolDriverInstanceNumber == ProtocolDriverInstanceNumber &&
                    c.enabled                     == true
                ).FirstOrDefault();

                if (connection_ == null)
                {
                    Log("FATAL: No enabled protocolConnections document found for this driver/instance.");
                    return StatusCodes.Bad;
                }
                Log($"Connection [{connection_.protocolConnectionNumber}]: {connection_.name}");
                Log("Topics filter: " + (connection_.topics?.Length > 0
                    ? string.Join(", ", connection_.topics) : "(all)"));
            }
            catch (Exception ex)
            {
                Log("FATAL: Error reading protocolConnections — " + ex.Message);
                return StatusCodes.Bad;
            }

            // --- Get MongoDB collections ---
            rtCollection_  = db.GetCollection<RtDataItem>("realtimeData");
            cmdCollection_ = db.GetCollection<BsonDocument>("commandsQueue");

            // --- Auto-create protocolDestinations for command tags (mirrors DNP3 server logic) ---
            if (connection_.commandsEnabled && connection_.autoCreateTags)
            {
                try
                {
                    AutoCreateTagDestinations(db);
                }
                catch (Exception ex)
                {
                    Log("Warning: AutoCreateTagDestinations — " + ex.Message);
                }
            }

            // --- Query realtimeData tags ---
            List<RtDataItem> tags;
            try
            {
                tags = QueryTags();
                Log($"Found {tags.Count} tags to expose as OPC-DA items.");
            }
            catch (Exception ex)
            {
                Log("FATAL: Error querying realtimeData — " + ex.ToString());
                return StatusCodes.Bad;
            }

            // --- Register OPC-DA items ---
            int registered = 0;
            foreach (var tag in tags)
            {
                try
                {
                    RegisterTag(tag);
                    registered++;
                }
                catch (Exception ex)
                {
                    Log($"Warning: Could not register tag '{tag.tag}' — {ex.Message}", 2);
                }
            }
            Log($"Registered {registered} OPC-DA items.");

            // --- Start MongoDB change-stream (live value updates) ---
            cts_ = new CancellationTokenSource();
            MongoChangeStream.Start(
                cfg:       jsConfig_,
                topics:    connection_.topics,
                ct:        cts_.Token,
                applyFn:   ApplyUpdate,
                hasTag:    t => tagHandles_.ContainsKey(t),
                convertFn: (tag, doc) => ConvertToVariant((RtDataItem)doc)
            );
            Log("MongoDB change-stream watcher started.");

            return StatusCodes.Good;
        }

        // -----------------------------------------------------------------------
        // Shutdown
        // -----------------------------------------------------------------------
        public override void OnShutdownSignal()
        {
            Log("Shutdown signal received — stopping background threads.");
            cts_.Cancel();
            MongoChangeStream.Stop();
            Thread.Sleep(500);
        }

        // -----------------------------------------------------------------------
        // Handle client writes: insert command into commandsQueue
        // mirrors OPC-UA-Server sendCommand()
        // -----------------------------------------------------------------------
        public override int OnWriteItems(DaDeviceItemValue[] values, out int[] errors)
        {
            errors = new int[values.Length];
            for (int i = 0; i < values.Length; i++)
                errors[i] = StatusCodes.Good;

            if (!connection_.commandsEnabled)
            {
                for (int i = 0; i < values.Length; i++)
                    errors[i] = StatusCodes.Bad;
                return StatusCodes.Good;
            }

            for (int i = 0; i < values.Length; i++)
            {
                var handle = values[i].DeviceItemHandle;

                // Find tag metadata by handle
                RtDataItem meta = tagMeta_.Values.FirstOrDefault(
                    t => tagHandles_.TryGetValue(t.tag, out var h) && h == handle);

                if (meta == null)
                {
                    errors[i] = StatusCodes.Bad;
                    Log($"OnWriteItems: handle not found in tagMeta_", 2);
                    continue;
                }

                if (meta.origin != "command")
                {
                    errors[i] = StatusCodes.Bad;
                    Log($"OnWriteItems: tag '{meta.tag}' is not a command (origin='{meta.origin}')", 2);
                    continue;
                }

                try
                {
                    double doubleVal = ConvertWriteValueToDouble(values[i].Value);
                    string strVal    = values[i].Value?.ToString() ?? "";

                    Log($"OnWriteItems: tag '{meta.tag}' doubleValue='{doubleVal}' stringValue='{strVal}'");

                    var cmdDoc = new BsonDocument
                    {
                        { "protocolSourceConnectionNumber", meta.protocolSourceConnectionNumber.ToDouble() },
                        { "protocolSourceObjectAddress",    meta.protocolSourceObjectAddress },
                        { "protocolSourceCommonAddress",    meta.protocolSourceCommonAddress }, 
                        { "protocolSourceASDU",             meta.protocolSourceASDU },
                        { "protocolSourceCommandDuration",  meta.protocolSourceCommandDuration },
                        { "protocolSourceCommandUseSBO",    meta.protocolSourceCommandUseSBO },
                        { "pointKey",                       meta._id.ToInt32() },
                        { "tag",                            meta.tag },
                        { "value",                          new BsonDouble(doubleVal) },
                        { "valueString",                    strVal },
                        { "originatorUserName",             $"OPC-DA connection: {connection_.protocolConnectionNumber} {connection_.name}" },
                        { "originatorIpAddress", "" },
                        { "timeTag", DateTime.UtcNow }
                    };
                    cmdCollection_.InsertOne(cmdDoc);
                    Log($"OnWriteItems: command queued — tag='{meta.tag}' value={doubleVal}", 2);

                    // Also update local cache so the client sees the new value
                    lock (setValueLock_)
                        SetItemValue(handle, values[i].Value ?? 0.0,
                                     DaQuality.Good.Code, DateTime.UtcNow);
                }
                catch (Exception ex)
                {
                    errors[i] = StatusCodes.Bad;
                    Log($"OnWriteItems: error for tag '{meta.tag}' — {ex.Message}");
                }
            }
            return StatusCodes.Good;
        }

        // -----------------------------------------------------------------------
        // Optional: expose description as item property 101
        // -----------------------------------------------------------------------
        public override int OnQueryProperties(
            IntPtr deviceItemHandle,
            out int noProp,
            out int[] iDs)
        {
            var meta = tagMeta_.Values.FirstOrDefault(
                t => tagHandles_.TryGetValue(t.tag, out var h) && h == deviceItemHandle);

            if (meta != null && !string.IsNullOrEmpty(meta.description))
            {
                noProp = 1;
                iDs    = new[] { DaProperty.Description.Code };
                return StatusCodes.Good;
            }
            noProp = 0;
            iDs    = null;
            return StatusCodes.Bad;
        }

        public override int OnGetPropertyValue(
            IntPtr deviceItemHandle,
            int propertyId,
            out object propertyValue)
        {
            if (propertyId == DaProperty.Description.Code)
            {
                var meta = tagMeta_.Values.FirstOrDefault(
                    t => tagHandles_.TryGetValue(t.tag, out var h) && h == deviceItemHandle);
                if (meta != null)
                {
                    propertyValue = meta.description ?? "";
                    return StatusCodes.Good;
                }
            }
            propertyValue = null;
            return StatusCodes.BadInvalidPropertyId;
        }

        // =======================================================================
        // Private helpers
        // =======================================================================

        // -----------------------------------------------------------------------
        // Validate protocolDriverInstances document
        // -----------------------------------------------------------------------
        private static void ValidateDriverInstance(IMongoDatabase db)
        {
            var col = db.GetCollection<ProtocolDriverInstancesClass>("protocolDriverInstances");
            var instances = col.Find(i =>
                i.protocolDriver              == ProtocolDriverName &&
                i.protocolDriverInstanceNumber == ProtocolDriverInstanceNumber &&
                i.enabled                     == true
            ).ToList();

            if (instances.Count == 0)
                throw new Exception(
                    $"protocolDriverInstances: no enabled instance [{ProtocolDriverInstanceNumber}] " +
                    $"found for driver '{ProtocolDriverName}'.");

            var inst = instances[0];
            if (inst.nodeNames.Length > 0 &&
                !inst.nodeNames.Contains(jsConfig_.nodeName))
                throw new Exception(
                    $"Node '{jsConfig_.nodeName}' is not listed in protocolDriverInstances " +
                    $"for instance {ProtocolDriverInstanceNumber}.");

            LogLevelCurrent = Math.Max(LogLevelCurrent, inst.logLevel);
            Log($"Driver instance [{inst.protocolDriverInstanceNumber}] validated. " +
                $"LogLevel={LogLevelCurrent}");
        }

        // -----------------------------------------------------------------------
        // Query realtimeData — mirrors OPC-UA-Server lines 344-384
        // -----------------------------------------------------------------------
        private static List<RtDataItem> QueryTags()
        {
            var filterBuilder = Builders<RtDataItem>.Filter;

            // Exclude data that originates from this same connection
            var filter = filterBuilder.Ne(
                x => x.protocolSourceConnectionNumber,
                new BsonDouble(connection_.protocolConnectionNumber));

            // Only positive _ids (exclude system records)
            filter &= filterBuilder.Gt("_id", 0);

            // Filter by group1 ∈ topics (same as OPC-UA-Server)
            if (connection_.topics?.Length > 0)
                filter &= filterBuilder.In(x => x.group1, connection_.topics);

            // Exclude commands when commandsEnabled == false
            if (!connection_.commandsEnabled)
                filter &= filterBuilder.Ne(x => x.origin, "command");

            var projection = Builders<RtDataItem>.Projection
                .Include(x => x._id)
                .Include(x => x.tag)
                .Include(x => x.protocolSourceBrowsePath)
                .Include(x => x.type)
                .Include(x => x.value)
                .Include(x => x.valueString)
                .Include(x => x.timeTag)
                .Include(x => x.timeTagAtSource)
                .Include(x => x.timeTagAtSourceOk)
                .Include(x => x.invalid)
                .Include(x => x.origin)
                .Include(x => x.description)
                .Include(x => x.ungroupedDescription)
                .Include(x => x.group1)
                .Include(x => x.group2)
                .Include(x => x.group3)
                .Include(x => x.protocolSourceConnectionNumber)
                .Include(x => x.protocolSourceASDU)
                .Include(x => x.protocolSourceObjectAddress)
                .Include(x => x.protocolSourceCommonAddress)
                .Include(x => x.protocolSourceCommandDuration)
                .Include(x => x.protocolSourceCommandUseSBO)
                .Include(x => x.protocolSourceAccessLevel)
                .Include(x => x.protocolDestinations);

            return rtCollection_
                .Find(filter)
                .Project<RtDataItem>(projection)
                .Sort(Builders<RtDataItem>.Sort
                    .Ascending(x => x.protocolSourceConnectionNumber)
                    .Descending(x => x.origin))
                .ToList();
        }

        // -----------------------------------------------------------------------
        // Register a single tag as an OPC-DA item
        // -----------------------------------------------------------------------
        private void RegisterTag(RtDataItem tag)
        {
            string itemId  = BuildItemId(tag);
            object initVal = ConvertToVariant(tag);

            bool isCommand = tag.origin == "command";
            bool isWritable = isCommand ||
                              (tag.protocolSourceAccessLevel.ToDouble() > 0);

            var access = isWritable
                ? DaAccessRights.ReadWritable
                : DaAccessRights.Readable;

            IntPtr handle;

            // Use AddAnalogItem for floating-point types (exposes EU properties)
            if (IsAnalogType(tag))
            {
                AddAnalogItem(itemId, access, initVal,
                    double.MinValue, double.MaxValue, out handle);
            }
            else
            {
                AddItem(itemId, access, initVal, out handle);
            }

            tagHandles_[tag.tag] = handle;
            tagMeta_[tag.tag]    = tag;

            // Set initial value and quality
            bool bad = tag.invalid?.ToBoolean() == true;
            DateTime ts = DateTime.UtcNow;
            try
            {
                if (tag.timeTagAtSource != null)
                    ts = ((DateTime)tag.timeTagAtSource).ToUniversalTime();
            }
            catch { }

            lock (setValueLock_)
                SetItemValue(handle, initVal,
                    bad ? DaQuality.Bad.Code : DaQuality.Good.Code, ts);

            Log($"  Registered: [{itemId}] val={initVal} type={tag.type} " +
                $"access={access}", 3);
        }

        // -----------------------------------------------------------------------
        // Build dot-separated OPC ItemID from group1/2/3 + ungroupedDescription
        // mirrors OPC-UA-Server folder hierarchy but as flat dot-path
        // -----------------------------------------------------------------------
        private static string BuildItemId(RtDataItem tag)
        {
            var parts = new List<string>();

            if (!string.IsNullOrWhiteSpace(tag.protocolSourceBrowsePath))
            {
                var pathParts = tag.protocolSourceBrowsePath.Split(new char[] { '/' }, StringSplitOptions.RemoveEmptyEntries);
                foreach (var part in pathParts)
                {
                    if (!string.IsNullOrWhiteSpace(part))
                        parts.Add(part.Trim());
                }
            }
            else
            {
                if (!string.IsNullOrWhiteSpace(tag.group1)) parts.Add(tag.group1.Trim());
                if (!string.IsNullOrWhiteSpace(tag.group2)) parts.Add(tag.group2.Trim());
                if (!string.IsNullOrWhiteSpace(tag.group3)) parts.Add(tag.group3.Trim());
            }

            if (!string.IsNullOrWhiteSpace(tag.tag))
            {
                parts.Add(tag.tag.Trim());
            }

            return string.Join(".", parts);
        }

        // -----------------------------------------------------------------------
        // Convert RtDataItem value to the appropriate .NET type
        // mirrors convertValueVariant() in OPC-UA-Server/index.js
        // -----------------------------------------------------------------------
        internal static object ConvertToVariant(RtDataItem tag)
        {
            if (tag == null) return 0.0;

            switch (tag.type)
            {
                case "digital":
                    return tag.value.ToDouble() != 0.0;

                case "string":
                    return tag.valueString ?? "";

                case "analog":
                default:
                    string asduStr = tag.protocolSourceASDU?.IsString == true ? tag.protocolSourceASDU.AsString : "";
                    switch (asduStr.ToLowerInvariant())
                    {
                        case "boolean": return tag.value.ToDouble() != 0.0;
                        case "float":   return (float)tag.value.ToDouble();
                        case "int16":   return (short)(int)tag.value.ToDouble();
                        case "uint16":  return (ushort)(uint)tag.value.ToDouble();
                        case "int32":   return (int)tag.value.ToDouble();
                        case "uint32":  return (uint)tag.value.ToDouble();
                        case "int64":   return (long)tag.value.ToDouble();
                        case "uint64":  return (ulong)tag.value.ToDouble();
                        case "byte":    return (byte)(uint)tag.value.ToDouble();
                        case "sbyte":   return (sbyte)(int)tag.value.ToDouble();
                        default:        return tag.value.ToDouble();  // VT_R8
                    }
            }
        }

        // Whether the tag type warrants AddAnalogItem (for EU range properties)
        private static bool IsAnalogType(RtDataItem tag)
        {
            if (tag.type == "digital" || tag.type == "string") return false;
            string asduStr = tag.protocolSourceASDU?.IsString == true ? tag.protocolSourceASDU.AsString : "";
            var asdu = asduStr.ToLowerInvariant();
            return asdu != "boolean";
        }

        // Convert a client-supplied write value to double for commandsQueue
        private static double ConvertWriteValueToDouble(object val)
        {
            if (val == null) return 0.0;
            if (val is bool b)   return b ? 1.0 : 0.0;
            try { return Convert.ToDouble(val); }
            catch { return 0.0; }
        }

        // -----------------------------------------------------------------------
        // Apply an update from the change-stream queue to the OPC-DA cache
        // Called from MongoChangeStream.ApplyLoop on its own thread.
        // -----------------------------------------------------------------------
        private void ApplyUpdate(string tag, object newVal, short quality, DateTime ts)
        {
            if (!tagHandles_.TryGetValue(tag, out IntPtr handle)) return;
            lock (setValueLock_)
                SetItemValue(handle, newVal, quality, ts);
            Log($"Update: {tag} = {newVal}" +
                (quality == (short)DaQualityBits.Bad ? " [BAD]" : ""), 3);
        }

        // -----------------------------------------------------------------------
        // Auto-create protocolDestinations entries for command tags exported by
        // this OPC-DA Server connection that do not yet have such an entry.
        // -----------------------------------------------------------------------
        private static void AutoCreateTagDestinations(IMongoDatabase db)
        {
            Log("Auto Create Tags: adding protocolDestinations for command tags.");

            var rtBsonCol = db.GetCollection<BsonDocument>("realtimeData");
            int connNum   = connection_.protocolConnectionNumber;

            var types = new[] { "digital", "analog" };

            foreach (var type in types)
            {
                var f = Builders<BsonDocument>.Filter.And(
                    Builders<BsonDocument>.Filter.Eq("type",   type),
                    Builders<BsonDocument>.Filter.Eq("origin", "command"),
                    Builders<BsonDocument>.Filter.Ne(
                        "protocolDestinations.protocolDestinationConnectionNumber",
                        (double)connNum));

                var sort = Builders<BsonDocument>.Sort.Ascending("_id");
                foreach (var doc in rtBsonCol.Find(f).Sort(sort).ToEnumerable())
                {
                    // Topic filter — substring match, same as DNP3 driver
                    if (connection_.topics?.Length > 0)
                    {
                        string g1 = GetBsonString(doc, "group1");
                        if (!connection_.topics.Any(t => g1.Contains(t))) continue;
                    }

                    var    idVal  = doc["_id"];
                    string tagStr = GetBsonString(doc, "tag");
                    string itemId = BuildItemIdFromBson(doc);

                    Log($"Auto Create Tags: adding destination for {type} command [{idVal}] '{tagStr}' itemId={itemId}");

                    EnsureProtocolDestinationsArray(rtBsonCol, idVal, doc);
                    PushProtocolDestination(rtBsonCol, idVal, connNum, itemId);
                }
            }
        }

        // -----------------------------------------------------------------------
        // Helpers for AutoCreateTagDestinations
        // -----------------------------------------------------------------------

        private static string BuildItemIdFromBson(BsonDocument doc)
        {
            var parts = new List<string>();

            string browsePath = GetBsonString(doc, "protocolSourceBrowsePath");
            if (!string.IsNullOrWhiteSpace(browsePath))
            {
                var pathParts = browsePath.Split(new char[] { '/' }, StringSplitOptions.RemoveEmptyEntries);
                foreach (var part in pathParts)
                {
                    if (!string.IsNullOrWhiteSpace(part))
                        parts.Add(part.Trim());
                }
            }
            else
            {
                string g1 = GetBsonString(doc, "group1");
                string g2 = GetBsonString(doc, "group2");
                string g3 = GetBsonString(doc, "group3");
                if (!string.IsNullOrWhiteSpace(g1)) parts.Add(g1.Trim());
                if (!string.IsNullOrWhiteSpace(g2)) parts.Add(g2.Trim());
                if (!string.IsNullOrWhiteSpace(g3)) parts.Add(g3.Trim());
            }

            string tagStr = GetBsonString(doc, "tag");
            if (!string.IsNullOrWhiteSpace(tagStr))
            {
                parts.Add(tagStr.Trim());
            }

            return string.Join(".", parts);
        }

        /// <summary>Safely read a string BSON field, returning "" if absent or non-string.</summary>
        private static string GetBsonString(BsonDocument d, string key)
        {
            if (!d.Contains(key)) return "";
            var v = d[key];
            if (v.BsonType == BsonType.String) return v.AsString;
            if (v.BsonType == BsonType.Null)   return "";
            return "";
        }

        /// <summary>
        /// Ensures the <c>protocolDestinations</c> field exists as an array.
        /// Initialises it to <c>[]</c> if the field is absent or null — the same
        /// guard used in the DNP3 driver before the $push update.
        /// </summary>
        private static void EnsureProtocolDestinationsArray(
            IMongoCollection<BsonDocument> col, BsonValue idVal, BsonDocument doc)
        {
            if (!doc.Contains("protocolDestinations") ||
                doc["protocolDestinations"].BsonType == BsonType.Null)
            {
                col.UpdateOne(
                    Builders<BsonDocument>.Filter.Eq("_id", idVal),
                    Builders<BsonDocument>.Update.Set("protocolDestinations", new BsonArray()));
            }
        }

        /// <summary>
        /// Appends a new protocolDestinations sub-document to the tag identified
        /// by <paramref name="idVal"/>.
        /// </summary>
        private static void PushProtocolDestination(
            IMongoCollection<BsonDocument> col, BsonValue idVal, int connNum, string itemId)
        {
            var dest = new BsonDocument
            {
                { "protocolDestinationConnectionNumber", new BsonDouble(connNum) },
                { "protocolDestinationCommonAddress",    new BsonDouble(0.0)     },
                { "protocolDestinationObjectAddress",    itemId                  },
                { "protocolDestinationASDU",             new BsonDouble(0.0)     },
                { "protocolDestinationCommandDuration",  new BsonDouble(0.0)     },
                { "protocolDestinationCommandUseSBO",    BsonBoolean.False       },
                { "protocolDestinationKConv1",           new BsonDouble(1.0)     },
                { "protocolDestinationKConv2",           new BsonDouble(0.0)     },
                { "protocolDestinationGroup",            new BsonDouble(0.0)     },
                { "protocolDestinationHoursShift",       new BsonDouble(0.0)     }
            };

            col.UpdateOne(
                Builders<BsonDocument>.Filter.Eq("_id", idVal),
                Builders<BsonDocument>.Update.Push("protocolDestinations", dest));
        }
    }
}
