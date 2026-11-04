# MODBUS Client Driver (Go)

Polls Modbus devices, updates {json:scada} tags and sends commands from the `commandsQueue`
to the devices. It registers as protocol driver **`MODBUS`**, the same name as the Node.js
driver it replaces, and reads the same configuration documents.

The binary is `bin/modbus-client` (`bin\modbus-client.exe` on Windows). The service keeps its
old name: `modbusclient` under Supervisor and `JSON_SCADA_modbusclient` on Windows.

Transports, value types, byte orders, addressing and logging levels are shared with the
server driver and are described in the [module README](../../README.md).

## Running

```
modbus-client [instance] [logLevel] [configFile]
```

| Argument | Default | Environment override |
|---|---|---|
| instance number | `1` | `JS_MODBUS_INSTANCE` |
| log level, 0 to 3 | `1` | `JS_MODBUS_LOGLEVEL` |
| config file | the first that exists of `../conf/json-scada.json`, `../../conf/json-scada.json`, `c:/json-scada/conf/json-scada.json` | `JS_MODBUS_CONFIG_FILE`, then `JS_CONFIG_FILE` |

A command-line argument takes precedence over its environment variable.

At startup the driver:

1. reads the config file and connects to MongoDB, retrying until it succeeds;
2. creates the `protocolDriverInstances` document for `MODBUS` and this instance number when
   there is none;
3. exits when that instance has `enabled: false`, and keeps checking every 30 seconds;
4. loads the `protocolConnections` with `protocolDriver: "MODBUS"`, this instance number and
   `enabled: true`. When there are none it logs
   `No enabled connections found for driver MODBUS instance N. Exiting.` and exits with code 0.

## Connection configuration (`protocolConnections`)

```js
use json_scada_db_name
db.protocolConnections.insertOne({
  protocolDriver: 'MODBUS',
  protocolDriverInstanceNumber: 1.0,
  protocolConnectionNumber: 3001.0,   // unique across the whole system
  name: 'PLC_AREA_51',
  description: 'Modbus TCP PLC',
  enabled: true,
  commandsEnabled: true,

  connectionMode: 'TCP Active',        // TCP Active | TLS Active | Serial | RTU over TCP | RTU over TLS
  ipAddresses: ['192.168.0.101:502', '192.168.0.102:502'],
  ipAddressLocalBind: '',
  portName: 'COM3',                    // serial only (Linux: /dev/ttyUSB0)
  baudRate: 9600.0, parity: 'Even', stopBits: 'One', handshake: 'None',

  timeoutMs: 1000.0,
  maxRetries: 2.0,
  pollingInterval: 1000.0,
  giInterval: 300.0,
  interRequestDelayMs: 0.0,
  interFrameDelayMs: 0.0,

  maxReadRegisters: 125.0,
  maxReadCoils: 2000.0,
  maxAddressGap: 8.0,

  byteOrder16: 'AB', byteOrder32: 'ABCD', byteOrder64: 'ABCDEFGH', byteOrderStr: 'AB',
  stringEncoding: 'latin1',
  useModiconAddresses: false,
  useMaskWrite: true,

  localCertFilePath: '', privateKeyFilePath: '', privateKeyPassphrase: '',
  peerCertFilePath: '', chainValidation: true, allowOnlySpecificCertificates: false,
  cipherList: '', allowTLSv12: true, allowTLSv13: true,

  autoCreateTags: false,
  topics: ['PLC1_FLOW|hr:4:float32_cdab', 'PLC1_STAT|hr:20:uint16[10]'],

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
| `protocolConnectionNumber` | `0` | Connection number; tags point to it with `protocolSourceConnectionNumber`. |
| `name` | `MODBUS-<number>` | Prefix of every log line of the connection. |
| `enabled` | `true` | Only enabled connections are loaded. |
| `commandsEnabled` | `true` | When `false`, commands for the connection are ignored and not acknowledged. |
| `keepProtocolRunningWhileInactive` | `false` | When `true`, the connection keeps polling on the standby node of a redundant pair. Commands are still executed only on the active node. |

### Transport

| Key | Default | Meaning |
|---|---|---|
| `connectionMode` | `TCP Active` | `TCP Active`, `TLS Active`, `Serial`, `RTU over TCP` or `RTU over TLS`. An unknown value is logged and treated as `TCP Active`. |
| `ipAddresses` | `[]` (`127.0.0.1`) | Addresses of **one** device or gateway, as `host`, `host:port`, `[v6]` or `[v6]:port`. A missing port is 502, or 802 in the TLS modes. After a failure the driver moves to the next address. |
| `ipAddressLocalBind` | empty | Local address (`ip` or `ip:port`) for the outgoing socket. Empty lets the system choose. |
| `portName` | empty | Serial port, such as `COM3` or `/dev/ttyUSB0`. |
| `baudRate` | `9600` | Serial speed. In `RTU over TCP`/`RTU over TLS` it sets the RTU frame timing. |
| `parity` | `Even` | `None`, `Even` or `Odd`. `Mark` and `Space` are not supported: they are logged and treated as `None`. |
| `stopBits` | `One` | `One` or `Two`. `One5` is treated as `Two`. |
| `handshake` | `None` | Flow control is not supported. Any other value is logged and ignored. |

The serial port always uses 8 data bits.

### Timing and polling

| Key | Default | Meaning |
|---|---|---|
| `timeoutMs` | `1000` | Response timeout of one request. |
| `maxRetries` | `2` | Retries of a request that timed out. A request is tried at most `maxRetries + 1` times. |
| `pollingInterval` | `1000` | Scan cycle in milliseconds, measured from the start of one cycle to the start of the next. |
| `giInterval` | `300` | Integrity cycle in seconds (see [Polling](#polling)). |
| `interRequestDelayMs` | `0` | Pause after each request, for slow lines and gateways. |
| `interFrameDelayMs` | `0` | Not used by this driver: the RTU frame timing comes from `baudRate`. A non-zero value is logged at level 2. |

### Read planning

| Key | Default | Meaning |
|---|---|---|
| `maxReadRegisters` | `125` | Largest register read. Values above 125 are reduced to 125. |
| `maxReadCoils` | `2000` | Largest coil or discrete-input read. Values above 2000 are reduced to 2000. |
| `maxAddressGap` | `8` | Two tags of the same unit and area are read in one request when no more than this many unused addresses separate them. |

### Value encoding

| Key | Default | Meaning |
|---|---|---|
| `byteOrder16` | `AB` | Byte order of 16-bit values whose ASDU names none. |
| `byteOrder32` | `ABCD` | Byte order of 32-bit values. |
| `byteOrder64` | `ABCDEFGH` | Byte order of 64-bit values. |
| `byteOrderStr` | `AB` | Byte order inside each register of a string: `AB` or `BA`. |
| `stringEncoding` | `latin1` | `latin1`, `utf8` or `ascii`. |
| `useModiconAddresses` | `false` | Accept plain 1-based Modicon numbers (`40001`) as object addresses. |
| `useMaskWrite` | `true` | Write a register bit with FC22. When `false`, read the register (FC3) and write it back (FC6). |

### TLS

Used in `TLS Active` and `RTU over TLS`.

| Key | Default | Meaning |
|---|---|---|
| `localCertFilePath` | empty | Client certificate (PEM), sent to the server. Required by servers that verify clients. |
| `privateKeyFilePath` | empty | Private key of that certificate (PEM: PKCS#1, PKCS#8 or EC). When empty, the key is read from the certificate file. |
| `privateKeyPassphrase` | empty | Passphrase of an encrypted key. |
| `peerCertFilePath` | empty | CA certificates that verify the server, or with `allowOnlySpecificCertificates` the server certificates that are accepted. Empty uses the system roots. |
| `chainValidation` | `true` | Verify the server certificate and its host name. When `false`, any server certificate is accepted. |
| `allowOnlySpecificCertificates` | `false` | Accept only a server whose certificate is one of those in `peerCertFilePath`. The host name is not checked, and the chain is checked only when `chainValidation` is `true`. |
| `cipherList` | empty | OpenSSL-style list of TLS 1.2 cipher suites, separated by `:` or `,`. Empty uses the Go defaults. TLS 1.3 suites cannot be restricted. |
| `allowTLSv12` | `true` | Allow TLS 1.2. |
| `allowTLSv13` | `true` | Allow TLS 1.3. With both `false` the connection fails. |

### Automatic tag creation

| Key | Default | Meaning |
|---|---|---|
| `autoCreateTags` | `false` | Create the tags named in `topics` that do not exist yet, at startup. |
| `topics` | `[]` | Tag descriptors, see [Automatic tag creation](#automatic-tag-creation-1). |

## Tag configuration (`realtimeData`)

```js
db.realtimeData.updateOne({ tag: 'A_TAG_NAME' }, {
  $set: {
    protocolSourceConnectionNumber: 3001.0,
    protocolSourceCommonAddress: 1.0,
    protocolSourceObjectAddress: 'hr:4',
    protocolSourceASDU: 'float32_cdab',
    protocolSourceCommandDuration: 0.0,
    protocolSourceCommandUseSBO: false,
    kconv1: 1.0,
    kconv2: 0.0
  }
})
```

| Field | Meaning |
|---|---|
| `protocolSourceConnectionNumber` | The connection that reads the tag. |
| `protocolSourceCommonAddress` | Modbus unit id, 0 to 255. Missing or out of range is 1. Unit 0 is broadcast and is valid only for commands. |
| `protocolSourceObjectAddress` | `area:offset[.bN]`, such as `hr:4`, `ir:20`, `co:5`, `di:3`, `hr:100.b7`. |
| `protocolSourceASDU` | Value type with an optional byte order: `uint16`, `int32_le`, `float32_cdab`, `string:16`. Empty means `bool` on `co`/`di` and `uint16` on `hr`/`ir`. |
| `kconv1`, `kconv2` | Scaling: `value = raw * kconv1 + kconv2`. On `co`/`di`, `kconv1 = -1` inverts the state. |

Tags with `origin: "command"` are not read. A tag whose address or ASDU cannot be parsed is
skipped and logged at level 2. Tag bindings are reloaded every 60 seconds while the node is
active, so new and changed tags are picked up without a restart.

## Polling

Tags are grouped by unit id and area and sorted by address. Neighbouring tags become one read
request when the gap between them is at most `maxAddressGap` and the request stays within
`maxReadRegisters` or `maxReadCoils`. Each block is read with FC1, FC2, FC3 or FC4.

Each value is written to the tag's `sourceDataUpdate`:

| Field | Value |
|---|---|
| `valueAtSource` | The scaled number. Strings write 0, booleans write 0 or 1. |
| `valueStringAtSource` | The value as text. |
| `valueJsonAtSource` | For `bitstring16`, the 16 bits, least significant first; otherwise `null`. |
| `asduAtSource` | The value type and byte order, such as `float32_cdab`. |
| `causeOfTransmissionAtSource` | `"20"` on an integrity cycle, `"3"` otherwise. |
| `invalidAtSource` | `true` for a float NaN or infinity, or a BCD digit above 9. |
| `timeTag` | Time of the read. |
| `timeTagAtSource`, `timeTagAtSourceOk` | `null` and `false`: Modbus carries no timestamps. |
| `transientAtSource` | `false`. |
| `originator` | `MODBUS\|<connection number>`. |

A block whose content did not change since the last read is not written again. Every
`giInterval` seconds, and on the first cycle after a connection, the integrity cycle writes all
values whatever they are.

A block that fails (timeout, exception, transport error) marks its tags invalid
(`sourceDataUpdate.invalidAtSource: true`) once, and they become valid again with the next good
read. An exception does not drop the connection: the device answered.

## Connection handling

The driver connects to the first address in `ipAddresses`. The link is considered down when it
cannot connect, or when a whole poll cycle gets no answer and at least one transport error. It
then:

- marks every tag of the connection invalid, once per outage;
- moves to the next address in `ipAddresses`;
- waits before the next attempt: 1 second, doubling up to 30 seconds, and back to 1 second after
  a successful connection.

When a node becomes standby in a redundant pair, connections stop unless
`keepProtocolRunningWhileInactive` is `true`.

## Commands

Inserts into `commandsQueue` with this connection's `protocolSourceConnectionNumber` are sent
to the device while the connection runs. They are executed only when:

- `commandsEnabled` is `true` (otherwise the command is ignored);
- this node is the active one of a redundant pair (otherwise the command is ignored, and the
  active node executes it);
- the command's `timeTag` is less than 10 seconds old (otherwise `command expired`);
- the link is up (otherwise `connection down`).

The value is sent with the function the address and type call for:

| Target | Function |
|---|---|
| `co` area, or ASDU `bool` | FC5 Write Single Coil. `kconv1 = -1` inverts. |
| Register bit `hr:N.bB` | FC22 Mask Write Register, or with `useMaskWrite: false` FC3 then FC6. |
| One 16-bit register (`uint16`, `int16`, `bcd16`, `bitstring16`) | FC6 Write Single Register |
| Multi-register values and strings | FC16 Write Multiple Registers |

The scaling is inverted: `raw = (value - kconv2) / kconv1`. The `kconv1` and `kconv2` come from
the supervised tag whose `_id` is the command's `pointKey`, then from the command document, then
default to 1 and 0. A string command sends `valueString`.

The driver updates the command document with `delivered: true`, `ack`, `ackTimeTag` and
`resultDescription`:

| `resultDescription` | `ack` |
|---|---|
| `OK` | `true` |
| `OK (broadcast, no confirmation)` (unit 0) | `true` |
| `response timeout` | `false` |
| `Modbus exception 02 ILLEGAL DATA ADDRESS (FC 6)` and the like | `false` |
| an encoding error, such as a value out of range for the type | `false` |
| `command expired`, `connection down` | `false` |

Modbus has no end-to-end confirmation: `ack: true` means the device answered the write.

A broadcast (unit 0) is not answered by the devices. The driver waits 100 ms (or `timeoutMs`
when shorter) and acknowledges it. A register bit write to unit 0 always uses FC22, because the
register cannot be read back first.

Commands go ahead of the poll queue: a command waits only for the request in progress.

## Automatic tag creation

With `autoCreateTags: true`, each entry of `topics` describes tags to create when no tag of that
name exists:

```
TAG_NAME|<area>:<offset>:<type[_byteorder]>[count]
```

- `PLC1_FLOW|hr:4:float32_cdab` creates `PLC1_FLOW` at `hr:4`.
- `PLC1_STAT|hr:20:uint16[10]` creates `PLC1_STAT[0]` to `PLC1_STAT[9]` at `hr:20` to `hr:29`.
  For multi-register types the addresses step by the register count.
- `PLC1_FLOW|hr:4:float32|cdab` is the same as `PLC1_FLOW|hr:4:float32_cdab`.

New tags get the next free `_id` (the highest `_id` plus one), unit id 1, `origin: "supervised"`,
`invalid: true`, `group1` set to the connection name, `description` `<connection>~<tag>`, and type
`digital` (bit areas and `bool`), `string`, or `analog`. Digital tags are events. A descriptor
that cannot be parsed is logged at level 1 and skipped.

## Statistics

While the node is active, every 5 seconds the driver writes the connection's `stats`:

| Field | Meaning |
|---|---|
| `status` | `connecting`, `connected`, `disconnected` or `stopped`. |
| `connectedTo` | `tcp://host:port`, `tls://host:port` or `serial://port@baud`. |
| `requests` | Block reads sent. |
| `errors` | Transport errors. |
| `timeouts` | Timed-out attempts, each retry included. |
| `exceptions` | Exception responses. |
| `lastError` | Text of the last failure. |
| `giCount` | Integrity cycles run. |
| `nodeName`, `timeTag` | The node that wrote the stats, and when. |

## Troubleshooting

| Symptom | Check |
|---|---|
| `connect failed (tcp://...)` repeating | Address, port and firewall. The wait between attempts grows to 30 seconds. |
| All tags invalid, `read failed ... ILLEGAL DATA ADDRESS` | The device does not have those addresses. Check `0`-based offsets against the device map, or set `useModiconAddresses`. Reduce `maxAddressGap` when the gap between tags crosses addresses the device refuses. |
| Values look swapped or huge | The byte order. Try `_cdab` (word swap) or `_badc` (byte swap) on the ASDU, or set `byteOrder32`. |
| Timeouts on a serial line | `baudRate`, `parity`, `stopBits` and the unit id. Raise `timeoutMs` or `interRequestDelayMs` for slow devices. |
| TLS handshake fails | Run at log level 3. Check that `peerCertFilePath` holds the CA that signed the server certificate, and that the server name matches the certificate, or use `allowOnlySpecificCertificates`. |
| Commands never acknowledged | `commandsEnabled`, and whether this node is the active one. |

At log level 3 every request is traced with its decoded PDU and the response hex:

```
PLC_AREA_51: TX unit=1 fc=3 READ_HOLDING_REGISTERS addr=0 qty=2 | PDU: 03 00 00 00 02
PLC_AREA_51: RX unit=1 fc=3 READ_HOLDING_REGISTERS bytes=4 | PDU: 03 04 04 D2 00 00
PLC_AREA_51:   PLC1_FLOW [hr:0 float32_cdab] = 12.34
```
