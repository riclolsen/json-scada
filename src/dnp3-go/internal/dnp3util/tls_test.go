package dnp3util

import (
	"context"
	"crypto/tls"
	"strings"
	"testing"
	"time"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/master"
	"github.com/dscsystems/go-dnp3/outstation"
)

// tlsSpecs builds the passive and active channel specifications for a TLS
// connection, exactly as the drivers build them from a protocolConnections
// document.
func tlsSpecs(certs certSet, addr string) (passive, active ChannelSpec) {
	passive = ChannelSpec{
		Name:               "SRV",
		Mode:               ModeTLSPassive,
		IPAddressLocalBind: addr,
		LocalCertFilePath:  certs.ServerCert,
		PrivateKeyFilePath: certs.ServerKey,
		PeerCertFilePath:   certs.CAFile,
		AllowTLSv12:        true,
		AllowTLSv13:        true,
	}
	active = ChannelSpec{
		Name:               "CLI",
		Mode:               ModeTLSActive,
		IPAddresses:        []string{addr},
		LocalCertFilePath:  certs.ClientCert,
		PrivateKeyFilePath: certs.ClientKey,
		PeerCertFilePath:   certs.CAFile,
		AllowTLSv12:        true,
		AllowTLSv13:        true,
	}
	return passive, active
}

// TestTLSLoopback runs a master against an outstation over a real mutually
// authenticated TLS connection, built from the same ChannelSpec fields a
// protocolConnections document supplies.
//
// This is the transport the loopback tests over channel.Pipe cannot reach: the
// certificate files, the TLS handshake and the socket are all real.
func TestTLSLoopback(t *testing.T) {
	certs := writeCerts(t)
	const addr = "127.0.0.1:20971"
	passiveSpec, activeSpec := tlsSpecs(certs, addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	passive, _, err := passiveSpec.BuildChannel(nil)
	if err != nil {
		t.Fatalf("building the passive TLS channel: %v", err)
	}
	defer func() { _ = passive.Close() }()

	out := outstation.New(outstation.Config{
		LocalAddr: 10, RemoteAddr: 1,
		Database:      outstation.DatabaseConfig{Analog: 1, Binary: 1, DefaultClass: dnp3.Class1},
		MaxTxFragment: 2048,
	}, outstation.NopApplication{}, nil)

	db := out.Database()
	if _, cfg, ok := db.Analog(0); ok {
		cfg.StaticVariation, cfg.EventVariation = 5, 7
		db.Configure(dnp3.TypeAnalog, 0, cfg)
	}
	db.UpdateAnalog(0, dnp3.Analog{Value: 42.5, Flags: dnp3.Online})
	db.UpdateBinary(0, dnp3.Binary{Value: true, Flags: dnp3.Online})
	out.Events().Reset()

	go func() { _ = out.Run(ctx, passive) }()

	active, counters, err := activeSpec.BuildChannel(nil)
	if err != nil {
		t.Fatalf("building the active TLS channel: %v", err)
	}
	defer func() { _ = active.Close() }()

	rec := newTLSRecorder()
	m := master.New(master.Config{
		LocalAddr: 1, RemoteAddr: 10,
		ResponseTimeout: 5 * time.Second,
	}, rec)
	go func() { _ = m.Run(ctx, active) }()

	waitForCond(t, "the TLS handshake to complete and the master to connect", m.Connected)

	if err := m.IntegrityPoll(ctx); err != nil {
		t.Fatalf("IntegrityPoll over TLS: %v", err)
	}

	rec.mu.Lock()
	analog, haveAnalog := rec.analogs[0]
	binary, haveBinary := rec.binaries[0]
	rec.mu.Unlock()

	if !haveAnalog || analog != 42.5 {
		t.Errorf("analog 0 over TLS = %v (present=%v), want 42.5", analog, haveAnalog)
	}
	if !haveBinary || !binary {
		t.Errorf("binary 0 over TLS = %v (present=%v), want true", binary, haveBinary)
	}

	// The counting wrapper sits beneath the TLS channel, so it sees the
	// encrypted stream: proof that the bytes really went through TLS rather
	// than a plain socket that happened to work.
	c := counters.Snapshot()
	if c.Opens != 1 {
		t.Errorf("opens = %d, want 1", c.Opens)
	}
	if c.BytesTx == 0 || c.BytesRx == 0 {
		t.Errorf("no bytes counted: tx=%d rx=%d", c.BytesTx, c.BytesRx)
	}
}

// TestTLSRejectsUnknownCA checks that peer verification is real: a client whose
// certificate comes from another authority must not get a session.
//
// go-dnp3's TLS is mutual-auth only, so this is the property the whole
// arrangement rests on. A test that only proved a good certificate works would
// pass just as happily against a stack that verified nothing.
func TestTLSRejectsUnknownCA(t *testing.T) {
	certs := writeCerts(t)
	const addr = "127.0.0.1:20972"
	passiveSpec, _ := tlsSpecs(certs, addr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	passive, _, err := passiveSpec.BuildChannel(nil)
	if err != nil {
		t.Fatalf("building the passive TLS channel: %v", err)
	}
	defer func() { _ = passive.Close() }()

	out := outstation.New(outstation.Config{
		LocalAddr: 10, RemoteAddr: 1,
		Database: outstation.DatabaseConfig{Analog: 1, DefaultClass: dnp3.Class1},
	}, outstation.NopApplication{}, nil)
	go func() { _ = out.Run(ctx, passive) }()

	// A client holding a certificate from an unrelated authority, and trusting
	// that authority rather than the outstation's.
	strangerSpec := ChannelSpec{
		Name:               "STRANGER",
		Mode:               ModeTLSActive,
		IPAddresses:        []string{addr},
		LocalCertFilePath:  certs.OtherClientCert,
		PrivateKeyFilePath: certs.OtherClientKey,
		PeerCertFilePath:   certs.OtherCAFile,
		AllowTLSv12:        true,
		AllowTLSv13:        true,
	}
	stranger, _, err := strangerSpec.BuildChannel(nil)
	if err != nil {
		t.Fatalf("building the stranger's channel: %v", err)
	}
	defer func() { _ = stranger.Close() }()

	m := master.New(master.Config{
		LocalAddr: 1, RemoteAddr: 10,
		ResponseTimeout: time.Second,
	}, master.NopHandler{})
	go func() { _ = m.Run(ctx, stranger) }()

	// Give it well past the time a good handshake takes.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if m.Connected() {
			t.Fatal("a client from an unknown authority was allowed to connect")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestTLSConfigMapping checks how the protocolConnections TLS fields map onto
// the library's configuration, including the two that cannot be honoured.
func TestTLSConfigMapping(t *testing.T) {
	certs := writeCerts(t)
	base := ChannelSpec{
		Name:               "C",
		Mode:               ModeTLSActive,
		LocalCertFilePath:  certs.ClientCert,
		PrivateKeyFilePath: certs.ClientKey,
		PeerCertFilePath:   certs.CAFile,
	}

	t.Run("the files map onto cert, key and CA", func(t *testing.T) {
		s := base
		s.AllowTLSv12, s.AllowTLSv13 = true, true
		cfg, err := s.tlsConfig()
		if err != nil {
			t.Fatalf("tlsConfig: %v", err)
		}
		if cfg.CertFile != certs.ClientCert {
			t.Errorf("CertFile = %q, want the localCertFilePath", cfg.CertFile)
		}
		if cfg.KeyFile != certs.ClientKey {
			t.Errorf("KeyFile = %q, want the privateKeyFilePath", cfg.KeyFile)
		}
		if cfg.CAFile != certs.CAFile {
			t.Errorf("CAFile = %q, want the peerCertFilePath", cfg.CAFile)
		}
	})

	t.Run("TLS 1.2 is the floor by default", func(t *testing.T) {
		s := base
		s.AllowTLSv12, s.AllowTLSv13 = true, true
		cfg, _ := s.tlsConfig()
		if cfg.MinVersion != tls.VersionTLS12 {
			t.Errorf("MinVersion = %#x, want TLS 1.2", cfg.MinVersion)
		}
	})

	t.Run("disallowing 1.2 raises the floor to 1.3", func(t *testing.T) {
		s := base
		s.AllowTLSv12, s.AllowTLSv13 = false, true
		cfg, _ := s.tlsConfig()
		if cfg.MinVersion != tls.VersionTLS13 {
			t.Errorf("MinVersion = %#x, want TLS 1.3", cfg.MinVersion)
		}
	})

	t.Run("the obsolete versions cannot lower the floor", func(t *testing.T) {
		// deviation D5: allowTLSv10 and allowTLSv11 are accepted and warned
		// about, but the library's floor is TLS 1.2 and cannot go below it.
		s := base
		s.AllowTLSv10, s.AllowTLSv11 = true, true
		s.AllowTLSv12, s.AllowTLSv13 = true, true
		cfg, _ := s.tlsConfig()
		if cfg.MinVersion != tls.VersionTLS12 {
			t.Errorf("MinVersion = %#x, want TLS 1.2 regardless of allowTLSv10/11", cfg.MinVersion)
		}
	})

	t.Run("a missing certificate is a configuration error", func(t *testing.T) {
		s := base
		s.LocalCertFilePath = ""
		if _, err := s.tlsConfig(); err == nil {
			t.Error("a missing localCertFilePath must be refused")
		} else if !strings.Contains(err.Error(), "localCertFilePath") {
			t.Errorf("the error must name the field, got %q", err)
		}

		s = base
		s.PrivateKeyFilePath = ""
		if _, err := s.tlsConfig(); err == nil {
			t.Error("a missing privateKeyFilePath must be refused")
		}
	})
}

// tlsRecorder collects what the master decodes over TLS.
type tlsRecorder struct {
	master.NopHandler
	mu       lockable
	analogs  map[uint16]float64
	binaries map[uint16]bool
}

func newTLSRecorder() *tlsRecorder {
	return &tlsRecorder{
		mu:       newLockable(),
		analogs:  map[uint16]float64{},
		binaries: map[uint16]bool{},
	}
}

func (r *tlsRecorder) HandleAnalog(_ master.HeaderInfo, vs []dnp3.Indexed[dnp3.Analog]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range vs {
		r.analogs[v.Index] = v.Value.Value
	}
}

func (r *tlsRecorder) HandleBinary(_ master.HeaderInfo, vs []dnp3.Indexed[dnp3.Binary]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range vs {
		r.binaries[v.Index] = v.Value.Value
	}
}
