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
- A full point-list integrity sync every hour, so points created, deleted or
  edited on the source are replayed to the destination.
- A `backfillData` time series collection that stores the forwarded changes for
  `BACKFILL_EXPIRATION` days (7 by default). The changes are replayed
  periodically so a destination that was offline catches up; replayed records
  carry `isHistorical = true`.

Commands, user actions and SOE data are not forwarded; the destination receives
the point database and its real-time values only.

The process only runs while it is the active node according to the
`processInstances` redundancy control, so it can be installed on redundant
source servers.

## Network

Data is sent as compressed JSON to `IP_DESTINATION:UDP_PORT`, which defaults to
a broadcast address `255.255.255.255:12345`. Set both values in `app-defs.js`;
the same port must be configured on the receiver. Packets are rate-limited and
split into smaller messages, and a message sequence number lets the receiver
detect loss. UDP is used because a data diode normally offers no acknowledgements.

## Configuration

Runtime parameters are constants in `app-defs.js`:

| Constant | Default | Meaning |
|---|---|---|
| `IP_DESTINATION` | `255.255.255.255` | Destination IP or broadcast address |
| `UDP_PORT` | `12345` | Destination UDP port |
| `MAX_LENGTH_JSON` | `60000` | Largest document that can be forwarded |
| `PACKET_SIZE_THRESHOLD` | `7000` | Size at which an array is split into another packet |
| `INTERVAL_INTEGRITY` | `3600` s | Full point-list sync interval |
| `BACKFILL_EXPIRATION` | `7` days | Retention of `backfillData` |
| `BACKFILL_REPLAY_INTERVAL` | `3` days | Automatic backfill replay interval |

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
read access to `realtimeData`, `backfillData` and `processInstances`.
