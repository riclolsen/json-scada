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

    void Log(const std::string& message, LogLevel level = LogLevel::Basic)
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

DatabaseConfig ConfigureDatabase()
{
    DatabaseConfig config(10); // 10 of each type with default settings

    config.analog_input[0].clazz = PointClass::Class2;
    config.analog_input[0].svariation = StaticAnalogVariation::Group30Var5;
    config.analog_input[0].evariation = EventAnalogVariation::Group32Var7;

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

    string uristrMongo = jsonCfg["mongoConnectionString"];
    string dbstrMongo = jsonCfg["mongoDatabaseName"];
    string nodeName = jsonCfg["nodeName"];

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
    result = protocolDriverConnectionsCollection.find(
        make_document(kvp("protocolDriver", "DNP3_SERVER"),
                      kvp("protocolDriverInstanceNumber", ProtocolDriverInstanceNumber), kvp("enabled", true)));
    if (!result)
    {
        Log.Log("No protocol driver connections enabled found!");
        return -1;
    }
    // std::cout << bsoncxx::to_json(*result) << std::endl;

    // Specify what log levels to use. NORMAL is warning and above
    // You can add all the comms logging by uncommenting below.
    const auto logLevels = levels::NORMAL | levels::ALL_COMMS;

    // This is the main point of interaction with the stack
    // Allocate a single thread to the pool since this is a single outstation
    // Log messages to the console
    DNP3Manager manager(1, ConsoleLogger::Create());

    // Create a TCP server (listener)
    auto channel = std::shared_ptr<IChannel>(nullptr);
    try
    {
        channel = manager.AddTCPServer("server", logLevels, ServerAcceptMode::CloseExisting,
                                       IPEndpoint("0.0.0.0", 20000), PrintingChannelListener::Create());
    }
    catch (const std::exception& e)
    {
        std::cerr << e.what() << '\n';
        return -1;
    }

    auto CommandHandler = std::make_shared<MyCommandHandler>();

    // The main object for a outstation. The defaults are useable,
    // but understanding the options are important.
    OutstationStackConfig config(ConfigureDatabase());

    // Specify the maximum size of the event buffers
    config.outstation.eventBufferConfig = EventBufferConfig::AllTypes(100);

    // you can override an default outstation parameters here
    // in this example, we've enabled the oustation to use unsolicted reporting
    // if the master enables it
    config.outstation.params.allowUnsolicited = true;

    // You can override the default link layer settings here
    // in this example we've changed the default link layer addressing
    config.link.LocalAddr = 10;
    config.link.RemoteAddr = 1;
    config.link.KeepAliveTimeout = TimeDuration::Max();

    // Create a new outstation with a log level, command handler, and
    // config info this	returns a thread-safe interface used for
    // updating the outstation's database.
    auto outstation = channel->AddOutstation("outstation", CommandHandler, app, config);

    // Enable the outstation and start communications
    outstation->Enable();

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
            outstation->Apply(builder.Build());
        }
    }

    return 0;
}

void AddUpdates(UpdateBuilder& builder, State& state, const std::string& arguments)
{
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
            break;
        }
    }
}
