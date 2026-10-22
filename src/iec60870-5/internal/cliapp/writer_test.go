/*
 * IEC 60870-5-101/103/104 protocol drivers for {json:scada} - writer tests
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Measures the latency added by the client MongoDB writer: the time from a
 * decoded value being enqueued to its sourceDataUpdate landing in
 * realtimeData, against a throwaway MongoDB (skips without one).
 */

package cliapp

import (
	"context"
	"math/rand"
	"sort"
	"strconv"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"iec60870-5/internal/conv"
	"iec60870-5/internal/model"
	"iec60870-5/internal/testmongo"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
)

const (
	testConn = 1
	testCA   = 1
	testIOA  = 100
)

func testValue(v float64) conv.IecValue {
	return conv.IecValue{
		Address:         testIOA,
		Asdu:            asdu.M_ME_NC_1,
		Value:           v,
		Cot:             3,
		ServerTimestamp: time.Now(),
		ConnNumber:      testConn,
		CommonAddress:   testCA,
	}
}

// waitValue polls the tag until valueAtSource equals v, returning when it
// landed (or failing after timeout).
func waitValue(t *testing.T, col *mongo.Collection, v float64, timeout time.Duration) time.Time {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		var doc struct {
			SDU struct {
				ValueAtSource *float64 `bson:"valueAtSource"`
			} `bson:"sourceDataUpdate"`
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := col.FindOne(ctx, bson.M{"_id": 1.0}).Decode(&doc)
		cancel()
		if err == nil && doc.SDU.ValueAtSource != nil && *doc.SDU.ValueAtSource == v {
			return time.Now()
		}
		if time.Now().After(deadline) {
			t.Fatalf("value %v not written within %v", v, timeout)
		}
		time.Sleep(200 * time.Microsecond)
	}
}

// TestMongoWriterLatency checks that a single value arriving while the
// writer is idle is written without waiting for a polling interval.
func TestMongoWriterLatency(t *testing.T) {
	jslog.SetLevel(0)
	uri, db := testmongo.DB(t, "json_scada_cliwriter_test")
	col := db.Collection(jsmongo.RealtimeDataCollectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	_, err := col.InsertOne(ctx, bson.M{
		"_id":                            1.0,
		"tag":                            "LATENCY_TEST",
		"type":                           "analog",
		"origin":                         "supervised",
		"protocolSourceConnectionNumber": float64(testConn),
		"protocolSourceCommonAddress":    float64(testCA),
		"protocolSourceObjectAddress":    float64(testIOA),
	})
	cancel()
	if err != nil {
		t.Fatalf("seed: %s", err)
	}

	e := New(jsconfig.Config{MongoConnectionString: uri, MongoDatabaseName: db.Name()},
		"IEC60870-5-104", 1, func() bool { return true })
	e.Conns = []*Conn{{Cfg: model.ConnCfg{Name: "T", ProtocolConnectionNumber: testConn}}}
	go e.RunMongoWriter()

	// first value only proves the writer is connected
	e.EnqueueValues([]conv.IecValue{testValue(-100)})
	waitValue(t, col, -100, 30*time.Second)

	const samples = 40
	rng := rand.New(rand.NewSource(1))
	lat := make([]time.Duration, 0, samples)
	for i := 0; i < samples; i++ {
		// arrivals unsynchronised with any writer cycle, as in the field
		time.Sleep(time.Duration(20+rng.Intn(80)) * time.Millisecond)
		v := float64(i)
		t0 := time.Now()
		e.EnqueueValues([]conv.IecValue{testValue(v)})
		lat = append(lat, waitValue(t, col, v, 10*time.Second).Sub(t0))
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	median, p90, worst := lat[samples/2], lat[samples*9/10], lat[samples-1]
	t.Logf("enqueue -> realtimeData latency: median %v, p90 %v, max %v", median, p90, worst)

	// An idle-polling writer adds about half its poll interval on average;
	// an event-driven one only the database round trip.
	if median > 25*time.Millisecond {
		t.Errorf("median writer latency %v, want <= 25ms", median)
	}
}

// TestMongoWriterBurst checks that a burst is still batched: every value of
// a large burst lands, in far less time than one round trip per value.
func TestMongoWriterBurst(t *testing.T) {
	jslog.SetLevel(0)
	uri, db := testmongo.DB(t, "json_scada_cliwriter_burst_test")
	col := db.Collection(jsmongo.RealtimeDataCollectionName)

	// stay within DataBufferLimit: values beyond it are discarded by design
	const points, rounds = 2000, 4
	docs := make([]interface{}, 0, points)
	for i := 0; i < points; i++ {
		docs = append(docs, bson.M{
			"_id":                            float64(i + 1),
			"tag":                            "BURST_" + strconv.Itoa(i),
			"protocolSourceConnectionNumber": float64(testConn),
			"protocolSourceCommonAddress":    float64(testCA),
			"protocolSourceObjectAddress":    float64(1000 + i),
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	_, err := col.InsertMany(ctx, docs)
	if err == nil {
		_, err = col.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{
			{Key: "protocolSourceConnectionNumber", Value: 1},
			{Key: "protocolSourceCommonAddress", Value: 1},
			{Key: "protocolSourceObjectAddress", Value: 1},
		}})
	}
	cancel()
	if err != nil {
		t.Fatalf("seed: %s", err)
	}

	e := New(jsconfig.Config{MongoConnectionString: uri, MongoDatabaseName: db.Name()},
		"IEC60870-5-104", 1, func() bool { return true })
	e.Conns = []*Conn{{Cfg: model.ConnCfg{Name: "T", ProtocolConnectionNumber: testConn}}}
	go e.RunMongoWriter()

	t0 := time.Now()
	for r := 0; r < rounds; r++ {
		vals := make([]conv.IecValue, 0, points)
		for i := 0; i < points; i++ {
			v := testValue(float64(r))
			v.Address = 1000 + i
			vals = append(vals, v)
		}
		e.EnqueueValues(vals)
	}

	// every point must end with the last round's value
	want := float64(rounds - 1)
	deadline := time.Now().Add(60 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		n, err := col.CountDocuments(ctx, bson.M{"sourceDataUpdate.valueAtSource": want})
		cancel()
		if err == nil && n == points {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("burst not fully written: %d of %d points have the last value", n, points)
		}
		time.Sleep(5 * time.Millisecond)
	}
	elapsed := time.Since(t0)
	t.Logf("%d values written in %v", points*rounds, elapsed)
	// one round trip per value would take about points*rounds milliseconds
	if elapsed > 6*time.Second {
		t.Errorf("burst took %v: values do not seem to be batched", elapsed)
	}
}
