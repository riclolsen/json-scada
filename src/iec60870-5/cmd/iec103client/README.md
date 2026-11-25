# IEC 60870-5-103 Client Protocol Driver

This driver implements a **master (primary station)** for the IEC 60870-5-103
protocol — the companion standard for the informative interface of protection
equipment (protective relays). It can maintain multiple connections, each
polling one or more protection devices over a serial line (FT1.2) or over a
TCP-encapsulated serial stream (terminal / serial-device server).

It is a Go driver built on the [go-iecp5](https://github.com/riclolsen/go-iecp5)
`cs103` package and shares the JSON-SCADA integration engine with the Go
IEC 101/104 drivers (see the [module README](../../README.md)).

There is **no server (device) side** — IEC 60870-5-103 defines the relay as the
secondary station, so this driver is a master only.

To configure the driver it is necessary to create one or more driver instances
and at least one connection per instance. Also the tags intended to be updated
should be configured appropriately.

## How IEC 60870-5-103 differs

Unlike IEC 101/104, a 103 information object is **not** addressed by an
Information Object Address (IOA). Instead it is addressed by:

- **Function type (FUN)** — identifies the protection function group
  (e.g. 128 distance protection, 160 overcurrent protection, 176 transformer
  differential, 192 line differential, 254 generic, 255 global).
- **Information number (INF)** — identifies the specific signal within the
  function (trips, starts, zones, supervision, measurand groups, ...).

All identifier fields are one octet. The link address and the ASDU common
address are conventionally equal (one address per device). Time-tagged events
carry a 4-octet `CP32Time2a` time; time synchronization uses `CP56Time2a`.

## Configure a driver instance

To create a new IEC 103 client instance, insert a new document in the
_protocolDriverInstances_ collection using a command like this:

    use json_scada_db_name
    db.protocolDriverInstances.insertOne({
            protocolDriver: "IEC60870-5-103",
            protocolDriverInstanceNumber: 1,
            enabled: true,
            logLevel: 1,
            nodeNames: ["mainNode"],
            activeNodeName: "mainNode",
            activeNodeKeepAliveTimeTag: new Date(),
            keepProtocolRunningWhileInactive: false
        });

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "IEC60870-5-103". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver, instance numbers should be unique. The instance number makes it possible to run multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
- _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
- _**nodeNames**_ [Array of Strings] - Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate nodes will have the same connections and data enabled for scanning and update. **Mandatory parameter**.
- _**activeNodeName**_ [String] - Name of the protocol driver that is currently active. This is updated by the drivers for redundancy control. **Optional**.
- _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly by the active driver. **Optional**.
- _**keepProtocolRunningWhileInactive**_ [Boolean] - Currently only the _false_ value is supported. **Optional**.

Changes in the _protocolDriverInstances_ config require that the driver instance
processes be restarted to be effective.

## Configure client connections to IEC-103 devices

Each instance for this driver can have many client connections defined in the
_protocolConnections_ collection.

### Serial connection

    use json_scada_db_name
    db.protocolConnections.insertOne({
        protocolDriver: "IEC60870-5-103",
        protocolDriverInstanceNumber: 1,
        protocolConnectionNumber: 71,
        name: "RELAY1",
        description: "Overcurrent relay bay 1",
        enabled: true,
        commandsEnabled: true,
        autoCreateTags: true,
        portName: "COM3",
        baudRate: 9600,
        parity: "Even",
        stopBits: "One",
        remoteLinkAddress: 3,
        giInterval: 300,
        timeSyncInterval: 3600,
        timeoutForACK: 1000,
        timeoutRepeat: 1000,
        maxQueueSize: 100,
        stats: null
    });

### TCP-encapsulated connection

When the relay is reached through a terminal server / serial-device server, set
`portName` to `host:port` — the FT1.2 frames are then carried over a TCP client
connection instead of a local serial port:

    use json_scada_db_name
    db.protocolConnections.insertOne({
        protocolDriver: "IEC60870-5-103",
        protocolDriverInstanceNumber: 1,
        protocolConnectionNumber: 72,
        name: "RELAY2",
        description: "Relay reached via terminal server",
        enabled: true,
        commandsEnabled: true,
        portName: "10.0.0.9:2400",
        remoteLinkAddress: 4,
        giInterval: 300,
        timeSyncInterval: 3600,
        t0: 30,
        stats: null
    });

### Connection parameters

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "IEC60870-5-103". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. **Mandatory parameter**.
- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is used to define the connection that can update a tag. **Mandatory parameter**.
- _**name**_ [String] - Name for the connection. Used for logging. **Mandatory parameter**.
- _**description**_ [String] - Description of the connection. Documental only. **Optional parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
- _**commandsEnabled**_ [Boolean] - Allows commands (general commands in control direction) for the connection. Use false to disable commands. **Mandatory parameter**.
- _**peerIdentification**_ [Boolean] - Records the peer's identity in _stats.peerIdentity_ when the connection is (re)established, and reports its changes as security events. Default: true. **Optional parameter**.
- _**portName**_ [String] - Serial port name (`COM3`, `/dev/ttyS0`, ...) for a local serial line, **or** `host:port` to carry the FT1.2 frames over TCP (terminal / serial-device server). **Mandatory parameter**.
- _**baudRate**_ [Double] - Serial baud rate (e.g. 9600, 19200). Serial transport only. **Optional parameter (default 9600)**.
- _**parity**_ [String] - Serial parity: `Even` (103 standard), `None`, `Odd`. Serial transport only. **Optional parameter (default Even)**.
- _**stopBits**_ [String] - Serial stop bits: `One`, `Two`. Serial transport only. **Optional parameter (default One)**.
- _**remoteLinkAddress**_ [Double] - Device link address (also used as the ASDU common address). Range 0–254 (255 = broadcast). **Mandatory parameter**.
- _**giInterval**_ [Double] - General interrogation period in seconds. Use zero to disable periodic GI. **Optional parameter (default 300)**.
- _**timeSyncInterval**_ [Double] - Time synchronization period in seconds. Use zero to disable. **Optional parameter**.
- _**timeoutForACK**_ [Double] - Link-layer response timeout T1 in milliseconds (converted to whole seconds). **Optional parameter**.
- _**timeoutRepeat**_ [Double] - Link-layer repeat timeout T2 in milliseconds (converted to whole seconds, must be < T1). **Optional parameter**.
- _**maxQueueSize**_ [Double] - Maximum size of the outbound command queue. **Optional parameter (default 100)**.
- _**t0**_ [Double] - TCP connect timeout in seconds (TCP transport only). **Optional parameter (default 30)**.
- _**stats**_ [Object] - Protocol statistics updated by the driver (nodeName, timeTag, linkLayerState, isConnected, peerIdentity). **Optional parameter**.

Parameters needed only for TLS-encrypted TCP connections:

- _**localCertFilePath**_ [String] - Path to the local certificate file presented on the TCP transport. A `*.pfx`/`*.p12` (with _passphrase_) or a PEM file containing the certificate and key. **Optional parameter**.
- _**passphrase**_ [String] - Password for the local `*.pfx` certificate file. **Optional parameter**.
- _**peerCertFilesPaths**_ [Array of Strings] - Certificate files used to verify the peer when _allowOnlySpecificCertificates=true_. **Optional parameter**.
- _**rootCertFilePath**_ [String] - CA certificate file to validate the peer chain when _chainValidation=true_. Without _rootCertFilePath_ the system root certificates are used. **Optional parameter**.
- _**allowOnlySpecificCertificates**_ [Boolean] - Accept only the certificates listed in _peerCertFilesPaths_. Default: false. **Optional parameter**.
- _**chainValidation**_ [Boolean] - Perform X.509 chain validation against the CA certificate. Default: false. **Optional parameter**.

## Addressing convention (FUN / INF)

Because 103 addresses signals by function type and information number, this
driver encodes them into the standard JSON-SCADA source fields as follows:

- _**protocolSourceCommonAddress**_ = the device common address (= link address).
- _**protocolSourceObjectAddress**_ = `FUN * 65536 + INF * 256 + index`, where
  `index` selects a value inside a multi-valued measurands ASDU (use `0` for
  single-valued time-tagged points and for commands).
- _**protocolSourceASDU**_ = the 103 type identification number (for supervised
  points this is documental; for commands use `20`, the general command).

For example, an overcurrent general trip (FUN 160, INF 69) maps to object
address `160*65536 + 69*256 + 0 = 10503424`.

### Data mapping

- **Time-tagged messages** (ASDU 1 and 2) become **digital** points: DPI `On` →
  1, `Off` → 0; `Transient`/`Unknown` set the point invalid. The event time tag
  (CP32Time2a) is stored as the source time tag.
- **Measurands** (ASDU 3 and 9) become **analog** points, one per `index`,
  scaled to the fraction of full scale in the range `[-1, 1)` (the rated value
  corresponds to 1/1.2 or 1/2.4 of full scale, depending on device
  parameterization). The overflow / error flags map to the point quality.
- **Identification** (ASDU 5) and **GI termination** (ASDU 8) are logged.

## Configure tags for update

Each supervised tag to be updated on a connection must have a protocol source
set configured. Only one source connection can update a tag.

    use json_scada_db_name
    db.realtimeData.updateOne({ tag: "RELAY1_TRIP" }, {
        $set: {
            protocolSourceConnectionNumber: 71,
            protocolSourceCommonAddress: 3,
            protocolSourceObjectAddress: 10503424, // FUN 160, INF 69, index 0
            protocolSourceASDU: 1,
            protocolSourceCommandDuration: 0,
            protocolSourceCommandUseSBO: false,
            kconv1: 1,
            kconv2: 0
        }
    });

- _**protocolSourceConnectionNumber**_ [Double] - Number code for the protocol connection allowed to update the tag. **Mandatory parameter**.
- _**protocolSourceCommonAddress**_ [Double] - Device common address. **Mandatory parameter**.
- _**protocolSourceObjectAddress**_ [Double] - `FUN*65536 + INF*256 + index`. Combined with _protocolSourceCommonAddress_ it must be unique for the connection. **Mandatory parameter**.
- _**protocolSourceASDU**_ [Double] - Source 103 type. Documental for supervised points; use `20` for command points. **Mandatory parameter**.
- _**protocolSourceCommandDuration**_ [Double] - For command points, carried as the Return Information Identifier (RII) of the general command. **Mandatory parameter**.
- _**protocolSourceCommandUseSBO**_ [Boolean] - Not used by 103 (no select-before-operate). Keep false. **Mandatory parameter**.
- _**kconv1 / kconv2**_ [Double] - Conversion factors are not applied by this driver (103 measurands are delivered as a fraction of full scale). Keep 1 and 0. **Mandatory parameter**.

## Commands (control direction)

Control is performed with the 103 **general command** (ASDU 20). Configure the
command tag with `protocolSourceASDU: 20` and set _protocolSourceObjectAddress_
to `FUN*65536 + INF*256`. A command inserted into the _commandsQueue_ sends a
double command derived from the value (non-zero → `On`, zero → `Off`); the RII
is taken from _protocolSourceCommandDuration_.

The device acknowledges with an ASDU 1 whose cause of transmission is
20 (positive) or 21 (negative); the driver correlates it by object address and
writes the result back to the command's `ack` field.

Command example (double command On to FUN 160, INF 145):

    db.realtimeData.updateOne({ tag: "RELAY1_RESET_LED" }, {
        $set: {
            origin: "command",
            protocolSourceConnectionNumber: 71,
            protocolSourceCommonAddress: 3,
            protocolSourceObjectAddress: 10522880, // FUN 160, INF 145
            protocolSourceASDU: 20,
            protocolSourceCommandDuration: 0
        }
    });

## Automatic startup behavior

On each device whose link becomes active, and then periodically per
_giInterval_ and _timeSyncInterval_, the driver issues a time synchronization
(ASDU 6) and a general interrogation (ASDU 7). Devices report their
identification message (ASDU 5) after reset/startup, which is logged.

## Automatic tag creation (autoCreateTags)

The optional connection parameter _**autoCreateTags**_ [Boolean, default false]
lets the driver bootstrap the point list directly from a live relay. When it is
set to `true`, the first time a value is received for a device / point that has
no tag yet in the _realtimeData_ collection, a new supervised tag is inserted
automatically.

- Each 103 signal is identified by its function type (FUN) and information
  number (INF); the auto-created tag uses the composite object address
  `FUN*65536 + INF*256 + index` described in the "Addressing convention"
  section, so every event signal and every measurand element gets its own tag.
- The tag is named `<connectionName>;<commonAddress>;<objectAddress>` and
  created as _digital_ (time-tagged messages) or _analog_ (measurands)
  according to the received ASDU, with `origin: "supervised"` and
  `invalid: true` until the next update.
- New tag keys (`_id`) are allocated inside a partition reserved for the
  connection — the range
  `[protocolConnectionNumber × 1000000, (protocolConnectionNumber + 1) × 1000000)`.
  Choose _protocolConnectionNumber_ values so these partitions do not overlap
  each other or your manually numbered tags.
- Addresses already present are preloaded at startup, so restarting the driver
  never re-creates existing tags.
- Only monitored (supervised) points are created; command tags are never
  auto-created and must be configured manually (see the "Commands" section).

Once the tags exist you can rename, group and edit them normally. Set
_autoCreateTags_ back to `false` when the address map is stable.

## Security events

The driver reports unexpected or suspicious conditions to the _secEvents_ collection, listed by the Security Events viewer of the Admin UI: go-iecp5 checks the FT1.2 link and the 103 ASDUs; the driver adds the restarts reported by the devices (identification with cause start/restart or power on), general commands refused by a device (negative acknowledgement) or not sent, stale queued commands, command floods and unnecessary commands. With _peerIdentification_ on, the driver records the device's address and link address and, from each device's identification ASDU 5, its compatibility level, manufacturer/product text and software identification (a change is reported as `peer.identity_changed`) in _stats.peerIdentity_ and reports a change between connections, also across driver restarts. See [Security events and peer identification](../../README.md#security-events-and-peer-identification) for the event kinds and the _stats.peerIdentity_ document.

## Command Line Arguments

- _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**.
- _**2nd arg. - Log Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**.
- _**3rd arg. - Config File Path/Name**_ [String] - Complete path/name of the JSON-SCADA config file. **Optional argument, default="../conf/json-scada.json"**.

## Limitations

- Generic services (ASDU 10/11/21) and disturbance data transfer (ASDU 23–31)
  are not decoded.
