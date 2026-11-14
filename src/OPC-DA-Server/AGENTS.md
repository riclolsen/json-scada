# DOX: src/OPC-DA-Server — OPC DA Server Driver

## Purpose

Windows-only OPC DA (Data Access) server plugin for JSON-SCADA. Exposes JSON-SCADA real-time data to legacy OPC DA clients. Built as a .NET Framework plugin for the Technosoftware OPC DA server.

## Ownership

- OPC-DA-Server owns the OPC DA server protocol driver

## Local Contracts

- **Language:** C# .NET Framework
- **Solution:** `OPC-DA-Server.sln`
- **Plugin project:** `ServerPlugin/ServerPlugin.csproj`
  - `ServerPlugin.csproj` — project file with NuGet dependencies
  - `packages.config` — NuGet package references
  - `ClassicBaseNodeManager.cs` — base node manager for tag hierarchy
  - `ClassicNodeManager.cs` — node manager implementation
  - `MongoChangeStream.cs` — MongoDB Change Stream subscription
  - `MongoConfig.cs` — MongoDB configuration handling
  - `AssemblyInfo.cs` — assembly metadata
  - `app.config` — application configuration
- **Build tools:** `nuget.exe` for package restore
- **Registration:** `register.bat`, `unregister.bat` — COM registration for OPC DA
- **Run:** `run.bat` — starts the server
- **Dependency:** Technosoftware.DaAeHdaClient COM libraries (in `packages/`)
- **Keep-alive and stats:** no redundancy arbitration; `ClassicNodeManager` counts DA clients (`OnClientConnect`/`OnClientDisconnect`) and a 5 s timer (`PublishStatus`) updates the instance keep-alive and the connection's `stats.isConnected` (clients > 0) and `stats.clients`. Connections are documented and created with `stats: null`, and MongoDB cannot create fields under a null element (write error 28), so `PublishStatus` replaces that null with an empty document and writes the stats again

## Work Guidance

- Windows-only: requires COM/DCOM infrastructure
- The server plugin registers as a OPC DA server accessible to OPC DA clients
- Exposes MongoDB tags as OPC DA items organized in a browseable hierarchy
- OPC DA 2.0 and 3.0 compatible through the Technosoftware framework
- ClassisBaseNodeManager and ClassicNodeManager pattern from OPC Foundation samples
- COM registration is required — use `register.bat` with administrator privileges

## Verification

- Run `register.bat` as Administrator — COM registration succeeds
- Build the solution in Visual Studio or with MSBuild
- Test with an OPC DA client (e.g., Matrikon OPC Explorer, OPC DA Client in this project)
- Run `unregister.bat` to clean up COM registration after testing
