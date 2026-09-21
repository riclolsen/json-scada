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

// Grouping connections onto shared channels. Replaces tryReuseChannel() plus
// IChannel::AddMaster/AddOutstation of opendnp3, which allowed several sessions
// per channel; go-dnp3 gives one session one channel, and multidrop.Bus is what
// puts several back on one line.

package dnp3util

import (
	"fmt"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"github.com/dscsystems/go-dnp3/channel"
	"github.com/dscsystems/go-dnp3/multidrop"
)

// StationSpec is one connection's place on a channel.
type StationSpec struct {
	ConnectionNumber  int
	Name              string
	LocalLinkAddress  int
	RemoteLinkAddress int
	Channel           ChannelSpec
	// AllowedRemoteIPs is the ipAddresses list of a passive connection, which
	// the server driver documents as the clients allowed to connect.
	AllowedRemoteIPs []string
	// ScanHint is the shortest configured poll period in seconds, used only to
	// warn about pacing on a shared line. Zero means unknown.
	ScanHint int
}

// Group is one physical channel and the sub-channels of the connections on it.
type Group struct {
	Key      string
	Spec     ChannelSpec
	Bus      *multidrop.Bus
	Counters *Counters
	// Channels maps a protocolConnectionNumber onto the channel its session
	// runs on.
	Channels map[int]channel.Channel
	// Members lists the connection numbers on this channel, in configuration
	// order.
	Members []int
}

// Stats returns the bus counters, which carry the frame and CRC statistics no
// session-level API exposes.
func (g *Group) Stats() multidrop.Stats {
	if g == nil || g.Bus == nil {
		return multidrop.Stats{}
	}
	return g.Bus.Stats()
}

// Close shuts the bus and the underlying channel down.
func (g *Group) Close() {
	if g != nil && g.Bus != nil {
		_ = g.Bus.Close()
	}
}

// BuildGroups builds one bus per distinct endpoint and adds every connection to
// it as a station.
//
// master says whether these are master sessions (the client driver) or
// outstation sessions (the server driver); the bus routes and arbitrates
// differently for each.
//
// Every connection goes on a bus, including one that is alone on its endpoint:
// the uniform path is what makes the frame and CRC counters available for the
// stats document, and with a single station the arbiter never blocks.
func BuildGroups(specs []StationSpec, master bool) ([]*Group, error) {
	var groups []*Group
	byKey := map[string]*Group{}

	// One pass to group, so the turnaround can be decided from the final size
	// of each group rather than from the order connections were read in.
	order := []string{}
	members := map[string][]StationSpec{}
	for _, s := range specs {
		key := s.Channel.GroupKey()
		if _, seen := members[key]; !seen {
			order = append(order, key)
		}
		members[key] = append(members[key], s)
	}

	for _, key := range order {
		group := members[key]
		lead := group[0]

		ch, counters, err := lead.Channel.BuildChannel(lead.AllowedRemoteIPs)
		if err != nil {
			closeAll(groups)
			return nil, fmt.Errorf("%s - error creating channel: %w", lead.Name, err)
		}

		cfg := multidrop.Config{
			Log:        jslog.NewStackLogger(lead.Channel.Endpoint()),
			Turnaround: TurnaroundFor(lead.Channel, len(group)),
			Queue:      StationQueue,
		}

		bus := multidrop.New(ch, cfg)
		g := &Group{
			Key:      key,
			Spec:     lead.Channel,
			Bus:      bus,
			Counters: counters,
			Channels: map[int]channel.Channel{},
		}

		for _, s := range group {
			sub, err := bus.Add(multidrop.Station{
				LocalAddr:  uint16(s.LocalLinkAddress),
				RemoteAddr: uint16(s.RemoteLinkAddress),
				Master:     master,
			})
			if err != nil {
				// Two stations that cannot be told apart would both match the
				// same frames. That is a configuration error in
				// protocolConnections, and starting with one of them silently
				// deaf is worse than not starting.
				g.Close()
				closeAll(groups)
				return nil, fmt.Errorf("%s - cannot share %s: %w",
					s.Name, s.Channel.Endpoint(), err)
			}
			g.Channels[s.ConnectionNumber] = sub
			g.Members = append(g.Members, s.ConnectionNumber)
		}

		if len(group) > 1 {
			jslog.Log(jslog.LevelBasic, "%s - %d stations share this channel.",
				lead.Channel.Endpoint(), len(group))
			warnPacing(lead.Channel.Endpoint(), group, cfg.Turnaround)
		}

		groups = append(groups, g)
		byKey[key] = g
	}
	return groups, nil
}

// DefaultTurnaround mirrors the library's, for the pacing estimate below.
const DefaultTurnaround = multidrop.DefaultTurnaround

// StationQueue is how many link frames may be waiting for one session before
// the bus starts dropping them.
//
// The library's default is 16, on the reasoning that a queue only fills when a
// session has stopped reading. That holds on a serial line, where frames
// arrive a few per second. It does not hold on a socket: the response to an
// integrity poll of a large outstation arrives as hundreds of link frames back
// to back — a 12000-point database is well over a hundred — and the session
// goroutine decodes them one at a time while the bus pump reads at wire speed.
// Sixteen frames of slack is consumed almost immediately.
//
// A dropped link frame is not a dropped measurement. It is a hole in a
// transport segment, so the whole application fragment it belonged to is lost,
// and the poll fails with a response timeout. With autoCreateTags on, every
// point in that fragment is a tag that never gets created.
//
// Frames are at most 292 octets, so this costs about 1.2 MB per session in the
// worst case, and only while a session is behind.
const StationQueue = 4096

// TurnaroundFor decides whether the bus arbitrates transmission on a channel.
//
// A single session on its own TCP, TLS or UDP endpoint owns the medium: nothing
// else can be transmitting, so waiting for a turn would only add latency, and
// the library reads a negative turnaround as arbitration disabled. A serial
// line is always shared — with the other stations on it, and possibly with
// equipment this program does not own — and so is any endpoint carrying more
// than one connection, because that is what a terminal server fronting a real
// serial line looks like from here.
//
// Zero leaves the library's own default in force.
func TurnaroundFor(spec ChannelSpec, stations int) time.Duration {
	if stations <= 1 && !spec.IsSharedMedium() {
		return -1
	}
	return 0
}

// warnPacing points out a shared line whose configured poll rates cannot fit in
// the turnaround the bus enforces. The bus deliberately does not schedule
// sessions against each other, so this shows up in the field as unexplained
// timeouts unless somebody has done the arithmetic.
func warnPacing(endpoint string, group []StationSpec, turnaround time.Duration) {
	if turnaround < 0 {
		return
	}
	if turnaround == 0 {
		turnaround = DefaultTurnaround
	}
	exchanges := 0.0
	for _, s := range group {
		if s.ScanHint > 0 {
			exchanges += 1.0 / float64(s.ScanHint)
		}
	}
	if exchanges <= 0 {
		return
	}
	// One exchange may occupy the line for a whole turnaround when a station
	// does not answer.
	if exchanges*turnaround.Seconds() > 1.0 {
		jslog.Log(jslog.LevelBasic,
			"%s - %d stations share this line and their scan periods imply %.2f exchanges per second; "+
				"the line allows about %.2f. Check the scan intervals.",
			endpoint, len(group), exchanges, 1.0/turnaround.Seconds())
	}
}

func closeAll(groups []*Group) {
	for _, g := range groups {
		g.Close()
	}
}
