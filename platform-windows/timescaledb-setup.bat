
call postgresql-stop.bat

SET PATH=%PATH%;c:\json-scada\postgresql-runtime\bin\

cd timescaledb
setup.exe
cd ..

echo Create json_scada database
psql -h 127.0.0.1 -U json_scada -W --command="create database json_scada template=template0 encoding=utf8;" postgres

echo Create TimescaleDB extension for the database
psql -h 127.0.0.1 -U json_scada -W --command="CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;" json_scada 

echo Create tables 
psql -h 127.0.0.1 -U json_scada -W -f sql\create_tables.sql json_scada 
