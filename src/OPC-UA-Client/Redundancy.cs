/* 
 * OPC-UA Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020-2022 - Ricardo L. Olsen
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
using System.Threading.Tasks;
using MongoDB.Bson;
using MongoDB.Driver;
using MongoDB.Bson.Serialization.Serializers;
using MongoDB.Bson.Serialization;
using System.Linq;

namespace OPCUAClientDriver
{
    partial class MainClass
    {
        // This process monitor and updates redundancy control of the driver instance in mongodb
        static async void ProcessRedundancyMongo()
        {
            do
            {
                try
                {
                    var lastActiveNodeKeepAliveTimeTag = DateTime.MinValue;
                    var countKeepAliveUpdates = 0;
                    var countKeepAliveUpdatesLimit = 4;
                    var Client = ConnectMongoClient(JSConfig);
                    var DB = Client.GetDatabase(JSConfig.mongoDatabaseName);

                    // read and process instances configuration
                    var collinsts =
                        DB
                            .GetCollection
                            <protocolDriverInstancesClass
                            >(ProtocolDriverInstancesCollectionName);
                    do
                    {
                        var serializer = new BsonValueSerializer();
                        var resStatus =
                            DB
                                .RunCommand((Command<BsonDocument>)
                                "{ping:1}");
                                //.Wait(15000);
                        if (resStatus.Elements.Count() < 1 || resStatus.Elements.ElementAt(0).Value.AsDouble == 0)
                           throw new Exception("Error on MongoDB connection!");

                        var collconns =
                            DB
                                .GetCollection
                                <OPCUA_connection
                                >(ProtocolConnectionsCollectionName);
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

                            var nodefound = false || inst.nodeNames.Length == 0;
                            foreach (var name in inst.nodeNames)
                            {
                                if (MainClass.JSConfig.nodeName == name)
                                {
                                    nodefound = true;
                                }
                            }
                            if (!nodefound)
                            {
                                Log("Node '" +
                                MainClass.JSConfig.nodeName +
                                "' not found in instances configuration!");
                                Environment.Exit(-1);
                            }

                            if (inst.activeNodeName == MainClass.JSConfig.nodeName)
                            {
                                if (!Active) // will go active
                                    Log("Redundancy - ACTIVATING this Node!");
                                Active = true;
                                countKeepAliveUpdates = 0;
                            }
                            else
                            {
                                if (Active) // will go inactive
                                {   // wait a random time
                                    Log("Redundancy - DEACTIVATING this Node (other node active)!");
                                    countKeepAliveUpdates = 0;
                                    Random rnd = new Random();
                                    await Task.Delay(rnd.Next(1000, 5000));
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
                                }
                            }

                            if (Active)
                            {
                                Log($"Redundancy - This node is active. - Notification events: {CntNotificEvents} - Lost updates: {CntLostDataUpdates}");

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

                                // update statistics for connections
                                foreach (OPCUA_connection srv in OPCUAconns)
                                {
                                    if (!(srv.connection is null))
                                    {
                                        // var stats = srv.connection.GetStatistics();
                                        var filt =
                                            new BsonDocument(new BsonDocument("protocolConnectionNumber",
                                                srv.protocolConnectionNumber));
                                        var upd =
                                            new BsonDocument("$set", new BsonDocument{
                                            {"stats", new BsonDocument{
                                                { "nodeName", MainClass.JSConfig.nodeName },
                                                { "timeTag", BsonDateTime.Create(DateTime.Now) },
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
                                await Task.Delay(rnd.Next(1000, 5000));
                            }
                            Active = false;
                        }

                        await Task.Delay(5000);
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
                    await Task.Delay(3000);
                }
            }
            while (true);
        }
    }
}