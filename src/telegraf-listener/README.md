# Telegraf UDP JSON Listener Driver

This client driver listen on a UDP port for JSON formatted data coming from Telegraf sources.

Telegraf is an incredibly powerful tool that can collect data from diverse sources like application metrics (MongoDB, Nginx, etc.) and protocols like SNMP, OPC-UA, MQTT and Modbus.

Telegraf output plugin (socket_writer) works in **monitoring direction only**, i.e. **this driver can not send controls**.

Telegraf must be configured for UDP output with JSON format (socket_writer output).

    [[outputs.socket_writer]]
      address = "udp://127.0.0.1:51920"
      data_format = "json"

See Telegraf documentation for more information.
https://github.com/influxdata/telegraf

##  Configure a driver instance

To create a new _TELEGRAF-LISTENER_ instance, use the Admin UI or insert a new document in the _protocolDriverInstances_ collection using a command like this:

    use json_scada_db_name
    db.protocolDriverInstances.insert({
            protocolDriver: "TELEGRAF-LISTENER",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: ["mainNode"], 
            activeNodeName: "mainNode",
            activeNodeKeepAliveTimeTag: new Date(),
        });

* _**protocolDriver**_ [String] - Name of the protocol driver, must be "TELEGRAF-LISTENER". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
* _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
* _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate nodes will have the same connections and data enabled for scanning and update. **Mandatory parameter**.
* _**activeNodeName**_ [String] - Name of the protocol driver that is currently active. This is updated by the drivers for redundancy control.**Optional**.
* _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly  by the active driver. **Optional**.

## Configure client connections to DNP3 servers

Each instance for this driver can have just one connection defined that must be described in the _protocolConnections_ collection.

    use json_scada_db_name
    db.protocolConnections.insert({
        protocolDriver: "TELEGRAF-LISTENER",
        protocolDriverInstanceNumber: 1,
        protocolConnectionNumber: 1100,
        name: "TELEGRAF",
        description: "TELEGRAF Demo",
        enabled: true,
        autoCreateTags: true,
        commandsEnabled: true,
        ipAddressLocalBind: "0.0.0.0:51920", 
        ipAddresses: ["127.0.0.1"],
    });

* _**protocolDriver**_ [String] - Name of the protocol driver, must be  "DNP3". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is be used to define the connection that can update a tag. **Mandatory parameter**.
* _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
* _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
* _**autoCreateTags**_ [Boolean] - Enables automatic creation of all discovered tags. **Mandatory parameter**.
* _**ipAddressLocalBind**_ [String] - Address and port to bind for listening UDP messages. **Mandatory parameter**.
* _**ipAddresses**_ [Array of Strings] - Restrict IP address sources of data allowed. **Mandatory parameter**.
