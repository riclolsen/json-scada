/*
 * IEC 61850 MMS Client driver for {json:scada}, in Go.
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

package main

import (
	"strings"
	"sync"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jstags"

	"github.com/dscsystems/go-iec61850/client"
	"github.com/dscsystems/go-iec61850/model"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Driver identity, matching the C# driver so the same instance and
// connection documents drive either binary.
const (
	CopyrightMessage   = "{json:scada} IEC61850 Client Driver (Go) - Copyright 2020-2026 Ricardo Olsen"
	ProtocolDriverName = "IEC61850"
	DriverVersion      = "0.1.0"
	LibraryVersion     = "v0.2.5"
)

// Queue and key-allocation limits, same values as the C# driver.
const (
	DataBufferLimit   = 20000   // discard oldest above this when MongoDB is down
	BulkWriteLimit    = 1250    // maximum write models per bulk write
	AutoKeyMultiplier = 1000000 // _id range reserved per connection for auto-created tags
	PointKeyInsert    = 100000  // kept for parity with the C# driver (unused)
)

// Default config file locations, in the C# driver's resolution order.
const (
	JSONConfigFilePath    = "../conf/json-scada.json"
	JSONConfigFilePathAlt = "c:/json-scada/conf/json-scada.json"
)

// Iec61850Entry is one IEC 61850 object the driver reads or commands.
type Iec61850Entry struct {
	Path        string   // IEC 61850 object path, "LD/LN.DO[.DA]"
	FC          model.FC // functional constraint
	Childs      []string // names of the child attributes, when structured
	DataSetName string   // dataset containing the object, if any
	RcbName     string   // report control block reporting the object, if any
	JsTag       string   // json-scada tag to update (logging only for auto tags)

	// AutoPublish marks an entry the driver discovered itself, either by
	// browsing the server or from a report. Its values carry the
	// self-publish flag, so the writer creates the tag. Entries preloaded
	// from realtimeData already have a tag and never publish.
	AutoPublish bool
}

// Iec61850Connection is a document of protocolConnections plus the runtime
// state of the association it describes.
type Iec61850Connection struct {
	ID                            bson.ObjectID
	ProtocolDriver                string
	ProtocolDriverInstanceNumber  int
	ProtocolConnectionNumber      int
	Name                          string
	Description                   string
	Enabled                       bool
	CommandsEnabled               bool
	IPAddresses                   []string
	Topics                        []string
	AutoCreateTags                bool
	TimeoutMs                     float64
	Password                      string
	UseSecurity                   bool
	LocalCertFilePath             string
	PeerCertFilesPaths            []string
	RootCertFilePath              string
	ChainValidation               bool
	AllowOnlySpecificCertificates bool
	PrivateKeyFilePath            string
	CipherList                    string
	AllowTLSv10                   bool
	AllowTLSv11                   bool
	AllowTLSv12                   bool
	AllowTLSv13                   bool
	GiInterval                    float64
	Class0ScanInterval            float64
	UseBrcb                       bool
	UseUrcb                       bool
	Browse                        bool

	// Runtime state.
	mu            sync.Mutex
	LastReportIds map[string][]byte // rcb reference -> last seen EntryID
	Entries       map[string]*Iec61850Entry
	EntryOrder    []string // stable iteration order for the polling sweep
	InsertedTags  map[string]bool
	// TagKeys allocates _id values inside this connection's partition.
	TagKeys      jstags.KeyAllocator
	Cli          *client.Client
	Subs         []*client.ReportSubscription
	RcbByRptID   map[string]*rcbState
	RcbByDataSet map[string]*rcbState // data set -> the block reporting it
	Brcb         []string
	Urcb         []string
	Datasets     []string
	BrcbCount    int
}

// SetLastReportID records a buffered report's EntryID for resync.
func (c *Iec61850Connection) SetLastReportID(rcbRef string, entryID []byte) {
	c.mu.Lock()
	c.LastReportIds[rcbRef] = entryID
	c.BrcbCount++
	c.mu.Unlock()
}

// LastReportID returns the EntryID last seen on an RCB.
func (c *Iec61850Connection) LastReportID(rcbRef string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.LastReportIds[rcbRef]
	return v, ok
}

// SnapshotReportIDs copies the map for the redundancy writer.
func (c *Iec61850Connection) SnapshotReportIDs() map[string][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string][]byte, len(c.LastReportIds))
	for k, v := range c.LastReportIds {
		out[k] = v
	}
	return out
}

// Entry looks up a configured entry by object address and functional
// constraint, both as written in the tag document.
func (c *Iec61850Connection) Entry(key string) *Iec61850Entry {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Entries[key]
}

// Entry state is shared between the connection goroutine (discovery) and
// the association's reader goroutine (reports), so it is read and written
// through the connection lock.

// SetEntryDataSet records the data set an entry belongs to.
func (c *Iec61850Connection) SetEntryDataSet(e *Iec61850Entry, dataSet string) {
	c.mu.Lock()
	e.DataSetName = dataSet
	c.mu.Unlock()
}

// SetEntryReport records that an entry is delivered by a report, which
// takes it out of the polling sweep.
func (c *Iec61850Connection) SetEntryReport(e *Iec61850Entry, rcbName, dataSet string) {
	c.mu.Lock()
	e.RcbName = rcbName
	e.DataSetName = dataSet
	c.mu.Unlock()
}

// EntryHasReport reports whether an entry is covered by a report.
func (c *Iec61850Connection) EntryHasReport(e *Iec61850Entry) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return e.RcbName != ""
}

// EntryNeedsChilds reports whether the child attribute names of an entry
// are still unknown.
func (c *Iec61850Connection) EntryNeedsChilds(e *Iec61850Entry) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(e.Childs) == 0
}

// AddEntryChild records the name of a child attribute.
func (c *Iec61850Connection) AddEntryChild(e *Iec61850Entry, name string) {
	c.mu.Lock()
	e.Childs = append(e.Childs, name)
	c.mu.Unlock()
}

// AddEntryChildOnce records a child attribute name, keeping the order the
// server listed them in and ignoring repeats: a name list names an
// attribute once per leaf below it.
func (c *Iec61850Connection) AddEntryChildOnce(e *Iec61850Entry, name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, existing := range e.Childs {
		if existing == name {
			return
		}
	}
	e.Childs = append(e.Childs, name)
}

// EntryChilds copies the child attribute names, for logging.
func (c *Iec61850Connection) EntryChilds(e *Iec61850Entry) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), e.Childs...)
}

// Client returns the association, which the command goroutine reads while
// the connection goroutine may be replacing it.
func (c *Iec61850Connection) Client() *client.Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Cli
}

// SetClient stores the association.
func (c *Iec61850Connection) SetClient(cli *client.Client) {
	c.mu.Lock()
	c.Cli = cli
	c.mu.Unlock()
}

// AddSubscription records an active report subscription.
func (c *Iec61850Connection) AddSubscription(sub *client.ReportSubscription) {
	c.mu.Lock()
	c.Subs = append(c.Subs, sub)
	c.mu.Unlock()
}

// TakeSubscriptions removes and returns the active subscriptions.
func (c *Iec61850Connection) TakeSubscriptions() []*client.ReportSubscription {
	c.mu.Lock()
	defer c.mu.Unlock()
	subs := c.Subs
	c.Subs = nil
	return subs
}

// AddEntry registers an entry discovered from a report (autoCreateTags) and
// returns the entry now in effect, which is the existing one if another
// report registered it first.
func (c *Iec61850Connection) AddEntry(key string, e *Iec61850Entry) *Iec61850Entry {
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing, seen := c.Entries[key]; seen {
		return existing
	}
	c.Entries[key] = e
	c.EntryOrder = append(c.EntryOrder, key)
	return e
}

// IECValue is one acquired value on its way to MongoDB.
type IECValue struct {
	ValueJSON          string
	SelfPublish        bool
	Address            string
	Asdu               string
	IsDigital          bool
	IsTransient        bool
	Value              float64
	ValueString        string
	Cot                int
	ServerTimestamp    time.Time
	SourceTimestamp    time.Time
	HasSourceTimestamp bool
	Quality            bool
	ConnNumber         int
	ConnName           string
	CommonAddress      string
	DisplayName        string
}

// entryKey builds the map key used for configured entries: the object
// address concatenated with the functional constraint mnemonic, matching
// the C# driver's `dataRef + fc`.
func entryKey(path string, fc model.FC) string {
	return path + fc.String()
}

// parseFCOrST parses a functional constraint mnemonic. The C# driver uses
// Enum.TryParse, which leaves the zero value on failure, and libiec61850's
// FunctionalConstraint zero is ST — so an unparseable value means ST here
// too, not FCNone.
func parseFCOrST(s string) model.FC {
	fc, err := model.ParseFC(strings.ToUpper(strings.TrimSpace(s)))
	if err != nil || fc == model.FCNone {
		return model.ST
	}
	return fc
}

// --- permissive BSON accessors -------------------------------------------
//
// Configuration numbers are BSON doubles by convention but hand-edited
// documents carry int32/int64/strings. These mirror the C# driver's
// BsonDoubleSerializer: read almost anything, produce a number.

// connectionFromDoc maps a protocolConnections document onto the runtime
// struct, applying the same defaults as the C# BsonDefaultValue attributes.
func connectionFromDoc(doc bson.M) *Iec61850Connection {
	c := &Iec61850Connection{
		ProtocolDriver:                jsmongo.GetString(doc, "protocolDriver", ""),
		ProtocolDriverInstanceNumber:  jsmongo.GetInt(doc, "protocolDriverInstanceNumber", 1),
		ProtocolConnectionNumber:      jsmongo.GetInt(doc, "protocolConnectionNumber", 1),
		Name:                          jsmongo.GetString(doc, "name", "NO NAME"),
		Description:                   jsmongo.GetString(doc, "description", "SERVER NOT DESCRIPTED"),
		Enabled:                       jsmongo.GetBool(doc, "enabled", true),
		CommandsEnabled:               jsmongo.GetBool(doc, "commandsEnabled", true),
		IPAddresses:                   jsmongo.GetStringArray(doc, "ipAddresses"),
		Topics:                        jsmongo.GetStringArray(doc, "topics"),
		AutoCreateTags:                jsmongo.GetBool(doc, "autoCreateTags", true),
		TimeoutMs:                     jsmongo.GetDouble(doc, "timeoutMs", 20000),
		Password:                      jsmongo.GetString(doc, "password", ""),
		UseSecurity:                   jsmongo.GetBool(doc, "useSecurity", false),
		LocalCertFilePath:             jsmongo.GetString(doc, "localCertFilePath", ""),
		PeerCertFilesPaths:            jsmongo.GetStringArray(doc, "peerCertFilesPaths"),
		RootCertFilePath:              jsmongo.GetString(doc, "rootCertFilePath", ""),
		ChainValidation:               jsmongo.GetBool(doc, "chainValidation", false),
		AllowOnlySpecificCertificates: jsmongo.GetBool(doc, "allowOnlySpecificCertificates", false),
		PrivateKeyFilePath:            jsmongo.GetString(doc, "privateKeyFilePath", ""),
		CipherList:                    jsmongo.GetString(doc, "cipherList", ""),
		AllowTLSv10:                   jsmongo.GetBool(doc, "allowTLSv10", false),
		AllowTLSv11:                   jsmongo.GetBool(doc, "allowTLSv11", false),
		AllowTLSv12:                   jsmongo.GetBool(doc, "allowTLSv12", true),
		AllowTLSv13:                   jsmongo.GetBool(doc, "allowTLSv13", true),
		GiInterval:                    jsmongo.GetDouble(doc, "giInterval", 10),
		Class0ScanInterval:            jsmongo.GetDouble(doc, "class0ScanInterval", 300),
		UseBrcb:                       jsmongo.GetBool(doc, "useBrcb", true),
		UseUrcb:                       jsmongo.GetBool(doc, "useUrcb", true),
		Browse:                        jsmongo.GetBool(doc, "browse", false),
		LastReportIds:                 jsmongo.GetBinaryMap(doc, "lastReportIds"),
		Entries:                       map[string]*Iec61850Entry{},
		InsertedTags:                  map[string]bool{},
		RcbByRptID:                    map[string]*rcbState{},
		RcbByDataSet:                  map[string]*rcbState{},
	}
	if id, ok := doc["_id"].(bson.ObjectID); ok {
		c.ID = id
	}
	return c
}
