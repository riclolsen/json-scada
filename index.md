<a href="https://github.com/riclolsen/json-scada/">
    <img src="https://github.com/riclolsen/json-scada/raw/master/src/AdminUI/public/images/json-scada.svg" alt="JSON:SCADA Logo" title="JSON:SCADA" align="right" height="60" />
</a>

# {json:scada}

A portable and scalable SCADA/IIoT-I4.0 platform centered on the MongoDB database server.

![](https://img.shields.io/badge/nodejs-20-green 'Node.js 20+')
![](https://img.shields.io/badge/golang-.21-green 'Go 1.21+')
![](https://img.shields.io/badge/dotnet-8.0-green 'Dotnet 8.0')

![](https://img.shields.io/badge/mongodb-6.0-green 'MongoDB 6.0+')
![](https://img.shields.io/badge/postgresql-16-green 'PostgreSQL 16')
![](https://img.shields.io/badge/timescaledb-2.0-green 'TimescaleDB 2.0')
![](https://img.shields.io/badge/grafana-11-green 'Grafana 11')

![](https://img.shields.io/badge/linux-x86--64-green 'Linux x86-64')
![](https://img.shields.io/badge/linux-ARM-green 'Linux ARM-64')
![](https://img.shields.io/badge/windows-x86--64-green 'Windows x86-64')
![](https://img.shields.io/badge/macosx-x86--64-green 'Mac OSX x86-64')
![](https://img.shields.io/badge/macosx-ARM--M1-yellow 'Mac ARM Mx')

![](https://img.shields.io/badge/IEC61850-green 'IEC61850')
![](https://img.shields.io/badge/IEC60870--5--104-green 'IEC60870-5-104')
![](https://img.shields.io/badge/IEC60870--5--101-green 'IEC60870-5-101')
![](https://img.shields.io/badge/DNP3-green 'DNP3')
![](https://img.shields.io/badge/MQTT-green 'MQTT')
![](https://img.shields.io/badge/Sparkplug--B-green 'Sparkplug B')
![](https://img.shields.io/badge/OPC--UA-green 'OPC-UA')
![](https://img.shields.io/badge/OPC--DA-green 'OPC-DA')
![](https://img.shields.io/badge/Modbus-green 'Modbus')

![](https://img.shields.io/badge/license-GPL-green 'License GPL')
![](https://img.shields.io/badge/contributors-welcome-green 'Contributors Welcome')

## Mission Statement

To provide an easy to use, fully-featured, scalable, and portable SCADA/IIoT-I4.0 platform built by leveraging mainstream open-source IT tools.

## Screenshots

![screenshots](https://github.com/riclolsen/json-scada/raw/master/docs/screenshots/anim-screenshots.gif '{json:scada} Screenshots')

## Major features and characteristics

- Standard IT tools applied to SCADA/IoT (MongoDB, PostgreSQL/TimescaleDB, Node.js, C#, Golang, Grafana, etc.).
- MongoDB as the real-time core database, persistence layer, config store, SOE historian.
- Event-based realtime async data processing with MongoDB Change Streams.
- Portability and modular interoperability over Linux, Windows, Mac OSX, x86/64, ARM.
- Windows installer available in the [releases section](https://github.com/riclolsen/json-scada/releases/tag/V0.54-alpha).
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
- Easy AI-helped custom app development using templates/API for tools like WindSurf/Cline/Cursor/Copilot/etc.

## Use cases

- Protocol Gateway.
- Secure Protocol Gateway with 1-way air gapped replication (via data diode or tap device).
- Power/Oil/Gas/Manufacturing/etc Local Station HMI.
- SCADA for Control Centers.
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

- [Generic Install Guide](docs/install.md)
- [Windows Installer](https://github.com/riclolsen/json-scada/releases/tag/V0.54-alpha)
- [RedHat/Rocky Linux Installer Script](docs/install.md#rhel94-and-compatible-systems-automated-installation)
- [Ubuntu Linux Installer Script](docs/install.md#ubuntu-2404-scripted-installation)
- [Generic Install Guide](docs/install.md)
- [Docker Demo](demo-docker/README.md)
- [Schema Documentation](docs/schema.md)
- [Config File](conf/README.md)
- [SVG Synoptic Display Editor](src/svg-display-editor/README.md)
- [IEC61850 Client Driver](src/iec61850_client/README.md)
- [IEC60870-5-104 Server Driver](src/lib60870.netcore/iec104server/README.md)
- [IEC60870-5-104 Client Driver](src/lib60870.netcore/iec104client/README.md)
- [IEC60870-5-101 Server Driver](src/lib60870.netcore/iec101server/README.md)
- [IEC60870-5-101 Client Driver](src/lib60870.netcore/iec101client/README.md)
- [DNP3 Client Driver](src/dnp3/Dnp3Client/README.md)
- [DNP3 Server Driver](src/dnp3/Dnp3Server/README.md)
- [Telegraf Listener Driver](src/telegraf-listener/README.md)
- [MQTT Sparkplug-B Client Driver](src/mqtt-sparkplug/README.md)
- [OPC-UA Client Driver](src/OPC-UA-Client/README.md)
- [OPC-UA Server Driver](src/OPC-UA-Server/README.md)
- [OPC-DA Client Driver](src/OPC-DA-Client/README.md)
- [PLC4X-GO Modbus Client Driver](src/plc4x-client/README.md)
- [CIP Ethernet/IP PLCTags Client Driver](src/libplctag/PLCTagsClient/README.md)
- [Calculations](src/calculations/README.md)
- [Change Stream Data Processor](src/cs_data_processor/README.md)
- [Custom Data Processor](src/cs_custom_processor/README.md)
- [Custom Developments](src/custom-developments/README.md)
- [Realtime Data Server](src/server_realtime_auth/README.md)
- [OSHMI2JSON Tool](src/oshmi2json/README.md)
- [Report Generators](docs/report_generators.md)
- [I104M Client Driver](src/i104m/README.md)
- [SAGE-web Displays](src/htdocs/sage-cepel-displays/README.md)

## Protocols Roadmap

- [x] IEC 60870-5-104 Server TCP/TLS
- [x] IEC 60870-5-104 Client TCP/TLS
- [x] IEC 60870-5-101 Server Serial/TCP
- [x] IEC 60870-5-101 Client Serial/TCP
- [ ] IEC 60870-5-103 Client
- [x] IEC 61850 MMS Client TCP/TLS
- [ ] IEC 61850 MMS Server
- [ ] IEC 61850 GOOSE/SV Client
- [x] DNP3 Client TCP/UDP/TLS/Serial - Windows x64 only!
- [x] DNP3 Server TCP/UDP/TLS/Serial
- [x] MQTT/Sparkplug-B PUB/SUB TCP/TLS
- [x] Modbus Client via PLC4X-GO
- [ ] ICCP Client TCP/TLS
- [ ] ICCP Server TCP/TLS
- [x] Telegraf Client (many data sources available such as MQTT, MODBUS, SNMP, ...)
- [x] OPC UA Client TCP/Secure
- [x] OPC UA Server TCP/Secure
- [ ] OPC UA Historical Data Server
- [x] OPC DA Client (Windows)
- [ ] OPC AE Client (Windows)
- [ ] OPC DA Server (Windows)
- [x] CIP Ethernet/IP (libplctag, experimental)
- [ ] Siemens S7
- [ ] BACNET
- [x] I104M (legacy adapter for some OSHMI drivers)
- [x] ONVIF Camera control and streaming

## Features Roadmap

- [x] Web-based Viewers
- [x] Web-based Configuration Manager
- [x] Excel-based Configuration
- [x] JWT Authentication
- [x] User auth/Role-based Access Control (RBAC)
- [x] LDAP/AD Authorization
- [x] Inkscape-based SVG Synoptic Editor
- [x] Compiled Cyclic Calculations Engine
- [ ] Low-latency/Asynchronous Calculations Engine
- [x] Customizable Change-Stream Processor (for user implemented scripts)
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
- [x] Install Script for Ubuntu 24.04 Linux x86-64 and arm64
- [ ] Linux Image / VM
- [x] Supervisor (Linux process manager) examples
- [x] Project IDX Configuration
- [ ] InfluxDB Integration
- [x] Telegraf Integration
- [x] PowerBI Integration (via PostgreSQL connector)
- [ ] PowerBI Direct Integration
- [ ] Kafka/Redpanda/Benthos Integration
- [ ] Eclipse 4diac
- [ ] Supabase Integration
- [ ] NodeRed Integration
- [ ] n8n Integration
- [ ] Alerta Integration (https://alerta.io/)
- [x] PLC4X-GO Integration (https://plc4x.apache.org/)
- [x] Example templates/API for fast AI-helped custom app developments
- [ ] Managed Cloud Service
- [ ] Supported LTS versions

## Spin up a free private instance on Google's Firebase Studio

With just a Google account, you can spin up a free private instance for test/dev on Google's Firebase Studio. This is a great way to get started with the project. This will build the code from the Github repo and deploy it to a private Linux VM on the cloud running protocols and providing a web UI for you to interact with. There will be a web-based code editor available for you to develop new apps and view/change the code on the VM. You can also get help from Google's Gemini AI for coding and other tasks. This is free and there no need to install any software on your local machine.

See details [here](platform-nix-idx/README.md). 

## Online Demo (substations simulation)

- http://150.230.171.172

This demo provides a public IEC 60870-5-104 server port on IP address 150.230.171.172:2404 (common address = 1) for testing.

The demo data is published as regular MQTT topics to the public broker mqtt://test.mosquitto.org:1883 (about 8600 topics in JsonScadaDemoVPS/# and ACME_Utility/#).

Data is also published as Sparkplug-B to mqtt://test.mosquitto.org:1883 (about 4300 device metrics in spBv1.0/Sparkplug B Devices/+/JSON-SCADA Server/#). Data/birth messages are compressed by Eclipse Tahu Javascript libs.

## Developer Contact

- https://www.linkedin.com/in/ricardo-olsen/
