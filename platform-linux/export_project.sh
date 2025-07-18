#!/bin/bash

JSPATH=~/json-scada
TMPPATH=$JSPATH/tmp
MONGOBIN=/usr/bin
JAVAPATH=/usr/bin
TARPATH=/usr/bin
SVGPATH=$JSPATH/svg
# read JSON config file
mongoConnectionString=$(jq -r '.mongoConnectionString' $JSPATH/conf/json-scada.json)
database=$(jq -r '.mongoDatabaseName' $JSPATH/conf/json-scada.json)

mkdir $TMPPATH
rm -rf $TMPPATH/*.*

project_name=${1:-jsproject.zip}

mongoexport --uri "$mongoConnectionString" --db $database --collection realtimeData --out $TMPPATH/realtimeData.json
mongoexport --uri "$mongoConnectionString" --db $database --collection processInstances --out $TMPPATH/processInstances.json
mongoexport --uri "$mongoConnectionString" --db $database --collection protocolDriverInstances --out $TMPPATH/protocolDriverInstances.json
mongoexport --uri "$mongoConnectionString" --db $database --collection protocolConnections --out $TMPPATH/protocolConnections.json
mongoexport --uri "$mongoConnectionString" --db $database --collection users --out $TMPPATH/users.json
mongoexport --uri "$mongoConnectionString" --db $database --collection roles --out $TMPPATH/roles.json
# optional historical data
# mongoexport --uri "$mongoConnectionString" --db $database --collection hist --out $TMPPATH\hist.json
# mongoexport --uri "$mongoConnectionString" --db $database --collection backfillData --out $TMPPATH\backfillData.json
# mongoexport --uri "$mongoConnectionString" --db $database --collection soeData --out $TMPPATH\soeData.json
# mongoexport --uri "$mongoConnectionString" --db $database --collection userActions --out $TMPPATH\userActions.json

cp $SVGPATH/*.svg $TMPPATH/
cp $SVGPATH/screen_list.js $TMPPATH/
# optional
# copy $JSPATH/conf/*.* $TMPPATH/

# zip files
cd $TMPPATH
zip -j $TMPPATH/${project_name}.zip $TMPPATH/*.json $TMPPATH/*.js $TMPPATH/*.svg