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

namespace OPCUAClientDriver
{
    partial class MainClass
    {
        public static Int32 PointKeyInsert = 100000;
        public static string JsonConfigFilePath = @"../conf/json-scada.json";
        public static string JsonConfigFilePathAlt = @"c:/json-scada/conf/json-scada.json";
        public static Int32 LogLevelNoLog = 0; // log level 0=no
        public static Int32 LogLevelBasic = 1; // log level 1=basic (default)
        public static Int32 LogLevelDetailed = 2; // log level 2=detailed
        public static Int32 LogLevelDebug = 3; // log level 3=debug
        public static Int32 LogLevel = 1; // log level 0=no 1=min
        private static Mutex LogMutex = new Mutex();
        public static JSONSCADAConfig JSConfig;
        public static protocolDriverInstancesClass DriverInstance = null;
        public static String
            ProtocolConnectionsCollectionName = "protocolConnections";
        public static String
            ProtocolDriverInstancesCollectionName = "protocolDriverInstances";
        public static String RealtimeDataCollectionName = "realtimeData";
        public static String SoeDataCollectionName = "soeData";
        public static String CommandsQueueCollectionName = "commandsQueue";
        public static Int32 ProtocolDriverInstanceNumber = 1;
        public static String redundantIpAddress = "";
        public static ConcurrentQueue<OPC_Value>
            OPCDataQueue = new ConcurrentQueue<OPC_Value>(); // acquired values queue (to be updated in mongodb realtime data collection)
        public static List<OPCUA_connection>
            OPCUAconns = new List<OPCUA_connection>(); // list of RTU connections

        public class
        JSONSCADAConfig // base configuration of the system (how to reach mongodb, etc.)
        {
            public String nodeName { get; set; }
            public String mongoConnectionString { get; set; }
            public String mongoDatabaseName { get; set; }
            public String tlsCaPemFile { get; set; }
            public String tlsClientPemFile { get; set; }
            public String tlsClientPfxFile { get; set; }
            public String tlsClientKeyPassword { get; set; }
            public bool tlsAllowInvalidHostnames { get; set; }
            public bool tlsAllowChainErrors { get; set; }
            public bool tlsInsecure { get; set; }
        }
        [BsonIgnoreExtraElements]
        public class
        OPCUA_connection // protocol connection to RTU
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
            public string[] endpointURLs { get; set; }
            [BsonDefaultValue("../conf/Opc.Ua.DefaultClient.Config.xml")]
            public string configFileName { get; set; }
            [BsonDefaultValue(true)]
            public bool autoCreateTags { get; set; }
            [BsonDefaultValue(5.0)]
            public double autoCreateTagPublishingInterval { get; set; }
            [BsonDefaultValue(5.0)]
            public double autoCreateTagSamplingInterval { get; set; }
            [BsonDefaultValue(5.0)]
            public double autoCreateTagQueueSize { get; set; }
            [BsonDefaultValue(20000.0)]
            public double timeoutMs { get; set; }
            [BsonDefaultValue(false)]
            public bool useSecurity { get; set; }
            public Double LastNewKeyCreated;
            public SortedSet<string> InsertedTags = new SortedSet<string>();
            public OPCUAClient connection;
            public Thread thrOPCStack;
        }
        [BsonIgnoreExtraElements]
        public class protocolDriverInstancesClass
        {
            public ObjectId Id { get; set; }
            public Int32 protocolDriverInstanceNumber { get; set; } = 1;
            public String protocolDriver { get; set; } = "";
            public Boolean enabled { get; set; } = true;
            public Int32 logLevel { get; set; } = 1;
            public String[] nodeNames { get; set; } = Array.Empty<string>();
            public String activeNodeName { get; set; } = "";
            public DateTime activeNodeKeepAliveTimeTag { get; set; } = DateTime.MinValue;
            public Boolean keepProtocolRunningWhileInactive { get; set; } = false;
        }
        public class OPC_Value
        {
            public string valueJson;
            public bool selfPublish;
            public string address;
            public string asdu;
            public bool isDigital;
            public double value;
            public string valueString;
            public int cot;
            public DateTime serverTimestamp;
            public DateTime sourceTimestamp;
            public bool hasSourceTimestamp;
            public bool quality;
            public int conn_number;
            public string conn_name;
            public string common_address;
            public string display_name;
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
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolSourceConnectionNumber { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolSourceCommonAddress { get; set; }
            [BsonDefaultValue("")]
            public BsonString protocolSourceObjectAddress { get; set; }
            [BsonDefaultValue("")]
            public BsonString protocolSourceASDU { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolSourceCommandDuration { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean protocolSourceCommandUseSBO { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)),BsonDefaultValue(0)]
            public BsonDouble pointKey { get; set; }
            [BsonDefaultValue("")]
            public BsonString tag { get; set; }
            public BsonDateTime timeTag { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
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
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolSourceConnectionNumber { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolSourceCommonAddress { get; set; }
            [BsonDefaultValue("")]
            public BsonString protocolSourceObjectAddress { get; set; }
            [BsonDefaultValue("")]
            public BsonString protocolSourceASDU { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolSourceCommandDuration { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean protocolSourceCommandUseSBO { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)),BsonDefaultValue(0)]
            public BsonDouble pointKey { get; set; }
            [BsonDefaultValue("")]
            public BsonString tag { get; set; }
            public BsonDateTime timeTag { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)),BsonDefaultValue(0)]            
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
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolDestinationConnectionNumber { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolDestinationCommonAddress { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolDestinationObjectAddress { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolDestinationASDU { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolDestinationCommandDuration { get; set; }
            [BsonDefaultValue(false)]
            public BsonBoolean protocolDestinationCommandUseSBO { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)),BsonDefaultValue(0)]
            public BsonDouble protocolDestinationGroup { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(1)]
            public BsonDouble protocolDestinationKConv1 { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolDestinationKConv2 { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
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
                LogMutex.WaitOne();
                Console.Write($"[{now.ToString("o")}]"); // 2022-01-13T16:25:35.1250000+06:00
                Console.WriteLine(" " + str);
                LogMutex.ReleaseMutex();
            }
        }

        static void Log(System.Exception e, int level = 1)
        {
            Log(e.ToString(), level);
        }
        public class BsonDoubleSerializer : SerializerBase<BsonDouble> // generic permissive numeric deserializer resulting double
        { // read most types as double, write to double
            public override void Serialize(MongoDB.Bson.Serialization.BsonSerializationContext context, MongoDB.Bson.Serialization.BsonSerializationArgs args, BsonDouble dval)
            {
                context.Writer.WriteDouble(dval.ToDouble());
            }
            public override BsonDouble Deserialize(BsonDeserializationContext context, BsonDeserializationArgs args)
            {
                var type = context.Reader.GetCurrentBsonType();
                var dval = 0.0;
                String s;
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
                        dval = System.Convert.ToDouble(context.Reader.ReadDecimal128());
                        break;
                    case BsonType.Boolean:
                        dval = System.Convert.ToDouble(context.Reader.ReadBoolean());
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
            public override void Serialize(MongoDB.Bson.Serialization.BsonSerializationContext context, MongoDB.Bson.Serialization.BsonSerializationArgs args, BsonInt32 ival)
            {
                context.Writer.WriteDouble(ival.ToDouble());
            }
            public override BsonInt32 Deserialize(BsonDeserializationContext context, BsonDeserializationArgs args)
            {
                var type = context.Reader.GetCurrentBsonType();
                var dval = 0.0;
                String s;
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
                        dval = System.Convert.ToDouble(context.Reader.ReadDecimal128());
                        break;
                    case BsonType.Boolean:
                        dval = System.Convert.ToDouble(context.Reader.ReadBoolean());
                        break;
                }
                return System.Convert.ToInt32(dval);
            }
        }
        static byte[] GetBytesFromPEM(string pemString, string section)
        {
            var header = String.Format("-----BEGIN {0}-----", section);
            var footer = String.Format("-----END {0}-----", section);

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
           System.Security.Cryptography.X509Certificates.X509Certificate certificate,
           System.Security.Cryptography.X509Certificates.X509Chain chain,
           System.Net.Security.SslPolicyErrors sslPolicyErrors)
        {
            // If the certificate is a valid, signed certificate, return true.
            if (sslPolicyErrors == System.Net.Security.SslPolicyErrors.None)
            {
                Console.WriteLine("It's ok");
                return true;
            }

            // If there are errors in the certificate chain, look at each error to determine the cause.
            if ((sslPolicyErrors & System.Net.Security.SslPolicyErrors.RemoteCertificateChainErrors) != 0)
            {
                if (chain != null && chain.ChainStatus != null)
                {
                    foreach (System.Security.Cryptography.X509Certificates.X509ChainStatus status in chain.ChainStatus)
                    {
                        if ((certificate.Subject == certificate.Issuer) &&
                                (status.Status == System.Security.Cryptography.X509Certificates.X509ChainStatusFlags.UntrustedRoot))
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

                            if (status.Status != System.Security.Cryptography.X509Certificates.X509ChainStatusFlags.NoError)
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