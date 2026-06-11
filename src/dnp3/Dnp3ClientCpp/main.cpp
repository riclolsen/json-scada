/*
 * DNP3 Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * main.cpp - Entry point only. All implementation lives in the other .cpp files.
 */

#include "dnp3client.h"

int main(int argc, char* argv[])
{
    try
    {
        Log.log(DriverMessage);
        Log.log("Driver version " + DriverVersion);
        Log.log("Main: Starting driver...", Logger::Level::Detailed);

        bool cliLogLevelProvided = false;
        if (argc > 1)
            ProtocolDriverInstanceNumber = atoi(argv[1]);
        if (argc > 2)
        {
            const int logLevel = atoi(argv[2]);
            Log.setLevel(logLevel);
            cliLogLevelProvided = true;
            Log.log("Main: Log level set to " + to_string(logLevel), Logger::Level::Detailed);
        }

        string configPath = argc > 3 ? argv[3] : JsonConfigFilePath;
        configPath = resolvePath(configPath);
        if (!fileExists(configPath))
            configPath = resolvePath(JsonConfigFilePathAlt);
        JSConfig = loadJsonConfig(configPath);
        if (JSConfig.mongoConnectionString.empty()
            || JSConfig.mongoDatabaseName.empty()
            || JSConfig.nodeName.empty())
            throw runtime_error("Invalid JSON-SCADA configuration");

        loadConnections(!cliLogLevelProvided);
        if (cliLogLevelProvided)
            Log.log("Main: Keeping CLI log level override after loading instance configuration.",
                Logger::Level::Detailed);
        else
            Log.log("Main: Effective log level loaded from instance configuration.",
                Logger::Level::Detailed);

        auto manager     = make_shared<DNP3Manager>(2 * thread::hardware_concurrency(),
                                                    ConsoleLogger::Create());
        auto dnp3LogLevel = mapLogLevel();

        Log.log("Main: Creating DNP3 channels...", Logger::Level::Detailed);
        for (const auto& conn : snapshotConnections())
        {
            conn->channel = createChannel(manager, conn, dnp3LogLevel);
            Log.log("Main: Channel created for " + conn->name, Logger::Level::Detailed);
            configureMaster(conn);
            Log.log(conn->name + " - Connection configured.");
        }
        Log.log("Main: All connections configured, starting threads...", Logger::Level::Detailed);

        thread(processMongo).detach();
        Log.log("Main: processMongo thread started.");
        thread(processMongoCmd).detach();
        Log.log("Main: processMongoCmd thread started.");
        thread(processRedundancy).detach();
        Log.log("Main: processRedundancy thread started.");

        Log.log("Main: Entering main loop...");
        int loopCount = 0;
        for (;;)
        {
            this_thread::sleep_for(chrono::milliseconds(500));
            if (++loopCount % 120 == 0)
                Log.log("Main: Still running...", Logger::Level::Detailed);
        }
    }
    catch (const exception& ex)
    {
        Log.log("Fatal error: " + string(ex.what()));
        return 1;
    }
}
