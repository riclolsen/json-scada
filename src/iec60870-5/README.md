# IEC 60870-5-101/103/104 Client and Server Drivers (Go)

Go reimplementation of the JSON-SCADA IEC 60870-5 protocol drivers, built
on the [go-iecp5](https://github.com/riclolsen/go-iecp5) library. The 101/104
drivers are **drop-in replacements** for the legacy C# drivers (lib60870.netcore,
no longer in this repository): same protocol driver names, same
MongoDB collections and field semantics, same command-line contract and same
binary names, so AdminUI, the service definitions and the demo configurations
work unchanged. The 103 client is a new driver (no C# predecessor).

| Binary | Protocol driver name | Role | Transport |
|---|---|---|---|
| `iec104client` | `IEC60870-5-104` | Master (client) | TCP/IP (+TLS) |
| `iec104server` | `IEC60870-5-104_SERVER` | Outstation (server) | TCP/IP (+TLS) |
| `iec101client` | `IEC60870-5-101` | Primary station (master) | Serial or TCP (unbalanced) |
| `iec101server` | `IEC60870-5-101_SERVER` | Secondary station (outstation) | Serial or TCP (unbalanced) |
| `iec103client` | `IEC60870-5-103` | Master (protection relays) | Serial or TCP (unbalanced) |

## IEC 60870-5-103 client

IEC 60870-5-103 is the informative interface of protection equipment. The
`iec103client` master runs the FT1.2 unbalanced link procedure (shared with
101) and the 103 application layer. Points are addressed by **function type
(FUN)** and **information number (INF)** rather than an IOA; the driver maps
them into the JSON-SCADA `realtimeData` model as:

- `protocolSourceCommonAddress` = device common address (= link address).
- `protocolSourceObjectAddress` = `FUN*65536 + INF*256 + index`, where
  `index` selects a value inside a multi-valued measurands ASDU (0 otherwise).
- `protocolSourceASDU` = the 103 type identification number.

Time-tagged ASDUs (1/2) become digital points (DPI On/Off; transient/unknown →
invalid quality); measurands ASDUs (3/9) become analog points, one per index,
scaled to the fraction of full scale (`Measurand.Float64()`). General
interrogation and time synchronization are issued periodically per
`giInterval` / `timeSyncInterval`. Control is via the 103 **general command**
(ASDU 20): a `commandsQueue` entry with `protocolSourceASDU = 20` and
`protocolSourceObjectAddress = FUN*65536 + INF*256` sends DCO On/Off from the
command value; the device's ASDU 1 acknowledgement (cause 20/21) is written
back as the command `ack`.

## Command line

```
iec10Xclient|server [instanceNumber [logLevel [configFileName]]]
```

- `instanceNumber` — driver instance number (default `1`).
- `logLevel` — `0` none, `1` basic (default), `2` detailed, `3` debug. Level 3
  additionally enables the go-iecp5 protocol logger.
- `configFileName` — path to `json-scada.json`; defaults to
  `../conf/json-scada.json`, then `c:/json-scada/conf/json-scada.json`. The
  `JS_CONFIG_FILE` environment variable is also honored.

## Source Code Layout

```
cmd/                 one main package per binary (thin: wire config -> shared engines -> endpoint)
internal/
  jscfg/             strict command-line parsing (config, logging and Mongo helpers are in ../go-common)
  mongoutil/         driver instance / connection loading
  model/             protocolConnections / realtimeData / instances document structs
  conv/              ASDU encode (BuildInfoObj) / decode / batched send — the protocol core
  conv103/           IEC 103 ASDU decode to realtimeData values (FUN/INF addressing)
  cliapp/            client engine: data queue -> event-driven Mongo bulk writer, command change stream,
                     autoCreateTags
  srvapp/            server engine: realtimeData change stream, spontaneous batcher, interrogation,
                     command validation/forwarding, DistributeAutoTags
  cs101util/         serial + link config mapping for the 101 drivers
  tlsutil/           *tls.Config from the connection certificate fields (PFX or PEM)
  iecsec/            security events (secEvents) and peer identification (stats.peerIdentity):
                     driver event kinds, filtering and mapping of go-iecp5's secevent channel
  testmongo/         test support: throwaway MongoDB (bundled mongod) for the Mongo-backed tests
test/                protocol-level loopback tests (no MongoDB required)
```

## Building

Handled by the platform build scripts (`platform-windows/build.bat`,
`platform-linux/build.sh`, and the RHEL/Ubuntu installers). Manually:

```sh
cd src/iec60870-5
go build -o ../../bin/iec104client ./cmd/iec104client
go build -o ../../bin/iec104server ./cmd/iec104server
go build -o ../../bin/iec101client ./cmd/iec101client
go build -o ../../bin/iec101server ./cmd/iec101server
go build -o ../../bin/iec103client ./cmd/iec103client
```

## Configuration fields

All `protocolConnections` fields honored by the C# drivers are honored here.
Notable mappings and current limitations:

- **Address fields accept strings or numbers**: `protocolSourceASDU`,
  `protocolSourceCommonAddress`, `protocolSourceObjectAddress` and the
  `protocolDestinations` entries `protocolDestinationASDU`,
  `protocolDestinationCommonAddress`, `protocolDestinationObjectAddress` may be
  stored as a BSON string (`"1001"`) or as any numeric type (double, int32,
  int64, decimal128); all are converted to `uint32`. Unparseable, negative or
  missing values become `0`, values above `2^32-1` are clamped. Queries match a
  field stored either as the number or as its decimal string, so mixed
  representations across documents work.

- **104 client dual-server failover** (`ipAddresses[0]` / `[1]`) is managed by
  the driver: on connection loss it swaps to the other address and reconnects.
- **104 server client whitelist** (`ipAddresses`): empty or `["*"]` accepts any
  client; otherwise the connecting IP must be listed. `maxClientConnections` is
  enforced by the driver.
- **TLS** (`localCertFilePath` + `passphrase`, `peerCertFilesPaths`,
  `rootCertFilePath`, `chainValidation`, `allowOnlySpecificCertificates`) is
  supported; the certificate file may be a `.pfx`/`.p12` (as in the C# drivers)
  or a PEM file containing the certificate and key. With `chainValidation` the
  peer chain is verified against `rootCertFilePath`, or against the system
  roots when no root file is set; a server checks the client certificate for
  the clientAuth usage, a client the server certificate for serverAuth.
- **101/103 serial** (`portName`, `baudRate`, `parity`, `stopBits`) maps to the
  go-iecp5 serial config. Serial `handshake` other than `none` is not applied.
  `timeoutForACK`/`timeoutRepeat` (ms) are converted to whole-second T1/T2.
- **101/103 TCP transport**: when `portName` is `host:port` with a numeric
  port (names starting with `/` or `\\` are always serial devices), the FT1.2 frames
  are carried over a TCP client connection to a terminal / serial-device server
  (this replaces the C# `TcpClientVirtualSerialPort`). TLS is used on that TCP
  transport when `localCertFilePath` is configured.
- **`autoCreateTags`**: on the **clients** it auto-creates a supervised
  `realtimeData` tag on the first value for a new address; on the **servers** it
  distributes `protocolDestinations` onto existing tags (the server link-level
  address `localLinkAddress` is used as the destination Common Address). With an
  empty or absent `topics` array the complete database is distributed: every
  digital and analog tag whatever its origin (supervised, calculated, manual,
  ...); a non-empty `topics` restricts it to tags whose `group1` contains one of
  the substrings. See the per-driver READMEs under `cmd/*/README.md` for the
  full behavior and IOA ranges.
- **`peerIdentification`** (boolean, default `true`): record the peer's
  identity in `stats.peerIdentity` and report its changes as security events.
  See [Security events and peer identification](#security-events-and-peer-identification).

## Security events and peer identification

Every driver reports unexpected or suspicious conditions to the `secEvents`
collection (see `docs/schema.md`, listed by the Security Events viewer of the
Admin UI), from two sources:

- `source: "library"`: go-iecp5's `secevent` channel (v0.5.0). The library
  checks every frame and ASDU: unexpected connections, TLS handshakes and
  certificates, sequence numbers (I-frame N(S)/N(R) duplicates, gaps and
  regressions, the 101/103 FCB repeat), malformed APCI/ASDU/FT1.2 frames and
  fields, wrong direction, unknown causes, foreign link addresses, select
  without execute, unsolicited confirmations, control actions (reset process,
  parameter change and activation, clock step, end of initialisation, 103
  setting change), file transfers, command brute force and floods. Routine
  operation (connection opened or closed, STARTDT/STOPDT, interrogation,
  clock sync, test command, link reset, each received command) is not stored.
- `source: "driver"`: the drivers' own checks, which know the tags and the
  configuration (see the per-role tables below).

A server refusing a command, and a 104 client seeing a device refuse one, are
reported by the driver (with the tag and the reason) instead of the library.
With `peerIdentification` off, the identity events (`peer.identified`,
`peer.identity_changed`, `peer.mac_changed*`, `peer.mac_shared`,
`tls.cert_changed`) are left out.

### Peer identification

IEC 60870-5-101/104 define no device identification, so a peer is identified
by what the connection shows of it: its address, the MAC address of its IP
from the operating system's neighbour (ARP) table (a peer behind a router
shows the router's MAC, and is left out), its TLS certificate and, on 101,
the link address. IEC 60870-5-103 devices identify themselves: the
identification ASDU 5 (sent after the reset of the communication unit, and on
a restart) carries the compatibility level, 8 ASCII characters (manufacturer
and product) and 4 octets of software identification.

| Driver | `stats.peerIdentity` |
|---|---|
| `iec104client` | `address`, `ip`, `mac`, `tls`, `connectedAt` of the current server, and `keys` |
| `iec101client` | `address` (the TCP peer, or `serial://<port>`), `linkAddress`, `ip`, `mac`, `tls`, `connectedAt`, `keys` |
| `iec103client` | as the 101 client, plus `devices`: one entry per device that sent its identification (`commonAddress`, `identification`, `compatibilityLevel`, `softwareId` in hex, `fun`, `inf`, `cause`) |
| `iec104server` | `clients`, the identity of each master connected now, and `known`, the last identity of every master IP seen (the 64 most recent: `ip`, `lastSeen`, `mac`, `tlsSha256`, `tlsSubject`) |
| `iec101server` | not recorded (go-iecp5's 101 server exposes no peer connection) |

`keys` (`k`, `v`) are the values compared between connections: `mac`, `cert`
(the certificate's SHA-256) and, on 103, `ident:<commonAddress>`. They are read
back from `stats` at start, so a change across a driver restart is reported
too. The identity is collected each time the connection or link is
(re)established; a 103 device that did not send its identification on a
connection keeps its previous key.

```json
{
  "address": "192.168.0.20:2404",
  "ip": "192.168.0.20",
  "mac": "00:1a:2b:3c:4d:5e",
  "connectedAt": "2026-11-22T12:00:00Z",
  "keys": [{ "k": "mac", "v": "00:1a:2b:3c:4d:5e" }]
}
```

### Client events (`iec104client`, `iec101client`, `iec103client`)

| Kind | When |
|---|---|
| `peer.identified` | First identification of the device. |
| `peer.mac_changed`, `tls.cert_changed` | The MAC address or the certificate of the device changed (on 104 detected by go-iecp5, which also reports `peer.mac_changed_midsession` and `peer.mac_shared`). |
| `peer.identity_changed` | A 103 device's identification (compatibility level, ASCII, software id) changed. |
| `tls.cert_rejected` | The device's certificate was refused (subject, issuer, fingerprint). |
| `ctrl.end_of_init` | The device restarted: 101 end of initialisation (the cause in `value`), 103 identification with cause start/restart or power on (104: from go-iecp5). |
| `cmd.rejected` | A command not sent (commands disabled, a type the driver does not send), or refused by the device: negative confirmation, unknown type or cause (101/104), negative acknowledgement (103). |
| `cmd.object_unknown` | The device answered a command with unknown IOA or common address. |
| `cmd.stale_time` | A queued command older than its 10 s expiry, not sent (delayed, or replayed into `commandsQueue`). |
| `flood.commands` | More than 10 commands to the device within 10 seconds. |
| `cmd.unnecessary` | An ON/OFF command to the state its supervised point already was in (a value updated before the command was sent). Checked in the background; the command is still sent. |

### Server events (`iec104server`, `iec101server`)

| Kind | When |
|---|---|
| `conn.unexpected` | A 104 master not in `ipAddresses` (refused). |
| `conn.rejected_limit` | `maxClientConnections` reached. |
| `tls.cert_rejected`, `tls.handshake_failed`, `tls.no_client_cert` | A client certificate refused, or another handshake failure (from go-iecp5). |
| `peer.identified` | First connection from a master IP (104). |
| `access.unknown_ioa` | A read (C_RD_NA_1) of an object without a mapped point. |
| `access.unknown_ca` | A read, command or interrogation to a common address without mapped points. |
| `access.scan` | More than 20 distinct nonexistent objects requested by one master within a minute. |
| `access.unknown_type` | A request of a type the server does not serve. |
| `file.upload` | A file transfer request (the servers serve no files). |
| `cmd.object_unknown` | A command to an object without a mapped point. |
| `cmd.rejected` | A command refused: commands disabled for the connection, the test bit set, an invalid value, an unexpected select, a qualifier mismatch, a database error. |
| `cmd.execute_without_select` | An execute without a valid select on a select-before-operate point. |
| `cmd.stale_timetag` | A time-tagged command outside the 20 s window (a replayed command, or clocks out of step). |
| `cmd.unnecessary` | An ON/OFF (C_SC/C_DC) command to the state the point already was in. Checked in the background; the command is still forwarded. |
| `flood.commands`, `cmd.brute_force` | Command floods and repeated refused commands (from go-iecp5). |

A 101 server also gets go-iecp5's link layer events (`link.foreign_address`,
`link.unexpected_address`, `seq.fcb_repeat`, `proto.cs101_*`).

### Known limitations vs. the C# drivers

- 104 client connection statistics currently report `isConnected` only; the
  detailed APCI counters are not yet exposed by go-iecp5 (gap G4).
- `serverModeMultiActive=false` (single-redundancy-group buffering) is emulated
  with the per-connection spontaneous FIFO held while no master is connected
  (gap G3). The FIFO holds `maxQueueSize` objects (5000 when unset); the oldest
  are dropped when it is full, and the drops are logged.

## Behavior parity notes

Replicated intentionally from the C# drivers:

- CP24-tagged values always write `timeTagAtSourceOk=false` (the C# driver tests
  an unused CP56 placeholder). CP56-tagged values set it from the time validity.
- Step-position `transient` is merged into invalid quality on decode.
- Client commands read from `commandsQueue` are sent without kconv conversion.
- Command-expiry windows: 10 s on the client side, 20 s for time-tagged commands
  on the server side (on the server, a command time more than 20 s in the
  future is rejected too).
- Counter re-initialization offsets on (re)connect (`giInterval-3`/`-2`, etc.).

Deliberately different from the C# drivers:

- Every connection publishes `stats.isConnected` for the data processor's
  communication status tags: the 101/103 clients with the link-layer state
  (next to `linkLayerState`), the 104 client with the session state, the
  104 server on each connection event. The 101/104 servers also run
  `srvapp.Engine.RunStatusLoop`, which every 5 s advances the instance
  `activeNodeKeepAliveTimeTag` and sets `stats.isConnected` from
  `HasSession` (the C# servers never updated the keep-alive, so the driver
  looked stopped). With the same server instance on several nodes all write.

- The servers send a monitored value the same way whether it goes out as
  spontaneous data, in an interrogation or in a read reply: the destination
  `protocolDestinationKConv1`/`KConv2` apply (kconv1 -1 inverts digitals), and
  the point's `invalid`, `substituted` and `overflow` flags become the quality.
  The C# drivers applied the conversion only in interrogations. Interrogations
  send the untagged type, spontaneous data the CP56 variant when the point has
  a source time.
- Server command handling: the point is found by connection, common address,
  object address and type on the same `protocolDestinations` entry; the
  activation confirmation is sent only after every check passed and the
  command was queued (a rejected command gets one negative confirmation). A
  select is held per connection for the master link that sent it, expires
  after 30 s and is consumed by the execute (the C# drivers used one
  process-wide slot).
- `commandsEnabled: false` on a server refuses every process command and
  parameter with a negative confirmation (not only the automatic mapping of
  command destinations), and so does one with the test bit set (nothing is
  queued). Counter interrogation and reset process are forwarded as before.
- A station interrogation answers the points of its common address only; the
  global address takes every point of the connection. An interrogation of a
  common address no tag is mapped to gets a negative confirmation.
- The 101/104 clients execute the stored select-before-operate twin once, on a
  positive select confirmation that matches its common address, object address
  and type within 30 s of the select; the pending execute is dropped when the
  connection is lost.
- The servers wait for room when a link's send buffer is full instead of
  dropping the ASDU, so a large interrogation or a burst of spontaneous data
  arrives complete. A link that has accepted nothing for 10 s (a 104 standby
  connection without STARTDT) is not waited on. Spontaneous data is sent per
  connection, each 104 master on its own.
- Scaled, normalized and step values are rounded to the nearest integer
  (C#: truncated); bitstrings keep bit 31.
- The 104 client lets a connection attempt run up to `t0` before trying the
  other server.
- The client MongoDB writer (shared by the 101, 103 and 104 clients) blocks on
  its queues instead of polling them every 100 ms, so a decoded value or command
  ack is written immediately (about 1 ms added, against 50 ms on average and up
  to 100 ms before). Values arriving while a bulk write is in flight go into the
  next one, so bursts are still batched (up to 5000 per write). The database is
  pinged only after 5 s without traffic; a failed write also forces a reconnect,
  and the batch is written again after it. A tag that `autoCreateTags` could not
  insert is tried again on the next value for its address.
  The per-batch `MongoDB - Bulk write N` line is logged at detailed level (2),
  since batches are now as small as one value.

## Testing

```sh
go test ./internal/conv/      # encode/decode unit tests
go test ./internal/tlsutil/   # TLS peer verification (generated certificates)
go test ./internal/iecsec/    # security event filtering, mapping, scan detection, peer identity changes
go test ./internal/cs101util/ # serial vs TCP port names
go test ./test/               # cs104 client <-> server protocol loopback (no MongoDB)
go test ./internal/cliapp/    # client writer latency, batching and tag auto-creation (MongoDB)
go test ./internal/srvapp/    # server flow control and large-interrogation loopback; commands,
                              # SBO, interrogation, value conversion, autoCreateTags (MongoDB)
```

The loopback test wires a go-iecp5 server and client over `127.0.0.1` and checks
interrogation data and control command round-trips (untagged and CP56 time-tagged,
with their confirmations decoded as acks) through the `conv` package.
The MongoDB-backed tests start the bundled `mongod` of `platform-windows` or
`platform-linux` on a temporary dbpath and free port (override with
`JS_TEST_MONGOD`, or point them at a running server with `JS_TEST_MONGO_URI`);
they skip when neither is available.
