package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dscsystems/go-iec61850/client"
	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
)

func utcAt(t time.Time) *mms.Value { return mms.NewUTCTime(t, mms.TimeAccuracy(10)) }

func invalidQ() *mms.Value { return model.QualityGood.WithValidity(model.ValidityInvalid).Value() }

// cmvValue is a CMV: { cVal { mag { f }, ang { f } }, q, t }.
func cmvValue(mag, ang float32, q *mms.Value) *mms.Value {
	return mms.NewStructure(
		mms.NewStructure(mms.NewStructure(mms.NewFloat32(mag)), mms.NewStructure(mms.NewFloat32(ang))),
		q, utcAt(time.Now()))
}

// The IED's quality reaches the tag (D13): an invalid q is invalid whether
// or not the attribute names are known, and t is the timestamp even when a
// later member is also a time (tOpOk of a controllable object).
func TestQualityAndTimestampComeFromQAndT(t *testing.T) {
	conn := newTestConnection("127.0.0.1:1")
	when := time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	later := when.Add(time.Hour)

	sps := mms.NewStructure(mms.NewBool(true), invalidQ(), utcAt(when))
	for _, childs := range [][]string{nil, {"stVal", "q", "t"}} {
		e := &Iec61850Entry{Path: "LD/GGIO1.Ind1", FC: model.ST, Childs: childs}
		iv, ts := buildIECValue(conn, e, sps, false, true, nil)
		if iv.Quality {
			t.Errorf("childs %v: an invalid q read as good", childs)
		}
		if iv.Value != 1 || !iv.IsDigital {
			t.Errorf("childs %v: value %v digital %v, want 1 true", childs, iv.Value, iv.IsDigital)
		}
		if ts != uint64(when.UnixMilli()) {
			t.Errorf("childs %v: timestamp %d, want %d", childs, ts, when.UnixMilli())
		}
	}

	mv := mms.NewStructure(mms.NewStructure(mms.NewFloat32(5)), invalidQ(), utcAt(when))
	iv, _ := buildIECValue(conn, &Iec61850Entry{Path: "LD/GGIO1.AnIn1", FC: model.MX}, mv, false, true, nil)
	if iv.Quality || iv.Value != 5 {
		t.Errorf("MV: quality good %v value %v, want false 5", iv.Quality, iv.Value)
	}

	// A DPC's status: origin, ctlNum, stVal, q, t, then tOpOk last.
	dpc := mms.NewStructure(
		mms.NewStructure(mms.NewInt32(3), mms.NewOctetString(nil)),
		mms.NewUint8(1), dbpos(true, false), model.QualityGood.Value(), utcAt(when), utcAt(later))
	e := &Iec61850Entry{Path: "LD/CSWI1.Pos", FC: model.ST,
		Childs: []string{"origin", "ctlNum", "stVal", "q", "t", "tOpOk"}}
	iv, ts := buildIECValue(conn, e, dpc, false, true, nil)
	if !iv.Quality || iv.Value != 1 {
		t.Errorf("DPC: quality good %v value %v, want true 1", iv.Quality, iv.Value)
	}
	if ts != uint64(when.UnixMilli()) {
		t.Errorf("DPC: timestamp is tOpOk, not t")
	}
}

// A complex measurand reads its magnitude, not its angle (D14).
func TestComplexMeasurandReadsMagnitude(t *testing.T) {
	conn := newTestConnection("127.0.0.1:1")
	for _, childs := range [][]string{nil, {"cVal", "q", "t"}} {
		e := &Iec61850Entry{Path: "LD/MMXU1.PhV.phsA", FC: model.MX, Childs: childs}
		iv, _ := buildIECValue(conn, e, cmvValue(230, -120, model.QualityGood.Value()), false, false, nil)
		if iv.Value != 230 {
			t.Errorf("childs %v: value %v, want the magnitude 230", childs, iv.Value)
		}
	}
}

func wyeSpec() *mms.TypeSpec {
	st := func(name string, comps ...mms.Component) mms.Component {
		return mms.Component{Name: name, Spec: &mms.TypeSpec{Kind: mms.TypeStructure, Components: comps}}
	}
	leaf := func(name string, k mms.Type) mms.Component {
		return mms.Component{Name: name, Spec: &mms.TypeSpec{Kind: k}}
	}
	cmv := func(name string) mms.Component {
		return st(name,
			st("cVal", st("mag", leaf("f", mms.TypeFloat32)), st("ang", leaf("f", mms.TypeFloat32))),
			leaf("q", mms.TypeBitString), leaf("t", mms.TypeUTCTime))
	}
	return &mms.TypeSpec{Kind: mms.TypeStructure, Components: []mms.Component{
		st("MX",
			st("PhV", cmv("phsA"), cmv("phsB"), cmv("phsC")),
			st("TotW", st("mag", leaf("f", mms.TypeFloat32)), leaf("q", mms.TypeBitString), leaf("t", mms.TypeUTCTime)),
		),
	}}
}

// A composite data object (WYE) is registered once, split into one point
// per phase, and its value is split the same way.
func TestCompositeDataObjectIsSplitIntoPhases(t *testing.T) {
	conn := newTestConnection("127.0.0.1:1")
	conn.Entries = map[string]*Iec61850Entry{}
	spec := wyeSpec()

	if !registerBrowsedEntry(conn, browsedEntry{ref: "LD/MMXU1.PhV", key: entryKey("LD/MMXU1.PhV", model.MX),
		fc: model.MX, do: "PhV"}, spec) {
		t.Fatal("PhV not registered")
	}
	if !registerBrowsedEntry(conn, browsedEntry{ref: "LD/MMXU1.TotW", key: entryKey("LD/MMXU1.TotW", model.MX),
		fc: model.MX, do: "TotW"}, spec) {
		t.Fatal("TotW not registered")
	}
	phv := conn.Entry(entryKey("LD/MMXU1.PhV", model.MX))
	if len(phv.Parts) != 3 || phv.Parts[1].Path != "LD/MMXU1.PhV.phsB" {
		t.Fatalf("PhV parts = %+v, want phsA..phsC", phv.Parts)
	}
	if totw := conn.Entry(entryKey("LD/MMXU1.TotW", model.MX)); len(totw.Parts) != 0 ||
		fmt.Sprint(totw.Childs) != "[mag q t]" {
		t.Errorf("TotW is not composite: parts %d childs %v", len(totw.Parts), totw.Childs)
	}

	drainQueue()
	wye := mms.NewStructure(
		cmvValue(230, 0, model.QualityGood.Value()),
		cmvValue(231, -120, invalidQ()),
		cmvValue(232, 120, model.QualityGood.Value()))
	emitValues(conn, phv, wye, true, false, 0, nil)
	got := drainQueue()
	if len(got) != 3 {
		t.Fatalf("%d values, want one per phase", len(got))
	}
	for i, want := range []float64{230, 231, 232} {
		if got[i].Value != want || got[i].Address != phv.Parts[i].Path || !got[i].SelfPublish {
			t.Errorf("phase %d: %s = %v (publish %v), want %s = %v", i, got[i].Address, got[i].Value,
				got[i].SelfPublish, phv.Parts[i].Path, want)
		}
	}
	if got[1].Quality {
		t.Error("phsB's invalid quality was lost")
	}
}

// Every segment of a buffered report carries the same EntryID: only a
// segment already seen is a repeat.
func TestSegmentedBufferedReportIsNotDroppedAsRepeated(t *testing.T) {
	conn := newTestConnection("127.0.0.1:1")
	conn.Entries = map[string]*Iec61850Entry{}
	conn.AutoCreateTags = true
	st := &rcbState{ref: "LD/LLN0.BR.brcb01", buffered: true}
	id := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	seg := func(sub uint32, ref string) *client.Report {
		return &client.Report{EntryID: id, SubSeqNum: sub, Entries: []client.ReportEntry{{
			Ref: model.ObjectReference(ref), FC: model.ST, Reason: model.ReasonGI,
			Value: mms.NewStructure(mms.NewBool(true), model.QualityGood.Value(), mms.NewUTCTimeNow()),
		}}}
	}

	drainQueue()
	reportHandler(conn, st, seg(0, "LD/GGIO1.Ind1"))
	reportHandler(conn, st, seg(1, "LD/GGIO1.Ind2"))
	reportHandler(conn, st, seg(1, "LD/GGIO1.Ind2")) // a repeat of segment 1
	if got := len(drainQueue()); got != 2 {
		t.Fatalf("%d values, want one per segment", got)
	}

	// After a reconnection the IED resends from the saved EntryID: the
	// whole report is a repeat, every segment of it.
	st2 := &rcbState{ref: st.ref, buffered: true}
	reportHandler(conn, st2, seg(0, "LD/GGIO1.Ind1"))
	reportHandler(conn, st2, seg(1, "LD/GGIO1.Ind2"))
	if got := len(drainQueue()); got != 0 {
		t.Errorf("%d values from a resent report, want none", got)
	}
}

// A point stays out of the polling sweep only for as long as its report
// block is enabled on the current association.
func TestReportCoverageIsResetPerAssociation(t *testing.T) {
	conn := newTestConnection("127.0.0.1:1")
	e := &Iec61850Entry{Path: "LD/GGIO1.Ind1", FC: model.ST}
	conn.Entries = map[string]*Iec61850Entry{entryKey(e.Path, e.FC): e}
	conn.SetEntryReport(e, "LD/LLN0.BR.brcb01", "LD/LLN0$DS")
	conn.ResetEntryReports()
	if conn.EntryHasReport(e) {
		t.Error("the entry kept the report of a previous association")
	}
}

// A request that timed out on its own budget means the IED stopped
// answering; one cut short by the driver stopping does not.
func TestTimeoutMeansAssociationDown(t *testing.T) {
	if !isAssociationDown(context.Background(), nil, fmt.Errorf("read: %w", context.DeadlineExceeded)) {
		t.Error("a timed-out request did not count as a dead association")
	}
	stopped, cancel := context.WithCancel(context.Background())
	cancel()
	if isAssociationDown(stopped, nil, context.DeadlineExceeded) {
		t.Error("a request cut short by the driver stopping counted as a dead association")
	}
	if isAssociationDown(context.Background(), nil, mms.AccessObjectNonExistent) {
		t.Error("a rejected point counted as a dead association")
	}
}

// Pinning with no certificate to pin to must not accept every peer.
func TestPinningWithoutCertificatesIsRefused(t *testing.T) {
	conn := newTestConnection("127.0.0.1:1")
	conn.UseSecurity = true
	conn.AllowOnlySpecificCertificates = true
	conn.PeerCertFilesPaths = []string{""}
	if _, err := buildTLS(conn); err == nil {
		t.Error("pinning with no certificate was accepted")
	}
}
