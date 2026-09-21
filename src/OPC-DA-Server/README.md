# OPC-DA Server Driver for JSON-SCADA

OPC-DA 2.05a / 3.00 server driver that exposes JSON-SCADA `realtimeData` tags as OPC-DA items, using the [Technosoftware Classic Server .NET](https://technosoftware.com/product/opc-classic-server-solution/) plugin architecture.

Tags whose `group1` property matches any value in the connection's `topics` array are made available to OPC-DA clients. Live value updates are delivered via a MongoDB change stream.

The server is registered by the Windows installer, there is no need to configure a Windows service. It will start automatically when the first client connects.

## Configuration

### protocolConnections document (MongoDB)

Use the ADMIN UI to create a new OPC-DA-Server instance and connection, or manually insert a documents into MongoDB.

Insert one document into the `protocolConnections` collection per server instance.  
Generate fresh GUIDs with PowerShell: `[guid]::NewGuid()`.

```js
db.protocolConnections.insertOne({
  protocolDriver:               "OPC-DA_SERVER",
  protocolDriverInstanceNumber: 1,
  protocolConnectionNumber:     200,
  name:                         "OPC-DA-SERVER #1",
  description:                  "JSON-SCADA OPC-DA Server",
  enabled:                      true,
  commandsEnabled:              true,

  // list group1 values to expose tags as OPC-DA variables
  // Leave empty array [] to expose ALL tags (no group1 filter)
  topics: ["KAW2", "KOR1"],

  // COM / DCOM registration identity — MUST be unique per running instance
  // Generate with: [guid]::NewGuid() in PowerShell
  clsIdServer:    "{XXXXXXXX-XXXX-XXXX-XXXX-XXXXXXXXXXXX}",
  clsIdApp:       "{YYYYYYYY-YYYY-YYYY-YYYY-YYYYYYYYYYYY}",
  prgIdServer:    "JsonScada.OpcDaServer1",
  prgIdCurrServer:"JsonScada.OpcDaServer1.1",
  serverName:     "JSON-SCADA OPC-DA Server Instance 1"
})
```

### protocolDriverInstances document (MongoDB)

```js
db.protocolDriverInstances.insertOne({
  protocolDriver:               "OPC-DA_SERVER",
  protocolDriverInstanceNumber: 1,
  enabled:                      true,
  logLevel:                     1,
  nodeNames:                    ["mainNode"],   // must match nodeName in json-scada.json
  activeNodeName:               "",
  activeNodeKeepAliveTimeTag:   new Date(),
  keepProtocolRunningWhileInactive: false
})
```

### Tag field mapping

| `realtimeData` field | OPC-DA item property |
|---|---|
| `group1 + "." + group2 + "." + group3 + "." + ungroupedDescription` | ItemID (dot-separated path) |
| `type` | Canonical data type (`digital`→VT_BOOL, `string`→VT_BSTR, `analog`→VT_R8 by default) |
| `protocolSourceASDU` | OPC canonical type override (`float`, `int32`, `uint16`, `boolean`, etc.) |
| `value` / `valueString` | Current value |
| `invalid` | OPC quality (Good / Bad) |
| `timeTagAtSource` | Item timestamp |
| `origin == "command"` | Item is writable (ReadWritable); writes insert into `commandsQueue` |
| `description` | OPC item property 101 (Description) |

## Multiple Instances

The `OpcNetDaServer.exe` executable can accept multiple client connections simultaneously, but multiple server instances can also be run on the same machine to distribute the load.
Each running instance of `OpcNetDaServer.exe` **must** have its own unique `clsIdServer` / `clsIdApp` / `prgIdServer` values registered in the Windows registry. Create separate `protocolConnections` documents with distinct GUIDs and different `protocolConnectionNumber` values, then launch a separate `OpcNetDaServer.exe` process for each.

## Item ID Format

Item IDs follow a dot-separated hierarchy mirroring the `group1` / `group2` / `group3` / `ungroupedDescription` fields:

```
Plant1.Substation_A.Feeders.Breaker_101_Status
Plant1.Substation_A.Feeders.Breaker_101_Voltage
Plant1.Control.CB101_Open
```

OPC-DA clients browse and subscribe using these item IDs.

## Prerequisites

| Requirement | Notes |
|---|---|
| Windows 10/11 or Server 2016+ | DCOM requires Windows |
| .NET Framework 4.8 | Installed on all modern Windows |
| Visual Studio 2022 | For building the plugin DLL |
| MongoDB 6.0+ (replica set) | Change streams require a replica set |
| Technosoftware OpcNetDaServer.exe | From the ClassicServerSolutions distribution |

## Directory Layout

```
OPC-DA-Server/
├── OPC-DA-Server.sln
├── register.bat          ← register COM server (run as Admin)
├── unregister.bat        ← unregister COM server (run as Admin)
├── run.bat               ← start server in foreground for debug
├── bin/
│   ├── x86/Release/      ← OpcNetDaServer.exe + ServerPlugin.dll (x86)
│   └── x64/Release/      ← OpcNetDaServer.exe + ServerPlugin.dll (x64)
└── ServerPlugin/
    ├── ServerPlugin.csproj
    ├── ClassicBaseNodeManager.cs   (Technosoftware base — unmodified)
    ├── ClassicNodeManager.cs       (JSON-SCADA plugin logic)
    ├── MongoConfig.cs              (config models + BSON serializers)
    └── MongoChangeStream.cs        (live update watcher)
```

## Build

### 1. Restore NuGet packages

```bat
cd ServerPlugin
nuget restore ServerPlugin.csproj -PackagesDirectory ..\packages
```

Or open `OPC-DA-Server.sln` in Visual Studio 2022 and let NuGet auto-restore.

### 2. Build the generic server executable

`OpcNetDaServer.exe` is the Technosoftware generic C++/CLI server (`OpcNetDaAeServer.vcxproj`) from `src\ClassicServerSolutions`. It is not part of `OPC-DA-Server.sln`, so build it first (use `/p:Platform=Win32` for x86):

```bat
msbuild ..\ClassicServerSolutions\src\Technosoftware\Server\ClassicServer\OpcNetDaAeServer.vcxproj /p:Configuration=Release /p:Platform=x64 /p:SolutionDir=%CD%\..\ClassicServerSolutions\
```

### 3. Build the plugin

```bat
msbuild OPC-DA-Server.sln /p:Configuration=Release /p:Platform=x64
```

The post-build step copies `ServerPlugin.dll` and the MongoDB runtime DLLs to `bin\<platform>\Release\`, and copies `OpcNetDaAeServer.exe` there renamed to `OpcNetDaServer.exe`. If the executable from step 2 is missing, the build prints a warning and `bin\` has no exe. `build.bat` runs steps 1–3.

