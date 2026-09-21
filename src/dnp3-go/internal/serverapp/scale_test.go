package serverapp

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/channel"
	"github.com/dscsystems/go-dnp3/master"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
)

// counter tallies what a master receives, per family.
type counter struct {
	master.NopHandler
	mu  sync.Mutex
	bi  map[uint16]bool
	ai  map[uint16]float64
	bos map[uint16]bool
	aos map[uint16]float64
}

func newCounter() *counter {
	return &counter{
		bi:  map[uint16]bool{},
		ai:  map[uint16]float64{},
		bos: map[uint16]bool{},
		aos: map[uint16]float64{},
	}
}

func (c *counter) HandleBinary(_ master.HeaderInfo, vs []dnp3.Indexed[dnp3.Binary]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, v := range vs {
		c.bi[v.Index] = v.Value.Value
	}
}
func (c *counter) HandleAnalog(_ master.HeaderInfo, vs []dnp3.Indexed[dnp3.Analog]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, v := range vs {
		c.ai[v.Index] = v.Value.Value
	}
}
func (c *counter) HandleBinaryOutputStatus(_ master.HeaderInfo, vs []dnp3.Indexed[dnp3.BinaryOutputStatus]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, v := range vs {
		c.bos[v.Index] = v.Value.Value
	}
}
func (c *counter) HandleAnalogOutputStatus(_ master.HeaderInfo, vs []dnp3.Indexed[dnp3.AnalogOutputStatus]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, v := range vs {
		c.aos[v.Index] = v.Value.Value
	}
}

func (c *counter) sizes() (bi, ai, bos, aos int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.bi), len(c.ai), len(c.bos), len(c.aos)
}

// TestIntegrityPollReturnsEveryFamily polls an outstation carrying a few
// hundred points of each family and checks the master receives all of them.
//
// A single point of each family is served correctly — TestServerLoopback proves
// that — so anything that goes wrong only at scale goes wrong in the response
// building, which is exactly what an operator sees as a family that never
// appears while its neighbours do.
func TestIntegrityPollReturnsEveryFamily(t *testing.T) {
	testIntegrityPollScale(t, 300, 300, 300, 300)
}

// TestIntegrityPollAtFieldScale repeats it at the point counts of a real
// installation — the demo database as an explorer reported it: 3306 binary
// inputs, 1552 analog inputs, 298 binary output statuses and 351 analog output
// statuses.
//
// This is the shape that showed binary output statuses missing from a browser's
// view while its neighbours appeared, so the counts are the ones to reproduce.
func TestIntegrityPollAtFieldScale(t *testing.T) {
	testIntegrityPollScale(t, 3306, 1552, 298, 351)
}

func testIntegrityPollScale(t *testing.T, nBI, nAI, nBOS, nAOS int) {
	t.Helper()

	// The per-point "Updating tag" lines would be tens of thousands of lines.
	jslog.SetLevel(0)
	t.Cleanup(func() { jslog.SetLevel(1) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn := &Connection{
		ProtocolConnectionNumber: 1,
		Name:                     "SCALE",
		LocalLinkAddress:         10,
		RemoteLinkAddress:        1,
		ServerQueueSize:          5000,
	}

	var tags []map[string]any
	id := 1.0
	add := func(name string, value any, group, index, asdu int) {
		tags = append(tags, tag(id, name, value, group, index, asdu, nil))
		id++
	}
	for i := 0; i < nBI; i++ {
		add(fmt.Sprintf("BI%d", i), i%2 == 0, 1, i, 2)
	}
	for i := 0; i < nAI; i++ {
		add(fmt.Sprintf("AI%d", i), float64(i)+0.5, 30, i, 5)
	}
	for i := 0; i < nBOS; i++ {
		add(fmt.Sprintf("BOS%d", i), i%3 == 0, 10, i, 2)
	}
	for i := 0; i < nAOS; i++ {
		add(fmt.Sprintf("AOS%d", i), float64(i)*2+0.25, 40, i, 3)
	}

	e := &Engine{byNum: map[int]*Connection{1: conn}}
	station := e.newOutstation(conn, toBsonSlice(tags))
	conn.setStation(station)

	counts := station.Database().Counts()
	t.Logf("database sized: %d BI, %d AI, %d BOS, %d AOS",
		counts.Binary, counts.Analog, counts.BinaryOutputStatus, counts.AnalogOutputStatus)
	if counts.BinaryOutputStatus != nBOS {
		t.Errorf("database has %d binary output statuses, want %d", counts.BinaryOutputStatus, nBOS)
	}

	mch, och := channel.Pipe()
	go func() { _ = station.Run(ctx, och) }()

	rec := newCounter()
	m := master.New(master.Config{
		LocalAddr: 1, RemoteAddr: 10,
		ResponseTimeout: 10 * time.Second,
	}, rec)
	go func() { _ = m.Run(ctx, mch) }()

	waitFor(t, "the master to connect", m.Connected)

	pollCtx, pollCancel := context.WithTimeout(ctx, 30*time.Second)
	defer pollCancel()
	if err := m.IntegrityPoll(pollCtx); err != nil {
		t.Fatalf("IntegrityPoll: %v", err)
	}

	bi, ai, bos, aos := rec.sizes()
	t.Logf("master received: %d BI, %d AI, %d BOS, %d AOS", bi, ai, bos, aos)

	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"binary inputs", bi, nBI},
		{"analog inputs", ai, nAI},
		{"binary output statuses", bos, nBOS},
		{"analog output statuses", aos, nAOS},
	} {
		if c.got != c.want {
			t.Errorf("received %d %s, want %d", c.got, c.name, c.want)
		}
	}
}
