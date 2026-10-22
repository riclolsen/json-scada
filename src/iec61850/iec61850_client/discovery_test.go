package main

import (
	"reflect"
	"testing"

	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
)

// The type description of an XCBR as the demo IED at 192.168.0.96 serves
// it: LN -> FC -> DO -> attributes, in declaration order.
func xcbrSpec() *mms.TypeSpec {
	st := func(name string, comps ...mms.Component) mms.Component {
		return mms.Component{Name: name, Spec: &mms.TypeSpec{Kind: mms.TypeStructure, Components: comps}}
	}
	leaf := func(name string, k mms.Type) mms.Component {
		return mms.Component{Name: name, Spec: &mms.TypeSpec{Kind: k}}
	}
	return &mms.TypeSpec{Kind: mms.TypeStructure, Components: []mms.Component{
		st("ST",
			st("Pos", leaf("stVal", mms.TypeBitString), leaf("q", mms.TypeBitString), leaf("t", mms.TypeUTCTime)),
			st("OpCnt", leaf("stVal", mms.TypeInteger), leaf("q", mms.TypeBitString), leaf("t", mms.TypeUTCTime)),
		),
		st("CF", st("Pos", leaf("ctlModel", mms.TypeInteger))),
	}}
}

// Attribute names come from the type description, in the order the value
// carries them. The name list of the same IED is alphabetical
// ("Pos$q", "Pos$stVal", "Pos$t"), which would make stVal the quality.
func TestChildsFromLNSpec(t *testing.T) {
	spec := xcbrSpec()
	if got := childsFromLNSpec(spec, model.ST, "Pos"); !reflect.DeepEqual(got, []string{"stVal", "q", "t"}) {
		t.Errorf("ST Pos childs = %v, want [stVal q t]", got)
	}
	if got := childsFromLNSpec(spec, model.CF, "Pos"); !reflect.DeepEqual(got, []string{"ctlModel"}) {
		t.Errorf("CF Pos childs = %v, want [ctlModel]", got)
	}
	if got := childsFromLNSpec(spec, model.MX, "Pos"); got != nil {
		t.Errorf("missing FC: childs = %v, want none", got)
	}
	if got := childsFromLNSpec(spec, model.ST, "Nope"); got != nil {
		t.Errorf("missing DO: childs = %v, want none", got)
	}
	if got := childsFromLNSpec(nil, model.ST, "Pos"); got != nil {
		t.Errorf("no spec: childs = %v, want none", got)
	}
}

// A Pos double point becomes a boolean, on or off, with the transient flag
// on the intermediate state, when its attribute names are in value order.
func TestDoublePointBecomesBooleanWithTransient(t *testing.T) {
	conn := newTestConnection("127.0.0.1:1")
	entry := &Iec61850Entry{Path: "DemoProtCtrl/Obj1XCBR1.Pos", FC: model.ST,
		Childs: childsFromLNSpec(xcbrSpec(), model.ST, "Pos")}
	cases := []struct {
		name          string
		b0, b1        bool
		value         float64
		valueString   string
		transient, ok bool
	}{
		{"off", false, true, 0, "false", false, true},
		{"on", true, false, 1, "true", false, true},
		{"intermediate", false, false, 0, "false", true, false},
		{"bad", true, true, 1, "true", false, false},
	}
	for _, c := range cases {
		v := mms.NewStructure(dbpos(c.b0, c.b1), mms.NewBitString(13), mms.NewUTCTimeNow())
		iv, _ := buildIECValue(conn, entry, v, true, true, nil)
		if !iv.IsDigital || iv.Value != c.value || iv.ValueString != c.valueString {
			t.Errorf("%s: digital=%v value=%v %q, want digital %v %q",
				c.name, iv.IsDigital, iv.Value, iv.ValueString, c.value, c.valueString)
		}
		if iv.IsTransient != c.transient {
			t.Errorf("%s: transient = %v, want %v", c.name, iv.IsTransient, c.transient)
		}
		if iv.Quality != c.ok {
			t.Errorf("%s: quality good = %v, want %v", c.name, iv.Quality, c.ok)
		}
	}
}
