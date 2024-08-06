/* 
 * DNP3 Client Protocol driver for {json:scada}
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
using System.Linq;
using System.IO;
using System.Text.Json;
using System.Threading;
using MongoDB.Bson;
using MongoDB.Bson.Serialization.Attributes;
using MongoDB.Driver;
using Automatak.DNP3.Adapter;
using Automatak.DNP3.Interface;

namespace Dnp3Driver
{
    partial class MainClass
    {
        static IDNP3Manager mgr;
        public static String DriverMessage = "{json:scada} DNP3 Client Driver - Copyright 2020-2024 RLO";
        public static String ProtocolDriverName = "DNP3";
        public static String DriverVersion = "0.1.5";
        public static uint CROB_PulseOnTime = 100;
        public static uint CROB_PulseOffTime = 100;
        public static bool Active = false; // indicates this driver instance is the active node in the moment
        public static Int32 DataBufferLimit = 10000; // limit to start dequeuing and discarding data from the acquisition buffer

        [BsonIgnoreExtraElements]
        public class
        RangeScans
        {
            [BsonDefaultValue(1)]
            public int group { get; set; }
            [BsonDefaultValue(1)]
            public int variation { get; set; }
            [BsonDefaultValue(0)]
            public int startAddress { get; set; }
            [BsonDefaultValue(0)]
            public int stopAddress { get; set; }
            [BsonDefaultValue(0)]
            public int period { get; set; }
        }

        [BsonIgnoreExtraElements]
        public class
        DNP3_connection // DNP3 connection to RTU
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
            [BsonDefaultValue("")]
            public string portName { get; set; }
            [BsonDefaultValue(9600)]
            public int baudRate { get; set; }
            [BsonDefaultValue("None")]
            public string parity { get; set; }
            [BsonDefaultValue("One")]
            public string stopBits { get; set; }
            [BsonDefaultValue("None")]
            public string handshake { get; set; }
            [BsonDefaultValue(0)]
            public int asyncOpenDelay { get; set; }
            [BsonDefaultValue(false)]
            public bool allowTLSv10 { get; set; }
            [BsonDefaultValue(false)]
            public bool allowTLSv11 { get; set; }
            [BsonDefaultValue(true)]
            public bool allowTLSv12 { get; set; }
            [BsonDefaultValue(true)]
            public bool allowTLSv13 { get; set; }
            [BsonDefaultValue("")]
            public string cipherList { get; set; }
            [BsonDefaultValue("")]
            public string localCertFilePath { get; set; }
            [BsonDefaultValue("")]
            public string peerCertFilePath { get; set; }
            [BsonDefaultValue("")]
            public string privateKeyFilePath { get; set; }
            [BsonDefaultValue(1)]
            public int localLinkAddress { get; set; }
            [BsonDefaultValue(1)]
            public int remoteLinkAddress { get; set; }
            [BsonDefaultValue(300)]
            public int giInterval { get; set; }
            [BsonDefaultValue(0)]
            public int class0ScanInterval { get; set; }
            [BsonDefaultValue(0)]
            public int class1ScanInterval { get; set; }
            [BsonDefaultValue(0)]
            public int class2ScanInterval { get; set; }
            [BsonDefaultValue(0)]
            public int class3ScanInterval { get; set; }
            public RangeScans []rangeScans { get; set; }
            [BsonDefaultValue(0)]
            public int timeSyncMode { get; set; }
            [BsonDefaultValue(true)]
            public bool enableUnsolicited { get; set; }
            [BsonDefaultValue(true)]
            public bool serverModeMultiActive { get; set; }
            [BsonDefaultValue(2)]
            public int maxClientConnections { get; set; }
            [BsonDefaultValue(1000)]
            public int MaxQueueSize { get; set; }
            public IChannel channel;
            public IMaster master;
            public bool isConnected;
        }

        public class rtData
        {
            public BsonDocument sourceDataUpdate { get; set; }
        }

        // demonstrates how to build a set of command headers for a complex command request
        static ICommandHeaders GetCommandHeaders()
        {
            var crob = new ControlRelayOutputBlock(OperationType.PULSE_ON, TripCloseCode.NUL, false, 1, 100, 100);
            var ao = new AnalogOutputDouble64(1.37);

            return CommandSet.From(
                CommandHeader.From(IndexedValue.From(crob, 0)),
                CommandHeader.From(IndexedValue.From(ao, 1))
            );
        }

        static int Main(string[] args)
        {
            Log(DriverMessage);
            Log("Driver version " + DriverVersion);
            Log("Using opendnp3 version 3.1.1");

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
                    <DNP3_connection>(ProtocolConnectionsCollectionName);
            var conns =
                collconns
                    .Find(conn =>
                        conn.protocolDriver == ProtocolDriverName &&
                        conn.protocolDriverInstanceNumber ==
                        ProtocolDriverInstanceNumber &&
                        conn.enabled == true)
                    .ToList();
            foreach (DNP3_connection isrv in conns)
            {
                if (isrv.ipAddresses.Length < 1 && isrv.portName.Trim() == "")
                {
                    Log("No ipAddresses or port name defined on conenction! " + isrv.name);
                    Environment.Exit(-1);
                }
                DNP3conns.Add(isrv);
                Log(isrv.name.ToString() + " - New Connection");
            }
            if (DNP3conns.Count == 0)
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

            Log("Setting up connections & ASDU handlers...");
            mgr = DNP3ManagerFactory.CreateManager(2*Environment.ProcessorCount, new PrintingLogAdapter());
            foreach (DNP3_connection srv in DNP3conns)
            {
                uint logLevel = LogLevels.NONE;
                if (LogLevel >= LogLevelBasic)
                    logLevel = LogLevels.NORMAL;
                if (LogLevel >= LogLevelDetailed)
                    logLevel = LogLevels.NORMAL | LogLevels.APP_COMMS;
                if (LogLevel >= LogLevelDebug)
                    logLevel = LogLevels.ALL;
                
                MyChannelListener chlistener = new MyChannelListener();
                chlistener.dnp3conn = srv;

                IChannel channel = null; // can be tcp, udp, tls, or serial
                if (srv.ipAddresses.Length > 0) // TCP, TLS or UDP
                { 
                    if (srv.ipAddressLocalBind.Trim() != "")
                    { // UDP
                        // look for the same channel config already created (multi-drop case)
                        // if found, just reuse
                        foreach (DNP3_connection conn in DNP3conns)
                        {
                            if (!(conn.channel is null))
                                if (conn.ipAddressLocalBind.Trim() != "" &&
                                    srv.ipAddressLocalBind.Trim() == conn.ipAddressLocalBind.Trim() &&
                                    srv.ipAddresses[0] == conn.ipAddresses[0])
                                {
                                    channel = conn.channel;
                                    break;
                                }
                        }
                        if (!(channel is null))
                        {
                            Log(srv.name + " - Reusing channel...");
                        }
                        else
                        {
                            Log(srv.name + " - Creating UDP channel...");
                            ushort localUdpPort = 20000;
                            string[] localIpAddrPort = srv.ipAddressLocalBind.Split(':');
                            if (localIpAddrPort.Length > 1)
                                if (int.TryParse(localIpAddrPort[1], out _))
                                    localUdpPort = System.Convert.ToUInt16(localIpAddrPort[1]);
                            ushort remoteUdpPort = 20000;
                            string[] remoteIpAddrPort = srv.ipAddresses[0].Split(':');
                            if (remoteIpAddrPort.Length > 1)
                                if (int.TryParse(remoteIpAddrPort[1], out _))
                                    remoteUdpPort = System.Convert.ToUInt16(remoteIpAddrPort[1]);
                            channel = mgr.AddUDPChannel(
                                "UDP:" + srv.name,
                                logLevel,
                                new ChannelRetry(TimeSpan.FromSeconds(1), TimeSpan.FromSeconds(30), TimeSpan.FromSeconds(5)),
                                new IPEndpoint(localIpAddrPort[0], localUdpPort),
                                new IPEndpoint(remoteIpAddrPort[0], remoteUdpPort),
                                chlistener
                                );
                        }
                    }
                    else
                    { // TCP or TLS
                        ushort tcpPort = 20000;
                        string[] ipAddrPort = srv.ipAddresses[0].Split(':');
                        if (ipAddrPort.Length > 1)
                            if (int.TryParse(ipAddrPort[1], out _))
                                tcpPort = System.Convert.ToUInt16(ipAddrPort[1]);

                        // look for the same channel config already created (multi-drop case)
                        // if found, just reuse
                        foreach (DNP3_connection conn in DNP3conns)
                        {
                            if ( !(conn.channel is null) ) 
                            if (srv.ipAddresses.SequenceEqual(conn.ipAddresses))
                            {
                                channel = conn.channel;                                
                                break;
                            }
                        }
                        if (!(channel is null))
                        {
                            Log(srv.name + " - Reusing channel...");
                        }
                        else
                        {
                            if (srv.localCertFilePath.Trim() != "")
                            { // TLS
                                Log(srv.name + " - Creating TLS channel...");
                                TLSConfig tlscfg = new TLSConfig(
                                    srv.peerCertFilePath,
                                    srv.localCertFilePath,
                                    srv.privateKeyFilePath,
                                    srv.allowTLSv10,
                                    srv.allowTLSv11,
                                    srv.allowTLSv12,
                                    srv.allowTLSv13,
                                    srv.cipherList
                                    );
                                channel = mgr.AddTLSClient(
                                    "TLS:" + srv.name,
                                    logLevel,
                                    new ChannelRetry(TimeSpan.FromSeconds(1), TimeSpan.FromSeconds(30), TimeSpan.FromSeconds(5)),
                                    new List<IPEndpoint> { new IPEndpoint(ipAddrPort[0], tcpPort) },
                                    tlscfg,
                                    chlistener
                                    );
                            }
                            else
                            { // TCP
                                Log(srv.name + " - Creating TCP channel...");
                                channel = mgr.AddTCPClient(
                                    "TCP:" + srv.name,
                                    logLevel,
                                    new ChannelRetry(TimeSpan.FromSeconds(1), TimeSpan.FromSeconds(30), TimeSpan.FromSeconds(5)),
                                    new List<IPEndpoint> { new IPEndpoint(ipAddrPort[0], tcpPort) },
                                    chlistener
                                    );
                            }
                        }
                    }
                }
                else if (srv.portName.Trim() != "" )
                { // serial connection
                    // look for the same channel config already created (multi-drop case)
                    // if found, just reuse
                    foreach (DNP3_connection conn in DNP3conns)
                    {
                        if (!(conn.channel is null))
                            if (conn.portName.Trim() != "" &&
                                srv.portName.Trim() == conn.portName.Trim())
                            {
                                channel = conn.channel;
                                break;
                            }
                    }
                    if (!(channel is null))
                    {
                        Log(srv.name + " - Reusing channel...");
                    }
                    else
                    {
                        Log(srv.name + " - Creating serial channel...");
                        StopBits stopbits;
                        switch (srv.stopBits.ToLower())
                        {
                            default:
                            case "1":
                            case "one":
                                stopbits = StopBits.One;
                                break;
                            case "1.5":
                            case "one.five":
                            case "one5":
                                stopbits = StopBits.OnePointFive;
                                break;
                            case "2":
                            case "two":
                                stopbits = StopBits.Two;
                                break;
                            case "none":
                                stopbits = StopBits.None;
                                break;
                        }
                        Parity parity;
                        switch (srv.parity.ToLower())
                        {
                            default:
                            case "none":
                                parity = Parity.None;
                                break;
                            case "even":
                                parity = Parity.Even;
                                break;
                            case "odd":
                                parity = Parity.Odd;
                                break;
                        }
                        FlowControl fc;
                        switch (srv.handshake.ToLower())
                        {
                            default:
                            case "none":
                                fc = FlowControl.None;
                                break;
                            case "xon":
                                fc = FlowControl.XONXOFF;
                                break;
                            case "rts":
                                fc = FlowControl.Hardware;
                                break;
                        }
                        SerialSettings ss = new SerialSettings(srv.portName, srv.baudRate, 8, stopbits, parity, fc);
                        channel = mgr.AddSerial(
                            "SERIAL:" + srv.name,
                            logLevel,
                            new ChannelRetry(TimeSpan.FromSeconds(1), TimeSpan.FromSeconds(30), TimeSpan.FromSeconds(5)),
                            ss,
                            chlistener
                            );
                    }
                }
                else
                { // bad connection config, skip
                    Log(srv.name  + " - Bad connection config! Ingoring...");
                    continue;
                }

                if (channel is null)
                {
                    Log("Channel allocation error!");
                    continue;
                }
                else
                {
                    srv.channel = channel;
                    var config = new MasterStackConfig();
                    // setup stack configuration here.
                    config.link.localAddr = System.Convert.ToUInt16(srv.localLinkAddress);
                    config.link.remoteAddr = System.Convert.ToUInt16(srv.remoteLinkAddress);
                    //config.link.responseTimeout = TimeSpan.FromSeconds(5);
                    //config.link.keepAliveTimeout =  TimeSpan.FromSeconds(60);
                    config.master.startupIntegrityClassMask = ClassField.AllClasses;
                    config.master.timeSyncMode = TimeSyncMode.None;
                    if (srv.timeSyncMode>=2)
                      config.master.timeSyncMode = TimeSyncMode.LAN;
                    else
                    if (srv.timeSyncMode == 1)
                        config.master.timeSyncMode = TimeSyncMode.NonLAN;
                    // config.master.responseTimeout = TimeSpan.FromSeconds(5);
                    if (srv.enableUnsolicited)
                    {
                        config.master.disableUnsolOnStartup = false;
                        config.master.unsolClassMask = ClassField.AllClasses;
                    }                    
                    else
                    {
                        config.master.disableUnsolOnStartup = true;
                        config.master.unsolClassMask = ClassField.None;
                    }

                    var soe_handler = new MySOEHandler();
                    soe_handler.ConnectionName = srv.name;
                    soe_handler.ConnectionNumber = srv.protocolConnectionNumber;
                    var master = channel.AddMaster(srv.name, soe_handler, DefaultMasterApplication.Instance, config);
                    srv.master = master;
                    if (srv.giInterval > 0)
                        master.AddClassScan(ClassField.AllClasses, TimeSpan.FromSeconds(srv.giInterval), soe_handler, TaskConfig.Default);

                    if (srv.class0ScanInterval > 0)
                        master.AddClassScan(ClassField.From(PointClass.Class0), TimeSpan.FromSeconds(srv.class0ScanInterval), soe_handler, TaskConfig.Default);
                    if (srv.class1ScanInterval > 0)
                        master.AddClassScan(ClassField.From(PointClass.Class1), TimeSpan.FromSeconds(srv.class1ScanInterval), soe_handler, TaskConfig.Default);
                    if (srv.class2ScanInterval > 0)
                        master.AddClassScan(ClassField.From(PointClass.Class2), TimeSpan.FromSeconds(srv.class2ScanInterval), soe_handler, TaskConfig.Default);
                    if (srv.class3ScanInterval > 0)
                        master.AddClassScan(ClassField.From(PointClass.Class3), TimeSpan.FromSeconds(srv.class3ScanInterval), soe_handler, TaskConfig.Default);

                    foreach (RangeScans rs in srv.rangeScans)
                    {
                        master.AddRangeScan(
                            System.Convert.ToByte(rs.group),
                            System.Convert.ToByte(rs.variation),
                            System.Convert.ToUInt16(rs.startAddress),
                            System.Convert.ToUInt16(rs.stopAddress),
                            TimeSpan.FromSeconds(rs.period),
                            soe_handler,
                            TaskConfig.Default);
                    }

                    master.Disable(); // disable communications
                    srv.isConnected = false;
                }
            }

            while (true)
            {
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
        }
    }
}
