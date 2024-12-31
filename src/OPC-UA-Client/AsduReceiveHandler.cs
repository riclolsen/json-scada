/* 
* OPC-UA Client Protocol driver for {json:scada}
* {json:scada} - Copyright (c) 2020-2022 - Ricardo L. Olsen
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
using System.IO;
using System.Linq;
using System.Diagnostics;

namespace OPCUAClientDriver
{
    partial class MainClass
    {
        public static JsonSerializerOptions jsonSerOpts = new JsonSerializerOptions
        {
            NumberHandling = JsonNumberHandling.AllowReadingFromString | JsonNumberHandling.WriteAsString
        };
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
            ErrorInvalidCommandLine = 0x100,
            ErrorClient = 0x200,
        };
        public class NodeDetails
        {
            public string DisplayName;
            public string BrowseName;
            public string ParentName;
            public string Path;
        }
        public class OPCUAClient
        {
            public bool failed = false;
            const int ReconnectPeriod = 10;
            public ISession session;
            SessionReconnectHandler reconnectHandler;
            int conn_number = 0;
            string conn_name;
            int clientRunTime = Timeout.Infinite;
            bool autoAccept = true;
            static ExitCode exitCode;
            List<MonitoredItem> ListMon = new List<MonitoredItem>();
            HashSet<string> NodeIds = new HashSet<string>();
            HashSet<string> NodeIdsFromObjects = new HashSet<string>();
            Dictionary<string, NodeDetails> NodeIdsDetails = new Dictionary<string, NodeDetails>();
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
                    Log(conn_name + " - " + "Exception in ConsoleClient: " + ex.Message);
                    exitCode = ExitCode.ErrorClient;
                    failed = true;
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
                failed = false;
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
                    if (!File.Exists(OPCUA_conn.configFileName))
                    {
                        if (File.Exists(Path.Join("..", "conf", "Opc.Ua.DefaultClient.Config.xml")))
                            OPCUA_conn.configFileName = Path.Join("..", "conf", "Opc.Ua.DefaultClient.Config.xml");
                        else
                        if (File.Exists(Path.Combine("\\", "json-scada", "conf", "Opc.Ua.DefaultClient.Config.xml")))
                            OPCUA_conn.configFileName = Path.Combine("\\", "json-scada", "conf", "Opc.Ua.DefaultClient.Config.xml");
                    }
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

                if (OPCUA_conn.useSecurity && config == null)
                {
                    Log(conn_name + " - " + "FATAL: error in XML config file!", LogLevelNoLog);
                    Environment.Exit(1);
                }

                if (config == null)
                {
                    config = new ApplicationConfiguration
                    {
                        ApplicationUri = "urn:localhost:OPCUA:JSON_SCADA_OPCUAClient",
                        ApplicationName = "JSON-SCADA OPC-UA Client",
                        ApplicationType = ApplicationType.Client,
                        CertificateValidator = new CertificateValidator(),
                        ServerConfiguration = null,
                        SecurityConfiguration = new SecurityConfiguration
                        {
                            AutoAcceptUntrustedCertificates = true,
                        },
                        TransportQuotas = new TransportQuotas
                        {
                            OperationTimeout = 600000,
                            MaxStringLength = 1048576,
                            MaxByteStringLength = 1048576,
                            MaxArrayLength = 65535,
                            MaxMessageSize = 4194304,
                            MaxBufferSize = 65535,
                            ChannelLifetime = 600000,
                            SecurityTokenLifetime = 3600000,
                        },
                        ClientConfiguration = new ClientConfiguration
                        {
                            DefaultSessionTimeout = 60000,
                            MinSubscriptionLifetime = 10000,
                        },
                        DisableHiResClock = true,
                    };
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
                    failed = true;
                    exitCode = ExitCode.ErrorCreateSession;
                    return;
                }

                if (OPCUA_conn.autoCreateTags)
                {
                    Log(conn_name + " - " + "Browsing the OPC UA server namespace.");
                    exitCode = ExitCode.ErrorBrowseNamespace;

                    var uac = new UAClient(session);
                    var refDescr =
                                        await BrowseFullAddressSpaceAsync(uac, Objects.ObjectsFolder).ConfigureAwait(false);
                    //var variableIds = new NodeIdCollection(referenceDescriptions
                    //    .Where(r => r.NodeClass == NodeClass.Variable && r.TypeDefinition.NamespaceIndex != 0)
                    //    .Select(r => ExpandedNodeId.ToNodeId(r.NodeId, session.NamespaceUris)));
                    foreach (var (reference, path) in refDescr.Values)
                    {
                        m_output.WriteLine("NodeId {0} {1} {2} Path: {3}", reference.NodeId, reference.NodeClass, reference.BrowseName, path);
                        ListMon.Add(new MonitoredItem()
                        {
                            DisplayName = reference.BrowseName.Name,
                            StartNodeId = reference.NodeId.ToString(),
                            SamplingInterval = System.Convert.ToInt32(System.Convert.ToDouble(OPCUA_conn.autoCreateTagSamplingInterval) * 1000),
                            QueueSize = System.Convert.ToUInt32(OPCUA_conn.autoCreateTagQueueSize),
                            MonitoringMode = MonitoringMode.Reporting,
                            DiscardOldest = true,
                            AttributeId = Attributes.Value
                        });
                        NodeIdsDetails[reference.NodeId.ToString()] = new NodeDetails
                        {
                            BrowseName = reference.BrowseName.Name,
                            DisplayName = reference.DisplayName.Text,
                            ParentName = Path.GetFileNameWithoutExtension(path),
                            Path = path,
                        };
                    }
                    Log($"{conn_name} - Found {refDescr.Count} objects and variables.");
                    // await FindObjects(session, ObjectIds.ObjectsFolder, "");

                    await Task.Delay(50);
                    Log(conn_name + " - " + "Add a list of items (server current time and status) to the subscription.");
                    exitCode = ExitCode.ErrorMonitoredItem;
                    ListMon.ForEach(i => i.Notification += OnNotification);
                    //OPCUA_conn.connection.session.Notification += OnSessionNotification;
                    ListMon.ForEach(i => i.SamplingInterval = System.Convert.ToInt32(System.Convert.ToDouble(OPCUA_conn.autoCreateTagSamplingInterval) * 1000));
                    ListMon.ForEach(i => i.QueueSize = System.Convert.ToUInt32(System.Convert.ToDouble(OPCUA_conn.autoCreateTagQueueSize)));
                    Log(conn_name + " - " + ListMon.Count + " Objects found");

                    Log(conn_name + " - " + "Create a subscription with publishing interval of " + System.Convert.ToDouble(OPCUA_conn.autoCreateTagPublishingInterval) + " seconds");
                    exitCode = ExitCode.ErrorCreateSubscription;
                    var subscription =
                        new Subscription(session.DefaultSubscription)
                        {
                            PublishingInterval = System.Convert.ToInt32(System.Convert.ToDouble(OPCUA_conn.autoCreateTagPublishingInterval) * 1000),
                            PublishingEnabled = true,
                            TimestampsToReturn = TimestampsToReturn.Both,
                            // MaxNotificationsPerPublish = 1,
                            SequentialPublishing = false,
                        };

                    await Task.Delay(50);
                    subscription.AddItems(ListMon);

                    await Task.Delay(50);
                    Log(conn_name + " - " + "Add the subscription to the session.");
                    Log(conn_name + " - " + subscription.MonitoredItemCount + " Monitored items");
                    exitCode = ExitCode.ErrorAddSubscription;
                    try
                    {
                        session.AddSubscription(subscription);
                        subscription.Create();
                        subscription.ApplyChanges();
                    }
                    catch (Exception e)
                    {
                        Log(conn_name + " - Error creating subscription: " + e.Message);
                    }
                }
                else
                {
                    Log(conn_name + " - " + "Create subscription for inserted tags.");
                    exitCode = ExitCode.ErrorBrowseNamespace;

                    foreach (var sub in OPCUA_conn.OpcSubscriptions)
                    {
                        List<MonitoredItem> lm = new List<MonitoredItem>();
                        foreach (var tm in sub.Value)
                        {
                            lm.Add(new MonitoredItem()
                            {
                                DisplayName = tm.ungroupedDescription,
                                StartNodeId = tm.protocolSourceObjectAddress,
                                SamplingInterval = (int)(tm.protocolSourceSamplingInterval * 1000),
                                QueueSize = (uint)OPCUA_conn.autoCreateTagQueueSize,
                                MonitoringMode = MonitoringMode.Reporting,
                                DiscardOldest = true,
                                AttributeId = Attributes.Value,
                            });
                        }
                        lm.ForEach(i => i.Notification += OnNotification);

                        Log(conn_name + " - " + "Create a subscription with publishing interval of " + sub.Key + " seconds");
                        exitCode = ExitCode.ErrorCreateSubscription;
                        var subscription =
                            new Subscription(session.DefaultSubscription)
                            {
                                PublishingInterval = (int)(sub.Key * 1000),
                                PublishingEnabled = true,
                                TimestampsToReturn = TimestampsToReturn.Both,
                                // MaxNotificationsPerPublish = 1,
                                SequentialPublishing = false,
                            };

                        await Task.Delay(50);
                        subscription.AddItems(lm);

                        await Task.Delay(50);
                        Log(conn_name + " - " + "Add the subscription to the session.");
                        Log(conn_name + " - " + subscription.MonitoredItemCount + " Monitored items");
                        exitCode = ExitCode.ErrorAddSubscription;
                        try
                        {
                            session.AddSubscription(subscription);
                            subscription.Create();
                            subscription.ApplyChanges();
                        }
                        catch (Exception e)
                        {
                            Log(conn_name + " - Error creating subscription: " + e.Message);
                        }
                    }
                }

                Log(conn_name + " - " + "Running...");
                exitCode = ExitCode.ErrorRunning;
            }
            private async Task FindObjects(Opc.Ua.Client.ISession session, NodeId nodeid, string path)
            {
                if (session == null)
                    return;

                try
                {
                    ReferenceDescriptionCollection references;
                    Byte[] continuationPoint;

                    if (NodeIdsFromObjects.Contains(nodeid.ToString()))
                        return;

                    var parentNode = await session.ReadNodeAsync(nodeid);
                    path += "/" + parentNode.BrowseName.Name;
                    Log(conn_name + " - Browsing object: " + path + ", " + nodeid.ToString() + ", " + parentNode.BrowseName.Name);
                    if (path.StartsWith("/Objects/DeviceSet/BottleFiller/Admin"))
                    {
                        Log(conn_name + " - Browsing object: " + path + ", " + nodeid.ToString() + ", " + parentNode.BrowseName.Name);
                    }
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

                    Log(conn_name + " - Found " + references.Count.ToString() + " references on object " + nodeid.ToString());

                    if (continuationPoint != null && continuationPoint.Length > 0)
                    {
                        Console.WriteLine(continuationPoint);
                    }

                    foreach (var rd in references)
                    {
                        Log(conn_name + " - " + path + ", " + rd.NodeId + ", " + rd.DisplayName + ", " + rd.BrowseName + ", " + rd.NodeClass);
                        if (rd.NodeClass == NodeClass.Method) continue;
                        if (rd.NodeClass == NodeClass.Variable && !NodeIds.Contains(rd.NodeId.ToString()))
                        {
                            // var resp = await session.ReadNodeAsync(rd.NodeId.ToString());

                            NodeIdsDetails[rd.NodeId.ToString()] = new NodeDetails
                            {
                                BrowseName = rd.BrowseName.Name,
                                DisplayName = rd.DisplayName.Text,
                                ParentName = parentNode.BrowseName.Name,
                                Path = path,
                            };
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

                            //NodeIdsFromObjects.Add(nodeid.ToString());
                            //await FindObjects(session, ExpandedNodeId.ToNodeId(rd.NodeId, session.NamespaceUris), path);
                        }
                        else
                        if (rd.NodeClass == NodeClass.Object)
                        {
                            NodeIdsFromObjects.Add(nodeid.ToString());
                            await FindObjects(session, ExpandedNodeId.ToNodeId(rd.NodeId, session.NamespaceUris), path);
                        }
                    }
                }
                catch (Exception ex)
                {
                    Log(conn_name + " - FindObjects - " + ex.Message);
                }
            }
            private void Client_KeepAlive(ISession sender, KeepAliveEventArgs e)
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
                //MonitoredItemNotification notification = e.N
                //otificationValue as MonitoredItemNotification;
                //Console.WriteLine("Notification Received for Variable \"{0}\" and Value = {1} type {2}.", item.DisplayName, notification.Value, notification.TypeId);

                foreach (var efl in item.DequeueEvents())
                {
                    // Log(efl.ToString());
                }
                foreach (var value in item.DequeueValues())
                {
                    //if (value == null || value.Value == null)
                    //{
                    //MonitoredItemNotification notification = e.NotificationValue as MonitoredItemNotification;
                    //Console.WriteLine("Notification Received for Variable \"{0}\" value=null type {1}.", item.DisplayName, notification.TypeId);
                    //}

                    if (value != null)
                    {
                        string tp = "unknown";
                        bool isArray = false;

                        try
                        {
                            if (value.Value != null)
                            {
                                CntNotificEvents++;

                                Double dblValue = 0.0;
                                string strValue = "";
                                string jsonValue = JsonSerializer.Serialize(value.Value, jsonSerOpts);

                                if (value.WrappedValue.TypeInfo != null)
                                {
                                    tp = value.WrappedValue.TypeInfo.BuiltInType.ToString();
                                    isArray = value.Value.GetType().ToString().Contains("[");
                                    // Log(conn_name + " - " + item.ResolvedNodeId + "TYPE: " + tp, LogLevelDetailed);
                                }
                                else
                                {
                                    Log(conn_name + " - " + item.ResolvedNodeId + " TYPE: ?????", LogLevelDetailed);
                                }

                                if (LogLevel >= LogLevelDetailed)
                                    Log(conn_name + " - " + item.ResolvedNodeId + " " + item.DisplayName + " " + value.Value + " " + value.SourceTimestamp + " " + value.StatusCode, LogLevelDetailed);

                                try
                                {
                                    if (tp == "Variant" && !isArray)
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
                                                        strValue = JsonSerializer.Serialize(value.Value, jsonSerOpts);
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
                                    if ((tp == "DateTime" || tp == "UtcTime") && !isArray)
                                    {
                                        dblValue = ((DateTimeOffset)System.Convert.ToDateTime(value.Value)).ToUnixTimeMilliseconds();
                                        strValue = System.Convert.ToDateTime(value.Value).ToString("o");
                                    }
                                    else
                                    if (tp == "XmlElement" && !isArray)
                                    {
                                        dblValue = 0;
                                        strValue = value.WrappedValue.ToString();
                                        jsonValue = "\"" + value.WrappedValue.ToString() + "\"";
                                    }
                                    else
                                    if (tp == "ByteString")
                                    {
                                        dblValue = 0;
                                        strValue = System.Convert.ToBase64String(value.GetValue<byte[]>([]));
                                    }
                                    else
                                    if (
                                        (tp == "String" ||
                                        tp == "LocaleId" ||
                                        tp == "LocalizedText" ||
                                        tp == "NodeId" ||
                                        tp == "ExpandedNodeId" ||
                                        tp == "XmlElement" ||
                                        tp == "QualifiedName" ||
                                        tp == "Guid")
                                        && isArray
                                        )
                                    {
                                        dblValue = 0;
                                        strValue = jsonValue;
                                    }
                                    else
                                    if (tp == "String" ||
                                        tp == "LocaleId" ||
                                        tp == "LocalizedText" ||
                                        tp == "NodeId" ||
                                        tp == "ExpandedNodeId" ||
                                        tp == "XmlElement" ||
                                        tp == "QualifiedName" ||
                                        tp == "Guid" ||
                                        // tp == "ExtensionObject" ||
                                        isArray)
                                    {
                                        dblValue = 0;
                                        strValue = System.Convert.ToString(value.Value);
                                    }
                                    else
                                    {
                                        try
                                        {
                                            dblValue = System.Convert.ToDouble(value.Value);
                                            strValue = value.Value.ToString();
                                        }
                                        catch
                                        {
                                            dblValue = 0;
                                            strValue = value.Value.ToString();
                                        }
                                    }
                                }
                                catch
                                {
                                    dblValue = 0;
                                    strValue = value.Value.ToString();
                                }

                                var parentName = "";
                                var path = "";
                                var details = NodeIdsDetails[item.ResolvedNodeId.ToString()];
                                if (details != null)
                                {
                                    parentName = details.ParentName;
                                    path = details.Path;
                                }
                                OPC_Value iv =
                                    new OPC_Value()
                                    {
                                        valueJson = jsonValue,
                                        selfPublish = true,
                                        address = item.ResolvedNodeId.ToString(),
                                        isArray = isArray,
                                        asdu = tp,
                                        value = dblValue,
                                        valueString = strValue,
                                        hasSourceTimestamp = value.SourceTimestamp != DateTime.MinValue,
                                        sourceTimestamp = value.SourceTimestamp,
                                        serverTimestamp = DateTime.Now,
                                        quality = StatusCode.IsGood(value.StatusCode),
                                        cot = 3,
                                        conn_number = conn_number,
                                        conn_name = conn_name,
                                        common_address = "",
                                        display_name = item.DisplayName,
                                        parentName = parentName,
                                        path = path,
                                    };
                                if (OPCDataQueue.Count < DataBufferLimit)
                                    OPCDataQueue.Enqueue(iv);
                                else
                                    CntLostDataUpdates++;
                                //if (item.ResolvedNodeId.ToString().ToLower().Contains("byteswritten"))
                                //{
                                //    MonitoredItemNotification notification = e.NotificationValue as MonitoredItemNotification;
                                //    Console.WriteLine("Notification Received for Variable \"{0}\" and Value = {1} type {2}.", item.DisplayName, notification.Value, notification.TypeId);
                                //    Console.WriteLine($"----------------------- {conn_name} {iv.address} {iv.value} {iv.sourceTimestamp}");
                                //}
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

            // not used yet (session events)
            private void OnSessionNotification(ISession session, NotificationEventArgs e)
            {
                var notificationMsg = e.NotificationMessage;
                Console.WriteLine(conn_name + " - Notification Received Value = {0} type {1}.", notificationMsg.NotificationData, notificationMsg.TypeId);

                int count = 0;

                for (int ii = 0; ii < e.NotificationMessage.NotificationData.Count; ii++)
                {
                    DataChangeNotification notification = e.NotificationMessage.NotificationData[ii].Body as DataChangeNotification;

                    if (notification == null)
                    {
                        continue;
                    }

                    for (int jj = 0; jj < notification.MonitoredItems.Count; jj++)
                    {
                        CntNotificEvents++;
                        count++;
                        var value = notification.MonitoredItems[jj].Value;
                    }
                }

                // ReportMessage("OnDataChange. Time={0} ({3}), Count={1}/{2}", DateTime.UtcNow.ToString("mm:ss.fff"), count, m_totalItemUpdateCount, (m_lastMessageTime - m_firstMessageTime).TotalMilliseconds);
            }

            public interface IUAClient
            {
                /// <summary>
                /// The session to use.
                /// </summary>
                ISession Session { get; }
            }

            public class UAClient : IUAClient
            {
                public ISession Session { get; private set; }

                public UAClient(ISession session)
                {
                    Session = session;
                }
            }

            private static BrowseDescriptionCollection CreateBrowseDescriptionCollectionFromNodeId(
                   NodeIdCollection nodeIdCollection,
                   BrowseDescription template)
            {
                var browseDescriptionCollection = new BrowseDescriptionCollection();
                foreach (var nodeId in nodeIdCollection)
                {
                    BrowseDescription browseDescription = (BrowseDescription)template.MemberwiseClone();
                    browseDescription.NodeId = nodeId;
                    browseDescriptionCollection.Add(browseDescription);
                }
                return browseDescriptionCollection;

            }
            const int kMaxSearchDepth = 128;
            private readonly TextWriter m_output = Console.Out;
            private readonly ManualResetEvent m_quitEvent;
            private readonly bool m_verbose = false;
            private static ByteStringCollection PrepareBrowseNext(BrowseResultCollection browseResultCollection)
            {
                var continuationPoints = new ByteStringCollection();
                foreach (var browseResult in browseResultCollection)
                {
                    if (browseResult.ContinuationPoint != null)
                    {
                        continuationPoints.Add(browseResult.ContinuationPoint);
                    }
                }
                return continuationPoints;
            }
            public async Task<Dictionary<ExpandedNodeId, (ReferenceDescription Reference, string Path)>> BrowseFullAddressSpaceAsync(
                IUAClient uaClient,
                NodeId startingNode = null,
                BrowseDescription browseDescription = null,
                CancellationToken ct = default)
            {
                Stopwatch stopWatch = new Stopwatch();
                stopWatch.Start();

                // Browse template
                const int kMaxReferencesPerNode = 1000;
                var browseTemplate = browseDescription ?? new BrowseDescription
                {
                    NodeId = startingNode ?? ObjectIds.RootFolder,
                    BrowseDirection = BrowseDirection.Forward,
                    ReferenceTypeId = ReferenceTypeIds.HierarchicalReferences,
                    IncludeSubtypes = true,
                    NodeClassMask = (uint)NodeClass.Variable | (uint)NodeClass.Object,
                    ResultMask = (uint)BrowseResultMask.All
                };
                var browseDescriptionCollection = CreateBrowseDescriptionCollectionFromNodeId(
                    new NodeIdCollection(new NodeId[] { startingNode ?? ObjectIds.RootFolder }),
                    browseTemplate);

                // Browse
                var referenceDescriptions = new Dictionary<ExpandedNodeId, (ReferenceDescription Reference, string Path)>();
                var startingNodeName = (await uaClient.Session.ReadNodeAsync(startingNode ?? ObjectIds.RootFolder)).BrowseName.Name;
                var rootPath = startingNodeName;

                int searchDepth = 0;
                uint maxNodesPerBrowse = uaClient.Session.OperationLimits.MaxNodesPerBrowse;
                while (browseDescriptionCollection.Any() && searchDepth < kMaxSearchDepth)
                {
                    searchDepth++;
                    Utils.LogInfo("{0}: Browse {1} nodes after {2}ms",
                        searchDepth, browseDescriptionCollection.Count, stopWatch.ElapsedMilliseconds);

                    BrowseResultCollection allBrowseResults = new BrowseResultCollection();
                    bool repeatBrowse;
                    BrowseResultCollection browseResultCollection = new BrowseResultCollection();
                    BrowseDescriptionCollection unprocessedOperations = new BrowseDescriptionCollection();
                    DiagnosticInfoCollection diagnosticsInfoCollection;
                    BrowseDescriptionCollection browseCollection = new BrowseDescriptionCollection();
                    do
                    {
                        if (m_quitEvent?.WaitOne(0) == true)
                        {
                            m_output.WriteLine("Browse aborted.");
                            break;
                        }

                        browseCollection = (maxNodesPerBrowse == 0) ?
                            browseDescriptionCollection :
                            browseDescriptionCollection.Take((int)maxNodesPerBrowse).ToArray();
                        repeatBrowse = false;
                        try
                        {
                            var browseResponse = await uaClient.Session.BrowseAsync(null, null,
                                kMaxReferencesPerNode, browseCollection, ct).ConfigureAwait(false);
                            browseResultCollection = browseResponse.Results;
                            diagnosticsInfoCollection = browseResponse.DiagnosticInfos;
                            ClientBase.ValidateResponse(browseResultCollection, browseCollection);
                            ClientBase.ValidateDiagnosticInfos(diagnosticsInfoCollection, browseCollection);

                            // separate unprocessed nodes for later
                            int ii = 0;
                            foreach (BrowseResult browseResult in browseResultCollection)
                            {
                                // check for error.
                                StatusCode statusCode = browseResult.StatusCode;
                                if (StatusCode.IsBad(statusCode))
                                {
                                    // this error indicates that the server does not have enough simultaneously active 
                                    // continuation points. This request will need to be resent after the other operations
                                    // have been completed and their continuation points released.
                                    if (statusCode == StatusCodes.BadNoContinuationPoints)
                                    {
                                        unprocessedOperations.Add(browseCollection[ii++]);
                                        continue;
                                    }
                                }

                                // save results.
                                allBrowseResults.Add(browseResult);
                                ii++;
                            }
                        }
                        catch (ServiceResultException sre)
                        {
                            if (sre.StatusCode == StatusCodes.BadEncodingLimitsExceeded ||
                                sre.StatusCode == StatusCodes.BadResponseTooLarge)
                            {
                                // try to address by overriding operation limit
                                maxNodesPerBrowse = maxNodesPerBrowse == 0 ?
                                    (uint)browseCollection.Count / 2 : maxNodesPerBrowse / 2;
                                repeatBrowse = true;
                            }
                            else
                            {
                                m_output.WriteLine("Browse error: {0}", sre.Message);
                                throw;
                            }
                        }
                    } while (repeatBrowse);

                    if (maxNodesPerBrowse == 0)
                    {
                        browseDescriptionCollection.Clear();
                    }
                    else
                    {
                        browseDescriptionCollection = browseDescriptionCollection.Skip(browseResultCollection.Count).ToArray();
                    }

                    // Browse next
                    var continuationPoints = PrepareBrowseNext(browseResultCollection);
                    while (continuationPoints.Any())
                    {
                        if (m_quitEvent?.WaitOne(0) == true)
                        {
                            m_output.WriteLine("Browse aborted.");
                        }

                        Utils.LogInfo("BrowseNext {0} continuation points.", continuationPoints.Count);
                        var browseNextResult = await uaClient.Session.BrowseNextAsync(null, false, continuationPoints, ct).ConfigureAwait(false);
                        var browseNextResultCollection = browseNextResult.Results;
                        diagnosticsInfoCollection = browseNextResult.DiagnosticInfos;
                        ClientBase.ValidateResponse(browseNextResultCollection, continuationPoints);
                        ClientBase.ValidateDiagnosticInfos(diagnosticsInfoCollection, continuationPoints);
                        allBrowseResults.AddRange(browseNextResultCollection);
                        continuationPoints = PrepareBrowseNext(browseNextResultCollection);
                    }

                    // Build browse request for next level
                    var browseTable = new NodeIdCollection();
                    int duplicates = 0;
                    /*
                    foreach (var browseResult in allBrowseResults)
                    {
                        foreach (ReferenceDescription reference in browseResult.References)
                        {
                            if (!referenceDescriptions.ContainsKey(reference.NodeId))
                            {
                                referenceDescriptions[reference.NodeId] = reference;
                                if (reference.ReferenceTypeId != ReferenceTypeIds.HasProperty)
                                {
                                    browseTable.Add(ExpandedNodeId.ToNodeId(reference.NodeId, uaClient.Session.NamespaceUris));
                                }
                            }
                            else
                            {
                                duplicates++;
                            }
                        }
                    }
                    */

                    // Process each browse result to build paths
                    for (int i = 0; i < browseCollection.Count; i++)
                    {
                        var parentNodeId = browseCollection[i].NodeId;
                        var parentPath = "/" + rootPath;

                        // Try to get parent path from existing references
                        var parentExpandedNodeId = new ExpandedNodeId(parentNodeId);
                        if (referenceDescriptions.TryGetValue(parentExpandedNodeId, out var parentRef))
                        {
                            parentPath = parentRef.Path;
                        }

                        if (i < browseResultCollection.Count && browseResultCollection[i].References != null)
                        {
                            foreach (ReferenceDescription reference in browseResultCollection[i].References)
                            {
                                // Build complete path by appending this node's browse name to its parent's path
                                var newPath = parentPath.TrimEnd('/') + "/" + reference.BrowseName;

                                if (!referenceDescriptions.ContainsKey(reference.NodeId))
                                {
                                    referenceDescriptions[reference.NodeId] = (reference, newPath);
                                    if (reference.ReferenceTypeId != ReferenceTypeIds.HasProperty)
                                    {
                                        browseTable.Add(ExpandedNodeId.ToNodeId(reference.NodeId, uaClient.Session.NamespaceUris));
                                    }
                                }
                                else
                                {
                                    duplicates++;
                                }
                            }
                        }
                  
                    }
                          
                    
                    if (duplicates > 0)
                    {
                        Utils.LogInfo("Browse Result {0} duplicate nodes were ignored.", duplicates);
                    }
                    browseDescriptionCollection.AddRange(CreateBrowseDescriptionCollectionFromNodeId(browseTable, browseTemplate));

                    // add unprocessed nodes if any
                    browseDescriptionCollection.AddRange(unprocessedOperations);
                }
                stopWatch.Stop();

                var result = new ReferenceDescriptionCollection(referenceDescriptions.Values.Select(v => v.Reference));
                result.Sort((x, y) => (x.NodeId.CompareTo(y.NodeId)));

                m_output.WriteLine("BrowseFullAddressSpace found {0} references on server in {1}ms.",
                    referenceDescriptions.Count, stopWatch.ElapsedMilliseconds);

                if (m_verbose)
                {
                    foreach (var (reference, path) in referenceDescriptions.Values)
                    {
                        m_output.WriteLine("NodeId {0} {1} {2} Path: {3}", reference.NodeId, reference.NodeClass, reference.BrowseName, path);
                    }
                }

                return referenceDescriptions;
            }
        }
    }
}