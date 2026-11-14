/*
 * This process calculates point values based of predefined formulas and configured parcels.
 * All data is read from and results are written to the MongoDB server.
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * Formulas are defined in formulas.go.
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
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const softwareVersion string = "0.2.0"
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

func (p *pointCalc) equal(o *pointCalc) bool {
	if p.calc != o.calc || len(p.idParcels) != len(o.idParcels) {
		return false
	}
	for i := range p.idParcels {
		if p.idParcels[i] != o.idParcels[i] {
			return false
		}
	}
	return true
}

// output last written for a calculated point
type sourceDataUpdate struct {
	VALUE     float64 `bson:"valueAtSource"`
	INVALID   bool    `bson:"invalidAtSource"`
	TRANSIENT bool    `bson:"transientAtSource"`
}

type realtimeData struct {
	ID      int               `bson:"_id"`
	VALUE   float64           `bson:"value"`
	INVALID bool              `bson:"invalid"`
	SDU     *sourceDataUpdate `bson:"sourceDataUpdate"`
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

	client, err = mongo.Connect(options.Client().ApplyURI(cfg.MongoConnectionString))
	if err != nil {
		mongoClient = nil
		return client, colRTD, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err = client.Ping(ctx, nil)
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
						"activeNodeKeepAliveTimeTag": bson.NewDateTimeFromTime(time.Now()),
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
						bson.M{"$set": bson.M{"activeNodeName": cfg.NodeName, "activeNodeKeepAliveTimeTag": bson.NewDateTimeFromTime(time.Now())}},
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

	calcs, parcelKeys := loadCalcs(collection, nil)
	lastLoad := time.Now()

	projection := bson.D{
		{Key: "_id", Value: 1},
		{Key: "value", Value: 1},
		{Key: "invalid", Value: 1},
		{Key: "sourceDataUpdate.valueAtSource", Value: 1},
		{Key: "sourceDataUpdate.invalidAtSource", Value: 1},
		{Key: "sourceDataUpdate.transientAtSource", Value: 1},
	}
	for {
		if !isActive {
			time.Sleep(1 * time.Second)
			continue
		}
		tbegin := time.Now()
		after := tbegin.Add(time.Duration(periodOfCalculation * float64(time.Second)))

		// Check the connection
		errp := client.Ping(context.TODO(), nil)
		if errp != nil {
			log.Printf("MongoDB ping failed: %s\n", errp)
			client.Disconnect(context.TODO())
			client, collection, _ = mongoConnect(cfg)
		}

		// pick up calculated points created, changed or removed since the last load
		if time.Since(lastLoad) >= reloadPeriod {
			calcs, parcelKeys = loadCalcs(collection, calcs)
			lastLoad = time.Now()
		}

		// find all parcels and current calculated values
		cur, err := collection.Find(context.Background(),
			bson.D{
				{Key: "_id", Value: bson.D{
					{Key: "$in", Value: parcelKeys},
				}},
			},
			options.Find().SetProjection(projection),
		)
		if err != nil {
			log.Print("find")
			log.Fatal(err)
		}

		points := make(map[int]*realtimeData, len(parcelKeys))
		for cur.Next(context.Background()) {
			elem := &realtimeData{}
			err := cur.Decode(elem)
			if err != nil {
				log.Print("decode")
				log.Print(err)
				continue
			}
			points[elem.ID] = elem
		}
		cur.Close(context.Background())
		if logLevel > 0 {
			log.Printf("Read %v points from MongoDB.\n", len(points))
		}

		var opers []mongo.WriteModel

		// loop over all calcs
		for id, p := range calcs {
			// parcels missing from the database count as 0, invalid
			vals := make([]float64, len(p.idParcels))
			invs := make([]bool, len(p.idParcels))
			for i, k := range p.idParcels {
				if pt, ok := points[k]; ok {
					vals[i], invs[i] = pt.VALUE, pt.INVALID
				} else {
					invs[i] = true
				}
			}
			val, invalid, transient, ok := calculate(p.calc, vals, invs)
			if !ok {
				continue // not computable, reported by loadCalcs
			}

			// write only when the output differs from the one last written
			changed := true
			if pt, found := points[id]; found && pt.SDU != nil &&
				pt.SDU.VALUE == val && pt.SDU.INVALID == invalid && pt.SDU.TRANSIENT == transient {
				changed = false
			}
			if logLevel > 2 {
				log.Printf("Key %d Formula %d Parcels %+v Result %f Invalid %v Transient %v Changed %v",
					id, p.calc, p.idParcels, val, invalid, transient, changed)
			}
			if changed {
				oper := mongo.NewUpdateOneModel()
				oper.Filter = bson.D{
					{Key: "_id", Value: id},
				}
				oper.Update = bson.D{{
					Key: "$set", Value: bson.D{{
						Key: "sourceDataUpdate",
						Value: bson.D{
							{Key: "valueAtSource", Value: val},
							{Key: "invalidAtSource", Value: invalid},
							{Key: "transientAtSource", Value: transient},
							{Key: "timeTag", Value: time.Now()},
						},
					}},
				}}
				opers = append(opers, oper)
			}
		}

		// bulk write the update operations to the MongoDB server
		if len(opers) > 0 {
			res, err := collection.BulkWrite(
				context.Background(),
				opers,
			)
			if res == nil {
				log.Print("bulk")
				log.Fatal(err)
			}
			if logLevel > 0 {
				log.Printf("Count %d Elapsed %s\n", res.MatchedCount, time.Since(tbegin))
			}
		}

		// wait for calculation time period to end
		for time.Until(after) > 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
}

// period of reloading the calculated point definitions
const reloadPeriod = 30 * time.Second

// loadCalcs reads the calculated points (origin "calculated", formula > 0) and
// returns them with the keys to read each cycle (calculated points and their
// parcels). Points that cannot be computed are logged and left out. prev is
// the previous result, used to log only changes.
func loadCalcs(collection *mongo.Collection, prev map[int]*pointCalc) (map[int]*pointCalc, bson.A) {
	cur, err := collection.Find(context.Background(),
		bson.D{
			{Key: "formula", Value: bson.D{{Key: "$gt", Value: 0}}},
			{Key: "origin", Value: "calculated"},
		},
		options.Find().SetProjection(bson.D{
			{Key: "_id", Value: 1},
			{Key: "formula", Value: 1},
			{Key: "parcels", Value: 1},
		}),
	)
	if err != nil {
		log.Print("find")
		log.Fatal(err)
	}
	defer cur.Close(context.Background())

	calcs := make(map[int]*pointCalc)
	for cur.Next(context.Background()) {
		elem := &realtimeDataForm{}
		if err := cur.Decode(elem); err != nil {
			log.Printf("Calculated point %v skipped, cannot decode formula/parcels: %v", cur.Current.Lookup("_id"), err)
			continue
		}
		p := &pointCalc{calc: elem.FORMULA, idParcels: elem.PARCELS}
		if old, ok := prev[elem.ID]; ok && old.equal(p) {
			calcs[elem.ID] = old
			continue
		}
		f, found := formulas[elem.FORMULA]
		switch {
		case elem.FORMULA >= userFormulaMin:
			if logLevel > 1 {
				log.Printf("Key %d formula %d is reserved for user processes, ignored", elem.ID, elem.FORMULA)
			}
			continue
		case !found:
			log.Printf("Key %d formula %d is not implemented, point skipped", elem.ID, elem.FORMULA)
			continue
		case !f.parcelsOK(len(elem.PARCELS)):
			log.Printf("Key %d formula %d needs %s parcels, has %d, point skipped",
				elem.ID, elem.FORMULA, f.parcelsText(), len(elem.PARCELS))
			continue
		}
		if logLevel > 1 || prev != nil {
			log.Printf("Key %d formula %d parcels %v loaded", elem.ID, elem.FORMULA, elem.PARCELS)
		}
		calcs[elem.ID] = p
	}
	if prev == nil || len(prev) != len(calcs) {
		log.Printf("%d calculated points loaded", len(calcs))
	}

	keys := bson.A{}
	for id, p := range calcs {
		keys = append(keys, id)
		for _, k := range p.idParcels {
			keys = append(keys, k)
		}
	}
	return calcs, keys
}
