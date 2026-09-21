# OPC-UA Client (Go)

A pure-Go OPC UA client driver for JSON-SCADA, built on
[gopcua](https://github.com/gopcua/opcua).

It is a **drop-in alternative** to the C# driver in `src/OPC-UA-Client`, not a replacement:
same `protocolDriver` name (`OPC-UA`), same `protocolDriverInstances` / `protocolConnections` /
`realtimeData` / `commandsQueue` documents, same MongoDB semantics, same command line. Either
binary can run a given instance.

> **Run only one of them per instance number.** Both would connect to the same server and write
> the same tags.

| | `src/OPC-UA-Client` (C#) | this driver |
|---|---|---|
| Executable | `OPC-UA-Client` | `opcua-client` |
| Runtime | .NET 8 must be installed | single static binary |
| Stack | UA-.NETStandard | gopcua v0.9.1 |
| Certificates | OPC Foundation certificate stores + XML config | plain PEM / PKCS#12 files |

## Building

```bash
cd src/OPC-UA-Client-Go
go build -ldflags="-s -w" -o ../../bin/opcua-client
```

The platform build scripts (`platform-linux/build.sh`, `platform-windows/build.bat`,
`platform-mac/build.sh`) already do this.

## Running

```
opcua-client [instance] [logLevel] [config file]
```

* `instance` — `protocolDriverInstanceNumber`, default `1`.
* `logLevel` — `0` minimum, `1` basic (default), `2` detailed, `3` debug.
* `config file` — defaults to `../conf/json-scada.json`, then `c:/json-scada/conf/json-scada.json`.
  The `JS_CONFIG_FILE` environment variable is honoured too.

The instance document's `logLevel` field is **not** read — the command line is the only way to
set verbosity. That matches the C# driver, which declares the field but never applies it.

## Configuration

Instance and connection documents are exactly those of the C# driver — see
[`src/OPC-UA-Client/README.md`](../OPC-UA-Client/README.md) for the full parameter reference. All
of its `protocolConnections` fields are honoured, with these notes:

| Field | Note |
|---|---|
| `endpointURLs` | All of them are used: on a failed connection attempt the driver moves to the next URL, so a redundant server pair works. |
| `configFileName` | Only `ApplicationName`, `ApplicationUri` and `ProductUri` are read from it (D1). |
| `localCertFilePath` | `.pfx`, `.p12`, or PEM. If empty and security is on, a certificate is generated (D2, D3). |
| `pfxFilePath` | Client certificate for user authentication; same formats. |
| `passphrase` | Decrypts both of the above, including PEM files holding an `ENCRYPTED PRIVATE KEY` block. |
| `autoAcceptUntrustedCertificates` | `false` validates the server certificate against the system roots plus `conf/opcua/trusted/*.pem` (D4). |
| `giInterval` | Not used, exactly as in the C# driver (D12). |

Tag fields are likewise unchanged. `protocolSourceDiscardOldest` is not read; the driver always
discards the oldest, as the C# driver does (D12).

### Certificates

With `useSecurity: true` and a security mode other than `None`, the driver needs an application
instance certificate:

* `localCertFilePath` set → that file is loaded (`.pfx`/`.p12`/PEM, `passphrase` applied).
* `localCertFilePath` empty → a self-signed certificate is generated once into
  `conf/opcua/js_opcua_client_go_cert.pem` and `..._key.pem`, and reused on every later start.

The certificate's URI SAN becomes the session's `ApplicationUri`; servers reject a mismatch with
`BadCertificateUriInvalid`. `platform-windows/create_client_cert.ps1` produces a compatible pair.

To use the driver's generated certificate, copy `js_opcua_client_go_cert.pem` into the server's
trust list.

## Testing

```bash
go vet ./...
go test ./...
```

The tests need neither MongoDB nor a device: they start an in-process OPC UA server and run
browsing, tag creation, subscriptions and the command conversions against it.

## Deviations from the C# driver

Behaviour is matched to `src/OPC-UA-Client` on purpose, including its quirks — those are marked
`parity:` in the code. Where the two genuinely differ, the difference is numbered here and marked
`deviation Dn` in the code. **Do not diverge without adding to this list.**

| # | Deviation | Why |
|---|---|---|
| **D1** | `configFileName` is mined only for `ApplicationName`, `ApplicationUri` and `ProductUri`. Transport quotas, trace settings and certificate-store paths in that file are ignored; the driver uses its own equivalents. | gopcua has no XML application configuration and reimplementing the OPC Foundation schema is out of scope. |
| **D2** | Certificates come from explicit files rather than the OPC Foundation certificate stores (`%CommonApplicationData%\OPC Foundation\CertificateStores\...`). A missing client certificate is generated into `conf/opcua/`. | No Go implementation of those stores exists; explicit files are what the connection document already points at. |
| **D3** | `.pfx`/`.p12` files are read with `software.sslmate.com/src/go-pkcs12`; PEM pairs and encrypted PKCS#8 PEM are also accepted. | Go has no PKCS#12 in the standard library. |
| **D4** | With `autoAcceptUntrustedCertificates: false`, the server certificate is validated against the system roots plus `conf/opcua/trusted/*.pem` instead of the OPC Foundation trust list. | Same reason as D2. |
| **D5** | Endpoint selection is reimplemented to follow the C# preference order (exact policy+mode, then mode, then first) rather than using `opcua.SelectEndpoint`. | `SelectEndpoint` orders by `SecurityLevel` and would connect to a different endpoint on the same server. |
| **D6** | The JSON of structured values (`LocalizedText`, `QualifiedName`, `NodeId`, `ExtensionObject` bodies, `Guid`, `StatusCode`) differs in shape from `System.Text.Json` on the .NET types. Numbers, booleans, strings, byte strings (base64) and arrays are identical. | Different type systems. `valueJsonAtSource` and `valueBsonAtSource` are display data; `valueAtSource`, `valueStringAtSource`, `asduAtSource` and the quality flags — everything `cs_data_processor` acts on — are the same. |
| **D7** | Writes set only the Value bit of the `DataValue` encoding mask. The C# driver also sends a status code and both timestamps. | Many servers reject a write carrying timestamps with `BadWriteNotSupported`. Value-only is the interoperable form and what the other json-scada client drivers send. |
| **D8** | `CreateMonitoredItems` is chunked at 1000 items per request. | The C# stack chunks for the caller; gopcua does not, and a large server refuses an oversized request. |
| **D9** | Acquisition and the MongoDB writer run on **both** the active and the standby node; only command execution honours the active flag. | This is what the C# driver does — `ProcessMongo` and the OPC UA session never look at `Active`. Reproduced so the two binaries stay interchangeable. Note it means two redundant nodes write the same data; changing it is a decision for both drivers, and the C# one should change first. |
| **D10** | No `Esc`-key shutdown; `SIGINT`/`SIGTERM` shut down cleanly. | Drivers run under supervisord or NSSM. |
| **D11** | Reconnection uses gopcua's `AutoReconnect` (session re-activation and subscription republish) instead of `SessionReconnectHandler`. On top of it, a connection that stays out of the `Connected` state for 60 s is torn down and rebuilt against the next endpoint URL. | The give-up period is needed because gopcua's auto-reconnect retries a single URL forever, so without it a connection with several `endpointURLs` could never fail over. |
| **D12** | `giInterval` and `protocolSourceDiscardOldest` are read from the documents but never used. | Parity — the C# driver ignores them too. Listed so nobody "fixes" it here alone. |
| **D13** | Numbers rendered into `valueStringAtSource` use invariant formatting (`42.5`). The C# driver uses the machine's current culture, so on a comma-decimal locale it writes `42,5`. | The invariant form is what the rest of json-scada parses. `valueAtSource` is a BSON double in both, so nothing computed is affected. |
| **D14** | `valueJsonAtSource` is written without escaping `<`, `>` and `&` into their unicode forms. | Keeps XML elements and text readable in the database and the UI. Both forms are valid JSON and decode identically. |
| **D15** | ~~References returned by `BrowseNext` are merged back onto the node they continue.~~ **Resolved — no longer a deviation.** The C# driver used to collect them into a list it never read, so on a node with more than 1000 references everything past the first 1000 was silently discarded and never tagged. That has now been fixed in `src/OPC-UA-Client` (`MergeContinuedReferences` in `AsduReceiveHandler.cs`), so both drivers merge the continued references and agree. | It was a data-loss bug rather than behaviour worth keeping. **Measured before the fix:** on the json-scada OPC-UA server at `150.230.171.172:4840` the `JsonScadaServer` object publishes 1994 references; the C# driver kept 1000 and discarded 994, losing 43% of the address space. After the fix both drivers browse 2961 references and produce identical tag sets. |

| **D16** | Browse paths keep every character of a browse name. The C# driver builds `group2`, `protocolSourceBrowsePath` and `description` with `Path.GetDirectoryName`, a filesystem API, which collapses `//` — so a browse name holding a namespace URI becomes `Server/Namespaces/http:/opcfoundation.org/UA` there and `http://opcfoundation.org/UA` here. | The C# form is a corrupted path, and being a filesystem API its behaviour also varies by operating system. Measured against Sterfive's public demo server this affects 83 of 4157 common tags. |
| **D17** | Transport limits are left at gopcua's defaults, which advertise "no client limit" in the UACP handshake. The C# driver sets `TransportQuotas.MaxMessageSize` to 4 MB, and this driver used to copy that number. | The number is not portable. Advertising a 4 MB cap makes a server whose response exceeds it answer `BadTCPMessageTooLarge` and drop the connection. On Sterfive's demo server this happens while reading the values of a few hundred nodes with large arrays: the C# driver loses one whole batch of 500 tags to it (logged only at level 2, so invisible by default), and this driver used to lose the entire session. |
| **D18** | Discovery that does not finish is not published. When the session is lost partway through the autotag read pass, the driver reloads the tags from `realtimeData`, rebuilds the connection and browses again. The C# driver skips the failed batch of 500 nodes and carries on, leaving those points without tags until it is restarted. | A partial namespace is worse than a slower start: the missing points never appear, and nothing says so at the default log level. |

| **D19** | The value read adapts: it starts at the C# batch size of 500 and halves, down to a floor of 10, whenever a discovery attempt dies inside a value read. Repeated discovery failures also back off from 5 s up to 5 minutes. | Some servers close the connection instead of answering when a value-read response would be too large, and the C# driver's fixed 500 would then fail identically forever. Backing off also matters because a failed attempt leaves a session behind on the server — enough retries and the server stops accepting new sessions altogether. |

## Verified against real servers

Both drivers were run **concurrently**, each into its own database, with identical instance and
connection documents and `autoCreateTags: true`, then the resulting `realtimeData` collections
were compared. Four servers, four different stacks.

### Prosys OPC UA Simulation Server

`opc.tcp://<host>:53530/OPCUA/SimulationServer`. A static address space, so this is the cleanest
comparison. Ignoring the per-session diagnostics subtrees under
`Server/ServerDiagnostics/SessionsDiagnosticsSummary/<session-guid>/`, which exist only while a
session does and therefore differ by construction:

| | |
|---|---|
| Stable supervised tags | C# **443**, Go **443** |
| Tags only one driver created | **0 on either side** |
| Command tags | C# **206**, Go **206** |
| `asduAtSource` differences | **0** of 443 |
| `invalidAtSource` (quality) differences | **0** of 443 |
| Other field differences | 11, all of them D16 |

### Sterfive public demo server

`opc.tcp://opcuademo.sterfive.com:26543`. A large, dynamic address space. Both browsed the same
5636 references and read the same 5035 nodes:

| | |
|---|---|
| Supervised tags | C# **4157**, Go **4712** |
| Tags only the C# driver created | **0** |
| Tags in both | 4157 |
| Field differences over the common tags | 83, all of them D16 |

The 555 extra tags are ones the C# driver loses to D17 — a silently dropped batch of 500 — plus
nodes it excludes on attribute-read errors.

### OPC Foundation UA Sample Server

`opc.tcp://opcua.demo-this.com:51210/UA/SampleServer`. This server closes the connection
(`unexpected EOF`) rather than answering a large value read, which is what D19 exists for. The
driver converged on its own — 500, 250, 125, 62, 31, and then succeeded at **15 values per
read** — and completed discovery:

| | |
|---|---|
| References browsed | 1735 |
| Supervised tags | 1079, every one carrying data |
| Command tags | 545 |
| Distinct OPC UA types acquired | **44**, including every array form |
| Tags with bad quality | 2 |

That is the richest type coverage of the three servers: `boolean[]`, `bytestring[]`,
`datetime[]`, `double[]`, `expandednodeid[]`, `extensionobject[]`, `guid[]`, `int16[]` and the
rest all round-trip through the conversion ladder.

**No comparison against the C# driver was possible here.** The server is session-constrained and
answers `BadServerTooBusy` (`0x80EE0000`); across roughly 15 minutes and 94 attempts the C#
driver never obtained a session and produced no tags. This is not a defect in either driver —
checked at the same moment, both clients were refused identically. Worth noting that a failed
discovery attempt leaves a session behind on the server (the connection is already dead, so it
cannot be closed politely), which is the other reason D19 backs off.

### A json-scada OPC-UA server (`opc.tcp://150.230.171.172:4840/`)

The comparison that found the C# `BrowseNext` bug, and the one that confirms it is fixed. Both
drivers ran concurrently; the per-session and per-subscription diagnostics subtrees are excluded
because they exist only while a session or subscription does.

**Before the C# fix:**

| | |
|---|---|
| Stable supervised tags | C# **1309**, Go **2303** |
| Tags only the C# driver created | 0 |
| Tags only this driver created | **994**, every one under `JsonScadaServer` |

Measured directly rather than inferred: the `JsonScadaServer` object carries **1994** references;
one Browse returns the first **1000** plus a continuation point, and the C# driver discarded the
continued results — exactly **994**, 43% of the address space, including most of the json-scada
tags the server exists to publish.

**After the C# fix** (`MergeContinuedReferences`), both drivers browse the same **2961**
references and create the same **2810** monitored items:

| | |
|---|---|
| Stable supervised tags | C# **2369**, Go **2369** |
| Tags only one driver created | **0 on either side** |
| Field differences over the 2369 common tags | 8, all of them D16 |

### Observed on both servers### Observed on both servers

* `causeOfTransmissionAtSource` is more often `3` here and more often `20` in the C# driver: this
  driver receives the subscription's initial-value notification for every monitored item, while
  the C# driver received it for 157 of 558 on the Prosys server, leaving the rest showing the
  value from the initial read. The stored values are correct either way; this driver's `timeTag`
  is simply fresher.
* A `datetime` renders its `valueStringAtSource` as `2026-08-30T07:25:27.073Z` here and
  `2026-08-30T07:26:06.0950000Z` in the C# driver, which always emits seven fractional digits.
  Both are valid ISO-8601 for the same instant, and `valueAtSource` — the Unix milliseconds the
  command path reads back — is identical in form. Part of D6.

## Known limitations

* **Methods are dispatched but not yet verified against a real server.** The driver resolves the
  object owning a method by browsing its hierarchical references backwards and then issues the
  `Call` service; the in-process test server used by `go test` does not implement `Call`, so that
  last hop is exercised only against real equipment. Parent resolution is covered by tests.
* History, events, alarms and PubSub are not implemented — the C# driver does not have them
  either.
