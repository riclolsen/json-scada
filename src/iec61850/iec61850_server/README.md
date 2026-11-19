# IEC 61850 Server (Go)

This driver exposes the JSON-SCADA real-time database as an **IEC 61850 MMS server**, acting as a telecontrol gateway / proxy in the sense of **IEC 61850-90-2**. It is written in Go on top of
[go-iec61850](https://github.com/dscsystems/go-iec61850), a pure-Go implementation of the protocol stack.

    Binary:  iec61850_server (iec61850_server.exe on Windows)
    Service: JSON_SCADA_iec61850server (Windows), [program:iec61850server] (Linux)
    Log:     log/iec61850server.log

It works making all points from `realtimeData` available to IEC 61850 clients, **filtered by `group1` via the
connection's `topics` list** (an empty `topics` list exposes every point).

## How it works

1. On startup the driver reads its single `protocolConnections` document, then queries
   `realtimeData` for points whose `group1` is in `topics` (excluding internal points, points that
   originate from this same connection, and — when `commandsEnabled` is false — command points).
2. It **builds an IEC 61850 data model** (no SCL file needed):
   - one **logical device per topic** (`group1`), plus a default `GEN` device for points with none;
   - each device gets `LLN0` (Beh/Health/NamPlt/LastApplError) and `LPHD1` with `Proxy.stVal = TRUE`,
     the IEC 61850-90-2 gateway marker;
   - points become **GGIO** data objects, at most 30 per instance; further points open `GGIO2`,
     `GGIO3`, … Each instance numbers its own objects from 1, as real IEDs do:

     | JSON-SCADA point | CDC | GGIO object |
     |---|---|---|
     | digital monitor | SPS | `Indn` (`stVal`) |
     | analog monitor | MV | `AnInn` (`mag.f`) |
     | string monitor | VSS | `Strn` (`stVal`) |
     | digital command | SPC | `SPCSOn` |
     | analog command | APC | `AnOutn` |

   - per-device **data sets** (`DS_ST_k` status, `DS_MX_k` measurand, ≤40 entries each) and
     **buffered + unbuffered report control blocks** (`brcbST0101`, `urcbST0101`, …), one pair per
     data set with as many instances as `maxClientConnections` (capped at 3, so the status and the
     measurand blocks of a logical device fit the 7 per `LLN0`; the C# driver allowed 4).
3. A **MongoDB change stream** pushes value, quality and timestamp updates into the model; the
   library handles reporting, buffering, integrity scans and general interrogation. Each time the
   stream is opened (at startup and after a MongoDB outage or failover) the current values are read
   again, so changes made while it was closed are not missed.
4. IEC 61850 **control operations** (SPC/APC) become `commandsQueue` documents, which JSON-SCADA
   routes to the driver that owns the source point. The command tag's `kconv1`/`kconv2` are applied
   to the queued value, as in the lib60870 server drivers:

   | tag `type` | queued value |
   |---|---|
   | `digital`, `kconv1` ≠ -1 | forced to `1`/`0` (`ctlVal` true → 1) |
   | `digital`, `kconv1` = -1 | forced to `1`/`0` and **inverted** |
   | `analog` | `value * kconv1 + kconv2` |

   A control flagged **test** is refused (`Blocked-by-mode`): the gateway has no test mode, and
   the command it would queue is a real one for the source device.

A **mapping manifest** (`log/iec61850_server_map_<conn>.json`) listing every tag → object reference
is written at startup.

## Command line

```
iec61850_server [instance [logLevel [configFile]]]
```

```
iec61850_server selftest [port [bulkPoints]]
```

- **instance** — `protocolDriverInstanceNumber`. *Optional, default 1*.
- **logLevel** — 0=none, 1=basic, 2=detailed, 3=debug (3 also traces MMS PDUs). *Optional, default 1*.
- **configFile** — path of `json-scada.json`. *Optional, default `../conf/json-scada.json`, then
  `c:/json-scada/conf/json-scada.json`*. `JS_CONFIG_FILE` is also honoured.
- **selftest** — builds and serves a synthetic model without MongoDB, so the model can be browsed
  with any IEC 61850 client. `bulkPoints` adds N extra points to exercise the layout at scale.

## Connection configuration (`protocolConnections`)

| Field | Meaning |
|---|---|
| `protocolDriver` | `"IEC61850_SERVER"` |
| `protocolConnectionNumber` | unique connection number (also the loop-prevention key) |
| `name` | connection name — sanitized into the **IED name** |
| `topics` | list of `group1` values to expose (empty = all points) |
| `commandsEnabled` | allow IEC 61850 controls → `commandsQueue` |
| `ipAddressLocalBind` | `ip:port` to bind (default `0.0.0.0:102`, `:3782` when `useSecurity`) |
| `ipAddresses` | optional allow-list of client IPs (others are disconnected) |
| `serverModeMultiActive` / `maxClientConnections` | max simultaneous MMS clients, and the number of report control block instances per data set |
| `maxQueueSize` | buffered-report depth, in **reports** per buffered control block |
| `useSecurity` + `localCertFilePath` / `privateKeyFilePath` / `rootCertFilePath` / `peerCertFilesPaths` / `chainValidation` / `allowOnlySpecificCertificates` / `allowTLSv1x` / `cipherList` | IEC 62351-3 TLS; `allowOnlySpecificCertificates` needs at least one certificate in `peerCertFilesPaths` |
| `password` | passphrase of an encrypted `privateKeyFilePath` (PKCS#8 or legacy PEM encryption), as in the C# driver |

Example:

```json
{
  "protocolDriver": "IEC61850_SERVER",
  "protocolDriverInstanceNumber": 1,
  "protocolConnectionNumber": 8001,
  "name": "IEC61850SRV",
  "description": "IEC61850-90-2 Gateway Server",
  "enabled": true,
  "commandsEnabled": true,
  "ipAddressLocalBind": "0.0.0.0:102",
  "topics": ["KAW2", "KIK3"],
  "serverModeMultiActive": true,
  "maxClientConnections": 4,
  "maxQueueSize": 5000,
  "useSecurity": false
}
```

## Model sizing

The model is bounded so that no MMS response grows past what a client with a small negotiated PDU can take. The bounds are:

| Bound | Default | Why |
|---|---|---|
| objects per logical node | 30 | a logical node's type description is returned in one response |
| entries per data set | 40 | reading a data set returns every member at once |
| report control blocks per LLN0 | 7 | `LLN0[BR]`/`[RP]` returns every block of that family at once |
| RCB copies per data set | 3 | each instance multiplies that response; a status and a measurand data set must fit 7 |
| description length | 32 | `d` is published per object and one `[DC]` read returns them all |
| string point cost | 8 | a string value can be far larger than a status or measurand |

Points per logical device are derived from `maxClientConnections`, and topics larger than the limit
are split across several logical devices (`KAW2`, `KAW2_2`, …). The bounds are logged at startup:

```
Model bounds: 2 RCB copies/data set, <= 80 points per logical device,
              <= 40 entries per data set, <= 30 objects per logical node
Topic 'BULK' has 3000 points - split across 38 logical devices.
```

Unlike libiec61850, the Go stack does not refuse oversized responses (COTP segments them), so these
bounds protect clients rather than the server.

## Notes and limitations

- **Static model**: the model is fixed at startup. Tags added later are not exposed until the driver
  is restarted; matching updates for unknown tags are logged at level 2.
- **No redundancy control**: an MMS server is a passive TCP listener, so this driver is always
  active — every node of a redundant pair listens and serves, and the clients choose which one they
  talk to. This differs from the C# driver, which stops the server on the standby node. The driver
  still publishes `activeNodeName` / `activeNodeKeepAliveTimeTag` on its instance document and the
  `stats` on its connection document every 5 s; with more than one node running, those fields show
  whichever node wrote last.
- **Listener**: if the listening socket fails, the server stops reporting itself as running and
  binds a new one within a second; the model and its values are kept.
- **Port 102 on Linux** needs privileges: `setcap 'cap_net_bind_service=+ep' bin/iec61850_server`,
  or use a port above 1024 in `ipAddressLocalBind`.
- GOOSE/SV publishing, setting groups, log services and SCL export are out of scope.

**RptID in the control block:** go-iec61850 leaves a report control block's `RptID` empty and sends
the instance reference (`LD/LLN0$BR$brcbMX0101`) in its reports, as IEC 61850-8-1 prescribes. A
client that reads `RptID` would then see an empty string and, if it matches incoming reports against
what it read (IEDExplorer does), could not attach the values to its model. `fixReportIDs` in
`server.go` writes each instance's own reference into the attribute, as libiec61850 does, so the
value read and the value reported agree.

Reproduced on purpose, so both drivers present the same gateway: the model layout and object
references, the sizing bounds (except the RCB copies, see above), description truncation (descriptions are
also reduced to printable ASCII, with accents dropped, since `d` is a VisibleString), `invalid` treated as true when the field is
missing, command points excluded from data sets and never written from the database, and one
logical device per topic with the same splitting rule.
