# IEC 60870-5-101 Client Protocol Driver

This driver implements a client for the IEC 101 protocol. It can have multiple connections to IEC-101 servers on multiple computers, if needed.

To configure the driver it is necessary to create one or more driver instances and at least on connection per instance. Also the tags intended to be updated should be configured appropriately.

##  Configure a driver instance

To create a new IEC 101 client instance, insert a new document in the _protocolDriverInstances_ collection using a command like this:

    use json_scada_db_name
    db.protocolDriverInstances.insert({
            protocolDriver: "IEC60870-5-101",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: ["mainNode"], 
            activeNodeName: "mainNode",
            activeNodeKeepAliveTimeTag: new Date(),
            keepProtocolRunningWhileInactive: false
        });

* _**protocolDriver**_ [String] - Name of the protocol driver, must be  "IEC60870-5-101". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
* _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
* _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate nodes will have the same connections and data enabled for scanning and update. **Mandatory parameter**.
* _**activeNodeName**_ [String] - Name of the protocol driver that is currently active. This is updated by the drivers for redundancy control.**Optional**.
* _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly  by the active driver. **Optional**.
* _**keepProtocolRunningWhileInactive**_ [Boolean] - Define a driver will keep the protocol running while not the main active driver. Currently only the _false_ value is supported. **Optional**.

Changes in the _protocolDriverInstances_ config requires that the driver instances processes be restarted to be effective.

## Configure client connections to IEC-101 servers

Each instance for this driver can have many client connection defined that must be described in the _protocolConnections_ collection.

    use json_scada_db_name
    db.protocolConnections.insert({
        protocolDriver: "IEC60870-5-101",
        protocolDriverInstanceNumber: 1,
        protocolConnectionNumber: 62,
        name: "KAK1",
        description: "KAK1 Station IEC-101",
        enabled: true,
        commandsEnabled: true,
        portName: "COM3", 
        baudRate: 9600,
        parity: "Even",
        stopBits: "One",
        handshake: "None",
        timeoutForACK: 1000,
        timeoutRepeat: 1000,
        useSingleCharACK: true,
        sizeOfLinkAddress: 1,
        localLinkAddress: 1,
        remoteLinkAddress: 206,
        sizeOfCOT: 1,
        sizeOfCA: 1,
        sizeOfIOA: 2,
        giInterval: 300,
        testCommandInterval: 15,
        timeSyncInterval: 650
    });

* _**protocolDriver**_ [String] - Name of the protocol driver, must be  "IEC60870-5-101". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is be used to define the connection that can update a tag. **Mandatory parameter**.
* _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
* _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
* _**commandsEnabled**_ [Boolean] - Allows to disable commands (messages in control direction) for a connection. Use false here to disable commands. **Mandatory parameter**.
* _**portName**_ [String] - Comm port name, e.g. "COM1", "/dev/ttyS0", "192.168.0.1:2410" for TCP address:port. **Mandatory parameter**.
* _**baudRate**_ [Double] - Comm port baud rate. **Mandatory parameter**.
* _**parity**_ [String] - Comm port parity Even|None|Odd|Mark|Space. **Mandatory parameter**.
* _**stopBits**_ [String] - Comm port number of stop bits One|One5|Two. **Mandatory parameter**.
* _**handshake**_ [String] - Comm port handshake option None|Xon|Rts|RtsXon. **Mandatory parameter**.
* _**timeoutForACK**_ [Double] - Comm port timeout for ACK of the link layer message (ms). **Mandatory parameter**.
* _**timeoutRepeat**_ [Double] - Comm port timeout for repeated transmission of link layer messages (ms). **Mandatory parameter**.
* _**useSingleCharACK**_ [Boolean] - Indicates if the secondary link layer will use single char ACK (E5). **Mandatory parameter**.
* _**sizeOfLinkAddress**_ [Double] - Size of the link layer address field of the LPCI. Can be 0, 1, or 2. **Mandatory parameter**.
* _**localLinkAddress**_ [Double] - Local link address for the connection (originator address). **Mandatory parameter**.
* _**remoteLinkAddress**_ [Double] - Remote link address of the server (originator address). **Optional parameter**.
* _**giInterval**_ [Double] - General station interrogation period in seconds. **Optional parameter**.
* _**testCommandInterval**_ [Double] - Interval to send test command messages in seconds. Use zero to disable test command messages. **Optional parameter**.
* _**timeSyncInterval**_ [Double] - Time interval in seconds to send time sync messages. Use zero to disable. **Mandatory parameter**.
* _**sizeOfCOT**_ [Double] - Size of Cause Of Transmission protocol field in bytes (1 or 2). **Mandatory parameter**.
* _**sizeOfCA**_ [Double] - Size of Command Address protocol field in bytes (1 or 2). **Mandatory parameter**.
* _**sizeOfIOA**_ [Double] - Size of Information Object Address protocol field in bytes (1, 2, or 3). **Mandatory parameter**.

## Configure tags for update

Each tag to be update on a connection must have a protocol source set configured. 
Only one source connection can update a tag.

Select a tag for a update on a connection as below.

    use json_scada_db_name
    db.realtimeData.updateOne({"tag":"A_TAG_NAME"}, {
        $set: {
            protocolSourceConnectionNumber: 62,
            protocolSourceCommonAddress: 206,
            protocolSourceObjectAddress: 2000,
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
* _**protocolSourceCommandDuration**_ [Double] - Command options, IEC-101 QU field: 0=Unspecified, 1=Short Pulse, 2=Long Pulse, 3=Persistent. Just meaningful for commands. **Mandatory parameter**.
* _**protocolSourceCommandUseSBO**_ [Double] - Use Select-Before-Operate control sequence. Just meaningful for commands. **Mandatory parameter**.
* _**kconv1**_ [Double] - Analog conversion factor: multiplier. Use -1 to invert digital values. **Mandatory parameter**.
* _**kconv2**_ [Double] - Analog conversion factor: adder. **Mandatory parameter**.

## Command Line Arguments

This driver has the following command line arguments.

* _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**.
* _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**.
* _**3rd arg. - Config File Path/Name**_ [String] - Complete path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**.
