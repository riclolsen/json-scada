package clientapp

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"dnp3-go/internal/dnp3util"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/master"
	"github.com/riclolsen/json-scada/src/go-common/jsredundancy"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// TestRequeueKeepsOrderAndCoalescing: a batch the writer could not write goes
// back ahead of what arrived since, in order, and a later static still folds
// into the newest static of its point.
func TestRequeueKeepsOrderAndCoalescing(t *testing.T) {
	q := NewValueQueue()
	q.Push(mkValue(1, 1, 1, true))
	q.Push(mkValue(1, 1, 0, true))
	q.Push(mkValue(30, 7, 10, false))
	batch, _ := q.Drain()

	q.Push(mkValue(1, 1, 1, true)) // arrived while the write was failing
	q.Requeue(batch)
	q.Push(mkValue(30, 7, 11, false)) // folds into the requeued static

	got, _ := q.Drain()
	want := []struct {
		group, addr int
		value       float64
	}{{1, 1, 1}, {1, 1, 0}, {30, 7, 11}, {1, 1, 1}}
	if len(got) != len(want) {
		t.Fatalf("got %d values, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].BaseGroup != w.group || got[i].Address != w.addr || got[i].Value != w.value {
			t.Errorf("value %d = g%d #%d %v, want g%d #%d %v", i,
				got[i].BaseGroup, got[i].Address, got[i].Value, w.group, w.addr, w.value)
		}
	}
}

// TestFailedInsertUnmarksTheAddress: an address whose tag insert did not reach
// the server must be tried again, not remembered as created.
func TestFailedInsertUnmarksTheAddress(t *testing.T) {
	conn := &Connection{Name: "C", ProtocolConnectionNumber: 1, AutoCreateTags: true, CommandsEnabled: true,
		insertedAddresses: map[[2]int]bool{}}
	// An unreachable server: the key lookup fails, as it would in the outage
	// this test is about.
	cli, err := mongo.Connect(options.Client().ApplyURI("mongodb://127.0.0.1:1").
		SetServerSelectionTimeout(50 * time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cli.Disconnect(context.Background()) }()
	rtd := cli.Database("test").Collection("realtimeData")
	conn.tagKeys.Timeout = 200 * time.Millisecond

	tags := autoCreateFor(context.Background(), conn, rtd, mkValue(dnp3util.GroupBinaryOutputStatus, 4, 1, false))
	if len(tags) != 2 {
		t.Fatalf("got %d tags, want the supervised point and its command twin", len(tags))
	}
	if !conn.insertedAddresses[[2]int{dnp3util.GroupBinaryOutputStatus, 4}] {
		t.Fatal("address not marked while its insert is pending")
	}

	err = settleInserts(context.Background(), rtd, tags, errors.New("server selection timeout"))
	if err == nil {
		t.Fatal("a failed insert call must be returned, so the batch is retried")
	}
	if len(conn.insertedAddresses) != 0 {
		t.Errorf("addresses still marked after a failed insert: %v", conn.insertedAddresses)
	}
}

func TestSanitizeTimeIsRelative(t *testing.T) {
	now := time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		at   time.Time
		keep bool
	}{
		{now, true},                          // past the old fixed 2033 bound
		{now.Add(30 * 24 * time.Hour), true}, // a clock somewhat ahead
		{now.Add(5 * 365 * 24 * time.Hour), false},
		{time.Unix(0, 0), false}, // an unset clock
	}
	for _, c := range cases {
		got := sanitizeTime(c.at.UnixMilli(), now) != 0
		if got != c.keep {
			t.Errorf("sanitizeTime(%v) kept=%v, want %v", c.at, got, c.keep)
		}
	}
}

func TestAnalogOutputRefusesWhatDoesNotFit(t *testing.T) {
	cases := []struct {
		variation int
		value     float64
		ok        bool
	}{
		{2, 32767, true}, {2, -32768, true}, {2, 40000, false}, {2, -40000, false}, {2, 32767.9, true},
		{1, 2147483647, true}, {1, 3e9, false}, {1, -2147483649, false},
		{3, 1e30, true}, {3, 1e300, false},
		{4, 1e300, true}, {4, math.Inf(1), false},
		{1, math.NaN(), false}, {3, math.NaN(), false},
	}
	for _, c := range cases {
		if _, ok := analogOutput(3, c.variation, c.value); ok != c.ok {
			t.Errorf("g41v%d %v: ok=%v, want %v", c.variation, c.value, ok, c.ok)
		}
	}
}

// TestStandbyDoesNotInvalidate: a link lost while this node is active
// invalidates the points; the drop caused by going to standby must not, since
// the other node is writing the same tags by then.
func TestStandbyDoesNotInvalidate(t *testing.T) {
	newConn := func() *Connection {
		c := &Connection{Name: "C"}
		c.wasConnected = true
		return c
	}

	// Active, session running but disconnected: a real link loss.
	e := &Engine{redundancy: &jsredundancy.Controller{}}
	e.redundancy.ForceActive(true)
	conn := newConn()
	conn.setSession(master.New(master.Config{LocalAddr: 1, RemoteAddr: 2}, master.NopHandler{}), func() {})
	if !e.observeLink(conn) {
		t.Error("a link lost while active did not invalidate the points")
	}

	// Gone to standby: the session was stopped and redundancy is inactive.
	e.redundancy.ForceActive(false)
	conn = newConn()
	if e.observeLink(conn) {
		t.Error("going to standby invalidated the points")
	}
}

// TestFrozenAnalogsHaveTheirOwnAddress: frozen analogs must not land on the
// running analog of the same index.
func TestFrozenAnalogsHaveTheirOwnAddress(t *testing.T) {
	var _ master.FrozenAnalogHandler = (*soeHandler)(nil)

	q := NewValueQueue()
	h := newSOEHandler(&Connection{Name: "C", ProtocolConnectionNumber: 1}, q)
	h.HandleFrozenAnalog(master.HeaderInfo{}, []dnp3.Indexed[dnp3.Analog]{{Index: 3, Value: dnp3.Analog{Value: 1.5}}})
	h.HandleAnalog(master.HeaderInfo{}, []dnp3.Indexed[dnp3.Analog]{{Index: 3, Value: dnp3.Analog{Value: 2.5}}})

	got, _ := q.Drain()
	if len(got) != 2 || got[0].BaseGroup != dnp3util.GroupFrozenAnalogInput || got[1].BaseGroup != dnp3util.GroupAnalogInput {
		t.Fatalf("frozen and running analogs not kept apart: %+v", got)
	}
}
