# IEC 60870-5-104 Client Protocol Driver

This driver implements a client for the IEC 104 protocol. It can have multiple connections to IEC-104 servers on multiple computers, if needed.

To configure the driver it is necessary to create one or more driver instances and at least one connection per instance. Also the tags intended to be updated should be configured appropriately.

##  Configure a driver instance

To create a new IEC 104 client instance, insert a new document in the _protocolDriverInstances_ collection using a command like this:

    use json_scada_db_name
    db.protocolDriverInstances.insert({
            protocolDriver: "IEC60870-5-104",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: ["mainNode"], 
            activeNodeName: "mainNode",
            activeNodeKeepAliveTimeTag: new Date(),
            keepProtocolRunningWhileInactive: false
        });

* _**protocolDriver**_ [String] - Name of the protocol driver, must be  "IEC60870-5-104". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
* _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
* _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate nodes will have the same connections and data enabled for scanning and update. **Mandatory parameter**.
* _**activeNodeName**_ [String] - Name of the protocol driver that is currently active. This is updated by the drivers for redundancy control.**Optional**.
* _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly  by the active driver. **Optional**.
* _**keepProtocolRunningWhileInactive**_ [Boolean] - Define a driver will keep the protocol running while not the main active driver. Currently only the _false_ value is supported. **Optional**.

Changes in the _protocolDriverInstances_ config requires that the driver instances processes be restarted to be effective.

## Configure client connections to IEC-104 servers

Each instance for this driver can have many client connection defined that must be described in the _protocolConnections_ collection.

    use json_scada_db_name
    db.protocolConnections.insert({
        protocolDriver: "IEC60870-5-104",
        protocolDriverInstanceNumber: 1,
        protocolConnectionNumber: 61,
        name: "KAW2",
        description: "KAW2 Station IEC-104",
        enabled: true,
        commandsEnabled: true,
        ipAddressLocalBind: "", 
        ipAddresses: ["192.168.0.21:2404", "192.168.0.22:2404"],
        localLinkAddress: 1,
        remoteLinkAddress: 205,
        giInterval: 300,
        testCommandInterval: 5,
        timeSyncInterval: 650,
        sizeOfCOT: 2,
        sizeOfCA: 2,
        sizeOfIOA: 3,
        k: 12,
        w: 8,
        t0: 10,
        t1: 15,
        t2: 10,
        t3: 20,
        stats: null
    });

* _**protocolDriver**_ [String] - Name of the protocol driver, must be  "IEC60870-5-104". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is be used to define the connection that can update a tag. **Mandatory parameter**.
* _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
* _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
* _**commandsEnabled**_ [Boolean] - Allows to disable commands (messages in control direction) for a connection. Use false here to disable commands. **Mandatory parameter**.
* _**ipAddressLocalBind**_ [String] - Not used for this driver. **Optional parameter**.
* _**ipAddresses**_ [Array of Strings] - Array of IP addresses and ports for IEC-104 servers to be scanned (only the first 2 servers are currently supported). When there are 2 servers configured, only one is connected and scanned at each time, servers are swapped when disconnected. **Mandatory parameter**.
* _**localLinkAddress**_ [Double] - Local link address for the connection (originator address). **Mandatory parameter**.
* _**remoteLinkAddress**_ [Double] - Remote link address of the server (originator address). **Optional parameter**.
* _**giInterval**_ [Double] - General station interrogation period in seconds. **Optional parameter**.
* _**testCommandInterval**_ [Double] - Interval to send test command messages in seconds. Use zero to disable test command messages. **Optional parameter**.
* _**timeSyncInterval**_ [Double] - Time interval in seconds to send time sync messages. Use zero to disable. **Mandatory parameter**.
* _**sizeOfCOT**_ [Double] - Size of Cause Of Transmission protocol field in bytes (1 or 2). **Mandatory parameter**.
* _**sizeOfCA**_ [Double] - Size of Command Address protocol field in bytes (1 or 2). **Mandatory parameter**.
* _**sizeOfIOA**_ [Double] - Size of Information Object Address protocol field in bytes (1, 2, or 3). **Mandatory parameter**.
* _**k**_ [Double] - Protocol _k_ parameter. **Mandatory parameter**.
* _**w**_ [Double] - Protocol _w_ parameter. **Mandatory parameter**.
* _**t0**_ [Double] - Protocol _t0_ timeout in seconds. **Mandatory parameter**.
* _**t1**_ [Double] - Protocol _t1_ timeout in seconds. **Mandatory parameter**.
* _**t2**_ [Double] - Protocol _t2_ timeout in seconds. **Mandatory parameter**.
* _**t3**_ [Double] - Protocol _t3_ timeout in seconds. **Mandatory parameter**.
* _**stats**_ [Double] - Protocol statistics updated by the driver. **Mandatory parameter**.

Parameters needed only for TLS encrypted connections (when there are redundant servers, the same set of certificates is applied to connections to both servers).

* _**localCertFilePath**_ [String] - File that contains the certificate (*.pfx) that will be presented to the remote side of the connection. **Optional parameter**.
* _**peerCertFilePath**_ [String] - Certificate file used to verify the peer or server (*.cer). **Optional parameter**.
* _**rootCertFilePath**_ [String] - CA certificate to check the certificate provided by the server - not required when ChainValidation == false. **Optional parameter**.
* _**allowOnlySpecificCertificates**_ [bool] - Indicates whether the driver allows only specific certificates. Default: false. **Optional parameter**.
* _**chainValidation**_ [bool] - Indicates whether the drivers performs a X509 chain validation against the registered CA certificates. Default: false. **Optional parameter**.

## Configure tags for update

Each tag to be update on a connection must have a protocol source set configured. 
Only one source connection can update a tag.

Select a tag for a update on a connection as below.

    use json_scada_db_name
    db.realtimeData.updateOne({"tag":"A_TAG_NAME"}, {
        $set: {
            protocolSourceConnectionNumber: 61,
            protocolSourceCommonAddress: 205,
            protocolSourceObjectAddress: 1000,
            protocolSourceASDU: 13,
            protocolSourceCommandDuration: 0,
            protocolSourceCommandUseSBO: false,
            kconv1: 1,
            kconv2: 0
            }
    });

* _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. Only this protocol connection can update the tag. **Mandatory parameter**.
* _**protocolSourceCommonAddress**_ [Double] - Common Address of ASDU. There can be more than one common address in the same connection. **Mandatory parameter**.
* _**protocolSourceObjectAddress**_ [Double] - Object address. This address combined with _protocolSourceCommonAddress_ must be unique for the connection. **Mandatory parameter**.
* _**protocolSourceASDU**_ [Double] - Source ASDU TI type. This is documental, the protocol driver will update the tag using any supported ASDU type. **Mandatory parameter**.
* _**protocolSourceCommandDuration**_ [Double] - Command options, IEC-104 QU field: 0=Unspecified, 1=Short Pulse, 2=Long Pulse, 3=Persistent. Just meaningful for commands. **Mandatory parameter**.
* _**protocolSourceCommandUseSBO**_ [Double] - Use Select-Before-Operate control sequence. Just meaningful for commands. **Mandatory parameter**.
* _**kconv1**_ [Double] - Analog conversion factor: multiplier. Use -1 to invert digital values. **Mandatory parameter**.
* _**kconv2**_ [Double] - Analog conversion factor: adder. **Mandatory parameter**.

## Command Line Arguments

This driver has the following command line arguments.

* _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**.
* _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**.
* _**3rd arg. - Config File Path/Name**_ [String] - Complete path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**.
