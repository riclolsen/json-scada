# OPC-UA Client

This driver implements a client for the OPC-UA protocol.

This driver uses the OPC Foundation's .NET Standard library compiled to .NET 5.0 target platform.

    https://github.com/OPCFoundation/UA-.NETStandard

The driver can have multiple connections to OPC-UA servers on multiple computers, if needed.
To configure the driver it is necessary to create one or more driver instances and at least one connection per instance. Also the tags intended to be updated should be configured appropriately.

##  Configure a driver instance

To create a new OPC-UA client instance, insert a new document in the _protocolDriverInstances_ collection using a command like this:

    use json_scada_db_name
    db.protocolDriverInstances.insert({
            protocolDriver: "OPC-UA",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: ["mainNode"], 
            activeNodeName: "mainNode",
            activeNodeKeepAliveTimeTag: new Date(),
            keepProtocolRunningWhileInactive: false
        });

* _**protocolDriver**_ [String] - Name of the protocol driver, must be "OPC-UA". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
* _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
* _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate nodes will have the same connections and data enabled for scanning and update. **Mandatory parameter**.
* _**activeNodeName**_ [String] - Name of the protocol driver that is currently active. This is updated by the drivers for redundancy control.**Optional**.
* _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly  by the active driver. **Optional**.
* _**keepProtocolRunningWhileInactive**_ [Boolean] - Define a driver will keep the protocol running while not the main active driver. Currently only the _false_ value is supported. **Optional**.

Changes in the _protocolDriverInstances_ config requires that the driver instances processes be restarted to be effective.

## Configure client connections to OPC-UA servers

Each instance for this driver can have many client connection defined that must be described in the _protocolConnections_ collection.

    use json_scada_db_name
    db.protocolConnections.insert({
        protocolDriver: "OPC-UA",
        protocolDriverInstanceNumber: 1.0,
        protocolConnectionNumber: 81.0,
        name: "PLC1",
        description: "PLC1 - OPC-UA",
        enabled: true,
        commandsEnabled: true,
        endpointURLs: ["opc.tcp://opcuaserver.com:48010"],
        configFileName: "../conf/Opc.Ua.DefaultClient.Config.xml",
        autoCreateTags: true,
        autoCreateTagPublishingInterval: 2.5,
        autoCreateTagSamplingInterval: 0.0,
        autoCreateTagQueueSize: 5.0,
        timeoutMs: 20000,
        useSecurity: false,
        stats: {}
    });

Parameters for communication with OPC-UA servers.
* _**protocolDriver**_ [String] - Name of the protocol driver, must be  "DNP3". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is be used to define the connection that can update a tag. **Mandatory parameter**.
* _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
* _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
* _**commandsEnabled**_ [Boolean] - Allows to disable commands (messages in control direction) for a connection. Use false here to disable commands. **Mandatory parameter**.
* _**endpointURLs**_ [Array of Strings] - Array of server endpoints URLs (only the first server is currently supported). **Mandatory parameter**.
* _**configFileName**_ [String] - Name of the config file (with absolute path or relative to the bin folder). Default="../conf/Opc.Ua.DefaultClient.Config.xml". Use this file or crete new files to configure certificates and other OPC-UA parameters for a connection. **Optional parameter**.
* _**autoCreateTags**_ [Boolean] - When true the driver will create tags for every data point found in the server, all point will be subscribed. When false, only preconfigured tags will be updated. **Mandatory parameter**.
* _**autoCreateTagPublishingInterval**_ [Double] - Default publishing interval in seconds for subscription of auto created tags. **Mandatory parameter**.
* _**autoCreateTagSamplingInterval**_ [Double] - Default sampling interval in seconds for subscription of auto created tags. **Mandatory parameter**.
* _**autoCreateTagQueueSize**_ [Double] - Default queue size for subscription of auto created tags. **Mandatory parameter**.
* _**timeoutMs**_ [Double] - Timeout for keepalive messages. **Mandatory parameter**.
* _**useSecurity**_ [Boolean] - Use (true) or not (false) secure encrypted connection. **Mandatory parameter**.
* _**stats**_ [Object] - Protocol statistics updated by the driver. **Mandatory parameter**.

## Configure JSON-SCADA tags for update (read from OPC-UA Server)

Each tag to be update on a connection must have a protocol source set configured. Only one source connection can update a tag.

Select a tag for a update on a connection as below.

    use json_scada_db_name
    db.realtimeData.updateOne({"tag":"Demo.Dynamic.Scalar.StatusCode"}, {
        $set: {
            protocolSourceConnectionNumber: 81.0,
            protocolSourceCommonAddress: "Subscription1",
            protocolSourceObjectAddress: "ns=2;s=Demo.Dynamic.Scalar.StatusCode",
            protocolSourceASDU: "StatusCode", 
            protocolSourcePublishingInterval: 5.0,
            protocolSourceSamplingInterval: 0.0,
            protocolSourceQueueSize: 5.0,
            protocolSourceDiscardOldest: true,
            kconv1: 1.0,
            kconv2: 0.0
            }
    });

* _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. Only this protocol connection can update the tag. **Mandatory parameter**.
* _**protocolSourceCommonAddress**_ [String] - Name of a subscription for grouping. Leave empty for not subscribing (just polling). **Mandatory parameter**.
* _**protocolSourceObjectAddress**_ [String] - OPC-UA Node Id. This address must be unique for a connection. **Mandatory parameter**.
* _**protocolSourceASDU**_ [String] - Data type: Boolean | SByte | Byte | Int16 | UInt16 | Int32 | UInt32, | StatusCode | Int64 | UInt64 | DateTime | Float | Double | String | ByteString | XmlElement | JSON (JSON in a string) | [, Array Range]. E.g. UInt64,0:10. **Mandatory parameter**.
* _**protocolSourcePublishingInterval**_ [Double] - Publishing interval in seconds for the subscription group (repeat the same value for all members of a subscription). If not a subscription this is the polling interval. **Mandatory parameter**.
* _**protocolSourceSamplingInterval**_ [Double] - Sampling interval in seconds requested for the server. Only meaningful for subscriptions. Use zero for auto adjust on the server. **Mandatory parameter**.
* _**protocolSourceQueueSize**_ [Double] - Queue size for buffering of changes in the server between reports. Only meaningful for subscriptions. Use zero to avoid buffering. **Mandatory parameter**.
* _**protocolSourceDiscardOldest**_ [Boolean] - What to do when changes queue overflows. Use true to discard oldest changes.Only meaningful for subscriptions. **Mandatory parameter**.

* _**kconv1**_ [Double] - Analog conversion factor: multiplier. Use -1 to invert digital values. **Mandatory parameter**.
* _**kconv2**_ [Double] - Analog conversion factor: adder. **Mandatory parameter**.

## Command Line Arguments

This driver has the following command line arguments.

* _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**.
* _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**.
* _**3rd arg. - Config File Path/Name**_ [String] - Complete path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**.
