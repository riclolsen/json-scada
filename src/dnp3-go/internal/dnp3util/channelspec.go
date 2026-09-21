/*
 * DNP3 Client and Server Protocol drivers for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

// The physical channel of a protocolConnections document: how to build it, and
// what makes two connections share one. Port of createChannel() and
// tryReuseChannel() of the C++ client and of the equivalent block of the C++
// server.

package dnp3util

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/dscsystems/go-dnp3/channel"
)

// Connection modes, upper-cased as both C++ drivers upper-case them on load.
const (
	ModeTCPActive  = "TCP ACTIVE"
	ModeTCPPassive = "TCP PASSIVE"
	ModeTLSActive  = "TLS ACTIVE"
	ModeTLSPassive = "TLS PASSIVE"
	ModeSerial     = "SERIAL"
	ModeUDP        = "UDP"
)

// ChannelSpec is everything about a connection that decides its physical
// channel.
type ChannelSpec struct {
	Name               string
	Mode               string
	IPAddresses        []string
	IPAddressLocalBind string
	PortName           string
	BaudRate           int
	Parity             string
	StopBits           string
	Handshake          string
	AsyncOpenDelayMs   int

	LocalCertFilePath  string
	PeerCertFilePath   string
	PrivateKeyFilePath string
	CipherList         string
	AllowTLSv10        bool
	AllowTLSv11        bool
	AllowTLSv12        bool
	AllowTLSv13        bool
}

// IsPassive reports whether the channel listens rather than dials.
func (s ChannelSpec) IsPassive() bool {
	return s.Mode == ModeTCPPassive || s.Mode == ModeTLSPassive
}

// IsSharedMedium reports whether the far end is, or may be, a line shared by
// several stations. It decides whether the multi-drop bus arbitrates
// transmission: a dedicated socket needs no turn taking, a serial line does,
// and a TCP connection to a terminal server is a serial line with a longer
// wire, which only shows up as more than one connection on the endpoint.
func (s ChannelSpec) IsSharedMedium() bool {
	return s.Mode == ModeSerial
}

// GroupKey identifies the physical channel. Connections with equal keys share
// one channel, which is how multi-drop is configured: repeat the endpoint and
// vary the link addresses.
//
// The rules are those of tryReuseChannel(): active connections share a channel
// when their address lists match, passive ones when their bind addresses match,
// serial ones when the port name matches, and UDP when both match.
func (s ChannelSpec) GroupKey() string {
	switch s.Mode {
	case ModeTCPActive, ModeTLSActive:
		return s.Mode + "|" + strings.Join(s.normalizedRemotes(), ",")
	case ModeTCPPassive, ModeTLSPassive:
		return s.Mode + "|" + NormalizeEndpoint(s.IPAddressLocalBind, "0.0.0.0")
	case ModeSerial:
		return s.Mode + "|" + strings.ToUpper(s.PortName)
	case ModeUDP:
		return s.Mode + "|" + NormalizeEndpoint(s.IPAddressLocalBind, "0.0.0.0") +
			"|" + strings.Join(s.normalizedRemotes(), ",")
	default:
		return s.Mode + "|" + s.Name
	}
}

func (s ChannelSpec) normalizedRemotes() []string {
	out := make([]string, 0, len(s.IPAddresses))
	for _, a := range s.IPAddresses {
		out = append(out, NormalizeEndpoint(a, ""))
	}
	return out
}

// Endpoint renders the channel for a log line.
func (s ChannelSpec) Endpoint() string {
	switch s.Mode {
	case ModeSerial:
		return s.PortName
	case ModeTCPActive, ModeTLSActive:
		if len(s.IPAddresses) > 0 {
			return NormalizeEndpoint(s.IPAddresses[0], "")
		}
		return "?"
	default:
		return NormalizeEndpoint(s.IPAddressLocalBind, "0.0.0.0")
	}
}

// tlsConfig maps the TLS fields of a connection onto the library's.
//
// go-dnp3 is mutual-auth only with a TLS 1.2 floor, so allowTLSv10 and
// allowTLSv11 cannot be honoured and a cipher list cannot be set. Both are
// reported rather than ignored silently, because a connection that a site
// believes is running TLS 1.0 will not come up.
func (s ChannelSpec) tlsConfig() (channel.TLSConfig, error) {
	if s.LocalCertFilePath == "" {
		return channel.TLSConfig{}, fmt.Errorf("missing localCertFilePath parameter")
	}
	if s.PrivateKeyFilePath == "" {
		return channel.TLSConfig{}, fmt.Errorf("missing privateKeyFilePath parameter")
	}
	if s.AllowTLSv10 || s.AllowTLSv11 {
		jslog.Log(jslog.LevelBasic,
			"%s - allowTLSv10/allowTLSv11 are not supported; TLS 1.2 is the lowest version offered", s.Name)
	}
	if s.CipherList != "" {
		jslog.Log(jslog.LevelBasic,
			"%s - cipherList is not supported; the Go TLS defaults are used", s.Name)
	}
	cfg := channel.TLSConfig{
		CertFile:   s.LocalCertFilePath,
		KeyFile:    s.PrivateKeyFilePath,
		CAFile:     s.PeerCertFilePath,
		MinVersion: tls.VersionTLS12,
	}
	// Only 1.3 allowed means 1.3 is also the floor.
	if !s.AllowTLSv12 && s.AllowTLSv13 {
		cfg.MinVersion = tls.VersionTLS13
	}
	return cfg, nil
}

// serialConfig maps the serial fields, applying the same defaults the C++
// drivers apply.
func (s ChannelSpec) serialConfig() channel.SerialConfig {
	cfg := channel.SerialConfig{
		Device:      s.PortName,
		Baud:        s.BaudRate,
		DataBits:    8,
		Parity:      channel.ParityNone,
		StopBits:    channel.StopBits1,
		ReadTimeout: time.Second,
	}
	switch strings.ToUpper(s.Parity) {
	case "EVEN":
		cfg.Parity = channel.ParityEven
	case "ODD":
		cfg.Parity = channel.ParityOdd
	}
	switch strings.ToUpper(s.StopBits) {
	case "TWO", "2":
		cfg.StopBits = channel.StopBits2
	case "ONE5", "ONE.FIVE", "1.5":
		// The C++ drivers fold one-and-a-half stop bits onto two, because
		// opendnp3's SerialSettings has no other value for it. go-dnp3 carries
		// it, so the documented One5 setting is honoured (deviation D22).
		cfg.StopBits = channel.StopBits1Point5
	}
	return cfg
}

// BuildChannel constructs the physical channel and wraps it in the counters,
// the allowed-address filter and the open delay.
func (s ChannelSpec) BuildChannel(allowedRemoteIPs []string) (channel.Channel, *Counters, error) {
	var (
		inners []channel.Channel
		inner  channel.Channel
		err    error
	)

	switch s.Mode {
	case ModeTCPActive:
		remotes, rerr := s.remoteEndpoints()
		if rerr != nil {
			return nil, nil, rerr
		}
		for _, addr := range remotes {
			inners = append(inners, channel.TCPClient(addr, channel.NoRetry))
		}

	case ModeTCPPassive:
		inner = channel.TCPServer(NormalizeEndpoint(s.IPAddressLocalBind, "0.0.0.0"))

	case ModeTLSActive:
		remotes, rerr := s.remoteEndpoints()
		if rerr != nil {
			return nil, nil, rerr
		}
		tlsCfg, terr := s.tlsConfig()
		if terr != nil {
			return nil, nil, terr
		}
		for _, addr := range remotes {
			ch, cerr := channel.TLSClient(addr, tlsCfg, channel.NoRetry)
			if cerr != nil {
				return nil, nil, cerr
			}
			inners = append(inners, ch)
		}

	case ModeTLSPassive:
		tlsCfg, terr := s.tlsConfig()
		if terr != nil {
			return nil, nil, terr
		}
		inner, err = channel.TLSServer(NormalizeEndpoint(s.IPAddressLocalBind, "0.0.0.0"), tlsCfg)

	case ModeSerial:
		if s.PortName == "" {
			return nil, nil, fmt.Errorf("missing portName parameter")
		}
		inner = channel.SerialChannel(s.serialConfig(), channel.NoRetry)

	case ModeUDP:
		if s.IPAddressLocalBind == "" {
			return nil, nil, fmt.Errorf("missing ipAddressLocalBind parameter")
		}
		if len(s.IPAddresses) == 0 || strings.TrimSpace(s.IPAddresses[0]) == "" {
			return nil, nil, fmt.Errorf("invalid list of ipAddresses parameter")
		}
		inner = channel.UDPChannel(channel.UDPConfig{
			LocalAddr:  NormalizeEndpoint(s.IPAddressLocalBind, "0.0.0.0"),
			RemoteAddr: NormalizeEndpoint(s.IPAddresses[0], ""),
		})

	default:
		return nil, nil, fmt.Errorf("unsupported connectionMode: %s", s.Mode)
	}
	if err != nil {
		return nil, nil, err
	}
	if inners == nil {
		inners = []channel.Channel{inner}
	}

	opts := CountOptions{Name: s.Name}
	if s.IsPassive() {
		// A passive channel blocks on accept rather than failing, so it needs
		// no backoff; the allowed-address list is what it does need.
		opts.AllowedRemoteIPs = allowedRemoteIPs
	} else {
		// The inner channel is built with NoRetry so that each attempt surfaces
		// here, where it can be counted and the real error logged.
		opts.Retry = DefaultRetry
	}
	if s.Mode == ModeSerial && s.AsyncOpenDelayMs > 0 {
		opts.OpenDelay = time.Duration(s.AsyncOpenDelayMs) * time.Millisecond
	}
	wrapped, counters := WrapCountingAll(inners, opts)
	return wrapped, counters, nil
}

// remoteEndpoints returns every address an active connection may dial.
//
// The C++ drivers use only the first and their documentation says as much;
// here the whole list is an ordered set of alternatives for one device, tried
// in turn until one answers (deviation D26).
func (s ChannelSpec) remoteEndpoints() ([]string, error) {
	out := make([]string, 0, len(s.IPAddresses))
	for _, a := range s.IPAddresses {
		if strings.TrimSpace(a) == "" {
			continue
		}
		out = append(out, NormalizeEndpoint(a, ""))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("invalid list of ipAddresses parameter")
	}
	return out, nil
}
