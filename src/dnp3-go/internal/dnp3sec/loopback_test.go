package dnp3sec

import (
	"context"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"testing"
	"time"

	"dnp3-go/internal/dnp3util"

	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/master"
	"github.com/dscsystems/go-dnp3/objects"
	"github.com/dscsystems/go-dnp3/outstation"
	"github.com/dscsystems/go-dnp3/secevent"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// capture collects the events a reporter stores.
type capture struct {
	mu     sync.Mutex
	events []jssecevents.Event
}

func (c *capture) add(e jssecevents.Event) {
	c.mu.Lock()
	c.events = append(c.events, e)
	c.mu.Unlock()
}

func (c *capture) all() []jssecevents.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]jssecevents.Event(nil), c.events...)
}

// wait returns the first event match accepts, failing after a timeout.
func (c *capture) wait(t *testing.T, what string, match func(jssecevents.Event) bool) jssecevents.Event {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range c.all() {
			if match(e) {
				return e
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	var kinds []string
	for _, e := range c.all() {
		kinds = append(kinds, e.Kind+"/"+e.Source)
	}
	t.Fatalf("timed out waiting for %s; got %v", what, kinds)
	return jssecevents.Event{}
}

func kindIs(kind, source string) func(jssecevents.Event) bool {
	return func(e jssecevents.Event) bool { return e.Kind == kind && (source == "" || e.Source == source) }
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("pick port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestLoopbackOverTheDriverBus runs a master and an outstation over a real
// TCP socket, each on the bus the drivers build (dnp3util.BuildGroups): the
// sessions on it describe the peer of the bus line, the client identifies
// the outstation from its device attributes, and a control the outstation
// does not have is reported on both sides.
func TestLoopbackOverTheDriverBus(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port := freePort(t)
	addr := "127.0.0.1:" + strconv.Itoa(port)

	srvEvents, cliEvents := &capture{}, &capture{}
	srvRep := NewReporter(nil, jssecevents.Origin{}, "server", true)
	srvRep.Capture = srvEvents.add
	srv := NewServer(srvRep, nil, 1, nil)
	srvRep.Drop = srv.DropServerRefusals
	srvPolicy := srvRep.Policy()

	outGroups, err := dnp3util.BuildGroups([]dnp3util.StationSpec{{ConnectionNumber: 1, Name: "SRV",
		LocalLinkAddress: 10, RemoteLinkAddress: 1, Security: srvPolicy,
		Channel: dnp3util.ChannelSpec{Name: "SRV", Mode: dnp3util.ModeTCPPassive, IPAddressLocalBind: addr}}}, false)
	if err != nil {
		t.Fatalf("outstation bus: %v", err)
	}
	defer outGroups[0].Close()
	out := outstation.New(outstation.Config{LocalAddr: 10, RemoteAddr: 1, Security: srvPolicy,
		Database:    outstation.DatabaseConfig{Binary: 1},
		Unsolicited: outstation.UnsolicitedConfig{Enabled: true},
		Attributes: []dnp3.Attribute{
			objects.StringAttribute(252, "ACME"),
			objects.StringAttribute(250, "RTU-9"),
			objects.StringAttribute(242, "3.1.4"),
		}}, outstation.NopApplication{}, nil)
	go func() { _ = out.Run(ctx, outGroups[0].Channels[1]) }()

	cliRep := NewReporter(nil, jssecevents.Origin{}, "client", true)
	cliRep.Capture = cliEvents.add
	cliRep.Drop = DropClientRefusals
	stored, _ := bson.Marshal(bson.M{"keys": bson.A{bson.M{"k": "softwareVersion", "v": "3.0.0"}}})
	var storedDoc bson.M
	_ = bson.Unmarshal(stored, &storedDoc)
	cli := NewClient(cliRep, storedDoc)
	cliPolicy := cliRep.Policy()

	mGroups, err := dnp3util.BuildGroups([]dnp3util.StationSpec{{ConnectionNumber: 2, Name: "CLI",
		LocalLinkAddress: 1, RemoteLinkAddress: 10, Security: cliPolicy,
		Channel: dnp3util.ChannelSpec{Name: "CLI", Mode: dnp3util.ModeTCPActive, IPAddresses: []string{addr}}}}, true)
	if err != nil {
		t.Fatalf("master bus: %v", err)
	}
	defer mGroups[0].Close()
	m := master.New(master.Config{LocalAddr: 1, RemoteAddr: 10, ResponseTimeout: 3 * time.Second,
		IntegrityOnStartup: true, Security: cliPolicy, UnsolClassMask: dnp3.Class123}, master.NopHandler{})
	go func() { _ = m.Run(ctx, mGroups[0].Channels[2]) }()
	waitFor(t, "the master to connect", m.Connected)

	// The sessions on a bus know the peer of the line.
	p, ok := m.PeerInfo()
	if !ok || p.RemoteAddr.String() != addr {
		t.Fatalf("master PeerInfo = %+v %v, want %s", p, ok, addr)
	}
	waitFor(t, "the outstation to know its master", func() bool {
		sp, ok := out.PeerInfo()
		return ok && sp.RemoteAddr.Addr().String() == "127.0.0.1"
	})
	sp, _ := out.PeerInfo()
	srv.Opened(srv.Opening(), sp)
	if c, _ := srv.StatsDoc()["clients"].(bson.A); len(c) != 1 || c[0].(bson.M)["ip"] != "127.0.0.1" {
		t.Errorf("server peerIdentity.clients = %v", c)
	}
	srvEvents.wait(t, "peer.identified (server)", kindIs(KindPeerIdentified, SourceDriver))

	// peer.identified is reported once the attributes were read, not for
	// the transport alone.
	gen, read := cli.Connected(p, 10)
	if !read {
		t.Fatal("Connected: attributes not to be read")
	}
	for _, e := range cliEvents.all() {
		if e.Kind == KindPeerIdentified {
			t.Errorf("peer.identified before the attributes: %+v", e)
		}
	}
	// The client identifies the outstation.
	attrs, err := m.ReadAttributes(ctx)
	cli.Attributes(gen, attrs, err)
	id := cli.PeerIdentity()
	if id["vendor"] != "ACME" || id["model"] != "RTU-9" || id["softwareVersion"] != "3.1.4" ||
		id["method"] != "attributes" || id["ip"] != "127.0.0.1" || id["linkAddress"] != 10 {
		t.Errorf("client identity = %v", id)
	}
	e := cliEvents.wait(t, "peer.identity_changed", kindIs(KindPeerIdentityChanged, SourceDriver))
	if e.ObjectRef != "softwareVersion" || e.Value != "3.1.4" || e.Previous != "3.0.0" || e.PeerIP != "127.0.0.1" {
		t.Errorf("peer.identity_changed = %+v", e)
	}

	// A control the outstation has no handler for: the library reports it
	// at the outstation, the driver at the master (with the command).
	doc := bson.M{"tag": "BRK1", "value": 1.0}
	res, _ := m.DirectOperate(ctx, master.CROB(5, dnp3.ControlRelayOutputBlock{Code: dnp3.ControlLatchOn, Count: 1}))
	cli.Answer(doc, res.Statuses, 12, 1, 5)
	e = cliEvents.wait(t, "cmd.rejected (client)", kindIs(KindCmdRejected, SourceDriver))
	if e.Tag != "BRK1" || e.ObjectRef != "g12v1 #5" || e.Role != "client" || e.Value != "NOT_SUPPORTED" {
		t.Errorf("client cmd.rejected = %+v", e)
	}
	e = srvEvents.wait(t, "cmd.rejected (library)", kindIs(KindCmdRejected, SourceLibrary))
	if e.PeerIP != "127.0.0.1" || e.ObjectRef == "" || e.Role != "server" {
		t.Errorf("library cmd.rejected = %+v", e)
	}
	time.Sleep(300 * time.Millisecond)
	for _, e := range cliEvents.all() {
		if e.Kind == "remote.rejected" {
			t.Errorf("library remote.rejected of a control stored besides the driver's: %+v", e)
		}
	}
	for _, e := range append(srvEvents.all(), cliEvents.all()...) {
		t.Logf("%s %s/%s %s %s %q", e.Role, e.Kind, e.Source, e.Severity, e.ObjectRef, e.Detail)
	}
}

// TestDropServerRefusals: a control the driver refused (and reported) is
// not stored a second time from go-dnp3.
func TestDropServerRefusals(t *testing.T) {
	var got capture
	rep := NewReporter(nil, jssecevents.Origin{}, "server", true)
	rep.Capture = got.add
	s := NewServer(rep, nil, 1, nil)
	s.Refused(secevent0(), Refusal{Kind: KindCmdObjectUnknown, Service: "SELECT", Group: 12, Index: 7,
		Status: "NOT_SUPPORTED", Why: "no command tag at this index"})
	s.Refused(secevent0(), Refusal{Kind: KindCmdObjectUnknown, Service: "SELECT", Group: 12, Index: 7,
		Status: "NOT_SUPPORTED", Why: "no command tag at this index"})
	if n := len(got.all()); n != 1 {
		t.Fatalf("stored %d refusals, want 1 per period", n)
	}
	// The same refusal to another master is its own event.
	other := secevent.PeerInfo{RemoteAddr: netip.MustParseAddrPort("10.0.0.9:20000")}
	s.Refused(other, Refusal{Kind: KindCmdObjectUnknown, Service: "SELECT", Group: 12, Index: 7,
		Status: "NOT_SUPPORTED", Why: "no command tag at this index"})
	if n := len(got.all()); n != 2 || got.all()[1].PeerIP != "10.0.0.9" {
		t.Fatalf("refusal to another master = %v", got.all())
	}
	lib := libCmdRejected(12, 7)
	if !s.DropServerRefusals(lib) {
		t.Error("library cmd.rejected of a refusal the driver reported is kept")
	}
	if s.DropServerRefusals(libCmdRejected(12, 8)) {
		t.Error("library cmd.rejected of another index is dropped")
	}
}
