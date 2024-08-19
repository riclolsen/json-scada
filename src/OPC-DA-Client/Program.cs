/* 
 * OPC-DA Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2024 - Ricardo L. Olsen
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

using MongoDB.Bson;
using MongoDB.Driver;
using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Text.Json;
using System.Threading;
using System.Threading.Tasks;
using Technosoftware.DaAeHdaClient;
using Technosoftware.DaAeHdaClient.Da;

namespace OPCDAClientDriver
{
    partial class MainClass
    {
        public static string CopyrightMessage = "{json:scada} OPC-DA Client Driver - Copyright 2021-2024 RLO";
        public static string ProtocolDriverName = "OPC-DA";
        public static string DriverVersion = "0.1.0";
        public static bool Active = false; // indicates this driver instance is the active node in the moment
        public static int DataBufferLimit = 20000; // limit to start dequeuing and discarding data from the acquisition buffer
        public static int BulkWriteLimit = 1250; // limit of each bulk write to mongodb

        public static int HandleCnt = 0;
        public static void Main(string[] args)
        {
            Log(CopyrightMessage);
            Log("Driver version " + DriverVersion);
            Log("Using Technosoftware " + LicenseHandler.Product + " " + LicenseHandler.Version);
            Technosoftware.DaAeHdaClient.Com.ApplicationInstance.InitializeSecurity(Technosoftware.DaAeHdaClient.Com.ApplicationInstance.AuthenticationLevel.Integrity);
            ApplicationInstance.EnableTrace(ApplicationInstance.GetLogFileDirectory(), "SampleClients.HDa.log");

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
                    <OPCDA_connection>(ProtocolConnectionsCollectionName);
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

            // LogLevel = LogLevelDetailed; // <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<

            foreach (OPCDA_connection srv in conns)
            {
                List<string> topics = new List<string>(srv.topics);
                // look for existing tags in this connections, if autotag enabled missing tags will be inserted later when discovered
                var results = collRtData.Find<rtData>(new BsonDocument {
                                        { "protocolSourceConnectionNumber", srv.protocolConnectionNumber },
                                        { "origin", "supervised" }
                                    }).Sort(new BsonDocument {
                                        { "_id", 1 },
                                    }).ToList();

                // put in branches names found in topics from connection plus protocolSourceCommonAddress from tags
                foreach (var topic in srv.topics) srv.branches.Add(topic);
                srv.LastNewKeyCreated = AutoKeyMultiplier * srv.protocolConnectionNumber;
                for (int i = 0; i < results.Count; i++)
                {
                    if (results[i].protocolSourceCommonAddress.ToString() != string.Empty) srv.branches.Add(results[i].protocolSourceCommonAddress.ToString());
                    srv.InsertedTags.Add(results[i].tag.ToString());
                    srv.InsertedAddresses.Add(results[i].protocolSourceObjectAddress.ToString());
                    if (results[i]._id.ToDouble() > srv.LastNewKeyCreated)
                        srv.LastNewKeyCreated = results[i]._id.ToDouble();
                }
                if (srv.endpointURLs.Length < 1)
                {
                    Log("Missing remote endpoint URLs list!");
                    Environment.Exit(-1);
                }
                OPCDAconns.Add(srv);
                Log(srv.name.ToString() + " - New Connection");
            }
            if (OPCDAconns.Count == 0)
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

            // start thread to watch for commands in the database using a change stream
            Thread thrMongoCmd =
                new Thread(() =>
                        ProcessMongoCmd(JSConfig));
            thrMongoCmd.Start();

            var wasActive = false;
            while (!Active)
            {
                Thread.Sleep(2000);
            }

            Log("Creating connections...");
            do
            {
                if (!Active) // handle redundant state active or inactive 
                {
                    if (wasActive) // when switched to inactive, disconnect servers
                    {
                        wasActive = false;
                        for (var i = 0; i < OPCDAconns.Count; i++)
                        {
                            var srv = OPCDAconns[i];
                            if (srv.connection != null)
                            {
                                srv.cancellationTokenSource.Cancel();
                                Thread.Sleep(100);
                                for (var j = 0; j < srv.subscriptions.Count; j++)
                                {
                                    srv.subscriptions[j].SetEnabled(false);
                                    srv.subscriptions[j].DataChangedEvent -= null;
                                }
                                srv.connection.Subscriptions.Clear();
                                srv.connection.Disconnect();
                                srv.connection.Dispose();
                                srv.connection = null;
                            }
                        }
                        Thread.Sleep(2000);
                        continue;
                    }
                }
                wasActive = true;

                for (var ii = 0; ii < OPCDAconns.Count; ii++)
                {
                    if (!Active) break;

                    var srv = OPCDAconns[ii];
                    if (srv.connection != null)
                    {
                        try
                        {
                            switch (srv.connection.GetServerStatus().ServerState)
                            {
                                //case OpcServerState.NeedsConfiguration:
                                //case OpcServerState.Initializing:
                                //    Log(srv.name.ToString() + "Status of Server is " + srv.connection.GetServerStatus().ServerState);
                                //    Thread.Sleep(1000);
                                //    continue;
                                case OpcServerState.Operational:
                                    Thread.Sleep(100);
                                    continue;
                                default:
                                    Log(srv.name.ToString() + "Status of Server is " + srv.connection.GetServerStatus().ServerState);
                                    // update as invalid
                                    Log("Invalidating points on connection " + srv.protocolConnectionNumber);
                                    var filter =
                                        new BsonDocument(new BsonDocument("protocolSourceConnectionNumber",
                                            srv.protocolConnectionNumber));
                                    var update =
                                        new BsonDocument("$set", new BsonDocument{
                                                {"invalid",  true},
                                                {"invalid",  true},
                                                {"timeTag", BsonValue.Create(DateTime.Now) },
                                            });
                                    var res = collRtData.UpdateManyAsync(filter, update);
                                    //groupState.Active = false;
                                    //group.ModifyState((int)TsCDaStateMask.Active, groupState);
                                    //group.Dispose();
                                    srv.cancellationTokenSource.Cancel();
                                    Thread.Sleep(100);
                                    srv.connection.Subscriptions.Clear();
                                    srv.connection.Disconnect();
                                    srv.connection.Dispose();
                                    srv.connection = null;
                                    break;
                            }
                        }
                        catch (OpcResultException e)
                        {
                            Log(srv.name + " - " + e.Message);
                        }
                        catch (Exception e)
                        {
                            Log(srv.name + " - " + e.Message);
                        }
                    }
                    if (!Active) break;

                    try
                    {
                        // "opcda://localhost/Matrikon.OPC.Simulation.1"; "opcda://localhost/SampleCompany.DaSample.30"; 
                        var serverUrl = srv.endpointURLs[srv.cntConnectRetries % srv.endpointURLs.Length];
                        srv.cntConnectRetries++;

                        var opcUrl = new OpcUrl(serverUrl);
                        if (opcUrl.Scheme != OpcUrlScheme.DA)
                        {
                            Log(srv.name + " - " + "Error! URL Scheme must be opcda://...");
                            Thread.Sleep(1000);
                            continue;
                        }
                        TsCDaServer daServer = new TsCDaServer(new Technosoftware.DaAeHdaClient.Com.Factory(), opcUrl);
                        daServer.SetClientName(srv.name);
                        srv.connection = daServer;

                        Log("Connecting to " + serverUrl);
                        // Connect to the server
                        OpcUserIdentity userIdentity = null;
                        if (srv.username != "")
                        {
                            var domainUser = srv.username.Split("/");
                            if (srv.useSecurity)
                            {
                                if (domainUser.Length > 1)
                                    userIdentity = new OpcUserIdentity(domainUser[0], domainUser[1], srv.password, srv.localCertFilePath, srv.peerCertFilePath);
                                else
                                    userIdentity = new OpcUserIdentity("", srv.username, srv.password, srv.localCertFilePath, srv.peerCertFilePath);
                            }
                            else
                            {
                                if (domainUser.Length > 1)
                                    userIdentity = new OpcUserIdentity(domainUser[0], domainUser[1], srv.password);
                                else
                                    userIdentity = new OpcUserIdentity(srv.username, srv.password);
                            }
                        }
                        daServer.Connect(new OpcConnectData(userIdentity));

                        Thread.Sleep(250);
                        switch (srv.connection.GetServerStatus().ServerState)
                        {
                            case OpcServerState.Operational:
                                break;
                            case OpcServerState.Initializing:
                                Log(srv.name.ToString() + srv.connection.GetServerStatus().ServerState);
                                Thread.Sleep(15000);
                                break;
                            default:
                                Log(srv.name.ToString() + srv.connection.GetServerStatus().ServerState);
                                Thread.Sleep(15000);
                                break;
                        }
                        switch (srv.connection.GetServerStatus().ServerState)
                        {
                            case OpcServerState.Operational:
                                break;
                            default:
                                Log(srv.name.ToString() + srv.connection.GetServerStatus().ServerState);
                                Thread.Sleep(1000);
                                continue;
                        }

                        // Get the status from the server
                        var status = daServer.GetServerStatus();
                        Log($"{srv.name} - Status of Server is {status.ServerState}");

                        var itemsBrowsed = new List<TsCDaItem>();
                        var itemsForGroup = new List<TsCDaItem>();
                        BrowseServer(ref daServer, null, ref itemsBrowsed, ref srv);

                        // will read only data wanted
                        for (int i = 0; i < itemsBrowsed.Count; i++)
                        {
                            //itemsBrowsed[i].Deadband = 10;
                            //itemsBrowsed[i].DeadbandSpecified = true;
                            //itemsBrowsed[i].SamplingRate = 10;
                            //itemsBrowsed[i].SamplingRateSpecified = true;
                            //itemsBrowsed[i].Active = true;
                            //itemsBrowsed[i].ActiveSpecified = true;
                            //itemsBrowsed[i].EnableBuffering = false;
                            //itemsBrowsed[i].EnableBufferingSpecified = true;
                            //itemsBrowsed[i].MaxAge = 0;
                            //itemsBrowsed[i].MaxAgeSpecified = true;

                            if (srv.InsertedAddresses.Contains(itemsBrowsed[i].ItemName))
                            {
                                itemsForGroup.Add(itemsBrowsed[i]);
                            }
                            else
                            {
                                if (srv.autoCreateTags)
                                {
                                    itemsForGroup.Add(itemsBrowsed[i]);
                                }
                            }
                        }

                        var listWrites = new List<WriteModel<rtData>>();

                        // Synchronous Read with server read function (DA 3.0) without a group
                        Log($"{srv.name} - Reading {itemsForGroup.Count} items...");
                        var itemValues = daServer.Read(itemsForGroup.ToArray());
                        processValueResults(ref srv, ref itemValues, ref collRtData, true);
                        var cntRemoved = 0;
                        for (var i = 0; i < itemValues.Length; i++) // remove items that can't be read from subscription
                        {
                            if (itemValues[i].Result.IsError())
                            {
                                for (int j = 0; j < itemsForGroup.Count; j++)
                                {
                                    if (itemValues[i].ItemName == itemsForGroup[j].ItemName)
                                    {
                                        itemsForGroup.RemoveAt(j);
                                        cntRemoved++;
                                        break;
                                    }
                                }
                            }
                        }
                        if (cntRemoved > 0)
                        {
                            Log($"{srv.name} - Removed {cntRemoved} items that can't be read from subscription.");
                        }

                        // Add a group with default values Active = true and UpdateRate = 500ms
                        var subscrState = new TsCDaSubscriptionState
                        {
                            Name = "JsonScadaGroup1",
                            UpdateRate = (int)srv.autoCreateTagPublishingInterval * 1000,
                            Deadband = (float)srv.deadBand,
                            // TimeBias = (int)(srv.hoursShift * 60)
                            // KeepAlive = (int)srv.giInterval/2,
                        };
                        var subscr = (TsCDaSubscription)daServer.CreateSubscription(subscrState);
                        var itemResults = subscr.AddItems(itemsForGroup.ToArray());
                        for (var i = 0; i < itemResults.Length; i++)
                        {
                            if (itemResults[i].Result.IsError())
                            {
                                Log($"{srv.name} - Item {itemResults[i].ItemName} could not be added to the subscription group");
                            }
                        }

                        subscr.DataChangedEvent += (object subscriptionHandle, object requestHandle, TsCDaItemValueResult[] values) =>
                        {
                            OnDataChangeEvent(subscriptionHandle, requestHandle, values, ref srv);
                        };
                        srv.subscriptions.Add(subscr);
                        if (srv.giInterval > 0) // do periodic general interrogations
                        {
                            Task.Run(async () =>
                            {
                                while (!srv.cancellationTokenSource.Token.IsCancellationRequested)
                                {
                                    await Task.Delay(TimeSpan.FromSeconds(srv.giInterval), srv.cancellationTokenSource.Token);
                                    Log(string.Format($"{srv.name} -  Read All: {subscr.Items.Length}"));
                                    var itemValues = daServer.Read(subscr.Items);
                                    processValueResults(ref srv, ref itemValues, ref collRtData, true);
                                }
                            }, srv.cancellationTokenSource.Token);
                        }
                    }
                    catch (OpcResultException e)
                    {
                        Log(srv.name + " - " + e.Message);
                        Thread.Sleep(1000);
                    }
                    catch (Exception e)
                    {
                        Log(srv.name + " - " + e.Message);
                        Thread.Sleep(1000);
                    }
                }

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
