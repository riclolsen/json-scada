# Installing JSON-SCADA

## Compatible Hardware/OS Platforms

* Windows 10/11 (Home, Pro, Enterprise or IoT) or Server 2019+ x86-64 bits (installer available).

* Most modern Linux x86-64 bits. Recommend Red Hat 9/10 or equivalent such as Rocky, Alma or Oracle Linux (installer script).

* Linux ARM 64 bits.

* Linux ARM 32 bits (only for protocol drivers). MongoDB does not support any 32 bit OS.

* If using VirtualBox, configure "paravirtualization interface"=KVM, otherwise Nodejs errors may occur.

* MongoDB requires AVX instructions on x86 CPU.

* At least 8 GB of RAM and 15 GB of free disk space, dual core CPU.

A full system can run on a single commodity x86 computer but for high performance and high availability on big systems (> 50.000 tags) it is strongly recommended the following hardware

* Modern Intel Xeon, AMD Epyc or Threadripper server processors with 8+ cores.

* 32 GB RAM or more.

* Exclusive data disks (XFS formatted on Linux) on 512GB+ NVMe SSDs for MongoDB (RAID-1 mirrored for high availability).

* Exclusive data disks on 1TB+ NVMe SSDs for PostgreSQL (RAID-1 mirrored for high availability).

* MongoDB replica set cluster with at least 3 servers (one node can be just arbiter). See https://www.mongodb.com/docs/manual/core/replica-set-architectures/.

* 2 PostgreSQL servers with replication.

For very large systems (like with more than 200.000 tags), a MongoDB sharded cluster may be needed.

## Windows Installer

The Windows Installer has everything needed to run the system (MongoDB, PostgreSQL, Grafana, Metabase, Nginx, etc.). It is available in the [releases section](https://github.com/riclolsen/json-scada/releases/).

### REQUIREMENTS

* Windows 10/11 64 bits or Server >=2019, Windows PowerShell. At least 15GB of free space in the "C:" drive.
* Administrative rights. corporate Windows policies may cause problems with the creation of services and the opening of TCP ports.
* Free TCP ports 6688, 6689, 27017, 5432, 80, 8080, 3000, 3001, 9000. Other ports may be required for optional services and protocols.
* If the server already has MongoDB, PostgreSQL, Grafana, Nginx or another webserver, please uninstall, disable or watch out for possible conflicts.
* Do not update previously installed JSON-SCADA. Please uninstall previous JSON-SCADA versions before installing a new version.

### QUICKSTART

To quickly run the system after installed, open the JSON-SCADA desktop folder and:

  - On the JSON-SCADA desktop folder: execute "_Start_Services".
  - On the JSON-SCADA desktop folder: execute "_JSON SCADA WEB".

The default credentials are user=admin, password=jsonscada. Metabase credentials: username=json@scada.com password=jsonscada123.

The system is preconfigured to connect to a online demo simulation via IEC60870-5-104 protocol with an example point list and some displays.

To issue a command, open the Display Viewer, click on a breaker and push the "Command" button then choose an action like "open" or "close" and push the action button.

The SVG display files are in "c:\json-scada\svg\". The configuration files are in "c:\json-scada\conf\".

To edit and create new SVG displays, use the Inkscape+SAGE (shortcut in the JSON-SCADA folder). 

By default, the system is configured to allow HTTP access only by the local machine.
To allow other IP addresses edit the "c:\json-scada\conf\nginx_access_control.conf" file.
To configure safe remote client access, configure IP address access control, HTTPS, client certificates directly in the Nginx configuration files in "c:\json-scada\conf\".

For more info about configuration please read the protocols documentation.

## RHEL 9/10 and compatible systems (Rocky/Alma/Oracle Linux), scripted installation

Execute commands below for scripted installation:

    # firstly create a user named "jsonscada" that can do "sudo". Login as "jsonscada".

    sudo adduser jsonscada
    sudo usermod -aG wheel jsonscada
    sudo usermod -aG docker jsonscada
    sudo passwd jsonscada
    sudo su - jsonscada

    # if necessary, add the below line to the end of the file /etc/sudoers.
    jsonscada ALL=(ALL) ALL   # Change the user name before you issue the commands

    # next, clone the json-scada repo

    sudo dnf -y install git
    cd /home/jsonscada
    git clone --recurse-submodules https://github.com/riclolsen/json-scada --config core.autocrlf=input
    cd json-scada/platform-rhel9
    # or 
    # cd json-scada/platform-rhel10

    # on x86-64 platform run
    sudo sh ./json-scada-install.sh

    # on ARM64 platform instead run
    # sudo sh ./json-scada-install-aarch64.sh 

    # to compile and install Inkscape+SAGE, run the following command: 
    sudo sh ./inkscape-plus-sage.sh

## Ubuntu 24.04/26.04, scripted installation

Execute commands below for scripted installation:

    # firstly create a user named "jsonscada" that can do "sudo". Login as "jsonscada".

    sudo adduser jsonscada
    sudo usermod -aG sudo jsonscada
    sudo usermod -aG docker jsonscada
    sudo passwd jsonscada
    sudo su - jsonscada

    # if necessary, add the below line to the end of the file /etc/sudoers.
    jsonscada ALL=(ALL) ALL   # Change the user name before you issue the commands

    # next, clone the json-scada repo

    sudo apt install -y git
    cd /home/jsonscada
    git clone --recurse-submodules https://github.com/riclolsen/json-scada --config core.autocrlf=input
    cd json-scada/platform-ubuntu-2404
    # or 
    # cd json-scada/platform-ubuntu-2604

    # on x86-64 or ARM64 platform run
    sudo sh ./json-scada-install.sh

    # to compile and install Inkscape+SAGE, run the following command: 
    sudo sh ./inkscape-plus-sage.sh

## Manual Installation

To install JSON-SCADA manually, it is required to install all the requirements first. There is no point reproducing original installation instructions for each upstream project. Here are links and relevant information specific to JSON-SCADA.

### 1. MongoDB Server

Versions 8.x.x - Lower versions are not tested or supported. The Docker image uses 9.x; newer versions can work but were not tested.

* https://www.mongodb.com/try/download/community
* https://docs.mongodb.com/manual/installation/

The _MongoDB Atlas_ cloud service is also compatible.

The _Replica Set_ feature must be enabled, even when just one server is used because this is necessary for Change Streams to work.

MongoDB supports many architectures, it is very flexible. You can deploy on just one server, on the classic 3 member replica set or on a big sharded cluster (with MongoS and config servers).

On untrusted networks or networks open to the Internet it is important to use TLS for MongoDB connections. Consult the MongoDB docs to learn how to configure connections using certificates. Configure "c:\json-scada\conf\json-scada.json" accordingly. MongoDB user auth is optional.

* https://docs.mongodb.com/manual/tutorial/configure-ssl/
* https://www.mongodb.com/docs/manual/core/authentication/

### 2. PostgreSQL / TimescaleDB

PostgreSQL version 18. TimescaleDB version 2.x.x. Previous versions can work but are not recommended. Newer versions can work but were not tested.

* https://www.timescale.com/products
* https://docs.timescale.com/latest/getting-started/installation
* https://www.postgresql.org/download/

On untrusted networks or networks open to the Internet it is important to secure PostgreSQL connections. Consult the PostgreSQL docs to learn how to configure connections using certificates.

* https://www.postgresql.org/docs/18/ssl-tcp.html

Replication to a Standby server is recommended for high availability.

* https://www.postgresql.org/docs/18/different-replication-solutions.html
* https://www.postgresql.org/docs/18/warm-standby.html#STANDBY-SERVER-OPERATION

### 3. Grafana

Grafana version 13.x.x, OSS or Enterprise editions. Previous versions may work but are not recommended.

* https://grafana.com/grafana/download
* https://grafana.com/docs/grafana/latest/installation/

If certificates are configured for PostgreSQL connections to the server, it must be also configured in the Grafana PostgreSQL data source to access the historian data.

### 4. Node.js

* Node.js version 24.x.x or later. Previous versions may work, but are not tested or supported.
* https://nodejs.org/en/

### 5. Golang

* Golang version 1.26.x or later. Previous versions may work, but are not tested or supported.
* https://golang.org/dl/

### 6. DotNet

* DotNet version 8.0.x. Previous versions are not tested or supported.
* https://dotnet.microsoft.com/download

### 7. Other recommended software tools

* Inkscape SAGE or SCADAvis.io SVG Editor for synoptic display creation - https://www.microsoft.com/en-us/p/scadavisio-synoptic-editor/9p9905hmkz7x . Available only for Windows.
* OpenSSL 3.x (for Windows) - https://slproweb.com/products/Win32OpenSSL.html.
* NSSM (for Windows) - https://nssm.cc/
* Visual Studio Community (for Windows) - https://visualstudio.microsoft.com/vs/community/
* Supervisor (for Linux) - http://supervisord.org/installing.html
* Visual Studio Code - https://code.visualstudio.com/
* Nginx or some other web server - https://nginx.org/.
* MongoDB Compass - https://www.mongodb.com/products/compass
* Git - https://git-scm.com/
* Metabase - https://www.metabase.com/start/oss/

## JSON-SCADA Processes - Build and Setup

Download the code from the online repo

* https://github.com/riclolsen/json-scada

Or do a git clone

    git clone --recurse-submodules https://github.com/riclolsen/json-scada --config core.autocrlf=input

Build the code using the adequate script for the host platform:

* https://github.com/riclolsen/json-scada/blob/master/platform-linux/build.sh

* https://github.com/riclolsen/json-scada/blob/master/platform-windows/build.bat

Configure the conf/json-scada.json file to define the node name and to point to the MongoDB server. Processes will look for the config file on the _APPDIR_/conf/ folder.

* [Config File Documentation](../conf/README.md)

Initiate/seed the MongoDB database.

* https://github.com/riclolsen/json-scada/blob/master/mongo_seed/init.sh

Initiate the Postgresql database.

    cd json-scada/sql
    psql -h 127.0.0.1 -U postgres -w -f create_tables.sql

* https://github.com/riclolsen/json-scada/blob/master/sql/create_tables.sql

Processes can be distributed on distinct servers, each server must have a different node name.

Multiple JSON-SCADA systems can run on the same server; each one must have distinct MongoDB and PostgreSQL databases, a separate folder structure and distinct HTTP listen ports.

It is recommended to run JSON-SCADA processes as services or daemons. On Linux, use the _Supervisor_ tool to manage processes. On Windows, convert processes into Windows services with the NSSM tool.

### Supervisor Configuration (Linux)

Install supervisor for your OS

* http://supervisord.org/installing.html

The platform installers already install supervisor and copy the program definitions:

* Ubuntu: `platform-ubuntu-*/supervisord.conf` -> `/etc/supervisor/supervisord.conf`,
  and `platform-ubuntu-*/*.ini` -> `/etc/supervisor/conf.d/`.
* RHEL: `platform-rhel*/supervisord.conf` -> `/etc/supervisord.conf`, and
  `platform-rhel*/*.ini` -> `/etc/supervisord.d/`.

For a manual installation, do the same with the directory for your distribution
(`platform-rhel9`, `platform-rhel10`, `platform-ubuntu-2404` or `platform-ubuntu-2604`):

    sudo apt install -y supervisor          # or: sudo dnf install -y supervisor
    sudo cp platform-ubuntu-2404/supervisord.conf /etc/supervisor/supervisord.conf
    sudo cp platform-ubuntu-2404/*.ini /etc/supervisor/conf.d/

    # or, on RHEL-compatible systems:
    sudo cp platform-rhel9/supervisord.conf /etc/supervisord.conf
    sudo cp platform-rhel9/*.ini /etc/supervisord.d/

Adjust the `/home/jsonscada/json-scada` paths and the `user=` line in the `.ini` files
if you installed elsewhere. Programs that ship with `autostart=false` stay stopped until
you start them.

Execute the supervisor daemon:

    sudo supervisord -c /etc/supervisor/supervisord.conf   # RHEL: -c /etc/supervisord.conf

Use the manager tool to start, stop and monitor the system:

    supervisorctl
        start all
        status
        tail -f cs_data_processor
        help

### NSSM Configuration on Windows (not necessary if used the Windows installer)

Install the NSSM tool. It can be installed in "c:\json-scada\platform-windows\".

Use the tool to create necessary services. See "C:\json-scada\platform-windows\create_services.bat".

    cd c:\json-scada\bin
    nssm install JSON_SCADA_calculations "C:\json-scada\bin\calculations.exe"
    nssm install JSON_SCADA_iec104server "C:\json-scada\bin\iec104server.exe"
    nssm install JSON_SCADA_iec104client "C:\json-scada\bin\iec104client.exe"
    nssm install JSON_SCADA_cs_data_processor <PATH_TO_NODEJSEXE>\node "C:\json-scada\src\cs_data_processor\cs_data_processor.js"
    nssm install JSON_SCADA_server_realtime_auth <PATH_TO_NODEJSEXE>\node "C:\json-scada\src\server_realtime_auth\index.js"

    ... and so on ...

To manage services use

    nssm start service_name
    nssm stop service_name
    nssm restart service_name
    nssm status service_name
    nssm remove service_name

    
