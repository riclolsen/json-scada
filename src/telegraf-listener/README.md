# {json:scada} Telegraf UDP JSON Listener Driver

This client driver listens on a UDP port for JSON-formatted data coming from
Telegraf sources.

Telegraf is an incredibly powerful tool that can collect data from diverse sources like application metrics (MongoDB, Nginx, etc.) and protocols like SNMP, OPC-UA, MQTT and Modbus.

The Telegraf output plugin (socket_writer) works in **monitoring direction only**, i.e. **this driver cannot send controls**.

Requires Node.js.

## Run

    cd src/telegraf-listener
    npm install
    node index.js [instance number] [log level] [config file]

Arguments and environment variables:

- **1st arg. — Instance Number** [Integer] — instance to run. Default `1`. Env: `JS_TELEGRAF_LISTENER_INSTANCE`.
- **2nd arg. — Log Level** [Integer] — `0` minimum, `1` basic, `2` detailed, `3` debug. Default `1`. Env: `JS_TELEGRAF_LISTENER_LOGLEVEL`.
- **3rd arg. — Config File** [String] — JSON-SCADA config file. Default `../../conf/json-scada.json`. Env: `JS_CONFIG_FILE`.

Additional environment variables override the defaults before the connection is
read from the database:

- `JS_TELEGRAF_LISTENER_BIND_ADDRESS` — default `0.0.0.0`.
- `JS_TELEGRAF_LISTENER_BIND_PORT` — default `51920`.

Command-line arguments take precedence over environment variables.

## Telegraf configuration

Telegraf must be configured for UDP output with JSON format (socket_writer output).

    [[outputs.socket_writer]]
      address = "udp://127.0.0.1:51920"
      data_format = "json"

See Telegraf documentation for more information.
https://github.com/influxdata/telegraf

## Configure a driver instance

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

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "TELEGRAF-LISTENER". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes it possible to run multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
- _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
- _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate nodes will have the same connections and data enabled for scanning and update. **Mandatory parameter**.
- _**activeNodeName**_ [String] - Name of the protocol driver that is currently active. This is updated by the drivers for redundancy control.**Optional**.
- _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly by the active driver. **Optional**.

## Configure client connections to Telegraf sources

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
        ipAddressLocalBind: "0.0.0.0:51920",
        ipAddresses: ["127.0.0.1"],
    });

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "TELEGRAF-LISTENER". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes it possible to run multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is used to define the connection that can update a tag. **Mandatory parameter**.
- _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
- _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
- _**autoCreateTags**_ [Boolean] - Enables automatic creation of all discovered tags. **Mandatory parameter**.
- _**ipAddressLocalBind**_ [String] - Address and port to bind for listening UDP messages. It overrides `JS_TELEGRAF_LISTENER_BIND_ADDRESS` / `JS_TELEGRAF_LISTENER_BIND_PORT` when non-empty. **Mandatory parameter**.
- _**ipAddresses**_ [Array of Strings] - Restrict IP address sources of data allowed. Leave the array empty to accept messages from any source. **Mandatory parameter**.

Notice: the _ipAddresses_ filter is based on the UDP source address, which can be spoofed. It is a convenience filter only; for real isolation, restrict access to the UDP port with a firewall and prefer binding to a specific interface instead of "0.0.0.0".

If `autoCreateTags` is enabled, incoming metric names that do not match an
existing point are created in `realtimeData` with `_id` values allocated in the
range `protocolConnectionNumber × 100000`.

## Example of JSON-SCADA Protocol Driver Instances and Connections Numbering

![Driver instances and connections](https://github.com/riclolsen/json-scada/raw/master/docs/JSON-SCADA_Connections.png 'Driver Instances and Connections Numbering')
