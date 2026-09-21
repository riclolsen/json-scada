/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
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

// Documents, permissive BSON decoding and the MongoDB connection.
// Port of Common_srv_cli.cs.

package main

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jstags"

	"github.com/gopcua/opcua"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Driver identity, matching the C# driver so the same instance and
// connection documents drive either binary.
const (
	CopyrightMessage   = "{json:scada} OPC-UA Client Driver (Go) - Copyright 2020-2026 Ricardo L. Olsen"
	ProtocolDriverName = "OPC-UA"
	DriverVersion      = "0.1.0"
	LibraryVersion     = "gopcua v0.9.1"
)

// Queue and key-allocation limits, same values as the C# driver.
const (
	DataBufferLimit   = 50000   // stop enqueuing acquired values above this
	BulkWriteLimit    = 6000    // maximum write models per bulk write
	AutoKeyMultiplier = 1000000 // _id range reserved per connection for auto-created tags
	PointKeyInsert    = 100000  // kept for parity with the C# driver (unused)
)

// Default config file locations, in the C# driver's resolution order.
const (
	JSONConfigFilePath    = "../conf/json-scada.json"
	JSONConfigFilePathAlt = "c:/json-scada/conf/json-scada.json"
)

// Counters reported by the redundancy loop, as in the C# driver.
var (
	CntNotificEvents   atomic.Uint64
	CntLostDataUpdates atomic.Uint64
)

// NodeDetails is what browsing learned about a node, kept so the MongoDB
// writer can fill in the tag document of a value that arrived through a
// subscription notification (which carries neither path nor parent).
// Port of the NodeDetails class of AsduReceiveHandler.cs.
type NodeDetails struct {
	DisplayName string
	BrowseName  string
	ParentName  string
	Path        string
}

// monItem is one monitored item: the OPC UA subscription reports values by
// ClientHandle only, so the driver owns the mapping back to the address and
// the display name.
//
// The C# driver builds two parallel objects for a preconfigured tag (an
// rtMonitTag and a MonitoredItem) carrying identical parameters; one struct
// serves both roles here.
type monItem struct {
	// NodeID is the address exactly as it must appear in
	// protocolSourceObjectAddress. For a preconfigured tag it is the
	// verbatim string from the document, never a re-rendered NodeID, or
	// the update filter would stop matching.
	NodeID      string
	DisplayName string
	SamplingMs  float64
	QueueSize   uint32
	Handle      uint32
}

// OPCUAConnection is a document of protocolConnections plus the runtime
// state of the session it describes.
type OPCUAConnection struct {
	ID                              bson.ObjectID
	ProtocolDriver                  string
	ProtocolDriverInstanceNumber    int
	ProtocolConnectionNumber        int
	Name                            string
	Description                     string
	Enabled                         bool
	CommandsEnabled                 bool
	EndpointURLs                    []string
	ConfigFileName                  string
	AutoCreateTags                  bool
	AutoCreateTagPublishingInterval float64
	AutoCreateTagSamplingInterval   float64
	AutoCreateTagQueueSize          float64
	TimeoutMs                       float64
	UseSecurity                     bool
	HoursShift                      float64
	GiInterval                      float64 // parity: declared, never used (deviation D12)
	Topics                          []string
	Username                        string
	Password                        string
	PfxFilePath                     string
	Passphrase                      string
	LocalCertFilePath               string
	SecurityMode                    string
	SecurityPolicy                  string
	AutoAcceptUntrustedCertificates bool

	// Runtime state, guarded by mu: the connection goroutine writes it
	// while the MongoDB writer and the command dispatcher read it.
	mu                sync.Mutex
	InsertedAddresses map[string]bool
	NodeIdsDetails    map[string]*NodeDetails
	// TagKeys allocates _id values inside this connection's partition.
	TagKeys    jstags.KeyAllocator
	handles    map[uint32]*monItem
	nextHandle uint32
	client     *opcua.Client
	// valueReadChunk adapts to what the server will answer; see
	// ValueReadChunk.
	valueReadChunk int

	// ListMon holds every monitored item of the connection, preconfigured
	// ones first. It is the subscription content when autoCreateTags is
	// on; OpcSubscriptions is used instead when it is off.
	ListMon []*monItem

	// OpcSubscriptions groups preconfigured items by their publishing
	// interval in seconds. SubscriptionOrder keeps the order the intervals
	// were first seen, so subscriptions are created deterministically.
	OpcSubscriptions  map[float64][]*monItem
	SubscriptionOrder []float64
}

// OPCValue is one acquired value on its way to MongoDB.
// Port of the OPC_Value class of Common_srv_cli.cs.
type OPCValue struct {
	CreateCommandForMethod     bool
	CreateCommandForSupervised bool
	AccessLevels               byte
	ValueJSON                  string
	SelfPublish                bool
	Address                    string
	Asdu                       string
	IsArray                    bool
	Value                      float64
	ValueString                string
	Cot                        int
	ServerTimestamp            time.Time
	SourceTimestamp            time.Time
	HasSourceTimestamp         bool
	Quality                    bool
	ConnNumber                 int
	ConnName                   string
	CommonAddress              string
	DisplayName                string
	ParentName                 string
	Path                       string
}

// --- connection runtime accessors ----------------------------------------

// AddInsertedAddress records an address as present in realtimeData and
// reports whether it was new, like SortedSet<string>.Add in C#.
func (c *OPCUAConnection) AddInsertedAddress(addr string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.InsertedAddresses[addr] {
		return false
	}
	c.InsertedAddresses[addr] = true
	return true
}

// HasInsertedAddress reports whether the address already has a tag.
func (c *OPCUAConnection) HasInsertedAddress(addr string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.InsertedAddresses[addr]
}

// SetNodeDetails records what browsing learned about a node.
func (c *OPCUAConnection) SetNodeDetails(nodeID string, d *NodeDetails) {
	c.mu.Lock()
	c.NodeIdsDetails[nodeID] = d
	c.mu.Unlock()
}

// NodeDetailsFor returns what browsing learned about a node, if anything.
func (c *OPCUAConnection) NodeDetailsFor(nodeID string) (*NodeDetails, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	d, ok := c.NodeIdsDetails[nodeID]
	return d, ok
}

// NewHandle allocates a client handle for a monitored item and registers
// it, so notifications can be mapped back to the address.
func (c *OPCUAConnection) NewHandle(it *monItem) uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextHandle++
	it.Handle = c.nextHandle
	c.handles[it.Handle] = it
	return it.Handle
}

// ItemForHandle resolves the monitored item a notification belongs to.
func (c *OPCUAConnection) ItemForHandle(h uint32) *monItem {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.handles[h]
}

// ValueReadChunk is how many node values the driver asks for in one Read.
// It starts at the C# batch size and halves whenever a discovery attempt
// dies inside a value read, because some servers close the connection
// instead of answering when the response would be too large. Without this
// the retry would fail identically forever.
func (c *OPCUAConnection) ValueReadChunk() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.valueReadChunk <= 0 {
		c.valueReadChunk = maxNodesToRead
	}
	return c.valueReadChunk
}

// ShrinkValueReadChunk halves the value-read size, returning the new size
// and whether there was any room left to shrink.
func (c *OPCUAConnection) ShrinkValueReadChunk() (int, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.valueReadChunk <= 0 {
		c.valueReadChunk = maxNodesToRead
	}
	if c.valueReadChunk <= minValueReadChunk {
		return c.valueReadChunk, false
	}
	c.valueReadChunk = max(c.valueReadChunk/2, minValueReadChunk)
	return c.valueReadChunk, true
}

// ResetDiscovery drops everything the last discovery built, so the tags can
// be reloaded from the database and browsing can start over.
func (c *OPCUAConnection) ResetDiscovery() {
	c.mu.Lock()
	c.ListMon = nil
	c.OpcSubscriptions = map[float64][]*monItem{}
	c.SubscriptionOrder = nil
	c.InsertedAddresses = map[string]bool{}
	c.NodeIdsDetails = map[string]*NodeDetails{}
	c.handles = map[uint32]*monItem{}
	c.nextHandle = 0
	c.mu.Unlock()
	// Forces the _id allocator to look up the highest key again, which a
	// partially completed pass will have moved.
	c.TagKeys.Reset()
}

// CommitPreloadedTags installs the tags read from realtimeData.
func (c *OPCUAConnection) CommitPreloadedTags(listMon []*monItem, subs map[float64][]*monItem, order []float64, addrs map[string]bool) {
	c.mu.Lock()
	c.ListMon = listMon
	c.OpcSubscriptions = subs
	c.SubscriptionOrder = order
	c.InsertedAddresses = addrs
	c.mu.Unlock()
}

// Client returns the session, which the command dispatcher reads while the
// connection goroutine may be replacing it.
func (c *OPCUAConnection) Client() *opcua.Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.client
}

// setClient stores the session, or clears it when the connection is down.
// Clearing also drops the monitored item handles: the next session issues
// its own, and a stale handle would resolve to the wrong item.
func (c *OPCUAConnection) setClient(cli *opcua.Client) {
	c.mu.Lock()
	c.client = cli
	if cli == nil {
		c.handles = map[uint32]*monItem{}
		c.nextHandle = 0
	}
	c.mu.Unlock()
}

// --- startup configuration -----------------------------------------------

// --- permissive BSON accessors -------------------------------------------
//
// Configuration numbers are BSON doubles by convention but hand-edited
// documents carry int32/int64/strings. These mirror the C# driver's
// BsonDoubleSerializer: read almost anything, produce a number.

// --- document mapping ----------------------------------------------------

// connectionFromDoc maps a protocolConnections document onto the runtime
// struct, applying the same defaults as the C# BsonDefaultValue attributes.
func connectionFromDoc(doc bson.M) *OPCUAConnection {
	c := &OPCUAConnection{
		ProtocolDriver:                  jsmongo.GetString(doc, "protocolDriver", ""),
		ProtocolDriverInstanceNumber:    jsmongo.GetInt(doc, "protocolDriverInstanceNumber", 1),
		ProtocolConnectionNumber:        jsmongo.GetInt(doc, "protocolConnectionNumber", 1),
		Name:                            jsmongo.GetString(doc, "name", "NO NAME"),
		Description:                     jsmongo.GetString(doc, "description", "SERVER NOT DESCRIPTED"),
		Enabled:                         jsmongo.GetBool(doc, "enabled", true),
		CommandsEnabled:                 jsmongo.GetBool(doc, "commandsEnabled", true),
		EndpointURLs:                    jsmongo.GetStringArray(doc, "endpointURLs"),
		ConfigFileName:                  jsmongo.GetString(doc, "configFileName", "../conf/Opc.Ua.DefaultClient.Config.xml"),
		AutoCreateTags:                  jsmongo.GetBool(doc, "autoCreateTags", true),
		AutoCreateTagPublishingInterval: jsmongo.GetDouble(doc, "autoCreateTagPublishingInterval", 5.0),
		AutoCreateTagSamplingInterval:   jsmongo.GetDouble(doc, "autoCreateTagSamplingInterval", 5.0),
		AutoCreateTagQueueSize:          jsmongo.GetDouble(doc, "autoCreateTagQueueSize", 5.0),
		TimeoutMs:                       jsmongo.GetDouble(doc, "timeoutMs", 20000),
		UseSecurity:                     jsmongo.GetBool(doc, "useSecurity", false),
		HoursShift:                      jsmongo.GetDouble(doc, "hoursShift", 0),
		GiInterval:                      jsmongo.GetDouble(doc, "giInterval", 300),
		Topics:                          jsmongo.GetStringArray(doc, "topics"),
		Username:                        jsmongo.GetString(doc, "username", ""),
		Password:                        jsmongo.GetString(doc, "password", ""),
		PfxFilePath:                     jsmongo.GetString(doc, "pfxFilePath", ""),
		Passphrase:                      jsmongo.GetString(doc, "passphrase", ""),
		LocalCertFilePath:               jsmongo.GetString(doc, "localCertFilePath", ""),
		SecurityMode:                    jsmongo.GetString(doc, "securityMode", "None"),
		SecurityPolicy:                  jsmongo.GetString(doc, "securityPolicy", "None"),
		AutoAcceptUntrustedCertificates: jsmongo.GetBool(doc, "autoAcceptUntrustedCertificates", true),

		InsertedAddresses: map[string]bool{},
		NodeIdsDetails:    map[string]*NodeDetails{},
		OpcSubscriptions:  map[float64][]*monItem{},
		handles:           map[uint32]*monItem{},
	}
	if id, ok := doc["_id"].(bson.ObjectID); ok {
		c.ID = id
	}
	return c
}

// connByNumber finds a connection by its protocolConnectionNumber.
func connByNumber(conns []*OPCUAConnection, number int) *OPCUAConnection {
	for _, c := range conns {
		if c.ProtocolConnectionNumber == number {
			return c
		}
	}
	return nil
}
