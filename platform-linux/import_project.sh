#!/bin/bash

# to be completed and tested

JSPATH=~/json-scada
TMPPATH=$JSPATH/tmp
MONGOBIN=/usr/bin
JAVAPATH=/usr/bin
TARPATH=/usr/bin
SVGPATH=$JSPATH/svg
# read JSON config file
mongoConnectionString=$(jq -r '.mongoConnectionString' $JSPATH/conf/json-scada.json)
database=$(jq -r '.mongoDatabaseName' $JSPATH/conf/json-scada.json)
FLAGS=--mode=upsert

cd $TMPPATH

mongosh --quiet --eval "db.realtimeData.deleteMany({})" "$mongoConnectionString"
mongosh --quiet --eval "db.processInstances.deleteMany({})" "$mongoConnectionString"
mongosh --quiet --eval "db.protocolConnections.deleteMany({})" "$mongoConnectionString"
mongosh --quiet --eval "db.protocolDriverInstances.deleteMany({})" "$mongoConnectionString"
mongosh --quiet --eval "db.users.deleteMany({})" "$mongoConnectionString"
mongosh --quiet --eval "db.roles.deleteMany({})" "$mongoConnectionString"

mongoimport --uri "$mongoConnectionString" --db $database --collection roles  $FLAGS --file roles.json
mongoimport --uri "$mongoConnectionString" --db $database --collection users $FLAGS --file users.json 
mongoimport --uri "$mongoConnectionString" --db $database --collection processInstances $FLAGS --file processInstances.json
mongoimport --uri "$mongoConnectionString" --db $database --collection protocolDriverInstances $FLAGS --file protocolDriverInstances.json
mongoimport --uri "$mongoConnectionString" --db $database --collection protocolConnections $FLAGS --file protocolConnections.json
mongoimport --uri "$mongoConnectionString" --db $database --collection realtimeData $FLAGS --file realtimeData.json
# optional historical data
# mongoimport --uri "$mongoConnectionString" --db $database --collection hist --file hist.json $FLAGS
# mongoimport --uri "$mongoConnectionString" --db $database --collection backfillData --file backfillData.json $FLAGS
# mongoimport --uri "$mongoConnectionString" --db $database --collection soeData --file soeData.json $FLAGS
# mongoimport --uri "$mongoConnectionString" --db $database --collection userActions --file userActions.json $FLAGS

cp $TMPPATH/*.svg $SVGPATH/
cp $TMPPATH/screen_list.js $SVGPATH/
