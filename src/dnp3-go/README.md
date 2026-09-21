# DNP3 Client and Server Protocol Drivers (Go)

Pure-Go implementations of the JSON-SCADA DNP3 master and outstation drivers, built on
[github.com/dscsystems/go-dnp3](https://github.com/dscsystems/go-dnp3).

They are **drop-in alternatives** to the C++ drivers in `src/dnp3`:

| Binary | Replaces | `protocolDriver` |
| --- | --- | --- |
| `dnp3-client` | `Dnp3ClientCpp` | `DNP3` |
| `dnp3-server` | `Dnp3Server` | `DNP3_SERVER` |

Same configuration documents, same collections, same command line, same `stats` sub-document.
An existing installation switches driver by changing which executable the process manager
starts. No schema change is required.

For the configuration reference — driver instances, connections, tag setup, the group and
variation tables, the CROB duration codes — see the existing documentation, which applies
unchanged:

- [`src/dnp3/Dnp3ClientCpp/README.md`](../dnp3/Dnp3ClientCpp/README.md) for the client
- [`src/dnp3/Dnp3Server/README.md`](../dnp3/Dnp3Server/README.md) for the server

This file documents only what is specific to the Go implementation.

## Why

The C++ drivers need opendnp3, mongo-cxx-driver and OpenSSL built from source, through vcpkg and
CMake, which is a one-time build measured in tens of minutes and the reason the DNP3 stack has
been awkward to package. The Go drivers need `go build`, and cross-compile to Windows, Linux and
macOS from any of them.

## Building

```
cd src/dnp3-go
go build -ldflags="-s -w" -o ../../bin/dnp3-client ./cmd/dnp3client
go build -ldflags="-s -w" -o ../../bin/dnp3-server ./cmd/dnp3server
```

On Windows, `build.bat` does the same. The platform build scripts
(`platform-linux/build.sh`, `platform-mac/build.sh`, `platform-windows/build.bat`) build both
binaries automatically.

## Running

```
dnp3-client [instanceNumber] [logLevel] [configFile]
dnp3-server [instanceNumber] [logLevel] [configFile]
```

All three arguments are optional and default to `1`, `1` and `../conf/json-scada.json`. The
client falls back to `~/json-scada/conf/json-scada.json` and the server to
`/json-scada/conf/json-scada.json`, each keeping its own C++ predecessor's fallback. Log levels
are 0 = none, 1 = basic, 2 = detailed, 3 = debug; a level given on the command line takes
precedence over the one in the instance document.

To select the Go drivers from the AdminUI process manager, set the instance's variant to
`dnp3go` or `dnp3servergo` (see `driver-catalog.js`). The C++ binaries remain the default.

## Licence

go-dnp3 is GPL-3.0-or-later and JSON-SCADA is GPL-3.0, so the two are compatible and nothing
special is needed. Anything linking these drivers must be GPL too.

## Deviations from the C++ drivers

Intentional differences. Everything not listed here is meant to behave identically.

| ID | Deviation |
| --- | --- |
| D1 | Shutdown is on `SIGINT`/`SIGTERM` rather than a 500 ms poll loop or a console key. |
| D2 | Master sessions are created when redundancy activates the node and destroyed when it goes to standby, rather than created up front and `Enable`/`Disable`d. Channels and buses persist, so a passive listener keeps its bound port across an activation cycle. |
| D3 | Time sync is scheduled — once per connection and then every 60 s — rather than triggered by the outstation's NEED_TIME indication. go-dnp3 tracks that indication internally but never acts on it, and its type cannot be named from outside the module to test it. Functionally a superset, at the cost of a periodic write the C++ driver does not send. |
| D4 | The frame and CRC statistics come from the shared multi-drop bus and are therefore per bus: every connection of a multi-drop group reports the same `numLinkFrameRx`, `numHeaderCrcError` and `numBodyCrcError`. On the client, `numLinkFrameTx` is the master's task count — requests issued — because nothing counts frames on the way out. |
| D5 | TLS is mutual-auth only with a TLS 1.2 floor, which is what the library provides. `allowTLSv10` and `allowTLSv11` are ignored with a warning; `cipherList` is ignored with a warning; `allowTLSv12: false` with `allowTLSv13: true` raises the floor to TLS 1.3. |
| D6 | Group 50/52 (time and interval) destinations are unsupported and are skipped with one warning per connection. `outstation.DatabaseConfig` has no such family. |
| D7 | Server updates use change detection. The C++ server forces an event on every change-stream update, so a repeated identical value produced a repeated DNP3 event; here an unchanged value produces none. |
| D8 | Point invalidation on disconnect is per connection rather than per shared channel. The effect is the same, since every connection on a shared channel loses its link at the same moment. |
| D9 | Both change streams resume from a stored token after a MongoDB outage, so a command or a value change inserted during it is not lost. The C++ drivers restart from "now". |
| D10 | `resultDescription` may carry the DNP3 command status in parentheses after the legacy prefix, e.g. `FAILURE_BAD_RESPONSE (NOT_SUPPORTED)`. The prefix is byte-identical, so anything matching the old strings still matches. |
| D11 | The server banner no longer says "IEC60870-5-104 Server Driver". |
| D12 | `timeSyncInterval` is accepted and ignored. The library has no periodic NEED_TIME re-assertion, and the C++ server reads the field into its struct and never uses it either. |
| D15 | A tag with no source timestamp is sent as unsynchronised-with-a-time rather than invalid-with-a-time. `dnp3.TimestampInvalid` discards the time entirely, and a measurement with no time at all is worse for a master than one honestly labelled unsynchronised. |
| D16 | Frozen counters are written to the frozen counter family. The C++ server falls through into the counter branch and writes both to the counter array; the Go database keeps them apart, so the point has to go where it was sized. |
| D20 | The server writes a `stats` sub-document, which the C++ server never does. It carries the same fields as the client's plus `confirmTimeouts` and `eventsQueued`. |
| D21 | Two connections sharing an endpoint that cannot be told apart by link address — two masters on one `remoteLinkAddress`, two outstations on one `localLinkAddress` — fail startup with a configuration error. The C++ drivers start and let the two sessions steal each other's frames. |
| D22 | `stopBits: "One5"` is honoured as one-and-a-half stop bits. The C++ drivers fold it onto two, because opendnp3's `SerialSettings` has no other value for it. |
| D23 | A CROB carrying no trip/close code is executed, with the operation type deciding on or off. The C++ server reads the value from the trip/close field alone and rejects anything else as a format error — which means a plain `LATCH_ON` never reaches the field, and since the client driver's auto-created command tags use duration 3 (`LATCH 1=ON 0=OFF`), the two halves of the product could not operate a point through each other on their default settings. |
| D24 | The server answers device attribute reads (group 0) with the driver's identity and the point counts of the connection. opendnp3 has no group 0 support, so the C++ server answers none of it. Read-only, and nothing else in JSON-SCADA is affected. |
| D25 | Auto-created command destinations get a matching output status destination: a CROB at group 12 index N is mirrored by group 10 index N on the command's supervised twin, and an analog output block at group 41 index N by group 40 index N. The C++ server creates the command alone, leaving a master able to operate a point but not read it back. |
| D26 | An active connection uses every entry of `ipAddresses` as an alternative address for the same device, trying them in turn until one answers. The C++ drivers use only the first and their documentation says so. |



D13, D14, D17, D18 and D19 were opened against the C++ server and then **withdrawn**: the defects
they described were fixed in `Dnp3Server` v0.1.1 instead, so both implementations now agree. See
[`DNP3_GO_DRIVERS_PLAN.md`](DNP3_GO_DRIVERS_PLAN.md).

## Auto-created destinations

With `autoCreateTags` set, the connection publishes tags it is not already distributing, in the
order the C++ server uses: CROB commands, analog output commands, then supervised digitals and
analogs.

Each command also gets its **output status**, so a master can read back the state of what it
operated:

| Command | Distributed as | Readback | On which tag |
| --- | --- | --- | --- |
| digital command | group 12 index N, variation 1 | group 10 index N, variation 2 | the command's `supervisedOfCommand` twin |
| analog command | group 41 index N, variation 3 | group 40 index N, variation 3 | the command's `supervisedOfCommand` twin |

The status carries the **same object address as the command**, because that is what the protocol
means: a CROB at index N operates binary output N, and group 10 index N is that output's state.
The address is therefore not this driver's to choose. If something already occupies it, the clash
is logged and no status is created — the command still works, and silently moving a configured
point would be worse than a missing readback.

Two consequences worth knowing:

- **A command with no supervised twin gets no status.** `supervisedOfCommand` is what the schema
  calls the tag "where the command feedback manifests"; a command without one is what the schema
  calls a blind command, and there is nothing whose state could be reported.
- **A twin is published as an output status rather than as an input.** The supervised passes run
  after the command passes and skip tags already distributed on the connection, so a controllable
  point appears once, as group 10 or 40, rather than also as group 1 or 30. A controllable point
  is an output.

Re-running auto-create adds nothing: a command already distributed is not a candidate, and a twin
already carrying its status destination is left alone.

## Device attributes

The server answers a read of group 0, which is how a master asks an outstation what it is. A
commissioning engineer facing several identical-looking gateways reads this instead of trusting a
drawing.

| Variation | Attribute | Value |
| --- | --- | --- |
| 252 | manufacturer name | `{json:scada}` |
| 250 | product name and model | `JSON-SCADA DNP3 Outstation Server (Go)` |
| 242 | software version | the driver version |
| 243 | hardware version | the host platform, e.g. `linux/amd64` — the nearest honest thing to hardware for a software outstation, and what you want when a gateway misbehaves on one machine and not another |
| 247 | device name | the connection's `name`, which is what the tag names and every log line already use |
| 245 | location | the connection's `description`, omitted when it is empty |
| 246 | ID code | the `protocolConnectionNumber`, unique across every driver of an installation |

The library adds the point counts and the fragment sizes from the session it built — number of
binary inputs, analog inputs, counters and so on — so those cannot drift from the database they
describe. A point type the connection does not carry is left unreported rather than reported as
zero: "none" and "I did not say" are different answers.

Two standard attributes are deliberately **not** answered:

- **Subset level and conformance (249).** Nothing here has been through certified conformance
  testing, and go-dnp3's own device profile says the same. Answering it would be a claim, not a
  fact.
- **Serial number (248).** A software gateway has no serial number, and inventing one from a
  connection number invites somebody to key an asset register off it.

A master reading an attribute the outstation does not report learns that it does not report it,
which is true. A plausible wrong value propagates.

Nothing is configurable from MongoDB: no schema change was needed, and every value is either
fixed or already in the connection document.

## Quirks reproduced on purpose

These look like bugs and are kept anyway, because a database configured against the C++ driver
depends on them.

| ID | Quirk |
| --- | --- |
| Q1 | The client files frozen counters under `protocolSourceCommonAddress` **23**, the event group, not the 21 the README's table names. Changing it would orphan every existing frozen counter tag. |
| Q2 | Timestamps outside 2001-09-09 … 2033-05-18 are zeroed before `sourceDataUpdate` is written. It is a guard against a device reporting a wild time; it will also discard legitimate timestamps from 2033. |
| Q3 | CROB durations 10, 12, 20 and 22 appear in the driver README's table but were never implemented by the C++ switch. They produce a block that operates nothing. A guess here would operate the wrong coil of a breaker, so they are left as they are and documented instead. |
| Q4 | `sourceDataUpdate.asduAtSource` always ends in variation 0, and `causeOfTransmissionAtSource` is always 20. |

## Multi-drop

Several `protocolConnections` documents that repeat one endpoint — the same
`ipAddresses`, the same `ipAddressLocalBind`, or the same `portName` — and differ in their link
addresses share one physical channel, exactly as with the C++ drivers.

The sharing is done by the library's `multidrop.Bus`, which routes inbound frames by link
address, serialises transmission, and holds the half-duplex line for one master's exchange at a
time. Two things follow that are worth knowing:

- **The bus does not pace the sessions against each other.** Three masters polling a slow line
  every second will spend their time waiting for each other. The driver logs a warning at startup
  when the configured scan intervals on a shared line imply more exchanges per second than the
  line can carry, but the intervals are still yours to choose.
- **Arbitration is on for a shared line and off for a dedicated socket.** A single connection on
  its own TCP, TLS or UDP endpoint needs no turn taking; a serial line always does, and so does
  any endpoint carrying more than one connection, because that is what a terminal server fronting
  a real serial line looks like.

## Alternative addresses

For an active connection, `ipAddresses` is a list of ways to reach **one** device — a second NIC,
a redundant gateway, a standby route — not a list of devices. The driver tries them in order:

- an attempt that fails moves straight to the next address, with no delay, because the whole
  point of a second address is that failing over should cost one dial rather than a retry
  interval;
- the backoff is paid once the entire list has been tried, not once per address, so three dead
  addresses do not take three backoffs to get through;
- a connection that succeeds stays where it is and only moves when that address stops answering.

Blank entries are ignored; a list with nothing usable in it is a configuration error.

Failing over across three addresses where only the last answers:

```
FOCLI - Connection attempt to tcp-client 127.0.0.1:20560 failed: ... actively refused it.
FOCLI - Connection attempt to tcp-client 127.0.0.1:20561 failed: ... actively refused it.
FOCLI - Channel state: OPEN
```

Both refusals and the successful connection land within a millisecond of each other, and
`numOpenFail` counts each attempt.

This applies to `TCP Active` and `TLS Active`. A passive connection listens on one bind address,
and for it `ipAddresses` remains the list of clients allowed to connect; UDP and serial each have
one endpoint.

## Large outstations

Two things had to change before a client could scan a device with thousands of points and create a
tag for every one of them. Both were silent losses, which is the worst kind: the driver kept
running and the database was simply short.

**The multi-drop bus queues 16 link frames per session by default.** That is sized for a serial
line, where frames arrive a few per second and a full queue really does mean a wedged session. On
a socket, the response to an integrity poll of a large outstation arrives as hundreds of link
frames back to back — a 12000-point database is well over a hundred — and sixteen frames of slack
is consumed almost at once. A dropped link frame is not a dropped measurement: it is a hole in a
transport segment, so the whole application fragment is lost and the poll ends in a response
timeout. With `autoCreateTags` on, every point in that fragment is a tag that never gets created.
The driver now asks for 4096 (`dnp3util.StationQueue`), about 1.2 MB per session in the worst
case and only while a session is behind.

**The value queue dropped its oldest entry once its limit was reached**, which is what the C++
driver does at 10000. On a large device the points that arrive first in every integrity poll are then the
ones discarded in every integrity poll, so the same tags are missing every time. The queue now
bounds itself by coalescing instead:

- a **static** value is a snapshot, so a newer one replaces the older one for the same point;
- an **event** is a discrete occurrence — a point that went on, off and on again produced three of
  them — so events are kept in sequence;
- past the threshold — `DataBufferLimit`, 50000 values — events coalesce too, and the writer
  reports how many were folded. Losing the intermediate values of a chattering point is a fair
  degradation; losing the point is not.

Because the threshold no longer decides what is discarded, only when event sequences stop being
kept whole, raising it buys sequence-of-events fidelity through a longer burst and costs memory
only while the writer is behind: roughly 150 bytes per waiting value, about 8 MB at this figure.

The queue is then bounded by the number of distinct points rather than by the arrival rate, and no
point is ever lost.

### The index matters

`realtimeData` must carry the standard index on
`(protocolSourceConnectionNumber, protocolSourceCommonAddress, protocolSourceObjectAddress)`,
which `mongo_seed/b_create-db.js` creates. It is what the driver's `sourceDataUpdate` filter is
served by. Without it every update in a batch is a collection scan, a large batch cannot finish
inside its timeout, and updates stop while tag creation carries on — 12000 tags created and 2608
of them ever updated, in the run that found this. The driver now says so when a large bulk write
times out rather than leaving it to be discovered.

Measured, 12000 points on a freshly seeded database: 12000 tags created and all 12000 updated
within 15 seconds of the first poll, with no dropped frames, no timeouts and no coalescing.

## A browsing tool showing a family as missing

If a DNP3 browser reports fewer points than the outstation declares — or a whole family, commonly
the binary output statuses, as absent — check its **dropped** counter before suspecting the
server.

`master.ChannelHandler`, which browsing tools use to get updates onto a screen, does a
non-blocking send and **discards updates when the consumer falls behind**, counting them in
`Dropped()`. That is the right trade for the protocol — a stalled display must not stall the
session — and it is lossy for the display.

An integrity poll of a large database sends static data grouped by object group, so a contiguous
run of one family arrives together. A consumer that falls behind partway through the binary
inputs drops the whole of the group-10 block that follows, and picks up only a trickle of the
later groups. The result looks exactly like an outstation that does not serve binary outputs.

Measured against this driver, at 3306 binary inputs, 1552 analog inputs, 298 binary output
statuses and 351 analog output statuses:

| Consumer | BI | AI | BOS | AOS | dropped |
| --- | --- | --- | --- | --- | --- |
| a handler that never drops | 3306 | 1552 | 298 | 351 | — |
| a `ChannelHandler` behind a slow reader | 257 | 1 | 0 | 0 | 5249 |

Same server, same poll, 2 ms and 8 fragments in both cases. `TestIntegrityPollAtFieldScale` pins
the first row.

To see a family a busy browser is losing, read it on its own — a range scan of group 10 returns
the binary output statuses without the rest of the database competing for the buffer.

## Statistics

`protocolConnections.stats` carries the same fields the C++ client writes. Sources differ, see
D4: byte and open/close counters come from the channel, frame and CRC counters from the bus.
`numBodyCrcError` is now a real number rather than always zero, because the bus decodes the
stream and reports its parser's counters.

`numOpenFail` counts each failed connection attempt, as opendnp3 counted them. A deliberate
shutdown is not one: a cancelled context or a closed channel propagates to the session as the
clean shutdown it is, without being counted or retried. That needs the
backoff to sit in the driver rather than in the library: go-dnp3's active channels retry inside
`Connect` and return only when the context ends, so a caller sees one context error at the end of
the cycle and never learns why the port would not open. The active channels are therefore built
with no retry of their own, and the driver retries — counting every attempt, and logging the real
reason. An operator whose serial port is missing now reads
`Connection attempt failed: channel: opening COM3: Serial port not found` rather than
`context deadline exceeded`.

## Testing

```
go test ./...          # conversions, variations, tag documents, grouping, loopback
go test -race ./...
go vet ./... && gofmt -l .
```

The loopback tests run a real master against a real outstation through `channel.Pipe` — the
whole link, transport, application and object stack — with no socket, no MongoDB and no
hardware. `TestServerMultidrop` puts two outstations and two masters on one line and checks each
master reaches only its own station.

### Running the serial port test

`TestSerialPortPair` runs a master against an outstation over two real serial ports. It skips
unless a pair is given, because a virtual port pair needs either a kernel driver on Windows or a
PTY on Linux:

```
# Linux, with a socat PTY pair (socat prints the two device names it created)
socat -d -d pty,raw,echo=0 pty,raw,echo=0
go test ./internal/dnp3util/ -run TestSerialPortPair -serial.a=/dev/pts/3 -serial.b=/dev/pts/4

# Windows, with a com0com pair (installing com0com needs an elevated shell)
go test ./internal/dnp3util/ -run TestSerialPortPair -serial.a=COM10 -serial.b=COM11
```

Everything else about serial operation is covered without hardware, including the link-layer
confirmation and the line arbitration that serial mode turns on.

### Transport coverage

| Transport | How far the tests go |
| --- | --- |
| TCP active / passive | Loopback tests, plus an end-to-end run of both binaries against a real MongoDB. |
| TLS active / passive | `TestTLSLoopback` runs a master against an outstation over a real mutually authenticated TLS socket, with certificates generated into the test's temporary directory. `TestTLSRejectsUnknownCA` proves peer verification is enforced: a client holding a certificate from another authority never gets a session. `TestTLSConfigMapping` covers the version floor and the two settings that cannot be honoured. Also run end to end with both binaries. |
| Serial | Everything except the physical port, which `TestSerialPortPair` covers as soon as a port pair exists — see below.
| UDP | `TestUDPLoopback` runs a master against an outstation over real UDP sockets; `TestUDPMultidrop` puts two stations behind one endpoint and checks each master reaches only its own; `TestUDPConfigValidation` covers the two fields a UDP connection cannot do without. `TestUDPNoPeer` pins the one thing UDP does differently: it is connectionless, so binding always succeeds and `isConnected` goes true the moment the socket is up — an absent peer shows up one layer higher, as a poll that times out, not as a link that is down. Also run end to end with both binaries. |
