#!/bin/sh 
JSON_SCADA_DATABASE=json_scada
sleep 5 
mongoimport --db $JSON_SCADA_DATABASE --collection protocolDriverInstances --type json --file demo_instances.json 
mongoimport --db $JSON_SCADA_DATABASE --collection protocolConnections --type json --file demo_connections_linux.json 
mongoimport --db $JSON_SCADA_DATABASE --collection realtimeData --type json --file demo_data.json 
mongoimport --db $JSON_SCADA_DATABASE --collection processInstances --type json --file demo_process_instances.json 
# Svedala CIM-EMS demo (ReliCapGrid model, CC BY-SA 4.0), x86_64 only like the CIM-EMS driver
if [ "$(uname -m)" = "x86_64" ]; then
  mongoimport --db $JSON_SCADA_DATABASE --collection realtimeData --type json --file demo_cim_data.json
  mongoimport --db $JSON_SCADA_DATABASE --collection cimModels --type json --file demo_cim_models.json
  mongoimport --db $JSON_SCADA_DATABASE --collection cimMeasurementBindings --type json --file demo_cim_bindings.json
  mongoimport --db $JSON_SCADA_DATABASE --collection cimArtifacts.files --type json --file demo_cim_artifacts_files.json
  mongoimport --db $JSON_SCADA_DATABASE --collection cimArtifacts.chunks --type json --file demo_cim_artifacts_chunks.json
fi
# mark tags as demo to make it easy to remove later
mongosh $JSON_SCADA_DATABASE --eval 'db.realtimeData.updateMany({_id:{$gt:0}},{$set:{dbId:"demo"}})'
