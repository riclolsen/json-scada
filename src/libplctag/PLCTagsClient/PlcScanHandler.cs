/* 
 * IEC 60870-5-104 Client Protocol driver for {json:scada}
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

using libplctag.DataTypes;
using System;
using System.Diagnostics;
using System.Net.Sockets;
using System.Security.Cryptography;
using System.Threading;
using System.Threading.Tasks;

namespace PLCTagDriver
{
    partial class MainClass
    {
        static async void ProcessPLCScan(PLC_connection srv)
        {
            for (; ;) 
            {
                if (!Active)
                {
                    Thread.Sleep(1000);
                    continue;
                }

                var asyncStopWatch = Stopwatch.StartNew();

                foreach (var tag in srv.listBoolTags)
                {
                    try
                    {
                        await tag.ReadAsync();
                        PLC_Value iv =
                            new PLC_Value()
                            {
                                conn_number = 81,
                                address = tag.Name,
                                common_address = tag.Path,
                                asdu = tag.GetType().ToString(),
                                isDigital = true,
                                value = tag.Value ? 1 : 0,
                                time_tag = DateTime.Now,
                                cot = 20
                            };
                        PLCDataQueue.Enqueue(iv);
                        Log(srv.name + "- " + tag.Name + " " + tag.Value);
                    }
                    catch (Exception e)
                    {
                        Log(srv.name + "- " + "Error scanning tag: " + tag.Name);
                        Log(srv.name + "- " + e);
                    }
                }

                foreach (var tag in srv.listSintTags)
                {
                    try
                    {
                        await tag.ReadAsync();
                        PLC_Value iv =
                            new PLC_Value()
                            {
                                conn_number = 81,
                                address = tag.Name,
                                common_address = tag.Path,
                                asdu = tag.GetType().ToString(),
                                isDigital = true,
                                value = tag.Value,
                                time_tag = DateTime.Now,
                                cot = 20
                            };
                        PLCDataQueue.Enqueue(iv);
                        Log(srv.name + "- " + tag.Name + " " + tag.Value);
                    }
                    catch (Exception e)
                    {
                        Log(srv.name + "- " + "Error scanning tag: " + tag.Name);
                        Log(srv.name + "- " + e);                        
                    }
                }

                foreach (var tag in srv.listIntTags)
                {
                    try
                    {
                        await tag.ReadAsync();
                        PLC_Value iv =
                            new PLC_Value()
                            {
                                conn_number = 81,
                                address = tag.Name,
                                common_address = tag.Path,
                                asdu = tag.GetType().ToString(),
                                isDigital = true,
                                value = tag.Value,
                                time_tag = DateTime.Now,
                                cot = 20
                            };
                        PLCDataQueue.Enqueue(iv);
                        Log(srv.name + "- " + tag.Name + " " + tag.Value);
                    }
                    catch (Exception e)
                    {
                        Log(srv.name + "- " + "Error scanning tag: " + tag.Name);
                        Log(srv.name + "- " + e);
                    }
                }

                // Task.WaitAll(srv.listDintTags[0].ReadAsync(), srv.listDintTags[1].ReadAsync());

                foreach (var tag in srv.listDintTags)
                {
                    try
                    {
                        await tag.ReadAsync();
                        PLC_Value iv =
                            new PLC_Value()
                            {
                                conn_number = 81,
                                address = tag.Name,
                                common_address = tag.Path,
                                asdu = tag.GetType().ToString(),
                                isDigital = true,
                                value = tag.Value,
                                time_tag = DateTime.Now,
                                cot = 20
                            };
                        PLCDataQueue.Enqueue(iv);
                        Log(srv.name + "- " + tag.Name + " " + tag.Value);
                    }
                    catch (Exception e)
                    {
                        Log(srv.name + "- " + "Error scanning tag: " + tag.Name);
                        Log(srv.name + "- " + e);
                    }
                }

                foreach (var tag in srv.listLintTags)
                {
                    try
                    {
                        await tag.ReadAsync();
                        PLC_Value iv =
                            new PLC_Value()
                            {
                                conn_number = 81,
                                address = tag.Name,
                                common_address = tag.Path,
                                asdu = tag.GetType().ToString(),
                                isDigital = true,
                                value = tag.Value,
                                time_tag = DateTime.Now,
                                cot = 20
                            };
                        PLCDataQueue.Enqueue(iv);
                        Log(srv.name + "- " + tag.Name + " " + tag.Value);
                    }
                    catch (Exception e)
                    {
                        Log(srv.name + "- " + "Error scanning tag: " + tag.Name);
                        Log(srv.name + "- " + e);
                    }
                }
                foreach (var tag in srv.listRealTags)
                {
                    try
                    {
                        await tag.ReadAsync();
                        PLC_Value iv =
                            new PLC_Value()
                            {
                                conn_number = 81,
                                address = tag.Name,
                                common_address = tag.Path,
                                asdu = tag.GetType().ToString(),
                                isDigital = true,
                                value = tag.Value,
                                time_tag = DateTime.Now,
                                cot = 20
                            };
                        PLCDataQueue.Enqueue(iv);
                        Log(srv.name + "- " + tag.Name + " " + tag.Value);
                    }
                    catch (Exception e)
                    {
                        Log(srv.name + "- " + "Error scanning tag: " + tag.Name);
                        Log(srv.name + "- " + e);
                    }
                }
                foreach (var tag in srv.listLrealTags)
                {
                    try
                    {
                        await tag.ReadAsync();
                        PLC_Value iv =
                            new PLC_Value()
                            {
                                conn_number = 81,
                                address = tag.Name,
                                common_address = tag.Path,
                                asdu = tag.GetType().ToString(),
                                isDigital = true,
                                value = tag.Value,
                                time_tag = DateTime.Now,
                                cot = 20
                            };
                        PLCDataQueue.Enqueue(iv);
                        Log(srv.name + "- " + tag.Name + " " + tag.Value);
                    }
                    catch (Exception e)
                    {
                        Log(srv.name + "- " + "Error scanning tag: " + tag.Name);
                        Log(srv.name + "- " + e);
                    }
                }

                foreach (var tag in srv.listBoolArrayTags)
                {
                    try
                    {
                        await tag.ReadAsync();

                        var cnt = 0;
                        foreach (var value in tag.Value)
                        {
                            PLC_Value iv =
                                new PLC_Value()
                                {
                                    conn_number = 81,
                                    address = tag.Name + "[" + cnt + "]",
                                    common_address = tag.Path,
                                    asdu = tag.GetType().ToString(),
                                    isDigital = true,
                                    value = value ? 1 : 0,
                                    time_tag = DateTime.Now,
                                    cot = 20
                                };
                            PLCDataQueue.Enqueue(iv);
                            Log(srv.name + "- " + iv.address + " " + iv.value);
                            cnt++;
                        }
                    }
                    catch (Exception e)
                    {
                        Log("Error scanning tag: " + tag.Name);
                        Log(e);
                    }
                }

                foreach (var tag in srv.listRealArrayTags)
                {
                    try
                    {
                        await tag.ReadAsync();

                        var cnt = 0;
                        foreach (var value in tag.Value)
                        {
                            PLC_Value iv =
                                new PLC_Value()
                                {
                                    conn_number = 81,
                                    address = tag.Name + "[" + cnt + "]",
                                    common_address = tag.Path,
                                    asdu = tag.GetType().ToString(),
                                    isDigital = true,
                                    value = value,
                                    time_tag = DateTime.Now,
                                    cot = 20
                                };
                            PLCDataQueue.Enqueue(iv);
                            Log(srv.name + "- " + iv.address + " " + iv.value);
                            cnt++;
                        }
                    }
                    catch (Exception e)
                    {
                        Log("Error scanning tag: " + tag.Name);
                        Log(e);
                    }
                }

                asyncStopWatch.Stop();
                Log($"{srv.name} - Connection scan took {(float)asyncStopWatch.ElapsedMilliseconds} ms.");

                Log($"{srv.name} - Sleep {(float)srv.giInterval} ms...");
                Thread.Sleep(srv.giInterval);
            }
        }
    }
}