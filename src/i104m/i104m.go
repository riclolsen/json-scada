/*
 * I104M Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020 - Ricardo L. Olsen
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
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var Version string = "{json:scada} I104M Protocol Driver v.0.1.1 - Copyright 2020 Ricardo L. Olsen"
var DriverName string = "I104M"
var IsActive bool = false

var LogLevel = 1

const LogLevelMin = 0
const LogLevelBasic = 1
const LogLevelDetailed = 2
const LogLevelDebug = 3

const UDPChannelSize = 1000
const UDPReadBufferPackets = 100

var PointFilter uint32 = 0

type ConfigData struct {
	NodeName                 string `json: "nodeName"`
	MongoConnectionString    string `json: "mongoConnectionString"`
	MongoDatabaseName        string `json: "mongoDatabaseName"`
	TlsCaPemFile             string `json: "tlsCaPemFile"`
	TlsClientPemFile         string `json: "tlsClientPemFile"`
	TlsClientPfxFile         string `json: "tlsClientPfxFile"`
	TlsClientKeyPassword     string `json: "tlsClientKeyPassword"`
	TlsAllowInvalidHostnames bool   `json: "tlsAllowInvalidHostnames"`
	TlsAllowChainErrors      bool   `json: "tlsAllowChainErrors"`
	TlsInsecure              bool   `json: "tlsInsecure"`
}

type Command struct {
	Id                             primitive.ObjectID `json:"_id" bson:"_id"`
	ProtocolSourceConnectionNumber int                `json: "protocolSourceConnectionNumber"`
	ProtocolSourceCommonAddress    int                `json: "protocolSourceCommonAddress"`
	ProtocolSourceObjectAddress    int                `json: "protocolSourceObjectAddress"`
	ProtocolSourceASDU             int                `json: "protocolSourceASDU"`
	ProtocolSourceCommandDuration  int                `json: "protocolSourceCommandDuration"`
	ProtocolSourceCommandUseSBO    bool               `json: "protocolSourceCommandUseSBO"`
	PointKey                       int                `json: "pointKey"`
	Tag                            string             `json: "tag"`
	TimeTag                        time.Time          `json: "timeTag"`
	Value                          float64            `json: "value"`
	ValueString                    string             `json: "valueString"`
	OriginatorUserName             string             `json: "originatorUserName"`
	OriginatorIpAddress            string             `json: "originatorIpAddress"`
}

type InsertChange struct {
	FullDocument  Command `json: "fullDocument"`
	OperationType string  `json: "operationType"`
}

type ProtocolDriverInstance struct {
	Id                               primitive.ObjectID `json:"_id" bson:"_id"`
	ProtocolDriver                   string             `json: "protocolDriver"`
	ProtocolDriverInstanceNumber     int                `json: "protocolDriverInstanceNumber"`
	Enabled                          bool               `json: "enabled"`
	LogLevel                         int                `json: "logLevel"`
	NodeNames                        []string           `json: "nodeNames"`
	ActiveNodeName                   string             `json: "activeNodeName"`
	ActiveNodeKeepAliveTimeTag       time.Time          `json: "activeNodeKeepAliveTimeTag"`
	KeepProtocolRunningWhileInactive bool               `json: "keepProtocolRunningWhileInactive"`
}

type ProtocolConnection struct {
	ProtocolDriver               string   `json: "protocolDriver"`
	ProtocolDriverInstanceNumber int      `json: "protocolDriverInstanceNumber"`
	ProtocolConnectionNumber     int      `json: "protocolConnectionNumber"`
	Name                         string   `json: "name"`
	Description                  string   `json: "description"`
	Enabled                      bool     `json: "enabled"`
	CommandsEnabled              bool     `json: "commandsEnabled"`
	IpAddressLocalBind           string   `json: "ipAddressLocalBind"`
	IpAddresses                  []string `json: "ipAddresses"`
}

// check error, terminate app if error
func checkFatalError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func mongoConnect(cfg ConfigData) (client *mongo.Client, err error, collRTD *mongo.Collection, collInsts *mongo.Collection, collConns *mongo.Collection, collCmds *mongo.Collection) {

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if cfg.TlsCaPemFile != "" || cfg.TlsClientPemFile != "" {
		cfg.MongoConnectionString = cfg.MongoConnectionString + "&tls=true"
	}
	if cfg.TlsCaPemFile != "" {
		cfg.MongoConnectionString = cfg.MongoConnectionString + "&tlsCAFile=" + cfg.TlsCaPemFile
	}
	if cfg.TlsClientPemFile != "" {
		cfg.MongoConnectionString = cfg.MongoConnectionString + "&tlsCertificateKeyFile=" + cfg.TlsClientPemFile
	}
	if cfg.TlsClientKeyPassword != "" {
		cfg.MongoConnectionString = cfg.MongoConnectionString + "&tlsCertificateKeyFilePassword=" + cfg.TlsClientKeyPassword
	}
	if cfg.TlsInsecure {
		cfg.MongoConnectionString = cfg.MongoConnectionString + "&tlsInsecure=true"
	}
	if cfg.TlsAllowChainErrors {
		cfg.MongoConnectionString = cfg.MongoConnectionString + "&tlsInsecure=true"
	}
	if cfg.TlsAllowInvalidHostnames {
		cfg.MongoConnectionString = cfg.MongoConnectionString + "&tlsAllowInvalidHostnames=true"
	}

	client, err = mongo.NewClient(options.Client().ApplyURI(cfg.MongoConnectionString))
	if err != nil {
		return client, err, collRTD, collInsts, collConns, collCmds
	}

	err = client.Connect(ctx)
	if err != nil {
		return client, err, collRTD, collInsts, collConns, collCmds
	}
	collRTD = client.Database(cfg.MongoDatabaseName).Collection("realtimeData")
	collInsts = client.Database(cfg.MongoDatabaseName).Collection("protocolDriverInstances")
	collConns = client.Database(cfg.MongoDatabaseName).Collection("protocolConnections")
	collCmds = client.Database(cfg.MongoDatabaseName).Collection("commandsQueue")

	return client, err, collRTD, collInsts, collConns, collCmds
}

// Cancel a command on commandsQueue collection
func CommandCancel(collectionCommands *mongo.Collection, Id primitive.ObjectID, cancelReason string) {
	// write cancel to the command in mongo
	_, err := collectionCommands.UpdateOne(
		context.TODO(),
		bson.M{"_id": bson.M{"$eq": Id}},
		bson.M{"$set": bson.M{"cancelReason": cancelReason}},
	)
	if err != nil {
		log.Println(err)
		log.Println("Mongodb - Can not write update to command on mongo!")
	}
}

// Signals a command delvered to protocol on commandsQueue collection
func CommandDelivered(collectionCommands *mongo.Collection, Id primitive.ObjectID) {
	// write cancel to the command in mongo
	_, err := collectionCommands.UpdateOne(
		context.TODO(),
		bson.M{"_id": bson.M{"$eq": Id}},
		bson.M{"$set": bson.M{"delivered": true, "ack": true, "ackTimeTag": time.Now()}},
	)
	if err != nil {
		log.Println(err)
		log.Println("Mongodb - Can not write update to command on mongo!")
	}
}

// process commands from change stream, forward commands via UDP
func iterateChangeStream(routineCtx context.Context, waitGroup *sync.WaitGroup, stream *mongo.ChangeStream, protCon *ProtocolConnection, UdpConn *net.UDPConn, collectionCommands *mongo.Collection) {
	defer stream.Close(routineCtx)
	defer waitGroup.Done()
	for stream.Next(routineCtx) {
		if !IsActive {
			return
		}

		var insDoc InsertChange
		if err := stream.Decode(&insDoc); err != nil {
			log.Printf("Commands - %s", err)
			continue
		}

		if insDoc.OperationType == "insert" && insDoc.FullDocument.ProtocolSourceConnectionNumber == protCon.ProtocolConnectionNumber {
			log.Printf("Commands - Command received on connection %d, %s %f", insDoc.FullDocument.ProtocolSourceConnectionNumber, insDoc.FullDocument.Tag, insDoc.FullDocument.Value)

			// test for time expired, if too old command (> 10s) then cancel it
			if time.Now().Sub(insDoc.FullDocument.TimeTag) > 10*time.Second {
				log.Println("Commands - Command expired ", time.Now().Sub(insDoc.FullDocument.TimeTag))
				// write cancel to the command in mongo
				CommandCancel(collectionCommands, insDoc.FullDocument.Id, "expired")
				continue
			}

			// All is ok, so send command to I104M UPD

			buf := new(bytes.Buffer)
			var cmdSig uint32 = 0x4b4b4b4b
			err := binary.Write(buf, binary.LittleEndian, cmdSig)
			if err != nil {
				CommandCancel(collectionCommands, insDoc.FullDocument.Id, "udp buffer write error")
				log.Println("Commands - binary.Write failed:", err)
				continue
			}
			var addr uint32 = uint32(insDoc.FullDocument.ProtocolSourceObjectAddress)
			err = binary.Write(buf, binary.LittleEndian, addr)
			if err != nil {
				CommandCancel(collectionCommands, insDoc.FullDocument.Id, "udp buffer write error")
				log.Println("Commands - binary.Write failed:", err)
				continue
			}
			var tiType uint32 = uint32(insDoc.FullDocument.ProtocolSourceASDU)
			err = binary.Write(buf, binary.LittleEndian, tiType)
			if err != nil {
				CommandCancel(collectionCommands, insDoc.FullDocument.Id, "udp buffer write error")
				log.Println("Commands - binary.Write failed:", err)
				continue
			}
			var value uint32 = uint32(insDoc.FullDocument.Value)
			err = binary.Write(buf, binary.LittleEndian, value)
			if err != nil {
				CommandCancel(collectionCommands, insDoc.FullDocument.Id, "udp buffer write error")
				log.Println("Commands - binary.Write failed:", err)
				continue
			}
			var sbo uint32 = 0
			if insDoc.FullDocument.ProtocolSourceCommandUseSBO {
				sbo = 1
			}
			err = binary.Write(buf, binary.LittleEndian, sbo)
			if err != nil {
				CommandCancel(collectionCommands, insDoc.FullDocument.Id, "udp buffer write error")
				log.Println("Commands - binary.Write failed:", err)
				continue
			}
			var qu uint32 = uint32(insDoc.FullDocument.ProtocolSourceCommandDuration)
			err = binary.Write(buf, binary.LittleEndian, qu)
			if err != nil {
				CommandCancel(collectionCommands, insDoc.FullDocument.Id, "udp buffer write error")
				log.Println("Commands - binary.Write failed:", err)
				continue
			}
			var ca uint32 = uint32(insDoc.FullDocument.ProtocolSourceCommonAddress)
			err = binary.Write(buf, binary.LittleEndian, ca)
			if err != nil {
				CommandCancel(collectionCommands, insDoc.FullDocument.Id, "udp buffer write error")
				log.Println("Commands - binary.Write failed:", err)
				continue
			}

			errMsg := ""
			ok := false
			for _, ipAddressDest := range protCon.IpAddresses {
				if strings.TrimSpace(ipAddressDest) == "" {
					errMsg = "no IP destination"
					continue
				}
				udpAddr, err := net.ResolveUDPAddr("udp", ipAddressDest)
				if err != nil {
					errMsg = "IP address error"
					log.Println("Commands - Error on IP: ", err)
					continue
				}
				_, err = UdpConn.WriteToUDP(buf.Bytes(), udpAddr)
				if err != nil {
					errMsg = "UDP send error"
					log.Println("Commands - Error on IP: ", err)
					continue
				}
				// success delivering command
				log.Println("Commands - Command sent to: ", ipAddressDest)
				ok = true
				// log.Println(buf.Bytes())
			}
			if ok == true {
				CommandDelivered(collectionCommands, insDoc.FullDocument.Id)
			} else {
				CommandCancel(collectionCommands, insDoc.FullDocument.Id, errMsg)
				log.Println("Commands - Command canceled!")
			}
		}
	}
}

func i104mParseObj(oper *mongo.UpdateOneModel, buf []byte, objAddr uint32, iecAsdu uint32, cause uint32, connectionNumber int) (ok bool) {
	var flags byte
	var value float64
	var f32value float32
	var i16value int16
	var srcTime time.Time
	var srcTimeMSec uint16 // seconds * 1000 + milliseconds
	var srcTimeQualityOk = false
	var hasTime = false
	var invalid = false
	var notTopical = false
	var blocked = false
	var substituted = false
	var overflow = false
	var transient = false
	var carry = false

	ok = false

	switch iecAsdu {
	case 45, 46, 47:
		if LogLevel >= LogLevelDetailed {
			log.Println("Parser - Command ack")
		}
		return false
	case 9, 11, 34, 35:
		ok = true
		flags = buf[2]
		invalid = (flags & 0x80) == 0x80
		notTopical = (flags & 0x40) == 0x40
		substituted = (flags & 0x20) == 0x20
		blocked = (flags & 0x10) == 0x10
		buffer := bytes.NewBuffer(buf[0:])
		binary.Read(buffer, binary.LittleEndian, &i16value)
		value = float64(i16value)
		if LogLevel >= LogLevelDetailed || PointFilter == objAddr {
			log.Printf("Parser - Analogic %d: %d %f %d\n", iecAsdu, objAddr, value, flags)
		}
	case 5, 32:
		ok = true
		flags = buf[1]
		invalid = (flags & 0x80) == 0x80
		notTopical = (flags & 0x40) == 0x40
		substituted = (flags & 0x20) == 0x20
		blocked = (flags & 0x10) == 0x10
		transient = (buf[0] & 0x80) == 0x80
		value = float64(buf[0] & 0x7F)
		if LogLevel >= LogLevelDetailed || PointFilter == objAddr {
			log.Printf("Parser - Analogic %d: %d %f %d\n", iecAsdu, objAddr, value, flags)
		}
	case 13, 36: // float
		ok = true
		flags = buf[4]
		overflow = (flags & 0x01) == 0x01
		invalid = (flags & 0x80) == 0x80
		notTopical = (flags & 0x40) == 0x40
		substituted = (flags & 0x20) == 0x20
		blocked = (flags & 0x10) == 0x10
		buffer := bytes.NewBuffer(buf[0:])
		binary.Read(buffer, binary.LittleEndian, &f32value)
		value = float64(f32value)
		if LogLevel >= LogLevelDetailed || PointFilter == objAddr {
			log.Printf("Parser - Analogic %d: %d %f %d\n", iecAsdu, objAddr, value, flags)
		}
	case 1, 2, 3, 4, 30, 31: // digital
		ok = true
		// convert qualifiers
		flags = buf[0]
		invalid = (flags&0x80) == 0x80 || (flags&0x03) == 0 || (flags&0x03) == 0x03
		invalid = (flags & 0x80) == 0x80
		notTopical = (flags & 0x40) == 0x40
		substituted = (flags & 0x20) == 0x20
		blocked = (flags & 0x10) == 0x10
		if iecAsdu == 3 || iecAsdu == 4 || iecAsdu == 31 { // double
			if flags&0x02 == 0x02 {
				value = 1
			} else {
				value = 0
			}
			if flags&0x03 == 0x00 || flags&0x03 == 0x03 {
				transient = true
			}
		} else { // single
			invalid = (flags & 0x80) == 0x80
			if flags&0x01 == 0x01 {
				value = 1
			} else {
				value = 0
			}
		}
		if LogLevel >= LogLevelDetailed || PointFilter == objAddr {
			log.Printf("Parser - Digital %d: %d %f %d\n", iecAsdu, objAddr, value, flags)
		}
	}
	if iecAsdu == 2 || iecAsdu == 4 || iecAsdu == 30 || iecAsdu == 31 {
		hasTime = true
		srcTimeQualityOk = !(buf[3]&0x80 == 0x80)
		srcTimeMSec = binary.LittleEndian.Uint16(buf[1:])
		srcTime = time.Date(2000+int(buf[7]), time.Month(buf[6]), int(buf[5]), int(buf[4]), int(buf[3]&0x3F), int(srcTimeMSec/1000), 0 /*1000*int(srcTimeMSec%1000)*/, time.Local)
		srcTime = srcTime.Add(time.Duration(srcTimeMSec%1000) * time.Millisecond)
		// log.Printf(">>>> %v\n", srcTime);
	}

	if ok {
		oper.SetFilter(bson.D{
			{"protocolSourceConnectionNumber", connectionNumber},
			{"protocolSourceObjectAddress", objAddr},
		})

		if hasTime {
			oper.SetUpdate(
				bson.D{{"$set",
					bson.D{{"sourceDataUpdate",
						bson.D{
							{"valueAtSource", value},
							{"valueStringAtSource", fmt.Sprintf("%f", value)},
							{"invalidAtSource", invalid},
							{"notTopicalAtSource", notTopical},
							{"substitutedAtSource", substituted},
							{"blockedAtSource", blocked},
							{"overflowAtSource", overflow},
							{"transientAtSource", transient},
							{"carryAtSource", carry},
							{"asduAtSource", fmt.Sprintf("%d", iecAsdu)},
							{"causeOfTransmissionAtSource", cause},
							{"timeTag", time.Now()},
							{"timeTagAtSource", srcTime},
							{"timeTagAtSourceOk", srcTimeQualityOk},
						}},
					}}})
		} else {
			oper.SetUpdate(
				bson.D{{"$set",
					bson.D{{"sourceDataUpdate",
						bson.D{
							{"valueAtSource", value},
							{"valueStringAtSource", fmt.Sprintf("%f", value)},
							{"invalidAtSource", invalid},
							{"notTopicalAtSource", notTopical},
							{"substitutedAtSource", substituted},
							{"blockedAtSource", blocked},
							{"overflowAtSource", overflow},
							{"transientAtSource", transient},
							{"carryAtSource", carry},
							{"asduAtSource", fmt.Sprintf("%d", iecAsdu)},
							{"causeOfTransmissionAtSource", cause},
							{"timeTag", time.Now()},
							{"timeTagAtSourceOk", false},
						}},
					}}})
		}
	}
	return ok // , oksoe, aggrSoePipeline
}

var countKeepAliveUpdates = 0
var countKeepAliveUpdatesLimit = 4
var lastActiveNodeKeepAliveTimeTag time.Time

func processRedundancy(collectionInstances *mongo.Collection, id primitive.ObjectID, cfg ConfigData) {

	var instance ProtocolDriverInstance
	filter := bson.D{{"_id", id}}
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
		if IsActive == false {
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
			if LogLevel >= LogLevelDebug {
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

// find if array contains a IP address ( compare just left part of ":" as in 127.0.0.1:8099 )
func containsIp(a []string, str string) bool {
	tStr := strings.Split(strings.TrimSpace(str), ":")[0]
	for _, n := range a {
		str = strings.Split(n, ":")[0]
		if tStr == strings.TrimSpace(str) {
			return true
		}
	}
	return false
}

// listen for I104M UDP packets, put packets on channel
func listenI104MUdpPackets(con *net.UDPConn, ipAddresses []string, chanBuf chan []byte) {
	cntEnqPkt := 0
	prevBuf := make([]byte, 2048)

	for {
		buf := make([]byte, 2048)

		n, addr, err := con.ReadFromUDP(buf)
		if err != nil {
			log.Printf("UDP - Error: %s \n", err)
			continue
		}

		ipPort := strings.Split(addr.String(), ":")
		if !containsIp(ipAddresses, ipPort[0]) {
			if LogLevel >= LogLevelDebug {
				log.Printf("UDP - Message origin not allowed!\n")
			}
			continue
		}

		if n > 4 {
			if bytes.Equal(buf, prevBuf) {
				if LogLevel >= LogLevelDebug {
					log.Printf("UDP - Duplicated message. Ignored.\n")
				}
				continue
			}
			copy(prevBuf, buf)

			if !IsActive { // do not process packets while inactive
				continue
			}
			select {
			case chanBuf <- buf: // Put buffer in the channel unless it is full
			default:
				log.Println("UDP - Channel is full. Discarding packet!")
				continue
			}
			cntEnqPkt++
			if LogLevel >= LogLevelBasic {
				log.Printf("UDP - Enqueued received packet with %d bytes from %s, #%d", n, ipPort, cntEnqPkt)
			}
		} else {
			if LogLevel >= LogLevelDebug {
				log.Printf("UDP - Invalid small packet. Ignored.\n")
			}
		}
	}
}

func main() {

	log.SetOutput(os.Stdout) // log to standard output

	var client *mongo.Client
	var err error
	var collection, collectionInstances, collectionConnections, collectionCommands *mongo.Collection
	var csCommands *mongo.ChangeStream

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println(Version)
	log.Println("Usage i104m [instance number] [log level] [config file name] [point address filter]")

	instanceNumber := 1
	if len(os.Args) > 1 {
		instanceNumber, _ = strconv.Atoi(os.Args[1])
	}

	if len(os.Args) > 2 {
		LogLevel, _ = strconv.Atoi(os.Args[2])
	}

	cfgFileName := filepath.Join("..", "conf", "json-scada.json")
	cfg := ConfigData{}
	if len(os.Args) > 3 {
		cfgFileName = os.Args[3]
	}
	file, err := ioutil.ReadFile(cfgFileName)
	if err != nil {
		log.Printf("Failed to read file: %v", err)
		os.Exit(1)
	}

	if len(os.Args) > 4 {
		ipf, _ := strconv.Atoi(os.Args[4])
		PointFilter = uint32(ipf)
		log.Printf("Point filter set to: %d", PointFilter)
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
	client, err, collection, collectionInstances, collectionConnections, collectionCommands = mongoConnect(cfg)
	checkFatalError(err)
	defer client.Disconnect(context.TODO())

	// Check the connection
	err = client.Ping(context.TODO(), nil)
	checkFatalError(err)
	log.Print("Mongodb - Connected to server.")

	csCommands, err = collectionCommands.Watch(context.TODO(), mongo.Pipeline{bson.D{
		{
			"$match", bson.D{
				{"operationType", "insert"},
			},
		},
	}})
	checkFatalError(err)
	defer csCommands.Close(context.TODO())

	// read instances config
	var instance ProtocolDriverInstance
	filter := bson.D{{"protocolDriver", DriverName}, {"protocolDriverInstanceNumber", instanceNumber}, {"enabled", true}}
	err = collectionInstances.FindOne(context.TODO(), filter).Decode(&instance)
	if err != nil || instance.ProtocolDriver == "" {
		log.Fatal("No driver instance found on configuration! Driver Name: ", DriverName, " Instance number: ", instanceNumber)
	}

	// read connections config
	// This driver admits only 1 connection per instance!
	var protocolConn ProtocolConnection
	filter = bson.D{{"protocolDriver", DriverName}, {"protocolDriverInstanceNumber", instanceNumber}, {"enabled", true}}
	err = collectionConnections.FindOne(context.TODO(), filter).Decode(&protocolConn)
	checkFatalError(err)
	if LogLevel >= LogLevelDebug {
		log.Println(protocolConn)
	}
	if protocolConn.ProtocolDriver == "" {
		log.Fatal("No connection found!")
	}

	if strings.TrimSpace(protocolConn.IpAddressLocalBind) == "" {
		protocolConn.IpAddressLocalBind = "0.0.0.0:8099"
	}
	if LogLevel >= LogLevelBasic {
		log.Printf("UDP - Binding to address: %s", protocolConn.IpAddressLocalBind)
	}

	if len(protocolConn.IpAddresses) == 0 {
		protocolConn.IpAddresses[0] = "127.0.0.1"
	}

	log.Printf("Instance:%d Connection:%d", protocolConn.ProtocolDriverInstanceNumber, protocolConn.ProtocolConnectionNumber)

	// Lets prepare an server address at any address at port 10001
	ServerAddr, err := net.ResolveUDPAddr("udp", protocolConn.IpAddressLocalBind)
	checkFatalError(err)

	// Now listen at selected port
	ServerConn, err := net.ListenUDP("udp", ServerAddr)
	checkFatalError(err)
	defer ServerConn.Close()
	err = ServerConn.SetReadBuffer(1500 * UDPReadBufferPackets)
	checkFatalError(err)

	buf := make([]byte, 2048)

	tm := time.Now().Add(-6 * time.Second)

	var waitGroup sync.WaitGroup
	if protocolConn.CommandsEnabled == true {
		waitGroup.Add(1)
		routineCtx, _ := context.WithCancel(context.Background())
		go iterateChangeStream(routineCtx, &waitGroup, csCommands, &protocolConn, ServerConn, collectionCommands)
	}

	// listen for UDP packets on a go routine, return packets via a channel (packets as []byte )
	cntDequeuedPackets := 0
	chanBuf := make(chan []byte, UDPChannelSize)
	go listenI104MUdpPackets(ServerConn, protocolConn.IpAddresses, chanBuf)

	for {
		if time.Since(tm) > 5*time.Second {
			if LogLevel >= LogLevelDebug {
				log.Printf("Mongodb - Ping server.\n")
			}
			tm = time.Now()

			for {
				// Check the connection
				err = client.Ping(context.TODO(), nil)
				if err != nil {
					log.Printf("Mongodb - %s \n", err)
					log.Print("Mongodb - Disconnected MongoDB server...")
				}
				if err == nil {
					break
				}
			}

			processRedundancy(collectionInstances, instance.Id, cfg)
		}

		select {
		case buf = <-chanBuf: // receive UDP packets via channel
			break
		case <-time.After(5 * time.Second):
			continue
		}
		cntDequeuedPackets++

		n := len(buf)
		if n > 4 {
			signature := binary.LittleEndian.Uint32(buf[0:])
			if signature == 0x64646464 {

				numpoints := binary.LittleEndian.Uint32(buf[4:])
				iecASDU := binary.LittleEndian.Uint32(buf[8:])
				primaryAddr := binary.LittleEndian.Uint32(buf[12:])
				secondaryAddr := binary.LittleEndian.Uint32(buf[16:])
				cause := binary.LittleEndian.Uint32(buf[20:])
				infoSize := binary.LittleEndian.Uint32(buf[24:])
				incinfo := uint32(0)
				ok := true

				if LogLevel >= LogLevelBasic {
					log.Println("Channel - Received Seqncy ",
						// n, " ",
						// signature, " ",
						numpoints, " ",
						iecASDU, " ",
						primaryAddr, " ",
						secondaryAddr, " ",
						cause, " ",
						infoSize, " #", cntDequeuedPackets)
				}

				switch {
				case iecASDU == 1 || // simples sem tag
					iecASDU == 3: // duplo sem tag
					incinfo = 4 + 1
				case iecASDU == 2 || // simples com tag
					iecASDU == 4: // duplo com tag
					incinfo = 4 + 1 + 3
				case iecASDU == 30 || // simples com tag longa
					iecASDU == 31: // duplo com tag longa
					incinfo = 4 + 1 + 7
				case iecASDU == 5: // reg pos
					incinfo = 4 + 2
				case iecASDU == 32: // reg pos c/ tag
					incinfo = 4 + 2 + 7
				case iecASDU == 9 || // normalized
					iecASDU == 11: // scaled
					incinfo = 4 + 3
				case iecASDU == 34 || // normalized c/ tag
					iecASDU == 35: // scaled c/ tag
					incinfo = 4 + 3 + 7
				case iecASDU == 13: // ponto flutuante
					incinfo = 4 + 5
				case iecASDU == 36: // ponto flutuante c/ tag
					incinfo = 4 + 5 + 7
				case iecASDU == 15:
					incinfo = 4 + 5
				default:
					ok = false
					if LogLevel >= LogLevelDebug {
						log.Printf("Channel - ASDU type [%d] not supported", iecASDU)
					}
					return
				}

				if ok {
					t1 := time.Now()
					var opers []mongo.WriteModel
					// var opersSOE []mongo.WriteModel
					for i := uint32(0); i < numpoints; i++ {
						oper := mongo.NewUpdateOneModel()
						objAddr := binary.LittleEndian.Uint32(buf[28+i*incinfo:])
						okrt := i104mParseObj(oper, buf[32+i*incinfo:], objAddr, iecASDU, cause, protocolConn.ProtocolConnectionNumber)
						if okrt {
							opers = append(opers, oper)
						}
					}
					if len(opers) > 0 {
						res, err := collection.BulkWrite(
							context.Background(),
							opers,
							options.BulkWrite().SetOrdered(false),
						)
						if res == nil {
							log.Print("Mongodb - bulk error!")
							log.Fatal(err)
						}
						t2 := time.Now()

						if LogLevel >= LogLevelDetailed {
							log.Printf("Mongodb - Matched count: %d, Updated Count: %d, Bulk upsert time: %d ms \n", res.MatchedCount, res.ModifiedCount, t2.Sub(t1).Milliseconds())
							if numpoints > 20 {
								log.Printf("Mongodb - %d bulk upserts/s\n", int64(float64(numpoints)/t2.Sub(t1).Seconds()))
							}
						}
					}
				}
			} else if signature == 0x53535353 {

				objAddr := binary.LittleEndian.Uint32(buf[4:])
				iecASDU := binary.LittleEndian.Uint32(buf[8:])
				primaryAddr := binary.LittleEndian.Uint32(buf[12:])
				secondaryAddr := binary.LittleEndian.Uint32(buf[16:])
				cause := binary.LittleEndian.Uint32(buf[20:])
				infoSize := binary.LittleEndian.Uint32(buf[24:])

				if LogLevel >= LogLevelBasic {
					log.Println("Channel - Received Single ",
						// n, " ",
						// signature, " ",
						objAddr, " ",
						iecASDU, " ",
						primaryAddr, " ",
						secondaryAddr, " ",
						cause, " ",
						infoSize, " #", cntDequeuedPackets)
				}

				var opers []mongo.WriteModel
				oper := mongo.NewUpdateOneModel()
				okrt := i104mParseObj(oper, buf[28:], objAddr, iecASDU, cause, protocolConn.ProtocolConnectionNumber)
				if okrt {
					opers = append(opers, oper)
					res, err := collection.BulkWrite(
						context.Background(),
						opers,
					)
					if res == nil {
						log.Print("Mongodb - bulk error!")
						log.Fatal(err)
					}
					log.Println(res)
				}
			} else {
				if LogLevel >= LogLevelBasic {
					log.Println("Channel - Invalid message!")
				}
			}

			if err != nil {
				log.Println("Channel - Error: ", err)
			}
		}
	}
}
