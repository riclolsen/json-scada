# IEC 60870-5-104 Server Protocol Driver

This driver implements a server for the IEC 104 protocol. It can distribute data over multiple connections on multiple computers, if needed. It can have multiple clients connected on each TCP server opened port.
The driver listen for relevant data changes on a MongoDB database change stream, sending data changes upwards by exception. General interrogation and group interrogations are supported for integrity poll by the clients.

To configure the driver it is necessary to create one or more driver instances and at least one connection per instance. Also the tags intended to be sent on connections should be configured appropriately.

##  Configure a driver instance

To create a new IEC 104 server instance, insert a new document in the _protocolDriverInstances_ collection using a command like this:

    use json_scada_db_name
    db.protocolDriverInstances.insert({
            protocolDriver: "IEC60870-5-104_SERVER",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: ["mainNode"], 
            activeNodeName: "mainNode",
            activeNodeKeepAliveTimeTag: new Date(),
            keepProtocolRunningWhileInactive: true
        });

* _**protocolDriver**_ [String] - Name of the protocol driver, must be  "IEC60870-5-104_SERVER". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
* _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
* _**nodeNames**_ [Array of Strings] - Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate nodes will have the same connections and data enabled for distribution. **Mandatory parameter**.
* _**activeNodeName**_ [String] - Name of the protocol driver node that is currently active. This is updated by the drivers for redundancy control. This is not really used by this driver as all instances are always active. **Optional**.
* _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly  by the active driver. This is not really used by this driver as all instances are always active. **Optional**.
* _**keepProtocolRunningWhileInactive**_ [Boolean] - Defines that the driver will keep the protocol running while not the main active driver. This is not really used by this driver as all instances are always active. **Optional**.

Changes in the _protocolDriverInstances_ config requires that the driver instances processes be restarted to be effective.

## Configure servers

A instance for this driver can have many server ports defined that must be described in the _protocolConnections_ collection.

    use json_scada_db_name
    db.protocolConnections.insert({
            protocolDriver: "IEC60870-5-104_SERVER",
            protocolDriverInstanceNumber: 1,
            protocolConnectionNumber: 1001,
            name: "IEC104DIST",
            description: "Distribution of IEC 104",
            enabled: true,
            commandsEnabled: true,
            ipAddressLocalBind: "0.0.0.0:2404", 
            ipAddresses: [],
            localLinkAddress: 1,
            remoteLinkAddress: 0,
            giInterval: null,
            testCommandInterval: 0,
            timeSyncInterval: 0,
            sizeOfCOT: 2,
            sizeOfCA: 2,
            sizeOfIOA: 3,
            k: 12,
            w: 8,
            t0: 10,
            t1: 15,
            t2: 10,
            t3: 20,
            serverModeMultiActive: true,
            maxClientConnections: 20,
            maxQueueSize: 500
        });


* _**protocolDriver**_ [String] - Name of the protocol driver, must be  "IEC60870-5-104_SERVER". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number will be used to direct tags to a distribution server connection. **Mandatory parameter**.
* _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
* _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the conenction. Use false here to disable the connection. **Mandatory parameter**.
* _**commandsEnabled**_ [Boolean] - Allows to disable commands (messages in control direction) for a connection. Use false here to disable commands. **Mandatory parameter**.
* _**ipAddressLocalBind**_ [String] - Bind IP address and port for the server. Use 127.0.0.1:2404 for just local connections on port 2404. Use 0.0.0.0:2404 lo listen on all interfaces. Port must not repeat for other connections on the same computer (as only one listen socket can be opened for each TCP port on same machine). **Mandatory parameter**.
* _**ipAddresses**_ [Array of Strings] - Array of IP addresses for clients allowed to connect to the server. Keep empty array to accept any client. **Mandatory parameter**.
* _**localLinkAddress**_ [Double] - Local link address for the connection (originator address). **Mandatory parameter**.
* _**remoteLinkAddress**_ [Double] - Not used for this driver. **Optional parameter**.
* _**giInterval**_ [Double] - Not used for this driver. **Optional parameter**.
* _**testCommandInterval**_ [Double] - Not used for this driver. **Optional parameter**.
* _**timeSyncInterval**_ [Double] - Not used for this driver. **Mandatory parameter**.
* _**sizeOfCOT**_ [Double] - Size of Cause Of Transmission protocol field in bytes  (1 or 2). **Mandatory parameter**.
* _**sizeOfCA**_ [Double] - Size of Command Address protocol field in bytes (1 or 2). **Mandatory parameter**.
* _**sizeOfIOA**_ [Double] - Size of Information Object Address protocol field in bytes (1, 2, or 3). **Mandatory parameter**.
* _**k**_ [Double] - Protocol _k_ parameter. **Mandatory parameter**.
* _**w**_ [Double] - Protocol _w_ parameter. **Mandatory parameter**.
* _**t0**_ [Double] - Protocol _t0_ timeout in seconds. **Mandatory parameter**.
* _**t1**_ [Double] - Protocol _t1_ timeout in seconds. **Mandatory parameter**.
* _**t2**_ [Double] - Protocol _t2_ timeout in seconds. **Mandatory parameter**.
* _**t3**_ [Double] - Protocol _t3_ timeout in seconds. **Mandatory parameter**.
* _**serverModeMultiActive**_ [Boolean] - When true there is kept a separate data buffer for each client. **Mandatory parameter**.
* _**maxClientConnections**_ [Double] - Maximum number of clients allowed to connect at the same time. **Mandatory parameter**.
* _**maxQueueSize**_ [Double] - Maximum number of messages that can be buffered. **Mandatory parameter**.

Parameters needed only for TLS encrypted connections (when there are redundant servers, the same set of certificates is applied to connections to both servers).

* _**localCertFilePath**_ [String] - Path to file that contains the server certificate (*.pfx) that will be presented to the remote side of the connection. **Optional parameter**.
* _**passphrase**_ [String] - Password to the server certificate file (*.pfx). **Optional parameter**.
* _**peerCertFilePath**_ [String] - Path to certificate file used to verify the client (*.cer). Not required when _allowOnlySpecificCertificates=false_. **Optional parameter**.
* _**peerCertFilesPaths**_ [Array of Strings] - Path to certificate files used to verify additional clients (*.cer). Not required when _allowOnlySpecificCertificates=false_. **Optional parameter**.
* _**rootCertFilePath**_ [String] - Path to CA certificate file to check the certificates - not required when _chainValidation=false_. **Optional parameter**.
* _**allowOnlySpecificCertificates**_ [bool] - Indicates whether the driver allows only specific certificates. Default: false. **Optional parameter**.
* _**chainValidation**_ [bool] - Indicates whether the drivers performs a X509 chain validation against the registered CA certificates. Default: false. **Optional parameter**.

## Configure tags for distribution

Each tag to be distributed on a connection must have a protocol destination set. A tag can be also distributed on multiple connections.
The array of protocols destinations goes on the _protocolDestinations_ field of a tag on the _realtimeData_ collection.

The parameters of a protocol destination for a tag will define how the information will be transported by the protocol.

Select a tag for a distribution connection as below.

    use json_scada_db_name
    db.realtimeData.updateOne({"tag":"A_TAG_NAME"}, {
        $set: {
            "protocolDestinations":[{
                "protocolDestinationConnectionNumber": 1001,
                "protocolDestinationCommonAddress": 1,
                "protocolDestinationObjectAddress": 12345,
                "protocolDestinationASDU": 13,
                "protocolDestinationCommandDuration": 0,
                "protocolDestinationCommandUseSBO": false,
                "protocolDestinationKConv1": 1,
                "protocolDestinationKConv2": 0,
                "protocolDestinationGroup": 0,
                "protocolDestinationHoursShift": -2
                }]
        }
    });

This last command will remove any previous existing destinations for a tag.

Use a command like this below to add a destination without removing others.
 
    db.realtimeData.updateOne({"tag":"A_TAG_NAME"}, {
        $push:{
            "protocolDestinations":{
                "protocolDestinationConnectionNumber": 1001,
                "protocolDestinationCommonAddress": 1,
                "protocolDestinationObjectAddress": 12345,
                "protocolDestinationASDU": 13,
                "protocolDestinationCommandDuration": 0,
                "protocolDestinationCommandUseSBO": false,
                "protocolDestinationKConv1": 1,
                "protocolDestinationKConv2": 0,
                "protocolDestinationGroup": 0,
                "protocolDestinationHoursShift": -2
                }
        }
    });

Parameters description for _protocolDestinations_
* _**protocolDestinationConnectionNumber**_ - Number code of the protocol connection (must match the number code of the desired distribution connection defined on _protocolConnections_ collection). **Mandatory parameter**.
* _**protocolDestinationCommonAddress**_ - Common Address (CA) used for the IEC 104 protocol. **Mandatory parameter**.
* _**protocolDestinationObjectAddress**_ - Information Object Address (IOA) for the distributed value on protocol (the combination of CA/IOA must not repeat for distinct objects on the same connection). **Mandatory parameter**.
* _**protocolDestinationASDU**_ - ASDU type number code for data transport. E.g 13 for float, 1 for digital single, 45 for digital command. **Mandatory parameter**.
* _**protocolDestinationCommandDuration**_ - Command options, IEC-104 QU field: 0=Unspecified, 1=Short Pulse, 2=Long Pulse, 3=Persistent. Just meaningful for commands. **Mandatory parameter**.
* _**protocolDestinationCommandUseSBO**_ - Use or not Select Before Operate for the command. Use _false_ for direct execution. Only meaningful for commands. **Mandatory parameter**.
* _**protocolDestinationKConv1**_ - Conversion factor for values (multiplier). Use -1 to invert digital states. **Mandatory parameter**.
* _**protocolDestinationKConv2**_ - Conversion factor for values (adder). **Mandatory parameter**.
* _**protocolDestinationGroup**_ - Group of distribution (0-16). Zero will respond only to station general interrogation (INROGEN). 1-16 will respond to the specific group interrogation and INROGEN. Use -1 to avoid inclusion on any group interrogation response. **Mandatory parameter**.
* _**protocolDestinationHoursShift**_ - Number of hours to shift field time stamps. Use positive values to add hours and negative to subtract. Zero will keep the time stamps intact. **Mandatory parameter**.

When the protocol destination is changed for a tag, the change will be immediately effective on running drivers. There is no need to restart any process.

## Command Line Arguments

This driver has the following command line arguments.

* _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**.
* _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**.
* _**3rd arg. - Config File Path/Name**_ [String] - Complete path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**.
