package serverapp

import (
	"math"
	"testing"

	dnp3 "github.com/dscsystems/go-dnp3"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func analogTag(id float64, value float64, address float64) bson.M {
	return bson.M{"_id": id, "tag": "t", "origin": "supervised", "type": "analog", "value": value,
		"protocolDestinations": bson.A{bson.M{
			"protocolDestinationConnectionNumber": 1.0,
			"protocolDestinationCommonAddress":    30.0,
			"protocolDestinationObjectAddress":    address,
		}}}
}

// TestOutOfRangeAddressesDoNotWrap: a destination past 65535 must not land on
// the point its low 16 bits name, nor size the database for it.
func TestOutOfRangeAddressesDoNotWrap(t *testing.T) {
	conn := &Connection{ProtocolConnectionNumber: 1, Name: "S", ServerQueueSize: 100}
	tags := []bson.M{analogTag(1, 111, 5), analogTag(2, 999, 65541), analogTag(3, 7, -1)}
	st := (&Engine{}).newOutstation(conn, tags)

	if conn.counts.Analog != 6 {
		t.Errorf("sized %d analogs, want 6", conn.counts.Analog)
	}
	if v, _, _ := st.Database().Analog(5); v.Value != 111 {
		t.Errorf("point 5 = %v, want 111: an out-of-range address wrapped onto it", v.Value)
	}

	// The change stream path must refuse it too.
	e := &Engine{byNum: map[int]*Connection{1: conn}}
	conn.setStation(st)
	e.distribute(analogTag(2, 555, 65541))
	if v, _, _ := st.Database().Analog(5); v.Value != 111 {
		t.Errorf("point 5 = %v after a change, want 111", v.Value)
	}
}

func TestCounterValueClamps(t *testing.T) {
	cases := []struct {
		in      float64
		want    uint32
		clamped bool
	}{
		{0, 0, false}, {42.9, 42, false}, {math.MaxUint32, math.MaxUint32, false},
		{-1, 0, true}, {math.MaxUint32 + 6, math.MaxUint32, true}, {math.NaN(), 0, true},
	}
	for _, c := range cases {
		got, clamped := counterValue(c.in)
		if got != c.want || clamped != c.clamped {
			t.Errorf("counterValue(%v) = %d,%v; want %d,%v", c.in, got, clamped, c.want, c.clamped)
		}
	}
	if f := counterFlags(tagQuality(bson.M{}), true); f&dnp3.Discontinuity == 0 {
		t.Errorf("a clamped counter carries flags %v, want DISCONTINUITY", f)
	}
}
