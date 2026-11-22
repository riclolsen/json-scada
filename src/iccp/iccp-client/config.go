/*
 * ICCP/TASE.2 Client Driver for JSON-SCADA
 * {json:scada} - Copyright (c) 2020-present - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// configData holds the JSON-SCADA main configuration file contents.
type configData struct {
	NodeName                 string `json:"nodeName"`
	MongoConnectionString    string `json:"mongoConnectionString"`
	MongoDatabaseName        string `json:"mongoDatabaseName"`
	TLSCaPemFile             string `json:"tlsCaPemFile"`
	TLSClientPemFile         string `json:"tlsClientPemFile"`
	TLSClientPfxFile         string `json:"tlsClientPfxFile"`
	TLSClientKeyPassword     string `json:"tlsClientKeyPassword"`
	TLSAllowInvalidHostnames bool   `json:"tlsAllowInvalidHostnames"`
	TLSAllowChainErrors      bool   `json:"tlsAllowChainErrors"`
	TLSInsecure              bool   `json:"tlsInsecure"`
	ICCPMaxTPDUSizeParam     int    `json:"iccpMaxTPDUSizeParam"`
}

// protocolDriverInstance mirrors the protocolDriverInstances collection document.
type protocolDriverInstance struct {
	ID                               bson.ObjectID `bson:"_id"`
	ProtocolDriver                   string        `bson:"protocolDriver"`
	ProtocolDriverInstanceNumber     int           `bson:"protocolDriverInstanceNumber"`
	Enabled                          bool          `bson:"enabled"`
	LogLevel                         int           `bson:"logLevel"`
	NodeNames                        []string      `bson:"nodeNames"`
	ActiveNodeName                   string        `bson:"activeNodeName"`
	ActiveNodeKeepAliveTimeTag       time.Time     `bson:"activeNodeKeepAliveTimeTag"`
	KeepProtocolRunningWhileInactive bool          `bson:"keepProtocolRunningWhileInactive"`
}

// protocolConnection mirrors the protocolConnections collection document.
type protocolConnection struct {
	ID                           bson.ObjectID `bson:"_id"`
	ProtocolDriver               string        `bson:"protocolDriver"`
	ProtocolDriverInstanceNumber int           `bson:"protocolDriverInstanceNumber"`
	ProtocolConnectionNumber     int           `bson:"protocolConnectionNumber"`
	Name                         string        `bson:"name"`
	Description                  string        `bson:"description"`
	Enabled                      bool          `bson:"enabled"`
	CommandsEnabled              bool          `bson:"commandsEnabled"`
	EndpointURLs                 []string      `bson:"endpointURLs"`
	Topics                       []string      `bson:"topics"`
	TimeoutMs                    float64       `bson:"timeoutMs"`
	GiInterval                   float64       `bson:"giInterval"`
	LocalApTitle                 string        `bson:"localApTitle"`
	LocalAeQualifier             int           `bson:"localAeQualifier"`
	RemoteApTitle                string        `bson:"remoteApTitle"`
	RemoteAeQualifier            int           `bson:"remoteAeQualifier"`
	Password                     string        `bson:"password"`
	HoursShift                   float64       `bson:"hoursShift"`
	UseSecurity                  bool          `bson:"useSecurity"`
	LocalCertFilePath            string        `bson:"localCertFilePath"`
	PrivateKeyFilePath           string        `bson:"privateKeyFilePath"`
	PeerCertFilePath             string        `bson:"peerCertFilePath"`
	RootCertFilePath             string        `bson:"rootCertFilePath"`
	ChainValidation              bool          `bson:"chainValidation"`
	AutoAcceptUntrusted          bool          `bson:"autoAcceptUntrustedCertificates"`
	PeerIdentification           *bool         `bson:"peerIdentification"`
	Stats                        storedStats   `bson:"stats"`
}

// storedStats is the part of protocolConnections.stats read back at
// startup: the peer identity the change detection starts from (decoded
// leniently by connSec.seed).
type storedStats struct {
	PeerIdentity bson.Raw `bson:"peerIdentity,omitempty"`
}

// peerIdentificationOn reports whether the server is to be identified on
// every association (peerIdentification, default true).
func (c protocolConnection) peerIdentificationOn() bool {
	return c.PeerIdentification == nil || *c.PeerIdentification
}

// rtData holds a subset of realtimeData fields used by the client driver.
type rtData struct {
	ID                             float64     `bson:"_id"` // realtimeData keys are numbers
	Tag                            string      `bson:"tag"`
	Type                           string      `bson:"type"`
	Origin                         string      `bson:"origin"`
	ProtocolSourceConnectionNumber float64     `bson:"protocolSourceConnectionNumber"`
	ProtocolSourceObjectAddress    interface{} `bson:"protocolSourceObjectAddress"`
	ProtocolSourceASDU             interface{} `bson:"protocolSourceASDU"`
	ProtocolSourceCommonAddress    interface{} `bson:"protocolSourceCommonAddress"`
	ProtocolSourceCommandDuration  float64     `bson:"protocolSourceCommandDuration"`
	ProtocolSourceCommandUseSBO    bool        `bson:"protocolSourceCommandUseSBO"`
	SourceDataUpdate               *sourceData `bson:"sourceDataUpdate"`
}

// sourceData is the part of realtimeData.sourceDataUpdate the client reads
// back at startup, so a tag can be invalidated (keeping its last raw value)
// even when no value has been received since the driver started.
type sourceData struct {
	ValueAtSource       *float64 `bson:"valueAtSource"`
	ValueStringAtSource string   `bson:"valueStringAtSource"`
	ValueJsonAtSource   string   `bson:"valueJsonAtSource"`
	InvalidAtSource     bool     `bson:"invalidAtSource"`
}

// commandQueueEntry mirrors the commandsQueue collection document.
type commandQueueEntry struct {
	ID                             bson.ObjectID `bson:"_id"`
	ProtocolSourceConnectionNumber float64       `bson:"protocolSourceConnectionNumber"`
	ProtocolSourceCommonAddress    interface{}   `bson:"protocolSourceCommonAddress"`
	ProtocolSourceObjectAddress    interface{}   `bson:"protocolSourceObjectAddress"`
	ProtocolSourceASDU             interface{}   `bson:"protocolSourceASDU"`
	ProtocolSourceCommandDuration  float64       `bson:"protocolSourceCommandDuration"`
	ProtocolSourceCommandUseSBO    bool          `bson:"protocolSourceCommandUseSBO"`
	PointKey                       float64       `bson:"pointKey"`
	Tag                            string        `bson:"tag"`
	TimeTag                        time.Time     `bson:"timeTag"`
	Value                          float64       `bson:"value"`
	ValueString                    string        `bson:"valueString"`
	OriginatorUserName             string        `bson:"originatorUserName"`
	OriginatorIPAddress            string        `bson:"originatorIpAddress"`
}

// insertChange wraps a MongoDB change stream insert event.
type insertChange struct {
	FullDocument  commandQueueEntry `bson:"fullDocument"`
	OperationType string            `bson:"operationType"`
}

// dataUpdate holds a single acquired data point ready for MongoDB write.
type dataUpdate struct {
	protocolSourceConnectionNumber float64
	protocolSourceObjectAddress    string
	value                          float64
	valueString                    string
	valueJson                      string
	asdu                           string
	invalid                        bool
	transient                      bool
	notTopical                     bool
	substituted                    bool
	timeTag                        time.Time
	timeTagAtSource                time.Time
	timeTagAtSourceOk              bool
}

// readConfigFile loads the JSON-SCADA configuration file.
func readConfigFile() (cfg configData, instanceNumber int, instLogLevel int) {
	var err error
	instanceNumber = 1
	instLogLevel = LogLevelNormal

	if os.Getenv(EnvPrefix+"INSTANCE") != "" {
		i, e := strconv.Atoi(os.Getenv(EnvPrefix + "INSTANCE"))
		if e != nil {
			log.Fatalf("%sINSTANCE environment variable should be a number!", EnvPrefix)
		}
		instanceNumber = i
	}
	if len(os.Args) > 1 {
		i, e := strconv.Atoi(os.Args[1])
		if e != nil {
			log.Fatalf("Instance parameter should be a number!")
		}
		instanceNumber = i
	}

	if os.Getenv(EnvPrefix+"LOGLEVEL") != "" {
		i, e := strconv.Atoi(os.Getenv(EnvPrefix + "LOGLEVEL"))
		if e != nil {
			log.Fatalf("%sLOGLEVEL environment variable should be a number!", EnvPrefix)
		}
		instLogLevel = i
	}
	if len(os.Args) > 2 {
		instLogLevel, err = strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatalf("Log Level parameter should be a number!")
		}
	}

	cfgFileName := filepath.Join("..", "conf", "json-scada.json")
	if _, err := os.Stat(cfgFileName); err != nil {
		cfgFileName = filepath.Join("~", "json-scada", "conf", "json-scada.json")
	}
	if _, err := os.Stat(cfgFileName); err != nil {
		cfgFileName = filepath.Join("/json-scada", "conf", "json-scada.json")
	}
	if _, err := os.Stat(cfgFileName); err != nil {
		cfgFileName = filepath.Join("c:/json-scada", "conf", "json-scada.json")
	}
	if os.Getenv("JS_CONFIG_FILE") != "" {
		cfgFileName = os.Getenv("JS_CONFIG_FILE")
	}
	if len(os.Args) > 3 {
		cfgFileName = os.Args[3]
	}

	file, err := os.ReadFile(cfgFileName)
	if err != nil {
		log.Fatalf("Failed to read config file %s: %v", cfgFileName, err)
	}

	if err := json.Unmarshal(file, &cfg); err != nil {
		log.Fatalf("Failed to parse config file JSON: %v", err)
	}
	cfg.MongoConnectionString = strings.TrimSpace(cfg.MongoConnectionString)
	cfg.MongoDatabaseName = strings.TrimSpace(cfg.MongoDatabaseName)
	cfg.NodeName = strings.TrimSpace(cfg.NodeName)

	if cfg.MongoConnectionString == "" || cfg.MongoDatabaseName == "" || cfg.NodeName == "" {
		log.Fatalf("Empty string in config file.")
	}

	return
}

// mongoConnect establishes a MongoDB connection.
func mongoConnect(cfg configData) (*mongo.Client, error) {
	connStr := cfg.MongoConnectionString
	if cfg.TLSCaPemFile != "" || cfg.TLSClientPemFile != "" {
		if !strings.Contains(connStr, "tls=true") {
			connStr = connStr + "&tls=true"
		}
	}
	if cfg.TLSCaPemFile != "" {
		connStr = connStr + "&tlsCAFile=" + cfg.TLSCaPemFile
	}
	if cfg.TLSClientPemFile != "" {
		connStr = connStr + "&tlsCertificateKeyFile=" + cfg.TLSClientPemFile
	}
	if cfg.TLSClientKeyPassword != "" {
		connStr = connStr + "&tlsCertificateKeyFilePassword=" + cfg.TLSClientKeyPassword
	}

	client, err := mongo.Connect(options.Client().ApplyURI(connStr))
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// getInstanceConfig reads the protocol driver instance configuration from MongoDB.
func getInstanceConfig(collectionInstances *mongo.Collection, instanceNumber int) (protocolDriverInstance, error) {
	var instance protocolDriverInstance
	filter := bson.D{
		{Key: "protocolDriver", Value: DriverName},
		{Key: "protocolDriverInstanceNumber", Value: instanceNumber},
		{Key: "enabled", Value: true},
	}
	err := collectionInstances.FindOne(context.TODO(), filter).Decode(&instance)
	if err != nil || instance.ProtocolDriver == "" {
		return instance, fmt.Errorf("no driver instance found for %s instance %d", DriverName, instanceNumber)
	}
	return instance, nil
}

// getConnections reads all enabled protocol connections for this driver instance.
func getConnections(collectionConnections *mongo.Collection, instanceNumber int) ([]protocolConnection, error) {
	filter := bson.D{
		{Key: "protocolDriver", Value: DriverName},
		{Key: "protocolDriverInstanceNumber", Value: instanceNumber},
		{Key: "enabled", Value: true},
	}
	cursor, err := collectionConnections.Find(context.TODO(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	var connections []protocolConnection
	for cursor.Next(context.TODO()) {
		var conn protocolConnection
		if err := cursor.Decode(&conn); err != nil {
			return nil, err
		}
		connections = append(connections, conn)
	}

	if len(connections) == 0 {
		return nil, fmt.Errorf("no enabled protocol connections found for %s instance %d", DriverName, instanceNumber)
	}
	return connections, nil
}

// getTagsForConnection reads all realtimeData tags assigned to a connection.
func getTagsForConnection(collectionRtData *mongo.Collection, connNumber int) ([]rtData, error) {
	filter := bson.D{
		{Key: "protocolSourceConnectionNumber", Value: float64(connNumber)},
		{Key: "origin", Value: bson.M{"$ne": "command"}}, // exclude command tags
		{Key: "_id", Value: bson.M{"$gt": 0}},
	}

	projection := bson.D{
		{Key: "_id", Value: 1},
		{Key: "tag", Value: 1},
		{Key: "type", Value: 1},
		{Key: "origin", Value: 1},
		{Key: "protocolSourceConnectionNumber", Value: 1},
		{Key: "protocolSourceObjectAddress", Value: 1},
		{Key: "protocolSourceASDU", Value: 1},
		{Key: "sourceDataUpdate.valueAtSource", Value: 1},
		{Key: "sourceDataUpdate.valueStringAtSource", Value: 1},
		{Key: "sourceDataUpdate.valueJsonAtSource", Value: 1},
		{Key: "sourceDataUpdate.invalidAtSource", Value: 1},
	}

	cursor, err := collectionRtData.Find(context.TODO(), filter, options.Find().SetProjection(projection))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.TODO())

	var tags []rtData
	if err := cursor.All(context.TODO(), &tags); err != nil {
		return nil, err
	}
	return tags, nil
}

func readTuningInt(envName string, configured, defaultValue int) int {
	if raw := strings.TrimSpace(os.Getenv(envName)); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			return v
		}
	}
	if configured > 0 {
		return configured
	}
	return defaultValue
}

// Redundancy state. isActive and keepRunningWhileInactive are read from the
// protocol, report and change-stream goroutines, so they are atomic.
var (
	countKeepAliveUpdates          = 0
	countKeepAliveUpdatesLimit     = 4
	lastActiveNodeKeepAliveTimeTag time.Time
	isActive                       atomic.Bool
	keepRunningWhileInactive       atomic.Bool
)

// protocolRunning reports whether this node keeps ICCP associations up: it is
// the active node, or the instance has keepProtocolRunningWhileInactive set.
func protocolRunning() bool {
	return isActive.Load() || keepRunningWhileInactive.Load()
}

// processRedundancy handles active/passive node switching for high availability.
func processRedundancy(collectionInstances *mongo.Collection, id bson.ObjectID, cfg configData) {
	var instance protocolDriverInstance
	filter := bson.D{{Key: "_id", Value: id}}
	err := collectionInstances.FindOne(context.TODO(), filter).Decode(&instance)
	if err != nil {
		LogMsg(LogLevelMin, "Redundancy - Error querying protocolDriverInstances: %v", err)
		return
	}

	if len(instance.NodeNames) > 0 && !contains(instance.NodeNames, cfg.NodeName) {
		log.Fatalf("Redundancy - This node name not in the list of nodes from driver instance!")
	}
	keepRunningWhileInactive.Store(instance.KeepProtocolRunningWhileInactive)

	if instance.ActiveNodeName == cfg.NodeName {
		if !isActive.Load() {
			LogMsg(LogLevelMin, "Redundancy - ACTIVATING this Node!")
		}
		isActive.Store(true)
	} else {
		if isActive.Load() {
			LogMsg(LogLevelMin, "Redundancy - DEACTIVATING this Node (other node active)!")
			countKeepAliveUpdates = 0
			isActive.Store(false)
			time.Sleep(1 * time.Second)
		}
		if lastActiveNodeKeepAliveTimeTag == instance.ActiveNodeKeepAliveTimeTag {
			countKeepAliveUpdates++
		}
		lastActiveNodeKeepAliveTimeTag = instance.ActiveNodeKeepAliveTimeTag
		if countKeepAliveUpdates > countKeepAliveUpdatesLimit {
			LogMsg(LogLevelMin, "Redundancy - ACTIVATING this Node!")
			isActive.Store(true)
		}
	}

	if isActive.Load() {
		_, err := collectionInstances.UpdateOne(
			context.TODO(),
			bson.M{"_id": bson.M{"$eq": id}},
			bson.M{"$set": bson.M{
				"activeNodeName":             cfg.NodeName,
				"activeNodeKeepAliveTimeTag": bson.NewDateTimeFromTime(time.Now()),
			}},
		)
		if err != nil {
			LogMsg(LogLevelMin, "Redundancy - Error updating keepalive: %v", err)
		}
	}
}

// contains checks if a string slice contains a value.
func contains(a []string, str string) bool {
	tStr := strings.TrimSpace(str)
	for _, n := range a {
		if tStr == strings.TrimSpace(n) {
			return true
		}
	}
	return false
}

// asduToString converts protocolSourceASDU (which can be string or float64 in MongoDB) to a string.
func asduToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case int32:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case int:
		return fmt.Sprintf("%d", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// objAddrToString converts protocolSourceObjectAddress (which can be string or float64 in MongoDB) to a string.
func objAddrToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%v", val)
	case int32:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	case int:
		return fmt.Sprintf("%d", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// commonAddrToFloat64 converts protocolSourceCommonAddress (which can be string or float64 in MongoDB) to float64.
func commonAddrToFloat64(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int32:
		return float64(val)
	case int64:
		return float64(val)
	case int:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
		return 0
	default:
		return 0
	}
}

// cancelCommand marks a command as cancelled in commandsQueue.
func cancelCommand(collectionCommands *mongo.Collection, id bson.ObjectID, reason string) {
	_, err := collectionCommands.UpdateOne(
		context.TODO(),
		bson.M{"_id": bson.M{"$eq": id}},
		bson.M{"$set": bson.M{"cancelReason": reason}},
	)
	if err != nil {
		LogMsg(LogLevelMin, "Commands - Cannot cancel command: %v", err)
	}
}

// deliverCommand marks a command as delivered in commandsQueue.
func deliverCommand(collectionCommands *mongo.Collection, id bson.ObjectID, ok bool, description string) {
	_, err := collectionCommands.UpdateOne(
		context.TODO(),
		bson.M{"_id": bson.M{"$eq": id}},
		bson.M{"$set": bson.M{"delivered": true, "ack": ok, "ackTimeTag": time.Now(), "resultDescription": description}},
	)
	if err != nil {
		LogMsg(LogLevelMin, "Commands - Cannot mark command as delivered: %v", err)
	}
}
