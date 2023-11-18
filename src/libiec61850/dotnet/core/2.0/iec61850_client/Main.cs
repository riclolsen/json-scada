/* 
 * This software implements a IEC61850 driver for JSON SCADA.
 * Copyright - 2020-2023 - Ricardo Lastra Olsen
 * 
 * Requires libiec61850 from MZ Automation.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 * 
 */

using System;
using System.Threading;
using System.Text.Json;
using System.Globalization;
using System.IO;
using System.Linq;
using IEC61850.Common;
using MongoDB.Driver;
using MongoDB.Bson;

namespace IEC61850_Client
{
    partial class MainClass
    {
        public static String CopyrightMessage = "{json:scada} IEC61850 Client Driver - Copyright 2023 Ricardo Olsen";
        public static String ProtocolDriverName = "IEC61850";
        public static String DriverVersion = "0.1.5";
        public static bool Active = false; // indicates this driver instance is the active node in the moment
        public static Int32 DataBufferLimit = 20000; // limit to start dequeuing and discarding data from the acquisition buffer
        public static Int32 BulkWriteLimit = 1250; // limit of each bulk write to mongodb

        public static void Main(string[] args)
        {
            // var browse = false;

            CultureInfo ci = new CultureInfo("en-US");
            Thread.CurrentThread.CurrentCulture = ci;
            Thread.CurrentThread.CurrentUICulture = ci;

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
            Log(CopyrightMessage + " Version " + DriverVersion);
            Log("Log level: " + LogLevel);

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
                    var nodefound = false;
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

            // read and process connections configuration for this driver instance
            var collconns =
                DB
                    .GetCollection
                    <Iec61850Connection>(ProtocolConnectionsCollectionName);
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

            foreach (Iec61850Connection conn in conns)
            {
                Iec61850Connections.Add(conn);
                // look for existing tags in this connections, missing tags will be inserted later when discovered
                var results = collRtData.Find<rtData>(new BsonDocument {
                                        { "protocolSourceConnectionNumber",
                                         BsonDouble.Create(conn.protocolConnectionNumber)},
                                    }).ToList();

                for (int i = 0; i < results.Count; i++)
                {
                    if (conn.autoCreateTags)
                        conn.InsertedTags.Add(results[i].tag.ToString());
                    Enum.TryParse(results[i].protocolSourceCommonAddress.ToString().Trim().ToUpper(), out FunctionalConstraint fc);
                    Iec61850Entry entry = new Iec61850Entry()
                    {
                        path = results[i].protocolSourceObjectAddress.ToString().Trim(),
                        fc = fc,
                        js_tag = results[i].tag.ToString(),
                        
                    };
                    conn.entries[results[i].protocolSourceObjectAddress.ToString().Trim()+ results[i].protocolSourceCommonAddress.ToString().Trim().ToUpper()] = entry;
                }

                conn.LastNewKeyCreated = 0;
                if (conn.ipAddresses.Length < 1)
                {
                    Log("Missing remote endpoint URLs list!");
                    Environment.Exit(-1);
                }
                Log(conn.name.ToString() + " - New Connection");
                Thread t = new Thread(() => Process(conn));
                t.Start();
            }
            if (conns.Count == 0)
            {
                Log("No connections found!");
                Environment.Exit(-1);
            }

            Thread.Sleep(1000);
            do
            {
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
        }
    }
}
