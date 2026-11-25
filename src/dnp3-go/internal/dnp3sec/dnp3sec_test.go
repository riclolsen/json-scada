package dnp3sec

import (
	"errors"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/objects"
	"github.com/dscsystems/go-dnp3/secevent"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestKeepLeavesOutRoutineEvents(t *testing.T) {
	r := NewReporter(nil, jssecevents.Origin{}, "client", true)
	cases := []struct {
		e    secevent.Event
		keep bool
	}{
		{secevent.Event{Kind: secevent.KindConnClosed, Severity: secevent.Info}, false},
		{secevent.Event{Kind: secevent.KindCmdReceived, Severity: secevent.Notice}, false},
		{secevent.Event{Kind: secevent.KindCtrlTimeWrite, Severity: secevent.Notice}, false},
		{secevent.Event{Kind: secevent.KindTLSCertRejected, Severity: secevent.Alert}, true},
		{secevent.Event{Kind: secevent.KindPeerMACChanged, Severity: secevent.Alert}, true},
		{secevent.Event{Kind: secevent.KindCtrlColdRestart, Severity: secevent.Alert}, true},
	}
	for _, c := range cases {
		if got := r.Keep(c.e); got != c.keep {
			t.Errorf("Keep(%s) = %v, want %v", c.e.Kind, got, c.keep)
		}
	}
	off := NewReporter(nil, jssecevents.Origin{}, "client", false)
	if off.Keep(secevent.Event{Kind: secevent.KindPeerMACChanged, Severity: secevent.Alert}) {
		t.Error("a MAC change kept with peerIdentification off")
	}
	if !off.Policy().DisableMACLookup || r.Policy().DisableMACLookup {
		t.Error("MAC lookups must follow peerIdentification")
	}
}

func TestDialFailedThrottled(t *testing.T) {
	var got capture
	r := NewReporter(nil, jssecevents.Origin{}, "client", true)
	r.Capture = got.add
	sink := r.Sink()
	dial := secevent.Event{Kind: secevent.KindConnDialFailed, Severity: secevent.Notice}
	for range 5 {
		sink.HandleEvent(dial)
	}
	if n := len(got.all()); n != 1 {
		t.Fatalf("stored %d dial failures, want 1 per period", n)
	}
	sink.HandleEvent(secevent.Event{Kind: secevent.KindConnConnected, Severity: secevent.Info})
	sink.HandleEvent(dial)
	if n := len(got.all()); n != 2 {
		t.Errorf("stored %d after a connection, want the next outage reported at once", n)
	}
}

func TestFromLib(t *testing.T) {
	e := secevent.Event{Kind: secevent.KindAccessUnknownObject, Category: secevent.CatAccess, Severity: secevent.Warning,
		Action: secevent.Rejected, ConnID: 7, Transport: "tcp",
		Fields:  secevent.HasAppFunc | secevent.HasObject | secevent.HasIndex | secevent.HasLinkAddrs,
		AppFunc: 1, Group: 30, Variation: 5, Index: 12, LinkSrc: 1, LinkDest: 10,
		Peer: secevent.PeerInfo{RemoteAddr: netip.MustParseAddrPort("10.1.2.3:20000"),
			MAC: net.HardwareAddr{0, 1, 2, 3, 4, 5}, MACSource: secevent.MACNeighbor},
		Raw: []byte{0x05, 0x64}}
	d := FromLib(e)
	if d.Kind != "access.unknown_object" || d.Category != "access" || d.Severity != "warning" ||
		d.Action != "rejected" || d.Source != SourceLibrary || d.SessionID != 7 || d.FC != "READ" {
		t.Errorf("FromLib = %+v", d)
	}
	if d.ObjectRef != "g30v5 #12" || d.PeerIP != "10.1.2.3" || d.PeerMAC != "00:01:02:03:04:05" {
		t.Errorf("object/peer = %q %q %q", d.ObjectRef, d.PeerIP, d.PeerMAC)
	}
	if d.Fields["linkSource"] != 1 || d.Fields["linkDestination"] != 10 || d.Fields["raw"] != "0564" {
		t.Errorf("fields = %v", d.Fields)
	}
	// A gateway's MAC is not the peer's.
	e.Peer.MACSource = secevent.MACGateway
	if FromLib(e).PeerMAC != "" {
		t.Error("a gateway MAC reported as the peer's")
	}
}

func TestDropClientRefusals(t *testing.T) {
	// go-dnp3 gives the response's function, not the request's.
	attrs := secevent.Event{Kind: secevent.KindRemoteRejected, Fields: secevent.HasAppFunc, AppFunc: 129,
		Detail: "read-attributes refused: NO_FUNC_CODE_SUPPORT"}
	iinCtl := secevent.Event{Kind: secevent.KindRemoteRejected, Fields: secevent.HasAppFunc, AppFunc: 129,
		Detail: "direct-operate refused: NO_FUNC_CODE_SUPPORT"}
	read := secevent.Event{Kind: secevent.KindRemoteRejected, Fields: secevent.HasAppFunc, AppFunc: 129,
		Detail: "integrity refused: OBJECT_UNKNOWN"}
	status := secevent.Event{Kind: secevent.KindRemoteRejected, Fields: secevent.HasCommandStatus}
	if !DropClientRefusals(status) || !DropClientRefusals(attrs) {
		t.Error("a refused control status or attribute read is stored")
	}
	if DropClientRefusals(iinCtl) || DropClientRefusals(read) ||
		DropClientRefusals(secevent.Event{Kind: secevent.KindRemoteRestart}) {
		t.Error("a request refused by IIN, a refused read or a restart is dropped")
	}
}

func TestAttributesUnsupportedAreAskedOnEveryConnection(t *testing.T) {
	var got capture
	r := NewReporter(nil, jssecevents.Origin{}, "client", true)
	r.Capture = got.add
	c := NewClient(r, nil)
	gen, read := c.Connected(secevent.PeerInfo{}, 10)
	if !read {
		t.Fatal("first connection: attributes not to be read")
	}
	c.Attributes(gen, nil, errors.New("wrapped: "+dnp3.ErrNotSupported.Error()))
	if c.PeerIdentity()["method"] != "error" {
		t.Errorf("a failed read = %v", c.PeerIdentity())
	}
	if n := len(got.all()); n != 1 || got.all()[0].Kind != KindPeerIdentified {
		t.Errorf("events = %v, want one peer.identified", got.all())
	}
	gen, _ = c.Connected(secevent.PeerInfo{}, 10)
	c.Attributes(gen, nil, dnp3.ErrNotSupported)
	if c.PeerIdentity()["method"] != "unsupported" {
		t.Errorf("method = %v, want unsupported", c.PeerIdentity()["method"])
	}
	old := gen
	gen, read = c.Connected(secevent.PeerInfo{}, 10)
	if !read {
		t.Error("attributes not asked again: the device may have been replaced")
	}
	// A late answer for a replaced connection is ignored.
	c.Attributes(old, []dnp3.Attribute{objects.StringAttribute(252, "LATE")}, nil)
	if c.PeerIdentity()["vendor"] != nil {
		t.Error("attributes of a replaced connection recorded")
	}
	c.Attributes(gen, []dnp3.Attribute{objects.StringAttribute(252, "NEW")}, nil)
	if c.PeerIdentity()["vendor"] != "NEW" || c.PeerIdentity()["method"] != "attributes" {
		t.Errorf("identity of the replacement = %v", c.PeerIdentity())
	}
	n := 0
	for _, e := range got.all() {
		if e.Kind == KindPeerIdentified {
			n++
		}
	}
	if n != 1 {
		t.Errorf("peer.identified reported %d times: %v", n, got.all())
	}
}

func TestClientIdentityOff(t *testing.T) {
	stored := bson.M{"ip": "10.0.0.1", "keys": bson.A{bson.M{"k": "vendor", "v": "X"}}}
	c := NewClient(NewReporter(nil, jssecevents.Origin{}, "client", false), stored)
	if _, read := c.Connected(secevent.PeerInfo{}, 1); read {
		t.Error("attributes read with peerIdentification off")
	}
	if c.PeerIdentity() != nil {
		t.Error("an identity published with peerIdentification off")
	}
}

func TestClientCommandEvents(t *testing.T) {
	var got capture
	r := NewReporter(nil, jssecevents.Origin{}, "client", true)
	r.Capture = got.add
	c := NewClient(r, nil)
	doc := bson.M{"tag": "SP1", "value": 1e9}
	c.Refused(KindCmdWrongType, doc, "the setpoint does not fit its variation", 41, 2, 3)
	c.Stale(doc, time.Now().Add(-time.Minute), 41, 2, 3)
	for range CommandFloodMax + 1 {
		c.Command(nil, doc, 1, false, 41, 2, 3)
	}
	c.Answer(doc, []dnp3.CommandStatus{dnp3.CommandSuccess}, 41, 2, 3)
	c.Answer(doc, []dnp3.CommandStatus{dnp3.CommandBlocked}, 41, 2, 3)
	kinds := map[string]jssecevents.Event{}
	for _, e := range got.all() {
		kinds[e.Kind] = e
	}
	for _, k := range []string{KindCmdWrongType, KindCmdStaleTime, "flood.commands", KindCmdRejected} {
		e, ok := kinds[k]
		if !ok {
			t.Errorf("no %s in %v", k, got.all())
			continue
		}
		if e.Tag != "SP1" || e.ObjectRef != "g41v2 #3" || e.Role != "client" || e.Protocol != "dnp3" {
			t.Errorf("%s = %+v", k, e)
		}
	}
	if e := kinds[KindCmdRejected]; e.Value != "BLOCKED" {
		t.Errorf("cmd.rejected value = %q, want the status", e.Value)
	}
	if n := len(got.all()); n != 4 {
		t.Errorf("stored %d events, want 4 (an accepted command is not reported)", n)
	}
}

func TestServerKnownAndSeeded(t *testing.T) {
	stored := bson.M{"known": bson.A{bson.M{"ip": "10.0.0.5", "mac": "00:01:02:03:04:05", "linkAddress": 1,
		"lastSeen": time.Now().Add(-time.Hour)}}}
	r := NewReporter(nil, jssecevents.Origin{}, "server", true)
	var got capture
	r.Capture = got.add
	s := NewServer(r, stored, 1, nil)
	// The stored MAC seeds the connection's peer history: another one is a
	// change.
	ch := r.cache.Observe(netpeerObservation("10.0.0.5", "00:01:02:03:04:06"))
	if len(ch) == 0 {
		t.Error("the stored MAC did not seed the peer history")
	}
	s.Opened(s.Opening(), secevent.PeerInfo{RemoteAddr: netip.MustParseAddrPort("10.0.0.5:4000")})
	if len(got.all()) != 0 {
		t.Errorf("a known master reported as new: %v", got.all())
	}
	s.Opened(s.Opening(), secevent.PeerInfo{RemoteAddr: netip.MustParseAddrPort("10.0.0.6:4000")})
	if len(got.all()) != 1 || got.all()[0].Kind != KindPeerIdentified {
		t.Errorf("a new master = %v, want peer.identified", got.all())
	}
	doc := s.StatsDoc()
	if k, _ := doc["known"].(bson.A); len(k) != 2 {
		t.Errorf("known = %v", doc["known"])
	}
	s.Closed()
	if c, _ := s.StatsDoc()["clients"].(bson.A); len(c) != 0 {
		t.Errorf("clients after close = %v", c)
	}
	// The peer of a connection that closed before it was described is not
	// published afterwards.
	late := s.Opening()
	s.Closed()
	s.Opened(late, secevent.PeerInfo{RemoteAddr: netip.MustParseAddrPort("10.0.0.7:4000")})
	if c, _ := s.StatsDoc()["clients"].(bson.A); len(c) != 0 {
		t.Errorf("clients after a late open = %v", c)
	}
	if NewServer(NewReporter(nil, jssecevents.Origin{}, "server", false), stored, 1, nil).StatsDoc() != nil {
		t.Error("statsDoc with peerIdentification off, want nil")
	}
}
