package main

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dscsystems/go-iec61850/client"
	"github.com/dscsystems/go-iec61850/mms"
	"github.com/dscsystems/go-iec61850/model"
)

// A buffered block of the sample model (indexed, so the instance is 01).
const sampleBRCB = model.ObjectReference("simpleIOGenericIO/LLN0.BR.EventsBRCB01")

// No EntryID is written for a block the driver has never seen a report on:
// all zeros is refused by some IEDs, and there is nothing to resume after.
func TestResyncEntryID(t *testing.T) {
	conn := &Iec61850Connection{}
	if id := resyncEntryID(conn, "rcb"); id != nil {
		t.Errorf("nothing saved: resync = %x, want none", id)
	}
	conn.SetLastReportID("rcb", make([]byte, 8))
	if id := resyncEntryID(conn, "rcb"); id != nil {
		t.Errorf("all zeros saved: resync = %x, want none", id)
	}
	saved := []byte{0, 0, 0, 0, 0, 0, 0, 0x30}
	conn.SetLastReportID("rcb", saved)
	if id := resyncEntryID(conn, "rcb"); !bytes.Equal(id, saved) {
		t.Errorf("resync = %x, want the saved %x", id, saved)
	}
}

// An EntryID the IED no longer buffers (it restarted) is refused. The block
// must still come up, resuming from the IED's position, and the stale ID
// must not be kept for the next attempt.
func TestLoopbackStaleEntryIDResumesWithoutResync(t *testing.T) {
	addr, _ := startTestIED(t)
	conn := newTestConnection(addr)
	redundancy.ForceActive(true)
	defer redundancy.ForceActive(false)
	connectTest(t, conn)
	drainQueue()
	defer drainQueue()

	stale := []byte{0, 0, 0, 0, 0, 0, 0x7f, 0x7f}
	conn.SetLastReportID(string(sampleBRCB), stale)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	enableRCB(ctx, conn, sampleBRCB, true)

	if len(conn.Subs) != 1 {
		t.Fatalf("subscriptions = %d, want the block enabled without resync", len(conn.Subs))
	}
	if id, ok := conn.LastReportID(string(sampleBRCB)); ok && bytes.Equal(id, stale) {
		t.Error("the refused EntryID was kept")
	}
	values := waitForValues(t, 10*time.Second, func(v []IECValue) bool {
		return hasAddressPrefix(v, "simpleIOGenericIO/GGIO1")
	})
	if !hasAddressPrefix(values, "simpleIOGenericIO/GGIO1") {
		t.Error("no report arrived after enabling without resync")
	}
}

// A block another client holds refuses every write, the disable included,
// and the refusal comes back per item, not as the call's error. The driver
// must see it and give up, leaving the data set to another block.
func TestLoopbackRCBHeldByAnotherClient(t *testing.T) {
	addr, _ := startTestIED(t)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	other, err := client.Dial(ctx, addr, client.WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("dial the other client: %v", err)
	}
	defer other.Close()
	held, err := other.GetRCB(ctx, sampleBRCB)
	if err != nil {
		t.Fatalf("GetRCB: %v", err)
	}
	held.Buffered = true
	if _, err := other.EnableReporting(ctx, held, func(*client.Report) {}); err != nil {
		t.Fatalf("the other client could not take the block: %v", err)
	}

	conn := newTestConnection(addr)
	redundancy.ForceActive(true)
	defer redundancy.ForceActive(false)
	connectTest(t, conn)
	drainQueue()
	defer drainQueue()

	domain, item := rcbToMMS(sampleBRCB)
	derr := disableRCB(ctx, conn.Client(), domain, item)
	if !errors.Is(derr, mms.AccessTemporarilyUnavailable) {
		t.Errorf("disable of a held block = %v, want the per-item temporarily-unavailable", derr)
	}

	enableRCB(ctx, conn, sampleBRCB, true)
	if len(conn.Subs) != 0 {
		t.Errorf("subscriptions = %d, want none on a held block", len(conn.Subs))
	}
	if len(conn.RcbByDataSet) != 0 {
		t.Errorf("data set still claimed by the held block: %v", conn.RcbByDataSet)
	}
}

// What the driver asks of a block before enabling it. The buffer time read
// from the IED is not written back: the library writes any non-zero BufTm,
// and an IED with fixed report settings may refuse it.
func TestConfigureRCB(t *testing.T) {
	conn := &Iec61850Connection{Class0ScanInterval: 30}
	rcb := &client.RCB{BufTm: 100} // as GetRCB read it
	configureRCB(conn, rcb, "rcb", true)
	if rcb.BufTm != 0 {
		t.Errorf("BufTm = %d, want 0 so the IED keeps its own", rcb.BufTm)
	}
	if rcb.TrgOps&model.TrgGI == 0 {
		t.Error("TrgOps lacks GI")
	}
	if rcb.IntgPd != 30*time.Second {
		t.Errorf("IntgPd = %v, want the class 0 scan interval", rcb.IntgPd)
	}
	if rcb.OptFlds&model.OptEntryID == 0 {
		t.Error("a buffered block must ask for the EntryID")
	}
	if rcb.ResyncEntryID != nil {
		t.Errorf("ResyncEntryID = %x with nothing saved, want none", rcb.ResyncEntryID)
	}
}
