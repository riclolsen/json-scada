# OPC-DA Client

This driver implements a client for the OPC-DA protocol.

This driver uses the OPC DA/AE/HDA Solution .NET from Technosoftware, licensed under GPL 3.0.

    https://github.com/technosoftware-gmbh/opcdaaehda-client-solution-net

The driver can have multiple connections to OPC-DA servers on multiple computers, as needed.
To configure the driver it is necessary to create one or more driver instances and at least one connection per instance. Also the tags intended to be updated should be configured appropriately or the Autotag function should be enabled.

## Configure a driver instance

To create a new OPC-DA client instance, insert a new document in the _protocolDriverInstances_ collection using a command like below or use the Admin UI to create a new entry. 

    use json_scada_db_name
    db.protocolDriverInstances.insert({
            protocolDriver: "OPC-DA",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: ["mainNode"],
            activeNodeName: "mainNode",
            activeNodeKeepAliveTimeTag: new Date(),
            keepProtocolRunningWhileInactive: false
        });

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "OPC-DA". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
- _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
- _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate nodes will have the same connections and data enabled for scanning and update. **Mandatory parameter**.
- _**activeNodeName**_ [String] - Name of the protocol driver that is currently active. This is updated by the drivers for redundancy control.**Optional**.
- _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly by the active driver. **Optional**.
- _**keepProtocolRunningWhileInactive**_ [Boolean] - Define a driver will keep the protocol running while not the main active driver. Currently only the _false_ value is supported. **Optional**.

Changes in the _protocolDriverInstances_ config requires that the driver instances processes be restarted to be effective.

## Configure client connections to OPC-DA servers

Each instance for this driver can have many client connection defined that must be described in the _protocolConnections_ collection. Also the Admin UI can be use to add a new connection.

    use json_scada_db_name
    db.protocolConnections.insert({
        protocolDriver: "OPC-DA",
        protocolDriverInstanceNumber: 1.0,
        protocolConnectionNumber: 5001.0,
        name: "PLCDA1",
        description: "PLCDA1 - OPC-DA",
        enabled: true,
        commandsEnabled: true,
        endpointURLs: ["opcda://localhost/SampleCompany.DaSample.30"],
        topics: [],
        giInterval: 300,
        autoCreateTags: true,
        autoCreateTagPublishingInterval: 5.0,
        autoCreateTagSamplingInterval: 0.0,
        autoCreateTagQueueSize: 0.0,
        deadBand: 5.0,
        hoursShift: 0.0,
        timeoutMs: 20000,
        useSecurity: false,
        localCertFilePath: "",
        peerCertFilePath: "",
        password: "",
        username: "",
        stats: {}
    });

Parameters for communication with OPC-DA servers.

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "OPC-DA". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is be used to define the connection that can update a tag. **Mandatory parameter**.
- _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
- _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
- _**commandsEnabled**_ [Boolean] - Allows to disable commands (messages in control direction) for a connection. Use false here to disable commands. **Mandatory parameter**.
- _**endpointURLs**_ [Array of Strings] - Array of server endpoints URLs. Use multiple entries for redundant servers. Entries must be defined in the following form "opcda://hostname/ServiceName". **Mandatory parameter**.
- _**autoCreateTags**_ [Boolean] - When true the driver will create tags for every data point found in the server, all points will be subscribed. When false, only preconfigured tags will be updated. **Mandatory parameter**.
- _**autoCreateTagPublishingInterval**_ [Double] - Default publishing interval in seconds for subscription of auto created tags. **Mandatory parameter**.
- _**autoCreateTagSamplingInterval**_ [Double] - Currently not used. **Mandatory parameter**.
- _**autoCreateTagQueueSize**_ [Double] - Currently not used. **Mandatory parameter**.
- _**deadBand**_ [Double] - Default dead-band (percent) for generating data updates by exception. **Mandatory parameter**.
- _**hoursShift**_ [Double] - Time shift to be applied to server timestamps (hours). **Mandatory parameter**.
- _**timeoutMs**_ [Double] - Currently not used. **Mandatory parameter**.
- _**useSecurity**_ [Boolean] - Use (true) or not (false) certificates. **Mandatory parameter**.
- _**localCertFilePath**_ [String] - Name of the file that contains the certificate that will be presented to the remote side of the connection. **Mandatory parameter**.
- _**peerCertFilePath**_ [String] - Name of the file that contains the certificate used to verify the server. **Mandatory parameter**.
- _**username**_ [String] - Username for authentication. Domain can be specified like this: "domain/username". Leave empty if not required by the server. **Mandatory parameter**.
- _**password**_ [String] - Password for authentication. Leave empty if not required by the server. **Mandatory parameter**.
- _**stats**_ [Object] - Protocol statistics updated by the driver. **Mandatory parameter**.

## Configure JSON-SCADA tags for update (reading from an OPC-DA Server)

Each tag to be update on a connection must have a protocol source set configured. Only one source connection can update a tag.

Select a tag for a update on a connection as below.

    use json_scada_db_name
    db.realtimeData.updateOne({"tag":"PLCDA1.Square Waves.Boolean"}, {
        $set: {
            protocolSourceConnectionNumber: 5001.0,
            protocolSourceCommonAddress: "Square Waves",
            protocolSourceObjectAddress: "Square Waves.Boolean",
            protocolSourceASDU: "StatusCode",
            protocolSourcePublishingInterval: 5.0,
            protocolSourceSamplingInterval: 0.0,
            protocolSourceQueueSize: 0.0,
            protocolSourceDiscardOldest: true,
            kconv1: 1.0,
            kconv2: 0.0
            }
    });

- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. Only this protocol connection can update the tag. **Mandatory parameter**.
- _**protocolSourceCommonAddress**_ [String] - Branch name when autotag used. Leave empty if not known. Just documental. **Mandatory parameter**.
- _**protocolSourceObjectAddress**_ [String] - OPC-DA item Id. **Mandatory parameter**.
- _**protocolSourceASDU**_ [String] - Data type as detected by the library when autotag used: Boolean | SByte | Byte | Int16 | UInt16 | Int32 | UInt32, | Int64 | UInt64 | DateTime | Decimal | Float | Double | String | Type[] (array with elements of a type). Leave empty if not known. Just documental, not really used. **Mandatory parameter**.
- _**protocolSourcePublishingInterval**_ [Double] - Currently unused. **Mandatory parameter**.
- _**protocolSourceSamplingInterval**_ [Double] - Currently unused. **Mandatory parameter**.
- _**protocolSourceQueueSize**_ [Double] - Currently unused. **Mandatory parameter**.
- _**protocolSourceDiscardOldest**_ [Boolean] - Currently unused. **Mandatory parameter**.
- _**kconv1**_ [Double] - Analog conversion factor: multiplier. Use -1 to invert digital values. **Mandatory parameter**.
- _**kconv2**_ [Double] - Analog conversion factor: adder. **Mandatory parameter**.

## Configure JSON-SCADA command tags (writing to an OPC-DA Server)

Create a regular command tag. Configure the connection number, OPCDA node id (object address) and OPCDA type (ASDU).

    use json_scada_db_name
    db.realtimeData.updateOne({"tag":"PLCDA1.Square Waves.Boolean.Cmd"}, {
        $set: {
            protocolSourceConnectionNumber: 5001.0,
            protocolSourceCommonAddress: "Square Waves",
            protocolSourceObjectAddress: "Square Waves.Boolean",
            protocolSourceASDU: "VT_BOOL",
            kconv1: 1.0,
            kconv2: 0.0
            }
    });

- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. Only this protocol connection can command this tag. **Mandatory parameter**.
- _**protocolSourceCommonAddress**_ [String] - Branch name. Not really used, just documental. **Mandatory parameter**.
- _**protocolSourceObjectAddress**_ [String] - OPC-DA Node Id. This address must be unique in a connection (for commands). **Mandatory parameter**.
- _**protocolSourceASDU**_ [String] - Data type: VT_BOOL | VT_I1 | VT_UI1 | VT_I2 | VT_UI2 | VT_I4 | VT_UI4, | VT_I8 | VT_UI8 | VT_DATE | VT_R4 | VT_R8 | VT_CY | VT_BSTR | VT_TYPE[] (array with elements of a type). **Mandatory parameter**.
- _**kconv1**_ [Double] - Analog conversion factor: multiplier. Use -1 to invert digital values. **Mandatory parameter**.
- _**kconv2**_ [Double] - Analog conversion factor: adder. **Mandatory parameter**.

## Send commands by inserting document into commandsQueue collection

Commands can be also send via code or mongoshell by inserting documents into the into commandsQueue collection.

    use json_scada_db_name
    db.commandsQueue.insert(
    {
    "protocolSourceConnectionNumber": 5001,
    "protocolSourceCommonAddress": "Square Waves",
    "protocolSourceObjectAddress": "Square Waves.Boolean",
    "protocolSourceASDU": "VT_BOOL",
    "protocolSourceCommandDuration": 0,
    "protocolSourceCommandUseSBO": false,
    "pointKey": 500100000011,
    "tag": "PLCDA1.Square Waves.Boolean.Cmd",
    "timeTag": {
        "$date": "2025-06-15T13:16:53.291Z"
    },
    "value": 1,
    "valueString": "true",
    "originatorUserName": "admin",
    "originatorIpAddress": "127.0.0.1"
    });

## Command Line Arguments

This driver has the following command line arguments.

- _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**.
- _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**.
- _**3rd arg. - Config File Path/Name**_ [String] - Complete path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**.

## Example of JSON-SCADA Protocol Driver Instances and Connections Numbering

![Driver instances and connections](https://github.com/riclolsen/json-scada/raw/master/docs/JSON-SCADA_Connections.png 'Driver Instances and Connections Numbering')