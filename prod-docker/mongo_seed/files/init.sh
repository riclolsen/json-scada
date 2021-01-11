#!/bin/sh 
sleep 5 
mongoimport --db $MONGO_INITDB_DATABASE --collection protocolDriverInstances --type json --file /docker-entrypoint-initdb.d/prod_instances.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection protocolConnections --type json --file /docker-entrypoint-initdb.d/prod_connections.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection realtimeData --type json --file /docker-entrypoint-initdb.d/prod_data.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection processInstances --type json --file /docker-entrypoint-initdb.d/prod_process_instances.json 
