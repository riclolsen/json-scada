# ONVIF Camera Client Protocol Driver

This driver implements a client for the ONVIF Camera protocol. It can have multiple connections to ONVIF camera servers on the network for monitoring and control. The image stream can be accessed via a WebSocket on the browser.

The ONVIF/RTSP camera streaming is not supported by browsers, so this driver uses the ONVIF PTZ Streaming API to get the image stream and convert it to a mpeg stream and send it to the browser where it is accessible via WebSocket.

To configure the driver it is necessary to create one or more driver instances and at least one connection per instance.

## Configure a driver instance

To create a new ONVIF driver instance, insert a new document in the _protocolDriverInstances_ collection using a command like below or use the Admin UI.

    use json_scada_db_name
    db.protocolDriverInstances.insertOne({
            protocolDriver: "ONVIF",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: ["mainNode"],
            activeNodeName: "mainNode",
            activeNodeKeepAliveTimeTag: new Date(),
            keepProtocolRunningWhileInactive: false
        });

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "ONVIF". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
- _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
- _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate nodes will have the same connections and data enabled for scanning and update. **Mandatory parameter**.
- _**activeNodeName**_ [String] - Name of the node that is currently active. This is updated by the drivers for redundancy control.**Optional**.
- _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly by the active driver. **Optional**.
- _**keepProtocolRunningWhileInactive**_ [Boolean] - Define a driver will keep the protocol running while not the main active driver. Currently only the _false_ value is supported. **Optional**.

Changes in the _protocolDriverInstances_ config requires that the driver instances processes be restarted to be effective.

## Configure client connections to ONVIF cameras servers

Each instance for this driver can have many client connection defined that must be described in the _protocolConnections_ collection.
Create a new connection in Admin UI or directly on MongoDB as below.

    use json_scada_db_name
    db.protocolConnections.insertOne({
        protocolDriver: "ONVIF",
        protocolDriverInstanceNumber: 1,
        protocolConnectionNumber: 9001,
        name: "CAM001",
        description: "CAM001 - Camera",
        enabled: true,
        commandsEnabled: true,
        endpointURLs: ["http://192.168.1.100/onvif/device_service"],
        username: "admin",
        password: "admin",
        timeoutMs: 5000,
        giInterval: 10,
        ipAddressLocalBind: "127.0.0.1:9001",
        options: "{'-r': 30, '-s': '320x240'}", // JSON string with ffmpeg options
    });

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "ONVIF". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. **Mandatory parameter**.
- _**name**_ [String] - Name for a camera connection. **Mandatory parameter**.
- _**description**_ [String] - Description for the purpose of a camera connection. Just documental. **Optional parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the camera connection. **Mandatory parameter**.
- _**commandsEnabled**_ [Boolean] - Allows to disable commands (messages in control direction) for a camera connection. Use false here to disable commands. **Mandatory parameter**.
- _**endpointURLs**_ [Array] - Array of endpoint URLs for the camera server. Only the first URL is used. **Mandatory parameter**.
- _**username**_ [String] - Username for the camera server. **Mandatory parameter**.
- _**password**_ [String] - Password for the camera server. **Mandatory parameter**.
- _**timeout**_ [Double] - Timeout for the camera connection in milliseconds. **Mandatory parameter**.
- _**ipAddressLocalBind**_ [String] - IP address and port to bind the websocket server. Usually the first camera connection should bind to 127.0.0.1:9001, the second to 127.0.0.1:9002 and so on. **Mandatory parameter**.
- _**options**_ [String] - JSON string with options for the ffmpeg encoding. Default is "{'-r': 30, '-s': '320x240'}". See the ffmpeg documentation for more options. **Optional parameter**.
- _**giInterval**_ [Double] - Interval for camera snapshots in seconds. Use 0 to disable snapshots. Default is 0. **Optional parameter**.

## ONVIF Camera Commands

To send commands to the camera, you can use the following tags.

    $$Name$$Command$$Variable

Commands: relativeMove, absoluteMove, continuousMove, stop, setHomePosition, gotoHomePosition, setPreset, removePreset, gotoPreset

Examples:

* To move the camera relative to its current position in x direction - Tag: $$CAM001$$relativeMove$$x, Command Value: 0.1
* To move the camera relative to its current position in y direction - Tag: $$CAM001$$relativeMove$$y, Command Value: -0.1
* To zoom the camera in - Tag: $$CAM001$$relativeMove$$zoom, Command Value: 0.1
* To move the camera to the preset 1 - Tag: $$CAM001$$gotoPreset, Command Value: 1
* To set the current position as preset 1 - Tag: $$CAM001$$setPreset, Command Value: 1
* To move the camera to the home position - Tag: $$CAM001$$gotoHomePosition, Command Value: 0
* To set the current position as home position - Tag: $$CAM001$$setHomePosition, Command Value: 0
* To stop the camera movement - Tag: $$CAM001$$stop, Command Value: 0

### Example of command in MongoDB

    use json_scada_db_name
    db.commandsQueue.insertOne({
        protocolSourceConnectionNumber: -1.0,
        protocolSourceCommonAddress: -1.0,
        protocolSourceObjectAddress: -1.0,
        protocolSourceASDU: -1.0,
        protocolSourceCommandDuration: 0.0,
        protocolSourceCommandUseSBO: false,
        pointKey: 0.0,
        tag: '$$CAM001$$relativeMove$$x',
        timeTag: new Date(),
        value: 1.0,
        valueString: "1.0",
        originatorUserName: 'username',
        originatorIpAddress: '127.0.0.1'
    })

## Camera UI

To embed the camera UI inside an SVG display see the SVG editor [documentation](https://github.com/riclolsen/json-scada/tree/master/src/svg-display-editor#set-tab).

The camera web interface is coded in the file [camera.html](https://github.com/riclolsen/json-scada/blob/master/src/AdminUI/dist/camera.html).
