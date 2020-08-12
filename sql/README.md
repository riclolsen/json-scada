# SQL Folder

Here the _cs_data_processor_ process will write SQL files to be passed to the PostgreSQL database.

The _process_pg_hist_ and _process_pg_rtdata_ processes must be running in backgroud to send the SQL files to the database.

In case of lost connection to the database server, SQL files will accumulate here for later upload.

Environment variables or a _.pgpass_ file should be created to allow the access to the database server.

Also certificate files may be needed if required by the database server.

The path to the PostgreSQL _psql_ client tool may also be adjusted in the scripts.

