/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package serverapp

import (
	"fmt"
	"strings"
	"testing"
)

// The first stage must not reference fullDocument, so the server applies it
// before the updateLookup; it carries the mapped _ids. A changed tag set
// asks the stream to reopen, an unchanged one does not.
func TestWatcherPipelineAndRestart(t *testing.T) {
	w := &Watcher{Cfg: Config{ProtocolConnectionNumber: 3101}, restart: make(chan struct{}, 1)}

	p := w.pipeline()
	if len(p) != 2 {
		t.Fatalf("stages = %d, want 2", len(p))
	}
	first := fmt.Sprint(p[0][0].Value)
	if strings.Contains(first, "fullDocument") {
		t.Errorf("first stage references fullDocument: %s", first)
	}
	if !strings.Contains(fmt.Sprint(p[1][0].Value), "3101") {
		t.Errorf("second stage does not filter the connection: %v", p[1][0].Value)
	}

	w.setIDs([]TagDoc{{ID: 7.0}, {ID: 9.0}})
	select {
	case <-w.restart:
	default:
		t.Fatal("a new tag set must restart the stream")
	}
	if s := fmt.Sprint(w.pipeline()[0][0].Value); !strings.Contains(s, `"documentKey._id":{"$in":[{"$numberDouble":"7.0"},{"$numberDouble":"9.0"}]}`) {
		t.Errorf("first stage does not carry the mapped _ids: %s", s)
	}
	w.setIDs([]TagDoc{{ID: 7.0}, {ID: 9.0}})
	select {
	case <-w.restart:
		t.Fatal("an unchanged tag set must not restart the stream")
	default:
	}
}
