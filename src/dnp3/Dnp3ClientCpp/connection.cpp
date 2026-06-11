/*
 * DNP3 Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2026 - Ricardo L. Olsen
 *
 * connection.cpp - Channel/master lifecycle: ChannelListener, MasterApplication,
 * ScanTaskCallback, channel factory, master configuration, and connection loading.
 */

#include "soe_handler.h"  // pulls in dnp3client.h; SOEHandler used in configureMaster

// ---------------------------------------------------------------------------
// Local helpers
// ---------------------------------------------------------------------------

static pair<string, uint16_t> parseEndpoint(const string& text, uint16_t defaultPort = 20000)
{
    auto pos = text.find(':');
    if (pos == string::npos)
        return {text, defaultPort};
    return {text.substr(0, pos), static_cast<uint16_t>(stoi(text.substr(pos + 1)))};
}

static string taskCompletionToString(TaskCompletion result)
{
    switch (result)
    {
    case TaskCompletion::SUCCESS:                   return "SUCCESS";
    case TaskCompletion::FAILURE_BAD_RESPONSE:      return "FAILURE_BAD_RESPONSE";
    case TaskCompletion::FAILURE_RESPONSE_TIMEOUT:  return "FAILURE_RESPONSE_TIMEOUT";
    case TaskCompletion::FAILURE_START_TIMEOUT:     return "FAILURE_START_TIMEOUT";
    case TaskCompletion::FAILURE_NO_COMMS:          return "FAILURE_NO_COMMS";
    case TaskCompletion::FAILURE_MESSAGE_FORMAT_ERROR: return "FAILURE_MESSAGE_FORMAT_ERROR";
    default: return "UNKNOWN(" + to_string(static_cast<int>(result)) + ")";
    }
}

static string masterTaskTypeToString(MasterTaskType type)
{
    switch (type)
    {
    case MasterTaskType::CLEAR_RESTART:           return "CLEAR_RESTART";
    case MasterTaskType::DISABLE_UNSOLICITED:     return "DISABLE_UNSOLICITED";
    case MasterTaskType::ASSIGN_CLASS:            return "ASSIGN_CLASS";
    case MasterTaskType::STARTUP_INTEGRITY_POLL:  return "STARTUP_INTEGRITY_POLL";
    case MasterTaskType::NON_LAN_TIME_SYNC:       return "NON_LAN_TIME_SYNC";
    case MasterTaskType::LAN_TIME_SYNC:           return "LAN_TIME_SYNC";
    case MasterTaskType::ENABLE_UNSOLICITED:      return "ENABLE_UNSOLICITED";
    case MasterTaskType::AUTO_EVENT_SCAN:         return "AUTO_EVENT_SCAN";
    case MasterTaskType::USER_TASK:               return "USER_TASK";
    default: return "UNKNOWN(" + to_string(static_cast<int>(type)) + ")";
    }
}

static string formatIIN(const IINField& iin)
{
    ostringstream ss;
    ss << "LSB=0x" << hex << setw(2) << setfill('0') << static_cast<int>(iin.LSB)
       << " MSB=0x" << setw(2) << static_cast<int>(iin.MSB) << dec;
    return ss.str();
}

// ---------------------------------------------------------------------------
// ChannelListener
// ---------------------------------------------------------------------------

class ChannelListener final : public IChannelListener
{
public:
    explicit ChannelListener(shared_ptr<DNP3Connection> conn) : conn(std::move(conn)) {}

    void OnStateChange(ChannelState state) override
    {
        conn->isConnected = state == ChannelState::OPEN;
        Log.log(conn->name + " - Channel state changed.");
        if (state != ChannelState::CLOSED)
            return;
        try
        {
            auto client = connectMongoClient();
            auto db = (*client)[JSConfig.mongoDatabaseName];
            auto collection = db[RealtimeDataCollectionName];
            auto filter = make_document(kvp("protocolSourceConnectionNumber", conn->protocolConnectionNumber));
            auto update  = make_document(kvp("$set", make_document(
                kvp("invalid", true),
                kvp("timeTag", bsoncxx::types::b_date(chrono::milliseconds(nowMs()))))));
            collection.update_many(filter.view(), update.view());
        }
        catch (const exception& ex)
        {
            Log.log(conn->name + " - Failed to invalidate points: " + string(ex.what()),
                Logger::Level::Detailed);
        }
    }

private:
    shared_ptr<DNP3Connection> conn;
};

// ---------------------------------------------------------------------------
// MasterApplication
// ---------------------------------------------------------------------------

class MasterApplication final : public IMasterApplication
{
public:
    explicit MasterApplication(shared_ptr<DNP3Connection> conn) : conn(std::move(conn)) {}

    void OnReceiveIIN(const IINField& iin) override
    {
        Log.log(conn->name + " - Received IIN: " + formatIIN(iin), Logger::Level::Detailed);
    }

    void OnTaskStart(MasterTaskType type, TaskId) override
    {
        Log.log(conn->name + " - Task start: " + masterTaskTypeToString(type), Logger::Level::Detailed);
    }

    void OnTaskComplete(const TaskInfo& info) override
    {
        Log.log(conn->name + " - Task complete: " + masterTaskTypeToString(info.type)
            + " result=" + taskCompletionToString(info.result), Logger::Level::Detailed);
    }

    void OnOpen() override
    {
        Log.log(conn->name + " - Application layer opened", Logger::Level::Detailed);
    }

    void OnClose() override
    {
        Log.log(conn->name + " - Application layer closed", Logger::Level::Detailed);
    }

    bool AssignClassDuringStartup() override { return false; }
    void ConfigureAssignClassRequest(const WriteHeaderFunT&) override {}

    UTCTimestamp Now() override { return UTCTimestamp(nowMs()); }

    void OnStateChange(LinkStatus value) override
    {
        Log.log(conn->name + " - Link state change: " + to_string(static_cast<int>(value)),
            Logger::Level::Detailed);
    }

private:
    shared_ptr<DNP3Connection> conn;
};

// ---------------------------------------------------------------------------
// ScanTaskCallback
// ---------------------------------------------------------------------------

class ScanTaskCallback final : public ITaskCallback
{
public:
    explicit ScanTaskCallback(shared_ptr<DNP3Connection> conn) : conn(std::move(conn)) {}

    void OnStart() override
    {
        Log.log(conn->name + " - Scan task callback start", Logger::Level::Detailed);
    }

    void OnComplete(TaskCompletion result) override
    {
        Log.log(conn->name + " - Scan task callback complete: " + taskCompletionToString(result),
            Logger::Level::Detailed);
    }

    void OnDestroyed() override
    {
        Log.log(conn->name + " - Scan task callback destroyed", Logger::Level::Detailed);
    }

private:
    shared_ptr<DNP3Connection> conn;
};

// ---------------------------------------------------------------------------
// Channel factory
// ---------------------------------------------------------------------------

static shared_ptr<IChannel> tryReuseChannel(const shared_ptr<DNP3Connection>& conn)
{
    for (const auto& existing : snapshotConnections())
    {
        if (existing.get() == conn.get() || !existing->channel)
            continue;
        if ((conn->connectionMode == "TCP ACTIVE" || conn->connectionMode == "TLS ACTIVE")
            && existing->ipAddresses == conn->ipAddresses)
            return existing->channel;
        if ((conn->connectionMode == "TCP PASSIVE" || conn->connectionMode == "TLS PASSIVE")
            && existing->ipAddressLocalBind == conn->ipAddressLocalBind)
            return existing->channel;
        if (conn->connectionMode == "SERIAL" && !conn->portName.empty()
            && conn->portName == existing->portName)
            return existing->channel;
        if (conn->connectionMode == "UDP"
            && conn->ipAddressLocalBind == existing->ipAddressLocalBind
            && conn->ipAddresses == existing->ipAddresses)
            return existing->channel;
    }
    return {};
}

shared_ptr<IChannel> createChannel(const shared_ptr<DNP3Manager>& manager,
                                   const shared_ptr<DNP3Connection>& conn,
                                   LogLevels logLevel)
{
    if (auto reused = tryReuseChannel(conn))
        return reused;

    auto listener = make_shared<ChannelListener>(conn);

    if (conn->connectionMode == "TCP ACTIVE")
    {
        auto remote   = parseEndpoint(conn->ipAddresses.front());
        auto bindHost = conn->ipAddressLocalBind.empty()
            ? string("0.0.0.0") : parseEndpoint(conn->ipAddressLocalBind).first;
        return manager->AddTCPClient(conn->name, logLevel, ChannelRetry::Default(),
            vector<IPEndpoint>{IPEndpoint(remote.first, remote.second)}, bindHost, listener);
    }
    if (conn->connectionMode == "TCP PASSIVE")
    {
        auto local = parseEndpoint(conn->ipAddressLocalBind.empty()
            ? string("0.0.0.0:20000") : conn->ipAddressLocalBind);
        return manager->AddTCPServer(conn->name, logLevel, ServerAcceptMode::CloseNew,
            IPEndpoint(local.first, local.second), listener);
    }
    if (conn->connectionMode == "TLS ACTIVE")
    {
        auto remote   = parseEndpoint(conn->ipAddresses.front());
        auto bindHost = conn->ipAddressLocalBind.empty()
            ? string("0.0.0.0") : parseEndpoint(conn->ipAddressLocalBind).first;
        auto tlsConfig = TLSConfig(conn->peerCertFilePath, conn->localCertFilePath,
            conn->privateKeyFilePath,
            conn->allowTLSv10, conn->allowTLSv11, conn->allowTLSv12, conn->allowTLSv13,
            conn->cipherList);
        return manager->AddTLSClient(conn->name, logLevel, ChannelRetry::Default(),
            vector<IPEndpoint>{IPEndpoint(remote.first, remote.second)}, bindHost, tlsConfig, listener);
    }
    if (conn->connectionMode == "TLS PASSIVE")
    {
        auto local = parseEndpoint(conn->ipAddressLocalBind.empty()
            ? string("0.0.0.0:20000") : conn->ipAddressLocalBind);
        auto tlsConfig = TLSConfig(conn->peerCertFilePath, conn->localCertFilePath,
            conn->privateKeyFilePath,
            conn->allowTLSv10, conn->allowTLSv11, conn->allowTLSv12, conn->allowTLSv13,
            conn->cipherList);
        return manager->AddTLSServer(conn->name, logLevel, ServerAcceptMode::CloseNew,
            IPEndpoint(local.first, local.second), tlsConfig, listener);
    }
    if (conn->connectionMode == "SERIAL")
    {
        SerialSettings settings;
        settings.deviceName = conn->portName;
        settings.baud       = conn->baudRate;
        settings.dataBits   = 8;
        settings.parity     = conn->parity == "Even" ? Parity::Even
                            : conn->parity == "Odd"  ? Parity::Odd
                            : Parity::None;
        settings.stopBits   = (conn->stopBits == "Two" || conn->stopBits == "2")
                            ? StopBits::Two : StopBits::One;
        settings.flowType   = conn->handshake == "XON"  ? FlowControl::XONXOFF
                            : conn->handshake == "RTS"  ? FlowControl::Hardware
                            : FlowControl::None;
        return manager->AddSerial(conn->name, logLevel, ChannelRetry::Default(), settings, listener);
    }
    if (conn->connectionMode == "UDP")
    {
        auto local  = parseEndpoint(conn->ipAddressLocalBind);
        auto remote = parseEndpoint(conn->ipAddresses.front());
        return manager->AddUDPChannel(conn->name, logLevel, ChannelRetry::Default(),
            IPEndpoint(local.first, local.second),
            IPEndpoint(remote.first, remote.second), listener);
    }
    throw runtime_error("Unsupported connectionMode: " + conn->connectionMode);
}

// ---------------------------------------------------------------------------
// Master configuration
// ---------------------------------------------------------------------------

void configureMaster(const shared_ptr<DNP3Connection>& conn)
{
    MasterStackConfig config;
    config.link.LocalAddr  = static_cast<uint16_t>(conn->localLinkAddress);
    config.link.RemoteAddr = static_cast<uint16_t>(conn->remoteLinkAddress);
    config.master.startupIntegrityClassMask = ClassField::AllClasses();
    config.master.timeSyncMode = TimeSyncMode::None;
    if (conn->timeSyncMode >= 2)
        config.master.timeSyncMode = TimeSyncMode::LAN;
    else if (conn->timeSyncMode == 1)
        config.master.timeSyncMode = TimeSyncMode::NonLAN;

    if (conn->enableUnsolicited)
    {
        config.master.disableUnsolOnStartup = false;
        config.master.unsolClassMask = ClassField::AllClasses();
    }
    else
    {
        config.master.disableUnsolOnStartup = true;
        config.master.unsolClassMask = ClassField::None();
    }

    auto soe = make_shared<SOEHandler>(conn);
    conn->masterApplication = make_shared<MasterApplication>(conn);
    conn->scanTaskCallback  = make_shared<ScanTaskCallback>(conn);
    auto scanConfig = TaskConfig::With(conn->scanTaskCallback);
    conn->master = conn->channel->AddMaster(conn->name, soe, conn->masterApplication, config);

    if (conn->giInterval > 0)
        conn->master->AddClassScan(ClassField::AllClasses(),
            TimeDuration::Seconds(conn->giInterval), soe, scanConfig);
    if (conn->class0ScanInterval > 0)
        conn->master->AddClassScan(ClassField(PointClass::Class0),
            TimeDuration::Seconds(conn->class0ScanInterval), soe, scanConfig);
    if (conn->class1ScanInterval > 0)
        conn->master->AddClassScan(ClassField(PointClass::Class1),
            TimeDuration::Seconds(conn->class1ScanInterval), soe, scanConfig);
    if (conn->class2ScanInterval > 0)
        conn->master->AddClassScan(ClassField(PointClass::Class2),
            TimeDuration::Seconds(conn->class2ScanInterval), soe, scanConfig);
    if (conn->class3ScanInterval > 0)
        conn->master->AddClassScan(ClassField(PointClass::Class3),
            TimeDuration::Seconds(conn->class3ScanInterval), soe, scanConfig);

    for (const auto& scan : conn->rangeScans)
    {
        if (scan.period > 0)
            conn->master->AddRangeScan(GroupVariationID(scan.group, scan.variation),
                static_cast<uint16_t>(scan.startAddress),
                static_cast<uint16_t>(scan.stopAddress),
                TimeDuration::Seconds(scan.period), soe, scanConfig);
    }

    conn->master->Disable();
    conn->isConnected = false;
    Log.log(conn->name + " - Master created in disabled state; waiting for redundancy activation.",
        Logger::Level::Detailed);
}

// ---------------------------------------------------------------------------
// Log level mapping
// ---------------------------------------------------------------------------

LogLevels mapLogLevel()
{
    if (Log.getLevel() >= Logger::Level::Debug)
        return levels::ALL;
    if (Log.getLevel() >= Logger::Level::Detailed)
        return levels::NORMAL | levels::ALL_COMMS;
    if (Log.getLevel() >= Logger::Level::Basic)
        return levels::NORMAL;
    return levels::NOTHING;
}

// ---------------------------------------------------------------------------
// Load connections from MongoDB
// ---------------------------------------------------------------------------

void loadConnections(bool applyInstanceLogLevel)
{
    auto client      = connectMongoClient();
    auto db          = (*client)[JSConfig.mongoDatabaseName];
    auto instances   = db[ProtocolDriverInstancesCollectionName];
    auto connections = db[ProtocolConnectionsCollectionName];

    auto instance = instances.find_one(make_document(
        kvp("protocolDriver", ProtocolDriverName),
        kvp("protocolDriverInstanceNumber", ProtocolDriverInstanceNumber),
        kvp("enabled", true)));
    if (!instance)
        throw runtime_error("Driver instance not found");
    if (applyInstanceLogLevel && instance->view()["logLevel"])
        Log.setLevel(static_cast<int>(getDouble(instance->view(), "logLevel", 1)));

    vector<shared_ptr<DNP3Connection>> loaded;
    auto cursor = connections.find(make_document(
        kvp("protocolDriver", ProtocolDriverName),
        kvp("protocolDriverInstanceNumber", ProtocolDriverInstanceNumber),
        kvp("enabled", true)));

    for (auto&& doc : cursor)
    {
        auto conn = make_shared<DNP3Connection>();
        conn->protocolDriverInstanceNumber = static_cast<int>(getDouble(doc, "protocolDriverInstanceNumber", 1));
        conn->protocolConnectionNumber     = static_cast<int>(getDouble(doc, "protocolConnectionNumber", 1));
        conn->name                         = getString(doc, "name", "NO NAME");
        conn->enabled                      = getBool(doc, "enabled", true);
        conn->commandsEnabled              = getBool(doc, "commandsEnabled", true);
        conn->connectionMode               = upper(getString(doc, "connectionMode", "TCP ACTIVE"));
        conn->ipAddressLocalBind           = getString(doc, "ipAddressLocalBind");
        conn->ipAddresses                  = getStringArray(doc, "ipAddresses");
        conn->portName                     = getString(doc, "portName");
        conn->baudRate                     = static_cast<int>(getDouble(doc, "baudRate", 9600));
        conn->parity                       = getString(doc, "parity", "None");
        conn->stopBits                     = getString(doc, "stopBits", "One");
        conn->handshake                    = getString(doc, "handshake", "None");
        conn->allowTLSv10                  = getBool(doc, "allowTLSv10", false);
        conn->allowTLSv11                  = getBool(doc, "allowTLSv11", false);
        conn->allowTLSv12                  = getBool(doc, "allowTLSv12", true);
        conn->allowTLSv13                  = getBool(doc, "allowTLSv13", true);
        conn->cipherList                   = getString(doc, "cipherList");
        conn->localCertFilePath            = getString(doc, "localCertFilePath");
        conn->peerCertFilePath             = getString(doc, "peerCertFilePath");
        conn->privateKeyFilePath           = getString(doc, "privateKeyFilePath");
        conn->localLinkAddress             = static_cast<int>(getDouble(doc, "localLinkAddress", 1));
        conn->remoteLinkAddress            = static_cast<int>(getDouble(doc, "remoteLinkAddress", 1));
        conn->giInterval                   = static_cast<int>(getDouble(doc, "giInterval", 300));
        conn->class0ScanInterval           = static_cast<int>(getDouble(doc, "class0ScanInterval", 0));
        conn->class1ScanInterval           = static_cast<int>(getDouble(doc, "class1ScanInterval", 0));
        conn->class2ScanInterval           = static_cast<int>(getDouble(doc, "class2ScanInterval", 0));
        conn->class3ScanInterval           = static_cast<int>(getDouble(doc, "class3ScanInterval", 0));
        conn->rangeScans                   = getRangeScans(doc);
        conn->timeSyncMode                 = static_cast<int>(getDouble(doc, "timeSyncMode", 0));
        conn->enableUnsolicited            = getBool(doc, "enableUnsolicited", true);
        conn->autoCreateTags               = getBool(doc, "autoCreateTags", false);
        loaded.push_back(conn);
    }
    if (loaded.empty())
        throw runtime_error("No DNP3 connections found");

    // Pre-populate insertedAddresses so existing tags are not re-created on restart.
    auto realtimeData = db[RealtimeDataCollectionName];
    for (const auto& conn : loaded)
    {
        if (!conn->autoCreateTags)
            continue;
        mongocxx::options::find opts;
        opts.projection(make_document(
            kvp("protocolSourceCommonAddress", 1),
            kvp("protocolSourceObjectAddress", 1)));
        auto tagCursor = realtimeData.find(
            make_document(kvp("protocolSourceConnectionNumber", conn->protocolConnectionNumber)), opts);
        size_t count = 0;
        for (auto&& tagDoc : tagCursor)
        {
            conn->insertedAddresses.insert({
                static_cast<int>(getDouble(tagDoc, "protocolSourceCommonAddress", -1)),
                static_cast<int>(getDouble(tagDoc, "protocolSourceObjectAddress", -1))});
            ++count;
        }
        Log.log(conn->name + " - Found " + to_string(count) + " tags in database.");
    }

    lock_guard<mutex> guard(ConnectionsMutex);
    DNP3conns = std::move(loaded);
}
