# Installing JSON-SCADA

## Compatible Hardware/OS Platforms

* Windows 10/11 (Home, Pro, Enterprise or IoT) or Server 2019+ x86-64 bits (installer available).

* Most modern Linux x86-64 bits. Recommend Redhat 9.4 or equivalent such as Rocky, Alma or Oracle Linux (installer script).

* Linux ARM 64 bits.

* Linux ARM 32 bits (only for protocol drivers). MongoDB does not support any 32 bit OS.

* Mac OSX (x64 Intel or M1).

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

The SVG display files are in "c:\json-scada\src\htdocs\svg\". The configuration files are in "c:\json-scada\conf\".

To edit and create new SVG displays, use the Inkscape+SAGE (shortcut in the JSON-SCADA folder). 

By default, the system is configured to allow HTTP access only by the local machine.
To allow other IP addresses edit the "c:\json-scada\conf\nginx_access_control.conf" file.
To configure safe remote client access, configure IP address access control, HTTPS, client certificates directly in the Nginx configuration files in "c:\json-scada\conf\".

For more info about configuration please read the protocols documentation.

## RHEL9.4 and compatible systems (Rocky/Alma/Oracle Linux), scripted installation

Execute commands below for scripted installation:

    # firstly create a user named "jsonscada" that can do "sudo". Login as "jsonscada".

    sudo adduser jsonscada
    sudo usermod -aG wheel jsonscada
    sudo usermod -aG docker jsonscada
    sudo passwd jsonscada
    sudo su - jsonscada

    # next, clone the json-scada repo

    sudo dnf -y install git
    cd /home/jsonscada
    git clone https://github.com/riclolsen/json-scada --config core.autocrlf=input
    cd json-scada/platform-rhel9

    # on x86-64 platform run
    sudo sh ./json-scada-install.sh

    # on ARM64 platform instead run
    # sudo sh ./json-scada-install-aarch64.sh 

    # to compile and install Inkscape+SAGE, run the following command: 
    sudo sh ./inkscape-plus-sage.sh

## Manual Installation

To install JSON-SCADA manually, it is required to install all the requirements first. There is no point reproducing original installation instructions for each upstream project. Here are links and relevant information specific to JSON-SCADA.

### 1. MongoDB Server

Versions 6.x.x or 7.x.x - Lower versions are not tested or supported. Newer versions can work but were not tested.

* https://www.mongodb.com/try/download/community
* https://docs.mongodb.com/manual/installation/

The _MongoDB Atlas_ cloud service is also compatible.

The _Replica Set_ feature must be enabled, even when just one server is used because this is necessary for Change Streams to work.

MongoDB supports many architectures, it is very flexible. You can deploy on just one server, on the classic 3 member replica set or on a big sharded cluster (with MongoS and config servers).

For not trusted or open to Internet networks it is important to use TLS for MongoDB connections. Consult the MongoDB docs to learn how to config connections using certificates. Configure "c:\json-scada\conf\json-scada.json" accordingly. Mongodb user auth is optional.

* https://docs.mongodb.com/manual/tutorial/configure-ssl/
* https://www.mongodb.com/docs/manual/core/authentication/

### 2. PostgreSQL / TimescaleDB

PostgreSQL version 12 - 16. TimescaleDB version 2.x.x. Previous versions can work but are not recommended. Newer versions can work but were not tested.

* https://www.timescale.com/products
* https://docs.timescale.com/latest/getting-started/installation
* https://www.postgresql.org/download/

For not trusted or open to Internet networks it is important to secure PostgreSQL connections. Consult the PostgreSQL docs to learn how to config connections using certificates.

* https://www.postgresql.org/docs/12/ssl-tcp.html

Replication to a Standby server is recommended for high availability.

* https://www.postgresql.org/docs/12/different-replication-solutions.html
* https://www.postgresql.org/docs/12/warm-standby.html#STANDBY-SERVER-OPERATION

### 3. Grafana

Grafana version 9.x.x, 10.x.x or 11.x.x, OSS or Enterprise editions. Previous versions may work but are not recommended.

* https://grafana.com/grafana/download
* https://grafana.com/docs/grafana/latest/installation/

If certificates are configured for PostgreSQL connections to the server, it must be also configured in the Grafana PostgreSQL data source to access the historian data.

### 4. Node.js

* Node.js version 20.x.x. Previous versions may work, but are not tested or supported.
* https://nodejs.org/en/

### 5. Golang

* Golang version 1.22.x. Previous versions may work, but are not tested or supported.
* https://golang.org/dl/

### 6. DotNet

* DotNet version 6.0.x. Previous versions are not tested or supported.
* https://dotnet.microsoft.com/download

### 7. Other recommended software tools

* Inkscape SAGE or SCADAvis.io SVG Editor for synoptic display creation - https://sourceforge.net/projects/oshmiopensubstationhmi/ or https://www.microsoft.com/en-us/p/scadavisio-synoptic-editor/9p9905hmkz7x . Available only for Windows.
* Open SSL Light 1.1.1m (for Windows) - https://slproweb.com/download/Win64OpenSSL_Light-1_1_1m.msi.
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

    git clone https://github.com/riclolsen/json-scada --config core.autocrlf=input

Build the code using the adequate script for the host platform:

* https://github.com/riclolsen/json-scada/blob/master/platform-linux/build.sh

* https://github.com/riclolsen/json-scada/blob/master/platform-windows/build.bat

Configure the conf/json-scada.json file to define the node name and to point to the MongoDB server. Processes will look for the config file on the ../conf/ folder.

* [Config File Documentation](../conf/README.md)

Initiate/seed the MongoDB database.

* https://github.com/riclolsen/json-scada/blob/master/mongo_seed/init.sh

Initiate the Postgresql database.

    cd json-scada/sql
    psql -h 127.0.0.1 -U postgres -w -f create_tables.sql

* https://github.com/riclolsen/json-scada/blob/master/sql/create_tables.sql

Processes can be distributed on distinct servers, each server must have a different node name.

Multiple JSON-SCADA systems can run on the same server, for this each one must have a distinct MongoDB and PostgreSQL database and a separate folder structure. Also a distinct listen HTTP ports must be configured.

It is recommended to run JSON-SCADA processes as services or daemons. On Linux it is recommended the _Supervisor_ tool to manage processes. On Windows it is recommend to convert processes on Windows services using the NSSM tool.

### Supervisor Configuration (Linux)

Install supervisor for your OS

* http://supervisord.org/installing.html

Configure the _/etc/supervisord.conf_ file to manage JSON-SCADA processes.


    ; Sample supervisor config file.
    ;
    ; For more information on the config file, please see:
    ; http://supervisord.org/configuration.html
    ;

    [inet_http_server]         ; inet (TCP) server disabled by default
    port=127.0.0.1:9000        ; ip_address:port specifier, *:port for all iface
    username=jsonscada         ; default is no username (open server)
    password=secret            ; default is no password (open server)

    [supervisord]
    logfile=/tmp/supervisord.log ; main log file; default $CWD/supervisord.log
    logfile_maxbytes=50MB        ; max main logfile bytes b4 rotation; default 50MB
    logfile_backups=10           ; # of main logfile backups; 0 means none, default 10
    loglevel=info                ; log level; default info; others: debug,warn,trace
    pidfile=/tmp/supervisord.pid ; supervisord pidfile; default supervisord.pid
    nodaemon=false               ; start in foreground if true; default false
    silent=false                 ; no logs to stdout if true; default false
    minfds=1024                  ; min. avail startup file descriptors; default 1024
    minprocs=200                 ; min. avail process descriptors;default 200

    ; The rpcinterface:supervisor section must remain in the config file for
    ; RPC (supervisorctl/web interface) to work.  Additional interfaces may be
    ; added by defining them in separate [rpcinterface:x] sections.

    [rpcinterface:supervisor]
    supervisor.rpcinterface_factory = supervisor.rpcinterface:make_main_rpcinterface

    ; The supervisorctl section configures how supervisorctl will connect to
    ; supervisord.  configure it match the settings in either the unix_http_server
    ; or inet_http_server section.

    [supervisorctl]
    serverurl=unix:///tmp/supervisor.sock ; use a unix:// URL  for a unix socket
    ;serverurl=http://127.0.0.1:9001 ; use an http:// url to specify an inet socket
    ;username=user               ; should be same as in [*_http_server] if set
    ;password=123                ; should be same as in [*_http_server] if set
    ;prompt=mysupervisor         ; cmd line prompt (default "supervisor")
    ;history_file=~/.sc_history  ; use readline history if available

    [program:server_realtime_auth]
    command=/usr/bin/node /home/jsuser/json-scada/src/server_realtime_auth/index.js
    ;process_name=%(program_name)s ; process_name expr (default %(program_name)s)
    numprocs=1                     ; number of processes copies to start (def 1)
    directory=/home/jsuser/json-scada/src/server_realtime_auth/    ; directory to cwd to before exec (def no cwd)
    user=jsuser                    ; setuid to this UNIX account to run the program
    stdout_logfile=/home/jsuser/json-scada/log/server_realtime_auth.log    ; stdout log path, NONE for none;
    stdout_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stdout_logfile_backups=0       ; # of stdout logfile backups (0 means none, default 10)
    stdout_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)
    stderr_logfile=/home/jsuser/json-scada/log/server_realtime_auth.err    ; stderr log path, NONE for none;
    stderr_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stderr_logfile_backups=0       ; # of stderr logfile backups (0 means none, default 10)
    stderr_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)

    [program:cs_data_processor]
    command=/usr/bin/node /home/jsuser/json-scada/src/cs_data_processor/cs_data_processor.js
    ;process_name=%(program_name)s ; process_name expr (default %(program_name)s)
    numprocs=1                     ; number of processes copies to start (def 1)
    directory=/home/jsuser/json-scada/src/cs_data_processor/    ; directory to cwd to before exec (def no cwd)
    user=jsuser                    ; setuid to this UNIX account to run the program
    stdout_logfile=/home/jsuser/json-scada/log/cs_data_processor.log    ; stdout log path, NONE for none;
    stdout_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stdout_logfile_backups=0       ; # of stdout logfile backups (0 means none, default 10)
    stdout_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)
    stderr_logfile=/home/jsuser/json-scada/log/cs_data_processor.err    ; stderr log path, NONE for none;
    stderr_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stderr_logfile_backups=0       ; # of stderr logfile backups (0 means none, default 10)
    stderr_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)

    [program:calculations]
    command=/home/jsuser/json-scada/bin/calculations
    ;process_name=%(program_name)s ; process_name expr (default %(program_name)s)
    numprocs=1                     ; number of processes copies to start (def 1)
    directory=/home/jsuser/json-scada/bin/    ; directory to cwd to before exec (def no cwd)
    user=jsuser                    ; setuid to this UNIX account to run the program
    stdout_logfile=/home/jsuser/json-scada/log/calculations.log    ; stdout log path, NONE for none;
    stdout_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stdout_logfile_backups=0       ; # of stdout logfile backups (0 means none, default 10)
    stdout_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)
    stderr_logfile=/home/jsuser/json-scada/log/calculations.err    ; stderr log path, NONE for none;
    stderr_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stderr_logfile_backups=0       ; # of stderr logfile backups (0 means none, default 10)
    stderr_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)

    [program:iec104client]
    command=/home/jsuser/json-scada/bin/iec104client 1 1
    ;process_name=%(program_name)s ; process_name expr (default %(program_name)s)
    numprocs=1                     ; number of processes copies to start (def 1)
    directory=/home/jsuser/json-scada/bin/    ; directory to cwd to before exec (def no cwd)
    user=jsuser                    ; setuid to this UNIX account to run the program
    stdout_logfile=/home/jsuser/json-scada/log/iec104client.log    ; stdout log path, NONE for none; 
    stdout_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stdout_logfile_backups=0       ; # of stdout logfile backups (0 means none, default 10)
    stdout_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)
    stderr_logfile=/home/jsuser/json-scada/log/iec104client.err    ; stderr log path, NONE for none;
    stderr_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stderr_logfile_backups=0       ; # of stderr logfile backups (0 means none, default 10)
    stderr_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)

    [program:iec104server]
    command=/home/jsuser/json-scada/bin/iec104server 1 1
    ;process_name=%(program_name)s ; process_name expr (default %(program_name)s)
    numprocs=1                     ; number of processes copies to start (def 1)
    directory=/home/jsuser/json-scada/bin/    ; directory to cwd to before exec (def no cwd)
    user=jsuser                    ; setuid to this UNIX account to run the program
    stdout_logfile=/home/jsuser/json-scada/log/iec104server.log    ; stdout log path, NONE for none;
    stdout_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stdout_logfile_backups=0       ; # of stdout logfile backups (0 means none, default 10)
    stdout_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)
    stderr_logfile=/home/jsuser/json-scada/log/iec104server.err    ; stderr log path, NONE for none;
    stderr_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stderr_logfile_backups=0       ; # of stderr logfile backups (0 means none, default 10)
    stderr_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)

    [program:pg_hist]
    command=/home/jsuser/json-scada/sql/process_pg_hist.sh
    ;process_name=%(program_name)s ; process_name expr (default %(program_name)s)
    numprocs=1                     ; number of processes copies to start (def 1)
    directory=/home/jsuser/json-scada/sql/   ; directory to cwd to before exec (def no cwd)
    user=jsuser                    ; setuid to this UNIX account to run the program
    stdout_logfile=/home/jsuser/json-scada/log/pg_hist.log    ; stdout log path, NONE for none;
    stdout_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stdout_logfile_backups=0       ; # of stdout logfile backups (0 means none, default 10)
    stdout_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)
    stderr_logfile=/home/jsuser/json-scada/log/pg_hist.err    ; stderr log path, NONE for none;
    stderr_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stderr_logfile_backups=0       ; # of stderr logfile backups (0 means none, default 10)
    stderr_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)

    [program:pg_rtdata]
    command=/home/jsuser/json-scada/sql/process_pg_rtdata.sh
    ;process_name=%(program_name)s ; process_name expr (default %(program_name)s)
    numprocs=1                     ; number of processes copies to start (def 1)
    directory=/home/jsuser/json-scada/sql/    ; directory to cwd to before exec (def no cwd)
    user=jsuser                    ; setuid to this UNIX account to run the program
    stdout_logfile=/home/jsuser/json-scada/log/pg_rtdata.log    ; stdout log path, NONE for none;
    stdout_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stdout_logfile_backups=0       ; # of stdout logfile backups (0 means none, default 10)
    stdout_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)
    stderr_logfile=/home/jsuser/json-scada/log/pg_rtdata.err    ; stderr log path, NONE for none;
    stderr_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stderr_logfile_backups=0       ; # of stderr logfile backups (0 means none, default 10)
    stderr_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)

Execute the supervisor daemon

    supervisord -c /etc/supervisord.conf

Use the manager tool to start, stop and monitor the system

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
    nssm install JSON_SCADA_server_realtime <PATH_TO_NODEJSEXE>\node "C:\json-scada\src\server_realtime\index.js"

    ... and so on ...

To manage services use

    nssm start service_name
    nssm stop service_name
    nssm restart service_name
    nssm status service_name
    nssm remove service_name

    
