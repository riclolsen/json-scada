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

sudo -u $JS_USERNAME sh -c 'mkdir ../log'
sudo chmod +x *.sh

# MONGODB WON'T START ON KERNEL 6.19 OR LATER!
# Use the Ubuntu 24.04 GA kernel (6.8), drop the HWE kernel track and lock the kernel.
# If the running kernel is 6.19 or newer, remove the newer kernels and ask for a reboot.

# Prints 1 if the kernel version given (e.g. 6.19.0-1-generic) is 6.19 or newer.
kernel_too_new() {
    KMAJ=$(echo "$1" | cut -d. -f1)
    KMIN=$(echo "$1" | cut -d. -f2 | sed 's/[^0-9].*//')
    if [ "$KMAJ" -gt 6 ] || { [ "$KMAJ" -eq 6 ] && [ "$KMIN" -ge 19 ]; }; then
        echo 1
    else
        echo 0
    fi
}

sudo apt update
sudo apt -y install linux-generic linux-image-generic linux-headers-generic
sudo apt -y purge linux-generic-hwe-24.04 linux-image-generic-hwe-24.04 linux-headers-generic-hwe-24.04 linux-tools-generic-hwe-24.04
sudo apt-mark hold linux-generic linux-image-generic linux-headers-generic
# Block the HWE kernel metapackages so they cannot be pulled in again.
sudo tee /etc/apt/preferences.d/json-scada-kernel <<EOF
# JSON-SCADA: MongoDB does not start on kernel 6.19 or later, keep the GA kernel.
Package: linux-*-hwe-24.04 linux-hwe-*
Pin: version *
Pin-Priority: -1
EOF

RUNNING_KERNEL=$(uname -r)
if [ "$(kernel_too_new "$RUNNING_KERNEL")" = "1" ]; then
    echo "Running kernel $RUNNING_KERNEL is 6.19 or newer, MongoDB is not compatible with it."

    # Versioned kernel packages (image, modules, headers, tools) of kernels 6.19 or newer.
    NEW_KERNEL_PKGS=""
    OLD_KERNEL_FOUND=0
    for PKG in $(dpkg-query -W -f='${db:Status-Abbrev} ${Package}\n' 'linux-*' 2>/dev/null | awk '$1 ~ /^.i/ {print $2}'); do
        PKG_KVER=$(echo "$PKG" | grep -oE '[0-9]+\.[0-9]+\.[0-9]+-' | head -n 1)
        [ -z "$PKG_KVER" ] && continue
        if [ "$(kernel_too_new "$PKG_KVER")" = "1" ]; then
            NEW_KERNEL_PKGS="$NEW_KERNEL_PKGS $PKG"
        elif echo "$PKG" | grep -qE '^linux-image-(unsigned-)?[0-9]'; then
            OLD_KERNEL_FOUND=1
        fi
    done

    if [ "$OLD_KERNEL_FOUND" != "1" ]; then
        echo "ERROR: no kernel older than 6.19 is installed, cannot remove the newer kernels."
        echo "Install linux-generic (GA kernel) manually and run this script again."
        exit 1
    fi

    echo "Removing kernel packages:$NEW_KERNEL_PKGS"
    # Allow removing the running kernel (the machine is rebooted right after).
    echo "linux-base linux-base/removing-running-kernel boolean false" | sudo debconf-set-selections
    sudo env DEBIAN_FRONTEND=noninteractive apt-get -y purge $NEW_KERNEL_PKGS
    sudo update-grub

    echo ""
    echo "An older kernel was installed and locked, kernels 6.19 or newer were removed."
    echo "A REBOOT IS REQUIRED. After rebooting, run this installation script again."
    printf "Reboot now? [y/N] "
    read REBOOT_ANSWER < /dev/tty
    case "$REBOOT_ANSWER" in
        y|Y|yes|YES) sudo reboot ;;
    esac
    exit 0
fi

# Update and install base packages
sudo apt update
sudo apt -y upgrade
sudo apt -y install ffmpeg bzip2 tar build-essential openjdk-21-jdk maven php-fpm nginx wget curl vim nano cmake libpcap-dev sasl2-bin libsasl2-dev libsqlite3-dev libzstd-dev python3-venv
sudo apt -y install dotnet-sdk-8.0 

# Docker and container tools
sudo apt -y remove containerd.io
sudo apt -y install podman docker.io
sudo systemctl enable docker
sudo systemctl start docker

# Install Go
wget --inet4-only https://go.dev/dl/go1.27.1.linux-$ARCHITECTURE.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.27.1.linux-$ARCHITECTURE.tar.gz
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
echo "deb [arch=amd64,arm64 signed-by=/usr/share/keyrings/mongodb-archive-keyring.gpg] https://repo.mongodb.org/apt/ubuntu $(lsb_release -cs)/mongodb-org/8.3 multiverse" | sudo tee /etc/apt/sources.list.d/mongodb-org-8.0.list

# Grafana
wget -qO- https://packages.grafana.com/gpg.key | sudo gpg --dearmor -o /usr/share/keyrings/grafana-archive-keyring.gpg
echo "deb [signed-by=/usr/share/keyrings/grafana-archive-keyring.gpg] https://packages.grafana.com/oss/deb stable main" | sudo tee /etc/apt/sources.list.d/grafana.list

# Install Postgresql/Timescaledb packages
sudo apt update
sudo apt -y install gnupg postgresql-common apt-transport-https lsb-release
sudo /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh -y
sudo apt -y install postgresql-server-dev-18
echo "deb https://packagecloud.io/timescale/timescaledb/ubuntu/ $(lsb_release -c -s) main" | sudo tee /etc/apt/sources.list.d/timescaledb.list
wget --quiet -O - https://packagecloud.io/timescale/timescaledb/gpgkey | sudo gpg --dearmor -o /etc/apt/trusted.gpg.d/timescaledb.gpg --yes
sudo apt update
sudo apt -y install timescaledb-2-postgresql-18 postgresql-client-18
sudo timescaledb-tune -yes
sudo systemctl enable postgresql
sudo systemctl restart postgresql

sudo cp pg_hba.conf /etc/postgresql/18/main/
sudo chown postgres:postgres /etc/postgresql/18/main/pg_hba.conf
sudo cp postgresql.conf /etc/postgresql/18/main/
sudo chown postgres:postgres /etc/postgresql/18/main/postgresql.conf
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
sudo cp json_scada_http.conf json_scada_https.conf /etc/nginx/conf.d/
sudo cp nginx.conf /etc/nginx/
sudo systemctl enable nginx
sudo systemctl enable php8.3-fpm

# Install MongoDB
sudo apt -y install mongodb-org
sudo cp mongod.conf /etc/
sudo systemctl enable mongod

# Install monitoring tools
sudo sh -c "wget -q -O - https://repos.influxdata.com/influxdata-archive.key | gpg --dearmor -o /etc/apt/keyrings/influxdata-archive.gpg"
sudo sh -c "echo \"deb [signed-by=/etc/apt/keyrings/influxdata-archive.gpg] https://repos.influxdata.com/debian stable main\" > /etc/apt/sources.list.d/influxdata.list" 
sudo apt-get update && sudo apt-get -y install telegraf
sudo cp telegraf-*.conf /etc/telegraf/telegraf.d/
sudo systemctl enable telegraf

# Install supervisor
sudo apt -y install supervisor
sudo cp supervisord.conf /etc/supervisor/
sudo cp *.ini /etc/supervisor/conf.d/
# JSON-SCADA process manager: dir for driver services created from the AdminUI
# (owned by jsonscada, included by the copied supervisord.conf).
mkdir -p ~/json-scada/conf/supervisor.d
sudo systemctl enable supervisor

# Install Grafana
sudo apt -y install grafana=13.2.2
sudo apt-mark hold grafana
sudo cp grafana.ini /etc/grafana/
sudo systemctl enable grafana-server
sudo systemctl daemon-reload

# Install Metabase
sudo -u $JS_USERNAME sh -c 'mkdir ../metabase'
sudo -u $JS_USERNAME sh -c 'wget --inet4-only https://downloads.metabase.com/v0.63.17.x/metabase.jar -O ../metabase/metabase.jar'

# Install Mongodb Compass
sudo -u $JS_USERNAME sh -c "wget https://downloads.mongodb.com/compass/mongodb-compass_1.49.14_$ARCHITECTURE.deb"
sudo apt install ./mongodb-compass_1.49.14_$ARCHITECTURE.deb

# Install Node.js
curl -fsSL https://deb.nodesource.com/setup_24.x | sudo -E bash -
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
# Svedala CIM-EMS demo (ReliCapGrid model, CC BY-SA 4.0): model, bindings, field and result points
mongoimport --db json_scada --collection realtimeData --type json --file ../demo-docker/mongo_seed/files/demo_cim_data.json
mongoimport --db json_scada --collection cimModels --type json --file ../demo-docker/mongo_seed/files/demo_cim_models.json
mongoimport --db json_scada --collection cimMeasurementBindings --type json --file ../demo-docker/mongo_seed/files/demo_cim_bindings.json
mongoimport --db json_scada --collection cimArtifacts.files --type json --file ../demo-docker/mongo_seed/files/demo_cim_artifacts_files.json
mongoimport --db json_scada --collection cimArtifacts.chunks --type json --file ../demo-docker/mongo_seed/files/demo_cim_artifacts_chunks.json
mongosh json_scada --eval "db.realtimeData.updateMany({_id:{\$gt:0}},{\$set:{dbId:'demo'}})"

# Start Grafana
sudo systemctl start grafana-server

# Install a local Node-RED runtime for the NODE-RED driver.
# The driver also works with a remote or containerized Node-RED.
sudo -u $JS_USERNAME bash -c 'cd ~/json-scada && mkdir -p nodered-runtime && npm install --prefix nodered-runtime node-red node-red-contrib-jsonscada'
sudo -u $JS_USERNAME bash -c 'cp ~/json-scada/conf-templates/node-red-settings.js ~/json-scada/conf/node-red-settings.js'
sudo -u $JS_USERNAME bash -c 'mkdir -p ~/json-scada/conf/node-red'
# Then enable the nodered_driver (and optionally nodered_runtime) supervisor programs.

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
echo "To compile and install Inkscape+SAGE, run: sudo sh ./inkscape-plus-sage.sh"
echo "To open web interface run: firefox http://localhost"
echo "Default credentials: admin / jsonscada"
echo "Default Metabase credentials: json@scada.com / jsonscada123"
