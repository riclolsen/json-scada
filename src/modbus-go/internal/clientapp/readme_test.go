/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package clientapp

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestReadmeDocumentsEveryConfigKey fails when config.go reads a key that the
// driver README does not name.
func TestReadmeDocumentsEveryConfigKey(t *testing.T) {
	src, err := os.ReadFile("config.go")
	if err != nil {
		t.Fatal(err)
	}
	readme, err := os.ReadFile("../../cmd/modbusclient/README.md")
	if err != nil {
		t.Fatal(err)
	}
	keys := regexp.MustCompile(`\(doc, "([A-Za-z0-9]+)"`).FindAllStringSubmatch(string(src), -1)
	if len(keys) < 20 {
		t.Fatalf("only %d keys found in config.go", len(keys))
	}
	for _, k := range keys {
		if !strings.Contains(string(readme), "`"+k[1]+"`") {
			t.Errorf("config key %s is not documented in the README", k[1])
		}
	}
}
