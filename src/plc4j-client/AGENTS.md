# DOX: src/plc4j-client — PLC4J Client Driver (PLC4X for Java)

## Purpose

Java-based generic PLC client driver using Apache PLC4X/PLC4J. This is the shipped implementation of protocolDriver "PLC4X"; preserve existing instance, connection and tag configurations, including those from the retired Go implementation.

## Ownership

- plc4j-client owns the Java implementation of the PLC4X protocol driver

## Local Contracts

- **Language:** Java 17+ (Temurin suggested)
- **Structure:**
  - `src/main/java/org/jsonscada/plc4jclient/Plc4jClient.java` — main: wiring, independent heartbeat scheduler, supervision loop, asynchronous PLC connect/failover
  - `MongoWriter.java` — Mongo lifecycle + bulk-write queue consumer
  - `CommandsWatcher.java` — commandsQueue change stream → PLC writes
  - `ScanTask.java` — per-connection periodic integrity reads
  - `ValueExtractor.java` / `EndianUtils.java` — PlcValue → JSON-SCADA value tuple
  - `AutoTagCreator.java` / `RtDataTagDefaults.java` — automatic tag creation (key space connNumber*1e6)
  - `RedundancyManager.java` — active/inactive node state machine
  - `ConfigLoader.java` / `MongoConnector.java` — args/env/config file, Mongo client settings
  - `MongoTls.java` — SSLContext from CA PEM, client PEM or PKCS#12 material
- **PLC4X version:** 1.0.0 release, pinned in `pom.xml` property `plc4x.version`. The `plc4x-dev` profile (`mvn -Pplc4x-dev package`) builds against the PLC4X development line (1.1.0-SNAPSHOT from the ASF snapshot repository); the default stays on the release so builds are reproducible. BACnet/C-Bus drivers exist nowhere in the 1.x line (last published at 0.13.1) and the transport artifacts are `plc4j-transports-*`, not the old singular `plc4j-transport-*`.
- **Dependency management:** Maven (`pom.xml`), fat jar via maven-shade-plugin
- **Build:** `mvn package` → `target/plc4j-client.jar` (copied to `bin/` with launcher scripts)
- **Config:** `conf/json-scada.json` + MongoDB collections (protocolDriverInstances, protocolConnections, realtimeData, commandsQueue)
- **Connection stats:** `Plc4jClient.updateConnectionStats` writes `stats.nodeName`, `stats.timeTag` and `stats.isConnected` = `PlcConnection.isConnected()` each supervision-loop pass on the active node. Connections are documented and created with `stats: null`, and MongoDB cannot create fields under a null element, so the dotted update replaces that null with an empty document and retries once

## Work Guidance

- Preserve the established PLC4X topics format, sourceDataUpdate shape, auto-tag key allocation and command bookkeeping.
- Redundancy runs every second independently of PLC connect/ping and stats writes. Only consecutive unchanged heartbeats count toward takeover; updates compare the observed owner and heartbeat before claiming. Database/ownership failure deactivates the node.
- Commands run serially per connection; inactive, stopped or retired handlers must neither dispatch nor cancel commands. Reopen failed change streams. Synthetic array-element addresses are explicitly cancelled; use a scalar PLC address for commands. Reject numeric overflow and non-finite command values.
- Bind each scan task to its original PlcConnection; stale tasks must never close a replacement. Retire and close all old connections when a Mongo session ends. Cancel timed-out reads/pings/writes; recycle a connection after three consecutive read failures.
- Failed/short array responses invalidate every configured element. Non-finite real samples are bad quality and carry valid JSON.
- Auto-tag counters use long arithmetic and retain BSON double IDs within the connection's million-key range.
- Mongo TLS certificate files enable TLS; validation switches alone preserve the URI TLS setting. Apply CA/client material through SSLContext, never unsupported URI file options.
- Document-level unordered bulk errors are logged without tearing down MongoDB; transport failures reconnect.
- The fourth CLI argument is accepted with an explicit unsupported-point-filter message; it does not filter acquisition.
- The shade plugin's ServicesResourceTransformer is CRITICAL: PLC4J drivers register via META-INF/services; without it only one protocol survives in the fat jar.
- Auto-created tag `_id`s must be BSON doubles.
- Array addresses: PLC4X 1.0.0 moved the array notation before the type and made it a range (`holding-register:20[0..9]:INT`). `TopicParser.toPlc4xAddress()` translates the legacy `:TYPE[count]` form so existing configs and legacy topics keep working; the configured text stays the PLC4X tag name and the realtimeData key, so never send the translated address as the tag name.
- API note: 1.0.0 replaced `PlcDriverManager.getConnectionManager()` with `getConnectionFactory()`; the 1.1.0 development line needs no further source change.
- Check for newer PLC4X code by listing the version directory (`repo1.maven.org/maven2/org/apache/plc4x/plc4j-api/`) instead of trusting `maven-metadata.xml`, which is served from a CDN cache and can lag a release by weeks.
- Endianness: LITTLE_ENDIAN/REV_ENDIAN swap bytes of the PLC4X-decoded value; BIG_ENDIAN/empty is a no-op.

## Verification

- `mvn package` — runs JUnit regression tests (redundancy, command guards/ranges, TLS keys, scan timeout cancellation, lifecycle, parsing, endian conversion and array quality), compiles and produces the shaded jar
- `mvn -Pplc4x-dev package` — same against the PLC4X development line, to catch upstream API drift early
- `java -jar target/plc4j-client.jar` — prints banner and the list of registered PLC4X drivers (verifies service-file merging)
- Test with a Modbus simulator (e.g. diagslave) or real PLC against a MongoDB with a PLC4X instance configured

## Child DOX Index

- None. This document owns implementation, tests, TLS fixtures, build scripts and README.
