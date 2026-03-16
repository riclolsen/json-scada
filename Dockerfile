# Based on Ubuntu 24.04 with all required services for JSON-SCADA
# Build context should be the PROJECT ROOT (one level up from demo-docker)
# Build command: sudo docker build --pull --no-cache -t json-scada:latest -f Dockerfile .
# Run command: sudo docker run -p 80:80 -p 9000:9000 -p 4840:4840 -p 2404:2404 -p 20000:20000 -i -t json-scada:latest
# ==============================================================================
FROM ubuntu:24.04

LABEL maintainer="JSON-SCADA"
LABEL description="Multi-service container with Node.js, .NET, Go, PostgreSQL/TimescaleDB, MongoDB, Grafana, Telegraf, Nginx"

# Prevent interactive prompts during package installation
ENV DEBIAN_FRONTEND=noninteractive
ENV TZ=UTC

# ==============================================================================
# BASE SYSTEM PACKAGES AND BUILD TOOLS
# ==============================================================================
RUN apt-get update && apt-get install -y \
    build-essential \
    openjdk-21-jdk \
    cmake \
    sasl2-bin \
    libsasl2-dev \
    libssl-dev \
    libzstd-dev \
    libsnappy-dev \
    libsqlite3-dev \
    ffmpeg \
    curl \
    wget \
    gnupg \
    lsb-release \
    ca-certificates \
    apt-transport-https \
    software-properties-common \
    supervisor \
    unzip \
    git \
    nano \
    && rm -rf /var/lib/apt/lists/*

# ==============================================================================
# NODE.JS 24
# ==============================================================================
RUN curl -fsSL https://deb.nodesource.com/setup_24.x | bash - \
    && apt-get install -y nodejs \
    && npm install -g npm@latest \
    && rm -rf /var/lib/apt/lists/*

# Verify Node.js installation
RUN node --version && npm --version

# ==============================================================================
# .NET SDK 8
# ==============================================================================
RUN wget https://packages.microsoft.com/config/ubuntu/24.04/packages-microsoft-prod.deb -O packages-microsoft-prod.deb \
    && dpkg -i packages-microsoft-prod.deb \
    && rm packages-microsoft-prod.deb \
    && apt-get update \
    && apt-get install -y dotnet-sdk-8.0 \
    && rm -rf /var/lib/apt/lists/*

# Verify .NET installation
RUN dotnet --version

# ==============================================================================
# GOLANG
# ==============================================================================
ENV GO_VERSION=1.26.0
RUN wget https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz \
    && rm go${GO_VERSION}.linux-amd64.tar.gz

ENV PATH=$PATH:/usr/local/go/bin
ENV GOPATH=/go
ENV PATH=$PATH:$GOPATH/bin

# Verify Go installation
RUN go version

# ==============================================================================
# POSTGRESQL (Latest Stable/18) with TIMESCALEDB
# ==============================================================================
# Add PostgreSQL repository with modern keyring
RUN mkdir -p /etc/apt/keyrings \
    && wget --quiet -O - https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor -o /etc/apt/keyrings/postgresql.gpg \
    && echo "deb [signed-by=/etc/apt/keyrings/postgresql.gpg] http://apt.postgresql.org/pub/repos/apt noble-pgdg main" > /etc/apt/sources.list.d/pgdg.list

# Add TimescaleDB repository with modern keyring
RUN wget --quiet -O - https://packagecloud.io/timescale/timescaledb/gpgkey | gpg --dearmor -o /etc/apt/keyrings/timescaledb.gpg \
    && echo "deb [signed-by=/etc/apt/keyrings/timescaledb.gpg] https://packagecloud.io/timescale/timescaledb/ubuntu/ noble main" > /etc/apt/sources.list.d/timescaledb.list

# Install PostgreSQL and TimescaleDB
RUN apt-get update \
    && apt-get install -y postgresql-18 postgresql-contrib-18 timescaledb-2-postgresql-18 \
    && rm -rf /var/lib/apt/lists/*

# Configure TimescaleDB
RUN echo "shared_preload_libraries = 'timescaledb'" >> /etc/postgresql/*/main/postgresql.conf || true

# Create PostgreSQL data directory
RUN mkdir -p /var/run/postgresql && chown -R postgres:postgres /var/run/postgresql

# ==============================================================================
# MONGODB 8
# ==============================================================================
RUN curl -fsSL https://www.mongodb.org/static/pgp/server-8.0.asc | gpg --dearmor -o /usr/share/keyrings/mongodb-server-8.2.gpg \
    && echo "deb [arch=amd64,arm64 signed-by=/usr/share/keyrings/mongodb-server-8.2.gpg] https://repo.mongodb.org/apt/ubuntu noble/mongodb-org/8.2 multiverse" > /etc/apt/sources.list.d/mongodb-org-8.2.list \
    && apt-get update \
    && apt-get install -y mongodb-org \
    && rm -rf /var/lib/apt/lists/*

# Create MongoDB data directory
RUN mkdir -p /data/db && chown -R mongodb:mongodb /data/db || mkdir -p /data/db

# ==============================================================================
# GRAFANA (Latest)
# ==============================================================================
RUN mkdir -p /etc/apt/keyrings/ \
    && wget -q -O - https://apt.grafana.com/gpg.key | gpg --dearmor > /etc/apt/keyrings/grafana.gpg \
    && echo "deb [signed-by=/etc/apt/keyrings/grafana.gpg] https://apt.grafana.com stable main" > /etc/apt/sources.list.d/grafana.list \
    && apt-get update \
    && apt-get install -y grafana \
    && rm -rf /var/lib/apt/lists/*

# ==============================================================================
# METABASE
# ==============================================================================
RUN mkdir -p /app/json-scada/metabase/ \
    && wget --inet4-only https://downloads.metabase.com/v0.58.4/metabase.jar -O /app/json-scada/metabase/metabase.jar \
    && chmod +x /app/json-scada/metabase/metabase.jar 

# ==============================================================================
# TELEGRAF (Latest)
# ==============================================================================
RUN mkdir -p /etc/apt/keyrings \
    && wget -q -O - https://repos.influxdata.com/influxdata-archive.key | gpg --dearmor -o /etc/apt/keyrings/influxdata-archive.gpg \
    && echo "deb [signed-by=/etc/apt/keyrings/influxdata-archive.gpg] https://repos.influxdata.com/debian stable main" > /etc/apt/sources.list.d/influxdata.list \
    && apt-get update \
    && apt-get install -y telegraf \
    && rm -rf /var/lib/apt/lists/*

# ==============================================================================
# NGINX (Latest)
# ==============================================================================
RUN apt-get update \
    && apt-get install -y nginx \
    && rm -rf /var/lib/apt/lists/*

# ==============================================================================
# CREATE DIRECTORIES AND SET PERMISSIONS
# ==============================================================================
RUN mkdir -p /var/log/supervisor \
    && mkdir -p /app \
    && mkdir -p /var/lib/grafana \
    && mkdir -p /var/log/grafana \
    && mkdir -p /var/log/nginx \
    && mkdir -p /var/log/mongodb \
    && mkdir -p /var/log/postgresql \
    && mkdir -p /var/log/telegraf

# ==============================================================================
# COPY PROJECT SOURCE AND DATA (Build context must be project root)
# ==============================================================================
# Copy the entire project into /app/json-scada
# We assume the context is the project root (one level up from demo-docker)
COPY ./src/ /app/json-scada/src/
COPY ./svg/ /app/json-scada/svg/
RUN chmod o+w -R /app/json-scada/svg

# Set environment for builds
ENV DOTNET_CLI_TELEMETRY_OPTOUT=1
ENV NODE_OPTIONS=--max-old-space-size=10000
ENV GO111MODULE=auto
ENV CGO_ENABLED=1

# Create publish directory
RUN mkdir -p /app/json-scada/bin
RUN mkdir -p /app/json-scada/log
RUN ln -s /app /home/jsonscada
RUN useradd nginx
RUN useradd jsonscada

# ==============================================================================
# BUILD .NET PROJECTS
# ==============================================================================
WORKDIR /app/json-scada

# Build lib60870
RUN cd src/lib60870.netcore/ && dotnet publish --self-contained -p:IsPackable=false -p:GeneratePackageOnBuild=false -p:PublishReadyToRun=true -c Release -o /app/json-scada/bin/

# Cleanup lib60870
RUN cd src/lib60870.netcore/iec101client/ && rm -rf obj bin
RUN cd src/lib60870.netcore/iec101server/ && rm -rf obj bin
RUN cd src/lib60870.netcore/iec104client/ && rm -rf obj bin
RUN cd src/lib60870.netcore/iec104server/ && rm -rf obj bin
RUN cd src/lib60870.netcore/lib60870.netcore/ && rm -rf obj bin

# Build OPC-UA Client
RUN cd src/OPC-UA-Client/ && \
    rm -rf obj bin && dotnet clean && \
    dotnet publish --self-contained -p:PublishReadyToRun=true -c Release -o /app/json-scada/bin/ && \
    rm -rf obj bin

# Build libiec61850 (C library)
RUN cd src/libiec61850 && \
    rm -rf build && \
    mkdir -p build && \
    cd build && \
    cmake .. && \
    make && \
    cp src/libiec61850.so src/libiec61850.so.* /app/json-scada/bin/ || true

# Build IEC61850.NET.core
RUN cd src/libiec61850/dotnet/core/2.0/IEC61850.NET.core.2.0 && \
    dotnet publish --self-contained -c Release || true

# Build IEC 61850 Client
RUN cd src/iec61850_client && \
    dotnet publish --self-contained -p:PublishReadyToRun=true -c Release -o /app/json-scada/bin/ && \
    rm -rf obj bin || true

# Cleanup libiec61850
RUN cd src/libiec61850/dotnet/core/2.0/IEC61850.NET.core.2.0/ && rm -rf obj bin || true
RUN cd src/libiec61850 && rm -rf .install || true

# Build mongo-cxx-driver
RUN cd src/mongo-cxx-driver/mongo-cxx-driver && \
    rm -rf build && \
    mkdir -p build && \
    cd build && \
    sed -i '/   $${fetch_args}/d' ../cmake/FetchMongoC.cmake || true && \
    cmake .. -DCMAKE_INSTALL_PREFIX=../../../mongo-cxx-driver-lib -DCMAKE_CXX_STANDARD=17 -DBUILD_VERSION=4.0.0 -DBUILD_SHARED_LIBS=OFF -DBUILD_SHARED_AND_STATIC_LIBS=OFF && \
    cmake --build . --config Release && \
    cmake --build . --target install --config Release || true

# Build OpenDNP3
RUN cd src/dnp3/opendnp3 && \
    rm -rf build && \
    mkdir -p build && \
    cd build && \
    cmake -DDNP3_EXAMPLES=OFF -DDNP3_TLS=ON .. && \
    make && \
    cp cpp/lib/libopendnp3.so /app/json-scada/bin/ || true

# Build DNP3 Server
RUN cd src/dnp3/Dnp3Server/ && \
    sed -i 's/mongo-cxx-driver-lib\\/lib64\\//mongo-cxx-driver-lib\\/lib\\//g' ./CMakeLists.txt || true && \
    sed -i '/sasl2/a  snappy' ./CMakeLists.txt || true && \
    rm -rf build && \
    mkdir -p build && \
    cd build && \
    cmake .. && \
    make && \
    cp Dnp3Server /app/json-scada/bin/ || true

# ==============================================================================
# BUILD GO PROJECTS
# ==============================================================================
# Install libpcap for Go builds
RUN apt-get update && apt-get install -y libpcap-dev && rm -rf /var/lib/apt/lists/*

# Build calculations
RUN cd src/calculations/ && \
    go mod tidy && \
    go build && \
    cp calculations /app/json-scada/bin/

# Build i104m
RUN cd src/i104m/ && \
    go mod tidy && \
    go build && \
    cp i104m /app/json-scada/bin/

# Build plc4x-client
RUN cd src/plc4x-client/ && \
    go mod tidy && \
    CGO_ENABLED=1 go build && \
    cp plc4x-client /app/json-scada/bin/ || true

# ==============================================================================
# BUILD NODE.JS PROJECTS
# ==============================================================================
RUN cd src/cs_data_processor && npm install \
    && cd ../cs_custom_processor && npm install && npm run build \
    && cd ../config_server_for_excel && npm install \
    && cd ../server_realtime_auth && npm install \
    && cd ../camera-onvif && npm install \
    && cd ../oshmi2json && npm install \
    && cd ../telegraf-listener && npm install \
    && cd ../OPC-UA-Server && npm install \
    && cd ../mqtt-sparkplug && npm install \
    && cd ../mcp-json-scada-db && npm install && npm run build \
    && cd ../AdminUI && npm install && npm run build && rm -rf node_modules \
    && cd ../svgedit && npm install && npm run build && rm -rf node_modules \
    && cd ../custom-developments/basic_bargraph \
    && npm install \
    && npx astro telemetry disable \
    && npm run build && rm -rf node_modules || true \
    && cd ../../custom-developments/advanced_dashboard \
    && npm install \
    && npm run build && rm -rf node_modules || true \
    && cd ../../custom-developments/transformer_with_command \
    && npm install \
    && npm run build && rm -rf node_modules || true

# ==============================================================================
# DATABASE INITIALIZATION AND CONFIGURATION
# ==============================================================================
# Set database environment variables
ENV MONGO_INITDB_DATABASE=json_scada
ENV PGDATABASE=json_scada
ENV PGUSER=postgres
ENV PGHOST=localhost
ENV PGPORT=5432

COPY ./platform-ubuntu-2404/postgresql.conf /etc/postgresql/18/main/postgresql.conf
COPY ./platform-ubuntu-2404/pg_hba.conf /etc/postgresql/18/main/pg_hba.conf
COPY ./platform-ubuntu-2404/telegraf-input-opcua.conf /etc/telegraf/telegraf.d/telegraf-input-opcua.conf
COPY ./platform-ubuntu-2404/telegraf-input-mqtt.conf /etc/telegraf/telegraf.d/telegraf-input-mqtt.conf
COPY ./platform-ubuntu-2404/telegraf-input-mongodb.conf /etc/telegraf/telegraf.d/telegraf-input-mongodb.conf
COPY ./platform-ubuntu-2404/telegraf-output-json-scada.conf /etc/telegraf/telegraf.d/telegraf-output-json-scada.conf

COPY ./platform-ubuntu-2404/calculations.ini /etc/supervisor/conf.d/calculations.ini
COPY ./platform-ubuntu-2404/config_server_excel.ini /etc/supervisor/conf.d/config_server_excel.ini
COPY ./platform-ubuntu-2404/cs_custom_processor.ini /etc/supervisor/conf.d/cs_custom_processor.ini
COPY ./platform-ubuntu-2404/cs_data_processor.ini /etc/supervisor/conf.d/cs_data_processor.ini
COPY ./platform-ubuntu-2404/iec104client.ini /etc/supervisor/conf.d/iec104client.ini
COPY ./platform-ubuntu-2404/iec104server.ini /etc/supervisor/conf.d/iec104server.ini
COPY ./platform-ubuntu-2404/iec61850client.ini /etc/supervisor/conf.d/iec61850client.ini
COPY ./platform-ubuntu-2404/metabase.ini /etc/supervisor/conf.d/metabase.ini
COPY ./platform-ubuntu-2404/grafana_server.ini /etc/supervisor/conf.d/grafana_server.ini
COPY ./platform-ubuntu-2404/dnp3_server.ini /etc/supervisor/conf.d/dnp3_server.ini
COPY ./platform-ubuntu-2404/mcp_server.ini /etc/supervisor/conf.d/mcp_server.ini
COPY ./platform-ubuntu-2404/mongofw.ini /etc/supervisor/conf.d/mongofw.ini
COPY ./platform-ubuntu-2404/mongowr.ini /etc/supervisor/conf.d/mongowr.ini
COPY ./platform-ubuntu-2404/mqtt-sparkplug.ini /etc/supervisor/conf.d/mqtt-sparkplug.ini
COPY ./platform-ubuntu-2404/opcua_client.ini /etc/supervisor/conf.d/opcua_client.ini
COPY ./platform-ubuntu-2404/opcua_server.ini /etc/supervisor/conf.d/opcua_server.ini
COPY ./platform-ubuntu-2404/plc4xclient.ini /etc/supervisor/conf.d/plc4xclient.ini
COPY ./platform-ubuntu-2404/process_pg_hist.ini /etc/supervisor/conf.d/process_pg_hist.ini
COPY ./platform-ubuntu-2404/process_pg_rtdata.ini /etc/supervisor/conf.d/process_pg_rtdata.ini
COPY ./platform-ubuntu-2404/server_realtime_auth.ini /etc/supervisor/conf.d/server_realtime_auth.ini
COPY ./platform-ubuntu-2404/telegraf_listener.ini /etc/supervisor/conf.d/telegraf_listener.ini
COPY ./platform-ubuntu-2404/telegraf.ini /etc/supervisor/conf.d/telegraf.ini
COPY ./platform-ubuntu-2404/nginx.conf /etc/nginx/nginx.conf
COPY ./platform-ubuntu-2404/json_scada_http_open.conf /etc/nginx/conf.d/json_scada_http.conf
COPY ./platform-ubuntu-2404/json_scada_https.conf /etc/nginx/conf.d/json_scada_https.conf

# Create necessary directories
RUN mkdir -p /docker-entrypoint-initdb.d/mongo \
    && mkdir -p /docker-entrypoint-initdb.d/postgres \
    && mkdir -p /app/json-scada/conf \
    && mkdir -p /app/json-scada/log \
    && mkdir -p /app/json-scada/files \
    && mkdir -p /app/json-scada/sql \
    && chmod o+w /app/json-scada/sql

# Copy initialization scripts and data (relative to project root context)
COPY ./demo-docker/mongo_seed/files/ /docker-entrypoint-initdb.d/mongo/
COPY ./mongo_seed/ /docker-entrypoint-initdb.d/mongo/
COPY ./demo-docker/conf/ /app/json-scada/conf/
COPY ./conf-templates/json-scada.json /app/json-scada/conf/json-scada.json
COPY ./sql/ /app/json-scada/sql/

# Make scripts executable
RUN chmod +x /docker-entrypoint-initdb.d/mongo/*.sh \
    && chmod +x /app/json-scada/sql/*.sh 

# Create a master database initialization script
RUN echo '#!/bin/bash\n\
export POSTGRES_HOST_AUTH_METHOD=trust\n\
if [ -f /app/db_initialized ]; then\n\
  echo "Database already initialized."\n\
  exit 0\n\
fi\n\
# Wait for PostgreSQL to be ready\n\
until psql -h localhost -U "$POSTGRES_USER" -w -d template1 -c "select 1" > /dev/null 2>&1; do\n\
  echo "Waiting for PostgreSQL..."\n\
  sleep 2\n\
done\n\
echo "PostgreSQL is ready, running init scripts..."\n\
/bin/psql -U postgres -h localhost -d template1 -w -f /app/json-scada/sql/create_tables.sql template1 \n\
/bin/psql -U postgres -h localhost -w -f /app/json-scada/sql/grafanaappdb.sql grafanaappdb \n\
/bin/psql -U postgres -h localhost -w -f /app/json-scada/sql/metabaseappdb.sql metabaseappdb \n\
\n\
# Wait for MongoDB to be ready\n\
until mongosh --host localhost --port 27017 --eval "db.adminCommand(\"ping\")" > /dev/null 2>&1; do\n\
  echo "Waiting for MongoDB..."\n\
  sleep 2\n\
done\n\
echo "MongoDB is ready, running init scripts..."\n\
# Run mongo initialization\n\
cd /docker-entrypoint-initdb.d/mongo && ./init.sh && ./init-demo.sh && touch /app/db_initialized\n\
' > /app/init_databases.sh && chmod +x /app/init_databases.sh

# ==============================================================================
# SUPERVISOR CONFIGURATION
# ==============================================================================
COPY supervisord.conf /etc/supervisor/supervisord.conf
    
# Add swappiness to /etc/sysctl.conf for mongodb
RUN echo "vm.swappiness=1" >> /etc/sysctl.conf

# ==============================================================================
# EXPOSE PORTS
# ==============================================================================
# Nginx
EXPOSE 80 443
# Node.js application ports (customize as needed)
EXPOSE 8080
# PostgreSQL
EXPOSE 5432
# MongoDB
EXPOSE 27017
# Grafana
EXPOSE 3000
# Supervisor UI
EXPOSE 9000
# OPC-UA Server
EXPOSE 4840
# IEC104 Server
EXPOSE 2404
# DNP3 Server
EXPOSE 20000

# ==============================================================================
# VOLUMES
# ==============================================================================
VOLUME ["/var/lib/mongodb", "/var/lib/postgresql", "/var/lib/grafana", "/app/json-scada/svg", "/app/json-scada/conf", "/etc"]

# ==============================================================================
# HEALTHCHECK
# ==============================================================================
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD curl -f http://localhost:8080 || exit 1

# ==============================================================================
# ENTRYPOINT
# ==============================================================================
WORKDIR /app

CMD ["/usr/bin/supervisord", "-c", "/etc/supervisor/supervisord.conf"]
