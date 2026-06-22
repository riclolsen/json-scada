/*
 * DNP3 Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2026 - Ricardo L. Olsen
 *
 * mongo.cpp - Three long-running threads:
 *   processMongo()       - dequeue Dnp3Value items, auto-create tags, bulk-write updates
 *   processMongoCmd()    - watch commandsQueue change stream and issue DNP3 commands
 *   processRedundancy()  - manage active/standby state and write keep-alive heartbeat
 */

#include "dnp3client.h"

// ---------------------------------------------------------------------------
// Command helpers (file-local)
// ---------------------------------------------------------------------------

static void cancelCommand(mongocxx::collection& collection,
                          const bsoncxx::oid& id, const string& reason)
{
    auto filter = make_document(kvp("_id", id));
    auto update = make_document(kvp("$set", make_document(kvp("cancelReason", reason))));
    collection.update_one(filter.view(), update.view());
}

static void ackCommand(mongocxx::collection& collection,
                       const bsoncxx::oid& id, bool ack, const string& resultDescription)
{
    auto filter = make_document(kvp("_id", id));
    auto update = make_document(kvp("$set", make_document(
        kvp("delivered",      true),
        kvp("ack",            ack),
        kvp("ackTimeTag",     bsoncxx::types::b_date(chrono::milliseconds(nowMs()))),
        kvp("resultDescription", resultDescription))));
    collection.update_one(filter.view(), update.view());
}

static void executeCommand(const bsoncxx::document::view& command, mongocxx::collection& collection)
{
    if (!Active)
        return;

    const auto id   = command["_id"].get_oid().value;
    const auto conn = findConnection(
        static_cast<int>(getDouble(command, "protocolSourceConnectionNumber")));
    if (!conn || !conn->master)
    {
        cancelCommand(collection, id, "connection_not_found");
        return;
    }
    if (!conn->isConnected)
    {
        cancelCommand(collection, id, "not_connected");
        return;
    }
    if (!conn->commandsEnabled)
    {
        cancelCommand(collection, id, "cmds_disabled");
        return;
    }
    if (nowMs() - getDateMs(command, "timeTag", nowMs()) > 10000)
    {
        cancelCommand(collection, id, "expired");
        return;
    }

    const int      group     = static_cast<int>(getDouble(command, "protocolSourceCommonAddress"));
    const int      variation = static_cast<int>(getDouble(command, "protocolSourceASDU"));
    const uint16_t index     = static_cast<uint16_t>(getDouble(command, "protocolSourceObjectAddress"));
    const bool     useSbo    = getBool(command, "protocolSourceCommandUseSBO");
    const double   value     = getDouble(command, "value");
    const int      duration  = static_cast<int>(getDouble(command, "protocolSourceCommandDuration"));

    // The callback is invoked on the opendnp3 executor strand. Blocking that strand
    // prevents the master from sending the application confirm for outstation unsolicited
    // responses, causing repeated "Unsolicited confirmation timed out" on the outstation.
    // Dispatch the MongoDB write to a detached thread so the strand returns immediately.
    // The thread must NOT reuse the caller's collection: a mongocxx::client and its
    // children are single-threaded, and the processMongoCmd thread is concurrently
    // blocked on the change-stream cursor of that same client. Open a fresh client.
    auto callback = [id](const ICommandTaskResult& result) {
        const bool ok = result.summary == TaskCompletion::SUCCESS;
        string resultStr;
        switch (result.summary)
        {
        case TaskCompletion::SUCCESS:                       resultStr = "SUCCESS";                       break;
        case TaskCompletion::FAILURE_BAD_RESPONSE:         resultStr = "FAILURE_BAD_RESPONSE";          break;
        case TaskCompletion::FAILURE_RESPONSE_TIMEOUT:     resultStr = "FAILURE_RESPONSE_TIMEOUT";      break;
        case TaskCompletion::FAILURE_START_TIMEOUT:        resultStr = "FAILURE_START_TIMEOUT";         break;
        case TaskCompletion::FAILURE_MESSAGE_FORMAT_ERROR: resultStr = "FAILURE_MESSAGE_FORMAT_ERROR";  break;
        case TaskCompletion::FAILURE_NO_COMMS:             resultStr = "FAILURE_NO_COMMS";              break;
        default:                                            resultStr = "UNKNOWN";                       break;
        }
        thread([id, ok, resultStr]() {
            try
            {
                auto client = connectMongoClient();
                auto db = (*client)[JSConfig.mongoDatabaseName];
                auto cmdCollection = db[CommandsQueueCollectionName];
                ackCommand(cmdCollection, id, ok, resultStr);
            }
            catch (const exception& ex)
            {
                Log.log("ackCommand error: " + string(ex.what()));
            }
        }).detach();
    };

    if (group == 12)
    {
        OperationType  operation    = OperationType::NUL;
        TripCloseCode  tripCloseCode = TripCloseCode::NUL;
        uint32_t onTime  = 0;
        uint32_t offTime = 0;
        switch (duration)
        {
        case 1:
            onTime = CROB_PulseOnTime; offTime = CROB_PulseOffTime;
            operation = value != 0 ? OperationType::PULSE_ON : OperationType::PULSE_OFF; break;
        case 2:
            onTime = CROB_PulseOnTime; offTime = CROB_PulseOffTime;
            operation = value != 0 ? OperationType::PULSE_OFF : OperationType::PULSE_ON; break;
        case 3:
            operation = value != 0 ? OperationType::LATCH_ON : OperationType::LATCH_OFF; break;
        case 4:
            operation = value != 0 ? OperationType::LATCH_OFF : OperationType::LATCH_ON; break;
        case 11:
            onTime = CROB_PulseOnTime; offTime = CROB_PulseOffTime;
            operation     = value != 0 ? OperationType::PULSE_ON  : OperationType::PULSE_OFF;
            tripCloseCode = value != 0 ? TripCloseCode::CLOSE      : TripCloseCode::TRIP; break;
        case 13:
            operation     = value != 0 ? OperationType::LATCH_ON  : OperationType::LATCH_OFF;
            tripCloseCode = value != 0 ? TripCloseCode::CLOSE      : TripCloseCode::TRIP; break;
        case 21:
            onTime = CROB_PulseOnTime; offTime = CROB_PulseOffTime;
            operation     = value != 0 ? OperationType::PULSE_ON  : OperationType::PULSE_OFF;
            tripCloseCode = value != 0 ? TripCloseCode::TRIP       : TripCloseCode::CLOSE; break;
        case 23:
            operation     = value != 0 ? OperationType::LATCH_ON  : OperationType::LATCH_OFF;
            tripCloseCode = value != 0 ? TripCloseCode::TRIP       : TripCloseCode::CLOSE; break;
        default: break;
        }
        ControlRelayOutputBlock crob(operation, tripCloseCode, false, 1, onTime, offTime);
        if (useSbo)
            conn->master->SelectAndOperate(crob, index, callback, TaskConfig::Default());
        else
            conn->master->DirectOperate(crob, index, callback, TaskConfig::Default());
        return;
    }
    if (group == 41 && variation == 1)
    {
        if (useSbo) conn->master->SelectAndOperate(AnalogOutputInt32(static_cast<int32_t>(value)), index, callback, TaskConfig::Default());
        else        conn->master->DirectOperate   (AnalogOutputInt32(static_cast<int32_t>(value)), index, callback, TaskConfig::Default());
        return;
    }
    if (group == 41 && variation == 2)
    {
        if (useSbo) conn->master->SelectAndOperate(AnalogOutputInt16(static_cast<int16_t>(value)), index, callback, TaskConfig::Default());
        else        conn->master->DirectOperate   (AnalogOutputInt16(static_cast<int16_t>(value)), index, callback, TaskConfig::Default());
        return;
    }
    if (group == 41 && variation == 4)
    {
        if (useSbo) conn->master->SelectAndOperate(AnalogOutputDouble64(value), index, callback, TaskConfig::Default());
        else        conn->master->DirectOperate   (AnalogOutputDouble64(value), index, callback, TaskConfig::Default());
        return;
    }
    if (group == 41)
    {
        if (useSbo) conn->master->SelectAndOperate(AnalogOutputFloat32(static_cast<float>(value)), index, callback, TaskConfig::Default());
        else        conn->master->DirectOperate   (AnalogOutputFloat32(static_cast<float>(value)), index, callback, TaskConfig::Default());
        return;
    }

    cancelCommand(collection, id, "unsupported_group");
}

// ---------------------------------------------------------------------------
// processMongo — dequeue + update realtimeData
// ---------------------------------------------------------------------------

void processMongo()
{
    Log.log("processMongo: Function entered");
    try
    {
        Log.log("processMongo thread started");
        for (;;)
        {
            try
            {
                Log.log("processMongo: Connecting to MongoDB...", Logger::Level::Detailed);
                auto client     = connectMongoClient();
                auto db         = (*client)[JSConfig.mongoDatabaseName];
                auto collection = db[RealtimeDataCollectionName];
                Log.log("processMongo: Connected to MongoDB", Logger::Level::Detailed);

                for (;;)
                {
                    unique_lock<mutex> lock(QueueMutex);
                    QueueCv.wait_for(lock, chrono::milliseconds(100), [] { return !DNP3DataQueue.empty(); });
                    deque<Dnp3Value> batch;
                    batch.swap(DNP3DataQueue);
                    lock.unlock();

                    if (!isMongoLive(db))
                    {
                        Log.log("processMongo: MongoDB connection lost, reconnecting...",
                            Logger::Level::Detailed);
                        throw runtime_error("MongoDB connection failed");
                    }
                    if (batch.empty())
                        continue;

                    vector<mongocxx::model::write>     writes;
                    vector<bsoncxx::document::value>   insertDocValues;
                    vector<bsoncxx::document::view>    insertDocViews;
                    vector<bsoncxx::document::value>   filterStore;
                    vector<bsoncxx::document::value>   updateStore;
                    filterStore.reserve(batch.size());
                    updateStore.reserve(batch.size());

                    for (const auto& iv : batch)
                    {
                        try
                        {
                            if (!isfinite(iv.value) || iv.value > 1e100 || iv.value < -1e100)
                            {
                                Log.log("Mongo: Skipping invalid value addr=" + to_string(iv.address)
                                    + " val=" + to_string(iv.value), Logger::Level::Detailed);
                                continue;
                            }

                            int64_t safeServerTime = iv.serverTimestamp;
                            if (safeServerTime < 1000000000000 || safeServerTime > 2000000000000)
                                safeServerTime = 0;
                            int64_t safeSourceTime = iv.hasSourceTimestamp ? iv.sourceTimestamp : 0;
                            if (safeSourceTime < 1000000000000 || safeSourceTime > 2000000000000)
                                safeSourceTime = 0;

                            // Auto-create tags for new (baseGroup, address) pairs
                            auto conn = findConnection(iv.connNumber);
                            if (conn && conn->autoCreateTags
                                && conn->insertedAddresses.insert({iv.baseGroup, iv.address}).second)
                            {
                                double commandId = 0.0;
                                if (conn->commandsEnabled && (iv.baseGroup == 10 || iv.baseGroup == 40))
                                {
                                    commandId          = getNextAutoKey(conn, collection);
                                    const int    cmdGroup = iv.baseGroup == 10 ? 12 : 41;
                                    const double asdu     = iv.baseGroup == 10 ? 1.0 : 3.0;
                                    const double dur      = iv.baseGroup == 10 ? 3.0 : 0.0;
                                    insertDocValues.push_back(newRealtimeTagDoc(
                                        iv, conn->name, commandId, true, cmdGroup, asdu, dur,
                                        0.0, commandId + 1.0));
                                    insertDocViews.push_back(insertDocValues.back().view());
                                    conn->insertedAddresses.insert({cmdGroup, iv.address});
                                    Log.log(conn->name + " - INSERT NEW COMMAND TAG: "
                                        + conn->name + ";" + to_string(cmdGroup) + ";" + to_string(iv.address),
                                        Logger::Level::Basic);
                                }
                                const double newId = getNextAutoKey(conn, collection);
                                insertDocValues.push_back(newRealtimeTagDoc(
                                    iv, conn->name, newId, false, iv.baseGroup, 0.0, 0.0, commandId, 0.0));
                                insertDocViews.push_back(insertDocValues.back().view());
                                Log.log(conn->name + " - INSERT NEW TAG: "
                                    + conn->name + ";" + to_string(iv.baseGroup) + ";" + to_string(iv.address),
                                    Logger::Level::Basic);
                            }

                            Log.log("Mongo: Writing conn=" + to_string(iv.connNumber)
                                + " addr=" + to_string(iv.address)
                                + " group=" + to_string(iv.baseGroup)
                                + " val=" + to_string(iv.value), Logger::Level::Detailed);

                            filterStore.push_back(make_document(
                                kvp("protocolSourceConnectionNumber", iv.connNumber),
                                kvp("protocolSourceCommonAddress",    iv.baseGroup),
                                kvp("protocolSourceObjectAddress",    iv.address)));

                            updateStore.push_back(make_document(kvp("$set", make_document(
                                kvp("sourceDataUpdate", make_document(
                                    kvp("valueAtSource",          iv.value),
                                    kvp("valueStringAtSource",    iv.valueString),
                                    kvp("asduAtSource",           to_string(iv.group) + " " + to_string(iv.variation)),
                                    kvp("causeOfTransmissionAtSource", to_string(iv.cot)),
                                    kvp("timeTagAtSource",        bsoncxx::types::b_date(chrono::milliseconds(safeSourceTime))),
                                    kvp("timeTagAtSourceOk",      iv.timeTagOk),
                                    kvp("timeTag",                bsoncxx::types::b_date(chrono::milliseconds(safeServerTime))),
                                    kvp("notTopicalAtSource",     iv.qCommLost),
                                    kvp("invalidAtSource",        iv.qCommLost || iv.qReferenceError || !iv.qOnline),
                                    kvp("overflowAtSource",       iv.qOverrange),
                                    kvp("blockedAtSource",        !iv.qOnline),
                                    kvp("substitutedAtSource",    iv.qRemoteForced || iv.qLocalForced),
                                    kvp("carryAtSource",          iv.qRollover),
                                    kvp("transientAtSource",      iv.qTransient),
                                    kvp("originator",             ProtocolDriverName + "|" + to_string(iv.connNumber))))))));

                            mongocxx::model::update_one op(filterStore.back().view(), updateStore.back().view());
                            writes.emplace_back(std::move(op));
                        }
                        catch (const exception& ex)
                        {
                            Log.log("Mongo: Error processing item: " + string(ex.what()),
                                Logger::Level::Detailed);
                        }
                    }

                    // Inserts before bulk_write so update_one filter finds the new doc
                    if (!insertDocViews.empty())
                    {
                        try
                        {
                            mongocxx::options::insert iopts;
                            iopts.ordered(false);
                            collection.insert_many(insertDocViews, iopts);
                        }
                        catch (const exception& ex)
                        {
                            Log.log("Mongo: insert_many error (possible duplicate): " + string(ex.what()),
                                Logger::Level::Detailed);
                        }
                        insertDocValues.clear();
                        insertDocViews.clear();
                    }
                    if (!writes.empty())
                        collection.bulk_write(writes);
                }
            }
            catch (const exception& ex)
            {
                Log.log("Exception Mongo: " + string(ex.what()));
                this_thread::sleep_for(chrono::seconds(3));
            }
        }
    }
    catch (const exception& ex)
    {
        Log.log("FATAL processMongo: " + string(ex.what()));
    }
}

// ---------------------------------------------------------------------------
// processMongoCmd — command stream watcher
// ---------------------------------------------------------------------------

void processMongoCmd()
{
    for (;;)
    {
        try
        {
            auto client     = connectMongoClient();
            auto db         = (*client)[JSConfig.mongoDatabaseName];
            auto collection = db[CommandsQueueCollectionName];
            mongocxx::pipeline pipeline;
            pipeline.match(make_document(kvp("operationType", "insert")));
            auto cursor = collection.watch(pipeline);
            for (auto&& change : cursor)
            {
                auto fullDocument = change["fullDocument"];
                if (fullDocument && fullDocument.type() == bsoncxx::type::k_document)
                    executeCommand(fullDocument.get_document().view(), collection);
            }
        }
        catch (const exception& ex)
        {
            Log.log("Exception Mongo CMD: " + string(ex.what()));
            this_thread::sleep_for(chrono::seconds(3));
        }
    }
}

// ---------------------------------------------------------------------------
// processRedundancy — active/standby arbitration and keep-alive heartbeat
// ---------------------------------------------------------------------------

void processRedundancy()
{
    constexpr int64_t RedundancyStaleTimeoutMs = 15000;
    for (;;)
    {
        try
        {
            auto client      = connectMongoClient();
            auto db          = (*client)[JSConfig.mongoDatabaseName];
            auto instances   = db[ProtocolDriverInstancesCollectionName];
            auto connections = db[ProtocolConnectionsCollectionName];

            for (;;)
            {
                if (!isMongoLive(db))
                    throw runtime_error("MongoDB connection failed");

                auto instance = instances.find_one(make_document(
                    kvp("protocolDriver",               ProtocolDriverName),
                    kvp("protocolDriverInstanceNumber", ProtocolDriverInstanceNumber)));

                bool shouldBeActive = true;
                if (instance)
                {
                    auto view           = instance->view();
                    const auto activeNode = getString(view, "activeNodeName");
                    shouldBeActive        = activeNode == JSConfig.nodeName;
                    if (!shouldBeActive && !activeNode.empty())
                    {
                        const auto keepAlive    = getDateMs(view, "activeNodeKeepAliveTimeTag");
                        const auto keepAliveAge = keepAlive > 0
                            ? nowMs() - keepAlive : RedundancyStaleTimeoutMs + 1;
                        if (keepAliveAge > RedundancyStaleTimeoutMs)
                            shouldBeActive = true;
                    }
                }

                const bool becameActive   = shouldBeActive && !Active.load();
                const bool becameInactive = !shouldBeActive && Active.load();
                Active = shouldBeActive;

                if (becameActive)
                {
                    Log.log("Redundancy - ACTIVATING this node!");
                    try
                    {
                        for (const auto& conn : snapshotConnections())
                        {
                            if (conn->master)
                            {
                                Log.log(conn->name + " - Enabling master...", Logger::Level::Detailed);
                                conn->master->Enable();
                                Log.log(conn->name + " - Master enabled successfully",
                                    Logger::Level::Detailed);
                            }
                        }
                    }
                    catch (const exception& ex)
                    {
                        Log.log("Exception enabling master: " + string(ex.what()),
                            Logger::Level::Detailed);
                    }
                }
                if (becameInactive)
                {
                    Log.log("Redundancy - DEACTIVATING this node.");
                    for (const auto& conn : snapshotConnections())
                    {
                        if (conn->master)
                            conn->master->Disable();
                        conn->isConnected = false;
                    }
                }
                else if (!Active)
                {
                    Log.log("Redundancy - Node is STANDBY; masters remain disabled.",
                        Logger::Level::Detailed);
                }

                if (Active)
                {
                    instances.find_one_and_update(
                        make_document(
                            kvp("protocolDriver",               ProtocolDriverName),
                            kvp("protocolDriverInstanceNumber", ProtocolDriverInstanceNumber)),
                        make_document(kvp("$set", make_document(
                            kvp("activeNodeName", JSConfig.nodeName),
                            kvp("activeNodeKeepAliveTimeTag",
                                bsoncxx::types::b_date(chrono::milliseconds(nowMs())))))));

                    for (const auto& conn : snapshotConnections())
                    {
                        if (!conn->channel)
                            continue;
                        auto stats  = conn->channel->GetStatistics();
                        auto filter = make_document(
                            kvp("protocolConnectionNumber", conn->protocolConnectionNumber));
                        auto update = make_document(kvp("$set", make_document(
                            kvp("stats", make_document(
                                kvp("nodeName",          JSConfig.nodeName),
                                kvp("timeTag",           bsoncxx::types::b_date(chrono::milliseconds(nowMs()))),
                                kvp("isConnected",       conn->isConnected.load()),
                                kvp("numHeaderCrcError", static_cast<int64_t>(stats.parser.numHeaderCrcError)),
                                kvp("numBodyCrcError",   static_cast<int64_t>(stats.parser.numBodyCrcError)),
                                kvp("numBytesRx",        static_cast<int64_t>(stats.channel.numBytesRx)),
                                kvp("numBytesTx",        static_cast<int64_t>(stats.channel.numBytesTx)),
                                kvp("numClose",          static_cast<int64_t>(stats.channel.numClose)),
                                kvp("numLinkFrameRx",    static_cast<int64_t>(stats.parser.numLinkFrameRx)),
                                kvp("numLinkFrameTx",    static_cast<int64_t>(stats.channel.numLinkFrameTx)),
                                kvp("numOpen",           static_cast<int64_t>(stats.channel.numOpen)),
                                kvp("numOpenFail",       static_cast<int64_t>(stats.channel.numOpenFail)))))));
                        connections.update_one(filter.view(), update.view());
                    }
                }

                this_thread::sleep_for(chrono::seconds(5));
            }
        }
        catch (const exception& ex)
        {
            Log.log("Exception Mongo Redundancy: " + string(ex.what()));
            this_thread::sleep_for(chrono::seconds(3));
        }
    }
}
