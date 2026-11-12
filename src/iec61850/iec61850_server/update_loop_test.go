package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
	"github.com/dscsystems/go-iec61850/server"
)

// The quality mapping is the C# MapQuality, bit for bit.
func TestMapQuality(t *testing.T) {
	cases := []struct {
		name     string
		upd      PointUpdate
		validity model.Validity
		flags    model.Quality
	}{
		{"good", PointUpdate{}, model.ValidityGood, 0},
		{"invalid", PointUpdate{Invalid: true}, model.ValidityInvalid, 0},
		{"overflow", PointUpdate{Overflow: true}, model.ValidityQuestionable, model.QualityOverflow},
		{"transient", PointUpdate{Transient: true}, model.ValidityQuestionable, model.QualityOscillatory},
		{"both", PointUpdate{Overflow: true, Transient: true}, model.ValidityQuestionable,
			model.QualityOverflow | model.QualityOscillatory},
		{"substituted", PointUpdate{Substituted: true}, model.ValidityGood, model.QualitySubstituted},
		{"test", PointUpdate{Test: true}, model.ValidityGood, model.QualityTest},
		// Invalid wins over the questionable causes, as in the C#.
		{"invalid wins", PointUpdate{Invalid: true, Overflow: true}, model.ValidityInvalid, 0},
	}
	for _, c := range cases {
		q := mapQuality(c.upd)
		if q.Validity() != c.validity {
			t.Errorf("%s: validity = %v, want %v", c.name, q.Validity(), c.validity)
		}
		if c.flags != 0 && !q.Is(c.flags) {
			t.Errorf("%s: flags %v not set in %v", c.name, c.flags, q)
		}
	}
}

// applyUpdate writes the value in the shape each class expects, and always
// writes quality and timestamp alongside it.
func TestApplyUpdatePerClass(t *testing.T) {
	conn := testConn()
	points := []*Point{
		pt(1, "D1", "digital", "supervised", "T"),
		pt(2, "A1", "analog", "supervised", "T"),
		pt(3, "S1", "string", "supervised", "T"),
	}
	built := BuildModel(points, conn)
	srv := server.New(built.Model)

	when := time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC)
	srv.Update(func(tx *server.Tx) {
		applyUpdate(tx, PointUpdate{
			Point: built.ByTag["D1"], Value: 1,
			SourceTime: when, HasSourceTime: true, SourceTimeOk: true,
		})
		applyUpdate(tx, PointUpdate{
			Point: built.ByTag["A1"], Value: 42.5,
			SourceTime: when, HasSourceTime: true, SourceTimeOk: true,
		})
		applyUpdate(tx, PointUpdate{
			Point: built.ByTag["S1"], ValueString: "hello",
			SourceTime: when, HasSourceTime: true, SourceTimeOk: true,
		})
	})

	if v := srv.Read("IEC61850SRVT/GGIO1.Ind1.stVal", model.ST); v == nil || !v.Bool() {
		t.Errorf("digital value = %v", v)
	}
	if v := srv.Read("IEC61850SRVT/GGIO1.AnIn1.mag.f", model.MX); v == nil || v.Float64() != 42.5 {
		t.Errorf("analog value = %v", v)
	}
	if v := srv.Read("IEC61850SRVT/GGIO1.Str1.stVal", model.ST); v == nil || v.Text() != "hello" {
		t.Errorf("string value = %v", v)
	}

	// Quality and timestamp travel with the value.
	q := srv.Read("IEC61850SRVT/GGIO1.Ind1.q", model.ST)
	if q == nil || model.QualityFromValue(q).Validity() != model.ValidityGood {
		t.Errorf("quality = %v", q)
	}
	ts := srv.Read("IEC61850SRVT/GGIO1.Ind1.t", model.ST)
	if ts == nil || ts.Time().Unix() != when.Unix() {
		t.Errorf("timestamp = %v, want %v", ts, when)
	}
	if ts.TimeQualityFlags()&mms.TimeClockNotSynchronized != 0 {
		t.Error("a trustworthy source time must not be marked clock-not-synchronised")
	}
}

// A missing or untrustworthy source timestamp is flagged in the time
// quality, and the server's own clock is used.
func TestApplyUpdateTimeQuality(t *testing.T) {
	conn := testConn()
	built := BuildModel([]*Point{pt(1, "D1", "digital", "supervised", "T")}, conn)
	srv := server.New(built.Model)
	ref := model.ObjectReference("IEC61850SRVT/GGIO1.Ind1.t")

	srv.Update(func(tx *server.Tx) {
		applyUpdate(tx, PointUpdate{Point: built.ByTag["D1"], HasSourceTime: false})
	})
	ts := srv.Read(ref, model.ST)
	if ts.TimeQualityFlags()&mms.TimeClockNotSynchronized == 0 {
		t.Error("no source time must be marked clock-not-synchronised")
	}
	if time.Since(ts.Time()) > time.Minute {
		t.Errorf("fallback timestamp is not the server clock: %v", ts.Time())
	}

	srv.Update(func(tx *server.Tx) {
		applyUpdate(tx, PointUpdate{
			Point: built.ByTag["D1"], HasSourceTime: true, SourceTimeOk: false,
			SourceTime: time.Now().UTC(),
		})
	})
	ts = srv.Read(ref, model.ST)
	if ts.TimeQualityFlags()&mms.TimeClockNotSynchronized == 0 {
		t.Error("a source time flagged not-OK must be marked clock-not-synchronised")
	}
}

// The initial snapshot reaches the model, and command points are not
// written from the database.
func TestApplyInitialValues(t *testing.T) {
	conn := testConn()
	points := []*Point{
		pt(1, "D1", "digital", "supervised", "T"),
		pt(2, "C1", "digital", "command", "T"),
	}
	points[0].Value = 1
	points[0].Invalid = false
	points[1].Value = 1

	built := BuildModel(points, conn)
	g := &Gateway{conn: conn, built: built, srv: server.New(built.Model)}
	applyInitialValues(g, points)

	if v := g.srv.Read("IEC61850SRVT/GGIO1.Ind1.stVal", model.ST); v == nil || !v.Bool() {
		t.Errorf("monitored point not loaded: %v", v)
	}
	if v := g.srv.Read("IEC61850SRVT/GGIO1.SPCSO1.stVal", model.ST); v == nil || v.Bool() {
		t.Error("command point must not be written from the database")
	}
}

// The change-stream pipeline keeps the C# filter semantics. The first stage
// must not reference fullDocument, so the server can apply it before the
// updateLookup; only the topic filter (a second stage) may.
func TestChangeStreamPipeline(t *testing.T) {
	p := csPipeline(nil)
	if len(p) != 1 {
		t.Fatalf("pipeline stages = %d", len(p))
	}
	if p[0][0].Key != "$match" {
		t.Fatalf("first stage = %q, want $match", p[0][0].Key)
	}
	if s := fmt.Sprint(p[0][0].Value); strings.Contains(s, "fullDocument") {
		t.Errorf("first stage references fullDocument: %s", s)
	}
	withTopics := csPipeline([]string{"KAW2"})
	if len(withTopics) != 2 {
		t.Fatalf("pipeline stages with topics = %d", len(withTopics))
	}
	if s := fmt.Sprint(withTopics[1][0].Value); !strings.Contains(s, "fullDocument.group1") {
		t.Errorf("second stage does not filter the topics: %s", s)
	}
}
