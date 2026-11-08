#!/bin/sh 
sleep 5 
mongoimport --db $MONGO_INITDB_DATABASE --collection protocolDriverInstances --type json --file /docker-entrypoint-initdb.d/demo_instances.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection protocolConnections --type json --file /docker-entrypoint-initdb.d/demo_connections_linux.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection realtimeData --type json --file /docker-entrypoint-initdb.d/demo_data.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection processInstances --type json --file /docker-entrypoint-initdb.d/demo_process_instances.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection users --type json --file /docker-entrypoint-initdb.d/demo_users.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection roles --type json --file /docker-entrypoint-initdb.d/demo_roles.json 
# Svedala CIM-EMS demo (ReliCapGrid model, CC BY-SA 4.0): model, bindings, field and result points
mongoimport --db $MONGO_INITDB_DATABASE --collection realtimeData --type json --file /docker-entrypoint-initdb.d/demo_cim_data.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection cimModels --type json --file /docker-entrypoint-initdb.d/demo_cim_models.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection cimMeasurementBindings --type json --file /docker-entrypoint-initdb.d/demo_cim_bindings.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection cimArtifacts.files --type json --file /docker-entrypoint-initdb.d/demo_cim_artifacts_files.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection cimArtifacts.chunks --type json --file /docker-entrypoint-initdb.d/demo_cim_artifacts_chunks.json 
# mark tags as demo to make it easy to remove later
mongosh $MONGO_INITDB_DATABASE --eval 'db.realtimeData.updateMany({_id:{$gt:0}},{$set:{dbId:"demo"}})'
