/* 
 * IEC 60870-5-104 Server Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2024 - Ricardo L. Olsen
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
using System.Text.Json;
using Technosoftware.DaAeHdaClient;
using Technosoftware.DaAeHdaClient.Da;

namespace OPCDAClientDriver
{
    partial class MainClass
    {
        public static void OnDataChangeEvent(object subscriptionHandle, object requestHandle, TsCDaItemValueResult[] values, ref OPCDA_connection srv)
        {
            string connName= srv.name;
            Log(string.Format($"{connName} - DataChange: {values.Length} -----------------------------------------------------------"), 1);
            //if (requestHandle != null)
            //{
            //    Log("DataChange() for requestHandle :" + requestHandle.GetHashCode().ToString());
            //}
            for (var i = 0; i < values.Length; i++)
            {                
                if (values[i].Result.IsSuccess() && values[i].Value != null)
                {
                    string strHandle = string.Format("{0}", values[i].ClientHandle);
                    double value = 0;
                    string valueJson = string.Empty;
                    string valueString = string.Empty;
                    bool isGood = true;
                    bool isDigital = false;
                    convertItemValue(values[i].Value, out value, out valueString, out valueJson, out isGood, out isDigital);
                    isGood = values[i].Quality.QualityBits.HasFlag(TsDaQualityBits.Good) && isGood;
                    if (LogLevel > LogLevelDetailed)
                        Log($"{srv.name} - {values[i].ItemName} {valueString} {values[i].Quality} {values[i].Value.GetType().Name}", LogLevelDetailed);

                    var ov = new OPC_Value()
                    {
                        valueJson = valueJson,
                        selfPublish = true,
                        address = values[i].ItemName,
                        asdu = values[i].Value.GetType().Name,
                        isDigital = isDigital,
                        isArray = values[i].Value.GetType().IsArray,
                        value = value,
                        valueString = valueString,
                        cot = 3,
                        serverTimestamp = DateTime.Now,
                        sourceTimestamp = values[i].Timestamp,
                        hasSourceTimestamp = true,
                        isGood = isGood,
                        conn_number = srv.protocolConnectionNumber,
                        conn_name = srv.name,
                        display_name = values[i].ItemName,
                    };
                    OPCDataQueue.Enqueue(ov);
                    /*
                    if (values[i].Value.GetType().IsArray)
                    {
                        for (var j = 0; j < values.Length; j++)
                        {
                            if (LogLevel >= LogLevelDetailed && j < 33 * LogLevel)
                                Log($"{connName} - Change: {srv.MapHandlerToItemName[strHandle]}  Val[{j}]: " + string.Format("{0}", values[j].Value), 2);
                        }
                        if (LogLevel >= LogLevelDetailed)
                        {
                            Log($"{connName} - Change: {srv.MapHandlerToItemName[strHandle]} TS: " + values[i].Timestamp.ToString(CultureInfo.InvariantCulture), 2);
                            Log($"{connName} - Change: {srv.MapHandlerToItemName[strHandle]} Q: " + values[i].Quality, 2);
                        }
                        }
                        else
                    {
                        var valueResult = values[i];
                        //var quality = new TsCDaQuality(193);
                        //valueResult.Quality = quality;
                        //var message =
                        //    $"\r\n    Quality: is not good : {valueResult.Quality} Code:{valueResult.Quality.GetCode()} LimitBits: {valueResult.Quality.LimitBits} QualityBits: {valueResult.Quality.QualityBits} VendorBits: {valueResult.Quality.VendorBits}";
                        //if (valueResult.Quality.QualityBits != TsDaQualityBits.Good && valueResult.Quality.QualityBits != TsDaQualityBits.GoodLocalOverride)
                        //{
                        //    Log(message);
                        //}

                        if (LogLevel >= LogLevelDetailed && i < 33 * LogLevel)
                            Log($"{connName} - Change: {srv.MapHandlerToItemName[strHandle]} Val: {values[i].Value} Q: {values[i].Quality} TS: {values[i].Timestamp.ToString(CultureInfo.InvariantCulture)}", 2);
                    }
                    */
                }
                //Console.Write("    Result        : "); Console.WriteLine(values[i].Result.Description());
            }
            Log("----------------------------------------------------------- End", 2);
        }
        public static void convertItemValue(object iv, out double value, out string valueString, out string valueJson, out bool quality, out bool isDigital)
        {
            value = 0;
            valueJson = string.Empty;
            valueString = string.Empty;
            quality = true;
            isDigital = false;
            try
            {
                if (iv.GetType().IsArray)
                {
                    valueJson = JsonSerializer.Serialize(iv);
                    valueString = valueJson;
                }
                else
                    switch (iv.GetType().Name)
                    {
                        case "String":
                            valueString = Convert.ToString(iv);
                            valueJson = JsonSerializer.Serialize(iv);
                            break;
                        case "Boolean":
                            value = Convert.ToBoolean(iv) ? 1 : 0;
                            isDigital = true;
                            valueJson = JsonSerializer.Serialize(iv);
                            valueString = valueJson;
                            break;
                        case "SByte":
                            value = Convert.ToSByte(iv);
                            valueJson = JsonSerializer.Serialize(iv);
                            valueString = valueJson;
                            break;
                        case "Decimal":
                            value = Convert.ToDouble(Convert.ToDecimal(iv));
                            valueJson = JsonSerializer.Serialize(iv);
                            valueString = valueJson;
                            break;
                        case "Time":
                        case "DateTime":
                            value = Convert.ToDateTime(iv).Subtract(DateTime.UnixEpoch).TotalMilliseconds;
                            valueString = Convert.ToDateTime(iv).ToString();
                            valueJson = JsonSerializer.Serialize(iv);
                            break;
                        case "Single":
                        case "Double":
                            value = Convert.ToDouble(iv);
                            valueString = Convert.ToDouble(iv).ToString();
                            valueJson = JsonSerializer.Serialize(iv);
                            break;
                        case "Int64":
                            value = Convert.ToDouble(iv);
                            valueString = Convert.ToInt64(iv).ToString();
                            valueJson = JsonSerializer.Serialize(iv);
                            break;
                        case "UInt64":
                            value = Convert.ToDouble(iv);
                            valueString = Convert.ToUInt64(iv).ToString();
                            valueJson = JsonSerializer.Serialize(iv);
                            break;
                        case "Int32":
                            value = Convert.ToDouble(iv);
                            valueString = Convert.ToInt32(iv).ToString();
                            valueJson = JsonSerializer.Serialize(iv);
                            break;
                        case "UInt32":
                            value = Convert.ToDouble(iv);
                            valueString = Convert.ToUInt32(iv).ToString();
                            valueJson = JsonSerializer.Serialize(iv);
                            break;
                        case "Int16":
                            value = Convert.ToDouble(iv);
                            valueString = Convert.ToInt16(iv).ToString();
                            valueJson = JsonSerializer.Serialize(iv);
                            break;
                        case "UInt16":
                            value = Convert.ToDouble(iv);
                            valueString = Convert.ToUInt16(iv).ToString();
                            valueJson = JsonSerializer.Serialize(iv);
                            break;
                        default:
                            value = Convert.ToDouble(iv);
                            valueJson = JsonSerializer.Serialize(iv);
                            valueString = valueJson;
                            break;
                    }
            }
            catch
            {
                value = 0;
                quality = false;
                Log(iv.GetType().Name);
            }
        }

        public static void BrowseServer(ref TsCDaServer daServer, OpcItem item, ref List<TsCDaItem> itemsBrowsed, ref OPCDA_connection srv)
        {
            TsCDaBrowsePosition position = null;
            TsCDaBrowseFilters filters = new TsCDaBrowseFilters();
            filters.BrowseFilter = TsCDaBrowseFilter.All;

            TsCDaBrowseElement[] elements = daServer.Browse(item, filters, out position);

            if (elements != null)
            {
                do
                {
                    foreach (TsCDaBrowseElement elem in elements)
                    {
                        item = new OpcItem(elem.ItemPath, elem.ItemName);
                        // srv.topics.Length==0 : read all branches
                        if (elem.GetType() == typeof(TsCDaBrowseElement) && elem.HasChildren && (srv.topics.Length == 0 || srv.branches.Contains(elem.Name)))
                        {
                            BrowseServer(ref daServer, item, ref itemsBrowsed, ref srv);
                        }

                        if (!elem.HasChildren)
                        {
                            var it = new TsCDaItem(item);
                            HandleCnt++;
                            it.ClientHandle = HandleCnt;
                            srv.MapHandlerToItemName[it.ClientHandle.ToString()] = it.ItemName;
                            srv.MapHandlerToConnName[it.ClientHandle.ToString()] = daServer.ClientName;
                            // MapNameToHandler[it.ItemName] = it.ClientHandle.ToString();
                            itemsBrowsed.Add(it);
                        }
                    }
                    if (position != null)
                    {
                        elements = daServer.BrowseNext(ref position);
                        continue;
                    }
                } while (position != null);
            }
        }
    }
}