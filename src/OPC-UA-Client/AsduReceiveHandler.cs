/* 
* OPC-UA Client Protocol driver for {json:scada}
* {json:scada} - Copyright (c) 2020-2025 - Ricardo L. Olsen
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
using System.Text.Json.Nodes;
using System.Text.RegularExpressions;
using System.IO;
using System.Linq;
using System.Diagnostics;

namespace OPCUAClientDriver
{
    partial class MainClass
    {
        public static JsonSerializerOptions jsonSerOpts = new JsonSerializerOptions
        {
            NumberHandling = JsonNumberHandling.AllowReadingFromString
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
            public ISession session;
            const int ReconnectPeriod = 10;
            SessionReconnectHandler reconnectHandler;
            int conn_number = 0;
            string conn_name;
            int clientRunTime = Timeout.Infinite;
            bool autoAccept = true;
            static ExitCode exitCode;
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
                    var refDescr = await BrowseFullAddressSpaceAsync(uac, Objects.ObjectsFolder).ConfigureAwait(false);
                    Regex regexp = new Regex("/Objects/");
                    IList<NodeId> nodesList = [];

                    foreach (var (reference, path) in refDescr.Values)
                    {
                        if (reference.NodeClass == NodeClass.Object) continue;
                        if (reference.NodeClass == NodeClass.Method && !OPCUA_conn.commandsEnabled) continue;
                        nodesList.Add(reference.NodeId.ToString());
                    }

                    const int maxNodesToRead = 100;
                    for (var j = 0; j < nodesList.Count; j = j + maxNodesToRead)
                    {
                        try
                        {
                            var nodesToRead = nodesList.Skip(j).Take(maxNodesToRead).ToList();
                            (IList<Node> sourceNodes, IList<ServiceResult> readErrors) = await session.ReadNodesAsync(nodesToRead);
                            DataValueCollection dataValues = []; IList<ServiceResult> readErrorsDv = [];
                            try
                            {
                                (dataValues, readErrorsDv) = await session.ReadValuesAsync(nodesToRead);
                            }
                            catch (Exception)
                            {
                                Log(conn_name + " - Error reading values " + j, LogLevelDetailed);
                            }
                            Log(conn_name + " - " + " Autotag - Read " + sourceNodes.Count + " nodes at offset " + j + " from a total of " + nodesList.Count);
                            for (int i = 0; i < sourceNodes.Count; i++)
                            {
                                if (OPCUA_conn.InsertedAddresses.Contains(sourceNodes[i].NodeId.ToString())) continue;
                                if (!StatusCode.IsGood(readErrors[i].StatusCode)) continue;
                                var reference = refDescr[sourceNodes[i].NodeId];
                                var pathMinusLastName = Path.GetDirectoryName(reference.Path).Replace('\\', '/');
                                var parentName = Path.GetFileName(pathMinusLastName);
                                var path = regexp.Replace(pathMinusLastName, "", 1); // remove initial /Objects from the path,

                                if (sourceNodes[i].NodeClass == NodeClass.Method && OPCUA_conn.commandsEnabled)
                                {
                                    var res = (MethodNode)sourceNodes[i];
                                    if (res.Executable && res.UserExecutable)
                                    {
                                        Log(conn_name + " - " + string.Format("NodeId {0} {1} {2} Path: {3}", sourceNodes[i].NodeId, sourceNodes[i].NodeClass, sourceNodes[i].BrowseName, reference.Path), LogLevelDetailed);

                                        OPCUA_conn.NodeIdsDetails[sourceNodes[i].NodeId.ToString()] = new NodeDetails
                                        {
                                            BrowseName = sourceNodes[i].BrowseName.Name,
                                            DisplayName = sourceNodes[i].DisplayName.Text,
                                            ParentName = parentName,
                                            Path = path,
                                        };
                                        // if not created, create a new command/method tag
                                        // Console.WriteLine(res);
                                        OPC_Value ov =
                                            new OPC_Value()
                                            {
                                                createCommandForMethod = true,
                                                createCommandForSupervised = false,
                                                valueJson = "",
                                                selfPublish = true,
                                                address = sourceNodes[i].NodeId.ToString(),
                                                isArray = false,
                                                asdu = "method",
                                                value = 0,
                                                valueString = "",
                                                hasSourceTimestamp = false,
                                                sourceTimestamp = DateTime.MinValue,
                                                serverTimestamp = DateTime.Now,
                                                quality = false,
                                                cot = 0,
                                                conn_number = conn_number,
                                                conn_name = conn_name,
                                                common_address = "",
                                                display_name = sourceNodes[i].DisplayName.Text,
                                                parentName = parentName,
                                                path = path,
                                            };
                                        OPCDataQueue.Enqueue(ov);
                                    }
                                    continue;
                                }
                                if (sourceNodes.Count != readErrorsDv.Count) continue; // must have read all the values to proceed
                                if (sourceNodes.Count != dataValues.Count) continue;
                                if (!StatusCode.IsGood(readErrorsDv[i].StatusCode)) continue;

                                var addToMonitoring = false;
                                if (OPCUA_conn.topics.Length == 0) addToMonitoring = true;
                                foreach (var topic in OPCUA_conn.topics)
                                {
                                    if (reference.Path.Contains(topic))
                                    {
                                        addToMonitoring = true;
                                        break;
                                    }
                                }
                                if (!addToMonitoring) continue;

                                Log(conn_name + " - " + string.Format("NodeId {0} {1} {2} Path: {3}", sourceNodes[i].NodeId, sourceNodes[i].NodeClass, sourceNodes[i].BrowseName, reference.Path), LogLevelDetailed);
                                OPCUA_conn.NodeIdsDetails[sourceNodes[i].NodeId.ToString()] = new NodeDetails
                                {
                                    BrowseName = sourceNodes[i].BrowseName.Name,
                                    DisplayName = sourceNodes[i].DisplayName.Text,
                                    ParentName = parentName,
                                    Path = path,
                                };
                                OPCUA_conn.ListMon.Add(new MonitoredItem()
                                {
                                    DisplayName = sourceNodes[i].BrowseName.Name,
                                    StartNodeId = sourceNodes[i].NodeId.ToString(),
                                    SamplingInterval = System.Convert.ToInt32(System.Convert.ToDouble(OPCUA_conn.autoCreateTagSamplingInterval) * 1000),
                                    QueueSize = System.Convert.ToUInt32(OPCUA_conn.autoCreateTagQueueSize),
                                    MonitoringMode = MonitoringMode.Reporting,
                                    DiscardOldest = true,
                                    AttributeId = Attributes.Value
                                });

                                if (dataValues[i].Value == null) continue;

                                ConvertOpcValue(dataValues[i], out string tp, out double dblValue, out string strValue, out string jsonValue, out bool isArray);

                                var createCommandForSupervised = false;
                                if (OPCUA_conn.commandsEnabled &&
                                    ((sourceNodes[i] as VariableNode).UserAccessLevel == AccessLevels.CurrentReadOrWrite ||
                                    (sourceNodes[i] as VariableNode).UserAccessLevel == AccessLevels.CurrentWrite))
                                    createCommandForSupervised = true; // variable can be written, create a command for it
                                OPC_Value iv =
                                        new OPC_Value()
                                        {
                                            createCommandForMethod = false,
                                            createCommandForSupervised = createCommandForSupervised,
                                            valueJson = jsonValue,
                                            selfPublish = true,
                                            address = sourceNodes[i].NodeId.ToString(),
                                            isArray = isArray,
                                            asdu = tp,
                                            value = dblValue,
                                            valueString = strValue,
                                            hasSourceTimestamp = false,
                                            sourceTimestamp = DateTime.MinValue,
                                            serverTimestamp = DateTime.Now,
                                            quality = false,
                                            cot = 20,
                                            conn_number = conn_number,
                                            conn_name = conn_name,
                                            common_address = "",
                                            display_name = sourceNodes[i].DisplayName.Text,
                                            parentName = parentName,
                                            path = path,
                                        };
                                OPCDataQueue.Enqueue(iv);
                            }
                        }
                        catch (Exception e)
                        {
                            Log(conn_name + " - Error reading nodes " + j);
                            Log(e);
                        }
                    }

                    await Task.Delay(50);
                    Log(conn_name + " - " + "Add a list of items (server current time and status) to the subscription.");
                    exitCode = ExitCode.ErrorMonitoredItem;
                    OPCUA_conn.ListMon.ForEach(i => i.Notification += OnNotification);
                    //OPCUA_conn.connection.session.Notification += OnSessionNotification;
                    OPCUA_conn.ListMon.ForEach(i => i.SamplingInterval = System.Convert.ToInt32(System.Convert.ToDouble(OPCUA_conn.autoCreateTagSamplingInterval) * 1000));
                    OPCUA_conn.ListMon.ForEach(i => i.QueueSize = System.Convert.ToUInt32(System.Convert.ToDouble(OPCUA_conn.autoCreateTagQueueSize)));
                    Log(conn_name + " - " + OPCUA_conn.ListMon.Count + " variables added to monitoring.");

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
                    subscription.AddItems(OPCUA_conn.ListMon);

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

            private void ConvertOpcValue(DataValue value, out string tp, out double dblValue, out string strValue, out string jsonValue, out bool isArray)
            {
                var base_type = "unknown";
                tp = "unknown";
                dblValue = 0.0;
                jsonValue = "";
                strValue = "";
                isArray = false;

                if (value != null)
                {
                    try
                    {
                        if (value.Value != null)
                        {
                            CntNotificEvents++;

                            try
                            {
                                jsonValue = JsonSerializer.Serialize(value.Value, jsonSerOpts);
                            }
                            catch (Exception)
                            { }

                            if (value.WrappedValue.TypeInfo != null)
                            {
                                base_type = value.WrappedValue.TypeInfo.BuiltInType.ToString().ToLower();
                                tp = value.WrappedValue.TypeInfo.ToString().ToLower();
                                isArray = value.WrappedValue.TypeInfo.ToString().Contains("[");
                            }
                            else
                            {
                                // Log(conn_name + " - " + item.ResolvedNodeId + " TYPE: ?????", LogLevelDetailed);
                            }

                            try
                            {
                                if (base_type == "variant" && !isArray)
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
                                                    strValue = jsonValue;
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
                                if ((base_type == "datetime" || base_type == "utctime") && !isArray)
                                {
                                    dblValue = ((DateTimeOffset)System.Convert.ToDateTime(value.Value)).ToUnixTimeMilliseconds();
                                    strValue = System.Convert.ToDateTime(value.Value).ToString("o");
                                }
                                else
                                if (base_type == "extensionobject" && !isArray)
                                {
                                    dblValue = 0;
                                    try
                                    {
                                        var obj = JsonNode.Parse(jsonValue);
                                        var obj2 = obj["Body"];
                                        if (obj2 != null)
                                        {
                                            obj2.AsObject().Remove("TypeId");
                                            obj2.AsObject().Remove("BinaryEncodingId");
                                            obj2.AsObject().Remove("XmlEncodingId");
                                            obj2.AsObject().Remove("JsonEncodingId");
                                            strValue = obj2.ToString();
                                        }
                                        else
                                        {
                                            strValue = obj.ToString();
                                        }
                                    }
                                    catch
                                    {
                                        strValue = jsonValue;
                                    }
                                }
                                else
                                if (base_type == "xmlelement" && !isArray)
                                {
                                    dblValue = 0;
                                    strValue = value.WrappedValue.ToString();
                                    jsonValue = "\"" + value.WrappedValue.ToString() + "\"";
                                }
                                else
                                if (base_type == "bytestring" && !isArray)
                                {
                                    dblValue = 0;
                                    strValue = System.Convert.ToBase64String(value.GetValue<byte[]>([]));
                                }
                                else
                                if (
                                    (base_type == "string" ||
                                    base_type == "localeid" ||
                                    base_type == "localizedtext" ||
                                    base_type == "nodeid" ||
                                    base_type == "expandediodeid" ||
                                    base_type == "xmlelement" ||
                                    base_type == "qualifiedname" ||
                                    base_type == "guid")
                                    && isArray
                                    )
                                {
                                    dblValue = 0;
                                    strValue = jsonValue;
                                }
                                else
                                if (base_type == "string" ||
                                    base_type == "localeid" ||
                                    base_type == "localizedtext" ||
                                    base_type == "nodeid" ||
                                    base_type == "expandednodeid" ||
                                    base_type == "xmlelement" ||
                                    base_type == "gualifiedname" ||
                                    base_type == "guid")
                                {
                                    dblValue = 0;
                                    strValue = System.Convert.ToString(value.Value);
                                }
                                else
                                if (isArray)
                                {
                                    dblValue = 0;
                                    strValue = jsonValue;
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
                        }
                    }
                    catch (Exception excpt)
                    {
                        Log(conn_name + " - ConvertValue - " + excpt.Message);
                    }
                }
            }
            private void OnNotification(MonitoredItem item, MonitoredItemNotificationEventArgs e)
            {
                //MonitoredItemNotification notification = e.N
                //notificationValue as MonitoredItemNotification;
                //Console.WriteLine("Notification Received for Variable \"{0}\" and Value = {1} type {2}.", item.DisplayName, notification.Value, notification.TypeId);

                foreach (var efl in item.DequeueEvents())
                {
                    // Log(efl.ToString());
                }
                foreach (var value in item.DequeueValues())
                {
                    if (value != null)
                    {
                        if (value.Value != null)
                        {
                            ConvertOpcValue(value, out string tp, out double dblValue, out string strValue, out string jsonValue, out bool isArray);
                            CntNotificEvents++;

                            // var parentName = "";
                            // var path = "";
                            // var details = NodeIdsDetails[item.ResolvedNodeId.ToString()];
                            // if (details != null)
                            // {
                            //     parentName = details.ParentName;
                            //     path = details.Path;
                            // }

                            OPC_Value iv =
                            new OPC_Value()
                            {
                                createCommandForMethod = false,
                                createCommandForSupervised = false,
                                valueJson = jsonValue,
                                selfPublish = true,
                                address = item.ResolvedNodeId.ToString(),
                                isArray = isArray,
                                asdu = tp,
                                value = dblValue,
                                valueString = strValue,
                                hasSourceTimestamp = value.SourceTimestamp != DateTime.MinValue,
                                sourceTimestamp = value.SourceTimestamp != DateTime.MinValue ? value.SourceTimestamp.AddHours(OPCUA_conn.hoursShift) : DateTime.MinValue,
                                serverTimestamp = DateTime.Now,
                                quality = StatusCode.IsGood(value.StatusCode),
                                cot = 3,
                                conn_number = conn_number,
                                conn_name = conn_name,
                                common_address = "",
                                display_name = item.DisplayName,
                                parentName = "",
                                path = "",
                            };
                            if (OPCDataQueue.Count < DataBufferLimit)
                                OPCDataQueue.Enqueue(iv);
                            else
                                CntLostDataUpdates++;
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
                // Console.WriteLine(conn_name + " - Notification Received Value = {0} type {1}.", notificationMsg.NotificationData, notificationMsg.TypeId);

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
                    NodeClassMask = (uint)NodeClass.Variable | (uint)NodeClass.Object | (uint)NodeClass.Method,
                    ResultMask = (uint)BrowseResultMask.All
                };
                var browseDescriptionCollection = CreateBrowseDescriptionCollectionFromNodeId(
                    new NodeIdCollection(new NodeId[] { startingNode ?? ObjectIds.RootFolder }),
                    browseTemplate);

                // Browse
                var referenceDescriptions = new Dictionary<ExpandedNodeId, (ReferenceDescription Reference, string Path)>();
                var rootPath = (await uaClient.Session.ReadNodeAsync(startingNode ?? ObjectIds.RootFolder)).BrowseName.Name;

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
                                Log(string.Format("Browse error: {0}", sre.Message));
                                throw;
                            }
                        }
                    } while (repeatBrowse);

                    if (maxNodesPerBrowse == 0)
                    {
                        // browseDescriptionCollection.Clear();
                    }
                    else
                    {
                        browseDescriptionCollection = browseDescriptionCollection.Skip(browseResultCollection.Count).ToArray();
                    }

                    // Browse next
                    var continuationPoints = PrepareBrowseNext(browseResultCollection);
                    while (continuationPoints.Any())
                    {
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
                                var newPath = parentPath.TrimEnd('/') + "/" + reference.BrowseName.Name;

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

                Log(string.Format("BrowseFullAddressSpace found {0} references on server in {1}ms.",
                    referenceDescriptions.Count, stopWatch.ElapsedMilliseconds));

                if (LogLevel >= LogLevelDebug)
                {
                    foreach (var (reference, path) in referenceDescriptions.Values)
                    {
                        Log(string.Format("NodeId {0} {1} {2} Path: {3}", reference.NodeId, reference.NodeClass, reference.BrowseName, path), LogLevelDebug);
                    }
                }

                return referenceDescriptions;
            }
        }
    }
}