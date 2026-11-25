package dnp3sec

import (
	"net"
	"net/netip"

	"github.com/dscsystems/go-dnp3/netpeer"
	"github.com/dscsystems/go-dnp3/secevent"
)

func secevent0() secevent.PeerInfo { return secevent.PeerInfo{} }

func libCmdRejected(group uint8, index uint32) secevent.Event {
	return secevent.Event{Kind: secevent.KindCmdRejected, Fields: secevent.HasObject | secevent.HasIndex,
		Group: group, Index: index}
}

func netpeerObservation(ip, mac string) netpeer.Observation {
	hw, _ := net.ParseMAC(mac)
	return netpeer.Observation{IP: netip.MustParseAddr(ip), MAC: hw, Source: netpeer.SourceNeighbor}
}
