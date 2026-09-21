# IEC 60870-5-101/103/104 Client and Server Drivers (Go)

Go reimplementation of the JSON-SCADA IEC 60870-5 protocol drivers, built
on the [go-iecp5](https://github.com/riclolsen/go-iecp5) library. The 101/104
drivers are **drop-in replacements** for the legacy C# drivers in
[`../lib60870.netcore`](../lib60870.netcore): same protocol driver names, same
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
  jscfg/             json-scada.json + CLI args + log formatting
  mongoutil/         Mongo connect (TLS via URI), permissive numeric decode, instance/conn loading
  model/             protocolConnections / realtimeData / instances document structs
  redundancy/        active-node arbitration (clients), connection-stats updates
  conv/              ASDU encode (BuildInfoObj) / decode / batched send — the protocol core
  cliapp/            client engine: data queue -> Mongo bulk writer, command change stream, autoCreateTags
  srvapp/            server engine: realtimeData change stream, spontaneous batcher, interrogation,
                     command validation/forwarding, DistributeAutoTags
  cs101util/         serial + link config mapping for the 101 drivers
  tlsutil/           *tls.Config from the connection certificate fields (PFX or PEM)
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
  or a PEM file containing the certificate and key.
- **101/103 serial** (`portName`, `baudRate`, `parity`, `stopBits`) maps to the
  go-iecp5 serial config. Serial `handshake` other than `none` is not applied.
  `timeoutForACK`/`timeoutRepeat` (ms) are converted to whole-second T1/T2.
- **101/103 TCP transport**: when `portName` is `host:port`, the FT1.2 frames
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

### Known limitations vs. the C# drivers

- 104 client connection statistics currently report `isConnected` only; the
  detailed APCI counters are not yet exposed by go-iecp5 (gap G4).
- `serverModeMultiActive=false` (single-redundancy-group buffering) is emulated
  with a bounded FIFO held while no master is connected (gap G3).

## Behavior parity notes

Replicated intentionally from the C# drivers:

- CP24-tagged values always write `timeTagAtSourceOk=false` (the C# driver tests
  an unused CP56 placeholder). CP56-tagged values set it from the time validity.
- Step-position `transient` is merged into invalid quality on decode.
- The 104/101 servers use a single process-wide "selected point" slot for SBO.
- Client commands read from `commandsQueue` are sent without kconv conversion.
- Command-expiry windows: 10 s on the client side, 20 s for time-tagged commands
  on the server side.
- Counter re-initialization offsets on (re)connect (`giInterval-3`/`-2`, etc.).

## Testing

```sh
go test ./internal/conv/   # encode/decode parity unit tests
go test ./test/            # cs104 client <-> server protocol loopback (no MongoDB)
```

The loopback test wires a go-iecp5 server and client over `127.0.0.1` and checks
interrogation data and a control command round-trip through the `conv` package.
Full end-to-end verification against a running JSON-SCADA MongoDB, and interop
against the legacy C# drivers, is described in `GO_DRIVERS_PLAN.md`.
