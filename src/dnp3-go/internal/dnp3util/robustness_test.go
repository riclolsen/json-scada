package dnp3util

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

// TestPassiveChannelRetriesBind: a listener that cannot bind at startup — the
// port still held by a previous process — must keep trying, not end the
// outstation for good while the process stays up.
func TestPassiveChannelRetriesBind(t *testing.T) {
	held, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := held.Addr().String()

	ch, _, err := ChannelSpec{Name: "S", Mode: ModeTCPPassive, IPAddressLocalBind: addr}.BuildChannel(nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ch.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	accepted := make(chan error, 1)
	go func() {
		conn, err := ch.Connect(ctx)
		if conn != nil {
			_ = conn.Close()
		}
		accepted <- err
	}()

	time.Sleep(300 * time.Millisecond) // at least one failed bind
	_ = held.Close()

	// Once the port is free the channel binds and accepts.
	var dialErr error
	for range 50 {
		var c net.Conn
		if c, dialErr = net.DialTimeout("tcp", addr, time.Second); dialErr == nil {
			_ = c.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if dialErr != nil {
		t.Fatalf("the channel never listened after the port was freed: %v", dialErr)
	}
	if err := <-accepted; err != nil {
		t.Fatalf("Connect: %v", err)
	}
}

func TestAllowedListAcrossAStation(t *testing.T) {
	spec := func(ips ...string) StationSpec { return StationSpec{AllowedRemoteIPs: ips} }
	if got := allowedOf([]StationSpec{spec("10.0.0.1"), spec("10.0.0.2:20000")}); len(got) != 2 {
		t.Errorf("lists not merged: %v", got)
	}
	if got := allowedOf([]StationSpec{spec("10.0.0.1"), spec()}); got != nil {
		t.Errorf("a station with no list must leave the endpoint open, got %v", got)
	}
}

func TestAllowedAddressesCompareCanonically(t *testing.T) {
	ch, _ := WrapCounting(&fakeChannel{}, CountOptions{AllowedRemoteIPs: []string{"::ffff:127.0.0.1", "localhost"}})
	c := ch.(*countChannel)
	if !c.allowed["127.0.0.1"] {
		t.Errorf("an IPv4-mapped address was not canonicalised: %v", c.allowed)
	}
	if len(c.allowedHosts) != 1 || c.allowedHosts[0] != "localhost" {
		t.Errorf("host name not kept for resolution: %v", c.allowedHosts)
	}
	if !c.resolvesTo("127.0.0.1") {
		t.Error("an allowed host name does not match the address it resolves to")
	}
}

func TestLinkAddressOutOfRange(t *testing.T) {
	_, err := BuildGroups([]StationSpec{{Name: "S", LocalLinkAddress: 1, RemoteLinkAddress: 65537,
		Channel: ChannelSpec{Name: "S", Mode: ModeTCPActive, IPAddresses: []string{"127.0.0.1:1"}}}}, true)
	if err == nil || !strings.Contains(err.Error(), "remoteLinkAddress") {
		t.Fatalf("an out-of-range link address was accepted: %v", err)
	}
}
