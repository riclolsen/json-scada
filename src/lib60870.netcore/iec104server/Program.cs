/* 
 * IEC 60870-5-104 Server Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020 - 2024 - Ricardo L. Olsen
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
using System.Collections.Concurrent;
using System.IO;
using System.Text.Json;
using System.Threading;
using System.Linq;
using MongoDB.Bson;
using MongoDB.Bson.Serialization;
using MongoDB.Bson.Serialization.Attributes;
using MongoDB.Driver;
using lib60870;
using lib60870.CS101;
using lib60870.CS104;
using System.Security.Cryptography.X509Certificates;

namespace Iec10XDriver
{
    partial class MainClass
    {
        public static String ProtocolDriverName = "IEC60870-5-104_SERVER";
        public static String DriverVersion = "0.2.1";
        public static MongoClient Client = null;
        public static Boolean IsMongoLive = false;
        public static Int32 timeToExpireCommandsWithTime = 20;

        public class InfoCA
        {
            public InformationObject io;
            public Int32 ca;
        }

        [BsonIgnoreExtraElements]
        public class
        IEC10X_connection // IEC 104 connection to RTU
        {
            public ObjectId Id { get; set; }
            [BsonDefaultValue("")]
            public string protocolDriver { get; set; }
            [BsonDefaultValue(1)]
            public int protocolDriverInstanceNumber { get; set; }
            [BsonDefaultValue(1)]
            public int protocolConnectionNumber { get; set; }
            [BsonDefaultValue("NO NAME")]
            public string name { get; set; }
            [BsonDefaultValue("SERVER NOT DESCRIPTED")]
            public string description { get; set; }
            [BsonDefaultValue(true)]
            public bool enabled { get; set; }
            [BsonDefaultValue(true)]
            public bool commandsEnabled { get; set; }
            [BsonDefaultValue("0.0.0.0:2404")]
            public string ipAddressLocalBind { get; set; }
            public string[] ipAddresses { get; set; }
            [BsonDefaultValue(1)]
            public int localLinkAddress { get; set; }
            [BsonDefaultValue(1)]
            public int remoteLinkAddress { get; set; }
            [BsonDefaultValue(0)]
            public int testCommandInterval { get; set; }
            [BsonDefaultValue(0)]
            public int timeSyncInterval { get; set; }
            [BsonDefaultValue(2)]
            public int sizeOfCOT { get; set; }
            [BsonDefaultValue(2)]
            public int sizeOfCA { get; set; }
            [BsonDefaultValue(3)]
            public int sizeOfIOA { get; set; }
            [BsonDefaultValue(12)]
            public int k { get; set; }
            [BsonDefaultValue(8)]
            public int w { get; set; }
            [BsonDefaultValue(10)]
            public int t0 { get; set; }
            [BsonDefaultValue(15)]
            public int t1 { get; set; }
            [BsonDefaultValue(10)]
            public int t2 { get; set; }
            [BsonDefaultValue(20)]
            public int t3 { get; set; }
            [BsonDefaultValue(true)]
            public bool serverModeMultiActive { get; set; }
            [BsonDefaultValue(2)]
            public int maxClientConnections { get; set; }
            [BsonDefaultValue(1000)]
            public int maxQueueSize { get; set; }
            [BsonDefaultValue("")]
            public string localCertFilePath { get; set; }
            [BsonDefaultValue("")]
            public string passphrase { get; set; }
            [BsonDefaultValue(new string[] { })]
            public string[] peerCertFilesPaths { get; set; }
            [BsonDefaultValue("")]
            public string rootCertFilePath { get; set; }
            [BsonDefaultValue(false)]
            public bool allowOnlySpecificCertificates { get; set; }
            [BsonDefaultValue(false)]
            public bool chainValidation { get; set; }
            public Server server;
            public ConcurrentQueue<InfoCA> infoCAQueue = new ConcurrentQueue<InfoCA>(); // data objects to send 
            public List<ClientConnection> clientConnections = new List<ClientConnection>();
        }

        [BsonIgnoreExtraElements]
        public class rtData
        {
            [BsonSerializer(typeof(BsonIntSerializer))]
            public BsonInt32 _id { get; set; }
            [BsonDefaultValue("")]
            public BsonString tag { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble value { get; set; }
            [BsonDefaultValue("")]
            public BsonString valueString { get; set; }
            [BsonDefaultValue(null)]
            public BsonDateTime timeTag { get; set; }
            [BsonDefaultValue(null)]
            public BsonDateTime timeTagAtSource { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean timeTagAtSourceOk { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean invalid { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean transient { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean substituted { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean overflow { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolSourceConnectionNumber { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolSourceCommonAddress { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolSourceObjectAddress { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolSourceASDU { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolSourceCommandDuration { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean protocolSourceCommandUseSBO { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(1)]
            public BsonDouble kconv1 { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble kconv2 { get; set; }
            [BsonDefaultValue(null)]
            public rtSourceDataUpdate sourceDataUpdate { get; set; }
            [BsonDefaultValue(null)]
            public rtDataProtocDest[] protocolDestinations { get; set; }
        }

        static void Main(string[] args)
        {
            Log("{json:scada} IEC60870-5-104 Server Driver - Copyright 2020-2024 RLO");
            Log("Driver version " + DriverVersion);
            Log("Using lib60870.NET version " +
            LibraryCommon.GetLibraryVersionString());

            if (
                args.Length > 0 // first argument in number of the driver instance
            )
            {
                int num;
                bool res = int.TryParse(args[0], out num);
                if (res) ProtocolDriverInstanceNumber = num;
            }
            if (
                args.Length > 1 // second argument is logLevel
            )
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

            // connect to MongoDB Database server
            Client = ConnectMongoClient(JSConfig);
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
                    <IEC10X_connection>(ProtocolConnectionsCollectionName);
            var conns =
                collconns
                    .Find(conn =>
                        conn.protocolDriver == ProtocolDriverName &&
                        conn.protocolDriverInstanceNumber ==
                        ProtocolDriverInstanceNumber &&
                        conn.enabled == true)
                    .ToList();
            foreach (IEC10X_connection isrv in conns)
            {
                IEC10Xconns.Add(isrv);
                Log(isrv.name.ToString());
            }
            if (IEC10Xconns.Count == 0)
            {
                Log("No connections found!");
                Environment.Exit(-1);
            }

            // start thread to dequeue iec data and send to connections
            Thread thrDeqIecInfo =
                new Thread(() =>
                        DequeueIecInfo());
            thrDeqIecInfo.Start();

            // start thread to watch for commands in the database using a change stream
            Thread thrMongoCS =
                new Thread(() =>
                        ProcessMongoCS(JSConfig));
            thrMongoCS.Start();

            Log("Setting up IEC Connections & ASDU handlers...");
            int cntIecSrv = 0;
            foreach (IEC10X_connection srv in IEC10Xconns)
            {
                var apcipars = new APCIParameters();
                apcipars.K = srv.k;
                apcipars.W = srv.w;
                apcipars.T0 = srv.t0;
                apcipars.T1 = srv.t1;
                apcipars.T2 = srv.t2;
                apcipars.T3 = srv.t3;
                var alpars = new ApplicationLayerParameters();
                alpars.SizeOfCOT = srv.sizeOfCOT;
                alpars.SizeOfCA = srv.sizeOfCA;
                alpars.SizeOfIOA = srv.sizeOfIOA;
                alpars.OA = srv.localLinkAddress;

                TlsSecurityInformation secInfo = null;
                if (srv.localCertFilePath != "")
                {
                    try
                    {
                        // Own certificate has to be a pfx file that contains the private key
                        X509Certificate2 ownCertificate = new X509Certificate2(srv.localCertFilePath, srv.passphrase, X509KeyStorageFlags.MachineKeySet);

                        // Create a new security information object to configure TLS
                        secInfo = new TlsSecurityInformation(null, ownCertificate);

                        // Add allowed server certificates - not required when AllowOnlySpecificCertificates == false
                        foreach (string peerCertFilePath in srv.peerCertFilesPaths)
                            secInfo.AddAllowedCertificate(new X509Certificate2(peerCertFilePath));

                        // Add a CA certificate to check the certificate provided by the server - not required when ChainValidation == false
                        secInfo.AddCA(new X509Certificate2(srv.rootCertFilePath));

                        // Check if the certificate is signed by a provided CA
                        secInfo.ChainValidation = srv.chainValidation;

                        // Check that the shown server certificate is in the list of allowed certificates
                        secInfo.AllowOnlySpecificCertificates = srv.allowOnlySpecificCertificates;
                    }
                    catch (Exception e)
                    {
                        Log(srv.name + " - Error configuring TLS certificates.");
                        Log(srv.name + " - " + e.Message);
                        Environment.Exit(1);
                    }
                }

                var server = new Server(apcipars, alpars, secInfo);
                srv.server = server;
                if (srv.serverModeMultiActive)
                    server.ServerMode = ServerMode.CONNECTION_IS_REDUNDANCY_GROUP;
                else
                    server.ServerMode = ServerMode.SINGLE_REDUNDANCY_GROUP;
                var localBindIpAddress = "0.0.0.0";
                var tcpPort = 2404;
                string[] ipAddrPort = srv.ipAddressLocalBind.Split(':');
                if (ipAddrPort.Length > 0)
                    if (ipAddrPort[0] != "")
                        localBindIpAddress = ipAddrPort[0];
                if (ipAddrPort.Length > 1)
                    if (int.TryParse(ipAddrPort[1], out _))
                        tcpPort = System.Convert.ToInt32(ipAddrPort[1]);
                server.SetLocalAddress(localBindIpAddress);
                server.SetLocalPort(tcpPort);
                //RedundancyGroup redGroup = new RedundancyGroup("catch all");
                //server.AddRedundancyGroup(redGroup);
                if (LogLevel >= LogLevelDebug)
                    server.DebugOutput = true;
                server.MaxQueueSize = srv.maxQueueSize;
                server.MaxOpenConnections = srv.maxClientConnections;
                Log(srv.name + " - Max Queue Size: " + server.MaxQueueSize);
                Log(srv.name + " - Max Client Connections: " + server.MaxOpenConnections);
                server.SetConnectionRequestHandler(
                    ConnectionRequestHandler,
                    cntIecSrv
                );
                server.SetConnectionEventHandler(
                    ConnectionEventHandler,
                    cntIecSrv
                );
                server.SetInterrogationHandler(
                    InterrogationHandler,
                    cntIecSrv
                );
                server.SetASDUHandler(AsduReceivedHandler, cntIecSrv);
                server.Start();

                Log(srv.name + " - New server listening on " + localBindIpAddress + ":" + tcpPort);
                cntIecSrv++;
            }
            Thread.Sleep(1000);
            bool running = true;
            Console.CancelKeyPress +=
                delegate (object sender, ConsoleCancelEventArgs e)
                {
                    e.Cancel = true;
                    running = false;
                };
            Log("Press [CTRL]+[C] to terminate...");

            do
            {
                try
                {
                    if (Client == null)
                    {
                        // retry connection
                        Client = new MongoClient(JSConfig.mongoConnectionString);
                        DB = Client.GetDatabase(JSConfig.mongoDatabaseName);
                    }
                    IsMongoLive =
                        DB
                            .RunCommandAsync((Command<BsonDocument>)
                            "{ping:1}")
                            .Wait(10000);
                    if (!IsMongoLive)
                        throw new Exception("Error on MongoDB connection ");
                    //foreach (IEC104_connection srv in IEC10Xconns)
                    //{
                    //}
                }
                catch (Exception e)
                { // Disconnects to retry after some time
                    Client = null;
                    Log("Exception Mongo");
                    Log(e);
                    Log(e
                        .ToString()
                        .Substring(0,
                        e.ToString().IndexOf(Environment.NewLine)));
                    System.Threading.Thread.Sleep(3000);
                }

                Thread.Sleep(1000);
            }
            while (running);
            Log("Exiting application!");
            Environment.Exit(0);

            /* Synchronize clock of the controlled station */
            //con.SendClockSyncCommand(1 /* CA */, new CP56Time2a(DateTime.Now));
        }
    }
}
