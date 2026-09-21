package clientapp

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

	"dnp3-go/internal/dnp3util"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Change-detector for the automatically created tag document, which the rest
// of JSON-SCADA reads and which had no test before the go-common migration.
//
// Regenerate deliberately, and review the diff:
//
//	go test ./internal/clientapp -run TestGoldenTagDoc -update

var updateGolden = flag.Bool("update", false, "rewrite the golden files")

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

func TestGoldenTagDoc(t *testing.T) {
	analog := Dnp3Value{
		ConnNumber: 2, Address: 11, BaseGroup: dnp3util.GroupAnalogInput,
		Group: 30, Variation: 1, COT: 3, Value: 12.5, ValueString: "12.5",
	}
	digital := analog
	digital.BaseGroup = dnp3util.GroupBinaryInput
	digital.Value = 1

	var b strings.Builder
	fmt.Fprintf(&b, "=== analogSupervised\n%s\n",
		canonical(newTagDoc(analog, "CONN", 2000001, false, dnp3util.GroupAnalogInput, 30, 0, 0, 0)))
	fmt.Fprintf(&b, "=== digitalSupervised\n%s\n",
		canonical(newTagDoc(digital, "CONN", 2000002, false, dnp3util.GroupBinaryInput, 1, 0, 0, 0)))
	fmt.Fprintf(&b, "=== analogCommand\n%s\n",
		canonical(newTagDoc(analog, "CONN", 2000003, true, dnp3util.GroupAnalogOutputBlock, 3, 0, 0, 2000004)))
	fmt.Fprintf(&b, "=== digitalCommand\n%s\n",
		canonical(newTagDoc(digital, "CONN", 2000005, true, dnp3util.GroupCROBCommand, 1, 3, 0, 2000006)))
	got := b.String()

	path := filepath.Join("testdata", "tagdoc.golden")
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
		t.Fatalf("%v (run with -update)", err)
	}
	if got != string(want) {
		t.Errorf("tag documents changed.\n%s", firstDiff(string(want), got))
	}
}

var _ = bson.M{}
