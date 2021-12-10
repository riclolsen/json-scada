#!/bin/bash

# call with argument linux-arm64 for ARM architecture

ARG1=${1:-linux-x64}

cd ..
mkdir bin
mkdir bin-wine

cd src/lib60870.netcore
dotnet publish --runtime $ARG1 -p:PublishReadyToRun=true -c Release -o ../../bin/

cd ../OPC-UA-Client
dotnet publish --runtime $ARG1 -p:PublishReadyToRun=true -c Release -o ../../bin/

# Dnp3Client is Windows-only (must run under Wine on Linux)
cd ../dnp3/Dnp3Client
dotnet publish --runtime win-x64 -p:PublishReadyToRun=false -c Release -o ../../../bin-wine/ Dnp3Client.csproj

export GOBIN=~/json-scada/bin
go env -w GO111MODULE=auto

cd ../../calculations
go get ./...
go build
cp calculations ../../bin/

cd ../plc4x-client
go get "github.com/icza/bitio"
go get ./... 
go build 
cp plc4x-client ../../bin/

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
cd ../updateUser
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
cd ../OPC-UA-Server
npm install
cd ../carbone-reports
npm install
cd ../..

