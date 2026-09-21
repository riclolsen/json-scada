/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 */

package main

import (
	"testing"
	"time"
)

// A discovery that keeps failing must not hammer the server: every failed
// attempt can leave a session behind, and some servers then refuse new ones.
func TestDiscoveryBackoffGrowsAndIsCapped(t *testing.T) {
	first := discoveryBackoff(1)
	if first != retryPeriod {
		t.Errorf("first retry waits %v, want %v", first, retryPeriod)
	}

	prev := first
	for failures := 2; failures <= 8; failures++ {
		got := discoveryBackoff(failures)
		if got < prev {
			t.Errorf("backoff shrank at %d failures: %v after %v", failures, got, prev)
		}
		if got > maxDiscoveryBackoff {
			t.Errorf("backoff at %d failures is %v, above the %v cap", failures, got, maxDiscoveryBackoff)
		}
		prev = got
	}

	if discoveryBackoff(100) != maxDiscoveryBackoff {
		t.Errorf("backoff must settle at the cap, got %v", discoveryBackoff(100))
	}
	// A zero or negative count must still produce a usable wait.
	if got := discoveryBackoff(0); got != retryPeriod {
		t.Errorf("backoff(0) = %v, want %v", got, retryPeriod)
	}
}

func TestDiscoveryBackoffReachesTheCapEventually(t *testing.T) {
	// Ten failures is a little over eight minutes of doubling from 5 s, so
	// the cap must already be in force.
	if got := discoveryBackoff(10); got != maxDiscoveryBackoff {
		t.Errorf("backoff(10) = %v, want the %v cap", got, maxDiscoveryBackoff)
	}
	if maxDiscoveryBackoff <= retryPeriod {
		t.Fatal("the cap must be longer than a single retry period")
	}
	if maxDiscoveryBackoff > 15*time.Minute {
		t.Error("a cap this long would leave a recovered server unattended for too long")
	}
}

// A server that drops the connection rather than answering a large value
// read would fail identically forever unless the driver asks for less.
func TestShrinkValueReadChunk(t *testing.T) {
	conn := testConn()

	if got := conn.ValueReadChunk(); got != maxNodesToRead {
		t.Fatalf("initial chunk = %d, want %d", got, maxNodesToRead)
	}

	prev := conn.ValueReadChunk()
	for i := 0; i < 20; i++ {
		got, shrank := conn.ShrinkValueReadChunk()
		if !shrank {
			if got != minValueReadChunk {
				t.Errorf("stopped shrinking at %d, want the %d floor", got, minValueReadChunk)
			}
			break
		}
		if got >= prev {
			t.Fatalf("chunk did not shrink: %d after %d", got, prev)
		}
		if got < minValueReadChunk {
			t.Fatalf("chunk %d went below the %d floor", got, minValueReadChunk)
		}
		prev = got
	}

	// Once at the floor it must stay there and keep reporting that there is
	// nothing left to give.
	for i := 0; i < 3; i++ {
		got, shrank := conn.ShrinkValueReadChunk()
		if shrank {
			t.Errorf("shrank past the floor to %d", got)
		}
		if got != minValueReadChunk {
			t.Errorf("chunk = %d, want the %d floor", got, minValueReadChunk)
		}
	}
}

// The shrunken size has to actually reach the read, or the retry changes
// nothing.
func TestValueReadChunkIsUsed(t *testing.T) {
	cli, _, _ := startTestServer(t)
	conn := testConn()

	for i := 0; i < 3; i++ {
		conn.ShrinkValueReadChunk()
	}
	want := conn.ValueReadChunk()
	if want >= maxNodesToRead {
		t.Fatalf("chunk did not shrink: %d", want)
	}

	// Reading fewer nodes than the chunk size must still work; this is the
	// path every server takes once the driver has backed off.
	addrs := []string{"ns=1;s=Boiler.Temp", "ns=1;s=Boiler.Text"}
	values, err := readNodeValues(t.Context(), cli, addrs, 0, want)
	if err != nil {
		t.Fatalf("read with a shrunken chunk failed: %v", err)
	}
	if len(values) != len(addrs) {
		t.Errorf("got %d values for %d nodes", len(values), len(addrs))
	}
}
