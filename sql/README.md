# SQL Folder

Here the _cs_data_processor_ process writes SQL files to be passed to the PostgreSQL database: `pg_hist_*.sql` (historian, `hist` table) and `pg_rtdata_*.sql` (latest values, `realtime_data` table).

The _process_pg_hist_ and _process_pg_rtdata_ services must be running to load the SQL files into the database and delete them. Both run the Node.js loader in [src/process_pg_sql](../src/process_pg_sql/README.md), one per queue.

In case of lost connection to the database server, SQL files will accumulate here for later upload.

Environment variables (`PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD`, `PGSSLMODE`) or a _.pgpass_ file should be set up to allow the access to the database server. See the loader README for the defaults and for certificate-based connections.

This folder also holds the database creation and maintenance scripts (`create_tables.sql`, `delete_old.*`, `grafanaappdb.sql`, `metabaseappdb.sql`).
