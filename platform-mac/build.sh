#!/bin/bash

# Required tools:
# Dotnet SDK 6.0+
# Golang 1.14+
# Node.js 14+

ARG1=${1:-osx-x64}

cd ..
mkdir bin
mkdir bin-wine

export DOTNET_CLI_TELEMETRY_OPTOUT=1

# Dnp3Client is Windows-only (must run under Wine on Linux)
cp src/dnp3/Dnp3Client/Dependencies/OpenSSL/*.dll bin-wine/ 
cd src/dnp3/Dnp3Client
dotnet publish --self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ../../../bin-wine/ Dnp3Client.csproj

cd ../../libiec61850
make
make install
cd dotnet/core/2.0
dotnet publish --no-self-contained --runtime $ARG1 -c Release IEC61850.NET.core.2.0
cd iec61850_client
dotnet publish --no-self-contained --runtime $ARG1 -p:PublishReadyToRun=true -c Release -o ../../../../../../bin/ 

cd ../../../../../lib60870.netcore
dotnet publish --no-self-contained --runtime $ARG1 -p:PublishReadyToRun=true -c Release -o ../../bin/

cd ../OPC-UA-Client
dotnet publish --no-self-contained --runtime $ARG1 -p:PublishReadyToRun=true -c Release -o ../../bin/

export GOBIN=~/json-scada/bin
go env -w GO111MODULE=auto

cd ../calculations
go mod tidy 
go build
cp calculations ../../bin/

cd ../i104m
go mod tidy 
go build
cp i104m ../../bin/

cd ../plc4x-client
go mod tidy 
go build
cp plc4x-client ../../bin/

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
export NODE_OPTIONS=--openssl-legacy-provider
npm install
npm run build
export NODE_OPTIONS=
cd ../shell-api
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
cd ../backup-mongo
npm install
cd ../mongofw
npm install
cd ../mongowr
npm install
cd ../log-io/ui
npm install
npm run build
cd ../server
npm install
npm run build
cd ../inputs/file
npm install
npm run build

cd ../../../..
