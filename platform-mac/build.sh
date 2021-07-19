#!/bin/bash

ARG1=${1:-osx-x64}

cd ..
mkdir bin

cd src/lib60870.netcore
dotnet publish --runtime $ARG1 -p:PublishReadyToRun=true -c Release -o ../../bin/

cd ../OPC-UA-Client
dotnet publish --runtime $ARG1 -p:PublishReadyToRun=true -c Release -o ../../bin/

cd ../dnp3/Dnp3Client
dotnet publish --runtime $ARG1 -p:PublishReadyToRun=true -c Release -o ../../../bin/

export GOBIN=~/json-scada/bin
go env -w GO111MODULE=auto

cd ../../calculations
go get ./...
go build
cp calculations ../../bin/

cd ../i104m
go get ./...
go build
cp i104m ../../bin/

cd ../cs_data_processor
npm install
cd ../cs_custom_processor
npm install
cd ../grafana_alert2event
npm install
cd ../demo_simul
npm install
cd ../server_realtime
npm install
cd ../server_realtime_auth
npm install
cd ../oshmi2json
npm install
cd ../oshmi_sync
npm install
cd ../htdocs-admin
npm install
npm run build
cd ../shell_api
npm install
cd ../alarm_beep
npm install
cd ../telegraf-listener
npm install
cd ../mqtt-sparkplug
npm install
cd ../..

