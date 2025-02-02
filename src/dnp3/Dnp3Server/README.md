# DNP3 Server Protocol Driver

This DNP3 client driver is programmed in C++ with the OpenDNP3 library.

This driver implements a server for the DNP3 protocol. It can have multiple channels (each channel can have multiple servers) with options like TCP, UDP, TLS, serial, etc.

To configure the driver it is necessary to create one or more driver instances and at least one connection per instance. Also the tags intended to be updated should be configured appropriately.

## Configure a driver instance

To create a new DNP3 server instance, insert a new document in the _protocolDriverInstances_ collection using a command like below or via the Admin UI.

    use json_scada_db_name
    db.protocolDriverInstances.insert({
            protocolDriver: "DNP3_SERVER",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: ["mainNode"],
            activeNodeName: "",
            activeNodeKeepAliveTimeTag: new Date(),
            keepProtocolRunningWhileInactive: false
        });

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "DNP3_SERVER". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
- _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
- _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. **Mandatory parameter**.
- _**activeNodeName**_ [String] - This parameter is not used, as the server is always active in all enabled nodes. **Optional**.
- _**activeNodeKeepAliveTimeTag**_ [Date] - This parameter is not used, as the server is always active in all enabled nodes. **Optional**.
- _**keepProtocolRunningWhileInactive**_ [Boolean] - This parameter is not used, leave the value as _false_. **Optional**.

Changes in the _protocolDriverInstances_ config requires that the driver process be restarted to be effective.

## Configure DNP3 server connections

Each instance for this driver can have many client connection defined that must be described in the _protocolConnections_ collection.
Create a new connection in Admin UI or directly on MongoDB as below or using the Admin UI.

    use json_scada_db_name
    db.protocolConnections.insert({
        "protocolDriver": "DNP3_SERVER",
        "protocolDriverInstanceNumber": 1,
        "protocolConnectionNumber": 34,
        "name": "DNP3SRV",
        "description": "DNP3 SERVER DEMO",
        "enabled": true,
        "commandsEnabled": true,
        "autoCreateTags": true,
        "topics": [ "KAW2" ],
        "localLinkAddress": 2,
        "remoteLinkAddress": 1,
        "enableUnsolicited": true,
        "connectionMode": "TCP Passive",
        "ipAddresses": [],
        "ipAddressLocalBind": "0.0.0.0:20000",
        "hoursShift": 0,
        "timeoutMs": 10000,
        "timeSyncInterval": 0,
        "timeSyncMode": 0,
        "privateKeyFilePath": "",
        "localCertFilePath": "",
        "peerCertFilePath": "",
        "chainValidation": false,
        "allowOnlySpecificCertificates": false,
        "allowTLSv10": false,
        "allowTLSv11": false,
        "allowTLSv12": true,
        "allowTLSv13": true,
        "cipherList": "",
        "portName": "COM1"
        "baudRate": 9600,
        "parity": "Even",
        "stopBits": "One",
        "handshake": "None",
        "asyncOpenDelay": 0,
        "serverQueueSize": 2000,
        "stats": null
    });
- _**protocolDriver**_ [String] - Name of the protocol driver, must be "DNP3". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is be used to define the connection that can update a tag. **Mandatory parameter**.
- _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
- _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
- _**commandsEnabled**_ [Boolean] - Allows to disable commands (messages in control direction) for a connection. Use false here to disable commands. **Mandatory parameter**.
- _**localLinkAddress**_ [Double] - Local link address for the connection (originator address). **Mandatory parameter**.
- _**remoteLinkAddress**_ [Double] - Remote link address of the server (originator address). **Optional parameter**.
- _**timeSyncMode**_ [Double] - Time sync mode (from client when requested by the RTU server): 0=none, 1=non-lan, 2=lan. Use zero to disable. **Mandatory parameter**.
- _**timeSyncInterval**_ [Double] - Time sync interval in seconds. Use zero to disable. **Mandatory parameter**.
- _**enableUnsolicited**_ [Boolean] - Enable (true) or disable (false) unsolicited mode. **Mandatory parameter**.
- _**stats**_ [Object] - Protocol statistics updated by the driver. **Mandatory parameter**.
- _**connectionMode**_ [String] - Connection mode: 'TCP Active'|'TCP Passive'|'TLS Active'|'TLS Passive'|'UDP'|'Serial'. **Mandatory parameter**.
- _**autoCreateTags**_ [Boolean] - When true, auto create entry in the protocolDestinations field for existing tags (analog and digital supervised tags, and analog and digital command tags) for distribution in the current connection. **Mandatory parameter**.
- _**topics**_ [Array of Strings] - Filter auto created destinations for tags by group1. **Mandatory parameter**.
- _**hoursShift**_ [Double] - Number of hours to shift time stamps. Use positive values to add hours and negative to subtract. Zero will keep the time stamps intact. This affects all the points of the connection. Use _protocolDestinationHoursShift_ to specify time shift point by point. The to parameters will be added. **Mandatory parameter**.

For TCP communication.

- _**ipAddressLocalBind**_ [String] - For passive mode: local bind address. For active mode: address of the network interface used to reach the client peer. **Mandatory parameter**.
- _**ipAddresses**_ [Array of Strings] - For passive mode: array of IP addresses allowed to connect to the server. For active mode: addresses of the client peer(s). **Mandatory parameter**.

For UDP communication.

- _**ipAddressLocalBind**_ [String] - IP address and port of local UDP endpoint. **Mandatory parameter**.
- _**ipAddresses**_ [Array of Strings] - Array of IP addresses and ports for DNP3 servers to be scanned (only the first server is currently supported). **Mandatory parameter**.

For Serial communication.

- _**portName**_ [String] - Comm port name, e.g. "COM1", "/dev/ttyS0". **Mandatory parameter**.
- _**baudRate**_ [Double] - Comm port baud rate. **Mandatory parameter**.
- _**parity**_ [String] - Comm port parity None|Even|Odd. **Mandatory parameter**.
- _**stopBits**_ [String] - Comm port number of stop bits One|One5|Two. **Mandatory parameter**.
- _**handshake**_ [String] - Comm port handshake option None|Xon|Rts. **Mandatory parameter**.
- _**asyncOpenDelay**_ [Double] - Delay on first TX in ms. **Mandatory parameter**.

For TLS over TCP.

- _**ipAddressLocalBind**_ [String] - For passive mode: local bind address. For active mode: address of the network interface used to reach the client peer. **Mandatory parameter**.
- _**ipAddresses**_ [Array of Strings] - For passive mode: array of IP addresses allowed to connect to the server. For active mode: addresses of the client peer(s). **Mandatory parameter**.
- _**allowTLSv10**_ [Boolean] - Allow TLS version 1.0 (default false).
- _**allowTLSv11**_ [Boolean] - Allow TLS version 1.1 (default false).
- _**allowTLSv12**_ [Boolean] - Allow TLS version 1.2 (default true).
- _**allowTLSv13**_ [Boolean] - Allow TLS version 1.3 (default true).
- _**cipherList**_ [String] - Openssl format cipher list .
- _**localCertFilePath**_ [String] - File that contains the certificate (or certificate chain) that will be presented to the remote side of the connection. **Mandatory parameter**.
- _**peerCertFilePath**_ [String] - Certificate file used to verify the peer or server. **Mandatory parameter**.
- _**privateKeyFilePath**_ [String] - File that contains the private key corresponding to the local certificate. **Mandatory parameter**.

### Multi-drop

In the multi-drop case, multiple slave devices share the same TCP, UDP, TLS or Serial channel connection.

For a multi-drop configuration, use a new connection (in the _protocolConnections_ collection) for each device repeating channel specification (endpoint IP address or serial port). Each device on a shared channel must have a distinct _remoteLinkAddress_ parameter.

## Configure tags for distribution on a connection

Each tag to be updated on a server connection must have a protocol destination entry configured.
Only one source connection can update a tag.

Select a tag for update on a connection as below or using the Admin UI.

    db.realtimeData.updateOne({"tag":"A_TAG_NAME"}, {
        $push:{
            "protocolDestinations":{
                "protocolDestinationConnectionNumber": 34,
                "protocolDestinationCommonAddress": 1,
                "protocolDestinationObjectAddress": 0,
                "protocolDestinationASDU": 2,
                "protocolDestinationCommandDuration": 0,
                "protocolDestinationCommandUseSBO": false,
                "protocolDestinationKConv1": 1,
                "protocolDestinationKConv2": 0,
                "protocolDestinationGroup": 0,
                "protocolDestinationHoursShift": 0
                }
        }
    });

Parameters description for _protocolDestinations_

- _**protocolDestinationConnectionNumber**_ [Double]- Number code of the protocol connection (must match the number code of the desired distribution connection defined on _protocolConnections_ collection). **Mandatory parameter**.
- _**protocolDestinationCommonAddress**_ [Double]- DNP3 Group number (e.g. 1 for binary input, 30 for analog input). **Mandatory parameter**.
- _**protocolDestinationObjectAddress**_ [Double]- DNP3 Object Address (0-65535), unique in the connection for each type of object (analog inputs, binary inputs, counters, analog outputs, binary outputs, etc.). **Mandatory parameter**.
- _**protocolDestinationASDU**_ [Double] - Variation within DNP3 group (e.g. group 30 variation 1 for analog input 32 bits). **Mandatory parameter**.
- _**protocolDestinationCommandDuration**_ [Double] - CROB Command options (see table below). Just documental placeholder (command will be routed to be executed on the source protocol connection). **Mandatory parameter**.
- _**protocolDestinationCommandUseSBO**_ [Boolean] - Use or not Select Before Operate for the command. **Mandatory parameter**.
- _**protocolDestinationKConv1**_ [Double] - Conversion factor for values (multiplier). Use -1 to invert digital states. **Mandatory parameter**.
- _**protocolDestinationKConv2**_ [Double] - Conversion factor for values (adder). **Mandatory parameter**.
- _**protocolDestinationGroup**_ [Double] - Not used. Leave as zero. **Mandatory parameter**.
- _**protocolDestinationHoursShift**_ [Double] - Number of hours to shift time stamps. Use positive values to add hours and negative to subtract. Zero will keep the time stamps intact. **Mandatory parameter**.

When the protocol destination is changed for a tag, it is necessary to restart the protocol driver to apply the changes.

## Supported DNP3 Group/Variations for Common Address and ASDU Parameters

| Object Group | Variation | Common Address / ASDU | Description                                    |
|--------------|-----------|-----------------------|------------------------------------------------|
| 1            | 2         | **1 / 2**             | Binary Input, flags                            |
| 2            | 2         | **1 / 2**             | Binary Input Change, with absolute time        |
| 3            | 2         | **3 / 2**             | Double Binary Input, flags                     |
| 4            | 2         | **3 / 2**             | Double Binary Input Change, with absolute time |
| 30           | 1         | **30 / 1**            | 32 bit Analog Input, flag                      |
| 32           | 1         | **30 / 1**            | 32 bit Analog Input Change, w/o time           |
| 30           | 2         | **30 / 2**            | 16 bit Analog Input                            |
| 32           | 2         | **30 / 2**            | 16 bit Analog Input Change, w/o time           |
| 30           | 3         | **30 / 3**            | 32 bit Analog Input                            |
| 32           | 3         | **30 / 3**            | 32 bit Analog Input Change, with time          |
| 30           | 4         | **30 / 4**            | 16 bit Analog Input                            |
| 32           | 4         | **30 / 4**            | 16 bit Analog Input Change, with time          |
| 30           | 5         | **30 / 5**            | Single Precision Analog Input                  |
| 32           | 5         | **30 / 5**            | Single Precision Analog Input Change, w/o time |
| 30           | 6         | **30 / 6**            | Double Precision Analog Input                  |
| 32           | 6         | **30 / 6**            | Double Precision Input Change, w/o time        |
| 30           | 5         | **30 / 7**            | Single Precision Analog Input                  |
| 32           | 7         | **30 / 7**            | Single Precision Analog Input Change, w/ time  |
| 30           | 6         | **30 / 8**            | Double Precision Analog Input                  |
| 32           | 8         | **30 / 8**            | Double Precision Input Change, w/ time         |
| 20           | 1         | **20 / 1**            | 32 bit Counter, flag                           |
| 22           | 5         | **20 / 1**            | 32 bit Counter Change, flag w/ time            |
| 20           | 2         | **20 / 2**            | 16 bit Counter, flag                           |
| 22           | 6         | **20 / 2**            | 16 bit Counter Change, flag w/ time            |
| 20           | 5         | **20 / 5**            | 32 bit Counter, w/o flag                       |
| 22           | 5         | **20 / 5**            | 32 bit Counter Change, flag w/ time            |
| 20           | 6         | **20 / 6**            | 32 bit Counter, w/o flag                       |
| 22           | 6         | **20 / 6**            | 32 bit Counter Change, flag w/ time            |
| 21           | 1         | **21 / 1**            | 32 bit Frozen Counter, flag                    |
| 23           | 5         | **21 / 1**            | 32 bit Frozen Counter Event, flag w/ time      |
| 21           | 2         | **21 / 2**            | 16 bit Frozen Counter, flag                    |
| 23           | 6         | **21 / 2**            | 16 bit Frozen Counter Event, flag w/ time      |
| 21           | 5         | **21 / 5**            | 32 bit Frozen Counter, flag w/ time            |
| 23           | 5         | **21 / 5**            | 32 bit Frozen Counter Event, flag w/ time      |
| 21           | 6         | **21 / 6**            | 16 bit Frozen Counter, flag w/ time            |
| 23           | 6         | **21 / 6**            | 16 bit Frozen Counter Event, flag w/ time      |
| 10           | 2         | **10 / 2**            | Binary Output, flags                           |
| 11           | 2         | **10 / 2**            | Binary Output Event, w/ time                   |
| 40           | 1         | **40 / 1**            | 32 bit Analog Output Status, flag              |
| 42           | 3         | **40 / 1**            | 32 bit Analog Output Event, w/ time            |
| 40           | 2         | **40 / 2**            | 16 bit Analog Output Status, flag              |
| 42           | 4         | **40 / 2**            | 16 bit Analog Output Event, w/ time            |
| 40           | 3         | **40 / 3**            | Single Precision Analog Output Status, flag    |
| 42           | 7         | **40 / 3**            | Single Precision Analog Output Event, w/ time  |
| 40           | 4         | **40 / 4**            | Double Precision Analog Output Status, flag    |
| 42           | 8         | **40 / 8**            | Double Precision Analog Output Event, w/ time  |
| 110          | 0         | **110 / 0**           | Octet String                                   |
| 111          | 0         | **110 / 0**           | Octet String Event                             |
| 50           | 4         | **50 / 4**            | Time and Interval                              |
| 12           | 1         | **12 / 1**            | CROB (command)                                 |
| 41           | 1         | **41 / 1**            | 32 bit Analog Output Block (command)           |
| 41           | 2         | **41 / 2**            | 16 bit Analog Output Block (command)           |
| 41           | 3         | **41 / 3**            | Single Precision Analog Output Block (command) |
| 41           | 4         | **41 / 4**            | Double Precision Analog Output Block (command) |
       
In the _protocolSourceCommonAddress_ field enter the appropriate value from the column _Common Address_.
For commands, also configure the variation on _protocolSourceASDU_.

For CROB commands use _protocolSourceCommandDuration_ to set command details:

| Operation Type               | protocolSourceCommandDuration |
| ---------------------------- | ----------------------------- |
| UNDEFINED/NUL                | 0                             |
| PULSE 1=ON 0=OFF             | 1                             |
| PULSE 0=ON 1=OFF             | 2                             |
| LATCH 1=ON 0=OFF             | 3                             |
| LATCH 0=ON 1=OFF             | 4                             |
| PULSE 1=ON,CLOSE 0=OFF,TRIP  | 11                            |
| PULSE 1=ON,TRIP 0=OFF,CLOSE  | 21                            |
| LATCH 1=ON,CLOSE 0=OFF,TRIP  | 13                            |
| LATCH 1=ON,TRIP 0=OFF,CLOSE  | 23                            |
| PULSE 1=ON,CLOSE 0=ON,TRIP   | 12                            |
| PULSE 1=ON,TRIP 0=ON,CLOSE   | 22                            |
| PULSE 1=OFF,CLOSE 0=OFF,TRIP | 10                            |
| PULSE 1=OFF,TRIP 0=OFF,CLOSE | 20                            |

## Process Command Line Arguments

This driver has the following command line arguments.

- _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**.
- _**2nd arg. - Log Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**.
- _**3rd arg. - Config File Path/Name**_ [String] - Complete path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**.

## Example of JSON-SCADA Protocol Driver Instances and Connections Numbering

![Driver instances and connections](https://github.com/riclolsen/json-scada/raw/master/docs/JSON-SCADA_Connections.png 'Driver Instances and Connections Numbering')
