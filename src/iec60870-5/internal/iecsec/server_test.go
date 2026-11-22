package iecsec

import (
	"fmt"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/go-iecp5/secevent"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// fakeConn is a 104 session as the driver's handlers see it.
type fakeConn struct{ peer secevent.PeerInfo }

func (f fakeConn) Params() *asdu.Params        { return asdu.ParamsWide }
func (f fakeConn) Send(*asdu.ASDU) error       { return nil }
func (f fakeConn) UnderlyingConn() net.Conn    { return nil }
func (f fakeConn) PeerInfo() secevent.PeerInfo { return f.peer }
func peerAt(addr string) fakeConn {
	return fakeConn{peer: secevent.PeerInfo{Network: "tcp", RemoteAddr: netip.MustParseAddrPort(addr)}}
}
func readASDU(ca asdu.CommonAddr) *asdu.ASDU {
	return asdu.NewASDU(asdu.ParamsWide, asdu.Identifier{Type: asdu.C_RD_NA_1,
		Variable: asdu.VariableStruct{Number: 1}, Coa: asdu.CauseOfTransmission{Cause: asdu.Request}, CommonAddr: ca})
}

func TestServerScanCountsDistinctObjects(t *testing.T) {
	rep, rec := newRep(true)
	s := NewServer(rep, nil, nil)
	c := peerAt("10.1.1.1:50000")

	// polling the same missing object is not a scan
	for i := 0; i < 3*ScanMax; i++ {
		s.UnknownRead(c, readASDU(1), 5000)
	}
	for _, k := range rec.kinds() {
		if k == string(secevent.KindAccessScan) {
			t.Fatal("repeated reads of one object reported as a scan")
		}
	}
	// many distinct missing objects is
	for ioa := 0; ioa <= ScanMax; ioa++ {
		s.UnknownRead(c, readASDU(1), 6000+ioa)
	}
	scans := 0
	for _, e := range rec.evs {
		if e.Kind == string(secevent.KindAccessScan) {
			scans++
			if e.PeerIP != "10.1.1.1" {
				t.Errorf("scan peerIp %q", e.PeerIP)
			}
		}
	}
	if scans != 1 {
		t.Fatalf("%d scan events, want 1", scans)
	}
}

func TestServerNotImplemented(t *testing.T) {
	rep, rec := newRep(true)
	s := NewServer(rep, nil, nil)
	c := peerAt("10.1.1.2:50000")
	mk := func(t asdu.TypeID) *asdu.ASDU {
		return asdu.NewASDU(asdu.ParamsWide, asdu.Identifier{Type: t, Variable: asdu.VariableStruct{Number: 1},
			Coa: asdu.CauseOfTransmission{Cause: asdu.Activation}, CommonAddr: 1})
	}
	s.NotImplemented(c, mk(asdu.C_SE_TA_1))
	s.NotImplemented(c, mk(asdu.F_FR_NA_1)) // monitor direction: go-iecp5 reports it
	s.NotImplemented(c, mk(asdu.F_SC_NA_1))
	k := rec.kinds()
	if len(k) != 2 || k[0] != string(secevent.KindAccessUnknownType) || k[1] != string(secevent.KindFileUpload) {
		t.Fatalf("events %v", k)
	}
}

// Masters loaded from stats (lastSeen a BSON date) are forgotten before a
// new one.
func TestServerKnownEvictsOldestStored(t *testing.T) {
	known := bson.A{}
	for i := 0; i < maxKnownPeers; i++ {
		known = append(known, bson.M{"ip": fmt.Sprintf("10.0.0.%d", i),
			"lastSeen": bson.NewDateTimeFromTime(time.Now().Add(-time.Duration(maxKnownPeers-i) * time.Minute))})
	}
	rep, rec := newRep(true)
	s := NewServer(rep, bson.M{"known": known}, nil)
	p := peerAt("10.9.9.9:50000")
	s.Opened("10.9.9.9:50000", p.peer)
	s.Closed("10.9.9.9:50000")
	s.Opened("10.9.9.9:50000", p.peer)
	if _, ok := s.known["10.9.9.9"]; !ok {
		t.Fatal("the new master was evicted")
	}
	if _, ok := s.known["10.0.0.0"]; ok {
		t.Fatal("the oldest stored master was kept")
	}
	if n := len(rec.kinds()); n != 1 {
		t.Fatalf("%d peer.identified events, want 1", n)
	}
}
