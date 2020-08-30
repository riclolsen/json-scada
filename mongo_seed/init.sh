#!/bin/sh 
JSON_SCADA_DATABASE=json_scada
mongo --db $JSON_SCADA_DATABASE  < a_rs-init.js
mongo --db $JSON_SCADA_DATABASE  < b_create-db.js
mongoimport --db $JSON_SCADA_DATABASE --collection realtimeData --type json --file /docker-entrypoint-initdb.d/realtime_data.json 
