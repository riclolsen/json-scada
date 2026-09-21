package dnp3util

import (
	"strings"
	"testing"
)

// TestGroupKey checks the channel-sharing rules of tryReuseChannel(): active
// connections share on their address list, passive ones on the bind address,
// serial on the port name, UDP on both.
func TestGroupKey(t *testing.T) {
	same := func(a, b ChannelSpec) bool { return a.GroupKey() == b.GroupKey() }

	t.Run("tcp active shares on the remote list", func(t *testing.T) {
		a := ChannelSpec{Mode: ModeTCPActive, IPAddresses: []string{"10.0.0.5:20000"}}
		b := ChannelSpec{Mode: ModeTCPActive, IPAddresses: []string{"10.0.0.5:20000"}}
		c := ChannelSpec{Mode: ModeTCPActive, IPAddresses: []string{"10.0.0.6:20000"}}
		if !same(a, b) {
			t.Error("equal address lists must share a channel")
		}
		if same(a, c) {
			t.Error("different hosts must not share a channel")
		}
	})

	t.Run("the default port is applied before comparing", func(t *testing.T) {
		a := ChannelSpec{Mode: ModeTCPActive, IPAddresses: []string{"10.0.0.5"}}
		b := ChannelSpec{Mode: ModeTCPActive, IPAddresses: []string{"10.0.0.5:20000"}}
		if !same(a, b) {
			t.Error("a bare host must match the same host on the default port")
		}
	})

	t.Run("tcp passive shares on the bind address", func(t *testing.T) {
		a := ChannelSpec{Mode: ModeTCPPassive, IPAddressLocalBind: "0.0.0.0:20000"}
		b := ChannelSpec{Mode: ModeTCPPassive, IPAddressLocalBind: ":20000"}
		c := ChannelSpec{Mode: ModeTCPPassive, IPAddressLocalBind: "0.0.0.0:20001"}
		if !same(a, b) {
			t.Error("an empty host must normalise to 0.0.0.0")
		}
		if same(a, c) {
			t.Error("different ports must not share a channel")
		}
	})

	t.Run("serial shares on the port name", func(t *testing.T) {
		a := ChannelSpec{Mode: ModeSerial, PortName: "COM1"}
		b := ChannelSpec{Mode: ModeSerial, PortName: "com1"}
		c := ChannelSpec{Mode: ModeSerial, PortName: "COM2"}
		if !same(a, b) {
			t.Error("a port name must match case-insensitively")
		}
		if same(a, c) {
			t.Error("different ports must not share a channel")
		}
	})

	t.Run("udp shares on both endpoints", func(t *testing.T) {
		a := ChannelSpec{Mode: ModeUDP, IPAddressLocalBind: "0.0.0.0:20000", IPAddresses: []string{"10.0.0.5:20000"}}
		b := ChannelSpec{Mode: ModeUDP, IPAddressLocalBind: "0.0.0.0:20000", IPAddresses: []string{"10.0.0.5:20000"}}
		c := ChannelSpec{Mode: ModeUDP, IPAddressLocalBind: "0.0.0.0:20000", IPAddresses: []string{"10.0.0.6:20000"}}
		if !same(a, b) {
			t.Error("equal endpoints must share a channel")
		}
		if same(a, c) {
			t.Error("a different remote must not share a channel")
		}
	})

	t.Run("modes never share with each other", func(t *testing.T) {
		a := ChannelSpec{Mode: ModeTCPActive, IPAddresses: []string{"10.0.0.5:20000"}}
		b := ChannelSpec{Mode: ModeTLSActive, IPAddresses: []string{"10.0.0.5:20000"}}
		if same(a, b) {
			t.Error("TCP and TLS must not share a channel")
		}
	})
}

// TestBuildGroupsMultidrop checks that connections repeating one endpoint land
// on one bus, each with its own sub-channel.
func TestBuildGroupsMultidrop(t *testing.T) {
	spec := func(num, remote int) StationSpec {
		return StationSpec{
			ConnectionNumber:  num,
			Name:              "CONN" + string(rune('0'+num)),
			LocalLinkAddress:  1,
			RemoteLinkAddress: remote,
			Channel: ChannelSpec{
				Name: "shared", Mode: ModeTCPActive,
				IPAddresses: []string{"127.0.0.1:20999"},
			},
		}
	}

	groups, err := BuildGroups([]StationSpec{spec(1, 10), spec(2, 11), spec(3, 12)}, true)
	if err != nil {
		t.Fatalf("BuildGroups: %v", err)
	}
	defer func() {
		for _, g := range groups {
			g.Close()
		}
	}()

	if len(groups) != 1 {
		t.Fatalf("got %d groups, want 1", len(groups))
	}
	g := groups[0]
	if len(g.Channels) != 3 {
		t.Fatalf("got %d sub-channels, want 3", len(g.Channels))
	}
	for _, num := range []int{1, 2, 3} {
		if g.Channels[num] == nil {
			t.Errorf("connection %d has no channel", num)
		}
	}
	// The three sub-channels must be distinct.
	if g.Channels[1] == g.Channels[2] || g.Channels[2] == g.Channels[3] {
		t.Error("stations must not share one sub-channel")
	}
}

// TestBuildGroupsSeparateEndpoints checks that different endpoints get separate
// buses.
func TestBuildGroupsSeparateEndpoints(t *testing.T) {
	mk := func(num int, host string) StationSpec {
		return StationSpec{
			ConnectionNumber:  num,
			Name:              "CONN",
			LocalLinkAddress:  1,
			RemoteLinkAddress: 10,
			Channel: ChannelSpec{
				Name: "c", Mode: ModeTCPActive, IPAddresses: []string{host},
			},
		}
	}
	groups, err := BuildGroups([]StationSpec{mk(1, "127.0.0.1:20998"), mk(2, "127.0.0.2:20998")}, true)
	if err != nil {
		t.Fatalf("BuildGroups: %v", err)
	}
	defer func() {
		for _, g := range groups {
			g.Close()
		}
	}()
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2", len(groups))
	}
}

// TestBuildGroupsConflict pins deviation D21: two stations that cannot be told
// apart by link address are a configuration error, not something to start with
// one of them silently deaf.
func TestBuildGroupsConflict(t *testing.T) {
	mk := func(num int, name string) StationSpec {
		return StationSpec{
			ConnectionNumber:  num,
			Name:              name,
			LocalLinkAddress:  1,
			RemoteLinkAddress: 10, // the same outstation, twice
			Channel: ChannelSpec{
				Name: "shared", Mode: ModeTCPActive,
				IPAddresses: []string{"127.0.0.1:20997"},
			},
		}
	}
	groups, err := BuildGroups([]StationSpec{mk(1, "FIRST"), mk(2, "SECOND")}, true)
	for _, g := range groups {
		g.Close()
	}
	if err == nil {
		t.Fatal("two masters on one remote link address must be refused")
	}
	if !strings.Contains(err.Error(), "SECOND") {
		t.Errorf("the error must name the offending connection, got %q", err)
	}
}

// TestBuildGroupsOutstationConflict is the server-side equivalent: two
// outstations sharing a local link address are indistinguishable.
func TestBuildGroupsOutstationConflict(t *testing.T) {
	mk := func(num int, name string, local int) StationSpec {
		return StationSpec{
			ConnectionNumber:  num,
			Name:              name,
			LocalLinkAddress:  local,
			RemoteLinkAddress: 1,
			Channel: ChannelSpec{
				Name: "shared", Mode: ModeTCPPassive,
				IPAddressLocalBind: "127.0.0.1:20996",
			},
		}
	}
	groups, err := BuildGroups([]StationSpec{mk(1, "A", 10), mk(2, "B", 10)}, false)
	for _, g := range groups {
		g.Close()
	}
	if err == nil {
		t.Fatal("two outstations on one local link address must be refused")
	}

	// Distinct addresses are fine.
	groups, err = BuildGroups([]StationSpec{mk(1, "A", 10), mk(2, "B", 11)}, false)
	for _, g := range groups {
		g.Close()
	}
	if err != nil {
		t.Fatalf("distinct link addresses must be accepted: %v", err)
	}
}
