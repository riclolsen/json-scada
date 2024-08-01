# OPC-UA Server

This driver implements a server for the OPC-UA protocol (binary transport only, opc.tcp://hostname:port/resourcePath).

Implemented using the Node OPC-UA library.

    https://github.com/node-opcua/node-opcua

The driver can serve multiple connections to OPC-UA clients on multiple computers, if needed.

To configure the driver it is necessary to create one or more driver instances and one protocol connection per instance.

## Configure a driver instance

To create a new OPC-UA client instance, insert a new document in the _protocolDriverInstances_ collection using a command like this:

    use json_scada_db_name
    db.protocolDriverInstances.insert({
            protocolDriver: "OPC-UA_SERVER",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: [],
        });

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "OPC-UA". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
- _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
- _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Leave empty to allow any node to run this driver. **Mandatory parameter**.

Changes in the _protocolDriverInstances_ config requires that the driver instances processes be restarted to be effective.

## Configure client connections to OPC-UA servers

Each instance for this driver can have many client connection defined that must be described in the _protocolConnections_ collection.

This driver will make all points available to the clients, unless filtered. There is no need to configure tags for protocol destinations (_protocolDestinations_ property).

    use json_scada_db_name
    db.protocolConnections.insert({
        protocolDriver: "OPC-UA_SERVER",
        protocolDriverInstanceNumber: 1.0,
        protocolConnectionNumber: 81.0,
        name: "OPCUAServer",
        description: "OPC-UA Server",
        enabled: true,
        commandsEnabled: true,
        groupId: "UA/JsonScada",
        ipAddressLocalBind: "0.0.0.0:4840",
        ipAddresses: ["192.168.1.1"],
        topics: ["KAW2", "KOR1"],
        timeoutMs: 15000,
        useSecurity: false,
        localCertFilePath: "",
        privateKeyFilePath: "",
        stats: {}
    });

Parameters for communication with OPC-UA servers.

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "OPC-UA_SERVER". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is be used to define the connection that can update a tag. **Mandatory parameter**.
- _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
- _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
- _**commandsEnabled**_ [Boolean] - Allows to disable/enable commands (messages in control direction) for a connection. Use false to disable all commands. If true the driver will create writable command tags for the enabled topics (_group1_ list). **Mandatory parameter**.
- _**groupId**_ [String] - OPC-UA resource path. This path will be added to the endpoint resource name. Default value is "UA/JsonScada". **Optional parameter**.
- _**ipAddresses**_ [Array of Strings] - List of client's IP addresses allowed. Leave empty to allow any IP address to connect to the server. **Optional parameter**.
- _**ipAddressLocalBind**_ [String] - Interface bind IP address and port. Currently supports only IP "0.0.0.0". Default "0.0.0.0:4840". **Optional parameter**.
- _**topics**_ [Array of Strings] - List of _group1_ filter for the available tags on the OPC-UA server. Leave empty to include all tags. **Optional parameter**.
- _**timeoutMs**_ [Double] - Timeout. The HEL/ACK transaction timeout in ms. Use a large value (i.e. 15000 ms) for slow connections or embedded devices. **Mandatory parameter**.
- _**useSecurity**_ [Boolean] - Use (true) or not (false) secure encrypted connection. **Mandatory parameter**.
- _**localCertFilePath**_ [String] - File that contains the certificate (\*.PEM) that will be presented to the remote side of the connection (equiv. to NodeJS TLS option 'cert'). **Optional parameter**.
- _**privateKeyFilePath**_ [String] - File (\*.PEM) that contains the private key corresponding to the local certificate (equiv. to NodeJS TLS option 'key'). **Optional parameter**.
- _**stats**_ [Object] - Protocol statistics updated by the driver. **Mandatory parameter**.

## Command Line Arguments

This driver has the following command line arguments.

- _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**.
- _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**.
- _**3rd arg. - Config File Path/Name**_ [String] - Complete path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**.

## Example of JSON-SCADA Protocol Driver Instances and Connections Numbering

![Driver instances and connections](https://github.com/riclolsen/json-scada/raw/master/docs/JSON-SCADA_Connections.png 'Driver Instances and Connections Numbering')
