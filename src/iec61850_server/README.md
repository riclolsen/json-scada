# IEC 61850 Server Driver (IEC61850_SERVER)

This driver exposes the JSON-SCADA real-time database as an **IEC 61850 MMS server**,
acting as a **telecontrol gateway / proxy** in the sense of **IEC 61850-90-2**
(*Using IEC 61850 for communication between substations and control centres*).

It works like the OPC server drivers (`OPC-UA_SERVER`, `OPC-DA_SERVER`): all points from
`realtimeData` are made available to IEC 61850 clients, **filtered by `group1` via the
connection's `topics` list** (an empty `topics` list exposes every point).

Built in C# on **.NET 8** using **libiec61850** (MZ Automation) through the vendored
`IEC61850.NET.core.2.0` wrapper.

## How it works

1. On startup the driver reads its single `protocolConnections` document, then queries
   `realtimeData` for points whose `group1` is in `topics` (excluding internal points,
   points that originate from this same connection, and — when `commandsEnabled` is false —
   command points).
2. It **dynamically builds an IEC 61850 data model** (no SCL file needed):
   - One **Logical Device per topic** (`group1`), plus a default `GEN` LD for points with no group1.
   - Each LD gets `LLN0` (Beh/Health/NamPlt) and an `LPHD1` with `Proxy.stVal = TRUE`
     (the IEC 61850-90-2 gateway marker).
   - Points are exposed as **GGIO** data objects (100 per category per GGIO instance):
     | JSON-SCADA point | CDC | GGIO object |
     |---|---|---|
     | digital monitor | SPS | `Indn` (`stVal`) |
     | analog monitor | MV | `AnInn` (`mag.f`) |
     | string monitor | VSS | `Strn` (`stVal`) |
     | digital command | SPC | `SPCSOn` |
     | analog command | APC | `AnOutn` |
   - Per-LD **datasets** (`DS_ST_k` status, `DS_MX_k` measurand, ≤100 FCDAs each) and
     **buffered + unbuffered report control blocks** (`brcbST0101`, `urcbST0101`, …),
     one pair per dataset per allowed client (`maxClientConnections`).
3. A **MongoDB change stream** (same event source as the OPC servers) pushes value, quality
   and timestamp updates into the model; libiec61850 handles reporting, buffering, integrity
   scans and general interrogation natively.
4. IEC 61850 **control operations** (SPC/APC) are mapped to `commandsQueue` documents
   (identical field set to the OPC-UA server driver) and routed by JSON-SCADA to the
   originating source device.

A **mapping manifest** (`log/iec61850_server_map_<conn>.json`) listing every tag →
object reference is written at startup — the 90-2 name-mapping deliverable.

## Command line

```
iec61850_server <instance-number> <log-level> [config-file]
```

- `instance-number`: `protocolDriverInstanceNumber` (default 1).
- `log-level`: 0=none, 1=basic, 2=detailed, 3=debug.
- `config-file`: path to `json-scada.json` (defaults to `../conf/json-scada.json`,
  then `c:/json-scada/conf/json-scada.json`).

## Connection configuration (`protocolConnections`)

| Field | Meaning |
|---|---|
| `protocolDriver` | `"IEC61850_SERVER"` |
| `protocolDriverInstanceNumber` | driver instance number |
| `protocolConnectionNumber` | unique connection number (loop-prevention key) |
| `name` | connection name — also used as the sanitized **IED name** |
| `topics` | list of `group1` values to expose (empty = all points) |
| `commandsEnabled` | allow IEC 61850 controls → `commandsQueue` |
| `ipAddressLocalBind` | `ip:port` to bind (default `0.0.0.0:102`, `:3782` for TLS) |
| `ipAddresses` | optional allow-list of client IPs (others are aborted) |
| `serverModeMultiActive` / `maxClientConnections` | max simultaneous MMS clients |
| `maxQueueSize` | buffered-report buffer depth (per BRCB) |
| `useSecurity` + `localCertFilePath` / `privateKeyFilePath` / `rootCertFilePath` / `peerCertFilesPaths` / `chainValidation` / `allowOnlySpecificCertificates` / `allowTLSv1x` | IEC 62351-3 TLS |
| `password` | optional ACSE password (reserved) |

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

## Notes & limitations

- **Static model**: the IEC 61850 model is fixed at startup. Tags added to the database
  later (e.g. by client drivers' autoCreateTags) are **not** exposed until the driver is
  restarted. New matching inserts are logged as a warning.
- **Redundancy**: only the active driver instance serves clients; the standby keeps the MMS
  server stopped so clients never read stale data.
- **Port 102 on Linux** requires elevated privileges:
  `setcap 'cap_net_bind_service=+ep' bin/iec61850_server`, or use a port > 1024 in
  `ipAddressLocalBind`.
- GOOSE/SV publishing, setting groups, log services and SCL export are out of scope for this
  release (candidate enhancements).

## Loopback end-to-end test

The JSON-SCADA `IEC61850` **client** driver can consume this server on the same node:

1. Configure an `IEC61850_SERVER` connection (e.g. conn 8001, `topics: ["KAW2"]`, port 10102).
2. Configure an `IEC61850` client connection (conn 8002, different number) pointing at
   `127.0.0.1:10102` with `autoCreateTags: true`.
3. Start `iec61850_server 1 2`, then `iec61850_client 1 2`.
4. The client discovers the model, auto-creates tags mirroring the KAW2 points, and values
   track the originals within ~1 s. Operating a `SPCSO` from the client produces a
   `commandsQueue` document carrying the original source point's routing fields.
