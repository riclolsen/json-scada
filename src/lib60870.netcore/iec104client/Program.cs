/* 
 * IEC 60870-5-104 Client Protocol driver for {json:scada}
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
using System.IO;
using System.Text.Json;
using System.Threading;
using System.Timers;
using MongoDB.Bson;
using MongoDB.Bson.Serialization.Attributes;
using MongoDB.Driver;
using lib60870;
using lib60870.CS101;
using lib60870.CS104;
using System.Security.Cryptography.X509Certificates;
using System.Text;

namespace Iec10XDriver
{
    partial class MainClass
    {
        public static String ProtocolDriverName = "IEC60870-5-104";
        public static String DriverVersion = "0.1.0";
        public static bool Active = false; // indicates this driver instance is the active node in the moment
        public static Int32 DataBufferLimit = 10000; // limit to start dequeuing and discarding data from the acquisition buffer

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
            [BsonDefaultValue("")]
            public string ipAddressLocalBind { get; set; }
            public string[] ipAddresses { get; set; }
            [BsonDefaultValue(1)]
            public int localLinkAddress { get; set; }
            [BsonDefaultValue(1)]
            public int remoteLinkAddress { get; set; }
            [BsonDefaultValue(300)]
            public int giInterval { get; set; }
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
            public int MaxQueueSize { get; set; }
            public Connection connection;
            public int CntGI;
            public int CntTestCommand;
            public ushort CntTestCommandSeq;
            public int CntTimeSync;
            public System.Timers.Timer TimerCnt;
        }

        public class rtData
        {
            public BsonDocument sourceDataUpdate { get; set; }
        }

        // This is a handler IEC10X connection events
        private static void ConnectionHandler(
            object parameter,
            ConnectionEvent connectionEvent
        )
        {
            var srv = IEC10Xconns[(int)parameter];
            switch (connectionEvent)
            {
                case ConnectionEvent.OPENED:
                    Log(srv.name + " - Connected");
                    break;
                case ConnectionEvent.CLOSED:
                    Log(srv.name + " - Connection closed ");
                    var Client = ConnectMongoClient(JSConfig);
                    var DB = Client.GetDatabase(JSConfig.mongoDatabaseName);
                    var collection = DB.GetCollection<rtData>(RealtimeDataCollectionName);
                    // update as invalid
                    Log("Invalidating points on connection " + srv.protocolConnectionNumber);
                    var filter =
                        new BsonDocument(new BsonDocument("protocolSourceConnectionNumber",
                            srv.protocolConnectionNumber));
                    var update =
                        new BsonDocument("$set", new BsonDocument{
                        {"invalid",  true},
                        {"timeTag", BsonValue.Create(DateTime.Now) },
                            });
                    var res = collection.UpdateManyAsync(filter, update);
                    break;
                case ConnectionEvent.STARTDT_CON_RECEIVED:
                    Log(srv.name + " - STARTDT CON received ");
                    break;
                case ConnectionEvent.STOPDT_CON_RECEIVED:
                    Log(srv.name + " - STOPDT CON received ");
                    break;
            }
        }

        static void Main(string[] args)
        {
            Log("{json:scada} IEC60870-5-104 Driver - Copyright 2020 RLO");
            Log("Driver version " + DriverVersion);
            Log("Using lib60870.NET version " +
            LibraryCommon.GetLibraryVersionString());

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
                if (isrv.ipAddresses.Length < 1)
                {
                    Log("Missing ipAddresses list!");
                    Environment.Exit(-1);
                }
                IEC10Xconns.Add(isrv);
                Log(isrv.name.ToString() + " - New Connection");
            }
            if (IEC10Xconns.Count == 0)
            {
                Log("No connections found!");
                Environment.Exit(-1);
            }

            // start thread to process redundancy control
            Thread thrMongoRedundacy =
                new Thread(() =>
                        ProcessRedundancyMongo(JSConfig));
            thrMongoRedundacy.Start();

            // start thread to update acquired data to database
            Thread thrMongo =
                new Thread(() =>
                        ProcessMongo(JSConfig));
            thrMongo.Start();

            // start thread to watch for commands in the database using a change stream
            Thread thrMongoCmd =
                new Thread(() =>
                        ProcessMongoCmd(JSConfig));
            thrMongoCmd.Start();

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
                var tcpPort = 2404;
                string[] ipAddrPort = srv.ipAddresses[0].Split(':');
                if (ipAddrPort.Length > 1)
                    if (int.TryParse(ipAddrPort[1], out _))
                        tcpPort = System.Convert.ToInt32(ipAddrPort[1]);
                var con =
                    new Connection(ipAddrPort[0],
                        tcpPort,
                        apcipars,
                        alpars);
                con.Parameters.OA = srv.localLinkAddress;
                srv.connection = con;
                srv.CntGI = srv.giInterval - 3;
                srv.CntTestCommand = srv.testCommandInterval - 1;
                srv.CntTimeSync = 0;
                srv.CntTestCommandSeq = 0;
                if (LogLevel >= LogLevelDebug)
                    con.DebugOutput = true;
                con.SetASDUReceivedHandler(AsduReceivedHandler, cntIecSrv);
                con.SetConnectionHandler(ConnectionHandler, cntIecSrv);

                // create timer to increment counters each second
                srv.TimerCnt = new System.Timers.Timer();
                srv.TimerCnt.Interval = 1000;
                srv.TimerCnt.Elapsed += (sender, e) => MyElapsedMethod(sender, e, srv);
                static void MyElapsedMethod(object sender, ElapsedEventArgs e, IEC10X_connection serv)
                {
                    if (serv.testCommandInterval > 0)
                        serv.CntTestCommand++;
                    if (serv.giInterval > 0)
                        serv.CntGI++;
                    if (serv.timeSyncInterval > 0)
                        serv.CntTimeSync++;
                }
                srv.TimerCnt.Enabled = true;

                cntIecSrv++;
            }
            Thread.Sleep(1000);

            do
            {
                foreach (IEC10X_connection srv in IEC10Xconns)
                {
                    if (Active)
                    {
                        if (srv.connection.IsRunning)
                        {
                            if (srv.giInterval > 0)
                            {
                                if (srv.CntGI >= srv.giInterval)
                                {
                                    Log("Send Interrogation Request");
                                    srv.CntGI = 0;
                                    srv
                                        .connection
                                        .SendInterrogationCommand(CauseOfTransmission
                                            .ACTIVATION,
                                        srv.remoteLinkAddress,
                                        QualifierOfInterrogation.STATION);
                                }
                            }
                            if (srv.testCommandInterval > 0)
                            {
                                if (srv.CntTestCommand >= srv.testCommandInterval)
                                {
                                    Log("Send Test Command");
                                    srv.CntTestCommand = 0;
                                    srv.CntTestCommandSeq++;
                                    srv.connection.SendTestCommandWithCP56Time2a(srv.remoteLinkAddress, srv.CntTestCommandSeq, new CP56Time2a(DateTime.Now));
                                }
                            }
                            if (srv.timeSyncInterval > 0)
                            {
                                if (srv.CntTimeSync >= srv.timeSyncInterval)
                                {
                                    Log("Send Clock Sync");
                                    srv.CntTimeSync = 0;
                                    srv.connection.SendClockSyncCommand(srv.remoteLinkAddress, new CP56Time2a(DateTime.Now));
                                }
                            }
                        }
                        else
                        {
                            srv.CntGI = srv.giInterval - 2;
                            srv.CntTestCommand = srv.testCommandInterval - 1;
                            srv.CntTimeSync = srv.timeSyncInterval;
                            srv.CntTestCommandSeq = 0;
                            srv.connection.Close();
                            srv.connection.Cancel();
                            try
                            {
                                srv.connection.Connect();
                            }
                            catch
                            {
                                Log("Error connecting " + srv.name);
                            }
                        }
                    }
                    else
                    { // Inactive                        
                        if (srv.connection.IsRunning)
                        {
                            srv.CntGI = srv.giInterval - 2;
                            srv.CntTestCommand = srv.testCommandInterval - 1;
                            srv.CntTimeSync = srv.timeSyncInterval;
                            srv.CntTestCommandSeq = 0;
                            srv.connection.Close();
                            srv.connection.Cancel();
                        }
                    }
                }
                Thread.Sleep(500);

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
            }
            while (true);

            /* Synchronize clock of the controlled station */
            //con.SendClockSyncCommand(1 /* CA */, new CP56Time2a(DateTime.Now));
        }
    }
}
