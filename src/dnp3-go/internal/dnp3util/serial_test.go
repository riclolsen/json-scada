package dnp3util

import (
	"context"
	"io"
	"testing"
	"time"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/channel"
	"github.com/dscsystems/go-dnp3/master"
	"github.com/dscsystems/go-dnp3/multidrop"
	"github.com/dscsystems/go-dnp3/outstation"
)

// TestSerialConfigMapping checks how the serial fields of a
// protocolConnections document map onto the library's SerialConfig.
func TestSerialConfigMapping(t *testing.T) {
	base := ChannelSpec{Name: "S", Mode: ModeSerial, PortName: "COM3", BaudRate: 19200}

	t.Run("the port, rate and eight data bits", func(t *testing.T) {
		cfg := base.serialConfig()
		if cfg.Device != "COM3" {
			t.Errorf("Device = %q, want COM3", cfg.Device)
		}
		if cfg.Baud != 19200 {
			t.Errorf("Baud = %d, want 19200", cfg.Baud)
		}
		// The C++ drivers hard-code eight data bits; DNP3 has no other size.
		if cfg.DataBits != 8 {
			t.Errorf("DataBits = %d, want 8", cfg.DataBits)
		}
		// A blocking read has to be bounded so the session notices its context.
		if cfg.ReadTimeout <= 0 {
			t.Errorf("ReadTimeout = %v, want a positive bound", cfg.ReadTimeout)
		}
	})

	t.Run("parity", func(t *testing.T) {
		cases := map[string]channel.Parity{
			"":     channel.ParityNone,
			"None": channel.ParityNone,
			"none": channel.ParityNone,
			"Even": channel.ParityEven,
			"EVEN": channel.ParityEven,
			"Odd":  channel.ParityOdd,
			"odd":  channel.ParityOdd,
			// Anything unrecognised falls back to none, as in the C++ drivers.
			"Mark": channel.ParityNone,
		}
		for in, want := range cases {
			s := base
			s.Parity = in
			if got := s.serialConfig().Parity; got != want {
				t.Errorf("parity %q -> %v, want %v", in, got, want)
			}
		}
	})

	t.Run("stop bits", func(t *testing.T) {
		cases := map[string]channel.StopBits{
			"":    channel.StopBits1,
			"One": channel.StopBits1,
			"one": channel.StopBits1,
			"Two": channel.StopBits2,
			"2":   channel.StopBits2,
			// deviation D22: the C++ drivers fold one-and-a-half onto two,
			// because opendnp3's SerialSettings has no other value for it.
			"One5":     channel.StopBits1Point5,
			"ONE.FIVE": channel.StopBits1Point5,
			"1.5":      channel.StopBits1Point5,
		}
		for in, want := range cases {
			s := base
			s.StopBits = in
			if got := s.serialConfig().StopBits; got != want {
				t.Errorf("stop bits %q -> %v, want %v", in, got, want)
			}
		}
	})
}

// TestSerialIsSharedMedium pins the arbitration decision. A serial line is
// always shared; a dedicated socket never is until a second connection appears
// on it.
func TestSerialIsSharedMedium(t *testing.T) {
	serial := ChannelSpec{Mode: ModeSerial, PortName: "COM1"}
	tcp := ChannelSpec{Mode: ModeTCPActive, IPAddresses: []string{"10.0.0.5:20000"}}
	tlsActive := ChannelSpec{Mode: ModeTLSActive, IPAddresses: []string{"10.0.0.5:20000"}}
	udp := ChannelSpec{Mode: ModeUDP, IPAddressLocalBind: ":20000", IPAddresses: []string{"10.0.0.5:20000"}}

	if !serial.IsSharedMedium() {
		t.Error("a serial line is a shared medium")
	}
	for _, s := range []ChannelSpec{tcp, tlsActive, udp} {
		if s.IsSharedMedium() {
			t.Errorf("%s must not be treated as a shared medium on its own", s.Mode)
		}
	}

	// One station on a socket: no turn taking, which the library reads as a
	// negative turnaround.
	if got := TurnaroundFor(tcp, 1); got >= 0 {
		t.Errorf("a lone TCP station should disable arbitration, got %v", got)
	}
	// One station on a serial line: arbitrate anyway, the line may carry
	// equipment this driver does not own.
	if got := TurnaroundFor(serial, 1); got != 0 {
		t.Errorf("a lone serial station should keep the default turnaround, got %v", got)
	}
	// More than one station on any transport: a terminal server fronting a real
	// serial line looks exactly like this.
	if got := TurnaroundFor(tcp, 2); got != 0 {
		t.Errorf("a shared TCP endpoint should keep the default turnaround, got %v", got)
	}
	if got := TurnaroundFor(serial, 3); got != 0 {
		t.Errorf("a shared serial line should keep the default turnaround, got %v", got)
	}
}

// TestSerialMissingPortName checks the configuration error an operator gets for
// a serial connection with no port.
func TestSerialMissingPortName(t *testing.T) {
	s := ChannelSpec{Name: "S", Mode: ModeSerial}
	if _, _, err := s.BuildChannel(nil); err == nil {
		t.Error("a serial connection with no portName must be refused")
	}
}

// TestSerialOpenFailure checks that naming a port that is not there fails
// rather than hanging.
//
// This is as far as a test can go without hardware or a virtual port pair: the
// bytes on a real UART are go.bug.st/serial's business. What is verified here
// is that the driver's own path — spec to SerialConfig to channel to Connect —
// reaches the port and reports what happened.
func TestSerialOpenFailure(t *testing.T) {
	s := ChannelSpec{Name: "S", Mode: ModeSerial, PortName: "COM_DOES_NOT_EXIST", BaudRate: 9600}
	ch, counters, err := s.BuildChannel(nil)
	if err != nil {
		t.Fatalf("building the serial channel: %v", err)
	}
	defer func() { _ = ch.Close() }()

	// DefaultRetry keeps trying, so the context is what ends the attempt.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := ch.Connect(ctx); err == nil {
		t.Fatal("opening a port that does not exist must fail")
	}
	if c := counters.Snapshot(); c.OpenFails == 0 {
		t.Error("a failed open must be counted in numOpenFail")
	}
}

// TestSerialAsyncOpenDelay checks that asyncOpenDelay holds off the first
// transmission. go-dnp3's SerialConfig has no such setting, so the driver
// applies it in the channel wrapper.
func TestSerialAsyncOpenDelay(t *testing.T) {
	inner := &instantChannel{}
	ch, _ := WrapCounting(inner, CountOptions{Name: "S", OpenDelay: 300 * time.Millisecond})

	start := time.Now()
	conn, err := ch.Connect(context.Background())
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	elapsed := time.Since(start)
	_ = conn.Close()

	if elapsed < 250*time.Millisecond {
		t.Errorf("Connect returned after %v, want it held for the open delay", elapsed)
	}
	if elapsed > 3*time.Second {
		t.Errorf("Connect took %v, far longer than the open delay", elapsed)
	}
}

// TestSerialLinkConfirms runs a master and an outstation with link-layer
// confirmation enabled, which is what the drivers turn on for a serial
// connection and leave off over TCP.
//
// The pipe stands in for the line; what is exercised is the confirmed link
// layer itself — the secondary's acknowledgements and the primary's retry
// state machine — which is the substantive protocol difference between serial
// and TCP operation.
func TestSerialLinkConfirms(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mch, och := channel.Pipe()

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
	db.UpdateAnalog(0, dnp3.Analog{Value: 76.5, Flags: dnp3.Online})
	db.UpdateBinary(0, dnp3.Binary{Value: true, Flags: dnp3.Online})
	out.Events().Reset()

	go func() { _ = out.Run(ctx, och) }()

	rec := newTLSRecorder()
	m := master.New(master.Config{
		LocalAddr: 1, RemoteAddr: 10,
		ResponseTimeout: 5 * time.Second,
		// The settings startSessions applies for ModeSerial.
		UseLinkConfirms: true,
		LinkRetries:     3,
		LinkTimeout:     time.Second,
	}, rec)
	go func() { _ = m.Run(ctx, mch) }()

	waitForCond(t, "the confirmed link to come up", m.Connected)

	if err := m.IntegrityPoll(ctx); err != nil {
		t.Fatalf("IntegrityPoll over a confirmed link: %v", err)
	}

	rec.mu.Lock()
	analog, haveAnalog := rec.analogs[0]
	binary, haveBinary := rec.binaries[0]
	rec.mu.Unlock()

	if !haveAnalog || analog != 76.5 {
		t.Errorf("analog 0 = %v (present=%v), want 76.5", analog, haveAnalog)
	}
	if !haveBinary || !binary {
		t.Errorf("binary 0 = %v (present=%v), want true", binary, haveBinary)
	}

	// A confirmed link acknowledges every frame, so more frames cross the wire
	// than the request and response alone.
	if s := m.Stats(); s.TasksSucceeded == 0 {
		t.Error("no task completed over the confirmed link")
	}
}

// TestSerialMultidropArbitration puts two stations on one line with
// arbitration in force and checks that a station which never answers does not
// stop its neighbour being polled.
//
// This is the serial failure that matters in the field: one dead RTU on an
// RS-485 pair must not take the line down for the others. The arbiter's hold is
// a reservation with an expiry, and this is what proves it.
func TestSerialMultidropArbitration(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mch, och := channel.Pipe()

	// A short turnaround so the test does not wait out the two-second default.
	const turnaround = 300 * time.Millisecond

	outBus := multidrop.New(och, multidrop.Config{Turnaround: turnaround})
	defer func() { _ = outBus.Close() }()

	// Only station 10 exists on the line. Station 11 is configured on the
	// master side but has no outstation answering, which is what a powered-down
	// RTU looks like.
	sub, err := outBus.Add(multidrop.Station{LocalAddr: 10, RemoteAddr: 1, Master: false})
	if err != nil {
		t.Fatalf("adding the outstation: %v", err)
	}
	out := outstation.New(outstation.Config{
		LocalAddr: 10, RemoteAddr: 1,
		Database: outstation.DatabaseConfig{Analog: 1, DefaultClass: dnp3.Class1},
	}, outstation.NopApplication{}, nil)
	if _, cfg, ok := out.Database().Analog(0); ok {
		cfg.StaticVariation, cfg.EventVariation = 5, 7
		out.Database().Configure(dnp3.TypeAnalog, 0, cfg)
	}
	out.Database().UpdateAnalog(0, dnp3.Analog{Value: 11.25, Flags: dnp3.Online})
	out.Events().Reset()
	go func() { _ = out.Run(ctx, sub) }()

	mBus := multidrop.New(mch, multidrop.Config{Turnaround: turnaround})
	defer func() { _ = mBus.Close() }()

	// The master for the station that is not there. Its polls will time out.
	deadSub, err := mBus.Add(multidrop.Station{LocalAddr: 1, RemoteAddr: 11, Master: true})
	if err != nil {
		t.Fatalf("adding the master for the absent station: %v", err)
	}
	dead := master.New(master.Config{
		LocalAddr: 1, RemoteAddr: 11,
		ResponseTimeout: 500 * time.Millisecond,
		TaskRetryPeriod: 200 * time.Millisecond,
	}, master.NopHandler{})
	go func() { _ = dead.Run(ctx, deadSub) }()

	// Keep the absent station's master polling throughout, so the line is
	// under contention for the whole test.
	go func() {
		for ctx.Err() == nil {
			_ = dead.ScanClasses(ctx, dnp3.ClassAll)
		}
	}()

	liveSub, err := mBus.Add(multidrop.Station{LocalAddr: 1, RemoteAddr: 10, Master: true})
	if err != nil {
		t.Fatalf("adding the master for the live station: %v", err)
	}
	rec := newTLSRecorder()
	live := master.New(master.Config{
		LocalAddr: 1, RemoteAddr: 10,
		ResponseTimeout: 5 * time.Second,
	}, rec)
	go func() { _ = live.Run(ctx, liveSub) }()

	waitForCond(t, "the live station's master to connect", live.Connected)

	pollCtx, pollCancel := context.WithTimeout(ctx, 20*time.Second)
	defer pollCancel()
	if err := live.IntegrityPoll(pollCtx); err != nil {
		t.Fatalf("the live station could not be polled while a dead station held the line: %v", err)
	}

	rec.mu.Lock()
	analog, ok := rec.analogs[0]
	rec.mu.Unlock()
	if !ok || analog != 11.25 {
		t.Errorf("analog 0 = %v (present=%v), want 11.25", analog, ok)
	}
}

// instantChannel connects immediately, for timing the open delay without
// touching a real port.
type instantChannel struct{}

func (c *instantChannel) Connect(ctx context.Context) (io.ReadWriteCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nopConn{}, nil
}

func (c *instantChannel) Close() error   { return nil }
func (c *instantChannel) String() string { return "instant" }

type nopConn struct{}

func (nopConn) Read(p []byte) (int, error)  { return 0, nil }
func (nopConn) Write(p []byte) (int, error) { return len(p), nil }
func (nopConn) Close() error                { return nil }
