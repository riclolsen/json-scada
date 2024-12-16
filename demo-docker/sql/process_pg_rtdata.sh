#!/bin/sh

# process sql files in a postgresql database then removes the files

# set defaults
psqlPath=${PSQL_PATH:-"/usr/bin"}
dbHost=${PGHOST:-"127.0.0.1"}
dbPort=${PGPORT:-5432}
dbName=${PGDATABASE:-"json_scada"}
dbUser=${PGUSER:-"json_scada"}
# PGPASSWORD=${PGPASSWORD :-""}

cd /sql

# exit when flock fails
set -e

(
# locks to avoid multiples of this process running
# flock -n -x 10 9

# avoids exit in case of errors
set +e

while [ 1 ]; do

# gets all sql rtdata files
  for file in pg_rtdata_*.sql; do
  if [ "$file" != "pg_rtdata_*.sql" ]; then

# process sql file into the database
    res=`psql -h $dbHost -U "$dbUser" -d $dbName -p $dbPort < "$file" `

    if [ "$?" = "0" ]; then

# if ok, deletes the sql file
      rm -f "$file"
    fi

  fi
  done

  sleep 2
  echo "$file"

done

) 9>/sql/process_pg_rtdata.exclusivelock
