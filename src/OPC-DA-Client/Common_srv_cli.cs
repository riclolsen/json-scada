/* 
 * OPC-UA Client Protocol driver for {json:scada}
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
using System.Collections.Concurrent;
using System.Collections.Generic;
using System.Threading;
using MongoDB.Bson;
using MongoDB.Bson.Serialization;
using MongoDB.Bson.Serialization.Serializers;
using MongoDB.Bson.Serialization.Attributes;
using System.Net.Security;
using MongoDB.Driver;
using System.Security.Cryptography.X509Certificates;
using Technosoftware.DaAeHdaClient.Da;

namespace OPCDAClientDriver
{
    partial class MainClass
    {
        public static int PointKeyInsert = 100000;
        public static string JsonConfigFilePath = @"../conf/json-scada.json";
        public static string JsonConfigFilePathAlt = @"c:/json-scada/conf/json-scada.json";
        public static int LogLevelNoLog = 0; // log level 0=no
        public static int LogLevelBasic = 1; // log level 1=basic (default)
        public static int LogLevelDetailed = 2; // log level 2=detailed
        public static int LogLevelDebug = 3; // log level 3=debug
        public static int LogLevel = 1; // log level 0=no 1=min
        private static Mutex logMutex_ = new Mutex();
        public static JSONSCADAConfig JSConfig = null;
        public static protocolDriverInstancesClass DriverInstance = null;
        public static string ProtocolConnectionsCollectionName = "protocolConnections";
        public static string ProtocolDriverInstancesCollectionName = "protocolDriverInstances";
        public static string RealtimeDataCollectionName = "realtimeData";
        public static string SoeDataCollectionName = "soeData";
        public static string CommandsQueueCollectionName = "commandsQueue";
        public static int ProtocolDriverInstanceNumber = 1;
        public static string redundantIpAddress = "";
        public static ConcurrentQueue<OPC_Value> OPCDataQueue = new ConcurrentQueue<OPC_Value>(); // acquired values queue (to be updated in mongodb realtime data collection)
        public static List<OPCDA_connection> OPCDAconns = new List<OPCDA_connection>(); // list of RTU connections

        public class
        JSONSCADAConfig // base configuration of the system (how to reach mongodb, etc.)
        {
            public string nodeName { get; set; }
            public string mongoConnectionString { get; set; }
            public string mongoDatabaseName { get; set; }
            public string tlsCaPemFile { get; set; }
            public string tlsClientPemFile { get; set; }
            public string tlsClientPfxFile { get; set; }
            public string tlsClientKeyPassword { get; set; }
            public bool tlsAllowInvalidHostnames { get; set; }
            public bool tlsAllowChainErrors { get; set; }
            public bool tlsInsecure { get; set; }
        }
        [BsonIgnoreExtraElements]
        public class
        OPCDA_connection // protocol connection to RTU
        {
            public ObjectId Id { get; set; }
            public string protocolDriver { get; set; } = string.Empty;
            public int protocolDriverInstanceNumber { get; set; } = 1;
            public int protocolConnectionNumber { get; set; } = 1;
            public string name { get; set; } = "NO NAME";
            public string description { get; set; } = "CONNECTION NOT DESCRIPTED";
            public bool enabled { get; set; } = true;
            public bool commandsEnabled { get; set; } = true;
            public string[] endpointURLs { get; set; }
            public string[] topics { get; set; }
            public bool autoCreateTags { get; set; } = true;
            public int giInterval { get; set; } = 0;
            public double deadBand { get; set; } = 0;
            public double hoursShift { get; set; } = 0;
            public double autoCreateTagPublishingInterval { get; set; } = 2.5;
            public double autoCreateTagSamplingInterval { get; set; } = 0.0;
            public double autoCreateTagQueueSize { get; set; } = 5.0;
            public double timeoutMs { get; set; } = 20000.0;
            public string username { get; set; } = string.Empty;
            public string password { get; set; } = string.Empty;
            public bool useSecurity { get; set; } = false;
            public string localCertFilePath { get; set; } = string.Empty;
            public string peerCertFilePath { get; set; } = string.Empty;
            //[BsonDefaultValue("")] 
            //public string privateKeyFilePath { get; set; }
            //[BsonDefaultValue("")] 
            //public string pfxFilePath { get; set; }
            //[BsonDefaultValue("")] 
            //public string passphrase { get; set; }
            //[BsonDefaultValue(false)] 
            //public bool allowTLSv10 { get; set; }
            //[BsonDefaultValue(false)]
            //public bool allowTLSv11 { get; set; }
            //[BsonDefaultValue(false)]
            //public bool allowTLSv12 { get; set; }
            //[BsonDefaultValue(false)]
            //public bool allowTLSv13 { get; set; }
            //[BsonDefaultValue("")] 
            //public string cipherList { get; set; }

            public double LastNewKeyCreated = -1;
            public SortedSet<string> InsertedTags = new SortedSet<string>();
            public SortedSet<string> InsertedAddresses = new SortedSet<string>();
            // public static Dictionary<string, string> MapNameToHandler = new Dictionary<string, string>();
            public Dictionary<string, string> MapHandlerToItemName = new Dictionary<string, string>();
            public Dictionary<string, string> MapHandlerToConnName = new Dictionary<string, string>();
            // public Dictionary<string, string> MapItemNameToBranch = new Dictionary<string, string>();
            public SortedSet<string> branches = new SortedSet<string>(); // branches to scan
            public TsCDaServer connection = null; // opc da connection to a server
            public List<TsCDaSubscription> subscriptions = new List<TsCDaSubscription>(); // data subscriptions on a server
            public Thread thrOPCStack = null;
            public int cntConnectRetries = 0;
            // public CancellationToken cancellationToken = new CancellationToken();
            public CancellationTokenSource cancellationTokenSource = new CancellationTokenSource();

        }
        [BsonIgnoreExtraElements]
        public class protocolDriverInstancesClass
        {
            public ObjectId Id { get; set; }
            public int protocolDriverInstanceNumber { get; set; } = 1;
            public string protocolDriver { get; set; } = "";
            public bool enabled { get; set; } = true;
            public int logLevel { get; set; } = 1;
            public string[] nodeNames { get; set; } = new string[0];
            public string activeNodeName { get; set; } = "";
            public DateTime activeNodeKeepAliveTimeTag { get; set; } = DateTime.MinValue;
            public bool keepProtocolRunningWhileInactive { get; set; } = false;
        }
        public struct OPC_Value
        {
#pragma warning disable 0649
            public string valueJson;
            public bool selfPublish;
            public string address;
            public string asdu;
            public bool isDigital;
            public bool isArray;
            public double value;
            public string valueString;
            public int cot;
            public DateTime serverTimestamp;
            public DateTime sourceTimestamp;
            public bool hasSourceTimestamp;
            public bool isGood;
            public int conn_number;
            public string conn_name;
            public string common_address;
            public string display_name;
            public string group1;
            public string group2;
            public string group3;
            public string ungroupedDescription;
        }
        public class rtFilt
        {
            public int protocolSourceConnectionNumber;
            public string protocolSourceObjectAddress;
            public string origin;
        }
        [BsonIgnoreExtraElements]
        public class rtCommand
        {
            public BsonObjectId id { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolSourceConnectionNumber { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolSourceCommonAddress { get; set; }
            [BsonDefaultValue("")]
            public BsonString protocolSourceObjectAddress { get; set; }
            [BsonDefaultValue("")]
            public BsonString protocolSourceASDU { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolSourceCommandDuration { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean protocolSourceCommandUseSBO { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble pointKey { get; set; }
            [BsonDefaultValue("")]
            public BsonString tag { get; set; }
            public BsonDateTime timeTag { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble value { get; set; }
            [BsonDefaultValue("")]
            public BsonString valueString { get; set; }
            [BsonDefaultValue("")]
            public BsonString originatorUserName { get; set; }
            [BsonDefaultValue("")]
            public BsonString originatorIpAddress { get; set; }
            public BsonBoolean ack { get; set; }
            public BsonDateTime ackTimeTag { get; set; }
        }
        public class rtCommandNoAck
        {
            public BsonObjectId id { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolSourceConnectionNumber { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolSourceCommonAddress { get; set; }
            [BsonDefaultValue("")]
            public BsonString protocolSourceObjectAddress { get; set; }
            [BsonDefaultValue("")]
            public BsonString protocolSourceASDU { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolSourceCommandDuration { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean protocolSourceCommandUseSBO { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble pointKey { get; set; }
            [BsonDefaultValue("")]
            public BsonString tag { get; set; }
            public BsonDateTime timeTag { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble value { get; set; }
            [BsonDefaultValue("")]
            public BsonString valueString { get; set; }
            [BsonDefaultValue("")]
            public BsonString originatorUserName { get; set; }
            [BsonDefaultValue("")]
            public BsonString originatorIpAddress { get; set; }
        }

        [BsonIgnoreExtraElements]
        public class rtDataProtocDest
        {
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolDestinationConnectionNumber { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolDestinationCommonAddress { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolDestinationObjectAddress { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolDestinationASDU { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolDestinationCommandDuration { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean protocolDestinationCommandUseSBO { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolDestinationGroup { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(1.0)]
            public BsonDouble protocolDestinationKConv1 { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolDestinationKConv2 { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0.0)]
            public BsonDouble protocolDestinationHoursShift { get; set; }
        }

        [BsonIgnoreExtraElements]
        public class rtSourceDataUpdate
        {
            public BsonBoolean notTopicalAtSource { get; set; }
            public BsonBoolean invalidAtSource { get; set; }
            public BsonBoolean blockedAtSource { get; set; }
            public BsonBoolean substitutedAtSource { get; set; }
            public BsonBoolean carryAtSource { get; set; }
            public BsonBoolean overflowAtSource { get; set; }
            public BsonBoolean transientAtSource { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer))]
            public BsonDouble valueAtSource { get; set; }
            public BsonString valueStringAtSource { get; set; }
            public BsonString asduAtSource { get; set; }
            public BsonString causeOfTransmissionAtSource { get; set; }
            public BsonDateTime timeTagAtSource { get; set; }
            public BsonBoolean timeTagAtSourceOk { get; set; }
            public BsonDateTime timeTag { get; set; }
        }

        static void Log(string str, int level = 1)
        {
            if (LogLevel >= level)
            {
                var now = DateTime.Now;
                logMutex_.WaitOne();
                Console.Write($"[{now.ToString("o")}]"); // 2022-01-13T16:25:35.1250000+06:00
                Console.WriteLine(" " + str);
                logMutex_.ReleaseMutex();
            }
        }

        static void Log(Exception e, int level = 1)
        {
            Log(e.ToString(), level);
        }
        public class BsonDoubleSerializer : SerializerBase<BsonDouble> // generic permissive numeric deserializer resulting double
        { // read most types as double, write to double
            public override void Serialize(BsonSerializationContext context, BsonSerializationArgs args, BsonDouble dval)
            {
                context.Writer.WriteDouble(dval.ToDouble());
            }
            public override BsonDouble Deserialize(BsonDeserializationContext context, BsonDeserializationArgs args)
            {
                var type = context.Reader.GetCurrentBsonType();
                var dval = 0.0;
                string s;
                switch (type)
                {
                    case BsonType.Double:
                        return context.Reader.ReadDouble();
                    case BsonType.Null:
                        context.Reader.ReadNull();
                        break;
                    case BsonType.String:
                        s = context.Reader.ReadString();
                        try
                        {
                            dval = double.Parse(s);
                        }
                        catch (Exception)
                        {
                        }
                        break;
                    case BsonType.ObjectId:
                        s = context.Reader.ReadObjectId().ToString();
                        try
                        {
                            dval = double.Parse(s);
                        }
                        catch (Exception)
                        {
                        }
                        break;
                    case BsonType.JavaScript:
                        s = context.Reader.ReadJavaScript();
                        try
                        {
                            dval = double.Parse(s);
                        }
                        catch (Exception)
                        {
                        }
                        break;
                    case BsonType.Decimal128:
                        dval = Convert.ToDouble(context.Reader.ReadDecimal128());
                        break;
                    case BsonType.Boolean:
                        dval = Convert.ToDouble(context.Reader.ReadBoolean());
                        break;
                    case BsonType.Int32:
                        dval = context.Reader.ReadInt32();
                        break;
                    case BsonType.Int64:
                        dval = context.Reader.ReadInt64();
                        break;
                }
                return dval;
            }
        }

        public class BsonIntSerializer : SerializerBase<BsonInt32> // generic permissive numeric deserializer resulting int
        { // read most types as int but write to double
            public override void Serialize(BsonSerializationContext context, BsonSerializationArgs args, BsonInt32 ival)
            {
                context.Writer.WriteDouble(ival.ToDouble());
            }
            public override BsonInt32 Deserialize(BsonDeserializationContext context, BsonDeserializationArgs args)
            {
                var type = context.Reader.GetCurrentBsonType();
                var dval = 0.0;
                string s;
                switch (type)
                {
                    case BsonType.Int32:
                        return context.Reader.ReadInt32();
                    case BsonType.Int64:
                        dval = context.Reader.ReadInt64();
                        break;
                    case BsonType.Double:
                        dval = context.Reader.ReadDouble();
                        break;
                    case BsonType.Null:
                        context.Reader.ReadNull();
                        break;
                    case BsonType.String:
                        s = context.Reader.ReadString();
                        try
                        {
                            dval = double.Parse(s);
                        }
                        catch (Exception)
                        {
                        }
                        break;
                    case BsonType.ObjectId:
                        s = context.Reader.ReadObjectId().ToString();
                        try
                        {
                            dval = double.Parse(s);
                        }
                        catch (Exception)
                        {
                        }
                        break;
                    case BsonType.JavaScript:
                        s = context.Reader.ReadJavaScript();
                        try
                        {
                            dval = double.Parse(s);
                        }
                        catch (Exception)
                        {
                        }
                        break;
                    case BsonType.Decimal128:
                        dval = Convert.ToDouble(context.Reader.ReadDecimal128());
                        break;
                    case BsonType.Boolean:
                        dval = Convert.ToDouble(context.Reader.ReadBoolean());
                        break;
                }
                return Convert.ToInt32(dval);
            }
        }
        static byte[] GetBytesFromPEM(string pemString, string section)
        {
            var header = string.Format("-----BEGIN {0}-----", section);
            var footer = string.Format("-----END {0}-----", section);

            var start = pemString.IndexOf(header, StringComparison.Ordinal);
            if (start < 0)
                return null;

            start += header.Length;
            var end = pemString.IndexOf(footer, start, StringComparison.Ordinal) - start;

            if (end < 0)
                return null;

            return Convert.FromBase64String(pemString.Substring(start, end));
        }
        private static bool CertificateValidationCallBack(
           object sender,
           X509Certificate certificate,
           X509Chain chain,
           SslPolicyErrors sslPolicyErrors)
        {
            // If the certificate is a valid, signed certificate, return true.
            if (sslPolicyErrors == SslPolicyErrors.None)
            {
                Console.WriteLine("It's ok");
                return true;
            }

            // If there are errors in the certificate chain, look at each error to determine the cause.
            if ((sslPolicyErrors & SslPolicyErrors.RemoteCertificateChainErrors) != 0)
            {
                if (chain != null && chain.ChainStatus != null)
                {
                    foreach (X509ChainStatus status in chain.ChainStatus)
                    {
                        if ((certificate.Subject == certificate.Issuer) &&
                                (status.Status == X509ChainStatusFlags.UntrustedRoot))
                        {
                            // Self-signed certificates with an untrusted root are valid.
                            Console.WriteLine("Untrusted root certificate");
                            continue;
                        }
                        else
                        {
                            Console.WriteLine(sslPolicyErrors);
                            SslPolicyErrors ignoredErrors = SslPolicyErrors.None;
                            if (JSConfig.tlsAllowInvalidHostnames)
                                ignoredErrors |= SslPolicyErrors.RemoteCertificateNameMismatch; // name mismatch
                            if (JSConfig.tlsAllowChainErrors)
                                ignoredErrors |= SslPolicyErrors.RemoteCertificateChainErrors; // self-signed
                            if ((sslPolicyErrors & ~ignoredErrors) == SslPolicyErrors.None)
                            {
                                Console.WriteLine("FORCED ACCEPT CERTIFICATE!");
                                return true;
                            }

                            if (status.Status != X509ChainStatusFlags.NoError)
                            {

                                Console.WriteLine(status.StatusInformation);
                                // If there are any other errors in the certificate chain, the certificate is invalid,
                                // so the method returns false.
                                return false;
                            }
                        }
                    }
                }

                // When processing reaches this line, the only errors in the certificate chain are
                // untrusted root errors for self-signed certificates. These certificates are valid
                // for default Exchange server installations, so return true.
                Console.WriteLine("Certificates ok.");
                return true;
            }
            else
            {
                Console.WriteLine("Certificate Error!");
                // In all other cases, return false.
                return false;
            }
        }

        static MongoClient ConnectMongoClient(JSONSCADAConfig jsConfig)
        {
            // connect to MongoDB Database server
            MongoClientSettings settings = MongoClientSettings.FromUrl(new MongoUrl(jsConfig.mongoConnectionString));
            if (jsConfig.tlsClientPfxFile != null && jsConfig.tlsClientPfxFile != "")
            {

                var pem = System.IO.File.ReadAllText(jsConfig.tlsCaPemFile);
                byte[] certBuffer = GetBytesFromPEM(pem, "CERTIFICATE");
                var caCert = new X509Certificate2(certBuffer);

                var cliCert = new X509Certificate2(jsConfig.tlsClientPfxFile, jsConfig.tlsClientKeyPassword);
                settings.UseTls = true;
                settings.AllowInsecureTls = true;
                settings.SslSettings = new SslSettings
                {
                    ClientCertificates = new[] { caCert, cliCert },
                    CheckCertificateRevocation = false,
                    ServerCertificateValidationCallback = CertificateValidationCallBack
                };
            }

            return new MongoClient(settings);
        }
    }
}