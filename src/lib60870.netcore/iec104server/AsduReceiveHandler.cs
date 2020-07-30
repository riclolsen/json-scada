using System;
using lib60870.CS101;
using MongoDB.Driver;

namespace Iec10XDriver
{
    partial class MainClass
    {
        public static Int32 LastPointKeySelectedOk = 0;
        // Receive commands from client and insert in Mongo if appropriate.
        private static bool
        AsduReceivedHandler(object parameter, IMasterConnection connection, ASDU asdu)
        {
            var srv = IEC10Xconns[(int)parameter];
            var conNameStr = srv.name + " - ";
            Log(conNameStr + asdu.ToString(), LogLevelDetailed);

            try
            {
                if (IsMongoLive)
                {
                    var DB = Client.GetDatabase(JSConfig.mongoDatabaseName);
                    var collection_rtd =
                        DB
                            .GetCollection
                            <rtData>(RealtimeDataCollectionName);
                    Double val = 0;
                    Int32 objaddr = 0;
                    Boolean sbo = false;
                    Int32 dur = 0;
                    Boolean isselect = false;
                    Boolean cmdhastime = false;
                    DateTime cmdtime = new DateTime();

                    Double dstkconv1 = 1;
                    Double dstkconv2 = 0;
                    Boolean dstsbo = false;
                    Int32 dstdur = 0;

                    Double srcval = 0;
                    Int32 srcobjaddr = 0;
                    Int32 srcconn = 0;
                    Boolean srcsbo = false;
                    Int32 srcdur = 0;
                    Int32 srcasdu = 0;
                    Int32 srcca = 0;
                    Int32 srcpointkey = 0;
                    String srctag = "";
                    Double srckconv1 = 1;
                    Double srckconv2 = 0;

                    switch (asdu.TypeId)
                    {
                        case TypeID.C_SC_NA_1: // 45
                            {
                                var cmd = (SingleCommand)asdu.GetElement(0);
                                isselect = cmd.Select;
                                dur = cmd.QU;
                                objaddr = cmd.ObjectAddress;
                                val = System.Convert.ToDouble(cmd.State);
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_SC_TA_1: // 58
                            {
                                var cmd = (SingleCommandWithCP56Time2a)asdu.GetElement(0);
                                isselect = cmd.Select;
                                dur = cmd.QU;
                                objaddr = cmd.ObjectAddress;
                                val = System.Convert.ToDouble(cmd.State);
                                cmdtime = cmd.Timestamp.GetDateTime();
                                cmdhastime = true;
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_DC_NA_1: // 46
                            {
                                var cmd = (DoubleCommand)asdu.GetElement(0);
                                isselect = cmd.Select;
                                dur = cmd.QU;
                                objaddr = cmd.ObjectAddress;
                                if (cmd.State != DoubleCommand.ON && cmd.State != DoubleCommand.OFF)
                                {
                                    connection.SendACT_CON(asdu, true); // activation confirm negative
                                    Log(conNameStr + "  Invalid double state command " + cmd.State, LogLevelBasic);
                                    LastPointKeySelectedOk = 0;
                                    return true;
                                }
                                val = cmd.State == DoubleCommand.ON ? 1 : 0;
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_DC_TA_1: // 59
                            {
                                var cmd = (DoubleCommandWithCP56Time2a)asdu.GetElement(0);
                                isselect = cmd.Select;
                                dur = cmd.QU;
                                objaddr = cmd.ObjectAddress;
                                if (cmd.State != DoubleCommand.ON && cmd.State != DoubleCommand.OFF)
                                {
                                    connection.SendACT_CON(asdu, true); // activation confirm negative
                                    Log(conNameStr + "  Invalid double state command " + cmd.State, LogLevelBasic);
                                    LastPointKeySelectedOk = 0;
                                    return true;
                                }
                                cmdtime = cmd.Timestamp.GetDateTime();
                                cmdhastime = true;
                                val = cmd.State == DoubleCommand.ON ? 1 : 0;
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_RC_NA_1: // 47
                            {
                                var cmd = (StepCommand)asdu.GetElement(0);
                                isselect = cmd.Select;
                                dur = cmd.QU;
                                objaddr = cmd.ObjectAddress;
                                if (cmd.State != StepCommandValue.HIGHER && cmd.State != StepCommandValue.LOWER)
                                {
                                    connection.SendACT_CON(asdu, true); // activation confirm negative
                                    Log(conNameStr + "  Invalid step state command " + cmd.State, LogLevelBasic);
                                    LastPointKeySelectedOk = 0;
                                    return true;
                                }
                                val = cmd.State == StepCommandValue.HIGHER ? 1 : 0;
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_RC_TA_1: // 60    
                            {
                                var cmd = (StepCommandWithCP56Time2a)asdu.GetElement(0);
                                isselect = cmd.Select;
                                dur = cmd.QU;
                                objaddr = cmd.ObjectAddress;
                                if (cmd.State != StepCommandValue.HIGHER && cmd.State != StepCommandValue.LOWER)
                                {
                                    connection.SendACT_CON(asdu, true); // activation confirm negative
                                    Log(conNameStr + "  Invalid step state command " + cmd.State, LogLevelBasic);
                                    LastPointKeySelectedOk = 0;
                                    return true;
                                }
                                cmdtime = cmd.Timestamp.GetDateTime();
                                cmdhastime = true;
                                val = cmd.State == StepCommandValue.HIGHER ? 1 : 0;
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_SE_NA_1: // 48
                            {
                                var cmd = (SetpointCommandNormalized)asdu.GetElement(0);
                                isselect = cmd.QOS.Select;
                                dur = cmd.QOS.QL;
                                objaddr = cmd.ObjectAddress;
                                val = cmd.NormalizedValue;
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_SE_TA_1: // 61
                            {
                                var cmd = (SetpointCommandNormalizedWithCP56Time2a)asdu.GetElement(0);
                                isselect = cmd.QOS.Select;
                                dur = cmd.QOS.QL;
                                objaddr = cmd.ObjectAddress;
                                cmdtime = cmd.Timestamp.GetDateTime();
                                cmdhastime = true;
                                val = cmd.NormalizedValue;
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_SE_NB_1: // 49
                            {
                                var cmd = (SetpointCommandScaled)asdu.GetElement(0);
                                isselect = cmd.QOS.Select;
                                dur = cmd.QOS.QL;
                                objaddr = cmd.ObjectAddress;
                                val = System.Convert.ToDouble(cmd.ScaledValue);
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_SE_TB_1: // 62
                            {
                                var cmd = (SetpointCommandScaledWithCP56Time2a)asdu.GetElement(0);
                                isselect = cmd.QOS.Select;
                                dur = cmd.QOS.QL;
                                objaddr = cmd.ObjectAddress;
                                cmdtime = cmd.Timestamp.GetDateTime();
                                cmdhastime = true;
                                val = System.Convert.ToDouble(cmd.ScaledValue);
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_SE_NC_1: // 50
                            {
                                var cmd = (SetpointCommandShort)asdu.GetElement(0);
                                isselect = cmd.QOS.Select;
                                dur = cmd.QOS.QL;
                                objaddr = cmd.ObjectAddress;
                                val = System.Convert.ToDouble(cmd.Value);
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_SE_TC_1: // 63
                            {
                                var cmd = (SetpointCommandShortWithCP56Time2a)asdu.GetElement(0);
                                isselect = cmd.QOS.Select;
                                dur = cmd.QOS.QL;
                                objaddr = cmd.ObjectAddress;
                                cmdtime = cmd.Timestamp.GetDateTime();
                                cmdhastime = true;
                                val = System.Convert.ToDouble(cmd.Value);
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_BO_NA_1: // 51
                            {
                                var cmd = (Bitstring32Command)asdu.GetElement(0);
                                isselect = false;
                                dur = 0;
                                objaddr = cmd.ObjectAddress;
                                val = System.Convert.ToDouble(cmd.Value);
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_BO_TA_1: // 64
                            {
                                var cmd = (Bitstring32CommandWithCP56Time2a)asdu.GetElement(0);
                                isselect = false;
                                dur = 0;
                                objaddr = cmd.ObjectAddress;
                                cmdtime = cmd.Timestamp.GetDateTime();
                                cmdhastime = true;
                                val = System.Convert.ToDouble(cmd.Value);
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.P_ME_NA_1: // 110
                            {
                                var cmd = (ParameterNormalizedValue)asdu.GetElement(0);
                                isselect = false;
                                dur = cmd.QPM;
                                objaddr = cmd.ObjectAddress;
                                val = System.Convert.ToDouble(cmd.NormalizedValue);
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.P_ME_NB_1: // 111
                            {
                                var cmd = (ParameterScaledValue)asdu.GetElement(0);
                                isselect = false;
                                dur = cmd.QPM;
                                objaddr = cmd.ObjectAddress;
                                val = System.Convert.ToDouble(cmd.ScaledValue);
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.P_ME_NC_1: // 112
                            {
                                var cmd = (ParameterFloatValue)asdu.GetElement(0);
                                isselect = false;
                                dur = cmd.QPM;
                                objaddr = cmd.ObjectAddress;
                                val = System.Convert.ToDouble(cmd.Value);
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.P_AC_NA_1: // 113                
                            {
                                var cmd = (ParameterActivation)asdu.GetElement(0);
                                isselect = false;
                                dur = cmd.QPA;
                                objaddr = cmd.ObjectAddress;
                                val = 0;
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_RP_NA_1: // 105 reset process
                            {
                                var cmd = (ResetProcessCommand)asdu.GetElement(0);
                                isselect = false;
                                dur = cmd.QRP;
                                objaddr = cmd.ObjectAddress;
                                val = 0;
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        // case TypeID.C_IC_NA_1: // 100 already managed by interrogation handler function
                        case TypeID.C_TS_NA_1: // 104 test command
                            {
                                Log(conNameStr + "  Test command C_TS_NA_1", LogLevelDetailed);
                                connection.SendACT_CON(asdu, false); // activation confirm positive
                            }
                            return true;
                        case TypeID.C_TS_TA_1: // 107 test command
                            {
                                Log(conNameStr + "  Test command C_TS_TA_1", LogLevelDetailed);
                                connection.SendACT_CON(asdu, false); // activation confirm positive
                            }
                            return true;
                        case TypeID.C_CI_NA_1: // 101
                            {
                                var cmd = (CounterInterrogationCommand)asdu.GetElement(0);
                                isselect = false;
                                dur = cmd.QCC;
                                objaddr = cmd.ObjectAddress; // should be zero
                                val = 0;
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_RD_NA_1: // 102
                            {
                                var cmd = (ReadCommand)asdu.GetElement(0);
                                objaddr = cmd.ObjectAddress;
                                Log(conNameStr + "  " + cmd.ToString() + " Obj Address " + objaddr, LogLevelBasic);
                            }
                            break;
                        case TypeID.C_CS_NA_1: // 103 clock sync
                            {
                                ClockSynchronizationCommand qsc =
                                    (ClockSynchronizationCommand)asdu.GetElement(0);
                                connection.SendACT_CON(asdu, false); // activation confirm positive
                                Log(conNameStr + "  Received clock sync command with time " +
                                    qsc.NewTime.ToString(), LogLevelBasic);
                                LastPointKeySelectedOk = 0;
                                return true;
                            }
                        default:
                            connection.SendACT_CON(asdu, true);
                            Log(conNameStr + "  Not implemented type of ASDU received: " + asdu.TypeId, LogLevelBasic);
                            LastPointKeySelectedOk = 0;
                            return true;
                    }

                    var filter1 = Builders<rtData>.Filter.Eq("protocolDestinations.protocolDestinationConnectionNumber", srv.protocolConnectionNumber);
                    var filter2 = Builders<rtData>.Filter.Eq("protocolDestinations.protocolDestinationCommonAddress", asdu.Ca);
                    var filter3 = Builders<rtData>.Filter.Eq("protocolDestinations.protocolDestinationObjectAddress", objaddr);
                    var filter4 = Builders<rtData>.Filter.Eq("protocolDestinations.protocolDestinationASDU", asdu.TypeId);
                    var filter = Builders<rtData>
                                    .Filter
                                    .And(filter1, filter2, filter3, filter4);

                    if (asdu.TypeId == TypeID.C_RD_NA_1)
                    { // READ COMMAND, look for object by object address of any type to send it
                        filter = Builders<rtData>
                                        .Filter
                                        .And(filter1, filter2, filter3);
                    }

                    var list =
                        collection_rtd.Find(filter).ToList();
                    if (list.Count > 0)
                    {
                        Log(conNameStr + "  Command found.", LogLevelBasic);
                        foreach (var dst in list[0].protocolDestinations)
                        {
                            if (dst.protocolDestinationConnectionNumber == srv.protocolConnectionNumber)
                            {
                                dstkconv1 = dst.protocolDestinationKConv1.ToDouble();
                                dstkconv2 = dst.protocolDestinationKConv2.ToDouble();
                                dstsbo = dst.protocolDestinationCommandUseSBO.ToBoolean();
                                dstdur = dst.protocolDestinationCommandDuration.ToInt32();

                                if (asdu.TypeId == TypeID.C_RD_NA_1)
                                { // READ REQUEST
                                    connection.SendACT_CON(asdu, false); // activation confirm positive
                                    ApplicationLayerParameters cp =
                                        srv.server.GetApplicationLayerParameters();
                                    var newAsdu = new ASDU(cp,
                                                       CauseOfTransmission.REQUEST,
                                                       false,
                                                       false,
                                                       System.Convert.ToByte(dst.protocolDestinationCommonAddress),
                                                       dst.protocolDestinationCommonAddress.ToInt32(),
                                                       true);
                                    var quality = new QualityDescriptor();
                                    quality.Invalid = list[0].invalid.ToBoolean() ||
                                                      list[0].overflow.ToBoolean() ||
                                                      list[0].transient.ToBoolean();
                                    quality.Substituted = list[0].substituted.ToBoolean();
                                    quality.Blocked = false;
                                    quality.NonTopical = false;
                                    InformationObject io = BuildInfoObj(
                                        System.Convert.ToInt32(dst.protocolDestinationASDU),
                                        System.Convert.ToInt32(dst.protocolDestinationObjectAddress),
                                        list[0].value.ToDouble(),
                                        false,
                                        0,
                                        quality
                                        );
                                    if (io != null)
                                    {
                                        newAsdu.AddInformationObject(io);
                                        srv.server.EnqueueASDU(newAsdu);
                                    }
                                    return true;
                                }

                                if (isselect && !dstsbo)
                                {  // tried a select when there is no select
                                    connection.SendACT_CON(asdu, true); // activation confirm negative
                                    Log(conNameStr + "  Select tried but not expected!", LogLevelBasic);
                                    LastPointKeySelectedOk = 0;
                                    return true;
                                }

                                if (dur != dstdur)
                                {  // duration spec different than expected, reject commmand
                                    connection.SendACT_CON(asdu, true); // activation confirm negative
                                    Log(conNameStr + "  QU/QL command qualifier not expected: " + dur + ", " + dstdur + " wanted ", LogLevelBasic);
                                    LastPointKeySelectedOk = 0;
                                    return true;
                                }

                                srcconn = list[0].protocolSourceConnectionNumber.ToInt32();
                                srcdur = list[0].protocolSourceCommandDuration.ToInt32();
                                srcobjaddr = list[0].protocolSourceObjectAddress.ToInt32();
                                srcasdu = list[0].protocolSourceASDU.ToInt32();
                                srcca = list[0].protocolSourceCommonAddress.ToInt32();
                                srckconv1 = list[0].kconv1.ToDouble();
                                srckconv2 = list[0].kconv2.ToDouble();
                                srcpointkey = list[0]._id.ToInt32();
                                srctag = list[0].tag.ToString();
                                break;
                            }
                        }
                    }
                    else
                    {
                        connection.SendACT_CON(asdu, true); // activation confirm negative
                        if (asdu.TypeId == TypeID.C_RD_NA_1)
                            Log(conNameStr + "  Request to read object not found, address: " + objaddr, LogLevelBasic);
                        else
                            Log(conNameStr + "  Command not found!", LogLevelBasic);
                        LastPointKeySelectedOk = 0;
                        return true;
                    }

                    if (srcasdu == 0)
                    {
                        Log(conNameStr + "  Command rejected!", LogLevelBasic);
                        connection.SendACT_CON(asdu, true); // activation confirm negative
                        LastPointKeySelectedOk = 0;
                        return true;
                    }

                    if (cmdhastime) // check command time
                        if (DateTime.Now.Subtract(cmdtime).TotalSeconds > timeToExpireCommandsWithTime)
                        { // expired 
                            Log(conNameStr + "  Command with time expired after " +
                                timeToExpireCommandsWithTime + "s, diff: " +
                                (DateTime.Now.Subtract(cmdtime).TotalSeconds - timeToExpireCommandsWithTime) + "s",
                                LogLevelBasic);
                            connection.SendACT_CON(asdu, true); // activation confirm negative
                            LastPointKeySelectedOk = 0;
                            return true;
                        }

                    connection.SendACT_CON(asdu, false); // activation confirm positive

                    if (isselect)
                    {
                        LastPointKeySelectedOk = srcpointkey; // flag selected point
                        Log(conNameStr + "  Select!", LogLevelBasic);
                        return true; // do not forward a select
                    }

                    if (!isselect && dstsbo && LastPointKeySelectedOk != srcpointkey)
                    {  // tried execute without select first when there is select expected
                        connection.SendACT_CON(asdu, true);
                        Log(conNameStr + "  Tried execute without select first!", LogLevelBasic);
                        LastPointKeySelectedOk = 0;
                        return true;
                    }
                    LastPointKeySelectedOk = 0;
                    Log(conNameStr + "  Execute (forward to queue)!", LogLevelBasic);

                    switch (asdu.TypeId)
                    {
                        case TypeID.C_SC_NA_1: // 45
                        case TypeID.C_SC_TA_1: // 58
                        case TypeID.C_DC_NA_1: // 46
                        case TypeID.C_DC_TA_1: // 59
                        case TypeID.C_RC_NA_1: // 47
                        case TypeID.C_RC_TA_1: // 60    
                            if (dstkconv1 == -1) // invert digital for kconv1 -1
                                val = val == 0 ? 1 : 0;
                            break;
                        case TypeID.C_SE_NA_1: // 48
                        case TypeID.C_SE_TA_1: // 61
                        case TypeID.C_SE_NB_1: // 49
                        case TypeID.C_SE_TB_1: // 62
                        case TypeID.C_SE_NC_1: // 50
                        case TypeID.C_SE_TC_1: // 63
                        case TypeID.P_ME_NA_1: // 110
                        case TypeID.P_ME_NB_1: // 111
                        case TypeID.P_ME_NC_1: // 112
                        case TypeID.P_AC_NA_1: // 113                
                        case TypeID.C_RP_NA_1: // 105 
                            val = val * dstkconv1 + dstkconv2;
                            break;
                        case TypeID.C_BO_NA_1: // 51
                        case TypeID.C_BO_TA_1: // 64
                            if (dstkconv1 == -1) // invert digital bits for kconv1 -1
                                val = System.Convert.ToInt32(val);
                            break;
                        default:
                            break;
                    }

                    switch ((TypeID)srcasdu)
                    {
                        case TypeID.C_SC_NA_1: // 45
                        case TypeID.C_SC_TA_1: // 58
                        case TypeID.C_DC_NA_1: // 46
                        case TypeID.C_DC_TA_1: // 59
                        case TypeID.C_RC_NA_1: // 47
                        case TypeID.C_RC_TA_1: // 60    
                            if (srckconv1 == -1) // invert digital for kconv1 -1
                                srcval = val == 0 ? 1 : 0;
                            else
                                srcval = val;
                            break;
                        case TypeID.C_SE_NA_1: // 48
                        case TypeID.C_SE_TA_1: // 61
                        case TypeID.C_SE_NB_1: // 49
                        case TypeID.C_SE_TB_1: // 62
                        case TypeID.C_SE_NC_1: // 50
                        case TypeID.C_SE_TC_1: // 63
                        case TypeID.P_ME_NA_1: // 110
                        case TypeID.P_ME_NB_1: // 111
                        case TypeID.P_ME_NC_1: // 112
                        case TypeID.P_AC_NA_1: // 113                
                        case TypeID.C_RP_NA_1: // 105 
                            srcval = val * srckconv1 + srckconv2;
                            break;
                        case TypeID.C_BO_NA_1: // 51
                        case TypeID.C_BO_TA_1: // 64
                            if (srckconv1 == -1) // invert digital bits for kconv1 -1
                                srcval = ~System.Convert.ToInt32(val);
                            else
                                srcval = System.Convert.ToInt32(val);
                            break;
                        default:
                            break;
                    }

                    // not sure how to detect the client connection as can be more than one
                    String orgip = "";
                    foreach (var conn in srv.clientConnections)
                        orgip = orgip + srv.clientConnections[0].RemoteEndpoint.ToString() + " ";

                    var collection_cmd =
                        DB
                            .GetCollection
                            <rtCommandNoAck>(CommandsQueueCollectionName);
                    var rtcmd = new rtCommandNoAck
                    {
                        protocolSourceConnectionNumber = srcconn,
                        protocolSourceCommonAddress = srcca,
                        protocolSourceObjectAddress = srcobjaddr,
                        protocolSourceASDU = srcasdu,
                        protocolSourceCommandDuration = srcdur,
                        protocolSourceCommandUseSBO = srcsbo,
                        pointKey = srcpointkey,
                        tag = srctag,
                        value = srcval,
                        valueString = srcval.ToString(),
                        originatorUserName = "Protocol connection: " + srv.name,
                        originatorIpAddress = orgip,
                        timeTag = DateTime.Now
                    };
                    collection_cmd.InsertOne(rtcmd);
                }
            }
            catch (Exception e)
            {
                Log("  Exception Mongo");
                Log(e);
                Log(e
                    .ToString()
                    .Substring(0,
                    e.ToString().IndexOf(Environment.NewLine)));
            }

            return true;
        }
    }
}