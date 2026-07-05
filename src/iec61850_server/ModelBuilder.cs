/*
 * IEC 61850 Server driver for {json:scada} - dynamic data model builder.
 *
 * Builds an IEC 61850 data model at startup from the JSON-SCADA realtimeData points
 * (filtered by group1 via the connection topics). Following IEC 61850-90-2, one Logical
 * Device is created per topic (group1), points are exposed as GGIO data objects, and
 * buffered/unbuffered report control blocks over per-LD datasets provide event-driven,
 * outage-tolerant transmission to control-centre clients.
 *
 * {json:scada} - Copyright (c) 2020-2024 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3. See <http://www.gnu.org/licenses/>.
 */

using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text;
using System.Text.Json;
using IEC61850.Common;
using IEC61850.Server;

namespace IEC61850_Server
{
    partial class MainClass
    {
        // CDC option bit that adds the 'd' (description) data attribute (FC=DC).
        const uint MonitorOpts = (uint)CDC.CDC_OPTION_DESC;

        // Control model / option bits (from iec61850_cdc.h).
        const uint CDC_CTL_MODEL_DIRECT_NORMAL = 1;
        const uint CDC_CTL_MODEL_SBO_NORMAL = 2;
        const uint CDC_CTL_OPTION_ORIGIN = (1u << 6);
        const uint CDC_CTL_OPTION_CTL_NUM = (1u << 7);

        // Max data objects of one category per GGIO instance (keeps DO names short and
        // reports below the MMS PDU size limit).
        const int PointsPerGGIO = 100;
        // Max FCDA entries per dataset (each report of a full dataset must fit one PDU).
        const int EntriesPerDataSet = 100;

        // Accumulates the state needed to lay out points inside one Logical Device.
        class LdContext
        {
            public string ldInst;
            public LogicalDevice ld;
            public LogicalNode lln0;
            public Dictionary<int, LogicalNode> ggios = new Dictionary<int, LogicalNode>();
            public Dictionary<string, int> categoryCounters = new Dictionary<string, int>();
            public List<string> statusEntries = new List<string>(); // "GGIOn$ST$Do"
            public List<string> mxEntries = new List<string>();      // "GGIOn$MX$Do"
            public List<string> memberRefsForConfRev = new List<string>();
        }

        // Build the whole model. Populates MapByTag / MapByCtlObjRef and returns the IedModel.
        static IedModel BuildModel(List<rtData> points)
        {
            string iedName = SanitizeMms(srvConn.name, 20);
            if (iedName.Length == 0) iedName = "JSONSCADA";
            Log($"IED name: {iedName}", LogLevelBasic);

            var model = new IedModel(iedName);

            // Group points by group1 (empty -> "GEN"), preserving _id-sorted order for stable refs.
            var groups = new List<string>();
            var byGroup = new Dictionary<string, List<rtData>>();
            foreach (var p in points.OrderBy(p => (int)p._id))
            {
                var g1 = p.group1?.ToString() ?? "";
                if (g1.Length == 0) g1 = "GEN";
                if (!byGroup.ContainsKey(g1))
                {
                    byGroup[g1] = new List<rtData>();
                    groups.Add(g1);
                }
                byGroup[g1].Add(p);
            }

            int ldNameBudget = Math.Max(8, 62 - iedName.Length); // IEDName+ldInst must stay <= 64
            var usedLdInst = new HashSet<string>();
            var ldContexts = new List<LdContext>();

            foreach (var g1 in groups)
            {
                string ldInst = UniqueName(SanitizeMms(g1, ldNameBudget), usedLdInst);
                var ctx = new LdContext { ldInst = ldInst };
                ctx.ld = new LogicalDevice(ldInst, model);
                ctx.lln0 = new LogicalNode("LLN0", ctx.ld);
                CDC.Create_CDC_ENS(ctx.lln0, "Beh", 0);
                CDC.Create_CDC_ENS(ctx.lln0, "Health", 0);
                CDC.Create_CDC_LPL(ctx.lln0, "NamPlt", 0);

                // LPHD with Proxy flag (IEC 61850-90-2 gateway marker).
                var lphd = new LogicalNode("LPHD1", ctx.ld);
                CDC.Create_CDC_DPL(lphd, "PhyNam", 0);
                CDC.Create_CDC_INS(lphd, "PhyHealth", 0);
                var proxy = CDC.Create_CDC_SPS(lphd, "Proxy", 0);
                var proxyAttr = ResolveAttr(proxy, "stVal", FunctionalConstraint.ST);
                if (proxyAttr != null) ProxyAttrs.Add(proxyAttr);

                foreach (var p in byGroup[g1])
                    MapPoint(ctx, iedName, p);

                BuildDataSetsAndReports(ctx, iedName);
                ldContexts.Add(ctx);
            }

            Log($"Model built: {ldContexts.Count} logical device(s), {MapByTag.Count} point(s), " +
                $"{MapByCtlObjRef.Count} command(s).", LogLevelBasic);
            return model;
        }

        // description-attribute list and proxy-flag list, filled during build, applied after server start.
        public static List<Tuple<IEC61850.Server.DataAttribute, string>> DescAttrs =
            new List<Tuple<IEC61850.Server.DataAttribute, string>>();
        public static List<IEC61850.Server.DataAttribute> ProxyAttrs =
            new List<IEC61850.Server.DataAttribute>();

        static void MapPoint(LdContext ctx, string iedName, rtData p)
        {
            var tag = p.tag?.ToString() ?? "";
            if (tag.Length == 0) return;
            var type = (p.type?.ToString() ?? "digital").ToLower();
            var isCommand = (p.origin?.ToString() ?? "") == "command";

            PointKind kind;
            string prefix;
            bool isMx = false;

            if (isCommand)
            {
                if (type == "analog") { kind = PointKind.APC; prefix = "AnOut"; isMx = true; }
                else { kind = PointKind.SPC; prefix = "SPCSO"; }
            }
            else
            {
                if (type == "analog") { kind = PointKind.MV; prefix = "AnIn"; isMx = true; }
                else if (type == "string" || type == "json") { kind = PointKind.VSS; prefix = "Str"; }
                else { kind = PointKind.SPS; prefix = "Ind"; }
            }

            if (!ctx.categoryCounters.ContainsKey(prefix)) ctx.categoryCounters[prefix] = 0;
            int n = ++ctx.categoryCounters[prefix];
            int ggioIdx = ((n - 1) / PointsPerGGIO) + 1;
            int localN = ((n - 1) % PointsPerGGIO) + 1;
            var ggio = GetOrCreateGGIO(ctx, ggioIdx);
            string doName = prefix + localN;

            IEC61850.Server.DataObject dobj;
            IEC61850.Server.DataAttribute daValue = null, daQ = null, daT = null;

            switch (kind)
            {
                case PointKind.MV:
                    dobj = CDC.Create_CDC_MV(ggio, doName, MonitorOpts, false);
                    daValue = ResolveAttr(dobj, "mag.f", FunctionalConstraint.MX);
                    daQ = ResolveAttr(dobj, "q", FunctionalConstraint.MX);
                    daT = ResolveAttr(dobj, "t", FunctionalConstraint.MX);
                    break;
                case PointKind.VSS:
                    dobj = CDC.Create_CDC_VSS(ggio, doName, MonitorOpts);
                    daValue = ResolveAttr(dobj, "stVal", FunctionalConstraint.ST);
                    daQ = ResolveAttr(dobj, "q", FunctionalConstraint.ST);
                    daT = ResolveAttr(dobj, "t", FunctionalConstraint.ST);
                    break;
                case PointKind.SPC:
                    dobj = CDC.Create_CDC_SPC(ggio, doName, MonitorOpts, ControlOptions(p));
                    daValue = ResolveAttr(dobj, "stVal", FunctionalConstraint.ST);
                    daQ = ResolveAttr(dobj, "q", FunctionalConstraint.ST);
                    daT = ResolveAttr(dobj, "t", FunctionalConstraint.ST);
                    break;
                case PointKind.APC:
                    dobj = CDC.Create_CDC_APC(ggio, doName, MonitorOpts, ControlOptions(p), false);
                    daValue = ResolveAttr(dobj, "mxVal.f", FunctionalConstraint.MX);
                    daQ = ResolveAttr(dobj, "q", FunctionalConstraint.MX);
                    daT = ResolveAttr(dobj, "t", FunctionalConstraint.MX);
                    break;
                case PointKind.SPS:
                default:
                    kind = PointKind.SPS;
                    dobj = CDC.Create_CDC_SPS(ggio, doName, MonitorOpts);
                    daValue = ResolveAttr(dobj, "stVal", FunctionalConstraint.ST);
                    daQ = ResolveAttr(dobj, "q", FunctionalConstraint.ST);
                    daT = ResolveAttr(dobj, "t", FunctionalConstraint.ST);
                    break;
            }

            var objRef = dobj.GetObjectReference(false);
            var mp = new MappedPoint
            {
                tag = tag,
                kind = kind,
                isCommand = isCommand,
                pointKey = (int)p._id,
                objRef = objRef,
                protocolSourceConnectionNumber = p.protocolSourceConnectionNumber?.ToDouble() ?? 0,
                protocolSourceCommonAddress = p.protocolSourceCommonAddress?.ToString() ?? "",
                protocolSourceObjectAddress = p.protocolSourceObjectAddress?.ToString() ?? "",
                protocolSourceASDU = p.protocolSourceASDU?.ToString() ?? "",
                protocolSourceCommandDuration = p.protocolSourceCommandDuration?.ToDouble() ?? 0,
                protocolSourceCommandUseSBO = p.protocolSourceCommandUseSBO?.ToBoolean() ?? false,
                dobj = dobj,
                daValue = daValue,
                daQ = daQ,
                daT = daT
            };
            MapByTag[tag] = mp;
            if (isCommand)
                MapByCtlObjRef[objRef] = mp;

            // description attribute: fill with tag so the model is self-documenting.
            var daDesc = ResolveAttr(dobj, "d", FunctionalConstraint.DC);
            if (daDesc != null)
            {
                var desc = p.description?.ToString() ?? "";
                DescAttrs.Add(Tuple.Create(daDesc, string.IsNullOrEmpty(desc) ? tag : desc));
            }

            // dataset membership (monitoring points only): DO-level FCDA carries value+q+t.
            if (!isCommand)
            {
                var entry = $"GGIO{ggioIdx}${(isMx ? "MX" : "ST")}${doName}";
                if (isMx) ctx.mxEntries.Add(entry);
                else ctx.statusEntries.Add(entry);
                ctx.memberRefsForConfRev.Add(objRef);
            }
        }

        static uint ControlOptions(rtData p)
        {
            uint model = (p.protocolSourceCommandUseSBO?.ToBoolean() ?? false)
                ? CDC_CTL_MODEL_SBO_NORMAL : CDC_CTL_MODEL_DIRECT_NORMAL;
            return model | CDC_CTL_OPTION_ORIGIN | CDC_CTL_OPTION_CTL_NUM;
        }

        static LogicalNode GetOrCreateGGIO(LdContext ctx, int idx)
        {
            if (ctx.ggios.TryGetValue(idx, out var ln))
                return ln;
            ln = new LogicalNode("GGIO" + idx, ctx.ld);
            CDC.Create_CDC_ENS(ln, "Beh", 0);
            ctx.ggios[idx] = ln;
            return ln;
        }

        static IEC61850.Server.DataAttribute ResolveAttr(IEC61850.Server.DataObject dobj, string path, FunctionalConstraint fc)
        {
            try
            {
                if (dobj == null) return null;
                return dobj.GetChildWithFc(path, fc);
            }
            catch (Exception) { return null; }
        }

        static void BuildDataSetsAndReports(LdContext ctx, string iedName)
        {
            uint confRev = Fnv32(string.Join("|", ctx.memberRefsForConfRev));
            int maxClients = Math.Max(1, (int)srvConn.maxClientConnections);

            int dsIdx = 0;
            foreach (var chunk in Chunk(ctx.statusEntries, EntriesPerDataSet))
            {
                dsIdx++;
                var dsName = "DS_ST_" + dsIdx;
                CreateDataSet(ctx, dsName, chunk);
                CreateReports(ctx, iedName, dsName, "ST", dsIdx, confRev, maxClients);
            }
            dsIdx = 0;
            foreach (var chunk in Chunk(ctx.mxEntries, EntriesPerDataSet))
            {
                dsIdx++;
                var dsName = "DS_MX_" + dsIdx;
                CreateDataSet(ctx, dsName, chunk);
                CreateReports(ctx, iedName, dsName, "MX", dsIdx, confRev, maxClients);
            }
        }

        static void CreateDataSet(LdContext ctx, string dsName, List<string> entries)
        {
            var ds = new DataSet(dsName, ctx.lln0);
            foreach (var e in entries)
                new DataSetEntry(ds, e, -1, null);
        }

        static void CreateReports(LdContext ctx, string iedName, string dsName, string cat, int dsIdx, uint confRev, int maxClients)
        {
            // dataset object reference required by ReportControlBlock_create: IEDName+ldInst/LLN0$dsName
            string dsRef = iedName + ctx.ldInst + "/LLN0$" + dsName;
            byte trgOps = (byte)(TriggerOptions.DATA_CHANGED | TriggerOptions.QUALITY_CHANGED |
                                 TriggerOptions.INTEGRITY | TriggerOptions.GI);
            byte rptOpts = (byte)(ReportOptions.SEQ_NUM | ReportOptions.TIME_STAMP |
                                  ReportOptions.REASON_FOR_INCLUSION | ReportOptions.DATA_SET |
                                  ReportOptions.CONF_REV);
            for (int i = 1; i <= maxClients; i++)
            {
                string bName = $"brcb{cat}{dsIdx:D2}{i:D2}";
                new ReportControlBlock(bName, ctx.lln0, bName, true, dsRef, confRev, trgOps, rptOpts, 500, 0);
                string uName = $"urcb{cat}{dsIdx:D2}{i:D2}";
                new ReportControlBlock(uName, ctx.lln0, uName, false, dsRef, confRev, trgOps, rptOpts, 500, 0);
            }
        }

        // ---- helpers ---------------------------------------------------------

        // Sanitize a string to a valid MMS identifier ([A-Za-z0-9_], not starting with a digit),
        // truncating to maxLen with a stable hash suffix to keep names collision-resistant.
        static string SanitizeMms(string s, int maxLen)
        {
            if (s == null) s = "";
            var sb = new StringBuilder(s.Length);
            foreach (var c in s)
                sb.Append((c < 128 && (char.IsLetterOrDigit(c))) ? c : '_');
            var r = sb.ToString().Trim('_');
            if (r.Length == 0) r = "P";
            if (char.IsDigit(r[0])) r = "P" + r;
            if (r.Length > maxLen)
                r = r.Substring(0, Math.Max(1, maxLen - 5)) + "_" + (Fnv32(s) & 0xFFFF).ToString("X4");
            return r;
        }

        static string UniqueName(string baseName, HashSet<string> used)
        {
            var name = baseName;
            int i = 1;
            while (used.Contains(name))
            {
                i++;
                name = baseName + "_" + i;
            }
            used.Add(name);
            return name;
        }

        static uint Fnv32(string s)
        {
            unchecked
            {
                uint hash = 2166136261;
                foreach (var c in s)
                {
                    hash ^= c;
                    hash *= 16777619;
                }
                return hash == 0 ? 1u : hash;
            }
        }

        static IEnumerable<List<T>> Chunk<T>(List<T> list, int size)
        {
            for (int i = 0; i < list.Count; i += size)
                yield return list.GetRange(i, Math.Min(size, list.Count - i));
        }

        // Write the tag -> objectReference mapping manifest (the 90-2 name-mapping deliverable).
        static void ExportManifest()
        {
            try
            {
                var list = new List<Dictionary<string, object>>();
                foreach (var kv in MapByTag)
                {
                    var mp = kv.Value;
                    list.Add(new Dictionary<string, object>
                    {
                        ["tag"] = mp.tag,
                        ["pointKey"] = mp.pointKey,
                        ["objectReference"] = mp.objRef,
                        ["cdc"] = mp.kind.ToString(),
                        ["isCommand"] = mp.isCommand
                    });
                }
                var json = JsonSerializer.Serialize(list, new JsonSerializerOptions { WriteIndented = true });
                var fname = "iec61850_server_map_" + srvConn.protocolConnectionNumber + ".json";
                string path = fname;
                if (Directory.Exists("../log")) path = Path.Combine("../log", fname);
                File.WriteAllText(path, json);
                Log($"Mapping manifest written: {path} ({list.Count} points)", LogLevelBasic);
            }
            catch (Exception e)
            {
                Log("Could not write mapping manifest: " + e.Message);
            }
        }
    }
}
