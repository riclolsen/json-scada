package main

import (
	"context"
	"math"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func sampleValue() IECValue {
	return IECValue{
		Address:       "DemoProtCtrl/Obj1XCBR1.Pos",
		Asdu:          "MMS_STRUCTURE",
		Value:         1,
		ValueString:   "true",
		Cot:           20,
		Quality:       true,
		ConnNumber:    101,
		ConnName:      "IED1",
		CommonAddress: "ST",
		DisplayName:   "DemoProtCtrl/Obj1XCBR1.Pos",
	}
}

func TestTagFromParameters(t *testing.T) {
	got := TagFromParameters(sampleValue())
	want := "IED1;DemoProtCtrl/Obj1XCBR1.Pos[ST]"
	if got != want {
		t.Errorf("TagFromParameters = %q, want %q", got, want)
	}
}

// The tag documents must carry every field the C# driver writes, with the
// same types: json-scada stores configuration numbers as BSON doubles.
func TestNewRealtimeDocFields(t *testing.T) {
	iv := sampleValue()
	iv.IsDigital = true
	doc := newRealtimeDoc(iv, 101000001)

	wantFields := []string{
		"_id", "protocolSourceASDU", "protocolSourceCommonAddress",
		"protocolSourceConnectionNumber", "protocolSourceObjectAddress",
		"protocolSourceCommandUseSBO", "protocolSourceCommandDuration",
		"alarmState", "description", "ungroupedDescription", "group1", "group2",
		"group3", "stateTextFalse", "stateTextTrue", "eventTextFalse",
		"eventTextTrue", "origin", "tag", "type", "value", "valueString",
		"alarmDisabled", "alerted", "alarmed", "alertState", "annotation",
		"commandBlocked", "commandOfSupervised", "commissioningRemarks",
		"formula", "frozen", "frozenDetectTimeout", "hiLimit", "hihiLimit",
		"hihihiLimit", "historianDeadBand", "historianPeriod", "hysteresis",
		"invalid", "invalidDetectTimeout", "isEvent", "kconv1", "kconv2",
		"location", "loLimit", "loloLimit", "lololoLimit", "notes", "overflow",
		"parcels", "priority", "protocolDestinations", "sourceDataUpdate",
		"substituted", "supervisedOfCommand", "timeTag", "timeTagAlarm",
		"timeTagAtSource", "timeTagAtSourceOk", "transient", "unit",
		"updatesCnt", "valueDefault", "zeroDeadband",
	}
	for _, f := range wantFields {
		if _, ok := doc[f]; !ok {
			t.Errorf("missing field %q", f)
		}
	}

	// Every number must be a float64 so it lands in MongoDB as a double.
	for k, v := range doc {
		switch v.(type) {
		case int, int32, int64:
			t.Errorf("field %q is an integer (%T), must be a double", k, v)
		}
	}

	// group1 the connection, group2 the logical device, group3 the logical
	// node; the description starts with group1, which viewers strip.
	if doc["description"] != "IED1~DemoProtCtrl~Obj1XCBR1~Pos" {
		t.Errorf("description = %v", doc["description"])
	}
	if doc["ungroupedDescription"] != "Pos" {
		t.Errorf("ungroupedDescription = %v", doc["ungroupedDescription"])
	}
	if doc["group1"] != "IED1" || doc["group2"] != "DemoProtCtrl" || doc["group3"] != "Obj1XCBR1" {
		t.Errorf("groups = %v/%v/%v", doc["group1"], doc["group2"], doc["group3"])
	}
	if doc["hiLimit"] != math.MaxFloat64 || doc["loLimit"] != -math.MaxFloat64 {
		t.Error("limit defaults wrong")
	}
	if doc["invalid"] != true || doc["invalidDetectTimeout"] != 60000.0 {
		t.Error("invalid defaults wrong")
	}
	if doc["origin"] != "supervised" {
		t.Error("origin must be supervised")
	}
}

func TestNewRealtimeDocTypes(t *testing.T) {
	digital := sampleValue()
	digital.IsDigital = true
	d := newRealtimeDoc(digital, 1)
	if d["type"] != "digital" || d["alarmState"] != 2.0 ||
		d["stateTextTrue"] != "TRUE" || d["valueString"] != "????" {
		t.Errorf("digital document wrong: %v", d)
	}

	analog := sampleValue()
	analog.Asdu = "MMS_FLOAT"
	analog.Value = 12.5
	a := newRealtimeDoc(analog, 2)
	if a["type"] != "analog" || a["alarmState"] != -1.0 ||
		a["value"] != 12.5 || a["stateTextTrue"] != "" {
		t.Errorf("analog document wrong: %v", a)
	}

	str := sampleValue()
	str.Asdu = "string"
	str.ValueString = "hello"
	s := newRealtimeDoc(str, 3)
	if s["type"] != "string" || s["value"] != 0.0 || s["valueString"] != "hello" {
		t.Errorf("string document wrong: %v", s)
	}
}

// valueBsonAtSource keeps the { a: <value> } wrapper the C# driver produced;
// the UI and cs_data_processor read that shape.
func TestValueBsonWrapper(t *testing.T) {
	iv := sampleValue()
	iv.ValueJSON = `["1","0000000000000",""]`
	got := parseValueJSON(iv)
	inner, ok := got["a"]
	if !ok {
		t.Fatalf("missing 'a' wrapper: %v", got)
	}
	arr, ok := inner.([]any)
	if !ok || len(arr) != 3 {
		t.Fatalf("wrapped value = %#v", inner)
	}

	// Unparseable JSON yields an empty document, not a panic.
	iv.ValueJSON = "not json"
	if len(parseValueJSON(iv)) != 0 {
		t.Error("bad JSON should produce an empty document")
	}
}

// The sourceDataUpdate document is the driver's only write target for data;
// its field names and types are part of the platform contract.
func TestUpdateModelShape(t *testing.T) {
	iv := sampleValue()
	iv.ValueJSON = `"1"`
	iv.IsTransient = true

	wm := updateModel(iv)
	um, ok := wm.(*mongo.UpdateOneModel)
	if !ok {
		t.Fatalf("updateModel returned %T", wm)
	}

	filter, ok := um.Filter.(bson.M)
	if !ok {
		t.Fatalf("filter is %T", um.Filter)
	}
	if filter["protocolSourceConnectionNumber"] != float64(101) ||
		filter["protocolSourceObjectAddress"] != iv.Address ||
		filter["origin"] != "supervised" {
		t.Errorf("filter = %v", filter)
	}

	update := um.Update.(bson.M)
	set := update["$set"].(bson.M)
	sdu := set["sourceDataUpdate"].(bson.M)

	for _, f := range []string{
		"valueBsonAtSource", "valueAtSource", "valueStringAtSource",
		"asduAtSource", "causeOfTransmissionAtSource", "timeTagAtSource",
		"timeTagAtSourceOk", "timeTag", "notTopicalAtSource", "invalidAtSource",
		"overflowAtSource", "blockedAtSource", "substitutedAtSource",
		"transientAtSource", "originator",
	} {
		if _, ok := sdu[f]; !ok {
			t.Errorf("sourceDataUpdate missing %q", f)
		}
	}
	if sdu["causeOfTransmissionAtSource"] != "20" {
		t.Errorf("cause of transmission = %v, want \"20\"", sdu["causeOfTransmissionAtSource"])
	}
	if sdu["originator"] != "IEC61850|101" {
		t.Errorf("originator = %v", sdu["originator"])
	}
	if sdu["invalidAtSource"] != false || sdu["transientAtSource"] != true {
		t.Errorf("quality flags = %v / %v", sdu["invalidAtSource"], sdu["transientAtSource"])
	}
	if sdu["timeTagAtSource"] != nil || sdu["timeTagAtSourceOk"] != false {
		t.Error("a value without a source timestamp must store null")
	}
	if _, isFloat := sdu["valueAtSource"].(float64); !isFloat {
		t.Errorf("valueAtSource is %T, must be a double", sdu["valueAtSource"])
	}
}

// A command tag carries the routing fields the dispatcher needs, the link
// to its supervised point, and the platform's document shape.
func TestNewCommandDoc(t *testing.T) {
	ct := CommandTag{
		ConnNumber: 101,
		ConnName:   "IED1",
		Ref:        "DemoProtCtrl/Obj1CSWI1.Pos",
		IsDigital:  true,
		UseSBO:     true,
		Asdu:       "MMS_BOOLEAN",
	}
	if got := ct.Tag(); got != "IED1;DemoProtCtrl/Obj1CSWI1.Pos[CO]" {
		t.Errorf("tag = %q", got)
	}

	doc := newCommandDoc(ct, 101000042, 101000007)

	if doc["group1"] != "IED1" || doc["group2"] != "DemoProtCtrl" || doc["group3"] != "Obj1CSWI1" {
		t.Errorf("groups = %v/%v/%v", doc["group1"], doc["group2"], doc["group3"])
	}
	if doc["description"] != "IED1~DemoProtCtrl~Obj1CSWI1~Pos command" {
		t.Errorf("description = %v", doc["description"])
	}

	if doc["origin"] != "command" {
		t.Errorf("origin = %v, want command", doc["origin"])
	}
	if doc["protocolSourceCommonAddress"] != "CO" {
		t.Errorf("common address = %v, want CO", doc["protocolSourceCommonAddress"])
	}
	if doc["protocolSourceObjectAddress"] != ct.Ref {
		t.Errorf("object address = %v", doc["protocolSourceObjectAddress"])
	}
	if doc["protocolSourceCommandUseSBO"] != true {
		t.Errorf("useSBO = %v", doc["protocolSourceCommandUseSBO"])
	}
	if doc["protocolSourceConnectionNumber"] != float64(101) {
		t.Errorf("connection number = %#v", doc["protocolSourceConnectionNumber"])
	}
	// The pair: the command names the point where its effect shows.
	if doc["supervisedOfCommand"] != float64(101000007) {
		t.Errorf("supervisedOfCommand = %#v, want the supervised key", doc["supervisedOfCommand"])
	}
	if doc["commandOfSupervised"] != 0.0 {
		t.Errorf("commandOfSupervised = %#v, must be zero on a command point", doc["commandOfSupervised"])
	}
	if doc["type"] != "digital" || doc["stateTextTrue"] != "TRUE" {
		t.Errorf("digital command wrong: %v / %v", doc["type"], doc["stateTextTrue"])
	}
	// A command has no acquisition, so it must not expire as invalid.
	if doc["invalid"] != false || doc["invalidDetectTimeout"] != 0.0 {
		t.Errorf("invalid handling wrong: %v / %v", doc["invalid"], doc["invalidDetectTimeout"])
	}
	for k, v := range doc {
		switch v.(type) {
		case int, int32, int64:
			t.Errorf("field %q is an integer (%T), must be a double", k, v)
		}
	}

	// An analogue setpoint.
	ct.IsDigital = false
	ct.UseSBO = false
	ct.Asdu = "MMS_STRUCTURE"
	a := newCommandDoc(ct, 2, 0)
	if a["type"] != "analog" || a["stateTextTrue"] != "" {
		t.Errorf("analog command wrong: %v / %v", a["type"], a["stateTextTrue"])
	}
	// No supervised twin found: still commandable, just without feedback.
	if a["supervisedOfCommand"] != 0.0 {
		t.Errorf("unlinked command = %#v", a["supervisedOfCommand"])
	}
}

// The groups come from the object reference even when the value carries a
// display name of its own, which then becomes the ungrouped description.
func TestNewRealtimeDocGroupsFromReference(t *testing.T) {
	iv := sampleValue()
	iv.Address = "IED1/MMXU1.A.phsA.cVal.mag.f"
	iv.DisplayName = "Phase A current"
	doc := newRealtimeDoc(iv, 101000001)
	if doc["group1"] != "IED1" || doc["group2"] != "IED1" || doc["group3"] != "MMXU1" {
		t.Errorf("groups = %v/%v/%v", doc["group1"], doc["group2"], doc["group3"])
	}
	if doc["ungroupedDescription"] != "Phase A current" {
		t.Errorf("ungroupedDescription = %v", doc["ungroupedDescription"])
	}
	if doc["description"] != "IED1~IED1~MMXU1~Phase A current" {
		t.Errorf("description = %v", doc["description"])
	}
}

func TestSplitRef(t *testing.T) {
	cases := []struct{ in, ld, ln, rest string }{
		{"DemoProtCtrl/Obj1XCBR1.Pos", "DemoProtCtrl", "Obj1XCBR1", "Pos"},
		{"IED1/MMXU1.A.phsA.cVal.mag.f", "IED1", "MMXU1", "A.phsA.cVal.mag.f"},
		{"IED1/LLN0", "IED1", "", "LLN0"},
		{"NoDevice", "", "", "NoDevice"},
	}
	for _, c := range cases {
		ld, ln, rest := splitRef(c.in)
		if ld != c.ld || ln != c.ln || rest != c.rest {
			t.Errorf("splitRef(%q) = %q,%q,%q; want %q,%q,%q", c.in, ld, ln, rest, c.ld, c.ln, c.rest)
		}
	}
}

// A point that already has a tag is never inserted again, whatever that tag
// is called: tags created before the naming changed ("IEC61850;<conn>;...")
// are matched by object reference and functional constraint, which is what
// preloadEntries records for every tag of the connection.
func TestExistingTagIsMatchedByReferenceNotName(t *testing.T) {
	iv := sampleValue()
	conn := &Iec61850Connection{
		ProtocolConnectionNumber: iv.ConnNumber,
		InsertedTags:             map[string]bool{},
	}
	// What preloadEntries records for the old-named document
	// {tag: "IEC61850;IED1;DemoProtCtrl/Obj1XCBR1.Pos[ST]",
	//  protocolSourceObjectAddress: "DemoProtCtrl/Obj1XCBR1.Pos", protocolSourceCommonAddress: "ST"}.
	conn.InsertedTags[pointKey("DemoProtCtrl/Obj1XCBR1.Pos", "ST")] = true

	iv.CommonAddress = "st" // the FC case of a value does not matter
	if m, _, _ := maybeInsertTag(context.Background(), nil, []*Iec61850Connection{conn}, iv); m != nil {
		t.Error("a point that already has a tag was inserted again")
	}
}
