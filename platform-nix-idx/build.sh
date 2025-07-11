#!/bin/bash

# Required tools:
# Dotnet SDK 8.0+
# Golang 1.21+
# Node.js 20+

# call with argument linux-arm64 for ARM architecture

ARG1=${1:-linux-x64}

# extract submodules for mongodb C++ driver and opendnp3 (may take too much storage space)
#git submodule update --init --recursive

# remove flutter to release space (flutter is not used in this project)
rm -rf ~/flutter

cd ..
mkdir bin
mkdir bin-wine

unzip platform-nix-idx/bin/bin.zip -d bin/
chmod +x bin/*

export DOTNET_CLI_TELEMETRY_OPTOUT=1

# Dnp3Client is Windows-only (must run under Wine on Linux)
#cp src/dnp3/Dnp3Client/Dependencies/OpenSSL/*.dll bin-wine/ 
#cd src/dnp3/Dnp3Client
#dotnet publish --self-contained --runtime win-x64 -p:PublishReadyToRun=true -c Release -o ../../../bin-wine/ Dnp3Client.csproj

cd src/libiec61850
mkdir build
cd build
cmake ..
make
cp src/libiec61850.so src/libiec61850.so.1.6.0 ../../../bin/
cd ../dotnet/core/2.0/IEC61850.NET.core.2.0
dotnet publish --no-self-contained --runtime $ARG1 -c Release
cd ../../../../../iec61850_client
dotnet publish --no-self-contained --runtime $ARG1 -p:PublishReadyToRun=true -c Release -o ../../bin/ 

cd ../lib60870.netcore
dotnet publish --no-self-contained --runtime $ARG1 -p:PublishReadyToRun=true -c Release -o ../../bin/

cd ../OPC-UA-Client
dotnet restore
dotnet publish --no-self-contained --runtime $ARG1 -p:PublishReadyToRun=true -c Release -o ../../bin/

#cd ../opcdaaehda-client-solution-net
#dotnet build -f net8.0-windows DaAeHdaNetStandard.sln

#cd ../OPC-DA-Client
#dotnet restore
#dotnet publish --no-self-contained --runtime win-x64 -p:PublishReadyToRun=true -f net8.0-windows -c Release -o ../../bin-wine/ OPC-DA-Client.csproj

#git submodule update --init --recursive
#cd ../mongo-cxx-driver/mongo-cxx-driver/build
#cmake .. -DCMAKE_INSTALL_PREFIX="../../../mongo-cxx-driver-lib" -DCMAKE_CXX_STANDARD=17 -DBUILD_VERSION=4.0.0 -DBUILD_SHARED_LIBS=OFF -DBUILD_SHARED_AND_STATIC_LIBS=OFF  -DOPENSSL_INCLUDE_DIR=/nix/store/jv45xs1p8v9mcychfgkv6vxridcn532h-openssl-3.4.1-dev/include/ -DOPENSSL_ROOT_DIR=/nix/store/jv45xs1p8v9mcychfgkv6vxridcn532h-openssl-3.4.1-dev/ -DCMAKE_INCLUDE_PATH=/nix/store/l5ghfypzbn253233wkxhija58nlpdg4g-snappy-1.1.10-dev/include/ -DCMAKE_LIBRARY_PATH=/nix/store/l5ghfypzbn253233wkxhija58nlpdg4g-snappy-1.1.10-dev/ 
## -DLIBRESSL_CRYPTO_LIBRARY=/nix/store/ip844kbgzqrkhg42g2m9ghygfkyamrw2-libressl-3.8.4-dev/ -DLIBRESSL_INCLUDE_DIR=/nix/store/ip844kbgzqrkhg42g2m9ghygfkyamrw2-libressl-3.8.4-dev/include/
#cmake --build . --config Release
#cmake --build . --target install --config Release
#
#cd ../../../dnp3/opendnp3
#mkdir build
#cd build
#cmake -DDNP3_EXAMPLES=OFF -DDNP3_TLS=ON ..
#make
#cp cpp/lib/libopendnp3.so ../../../../bin/
#
#cd ../../Dnp3Server
#mkdir build
#cd build
#cmake ..
#make
#cp Dnp3Server ../../../../bin/
#cd ../..

export GOBIN=~/json-scada/bin
go env -w GO111MODULE=auto
export CGO_CPPFLAGS="-I /usr/include"
export CGO_LDFLAGS="-L /usr/lib"

# you may need a lot of memory to build go drivers, the build may be killed by the system, if necessary add swap, e.g. 8GB RAM + 4GB Swap

#cd ../calculations
#go mod tidy 
#go build
#cp calculations ../../bin/
#
#cd ../i104m
#go mod tidy 
#go build
#cp i104m ../../bin/
#
#cd ../plc4x-client
#go mod tidy 
#go build
#cp plc4x-client ../../bin/

# release some disk space
rm -rf ~/.cache
rm -rf ~/.nuget

cd ../cs_data_processor
npm install
cd ../cs_custom_processor
npm install
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
rm -rf ~/json-scada/src/AdminUI/node_modules

#cd ../log-io/ui
#npm install
#npm run build
#cd ../server
#npm install
#npm run build
#cd ../inputs/file
#npm install
#npm run build

export NODE_OPTIONS=--max-old-space-size=10000
export ASTRO_TELEMETRY_DISABLED=1

cd ../custom-developments/basic_bargraph
npm install
npm run build

# release some disk space
rm -rf ~/json-scada/src/custom-developments/basic_bargraph/node_modules

cd ../../custom-developments/advanced_dashboard
npm install
npm run build

# release some disk space
rm -rf ~/json-scada/src/custom-developments/advanced_dashboard/node_modules

cd ../../custom-developments/transformer_with_command
npm install
npm run build

# release some disk space
rm -rf ~/json-scada/src/custom-developments/transformer_with_commands/node_modules

cd ~/json-scada/platform-nix-idx

