# {json:scada}

![logo](https://github.com/riclolsen/json-scada/raw/master/src/htdocs/images/json-scada.svg "{json:scada} Logo")

A portable SCADA/IoT platform centered on the MongoDB database server.

## Major features and characteristics
* Standard IT tools applied to SCADA/IoT (MongoDB, PostgreSQL/TimescaleDB,Node.js, C#, Golang, Grafana, etc.).
* MongoDB as the real-time core database, persistence layer, config store, SOE historian.
* Portability and interoperability over Linux, Windows, x86/64, ARM.
* Horizontal scalability, from a single computer to big clusters (MongoDB-sharding), Docker containers, VMs, Kubernetes, cloud, or hybrid deployments.
* Unlimited tags, servers, and users.
* Modular distributed architecture. Lightweight redundant data acquisition nodes can connect securely over TLS to the database server. E.g. a Raspberry PI can be a data acquisition node.
* HTML5 Web interface. UTF-8/I18N. Mobile access.
* Inkscape-based SVG synoptic display editor.
* IEC60870-101/104 Client and Server protocols.
* PostgreSQL/TimescaleDB historian integrated with Grafana for easy creation of dashboards.
* Extensibility of data model (MongoDB: NoSQL/schema-less).
* Development of custom applications with modern stacks like MEAN/MERN, etc.
* Big data / ML capabilities through MongoDB Spark connector.
* Access to the MongoDB ecosystem of tools, community, services, etc.
* Easy to understand system with small code size for each independent module. Extensive use of JSON from bottom up.
* Possibility of easy integration of new and custom protocol drivers developed with modern programming languages.
* Future-proof, vendor independence, flexibility, extensibility.
* Reduced human costs for maintenance and development thanks to the employment of widely-used open-source IT technologies.
* Live configuration updates.
* Planned protocol drivers: OPC-UA, DNP3, MODBUS, MQTT, Ethernet/IP.
* Planned integrations: InfluxDB/Telegraf, NodeRed, MS Power BI.

## Use cases
* Power/Oil/Gas/etc Local Station HMI.
* Manufacturing Local HMI.
* SCADA Protocol Gateway.
* SCADA Control Center Full System.
* SCADA/IoT Historian. MS Power BI integration.
* Intranet/Internet HTTPS Gateway - Visualization Server.
* Multilevel Systems Integrator (SCADA/IoT/ERP/MES/PLC).
* Global-Level SCADA Systems Integration/Centralization.
* Extensible Development Platform For Data Acquisition And Processing.
* Data concentrator for Big Data / ML processing.

## Architecture
![architecture](https://github.com/riclolsen/json-scada/raw/master/docs/JSON-SCADA_ARCHITECTURE.png "{json:scada} Architecture")

## License
    {json:scada} A portable SCADA/IoT platform centered on the MongoDB database server.
    Copyright (C) 2020 Ricardo L. Olsen

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License version 3 as published by the Free Software Foundation.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.

## Contributors License Agreement - CLA

Contributors are welcome. Pull request authors must agree with this CLA.

You, the Contributor, accept and agree to the following terms and conditions for Your present and future Contributions submitted to this project. Except for the license granted herein to this project author, You reserve all right, title, and interest in and to Your Contributions.

Licenses

This project (code, documentation, and any other materials) is released under the terms of the individual licenses as noted in the project's repository, or, if no separate license is specified, under the terms of the GPL3 license.

You certify that:

* (a) Your Contributions are created in whole or in part by You and You have the right to submit it under the designated license; or

* (b) Your Contributions are based upon previous work that, to the best of your knowledge, is covered under an appropriate open source license and You have the right under that license to submit that work with modifications, whether created in whole or in part by You, under the designated license; or

* (c) Your Contributions are provided directly to You by some other person who certified (a) or (b) and You have not modified them.

* (d) You understand and agree that Your Contributions are public and that a record of the Contributions (including all metadata and personal information You submit with them) is maintained indefinitely in the project repositories and all its forks.

* (e) You are granting Your Contributions to this project under the terms of the license as noted in the project's repository.

* (f) Contributors must provide name, email or social network contact and the phrase "I AGREE with the Contributors License Agreement of the project.".
 
## Documentation

* [Docker Demo](demo-docker/README.md)
* [Calculations](src/calculations/README.md)
* [IEC60870-5-104 Server Driver](src/lib60870.netcore/iec104server/README.md)
* [IEC60870-5-104 Client Driver](src/lib60870.netcore/iec104client/README.md)
* [IEC60870-5-101 Server Driver](src/lib60870.netcore/iec101server/README.md)
* [IEC60870-5-101 Client Driver](src/lib60870.netcore/iec101client/README.md)
* [I104M Client Driver](src/lib60870.netcore/i104m/README.md)
* [Change Stream Data Processor](src/cs_data_processor/README.md)
* [Realtime Data Server](src/server_realtime/README.md)
* [SVG Synoptic Display Editor](src/svg-display-editor/README.md)
* [OSHMI2JSON Tool](src/oshmi2json/README.md)
* [Schema Documentation](docs/schema.md)

## Contact

https://www.linkedin.com/in/ricardo-olsen/