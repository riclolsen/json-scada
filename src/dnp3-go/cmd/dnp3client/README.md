# DNP3 Client Driver (Go)

`dnp3-client` is the JSON-SCADA DNP3 **master**. It connects to DNP3 outstations (RTUs, IEDs,
gateways), reads their points by class and integrity polls and by unsolicited reports, writes the
values into the JSON-SCADA real-time database, and carries operator commands back to the field as
control relay output blocks and analog setpoints.

It is a single static binary with no native dependencies. The protocol stack is
[go-dnp3](https://github.com/dscsystems/go-dnp3) (pure Go); the driver adds the JSON-SCADA side:
MongoDB, tag creation, redundancy, statistics and commands.

| | |
| --- | --- |
| Binary | `dnp3-client` (`dnp3-client.exe` on Windows) |
| `protocolDriver` | `DNP3` |
| Role | DNP3 master; one session per `protocolConnections` document |
| Transports | TCP (active and passive), TLS (active and passive), serial, UDP |
| Reads | static (integrity, class and range polls) and event data (polled and unsolicited) |
| Writes | `realtimeData.sourceDataUpdate`; commands acknowledged in `commandsQueue` |
| Redundancy | active/standby between nodes |

The driver only ever *acquires*. It writes the `sourceDataUpdate` sub-document of a tag and leaves
the tag's `value`, alarms, history and everything else to the JSON-SCADA data processor
(`cs_data_processor`), which must be running for values to appear on screens.

## Contents

1. [Building](#building)
2. [Running](#running)
3. [How it works](#how-it-works)
4. [Configuration](#configuration)
5. [Transports](#transports)
6. [Polling](#polling)
7. [Tags](#tags)
8. [Automatic tag creation](#automatic-tag-creation)
9. [Commands](#commands)
10. [Time synchronisation](#time-synchronisation)
11. [Redundancy](#redundancy)
12. [Statistics](#statistics)
13. [Large outstations](#large-outstations)
14. [Logging and troubleshooting](#logging-and-troubleshooting)
15. [Behaviours worth knowing](#behaviours-worth-knowing)
16. [Testing](#testing)

## Building

Go 1.26 or later.

```
cd src/dnp3-go
go build -ldflags="-s -w" -o ../../bin/dnp3-client ./cmd/dnp3client
```

On Windows `build.bat` in the same directory builds both DNP3 drivers into `\json-scada\bin`. The
platform build scripts (`platform-linux/build.sh`, `platform-mac/build.sh`,
`platform-windows/build.bat`) build it with everything else.

Cross-compiling needs nothing but `GOOS` and `GOARCH`.

> **Rebuild any binary built against go-dnp3 before v0.5.5.** Earlier library releases had two
> command faults. Before v0.5.3 the control relay close and trip codes were transposed, so a
> command with a trip/close duration code (11, 13, 21, 23) was sent as its opposite. Before v0.5.4
> a command's point index was written in one octet, so a command for point 300 operated point 44.
> Plain pulse and latch commands (durations 1 to 4) on points 0 to 255 were never affected.

## Running

```
dnp3-client [instanceNumber] [logLevel] [configFile]
```

All three arguments are positional and optional.

| Argument | Default | Meaning |
| --- | --- | --- |
| `instanceNumber` | `1` | Which `protocolDriverInstanceNumber` to run. Several instances can run side by side, each with its own connections. |
| `logLevel` | from the instance document, else `1` | `0` none, `1` basic, `2` detailed, `3` debug. A level given here overrides the instance document. |
| `configFile` | see below | Path of `json-scada.json`. |

The configuration file is the first that exists of: the `configFile` argument, the path in the
environment variable `JS_CONFIG_FILE`, `../conf/json-scada.json`, and finally
`~/json-scada/conf/json-scada.json`.

The driver exits cleanly on `SIGINT` or `SIGTERM` (Ctrl+C). In the JSON-SCADA process manager it
is started as `DNP3` with the executable `{bin}/dnp3-client`.

### json-scada.json

Only a few keys of the shared configuration file are used:

| Key | Meaning |
| --- | --- |
| `nodeName` | Name of this node. Used for redundancy and written into the statistics. Required. |
| `mongoConnectionString` | MongoDB connection string. Must point at a replica set: change streams need one. Required. |
| `mongoDatabaseName` | Database name. Required. |
| `tlsCaPemFile`, `tlsClientPemFile`, `tlsClientPfxFile`, `tlsClientKeyPassword`, `tlsAllowInvalidHostnames`, `tlsAllowChainErrors`, `tlsInsecure` | TLS to MongoDB, when the connection string asks for it. |

## How it works

At startup the driver reads its instance document and every enabled connection of the instance,
builds the physical channels, and then waits for redundancy to make this node the active one. On
the active node, one DNP3 master session runs per connection:

1. The session connects. It clears the outstation's restart indication, disables unsolicited
   reporting, reads an **integrity poll** (all static data and pending events) and then enables
   unsolicited reporting for classes 1, 2 and 3 (unless the connection disables it).
2. Configured periodic polls are scheduled: integrity, class 0 to 3, and range scans.
3. Every measurement received, from a poll or unsolicited, is turned into a value and queued.
   A single writer drains the queue into MongoDB in batches.
4. If the connection drops, every point of the connection is marked invalid, and the driver
   reconnects with the next configured address.
5. Commands inserted into `commandsQueue` are issued to the right session and acknowledged.

The driver is built around three independent loops: the DNP3 sessions, the MongoDB writer, and
the command watcher. A slow database does not block the sessions, and a stalled outstation does
not block the writer.

### Collections touched

| Collection | Use |
| --- | --- |
| `protocolDriverInstances` | Read: instance settings and log level. Written: redundancy keep-alive. |
| `protocolConnections` | Read: connection settings. Written: the `stats` sub-document. |
| `realtimeData` | Written: `sourceDataUpdate` of tags; new tags when `autoCreateTags` is set; `invalid` on link loss. |
| `commandsQueue` | Watched for new commands; written: acknowledgement fields. |

## Configuration

### Driver instance

A document in `protocolDriverInstances`:

```json
{
  "protocolDriver": "DNP3",
  "protocolDriverInstanceNumber": 1,
  "enabled": true,
  "logLevel": 1,
  "nodeNames": ["node1", "node2"],
  "activeNodeName": "",
  "activeNodeKeepAliveTimeTag": null
}
```

| Field | Meaning |
| --- | --- |
| `protocolDriver` | Must be `DNP3`. |
| `protocolDriverInstanceNumber` | Instance number, matched against the command line. |
| `enabled` | The instance is ignored unless `true`. If no enabled instance document exists the driver stops with an error. |
| `logLevel` | Log level, unless one is given on the command line. |
| `nodeNames` | Nodes allowed to run the instance. A node whose `nodeName` is not in a non-empty list **exits with an error**. An empty list allows every node. |
| `activeNodeName`, `activeNodeKeepAliveTimeTag` | Maintained by the driver for redundancy. Do not edit. |

### Connection

One document in `protocolConnections` per outstation:

```json
{
  "protocolDriver": "DNP3",
  "protocolDriverInstanceNumber": 1,
  "protocolConnectionNumber": 1001,
  "name": "KAW2-RTU1",
  "description": "KAW2 substation RTU",
  "enabled": true,
  "commandsEnabled": true,
  "connectionMode": "TCP ACTIVE",
  "ipAddresses": ["192.168.0.10:20000", "192.168.1.10:20000"],
  "localLinkAddress": 1,
  "remoteLinkAddress": 10,
  "giInterval": 300,
  "class1ScanInterval": 0,
  "enableUnsolicited": true,
  "timeSyncMode": 0,
  "autoCreateTags": true
}
```

#### Identity and behaviour

| Field | Default | Meaning |
| --- | --- | --- |
| `protocolDriver` | | Must be `DNP3`. |
| `protocolDriverInstanceNumber` | `1` | Instance the connection belongs to. |
| `protocolConnectionNumber` | `1` | Unique number of the connection across the whole installation. It is stored in every tag acquired through it (`protocolSourceConnectionNumber`) and selects the tag `_id` range of automatically created tags. |
| `name` | `NO NAME` | Name of the connection. Prefixes every log line and every automatically created tag name. |
| `enabled` | `true` | Only enabled connections are loaded. |
| `commandsEnabled` | `true` | When `false`, commands for this connection are cancelled, and no command tags are created automatically. |
| `autoCreateTags` | `false` | Create a tag for every point the outstation reports that has none. See [Automatic tag creation](#automatic-tag-creation). |
| `enableUnsolicited` | `true` | Enable unsolicited reporting (classes 1, 2, 3) on the outstation at startup. When `false`, the driver disables it, and events arrive only by polling. |

#### Link layer

| Field | Default | Meaning |
| --- | --- | --- |
| `localLinkAddress` | `1` | The master's own DNP3 link address. |
| `remoteLinkAddress` | `1` | The outstation's DNP3 link address. |

Connections that share one physical endpoint must differ in link address; see
[Multi-drop](#multi-drop).

#### Polling

| Field | Default | Meaning |
| --- | --- | --- |
| `giInterval` | `300` | Seconds between integrity polls (all classes). `0` disables the periodic integrity poll; one still runs when the session starts. |
| `class0ScanInterval` | `0` | Seconds between class 0 polls (static data). `0` disables. |
| `class1ScanInterval` | `0` | Seconds between class 1 event polls. `0` disables. |
| `class2ScanInterval` | `0` | Seconds between class 2 event polls. `0` disables. |
| `class3ScanInterval` | `0` | Seconds between class 3 event polls. `0` disables. |
| `rangeScans` | `[]` | Array of range scans, see [Range scans](#range-scans). |

#### Time

| Field | Default | Meaning |
| --- | --- | --- |
| `timeSyncMode` | `0` | `0` no time synchronisation, `1` non-LAN (with measured delay), `2` LAN. See [Time synchronisation](#time-synchronisation). |

#### Transport

| Field | Default | Meaning |
| --- | --- | --- |
| `connectionMode` | `TCP ACTIVE` | `TCP ACTIVE`, `TCP PASSIVE`, `TLS ACTIVE`, `TLS PASSIVE`, `SERIAL` or `UDP`. Case does not matter. |
| `ipAddresses` | `[]` | Addresses `host:port` (port `20000` when omitted). For an active connection: the alternative addresses of the one outstation. For UDP: the peer. For a passive connection: not used to filter peers. |
| `ipAddressLocalBind` | | Local address `host:port` to listen on (`TCP PASSIVE`, `TLS PASSIVE`) or to bind (`UDP`). An empty host binds all interfaces. Ignored by active TCP and TLS connections. |
| `portName` | | Serial port: `COM3`, `/dev/ttyUSB0`. |
| `baudRate` | `9600` | Serial baud rate. |
| `parity` | `None` | Serial parity: `None`, `Even`, `Odd`. |
| `stopBits` | `One` | Serial stop bits: `One`, `One5`, `Two`. |
| `handshake` | `None` | Read, but flow control is not applied: serial lines run without handshaking. |
| `asyncOpenDelay` | `0` | Milliseconds to wait after opening a serial port before transmitting. |
| `localCertFilePath` | | TLS: this node's certificate (PEM). |
| `privateKeyFilePath` | | TLS: the private key of that certificate (PEM). |
| `peerCertFilePath` | | TLS: the certificate authority (or peer certificate) used to verify the outstation. |
| `allowTLSv12`, `allowTLSv13` | `true` | TLS versions offered. TLS 1.2 is the lowest supported; `allowTLSv12: false` with `allowTLSv13: true` requires TLS 1.3. |
| `allowTLSv10`, `allowTLSv11` | `false` | Not supported; if set, a warning is logged and ignored. |
| `cipherList` | | Not supported; if set, a warning is logged and ignored. Go's TLS defaults apply. |

### Range scans

`rangeScans` schedules polls of a fixed range of points, in addition to the class polls:

```json
{
  "rangeScans": [
    { "group": 30, "variation": 0, "startAddress": 0, "stopAddress": 99, "period": 10 },
    { "group": 1,  "variation": 2, "startAddress": 0, "stopAddress": 63, "period": 5 }
  ]
}
```

| Field | Default | Meaning |
| --- | --- | --- |
| `group` | `1` | DNP3 object group to read (for example `1` binary input, `10` binary output status, `20` counter, `30` analog input, `40` analog output status). |
| `variation` | `1` | Object variation. `0` lets the outstation choose its default, which is usually what is wanted. |
| `startAddress`, `stopAddress` | `0`, `0` | Point range, `0` to `65535`, `startAddress <= stopAddress`. A range outside these limits is ignored with a log message. |
| `period` | `0` | Seconds between scans. A scan with a period of `0` or less is not run. The first scan is attempted when the session starts, then one every period; a scan attempted while the link is down fails (logged at level 2) and is tried again at the next period. |

## Transports

### TCP active and TLS active

The driver dials the outstation. With several `ipAddresses`, they are alternatives for **one**
device: a second network card, a standby gateway, a backup route.

* They are tried in the order given.
* A failed attempt moves on to the next address immediately.
* The retry delay is applied once the whole list has been tried, not once per address.
* A connection that succeeds stays on its address, and only moves on when that address stops
  answering.

Blank entries are ignored. A list with no usable address is a configuration error and stops the
driver at startup.

### TCP passive and TLS passive

The driver listens on `ipAddressLocalBind` and the outstation dials in (some devices work this
way: the outstation is the TCP client). One outstation connection is served at a time.

### TLS

TLS is mutually authenticated: both ends present certificates and both verify the other. Set
`localCertFilePath`, `privateKeyFilePath` and `peerCertFilePath`. TLS 1.2 is the lowest version
the driver speaks.

### Serial

`portName`, `baudRate`, `parity` and `stopBits` configure the port; eight data bits are always
used. Serial lines run with link-layer confirmation: each frame is acknowledged, with 3
retransmissions and a one second timeout. A serial line is also treated as a shared medium, see
below.

### UDP

Set `ipAddressLocalBind` (the local `host:port`) and the peer in `ipAddresses`. Each DNP3
application message is sent as one UDP datagram. UDP is connectionless, so the link is "up" as
soon as the socket is bound; an absent outstation shows up as polls that time out. Use UDP only
on a reliable network.

### Multi-drop

Several connections that name **the same endpoint** (the same `ipAddresses`, the same
`ipAddressLocalBind`, or the same `portName`) and differ in their link addresses share one
physical channel. This is how several outstations on one serial multi-drop line, or behind one
terminal server, are configured.

* Frames are routed to the right session by link address.
* Transmission is serialised, and on a serial line or any endpoint carrying more than one
  connection the line is held for one exchange at a time.
* Two connections on one endpoint with the same link addresses cannot be told apart; the driver
  refuses to start.
* The line is not paced for you: three masters polling a slow serial line every second spend their
  time waiting for each other. At startup the driver logs a warning when the configured intervals
  on a shared line imply more exchanges per second than the line can carry.

## Polling

On every connection the session starts with an integrity poll, whatever the intervals say. After
that:

* **Unsolicited reports** (`enableUnsolicited`) deliver events as the outstation produces them.
* **Class 1, 2, 3 polls** collect events at a fixed interval. They are useful as a safety net when
  unsolicited reporting is on, and necessary when it is off.
* **Class 0** reads all current static data without touching events.
* **The integrity poll** (`giInterval`) reads everything and resynchronises the picture; keep it
  on, with a long interval, as protection against missed events.
* **Range scans** read specific groups, for devices that do not assign points to classes.

Responses have a 5 second timeout. The master probes an idle link every 30 seconds, and a link
that does not answer is closed and reopened.

## Tags

A tag is a document in `realtimeData`. The driver finds the tag of a measurement by its source
address:

| Field | Meaning |
| --- | --- |
| `protocolSourceConnectionNumber` | The `protocolConnectionNumber` it is acquired from. |
| `protocolSourceCommonAddress` | The **object family** (see the table below). |
| `protocolSourceObjectAddress` | The point **index** within that family, `0` to `65535`. |

For a command tag, `protocolSourceASDU`, `protocolSourceCommandDuration` and
`protocolSourceCommandUseSBO` describe how the command is sent, see [Commands](#commands).

A measurement for which no tag exists is discarded, unless `autoCreateTags` is on.

### Object families

| `protocolSourceCommonAddress` | Family | Tag `type` | Notes |
| --- | --- | --- | --- |
| `1` | Binary input (g1, g2) | `digital` | |
| `3` | Double-bit binary input (g3, g4) | `digital` | See below. |
| `10` | Binary output status (g10, g11) | `digital` | The state of an output, for feedback. |
| `20` | Counter (g20, g22) | `analog` | |
| `23` | Frozen counter (g21, g23) | `analog` | Filed under `23`, the event group, not under 21. |
| `30` | Analog input (g30, g32) | `analog` | Frozen analog inputs are filed here too. |
| `40` | Analog output status (g40, g42) | `analog` | The setpoint currently in effect. |
| `12` | Control relay output block (g12) | `digital` | **Command** tag, see [Commands](#commands). |
| `41` | Analog output block (g41) | `analog` | **Command** tag, see [Commands](#commands). |

Static data and events of one family share one tag. Octet strings (g110, g111) are received and
discarded: there is no family to file them under.

### What is written

For every measurement the driver updates `sourceDataUpdate` of the tag:

| Field | Value |
| --- | --- |
| `valueAtSource` | The value. Binary: `0` or `1`. Counters and analogs: the number. |
| `valueStringAtSource` | Text of the value: `true`/`false` for binary points, the number for others, the raw two-bit state (`0` intermediate, `1` off, `2` on, `3` indeterminate) for double-bit points. |
| `asduAtSource` | `"<common address> 0"`, for example `"30 0"`. |
| `causeOfTransmissionAtSource` | Always `"20"`. |
| `timeTagAtSource` | The time the outstation reported, in UTC. `1970-01-01` when it reported none. |
| `timeTagAtSourceOk` | `true` when the outstation's clock was synchronised, so the time can be trusted. |
| `timeTag` | The time the driver received the value. |
| `invalidAtSource` | Communication lost, reference error, or not online. |
| `notTopicalAtSource` | The point's communication-lost flag. |
| `blockedAtSource` | The point is not online. |
| `substitutedAtSource` | The point was forced remotely or locally. |
| `overflowAtSource` | Analog over-range. |
| `carryAtSource` | Counter rollover. |
| `transientAtSource` | A double-bit point is intermediate or indeterminate. |
| `originator` | `DNP3\|<connection number>`. |

Time stamps outside 2001-09-09 to 2033-05-18 are zeroed before they are written, as a guard
against devices that report a wild clock.

Values that are not finite numbers, or whose size exceeds 1e100, are skipped.

### Double-bit points

A double-bit point is a device with two contacts. Its value is `1` for *determined on* and for
*indeterminate*, `0` for *determined off* and *intermediate*. It is marked **transient** while it
is moving (*intermediate*) or reports an impossible reading (*indeterminate*), which is what an
operator wants flagged.

### Quality

The DNP3 quality flags are reduced to the JSON-SCADA flags above:

| DNP3 flag | `sourceDataUpdate` field |
| --- | --- |
| COMM_LOST | `notTopicalAtSource`, and `invalidAtSource` |
| not ONLINE | `blockedAtSource`, and `invalidAtSource` |
| REFERENCE_ERR (analogs) | `invalidAtSource` |
| OVER_RANGE (analogs) | `overflowAtSource` |
| REMOTE_FORCED, LOCAL_FORCED | `substitutedAtSource` |
| ROLLOVER (counters) | `carryAtSource` |

### Events and ordering

An event (class poll or unsolicited) is kept as its own entry, so a point that went on, off and on
again produces three updates in order. A static value (integrity, class 0, range scan) is a
snapshot, so if a newer snapshot of the same point is waiting it replaces the older one.

### Loss of link

When a connection's link goes down, the driver marks **every tag of the connection** `invalid:
true` with the current `timeTag`. Valid data returns with the first values after reconnection.

## Automatic tag creation

With `autoCreateTags: true`, the first time a point is seen that has no tag, the driver creates
one. Tags already in the database are found at startup, so restarting the driver does not create
duplicates. Creation happens in the same batch as the value write, so a new tag has its first
value immediately.

For a point of family `G` at index `N` of connection `NAME`:

| Property | Value |
| --- | --- |
| `tag` | `NAME;G;N`, for example `KAW2-RTU1;30;5` |
| `description` | `NAME~<family description>~N` |
| `group1`, `group2` | `NAME`, and the family description |
| `type` | `digital` or `analog` per family |
| `origin` | `supervised` |
| `_id` | Allocated from the range `[connection number x 1,000,000, (connection number + 1) x 1,000,000)`, continuing after the highest `_id` already in that range |
| `protocolSource*` | The connection, family and index, so the tag is found again |
| Digital tags | `stateTextFalse`/`eventTextFalse` `FALSE`, `stateTextTrue`/`eventTextTrue` `TRUE`, `alarmState` `2` |
| Analog tags | `alarmState` `-1` |
| `invalid` | `true` until the first value arrives |

The remaining fields are the standard defaults of a new tag.

**Output points get a command tag.** When `commandsEnabled` is on, a binary output status (family
`10`) or analog output status (family `40`) also creates a **command tag** for it, and the two are
linked (`commandOfSupervised` on the status tag, `supervisedOfCommand` on the command tag), so an
operator can act on what the outstation reports:

| Status family | Command tag | Command parameters |
| --- | --- | --- |
| `10` binary output status | family `12`, same index | `protocolSourceASDU` 1, duration `3` (latch, `1` = on, `0` = off), `protocolSourceCommandUseSBO` `false` |
| `40` analog output status | family `41`, same index | `protocolSourceASDU` 3 (single precision float) |

The command tag has `origin: command` and the description suffix `-Command`. Its `_id` is allocated
first, so it is one lower than its status tag.

## Commands

### Flow

An operator action (or any application) inserts a document into `commandsQueue`. The driver:

1. Ignores the command unless this node is the **active** one and the document names a connection
   of this driver instance. A connection of another driver or instance is not this driver's to
   cancel and is left alone.
2. Cancels it if it cannot be issued (see the reasons below).
3. Sends it to the outstation and waits for the result.
4. Writes the outcome back to the document.

Commands are watched with a change stream that resumes from where it left off, so a command
inserted while the database connection was down is not lost.

### Command document

The fields the driver reads:

| Field | Meaning |
| --- | --- |
| `protocolSourceConnectionNumber` | Connection to send it on. |
| `protocolSourceCommonAddress` | `12` for a control relay output block, `41` for an analog output. |
| `protocolSourceObjectAddress` | Point index, `0` to `65535`. |
| `protocolSourceASDU` | Analog: encoding of the setpoint (below). Ignored for `12`. |
| `protocolSourceCommandDuration` | CROB: how the value is encoded (below). Ignored for `41`. |
| `protocolSourceCommandUseSBO` | `true`: select-before-operate. `false`: direct operate. |
| `value` | The command value. |
| `timeTag` | When the command was created, used for expiry. |

### Outcome

On completion the driver sets:

| Field | Meaning |
| --- | --- |
| `delivered` | `true` |
| `ack` | `true` when the outstation executed the command |
| `ackTimeTag` | When the result arrived |
| `resultDescription` | Text of the result |

When the command is **cancelled** instead (never sent), `cancelReason` is set:

| `cancelReason` | Cause |
| --- | --- |
| `connection_not_found` | The connection exists but has no running session. |
| `not_connected` | The link to the outstation is down. |
| `cmds_disabled` | The connection has `commandsEnabled: false`. |
| `expired` | The command is more than 10 seconds old. |
| `unsupported_group` | `protocolSourceCommonAddress` is neither `12` nor `41`. |
| `invalid_address` | The object address is outside `0` to `65535`. |

`resultDescription` on a sent command:

| Text | Meaning |
| --- | --- |
| `SUCCESS` | The outstation executed the command. |
| `FAILURE_RESPONSE_TIMEOUT` | No answer within the response timeout. |
| `FAILURE_START_TIMEOUT` | The command could not be started (the session was reset while it waited). |
| `FAILURE_NO_COMMS` | The link was down or closing. |
| `FAILURE_MESSAGE_FORMAT_ERROR` | The request could not be formed. |
| `FAILURE_BAD_RESPONSE (<status>)` | The outstation refused it; `<status>` is the DNP3 command status, for example `NOT_SUPPORTED`, `BLOCKED`, `LOCAL`, `HARDWARE_ERROR`. |
| `UNKNOWN` | Anything else. |

A command that has not been answered after 60 seconds is abandoned.

### Control relay output blocks (family `12`)

`protocolSourceCommandDuration` selects how `value` (non-zero is *on*, zero is *off*) becomes a
control code. Pulse commands use 100 ms on and 100 ms off, one pulse.

| Duration | Value on | Value off |
| --- | --- | --- |
| `1` | pulse on | pulse off |
| `2` | pulse off | pulse on |
| `3` | latch on | latch off |
| `4` | latch off | latch on |
| `11` | pulse on, **close** | pulse off, **trip** |
| `13` | latch on, **close** | latch off, **trip** |
| `21` | pulse on, **trip** | pulse off, **close** |
| `23` | latch on, **trip** | latch off, **close** |
| other (`0`, `10`, `12`, `20`, `22`, ...) | an empty control block, which operates nothing |

Durations `10`, `12`, `20` and `22` are not defined, and send a block that operates nothing rather
than guess which coil of a breaker to operate.

Close and trip are the trip/close code of the control, bits 7-6 of the control code: close is
`0x40` and trip is `0x80`. A device that implements the two-output (complementary) model needs the
trip/close durations; a device that implements a single output needs the plain pulse or latch.

### Analog outputs (family `41`)

`protocolSourceASDU` selects the encoding of the setpoint:

| `protocolSourceASDU` | Sent as |
| --- | --- |
| `1` | 32-bit integer |
| `2` | 16-bit integer |
| `3` (and anything else) | single precision floating point |
| `4` | double precision floating point |

## Time synchronisation

`timeSyncMode` makes the driver set the outstation's clock:

| Mode | Procedure |
| --- | --- |
| `0` | None. |
| `1` | Non-LAN: the master measures the round trip first and corrects for it. |
| `2` | LAN: the clock is written directly. |

The clock is written about two seconds after each connection comes up and then every 60 seconds
while it is connected. A failure is logged at level 2 and retried at the next interval.

## Redundancy

Several nodes can run the same driver instance for high availability. Exactly one is **active**:
only the active node runs DNP3 sessions, writes values, issues commands and writes statistics. The
others wait on standby, with their channels built but no sessions running.

* The active node writes `activeNodeName` and a keep-alive time into the instance document every
  5 seconds.
* A standby node that sees the keep-alive unchanged for 5 consecutive checks takes over. Takeover
  takes about 25 to 30 seconds. The decision does not compare clocks, so it does not depend on the
  nodes agreeing about the time.
* A node that lost sight of the active node and finds another one active yields, after a short
  random pause so that two nodes do not flip together.
* When no node is active, a standby takes over after the same wait.

Listed in `nodeNames`, nodes may be on different machines: all of them must be able to reach the
outstations.

## Statistics

Every 5 seconds the active node writes a `stats` sub-document into each connection document:

| Field | Meaning |
| --- | --- |
| `isConnected` | The DNP3 session has a live link. |
| `numBytesRx`, `numBytesTx` | Bytes received and sent on the channel. |
| `numOpen`, `numClose` | Times the channel was opened and closed. |
| `numOpenFail` | Failed attempts to open the channel, counted per attempt (each address tried counts). A deliberate shutdown is not a failure. |
| `numLinkFrameRx` | Link frames received without error. |
| `numLinkFrameTx` | **Requests issued** by the master (tasks run). Nothing counts frames on the way out, so this counts requests. |
| `numHeaderCrcError`, `numBodyCrcError` | Link frames rejected for a bad header or body checksum. |

The frame and CRC counters come from the decoder shared by the connections on one channel, so
connections on the same multi-drop endpoint report the same numbers.

The reason a connection would not open is logged with the failure, for example
`Connection attempt failed: channel: opening COM3: Serial port not found`.

## Large outstations

The driver is built to scan devices with tens of thousands of points and create a tag for every one
of them, without losing any.

* **No point is ever dropped.** Values wait in a queue for the MongoDB writer. Static values for
  one point collapse into the newest; events are kept in sequence. Past 50,000 waiting values,
  events for one point collapse too, and the writer logs how many were folded together. About
  8 MB of memory is used at that limit, and only while the writer is behind.
* **The multi-drop bus queues 4096 link frames per session.** A large integrity response arrives
  as hundreds of link frames back to back; a smaller queue loses one, and with it a whole
  application message and every tag it would have created.
* **The database needs the standard `realtimeData` index** on `(protocolSourceConnectionNumber,
  protocolSourceCommonAddress, protocolSourceObjectAddress)`, created by `mongo_seed`. Without it
  every update scans the collection, a large batch cannot finish inside its 30 second timeout, and
  updates stall while tag creation carries on. The driver says so in the log when a large bulk
  write times out.

Measured, on a freshly seeded database: 12,000 points created as tags and all 12,000 updated
within 15 seconds of the first poll.

## Logging and troubleshooting

Log levels: `0` start-up banner, fatal errors and redundancy state only, `1` basic (connections,
tag creation, commands, protocol-stack warnings), `2` detailed (every received header, every
value written, time sync, protocol-stack information), `3` debug (adds the protocol stack's debug
trace). Log lines of a connection start with its `name`.

Typical lines:

```
KAW2-RTU1 - Connection configured.
KAW2-RTU1 - Channel state: OPEN
KAW2-RTU1 - INSERT NEW TAG: KAW2-RTU1;30;5
KAW2-RTU1 - Issuing command ... useSBO=false
KAW2-RTU1 - Command result: SUCCESS
Redundancy - ACTIVATING this Node!
```

| Symptom | Likely cause |
| --- | --- |
| `driver instance not found` at startup | No enabled instance document with the instance number and `protocolDriver: DNP3`. |
| `no DNP3 connections found` | No enabled connection documents for the instance. |
| The driver says it is inactive and waits | Another node holds the instance; wait for the takeover period, or check `nodeNames` and `activeNodeName`. |
| Connection never opens | Read `numOpenFail` and the logged reason: refused (outstation not listening or wrong port), no route, or serial port missing. For a passive connection, the outstation is not dialling in. |
| Connects, but polls time out | Wrong link addresses (`localLinkAddress` / `remoteLinkAddress`), or the outstation requires a different master address. |
| Connects and values flow, but screens stay empty | `cs_data_processor` is not running, or the tags do not match the source addresses. |
| Tags created but not updated on a big device | The `realtimeData` index is missing. |
| Values arrive but with the wrong index | `protocolSourceObjectAddress` is the DNP3 point index, starting at zero. |
| Commands cancelled `not_connected` | The link is down, or this node is not the active one. |
| Commands fail `FAILURE_BAD_RESPONSE (NOT_SUPPORTED)` | The outstation has no such control point, or does not accept that control code: check the duration (a single-output device needs `1`..`4`, not `11`..`23`). |
| Connection shows `isConnected: true` on UDP with no data | UDP is connectionless; the link is up when the socket is. Check addresses and firewall. |

A DNP3 browser that shows fewer points than the outstation declares may be losing updates itself
(a slow consumer in the viewer); check the viewer's dropped counter before suspecting the
outstation or the driver. Reading one family on its own with a range scan gets around it.

## Behaviours worth knowing

* **Unsolicited reporting is a request, not a guarantee.** If the outstation does not support it,
  rely on class polls.
* **Events and statics are separate sources of the same tag.** Both write `sourceDataUpdate`; the
  data processor decides what to do with repeated values.
* **The timestamps come from the outstation.** A device that does not stamp its events produces
  `timeTagAtSource` of 1970; the data processor then uses `timeTag`.
* **Frozen analog inputs** (groups 31 and 33) are delivered as analog inputs, family `30`.
* **Octet strings are discarded.** Device attributes (group 0) and file transfer are not read.
* **The driver does not filter peers.** A passive connection accepts any dialling outstation that
  speaks the right link addresses.

## Testing

```
go test ./...
go test -race ./...
go vet ./... && gofmt -l .
```

The tests run a real master against a real outstation over an in-memory link, and over real TCP,
TLS, UDP and (when a port pair is supplied) serial sockets. They need no database and no hardware.
They cover the polling, the tag documents, the value queue, the CROB codes on the wire, commands to
points above 255, reconnection, and the alternative addresses.
