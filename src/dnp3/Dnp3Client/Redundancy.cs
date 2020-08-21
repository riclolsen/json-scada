/* 
 * DNP3 Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020 - Ricardo L. Olsen
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
using System.Threading;
using MongoDB.Bson;
using MongoDB.Bson.Serialization;
using MongoDB.Bson.Serialization.Serializers;
using MongoDB.Bson.Serialization.Attributes;
using MongoDB.Driver;
using System.ComponentModel.DataAnnotations;

namespace Dnp3Driver
{
    partial class MainClass
    {
        // This process monitor and updates redundancy control of the driver instance in mongodb
        static async void ProcessRedundancyMongo(JSONSCADAConfig jsConfig)
        {
            Thread.Sleep(5000);
            do
            {
                try
                {
                    var lastActiveNodeKeepAliveTimeTag = DateTime.MinValue;
                    var countKeepAliveUpdates = 0;
                    var countKeepAliveUpdatesLimit = 4;
                    var Client = ConnectMongoClient(jsConfig);
                    var DB = Client.GetDatabase(jsConfig.mongoDatabaseName);

                    // read and process instances configuration
                    var collinsts =
                        DB
                            .GetCollection
                            <protocolDriverInstancesClass
                            >(ProtocolDriverInstancesCollectionName);
                    var collconns =
                        DB
                            .GetCollection
                            <DNP3_connection
                            >(ProtocolConnectionsCollectionName);
                    do
                    {
                        bool isMongoLive =
                            DB
                                .RunCommandAsync((Command<BsonDocument>)
                                "{ping:1}")
                                .Wait(1000);
                        if (!isMongoLive)
                            throw new Exception("Error on MongoDB connection ");

                        var instances =
                            collinsts
                                .Find(inst =>
                                    inst.protocolDriver == ProtocolDriverName &&
                                    inst.protocolDriverInstanceNumber == ProtocolDriverInstanceNumber)
                                .ToList();
                        var foundinstance = false;
                        foreach (protocolDriverInstancesClass inst in instances)
                        {
                            foundinstance = true;

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

                            if (inst.activeNodeName == JSConfig.nodeName)
                            {
                                if (!Active) // will go active
                                {
                                    Log("Redundancy - ACTIVATING this Node!");
                                    foreach (DNP3_connection srv in DNP3conns)
                                    {                                       
                                        if (!(srv.master is null))
                                            if (!srv.isConnected)
                                                srv.master.Enable();
                                    }
                                }
                                Active = true;
                                countKeepAliveUpdates = 0;
                            }
                            else
                            {
                                if (Active) // will go inactive
                                {   // wait a random time
                                    Log("Redundancy - DEACTIVATING this Node (other node active)!");
                                    countKeepAliveUpdates = 0;
                                    foreach (DNP3_connection srv in DNP3conns)
                                    {
                                        if (!(srv.master is null))
                                            srv.master.Disable();
                                        srv.isConnected = false;
                                    }
                                    Random rnd = new Random();
                                    Thread.Sleep(rnd.Next(1000, 5000));
                                }
                                Active = false;
                                if (lastActiveNodeKeepAliveTimeTag == inst.activeNodeKeepAliveTimeTag)
                                {
                                    countKeepAliveUpdates++;
                                }
                                lastActiveNodeKeepAliveTimeTag = inst.activeNodeKeepAliveTimeTag;
                                if (countKeepAliveUpdates > countKeepAliveUpdatesLimit)
                                { // time exceeded, be active
                                    Log("Redundancy - ACTIVATING this Node!");
                                    Active = true;
                                    foreach (DNP3_connection srv in DNP3conns)
                                    {
                                        if (!(srv.master is null))
                                            if (!srv.isConnected)
                                                srv.master.Enable();
                                    }
                                }
                            }

                            if (Active)
                            {
                                Log("Redundancy - This node is active.");

                                // update keep alive time 
                                var filter1 =
                                    Builders<protocolDriverInstancesClass>
                                        .Filter
                                        .Eq(m => m.protocolDriver,
                                        ProtocolDriverName);
                                var filter2 =
                                    Builders<protocolDriverInstancesClass>
                                        .Filter
                                        .Eq(m => m.protocolDriverInstanceNumber,
                                        ProtocolDriverInstanceNumber);
                                var filter =
                                    Builders<protocolDriverInstancesClass>
                                        .Filter
                                        .And(filter1, filter2);

                                var update =
                                    Builders<protocolDriverInstancesClass>
                                        .Update
                                        .Set(m => m.activeNodeName, JSConfig.nodeName)
                                        .Set(m => m.activeNodeKeepAliveTimeTag, DateTime.Now);

                                var options =
                                    new FindOneAndUpdateOptions<protocolDriverInstancesClass, protocolDriverInstancesClass
                                    >();
                                options.IsUpsert = false;
                                await collinsts
                                    .FindOneAndUpdateAsync(filter, update, options);

                                // update statistics
                                foreach (DNP3_connection srv in DNP3conns)
                                {
                                    if (!(srv.channel is null))
                                    {
                                        var stats = srv.channel.GetChannelStatistics();
                                        var filt =
                                            new BsonDocument(new BsonDocument("protocolConnectionNumber",
                                                srv.protocolConnectionNumber));
                                        var upd =
                                            new BsonDocument("$set", new BsonDocument{
                                            {"stats", new BsonDocument{
                                                { "nodeName", JSConfig.nodeName },
                                                { "timeTag", BsonValue.Create(DateTime.Now) },
                                                { "isConnected", BsonBoolean.Create(srv.isConnected) },
                                                { "numBadLinkFrameRx", BsonDouble.Create(stats.NumBadLinkFrameRx) },
                                                { "NumBytesRx", BsonDouble.Create(stats.NumBytesRx) },
                                                { "NumBytesTx", BsonDouble.Create(stats.NumBytesTx) },
                                                { "NumClose", BsonDouble.Create(stats.NumClose) },
                                                { "NumCrcError", BsonDouble.Create(stats.NumCrcError) },
                                                { "NumLinkFrameRx", BsonDouble.Create(stats.NumLinkFrameRx) },
                                                { "NumLinkFrameTx", BsonDouble.Create(stats.NumLinkFrameTx) },
                                                { "NumOpen", BsonDouble.Create(stats.NumOpen) },
                                                { "NumOpenFail", BsonDouble.Create(stats.NumOpenFail) }
                                                }},
                                                });
                                        var res = collconns.UpdateOneAsync(filt, upd);
                                    }
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
                            if (Active) // will go inactive
                            {   // wait a random time
                                Log("Redundancy - DEACTIVATING this Node (no instance found)!");
                                countKeepAliveUpdates = 0;
                                Random rnd = new Random();
                                Thread.Sleep(rnd.Next(1000, 5000));
                                foreach (DNP3_connection srv in DNP3conns)
                                {
                                    if (!(srv.master is null))
                                        srv.master.Disable();
                                    srv.isConnected = false;
                                }
                            }
                            Active = false;
                        }

                        Thread.Sleep(5000);
                    }
                    while (true);
                }
                catch (Exception e)
                {
                    Log("Exception Mongo");
                    Log(e);
                    Log(e
                        .ToString()
                        .Substring(0,
                        e.ToString().IndexOf(Environment.NewLine)));
                    System.Threading.Thread.Sleep(3000);
                }
            }
            while (true);
        }
    }
}