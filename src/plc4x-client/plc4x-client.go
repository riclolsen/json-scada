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
	"strings"
	"sync"
	"time"

	plc4go "github.com/apache/plc4x/plc4go/pkg/api"
	"github.com/apache/plc4x/plc4go/pkg/api/config"
	"github.com/apache/plc4x/plc4go/pkg/api/drivers"
	"github.com/apache/plc4x/plc4go/pkg/api/transports"
	"github.com/rs/zerolog"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
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
func iterateCommandsChangeStream(routineCtx context.Context, waitGroup *sync.WaitGroup, stream *mongo.ChangeStream, protConns []*protocolConnection, collectionCommands *mongo.Collection) {
	defer stream.Close(routineCtx)
	defer waitGroup.Done()
	for stream.Next(routineCtx) {
		if !IsActive {
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

func main() {
	log.SetOutput(os.Stdout) // log to standard output
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println(SoftwareVersion)
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
	var collectionRtData, collectionInstances, collectionConnections, collectionCommands *mongo.Collection
	someConnectionHasCommandsEnabled := false

	cfg, instanceNumber, ll := readConfigFile()
	logLevel = ll

	log.Print("Mongodb - Try to connect server...")
	client, collectionRtData, collectionInstances, collectionConnections, collectionCommands, err = mongoConnect(cfg)
	checkFatalError(err)
	defer client.Disconnect(context.TODO())
	protocolConns, csCommands := configInstance(client, collectionInstances, collectionConnections, collectionCommands, instanceNumber)

	var waitGroup sync.WaitGroup
	if someConnectionHasCommandsEnabled {
		waitGroup.Add(1)
		routineCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go iterateCommandsChangeStream(routineCtx, &waitGroup, csCommands, protocolConns, collectionCommands)
	}

	// keep retrying reconnection when disconnected
	for {
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

			if protocolConn.CommandsEnabled {
				someConnectionHasCommandsEnabled = true
			}

			// log connection info
			log.Printf("Instance:%d Connection:%d %s", protocolConn.ProtocolDriverInstanceNumber, protocolConn.ProtocolConnectionNumber, protocolConn.Name)
			log.Printf("%s: Server endpoint URL: %s", protocolConn.Name, protocolConn.EndpointURLs[0])
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
				log.Fatal(protocolConn.Name + ": Unsupported protocol - " + protocolConn.EndpointURLs[0])
				continue
			}
			_, _ = addrSep, addrParts

			// try to connect to plc
			connectionRequestChanel := driverManager.GetConnection(protocolConn.EndpointURLs[0])
			connectionResult := <-connectionRequestChanel
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
				var plc4xTagName, plc4xAddress, jsTagName string
				splNameAddr := strings.Split(topic, "|")
				if len(splNameAddr) > 1 {
					jsTagName = splNameAddr[0]
					plc4xAddress = splNameAddr[1]
					plc4xTagName = plc4xAddress
				} else {
					plc4xAddress = topic
				}
				plc4xTagName = plc4xAddress
				/*
					switch len(spl) {
					case 1:
						//jsTagName = fmt.Sprintf("PLC4X_%d_%s", protocolConn.ProtocolConnectionNumber, spl[0])
						plc4xAddress = spl[0]
						plc4xTagName = plc4xAddress
					case 2:
						//jsTagName = fmt.Sprintf("PLC4X_%d_%s.%s", protocolConn.ProtocolConnectionNumber, spl[0], spl[1])
						plc4xAddress = spl[0] + addrSep + spl[1]
						plc4xTagName = plc4xAddress
					case 3:
						//jsTagName = fmt.Sprintf("PLC4X_%d_%s.%s.%s", protocolConn.ProtocolConnectionNumber, spl[0], spl[1], spl[2])
						plc4xAddress = spl[0] + ":" + spl[1] + addrSep + spl[2]
						plc4xTagName = plc4xAddress
					case 4:
						//jsTagName = spl[0]
						plc4xAddress = spl[1] + addrSep + spl[2] + addrSep + spl[3]
						plc4xTagName = plc4xAddress
					default:
						log.Printf(protocolConn.Name+": error: wrong topic format: %s", topic)
						continue
					}
				*/
				reqBld.AddTagAddress(plc4xTagName, plc4xAddress)

				if protocolConn.AutoCreateTags {
					rtd := NewRtDataTag()
					rtd.Tag = jsTagName
					rtd.ProtocolSourceConnectionNumber = float64(protocolConn.ProtocolConnectionNumber)
					rtd.ProtocolSourceObjectAddress = plc4xAddress
					rtd.Group1 = DriverName
					rtd.Group2 = protocolConn.Name
					rtd.Group3 = topic
					protocolConn.AutoCreateTag(&rtd, collectionRtData)
					log.Printf(protocolConn.Name+": tagName: %s address: %s", rtd.Tag, plc4xAddress)
				} else {
					log.Printf(protocolConn.Name+": address: %s", plc4xAddress)
				}
			}
			protocolConn.ReadRequest, err = reqBld.Build()
			if err != nil {
				log.Printf(protocolConn.Name+": error preparing read-request: %s", connectionResult.GetErr().Error())
				log.Fatal(err)
			}

			execRead := func() error {
				log.Println(protocolConn.Name + ": integrity read...")

				// Execute a read-request
				readResponseChanel := protocolConn.ReadRequest.Execute()

				// Wait for the response to finish
				readRequestResult := <-readResponseChanel
				if readRequestResult.GetErr() != nil {
					log.Printf(protocolConn.Name+": error executing read-request: %s", readRequestResult.GetErr().Error())
					return readRequestResult.GetErr()
				}

				// Do something with the response
				for _, plc4xTagName := range readRequestResult.GetResponse().GetTagNames() {
					v := readRequestResult.GetResponse().GetValue(plc4xTagName)
					var valDbl float64 = 0
					var valStr string = ""
					switch v.GetPlcValueType().String() {
					case "Unknown":
					case "NULL":
					case "BOOL":
					case "BYTE":
					case "WORD":
					case "DWORD":
					case "LWORD":
					case "USINT":
					case "UINT":
					case "UDINT":
					case "ULINT":
					case "SINT":
					case "INT":
						log.Printf(protocolConn.Name+": Read result '%s': %04xh %d\n", plc4xTagName, v.GetInt16(), v.GetInt16())
						valDbl = float64(v.GetFloat64())
						/*
							wReqBld := protocolConn.PlcConn.WriteRequestBuilder()
							wReqBld.AddTagAddress(plc4xTagName, plc4xTagName, v.GetInt16()+1)
							if wReq, err := wReqBld.Build(); err != nil {
								log.Printf(protocolConn.Name+": error preparing write-request: %s", err.Error())
							} else {
								// Execute a write-request
								resChan := wReq.Execute()
								// Wait for the response to finish
								wResponse := <-resChan
								log.Println(wResponse.GetResponse().GetResponseCode(plc4xTagName))
								log.Println(wResponse.String())
							}
						*/

					case "DINT":
					case "LINT":
					case "REAL":
					case "LREAL":
					case "CHAR":
					case "WCHAR":
					case "STRING":
						valStr = v.GetString()
						log.Printf(protocolConn.Name+": Read result '%s': %s\n", plc4xTagName, valStr)
					case "WSTRING":
					case "TIME":
					case "LTIME":
					case "DATE":
					case "LDATE":
					case "TIME_OF_DAY":
					case "LTIME_OF_DAY":
					case "DATE_AND_TIME":
					case "DATE_AND_LTIME":
					case "LDATE_AND_TIME":
					case "Struct":
					case "List":
					case "RAW_BYTE_ARRAY":
					}
					_ = valDbl
				}
				return nil
			}
			go func() {
				execRead()
				for range time.Tick(time.Millisecond * 100) {
					if protocolConn.PlcConn != nil && protocolConn.PlcConn.IsConnected() {
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

		time.Sleep(10 * time.Second)
	}

	// Make sure the connection is closed at the end
	defer protocolConns[0].PlcConn.Close()

}
