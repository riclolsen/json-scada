<a href="https://aimeos.org/">
    <img src="https://github.com/riclolsen/json-scada/raw/master/src/htdocs/images/json-scada.svg" alt="JSON:SCADA Logo" title="JSON:SCADA" align="right" height="60" />
</a>

{json:scada}
============

A portable and scalable SCADA/IoT platform centered on the MongoDB database server.

![](https://img.shields.io/badge/node-%3E%3D14-green "Node.js >= 14")
![](https://img.shields.io/badge/golang-%3E%3D1.14-green "Go >= 1.14")
![](https://img.shields.io/badge/dotnet-%3E%3D5.0-green "Dotnet >=5.0")

![](https://img.shields.io/badge/mongodb-%3E%3D4.2-green "MongoDB >= 4.2")
![](https://img.shields.io/badge/postgresql-12-green "PostgreSQL 12")
![](https://img.shields.io/badge/timescaledb-1.7-green "TimescaleDB 1.7")
![](https://img.shields.io/badge/grafana-%3E%3D7-green "Grafana >= 7")

![](https://img.shields.io/badge/linux-x86--64-green "Linux x86-64")
![](https://img.shields.io/badge/linux-ARM-green "Linux ARM")
![](https://img.shields.io/badge/windows-x86--64-green "Windows x86-64")
![](https://img.shields.io/badge/macosx-x86--64-green "Mac OSX x86-64")
![](https://img.shields.io/badge/macosx-ARM--M1-yellow "Mac ARM M1 x86-64")

![](https://img.shields.io/badge/IEC60870--5--104-green "IEC60870-5-104")
![](https://img.shields.io/badge/IEC60870--5--101-green "IEC60870-5-101")
![](https://img.shields.io/badge/DNP3-yellow "DNP3")
![](https://img.shields.io/badge/CIP.Ethernet/IP-yellow "CIP Ethernet/IP")

![](https://img.shields.io/badge/license-GPL-green "License GPL")
![](https://img.shields.io/badge/contributors-welcome-green "Contributors Welcome")

## Mission Statement

To provide an easy to use, fully-featured, flexible, scalable, and portable SCADA/IoT system built by leveraging mainstream open-source IT tools.
 
## Screenshots

![screenshots](docs/screenshots/anim-screenshots.gif "{json:scada} Screenshots")

## Major features and characteristics

* Standard IT tools applied to SCADA/IoT (MongoDB, PostgreSQL/TimescaleDB,Node.js, C#, Golang, Grafana, etc.).
* MongoDB as the real-time core database, persistence layer, config store, SOE historian.
* Portability and interoperability over Linux, Windows, Mac OSX, x86/64, ARM.
* Windows installer available in the [releases section](https://github.com/riclolsen/json-scada/releases/tag/V0.5-alpha).
* Horizontal scalability, from a single computer to big clusters (MongoDB-sharding), Docker containers, VMs, Kubernetes, cloud, or hybrid deployments.
* Unlimited tags, servers, and users.
* Modular distributed architecture. Lightweight redundant data acquisition nodes can connect securely over TLS to the database server. E.g. a Raspberry PI can be a data acquisition node.
* MongoDB Change Streams for realtime async database events processing.
* HTML5 Web interface. UTF-8/I18N. Mobile access.
* Inkscape-based SVG synoptic display editor.
* PostgreSQL/TimescaleDB historian integrated with Grafana for easy creation of dashboards.
* Extensibility of data model (MongoDB: NoSQL/schema-less).
* Development of custom applications with modern stacks like MEAN/MERN, etc.
* Big data / ML capabilities through MongoDB Spark connector.
* Access to the huge MongoDB and PostgreSQL ecosystem of tools, community, services, etc.
* Easy to understand system with small code footprint for each independent module. Extensive use of JSON from bottom up.
* Possibility of easy integration of new and custom protocol drivers developed with modern programming languages (just read/write to MongoDB).
* Future-proof, vendor independence, flexibility, extensibility.
* Reduced human costs for maintenance and development thanks to the employment of widely-used open-source IT technologies.
* Live point configuration updates.
* Role-based access control (RBAC).
* Web-based configuration management (now under development).

## Use cases

* Power/Oil/Gas/etc Local Station HMI.
* Manufacturing Local HMI.
* SCADA Protocol Gateway.
* SCADA Control Center Full System.
* SCADA/IoT Historian. MS Power BI integration.
* Intranet/Internet HTTPS Gateway - Visualization Server.
* Multilevel Systems Integrator (SCADA/IoT/ERP/MES/PLC).
* Global-Level SCADA Systems Integration/Centralization.
* Edge data processor.
* Extensible Development Platform For Data Acquisition And Processing.
* Data concentrator for Big Data / ML processing.

## Architecture

![architecture](https://github.com/riclolsen/json-scada/raw/master/docs/JSON-SCADA_ARCHITECTURE.png "{json:scada} Architecture")

## Documentation

* [Screenshots](docs/screenshots/)
* [Docker Demo](demo-docker/README.md)
* [Config File](conf/README.md)
* [Calculations](src/calculations/README.md)
* [IEC60870-5-104 Server Driver](src/lib60870.netcore/iec104server/README.md)
* [IEC60870-5-104 Client Driver](src/lib60870.netcore/iec104client/README.md)
* [IEC60870-5-101 Server Driver](src/lib60870.netcore/iec101server/README.md)
* [IEC60870-5-101 Client Driver](src/lib60870.netcore/iec101client/README.md)
* [DNP3 Client Driver](src/dnp3/Dnp3Client/README.md)
* [CIP Ethernet/IP PLCTags Client Driver](src/libplctag/PLCTagsClient/README.md)
* [I104M Client Driver](src/i104m/README.md)
* [Change Stream Data Processor](src/cs_data_processor/README.md)
* [Realtime Data Server](src/server_realtime/README.md)
* [SVG Synoptic Display Editor](src/svg-display-editor/README.md)
* [OSHMI2JSON Tool](src/oshmi2json/README.md)
* [Schema Documentation](docs/schema.md)
* [Install Guide](docs/install.md)

## Protocols Roadmap

- [x] IEC 60870-5-104 Server TCP
- [ ] IEC 60870-5-104 Server TLS
- [x] IEC 60870-5-104 Client TCP/TLS
- [x] IEC 60870-5-101 Server (Serial, TCP)
- [x] IEC 60870-5-101 Client (Serial, TCP)
- [x] DNP3 Client (TCP, UDP, TLS, Serial)
- [ ] DNP3 Server (TCP, UDP, TLS, Serial)
- [x] I104M (adapter for OSHMI driver)
- [x] ICCP Client (via I104M)
- [ ] Secure ICCP Client
- [ ] OPC UA Client
- [ ] OPC UA Server
- [ ] OPC DA Client
- [ ] OPC DA Server
- [ ] Modbus Client
- [ ] MQTT
- [ ] IEC 61850 MMS
- [ ] IEC 61850 GOOSE
- [x] CIP Ethernet/IP (libplctag)
- [ ] Siemens S7
- [ ] BACNET
- [ ] IEC 60870-5-103 Client
- [ ] OPC UA Historical Data Server

## Features Roadmap

- [x] Web-based Viewers
- [x] Inkscape-based SVG Synoptic Editor
- [x] Compiled Calculations Engine
- [x] Basic Alarms Processor
- [ ] Advanced Alarms Processor 
- [x] PostgreSQL/TimescaleDB Historian
- [x] Grafana Integration 
- [x] Grafana Alerting Integration (Events Viewer)
- [x] Docker Demo
- [x] Windows Installer
- [ ] Linux Image / VM / Container
- [x] User auth/RBAC with web-based config tool
- [x] Supervisor (Linux process manager) examples
- [ ] Web-based Configuration Manager (now under development)
- [ ] Low-latency calculations
- [ ] InfluxDB Integration
- [ ] Telegraf Integration
- [ ] Kafka Integration
- [x] PowerBI Integration (through PostgreSQL connector)
- [ ] PowerBI Direct Integration
- [ ] NodeRed Integration
- [x] Metabase Integration (through PostgreSQL connector)
- [ ] Alerta Integration (https://alerta.io/)
- [ ] Command Line Manager

## Contact

https://www.linkedin.com/in/ricardo-olsen/
