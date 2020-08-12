# SQL Folder

Here the _cs_data_processor_ process will write SQL files to be passed to the PostgreSQL database.

The _process_pg_hist_ and _process_pg_rtdata_ must be running in backgroud to send the SQL files to the database.

In case of lost connection to the database server, SQL files will accumulate here for later upload.

A _.pgpass_ file may be created to allow the access to the database.

Also certificate files may be needed if required by the database server.


