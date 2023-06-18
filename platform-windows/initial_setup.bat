@echo off
if not "%1"=="am_admin" (powershell start -verb runas '%0' am_admin & exit /b)

cd \json-scada\platform-windows\

ping -n 2 127.0.0.1
call postgresql-initdb.bat
ping -n 15 127.0.0.1
copy /Y ..\conf-templates\pg_hba.conf postgresql-data\
copy /Y ..\conf-templates\postgresql.conf postgresql-data\
call postgresql-create_service.bat
ping -n 2 127.0.0.1
call postgresql-start.bat
ping -n 18 127.0.0.1
postgresql-runtime\bin\psql -U postgres -h localhost -f ..\sql\create_tables.sql template1

call create_services.bat
ping -n 8 127.0.0.1
call mongodb-start.bat
ping -n 8 127.0.0.1
mongodb-runtime\bin\mongo json_scada < ..\mongo_seed_demo\a_rs-init.js
mongodb-runtime\bin\mongo json_scada < ..\mongo_seed_demo\b_create-db.js
mongodb-runtime\bin\mongoimport --db json_scada --collection protocolDriverInstances --type json --file ..\mongo_seed_demo\demo_instances.json 
mongodb-runtime\bin\mongoimport --db json_scada --collection protocolConnections --type json --file ..\mongo_seed_demo\demo_connections.json 
mongodb-runtime\bin\mongoimport --db json_scada --collection realtimeData --type json --file ..\mongo_seed_demo\demo_data.json 
mongodb-runtime\bin\mongoimport --db json_scada --collection processInstances --type json --file ..\mongo_seed_demo\demo_process_instances.json 
mongodb-runtime\bin\mongoimport --db json_scada --collection users --type json --file ..\mongo_seed_demo\demo_users.json 
mongodb-runtime\bin\mongoimport --db json_scada --collection roles --type json --file ..\mongo_seed_demo\demo_roles.json 
mongodb-runtime\bin\mongo json_scada --eval "db.realtimeData.updateMany({_id:{$gt:0}},{$set:{dbId:'demo'}})"
