/* 
 * IEC 60870-5-101 Client Protocol driver for {json:scada}
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
using System.IO.Ports;
using System.Text.Json;
using System.Threading;
using System.Linq;
using System.Timers;
using MongoDB.Bson;
using MongoDB.Bson.Serialization.Attributes;
using MongoDB.Driver;
using lib60870;
using lib60870.CS101;
using lib60870.linklayer;

namespace Iec10XDriver
{
	partial class MainClass
	{
		public static String ProtocolDriverName = "IEC60870-5-101";
        public static String DriverVersion = "0.1.0";
        public static bool Active = false; // indicates this driver instance is the active node in the moment
		public static Int32 DataBufferLimit = 10000; // limit to start dequeuing and discarding data from the acquisition buffer

		[BsonIgnoreExtraElements]
		public class
		IEC10X_connection // IEC 10X connection to RTU
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
            public int timeoutCharacter{ get; set; }
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
			[BsonDefaultValue(300)]
			public int giInterval { get; set; }
			[BsonDefaultValue(0)]
			public int testCommandInterval { get; set; }
			[BsonDefaultValue(0)]
			public int timeSyncInterval { get; set; }
			[BsonDefaultValue(1000)]
			public int MaxQueueSize { get; set; }
			public CS101Master master;
			public int CntGI;
            public int CntTestCommand;
            public int CntTimeSync;
            public System.Timers.Timer TimerCnt;
        }

        public class rtData
        {
            public BsonDocument sourceDataUpdate { get; set; }
        }

        private static bool rcvdRawMessageHandler(object parameter, byte[] message, int messageSize)
		{
            //if (LogLevel > LogLevelDebug)
            //  Log("RECV " + BitConverter.ToString(message, 0, messageSize), LogLevelDebug);
			return true;
		}

        private static void linkLayerStateChanged (object parameter, int address, lib60870.linklayer.LinkLayerState newState)
		{
			Log("LL state event " + newState.ToString() + " for slave " + address);
		}

        private static bool AsduReceivedHandlerPre(object parameter, int slaveAddress, ASDU asdu)
        {
            return AsduReceivedHandler(parameter, asdu);
        }

        public static void Main (string[] args)
		{
            Log("{json:scada} IEC60870-5-101 Driver - Copyright 2020 RLO");
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

            // connect to MongoDB Database server
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

                ApplicationLayerParameters alpars = new ApplicationLayerParameters();
                alpars.SizeOfCOT = srv.sizeOfCOT;
                alpars.SizeOfCA = srv.sizeOfCA;
                alpars.SizeOfIOA = srv.sizeOfIOA;
                alpars.OA = srv.localLinkAddress;

                CS101Master master;
                if (port != null)
                {
                    Log("Serial Port: " + srv.portName);
                    master =
                        new CS101Master(port,
                            LinkLayerMode.UNBALANCED,
                            llParameters,
                            alpars);
                }
                else
                {
                    Log("Virtual Serial Port: " + srv.portName);
                    master =
                        new CS101Master(virtualPort,
                            LinkLayerMode.UNBALANCED,
                            llParameters,
                            alpars);
                }
                // master.OwnAddress = srv.localLinkAddress;
                master.SetTimeouts(srv.timeoutMessage, srv.timeoutCharacter);
                master.AddSlave(srv.remoteLinkAddress);
                master.SlaveAddress = srv.remoteLinkAddress;

                srv.master = master;
                srv.CntGI = srv.giInterval - 5;
                srv.CntTestCommand = srv.testCommandInterval - 2;
                srv.CntTimeSync = srv.timeSyncInterval;
                if (LogLevel >= LogLevelDebug)
                    master.DebugOutput = true;
                master.SetASDUReceivedHandler(AsduReceivedHandlerPre, cntIecSrv);
                master.SetLinkLayerStateChangedHandler(linkLayerStateChanged, cntIecSrv);
                master.SetReceivedRawMessageHandler(rcvdRawMessageHandler, cntIecSrv);
                master.Start();

                // create timer to increment counters each second
                srv.TimerCnt = new System.Timers.Timer();
                srv.TimerCnt.Interval = 1000;
                srv.TimerCnt.Elapsed += (sender, e) => MyElapsedMethod(sender, e, srv);
                static void MyElapsedMethod(object sender, ElapsedEventArgs e, IEC10X_connection serv)
                {
                    if (serv.giInterval > 0)
                        serv.CntGI++;
                    if (serv.testCommandInterval > 0)
                        serv.CntTestCommand++;
                    if (serv.timeSyncInterval > 0)
                        serv.CntTimeSync++;
                }
                srv.TimerCnt.Enabled = true;

                cntIecSrv++;
            }

            Thread.Sleep(500);

            do
            {
                foreach (IEC10X_connection srv in IEC10Xconns)
                {
                    if (Active)
                    {
                        //Log("MASTER LL: " + srv.master.GetLinkLayerState().ToString());
                        if (/*srv.master.GetLinkLayerState() == LinkLayerState.AVAILABLE && */srv.master.GetLinkLayerState(srv.master.SlaveAddress) == LinkLayerState.AVAILABLE )
                        {
                            if (srv.giInterval > 0)
                            {
                                if (srv.CntGI >= srv.giInterval)
                                {
                                    try
                                    {
                                        Log("Interrogation ----------------------------------- " + srv.remoteLinkAddress);
                                        srv.master.SlaveAddress = srv.remoteLinkAddress;
                                        srv
                                            .master
                                            .SendInterrogationCommand(CauseOfTransmission
                                                .ACTIVATION,
                                            srv.master.SlaveAddress,
                                            QualifierOfInterrogation.STATION);
                                    }
                                    catch (LinkLayerBusyException)
                                    {
                                        srv.CntGI = 0;
                                        Log("Master " + srv.name + ": Link layer busy or not ready");
                                        Thread.Sleep(100);
                                    }
                                    srv.CntGI = 0;
                                }
                            }
                            if (srv.testCommandInterval > 0)
                            {
                                if (srv.CntTestCommand >= srv.testCommandInterval)
                                {
                                    try
                                    {
                                        Log("Test Command");
                                        srv.master.SlaveAddress = srv.remoteLinkAddress;
                                        srv.master.SendTestCommand(srv.master.SlaveAddress);
                                    }
                                    catch (LinkLayerBusyException)
                                    {
                                        Log("Master " + srv.name + ": Link layer busy or not ready");
                                        Thread.Sleep(100);
                                    }
                                    srv.CntTestCommand = 0;
                                }
                            }
                            if (srv.timeSyncInterval > 0)
                            {
                                if (srv.CntTimeSync >= srv.timeSyncInterval)
                                {
                                    try
                                    {
                                        Log("Send Clock Sync");
                                        srv.master.SlaveAddress = srv.remoteLinkAddress;
                                        srv.master.SendClockSyncCommand(srv.master.SlaveAddress, new CP56Time2a(DateTime.Now));
                                    }
                                    catch (LinkLayerBusyException)
                                    {
                                        Log("Master " + srv.name + ": Link layer busy or not ready");
                                        Thread.Sleep(100);
                                    }
                                    srv.CntTimeSync = 0;
                                }
                            }
                            //Log("SLAVE " + srv.master.SlaveAddress + " LL: " + srv.master.GetLinkLayerState(srv.master.SlaveAddress).ToString());
                            srv.master.SlaveAddress = srv.remoteLinkAddress;
                            srv.master.PollSingleSlave(srv.master.SlaveAddress);
                            //srv.master.RequestClass1Data(srv.master.SlaveAddress)
                            //Log("Poll: " + srv.name, LogLevelDebug);
                        }

                        if (srv.master.GetLinkLayerState(srv.master.SlaveAddress) == LinkLayerState.ERROR)
                        {   
                            srv.CntGI = srv.giInterval - 5;
                            srv.CntTestCommand = srv.testCommandInterval - 2;
                            srv.CntTimeSync = srv.timeSyncInterval;
                            Log("Link error: " + srv.name);
                            srv.master.Stop();
                            Thread.Sleep(50);
                            srv.master.Start();
                        }
                    }
                    else
                    { // Inactive                        
                        if (srv.master.GetLinkLayerState() != LinkLayerState.ERROR)
                        {
                            srv.CntGI = 0;
                            srv.CntTimeSync = 0;
                            srv.CntTestCommand = 0;
                        }
                    }
                }
                Thread.Sleep(25);

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

            

            // Synchronize clock of the controlled station 
            //con.SendClockSyncCommand(1, new CP56Time2a(DateTime.Now));
                    
/*
            string portName = "COM3";

			SerialPort port = new SerialPort();

			port.PortName = portName;
			port.BaudRate = 9600;
			port.Parity = Parity.Even;
			port.StopBits = StopBits.One;
			port.Handshake = Handshake.None;
			port.Open ();
			port.DiscardInBuffer ();

			// set link layer address length 
			LinkLayerParameters llParameters = new LinkLayerParameters ();
			llParameters.AddressLength = 1;
            llParameters.TimeoutForACK = 1000;

            ApplicationLayerParameters alParameters = new ApplicationLayerParameters();
			alParameters.SizeOfCOT = 1;
			alParameters.SizeOfCA = 1;
			alParameters.SizeOfIOA = 2;

			// unbalanced mode allows multiple slaves on a single serial line 
			CS101Master master = new CS101Master(port, LinkLayerMode.UNBALANCED, llParameters, alParameters);
			master.DebugOutput = true;
			// master.OwnAddress = 2;
			master.SetASDUReceivedHandler (AsduReceivedHandlerTest, null);
			master.SetLinkLayerStateChangedHandler (linkLayerStateChanged, null);
			master.SetReceivedRawMessageHandler(rcvdRawMessageHandler, null);
			master.SlaveAddress = 37;
			// master.SetTimeouts(100, 50);

			master.AddSlave(37); 
			//master.AddSlave (2);
			//master.AddSlave (3);

			long lastTimestamp = SystemUtils.currentTimeMillis ();

			//master.GetFile (1, 30000, NameOfFile.TRANSPARENT_FILE, new Receiver ());

			//master.Start();

			while (running) {

                master.Run();
                master.PollSingleSlave(37);
                master.Run();

                //master.PollSingleSlave(2);
                //master.Run ();
                //master.PollSingleSlave(3);
                //master.Run ();

                if ((SystemUtils.currentTimeMillis() - lastTimestamp) >= 10000) {

					lastTimestamp = SystemUtils.currentTimeMillis ();

					try {
						Console.WriteLine("Interrogation");
						master.SlaveAddress = 37;
						master.SendInterrogationCommand (CauseOfTransmission.ACTIVATION, 37, 20);
					}
					catch (LinkLayerBusyException) {
						Console.WriteLine ("Slave 1: Link layer busy or not ready");
					}
				}
				
				Thread.Sleep(100);
			}

			//master.Stop();
			port.Close ();
*/
        }
    }
}
