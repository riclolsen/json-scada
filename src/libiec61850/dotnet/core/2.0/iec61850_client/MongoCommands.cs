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
using MongoDB.Bson;
using MongoDB.Driver;
using System.Threading;
using IEC61850.Client;
using IEC61850.Common;

namespace IEC61850_Client
{
    partial class MainClass
    {
        // This process watches (via change stream) for commands inserted to a commands collection
        // When the command is considered valid it is forwarded to the RTU
        static async void ProcessMongoCmd(JSONSCADAConfig jsConfig)
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
                            <rtCommand>(CommandsQueueCollectionName);

                    bool isMongoLive =
                        DB
                            .RunCommandAsync((Command<BsonDocument>)"{ping:1}")
                            .Wait(1000);
                    if (!isMongoLive)
                        throw new Exception("Error on connection " + jsConfig.mongoConnectionString);

                    Log("MongoDB CMD CS - Start listening for commands via changestream...");
                    var filter = "{ operationType: 'insert' }";

                    var pipeline =
                        new EmptyPipelineDefinition<ChangeStreamDocument<rtCommand
                            >
                        >().Match(filter);
                    using (var cursor = await collection.WatchAsync(pipeline))
                    {
                        await cursor
                            .ForEachAsync(async change =>
                            {
                                if (!Active)
                                    return;

                                // process change event, only process inserts
                                if (
                                    change.OperationType ==
                                    ChangeStreamOperationType.Insert
                                )
                                {
                                    Log("MongoDB CMD CS - Looking for connection " +
                                    change
                                        .FullDocument
                                        .protocolSourceConnectionNumber +
                                    "...");
                                    var found = false;
                                    foreach (Iec61850Connection
                                        srv
                                        in
                                        Iec61850Connections
                                    )
                                    {
                                        if (
                                            srv.protocolConnectionNumber ==
                                            change
                                                .FullDocument
                                                .protocolSourceConnectionNumber
                                        )
                                        {
                                            found = true;

                                            int timeDif = DateTime
                                                    .Now
                                                    .ToLocalTime()
                                                    .Subtract(change
                                                        .FullDocument
                                                        .timeTag
                                                        .ToLocalTime(
                                                        ))
                                                    .Seconds;

                                            // test for command expired
                                            if (timeDif > 10)
                                            {
                                                // update as expired
                                                Log("MongoDB CMD CS - " +
                                                srv.name +
                                                " - " +
                                                " Address " +
                                                change
                                                    .FullDocument
                                                    .protocolSourceObjectAddress +
                                                " value " +
                                                change
                                                    .FullDocument
                                                    .value +
                                                " Expired, " + timeDif + " Seconds old");
                                                var filter =
                                                    new BsonDocument(new BsonDocument("_id",
                                                            change
                                                                .FullDocument
                                                                .id));
                                                var update =
                                                    new BsonDocument("$set",
                                                        new BsonDocument("cancelReason",
                                                            "expired"));
                                                var result =
                                                    await collection
                                                        .UpdateOneAsync(filter,
                                                        update);

                                                break;
                                            }

                                            if (
                                            srv.connection.GetState() == IedConnectionState.IED_STATE_CONNECTED &&
                                            srv.commandsEnabled &&
                                            srv.entries.ContainsKey(change.FullDocument.protocolSourceObjectAddress.ToString() + change.FullDocument.protocolSourceCommonAddress.ToString().ToUpper())
                                            )
                                            {
                                                var con = srv.connection;
                                                var ic = new Iec61850Control
                                                {
                                                    timestamp = change.FullDocument.timeTag.ToUniversalTime(),
                                                    js_cmd_tag = change.FullDocument.tag.ToString(),
                                                    value = change.FullDocument.value.ToDouble(),
                                                    fc = change.FullDocument.protocolSourceCommonAddress.ToString().ToUpper(),
                                                    iecEntry = srv.entries[change.FullDocument.protocolSourceObjectAddress.ToString() + change.FullDocument.protocolSourceCommonAddress.ToString().ToUpper()],
                                                };

                                                var resultDescription = "";
                                                string[] results = { "" };
                                                var okres = false;

                                                Log(srv.name + " Control " + ic.iecEntry.path + " Value " + ic.value);

                                                if (ic.iecEntry.fc != FunctionalConstraint.CO)
                                                { // simple MMS write when not control block
                                                    try
                                                    {
                                                        var mmsv = con.ReadValue(ic.iecEntry.path, ic.iecEntry.fc);
                                                        switch (mmsv.GetType())
                                                        {
                                                            default:
                                                            case MmsType.MMS_BCD:
                                                            case MmsType.MMS_OBJ_ID:
                                                            case MmsType.MMS_GENERALIZED_TIME:
                                                            case MmsType.MMS_STRUCTURE:
                                                            case MmsType.MMS_ARRAY:
                                                                okres = false;
                                                                Log(srv.name + " Writable object of unsupported type! " + ic.iecEntry.path);
                                                                break;
                                                            case MmsType.MMS_BOOLEAN:
                                                                con.WriteValue(ic.iecEntry.path, ic.iecEntry.fc, new MmsValue(ic.value != 0));
                                                                okres = true;
                                                                break;
                                                            case MmsType.MMS_UNSIGNED:
                                                                con.WriteValue(ic.iecEntry.path, ic.iecEntry.fc, new MmsValue((uint)ic.value));
                                                                okres = true;
                                                                break;
                                                            case MmsType.MMS_INTEGER:
                                                                con.WriteValue(ic.iecEntry.path, ic.iecEntry.fc, new MmsValue((long)ic.value));
                                                                okres = true;
                                                                break;
                                                            case MmsType.MMS_FLOAT:
                                                                con.WriteValue(ic.iecEntry.path, ic.iecEntry.fc, new MmsValue(ic.value));
                                                                okres = true;
                                                                break;
                                                            case MmsType.MMS_STRING:
                                                            case MmsType.MMS_VISIBLE_STRING:
                                                                con.WriteValue(ic.iecEntry.path, ic.iecEntry.fc, new MmsValue(ic.value.ToString()));
                                                                okres = true;
                                                                break;
                                                            case MmsType.MMS_BIT_STRING:
                                                                var bs = MmsValue.NewBitString(mmsv.Size());
                                                                bs.BitStringFromUInt32((uint)ic.value);
                                                                con.WriteValue(ic.iecEntry.path, ic.iecEntry.fc, bs);
                                                                okres = true;
                                                                break;
                                                            case MmsType.MMS_UTC_TIME:
                                                                var ut = MmsValue.NewUtcTime((ulong)ic.value);
                                                                con.WriteValue(ic.iecEntry.path, ic.iecEntry.fc, ut);
                                                                okres = true;
                                                                break;
                                                            case MmsType.MMS_BINARY_TIME:
                                                                var bt = MmsValue.NewBinaryTime(true);
                                                                bt.SetBinaryTime((ulong)ic.value);
                                                                con.WriteValue(ic.iecEntry.path, ic.iecEntry.fc, bt);
                                                                okres = true;
                                                                break;
                                                            case MmsType.MMS_OCTET_STRING:
                                                                var os = MmsValue.NewOctetString(mmsv.Size());
                                                                os.SetOctetStringOctet(0, (byte)(((uint)ic.value) % 256));
                                                                con.WriteValue(ic.iecEntry.path, ic.iecEntry.fc, os);
                                                                okres = true;
                                                                break;
                                                        }
                                                    }
                                                    catch (IedConnectionException ex)
                                                    {
                                                        okres = false;
                                                        Log(srv.name + " Writable object not found! " + ic.iecEntry.path);
                                                        Log(ex.Message);
                                                        return;
                                                    }
                                                }
                                                else
                                                { // control object
                                                    try
                                                    {
                                                        ControlObject control = con.CreateControlObject(ic.iecEntry.path);
                                                        if (control == null)
                                                        {
                                                            okres = false;
                                                            Log(srv.name + " Control object not found! " + ic.iecEntry.path);
                                                        }
                                                        else
                                                        {
                                                            control.SetOrigin(CopyrightMessage, OrCat.STATION_CONTROL);
                                                            control.SetInterlockCheck(true);
                                                            control.SetSynchroCheck(true);
                                                            control.SetTestMode(false);

                                                            ControlModel controlModel = control.GetControlModel();
                                                            MmsType controlType = control.GetCtlValType();
                                                            Log(ic.iecEntry.path + " has control model " + controlModel.ToString());
                                                            Log("  type of ctlVal: " + controlType.ToString());

                                                            switch (controlModel)
                                                            {
                                                                case ControlModel.STATUS_ONLY:
                                                                    okres = false;
                                                                    Log("Control is status-only!");
                                                                    break;
                                                                case ControlModel.DIRECT_NORMAL:
                                                                case ControlModel.DIRECT_ENHANCED:                                                     
                                                                    switch (controlType)
                                                                    {
                                                                        case MmsType.MMS_BOOLEAN:
                                                                            if (control.Operate(ic.value != 0))
                                                                                if (control.Operate(ic.value != 0))
                                                                                {
                                                                                    okres = true;
                                                                                    Log("Operated successfully!");
                                                                                }
                                                                                else
                                                                                {
                                                                                    okres = false;
                                                                                    Log("Operate failed!");
                                                                                }
                                                                            break;
                                                                        case MmsType.MMS_UNSIGNED:
                                                                        case MmsType.MMS_INTEGER:
                                                                            if (control.Operate(ic.value != 0))
                                                                            {
                                                                                okres = true;
                                                                                Log("Operated successfully!");
                                                                            }
                                                                            else
                                                                            {
                                                                                okres = false;
                                                                                Log("Operate failed!");
                                                                            }
                                                                            break;
                                                                        case MmsType.MMS_FLOAT:
                                                                            if (control.Operate(ic.value != 0))
                                                                            {
                                                                                okres = true;
                                                                                Log("Operated successfully!");
                                                                            }
                                                                            else
                                                                            {
                                                                                okres = false;
                                                                                Log("Operate failed!");
                                                                            }
                                                                            break;
                                                                        default:
                                                                            Log("Unsupported Command Type!");
                                                                            break;
                                                                    }
                                                                    break;
                                                                case ControlModel.SBO_NORMAL:
                                                                case ControlModel.SBO_ENHANCED:
                                                                    if (control.Select())
                                                                    {
                                                                        switch (controlType)
                                                                        {
                                                                            case MmsType.MMS_BOOLEAN:
                                                                                if (control.Operate(ic.value != 0))
                                                                                    if (control.Operate(ic.value != 0))
                                                                                    {
                                                                                        okres = true;
                                                                                        Log("Operated successfully!");
                                                                                    }
                                                                                    else
                                                                                    {
                                                                                        okres = false;
                                                                                        Log("Operate failed!");
                                                                                    }
                                                                                break;
                                                                            case MmsType.MMS_UNSIGNED:
                                                                            case MmsType.MMS_INTEGER:
                                                                                if (control.Operate(ic.value != 0))
                                                                                {
                                                                                    okres = true;
                                                                                    Log("Operated successfully!");
                                                                                }
                                                                                else
                                                                                {
                                                                                    okres = false;
                                                                                    Log("Operate failed!");
                                                                                }
                                                                                break;
                                                                            case MmsType.MMS_FLOAT:
                                                                                if (control.Operate(ic.value != 0))
                                                                                {
                                                                                    okres = true;
                                                                                    Log("Operated successfully!");
                                                                                }
                                                                                else
                                                                                {
                                                                                    okres = false;
                                                                                    Log("Operate failed!");
                                                                                }
                                                                                break;
                                                                            default:
                                                                                Log("Unsupported Command Type!");
                                                                                break;
                                                                        }
                                                                    }
                                                                    else
                                                                    {
                                                                        okres = false;
                                                                        Log("Select failed!");
                                                                    }
                                                                    break;
                                                            }
                                                            control.Dispose();
                                                        }
                                                    }
                                                    catch (IedConnectionException ex)
                                                    {
                                                        okres = false;
                                                        Log(srv.name + " Control object exception! " + ic.iecEntry.path);
                                                        return;
                                                    }
                                                }

                                                Log("MongoDB CMD CS - " +
                                                    srv.name +
                                                    " - " +
                                                    " Address: " +
                                                    change
                                                        .FullDocument
                                                        .protocolSourceObjectAddress +
                                                    " - Command delivered - " + results[0].ToString());

                                                // update as delivered
                                                var filter =
                                                    new BsonDocument(new BsonDocument("_id",
                                                            change
                                                                .FullDocument
                                                                .id));
                                                var update =
                                                    new BsonDocument
                                                        { {"$set",
                                                                    new BsonDocument{
                                                                        { "delivered", true },
                                                                        { "ack", okres },
                                                                        { "ackTimeTag", new BsonDateTime(DateTime.Now) },
                                                                        { "resultDescription", resultDescription }
                                                                    }
                                                                } };
                                                var result =
                                                    await collection
                                                        .UpdateOneAsync(filter,
                                                        update);
                                            }
                                            else
                                            {
                                                // update as canceled (not connected, not found, disabled)
                                                Log("MongoDB CMD CS - " +
                                                srv.name +
                                                " OA " +
                                                change
                                                    .FullDocument
                                                    .protocolSourceObjectAddress +
                                                " value " +
                                                change.FullDocument.value +
                                                (
                                                !srv.commandsEnabled
                                                    ? " Commands Disabled"
                                                    : srv.entries.ContainsKey(change.FullDocument.protocolSourceObjectAddress.ToString() + 
                                                                              change.FullDocument.protocolSourceCommonAddress.ToString().ToUpper()) 
                                                    ? " Not connected" : " Command not found!"
                                                ));
                                                var filter =
                                                    new BsonDocument(new BsonDocument("_id",
                                                            change
                                                                .FullDocument
                                                                .id));
                                                var update =
                                                    new BsonDocument("$set",
                                                        new BsonDocument("cancelReason",
                                                            (
                                                            !srv.commandsEnabled
                                                                ? "commands disabled"
                                                                : srv.entries.ContainsKey(change.FullDocument.protocolSourceObjectAddress.ToString() + 
                                                                                          change.FullDocument.protocolSourceCommonAddress.ToString().ToUpper()) 
                                                                ? "not connected" : "command not found!"
                                                            )));
                                                var result =
                                                    await collection
                                                        .UpdateOneAsync(filter,
                                                        update);
                                            }
                                            break;
                                        }
                                    }
                                    if (!found)
                                    {
                                        // not for a connection managed by this driver instance, just ignore
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
                    Thread.Sleep(3000);
                }
            }
            while (true);
        }
    }
}