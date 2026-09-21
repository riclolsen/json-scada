package dnp3util

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/master"
	"github.com/dscsystems/go-dnp3/outstation"
)

// udpSpecs builds the two channel specifications of a UDP pair, as the drivers
// build them from a protocolConnections document: each end binds its own local
// endpoint and sends to the other's.
func udpSpecs(masterAddr, outstationAddr string) (masterSpec, outstationSpec ChannelSpec) {
	masterSpec = ChannelSpec{
		Name:               "CLI",
		Mode:               ModeUDP,
		IPAddressLocalBind: masterAddr,
		IPAddresses:        []string{outstationAddr},
	}
	outstationSpec = ChannelSpec{
		Name:               "SRV",
		Mode:               ModeUDP,
		IPAddressLocalBind: outstationAddr,
		IPAddresses:        []string{masterAddr},
	}
	return masterSpec, outstationSpec
}

// TestUDPLoopback runs a master against an outstation over real UDP sockets
// built from the same ChannelSpec fields a protocolConnections document
// supplies.
func TestUDPLoopback(t *testing.T) {
	const (
		masterAddr     = "127.0.0.1:20981"
		outstationAddr = "127.0.0.1:20982"
	)
	masterSpec, outstationSpec := udpSpecs(masterAddr, outstationAddr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	outCh, _, err := outstationSpec.BuildChannel(nil)
	if err != nil {
		t.Fatalf("building the outstation's UDP channel: %v", err)
	}
	defer func() { _ = outCh.Close() }()

	out := outstation.New(outstation.Config{
		LocalAddr: 10, RemoteAddr: 1,
		Database: outstation.DatabaseConfig{Analog: 2, Binary: 2, Counter: 1, DefaultClass: dnp3.Class1},
	}, outstation.NopApplication{}, nil)

	db := out.Database()
	if _, cfg, ok := db.Analog(0); ok {
		cfg.StaticVariation, cfg.EventVariation = 5, 7
		db.Configure(dnp3.TypeAnalog, 0, cfg)
	}
	db.UpdateAnalog(0, dnp3.Analog{Value: 88.25, Flags: dnp3.Online})
	db.UpdateBinary(0, dnp3.Binary{Value: true, Flags: dnp3.Online})
	db.UpdateCounter(0, dnp3.Counter{Value: 1234, Flags: dnp3.Online})
	out.Events().Reset()

	go func() { _ = out.Run(ctx, outCh) }()

	masterCh, counters, err := masterSpec.BuildChannel(nil)
	if err != nil {
		t.Fatalf("building the master's UDP channel: %v", err)
	}
	defer func() { _ = masterCh.Close() }()

	rec := newTLSRecorder()
	m := master.New(master.Config{
		LocalAddr: 1, RemoteAddr: 10,
		ResponseTimeout: 5 * time.Second,
	}, rec)
	go func() { _ = m.Run(ctx, masterCh) }()

	waitForCond(t, "the master's UDP socket to bind", m.Connected)

	if err := m.IntegrityPoll(ctx); err != nil {
		t.Fatalf("IntegrityPoll over UDP: %v", err)
	}

	rec.mu.Lock()
	analog, haveAnalog := rec.analogs[0]
	binary, haveBinary := rec.binaries[0]
	rec.mu.Unlock()

	if !haveAnalog || analog != 88.25 {
		t.Errorf("analog 0 over UDP = %v (present=%v), want 88.25", analog, haveAnalog)
	}
	if !haveBinary || !binary {
		t.Errorf("binary 0 over UDP = %v (present=%v), want true", binary, haveBinary)
	}

	if c := counters.Snapshot(); c.BytesTx == 0 || c.BytesRx == 0 {
		t.Errorf("no datagrams counted: tx=%d rx=%d", c.BytesTx, c.BytesRx)
	}
}

// TestUDPNoPeer checks the failure an operator meets when the far end is not
// there.
//
// UDP is connectionless, so binding always succeeds and the master reports
// itself connected the moment its socket is up: there is no connection to fail.
// The absence of the peer shows up one layer higher, as a poll that times out.
// A driver that treated "connected" as "reachable" would report a dead link as
// healthy, so this is worth pinning.
func TestUDPNoPeer(t *testing.T) {
	masterSpec, _ := udpSpecs("127.0.0.1:20983", "127.0.0.1:20984")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, _, err := masterSpec.BuildChannel(nil)
	if err != nil {
		t.Fatalf("building the UDP channel: %v", err)
	}
	defer func() { _ = ch.Close() }()

	m := master.New(master.Config{
		LocalAddr: 1, RemoteAddr: 10,
		ResponseTimeout: 500 * time.Millisecond,
	}, master.NopHandler{})
	go func() { _ = m.Run(ctx, ch) }()

	waitForCond(t, "the UDP socket to bind", m.Connected)

	pollCtx, pollCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pollCancel()
	err = m.IntegrityPoll(pollCtx)
	if err == nil {
		t.Fatal("a poll with nothing at the far end must not succeed")
	}
	if !errors.Is(err, dnp3.ErrTimeout) && !errors.Is(err, dnp3.ErrTaskFailed) &&
		!errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("unexpected error for an absent peer: %v", err)
	}
}

// TestUDPConfigValidation checks the two fields a UDP connection cannot do
// without. Both are refused by the C++ drivers too, which log and skip the
// connection.
func TestUDPConfigValidation(t *testing.T) {
	t.Run("a missing local bind is refused", func(t *testing.T) {
		s := ChannelSpec{Name: "U", Mode: ModeUDP, IPAddresses: []string{"127.0.0.1:20000"}}
		_, _, err := s.BuildChannel(nil)
		if err == nil {
			t.Fatal("a UDP connection with no ipAddressLocalBind must be refused")
		}
		if !strings.Contains(err.Error(), "ipAddressLocalBind") {
			t.Errorf("the error must name the field, got %q", err)
		}
	})

	t.Run("a missing remote is refused", func(t *testing.T) {
		s := ChannelSpec{Name: "U", Mode: ModeUDP, IPAddressLocalBind: "127.0.0.1:20000"}
		_, _, err := s.BuildChannel(nil)
		if err == nil {
			t.Fatal("a UDP connection with no ipAddresses must be refused")
		}
		if !strings.Contains(err.Error(), "ipAddresses") {
			t.Errorf("the error must name the field, got %q", err)
		}

		s.IPAddresses = []string{"  "}
		if _, _, err := s.BuildChannel(nil); err == nil {
			t.Error("a blank remote address must be refused")
		}
	})

	t.Run("a UDP endpoint is not a shared medium on its own", func(t *testing.T) {
		s := ChannelSpec{
			Mode: ModeUDP, IPAddressLocalBind: "127.0.0.1:20000",
			IPAddresses: []string{"127.0.0.1:20001"},
		}
		if s.IsSharedMedium() {
			t.Error("a lone UDP endpoint needs no arbitration")
		}
		if got := TurnaroundFor(s, 1); got >= 0 {
			t.Errorf("a lone UDP station should disable arbitration, got %v", got)
		}
		// Two connections on one UDP endpoint is multi-drop again.
		if got := TurnaroundFor(s, 2); got != 0 {
			t.Errorf("a shared UDP endpoint should keep the default turnaround, got %v", got)
		}
	})
}

// TestUDPMultidrop puts two outstations behind one UDP endpoint and checks the
// bus routes each master to its own station.
//
// A UDP socket carries every station's datagrams on one pair of endpoints, so
// this is the same arrangement as a serial line: the link address is all there
// is to tell them apart.
func TestUDPMultidrop(t *testing.T) {
	const (
		masterAddr     = "127.0.0.1:20985"
		outstationAddr = "127.0.0.1:20986"
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Two outstation connections repeating one endpoint: what a multi-drop
	// UDP configuration looks like in protocolConnections.
	outSpecs := []StationSpec{
		{
			ConnectionNumber: 1, Name: "SRV10", LocalLinkAddress: 10, RemoteLinkAddress: 1,
			Channel: ChannelSpec{
				Name: "SRV", Mode: ModeUDP,
				IPAddressLocalBind: outstationAddr, IPAddresses: []string{masterAddr},
			},
		},
		{
			ConnectionNumber: 2, Name: "SRV11", LocalLinkAddress: 11, RemoteLinkAddress: 1,
			Channel: ChannelSpec{
				Name: "SRV", Mode: ModeUDP,
				IPAddressLocalBind: outstationAddr, IPAddresses: []string{masterAddr},
			},
		},
	}
	outGroups, err := BuildGroups(outSpecs, false)
	if err != nil {
		t.Fatalf("grouping the outstations: %v", err)
	}
	defer func() {
		for _, g := range outGroups {
			g.Close()
		}
	}()
	if len(outGroups) != 1 {
		t.Fatalf("the two outstations should share one UDP endpoint, got %d groups", len(outGroups))
	}

	for _, num := range []int{1, 2} {
		addr := uint16(9 + num) // 10 and 11
		out := outstation.New(outstation.Config{
			LocalAddr: addr, RemoteAddr: 1,
			Database: outstation.DatabaseConfig{Analog: 1, DefaultClass: dnp3.Class1},
		}, outstation.NopApplication{}, nil)
		if _, cfg, ok := out.Database().Analog(0); ok {
			cfg.StaticVariation, cfg.EventVariation = 5, 7
			out.Database().Configure(dnp3.TypeAnalog, 0, cfg)
		}
		out.Database().UpdateAnalog(0, dnp3.Analog{Value: float64(addr) * 10, Flags: dnp3.Online})
		out.Events().Reset()
		ch := outGroups[0].Channels[num]
		go func() { _ = out.Run(ctx, ch) }()
	}

	// Two master connections repeating the other endpoint.
	masterSpecs := []StationSpec{
		{
			ConnectionNumber: 1, Name: "CLI10", LocalLinkAddress: 1, RemoteLinkAddress: 10,
			Channel: ChannelSpec{
				Name: "CLI", Mode: ModeUDP,
				IPAddressLocalBind: masterAddr, IPAddresses: []string{outstationAddr},
			},
		},
		{
			ConnectionNumber: 2, Name: "CLI11", LocalLinkAddress: 1, RemoteLinkAddress: 11,
			Channel: ChannelSpec{
				Name: "CLI", Mode: ModeUDP,
				IPAddressLocalBind: masterAddr, IPAddresses: []string{outstationAddr},
			},
		},
	}
	masterGroups, err := BuildGroups(masterSpecs, true)
	if err != nil {
		t.Fatalf("grouping the masters: %v", err)
	}
	defer func() {
		for _, g := range masterGroups {
			g.Close()
		}
	}()

	recs := map[int]*tlsRecorder{}
	masters := map[int]*master.Session{}
	for _, num := range []int{1, 2} {
		addr := uint16(9 + num)
		rec := newTLSRecorder()
		m := master.New(master.Config{
			LocalAddr: 1, RemoteAddr: addr,
			ResponseTimeout: 5 * time.Second,
		}, rec)
		ch := masterGroups[0].Channels[num]
		go func() { _ = m.Run(ctx, ch) }()
		recs[num], masters[num] = rec, m
	}

	for _, num := range []int{1, 2} {
		waitForCond(t, "the masters to bind", masters[num].Connected)
	}
	for _, num := range []int{1, 2} {
		if err := masters[num].IntegrityPoll(ctx); err != nil {
			t.Fatalf("IntegrityPoll for station %d: %v", 9+num, err)
		}
	}

	for _, num := range []int{1, 2} {
		want := float64(9+num) * 10
		recs[num].mu.Lock()
		got, ok := recs[num].analogs[0]
		count := len(recs[num].analogs)
		recs[num].mu.Unlock()

		if !ok || got != want {
			t.Errorf("master for station %d read %v (present=%v), want %v — the bus routed the wrong station's data",
				9+num, got, ok, want)
		}
		if count != 1 {
			t.Errorf("master for station %d saw %d analogs, want 1", 9+num, count)
		}
	}

	if s := masterGroups[0].Stats(); s.FramesUnrouted > 0 {
		t.Errorf("%d datagrams were routed to nobody; the link addresses do not line up",
			s.FramesUnrouted)
	}
}
