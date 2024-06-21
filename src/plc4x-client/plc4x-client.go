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
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	plc4go "github.com/apache/plc4x/plc4go/pkg/api"
	"github.com/apache/plc4x/plc4go/pkg/api/config"
	"github.com/apache/plc4x/plc4go/pkg/api/drivers"
	"github.com/apache/plc4x/plc4go/pkg/api/model"
	"github.com/apache/plc4x/plc4go/pkg/api/transports"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var softwareVersion string = "{json:scada} PLC4X Generic PlC Protocol Driver v.0.1.0 - Copyright 2020-2024 Ricardo L. Olsen"
var driverName string = "PLC4X"
var isActive bool = false

var logLevel = 1

const logLevelMin = 0
const logLevelBasic = 1
const logLevelDetailed = 2
const logLevelDebug = 3

const udpChannelSize = 1000
const udpReadBufferPackets = 100

var pointFilter uint32 = 0

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
	IPAddressLocalBind           string   `json:"ipAddressLocalBind"`
	IPAddresses                  []string `json:"ipAddresses"`
	EndpointURLs                 []string `json:"endpointURLs"`
	Topics                       []string `json:"topics"`
	GiInterval                   float32  `json:"giInterval"`
	PlcConn                      plc4go.PlcConnection
	ReadRequest                  model.PlcReadRequest
}

// check error, terminate app if error
func checkFatalError(err error) {
	if err != nil {
		log.Fatal(err)
	}
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

	client, err = mongo.Connect(ctx, options.Client().ApplyURI(cfg.MongoConnectionString))
	if err != nil {
		return client, collRTD, collInsts, collConns, collCmds, err
	}
	collRTD = client.Database(cfg.MongoDatabaseName).Collection("realtimeData")
	collInsts = client.Database(cfg.MongoDatabaseName).Collection("protocolDriverInstances")
	collConns = client.Database(cfg.MongoDatabaseName).Collection("protocolConnections")
	collCmds = client.Database(cfg.MongoDatabaseName).Collection("commandsQueue")

	return client, collRTD, collInsts, collConns, collCmds, err
}

// cancel a command on commandsQueue collection
func commandCancel(collectionCommands *mongo.Collection, ID primitive.ObjectID, cancelReason string) {
	// write cancel to the command in mongo
	_, err := collectionCommands.UpdateOne(
		context.TODO(),
		bson.M{"_id": bson.M{"$eq": ID}},
		bson.M{"$set": bson.M{"cancelReason": cancelReason}},
	)
	if err != nil {
		log.Println(err)
		log.Println("Mongodb - Can not write update to command on mongo!")
	}
}

// signals a command delvered to protocol on commandsQueue collection
func commandDelivered(collectionCommands *mongo.Collection, ID primitive.ObjectID) {
	// write cancel to the command in mongo
	_, err := collectionCommands.UpdateOne(
		context.TODO(),
		bson.M{"_id": bson.M{"$eq": ID}},
		bson.M{"$set": bson.M{"delivered": true, "ack": true, "ackTimeTag": time.Now()}},
	)
	if err != nil {
		log.Println(err)
		log.Println("Mongodb - Can not write update to command on mongo!")
	}
}

// process commands from change stream, forward commands
func iterateChangeStream(routineCtx context.Context, waitGroup *sync.WaitGroup, stream *mongo.ChangeStream, protConns []*protocolConnection, collectionCommands *mongo.Collection) {
	defer stream.Close(routineCtx)
	defer waitGroup.Done()
	for stream.Next(routineCtx) {
		if !isActive {
			return
		}

		var insDoc insertChange
		if err := stream.Decode(&insDoc); err != nil {
			log.Printf("Commands - %s", err)
			continue
		}

		for _, protCon := range protConns {

			if insDoc.OperationType == "insert" && insDoc.FullDocument.ProtocolSourceConnectionNumber == protCon.ProtocolConnectionNumber {
				log.Printf("Commands - Command received on connection %d, %s %f", insDoc.FullDocument.ProtocolSourceConnectionNumber, insDoc.FullDocument.Tag, insDoc.FullDocument.Value)

				// test for time expired, if too old command (> 10s) then cancel it
				if time.Since(insDoc.FullDocument.TimeTag) > 10*time.Second {
					log.Println("Commands - Command expired ", time.Since(insDoc.FullDocument.TimeTag))
					// write cancel to the command in mongo
					commandCancel(collectionCommands, insDoc.FullDocument.ID, "expired")
					continue
				}

				// All is ok, so send command to I104M UPD

				buf := new(bytes.Buffer)
				var cmdSig uint32 = 0x4b4b4b4b
				err := binary.Write(buf, binary.LittleEndian, cmdSig)
				if err != nil {
					commandCancel(collectionCommands, insDoc.FullDocument.ID, "udp buffer write error")
					log.Println("Commands - binary.Write failed:", err)
					continue
				}
				var addr uint32 = uint32(insDoc.FullDocument.ProtocolSourceObjectAddress)
				err = binary.Write(buf, binary.LittleEndian, addr)
				if err != nil {
					commandCancel(collectionCommands, insDoc.FullDocument.ID, "udp buffer write error")
					log.Println("Commands - binary.Write failed:", err)
					continue
				}
				var tiType uint32 = uint32(insDoc.FullDocument.ProtocolSourceASDU)
				err = binary.Write(buf, binary.LittleEndian, tiType)
				if err != nil {
					commandCancel(collectionCommands, insDoc.FullDocument.ID, "udp buffer write error")
					log.Println("Commands - binary.Write failed:", err)
					continue
				}
				var value uint32 = uint32(insDoc.FullDocument.Value)
				err = binary.Write(buf, binary.LittleEndian, value)
				if err != nil {
					commandCancel(collectionCommands, insDoc.FullDocument.ID, "udp buffer write error")
					log.Println("Commands - binary.Write failed:", err)
					continue
				}
				var sbo uint32 = 0
				if insDoc.FullDocument.ProtocolSourceCommandUseSBO {
					sbo = 1
				}
				err = binary.Write(buf, binary.LittleEndian, sbo)
				if err != nil {
					commandCancel(collectionCommands, insDoc.FullDocument.ID, "udp buffer write error")
					log.Println("Commands - binary.Write failed:", err)
					continue
				}
				var qu uint32 = uint32(insDoc.FullDocument.ProtocolSourceCommandDuration)
				err = binary.Write(buf, binary.LittleEndian, qu)
				if err != nil {
					commandCancel(collectionCommands, insDoc.FullDocument.ID, "udp buffer write error")
					log.Println("Commands - binary.Write failed:", err)
					continue
				}
				var ca uint32 = uint32(insDoc.FullDocument.ProtocolSourceCommonAddress)
				err = binary.Write(buf, binary.LittleEndian, ca)
				if err != nil {
					commandCancel(collectionCommands, insDoc.FullDocument.ID, "udp buffer write error")
					log.Println("Commands - binary.Write failed:", err)
					continue
				}

				errMsg := ""
				ok := false
				for i, ipAddressDest := range protCon.IPAddresses {

					if i >= 2 { // only send to the first 2 addresses
						break
					}

					if strings.TrimSpace(ipAddressDest) == "" {
						errMsg = "no IP destination"
						continue
					}
					//udpAddr, err := net.ResolveUDPAddr("udp", ipAddressDest)
					//if err != nil {
					//	errMsg = "IP address error"
					//	log.Println("Commands - Error on IP: ", err)
					//	continue
					//}
					//_, err = UdpConn.WriteToUDP(buf.Bytes(), udpAddr)
					//if err != nil {
					//	errMsg = "UDP send error"
					//	log.Println("Commands - Error on IP: ", err)
					//	continue
					//}
					// success delivering command
					log.Println("Commands - Command sent to: ", ipAddressDest)
					ok = true
					// log.Println(buf.Bytes())
				}
				if ok {
					commandDelivered(collectionCommands, insDoc.FullDocument.ID)
				} else {
					commandCancel(collectionCommands, insDoc.FullDocument.ID, errMsg)
					log.Println("Commands - Command canceled!")
				}
			}
		}
	}
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
		if !isActive {
			log.Println("Redundancy - ACTIVATING this Node!")
		}
		isActive = true
	} else {
		if isActive { // was active, other node assumed, so be inactive and wait a random time
			log.Println("Redundancy - DEACTIVATING this Node (other node active)!")
			countKeepAliveUpdates = 0
			isActive = false
			time.Sleep(time.Duration(1000) * time.Millisecond)
		}
		isActive = false
		if lastActiveNodeKeepAliveTimeTag == instance.ActiveNodeKeepAliveTimeTag {
			countKeepAliveUpdates++
		}
		lastActiveNodeKeepAliveTimeTag = instance.ActiveNodeKeepAliveTimeTag
		if countKeepAliveUpdates > countKeepAliveUpdatesLimit { // time exceeded, be active
			log.Println("Redundancy - ACTIVATING this Node!")
			isActive = true
		}

	}

	if isActive {
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

func main() {
	log.SetOutput(os.Stdout) // log to standard output
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println(softwareVersion)
	log.Println("Usage plc4x-client [instance number] [log level] [config file name]")

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.WarnLevel)

	// Create a new instance of the PlcDriverManager
	driverManager := plc4go.NewPlcDriverManager(config.WithCustomLogger(zerolog.Logger{}))
	// Register the Transports
	transports.RegisterTcpTransport(driverManager)
	transports.RegisterUdpTransport(driverManager)
	transports.RegisterSerialTransport(driverManager)
	// Register the Drivers
	drivers.RegisterAdsDriver(driverManager)
	drivers.RegisterBacnetDriver(driverManager)
	drivers.RegisterCBusDriver(driverManager)
	drivers.RegisterKnxDriver(driverManager)
	drivers.RegisterModbusTcpDriver(driverManager)
	drivers.RegisterModbusRtuDriver(driverManager)
	drivers.RegisterModbusAsciiDriver(driverManager)
	drivers.RegisterOpcuaDriver(driverManager)
	drivers.RegisterS7Driver(driverManager)

	//driverManager.Discover(func(event model.PlcDiscoveryItem) {
	//	log.Printf("Driver Event: %s", event)
	//}, plc4go.WithDiscoveryOptionRemoteAddress("opcua:tcp://opcua.demo-this.com:51210/UA/SampleServer"))

	var client *mongo.Client
	var err error
	var collectionInstances, collectionConnections, collectionCommands *mongo.Collection
	var csCommands *mongo.ChangeStream
	someConnectionHasCommandsEnabled := false

	instanceNumber := 1
	if os.Getenv("JS_PLC4X_INSTANCE") != "" {
		i, err := strconv.Atoi(os.Getenv("JS_PLC4X_INSTANCE"))
		if err != nil {
			log.Println("JS_PLC4X_INSTANCE environment variable should be a number!")
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

	if os.Getenv("JS_PLC4X_LOGLEVEL") != "" {
		i, err := strconv.Atoi(os.Getenv("JS_I104M_LOGLEVEL"))
		if err != nil {
			log.Println("JS_PLC4X_LOGLEVEL environment variable should be a number!")
			os.Exit(2)
		}
		logLevel = i
	}
	if len(os.Args) > 2 {
		logLevel, err = strconv.Atoi(os.Args[2])
		if err != nil {
			log.Println("Log Level parameter should be a number!")
			os.Exit(2)
		}
	}

	cfgFileName := filepath.Join("..", "conf", "json-scada.json")
	if _, err := os.Stat(cfgFileName); err != nil {
		cfgFileName = filepath.Join("c:\\", "json-scada", "conf", "json-scada.json")
	}
	cfg := configData{}
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
		pointFilter = uint32(ipf)
		log.Printf("Point filter set to: %d", pointFilter)
	}

	_ = json.Unmarshal([]byte(file), &cfg)
	cfg.MongoConnectionString = strings.TrimSpace(cfg.MongoConnectionString)
	cfg.MongoDatabaseName = strings.TrimSpace(cfg.MongoDatabaseName)
	cfg.NodeName = strings.TrimSpace(cfg.NodeName)

	if cfg.MongoConnectionString == "" || cfg.MongoDatabaseName == "" || cfg.NodeName == "" {
		log.Printf("Empty string in config file.")
		os.Exit(1)
	}

	log.Print("Mongodb - Try to connect server...")
	client, _, collectionInstances, collectionConnections, collectionCommands, err = mongoConnect(cfg)
	checkFatalError(err)
	defer client.Disconnect(context.TODO())

	// Check the connection
	err = client.Ping(context.TODO(), nil)
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
		{Key: "protocolDriver", Value: driverName},
		{Key: "protocolDriverInstanceNumber", Value: instanceNumber},
		{Key: "enabled", Value: true},
	}
	err = collectionInstances.FindOne(context.TODO(), filter).Decode(&instance)
	if err != nil || instance.ProtocolDriver == "" {
		log.Fatal("No driver instance found on configuration! Driver Name: ", driverName, " Instance number: ", instanceNumber)
	}

	// read connections config
	filter = bson.D{
		{Key: "protocolDriver", Value: driverName},
		{Key: "protocolDriverInstanceNumber", Value: instanceNumber},
		{Key: "enabled", Value: true},
	}
	var protocolConns []*protocolConnection
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

	for _, protocolConn := range protocolConns {
		if len(protocolConn.EndpointURLs) == 0 {
			log.Printf("No server endpoint for connection %s!", protocolConn.Name)
			continue
		}

		if protocolConn.CommandsEnabled {
			someConnectionHasCommandsEnabled = true
		}

		// log connection info
		log.Printf("Instance:%d Connection:%d %s", protocolConn.ProtocolDriverInstanceNumber, protocolConn.ProtocolConnectionNumber, protocolConn.Name)
		log.Printf("Server endpoint URL: %s", protocolConn.EndpointURLs[0])

		// Get a connection to a remote PLC
		connectionRequestChanel := driverManager.GetConnection(protocolConn.EndpointURLs[0])
		// connectionRequestChanel := driverManager.GetConnection("opcua://opcua.demo-this.com:51210/UA/SampleServer?discovery=true&SecurityPolicy=None")
		// connectionRequestChanel := driverManager.GetConnection("opcua://opcuaserver.com:48010?discovery=false&SecurityPolicy=None")

		// Wait for the driver to connect (or not)
		connectionResult := <-connectionRequestChanel

		// Check if something went wrong
		if connectionResult.GetErr() != nil {
			log.Printf("Error connecting to PLC: %s", connectionResult.GetErr().Error())
			return
		}

		// If all was ok, get the connection instance
		protocolConn.PlcConn = connectionResult.GetConnection()

		// Try to ping the remote device
		pingResultChannel := protocolConn.PlcConn.Ping()

		// Wait for the Ping operation to finish
		pingResult := <-pingResultChannel
		if pingResult.GetErr() != nil {
			log.Printf("Couldn't ping device: %s", pingResult.GetErr().Error())
			return
		}

		// Get the Metadata of the connection
		ca, _ := json.Marshal(protocolConn.PlcConn.GetMetadata().GetConnectionAttributes())
		log.Printf("Connection attributes: %s", ca)
		if !protocolConn.PlcConn.GetMetadata().CanRead() {
			log.Printf("This connection doesn't support read operations")
			return
		}
		if !protocolConn.PlcConn.GetMetadata().CanWrite() {
			log.Printf("This connection doesn't support write operations")
		}
		if !protocolConn.PlcConn.GetMetadata().CanBrowse() {
			log.Printf("This connection doesn't support browsing")
		}
		if !protocolConn.PlcConn.GetMetadata().CanSubscribe() {
			log.Printf("This connection doesn't support subscriptions")
		}

		// Prepare a read-request
		protocolConn.ReadRequest, err = protocolConn.PlcConn.ReadRequestBuilder().
			AddTagAddress("field1", "holding-register:1:INT").
			AddTagAddress("field5", "holding-register:5:INT").
			Build()
		if err != nil {
			log.Printf("error preparing read-request: %s", connectionResult.GetErr().Error())
			return
		}

		execRead := func() {
			log.Println(protocolConn.Name + ": integrity read...")

			// Execute a read-request
			readResponseChanel := protocolConn.ReadRequest.Execute()

			// Wait for the response to finish
			readRequestResult := <-readResponseChanel
			if readRequestResult.GetErr() != nil {
				log.Printf(protocolConn.Name+": error executing read-request: %s", readRequestResult.GetErr().Error())
				return
			}

			// Do something with the response
			value1 := readRequestResult.GetResponse().GetValue("field1")
			log.Printf(protocolConn.Name+": Result field1: %d\n", value1.GetInt16())
			value5 := readRequestResult.GetResponse().GetValue("field5")
			log.Printf(protocolConn.Name+": Result field5: %d\n", value5.GetInt16())
		}
		go func() {
			execRead()
			for range time.Tick(time.Millisecond * 100) {
				execRead()
			}
		}()
	}

	var waitGroup sync.WaitGroup
	if someConnectionHasCommandsEnabled {
		waitGroup.Add(1)
		routineCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go iterateChangeStream(routineCtx, &waitGroup, csCommands, protocolConns, collectionCommands)
	}

	// wait forever
	for {
		time.Sleep(10000)
	}

	// Make sure the connection is closed at the end
	defer protocolConns[0].PlcConn.Close()

}
