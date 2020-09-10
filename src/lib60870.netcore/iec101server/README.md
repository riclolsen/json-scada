# IEC 60870-5-101 Server Protocol Driver

This driver implements a server for the IEC 101 protocol. This driver can use serial RS-232C ports or a TCP connection (a virtual serial port). It can distribute data over multiple connections on multiple computers, if needed. It can have only one clients connected on each TCPor serial port.
The driver listen for relevant data changes on a MongoDB database change stream, sending data changes upwards by exception. General interrogation and group interrogations are supported for integrity poll by the clients.

To configure the driver it is necessary to create one or more driver instances and at least on connection per instance. Also the tags intended to be sent on connections should be configured appropriately.

##  Configure a driver instance

To create a new IEC 101 server instance, insert a new document in the _protocolDriverInstances_ collection using a command like this:

    use json_scada_db_name
    db.protocolDriverInstances.insert({
            protocolDriver: "IEC60870-5-101_SERVER",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: ["mainNode"], 
            activeNodeName: "mainNode",
            activeNodeKeepAliveTimeTag: new Date(),
            keepProtocolRunningWhileInactive: true
        });

* _**protocolDriver**_ [String] - Name of the protocol driver, must be  "IEC60870-5-101_SERVER". **Mandatory parameter**.
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
            protocolDriver: "IEC60870-5-101_SERVER",
            protocolDriverInstanceNumber: 1,
            protocolConnectionNumber: 1002,
            name: "IEC101DIST",
            description: "Distribution of IEC 101",
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
            remoteLinkAddress: 0,
            giInterval: null,
            testCommandInterval: 0,
            timeSyncInterval: 0,
            sizeOfCOT: 2,
            sizeOfCA: 2,
            sizeOfIOA: 3,
            maxQueueSize: 50
        });


* _**protocolDriver**_ [String] - Name of the protocol driver, must be  "IEC60870-5-101_SERVER". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number will be used to direct tags to a distribution server connection. **Mandatory parameter**.
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
* _**remoteLinkAddress**_ [Double] - Not used for this driver. **Optional parameter**.
* _**giInterval**_ [Double] - Not used for this driver. **Optional parameter**.
* _**testCommandInterval**_ [Double] - Not used for this driver. **Optional parameter**.
* _**timeSyncInterval**_ [Double] - Not used for this driver. **Mandatory parameter**.
* _**sizeOfCOT**_ [Double] - Size of Cause Of Transmission protocol field in bytes (1 or 2). **Mandatory parameter**.
* _**sizeOfCA**_ [Double] - Size of Command Address protocol field in bytes (1 or 2). **Mandatory parameter**.
* _**sizeOfIOA**_ [Double] - Size of Information Object Address protocol field in bytes (1, 2, or 3). **Mandatory parameter**.
* _**maxQueueSize**_ [Double] - Maximum number of (Class1 or Class 2) messages that can be buffered. **Mandatory parameter**.

## Configure tags for distribution

Each tag to be distributed on a connection must have a protocol destination set. A tag can be also distributed on multiple connections.
The array of protocols destinations goes on the _protocolDestinations_ field of a tag on the _realtimeData_ collection.

The parameters of a protocol destination for a tag will define how the information will be transported by the protocol.

Select a tag for a distribution connection as below.

    use json_scada_db_name
    db.realtimeData.updateOne({"tag":"A_TAG_NAME"}, {
        $set: {
            "protocolDestinations":[{
                "protocolDestinationConnectionNumber": 1002,
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
        $push: {
            "protocolDestinations": {
                "protocolDestinationConnectionNumber": 1002,
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
* _**protocolDestinationCommonAddress**_ - Common Address (CA) used for the IEC 101 protocol. **Mandatory parameter**.
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
