# DNP3 Client Protocol Driver

This DNP3 client driver is programmed with C# on Dotnet Core and depends on the OpenDNP3 library Dotnet package that is Windows-only, so it will not run natively on other platforms. The Windows build can possibly run under WINE on x64 Linux, but this is untested.

This driver implements a client for the DNP3 protocol. It can have multiple connections to DNP3 servers on multiple computers, if needed.

To configure the driver it is necessary to create one or more driver instances and at least one connection per instance. Also the tags intended to be updated should be configured appropriately.

## Configure a driver instance

To create a new DNP3 client instance, insert a new document in the _protocolDriverInstances_ collection using a command like this:

    use json_scada_db_name
    db.protocolDriverInstances.insert({
            protocolDriver: "DNP3",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: ["mainNode"],
            activeNodeName: "mainNode",
            activeNodeKeepAliveTimeTag: new Date(),
            keepProtocolRunningWhileInactive: false
        });

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "DNP3". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
- _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
- _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate nodes will have the same connections and data enabled for scanning and update. **Mandatory parameter**.
- _**activeNodeName**_ [String] - Name of the protocol driver that is currently active. This is updated by the drivers for redundancy control.**Optional**.
- _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly by the active driver. **Optional**.
- _**keepProtocolRunningWhileInactive**_ [Boolean] - Define a driver will keep the protocol running while not the main active driver. Currently only the _false_ value is supported. **Optional**.

Changes in the _protocolDriverInstances_ config requires that the driver instances processes be restarted to be effective.

## Configure client connections to DNP3 servers

Each instance for this driver can have many client connection defined that must be described in the _protocolConnections_ collection.
Create a new connection in Admin UI or directly on MongoDB as below.

    use json_scada_db_name
    db.protocolConnections.insert({
        protocolDriver: "DNP3",
        protocolDriverInstanceNumber: 1,
        protocolConnectionNumber: 64,
        name: "KIK1",
        description: "KIK1 Station DNP3",
        enabled: true,
        commandsEnabled: true,
        connectionMode: "TCP Active",
        ipAddressLocalBind: "",
        ipAddresses: ["192.168.0.31:20000"],
        portName: "",
        baudRate: 0,
        parity: "",
        stopBits: "",
        handshake: "",
        asyncOpenDelay: 0,
        localLinkAddress: 1,
        remoteLinkAddress: 2,
        giInterval: 300,
        class0ScanInterval: 30,
        class1ScanInterval: 30,
        class2ScanInterval: 30,
        class3ScanInterval: 30,
        timeSyncMode: 2,
        enableUnsolicited: true,
        rangeScans: [
            {
                group: 1,
                variation: 1,
                startAddress: 10,
                stopAddress: 15,
                period: 120
            },
            {
                group: 2,
                variation: 1,
                startAddress: 0,
                stopAddress: 50,
                period: 150
            },
        ],
        stats: {}
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
- _**giInterval**_ [Double] - General station interrogation period in seconds. **Optional parameter**.
- _**timeSyncMode**_ [Double] - Time sync mode (from client when requested by the RTU server): 0=none, 1=non-lan, 2=lan. Use zero to disable. **Mandatory parameter**.

- _**class0ScanInterval**_ [Double] - Time interval in seconds for class 0 scan. Use zero to disable. **Mandatory parameter**.
- _**class1ScanInterval**_ [Double] - Time interval in seconds for class 1 scan. Use zero to disable. **Mandatory parameter**.
- _**class3ScanInterval**_ [Double] - Time interval in seconds for class 2 scan. Use zero to disable. **Mandatory parameter**.
- _**class4ScanInterval**_ [Double] - Time interval in seconds for class 3 scan. Use zero to disable. **Mandatory parameter**.
- _**enableUnsolicited**_ [Boolean] - Enable (true) or disable (false) unsolicited mode. **Mandatory parameter**.

- _**rangeScans**_ [Array of Objects] - Configure range scans. **Mandatory parameter**.

  - _**group**_ [Double] - Group for a range scan. **Mandatory parameter**.
  - _**variation**_ [Double] - Variation for a range scan. **Mandatory parameter**.
  - _**startAddress**_ [Double] - Start address for a range scan. **Mandatory parameter**.
  - _**stopAddress**_ [Double] - Stop address for a range scan. **Mandatory parameter**.
  - _**period**_ [Double] - Period in seconds for a range scan. **Mandatory parameter**.

- _**stats**_ [Object] - Protocol statistics updated by the driver.
- _**connectionMode**_ [String] - Connection mode: 'TCP Active'|'TCP Passive'|'TLS Active'|'TLS Passive'|'UDP'|'Serial'.

For TCP communication.

- _**ipAddressLocalBind**_ [String] - Local bind address for passive option. Not used for TCP active option (leave empty). **Mandatory parameter**.
- _**ipAddresses**_ [Array of Strings] - Array of IP addresses and ports for DNP3 servers to be scanned (only the first server is currently supported). **Mandatory parameter**.

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

- _**ipAddressLocalBind**_ [String] - Local bind address for passive option. Not used for TCP active option (leave empty). **Mandatory parameter**.
- _**ipAddresses**_ [Array of Strings] - Array of IP addresses and ports for DNP3 servers to be scanned (only the first server is currently supported). **Mandatory parameter**.
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

## Configure tags for update

Each tag to be update on a connection must have a protocol source set configured.
Only one source connection can update a tag.

Select a tag for a update on a connection as below.

    use json_scada_db_name
    db.realtimeData.updateOne({"tag":"A_TAG_NAME"}, {
        $set: {
            protocolSourceConnectionNumber: 64,
            protocolSourceCommonAddress: 1,
            protocolSourceObjectAddress: 0,
            protocolSourceASDU: 0,
            protocolSourceCommandDuration: 0,
            protocolSourceCommandUseSBO: false,
            kconv1: 1,
            kconv2: 0
            }
    });

- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. Only this protocol connection can update the tag. **Mandatory parameter**.
- _**protocolSourceCommonAddress**_ [Double] - DNP3 Group code (see below). **Mandatory parameter**.
- _**protocolSourceObjectAddress**_ [Double] - Object address. This address combined with _protocolSourceCommonAddress_ must be unique for the connection. **Mandatory parameter**.
- _**protocolSourceASDU**_ [Double] - For commands, this is the variation to be used. For supervised points, this parameter is ignored. **Mandatory parameter**.
- _**protocolSourceCommandDuration**_ [Double] - Command options. Just meaningful for commands. **Mandatory parameter**.
- _**protocolSourceCommandUseSBO**_ [Double] - Use Select-Before-Operate control sequence. Just meaningful for commands. **Mandatory parameter**.
- _**kconv1**_ [Double] - Analog conversion factor: multiplier. Use -1 to invert digital values. **Mandatory parameter**.
- _**kconv2**_ [Double] - Analog conversion factor: adder. **Mandatory parameter**.

## Supported DNP3 Group/Variations and Codes for Common Address Parameter

| Object Group | Variations        | Common Address | Description                    |
| ------------ | ----------------- | -------------- | ------------------------------ |
| 1            | 0,1,2             | **1**          | Binary Input                   |
| 2            | 0,1,2,3           | **1**          | Binary Input Change            |
| 3            | 0,1,2             | **3**          | Double Binary Input            |
| 4            | 0,1,2,3           | **3**          | Double Binary Input Change     |
| 30           | 0,1,2,3,4,5,6     | **30**         | Analog Input                   |
| 32           | 0,1,2,3,4,5,6,7,8 | **30**         | Analog Input Change            |
| 20           | 0,1,2,5,6         | **20**         | Counter                        |
| 22           | 0,1,2,5,6         | **20**         | Counter Change                 |
| 21           | 0,1,2,5,6,9,10    | **21**         | Frozen Counter                 |
| 23           | 0,1,2,5,6         | **21**         | Frozen Counter Event           |
| 10           | 0,1,2             | **10**         | Binary Output                  |
| 11           | 0,1,2             | **10**         | Binary Output Event            |
| 12           | 0,1               | **12**         | CROB (commands)                |
| 40           | 0,1,2,3,4         | **40**         | Analog Output Status           |
| 42           | 0,1,2,3,4         | **40**         | Analog Output Event            |
| 41           | 0,1,2,3,4         | **41**         | Analog Output Block (commands) |
| 43           | 1,2               | **43**         | Analog Command Event           |
| 13           | 1,2               | **13**         | Binary Command Event           |
| 110          | 0                 | **110**        | Octet String                   |
| 111          | 0                 | **110**        | Octet String Event             |
| 50           | 1,3,4             | **50**         | Time and Interval              |

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

Currently, pulse CROB controls have a fixed time of 100ms hardcoded in the source code.

## Firing Commands Directly via MongoDB

Is is possible to fire commands over a running protocol connection just by inserting documents like below into the _commandsQueue_ collection.

    {
        "protocolSourceConnectionNumber": 1.0,          // [double]  protocolConnectionNumber of the scanned dnp3 device
        "protocolSourceCommonAddress": 12.0,            // [double]  DNP3 object group of command: 12.0 (CROB) or 41.0 (analog)
        "protocolSourceObjectAddress": 1.0,             // [double]  DNP3 object address: 0.0 ... 65535.0
        "protocolSourceASDU": 0.0,                      // [double]  DNP3 variation: 0.0 ... 4.0
        "protocolSourceCommandDuration": 1.0,           // [double]  Operation Type for CROB 0.0 ... 23.0 see table above, use 0.0 if analog
        "protocolSourceCommandUseSBO": false,           // [boolean] Use SBO
        "pointKey": 0.0,                                // [double]  _id of command point, put 0.0 if not created a command point
        "tag": "",                                      // [string]  Tag of command point, leave empty if not created a command point
        "timeTag": new Date(),                          // [date]    Must put current date/time here, the command will expire in 10 seconds
        "value": 1.0                                    // [double]  Command value, 0.0 or 1.0 for digital
        "valueString": "PULSE 1=ON",                    // [string]  Not used by this driver, just document value as a string
        "originatorUserName": "Protocol connection: 1", // [string]  Just document the originator of command
        "originatorIpAddress": "127.0.0.1:58446"        // [string]  Just document the originator IP
    }

The DNP3 client driver will consume each inserted document and will fire a command in the protocol connection.

## Process Command Line Arguments

This driver has the following command line arguments.

- _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**.
- _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**.
- _**3rd arg. - Config File Path/Name**_ [String] - Complete path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**.

## Example of JSON-SCADA Protocol Driver Instances and Connections Numbering

![Driver instances and connections](https://github.com/riclolsen/json-scada/raw/master/docs/JSON-SCADA_Connections.png 'Driver Instances and Connections Numbering')
