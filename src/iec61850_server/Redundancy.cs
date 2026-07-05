/*
 * IEC 61850 Server driver for {json:scada} - process-level redundancy control.
 *
 * Elects the active driver instance among nodes listed in protocolDriverInstances
 * (same mechanism as the other JSON-SCADA C# drivers). Only the active node serves
 * IEC 61850 clients; the standby keeps the MMS server stopped so clients never read
 * stale data from a passive node.
 *
 * {json:scada} - Copyright (c) 2020-2024 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3. See <http://www.gnu.org/licenses/>.
 */

using System;
using System.Threading.Tasks;
using MongoDB.Bson;
using MongoDB.Driver;

namespace IEC61850_Server
{
    partial class MainClass
    {
        static async void ProcessRedundancyMongo(JSONSCADAConfig jsConfig)
        {
            do
            {
                try
                {
                    var lastActiveNodeKeepAliveTimeTag = DateTime.MinValue;
                    var countKeepAliveUpdates = 0;
                    var countKeepAliveUpdatesLimit = 4;
                    var Client = ConnectMongoClient(jsConfig);
                    var DB = Client.GetDatabase(jsConfig.mongoDatabaseName);

                    var collinsts =
                        DB.GetCollection<protocolDriverInstancesClass>(ProtocolDriverInstancesCollectionName);
                    var collconns =
                        DB.GetCollection<Iec61850ServerConnection>(ProtocolConnectionsCollectionName);

                    do
                    {
                        bool isMongoLive =
                            DB.RunCommandAsync((Command<BsonDocument>)"{ping:1}").Wait(1000);
                        if (!isMongoLive)
                            throw new Exception("Error on MongoDB connection ");

                        var instances =
                            collinsts.Find(inst =>
                                inst.protocolDriver == ProtocolDriverName &&
                                inst.protocolDriverInstanceNumber == ProtocolDriverInstanceNumber)
                                .ToList();

                        var foundinstance = false;
                        foreach (protocolDriverInstancesClass inst in instances)
                        {
                            foundinstance = true;

                            var nodefound = inst.nodeNames.Length == 0;
                            foreach (var name in inst.nodeNames)
                                if (JSConfig.nodeName == name) nodefound = true;
                            if (!nodefound)
                            {
                                Log("Node '" + JSConfig.nodeName + "' not found in instances configuration!");
                                Environment.Exit(-1);
                            }

                            if (inst.activeNodeName == JSConfig.nodeName)
                            {
                                if (!Active)
                                    Log("Redundancy - ACTIVATING this Node!");
                                Active = true;
                                countKeepAliveUpdates = 0;
                            }
                            else
                            {
                                if (Active)
                                {
                                    Log("Redundancy - DEACTIVATING this Node (other node active)!");
                                    countKeepAliveUpdates = 0;
                                    Random rnd = new Random();
                                    await Task.Delay(rnd.Next(1000, 5000));
                                }
                                Active = false;
                                if (lastActiveNodeKeepAliveTimeTag == inst.activeNodeKeepAliveTimeTag)
                                    countKeepAliveUpdates++;
                                lastActiveNodeKeepAliveTimeTag = inst.activeNodeKeepAliveTimeTag;
                                if (countKeepAliveUpdates > countKeepAliveUpdatesLimit)
                                {
                                    Log("Redundancy - ACTIVATING this Node!");
                                    Active = true;
                                }
                            }

                            if (Active)
                            {
                                var filter =
                                    Builders<protocolDriverInstancesClass>.Filter.And(
                                        Builders<protocolDriverInstancesClass>.Filter.Eq(m => m.protocolDriver, ProtocolDriverName),
                                        Builders<protocolDriverInstancesClass>.Filter.Eq(m => m.protocolDriverInstanceNumber, ProtocolDriverInstanceNumber));
                                var update =
                                    Builders<protocolDriverInstancesClass>.Update
                                        .Set(m => m.activeNodeName, JSConfig.nodeName)
                                        .Set(m => m.activeNodeKeepAliveTimeTag, DateTime.Now);
                                var options = new FindOneAndUpdateOptions<protocolDriverInstancesClass, protocolDriverInstancesClass> { IsUpsert = false };
                                await collinsts.FindOneAndUpdateAsync(filter, update, options);

                                // update connection statistics
                                if (srvConn != null)
                                {
                                    int openConns = 0;
                                    bool running = false;
                                    try
                                    {
                                        if (iedServer != null) { openConns = iedServer.GetNumberOfOpenConnections(); running = iedServer.IsRunning(); }
                                    }
                                    catch (Exception) { }
                                    var filt = new BsonDocument("protocolConnectionNumber", srvConn.protocolConnectionNumber);
                                    var upd = new BsonDocument("$set", new BsonDocument{
                                        {"stats", new BsonDocument{
                                            { "nodeName", JSConfig.nodeName },
                                            { "timeTag", BsonDateTime.Create(DateTime.Now) },
                                            { "isRunning", BsonBoolean.Create(running) },
                                            { "openConnections", openConns },
                                            { "pointsMapped", MapByTag.Count },
                                            { "updatesQueued", UpdateQueue.Count },
                                        }},
                                    });
                                    await collconns.UpdateOneAsync(filt, upd);
                                }
                            }
                            else
                            {
                                if (inst.activeNodeName != "")
                                    Log("Redundancy - This node is INACTIVE! Node '" + inst.activeNodeName + "' is active, wait...");
                                else
                                    Log("Redundancy - This node is INACTIVE! No node is active, wait...");
                            }
                            break; // process just first result
                        }

                        if (!foundinstance)
                        {
                            if (Active)
                            {
                                Log("Redundancy - DEACTIVATING this Node (no instance found)!");
                                countKeepAliveUpdates = 0;
                                Random rnd = new Random();
                                await Task.Delay(rnd.Next(1000, 5000));
                            }
                            Active = false;
                        }

                        await Task.Delay(5000);
                    }
                    while (!Shutdown);
                }
                catch (Exception e)
                {
                    Log("Exception Mongo");
                    Log(e);
                    await Task.Delay(3000);
                }
            }
            while (!Shutdown);
        }
    }
}
