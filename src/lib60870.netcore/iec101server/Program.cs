/* 
 * IEC 60870-5-101 Server Protocol driver for {json:scada}
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
using System.Collections.Generic;
using System.Collections.Concurrent;
using System.IO;
using System.IO.Ports;
using System.Text.Json;
using System.Threading;
using System.Linq;
using MongoDB.Bson;
using MongoDB.Bson.Serialization;
using MongoDB.Bson.Serialization.Attributes;
using MongoDB.Driver;
using lib60870;
using lib60870.CS101;
using lib60870.linklayer;

namespace Iec10XDriver
{
    partial class MainClass
    {
        public static String ProtocolDriverName = "IEC60870-5-101_SERVER";
        public static String DriverVersion = "0.1.0";
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
        IEC10X_connection // IEC 101 connection to RTU
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
            [BsonDefaultValue("COM1")]
            public string portName { get; set; }
            [BsonDefaultValue(9600)]
            public int baudRate { get; set; }
            [BsonDefaultValue("Even")]
            public string parity { get; set; }
            [BsonDefaultValue("One")]
            public string stopBits { get; set; }
            [BsonDefaultValue("None")]
            public string handshake { get; set; }
            [BsonDefaultValue(1000)]
            public int timeoutForACK { get; set; }
            [BsonDefaultValue(1000)]
            public int timeoutRepeat { get; set; }
            [BsonDefaultValue(200)]
            public int timeoutMessage { get; set; }
            [BsonDefaultValue(50)]
            public int timeoutCharacter { get; set; }
            [BsonDefaultValue(false)]
            public bool useSingleCharACK { get; set; }
            [BsonDefaultValue(1)]
            public int sizeOfLinkAddress { get; set; }
            [BsonDefaultValue(1)]
            public int sizeOfCOT { get; set; }
            [BsonDefaultValue(1)]
            public int sizeOfCA { get; set; }
            [BsonDefaultValue(2)]
            public int sizeOfIOA { get; set; }
            [BsonDefaultValue(1)]
            public int localLinkAddress { get; set; }
            [BsonDefaultValue(1)]
            public int remoteLinkAddress { get; set; }
            [BsonDefaultValue(true)]
            public bool serverModeMultiActive { get; set; }
            [BsonDefaultValue(2)]
            public int maxClientConnections { get; set; }
            [BsonDefaultValue(1000)]
            public int maxQueueSize { get; set; }
            public CS101Slave server;
            public ConcurrentQueue<InfoCA> infoCAQueue = new ConcurrentQueue<InfoCA>(); // data objects to send 
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
            [BsonDefaultValue(false)]
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
            Log("{json:scada} IEC60870-5-101 Server Driver - Copyright 2020 RLO");
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
                    var nodefound = false;
                    foreach ( var name in inst.nodeNames)
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
                TcpClientVirtualSerialPort virtualPort = null;
                SerialPort port = null;
                if (srv.portName.Contains(":"))
                {
                    var hostport = srv.portName.Split(":");
                    virtualPort = new TcpClientVirtualSerialPort(hostport[0], System.Convert.ToInt32(hostport[1]));
                    if (LogLevel >= LogLevelDebug)
                        virtualPort.DebugOutput = true;
                    virtualPort.Start();
                }
                else
                {
                    port = new SerialPort();
                    port.PortName = srv.portName;
                    port.BaudRate = srv.baudRate;
                    switch (srv.parity.ToLower())
                    {
                        default: // Even is the starndard parity for 101
                        case "even":
                            port.Parity = Parity.Even;
                            break;
                        case "none":
                            port.Parity = Parity.None;
                            break;
                        case "odd":
                            port.Parity = Parity.Odd;
                            break;
                        case "mark":
                            port.Parity = Parity.Mark;
                            break;
                        case "space":
                            port.Parity = Parity.Space;
                            break;
                    }
                    switch (srv.stopBits.ToLower())
                    {
                        default:
                        case "one":
                            port.StopBits = StopBits.One;
                            break;
                        case "one5":
                        case "onepointfive":
                            port.StopBits = StopBits.OnePointFive;
                            break;
                        case "two":
                            port.StopBits = StopBits.Two;
                            break;
                    }
                    switch (srv.handshake.ToLower())
                    {
                        default:
                        case "none":
                            port.Handshake = Handshake.None;
                            break;
                        case "xon":
                        case "xonxoff":
                            port.Handshake = Handshake.XOnXOff;
                            break;
                        case "rts":
                        case "requesttosend":
                            port.Handshake = Handshake.RequestToSend;
                            break;
                        case "rtsxon":
                        case "requesttosendxonxoff":
                            port.Handshake = Handshake.RequestToSendXOnXOff;
                            break;
                    }
                    port.Open();
                    port.DiscardInBuffer();
                }

                LinkLayerParameters llParameters = new LinkLayerParameters();
                llParameters.AddressLength = srv.sizeOfLinkAddress;
                llParameters.TimeoutForACK = srv.timeoutForACK;
                llParameters.TimeoutRepeat = srv.timeoutRepeat;
                llParameters.UseSingleCharACK = srv.useSingleCharACK;

                CS101Slave slave;
                if (port != null)
                {
                    slave = new CS101Slave(port, llParameters);
                }
                else
                {
                    slave = new CS101Slave(virtualPort, llParameters);
                }
                slave.Parameters.SizeOfCOT = srv.sizeOfCOT;
                slave.Parameters.SizeOfCA = srv.sizeOfCA;
                slave.Parameters.SizeOfIOA = srv.sizeOfIOA;
                slave.Parameters.OA = srv.localLinkAddress;
                if (LogLevel >= LogLevelDebug)
                   slave.DebugOutput = true;
                slave.LinkLayerAddress = srv.localLinkAddress;
                slave.LinkLayerMode = lib60870.linklayer.LinkLayerMode.UNBALANCED;
                slave.SetInterrogationHandler(InterrogationHandler, cntIecSrv);
                slave.SetUserDataQueueSizes(srv.maxQueueSize, srv.maxQueueSize);
                srv.server = slave;
                slave.SetASDUHandler(AsduReceivedHandler, cntIecSrv);
                // slave.Start();

                Log(srv.name + " - New server listening on " + srv.portName);
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

            int cnt = 1;
            do
            {
                try
                {
                    foreach (IEC10X_connection srv in IEC10Xconns)
                    {
                        srv.server.Run();
                    }

                    if (Client == null)
                    {
                        // retry connection
                        IsMongoLive = false;
                        Client = new MongoClient(JSConfig.mongoConnectionString);
                        DB = Client.GetDatabase(JSConfig.mongoDatabaseName);
                        IsMongoLive = true;
                    }

                    if ((cnt % 20) == 0) // each 1 second test mongo connection
                    {
                        IsMongoLive =
                            DB
                                .RunCommandAsync((Command<BsonDocument>)
                                "{ping:1}")
                                .Wait(1000);
                        if (!IsMongoLive)
                            throw new Exception("Error on MongoDB connection ");
                    }
                }
                catch (Exception e)
                { // Disconnects to retry after some time
                    IsMongoLive = false;
                    Client = null;
                    Log("Exception");
                    Log(e);
                    Log(e
                        .ToString()
                        .Substring(0,
                        e.ToString().IndexOf(Environment.NewLine)));
                    System.Threading.Thread.Sleep(100);
                }

                Thread.Sleep(50);
                cnt++;
            }
            while (running);
            Log("Exiting application!");
            Environment.Exit(0);
        }
    }
}
