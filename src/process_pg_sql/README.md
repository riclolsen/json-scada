# process_pg_sql - PostgreSQL SQL File Loader

Node.js service that loads into PostgreSQL/TimescaleDB the SQL files queued by
_cs_data_processor_ (Node.js or Go) in the `sql/` folder:

* `pg_hist_*.sql` - inserts into the `hist` table (historian).
* `pg_rtdata_*.sql` - upserts into the `realtime_data` table (latest value of each tag).

It replaces the former `sql/process_pg_hist.sh`/`.bat` and `sql/process_pg_rtdata.sh`/`.bat`
scripts, which launched `psql` once per file. The service keeps one connection per queue
and runs each file as a single query.

## How it works

* Scans the SQL folder every `JS_PGSQL_POLL_INTERVAL_MS` (1 s) and loads the files of its
  queue oldest first (by the epoch milliseconds in the file name), so newer `realtime_data`
  values always win.
* A file is deleted only after the database accepted it.
* Connection failures, a missing database/table, missing grants, shutdowns, deadlocks and
  similar transient errors keep the file and stop the queue, which is retried after
  `JS_PGSQL_RETRY_INTERVAL_MS` (5 s). While PostgreSQL is unreachable, files accumulate in
  the folder and are loaded in order when it comes back.
* A file rejected for its content (syntax or data error) is discarded and logged, as `psql`
  did, so it cannot block the queue. Set `JS_PGSQL_KEEP_FAILED=1` to rename it to
  `*.sql.failed` instead.
* Files modified less than `JS_PGSQL_MIN_FILE_AGE_MS` (500 ms) ago are left for the next
  scan. Both _cs_data_processor_ versions write to a `.tmp` name and rename it, so a partial
  file is never visible; the age check covers other writers.
* Every `JS_PGSQL_STATS_INTERVAL_MS` (60 s) it logs the files and bytes loaded, rejected
  files and the backlog of each queue.

## Running

    cd json-scada/src/process_pg_sql
    npm install
    node index.js [hist|rtdata|all] [logLevel]

* `hist`, `rtdata`: load only that queue. The platform service definitions run one process
  per queue (`process_pg_hist`, `process_pg_rtdata`), so a large historian backlog never
  delays realtime updates and the logs stay separate.
* `all` (default): both queues in one process, each with its own connection.
* `logLevel`: 0 = errors and connection events, 1 = normal (statistics), 2 = detailed,
  3 = debug (every file).

Stop it with SIGINT/SIGTERM: the file being loaded is finished (or rolled back) first.

Run a single instance of each queue per SQL folder.

## Database connection

Uses the standard libpq environment variables, with the defaults of the former scripts:

| Variable | Default |
|---|---|
| `PGHOST` (or `PGHOSTADDR`) | `127.0.0.1` |
| `PGPORT` | `5432` |
| `PGDATABASE` | `json_scada` |
| `PGUSER` | `json_scada` |
| `PGPASSWORD` | none: looked up in the password file when the server asks for one |
| `PGPASSFILE` | `~/.pgpass` (Linux), `%APPDATA%\postgresql\pgpass.conf` (Windows) |
| `PGSSLMODE` | `disable`, `require`, `verify-ca`, `verify-full` or `no-verify` (node-postgres verifies the server certificate for every mode except `no-verify`) |

For TLS with certificates, or any option not covered above, set
`JS_PGSQL_CONNECTION_STRING` (for example
`postgresql://json_scada@dbhost:5432/json_scada?sslmode=verify-full&sslrootcert=/path/ca.crt&sslcert=/path/client.crt&sslkey=/path/client.key`).
It replaces all the `PG*` settings above.

## Other environment variables

| Variable | Default | Meaning |
|---|---|---|
| `JS_PGSQL_QUEUE` | `all` | Queue when no command line argument is given |
| `JS_PGSQL_LOGLEVEL` | `1` | Log level when no command line argument is given |
| `JS_PGSQL_DIR` | `../../sql` from this folder | Folder of the SQL files (same search as _cs_data_processor_) |
| `JS_PGSQL_POLL_INTERVAL_MS` | `1000` | Wait between scans of an idle queue |
| `JS_PGSQL_RETRY_INTERVAL_MS` | `5000` | Wait after a connection or transient database error |
| `JS_PGSQL_MIN_FILE_AGE_MS` | `500` | Minimum age of a file before it is loaded |
| `JS_PGSQL_STATS_INTERVAL_MS` | `60000` | Interval of the statistics log line |
| `JS_PGSQL_CONNECT_TIMEOUT_MS` | `10000` | Connection timeout |
| `JS_PGSQL_KEEP_FAILED` | off | `1`/`true`: rename rejected files to `*.sql.failed` instead of deleting them |

## Service definitions

* Linux (supervisor): `platform-*/process_pg_hist.ini` and `process_pg_rtdata.ini`,
  logs in `log/process_pg_hist.log` and `log/process_pg_rtdata.log`.
* Windows (NSSM): services `JSON_SCADA_process_hist` and `JSON_SCADA_process_rtdata` in
  `platform-windows/create_services.bat`. The installer repoints existing services from the
  old batch files on upgrade.
* Docker: the image installs the Ubuntu 24.04 `.ini` files.
