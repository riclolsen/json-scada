/*
 * PLC4X Client - Generic PLC Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020-2024 - Ricardo L. Olsen
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
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	plc4go "github.com/apache/plc4x/plc4go/pkg/api"
	"github.com/apache/plc4x/plc4go/pkg/api/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	logLevelMin      = 0
	logLevelBasic    = 1
	logLevelDetailed = 2
	logLevelDebug    = 3
)

//const udpChannelSize = 1000
//const udpReadBufferPackets = 100

var logLevel = logLevelBasic

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
}

type commandQueueEntry struct {
	ID                             primitive.ObjectID `json:"_id" bson:"_id"`
	ProtocolSourceConnectionNumber int                `json:"protocolSourceConnectionNumber"`
	ProtocolSourceCommonAddress    int                `json:"protocolSourceCommonAddress"`
	ProtocolSourceObjectAddress    int                `json:"protocolSourceObjectAddress"`
	ProtocolSourceASDU             int                `json:"protocolSourceASDU"`
	ProtocolSourceCommandDuration  int                `json:"protocolSourceCommandDuration"`
	ProtocolSourceCommandUseSBO    bool               `json:"protocolSourceCommandUseSBO"`
	PointKey                       int                `json:"pointKey"`
	Tag                            string             `json:"tag"`
	TimeTag                        time.Time          `json:"timeTag"`
	Value                          float64            `json:"value"`
	ValueString                    string             `json:"valueString"`
	OriginatorUserName             string             `json:"originatorUserName"`
	OriginatorIPAddress            string             `json:"originatorIpAddress"`
}

type insertChange struct {
	FullDocument  commandQueueEntry `json:"fullDocument"`
	OperationType string            `json:"operationType"`
}

type protocolDriverInstance struct {
	ID                               primitive.ObjectID `json:"_id" bson:"_id"`
	ProtocolDriver                   string             `json:"protocolDriver"`
	ProtocolDriverInstanceNumber     int                `json:"protocolDriverInstanceNumber"`
	Enabled                          bool               `json:"enabled"`
	LogLevel                         int                `json:"logLevel"`
	NodeNames                        []string           `json:"nodeNames"`
	ActiveNodeName                   string             `json:"activeNodeName"`
	ActiveNodeKeepAliveTimeTag       time.Time          `json:"activeNodeKeepAliveTimeTag"`
	KeepProtocolRunningWhileInactive bool               `json:"keepProtocolRunningWhileInactive"`
}

type protocolConnection struct {
	ProtocolDriver               string   `json:"protocolDriver"`
	ProtocolDriverInstanceNumber int      `json:"protocolDriverInstanceNumber"`
	ProtocolConnectionNumber     int      `json:"protocolConnectionNumber"`
	Name                         string   `json:"name"`
	Description                  string   `json:"description"`
	Enabled                      bool     `json:"enabled"`
	CommandsEnabled              bool     `json:"commandsEnabled"`
	AutoCreateTags               bool     `json:"autoCreateTags"`
	IPAddressLocalBind           string   `json:"ipAddressLocalBind"`
	IPAddresses                  []string `json:"ipAddresses"`
	EndpointURLs                 []string `json:"endpointURLs"`
	Topics                       []string `json:"topics"`
	GiInterval                   float32  `json:"giInterval"`
	PlcConn                      plc4go.PlcConnection
	ReadRequest                  model.PlcReadRequest
	AutoKeyId                    int
}

// check error, terminate app if error
func checkFatalError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func readConfigFile() (cfg configData, instanceNumber int, instLogLevel int) {
	var err error
	instanceNumber = 1
	instLogLevel = logLevelBasic
	if os.Getenv("JS_"+DriverName+"_INSTANCE") != "" {
		i, err := strconv.Atoi(os.Getenv("JS_PLC4X_INSTANCE"))
		if err != nil {
			log.Println("JS_" + DriverName + "_INSTANCE environment variable should be a number!")
			os.Exit(2)
		}
		instanceNumber = i
	}
	if len(os.Args) > 1 {
		i, err := strconv.Atoi(os.Args[1])
		if err != nil {
			log.Println("Instance parameter should be a number!")
			os.Exit(2)
		}
		instanceNumber = i
	}

	if os.Getenv("JS_"+DriverName+"_LOGLEVEL") != "" {
		i, err := strconv.Atoi(os.Getenv("JS_" + DriverName + "_LOGLEVEL"))
		if err != nil {
			log.Println("JS_" + DriverName + "_LOGLEVEL environment variable should be a number!")
			os.Exit(2)
		}
		instLogLevel = i
	}
	if len(os.Args) > 2 {
		instLogLevel, err = strconv.Atoi(os.Args[2])
		if err != nil {
			log.Println("Log Level parameter should be a number!")
			os.Exit(2)
		}
	}

	cfgFileName := filepath.Join("..", "conf", "json-scada.json")
	if _, err := os.Stat(cfgFileName); err != nil {
		cfgFileName = filepath.Join("c:\\", "json-scada", "conf", "json-scada.json")
	}
	if os.Getenv("JS_CONFIG_FILE") != "" {
		cfgFileName = os.Getenv("JS_CONFIG_FILE")
	}
	if len(os.Args) > 3 {
		cfgFileName = os.Args[3]
	}
	file, err := os.ReadFile(cfgFileName)
	if err != nil {
		log.Printf("Failed to read file: %v", err)
		os.Exit(1)
	}

	if len(os.Args) > 4 {
		ipf, _ := strconv.Atoi(os.Args[4])
		PointFilter = uint32(ipf)
		log.Printf("Point filter set to: %d", PointFilter)
	}

	if err := json.Unmarshal([]byte(file), &cfg); err != nil {
		log.Printf("Failed to parse config file JSON: %v", err)
		os.Exit(1)
	}
	cfg.MongoConnectionString = strings.TrimSpace(cfg.MongoConnectionString)
	cfg.MongoDatabaseName = strings.TrimSpace(cfg.MongoDatabaseName)
	cfg.NodeName = strings.TrimSpace(cfg.NodeName)

	if cfg.MongoConnectionString == "" || cfg.MongoDatabaseName == "" || cfg.NodeName == "" {
		log.Printf("Empty string in config file.")
		os.Exit(1)
	}
	return
}

// connects to mongodb database
func mongoConnect(cfg configData) (client *mongo.Client, collRTD *mongo.Collection, collInsts *mongo.Collection, collConns *mongo.Collection, collCmds *mongo.Collection, err error) {

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if cfg.TLSCaPemFile != "" || cfg.TLSClientPemFile != "" {
		cfg.MongoConnectionString = cfg.MongoConnectionString + "&tls=true"
	}
	if cfg.TLSCaPemFile != "" {
		cfg.MongoConnectionString = cfg.MongoConnectionString + "&tlsCAFile=" + cfg.TLSCaPemFile
	}
	if cfg.TLSClientPemFile != "" {
		cfg.MongoConnectionString = cfg.MongoConnectionString + "&tlsCertificateKeyFile=" + cfg.TLSClientPemFile
	}
	if cfg.TLSClientKeyPassword != "" {
		cfg.MongoConnectionString = cfg.MongoConnectionString + "&tlsCertificateKeyFilePassword=" + cfg.TLSClientKeyPassword
	}
	if cfg.TLSInsecure {
		cfg.MongoConnectionString = cfg.MongoConnectionString + "&tlsInsecure=true"
	}
	if cfg.TLSAllowChainErrors {
		cfg.MongoConnectionString = cfg.MongoConnectionString + "&tlsInsecure=true"
	}
	if cfg.TLSAllowInvalidHostnames {
		cfg.MongoConnectionString = cfg.MongoConnectionString + "&tlsAllowInvalidHostnames=true"
	}

	bsonOpts := &options.BSONOptions{
		// UseJSONStructTags:       true,
		// StringifyMapKeysWithFmt: true,
	}
	client, err = mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoConnectionString).SetBSONOptions(bsonOpts))
	if err != nil {
		return client, collRTD, collInsts, collConns, collCmds, err
	}
	collRTD = client.Database(cfg.MongoDatabaseName).Collection("realtimeData")
	collInsts = client.Database(cfg.MongoDatabaseName).Collection("protocolDriverInstances")
	collConns = client.Database(cfg.MongoDatabaseName).Collection("protocolConnections")
	collCmds = client.Database(cfg.MongoDatabaseName).Collection("commandsQueue")

	return client, collRTD, collInsts, collConns, collCmds, err
}

var countKeepAliveUpdates = 0
var countKeepAliveUpdatesLimit = 4
var lastActiveNodeKeepAliveTimeTag time.Time

func processRedundancy(collectionInstances *mongo.Collection, id primitive.ObjectID, cfg configData) {

	var instance protocolDriverInstance
	filter := bson.D{{Key: "_id", Value: id}}
	err := collectionInstances.FindOne(context.TODO(), filter).Decode(&instance)
	if err != nil {
		log.Println("Redundancy - Error querying protocolDriverInstances!")
		log.Println(err)
	}
	if instance.ProtocolDriver == "" {
		log.Println("Redundancy - No driver instance found!")
	}

	if !contains(instance.NodeNames, cfg.NodeName) {
		log.Fatal("Redundancy - This node name not in the list of nodes from driver instance!")
	}

	if instance.ActiveNodeName == cfg.NodeName {
		if !IsActive {
			log.Println("Redundancy - ACTIVATING this Node!")
		}
		IsActive = true
	} else {
		if IsActive { // was active, other node assumed, so be inactive and wait a random time
			log.Println("Redundancy - DEACTIVATING this Node (other node active)!")
			countKeepAliveUpdates = 0
			IsActive = false
			time.Sleep(time.Duration(1000) * time.Millisecond)
		}
		IsActive = false
		if lastActiveNodeKeepAliveTimeTag == instance.ActiveNodeKeepAliveTimeTag {
			countKeepAliveUpdates++
		}
		lastActiveNodeKeepAliveTimeTag = instance.ActiveNodeKeepAliveTimeTag
		if countKeepAliveUpdates > countKeepAliveUpdatesLimit { // time exceeded, be active
			log.Println("Redundancy - ACTIVATING this Node!")
			IsActive = true
		}

	}

	if IsActive {
		log.Println("Redundancy - This node is active.")

		// update keep alive time and node name
		result, err := collectionInstances.UpdateOne(
			context.TODO(),
			bson.M{"_id": bson.M{"$eq": id}},
			bson.M{"$set": bson.M{"activeNodeName": cfg.NodeName, "activeNodeKeepAliveTimeTag": primitive.NewDateTimeFromTime(time.Now())}},
		)
		if err != nil {
			log.Println(err)
		} else {
			if logLevel >= logLevelDebug {
				log.Println("Redundancy - Update result: ", result)
			}
		}
	}
}

// find if array contains a string
func contains(a []string, str string) bool {
	tStr := strings.TrimSpace(str)
	for _, n := range a {
		if tStr == strings.TrimSpace(n) {
			return true
		}
	}
	return false
}

func configInstance(client *mongo.Client, collectionInstances, collectionConnections, collectionCommands *mongo.Collection, instanceNumber int) (protocolConns []*protocolConnection, csCommands *mongo.ChangeStream) {

	// Check the connection
	err := client.Ping(context.TODO(), nil)
	checkFatalError(err)
	log.Print("Mongodb - Connected to server.")

	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)
	csCommands, err = collectionCommands.Watch(context.TODO(), mongo.Pipeline{bson.D{
		{
			Key: "$match", Value: bson.D{
				{Key: "operationType", Value: "insert"},
			},
		},
	}}, opts)
	checkFatalError(err)
	defer csCommands.Close(context.TODO())

	// read instances config
	var instance protocolDriverInstance
	filter := bson.D{
		{Key: "protocolDriver", Value: DriverName},
		{Key: "protocolDriverInstanceNumber", Value: instanceNumber},
		{Key: "enabled", Value: true},
	}
	err = collectionInstances.FindOne(context.TODO(), filter).Decode(&instance)
	if err != nil || instance.ProtocolDriver == "" {
		log.Fatal("No driver instance found on configuration! Driver Name: ", DriverName, " Instance number: ", instanceNumber)
	}

	// read connections config
	filter = bson.D{
		{Key: "protocolDriver", Value: DriverName},
		{Key: "protocolDriverInstanceNumber", Value: instanceNumber},
		{Key: "enabled", Value: true},
	}
	cursor, err := collectionConnections.Find(context.TODO(), filter)
	if err != nil {
		log.Fatal(err)
	}
	defer cursor.Close(context.TODO())
	count := 0
	for cursor.Next(context.TODO()) {
		var protocolConn protocolConnection
		if err = cursor.Decode(&protocolConn); err != nil {
			log.Fatal(err)
		}
		protocolConns = append(protocolConns, &protocolConn)
		count++
	}

	return
}
