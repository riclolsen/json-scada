<a href="https://github.com/riclolsen/json-scada/">
    <img src="https://github.com/riclolsen/json-scada/raw/master/src/htdocs/images/json-scada.svg" alt="JSON:SCADA Logo" title="JSON:SCADA" align="right" height="60" />
</a>

{json:scada}
============

A portable and scalable SCADA/IIoT-I4.0 platform centered on the MongoDB database server.

![](https://img.shields.io/badge/node-%3E%3D14-green "Node.js >= 14")
![](https://img.shields.io/badge/golang-%3E%3D1.14-green "Go >= 1.14")
![](https://img.shields.io/badge/dotnet-%3E%3D5.0-green "Dotnet >=5.0")

![](https://img.shields.io/badge/mongodb-%3E%3D4.2-green "MongoDB >= 4.2")
![](https://img.shields.io/badge/postgresql-12-green "PostgreSQL 12")
![](https://img.shields.io/badge/timescaledb-2.0-green "TimescaleDB 2.0")
![](https://img.shields.io/badge/grafana-%3E%3D7-green "Grafana >= 7")

![](https://img.shields.io/badge/linux-x86--64-green "Linux x86-64")
![](https://img.shields.io/badge/linux-ARM-green "Linux ARM-64")
![](https://img.shields.io/badge/windows-x86--64-green "Windows x86-64")
![](https://img.shields.io/badge/macosx-x86--64-green "Mac OSX x86-64")
![](https://img.shields.io/badge/macosx-ARM--M1-yellow "Mac ARM M1")

![](https://img.shields.io/badge/IEC60870--5--104-green "IEC60870-5-104")
![](https://img.shields.io/badge/IEC60870--5--101-green "IEC60870-5-101")
![](https://img.shields.io/badge/DNP3-green "DNP3")
![](https://img.shields.io/badge/MQTT-green "MQTT")
![](https://img.shields.io/badge/Sparkplug--B-green "Sparkplug B")
![](https://img.shields.io/badge/OPC--UA-green "OPC-UA")
![](https://img.shields.io/badge/CIP.Ethernet/IP-yellow "CIP Ethernet/IP")

![](https://img.shields.io/badge/license-GPL-green "License GPL")
![](https://img.shields.io/badge/contributors-welcome-green "Contributors Welcome")

## Mission Statement

To provide an easy to use, fully-featured, scalable, and portable SCADA/IIoT-I4.0 platform built by leveraging mainstream open-source IT tools.
 
## Screenshots

![screenshots](https://github.com/riclolsen/json-scada/raw/master/docs/screenshots/anim-screenshots.gif "{json:scada} Screenshots")

## Major features and characteristics

* Standard IT tools applied to SCADA/IoT (MongoDB, PostgreSQL/TimescaleDB, Node.js, C#, Golang, Grafana, etc.).
* MongoDB as the real-time core database, persistence layer, config store, SOE historian.
* Event-based realtime async data processing with MongoDB Change Streams.
* Portability and modular interoperability over Linux, Windows, Mac OSX, x86/64, ARM.
* Windows installer available in the [releases section](https://github.com/riclolsen/json-scada/releases/tag/V0.22-alpha).
* Unlimited tags, servers, and users.
* Horizontal scalability, from a single computer to big clusters (MongoDB-sharding), Docker containers, VMs, Kubernetes, cloud, or hybrid deployments.
* Modular distributed architecture. Lightweight redundant data acquisition nodes can connect securely over TLS to the database server. E.g. a Raspberry PI can be a data acquisition node.
* Extensibility of the core data model (MongoDB: NoSQL/schema-less).
* HTML5 Web interface. UTF-8/I18N. Mobile access. Web-based configuration management.
* Role-based access control (RBAC). 
* Various high-quality protocol drivers.
* Integration with MQTT brokers (compatibility with Sparkplug B).
* Live point configuration updates.
* Inkscape-based SVG synoptic display editor.
* PostgreSQL/TimescaleDB historian integrated with Grafana for easy creation of dashboards.
* Easy development of custom applications with modern stacks like MEAN/MERN, etc. Extensive use of JSON from bottom up.
* Leverage a huge ecosystem of MongoDB/PostgreSQL tools, community, services, etc.

## Use cases

* Power/Oil/Gas/Manufacturing/etc Local Station HMI.
* SCADA Protocol Gateway.
* SCADA Control Centers.
* SCADA/IIoT Historian.
* Intranet/Internet HTTPS Gateway - Visualization Server.
* Multilevel Systems Integration (SCADA/IIoT/ERP/MES/PLC).
* Global-Level/Cloud SCADA Systems Integration.
* Edge processing.
* Data concentrator for Big Data / ML processing.
* Digital Transformation, Industry 4.0 enabler.

## Architecture

![architecture](http://www.plantuml.com/plantuml/proxy?cache=no&src=https://raw.githubusercontent.com/riclolsen/json-scada/master/docs/JSON-SCADA_Arquitecture.txt "{json:scada} Architecture")

## Documentation

* [Install Guide](docs/install.md)
* [Windows installer](https://github.com/riclolsen/json-scada/releases/tag/V0.22-alpha)
* [Docker Demo](demo-docker/README.md)
* [Schema Documentation](docs/schema.md)
* [Config File](conf/README.md)
* [SVG Synoptic Display Editor](src/svg-display-editor/README.md)
* [IEC60870-5-104 Server Driver](src/lib60870.netcore/iec104server/README.md)
* [IEC60870-5-104 Client Driver](src/lib60870.netcore/iec104client/README.md)
* [IEC60870-5-101 Server Driver](src/lib60870.netcore/iec101server/README.md)
* [IEC60870-5-101 Client Driver](src/lib60870.netcore/iec101client/README.md)
* [DNP3 Client Driver](src/dnp3/Dnp3Client/README.md)
* [Telegraf Listener Driver](src/telegraf-listener/README.md)
* [MQTT Sparkplug-B Client Driver](src/mqtt-sparkplug/README.md)
* [OPC-UA Client Driver](src/OPC-UA-Client/README.md)
* [OPC-UA Server Driver](src/OPC-UA-Server/README.md)
* [CIP Ethernet/IP PLCTags Client Driver](src/libplctag/PLCTagsClient/README.md)
* [I104M Client Driver](src/i104m/README.md)
* [Calculations](src/calculations/README.md)
* [Change Stream Data Processor](src/cs_data_processor/README.md)
* [Custom Data Processor](src/cs_custom_processor/README.md)
* [Realtime Data Server](src/server_realtime/README.md)
* [OSHMI2JSON Tool](src/oshmi2json/README.md)
* [Report Generators](docs/report_generators.md)
* [SAGE-web Displays](src/htdocs/sage-cepel-displays/README.md)

## Protocols Roadmap

- [x] IEC 60870-5-104 Server TCP/TLS
- [x] IEC 60870-5-104 Client TCP/TLS
- [x] IEC 60870-5-101 Server (Serial, TCP)
- [x] IEC 60870-5-101 Client (Serial, TCP)
- [ ] IEC 60870-5-103 Client
- [x] DNP3 Client (TCP, UDP, TLS, Serial) - Windows x64 only!
- [ ] DNP3 Server (TCP, UDP, TLS, Serial)
- [x] MQTT/Sparkplug-B Client
- [x] I104M (adapter for some OSHMI drivers)
- [x] ICCP Client (via I104M)
- [x] Telegraf Client (many data sources available such as MQTT, MODBUS, SNMP, ...)
- [x] OPC UA Client
- [x] OPC UA Server
- [ ] OPC UA Historical Data Server
- [ ] OPC UA Server
- [ ] OPC DA Client
- [ ] Modbus Client
- [ ] IEC 61850 MMS Client
- [ ] IEC 61850 GOOSE Client
- [x] CIP Ethernet/IP (libplctag, experimental)
- [ ] Siemens S7
- [ ] BACNET

## Features Roadmap

- [x] Web-based Viewers
- [x] Web-based Configuration Manager
- [x] User auth/Role-based Access Control (RBAC)
- [x] Inkscape-based SVG Synoptic Editor
- [x] Compiled Calculations Engine
- [ ] Low-latency/Interpreted Calculations Engine
- [x] Basic Alarms Processor
- [ ] Advanced Alarms Processor 
- [x] PostgreSQL/TimescaleDB Historian
- [x] Grafana Integration 
- [x] Grafana Alerting Integration (w/ Events Viewer)
- [x] Windows Installer
- [x] Online Demo
- [x] Docker Demo (docker-compose.yaml scripts)
- [ ] Linux Image / VM
- [x] Supervisor (Linux process manager) examples
- [ ] InfluxDB Integration
- [x] Telegraf Integration
- [ ] Kafka Integration
- [x] PowerBI Integration (via PostgreSQL connector)
- [ ] PowerBI Direct Integration
- [ ] NodeRed Integration
- [x] Metabase Integration (via PostgreSQL/MongoDB connectors)
- [ ] Alerta Integration (https://alerta.io/)
- [ ] PLC4X Integration (https://plc4x.apache.org/)
- [ ] Managed Cloud Service
- [x] SAGE (brazilian SCADA from CEPEL) web displays visualization

## Online Demo (substations simulation)

* http://vmi233205.contaboserver.net:8080/

This demo provides a public IEC 60870-5-104 server port on IP address 207.180.242.96:2404 (common address = 1) for testing.

The demo data is published as regular MQTT topics to the public broker mqtt://test.mosquitto.org:1883 (about 8600 topics in ACME_Utility/#).

Data is also published as Sparkplug-B to mqtt://test.mosquitto.org:1883 (about 4300 device metrics in spBv1.0/Sparkplug B Devices/+/JSON-SCADA Server/#). Data/birth messages are compressed by Eclipse Tahu Javascript libs.

## Developer Contact

* https://www.linkedin.com/in/ricardo-olsen/
