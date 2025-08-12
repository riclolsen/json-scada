using IEC61850.Common;
using IEC61850.Server;
using System;
using System.Collections.Generic;
using System.Collections.ObjectModel;
using System.Data.Common;
using System.Linq;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading;
using System.Threading.Tasks;
using static IEC61850.Server.IedServer;
using static IEC61850.Server.LogStorage;

namespace log_server
{
    internal class Program
    {
        public static void Main(string[] args)
        {

            bool entryCallback(System.IntPtr self, long timestamp, long entryID1, bool moreFollow)
            {
                if (moreFollow)
                    Console.WriteLine($"Found entry ID:{entryID1} timestamp:{timestamp}");
                return true;
            }

             
            bool entryDataCallback(System.IntPtr self, string dataRef, byte[] data, int dataSize, int reasonCode, bool moreFollow)
            {
                if (moreFollow)
                {
                    Console.WriteLine($"EntryData: ref: {dataRef}\n");                   
                }

                return true;
            }

            bool running = true;

            /* run until Ctrl-C is pressed */
            Console.CancelKeyPress += delegate (object sender, ConsoleCancelEventArgs e)
            {
                e.Cancel = true;
                running = false;
            };

            IedModel iedModel = ConfigFileParser.CreateModelFromConfigFile("model.cfg");

            if (iedModel == null)
            {
                Console.WriteLine("No valid data model found!");
                return;
            }

            IedServerConfig config = new IedServerConfig();
            config.ReportBufferSize = 100000;

            IedServer iedServer = new IedServer(iedModel, config);

            LogStorage statusLog = SqliteLogStorage.CreateLogStorage("log_status.db");

            statusLog.MaxLogEntries = 10;

            iedServer.SetLogStorage("GenericIO/LLN0$EventLog", statusLog);

            long time = DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();          

            int entryID = statusLog.AddEntry(time);

            MmsValue value = new MmsValue(123);
            byte[] blob = new byte[256];

            int blobSize = value.EncodeMmsData(blob, 0, true);

            bool restun = statusLog.AddEntryData(entryID, "GenericIO/GGIO1.SPCSO1$stVal", blob, blobSize, 0);

            value.Dispose();
            ulong unixTimeMs = (ulong)DateTimeOffset.UtcNow.ToUnixTimeMilliseconds();


            entryID = statusLog.AddEntry((long)unixTimeMs);

            value = MmsValue.NewUtcTime(unixTimeMs);
            blob = new byte[256];

            blobSize = value.EncodeMmsData(blob, 0, true);

            value.Dispose();


            bool restun1 = statusLog.AddEntryData(entryID, "simpleIOGenerioIO/GPIO1$ST$SPCSO1$t", blob, blobSize, 0);


            bool return3 = statusLog.GetEntries(0, DateTimeOffset.UtcNow.ToUnixTimeMilliseconds(), (LogEntryCallback)entryCallback, (LogEntryDataCallback)entryDataCallback, (System.IntPtr)null);

            iedServer.Start(102);

            if (iedServer.IsRunning())
            {
                Console.WriteLine("Server started");

                DataObject ggio1AnIn1 = (DataObject)iedModel.GetModelNodeByShortObjectReference("GenericIO/GGIO1.AnIn1");

                DataAttribute ggio1AnIn1magF = (DataAttribute)ggio1AnIn1.GetChild("mag.f");
                DataAttribute ggio1AnIn1T = (DataAttribute)ggio1AnIn1.GetChild("t");

                DataObject ggio1Spcso1 = (DataObject)iedModel.GetModelNodeByShortObjectReference("GenericIO/GGIO1.SPCSO1");

                DataAttribute ggio1Spcso1stVal = (DataAttribute)ggio1Spcso1.GetChild("stVal");
                DataAttribute ggio1Spcso1T = (DataAttribute)ggio1Spcso1.GetChild("t");

                float floatVal = 1.0f;

                bool stVal = true;

                while (running)
                {
                    floatVal += 1f;
                    stVal = !stVal;

                    iedServer.LockDataModel();

                    long unixSeconds = DateTimeOffset.UtcNow.ToUnixTimeSeconds();
                    DateTime utcTime = DateTimeOffset.FromUnixTimeSeconds(unixSeconds).UtcDateTime;

                    var ts = new Timestamp(utcTime);
                    iedServer.UpdateTimestampAttributeValue(ggio1AnIn1T, ts);
                    iedServer.UpdateFloatAttributeValue(ggio1AnIn1magF, floatVal);
                    iedServer.UpdateTimestampAttributeValue(ggio1Spcso1T, ts);
                    iedServer.UpdateBooleanAttributeValue(ggio1Spcso1stVal, stVal);
                    iedServer.UnlockDataModel();

                    Thread.Sleep(100);
                }

                iedServer.Stop();
                Console.WriteLine("Server stopped");
            }
            else
            {
                Console.WriteLine("Failed to start server");
            }

            iedServer.Destroy();
            statusLog.Dispose();
        }
    }
}
