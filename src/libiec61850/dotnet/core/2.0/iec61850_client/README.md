# IEC61850 Client

This driver implements a client for the IEC61850 protocol.

This driver uses the MZ-Automation's libiec61850 with .NET 6.0 target platform.

    https://github.com/mz-automation/libiec61850

The driver can have multiple connections to IEC61850 servers (IEDs).

To configure the driver it is necessary to create one or more driver instances and at least one connection per instance, this can be accomplished using the Web interface (Admin UI) or programaticallly as below. Also the tags intended to be updated should be configured appropriately. The AutoCreateTags feature can be used to create all tags for reports automatically (this can be overkill, it will create lots of unecessary data and traffic).


##  Configure a driver instance

To create a new IEC61850 client instance, insert a new document in the _protocolDriverInstances_ collection using a Mongodb command like this:

    use json_scada_db_name
    db.protocolDriverInstances.insert({
            protocolDriver: "IEC61850",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: ["mainNode"], 
            activeNodeName: "mainNode",
            activeNodeKeepAliveTimeTag: new Date(),
            keepProtocolRunningWhileInactive: false
        });

* _**protocolDriver**_ [String] - Name of the protocol driver, must be "IEC61850". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
* _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
* _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate nodes will have the same connections and data enabled for scanning and update. **Mandatory parameter**.
* _**activeNodeName**_ [String] - Name of the protocol driver that is currently active. This is updated by the drivers for redundancy control.**Optional**.
* _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly  by the active driver. **Optional**.
* _**keepProtocolRunningWhileInactive**_ [Boolean] - Define a driver will keep the protocol running while not the main active driver. Currently only the _false_ value is supported. **Optional**.

Changes in the _protocolDriverInstances_ config requires that the driver instances processes be restarted to be effective.

## Configure client connections to IEC61850 servers

Each instance for this driver can have many client connection defined that must be described in the _protocolConnections_ collection.

    use json_scada_db_name
    db.protocolConnections.insert({
        protocolDriver: "IEC61850",
        protocolDriverInstanceNumber: 1.0,
        protocolConnectionNumber: 101.0,
        name: "IED1",
        description: "IED1 - IEC61850",
        enabled: true,
        commandsEnabled: true,
        ipAddresses: ["192.168.0.10:102"],
        topics: ["DemoMeasurement/LLN0.RP.urcb01", "DemoMeasurement/LLN0.RP.urcb02"],
        autoCreateTags: true,
        timeoutMs: 20000,
        giInterval: 300,
        class0ScanInterval: 300,
        useSecurity: false,
    });

Parameters for communication with IEC61850 servers.
* _**protocolDriver**_ [String] - Name of the protocol driver, must be "IEC61850". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is be used to define the connection that can update a tag. **Mandatory parameter**.
* _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
* _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
* _**commandsEnabled**_ [Boolean] - Allows to disable commands (messages in control direction) for a connection. Use false here to disable commands. **Mandatory parameter**.
* _**ipAddresses**_ [Array of Strings] - Array of server IP addresses (or hostnames) and TCP ports  (only the first server is currently supported). **Mandatory parameter**.
* _**topics**_ [Array of Strings] - Array of report names to be activated (will activate all if none was specified). *Mandatory parameter**.
* _**autoCreateTags**_ [Boolean] - When true the driver will auto create tags for every data point found in activated reports in the server. When false, only preconfigured tags will be updated. **Mandatory parameter**.
* _**giInterval**_ [Double] - Scan interval in seconds for data not in reports. **Mandatory parameter**.
* _**class0ScanInterval**_ [Double] - Integrity interval in seconds for data in reports. **Mandatory parameter**.
* _**useSecurity**_ [Boolean] - Use (true) or not (false) secure encrypted connection. **Mandatory parameter**.

## Configure JSON-SCADA tags for update (reading from an IEC61850 Server)

Each tag to be update on a connection must have a protocol source configured. Only one source connection can update a tag.

Select a tag for a update on a connection as below.

    use json_scada_db_name
    db.realtimeData.updateOne({"tag":"Demo.Dynamic.Scalar.StatusCode"}, {
        $set: {
            protocolSourceConnectionNumber: 101.0,
            protocolSourceCommonAddress: "ST",
            protocolSourceObjectAddress: "DemoProtCtrl/Obj1XCBR1.Pos",
            protocolSourceASDU: "", 
            kconv1: 1.0,
            kconv2: 0.0
            }
    });

* _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. Only this protocol connection can update the tag. **Mandatory parameter**.
* _**protocolSourceCommonAddress**_ [String] - Functional contraint (ST, MX, CF, etc.). **Mandatory parameter**.
* _**protocolSourceObjectAddress**_ [String] -  IEC61850 element address. This address must be unique in a connection (for supervised points). **Mandatory parameter**.
* _**protocolSourceASDU**_ [String] - Unused. **Optional parameter**.
* _**kconv1**_ [Double] - Analog conversion factor: multiplier. Use -1 to invert digital values. **Mandatory parameter**.
* _**kconv2**_ [Double] - Analog conversion factor: adder. **Mandatory parameter**.

## Configure JSON-SCADA command tags (writing to an IEC61850 Server)

Create a regular command tag. Configure the connection number, IEC61850 object id (object address) and FC (common address).

    use json_scada_db_name
    db.realtimeData.updateOne({"tag":"a_command_tag"}, {
        $set: {
            protocolSourceConnectionNumber: 101.0,
            protocolSourceCommonAddress: "CO",
            protocolSourceObjectAddress: "DemoProtCtrl/Obj1CSWI1.Pos",
            protocolSourceASDU: "", 
            kconv1: 1.0,
            kconv2: 0.0
            }
    });

* _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. Only this protocol connection can command this tag. **Mandatory parameter**.
* _**protocolSourceCommonAddress**_ [String] - Functional contraint ("CO" for control block, other FCs will generate a simple MMS write). **Mandatory parameter**.
* _**protocolSourceObjectAddress**_ [String] - IEC61850 element address. This address must be unique in a connection (for commands). **Mandatory parameter**.
* _**protocolSourceASDU**_ [String] - unused.  **Optional parameter**.
* _**kconv1**_ [Double] - Analog conversion factor: multiplier. Use -1 to invert digital values. **Mandatory parameter**.
* _**kconv2**_ [Double] - Analog conversion factor: adder. **Mandatory parameter**.

## Command Line Arguments

This driver has the following command line arguments.

* _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**.
* _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**.
* _**3rd arg. - Config File Path/Name**_ [String] - Complete path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**.

## Example of JSON-SCADA Protocol Driver Instances and Connections Numbering

![Driver instances and connections](https://github.com/riclolsen/json-scada/raw/master/docs/JSON-SCADA_Connections.png "Driver Instances and Connections Numbering")
