#!/bin/sh

# process sql files in a postgresql database then removes the files

# set defaults
psqlPath=${PSQL_PATH:-"/usr/bin"}
dbHost=${PGHOST:-"127.0.0.1"}
dbPort=${PGPORT:-5432}
dbName=${PGDATABASE:-"json_scada"}
dbUser=${PGUSER:-"json_scada"}
# PGPASSWORD=${PGPASSWORD :-""}

# cd ../sql

# exit when flock fails
set -e

(
# locks to avoid multiples of this process running
# flock -n -x 10 /sql/process_pg_hist.exclusivelock

# avoids exit in case of errors
set +e

while [ 1 ]; do

# gets all sql hist files
  for file in pg_hist_*.sql; do
  if [ "$file" != "pg_hist_*.sql" ]; then

# process sql file into the database
    res=`$psqlPath/psql -h $dbHost -U "$dbUser" -d $dbName -p $dbPort < "$file" `

    if [ "$?" = "0" ]; then

# if ok, deletes the sql file
      rm -f "$file"

    fi

  fi
  done

  sleep 2
  echo "$file"

done

) 9>process_pg_hist.exclusivelock
