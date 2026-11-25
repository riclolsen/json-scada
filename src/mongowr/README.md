# {json:scada} mongowr

One-way data replication **receiver** for air-gapped or physically isolated
networks. It listens for the packets sent by [mongofw](../mongofw/README.md) and
writes them to the local MongoDB database, maintaining the destination point
database, real-time values and security events.

Requires Node.js and a MongoDB deployment.

## What it receives

- Compressed JSON packets on UDP port `12345` (bound to `0.0.0.0` by default).
- Real-time changes, which are written to `realtimeData`.
- Integrity/point-list packets, which are upserted into `realtimeData` so points
  created or changed on the source appear on the destination. The first
  matching integrity packet records the source ID in `replicatedSourceId`.
  A point matched by tag keeps its local `_id`, including through later renames.
- Explicit deletion records for points with a recorded source ID. The writer
  keeps source revisions, including deletion revisions, in
  `replicationPointRevisions` and rejects older writes and deletions. A live
  value arriving before its initial point list is held until the point exists.
- Security events, inserted into `secEvents` with their original BSON types
  and IDs. Repeated packets and backfill leave an existing event intact.
- A message sequence number is tracked; gaps are logged as lost messages.

Entries in a batch are written in arrival order. The writer retries the
unacknowledged operations after a transient MongoDB failure. It logs a
permanent per-operation error and retries the subsequent operations. If
incoming packets fill `MAX_QUEUE`, the writer discards the queued packets
and keeps the batch awaiting a database result. The next integrity sync can
refresh points and the replay window can recover recent values, deletions and
security events.

There is no reverse path: commands and controls are never sent from the
destination to the source.

## Network

The receiver must be reachable by the sender, with UDP port `UDP_PORT` open in
the one-way direction. If the sender broadcasts, both processes must be on the
same layer-2 network; otherwise set a unicast `IP_DESTINATION` on the sender and
allow that source address. Set `IP_BIND` and `UDP_PORT` in `app-defs.js` to
match the sender.

## UDP authentication

Configure the shared 32-byte key in `udpAuthKeyFile` or
`JS_MONGOWR_UDP_KEY_FILE`. See the [forwarder setup](../mongofw/README.md#udp-authentication).
The writer checks HMAC-SHA-256 before queuing or decompressing packets. It
accepts authenticated packets less than 60 seconds old and at most 5 seconds
ahead of its clock. Keep the sender, receiver and MongoDB host clocks synchronized.
Upgrade both services together so point packets include the source revision
required for ordering.

The writer commits authenticated payloads and their nonces to
`replicationUdpReceipts` before applying updates. On restart or HA takeover,
the active writer recovers unfinished payloads from the same database.
After the payload writes are acknowledged with majority write concern, the
writer removes the stored payload and gives the receipt a TTL. The destination
account needs read, write and index creation access for this collection.

## Configuration

Runtime parameters are constants in `app-defs.js`:

| Constant | Default | Meaning |
|---|---|---|
| `IP_BIND` | `0.0.0.0` | Interface to bind |
| `UDP_PORT` | `12345` | UDP port to receive on |
| `MAX_QUEUE` | `5000` | Maximum queued packets before the queue is cleared |
| `MAX_MSG_SEQ` | `50` | Messages accumulated per bulk write |
| `INTERVAL_AFTER_WRITE` | `100` ms | Pause after a bulk write or empty queue |

`customized_module.js` is the user-editable part; `index.js`, `load-config.js`
and `redundancy.js` are the shared framework and should not be edited.

## Process Command Line Arguments And Environment Variables

This process has the following command line arguments and equivalent environment variables.

- _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**. Env. variable: **JS_MONGOWR_INSTANCE**.
- _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**. Env. variable: **JS_MONGOWR_LOGLEVEL**.
- _**3rd arg. - Config File Path/Name**_ [String] - Path/name of the JSON-SCADA config file. **Optional argument, default="../../conf/json-scada.json"**. Env. variable: **JS_CONFIG_FILE**.

Command line args take precedence over environment variables.

## Run

    cd src/mongowr
    npm install
    node index.js 1 1 ../../conf/json-scada.json

The process must run on the destination (receiving) side. The MongoDB user needs
read/write access to `realtimeData`, `secEvents`, `replicationPointRevisions` and
`processInstances`, and index creation access to `realtimeData` and
`processInstances`. The HA takeover timeout is 30 seconds.
