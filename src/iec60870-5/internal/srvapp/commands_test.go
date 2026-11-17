/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package srvapp

import (
	"context"
	"testing"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"iec60870-5/internal/conv"
	"iec60870-5/internal/model"

	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
)

// cmdEngine returns an engine on a fresh database serving connection 1.
func cmdEngine(t *testing.T, docs ...bson.M) (*Engine, *Conn, *mongo.Database) {
	t.Helper()
	db := testDB(t)
	if len(docs) > 0 {
		in := make([]any, len(docs))
		for i, d := range docs {
			in[i] = d
		}
		if _, err := db.Collection(jsmongo.RealtimeDataCollectionName).InsertMany(context.Background(), in); err != nil {
			t.Fatal(err)
		}
	}
	c := NewConn(model.ConnCfg{Name: "T", ProtocolConnectionNumber: 1})
	e := &Engine{DriverName: "IEC60870-5-104_SERVER", Conns: []*Conn{c}, db: db}
	e.IsMongoLive.Store(true)
	return e, c, db
}

func dest(conn, ca, ioa, typ int, extra bson.M) bson.M {
	d := bson.M{
		"protocolDestinationConnectionNumber": float64(conn),
		"protocolDestinationCommonAddress":    float64(ca),
		"protocolDestinationObjectAddress":    float64(ioa),
		"protocolDestinationASDU":             float64(typ),
	}
	for k, v := range extra {
		d[k] = v
	}
	return d
}

func point(id int, ptype string, value float64, dests ...bson.M) bson.M {
	a := bson.A{}
	for _, d := range dests {
		a = append(a, d)
	}
	return bson.M{
		"_id": float64(id), "tag": "P" + jsmongo.FormatID(float64(id)), "type": ptype, "origin": "supervised",
		"value": value, "protocolSourceConnectionNumber": 50.0, "protocolSourceCommonAddress": 3.0,
		"protocolSourceObjectAddress": float64(1000 + id), "protocolSourceASDU": 45.0,
		"protocolDestinations": a,
	}
}

// Connection, CA, IOA and type must match on the same destination entry.
func TestFindMappedPointSameEntry(t *testing.T) {
	e, c, _ := cmdEngine(t,
		point(1, "digital", 0, dest(1, 1, 100, 45, nil), dest(2, 1, 200, 45, nil)),
		point(2, "digital", 0, dest(1, 1, 200, 45, nil)),
		point(3, "digital", 0, dest(1, 1, 300, 45, nil), dest(1, 1, 301, 46, nil)),
	)
	if p, d := e.findMappedPoint(c, 1, 200, 45); p == nil || p.ID != 2 || int(d.ObjectAddress) != 200 {
		t.Errorf("IOA 200: got point %v", p)
	}
	if p, d := e.findMappedPoint(c, 1, 100, 0); p == nil || p.ID != 1 || int(d.ObjectAddress) != 100 {
		t.Errorf("IOA 100: got point %v", p)
	}
	if p, d := e.findMappedPoint(c, 1, 301, 46); p == nil || p.ID != 3 || int(d.ASDU) != 46 || int(d.ObjectAddress) != 301 {
		t.Errorf("IOA 301: got point %v dst %v", p, d)
	}
	if p, _ := e.findMappedPoint(c, 1, 300, 46); p != nil {
		t.Errorf("IOA 300 type 46 matched point %v", p.ID)
	}
}

func singleCmd(t *testing.T, ioa int, sel bool, typ asdu.TypeID, tm time.Time) *asdu.ASDU {
	return buildASDU(t, func(l asdu.Connect) error {
		return asdu.SingleCmd(l, typ, asdu.CauseOfTransmission{Cause: asdu.Activation}, 1,
			asdu.SingleCommandInfo{Ioa: asdu.InfoObjAddr(ioa), Value: true, Time: tm,
				Qoc: asdu.QualifierOfCommand{InSelect: sel}})
	})
}

// confirmations returns the activation confirmations sent on a link, as
// true for positive.
func confirmations(l *fakeLink) []bool {
	var out []bool
	for _, a := range l.all() {
		if a.Coa.Cause == asdu.ActivationCon {
			out = append(out, !a.Coa.IsNegative)
		}
	}
	return out
}

func queued(t *testing.T, db *mongo.Database) []bson.M {
	t.Helper()
	cur, err := db.Collection(jsmongo.CommandsQueueCollectionName).Find(context.Background(), bson.M{})
	if err != nil {
		t.Fatal(err)
	}
	var out []bson.M
	if err := cur.All(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCommandSelectBeforeOperate(t *testing.T) {
	e, c, db := cmdEngine(t, point(10, "digital", 0,
		dest(1, 1, 500, 45, bson.M{"protocolDestinationCommandUseSBO": true})))

	// execute without select: one negative confirmation only
	l1 := &fakeLink{}
	e.HandleCommandASDU(c, l1, singleCmd(t, 500, false, asdu.C_SC_NA_1, time.Time{}))
	if got := confirmations(l1); len(got) != 1 || got[0] {
		t.Fatalf("execute without select: confirmations %v, want [false]", got)
	}

	// select by one master does not authorize another master's execute
	l2 := &fakeLink{}
	e.HandleCommandASDU(c, l1, singleCmd(t, 500, true, asdu.C_SC_NA_1, time.Time{}))
	e.HandleCommandASDU(c, l2, singleCmd(t, 500, false, asdu.C_SC_NA_1, time.Time{}))
	if got := confirmations(l2); len(got) != 1 || got[0] {
		t.Fatalf("execute from another link: confirmations %v, want [false]", got)
	}
	if n := len(queued(t, db)); n != 0 {
		t.Fatalf("%d commands queued, want 0", n)
	}

	// select then execute on the same link
	l3 := &fakeLink{}
	e.HandleCommandASDU(c, l3, singleCmd(t, 500, true, asdu.C_SC_NA_1, time.Time{}))
	e.HandleCommandASDU(c, l3, singleCmd(t, 500, false, asdu.C_SC_NA_1, time.Time{}))
	if got := confirmations(l3); len(got) != 2 || !got[0] || !got[1] {
		t.Fatalf("select+execute: confirmations %v, want [true true]", got)
	}
	q := queued(t, db)
	if len(q) != 1 || jsmongo.GetDouble(q[0], "value", -1) != 1 ||
		jsmongo.GetDouble(q[0], "protocolSourceObjectAddress", 0) != 1010 {
		t.Fatalf("queued %v", q)
	}

	// the selection is consumed by the execute
	e.HandleCommandASDU(c, l3, singleCmd(t, 500, false, asdu.C_SC_NA_1, time.Time{}))
	if got := confirmations(l3); len(got) != 3 || got[2] {
		t.Fatalf("second execute: confirmations %v, want a negative third", got)
	}
}

func TestTimeTaggedCommandWindow(t *testing.T) {
	e, c, db := cmdEngine(t, point(11, "digital", 0, dest(1, 1, 510, 58, nil)))
	l := &fakeLink{}
	e.HandleCommandASDU(c, l, singleCmd(t, 510, false, asdu.C_SC_TA_1, time.Now().Add(time.Hour)))
	e.HandleCommandASDU(c, l, singleCmd(t, 510, false, asdu.C_SC_TA_1, time.Now().Add(-time.Hour)))
	e.HandleCommandASDU(c, l, singleCmd(t, 510, false, asdu.C_SC_TA_1, time.Now()))
	if got := confirmations(l); len(got) != 3 || got[0] || got[1] || !got[2] {
		t.Fatalf("confirmations %v, want [false false true]", got)
	}
	if n := len(queued(t, db)); n != 1 {
		t.Fatalf("%d commands queued, want 1", n)
	}
}

// values sent on a link, by IOA
func sentValues(l *fakeLink) map[int]float64 {
	out := map[int]float64{}
	for _, a := range l.all() {
		for _, v := range conv.Decode(a, 1).Values {
			out[v.Address] = v.Value
		}
	}
	return out
}

// Interrogation, read replies and spontaneous data apply the destination
// conversion the same way.
func TestDestinationConversionConsistent(t *testing.T) {
	p := point(20, "analog", 10, dest(1, 1, 600, 13, bson.M{
		"protocolDestinationKConv1": 2.0, "protocolDestinationKConv2": 1.0}))
	e, c, db := cmdEngine(t, p)

	gi := &fakeLink{}
	e.Interrogation(c, gi, buildASDU(t, func(l asdu.Connect) error {
		return asdu.InterrogationCmd(l, asdu.CauseOfTransmission{Cause: asdu.Activation}, 1, asdu.QOIStation)
	}), asdu.QOIStation)
	if v := sentValues(gi)[600]; v != 21 {
		t.Errorf("interrogation value %v, want 21", v)
	}

	rd := &fakeLink{}
	e.HandleRead(c, rd, buildASDU(t, func(l asdu.Connect) error {
		return asdu.ReadCmd(l, asdu.CauseOfTransmission{Cause: asdu.Request}, 1, 600)
	}), 600)
	if v := sentValues(rd)[600]; v != 21 {
		t.Errorf("read value %v, want 21", v)
	}

	var rt model.RtDataPoint
	if err := db.Collection(jsmongo.RealtimeDataCollectionName).
		FindOne(context.Background(), bson.M{"_id": 20.0}).Decode(&rt); err != nil {
		t.Fatal(err)
	}
	e.processPointUpdate(&rt)
	b := c.outQ.popBatch(30)
	if len(b) != 1 || b[0].obj.Info.(asdu.MeasuredValueFloatInfo).Value != 21 {
		t.Errorf("spontaneous batch %v, want one value 21", b)
	}
}

// Group membership is taken from the destination of the interrogated
// connection, and every destination of a point on that connection is sent.
func TestInterrogationDestinations(t *testing.T) {
	e, c, _ := cmdEngine(t,
		// group 1 only on another connection
		point(30, "analog", 1, dest(2, 1, 700, 13, bson.M{"protocolDestinationGroup": 1.0}),
			dest(1, 1, 700, 13, bson.M{"protocolDestinationGroup": 0.0})),
		// two addresses on this connection, one in group 1
		point(31, "analog", 2, dest(1, 1, 801, 13, bson.M{"protocolDestinationGroup": 1.0}),
			dest(1, 1, 802, 13, nil)),
	)
	gi := func(q asdu.QualifierOfInterrogation) map[int]float64 {
		l := &fakeLink{}
		e.Interrogation(c, l, buildASDU(t, func(l asdu.Connect) error {
			return asdu.InterrogationCmd(l, asdu.CauseOfTransmission{Cause: asdu.Activation}, 1, q)
		}), q)
		return sentValues(l)
	}
	all := gi(asdu.QOIStation)
	if len(all) != 3 || all[700] != 1 || all[801] != 2 || all[802] != 2 {
		t.Errorf("station interrogation sent %v", all)
	}
	g1 := gi(asdu.QOIGroup1)
	if len(g1) != 1 || g1[801] != 2 {
		t.Errorf("group 1 interrogation sent %v, want only IOA 801", g1)
	}
}
