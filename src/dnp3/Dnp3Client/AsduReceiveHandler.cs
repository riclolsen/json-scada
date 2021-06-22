/* 
 * DNP3 Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020 - Ricardo L. Olsen
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
using System.Collections.Generic;
using MongoDB.Bson;
using Automatak.DNP3.Interface;

namespace Dnp3Driver
{
    partial class MainClass
    {
        // handle incomming protocol data
        public class MyChannelListener : IChannelListener
        {
            public DNP3_connection dnp3conn;

            void IChannelListener.OnStateChange(ChannelState state)
            {
                Log(dnp3conn.name + " - Channel State: " + state.ToString());
                if (state == ChannelState.OPEN)
                {
                    dnp3conn.isConnected = true;
                }
                else
                {
                    dnp3conn.isConnected = false;
                }

                if (state == ChannelState.CLOSED)
                {
                    // Ivalidate points on connection if disconnected
                    var Client = ConnectMongoClient(JSConfig);
                    var DB = Client.GetDatabase(JSConfig.mongoDatabaseName);
                    var collection = DB.GetCollection<rtData>(RealtimeDataCollectionName);
                    foreach (DNP3_connection srv in DNP3conns)
                    {
                        if (srv.channel == dnp3conn.channel) // all connections on the same channel
                        {
                            srv.isConnected = dnp3conn.isConnected;
                            if (dnp3conn.isConnected == false)
                            {
                                // update as invalid
                                Log("Invalidating points on connection " + srv.protocolConnectionNumber);
                                var filter =
                                    new BsonDocument(new BsonDocument("protocolSourceConnectionNumber",
                                        srv.protocolConnectionNumber));
                                var update =
                                    new BsonDocument("$set", new BsonDocument{
                                    {"invalid",  true},
                                    {"timeTag", BsonValue.Create(DateTime.Now) },
                                        });
                                var res = collection.UpdateManyAsync(filter, update);
                            }
                        }
                    }
                }
            }
        }

        // handle incomming protocol data
        public class MySOEHandler : ISOEHandler
        {
            public string ConnectionName;
            public int ConnectionNumber;

            public void BeginFragment(ResponseInfo info)
            {

            }
            public void EndFragment(ResponseInfo info)
            {

            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<Binary>> values)
            {
                var base_group = 1;
                var group = 1;
                var variation = 0;
                switch (info.variation)
                {
                    case GroupVariation.Group1Var0:
                        group = 1;
                        variation = 0;
                        break;
                    case GroupVariation.Group1Var1:
                        group = 1;
                        variation = 1;
                        break;
                    case GroupVariation.Group1Var2:
                        group = 1;
                        variation = 2;
                        break;
                    case GroupVariation.Group2Var0:
                        group = 2;
                        variation = 0;
                        break;
                    case GroupVariation.Group2Var1:
                        group = 2;
                        variation = 1;
                        break;
                    case GroupVariation.Group2Var2:
                        group = 2;
                        variation = 2;
                        break;
                    case GroupVariation.Group2Var3:
                        group = 2;
                        variation = 3;
                        break;
                }
                foreach (var idxvalue in values)
                {
                    var DNP3_Value = new DNP3_Value
                    {
                        address = idxvalue.Index,
                        base_group = base_group,
                        group = group,
                        variation = variation,
                        value = idxvalue.Value.Value ? 1 : 0,
                        valueBSON = new BsonDocumentWrapper(idxvalue.Value),
                        valueString = idxvalue.Value.ToString(),
                        cot = info.isEvent ? 3 : 20,
                        serverTimestamp = DateTime.Now,
                        hasSourceTimestamp = (idxvalue.Value.Timestamp.ToEpoch() != 0),
                        sourceTimestamp = idxvalue.Value.Timestamp.Value,
                        timeStampQuality = idxvalue.Value.Timestamp.Quality,
                        qOnline = idxvalue.Value.Quality.IsSet(BinaryQuality.ONLINE),
                        qRestart = idxvalue.Value.Quality.IsSet(BinaryQuality.RESTART),
                        qCommLost = idxvalue.Value.Quality.IsSet(BinaryQuality.COMM_LOST),
                        qRemoteForced = idxvalue.Value.Quality.IsSet(BinaryQuality.REMOTE_FORCED),
                        qLocalForced = idxvalue.Value.Quality.IsSet(BinaryQuality.LOCAL_FORCED),
                        qOverrange = false,
                        qRollover = false,
                        qDiscontinuity = false,
                        qReferenceError = false,
                        qTransient = false,
                        conn_number = ConnectionNumber
                    };
                    DNP3DataQueue.Enqueue(DNP3_Value);
                    if (LogLevel >= LogLevelDetailed)
                        Log(ConnectionName + ": Binary, Ind: " + idxvalue.Index + " " + idxvalue.Value.ToString(), LogLevelDetailed);
                }
            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<DoubleBitBinary>> values)
            {
                var base_group = 3;
                var group = 3;
                var variation = 0;
                switch (info.variation)
                {
                    case GroupVariation.Group3Var0:
                        group = 3;
                        variation = 0;
                        break;
                    case GroupVariation.Group3Var1:
                        group = 3;
                        variation = 1;
                        break;
                    case GroupVariation.Group3Var2:
                        group = 3;
                        variation = 2;
                        break;
                    case GroupVariation.Group4Var0:
                        group = 4;
                        variation = 0;
                        break;
                    case GroupVariation.Group4Var1:
                        group = 4;
                        variation = 1;
                        break;
                    case GroupVariation.Group4Var2:
                        group = 4;
                        variation = 2;
                        break;
                    case GroupVariation.Group4Var3:
                        group = 4;
                        variation = 3;
                        break;
                }
                foreach (var idxvalue in values)
                {
                    var DNP3_Value = new DNP3_Value
                    {
                        address = idxvalue.Index,
                        base_group = base_group,
                        group = group,
                        variation = variation,
                        value = ( idxvalue.Value.Value == DoubleBit.DETERMINED_ON || idxvalue.Value.Value == DoubleBit.INDETERMINATE )? 1 : 0,
                        valueBSON = new BsonDocumentWrapper(idxvalue.Value),
                        valueString = idxvalue.Value.ToString(),
                        cot = info.isEvent ? 3 : 20,
                        serverTimestamp = DateTime.Now,
                        hasSourceTimestamp = (idxvalue.Value.Timestamp.ToEpoch() != 0),
                        sourceTimestamp = idxvalue.Value.Timestamp.Value,
                        timeStampQuality = idxvalue.Value.Timestamp.Quality,
                        qOnline = idxvalue.Value.Quality.IsSet(BinaryQuality.ONLINE),
                        qRestart = idxvalue.Value.Quality.IsSet(BinaryQuality.RESTART),
                        qCommLost = idxvalue.Value.Quality.IsSet(BinaryQuality.COMM_LOST),
                        qRemoteForced = idxvalue.Value.Quality.IsSet(BinaryQuality.REMOTE_FORCED),
                        qLocalForced = idxvalue.Value.Quality.IsSet(BinaryQuality.LOCAL_FORCED),
                        qOverrange = false,
                        qRollover = false,
                        qDiscontinuity = false,
                        qReferenceError = false,
                        qTransient = idxvalue.Value.Value == DoubleBit.INTERMEDIATE || idxvalue.Value.Value == DoubleBit.INDETERMINATE,
                        conn_number = ConnectionNumber
                    };
                    DNP3DataQueue.Enqueue(DNP3_Value);
                    if (LogLevel >= LogLevelDetailed)
                        Log(ConnectionName + ": Double Binary, Ind: " + idxvalue.Index + " " + idxvalue.Value.ToString(), LogLevelDetailed);
                }
            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<Analog>> values)
            {
                var base_group = 30;
                var group = 30;
                var variation = 0;
                switch (info.variation)
                {
                    case GroupVariation.Group30Var0:
                        group = 30;
                        variation = 0;
                        break;
                    case GroupVariation.Group30Var1:
                        group = 30;
                        variation = 1;
                        break;
                    case GroupVariation.Group30Var2:
                        group = 30;
                        variation = 2;
                        break;
                    case GroupVariation.Group30Var3:
                        group = 30;
                        variation = 3;
                        break;
                    case GroupVariation.Group30Var4:
                        group = 30;
                        variation = 4;
                        break;
                    case GroupVariation.Group30Var5:
                        group = 30;
                        variation = 5;
                        break;
                    case GroupVariation.Group30Var6:
                        group = 30;
                        variation = 6;
                        break;
                    case GroupVariation.Group32Var0:
                        group = 32;
                        variation = 0;
                        break;
                    case GroupVariation.Group32Var1:
                        group = 32;
                        variation = 1;
                        break;
                    case GroupVariation.Group32Var2:
                        group = 32;
                        variation = 2;
                        break;
                    case GroupVariation.Group32Var3:
                        group = 32;
                        variation = 3;
                        break;
                    case GroupVariation.Group32Var4:
                        group = 32;
                        variation = 4;
                        break;
                    case GroupVariation.Group32Var5:
                        group = 32;
                        variation = 5;
                        break;
                    case GroupVariation.Group32Var6:
                        group = 32;
                        variation = 6;
                        break;
                    case GroupVariation.Group32Var7:
                        group = 32;
                        variation = 7;
                        break;
                    case GroupVariation.Group32Var8:
                        group = 32;
                        variation = 8;
                        break;
                }
                foreach (var idxvalue in values)
                {
                    var DNP3_Value = new DNP3_Value
                    {
                        address = idxvalue.Index,
                        base_group = base_group,
                        group = group,
                        variation = variation,
                        value = idxvalue.Value.Value,
                        valueBSON = new BsonDocumentWrapper(idxvalue.Value),
                        valueString = idxvalue.Value.ToString(),
                        cot = info.isEvent ? 3 : 20,
                        serverTimestamp = DateTime.Now,
                        hasSourceTimestamp = (idxvalue.Value.Timestamp.ToEpoch() != 0),
                        sourceTimestamp = idxvalue.Value.Timestamp.Value,
                        timeStampQuality = idxvalue.Value.Timestamp.Quality,
                        qOnline = idxvalue.Value.Quality.IsSet(AnalogQuality.ONLINE),
                        qRestart = idxvalue.Value.Quality.IsSet(AnalogQuality.RESTART),
                        qCommLost = idxvalue.Value.Quality.IsSet(AnalogQuality.COMM_LOST),
                        qRemoteForced = idxvalue.Value.Quality.IsSet(AnalogQuality.REMOTE_FORCED),
                        qLocalForced = idxvalue.Value.Quality.IsSet(AnalogQuality.LOCAL_FORCED),
                        qOverrange = idxvalue.Value.Quality.IsSet(AnalogQuality.OVERRANGE),
                        qRollover = false,
                        qDiscontinuity = false,
                        qReferenceError = idxvalue.Value.Quality.IsSet(AnalogQuality.REFERENCE_ERR),
                        qTransient = false,
                        conn_number = ConnectionNumber
                    };
                    DNP3DataQueue.Enqueue(DNP3_Value);
                    if (LogLevel >= LogLevelDetailed)
                        Log(ConnectionName + ": Analog, Ind: " + idxvalue.Index + " " + idxvalue.Value.ToString(), LogLevelDetailed);
                }
            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<Counter>> values)
            {
                var base_group = 20;
                var group = 20;
                var variation = 0;
                switch (info.variation)
                {
                    case GroupVariation.Group20Var0:
                        group = 20;
                        variation = 0;
                        break;
                    case GroupVariation.Group20Var1:
                        group = 20;
                        variation = 1;
                        break;
                    case GroupVariation.Group20Var2:
                        group = 20;
                        variation = 2;
                        break;
                    case GroupVariation.Group20Var5:
                        group = 20;
                        variation = 5;
                        break;
                    case GroupVariation.Group20Var6:
                        group = 20;
                        variation = 6;
                        break;
                    case GroupVariation.Group22Var0:
                        group = 22;
                        variation = 0;
                        break;
                    case GroupVariation.Group22Var1:
                        group = 22;
                        variation = 1;
                        break;
                    case GroupVariation.Group22Var2:
                        group = 22;
                        variation = 2;
                        break;
                    case GroupVariation.Group22Var5:
                        group = 22;
                        variation = 5;
                        break;
                    case GroupVariation.Group22Var6:
                        group = 22;
                        variation = 6;
                        break;
                }
                foreach (var idxvalue in values)
                {
                    var DNP3_Value = new DNP3_Value
                    {
                        address = idxvalue.Index,
                        base_group = base_group,
                        group = group,
                        variation = variation,
                        value = idxvalue.Value.Value,
                        valueBSON = new BsonDocumentWrapper(idxvalue.Value),
                        valueString = idxvalue.Value.ToString(),
                        cot = info.isEvent ? 3 : 20,
                        serverTimestamp = DateTime.Now,
                        hasSourceTimestamp = (idxvalue.Value.Timestamp.ToEpoch() != 0),
                        sourceTimestamp = idxvalue.Value.Timestamp.Value,
                        timeStampQuality = idxvalue.Value.Timestamp.Quality,
                        qOnline = idxvalue.Value.Quality.IsSet(CounterQuality.ONLINE),
                        qRestart = idxvalue.Value.Quality.IsSet(CounterQuality.RESTART),
                        qCommLost = idxvalue.Value.Quality.IsSet(CounterQuality.COMM_LOST),
                        qRemoteForced = idxvalue.Value.Quality.IsSet(CounterQuality.REMOTE_FORCED),
                        qLocalForced = idxvalue.Value.Quality.IsSet(CounterQuality.LOCAL_FORCED),
                        qOverrange = false,
                        qRollover = idxvalue.Value.Quality.IsSet(CounterQuality.ROLLOVER),
                        qDiscontinuity = idxvalue.Value.Quality.IsSet(CounterQuality.DISCONTINUITY),
                        qReferenceError = false,
                        qTransient = false,
                        conn_number = ConnectionNumber
                    };
                    DNP3DataQueue.Enqueue(DNP3_Value);
                    if (LogLevel >= LogLevelDetailed)
                        Log(ConnectionName + ": Counter, Ind: " + idxvalue.Index + " " + idxvalue.Value.ToString(), LogLevelDetailed);
                }
            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<FrozenCounter>> values)
            {
                var base_group = 23;
                var group = 23;
                var variation = 0;
                switch (info.variation)
                {
                    case GroupVariation.Group23Var0:
                        group = 23;
                        variation = 0;
                        break;
                    case GroupVariation.Group23Var1:
                        group = 23;
                        variation = 1;
                        break;
                    case GroupVariation.Group23Var2:
                        group = 23;
                        variation = 2;
                        break;
                    case GroupVariation.Group23Var5:
                        group = 23;
                        variation = 5;
                        break;
                    case GroupVariation.Group23Var6:
                        group = 23;
                        variation = 6;
                        break;
                }
                foreach (var idxvalue in values)
                {
                    var DNP3_Value = new DNP3_Value
                    {
                        address = idxvalue.Index,
                        base_group = base_group,
                        group = group,
                        variation = variation,
                        value = idxvalue.Value.Value,
                        valueBSON = new BsonDocumentWrapper(idxvalue.Value),
                        valueString = idxvalue.Value.ToString(),
                        cot = info.isEvent ? 3 : 20,
                        serverTimestamp = DateTime.Now,
                        hasSourceTimestamp = (idxvalue.Value.Timestamp.ToEpoch() != 0),
                        sourceTimestamp = idxvalue.Value.Timestamp.Value,
                        timeStampQuality = idxvalue.Value.Timestamp.Quality,
                        qOnline = idxvalue.Value.Quality.IsSet(CounterQuality.ONLINE),
                        qRestart = idxvalue.Value.Quality.IsSet(CounterQuality.RESTART),
                        qCommLost = idxvalue.Value.Quality.IsSet(CounterQuality.COMM_LOST),
                        qRemoteForced = idxvalue.Value.Quality.IsSet(CounterQuality.REMOTE_FORCED),
                        qLocalForced = idxvalue.Value.Quality.IsSet(CounterQuality.LOCAL_FORCED),
                        qOverrange = false,
                        qRollover = idxvalue.Value.Quality.IsSet(CounterQuality.ROLLOVER),
                        qDiscontinuity = idxvalue.Value.Quality.IsSet(CounterQuality.DISCONTINUITY),
                        qReferenceError = false,
                        qTransient = false,
                        conn_number = ConnectionNumber
                    };
                    DNP3DataQueue.Enqueue(DNP3_Value);
                    if (LogLevel >= LogLevelDetailed)
                        Log(ConnectionName + ": FrozenCounter, Ind: " + idxvalue.Index + " " + idxvalue.Value.ToString(), LogLevelDetailed);
                }
            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<BinaryOutputStatus>> values)
            {
                var base_group = 10;
                var group = 10;
                var variation = 0;
                switch (info.variation)
                {
                    case GroupVariation.Group10Var0:
                        group = 10;
                        variation = 0;
                        break;
                    case GroupVariation.Group10Var1:
                        group = 10;
                        variation = 1;
                        break;
                    case GroupVariation.Group10Var2:
                        group = 10;
                        variation = 2;
                        break;
                    case GroupVariation.Group11Var0:
                        group = 11;
                        variation = 0;
                        break;
                    case GroupVariation.Group11Var1:
                        group = 11;
                        variation = 1;
                        break;
                    case GroupVariation.Group11Var2:
                        group = 11;
                        variation = 2;
                        break;
                }
                foreach (var idxvalue in values)
                {
                    var DNP3_Value = new DNP3_Value
                    {
                        address = idxvalue.Index,
                        base_group = base_group,
                        group = group,
                        variation = variation,
                        value = idxvalue.Value.Value ? 1 : 0,
                        valueBSON = new BsonDocumentWrapper(idxvalue.Value),
                        valueString = idxvalue.Value.ToString(),
                        cot = info.isEvent ? 3 : 20,
                        serverTimestamp = DateTime.Now,
                        hasSourceTimestamp = (idxvalue.Value.Timestamp.ToEpoch() != 0),
                        sourceTimestamp = idxvalue.Value.Timestamp.Value,
                        timeStampQuality = idxvalue.Value.Timestamp.Quality,
                        qOnline = idxvalue.Value.Quality.IsSet(BinaryOutputStatusQuality.ONLINE),
                        qRestart = idxvalue.Value.Quality.IsSet(BinaryOutputStatusQuality.RESTART),
                        qCommLost = idxvalue.Value.Quality.IsSet(BinaryOutputStatusQuality.COMM_LOST),
                        qRemoteForced = idxvalue.Value.Quality.IsSet(BinaryOutputStatusQuality.REMOTE_FORCED),
                        qLocalForced = idxvalue.Value.Quality.IsSet(BinaryOutputStatusQuality.LOCAL_FORCED),
                        qOverrange = false,
                        qRollover = false,
                        qDiscontinuity = false,
                        qReferenceError = false,
                        qTransient = false,
                        conn_number = ConnectionNumber
                    };
                    DNP3DataQueue.Enqueue(DNP3_Value);
                    if (LogLevel >= LogLevelDetailed)
                        Log(ConnectionName + ": BinaryOutput, Ind: " + idxvalue.Index + " " + idxvalue.Value.ToString(), LogLevelDetailed);
                }
            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<AnalogOutputStatus>> values)
            {
                var base_group = 40;
                var group = 40;
                var variation = 0;
                switch (info.variation)
                {
                    case GroupVariation.Group40Var0:
                        group = 40;
                        variation = 0;
                        break;
                    case GroupVariation.Group40Var1:
                        group = 40;
                        variation = 1;
                        break;
                    case GroupVariation.Group40Var2:
                        group = 40;
                        variation = 2;
                        break;
                    case GroupVariation.Group40Var3:
                        group = 40;
                        variation = 3;
                        break;
                    case GroupVariation.Group40Var4:
                        group = 40;
                        variation = 4;
                        break;
                    case GroupVariation.Group42Var0:
                        group = 42;
                        variation = 0;
                        break;
                    case GroupVariation.Group42Var1:
                        group = 42;
                        variation = 1;
                        break;
                    case GroupVariation.Group42Var2:
                        group = 42;
                        variation = 2;
                        break;
                    case GroupVariation.Group42Var3:
                        group = 42;
                        variation = 3;
                        break;
                    case GroupVariation.Group42Var4:
                        group = 42;
                        variation = 4;
                        break;
                    case GroupVariation.Group42Var5:
                        group = 42;
                        variation = 5;
                        break;
                    case GroupVariation.Group42Var6:
                        group = 42;
                        variation = 6;
                        break;
                    case GroupVariation.Group42Var7:
                        group = 42;
                        variation = 7;
                        break;
                    case GroupVariation.Group42Var8: 
                        group = 42;
                        variation = 8;
                        break;
                }
                foreach (var idxvalue in values)
                {
                    var DNP3_Value = new DNP3_Value
                    {
                        address = idxvalue.Index,
                        base_group = base_group,
                        group = group,
                        variation = variation,
                        value = idxvalue.Value.Value,
                        valueBSON = new BsonDocumentWrapper(idxvalue.Value),
                        valueString = idxvalue.Value.ToString(),
                        cot = info.isEvent ? 3 : 20,
                        serverTimestamp = DateTime.Now,
                        hasSourceTimestamp = (idxvalue.Value.Timestamp.ToEpoch() != 0),
                        sourceTimestamp = idxvalue.Value.Timestamp.Value,
                        timeStampQuality = idxvalue.Value.Timestamp.Quality,
                        qOnline = idxvalue.Value.Quality.IsSet(AnalogOutputStatusQuality.ONLINE),
                        qRestart = idxvalue.Value.Quality.IsSet(AnalogOutputStatusQuality.RESTART),
                        qCommLost = idxvalue.Value.Quality.IsSet(AnalogOutputStatusQuality.COMM_LOST),
                        qRemoteForced = idxvalue.Value.Quality.IsSet(AnalogOutputStatusQuality.REMOTE_FORCED),
                        qLocalForced = idxvalue.Value.Quality.IsSet(AnalogOutputStatusQuality.LOCAL_FORCED),
                        qOverrange = idxvalue.Value.Quality.IsSet(AnalogOutputStatusQuality.OVERRANGE),
                        qRollover = false,
                        qDiscontinuity = false,
                        qReferenceError = idxvalue.Value.Quality.IsSet(AnalogOutputStatusQuality.REFERENCE_ERR),
                        qTransient = false,
                        conn_number = ConnectionNumber
                    };
                    DNP3DataQueue.Enqueue(DNP3_Value);
                    if (LogLevel >= LogLevelDetailed)
                        Log(ConnectionName + ": AnalogOutputStatus, Ind: " + idxvalue.Index + " " + idxvalue.Value.ToString(), LogLevelDetailed);
                }
            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<AnalogOutputInt32>> values)
            {
                var base_group = 41;
                var group = 41;
                var variation = 1;
                switch (info.variation)
                {
                    case GroupVariation.Group41Var0:
                        group = 41;
                        variation = 0;
                        break;
                    case GroupVariation.Group41Var1:
                        group = 41;
                        variation = 1;
                        break;
                }
                foreach (var idxvalue in values)
                {
                    var DNP3_Value = new DNP3_Value
                    {
                        address = idxvalue.Index,
                        base_group = base_group,
                        group = group,
                        variation = variation,
                        value = idxvalue.Value.value,
                        valueBSON = new BsonDocumentWrapper(idxvalue.Value),
                        valueString = idxvalue.Value.ToString(),
                        cot = info.isEvent ? 3 : 20,
                        serverTimestamp = DateTime.Now,
                        hasSourceTimestamp = false,
                        sourceTimestamp = new DateTime(),
                        timeStampQuality = TimestampQuality.INVALID,
                        qOnline = true,
                        qRestart = false,
                        qCommLost = false,
                        qRemoteForced = false,
                        qLocalForced = false,
                        qOverrange = false,
                        qRollover = false,
                        qDiscontinuity = false,
                        qReferenceError = false,
                        qTransient = false,
                        conn_number = ConnectionNumber
                    };
                    DNP3DataQueue.Enqueue(DNP3_Value);
                    if (LogLevel >= LogLevelDetailed)
                        Log(ConnectionName + ": AnalogOutputInt32, Ind: " + idxvalue.Index + " " + idxvalue.Value.ToString(), LogLevelDetailed);
                }
            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<AnalogOutputInt16>> values)
            {
                var base_group = 41;
                var group = 41;
                var variation = 2;
                switch (info.variation)
                {
                    case GroupVariation.Group41Var0:
                        group = 41;
                        variation = 0;
                        break;
                    case GroupVariation.Group41Var2:
                        group = 41;
                        variation = 2;
                        break;
                }
                foreach (var idxvalue in values)
                {
                    var DNP3_Value = new DNP3_Value
                    {
                        address = idxvalue.Index,
                        base_group = base_group,
                        group = group,
                        variation = variation,
                        value = idxvalue.Value.value,
                        valueBSON = new BsonDocumentWrapper(idxvalue.Value),
                        valueString = idxvalue.Value.ToString(),
                        cot = info.isEvent ? 3 : 20,
                        serverTimestamp = DateTime.Now,
                        hasSourceTimestamp = false,
                        sourceTimestamp = new DateTime(),
                        timeStampQuality = TimestampQuality.INVALID,
                        qOnline = true,
                        qRestart = false,
                        qCommLost = false,
                        qRemoteForced = false,
                        qLocalForced = false,
                        qOverrange = false,
                        qRollover = false,
                        qDiscontinuity = false,
                        qReferenceError = false,
                        qTransient = false,
                        conn_number = ConnectionNumber
                    };
                    DNP3DataQueue.Enqueue(DNP3_Value);
                    if (LogLevel >= LogLevelDetailed)
                        Log(ConnectionName + ": AnalogOutputInt16, Ind: " + idxvalue.Index + " " + idxvalue.Value.ToString(), LogLevelDetailed);
                }
            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<AnalogOutputFloat32>> values)
            {
                var base_group = 41;
                var group = 41;
                var variation = 3;
                switch (info.variation)
                {
                    case GroupVariation.Group41Var0:
                        group = 41;
                        variation = 0;
                        break;
                    case GroupVariation.Group41Var3:
                        group = 41;
                        variation = 3;
                        break;
                }
                foreach (var idxvalue in values)
                {
                    var DNP3_Value = new DNP3_Value
                    {
                        address = idxvalue.Index,
                        base_group = base_group,
                        group = group,
                        variation = variation,
                        value = idxvalue.Value.value,
                        valueBSON = new BsonDocumentWrapper(idxvalue.Value),
                        valueString = idxvalue.Value.ToString(),
                        cot = info.isEvent ? 3 : 20,
                        serverTimestamp = DateTime.Now,
                        hasSourceTimestamp = false,
                        sourceTimestamp = new DateTime(),
                        timeStampQuality = TimestampQuality.INVALID,
                        qOnline = true,
                        qRestart = false,
                        qCommLost = false,
                        qRemoteForced = false,
                        qLocalForced = false,
                        qOverrange = false,
                        qRollover = false,
                        qDiscontinuity = false,
                        qReferenceError = false,
                        qTransient = false,
                        conn_number = ConnectionNumber
                    };
                    DNP3DataQueue.Enqueue(DNP3_Value);
                    if (LogLevel >= LogLevelDetailed)
                        Log(ConnectionName + ": AnalogOutputFloat32, Ind: " + idxvalue.Index + " " + idxvalue.Value.ToString(), LogLevelDetailed);
                }
            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<AnalogOutputDouble64>> values)
            {
                var base_group = 41;
                var group = 41;
                var variation = 1;
                switch (info.variation)
                {
                    case GroupVariation.Group41Var0:
                        group = 41;
                        variation = 0;
                        break;
                    case GroupVariation.Group41Var4:
                        group = 41;
                        variation = 4;
                        break;
                }
                foreach (var idxvalue in values)
                {
                    var DNP3_Value = new DNP3_Value
                    {
                        address = idxvalue.Index,
                        base_group = base_group,
                        group = group,
                        variation = variation,
                        value = idxvalue.Value.value,
                        valueBSON = new BsonDocumentWrapper(idxvalue.Value),
                        valueString = idxvalue.Value.ToString(),
                        cot = info.isEvent ? 3 : 20,
                        serverTimestamp = DateTime.Now,
                        hasSourceTimestamp = false,
                        sourceTimestamp = new DateTime(),
                        timeStampQuality = TimestampQuality.INVALID,
                        qOnline = true,
                        qRestart = false,
                        qCommLost = false,
                        qRemoteForced = false,
                        qLocalForced = false,
                        qOverrange = false,
                        qRollover = false,
                        qDiscontinuity = false,
                        qReferenceError = false,
                        qTransient = false,
                        conn_number = ConnectionNumber
                    };
                    DNP3DataQueue.Enqueue(DNP3_Value);
                    if (LogLevel >= LogLevelDetailed)
                        Log(ConnectionName + ": AnalogOutputDouble64, Ind: " + idxvalue.Index + " " + idxvalue.Value.ToString(), LogLevelDetailed);
                }
            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<OctetString>> values)
            {
                var base_group = 110;
                var group = 110;
                switch (info.variation)
                {
                    case GroupVariation.Group110Var0:
                        group = 110;
                        break;
                    case GroupVariation.Group111Var0:
                        group = 111;
                        break;
                }
                foreach (var idxvalue in values)
                {
                    var DNP3_Value = new DNP3_Value
                    {
                        address = idxvalue.Index,
                        base_group = base_group,
                        group = group,
                        variation = idxvalue.Value.Bytes.Length,
                        value = idxvalue.Value.Bytes.Length,
                        valueBSON = BsonValue.Create(idxvalue.Value.Bytes),
                        valueString = System.Text.Encoding.Default.GetString(idxvalue.Value.Bytes),
                        cot = info.isEvent ? 3 : 20,
                        serverTimestamp = DateTime.Now,
                        hasSourceTimestamp = false,
                        sourceTimestamp = new DateTime(),
                        timeStampQuality = TimestampQuality.INVALID,
                        qOnline = true,
                        qRestart = false,
                        qCommLost = false,
                        qRemoteForced = false,
                        qLocalForced = false,
                        qOverrange = false,
                        qRollover = false,
                        qDiscontinuity = false,
                        qReferenceError = false,
                        qTransient = false,
                        conn_number = ConnectionNumber
                    };
                    DNP3DataQueue.Enqueue(DNP3_Value);
                    if (LogLevel >= LogLevelDetailed)
                        Log(ConnectionName + ": OctetString, Ind: " + idxvalue.Index + " " + idxvalue.Value.ToString(), LogLevelDetailed);
                }
            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<TimeAndInterval>> values)
            {
                var base_group = 50;
                var group = 50;
                var variation = 1;
                switch (info.variation)
                {
                    case GroupVariation.Group50Var1:
                        group = 50;
                        variation = 1;
                        break;
                    case GroupVariation.Group50Var3:
                        group = 50;
                        variation = 3;
                        break;
                    case GroupVariation.Group50Var4:
                        group = 50;
                        variation = 4;
                        break;
                }
                foreach (var idxvalue in values)
                {
                    var DNP3_Value = new DNP3_Value
                    {
                        address = idxvalue.Index,
                        base_group = base_group,
                        group = group,
                        variation = variation,
                        value = idxvalue.Value.interval,
                        valueBSON = new BsonDocumentWrapper(idxvalue.Value),
                        valueString = idxvalue.Value.ToString(),
                        cot = info.isEvent ? 3 : 20,
                        serverTimestamp = DateTime.Now,
                        hasSourceTimestamp = true,
                        sourceTimestamp = (new DateTime()).AddSeconds(idxvalue.Value.time),
                        timeStampQuality = TimestampQuality.SYNCHRONIZED,
                        qOnline = true,
                        qRestart = false,
                        qCommLost = false,
                        qRemoteForced = false,
                        qLocalForced = false,
                        qOverrange = false,
                        qRollover = false,
                        qDiscontinuity = false,
                        qReferenceError = false,
                        qTransient = false,
                        conn_number = ConnectionNumber
                    };
                    DNP3DataQueue.Enqueue(DNP3_Value);
                    if (LogLevel >= LogLevelDetailed)
                        Log(ConnectionName + ": TimeAndInterval, Ind: " + idxvalue.Index + " " + idxvalue.Value.ToString(), LogLevelDetailed);
                }
            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<BinaryCommandEvent>> values)
            {
                var base_group = 13;
                var group = 13;
                var variation = 1;
                switch (info.variation)
                {
                    case GroupVariation.Group13Var1:
                        group = 13;
                        variation = 1;
                        break;
                    case GroupVariation.Group13Var2:
                        group = 13;
                        variation = 2;
                        break;
                }
                foreach (var idxvalue in values)
                {
                    var DNP3_Value = new DNP3_Value
                    {
                        address = idxvalue.Index,
                        base_group = base_group,
                        group = group,
                        variation = variation,
                        value = System.Convert.ToDouble(idxvalue.Value.Value),
                        valueBSON = new BsonDocumentWrapper(idxvalue.Value),
                        valueString = idxvalue.Value.ToString(),
                        cot = info.isEvent ? 3 : 20,
                        serverTimestamp = DateTime.Now,
                        hasSourceTimestamp = (idxvalue.Value.Timestamp.ToEpoch() != 0),
                        sourceTimestamp = idxvalue.Value.Timestamp.Value.Date,
                        timeStampQuality = idxvalue.Value.Timestamp.Quality,
                        qOnline = idxvalue.Value.Status == 0, // success
                        qRestart = false,
                        qCommLost = false,
                        qRemoteForced = false,
                        qLocalForced = false,
                        qOverrange = false,
                        qRollover = false,
                        qDiscontinuity = false,
                        qReferenceError = false,
                        qTransient = false,
                        conn_number = ConnectionNumber
                    };
                    DNP3DataQueue.Enqueue(DNP3_Value);
                    if (LogLevel >= LogLevelDetailed)
                        Log(ConnectionName + ": BinaryCommandEvent, Ind: " + idxvalue.Index + " " + idxvalue.Value.ToString(), LogLevelDetailed);
                }
            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<AnalogCommandEvent>> values)
            {
                var base_group = 43;
                var group = 43;
                var variation = 1;
                switch (info.variation)
                {
                    case GroupVariation.Group13Var1:
                        group = 43;
                        variation = 1;
                        break;
                    case GroupVariation.Group13Var2:
                        group = 43;
                        variation = 2;
                        break;
                }
                foreach (var idxvalue in values)
                {
                    var DNP3_Value = new DNP3_Value
                    {
                        address = idxvalue.Index,
                        base_group = base_group,
                        group = group,
                        variation = variation,
                        value = idxvalue.Value.Value,
                        valueBSON = new BsonDocumentWrapper(idxvalue.Value),
                        valueString = idxvalue.Value.ToString(),
                        cot = info.isEvent ? 3 : 20,
                        serverTimestamp = DateTime.Now,
                        hasSourceTimestamp = (idxvalue.Value.Timestamp.ToEpoch() != 0),
                        sourceTimestamp = idxvalue.Value.Timestamp.Value.Date,
                        timeStampQuality = idxvalue.Value.Timestamp.Quality,
                        qOnline = idxvalue.Value.Status == 0, // success
                        qRestart = false,
                        qCommLost = false,
                        qRemoteForced = false,
                        qLocalForced = false,
                        qOverrange = false,
                        qRollover = false,
                        qDiscontinuity = false,
                        qReferenceError = false,
                        qTransient = false,
                        conn_number = ConnectionNumber
                    };
                    DNP3DataQueue.Enqueue(DNP3_Value);
                    if (LogLevel >= LogLevelDetailed)
                        Log(ConnectionName + ": AnalogCommandEvent, Ind: " + idxvalue.Index + " " + idxvalue.Value.ToString(), LogLevelDetailed);
                }
            }
            public void Process(HeaderInfo info, IEnumerable<IndexedValue<SecurityStat>> values)
            {
            }
        }
    }
}