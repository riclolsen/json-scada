package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// updateModel writes every acquired value and had no test before the
// go-common migration. This pins the filter and the sourceDataUpdate
// sub-document, which the rest of JSON-SCADA reads.
//
// Regenerate deliberately, and review the diff:
//
//	go test -run TestGoldenUpdateModel -update

func renderWriteModel(t *testing.T, wm mongo.WriteModel) string {
	t.Helper()
	if wm == nil {
		return "<nil>\n"
	}
	m, ok := wm.(*mongo.UpdateOneModel)
	if !ok {
		t.Fatalf("expected *mongo.UpdateOneModel, got %T", wm)
	}
	filter, ok := m.Filter.(bson.M)
	if !ok {
		t.Fatalf("expected bson.M filter, got %T", m.Filter)
	}
	update, ok := m.Update.(bson.M)
	if !ok {
		t.Fatalf("expected bson.M update, got %T", m.Update)
	}
	set, ok := update["$set"].(bson.M)
	if !ok {
		t.Fatalf(`expected a $set document, got %T`, update["$set"])
	}
	sdu, ok := set["sourceDataUpdate"].(bson.M)
	if !ok {
		t.Fatalf("expected a sourceDataUpdate document, got %T", set["sourceDataUpdate"])
	}

	var b strings.Builder
	b.WriteString("filter:\n")
	b.WriteString(canonical(filter))
	b.WriteString("sourceDataUpdate:\n")
	b.WriteString(canonical(sdu))
	if len(set) != 1 {
		b.WriteString("EXTRA $set KEYS:\n")
		for k := range set {
			if k != "sourceDataUpdate" {
				fmt.Fprintf(&b, "  %s\n", k)
			}
		}
	}
	return b.String()
}

func TestGoldenUpdateModel(t *testing.T) {
	ts := time.Date(2026, 3, 4, 5, 6, 7, 80000000, time.UTC)

	withSource := OPCValue{
		ConnNumber: 3, ConnName: "CONN", Address: "ns=2;i=7", Asdu: "Double",
		Value: 1.5, ValueString: "1.5", ValueJSON: "1.5", Cot: 3,
		Quality: true, HasSourceTimestamp: true,
		SourceTimestamp: ts, ServerTimestamp: ts.Add(time.Second),
	}
	noSource := withSource
	noSource.HasSourceTimestamp = false
	badQuality := withSource
	badQuality.Quality = false
	noJSON := withSource
	noJSON.ValueJSON = ""

	cases := []struct {
		name string
		v    OPCValue
	}{
		{"withSourceTimestamp", withSource},
		{"noSourceTimestamp", noSource},
		{"badQuality", badQuality},
		{"emptyValueJSON", noJSON},
	}

	var b strings.Builder
	for _, c := range cases {
		fmt.Fprintf(&b, "=== %s\n%s\n", c.name, renderWriteModel(t, updateModel(c.v)))
	}
	got := b.String()

	path := filepath.Join("testdata", "updatemodel.golden")
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", path)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run: go test -run TestGoldenUpdateModel -update)", err)
	}
	if got != string(want) {
		t.Errorf("update documents changed.\n%s", firstDiff(string(want), got))
	}
}
