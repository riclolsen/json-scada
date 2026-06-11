/*
 * DNP3 Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2026 - Ricardo L. Olsen
 *
 * autocreate.cpp - Auto-create tag helpers: type classification, human labels,
 * key allocation, and BSON document factory for new realtimeData entries.
 */

#include "dnp3client.h"

string dnp3TypeFromBaseGroup(int g)
{
    switch (g)
    {
    case 1:   // Binary Input
    case 3:   // Double-bit Binary Input
    case 10:  // Binary Output Status
        return "digital";
    default:
        return "analog";
    }
}

string dnp3GroupDescription(int g)
{
    switch (g)
    {
    case 1:  return "Binary Input";
    case 3:  return "Double Binary Input";
    case 10: return "Binary Output Status";
    case 20: return "Counter";
    case 23: return "Frozen Counter";
    case 30: return "Analog Input";
    case 40: return "Analog Output Status";
    case 12: return "CROB Command";
    case 41: return "Analog Output Command";
    default: return "Group " + to_string(g);
    }
}

double getNextAutoKey(const shared_ptr<DNP3Connection>& conn, mongocxx::collection& collection)
{
    if (conn->lastNewKeyCreated == 0.0)
    {
        const double base = static_cast<double>(conn->protocolConnectionNumber) * AutoKeyMultiplier;
        const double top  = static_cast<double>(conn->protocolConnectionNumber + 1) * AutoKeyMultiplier;
        mongocxx::options::find opts;
        opts.sort(make_document(kvp("_id", -1)));
        opts.limit(1);
        auto res = collection.find_one(
            make_document(kvp("_id", make_document(kvp("$gt", base), kvp("$lt", top)))), opts);
        conn->lastNewKeyCreated = res ? getDouble(res->view(), "_id") + 1.0 : base;
    }
    else
    {
        conn->lastNewKeyCreated += 1.0;
    }
    return conn->lastNewKeyCreated;
}

bsoncxx::document::value newRealtimeTagDoc(
    const Dnp3Value& iv,
    const string& connName,
    double id,
    bool isCommand,
    int srcCommonAddress,
    double asdu,
    double commandDuration,
    double commandOfSupervised,
    double supervisedOfCommand)
{
    const string type     = dnp3TypeFromBaseGroup(srcCommonAddress);
    const string grpDesc  = dnp3GroupDescription(srcCommonAddress);
    const string addrStr  = to_string(iv.address);
    const string tag      = connName + ";" + to_string(srcCommonAddress) + ";" + addrStr;
    const string desc     = connName + "~" + grpDesc + "~" + addrStr + (isCommand ? "-Command" : "");
    const string ungrouped = grpDesc + " " + addrStr;
    const bool isDigital   = (type == "digital");

    using bsoncxx::types::b_null;

    auto protoDest = bsoncxx::builder::basic::array{};

    return make_document(
        kvp("_id",                            id),
        kvp("tag",                            tag),
        kvp("type",                           type),
        kvp("origin",                         isCommand ? "command" : "supervised"),
        kvp("description",                    desc),
        kvp("ungroupedDescription",           ungrouped),
        kvp("group1",                         connName),
        kvp("group2",                         grpDesc),
        kvp("group3",                         ""),
        kvp("protocolSourceConnectionNumber", static_cast<double>(iv.connNumber)),
        kvp("protocolSourceCommonAddress",    static_cast<double>(srcCommonAddress)),
        kvp("protocolSourceObjectAddress",    static_cast<double>(iv.address)),
        kvp("protocolSourceASDU",             asdu),
        kvp("protocolSourceCommandDuration",  commandDuration),
        kvp("protocolSourceCommandUseSBO",    false),
        kvp("commandOfSupervised",            commandOfSupervised),
        kvp("supervisedOfCommand",            supervisedOfCommand),
        kvp("kconv1",                         1.0),
        kvp("kconv2",                         0.0),
        kvp("alarmState",                     isDigital ? 2.0 : -1.0),
        kvp("stateTextFalse",                 isDigital ? "FALSE" : ""),
        kvp("stateTextTrue",                  isDigital ? "TRUE"  : ""),
        kvp("eventTextFalse",                 isDigital ? "FALSE" : ""),
        kvp("eventTextTrue",                  isDigital ? "TRUE"  : ""),
        kvp("value",                          isCommand ? 0.0 : iv.value),
        kvp("valueString",                    isCommand ? "" : iv.valueString),
        kvp("invalid",                        true),
        kvp("invalidDetectTimeout",           60000.0),
        kvp("isEvent",                        false),
        kvp("alarmDisabled",                  false),
        kvp("alarmed",                        false),
        kvp("alerted",                        false),
        kvp("alertState",                     ""),
        kvp("annotation",                     ""),
        kvp("commandBlocked",                 false),
        kvp("commissioningRemarks",           ""),
        kvp("formula",                        0.0),
        kvp("frozen",                         false),
        kvp("frozenDetectTimeout",            0.0),
        kvp("hiLimit",                        numeric_limits<double>::max()),
        kvp("hihiLimit",                      numeric_limits<double>::max()),
        kvp("hihihiLimit",                    numeric_limits<double>::max()),
        kvp("historianDeadBand",              0.0),
        kvp("historianPeriod",                0.0),
        kvp("hysteresis",                     0.0),
        kvp("loLimit",                        -numeric_limits<double>::max()),
        kvp("loloLimit",                      -numeric_limits<double>::max()),
        kvp("lololoLimit",                    -numeric_limits<double>::max()),
        kvp("location",                       b_null{}),
        kvp("notes",                          ""),
        kvp("overflow",                       false),
        kvp("parcels",                        b_null{}),
        kvp("priority",                       0.0),
        kvp("protocolDestinations",           protoDest),
        kvp("sourceDataUpdate",               b_null{}),
        kvp("substituted",                    false),
        kvp("timeTag",                        b_null{}),
        kvp("timeTagAlarm",                   b_null{}),
        kvp("timeTagAtSource",                b_null{}),
        kvp("timeTagAtSourceOk",              false),
        kvp("transient",                      false),
        kvp("unit",                           ""),
        kvp("updatesCnt",                     0.0),
        kvp("valueDefault",                   0.0),
        kvp("zeroDeadband",                   0.0));
}
