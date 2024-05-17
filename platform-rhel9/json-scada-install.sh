#!/bin/bash

#cd
#git clone https://github.com/riclolsen/json-scada --config core.autocrlf=input
#cd json-scada/platform-rhel9

sudo dnf -y update 
sudo dnf -y install epel-release 
sudo dnf -y install tar vim nano golang nginx wget chkconfig dotnet-sdk-6.0
sudo dnf -y group install --with-optional "Development Tools" ".NET Development" 
sudo update-crypto-policies --set LEGACY

sudo tee /etc/yum.repos.d/mongodb-org-7.0.repo <<EOF
[mongodb-org-7.0]
name=MongoDB Repository
baseurl=https://repo.mongodb.org/yum/redhat/9/mongodb-org/7.0/\$basearch/
gpgcheck=1
enabled=1
gpgkey=https://www.mongodb.org/static/pgp/server-7.0.asc
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
sudo dnf -y install https://download.postgresql.org/pub/repos/yum/reporpms/EL-$(rpm -E %{rhel})-x86_64/pgdg-redhat-repo-latest.noarch.rpm
sudo dnf -y install timescaledb-2-postgresql-16 postgresql16
sudo /usr/pgsql-16/bin/postgresql-16-setup initdb
# config postgresql local connections with trust method
sudo cp pg_hba.conf /var/lib/pgsql/16/data/
sudo chown postgres:postgres /var/lib/pgsql/16/data/pg_hba.conf
sudo cp postgresql.conf /var/lib/pgsql/16/data/
sudo chown postgres:postgres /var/lib/pgsql/16/data/postgresql.conf
sudo systemctl enable postgresql-16

sudo dnf -y install mongodb-org 
sudo cp mongod.conf /etc/
sudo systemctl enable mongod

sudo dnf -y install telegraf
sudo cp telegraf-*.conf /etc/telegraf/telegraf.d/
sudo systemctl enable telegraf

sudo dnf -y install supervisor
sudo systemctl enable supervisor

sudo dnf -y install grafana
sudo systemctl enable grafana-server

sudo cp *.ini /etc/supervisord.d/

mkdir ~/metabase
wget https://downloads.metabase.com/v0.49.10/metabase.jar -O ~/metabase/metabase.jar

# install nvm to be able to choose a specific nodejs version
curl https://raw.githubusercontent.com/creationix/nvm/master/install.sh | bash
source ~/.bashrc
nvm install 20.13.1
npm install -g npm

cd ../platform-linux 
./build.sh

sudo systemctl daemon-reload
sudo systemctl start postgresql-16
sudo systemctl start mongod
#sudo systemctl start grafana
#sudo systemctl start supervisor
#sudo systemctl start telegraf

psql -U postgres -h 127.0.0.1 -f ../sql/create_tables.sql template1
mongosh json_scada < ../mongo_seed/a_rs-init.js
mongosh json_scada < ../mongo_seed/b_create-db.js
mongoimport --db json_scada --collection protocolDriverInstances --type json --file ../demo-docker/mongo_seed/files/demo_instances.json 
mongoimport --db json_scada --collection protocolConnections --type json --file ../demo-docker/mongo_seed/files/demo_connections.json 
mongoimport --db json_scada --collection realtimeData --type json --file ../demo-docker/mongo_seed/files/demo_data.json 
mongoimport --db json_scada --collection processInstances --type json --file ../demo-docker/mongo_seed/files/demo_process_instances.json 
mongoimport --db json_scada --collection users --type json --file ../demo-docker/mongo_seed/files/demo_users.json 
mongoimport --db json_scada --collection roles --type json --file ../demo-docker/mongo_seed/files/demo_roles.json 
mongosh json_scada --eval "db.realtimeData.updateMany({_id:{\$gt:0}},{\$set:{dbId:'demo'}})"

