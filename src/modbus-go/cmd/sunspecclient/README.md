# SunSpec Client Driver (Go)

Reads SunSpec devices (PV inverters, meters, batteries, storage and other DER equipment) over
Modbus, discovers the SunSpec models each device exposes, creates the {json:scada} tags of the
discovered points, keeps them updated and writes the commands from `commandsQueue` to the
devices. It registers as protocol driver **`SUNSPEC`**.

The binary is `bin/sunspec-client` (`bin\sunspec-client.exe` on Windows); the service is
`sunspecclient` under Supervisor and `JSON_SCADA_sunspecclient` on Windows. The protocol is
[go-sunspec](https://github.com/riclolsen/go-sunspec) over go-modbus. Transports, TLS, the
start-up, redundancy and logging are those of the [MODBUS client](../modbusclient/README.md).

## Running

```
sunspec-client [instance] [logLevel] [configFile]
```

| Argument | Default | Environment override |
|---|---|---|
| instance number | `1` | `JS_SUNSPEC_INSTANCE` |
| log level, 0 to 3 | `1` | `JS_SUNSPEC_LOGLEVEL` |
| config file | the first that exists of `../conf/json-scada.json`, `../../conf/json-scada.json`, `c:/json-scada/conf/json-scada.json` | `JS_SUNSPEC_CONFIG_FILE`, then `JS_CONFIG_FILE` |

At startup the driver creates the `protocolDriverInstances` document for `SUNSPEC` when there
is none, exits when the instance is disabled, and loads the enabled `protocolConnections` with
`protocolDriver: "SUNSPEC"` and its instance number (it exits when there are none).

## Connection configuration (`protocolConnections`)

```js
db.protocolConnections.insertOne({
  protocolDriver: 'SUNSPEC',
  protocolDriverInstanceNumber: 1.0,
  protocolConnectionNumber: 3101.0,   // unique across the whole system
  name: 'PV_INV1',
  description: 'PV inverter',
  enabled: true,
  commandsEnabled: true,

  connectionMode: 'TCP Active',        // TCP Active | TLS Active | Serial | RTU over TCP | RTU over TLS
  ipAddresses: ['192.168.0.120:502'],
  ipAddressLocalBind: '',
  portName: '', baudRate: 9600.0, parity: 'Even', stopBits: 'One', handshake: 'None',

  timeoutMs: 1000.0,
  maxRetries: 2.0,
  pollingInterval: 5000.0,
  giInterval: 300.0,
  interRequestDelayMs: 0.0,

  unitIds: [1.0],
  baseAddresses: [],                   // empty: 0, 40000, 50000, 1, 39999, 40001, 49999, 50001
  registerType: 'holding',             // holding | input
  maxModels: 256.0,
  modelIds: [],                        // empty: tags for every model

  autoCreateTags: true,
  autoCreateCommandTags: true,
  autoCreateUnimplemented: false,

  localCertFilePath: '', privateKeyFilePath: '', privateKeyPassphrase: '',
  peerCertFilePath: '', chainValidation: true, allowOnlySpecificCertificates: false,
  cipherList: '', allowTLSv12: true, allowTLSv13: true,

  keepProtocolRunningWhileInactive: false,
  peerIdentification: true,
  stats: null
})
```

Every key is optional except `protocolDriver`, `protocolDriverInstanceNumber`,
`protocolConnectionNumber` and `enabled`. A missing key, `null`, an empty string or a value of
the wrong type takes the default.

### Keys shared with the MODBUS client

These keys mean what they mean for the [MODBUS client](../modbusclient/README.md#connection-configuration-protocolconnections):

- general: `name` (default `SUNSPEC-<number>`), `enabled`, `commandsEnabled`,
  `keepProtocolRunningWhileInactive`, `peerIdentification`;
- transport: `connectionMode`, `ipAddresses` (port 502, or 802 in the TLS modes),
  `ipAddressLocalBind`, `portName`, `baudRate`, `parity`, `stopBits`, `handshake`;
- timing: `timeoutMs`, `maxRetries` (retries of a read that timed out; writes are not
  retried), `pollingInterval`, `giInterval`, `interRequestDelayMs`;
- TLS: `localCertFilePath`, `privateKeyFilePath`, `privateKeyPassphrase`, `peerCertFilePath`,
  `chainValidation`, `allowOnlySpecificCertificates`, `cipherList`, `allowTLSv12`, `allowTLSv13`;
- `autoCreateTags` (default `false`): create the tags of the discovered points, see
  [Automatic tag creation](#automatic-tag-creation).

The MODBUS-only keys (`maxReadRegisters`, `maxReadCoils`, `maxAddressGap`, the byte orders,
`stringEncoding`, `useModiconAddresses`, `useMaskWrite`, `interFrameDelayMs`, `topics`) are not
used: SunSpec fixes the encoding and the read layout.

### SunSpec keys

| Key | Default | Meaning |
|---|---|---|
| `unitIds` | `[1]` | Modbus unit ids of the SunSpec devices behind the address (several for a gateway or a serial line), 1 to 255; unit 0 (broadcast) is ignored. Each is discovered on its own. |
| `baseAddresses` | `[]` | Register addresses probed, in order, for the `SunS` marker. Empty uses go-sunspec's list: 0, 40000, 50000, 1, 39999, 40001, 49999, 50001. |
| `registerType` | `holding` | `holding` (function 3, nearly every device) or `input` (function 4; the map is then read-only and no command tags are created). |
| `maxModels` | `256` | Most model headers read from a chain, the end marker included. |
| `modelIds` | `[]` | Model ids whose points get tags; empty means every model. Tags that exist are polled whatever this list says. |
| `autoCreateCommandTags` | `true` | With `autoCreateTags`, also create a command tag for every writable (`RW`) point of a model's top-level group. |
| `autoCreateUnimplemented` | `false` | With `autoCreateTags`, also create tags for the points the device reports as not implemented. |

## Discovery

On every connection the driver discovers each unit of `unitIds`: it probes the base addresses
for the `SunS` marker and walks the model chain to the end model, then reads every model once.
The connection is `connected` when at least one unit has a SunSpec map; with none the link
counts as failed and is retried with the back-off of the MODBUS client (1 s doubling to 30 s,
moving to the next address of `ipAddresses`). A configured unit without a map is discovered
again on every integrity cycle.

When a read finds a model header that no longer matches the discovery (a firmware update, a
reconfigured device), the unit is discovered again before the next cycle, its identity is
compared again and, with `autoCreateTags`, the tags of new points are created.

## Tag addressing (`realtimeData`)

| Field | Value |
|---|---|
| `protocolSourceConnectionNumber` | The connection's `protocolConnectionNumber`. |
| `protocolSourceCommonAddress` | The unit id (default 1). |
| `protocolSourceObjectAddress` | `<modelId>[#<occurrence>]/[<group>[<index>]/...]<point>`, see below. |
| `protocolSourceASDU` | The SunSpec point type (`int16`, `uint32`, `acc32`, `enum16`, `bitfield32`, `float32`, `string`, ...); informative. |

The object address names a point by its model and the path of group instances to it:

- `103/W` — point `W` of the first model 103 in the chain;
- `160/module[2]/DCW` — point `DCW` of the second instance of the repeating group `module`;
- `160#2/module[1]/DCA` — the second model 160 of the chain (`#n` counts the models with the
  same id from 1; it is left out for the first);
- `705/curve[1]/pt[3]/V` — nested groups.

Group and point names are the SunSpec schema names. A tag whose model or point the device does
not expose stays invalid and is reported once per connection (`access.object_nonexistent`).

## Values

- Points with a scale factor publish the scaled value (engineering units); a scale factor the
  device reports as not implemented, or that cannot be resolved, makes the tag invalid.
- Enum and bitfield points publish the raw number in `value` and the names of the active
  symbols (comma separated) in `valueString`.
- `string`, `ipaddr`, `ipv6addr` and `eui48` points are string tags.
- A point the device reports as not implemented (0x8000, 0xFFFF, NaN, ...) is invalid.
- The driver does not apply `kconv1`/`kconv2`; the data processor does.

Each poll cycle reads, per unit, the model instances the tags use (adjacent models in shared
requests of up to 125 registers) and publishes the values that changed; every `giInterval`
seconds an integrity cycle publishes every value (cause of transmission 20, else 3). A link
that does not answer marks every tag invalid, keeping its last value. Values are written in
the order they were read: an older poll value never overwrites a newer one (a command
read-back), and a value whose write failed is written again on the next cycle.

## Automatic tag creation

With `autoCreateTags`, after each discovery every point of every decoded model instance
(repeating groups included) without a tag gets one. Scale factor (`sunssf`) and `pad` points,
the model header (`ID`, `L`) and, unless `autoCreateUnimplemented`, the points the device
reports as not implemented get no tag. A tag is matched by unit and address, so a renamed tag
is not created again.

| Field | Value |
|---|---|
| `tag` | `<connection name>;<unit>;<address>`, e.g. `PV_INV1;1;103/W` (a suffix `~2`, `~3`, ... when another tag, of any connection, has the name). |
| `_id` | The next free key in the connection's range (`protocolConnectionNumber` × 1,000,000). |
| `type` | `string` for string points, else `analog`. |
| `description` | `<connection>~<model label>~<group path> <point label>`. |
| `ungroupedDescription`, `group1`, `group2`, `group3` | Point label, connection name, model label (`unit N ` first when the connection has several units, ` #n` for a repeated model), group path. |
| `unit` | The schema unit of the point (`W`, `V`, `Hz`, `Wh`, ...). |
| `notes` | The schema description of the point. |

A writable point of a model's top-level group also gets, with `autoCreateCommandTags`, a
command tag `<tag>;cmd` (`origin: "command"`) linked to the supervised one
(`supervisedOfCommand` / `commandOfSupervised`). Count points of repeating groups are not
writable.

## Commands

A `commandsQueue` document for the connection is executed when the node is active and the
link is connected:

1. A command older than its expiry is not executed (`command expired`, `cmd.stale_time`).
   This, the node being active and the session are checked again when the command gets the
   link (after a poll request in progress), so a command is never sent on a new session or
   after a failover.
2. The address must name a top-level point of a discovered model of the unit; otherwise
   `cmd.object_unknown` (or `cmd.rejected` for a point inside a group).
3. The value is `value` for numeric points (engineering value: the driver applies the point's
   current scale factor) and `valueString` for string points.
4. go-sunspec reads the model (header check, scale factor), writes the point's registers with
   function 16 and reads the point back; the read-back value is published to the supervised
   tags of the point at once.

The command is acknowledged `OK`, or with the failure: `point not writable` (`R` point,
count point, input registers, unusable scale factor), a value the encoder refuses, a device
exception, `response timeout`, `connection down`. Commands run one at a time, in arrival
order, and overtake polling.

## Statistics

| Field | Meaning |
|---|---|
| `status` | `connecting`, `connected` (a SunSpec device discovered), `disconnected` or `stopped`. `stats.isConnected` is `true` while connected. |
| `connectedTo` | `tcp://host:port`, `tls://host:port` or `serial://port@baud`. |
| `reads`, `writes` | Model read batches (one per unit and cycle) and point writes. |
| `errors`, `timeouts`, `exceptions`, `lastError` | Transport errors, timed-out attempts, exception responses, text of the last failure. |
| `giCount` | Integrity cycles run. |
| `discoveries`, `discoveredUnits`, `models` | Discovery attempts, units with a SunSpec map, models discovered. |
| `peerIdentity` | The device's identity, when `peerIdentification` is on. |

## Peer identification

With `peerIdentification` on (the default), after each discovery the driver reads the Common
model (1) of every discovered unit, and records the identity in `stats.peerIdentity`:
`manufacturer` (`Mn`), `model` (`Md`), `options` (`Opt`), `version` (`Vr`), `serialNumber`
(`SN`), `deviceAddress` (`DA`), the base address, register type and model chain of each unit,
plus the MAC address of the device's IP (TCP and TLS modes) and its TLS certificate.

`keys` holds what is compared: the identification of each unit (`unit:N`), its model chain
(`models:N`), the MAC address and the certificate. A change is reported as
`peer.identity_changed` (a firmware version, serial number or model chain change),
`peer.mac_changed` or `tls.cert_changed`, also across a driver restart; the first
identification as `peer.identified`.

```json
{
  "collectedAt": "2026-11-23T12:00:00Z",
  "address": "192.168.0.120:502",
  "ip": "192.168.0.120",
  "mac": "00:1a:2b:3c:4d:5e",
  "units": [
    {
      "unitId": 1,
      "baseAddress": 40000,
      "registerType": "holding",
      "models": "1,103,120,123",
      "manufacturer": "Acme",
      "model": "PV-5000",
      "version": "1.0.0",
      "serialNumber": "A-0001",
      "deviceAddress": "1"
    }
  ],
  "keys": [
    { "k": "models:1", "v": "1,103,120,123" },
    { "k": "unit:1", "v": "Acme | PV-5000 | 1.0.0 | A-0001" },
    { "k": "mac", "v": "00:1a:2b:3c:4d:5e" }
  ]
}
```

## Security events

The driver reports to the `secEvents` collection (see `docs/schema.md`) with `protocol:
"sunspec"` (`tls` for handshake events), using the same writer and kinds as the MODBUS client.
go-sunspec's `secevent` package (go-modbus's security event API) reports only on servers, so
the events of this client come from the driver, except where the source is the library's log
of a mismatched response.

| Kind | When |
|---|---|
| `tls.cert_rejected`, `tls.handshake_failed` | The device's certificate was refused, or the handshake failed otherwise. |
| `peer.identified`, `peer.identity_changed`, `peer.mac_changed`, `tls.cert_changed` | First identification of the device, and changes of it. |
| `access.unit_unknown` | A configured unit has no SunSpec map, or a gateway answers that it does not reach it (exception 0x0A/0x0B). Once per unit and connection. |
| `access.object_nonexistent` | The device answered ILLEGAL DATA ADDRESS to a model read, or a configured tag names a unit, model or point the device does not expose. Once per connection. |
| `access.unrecognized_service` | The device answered ILLEGAL FUNCTION to the discovery or a model read. |
| `proto.bad_response`, `proto.bad_frame`, `proto.unit_mismatch`, `proto.txn_mismatch` | An invalid model chain or model data, a malformed response, a bad CRC or a short frame, a response from another unit, an unexpected transaction id (not shortly after a timeout). |
| `cmd.object_unknown` | A command to an address that does not parse, or to a unit, model or point that does not exist. |
| `cmd.rejected` | A command refused by the driver (not writable, inside a group, invalid value, model chain changed) or by the device (an exception). |
| `cmd.stale_time` | A command older than its expiry: not executed. |
| `flood.controls` | More than 10 commands to the device within 10 seconds. |
| `cmd.unnecessary` | A numeric command to the value its supervised point already had (checked in the background; the command is still sent). |

A repeating condition is reported once every 10 minutes, with the number of occurrences in
`count`.

## Testing

`go test ./internal/sunspecapp/` runs the driver against an in-process SunSpec device
(go-sunspec `NewServer` behind a go-modbus server on a loopback port): discovery, automatic
tags, polling with scale factors and enum symbols, a command and the refused ones, the unit
without a device and the security events.
