package jscommands

import (
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestInsertOnlyPipeline(t *testing.T) {
	p := InsertOnlyPipeline()
	if len(p) != 1 {
		t.Fatalf("pipeline has %d stages, want 1", len(p))
	}
	stage := p[0]
	if len(stage) != 1 || stage[0].Key != "$match" {
		t.Fatalf("stage = %v, want a single $match", stage)
	}
	match, ok := stage[0].Value.(bson.D)
	if !ok || len(match) != 1 {
		t.Fatalf("match = %v", stage[0].Value)
	}
	if match[0].Key != "operationType" || match[0].Value != "insert" {
		t.Errorf("match = %v, want operationType: insert", match)
	}
}

func TestFieldsFromDocReadsPermissively(t *testing.T) {
	id := bson.NewObjectID()
	tt := time.Now().Truncate(time.Millisecond)
	doc := bson.M{
		"_id": id,
		// deliberately mixed types, as hand-edited documents carry
		"protocolSourceConnectionNumber": float64(3),
		"protocolSourceObjectAddress":    "ns=2;i=7",
		"protocolSourceASDU":             "Double",
		"value":                          "12.5", // a numeric string
		"valueString":                    "12.5",
		"protocolSourceCommandUseSBO":    int32(1), // a number for a bool
		"protocolSourceCommandDuration":  int64(3),
		"timeTag":                        bson.NewDateTimeFromTime(tt),
	}
	f := FieldsFromDoc(doc)
	if f.ID != id {
		t.Errorf("ID = %v", f.ID)
	}
	if f.ConnNumber != 3 {
		t.Errorf("ConnNumber = %d", f.ConnNumber)
	}
	if f.Address != "ns=2;i=7" || f.Asdu != "Double" {
		t.Errorf("Address/Asdu = %q/%q", f.Address, f.Asdu)
	}
	if f.Value != 12.5 {
		t.Errorf("Value = %v, want the numeric string coerced", f.Value)
	}
	if !f.UseSBO {
		t.Error("UseSBO should read 1 as true")
	}
	if f.Duration != 3 {
		t.Errorf("Duration = %d", f.Duration)
	}
	if !f.TimeTag.Equal(tt) {
		t.Errorf("TimeTag = %v, want %v", f.TimeTag, tt)
	}
}

func TestFieldsFromEmptyDoc(t *testing.T) {
	f := FieldsFromDoc(bson.M{})
	if f.ConnNumber != 0 || f.Address != "" || f.Value != 0 || !f.TimeTag.IsZero() {
		t.Errorf("empty document should give zero values: %+v", f)
	}
}

func TestExpired(t *testing.T) {
	fresh := Fields{TimeTag: time.Now()}
	if fresh.Expired(DefaultExpiry) {
		t.Error("a command queued now is not expired")
	}
	stale := Fields{TimeTag: time.Now().Add(-30 * time.Second)}
	if !stale.Expired(DefaultExpiry) {
		t.Error("a 30 s old command is expired at the 10 s default")
	}
	if stale.Age() < 30*time.Second {
		t.Errorf("Age = %v", stale.Age())
	}
}

// A missing timeTag decodes as the zero time, which is far in the past, so
// the command reads as expired. That is the safe direction: a command with no
// timestamp is refused rather than acted on.
func TestMissingTimeTagCountsAsExpired(t *testing.T) {
	if !(Fields{}).Expired(DefaultExpiry) {
		t.Error("a command with no timeTag must be treated as expired")
	}
}

func TestCancelDoc(t *testing.T) {
	set, ok := CancelDoc("expired")["$set"].(bson.M)
	if !ok {
		t.Fatal("CancelDoc must produce a $set document")
	}
	if len(set) != 1 || set["cancelReason"] != "expired" {
		t.Errorf("CancelDoc = %v, want only cancelReason", set)
	}
}

func TestAckDoc(t *testing.T) {
	set, ok := AckDoc(true, "ok")["$set"].(bson.M)
	if !ok {
		t.Fatal("AckDoc must produce a $set document")
	}
	if set["delivered"] != true {
		t.Errorf("delivered = %v", set["delivered"])
	}
	if set["ack"] != true {
		t.Errorf("ack = %v", set["ack"])
	}
	if set["resultDescription"] != "ok" {
		t.Errorf("resultDescription = %v", set["resultDescription"])
	}
	if _, ok := set["ackTimeTag"].(bson.DateTime); !ok {
		t.Errorf("ackTimeTag = %T, want bson.DateTime", set["ackTimeTag"])
	}
	if len(set) != 4 {
		t.Errorf("AckDoc has %d fields, want 4", len(set))
	}

	// A refused command still records delivered:true with ack:false, which is
	// how the viewers tell "reached the device and failed" from "never sent".
	set, _ = AckDoc(false, "failed")["$set"].(bson.M)
	if set["delivered"] != true || set["ack"] != false {
		t.Errorf("a failed ack must still be delivered: %v", set)
	}
}
