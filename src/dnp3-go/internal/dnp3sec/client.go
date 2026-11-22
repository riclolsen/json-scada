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

package dnp3sec

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/secevent"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Command flood limit of a client connection: more than CommandFloodMax
// commands to the device within CommandFloodWindow.
const (
	CommandFloodMax    = 10
	CommandFloodWindow = 10 * time.Second
	// maxAttributes bounds the attributes kept in stats.peerIdentity.
	maxAttributes = 64
)

// identityAttrs are the device attributes (group 0, set 0) that identify an
// outstation, by IEEE 1815-2012 variation: the field of stats.peerIdentity
// and the label of its change. Each is compared between connections.
var identityAttrs = []struct {
	variation uint8
	field     string
	label     string
}{
	{252, "vendor", "manufacturer name"},
	{250, "model", "product name and model"},
	{243, "hardwareVersion", "hardware version"},
	{242, "softwareVersion", "software version"},
	{248, "serialNumber", "serial number"},
	{249, "dnp3Subset", "DNP3 subset and conformance"},
	{247, "deviceName", "user-assigned device name"},
	{246, "deviceId", "user-assigned ID code"},
	{245, "location", "user-assigned location"},
	{196, "configId", "configuration ID"},
	{197, "configVersion", "configuration version"},
	{200, "configSignature", "configuration signature"},
}

// Key is one compared part of a peer's identity: an attribute, the MAC
// address, the certificate.
type Key struct {
	K, V  string
	Label string
}

// libKeys are the keys go-dnp3 compares itself (its peer cache, seeded from
// the stored identity): they are stored, never reported here.
var libKeys = map[string]bool{"mac": true, "cert": true, "linkAddress": true}

// Client is the security state of one client (master) connection: the
// identity of its outstation in stats.peerIdentity and its changes, and the
// command events (refused, unknown object, stale, flood, unnecessary).
type Client struct {
	Rep *Reporter

	mu       sync.Mutex
	changes  jssecevents.Tracker
	flood    jssecevents.RateLimit
	identity bson.M // stats.peerIdentity
	session  bson.M // what this connection learned of the outstation
	keys     map[string]Key
	peer     secevent.PeerInfo
	gen      uint64 // the current connection
	// identified: peer.identified was reported (or an identity was stored
	// before the driver started).
	identified bool
}

// NewClient starts the change detection from the identity stored in the
// connection's statistics, so a change across a driver restart is seen.
func NewClient(rep *Reporter, stored bson.M) *Client {
	c := &Client{Rep: rep, flood: jssecevents.RateLimit{Max: CommandFloodMax, Window: CommandFloodWindow}}
	if stored == nil || !c.ident() {
		return c
	}
	c.identity, c.identified = stored, true
	mac, cert := "", ""
	for _, k := range jsmongo.GetDocArray(stored, "keys") {
		kk, v := jsmongo.GetString(k, "k", ""), jsmongo.GetString(k, "v", "")
		c.changes.Seed(kk, v)
		switch kk {
		case "mac":
			mac = v
		case "cert":
			cert = v
		}
	}
	c.Rep.Seed(jsmongo.GetString(stored, "ip", ""), mac, jsmongo.GetInt(stored, "linkAddress", -1), cert)
	return c
}

func (c *Client) ident() bool { return c.Rep != nil && c.Rep.Ident }

// PeerIdentity is stats.peerIdentity, nil when never identified or when
// peerIdentification is off.
func (c *Client) PeerIdentity() bson.M {
	if c == nil || !c.ident() {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.identity
}

// Emit stores an event about this connection's outstation.
func (c *Client) Emit(e jssecevents.Event) {
	if c == nil {
		return
	}
	c.mu.Lock()
	p := c.peer
	c.mu.Unlock()
	SetPeer(&e, p)
	c.Rep.Emit(e)
}

// Connected starts the identity of a connection that just came up: the
// peer go-dnp3 describes (address, MAC, TLS session) and the outstation's
// link address. It returns the connection's generation for Attributes, and
// whether the device attributes should be read: on every connection, as the
// device behind the address may have been replaced by one that has them.
func (c *Client) Connected(p secevent.PeerInfo, linkAddress int) (gen uint64, readAttrs bool) {
	if c == nil {
		return 0, false
	}
	c.mu.Lock()
	c.peer = p
	c.mu.Unlock()
	if !c.ident() {
		return 0, false
	}
	base := peerBase(p)
	base["linkAddress"] = linkAddress
	keys := []Key{{K: "linkAddress", V: fmt.Sprint(linkAddress), Label: "link address"}}
	if mac, _ := base["mac"].(string); mac != "" {
		keys = append(keys, Key{K: "mac", V: mac, Label: "MAC address"})
	}
	if p.TLS != nil && p.TLS.FingerprintSHA256 != "" {
		keys = append(keys, Key{K: "cert", V: p.TLS.FingerprintSHA256, Label: "certificate"})
	}
	c.mu.Lock()
	c.gen++
	gen = c.gen
	c.session, c.keys = base, map[string]Key{}
	c.mu.Unlock()
	c.observe(keys)
	return gen, true
}

// Attributes adds what the outstation says about itself (its group 0
// attributes, read on connection gen) to the identity and reports what
// changed; err is the read's failure. A connection that has been replaced
// meanwhile is ignored.
func (c *Client) Attributes(gen uint64, attrs []dnp3.Attribute, err error) {
	if c == nil || !c.ident() {
		return
	}
	c.mu.Lock()
	if c.gen != gen || c.session == nil {
		c.mu.Unlock()
		return
	}
	s := c.session
	s["collectedAt"] = time.Now()
	var keys []Key
	switch {
	case errors.Is(err, dnp3.ErrNotSupported):
		s["method"] = "unsupported"
	case err != nil:
		s["method"], s["error"] = "error", err.Error()
	default:
		s["method"] = "attributes"
		byVar := map[uint8]dnp3.Attribute{}
		list := bson.A{}
		for _, a := range attrs {
			if a.Set != dnp3.AttrSetStandard {
				continue
			}
			byVar[a.Variation] = a
			if len(list) < maxAttributes && a.Type != dnp3.AttrAttributeList && a.Type != dnp3.AttrExtAttributeList {
				name, _ := dnp3.AttributeName(a.Variation)
				list = append(list, bson.M{"variation": int(a.Variation), "name": name, "value": a.Value()})
			}
		}
		for _, ia := range identityAttrs {
			if a, ok := byVar[ia.variation]; ok {
				v := strings.TrimSpace(a.Value())
				s[ia.field] = v
				keys = append(keys, Key{K: ia.field, V: v, Label: ia.label})
			}
		}
		if len(list) > 0 {
			s["attributes"] = list
		}
	}
	c.mu.Unlock()
	c.observe(keys)
}

func (c *Client) observe(keys []Key) {
	for _, k := range keys {
		if k.V == "" {
			continue
		}
		prev, changed := c.changes.Observe(k.K, k.V)
		c.mu.Lock()
		if c.keys == nil {
			c.keys = map[string]Key{}
		}
		c.keys[k.K] = k
		c.mu.Unlock()
		if !changed || libKeys[k.K] {
			continue
		}
		c.Emit(jssecevents.Event{Kind: KindPeerIdentityChanged, ObjectRef: k.K, Value: k.V, Previous: prev,
			Detail: "the outstation's " + k.Label + " changed"})
	}
	c.rebuild()
}

// rebuild publishes the identity: this connection's, and the keys a previous
// connection read and this one did not (attributes not read yet), so their
// change is still seen later.
func (c *Client) rebuild() {
	c.mu.Lock()
	previous := c.identity
	id := bson.M{}
	for k, v := range c.session {
		id[k] = v
	}
	stored := bson.A{}
	for _, k := range jsmongo.GetDocArray(previous, "keys") {
		if kk := jsmongo.GetString(k, "k", ""); kk != "" {
			if _, now := c.keys[kk]; !now {
				stored = append(stored, bson.M{"k": kk, "v": jsmongo.GetString(k, "v", "")})
			}
		}
	}
	names := make([]string, 0, len(c.keys))
	for n := range c.keys {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		stored = append(stored, bson.M{"k": n, "v": c.keys[n].V})
	}
	id["keys"] = stored
	c.identity = id
	// The identity of the transport alone is published at once; the first
	// identification is reported once the attributes were asked for.
	first := !c.identified && id["method"] != nil
	if first {
		c.identified = true
	}
	c.mu.Unlock()
	if first {
		c.Emit(jssecevents.Event{Kind: KindPeerIdentified, Value: Summary(id),
			Detail: "first identification of the outstation"})
	}
}

// Summary is a one-line form of an identity.
func Summary(id bson.M) string {
	var parts []string
	for _, f := range []string{"vendor", "model", "softwareVersion", "address", "mac"} {
		if v, ok := id[f].(string); ok && v != "" {
			parts = append(parts, f+"="+v)
		}
	}
	if t, ok := id["tls"].(bson.M); ok {
		if v, ok := t["subject"].(string); ok && v != "" {
			parts = append(parts, "tls="+v)
		}
	}
	return strings.Join(parts, " ")
}

// --- commands ---------------------------------------------------------------

func cmdFields(doc bson.M, group, variation, index int) bson.M {
	f := bson.M{"group": group, "index": index}
	if variation > 0 {
		f["variation"] = variation
	}
	if v, ok := doc["value"]; ok {
		f["commandValue"] = v
	}
	return f
}

func cmdEvent(kind string, doc bson.M, group, variation, index int, detail string) jssecevents.Event {
	return jssecevents.Event{Kind: kind, Tag: jsmongo.GetString(doc, "tag", ""),
		ObjectRef: ObjectRef(group, variation, index), Fields: cmdFields(doc, group, variation, index), Detail: detail}
}

// Command counts a command sent for the flood detector and checks, in the
// background, a digital one against the state its point was in before it
// was sent (a later state may be its own result). rt is the realtimeData
// collection; nil skips that check.
func (c *Client) Command(rt *mongo.Collection, doc bson.M, value float64, digital bool, group, variation, index int) {
	if c == nil {
		return
	}
	if over, n := c.flood.Hit("cmd", time.Now()); over {
		e := cmdEvent(string(secevent.KindFloodCommands), doc, group, variation, index,
			fmt.Sprintf("more than %d commands to the outstation within %s", CommandFloodMax, CommandFloodWindow))
		e.Value = fmt.Sprint(n)
		c.Emit(e)
	}
	if rt == nil || !digital || (value != 0 && value != 1) {
		return
	}
	key, before := jsmongo.GetDouble(doc, "pointKey", 0), time.Now()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		state, ok, err := jssecevents.SupervisedState(ctx, rt, key, before)
		if err != nil || !ok || (state != 0) != (value != 0) {
			return
		}
		e := cmdEvent(KindCmdUnnecessary, doc, group, variation, index,
			"the supervised point already was in the commanded state")
		e.Value = fmt.Sprint(value)
		c.Emit(e)
	}()
}

// Stale reports a queued command older than its expiry: delayed, or
// replayed.
func (c *Client) Stale(doc bson.M, ts time.Time, group, variation, index int) {
	if c == nil {
		return
	}
	e := cmdEvent(KindCmdStaleTime, doc, group, variation, index, "command older than its expiry, not sent")
	e.Action, e.Value = "rejected", ts.UTC().Format(time.RFC3339)
	c.Emit(e)
}

// Refused reports a command the driver did not send (commands disabled, a
// value or group it cannot carry). kind is cmd.rejected or cmd.wrong_type.
func (c *Client) Refused(kind string, doc bson.M, why string, group, variation, index int) {
	if c == nil {
		return
	}
	e := cmdEvent(kind, doc, group, variation, index, "command not sent: "+why)
	e.Action = "rejected"
	c.Emit(e)
}

// Answer reports the outstation refusing a command (cmd.rejected): the first
// status that is not SUCCESS. NOT_SUPPORTED is not taken as proof that the
// point does not exist: outstations also answer it for controls they have
// disabled. It reports nothing for a command accepted.
func (c *Client) Answer(doc bson.M, statuses []dnp3.CommandStatus, group, variation, index int) {
	if c == nil {
		return
	}
	for _, st := range statuses {
		if st.OK() {
			continue
		}
		detail := "the outstation refused the command: " + st.String()
		if st == dnp3.CommandNotSupported {
			detail += " (no such output or operation there, or controls disabled)"
		}
		e := cmdEvent(KindCmdRejected, doc, group, variation, index, detail)
		e.Action, e.Value = "rejected", st.String()
		e.Fields["commandStatus"] = st.String()
		c.Emit(e)
		return
	}
}

// DropClientRefusals is the Drop of a client. Left out of go-dnp3's
// remote.rejected: a command status (the driver reports every command the
// outstation refuses, with the command's tag), and the refusal of the device
// attribute read, which the driver records as method "unsupported" and asks
// again on every connection. go-dnp3 names the refused task only in the
// detail ("read-attributes refused: ..."); the event's function is the
// response's.
func DropClientRefusals(e secevent.Event) bool {
	if e.Kind != secevent.KindRemoteRejected {
		return false
	}
	return e.Fields&secevent.HasCommandStatus != 0 || strings.HasPrefix(e.Detail, "read-attributes ")
}
