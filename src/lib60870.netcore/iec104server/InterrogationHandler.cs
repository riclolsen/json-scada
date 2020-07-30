using System;
using System.Linq;
using MongoDB.Driver;
using lib60870.CS101;

namespace Iec10XDriver
{
    partial class MainClass
    {
        // Process general and group interrogation from clients 
        private static bool
       InterrogationHandler(
           object parameter,
           IMasterConnection connection,
           ASDU asdu,
           byte qoi
       )
        {
            var srv = IEC10Xconns[(int)parameter];
            var conNameStr = srv.name + " - ";
            Log(conNameStr + "[" + qoi + "] Group interrogation BEGIN", LogLevelBasic);

            try
            {
                var Client = new MongoClient(JSConfig.mongoConnectionString);
                var DB = Client.GetDatabase(JSConfig.mongoDatabaseName);
                var collection =
                    DB.GetCollection<rtData>(RealtimeDataCollectionName);

                ApplicationLayerParameters cp =
                    connection.GetApplicationLayerParameters();

                // query mongodb for all data to distribute in this connection

                // for group 20 (general interogation) request filter by all in destination connection but those marked with group -1
                // for other groups requests filter by by those marked with group this same group
                var filter_conn = Builders<rtData>.Filter.Eq("protocolDestinations.protocolDestinationConnectionNumber", srv.protocolConnectionNumber);
                var filter_cmd = Builders<rtData>.Filter.Ne("origin", "command");
                var filter_gen_int = Builders<rtData>.Filter.And(filter_conn, filter_cmd, Builders<rtData>.Filter.Ne("protocolDestinations.protocolDestinationGroup", -1));
                var filter_oth_grp = Builders<rtData>.Filter.And(filter_conn, filter_cmd,
                         Builders<rtData>.Filter.Or(
                            Builders<rtData>.Filter.Eq("protocolDestinations.protocolDestinationGroup", qoi), // accept 20-36 or 0-16
                            Builders<rtData>.Filter.Eq("protocolDestinations.protocolDestinationGroup", qoi - 20)
                            )
                         );
                var sort = Builders<rtData>.Sort.Descending("$natural");
                var options = new FindOptions<rtData, rtData>();
                options.Sort = sort;
                var filter = filter_gen_int;
                if (qoi != 20)
                    filter = filter_oth_grp;
                var list =
                    collection.Find(filter).ToList();
                int CompareASDUInDest(rtData x, rtData y)
                {
                    var asdu1 = x.protocolDestinations[0].protocolDestinationASDU;
                    foreach (var dst in x.protocolDestinations)
                    {
                        if (dst.protocolDestinationConnectionNumber == srv.protocolConnectionNumber)
                        {
                            asdu1 = dst.protocolDestinationASDU;
                        }
                    }
                    var asdu2 = y.protocolDestinations[0].protocolDestinationASDU;
                    foreach (var dst in y.protocolDestinations)
                    {
                        if (dst.protocolDestinationConnectionNumber == srv.protocolConnectionNumber)
                        {
                            asdu2 = dst.protocolDestinationASDU;
                        }
                    }
                    if (asdu1 == asdu2)
                        return 0;
                    if (asdu1 > asdu2)
                        return 1;
                    return -1;
                }

                list.Sort(CompareASDUInDest); // order by ASDU

                Log(conNameStr + "[" + qoi + "] Group request, " + list.Count() + " objects to send.", LogLevelBasic);

                connection.SendACT_CON(asdu, false); // confirm positive
                var lastasdu = -1;
                var cntasduobj = 0;
                ASDU nwasdu = null;
                foreach (rtData entry in list)
                {
                    Log(conNameStr + "[" + qoi + "] " + entry.tag.ToString() + " " + entry.value + " Key " + entry._id, LogLevelDetailed);
                    foreach (var dst in entry.protocolDestinations)
                    {
                        var q = new QualityDescriptor();
                        q.Invalid = entry.invalid.ToBoolean();
                        q.Substituted = entry.substituted.ToBoolean();
                        q.NonTopical = false;
                        q.Blocked = false;
                        q.Overflow = entry.overflow.ToBoolean();
                        if (dst.protocolDestinationConnectionNumber == srv.protocolConnectionNumber)
                        {
                            if ((lastasdu != dst.protocolDestinationASDU && cntasduobj > 0) || cntasduobj >= 30)
                            {
                                if (nwasdu != null)
                                {
                                    Log(conNameStr + "[" + qoi + "] Send ASDU TI:" + System.Convert.ToByte(nwasdu.TypeId) + "." + nwasdu.TypeId + " CA:" + nwasdu.Ca +
                                        " with " + nwasdu.NumberOfElements + " objects.");
                                    connection.SendASDU(nwasdu);
                                    nwasdu = null;
                                    cntasduobj = 0;
                                    lastasdu = -1;
                                }
                            }
                            switch (dst.protocolDestinationASDU.ToInt32())
                            {
                                case 1:
                                case 30:
                                    if (cntasduobj == 0)
                                    {
                                        nwasdu =
                                            new ASDU(cp,
                                                (CauseOfTransmission)qoi,
                                                false,
                                                false,
                                                System.Convert.ToByte(cp.OA),
                                                System.Convert.ToInt32(dst.protocolDestinationCommonAddress.ToDouble()),
                                                false);
                                    }
                                    if (nwasdu != null)
                                    {
                                        var bval = entry.value != 0 ? true : false;
                                        if (dst.protocolDestinationKConv1 == -1)
                                            bval = !bval;
                                        nwasdu
                                            .AddInformationObject(new SinglePointInformation(System.Convert.ToInt32(dst.protocolDestinationObjectAddress.ToDouble()),
                                                bval,
                                                q));
                                        cntasduobj++;
                                    }
                                    break;
                                case 3:
                                case 31:
                                    if (cntasduobj == 0)
                                    {
                                        nwasdu =
                                            new ASDU(cp,
                                                (CauseOfTransmission)qoi,
                                                false,
                                                false,
                                                System.Convert.ToByte(cp.OA),
                                                System.Convert.ToInt32(dst.protocolDestinationCommonAddress.ToDouble()),
                                                false);
                                    }
                                    if (nwasdu != null)
                                    {
                                        var dpval = entry.value != 0 ? DoublePointValue.ON : DoublePointValue.OFF;
                                        if (dst.protocolDestinationKConv1 == -1)
                                            dpval = entry.value != 0 ? DoublePointValue.OFF : DoublePointValue.ON;
                                        if (entry.transient.ToBoolean())
                                            dpval = DoublePointValue.INTERMEDIATE;
                                        nwasdu
                                            .AddInformationObject(new DoublePointInformation(System.Convert.ToInt32(dst.protocolDestinationObjectAddress.ToDouble()),
                                                dpval,
                                                q));
                                        cntasduobj++;
                                    }
                                    break;
                                case 5:
                                case 32:
                                    if (cntasduobj == 0)
                                    {
                                        nwasdu =
                                            new ASDU(cp,
                                                (CauseOfTransmission)qoi,
                                                false,
                                                false,
                                                System.Convert.ToByte(cp.OA),
                                                System.Convert.ToInt32(dst.protocolDestinationCommonAddress.ToDouble()),
                                                false);
                                    }
                                    if (nwasdu != null)
                                    {
                                        var val = dst.protocolDestinationKConv1.ToDouble() * System.Convert.ToDouble(entry.value) + dst.protocolDestinationKConv2.ToDouble();
                                        if (val > 63)
                                        {
                                            val = 63;
                                            q.Overflow = true;
                                        }
                                        else
                                        if (val < -64)
                                        {
                                            val = -64;
                                            q.Overflow = true;
                                        }
                                        nwasdu
                                            .AddInformationObject(new StepPositionInformation(System.Convert.ToInt32(dst.protocolDestinationObjectAddress.ToDouble()),
                                                System.Convert.ToInt16(val),
                                                entry.transient.ToBoolean(),
                                                q));
                                        cntasduobj++;
                                    }
                                    break;
                                case 9:
                                case 34:
                                    if (cntasduobj == 0)
                                    {
                                        nwasdu =
                                            new ASDU(cp,
                                                (CauseOfTransmission)qoi,
                                                false,
                                                false,
                                                System.Convert.ToByte(cp.OA),
                                                System.Convert.ToInt32(dst.protocolDestinationCommonAddress.ToDouble()),
                                                false);
                                    }
                                    if (nwasdu != null)
                                    {
                                        var val = dst.protocolDestinationKConv1.ToDouble() * System.Convert.ToDouble(entry.value) + dst.protocolDestinationKConv2.ToDouble();
                                        if (val > 32767)
                                        {
                                            val = 32767;
                                            q.Overflow = true;
                                        }
                                        else
                                        if (val < -32768)
                                        {
                                            val = -32768;
                                            q.Overflow = true;
                                        }
                                        nwasdu
                                            .AddInformationObject(new MeasuredValueNormalized(System.Convert.ToInt32(dst.protocolDestinationObjectAddress.ToDouble()),
                                                System.Convert.ToInt16(val),
                                                new QualityDescriptor()));
                                        cntasduobj++;
                                    }
                                    break;
                                case 11:
                                case 35:
                                    if (cntasduobj == 0)
                                    {
                                        nwasdu =
                                            new ASDU(cp,
                                                (CauseOfTransmission)qoi,
                                                false,
                                                false,
                                                System.Convert.ToByte(cp.OA),
                                                System.Convert.ToInt32(dst.protocolDestinationCommonAddress.ToDouble()),
                                                false);
                                    }
                                    if (nwasdu != null)
                                    {
                                        var val = dst.protocolDestinationKConv1.ToDouble() * System.Convert.ToDouble(entry.value) + dst.protocolDestinationKConv2.ToDouble();
                                        if (val > 32767)
                                        {
                                            val = 32767;
                                            q.Overflow = true;
                                        }
                                        else
                                        if (val < -32768)
                                        {
                                            val = -32768;
                                            q.Overflow = true;
                                        }
                                        nwasdu
                                            .AddInformationObject(new MeasuredValueScaled(System.Convert.ToInt32(dst.protocolDestinationObjectAddress.ToDouble()),
                                                System.Convert.ToInt16(val),
                                                new QualityDescriptor()));
                                        cntasduobj++;
                                    }
                                    break;
                                case 13:
                                case 36:
                                    if (cntasduobj == 0)
                                    {
                                        nwasdu =
                                            new ASDU(cp,
                                                (CauseOfTransmission)qoi,
                                                false,
                                                false,
                                                System.Convert.ToByte(cp.OA),
                                                System.Convert.ToInt32(dst.protocolDestinationCommonAddress.ToDouble()),
                                                false);
                                    }
                                    if (nwasdu != null)
                                    {
                                        var val = dst.protocolDestinationKConv1.ToDouble() * System.Convert.ToDouble(entry.value) + dst.protocolDestinationKConv2.ToDouble();
                                        nwasdu
                                            .AddInformationObject(new MeasuredValueShort(System.Convert.ToInt32(dst.protocolDestinationObjectAddress.ToDouble()),
                                                System.Convert.ToSingle(val),
                                                new QualityDescriptor()));
                                        cntasduobj++;
                                    }
                                    break;
                                default:
                                    break;
                            }
                            lastasdu = dst.protocolDestinationASDU.ToInt32();
                            break;
                        }
                    }
                }
                if (nwasdu != null)
                {
                    Log(conNameStr + "[" + qoi + "] Send ASDU TI:" + System.Convert.ToByte(nwasdu.TypeId) + "." + nwasdu.TypeId + " CA:" + nwasdu.Ca +
                       " with " + nwasdu.NumberOfElements + " objects.", LogLevelBasic);
                    connection.SendASDU(nwasdu);
                    nwasdu = null;
                }
                connection.SendACT_TERM(asdu);
                Log(conNameStr + "[" + qoi + "] Group interrogation END", LogLevelBasic);
            }
            catch (Exception e)
            {
                if (e.Message == "Connection not active")
                    return true;

                Log("Exception on Interrogation");
                Log(e);
                Log(e
                    .ToString()
                    .Substring(0, e.ToString().IndexOf(Environment.NewLine)));
                System.Threading.Thread.Sleep(3000);
                connection.SendACT_CON(asdu, true); // negative confirm
                return true;
            }

            return true;
        }
    }
}