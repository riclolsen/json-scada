# IEC61850 Client (Go)

This driver implements a client for the IEC 61850 (MMS) protocol in Go, on top of
[go-iec61850](https://github.com/dscsystems/go-iec61850) — a pure-Go implementation of the
protocol stack.

    Binary:  iec61850-client (iec61850-client.exe on Windows)
    Service: JSON_SCADA_iec61850client (Windows), [program:iec61850client] (Linux)
    Log:     log/iec61850client.log

The driver can have multiple connections to IEC61850 servers (IEDs).

To configure the driver it is necessary to create one or more driver instances and at least one connection per instance, this can be accomplished using the Web interface (Admin UI) or programaticallly as below. Also the tags intended to be updated should be configured appropriately. The AutoCreateTags feature can be used to create all tags for reports automatically.

## Configure a driver instance

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

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "IEC61850". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the instance. Use false here to disable the instance. **Mandatory parameter**.
- _**logLevel**_ [Double] - Number code for log level (0=minimum,1=basic,2=detailed,3=debug). Too much logging (levels 2 and 3) can affect performance. **Mandatory parameter**.
- _**nodeNames**_ [Array of Strings]- Array of node names that can run the instance. Use more than one node for redundancy. Each redundant instance running on separate nodes will have the same connections and data enabled for scanning and update. **Mandatory parameter**.
- _**activeNodeName**_ [String] - Name of the protocol driver that is currently active. This is updated by the drivers for redundancy control.**Optional**.
- _**activeNodeKeepAliveTimeTag**_ [Date] - This is updated regularly by the active driver. **Optional**.
- _**keepProtocolRunningWhileInactive**_ [Boolean] - Define a driver will keep the protocol running while not the main active driver. Currently only the _false_ value is supported. **Optional**.

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

- _**protocolDriver**_ [String] - Name of the protocol driver, must be "IEC61850". **Mandatory parameter**.
- _**protocolDriverInstanceNumber**_ [Double] - Number of the instance. Use 1 to N to number instances. For the same driver instance numbers should be unique. The instance number makes possible to run use multiple processes of the driver, each one with a distinct configuration. **Mandatory parameter**.
- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. This must be unique for all connections over all drivers on a system. This number is be used to define the connection that can update a tag. **Mandatory parameter**.
- _**name**_ [String] - Name for a connection. Will be used for logging. **Mandatory parameter**.
- _**description**_ [String] - Description for the purpose of a connection. Just documental. **Optional parameter**.
- _**enabled**_ [Boolean] - Controls the enabling of the connection. Use false here to disable the connection. **Mandatory parameter**.
- _**commandsEnabled**_ [Boolean] - Allows to disable commands (messages in control direction) for a connection. Use false here to disable commands. **Mandatory parameter**.
- _**ipAddresses**_ [Array of Strings] - Array of server IP addresses (or hostnames) and TCP ports (only the first server is currently supported). **Mandatory parameter**.
- _**topics**_ [Array of Strings] - Array of report names to be activated (will activate all if none was specified). \*Mandatory parameter\*\*.
- _**autoCreateTags**_ [Boolean] - When true the driver will auto create tags for every data point found in activated reports in the server. When false, only preconfigured tags will be updated. **Mandatory parameter**.
- _**giInterval**_ [Double] - Scan interval in seconds for data not in reports. **Mandatory parameter**.
- _**class0ScanInterval**_ [Double] - Integrity interval in seconds for data in reports. **Mandatory parameter**.
- _**useSecurity**_ [Boolean] - Use (true) or not (false) secure encrypted connection. **Mandatory parameter**.

TLS notes: certificates may be PEM or DER. `chainValidation: false` skips chain verification;
`allowOnlySpecificCertificates: true` pins the peer to `peerCertFilesPaths`. The version window is the lowest to the highest enabled flag. Go does not negotiate TLS 1.0/1.1, so a request for them is clamped to TLS 1.2 and logged. The default port for MMS over TLS is 3782 — put it in `ipAddresses` explicitly, since the default here stays 102 for compatibility.

Commands use `protocolSourceCommonAddress: "CO"` for a control object; any other functional
constraint performs a plain MMS write. `protocolSourceCommandUseSBO` forces select-with-value on a normal-security SBO object; otherwise the control model decides the sequence.

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

- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. Only this protocol connection can update the tag. **Mandatory parameter**.
- _**protocolSourceCommonAddress**_ [String] - Functional contraint (ST, MX, CF, etc.). **Mandatory parameter**.
- _**protocolSourceObjectAddress**_ [String] - IEC61850 element address. This address must be unique in a connection (for supervised points). **Mandatory parameter**.
- _**protocolSourceASDU**_ [String] - Unused. **Optional parameter**.
- _**kconv1**_ [Double] - Analog conversion factor: multiplier. Use -1 to invert digital values. **Mandatory parameter**.
- _**kconv2**_ [Double] - Analog conversion factor: adder. **Mandatory parameter**.

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

- _**protocolConnectionNumber**_ [Double] - Number code for the protocol connection. Only this protocol connection can command this tag. **Mandatory parameter**.
- _**protocolSourceCommonAddress**_ [String] - Functional contraint ("CO" for control block, other FCs will generate a simple MMS write). **Mandatory parameter**.
- _**protocolSourceObjectAddress**_ [String] - IEC61850 element address. This address must be unique in a connection (for commands). **Mandatory parameter**.
- _**protocolSourceASDU**_ [String] - unused. **Optional parameter**.
- _**kconv1**_ [Double] - Analog conversion factor: multiplier. Use -1 to invert digital values. **Mandatory parameter**.
- _**kconv2**_ [Double] - Analog conversion factor: adder. **Mandatory parameter**.

## Command line arguments

    iec61850-client [instance [logLevel [configFile]]]

- **1st — Instance number** [Integer] — driver instance to run. *Optional, default 1*.
- **2nd — Log level** [Integer] — 0=minimum, 1=basic, 2=detailed, 3=debug. *Optional, default 1*.
- **3rd — Config file** [String] — path of `json-scada.json`. *Optional, default `../conf/json-scada.json`,
  then `c:/json-scada/conf/json-scada.json`*. The `JS_CONFIG_FILE` environment variable is also honoured.

Log level 3 additionally dumps the MMS protocol exchange (the equivalent of libiec61850's debug
output). Logging goes to stdout only; the service manager redirects it to the log file.

## Value extraction

The value of a data object is the attribute its common data class names — `stVal` for a status, `mag` for a measurand — not whichever member happens to be numeric. The driver learns those names, in the order the value carries them, from the type description of each logical node while browsing (and of each data set member), so a controllable object that also carries an operation counter under `ST` still reads its position.

**Double points** (`Pos` and any other DPS/DPC) are read from their two-bit `stVal` and become
**digital** points: `01` off → 0, `10` on → 1. The two remaining states are not positions, and both reach the tag's quality: `00` (intermediate — the switch is moving) is invalid *and* transient, `11` (faulty) is invalid.

When the attribute names are not known, the value is found by type, preferring a status attribute (a boolean or a two-bit position) over a numeric one.

## autoCreateTags

With `autoCreateTags: true` the driver creates a tag for every value-bearing object it finds:

* **while browsing** — every data object the server exposes under a status (`ST`) or measurand
  (`MX`) constraint, whether or not any report carries it. These are polled every `giInterval`
  seconds until a report covers them, at which point they follow the report instead;
* **from reports** — any member of an activated data set that is not configured yet;
* **command tags** — every controllable object the device exposes (one carrying an `Oper`
  attribute), when `commandsEnabled` is on. The tag is `origin: "command"` with
  `protocolSourceCommonAddress: "CO"`, `protocolSourceCommandUseSBO` taken from whether the object has a select attribute, and `type` from the control value the device declares (`digital` for a single or double point, `analog` for a setpoint).

Each command tag is **linked to the point where its effect shows**: the command carries
`supervisedOfCommand` with the `_id` of the supervised point of the same data object, and that
point gets `commandOfSupervised` pointing back. The driver waits for the supervised point to exist before creating the command, so the pair is always complete; a controllable object with no status at all yields an unlinked command, which still operates but gives the operator no feedback.

A controllable object is registered as soon as it is discovered, so it can be commanded in the same session — no restart needed to use a freshly created command tag. Control objects are never polled: reading one returns the operate structure, not a measurement.

Settings, configuration and description attributes are not points and are not created.

Automatically created tags are named `<connection>;<object reference>[<FC>]` and grouped by the device hierarchy:

| Field | Value | Example |
|---|---|---|
| `tag` | `<connection>;<object reference>[<FC>]` | `IED1;DemoProtCtrl/Obj1XCBR1.Pos[ST]` |
| `group1` | connection `name` | `IED1` |
| `group2` | logical device | `DemoProtCtrl` |
| `group3` | logical node | `Obj1XCBR1` |
| `ungroupedDescription` | the rest of the reference (data object and attributes) | `Pos` |
| `description` | `group1~group2~group3~ungroupedDescription` | `IED1~DemoProtCtrl~Obj1XCBR1~Pos` |

A command tag ends in `[CO]` and its descriptions in ` command`. Tags are allocated `_id`s from the range `protocolConnectionNumber * 1000000`.

A point that already has a tag, configured by hand or created earlier, is never duplicated: tags are matched by object reference and functional constraint (`protocolSourceObjectAddress` and `protocolSourceCommonAddress`), not by name. Tags created by earlier versions of the driver, named `IEC61850;<connection>;…` with `group1` `IEC61850`, keep their name and groups; delete them to have them recreated in the new form.

 On a large IED this creates a tag per data object. The driver logs the count per logical device (`N browsed object(s) registered for tag creation`). Leave `autoCreateTags` off and configure the points manually instead.

## Example of JSON-SCADA Protocol Driver Instances and Connections Numbering

![Driver instances and connections](https://github.com/riclolsen/json-scada/raw/master/docs/JSON-SCADA_Connections.png 'Driver Instances and Connections Numbering')

