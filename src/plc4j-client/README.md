# {json:scada} PLC4J Client (PLC4X for Java)

A generic PLC client driver for JSON-SCADA. Based on the Apache PLC4X/PLC4J project (the Java reference implementation of PLC4X).

    https://github.com/apache/plc4x
    https://plc4x.apache.org/users/protocols/modbus.html

**This driver registers in MongoDB as `protocolDriver: "PLC4X"`** — it is an alternative Java implementation of the same driver provided by the Go `plc4x-client`. It uses exactly the same MongoDB configuration (instances, connections, tags). IMPORTANT: only ONE of the two executables (Go `plc4x-client` or Java `plc4j-client`) may run for a given instance number — running both would make them dispute the active role and double-poll devices. To use both side by side, assign them distinct instance numbers.

This driver is built on **Apache PLC4X 1.0.0** and supports the protocols provided by that release: Modbus (TCP/RTU/ASCII), S7 (including s7-light), ADS, OPC UA, KNXnet/IP, EtherNet/IP (and Logix), plus the `simulated` driver for testing. The PLC4J drivers for S7 and EtherNet/IP are more mature than their Go counterparts. Only Modbus TCP was tested with real equipment. Any help with testing other protocols is welcome.

Note: BACnet/IP and C-Bus are **not** available here, as those two drivers were not published in the PLC4X 1.0.0 release (they stop at 0.13.1) and cannot be mixed into a 1.0.0 build. They can be re-enabled in `pom.xml` if a 1.0.x release includes them.

Discovery and Subscription features are currently not supported by this driver.

## Requirements

- JRE or JDK 17+ to run (Temurin/Adoptium suggested).
- JDK 17+ and Apache Maven to build.

## Build

    cd src/plc4j-client
    mvn package

This produces the self-contained `target/plc4j-client.jar`. Copy it to the JSON-SCADA `bin` folder together with the launcher script (`plc4j-client.bat` on Windows, `plc4j-client.sh` on Linux).

## Configure a driver instance

Same configuration as the Go PLC4X client. To create a new instance, insert a new document in the _protocolDriverInstances_ collection using the Admin UI or a Mongodb command like below.

    use json_scada_db_name
    db.protocolDriverInstances.insert({
            protocolDriver: "PLC4X",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: ["mainNode"],
            activeNodeName: "mainNode",
            activeNodeKeepAliveTimeTag: new Date(),
            keepProtocolRunningWhileInactive: false
        });

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "PLC4X". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
- _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
- _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate node will have the same connections and data enabled for scanning and update. **Mandatory parameter**.
- _**activeNodeName**_ [String] - Name of the protocol driver that is currently active. This is updated by the drivers for redundancy control. **Optional**.
- _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly by the active driver. **Optional**.
- _**keepProtocolRunningWhileInactive**_ [Boolean] - Define a driver will keep the protocol running while not the main active driver. Currently only the _false_ value is supported. **Optional**.

Changes in the _protocolDriverInstances_ config requires that the driver instances processes be restarted to be effective.

## Configure client connections to PLCs

Each instance for this driver can have many client connections defined that must be described in the _protocolConnections_ collection. Create new connections using the Admin UI or Mongodb commands like below.

    use json_scada_db_name
    db.protocolConnections.insert({
        protocolDriver: "PLC4X",
        protocolDriverInstanceNumber: 1,
        protocolConnectionNumber: 201,
        name: "MODBUS-PLC1",
        description: "PLC device #1 on MODBUS TCP",
        enabled: true,
        commandsEnabled: true,
        autoCreateTags: true,
        endpointURLs: ["modbus-tcp://192.168.0.101:5001?unit-identifier=1",
                       "modbus-tcp://192.168.0.102:5001?unit-identifier=1"
                      ],
        topics: ["MODBUS_PLC1_REG_1|holding-register:4:UINT|LITTLE_ENDIAN",
                 "MODBUS_PLC1_REG_20N|holding-register:20:INT[10]"
                ],
        giInterval: 300,
        stats: null
    });

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "PLC4X". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is be used to define the connection that can update a tag. **Mandatory parameter**.
- _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
- _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
- _**commandsEnabled**_ [Boolean] - Allows to disable commands (messages in control direction) for a connection. Use false here to disable commands. **Mandatory parameter**.
- _**autoCreateTags**_ [Boolean] - Allows to enable automatic creation of tags. **Mandatory parameter**.
- _**endpointURLs**_ [Array of Strings] - Array of PLC4X formatted PLC URL addresses. When having redundant devices, configure multiple entries. The driver will try to connect with the first device, when disconnected it will switch to the next device in the list. Supported protocol prefixes: "modbus-tcp", "modbus-rtu", "modbus-ascii", "s7", "s7-light", "ads", "opcua", "knxnet-ip", "eip", "logix", "simulated". See PLC4X docs for URL parameters. **Mandatory parameter**.
- _**topics**_ [Array of Strings] - Array of PLC tag addresses to be scanned. The format is "TAG_NAME|PLC4X_ADDRESS|ENDIANNESS". A tag name can be provided for automatic creation of tags. See PLC4X docs for the address format. Endianness can be empty (default), LITTLE_ENDIAN, BIG_ENDIAN or REV_ENDIAN (reverse endianness). **Mandatory parameter**.
- _**giInterval**_ [Double] - General station interrogation period in seconds (when absent or zero, 300s is assumed). **Optional parameter**.

Examples of topics to scan:

- "MODBUS_PLC1_REG_1|holding-register:4:UINT|LITTLE_ENDIAN" - will scan the address 4 holding register, 16 bit unsigned int, little endian, tag will be autocreated as "MODBUS_PLC1_REG_1".
- "MODBUS_PLC1_REG_20N|holding-register:20:INT[10]" - will scan 10 holding registers at address 20, as 16 bit int, default endianness, tags will be named MODBUS_PLC1_REG_20N[0] ... MODBUS_PLC1_REG_20N[9].
- "MODBUS_PLC1_REG_20N|holding-register:20[0..9]:INT" - the same scan written in the PLC4X 1.0.0 array notation.

### Array address notation

PLC4X 1.0.0 changed the array notation: the element count that used to follow the data type became a range placed **before** it, so `holding-register:20:INT[10]` is now written `holding-register:20[0..9]:INT`.

This driver accepts **both** forms. A legacy address is translated to the 1.0.0 form before it is sent to PLC4X, while the text you configured remains the tag identity in `realtimeData` (`protocolSourceObjectAddress`). So existing configurations - including those shared with the Go `plc4x-client`, which still uses the legacy notation - keep working unchanged, and the two executables stay interchangeable. The translation is logged at log level 1 or higher.

Array elements are always mapped to tags by position: an address yielding N values produces `TAG[0]` ... `TAG[N-1]` with object addresses `ADDRESS[0]` ... `ADDRESS[N-1]`, regardless of the lower bound used in a range.

Endianness semantics: PLC4X delivers values interpreted with the protocol default byte order (big-endian for Modbus). Use LITTLE_ENDIAN when the device stores little-endian words (bytes are swapped); REV_ENDIAN is an unconditional byte reversal; BIG_ENDIAN or empty keeps the value as delivered by PLC4X.

## Configure tags for update

Each tag to be update on a connection must have a protocol source set configured.
Only one source connection can update a tag.

Select an existing tag for a update on a connection as below. Or create a new tag in Admin UI with parameters as described below.

    use json_scada_db_name
    db.realtimeData.updateOne({"tag":"A_TAG_NAME"}, {
        $set: {
            protocolSourceConnectionNumber: 201,
            protocolSourceCommonAddress: null,
            protocolSourceObjectAddress: "holding-register:4:UINT",
            protocolSourceASDU: "LITTLE_ENDIAN",
            protocolSourceCommandDuration: 0,
            protocolSourceCommandUseSBO: false,
            kconv1: 1.0,
            kconv2: 0.0
            }
    });

- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. Only this protocol connection can update the tag. **Mandatory parameter**.
- _**protocolSourceCommonAddress**_ [String] - Common Address of ASDU. Ignored: leave it as null or empty string. **Mandatory parameter**.
- _**protocolSourceObjectAddress**_ [String] - Object address. Use the PLC4X address convention. **Mandatory parameter**.
- _**protocolSourceASDU**_ [String] - Source ASDU TI type. Ignored for supervised tags. For commands, use to force BIG_ENDIAN, LITTLE_ENDIAN or REV_ENDIAN values. Leave empty for PLC4X default option. **Mandatory parameter**.
- _**protocolSourceCommandDuration**_ [Double] - Ignored: use zero here. **Mandatory parameter**.
- _**protocolSourceCommandUseSBO**_ [Boolean] - Ignored: use false here. **Mandatory parameter**.
- _**kconv1**_ [Double] - Analog conversion factor: multiplier. Use -1 to invert digital values. **Mandatory parameter**.
- _**kconv2**_ [Double] - Analog conversion factor: adder. **Mandatory parameter**.

## Command Line Arguments

This driver has the following command line arguments.

- _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**.
- _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**.
- _**3rd arg. - Config File Path/Name**_ [String] - Complete path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**.

Environment variables JS_PLC4X_INSTANCE, JS_PLC4X_LOGLEVEL and JS_CONFIG_FILE can also be used (command line arguments have precedence).

Example:

    java -jar plc4j-client.jar 1 1 c:\json-scada\conf\json-scada.json

Or via the launcher script:

    plc4j-client.bat 1 1

## Example of JSON-SCADA Protocol Driver Instances and Connections Numbering

![Driver instances and connections](https://github.com/riclolsen/json-scada/raw/master/docs/JSON-SCADA_Connections.png 'Driver Instances and Connections Numbering')
