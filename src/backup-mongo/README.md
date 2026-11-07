# {json:scada} backup-mongo

Command-line tool that connects to the JSON-SCADA MongoDB database and writes
mongo shell scripts that can recreate or refresh the configuration data later.

It is not a full database backup: it only exports the collections needed to
rebuild an installation (users, point database and protocol connections), and it
does not export historical or SOE data. Use `mongodump`/`mongorestore` (or the
MongoDB backup mechanism of your deployment) for full backups.

## Output

The scripts are written to an `output/` directory created in the current working
directory:

| File | Content |
|---|---|
| `users_insert.js` | `insertOne` for every document in `users` |
| `realtimeData_insert.js` | `insertOne` for every point in `realtimeData` |
| `realtimeData_update.js` | `updateOne` by `tag`, setting only operator-editable fields (`hiLimit`/`hihiLimit`/`hihihiLimit`, `loLimit`/`loloLimit`/`lololoLimit`, `hysteresis`, `substituted`, `alarmDisabled`, `annotation`, `notes`, `commandBlocked`) |
| `protocolConnections_insert.js` | `insertOne` for every document in `protocolConnections` |

The `*_insert.js` scripts recreate documents as they are, including stored
password hashes. Handle the output as sensitive data.

## Usage

    cd src/backup-mongo
    npm install
    node backup-mongo.js [config file name]

With no argument the tool reads `../../conf/json-scada.json`; the alternative
config path can also be set with the `JS_CONFIG_FILE` environment variable. The
tool accepts no other arguments. On a large point database the whole
`realtimeData` collection is loaded into memory, so run it during low activity.

Restore a generated script with the mongo shell (connect to the target
database):

    mongosh "mongodb://localhost:27017/json_scada" output/users_insert.js
    mongosh "mongodb://localhost:27017/json_scada" output/realtimeData_insert.js
    mongosh "mongodb://localhost:27017/json_scada" output/protocolConnections_insert.js

The `realtimeData_update.js` script is useful to reapply operator changes
(limits, annotations, inhibits) without overwriting the rest of the point
definition.
