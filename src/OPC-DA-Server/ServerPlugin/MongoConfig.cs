/*
 * OPC-DA Server Driver for {json:scada}
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
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
using MongoDB.Bson;
using MongoDB.Bson.Serialization;
using MongoDB.Bson.Serialization.Attributes;
using MongoDB.Bson.Serialization.Serializers;
using MongoDB.Driver;

namespace ServerPlugin
{
    // -------------------------------------------------------------------------
    // JSON-SCADA system configuration (read from json-scada.json)
    // -------------------------------------------------------------------------
    public class JSONSCADAConfig
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

    // -------------------------------------------------------------------------
    // protocolDriverInstances collection document
    // -------------------------------------------------------------------------
    [BsonIgnoreExtraElements]
    public class ProtocolDriverInstancesClass
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

    // -------------------------------------------------------------------------
    // protocolConnections collection document for OPC-DA_SERVER driver
    // -------------------------------------------------------------------------
    [BsonIgnoreExtraElements]
    public class OpcDaConnection
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

        [BsonDefaultValue("")]
        public string description { get; set; }

        [BsonDefaultValue(true)]
        public bool enabled { get; set; }

        [BsonDefaultValue(true)]
        public bool commandsEnabled { get; set; }

        // group1 values to expose as OPC-DA items (matches OPC-UA-Server 'topics')
        [BsonDefaultValue(new string[] { })]
        public string[] topics { get; set; }

        // OPC-DA COM registration identity (must be unique per server instance)
        [BsonDefaultValue("{A1B2C3D4-0001-0001-0001-A1B2C3D40002}")]
        public string clsIdServer { get; set; }

        [BsonDefaultValue("{A1B2C3D4-0001-0001-0001-A1B2C3D40001}")]
        public string clsIdApp { get; set; }

        [BsonDefaultValue("JsonScada.OpcDaServer")]
        public string prgIdServer { get; set; }

        [BsonDefaultValue("JsonScada.OpcDaServer.1")]
        public string prgIdCurrServer { get; set; }

        [BsonDefaultValue("JSON-SCADA OPC-DA Server")]
        public string serverName { get; set; }
    }

    // -------------------------------------------------------------------------
    // realtimeData collection document projection
    // -------------------------------------------------------------------------
    [BsonIgnoreExtraElements]
    public class RtDataItem
    {
        [BsonSerializer(typeof(BsonIntSerializer))]
        public BsonInt32 _id { get; set; }

        [BsonDefaultValue("")]
        public string tag { get; set; }

        [BsonDefaultValue("")]
        public string protocolSourceBrowsePath { get; set; }

        [BsonDefaultValue("analog")]
        public string type { get; set; }          // "digital", "analog", "string", "json"

        [BsonSerializer(typeof(BsonDoubleSerializer))]
        public BsonDouble value { get; set; } = new BsonDouble(0.0);

        [BsonDefaultValue("")]
        public string valueString { get; set; }

        [BsonDefaultValue(null)]
        public BsonDateTime timeTag { get; set; }

        [BsonDefaultValue(null)]
        public BsonDateTime timeTagAtSource { get; set; }

        [BsonDefaultValue(false)]
        public BsonBoolean timeTagAtSourceOk { get; set; }

        [BsonDefaultValue(false)]
        public BsonBoolean invalid { get; set; }

        [BsonDefaultValue("")]
        public string origin { get; set; }        // "command" = client-writable

        [BsonDefaultValue("")]
        public string description { get; set; }

        [BsonDefaultValue("")]
        public string ungroupedDescription { get; set; }

        [BsonDefaultValue("")]
        public string group1 { get; set; }

        [BsonDefaultValue("")]
        public string group2 { get; set; }

        [BsonDefaultValue("")]
        public string group3 { get; set; }

        [BsonSerializer(typeof(BsonDoubleSerializer))]
        public BsonDouble protocolSourceConnectionNumber { get; set; } = new BsonDouble(0.0);

        [BsonSerializer(typeof(BsonStringSerializer)), BsonDefaultValue("double")]
        public string protocolSourceASDU { get; set; }   // canonical OPC type hint

        [BsonSerializer(typeof(BsonDoubleSerializer))]
        public BsonDouble protocolSourceObjectAddress { get; set; } = new BsonDouble(0.0);

        [BsonSerializer(typeof(BsonDoubleSerializer))]
        public BsonDouble protocolSourceAccessLevel { get; set; } = new BsonDouble(0.0);

        [BsonDefaultValue(null)]
        public RtDataProtocDest[] protocolDestinations { get; set; }
    }

    // -------------------------------------------------------------------------
    // protocolDestinations sub-document (used for server→client push drivers)
    // -------------------------------------------------------------------------
    [BsonIgnoreExtraElements]
    public class RtDataProtocDest
    {
        [BsonSerializer(typeof(BsonDoubleSerializer))]
        public BsonDouble protocolDestinationConnectionNumber { get; set; } = new BsonDouble(0.0);

        [BsonSerializer(typeof(BsonDoubleSerializer))]
        public BsonDouble protocolDestinationObjectAddress { get; set; } = new BsonDouble(0.0);

        [BsonSerializer(typeof(BsonDoubleSerializer))]
        public BsonDouble protocolDestinationASDU { get; set; } = new BsonDouble(0.0);

        [BsonSerializer(typeof(BsonDoubleSerializer))]
        public BsonDouble protocolDestinationKConv1 { get; set; } = new BsonDouble(1.0);

        [BsonSerializer(typeof(BsonDoubleSerializer))]
        public BsonDouble protocolDestinationKConv2 { get; set; } = new BsonDouble(0.0);
    }

    // -------------------------------------------------------------------------
    // BSON serializers (permissive numeric conversion — same as iec104server)
    // -------------------------------------------------------------------------

    public class BsonDoubleSerializer : SerializerBase<BsonDouble>
    {
        public override void Serialize(BsonSerializationContext context, BsonSerializationArgs args, BsonDouble dval)
            => context.Writer.WriteDouble(dval.ToDouble());

        public override BsonDouble Deserialize(BsonDeserializationContext context, BsonDeserializationArgs args)
        {
            var type = context.Reader.GetCurrentBsonType();
            double dval = 0.0;
            switch (type)
            {
                case BsonType.Double:  dval = context.Reader.ReadDouble(); break;
                case BsonType.Null:    context.Reader.ReadNull(); break;
                case BsonType.String:
                    double.TryParse(context.Reader.ReadString(), out dval); break;
                case BsonType.Decimal128:
                    dval = Convert.ToDouble(context.Reader.ReadDecimal128()); break;
                case BsonType.Boolean:
                    dval = Convert.ToDouble(context.Reader.ReadBoolean()); break;
                case BsonType.Int32:   dval = context.Reader.ReadInt32(); break;
                case BsonType.Int64:   dval = context.Reader.ReadInt64(); break;
                default:               context.Reader.SkipValue(); break;
            }
            return new BsonDouble(dval);
        }
    }

    public class BsonIntSerializer : SerializerBase<BsonInt32>
    {
        public override void Serialize(BsonSerializationContext context, BsonSerializationArgs args, BsonInt32 ival)
            => context.Writer.WriteDouble(ival.ToDouble());

        public override BsonInt32 Deserialize(BsonDeserializationContext context, BsonDeserializationArgs args)
        {
            var type = context.Reader.GetCurrentBsonType();
            int ival = 0;
            switch (type)
            {
                case BsonType.Int32:   ival = context.Reader.ReadInt32(); break;
                case BsonType.Int64:   ival = Convert.ToInt32(context.Reader.ReadInt64()); break;
                case BsonType.Double:  ival = Convert.ToInt32(context.Reader.ReadDouble()); break;
                case BsonType.Null:    context.Reader.ReadNull(); break;
                case BsonType.String:
                    double.TryParse(context.Reader.ReadString(), out var d);
                    ival = Convert.ToInt32(d);
                    break;
                case BsonType.Decimal128:
                    ival = Convert.ToInt32(context.Reader.ReadDecimal128()); break;
                case BsonType.Boolean:
                    ival = Convert.ToInt32(context.Reader.ReadBoolean()); break;
                default:               context.Reader.SkipValue(); break;
            }
            return new BsonInt32(ival);
        }
    }

    public class BsonStringSerializer : SerializerBase<string>
    {
        public override void Serialize(BsonSerializationContext context, BsonSerializationArgs args, string value)
        {
            if (value == null)
                context.Writer.WriteNull();
            else
                context.Writer.WriteString(value);
        }

        public override string Deserialize(BsonDeserializationContext context, BsonDeserializationArgs args)
        {
            var type = context.Reader.GetCurrentBsonType();
            switch (type)
            {
                case BsonType.String:  return context.Reader.ReadString();
                case BsonType.Double:  return context.Reader.ReadDouble().ToString();
                case BsonType.Int32:   return context.Reader.ReadInt32().ToString();
                case BsonType.Int64:   return context.Reader.ReadInt64().ToString();
                case BsonType.Boolean: return context.Reader.ReadBoolean().ToString();
                case BsonType.Null:    context.Reader.ReadNull(); return "";
                default:
                    context.Reader.SkipValue();
                    return "";
            }
        }
    }

    // -------------------------------------------------------------------------
    // Config file loader
    // -------------------------------------------------------------------------
    public static class ConfigLoader
    {
        public static readonly string DefaultConfigPath  = @"../conf/json-scada.json";
        public static readonly string FallbackConfigPath = @"c:/json-scada/conf/json-scada.json";

        public static JSONSCADAConfig Load(string[] cmdArgs)
        {
            // args: [instanceNumber] [logLevel] [configFilePath]
            string fname = cmdArgs.Length > 2 ? cmdArgs[2] : DefaultConfigPath;
            if (!File.Exists(fname))
                fname = FallbackConfigPath;
            if (!File.Exists(fname))
                throw new FileNotFoundException($"Config file not found: {fname}");

            var json = File.ReadAllText(fname);
            var cfg  = BsonSerializer.Deserialize<JSONSCADAConfig>(json);

            if (string.IsNullOrEmpty(cfg?.mongoConnectionString))
                throw new InvalidOperationException("Missing mongoConnectionString in config.");
            if (string.IsNullOrEmpty(cfg?.mongoDatabaseName))
                throw new InvalidOperationException("Missing mongoDatabaseName in config.");
            if (string.IsNullOrEmpty(cfg?.nodeName))
                throw new InvalidOperationException("Missing nodeName in config.");

            return cfg;
        }

        public static MongoClient ConnectMongo(JSONSCADAConfig cfg)
        {
            var settings = MongoClientSettings.FromConnectionString(cfg.mongoConnectionString);
            settings.ApplicationName = "OPC-DA-SERVER";
            settings.ServerSelectionTimeout = TimeSpan.FromSeconds(30);
            return new MongoClient(settings);
        }
    }
}
