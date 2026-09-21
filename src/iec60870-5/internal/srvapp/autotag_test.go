/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - autotag tests
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Exercises DistributeAutoTags against a real MongoDB: the bundled mongod of
 * platform-windows/platform-linux is started on a temporary dbpath, or the
 * server given by JS_TEST_MONGO_URI is used. The test skips when neither is
 * available.
 */

package srvapp

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"iec60870-5/internal/model"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
)

// mongodPath returns the bundled mongod binary, or "" when not present.
func mongodPath() string {
	if p := os.Getenv("JS_TEST_MONGOD"); p != "" {
		return p
	}
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	root := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "..")
	cands := []string{
		filepath.Join(root, "platform-windows", "mongodb-runtime", "bin", "mongod.exe"),
		filepath.Join(root, "platform-linux", "mongodb-runtime", "bin", "mongod"),
	}
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %s", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// testDB connects to a temporary MongoDB, starting the bundled mongod when
// JS_TEST_MONGO_URI is not set. Skips the test when neither is available.
func testDB(t *testing.T) *mongo.Database {
	t.Helper()
	uri := os.Getenv("JS_TEST_MONGO_URI")
	if uri == "" {
		bin := mongodPath()
		if bin == "" {
			t.Skip("no mongod available (set JS_TEST_MONGOD or JS_TEST_MONGO_URI)")
		}
		port := freePort(t)
		dbpath := t.TempDir()
		cmd := exec.Command(bin, "--dbpath", dbpath, "--port", strconv.Itoa(port),
			"--bind_ip", "127.0.0.1")
		if err := cmd.Start(); err != nil {
			t.Skipf("cannot start mongod: %s", err)
		}
		t.Cleanup(func() {
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		})
		uri = "mongodb://127.0.0.1:" + strconv.Itoa(port)
	}

	var client *mongo.Client
	var lastErr error
	deadline := time.Now().Add(60 * time.Second)
	for {
		c, err := mongo.Connect(options.Client().ApplyURI(uri).
			SetServerSelectionTimeout(2 * time.Second))
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err = c.Ping(ctx, nil)
			cancel()
			if err == nil {
				client = c
				break
			}
			_ = c.Disconnect(context.Background())
		}
		lastErr = err
		if time.Now().After(deadline) {
			t.Skipf("MongoDB not reachable at %s: %v", uri, lastErr)
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	db := client.Database("json_scada_autotag_test")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = db.Drop(ctx)
	return db
}

// seedTag builds one realtimeData document; an empty origin omits the field.
func seedTag(id int, tag, ptype, origin, group1 string) bson.M {
	d := bson.M{
		"_id":    float64(id),
		"tag":    tag,
		"type":   ptype,
		"group1": group1,
		"value":  0.0,
	}
	if origin != "" {
		d["origin"] = origin
	}
	return d
}

func newTestConn(num int, topics []string) *Conn {
	return &Conn{Cfg: model.ConnCfg{
		Name:                     "TESTSRV",
		ProtocolConnectionNumber: float64(num),
		AutoCreateTags:           true,
		SizeOfIOA:                3,
		LocalLinkAddress:         7,
		Topics:                   topics,
	}}
}

// destsFor returns the destinations of a tag for the given connection.
func destsFor(t *testing.T, db *mongo.Database, id int, connNumber float64) []model.ProtocolDestination {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var point model.RtDataPoint
	err := db.Collection(jsmongo.RealtimeDataCollectionName).
		FindOne(ctx, bson.M{"_id": float64(id)}).Decode(&point)
	if err != nil {
		t.Fatalf("tag %d not found: %s", id, err)
	}
	var out []model.ProtocolDestination
	for _, d := range point.ProtocolDestinations {
		if d.ConnectionNumber == connNumber {
			out = append(out, d)
		}
	}
	return out
}

// TestDistributeAutoTagsCompleteDatabase checks that with an empty topics
// array every digital/analog tag gets a destination, whatever its origin
// (supervised, calculated, manual, missing), and that command tags get the
// control ASDUs. Tags of unsupported types and pre-existing destinations are
// left alone.
func TestDistributeAutoTagsCompleteDatabase(t *testing.T) {
	jslog.SetLevel(0)
	db := testDB(t)
	colRt := db.Collection(jsmongo.RealtimeDataCollectionName)

	docs := []interface{}{
		seedTag(1, "DIG_SUP", "digital", "supervised", "KAW2"),
		seedTag(2, "ANA_SUP", "analog", "supervised", "KAW2"),
		seedTag(3, "DIG_CALC", "digital", "calculated", "OTHER"),
		seedTag(4, "ANA_CALC", "analog", "calculated", "OTHER"),
		seedTag(5, "DIG_MAN", "digital", "manual", "OTHER"),
		seedTag(6, "ANA_MAN", "analog", "manual", "OTHER"),
		seedTag(7, "DIG_CMD", "digital", "command", "KAW2"),
		seedTag(8, "ANA_CMD", "analog", "command", "KAW2"),
		seedTag(9, "DIG_NOORIGIN", "digital", "", "OTHER"),
		seedTag(10, "STR_SUP", "string", "supervised", "OTHER"),
	}
	// tag 11 is already mapped to the connection: it must be preserved and it
	// must push the next allocated digital IOA past it
	mapped := seedTag(11, "DIG_MAPPED", "digital", "supervised", "KAW2")
	mapped["protocolDestinations"] = bson.A{bson.M{
		"protocolDestinationConnectionNumber": 1.0,
		"protocolDestinationCommonAddress":    7.0,
		"protocolDestinationObjectAddress":    100.0,
		"protocolDestinationASDU":             1.0,
	}}
	docs = append(docs, mapped)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	_, err := colRt.InsertMany(ctx, docs)
	cancel()
	if err != nil {
		t.Fatalf("seed: %s", err)
	}

	e := &Engine{DriverName: "IEC60870-5-104_SERVER", Conns: []*Conn{newTestConn(1, nil)}}
	e.db = db
	e.DistributeAutoTags()

	cases := []struct {
		id    int
		asdu  model.U32
		ioaLo model.U32
		ioaHi model.U32
	}{
		{1, 1, ioaBaseDigital, ioaTopDigital},
		{2, 13, ioaBaseAnalog, ioaTopAnalog},
		{3, 1, ioaBaseDigital, ioaTopDigital},
		{4, 13, ioaBaseAnalog, ioaTopAnalog},
		{5, 1, ioaBaseDigital, ioaTopDigital},
		{6, 13, ioaBaseAnalog, ioaTopAnalog},
		{7, 45, ioaBaseDigCmd, ioaTopDigCmd},
		{8, 50, ioaBaseAnaCmd, ioaTopAnaCmd},
		{9, 1, ioaBaseDigital, ioaTopDigital},
	}
	seenIoa := map[model.U32]int{}
	for _, c := range cases {
		ds := destsFor(t, db, c.id, 1)
		if len(ds) != 1 {
			t.Errorf("tag %d: got %d destinations, want 1", c.id, len(ds))
			continue
		}
		d := ds[0]
		if d.ASDU != c.asdu {
			t.Errorf("tag %d: ASDU %d, want %d", c.id, d.ASDU, c.asdu)
		}
		if d.ObjectAddress < c.ioaLo || d.ObjectAddress > c.ioaHi {
			t.Errorf("tag %d: IOA %d outside range %d-%d", c.id, d.ObjectAddress, c.ioaLo, c.ioaHi)
		}
		if d.CommonAddress != 7 {
			t.Errorf("tag %d: CA %d, want 7 (localLinkAddress)", c.id, d.CommonAddress)
		}
		if prev, dup := seenIoa[d.ObjectAddress]; dup {
			t.Errorf("tag %d: IOA %d already used by tag %d", c.id, d.ObjectAddress, prev)
		}
		seenIoa[d.ObjectAddress] = c.id
	}

	// unsupported type: no destination created
	if ds := destsFor(t, db, 10, 1); len(ds) != 0 {
		t.Errorf("string tag: got %d destinations, want 0", len(ds))
	}

	// pre-existing destination preserved, new digital IOAs allocated after it
	ds := destsFor(t, db, 11, 1)
	if len(ds) != 1 || ds[0].ObjectAddress != 100 {
		t.Errorf("mapped tag: got %v, want the single pre-existing IOA 100", ds)
	}
	for ioa, id := range seenIoa {
		if ioa <= ioaTopDigital && ioa <= 100 {
			t.Errorf("tag %d got digital IOA %d, expected allocation above the existing 100", id, ioa)
		}
	}
}

// TestDistributeAutoTagsTopicsFilter checks that a non-empty topics array
// still restricts the distribution to matching group1 tags.
func TestDistributeAutoTagsTopicsFilter(t *testing.T) {
	jslog.SetLevel(0)
	db := testDB(t)
	colRt := db.Collection(jsmongo.RealtimeDataCollectionName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	_, err := colRt.InsertMany(ctx, []interface{}{
		seedTag(1, "DIG_KAW2", "digital", "supervised", "KAW2"),
		seedTag(2, "DIG_CALC_KAW2", "digital", "calculated", "KAW2-CALC"),
		seedTag(3, "DIG_OTHER", "digital", "supervised", "OTHER"),
	})
	cancel()
	if err != nil {
		t.Fatalf("seed: %s", err)
	}

	e := &Engine{DriverName: "IEC60870-5-104_SERVER", Conns: []*Conn{newTestConn(1, []string{"KAW2"})}}
	e.db = db
	e.DistributeAutoTags()

	if n := len(destsFor(t, db, 1, 1)); n != 1 {
		t.Errorf("KAW2 supervised tag: got %d destinations, want 1", n)
	}
	if n := len(destsFor(t, db, 2, 1)); n != 1 {
		t.Errorf("KAW2 calculated tag: got %d destinations, want 1", n)
	}
	if n := len(destsFor(t, db, 3, 1)); n != 0 {
		t.Errorf("non-matching tag: got %d destinations, want 0", n)
	}
}
