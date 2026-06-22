/*
 * IEC 60870-5-104 Server Protocol driver for {json:scada}
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
using System.Collections.Generic;
using MongoDB.Bson;
using MongoDB.Driver;

namespace Iec10XDriver
{
    partial class MainClass
    {
        // IOA partition — one shared address space per CA, split into non-overlapping ranges.
        // Ranges fit both 2-byte (≤65535) and 3-byte (≤16777215) IOAs.
        const int IoaBaseDigital = 1;     const int IoaTopDigital = 20000;  // ASDU  1 (M_SP_NA_1)
        const int IoaBaseAnalog  = 20001; const int IoaTopAnalog  = 40000;  // ASDU 13 (M_ME_NC_1)
        const int IoaBaseDigCmd  = 40001; const int IoaTopDigCmd  = 50000;  // ASDU 45 (C_SC_NA_1)
        const int IoaBaseAnaCmd  = 50001; const int IoaTopAnaCmd  = 60000;  // ASDU 50 (C_SE_NC_1)

        // Runs once at startup per connection with autoCreateTags=true.
        // Pushes a protocolDestinations entry onto every existing realtimeData tag not yet mapped
        // to this connection, allocating sequential IOAs within the per-category ranges.
        static void DistributeAutoTags(IMongoDatabase DB)
        {
            var colRt = DB.GetCollection<BsonDocument>(RealtimeDataCollectionName);

            foreach (var srv in IEC10Xconns)
            {
                if (!srv.autoCreateTags) continue;

                if (srv.sizeOfIOA < 2)
                {
                    Log(srv.name + " - autoCreateTags not supported with sizeOfIOA=1, skipping.");
                    continue;
                }
                Log(srv.name + " - autoCreateTags: distributing protocol destinations...");

                // Detect highest IOA already in use for this connection, bucketed by range.
                int lastDigital = IoaBaseDigital - 1;
                int lastAnalog  = IoaBaseAnalog  - 1;
                int lastDigCmd  = IoaBaseDigCmd  - 1;
                int lastAnaCmd  = IoaBaseAnaCmd  - 1;

                var existingFilter = Builders<BsonDocument>.Filter.ElemMatch<BsonDocument>(
                    "protocolDestinations",
                    new BsonDocument("protocolDestinationConnectionNumber", (double)srv.protocolConnectionNumber));
                var existingDocs = colRt.Find(existingFilter)
                    .Project(Builders<BsonDocument>.Projection.Include("protocolDestinations"))
                    .ToList();
                foreach (var doc in existingDocs)
                {
                    if (!doc.Contains("protocolDestinations")) continue;
                    foreach (var dest in doc["protocolDestinations"].AsBsonArray)
                    {
                        var d = dest.AsBsonDocument;
                        if (d.GetValue("protocolDestinationConnectionNumber", -1.0).ToDouble()
                                != srv.protocolConnectionNumber) continue;
                        int ioa = (int)d.GetValue("protocolDestinationObjectAddress", 0.0).ToDouble();
                        if (ioa >= IoaBaseDigital && ioa <= IoaTopDigital && ioa > lastDigital) lastDigital = ioa;
                        if (ioa >= IoaBaseAnalog  && ioa <= IoaTopAnalog  && ioa > lastAnalog)  lastAnalog  = ioa;
                        if (ioa >= IoaBaseDigCmd  && ioa <= IoaTopDigCmd  && ioa > lastDigCmd)  lastDigCmd  = ioa;
                        if (ioa >= IoaBaseAnaCmd  && ioa <= IoaTopAnaCmd  && ioa > lastAnaCmd)  lastAnaCmd  = ioa;
                    }
                }

                // Process four categories: commands first (when enabled), then supervised.
                var categories = new (string type, string origin, int asdu, int startIoa, int topIoa)[]
                {
                    ("digital", "command",    45, lastDigCmd,  IoaTopDigCmd),
                    ("analog",  "command",    50, lastAnaCmd,  IoaTopAnaCmd),
                    ("digital", "supervised",  1, lastDigital, IoaTopDigital),
                    ("analog",  "supervised", 13, lastAnalog,  IoaTopAnalog),
                };

                foreach (var cat in categories)
                {
                    if (cat.origin == "command" && !srv.commandsEnabled) continue;

                    int nextIoa = cat.startIoa;
                    var filter = Builders<BsonDocument>.Filter.And(
                        Builders<BsonDocument>.Filter.Eq("type",   cat.type),
                        Builders<BsonDocument>.Filter.Eq("origin", cat.origin),
                        Builders<BsonDocument>.Filter.Not(
                            Builders<BsonDocument>.Filter.ElemMatch<BsonDocument>(
                                "protocolDestinations",
                                new BsonDocument("protocolDestinationConnectionNumber",
                                    (double)srv.protocolConnectionNumber))));
                    var sort = Builders<BsonDocument>.Sort.Ascending("_id");
                    var projection = Builders<BsonDocument>.Projection
                        .Include("_id").Include("tag").Include("group1");
                    var tags = colRt.Find(filter).Sort(sort).Project(projection).ToList();

                    bool exhausted = false;
                    foreach (var tag in tags)
                    {
                        if (exhausted) break;

                        // Optional topics filter: skip tags whose group1 does not contain any topic substring
                        if (srv.topics != null && srv.topics.Length > 0)
                        {
                            string g1 = tag.GetValue("group1", "").AsString;
                            bool matched = false;
                            foreach (var topic in srv.topics)
                                if (g1.Contains(topic)) { matched = true; break; }
                            if (!matched) continue;
                        }

                        nextIoa++;
                        if (nextIoa > cat.topIoa)
                        {
                            Log(srv.name + " - autoCreateTags: IOA range exhausted for "
                                + cat.type + " " + cat.origin + ", stopping.");
                            exhausted = true;
                            break;
                        }

                        double tagId  = tag["_id"].ToDouble();
                        string tagStr = tag.GetValue("tag", "").AsString;

                        // Ensure the protocolDestinations array field exists before pushing
                        colRt.UpdateOne(
                            Builders<BsonDocument>.Filter.And(
                                Builders<BsonDocument>.Filter.Eq("_id", tag["_id"]),
                                Builders<BsonDocument>.Filter.Exists("protocolDestinations", false)),
                            new BsonDocument("$set",
                                new BsonDocument("protocolDestinations", new BsonArray())));

                        var destDoc = new BsonDocument
                        {
                            { "protocolDestinationConnectionNumber", (double)srv.protocolConnectionNumber },
                            { "protocolDestinationCommonAddress",    (double)srv.autoCreateTagsCommonAddress },
                            { "protocolDestinationObjectAddress",    (double)nextIoa },
                            { "protocolDestinationASDU",            (double)cat.asdu },
                            { "protocolDestinationCommandDuration",  0.0 },
                            { "protocolDestinationCommandUseSBO",    false },
                            { "protocolDestinationKConv1",          1.0 },
                            { "protocolDestinationKConv2",          0.0 },
                            { "protocolDestinationGroup",           0.0 },
                            { "protocolDestinationHoursShift",      0.0 }
                        };

                        colRt.UpdateOne(
                            Builders<BsonDocument>.Filter.Eq("_id", tag["_id"]),
                            new BsonDocument("$push",
                                new BsonDocument("protocolDestinations", destDoc)));

                        Log(srv.name + " - autoCreateTags: Creating destination for tag: "
                            + tagId + " " + tagStr + " IOA: " + nextIoa);
                    }
                }
                Log(srv.name + " - autoCreateTags: Distribution complete.");
            }
        }
    }
}
