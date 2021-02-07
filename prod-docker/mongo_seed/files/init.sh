#!/bin/sh 
sleep 5 
<<<<<<< HEAD
mongoimport --db $MONGO_INITDB_DATABASE --collection protocolDriverInstances --type json --file /docker-entrypoint-initdb.d/prod_instances.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection protocolConnections --type json --file /docker-entrypoint-initdb.d/prod_connections.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection realtimeData --type json --file /docker-entrypoint-initdb.d/prod_data.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection processInstances --type json --file /docker-entrypoint-initdb.d/prod_process_instances.json 
=======
mongoimport --db $MONGO_INITDB_DATABASE --collection protocolDriverInstances --type json --file /docker-entrypoint-initdb.d/demo_instances.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection protocolConnections --type json --file /docker-entrypoint-initdb.d/demo_connections.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection realtimeData --type json --file /docker-entrypoint-initdb.d/demo_data.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection processInstances --type json --file /docker-entrypoint-initdb.d/demo_process_instances.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection users --type json --file /docker-entrypoint-initdb.d/demo_users.json 
mongoimport --db $MONGO_INITDB_DATABASE --collection roles --type json --file /docker-entrypoint-initdb.d/demo_roles.json 
>>>>>>> 4e5cdd18a1acc046429e79ffd0f83eb1dca35f58
