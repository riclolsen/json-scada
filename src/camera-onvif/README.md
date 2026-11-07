# ONVIF Camera Client Protocol Driver

This driver implements a client for ONVIF cameras and plain RTSP video sources. It can have multiple connections to cameras on the network for monitoring (video streaming, snapshots) and control (PTZ commands). The image stream can be accessed via a WebSocket on the browser.

ONVIF/RTSP camera streaming is not supported natively by browsers, so this driver uses the ONVIF Media API to discover the camera's RTSP stream, converts it with _ffmpeg_ to an MPEG1 stream and serves it over WebSocket where it can be played with JSMpeg (see the provided [camera.html](camera.html) viewer).

Driver features:

- Multiple camera connections per driver instance.
- ONVIF endpoints (`http://` or `https://` device service URLs) with PTZ control, snapshots and camera events.
- Plain stream endpoints for streaming-only sources (no PTZ/snapshot support): any URL scheme supported by ffmpeg, e.g. `rtsp://`, `rtmp://`, `udp://`, `srt://`.
- Automatic reconnection to MongoDB and to cameras/streams (ffmpeg is restarted when it exits).
- Redundancy control (active/standby driver nodes) like the other JSON-SCADA drivers.
- Periodic JPEG snapshots (`giInterval` seconds) fetched with basic or digest HTTP authentication, saved to the `snapshots/` folder as `<connection name>.jpg`.
- PTZ commands consumed from the _commandsQueue_ collection with delivery/ack feedback.
- Connection statistics written to the connection document (`stats` field).

The bundled `ffmpeg.exe` is used on Windows. On Linux, install ffmpeg with your distribution's package manager (`ffmpeg` must be on the PATH). The ffmpeg executable path can be overridden with the `JS_ONVIF_FFMPEG_PATH` environment variable.

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
        options: '{"-r": 30, "-s": "320x240"}', // JSON string with ffmpeg options
    });

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "ONVIF". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. **Mandatory parameter**.
- _**name**_ [String] - Name for a camera connection. Also used to address commands (see below). **Mandatory parameter**.
- _**description**_ [String] - Description for the purpose of a camera connection. Just documental. **Optional parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the camera connection. **Mandatory parameter**.
- _**commandsEnabled**_ [Boolean] - Allows to disable commands (messages in control direction) for a camera connection. Use false here to disable commands. **Mandatory parameter**.
- _**endpointURLs**_ [Array] - Array of endpoint URLs for the camera server. Only the first URL is used. Use a `http://` or `https://` ONVIF device service URL for full functionality (PTZ, snapshots, events), or any other ffmpeg-supported stream URL (`rtsp://`, `rtmp://`, `udp://`, `srt://`, ...) for a streaming-only source. **Mandatory parameter**.
- _**username**_ [String] - Username for the camera server. Also injected in the RTSP stream URL and used for snapshot HTTP (basic/digest) authentication. **Mandatory parameter**.
- _**password**_ [String] - Password for the camera server. **Mandatory parameter**.
- _**timeoutMs**_ [Double] - Timeout for the camera connection in milliseconds. Default is 5000. **Optional parameter**.
- _**ipAddressLocalBind**_ [String] - IP address and port for the JSMpeg WebSocket server (only the port part is currently used, the server listens on all interfaces). Usually the first camera connection should use 127.0.0.1:9001, the second 127.0.0.1:9002 and so on, matching the /camNNN reverse proxy config (see below). **Mandatory parameter**.
- _**options**_ [String] - JSON string with options for the ffmpeg encoding. Default is `'{"-r": 30, "-s": "320x240"}'` (30 fps, 320x240 pixels). See the ffmpeg documentation for more options. **Optional parameter**.
- _**giInterval**_ [Double] - Interval for camera snapshots in seconds. Use 0 to disable periodic snapshots. Snapshots are saved to the driver's `snapshots/` folder as `<connection name>.jpg`. Default is 0. **Optional parameter**.

## ONVIF Camera Commands

To send commands to the camera, use command tags with the following naming convention (the tag itself carries the addressing, no protocol addresses are needed).

    $$Name$$Command$$Variable

Commands: relativeMove, absoluteMove, continuousMove, stop, setHomePosition, gotoHomePosition, setPreset, removePreset, gotoPreset, snapshot.

For the move commands, the _Variable_ suffix can be `x`, `y` or `zoom`, and the command value is the amount to move. Alternatively, the command's _valueString_ can carry a JSON object that is passed directly to the ONVIF PTZ call (e.g. `{"x": 0.1, "y": -0.1, "zoom": 0.2}`), in which case the variable suffix is not needed.

Examples:

- To move the camera relative to its current position in x direction - Tag: $$CAM001$$relativeMove$$x, Command Value: 0.1
- To move the camera relative to its current position in y direction - Tag: $$CAM001$$relativeMove$$y, Command Value: -0.1
- To zoom the camera in - Tag: $$CAM001$$relativeMove$$zoom, Command Value: 0.1
- To move the camera to the preset 1 - Tag: $$CAM001$$gotoPreset, Command Value: 1
- To set the current position as preset 1 - Tag: $$CAM001$$setPreset, Command Value: 1
- To remove the preset 1 - Tag: $$CAM001$$removePreset, Command Value: 1
- To move the camera to the home position - Tag: $$CAM001$$gotoHomePosition, Command Value: 0
- To set the current position as home position - Tag: $$CAM001$$setHomePosition, Command Value: 0
- To stop the camera movement - Tag: $$CAM001$$stop, Command Value: 0
- To capture a snapshot on demand - Tag: $$CAM001$$snapshot, Command Value: 0

The driver acknowledges commands in the _commandsQueue_ document (`delivered`, `ack`, `ackTimeTag`, `resultDescription` fields). Commands older than 10 seconds are discarded. PTZ commands require an ONVIF endpoint (not available for plain `rtsp://` sources).

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

The camera web interface is coded in the file [camera.html](https://github.com/riclolsen/json-scada/blob/master/src/AdminUI/public/camera.html). It accepts the following URL query parameters:

- _**CameraName**_ - Connection name used for PTZ commands (default `CAM001`).
- _**wsUrl**_ - Full WebSocket URL of the JSMpeg stream (e.g. `ws://192.168.1.10:9001`).
- _**wsPort**_ - Port of the JSMpeg stream on the page's own host.
- _**camId**_ - 3-digit camera id when the stream is reverse-proxied at `/camNNN` (see below), e.g. `001`.

When no parameter is given, the legacy default `ws://localhost:9001` is used.

The provided nginx configuration templates ([conf-templates/nginx_http.conf](../../conf-templates/nginx_http.conf)) proxy `/camNNN` paths to the corresponding local JSMpeg WebSocket ports (`/cam001` → port 9001, `/cam002` → port 9002, ...), so the camera stream can be reached through the main web server with `camera.html?CameraName=CAM001&camId=001`.

## Run

Requires Node.js and `ffmpeg` (see `JS_ONVIF_FFMPEG_PATH` below).

    cd src/camera-onvif
    npm install
    node index.js [instance number] [log level] [config file]

The optional arguments default to instance `1`, log level `1` and
`../../conf/json-scada.json`.

## Environment Variables

- _**JS_CONFIG_FILE**_ - Path to the _json-scada.json_ config file.
- _**JS_ONVIF_INSTANCE**_ - Instance number (when not passed as first command line argument).
- _**JS_ONVIF_LOGLEVEL**_ - Log level (when not passed as second command line argument).
- _**JS_ONVIF_FFMPEG_PATH**_ - Path of the ffmpeg executable. When not defined, the driver uses `ffmpeg.exe`/`ffmpeg` from its own folder if present, else `ffmpeg` from the PATH.
