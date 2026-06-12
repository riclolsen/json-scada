/*
 * DNP3 Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 */

#pragma once

#include <opendnp3/DNP3Manager.h>
#include <opendnp3/channel/PrintingChannelListener.h>
#include <opendnp3/logging/ILogHandler.h>
#include <opendnp3/logging/LogLevels.h>
#include <opendnp3/master/DefaultMasterApplication.h>
#include <opendnp3/master/IMasterApplication.h>
#include <opendnp3/master/ISOEHandler.h>
#include <opendnp3/master/MasterStackConfig.h>
#include <opendnp3/master/IMaster.h>
#include <opendnp3/master/ICommandTaskResult.h>
#include <opendnp3/master/ITaskCallback.h>
#include <opendnp3/gen/TaskCompletion.h>
#include <opendnp3/gen/MasterTaskType.h>
#include <opendnp3/app/ControlRelayOutputBlock.h>
#include <opendnp3/app/IINField.h>
#include <opendnp3/app/MeasurementTypes.h>
#include <opendnp3/app/OctetString.h>
#include <opendnp3/app/BinaryCommandEvent.h>
#include <opendnp3/app/AnalogCommandEvent.h>

#include <bsoncxx/builder/basic/document.hpp>
#include <mongocxx/client.hpp>
#include <mongocxx/instance.hpp>
#include <mongocxx/options/find.hpp>
#include <mongocxx/options/insert.hpp>
#include <mongocxx/pipeline.hpp>
#include <mongocxx/model/update_one.hpp>
#include <mongocxx/model/write.hpp>
#include <mongocxx/uri.hpp>

#include <algorithm>
#include <atomic>
#include <chrono>
#include <cmath>
#include <condition_variable>
#include <cstdint>
#include <cstdlib>
#include <deque>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <limits>
#include <memory>
#include <mutex>
#include <set>
#include <sstream>
#include <stdexcept>
#include <string>
#include <thread>
#include <utility>
#include <vector>

using namespace std;
using namespace opendnp3;
using bsoncxx::builder::basic::kvp;
using bsoncxx::builder::basic::make_document;

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

inline const string DriverMessage  = "{json:scada} DNP3 Client Driver (C++)";
inline const string ProtocolDriverName = "DNP3";
inline const string DriverVersion  = "0.1.3";
inline const string JsonConfigFilePath    = "../conf/json-scada.json";
inline const string JsonConfigFilePathAlt = "~/json-scada/conf/json-scada.json";
inline const string ProtocolConnectionsCollectionName       = "protocolConnections";
inline const string ProtocolDriverInstancesCollectionName   = "protocolDriverInstances";
inline const string RealtimeDataCollectionName  = "realtimeData";
inline const string CommandsQueueCollectionName = "commandsQueue";

constexpr uint32_t CROB_PulseOnTime  = 100;
constexpr uint32_t CROB_PulseOffTime = 100;
constexpr size_t   DataBufferLimit   = 10000;
constexpr double   AutoKeyMultiplier = 1000000.0;

// ---------------------------------------------------------------------------
// Logger
// ---------------------------------------------------------------------------

class Logger
{
public:
    enum class Level { NoLog = 0, Basic = 1, Detailed = 2, Debug = 3 };

    void setLevel(int value) { level = static_cast<Level>(value); }
    Level getLevel() const { return level; }

    void log(const string& msg, Level msgLevel = Level::Basic) const
    {
        if (msgLevel > level)
            return;
        lock_guard<mutex> guard(mtx);
        auto now = chrono::system_clock::now();
        auto tt  = chrono::system_clock::to_time_t(now);
        auto tm  = *localtime(&tt);
        auto ms  = chrono::duration_cast<chrono::milliseconds>(now.time_since_epoch()) % 1000;
        cout << "[" << put_time(&tm, "%Y-%m-%dT%H:%M:%S") << "."
             << setw(3) << setfill('0') << ms.count() << "] " << msg << endl;
    }

private:
    mutable mutex mtx;
    Level level = Level::Basic;
};

// ---------------------------------------------------------------------------
// Data structures
// ---------------------------------------------------------------------------

struct JSONSCADAConfig
{
    string nodeName;
    string mongoConnectionString;
    string mongoDatabaseName;
};

struct RangeScan
{
    int group = 1;
    int variation = 1;
    int startAddress = 0;
    int stopAddress = 0;
    int period = 0;
};

struct Dnp3Value
{
    int address = 0;
    int baseGroup = 0;
    int group = 0;
    int variation = 0;
    double value = 0.0;
    string valueString;
    int cot = 20;
    int64_t serverTimestamp = 0;
    bool hasSourceTimestamp = false;
    int64_t sourceTimestamp = 0;
    bool timeTagOk = false;
    bool qOnline = true;
    bool qCommLost = false;
    bool qRemoteForced = false;
    bool qLocalForced = false;
    bool qOverrange = false;
    bool qRollover = false;
    bool qReferenceError = false;
    bool qTransient = false;
    int connNumber = 0;
};

struct DNP3Connection
{
    int protocolDriverInstanceNumber = 1;
    int protocolConnectionNumber = 1;
    string name = "NO NAME";
    bool enabled = true;
    bool commandsEnabled = true;
    string connectionMode = "TCP ACTIVE";
    string ipAddressLocalBind;
    vector<string> ipAddresses;
    string portName;
    int baudRate = 9600;
    string parity = "None";
    string stopBits = "One";
    string handshake = "None";
    bool allowTLSv10 = false;
    bool allowTLSv11 = false;
    bool allowTLSv12 = true;
    bool allowTLSv13 = true;
    string cipherList;
    string localCertFilePath;
    string peerCertFilePath;
    string privateKeyFilePath;
    int localLinkAddress = 1;
    int remoteLinkAddress = 1;
    int giInterval = 300;
    int class0ScanInterval = 0;
    int class1ScanInterval = 0;
    int class2ScanInterval = 0;
    int class3ScanInterval = 0;
    vector<RangeScan> rangeScans;
    int timeSyncMode = 0;
    bool enableUnsolicited = true;
    bool autoCreateTags = false;
    // Touched only by the processMongo thread after initial preload in loadConnections():
    double lastNewKeyCreated = 0.0;
    set<pair<int, int>> insertedAddresses; // {commonAddress, objectAddress} already in realtimeData
    shared_ptr<IChannel> channel;
    shared_ptr<IMaster> master;
    shared_ptr<IMasterApplication> masterApplication;
    shared_ptr<ITaskCallback> scanTaskCallback;
    atomic<bool> isConnected{false};
};

// ---------------------------------------------------------------------------
// Global variables (defined in utils.cpp)
// ---------------------------------------------------------------------------

extern Logger Log;
extern mongocxx::instance MongoInstance;
extern int ProtocolDriverInstanceNumber;
extern atomic<bool> Active;
extern JSONSCADAConfig JSConfig;
extern vector<shared_ptr<DNP3Connection>> DNP3conns;
extern mutex ConnectionsMutex;
extern mutex QueueMutex;
extern condition_variable QueueCv;
extern deque<Dnp3Value> DNP3DataQueue;

// ---------------------------------------------------------------------------
// Utility functions (defined in utils.cpp)
// ---------------------------------------------------------------------------

int64_t nowMs();
string upper(string value);
string resolvePath(const string& path);
bool fileExists(const string& path);

double getDouble(const bsoncxx::document::view& doc, const string& key, double defaultValue = 0.0);
bool getBool(const bsoncxx::document::view& doc, const string& key, bool defaultValue = false);
string getString(const bsoncxx::document::view& doc, const string& key, const string& defaultValue = "");
int64_t getDateMs(const bsoncxx::document::view& doc, const string& key, int64_t defaultValue = 0);
vector<string> getStringArray(const bsoncxx::document::view& doc, const string& key);
vector<RangeScan> getRangeScans(const bsoncxx::document::view& doc);

JSONSCADAConfig loadJsonConfig(const string& path);
shared_ptr<mongocxx::client> connectMongoClient();
bool isMongoLive(mongocxx::database& db);

void enqueueValue(const Dnp3Value& value);
shared_ptr<DNP3Connection> findConnection(int number);
vector<shared_ptr<DNP3Connection>> snapshotConnections();

// ---------------------------------------------------------------------------
// Connection / protocol functions (defined in connection.cpp)
// ---------------------------------------------------------------------------

shared_ptr<IChannel> createChannel(const shared_ptr<DNP3Manager>& manager,
                                   const shared_ptr<DNP3Connection>& conn,
                                   LogLevels logLevel);
void configureMaster(const shared_ptr<DNP3Connection>& conn);
LogLevels mapLogLevel();
void loadConnections(bool applyInstanceLogLevel = true);

// ---------------------------------------------------------------------------
// Auto-tag creation helpers (defined in autocreate.cpp)
// ---------------------------------------------------------------------------

string dnp3TypeFromBaseGroup(int g);
string dnp3GroupDescription(int g);
double getNextAutoKey(const shared_ptr<DNP3Connection>& conn, mongocxx::collection& collection);
bsoncxx::document::value newRealtimeTagDoc(
    const Dnp3Value& iv,
    const string& connName,
    double id,
    bool isCommand,
    int srcCommonAddress,
    double asdu,
    double commandDuration,
    double commandOfSupervised,
    double supervisedOfCommand);

// ---------------------------------------------------------------------------
// MongoDB processing threads (defined in mongo.cpp)
// ---------------------------------------------------------------------------

void processMongo();
void processMongoCmd();
void processRedundancy();
