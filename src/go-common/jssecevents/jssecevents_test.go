package jssecevents

import (
	"net/netip"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTrackerReportsOnlyChanges(t *testing.T) {
	var tr Tracker
	tr.Seed("10.0.0.1", "aa")
	if _, ch := tr.Observe("10.0.0.1", "aa"); ch {
		t.Error("same value reported as a change")
	}
	if prev, ch := tr.Observe("10.0.0.1", "bb"); !ch || prev != "aa" {
		t.Errorf("change not reported: %q %v", prev, ch)
	}
	if _, ch := tr.Observe("10.0.0.1", ""); ch {
		t.Error("an unknown value reported as a change")
	}
	if _, ch := tr.Observe("10.0.0.2", "cc"); ch {
		t.Error("a first value reported as a change")
	}
}

func TestRateLimitReportsOncePerWindow(t *testing.T) {
	r := RateLimit{Max: 3, Window: 10 * time.Second}
	now := time.Unix(1000, 0)
	reports := 0
	for i := 0; i < 10; i++ {
		if ok, _ := r.Hit("c", now.Add(time.Duration(i)*time.Second)); ok {
			reports++
		}
	}
	if reports != 1 {
		t.Errorf("%d reports in one window, want 1", reports)
	}
	if ok, _ := r.Hit("c", now.Add(11*time.Second)); ok {
		t.Error("the first hit of a new window reported")
	}
}

func TestMACLookup(t *testing.T) {
	p := filepath.Join(t.TempDir(), "arp")
	_ = os.WriteFile(p, []byte("IP address       HW type     Flags       HW address            Mask     Device\n"+
		"192.168.1.10     0x1         0x2         00:1A:2b:3c:4d:5e     *        eth0\n"+
		"192.168.1.11     0x1         0x0         00:00:00:00:00:00     *        eth0\n"), 0o600)
	if m := lookupProcARP(netip.MustParseAddr("192.168.1.10"), p); m != "00:1a:2b:3c:4d:5e" {
		t.Errorf("got %q", m)
	}
	if m := lookupProcARP(netip.MustParseAddr("192.168.1.11"), p); m != "" {
		t.Errorf("incomplete entry gave %q", m)
	}
	win := "\nInterface: 192.168.1.2 --- 0x7\n  Internet Address      Physical Address      Type\n" +
		"  192.168.1.10          00-1a-2b-3c-4d-5e     dynamic\n  192.168.1.100         00-1a-2b-3c-4d-ff     dynamic\n"
	if m := parseARPOutput(netip.MustParseAddr("192.168.1.10"), win); m != "00:1a:2b:3c:4d:5e" {
		t.Errorf("windows: got %q", m)
	}
	mac := "? (192.168.1.10) at 0:1a:2b:3c:4d:5e on en0 ifscope [ethernet]\n"
	if m := parseARPOutput(netip.MustParseAddr("192.168.1.10"), mac); m != "00:1a:2b:3c:4d:5e" {
		t.Errorf("macos: got %q", m)
	}
	if a := SplitAddr("[::ffff:10.1.2.3]:102"); a.String() != "10.1.2.3" {
		t.Errorf("SplitAddr: %v", a)
	}
}

func TestEmitCompletesAndNeverBlocks(t *testing.T) {
	w := NewWriter(1)
	o := Origin{ProtocolDriver: "IEC61850", ProtocolDriverInstanceNumber: 1, ProtocolConnectionNumber: 7, NodeName: "n"}
	w.Emit(o, Event{Kind: "conn.unexpected", Severity: "warning"})
	w.Emit(o, Event{Kind: "x"}) // full: dropped, not blocked
	if w.Dropped() != 1 {
		t.Errorf("dropped %d, want 1", w.Dropped())
	}
	e := <-w.ch
	if e.SeverityLevel != 2 || e.Count != 1 || e.Source != "driver" || e.ProtocolConnectionNumber != 7 ||
		e.NodeName != "n" || e.TimeTag.IsZero() || e.Action != "observed" {
		t.Errorf("event not completed: %+v", e)
	}
}
