# {json:scada} Main build docker container - (c) 2021 - Ricardo L. Olsen 

FROM mcr.microsoft.com/dotnet/sdk:5.0-alpine3.12 AS dotnetDrivers
ENV DOTNET_CLI_TELEMETRY_OPTOUT=1
COPY src/lib60870.netcore /json-scada/src/lib60870.netcore
COPY src/dnp3 /json-scada/src/dnp3
COPY src/libplctag /json-scada/src/libplctag
RUN sh -c "mkdir /json-scada/bin"
RUN sh -c "cd /json-scada/src/lib60870.netcore/lib60870.netcore/ && \
           dotnet build --runtime linux-musl-x64 -c Release -o /json-scada/bin/ && \
           cd /json-scada/src/lib60870.netcore/iec101client/ && \
           dotnet publish --runtime linux-musl-x64 -p:PublishReadyToRun=true -c Release -o /json-scada/bin/ && \
           cd /json-scada/src/lib60870.netcore/iec101server/ && \
           dotnet publish --runtime linux-musl-x64 -p:PublishReadyToRun=true -c Release -o /json-scada/bin/ && \
           cd /json-scada/src/lib60870.netcore/iec104client/ && \
           dotnet publish --runtime linux-musl-x64 -p:PublishReadyToRun=true -c Release -o /json-scada/bin/ && \
           cd /json-scada/src/lib60870.netcore/iec104server/ && \
           dotnet publish --runtime linux-musl-x64 -p:PublishReadyToRun=true -c Release -o /json-scada/bin/ && \
           cd /json-scada/src/dnp3/Dnp3Client/ && \
           dotnet publish --runtime linux-musl-x64 -p:PublishReadyToRun=true -c Release -o /json-scada/bin/ && \
           cd /json-scada/src/libplctag/libplctag.NET/src/libplctag && \
           dotnet build --runtime linux-musl-x64 -c Release -o /json-scada/bin/ && \
           cd /json-scada/src/libplctag/PLCTagsClient && \
           dotnet publish --runtime linux-musl-x64 -p:PublishReadyToRun=true -c Release -o /json-scada/bin/"

FROM golang:alpine3.12 AS golangProgs
COPY src/calculations /go/src/calculations
COPY src/i104m /go/src/i104m
COPY --from=dotnetDrivers /json-scada/bin /json-scada/bin
RUN sh -c "apk add --no-cache git && \
           cd /go/src/calculations/ && \
           go get ./... && \
           go build && \
           cp calculations /json-scada/bin/"
RUN sh -c "apk add --no-cache git && \
           cd /go/src/i104m/ && \
           go get ./... && \
           go build && \
           cp i104m /json-scada/bin/"

FROM node:current-alpine3.12 AS nodejsProgs
RUN sh -c "apk add --no-cache postgresql-client bash git"
RUN sh -c "npm install -g npm"
COPY --from=golangProgs /json-scada/bin /json-scada/bin
#COPY --from=dotnetProgs src/lib60870.netcore /json-scada/src/lib60870.netcore
#COPY --from=dotnetProgs src/dnp3 /json-scada/src/dnp3
#COPY --from=dotnetProgs src/libplctag /json-scada/src/libplctag
COPY sql /json-scada/sql
COPY demo-docker/conf/json-scada.json /json-scada/conf/json-scada.json
COPY src/cs_data_processor /json-scada/src/cs_data_processor
COPY src/server_realtime /json-scada/src/server_realtime
COPY src/server_realtime_auth /json-scada/src/server_realtime_auth
COPY src/htdocs-admin /json-scada/src/htdocs-admin
COPY src/htdocs-login /json-scada/src/htdocs-login
COPY src/htdocs /json-scada/src/htdocs
COPY src/alarm_beep /json-scada/src/alarm_beep
COPY src/oshmi2json /json-scada/src/oshmi2json
RUN sh -c "cd /json-scada/src/cs_data_processor && npm update"
RUN sh -c "cd /json-scada/src/server_realtime && npm update"
RUN sh -c "cd /json-scada/src/server_realtime_auth && npm update"
RUN sh -c "cd /json-scada/src/htdocs-admin && npm update && npm run build"
RUN sh -c "cd /json-scada/src/alarm_beep && npm update"
RUN sh -c "cd /json-scada/src/oshmi2json && npm update"

# Dotnet runtime deps
#  from https://github.com/dotnet/dotnet-docker/blob/master/src/runtime-deps/3.1/alpine3.12/amd64/Dockerfile
USER root
RUN apk add --no-cache \
        ca-certificates \
        \
        # .NET Core dependencies
        krb5-libs \
        libgcc \
        libintl \
        libssl1.1 \
        libstdc++ \
        zlib

ENV \
    # Configure web servers to bind to port 80 when present
    ASPNETCORE_URLS=http://+:80 \
    # Enable detection of running in a container
    DOTNET_RUNNING_IN_CONTAINER=true \
    # Set the invariant mode since icu_libs isn't included (see https://github.com/dotnet/announcements/issues/20)
    DOTNET_SYSTEM_GLOBALIZATION_INVARIANT=true

# Install .NET
# from https://github.com/dotnet/dotnet-docker/blob/master/src/runtime/5.0/alpine3.12/amd64/Dockerfile
ENV DOTNET_VERSION=5.0.2

RUN wget -O dotnet.tar.gz https://dotnetcli.azureedge.net/dotnet/Runtime/$DOTNET_VERSION/dotnet-runtime-$DOTNET_VERSION-linux-musl-x64.tar.gz \
    && dotnet_sha512='84e69846188689cf5ee1eddce77c6cf92a7becddac9cdd9b985a490446d5acbd5d59e3703e8da4241895c1907b85bac852735c756098774e3b890c1944cda64f' \
    && echo "$dotnet_sha512  dotnet.tar.gz" | sha512sum -c - \
    && mkdir -p /usr/share/dotnet \
    && tar -C /usr/share/dotnet -oxzf dotnet.tar.gz \
    && ln -s /usr/share/dotnet/dotnet /usr/bin/dotnet \
    && rm dotnet.tar.gz
