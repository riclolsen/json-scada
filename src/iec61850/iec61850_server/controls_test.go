package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dscsystems/go-iec61850/client"
	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
	"github.com/dscsystems/go-iec61850/server"
)

// helpers shared with the loopback test.

func mmsBool(b bool) *mms.Value { return mms.NewBool(b) }

// mmsAnalogue builds the AnalogueValue an APC control carries.
func mmsAnalogue(f float64) *mms.Value {
	return mms.NewStructure(mms.NewFloat32(float32(f)))
}

func asControlError(err error, target **client.ControlError) bool {
	return errors.As(err, target)
}

func waitCommand(t *testing.T, d time.Duration) map[string]any {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if doc, ok := dequeueCommand(); ok {
			return doc
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for a queued command")
	return nil
}

// controlValue converts a ctlVal the way each class expects, which is what
// decides the value pair of the commandsQueue document.
func TestControlValueConversion(t *testing.T) {
	cases := []struct {
		name       string
		kind       PointKind
		in         *mms.Value
		wantValue  float64
		wantString string
		wantOK     bool
	}{
		{"spc true", KindSPC, mms.NewBool(true), 1, "true", true},
		{"spc false", KindSPC, mms.NewBool(false), 0, "false", true},
		{"spc wrong type", KindSPC, mms.NewFloat32(1), 0, "", false},
		{"apc float", KindAPC, mms.NewStructure(mms.NewFloat32(37.5)), 37.5, "37.5", true},
		{"apc integer", KindAPC, mms.NewStructure(mms.NewInt32(42)), 42, "42", true},
		{"apc bare float", KindAPC, mms.NewFloat32(1.5), 1.5, "1.5", true},
		{"inc", KindINC, mms.NewInt32(7), 7, "7", true},
		{"nil", KindSPC, nil, 0, "", false},
	}
	for _, c := range cases {
		v, s, ok := controlValue(c.kind, c.in)
		if ok != c.wantOK {
			t.Errorf("%s: ok = %v, want %v", c.name, ok, c.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if v != c.wantValue || s != c.wantString {
			t.Errorf("%s: (%v,%q), want (%v,%q)", c.name, v, s, c.wantValue, c.wantString)
		}
	}
}

// The queued document must carry every field the C# driver writes, with the
// routing fields keeping the BSON type of the source tag.
func TestCommandDocumentShape(t *testing.T) {
	conn := testConn()
	p := pt(4, "CMD1", "digital", "command", "T")
	p.SrcCommonAddress = "3"           // string, as an IEC 104 tag would have
	p.SrcObjectAddress = float64(1234) // double, as an OPC tag might have
	p.SrcCommandDuration = 2
	built := BuildModel([]*Point{p}, conn)

	g := &Gateway{conn: conn, built: built}
	drainCommands()

	mp := built.ByTag["CMD1"]
	cause := g.handleControl(mp, &server.ControlCtx{
		Ref:   mp.ObjRef,
		Value: mms.NewBool(true),
		Peer:  "10.1.2.3:45678",
	})
	if cause != model.AddCauseNone {
		t.Fatalf("control refused: %v", cause)
	}

	doc, ok := dequeueCommand()
	if !ok {
		t.Fatal("no command queued")
	}
	for _, f := range []string{
		"protocolSourceConnectionNumber", "protocolSourceCommonAddress",
		"protocolSourceObjectAddress", "protocolSourceASDU",
		"protocolSourceCommandDuration", "protocolSourceCommandUseSBO",
		"pointKey", "tag", "value", "valueString",
		"originatorUserName", "originatorIpAddress", "timeTag",
	} {
		if _, present := doc[f]; !present {
			t.Errorf("missing field %q", f)
		}
	}

	// Routing fields keep the type the source tag had.
	if s, isStr := doc["protocolSourceCommonAddress"].(string); !isStr || s != "3" {
		t.Errorf("common address = %#v, want the string \"3\"", doc["protocolSourceCommonAddress"])
	}
	if f, isNum := doc["protocolSourceObjectAddress"].(float64); !isNum || f != 1234 {
		t.Errorf("object address = %#v, want the double 1234", doc["protocolSourceObjectAddress"])
	}
	if doc["protocolSourceCommandDuration"] != float64(2) {
		t.Errorf("command duration = %#v", doc["protocolSourceCommandDuration"])
	}
	if doc["tag"] != "CMD1" || doc["value"] != float64(1) || doc["valueString"] != "true" {
		t.Errorf("value fields = %v / %v / %v", doc["tag"], doc["value"], doc["valueString"])
	}
	if doc["originatorUserName"] != "IEC61850 connection: 8001 IEC61850SRV" {
		t.Errorf("originatorUserName = %v", doc["originatorUserName"])
	}
	if doc["originatorIpAddress"] != "10.1.2.3:45678" {
		t.Errorf("originatorIpAddress = %v", doc["originatorIpAddress"])
	}

	// The select phase accepts but queues nothing.
	cause = g.handleControl(mp, &server.ControlCtx{Ref: mp.ObjRef, Value: mms.NewBool(true), Select: true})
	if cause != model.AddCauseNone {
		t.Errorf("select refused: %v", cause)
	}
	if _, queued := dequeueCommand(); queued {
		t.Error("the select phase must not queue a command")
	}

	// With commands disabled, controls are blocked.
	conn.CommandsEnabled = false
	if cause := g.handleControl(mp, &server.ControlCtx{Ref: mp.ObjRef, Value: mms.NewBool(true)}); cause != model.AddCauseBlockedByMode {
		t.Errorf("disabled-commands control cause = %v, want blocked-by-mode", cause)
	}
	if _, queued := dequeueCommand(); queued {
		t.Error("a blocked control must not queue a command")
	}
}

// kconv1/kconv2 of the command tag scale the value written to commandsQueue.
func TestConvertCommandValue(t *testing.T) {
	cases := []struct {
		name           string
		typ            string
		kconv1, kconv2 float64
		in             float64
		inString       string
		wantValue      float64
		wantString     string
	}{
		{"digital direct on", "digital", 1, 0, 1, "true", 1, "true"},
		{"digital direct off", "digital", 1, 0, 0, "false", 0, "false"},
		{"digital inverted on", "digital", -1, 0, 1, "true", 0, "false"},
		{"digital inverted off", "digital", -1, 0, 0, "false", 1, "true"},
		{"digital forced to 1", "digital", 1, 0, 42.5, "42.5", 1, "true"},
		{"digital inverted, forced to 0", "digital", -1, 0, 42.5, "42.5", 0, "false"},
		// kconv2 does not apply to a digital.
		{"digital ignores kconv2", "digital", 1, 100, 1, "true", 1, "true"},
		{"analog identity", "analog", 1, 0, 37.5, "37.5", 37.5, "37.5"},
		{"analog scaled", "analog", 10, 0, 37.5, "37.5", 375, "375"},
		{"analog offset", "analog", 1, -5, 37.5, "37.5", 32.5, "32.5"},
		{"analog scaled and offset", "analog", 0.5, 2, 10, "10", 7, "7"},
		{"analog inverted", "analog", -1, 0, 37.5, "37.5", -37.5, "-37.5"},
		// A string/json command passes through untouched.
		{"string untouched", "string", -1, 3, 5, "five", 5, "five"},
	}
	for _, c := range cases {
		mp := &MappedPoint{Tag: "T", Type: c.typ, Kconv1: c.kconv1, Kconv2: c.kconv2}
		v, s := convertCommandValue(mp, c.in, c.inString)
		if v != c.wantValue || s != c.wantString {
			t.Errorf("%s: (%v,%q), want (%v,%q)", c.name, v, s, c.wantValue, c.wantString)
		}
	}
}

// The conversion has to reach the queued document, not just the helper.
func TestCommandDocumentAppliesKconv(t *testing.T) {
	conn := testConn()
	dig := pt(4, "CMDD", "digital", "command", "T")
	dig.Kconv1 = -1
	ana := pt(5, "CMDA", "analog", "command", "T")
	ana.Kconv1, ana.Kconv2 = 2, 1
	built := BuildModel([]*Point{dig, ana}, conn)

	g := &Gateway{conn: conn, built: built}
	drainCommands()

	send := func(tag string, v *mms.Value) map[string]any {
		t.Helper()
		mp := built.ByTag[tag]
		if cause := g.handleControl(mp, &server.ControlCtx{Ref: mp.ObjRef, Value: v}); cause != model.AddCauseNone {
			t.Fatalf("%s: control refused: %v", tag, cause)
		}
		doc, ok := dequeueCommand()
		if !ok {
			t.Fatalf("%s: no command queued", tag)
		}
		return doc
	}

	// An inverted digital: ctlVal true has to reach the source driver as 0.
	doc := send("CMDD", mms.NewBool(true))
	if doc["value"] != float64(0) || doc["valueString"] != "false" {
		t.Errorf("inverted digital = %v / %v, want 0 / false", doc["value"], doc["valueString"])
	}

	// An analogue setpoint: 10 * 2 + 1 = 21.
	doc = send("CMDA", mms.NewStructure(mms.NewFloat32(10)))
	if doc["value"] != float64(21) || doc["valueString"] != "21" {
		t.Errorf("scaled analog = %v / %v, want 21 / 21", doc["value"], doc["valueString"])
	}
}

// A client cannot change a status point by sending it a control. A forged
// operate of a status object (GGIO1.Ind1 has no Oper) used to be taken for a
// direct control and written into its stVal, which every other client of the
// gateway would then see. It must be refused, the value left alone and
// nothing queued as a command.
func TestLoopbackForgedControlOfStatusPointIsRefused(t *testing.T) {
	points := loopbackPoints()
	gw := startGateway(t, testConn(), points)
	c := dialGateway(t, gw)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pushUpdate(gw, points[0], 1) // LB_DIG_1 -> GGIO1.Ind1 = true
	drainCommands()

	res, err := c.MMS().Write(ctx, "IEC61850SRVKAW2", []string{"GGIO1$CO$Ind1$Oper"},
		[]*mms.Value{mms.NewStructure(mms.NewBool(false))})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if len(res) == 0 || res[0] == nil {
		t.Error("a forged operate of a status point was accepted")
	}
	if v, err := c.Read(ctx, "IEC61850SRVKAW2/GGIO1.Ind1.stVal", model.ST); err != nil || !v.Bool() {
		t.Errorf("status value changed by a forged control: %v (%v)", v, err)
	}
	if _, queued := dequeueCommand(); queued {
		t.Error("a forged control queued a command")
	}
}
