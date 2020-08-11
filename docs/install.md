# Installing JSON-SCADA

To install JSON-SCADA, it is required to install all the requirements first. There is no point reproducing original installation instructions for each upstream project. Here are links and relevant information specific to JSON-SCADA.

## Supported platforms

* Most modern Linux x86/64 bits. Recommend Centos/Redhat 8.2.

* Windows 10 or Server x86/64 bits.

* Linux ARM 32 bits (tested at least for protocol drivers on Raspberry Pi 3/Raspbian OS).

It can possibly work also on MacOS and Linux ARM-64.

A full system can run on a single commodity computer but for high performance and high availability on big systems (> 10.000 tags) it is strongly recommended the following hardware

* Recent Intel Xeon, AMD Epyc or Threadripper server processors.

* 32 GB RAM or more.

* Exclusive data disks (XFS formatted on Linux) on 1TB+ NVMe SSDs for MongoDB (RAID-1 mirrored for high availability).

* Exclusive data disks on 1TB+ NVMe SSDs for PostgreSQL (RAID-1 mirrored for high availability).

* MongoDB replica set cluster with 3 servers.

* 2 PostgreSQL servers with replication.

For more than 200.000 tags, a MongoDB sharded cluster may be needed.

## Software Requirements

### 1. MongoDB Server

Version 4.2.8 - Lower versions are not supported and not recommended. Newer versions can work but were not tested.

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

### 3. Grafana

Grafana version 7.1.x Previous versions can work but are not recommended.

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

* DotNet Core version 3.1. Previous versions are not tested or supported.
* https://dotnet.microsoft.com/download

### 7. Other recommended software tools

* Inkscape SAGE or SCADAvis.io SVG Editor - https://sourceforge.net/projects/oshmiopensubstationhmi/ or https://www.microsoft.com/en-us/p/scadavisio-synoptic-editor/9p9905hmkz7x . Available only for Windows.
* MongoDB Compass - https://www.mongodb.com/products/compass
* Git - https://git-scm.com/
* Visual Studio Code - https://code.visualstudio.com/
* Supervisor (for Linux) - http://supervisord.org/installing.html
* NSSM (for Windows) - https://nssm.cc/

## JSON-SCADA Processes - Build and Setup

Download the code from the online repo

* https://github.com/riclolsen/json-scada

Or do a git clone

    git clone https://github.com/riclolsen/json-scada

Build the code (use inverted slashes and copy instead of cp on Windows)
    
    cd json-scada
    mkdir bin

    cd src/lib60870
    dotnet publish -p:PublishSingleFile=true -p:PublishReadyToRun=true -c Release -o ../../bin/

    cd ../calculations
    go get ./... 
    go build 
    cp calculations ../../bin/

    cd ../i104m
    go get ./... 
    go build 
    cp i104m ../../bin/

    cd ../cs_data_processor
    npm update
    cd ../server_realtime
    npm update
    cd ../oshmi2json
    npm update

Configure the conf/json-scada.json file to define the node name and to point to the MongoDB server. Processes will look for the config file on the ../conf/ folder.

* [Config File Documentation](../conf/README.md)

Processes can be distributed on distinct servers, each server must have a different node name.

Multiple JSON-SCADA systems can run on the same server, for this each one must have a distinct MongoDB and PostgreSQL database and a separate folder structure. Also a distinct listen HTTP ports must be configured.

It is recommended to run JSON-SCADA processes as services or daemons. On Linux we recommend the _Supervisor_ tool to manage processes. On Windows we recommend to convert processes on Windows services using the NSSM tool.

### Supervisor Configuration (Linux)


### NSSM Configuration (Windows)

