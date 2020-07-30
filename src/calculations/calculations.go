// This process calculates point values based of predefined formulas and configured parcels.
// All data is read from and results are written to the MongoDB server.
// {json:scada} - Copyright 2020 - Ricardo L. Olsen

package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var configFileName string = "json-scada.json"
var realtimeDataConnectionName string = "realtimeData"
var defaultPeriodOfCalculation float64 = 2.0

type config struct {
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

// A Simple function to verify error
func checkError(err error) {
	if err != nil {
		log.Println("Error: ", err)
		os.Exit(0)
	}
}

// Reads the config file and connects to MongoDB server
func mongoConnect() (client *mongo.Client, colRTD *mongo.Collection, err error) {

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	// tries to open and read the json config file
	jsonFile, err := os.Open(filepath.Join("..", "conf", configFileName))
	if err != nil {
		log.Printf("Fail to read file: %v", err)
		os.Exit(1)
	}
	byteValue, _ := ioutil.ReadAll(jsonFile)

	// unmarshals the json file's content into a config structure
	var cfg config
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
		return client, colRTD, err
	}
	err = client.Connect(ctx)
	if err != nil {
		return client, colRTD, err
	}
	colRTD = client.Database(cfg.MongoDatabaseName).Collection(realtimeDataConnectionName)

	return client, colRTD, err
}

func main() {

	client, collection, err := mongoConnect()
	if err != nil {
		log.Fatal(err)
	}

	var calcs map[int]*pointCalc
	calcs = make(map[int]*pointCalc)

	var nponto int

	projection := bson.D{
		{"_id", 1},
		{"formula", 1},
		{"parcels", 1},
	}
	cur, err := collection.Find(context.Background(),
		bson.D{
			{"formula", bson.D{
				{"$type", 1},
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
			log.Printf("%d %d\n", nponto, parcel)
		}
	}

	// maps for values and flags of all parcels and calculated points
	var vals map[int]float64
	vals = make(map[int]float64)
	var invalids map[int]bool
	invalids = make(map[int]bool)

	// creates the find array for all parcels
	barr := bson.A{}
	for pointnum, p := range calcs {
		barr = append(barr, pointnum)
		for _, idparc := range p.idParcels {
			barr = append(barr, idparc)
		}
	}

	projection = bson.D{
		{"_id", 1},
		{"value", 1},
		{"invalid", 1},
	}
	for {
		tbegin := time.Now()
		after := tbegin.Add(time.Duration(defaultPeriodOfCalculation) * time.Second)

		// Check the connection
		errp := client.Ping(context.TODO(), nil)
		if errp != nil {
			log.Printf("%s \n", err)
			client.Disconnect(context.TODO())
			client, collection, errp = mongoConnect()
		}

		// find all parcel and current calculated values
		cur, err := collection.Find(context.Background(),
			bson.D{
				{"_id", bson.D{
					{"$in", barr},
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
					if invalids[elem] == false {
						val = vals[elem]
						invalid = false
					}
				}
				ok = true
			case 52: // Any ok? (1 if any parcel is ok)
				invalid = false
				val = 0
				for _, elem := range p.idParcels {
					if invalids[elem] == false {
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
			log.Printf("Count %d Elapsed %s\n", res.MatchedCount, time.Now().Sub(tbegin))
		}

		// wait for calculation time period to end
		for after.Sub(time.Now()) > 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}
}
