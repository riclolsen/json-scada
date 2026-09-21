/*
 * DNP3 Outstation Server Protocol driver for {json:scada}, in Go.
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

// The connection and destination model. Ports of DNP3Connection_t and of the
// protocolDestinations entries the C++ server reads.

package serverapp

import (
	"context"
	"strings"
	"sync"

	"dnp3-go/internal/dnp3util"

	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"github.com/dscsystems/go-dnp3/channel"
	"github.com/dscsystems/go-dnp3/outstation"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Driver identity.
const (
	ProtocolDriverName = "DNP3_SERVER"
	DriverVersion      = "0.2.0"
	DriverMessage      = "{json:scada} DNP3 Outstation Server Driver (Go)"
)

// Destination is one entry of a tag's protocolDestinations array.
type Destination struct {
	ConnectionNumber int
	CommonAddress    int
	ObjectAddress    int
	ASDU             int
	CommandDuration  float64
	CommandUseSBO    bool
	KConv1           float64
	KConv2           float64
	Group            int
	HoursShift       float64
}

// DestinationFromDoc reads one protocolDestinations entry.
func DestinationFromDoc(d bson.M) Destination {
	return Destination{
		ConnectionNumber: jsmongo.GetInt(d, "protocolDestinationConnectionNumber", 0),
		CommonAddress:    jsmongo.GetInt(d, "protocolDestinationCommonAddress", 0),
		ObjectAddress:    jsmongo.GetInt(d, "protocolDestinationObjectAddress", 0),
		ASDU:             jsmongo.GetInt(d, "protocolDestinationASDU", 0),
		CommandDuration:  jsmongo.GetDouble(d, "protocolDestinationCommandDuration", 0),
		CommandUseSBO:    jsmongo.GetBool(d, "protocolDestinationCommandUseSBO", false),
		KConv1:           jsmongo.GetDouble(d, "protocolDestinationKConv1", 1),
		KConv2:           jsmongo.GetDouble(d, "protocolDestinationKConv2", 0),
		Group:            jsmongo.GetInt(d, "protocolDestinationGroup", 0),
		HoursShift:       jsmongo.GetDouble(d, "protocolDestinationHoursShift", 0),
	}
}

// DestinationsOf reads the whole protocolDestinations array of a tag.
func DestinationsOf(doc bson.M) []Destination {
	entries := jsmongo.GetDocArray(doc, "protocolDestinations")
	out := make([]Destination, 0, len(entries))
	for _, e := range entries {
		out = append(out, DestinationFromDoc(e))
	}
	return out
}

// Connection is one protocolConnections document plus its outstation session.
type Connection struct {
	ProtocolConnectionNumber int
	Name                     string
	Description              string
	Enabled                  bool
	CommandsEnabled          bool
	AutoCreateTags           bool
	Topics                   []string
	ConnectionMode           string
	IPAddressLocalBind       string
	IPAddresses              []string
	PortName                 string
	BaudRate                 int
	Parity                   string
	StopBits                 string
	Handshake                string
	AsyncOpenDelay           int
	LocalLinkAddress         int
	RemoteLinkAddress        int
	TimeSyncInterval         int
	TimeSyncMode             int
	HoursShift               float64
	EnableUnsolicited        bool
	ServerQueueSize          int

	LocalCertFilePath  string
	PrivateKeyFilePath string
	PeerCertFilePath   string
	CipherList         string
	AllowTLSv10        bool
	AllowTLSv11        bool
	AllowTLSv12        bool
	AllowTLSv13        bool

	// Runtime state.
	Chan  channel.Channel
	Group *dnp3util.Group

	mu      sync.Mutex
	station *outstation.Session

	// link reports whether the session currently has a connection. An
	// outstation has no Connected() of its own, so the state is read from the
	// channel it runs on.
	link *dnp3util.LinkState
	// counts of the point families sized into the database, for logging.
	counts outstation.DatabaseConfig
}

// Station returns the outstation session, or nil before it is built.
func (c *Connection) Station() *outstation.Session {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.station
}

func (c *Connection) setStation(s *outstation.Session) {
	c.mu.Lock()
	c.station = s
	c.mu.Unlock()
}

// Update applies a change to the outstation database, if the session exists.
func (c *Connection) Update(fn func(*outstation.Database)) {
	if s := c.Station(); s != nil {
		s.Update(fn)
	}
}

// MatchesTopic reports whether a tag's group1 matches this connection's topic
// filter. An empty filter matches everything.
func (c *Connection) MatchesTopic(group1 string) bool {
	if len(c.Topics) == 0 {
		return true
	}
	for _, t := range c.Topics {
		// parity: the C++ server uses a substring match on group1, not
		// equality, so a topic of "KAW" also selects "KAW2".
		if t != "" && strings.Contains(group1, t) {
			return true
		}
	}
	return false
}

// ChannelSpec describes this connection's physical channel.
func (c *Connection) ChannelSpec() dnp3util.ChannelSpec {
	return dnp3util.ChannelSpec{
		Name:               c.Name,
		Mode:               c.ConnectionMode,
		IPAddresses:        c.IPAddresses,
		IPAddressLocalBind: c.IPAddressLocalBind,
		PortName:           c.PortName,
		BaudRate:           c.BaudRate,
		Parity:             c.Parity,
		StopBits:           c.StopBits,
		Handshake:          c.Handshake,
		AsyncOpenDelayMs:   c.AsyncOpenDelay,
		LocalCertFilePath:  c.LocalCertFilePath,
		PeerCertFilePath:   c.PeerCertFilePath,
		PrivateKeyFilePath: c.PrivateKeyFilePath,
		CipherList:         c.CipherList,
		AllowTLSv10:        c.AllowTLSv10,
		AllowTLSv11:        c.AllowTLSv11,
		AllowTLSv12:        c.AllowTLSv12,
		AllowTLSv13:        c.AllowTLSv13,
	}
}

// runCtx is the context every outstation session runs under.
type runCtx = context.Context
