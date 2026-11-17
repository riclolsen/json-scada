# DOX: src/process_pg_sql — PostgreSQL SQL File Loader

## Purpose

Node.js service that loads the `pg_hist_*.sql` and `pg_rtdata_*.sql` files queued by _cs_data_processor_ in `sql/` into PostgreSQL/TimescaleDB, then deletes them. Replaces the former `sql/process_pg_*.sh`/`.bat` psql loops.

## Ownership

- process_pg_sql owns the delivery of queued SQL files to PostgreSQL (`hist` and `realtime_data` tables)
- The file format and naming belong to the writers: `src/cs_data_processor` and `src/cs_data_processor-go`
- The table DDL belongs to `sql/create_tables.sql`

## Local Contracts

- **Language:** Node.js, single file `index.js`, only dependency `pg`; no MongoDB access, no `conf/json-scada.json`
- **Arguments:** `node index.js [hist|rtdata|all] [logLevel]`; services run one process per queue (`hist`, `rtdata`)
- **Connection:** libpq `PG*` env vars with the former scripts' defaults (`127.0.0.1:5432`, database and user `json_scada`), `PGPASSFILE`/`~/.pgpass`/`%APPDATA%\postgresql\pgpass.conf` (own lookup, not pg's deprecated one), `PGSSLMODE`; `JS_PGSQL_CONNECTION_STRING` replaces them all. Other settings are `JS_PGSQL_*` env vars listed in README.md
- **Files:** `<prefix><epoch ms>[-<seq>]_<instance>.sql`, loaded oldest first, one query per file; writers create them as `*.tmp` and rename
- **Error handling:** a file is deleted only after the query succeeds. Errors without SQLSTATE and SQLSTATE classes 08, 28, 53, 57, 58, XX plus 3D000, 42P01, 42501, 40001, 40P01, 55P03, 25006 keep the file and stop the queue until the retry; any other SQLSTATE discards the file (or renames it `*.sql.failed` with `JS_PGSQL_KEEP_FAILED`)
- **Service names (kept from the scripts):** supervisor `process_pg_hist`/`process_pg_rtdata` (`pg_hist`/`pg_rtdata` in `conf-templates/supervisord.conf`), Windows `JSON_SCADA_process_hist`/`JSON_SCADA_process_rtdata`

## Work Guidance

- Keep `realtime_data` files in order: never skip ahead of a file that is kept for retry
- When the writers change the file name pattern, update `listFiles()` here and the `.gitignore` patterns
- When adding or renaming service wiring, keep in step: `platform-*/process_pg_*.ini`, `conf-templates/supervisord.conf`, `platform-windows/create_services.bat` and the upgrade block in `platform-windows/json-scada.nsi`, `Dockerfile`, `compile-docker/docker-compose.yaml`, `platform-linux/build.sh`, `platform-windows/build.bat`/`buildupd.bat`

## Verification

- `npm install` and `node -c index.js`
- Against a throwaway PostgreSQL with the `hist`/`realtime_data` tables of `sql/create_tables.sql`: copy queued files to a temp folder and run `JS_PGSQL_DIR=<folder> PGPORT=<port> PGPASSWORD=<pw> node index.js all 2`; the files must disappear and the rows appear. Check that files stay queued while the server is down or a table is missing, and that a syntax-error file is discarded
