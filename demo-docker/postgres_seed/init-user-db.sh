psql -h 127.0.0.1 -U postgres -w -f create_tables.sql template1
psql -h 127.0.0.1 -U postgres -w -f metabaseappdb.sql metabaseappdb
psql -h 127.0.0.1 -U postgres -w -f grafanaappdb.sql grafanaappdb

