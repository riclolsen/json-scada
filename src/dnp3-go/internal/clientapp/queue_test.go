package clientapp

import (
	"fmt"
	"testing"
)

// mkValue builds a queued value for one point.
func mkValue(group, address int, value float64, isEvent bool) Dnp3Value {
	return Dnp3Value{
		Address:     address,
		BaseGroup:   group,
		Group:       group,
		Value:       value,
		ValueString: fmt.Sprintf("%v", value),
		COT:         20,
		ConnNumber:  1,
		IsEvent:     isEvent,
	}
}

// TestQueueKeepsEveryPoint is the case a large outstation produces: one
// integrity poll delivers more points than the writer can drain in a batch.
//
// Every distinct point has to survive. A point dropped here is not merely a
// stale value — with autoCreateTags on, it is a tag that never gets created,
// and because the same points arrive first on every poll, the same ones are
// lost every time.
func TestQueueKeepsEveryPoint(t *testing.T) {
	const points = 15000

	q := NewValueQueue()
	for i := 0; i < points; i++ {
		q.Push(mkValue(1, i, float64(i), false))
	}

	batch, _ := q.Drain()
	seen := map[int]bool{}
	for _, v := range batch {
		seen[v.Address] = true
	}
	if len(seen) != points {
		t.Errorf("%d distinct points survived the queue, want %d (%d lost)",
			len(seen), points, points-len(seen))
	}
}

// TestQueueCoalescesStatics checks that repeated static reads of one point do
// not pile up: a static value is a snapshot, so the newest replaces the older
// one and the queue stays bounded by the number of points rather than by the
// poll rate.
func TestQueueCoalescesStatics(t *testing.T) {
	q := NewValueQueue()
	for i := 0; i < 500; i++ {
		q.Push(mkValue(30, 7, float64(i), false))
	}

	batch, _ := q.Drain()
	if len(batch) != 1 {
		t.Fatalf("%d entries queued for one point, want 1", len(batch))
	}
	if batch[0].Value != 499 {
		t.Errorf("value = %v, want the newest, 499", batch[0].Value)
	}
}

// TestQueueKeepsEveryEvent checks the opposite rule for events.
//
// An event is a discrete occurrence, not a snapshot: a point that goes on, off
// and on again produced three events, and collapsing them to the last one
// throws away the sequence of events a master polled class 1 to obtain.
func TestQueueKeepsEveryEvent(t *testing.T) {
	q := NewValueQueue()
	q.Push(mkValue(1, 3, 1, true))
	q.Push(mkValue(1, 3, 0, true))
	q.Push(mkValue(1, 3, 1, true))

	batch, _ := q.Drain()
	if len(batch) != 3 {
		t.Fatalf("%d events queued, want 3: an event sequence must not be collapsed", len(batch))
	}
	for i, want := range []float64{1, 0, 1} {
		if batch[i].Value != want {
			t.Errorf("event %d = %v, want %v", i, batch[i].Value, want)
		}
	}
}

// TestQueuePreservesOrder checks that points come out in the order they were
// first seen, so the bulk write applies them in a predictable sequence.
func TestQueuePreservesOrder(t *testing.T) {
	q := NewValueQueue()
	for i := 0; i < 100; i++ {
		q.Push(mkValue(1, i, float64(i), false))
	}
	// A second read of an earlier point updates it in place rather than
	// moving it to the back.
	q.Push(mkValue(1, 5, 999, false))

	batch, _ := q.Drain()
	if len(batch) != 100 {
		t.Fatalf("%d entries, want 100", len(batch))
	}
	for i, v := range batch {
		if v.Address != i {
			t.Fatalf("entry %d has address %d; order not preserved", i, v.Address)
		}
	}
	if batch[5].Value != 999 {
		t.Errorf("point 5 = %v, want the newer value 999", batch[5].Value)
	}
}

// TestQueueUnderExtremeEventPressure checks the backstop: when events alone
// exceed the pressure threshold, they coalesce rather than being dropped.
//
// Losing the intermediate values of a point that is chattering is a fair
// degradation; losing the point altogether is not, because that is what stops
// its tag being created.
func TestQueueUnderExtremeEventPressure(t *testing.T) {
	q := NewValueQueue()

	// Far more events than the threshold, spread over half as many points as
	// the threshold so coalescing has to do the bounding.
	const points = DataBufferLimit / 2
	for round := 0; round < 4; round++ {
		for i := 0; i < points; i++ {
			q.Push(mkValue(1, i, float64(round), true))
		}
	}

	batch, _ := q.Drain()
	seen := map[int]bool{}
	for _, v := range batch {
		seen[v.Address] = true
	}
	if len(seen) != points {
		t.Errorf("%d distinct points survived, want %d: no point may be lost", len(seen), points)
	}
}
