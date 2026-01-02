#!/bin/bash

# INSTALL SCRIPT FOR JSON-SCADA ON UBUNTU 24.04
# username is supposed to be jsonscada
JS_USERNAME=jsonscada

# Execute commands below to prepare for this script:
# sudo apt update
# sudo apt -y install git
# cd /home/jsonscada
# git clone --recurse-submodules https://github.com/riclolsen/json-scada --config core.autocrlf=input
# cd json-scada/platform-ubuntu-2404
# sudo sh ./json-scada-install.sh

# AFTER INSTALLATION
# OPEN BROWSER AT http://localhost (must allow popups for issuing controls)
# Default credentials: admin / jsonscada
# Metabase credentials: json@scada.com / jsonscada123

ARCHITECTURE="amd64"
case $(uname -m) in
    x86_64) ARCHITECTURE="amd64" ;;
    arm)    ARCHITECTURE="arm64";;
esac

sudo -u $JS_USERNAME sh -c 'mkdir ../log'

# Update and install base packages
sudo apt update
sudo apt -y upgrade
sudo apt -y install ffmpeg bzip2 tar build-essential dotnet-sdk-8.0 openjdk-21-jdk php-fpm nginx wget curl vim nano cmake libpcap-dev sasl2-bin libsasl2-dev

# Docker and container tools
sudo apt -y remove containerd.io
sudo apt -y install podman docker.io
sudo systemctl enable docker
sudo systemctl start docker

# Inkscape build dependencies
sudo apt -y install ninja-build libjpeg-dev libxslt-dev libgtkmm-3.0-dev libboost-all-dev \
    libpoppler-dev libpoppler-glib-dev libgtest-dev libharfbuzz-dev libwpg-dev librevenge-dev libvisio-dev \
    libcdr-dev libreadline-dev libmagick++-dev libgraphicsmagick++1-dev libpango1.0-dev libgsl-dev \
    libsoup2.4-dev liblcms2-dev libgc-dev libdouble-conversion-dev potrace python3-scour
sudo apt -y install libgspell-1-dev libgspell-1-2 libpotrace-dev libpoppler-private-dev

# Install Go
wget --inet4-only https://go.dev/dl/go1.23.4.linux-$ARCHITECTURE.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.4.linux-$ARCHITECTURE.tar.gz
sudo -u $JS_USERNAME sh -c 'export PATH=$PATH:/usr/local/go/bin'
sudo -u $JS_USERNAME sh -c 'echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.bashrc'

# MongoDB configuration
sudo sh -c 'echo "vm.max_map_count=102400" >> /etc/sysctl.conf'
sudo tee /etc/systemd/system/disable-transparent-huge-pages.service  <<EOF
[Unit]
Description=Disable Transparent Huge Pages (THP)
DefaultDependencies=no
After=sysinit.target local-fs.target
Before=mongod.service

[Service]
Type=oneshot
ExecStart=/bin/sh -c 'echo never | tee /sys/kernel/mm/transparent_hugepage/enabled > /dev/null'

[Install]
WantedBy=basic.target
EOF
sudo systemctl enable disable-transparent-huge-pages
sudo systemctl daemon-reload
sudo systemctl start disable-transparent-huge-pages

# Add repositories
# MongoDB
wget -qO - https://www.mongodb.org/static/pgp/server-8.0.asc | sudo gpg --dearmor -o /usr/share/keyrings/mongodb-archive-keyring.gpg
echo "deb [arch=amd64,arm64 signed-by=/usr/share/keyrings/mongodb-archive-keyring.gpg] https://repo.mongodb.org/apt/ubuntu $(lsb_release -cs)/mongodb-org/8.0 multiverse" | sudo tee /etc/apt/sources.list.d/mongodb-org-8.0.list

# Grafana
wget -qO- https://packages.grafana.com/gpg.key | sudo gpg --dearmor -o /usr/share/keyrings/grafana-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/grafana-archive-keyring.gpg] https://packages.grafana.com/oss/deb stable main" | sudo tee /etc/apt/sources.list.d/grafana.list

# Install Postgresql/Timescaledb packages
sudo apt update
sudo apt -y install gnupg postgresql-common apt-transport-https lsb-release
sudo /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh -y
sudo apt -y install postgresql-server-dev-17
echo "deb https://packagecloud.io/timescale/timescaledb/ubuntu/ $(lsb_release -c -s) main" | sudo tee /etc/apt/sources.list.d/timescaledb.list
wget --quiet -O - https://packagecloud.io/timescale/timescaledb/gpgkey | sudo gpg --dearmor -o /etc/apt/trusted.gpg.d/timescaledb.gpg --yes
sudo apt update
sudo apt -y install timescaledb-2-postgresql-17 postgresql-client-17
sudo timescaledb-tune -yes
sudo systemctl enable postgresql
sudo systemctl restart postgresql

sudo cp pg_hba.conf /etc/postgresql/17/main/
sudo chown postgres:postgres /etc/postgresql/17/main/pg_hba.conf
sudo cp postgresql.conf /etc/postgresql/17/main/
sudo chown postgres:postgres /etc/postgresql/17/main/postgresql.conf
sudo systemctl restart postgresql

# Install Inkscape and SCADA extension
# sudo apt -y install inkscape
sudo add-apt-repository -y universe
sudo add-apt-repository -y ppa:inkscape.dev/stable
sudo apt-get update
sudo apt -y install inkscape python3-tk

sudo -u $JS_USERNAME sh -c 'cp ../src/inkscape-extension/scada.inx ~/.config/inkscape/extensions/'
sudo -u $JS_USERNAME sh -c 'cp ../src/inkscape-extension/scada.py ~/.config/inkscape/extensions/'

# Configure web server
sudo cp json_scada_*.conf /etc/nginx/conf.d/
sudo cp nginx.conf /etc/nginx/
sudo systemctl enable nginx
sudo systemctl enable php8.3-fpm

# Install MongoDB
sudo apt -y install mongodb-org
sudo cp mongod.conf /etc/
sudo systemctl enable mongod

# Install monitoring tools
curl --silent --location -O \
https://repos.influxdata.com/influxdata-archive.key \
&& echo "943666881a1b8d9b849b74caebf02d3465d6beb716510d86a39f6c8e8dac7515  influxdata-archive.key" \
| sha256sum -c - && cat influxdata-archive.key \
| gpg --dearmor \
| sudo tee /etc/apt/trusted.gpg.d/influxdata-archive.gpg > /dev/null \
&& echo 'deb [signed-by=/etc/apt/trusted.gpg.d/influxdata-archive.gpg] https://repos.influxdata.com/debian stable main' \
| sudo tee /etc/apt/sources.list.d/influxdata.list
sudo apt-get update && sudo apt-get -y install telegraf
sudo cp telegraf-*.conf /etc/telegraf/telegraf.d/
sudo systemctl enable telegraf

# Install supervisor
sudo apt -y install supervisor
sudo cp supervisord.conf /etc/supervisor/
sudo cp *.ini /etc/supervisor/conf.d/
sudo systemctl enable supervisor

# Install Grafana
sudo apt -y install grafana
sudo cp grafana.ini /etc/grafana/
sudo systemctl enable grafana-server
sudo systemctl daemon-reload

# Install Metabase
sudo -u $JS_USERNAME sh -c 'mkdir ../metabase'
sudo -u $JS_USERNAME sh -c 'wget --inet4-only https://downloads.metabase.com/v0.52.5/metabase.jar -O ../metabase/metabase.jar'

# Install Node.js
curl -fsSL https://deb.nodesource.com/setup_22.x | sudo -E bash -
sudo apt -y install nodejs

# Start services
sudo systemctl daemon-reload
sudo systemctl start postgresql
sudo systemctl start mongod

sleep 5

# Initialize databases
sudo -u postgres psql -c "CREATE DATABASE template1;"
sudo -u postgres psql -f ../sql/create_tables.sql template1
sudo -u postgres psql -c "CREATE DATABASE metabaseappdb;"
sudo -u postgres psql -f ../sql/metabaseappdb.sql metabaseappdb
sudo -u postgres psql -c "CREATE DATABASE grafanaappdb;"
sudo -u postgres psql -f ../sql/grafanaappdb.sql grafanaappdb
sudo -u postgres psql -d json_scada -c "CREATE EXTENSION timescaledb_toolkit;"

# Initialize MongoDB
mongosh json_scada < ../mongo_seed/a_rs-init.js
mongosh json_scada < ../mongo_seed/b_create-db.js
mongoimport --db json_scada --collection protocolDriverInstances --type json --file ../demo-docker/mongo_seed/files/demo_instances.json
mongoimport --db json_scada --collection protocolConnections --type json --file ../demo-docker/mongo_seed/files/demo_connections_linux.json
mongoimport --db json_scada --collection realtimeData --type json --file ../demo-docker/mongo_seed/files/demo_data.json
mongoimport --db json_scada --collection processInstances --type json --file ../demo-docker/mongo_seed/files/demo_process_instances.json
mongoimport --db json_scada --collection users --type json --file ../demo-docker/mongo_seed/files/demo_users.json
mongoimport --db json_scada --collection roles --type json --file ../demo-docker/mongo_seed/files/demo_roles.json
mongosh json_scada --eval "db.realtimeData.updateMany({_id:{\$gt:0}},{\$set:{dbId:'demo'}})"

# Start Grafana
sudo systemctl start grafana-server

# Build JSON-SCADA
cd ../platform-linux
sudo -u $JS_USERNAME bash -c 'source ~/.bashrc;export PATH=$PATH:/usr/local/go/bin;./build.sh'

# Start remaining services
sudo systemctl start php8.3-fpm
sudo systemctl restart nginx
sudo systemctl start supervisor
sudo systemctl start telegraf

# Final status check
cd /home/jsonscada
sleep 10
sudo supervisorctl reload
sudo supervisorctl start all
sudo supervisorctl status

echo "Installation complete!"
# echo "To compile and install Inkscape+SAGE, run: sudo sh ./inkscape-plus-sage.sh"
echo "To open web interface run: firefox http://localhost"
echo "Default credentials: admin / jsonscada"
echo "Default Metabase credentials: json@scada.com / jsonscada123"
