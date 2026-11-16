/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package cliapp

import (
	"context"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"iec60870-5/internal/conv"
	"iec60870-5/internal/model"
	"iec60870-5/internal/testmongo"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
)

func acValue(ioa int) conv.IecValue {
	return conv.IecValue{Address: ioa, Asdu: asdu.M_ME_NC_1, Value: 1, ConnNumber: 1, CommonAddress: 1}
}

// A tag that could not be inserted is created on the next value for its
// address; a tag whose name exists already counts as created.
func TestAutoCreateRetriesFailedInsert(t *testing.T) {
	jslog.SetLevel(0)
	_, db := testmongo.DB(t, "json_scada_cliautotag_test")
	ctx := context.Background()
	name := jsmongo.RealtimeDataCollectionName
	if err := db.CreateCollection(ctx, name, options.CreateCollection().
		SetValidator(bson.M{"tag": bson.M{"$ne": "AC;1;100"}})); err != nil {
		t.Fatal(err)
	}
	col := db.Collection(name)
	if _, err := col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.M{"tag": 1}, Options: options.Index().SetUnique(true)}); err != nil {
		t.Fatal(err)
	}
	if _, err := col.InsertOne(ctx, bson.M{"_id": 5.0, "tag": "AC;1;200"}); err != nil {
		t.Fatal(err)
	}

	e := New(jsconfig.Config{}, "IEC60870-5-104", 1, func() bool { return true })
	c := &Conn{Cfg: model.ConnCfg{Name: "AC", ProtocolConnectionNumber: 1, AutoCreateTags: true},
		inserted: map[[2]int]bool{}}
	e.Conns = []*Conn{c}

	_, ins := e.buildWrites(col, []conv.IecValue{acValue(100), acValue(200)})
	if len(ins) != 2 || !e.insertTags(col, ins) {
		t.Fatalf("inserts %d", len(ins))
	}
	if c.inserted[[2]int{1, 100}] {
		t.Error("rejected tag still marked as inserted")
	}
	if !c.inserted[[2]int{1, 200}] {
		t.Error("existing tag name not treated as created")
	}

	if err := db.RunCommand(ctx, bson.D{{Key: "collMod", Value: name}, {Key: "validator", Value: bson.M{}}}).Err(); err != nil {
		t.Fatal(err)
	}
	_, ins = e.buildWrites(col, []conv.IecValue{acValue(100), acValue(200)})
	if len(ins) != 1 || !e.insertTags(col, ins) {
		t.Fatalf("retry inserts %d, want 1", len(ins))
	}
	if n, _ := col.CountDocuments(ctx, bson.M{"tag": "AC;1;100"}); n != 1 {
		t.Errorf("tag AC;1;100 count %d, want 1", n)
	}
}
