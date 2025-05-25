# JSON SCADA Protocol Driver Developer Guide

## Introduction

Welcome to the JSON SCADA Protocol Driver Developer Guide! This document provides essential information and guidelines for creating new protocol drivers to extend the capabilities of the JSON SCADA platform.

Protocol drivers are crucial components in JSON SCADA. They act as bridges between the central JSON SCADA system and external devices, systems, or data sources that use various communication protocols. The primary role of a driver is to:

*   **Acquire data:** Read data from external sources and update the relevant tags in the JSON SCADA real-time database (MongoDB).
*   **Execute commands:** Send commands initiated from JSON SCADA to the external devices/systems.
*   **Manage configuration:** Read their specific operational parameters and connection details from the MongoDB configuration collections.

JSON SCADA's core is built around MongoDB, which serves as the central hub for:

*   **Real-time Data:** Storing current values, timestamps, and quality status for all tags.
*   **Configuration:** Hosting settings for driver instances, specific connections, and individual tag parameters.
*   **Commands:** Queuing commands to be picked up by drivers and sent to devices.

This guide will walk you through the architecture, common patterns, and language-specific considerations for developing protocol drivers in C#, Go, and Node.js.

## Core Architecture Concepts

Understanding the core architectural concepts of JSON SCADA is essential before diving into driver development. Drivers are standalone processes that interact with the central MongoDB database for most oftheir operations.

### Main Configuration File (`json-scada.json`)

While individual driver configurations are stored in MongoDB, there's a primary JSON configuration file, typically named `conf/json-scada.json`. Drivers need to read this file at startup to obtain:

*   **MongoDB Connection String:** The URI to connect to the MongoDB server.
*   **MongoDB Database Name:** The specific database within MongoDB that JSON SCADA uses.
*   **Current Node Name:** An identifier for the machine or container the driver is running on. This is important for redundancy management.

The path to this file is usually passed as a command-line argument to the driver. If not provided, drivers often default to a relative path like `../conf/json-scada.json`.

### `protocolDriverInstances` Collection

This MongoDB collection defines runnable instances of protocol drivers. Each document in this collection represents a specific driver process that can be launched.

Key fields for a new driver:

*   `protocolDriver` (String): The unique name of your protocol driver (e.g., "MY_CUSTOM_PROTOCOL"). This name is used to find the relevant instance configuration.
*   `protocolDriverInstanceNumber` (Double): A numerical identifier for the instance (e.g., 1, 2). Allows running multiple instances of the same driver, possibly with different configurations.
*   `enabled` (Boolean): If `true`, this driver instance is active and should run.
*   `logLevel` (Double): Defines the verbosity of logging for the driver instance (e.g., 0 for minimal, 1 for basic, 2 for detailed, 3 for debug).
*   `nodeNames` (Array of Strings): A list of node names (matching the `nodeName` from `json-scada.json`) that are permitted to run this driver instance. This is key for redundancy.
*   `activeNodeName` (String): Automatically managed by active driver instances to indicate which node is currently responsible for this instance's tasks.
*   `activeNodeKeepAliveTimeTag` (Date): Timestamp updated by the active driver instance to signal it's alive. Inactive instances monitor this to detect failures of the active node.
*   `keepProtocolRunningWhileInactive` (Boolean): Typically `false`. Determines if an inactive redundant instance should still maintain its protocol connections.

### `protocolConnections` Collection

This collection stores the specific connection parameters for each driver instance. A single driver instance can manage multiple connections to different devices or endpoints.

Key fields:

*   `protocolDriver` (String): The name of the protocol driver (must match the one in `protocolDriverInstances`).
*   `protocolDriverInstanceNumber` (Double): Links this connection configuration to a specific driver instance.
*   `protocolConnectionNumber` (Double): **A system-wide unique number** identifying this specific connection. Tags in the `realtimeData` collection use this number to associate themselves with a data source or command target.
*   `name` (String): A user-friendly name for the connection (e.g., "PLC_AREA_51").
*   `enabled` (Boolean): If `true`, this connection should be actively managed by the driver.
*   `commandsEnabled` (Boolean): If `true`, the driver should process commands for tags associated with this connection.
*   **Protocol-Specific Fields:** Additional fields required by your specific protocol (e.g., `endpointURL`, `ipAddress`, `port`, etc.). These are defined and used by your driver. See if existing fields can be used by your driver, it may be necessary to create new fields.

### `realtimeData` Collection (Tags)

This is the central repository for all data points (tags) in JSON SCADA. Drivers interact with documents in this collection to update values and read configuration for specific tags.

To link a tag to your driver for **data acquisition**:

*   `protocolSourceConnectionNumber` (Double): Must match the `protocolConnectionNumber` of one of the connections your driver instance is configured to handle.
*   `protocolSourceObjectAddress` (String/Double): A string that your driver understands as the unique address or identifier of the data point within the external device/protocol (e.g., an OPC-UA NodeId, a Modbus register address, an MQTT topic, a DNP3 point index).
*   `protocolSourceASDU` (String): Type of Application Service Data Unit. A string indicating the data type or format as understood by your protocol (e.g., "UINT16", "FLOAT32", "DIGITAL_INPUT", "JSON"). This helps your driver interpret the data correctly.
*   `protocolSourceCommonAddress` (String/Double): Used to identify groups of objects that can have the same `protocolSourceObjectAddress` (e.g., DNP3 can have repeated addresses for binary inputs (group 1) and analog inputs (group 30).
*   `protocolSourceCommandUseSBO` (Boolean): Used to flag commands that require the Select-Before-Operate procedure.
*   `protocolSourceCommandDuration` (Double): Used to specify additional command options like duration of pulse.
*   Other fields like `kconv1`, `kconv2` (for scaling), and protocol-specific polling/subscription parameters might also be used by the driver if relevant.

When your driver reads a new value for a tag, it should update the `sourceDataUpdate` sub-document within the tag's document in `realtimeData`. This sub-document typically includes:
    *   `valueAtSource` (appropriate BSON type for the value)
    *   `valueStringAtSource` (String representation)
    *   `valueJsonAtSource` (JSON representation)
    *   `timeTagAtSource` (Date, timestamp from the source, if available)
    *   `timeTagAtSourceOk` (Boolean, true if `timeTagAtSource` is reliable)
    *   `invalidAtSource` (Boolean, true if the value is considered invalid by the source)
    *   `timeTag` (Date, timestamp when the driver processed the update)
    *   `originator` (String, identifies the source, e.g., "MY_DRIVER|conn_123")
    *   `causeOfTransmissionAtSource` (String, specify the cause of transmission, e.g. "3"=Spontaneous in IEC60870-5-101/104.
    *   `asduAtSource` (String, type representation of the data as detected by the protocol driver, e.g. "M_ME_NC_1".
    *   Other quality flags like `notTopicalAtSource`, `substitutedAtSource`, `blockedAtSource` as relevant.

To link a tag for **command execution**:

*   The same fields (`protocolSourceConnectionNumber`, `protocolSourceObjectAddress`, `protocolSourceASDU`) are used. The driver logic differentiates based on the tag's nature (often indicated by an `origin` field like "command" or by being present in the `commandsQueue`).

### `commandsQueue` Collection

When a user or another process in JSON SCADA wants to send a command to a device, it creates a new document in the `commandsQueue` collection. Your driver needs to monitor this collection for relevant commands, this is usually done via the changestream mechanism of mongodb.

Key fields in a `commandsQueue` document that your driver will use:

*   `protocolSourceConnectionNumber` (Double/Int): Your driver should only process commands matching one of its active connections.
*   `protocolSourceObjectAddress` (String): The protocol-specific address of the target point for the command.
*   `protocolSourceASDU` (String): The data type of the value to be written.
*   `value` (appropriate BSON type): The value to be written to the device.
*   `valueString` (String): String representation of the value.
*   `_id` (ObjectId): The unique ID of the command.

After processing a command, your driver should update the command document in `commandsQueue` to indicate its status (e.g., by setting `delivered: true`, `ack: true`, or adding error information).

## Implementing a New Protocol Driver

This section details the common logical flow and language-specific considerations for creating a new protocol driver.

### Common Driver Logic (Conceptual Overview)

Regardless of the programming language, most protocol drivers will follow a similar operational lifecycle:

1.  **Startup and Initialization:**
    *   **Parse Command-Line Arguments:** Drivers are typically launched with arguments specifying:
        *   The `protocolDriverInstanceNumber` they should run as.
        *   The desired `logLevel`.
        *   The path to the main `json-scada.json` configuration file.
    *   **Read Main Configuration:** Load `json-scada.json` to get:
        *   MongoDB connection string.
        *   MongoDB database name.
        *   The current `nodeName` where the driver is running.
    *   **Connect to MongoDB:** Establish a connection to the MongoDB server.
    *   **Load Driver Instance Configuration:** Query the `protocolDriverInstances` collection for the document matching its `protocolDriver` name and `protocolDriverInstanceNumber`. If not found or not `enabled`, the driver should exit. Store the `logLevel` and `nodeNames` from this configuration.
    *   **Load Connection Configurations:** Query the `protocolConnections` collection for all documents matching its `protocolDriver` name and `protocolDriverInstanceNumber` that are marked as `enabled`. Store these connection details (including `protocolConnectionNumber`, `commandsEnabled`, and any protocol-specific parameters).

2.  **Redundancy Management:**
    *   Implement a loop that periodically (e.g., every 5-10 seconds) interacts with the driver's document in the `protocolDriverInstances` collection to manage redundancy.
    *   **Check Active Node:** Read the `activeNodeName` and `activeNodeKeepAliveTimeTag` from its instance document.
    *   **Become Active:** If `activeNodeName` is empty, or if `activeNodeName` is different from the current `nodeName` and `activeNodeKeepAliveTimeTag` is older than a defined timeout (e.g., 30-60 seconds), the current instance should attempt to become active. It does this by updating its instance document, setting `activeNodeName` to its own `nodeName` and `activeNodeKeepAliveTimeTag` to the current time.
    *   **Maintain Active Status:** If the current instance is already the `activeNodeName`, it should periodically update `activeNodeKeepAliveTimeTag` to the current time to signal it's still alive.
    *   **Remain Inactive:** If another node is active and its `activeNodeKeepAliveTimeTag` is recent, the current instance remains inactive. Depending on the `keepProtocolRunningWhileInactive` flag (usually false), it might close its protocol connections.

3.  **Main Data Acquisition Loop (when active):**
    *   This is the core loop where the driver interacts with external devices.
    *   Iterate through each loaded and `enabled` connection from its `protocolConnections` configuration.
    *   **Establish Connection:** For each connection, establish and maintain communication with the external device/server using the protocol-specific parameters. Handle connection errors, retries, and reconnections.
    *   **Read Data:** Based on the tags in `realtimeData` that are linked to the current `protocolConnectionNumber` (via `protocolSourceConnectionNumber`), read data from the device. This might involve:
        *   Polling data at regular intervals.
        *   Subscribing to data changes (if the protocol supports it).
        *   Handling unsolicited messages from the device.
    *   **Update `realtimeData`:** When new data is received for a tag:
        *   Construct the `sourceDataUpdate` sub-document containing fields like `valueAtSource`, `valueStringAtSource`, `timeTagAtSource`, `invalidAtSource`, and other relevant quality flags.
        *   Update the corresponding tag document in the `realtimeData` collection using the `protocolSourceConnectionNumber` and `protocolSourceObjectAddress` to identify the correct tag.
        *   Ensure updates are efficient (e.g., using bulk updates if many tags change simultaneously).

4.  **Command Handling Loop (when active and `commandsEnabled` for the connection):**
    *   Monitor the `commandsQueue` collection for new documents. This is typically done using MongoDB Change Streams for real-time notifications.
    *   **Filter Commands:** Process only those commands where the `protocolSourceConnectionNumber` matches one of the driver's active connections for which `commandsEnabled` is true.
    *   **Parse Command:** Extract `protocolSourceObjectAddress`, `protocolSourceASDU`, and `value` (and `valueString`) from the command document.
    *   **Send Command:** Translate these details into a protocol-specific command and send it to the appropriate device.
    *   **Update Command Status:** Update the command document in `commandsQueue` to reflect the outcome (e.g., `delivered: true`, `ack: true`, or set an error message field if the command failed). This provides feedback to the originator of the command.

5.  **Graceful Shutdown:**
    *   The driver should handle signals (like SIGINT, SIGTERM) to shut down gracefully.
    *   This includes closing MongoDB connections, stopping protocol connections, and completing any pending operations if possible.
    *   If the driver is the active node, it might optionally clear its `activeNodeName` in `protocolDriverInstances` to allow a standby instance to take over more quickly, though the timeout mechanism will handle this eventually.

This conceptual flow provides a robust framework. The specific implementation details will vary based on the chosen programming language and the intricacies of the target protocol.

### C# Driver Implementation

When developing a protocol driver in C#, you'll typically use the official MongoDB.Driver for database interactions and standard .NET libraries for other functionalities.

**Project Structure:**

*   A typical C# driver is a console application.
*   Solution file (`.sln`)
*   Project file (`.csproj`) targeting a .NET version compatible with other JSON SCADA components (e.g., .NET 6, .NET 8).
*   `Program.cs`: Main entry point.
*   Separate `.cs` files for different concerns (e.g., `MongoLogic.cs`, `ProtocolHandler.cs`, `ConfigClasses.cs`).

**Recommended Libraries:**

*   `MongoDB.Driver`: For all interactions with MongoDB.
*   `System.Text.Json`: For deserializing JSON configuration from `json-scada.json` and potentially for handling JSON-based protocol data.

**Example Snippets (Conceptual):**

*   **Reading `json-scada.json` and Main Configuration:**
    ```csharp
    // In Program.cs or a config loading class
    public class JsonScadaConfig
    {
        public string mongoConnectionString { get; set; }
        public string mongoDatabaseName { get; set; }
        public string nodeName { get; set; }
        // Add other fields as needed, e.g., LogLevel from command line
        public int driverInstanceNum {get; set; }
        public int logLevelNum {get; set; }
    }

    // ...
    // string configFilePath = args.Length > 2 ? args[2] : "../conf/json-scada.json";
    // string jsonString = File.ReadAllText(configFilePath);
    // JsonScadaConfig mainConfig = JsonSerializer.Deserialize<JsonScadaConfig>(jsonString);
    // mainConfig.driverInstanceNum = instanceNumFromArgs;
    // mainConfig.logLevelNum = logLevelFromArgs;
    ```

*   **Connecting to MongoDB:**
    ```csharp
    // Using MongoDB.Driver
    // MongoClient client = new MongoClient(mainConfig.mongoConnectionString);
    // IMongoDatabase database = client.GetDatabase(mainConfig.mongoDatabaseName);
    // IMongoCollection<BsonDocument> instancesCollection = database.GetCollection<BsonDocument>("protocolDriverInstances");
    // IMongoCollection<BsonDocument> connectionsCollection = database.GetCollection<BsonDocument>("protocolConnections");
    // IMongoCollection<BsonDocument> realtimeDataCollection = database.GetCollection<BsonDocument>("realtimeData");
    // IMongoCollection<BsonDocument> commandsQueueCollection = database.GetCollection<BsonDocument>("commandsQueue");
    ```

*   **Loading Driver Instance & Connection Configs (Illustrative):**
    ```csharp
    // Define classes to represent your MongoDB documents
    public class ProtocolDriverInstance // Simplified
    {
        public ObjectId _id { get; set; }
        public string protocolDriver { get; set; }
        public int protocolDriverInstanceNumber { get; set; }
        public bool enabled { get; set; }
        public List<string> nodeNames { get; set; }
        public string activeNodeName { get; set; }
        public DateTime activeNodeKeepAliveTimeTag { get; set; }
        public int logLevel { get; set; }
    }

    public class MyProtocolConnection // Simplified, add protocol-specific fields
    {
        public ObjectId _id { get; set; }
        public string protocolDriver { get; set; }
        public int protocolDriverInstanceNumber { get; set; }
        public int protocolConnectionNumber { get; set; }
        public bool enabled { get; set; }
        public bool commandsEnabled { get; set; }
        public string targetIpAddress { get; set; } // Example specific field
        public int targetPort { get; set; }      // Example specific field
    }

    // ...
    // var instanceFilter = Builders<ProtocolDriverInstance>.Filter.Eq(pdi => pdi.protocolDriver, "MY_CUSTOM_PROTOCOL") &
    //                      Builders<ProtocolDriverInstance>.Filter.Eq(pdi => pdi.protocolDriverInstanceNumber, mainConfig.driverInstanceNum);
    // ProtocolDriverInstance driverInstance = instancesCollection.Find(instanceFilter).FirstOrDefault();
    // if (driverInstance == null || !driverInstance.enabled) { /* log and exit */ }

    // var connectionFilter = Builders<MyProtocolConnection>.Filter.Eq(pc => pc.protocolDriver, "MY_CUSTOM_PROTOCOL") &
    //                        Builders<MyProtocolConnection>.Filter.Eq(pc => pc.protocolDriverInstanceNumber, mainConfig.driverInstanceNum) &
    //                        Builders<MyProtocolConnection>.Filter.Eq(pc => pc.enabled, true);
    // List<MyProtocolConnection> activeConnections = connectionsCollection.Find(connectionFilter).ToList();
    ```

*   **Implementing Redundancy Check:**
    ```csharp
    // In a separate thread or async task
    // bool isActive = false;
    // while (true)
    // {
    //     // Read driverInstance from MongoDB (as above)
    //     if (driverInstance.activeNodeName == mainConfig.nodeName)
    //     {
    //         isActive = true;
    //         // Update activeNodeKeepAliveTimeTag
    //         var update = Builders<ProtocolDriverInstance>.Update.Set(pdi => pdi.activeNodeKeepAliveTimeTag, DateTime.UtcNow);
    //         instancesCollection.UpdateOne(instanceFilter, update);
    //     }
    //     else if (string.IsNullOrEmpty(driverInstance.activeNodeName) ||
    //              (driverInstance.activeNodeKeepAliveTimeTag < DateTime.UtcNow.AddSeconds(-60))) // 60s timeout
    //     {
    //         // Attempt to become active
    //         var update = Builders<ProtocolDriverInstance>.Update
    //             .Set(pdi => pdi.activeNodeName, mainConfig.nodeName)
    //             .Set(pdi => pdi.activeNodeKeepAliveTimeTag, DateTime.UtcNow);
    //         var result = instancesCollection.UpdateOne(instanceFilter, update); // Consider optimistic concurrency if needed
    //         isActive = result.ModifiedCount > 0;
    //     }
    //     else
    //     {
    //         isActive = false;
    //     }
    //     Thread.Sleep(10000); // Check every 10 seconds
    // }
    ```

*   **Updating `realtimeData`:**
    ```csharp
    // var rtFilter = Builders<BsonDocument>.Filter.Eq("protocolSourceConnectionNumber", connection.protocolConnectionNumber) &
    //                Builders<BsonDocument>.Filter.Eq("protocolSourceObjectAddress", "tag_address_from_device");

    // var sourceUpdateDoc = new BsonDocument
    // {
    //     { "valueAtSource", new BsonDouble(123.45) },
    //     { "valueStringAtSource", "123.45" },
    //     { "timeTagAtSource", new BsonDateTime(DateTime.UtcNow) }, // Or device timestamp
    //     { "timeTagAtSourceOk", true },
    //     { "invalidAtSource", false },
    //     { "timeTag", new BsonDateTime(DateTime.UtcNow) },
    //     { "originator", $"MY_CUSTOM_PROTOCOL|{connection.protocolConnectionNumber}" }
    // };
    // var rtUpdate = Builders<BsonDocument>.Update.Set("sourceDataUpdate", sourceUpdateDoc);
    // realtimeDataCollection.UpdateOne(rtFilter, rtUpdate, new UpdateOptions { IsUpsert = false }); // Typically do not upsert from driver
    ```

*   **Watching `commandsQueue` (Change Stream Example):**
    ```csharp
    // var pipeline = new EmptyPipelineDefinition<ChangeStreamDocument<BsonDocument>>()
    //     .Match(csDoc => csDoc.OperationType == ChangeStreamOperationType.Insert);
    //     // Further match on fullDocument for protocolSourceConnectionNumber if possible,
    //     // or filter after receiving the change.

    // using (var cursor = commandsQueueCollection.Watch(pipeline))
    // {
    //     foreach (var change in cursor.ToEnumerable())
    //     {
    //         var commandDoc = change.FullDocument;
    //         int cmdConnNumber = commandDoc["protocolSourceConnectionNumber"].AsInt32;
    //         // If cmdConnNumber matches one of this driver's active connections...
    //         // Process command...
    //         // Update commandDoc in commandsQueueCollection with status
    //         // e.g., Builders<BsonDocument>.Update.Set("delivered", true).Set("ackTimeTag", DateTime.UtcNow)
    //     }
    // }
    ```

**Key Considerations for C#:**

*   **Thread Management:** Use `Thread` or `Task` for background operations like redundancy checks, protocol communication for each connection, and command queue monitoring. Ensure proper synchronization if shared data is accessed.
*   **Error Handling:** Implement robust error handling for MongoDB operations and protocol communication (e.g., `try-catch` blocks, logging).
*   **Configuration Classes:** Define C# classes that map to the structure of your MongoDB documents for easier deserialization and manipulation.
*   **Logging:** Implement logging using a simple console logger or integrate with a more advanced logging framework if desired, respecting the `logLevel` from the configuration.

### Go Driver Implementation

Go is well-suited for protocol driver development due to its concurrency features (goroutines and channels) and performance.

**Project Structure:**

*   Typically a single main directory for the driver.
*   `go.mod` file for managing dependencies.
*   `main.go` (or a similar name) for the main application entry point.
*   Separate `.go` files for different packages/modules if the driver becomes complex (e.g., `config/config.go`, `mongodb/mongo_handler.go`, `protocol/protocol_logic.go`).

**Recommended Libraries:**

*   `go.mongodb.org/mongo-driver/mongo`: The official MongoDB Go driver.
*   `go.mongodb.org/mongo-driver/mongo/options`: For MongoDB client and operation options.
*   `go.mongodb.org/mongo-driver/bson`: For working with BSON data.
*   `encoding/json`: For deserializing `json-scada.json`.
*   Standard library packages like `os`, `log`, `time`, `context`, `sync`.

**Example Snippets (Conceptual):**

*   **Reading `json-scada.json` and Main Configuration:**
    ```go
    // config/config.go or main.go
    type JsonScadaConfig struct {
        MongoConnectionString    string `json:"mongoConnectionString"`
        MongoDatabaseName        string `json:"mongoDatabaseName"`
        NodeName                 string `json:"nodeName"`
        DriverInstanceNum        int    // Populated from args
        LogLevelNum              int    // Populated from args
    }

    // ...
    // configFilePath := "../conf/json-scada.json"
    // if len(os.Args) > 3 { configFilePath = os.Args[3] }
    // fileBytes, err := os.ReadFile(configFilePath)
    // if err != nil { log.Fatalf("Failed to read config file: %v", err) }
    // var mainConfig JsonScadaConfig
    // err = json.Unmarshal(fileBytes, &mainConfig)
    // if err != nil { log.Fatalf("Failed to unmarshal config: %v", err) }
    // mainConfig.DriverInstanceNum = instanceNumFromArgs
    // mainConfig.LogLevelNum = logLevelFromArgs
    ```

*   **Connecting to MongoDB:**
    ```go
    // import "go.mongodb.org/mongo-driver/mongo"
    // import "go.mongodb.org/mongo-driver/mongo/options"
    // import "context"

    // clientOptions := options.Client().ApplyURI(mainConfig.MongoConnectionString)
    // client, err := mongo.Connect(context.TODO(), clientOptions)
    // if err != nil { log.Fatal(err) }
    // err = client.Ping(context.TODO(), nil)
    // if err != nil { log.Fatal(err) }
    // database := client.Database(mainConfig.MongoDatabaseName)
    // instancesCollection := database.Collection("protocolDriverInstances")
    // connectionsCollection := database.Collection("protocolConnections")
    // realtimeDataCollection := database.Collection("realtimeData")
    // commandsQueueCollection := database.Collection("commandsQueue")
    ```

*   **Loading Driver Instance & Connection Configs (Illustrative):**
    ```go
    // Define structs to represent your MongoDB documents
    type ProtocolDriverInstance struct { // Simplified
        ID                      primitive.ObjectID `bson:"_id,omitempty"`
        ProtocolDriver          string             `bson:"protocolDriver"`
        ProtocolDriverInstanceNumber int          `bson:"protocolDriverInstanceNumber"`
        Enabled                 bool               `bson:"enabled"`
        NodeNames               []string           `bson:"nodeNames"`
        ActiveNodeName          string             `bson:"activeNodeName"`
        ActiveNodeKeepAliveTimeTag time.Time       `bson:"activeNodeKeepAliveTimeTag"`
        LogLevel                int                `bson:"logLevel"`
    }

    type MyProtocolConnection struct { // Simplified, add protocol-specific fields
        ID                      primitive.ObjectID `bson:"_id,omitempty"`
        ProtocolDriver          string             `bson:"protocolDriver"`
        ProtocolDriverInstanceNumber int          `bson:"protocolDriverInstanceNumber"`
        ProtocolConnectionNumber int             `bson:"protocolConnectionNumber"`
        Enabled                 bool               `bson:"enabled"`
        CommandsEnabled         bool               `bson:"commandsEnabled"`
        TargetHost              string             `bson:"targetHost"` // Example specific field
        TargetPort              int                `bson:"targetPort"` // Example specific field
    }
    // ...
    // var driverInstance ProtocolDriverInstance
    // instanceFilter := bson.M{
    //     "protocolDriver": "MY_CUSTOM_PROTOCOL",
    //     "protocolDriverInstanceNumber": mainConfig.DriverInstanceNum,
    // }
    // err = instancesCollection.FindOne(context.TODO(), instanceFilter).Decode(&driverInstance)
    // if err != nil { /* log and exit, check for mongo.ErrNoDocuments */ }
    // if !driverInstance.Enabled { /* log and exit */ }

    // var activeConnections []MyProtocolConnection
    // connectionFilter := bson.M{
    //     "protocolDriver": "MY_CUSTOM_PROTOCOL",
    //     "protocolDriverInstanceNumber": mainConfig.DriverInstanceNum,
    //     "enabled": true,
    // }
    // cursor, err := connectionsCollection.Find(context.TODO(), connectionFilter)
    // if err != nil { /* handle error */ }
    // if err = cursor.All(context.TODO(), &activeConnections); err != nil { /* handle error */ }
    ```

*   **Implementing Redundancy Check (using a goroutine):**
    ```go
    // var isActive bool
    // go func() {
    //     ticker := time.NewTicker(10 * time.Second)
    //     defer ticker.Stop()
    //     for {
    //         select {
    //         case <-ticker.C:
    //             // Read driverInstance from MongoDB (as above)
    //             if driverInstance.ActiveNodeName == mainConfig.NodeName {
    //                 isActive = true
    //                 update := bson.M{"$set": bson.M{"activeNodeKeepAliveTimeTag": time.Now()}}
    //                 _, err := instancesCollection.UpdateOne(context.TODO(), instanceFilter, update)
    //                 // Handle error
    //             } else if driverInstance.ActiveNodeName == "" || 
    //                       driverInstance.ActiveNodeKeepAliveTimeTag.Before(time.Now().Add(-60*time.Second)) {
    //                 update := bson.M{"$set": bson.M{
    //                     "activeNodeName": mainConfig.NodeName,
    //                     "activeNodeKeepAliveTimeTag": time.Now(),
    //                 }}
    //                 res, errUpdate := instancesCollection.UpdateOne(context.TODO(), instanceFilter, update)
    //                 // Handle error
    //                 isActive = errUpdate == nil && res.ModifiedCount > 0
    //             } else {
    //                 isActive = false
    //             }
    //         }
    //     }
    // }()
    ```

*   **Updating `realtimeData`:**
    ```go
    // rtFilter := bson.M{
    //     "protocolSourceConnectionNumber": connection.ProtocolConnectionNumber,
    //     "protocolSourceObjectAddress": "tag_address_from_device",
    // }
    // sourceUpdateDoc := bson.M{
    //     "valueAtSource": 123.45,
    //     "valueStringAtSource": "123.45",
    //     "timeTagAtSource": time.Now(), // Or device timestamp
    //     "timeTagAtSourceOk": true,
    //     "invalidAtSource": false,
    //     "timeTag": time.Now(),
    //     "originator": fmt.Sprintf("MY_CUSTOM_PROTOCOL|%d", connection.ProtocolConnectionNumber),
    // }
    // rtUpdate := bson.M{"$set": bson.M{"sourceDataUpdate": sourceUpdateDoc}}
    // _, err := realtimeDataCollection.UpdateOne(context.TODO(), rtFilter, rtUpdate)
    // // Handle error
    ```

*   **Watching `commandsQueue` (Change Stream Example):**
    ```go
    // import "go.mongodb.org/mongo-driver/mongo/options"
    // cmdPipeline := mongo.Pipeline{
    //     {{"$match", bson.D{{"operationType", "insert"}}}},
    //     // Optionally, add more stages to filter by protocolSourceConnectionNumber
    // }
    // cmdChangeStream, err := commandsQueueCollection.Watch(context.TODO(), cmdPipeline)
    // if err != nil { log.Fatal(err) }
    // defer cmdChangeStream.Close(context.TODO())

    // for cmdChangeStream.Next(context.TODO()) {
    //     var changeDoc struct { FullDocument bson.M } // Define a struct matching your command document
    //     if err := cmdChangeStream.Decode(&changeDoc); err != nil { log.Println(err); continue }
    //
    //     commandDoc := changeDoc.FullDocument
    //     cmdConnNumber, ok := commandDoc["protocolSourceConnectionNumber"].(int32) // Or appropriate type
    //     if !ok { continue }
    //     // If cmdConnNumber matches one of this driver's active connections...
    //     // Process command...
    //     // Update commandDoc in commandsQueueCollection with status
    //     // e.g., cmdUpdate := bson.M{"$set": bson.M{"delivered": true, "ackTimeTag": time.Now()}}
    //     // commandsQueueCollection.UpdateOne(context.TODO(), bson.M{"_id": commandDoc["_id"]}, cmdUpdate)
    // }
    ```

**Key Considerations for Go:**

*   **Goroutines and Channels:** Leverage goroutines for concurrent operations (redundancy, individual protocol connections, command monitoring). Use channels for safe communication between goroutines if needed.
*   **Context Management:** Use `context.Context` for managing timeouts and cancellation across API calls, especially for MongoDB operations and network requests.
*   **Error Handling:** Go's explicit error handling (`if err != nil`) should be used diligently.
*   **Structs for Configuration:** Define Go structs with `bson` tags to easily marshal and unmarshal data to/from MongoDB.
*   **Logging:** Use the standard `log` package or a more structured logging library. Ensure log output respects the `logLevel` from configuration.
*   **Dependency Management:** Use Go modules (`go.mod`, `go.sum`).

### Node.js Driver Implementation

Node.js is suitable for I/O-bound protocol drivers due to its asynchronous, event-driven nature.

**Project Structure:**

*   A main project directory.
*   `package.json`: Defines dependencies and scripts.
*   Main `.js` file (e.g., `index.js`, `app.js`).
*   Helper modules in separate `.js` files (e.g., `configLoader.js`, `mongoHelper.js`, `protocolClient.js`). JSON SCADA often uses `load-config.js` and `simple-logger.js` for these purposes.

**Recommended Libraries:**

*   `mongodb`: The official MongoDB Node.js driver.
*   `fs/promises`: For asynchronous file operations (reading `json-scada.json`).
*   Standard library modules like `path`, `events`.

**Example Snippets (Conceptual):**

*   **Reading `json-scada.json` and Main Configuration:**
    ```javascript
    // configLoader.js or main script
    // const fs = require('fs/promises');
    // const path = require('path');

    // async function loadMainConfig(configFilePath, instanceNumFromArgs, logLevelFromArgs) {
    //     const filePath = configFilePath || path.join(__dirname, '..', 'conf', 'json-scada.json');
    //     const jsonString = await fs.readFile(filePath, 'utf8');
    //     const mainConfig = JSON.parse(jsonString);
    //     mainConfig.driverInstanceNum = instanceNumFromArgs;
    //     mainConfig.logLevelNum = logLevelFromArgs;
    //     return mainConfig;
    // }
    ```

*   **Connecting to MongoDB:**
    ```javascript
    // const { MongoClient } = require('mongodb');

    // async function connectMongo(uri, dbName) {
    //     const client = new MongoClient(uri);
    //     await client.connect();
    //     const database = client.db(dbName);
    //     return { client, database };
    // }

    // // In main async function:
    // // const { client: mongoClient, database: mongoDb } = await connectMongo(mainConfig.mongoConnectionString, mainConfig.mongoDatabaseName);
    // // const instancesCollection = mongoDb.collection('protocolDriverInstances');
    // // const connectionsCollection = mongoDb.collection('protocolConnections');
    // // const realtimeDataCollection = mongoDb.collection('realtimeData');
    // // const commandsQueueCollection = mongoDb.collection('commandsQueue');
    // // ...
    // // Remember to mongoClient.close() on shutdown.
    ```

*   **Loading Driver Instance & Connection Configs (Illustrative):**
    ```javascript
    // // Assuming mongoDb is the database object from above
    // const instanceFilter = {
    //     protocolDriver: "MY_CUSTOM_PROTOCOL",
    //     protocolDriverInstanceNumber: mainConfig.driverInstanceNum
    // };
    // const driverInstance = await instancesCollection.findOne(instanceFilter);
    // if (!driverInstance || !driverInstance.enabled) { /* log and process.exit() */ }

    // const connectionFilter = {
    //     protocolDriver: "MY_CUSTOM_PROTOCOL",
    //     protocolDriverInstanceNumber: mainConfig.driverInstanceNum,
    //     enabled: true
    // };
    // const activeConnections = await connectionsCollection.find(connectionFilter).toArray();
    ```

*   **Implementing Redundancy Check (using `setInterval`):**
    ```javascript
    // let isActive = false;
    // setInterval(async () => {
    //     try {
    //         // Re-fetch driverInstance or ensure it's up-to-date
    //         const currentInstanceData = await instancesCollection.findOne(instanceFilter);
    //         if (!currentInstanceData) { isActive = false; return; }

    //         if (currentInstanceData.activeNodeName === mainConfig.nodeName) {
    //             isActive = true;
    //             await instancesCollection.updateOne(instanceFilter, { $set: { activeNodeKeepAliveTimeTag: new Date() } });
    //         } else if (!currentInstanceData.activeNodeName ||
    //                    (currentInstanceData.activeNodeKeepAliveTimeTag.getTime() < Date.now() - 60000)) { // 60s timeout
    //             const updateResult = await instancesCollection.updateOne(instanceFilter, {
    //                 $set: { activeNodeName: mainConfig.nodeName, activeNodeKeepAliveTimeTag: new Date() }
    //             });
    //             isActive = updateResult.modifiedCount > 0;
    //         } else {
    //             isActive = false;
    //         }
    //     } catch (err) {
    //         // Log error
    //         isActive = false;
    //     }
    // }, 10000); // Check every 10 seconds
    ```

*   **Updating `realtimeData`:**
    ```javascript
    // const rtFilter = {
    //     protocolSourceConnectionNumber: connection.protocolConnectionNumber,
    //     protocolSourceObjectAddress: "tag_address_from_device"
    // };
    // const sourceUpdateDoc = {
    //     valueAtSource: 123.45, // Use appropriate BSON types if needed, e.g., new Double(123.45)
    //     valueStringAtSource: "123.45",
    //     timeTagAtSource: new Date(), // Or device timestamp
    //     timeTagAtSourceOk: true,
    //     invalidAtSource: false,
    //     timeTag: new Date(),
    //     originator: `MY_CUSTOM_PROTOCOL|${connection.protocolConnectionNumber}`
    // };
    // await realtimeDataCollection.updateOne(rtFilter, { $set: { sourceDataUpdate: sourceUpdateDoc } });
    ```

*   **Watching `commandsQueue` (Change Stream Example):**
    ```javascript
    // const cmdChangeStream = commandsQueueCollection.watch([
    //     { $match: { operationType: 'insert' } }
    //     // Add further $match stages if needed to filter by protocolSourceConnectionNumber
    // ]);

    // cmdChangeStream.on('change', async (change) => {
    //     const commandDoc = change.fullDocument;
    //     if (commandDoc) {
    //         const cmdConnNumber = commandDoc.protocolSourceConnectionNumber;
    //         // If cmdConnNumber matches one of this driver's active connections...
    //         // Process command...
    //         // Update commandDoc in commandsQueueCollection with status
    //         // await commandsQueueCollection.updateOne({ _id: commandDoc._id }, { $set: { delivered: true, ackTimeTag: new Date() } });
    //     }
    // });
    // cmdChangeStream.on('error', (err) => { /* Log error, potentially restart stream */ });
    ```

**Key Considerations for Node.js:**

*   **Asynchronous Operations:** Embrace `async/await` and Promises for all I/O operations (MongoDB, network communication) to keep the event loop unblocked.
*   **Error Handling:** Use `try...catch` blocks for asynchronous operations and handle errors from MongoDB driver calls and protocol interactions. Attach `.on('error', ...)` handlers to streams.
*   **Event Emitters:** For managing events from protocol libraries or for internal communication, Node.js's `EventEmitter` can be useful.
*   **NPM Packages:** Leverage NPM for MongoDB drivers, protocol-specific libraries, and other utilities.
*   **Logging:** Use `console.log` for basic logging or incorporate a more robust logging library (like Winston or Pino), respecting the configured `logLevel`.
*   **Environment Variables:** Consider using environment variables for sensitive information or settings that might change between deployments, though `json-scada.json` is the primary config source.

## Auto-Tag Creation (Optional Feature)

Some protocols support browsing or discovery of data points available on the external device or server. If your target protocol has this capability, you can implement an "auto-tag creation" feature in your driver. This allows users to quickly integrate a large number of tags without manually configuring each one in JSON SCADA.

**Concept:**

When auto-tag creation is enabled for a specific connection (usually via a boolean flag like `autoCreateTags: true` in its `protocolConnections` document), the driver will:

1.  **Discover Points:** Use the protocol's browsing/discovery mechanism to get a list of available tags/points from the source device. This might include their addresses, data types, and descriptions.
2.  **Check for Existing Tags:** For each discovered point, the driver checks if a tag with the same `protocolSourceConnectionNumber` and `protocolSourceObjectAddress` already exists in the `realtimeData` collection.
3.  **Create New Tags:** If a tag does not exist, the driver creates a new document in the `realtimeData` collection.

**Key Information for New Tag Documents:**

When creating a new tag document, the driver should populate it with as much relevant information as possible:

*   `tag` (String): A unique tag name. This can be derived from the discovered point name or address. It might need sanitization or a prefix/suffix to ensure uniqueness within JSON SCADA. Some systems use the `protocolSourceObjectAddress` itself or a variation of it as the tag name if a human-friendly name isn't readily available.
*   `protocolSourceConnectionNumber` (Double/Int): The `protocolConnectionNumber` of the current connection being processed.
*   `protocolSourceObjectAddress` (String): The address of the point as discovered from the protocol.
*   `protocolSourceASDU` (String): The data type, if discoverable from the protocol.
*   `description` (String): A description of the tag, if available from the source.
*   `type` (String): The JSON SCADA data type (e.g., "analog", "digital", "string", "json"). This should be inferred from the `protocolSourceASDU`.
*   `origin` (String): Typically set to something like "auto-created" or the driver's name.
*   `enabled` (Boolean): Usually `true` by default for auto-created tags.
*   `value`, `valueString`, `timeTag`, `invalid`: Initialize with sensible default values (e.g., 0, empty string, current time, `true` for invalid until first update).
*   **Default Configuration for Acquisition:**
    *   `kconv1`: 1.0
    *   `kconv2`: 0.0
    *   Any protocol-specific fields needed for data acquisition for this tag (e.g., default polling interval, subscription settings) can be taken from `autoCreateTag...` parameters in the `protocolConnections` document (e.g., `autoCreateTagPublishingInterval`, `autoCreateTagSamplingInterval` as seen in the OPC-UA client).

**Implementation Notes:**

*   **Configuration Flag:** The auto-tag creation feature should be controllable per connection via a field in the `protocolConnections` document (e.g., `autoCreateTags: true/false`).
*   **Timing:** Auto-tag creation might run once when the driver establishes a connection or periodically if the set of available points on the source can change dynamically.
*   **Avoiding Duplicates:** Ensure the logic to check for existing tags is robust to prevent creating duplicate entries.
*   **User Feedback:** Log information about discovered and auto-created tags.
*   **Resource Consumption:** Be mindful of creating an excessive number of tags, especially if the source has a very large address space. It might be necessary to allow users to specify paths or filters for auto-tag creation within the `protocolConnections` configuration (e.g., the `topics` array in the OPC-UA client which acts as a browse path filter).

## Building and Running the Driver

Once you have developed your protocol driver, here are the general steps for building and running it within the JSON SCADA environment.

**Building the Driver:**

The build process depends on the language used:

*   **C#:**
    *   Ensure you have the .NET SDK installed.
    *   Navigate to your driver's project directory (containing the `.csproj` file).
    *   Run `dotnet build --configuration Release` (or `Debug` for testing).
    *   The output will typically be in `bin/Release/netX.Y/` or `bin/Debug/netX.Y/`, where `X.Y` is your target .NET version. You'll find your driver's executable (e.g., `MyCustomProtocolDriver.exe` on Windows or `MyCustomProtocolDriver` on Linux) and its dependencies there.

*   **Go:**
    *   Ensure you have the Go toolchain installed.
    *   Navigate to your driver's project directory (containing the `go.mod` file).
    *   Run `go build`. You can use flags to specify the output name and target platform if needed (e.g., `go build -o MyCustomDriver ./cmd/mydriver` or `GOOS=linux GOARCH=amd64 go build -o MyCustomDriver_linux_amd64`).
    *   This will produce a single executable file in the current directory or the specified output path.

*   **Node.js:**
    *   Ensure you have Node.js and npm installed.
    *   Navigate to your driver's project directory (containing `package.json`).
    *   Run `npm install` to install dependencies listed in `package.json`.
    *   Node.js drivers are script-based, so there isn't a separate "build" step that produces an executable in the same way as C# or Go, unless you are using a bundler like Webpack or pkg, which is not typical for JSON SCADA drivers. The source files themselves are run by the Node.js runtime.

**Deployment:**

*   **Location:**
    *   Compiled executables (C#, Go) are often placed in a dedicated directory within the JSON SCADA structure, for example, under a main `bin/` folder.
    *   Node.js drivers can be run directly from their source directories (e.g., `src/MyNodeDriver/`).
*   **Configuration:** Ensure the `json-scada.json` file is accessible to the driver (usually in a `../conf` directory relative to where the driver expects it or specified via command line). Also, ensure the MongoDB collections (`protocolDriverInstances`, `protocolConnections`) are correctly populated for your new driver.

**Running the Driver:**

Drivers are typically run as console applications from a terminal or via a process manager like `supervisord` (common on Linux) or NSSM (on Windows).

The standard command-line arguments are:

1.  **Instance Number (Integer):** The `protocolDriverInstanceNumber` that this process will run as. (e.g., `1`)
2.  **Log Level (Integer):** The desired logging verbosity (e.g., `0` for minimal, `1` for basic, `2` for detailed, `3` for debug). (e.g., `1`)
3.  **Config File Path (String, Optional):** The path to the main `json-scada.json` configuration file. If not provided, drivers often default to `../conf/json-scada.json`. (e.g., `../conf/json-scada.json`)

**Example (Linux):**

```bash
# For a Go or C# (published for Linux) driver
/path/to/json-scada/bin/MyCustomDriver 1 1 ../conf/json-scada.json

# For a Node.js driver
node /path/to/json-scada/src/MyNodeDriver/index.js 1 1 ../conf/json-scada.json
```

**Example (Windows):**

```batch
REM For a C# driver
C:\path	o\json-scadain\MyCustomDriver.exe 1 1 ..\conf\json-scada.json

REM For a Node.js driver
node C:\path	o\json-scada\src\MyNodeDriver\index.js 1 1 ..\conf\json-scada.json
```

**Process Management:**

For production environments, it's highly recommended to run drivers under a process manager:

*   **Linux:** `supervisord` is a common choice. You would create a configuration file for your driver in `/etc/supervisor/conf.d/` specifying the command, user, auto-restart behavior, and log file locations.
*   **Windows:** Tools like NSSM (Non-Sucking Service Manager) can be used to run console applications as Windows services, providing similar auto-restart and management capabilities. JSON SCADA's Windows build scripts often include examples or utilities for creating services.

Refer to the JSON SCADA platform-specific setup guides (e.g., `platform-linux/build.sh`, `platform-windows/create_services.bat`) for examples of how existing drivers are managed.

## Conclusion

Developing a new protocol driver for JSON SCADA involves understanding its MongoDB-centric architecture, implementing the core logic for data acquisition and command handling, and tailoring the implementation to your chosen language (C#, Go, or Node.js). By following the concepts and examples outlined in this guide, you should be well-equipped to extend JSON SCADA's connectivity to new devices and systems.

Remember to:

*   Thoroughly test your driver, including connection stability, data accuracy, command execution, and redundancy behavior.
*   Pay attention to error handling and logging to make troubleshooting easier.
*   Refer to the existing driver implementations in the `src/` directory for practical examples and more detailed insights.

We encourage contributions to the JSON SCADA platform. If you develop a new driver that could benefit the community, please consider sharing it. Good luck!
