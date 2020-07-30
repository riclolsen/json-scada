// IEC60870-104 Server/Client Protocol driver for JSON SCADA
// Copyright 2020 Ricardo Lastra Olsen
using System;
using lib60870;
using lib60870.CS101;
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

namespace Iec10XDriver
{
    partial class MainClass
    {
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
        public static ConcurrentQueue<IEC_Value>
            IECDataQueue = new ConcurrentQueue<IEC_Value>(); // acquired values queue (to be updated in mongodb realtime data collection)
        public static ConcurrentQueue<IEC_CmdAck>
            IECCmdAckQueue = new ConcurrentQueue<IEC_CmdAck>(); // command acknowledges queue (to be updated in mongodb commands collection)
        public static List<IEC10X_connection>
            IEC10Xconns = new List<IEC10X_connection>(); // list of RTU connections

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
        public class protocolDriverInstancesClass
        {
            public ObjectId Id { get; set; }
            public Int32 protocolDriverInstanceNumber { get; set; } = 1;
            public String protocolDriver { get; set; } = "";
            public Boolean enabled { get; set; } = true;
            public Int32 logLevel { get; set; } = 1;
            public String[] nodeNames { get; set; } = new string[0];
            public String activeNodeName { get; set; } = "";
            public DateTime activeNodeKeepAliveTimeTag { get; set; } = DateTime.MinValue;
            public Boolean keepProtocolRunningWhileInactive { get; set; } = false;
        }
        public struct IEC_Value
        {
            public int address;
            public lib60870.CS101.TypeID asdu;
            public bool isDigital;
            public double value;
            public int cot;
            public DateTime serverTimestamp;
            public bool hasSourceTimestampCP24;
            public bool hasSourceTimestampCP56;
            public CP24Time2a sourceTimestampCP24;
            public CP56Time2a sourceTimestampCP56;
            public lib60870.CS101.QualityDescriptor quality;
            public int conn_number;
            public int common_address;
        }
        public struct IEC_CmdAck
        {
            public bool ack; // ack positive(true) or negative(false)
            public int conn_number;
            public int object_address;
            public DateTime ack_time_tag;
        }
        public class rtFilt
        {
            public int protocolSourceConnectionNumber;
            public int protocolSourceCommonAddress;
            public int protocolSourceObjectAddress;
        }
        [BsonIgnoreExtraElements]
        public class rtCommand
        {
            public BsonObjectId id { get; set; }
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
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolSourceObjectAddress { get; set; }
            [BsonSerializer(typeof(BsonDoubleSerializer)), BsonDefaultValue(0)]
            public BsonDouble protocolSourceASDU { get; set; }
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
            var now = DateTime.Now;
            if (LogLevel >= level)
            {
                LogMutex.WaitOne();
                Console.Write(now + ":");
                Console.Write("{0,3:D3}", now.Millisecond);
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
        static InformationObject
        BuildInfoObj(
            Int32 asdu,
            Int32 addr,
            Double value,
            Boolean sbo = false,
            Int32 cmdqualif = 0,
            QualityDescriptor quality = null,
            CP56Time2a time_tag = null,
            Double kconv1 = 1,
            Double kconv2 = 0,
            Boolean transient = false
        )
        {
            InformationObject sc = null;

            if (time_tag == null)
                time_tag = new CP56Time2a(DateTime.Now);
            else
            { // has time tag, so change ASDU if necessary to embed a timetag                
                var maptowithcp56time = new Dictionary<TypeID, TypeID>();
                maptowithcp56time.Add(TypeID.M_SP_NA_1, TypeID.M_SP_TB_1);
                maptowithcp56time.Add(TypeID.M_DP_NA_1, TypeID.M_DP_TB_1);
                maptowithcp56time.Add(TypeID.M_ST_NA_1, TypeID.M_ST_TB_1);
                maptowithcp56time.Add(TypeID.M_BO_NA_1, TypeID.M_BO_TB_1);
                maptowithcp56time.Add(TypeID.M_ME_NA_1, TypeID.M_ME_TD_1);
                maptowithcp56time.Add(TypeID.M_ME_NB_1, TypeID.M_ME_TE_1);
                maptowithcp56time.Add(TypeID.M_ME_NC_1, TypeID.M_ME_TF_1);
                maptowithcp56time.Add(TypeID.M_IT_NA_1, TypeID.M_IT_TB_1);
                if (maptowithcp56time.TryGetValue((TypeID)asdu, out var newasdu))
                {
                    asdu = (Int32)newasdu;
                }
            }

            Boolean bval;
            Int32 ival;
            UInt32 uival;

            switch ((TypeID)asdu)
            {
                case TypeID.M_SP_NA_1: // 1
                    bval = value != 0 ? true : false;
                    if (kconv1 == -1)
                        bval = !bval;
                    sc =
                         new SinglePointInformation(addr,
                            bval,
                            quality);
                    break;
                case TypeID.M_SP_TB_1: // 30
                    bval = value != 0 ? true : false;
                    if (kconv1 == -1)
                        bval = !bval;
                    sc =
                         new SinglePointWithCP56Time2a(addr,
                            bval,
                            quality,
                            time_tag
                            );
                    break;
                case TypeID.M_DP_NA_1: // 3
                    if (transient)
                        sc =
                             new DoublePointInformation(addr,
                                DoublePointValue.INTERMEDIATE,
                                quality);
                    else
                    if (kconv1 == -1)
                        sc =
                             new DoublePointInformation(addr,
                                value != 0 ? DoublePointValue.OFF : DoublePointValue.ON,
                                quality);
                    else
                        sc =
                             new DoublePointInformation(addr,
                                value != 0 ? DoublePointValue.ON : DoublePointValue.OFF,
                                quality);
                    break;
                case TypeID.M_DP_TB_1: // 31
                    if (transient)
                        sc =
                             new DoublePointWithCP56Time2a(addr,
                                DoublePointValue.INTERMEDIATE,
                                quality,
                                time_tag);
                    else
                    if (kconv1 == -1)
                        sc =
                             new DoublePointWithCP56Time2a(addr,
                                value != 0 ? DoublePointValue.OFF : DoublePointValue.ON,
                                quality,
                                time_tag);
                    else
                        sc =
                             new DoublePointWithCP56Time2a(addr,
                                value != 0 ? DoublePointValue.ON : DoublePointValue.OFF,
                                quality,
                                time_tag);
                    break;
                case TypeID.M_ST_NA_1: // 5
                    value = value * kconv1 + kconv2;
                    if (value > 63)
                    {
                        value = 63;
                        quality.Overflow = true;
                    }
                    else
                    if (value < -64)
                    {
                        value = -64;
                        quality.Overflow = true;
                    }
                    sc = new StepPositionInformation(addr,
                                                System.Convert.ToInt16(value),
                                                transient,
                                                quality);
                    break;
                case TypeID.M_ST_TB_1: // 32
                    value = value * kconv1 + kconv2;
                    if (value > 63)
                    {
                        value = 63;
                        quality.Overflow = true;
                    }
                    else
                    if (value < -64)
                    {
                        value = -64;
                        quality.Overflow = true;
                    }
                    sc = new StepPositionWithCP56Time2a(addr,
                                                System.Convert.ToInt16(value),
                                                transient,
                                                quality,
                                                time_tag);
                    break;
                case TypeID.M_ME_NA_1: // 9
                    value = value * kconv1 + kconv2;
                    if (value > 32767)
                    {
                        value = 32767;
                        quality.Overflow = true;
                    }
                    else
                    if (value < -32768)
                    {
                        value = -32768;
                        quality.Overflow = true;
                    }
                    sc = new MeasuredValueNormalized(addr,
                                                System.Convert.ToInt16(value),
                                                quality);
                    break;
                case TypeID.M_ME_ND_1: // 21
                    value = value * kconv1 + kconv2;
                    if (value > 32767)
                    {
                        value = 32767;
                        quality.Overflow = true;
                    }
                    else
                    if (value < -32768)
                    {
                        value = -32768;
                        quality.Overflow = true;
                    }
                    sc = new MeasuredValueNormalizedWithoutQuality(addr,
                                                System.Convert.ToInt16(value));
                    break;
                case TypeID.M_ME_TD_1: // 34
                    value = value * kconv1 + kconv2;
                    if (value > 32767)
                    {
                        value = 32767;
                        quality.Overflow = true;
                    }
                    else
                    if (value < -32768)
                    {
                        value = -32768;
                        quality.Overflow = true;
                    }
                    sc = new MeasuredValueNormalizedWithCP56Time2a(addr,
                                                System.Convert.ToInt16(value),
                                                quality,
                                                time_tag);
                    break;
                case TypeID.M_ME_NB_1: // 11
                    value = value * kconv1 + kconv2;
                    if (value > 32767)
                    {
                        value = 32767;
                        quality.Overflow = true;
                    }
                    else
                    if (value < -32768)
                    {
                        value = -32768;
                        quality.Overflow = true;
                    }
                    sc = new MeasuredValueScaled(addr,
                                                System.Convert.ToInt16(value),
                                                quality);
                    break;
                case TypeID.M_ME_TE_1: // 35
                    value = value * kconv1 + kconv2;
                    if (value > 32767)
                    {
                        value = 32767;
                        quality.Overflow = true;
                    }
                    else
                    if (value < -32768)
                    {
                        value = -32768;
                        quality.Overflow = true;
                    }
                    sc = new MeasuredValueScaledWithCP56Time2a(addr,
                                                System.Convert.ToInt16(value),
                                                quality,
                                                time_tag);
                    break;
                case TypeID.M_ME_NC_1: // 13
                    value = value * kconv1 + kconv2;
                    sc = new MeasuredValueShort(addr,
                                                System.Convert.ToSingle(value),
                                                quality);
                    break;
                case TypeID.M_ME_TF_1: // 36   
                    value = value * kconv1 + kconv2;
                    sc = new MeasuredValueShortWithCP56Time2a(addr,
                                                System.Convert.ToSingle(value),
                                                quality,
                                                time_tag);
                    break;
                case TypeID.M_IT_NA_1: //15
                    break;
                case TypeID.M_IT_TB_1: // 37
                    break;
                case TypeID.M_PS_NA_1: // 20
                    Log(" TODO Packed single point information with status change detection! ");
                    break;
                case TypeID.M_EP_TD_1: // 38
                    break;
                case TypeID.M_EP_TE_1: // 39
                    break;
                case TypeID.M_EP_TF_1: // 40
                    break;

                case TypeID.M_BO_NA_1: // 7
                    uival = System.Convert.ToUInt32(value);
                    if (kconv1 == -1)
                        uival = ~uival;
                    sc = new Bitstring32(addr, uival, quality);
                    break;
                case TypeID.M_BO_TB_1: // 33
                    uival = System.Convert.ToUInt32(value);
                    if (kconv1 == -1)
                        uival = ~uival;
                    sc = new Bitstring32WithCP56Time2a(addr, uival, quality, time_tag);
                    break;
                case TypeID.C_SC_NA_1: // 45
                    bval = value != 0 ? true : false;
                    if (kconv1 == -1)
                        bval = !bval;
                    sc =
                        new SingleCommand(addr,
                            bval,
                            sbo,
                            cmdqualif);
                    break;
                case TypeID.C_DC_NA_1: // 46
                    if (kconv1 == -1)
                        ival = value != 0 ? System.Convert.ToInt32(DoublePointValue.OFF) : System.Convert.ToInt32(DoublePointValue.ON);
                    else
                        ival = value != 0 ? System.Convert.ToInt32(DoublePointValue.ON) : System.Convert.ToInt32(DoublePointValue.OFF);
                    sc =
                        new DoubleCommand(addr,
                            ival,
                            sbo,
                            cmdqualif);
                    break;
                case TypeID.C_RC_NA_1: // 47
                    if (kconv1 == -1)
                        sc =
                            new StepCommand(addr,
                                value >= 1
                                    ? StepCommandValue.LOWER
                                    : StepCommandValue.HIGHER,
                                sbo,
                                cmdqualif);
                    else
                        sc =
                            new StepCommand(addr,
                                value >= 1
                                    ? StepCommandValue.HIGHER
                                    : StepCommandValue.LOWER,
                                sbo,
                                cmdqualif);
                    break;
                case TypeID.C_SE_NA_1: // 48
                    value = value * kconv1 + kconv2;
                    if (value > 32767)
                        value = 32767;
                    else
                    if (value < -32768)
                        value = -32768;
                    sc =
                        new SetpointCommandNormalized(addr,
                            System.Convert.ToInt16(value),
                            new SetpointCommandQualifier(sbo, 0));
                    break;
                case TypeID.C_SE_NB_1: // 49
                    value = value * kconv1 + kconv2;
                    if (value > 32767)
                        value = 32767;
                    else
                    if (value < -32768)
                        value = -32768;
                    sc =
                        new SetpointCommandScaled(addr,
                            new ScaledValue(System.Convert.ToInt16(value)),
                            new SetpointCommandQualifier(sbo, 0));
                    break;
                case TypeID.C_SE_NC_1: // 50
                    value = value * kconv1 + kconv2;
                    sc =
                        new SetpointCommandShort(addr,
                            System.Convert.ToSingle(value),
                            new SetpointCommandQualifier(sbo, 0));
                    break;
                case TypeID.C_BO_NA_1: // 51
                    uival = System.Convert.ToUInt32(value);
                    if (kconv1 == -1)
                        uival = ~uival;
                    sc =
                        new Bitstring32Command(addr,
                            uival);
                    break;
                case TypeID.C_SC_TA_1: //  58
                    bval = value != 0 ? true : false;
                    if (kconv1 == -1)
                        bval = !bval;
                    sc =
                        new SingleCommandWithCP56Time2a(addr,
                            bval,
                            sbo,
                            cmdqualif,
                            time_tag);
                    break;
                case TypeID.C_DC_TA_1: // 59
                    if (kconv1 == -1)
                        ival = value != 0 ? System.Convert.ToInt32(DoublePointValue.OFF) : System.Convert.ToInt32(DoublePointValue.ON);
                    else
                        ival = value != 0 ? System.Convert.ToInt32(DoublePointValue.ON) : System.Convert.ToInt32(DoublePointValue.OFF);
                    sc =
                        new DoubleCommandWithCP56Time2a(addr,
                            ival,
                            sbo,
                            cmdqualif,
                            time_tag);
                    break;
                case TypeID.C_RC_TA_1: // 60
                    if (kconv1 == -1)
                        sc =
                            new StepCommandWithCP56Time2a(addr,
                                value >= 1
                                    ? StepCommandValue.LOWER
                                    : StepCommandValue.HIGHER,
                                sbo,
                                cmdqualif,
                                time_tag);
                    else
                        sc =
                            new StepCommandWithCP56Time2a(addr,
                                value >= 1
                                    ? StepCommandValue.HIGHER
                                    : StepCommandValue.LOWER,
                                sbo,
                                cmdqualif,
                                time_tag);
                    break;
                case TypeID.C_SE_TA_1: // 61
                    value = value * kconv1 + kconv2;
                    if (value > 32767)
                        value = 32767;
                    else
                    if (value < -32768)
                        value = -32768;
                    sc =
                        new SetpointCommandNormalizedWithCP56Time2a(addr,
                            System.Convert.ToInt16(value),
                            new SetpointCommandQualifier(sbo, 0),
                            time_tag);
                    break;
                case TypeID.C_SE_TB_1: // 62
                    value = value * kconv1 + kconv2;
                    if (value > 32767)
                        value = 32767;
                    else
                    if (value < -32768)
                        value = -32768;
                    sc =
                        new SetpointCommandScaledWithCP56Time2a(addr,
                            new ScaledValue(System.Convert.ToInt16(value)),
                            new SetpointCommandQualifier(sbo, 0),
                            time_tag);
                    break;
                case TypeID.C_SE_TC_1: // 63
                    value = value * kconv1 + kconv2;
                    sc =
                        new SetpointCommandShortWithCP56Time2a(addr,
                            System.Convert.ToSingle(value),
                            new SetpointCommandQualifier(sbo, 0),
                            time_tag);
                    break;
                case TypeID.C_BO_TA_1: // 64
                    uival = System.Convert.ToUInt32(value);
                    if (kconv1 == -1)
                        uival = ~uival;
                    sc =
                        new Bitstring32CommandWithCP56Time2a(addr,
                            uival,
                            time_tag);
                    break;
                case TypeID.C_IC_NA_1: // 100
                    sc =
                        new InterrogationCommand(0,
                            System.Convert.ToByte(cmdqualif));
                    break;
                case TypeID.C_CI_NA_1: // 101
                    sc =
                        new CounterInterrogationCommand(0,
                            System.Convert.ToByte(cmdqualif));
                    break;
                case TypeID.C_RD_NA_1: // 102
                    sc = new ReadCommand(addr);
                    break;
                case TypeID.C_CS_NA_1: // 103
                    sc = new ClockSynchronizationCommand(0, time_tag);
                    break;
                case TypeID.C_RP_NA_1: // 105
                    sc =
                        new ResetProcessCommand(addr,
                            System.Convert.ToByte(cmdqualif));
                    break;
                case TypeID.C_TS_TA_1: // 107
                    sc =
                        new TestCommandWithCP56Time2a(System
                                .Convert
                                .ToUInt16(value),
                            time_tag);
                    break;
                case TypeID.P_ME_NA_1: // 110
                    value = value * kconv1 + kconv2;
                    if (value > 32767)
                        value = 32767;
                    else
                    if (value < -32768)
                        value = -32768;
                    sc =
                        new ParameterNormalizedValue(System
                                .Convert
                                .ToInt32(addr),
                            System.Convert.ToInt32(value),
                            System.Convert.ToByte(cmdqualif));
                    break;
                case TypeID.P_ME_NB_1: // 111
                    value = value * kconv1 + kconv2;
                    if (value > 32767)
                        value = 32767;
                    else
                    if (value < -32768)
                        value = -32768;
                    sc =
                        new ParameterScaledValue(addr,
                            new ScaledValue(System.Convert.ToInt16(value)),
                            System.Convert.ToByte(cmdqualif));
                    break;
                case TypeID.P_ME_NC_1: // 112
                    value = value * kconv1 + kconv2;
                    sc =
                        new ParameterFloatValue(addr,
                            System.Convert.ToSingle(value),
                            System.Convert.ToByte(cmdqualif));
                    break;
                case TypeID.P_AC_NA_1: // 113                
                    sc =
                        new ParameterActivation(System.Convert.ToInt32(addr),
                            System.Convert.ToByte(cmdqualif));
                    break;
                default:
                    break;
            }
            return sc;
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