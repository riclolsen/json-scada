# Modbus Client and Server Drivers (Go)

Go drivers for Modbus in {json:scada}, built on
[go-modbus](https://github.com/otfabric/go-modbus) (MIT). They replace the Node.js drivers in
`src/modbus` and keep their configuration: the same protocol driver names, the same
`protocolConnections` and `realtimeData` fields and defaults, the same command-line arguments
and environment variables, and the same service names.

| Driver | Protocol driver | Binary | Service (Supervisor / Windows) | Reference |
|---|---|---|---|---|
| Client (master) | `MODBUS` | `bin/modbus-client` | `modbusclient` / `JSON_SCADA_modbusclient` | [cmd/modbusclient/README.md](cmd/modbusclient/README.md) |
| Server (slave) | `MODBUS_SERVER` | `bin/modbus-server` | `modbusserver` / `JSON_SCADA_modbusserver` | [cmd/modbusserver/README.md](cmd/modbusserver/README.md) |

The driver READMEs are the operator references: configuration, tags, behaviour, statistics
and troubleshooting. This page covers what both share, how they are built, and how they
differ from the Node.js drivers.

## Transports

| Client mode | Server mode | Framing | Default port |
|---|---|---|---|
| `TCP Active` | `TCP Passive` | MBAP over TCP | 502 |
| `TLS Active` | `TLS Passive` | MBAP over TLS (Modbus/TCP Security) | 802 |
| `Serial` | `Serial` | RTU over RS-232/RS-485 | none |
| `RTU over TCP` | `RTU over TCP Passive` | RTU frames over TCP | 502 |
| `RTU over TLS` | `RTU over TLS Passive` | RTU frames over TLS | 802 |

Serial ports use 8 data bits, parity none, even or odd, and one or two stop bits. Flow control
and mark/space parity are not available.

## Value types

| ASDU | Registers | Aliases |
|---|---|---|
| `bool` | coil or input bit | `boolean`, `bit`, `coil`, `digital` |
| `int16`, `uint16` | 1 | `short`, `int`; `word`, `uint` |
| `int32`, `uint32` | 2 | `dint`; `udint`, `dword` |
| `int64`, `uint64` | 4 | `lint`; `ulint`, `lword` |
| `float32`, `float64` | 2, 4 | `float`, `real`; `double`, `lreal` |
| `bcd16`, `bcd32` | 1, 2 | |
| `bitstring16` | 1 | `bits` |
| `string:<bytes>` | bytes / 2, rounded up | |

An empty ASDU is `bool` on coils and discrete inputs and `uint16` on registers. A float NaN or
infinity and a BCD digit above 9 are read as invalid. Strings end at the first NUL and lose
trailing blanks. 64-bit integers outside the range a double holds exactly lose precision, as
every value stored in MongoDB as a double does.

## Byte orders

On the wire, registers are 16-bit big-endian words in ascending address order. A byte order
says how the value's bytes are laid out across them. Add it to the ASDU after an underscore:
`float32_cdab`, `int32_le`, `uint64_ghefcdab`, `string:16_ba`.

| Name | 16-bit | 32-bit | 64-bit | Meaning |
|---|---|---|---|---|
| `BE` (default) | `AB` | `ABCD` | `ABCDEFGH` | Modbus standard, big-endian |
| `LE` | `BA` | `DCBA` | `HGFEDCBA` | Little-endian |
| `SW` | `AB` | `CDAB` | `GHEFCDAB` | Word swap |
| `SB` | `BA` | `BADC` | `BADCFEHG` | Byte swap |

Any permutation of the value's first N letters (N = width in bytes) is also accepted, so every
ordering can be described. `BIG_ENDIAN`, `LITTLE_ENDIAN` and `REV_ENDIAN` are accepted for
`BE`, `LE` and `LE`, for tags migrated from the PLC4X driver. For a string, the order is `AB`
or `BA` within each register.

When the ASDU names no order, the connection's `byteOrder16`, `byteOrder32`, `byteOrder64` or
`byteOrderStr` applies.

## Addressing

Object addresses (`protocolSourceObjectAddress` on the client, `protocolDestinationObjectAddress`
on the server) are written `<area>:<offset>[.b<bit>]`:

| Area | Aliases | Functions |
|---|---|---|
| `co` coils | `coil`, `coils` | FC1, FC5, FC15 |
| `di` discrete inputs | `discrete-input`, `discrete-inputs`, `discreteinput` | FC2 |
| `hr` holding registers | `holding`, `holding-register`, `holding-registers` | FC3, FC6, FC16, FC22 |
| `ir` input registers | `input`, `input-register`, `input-registers` | FC4 |

The offset is the 0-based address in the request, 0 to 65535. `.b0` to `.b15` selects one bit
of a holding or input register.

The classic 1-based Modicon numbers map as `00001` = `co:0`, `10005` = `di:4`, `30011` = `ir:10`
and `40001` = `hr:0`. Set `useModiconAddresses: true` on the connection to enter them as they
are.

The common address (`protocolSourceCommonAddress`, `protocolDestinationCommonAddress`) is the
Modbus unit id, 0 to 255. Unit 0 is broadcast. One connection can address many unit ids, as on a
multi-drop line or behind a gateway.

## Logging

The log level is the second command-line argument, or `JS_MODBUS_LOGLEVEL` /
`JS_MODBUSSRV_LOGLEVEL`.

| Level | What is logged |
|---|---|
| 0 | Startup, shutdown and redundancy changes |
| 1 | Connection state, command results, request errors (exceptions sent, refused writes, failed command inserts), configuration problems |
| 2 | Failed block reads, masters connecting and disconnecting, skipped tags, map summaries |
| 3 | Every request and response with decoded function and hex, poll cycle timing, each decoded value, retries, the server's point map |

The library's own messages are prefixed `go-modbus:`.

## Building

Go 1.26 or later. From this directory:

```bash
go build -ldflags="-s -w" -o ../../bin/modbus-client ./cmd/modbusclient
go build -ldflags="-s -w" -o ../../bin/modbus-server ./cmd/modbusserver
```

`build.bat` does the same on Windows. The platform build scripts and the Docker image build
both binaries.

The module uses `src/go-common` through a `replace` directive and is listed in `src/go.work`.

## How it works

go-modbus provides the protocol stacks. A few things the drivers need are not available from
it, and the drivers add them around it:

- **Client TLS and local bind.** The library's TLS client always sends a client certificate and
  does not offer cipher, version or pinning settings, and it cannot bind a local address. In the
  TLS modes, and when `ipAddressLocalBind` is set, the driver opens the socket itself with the
  connection's settings and gives the library a private loopback address that forwards to it.
  `RTU over TLS` works the same way, which the library does not offer at all.
- **Server front-end.** The library's server listens on a private port on `127.0.0.1`. Masters
  connect to a front-end that applies `ipAddresses`, `maxClientConnections` and
  `clientIdleTimeoutMs`, and terminates TLS with the connection's own settings (the library's
  server always requires client certificates). Each master's address is passed to the handler,
  so logs and relayed commands name the real master.
- **Server RTU.** In `Serial`, `RTU over TCP Passive` and `RTU over TLS Passive`, a gateway cuts
  RTU request frames (length prediction with CRC check, and the inter-frame gap as fallback),
  has them served by the library and sends the RTU response, or nothing for a broadcast.
- **Commands ahead of polling.** A command waits only for the request in progress, not for the
  poll cycle.

The code:

| Package | Contents |
|---|---|
| `cmd/modbusclient`, `cmd/modbusserver` | Entry points |
| `internal/mbcore` | Value codec and byte orders, addresses and ASDUs, CRC and RTU framing, request descriptions. No MongoDB. |
| `internal/mbnet` | TLS settings, the client bridge, the server front-end and RTU gateway, serial ports |
| `internal/drv` | Command line, config file, instance document, logging adapters, BSON conversions |
| `internal/clientapp` | The `MODBUS` driver |
| `internal/serverapp` | The `MODBUS_SERVER` driver |
| `internal/loopback` | Client against server tests |

## Differences from the Node.js drivers

Configuration and behaviour match the Node.js drivers except for the following.

1. **Commands and redundancy.** The client executes commands only on the active node and while
   the connection runs; a command for a link that is down is acknowledged `connection down`.
2. **String byte order.** `byteOrderStr` and a `_ba` suffix on a string ASDU now swap the bytes
   of each register; the Node.js driver read the setting and did not apply it.
3. **Broadcast.** Unit 0 writes work for coils and register bits and are acknowledged
   `OK (broadcast, no confirmation)` after 100 ms. A register bit write to unit 0 always uses
   FC22, because the register cannot be read back.
4. **Certificate pinning.** `allowOnlySpecificCertificates` accepts only the certificates in
   `peerCertFilePath`; the Node.js driver only skipped the host name check. It also applies on
   the server, to client certificates.
5. **Log format.** Timestamps follow the format of the other Go drivers. At level 3 the client
   traces request and response PDUs; the MBAP header and RTU CRC are not shown.
6. **`interFrameDelayMs` on the client** is not used: the library derives the RTU timing from
   `baudRate`. The server still uses it to cut request frames.
7. **Serial options.** Mark and space parity become none, and flow control is ignored; both are
   logged at level 1.
8. **Invalidation.** On link loss the client marks the connection's tags invalid once per outage,
   not on every failed attempt.
9. **Link loss detection.** The library reconnects by itself, so the client counts the link as
   lost when a whole poll cycle gets no answer and at least one transport error. A cycle stops at
   the first transport error when nothing has answered yet, so failover to the next address is
   not delayed by the remaining blocks.
10. **Command routing fields** relayed by the server (`protocolSourceObjectAddress`,
    `protocolSourceASDU`) are copied as stored on the tag, numbers included; the Node.js driver
    turned the address into a string.
11. **Point order.** The server maps tags in `_id` order, so overlapping claims resolve the same
    way on every load, and logs each overlap.
12. **FC22 on the server** computes the bit as the standard defines it,
    `(current AND andMask) OR (orMask AND NOT andMask)`. The Node.js driver set the bit when either
    mask had it set, so a mask write that kept the bit unchanged sent an ON command.
13. **Serial server recovery.** A serial port that fails is reopened every 5 seconds.
14. **Instance document.** The drivers create the `protocolDriverInstances` document when it is
    missing, exit when the instance is disabled, and check that every 30 seconds.
15. **Request statistics.** The server's `exceptions` also counts exceptions the library sends on
    its own (an unsupported function code, a quantity out of range), which are logged at level 1.

The Node.js server already took part in redundancy, and so does this one, unlike the other Go
server drivers.

## Migrating from the Node.js drivers

No configuration changes are needed. To switch a node:

1. Build or install `bin/modbus-client` and `bin/modbus-server`.
2. Point the services at them, keeping the service names:
   - Supervisor (`/etc/supervisor/conf.d/modbusclient.ini`, `modbusserver.ini`):
     `command=/home/jsonscada/json-scada/bin/modbus-client 1 1 /home/jsonscada/json-scada/conf/json-scada.json`
   - Windows (an existing installation; `platform-windows/create_services.bat` already defines
     the services this way):

     ```bat
     nssm set JSON_SCADA_modbusclient Application C:\json-scada\bin\modbus-client.exe
     nssm set JSON_SCADA_modbusclient AppParameters 1 1 c:\json-scada\conf\json-scada.json
     nssm set JSON_SCADA_modbusclient AppDirectory C:\json-scada\bin
     ```

     and the same for `JSON_SCADA_modbusserver` with `modbus-server.exe`.
   - The AdminUI process manager runs the binaries for `MODBUS` and `MODBUS_SERVER`.
3. Restart the services.

Things to check after switching:

- String tags with `byteOrderStr: "BA"` or a `_ba` ASDU now decode as configured (difference 2).
- `allowOnlySpecificCertificates` now rejects certificates that are not in `peerCertFilePath`
  (difference 4).
- A server mapping that relied on FC22 masks that keep the bit (difference 12).

The Node.js drivers stay in `src/modbus` and can be put back by pointing the services at
`node /home/jsonscada/json-scada/src/modbus/dist/client/main.js` (or `server/main.js`) again.

## Testing

```bash
go vet ./...
gofmt -l .
go test ./...
go test -race ./...
```

The tests need no MongoDB and no hardware:

- `internal/mbcore`: the value codec under every byte order, addresses and ASDUs, CRC and RTU
  framing, the JavaScript number formatting, ported from the Node.js unit tests.
- `internal/clientapp`: configuration defaults, read planning, topic parsing, tag documents.
- `internal/serverapp`: configuration defaults, the point map, command value routing (ported from
  the Node.js tests), the write relay.
- `internal/loopback`: the client polling the server and sending commands to it over TCP,
  RTU over TCP, TLS and RTU over TLS, with certificates generated for the test.
- `TestReadmeDocumentsEveryConfigKey` (client and server) fails when a configuration key the
  code reads is missing from the driver README.

Interoperability with other Modbus tools, for a manual check:

```bash
# A slave simulator for the client (diagslave), on port 5020
diagslave -m tcp -p 5020
# A master against the server (modpoll): read 10 holding registers of unit 1
modpoll -m tcp -p 5020 -r 1 -c 10 -t 4 127.0.0.1
```

`diagslave -m enron` serves RTU over TCP. For serial, a virtual port pair works:
`socat -d -d pty,raw,echo=0 pty,raw,echo=0` on Linux, com0com on Windows.
