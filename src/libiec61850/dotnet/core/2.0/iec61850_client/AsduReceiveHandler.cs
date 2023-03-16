/* 
 * This software implements a IEC61850 driver for JSON SCADA.
 * Copyright - 2020-2023 - Ricardo Lastra Olsen
 * 
 * Requires libiec61850 from MZ Automation.
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
using System.Threading;
using System.Globalization;
using System.Linq;
using IEC61850.Client;
using IEC61850.Common;

namespace IEC61850_Client
{
    partial class MainClass
    {
        public enum ExitCode : int
        {
            Ok = 0,
            ErrorCreateApplication = 0x11,
            ErrorDiscoverEndpoints = 0x12,
            ErrorCreateSession = 0x13,
            ErrorBrowseNamespace = 0x14,
            ErrorCreateSubscription = 0x15,
            ErrorMonitoredItem = 0x16,
            ErrorAddSubscription = 0x17,
            ErrorRunning = 0x18,
            ErrorNoKeepAlive = 0x30,
            ErrorInvalidCommandLine = 0x100
        };

        public class Iec61850Control
        {
            public string js_cmd_tag; // tag name on json scada 
            public double value; // command value
            public string fc; // command fc
            public DateTime timestamp; // timestamp
            public Iec61850Entry iecEntry; // iec61850 object entry
        }

        public class Iec61850Entry
        {
            public string path; // IEC61850 object path
            public FunctionalConstraint fc;
            public List<string> childs = new List<string>(); // list of child objects
            public string dataSetName = ""; // name of dataset that contains the object, if any
            public string rcbName = ""; // name of report that contains the object, if any
            public string js_tag = ""; // tag from json scada that will be updated
        }
        public class ReptParam
        {
            public Iec61850Connection srv;
            public ReportControlBlock rcb;
        }

        static Boolean MMSTestDoubleStateFailed(MmsValue mv)
        { // test for double state inconsistent (bitstring of 2 with same values)
            return (mv.GetType() == MmsType.MMS_BIT_STRING && mv.Size() == 2 && mv.GetBit(0) == mv.GetBit(1));
        }

        static Boolean MMSGetQualityFailed(MmsValue mv)
        { // tries to find a qualifier of iec61850 (bitstring) in a mms structure  
            Boolean f = false;
            Boolean found = false;
            switch (mv.GetType())
            {
                case MmsType.MMS_STRUCTURE:
                    for (int i = 0; i < mv.Size(); i++)
                        if (mv.GetElement(i).GetType() == MmsType.MMS_BIT_STRING)
                        {
                            f = !(mv.GetElement(i).BitStringToUInt32BigEndian() == 0);
                            found = true;
                            break;
                        }
                    if (!found)
                        f = MMSGetQualityFailed(mv.GetElement(0));
                    break;
                case MmsType.MMS_BIT_STRING:
                    if (MMSTestDoubleStateFailed(mv))
                        f = true;
                    else
                        f = !(mv.BitStringToUInt32BigEndian() == 0);
                    break;
            }
            return f;
        }

        static ulong MMSGetTimestamp(MmsValue mv)
        { // tries to find a timestamp of iec61850 (utc time) in a mms structure, return number of ms UTC
            ulong t = 0;
            Boolean found = false;
            switch (mv.GetType())
            {
                case MmsType.MMS_STRUCTURE:
                    for (int i = 0; i < mv.Size(); i++)
                        if (mv.GetElement(i).GetType() == MmsType.MMS_UTC_TIME)
                        {
                            t = mv.GetElement(i).GetUtcTimeInMs();
                            found = true;
                            break;
                        }
                    if (!found)
                        t = MMSGetTimestamp(mv.GetElement(0));
                    break;
                case MmsType.MMS_UTC_TIME:
                    t = mv.GetUtcTimeInMs();
                    break;
            }
            return t;
        }

        static Double MMSGetNumericVal(MmsValue mv, out Boolean isBinary)
        { // tries to find a numeric value of iec61850 (flot, integer, unsigned) in a mms structure  
            Double v = 0;
            Boolean found = false;
            isBinary = false;
            switch (mv.GetType())
            {
                case MmsType.MMS_STRUCTURE:
                    for (int i = 0; i < mv.Size(); i++)
                    {
                        switch (mv.GetElement(i).GetType())
                        {
                            case MmsType.MMS_FLOAT:
                                v = mv.GetElement(i).ToFloat();
                                found = true;
                                break;
                            case MmsType.MMS_INTEGER:
                                v = mv.GetElement(i).ToInt64();
                                found = true;
                                break;
                            case MmsType.MMS_UNSIGNED:
                                v = mv.GetElement(i).ToUint32();
                                found = true;
                                break;
                        }
                        if (found) break;
                    }
                    if (!found)
                        v = MMSGetNumericVal(mv.GetElement(0), out isBinary);
                    break;
                case MmsType.MMS_FLOAT:
                    v = mv.ToFloat();
                    break;
                case MmsType.MMS_INTEGER:
                    v = mv.ToInt64();
                    break;
                case MmsType.MMS_UNSIGNED:
                    v = mv.ToUint32();
                    break;
                case MmsType.MMS_BOOLEAN:
                    isBinary = true;
                    v = mv.GetBoolean() ? 1 : 0;
                    break;
                case MmsType.MMS_BIT_STRING:
                    if (mv.Size() == 2)
                    { // double state
                        isBinary = true;
                        switch (mv.ToString())
                        {
                            case "00":
                            case "01":
                                v = 0;
                                break;
                            case "11":
                            case "10":
                                v = 1;
                                break;
                        }
                    }
                    else
                        v = mv.BitStringToUInt32BigEndian();
                    break;
                default:
                    break;
            }
            return v;
        }

        static Double MMSGetDoubleVal(MmsValue mv, out Boolean isBinary)
        { // tries to convert any mms value into a double
            Double v = 0;
            isBinary = false;
            switch (mv.GetType())
            {
                case MmsType.MMS_STRUCTURE:
                    v = MMSGetNumericVal(mv, out isBinary);
                    break;
                case MmsType.MMS_BIT_STRING:
                    if (mv.Size() == 2)
                    { // double state
                        isBinary = true;
                        switch (mv.ToString())
                        {
                            case "00":
                            case "01":
                                v = 0;
                                break;
                            case "11":
                            case "10":
                                v = 1;
                                break;
                        }
                    }
                    else
                        v = mv.BitStringToUInt32BigEndian();
                    break;
                case MmsType.MMS_BOOLEAN:
                    isBinary = true;
                    v = mv.GetBoolean() ? 1 : 0;
                    break;
                case MmsType.MMS_OCTET_STRING:
                    v = mv.GetOctetStringOctet(0);
                    break;
                case MmsType.MMS_FLOAT:
                    v = mv.ToFloat();
                    break;
                case MmsType.MMS_INTEGER:
                    v = mv.ToInt64();
                    break;
                case MmsType.MMS_UNSIGNED:
                    v = mv.ToUint32();
                    break;
                case MmsType.MMS_UTC_TIME:
                    v = mv.GetUtcTimeInMs();
                    break;
                case MmsType.MMS_ARRAY:
                    v = MMSGetNumericVal(mv.GetElement(0), out isBinary);
                    break;
                case MmsType.MMS_BCD:
                case MmsType.MMS_VISIBLE_STRING:
                case MmsType.MMS_GENERALIZED_TIME:
                case MmsType.MMS_BINARY_TIME:
                case MmsType.MMS_OBJ_ID:
                case MmsType.MMS_STRING:
                case MmsType.MMS_DATA_ACCESS_ERROR:
                    break;
            }
            return v;
        }
        static string MMSGetStringValue(MmsValue mv)
        { // tries to convert any mms value into a double
            String v = "";
            switch (mv.GetType())
            {
                case MmsType.MMS_STRUCTURE:
                case MmsType.MMS_ARRAY:
                    if (mv.Size() == 1)
                    {
                        if (mv.GetElement(0).GetType() != MmsType.MMS_STRUCTURE && mv.GetElement(0).GetType() != MmsType.MMS_ARRAY)
                            v += "\"" + mv.GetElement(0).ToString() + "\"";
                        else
                            v += MMSGetStringValue(mv.GetElement(0));
                    }
                    else
                    {
                        v += "[";
                        for (int i = 0; i < mv.Size(); i++)
                        {
                            if (mv.GetElement(i).GetType() != MmsType.MMS_STRUCTURE && mv.GetElement(i).GetType() != MmsType.MMS_ARRAY)
                                v += "\"" + mv.GetElement(i).ToString() + "\",";
                            else
                                v += MMSGetStringValue(mv.GetElement(i)) + ",";
                        }
                        v += "]";
                    }
                    break;
                case MmsType.MMS_BIT_STRING:
                case MmsType.MMS_BOOLEAN:
                case MmsType.MMS_OCTET_STRING:
                case MmsType.MMS_FLOAT:
                case MmsType.MMS_INTEGER:
                case MmsType.MMS_UNSIGNED:
                case MmsType.MMS_UTC_TIME:
                case MmsType.MMS_BCD:
                case MmsType.MMS_VISIBLE_STRING:
                case MmsType.MMS_GENERALIZED_TIME:
                case MmsType.MMS_BINARY_TIME:
                case MmsType.MMS_OBJ_ID:
                case MmsType.MMS_STRING:
                case MmsType.MMS_DATA_ACCESS_ERROR:
                    v = "\"" + mv.ToString() + "\"";
                    break;
            }
            return v;
        }

        static string getRefFc(string dataRef, out FunctionalConstraint fc)
        {
            string ret = dataRef;
            fc = FunctionalConstraint.NONE;
            for (int i = 0; i < 17; i++)
            {
                var sfc = "$" + ((FunctionalConstraint)i).ToString() + "$";
                if (dataRef.Contains(sfc))
                {
                    fc = (FunctionalConstraint)i;
                    ret = dataRef.Replace(sfc, ".");
                    break;
                }
            }
            return ret.Replace("$", ".");
        }
        static string getRefFc2(string dataRef, out FunctionalConstraint fc)
        {
            string ret = dataRef;
            fc = FunctionalConstraint.NONE;
            for (int i = 0; i < 17; i++)
            {
                var sfc = "[" + ((FunctionalConstraint)i).ToString() + "]";
                if (dataRef.Contains(sfc))
                {
                    fc = (FunctionalConstraint)i;
                    ret = dataRef.Replace(sfc, "");
                    break;
                }
            }
            return ret;
        }
        private static void reportHandler(Report report, object parameter)
        { // handle reports, forward values when desired
            ReptParam rp = (ReptParam)parameter;

            string log = "";
            if (LogLevel > LogLevelNoLog)
            {
                log = rp.srv.name + " Report RCB: " + report.GetRcbReference();
                if (report.HasSeqNum())
                    log += " SeqNumb:" + report.GetSeqNum();
                if (report.HasSubSeqNum())
                    log += " SubSeqNumb:" + report.GetSubSeqNum();
                log += "\n";
            }
            byte[] entryId = report.GetEntryId().ToArray();
            if (entryId != null)
            {
                if (LogLevel > LogLevelNoLog)
                    log += "  entryID: " + BitConverter.ToString(entryId) + " \n";
                if (rp.srv.brcb.Contains(report.GetRcbReference()))
                {
                    if (rp.srv.lastReportIds.ContainsKey(report.GetRcbReference()))
                        if (BitConverter.ToString(rp.srv.lastReportIds[report.GetRcbReference()]) == BitConverter.ToString(entryId))
                        {
                            if (LogLevel > LogLevelNoLog)
                                log += "Repeated report!\n";
                            Log(log);
                            return;
                        }
                    rp.srv.lastReportIds[report.GetRcbReference()] = entryId.ToArray();
                    rp.srv.brcbCount++;
                }
            }
            if (LogLevel > LogLevelNoLog && report.HasDataSetName())
                log += "  data-set: " + rp.rcb.GetDataSetReference() + "\n";

            if (report.HasTimestamp() && LogLevel > LogLevelNoLog)
                log += "  timestamp: " + MmsValue.MsTimeToDateTimeOffset(report.GetTimestamp()).ToString() + "\n";

            MmsValue values = report.GetDataSetValues();

            if (LogLevel > LogLevelNoLog)
                log += "  report dataset contains " + values.Size() + " elements" + "\n";

            for (int k = 0; k < values.Size(); k++)
            {
                if (report.HasReasonForInclusion())
                    if (report.GetReasonForInclusion(k) != ReasonForInclusion.REASON_NOT_INCLUDED)
                    {
                        var dataRef = getRefFc(report.GetDataReference(k), out FunctionalConstraint fc);

                        if (!rp.srv.autoCreateTags && !rp.srv.entries.ContainsKey(dataRef + fc)) continue; // when no autoTag do not forward data for tags undefined
                        Iec61850Entry entry = new Iec61850Entry();
                        if (rp.srv.entries.ContainsKey(dataRef + fc))
                            entry = rp.srv.entries[dataRef + fc];
                        else
                        {  // autoTag undefined json scada tag with server name plus 61850 path
                            entry.path = dataRef;
                            entry.js_tag = rp.srv.name + ":" + dataRef;
                            entry.childs.Clear();
                            entry.fc = fc;
                        }
                        entry.rcbName = report.GetRcbReference();
                        entry.dataSetName = rp.rcb.GetDataSetReference();

                        log += "\nElement " + k + " , path " + entry.path + " [" + fc + "] , js_tag " + entry.js_tag + "\n";

                        if (LogLevel > LogLevelNoLog)
                            log += " Included for reason " + report.GetReasonForInclusion(k).ToString() + " \n";
                        string tag = entry.js_tag;
                        var value = values.GetElement(k);
                        double v;
                        bool failed;
                        ulong timestamp;
                        Boolean isBinary = false;

                        if (value.GetType() == MmsType.MMS_STRUCTURE)
                        {
                            if (LogLevel > LogLevelNoLog)
                            {
                                log += " Value is of complex type [";
                                foreach (var item in entry.childs)
                                {
                                    log += item + " ";
                                }
                                log += "]\n";
                            }
                            v = MMSGetNumericVal(value, out isBinary);
                            failed = MMSGetQualityFailed(value);
                            timestamp = MMSGetTimestamp(value);

                            for (int i = 0; i < value.Size(); i++)
                            {
                                if (LogLevel > LogLevelNoLog)
                                    log += "  " + value.GetElement(i).GetType();

                                if (value.GetElement(i).GetType() == MmsType.MMS_STRUCTURE)
                                {
                                    v = MMSGetNumericVal(value.GetElement(i), out isBinary);
                                    for (int j = 0; j < value.GetElement(i).Size(); j++)
                                    {
                                        if (LogLevel > LogLevelNoLog)
                                            log += "  " + value.GetElement(i).GetElement(j).GetType();
                                        if (LogLevel > LogLevelNoLog)
                                            log += "     -> " + value.GetElement(i).GetElement(j).ToString() + "\n";
                                        v = MMSGetNumericVal(value.GetElement(i).GetElement(j), out isBinary);
                                    }
                                }
                                failed = MMSGetQualityFailed(value.GetElement(i));
                                timestamp = MMSGetTimestamp(value.GetElement(i));
                                if (value.GetElement(i).GetType() == MmsType.MMS_BIT_STRING)
                                {
                                    if (LogLevel > LogLevelNoLog)
                                        log += "   -> " + value.GetElement(i).ToString() + "\n";
                                }
                                else
                                if (value.GetElement(i).GetType() == MmsType.MMS_UTC_TIME)
                                {
                                    if (LogLevel > LogLevelNoLog)
                                        log += "   -> " + value.GetElement(i).GetUtcTimeAsDateTimeOffset() + "\n";
                                }
                                else
                                {
                                    if (LogLevel > LogLevelNoLog)
                                        log += "   -> " + v + "\n";
                                }
                            }

                            string vstr;
                            if (isBinary)
                                vstr = v != 0 ? "true" : "false";
                            else
                                vstr = v.ToString("G", CultureInfo.CreateSpecificCulture("en-US"));

                            var iv = new IECValue
                            {
                                isDigital = isBinary,
                                value = v,
                                valueString = vstr,
                                valueJson = MMSGetStringValue(value),
                                serverTimestamp = DateTime.Now,
                                sourceTimestamp = DateTime.MinValue,
                                hasSourceTimestamp = false,
                                cot = 20,
                                common_address = entry.fc.ToString(),
                                address = entry.path,
                                asdu = value.GetType().ToString(),
                                quality = !failed,
                                selfPublish = true,
                                conn_name = rp.srv.name,
                                conn_number = rp.srv.protocolConnectionNumber,
                                display_name = entry.path,
                            };
                            if (report.GetReasonForInclusion(k) == ReasonForInclusion.REASON_DATA_CHANGE && timestamp != 0)
                            {
                                iv.hasSourceTimestamp = true;
                                iv.sourceTimestamp = DateTimeOffset.FromUnixTimeMilliseconds((long)timestamp).UtcDateTime;
                            }
                            IECDataQueue.Enqueue(iv);
                        }
                        else
                        {
                            v = MMSGetDoubleVal(value, out isBinary);
                            if (LogLevel > LogLevelNoLog)
                            {
                                log += " Value is of simple type " + value.GetType() + " " + v;
                            }
                            failed = false;
                            if (MMSTestDoubleStateFailed(value)) failed = true; // double state inconsistent status
                            string vstr;
                            if (isBinary)
                                vstr = v != 0 ? "true" : "false";
                            else
                                vstr = v.ToString("G", CultureInfo.CreateSpecificCulture("en-US"));

                            var iv = new IECValue
                            {
                                isDigital = isBinary,
                                value = v,
                                valueString = vstr,
                                valueJson = MMSGetStringValue(value),
                                serverTimestamp = DateTime.Now,
                                sourceTimestamp = DateTime.MinValue,
                                hasSourceTimestamp = false,
                                cot = 20,
                                common_address = entry.fc.ToString(),
                                address = entry.path,
                                asdu = value.GetType().ToString(),
                                quality = !failed,
                                selfPublish = true,
                                conn_name = rp.srv.name,
                                conn_number = rp.srv.protocolConnectionNumber,
                                display_name = entry.path,
                            };
                            IECDataQueue.Enqueue(iv);
                        }
                    }
            }
            Log(log);
        }

        static void Process(Iec61850Connection srv)
        { // handle a 61850 connection with a server (ied)
            CultureInfo ci = new CultureInfo("en-US");
            Thread.CurrentThread.CurrentCulture = ci;
            Thread.CurrentThread.CurrentUICulture = ci;
            int brcbCountPrev = 0;

            do
            {
                if (!Active)
                {
                    Thread.Sleep(1000);
                }
                else
                {
                    IedConnection con = new IedConnection();
                    try
                    {
                        if (srv.password != "")
                        {
                            IsoConnectionParameters parameters = con.GetConnectionParameters();
                            parameters.UsePasswordAuthentication(srv.password);
                        }

                        Log("Connect to " + srv.name);
                        var tcpPort = 102;
                        string[] ipAddrPort = srv.ipAddresses[0].Split(':');
                        if (ipAddrPort.Length > 1)
                            if (int.TryParse(ipAddrPort[1], out _))
                                tcpPort = System.Convert.ToInt32(ipAddrPort[1]);
                        con.Connect(ipAddrPort[0], tcpPort);
                        MmsConnection mmsCon = con.GetMmsConnection();
                        MmsServerIdentity identity = mmsCon.GetServerIdentity();
                        Log("Vendor:   " + identity.vendorName);
                        Log("Model:    " + identity.modelName);
                        Log("Revision: " + identity.revision);
                        srv.connection = con;

                        List<string> serverDirectory = con.GetServerDirectory(false);

                        foreach (string ldName in serverDirectory)
                        { // logical devices
                            Log(srv.name + " LD: " + ldName);
                            List<string> lnNames = con.GetLogicalDeviceDirectory(ldName);

                            foreach (string lnName in lnNames)
                            {
                                Log(srv.name + "  LN: " + lnName);
                                string logicalNodeReference = ldName + "/" + lnName;

                                // discover data objects
                                List<string> dataObjects =
                                    con.GetLogicalNodeDirectory(logicalNodeReference, ACSIClass.ACSI_CLASS_DATA_OBJECT);

                                if (srv.browse)
                                    foreach (string dataObject in dataObjects)
                                    {
                                        Log(srv.name + "    DO: " + dataObject);
                                        List<string> dataDirectory = con.GetDataDirectoryFC(logicalNodeReference + "." + dataObject);

                                        foreach (string dataDirectoryElement in dataDirectory)
                                        {
                                            string daReference = logicalNodeReference + "." + dataObject + "." + ObjectReference.getElementName(dataDirectoryElement);

                                            // get the type specification of a variable
                                            MmsVariableSpecification specification = con.GetVariableSpecification(daReference, ObjectReference.getFC(dataDirectoryElement));

                                            Log(srv.name + "      DA/SDO: [" + ObjectReference.getFC(dataDirectoryElement) + "] " +
                                                               ObjectReference.getElementName(dataDirectoryElement) + " : " + specification.GetType()
                                                               + "(" + specification.Size() + ") ... " + daReference);
                                            if (specification.GetType() == MmsType.MMS_STRUCTURE)
                                            {
                                                foreach (MmsVariableSpecification elementSpec in specification)
                                                {
                                                    Log(srv.name + "           " + elementSpec.GetName() + " : " + elementSpec.GetType() + " ... " + daReference + "." + elementSpec.GetName());
                                                }
                                            }
                                        }
                                    }

                                // discover data sets
                                var dataSets =
                                    con.GetLogicalNodeDirectory(logicalNodeReference, ACSIClass.ACSI_CLASS_DATA_SET);
                                srv.datasets.Clear();

                                foreach (var dataSet in dataSets)
                                {
                                    var dataSetName = logicalNodeReference + "." + dataSet;
                                    Log(srv.name + "    Dataset: " + dataSetName);
                                    srv.datasets.Add(dataSet);
                                    var listData = con.GetDataSetDirectory(dataSetName);
                                    // for each desired dataset entry find its child elements
                                    foreach (var dataName in listData)
                                    {
                                        Log(srv.name + "     " + dataSetName + " -> " + dataName);
                                        var dataRef = getRefFc2(dataName, out FunctionalConstraint fc);
                                        if (srv.entries.ContainsKey(dataRef + fc))
                                        {
                                            var entry = srv.entries[dataRef + fc];
                                            if (entry.fc != fc)
                                                continue;
                                            entry.dataSetName = dataSetName;
                                            Log(srv.name + "       Found desired entry " + entry.path);
                                            if (entry.childs.Count == 0)
                                            {
                                                try
                                                {
                                                    var sz = con.GetVariableSpecification(entry.path, entry.fc).Size();
                                                    for (int j = 0; j < sz; j++)
                                                    {
                                                        var cname = con.GetVariableSpecification(entry.path, entry.fc).GetElement(j).GetName();
                                                        Log(srv.name + "         Child " + cname);
                                                        entry.childs.Add(cname);
                                                    }

                                                }
                                                catch { }
                                            }
                                        }
                                    }

                                    /*
                                    var ds = con.ReadDataSetValues(dataSetName, null);
                                    var vals = ds.GetValues();
                                    foreach (var val in vals)
                                    {
                                        Log(val.ToString()); 
                                    }
                                    */
                                }

                                if (srv.useUrcb)
                                {
                                    // discover unbuffered report control blocks
                                    List<string> urcbs =
                                        con.GetLogicalNodeDirectory(logicalNodeReference, ACSIClass.ACSI_CLASS_URCB);
                                    srv.urcb.Clear();
                                    foreach (string urcb in urcbs)
                                    {
                                        var rpname = logicalNodeReference + ".RP." + urcb;
                                        Log(srv.name + " URCB: " + rpname);
                                        if (srv.topics.Length > 0 && !srv.topics.Contains(rpname))
                                        {
                                            Log(srv.name + " Report will not be activated! not in topics list.");
                                            continue;
                                        }
                                        srv.urcb.Add(rpname);
                                        var rcb = con.GetReportControlBlock(rpname);

                                        var dataSetName = rcb.GetDataSetReference();
                                        foreach (var entry in srv.entries)
                                        {
                                            if (entry.Value.dataSetName == dataSetName)
                                            {
                                                srv.entries[entry.Key].rcbName = rpname;
                                            }
                                        }

                                        if (rcb != null && rcb.GetObjectReference() != "")
                                        {
                                            try
                                            {
                                                rcb.GetRCBValues();
                                            }
                                            catch (IedConnectionException e)
                                            {
                                                Log(srv.name + " URCB: IED GetRCB excepion - " + e.Message);
                                            }

                                            rcb.InstallReportHandler(reportHandler, new ReptParam { srv = srv, rcb = rcb });
                                            rcb.SetTrgOps(TriggerOptions.DATA_UPDATE | TriggerOptions.DATA_CHANGED | TriggerOptions.INTEGRITY);
                                            rcb.SetIntgPd((uint)srv.class0ScanInterval * 1000);                                            
                                            rcb.SetRptEna(true);
                                            // rcb.SetResv(true);
                                            try
                                            {                                                
                                                rcb.SetRCBValues();
                                                rcb.SetGI(true);
                                            }
                                            catch (IedConnectionException e)
                                            {
                                                Log(srv.name + " URCB: IED SetRCB exception - " + e.Message + " Code:" + e.GetErrorCode());
                                            }
                                        }
                                    }
                                }

                                if (srv.useBrcb)
                                {
                                    // discover buffered report control blocks
                                    List<string> brcbs =
                                        con.GetLogicalNodeDirectory(logicalNodeReference, ACSIClass.ACSI_CLASS_BRCB);
                                    srv.brcb.Clear();
                                    foreach (string brcb in brcbs)
                                    {
                                        var rpname = logicalNodeReference + ".BR." + brcb;
                                        Log(srv.name + " BRCB: " + rpname);
                                        if (srv.topics.Length > 0 && !srv.topics.Contains(rpname))
                                        {
                                            Log(srv.name + " Report will not be activated! not in topics list.");
                                            continue;
                                        }
                                        srv.brcb.Add(rpname);
                                        var rcb = con.GetReportControlBlock(rpname);

                                        var dataSetName = rcb.GetDataSetReference();
                                        foreach (var entry in srv.entries)
                                        {
                                            if (entry.Value.dataSetName == dataSetName)
                                            {
                                                srv.entries[entry.Key].rcbName = rpname;
                                            }
                                        }

                                        if (rcb != null && rcb.GetObjectReference() != "")
                                        {
                                            try
                                            {
                                                rcb.GetRCBValues();
                                            }
                                            catch (IedConnectionException e)
                                            {
                                                Log(srv.name + " BRCB: IED GetRCB excepion - " + e.Message);
                                            }

                                            rcb.InstallReportHandler(reportHandler, new ReptParam { srv = srv, rcb = rcb });
                                            rcb.SetTrgOps(TriggerOptions.DATA_UPDATE | TriggerOptions.DATA_CHANGED | TriggerOptions.INTEGRITY);
                                            byte[] lastEntryId = { 0, 0, 0, 0, 0, 0, 0, 0 };
                                            if (srv.lastReportIds.ContainsKey(rpname))
                                            {
                                                lastEntryId = srv.lastReportIds[rpname];
                                                Log(srv.name + " BRCB: " + rpname + " - Last seen entryId: " + BitConverter.ToString(lastEntryId));
                                            }
                                            rcb.SetEntryID(lastEntryId);
                                            rcb.SetIntgPd((uint)srv.class0ScanInterval * 1000);
                                            rcb.SetRptEna(true);                                            
                                            try
                                            {                                                
                                                rcb.SetRCBValues();
                                                rcb.SetGI(true);
                                            }
                                            catch (IedConnectionException e)
                                            {
                                                Log(srv.name + " BRCB: IED SetRCB exception - " + e.Message + " Code:" + e.GetErrorCode());                                               
                                            }
                                        }
                                    }
                                }
                            }
                        }

                        do
                        {
                            for (int i = 0; i < srv.entries.Count; i++)
                            {
                                var entry = srv.entries.ElementAt(i).Value;
                                string tag;
                                if (entry.js_tag == "")
                                    tag = entry.path;
                                else
                                    tag = entry.js_tag;

                                if (entry.rcbName == "") // only read elements that are not in reports
                                {
                                    int err = 6;
                                    for (int j = 0; j < 5 && err == 6; j++)
                                    {
                                        Log(srv.name + " Async Reading " + entry.path + " " + entry.fc + " ind:" + (i + 1) + " try:" + (j + 1));
                                        err = 0;
                                        try
                                        {
                                            var ret = con.ReadValueAsync(entry.path, entry.fc,
                                               delegate (uint invokeId, object parameter, IedClientError err, MmsValue value)
                                               {
                                                   string log = "";
                                                   if (LogLevel > LogLevelNoLog)
                                                       log = srv.name + " READED " + " " + entry.path + " " + tag;
                                                   if (err == IedClientError.IED_ERROR_OK)
                                                   {
                                                       var tp = value.GetType();
                                                       double v = 0;
                                                       bool failed = false;
                                                       ulong timestamp = 0;
                                                       bool isBinary = false;

                                                       if (value.GetType() == MmsType.MMS_STRUCTURE)
                                                       {
                                                           failed = true;
                                                           if (LogLevel >= LogLevelDetailed) log += "\n    Value is of complex type \n";
                                                           v = MMSGetNumericVal(value, out isBinary);
                                                           failed = MMSGetQualityFailed(value);
                                                           timestamp = MMSGetTimestamp(value);

                                                           for (int i = 0; i < value.Size(); i++)
                                                           {
                                                               if (LogLevel > LogLevelDetailed) log += "    element: " + value.GetElement(i).GetType();

                                                               if (value.GetElement(i).GetType() == MmsType.MMS_STRUCTURE)
                                                               {
                                                                   v = MMSGetNumericVal(value.GetElement(i), out isBinary);
                                                                   for (int j = 0; j < value.GetElement(i).Size(); j++)
                                                                   {
                                                                       if (LogLevel >= LogLevelDetailed) log += "    element: " + value.GetElement(i).GetElement(j).GetType();
                                                                       if (LogLevel >= LogLevelDetailed) log += " -> " + value.GetElement(i).GetElement(j).ToString() + "\n";
                                                                       v = MMSGetNumericVal(value.GetElement(i).GetElement(j), out isBinary);
                                                                   }
                                                               }
                                                               failed = MMSGetQualityFailed(value.GetElement(i));
                                                               timestamp = MMSGetTimestamp(value.GetElement(i));
                                                               if (value.GetElement(i).GetType() == MmsType.MMS_BIT_STRING)
                                                               {
                                                                   if (LogLevel >= LogLevelDetailed) log += " -> " + value.GetElement(i).ToString() + "\n";
                                                               }
                                                               else
                                                               if (value.GetElement(i).GetType() == MmsType.MMS_UTC_TIME)
                                                               {
                                                                   if (LogLevel >= LogLevelDetailed) log += " -> " + value.GetElement(i).GetUtcTimeAsDateTimeOffset() + "\n";
                                                               }
                                                               else
                                                               {
                                                                   if (LogLevel > LogLevelNoLog)
                                                                       log += "   -> " + v + "\n";
                                                               }
                                                           }
                                                           string vstr;
                                                           if (isBinary)
                                                               vstr = v != 0 ? "true" : "false";
                                                           else
                                                               vstr = v.ToString("G", CultureInfo.CreateSpecificCulture("en-US"));

                                                           var iv = new IECValue
                                                           {
                                                               isDigital = isBinary,
                                                               value = v,
                                                               valueString = vstr,
                                                               valueJson = MMSGetStringValue(value),
                                                               serverTimestamp = DateTime.Now,
                                                               sourceTimestamp = DateTime.MinValue,
                                                               hasSourceTimestamp = false,
                                                               cot = 20,
                                                               common_address = entry.fc.ToString(),
                                                               address = entry.path,
                                                               asdu = tp.ToString(),
                                                               quality = !failed,
                                                               selfPublish = false,
                                                               conn_name = srv.name,
                                                               conn_number = srv.protocolConnectionNumber,
                                                               display_name = entry.path,
                                                           };
                                                           IECDataQueue.Enqueue(iv);

                                                           if (LogLevel > LogLevelNoLog) log += "    v=" + v.ToString("G", CultureInfo.CreateSpecificCulture("en-US")) + " f=" + failed + " t=" + timestamp;
                                                       }
                                                       else
                                                       {
                                                           v = MMSGetDoubleVal(value, out isBinary);
                                                           if (MMSTestDoubleStateFailed(value)) failed = true; // double state inconsistent status
                                                           string vstr;
                                                           if (isBinary)
                                                               vstr = v != 0 ? "true" : "false";
                                                           else
                                                               vstr = v.ToString("G", CultureInfo.CreateSpecificCulture("en-US"));

                                                           var iv = new IECValue
                                                           {
                                                               isDigital = isBinary,
                                                               value = v,
                                                               valueString = vstr,
                                                               valueJson = MMSGetStringValue(value),
                                                               serverTimestamp = DateTime.Now,
                                                               sourceTimestamp = DateTime.MinValue,
                                                               hasSourceTimestamp = false,
                                                               cot = 20,
                                                               common_address = entry.fc.ToString(),
                                                               address = entry.path,
                                                               asdu = tp.ToString(),
                                                               quality = !failed,
                                                               selfPublish = false,
                                                               conn_name = srv.name,
                                                               conn_number = srv.protocolConnectionNumber,
                                                               display_name = entry.path,
                                                           };
                                                           IECDataQueue.Enqueue(iv);

                                                           if (LogLevel > LogLevelNoLog) log += "    v=" + v.ToString("G", CultureInfo.CreateSpecificCulture("en-US"));
                                                       }
                                                   }
                                                   else
                                                   {
                                                       if (LogLevel > 0) log += "    Read error: " + err.ToString();
                                                   }
                                                   Log(log);
                                               }, null);

                                            // Thread.Sleep(500);
                                        }
                                        catch (IedConnectionException e)
                                        {
                                            err = e.GetErrorCode();
                                            if (e.GetErrorCode() == 6)
                                            {
                                                Thread.Sleep(250);
                                            }
                                            else
                                            if (LogLevel > LogLevelBasic)
                                                Log(srv.name + " Exception reading " + entry.path + " " + entry.fc + " error:" + e.GetErrorCode());
                                        }
                                    }
                                }
                            }

                            for (int i = 0; i < srv.giInterval * 10; i++)
                            {

                                if (!Active)
                                {
                                    throw new Exception(srv.name + " Node inactive! Disconnecting ...");
                                }
                                // wait 1/10 second
                                Thread.Sleep(100);
                            }
                            if (brcbCountPrev != srv.brcbCount)
                            {
                                brcbCountPrev = srv.brcbCount;
                                // File.WriteAllText(ReportIdsFilePrefix + srv.name + ".json", JsonSerializer.Serialize(srv.lastReportIds, new JsonSerializerOptions { WriteIndented = true }));
                            }
                        } while (true);
                    }
                    catch (Exception e)
                    {
                        Log(srv.name + "Exception");
                        if (LogLevel >= LogLevelDetailed)
                            Log(e);
                        else
                            Log(e.Message);
                        if (con.GetState() == IedConnectionState.IED_STATE_CONNECTED)
                            con.Abort();
                        con.Dispose();
                        con = null;
                        Thread.Sleep(5000);
                    }
                }
            } while (true);
        }
    }
}