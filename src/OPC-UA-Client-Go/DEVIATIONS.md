# Deviations from the former C# driver

Developer reference. The driver replaced the C# `OPC-UA-Client`, which has been removed from the
repository; `parity:` comments in the code mark behaviour kept on purpose from that driver, and
each place where this driver intentionally differs carries a `deviation Dn` comment. Add to this
list rather than diverging silently. Users read [README.md](README.md), not this file.

Numbers D2, D3, D13, D14, D20 and D22 have no description in the code; D2 and D3 are the
certificate items covered by `security.go` (D1 to D4).

| # | Deviation | Where |
|---|---|---|
| D1 | Only `ApplicationName`, `ApplicationUri` and `ProductUri` are read from `Opc.Ua.DefaultClient.Config.xml` | `security.go` |
| D2, D3 | Certificates are plain files (PKCS#12 or PEM), a self-signed one is generated; no OPC Foundation certificate store | `security.go` |
| D4 | `autoAcceptUntrustedCertificates: false` checks the server certificate against system roots and `conf/opcua/trusted` | `security.go` |
| D5 | Endpoint selection follows the C# preference order, not gopcua's `SelectEndpoint` (security level) | `session.go` |
| D6 | JSON of structured values differs from System.Text.Json | `uaconv.go` |
| D7 | Commands write only the value bit of the DataValue (no status, no timestamps) | `mongo_commands.go` |
| D8 | `CreateMonitoredItems` is chunked at 1,000 items | `subscribe.go` |
| D9 | The active flag gates command execution only; both nodes acquire and write | `redundancy.go` |
| D10 | Stopped by signal instead of the Esc key | `main.go` |
| D11 | A connection out of the Connected state for 60 s is torn down and rebuilt on the next endpoint URL | `session.go` |
| D12 | `protocolSourceDiscardOldest` and `giInterval` are ignored, as in C# | `subscribe.go`, `config.go` |
| D15 | Continuation-point references are merged into the browse result (the C# driver dropped them until fixed) | `browse.go` |
| D16 | Browse paths do not collapse `//` | `autotag.go` |
| D17 | Transport quotas are left at gopcua's defaults | `session.go` |
| D18 | A discovery that does not complete is not published; the connection is rebuilt | `session.go` |
| D19 | Value reads shrink (500 down to 10) after a server drops the connection during one | `autotag.go` |
| D21 | A finite value or array element too large for Float/Double is refused, not turned into Infinity | `commandconv.go` |
| D23 | Per-session and per-subscription server diagnostics are never tagged or monitored | `diagnostics.go` |
| D24 | One item may deliver at most 200 values per second; the excess is dropped | `subscribe.go` |
| D25 | Rejected monitored items are logged once per status code | `subscribe.go` |
| D26 | Updates over 3 MB encoded are dropped, not sent to fail with BSONObjectTooLarge | `mongo_update.go` |
| D27 | Browse depth is a path-depth limit, not a request count, and ends only when nothing is left to browse; a lost continuation point, an exhausted `BadNoContinuationPoints` retry budget or a node attribute batch that fails twice fails the discovery instead of publishing a partial namespace | `browse.go`, `autotag.go` |
| D28 | Session health: each subscription monitors the server clock, any notification counts as alive, silence beyond `max(30 s, 5 publishing intervals)` or an unrecoverable subscription error rebuilds the session; an incomplete subscription setup rebuilds it up to 3 times in a row. Notification pumps are bound to the session context | `subscribe.go`, `session.go` |
| D29 | Before the tags are reloaded after a failed discovery the driver waits for the writer to drain (and 0.5 s for the unacknowledged writes to land), so the reload sees the tags the partial pass created and the key allocator restarts above them | `mongo_update.go`, `session.go` |
| D30 | Endpoint fallback never picks a security mode weaker than configured; it fails instead | `session.go` |
| D31 | A node whose value reads null is registered for monitoring once, however many times the session is rebuilt | `autotag.go` |
| D32 | Shutdown waits (up to 10 s) for the sessions to close | `main.go` |
| D33 | Each connection executes its commands on its own worker, in order; more than 256 waiting are refused with `busy` | `mongo_commands.go` |
| D34 | Method arguments follow the declared `InputArguments` types; `ByteString`, `NodeId`, `ExpandedNodeId`, `Guid`, `LocalizedText`, `QualifiedName` and `XmlElement` commands are sent as that type, not as a String | `mongo_commands.go` |
| D35 | `timeoutMs` of zero or less means the 20,000 ms default | `config.go` |

## How it was verified

`go test ./...` runs the conversion, tag document, browse, command and loopback tests against an
in-process gopcua server. End-to-end comparison against the C# driver was done on public OPC UA
demo servers while it still existed; the outcomes are not reproduced here.
