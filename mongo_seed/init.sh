#!/bin/sh 
JSON_SCADA_DATABASE=json_scada
mongo mongodb://localhost:27017/$JSON_SCADA_DATABASE  < a_rs-init.js
mongo mongodb://localhost:27017/$JSON_SCADA_DATABASE  < b_create-db.js
mongoimport --db $JSON_SCADA_DATABASE --collection realtimeData --type json --file realtime_data.json 
