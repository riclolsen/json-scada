/*
 * IEC 60870-5-104 Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
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
using MongoDB.Bson;
using MongoDB.Driver;
using lib60870.CS101;

namespace Iec10XDriver
{
    partial class MainClass
    {
        public static double AutoKeyMultiplier = 1000000;

        // Normalize time-tagged and short TypeIDs to their canonical base types.
        public static int MapAsduToBaseType(TypeID asdu)
        {
            switch ((int)asdu)
            {
                case 2:  case 30: return 1;   // Single Point (with/without timestamp)
                case 4:  case 31: return 3;   // Double Point
                case 6:  case 32: return 5;   // Step Position
                case 8:  case 33: return 7;   // Bitstring 32
                case 10: case 21: case 34: return 9;  // Measured Normalized
                case 12: case 35: return 11;  // Measured Scaled
                case 14: case 36: return 13;  // Measured Float
                case 16: case 37: return 15;  // Integrated Totals
                default: return (int)asdu;
            }
        }

        public static string Iec10xTypeDescription(int baseType)
        {
            switch (baseType)
            {
                case 1:  return "Single Point";
                case 3:  return "Double Point";
                case 5:  return "Step Position";
                case 7:  return "Bitstring 32";
                case 9:  return "Measured Normalized";
                case 11: return "Measured Scaled";
                case 13: return "Measured Float";
                case 15: return "Integrated Totals";
                case 20: return "Packed Single Point";
                default: return "ASDU " + baseType;
            }
        }

        // Allocates the next _id in the connection's dedicated partition [connNumber*1e6, (connNumber+1)*1e6).
        // First call queries MongoDB for the current max; subsequent calls just increment the cached value.
        public static double GetNextAutoKey(IEC10X_connection srv, IMongoCollection<BsonDocument> colRt)
        {
            if (srv.LastNewKeyCreated == 0)
            {
                double baseKey = srv.protocolConnectionNumber * AutoKeyMultiplier;
                double topKey  = (srv.protocolConnectionNumber + 1) * AutoKeyMultiplier;
                var filter = new BsonDocument("_id", new BsonDocument
                {
                    { "$gt", baseKey },
                    { "$lt", topKey }
                });
                var res = colRt
                    .Find(filter)
                    .Sort(Builders<BsonDocument>.Sort.Descending("_id"))
                    .Limit(1)
                    .FirstOrDefault();
                srv.LastNewKeyCreated = res != null ? res["_id"].ToDouble() + 1 : baseKey;
            }
            else
                srv.LastNewKeyCreated += 1;
            return srv.LastNewKeyCreated;
        }

        // Builds a complete realtimeData document for a new auto-created tag.
        // IEC clients never create command twins — monitor direction has no output-status TypeIDs.
        public static BsonDocument NewRealtimeTagDoc(IEC_Value iv, string connName, double id)
        {
            int    baseType   = MapAsduToBaseType(iv.asdu);
            string desc       = Iec10xTypeDescription(baseType);
            string tag        = connName + ";" + iv.common_address + ";" + iv.address;
            bool   isDigital  = iv.isDigital;
            return new BsonDocument
            {
                { "_id",         id },
                { "tag",         tag },
                { "type",        isDigital ? "digital" : "analog" },
                { "origin",      "supervised" },
                { "description", connName + "~" + desc + "~" + iv.address },
                { "ungroupedDescription", desc + " " + iv.address },
                { "group1",      connName },
                { "group2",      "CA " + iv.common_address },
                { "group3",      "" },
                { "protocolSourceConnectionNumber", (double)iv.conn_number },
                { "protocolSourceCommonAddress",    (double)iv.common_address },
                { "protocolSourceObjectAddress",    (double)iv.address },
                { "protocolSourceASDU",             (double)baseType },
                { "protocolSourceCommandDuration",  0.0 },
                { "protocolSourceCommandUseSBO",    false },
                { "commandOfSupervised",            0.0 },
                { "supervisedOfCommand",            0.0 },
                { "kconv1",  1.0 },
                { "kconv2",  0.0 },
                { "alarmState",    isDigital ? 2.0 : -1.0 },
                { "stateTextFalse", isDigital ? "FALSE" : "" },
                { "stateTextTrue",  isDigital ? "TRUE"  : "" },
                { "eventTextFalse", isDigital ? "FALSE" : "" },
                { "eventTextTrue",  isDigital ? "TRUE"  : "" },
                { "value",         iv.value },
                { "valueString",   iv.value.ToString() },
                { "invalid",       true },
                { "invalidDetectTimeout", 60000.0 },
                { "isEvent",       false },
                { "alarmDisabled", false },
                { "alarmed",       false },
                { "alerted",       false },
                { "alertState",    "" },
                { "annotation",    "" },
                { "commandBlocked", false },
                { "commissioningRemarks", "" },
                { "formula",       0.0 },
                { "frozen",        false },
                { "frozenDetectTimeout", 0.0 },
                { "hiLimit",       double.MaxValue },
                { "hihiLimit",     double.MaxValue },
                { "hihihiLimit",   double.MaxValue },
                { "loLimit",      -double.MaxValue },
                { "loloLimit",    -double.MaxValue },
                { "lololoLimit",  -double.MaxValue },
                { "historianDeadBand", 0.0 },
                { "historianPeriod",   0.0 },
                { "hysteresis",        0.0 },
                { "location",      BsonNull.Value },
                { "notes",         "" },
                { "overflow",      false },
                { "parcels",       BsonNull.Value },
                { "priority",      0.0 },
                { "protocolDestinations", new BsonArray() },
                { "sourceDataUpdate",  BsonNull.Value },
                { "substituted",       false },
                { "timeTag",           BsonNull.Value },
                { "timeTagAlarm",      BsonNull.Value },
                { "timeTagAtSource",   BsonNull.Value },
                { "timeTagAtSourceOk", false },
                { "transient",         false },
                { "unit",              "" },
                { "updatesCnt",        0.0 },
                { "valueDefault",      0.0 },
                { "zeroDeadband",      0.0 }
            };
        }
    }
}
