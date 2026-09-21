package serverapp

import (
	"testing"
	"time"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/outstation"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// newTestDB builds a database big enough for the conversion tests.
func newTestDB() *outstation.Database {
	return outstation.NewDatabase(outstation.DatabaseConfig{
		Binary: 4, DoubleBitBinary: 4, Counter: 4, FrozenCounter: 4,
		Analog: 4, BinaryOutputStatus: 4, AnalogOutputStatus: 4,
		DefaultClass: dnp3.Class1,
	}, outstation.NewEventBuffer(outstation.EventBufferConfig{MaxEvents: 100}))
}

func dest(commonAddress, objectAddress int) Destination {
	return Destination{
		ConnectionNumber: 1,
		CommonAddress:    commonAddress,
		ObjectAddress:    objectAddress,
		KConv1:           1,
		KConv2:           0,
	}
}

// TestApplyValueDigital checks the digital families, including the KConv1 = -1
// inversion the driver documents as "use -1 to invert digital states".
func TestApplyValueDigital(t *testing.T) {
	conn := &Connection{ProtocolConnectionNumber: 1}
	db := newTestDB()

	doc := bson.M{"_id": 1.0, "tag": "T", "value": true}
	ApplyValue(db, doc, dest(1, 0), conn)
	if v, _, _ := db.Binary(0); !v.Value {
		t.Error("binary 0 should be true")
	}

	inverted := dest(1, 1)
	inverted.KConv1 = -1
	ApplyValue(db, doc, inverted, conn)
	if v, _, _ := db.Binary(1); v.Value {
		t.Error("binary 1 should be inverted to false by KConv1 = -1")
	}

	ApplyValue(db, doc, dest(10, 0), conn)
	if v, _, _ := db.BinaryOutputStatus(0); !v.Value {
		t.Error("binary output status 0 should be true")
	}
}

// TestApplyValueDoubleBit checks that a transient tag becomes an intermediate
// or indeterminate double-bit state rather than a comm-lost point.
func TestApplyValueDoubleBit(t *testing.T) {
	conn := &Connection{ProtocolConnectionNumber: 1}
	db := newTestDB()

	cases := []struct {
		value, transient bool
		want             dnp3.DoubleBit
	}{
		{true, false, dnp3.DoubleBitDeterminedOn},
		{false, false, dnp3.DoubleBitDeterminedOff},
		{false, true, dnp3.DoubleBitIntermediate},
		{true, true, dnp3.DoubleBitIndeterminate},
	}
	for i, c := range cases {
		doc := bson.M{"_id": 1.0, "value": c.value, "transient": c.transient}
		ApplyValue(db, doc, dest(3, i), conn)
		v, _, _ := db.DoubleBit(uint16(i))
		if v.Value != c.want {
			t.Errorf("value=%v transient=%v: state = %v, want %v",
				c.value, c.transient, v.Value, c.want)
		}
		// A double-bit point expresses movement in its value, so a transient
		// reading stays online rather than becoming comm-lost.
		if c.transient && v.Flags.Has(dnp3.CommLost) {
			t.Error("a transient double-bit point must not be marked comm-lost")
		}
	}
}

// TestApplyValueAnalogScaling checks the conversion factors.
func TestApplyValueAnalogScaling(t *testing.T) {
	conn := &Connection{ProtocolConnectionNumber: 1}
	db := newTestDB()

	d := dest(30, 0)
	d.KConv1, d.KConv2 = 2.0, 10.0
	ApplyValue(db, bson.M{"_id": 1.0, "value": 5.0}, d, conn)
	if v, _, _ := db.Analog(0); v.Value != 20.0 {
		t.Errorf("analog 0 = %v, want 20 (5 * 2 + 10)", v.Value)
	}

	// Unity factors must leave the value untouched, fraction included.
	ApplyValue(db, bson.M{"_id": 1.0, "value": 123.456}, dest(30, 1), conn)
	if v, _, _ := db.Analog(1); v.Value != 123.456 {
		t.Errorf("analog 1 = %v, want 123.456", v.Value)
	}

	d40 := dest(40, 0)
	d40.KConv1, d40.KConv2 = 0.5, -1.0
	ApplyValue(db, bson.M{"_id": 1.0, "value": 10.0}, d40, conn)
	if v, _, _ := db.AnalogOutputStatus(0); v.Value != 4.0 {
		t.Errorf("analog output 0 = %v, want 4 (10 * 0.5 - 1)", v.Value)
	}
}

// TestApplyValueCounters checks that a frozen counter is written to the frozen
// counter family, not to the counter family.
//
// The C++ server falls through from case 21/23 into case 20 and writes both to
// the counter array; the Go database keeps them apart, so a frozen counter has
// to go where it was sized (deviation D16).
func TestApplyValueCounters(t *testing.T) {
	conn := &Connection{ProtocolConnectionNumber: 1}
	db := newTestDB()

	ApplyValue(db, bson.M{"_id": 1.0, "value": 4242.0}, dest(20, 0), conn)
	if v, _, _ := db.Counter(0); v.Value != 4242 {
		t.Errorf("counter 0 = %d, want 4242", v.Value)
	}

	ApplyValue(db, bson.M{"_id": 1.0, "value": 99.0}, dest(21, 1), conn)
	if v, _, _ := db.FrozenCounter(1); v.Value != 99 {
		t.Errorf("frozen counter 1 = %d, want 99", v.Value)
	}
	if v, _, _ := db.Counter(1); v.Value != 0 {
		t.Error("a frozen counter must not be written into the counter family")
	}
}

// TestApplyValueQuality checks the flags octet each family produces.
func TestApplyValueQuality(t *testing.T) {
	conn := &Connection{ProtocolConnectionNumber: 1}
	db := newTestDB()

	ApplyValue(db, bson.M{"_id": 1.0, "value": 1.0, "invalid": true}, dest(30, 0), conn)
	if v, _, _ := db.Analog(0); !v.Flags.Has(dnp3.CommLost) || v.Flags.Has(dnp3.Online) {
		t.Errorf("an invalid tag must be comm-lost, got %s", v.Flags)
	}

	ApplyValue(db, bson.M{"_id": 1.0, "value": 1.0, "substituted": true}, dest(30, 1), conn)
	if v, _, _ := db.Analog(1); !v.Flags.Has(dnp3.Online) || !v.Flags.Has(dnp3.LocalForced) {
		t.Errorf("a substituted tag must be online and locally forced, got %s", v.Flags)
	}

	ApplyValue(db, bson.M{"_id": 1.0, "value": 1.0, "overflow": true}, dest(30, 2), conn)
	if v, _, _ := db.Analog(2); !v.Flags.Has(dnp3.OverRange) {
		t.Errorf("an overflowing analog must be over-range, got %s", v.Flags)
	}

	ApplyValue(db, bson.M{"_id": 1.0, "value": 1.0, "overflow": true}, dest(20, 2), conn)
	if v, _, _ := db.Counter(2); !v.Flags.Has(dnp3.Rollover) {
		t.Errorf("an overflowing counter must roll over, got %s", v.Flags)
	}
}

// TestTimestampFor checks the timestamp branches, including the hours shift,
// which is applied from both the destination and the connection.
func TestTimestampFor(t *testing.T) {
	const baseMs = 1700000000000

	t.Run("a source timestamp is preferred", func(t *testing.T) {
		doc := bson.M{
			"timeTagAtSource":   bson.DateTime(baseMs),
			"timeTagAtSourceOk": true,
			"timeTag":           bson.DateTime(baseMs + 999999),
		}
		ts := timestampFor(doc, dest(30, 0), 0)
		if ts.Time.UnixMilli() != baseMs {
			t.Errorf("time = %d, want %d", ts.Time.UnixMilli(), baseMs)
		}
		if ts.Quality != dnp3.TimestampSynchronized {
			t.Errorf("quality = %v, want synchronized", ts.Quality)
		}
	})

	t.Run("an unsynchronised source timestamp keeps its value", func(t *testing.T) {
		doc := bson.M{
			"timeTagAtSource":   bson.DateTime(baseMs),
			"timeTagAtSourceOk": false,
		}
		ts := timestampFor(doc, dest(30, 0), 0)
		if ts.Quality != dnp3.TimestampUnsynchronized {
			t.Errorf("quality = %v, want unsynchronized", ts.Quality)
		}
	})

	t.Run("the server timestamp is the fallback", func(t *testing.T) {
		doc := bson.M{"timeTag": bson.DateTime(baseMs)}
		ts := timestampFor(doc, dest(30, 0), 0)
		if ts.Time.UnixMilli() != baseMs {
			t.Errorf("time = %d, want %d", ts.Time.UnixMilli(), baseMs)
		}
		// deviation D15: the C++ server marks this INVALID, but that would
		// discard the time entirely in this library. Unsynchronised keeps a
		// usable timestamp and is honest about it.
		if ts.Quality != dnp3.TimestampUnsynchronized {
			t.Errorf("quality = %v, want unsynchronized", ts.Quality)
		}
		if !ts.IsValid() {
			t.Error("the timestamp must remain usable")
		}
	})

	t.Run("with no timestamp at all the present time is used", func(t *testing.T) {
		ts := timestampFor(bson.M{}, dest(30, 0), 0)
		if time.Since(ts.Time) > time.Minute {
			t.Errorf("time = %v, want approximately now", ts.Time)
		}
	})

	t.Run("the hours shift adds up", func(t *testing.T) {
		doc := bson.M{"timeTagAtSource": bson.DateTime(baseMs), "timeTagAtSourceOk": true}
		d := dest(30, 0)
		d.HoursShift = 2
		ts := timestampFor(doc, d, 1) // plus one hour from the connection
		want := int64(baseMs + 3*3600*1000)
		if ts.Time.UnixMilli() != want {
			t.Errorf("time = %d, want %d", ts.Time.UnixMilli(), want)
		}
	})
}

// TestSizeDatabase checks that the database is sized from the highest object
// address of each family, and that group 50 is dropped rather than sized.
func TestSizeDatabase(t *testing.T) {
	conn := &Connection{ProtocolConnectionNumber: 1}
	tags := []bson.M{
		{"protocolDestinations": bson.A{
			bson.M{"protocolDestinationConnectionNumber": 1.0,
				"protocolDestinationCommonAddress": 1.0, "protocolDestinationObjectAddress": 5.0},
		}},
		{"protocolDestinations": bson.A{
			bson.M{"protocolDestinationConnectionNumber": 1.0,
				"protocolDestinationCommonAddress": 30.0, "protocolDestinationObjectAddress": 2.0},
			// Another connection's destination must not size this one.
			bson.M{"protocolDestinationConnectionNumber": 2.0,
				"protocolDestinationCommonAddress": 30.0, "protocolDestinationObjectAddress": 99.0},
		}},
		{"protocolDestinations": bson.A{
			bson.M{"protocolDestinationConnectionNumber": 1.0,
				"protocolDestinationCommonAddress": 50.0, "protocolDestinationObjectAddress": 3.0},
		}},
	}

	cfg := sizeDatabase(conn, tags)
	if cfg.Binary != 6 {
		t.Errorf("binary = %d, want 6 (highest address 5, plus one)", cfg.Binary)
	}
	if cfg.Analog != 3 {
		t.Errorf("analog = %d, want 3 (another connection must not size this one)", cfg.Analog)
	}
	if cfg.Counter != 0 || cfg.DoubleBitBinary != 0 {
		t.Errorf("unused families must size to zero, got counter=%d doubleBit=%d",
			cfg.Counter, cfg.DoubleBitBinary)
	}
}

// TestConfigurePointsAppliesVariations checks that the per-point ASDU override
// is applied on top of the family defaults, and that the class survives — a
// zero class would be ClassNone and would silently kill the point's events.
func TestConfigurePointsAppliesVariations(t *testing.T) {
	conn := &Connection{ProtocolConnectionNumber: 1}
	tags := []bson.M{
		{"protocolDestinations": bson.A{
			bson.M{"protocolDestinationConnectionNumber": 1.0,
				"protocolDestinationCommonAddress": 30.0,
				"protocolDestinationObjectAddress": 0.0,
				"protocolDestinationASDU":          2.0}, // 16 bit
		}},
	}
	cfg := outstation.DatabaseConfig{Analog: 2}
	db := outstation.NewDatabase(cfg, outstation.NewEventBuffer(outstation.EventBufferConfig{}))

	configurePoints(db, conn, tags, cfg)

	_, pc, ok := db.Analog(0)
	if !ok {
		t.Fatal("analog 0 missing")
	}
	if pc.StaticVariation != 2 || pc.EventVariation != 2 {
		t.Errorf("analog 0 variations = %d/%d, want 2/2 from ASDU 2",
			pc.StaticVariation, pc.EventVariation)
	}
	if pc.Class != dnp3.Class2 {
		t.Errorf("analog 0 class = %v, want class 2", pc.Class)
	}

	// A point with no destination keeps the family default.
	_, pc, _ = db.Analog(1)
	if pc.StaticVariation != 5 || pc.EventVariation != 7 {
		t.Errorf("analog 1 variations = %d/%d, want the 5/7 default",
			pc.StaticVariation, pc.EventVariation)
	}
}
