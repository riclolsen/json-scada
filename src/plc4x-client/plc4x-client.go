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
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	plc4go "github.com/apache/plc4x/plc4go/pkg/api"
	"github.com/apache/plc4x/plc4go/pkg/api/config"
	"github.com/apache/plc4x/plc4go/pkg/api/drivers"
	"github.com/apache/plc4x/plc4go/pkg/api/model"
	"github.com/apache/plc4x/plc4go/pkg/api/transports"
	"github.com/apache/plc4x/plc4go/pkg/api/values"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	SoftwareVersion string = "{json:scada} PLC4X Generic PlC Protocol Driver v.0.1.0 - Copyright 2020-2024 Ricardo L. Olsen"
	DriverName      string = "PLC4X"
	IsActive        bool   = false
	PointFilter     uint32 = 0
)

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
func iterateCommandsChangeStream(stream *mongo.ChangeStream, protConns []*protocolConnection, collectionCommands *mongo.Collection) {
	defer stream.Close(context.TODO())
	for stream.Next(context.TODO()) {
		//if !IsActive {
		//	continue
		//}

		var insDoc insertChange
		if err := stream.Decode(&insDoc); err != nil {
			log.Printf("Commands - %s", err)
			continue
		}

		for _, protocolConn := range protConns {

			if insDoc.OperationType == "insert" && insDoc.FullDocument.ProtocolSourceConnectionNumber == protocolConn.ProtocolConnectionNumber {
				log.Printf("Commands - Command received on connection %d, %s %f", insDoc.FullDocument.ProtocolSourceConnectionNumber, insDoc.FullDocument.Tag, insDoc.FullDocument.Value)

				// test for time expired, if too old command (> 10s) then cancel it
				if time.Since(insDoc.FullDocument.TimeTag) > 10*time.Second {
					log.Println("Commands - Command expired ", time.Since(insDoc.FullDocument.TimeTag))
					// write cancel to the command in mongo
					commandCancel(collectionCommands, insDoc.FullDocument.ID, "expired")
					break
				}

				// All is ok, so send command
				wrBld := protocolConn.PlcConn.WriteRequestBuilder()
				var plc4xTagName, plc4xAddress string
				plc4xAddress = insDoc.FullDocument.ProtocolSourceObjectAddress
				plc4xTagName = plc4xAddress
				addr := strings.ToUpper(plc4xAddress)
				var wrReq model.PlcWriteRequest
				var err error

				// write based on type of address
				switch {
				case strings.Contains(addr, protocolConn.AddrSeparator+"BOOL"):
					wrReq, err = wrBld.AddTagAddress(plc4xTagName, plc4xAddress, insDoc.FullDocument.Value != 0).Build()
				case strings.Contains(addr, protocolConn.AddrSeparator+"BYTE"):
					wrReq, err = wrBld.AddTagAddress(plc4xTagName, plc4xAddress, byte(insDoc.FullDocument.Value)).Build()
				case strings.Contains(addr, protocolConn.AddrSeparator+"SINT"):
					wrReq, err = wrBld.AddTagAddress(plc4xTagName, plc4xAddress, int8(insDoc.FullDocument.Value)).Build()
				case strings.Contains(addr, protocolConn.AddrSeparator+"SUINT"):
					wrReq, err = wrBld.AddTagAddress(plc4xTagName, plc4xAddress, uint8(insDoc.FullDocument.Value)).Build()
				case strings.Contains(addr, protocolConn.AddrSeparator+"UINT"), strings.Contains(addr, protocolConn.AddrSeparator+"WORD"):
					var vlAux uint16
					switch insDoc.FullDocument.ProtocolSourceASDU {
					case "LITTLE_ENDIAN":
						b2 := make([]byte, 2)
						binary.NativeEndian.PutUint16(b2, uint16(insDoc.FullDocument.Value))
						vlAux = uint16(binary.LittleEndian.Uint16(b2))
					case "BIG_ENDIAN":
						b2 := make([]byte, 2)
						binary.NativeEndian.PutUint16(b2, uint16(insDoc.FullDocument.Value))
						vlAux = uint16(binary.BigEndian.Uint16(b2))
					default:
						vlAux = uint16(insDoc.FullDocument.Value)
					}
					wrReq, err = wrBld.AddTagAddress(plc4xTagName, plc4xAddress, vlAux).Build()
				case strings.Contains(addr, protocolConn.AddrSeparator+"INT"):
					var vlAux int16
					switch insDoc.FullDocument.ProtocolSourceASDU {
					case "LITTLE_ENDIAN":
						b2 := make([]byte, 2)
						binary.NativeEndian.PutUint16(b2, uint16(insDoc.FullDocument.Value))
						vlAux = int16(binary.LittleEndian.Uint16(b2))
					case "BIG_ENDIAN":
						b2 := make([]byte, 2)
						binary.NativeEndian.PutUint16(b2, uint16(insDoc.FullDocument.Value))
						vlAux = int16(binary.BigEndian.Uint16(b2))
					case "REV_ENDIAN":
						b2 := make([]byte, 2)
						binary.LittleEndian.PutUint16(b2, uint16(insDoc.FullDocument.Value))
						vlAux = int16(binary.BigEndian.Uint16(b2))
					default:
						vlAux = int16(insDoc.FullDocument.Value)
					}
					wrReq, err = wrBld.AddTagAddress(plc4xTagName, plc4xAddress, vlAux).Build()
				case strings.Contains(addr, protocolConn.AddrSeparator+"UDINT"):
					var vlAux uint32
					switch insDoc.FullDocument.ProtocolSourceASDU {
					case "LITTLE_ENDIAN":
						b4 := make([]byte, 4)
						binary.NativeEndian.PutUint32(b4, uint32(insDoc.FullDocument.Value))
						vlAux = uint32(binary.LittleEndian.Uint32(b4))
					case "BIG_ENDIAN":
						b4 := make([]byte, 4)
						binary.NativeEndian.PutUint32(b4, uint32(insDoc.FullDocument.Value))
						vlAux = uint32(binary.BigEndian.Uint32(b4))
					case "REV_ENDIAN":
						b4 := make([]byte, 4)
						binary.LittleEndian.PutUint32(b4, uint32(insDoc.FullDocument.Value))
						vlAux = uint32(binary.BigEndian.Uint32(b4))
					default:
						vlAux = uint32(insDoc.FullDocument.Value)
					}
					wrReq, err = wrBld.AddTagAddress(plc4xTagName, plc4xAddress, vlAux).Build()
				case strings.Contains(addr, protocolConn.AddrSeparator+"DINT"):
					var vlAux int32
					switch insDoc.FullDocument.ProtocolSourceASDU {
					case "LITTLE_ENDIAN":
						b4 := make([]byte, 4)
						binary.NativeEndian.PutUint32(b4, uint32(insDoc.FullDocument.Value))
						vlAux = int32(binary.LittleEndian.Uint32(b4))
					case "BIG_ENDIAN":
						b4 := make([]byte, 4)
						binary.NativeEndian.PutUint32(b4, uint32(insDoc.FullDocument.Value))
						vlAux = int32(binary.BigEndian.Uint32(b4))
					case "REV_ENDIAN":
						b4 := make([]byte, 4)
						binary.LittleEndian.PutUint32(b4, uint32(insDoc.FullDocument.Value))
						vlAux = int32(binary.BigEndian.Uint32(b4))
					default:
						vlAux = int32(insDoc.FullDocument.Value)
					}
					wrReq, err = wrBld.AddTagAddress(plc4xTagName, plc4xAddress, vlAux).Build()
				case strings.Contains(addr, protocolConn.AddrSeparator+"ULINT"), strings.Contains(addr, protocolConn.AddrSeparator+"LWORD"):
					var vlAux uint64
					switch insDoc.FullDocument.ProtocolSourceASDU {
					case "LITTLE_ENDIAN":
						b8 := make([]byte, 8)
						binary.NativeEndian.PutUint64(b8, uint64(insDoc.FullDocument.Value))
						vlAux = uint64(binary.LittleEndian.Uint64(b8))
					case "BIG_ENDIAN":
						b8 := make([]byte, 8)
						binary.NativeEndian.PutUint64(b8, uint64(insDoc.FullDocument.Value))
						vlAux = uint64(binary.BigEndian.Uint64(b8))
					case "REV_ENDIAN":
						b8 := make([]byte, 8)
						binary.LittleEndian.PutUint64(b8, uint64(insDoc.FullDocument.Value))
						vlAux = uint64(binary.BigEndian.Uint32(b8))
					default:
						vlAux = uint64(insDoc.FullDocument.Value)
					}
					wrReq, err = wrBld.AddTagAddress(plc4xTagName, plc4xAddress, vlAux).Build()
				case strings.Contains(addr, protocolConn.AddrSeparator+"LINT"):
					var vlAux int64
					switch insDoc.FullDocument.ProtocolSourceASDU {
					case "LITTLE_ENDIAN":
						b8 := make([]byte, 8)
						binary.NativeEndian.PutUint64(b8, uint64(insDoc.FullDocument.Value))
						vlAux = int64(binary.LittleEndian.Uint64(b8))
					case "BIG_ENDIAN":
						b8 := make([]byte, 8)
						binary.NativeEndian.PutUint64(b8, uint64(insDoc.FullDocument.Value))
						vlAux = int64(binary.BigEndian.Uint64(b8))
					case "REV_ENDIAN":
						b8 := make([]byte, 8)
						binary.LittleEndian.PutUint64(b8, uint64(insDoc.FullDocument.Value))
						vlAux = int64(binary.BigEndian.Uint32(b8))
					default:
						vlAux = int64(insDoc.FullDocument.Value)
					}
					wrReq, err = wrBld.AddTagAddress(plc4xTagName, plc4xAddress, vlAux).Build()
				case strings.Contains(addr, protocolConn.AddrSeparator+"REAL"):
					var vlAux float32
					switch insDoc.FullDocument.ProtocolSourceASDU {
					case "LITTLE_ENDIAN":
						b4 := make([]byte, 4)
						binary.NativeEndian.PutUint32(b4, math.Float32bits(float32(insDoc.FullDocument.Value)))
						vlAux = math.Float32frombits(binary.LittleEndian.Uint32(b4))
					case "BIG_ENDIAN":
						b4 := make([]byte, 4)
						binary.NativeEndian.PutUint32(b4, math.Float32bits(float32(insDoc.FullDocument.Value)))
						vlAux = math.Float32frombits(binary.BigEndian.Uint32(b4))
					case "REV_ENDIAN":
						b4 := make([]byte, 4)
						binary.LittleEndian.PutUint32(b4, math.Float32bits(float32(insDoc.FullDocument.Value)))
						vlAux = math.Float32frombits(binary.BigEndian.Uint32(b4))
					default:
						vlAux = float32(insDoc.FullDocument.Value)
					}
					wrReq, err = wrBld.AddTagAddress(plc4xTagName, plc4xAddress, vlAux).Build()
				case strings.Contains(addr, protocolConn.AddrSeparator+"LREAL"):
					var vlAux float64
					switch insDoc.FullDocument.ProtocolSourceASDU {
					case "LITTLE_ENDIAN":
						b8 := make([]byte, 8)
						binary.NativeEndian.PutUint64(b8, math.Float64bits(float64(insDoc.FullDocument.Value)))
						vlAux = math.Float64frombits(binary.LittleEndian.Uint64(b8))
					case "BIG_ENDIAN":
						b8 := make([]byte, 8)
						binary.NativeEndian.PutUint64(b8, math.Float64bits(float64(insDoc.FullDocument.Value)))
						vlAux = math.Float64frombits(binary.BigEndian.Uint64(b8))
					case "REV_ENDIAN":
						b8 := make([]byte, 8)
						binary.LittleEndian.PutUint64(b8, math.Float64bits(float64(insDoc.FullDocument.Value)))
						vlAux = math.Float64frombits(binary.BigEndian.Uint64(b8))
					default:
						vlAux = float64(insDoc.FullDocument.Value)
					}
					wrReq, err = wrBld.AddTagAddress(plc4xTagName, plc4xAddress, vlAux).Build()
				case strings.Contains(addr, protocolConn.AddrSeparator+"STRING"),
					strings.Contains(addr, protocolConn.AddrSeparator+"CHAR"),
					strings.Contains(addr, protocolConn.AddrSeparator+"WCHAR"):
					wrReq, err = wrBld.AddTagAddress(plc4xTagName, plc4xAddress, insDoc.FullDocument.ValueString).Build()
				case strings.Contains(addr, protocolConn.AddrSeparator+"Struct"),
					strings.Contains(addr, protocolConn.AddrSeparator+"List"),
					strings.Contains(addr, protocolConn.AddrSeparator+"RAW_BYTE_ARRAY"):
					wrReq, err = wrBld.AddTagAddress(plc4xTagName, plc4xAddress, nil).Build()
				default:
					wrReq, err = wrBld.AddTagAddress(plc4xTagName, plc4xAddress, insDoc.FullDocument.Value).Build()
				}
				if err != nil {
					commandCancel(collectionCommands, insDoc.FullDocument.ID, err.Error())
					log.Println("Commands - Command canceled!", insDoc.FullDocument.ID, err)
					break
				}
				ch := wrReq.Execute()
				wrReqResult := <-ch
				if wrReqResult.GetErr() != nil {
					commandCancel(collectionCommands, insDoc.FullDocument.ID, wrReqResult.GetErr().Error())
					log.Println("Commands - Command error executing!", insDoc.FullDocument.ID, err)
					break
				}
				log.Println("Commands - Command executed successfully!", insDoc.FullDocument.ID)
				commandDelivered(collectionCommands, insDoc.FullDocument.ID)
				break
			}
		}
	}
	log.Println("Commands - Exit change stream monitoring!")
}

func main() {
	log.SetOutput(os.Stdout) // log to standard output
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println(SoftwareVersion)
	log.Println("Usage plc4x-client [instance number] [log level] [config file name]")

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

	cfg, instanceNumber, ll := readConfigFile()
	logLevel := ll

	//log.Print("Mongodb - Try to connect server...")
	//client, collectionRtData, collectionInstances, collectionConnections, collectionCommands, err := mongoConnect(cfg)
	//checkFatalError(err)
	//defer client.Disconnect(context.TODO())
	//protocolConns, _ := configInstance(client, collectionInstances, collectionConnections, collectionCommands, instanceNumber)

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if logLevel >= logLevelDebug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else if logLevel >= logLevelDetailed {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	}
	log.Println("Log level set to:", logLevel)

	chMongoBw := make(chan []mongo.WriteModel, 1000)
	chProtConns := make(chan []*protocolConnection)
	chInstancesColl := make(chan *mongo.Collection)
	chRtDataColl := make(chan *mongo.Collection)
	chInstanceId := make(chan primitive.ObjectID)

	// start a go routine to handle commands from mongo
	go mongoWriter(cfg, instanceNumber, chMongoBw, chProtConns, chRtDataColl, chInstancesColl, chInstanceId, logLevel)
	var protocolConns []*protocolConnection = []*protocolConnection{}
	var collectionRtData *mongo.Collection
	var collectionInstances *mongo.Collection
	var instanceId primitive.ObjectID

	// keep retrying protocol reconnection when disconnected
	for {
		select {
		case protocolConns = <-chProtConns:
		case collectionRtData = <-chRtDataColl:
		case instanceId = <-chInstanceId:
		case collectionInstances = <-chInstancesColl:
		default:
		}
		if len(protocolConns) == 0 || collectionRtData == nil || collectionInstances == nil {
			time.Sleep(1 * time.Second)
			IsActive = false
			continue
		}
		processRedundancy(collectionInstances, instanceId, cfg)
		if !IsActive {
			time.Sleep(1 * time.Second)
			continue
		}
		for _, protocolConn := range protocolConns {
			if protocolConn.PlcConn != nil && protocolConn.PlcConn.IsConnected() {
				continue
			}

			if protocolConn.PlcConn != nil {
				protocolConn.PlcConn.Close()
				protocolConn.PlcConn = nil
			}

			protocolConn.GetAutoKeyInitialValueConn(collectionRtData, protocolConn.ProtocolConnectionNumber)

			if len(protocolConn.EndpointURLs) == 0 {
				log.Fatal("No server endpoint for connection: ", protocolConn.Name)
			}

			// log connection info
			connUrl := protocolConn.EndpointURLs[protocolConn.ReconnectCount%len(protocolConn.EndpointURLs)]
			protocolConn.ReconnectCount++
			log.Printf("Instance: %d Connection: %d %s", protocolConn.ProtocolDriverInstanceNumber, protocolConn.ProtocolConnectionNumber, protocolConn.Name)
			log.Printf("%s: Server endpoint URL: %s", protocolConn.Name, connUrl)
			protocolId := strings.Split(protocolConn.EndpointURLs[0], ":")[0]
			addrSep := ":"
			addrParts := 1
			switch protocolId {
			case "ab-eth":
			case "ads":
			case "bacnet-ip":
			case "c-bus":
			case "df1":
			case "eip":
				addrSep = ":"
				addrParts = 3
			case "firmata":
				addrSep = ":"
				addrParts = 3
			case "knxnet-ip":
				addrSep = "/"
				addrParts = 3
			case "modbus", "modbus-tcp", "modbus-rtu", "modbus-ascii":
				addrSep = ":"
				addrParts = 3
			case "opcua":
				addrSep = ";"
				addrParts = 3
			case "s7":
				addrSep = ":"
				addrParts = 3
			case "simulated":
			default:
				log.Fatal(protocolConn.Name + ": Unsupported protocol - " + protocolId)
				continue
			}
			protocolConn.AddrSeparator, _ = addrSep, addrParts

			// try to connect to plc
			connectionRequestChanel := driverManager.GetConnection(connUrl)
			var connectionResult plc4go.PlcConnectionConnectResult
			for {
				select {
				case connectionResult = <-connectionRequestChanel:
				default:
					processRedundancy(collectionInstances, instanceId, cfg)
					time.Sleep(2 * time.Second)
					continue
				}
				break
			}
			// connectionResult := <-connectionRequestChanel
			if connectionResult.GetErr() != nil {
				log.Printf("%s: Error connecting to PLC: %s", protocolConn.Name, connectionResult.GetErr().Error())
				protocolConn.PlcConn = nil
				continue
			}

			// get and store connection instance
			protocolConn.PlcConn = connectionResult.GetConnection()

			// try to ping the plc
			pingResultChannel := protocolConn.PlcConn.Ping()
			pingResult := <-pingResultChannel
			if pingResult.GetErr() != nil {
				log.Printf("%s: Couldn't ping device: %s", protocolConn.Name, pingResult.GetErr().Error())
				protocolConn.PlcConn.Close()
				protocolConn.PlcConn = nil
				continue
			}

			// log metadata of connection
			ca, _ := json.Marshal(protocolConn.PlcConn.GetMetadata().GetConnectionAttributes())
			log.Printf("%s: Connection attributes: %s", protocolConn.Name, ca)
			if !protocolConn.PlcConn.GetMetadata().CanRead() {
				log.Println(protocolConn.Name + ": This connection doesn't support read operations")
				protocolConn.PlcConn.Close()
				protocolConn.PlcConn = nil
				continue
			}
			if !protocolConn.PlcConn.GetMetadata().CanWrite() {
				log.Println(protocolConn.Name + ": This connection doesn't support write operations")
			}
			if !protocolConn.PlcConn.GetMetadata().CanBrowse() {
				log.Println(protocolConn.Name + ": This connection doesn't support browsing")
			}
			if !protocolConn.PlcConn.GetMetadata().CanSubscribe() {
				log.Println(protocolConn.Name + ": This connection doesn't support subscriptions")
			}

			// build a read-request
			reqBld := protocolConn.PlcConn.ReadRequestBuilder()
			for _, topic := range protocolConn.Topics {
				var plc4xTagName, plc4xAddress, jsTagName, endianness string
				splNameAddr := strings.Split(topic, "|")
				if len(splNameAddr) > 2 {
					jsTagName = splNameAddr[0]
					plc4xAddress = splNameAddr[1]
					plc4xTagName = plc4xAddress
					endianness = strings.ToUpper(splNameAddr[2])
				} else if len(splNameAddr) > 1 {
					jsTagName = splNameAddr[0]
					plc4xAddress = splNameAddr[1]
					plc4xTagName = plc4xAddress
				} else {
					plc4xAddress = topic
				}
				protocolConn.Endianness = append(protocolConn.Endianness, endianness)
				plc4xTagName = plc4xAddress
				typeJsTag := "analog"
				addr := strings.ToUpper(plc4xAddress)
				switch {
				case strings.Contains(addr, addrSep+"BOOL"):
					typeJsTag = "digital"
				case strings.Contains(addr, addrSep+"STRING"), strings.Contains(addr, addrSep+"CHAR"):
					typeJsTag = "string"
				case strings.Contains(addr, addrSep+"Struct"), strings.Contains(addr, addrSep+"LIST"), strings.Contains(addr, addrSep+"RAW_BYTE_ARRAY"):
					typeJsTag = "json"
				}
				reqBld.AddTagAddress(plc4xTagName, plc4xAddress)

				if protocolConn.AutoCreateTags {
					// extract number between [ ] from plc4xAddress
					sNum := regexp.MustCompile(`\[.*?\]`).FindString(plc4xAddress)
					numElemArr := 0
					if sNum != "" {
						sNum = sNum[1 : len(sNum)-1]
						var err error
						numElemArr, err = strconv.Atoi(sNum)
						if err != nil {
							log.Println(protocolConn.Name + ": error parsing array number from address: " + plc4xAddress)
							continue
						}
					}

					if numElemArr > 1 {
						for i := 0; i < numElemArr; i++ {
							rtd := NewRtDataTag()
							rtd.Tag = jsTagName + "[" + fmt.Sprint(i) + "]"
							rtd.ProtocolSourceConnectionNumber = float64(protocolConn.ProtocolConnectionNumber)
							rtd.ProtocolSourceObjectAddress = plc4xAddress + "[" + fmt.Sprint(i) + "]"
							rtd.ProtocolSourceASDU = endianness
							rtd.Group1 = DriverName
							rtd.Group2 = protocolConn.Name
							rtd.Group3 = plc4xAddress
							rtd.Type = typeJsTag
							protocolConn.AutoCreateTag(&rtd, collectionRtData)
							if logLevel >= logLevelBasic {
								log.Printf(protocolConn.Name+": tagName: %s address: %s", rtd.Tag, plc4xAddress+"["+fmt.Sprint(i)+"]")
							}
						}
					} else {
						rtd := NewRtDataTag()
						rtd.Tag = jsTagName
						rtd.ProtocolSourceConnectionNumber = float64(protocolConn.ProtocolConnectionNumber)
						rtd.ProtocolSourceObjectAddress = plc4xAddress
						rtd.ProtocolSourceASDU = endianness
						rtd.Group1 = DriverName
						rtd.Group2 = protocolConn.Name
						rtd.Group3 = plc4xAddress
						rtd.Type = typeJsTag
						protocolConn.AutoCreateTag(&rtd, collectionRtData)
						if logLevel >= logLevelBasic {
							log.Printf(protocolConn.Name+": tagName: %s address: %s", rtd.Tag, plc4xAddress)
						}
					}
				} else {
					if logLevel >= logLevelBasic {
						log.Printf(protocolConn.Name+": address: %s", plc4xAddress)
					}
				}
			}
			var err error
			protocolConn.ReadRequest, err = reqBld.Build()
			if err != nil {
				log.Printf(protocolConn.Name + ": error preparing read-request: %s")
				log.Fatal(err)
			}

			execRead := func() error {
				if logLevel >= logLevelBasic {
					log.Println(protocolConn.Name + ": integrity read...")
				}

				// Execute a read-request
				readResponseChanel := protocolConn.ReadRequest.Execute()

				// Wait for the response to finish
				readRequestResult := <-readResponseChanel
				if readRequestResult.GetErr() != nil {
					log.Printf(protocolConn.Name+": error executing read-request: %s", readRequestResult.GetErr().Error())
					return readRequestResult.GetErr()
				}

				// Do something with the response
				var updOpers []mongo.WriteModel
				for i, plc4xTagName := range readRequestResult.GetResponse().GetTagNames() {
					v := readRequestResult.GetResponse().GetValue(plc4xTagName)
					valDbl, valStr, valJson, valArrDbl, bad := extractValue(v, protocolConn.Endianness[i], protocolConn, plc4xTagName, logLevel)
					if len(valArrDbl) > 1 {
						for i := 0; i < len(valArrDbl); i++ {
							valStr = fmt.Sprintf("%f", valArrDbl[i])
							valJson = valStr
							//var valBson bson.D
							//bson.Unmarshal([]byte(valJson), &valBson)
							updOper := mongo.NewUpdateManyModel() // update one
							updOper.SetFilter(bson.D{
								{Key: "protocolSourceConnectionNumber", Value: protocolConn.ProtocolConnectionNumber},
								{Key: "protocolSourceObjectAddress", Value: plc4xTagName + "[" + fmt.Sprint(i) + "]"},
							})
							updOper.SetUpdate(bson.D{
								{Key: "$set", Value: bson.D{
									{Key: "sourceDataUpdate", Value: bson.D{
										{Key: "valueAtSource", Value: valArrDbl[i]},
										{Key: "valueStringAtSource", Value: valStr},
										{Key: "valueJsonAtSource", Value: valJson},
										//{Key: "valueBsonAtSource", Value: valBson},
										{Key: "invalidAtSource", Value: bad},
										{Key: "notTopicalAtSource", Value: false},
										{Key: "substitutedAtSource", Value: false},
										{Key: "blockedAtSource", Value: false},
										{Key: "overflowAtSource", Value: false},
										{Key: "transientAtSource", Value: false},
										{Key: "carryAtSource", Value: false},
										{Key: "asduAtSource", Value: v.GetPlcValueType().String()},
										{Key: "causeOfTransmissionAtSource", Value: 20},
										{Key: "timeTag", Value: time.Now()},
										//{Key: "timeTagAtSource", Value: time.Now()},
										//{Key: "timeTagAtSourceOk", Value: false},
									}},
								}},
							})
							updOpers = append(updOpers, updOper)
						}
					} else {
						var valBson bson.D
						bson.Unmarshal([]byte(valJson), &valBson)
						updOper := mongo.NewUpdateManyModel() // update one
						updOper.SetFilter(bson.D{
							{Key: "protocolSourceConnectionNumber", Value: protocolConn.ProtocolConnectionNumber},
							{Key: "protocolSourceObjectAddress", Value: plc4xTagName},
						})
						updOper.SetUpdate(bson.D{
							{Key: "$set", Value: bson.D{
								{Key: "sourceDataUpdate", Value: bson.D{
									{Key: "valueAtSource", Value: float64(valDbl)},
									{Key: "valueStringAtSource", Value: valStr},
									{Key: "valueJsonAtSource", Value: valJson},
									{Key: "valueBsonAtSource", Value: valBson},
									{Key: "invalidAtSource", Value: bad},
									{Key: "notTopicalAtSource", Value: false},
									{Key: "substitutedAtSource", Value: false},
									{Key: "blockedAtSource", Value: false},
									{Key: "overflowAtSource", Value: false},
									{Key: "transientAtSource", Value: false},
									{Key: "carryAtSource", Value: false},
									{Key: "asduAtSource", Value: v.GetPlcValueType().String()},
									{Key: "causeOfTransmissionAtSource", Value: 20},
									{Key: "timeTag", Value: time.Now()},
									//{Key: "timeTagAtSource", Value: time.Now()},
									//{Key: "timeTagAtSourceOk", Value: false},
								}},
							}},
						})
						updOpers = append(updOpers, updOper)
					}
				}
				if len(updOpers) > 0 {
					select {
					case chMongoBw <- updOpers: // Put values in the channel unless it is full
					default:
						fmt.Println("Error: mongo write channel full. Discarding values!")
					}
				}
				return nil
			}
			go func() {
				execRead()
				for range time.Tick(time.Second * time.Duration(protocolConn.GiInterval)) {
					if protocolConn.PlcConn != nil && protocolConn.PlcConn.IsConnected() {
						if !IsActive {
							log.Println("Instance inactive! Closing connection...")
							protocolConn.PlcConn.Close()
							protocolConn.PlcConn = nil
							break
						}
						err := execRead()
						if err == nil {
							continue
						}
						pingResultChannel := protocolConn.PlcConn.Ping()
						pingResult := <-pingResultChannel
						if pingResult.GetErr() != nil {
							log.Printf("Couldn't ping device: %s", pingResult.GetErr().Error())
							protocolConn.PlcConn.Close()
							protocolConn.PlcConn = nil
							break
						}
					} else {
						break
					}
				}
			}()
		}
		time.Sleep(2 * time.Second)
	}
}

func extractValue(
	v values.PlcValue,
	endianness string,
	protocolConn *protocolConnection,
	plc4xTagName string,
	logLevel int,
) (
	valDbl float64,
	valStr string,
	valJson string,
	valArrDbl []float64,
	bad bool,
) {
	valArrDbl = []float64{}
	valJson = "{}"
	switch v.GetPlcValueType().String() {
	default:
		bad = true
	case "Unknown", "NULL":
		bad = true
		valStr = v.GetPlcValueType().String()
		if ba, err := json.Marshal(v); err == nil {
			valJson = string(ba)
		}
	case "BOOL":
		if v.IsBool() {
			if v.GetBool() {
				valDbl = 1
				valStr = "true"
			} else {
				valDbl = 0
				valStr = "false"
			}
			if ba, err := json.Marshal(v.GetBool()); err == nil {
				valJson = string(ba)
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read result '%s': '%s'\n", plc4xTagName, valStr)
			}
		} else {
			bad = true
		}
	case "BYTE":
		if v.IsByte() {
			valDbl = float64(v.GetByte())
			valStr = fmt.Sprintf("%.0f", valDbl)
			if ba, err := json.Marshal(v.GetByte()); err == nil {
				valJson = string(ba)
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read result '%s': %.0f '%s'\n", plc4xTagName, valDbl, valStr)
			}
		} else {
			bad = true
		}
	case "USINT":
		if v.IsUint8() {
			valDbl = float64(v.GetUint8())
			valStr = fmt.Sprintf("%.0f", valDbl)
			if ba, err := json.Marshal(v.GetUint8()); err == nil {
				valJson = string(ba)
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read result '%s': %.0f '%s'\n", plc4xTagName, valDbl, valStr)
			}
		} else {
			bad = true
		}
	case "SINT":
		if v.IsInt8() {
			valDbl = float64(v.GetInt8())
			valStr = fmt.Sprintf("%.0f", valDbl)
			if ba, err := json.Marshal(v.GetInt8()); err == nil {
				valJson = string(ba)
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read result '%s': %.0f '%s'\n", plc4xTagName, valDbl, valStr)
			}
		} else {
			bad = true
		}
	case "UINT", "WORD":
		if v.IsUint16() {
			var vlAux uint16
			if endianness == "LITTLE_ENDIAN" {
				b2 := make([]byte, 2)
				binary.NativeEndian.PutUint16(b2, v.GetUint16())
				vlAux = binary.LittleEndian.Uint16(b2)
			} else if endianness == "BIG_ENDIAN" {
				b2 := make([]byte, 2)
				binary.NativeEndian.PutUint16(b2, v.GetUint16())
				vlAux = binary.BigEndian.Uint16(b2)
			} else if endianness == "REV_ENDIAN" {
				b2 := make([]byte, 2)
				binary.BigEndian.PutUint16(b2, v.GetUint16())
				vlAux = binary.LittleEndian.Uint16(b2)
			} else {
				vlAux = v.GetUint16()
			}
			valDbl = float64(vlAux)
			valStr = fmt.Sprintf("%.0f", valDbl)
			if ba, err := json.Marshal(vlAux); err == nil {
				valJson = string(ba)
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read result '%s': %.0f '%s'\n", plc4xTagName, valDbl, valStr)
			}
		} else {
			bad = true
		}
	case "INT":
		if v.IsInt16() {
			var vlAux int16
			if endianness == "LITTLE_ENDIAN" {
				b2 := make([]byte, 2)
				binary.BigEndian.PutUint16(b2, uint16(v.GetInt16()))
				vlAux = int16(binary.LittleEndian.Uint16(b2))
			} else {
				vlAux = v.GetInt16()
			}
			valDbl = float64(vlAux)
			valStr = fmt.Sprintf("%.0f", valDbl)
			if ba, err := json.Marshal(vlAux); err == nil {
				valJson = string(ba)
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read result '%s': %.0f '%s'\n", plc4xTagName, valDbl, valStr)
			}
		} else {
			bad = true
		}
	case "DINT":
		if v.IsInt32() {
			var vlAux int32
			if endianness == "LITTLE_ENDIAN" {
				b4 := make([]byte, 4)
				binary.BigEndian.PutUint32(b4, uint32(v.GetInt32()))
				vlAux = int32(binary.LittleEndian.Uint32(b4))
			} else {
				vlAux = v.GetInt32()
			}
			valDbl = float64(vlAux)
			valStr = fmt.Sprintf("%.0f", valDbl)
			if ba, err := json.Marshal(vlAux); err == nil {
				valJson = string(ba)
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read result '%s': %.0f '%s'\n", plc4xTagName, valDbl, valStr)
			}
		} else {
			bad = true
		}
	case "UDINT", "DWORD":
		if v.IsUint32() {
			var vlAux uint32
			if endianness == "LITTLE_ENDIAN" {
				b4 := make([]byte, 4)
				binary.BigEndian.PutUint32(b4, v.GetUint32())
				vlAux = binary.LittleEndian.Uint32(b4)
			} else {
				vlAux = v.GetUint32()
			}
			valDbl = float64(vlAux)
			valStr = fmt.Sprintf("%.0f", valDbl)
			if ba, err := json.Marshal(vlAux); err == nil {
				valJson = string(ba)
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read result '%s': %f '%s'\n", plc4xTagName, valDbl, valStr)
			}
		} else {
			bad = true
		}
	case "LINT":
		if v.IsInt64() {
			var vlAux int64
			if endianness == "LITTLE_ENDIAN" {
				b8 := make([]byte, 8)
				binary.BigEndian.PutUint64(b8, uint64(v.GetInt64()))
				vlAux = int64(binary.LittleEndian.Uint64(b8))
			} else {
				vlAux = v.GetInt64()
			}
			valDbl = float64(vlAux)
			valStr = fmt.Sprintf("%.0f", valDbl)
			if ba, err := json.Marshal(vlAux); err == nil {
				valJson = string(ba)
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read result '%s': %.0f '%s'\n", plc4xTagName, valDbl, valStr)
			}
		} else {
			bad = true
		}
	case "ULINT", "LWORD":
		if v.IsUint64() {
			var vlAux uint64
			if endianness == "LITTLE_ENDIAN" {
				b8 := make([]byte, 8)
				binary.BigEndian.PutUint64(b8, v.GetUint64())
				vlAux = binary.LittleEndian.Uint64(b8)
			} else {
				vlAux = v.GetUint64()
			}
			valDbl = float64(vlAux)
			valStr = fmt.Sprintf("%.0f", valDbl)
			if ba, err := json.Marshal(vlAux); err == nil {
				valJson = string(ba)
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read result '%s': %.0f '%s'\n", plc4xTagName, valDbl, valStr)
			}
		} else {
			bad = true
		}
	case "REAL":
		if v.IsFloat32() {
			var vlAux float32
			if endianness == "LITTLE_ENDIAN" {
				b4 := make([]byte, 4)
				binary.BigEndian.PutUint32(b4, v.GetUint32())
				vlAux = math.Float32frombits(binary.LittleEndian.Uint32(b4))
			} else {
				vlAux = v.GetFloat32()
			}
			valDbl = float64(vlAux)
			valStr = fmt.Sprintf("%f", valDbl)
			if ba, err := json.Marshal(valDbl); err == nil {
				valJson = string(ba)
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read result '%s': %f '%s'\n", plc4xTagName, valDbl, valStr)
			}
		} else {
			bad = true
		}
	case "LREAL":
		if v.IsFloat64() {
			var vlAux float64
			if endianness == "LITTLE_ENDIAN" {
				b8 := make([]byte, 8)
				binary.BigEndian.PutUint64(b8, v.GetUint64())
				vlAux = math.Float64frombits(binary.LittleEndian.Uint64(b8))
			} else {
				vlAux = v.GetFloat64()
			}
			valDbl = float64(vlAux)
			valStr = fmt.Sprintf("%f", valDbl)
			if ba, err := json.Marshal(valDbl); err == nil {
				valJson = string(ba)
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read result '%s': %f '%s'\n", plc4xTagName, valDbl, valStr)
			}
		} else {
			bad = true
		}
	case "TIME",
		"LTIME",
		"DATE",
		"LDATE",
		"TIME_OF_DAY",
		"LTIME_OF_DAY",
		"DATE_AND_TIME",
		"DATE_AND_LTIME",
		"LDATE_AND_TIME":
		if v.IsTime() || v.IsDate() || v.IsDateTime() || v.IsDuration() {
			var vlAux int64
			if endianness == "LITTLE_ENDIAN" {
				b8 := make([]byte, 8)
				binary.BigEndian.PutUint64(b8, uint64(v.GetDateTime().UnixMilli()))
				vlAux = int64(binary.LittleEndian.Uint64(b8))
			} else {
				vlAux = v.GetDateTime().UnixMilli()
			}

			valDbl := float64(vlAux)
			valStr = fmt.Sprintf("%.0f", valDbl)
			if ba, err := json.Marshal(v.GetDateTime()); err == nil {
				valJson = string(ba)
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read result '%s': %.0f '%s'\n", plc4xTagName, valDbl, v.GetDateTime().String())
			}
		} else {
			bad = true
		}
	case "CHAR":
	case "WCHAR":
	case "STRING":
	case "WSTRING":
		if v.IsString() {
			valStr := v.GetString()
			valDbl, _ = strconv.ParseFloat(valStr, 64)
			if ba, err := json.Marshal(valStr); err == nil {
				valJson = string(ba)
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read result '%s': '%s'\n", plc4xTagName, valStr)
			}
		} else {
			bad = true
		}
	case "Struct":
		if v.IsStruct() {
			valStr = fmt.Sprintf("%v", v.GetStruct())
			if ba, err := json.Marshal(v.GetStruct()); err == nil {
				valJson = string(ba)
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read result '%s': '%s'\n", plc4xTagName, valStr)
			}
		} else {
			bad = true
		}
	case "List":
		if v.IsList() {
			for i := 0; i < len(v.GetList()); i++ {
				vd, _, _, _, _ := extractValue(v.GetList()[i], endianness, protocolConn, plc4xTagName, logLevel)
				valArrDbl = append(valArrDbl, float64(vd))
				if i == 0 {
					valDbl = vd
				}
			}
			if ba, err := json.Marshal(valArrDbl); err != nil {
				log.Printf(protocolConn.Name+": error marshalling list: %s\n", err.Error())
			} else {
				valJson = string(ba)
				valStr = valJson
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read list result '%s': '%s'\n", plc4xTagName, valStr)
			}
		} else {
			bad = true
		}
	case "RAW_BYTE_ARRAY":
		if v.IsRaw() {
			vdArr := []float64{}
			for i := 0; i < len(v.GetRaw()); i++ {
				valArrDbl = append(vdArr, float64(v.GetRaw()[i]))
				if i == 0 {
					valDbl = float64(v.GetRaw()[i])
				}
			}
			if ba, err := json.Marshal(valArrDbl); err != nil {
				log.Printf(protocolConn.Name+": error marshalling raw array: %s\n", err.Error())
			} else {
				valJson = string(ba)
				valStr = valJson
			}
			if logLevel >= logLevelDetailed {
				log.Printf(protocolConn.Name+": Read raw array result '%s': '%s'\n", plc4xTagName, valStr)
			}
		} else {
			bad = true
		}
	}
	if logLevel >= logLevelDetailed && bad {
		log.Printf(protocolConn.Name+": Read result '%s': error reading %s! \n", plc4xTagName, v.GetPlcValueType().String())
	}
	return
}

func mongoWriter(
	cfg configData,
	instanceNumber int,
	chMongoBw chan []mongo.WriteModel,
	chProtConns chan []*protocolConnection,
	chRtData chan *mongo.Collection,
	chInstances chan *mongo.Collection,
	chInstanceId chan primitive.ObjectID,
	logLevel int,
) {
	for {
		log.Print("Mongodb - Try to connect server...")
		client, collectionRtData, collectionInstances, collectionConnections, collectionCommands, err := mongoConnect(cfg)
		if err != nil {
			log.Println("Mongodb - error connecting!")
			log.Println(err)
			time.Sleep(10 * time.Second)
			continue
		}
		defer client.Disconnect(context.TODO())
		protocolConns, csCommands, instanceId := configInstance(client, collectionInstances, collectionConnections, collectionCommands, instanceNumber)
		chInstanceId <- instanceId
		chProtConns <- protocolConns
		chInstances <- collectionInstances
		chRtData <- collectionRtData
		go iterateCommandsChangeStream(csCommands, protocolConns, collectionCommands)

		for updOpers := range chMongoBw {
			if len(updOpers) > 0 {
				res, err := collectionRtData.BulkWrite(
					context.Background(),
					updOpers,
					options.BulkWrite().SetOrdered(false),
				)
				if res == nil || err != nil {
					log.Println("Mongodb - bulk error!")
					log.Println(err)
					break
				}
				if logLevel >= logLevelDetailed {
					log.Printf("Mongodb - Opers: %d, Matched count: %d, Updated Count: %d\n", len(updOpers), res.MatchedCount, res.ModifiedCount)
				}
			}
		}
	}
}
