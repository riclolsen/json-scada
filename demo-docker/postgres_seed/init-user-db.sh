#!/bin/bash
psql -U postgres -w -f /sql_data/create_tables.sql template1
psql -U postgres -w -f /sql_data/grafanaappdb.sql grafanaappdb
psql -U postgres -w -f /sql_data/metabaseappdb.sql metabaseappdb