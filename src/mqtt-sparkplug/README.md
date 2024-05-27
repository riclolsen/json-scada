# MQTT Sparkplug-B Driver

This client driver connects to a MQTT Broker and can

* Act as a Sparkplug Primary (or non-primary) SCADA host.
* Subscribe to Sparkplug B devices, create and update tags with Sparkplug metrics.
* Subscribe to regular MQTT topics, create and updated tags with topics with auto-detected payloads like number, string, boolean, JSON, relaxed JSON.
* Subscribe to regular binary MQTT topics to be saved as files on MongoDB-Gridfs.
* Publish tags as regular MQTT topics.
* Publish tags as a Sparkplug B device.

This driver is based on the Eclipse Tahu Javascript Sparkplug B Client library.

* https://github.com/eclipse/tahu

Here are some nice introduction about MQTT/Sparkplug-B.

* https://www.mbtmag.com/best-practices/article/21172575/how-to-integrate-automation-data-with-mqttsparkplug-b
* https://www.linkedin.com/pulse/mqtt-sparkplug-what-marriage-all-kudzai-manditereza

Official MQTT and Sparkplug B Specifications

* https://mqtt.org/mqtt-specification/
* https://www.eclipse.org/tahu/spec/Sparkplug%20Topic%20Namespace%20and%20State%20ManagementV2.2-with%20appendix%20B%20format%20-%20Eclipse.pdf

##  Configure a driver instance

To create a new _MQTT-SPARKPLUG-B_ driver instance, use the Admin UI or insert a new document in the _protocolDriverInstances_ collection using a command like this:

    // be sure to be in the right database, normally json_scada
    // uncomment and adjust the next line as necessary
    // use json_scada
    db.protocolDriverInstances.insert({
            protocolDriver: "MQTT-SPARKPLUG-B",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: ["mainNode"], 
            activeNodeName: "mainNode",
            activeNodeKeepAliveTimeTag: new Date(),
        });

* _**protocolDriver**_ [String] - Name of the protocol driver, must be "MQTT-SPARKPLUG-B". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
* _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
* _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate nodes will have the same connections and data enabled for scanning and update. **Mandatory parameter**.
* _**activeNodeName**_ [String] - Name of the protocol driver that is currently active. This is updated by the drivers for redundancy control.**Optional**.
* _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly  by the active driver. **Optional**.

## Configure client connection to MQTT broker

Each instance for this driver can have just one connection defined that must be described in the _protocolConnections_ collection.

    // be sure to be in the right database, normally json_scada
    // uncomment and adjust the next line as necessary
    // use json_scada
    db.protocolConnections.insert({
        protocolDriver: "MQTT-SPARKPLUG-B",
        protocolDriverInstanceNumber: 1,
        protocolConnectionNumber: 1200,
        name: "MQTT-BROKER",
        description: "MQTT Demo",
        enabled: true,
        commandsEnabled: true,
        autoCreateTags: true,
        endpointURLs: ["mqtt://broker.hivemq.com:1883"],
        topics: ["spBv1.0/#"],
        topicsAsFiles: ["docs/#"],
        topicsScripted: [{topic: "sensors/sensor1", script: "shared.dataArray = []; vals = JSON.parse(shared.payload.toString()); cnt = 1; vals.forEach(elem => { shared.dataArray.push({ id: 'scrVal' + cnt, value: elem, qualityOk: true, timestamp: new Date().getTime() }); cnt++; });"}],
        clientId: "JSON-SCADA Server 1",
        groupId: "Sparkplug B Devices",
        edgeNodeId: "JSON-SCADA Server",
        deviceId: "JSON-SCADA Device",
        scadaHostId : "Primary Application",
        publishTopicRoot: "EnterpriseName",
        username: "",
        password: "",
        useSecurity: false,
        chainValidation: false,
        rootCertFilePath: "",
        localCertFilePath: "",
        privateKeyFilePath: "",
        pfxFilePath: "",
        passphrase: "",
        allowTLSv10: false,
        allowTLSv11: false,
        allowTLSv12: true,
        allowTLSv13: true,
        cipherList: "",
    });

* _**protocolDriver**_ [String] - Name of the protocol driver, must be "MQTT-SPARKPLUG-B". **Mandatory parameter**.
* _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
* _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is be used to define the connection that can update a tag. **Mandatory parameter**.
* _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
* _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
* _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
* _**commandsEnabled**_ [Boolean] - Allows to disable commands (publishing messages in control direction) for a connection. Use false here to disable commands. **Mandatory parameter**.
* _**autoCreateTags**_ [Boolean] - Enables automatic creation of all discovered tags. **Mandatory parameter**.
* _**endpointURLs**_ [Array of Strings] - List of URLS for connection to MQTT brokers. Use more than one broker only in case of redundant brokers. **Mandatory parameter**.
* _**topics**_ [Array of Strings] - List of topics to subscribe on MQTT broker. Sparkplug B devices publish to "spBv1.0/#". Sparkplug B metrics will be converted as tags with topic name (minus "spBv1.0" root and message type) plus metric name. JSON Path Plus can be used to extract values from MQTT JSON payloads, add a JSONPath query to the topic string, e.g. "root_topic/subtopic/$.property1" this will extract the value of _property1_ from the JSON payload. Regular MQTT topics will be converted to tags with the full topic name as the object address (including JSONPath, when used). **Mandatory parameter**.
* _**topicsAsFiles**_ [Array of Strings] - List of topics to subscribe on MQTT broker to be saved as files on MongoDB (Gridfs). **Mandatory parameter**.
* _**topicsScripted**_ [Array of Objects] - List of topics to subscribe on MQTT broker to be treated with dedicated scripts to extract data. **Mandatory parameter**.
* _**clientId**_ [String] - MQTT Client Id for the connection. If configured, it should be unique over all MQTT clients. Leave empty to to be automatically assigned by the broker. **Optional parameter**.
* _**groupId**_ [String] - Group Id for publishing. Leave empty to avoid Sparkplug B publishing. **Mandatory parameter**.
* _**edgeNodeId**_ [String] - Edge Node Id for publishing. **Mandatory parameter**.
* _**deviceId**_ [String] - Device Id for publishing. **Optional parameter**.
* _**scadaHostId**_ [String] - Scada host Id for Primary Application STATE publishing. Leave empty if not a primary application. **Mandatory parameter**.
* _**publishTopicRoot**_ [String] - Non-Sparkplug MQTT topic root for publishing tags. Leave empty to not publish tags as normal topics. **Mandatory parameter**.
* _**username**_ [String] - The username for the MQTT broker connection. **Optional parameter**.
* _**password**_ [String] - The password for the MQTT broker connection. **Optional parameter**.
* _**useSecurity**_ [Boolean] - Use (true) or not (false) secure encrypted connection. **Mandatory parameter**.
* _**rootCertFilePath**_ [String] - Trusted CA certificates PEM file path (equiv to NodeJS TLS option 'ca'). **Optional parameter**.
* _**pfxFilePath**_ [String] - PFX or PKCS12 File path to encoded private key and certificate chain. pfx is an alternative to providing key and cert individually. (equiv. to NodeJS TLS option 'pfx'). **Optional parameter**.
* _**passphrase**_ [String] - Shared passphrase used for a single private key and/or a PFX (equiv. to NodeJS TLS option 'passphrase'). **Optional parameter**.
* _**chainValidation**_ [Boolean] - Indicates whether the drivers performs a X509 chain validation against the registered CA certificates (equiv. to NodeJS TLS option 'rejectUnauthorized'). Default: false. **Optional parameter**.
* _**localCertFilePath**_ [String] - File that contains the certificate (*.PEM) that will be presented to the remote side of the connection (equiv. to NodeJS TLS option 'cert'). **Optional parameter**.
* _**privateKeyFilePath**_ [String] - File (*.PEM) that contains the private key corresponding to the local certificate (equiv. to NodeJS TLS option 'key'). **Optional parameter**.
* _**allowTLSv10**_ [Boolean] - Allow TLS version 1.0 (default true, recommended false). **Optional parameter**.
* _**allowTLSv11**_ [Boolean] - Allow TLS version 1.1 (default true, recommended false). **Optional parameter**.
* _**allowTLSv12**_ [Boolean] - Allow TLS version 1.2 (default true). **Optional parameter**.
* _**allowTLSv13**_ [Boolean] - Allow TLS version 1.3 (default true). **Optional parameter**.
* _**cipherList**_ [String] - TLS cipher list (equiv. to NodeJS TLS option 'ciphers'). Leave empty to use defaults. **Optional parameter**.

See also NodeJS TLS configuration and Sparkplug-Client original lib.

* https://nodejs.org/api/tls.html
* https://github.com/Cirrus-Link/Sparkplug/tree/master/client_libraries/javascript/sparkplug-client

JSON Path Plus
* https://github.com/JSONPath-Plus/JSONPath

### Configuration Hints

* To act as a Sparkplug Primary (or non-primary) SCADA host, configure the _scadaHostId_ property.
* To subscribe to Sparkplug B devices, configure the _topics_ property with a list of topics with root "spBv1.0/".
* To subscribe to regular MQTT topics, configure the _topics_ property with a list of the desired topics.
* The topics property can have mixed Sparkplug and regular topics listed.
* To subscribe to regular MQTT topics, and extract complex data structures with scripts, configure the _topicsScripted_ property.
* To subscribe to regular binary MQTT topics to be saved as files on MongoDB-Gridfs, configure the _topicsAsFiles_ property.
* To publish tags as regular MQTT topics, configure the _publishTopicRoot_ property.
* To publish tags as a Sparkplug B device, configure the _groupId_, _edgeNodeId_ and _deviceId_ properties.

## Configuring Tags for Update by Sparkplug B Metrics

The tag to be updated in the _realtimeData_ collection must have the _protocolSourceConnectionNumber_ set to the number of the respective connection and _protocolSourceObjectAddress_ must be configured with the Sparkplug addressing parameters SparkplugNamespace/GroupId/EdgeNodeId/DeviceId/MetricName published by the originator device/node. If _autoCreateTags_ is set to true for the connection, the tags will be auto created when not found by _protocolSourceConnectionNumber/protocolSourceObjectAddress_. Tags can also be configured via the Admin Management UI.

    db.realtimeData.updateOne({"tag":"SPB_TAG_NAME"}, {
        $set: {
            "protocolSourceConnectionNumber": 1200, // connection number used by the MQTT client driver
            "protocolSourceObjectAddress": "spBv1.0/group_id/edge_node_id/device_id/metric_name", // Sparkplug addressing
            "kconv1": 1.0,
            "kconv2": 0.0,
        });

## Configuring Tags for Update by regular MQTT Topics

The tag to be updated in the _realtimeData_ collection must have the _protocolSourceConnectionNumber_ set to the number of the respective connection and _protocolSourceObjectAddress_ must be configured with the topic name published by the originator device. If _autoCreateTags_ is set to true for the connection, the tags will be auto created when not found by _protocolSourceConnectionNumber/protocolSourceObjectAddress_. Use JSONPath syntax to extract property values of a JSON payload (e.g. "root_topic/subtopic/$.property1").

Tags can also be configured via the Admin Management UI.

    db.realtimeData.updateOne({"tag":"MQTT_TAG_NAME"}, {
        $set: {
            "protocolSourceConnectionNumber": 1200, // connection number used by the MQTT client driver
            "protocolSourceObjectAddress": "enterprise_name/area_name/device_name/metric_name", // topic name
            "kconv1": 1.0,
            "kconv2": 0.0,
        });

    db.realtimeData.updateOne({"tag":"MQTT_TAG_NAME_JSONPATH"}, {
        $set: {
            "protocolSourceConnectionNumber": 1200, // connection number used by the MQTT client driver
            "protocolSourceObjectAddress": "/root_topic/subtopic/$.property1", // topic name as JSONPath syntax
            "kconv1": 1.0,
            "kconv2": 0.0,
        });

The data type is automatically detected and converted by the driver. If the data published is not to be interpreted as a number, boolean, JSON or string, it should be subscribed as a _Scripted Topic_ so data will be extracted by the dedicated script.

## Send Commands As MQTT regular topic published

To send commands as a regular MQTT topic, configure the command tag as below. Tags can also be configured via the Admin Management UI.

    db.realtimeData.updateOne({"tag":"MQTT_COMMAND_TAG_NAME"}, {
        $set: {
            "protocolSourceConnectionNumber": 1200, // connection number used by the MQTT client driver
            // MQTT payload will be a string containing a value of type: 
            //   int,int8,int16,int32,int64,uint8,uint16,uint32,uint64,float,double,boolean,string,datetime,text
            "protocolSourceASDU": "boolean", // type
            "protocolSourceObjectAddress": "enterprise_name/area_name/device_name/metric_name", // topic name
            "protocolSourceCommandDuration": 0, // qos level
            "protocolSourceCommandUseSBO": false, // retain flag
            "kconv1": 1.0, // conversion factor: multiplier
            "kconv2": 0.0, // conversion factor: adder
        });

## Send Commands Via Sparkplug B

To send commands via Sparkplug B, configure the command tag as below. Tags can also be configured via the Admin Management UI.

    db.realtimeData.updateOne({"tag":"MQTT_SPB_COMMAND_TAG_NAME"}, {
        $set: {
            "protocolSourceConnectionNumber": 1200, // connection number used by the MQTT client driver
            "protocolSourceASDU": "boolean", // metric type: int,int8,int16,int32,int64,uint8,uint16,uint32,uint64,float,double,boolean,string,datetime,text
            "protocolSourceObjectAddress": "spBv1.0/group_id/edge_node_id/device_id/metric_name", // topic name/metric name, for DCMD (do not include DCMD in the parameter!)
            // "protocolSourceObjectAddress": "spBv1.0/group_id/edge_node_id/metric_name", // topic name/metric name, for NCMD (do not include NCMD)
            "protocolSourceCommandDuration": 0, // no effect for sparkplug B
            "protocolSourceCommandUseSBO": false, // no effect for sparkplug B
            "kconv1": 1.0, // conversion factor: multiplier
            "kconv2": 0.0, // conversion factor: adder
        });

## Receive (Device) Commands Via Sparkplug B

If commands are enabled for the connection and Sparkplug B is configured, all commands from the tags database are enabled automatically (even without _protocolDestinations_ defined!). In this case, commands are routed to other protocols. The tag is used as the Sparkplug B metric name.

Commands will be listed in the metrics available on the DBIRTH message with the property _isCommand=true_.

    {"name":"KAW2AL-21XCBR5238----K","alias":64001,"value":false,"type":"boolean","properties":{"isCommand":{"type":"boolean","value":true},"description":{"type":"string","value":"KAW2~FD21 13,8kV~BRK52-38:status-Cmd"},"good":{"type":"boolean","value":false}}}

Example topic for Sparkplug B command:

    spBv1.0/group_id/edge_node_id/DCMD/device_id/KAW2AL-21XCBR5238----K

## Example of JSON-SCADA Protocol Driver Instances and Connections Numbering

![Driver instances and connections](https://github.com/riclolsen/json-scada/raw/master/docs/JSON-SCADA_Connections.png "Driver Instances and Connections Numbering")

## Example of Script to Extract Data from complex Payloads

Given a topic "test/jsonarr" that publishes array of values as JSON like 

    [ 12345.2, 23456.7, 345678.9 ]

The script should extract values and return an array of objects like

    [
    {
        id: 'scrVal1',
        value: 12345.2,
        qualityOk: true,
        timestamp: 1619465592683
    },
    {
        id: 'scrVal2',
        value: 23456.7,
        qualityOk: true,
        timestamp: 1619465592683
    },
    {
        id: 'scrVal3',
        value: 345678.9,
        qualityOk: true,
        timestamp: 1619465592683
    }
    ]

It is necessary to subscribe the topic using the _topicsScripted_ array property, providing _topic_ and _script_ in each object of the array.

    "topicsScripted": [{ 
        "topic": "Enterprise/test/jsonarr", 
        "script": " // remove comments and put all in the same line
                shared.dataArray = []; // array of objects to return
                vals = JSON.parse(shared.payload.toString()); 
                cnt = 1;
                vals.forEach(elem => {
                    shared.dataArray.push({'id': 'scrVal'+cnt, 'value': elem, 'qualityOk': true, 'timestamp': (new Date()).getTime() });
                    cnt++;
                })
                // return values in array of objects shared.dataArray
                "
        }]

All scripts are executed in a shared sandboxed Javascript VM. The MQTT payload of each message is passed as a buffer in the "shared.payload" object. The sandboxed VM context is preserved and reused at each call, so variables created inside the scripts are also preserved.

The "shared.dataArray" should receive the array of objects with at least "value" and "id" properties set.

Can also be set the following optional properties: "type", "qualityOk", "timestamp", "transient", "valueString", "valueJson", "causeOfTransmissionAtSource".

