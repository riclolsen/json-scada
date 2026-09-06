package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Change-detectors for the two documents the rest of JSON-SCADA reads: the
// automatically created tag and the sourceDataUpdate written for every
// acquired value. They pin every field so a refactor meant to be
// behaviour-preserving can be shown to be one.
//
// Regenerate deliberately, and review the diff:
//
//	go test -run TestGolden -update

var updateGolden = flag.Bool("update", false, "rewrite the golden files")

// canonical renders a document as sorted type-tagged key=value lines, so a
// mismatch names the field and shows a type change as clearly as a value one.
func canonical(doc map[string]any) string {
	keys := make([]string, 0, len(doc))
	for k := range doc {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		v := doc[k]
		if f, ok := v.(float64); ok {
			if f == math.MaxFloat64 {
				fmt.Fprintf(&b, "%s=float64:+MAXFLOAT\n", k)
				continue
			}
			if f == -math.MaxFloat64 {
				fmt.Fprintf(&b, "%s=float64:-MAXFLOAT\n", k)
				continue
			}
		}
		enc, err := json.Marshal(v)
		if err != nil {
			fmt.Fprintf(&b, "%s=%T:<unencodable>\n", k, v)
			continue
		}
		fmt.Fprintf(&b, "%s=%T:%s\n", k, v, enc)
	}
	return b.String()
}

func firstDiff(want, got string) string {
	w := strings.Split(want, "\n")
	g := strings.Split(got, "\n")
	for i := 0; i < len(w) || i < len(g); i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			return fmt.Sprintf("first difference at line %d:\n  want: %q\n  got:  %q", i+1, wl, gl)
		}
	}
	return "(no line differs; trailing whitespace?)"
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
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
		t.Fatalf("%v (run: go test -run TestGolden -update)", err)
	}
	if got != string(want) {
		t.Errorf("%s changed.\n%s", name, firstDiff(string(want), got))
	}
}

func TestGoldenTagDocs(t *testing.T) {
	base := IECValue{
		ConnNumber: 4, ConnName: "CONN", Address: "IED1/MMXU1.A.phsA.cVal.mag.f",
		Asdu: "float32", CommonAddress: "mx", DisplayName: "Phase A current",
		Value: 12.5, ValueString: "12.5", ValueJSON: "12.5",
	}
	digital := base
	digital.IsDigital = true
	digital.Asdu = "boolean"

	cmdAnalog := CommandTag{
		ConnNumber: 4, ConnName: "CONN", Ref: "IED1/CSWI1.Pos",
		IsDigital: false, UseSBO: true, Asdu: "int32",
	}
	cmdDigital := cmdAnalog
	cmdDigital.IsDigital = true
	cmdDigital.Asdu = "boolean"

	var b strings.Builder
	fmt.Fprintf(&b, "=== supervisedAnalog\n%s\n", canonical(newRealtimeDoc(base, 4000001)))
	fmt.Fprintf(&b, "=== supervisedDigital\n%s\n", canonical(newRealtimeDoc(digital, 4000002)))
	fmt.Fprintf(&b, "=== commandAnalog\n%s\n", canonical(newCommandDoc(cmdAnalog, 4000003, 0)))
	fmt.Fprintf(&b, "=== commandDigital\n%s\n", canonical(newCommandDoc(cmdDigital, 4000004, 4000002)))
	checkGolden(t, "tagdoc.golden", b.String())
}

func TestGoldenUpdateModel(t *testing.T) {
	ts := time.Date(2026, 3, 4, 5, 6, 7, 80000000, time.UTC)
	withSource := IECValue{
		ConnNumber: 4, ConnName: "CONN", Address: "IED1/MMXU1.A.phsA.cVal.mag.f",
		Asdu: "float32", Value: 12.5, ValueString: "12.5", ValueJSON: "12.5",
		Cot: 3, Quality: true, HasSourceTimestamp: true,
		SourceTimestamp: ts, ServerTimestamp: ts.Add(time.Second),
	}
	noSource := withSource
	noSource.HasSourceTimestamp = false
	badQuality := withSource
	badQuality.Quality = false
	transient := withSource
	transient.IsTransient = true

	cases := []struct {
		name string
		v    IECValue
	}{
		{"withSourceTimestamp", withSource},
		{"noSourceTimestamp", noSource},
		{"badQuality", badQuality},
		{"transient", transient},
	}

	var b strings.Builder
	for _, c := range cases {
		wm := updateModel(c.v)
		m, ok := wm.(*mongo.UpdateOneModel)
		if !ok {
			t.Fatalf("expected *mongo.UpdateOneModel, got %T", wm)
		}
		set := m.Update.(bson.M)["$set"].(bson.M)
		fmt.Fprintf(&b, "=== %s\nfilter:\n%ssourceDataUpdate:\n%s\n",
			c.name, canonical(m.Filter.(bson.M)), canonical(set["sourceDataUpdate"].(bson.M)))
	}
	checkGolden(t, "updatemodel.golden", b.String())
}
