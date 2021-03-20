#!/bin/bash

cd ..
mkdir bin

cd src/lib60870.netcore
dotnet publish --runtime linux-x64 -p:PublishReadyToRun=true -c Release -o ../../bin/

cd ../OPC-UA-Client
dotnet publish --runtime linux-x64 -p:PublishReadyToRun=true -c Release -o ../../bin/

cd ../dnp3/Dnp3Client
dotnet publish --runtime linux-x64 -p:PublishReadyToRun=true -c Release -o ../../../bin/

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
npm update
cd ../grafana_alert2event
npm update
cd ../demo_simul
npm update
cd ../server_realtime
npm update
cd ../server_realtime_auth
npm update
cd ../oshmi2json
npm update
cd ../oshmi_sync
npm update
cd ../htdocs-admin
npm update
npm run build
cd ../shell_api
npm update
cd ../alarm_beep
npm update
cd ../telegraf-listener
npm update

