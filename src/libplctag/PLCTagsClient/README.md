# PLC Tag Client Driver (CIP Ethernet/IP, Modbus TCP)

This driver implements a client for the CIP Ethernet/IP protocol for the following PLCs:

- Rockwell/Allen-Bradley ControlLogix
- Rockwell/Allen-Bradley MicroLogix 850
- Rockwell/Allen-Bradley PLC5, SLC 500, MicroLogix.
- Omron NX/N
- Allen-Bradley Micro80

This driver uses the _libplctag.NET_ library that is currently in alpha stage, be careful!

This driver eventually will also support the Modbus TCP protocol.

This driver is based on the [libplctag/libplctag.NET](https://github.com/libplctag/libplctag.NET) project.

The driver can have multiple connections to PLC servers on multiple computers, if needed.
To configure the driver it is necessary to create one or more driver instances and at least one connection per instance. Also the tags intended to be updated should be configured appropriately.

## Configure a driver instance

To create a new PLCTAG client instance, insert a new document in the _protocolDriverInstances_ collection using a command like this:

    use json_scada_db_name
    db.protocolDriverInstances.insert({
            protocolDriver: "PLCTAG",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: ["mainNode"],
            activeNodeName: "mainNode",
            activeNodeKeepAliveTimeTag: new Date(),
            keepProtocolRunningWhileInactive: false
        });

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "PLCTAG". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
- _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
- _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate nodes will have the same connections and data enabled for scanning and update. **Mandatory parameter**.
- _**activeNodeName**_ [String] - Name of the protocol driver that is currently active. This is updated by the drivers for redundancy control.**Optional**.
- _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly by the active driver. **Optional**.
- _**keepProtocolRunningWhileInactive**_ [Boolean] - Define a driver will keep the protocol running while not the main active driver. Currently only the _false_ value is supported. **Optional**.

Changes in the _protocolDriverInstances_ config requires that the driver instances processes be restarted to be effective.

## Configure client connections to PLCs

Each instance for this driver can have many client connection defined that must be described in the _protocolConnections_ collection.

    use json_scada_db_name
    db.protocolConnections.insert({
        protocolDriver: "PLCTAG",
        protocolDriverInstanceNumber: 1,
        protocolConnectionNumber: 81,
        name: "PLC1",
        description: "PLC1 - PLCTAG",
        enabled: true,
        commandsEnabled: true,
        ipAddressLocalBind: "",
        ipAddresses: ["127.0.0.1"],
        localLinkAddress: 1,
        remoteLinkAddress: 2,
        giInterval: 500,
        protocol: "ab_eip",
        plc: "controllogix",
        useConnectedMsg: true,
        readCacheMs: 100,
        timeoutMs: 1000,
        int16ByteOrder: "01",
        int32ByteOrder: "0123",
        int64ByteOrder: "01234567",
        float32ByteOrder: "0123",
        float64ByteOrder: "01234567",
        stats: {}
    });

Common parameters for _CIP Ethernet/IP_ and _Modbus TCP_ communication.

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "DNP3". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is be used to define the connection that can update a tag. **Mandatory parameter**.
- _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
- _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
- _**commandsEnabled**_ [Boolean] - Allows to disable commands (messages in control direction) for a connection. Use false here to disable commands. **Mandatory parameter**.
- _**ipAddressLocalBind**_ [String] - IP address and port of local UCP endpoint. **Mandatory parameter**.
- _**ipAddresses**_ [Array of Strings] - Array of IP addresses and ports for DNP3 servers to be scanned (only the first server is currently supported). **Mandatory parameter**.
- _**localLinkAddress**_ [Double] - Local link address for the connection (originator address). **Mandatory parameter**.
- _**remoteLinkAddress**_ [Double] - Remote link address of the server (originator address). **Optional parameter**.
- _**giInterval**_ [Double] - PLC interrogation interval in milliseconds. **Optional parameter**.
- _**timeSyncMode**_ [Double] - Time sync mode (from client when requested by the RTU server): 0=none, 1=non-lan, 2=lan. Use zero to disable. **Mandatory parameter**.
- _**stats**_ [Object] - Protocol statistics updated by the driver. **Mandatory parameter**.
- _**protocol**_ [String] - Protocol "ab_eip" or "modbus". **Mandatory parameter**.
- _**plc**_ [String] - PLC type: controllogix | logixpccc | micrologix800 | omronnjnx | micrologix |plc5 | slc500. **Mandatory parameter**.
- _**useConnectedMsg**_ [Boolean] - Use or not connected messages method. **Mandatory parameter**.
- _**readCacheMs**_ [Double] - Use this attribute to cause the tag read operations to cache data the requested number of milliseconds. This can be used to lower the actual number of requests against the PLC. **Mandatory parameter**.
- _**timeoutMs**_ [Double] - Timeout for read/write/init operations. **Mandatory parameter**.
- _**int16ByteOrder**_ [String] - Reserved parameter. **Optional parameter**.
- _**int32ByteOrder**_ [String] - Reserved parameter. **Optional parameter**.
- _**int64ByteOrder**_ [String] - Reserved parameter. **Optional parameter**.
- _**float32ByteOrder**_ [String] - Reserved parameter. **Optional parameter**.
- _**float64ByteOrder**_ [String] - Reserved parameter. **Optional parameter**.

### Multi-drop

In the Modbus TCP multi-drop case, multiple slave devices share the same TCP, UDP, TLS or Serial channel connection.

For a multi-drop configuration, use a new connection (in the _protocolConnections_ collection) for each device repeating channel specification (endpoint IP address or serial port). Each device on a shared channel must have a distinct _remoteLinkAddress_ parameter.

## Configure JSON-SCADA tags for update (read from PLC)

Each tag to be update on a connection must have a protocol source set configured. Only one source connection can update a tag.

Select a tag for a update on a connection as below.

    use json_scada_db_name
    db.realtimeData.updateOne({"tag":"A_TAG_NAME"}, {
        $set: {
            protocolSourceConnectionNumber: 81,
            protocolSourceCommonAddress: "1,0", // path
            protocolSourceObjectAddress: "MyPlcTag",
            protocolSourceASDU: "dint", // data type
            kconv1: 1,
            kconv2: 0
            }
    });

- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. Only this protocol connection can update the tag. **Mandatory parameter**.
- _**protocolSourceCommonAddress**_ [Double] - Plc Path or Modbus Slave ID. **Mandatory parameter**.
- _**protocolSourceObjectAddress**_ [Double] - Tag name or modbus address (co42). When the PLC Tag is an array, use the "TagName[element_pos]" notation to identify which element to read. It is possible to read a full array into a JSON-SCADA tag (as a JSON array) just by omitting the element position. This address combined with _protocolSourceCommonAddress_ must be unique for the connection. **Mandatory parameter**.
- _**protocolSourceASDU**_ [Double] - Data type: bool | sint | int | dint | lint | real | lreal. In case of arrays, indicate the size as in "dint[10]". **Mandatory parameter**.
- _**kconv1**_ [Double] - Analog conversion factor: multiplier. Use -1 to invert digital values. **Mandatory parameter**.
- _**kconv2**_ [Double] - Analog conversion factor: adder. **Mandatory parameter**.

## Command Line Arguments

This driver has the following command line arguments.

- _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**.
- _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**.
- _**3rd arg. - Config File Path/Name**_ [String] - Complete path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**.

## Example of JSON-SCADA Protocol Driver Instances and Connections Numbering

![Driver instances and connections](https://github.com/riclolsen/json-scada/raw/master/docs/JSON-SCADA_Connections.png 'Driver Instances and Connections Numbering')
