using System;
using MongoDB.Bson;
using MongoDB.Driver;
using lib60870;
using lib60870.CS101;
using System.Threading;
using System.Linq;

namespace Iec10XDriver
{
    partial class MainClass
    {
        static void DequeueIecInfo()
        {
            do
            {
                var sentSomething = false;
                foreach (IEC10X_connection
                    srv
                    in
                    IEC10Xconns
                )
                {
                    InfoCA ica;
                    if (srv.infoCAQueue.TryDequeue(out ica))
                    {
                        sentSomething = true;
                        var conNameStr = srv.name + " - ";
                        ApplicationLayerParameters alp =
                            srv.server.GetApplicationLayerParameters();
                        var newAsdu = new ASDU(alp,
                                           CauseOfTransmission.SPONTANEOUS,
                                           false,
                                           false,
                                           System.Convert.ToByte(alp.OA),
                                           ica.ca,
                                           false);
                        newAsdu.AddInformationObject(ica.io);
                        var cnt = 1;
                        InfoCA ica2;
                        // keep the same asdu while same type and common address, and size fits
                        while (srv.infoCAQueue.TryPeek(out ica2)) // do not remove from queue
                        {
                            if (ica2.ca == ica.ca && ica2.io.Type == ica.io.Type && cnt < 30) // check for same data type/ca and size
                            {
                                if (srv.infoCAQueue.TryDequeue(out ica2)) // will remove from queue only to send
                                {
                                    newAsdu.AddInformationObject(ica2.io);
                                    cnt++;
                                }
                                else
                                    break; // could not remove from queue, move on to enqueue asdu
                            }
                            else
                            {   // changed type or common address, move on to enqueue asdu
                                break;
                            }                            
                        }
                        srv.server.EnqueueASDU(newAsdu);
                        if (LogLevel >= LogLevelBasic)
                            Log(conNameStr + "Spont ASDU Type: " + ica.io.Type + " with " + newAsdu.NumberOfElements + " objects", LogLevelBasic);
                    }
                }
                if (!sentSomething) // if nothing was sent
                  Thread.Sleep(200); // let's give some time to wait for more data

            } while (true);
        }

        // This process watches (via change stream) for point updates 
        // Forward data to its connections 
        static async void ProcessMongoCS(JSONSCADAConfig jsConfig)
        {
            do
            {
                try
                {
                    var Client = ConnectMongoClient(jsConfig);
                    var DB = Client.GetDatabase(jsConfig.mongoDatabaseName);
                    var collection =
                        DB
                            .GetCollection
                            <rtData>(RealtimeDataCollectionName);

                    bool isMongoLive =
                        DB
                            .RunCommandAsync((Command<BsonDocument>)"{ping:1}")
                            .Wait(1000);
                    if (!isMongoLive)
                        throw new Exception("Error on connection " + jsConfig.mongoConnectionString);

                    Log("MongoDB CMD CS - Start listening for realtime data updates via changestream...");
                    // observe updates and replaces, avoid updates with sourceDataUpdateField (those are handled by cs_data_processor.js)
                    var filter = "{ $or: [{ $and:[{ 'updateDescription.updatedFields.sourceDataUpdate': { $exists: false } },{ operationType: 'update' }] }, { operationType: 'replace'}] }";

                    var pipeline =
                        new EmptyPipelineDefinition<ChangeStreamDocument<rtData
                            >
                        >().Match(filter);
                    var changeStreamOptions = new ChangeStreamOptions
                    {
                        FullDocument = ChangeStreamFullDocumentOption.UpdateLookup
                    };
                    using (var cursor = await collection.WatchAsync(pipeline, changeStreamOptions))
                    {
                        await cursor
                            .ForEachAsync(change =>
                            {
                                // process change event, only process updates and replaces
                                if (
                                    change.OperationType == ChangeStreamOperationType.Update ||
                                    change.OperationType == ChangeStreamOperationType.Replace
                                )
                                {
                                    if (change.FullDocument != null)
                                        if (change.FullDocument.protocolDestinations != null)
                                            foreach (var dst in change.FullDocument.protocolDestinations)
                                            {
                                                foreach (IEC10X_connection
                                                    srv
                                                    in
                                                    IEC10Xconns
                                                )
                                                {
                                                    if (dst.protocolDestinationConnectionNumber == srv.protocolConnectionNumber)
                                                    {
                                                        var conNameStr = srv.name + " - ";
                                                        ApplicationLayerParameters alp =
                                                            srv.server.GetApplicationLayerParameters();
                                                        var quality = new QualityDescriptor();
                                                        quality.Invalid = change.FullDocument.invalid.ToBoolean() ||
                                                                          change.FullDocument.overflow.ToBoolean() ||
                                                                          change.FullDocument.transient.ToBoolean();
                                                        quality.Substituted = change.FullDocument.substituted.ToBoolean();
                                                        quality.Blocked = false;
                                                        quality.NonTopical = false;
                                                        CP56Time2a cp56timesrc = null;
                                                        if (change.FullDocument.timeTagAtSource != null)
                                                        {
                                                            cp56timesrc = new CP56Time2a(System.Convert.ToDateTime(change.FullDocument.timeTagAtSource).AddHours(dst.protocolDestinationHoursShift.ToDouble()));
                                                            cp56timesrc.Invalid = false;
                                                            if (change.FullDocument.timeTagAtSourceOk != null)
                                                                cp56timesrc.Invalid = !change.FullDocument.timeTagAtSourceOk.ToBoolean();
                                                            else
                                                                cp56timesrc.Invalid = true;
                                                        }
                                                        var io = BuildInfoObj(
                                                                    dst.protocolDestinationASDU.ToInt32(),
                                                                    dst.protocolDestinationObjectAddress.ToInt32(),
                                                                    change.FullDocument.value.ToDouble(),
                                                                    false,
                                                                    0,
                                                                    quality,
                                                                    cp56timesrc
                                                                    );
                                                        if (io != null)
                                                        {
                                                            // queue data to make possible to assemble an ASDU with many elements, will send on DequeueIecInfo
                                                            InfoCA ica = new InfoCA()
                                                            {
                                                                io = io,
                                                                ca = dst.protocolDestinationCommonAddress.ToInt32()
                                                            };
                                                            srv.infoCAQueue.Enqueue(ica);

                                                            if (LogLevel >= LogLevelDetailed)
                                                                Log(conNameStr + "Spont Tag:" +
                                                                    change.FullDocument.tag + " Value:" + change.FullDocument.value +
                                                                    " Key:" + change.FullDocument._id + " TI:" + dst.protocolDestinationASDU.ToInt32() + " CA:" + dst.protocolDestinationCommonAddress + (cp56timesrc == null ? "" : " " + cp56timesrc.ToString()),
                                                                    LogLevelDetailed);
                                                        }
                                                    }
                                                }
                                            }
                                }
                            });
                    }
                }
                catch (Exception e)
                {
                    Log("Exception MongoCmd");
                    Log(e);
                    Log(e
                        .ToString()
                        .Substring(0,
                        e.ToString().IndexOf(Environment.NewLine)));
                    System.Threading.Thread.Sleep(3000);
                }
            }
            while (true);
        }
    }
}