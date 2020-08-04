#!/bin/sh
sleep 5
mongoimport --db $MONGO_INITDB_DATABASE --collection protocolDriverInstances --type json --file /docker-entrypoint-initdb.d/demo_instances.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection protocolConnections --type json --file /docker-entrypoint-initdb.d/demo_connections.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection realtimeData --type json --file /docker-entrypoint-initdb.d/demo_data.json