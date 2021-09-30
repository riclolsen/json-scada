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

var softwareVersion string = "{json:scada} I104M Protocol Driver v.0.1.2 - Copyright 2020-2021 Ricardo L. Olsen"
var driverName string = "I104M"
var isActive bool = false

var logLevel = 1

const logLevelMin = 0
const logLevelBasic = 1
const logLevelDetailed = 2
const logLevelDebug = 3

const udpChannelSize = 1000
const udpReadBufferPackets = 100

var udpForwardAddress = "" // assing a forward address for I104M UDP messages
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
}

// check error, terminate app if error
func checkFatalError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

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

	client, err = mongo.NewClient(options.Client().ApplyURI(cfg.MongoConnectionString))
	if err != nil {
		return client, collRTD, collInsts, collConns, collCmds, err
	}

	err = client.Connect(ctx)
	if err != nil {
		return client, collRTD, collInsts, collConns, collCmds, err
	}
	collRTD = client.Database(cfg.MongoDatabaseName).Collection("realtimeData")
	collInsts = client.Database(cfg.MongoDatabaseName).Collection("protocolDriverInstances")
	collConns = client.Database(cfg.MongoDatabaseName).Collection("protocolConnections")
	collCmds = client.Database(cfg.MongoDatabaseName).Collection("commandsQueue")

	return client, collRTD, collInsts, collConns, collCmds, err
}

// Cancel a command on commandsQueue collection
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

// Signals a command delvered to protocol on commandsQueue collection
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

// process commands from change stream, forward commands via UDP
func iterateChangeStream(routineCtx context.Context, waitGroup *sync.WaitGroup, stream *mongo.ChangeStream, protCon *protocolConnection, UdpConn *net.UDPConn, collectionCommands *mongo.Collection) {
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

		if insDoc.OperationType == "insert" && insDoc.FullDocument.ProtocolSourceConnectionNumber == protCon.ProtocolConnectionNumber {
			log.Printf("Commands - Command received on connection %d, %s %f", insDoc.FullDocument.ProtocolSourceConnectionNumber, insDoc.FullDocument.Tag, insDoc.FullDocument.Value)

			// test for time expired, if too old command (> 10s) then cancel it
			if time.Now().Sub(insDoc.FullDocument.TimeTag) > 10*time.Second {
				log.Println("Commands - Command expired ", time.Now().Sub(insDoc.FullDocument.TimeTag))
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
				commandDelivered(collectionCommands, insDoc.FullDocument.ID)
			} else {
				commandCancel(collectionCommands, insDoc.FullDocument.ID, errMsg)
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
		if logLevel >= logLevelDetailed {
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
		if logLevel >= logLevelDetailed || pointFilter == objAddr {
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
		if logLevel >= logLevelDetailed || pointFilter == objAddr {
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
		if logLevel >= logLevelDetailed || pointFilter == objAddr {
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
		if logLevel >= logLevelDetailed || pointFilter == objAddr {
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
			{Key: "protocolSourceConnectionNumber", Value: connectionNumber},
			{Key: "protocolSourceObjectAddress", Value: objAddr},
		})

		if hasTime {
			oper.SetUpdate(
				bson.D{{Key: "$set", Value: bson.D{{Key: "sourceDataUpdate", Value: bson.D{
					{Key: "valueAtSource", Value: value},
					{Key: "valueStringAtSource", Value: fmt.Sprintf("%f", value)},
					{Key: "invalidAtSource", Value: invalid},
					{Key: "notTopicalAtSource", Value: notTopical},
					{Key: "substitutedAtSource", Value: substituted},
					{Key: "blockedAtSource", Value: blocked},
					{Key: "overflowAtSource", Value: overflow},
					{Key: "transientAtSource", Value: transient},
					{Key: "carryAtSource", Value: carry},
					{Key: "asduAtSource", Value: fmt.Sprintf("%d", iecAsdu)},
					{Key: "causeOfTransmissionAtSource", Value: cause},
					{Key: "timeTag", Value: time.Now()},
					{Key: "timeTagAtSource", Value: srcTime},
					{Key: "timeTagAtSourceOk", Value: srcTimeQualityOk},
				}},
				}}})
		} else {
			oper.SetUpdate(
				bson.D{{Key: "$set", Value: bson.D{{Key: "sourceDataUpdate", Value: bson.D{
					{Key: "valueAtSource", Value: value},
					{Key: "valueStringAtSource", Value: fmt.Sprintf("%f", value)},
					{Key: "invalidAtSource", Value: invalid},
					{Key: "notTopicalAtSource", Value: notTopical},
					{Key: "substitutedAtSource", Value: substituted},
					{Key: "blockedAtSource", Value: blocked},
					{Key: "overflowAtSource", Value: overflow},
					{Key: "transientAtSource", Value: transient},
					{Key: "carryAtSource", Value: carry},
					{Key: "asduAtSource", Value: fmt.Sprintf("%d", iecAsdu)},
					{Key: "causeOfTransmissionAtSource", Value: cause},
					{Key: "timeTag", Value: time.Now()},
					{Key: "timeTagAtSourceOk", Value: false},
				}},
				}}})
		}
	}
	return ok // , oksoe, aggrSoePipeline
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

	if len(instance.NodeNames) > 0 && !contains(instance.NodeNames, cfg.NodeName) {
		log.Fatal("Redundancy - This node name not in the list of nodes from driver instance!")
	}

	if instance.ActiveNodeName == cfg.NodeName {
		if isActive == false {
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
		_, err := collectionInstances.UpdateOne(
			context.TODO(),
			bson.M{"_id": bson.M{"$eq": id}},
			bson.M{"$set": bson.M{"activeNodeName": cfg.NodeName, "activeNodeKeepAliveTimeTag": primitive.NewDateTimeFromTime(time.Now())}},
		)
		if err != nil {
			log.Println("Redundancy - error updating mongodb!")
			log.Println(err)
		} else {
			if logLevel >= logLevelDebug {
				log.Println("Redundancy - Update mongodb ok.")
			}
		}
	} else {
		log.Println("Redundancy - This node is not active.")
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
func containsIP(a []string, str string) bool {
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
		if !containsIP(ipAddresses, ipPort[0]) {
			if logLevel >= logLevelDebug {
				log.Printf("UDP - Message origin not allowed!\n")
			}
			continue
		}

		if n > 4 {
			if bytes.Equal(buf, prevBuf) {
				if logLevel >= logLevelDebug {
					log.Printf("UDP - Duplicated message. Ignored.\n")
				}
				continue
			}
			copy(prevBuf, buf)

			if !isActive { // do not process packets while inactive
				continue
			}
			select {
			case chanBuf <- buf: // Put buffer in the channel unless it is full
			default:
				log.Println("UDP - Channel is full. Discarding packet!")
				continue
			}
			cntEnqPkt++
			if logLevel >= logLevelBasic {
				log.Printf("UDP - Enqueued received packet with %d bytes from %s, #%d", n, ipPort, cntEnqPkt)
			}

			// forward message if address set
			if udpForwardAddress != "" {
				udpAddr, err := net.ResolveUDPAddr("udp", udpForwardAddress)
				if err == nil {
					con.WriteToUDP(buf, udpAddr)
				}
			}
		} else {
			if logLevel >= logLevelDebug {
				log.Printf("UDP - Invalid small packet. Ignored.\n")
			}
		}
	}
}

func main() {

	var client *mongo.Client
	var err error
	var collection, collectionInstances, collectionConnections, collectionCommands *mongo.Collection
	var csCommands *mongo.ChangeStream

	log.SetOutput(os.Stdout) // log to standard output
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println(softwareVersion)
	log.Println("Usage i104m [instance number] [log level] [config file name] [point address filter]")

	instanceNumber := 1
	if os.Getenv("JS_I104M_INSTANCE") != "" {
		i, err := strconv.Atoi(os.Getenv("JS_I104M_INSTANCE"))
		if err != nil {
			log.Println("JS_I104M_INSTANCE environment variable should be a number!")
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

	if os.Getenv("JS_I104M_LOGLEVEL") != "" {
		i, err := strconv.Atoi(os.Getenv("JS_I104M_LOGLEVEL"))
		if err != nil {
			log.Println("JS_I104M_LOGLEVEL environment variable should be a number!")
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

	if os.Getenv("JS_I104M_UDP_FORWARD_ADDRESS") != "" {
		udpForwardAddress = os.Getenv("JS_I104M_UDP_FORWARD_ADDRESS")
	}

	cfgFileName := filepath.Join("..", "conf", "json-scada.json")
	cfg := configData{}
	if os.Getenv("JS_CONFIG_FILE") != "" {
		cfgFileName = os.Getenv("JS_CONFIG_FILE")
	}
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
	client, collection, collectionInstances, collectionConnections, collectionCommands, err = mongoConnect(cfg)
	checkFatalError(err)
	defer client.Disconnect(context.TODO())

	// Check the connection
	err = client.Ping(context.TODO(), nil)
	checkFatalError(err)
	log.Print("Mongodb - Connected to server.")

	csCommands, err = collectionCommands.Watch(context.TODO(), mongo.Pipeline{bson.D{
		{
			Key: "$match", Value: bson.D{
				{Key: "operationType", Value: "insert"},
			},
		},
	}})
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
	// This driver admits only 1 connection per instance!
	var protocolConn protocolConnection
	filter = bson.D{
		{Key: "protocolDriver", Value: driverName},
		{Key: "protocolDriverInstanceNumber", Value: instanceNumber},
		{Key: "enabled", Value: true},
	}
	err = collectionConnections.FindOne(context.TODO(), filter).Decode(&protocolConn)
	checkFatalError(err)
	if logLevel >= logLevelDebug {
		log.Println(protocolConn)
	}
	if protocolConn.ProtocolDriver == "" {
		log.Fatal("No connection found!")
	}

	if strings.TrimSpace(protocolConn.IPAddressLocalBind) == "" {
		protocolConn.IPAddressLocalBind = "0.0.0.0:8099"
	}
	if logLevel >= logLevelBasic {
		log.Printf("UDP - Binding to address: %s", protocolConn.IPAddressLocalBind)
	}

	if len(protocolConn.IPAddresses) == 0 {
		protocolConn.IPAddresses = append(protocolConn.IPAddresses, "127.0.0.1:8098")
	}

	log.Printf("Instance:%d Connection:%d", protocolConn.ProtocolDriverInstanceNumber, protocolConn.ProtocolConnectionNumber)

	// Lets prepare an server address at any address at port 10001
	ServerAddr, err := net.ResolveUDPAddr("udp", protocolConn.IPAddressLocalBind)
	checkFatalError(err)

	// Now listen at selected port
	ServerConn, err := net.ListenUDP("udp", ServerAddr)
	checkFatalError(err)
	defer ServerConn.Close()
	err = ServerConn.SetReadBuffer(1500 * udpReadBufferPackets)
	checkFatalError(err)

	buf := make([]byte, 2048)

	tm := time.Now().Add(-6 * time.Second)

	var waitGroup sync.WaitGroup
	if protocolConn.CommandsEnabled == true {
		waitGroup.Add(1)
		routineCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go iterateChangeStream(routineCtx, &waitGroup, csCommands, &protocolConn, ServerConn, collectionCommands)
	}

	// listen for UDP packets on a go routine, return packets via a channel (packets as []byte )
	cntDequeuedPackets := 0
	chanBuf := make(chan []byte, udpChannelSize)
	go listenI104MUdpPackets(ServerConn, protocolConn.IPAddresses, chanBuf)

	for {
		if time.Since(tm) > 5*time.Second {
			if logLevel >= logLevelDebug {
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

			processRedundancy(collectionInstances, instance.ID, cfg)
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

				if logLevel >= logLevelBasic {
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
					if logLevel >= logLevelDebug {
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

						if logLevel >= logLevelDetailed {
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

				if logLevel >= logLevelBasic {
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
				if logLevel >= logLevelBasic {
					log.Println("Channel - Invalid message!")
				}
			}

			if err != nil {
				log.Println("Channel - Error: ", err)
			}
		}
	}
}
