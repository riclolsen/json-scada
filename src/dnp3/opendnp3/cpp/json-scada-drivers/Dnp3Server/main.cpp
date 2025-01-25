/*
 * IEC DNP3 Outstation Server Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2025 - Ricardo L. Olsen
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

#include "json.hpp" // Include the nlohmann/json header

#include <opendnp3/ConsoleLogger.h>
#include <opendnp3/DNP3Manager.h>
#include <opendnp3/channel/PrintingChannelListener.h>
#include <opendnp3/logging/LogLevels.h>
#include <opendnp3/outstation/DefaultOutstationApplication.h>
#include <opendnp3/outstation/IUpdateHandler.h>
#include <opendnp3/outstation/SimpleCommandHandler.h>
#include <opendnp3/outstation/UpdateBuilder.h>

#include <bsoncxx/builder/basic/document.hpp>
#include <bsoncxx/json.hpp>
#include <bsoncxx/stdx/optional.hpp>
#include <bsoncxx/stdx/string_view.hpp>
#include <mongocxx/client.hpp>
#include <mongocxx/instance.hpp>
#include <mongocxx/logger.hpp>
#include <mongocxx/options/change_stream.hpp>
#include <mongocxx/pool.hpp>
#include <mongocxx/uri.hpp>

#include <chrono>
#include <cstdint>
#include <cstdlib>
#include <fstream>
#include <iomanip>
#include <iostream>
#include <memory>
#include <sstream>
#include <stdexcept>
#include <string>
#include <thread>
#include <type_traits>
#include <vector>

using namespace std;
using namespace opendnp3;
using bsoncxx::builder::basic::kvp;
using bsoncxx::builder::basic::make_document;
using json = nlohmann::json;

const string CopyrightMessage = "{json:scada} IEC60870-5-104 Server Driver - Copyright 2025 RLO";
const string VersionStr = "0.0.1";

// Logger class to handle logging with different levels
class Logger
{
public:
    enum class LogLevel
    {
        NoLog = 0,    // log level 0=no
        Basic = 1,    // log level 1=basic (default)
        Detailed = 2, // log level 2=detailed
        Debug = 3     // log level 3=debug
    };

    Logger(LogLevel level = LogLevel::Basic) : logLevel(level) {}
    void SetLogLevel(LogLevel level)
    {
        logLevel = level;
    }
    void SetLogLevel(int level)
    {
        logLevel = (LogLevel)level;
    }
    auto GetLogLevel() const
    {
        return logLevel;
    }

    void Log(const std::string& message, const LogLevel level = LogLevel::Basic) const
    {
        if (level > logLevel)
            return;

        auto now = std::chrono::system_clock::now();
        auto now_time_t = std::chrono::system_clock::to_time_t(now);
        auto now_tm = *std::localtime(&now_time_t);

        auto ms = std::chrono::duration_cast<std::chrono::milliseconds>(now.time_since_epoch()) % 1000;

        std::ostringstream oss;
        oss << std::put_time(&now_tm, "%Y-%m-%dT%H:%M:%S") << '.' << std::setfill('0') << std::setw(3) << ms.count();

        // Get the timezone offset
        auto time_t = std::chrono::system_clock::to_time_t(now);
        auto local_tm = *std::localtime(&time_t);
        auto gmt_tm = *std::gmtime(&time_t);
        auto offset = std::difftime(std::mktime(&local_tm), std::mktime(&gmt_tm)) / 60;
        auto hours = int(offset) / 60;
        auto minutes = int(offset) % 60;

        oss << (offset >= 0 ? "+" : "-") << std::setfill('0') << std::setw(2) << std::abs(hours) << ":" << std::setw(2)
            << std::abs(minutes);

        std::cout << "[" << oss.str() << "] " << message << std::endl;
    }

private:
    LogLevel logLevel;
};

Logger Log;

static double getDouble(const bsoncxx::document::view& doc, const std::string& key, double defaultVal = 0)
{
    try
    {
        auto value = doc[key].get_value();
        switch (value.type())
        {
        case bsoncxx::type::k_null:
            return defaultVal;
        case bsoncxx::type::k_int32:
            return double(value.get_int32().value);
        case bsoncxx::type::k_int64:
            return double(value.get_int64().value);
        case bsoncxx::type::k_double:
            return value.get_double().value;
        default:
            return defaultVal;
        }
    }
    catch (const std::exception&)
    {
        Log.Log("Error getting double value for key: " + key);
    }
    return defaultVal;
}

static bool getBoolean(const bsoncxx::document::view& doc, const std::string& key, bool defaultVal = false)
{
    try
    {
        auto value = doc[key].get_value();
        switch (value.type())
        {
        case bsoncxx::type::k_null:
            return defaultVal;
        case bsoncxx::type::k_bool:
            return value.get_bool().value;
        case bsoncxx::type::k_int32:
            return bool(value.get_int32().value);
        case bsoncxx::type::k_int64:
            return bool(value.get_int64().value);
        case bsoncxx::type::k_double:
            return bool(value.get_double().value);
        default:
            return defaultVal;
        }
    }
    catch (const std::exception&)
    {
        Log.Log("Error getting boolean value for key: " + key);
    }
    return defaultVal;
}

static std::string getString(const bsoncxx::document::view& doc, const std::string& key, std::string defaultVal = "")
{
    try
    {
        auto value = doc[key].get_value();
        switch (value.type())
        {
        case bsoncxx::type::k_null:
            return defaultVal;
        case bsoncxx::type::k_string:
            return std::string(value.get_string().value);
        case bsoncxx::type::k_bool:
            return value.get_bool().value ? "true" : "false";
        case bsoncxx::type::k_int32:
            return std::to_string(value.get_int32().value);
        case bsoncxx::type::k_int64:
            return std::to_string(value.get_int64().value);
        case bsoncxx::type::k_double:
            return std::to_string(value.get_double().value);
        default:
            return defaultVal;
        }
    }
    catch (const std::exception&)
    {
        Log.Log("Error getting string value for key: " + key);
    }
    return defaultVal;
}

typedef struct
{
    int protocolConnectionNumber;
    std::string name;
    std::string ipAddressLocalBind;
    std::vector<std::string> ipAddresses;
    std::vector<std::string> topics;
    int localLinkAddress;
    int remoteLinkAddress;
    int timeSyncInterval;
    int timeSyncMode;
    double hoursShift;
    double asyncOpenDelay;
    bool autoCreateTags;
    bool enableUnsolicited;
    int serverQueueSize;
    std::string connectionMode;
    int baudRate;
    std::string parity;
    std::string stopBits;
    std::string handshake;
    bool useSecurity;
    std::string localCertFilePath;
    std::string privateKeyFilePath;
    std::string rootCertFilePath;
    std::vector<std::string> peerCertFilesPaths;
    std::vector<std::string> cipherList;
    bool chainValidation;
    bool allowOnlySpecificCertificates;
    bool allowTLSv10;
    bool allowTLSv11;
    bool allowTLSv12;
    bool allowTLSv13;

    DNP3Manager* manager;
    std::shared_ptr<IChannel> channel;
    std::shared_ptr<IOutstation> outstation;
    std::shared_ptr<ICommandHandler> commandHandler;
} DNP3Connection_t;

std::vector<DNP3Connection_t> dnp3Connections;

class MyCommandHandler : public opendnp3::ICommandHandler
{
public:
    void Begin() override {}
    void End() override {}
    CommandStatus Select(const ControlRelayOutputBlock& command, uint16_t index) override
    {
        return CommandStatus::SUCCESS;
    }
    CommandStatus Operate(const ControlRelayOutputBlock& command,
                          uint16_t index,
                          IUpdateHandler& handler,
                          OperateType opType) override
    {
        return CommandStatus::SUCCESS;
    }
    CommandStatus Select(const AnalogOutputInt16& command, uint16_t index) override
    {
        return CommandStatus::SUCCESS;
    }
    CommandStatus Operate(const AnalogOutputInt16& command,
                          uint16_t index,
                          IUpdateHandler& handler,
                          OperateType opType) override
    {
        return CommandStatus::SUCCESS;
    }
    CommandStatus Select(const AnalogOutputInt32& command, uint16_t index) override
    {
        return CommandStatus::SUCCESS;
    }
    CommandStatus Operate(const AnalogOutputInt32& command,
                          uint16_t index,
                          IUpdateHandler& handler,
                          OperateType opType) override
    {
        return CommandStatus::SUCCESS;
    }
    CommandStatus Select(const AnalogOutputFloat32& command, uint16_t index) override
    {
        return CommandStatus::SUCCESS;
    }
    CommandStatus Operate(const AnalogOutputFloat32& command,
                          uint16_t index,
                          IUpdateHandler& handler,
                          OperateType opType) override
    {
        return CommandStatus::SUCCESS;
    }
    CommandStatus Select(const AnalogOutputDouble64& command, uint16_t index) override
    {
        return CommandStatus::SUCCESS;
    }
    CommandStatus Operate(const AnalogOutputDouble64& command,
                          uint16_t index,
                          IUpdateHandler& handler,
                          OperateType opType) override
    {
        return CommandStatus::SUCCESS;
    }
};

static DatabaseConfig ConfigureDatabase()
{
    DatabaseConfig config(10); // 10 of each type with default settings

    for (int i=0; i < config.analog_input.size(); i++)
    {
        config.analog_input[i].clazz = PointClass::Class2;
        config.analog_input[i].svariation = StaticAnalogVariation::Group30Var5;
        config.analog_input[i].evariation = EventAnalogVariation::Group32Var7;
    }

    return config;
}

struct State
{
    uint32_t count = 0;
    double value = 0;
    bool binary = false;
    DoubleBit dbit = DoubleBit::DETERMINED_OFF;
    uint8_t octetStringValue = 1;
};

auto app = DefaultOutstationApplication::Create();

void AddUpdates(UpdateBuilder& builder, State& state, const std::string& arguments);

int __cdecl main(int argc, char* argv[])
{
    Log.Log(CopyrightMessage);
    Log.Log("Driver version: " + VersionStr);
    Log.Log("Using OpenDnp3 version 3.1.2");
    Log.Log("Usage: " + std::string(argv[0]) + " [ProtocolDriverInstanceNumber] [LogLevel] [ConfigurationFile]");

    auto ProtocolDriverInstanceNumber = 1;
    std::string configFileName = "../conf/json-scada.json";
    if (argc > 1)
    {
        try
        {
            ProtocolDriverInstanceNumber = std::stoi(argv[1]);
        }
        catch (const std::invalid_argument&)
        {
            std::cerr << "Conversion error: Invalid integer value for ProtocolDriverInstanceNumber" << std::endl;
            return 0;
        }
    }
    Log.Log("ProtocolDriverInstanceNumber: " + std::to_string(ProtocolDriverInstanceNumber));

    if (argc > 2)
    {
        try
        {
            Log.SetLogLevel(std::stoi(argv[2]));
        }
        catch (const std::invalid_argument&)
        {
            std::cerr << "Conversion error: Invalid integer value for LogLevel" << std::endl;
            return 0;
        }
    }
    Log.Log(std::string("LogLevel: ") + std::to_string((int)Log.GetLogLevel()));

    if (argc > 3)
    {
        configFileName = argv[3];
    }

    // Read the JSON configuration file
    std::ifstream configFile(configFileName);
    if (!configFile.good())
    {
        Log.Log("Could not open the configuration file: " + configFileName);
        configFileName = "/json-scada/conf/json-scada.json";
        configFile.open(configFileName);
    }
    if (!configFile.is_open())
    {
        Log.Log("Could not open the configuration file!" + configFileName);
        return -1;
    }
    Log.Log("ConfigurationFile: " + configFileName);

    json jsonCfg;
    configFile >> jsonCfg;

    std::string uristrMongo = jsonCfg["mongoConnectionString"];
    std::string dbstrMongo = jsonCfg["mongoDatabaseName"];
    std::string nodeName = jsonCfg["nodeName"];
    std::string connectionsListStr = "";

    if (uristrMongo.empty() || dbstrMongo.empty())
    {
        Log.Log("MongoDB connection string or database name or node name is empty in " + configFileName);
        return -1;
    }

    mongocxx::instance instance{};
    mongocxx::uri uri(uristrMongo);
    std::cout << "Connecting to MongoDB" << std::endl;
    mongocxx::client client(uri);
    std::cout << "Connected to MongoDB" << std::endl;

    auto db = client[dbstrMongo];
    // auto rtDataCollection = db["realtimeData"];
    // auto result = rtDataCollection.find_one(make_document(kvp("_id", -1.0)));
    auto protocolDriverInstancesCollection = db["protocolDriverInstances"];
    auto result = protocolDriverInstancesCollection.find_one(make_document(
        kvp("protocolDriver", "DNP3_SERVER"), kvp("protocolDriverInstanceNumber", ProtocolDriverInstanceNumber)));
    if (!result)
    {
        Log.Log("Protocol driver instance not found in the database");
        return -1;
    }
    if (!(*result)["enabled"].get_bool().value)
    {
        Log.Log("Protocol driver instance is disabled in the database");
        return -1;
    }
    if ((*result)["nodeNames"].type() == bsoncxx::type::k_array)
    {
        auto nodeNamesArray = (*result)["nodeNames"].get_array().value;
        auto found = false;
        auto cnt = 0;
        for (const auto& nodeNameEl : nodeNamesArray)
        {
            cnt++;
            if (nodeNameEl.get_string().value == nodeName)
            {
                Log.Log(std::string("Node Name: ") + std::string(nodeNameEl.get_string().value));
                found = true;
                break;
            }
        }
        if (cnt > 0 && !found)
        {
            Log.Log("Node name not found in the protocol driver instance configuration");
            return -1;
        }
    }
    auto protocolDriverConnectionsCollection = db["protocolConnections"];
    auto resConn = protocolDriverConnectionsCollection.find(
        make_document(kvp("protocolDriver", "DNP3_SERVER"),
                      kvp("protocolDriverInstanceNumber", ProtocolDriverInstanceNumber), kvp("enabled", true)));
    auto cnt = 0;
    for (auto&& doc : resConn)
    {
        cnt++;
        std::cout << bsoncxx::to_json(doc) << std::endl;
        connectionsListStr += std::to_string((int)getDouble(doc, "protocolConnectionNumber")) + ",";

        DNP3Connection_t dnp3Connection{(int)getDouble(doc, "protocolConnectionNumber"),
                                        getString(doc, "name"),
                                        getString(doc, "ipAddressLocalBind"),
                                        std::vector<std::string>{}, // ipAddresses
                                        std::vector<std::string>{}, // topics
                                        (int)getDouble(doc, "localLinkAddress", 1),
                                        (int)getDouble(doc, "remoteLinkAddress", 1),
                                        (int)getDouble(doc, "timeSyncInterval"),
                                        (int)getDouble(doc, "timeSyncMode"),
                                        getDouble(doc, "hoursShift"),
                                        getDouble(doc, "asyncOpenDelay"),
                                        getBoolean(doc, "autoCreateTags"),
                                        getBoolean(doc, "enableUnsolicited", true),
                                        (int)getDouble(doc, "serverQueueSize", 1000),
                                        getString(doc, "connectionMode", "TCP Inactive"),
                                        (int)getDouble(doc, "baudRate", 9600),
                                        getString(doc, "parity", "None"),
                                        getString(doc, "stopBits", "One"),
                                        getString(doc, "handshake", "None"),
                                        getBoolean(doc, "useSecurity"),
                                        getString(doc, "localCertFilePath"),
                                        getString(doc, "privateKeyFilePath"),
                                        getString(doc, "rootCertFilePath"),
                                        std::vector<std::string>{}, // peerCertFilePaths
                                        std::vector<std::string>{}, // cipherList
                                        getBoolean(doc, "allowOnlySpecificCertificates"),
                                        getBoolean(doc, "allowTLSv10"),
                                        getBoolean(doc, "allowTLSv11"),
                                        getBoolean(doc, "allowTLSv12"),
                                        getBoolean(doc, "allowTLSv13"),
                                        nullptr,
                                        nullptr,
                                        nullptr,
                                        nullptr};

        if (doc["ipAddresses"].get_value().type() == bsoncxx::type::k_array)
        {
            auto a1 = doc["ipAddresses"].get_array().value;
            for (const auto& el : a1)
            {
                dnp3Connection.ipAddresses.push_back(std::string(el.get_string().value));
            }
        }
        if (doc["topics"].get_value().type() == bsoncxx::type::k_array)
        {
            auto a2 = doc["topics"].get_array().value;
            for (const auto& el : a2)
            {
                dnp3Connection.topics.push_back(std::string(el.get_string().value));
            }
        }
        if (doc["peerCertFilesPaths"].get_value().type() == bsoncxx::type::k_array)
        {
            auto a3 = doc["peerCertFilesPaths"].get_array().value;
            for (const auto& el : a3)
            {
                dnp3Connection.peerCertFilesPaths.push_back(std::string(el.get_string().value));
            }
        }
        if (doc["cipherList"].get_value().type() == bsoncxx::type::k_array)
        {
            auto a4 = doc["cipherList"].get_array().value;
            for (const auto& el : a4)
            {
                dnp3Connection.cipherList.push_back(std::string(el.get_string().value));
            }
        }
        dnp3Connections.push_back(dnp3Connection);
    }
    if (cnt == 0)
    {
        Log.Log("No protocol connections found for the protocol driver instance");
        return -1;
    }
    // std::cout << bsoncxx::to_json(*result) << std::endl;

    for (auto& dnp3Conn : dnp3Connections)
    {
        Log.Log(std::string("Protocol Connection Number: ") + std::to_string(dnp3Conn.protocolConnectionNumber));

        if (dnp3Conn.autoCreateTags)
        {
            mongocxx::options::find opts;
            Log.Log("Auto Create Tags is enabled");
            // Create destination for tags on the DNP3 connection

            // DIGITAL TAGS, will distribute as Group 1 VAR 0
            auto lastG1Addr = -1;

            // find the latest used object address

            // find tags with a destination linked to this connection
            opts.sort(bsoncxx::from_json(R"({"protocolDestinations.protocolDestinationObjectAddress": 1})"));
            auto resTagsG1
                = db["realtimeData"].find(make_document(kvp("type", "digital"), kvp("origin", "supervised"),
                                                        kvp("protocolDestinations.protocolDestinationCommonAddress", 1),
                                                        kvp("protocolDestinations.protocolDestinationConnectionNumber",
                                                            dnp3Conn.protocolConnectionNumber)),
                                          opts);
            auto cntG1 = 0;
            for (auto&& docG1 : resTagsG1)
            {
                cntG1++;
                // std::cout << bsoncxx::to_json(docG1) << std::endl;

                // look in the protocolDestinations array for entry with the connection number
                auto protocolDestinations = docG1["protocolDestinations"].get_array().value;
                for (const auto& el : protocolDestinations)
                {
                    auto protocolDestination = el.get_document().value;
                    if ((int)getDouble(protocolDestination, "protocolDestinationConnectionNumber")
                        == dnp3Conn.protocolConnectionNumber)
                    {
                        if (getDouble(protocolDestination, "protocolDestinationObjectAddress") > lastG1Addr)
                            lastG1Addr = getDouble(protocolDestination, "protocolDestinationObjectAddress");
                    }
                }
            }

            // look for tags without a destination linked to this connection
            opts.sort(bsoncxx::from_json(R"({"_id": 1})"));
            auto resTagsDig = db["realtimeData"].find(
                make_document(kvp("type", "digital"), kvp("origin", "supervised"),
                              kvp("protocolDestinations.protocolDestinationConnectionNumber",
                                  make_document(kvp("$ne", dnp3Conn.protocolConnectionNumber)))),
                opts);

            for (auto&& doc : resTagsDig)
            {
                // std::cout << bsoncxx::to_json(doc) << std::endl;
                ++lastG1Addr;
                if (lastG1Addr > 65535)
                {
                    Log.Log("Object address for digitals exceeds 65535");
                    break;
                }
                Log.Log("Creating destination for tag: " + getString(doc, "_id") + " " + getString(doc, "tag")
                        + " Dnp3Address: " + std::to_string(lastG1Addr));
                db["realtimeData"].update_one(
                    make_document(kvp("_id", getDouble(doc, "_id"))),
                    make_document(kvp(
                        "$push",
                        make_document(kvp(
                            "protocolDestinations",
                            make_document(
                                kvp("protocolDestinationConnectionNumber", (double)dnp3Conn.protocolConnectionNumber),
                                kvp("protocolDestinationCommonAddress", 1.0),
                                kvp("protocolDestinationObjectAddress", (double)lastG1Addr),
                                kvp("protocolDestinationASDU", 0.0), kvp("protocolDestinationCommandDuration", 0.0),
                                kvp("protocolDestinationCommandUseSBO", false),
                                kvp("protocolDestinationCommandDuration", 0.0), kvp("protocolDestinationKConv1", 1.0),
                                kvp("protocolDestinationKConv2", 0.0), kvp("protocolDestinationGroup", 0.0),
                                kvp("protocolDestinationHoursShift", 0.0)))))));
            }

            // ANALOG TAGS, will distribute as Group 30 VAR 6 (double precision floating point)
            auto lastG30Addr = -1;

            // find the latest used object address

            // find tags with a destination linked to this connection
            opts.sort(bsoncxx::from_json(R"({"protocolDestinations.protocolDestinationObjectAddress": 1})"));
            auto resTagsG30 = db["realtimeData"].find(
                make_document(
                    kvp("type", "analog"), kvp("origin", "supervised"),
                    kvp("protocolDestinations.protocolDestinationCommonAddress", 30),
                    kvp("protocolDestinations.protocolDestinationConnectionNumber", dnp3Conn.protocolConnectionNumber)),
                opts);
            auto cntG30 = 0;
            for (auto&& docG30 : resTagsG1)
            {
                cntG1++;
                // std::cout << bsoncxx::to_json(docG1) << std::endl;

                // look in the protocolDestinations array for entry with the connection number
                auto protocolDestinations = docG30["protocolDestinations"].get_array().value;
                for (const auto& el : protocolDestinations)
                {
                    auto protocolDestination = el.get_document().value;
                    if ((int)getDouble(protocolDestination, "protocolDestinationConnectionNumber")
                        == dnp3Conn.protocolConnectionNumber)
                    {
                        if (getDouble(protocolDestination, "protocolDestinationObjectAddress") > lastG30Addr)
                            lastG30Addr = getDouble(protocolDestination, "protocolDestinationObjectAddress");
                    }
                }
            }

            // look for tags without a destination linked to this connection
            opts.sort(bsoncxx::from_json(R"({"_id": 1})"));
            auto resTagsAna = db["realtimeData"].find(
                make_document(kvp("type", "analog"), kvp("origin", "supervised"),
                              kvp("protocolDestinations.protocolDestinationConnectionNumber",
                                  make_document(kvp("$ne", dnp3Conn.protocolConnectionNumber)))),
                opts);

            for (auto&& doc : resTagsAna)
            {
                // std::cout << bsoncxx::to_json(doc) << std::endl;
                ++lastG30Addr;
                if (lastG30Addr > 65535)
                {
                    Log.Log("Object address for analogs exceeds 65535");
                    break;
                }

                Log.Log("Creating destination for tag: " + getString(doc, "_id") + " " + getString(doc, "tag")
                        + " Dnp3Address: " + std::to_string(lastG30Addr));
                db["realtimeData"].update_one(
                    make_document(kvp("_id", getDouble(doc, "_id"))),
                    make_document(kvp(
                        "$push",
                        make_document(kvp(
                            "protocolDestinations",
                            make_document(
                                kvp("protocolDestinationConnectionNumber", (double)dnp3Conn.protocolConnectionNumber),
                                kvp("protocolDestinationCommonAddress", 30.0),
                                kvp("protocolDestinationObjectAddress", (double)lastG30Addr),
                                kvp("protocolDestinationASDU", 6.0), kvp("protocolDestinationCommandDuration", 0.0),
                                kvp("protocolDestinationCommandUseSBO", false),
                                kvp("protocolDestinationCommandDuration", 0.0), kvp("protocolDestinationKConv1", 1.0),
                                kvp("protocolDestinationKConv2", 0.0), kvp("protocolDestinationGroup", 0.0),
                                kvp("protocolDestinationHoursShift", 0.0)))))));
            }
        }

        // Specify what log levels to use. NORMAL is warning and above
        // You can add all the comms logging by uncommenting below.
        const auto logLevels = levels::NORMAL | levels::ALL_COMMS;

        // This is the main point of interaction with the stack
        // Allocate a single thread to the pool since this is a single outstation
        // Log messages to the console
        dnp3Conn.manager = new DNP3Manager(1, ConsoleLogger::Create());
        // Create a TCP server (listener)
        dnp3Conn.channel = std::shared_ptr<IChannel>(nullptr);

        try
        {
            std::string ipAddr;
            int port;
            std::stringstream ss(dnp3Conn.ipAddressLocalBind);
            std::getline(ss, ipAddr, ':');
            std::string portStr;
            std::getline(ss, portStr);
            port = std::stoi(portStr);
            dnp3Conn.channel
                = dnp3Conn.manager->AddTCPServer(dnp3Conn.name, logLevels, ServerAcceptMode::CloseExisting,
                                                 IPEndpoint(ipAddr, port), PrintingChannelListener::Create());
        }
        catch (const std::exception& e)
        {
            std::cerr << e.what() << '\n';
            return -1;
        }

        dnp3Conn.commandHandler = std::make_shared<MyCommandHandler>();

        // The main object for a outstation. The defaults are useable,
        // but understanding the options are important.
        OutstationStackConfig config(ConfigureDatabase());

        // Specify the maximum size of the event buffers
        config.outstation.eventBufferConfig = EventBufferConfig::AllTypes(dnp3Conn.serverQueueSize);

        // you can override an default outstation parameters here
        // in this example, we've enabled the oustation to use unsolicted reporting
        // if the master enables it
        config.outstation.params.allowUnsolicited = dnp3Conn.enableUnsolicited;

        // You can override the default link layer settings here
        // in this example we've changed the default link layer addressing
        config.link.LocalAddr = dnp3Conn.localLinkAddress;
        config.link.RemoteAddr = dnp3Conn.remoteLinkAddress;
        config.link.KeepAliveTimeout = TimeDuration::Max();

        // Create a new outstation with a log level, command handler, and
        // config info this	returns a thread-safe interface used for
        // updating the outstation's database.
        dnp3Conn.outstation = dnp3Conn.channel->AddOutstation(dnp3Conn.name, dnp3Conn.commandHandler, app, config);

        UpdateBuilder builder;
        string input = "";
        State state;

        state.value += 1;
        builder.Update(Analog(state.value), 0);

        AddUpdates(builder, state, input);
        dnp3Conn.outstation->Apply(builder.Build());

        // Enable the outstation and start communications
        dnp3Conn.outstation->Enable();
    }

    auto rtDataCollection = db["realtimeData"];
    mongocxx::options::change_stream options;
    options.full_document("updateLookup");
    mongocxx::pipeline pipeline;
    std::string pipelineStr = R"(
    {
        "$or": 
        [
            {
                "$and":
                  [
                    { "fullDocument.protocolDestinations": { "$ne": null } },
                    { "fullDocument.protocolDestinations.protocolDestinationConnectionNumber": { "$in": [__CONNS__] } },
                    { "updateDescription.updatedFields.sourceDataUpdate": { "$exists": false } },
                    { "operationType": "update" }
                  ]
            },
            { "operationType": "replace" }
        ]
    }
    )";

    connectionsListStr.pop_back();
    pipelineStr.replace(pipelineStr.find("__CONNS__"), std::string("__CONNS__").length(), connectionsListStr);
    pipeline.match((bsoncxx::from_json(pipelineStr)));

    auto changeStream = rtDataCollection.watch(pipeline, options);

    std::cout << "Watching for changes on collection: realtimeData..." << std::endl;
    while (true)
    {
        for (const auto& event : changeStream)
        {
            // std::cout << "Change detected: " << bsoncxx::to_json(event) << std::endl;

            auto fullDocument = event["fullDocument"].get_document().value;
            auto protocolDestinations = fullDocument["protocolDestinations"].get_array().value;
            for (const auto& el : protocolDestinations)
            {
                auto protocolDestination = el.get_document().value;
                auto protocolDestinationConnectionNumber
                    = (int)getDouble(protocolDestination, "protocolDestinationConnectionNumber");
                auto protocolDestinationCommonAddress
                    = (int)getDouble(protocolDestination, "protocolDestinationCommonAddress");
                auto protocolDestinationObjectAddress
                    = (int)getDouble(protocolDestination, "protocolDestinationObjectAddress");
                auto protocolDestinationASDU = (int)getDouble(protocolDestination, "protocolDestinationASDU");
                auto protocolDestinationCommandDuration
                    = (int)getDouble(protocolDestination, "protocolDestinationCommandDuration");
                auto protocolDestinationCommandUseSBO
                    = getBoolean(protocolDestination, "protocolDestinationCommandUseSBO");
                auto protocolDestinationKConv1 = getDouble(protocolDestination, "protocolDestinationKConv1");
                auto protocolDestinationKConv2 = getDouble(protocolDestination, "protocolDestinationKConv2");
                auto protocolDestinationGroup = (int)getDouble(protocolDestination, "protocolDestinationGroup");
                auto protocolDestinationHoursShift = getDouble(protocolDestination, "protocolDestinationHoursShift");
                for (auto& dnp3Conn : dnp3Connections)
                {
                    if (dnp3Conn.protocolConnectionNumber == protocolDestinationConnectionNumber)
                    {
                        if (protocolDestinationObjectAddress > 9)
                            continue;

                        Log.Log("Updating tag: " + getString(fullDocument, "_id") + " " + getString(fullDocument, "tag")
                                + " Dnp3Address: " + std::to_string(protocolDestinationObjectAddress));
                        UpdateBuilder builder;
                            if (protocolDestinationCommonAddress == 1)
                            {
                                builder.Update(Binary(
                                     getBoolean(fullDocument, "value"),
                                     Flags(static_cast<uint8_t>(BinaryQuality::ONLINE)), DNPTime()
                                ),
                                               protocolDestinationObjectAddress, EventMode::Force);
                            }
                            else if (protocolDestinationCommonAddress == 30)
                            {
                                builder.Update(Analog(getDouble(fullDocument, "value"),
                                                      Flags(static_cast<uint8_t>(AnalogQuality::ONLINE)), DNPTime()),
                                               protocolDestinationObjectAddress, EventMode::Force);
                            }
                            const auto updates = builder.Build();
                            dnp3Conn.outstation->Apply(updates);
                    }
                }
            }
        }
    }

    // variables used in example loop
    string input;
    State state;

    while (true)
    {
        std::cout << "Enter one or more measurement changes then press <enter>" << std::endl;
        std::cout << "c = counter, b = binary, d = doublebit, a = analog, o = octet string, 'quit' = exit" << std::endl;
        std::cin >> input;

        if (input == "quit")
            return 0; // DNP3Manager destructor cleanups up everything automatically
        else
        {
            // update measurement values based on input string
            UpdateBuilder builder;
            AddUpdates(builder, state, input);
            // outstation->Apply(builder.Build());
        }
    }

    return 0;
}

void AddUpdates(UpdateBuilder& builder, State& state, const std::string& arguments)
{
    /*
    for (const char& c : arguments)
    {
        switch (c)
        {
        case ('c'): {
            builder.Update(Counter(state.count), 0);
            ++state.count;
            break;
        }
        case ('f'): {
            builder.FreezeCounter(0, false);
            break;
        }
        case ('a'): {
            builder.Update(Analog(state.value), 0);
            state.value += 1;
            break;
        }
        case ('b'): {
            builder.Update(Binary(state.binary, Flags(0x01), app->Now()), 0);
            state.binary = !state.binary;
            break;
        }
        case ('d'): {
            builder.Update(DoubleBitBinary(state.dbit), 0);
            state.dbit
                = (state.dbit == DoubleBit::DETERMINED_OFF) ? DoubleBit::DETERMINED_ON : DoubleBit::DETERMINED_OFF;
            break;
        }
        case ('o'): {
            OctetString value(Buffer(&state.octetStringValue, 1));
            builder.Update(value, 0);
            state.octetStringValue += 1;
            break;
        }
        default:
        }
    }
    */
}