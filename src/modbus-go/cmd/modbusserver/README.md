# MODBUS_SERVER Driver (Go)

Serves {json:scada} tags to third-party Modbus masters and relays their writes back into
{json:scada} as commands. It registers as protocol driver **`MODBUS_SERVER`**, the same name as
the former Node.js driver it replaces, and reads the same configuration documents.

The binary is `bin/modbus-server` (`bin\modbus-server.exe` on Windows). The service keeps its
old name: `modbusserver` under Supervisor and `JSON_SCADA_modbusserver` on Windows.

Transports, value types, byte orders, addressing and logging levels are shared with the
client driver and are described in the [module README](../../README.md).

## Running

```
modbus-server [instance] [logLevel] [configFile]
```

| Argument | Default | Environment override |
|---|---|---|
| instance number | `1` | `JS_MODBUSSRV_INSTANCE` |
| log level, 0 to 3 | `1` | `JS_MODBUSSRV_LOGLEVEL` |
| config file | the first that exists of `../conf/json-scada.json`, `../../conf/json-scada.json`, `c:/json-scada/conf/json-scada.json` | `JS_MODBUSSRV_CONFIG_FILE`, then `JS_CONFIG_FILE` |

A command-line argument takes precedence over its environment variable.

At startup the driver:

1. reads the config file and connects to MongoDB, retrying until it succeeds;
2. creates the `protocolDriverInstances` document for `MODBUS_SERVER` and this instance number
   when there is none;
3. exits when that instance has `enabled: false`, and keeps checking every 30 seconds;
4. loads the `protocolConnections` with `protocolDriver: "MODBUS_SERVER"`, this instance
   number and `enabled: true`. When there are none it logs
   `No enabled connections found for driver MODBUS_SERVER instance N. Exiting.` and exits with
   code 0.

The server takes part in redundancy like the client: it listens only on the active node,
unless `keepProtocolRunningWhileInactive` is `true`.

## Connection configuration (`protocolConnections`)

```js
use json_scada_db_name
db.protocolConnections.insertOne({
  protocolDriver: 'MODBUS_SERVER',
  protocolDriverInstanceNumber: 1.0,
  protocolConnectionNumber: 3101.0,
  name: 'MODBUS_SRV_1',
  description: 'Modbus TCP slave for external SCADA',
  enabled: true,
  commandsEnabled: true,

  connectionMode: 'TCP Passive',       // TCP Passive | TLS Passive | Serial |
                                       // RTU over TCP Passive | RTU over TLS Passive
  ipAddressLocalBind: '0.0.0.0:502',
  maxClientConnections: 8.0,
  clientIdleTimeoutMs: 60000.0,
  ipAddresses: [],

  portName: 'COM4', baudRate: 9600.0, parity: 'Even', stopBits: 'One', handshake: 'None',
  interFrameDelayMs: 0.0,

  serverUnitIds: [1.0],
  strictUnitId: false,
  serveUnmappedAsZero: false,

  byteOrder16: 'AB', byteOrder32: 'ABCD', byteOrder64: 'ABCDEFGH', byteOrderStr: 'AB',
  stringEncoding: 'latin1',
  useModiconAddresses: false,
  invalidValuePolicy: 'last',
  allowWritesToSupervised: false,

  localCertFilePath: '', privateKeyFilePath: '', privateKeyPassphrase: '',
  peerCertFilePath: '', chainValidation: true,
  allowOnlySpecificCertificates: false, cipherList: '',
  allowTLSv12: true, allowTLSv13: true,

  keepProtocolRunningWhileInactive: false,
  stats: null
})
```

Every key is optional except `protocolDriver`, `protocolDriverInstanceNumber`,
`protocolConnectionNumber` and `enabled`. A missing key, `null`, an empty string or a value of
the wrong type takes the default.

### General

| Key | Default | Meaning |
|---|---|---|
| `protocolConnectionNumber` | `0` | Connection number; tags point to it with `protocolDestinationConnectionNumber`. |
| `name` | `MODBUS_SERVER-<number>` | Prefix of every log line of the connection, and part of the relayed commands' `originatorUserName`. |
| `enabled` | `true` | Only enabled connections are loaded. |
| `commandsEnabled` | `true` | When `false`, every write is answered with exception 01 ILLEGAL FUNCTION. |
| `keepProtocolRunningWhileInactive` | `false` | When `true`, the server also listens on the standby node of a redundant pair. |

### Transport

| Key | Default | Meaning |
|---|---|---|
| `connectionMode` | `TCP Passive` | `TCP Passive`, `TLS Passive`, `Serial`, `RTU over TCP Passive` or `RTU over TLS Passive`. An unknown value is logged and treated as `TCP Passive`. |
| `ipAddressLocalBind` | `0.0.0.0:502` | Listening address, `ip`, `ip:port`, `[v6]` or `[v6]:port`. A missing port is 502, or 802 in the TLS modes. |
| `maxClientConnections` | `8` | Masters connected at the same time. A further master is disconnected at once. `0` means no limit. |
| `clientIdleTimeoutMs` | `60000` | A master that sends nothing for this long is disconnected. `0` disables the timeout. |
| `ipAddresses` | `[]` | Masters allowed to connect, by IP address. Empty allows any. |
| `portName` | empty | Serial port, such as `COM4` or `/dev/ttyUSB0`. |
| `baudRate` | `9600` | Serial speed; also sets the RTU frame timing in the RTU modes. |
| `parity` | `Even` | `None`, `Even` or `Odd`. `Mark` and `Space` are not supported: they are logged and treated as `None`. |
| `stopBits` | `One` | `One` or `Two`. `One5` is treated as `Two`. |
| `handshake` | `None` | Flow control is not supported. Any other value is logged and ignored. |
| `interFrameDelayMs` | `0` | Silence that ends an RTU request frame, in milliseconds. `0` means 3.5 character times at `baudRate`, rounded up, at least 2 ms. |

The serial port always uses 8 data bits. When the listening socket or the serial port cannot be
opened, the driver logs it and tries again every 5 seconds.

### Unit ids and unmapped addresses

| Key | Default | Meaning |
|---|---|---|
| `serverUnitIds` | `[1]` | Unit ids the server answers when `strictUnitId` is `true`, besides the unit ids that have mapped tags. |
| `strictUnitId` | `false` | When `true`, a request for any other unit id gets no answer, as on a multi-drop line. When `false`, every unit id is answered. |
| `serveUnmappedAsZero` | `false` | When `true`, reading an address no tag maps returns 0. When `false`, it returns exception 02 ILLEGAL DATA ADDRESS. |

Unit 0 (broadcast) is always accepted. In the RTU modes a request to unit 0 is executed and not
answered.

### Value encoding

| Key | Default | Meaning |
|---|---|---|
| `byteOrder16` | `AB` | Byte order of 16-bit values whose ASDU names none. |
| `byteOrder32` | `ABCD` | Byte order of 32-bit values. |
| `byteOrder64` | `ABCDEFGH` | Byte order of 64-bit values. |
| `byteOrderStr` | `AB` | Byte order inside each register of a string: `AB` or `BA`. |
| `stringEncoding` | `latin1` | `latin1`, `utf8` or `ascii`. |
| `useModiconAddresses` | `false` | Accept plain 1-based Modicon numbers (`40001`) as object addresses. |
| `invalidValuePolicy` | `last` | What an invalid tag serves: `last` keeps the last value, `zero` serves 0. Modbus has no quality flags. |
| `allowWritesToSupervised` | `false` | When `true`, a write to a supervised tag's address stores the written value in the server's register bank (not in the tag) instead of being refused. |

### TLS

Used in `TLS Passive` and `RTU over TLS Passive`.

| Key | Default | Meaning |
|---|---|---|
| `localCertFilePath` | empty | Server certificate (PEM). Required. |
| `privateKeyFilePath` | empty | Private key of that certificate (PEM: PKCS#1, PKCS#8 or EC). When empty, the key is read from the certificate file. |
| `privateKeyPassphrase` | empty | Passphrase of an encrypted key. |
| `peerCertFilePath` | empty | CA certificates that verify the masters, or with `allowOnlySpecificCertificates` the master certificates that are accepted. Empty uses the system roots. |
| `chainValidation` | `true` | Require a client certificate and verify it. When `false`, no client certificate is requested. |
| `allowOnlySpecificCertificates` | `false` | Require a client certificate that is one of those in `peerCertFilePath`. Its chain is checked only when `chainValidation` is `true`. |
| `cipherList` | empty | OpenSSL-style list of TLS 1.2 cipher suites, separated by `:` or `,`. Empty uses the Go defaults. TLS 1.3 suites cannot be restricted. |
| `allowTLSv12` | `true` | Allow TLS 1.2. |
| `allowTLSv13` | `true` | Allow TLS 1.3. With both `false` the listener fails, is logged as `listen failed`, and is retried every 5 seconds. |

## Tag distribution (`protocolDestinations`)

A tag is served by adding an entry to its `protocolDestinations` array:

```js
db.realtimeData.updateOne({ tag: 'A_TAG_NAME' }, {
  $push: {
    protocolDestinations: {
      protocolDestinationConnectionNumber: 3101.0,
      protocolDestinationCommonAddress: 1.0,
      protocolDestinationObjectAddress: 'hr:100',
      protocolDestinationASDU: 'float32_cdab',
      protocolDestinationKConv1: 1.0,
      protocolDestinationKConv2: 0.0,
      protocolDestinationGroup: 0.0,
      protocolDestinationHoursShift: 0.0
    }
  }
})
```

| Field | Meaning |
|---|---|
| `protocolDestinationConnectionNumber` | The server connection. |
| `protocolDestinationCommonAddress` | Unit id the tag lives under, 0 to 255. Missing or out of range is 1. |
| `protocolDestinationObjectAddress` | `area:offset[.bN]`, such as `hr:100`, `ir:5`, `co:3`, `di:0`, `hr:200.b4`. |
| `protocolDestinationASDU` | Value type with an optional byte order. Empty means `bool` on `co`/`di` and `uint16` on `hr`/`ir`. |
| `protocolDestinationKConv1`, `protocolDestinationKConv2` | Scaling of the served value: `served = value * KConv1 + KConv2`. On `co`/`di`, `KConv1 = -1` inverts the state. |
| `protocolDestinationGroup`, `protocolDestinationHoursShift` | Not used by this driver. |

- A **supervised** tag is readable in any area.
- A **command** tag (one with `origin: "command"`, or a `protocolSourceConnectionNumber`) mapped
  to `co` or `hr` makes that address **writable**: a master's write becomes a command for the
  tag.
- A supervised tag and its command twin may share an address. A read returns the status, a write
  sends the command.
- Tags are mapped in `_id` order. When two tags claim the same address of a unit, the later one
  overwrites the served value, the first command tag receives the writes, and the overlap is
  logged at level 2. A destination whose address or ASDU cannot be parsed is skipped and logged
  at level 2.

## Serving reads

Values are encoded into in-memory register and coil banks, one set per unit id. The banks are
loaded at start, kept current from a `realtimeData` change stream, and rebuilt every 5 minutes
and whenever a tag newly gains a destination on this connection. A read never waits for MongoDB.

The change stream only follows the processed values (`cs_data_processor` writes) of the mapped
tags, plus any change of `protocolDestinations`, inserts and replaces; MongoDB filters out the
other updates before reading the document. When a rebuild changes the set of mapped tags, the
stream reopens right after its last event (logged at level 2: `rt change stream reopened for the
new tag set`).

The served value is the tag's `value` (or `valueString` for strings) scaled with
`protocolDestinationKConv1/2` only: `value` already is the engineering value, so the tag's own
`kconv1/kconv2` is not applied again. A register bit (`hr:N.bB`) sets or clears one bit of the
served register.

Functions served: FC1, FC2, FC3, FC4 (reads), FC5, FC6, FC15, FC16, FC22 (writes). Any other
function code is answered with exception 01 ILLEGAL FUNCTION. Reads of unmapped addresses return
exception 02 unless `serveUnmappedAsZero` is `true`.

## Handling writes (command relay)

A write to a mapped command address:

1. is decoded with the address's ASDU and byte order;
2. becomes a command value, applying **both** the destination scaling and the tag's scaling,
   according to the tag's `type`:
   - `digital`: forced to 1 or 0, then inverted when exactly one of
     `protocolDestinationKConv1` and `kconv1` is -1 (two -1 cancel);
   - `analog`: `(written * protocolDestinationKConv1 + protocolDestinationKConv2) * kconv1 + kconv2`;
   - `string`: the decoded text in `valueString`, `value` 0;
3. is inserted into `commandsQueue` with the tag's own `protocolSource*` routing, so whichever
   client driver owns the tag (Modbus, IEC 104, DNP3, ...) sends it to the field;
4. is answered to the master at once. Modbus cannot wait for an end-to-end confirmation.

The inserted document:

```json
{
  "protocolSourceConnectionNumber": 3001.0,
  "protocolSourceCommonAddress": 1.0,
  "protocolSourceObjectAddress": "hr:300",
  "protocolSourceASDU": "int16",
  "protocolSourceCommandDuration": 0.0,
  "protocolSourceCommandUseSBO": false,
  "pointKey": 1234.0,
  "tag": "KAW2_CB1_CMD",
  "timeTag": "2026-01-01T00:00:00Z",
  "value": 42.0,
  "valueString": "42",
  "originatorUserName": "Protocol connection: MODBUS_SRV_1",
  "originatorIpAddress": "192.168.0.50:49812"
}
```

`timeTag` is a BSON date. `protocolSourceObjectAddress` and `protocolSourceASDU` are copied as
stored on the tag, numbers included.

How each write function is mapped:

| Function | Behaviour |
|---|---|
| FC5 | The coil's command tag. |
| FC15 | Each command tag whose address is in the written range. |
| FC6 | The register's command tag. |
| FC16 | Each command tag the written range covers **completely**; a tag written only in part is skipped. |
| FC22 | A register bit command tag (`hr:N.bB`). The resulting bit is `(current AND andMask) OR (orMask AND NOT andMask)`, the current bit being the served one. |

Write outcomes:

| Situation | Answer |
|---|---|
| Relayed | Normal response |
| No command tag at the address (or FC22 to a tag that is not a register bit) | Exception 02 ILLEGAL DATA ADDRESS |
| A supervised tag, with `allowWritesToSupervised: false` | Exception 01 ILLEGAL FUNCTION |
| `commandsEnabled: false` | Exception 01 ILLEGAL FUNCTION |
| The written registers cannot be decoded as the ASDU | Exception 03 ILLEGAL DATA VALUE |
| An FC5 value other than 0x0000 or 0xFF00 | Exception 03 ILLEGAL DATA VALUE |

Refused writes are logged at level 1 with the reason. A failed `commandsQueue` insert is logged
at level 1; the master has already been answered.

## Statistics

While the node is active, every 5 seconds the driver writes the connection's `stats`:

| Field | Meaning |
|---|---|
| `status` | `listening`, `listen failed` or `stopped`. |
| `clients` | Masters connected now. |
| `reads` | Read requests served. |
| `writes` | Write requests received. |
| `exceptions` | Exception responses sent. |
| `lastError` | Text of the last listener error. |
| `nodeName`, `timeTag` | The node that wrote the stats, and when. |

## Troubleshooting

| Symptom | Check |
|---|---|
| `listen failed: ... address already in use` | Another process holds the port. The driver retries every 5 seconds. |
| `client ... rejected: not in the ipAddresses allow-list` (level 2) | Add the master's IP address to `ipAddresses`, or empty the list. |
| `client ... rejected: maxClientConnections (N) reached` (level 2) | Raise `maxClientConnections`, or find masters that do not close their connections. |
| Master gets exception 02 | The address is not mapped for that unit id. At level 3 the map is listed with the coverage of each unit. Or set `serveUnmappedAsZero`. |
| Master times out | `strictUnitId` with a unit id that is not served; `request ignored: unit id N not served` is logged at level 2. |
| TLS handshake fails | By default the server requires a client certificate signed by a CA in `peerCertFilePath`. Set `chainValidation: false` to accept masters without one. |

Request errors are logged at level 1, naming the master, the decoded request and the exception:

```
MODBUS_SRV_1: [192.168.0.50:49812] request error: unit=1 fc=3 READ_HOLDING_REGISTERS addr=100 qty=2 -> exception 0x02 ILLEGAL DATA ADDRESS
MODBUS_SRV_1: write from 192.168.0.50:49812 to KAW2_CB1_CMD refused: commands are disabled for this connection
```
