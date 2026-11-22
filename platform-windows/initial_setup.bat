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
postgresql-runtime\bin\psql -U postgres -h localhost -f ..\sql\metabaseappdb.sql metabaseappdb
postgresql-runtime\bin\psql -U postgres -h localhost -f ..\sql\grafanaappdb.sql grafanaappdb

rem already elevated: am_admin runs the scripts synchronously (without it they relaunch
rem themselves in a new window and return at once, before the services exist)
call create_services.bat am_admin
cd /d \json-scada\platform-windows\
call mongodb-start.bat am_admin
rem wait until MongoDB accepts connections (up to 180 s) before seeding
powershell -NoProfile -ExecutionPolicy Bypass -Command "$d=(Get-Date).AddSeconds(180); while((Get-Date) -lt $d){ $c=New-Object Net.Sockets.TcpClient; try{ $c.Connect('127.0.0.1',27017); exit 0 }catch{ Start-Sleep 2 }finally{ $c.Close() } }; Write-Host 'MongoDB is not accepting connections on 127.0.0.1:27017'; exit 1"
if errorlevel 1 (
  echo MongoDB did not start: check the JSON_SCADA_mongodb service and c:\json-scada\log\mongodb.err.log
  sc query JSON_SCADA_mongodb
)
ping -n 3 127.0.0.1
mongodb-runtime\bin\mongo json_scada < ..\mongo_seed\a_rs-init.js
ping -n 5 127.0.0.1
mongodb-runtime\bin\mongo json_scada < ..\mongo_seed\b_create-db.js
mongodb-runtime\bin\mongoimport --db json_scada --collection protocolDriverInstances --type json --file ..\mongo_seed_demo\demo_instances.json 
mongodb-runtime\bin\mongoimport --db json_scada --collection protocolConnections --type json --file ..\mongo_seed_demo\demo_connections.json 
mongodb-runtime\bin\mongoimport --db json_scada --collection realtimeData --type json --file ..\mongo_seed_demo\demo_data.json 
mongodb-runtime\bin\mongoimport --db json_scada --collection processInstances --type json --file ..\mongo_seed_demo\demo_process_instances.json 
mongodb-runtime\bin\mongoimport --db json_scada --collection users --type json --file ..\mongo_seed_demo\demo_users.json 
mongodb-runtime\bin\mongoimport --db json_scada --collection roles --type json --file ..\mongo_seed_demo\demo_roles.json 
rem Svedala CIM-EMS demo (ReliCapGrid model, CC BY-SA 4.0): model, bindings, field and result points
mongodb-runtime\bin\mongoimport --db json_scada --collection realtimeData --type json --file ..\mongo_seed_demo\demo_cim_data.json 
mongodb-runtime\bin\mongoimport --db json_scada --collection cimModels --type json --file ..\mongo_seed_demo\demo_cim_models.json 
mongodb-runtime\bin\mongoimport --db json_scada --collection cimMeasurementBindings --type json --file ..\mongo_seed_demo\demo_cim_bindings.json 
mongodb-runtime\bin\mongoimport --db json_scada --collection cimArtifacts.files --type json --file ..\mongo_seed_demo\demo_cim_artifacts_files.json 
mongodb-runtime\bin\mongoimport --db json_scada --collection cimArtifacts.chunks --type json --file ..\mongo_seed_demo\demo_cim_artifacts_chunks.json 
mongodb-runtime\bin\mongo json_scada --eval "db.realtimeData.updateMany({_id:{$gt:0}},{$set:{dbId:'demo'}})"
