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

inline bool ends_with(std::string const& value, std::string const& ending)
{
    if (ending.size() > value.size())
        return false;
    return std::equal(ending.rbegin(), ending.rend(), value.rbegin());
}

bool isConnected(mongocxx::database& database)
{
    try
    {
        auto command = database.run_command(make_document(kvp("ping", 1)));
        return true;
    }
    catch (const std::exception& e)
    {
        // std::cerr << "Connection error: " << e.what() << std::endl;
        return false;
    }
}

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
        if (Log.GetLogLevel() >= Logger::LogLevel::Detailed)
            Log.Log("Error getting bson double value for key: " + key, Logger::LogLevel::Detailed);
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
        if (Log.GetLogLevel() >= Logger::LogLevel::Detailed)
            Log.Log("Error getting bson boolean value for key: " + key, Logger::LogLevel::Detailed);
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
        if (Log.GetLogLevel() >= Logger::LogLevel::Detailed)
            Log.Log("Error getting bson string value for key: " + key, Logger::LogLevel::Detailed);
    }
    return defaultVal;
}

static double getDate(const bsoncxx::document::view& doc, const std::string& key, double defaultVal = 0)
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
        case bsoncxx::type::k_date:
            return (double)value.get_date().value.count();
        default:
            return defaultVal;
        }
    }
    catch (const std::exception&)
    {
        if (Log.GetLogLevel() >= Logger::LogLevel::Detailed)
            Log.Log("Error getting bson date value for key: " + key, Logger::LogLevel::Detailed);
    }
    return defaultVal;
}

typedef struct
{
    int protocolConnectionNumber;
    std::string name;
    bool enabled;
    bool commandsEnabled;
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
    std::string portName;
    int baudRate;
    std::string parity;
    std::string stopBits;
    std::string handshake;
    std::string localCertFilePath;
    std::string privateKeyFilePath;
    std::string peerCertFilePath;
    std::string cipherList;
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
    mongocxx::client* mongoClient;
    DNP3Connection_t* dnp3Connection;
    std::string dbstrMongo;
    void Begin() override {}
    void End() override {}
    CommandStatus Select(const ControlRelayOutputBlock& command, uint16_t index) override
    {
        if (mongoClient == nullptr)
            return CommandStatus::DOWNSTREAM_FAIL;
        auto db = (*mongoClient)[dbstrMongo];
        auto rtDataCollection = db["realtimeData"];
        auto result = rtDataCollection.find_one(make_document(
            kvp("origin", "command"), kvp("type", "digital"),
            kvp("protocolDestinations.protocolDestinationConnectionNumber", dnp3Connection->protocolConnectionNumber),
            kvp("protocolDestinations.protocolDestinationCommonAddress", 12),
            kvp("protocolDestinations.protocolDestinationObjectAddress", index),
            kvp("protocolDestinations.protocolDestinationCommandUseSBO", true)));
        if (!result)
        {
            Log.Log(dnp3Connection->name
                    + (std::string) " - Tag not found in the database for ControlRelayOutputBlock index: "
                    + std::to_string(index));
            return CommandStatus::NOT_SUPPORTED;
        }
        if (!(*result)["enabled"].get_bool().value)
        {
            Log.Log(dnp3Connection->name
                    + (std::string) " - Tag disabled in the database for ControlRelayOutputBlock index: "
                    + std::to_string(index));
            return CommandStatus::BLOCKED;
        }
        return CommandStatus::SUCCESS;
    }
    CommandStatus Operate(const ControlRelayOutputBlock& command,
                          uint16_t index,
                          IUpdateHandler& handler,
                          OperateType opType) override
    {
        if (mongoClient == nullptr)
            return CommandStatus::DOWNSTREAM_FAIL;
        auto db = (*mongoClient)[dbstrMongo];
        auto rtDataCollection = db["realtimeData"];
        auto fullDocument = rtDataCollection.find_one(make_document(
            kvp("origin", "command"), kvp("type", "digital"),
            kvp("protocolDestinations.protocolDestinationConnectionNumber", dnp3Connection->protocolConnectionNumber),
            kvp("protocolDestinations.protocolDestinationCommonAddress", 12),
            kvp("protocolDestinations.protocolDestinationObjectAddress", index),
            kvp("protocolDestinations.protocolDestinationCommandUseSBO", true)));
        if (!fullDocument)
        {
            Log.Log(dnp3Connection->name
                    + (std::string) " - Tag not found in the database for ControlRelayOutputBlock index: "
                    + std::to_string(index));
            return CommandStatus::NOT_SUPPORTED;
        }
        if (!(*fullDocument)["enabled"].get_bool().value)
        {
            Log.Log(dnp3Connection->name
                    + (std::string) " - Tag disabled in the database for ControlRelayOutputBlock index: "
                    + std::to_string(index));
            return CommandStatus::BLOCKED;
        }
        if (command.opType == OperationType::NUL)
        {
            Log.Log(dnp3Connection->name + (std::string) " - ControlRelayOutputBlock index: " + std::to_string(index)
                    + (std::string) " - OperationType: NUL");
            return CommandStatus::FORMAT_ERROR;
        }
        if (command.tcc == TripCloseCode::NUL || command.tcc == TripCloseCode::RESERVED)
        {
            Log.Log(dnp3Connection->name + (std::string) " - ControlRelayOutputBlock index: " + std::to_string(index)
                    + (std::string) " - Invalid TripCloseCode!");
            return CommandStatus::FORMAT_ERROR;
        }
        auto protocolDestinations = (*fullDocument)["protocolDestinations"].get_array().value;
        for (const auto& el : protocolDestinations)
        {
            auto protocolDestination = el.get_document().value;
            auto protocolDestinationConnectionNumber
                = (int)getDouble(protocolDestination, "protocolDestinationConnectionNumber");
            if (dnp3Connection->protocolConnectionNumber != protocolDestinationConnectionNumber)
                continue;
            double dval;
            auto protocolDestinationKConv1 = getDouble(protocolDestination, "protocolDestinationKConv1");
            if (protocolDestinationKConv1 == -1.0)
            {
                if (command.tcc == TripCloseCode::CLOSE)
                    dval = 0.0;
                else
                    dval = 1.0;
            }
            else
            {
                if (command.tcc == TripCloseCode::CLOSE)
                    dval = 1.0;
                else
                    dval = 0.0;
            }

            auto commandsQueueCollection = db["commandsQueue"];
            auto res = rtDataCollection.insert_one(make_document(
                kvp("protocolSourceConnectionNumber", dnp3Connection->protocolConnectionNumber),
                kvp("protocolSourceCommonAddress", 12), kvp("protocolSourceObjectAddress", index),
                kvp("protocolSourceASDU", getDouble(protocolDestination, "protocolDestinationASDU")),
                kvp("protocolSourceCommandDuration", (double)(uint8_t)command.opType),
                kvp("protocolSourceCommandUseSBO", getDouble(protocolDestination, "protocolDestinationCommandUseSBO")),
                kvp("point_key", getDouble(*fullDocument, "_id")), kvp("tag", getString(*fullDocument, "tag")),
                kvp("value", dval), kvp("valueString", ""), kvp("originatorUserName", "DNP3 Server Driver"),
                kvp("originatorIpAddress", ""),
                kvp("timeTag", std::chrono::system_clock::now().time_since_epoch().count())));

            return CommandStatus::SUCCESS;
        }

        return CommandStatus::NOT_SUPPORTED;
    }
    CommandStatus Select(const AnalogOutputInt16& command, uint16_t index) override
    {
        AnalogOutputDouble64 cmd(command.value);
        return Select(cmd, index);
    }
    CommandStatus Operate(const AnalogOutputInt16& command,
                          uint16_t index,
                          IUpdateHandler& handler,
                          OperateType opType) override
    {
        AnalogOutputDouble64 cmd(command.value);
        return Operate(cmd, index, handler, opType);
    }
    CommandStatus Select(const AnalogOutputInt32& command, uint16_t index) override
    {
        AnalogOutputDouble64 cmd(command.value);
        return Select(cmd, index);
    }
    CommandStatus Operate(const AnalogOutputInt32& command,
                          uint16_t index,
                          IUpdateHandler& handler,
                          OperateType opType) override
    {
        AnalogOutputDouble64 cmd(command.value);
        return Operate(cmd, index, handler, opType);
    }
    CommandStatus Select(const AnalogOutputFloat32& command, uint16_t index) override
    {
        AnalogOutputDouble64 cmd(command.value);
        return Select(cmd, index);
    }
    CommandStatus Operate(const AnalogOutputFloat32& command,
                          uint16_t index,
                          IUpdateHandler& handler,
                          OperateType opType) override
    {
        AnalogOutputDouble64 cmd(command.value);
        return Operate(cmd, index, handler, opType);
    }
    CommandStatus Select(const AnalogOutputDouble64& command, uint16_t index) override
    {
        if (mongoClient == nullptr)
            return CommandStatus::DOWNSTREAM_FAIL;
        auto db = (*mongoClient)[dbstrMongo];
        auto rtDataCollection = db["realtimeData"];
        auto result = rtDataCollection.find_one(make_document(
            kvp("origin", "command"), kvp("type", "digital"),
            kvp("protocolDestinations.protocolDestinationConnectionNumber", dnp3Connection->protocolConnectionNumber),
            kvp("protocolDestinations.protocolDestinationCommonAddress", 12),
            kvp("protocolDestinations.protocolDestinationObjectAddress", index),
            kvp("protocolDestinations.protocolDestinationCommandUseSBO", true)));
        if (!result)
        {
            Log.Log(dnp3Connection->name + (std::string) " - Tag not found in the database for AnalogOutput index: "
                    + std::to_string(index));
            return CommandStatus::NOT_SUPPORTED;
        }
        if (!(*result)["enabled"].get_bool().value)
        {
            Log.Log(dnp3Connection->name
                    + (std::string) " - Tag disabled in the database for AnalogOutput index: " + std::to_string(index));
            return CommandStatus::BLOCKED;
        }
        return CommandStatus::SUCCESS;
    }
    CommandStatus Operate(const AnalogOutputDouble64& command,
                          uint16_t index,
                          IUpdateHandler& handler,
                          OperateType opType) override
    {
        if (mongoClient == nullptr)
            return CommandStatus::DOWNSTREAM_FAIL;
        auto db = (*mongoClient)[dbstrMongo];
        auto rtDataCollection = db["realtimeData"];
        auto fullDocument = rtDataCollection.find_one(make_document(
            kvp("origin", "command"), kvp("type", "digital"),
            kvp("protocolDestinations.protocolDestinationConnectionNumber", dnp3Connection->protocolConnectionNumber),
            kvp("protocolDestinations.protocolDestinationCommonAddress", 12),
            kvp("protocolDestinations.protocolDestinationObjectAddress", index),
            kvp("protocolDestinations.protocolDestinationCommandUseSBO", true)));
        if (!fullDocument)
        {
            Log.Log(dnp3Connection->name + (std::string) " - Tag not found in the database for AnalogOutput index: "
                    + std::to_string(index));
            return CommandStatus::NOT_SUPPORTED;
        }
        if (!(*fullDocument)["enabled"].get_bool().value)
        {
            Log.Log(dnp3Connection->name
                    + (std::string) " - Tag disabled in the database for AnalogOutput index: " + std::to_string(index));
            return CommandStatus::BLOCKED;
        }
        auto protocolDestinations = (*fullDocument)["protocolDestinations"].get_array().value;
        for (const auto& el : protocolDestinations)
        {
            auto protocolDestination = el.get_document().value;
            auto protocolDestinationConnectionNumber
                = (int)getDouble(protocolDestination, "protocolDestinationConnectionNumber");
            if (dnp3Connection->protocolConnectionNumber != protocolDestinationConnectionNumber)
                continue;

            double dval = command.value;
            auto protocolDestinationKConv1 = getDouble(protocolDestination, "protocolDestinationKConv1");
            auto protocolDestinationKConv2 = getDouble(protocolDestination, "protocolDestinationKConv2");
            if (protocolDestinationKConv1 != 1.0 || protocolDestinationKConv2 != 0.0)
            {
                dval = dval * protocolDestinationKConv1 + protocolDestinationKConv2;
            }

            auto commandsQueueCollection = db["commandsQueue"];
            auto res = rtDataCollection.insert_one(make_document(
                kvp("protocolSourceConnectionNumber", dnp3Connection->protocolConnectionNumber),
                kvp("protocolSourceCommonAddress", 12), kvp("protocolSourceObjectAddress", index),
                kvp("protocolSourceASDU", getDouble(protocolDestination, "protocolDestinationASDU")),
                kvp("protocolSourceCommandDuration", (double)0),
                kvp("protocolSourceCommandUseSBO", getDouble(protocolDestination, "protocolDestinationCommandUseSBO")),
                kvp("point_key", getDouble(*fullDocument, "_id")), kvp("tag", getString(*fullDocument, "tag")),
                kvp("value", dval), kvp("valueString", std::to_string(dval)),
                kvp("originatorUserName", "DNP3 Server Driver"), kvp("originatorIpAddress", ""),
                kvp("timeTag", std::chrono::system_clock::now().time_since_epoch().count())));

            return CommandStatus::SUCCESS;
        }
        return CommandStatus::NOT_SUPPORTED;
    };
};

opendnp3::DatabaseConfig database_by_sizes(uint16_t num_binary,
                                           uint16_t num_double_binary,
                                           uint16_t num_analog,
                                           uint16_t num_counter,
                                           uint16_t num_frozen_counter,
                                           uint16_t num_binary_output_status,
                                           uint16_t num_analog_output_status,
                                           uint16_t num_time_and_interval,
                                           uint16_t num_octet_string)
{
    opendnp3::DatabaseConfig config;

    for (uint16_t i = 0; i < num_binary; ++i)
    {
        config.binary_input[i] = {};
    }
    for (uint16_t i = 0; i < num_double_binary; ++i)
    {
        config.double_binary[i] = {};
    }
    for (uint16_t i = 0; i < num_analog; ++i)
    {
        config.analog_input[i] = {};
    }
    for (uint16_t i = 0; i < num_counter; ++i)
    {
        config.counter[i] = {};
    }
    for (uint16_t i = 0; i < num_frozen_counter; ++i)
    {
        config.frozen_counter[i] = {};
    }
    for (uint16_t i = 0; i < num_binary_output_status; ++i)
    {
        config.binary_output_status[i] = {};
    }
    for (uint16_t i = 0; i < num_analog_output_status; ++i)
    {
        config.analog_output_status[i] = {};
    }
    for (uint16_t i = 0; i < num_time_and_interval; ++i)
    {
        config.time_and_interval[i] = {};
    }
    for (uint16_t i = 0; i < num_octet_string; ++i)
    {
        config.octet_string[i] = {};
    }

    return config;
}

static opendnp3::Updates ConvertValue(const bsoncxx::document::view& doc,
                                      const bsoncxx::document::view& protocolDestination,
                                      EventMode eventMode = EventMode::Detect,
                                      double connectionHoursShift = 0.0)
{
    auto protocolDestinationCommonAddress = (int)getDouble(protocolDestination, "protocolDestinationCommonAddress");
    auto protocolDestinationObjectAddress = (int)getDouble(protocolDestination, "protocolDestinationObjectAddress");
    auto protocolDestinationASDU = (int)getDouble(protocolDestination, "protocolDestinationASDU");
    // auto protocolDestinationCommandDuration = (int)getDouble(protocolDestination,
    // "protocolDestinationCommandDuration"); auto protocolDestinationCommandUseSBO =
    // getBoolean(protocolDestination, "protocolDestinationCommandUseSBO");
    auto protocolDestinationKConv1 = getDouble(protocolDestination, "protocolDestinationKConv1");
    auto protocolDestinationKConv2 = getDouble(protocolDestination, "protocolDestinationKConv2");
    // auto protocolDestinationGroup = (int)getDouble(protocolDestination, "protocolDestinationGroup");
    auto protocolDestinationHoursShift = getDouble(protocolDestination, "protocolDestinationHoursShift");

    if (Log.GetLogLevel() >= Logger::LogLevel::Basic)
        Log.Log("Updating tag: " + getString(doc, "_id") + " " + getString(doc, "tag")
                    + " Dnp3Address: " + std::to_string(protocolDestinationObjectAddress),
                Logger::LogLevel::Basic);

    UpdateBuilder builder;
    uint8_t flags = 0;
    DoubleBit dbit;
    uint64_t utime = (uint64_t)getDate(doc, "timeTagAtSource");
    DNPTime dtime;
    bool bval;
    double dval;
    if (utime == 0)
    {
        utime = (uint64_t)getDate(doc, "timeTag");
        if (utime == 0)
            utime = std::chrono::system_clock::now().time_since_epoch().count();
        utime += (uint64_t)(protocolDestinationHoursShift * 3600000);
        utime += (uint64_t)(connectionHoursShift * 3600000);
        dtime = DNPTime(utime, TimestampQuality::INVALID);
    }
    else
    {
        utime += (uint64_t)(protocolDestinationHoursShift * 3600000);
        utime += (uint64_t)(connectionHoursShift * 3600000);
        if (getDate(doc, "timeTagAtSourceOk"))
            dtime = DNPTime(utime, TimestampQuality::SYNCHRONIZED);
        else
            dtime = DNPTime(utime, TimestampQuality::UNSYNCHRONIZED);
    }

    switch (protocolDestinationCommonAddress)
    {
    case 1:
    case 2:
        bval = getBoolean(doc, "value");
        if (protocolDestinationKConv1 == -1.0)
            bval = !bval;
        if (!getBoolean(doc, "invalid") && !getBoolean(doc, "transient"))
            flags = static_cast<uint8_t>(BinaryQuality::ONLINE);
        else
            flags = static_cast<uint8_t>(BinaryQuality::COMM_LOST);
        if (getBoolean(doc, "substituted"))
            flags |= static_cast<uint8_t>(BinaryQuality::LOCAL_FORCED);
        builder.Update(Binary(bval, Flags(flags), dtime), protocolDestinationObjectAddress, eventMode);
        break;
    case 3:
    case 4:
        bval = getBoolean(doc, "value");
        if (protocolDestinationKConv1 == -1.0)
            bval = !bval;
        if (getBoolean(doc, "transient"))
            dbit = bval ? DoubleBit::INTERMEDIATE : DoubleBit::INDETERMINATE;
        else
            dbit = bval ? DoubleBit::DETERMINED_ON : DoubleBit::DETERMINED_OFF;
        if (!getBoolean(doc, "invalid"))
            flags = static_cast<uint8_t>(DoubleBitBinaryQuality::ONLINE);
        else
            flags = static_cast<uint8_t>(DoubleBitBinaryQuality::COMM_LOST);
        if (getBoolean(doc, "substituted"))
            flags |= static_cast<uint8_t>(DoubleBitBinaryQuality::LOCAL_FORCED);
        builder.Update(DoubleBitBinary(dbit, Flags(flags), dtime), protocolDestinationObjectAddress, eventMode);
        break;
    case 21:
    case 23:
    //    builder.FreezeCounter(protocolDestinationObjectAddress, false, eventMode);
    case 20:
    case 22:
        dval = getDouble(doc, "value");
        if (protocolDestinationKConv1 != 1.0 || protocolDestinationKConv2 != 0.0)
        {
            dval = dval * protocolDestinationKConv1 + protocolDestinationKConv2;
        }
        if (!getBoolean(doc, "invalid") && !getBoolean(doc, "transient"))
            flags = static_cast<uint8_t>(CounterQuality::ONLINE);
        else
            flags = static_cast<uint8_t>(CounterQuality::COMM_LOST);
        if (getBoolean(doc, "substituted"))
            flags |= static_cast<uint8_t>(CounterQuality::LOCAL_FORCED);
        if (getBoolean(doc, "overflow"))
            flags |= static_cast<uint8_t>(CounterQuality::ROLLOVER);
        builder.Update(Counter((uint32_t)dval, Flags(flags), dtime), protocolDestinationObjectAddress, eventMode);
        break;
    case 10:
    case 11:
        bval = getBoolean(doc, "value");
        if (protocolDestinationKConv1 == -1.0)
            bval = !bval;
        if (!getBoolean(doc, "invalid") && !getBoolean(doc, "transient"))
            flags = static_cast<uint8_t>(BinaryOutputStatusQuality::ONLINE);
        else
            flags = static_cast<uint8_t>(BinaryOutputStatusQuality::COMM_LOST);
        if (getBoolean(doc, "substituted"))
            flags |= static_cast<uint8_t>(BinaryOutputStatusQuality::LOCAL_FORCED);
        builder.Update(BinaryOutputStatus(bval, Flags(flags), dtime), protocolDestinationObjectAddress, eventMode);
        break;
    case 40:
    case 42:
        dval = getDouble(doc, "value");
        if (protocolDestinationKConv1 != 1.0 || protocolDestinationKConv2 != 0.0)
        {
            dval = dval * protocolDestinationKConv1 + protocolDestinationKConv2;
        }
        if (!getBoolean(doc, "invalid") && !getBoolean(doc, "transient"))
            flags = static_cast<uint8_t>(AnalogOutputStatusQuality::ONLINE);
        else
            flags = static_cast<uint8_t>(AnalogOutputStatusQuality::COMM_LOST);
        if (getBoolean(doc, "substituted"))
            flags |= static_cast<uint8_t>(AnalogOutputStatusQuality::LOCAL_FORCED);
        if (getBoolean(doc, "overflow"))
            flags |= static_cast<uint8_t>(AnalogOutputStatusQuality::OVERRANGE);
        builder.Update(AnalogOutputStatus(dval, Flags(flags), dtime), protocolDestinationObjectAddress, eventMode);
        break;
    case 110:
    case 111:
        // convert from jsonValue if possible
        // builder.Update(OctetString{"012345test"}, protocolDestinationObjectAddress, eventMode);
        break;
    case 50:
    case 52:
        utime = (uint64_t)getDate(doc, "value");
        utime += (uint64_t)(protocolDestinationHoursShift * 3600000);
        utime += (uint64_t)(connectionHoursShift * 3600000);
        builder.Update(TimeAndInterval{DNPTime(utime, TimestampQuality::SYNCHRONIZED), 0, IntervalUnits::Seconds},
                       protocolDestinationObjectAddress);
        break;
    case 30:
    case 32:
    default:
        dval = getDouble(doc, "value");
        if (protocolDestinationKConv1 != 1.0 || protocolDestinationKConv2 != 0.0)
        {
            dval = dval * protocolDestinationKConv1 + protocolDestinationKConv2;
        }
        if (!getBoolean(doc, "invalid") && !getBoolean(doc, "transient"))
            flags = static_cast<uint8_t>(AnalogQuality::ONLINE);
        else
            flags = static_cast<uint8_t>(AnalogQuality::COMM_LOST);
        if (getBoolean(doc, "substituted"))
            flags |= static_cast<uint8_t>(AnalogQuality::LOCAL_FORCED);
        if (getBoolean(doc, "overflow"))
            flags |= static_cast<uint8_t>(AnalogQuality::OVERRANGE);
        builder.Update(Analog(dval, Flags(flags), dtime), protocolDestinationObjectAddress, eventMode);
        break;
    }
    const auto updates = builder.Build();

    return updates;
}

static void DefineGroupVar(const bsoncxx::document::view& doc,
                           const bsoncxx::document::view& protocolDestination,
                           opendnp3::DatabaseConfig& cfg)
{
    auto group = (int)getDouble(protocolDestination, "protocolDestinationCommonAddress");
    auto variation = (int)getDouble(protocolDestination, "protocolDestinationASDU");
    auto i = (int)getDouble(protocolDestination, "protocolDestinationObjectAddress");

    switch (group)
    {
    case 1:
    case 2:
        cfg.binary_input[i].svariation = StaticBinaryVariation::Group1Var2;
        cfg.binary_input[i].evariation = EventBinaryVariation::Group2Var2;
        break;
    case 3:
    case 4:
        cfg.double_binary[i].svariation = StaticDoubleBinaryVariation::Group3Var2;
        cfg.double_binary[i].evariation = EventDoubleBinaryVariation::Group4Var2;
        break;
    case 21:
    case 23:
        switch (variation)
        {
        default:
        case 1: // 32 bit flag
            cfg.frozen_counter[i].svariation = StaticFrozenCounterVariation::Group21Var1;
            cfg.frozen_counter[i].evariation = EventFrozenCounterVariation::Group23Var5;
            break;
        case 2: // 16 bit flag
            cfg.frozen_counter[i].svariation = StaticFrozenCounterVariation::Group21Var2;
            cfg.frozen_counter[i].evariation = EventFrozenCounterVariation::Group23Var6;
            break;
        case 3: // 32 bit flag delta
            cfg.frozen_counter[i].svariation = StaticFrozenCounterVariation::Group21Var1;
            cfg.frozen_counter[i].evariation = EventFrozenCounterVariation::Group23Var5;
            break;
        case 4: // 16 bit flag delta
            cfg.frozen_counter[i].svariation = StaticFrozenCounterVariation::Group21Var2;
            cfg.frozen_counter[i].evariation = EventFrozenCounterVariation::Group23Var6;
            break;
        case 5: // 32 bit flag time
            cfg.frozen_counter[i].svariation = StaticFrozenCounterVariation::Group21Var5;
            cfg.frozen_counter[i].evariation = EventFrozenCounterVariation::Group23Var5;
            break;
        case 6: // 16 bit flag time
            cfg.frozen_counter[i].svariation = StaticFrozenCounterVariation::Group21Var6;
            cfg.frozen_counter[i].evariation = EventFrozenCounterVariation::Group23Var6;
            break;
        case 7: // 32 bit flag time delta
            cfg.frozen_counter[i].svariation = StaticFrozenCounterVariation::Group21Var5;
            cfg.frozen_counter[i].evariation = EventFrozenCounterVariation::Group23Var5;
            break;
        case 8: // 16 bit flag time delta
            cfg.frozen_counter[i].svariation = StaticFrozenCounterVariation::Group21Var6;
            cfg.frozen_counter[i].evariation = EventFrozenCounterVariation::Group23Var6;
            break;
        case 9: // 32 bit w/o flag
            cfg.frozen_counter[i].svariation = StaticFrozenCounterVariation::Group21Var1;
            cfg.frozen_counter[i].evariation = EventFrozenCounterVariation::Group23Var5;
            break;
        case 10: // 16 bit w/o flag
            cfg.frozen_counter[i].svariation = StaticFrozenCounterVariation::Group21Var2;
            cfg.frozen_counter[i].evariation = EventFrozenCounterVariation::Group23Var6;
            break;
        case 11: // 32 bit w/o flag delta
            cfg.frozen_counter[i].svariation = StaticFrozenCounterVariation::Group21Var1;
            cfg.frozen_counter[i].evariation = EventFrozenCounterVariation::Group23Var5;
            break;
        case 12: // 16 bit w/o flag delta
            cfg.frozen_counter[i].svariation = StaticFrozenCounterVariation::Group21Var2;
            cfg.frozen_counter[i].evariation = EventFrozenCounterVariation::Group23Var6;
            break;
        }
        break;
    case 20:
    case 22:
        switch (variation)
        {
        default:
        case 1: // 32 bit flag
            cfg.counter[i].svariation = StaticCounterVariation::Group20Var1;
            cfg.counter[i].evariation = EventCounterVariation::Group22Var5;
            break;
        case 2: // 16 bit flag
            cfg.counter[i].svariation = StaticCounterVariation::Group20Var2;
            cfg.counter[i].evariation = EventCounterVariation::Group22Var6;
            break;
        case 3: // 32 bit flag delta
            cfg.counter[i].svariation = StaticCounterVariation::Group20Var1;
            cfg.counter[i].evariation = EventCounterVariation::Group22Var5;
            break;
        case 4: // 16 bit flag delta
            cfg.counter[i].svariation = StaticCounterVariation::Group20Var2;
            cfg.counter[i].evariation = EventCounterVariation::Group22Var6;
            break;
        case 5: // 32 bit w/o flag
            cfg.counter[i].svariation = StaticCounterVariation::Group20Var5;
            cfg.counter[i].evariation = EventCounterVariation::Group22Var5;
            break;
        case 6: // 16 bit w/o flag
            cfg.counter[i].svariation = StaticCounterVariation::Group20Var6;
            cfg.counter[i].evariation = EventCounterVariation::Group22Var6;
            break;
        case 7: // 32 bit w/o flag delta
            cfg.counter[i].svariation = StaticCounterVariation::Group20Var5;
            cfg.counter[i].evariation = EventCounterVariation::Group22Var5;
            break;
        case 8: // 16 bit w/o flag delta
            cfg.counter[i].svariation = StaticCounterVariation::Group20Var6;
            cfg.counter[i].evariation = EventCounterVariation::Group22Var6;
            break;
        }
        break;
    case 10:
    case 11:
        cfg.binary_output_status[i].svariation = StaticBinaryOutputStatusVariation::Group10Var2;
        cfg.binary_output_status[i].evariation = EventBinaryOutputStatusVariation::Group11Var2;
        break;
    case 40:
    case 42:
        switch (variation)
        {
        case 1: // 32 bit
            cfg.analog_output_status[i].svariation = StaticAnalogOutputStatusVariation::Group40Var1;
            cfg.analog_output_status[i].evariation = EventAnalogOutputStatusVariation::Group42Var3;
            break;
        case 2: // 16 bit
            cfg.analog_output_status[i].svariation = StaticAnalogOutputStatusVariation::Group40Var2;
            cfg.analog_output_status[i].evariation = EventAnalogOutputStatusVariation::Group42Var4;
            break;
        default:
        case 3: // single precision FP
            cfg.analog_output_status[i].svariation = StaticAnalogOutputStatusVariation::Group40Var3;
            cfg.analog_output_status[i].evariation = EventAnalogOutputStatusVariation::Group42Var7;
            break;
        case 4: // double precision FP
            cfg.analog_output_status[i].svariation = StaticAnalogOutputStatusVariation::Group40Var4;
            cfg.analog_output_status[i].evariation = EventAnalogOutputStatusVariation::Group42Var8;
            break;
        }
        break;
    case 110:
    case 111:
        cfg.octet_string[i].svariation = StaticOctetStringVariation::Group110Var0;
        cfg.octet_string[i].evariation = EventOctetStringVariation::Group111Var0;
        break;
    case 50:
    case 52:
        cfg.time_and_interval[i].svariation = StaticTimeAndIntervalVariation::Group50Var4;
        break;
    case 30:
    case 32:
    default:
        switch (variation)
        {
        case 1: // 32 bit
            cfg.analog_input[i].svariation = StaticAnalogVariation::Group30Var1;
            cfg.analog_input[i].evariation = EventAnalogVariation::Group32Var1;
            break;
        case 2: // 16 bit
            cfg.analog_input[i].svariation = StaticAnalogVariation::Group30Var2;
            cfg.analog_input[i].evariation = EventAnalogVariation::Group32Var2;
            break;
        case 3: // 32 bit w/o flag
            cfg.analog_input[i].svariation = StaticAnalogVariation::Group30Var3;
            cfg.analog_input[i].evariation = EventAnalogVariation::Group32Var3;
            break;
        case 4: // 16 bit w/o flag
            cfg.analog_input[i].svariation = StaticAnalogVariation::Group30Var4;
            cfg.analog_input[i].evariation = EventAnalogVariation::Group32Var4;
            break;
        case 5: // single precision FP
        default:
            cfg.analog_input[i].svariation = StaticAnalogVariation::Group30Var5;
            cfg.analog_input[i].evariation = EventAnalogVariation::Group32Var5;
            break;
        case 6: // double precision FP
            cfg.analog_input[i].svariation = StaticAnalogVariation::Group30Var6;
            cfg.analog_input[i].evariation = EventAnalogVariation::Group32Var6;
            break;
        case 7: // single precision FP
            cfg.analog_input[i].svariation = StaticAnalogVariation::Group30Var5;
            cfg.analog_input[i].evariation = EventAnalogVariation::Group32Var7;
            break;
        case 8: // double precision FP
            cfg.analog_input[i].svariation = StaticAnalogVariation::Group30Var6;
            cfg.analog_input[i].evariation = EventAnalogVariation::Group32Var8;
            break;
        }
        break;
    }
}

int __cdecl main(int argc, char* argv[])
{
    Log.Log(CopyrightMessage);
    Log.Log("Driver version: " + VersionStr);
    Log.Log("Using OpenDnp3 version 3.1.2");
    Log.Log("Usage: " + std::string(argv[0]) + " [ProtocolDriverInstanceNumber] [LogLevel] [ConfigurationFile]");

    auto app = DefaultOutstationApplication::Create();

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
            Log.Log("Conversion error: Invalid integer value for ProtocolDriverInstanceNumber");
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
            Log.Log("Conversion error: Invalid integer value for LogLevel");
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
    Log.Log("Connecting to MongoDB");
    mongocxx::client client(uri);
    auto db = client[dbstrMongo];

    while (!isConnected(db))
    {
        std::this_thread::sleep_for(std::chrono::seconds(5)); // Wait before reconnecting
        Log.Log("Connecting to MongoDB");
    }

    Log.Log("Connected to MongoDB");

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
        // std::cout << bsoncxx::to_json(doc) << std::endl;
        connectionsListStr += std::to_string((int)getDouble(doc, "protocolConnectionNumber")) + ",";

        DNP3Connection_t dnp3Connection{(int)getDouble(doc, "protocolConnectionNumber"),
                                        getString(doc, "name"),
                                        getBoolean(doc, "enabled"),
                                        getBoolean(doc, "commandsEnabled"),
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
                                        getString(doc, "connectionMode", "TCP PASSIVE"),
                                        getString(doc, "portName", ""),
                                        (int)getDouble(doc, "baudRate", 9600),
                                        getString(doc, "parity", "None"),
                                        getString(doc, "stopBits", "One"),
                                        getString(doc, "handshake", "None"),
                                        getString(doc, "localCertFilePath"),
                                        getString(doc, "privateKeyFilePath"),
                                        getString(doc, "peerCertFilePath"),
                                        getString(doc, "cipherList"),
                                        getBoolean(doc, "allowOnlySpecificCertificates"),
                                        getBoolean(doc, "allowTLSv10"),
                                        getBoolean(doc, "allowTLSv11"),
                                        getBoolean(doc, "allowTLSv12"),
                                        getBoolean(doc, "allowTLSv13"),
                                        nullptr,
                                        nullptr,
                                        nullptr,
                                        nullptr};

        for (auto& c : dnp3Connection.connectionMode)
            c = toupper(c);
        for (auto& c : dnp3Connection.parity)
            c = toupper(c);
        for (auto& c : dnp3Connection.stopBits)
            c = toupper(c);
        for (auto& c : dnp3Connection.handshake)
            c = toupper(c);

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
        Log.Log(dnp3Conn.name + std::string(" - Connection Number: ")
                + std::to_string(dnp3Conn.protocolConnectionNumber));

        if (dnp3Conn.autoCreateTags)
        {
            mongocxx::options::find opts;
            Log.Log("Auto Create Tags is enabled");
            // Create destination for tags on the DNP3 connection

            if (dnp3Conn.commandsEnabled)
            {
                // find the latest used object address for crob commands
                // DIGITAL COMMAND TAGS, will distribute as Group 12 VAR 1
                auto lastG12Addr = -1;

                // find tags with a destination linked to this connection
                opts.sort(bsoncxx::from_json(R"({"protocolDestinations.protocolDestinationObjectAddress": 1})"));
                auto resTagsG21 = db["realtimeData"].find(
                    make_document(kvp("type", "digital"), kvp("origin", "command"),
                                  kvp("protocolDestinations.protocolDestinationCommonAddress", 12),
                                  kvp("protocolDestinations.protocolDestinationConnectionNumber",
                                      dnp3Conn.protocolConnectionNumber)),
                    opts);
                for (auto&& docG1 : resTagsG21)
                {
                    // look in the protocolDestinations array for entry with the connection number
                    auto protocolDestinations = docG1["protocolDestinations"].get_array().value;
                    for (const auto& el : protocolDestinations)
                    {
                        auto protocolDestination = el.get_document().value;
                        if ((int)getDouble(protocolDestination, "protocolDestinationConnectionNumber")
                            == dnp3Conn.protocolConnectionNumber)
                        {
                            if (getDouble(protocolDestination, "protocolDestinationObjectAddress") > lastG12Addr)
                                lastG12Addr = (int)getDouble(protocolDestination, "protocolDestinationObjectAddress");
                        }
                    }
                }
                Log.Log(dnp3Conn.name + " - Last Group 12 Address: " + std::to_string(lastG12Addr));

                // look for tags without a destination linked to this connection
                opts.sort(bsoncxx::from_json(R"({"_id": 1})"));
                auto resTagsCrob = db["realtimeData"].find(
                    make_document(kvp("type", "digital"), kvp("origin", "command"),
                                  kvp("protocolDestinations.protocolDestinationConnectionNumber",
                                      make_document(kvp("$ne", dnp3Conn.protocolConnectionNumber)))),
                    opts);

                for (auto&& doc : resTagsCrob)
                {
                    if (dnp3Conn.topics.size() > 0) // check if topics are defined for the connection
                    {
                        bool found = false;
                        for (auto& topic : dnp3Conn.topics)
                            if (getString(doc, "group1").find(topic) != std::string::npos)
                                found = true;
                        if (!found)
                            continue; // skip this group if it does not match any topic
                    }

                    lastG12Addr++;
                    if (lastG12Addr > 65535)
                    {
                        Log.Log(dnp3Conn.name + " - Object address for crob commands exceeds 65535!");
                        break;
                    }
                    Log.Log(dnp3Conn.name + " - Creating destination for tag: " + getString(doc, "_id") + " "
                            + getString(doc, "tag") + " Dnp3Address: " + std::to_string(lastG12Addr));
                    if (doc["protocolDestinations"].type() == bsoncxx::type::k_null)
                    {
                        db["realtimeData"].update_one(make_document(kvp("_id", getDouble(doc, "_id"))),
                                                      bsoncxx::from_json(R"({ "$set": {"protocolDestinations": []}})"));
                    }
                    db["realtimeData"].update_one(
                        make_document(kvp("_id", getDouble(doc, "_id"))),
                        make_document(kvp(
                            "$push",
                            make_document(kvp(
                                "protocolDestinations",
                                make_document(
                                    kvp("protocolDestinationConnectionNumber",
                                        (double)dnp3Conn.protocolConnectionNumber),
                                    kvp("protocolDestinationCommonAddress", 12.0),
                                    kvp("protocolDestinationObjectAddress", (double)lastG12Addr),
                                    kvp("protocolDestinationASDU", 1.0), kvp("protocolDestinationCommandDuration", 0.0),
                                    kvp("protocolDestinationCommandUseSBO", false),
                                    kvp("protocolDestinationCommandDuration", 11.0),
                                    kvp("protocolDestinationKConv1", 1.0), kvp("protocolDestinationKConv2", 0.0),
                                    kvp("protocolDestinationGroup", 0.0),
                                    kvp("protocolDestinationHoursShift", 0.0)))))));
                }

                // find the latest used object address for analog output commands
                // ANALOG COMMAND TAGS, will distribute as Group 41 VAR 3
                auto lastG41Addr = -1;

                // find tags with a destination linked to this connection
                opts.sort(bsoncxx::from_json(R"({"protocolDestinations.protocolDestinationObjectAddress": 1})"));
                auto resTagsG41 = db["realtimeData"].find(
                    make_document(kvp("type", "analog"), kvp("origin", "command"),
                                  kvp("protocolDestinations.protocolDestinationCommonAddress", 41),
                                  kvp("protocolDestinations.protocolDestinationConnectionNumber",
                                      dnp3Conn.protocolConnectionNumber)),
                    opts);
                for (auto&& docG1 : resTagsG41)
                {
                    // look in the protocolDestinations array for entry with the connection number
                    auto protocolDestinations = docG1["protocolDestinations"].get_array().value;
                    for (const auto& el : protocolDestinations)
                    {
                        auto protocolDestination = el.get_document().value;
                        if ((int)getDouble(protocolDestination, "protocolDestinationConnectionNumber")
                            == dnp3Conn.protocolConnectionNumber)
                        {
                            if (getDouble(protocolDestination, "protocolDestinationObjectAddress") > lastG41Addr)
                                lastG41Addr = (int)getDouble(protocolDestination, "protocolDestinationObjectAddress");
                        }
                    }
                }
                Log.Log(dnp3Conn.name + " - Last Group 41 Address: " + std::to_string(lastG41Addr));

                // look for tags without a destination linked to this connection
                opts.sort(bsoncxx::from_json(R"({"_id": 1})"));
                auto resTagsAnalogCmd = db["realtimeData"].find(
                    make_document(kvp("type", "analog"), kvp("origin", "command"),
                                  kvp("protocolDestinations.protocolDestinationConnectionNumber",
                                      make_document(kvp("$ne", dnp3Conn.protocolConnectionNumber)))),
                    opts);

                for (auto&& doc : resTagsAnalogCmd)
                {
                    if (dnp3Conn.topics.size() > 0) // check if topics are defined for the connection
                    {
                        bool found = false;
                        for (auto& topic : dnp3Conn.topics)
                            if (getString(doc, "group1").find(topic) != std::string::npos)
                                found = true;
                        if (!found)
                            continue; // skip this group if it does not match any topic
                    }

                    lastG41Addr++;
                    if (lastG41Addr > 65535)
                    {
                        Log.Log(dnp3Conn.name + " - Object address for analog outputs exceeds 65535!");
                        break;
                    }
                    Log.Log(dnp3Conn.name + " - Creating destination for tag: " + getString(doc, "_id") + " "
                            + getString(doc, "tag") + " Dnp3Address: " + std::to_string(lastG41Addr));
                    if (doc["protocolDestinations"].type() == bsoncxx::type::k_null)
                    {
                        db["realtimeData"].update_one(make_document(kvp("_id", getDouble(doc, "_id"))),
                                                      bsoncxx::from_json(R"({ "$set": {"protocolDestinations": []}})"));
                    }
                    db["realtimeData"].update_one(
                        make_document(kvp("_id", getDouble(doc, "_id"))),
                        make_document(kvp(
                            "$push",
                            make_document(kvp(
                                "protocolDestinations",
                                make_document(
                                    kvp("protocolDestinationConnectionNumber",
                                        (double)dnp3Conn.protocolConnectionNumber),
                                    kvp("protocolDestinationCommonAddress", 41.0),
                                    kvp("protocolDestinationObjectAddress", (double)lastG41Addr),
                                    kvp("protocolDestinationASDU", 3.0), kvp("protocolDestinationCommandDuration", 0.0),
                                    kvp("protocolDestinationCommandUseSBO", false),
                                    kvp("protocolDestinationCommandDuration", 0.0),
                                    kvp("protocolDestinationKConv1", 1.0), kvp("protocolDestinationKConv2", 0.0),
                                    kvp("protocolDestinationGroup", 0.0),
                                    kvp("protocolDestinationHoursShift", 0.0)))))));
                }
            }

            // find the latest used object address for digitals group1
            // DIGITAL TAGS, will distribute as Group 1 VAR 2
            auto lastG1Addr = -1;

            // find tags with a destination linked to this connection
            opts.sort(bsoncxx::from_json(R"({"protocolDestinations.protocolDestinationObjectAddress": 1})"));
            auto resTagsG1
                = db["realtimeData"].find(make_document(kvp("type", "digital"), kvp("origin", "supervised"),
                                                        kvp("protocolDestinations.protocolDestinationCommonAddress", 1),
                                                        kvp("protocolDestinations.protocolDestinationConnectionNumber",
                                                            dnp3Conn.protocolConnectionNumber)),
                                          opts);
            for (auto&& docG1 : resTagsG1)
            {
                // look in the protocolDestinations array for entry with the connection number
                auto protocolDestinations = docG1["protocolDestinations"].get_array().value;
                for (const auto& el : protocolDestinations)
                {
                    auto protocolDestination = el.get_document().value;
                    if ((int)getDouble(protocolDestination, "protocolDestinationConnectionNumber")
                        == dnp3Conn.protocolConnectionNumber)
                    {
                        if (getDouble(protocolDestination, "protocolDestinationObjectAddress") > lastG1Addr)
                            lastG1Addr = (int)getDouble(protocolDestination, "protocolDestinationObjectAddress");
                    }
                }
            }
            Log.Log(dnp3Conn.name + " - Last Group 1 Address: " + std::to_string(lastG1Addr));

            // look for tags without a destination linked to this connection
            opts.sort(bsoncxx::from_json(R"({"_id": 1})"));
            auto resTagsDig = db["realtimeData"].find(
                make_document(kvp("type", "digital"), kvp("origin", "supervised"),
                              kvp("protocolDestinations.protocolDestinationConnectionNumber",
                                  make_document(kvp("$ne", dnp3Conn.protocolConnectionNumber)))),
                opts);

            for (auto&& doc : resTagsDig)
            {
                if (dnp3Conn.topics.size() > 0) // check if topics are defined for the connection
                {
                    bool found = false;
                    for (auto& topic : dnp3Conn.topics)
                        if (getString(doc, "group1").find(topic) != std::string::npos)
                            found = true;
                    if (!found)
                        continue; // skip this group if it does not match any topic
                }

                lastG1Addr++;
                if (lastG1Addr > 65535)
                {
                    Log.Log(dnp3Conn.name + " - Object address for digitals exceeds 65535!");
                    break;
                }
                Log.Log(dnp3Conn.name + " - Creating destination for tag: " + getString(doc, "_id") + " "
                        + getString(doc, "tag") + " Dnp3Address: " + std::to_string(lastG1Addr));
                if (doc["protocolDestinations"].type() == bsoncxx::type::k_null)
                {
                    db["realtimeData"].update_one(make_document(kvp("_id", getDouble(doc, "_id"))),
                                                  bsoncxx::from_json(R"({ "$set": {"protocolDestinations": []}})"));
                }
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
                                kvp("protocolDestinationASDU", 2.0), kvp("protocolDestinationCommandDuration", 0.0),
                                kvp("protocolDestinationCommandUseSBO", false),
                                kvp("protocolDestinationCommandDuration", 0.0), kvp("protocolDestinationKConv1", 1.0),
                                kvp("protocolDestinationKConv2", 0.0), kvp("protocolDestinationGroup", 0.0),
                                kvp("protocolDestinationHoursShift", 0.0)))))));
            }

            // find the latest used object address for analogs grooup 30
            // ANALOG TAGS, will distribute as Group 30 VAR 6 (double precision floating point)
            auto lastG30Addr = -1;

            // find tags with a destination linked to this connection
            opts.sort(bsoncxx::from_json(R"({"protocolDestinations.protocolDestinationObjectAddress": 1})"));
            auto resTagsG30 = db["realtimeData"].find(
                make_document(
                    kvp("type", "analog"), kvp("origin", "supervised"),
                    kvp("protocolDestinations.protocolDestinationCommonAddress", 30),
                    kvp("protocolDestinations.protocolDestinationConnectionNumber", dnp3Conn.protocolConnectionNumber)),
                opts);
            for (auto&& docG30 : resTagsG1)
            {
                // look in the protocolDestinations array for entry with the connection number
                auto protocolDestinations = docG30["protocolDestinations"].get_array().value;
                for (const auto& el : protocolDestinations)
                {
                    auto protocolDestination = el.get_document().value;
                    if ((int)getDouble(protocolDestination, "protocolDestinationConnectionNumber")
                        == dnp3Conn.protocolConnectionNumber)
                    {
                        if (getDouble(protocolDestination, "protocolDestinationObjectAddress") > lastG30Addr)
                            lastG30Addr = (int)getDouble(protocolDestination, "protocolDestinationObjectAddress");
                    }
                }
            }
            Log.Log(dnp3Conn.name + " - Last Group 30 Address: " + std::to_string(lastG30Addr));

            // look for tags without a destination linked to this connection
            opts.sort(bsoncxx::from_json(R"({"_id": 1})"));
            auto resTagsAna = db["realtimeData"].find(
                make_document(kvp("type", "analog"), kvp("origin", "supervised"),
                              kvp("protocolDestinations.protocolDestinationConnectionNumber",
                                  make_document(kvp("$ne", dnp3Conn.protocolConnectionNumber)))),
                opts);

            for (auto&& doc : resTagsAna)
            {
                if (dnp3Conn.topics.size() > 0) // check if topics are defined for the connection
                {
                    bool found = false;
                    for (auto& topic : dnp3Conn.topics)
                        if (getString(doc, "group1").find(topic) != std::string::npos)
                            found = true;
                    if (!found)
                        continue; // skip this group if it does not match any topic
                }

                lastG30Addr++;
                if (lastG30Addr > 65535)
                {
                    Log.Log(dnp3Conn.name + " - Object address for analogs exceeds 65535!");
                    break;
                }

                Log.Log(dnp3Conn.name + " - Creating destination for tag: " + getString(doc, "_id") + " "
                        + getString(doc, "tag") + " Dnp3Address: " + std::to_string(lastG30Addr));
                if (doc["protocolDestinations"].type() == bsoncxx::type::k_null)
                {
                    db["realtimeData"].update_one(make_document(kvp("_id", getDouble(doc, "_id"))),
                                                  bsoncxx::from_json(R"({ "$set": {"protocolDestinations": []}})"));
                }
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

        // Specify what log levels to use.
        auto logLevels = levels::NORMAL;
        switch (Log.GetLogLevel())
        {
        case Logger::LogLevel::NoLog:
            logLevels = levels::NOTHING;
            break;
        case Logger::LogLevel::Basic:
            logLevels = levels::NORMAL;
            break;
        case Logger::LogLevel::Detailed:
            logLevels = levels::ALL_APP_COMMS;
            break;
        case Logger::LogLevel::Debug:
            logLevels = levels::ALL;
            break;
        default:
            logLevels = levels::NORMAL;
            break;
        }

        // This is the main point of interaction with the stack
        // Allocate a single thread to the pool since this is a single outstation
        // Log messages to the console
        dnp3Conn.manager = new DNP3Manager(1, ConsoleLogger::Create());
        // Create a TCP server (listener)
        dnp3Conn.channel = std::shared_ptr<IChannel>(nullptr);

        try
        {
            if (dnp3Conn.connectionMode == "TCP ACTIVE" || dnp3Conn.connectionMode == "TCP PASSIVE"
                || dnp3Conn.connectionMode == "TLS ACTIVE" || dnp3Conn.connectionMode == "TLS PASSIVE"

            )
            {
                // look for the same channel config already created (multi-drop case)
                for (auto& conn : dnp3Connections)
                {
                    if ((conn.channel != nullptr))
                    {
                        if (ends_with(dnp3Conn.connectionMode, "ACTIVE") && dnp3Conn.ipAddresses == conn.ipAddresses)
                        {
                            dnp3Conn.channel = conn.channel;
                            break;
                        }
                        if (ends_with(dnp3Conn.connectionMode, "PASSIVE")
                            && dnp3Conn.ipAddressLocalBind == conn.ipAddressLocalBind)
                        {
                            dnp3Conn.channel = conn.channel;
                            break;
                        }
                    }
                }
                if (dnp3Conn.channel != nullptr)
                {
                    Log.Log(dnp3Conn.name + " - Reusing channel...");
                }
                else
                {
                    try
                    {
                        std::string ipAddrBind;
                        int port;
                        std::stringstream ss(dnp3Conn.ipAddressLocalBind);
                        std::getline(ss, ipAddrBind, ':');
                        std::string portBindStr;
                        std::getline(ss, portBindStr);
                        port = std::stoi(portBindStr);
                        std::string localAdapter = "0.0.0.0";
                        if (ipAddrBind != std::string())
                            localAdapter = ipAddrBind;
                        if (dnp3Conn.connectionMode == "TCP ACTIVE")
                        {
                            Log.Log(dnp3Conn.name + " - Creating TCP Active Client to " + ipAddrBind + ":"
                                    + portBindStr);
                            dnp3Conn.channel
                                = dnp3Conn.manager->AddTCPClient(dnp3Conn.name, logLevels, ChannelRetry::Default(),
                                                                 vector<IPEndpoint>{IPEndpoint(ipAddrBind, port)},
                                                                 localAdapter, PrintingChannelListener::Create());
                        }
                        else if (dnp3Conn.connectionMode == "TCP PASSIVE")
                        {
                            Log.Log(dnp3Conn.name + " - Creating TCP Passive Server on " + ipAddrBind + ":"
                                    + portBindStr);
                            dnp3Conn.channel = dnp3Conn.manager->AddTCPServer(
                                dnp3Conn.name, logLevels, ServerAcceptMode::CloseNew, IPEndpoint(ipAddrBind, port),
                                PrintingChannelListener::Create());
                        }
                        else if (dnp3Conn.connectionMode == "TLS ACTIVE")
                        {
                            if (dnp3Conn.localCertFilePath == std::string())
                            {
                                Log.Log(dnp3Conn.name + " - Missing localCertFilePath parameter, ignoring connection!");
                                continue;
                            }
                            Log.Log(dnp3Conn.name + " - Creating TLS Active Client to " + ipAddrBind + ":"
                                    + portBindStr);
                            auto tlsConfig
                                = TLSConfig(dnp3Conn.peerCertFilePath, dnp3Conn.localCertFilePath,
                                            dnp3Conn.privateKeyFilePath, dnp3Conn.allowTLSv10, dnp3Conn.allowTLSv11,
                                            dnp3Conn.allowTLSv12, dnp3Conn.allowTLSv13, dnp3Conn.cipherList);
                            dnp3Conn.channel = dnp3Conn.manager->AddTLSClient(
                                dnp3Conn.name, logLevels, ChannelRetry::Default(),
                                vector<IPEndpoint>{IPEndpoint(ipAddrBind, port)}, localAdapter, tlsConfig,
                                PrintingChannelListener::Create());
                        }
                        else
                        {
                            if (dnp3Conn.localCertFilePath == std::string())
                            {
                                Log.Log(dnp3Conn.name + " - Missing localCertFilePath parameter, ignoring connection!");
                                continue;
                            }
                            Log.Log(dnp3Conn.name + " - Creating TLS Passive Server on " + ipAddrBind + ":"
                                    + portBindStr);
                            auto tlsConfig
                                = TLSConfig(dnp3Conn.peerCertFilePath, dnp3Conn.localCertFilePath,
                                            dnp3Conn.privateKeyFilePath, dnp3Conn.allowTLSv10, dnp3Conn.allowTLSv11,
                                            dnp3Conn.allowTLSv12, dnp3Conn.allowTLSv13, dnp3Conn.cipherList);
                            dnp3Conn.channel = dnp3Conn.manager->AddTLSServer(
                                dnp3Conn.name, logLevels, ServerAcceptMode::CloseNew, IPEndpoint(ipAddrBind, port),
                                tlsConfig, PrintingChannelListener::Create());
                        }
                    }
                    catch (const std::exception& e)
                    {
                        Log.Log(dnp3Conn.name + " - Error creating TCP or TLS channel: " + e.what());
                        continue;
                    }
                }
            }
            else if (dnp3Conn.connectionMode == "SERIAL")
            {
                if (dnp3Conn.portName == std::string())
                {
                    Log.Log(dnp3Conn.name + " - Missing portName parameter, ignoring connection!");
                    continue;
                }
                // look for the same channel already created (multi-drop case)
                // if found one, just reuse it
                for (auto& conn : dnp3Connections)
                {
                    if ((conn.channel != nullptr))
                    {
                        if (dnp3Conn.portName == conn.portName)
                        {
                            dnp3Conn.channel = conn.channel;
                            break;
                        }
                    }
                }
                if (dnp3Conn.channel != nullptr)
                {
                    Log.Log(dnp3Conn.name + " - Reusing channel...");
                }
                else
                {
                    Log.Log(dnp3Conn.name + " - Creating Serial Port: " + dnp3Conn.portName);
                    try
                    {
                        auto settings = SerialSettings();
                        settings.deviceName = dnp3Conn.portName;
                        settings.baud = dnp3Conn.baudRate;
                        settings.dataBits = 8;
                        settings.stopBits = StopBits::One;
                        settings.parity = Parity::None;
                        settings.flowType = FlowControl::None;
                        settings.asyncOpenDelay = TimeDuration::Milliseconds((int64_t)dnp3Conn.asyncOpenDelay);

                        if (dnp3Conn.parity == "EVEN")
                            settings.parity = Parity::Even;
                        else if (dnp3Conn.parity == "ODD")
                            settings.parity = Parity::Odd;
                        if (dnp3Conn.stopBits == "1.5" || dnp3Conn.stopBits == "ONE.FIVE")
                            settings.stopBits = StopBits::Two;
                        if (dnp3Conn.stopBits == "2" || dnp3Conn.stopBits == "TWO")
                            settings.stopBits = StopBits::Two;
                        auto handshake = FlowControl::None;
                        if (dnp3Conn.handshake == "RTS" || dnp3Conn.handshake == "HARDWARE")
                            settings.flowType = FlowControl::Hardware;
                        else if (dnp3Conn.handshake == "XON" || dnp3Conn.handshake == "XON_XOFF"
                                 || dnp3Conn.handshake == "XONXOFF")
                            settings.flowType = FlowControl::XONXOFF;

                        dnp3Conn.channel
                            = dnp3Conn.manager->AddSerial(dnp3Conn.name, logLevels, ChannelRetry::Default(), settings,
                                                          PrintingChannelListener::Create());
                    }
                    catch (const std::exception& e)
                    {
                        Log.Log(dnp3Conn.name + " - Error creating serial channel: " + e.what());
                        continue;
                    }
                }
            }
            else if (dnp3Conn.connectionMode == "UDP")
            {
                if (dnp3Conn.ipAddressLocalBind == std::string())
                {
                    Log.Log(dnp3Conn.name + " - Missing ipAddressLocalBind parameter, ignoring connection!");
                    continue;
                }
                if (dnp3Conn.ipAddresses.size() == 0 || dnp3Conn.ipAddresses[0] == std::string())
                {
                    Log.Log(dnp3Conn.name + " - Invalid list of ipAddresses parameter, ignoring connection!");
                    continue;
                }
                // look for the same channel config already created (multi-drop case)
                for (auto& conn : dnp3Connections)
                {
                    if ((conn.channel != nullptr))
                    {
                        if (dnp3Conn.ipAddressLocalBind == conn.ipAddressLocalBind)
                        {
                            dnp3Conn.channel = conn.channel;
                            break;
                        }
                    }
                }
                if (dnp3Conn.channel != nullptr)
                {
                    Log.Log(dnp3Conn.name + " - Reusing channel...");
                }
                else
                {
                    Log.Log(dnp3Conn.name + " - Creating UDP channel...");
                    try
                    {
                        std::string ipAddrBind;
                        int port;
                        std::stringstream ss(dnp3Conn.ipAddressLocalBind);
                        std::getline(ss, ipAddrBind, ':');
                        std::string portBindStr;
                        std::getline(ss, portBindStr);
                        port = std::stoi(portBindStr);

                        std::string ipAddrRemote;
                        int portRemote;
                        std::stringstream ssr(dnp3Conn.ipAddresses[0]);
                        std::getline(ssr, ipAddrRemote, ':');
                        std::string portRemoteStr;
                        std::getline(ssr, portRemoteStr);
                        portRemote = std::stoi(portRemoteStr);

                        dnp3Conn.channel = dnp3Conn.manager->AddUDPChannel(
                            dnp3Conn.name, logLevels, ChannelRetry::Default(), IPEndpoint(ipAddrBind, port),
                            IPEndpoint(ipAddrRemote, portRemote), PrintingChannelListener::Create());
                    }
                    catch (const std::exception& e)
                    {
                        Log.Log(dnp3Conn.name + " - Error creating UDP channel: " + e.what());
                        continue;
                    }
                }
            }
        }
        catch (const std::exception& e)
        {
            Log.Log(dnp3Conn.name + " - Error configuring connection: " + e.what());
            return -1;
        }

        if (dnp3Conn.channel == nullptr)
        {
            Log.Log(dnp3Conn.name + " - Error allocating channel!");
            continue;
        }

        dnp3Conn.commandHandler = std::make_shared<MyCommandHandler>();

        // find tags with a destination linked to this connection
        mongocxx::options::find opts;
        opts.sort(bsoncxx::from_json(R"({"protocolDestinations.protocolDestinationObjectAddress": 1})"));
        auto resTags
            = db["realtimeData"].find(make_document(kvp("origin", "supervised"),
                                                    kvp("protocolDestinations.protocolDestinationConnectionNumber",
                                                        dnp3Conn.protocolConnectionNumber)),
                                      opts);
        auto lastBinaryInput = -1;
        auto lastDoubleBinaryInput = -1;
        auto lastAnalogInput = -1;
        auto lastCounter = -1;
        auto lastFrozenCounter = -1;
        auto lastBinaryOuputStatus = -1;
        auto lastAnalogOutputStatus = -1;
        auto lastTimeAndInterval = -1;
        auto lastOctetString = -1;

        for (auto&& doc : resTags)
        {
            auto protocolDestinations = doc["protocolDestinations"].get_array().value;
            for (const auto& el : protocolDestinations)
            {
                const auto protocolDestination = el.get_document().value;
                const auto protocolDestinationConnectionNumber
                    = (int)getDouble(protocolDestination, "protocolDestinationConnectionNumber");
                const auto protocolDestinationObjectAddress
                    = (int)getDouble(protocolDestination, "protocolDestinationObjectAddress");

                if (dnp3Conn.protocolConnectionNumber != protocolDestinationConnectionNumber)
                    continue;

                switch ((int)getDouble(protocolDestination, "protocolDestinationCommonAddress"))
                {
                case 1:
                case 2:
                    if (protocolDestinationObjectAddress > lastBinaryInput)
                        lastBinaryInput = protocolDestinationObjectAddress;
                    break;
                case 3:
                case 4:
                    if (protocolDestinationObjectAddress > lastDoubleBinaryInput)
                        lastDoubleBinaryInput = protocolDestinationObjectAddress;
                    break;
                case 20:
                case 22:
                    if (protocolDestinationObjectAddress > lastCounter)
                        lastCounter = protocolDestinationObjectAddress;
                    break;
                case 21:
                case 23:
                    if (protocolDestinationObjectAddress > lastFrozenCounter)
                        lastFrozenCounter = protocolDestinationObjectAddress;
                    break;
                case 10:
                case 11:
                    if (protocolDestinationObjectAddress > lastBinaryOuputStatus)
                        lastBinaryOuputStatus = protocolDestinationObjectAddress;
                    break;
                case 30:
                case 32:
                    if (protocolDestinationObjectAddress > lastAnalogInput)
                        lastAnalogInput = protocolDestinationObjectAddress;
                    break;
                case 40:
                case 42:
                    if (protocolDestinationObjectAddress > lastAnalogOutputStatus)
                        lastAnalogOutputStatus = protocolDestinationObjectAddress;
                    break;
                case 50:
                case 52:
                    if (protocolDestinationObjectAddress > lastTimeAndInterval)
                        lastTimeAndInterval = protocolDestinationObjectAddress;
                    break;
                case 110:
                case 111:
                    if (protocolDestinationObjectAddress > lastOctetString)
                        lastOctetString = protocolDestinationObjectAddress;
                    break;
                }
            }
        }

        // The main object for a outstation. The defaults are useable,
        // but understanding the options are important.

        DatabaseConfig cfg = database_by_sizes(
            lastBinaryInput + 1, lastDoubleBinaryInput + 1, lastAnalogInput + 1, lastCounter + 1, lastFrozenCounter + 1,
            lastBinaryOuputStatus + 1, lastAnalogOutputStatus + 1, lastTimeAndInterval + 1, lastOctetString + 1);
        Log.Log(dnp3Conn.name + " - Outstation created with " + std::to_string(cfg.binary_input.size())
                + " binary inputs, " + std::to_string(cfg.double_binary.size()) + " double binary inputs, "
                + std::to_string(cfg.analog_input.size()) + " analog inputs, " + std::to_string(cfg.counter.size())
                + " counters, " + std::to_string(cfg.frozen_counter.size()) + " frozen counters, "
                + std::to_string(cfg.binary_output_status.size()) + " binary output statuses, "
                + std::to_string(cfg.analog_output_status.size()) + " analog output statuses, "
                + std::to_string(cfg.time_and_interval.size()) + " time and intervals, "
                + std::to_string(cfg.octet_string.size()) + " octet strings");
        for (int i = 0; i < cfg.binary_input.size(); i++)
        {
            cfg.binary_input[i].clazz = PointClass::Class1;
            cfg.binary_input[i].svariation = StaticBinaryVariation::Group1Var2;
            cfg.binary_input[i].evariation = EventBinaryVariation::Group2Var2;
        }
        for (int i = 0; i < cfg.double_binary.size(); i++)
        {
            cfg.double_binary[i].clazz = PointClass::Class2;
            cfg.double_binary[i].svariation = StaticDoubleBinaryVariation::Group3Var2;
            cfg.double_binary[i].evariation = EventDoubleBinaryVariation::Group4Var2;
        }
        for (int i = 0; i < cfg.analog_input.size(); i++)
        {
            cfg.analog_input[i].clazz = PointClass::Class2;
            cfg.analog_input[i].svariation = StaticAnalogVariation::Group30Var5;
            cfg.analog_input[i].evariation = EventAnalogVariation::Group32Var7;
            cfg.analog_input[i].deadband = 0;
        }
        for (int i = 0; i < cfg.counter.size(); i++)
        {
            cfg.counter[i].clazz = PointClass::Class2;
            cfg.counter[i].svariation = StaticCounterVariation::Group20Var1;
            cfg.counter[i].evariation = EventCounterVariation::Group22Var5;
            cfg.counter[i].deadband = 0;
        }
        for (int i = 0; i < cfg.frozen_counter.size(); i++)
        {
            cfg.frozen_counter[i].clazz = PointClass::Class3;
            cfg.frozen_counter[i].svariation = StaticFrozenCounterVariation::Group21Var1;
            cfg.frozen_counter[i].evariation = EventFrozenCounterVariation::Group23Var5;
            cfg.frozen_counter[i].deadband = 0;
        }
        for (int i = 0; i < cfg.binary_output_status.size(); i++)
        {
            cfg.binary_output_status[i].clazz = PointClass::Class2;
            cfg.binary_output_status[i].svariation = StaticBinaryOutputStatusVariation::Group10Var2;
            cfg.binary_output_status[i].evariation = EventBinaryOutputStatusVariation::Group11Var2;
        }
        for (int i = 0; i < cfg.analog_output_status.size(); i++)
        {
            cfg.analog_output_status[i].clazz = PointClass::Class2;
            cfg.analog_output_status[i].svariation = StaticAnalogOutputStatusVariation::Group40Var3;
            cfg.analog_output_status[i].evariation = EventAnalogOutputStatusVariation::Group42Var7;
            cfg.analog_output_status[i].deadband = 0;
        }
        for (int i = 0; i < cfg.time_and_interval.size(); i++)
        {
            cfg.time_and_interval[i].svariation = StaticTimeAndIntervalVariation::Group50Var4;
        }
        for (int i = 0; i < cfg.octet_string.size(); i++)
        {
            cfg.octet_string[i].clazz = PointClass::Class3;
            cfg.octet_string[i].svariation = StaticOctetStringVariation::Group110Var0;
            cfg.octet_string[i].evariation = EventOctetStringVariation::Group111Var0;
        }

        auto resTags1 = db["realtimeData"].find(make_document(
            kvp("origin", "supervised"),
            kvp("protocolDestinations.protocolDestinationConnectionNumber", dnp3Conn.protocolConnectionNumber)));
        for (auto&& doc : resTags1)
        {
            auto protocolDestinations = doc["protocolDestinations"].get_array().value;
            for (const auto& el : protocolDestinations)
            {
                auto protocolDestination = el.get_document().value;
                auto protocolDestinationConnectionNumber
                    = (int)getDouble(protocolDestination, "protocolDestinationConnectionNumber");

                if (dnp3Conn.protocolConnectionNumber != protocolDestinationConnectionNumber)
                    continue;
                DefineGroupVar(doc, protocolDestination, cfg);
            }
        }
        OutstationStackConfig config(cfg);

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

        auto resTags2 = db["realtimeData"].find(make_document(
            kvp("origin", "supervised"),
            kvp("protocolDestinations.protocolDestinationConnectionNumber", dnp3Conn.protocolConnectionNumber)));
        for (auto&& doc : resTags2)
        {
            auto protocolDestinations = doc["protocolDestinations"].get_array().value;
            for (const auto& el : protocolDestinations)
            {
                auto protocolDestination = el.get_document().value;
                auto protocolDestinationConnectionNumber
                    = (int)getDouble(protocolDestination, "protocolDestinationConnectionNumber");

                if (dnp3Conn.protocolConnectionNumber != protocolDestinationConnectionNumber)
                    continue;

                auto updates = ConvertValue(doc, protocolDestination, EventMode::Suppress);
                dnp3Conn.outstation->Apply(updates);
            }
        }

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

    Log.Log("Watching for changes on collection: realtimeData...");

    bsoncxx::v_noabi::document::view resumeToken;
    while (true)
    {
        try
        {
            Log.Log("Connecting to MongoDB");
            mongocxx::client client(uri);
            db = client[dbstrMongo];

            while (!isConnected(db))
            {
                std::this_thread::sleep_for(std::chrono::seconds(5)); // Wait before reconnecting
                Log.Log("Connecting to MongoDB");
            }

            Log.Log("Connected to MongoDB");

            auto rtDataCollection = db["realtimeData"];
            options.resume_after(resumeToken);
            auto changeStream = rtDataCollection.watch(pipeline, options);

            for (auto& dnp3Conn : dnp3Connections)
            {
                static_cast<MyCommandHandler*>(dnp3Conn.commandHandler.get())->mongoClient = &client;
                static_cast<MyCommandHandler*>(dnp3Conn.commandHandler.get())->dnp3Connection = &dnp3Conn;
                static_cast<MyCommandHandler*>(dnp3Conn.commandHandler.get())->dbstrMongo = dbstrMongo;

                // after a reconnection do an integrity updated
                Log.Log(dnp3Conn.name + " - Store integrity data.");
                auto resTags3 = db["realtimeData"].find(
                    make_document(kvp("origin", "supervised"),
                                  kvp("protocolDestinations.protocolDestinationConnectionNumber",
                                      dnp3Conn.protocolConnectionNumber)));
                for (auto&& doc : resTags3)
                {
                    auto protocolDestinations = doc["protocolDestinations"].get_array().value;
                    for (const auto& el : protocolDestinations)
                    {
                        auto protocolDestination = el.get_document().value;
                        auto protocolDestinationConnectionNumber
                            = (int)getDouble(protocolDestination, "protocolDestinationConnectionNumber");

                        if (dnp3Conn.protocolConnectionNumber != protocolDestinationConnectionNumber)
                            continue;

                        auto updates = ConvertValue(doc, protocolDestination, EventMode::Detect);
                        dnp3Conn.outstation->Apply(updates);
                    }
                }
            }

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
                        for (auto& dnp3Conn : dnp3Connections)
                        {
                            if (dnp3Conn.protocolConnectionNumber == 0)
                                continue;
                            if (dnp3Conn.protocolConnectionNumber != protocolDestinationConnectionNumber)
                                continue;
                            if (dnp3Conn.outstation == nullptr)
                                continue;

                            auto updates = ConvertValue(fullDocument, protocolDestination, EventMode::Force);
                            dnp3Conn.outstation->Apply(updates);
                            break;
                        }
                    }
                }
            }
        }
        catch (const std::exception& e)
        {
            Log.Log("Mongo change stream - Exception: " + (std::string)e.what());
            std::this_thread::sleep_for(std::chrono::seconds(5)); // Wait before reconnecting
            continue;
        }
    }
    return 0;
}
