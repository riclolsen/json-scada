#!/bin/bash

# INSTALL SCRIPT FOR JSON-SCADA ON RHEL9 AND COMPATIBLE PLATFORMS
# username is supposed to be jsonscada
JS_USERNAME=jsonscada
JS_ARCH=amd64

# Execute commands below to prepare for this script:
# sudo dnf -y install git
# cd /home/jsonscada
# git clone --recurse-submodules https://github.com/riclolsen/json-scada --config core.autocrlf=input
# cd json-scada/platform-rhel9
# sudo sh ./json-scada-install.sh

# AFTER INSTALLATION
# OPEN BROWSER AT http://localhost (must allow popups for issuing controls)
# Default credentials: admin / jsonscada
# Metabase credentials: json@scada.com / jsonscada123

sudo -u $JS_USERNAME sh -c 'mkdir -p ../log'
sudo chmod +x *.sh

# Re-running this script over an existing installation upgrades it: databases, conf/ and locally
# changed configuration files are kept (see ../platform-linux/install-lib.sh).
. ../platform-linux/install-lib.sh
js_begin


sudo dnf -y update 
sudo dnf -y group install --with-optional "Development Tools" ".NET Development" maven
sudo dnf -y remove golang nodejs java-17-openjdk-headless java-1.8.0-openjdk-headless inkscape
sudo dnf -qy module disable postgresql nodejs
sudo subscription-manager repos --enable codeready-builder-for-rhel-9-$(arch)-rpms
sudo dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm 
sudo dnf -y install epel-release 
sudo dnf config-manager --set-enabled crb
sudo dnf -y install tar vim nano nginx wget chkconfig dotnet-sdk-8.0 java-21-openjdk php cmake libpcap-devel cyrus-sasl-lib cyrus-sasl-devel python3-tkinter sqlite-devel python3.12 python3.12-pip
sudo dnf -y install curl --allowerasing
sudo dnf -y install policycoreutils-python-utils setools-console audit

# docker/podman can be used to run DNP3 and OPC-DA on linux
sudo dnf install -y yum-utils
sudo yum-config-manager --add-repo https://download.docker.com/linux/rhel/docker-ce.repo
sudo dnf -y install podman docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
sudo systemctl enable docker
sudo systemctl start docker
#sudo usermod -aG docker $JS_USERNAME
#sudo chmod 777 /var/run/docker.sock

sudo dnf install --nogpgcheck https://dl.fedoraproject.org/pub/epel/epel-release-latest-$(rpm -E %rhel).noarch.rpm
sudo dnf install -y --nogpgcheck https://mirrors.rpmfusion.org/free/el/rpmfusion-free-release-$(rpm -E %rhel).noarch.rpm https://mirrors.rpmfusion.org/nonfree/el/rpmfusion-nonfree-release-$(rpm -E %rhel).noarch.rpm
sudo dnf install -y ffmpeg ffmpeg-devel
sudo dnf remove -y python3-circuitbreaker

#sudo dnf install -y flatpak
#flatpak remote-add --user --if-not-exists flathub https://flathub.org/repo/flathub.flatpakrepo
#flatpak install --user flathub org.inkscape.Inkscape -y
#sudo -u $JS_USERNAME sh -c 'cp ../src/inkscape-extension/scada.inx ~/.var/app/org.inkscape.Inkscape/config/inkscape/extensions/'
#sudo -u $JS_USERNAME sh -c 'cp ../src/inkscape-extension/scada.py ~/.var/app/org.inkscape.Inkscape/config/inkscape/extensions/'

sudo update-crypto-policies --set LEGACY

wget --inet4-only -N https://go.dev/dl/go1.27.1.linux-$JS_ARCH.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.27.1.linux-$JS_ARCH.tar.gz
sudo -u $JS_USERNAME sh -c 'export PATH=$PATH:/usr/local/go/bin'
js_append_once "$JS_HOME/.bashrc" 'export PATH=$PATH:/usr/local/go/bin' $JS_USERNAME
source ~/.bashrc

# for mongodb 
# https://www.mongodb.com/docs/manual/tutorial/transparent-huge-pages/
# https://www.mongodb.com/docs/manual/administration/production-checklist-operations/
js_append_once /etc/sysctl.conf 'vm.max_map_count=102400'
sudo tee /etc/systemd/system/disable-transparent-huge-pages.service  <<EOF
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

sudo tee /etc/yum.repos.d/mongodb-org-8.0.repo <<EOF
[mongodb-org-8.0]
name=MongoDB Repository
baseurl=https://repo.mongodb.org/yum/redhat/9/mongodb-org/8.2/\$basearch/
gpgcheck=1
enabled=1
gpgkey=https://pgp.mongodb.com/server-8.0.asc
EOF
sudo tee /etc/yum.repos.d/influxdata.repo <<EOF
[influxdata]
name = InfluxData Repository - Stable
baseurl = https://repos.influxdata.com/stable/\$basearch/main
enabled = 1
gpgcheck = 1
gpgkey = https://repos.influxdata.com/influxdata-archive.key
EOF
#sudo tee /etc/yum.repos.d/grafana.repo <<EOF
#[grafana]
#name=grafana
#baseurl=https://packages.grafana.com/oss/rpm
#repo_gpgcheck=1
#enabled=1
#gpgcheck=1
#gpgkey=https://packages.grafana.com/gpg.key
#sslverify=1
#sslcacert=/etc/pki/tls/certs/ca-bundle.crt
#EOF
sudo tee /etc/yum.repos.d/timescale_timescaledb.repo <<EOL
[timescale_timescaledb]
name=timescale_timescaledb
baseurl=https://packagecloud.io/timescale/timescaledb/el/$(rpm -E %{rhel})/\$basearch
repo_gpgcheck=1
gpgcheck=0
enabled=1
gpgkey=https://packagecloud.io/timescale/timescaledb/gpgkey
sslverify=1
sslcacert=/etc/pki/tls/certs/ca-bundle.crt
metadata_expire=300
EOL
sudo dnf -y update 
js_pg_prepare 18
sudo dnf -y install https://download.postgresql.org/pub/repos/yum/reporpms/EL-$(rpm -E %{rhel})-x86_64/pgdg-redhat-repo-latest.noarch.rpm
sudo dnf -y install timescaledb-2-postgresql-18 postgresql18 postgresql18-contrib
sudo dnf -y install timescaledb-toolkit-postgresql-18
[ -n "$JS_PG_KEEP_MAJOR" ] || sudo test -f /var/lib/pgsql/18/data/PG_VERSION || sudo /usr/pgsql-18/bin/postgresql-18-setup initdb
# config postgresql local connections with trust method
[ -n "$JS_PG_KEEP_MAJOR" ] || js_install_config pg_hba.conf /var/lib/pgsql/18/data/pg_hba.conf
[ -n "$JS_PG_KEEP_MAJOR" ] || sudo chown postgres:postgres /var/lib/pgsql/18/data/pg_hba.conf
[ -n "$JS_PG_KEEP_MAJOR" ] || js_install_config postgresql.conf /var/lib/pgsql/18/data/postgresql.conf
[ -n "$JS_PG_KEEP_MAJOR" ] || sudo chown postgres:postgres /var/lib/pgsql/18/data/postgresql.conf
sudo systemctl enable postgresql-${JS_PG_KEEP_MAJOR:-18}
[ "$JS_PG_HAS_DATA" = 1 ] || sudo timescaledb-tune -yes --pg-config=/usr/pgsql-18/bin/pg_config

for JS_F in json_scada_*.conf; do js_install_config $JS_F /etc/nginx/conf.d/$JS_F; done
js_install_config nginx.conf /etc/nginx/nginx.conf
sudo setsebool -P httpd_can_network_connect 1
sudo systemctl enable nginx
sudo systemctl enable php-fpm

sudo dnf -y install mongodb-org 
sudo rpm -e mongodb-org mongodb-mongosh
sudo dnf -y install mongodb-org mongodb-mongosh-shared-openssl3
js_install_config mongod.conf /etc/mongod.conf
sudo systemctl enable mongod

sudo dnf -y install telegraf
for JS_F in telegraf-*.conf; do js_install_config $JS_F /etc/telegraf/telegraf.d/$JS_F; done
sudo systemctl enable telegraf

sudo dnf -y install supervisor
js_install_inis . /etc/supervisord.d
js_install_config supervisord.conf /etc/supervisord.conf
JS_SUPCONF_RC=$?
# JSON-SCADA process manager: dir for driver services created from the AdminUI.
# Owned by jsonscada so services can be managed without root; scanned by supervisord.
sudo -u $JS_USERNAME mkdir -p "$JS_ROOT/conf/supervisor.d"
if ! grep -q 'json-scada/conf/supervisor.d' /etc/supervisord.conf; then
  sudo sed -i "s#^files *= *supervisord.d/\*.ini#files = supervisord.d/*.ini $JS_ROOT/conf/supervisor.d/*.ini#" /etc/supervisord.conf
fi
# installers before 0.72 could point the include at /root
[ "$JS_ROOT" = /root/json-scada ] || sudo sed -i "s#/root/json-scada/conf/supervisor.d#$JS_ROOT/conf/supervisor.d#" /etc/supervisord.conf
[ "$JS_SUPCONF_RC" != 0 ] || js_record /etc/supervisord.conf
sudo systemctl enable supervisord

sudo dnf install -y https://dl.grafana.com/grafana/release/13.2.2/grafana_13.2.2_34846740809_linux_$JS_ARCH.rpm
#sudo dnf -y install grafana
js_install_config grafana.ini /etc/grafana/grafana.ini
sudo systemctl enable grafana-server

sudo -u $JS_USERNAME sh -c 'mkdir -p ../metabase'
sudo -u $JS_USERNAME sh -c 'wget --inet4-only https://downloads.metabase.com/v0.63.17.x/metabase.jar -O ../metabase/metabase.jar'

sudo -u $JS_USERNAME sh -c 'curl -fsSL https://rpm.nodesource.com/setup_24.x -o nodesource_setup.sh'
sudo bash nodesource_setup.sh
sudo dnf -y install nodejs  

sudo systemctl daemon-reload
sudo systemctl restart postgresql-${JS_PG_KEEP_MAJOR:-18}
sudo systemctl restart mongod

if [ "$JS_PG_HAS_DATA" = 1 ]; then
    js_pg_upgrade
else
    psql -U postgres -w -h localhost -f ../sql/create_tables.sql template1
    psql -U postgres -w -h localhost -f ../sql/metabaseappdb.sql metabaseappdb
    psql -U postgres -w -h localhost -f ../sql/grafanaappdb.sql grafanaappdb
    psql -U postgres -w -h localhost -d json_scada -c "CREATE EXTENSION timescaledb_toolkit;"
fi

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

sudo systemctl restart grafana-server

sudo ausearch -c 'mongod' --raw | audit2allow -M my-mongod
sudo semodule -X 300 -i my-mongod.pp

# Install a local Node-RED runtime for the NODE-RED driver.
# The driver also works with a remote or containerized Node-RED.
sudo -u $JS_USERNAME bash -c 'cd ~/json-scada && mkdir -p nodered-runtime && npm install --prefix nodered-runtime node-red node-red-contrib-jsonscada'
sudo -u $JS_USERNAME bash -c '[ -f ~/json-scada/conf/node-red-settings.js ] || cp ~/json-scada/conf-templates/node-red-settings.js ~/json-scada/conf/node-red-settings.js'
sudo -u $JS_USERNAME bash -c 'mkdir -p ~/json-scada/conf/node-red'
# Then enable the nodered_driver (and optionally nodered_runtime) supervisor programs.

cd ../platform-linux 
sudo -u $JS_USERNAME sh -c 'source ~/.bashrc;./build.sh'

sudo systemctl restart php-fpm
sudo systemctl restart nginx
sudo systemctl start supervisord
sudo systemctl restart telegraf

cd /home/jsonscada
sleep 10
[ "$JS_IS_UPGRADE" = 1 ] || sudo supervisorctl status
js_finish
# sudo -u $JS_USERNAME sh -c 'firefox http://localhost &'

echo "To compile and install Inkscape+SAGE, run the following command: sudo sh ./inkscape-plus-sage.sh"
echo "To open web interface run: firefox http://localhost"
echo "Default credentials: admin / jsonscada"
echo "Default Metabase credentials: json@scada.com / jsonscada123"
