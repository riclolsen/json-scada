# DNP3 Server Driver (Go)

`dnp3-server` is the JSON-SCADA DNP3 **outstation** (slave). It publishes JSON-SCADA tags to DNP3
masters, such as a control centre, a SCADA front end or an engineering tool, as binary, analog and
counter points. Changes are reported as time-stamped events, and controls received from the master
are queued as JSON-SCADA commands.

It is a single static binary with no native dependencies. The protocol stack is
[go-dnp3](https://github.com/dscsystems/go-dnp3) (pure Go); the driver adds the JSON-SCADA side:
MongoDB, point mapping, automatic address assignment, statistics and commands.

| | |
| --- | --- |
| Binary | `dnp3-server` (`dnp3-server.exe` on Windows) |
| `protocolDriver` | `DNP3_SERVER` |
| Role | DNP3 outstation; one independent outstation per `protocolConnections` document |
| Transports | TCP (passive and active), TLS (passive and active), serial, UDP |
| Publishes | binary, double-bit binary, counters, frozen counters, analogs, binary and analog output status |
| Receives | control relay output blocks and analog outputs (as commands), clock writes, device attribute reads |
| Events | every value change, with a time stamp, in event classes 1, 2 and 3; polled or unsolicited |

## Contents

1. [Building](#building)
2. [Running](#running)
3. [How it works](#how-it-works)
4. [Configuration](#configuration)
5. [Transports](#transports)
6. [Tags and destinations](#tags-and-destinations)
7. [Object families and variations](#object-families-and-variations)
8. [Values, quality and time](#values-quality-and-time)
9. [Events](#events)
10. [Automatic destinations](#automatic-destinations)
11. [Commands](#commands)
12. [Device attributes](#device-attributes)
13. [Time](#time)
14. [Statistics](#statistics)
15. [Logging and troubleshooting](#logging-and-troubleshooting)
16. [Behaviours worth knowing](#behaviours-worth-knowing)
17. [Testing](#testing)

## Building

Go 1.26 or later.

```
cd src/dnp3-go
go build -ldflags="-s -w" -o ../../bin/dnp3-server ./cmd/dnp3server
```

On Windows `build.bat` in the same directory builds both DNP3 drivers into `\json-scada\bin`. The
platform build scripts (`platform-linux/build.sh`, `platform-mac/build.sh`,
`platform-windows/build.bat`) build it with everything else.

> **Rebuild any binary built against go-dnp3 before v0.5.5.** Before v0.5.3 the library had the
> control relay close and trip codes transposed, so a server built on it read a *close* sent by any
> standard master as a *trip* (value 0) and the reverse. Plain pulse and latch controls were not
> affected.

## Running

```
dnp3-server [instanceNumber] [logLevel] [configFile]
```

All three arguments are positional and optional.

| Argument | Default | Meaning |
| --- | --- | --- |
| `instanceNumber` | `1` | Which `protocolDriverInstanceNumber` to run. |
| `logLevel` | from the instance document, else `1` | `0` none, `1` basic, `2` detailed, `3` debug. A level given here overrides the instance document. |
| `configFile` | see below | Path of `json-scada.json`. |

The configuration file is the first that exists of: the `configFile` argument, the environment
variable `JS_CONFIG_FILE`, `../conf/json-scada.json`, and finally `/json-scada/conf/json-scada.json`.

The driver exits cleanly on `SIGINT` or `SIGTERM` (Ctrl+C). In the JSON-SCADA process manager it is
started as `DNP3_SERVER` with the executable `{bin}/dnp3-server`.

### json-scada.json

| Key | Meaning |
| --- | --- |
| `nodeName` | Name of this node. Checked against the instance's `nodeNames` and written into the statistics. |
| `mongoConnectionString` | MongoDB connection string. Must point at a replica set: change streams need one. Required. |
| `mongoDatabaseName` | Database name. Required. |
| `tlsCaPemFile`, `tlsClientPemFile`, `tlsClientPfxFile`, `tlsClientKeyPassword`, `tlsAllowInvalidHostnames`, `tlsAllowChainErrors`, `tlsInsecure` | TLS to MongoDB, when the connection string asks for it. |

## How it works

At startup the driver:

1. Reads its instance document and every enabled connection of the instance.
2. For connections with `autoCreateTags`, gives every tag that has no DNP3 address one
   ([Automatic destinations](#automatic-destinations)).
3. Builds the physical channels, and for each connection **sizes an outstation database** from the
   tags destined for it: for each object family, the highest point index in use plus one.
4. Loads the current value of every tag into that database, applies the point configuration
   (variations and event classes), and clears the event buffer, so the master's first integrity
   poll sees the present state and no history.
5. Starts the outstations. Each listens for, or connects to, its master.

From then on a change stream on `realtimeData` delivers every tag change to the outstations it is
destined for. A changed value becomes an event; a master reads it by polling or receives it
unsolicited. Controls from the master are looked up and queued on `commandsQueue`.

Every enabled connection is a separate DNP3 outstation with its own link address, point database,
event buffer and statistics. Unlike the client, the server has **no redundancy election**: it runs
on every node whose `nodeName` the instance allows, and each running copy answers on its own
transport.

### Collections touched

| Collection | Use |
| --- | --- |
| `protocolDriverInstances` | Read: instance settings, log level, allowed nodes. |
| `protocolConnections` | Read: connection settings. Written: the `stats` sub-document. |
| `realtimeData` | Read: tags destined for the connections, via a change stream. Written: `protocolDestinations`, when automatic destinations are created. |
| `commandsQueue` | Written: commands received from a master. |

The server only distributes tags with `origin: supervised`, both at startup and for later changes.
A command tag is never published as data.

## Configuration

### Driver instance

A document in `protocolDriverInstances`:

```json
{
  "protocolDriver": "DNP3_SERVER",
  "protocolDriverInstanceNumber": 1,
  "enabled": true,
  "logLevel": 1,
  "nodeNames": ["node1"]
}
```

| Field | Meaning |
| --- | --- |
| `protocolDriver` | Must be `DNP3_SERVER`. |
| `protocolDriverInstanceNumber` | Instance number, matched against the command line. |
| `enabled` | The driver stops with an error unless the instance exists and is `true`. |
| `logLevel` | Log level, unless one is given on the command line. |
| `nodeNames` | Nodes allowed to run the instance. If the list is not empty and this node's `nodeName` is not in it, the driver stops with an error. An empty or absent list allows every node. |

### Connection

One document in `protocolConnections` per outstation:

```json
{
  "protocolDriver": "DNP3_SERVER",
  "protocolDriverInstanceNumber": 1,
  "protocolConnectionNumber": 2001,
  "name": "KAW2-GW",
  "description": "KAW2 substation gateway",
  "enabled": true,
  "commandsEnabled": true,
  "autoCreateTags": false,
  "connectionMode": "TCP PASSIVE",
  "ipAddressLocalBind": "0.0.0.0:20000",
  "ipAddresses": [],
  "localLinkAddress": 10,
  "remoteLinkAddress": 1,
  "enableUnsolicited": true,
  "serverQueueSize": 1000,
  "topics": []
}
```

#### Identity and behaviour

| Field | Default | Meaning |
| --- | --- | --- |
| `protocolDriver` | | Must be `DNP3_SERVER`. |
| `protocolDriverInstanceNumber` | | Instance the connection belongs to. |
| `protocolConnectionNumber` | `0` | Unique number of the connection across the whole installation. Tags are destined for a connection by this number. |
| `name` | | Prefixes every log line; reported to masters as the device name. |
| `description` | | Free text; reported to masters as the device location. |
| `enabled` | `false` | Only enabled connections are loaded. |
| `commandsEnabled` | `false` | Accept controls from the master. When `false`, every control is answered `NOT_SUPPORTED`. |
| `autoCreateTags` | `false` | Assign DNP3 addresses to tags that have none. See [Automatic destinations](#automatic-destinations). |
| `topics` | `[]` | Restricts which tags automatic assignment may pick: only tags whose `group1` **contains** one of the strings. Empty means every tag. Used only when `autoCreateTags` is on. |
| `hoursShift` | `0` | Hours added to the time stamp of every event of this connection. Combined with the destination's own `protocolDestinationHoursShift`. |
| `timeSyncMode`, `timeSyncInterval` | `0` | Accepted and ignored. See [Time](#time). |

#### Link layer and events

| Field | Default | Meaning |
| --- | --- | --- |
| `localLinkAddress` | `1` | The outstation's own DNP3 link address. |
| `remoteLinkAddress` | `1` | The master's link address. Frames from any other source address are not answered. |
| `enableUnsolicited` | `true` | Let the outstation send unsolicited event reports (once the master enables them). When `false`, events can only be polled. |
| `serverQueueSize` | `1000` | Capacity of the event buffer of this connection, in events across all classes. When full, the oldest event is discarded and the master is told by the event buffer overflow indication. |

#### Transport

| Field | Default | Meaning |
| --- | --- | --- |
| `connectionMode` | `TCP PASSIVE` | `TCP PASSIVE`, `TCP ACTIVE`, `TLS PASSIVE`, `TLS ACTIVE`, `SERIAL` or `UDP`. Case does not matter. |
| `ipAddressLocalBind` | | `host:port` to listen on (`TCP PASSIVE`, `TLS PASSIVE`) or to bind (`UDP`). An empty host binds all interfaces; the default port is `20000`. |
| `ipAddresses` | `[]` | `TCP PASSIVE`, `TLS PASSIVE`: the hosts allowed to connect, empty meaning any. `TCP ACTIVE`, `TLS ACTIVE`: the alternative addresses of the one master to dial. `UDP`: the peer. |
| `portName` | | Serial port: `COM3`, `/dev/ttyUSB0`. |
| `baudRate` | `9600` | Serial baud rate. |
| `parity` | `None` | Serial parity: `None`, `Even`, `Odd`. |
| `stopBits` | `One` | Serial stop bits: `One`, `One5`, `Two`. |
| `handshake` | `None` | Read, but flow control is not applied: serial lines run without handshaking. |
| `asyncOpenDelay` | `0` | Milliseconds to wait after opening a serial port before transmitting. |
| `localCertFilePath` | | TLS: this node's certificate (PEM). |
| `privateKeyFilePath` | | TLS: the private key of that certificate (PEM). |
| `peerCertFilePath` | | TLS: the certificate authority (or peer certificate) used to verify the master. |
| `allowTLSv12`, `allowTLSv13` | `true` | TLS versions offered. TLS 1.2 is the lowest supported; `allowTLSv12: false` with `allowTLSv13: true` requires TLS 1.3. |
| `allowTLSv10`, `allowTLSv11` | `false` | Not supported; a warning is logged if set. |
| `cipherList` | | Not supported; a warning is logged if set. Go's TLS defaults apply. |

## Transports

### TCP passive and TLS passive

The usual case: the driver listens on `ipAddressLocalBind` and the master connects. One master
connection is served at a time on an endpoint. When `ipAddresses` is not empty, only connections
from those hosts are accepted (the host part is compared; the port is ignored); any other peer is
dropped.

### TCP active and TLS active

For masters that listen (dial-out outstations, for example behind a one-way firewall rule): the
driver connects to the master. With several `ipAddresses` they are alternatives for **one**
master, tried in order; a failure moves to the next address at once and the retry delay is
applied once the whole list has been tried. A connection that works stays on its address.

### TLS

TLS is mutually authenticated: both ends present certificates and both verify the other. Set
`localCertFilePath`, `privateKeyFilePath` and `peerCertFilePath`.

### Serial

`portName`, `baudRate`, `parity` and `stopBits` configure the port; eight data bits are always
used. Serial lines run with link-layer confirmation (3 retransmissions, one second timeout) and
are treated as a shared medium.

### UDP

Set `ipAddressLocalBind` and the peer in `ipAddresses`. Each DNP3 application message is one UDP
datagram, so a lost datagram loses a whole message. UDP is connectionless: `isConnected` is true as
soon as the socket is bound, and an absent master shows only as no traffic.

### Several outstations on one endpoint

Connections that name the same endpoint (the same `ipAddressLocalBind`, `ipAddresses` or
`portName`) and differ in `localLinkAddress` share one channel, each answering only frames
addressed to its own link address. This presents several outstations on one serial line, or one
port, as a multi-drop line. Two connections that cannot be told apart (the same endpoint and the
same link addresses) are a configuration error and the driver stops at startup.

## Tags and destinations

A tag is published to a connection by a **destination**: an entry in the tag's
`protocolDestinations` array.

```json
"protocolDestinations": [
  {
    "protocolDestinationConnectionNumber": 2001,
    "protocolDestinationCommonAddress": 30,
    "protocolDestinationObjectAddress": 12,
    "protocolDestinationASDU": 6,
    "protocolDestinationKConv1": 1,
    "protocolDestinationKConv2": 0,
    "protocolDestinationHoursShift": 0
  }
]
```

A tag may have several destinations, on the same or on different connections.

| Field | Default | Meaning |
| --- | --- | --- |
| `protocolDestinationConnectionNumber` | | The connection (outstation) the tag is published on. |
| `protocolDestinationCommonAddress` | | The **object family**, see the table below. |
| `protocolDestinationObjectAddress` | | The point **index**, `0` to `65535`. |
| `protocolDestinationASDU` | `0` | Selects the variations in which the point is sent. See [Object families and variations](#object-families-and-variations). |
| `protocolDestinationKConv1` | `1` | Scale factor. For a digital tag, `-1` inverts the value. |
| `protocolDestinationKConv2` | `0` | Offset. The value sent is `value x KConv1 + KConv2`. |
| `protocolDestinationHoursShift` | `0` | Hours added to the time stamp of this destination's events. |
| `protocolDestinationCommandDuration`, `protocolDestinationCommandUseSBO`, `protocolDestinationGroup` | | Written by automatic assignment for compatibility. Not used by the server: a master chooses select-before-operate or direct operate itself. |

### What is distributed

Only tags with `origin: supervised` are loaded and sized. Other tags are ignored when the
outstation is built.

The outstation database of a connection holds, for each family, the points `0` up to the highest
index any destination uses. An index with no tag (a gap) still exists: it reads as value zero with
no quality flags, that is, not online. Use contiguous indexes.

## Object families and variations

### Families

| `protocolDestinationCommonAddress` | Family | Static group | Event group |
| --- | --- | --- | --- |
| `1`, `2` | Binary input | g1 | g2 |
| `3`, `4` | Double-bit binary input | g3 | g4 |
| `20`, `22` | Counter | g20 | g22 |
| `21`, `23` | Frozen counter | g21 | g23 |
| `10`, `11` | Binary output status | g10 | g11 |
| `30`, `32` | Analog input | g30 | g32 |
| `40`, `42` | Analog output status | g40 | g42 |

Either number of a pair selects the family. Any other value (an unknown group number) is treated as
an **analog input**. Group `110`/`111` (octet string) destinations make the points exist but nothing is ever written
to them, and group `50`/`52` (time and interval) destinations are not supported and are skipped
with one warning per connection.

### Variations

Each point is reported in its own variation, chosen by `protocolDestinationASDU`. A master that
asks for "default" variations gets each point in the one configured here; a master that asks for
a specific variation gets that one for the whole range.

Every **event** variation includes the time of the change. The static variations carry no time,
which is how DNP3 defines them.

#### Binary input, double-bit input, binary output status

The ASDU is ignored: g1 variation 2 (flags) with g2 variation 2 (flags and time) for binary
inputs, g3v2 with g4v2 for double-bit inputs, g10v2 with g11v2 for binary output status.

#### Analog input

| `protocolDestinationASDU` | Static | Event | Value is sent as |
| --- | --- | --- | --- |
| `1` | g30v1 | g32v3 | 32-bit integer |
| `2` | g30v2 | g32v4 | 16-bit integer |
| `3` | g30v3 | g32v3 | 32-bit integer, no flags |
| `4` | g30v4 | g32v4 | 16-bit integer, no flags |
| `5`, `0`, anything else | g30v5 | g32v7 | single precision float |
| `6` | g30v6 | g32v8 | double precision float |
| `7` | g30v5 | g32v7 | single precision float |
| `8` | g30v6 | g32v8 | double precision float |

Integer variations drop the fraction, and a value beyond the range of the integer is sent
saturated with the OVER_RANGE flag. Floating point is the default and the one to use unless the
master insists on integers.

**Variations without flags** (analog 3 and 4, counter 5 and 6, and their frozen and event
counterparts where they exist) are only sent for a point whose quality is normal. A point that is
invalid, forced or overflowed is sent in the matching variation *with* flags instead
(g30v3 as g30v1, g20v5 as g20v1), because a master would otherwise read its value as good.

#### Counter and frozen counter

| `protocolDestinationASDU` | Counter: static / event | Frozen counter: static / event |
| --- | --- | --- |
| `1`, `3`, `0`, anything else | g20v1 / g22v5 | g21v1 / g23v5 |
| `2`, `4` | g20v2 / g22v6 | g21v2 / g23v6 |
| `5`, `7` | g20v5 / g22v5 | g21v5 / g23v5 |
| `6`, `8` | g20v6 / g22v6 | g21v6 / g23v6 |
| `9`, `11` | | g21v1 / g23v5 |
| `10`, `12` | | g21v2 / g23v6 |

Counters are 32-bit (variations 1 and 5) or 16-bit (2 and 6); events always carry the time.

#### Analog output status

| `protocolDestinationASDU` | Static | Event | Value is sent as |
| --- | --- | --- | --- |
| `1` | g40v1 | g42v3 | 32-bit integer |
| `2` | g40v2 | g42v4 | 16-bit integer |
| `3`, `0`, anything else | g40v3 | g42v7 | single precision float |
| `4` | g40v4 | g42v8 | double precision float |

### Event classes

Every point is assigned to an event class, so every change of every point can be reported:

| Family | Class |
| --- | --- |
| Binary input | 1 |
| Double-bit input, analog input, counter, binary output status, analog output status | 2 |
| Frozen counter | 3 |

No deadband is applied: any change of value is an event. (A point that is written again with the
same value and quality produces no event.)

## Values, quality and time

### Values

| Family | Source | Sent value |
| --- | --- | --- |
| Binary input, binary output status | `value` of a digital tag | `true`/`false`; inverted when `KConv1` is `-1` |
| Double-bit input | `value` of a digital tag | `true` is *determined on*, `false` is *determined off*; with `transient` set, `true` is *indeterminate* and `false` is *intermediate* |
| Counter, frozen counter | `value` | `value x KConv1 + KConv2`, as an unsigned 32-bit integer. Keep these tags non-negative |
| Analog input, analog output status | `value` | `value x KConv1 + KConv2` |

### Quality

The quality flags sent with every point come from the tag:

| Tag field | DNP3 flag |
| --- | --- |
| not `invalid` | ONLINE |
| `invalid` | COMM_LOST (and not ONLINE) |
| `transient` | COMM_LOST (and not ONLINE), for binary, counter and analog points. A double-bit point expresses it in its value instead |
| `substituted` | LOCAL_FORCED |
| `overflow` | OVER_RANGE on analogs and analog output status; ROLLOVER on counters and frozen counters |

### Time stamps

Each event carries a time, taken from the tag in this order:

1. `timeTagAtSource`, the time the field device reported, when present.
2. `timeTag`, the time the tag was last updated in JSON-SCADA.
3. The time the driver applied the change, when the tag has neither.

`protocolDestinationHoursShift` plus the connection's `hoursShift` is added to the time.
A measurement is never sent without a time: a missing time is replaced by the current one rather
than sent as invalid.

Whether a master is told the time is **synchronised** is a property of the outstation, not of the
tag. It is *unsynchronised* from the start of the driver until a master writes the clock, and
*synchronised* once one has (see [Time](#time)); the tag's own `timeTagAtSourceOk` does not change
it. A master that never synchronises the outstation therefore sees every time as unsynchronised,
including accurate field times.

### Changes from the database

The driver watches `realtimeData` for updates that are **not** writes to `sourceDataUpdate`, that is,
changes made by the JSON-SCADA data processor and by replacements, and applies the changed tag to
every connection it is destined for. All destinations of a tag on one connection are applied
together, so a master that polls mid-change does not see half of it.

If the connection to MongoDB is lost, the change stream resumes from where it stopped when it is
back, and the driver then re-reads every distributed tag and applies it, so changes that happened
during the outage are not lost to the masters.

## Events

When a point changes, the new value is queued as an event in the point's class, with its time. A
master collects events:

* **By polling** classes 1, 2 and 3 (and integrity polls).
* **Unsolicited**, when `enableUnsolicited` is on and the master has enabled unsolicited reporting.
  The outstation sends up to 20 events in one report, after waiting up to 200 ms for more to
  accumulate; the report must be confirmed within 5 seconds, and is repeated up to 3 times.
  Events that do not fit one report stay queued for the next.

Events are removed from the buffer only when the master confirms them, so a response lost on the
way is sent again. The buffer holds `serverQueueSize` events; beyond that the oldest are
discarded and the *event buffer overflow* indication is raised until the master has read the
rest. `eventsQueued` and `confirmTimeouts` in the statistics show it.

Responses are limited to 2048 octets per fragment and larger ones are sent as a series of
fragments; the master confirms each.

At startup the event buffer is cleared after the initial load, so a master does not receive the
history of tags that were already there.

## Automatic destinations

With `autoCreateTags: true` the driver gives DNP3 addresses to tags that have no destination on the
connection, so a new connection can be brought up without assigning point numbers by hand. It
runs at startup, before the outstation is sized, and adds nothing on a later run for tags that
already have a destination.

The passes, in order:

| Pass | Tags picked (`type`, `origin`) | Becomes | ASDU |
| --- | --- | --- | --- |
| 1 (only with `commandsEnabled`) | digital, `command` | group 12 (control relay output block) | 1 |
| 2 (only with `commandsEnabled`) | analog, `command` | group 41 (analog output block) | 3 |
| 3 | digital, `supervised` | group 1 (binary input) | 2 |
| 4 | analog, `supervised` | group 30 (analog input), double precision | 6 |

Within a pass:

* Tags are taken in `_id` order, skipping tags whose `group1` does not match `topics` (when
  `topics` is not empty).
* Each family has its **own** address space. Addresses continue after the highest one already used
  for that family on this connection, so a tag at group 30 does not push the next group 12
  address along.
* A tag that already has a destination on this connection is skipped, whichever family it is in.
* The addresses stop at 65535; beyond that a message is logged and no more are assigned.

### Output status of a command

Each command gets an **output status** so a master can read back what it operated: a control relay
output block at index N is operated through binary output N, and its state is group 10 index N.

| Command | Status | Placed on |
| --- | --- | --- |
| group 12 index N | group 10 index N, ASDU 2 | the command's `supervisedOfCommand` tag |
| group 41 index N | group 40 index N, ASDU 3 | the command's `supervisedOfCommand` tag |

The status carries the **same index as the command**, because that is what the protocol means; it
cannot be moved to another index. If something already occupies that index, the clash is logged
and no status is created: the command still works. A command with no supervised tag (a blind
command) gets no status. Because the supervised tag now has its destination, the supervised passes
skip it: a controllable point is published once, as an output, not also as an input.

## Commands

A master operates a point by sending a control relay output block (group 12) or an analog output
(group 41) for a point index. The driver looks up the command tag and, if found, **queues a
command** on `commandsQueue` and answers the master. Whatever is acting on that tag (normally the
driver that owns the tag's source, such as the DNP3, IEC 60870-5 or OPC UA client) then carries the
command out; the DNP3 server does not report that outcome back to the master.

### Lookup

For a control at index N on a connection, the command tag is the tag with:

* `origin: command`,
* `type: digital` for a control relay output block, `analog` for an analog output,
* a destination with this connection, group `12` (or `41`) and object address N.

| Situation | Answer to the master |
| --- | --- |
| `commandsEnabled` is `false` | `NOT_SUPPORTED` |
| No such command tag | `NOT_SUPPORTED` |
| The tag has `enabled: false` | `BLOCKED` |
| The database is unreachable, or the command could not be queued | `DOWNSTREAM_FAIL` |
| Control relay output block without an operation (no on/off/latch/pulse) | `FORMAT_ERROR` |
| Control relay output block with both trip and close set | `FORMAT_ERROR` |
| Queued | `SUCCESS` |

A **select** only checks that the tag exists and is enabled; nothing is operated or queued until
the **operate**. Direct operate is accepted too. Whether the master uses select-before-operate is
the master's choice.

### Command value

* **Control relay output block:** close gives `1`, trip gives `0`. A control with no trip/close
  code (a plain pulse or latch) is decided by its operation: pulse on and latch on give `1`, pulse
  off and latch off give `0`. When the destination has `KConv1` equal to `-1`, the value is inverted.
* **Analog output:** the setpoint, whichever of the four variations the master used
  (32-bit, 16-bit, single, double), as `value x KConv1 + KConv2`.

### Queued command document

| Field | Value |
| --- | --- |
| `protocolSourceConnectionNumber`, `protocolSourceCommonAddress`, `protocolSourceObjectAddress`, `protocolSourceASDU`, `protocolSourceCommandDuration`, `protocolSourceCommandUseSBO` | Copied from the **command tag's source fields**, so the command is routed to whatever acquires that tag |
| `pointKey` | The tag's `_id` |
| `tag` | The tag name |
| `value`, `valueString` | The command value; text of an analog setpoint (empty for a relay output) |
| `originatorUserName` | `DNP3 Server Driver` |
| `originatorIpAddress` | Empty |
| `timeTag` | Time the command was queued |

Commands older than their driver's expiry window (10 seconds for the JSON-SCADA drivers) are
cancelled by that driver; the DNP3 server does not wait for the result.

## Device attributes

A master can read the **device attributes** (group 0) of each outstation: who and what it is,
which software it runs, and how many points of each type it has. A commissioning engineer facing
several identical-looking gateways reads this instead of trusting a drawing. A master can read one
attribute, all of them (variation 254), or the list of attributes (variation 255). They are
read-only. Variation numbers are those of IEEE 1815-2012 set 0.

**Identity**

| Variation | Attribute | Value |
| --- | --- | --- |
| 252 | manufacturer name | `{json:scada}` |
| 250 | product name and model | `JSON-SCADA DNP3 Outstation Server (Go)` |
| 242 | software version | the driver version |
| 243 | hardware version | the host platform, for example `linux/amd64` |
| 247 | device name | the connection `name` |
| 245 | location | the connection `description`; omitted when empty |
| 246 | ID code | the `protocolConnectionNumber` |
| 208 | system name | `JSON-SCADA` |
| 211 | user-specific attribute sets | empty: none defined |

**Capacity**, from the database of the connection. Each point type reports whether it produces
events, its highest index and its count; a type the connection does not have reports no events,
count `0` and highest index `0`.

| Point type | Events supported | Max index | Count |
| --- | --- | --- | --- |
| binary inputs | 237 | 238 | 239 |
| double-bit binary inputs | 234 | 235 | 236 |
| analog inputs | 231 | 232 | 233 |
| counters | 227 | 228 | 229 |
| binary outputs | 222 | 223 | 224 |
| analog outputs | 219 | 220 | 221 |

| Variation | Attribute | Value |
| --- | --- | --- |
| 225, 226 | frozen counter events, frozen counters supported | yes when the connection has frozen counters |
| 230 | frozen analog inputs supported | no |
| 240, 241 | max transmit and receive fragment size | 2048 and 2048 |
| 216 | max binary outputs per request | 157 |

The "supported" attributes are signed integers, `1` or `0`.

Not reported, on purpose: the serial number (a gateway has none to give), the subset level and
conformance (nothing here is certified), and everything the connection document does not say
(owner and operator names, position, configuration identity, time accuracy).

## Time

A master may write the outstation's clock. The driver **accepts every clock write**, whatever
`timeSyncMode` says, and does not use the value: event times come from the tags, not from this
clock. What the write changes is the outstation's own state: it stops asking for the time (the
NEED_TIME indication, which it raises until a master sets it), and from then on it reports its time
stamps as **synchronised**. Without a write the stamps are reported as unsynchronised. The
outstation does not ask again later.

Refusing the write would not be harmless: the master would repeat it on every connection and see the
refusal each time.

## Statistics

Every 5 seconds the driver writes a `stats` sub-document into each connection document:

| Field | Meaning |
| --- | --- |
| `isConnected` | A master is connected (the link is up). |
| `numBytesRx`, `numBytesTx` | Bytes received and sent on the channel. |
| `numOpen`, `numClose` | Times the channel was opened and closed. |
| `numOpenFail` | Failed attempts to open the channel (dial-out modes and serial); a deliberate shutdown is not one. |
| `numLinkFrameRx` | Link frames decoded without error on the endpoint. |
| `numLinkFrameTx` | Link frames routed to this outstation. |
| `numHeaderCrcError`, `numBodyCrcError` | Link frames rejected for a bad header or body checksum. |
| `confirmTimeouts` | Times a master did not confirm an event response in time. |
| `eventsQueued` | Events waiting in the buffer, not yet confirmed by a master. |

The frame and CRC counters come from the decoder shared by every connection on the same endpoint,
so they are per endpoint.

`isConnected` follows the transport, not traffic: a master that polls once every five minutes is
still connected between polls.

## Logging and troubleshooting

Log levels: `0` start-up banner and fatal errors only, `1` basic (connections, destinations
assigned, commands queued, protocol-stack warnings), `2` detailed (every value applied, master
actions, protocol-stack information), `3` debug (adds the protocol stack's debug trace). Log lines
of a connection start with its `name`.

Typical lines:

```
KAW2-GW - Connection Number: 2001
KAW2-GW - Created TCP PASSIVE channel.
KAW2-GW - Outstation created with 64 binary inputs, 0 double binary inputs, 40 analog inputs, ...
KAW2-GW - Outstation enabled.
Watching for changes on collection: realtimeData...
KAW2-GW - Command queued for tag: KAW2-GW;CMD;3 Value: 1
```

| Symptom | Likely cause |
| --- | --- |
| `protocol driver instance not found in the database` | No instance with that number and `protocolDriver: DNP3_SERVER`. |
| `protocol driver instance is disabled` | The instance has `enabled: false`. |
| `node name not found in the protocol driver instance configuration` | `nodeNames` does not list this node. |
| `no protocol connections found for the protocol driver instance` | No enabled connection documents for the instance. |
| The master connects but sees no points | No tag has a destination on this connection, or the tags are not `origin: supervised`. Check the log line "Outstation created with ...". |
| The master reads points as offline or zero | The tag is `invalid`, or the point is in a gap between indexes. |
| The master gets answers for a different device | Two outstations share an endpoint with the wrong `localLinkAddress`. |
| The master polls but never gets answers | `remoteLinkAddress` is not the master's link address, or the master addresses a different `localLinkAddress`. |
| The master reads integers where floats were expected | `protocolDestinationASDU` selects an integer variation (`1` to `4`). Use `5` or `6`. |
| Controls answered `NOT_SUPPORTED` | `commandsEnabled` is `false`, or no command tag has a destination for that group and index. |
| Controls answered `BLOCKED` | The command tag has `enabled: false`. |
| No events, only static values | The value did not change, or the master is not polling the event classes (or unsolicited reporting is off). |
| `isConnected` true on UDP with no master | UDP is connectionless; the link is up as soon as the socket is. |

A DNP3 browser that shows fewer points than the outstation declares may be losing updates itself
(a slow consumer in the viewer); check the viewer's dropped counter before suspecting the
outstation. Reading one family on its own with a range read gets around it.

## Behaviours worth knowing

* **The database is sized and configured once, at startup.** A destination added later takes effect
  only after the driver restarts: an index beyond the size of its family is ignored, and one inside
  it is sent with the family's default variation rather than the one its ASDU selects. A change to
  the value of a tag that already has a point is applied at once.
* **Octet strings, group 50 and frozen analog inputs are not served.**
* **Command events (groups 13 and 43) are not generated.** The outstation does not report
  operated controls back as events.
* **Secure authentication is not implemented.** Use TLS.
* **One master connection at a time** is served on a passive TCP or TLS endpoint.
* **Clock writes are accepted and not applied**; the event times always come from the tags.

## Testing

```
go test ./...
go test -race ./...
go vet ./... && gofmt -l .
```

The tests run a real master against the real outstation over an in-memory link, and over real TCP,
TLS, UDP and (when a port pair is supplied) serial sockets. They need no database and no hardware.
They cover the point mapping and variations, events and their times, the attributes read through a
real master, controls and their values on the wire, multi-drop, address assignment, and a master
polling a 12,000-point outstation.
