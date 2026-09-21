package dnp3util

import (
	"context"
	"flag"
	"testing"
	"time"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/master"
	"github.com/dscsystems/go-dnp3/outstation"
)

// The two ends of a serial port pair. Without them there is nothing to test
// against, so the test below skips.
//
//	# Linux, with a socat PTY pair:
//	socat -d -d pty,raw,echo=0 pty,raw,echo=0
//	go test ./internal/dnp3util/ -run TestSerialPortPair \
//	    -serial.a=/dev/pts/3 -serial.b=/dev/pts/4
//
//	# Windows, with a com0com pair (needs an elevated install):
//	go test ./internal/dnp3util/ -run TestSerialPortPair \
//	    -serial.a=COM10 -serial.b=COM11
var (
	serialA = flag.String("serial.a", "", "one end of a serial port pair, for TestSerialPortPair")
	serialB = flag.String("serial.b", "", "the other end of the pair")
	// A PTY has no line rate, but a real port does and the pair must agree.
	serialBaud = flag.Int("serial.baud", 9600, "line rate for TestSerialPortPair")
)

// TestSerialPortPair runs a master against an outstation over two real serial
// ports, which is the one thing the rest of the serial tests cannot reach: the
// bytes actually crossing a port that go.bug.st/serial opened.
//
// It needs a port pair, so it skips by default. Everything else about serial
// operation is covered without hardware — the configuration mapping in
// TestSerialConfigMapping, the open failure in TestSerialOpenFailure, and the
// link-layer confirmation and arbitration that serial mode turns on in
// TestSerialLinkConfirms and TestSerialMultidropArbitration.
func TestSerialPortPair(t *testing.T) {
	if *serialA == "" || *serialB == "" {
		t.Skip("no serial port pair given; pass -serial.a and -serial.b (see the comment above)")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Both ends are built the way the drivers build them, from a
	// protocolConnections document's serial fields.
	outSpec := ChannelSpec{
		Name: "SRV", Mode: ModeSerial, PortName: *serialB,
		BaudRate: *serialBaud, Parity: "None", StopBits: "One",
	}
	cliSpec := ChannelSpec{
		Name: "CLI", Mode: ModeSerial, PortName: *serialA,
		BaudRate: *serialBaud, Parity: "None", StopBits: "One",
	}

	outCh, _, err := outSpec.BuildChannel(nil)
	if err != nil {
		t.Fatalf("building the outstation's serial channel on %s: %v", *serialB, err)
	}
	defer func() { _ = outCh.Close() }()

	// Link confirmation on, as startSessions turns it on for ModeSerial.
	out := outstation.New(outstation.Config{
		LocalAddr: 10, RemoteAddr: 1,
		Database:        outstation.DatabaseConfig{Analog: 2, Binary: 2, DefaultClass: dnp3.Class1},
		UseLinkConfirms: true,
		LinkRetries:     3,
		LinkTimeout:     time.Second,
	}, outstation.NopApplication{}, nil)

	db := out.Database()
	if _, cfg, ok := db.Analog(0); ok {
		cfg.StaticVariation, cfg.EventVariation = 5, 7
		db.Configure(dnp3.TypeAnalog, 0, cfg)
	}
	db.UpdateAnalog(0, dnp3.Analog{Value: 49.75, Flags: dnp3.Online})
	db.UpdateBinary(0, dnp3.Binary{Value: true, Flags: dnp3.Online})
	out.Events().Reset()

	go func() { _ = out.Run(ctx, outCh) }()

	cliCh, counters, err := cliSpec.BuildChannel(nil)
	if err != nil {
		t.Fatalf("building the master's serial channel on %s: %v", *serialA, err)
	}
	defer func() { _ = cliCh.Close() }()

	rec := newTLSRecorder()
	m := master.New(master.Config{
		LocalAddr: 1, RemoteAddr: 10,
		// A slow line needs a longer wait than a socket does.
		ResponseTimeout: 10 * time.Second,
		UseLinkConfirms: true,
		LinkRetries:     3,
		LinkTimeout:     2 * time.Second,
	}, rec)
	go func() { _ = m.Run(ctx, cliCh) }()

	waitForCond(t, "the serial port to open", m.Connected)

	pollCtx, pollCancel := context.WithTimeout(ctx, 30*time.Second)
	defer pollCancel()
	if err := m.IntegrityPoll(pollCtx); err != nil {
		t.Fatalf("IntegrityPoll over %s <-> %s: %v", *serialA, *serialB, err)
	}

	rec.mu.Lock()
	analog, haveAnalog := rec.analogs[0]
	binary, haveBinary := rec.binaries[0]
	rec.mu.Unlock()

	if !haveAnalog || analog != 49.75 {
		t.Errorf("analog 0 over the serial pair = %v (present=%v), want 49.75", analog, haveAnalog)
	}
	if !haveBinary || !binary {
		t.Errorf("binary 0 over the serial pair = %v (present=%v), want true", binary, haveBinary)
	}

	c := counters.Snapshot()
	if c.Opens != 1 {
		t.Errorf("numOpen = %d, want 1", c.Opens)
	}
	if c.BytesTx == 0 || c.BytesRx == 0 {
		t.Errorf("no bytes crossed the port: tx=%d rx=%d", c.BytesTx, c.BytesRx)
	}
	t.Logf("serial pair %s <-> %s at %d baud: %d bytes out, %d bytes in",
		*serialA, *serialB, *serialBaud, c.BytesTx, c.BytesRx)
}
