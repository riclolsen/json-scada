# {json:scada} mongowr

One-way data replication **receiver** for air-gapped or physically isolated
networks. It listens for the packets sent by [mongofw](../mongofw/README.md) and
writes them to the local MongoDB database, maintaining the destination point
database and its real-time values.

Requires Node.js and a MongoDB deployment.

## What it receives

- Compressed JSON packets on UDP port `12345` (bound to `0.0.0.0` by default).
- Real-time changes, which are written to `realtimeData`.
- Integrity/point-list packets, which are upserted into `realtimeData` so points
  created or changed on the source appear on the destination.
- A message sequence number is tracked; gaps are logged as lost messages.

Entries are accumulated and written in bulk. If the queue grows beyond
`MAX_QUEUE`, the pending messages are discarded to protect the destination
database; the periodic integrity sync from the source repairs the point list
afterwards.

There is no reverse path: commands and controls are never sent from the
destination to the source.

## Network

The receiver must be reachable by the sender, with UDP port `UDP_PORT` open in
the one-way direction. If the sender broadcasts, both processes must be on the
same layer-2 network; otherwise set a unicast `IP_DESTINATION` on the sender and
allow that source address. Set `IP_BIND` and `UDP_PORT` in `app-defs.js` to
match the sender.

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
write access to `realtimeData` and access to `processInstances` for redundancy
control.
