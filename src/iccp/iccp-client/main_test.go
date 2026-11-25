package main

import (
	"testing"
	"time"

	"github.com/riclolsen/tase2/tase2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func testMapping(item string) tagMapping {
	return tagMapping{
		tag: rtData{
			Tag:                         "TEST_" + item,
			ProtocolSourceObjectAddress: "SUB_A/" + item,
			ProtocolSourceASDU:          "realq",
		},
		ref: tase2.ObjectRef{Domain: "SUB_A", Item: item},
	}
}

func TestDataValueToUpdateRealQGood(t *testing.T) {
	now := time.Now()
	dv := tase2.NewRealQ(230.5, tase2.QualityGood)
	upd := dataValueToUpdate(dv, testMapping("ICCP_RealQ_01"), now, 0)

	if upd.value < 230.4 || upd.value > 230.6 {
		t.Errorf("value = %v, want ~230.5", upd.value)
	}
	if upd.invalid || upd.notTopical || upd.substituted {
		t.Errorf("quality flags = inv:%v nt:%v sub:%v, want all false", upd.invalid, upd.notTopical, upd.substituted)
	}
	if upd.timeTagAtSourceOk {
		t.Error("timeTagAtSourceOk = true, want false (no timetag in RealQ)")
	}
}

func TestDataValueToUpdateRealQTimeTagInvalid(t *testing.T) {
	now := time.Now()
	nowUTC := now.UTC()

	q := &tase2.Quality{Validity: tase2.QualityInvalid, Source: tase2.SourceTelemetered}
	dv := tase2.NewRealQTimeTag(42.0, q, tase2.TimeStampFrom(now))
	upd := dataValueToUpdate(dv, testMapping("ICCP_RealQTimeTag_01"), now, 0)

	if upd.value < 41.9 || upd.value > 42.1 {
		t.Errorf("value = %v, want ~42.0", upd.value)
	}
	if !upd.invalid {
		t.Error("invalid = false, want true")
	}
	if !upd.timeTagAtSourceOk {
		t.Fatal("timeTagAtSourceOk = false, want true")
	}
	diff := upd.timeTagAtSource.Sub(nowUTC)
	if diff < -2*time.Second || diff > 2*time.Second {
		t.Errorf("timeTagAtSource = %v, want ~%v (diff %v)", upd.timeTagAtSource, nowUTC, diff)
	}
}

func TestDataValueToUpdateStateQTimeTag(t *testing.T) {
	now := time.Now()
	dv := tase2.NewStateQTimeTag(tase2.StateOn, tase2.QualityGood, 1000)
	upd := dataValueToUpdate(dv, testMapping("ICCP_State_01"), now, 0)

	if upd.value != 1 {
		t.Errorf("value = %v, want 1 (StateOn)", upd.value)
	}
	if upd.invalid {
		t.Error("invalid = true, want false")
	}
	if !upd.timeTagAtSourceOk {
		t.Error("timeTagAtSourceOk = false, want true")
	}
}

func TestDataValueToUpdateSubstituted(t *testing.T) {
	q := &tase2.Quality{Validity: "questionable", Source: "substituted"}
	dv := tase2.NewDiscreteQ(7, q)
	upd := dataValueToUpdate(dv, testMapping("ICCP_DiscreteQ_01"), time.Now(), 0)

	if upd.value != 7 {
		t.Errorf("value = %v, want 7", upd.value)
	}
	if !upd.notTopical {
		t.Error("notTopical = false, want true (questionable)")
	}
	if !upd.substituted {
		t.Error("substituted = false, want true")
	}
}

func TestDataValueToUpdatePlainScalars(t *testing.T) {
	now := time.Now()

	upd := dataValueToUpdate(tase2.NewFloat32Value(1.5), testMapping("Real_01"), now, 0)
	if upd.value != 1.5 {
		t.Errorf("float value = %v, want 1.5", upd.value)
	}

	upd = dataValueToUpdate(tase2.NewBooleanValue(true), testMapping("Bool_01"), now, 0)
	if upd.value != 1 || upd.valueString != "true" {
		t.Errorf("bool value = %v/%q, want 1/\"true\"", upd.value, upd.valueString)
	}

	upd = dataValueToUpdate(nil, testMapping("Nil_01"), now, 0)
	if !upd.invalid {
		t.Error("nil value: invalid = false, want true")
	}
}

func TestDataValueToUpdateQTimeTagExtended(t *testing.T) {
	src := time.Date(2026, 9, 26, 8, 53, 7, 123000000, time.UTC)
	cases := []struct {
		name string
		dv   *tase2.DataValue
	}{
		{"real", tase2.NewRealQTimeTagExtended(1.5, tase2.QualityGood, src)},
		{"discrete", tase2.NewDiscreteQTimeTagExtended(7, tase2.QualityGood, src)},
		{"state", tase2.NewStateQTimeTagExtended(tase2.StateOn, tase2.QualityGood, src)},
	}
	for _, c := range cases {
		upd := dataValueToUpdate(c.dv, testMapping("Ext_01"), time.Now(), 0)
		if !upd.timeTagAtSourceOk || !upd.timeTagAtSource.Equal(src) {
			t.Errorf("%s: timeTagAtSource = %v (ok=%v), want %v (ms kept)", c.name, upd.timeTagAtSource, upd.timeTagAtSourceOk, src)
		}
	}

	// HoursShift: peer stamps local time 3h ahead of UTC; shift -3 corrects it.
	upd := dataValueToUpdate(cases[0].dv, testMapping("Ext_01"), time.Now(), -3)
	if want := src.Add(-3 * time.Hour); !upd.timeTagAtSource.Equal(want) {
		t.Errorf("hoursShift: got %v, want %v", upd.timeTagAtSource, want)
	}
}

// ICCP states map to JSON-SCADA digital values: On 1, Off 0; the
// in-between and invalid states are flagged invalid (in-between also
// transient).
func TestDataValueToUpdateStates(t *testing.T) {
	cases := []struct {
		state              tase2.StateValue
		value              float64
		invalid, transient bool
	}{
		{tase2.StateOn, 1, false, false},
		{tase2.StateOff, 0, false, false},
		{tase2.StateIntermediate, 0, true, true},
		{tase2.StateInvalid, 0, true, false},
	}
	for _, c := range cases {
		dv := tase2.NewStateQTimeTagExtended(c.state, tase2.QualityGood, time.Now())
		upd := dataValueToUpdate(dv, testMapping("State_01"), time.Now(), 0)
		if upd.value != c.value || upd.invalid != c.invalid || upd.transient != c.transient {
			t.Errorf("%v: value/invalid/transient = %v/%v/%v, want %v/%v/%v",
				c.state, upd.value, upd.invalid, upd.transient, c.value, c.invalid, c.transient)
		}
	}
}

func TestParseHostPort(t *testing.T) {
	cases := []struct {
		in   string
		host string
		port int
	}{
		{"192.168.1.60:102", "192.168.1.60", 102},
		{"192.168.1.60", "192.168.1.60", 102},
		{"scada.example:10102", "scada.example", 10102},
		{"[fe80::1]:2102", "fe80::1", 2102},
		{"fe80::1", "fe80::1", 102},
		{"iccp://10.0.0.5:103/", "10.0.0.5", 103},
		{" 10.0.0.5:bad ", "10.0.0.5", 102},
	}
	for _, c := range cases {
		if h, p := parseHostPort(c.in); h != c.host || p != c.port {
			t.Errorf("parseHostPort(%q) = %q, %d; want %q, %d", c.in, h, p, c.host, c.port)
		}
	}
}

// Without a source time, timeTagAtSource is stored as null, not year 1.
func TestBuildSourceDataUpdateNoSourceTime(t *testing.T) {
	model := buildSourceDataUpdate(dataUpdate{protocolSourceObjectAddress: "A/B", transient: true})
	upd := model.(*mongo.UpdateOneModel).Update.(bson.D)
	sdu := upd[0].Value.(bson.D)[0].Value.(bson.D)
	got := map[string]interface{}{}
	for _, e := range sdu {
		got[e.Key] = e.Value
	}
	if got["timeTagAtSource"] != nil {
		t.Errorf("timeTagAtSource = %v, want nil", got["timeTagAtSource"])
	}
	if got["transientAtSource"] != true {
		t.Errorf("transientAtSource = %v, want true", got["transientAtSource"])
	}
}

// Updates are only published on the active node; invalidation keeps the last
// value and skips points already invalid.
func TestConnRuntimePublishAndInvalidate(t *testing.T) {
	ch := make(chan dataUpdate, 10)
	rt := newConnRuntime(protocolConnection{ProtocolConnectionNumber: 7}, ch)

	isActive.Store(false)
	if rt.publish(dataUpdate{protocolSourceObjectAddress: "A/X", value: 3}) {
		t.Fatal("publish succeeded on an inactive node")
	}

	isActive.Store(true)
	defer isActive.Store(false)
	if !rt.publish(dataUpdate{protocolSourceObjectAddress: "A/X", value: 3}) {
		t.Fatal("publish failed on the active node")
	}
	<-ch
	if n := rt.invalidate(nil); n != 1 {
		t.Fatalf("invalidate = %d, want 1", n)
	}
	upd := <-ch
	if !upd.invalid || upd.value != 3 || upd.protocolSourceConnectionNumber != 7 || upd.timeTagAtSourceOk {
		t.Errorf("invalidated update = %+v, want value 3, invalid, connection 7, no source time", upd)
	}
	if n := rt.invalidate(nil); n != 0 {
		t.Errorf("second invalidate = %d, want 0 (already invalid)", n)
	}
}

// realtimeData keys are numbers: a tag document must decode.
func TestRtDataNumericID(t *testing.T) {
	raw, _ := bson.Marshal(bson.M{"_id": 12.0, "tag": "X"})
	var r rtData
	if err := bson.Unmarshal(raw, &r); err != nil || r.ID != 12 {
		t.Fatalf("decode realtimeData: %v (id %v), want numeric _id 12", err, r.ID)
	}
}
