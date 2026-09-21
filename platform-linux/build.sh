#!/bin/bash

# Required tools:
# Dotnet SDK 8.0+
# Golang 1.21+
# Node.js 20+

ARG1=${1:-linux-x64}
case $(uname -m) in
    x86_64) ARG1="linux-x64" ;;
    arm)    ARG1="linux-arm64";;
esac

ARCHITECTURE="amd64"
case $(uname -m) in
    x86_64) ARCHITECTURE="amd64" ;;
    arm)    ARCHITECTURE="arm64";;
esac

cd ..
mkdir bin
mkdir bin_alt
mkdir bin-wine
cd src
export DOTNET_CLI_TELEMETRY_OPTOUT=1

#cp src/dnp3/Dnp3Client/Dependencies/OpenSSL/*.dll bin-wine/ 
#cd src/dnp3/Dnp3Client
#dotnet publish --self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ../../../bin-wine/ Dnp3Client.csproj

# IEC 61850 main drivers are now built in Go (src/iec61850), see the Go section below.
# cd src/libiec61850
# mkdir build
# cd build
# cmake ..
# make
# cp src/libiec61850.so src/libiec61850.so.* ../../../bin_alt/
# cd ../dotnet/core/2.0/IEC61850.NET.core.2.0
# dotnet publish --self-contained --runtime $ARG1 -c Release
# cd ../../../../../iec61850_client
# dotnet publish --self-contained --runtime $ARG1 -p:PublishReadyToRun=true -c Release -o ../../bin_alt/

# cd ../iec61850_server
# dotnet publish --self-contained --runtime $ARG1 -p:PublishReadyToRun=true -c Release -o ../../bin_alt/

# IEC 60870-5-101/104 main drivers are now built in Go (src/iec60870-5), see the Go section below.
# cd ../lib60870.netcore
# dotnet restore
# dotnet publish --self-contained --runtime $ARG1 -p:IsPackable=false -p:GeneratePackageOnBuild=false -p:PublishReadyToRun=true -c Release -o ../../bin_alt/

# cd ../OPC-UA-Client
# dotnet restore
# dotnet publish --self-contained --runtime $ARG1 -p:PublishReadyToRun=true -c Release -o ../../bin/

#cd ../opcdaaehda-client-solution-net
#dotnet build -f net8.0-windows DaAeHdaNetStandard.sln

#cd ../OPC-DA-Client
#dotnet restore
#dotnet publish --self-contained --runtime win-x64 -p:PublishReadyToRun=true -f net8.0-windows -c Release -o ../../bin-wine/ OPC-DA-Client.csproj

#cd ../mongo-cxx-driver/mongo-cxx-driver/build
#cmake .. -DCMAKE_INSTALL_PREFIX="../../../mongo-cxx-driver-lib" -DCMAKE_CXX_STANDARD=17 -DBUILD_VERSION=4.0.0 -DBUILD_SHARED_LIBS=OFF -DBUILD_SHARED_AND_STATIC_LIBS=OFF
#cmake --build . --config Release
#cmake --build . --target install --config Release

# cd ../../../dnp3/opendnp3
# mkdir build
# cd build
# cmake -DDNP3_EXAMPLES=OFF -DDNP3_TLS=ON ..
# make
# cp cpp/lib/libopendnp3.so ../../../../bin/

# cd ../../Dnp3Server
# mkdir build
# cd build
# cmake ..
# make
# cp Dnp3Server ../../../../bin/
# cd ../..

# cd Dnp3ClientCpp
# mkdir build
# cd build
# cmake ..
# make
# cp Dnp3ClientCpp ../../../../bin/
# cd ../..

export GOBIN=~/json-scada/bin
go env -w GO111MODULE=auto

cd calculations
go mod tidy 
go build
cp calculations ../../bin/

# cd ../i104m
# go mod tidy 
# go build
# cp i104m ../../bin/

# you may need a lot of memory to build this step, the build may be killed by the system, if necessary add swap, e.g. 8GB RAM + 4GB Swap
# cd ../plc4x-client
# go mod tidy
# go build
# cp plc4x-client ../../bin/

cd ../OPC-UA-Client-Go
go mod tidy
go build -ldflags="-s -w" -o ../../bin/opcua-client

# Go implementation of the DNP3 client and server drivers, drop-in replacements
# for the C++ Dnp3ClientCpp and Dnp3Server. No opendnp3, mongo-cxx-driver or
# OpenSSL build is needed for these.
cd ../dnp3-go
go mod tidy
go build -ldflags="-s -w" -o ../../bin/dnp3-client ./cmd/dnp3client
go build -ldflags="-s -w" -o ../../bin/dnp3-server ./cmd/dnp3server

cd ../iec60870-5
go mod tidy
go build -o ../../bin/iec104client ./cmd/iec104client
go build -o ../../bin/iec104server ./cmd/iec104server
go build -o ../../bin/iec101client ./cmd/iec101client
go build -o ../../bin/iec101server ./cmd/iec101server
go build -o ../../bin/iec103client ./cmd/iec103client

cd ../iec61850/iec61850_client
go mod tidy
go build -ldflags="-s -w" -o ../../../bin/iec61850_client 

cd ../iec61850_server
go mod tidy
go build -ldflags="-s -w" -o ../../../bin/iec61850_server 
cd ..

# PLC4J client (Java alternative for the PLC4X driver) - built only when JDK 17+ and Maven are available
if command -v mvn >/dev/null 2>&1; then
  cd ../plc4j-client
  mvn -B -ntp -DskipTests package
  cp target/plc4j-client.jar ../../bin/
  cp plc4j-client.sh ../../bin/
  chmod +x ../../bin/plc4j-client.sh
  cd ../plc4x-client
fi

cd ../iccp/iccp-server
#go mod tidy 
#go build
cp iccp-server-linux-$ARCHITECTURE ../../../bin/iccp-server
chmod +x ../../../bin/iccp-server
cd ..

cd ../iccp/iccp-client
#go mod tidy 
#go build
cp iccp-client-linux-$ARCHITECTURE ../../../bin/iccp-client
chmod +x ../../../bin/iccp-client
cd ..

# release some disk space
rm -rf ~/.cache

cd ../config_server_for_excel
npm install
cd ../cs_data_processor
npm install

# Go implementation of cs_data_processor, a drop-in replacement for the
# Node.js one (run only one of them per instance number)
cd ../cs_data_processor-go
go mod tidy
go build -ldflags="-s -w" -o ../../bin/cs_data_processor-go

cd ../cs_custom_processor
npm install
npm run build
cd ../mcp-json-scada-db
npm install
npm run build
cd ../grafana_alert2event
npm install
cd ../demo_simul
npm install
cd ../server_realtime_auth
npm install
cd ../updateUser
npm install
cd ../oshmi2json
npm install
cd ../oshmi_sync
npm install
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
cd ../modbus
npm install
npm run build
cd ../node-red-driver
npm install
npm run build
cd ../n8n-client
npm install
cd ../carbone-reports
npm install
cd ../backup-mongo
npm install
cd ../mongofw
npm install
cd ../mongowr
npm install
cd ../camera-onvif
npm install

cd ../AdminUI
npm install
npm run build

cd ../svgedit
npm install
npm run build

cd ../log-io/ui
npm install
npm run build
cd ../server
npm install
npm run build
cd ../inputs/file
npm install
npm run build

export NODE_OPTIONS=--max-old-space-size=10000
export ASTRO_TELEMETRY_DISABLED=1

cd ../../../custom-developments/basic_bargraph
npm install
npx astro telemetry disable
npm run build

cd ../../custom-developments/advanced_dashboard
npm install
npm run build

cd ../../custom-developments/transformer_with_command
npm install
npm run build

cd ../../../..

