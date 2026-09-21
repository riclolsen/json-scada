package jsrtdata

import (
	"sync"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestQueueFIFO(t *testing.T) {
	var q Queue[int]
	if _, ok := q.Dequeue(); ok {
		t.Error("empty queue must report false")
	}
	if q.Len() != 0 {
		t.Errorf("Len = %d, want 0", q.Len())
	}
	for i := 1; i <= 3; i++ {
		q.Enqueue(i)
	}
	if q.Len() != 3 {
		t.Errorf("Len = %d, want 3", q.Len())
	}
	for i := 1; i <= 3; i++ {
		v, ok := q.Dequeue()
		if !ok || v != i {
			t.Errorf("Dequeue = %v,%v, want %d,true", v, ok, i)
		}
	}
	if _, ok := q.Dequeue(); ok {
		t.Error("drained queue must report false")
	}
}

func TestQueueTrimDropsOldest(t *testing.T) {
	var q Queue[int]
	for i := 1; i <= 5; i++ {
		q.Enqueue(i)
	}
	dropped := 0
	q.Trim(2, func() { dropped++ })
	if dropped != 3 {
		t.Errorf("onDrop called %d times, want 3", dropped)
	}
	if q.Len() != 2 {
		t.Fatalf("Len = %d, want 2", q.Len())
	}
	// The values kept must be the newest.
	v, _ := q.Dequeue()
	if v != 4 {
		t.Errorf("oldest kept = %d, want 4", v)
	}
	q.Trim(10, nil) // no-op, and a nil callback must not panic
}

func TestQueueIsConcurrencySafe(t *testing.T) {
	var q Queue[int]
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				q.Enqueue(j)
			}
		}()
	}
	wg.Wait()
	if q.Len() != 1600 {
		t.Errorf("Len = %d, want 1600", q.Len())
	}
	for q.Len() > 0 {
		q.Dequeue()
	}
}

// The twelve base fields must always be present, and Extra must be able to
// add protocol-specific ones without disturbing them.
func TestSourceDataUpdateBaseFields(t *testing.T) {
	u := SourceDataUpdate{
		ValueAtSource:               1.5,
		ValueStringAtSource:         "1.5",
		AsduAtSource:                "Double",
		CauseOfTransmissionAtSource: "3",
		TimeTagAtSource:             nil,
		TimeTagAtSourceOk:           false,
		TimeTag:                     bson.DateTime(1),
		NotTopicalAtSource:          true,
		InvalidAtSource:             true,
		OverflowAtSource:            true,
		BlockedAtSource:             true,
		SubstitutedAtSource:         true,
	}
	doc := u.BSON()

	want := map[string]any{
		"valueAtSource":               1.5,
		"valueStringAtSource":         "1.5",
		"asduAtSource":                "Double",
		"causeOfTransmissionAtSource": "3",
		"timeTagAtSource":             nil,
		"timeTagAtSourceOk":           false,
		"timeTag":                     bson.DateTime(1),
		"notTopicalAtSource":          true,
		"invalidAtSource":             true,
		"overflowAtSource":            true,
		"blockedAtSource":             true,
		"substitutedAtSource":         true,
	}
	if len(doc) != len(want) {
		t.Errorf("field count = %d, want %d: %v", len(doc), len(want), doc)
	}
	for k, v := range want {
		got, ok := doc[k]
		if !ok {
			t.Errorf("missing field %q", k)
			continue
		}
		if got != v {
			t.Errorf("%s = %v (%T), want %v (%T)", k, got, got, v, v)
		}
	}
}

func TestSourceDataUpdateExtraMerges(t *testing.T) {
	u := SourceDataUpdate{
		AsduAtSource: "x",
		Extra: bson.M{
			"originator":        "DNP3|1",
			"carryAtSource":     true,
			"transientAtSource": false,
		},
	}
	doc := u.BSON()
	if doc["originator"] != "DNP3|1" || doc["carryAtSource"] != true {
		t.Errorf("Extra not merged: %v", doc)
	}
	if doc["asduAtSource"] != "x" {
		t.Error("Extra must not disturb the base fields")
	}
	if len(doc) != 15 {
		t.Errorf("field count = %d, want 12 base + 3 extra", len(doc))
	}
}

// Extra deliberately wins, so a driver can override a base field if its
// original document did.
func TestSourceDataUpdateExtraOverridesBase(t *testing.T) {
	u := SourceDataUpdate{
		InvalidAtSource: false,
		Extra:           bson.M{"invalidAtSource": true},
	}
	if u.BSON()["invalidAtSource"] != true {
		t.Error("Extra should override the base field")
	}
}

func TestSetDocShape(t *testing.T) {
	set, ok := SourceDataUpdate{AsduAtSource: "a"}.SetDoc()["$set"].(bson.M)
	if !ok {
		t.Fatal("SetDoc must produce a $set document")
	}
	if _, ok := set["sourceDataUpdate"].(bson.M); !ok {
		t.Errorf("$set must carry sourceDataUpdate, got %v", set)
	}
}

func TestSupervisedFilterPinsOrigin(t *testing.T) {
	f := SupervisedFilter(float64(3), "ns=2;i=7")
	if f["origin"] != "supervised" {
		t.Error("the origin clause is what keeps a command tag from matching")
	}
	if f["protocolSourceConnectionNumber"] != float64(3) {
		t.Errorf("connection number = %v", f["protocolSourceConnectionNumber"])
	}
	if f["protocolSourceObjectAddress"] != "ns=2;i=7" {
		t.Errorf("object address = %v", f["protocolSourceObjectAddress"])
	}
	if len(f) != 3 {
		t.Errorf("filter has %d keys, want 3", len(f))
	}
}

func TestOversize(t *testing.T) {
	small := bson.M{"$set": bson.M{"a": "short"}}
	if over, _ := Oversize(small, 10); over {
		t.Error("a small update must not be reported oversize")
	}

	// Below the pre-test threshold the encoding is never examined, even for a
	// document that would exceed the limit.
	if over, _ := Oversize(small, oversizePretestBytes); over {
		t.Error("pre-test threshold is exclusive")
	}

	// Past the pre-test, a merely long string still gets through: the
	// decision is the encoded size, not the rendered length.
	long := bson.M{"$set": bson.M{"a": string(make([]byte, 2_000_000))}}
	if over, size := Oversize(long, 2_000_000); over {
		t.Errorf("2 MB encodes to %d bytes and must still be written", size)
	}

	huge := bson.M{"$set": bson.M{"a": string(make([]byte, MaxBSONDocumentSize+1000))}}
	over, size := Oversize(huge, MaxBSONDocumentSize+1000)
	if !over {
		t.Errorf("a document over the limit must be reported, size=%d", size)
	}
}
