<a href="https://github.com/riclolsen/json-scada/">
    <img src="https://github.com/riclolsen/json-scada/raw/master/src/htdocs/images/json-scada.svg" alt="JSON:SCADA Logo" title="JSON:SCADA" align="right" height="60" />
</a>

# {json:scada}

A portable and scalable SCADA/IIoT-I4.0 platform centered on the MongoDB database server.

![](https://img.shields.io/badge/nodejs-20-green 'Node.js 20+')
![](https://img.shields.io/badge/golang-1.21-green 'Go 1.21+')
![](https://img.shields.io/badge/dotnet-6.0-green 'Dotnet 6.0')

![](https://img.shields.io/badge/mongodb-6.0-green 'MongoDB 6.0+')
![](https://img.shields.io/badge/postgresql-12-green 'PostgreSQL 12+')
![](https://img.shields.io/badge/timescaledb-2.0-green 'TimescaleDB 2.0')
![](https://img.shields.io/badge/grafana-9-green 'Grafana 9+')

![](https://img.shields.io/badge/linux-x86--64-green 'Linux x86-64')
![](https://img.shields.io/badge/linux-ARM-green 'Linux ARM-64')
![](https://img.shields.io/badge/windows-x86--64-green 'Windows x86-64')
![](https://img.shields.io/badge/macosx-x86--64-green 'Mac OSX x86-64')
![](https://img.shields.io/badge/macosx-ARM--M1-yellow 'Mac ARM M1')

![](https://img.shields.io/badge/IEC61850-green 'IEC61850')
![](https://img.shields.io/badge/IEC60870--5--104-green 'IEC60870-5-104')
![](https://img.shields.io/badge/IEC60870--5--101-green 'IEC60870-5-101')
![](https://img.shields.io/badge/DNP3-green 'DNP3')
![](https://img.shields.io/badge/MQTT-green 'MQTT')
![](https://img.shields.io/badge/Sparkplug--B-green 'Sparkplug B')
![](https://img.shields.io/badge/OPC--UA-green 'OPC-UA')
![](https://img.shields.io/badge/OPC--DA-yellow 'OPC-DA')
![](https://img.shields.io/badge/Modbus-green 'Modbus')

![](https://img.shields.io/badge/license-GPL-green 'License GPL')
![](https://img.shields.io/badge/contributors-welcome-green 'Contributors Welcome')

## Mission Statement

To provide an easy to use, fully-featured, scalable, and portable SCADA/IIoT-I4.0 platform built by leveraging mainstream open-source IT tools.

## Screenshots

![screenshots](https://github.com/riclolsen/json-scada/raw/master/docs/screenshots/anim-screenshots.gif '{json:scada} Screenshots')

## Major features

- Standard IT tools applied to SCADA/IoT (MongoDB, PostgreSQL/TimescaleDB, Node.js, C#, Golang, Grafana, etc.).
- MongoDB as the real-time core database, persistence layer, config store, SOE historian.
- Event-based realtime async data processing with MongoDB Change Streams.
- Portability and modular interoperability over Linux, Windows, Mac OSX, x86/64, ARM.
- Windows installer available in the [releases section](https://github.com/riclolsen/json-scada/releases/tag/V0.37-alpha).
- Unlimited tags, servers, and users.
- Horizontal scalability, from a single computer to big clusters (MongoDB-sharding), Docker containers, VMs, Kubernetes, cloud, or hybrid deployments.
- Modular distributed architecture. Lightweight redundant data acquisition nodes can connect securely over TLS to the database server. E.g. a Raspberry PI can be a data acquisition node.
- Extensibility of the core data model (MongoDB: NoSQL/schema-less).
- HTML5 Web interface. UTF-8/I18N. Mobile access. Web-based configuration management.
- Role-based access control (RBAC).
- Various high-quality protocol drivers.
- Integration with MQTT brokers (compatibility with Sparkplug B).
- Live point configuration updates.
- Inkscape-based SVG synoptic display editor.
- PostgreSQL/TimescaleDB historian integrated with Grafana for easy creation of dashboards.
- Easy development of custom applications with modern stacks like MEAN/MERN, etc. Extensive use of JSON from bottom up.
- Leverage a huge ecosystem of MongoDB/PostgreSQL tools, community, services, etc.

## Use cases

- Power/Oil/Gas/Manufacturing/etc Local Station HMI.
- SCADA Protocol Gateway.
- SCADA Control Centers.
- SCADA/IIoT Historian.
- Intranet/Internet HTTPS Gateway - Visualization Server.
- Multilevel Systems Integration (SCADA/IIoT/ERP/MES/PLC).
- Global-Level/Cloud SCADA Systems Integration.
- Edge processing.
- Data concentrator for Big Data / ML processing.
- Digital Transformation, Industry 4.0 enabler.

## Real-world usage

- 5+ years of usage in 2 big control centers scanning data from 80+ substations, 90k tags.
- 5+ years of usage as HMI for local operation of circa 40 substations up to 230kV level.

## Architecture

![architecture](http://www.plantuml.com/plantuml/proxy?cache=no&src=https://raw.githubusercontent.com/riclolsen/json-scada/master/docs/JSON-SCADA_Arquitecture.txt '{json:scada} Architecture')

## Documentation

- [Generic Install Guide](https://github.com/riclolsen/json-scada/blob/master/docs/install.md)
- [Windows Installer](https://github.com/riclolsen/json-scada/releases/tag/V0.37-alpha)
- [RedHat/Rocky Linux Installer](https://github.com/riclolsen/json-scada/blob/master/docs/install.md#rhel94-and-compatible-systems-automated-installation)
- [Docker Demo](https://github.com/riclolsen/json-scada/blob/master/demo-docker/README.md)
- [Schema Documentation](https://github.com/riclolsen/json-scada/blob/master/docs/schema.md)
- [Config File](https://github.com/riclolsen/json-scada/blob/master/conf/README.md)
- [SVG Synoptic Display Editor](https://github.com/riclolsen/json-scada/blob/master/src/svg-display-editor/README.md)
- [IEC61850 Client Driver](https://github.com/riclolsen/json-scada/tree/master/src/libiec61850/dotnet/core/2.0/iec61850_client/README.md)
- [IEC60870-5-104 Server Driver](https://github.com/riclolsen/json-scada/blob/master/src/lib60870.netcore/iec104server/README.md)
- [IEC60870-5-104 Client Driver](https://github.com/riclolsen/json-scada/blob/master/src/lib60870.netcore/iec104client/README.md)
- [IEC60870-5-101 Server Driver](https://github.com/riclolsen/json-scada/blob/master/src/lib60870.netcore/iec101server/README.md)
- [IEC60870-5-101 Client Driver](https://github.com/riclolsen/json-scada/blob/master/src/lib60870.netcore/iec101client/README.md)
- [DNP3 Client Driver](https://github.com/riclolsen/json-scada/blob/master/src/dnp3/Dnp3Client/README.md)
- [Telegraf Listener Driver](https://github.com/riclolsen/json-scada/blob/master/src/telegraf-listener/README.md)
- [MQTT Sparkplug-B Client Driver](https://github.com/riclolsen/json-scada/blob/master/src/mqtt-sparkplug/README.md)
- [OPC-UA Client Driver](https://github.com/riclolsen/json-scada/blob/master/src/OPC-UA-Client/README.md)
- [OPC-UA Server Driver](https://github.com/riclolsen/json-scada/blob/master/src/OPC-UA-Server/README.md)
- [OPC-DA Client Driver](https://github.com/riclolsen/json-scada/blob/master/src/OPC-DA-Client/README.md)
- [PLC4X-GO Modbus Client Driver](https://github.com/riclolsen/json-scada/blob/master/src/plc4x-client/README.md)
- [CIP Ethernet/IP PLCTags Client Driver](https://github.com/riclolsen/json-scada/blob/master/src/libplctag/PLCTagsClient/README.md)
- [I104M Client Driver](https://github.com/riclolsen/json-scada/blob/master/src/i104m/README.md)
- [Calculations](https://github.com/riclolsen/json-scada/blob/master/src/calculations/README.md)
- [Change Stream Data Processor](https://github.com/riclolsen/json-scada/blob/master/src/cs_data_processor/README.md)
- [Custom Data Processor](https://github.com/riclolsen/json-scada/blob/master/src/cs_custom_processor/README.md)
- [Realtime Data Server](https://github.com/riclolsen/json-scada/blob/master/src/server_realtime_auth/README.md)
- [OSHMI2JSON Tool](https://github.com/riclolsen/json-scada/blob/master/src/oshmi2json/README.md)
- [Report Generators](https://github.com/riclolsen/json-scada/blob/master/docs/report_generators.md)
- [SAGE-web Displays](https://github.com/riclolsen/json-scada/blob/master/src/htdocs/sage-cepel-displays/README.md)

## Protocols Roadmap

- [x] IEC 60870-5-104 Server TCP/TLS
- [x] IEC 60870-5-104 Client TCP/TLS
- [x] IEC 60870-5-101 Server (Serial, TCP)
- [x] IEC 60870-5-101 Client (Serial, TCP)
- [ ] IEC 60870-5-103 Client
- [x] DNP3 Client (TCP, UDP, TLS, Serial) - Windows x64 only!
- [ ] DNP3 Server (TCP, UDP, TLS, Serial)
- [x] MQTT/Sparkplug-B Client
- [x] Modbus Client via PLC4X-GO
- [x] I104M (adapter for some OSHMI drivers)
- [ ] ICCP Client
- [ ] ICCP Server
- [x] Telegraf Client (OPC-UA, MQTT, MODBUS, SNMP, ...)
- [x] OPC UA Client
- [x] OPC UA Server
- [ ] OPC UA Historical Data Server
- [x] OPC DA Client (Windows)
- [ ] OPC AE Client (Windows)
- [ ] OPC DA Server (Windows)
- [x] IEC 61850 MMS Client
- [ ] IEC 61850 MMS Server
- [ ] IEC 61850 GOOSE Client
- [x] CIP Ethernet/IP (libplctag, experimental)
- [ ] Siemens S7
- [ ] BACNET

## Features Roadmap

- [x] Web-based Viewers
- [x] Web-based Configuration Manager
- [x] Excel-based Configuration
- [x] User auth/Role-based Access Control (RBAC)
- [x] Inkscape-based SVG Synoptic Editor
- [x] Compiled Calculations Engine
- [x] Customizable Change-Stream Processor (for user implemented scripts)
- [ ] Low-latency/Interpreted Calculations Engine
- [x] Basic Alarms Processor
- [ ] Advanced Alarms Processor
- [x] PostgreSQL/TimescaleDB Historian
- [x] Grafana Integration
- [x] Metabase Integration (via PostgreSQL/MongoDB connectors)
- [x] One-way realtime replication (over eth diode/tap device) w/ point db sync and historical backfill
- [x] Windows Installer
- [x] Online Demo
- [x] Docker Demo (docker-compose.yaml scripts)
- [x] Install Script for RedHat/Rocky 9.4 Linux x86-64 and arm64
- [ ] Linux Image / VM
- [x] Supervisor (Linux process manager) examples
- [ ] InfluxDB Integration
- [x] Telegraf Integration
- [ ] Kafka Integration
- [x] PowerBI Integration (via PostgreSQL connector)
- [ ] PowerBI Direct Integration
- [ ] NodeRed Integration
- [ ] Alerta Integration (https://alerta.io/)
- [x] PLC4X-GO Integration (https://plc4x.apache.org/)
- [ ] Managed Cloud Service
- [ ] Supported LTS versions

## Online Demo (substations simulation)

- http://150.230.171.172

This demo provides a public IEC 60870-5-104 server port on IP address 150.230.171.172:2404 (common address = 1) for testing.

The demo data is published as regular MQTT topics to the public broker mqtt://test.mosquitto.org:1883 (about 8600 topics in JsonScadaDemoVPS/# and ACME_Utility/#).

Data is also published as Sparkplug-B to mqtt://test.mosquitto.org:1883 (about 4300 device metrics in spBv1.0/Sparkplug B Devices/+/JSON-SCADA Server/#). Data/birth messages are compressed by Eclipse Tahu Javascript libs.

## Developer Contact

- https://www.linkedin.com/in/ricardo-olsen/
