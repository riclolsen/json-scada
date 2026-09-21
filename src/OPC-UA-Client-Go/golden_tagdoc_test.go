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
)

// This is a change-detector on purpose. Automatically created tag documents
// are read by the rest of JSON-SCADA, and a field that silently changes type,
// value or disappears shows up as a blank column in a viewer rather than as
// an error. The golden file pins every field of every shape so a refactor
// that was meant to be behaviour-preserving can be shown to be one.
//
// Regenerate deliberately, and review the diff:
//
//	go test -run TestGoldenTagDoc -update

var updateGolden = flag.Bool("update", false, "rewrite the tag document golden file")

// canonical renders a tag document as sorted type-tagged key=value lines, so
// a mismatch names the field and shows a type change as clearly as a value
// change.
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

// goldenSamples is one tag document per shape this driver builds.
func goldenSamples() (names []string, docs map[string]map[string]any) {
	base := OPCValue{
		ConnNumber: 3, ConnName: "CONN", Address: "ns=2;i=7", Path: "Objects/Dev/Tag",
		DisplayName: "Tag", Asdu: "Double", CommonAddress: "CA",
		Value: 1.5, ValueString: "1.5", ValueJSON: "1.5", AccessLevels: 3,
	}
	digital := base
	digital.Asdu = "Boolean"
	str := base
	str.Asdu = "String"
	js := base
	js.Asdu = "NodeId"
	arr := base
	arr.IsArray = true
	cmd := base
	cmd.CreateCommandForSupervised = true
	method := base
	method.CreateCommandForMethod = true
	cmdDigital := digital
	cmdDigital.CreateCommandForSupervised = true

	return []string{"analog", "digital", "string", "json", "array", "command", "method", "commandDigital"},
		map[string]map[string]any{
			"analog":         newRealtimeDoc(base, 3000001, 0),
			"digital":        newRealtimeDoc(digital, 3000002, 0),
			"string":         newRealtimeDoc(str, 3000003, 0),
			"json":           newRealtimeDoc(js, 3000004, 0),
			"array":          newRealtimeDoc(arr, 3000005, 0),
			"command":        newRealtimeDoc(cmd, 3000006, 0),
			"method":         newRealtimeDoc(method, 3000007, 0),
			"commandDigital": newRealtimeDoc(cmdDigital, 3000008, 42),
		}
}

func TestGoldenTagDoc(t *testing.T) {
	names, docs := goldenSamples()
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "=== %s\n%s\n", n, canonical(docs[n]))
	}
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
		t.Fatalf("%v (run: go test -run TestGoldenTagDoc -update)", err)
	}
	if got != string(want) {
		t.Errorf("tag documents changed.\n%s", firstDiff(string(want), got))
	}
}

// firstDiff reports the first differing line, which is what a reviewer needs.
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
