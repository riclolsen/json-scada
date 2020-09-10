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

using libplctag;
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
        static void ProcessPLCScan(PLC_connection srv)
        {
            for (; ; )
            {
                try
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
                            var type = "bool";
                            tag.Read();
                            Log(tag.GetStatus().ToString());
                            if (tag.GetStatus() != Status.Ok)
                            {
                                Log(srv.name + " - ERROR!");
                            }
                            else
                            {
                                PLC_Value iv =
                                new PLC_Value()
                                {
                                    conn_number = 81,
                                    address = tag.Name,
                                    common_address = tag.Path,
                                    asdu = type,
                                    value = tag.Value?1:0,
                                    time_tag = DateTime.Now,
                                    cot = 20
                                };
                                PLCDataQueue.Enqueue(iv);
                                Log(srv.name + " - " + iv.address + " " + iv.asdu + " " + iv.value);
                            }
                        }
                        catch (LibPlcTagException e)
                        {
                            Log(srv.name + "- " + "Error scanning tag: " + tag.Name);
                            Log(srv.name + "- " + e);
                            Thread.Sleep(10000);
                        }
                    }

                    foreach (var tag in srv.listSintTags)
                    {
                        try
                        {
                            var type = "sint";
                            tag.Read();
                            Log(tag.GetStatus().ToString());
                            if (tag.GetStatus() != Status.Ok)
                            {
                                Log(srv.name + " - ERROR!");
                            }
                            else
                            {
                                PLC_Value iv =
                                new PLC_Value()
                                {
                                    conn_number = 81,
                                    address = tag.Name,
                                    common_address = tag.Path,
                                    asdu = type,
                                    value = tag.Value,
                                    time_tag = DateTime.Now,
                                    cot = 20
                                };
                                PLCDataQueue.Enqueue(iv);
                                Log(srv.name + " - " + iv.address + " " + iv.asdu + " " + iv.value);
                            }
                        }
                        catch (LibPlcTagException e)
                        {
                            Log(srv.name + "- " + "Error scanning tag: " + tag.Name);
                            Log(srv.name + "- " + e);
                            Thread.Sleep(10000);
                        }
                    }

                    foreach (var tag in srv.listIntTags)
                    {
                        try
                        {
                            var type = "int";
                            tag.Read();
                            Log(tag.GetStatus().ToString());
                            if (tag.GetStatus() != Status.Ok)
                            {
                                Log(srv.name + " - ERROR!");
                            }
                            else
                            {
                                PLC_Value iv =
                                new PLC_Value()
                                {
                                    conn_number = 81,
                                    address = tag.Name,
                                    common_address = tag.Path,
                                    asdu = type,
                                    value = tag.Value,
                                    time_tag = DateTime.Now,
                                    cot = 20
                                };
                                PLCDataQueue.Enqueue(iv);
                                Log(srv.name + " - " + iv.address + " " + iv.asdu + " " + iv.value);
                            }
                        }
                        catch (LibPlcTagException e)
                        {
                            Log(srv.name + "- " + "Error scanning tag: " + tag.Name);
                            Log(srv.name + "- " + e);
                            Thread.Sleep(10000);
                        }
                    }

                    foreach (var tag in srv.listDintTags)
                    {
                        try
                        {
                            var type = "dint";
                            tag.Read();
                            Log(tag.GetStatus().ToString());
                            if (tag.GetStatus() != Status.Ok)
                            {
                                Log(srv.name + " - ERROR!");
                            }
                            else
                            {
                                PLC_Value iv =
                                new PLC_Value()
                                {
                                    conn_number = 81,
                                    address = tag.Name,
                                    common_address = tag.Path,
                                    asdu = type,
                                    value = tag.Value,
                                    time_tag = DateTime.Now,
                                    cot = 20
                                };
                                PLCDataQueue.Enqueue(iv);
                                Log(srv.name + " - " + iv.address + " " + iv.asdu + " " + iv.value);
                            }
                        }
                        catch (LibPlcTagException e)
                        {
                            Log(srv.name + "- " + "Error scanning tag: " + tag.Name);
                            Log(srv.name + "- " + e);
                            Thread.Sleep(10000);
                        }
                    }

                    foreach (var tag in srv.listLintTags)
                    {
                        try
                        {
                            var type = "lint";
                            tag.Read();
                            Log(tag.GetStatus().ToString());
                            if (tag.GetStatus() != Status.Ok)
                            {
                                Log(srv.name + " - ERROR!");
                            }
                            else
                            {
                                PLC_Value iv =
                                new PLC_Value()
                                {
                                    conn_number = 81,
                                    address = tag.Name,
                                    common_address = tag.Path,
                                    asdu = type,
                                    value = tag.Value,
                                    time_tag = DateTime.Now,
                                    cot = 20
                                };
                                PLCDataQueue.Enqueue(iv);
                                Log(srv.name + " - " + iv.address + " " + iv.asdu + " " + iv.value);
                            }
                        }
                        catch (LibPlcTagException e)
                        {
                            Log(srv.name + "- " + "Error scanning tag: " + tag.Name);
                            Log(srv.name + "- " + e);
                            Thread.Sleep(10000);
                        }
                    }

                    foreach (var tag in srv.listRealTags)
                    {
                        try
                        {
                            var type = "real";
                            tag.Read();
                            Log(tag.GetStatus().ToString());
                            if (tag.GetStatus() != Status.Ok)
                            {
                                Log(srv.name + " - ERROR!");
                            }
                            else
                            {
                                PLC_Value iv =
                                new PLC_Value()
                                {
                                    conn_number = 81,
                                    address = tag.Name,
                                    common_address = tag.Path,
                                    asdu = type,
                                    value = tag.Value,
                                    time_tag = DateTime.Now,
                                    cot = 20
                                };
                                PLCDataQueue.Enqueue(iv);
                                Log(srv.name + " - " + iv.address + " " + iv.asdu + " " + iv.value);
                            }
                        }
                        catch (LibPlcTagException e)
                        {
                            Log(srv.name + "- " + "Error scanning tag: " + tag.Name);
                            Log(srv.name + "- " + e);
                            Thread.Sleep(10000);
                        }
                    }

                    foreach (var tag in srv.listLrealTags)
                    {
                        try
                        {
                            var type = "lreal";
                            tag.Read();
                            Log(tag.GetStatus().ToString());
                            if (tag.GetStatus() != Status.Ok)
                            {
                                Log(srv.name + " - ERROR!");
                            }
                            else
                            {
                                PLC_Value iv =
                                new PLC_Value()
                                {
                                    conn_number = 81,
                                    address = tag.Name,
                                    common_address = tag.Path,
                                    asdu = type,
                                    value = tag.Value,
                                    time_tag = DateTime.Now,
                                    cot = 20
                                };
                                PLCDataQueue.Enqueue(iv);
                                Log(srv.name + " - " + iv.address + " " + iv.asdu + " " + iv.value);
                            }
                        }
                        catch (LibPlcTagException e)
                        {
                            Log(srv.name + "- " + "Error scanning tag: " + tag.Name);
                            Log(srv.name + "- " + e);
                            Thread.Sleep(10000);
                        }
                    }

                    foreach (var tag in srv.listBoolArrayTags)
                    {
                        try
                        {
                            var type = "bool";
                            tag.Read();
                            Log(tag.GetStatus().ToString());
                            if (tag.GetStatus() != Status.Ok)
                            {
                                Log(srv.name + " - ERROR!");
                            }
                            else
                            {
                                var cnt = 0;
                                foreach (var value in tag.Value)
                                {
                                    PLC_Value iv =
                                    new PLC_Value()
                                    {
                                        conn_number = 81,
                                        address = tag.Name + "[" + cnt + "]",
                                        common_address = tag.Path,
                                        asdu = type,
                                        value = value ? 1 : 0,
                                        time_tag = DateTime.Now,
                                        cot = 20
                                    };
                                    PLCDataQueue.Enqueue(iv);
                                    Log(srv.name + " - " + iv.address + " " + iv.asdu + " " + iv.value);
                                    cnt++;
                                }
                            }
                        }
                        catch (Exception e)
                        {
                            Log("Error scanning tag: " + tag.Name);
                            Log(e);
                        }
                    }

                    foreach (var tag in srv.listSintArrayTags)
                    {
                        try
                        {
                            var type = "sint";
                            tag.Read();
                            Log(tag.GetStatus().ToString());
                            if (tag.GetStatus() != Status.Ok)
                            {
                                Log(srv.name + " - ERROR!");
                            }
                            else
                            {
                                var cnt = 0;
                                foreach (var value in tag.Value)
                                {
                                    PLC_Value iv =
                                    new PLC_Value()
                                    {
                                        conn_number = 81,
                                        address = tag.Name + "[" + cnt + "]",
                                        common_address = tag.Path,
                                        asdu = type,
                                        value = value,
                                        time_tag = DateTime.Now,
                                        cot = 20
                                    };
                                    PLCDataQueue.Enqueue(iv);
                                    Log(srv.name + " - " + iv.address + " " + iv.asdu + " " + iv.value);
                                    cnt++;
                                }
                            }
                        }
                        catch (Exception e)
                        {
                            Log("Error scanning tag: " + tag.Name);
                            Log(e);
                        }
                    }

                    foreach (var tag in srv.listIntArrayTags)
                    {
                        try
                        {
                            var type = "int";
                            tag.Read();
                            Log(tag.GetStatus().ToString());
                            if (tag.GetStatus() != Status.Ok)
                            {
                                Log(srv.name + " - ERROR!");
                            }
                            else
                            {
                                var cnt = 0;
                                foreach (var value in tag.Value)
                                {
                                    PLC_Value iv =
                                    new PLC_Value()
                                    {
                                        conn_number = 81,
                                        address = tag.Name + "[" + cnt + "]",
                                        common_address = tag.Path,
                                        asdu = type,
                                        value = value,
                                        time_tag = DateTime.Now,
                                        cot = 20
                                    };
                                    PLCDataQueue.Enqueue(iv);
                                    Log(srv.name + " - " + iv.address + " " + iv.asdu + " " + iv.value);
                                    cnt++;
                                }
                            }
                        }
                        catch (Exception e)
                        {
                            Log("Error scanning tag: " + tag.Name);
                            Log(e);
                        }
                    }

                    foreach (var tag in srv.listDintArrayTags)
                    {
                        try
                        {
                            var type = "dint";
                            tag.Read();
                            Log(tag.GetStatus().ToString());
                            if (tag.GetStatus() != Status.Ok)
                            {
                                Log(srv.name + " - ERROR!");
                            }
                            else
                            {
                                var cnt = 0;
                                foreach (var value in tag.Value)
                                {
                                    PLC_Value iv =
                                    new PLC_Value()
                                    {
                                        conn_number = 81,
                                        address = tag.Name + "[" + cnt + "]",
                                        common_address = tag.Path,
                                        asdu = type,
                                        value = value,
                                        time_tag = DateTime.Now,
                                        cot = 20
                                    };
                                    PLCDataQueue.Enqueue(iv);
                                    Log(srv.name + " - " + iv.address + " " + iv.asdu + " " + iv.value);
                                    cnt++;
                                }
                            }
                        }
                        catch (Exception e)
                        {
                            Log("Error scanning tag: " + tag.Name);
                            Log(e);
                        }
                    }

                    foreach (var tag in srv.listLintArrayTags)
                    {
                        try
                        {
                            var type = "lint";
                            tag.Read();
                            Log(tag.GetStatus().ToString());
                            if (tag.GetStatus() != Status.Ok)
                            {
                                Log(srv.name + " - ERROR!");
                            }
                            else
                            {
                                var cnt = 0;
                                foreach (var value in tag.Value)
                                {
                                    PLC_Value iv =
                                    new PLC_Value()
                                    {
                                        conn_number = 81,
                                        address = tag.Name + "[" + cnt + "]",
                                        common_address = tag.Path,
                                        asdu = type,
                                        value = value,
                                        time_tag = DateTime.Now,
                                        cot = 20
                                    };
                                    PLCDataQueue.Enqueue(iv);
                                    Log(srv.name + " - " + iv.address + " " + iv.asdu + " " + iv.value);
                                    cnt++;
                                }
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
                            var type = "real";
                            tag.Read();
                            Log(tag.GetStatus().ToString());
                            if (tag.GetStatus() != Status.Ok)
                            {
                                Log(srv.name + " - ERROR!");
                            }
                            else
                            {
                                var cnt = 0;
                                foreach (var value in tag.Value)
                                {
                                    PLC_Value iv =
                                    new PLC_Value()
                                    {
                                        conn_number = 81,
                                        address = tag.Name + "[" + cnt + "]",
                                        common_address = tag.Path,
                                        asdu = type,
                                        value = value,
                                        time_tag = DateTime.Now,
                                        cot = 20
                                    };
                                    PLCDataQueue.Enqueue(iv);
                                    Log(srv.name + " - " + iv.address + " " + iv.asdu + " " + iv.value);
                                    cnt++;
                                }
                            }
                        }
                        catch (Exception e)
                        {
                            Log("Error scanning tag: " + tag.Name);
                            Log(e);
                        }
                    }

                    foreach (var tag in srv.listLrealArrayTags)
                    {
                        try
                        {
                            var type = "lreal";
                            tag.Read();
                            Log(tag.GetStatus().ToString());
                            if (tag.GetStatus() != Status.Ok)
                            {
                                Log(srv.name + " - ERROR!");
                            }
                            else
                            {
                                var cnt = 0;
                                foreach (var value in tag.Value)
                                {
                                    PLC_Value iv =
                                    new PLC_Value()
                                    {
                                        conn_number = 81,
                                        address = tag.Name + "[" + cnt + "]",
                                        common_address = tag.Path,
                                        asdu = type,
                                        value = value,
                                        time_tag = DateTime.Now,
                                        cot = 20
                                    };
                                    PLCDataQueue.Enqueue(iv);
                                    Log(srv.name + " - " + iv.address + " " + iv.asdu + " " + iv.value);
                                    cnt++;
                                }
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
                catch (Exception e)
                {
                    Log(srv.name + " - Error scanning!");
                    Log(e);
                }
            }
        }
    }
}

/*
 * Async read
 * 
foreach (var tag in srv.listIntTags)
{
    try
    {
        var type = "int";
        var task = tag.ReadAsync();
        var conttask = task.ContinueWith((tsk) =>
        {
            PLC_Value iv =
                new PLC_Value()
                {
                    conn_number = 81,
                    address = tag.Name,
                    common_address = tag.Path,
                    asdu = type,
                    value = tag.Value,
                    time_tag = DateTime.Now,
                    cot = 20
                };
            PLCDataQueue.Enqueue(iv);
            Log(srv.name + " - " + iv.address + " " + iv.asdu + " " + iv.value);
        });
    }
    catch (Exception e)
    {
        Log(srv.name + "- " + "Error scanning tag: " + tag.Name);
        Log(srv.name + "- " + e);
    }
}
*/