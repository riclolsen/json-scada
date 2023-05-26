/*
 * This process calculates point values based of predefined formulas and configured parcels.
 * All data is read from and results are written to the MongoDB server.
 * {json:scada} - Copyright (c) 2020 - 2023 - Ricardo L. Olsen
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
	"io"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const softwareVersion string = "0.1.2"
const processName string = "CALCULATIONS"
const appMsg string = "{json:scada} - " + processName + " - Version " + softwareVersion
const appUsage string = "Usage: calculations [instance number] [log level] [period of calculation in seconds] [config file path/name]"
const appUsageDefaults string = "Default args: calculations 1 1 2.0 ../conf/json-scada.json"

var mongoClient *mongo.Client // global mongodb connection handle

var defaultConfigFileName string = "json-scada.json"
var configFileCompletePath string = ""
var realtimeDataConnectionName string = "realtimeData"
var instanceNumber int = 1
var logLevel int = 1
var periodOfCalculation float64 = 2.0 // cycle period of calculation in seconds
var isActive bool = false             // redundancy flag, do not write calculations to the DB while inactive

type config struct {
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

type pointCalc struct {
	calc      int
	idParcels []int
}

type realtimeData struct {
	ID      int     `bson:"_id"`
	VALUE   float64 `bson:"value"`
	INVALID bool    `bson:"invalid"`
}

type realtimeDataForm struct {
	ID      int   `bson:"_id"`
	FORMULA int   `bson:"formula"`
	PARCELS []int `bson:"parcels"`
}

type processInstance struct {
	ProcessName                string    `bson:"processName"`
	ProcessInstanceNumber      int       `bson:"processInstanceNumber"`
	Enabled                    bool      `bson:"enabled"`
	LogLevel                   int       `bson:"logLevel"`
	NodeNames                  []string  `bson:"nodeNames"`
	ActiveNodeName             string    `bson:"activeNodeName"`
	ActiveNodeKeepAliveTimeTag time.Time `bson:"activeNodeKeepAliveTimeTag"`
	SoftwareVersion            string    `bson:"softwareVersion"`
	PeriodOfCalculation        float64   `bson:"periodOfCalculation"`
}

// Reads the config file
func readConfigFile(cfg *config) {
	if configFileCompletePath == "" {
		configFileCompletePath = filepath.Join("..", "conf", defaultConfigFileName)
	}

	// tries to open and read the json config file
	jsonFile, err := os.Open(configFileCompletePath)
	if err != nil {
		log.Printf("Fail to read file: %v", err)
		os.Exit(1)
	}
	byteValue, _ := io.ReadAll(jsonFile)

	// unmarshals the json file's content into a config structure
	err = json.Unmarshal(byteValue, &cfg)
	if err != nil {
		log.Printf("Error parsing json config file: %v", err)
		os.Exit(1)
	}

	cfg.MongoConnectionString = strings.TrimSpace(cfg.MongoConnectionString)
	cfg.MongoDatabaseName = strings.TrimSpace(cfg.MongoDatabaseName)
	cfg.NodeName = strings.TrimSpace(cfg.NodeName)
	if cfg.MongoConnectionString == "" || cfg.MongoDatabaseName == "" || cfg.NodeName == "" {
		log.Printf("Empty string in config file.")
		os.Exit(1)
	}
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
}

// Reads the config file and connects to MongoDB server
func mongoConnect(cfg config) (client *mongo.Client, colRTD *mongo.Collection, err error) {

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	client, err = mongo.NewClient(options.Client().ApplyURI(cfg.MongoConnectionString))
	if err != nil {
		mongoClient = nil
		return client, colRTD, err
	}
	err = client.Connect(ctx)
	if err != nil {
		mongoClient = nil
		return client, colRTD, err
	}
	mongoClient = client
	colRTD = client.Database(cfg.MongoDatabaseName).Collection(realtimeDataConnectionName)

	return client, colRTD, err
}

// Check for processInstances entry, if not found create one with defaults
// Keep checking active node and update keep alive time while active
func processRedundancy(cfg config) {
	const countKeepAliveUpdatesLimit = 4
	var countKeepAliveUpdates = 0
	var lastActiveNodeKeepAliveTimeTag time.Time
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// repeat for time period circa 5s (plus randomized time up to 100 ms to avoid exact sync with other nodes)
	for range time.Tick(time.Duration(5)*time.Second + time.Duration(100*r.Float64())*time.Millisecond) {

		if mongoClient == nil { // not connected?
			log.Println("Redundancy - Disconnected from Mongodb server!")
			continue
		}

		var collectionProcessInstances = mongoClient.Database(cfg.MongoDatabaseName).Collection("processInstances")
		var instance processInstance
		filter := bson.D{{Key: "processName", Value: processName}}
		err := collectionProcessInstances.FindOne(context.TODO(), filter).Decode(&instance)
		if err != nil && err != mongo.ErrNoDocuments {
			log.Println("Redundancy - Error querying processInstances!")
			log.Println(err)
		} else {
			if err == mongo.ErrNoDocuments {
				log.Println("Redundancy - No process instance found!")
				_, err := collectionProcessInstances.InsertOne(context.TODO(),
					bson.M{
						"processName":                processName,
						"processInstanceNumber":      instanceNumber,
						"enabled":                    true,
						"logLevel":                   logLevel,
						"nodeNames":                  bson.A{},
						"activeNodeName":             cfg.NodeName,
						"activeNodeKeepAliveTimeTag": primitive.NewDateTimeFromTime(time.Now()),
						"softwareVersion":            softwareVersion,
						"periodOfCalculation":        periodOfCalculation,
					})
				if err != nil {
					log.Println("Redundancy - Error inserting in processInstances!")
					log.Println(err)
					os.Exit(2)
				}
				continue
			} else {
				if !instance.Enabled {
					log.Println("Redundancy - Process instance disabled!")
					os.Exit(0)
				}
				if len(instance.NodeNames) > 0 { // check if node names allowed are limited
					var found bool = false
					for i := range instance.NodeNames {
						if instance.NodeNames[i] == cfg.NodeName {
							found = true
							break
						}
					}
					if !found {
						log.Println("Redundancy - Node name not allowed!")
						os.Exit(0)
					}
				}
				if instance.LogLevel > logLevel {
					logLevel = instance.LogLevel
					log.Println("Redundancy - Log level updated to ", logLevel)
				}
				if instance.PeriodOfCalculation > periodOfCalculation {
					periodOfCalculation = instance.PeriodOfCalculation
					log.Println("Redundancy - Period of calculation updated to ", periodOfCalculation)
				}
				// check node active
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
					_, err := collectionProcessInstances.UpdateOne(
						context.TODO(),
						bson.M{"processName": bson.M{"$eq": instance.ProcessName}},
						bson.M{"$set": bson.M{"activeNodeName": cfg.NodeName, "activeNodeKeepAliveTimeTag": primitive.NewDateTimeFromTime(time.Now())}},
					)
					if err != nil {
						log.Println("Redundancy - Error updating in processInstances!")
						log.Println(err)
					}
				} else {
					log.Println("Redundancy - This node is inactive.")
				}

			}
		}
	}
}

func main() {
	log.SetOutput(os.Stdout) // log to standard output
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Println(appMsg)
	log.Println(appUsage)
	log.Println(appUsageDefaults)

	if os.Getenv("JS_CALCULATIONS_INSTANCE") != "" {
		i, err := strconv.Atoi(os.Getenv("JS_CALCULATIONS_INSTANCE"))
		if err != nil {
			log.Println("JS_CALCULATIONS_INSTANCE environment variable should be a number!")
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
	if os.Getenv("JS_CALCULATIONS_LOGLEVEL") != "" {
		i, err := strconv.Atoi(os.Getenv("JS_CALCULATIONS_LOGLEVEL"))
		if err != nil {
			log.Println("JS_CALCULATIONS_LOGLEVEL environment variable should be a number!")
			os.Exit(2)
		}
		logLevel = i
	}
	if len(os.Args) > 2 {
		i, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Println("Log Level parameter should be a number!")
			os.Exit(2)
		}
		logLevel = i
	}
	if os.Getenv("JS_CALCULATIONS_PERIOD") != "" {
		f, err := strconv.ParseFloat(os.Getenv("JS_CALCULATIONS_PERIOD"), 64)
		if err != nil {
			log.Println("JS_CALCULATIONS_PERIOD environment variable should be a number!")
			os.Exit(2)
		}
		periodOfCalculation = f
	}
	if len(os.Args) > 3 {
		f, err := strconv.ParseFloat(os.Args[3], 64)
		if err != nil {
			log.Println("Period of Calculation parameter should be a number!")
			os.Exit(2)
		}
		periodOfCalculation = f
	}
	if os.Getenv("JS_CONFIG_FILE") != "" {
		configFileCompletePath = os.Getenv("JS_CONFIG_FILE")
	}
	if len(os.Args) > 4 {
		configFileCompletePath = os.Args[4]
	}

	log.Println("Instance number: ", instanceNumber)
	log.Println("Log level: ", logLevel)
	log.Println("Period of calculation (s): ", periodOfCalculation)
	log.Println("Config file: ", configFileCompletePath)

	var cfg config
	readConfigFile(&cfg)
	client, collection, err := mongoConnect(cfg)
	if err != nil {
		log.Fatal(err)
	}

	go processRedundancy(cfg)

	calcs := make(map[int]*pointCalc)
	var nponto int

	projection := bson.D{
		{Key: "_id", Value: 1},
		{Key: "formula", Value: 1},
		{Key: "parcels", Value: 1},
	}
	cur, err := collection.Find(context.Background(),
		bson.D{
			{Key: "formula", Value: bson.D{
				{Key: "$gt", Value: 0},
			}},
		},
		options.Find().SetProjection(projection),
	)
	if err != nil {
		log.Print("find")
		log.Fatal(err)
	}
	for cur.Next(context.Background()) {
		elem := &realtimeDataForm{}
		err := cur.Decode(elem)
		if err != nil {
			log.Print("decode")
			log.Print(err)
			continue
		}

		nponto = elem.ID
		calcs[nponto] = &pointCalc{
			calc:      elem.FORMULA,
			idParcels: []int{},
		}
		calcs[nponto].calc = elem.FORMULA
		for _, parcel := range elem.PARCELS {
			calcs[nponto].idParcels = append(calcs[nponto].idParcels, parcel)
			if logLevel > 1 {
				log.Printf("%d %d\n", nponto, parcel)
			}
		}
	}

	// maps for values and flags of all parcels and calculated points
	vals := make(map[int]float64)
	invalids := make(map[int]bool)

	// creates the find array for all parcels
	barr := bson.A{}
	for pointnum, p := range calcs {
		barr = append(barr, pointnum)
		for _, idparc := range p.idParcels {
			barr = append(barr, idparc)
		}
	}

	projection = bson.D{
		{Key: "_id", Value: 1},
		{Key: "value", Value: 1},
		{Key: "invalid", Value: 1},
	}
	for {
		if !isActive {
			time.Sleep(1 * time.Second)
			continue
		}
		tbegin := time.Now()
		after := tbegin.Add(time.Duration(periodOfCalculation) * time.Second)

		// Check the connection
		errp := client.Ping(context.TODO(), nil)
		if errp != nil {
			log.Printf("%s \n", err)
			client.Disconnect(context.TODO())
			client, collection, _ = mongoConnect(cfg)
		}

		// find all parcel and current calculated values
		cur, err := collection.Find(context.Background(),
			bson.D{
				{Key: "_id", Value: bson.D{
					{Key: "$in", Value: barr},
				}},
			},
			options.Find().SetProjection(projection),
		)
		if err != nil {
			log.Print("find")
			log.Fatal(err)
		}

		var cntReads = 0
		for cur.Next(context.Background()) {
			elem := &realtimeData{}
			err := cur.Decode(elem)
			if err != nil {
				log.Print("decode")
				log.Print(err)
				continue
			}

			vals[elem.ID] = elem.VALUE
			invalids[elem.ID] = elem.INVALID
			cntReads++
			// log.Printf("ID %d VAL %f\n", elem.ID, elem.VALUE)
		}

		log.Printf("Read %v points from MongoDB.\n", cntReads)

		cur.Close(context.Background())

		var opers []mongo.WriteModel

		// loop over all calcs
		for id, p := range calcs {

			ok := false
			val := 0.0
			invalid := true
			transient := false
			switch p.calc {
			default:
				if logLevel > 1 {
					log.Println("Formula not available ", p.calc)
				}
			case 1: // CURRENT
				if len(p.idParcels) == 3 {
					if vals[p.idParcels[2]] > 0 {
						val = 577.35027 * math.Sqrt(vals[p.idParcels[0]]*vals[p.idParcels[0]]+vals[p.idParcels[1]]*vals[p.idParcels[1]]) / vals[p.idParcels[2]]
					}
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]]
					ok = true
				}
			case 2: // Power Factor P1 / sqrt ((P1 * P1) + (P2 * P2))
				if len(p.idParcels) == 2 {
					val = vals[p.idParcels[0]] / (math.Sqrt(vals[p.idParcels[0]]*vals[p.idParcels[0]] + vals[p.idParcels[1]]*vals[p.idParcels[1]]))
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]]
					ok = true
				}
			case 3: // Apparent Power
				if len(p.idParcels) == 2 {
					val = math.Sqrt(vals[p.idParcels[0]]*vals[p.idParcels[0]] + vals[p.idParcels[1]]*vals[p.idParcels[1]])
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]]
					ok = true
				}
			case 4: // POSITIVE SUM
				invalid = false
				for _, elem := range p.idParcels {
					val += vals[elem]
					invalid = invalid || invalids[elem]
				}
				ok = true
			case 5: // SQRT
				if len(p.idParcels) == 1 {
					val = math.Sqrt(vals[p.idParcels[0]])
					invalid = invalids[p.idParcels[0]]
					ok = true
				}
			case 6: // AND
				invalid = false
				val = 1
				for _, elem := range p.idParcels {
					if vals[elem] == 0 {
						val = 0
					}
					invalid = invalid || invalids[elem]
				}
				ok = true
			case 7: // OR
				invalid = false
				val = 0
				for _, elem := range p.idParcels {
					if vals[elem] != 0 {
						val = 1
					}
					invalid = invalid || invalids[elem]
				}
				ok = true
			case 8: // timer
				val = float64(time.Now().Unix())
				invalid = false
				ok = true
			case 9: // Apparent Power based on amps and kV
				if len(p.idParcels) == 2 {
					val = vals[p.idParcels[0]] * vals[p.idParcels[1]] * math.Sqrt(3) / 1000
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]]
					ok = true
				}
			case 10: // NEGATIVE SUM
				invalid = false
				for _, elem := range p.idParcels {
					val -= vals[elem]
					invalid = invalid || invalids[elem]
				}
				ok = true
			case 11: // P1 + P2 + P3 + P4 + P5 + P6 + P7 + P8 + P9 + P10 + P11 + P12+ P13 + P14 + P15 + P16 + P17 + P18 + P19 + P20 + P21 + P22 + P23 + (P24 * 0.72)
				if len(p.idParcels) == 24 {
					val = vals[p.idParcels[0]] +
						vals[p.idParcels[1]] +
						vals[p.idParcels[2]] +
						vals[p.idParcels[3]] +
						vals[p.idParcels[4]] +
						vals[p.idParcels[5]] +
						vals[p.idParcels[6]] +
						vals[p.idParcels[7]] +
						vals[p.idParcels[8]] +
						vals[p.idParcels[9]] +
						vals[p.idParcels[10]] +
						vals[p.idParcels[11]] +
						vals[p.idParcels[12]] +
						vals[p.idParcels[13]] +
						vals[p.idParcels[14]] +
						vals[p.idParcels[15]] +
						vals[p.idParcels[16]] +
						vals[p.idParcels[17]] +
						vals[p.idParcels[18]] +
						vals[p.idParcels[19]] +
						vals[p.idParcels[20]] +
						vals[p.idParcels[21]] +
						vals[p.idParcels[22]] +
						vals[p.idParcels[23]]*0.72
					invalid = invalids[p.idParcels[0]] ||
						invalids[p.idParcels[1]] ||
						invalids[p.idParcels[2]] ||
						invalids[p.idParcels[3]] ||
						invalids[p.idParcels[4]] ||
						invalids[p.idParcels[5]] ||
						invalids[p.idParcels[6]] ||
						invalids[p.idParcels[7]] ||
						invalids[p.idParcels[8]] ||
						invalids[p.idParcels[9]] ||
						invalids[p.idParcels[10]] ||
						invalids[p.idParcels[11]] ||
						invalids[p.idParcels[12]] ||
						invalids[p.idParcels[13]] ||
						invalids[p.idParcels[14]] ||
						invalids[p.idParcels[15]] ||
						invalids[p.idParcels[16]] ||
						invalids[p.idParcels[17]] ||
						invalids[p.idParcels[18]] ||
						invalids[p.idParcels[19]] ||
						invalids[p.idParcels[20]] ||
						invalids[p.idParcels[21]] ||
						invalids[p.idParcels[22]] ||
						invalids[p.idParcels[23]]
					ok = true
				}
			case 13: // (P1 + P2 + P3 + P4 + P5 + P6 + P7 + P8) -
				// (P9) + (P10 + P11 + P12) + (P13 * 0.52) +
				// (P14 + P15) - (P16 + P17) + (P18 + P19 + P20 + P21 + P22) -
				// (P23 + P24) + (P25 + P26 + P27 + P28 + P29 + P30 + P31 + P32 + P33) +
				// (P34 * 0.2) - 1
				if len(p.idParcels) == 34 {
					val =
						(vals[p.idParcels[0]] +
							vals[p.idParcels[1]] +
							vals[p.idParcels[2]] +
							vals[p.idParcels[3]] +
							vals[p.idParcels[4]] +
							vals[p.idParcels[5]] +
							vals[p.idParcels[6]] +
							vals[p.idParcels[7]]) - vals[p.idParcels[8]] +
							vals[p.idParcels[9]] +
							vals[p.idParcels[10]] +
							vals[p.idParcels[11]] +
							vals[p.idParcels[12]]*0.52 +
							vals[p.idParcels[13]] +
							vals[p.idParcels[14]] -
							(vals[p.idParcels[15]] + vals[p.idParcels[16]]) +
							vals[p.idParcels[17]] +
							vals[p.idParcels[18]] +
							vals[p.idParcels[19]] +
							vals[p.idParcels[20]] +
							vals[p.idParcels[21]] - (vals[p.idParcels[22]] + vals[p.idParcels[23]]) +
							vals[p.idParcels[22]] +
							vals[p.idParcels[23]] +
							vals[p.idParcels[24]] +
							vals[p.idParcels[25]] +
							vals[p.idParcels[26]] +
							vals[p.idParcels[27]] +
							vals[p.idParcels[28]] +
							vals[p.idParcels[29]] +
							vals[p.idParcels[30]] +
							vals[p.idParcels[31]] +
							vals[p.idParcels[32]] +
							vals[p.idParcels[33]]*0.2 - 1
					invalid = invalids[p.idParcels[0]] ||
						invalids[p.idParcels[1]] ||
						invalids[p.idParcels[2]] ||
						invalids[p.idParcels[3]] ||
						invalids[p.idParcels[4]] ||
						invalids[p.idParcels[5]] ||
						invalids[p.idParcels[6]] ||
						invalids[p.idParcels[7]] ||
						invalids[p.idParcels[8]] ||
						invalids[p.idParcels[9]] ||
						invalids[p.idParcels[10]] ||
						invalids[p.idParcels[11]] ||
						invalids[p.idParcels[12]] ||
						invalids[p.idParcels[13]] ||
						invalids[p.idParcels[14]] ||
						invalids[p.idParcels[15]] ||
						invalids[p.idParcels[16]] ||
						invalids[p.idParcels[17]] ||
						invalids[p.idParcels[18]] ||
						invalids[p.idParcels[19]] ||
						invalids[p.idParcels[20]] ||
						invalids[p.idParcels[21]] ||
						invalids[p.idParcels[22]] ||
						invalids[p.idParcels[23]] ||
						invalids[p.idParcels[24]] ||
						invalids[p.idParcels[25]] ||
						invalids[p.idParcels[26]] ||
						invalids[p.idParcels[27]] ||
						invalids[p.idParcels[28]] ||
						invalids[p.idParcels[29]] ||
						invalids[p.idParcels[30]] ||
						invalids[p.idParcels[31]] ||
						invalids[p.idParcels[32]] ||
						invalids[p.idParcels[33]]
					ok = true
				}
			case 14: //	(P1 * 10) / 6
				if len(p.idParcels) == 1 {
					val = (vals[p.idParcels[0]] * 10) / 6
					invalid = invalids[p.idParcels[0]]
					ok = true
				}
			case 15: // DIFFERENCE
				if len(p.idParcels) == 2 {
					val = vals[p.idParcels[0]] - vals[p.idParcels[1]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]]
					ok = true
				}
			case 16: // P1 - P2 - P3
				if len(p.idParcels) == 3 {
					val = vals[p.idParcels[0]] - vals[p.idParcels[1]] - vals[p.idParcels[2]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]]
					ok = true
				}
			case 17: // P1 - P2 - P3 - P4
				if len(p.idParcels) == 4 {
					val = vals[p.idParcels[0]] - vals[p.idParcels[1]] - vals[p.idParcels[2]] - vals[p.idParcels[3]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]]
					ok = true
				}
			case 18: // P1 - P2 - P3 - P4 - P5 - P6
				if len(p.idParcels) == 6 {
					val = vals[p.idParcels[0]] - vals[p.idParcels[1]] - vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]]
					ok = true
				}
			case 19: // P1 + P2 - P3
				if len(p.idParcels) == 3 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] - vals[p.idParcels[2]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]]
					ok = true
				}
			case 20: // P1 + P2 + P3 - P4 - P5 - P6
				if len(p.idParcels) == 6 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]]
					ok = true
				}
			case 21: // P1 + P2 + P3 + P4 - P5 - P6 - P7 - P8 - P9
				if len(p.idParcels) == 9 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] + vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]] - vals[p.idParcels[6]] - vals[p.idParcels[7]] - vals[p.idParcels[8]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]] || invalids[p.idParcels[8]]
					ok = true
				}
			case 22: // P1 + P2 + P3 + P4 + P5 + P6 - P7 - P8 - P9
				if len(p.idParcels) == 9 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] + vals[p.idParcels[3]] + vals[p.idParcels[4]] + vals[p.idParcels[5]] - vals[p.idParcels[6]] - vals[p.idParcels[7]] - vals[p.idParcels[8]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]] || invalids[p.idParcels[8]]
					ok = true
				}
			case 23: // P1 + P2 + P3 + P4 + P5 + P6 + P7 + P8 + P9 + P10 - P11
				if len(p.idParcels) == 11 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] + vals[p.idParcels[3]] + vals[p.idParcels[4]] + vals[p.idParcels[5]] + vals[p.idParcels[6]] + vals[p.idParcels[7]] + vals[p.idParcels[8]] + vals[p.idParcels[9]] - vals[p.idParcels[10]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]] || invalids[p.idParcels[8]] || invalids[p.idParcels[9]] || invalids[p.idParcels[10]]
					ok = true
				}
			case 24: // P1 + P2 + P3 - P4 - P5 - P6 - P7 - P8 - P9
				if len(p.idParcels) == 9 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]] - vals[p.idParcels[6]] - vals[p.idParcels[7]] - vals[p.idParcels[8]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]] || invalids[p.idParcels[8]]
					ok = true
				}
			case 26: // RES=P2;if (abs(P1)>1.4) RES=0; if (abs(P1)<=0.5) RES=1
				if len(p.idParcels) == 2 {
					val = vals[p.idParcels[1]]
					if math.Abs(vals[p.idParcels[0]]) > 1.4 {
						val = 0
					}
					if math.Abs(vals[p.idParcels[0]]) <= 0.5 {
						val = 1
					}
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]]
					ok = true
				}

			case 50, 51: // DIGITAL/ANALOG CHOICE (pick the first ok value)
				invalid = invalids[p.idParcels[0]]
				val = vals[p.idParcels[0]]
				for _, elem := range p.idParcels {
					if !invalids[elem] {
						val = vals[elem]
						invalid = false
					}
				}
				ok = true
			case 52: // Any ok? (1 if any parcel is ok)
				invalid = false
				val = 0
				for _, elem := range p.idParcels {
					if !invalids[elem] {
						val = 1
					}
				}
				ok = true
			case 53: // MAX SPAN (difference between max / min parcel values)
				max := -math.MaxFloat64
				min := math.MaxFloat64
				invalid = false
				val = 0
				for _, elem := range p.idParcels {
					if vals[elem] > max {
						max = vals[elem]
					}
					if vals[elem] < min {
						min = vals[elem]
					}
					invalid = invalid || invalids[elem]
				}
				val = max - min
				ok = true
			case 54: // double point from 2 single OFF / ON = OFF,  ON / OFF = ON, equal values = bad
				val = vals[p.idParcels[0]]
				invalid = false
				transient = false
				if len(p.idParcels) == 2 {
					if vals[p.idParcels[0]] == 0 && vals[p.idParcels[1]] != 0 {
						val = 0
					}
					if vals[p.idParcels[0]] != 0 && vals[p.idParcels[1]] == 0 {
						val = 1
					}
					if vals[p.idParcels[0]] == vals[p.idParcels[1]] {
						transient = true
						invalid = true
					}
					invalid = invalid || invalids[p.idParcels[0]] || invalids[p.idParcels[1]]
					ok = true
				}
			case 55: // DIVISION P1/P2
				if len(p.idParcels) == 2 {
					if vals[p.idParcels[1]] == 0 { // avoids division by zero
						if vals[p.idParcels[0]] == 0 {
							val = 0
						} else if vals[p.idParcels[0]] > 0 {
							val = math.MaxFloat64
						} else {
							val = -math.MaxFloat64
						}
					} else {
						val = vals[p.idParcels[0]] / vals[p.idParcels[1]]
					}
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]]
					ok = true
				}
			case 200: // P1-P2-P3-P4-P5-P6-P7-P8
				if len(p.idParcels) == 8 {
					val = vals[p.idParcels[0]] - vals[p.idParcels[1]] - vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]] - vals[p.idParcels[6]] - vals[p.idParcels[7]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]]
					ok = true
				}
			case 201: // P1+P2+P3+P4+P5+P6+P7+P8-P9-P10-P11
				if len(p.idParcels) == 11 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] + vals[p.idParcels[3]] + vals[p.idParcels[4]] + vals[p.idParcels[5]] + vals[p.idParcels[6]] + vals[p.idParcels[7]] - vals[p.idParcels[8]] - vals[p.idParcels[9]] - vals[p.idParcels[10]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]] || invalids[p.idParcels[8]] || invalids[p.idParcels[9]] || invalids[p.idParcels[10]]
					ok = true
				}
			case 202: // ( P1 * 60 ) + P2
				if len(p.idParcels) == 2 {
					val = vals[p.idParcels[0]]*60 + vals[p.idParcels[1]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]]
					ok = true
				}
			case 203: // Choose from 2 measures P1 !=0 THEN P3 ELSE P2
				if len(p.idParcels) == 3 {
					if vals[p.idParcels[0]] != 0 {
						val = vals[p.idParcels[2]]
						invalid = invalids[p.idParcels[2]]
					} else {
						val = vals[p.idParcels[1]]
						invalid = invalids[p.idParcels[1]]
					}
					ok = true
				}
			case 204: // P1/2
				if len(p.idParcels) == 1 {
					val = vals[p.idParcels[0]] / 2
					invalid = invalids[p.idParcels[0]]
					ok = true
				}
			case 205: // P1+P2-P3-P4-P5
				if len(p.idParcels) == 5 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] - vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]]
					ok = true
				}
			case 206: // P1-P2-P3-P4-P5-P6-P7-P8-P9
				if len(p.idParcels) == 9 {
					val = vals[p.idParcels[0]] - vals[p.idParcels[1]] - vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]] - vals[p.idParcels[6]] - vals[p.idParcels[7]] - vals[p.idParcels[8]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]] || invalids[p.idParcels[8]]
					ok = true
				}
			case 207: // P1+P2-P3-P4-P5-P6
				if len(p.idParcels) == 6 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] - vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]]
					ok = true
				}
			case 208: // P1-P2-P3-P4-P5
				if len(p.idParcels) == 5 {
					val = vals[p.idParcels[0]] - vals[p.idParcels[1]] - vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]]
					ok = true
				}
			case 209: // P1+P2-P3-P4-P5-P6-P7-P8-P9
				if len(p.idParcels) == 9 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] - vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]] - vals[p.idParcels[6]] - vals[p.idParcels[7]] - vals[p.idParcels[8]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]] || invalids[p.idParcels[8]]
					ok = true
				}
			case 210: // P1-P2-P3-P4-P5-P6-P7
				if len(p.idParcels) == 7 {
					val = vals[p.idParcels[0]] - vals[p.idParcels[1]] - vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]] - vals[p.idParcels[6]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]]
					ok = true
				}
			case 211: // P1+P2+P3+P4+P5-P6-P7-P8-P9
				if len(p.idParcels) == 9 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] + vals[p.idParcels[3]] + vals[p.idParcels[4]] - vals[p.idParcels[5]] - vals[p.idParcels[6]] - vals[p.idParcels[7]] - vals[p.idParcels[8]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]] || invalids[p.idParcels[8]]
					ok = true
				}
			case 213: // P1+P2+P3-P4
				if len(p.idParcels) == 4 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] - vals[p.idParcels[3]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]]
					ok = true
				}
			case 214: // P1+P2+P3+P4-P5
				if len(p.idParcels) == 5 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] + vals[p.idParcels[3]] - vals[p.idParcels[4]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]]
					ok = true
				}
			case 215: // P1+P2-P3-P4
				if len(p.idParcels) == 4 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] - vals[p.idParcels[2]] - vals[p.idParcels[3]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]]
					ok = true
				}
			case 216: // IF P2 <= P1 <= P3 THEN 1 ELSE 0
				if len(p.idParcels) == 3 {
					val = 0
					if vals[p.idParcels[1]] <= vals[p.idParcels[0]] && vals[p.idParcels[0]] <= vals[p.idParcels[2]] {
						val = 1
					}
					invalid = invalids[p.idParcels[0]]
					ok = true
				}
			case 217: // P1+P2+P3+P4+P5-P6
				if len(p.idParcels) == 6 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] + vals[p.idParcels[3]] + vals[p.idParcels[4]] - vals[p.idParcels[5]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]]
					ok = true
				}
			case 218: // IF P2 < P1 <= P3 THEN 1 ELSE 0
				if len(p.idParcels) == 3 {
					val = 0
					if vals[p.idParcels[1]] < vals[p.idParcels[0]] && vals[p.idParcels[0]] <= vals[p.idParcels[2]] {
						val = 1
					}
					invalid = invalids[p.idParcels[0]]
					ok = true
				}
			case 219: // IF P2 <= P1 < P3 THEN 1 ELSE 0
				if len(p.idParcels) == 3 {
					val = 0
					if vals[p.idParcels[1]] <= vals[p.idParcels[0]] && vals[p.idParcels[0]] < vals[p.idParcels[2]] {
						val = 1
					}
					invalid = invalids[p.idParcels[0]]
					ok = true
				}
			case 220: // LT( P1 , P2 )
				if len(p.idParcels) == 2 {
					val = vals[p.idParcels[0]]
					if vals[p.idParcels[1]] < vals[p.idParcels[0]] {
						val = vals[p.idParcels[1]]
					}
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]]
					ok = true
				}
			case 221: // GT( P1 , P2 )
				if len(p.idParcels) == 2 {
					val = vals[p.idParcels[0]]
					if vals[p.idParcels[1]] > vals[p.idParcels[0]] {
						val = vals[p.idParcels[1]]
					}
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]]
					ok = true
				}
			case 222: // P1+P2+P3-P4-P5-P6-P7
				if len(p.idParcels) == 7 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]] - vals[p.idParcels[6]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]]
					ok = true
				}
			case 223: // P1+P2+P3-P4-P5-P6-P7-P8
				if len(p.idParcels) == 8 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]] - vals[p.idParcels[6]] - vals[p.idParcels[7]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]]
					ok = true
				}
			case 224: // P1+P2+P3+P4+P5+P6-P7
				if len(p.idParcels) == 7 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] + vals[p.idParcels[3]] + vals[p.idParcels[4]] + vals[p.idParcels[5]] - vals[p.idParcels[6]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]]
					ok = true
				}
			case 225: // VBD+0*P1
				if len(p.idParcels) <= 1 {
					val = 1
					invalid = false
					ok = true
				}
			case 226: // P1+P2-P3-P4-P5-P6-P7-P8
				if len(p.idParcels) == 8 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] - vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]] - vals[p.idParcels[6]] - vals[p.idParcels[7]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]]
					ok = true
				}
			case 227: // P1+P2+P3-P4-P5
				if len(p.idParcels) == 5 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]]
					ok = true
				}
			case 228: // P1+P2+P3-P4-P5-P6-P7-P8-P9-P10-P11-P12
				if len(p.idParcels) == 12 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]] - vals[p.idParcels[6]] - vals[p.idParcels[7]] - vals[p.idParcels[8]] - vals[p.idParcels[9]] - vals[p.idParcels[10]] - vals[p.idParcels[11]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]] || invalids[p.idParcels[8]] || invalids[p.idParcels[9]] || invalids[p.idParcels[10]] || invalids[p.idParcels[11]]
					ok = true
				}
			case 229: // P1+P2+P3+P4+P5+P6+P7+P8+P9-P10-P11-P12
				if len(p.idParcels) == 12 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] + vals[p.idParcels[3]] + vals[p.idParcels[4]] + vals[p.idParcels[5]] + vals[p.idParcels[6]] + vals[p.idParcels[7]] + vals[p.idParcels[8]] - vals[p.idParcels[9]] - vals[p.idParcels[10]] - vals[p.idParcels[11]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]] || invalids[p.idParcels[8]] || invalids[p.idParcels[9]] || invalids[p.idParcels[10]] || invalids[p.idParcels[11]]
					ok = true
				}
			case 230: // P1+P2+P3+P4+P5+P6+P7+P8+P9-P10
				if len(p.idParcels) == 10 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] + vals[p.idParcels[3]] + vals[p.idParcels[4]] + vals[p.idParcels[5]] + vals[p.idParcels[6]] + vals[p.idParcels[7]] + vals[p.idParcels[8]] - vals[p.idParcels[9]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]] || invalids[p.idParcels[8]] || invalids[p.idParcels[9]]
					ok = true
				}
			case 231: // IF P1 > 0 THEN 0 ELSE P1
				if len(p.idParcels) == 1 {
					val = vals[p.idParcels[0]]
					if vals[p.idParcels[0]] > 0 {
						val = 0
					}
					invalid = invalids[p.idParcels[0]]
					ok = true
				}
			case 232: // P1+P2+P3+P4+P5-P6-P7-P8
				if len(p.idParcels) == 8 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] + vals[p.idParcels[3]] + vals[p.idParcels[4]] - vals[p.idParcels[5]] - vals[p.idParcels[6]] - vals[p.idParcels[7]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]]
					ok = true
				}
			case 233: // P1+P2+P3+P4+P5+P6+P7-P8-P9-P10-P11
				if len(p.idParcels) == 11 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] + vals[p.idParcels[3]] + vals[p.idParcels[4]] + vals[p.idParcels[5]] + vals[p.idParcels[6]] - vals[p.idParcels[7]] - vals[p.idParcels[8]] - vals[p.idParcels[9]] - vals[p.idParcels[10]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]] || invalids[p.idParcels[8]] || invalids[p.idParcels[9]] || invalids[p.idParcels[10]]
					ok = true
				}
			case 500: // P1 | P2 | PN
				invalid = false
				val = 0
				for _, elem := range p.idParcels {
					if vals[elem] != 0 {
						val = 1
					}
					invalid = invalid || invalids[elem]
				}
				ok = true
			case 680: // P1+P2+P3-P4-P5
				if len(p.idParcels) == 5 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]]
					ok = true
				}
			case 5000: // P1+P2+P3-P4-0.60*P5+P6
				if len(p.idParcels) == 6 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] - vals[p.idParcels[3]] - 0.6*vals[p.idParcels[4]] + vals[p.idParcels[5]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]]
					ok = true
				}
			case 5022: // P1+(0.65*P2)
				if len(p.idParcels) == 2 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]]
					ok = true
				}
			case 5041: // P1+P2-P3+P4-P5
				if len(p.idParcels) == 5 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] - vals[p.idParcels[2]] + vals[p.idParcels[3]] - vals[p.idParcels[4]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]]
					ok = true
				}
			case 5043: // P1+P2+P3+P4-P5-P6+P7-P8-P9
				if len(p.idParcels) == 9 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] + vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]] + vals[p.idParcels[6]] - vals[p.idParcels[7]] - vals[p.idParcels[8]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]] || invalids[p.idParcels[8]]
					ok = true
				}
			case 5172: // -0.72*P1
				if len(p.idParcels) == 1 {
					val = -0.72 * vals[p.idParcels[0]]
					invalid = invalids[p.idParcels[0]]
					ok = true
				}
			case 5173: // 0.38*P1
				if len(p.idParcels) == 1 {
					val = 0.38 * vals[p.idParcels[0]]
					invalid = invalids[p.idParcels[0]]
					ok = true
				}
			case 5174: // 0.14*P1
				if len(p.idParcels) == 1 {
					val = 0.14 * vals[p.idParcels[0]]
					invalid = invalids[p.idParcels[0]]
					ok = true
				}
			case 5406: // -P1+P2+P3+P4+P5+P6-P7-P8+P9+P10+P11
				if len(p.idParcels) == 11 {
					val = -vals[p.idParcels[0]] + vals[p.idParcels[1]] + vals[p.idParcels[2]] + vals[p.idParcels[3]] + vals[p.idParcels[4]] + vals[p.idParcels[5]] - vals[p.idParcels[6]] - vals[p.idParcels[7]] + vals[p.idParcels[8]] + vals[p.idParcels[9]] + vals[p.idParcels[10]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]] || invalids[p.idParcels[8]] || invalids[p.idParcels[9]] || invalids[p.idParcels[10]]
					ok = true
				}
			case 5711: // -P1-P2-P3-P4
				if len(p.idParcels) == 4 {
					val = -vals[p.idParcels[0]] - vals[p.idParcels[1]] - vals[p.idParcels[2]] - vals[p.idParcels[3]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]]
					ok = true
				}
			case 7374: // P1-P2-P3-P4-P5-P6-P7-P8-P9-P10-P11-P12
				if len(p.idParcels) == 12 {
					val = vals[p.idParcels[0]] - vals[p.idParcels[1]] - vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]] - vals[p.idParcels[6]] - vals[p.idParcels[7]] - vals[p.idParcels[8]] - vals[p.idParcels[9]] - vals[p.idParcels[10]] - vals[p.idParcels[11]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]] || invalids[p.idParcels[8]] || invalids[p.idParcels[9]] || invalids[p.idParcels[10]] || invalids[p.idParcels[11]]
					ok = true
				}
			case 8055: // P1+P2-P3-P4-P5-P6-P7-P8
				if len(p.idParcels) == 8 {
					val = vals[p.idParcels[0]] + vals[p.idParcels[1]] - vals[p.idParcels[2]] - vals[p.idParcels[3]] - vals[p.idParcels[4]] - vals[p.idParcels[5]] - vals[p.idParcels[6]] - vals[p.idParcels[7]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]] || invalids[p.idParcels[2]] || invalids[p.idParcels[3]] || invalids[p.idParcels[4]] || invalids[p.idParcels[5]] || invalids[p.idParcels[6]] || invalids[p.idParcels[7]]
					ok = true
				}
			case 25673: // P1-P2
				if len(p.idParcels) == 2 {
					val = vals[p.idParcels[0]] - vals[p.idParcels[1]]
					invalid = invalids[p.idParcels[0]] || invalids[p.idParcels[1]]
					ok = true
				}
			}

			if logLevel > 2 {
				var chg string
				if val == vals[id] && invalid == invalids[id] {
					chg = "NOT_CHANGED"
				}
				log.Printf("Key %d Parcels %+v Result %f Invalid %v %s", id, p.idParcels, val, invalid, chg)
			}

			// accumulates updates for changed data
			if ok && (val != vals[id] || invalid != invalids[id]) {
				oper := mongo.NewUpdateOneModel()
				oper.Filter = bson.D{
					{"_id", id},
				}
				oper.Update = bson.D{{
					"$set", bson.D{{
						"sourceDataUpdate",
						bson.D{
							{"valueAtSource", val},
							{"invalidAtSource", invalid},
							{"transientAtSource", transient},
							{"timeTag", time.Now()},
						},
					}},
				}}
				opers = append(opers, oper)
			}
		}

		// bulb write the update operations to the MongoDB server
		if len(opers) > 0 {
			res, err := collection.BulkWrite(
				context.Background(),
				opers,
			)
			if res == nil {
				log.Print("bulk")
				log.Fatal(err)
			}
			log.Printf("Count %d Elapsed %s\n", res.MatchedCount, time.Since(tbegin))
		}

		// wait for calculation time period to end
		for time.Until(after) > 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
}
