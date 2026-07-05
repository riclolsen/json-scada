/*
 * IEC 61850 Server (IEC61850-90-2 gateway/proxy) protocol driver for {json:scada} - entry point.
 *
 * Exposes JSON-SCADA realtimeData points (filtered by group1 via the connection topics list)
 * as an IEC 61850 MMS server, mirroring the behavior of the OPC server drivers.
 *
 * {json:scada} - Copyright (c) 2020-2024 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Requires libiec61850 from MZ Automation.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3. See <http://www.gnu.org/licenses/>.
 */

using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Text.Json;
using System.Threading;
using MongoDB.Bson;
using MongoDB.Driver;
using IEC61850.Common;
using IEC61850.Server;
using IEC61850.TLS;

namespace IEC61850_Server
{
    partial class MainClass
    {
        public static string CopyrightMessage = "{json:scada} IEC61850 Server Driver (IEC61850-90-2) - Copyright 2020-2024 Ricardo Olsen";
        public static string ProtocolDriverName = "IEC61850_SERVER";
        public static string DriverVersion = "0.1.0";

        static List<rtData> PointsSnapshot = new List<rtData>();
        static bool ServerStarted = false;
        static bool ModelPropertiesApplied = false;
        static string BindAddress = "0.0.0.0";
        static int BindPort = 102;

        public static void Main(string[] args)
        {
            var ci = new CultureInfo("en-US");
            Thread.CurrentThread.CurrentCulture = ci;
            Thread.CurrentThread.CurrentUICulture = ci;

            if (args.Length > 0)
            {
                if (int.TryParse(args[0], out int num)) ProtocolDriverInstanceNumber = num;
            }
            if (args.Length > 1)
            {
                if (int.TryParse(args[1], out int num)) LogLevel = num;
            }

            Log(CopyrightMessage);
            Log("Driver version " + DriverVersion);
            Log("Using libiec61850 version " + LibIEC61850.GetVersionString());
            Log("Log level: " + LogLevel);

            if (args.Length > 0 && args[0] == "selftest")
            {
                RunSelfTest(args);
                return;
            }

            string fname = JsonConfigFilePath;
            if (args.Length > 2 && File.Exists(args[2]))
                fname = args[2];
            if (!File.Exists(fname))
                fname = JsonConfigFilePathAlt;
            if (!File.Exists(fname))
            {
                Log("Missing config file " + JsonConfigFilePath);
                Environment.Exit(-1);
            }

            Log("Reading config file " + fname);
            var json = File.ReadAllText(fname);
            JSConfig = JsonSerializer.Deserialize<JSONSCADAConfig>(json);
            if (string.IsNullOrEmpty(JSConfig.mongoConnectionString))
            {
                Log("Missing MongoDB connection string in JSON config file " + fname);
                Environment.Exit(-1);
            }
            if (string.IsNullOrEmpty(JSConfig.mongoDatabaseName))
            {
                Log("Missing MongoDB database name in JSON config file " + fname);
                Environment.Exit(-1);
            }
            if (string.IsNullOrEmpty(JSConfig.nodeName))
            {
                Log("Missing nodeName parameter in JSON config file " + fname);
                Environment.Exit(-1);
            }
            Log("MongoDB database name: " + JSConfig.mongoDatabaseName);
            Log("Node name: " + JSConfig.nodeName);

            var Client = ConnectMongoClient(JSConfig);
            var DB = Client.GetDatabase(JSConfig.mongoDatabaseName);

            // validate driver instance
            var collinsts = DB.GetCollection<protocolDriverInstancesClass>(ProtocolDriverInstancesCollectionName);
            var instances = collinsts.Find(inst =>
                inst.protocolDriver == ProtocolDriverName &&
                inst.protocolDriverInstanceNumber == ProtocolDriverInstanceNumber).ToList();
            var foundInstance = false;
            foreach (var inst in instances)
            {
                foundInstance = true;
                if (!inst.enabled)
                {
                    Log("Driver instance [" + ProtocolDriverInstanceNumber + "] disabled!");
                    Environment.Exit(-1);
                }
                var nodefound = inst.nodeNames.Length == 0;
                foreach (var name in inst.nodeNames)
                    if (JSConfig.nodeName == name) nodefound = true;
                if (!nodefound)
                {
                    Log("Node '" + JSConfig.nodeName + "' not found in instances configuration!");
                    Environment.Exit(-1);
                }
                DriverInstance = inst;
                break;
            }
            if (!foundInstance)
            {
                Log("Driver instance [" + ProtocolDriverInstanceNumber + "] not found in configuration!");
                Environment.Exit(-1);
            }

            // load the single server connection for this instance (server drivers: one connection per instance)
            var collconns = DB.GetCollection<Iec61850ServerConnection>(ProtocolConnectionsCollectionName);
            var conns = collconns.Find(c =>
                c.protocolDriver == ProtocolDriverName &&
                c.protocolDriverInstanceNumber == ProtocolDriverInstanceNumber &&
                c.enabled == true).ToList();
            if (conns.Count == 0)
            {
                Log("No enabled connection found for this instance!");
                Environment.Exit(-1);
            }
            srvConn = conns[0];
            if (conns.Count > 1)
                Log("WARNING: more than one connection for this instance, using the first: " + srvConn.name);
            Log("Connection: " + srvConn.name + " [" + srvConn.protocolConnectionNumber + "]");
            ParseBindAddress();

            CmdCollection = DB.GetCollection<BsonDocument>(CommandsQueueCollectionName);

            // query the points to expose (same filter semantics as the OPC server drivers)
            var collRtData = DB.GetCollection<rtData>(RealtimeDataCollectionName);
            var filter = Builders<rtData>.Filter.Gt("_id", 0) &
                         Builders<rtData>.Filter.Ne("protocolSourceConnectionNumber", srvConn.protocolConnectionNumber);
            if (srvConn.topics != null && srvConn.topics.Length > 0)
                filter &= Builders<rtData>.Filter.In("group1", srvConn.topics);
            if (!srvConn.commandsEnabled)
                filter &= Builders<rtData>.Filter.Ne("origin", "command");

            PointsSnapshot = collRtData.Find(filter).ToList();
            Log("Points selected from realtimeData: " + PointsSnapshot.Count);
            if (PointsSnapshot.Count == 0)
                Log("WARNING: no points matched the topics filter - server model will be empty.");

            // build the IEC 61850 model (must be complete before IedServer is created)
            iedModel = BuildModel(PointsSnapshot);
            ExportManifest();

            // create the server (kept alive; started/stopped by redundancy state)
            CreateServer();

            // background threads
            new Thread(() => ProcessRedundancyMongo(JSConfig)) { IsBackground = true, Name = "Redundancy" }.Start();
            new Thread(() => ProcessMongoCS(JSConfig)) { IsBackground = true, Name = "ChangeStream" }.Start();
            new Thread(ServerUpdateLoop) { IsBackground = true, Name = "UpdateLoop" }.Start();
            new Thread(CommandInserterLoop) { IsBackground = true, Name = "CommandInserter" }.Start();

            Console.CancelKeyPress += (s, e) =>
            {
                Log("Shutdown requested...");
                Shutdown = true;
                e.Cancel = true;
            };

            int lastOpen = -1;
            while (!Shutdown)
            {
                // start/stop the MMS server following the redundancy active state
                if (Active && !ServerStarted)
                    StartServer();
                else if (!Active && ServerStarted)
                    StopServer();

                if (ServerStarted && iedServer != null)
                {
                    try
                    {
                        int open = iedServer.GetNumberOfOpenConnections();
                        if (open != lastOpen)
                        {
                            Log("Open MMS connections: " + open);
                            lastOpen = open;
                        }
                    }
                    catch (Exception) { }
                }

                Thread.Sleep(1000);
            }

            Log("Exiting...");
            try { StopServer(); iedServer?.Destroy(); } catch (Exception) { }
            Thread.Sleep(500);
            Environment.Exit(0);
        }

        static void ParseBindAddress()
        {
            BindAddress = "0.0.0.0";
            BindPort = srvConn.useSecurity ? 3782 : 102;
            var s = srvConn.ipAddressLocalBind;
            if (!string.IsNullOrEmpty(s))
            {
                int idx = s.LastIndexOf(':');
                if (idx > 0 && idx < s.Length - 1 && int.TryParse(s.Substring(idx + 1), out int port))
                {
                    BindAddress = s.Substring(0, idx);
                    BindPort = port;
                }
                else
                    BindAddress = s;
            }
            if (BindAddress == "" ) BindAddress = "0.0.0.0";
            Log($"Bind address: {BindAddress}:{BindPort}" + (srvConn.useSecurity ? " (TLS)" : ""));
        }

        static void CreateServer()
        {
            var cfg = new IedServerConfig
            {
                MaxMmsConnections = srvConn.serverModeMultiActive ? Math.Max(1, (int)srvConn.maxClientConnections) : 1,
                Edition = Iec61850Edition.EDITION_2
            };
            // buffered report buffer bytes per BRCB (heuristic: ~128 bytes/entry)
            int bufBytes = Math.Min(64 * 1024 * 1024, Math.Max(65536, (int)srvConn.maxQueueSize * 128));
            cfg.ReportBufferSize = bufBytes;

            if (srvConn.useSecurity)
            {
                var tls = BuildTlsConfig();
                iedServer = new IedServer(iedModel, tls, cfg);
                Log("TLS enabled (IEC 62351-3).");
            }
            else
            {
                iedServer = new IedServer(iedModel, cfg);
            }

            iedServer.SetServerIdentity("JSON-SCADA", "IEC61850_SERVER", DriverVersion);
            iedServer.SetConnectionIndicationHandler(OnConnectionIndication, null);
            InstallControlHandlers();
            Log("IedServer created (buffered report buffer: " + bufBytes + " bytes/BRCB).");
        }

        static void StartServer()
        {
            if (iedServer == null || ServerStarted) return;
            try
            {
                iedServer.Start(BindAddress, BindPort);
                if (!iedServer.IsRunning())
                {
                    Log("ERROR: failed to start MMS server on " + BindAddress + ":" + BindPort +
                        " (port in use or insufficient privileges for port < 1024?).");
                    return;
                }
                ServerStarted = true;
                Log("IEC 61850 MMS server STARTED on " + BindAddress + ":" + BindPort);
                ApplyModelProperties();
                ApplyInitialValues();
            }
            catch (Exception e)
            {
                Log("StartServer error: " + e.Message);
            }
        }

        static void StopServer()
        {
            if (iedServer == null || !ServerStarted) return;
            try
            {
                iedServer.Stop();
                ServerStarted = false;
                Log("IEC 61850 MMS server STOPPED (node inactive).");
            }
            catch (Exception e)
            {
                Log("StopServer error: " + e.Message);
            }
        }

        // Set static description ('d') attributes and the LPHD Proxy flags (once).
        static void ApplyModelProperties()
        {
            if (ModelPropertiesApplied) return;
            try
            {
                iedServer.LockDataModel();
                foreach (var da in ProxyAttrs)
                    iedServer.UpdateBooleanAttributeValue(da, true);
                foreach (var t in DescAttrs)
                    iedServer.UpdateVisibleStringAttributeValue(t.Item1, t.Item2);
                iedServer.UnlockDataModel();
                ModelPropertiesApplied = true;
                Log($"Applied model properties (proxy flags: {ProxyAttrs.Count}, descriptions: {DescAttrs.Count}).", LogLevelBasic);
            }
            catch (Exception e)
            {
                Log("ApplyModelProperties error: " + e.Message);
            }
        }

        // Push a snapshot of current DB values into the model right after the server starts.
        static void ApplyInitialValues()
        {
            try
            {
                iedServer.LockDataModel();
                foreach (var p in PointsSnapshot)
                {
                    var tag = p.tag?.ToString();
                    if (string.IsNullOrEmpty(tag)) continue;
                    if (!MapByTag.TryGetValue(tag, out var mp)) continue;
                    if (mp.isCommand) continue;

                    var pu = new PointUpdate
                    {
                        point = mp,
                        value = p.value?.ToDouble() ?? 0.0,
                        valueString = p.valueString?.ToString() ?? "",
                        invalid = p.invalid?.ToBoolean() ?? true,
                        substituted = p.substituted?.ToBoolean() ?? false,
                        overflow = p.overflow?.ToBoolean() ?? false,
                        transient = p.transient?.ToBoolean() ?? false,
                        test = false,
                        hasSourceTime = false,
                        sourceTimeOk = p.timeTagAtSourceOk?.ToBoolean() ?? false
                    };
                    if (p.timeTagAtSource != null && !p.timeTagAtSource.IsBsonNull)
                    {
                        try { pu.sourceTime = p.timeTagAtSource.ToUniversalTime(); pu.hasSourceTime = true; }
                        catch (Exception) { }
                    }
                    ApplyUpdate(pu);
                }
                iedServer.UnlockDataModel();
                Log("Initial values loaded into the model.", LogLevelBasic);
            }
            catch (Exception e)
            {
                Log("ApplyInitialValues error: " + e.Message);
            }
        }

        static TLSConfiguration BuildTlsConfig()
        {
            var tls = new TLSConfiguration();
            if (!string.IsNullOrEmpty(srvConn.localCertFilePath))
                tls.SetOwnCertificate(srvConn.localCertFilePath);
            if (!string.IsNullOrEmpty(srvConn.privateKeyFilePath))
                tls.SetOwnKey(srvConn.privateKeyFilePath, string.IsNullOrEmpty(srvConn.password) ? null : srvConn.password);
            if (!string.IsNullOrEmpty(srvConn.rootCertFilePath))
                tls.AddCACertificate(srvConn.rootCertFilePath);
            if (srvConn.peerCertFilesPaths != null)
                foreach (var p in srvConn.peerCertFilesPaths)
                    if (!string.IsNullOrEmpty(p)) tls.AddAllowedCertificate(p);
            tls.ChainValidation = srvConn.chainValidation;
            tls.AllowOnlyKnownCertificates = srvConn.allowOnlySpecificCertificates;

            // min/max TLS version derived from the allow flags
            TLSConfigVersion min = TLSConfigVersion.TLS_1_2, max = TLSConfigVersion.TLS_1_3;
            if (srvConn.allowTLSv10) min = TLSConfigVersion.TLS_1_0;
            else if (srvConn.allowTLSv11) min = TLSConfigVersion.TLS_1_1;
            else if (srvConn.allowTLSv12) min = TLSConfigVersion.TLS_1_2;
            else if (srvConn.allowTLSv13) min = TLSConfigVersion.TLS_1_3;
            if (srvConn.allowTLSv13) max = TLSConfigVersion.TLS_1_3;
            else if (srvConn.allowTLSv12) max = TLSConfigVersion.TLS_1_2;
            else if (srvConn.allowTLSv11) max = TLSConfigVersion.TLS_1_1;
            else if (srvConn.allowTLSv10) max = TLSConfigVersion.TLS_1_0;
            tls.SetMinTlsVersion(min);
            tls.SetMaxTlsVersion(max);
            return tls;
        }
    }
}
