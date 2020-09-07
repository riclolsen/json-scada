/* 
 * PLCTags CIP Ethernet/IP & Modbus TCP Client Protocol driver for {json:scada}
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

using libplctag;
using libplctag.DataTypes;
using libplctag.NativeImport;
using System;
using System.IO;
using System.Text.Json;
using System.Threading;
using System.Timers;
using MongoDB.Bson;
using MongoDB.Bson.Serialization.Attributes;
using MongoDB.Driver;
using System.Linq;
using System.Diagnostics;
using System.Threading.Tasks;
using System.Collections.Generic;
using System.Collections;

namespace PLCTagDriver
{
    partial class MainClass
    {
        public static String ProtocolDriverName = "PLCTAG";
        public static String DriverVersion = "0.1.0";
        public static bool Active = false; // indicates this driver instance is the active node in the moment
        public static Int32 DataBufferLimit = 10000; // limit to start dequeuing and discarding data from the acquisition buffer

        [BsonIgnoreExtraElements]
        public class
        PLC_connection // connection to PLC
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
            [BsonDefaultValue(1000)]
            public int MaxQueueSize { get; set; }

            [BsonDefaultValue("controllogix")]
            public string plc { get; set; }
            [BsonDefaultValue("ab_eip")]
            public string protocol { get; set; }
            [BsonDefaultValue(true)]
            public bool useConnectedMsg { get; set; }
            [BsonDefaultValue(100)]
            public int readCacheMs { get; set; }
            [BsonDefaultValue(1000)]
            public int timeoutMs { get; set; }
            [BsonDefaultValue("01")]
            public string int16ByteOrder { get; set; }
            [BsonDefaultValue("0123")]
            public string int32ByteOrder { get; set; }
            [BsonDefaultValue("01234567")]
            public string int64ByteOrder { get; set; }
            [BsonDefaultValue("0123")]
            public string float32ByteOrder { get; set; }
            [BsonDefaultValue("01234567")]
            public string float64ByteOrder { get; set; }

            public List<libplctag.Tag<BoolPlcMapper, bool>> listBoolTags = new List<libplctag.Tag<BoolPlcMapper, bool>>();
            public List<libplctag.Tag<SintPlcMapper, sbyte>> listSintTags = new List<libplctag.Tag<SintPlcMapper, sbyte>>();
            public List<libplctag.Tag<IntPlcMapper, short>> listIntTags = new List<libplctag.Tag<IntPlcMapper, short>>();
            public List<libplctag.Tag<DintPlcMapper, int>> listDintTags = new List<libplctag.Tag<DintPlcMapper, int>>();
            public List<libplctag.Tag<LintPlcMapper, long>> listLintTags = new List<libplctag.Tag<LintPlcMapper, long>>();
            public List<libplctag.Tag<RealPlcMapper, float>> listRealTags = new List<libplctag.Tag<RealPlcMapper, float>>();
            public List<libplctag.Tag<LrealPlcMapper, double>> listLrealTags = new List<libplctag.Tag<LrealPlcMapper, double>>();


            public List<libplctag.Tag<BoolPlcMapper, bool[]>> listBoolArrayTags = new List<libplctag.Tag<BoolPlcMapper, bool[]>>();
            public List<libplctag.Tag<SintPlcMapper, sbyte[]>> listSintArrayTags = new List<libplctag.Tag<SintPlcMapper, sbyte[]>>();
            public List<libplctag.Tag<IntPlcMapper, short[]>> listIntArrayTags = new List<libplctag.Tag<IntPlcMapper, short[]>>();
            public List<libplctag.Tag<DintPlcMapper, int[]>> listDintArrayTags = new List<libplctag.Tag<DintPlcMapper, int[]>>();
            public List<libplctag.Tag<LintPlcMapper, long[]>> listLintArrayTags = new List<libplctag.Tag<LintPlcMapper, long[]>>();
            public List<libplctag.Tag<RealPlcMapper, float[]>> listRealArrayTags = new List<libplctag.Tag<RealPlcMapper, float[]>>();
            public List<libplctag.Tag<LrealPlcMapper, double[]>> listLrealArrayTags = new List<libplctag.Tag<LrealPlcMapper, double[]>>();

            public int CntGI;
            public System.Timers.Timer TimerCnt;
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
            [BsonDefaultValue("")]
            public BsonString protocolSourceCommonAddress { get; set; }
            [BsonDefaultValue("")]
            public  BsonString protocolSourceObjectAddress { get; set; }
            [BsonDefaultValue("")]
            public BsonString protocolSourceASDU { get; set; }
            public BsonDouble protocolSourceCommandDuration { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean protocolSourceCommandUseSBO { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(1)]
            public BsonDouble kconv1 { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble kconv2 { get; set; }
            public rtSourceDataUpdate sourceDataUpdate { get; set; }
            public rtDataProtocDest[] protocolDestinations { get; set; }
        }

        public static void Main(string[] args)
        {
            //Instantiate the tag with the proper mapper and datatype
//            var myTag = new Tag<DintPlcMapper, int>()
//            {
//                Name = "PLC1ANA0",
//                Gateway = "127.0.0.1",
//                Path = "1,0",
//                PlcType = PlcType.ControlLogix,
//                Protocol = Protocol.ab_eip,
//                Timeout = TimeSpan.FromSeconds(5)
//            };

            //Instantiate the tag with the proper mapper and datatype
            var myTag = new Tag<RealPlcMapper, float>()
            {
                Name = "FLOAT",
                Gateway = "127.0.0.1",
                Path = "1,0",
                PlcType = PlcType.ControlLogix,
                Protocol = Protocol.ab_eip,
                UseConnectedMessaging = true,
                Timeout = TimeSpan.FromSeconds(5)
            };

            //Initialize the tag to set up structures and prepare for read/write
            //This is optional as an optimization before using the tag
            //If omitted, the tag will initialize on the first Read() or Write()
            myTag.Initialize();

            //The value is held locally and only synchronized on Read() or Write()
            myTag.Value = (float)3337.431;

            //Transfer Value to PLC
            myTag.Write();

            var myTag2 = new Tag<DintPlcMapper, int>()
            {
                Name = "PLC1ANA1",
                Gateway = "127.0.0.1",
                Path = "1,0",
                PlcType = PlcType.ControlLogix,
                UseConnectedMessaging = true,
                Protocol = Protocol.ab_eip,
                Timeout = TimeSpan.FromSeconds(5)
            };

            //Initialize the tag to set up structures and prepare for read/write
            //This is optional as an optimization before using the tag
            //If omitted, the tag will initialize on the first Read() or Write()
            myTag2.Initialize();

            //The value is held locally and only synchronized on Read() or Write()
            myTag2.Value = 2233;

            //Transfer Value to PLC
            myTag2.Write();

            var myTag3 = new Tag<RealPlcMapper, float[]>()
            {
                Name = "SCADA",
                Gateway = "127.0.0.1",
                Path = "1,0",
                PlcType = PlcType.ControlLogix,
                UseConnectedMessaging = true,
                Protocol = Protocol.ab_eip,
                ArrayDimensions = new int[] { 10, 10 },
                Timeout = TimeSpan.FromSeconds(5)
            };

            //Initialize the tag to set up structures and prepare for read/write
            //This is optional as an optimization before using the tag
            //If omitted, the tag will initialize on the first Read() or Write()
            myTag3.Initialize();

            //The value is held locally and only synchronized on Read() or Write()
            myTag3.Value = new float[] { 1, 2, 3, 4, 5, 6, 7, 8, 9, 0 }; 

            //Transfer Value to PLC
            myTag3.Write();

            myTag.Dispose();
            myTag2.Dispose();
            myTag3.Dispose();



            Log("{json:scada} PLC TAG Driver - Copyright 2020 RLO");
            Log("Driver version " + DriverVersion);
            Log("Using libplctag version " + LibPlcTag.VersionMajor + "."  + LibPlcTag.VersionMinor + "." + LibPlcTag.VersionPatch);

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
                    <PLC_connection>(ProtocolConnectionsCollectionName);
            var conns =
                collconns
                    .Find(conn =>
                        conn.protocolDriver == ProtocolDriverName &&
                        conn.protocolDriverInstanceNumber ==
                        ProtocolDriverInstanceNumber &&
                        conn.enabled == true)
                    .ToList();
            foreach (PLC_connection isrv in conns)
            {
                if (isrv.ipAddresses.Length < 1)
                {
                    Log("Missing ipAddresses list!");
                    Environment.Exit(-1);
                }
                PLCconns.Add(isrv);
                Log(isrv.name.ToString() + " - New Connection");
            }
            if (PLCconns.Count == 0)
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

            Log("Setting up IEC Connections & ASDU handlers...");
            int cntIecSrv = 0;
            foreach (PLC_connection srv in PLCconns)
            {
                var collection = DB.GetCollection<rtData>(RealtimeDataCollectionName);
                var filter = Builders<rtData>.Filter.Eq("protocolSourceConnectionNumber", srv.protocolConnectionNumber);
                var documents = collection.Find(filter).ToList();
                var enumerator = documents.GetEnumerator();

                var tags = Enumerable.Range(0, documents.Count)
                    .Select(i => {
                    enumerator.MoveNext();
                    var plctp = PlcType.ControlLogix;
                    switch (srv.plc.ToLower())
                    {
                        default:
                        case "lgx":
                        case "compactlogix":
                        case "contrologix":
                        case "controllogix":
                            plctp = PlcType.ControlLogix;
                            break;
                        case "lgxpccc":
                        case "logixpccc":
                            plctp = PlcType.LogixPccc;
                            break;
                        case "omron":
                        case "omronnjnx":
                        case "omron-njnx":
                        case "micro800":
                        case "micrologix800":
                        case "mlgx800":
                            plctp = PlcType.Micro800;
                            break;
                        case "mlgx":
                        case "micrologix":
                            plctp = PlcType.MicroLogix;
                            break;
                        case "plc5":
                            plctp = PlcType.Plc5;
                            break;
                        case "slc500":
                            plctp = PlcType.Slc500;
                            break;
                    }
                    if (enumerator.Current.protocolSourceASDU.ToString().Contains("[") && enumerator.Current.protocolSourceASDU.ToString().Contains("]"))
                    {
                        var p1 = enumerator.Current.protocolSourceASDU.ToString().IndexOf("[");
                        var p2 = enumerator.Current.protocolSourceASDU.ToString().IndexOf("]");
                        var p3 = enumerator.Current.protocolSourceObjectAddress.ToString().IndexOf("[");
                        var datatype = enumerator.Current.protocolSourceASDU.ToString().Substring(0, p1).ToLower();
                        var arrlens = enumerator.Current.protocolSourceASDU.ToString().Substring(p1+1, p2-p1-1);
                        var arrlen = System.Convert.ToInt32(arrlens);
                        switch (datatype)
                        {
                            case "bool":
                            case "boolean":
                                {
                                    var tag = new Tag<BoolPlcMapper, bool[]>()
                                    {
                                        Name = enumerator.Current.protocolSourceObjectAddress.ToString().Substring(0, p3),
                                        Gateway = srv.ipAddresses[0],
                                        Path = enumerator.Current.protocolSourceCommonAddress.ToString(),
                                        PlcType = plctp,
                                        Protocol = (srv.protocol.ToLower() == "modbus") ? Protocol.modbus_tcp : Protocol.ab_eip,
                                        UseConnectedMessaging = srv.useConnectedMsg,
                                        Timeout = TimeSpan.FromMilliseconds(1000),
                                        ArrayDimensions = new int[] { arrlen, 1 },
                                    };

                                    var tagFound = false; // avoid tag re-insertion 
                                    foreach ( var tg in srv.listBoolArrayTags )
                                        {
                                            if (tg.Name == tag.Name)
                                                tagFound = true;
                                        }
                                    if (!tagFound)
                                        {
                                            tag.Initialize();
                                            srv.listBoolArrayTags.Add(tag);
                                        }
                                }
                                break;
                            case "byte":
                            case "sint":
                            case "int8":
                                {
                                    var tag = new Tag<SintPlcMapper, sbyte[]>()
                                    {
                                        Name = enumerator.Current.protocolSourceObjectAddress.ToString().Substring(0, p3),
                                        Gateway = srv.ipAddresses[0],
                                        Path = enumerator.Current.protocolSourceCommonAddress.ToString(),
                                        PlcType = plctp,
                                        Protocol = (srv.protocol.ToLower() == "modbus") ? Protocol.modbus_tcp : Protocol.ab_eip,
                                        UseConnectedMessaging = srv.useConnectedMsg,
                                        Timeout = TimeSpan.FromMilliseconds(1000),
                                        ArrayDimensions = new int[] { arrlen, 1 },
                                    };
                                    var tagFound = false; // avoid tag re-insertion 
                                    foreach (var tg in srv.listSintArrayTags)
                                    {
                                        if (tg.Name == tag.Name)
                                            tagFound = true;
                                    }
                                    if (!tagFound)
                                    {
                                        tag.Initialize();
                                        srv.listSintArrayTags.Add(tag);
                                    }
                            }
                            break;
                            default:
                            case "int":
                            case "int16":
                                {
                                    var tag = new Tag<IntPlcMapper, short[]>()
                                    {
                                        Name = enumerator.Current.protocolSourceObjectAddress.ToString().Substring(0, p3),
                                        Gateway = srv.ipAddresses[0],
                                        Path = enumerator.Current.protocolSourceCommonAddress.ToString(),
                                        PlcType = plctp,
                                        Protocol = (srv.protocol.ToLower() == "modbus") ? Protocol.modbus_tcp : Protocol.ab_eip,
                                        UseConnectedMessaging = srv.useConnectedMsg,
                                        Timeout = TimeSpan.FromMilliseconds(1000),
                                        ArrayDimensions = new int[] { arrlen, 1 },
                                    };
                                    var tagFound = false; // avoid tag re-insertion 
                                    foreach (var tg in srv.listIntArrayTags)
                                    {
                                        if (tg.Name == tag.Name)
                                            tagFound = true;
                                    }
                                    if (!tagFound)
                                    {
                                        tag.Initialize();
                                        srv.listIntArrayTags.Add(tag);
                                    }
                                }
                                break;
                            case "dint":
                            case "int32":
                                {
                                    var tag = new Tag<DintPlcMapper, int[]>()
                                    {
                                        Name = enumerator.Current.protocolSourceObjectAddress.ToString().Substring(0, p3),
                                        Gateway = srv.ipAddresses[0],
                                        Path = enumerator.Current.protocolSourceCommonAddress.ToString(),
                                        PlcType = plctp,
                                        Protocol = (srv.protocol.ToLower() == "modbus") ? Protocol.modbus_tcp : Protocol.ab_eip,
                                        UseConnectedMessaging = srv.useConnectedMsg,
                                        Timeout = TimeSpan.FromMilliseconds(1000),
                                        ArrayDimensions = new int[] { arrlen, 1 },
                                    };
                                    tag.Initialize();
                                    srv.listDintArrayTags.Add(tag);
                                    var tagFound = false; // avoid tag re-insertion 
                                    foreach (var tg in srv.listDintArrayTags)
                                    {
                                        if (tg.Name == tag.Name)
                                            tagFound = true;
                                    }
                                    if (!tagFound)
                                    {
                                        tag.Initialize();
                                        srv.listDintArrayTags.Add(tag);
                                    }
                                }
                                break;
                            case "lint":
                            case "int64":
                                {
                                    var tag = new Tag<LintPlcMapper, long[]>()
                                    {
                                        Name = enumerator.Current.protocolSourceObjectAddress.ToString().Substring(0, p3),
                                        Gateway = srv.ipAddresses[0],
                                        Path = enumerator.Current.protocolSourceCommonAddress.ToString(),
                                        PlcType = plctp,
                                        Protocol = (srv.protocol.ToLower() == "modbus") ? Protocol.modbus_tcp : Protocol.ab_eip,
                                        UseConnectedMessaging = srv.useConnectedMsg,
                                        Timeout = TimeSpan.FromMilliseconds(1000),
                                        ArrayDimensions = new int[] { arrlen, 1 },
                                    };
                                    tag.Initialize();
                                    srv.listLintArrayTags.Add(tag);
                                    var tagFound = false; // avoid tag re-insertion 
                                    foreach (var tg in srv.listLintArrayTags)
                                    {
                                        if (tg.Name == tag.Name)
                                            tagFound = true;
                                    }
                                    if (!tagFound)
                                    {
                                        tag.Initialize();
                                        srv.listLintArrayTags.Add(tag);
                                    }
                                }
                                break;
                            case "real":
                            case "float32":
                                {
                                    var tag = new Tag<RealPlcMapper, float[]>()
                                    {
                                        Name = enumerator.Current.protocolSourceObjectAddress.ToString().Substring(0, p3),
                                        Gateway = srv.ipAddresses[0],
                                        Path = enumerator.Current.protocolSourceCommonAddress.ToString(),
                                        PlcType = plctp,
                                        Protocol = (srv.protocol.ToLower() == "modbus") ? Protocol.modbus_tcp : Protocol.ab_eip,
                                        UseConnectedMessaging = srv.useConnectedMsg,
                                        Timeout = TimeSpan.FromMilliseconds(1000),
                                        ArrayDimensions = new int[] { arrlen, 1 },
                                    };
                                    var tagFound = false; // avoid tag re-insertion 
                                    foreach (var tg in srv.listRealArrayTags)
                                    {
                                        if (tg.Name == tag.Name)
                                            tagFound = true;
                                    }
                                    if (!tagFound)
                                    {
                                        tag.Initialize();
                                        srv.listRealArrayTags.Add(tag);
                                    }
                                }
                                break;
                            case "lreal":
                            case "float64":
                                {
                                    var tag = new Tag<LrealPlcMapper, double[]>()
                                    {
                                        Name = enumerator.Current.protocolSourceObjectAddress.ToString().Substring(0, p1+1),
                                        Gateway = srv.ipAddresses[0],
                                        Path = enumerator.Current.protocolSourceCommonAddress.ToString(),
                                        PlcType = plctp,
                                        Protocol = (srv.protocol.ToLower() == "modbus") ? Protocol.modbus_tcp : Protocol.ab_eip,
                                        UseConnectedMessaging = srv.useConnectedMsg,
                                        Timeout = TimeSpan.FromMilliseconds(1000),
                                        ArrayDimensions = new int[] { arrlen, 1 },
                                    };
                                    var tagFound = false; // avoid tag re-insertion 
                                    foreach (var tg in srv.listLrealArrayTags)
                                    {
                                        if (tg.Name == tag.Name)
                                            tagFound = true;
                                    }
                                    if (!tagFound)
                                    {
                                        tag.Initialize();
                                        srv.listLrealArrayTags.Add(tag);
                                    }
                                }
                                break;
                            }
                    }
                    else
                    switch (enumerator.Current.protocolSourceASDU.ToString().ToLower())
                    {
                        case "bool":
                        case "boolean":
                            {
                                var tag = new Tag<BoolPlcMapper, bool>()
                                {
                                    Name = enumerator.Current.protocolSourceObjectAddress.ToString(),
                                    Gateway = srv.ipAddresses[0],
                                    Path = enumerator.Current.protocolSourceCommonAddress.ToString(),
                                    PlcType = plctp,
                                    Protocol = (srv.protocol.ToLower() == "modbus") ? Protocol.modbus_tcp : Protocol.ab_eip,
                                    UseConnectedMessaging = srv.useConnectedMsg,
                                    Timeout = TimeSpan.FromMilliseconds(1000),
                                };
                                tag.Initialize();
                                srv.listBoolTags.Add(tag);
                            }
                            break;
                        case "byte":
                        case "sint":
                        case "int8":
                            {
                                var tag = new Tag<SintPlcMapper, sbyte>()
                                {
                                    Name = enumerator.Current.protocolSourceObjectAddress.ToString(),
                                    Gateway = srv.ipAddresses[0],
                                    Path = enumerator.Current.protocolSourceCommonAddress.ToString(),
                                    PlcType = plctp,
                                    Protocol = (srv.protocol.ToLower() == "modbus") ? Protocol.modbus_tcp : Protocol.ab_eip,
                                    UseConnectedMessaging = srv.useConnectedMsg,
                                    Timeout = TimeSpan.FromMilliseconds(1000),
                                };
                                tag.Initialize();
                                srv.listSintTags.Add(tag);
                            }
                            break;
                        default:
                        case "int":
                        case "int16":
                            {
                                var tag = new Tag<IntPlcMapper, short>()
                                {
                                    Name = enumerator.Current.protocolSourceObjectAddress.ToString(),
                                    Gateway = srv.ipAddresses[0],
                                    Path = enumerator.Current.protocolSourceCommonAddress.ToString(),
                                    PlcType = plctp,
                                    Protocol = (srv.protocol.ToLower() == "modbus") ? Protocol.modbus_tcp : Protocol.ab_eip,
                                    UseConnectedMessaging = srv.useConnectedMsg,
                                    Timeout = TimeSpan.FromMilliseconds(1000),
                                };
                                tag.Initialize();
                                srv.listIntTags.Add(tag);
                            }
                            break;
                        case "dint":
                        case "int32":
                            { 
                                var tag = new Tag<DintPlcMapper, int>()
                                {
                                    Name = enumerator.Current.protocolSourceObjectAddress.ToString(),
                                    Gateway = srv.ipAddresses[0],
                                    Path = enumerator.Current.protocolSourceCommonAddress.ToString(),
                                    PlcType = plctp,
                                    Protocol = (srv.protocol.ToLower() == "modbus") ? Protocol.modbus_tcp : Protocol.ab_eip,
                                    UseConnectedMessaging = srv.useConnectedMsg,
                                    Timeout = TimeSpan.FromMilliseconds(1000),
                                };
                                tag.Initialize();
                                srv.listDintTags.Add(tag);
                            }
                            break;
                        case "lint":
                        case "int64":
                            {
                                var tag = new Tag<LintPlcMapper, long>()
                                {
                                    Name = enumerator.Current.protocolSourceObjectAddress.ToString(),
                                    Gateway = srv.ipAddresses[0],
                                    Path = enumerator.Current.protocolSourceCommonAddress.ToString(),
                                    PlcType = plctp,
                                    Protocol = (srv.protocol.ToLower() == "modbus") ? Protocol.modbus_tcp : Protocol.ab_eip,
                                    UseConnectedMessaging = srv.useConnectedMsg,
                                    Timeout = TimeSpan.FromMilliseconds(1000),
                                };
                                tag.Initialize();
                                srv.listLintTags.Add(tag);
                            }
                            break;
                        case "real":
                        case "float32":
                            {
                                var tag = new Tag<RealPlcMapper, float>()
                                {
                                    Name = enumerator.Current.protocolSourceObjectAddress.ToString(),
                                    Gateway = srv.ipAddresses[0],
                                    Path = enumerator.Current.protocolSourceCommonAddress.ToString(),
                                    PlcType = plctp,
                                    Protocol = (srv.protocol.ToLower() == "modbus") ? Protocol.modbus_tcp : Protocol.ab_eip,
                                    UseConnectedMessaging = srv.useConnectedMsg,
                                    Timeout = TimeSpan.FromMilliseconds(1000),
                                };
                                tag.Initialize();
                                srv.listRealTags.Add(tag);
                            }
                            break;
                        case "lreal":
                        case "float64":
                            {
                                var tag = new Tag<LrealPlcMapper, double>()
                                {
                                    Name = enumerator.Current.protocolSourceObjectAddress.ToString(),
                                    Gateway = srv.ipAddresses[0],
                                    Path = enumerator.Current.protocolSourceCommonAddress.ToString(),
                                    PlcType = plctp,
                                    Protocol = (srv.protocol.ToLower() == "modbus") ? Protocol.modbus_tcp : Protocol.ab_eip,
                                    UseConnectedMessaging = srv.useConnectedMsg,
                                    Timeout = TimeSpan.FromMilliseconds(1000),
                                };
                                tag.Initialize();
                                srv.listLrealTags.Add(tag);
                            }
                            break;
                        }

                    return 0;
                    })
                    .ToList();

                // A thread for scanning each connection
                Thread thrPlcScan =
                    new Thread(() =>
                            ProcessPLCScan(srv));
                thrPlcScan.Start();
            }

            // start thread to watch for commands in the database using a change stream
            Thread thrMongoCmd =
                new Thread(() =>
                        ProcessMongoCmd(JSConfig));
            thrMongoCmd.Start();

            /*
            var asyncStopWatch = Stopwatch.StartNew();
            
            foreach (var tag in tags)
            {
                Task.WaitAll(tag.ReadAsync());
                PLC_Value iv =
                    new PLC_Value()
                    {
                        conn_number = 81,
                        address = tag.Name,
                        common_address = tag.Path,
                        asdu = tag.GetType().ToString(),
                        isDigital = true,
                        value = tag.Value,
                        time_tag = DateTime.Now,
                        cot = 20
                    };
                PLCDataQueue.Enqueue(iv);
                Console.WriteLine(tag.Name + " " + tag.Value);
            asyncStopWatch.Stop();
            Console.WriteLine($"\ttook {(float)asyncStopWatch.ElapsedMilliseconds} ms on average");
            }
            */


            do
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
            } while (true);


            /*

            foreach (var tag in tags)
            {
                Console.WriteLine(tag.Name, " ", tag.Value);
                tag.Dispose();
            }

            //Instantiate the tag with the proper mapper and datatype
            var myTag = new Tag<DintPlcMapper, int>()
            {
                Name = "PLC1ANA0",
                Gateway = "127.0.0.1",
                Path = "1,0",
                PlcType = PlcType.ControlLogix,
                Protocol = Protocol.ab_eip,
                Timeout = TimeSpan.FromSeconds(5)
            };
            
            //Initialize the tag to set up structures and prepare for read/write
            //This is optional as an optimization before using the tag
            //If omitted, the tag will initialize on the first Read() or Write()
            myTag.Initialize();

            //The value is held locally and only synchronized on Read() or Write()
            myTag.Value = 3737;

            //Transfer Value to PLC
            myTag.Write();

            //Transfer from PLC to Value
            myTag.Read();

            //Write to console
            int myDint = myTag.Value;
            Console.WriteLine(myDint);

            myTag.Dispose();
            */
        }
    }
}
