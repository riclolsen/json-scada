package dnp3util

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dscsystems/go-dnp3/channel"
)

// fakeChannel returns whatever the test tells it to, counting the attempts.
type fakeChannel struct {
	attempts atomic.Int32
	err      error
}

func (f *fakeChannel) Connect(ctx context.Context) (io.ReadWriteCloser, error) {
	f.attempts.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	return nopConn{}, nil
}

func (f *fakeChannel) Close() error   { return nil }
func (f *fakeChannel) String() string { return "fake" }

// TestCountChannelClosedPropagates pins the rule that a closed channel is not a
// failed connection attempt.
//
// The bus closes the underlying channel when the driver shuts down or a
// redundancy deactivation tears it down, and both sessions read
// channel.ErrClosed as a clean shutdown. Retrying it would spin against a
// channel that can never produce a connection, bump numOpenFail on every
// orderly shutdown, and hide the shutdown signal from the session.
func TestCountChannelClosedPropagates(t *testing.T) {
	inner := &fakeChannel{err: channel.ErrClosed}
	ch, counters := WrapCounting(inner, CountOptions{Name: "C", Retry: DefaultRetry})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	_, err := ch.Connect(ctx)
	elapsed := time.Since(start)

	if !errors.Is(err, channel.ErrClosed) {
		t.Errorf("Connect returned %v, want channel.ErrClosed", err)
	}
	if elapsed > time.Second {
		t.Errorf("Connect took %v; a closed channel must not be retried", elapsed)
	}
	if n := inner.attempts.Load(); n != 1 {
		t.Errorf("%d attempts, want exactly 1", n)
	}
	if c := counters.Snapshot(); c.OpenFails != 0 {
		t.Errorf("numOpenFail = %d, want 0: a shutdown is not a failed open", c.OpenFails)
	}
}

// TestCountChannelCancelledPropagates is the same rule for a cancelled context.
func TestCountChannelCancelledPropagates(t *testing.T) {
	inner := &fakeChannel{err: context.Canceled}
	ch, counters := WrapCounting(inner, CountOptions{Name: "C", Retry: DefaultRetry})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := ch.Connect(ctx); err == nil {
		t.Error("Connect on a cancelled context must fail")
	}
	if c := counters.Snapshot(); c.OpenFails != 0 {
		t.Errorf("numOpenFail = %d, want 0: cancellation is not a failed open", c.OpenFails)
	}
}

// TestCountChannelRetriesRealFailures checks the case the retry loop exists
// for: a genuine failure is counted and tried again, so numOpenFail reflects
// the attempts an operator is waiting on.
func TestCountChannelRetriesRealFailures(t *testing.T) {
	inner := &fakeChannel{err: errors.New("port is busy")}
	// A short backoff so several attempts fit in the test.
	ch, counters := WrapCounting(inner, CountOptions{
		Name:  "C",
		Retry: Retry{Min: 10 * time.Millisecond, Max: 20 * time.Millisecond, Factor: 1},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	if _, err := ch.Connect(ctx); err == nil {
		t.Error("Connect must fail while the inner channel keeps failing")
	}

	attempts := inner.attempts.Load()
	if attempts < 2 {
		t.Errorf("%d attempts, want the failure to be retried", attempts)
	}
	c := counters.Snapshot()
	if c.OpenFails < 2 {
		t.Errorf("numOpenFail = %d, want one per attempt (%d)", c.OpenFails, attempts)
	}
	if c.Opens != 0 {
		t.Errorf("numOpen = %d, want 0", c.Opens)
	}
}

// TestCountChannelCountsBytes checks the byte and open/close counters that feed
// the stats document.
func TestCountChannelCountsBytes(t *testing.T) {
	ch, counters := WrapCounting(&fakeChannel{}, CountOptions{Name: "C"})

	conn, err := ch.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if _, err := conn.Write([]byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	// Closing twice must not count two disconnections.
	_ = conn.Close()

	c := counters.Snapshot()
	if c.Opens != 1 {
		t.Errorf("numOpen = %d, want 1", c.Opens)
	}
	if c.BytesTx != 5 {
		t.Errorf("numBytesTx = %d, want 5", c.BytesTx)
	}
	if c.Closes != 1 {
		t.Errorf("numClose = %d, want 1 even though Close was called twice", c.Closes)
	}
}

// namedChannel is one endpoint that can be made to fail or succeed on demand.
type namedChannel struct {
	name     string
	fail     atomic.Bool
	attempts atomic.Int32
}

func (n *namedChannel) Connect(ctx context.Context) (io.ReadWriteCloser, error) {
	n.attempts.Add(1)
	if n.fail.Load() {
		return nil, errors.New(n.name + " refused")
	}
	return nopConn{}, nil
}
func (n *namedChannel) Close() error   { return nil }
func (n *namedChannel) String() string { return n.name }

// TestRotatesThroughEndpoints checks that the alternative addresses of one
// connection are tried in turn, which is what an ipAddresses list of more than
// one entry is for.
func TestRotatesThroughEndpoints(t *testing.T) {
	a := &namedChannel{name: "a"}
	b := &namedChannel{name: "b"}
	c := &namedChannel{name: "c"}
	a.fail.Store(true)
	b.fail.Store(true)

	ch, counters := WrapCountingAll([]channel.Channel{a, b, c},
		CountOptions{Name: "CONN", Retry: DefaultRetry})

	// The first two refuse, so the third must answer — and without waiting out
	// a retry delay, because the point of a second address is a fast failover.
	start := time.Now()
	conn, err := ch.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	_ = conn.Close()

	if elapsed := time.Since(start); elapsed > 300*time.Millisecond {
		t.Errorf("failing over took %v; the list should be tried before any backoff", elapsed)
	}
	for _, n := range []*namedChannel{a, b, c} {
		if n.attempts.Load() != 1 {
			t.Errorf("endpoint %s was tried %d times, want 1", n.name, n.attempts.Load())
		}
	}
	if got := counters.Snapshot().OpenFails; got != 2 {
		t.Errorf("numOpenFail = %d, want 2", got)
	}
}

// TestStaysOnAWorkingEndpoint checks that a connection settles: once an address
// answers, reconnecting goes back to that one rather than walking the list
// again.
func TestStaysOnAWorkingEndpoint(t *testing.T) {
	a := &namedChannel{name: "a"}
	b := &namedChannel{name: "b"}
	a.fail.Store(true)

	ch, _ := WrapCountingAll([]channel.Channel{a, b},
		CountOptions{Name: "CONN", Retry: DefaultRetry})

	for i := 0; i < 3; i++ {
		conn, err := ch.Connect(context.Background())
		if err != nil {
			t.Fatalf("Connect %d: %v", i, err)
		}
		_ = conn.Close()
	}

	// a is tried once, fails, and is not returned to while b keeps answering.
	if got := a.attempts.Load(); got != 1 {
		t.Errorf("the failed endpoint was tried %d times, want 1", got)
	}
	if got := b.attempts.Load(); got != 3 {
		t.Errorf("the working endpoint was used %d times, want 3", got)
	}
}

// TestFailsOverWhenTheCurrentEndpointDies checks the case the feature exists
// for: the address that has been working stops answering and the connection
// moves to the alternative.
func TestFailsOverWhenTheCurrentEndpointDies(t *testing.T) {
	a := &namedChannel{name: "a"}
	b := &namedChannel{name: "b"}

	ch, _ := WrapCountingAll([]channel.Channel{a, b},
		CountOptions{Name: "CONN", Retry: DefaultRetry})

	conn, err := ch.Connect(context.Background())
	if err != nil {
		t.Fatalf("first Connect: %v", err)
	}
	_ = conn.Close()
	if a.attempts.Load() != 1 || b.attempts.Load() != 0 {
		t.Fatalf("the first address should have answered, got a=%d b=%d",
			a.attempts.Load(), b.attempts.Load())
	}

	// The primary goes away.
	a.fail.Store(true)
	conn, err = ch.Connect(context.Background())
	if err != nil {
		t.Fatalf("failover Connect: %v", err)
	}
	_ = conn.Close()

	if b.attempts.Load() != 1 {
		t.Errorf("the alternative address was not tried, got %d attempts", b.attempts.Load())
	}
}

// TestBacksOffOnlyAfterTheWholeList checks that the retry delay is paid once
// per cycle rather than once per address, so three dead addresses do not take
// three backoffs to report.
func TestBacksOffOnlyAfterTheWholeList(t *testing.T) {
	var inners []channel.Channel
	for _, name := range []string{"a", "b", "c"} {
		n := &namedChannel{name: name}
		n.fail.Store(true)
		inners = append(inners, n)
	}

	ch, counters := WrapCountingAll(inners, CountOptions{
		Name:  "CONN",
		Retry: Retry{Min: 50 * time.Millisecond, Max: 50 * time.Millisecond, Factor: 1},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 220*time.Millisecond)
	defer cancel()
	if _, err := ch.Connect(ctx); err == nil {
		t.Fatal("Connect must fail while every endpoint refuses")
	}

	// Three addresses per 50 ms cycle: well over three attempts in the window,
	// which would be impossible if each address cost its own delay.
	if got := counters.Snapshot().OpenFails; got < 6 {
		t.Errorf("%d attempts in the window; the backoff is being paid per address", got)
	}
}

// TestBuildChannelUsesEveryAddress checks that the whole ipAddresses list
// reaches the channel, not just the first entry.
func TestBuildChannelUsesEveryAddress(t *testing.T) {
	s := ChannelSpec{
		Name: "CONN", Mode: ModeTCPActive,
		IPAddresses: []string{"127.0.0.1:20991", "127.0.0.1:20992", "127.0.0.1:20993"},
	}
	ch, _, err := s.BuildChannel(nil)
	if err != nil {
		t.Fatalf("BuildChannel: %v", err)
	}
	defer func() { _ = ch.Close() }()

	for _, want := range []string{"20991", "20992", "20993"} {
		if !strings.Contains(ch.String(), want) {
			t.Errorf("channel %q does not carry address %s", ch.String(), want)
		}
	}

	// Blank entries are ignored, and a list of nothing but blanks is refused.
	s.IPAddresses = []string{"", "127.0.0.1:20994", "  "}
	ch2, _, err := s.BuildChannel(nil)
	if err != nil {
		t.Fatalf("BuildChannel with blanks: %v", err)
	}
	defer func() { _ = ch2.Close() }()
	if strings.Contains(ch2.String(), "|") {
		t.Errorf("blank addresses were not skipped: %q", ch2.String())
	}

	s.IPAddresses = []string{"", "   "}
	if _, _, err := s.BuildChannel(nil); err == nil {
		t.Error("a list with no usable address must be refused")
	}
}
