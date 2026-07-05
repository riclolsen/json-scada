/*
 * IEC 61850 Server driver for {json:scada} - offline self test (diagnostic).
 *
 * Runs the full model-build + server-start path against synthetic points, without MongoDB,
 * so the libiec61850 data model, datasets, report control blocks and control handlers can be
 * validated with any IEC 61850 client. Enable with:  iec61850_server selftest [port]
 *
 * This is a diagnostic aid only; it is never reached during normal operation.
 */

using System;
using System.Collections.Generic;
using System.Threading;
using MongoDB.Bson;

namespace IEC61850_Server
{
    partial class MainClass
    {
        static void RunSelfTest(string[] args)
        {
            LogLevel = 3;
            int port = 10102;
            if (args.Length > 1 && int.TryParse(args[1], out int p)) port = p;

            Log("=== IEC61850_SERVER SELF TEST (no MongoDB) ===");

            srvConn = new Iec61850ServerConnection
            {
                protocolDriver = ProtocolDriverName,
                protocolDriverInstanceNumber = 1,
                protocolConnectionNumber = 8001,
                name = "IEC61850SRV",
                description = "self test",
                enabled = true,
                commandsEnabled = true,
                ipAddressLocalBind = "0.0.0.0:" + port,
                ipAddresses = new string[] { },
                topics = new string[] { },
                serverModeMultiActive = true,
                maxClientConnections = 2,
                maxQueueSize = 1000,
                useSecurity = false
            };

            PointsSnapshot = BuildSyntheticPoints();
            Log("Synthetic points: " + PointsSnapshot.Count);

            ParseBindAddress();
            iedModel = BuildModel(PointsSnapshot);
            ExportManifest();
            CreateServer();

            Active = true; // required for control handlers to accept commands
            StartServer();

            if (iedServer != null && iedServer.IsRunning())
                Log("SELF TEST: server RUNNING on port " + BindPort + " - browse it with an IEC 61850 client.");
            else
            {
                Log("SELF TEST: server FAILED to start.");
                Environment.Exit(-1);
            }

            // simulate a couple of live updates through the normal update path
            int tick = 0;
            var rnd = new Random();
            for (int i = 0; i < 10 && !Shutdown; i++)
            {
                foreach (var kv in MapByTag)
                {
                    var mp = kv.Value;
                    if (mp.isCommand) continue;
                    var pu = new PointUpdate
                    {
                        point = mp,
                        value = mp.kind == PointKind.MV ? rnd.NextDouble() * 100.0 : (tick % 2),
                        valueString = "s" + tick,
                        invalid = false,
                        sourceTime = DateTime.UtcNow,
                        hasSourceTime = true,
                        sourceTimeOk = true
                    };
                    UpdateQueue.Enqueue(pu);
                }
                tick++;
                // drain via the same loop the driver uses
                DrainOnce();
                Thread.Sleep(500);
            }

            Log("SELF TEST: updates applied without error. Server stays up 20 s for manual browsing...");
            for (int i = 0; i < 20 && !Shutdown; i++) Thread.Sleep(1000);

            StopServer();
            iedServer?.Destroy();
            Log("SELF TEST: done.");
            Environment.Exit(0);
        }

        static void DrainOnce()
        {
            if (iedServer == null || !iedServer.IsRunning()) return;
            iedServer.LockDataModel();
            try
            {
                while (UpdateQueue.TryDequeue(out var upd))
                    ApplyUpdate(upd);
            }
            finally { iedServer.UnlockDataModel(); }
        }

        static List<rtData> BuildSyntheticPoints()
        {
            var list = new List<rtData>();
            int id = 100;
            Action<string, string, string, string, double, string> add =
                (tag, type, group1, origin, value, valueString) =>
                {
                    list.Add(new rtData
                    {
                        _id = BsonInt32.Create(id++),
                        tag = BsonString.Create(tag),
                        type = BsonString.Create(type),
                        value = BsonDouble.Create(value),
                        valueString = BsonString.Create(valueString),
                        invalid = BsonBoolean.Create(false),
                        substituted = BsonBoolean.Create(false),
                        overflow = BsonBoolean.Create(false),
                        transient = BsonBoolean.Create(false),
                        origin = BsonString.Create(origin),
                        description = BsonString.Create(tag + " description"),
                        ungroupedDescription = BsonString.Create(tag),
                        group1 = BsonString.Create(group1),
                        group2 = BsonString.Create(""),
                        group3 = BsonString.Create(""),
                        timeTagAtSourceOk = BsonBoolean.Create(false),
                        protocolSourceASDU = BsonString.Create(""),
                        protocolSourceCommonAddress = BsonString.Create("1"),
                        protocolSourceConnectionNumber = BsonDouble.Create(999),
                        protocolSourceObjectAddress = BsonString.Create((id).ToString()),
                        protocolSourceCommandDuration = BsonDouble.Create(0),
                        protocolSourceCommandUseSBO = BsonBoolean.Create(false)
                    });
                };

            add("KAW2_DJ_52-1_STATUS", "digital", "KAW2", "supervised", 1, "");
            add("KAW2_DJ_52-2_STATUS", "digital", "KAW2", "supervised", 0, "");
            add("KAW2_MW_TOTAL", "analog", "KAW2", "supervised", 42.5, "");
            add("KAW2_KV_BUS", "analog", "KAW2", "supervised", 138.2, "");
            add("KAW2_NOTE", "string", "KAW2", "supervised", 0, "OK");
            add("KAW2_DJ_52-1_CMD", "digital", "KAW2", "command", 0, "");
            add("KAW2_TAP_SETPOINT", "analog", "KAW2", "command", 0, "");
            add("KIK3_DJ_52-1_STATUS", "digital", "KIK3", "supervised", 1, "");
            add("KIK3_MW_TOTAL", "analog", "KIK3", "supervised", 17.0, "");

            return list;
        }
    }
}
