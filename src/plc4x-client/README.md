# {json:scada} PLC4X Client (PLC4GO)

A generic PLC client driver for JSON-SCADA. Based on the Apache PLC4X/PLC4GO project.

    https://github.com/apache/plc4x

This driver intends to support all protocols provided by the PLC4GO library: Modbus (TCP/RTU/ASCII), KNXnet, ADS. Upcoming: S7, OPCUA, BACNET, ABETH, CBUS, DF1, EIP, Firmata. However, only Modbus TCP was tested. Any help with testing other protocols is welcome.

Discovery and Subscription features are currently not supported by this driver. 

To configure the driver it is necessary to create one or more driver instances and at least one connection per instance. Also the tags intended to be updated should be configured appropriately. Alternatively, use the autotag feature to create tags automatically.

##  Configure a driver instance

To create a new PLC4X client instance, insert a new document in the _protocolDriverInstances_ collection using the Admin UI or a Mongodb command like below.

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

* _**protocolDriver**_ [String] - Name of the protocol driver, must be  "PLC4X". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
* _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
* _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate node will have the same connections and data enabled for scanning and update. **Mandatory parameter**.
* _**activeNodeName**_ [String] - Name of the protocol driver that is currently active. This is updated by the drivers for redundancy control.**Optional**.
* _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly by the active driver. **Optional**.
* _**keepProtocolRunningWhileInactive**_ [Boolean] - Define a driver will keep the protocol running while not the main active driver. Currently only the _false_ value is supported. **Optional**.

Changes in the _protocolDriverInstances_ config requires that the driver instances processes be restarted to be effective.

## Configure client connections to PLCs

Each instance for this driver can have many client connections defined that must be described in the _protocolConnections_ collection. Create new connections usinng the Admin UI or Mongodb commands like below.

    use json_scada_db_name
    db.protocolConnections.insert({
        protocolDriver: "PLC4X",
        protocolDriverInstanceNumber: 1,
        protocolConnectionNumber: 201,
        name: "MODBUS-PLC1",
        description: "PLC device #1 on MODBUS TCP",
        enabled: true,
        commandsEnabled: true,
        endpointURLs: ["modbus-tcp://192.168.0.101:5001?unit-identifier=1", 
                       "modbus-tcp://192.168.0.102:5001?unit-identifier=1"],
        topics: ["MODBUS_PLC1_REG_1|holding-register:4:UINT|LITTLE_ENDIAN", 
                 "MODBUS_PLC1_REG_20N|holding-register:20:INT[10]"],
        giInterval: 300,
        stats: null
    });

* _**protocolDriver**_ [String] - Name of the protocol driver, must be  "PLC4X". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is be used to define the connection that can update a tag. **Mandatory parameter**.
* _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
* _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
* _**commandsEnabled**_ [Boolean] - Allows to disable commands (messages in control direction) for a connection. Use false here to disable commands. **Mandatory parameter**.
* _**endpointURLs**_ [Array of Strings] - Array of PLC4X formatted PLC URL addresses. When having redundant devices, configure multiple entries. The driver will try to connect with the first device, when disconnected it will switch to the next device in the list. **Mandatory parameter**.
* _**topics**_ [Array of Strings] - Array of PLC tag addresses to be scanned. The format is "TAG_NAME|PLC4X_ADDRESS|ENDIANNESS". A tag name can be provided for automatic creation of tags. See PLC4X docs for the address format. Endianness can be empty (default), LITTLE_ENDIAN, BIG_ENDIAN or REV_ENDIAN (reverse endianness). **Mandatory parameter**.
* _**giInterval**_ [Double] - General station interrogation period in seconds. **Optional parameter**.

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

* _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. Only this protocol connection can update the tag. **Mandatory parameter**.
* _**protocolSourceCommonAddress**_ [String] - Common Address of ASDU. Leave it as null or empty string. **Mandatory parameter**.
* _**protocolSourceObjectAddress**_ [String] - Object address. Use the PLC4X address convention. **Mandatory parameter**.
* _**protocolSourceASDU**_ [String] - Source ASDU TI type. Use to force BIG_ENDIAN, LITTLE_ENDIAN or REV_ENDIAN values. Leave empty for PLC4X default option. **Mandatory parameter**.
* _**protocolSourceCommandDuration**_ [Double] - Use zero here. Just meaningful for commands. **Mandatory parameter**.
* _**protocolSourceCommandUseSBO**_ [Boolean] - Use false here. Just meaningful for commands. **Mandatory parameter**.
* _**kconv1**_ [Double] - Analog conversion factor: multiplier. Use -1 to invert digital values. **Mandatory parameter**.
* _**kconv2**_ [Double] - Analog conversion factor: adder. **Mandatory parameter**.

## Command Line Arguments

This driver has the following command line arguments.

* _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**.
* _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**.
* _**3rd arg. - Config File Path/Name**_ [String] - Complete path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**.

## Example of JSON-SCADA Protocol Driver Instances and Connections Numbering

![Driver instances and connections](https://github.com/riclolsen/json-scada/raw/master/docs/JSON-SCADA_Connections.png "Driver Instances and Connections Numbering")