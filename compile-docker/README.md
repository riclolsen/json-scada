# Docker Compilation

Use this docker compose script to build, for the build host architecture (Linux x64 or ARM64), everything the [Docker demo](../demo-docker/README.md) runs. The services mirror the build stages of the project root `Dockerfile`.

    docker compose up

- `go_compile` — Go drivers and processes, built as static binaries into `demo-docker/bin`: `calculations`, `cs_data_processor` (Go), `iec104client`, `iec104server`, `iec101client`, `iec101server`, `iec103client`, `iec61850_client`, `iec61850_server`, `opcua-client`, `dnp3-client`, `dnp3-server`, `modbus-client`, `modbus-server`. Also copies the prebuilt `iccp-client` and `iccp-server` binaries.
- `plc4j_compile` — PLC4J client (Java) into `demo-docker/bin`: `plc4j-client.jar` and `plc4j-client.sh`.
- `nodejs_update` — `npm install` (and builds) of the Node.js modules, the AdminUI, svgedit and the custom developments, in place under `src/`.

A single service can be run alone, e.g. `docker compose up go_compile`.
