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
    aarch64|arm64)    ARCHITECTURE="arm64";;
esac

sudo -u $JS_USERNAME sh -c 'mkdir -p ../log'
sudo chmod +x *.sh

# Re-running this script over an existing installation upgrades it: databases, conf/ and locally
# changed configuration files are kept (see ../platform-linux/install-lib.sh).
. ../platform-linux/install-lib.sh
js_begin

# Update and install base packages
sudo apt update
sudo env DEBIAN_FRONTEND=noninteractive UCF_FORCE_CONFFOLD=1 apt -y upgrade
sudo apt -y install ffmpeg bzip2 tar build-essential openjdk-21-jdk maven php-fpm nginx wget curl vim nano cmake libpcap-dev sasl2-bin libsasl2-dev libsqlite3-dev libzstd-dev python3-venv
sudo apt -y install dotnet-sdk-10.0 

# Docker and container tools
sudo apt -y remove containerd.io
sudo apt -y install podman docker.io
sudo systemctl enable docker
sudo systemctl start docker

# Install Go
wget --inet4-only -N https://go.dev/dl/go1.27.1.linux-$ARCHITECTURE.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.27.1.linux-$ARCHITECTURE.tar.gz
sudo -u $JS_USERNAME sh -c 'export PATH=$PATH:/usr/local/go/bin'
js_append_once "$JS_HOME/.bashrc" 'export PATH=$PATH:/usr/local/go/bin' $JS_USERNAME

# MongoDB configuration
js_append_once /etc/sysctl.conf 'vm.max_map_count=102400'
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
wget -qO - https://www.mongodb.org/static/pgp/server-8.0.asc | sudo gpg --dearmor --yes -o /usr/share/keyrings/mongodb-archive-keyring.gpg
echo "deb [arch=amd64,arm64 signed-by=/usr/share/keyrings/mongodb-archive-keyring.gpg] https://repo.mongodb.org/apt/ubuntu noble/mongodb-org/8.3 multiverse" | sudo tee /etc/apt/sources.list.d/mongodb-org-8.0.list

# Grafana
wget -qO- https://packages.grafana.com/gpg.key | sudo gpg --dearmor --yes -o /usr/share/keyrings/grafana-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/grafana-archive-keyring.gpg] https://packages.grafana.com/oss/deb stable main" | sudo tee /etc/apt/sources.list.d/grafana.list

# Install Postgresql/Timescaledb packages
sudo apt update
js_pg_prepare 18
sudo apt -y install gnupg postgresql-common apt-transport-https lsb-release
sudo /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh -y
sudo apt -y install postgresql-server-dev-18
echo "deb https://packagecloud.io/timescale/timescaledb/ubuntu/ noble main" | sudo tee /etc/apt/sources.list.d/timescaledb.list
wget --quiet -O - https://packagecloud.io/timescale/timescaledb/gpgkey | sudo gpg --dearmor -o /etc/apt/trusted.gpg.d/timescaledb.gpg --yes
sudo apt update
sudo apt -y install timescaledb-2-postgresql-18 postgresql-client-18
[ "$JS_PG_HAS_DATA" = 1 ] || sudo timescaledb-tune -yes
sudo systemctl enable postgresql
sudo systemctl restart postgresql

[ -n "$JS_PG_KEEP_MAJOR" ] || js_install_config pg_hba.conf /etc/postgresql/18/main/pg_hba.conf
[ -n "$JS_PG_KEEP_MAJOR" ] || sudo chown postgres:postgres /etc/postgresql/18/main/pg_hba.conf
[ -n "$JS_PG_KEEP_MAJOR" ] || js_install_config postgresql.conf /etc/postgresql/18/main/postgresql.conf
[ -n "$JS_PG_KEEP_MAJOR" ] || sudo chown postgres:postgres /etc/postgresql/18/main/postgresql.conf
sudo systemctl restart postgresql

## Install Inkscape and SCADA extension
#sudo apt -y remove inkscape
#sudo add-apt-repository -y universe
#sudo add-apt-repository -y ppa:inkscape.dev/stable
#sudo apt-get update
#sudo apt -y install inkscape python3-tk
#
#sudo -u $JS_USERNAME sh -c 'cp ../src/inkscape-extension/scada.inx ~/.config/inkscape/extensions/'
#sudo -u $JS_USERNAME sh -c 'cp ../src/inkscape-extension/scada.py ~/.config/inkscape/extensions/'

# Configure web server
for JS_F in json_scada_http.conf json_scada_https.conf; do js_install_config $JS_F /etc/nginx/conf.d/$JS_F; done
js_install_config nginx.conf /etc/nginx/nginx.conf
sudo systemctl enable nginx
sudo systemctl enable php8.5-fpm

# Install MongoDB
sudo apt -y install mongodb-org
js_install_config mongod.conf /etc/mongod.conf
sudo systemctl enable mongod

# Install monitoring tools
sudo sh -c "wget -q -O - https://repos.influxdata.com/influxdata-archive.key | gpg --dearmor --yes -o /etc/apt/keyrings/influxdata-archive.gpg"
sudo sh -c "echo \"deb [signed-by=/etc/apt/keyrings/influxdata-archive.gpg] https://repos.influxdata.com/debian stable main\" > /etc/apt/sources.list.d/influxdata.list" 
# Keep the existing InfluxData source if its keyring package offers a replacement.
sudo apt-get update && sudo env DEBIAN_FRONTEND=noninteractive UCF_FORCE_CONFFOLD=1 apt-get -y install telegraf
for JS_F in telegraf-*.conf; do js_install_config $JS_F /etc/telegraf/telegraf.d/$JS_F; done
sudo systemctl enable telegraf

# Install supervisor
sudo apt -y install supervisor
js_install_config supervisord.conf /etc/supervisor/supervisord.conf
js_install_inis . /etc/supervisor/conf.d
# JSON-SCADA process manager: dir for driver services created from the AdminUI
# (owned by jsonscada, included by the copied supervisord.conf).
sudo -u $JS_USERNAME mkdir -p "$JS_ROOT/conf/supervisor.d"
sudo systemctl enable supervisor

# Install Grafana
sudo apt -y install --allow-change-held-packages grafana=13.2.2
sudo apt-mark hold grafana
js_install_config grafana.ini /etc/grafana/grafana.ini
sudo systemctl enable grafana-server
sudo systemctl daemon-reload

# Install Metabase
sudo -u $JS_USERNAME sh -c 'mkdir -p ../metabase'
sudo -u $JS_USERNAME sh -c 'wget --inet4-only https://downloads.metabase.com/v0.63.17.x/metabase.jar -O ../metabase/metabase.jar'

# Install Mongodb Compass
sudo -u $JS_USERNAME sh -c "wget -N https://downloads.mongodb.com/compass/mongodb-compass_1.49.14_$ARCHITECTURE.deb"
sudo apt -y install ./mongodb-compass_1.49.14_$ARCHITECTURE.deb

# Install Node.js
curl -fsSL https://deb.nodesource.com/setup_24.x | sudo -E bash -
sudo apt -y install nodejs

# Start services
sudo systemctl daemon-reload
sudo systemctl restart postgresql
sudo systemctl restart mongod

sleep 5

# Initialize new databases, upgrade existing ones
if [ "$JS_PG_HAS_DATA" = 1 ]; then
    js_pg_upgrade
else
    sudo -u postgres psql -c "CREATE DATABASE template1;"
    sudo -u postgres psql -f ../sql/create_tables.sql template1
    sudo -u postgres psql -c "CREATE DATABASE metabaseappdb;"
    sudo -u postgres psql -f ../sql/metabaseappdb.sql metabaseappdb
    sudo -u postgres psql -c "CREATE DATABASE grafanaappdb;"
    sudo -u postgres psql -f ../sql/grafanaappdb.sql grafanaappdb
    sudo -u postgres psql -d json_scada -c "CREATE EXTENSION timescaledb_toolkit;"
fi

# Initialize MongoDB
if js_mongo_has_data; then
    js_mongo_upgrade
else
    mongosh json_scada < ../mongo_seed/a_rs-init.js
    mongosh json_scada < ../mongo_seed/b_create-db.js
    mongoimport --db json_scada --collection protocolDriverInstances --type json --file ../demo-docker/mongo_seed/files/demo_instances.json
    mongoimport --db json_scada --collection protocolConnections --type json --file ../demo-docker/mongo_seed/files/demo_connections_linux.json
    mongoimport --db json_scada --collection realtimeData --type json --file ../demo-docker/mongo_seed/files/demo_data.json
    mongoimport --db json_scada --collection processInstances --type json --file ../demo-docker/mongo_seed/files/demo_process_instances.json
    mongoimport --db json_scada --collection users --type json --file ../demo-docker/mongo_seed/files/demo_users.json
    mongoimport --db json_scada --collection roles --type json --file ../demo-docker/mongo_seed/files/demo_roles.json
    # Svedala CIM-EMS demo (ReliCapGrid model, CC BY-SA 4.0): model, bindings, field and result points
    mongoimport --db json_scada --collection realtimeData --type json --file ../demo-docker/mongo_seed/files/demo_cim_data.json
    mongoimport --db json_scada --collection cimModels --type json --file ../demo-docker/mongo_seed/files/demo_cim_models.json
    mongoimport --db json_scada --collection cimMeasurementBindings --type json --file ../demo-docker/mongo_seed/files/demo_cim_bindings.json
    mongoimport --db json_scada --collection cimArtifacts.files --type json --file ../demo-docker/mongo_seed/files/demo_cim_artifacts_files.json
    mongoimport --db json_scada --collection cimArtifacts.chunks --type json --file ../demo-docker/mongo_seed/files/demo_cim_artifacts_chunks.json
    mongosh json_scada --eval "db.realtimeData.updateMany({_id:{\$gt:0}},{\$set:{dbId:'demo'}})"
fi

# Start Grafana
sudo systemctl restart grafana-server

# Install a local Node-RED runtime for the NODE-RED driver.
# The driver also works with a remote or containerized Node-RED.
sudo -u $JS_USERNAME bash -c 'cd ~/json-scada && mkdir -p nodered-runtime && npm install --prefix nodered-runtime node-red node-red-contrib-jsonscada'
sudo -u $JS_USERNAME bash -c '[ -f ~/json-scada/conf/node-red-settings.js ] || cp ~/json-scada/conf-templates/node-red-settings.js ~/json-scada/conf/node-red-settings.js'
sudo -u $JS_USERNAME bash -c 'mkdir -p ~/json-scada/conf/node-red'
# Then enable the nodered_driver (and optionally nodered_runtime) supervisor programs.

# Build JSON-SCADA
cd ../platform-linux
sudo -u $JS_USERNAME bash -c 'source ~/.bashrc;export PATH=$PATH:/usr/local/go/bin;./build.sh'

# Start remaining services
sudo systemctl restart php8.5-fpm
sudo systemctl restart nginx
sudo systemctl start supervisor
sudo systemctl restart telegraf

# Final status check
cd /home/jsonscada
sleep 10
if [ "$JS_IS_UPGRADE" != 1 ]; then
    sudo supervisorctl reload
    sudo supervisorctl start all
    sudo supervisorctl status
fi
js_finish

echo "To compile and install Inkscape+SAGE, run: sudo sh ./inkscape-plus-sage.sh"
echo "To open web interface run: firefox http://localhost"
echo "Default credentials: admin / jsonscada"
echo "Default Metabase credentials: json@scada.com / jsonscada123"
