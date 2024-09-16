#!/bin/bash
psql -h 127.0.0.1 -U postgres -w -f data/create_tables.sql template1
psql -h 127.0.0.1 -U postgres -w -f data/metabaseappdb.sql metabaseappdb 
psql -h 127.0.0.1 -U postgres -w -f data/grafanaappdb.sql grafanaappdb

