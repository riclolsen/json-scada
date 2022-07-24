#cd
#git clone https://github.com/riclolsen/json-scada --config core.autocrlf=input
#cd json-scada/platform-rhel9

sudo dnf -y update 
sudo dnf -y install epel-release 
sudo dnf -y install tar nodejs golang nginx wget chkconfig
sudo update-crypto-policies --set LEGACY
sudo tee /etc/yum.repos.d/mongodb-org-6.0.repo <<EOF
[mongodb-org-6.0]
name=MongoDB Repository
baseurl=https://repo.mongodb.org/yum/redhat/8/mongodb-org/6.0/\$basearch/
gpgcheck=1
enabled=1
gpgkey=https://www.mongodb.org/static/pgp/server-6.0.asc
EOF
sudo tee /etc/yum.repos.d/influxdb.repo <<EOF
[influxdb]
name = InfluxDB Repository - RHEL 
baseurl = https://repos.influxdata.com/rhel/\$releasever/\$basearch/stable/
enabled = 1
gpgcheck = 1
gpgkey = https://repos.influxdata.com/influxdb.key
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
sudo dnf -y install mongodb-org 
sudo cp mongod.conf /etc/
sudo dnf -y install telegraf 
sudo dnf -y install grafana
sudo dnf -y group install "Development Tools" ".NET Development" 

sudo systemctl daemon-reload
sudo systemctl enable supervisor
sudo systemctl enable mongod
sudo systemctl enable telegraf
sudo systemctl start mongod
#sudo systemctl start supervisor
#sudo systemctl start telegraf


