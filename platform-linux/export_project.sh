#!/bin/bash

# to be completed and tested

JSPATH=~/json-scada
TMPPATH=$JSPATH/tmp
MONGOBIN=/usr/bin
JAVAPATH=/usr/bin
TARPATH=/usr/bin
SVGPATH=$JSPATH/svg
mongoConnectionString=mongodb://127.0.0.1/json_scada?tls=false&directConnection=true
database=json_scada

# read JSON config file

mkdir -c $TMPPATH
rm -rf $TMPPATH/*.*

mongoexport.exe --uri "$mongoConnectionString" --db $database --collection realtimeData --out $TMPPATH\realtimeData.json
mongoexport.exe --uri "$mongoConnectionString" --db $database --collection processInstances --out $TMPPATH\processInstances.json
mongoexport.exe --uri "$mongoConnectionString" --db $database --collection protocolDriverInstances --out $TMPPATH\protocolDriverInstances.json
mongoexport.exe --uri "$mongoConnectionString" --db $database --collection protocolConnections --out $TMPPATH\protocolConnections.json
mongoexport.exe --uri "$mongoConnectionString" --db $database --collection users --out $TMPPATH\users.json
mongoexport.exe --uri "$mongoConnectionString" --db $database --collection roles --out $TMPPATH\roles.json
# optional historical data
# mongoexport.exe --uri "$mongoConnectionString" --db $database --collection hist --out $TMPPATH\hist.json
# mongoexport.exe --uri "$mongoConnectionString" --db $database --collection backfillData --out $TMPPATH\backfillData.json
# mongoexport.exe --uri "$mongoConnectionString" --db $database --collection soeData --out $TMPPATH\soeData.json
# mongoexport.exe --uri "$mongoConnectionString" --db $database --collection userActions --out $TMPPATH\userActions.json

copy %SVGPATH%\*.svg %TMPPATH%\
copy %SVGPATH%\screen_list.js %TMPPATH%\
# optional
# copy %JSPATH%\conf\*.* %TMPPATH%\

# zip files
