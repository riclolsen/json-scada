/*
 * IEC 61850 Server driver for {json:scada} - control handling and connection supervision.
 *
 * Maps IEC 61850 control operations (SPC/APC) received from clients into JSON-SCADA
 * commandsQueue documents (identical field set to the OPC server drivers), so commands are
 * routed by JSON-SCADA to the originating source protocol driver (IEC 61850-90-2 control pass-through).
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
using IEC61850.Server;
using IEC61850.Common;

namespace IEC61850_Server
{
    partial class MainClass
    {
        public static IMongoCollection<BsonDocument> CmdCollection = null;

        // Install check + control handlers for every mapped command point.
        static void InstallControlHandlers()
        {
            if (!srvConn.commandsEnabled) return;
            int count = 0;
            foreach (var kv in MapByCtlObjRef)
            {
                var mp = kv.Value;
                if (mp.dobj == null) continue;
                iedServer.SetCheckHandler(mp.dobj, CheckHandlerImpl, mp);
                iedServer.SetControlHandler(mp.dobj, ControlHandlerImpl, mp);
                count++;
            }
            Log($"Installed control handlers for {count} command point(s).", LogLevelBasic);
        }

        static CheckHandlerResult CheckHandlerImpl(ControlAction action, object parameter, MmsValue ctlVal, bool test, bool interlockCheck)
        {
            if (!srvConn.commandsEnabled || !Active)
                return CheckHandlerResult.OBJECT_ACCESS_DENIED;
            return CheckHandlerResult.ACCEPTED;
        }

        static ControlHandlerResult ControlHandlerImpl(ControlAction action, object parameter, MmsValue ctlVal, bool test)
        {
            var mp = parameter as MappedPoint;
            if (mp == null)
                return ControlHandlerResult.FAILED;

            if (!srvConn.commandsEnabled || !Active)
            {
                action.SetAddCause(ControlAddCause.ADD_CAUSE_BLOCKED_BY_MODE);
                return ControlHandlerResult.FAILED;
            }

            double doubleVal = 0.0;
            string strVal = "";
            try
            {
                switch (mp.kind)
                {
                    case PointKind.SPC:
                        doubleVal = ctlVal.GetBoolean() ? 1.0 : 0.0;
                        strVal = doubleVal != 0.0 ? "true" : "false";
                        break;
                    case PointKind.APC:
                        doubleVal = ctlVal.ToDouble();
                        strVal = doubleVal.ToString();
                        break;
                    case PointKind.INC:
                        doubleVal = ctlVal.ToInt32();
                        strVal = doubleVal.ToString();
                        break;
                    default:
                        try { doubleVal = ctlVal.ToDouble(); } catch (Exception) { }
                        strVal = ctlVal.ToString();
                        break;
                }
            }
            catch (Exception e)
            {
                Log("Control value conversion error for " + mp.tag + ": " + e.Message);
                action.SetAddCause(ControlAddCause.ADD_CAUSE_INCONSISTENT_PARAMETERS);
                return ControlHandlerResult.FAILED;
            }

            string peer = "";
            try { peer = action.GetClientConnection()?.GetPeerAddress() ?? ""; } catch (Exception) { }

            var cmdDoc = new BsonDocument
            {
                { "protocolSourceConnectionNumber", mp.protocolSourceConnectionNumber },
                { "protocolSourceCommonAddress", mp.protocolSourceCommonAddress ?? "" },
                { "protocolSourceObjectAddress", mp.protocolSourceObjectAddress ?? "" },
                { "protocolSourceASDU", mp.protocolSourceASDU ?? "" },
                { "protocolSourceCommandDuration", mp.protocolSourceCommandDuration },
                { "protocolSourceCommandUseSBO", mp.protocolSourceCommandUseSBO },
                { "pointKey", mp.pointKey },
                { "tag", mp.tag },
                { "value", doubleVal },
                { "valueString", strVal },
                { "originatorUserName", $"IEC61850 connection: {srvConn.protocolConnectionNumber} {srvConn.name}" },
                { "originatorIpAddress", peer },
                { "timeTag", DateTime.UtcNow }
            };

            CommandQueue.Enqueue(cmdDoc);
            Log($"Command queued: {mp.tag} = {strVal} (from {peer})", LogLevelBasic);
            // fire-and-forget: JSON-SCADA routes and acknowledges asynchronously
            return ControlHandlerResult.OK;
        }

        // Background thread that persists queued commands into commandsQueue.
        static void CommandInserterLoop()
        {
            while (!Shutdown)
            {
                if (!CommandQueue.TryDequeue(out var doc))
                {
                    Thread.Sleep(50);
                    continue;
                }
                try
                {
                    CmdCollection?.InsertOne(doc);
                }
                catch (Exception e)
                {
                    Log("commandsQueue insert error: " + e.Message);
                    Thread.Sleep(1000);
                }
            }
        }

        // Connection supervision: log connects/disconnects and enforce the optional IP allow-list.
        static void OnConnectionIndication(IedServer srv, ClientConnection con, bool connected, object parameter)
        {
            string peer = "";
            try { peer = con?.GetPeerAddress() ?? ""; } catch (Exception) { }
            if (connected)
            {
                Log("IEC61850 client connected: " + peer, LogLevelBasic);
                if (srvConn.ipAddresses != null && srvConn.ipAddresses.Length > 0)
                {
                    var ip = peer.Contains(":") ? peer.Substring(0, peer.LastIndexOf(':')) : peer;
                    bool allowed = false;
                    foreach (var a in srvConn.ipAddresses)
                    {
                        if (string.IsNullOrEmpty(a)) continue;
                        var allowIp = a.Contains(":") ? a.Substring(0, a.LastIndexOf(':')) : a;
                        if (ip == allowIp) { allowed = true; break; }
                    }
                    if (!allowed)
                    {
                        Log("Client " + peer + " not in allow-list, aborting connection.", LogLevelBasic);
                        try { con.Abort(); } catch (Exception) { }
                    }
                }
            }
            else
            {
                Log("IEC61850 client disconnected: " + peer, LogLevelBasic);
            }
        }
    }
}
