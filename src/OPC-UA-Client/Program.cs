﻿/* 
 * OPC-UA Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020-2024 - Ricardo L. Olsen
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
using System.IO;
using System.Text.Json;
using System.Threading;
using MongoDB.Bson;
using MongoDB.Driver;

namespace OPCUAClientDriver
{
    partial class MainClass
    {
        public static String CopyrightMessage = "{json:scada} OPC-UA Client Driver - Copyright 2021-2024 RLO";
        public static String ProtocolDriverName = "OPC-UA";
        public static String DriverVersion = "0.2.0";
        public static bool Active = false; // indicates this driver instance is the active node in the moment
        public static Int32 DataBufferLimit = 20000; // limit to start dequeuing and discarding data from the acquisition buffer
        public static Int32 BulkWriteLimit = 1250; // limit of each bulk write to mongodb
        //public static int OPCDefaultPublishingInterval = 2500;
        //public static int OPCDefaultSamplingInterval = 1000;
        //public static uint OPCDefaultQueueSize = 10;

        public static int Main(string[] args)
        {
            Log(CopyrightMessage);
            Log("Driver version " + DriverVersion);
            Log("Using UA-.NETStandard library from the OPC Foundation.");

            if (args.Length > 0) // first argument in number of the driver instance
            {
                int num;
                bool res = int.TryParse(args[0], out num);
                if (res) ProtocolDriverInstanceNumber = num;
            }
            if (args.Length > 1) // second argument is logLevel
            {
                int num;
                bool res = int.TryParse(args[1], out num);
                if (res) LogLevel = num;
            }

            string fname = JsonConfigFilePath;
            if (args.Length > 2) // third argument is config file name
            {
                if (File.Exists(args[2]))
                {
                    fname = args[2];
                }
            }
            if (!File.Exists(fname))
                fname = JsonConfigFilePathAlt;
            if (!File.Exists(fname))
            {
                Log("Missing config file " + JsonConfigFilePath);
                Environment.Exit(-1);
            }

            Log("Reading config file " + fname);
            string json = File.ReadAllText(fname);
            JSConfig = JsonSerializer.Deserialize<JSONSCADAConfig>(json);
            if (
                JSConfig.mongoConnectionString == "" ||
                JSConfig.mongoConnectionString == null
            )
            {
                Log("Missing MongoDB connection string in JSON config file " +
                fname);
                Environment.Exit(-1);
            }
            // Log("MongoDB connection string: " + JSConfig.mongoConnectionString);
            if (
                JSConfig.mongoDatabaseName == "" ||
                JSConfig.mongoDatabaseName == null
            )
            {
                Log("Missing MongoDB database name in JSON config file " +
                fname);
                Environment.Exit(-1);
            }
            Log("MongoDB database name: " + JSConfig.mongoDatabaseName);
            if (JSConfig.nodeName == "" || JSConfig.nodeName == null)
            {
                Log("Missing nodeName parameter in JSON config file " +
                fname);
                Environment.Exit(-1);
            }
            Log("Node name: " + JSConfig.nodeName);

            var Client = ConnectMongoClient(JSConfig);
            var DB = Client.GetDatabase(JSConfig.mongoDatabaseName);

            // read and process instances configuration
            var collinsts =
                DB
                    .GetCollection
                    <protocolDriverInstancesClass
                    >(ProtocolDriverInstancesCollectionName);
            var instances =
                collinsts
                    .Find(inst =>
                        inst.protocolDriver == ProtocolDriverName &&
                        inst.protocolDriverInstanceNumber ==
                        ProtocolDriverInstanceNumber &&
                        inst.enabled == true)
                    .ToList();
            var foundInstance = false;
            foreach (protocolDriverInstancesClass inst in instances)
            {
                if (
                    ProtocolDriverName == inst.protocolDriver &&
                    ProtocolDriverInstanceNumber ==
                    inst.protocolDriverInstanceNumber
                )
                {
                    foundInstance = true;
                    if (!inst.enabled)
                    {
                        Log("Driver instance [" +
                        ProtocolDriverInstanceNumber.ToString() +
                        "] disabled!");
                        Environment.Exit(-1);
                    }
                    Log("Instance: " +
                    inst.protocolDriverInstanceNumber.ToString());
                    var nodefound = false || inst.nodeNames.Length == 0;
                    foreach (var name in inst.nodeNames)
                    {
                        if (JSConfig.nodeName == name)
                        {
                            nodefound = true;
                        }
                    }
                    if (!nodefound)
                    {
                        Log("Node '" +
                        JSConfig.nodeName +
                        "' not found in instances configuration!");
                        Environment.Exit(-1);
                    }
                    DriverInstance = inst;
                    break;
                }
                break; // process just first result
            }
            if (!foundInstance)
            {
                Log("Driver instance [" +
                ProtocolDriverInstanceNumber +
                "] not found in configuration!");
                Environment.Exit(-1);
            }

            // read and process connections configuration for this driver instance
            var collconns =
                DB
                    .GetCollection
                    <OPCUA_connection>(ProtocolConnectionsCollectionName);
            var conns =
                collconns
                    .Find(conn =>
                        conn.protocolDriver == ProtocolDriverName &&
                        conn.protocolDriverInstanceNumber ==
                        ProtocolDriverInstanceNumber &&
                        conn.enabled == true)
                    .ToList();
            var collRtData =
                DB.GetCollection<rtData>(RealtimeDataCollectionName);

            foreach (OPCUA_connection isrv in conns)
            {
                if (isrv.autoCreateTags)
                {
                    // look for existing tags in this connections, missing tags will be inserted later when discovered
                    var results = collRtData.Find<rtData>(new BsonDocument {
                                        { "protocolSourceConnectionNumber", isrv.protocolConnectionNumber },
                                        { "origin", "supervised" }
                                    }).ToList();
                    for (int i = 0; i < results.Count; i++)
                    {
                        isrv.InsertedTags.Add(results[i].tag.ToString());
                    }
                }
                isrv.LastNewKeyCreated = 0;
                if (isrv.endpointURLs.Length < 1)
                {
                    Log("Missing remote endpoint URLs list!");
                    Environment.Exit(-1);
                }
                OPCUAconns.Add(isrv);
                Log(isrv.name.ToString() + " - New Connection");
            }
            if (OPCUAconns.Count == 0)
            {
                Log("No connections found!");
                Environment.Exit(-1);
            }

            // start thread to process redundancy control
            var thrMongoRedundacy =
                new Thread(() =>
                        ProcessRedundancyMongo(JSConfig));
            thrMongoRedundacy.Start();

            // start thread to update acquired data to database
            var thrMongo =
                new Thread(() =>
                        ProcessMongo(JSConfig));
            thrMongo.Start();

            // thrMongo.Priority = ThreadPriority.AboveNormal;

            // start thread to watch for commands in the database using a change stream
            Thread thrMongoCmd =
                new Thread(() =>
                        ProcessMongoCmd(JSConfig));
            thrMongoCmd.Start();

            Log("Setting up OPC-UA Connections & ASDU handlers...");
            var thrSrvs = new List<Thread>();
            foreach (OPCUA_connection srv in OPCUAconns)
            {
                srv.connection = new OPCUAClient(srv);
                srv.thrOPCStack = new Thread(() => srv.connection.Run());
                srv.thrOPCStack.Start();
            }

            Thread.Sleep(1000);

            do
            {
                foreach (OPCUA_connection srv in OPCUAconns)
                {
                    if (srv.connection.failed)
                    {
                        Log(srv.name.ToString() + " - Failed!");
                        srv.connection.Run();
                    }
                }

                Thread.Sleep(1000);

                if (!Console.IsInputRedirected)
                    if (Console.KeyAvailable)
                    {
                        if (Console.ReadKey().Key == ConsoleKey.Escape)
                        {
                            Log("Exiting application!");
                            Environment.Exit(0);
                        }
                        else
                            Log("Press 'Esc' key to terminate...");
                    }

            } while (true);

            // return (int)OPCUAClient.ExitCode;
        }
    }
}
