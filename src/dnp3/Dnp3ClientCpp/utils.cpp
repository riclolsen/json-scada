/*
 * DNP3 Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2026 - Ricardo L. Olsen
 *
 * utils.cpp - Global variable definitions and utility / BSON helper functions.
 */

#include "dnp3client.h"
#include "../Dnp3Server/json.hpp"

using json = nlohmann::json;

// ---------------------------------------------------------------------------
// Global variable definitions
// ---------------------------------------------------------------------------

Logger Log;
mongocxx::instance MongoInstance{};
int ProtocolDriverInstanceNumber = 1;
atomic<bool> Active{false};
JSONSCADAConfig JSConfig;
vector<shared_ptr<DNP3Connection>> DNP3conns;
mutex ConnectionsMutex;
mutex QueueMutex;
condition_variable QueueCv;
deque<Dnp3Value> DNP3DataQueue;

// ---------------------------------------------------------------------------
// Time helpers
// ---------------------------------------------------------------------------

int64_t nowMs()
{
    return chrono::duration_cast<chrono::milliseconds>(
        chrono::system_clock::now().time_since_epoch()).count();
}

// ---------------------------------------------------------------------------
// String helpers
// ---------------------------------------------------------------------------

string upper(string value)
{
    transform(value.begin(), value.end(), value.begin(),
        [](unsigned char c) { return static_cast<char>(toupper(c)); });
    return value;
}

string resolvePath(const string& path)
{
    if (path.rfind("~/", 0) == 0)
    {
        const char* home = getenv("HOME");
        if (!home)
            home = getenv("USERPROFILE");
        if (home)
            return string(home) + path.substr(1);
    }
    return path;
}

bool fileExists(const string& path)
{
    ifstream file(path);
    return file.good();
}

// ---------------------------------------------------------------------------
// BSON document accessors
// ---------------------------------------------------------------------------

double getDouble(const bsoncxx::document::view& doc, const string& key, double defaultValue)
{
    auto value = doc[key];
    if (!value)
        return defaultValue;
    switch (value.type())
    {
    case bsoncxx::type::k_double: return value.get_double().value;
    case bsoncxx::type::k_int32:  return value.get_int32().value;
    case bsoncxx::type::k_int64:  return static_cast<double>(value.get_int64().value);
    case bsoncxx::type::k_bool:   return value.get_bool().value ? 1.0 : 0.0;
    default:                      return defaultValue;
    }
}

bool getBool(const bsoncxx::document::view& doc, const string& key, bool defaultValue)
{
    auto value = doc[key];
    if (!value)
        return defaultValue;
    if (value.type() == bsoncxx::type::k_bool)
        return value.get_bool().value;
    return getDouble(doc, key, defaultValue ? 1 : 0) != 0;
}

string getString(const bsoncxx::document::view& doc, const string& key, const string& defaultValue)
{
    auto value = doc[key];
    if (!value)
        return defaultValue;
    if (value.type() == bsoncxx::type::k_string)
        return string(value.get_string().value);
    return defaultValue;
}

int64_t getDateMs(const bsoncxx::document::view& doc, const string& key, int64_t defaultValue)
{
    auto value = doc[key];
    if (!value)
        return defaultValue;
    if (value.type() == bsoncxx::type::k_date)
        return value.get_date().value.count();
    if (value.type() == bsoncxx::type::k_int64)
        return value.get_int64().value;
    if (value.type() == bsoncxx::type::k_int32)
        return value.get_int32().value;
    return defaultValue;
}

vector<string> getStringArray(const bsoncxx::document::view& doc, const string& key)
{
    vector<string> result;
    auto value = doc[key];
    if (!value || value.type() != bsoncxx::type::k_array)
        return result;
    for (auto&& item : value.get_array().value)
        if (item.type() == bsoncxx::type::k_string)
            result.emplace_back(string(item.get_string().value));
    return result;
}

vector<RangeScan> getRangeScans(const bsoncxx::document::view& doc)
{
    vector<RangeScan> scans;
    auto value = doc["rangeScans"];
    if (!value || value.type() != bsoncxx::type::k_array)
        return scans;
    for (auto&& item : value.get_array().value)
    {
        auto v = item.get_document().view();
        RangeScan scan;
        scan.group        = static_cast<int>(getDouble(v, "group", 1));
        scan.variation    = static_cast<int>(getDouble(v, "variation", 1));
        scan.startAddress = static_cast<int>(getDouble(v, "startAddress", 0));
        scan.stopAddress  = static_cast<int>(getDouble(v, "stopAddress", 0));
        scan.period       = static_cast<int>(getDouble(v, "period", 0));
        scans.push_back(scan);
    }
    return scans;
}

// ---------------------------------------------------------------------------
// Config / MongoDB helpers
// ---------------------------------------------------------------------------

JSONSCADAConfig loadJsonConfig(const string& path)
{
    ifstream file(path);
    if (!file.is_open())
        throw runtime_error("Missing config file " + path);
    auto j = json::parse(file);
    return {j.value("nodeName", ""),
            j.value("mongoConnectionString", ""),
            j.value("mongoDatabaseName", "")};
}

shared_ptr<mongocxx::client> connectMongoClient()
{
    return make_shared<mongocxx::client>(mongocxx::uri(JSConfig.mongoConnectionString));
}

bool isMongoLive(mongocxx::database& db)
{
    try
    {
        db.run_command(make_document(kvp("ping", 1)));
        return true;
    }
    catch (const exception&)
    {
        return false;
    }
}

// ---------------------------------------------------------------------------
// Queue / connection helpers
// ---------------------------------------------------------------------------

void enqueueValue(const Dnp3Value& value)
{
    lock_guard<mutex> guard(QueueMutex);
    if (DNP3DataQueue.size() >= DataBufferLimit)
        DNP3DataQueue.pop_front();
    DNP3DataQueue.push_back(value);
    QueueCv.notify_one();
}

shared_ptr<DNP3Connection> findConnection(int number)
{
    lock_guard<mutex> guard(ConnectionsMutex);
    for (const auto& conn : DNP3conns)
        if (conn->protocolConnectionNumber == number)
            return conn;
    return {};
}

vector<shared_ptr<DNP3Connection>> snapshotConnections()
{
    lock_guard<mutex> guard(ConnectionsMutex);
    return DNP3conns;
}
