# Installing JSON-SCADA

To install JSON-SCADA, it is required to install all the requirements first. There is no point reproducing original installation instructions for each upstream project. Here are links and relevant information specific to JSON-SCADA.

## Supported Hardware/OS Platforms

* Most modern Linux x86-64 bits. Recommend Centos/Redhat 8.2, Oracle Linux 8 or equivalent.

* Windows 10 or Server x86-64 bits.

* Linux ARM 32 bits (tested at least for protocol drivers on Raspberry Pi 3/Raspbian OS).

* Mac OSX (x64 Intel or M1 under Rosetta).

It can also possibly work on Linux ARM-64.

A full system can run on a single commodity x86 computer but for high performance and high availability on big systems (> 10.000 tags) it is strongly recommended the following hardware

* Recent Intel Xeon, AMD Epyc or Threadripper server processors.

* 32 GB RAM or more.

* Exclusive data disks (XFS formatted on Linux) on 512GB+ NVMe SSDs for MongoDB (RAID-1 mirrored for high availability).

* Exclusive data disks on 1TB+ NVMe SSDs for PostgreSQL (RAID-1 mirrored for high availability).

* MongoDB replica set cluster with 3 servers.

* 2 PostgreSQL servers with replication.

For large systems (like with more than 200.000 tags), a MongoDB sharded cluster may be needed.

## Software Requirements

### 1. MongoDB Server

Version 4.2.8 or 4.4.1 - Lower versions are not supported and not recommended. Newer versions can work but were not tested.

* https://www.mongodb.com/try/download/community
* https://docs.mongodb.com/manual/installation/

The _MongoDB Atlas_ cloud service is also supported.

The _Replica Set_ feature must be enabled, even when just one server is used because this is necessary for Change Streams to work.

MongoDB supports many architectures, it is very flexible. You can deploy on just one server, on the classic 3 member replica set or on a big sharded cluster (with MongoS and config servers).

* https://docs.mongodb.com/manual/core/sharded-cluster-components/

For not trusted or open to Internet networks it is important to use TLS over MongoDB connections. Consult the MongoDB docs to learn how to config connections using certificates.

* https://docs.mongodb.com/manual/tutorial/configure-ssl/

### 2. PostgreSQL / TimescaleDB

PostgreSQL version 12. TimescaleDB version 1.7. Previous versions can work but are not recommended. Newer versions can work but were not tested.

* https://www.timescale.com/products
* https://docs.timescale.com/latest/getting-started/installation
* https://www.postgresql.org/download/

For not trusted or open to Internet networks it is important to secure PostgreSQL connections. Consult the PostgreSQL docs to learn how to config connections using certificates.

* https://www.postgresql.org/docs/12/ssl-tcp.html

Replication to a Standby server is recommended for high availability.

* https://www.postgresql.org/docs/12/different-replication-solutions.html
* https://www.postgresql.org/docs/12/warm-standby.html#STANDBY-SERVER-OPERATION

### 3. Grafana

Grafana version 7.x.x. Previous versions can work but are not recommended.

* https://grafana.com/grafana/download
* https://grafana.com/docs/grafana/latest/installation/

If certificates are configured for PostgreSQL connections to the server, it must be also configured in the Grafana PostgreSQL data source to access the historian data.

### 4. Node.js

* Node.js version 14.x. Previous versions are not tested or supported.
* https://nodejs.org/en/

### 5. Golang

* Golang version 1.14.x. Previous versions are not tested or supported.
* https://golang.org/dl/

### 6. DotNet Core

* DotNet Core version 3.1 or 5.x. Previous versions are not tested or supported.
* https://dotnet.microsoft.com/download

### 7. Other recommended software tools

* Inkscape SAGE or SCADAvis.io SVG Editor for synoptic display creation - https://sourceforge.net/projects/oshmiopensubstationhmi/ or https://www.microsoft.com/en-us/p/scadavisio-synoptic-editor/9p9905hmkz7x . Available only for Windows.
* MongoDB Compass - https://www.mongodb.com/products/compass
* Git - https://git-scm.com/
* Visual Studio Code - https://code.visualstudio.com/
* Supervisor (for Linux) - http://supervisord.org/installing.html
* NSSM (for Windows) - https://nssm.cc/

## JSON-SCADA Processes - Build and Setup

Download the code from the online repo

* https://github.com/riclolsen/json-scada

Or do a git clone

    git clone https://github.com/riclolsen/json-scada --config core.autocrlf=input

Build the code (use inverted slashes, .exe extension and copy instead of cp on Windows, choose also the adequate Dotnet target platform, on Mac use --runtime osx-x64)
    
    cd json-scada
    mkdir bin

    cd src/lib60870.netcore
    dotnet publish --runtime linux-x64 -p:PublishReadyToRun=true -c Release -o ../../bin/

    cd ../dnp3/Dnp3Client
    dotnet publish --runtime linux-x64 -p:PublishReadyToRun=true -c Release -o ../../bin/

    export GOBIN=~/json-scada/bin
    cd ../../calculations
    go get ./... 
    go build 
    cp calculations ../../bin/

    cd ../i104m
    go get ./... 
    go build 
    cp i104m ../../bin/

    cd ../cs_data_processor
    npm update
    cd ../grafana_alert2event
    npm update
    cd ../demo_simul
    npm update
    cd ../server_realtime
    npm update
    cd ../server_realtime_auth
    npm update
    cd ../oshmi2json
    npm update
    cd ../htdocs-admin
    npm update
    npm run build


Configure the conf/json-scada.json file to define the node name and to point to the MongoDB server. Processes will look for the config file on the ../conf/ folder.

* [Config File Documentation](../conf/README.md)

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

    [program:server_realtime]
    command=/usr/bin/node /home/jsuser/json-scada/src/server_realtime/index.js
    ;process_name=%(program_name)s ; process_name expr (default %(program_name)s)
    numprocs=1                     ; number of processes copies to start (def 1)
    directory=/home/jsuser/json-scada/src/server_realtime/    ; directory to cwd to before exec (def no cwd)
    user=jsuser                    ; setuid to this UNIX account to run the program
    stdout_logfile=/home/jsuser/json-scada/log/server_realtime.log    ; stdout log path, NONE for none;
    stdout_logfile_maxbytes=1MB    ; max # logfile bytes b4 rotation (default 50MB)
    stdout_logfile_backups=0       ; # of stdout logfile backups (0 means none, default 10)
    stdout_capture_maxbytes=1MB    ; number of bytes in 'capturemode' (default 0)
    stderr_logfile=/home/jsuser/json-scada/log/server_realtime.err    ; stderr log path, NONE for none;
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

### NSSM Configuration (Windows)

Install the NSSM tool. It can be installed in c:\json-scada\bin\ .

Use the tool create necessary services.

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

    
