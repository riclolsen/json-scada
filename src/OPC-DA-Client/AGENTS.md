# DOX: src/OPC-DA-Client — OPC DA Client Driver

## Purpose

.NET Core OPC DA client driver for JSON-SCADA (Windows-only). Connects to OPC DA 2.0/3.0 servers, reads/writes tags using COM/DCOM, and synchronizes data with MongoDB.

## Ownership

- OPC-DA-Client owns the OPC DA client protocol driver

## Local Contracts

- **Language:** C# .NET Core 8.0+
- **Solution:** `OPC-DA-Client.sln` / `OPC-DA-Client.csproj`
- **Standard files:**
  - `Program.cs` — main entry point
  - `AsduReceiveHandler.cs` — ASDU-style message handling
  - `Common_srv_cli.cs` — shared server/client logic
  - `MongoCommands.cs` — MongoDB read/write operations
  - `MongoUpdate.cs` — MongoDB update helpers
  - `TagsCreation.cs` — tag auto-creation from OPC DA server
  - `Redundancy.cs` — connection failover support
- **Dependency:** OPC COM libraries via Technosoftware.DaAeHdaClient
- **Config:** INI file via Supervisor or environment variables
- **Build:** `dotnet build`
- **Commands (`rtCommand`/`rtCommandNoAck`):** `protocolSourceCommonAddress` is a `BsonValue` (string branch name from `server_realtime_auth`, or double for numeric addressing); a strict `BsonDouble` serializer throws on strings and kills the command change stream. Commands older than 10 s (`TotalSeconds` vs UTC now) or without `timeTag` are marked `cancelReason: "expired"`
- **Connection stats:** `Redundancy.cs` writes `stats.isConnected` = DA server session `IsConnected` for every connection (false while there is no session object)

## Work Guidance

- Windows-only due to COM/DCOM dependency
- Connects to local or remote OPC DA servers via DCOM
- Browses OPC DA server tag hierarchy for tag discovery
- Auto-creates JSON-SCADA tags from discovered OPC DA items
- Handles reconnection with exponential backoff
- Follows the standard JSON-SCADA .NET driver pattern

## Verification

- `dotnet build` — project builds cleanly
- Test with a local OPC DA server (e.g., OPC DA Server simulator)
- Verify tag discovery and data synchronization to MongoDB
