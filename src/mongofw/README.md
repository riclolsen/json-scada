# {json:scada} mongofw

One-way data replication **sender** for air-gapped or physically isolated
networks. It forwards real-time data collected on the source side to a MongoDB
instance on the destination side, without any return path. Typical deployments
cross a data diode, a tap device or a firewall that only permits UDP in one
direction.

The receiver counterpart is [mongowr](../mongowr/README.md).

Requires Node.js and a MongoDB replica set (change streams are used to capture
updates).

## What it forwards

- Changes to `realtimeData` written by protocol drivers (`sourceDataUpdate`).
  Every change is compressed and broadcast over UDP.
- A point-list integrity sync at activation and every hour. The first scan
  supplies the snapshot timestamp for the live point stream. The forwarder
  records published IDs in `replicationSourcePoints` and sends deletions for
  recorded IDs missing from the current snapshot, including points removed
  during a forwarder outage. Live inserts, tag renames and deletions also
  carry their MongoDB source revisions.
- New `secEvents` documents. The forwarder also scans the source collection
  at activation and periodically to replay events whose `timeTag` falls within
  `BACKFILL_REPLAY_INTERVAL`. The UDP packets carry the original BSON
  document, including its event ID and timestamps. Event retention runs
  independently on each side.
- A `backfillData` time series collection that stores live values for
  `BACKFILL_EXPIRATION` days (7 by default). Each replay covers the
  most recent `BACKFILL_REPLAY_INTERVAL` days (3 by default). Replayed values
  carry `isHistorical = true`.

The forwarded data covers the point database, its real-time values and
security events.

The active node claims and renews ownership through `processInstances`.
An expired heartbeat becomes eligible for takeover after 30 seconds.
Bounded queues give live traffic priority while maintaining progress on
the snapshot and backfill.

## Network

Data is sent as compressed JSON to `IP_DESTINATION:UDP_PORT`, which defaults to
a broadcast address `255.255.255.255:12345`. Set both values in `app-defs.js`;
the same port must be configured on the receiver. Packets are rate-limited and
split into smaller messages, and a message sequence number lets the receiver
detect loss. UDP is used because a data diode normally offers no acknowledgements.

## UDP authentication

Generate a 32-byte shared key in a restricted file with
`(umask 077 && openssl rand -out /absolute/path/replication.key 32)`.
Deploy the same raw key to the forwarder and writer, including their HA peers.
Restrict file access to the service accounts and use a dedicated key for each
replication deployment. Set `udpAuthKeyFile` in JSON-SCADA configuration, or
override it with `JS_MONGOFW_UDP_KEY_FILE` on the sender and
`JS_MONGOWR_UDP_KEY_FILE` on the receiver.

Upgrade both services together to enable the authenticated packet format.
Every packet carries a timestamp, a random nonce and an HMAC-SHA-256 tag
over the header and compressed payload. Backfill sends freshly signed packets.
Keep the sender, receiver and MongoDB host clocks synchronized. Both services
read the key at startup, so coordinate their restarts when rotating it.

## Configuration

Runtime parameters are constants in `app-defs.js`:

| Constant | Default | Meaning |
|---|---|---|
| `IP_DESTINATION` | `255.255.255.255` | Destination IP or broadcast address |
| `UDP_PORT` | `12345` | Destination UDP port |
| `MAX_LENGTH_JSON` | `60000` | Maximum serialized entry size in bytes, including Base64 for security events |
| `PACKET_SIZE_THRESHOLD` | `7000` | Size at which an array is split into another packet |
| `INTERVAL_INTEGRITY` | `3600` s | Full point-list sync interval |
| `BACKFILL_EXPIRATION` | `7` days | Retention of `backfillData` |
| `BACKFILL_REPLAY_INTERVAL` | `3` days | Lookback window for archived point changes and security events |

`customized_module.js` is the user-editable part; `index.js`, `load-config.js`
and `redundancy.js` are the shared framework (MongoDB connection, redundancy and
reconnection) and should not be edited.

## Process Command Line Arguments And Environment Variables

This process has the following command line arguments and equivalent environment variables.

- _**1st arg. - Instance Number**_ [Integer] - Instance number to be executed. **Optional argument, default=1**. Env. variable: **JS_MONGOFW_INSTANCE**.
- _**2nd arg. - Log. Level**_ [Integer] - Log level (0=minimum,1=basic,2=detailed,3=debug). **Optional argument, default=1**. Env. variable: **JS_MONGOFW_LOGLEVEL**.
- _**3rd arg. - Config File Path/Name**_ [String] - Path/name of the JSON-SCADA config file. **Optional argument, default="../../conf/json-scada.json"**. Env. variable: **JS_CONFIG_FILE**.

Command line args take precedence over environment variables.

## Run

    cd src/mongofw
    npm install
    node index.js 1 1 ../../conf/json-scada.json

The process must run on the source (data-producing) side. The MongoDB user needs
read and change-stream access to `realtimeData` and `secEvents`, permission to
create and read/write `backfillData` and `replicationSourcePoints`, and read/write
and index creation access to `processInstances`. The source catalog learns IDs
from published points and recent archived deletions. Reconcile any earlier
destination points whose IDs are absent from both sources during migration.
