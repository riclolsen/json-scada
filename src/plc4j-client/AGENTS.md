# DOX: src/plc4j-client — PLC4J Client Driver (PLC4X for Java)

## Purpose

Java-based generic PLC client driver using the Apache PLC4X/PLC4J library. Registers in MongoDB as protocolDriver "PLC4X" — it is a drop-in alternative executable for the Go driver in `src/plc4x-client`, sharing the same instance/connection/tag configuration. Only one of the two executables may run per instance number.

## Ownership

- plc4j-client owns the Java implementation of the PLC4X protocol driver

## Local Contracts

- **Language:** Java 17+ (Temurin suggested)
- **Structure:**
  - `src/main/java/org/jsonscada/plc4jclient/Plc4jClient.java` — main: wiring, supervision loop, PLC connect/failover
  - `MongoWriter.java` — Mongo lifecycle + bulk-write queue consumer
  - `CommandsWatcher.java` — commandsQueue change stream → PLC writes
  - `ScanTask.java` — per-connection periodic integrity reads
  - `ValueExtractor.java` / `EndianUtils.java` — PlcValue → JSON-SCADA value tuple
  - `AutoTagCreator.java` / `RtDataTagDefaults.java` — automatic tag creation (key space connNumber*1e6)
  - `RedundancyManager.java` — active/inactive node state machine
  - `ConfigLoader.java` / `MongoConnector.java` — args/env/config file, TLS conn string
- **PLC4X version:** 1.0.0 (pinned in `pom.xml` property `plc4x.version`). BACnet/C-Bus drivers and the old singular `plc4j-transport-*` artifacts do not exist at 1.0.0 (transports are now `plc4j-transports-*`).
- **Dependency management:** Maven (`pom.xml`), fat jar via maven-shade-plugin
- **Build:** `mvn package` → `target/plc4j-client.jar` (copied to `bin/` with launcher scripts)
- **Config:** `conf/json-scada.json` + MongoDB collections (protocolDriverInstances, protocolConnections, realtimeData, commandsQueue)

## Work Guidance

- Behavior must stay in functional parity with the Go plc4x-client (topics format, sourceDataUpdate shape, auto-tag key allocation, command handling) — both serve the same "PLC4X" driver configuration.
- The shade plugin's ServicesResourceTransformer is CRITICAL: PLC4J drivers register via META-INF/services; without it only one protocol survives in the fat jar.
- Auto-created tag `_id`s must be BSON doubles.
- Array addresses: PLC4X 1.0.0 moved the array notation before the type and made it a range (`holding-register:20[0..9]:INT`). `TopicParser.toPlc4xAddress()` translates the legacy `:TYPE[count]` form so existing configs and Go-driver-compatible topics keep working; the configured text stays the PLC4X tag name and the realtimeData key, so never send the translated address as the tag name.
- API note: 1.0.0 replaced `PlcDriverManager.getConnectionManager()` with `getConnectionFactory()`.
- Endianness: LITTLE_ENDIAN/REV_ENDIAN swap bytes of the PLC4X-decoded value; BIG_ENDIAN/empty is a no-op.

## Verification

- `mvn package` — compiles and produces the shaded jar without errors
- `java -jar target/plc4j-client.jar` — prints banner and the list of registered PLC4X drivers (verifies service-file merging)
- Test with a Modbus simulator (e.g. diagslave) or real PLC against a MongoDB with a PLC4X instance configured
