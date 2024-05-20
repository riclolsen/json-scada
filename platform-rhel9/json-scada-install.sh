#!/bin/bash

# INSTALL SCRIPT FOR JSON-SCADA ON RHEL9 AND COMPATIBLE PLATFORMS
# username is suppposed to be jsonscada
# use git clone to extract repo to /home/jsonscada/json-scada
# go to ~/json-scada/platform-rhel9 and run this script

#sudo dnf -y install git
#cd
#git clone https://github.com/riclolsen/json-scada --config core.autocrlf=input
#cd json-scada/platform-rhel9
#sudo sh ./json-scada-install.sh

mkdir ../log 

sudo dnf -y update 
sudo dnf -y group install --with-optional "Development Tools" ".NET Development" 
sudo dnf -y remove golang nodejs java-1.8.0-openjdk-headless
sudo subscription-manager repos --enable codeready-builder-for-rhel-9-$(arch)-rpms
sudo dnf -y install https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm 
sudo dnf -y install epel-release 
sudo dnf -y install tar vim nano nginx wget chkconfig dotnet-sdk-6.0 java-21-openjdk php
sudo update-crypto-policies --set LEGACY

wget https://go.dev/dl/go1.22.3.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.22.3.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
echo "export PATH=\$PATH:/usr/local/go/bin" >> ~/.bashrc

# for mongodb 
# https://www.mongodb.com/docs/manual/tutorial/transparent-huge-pages/
# https://www.mongodb.com/docs/manual/administration/production-checklist-operations/
sudo sh -c 'echo "vm.max_map_count=102400" >> /etc/sysctl.conf'
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

sudo tee /etc/yum.repos.d/mongodb-org-7.0.repo <<EOF
[mongodb-org-7.0]
name=MongoDB Repository
baseurl=https://repo.mongodb.org/yum/redhat/9/mongodb-org/7.0/\$basearch/
gpgcheck=1
enabled=1
gpgkey=https://pgp.mongodb.com/server-7.0.asc
EOF
sudo tee /etc/yum.repos.d/influxdata.repo <<EOF
[influxdata]
name = InfluxData Repository - Stable
baseurl = https://repos.influxdata.com/stable/\$basearch/main
enabled = 1
gpgcheck = 1
gpgkey = https://repos.influxdata.com/influxdata-archive_compat.key
EOF
sudo tee /etc/yum.repos.d/grafana.repo <<EOF
[grafana]
name=grafana
baseurl=https://packages.grafana.com/oss/rpm
repo_gpgcheck=1
enabled=1
gpgcheck=1
gpgkey=https://packages.grafana.com/gpg.key
sslverify=1
sslcacert=/etc/pki/tls/certs/ca-bundle.crt
EOF
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
sudo dnf -y install https://download.postgresql.org/pub/repos/yum/reporpms/EL-$(rpm -E %{rhel})-x86_64/pgdg-redhat-repo-latest.noarch.rpm
sudo dnf -y install timescaledb-2-postgresql-16 postgresql16 postgresql16-contrib
sudo /usr/pgsql-16/bin/postgresql-16-setup initdb
# config postgresql local connections with trust method
sudo cp pg_hba.conf /var/lib/pgsql/16/data/
sudo chown postgres:postgres /var/lib/pgsql/16/data/pg_hba.conf
sudo cp postgresql.conf /var/lib/pgsql/16/data/
sudo chown postgres:postgres /var/lib/pgsql/16/data/postgresql.conf
sudo systemctl enable postgresql-16

sudo cp json_scada_*.conf /etc/nginx/conf.d/
sudo cp nginx.conf /etc/nginx/
sudo setsebool -P httpd_can_network_connect 1
sudo systemctl enable nginx

sudo dnf -y install mongodb-org 
sudo rpm -e mongodb-org mongodb-mongosh
sudo dnf -y install mongodb-org mongodb-mongosh-shared-openssl3
sudo cp mongod.conf /etc/
sudo systemctl enable mongod
sudo ausearch -c 'mongod' --raw | audit2allow -M my-mongod
sudo semodule -X 300 -i my-mongod.pp


sudo dnf -y install telegraf
sudo cp telegraf-*.conf /etc/telegraf/telegraf.d/
sudo systemctl enable telegraf

sudo dnf -y install supervisor
sudo cp *.ini /etc/supervisord.d/
sudo systemctl enable supervisord

sudo dnf -y install grafana-9.5.18
sudo cp grafana.ini /etc/grafana
sudo systemctl enable grafana-server

mkdir ../metabase
wget https://downloads.metabase.com/v0.49.10/metabase.jar -O ../metabase/metabase.jar

# install nvm to be able to choose a specific nodejs version
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash
source ~/.bashrc
nvm install 20.13.1
npm install -g npm

sudo systemctl daemon-reload
sudo systemctl start postgresql-16
sudo systemctl start mongod

psql -U postgres -w -h localhost -f ../sql/create_tables.sql template1
psql -U postgres -w -h localhost -f ../sql/metabaseappdb.sql metabaseappdb
psql -U postgres -w -h localhost -f ../sql/grafanaappdb.sql grafanaappdb

mongosh json_scada < ../mongo_seed/a_rs-init.js
mongosh json_scada < ../mongo_seed/b_create-db.js
mongoimport --db json_scada --collection protocolDriverInstances --type json --file ../demo-docker/mongo_seed/files/demo_instances.json 
mongoimport --db json_scada --collection protocolConnections --type json --file ../demo-docker/mongo_seed/files/demo_connections_linux.json 
mongoimport --db json_scada --collection realtimeData --type json --file ../demo-docker/mongo_seed/files/demo_data.json 
mongoimport --db json_scada --collection processInstances --type json --file ../demo-docker/mongo_seed/files/demo_process_instances.json 
mongoimport --db json_scada --collection users --type json --file ../demo-docker/mongo_seed/files/demo_users.json 
mongoimport --db json_scada --collection roles --type json --file ../demo-docker/mongo_seed/files/demo_roles.json 
mongosh json_scada --eval "db.realtimeData.updateMany({_id:{\$gt:0}},{\$set:{dbId:'demo'}})"

sudo systemctl start grafana-server

cd ../platform-linux 
./build.sh

sudo systemctl start supervisord
sudo systemctl start telegraf
