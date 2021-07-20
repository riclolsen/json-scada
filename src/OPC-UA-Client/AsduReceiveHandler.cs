/* 
 * OPC-UA Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020-2021 - Ricardo L. Olsen
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
using Opc.Ua;
using Opc.Ua.Client;
using Opc.Ua.Configuration;
using System.Collections.Generic;
using System.Threading;
using System.Threading.Tasks;
using System.Text.Json;
using System.Text.Json.Serialization;

namespace OPCUAClientDriver
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
        public class OPCUAClient
        {
            const int ReconnectPeriod = 10;
            Session session;
            SessionReconnectHandler reconnectHandler;
            int conn_number = 0;
            string conn_name;
            int clientRunTime = Timeout.Infinite;
            bool autoAccept = true;
            static ExitCode exitCode;
            List<MonitoredItem> ListMon = new List<MonitoredItem>();
            HashSet<string> NodeIds = new HashSet<string>();
            HashSet<string> NodeIdsFromObjects = new HashSet<string>();
            OPCUA_connection OPCUA_conn;

            public OPCUAClient(OPCUA_connection _OPCUA_conn)
            {
                OPCUA_conn = _OPCUA_conn;
                conn_name = OPCUA_conn.name;
                conn_number = OPCUA_conn.protocolConnectionNumber;
            }

            public void Run()
            {
                try
                {
                    ConsoleClient().Wait();
                }
                catch (Exception ex)
                {
                    // Utils.Trace("ServiceResultException:" + ex.Message);
                    Log(conn_name + " - " + "Exception: " + ex.Message);
                    return;
                }

                ManualResetEvent quitEvent = new ManualResetEvent(false);

                try
                {
                    Console.CancelKeyPress += (sender, eArgs) =>
                    {
                        quitEvent.Set();
                        eArgs.Cancel = true;
                    };
                }
                catch
                {
                }

                // wait for timeout or Ctrl-C
                quitEvent.WaitOne(clientRunTime);

                // return error conditions
                if (session.KeepAliveStopped)
                {
                    exitCode = ExitCode.ErrorNoKeepAlive;
                    return;
                }

                exitCode = ExitCode.Ok;
            }

            public static ExitCode ExitCode { get => exitCode; }

            private async Task ConsoleClient()
            {
                Log(conn_name + " - " + "Create an Application Configuration...");
                exitCode = ExitCode.ErrorCreateApplication;

                ApplicationInstance application = new ApplicationInstance
                {
                    ApplicationName = "JSON-SCADA OPC-UA Client",
                    ApplicationType = ApplicationType.Client,
                    ConfigSectionName = "",                    
                };

                bool haveAppCertificate = false;
                ApplicationConfiguration config = null;

                try
                {
                    // load the application configuration.
                    Log(conn_name + " - " + "Load config from " + OPCUA_conn.configFileName);
                    config = await application.LoadApplicationConfiguration(OPCUA_conn.configFileName, false);
                    // config.SecurityConfiguration.AutoAcceptUntrustedCertificates = true;

                    // check the application certificate.
                    haveAppCertificate = await application.CheckApplicationInstanceCertificate(false, 0);

                    if (!haveAppCertificate)
                    {
                        Log(conn_name + " - " + "FATAL: Application instance certificate invalid!", LogLevelNoLog);
                        Environment.Exit(1);
                    }

                    if (haveAppCertificate)
                    {
                        config.ApplicationUri = X509Utils.GetApplicationUriFromCertificate(config.SecurityConfiguration.ApplicationCertificate.Certificate);
                        if (config.SecurityConfiguration.AutoAcceptUntrustedCertificates)
                        {
                            autoAccept = true;
                        }
                        config.CertificateValidator.CertificateValidation += new CertificateValidationEventHandler(CertificateValidator_CertificateValidation);
                    }
                    else
                    {
                        Log(conn_name + " - " + "WARN: missing application certificate, using unsecure connection.");
                    }
                }
                catch (Exception e)
                {
                    Log(conn_name + " - WARN: " + e.Message);
                }

                if (config == null)
                {
                    Log(conn_name + " - " + "FATAL: error in XML config file!", LogLevelNoLog);
                    Environment.Exit(1);
                }

                try 
                {
                    Log(conn_name + " - " + "Discover endpoints of " + OPCUA_conn.endpointURLs[0]);
                    exitCode = ExitCode.ErrorDiscoverEndpoints;
                    var selectedEndpoint = CoreClientUtils.SelectEndpoint(OPCUA_conn.endpointURLs[0], haveAppCertificate && OPCUA_conn.useSecurity, 15000);
                    Log(conn_name + " - " + "Selected endpoint uses: " +
                        selectedEndpoint.SecurityPolicyUri.Substring(selectedEndpoint.SecurityPolicyUri.LastIndexOf('#') + 1));

                    Log(conn_name + " - " + "Create a session with OPC UA server.");
                    exitCode = ExitCode.ErrorCreateSession;
                    var endpointConfiguration = EndpointConfiguration.Create(config);
                    var endpoint = new ConfiguredEndpoint(null, selectedEndpoint, endpointConfiguration);

                    await Task.Delay(50);
                    session = await Session.Create(config, endpoint, false, "OPC UA Console Client", 60000, new UserIdentity(new AnonymousIdentityToken()), null);

                    // Log("" + session.KeepAliveInterval); // default is 5000
                    session.KeepAliveInterval = System.Convert.ToInt32(OPCUA_conn.timeoutMs);

                    // register keep alive handler
                    session.KeepAlive += Client_KeepAlive;
                }
                catch (Exception e)
                {
                    Log(conn_name + " - WARN: " + e.Message);
                }

                if (session == null)
                {
                    Log(conn_name + " - " + "FATAL: error creating session!", LogLevelNoLog);
                    Environment.Exit(1);
                }

                Log(conn_name + " - " + "Browsing the OPC UA server namespace.");
                exitCode = ExitCode.ErrorBrowseNamespace;

                await FindObjects(session, ObjectIds.ObjectsFolder);

                await Task.Delay(50);
                Log(conn_name + " - " + "Add a list of items (server current time and status) to the subscription.");
                exitCode = ExitCode.ErrorMonitoredItem;
                ListMon.ForEach(i => i.Notification += OnNotification);
                //ListMon.ForEach(i => i.SamplingInterval = System.Convert.ToInt32(System.Convert.ToDouble(OPCUA_conn.autoCreateTagSamplingInterval) * 1000);
                // ListMon.ForEach(i => Log(conn_name + " - " + i.DisplayName));
                Log(conn_name + " - " + ListMon.Count + " Objects found");

                Log(conn_name + " - " + "Create a subscription with publishing interval of " + System.Convert.ToDouble(OPCUA_conn.autoCreateTagPublishingInterval) + "seconds");
                exitCode = ExitCode.ErrorCreateSubscription;
                var subscription = 
                    new Subscription(session.DefaultSubscription) 
                    { 
                        PublishingInterval = System.Convert.ToInt32(System.Convert.ToDouble(OPCUA_conn.autoCreateTagPublishingInterval) *1000), 
                        PublishingEnabled = true 
                    };

                await Task.Delay(50);
                subscription.AddItems(ListMon);

                await Task.Delay(50);
                Log(conn_name + " - " + "Add the subscription to the session.");
                Log(conn_name + " - " + subscription.MonitoredItemCount + " Monitored items"); 
                exitCode = ExitCode.ErrorAddSubscription;
                session.AddSubscription(subscription);
                subscription.Create();

                subscription.ApplyChanges();

                Log(conn_name + " - " + "Running...");
                exitCode = ExitCode.ErrorRunning;
            }
            private async Task FindObjects(Opc.Ua.Client.Session session, NodeId nodeid)
            {
                if (session == null)
                    return;

                try
                {
                ReferenceDescriptionCollection references;
                Byte[] continuationPoint;

                if (NodeIdsFromObjects.Contains(nodeid.ToString()))
                     return;

                session.Browse(
                    null,
                    null,
                    nodeid,
                    0u,
                    BrowseDirection.Forward,
                    ReferenceTypeIds.HierarchicalReferences,
                    true,
                    (uint)NodeClass.Variable | (uint)NodeClass.Object,
                    out continuationPoint,
                    out references);

                foreach (var rd in references)
                {
                    
                    Log(conn_name + " - "  + rd.NodeId + ", " + rd.DisplayName + ", " + rd.BrowseName + ", " + rd.NodeClass);
                    if (rd.NodeClass == NodeClass.Variable && !NodeIds.Contains(rd.NodeId.ToString()))
                    {
                        NodeIds.Add(rd.NodeId.ToString());
                        ListMon.Add(
                        new MonitoredItem()
                        {
                            DisplayName = rd.DisplayName.ToString(),
                            StartNodeId = rd.NodeId.ToString(),
                            SamplingInterval = System.Convert.ToInt32(System.Convert.ToDouble(OPCUA_conn.autoCreateTagSamplingInterval) * 1000),
                            QueueSize = System.Convert.ToUInt32(OPCUA_conn.autoCreateTagQueueSize),
                            MonitoringMode = MonitoringMode.Reporting,
                            DiscardOldest = true,
                            AttributeId = Attributes.Value
                        });
                    }
                    else
                    if (rd.NodeClass == NodeClass.Object)
                        {
                            NodeIdsFromObjects.Add(nodeid.ToString());
                            await FindObjects(session, ExpandedNodeId.ToNodeId(rd.NodeId, session.NamespaceUris));
                            Thread.Yield();
                            //Thread.Sleep(1);
                            //await Task.Delay(1);
                        }                            
                    }
                }
                catch (Exception ex)
                {
                    Log(conn_name + " - " + ex.Message);
                }
            }
            private void Client_KeepAlive(Session sender, KeepAliveEventArgs e)
            {
                if (e.Status != null && ServiceResult.IsNotGood(e.Status))
                {
                    Log(conn_name + " - " + e.Status + "," + sender.OutstandingRequestCount + ", " + sender.DefunctRequestCount);

                    if (reconnectHandler == null)
                    {
                        Log(conn_name + " - " + "--- RECONNECTING ---");
                        reconnectHandler = new SessionReconnectHandler();
                        reconnectHandler.BeginReconnect(sender, ReconnectPeriod * 1000, Client_ReconnectComplete);
                    }
                }
            }

            private void Client_ReconnectComplete(object sender, EventArgs e)
            {
                // ignore callbacks from discarded objects.
                if (!Object.ReferenceEquals(sender, reconnectHandler))
                {
                    return;
                }

                session = reconnectHandler.Session;
                reconnectHandler.Dispose();
                reconnectHandler = null;

                Log(conn_name + " - " + "--- RECONNECTED ---");
            }

            private void OnNotification(MonitoredItem item, MonitoredItemNotificationEventArgs e)
            {

                //MonitoredItemNotification notification = e.NotificationValue as MonitoredItemNotification;
                //Console.WriteLine("Notification Received for Variable \"{0}\" and Value = {1} type {2}.", item.DisplayName, notification.Value, notification.TypeId);

                foreach (var value in item.DequeueValues())
                {
                    if (value != null)
                    {
                        string tp = "unknown";

                        try
                        {

                            if (value.WrappedValue.TypeInfo != null)
                            {
                                tp = value.WrappedValue.TypeInfo.BuiltInType.ToString();
                                // Log(conn_name + " - " + item.ResolvedNodeId + "TYPE: " + tp, LogLevelDetailed);
                            }
                            else
                            {
                                Log(conn_name + " - " + item.ResolvedNodeId + " TYPE: ?????", LogLevelDetailed);
                            }

                            Log(conn_name + " - " + item.ResolvedNodeId + " " + item.DisplayName + " " + value.Value + " " + value.SourceTimestamp + " " + value.StatusCode, LogLevelDetailed);

                            if (value.Value != null)
                            {
                                Double dblValue = 0.0;
                                string strValue = "";

                                try
                                {
                                  if (tp == "Variant")
                                    {
                                        try
                                        {
                                            dblValue = System.Convert.ToDouble(value.Value);
                                        }
                                        catch
                                        {
                                            try
                                            {
                                                dblValue = System.Convert.ToInt64(value.Value);
                                            }
                                            catch
                                            {
                                                try
                                                {
                                                    dblValue = System.Convert.ToInt32(value.Value);
                                                }
                                                catch
                                                {
                                                    dblValue = 0;
                                                    try
                                                    {
                                                        var opt = new JsonSerializerOptions
                                                        {
                                                            NumberHandling = JsonNumberHandling.AllowReadingFromString | JsonNumberHandling.WriteAsString
                                                        };
                                                        strValue = JsonSerializer.Serialize(value.Value, opt);
                                                    }
                                                    catch
                                                    {
                                                        strValue = value.Value.ToString();
                                                    }                                                        
                                                }
                                            }                                            
                                        }
                                    }
                                  else                                        
                                  if (tp == "DateTime") 
                                    {                                    
                                    dblValue = ((DateTimeOffset)System.Convert.ToDateTime(value.Value)).ToUnixTimeMilliseconds();
                                    strValue = System.Convert.ToDateTime(value.Value).ToString("o");
                                    }
                                else
                                    {
                                    dblValue = System.Convert.ToDouble(value.Value);
                                    strValue = value.Value.ToString();
                                    }
                                }
                                catch (Exception excpt)
                                {
                                strValue = value.Value.ToString();
                                }

                                var options = new JsonSerializerOptions
                                {
                                    NumberHandling = JsonNumberHandling.AllowReadingFromString | JsonNumberHandling.WriteAsString
                                };

                                OPC_Value iv =
                                    new OPC_Value()
                                    {
                                        valueJson = JsonSerializer.Serialize(value, options),
                                        selfPublish = true,
                                        address = item.ResolvedNodeId.ToString(),
                                        asdu = tp,
                                        isDigital = true,
                                        value = dblValue,
                                        valueString = strValue,
                                        hasSourceTimestamp = value.SourceTimestamp!=DateTime.MinValue,
                                        sourceTimestamp = value.SourceTimestamp,
                                        serverTimestamp = DateTime.Now,
                                        quality = StatusCode.IsGood(value.StatusCode),
                                        cot = 3,
                                        conn_number = conn_number,
                                        conn_name = conn_name,
                                        common_address = "",
                                        display_name = item.DisplayName
                                    };
                                OPCDataQueue.Enqueue(iv);
                            }
                        }
                        catch (Exception excpt)
                        {
                            Log(conn_name + " - " + excpt.Message);
                            Log(conn_name + " - " + "TYPE:" + tp);
                            Log(conn_name + " - " + item.ResolvedNodeId + " " + item.DisplayName + " " + value.Value + " " + value.SourceTimestamp + " " + value.StatusCode);
                        }
                    }
                    else
                    {
                        Log(conn_name + " - " + item.ResolvedNodeId + " " + item.DisplayName + " NULL VALUE!", LogLevelDetailed);
                    }
                    
                    Thread.Yield();
                    Thread.Sleep(1);
                    //if ((OPCDataQueue.Count % 50) == 0)
                    //{
                    //    await Task.Delay(200);
                    //}
                }

            }

            private void CertificateValidator_CertificateValidation(CertificateValidator validator, CertificateValidationEventArgs e)
            {
                if (e.Error.StatusCode == StatusCodes.BadCertificateUntrusted)
                {
                    e.Accept = autoAccept;
                    if (autoAccept)
                    {
                        Log(conn_name + " - " + "Accepted Certificate: " + e.Certificate.Subject);
                    }
                    else
                    {
                        Log(conn_name + " - " + "Rejected Certificate: " + e.Certificate.Subject);
                    }
                }
            }
        }
    }
}