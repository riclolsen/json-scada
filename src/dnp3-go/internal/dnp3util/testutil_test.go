package dnp3util

import (
	"testing"
	"time"
)

// lockable is a mutex a handler running on the session goroutine can share with
// a test goroutine.
type lockable chan struct{}

func newLockable() lockable {
	l := make(lockable, 1)
	l <- struct{}{}
	return l
}

func (l lockable) Lock()   { <-l }
func (l lockable) Unlock() { l <- struct{}{} }

// waitForCond polls until cond holds or the deadline passes.
func waitForCond(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
