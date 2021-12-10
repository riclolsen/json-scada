/* 
 * IEC 60870-5-101 Client Protocol driver for {json:scada}
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
using lib60870;
using lib60870.CS101;

namespace Iec10XDriver
{
    partial class MainClass
    {
        // This is a handler IEC10X to receive and enqueue data (and command acks) comming from RTU connections
        // Data will be dequeued and updated in mongodb by another process
        private static bool AsduReceivedHandler(object parameter, ASDU asdu)
        {
            var srv = IEC10Xconns[(int)parameter];
            var conNameStr = srv.name + " - ";
            Log(conNameStr + asdu.ToString(), LogLevelDetailed);
            var invCP56 = new CP56Time2a();
            invCP56.Invalid = true;
            var invCP24 = new CP24Time2a();
            invCP24.Invalid = true;

            switch (asdu.TypeId)
            {
                case TypeID.M_SP_NA_1: // 1
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var val = (SinglePointInformation)asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = val.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = true,
                                value = System.Convert.ToDouble(val.Value),
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = val.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " SP value: " +
                        val.Value,
                        LogLevelDetailed);
                        Log(conNameStr + " - " + "   " + val.Quality.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_SP_TA_1: // 2
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var val =
                            (SinglePointWithCP24Time2a)asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = val.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = true,
                                value = System.Convert.ToDouble(val.Value),
                                hasSourceTimestampCP24 = true,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = val.Timestamp,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = val.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " SP value: " +
                        val.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Quality.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_DP_NA_1: // 3
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var val = (DoublePointInformation)asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = val.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = true,
                                value = System.Convert.ToDouble(val.Value),
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = val.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " DP value: " +
                        val.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Quality.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_DP_TA_1: // 4
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var val =
                            (DoublePointWithCP24Time2a)asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = val.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = true,
                                value = System.Convert.ToDouble(val.Value),
                                hasSourceTimestampCP24 = true,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = val.Timestamp,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = val.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " DP value: " +
                        val.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Quality.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_ST_NA_1: // 5
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var val = (StepPositionInformation)asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = val.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = val.Value,
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = val.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        iv.quality.Invalid = iv.quality.Invalid | val.Transient;
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " step pos: " +
                        val.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Quality.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_ST_TA_1: // 6
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var val =
                            (StepPositionWithCP24Time2a)asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = val.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = val.Value,
                                hasSourceTimestampCP24 = true,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = val.Timestamp,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = val.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        iv.quality.Invalid = iv.quality.Invalid | val.Transient;
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " step pos: " +
                        val.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Quality.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_BO_NA_1: // 7
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var bitstr = (Bitstring32)asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = bitstr.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = System.Convert.ToDouble(bitstr.Value),
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = new QualityDescriptor(),
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        bitstr.ObjectAddress +
                        " bitstring value: " +
                        bitstr.Value,
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_BO_TA_1: // 8
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var bitstr = (Bitstring32WithCP24Time2a)asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = bitstr.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = System.Convert.ToDouble(bitstr.Value),
                                hasSourceTimestampCP24 = true,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = bitstr.Timestamp,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = new QualityDescriptor(),
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        bitstr.ObjectAddress +
                        " bitstring value: " +
                        bitstr.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + bitstr.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_ME_NA_1: // 9
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var msv = (MeasuredValueNormalized)asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = msv.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = msv.NormalizedValue,
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = msv.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        msv.ObjectAddress +
                        " normalized value: " +
                        msv.NormalizedValue,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Quality.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_ME_TA_1: // 10
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var msv =
                            (MeasuredValueNormalizedWithCP24Time2a)
                            asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = msv.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = msv.NormalizedValue,
                                hasSourceTimestampCP24 = true,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = msv.Timestamp,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = msv.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        msv.ObjectAddress +
                        " normalized value: " +
                        msv.NormalizedValue,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Quality.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_ME_NB_1: // 11
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var msv = (MeasuredValueScaled)asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = msv.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = msv.ScaledValue.Value,
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = msv.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        msv.ObjectAddress +
                        " scaled value: " +
                        msv.ScaledValue,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Quality.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_ME_TB_1: // 12
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var msv =
                            (MeasuredValueScaledWithCP24Time2a)
                            asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = msv.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = msv.ScaledValue.Value,
                                hasSourceTimestampCP24 = true,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = msv.Timestamp,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = msv.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        msv.ObjectAddress +
                        " scaled value: " +
                        msv.ScaledValue,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Quality.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_ME_NC_1: // 13
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var msv = (MeasuredValueShort)asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = msv.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = msv.Value,
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = msv.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        msv.ObjectAddress +
                        " float value: " +
                        msv.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Quality.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_ME_TC_1: // 14
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var msv =
                            (MeasuredValueShortWithCP24Time2a)
                            asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = msv.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = msv.Value,
                                hasSourceTimestampCP24 = true,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = msv.Timestamp,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = msv.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        msv.ObjectAddress +
                        " float value: " +
                        msv.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Quality.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_IT_NA_1: // 15
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var msv = (IntegratedTotals)asdu.GetElement(i);
                        lib60870.CS101.QualityDescriptor qvalid =
                            new lib60870.CS101.QualityDescriptor();
                        qvalid.Invalid = false;
                        qvalid.NonTopical = false;
                        qvalid.Substituted = false;
                        qvalid.Blocked = false;
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = msv.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = msv.BCR.Value,
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = qvalid,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        msv.ObjectAddress +
                        " integrated total: " +
                        msv.BCR.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.BCR.Carry.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_IT_TA_1: // 16
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var msv =
                            (IntegratedTotalsWithCP24Time2a)asdu.GetElement(i);
                        lib60870.CS101.QualityDescriptor qvalid =
                            new lib60870.CS101.QualityDescriptor();
                        qvalid.Invalid = false;
                        qvalid.NonTopical = false;
                        qvalid.Substituted = false;
                        qvalid.Blocked = false;
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = msv.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = msv.BCR.Value,
                                hasSourceTimestampCP24 = true,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = msv.Timestamp,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = qvalid,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        msv.ObjectAddress +
                        " integrated total: " +
                        msv.BCR.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.BCR.Carry.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_EP_TA_1: // 17
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var val =
                            (EventOfProtectionEquipment)
                            asdu.GetElement(i);
                        var q = new QualityDescriptor();
                        q.Invalid = val.Event.QDP.Invalid;
                        q.NonTopical = val.Event.QDP.NonTopical;
                        q.Blocked = val.Event.QDP.Blocked;
                        q.Substituted = val.Event.QDP.Substituted;
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = val.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = true,
                                value =
                                    System
                                        .Convert
                                        .ToDouble(val.Event.State ==
                                        EventState.ON
                                            ? 1
                                            : 0),
                                hasSourceTimestampCP24 = true,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = val.Timestamp,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = q,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " Prot. State: " +
                        val.Event.State,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + q.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_EP_TB_1: // 18
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var val =
                            (PackedStartEventsOfProtectionEquipment)
                            asdu.GetElement(i);
                        var q = new QualityDescriptor();
                        q.Invalid = val.QDP.Invalid;
                        q.NonTopical = val.QDP.NonTopical;
                        q.Blocked = val.QDP.Blocked;
                        q.Substituted = val.QDP.Substituted;
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = val.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = true,
                                value = System.Convert.ToDouble(1),
                                hasSourceTimestampCP24 = true,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = val.Timestamp,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = q,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " Packed start event",
                        LogLevelDetailed);
                        Log(conNameStr + "   " + q.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_EP_TC_1: // 19
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var val =
                            (PackedOutputCircuitInfo)
                            asdu.GetElement(i);
                        var q = new QualityDescriptor();
                        q.Invalid = val.QDP.Invalid;
                        q.NonTopical = val.QDP.NonTopical;
                        q.Blocked = val.QDP.Blocked;
                        q.Substituted = val.QDP.Substituted;
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = val.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = true,
                                value = System.Convert.ToDouble(1),
                                hasSourceTimestampCP24 = true,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = val.Timestamp,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = q,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " Packed output ckt event",
                        LogLevelDetailed);
                        Log(conNameStr + "   " + q.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_PS_NA_1: // 20
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var pspscd = (PackedSinglePointWithSCD)asdu.GetElement(i);
                        var q = new QualityDescriptor();
                        q.Invalid = pspscd.QDS.Invalid;
                        q.NonTopical = pspscd.QDS.NonTopical;
                        q.Blocked = pspscd.QDS.Blocked;
                        q.Substituted = pspscd.QDS.Substituted;
                        q.Overflow = pspscd.QDS.Overflow;
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = pspscd.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = System.Convert.ToDouble(pspscd.SCD),
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = new QualityDescriptor(),
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        pspscd.ObjectAddress +
                        " Packed single point w/SCD value: " +
                        pspscd.SCD,
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_ME_ND_1: // 21
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        lib60870.CS101.QualityDescriptor qvalid =
                            new lib60870.CS101.QualityDescriptor();
                        qvalid.Invalid = false;
                        qvalid.NonTopical = false;
                        qvalid.Substituted = false;
                        qvalid.Blocked = false;
                        var msv =
                            (MeasuredValueNormalizedWithoutQuality)
                            asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = msv.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = msv.NormalizedValue,
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = false,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = invCP56,
                                serverTimestamp = DateTime.Now,
                                quality = qvalid,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        msv.ObjectAddress +
                        " normalized value: " +
                        msv.NormalizedValue,
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_SP_TB_1: // 30
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var val =
                            (SinglePointWithCP56Time2a)asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = val.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = true,
                                value = System.Convert.ToDouble(val.Value),
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = true,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = val.Timestamp,
                                serverTimestamp = DateTime.Now,
                                quality = val.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " SP value: " +
                        val.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Quality.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_DP_TB_1: // 31
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var val =
                            (DoublePointWithCP56Time2a)asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = val.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = true,
                                value = System.Convert.ToDouble(val.Value),
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = true,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = val.Timestamp,
                                serverTimestamp = DateTime.Now,
                                quality = val.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " SP value: " +
                        val.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Quality.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_ST_TB_1: // 32
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var msv =
                            (StepPositionWithCP56Time2a)asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = msv.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = msv.Value,
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = true,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = msv.Timestamp,
                                serverTimestamp = DateTime.Now,
                                quality = msv.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        iv.quality.Invalid = iv.quality.Invalid | msv.Transient;
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        msv.ObjectAddress +
                        " step pos: " +
                        msv.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Quality.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_BO_TB_1: // 33
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var bitstr = (Bitstring32WithCP56Time2a)asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = bitstr.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = System.Convert.ToDouble(bitstr.Value),
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = true,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = bitstr.Timestamp,
                                serverTimestamp = DateTime.Now,
                                quality = new QualityDescriptor(),
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        bitstr.ObjectAddress +
                        " bitstring value: " +
                        bitstr.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + bitstr.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_ME_TD_1: // 34
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var msv =
                            (MeasuredValueNormalizedWithCP56Time2a)
                            asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = msv.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = msv.NormalizedValue,
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = true,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = msv.Timestamp,
                                serverTimestamp = DateTime.Now,
                                quality = msv.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        msv.ObjectAddress +
                        " normalized value: " +
                        msv.NormalizedValue,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Quality.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_ME_TE_1: // 35
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var msv =
                            (MeasuredValueScaledWithCP56Time2a)
                            asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = msv.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = msv.ScaledValue.Value,
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = true,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = msv.Timestamp,
                                serverTimestamp = DateTime.Now,
                                quality = msv.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        msv.ObjectAddress +
                        " scaled value: " +
                        msv.ScaledValue,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Quality.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_ME_TF_1: // 36
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var msv =
                            (MeasuredValueShortWithCP56Time2a)
                            asdu.GetElement(i);
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = msv.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = msv.Value,
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = true,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = msv.Timestamp,
                                serverTimestamp = DateTime.Now,
                                quality = msv.Quality,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        msv.ObjectAddress +
                        " float value: " +
                        msv.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Quality.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_IT_TB_1: // 37
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var msv =
                            (IntegratedTotalsWithCP56Time2a)asdu.GetElement(i);
                        lib60870.CS101.QualityDescriptor qvalid =
                            new lib60870.CS101.QualityDescriptor();
                        qvalid.Invalid = false;
                        qvalid.NonTopical = false;
                        qvalid.Substituted = false;
                        qvalid.Blocked = false;
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = msv.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = false,
                                value = msv.BCR.Value,
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = true,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = msv.Timestamp,
                                serverTimestamp = DateTime.Now,
                                quality = qvalid,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        msv.ObjectAddress +
                        " integrated total: " +
                        msv.BCR.Value,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.BCR.Carry.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + msv.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_EP_TD_1: // 38
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var val =
                            (EventOfProtectionEquipmentWithCP56Time2a)
                            asdu.GetElement(i);
                        var q = new QualityDescriptor();
                        q.Invalid = val.Event.QDP.Invalid;
                        q.NonTopical = val.Event.QDP.NonTopical;
                        q.Blocked = val.Event.QDP.Blocked;
                        q.Substituted = val.Event.QDP.Substituted;
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = val.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = true,
                                value =
                                    System
                                        .Convert
                                        .ToDouble(val.Event.State ==
                                        EventState.ON
                                            ? 1
                                            : 0),
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = true,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = val.Timestamp,
                                serverTimestamp = DateTime.Now,
                                quality = q,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " Prot. State: " +
                        val.Event.State,
                        LogLevelDetailed);
                        Log(conNameStr + "   " + q.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_EP_TE_1: // 39
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var val =
                            (PackedStartEventsOfProtectionEquipmentWithCP56Time2a)
                            asdu.GetElement(i);
                        var q = new QualityDescriptor();
                        q.Invalid = val.QDP.Invalid;
                        q.NonTopical = val.QDP.NonTopical;
                        q.Blocked = val.QDP.Blocked;
                        q.Substituted = val.QDP.Substituted;
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = val.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = true,
                                value = System.Convert.ToDouble(1),
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = true,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = val.Timestamp,
                                serverTimestamp = DateTime.Now,
                                quality = q,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " Packed start event",
                        LogLevelDetailed);
                        Log(conNameStr + "   " + q.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.M_EP_TF_1: // 40
                    for (int i = 0; i < asdu.NumberOfElements; i++)
                    {
                        var val =
                            (PackedOutputCircuitInfoWithCP56Time2a)
                            asdu.GetElement(i);
                        var q = new QualityDescriptor();
                        q.Invalid = val.QDP.Invalid;
                        q.NonTopical = val.QDP.NonTopical;
                        q.Blocked = val.QDP.Blocked;
                        q.Substituted = val.QDP.Substituted;
                        IEC_Value iv =
                            new IEC_Value()
                            {
                                address = val.ObjectAddress,
                                asdu = asdu.TypeId,
                                isDigital = true,
                                value = System.Convert.ToDouble(1),
                                hasSourceTimestampCP24 = false,
                                hasSourceTimestampCP56 = true,
                                sourceTimestampCP24 = invCP24,
                                sourceTimestampCP56 = val.Timestamp,
                                serverTimestamp = DateTime.Now,
                                quality = q,
                                cot = (int)asdu.Cot,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                common_address = asdu.Ca
                            };
                        IECDataQueue.Enqueue(iv);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " Packed output ckt event",
                        LogLevelDetailed);
                        Log(conNameStr + "   " + q.ToString(),
                        LogLevelDetailed);
                        Log(conNameStr + "   " + val.Timestamp.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.C_SC_NA_1: // 45
                    {
                        var val = (SingleCommand)asdu.GetElement(0);
                        Log(conNameStr +
                        (val.Select ? "Select " : "Execute ") +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " State " +
                        val.State,
                        LogLevelDetailed);

                        if (val.Select && asdu.Cot == CauseOfTransmission.ACTIVATION_CON &&
                            !asdu.IsNegative && val.ObjectAddress == srv.LastCommandSelected.ObjectAddress)
                        {
                            srv
                                .master
                                .SendControlCommand(CauseOfTransmission
                                    .ACTIVATION,
                                asdu.Ca,
                                srv.LastCommandSelected);
                            Log(conNameStr +
                            " Sending command execute after select confirmed " +
                            " Object Address " +
                            val.ObjectAddress +
                            " State " +
                            val.State,
                            LogLevelDetailed);
                        }

                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.C_DC_NA_1: // 46
                    {
                        var val = (DoubleCommand)asdu.GetElement(0);
                        Log(conNameStr +
                        (val.Select ? "Select " : "Execute ") +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " State " +
                        val.State,
                        LogLevelDetailed);

                        if (val.Select && asdu.Cot == CauseOfTransmission.ACTIVATION_CON &&
                            !asdu.IsNegative && val.ObjectAddress == srv.LastCommandSelected.ObjectAddress)
                        {
                            srv
                                .master
                                .SendControlCommand(CauseOfTransmission
                                    .ACTIVATION,
                                asdu.Ca,
                                srv.LastCommandSelected);
                            Log(conNameStr +
                            " Sending command execute after select confirmed " +
                            " Object Address " +
                            val.ObjectAddress +
                            " State " +
                            val.State,
                            LogLevelDetailed);
                        }

                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.C_RC_NA_1: // 47
                    {
                        var val = (StepCommand)asdu.GetElement(0);
                        Log(conNameStr +
                        (val.Select ? "Select " : "Execute ") +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " State " +
                        val.State,
                        LogLevelDetailed);

                        if (val.Select && asdu.Cot == CauseOfTransmission.ACTIVATION_CON &&
                            !asdu.IsNegative && val.ObjectAddress == srv.LastCommandSelected.ObjectAddress)
                        {
                            srv
                                .master
                                .SendControlCommand(CauseOfTransmission
                                    .ACTIVATION,
                                asdu.Ca,
                                srv.LastCommandSelected);
                            Log(conNameStr +
                            " Sending command execute after select confirmed " +
                            " Object Address " +
                            val.ObjectAddress +
                            " State " +
                            val.State,
                            LogLevelDetailed);
                        }

                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.C_SE_NA_1: // 48
                    {
                        var val = (SetpointCommandNormalized)asdu.GetElement(0);
                        Log(conNameStr +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " State " +
                        val.NormalizedValue,
                        LogLevelDetailed);
                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.C_SE_NB_1: // 49
                    {
                        var val = (SetpointCommandScaled)asdu.GetElement(0);
                        Log(conNameStr +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " State " +
                        val.ScaledValue,
                        LogLevelDetailed);
                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.C_SE_NC_1: // 50
                    {
                        var val = (SetpointCommandShort)asdu.GetElement(0);
                        Log(conNameStr +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " State " +
                        val.Value,
                        LogLevelDetailed);
                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.C_BO_NA_1: // 51
                    {
                        var val = (Bitstring32Command)asdu.GetElement(0);
                        Log(conNameStr +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " State " +
                        val.Value,
                        LogLevelDetailed);
                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.C_SC_TA_1: // 58
                    {
                        var val = (SingleCommandWithCP56Time2a)asdu.GetElement(0);
                        Log(conNameStr +
                        (val.Select ? "Select " : "Execute ") +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " State " +
                        val.State,
                        LogLevelDetailed);

                        if (val.Select && asdu.Cot == CauseOfTransmission.ACTIVATION_CON &&
                            !asdu.IsNegative && val.ObjectAddress == srv.LastCommandSelected.ObjectAddress)
                        {
                            srv
                                .master
                                .SendControlCommand(CauseOfTransmission
                                    .ACTIVATION,
                                asdu.Ca,
                                srv.LastCommandSelected);
                            Log(conNameStr +
                            " Sending command execute after select confirmed " +
                            " Object Address " +
                            val.ObjectAddress +
                            " State " +
                            val.State,
                            LogLevelDetailed);
                        }

                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.C_DC_TA_1: // 59
                    {
                        var val = (DoubleCommandWithCP56Time2a)asdu.GetElement(0);
                        Log(conNameStr +
                        (val.Select ? "Select " : "Execute ") +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " State " +
                        val.State,
                        LogLevelDetailed);

                        if (val.Select && asdu.Cot == CauseOfTransmission.ACTIVATION_CON &&
                            !asdu.IsNegative && val.ObjectAddress == srv.LastCommandSelected.ObjectAddress)
                        {
                            srv
                                .master
                                .SendControlCommand(CauseOfTransmission
                                    .ACTIVATION,
                                asdu.Ca,
                                srv.LastCommandSelected);
                            Log(conNameStr +
                            " Sending command execute after select confirmed " +
                            " Object Address " +
                            val.ObjectAddress +
                            " State " +
                            val.State,
                            LogLevelDetailed);
                        }

                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.C_RC_TA_1: // 60    
                    {
                        var val = (StepCommandWithCP56Time2a)asdu.GetElement(0);
                        Log(conNameStr +
                        (val.Select ? "Select " : "Execute ") +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " State " +
                        val.State,
                        LogLevelDetailed);

                        if (val.Select && asdu.Cot == CauseOfTransmission.ACTIVATION_CON &&
                            !asdu.IsNegative && val.ObjectAddress == srv.LastCommandSelected.ObjectAddress)
                        {
                            srv
                                .master
                                .SendControlCommand(CauseOfTransmission
                                    .ACTIVATION,
                                asdu.Ca,
                                srv.LastCommandSelected);
                            Log(conNameStr +
                            " Sending command execute after select confirmed " +
                            " Object Address " +
                            val.ObjectAddress +
                            " State " +
                            val.State,
                            LogLevelDetailed);
                        }

                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.C_SE_TA_1: // 61
                    {
                        var val = (SetpointCommandNormalizedWithCP56Time2a)asdu.GetElement(0);
                        Log(conNameStr +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " State " +
                        val.NormalizedValue,
                        LogLevelDetailed);
                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.C_SE_TB_1: // 62
                    {
                        var val = (SetpointCommandScaledWithCP56Time2a)asdu.GetElement(0);
                        Log(conNameStr +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " State " +
                        val.ScaledValue,
                        LogLevelDetailed);
                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.C_SE_TC_1: // 63
                    {
                        var val = (SetpointCommandShortWithCP56Time2a)asdu.GetElement(0);
                        Log(conNameStr +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " State " +
                        val.Value,
                        LogLevelDetailed);
                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.C_BO_TA_1: // 64
                    {
                        var val = (Bitstring32CommandWithCP56Time2a)asdu.GetElement(0);
                        Log(conNameStr +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " State " +
                        val.Value,
                        LogLevelDetailed);
                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.C_TS_NA_1: // 104
                    {
                        Log(conNameStr +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for test command.",
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.C_TS_TA_1: // 107
                    {
                        var val = (TestCommandWithCP56Time2a)asdu.GetElement(0);
                        Log(conNameStr +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for test command. TSC: " + val.TSC + " Time: " + val.Time.ToString(),
                        LogLevelDetailed);
                    }
                    break;
                case TypeID.P_ME_NA_1: // 110
                    {
                        var val = (ParameterNormalizedValue)asdu.GetElement(0);
                        Log(conNameStr +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " Value " +
                        val.NormalizedValue +
                        " QPM " +
                        val.QPM,
                        LogLevelDetailed);
                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.P_ME_NB_1: // 111
                    {
                        var val = (ParameterScaledValue)asdu.GetElement(0);
                        Log(conNameStr +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " Value " +
                        val.ScaledValue +
                        " QPM " +
                        val.QPM,
                        LogLevelDetailed);
                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.P_ME_NC_1: // 112
                    {
                        var val = (ParameterFloatValue)asdu.GetElement(0);
                        Log(conNameStr +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " Value " +
                        val.Value +
                        " QPM " +
                        val.QPM,
                        LogLevelDetailed);
                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.P_AC_NA_1: // 113                
                    {
                        var val = (ParameterActivation)asdu.GetElement(0);
                        Log(conNameStr +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        " confirmation for command",
                        LogLevelDetailed);
                        Log(conNameStr +
                        " CA: " +
                        asdu.Ca +
                        " IOA: " +
                        val.ObjectAddress +
                        " QPA " +
                        val.QPA,
                        LogLevelDetailed);
                        IEC_CmdAck ia =
                            new IEC_CmdAck()
                            {
                                ack = !asdu.IsNegative,
                                object_address = val.ObjectAddress,
                                conn_number =
                                    (int)srv.protocolConnectionNumber,
                                ack_time_tag = DateTime.Now
                            };
                        IECCmdAckQueue.Enqueue(ia);
                    }
                    break;
                case TypeID.C_IC_NA_1: // 100
                    if (asdu.Cot == CauseOfTransmission.ACTIVATION_CON)
                        Log(conNameStr +
                        (asdu.IsNegative ? "Negative" : "Positive") +
                        "confirmation for interrogation command",
                        LogLevelDetailed);
                    else if (
                        asdu.Cot == CauseOfTransmission.ACTIVATION_TERMINATION
                    )
                        Log(conNameStr + "Interrogation command terminated",
                        LogLevelDetailed);
                    break;
                default:
                    Log(conNameStr +
                    "Unknown message type! " +
                    asdu.TypeId.ToString(),
                    LogLevelDetailed);
                    break;
            }

            return true;
        }
    }
}