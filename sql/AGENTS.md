# DOX: sql — SQL Scripts

## Purpose

SQL scripts for PostgreSQL/TimescaleDB database setup and maintenance (table creation, historical data archival, Grafana/Metabase app database schemas), and the queue folder of SQL files loaded by `src/process_pg_sql`.

## Ownership

- sql owns all PostgreSQL DDL and DML scripts
- Scripts are referenced by platform installation procedures

## Local Contracts

- Files: SQL scripts (`.sql`), shell scripts (`.sh`)
- `create_tables.sql` — main schema creation script
- `delete_old.*` — old data cleanup/archival
- Runtime queue (gitignored): `pg_hist_*.sql` / `pg_rtdata_*.sql` written by _cs_data_processor_ (as `*.tmp`, then renamed) and loaded/deleted by [src/process_pg_sql](../src/process_pg_sql/AGENTS.md); the folder must be writable by the service user
- `grafanaappdb.sql` — Grafana application database schema
- `metabaseappdb.sql` — Metabase application database schema

## Work Guidance

- Maintain compatibility with PostgreSQL 16+ and TimescaleDB 2.0+
- Always test schema changes with both `psql` and application code
- Document table structures and relationships

## Verification

- Validate `.sql` files with `psql --file <script>`
- Test on PostgreSQL 16+ with TimescaleDB extension
