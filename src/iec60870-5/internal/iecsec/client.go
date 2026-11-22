/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package iecsec

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/go-iecp5/secevent"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Command flood limit of a client connection: more than CommandFloodMax
// commands to the device within CommandFloodWindow.
const (
	CommandFloodMax    = 10
	CommandFloodWindow = 10 * time.Second
)

// Key is one compared part of a peer's identity: its MAC address, its
// certificate, a device identification.
type Key struct {
	K, V  string
	Label string
}

type sentCmd struct {
	tag, ref string
}

// Client is the security state of one client (master) connection: the
// identity of the device in stats.peerIdentity and its changes, and the
// command events (refused, unknown object, stale, flood, unnecessary).
type Client struct {
	Rep *Reporter
	// LibPeer is set when go-iecp5 reports the MAC address and certificate
	// changes itself (104): the driver then only stores them.
	LibPeer bool

	mu       sync.Mutex
	changes  jssecevents.Tracker
	flood    jssecevents.RateLimit
	identity bson.M // stats.peerIdentity
	session  bson.M // what this connection learned of the peer
	keys     map[string]Key
	devices  map[string]bson.M
	peer     secevent.PeerInfo
	sent     map[[2]int]sentCmd
	gen      uint64 // the current connection
}

// NewClient starts the change detection from the identity stored in the
// connection's statistics, so a change across a driver restart is seen.
func NewClient(rep *Reporter, stored bson.M, libPeer bool) *Client {
	c := &Client{Rep: rep, LibPeer: libPeer,
		flood: jssecevents.RateLimit{Max: CommandFloodMax, Window: CommandFloodWindow}}
	if stored == nil || !c.ident() {
		return c
	}
	c.identity = stored
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
	if libPeer {
		SeedLibCache(jsmongo.GetString(stored, "ip", ""), mac, cert)
	}
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

// SetPeer sets the peer the events of the current connection name.
func (c *Client) SetPeer(p secevent.PeerInfo) {
	c.mu.Lock()
	c.peer = p
	c.mu.Unlock()
}

// Emit stores an event about this connection's device.
func (c *Client) Emit(e jssecevents.Event) {
	if c == nil {
		return
	}
	c.mu.Lock()
	p := c.peer
	c.mu.Unlock()
	if e.PeerAddress == "" {
		SetPeer(&e, p)
	}
	c.Rep.Emit(e)
}

// Begin records what a new connection learned of the peer (base, stored as
// stats.peerIdentity) and reports what changed since the previous one.
func (c *Client) Begin(base bson.M, keys []Key) { c.begin(base, keys) }

// begin starts the identity of a new connection and returns its generation,
// with which a late lookup tells whether its connection is still current.
func (c *Client) begin(base bson.M, keys []Key) uint64 {
	if c == nil || !c.ident() {
		return 0
	}
	c.mu.Lock()
	c.gen++
	gen := c.gen
	c.session, c.keys, c.devices = base, map[string]Key{}, map[string]bson.M{}
	c.mu.Unlock()
	c.observe(keys)
	return gen
}

// addToSession adds what a lookup learned to the identity of connection gen,
// unless a newer connection has begun meanwhile.
func (c *Client) addToSession(gen uint64, field string, v any, k Key) {
	c.mu.Lock()
	if c.gen != gen || c.session == nil {
		c.mu.Unlock()
		return
	}
	c.session[field] = v
	c.mu.Unlock()
	c.observe([]Key{k})
}

// AddDevice adds what a device of the connection says about itself (IEC 103
// identification) to the identity and reports what changed.
func (c *Client) AddDevice(name string, doc bson.M, keys []Key) {
	if c == nil || !c.ident() {
		return
	}
	c.mu.Lock()
	if c.devices == nil {
		c.devices = map[string]bson.M{}
	}
	c.devices[name] = doc
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
		if !changed {
			continue
		}
		kind := KindPeerIdentityChanged
		switch k.K {
		case "mac":
			kind = string(secevent.KindPeerMACChanged)
		case "cert":
			kind = string(secevent.KindTLSCertChanged)
		}
		if c.LibPeer && (k.K == "mac" || k.K == "cert") {
			continue // go-iecp5 reported it
		}
		c.Emit(jssecevents.Event{Kind: kind, ObjectRef: k.K, Value: k.V, Previous: prev,
			Detail: k.Label + " changed"})
	}
	c.rebuild()
}

// rebuild publishes the identity: this connection's, the devices', and the
// keys a previous connection read and this one did not (a device that has
// not sent its identification yet), so their change is still seen later.
func (c *Client) rebuild() {
	c.mu.Lock()
	first, previous := c.identity == nil, c.identity
	id := bson.M{}
	for k, v := range c.session {
		id[k] = v
	}
	if len(c.devices) > 0 {
		names := make([]string, 0, len(c.devices))
		for n := range c.devices {
			names = append(names, n)
		}
		sort.Strings(names)
		devs := bson.A{}
		for _, n := range names {
			devs = append(devs, c.devices[n])
		}
		id["devices"] = devs
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
	c.mu.Unlock()
	if first {
		c.Emit(jssecevents.Event{Kind: KindPeerIdentified, Value: Summary(id),
			Detail: "first identification of the device"})
	}
}

// Summary is a one-line form of an identity.
func Summary(id bson.M) string {
	var parts []string
	for _, f := range []string{"address", "mac"} {
		if v, ok := id[f].(string); ok && v != "" {
			parts = append(parts, f+"="+v)
		}
	}
	if t, ok := id["tls"].(bson.M); ok {
		if v, ok := t["subject"].(string); ok && v != "" {
			parts = append(parts, "tls="+v)
		}
	}
	for _, d := range jsmongo.GetDocArray(id, "devices") {
		if v := jsmongo.GetString(d, "identification", ""); v != "" {
			parts = append(parts, "device="+v)
		}
	}
	return strings.Join(parts, " ")
}

// --- commands ---------------------------------------------------------------

func cmdFields(doc bson.M, typeID, ca, ioa int) bson.M {
	f := Fields(typeID, -1, ca, ioa)
	if v, ok := doc["value"]; ok {
		f["commandValue"] = v
	}
	return f
}

// Command counts a command for the flood detector, remembers its tag for the
// device's answer and checks, in the background, a digital one against the
// state its point was in before it was sent (a later state may be its own
// result). rt is the realtimeData collection; nil skips that check.
func (c *Client) Command(rt *mongo.Collection, doc bson.M, value float64, digital bool, typeID, ca, ioa int) {
	if c == nil {
		return
	}
	tag := jsmongo.GetString(doc, "tag", "")
	ref := ObjectRef(ca, ioa)
	c.mu.Lock()
	if c.sent == nil || len(c.sent) > 4096 {
		c.sent = map[[2]int]sentCmd{}
	}
	c.sent[[2]int{ca, ioa}] = sentCmd{tag: tag, ref: ref}
	c.mu.Unlock()
	if over, n := c.flood.Hit("cmd", time.Now()); over {
		c.Emit(jssecevents.Event{Kind: string(secevent.KindFloodCommands), Tag: tag, ObjectRef: ref, Value: fmt.Sprint(n),
			Detail: fmt.Sprintf("more than %d commands to the device within %s", CommandFloodMax, CommandFloodWindow)})
	}
	if rt == nil || !digital || (value != 0 && value != 1) {
		return
	}
	key, before := jsmongo.GetDouble(doc, "pointKey", 0), time.Now()
	fields := cmdFields(doc, typeID, ca, ioa)
	go func() {
		state, ok, err := jssecevents.SupervisedState(context.Background(), rt, key, before)
		if err != nil || !ok || (state != 0) != (value != 0) {
			return
		}
		c.Emit(jssecevents.Event{Kind: KindCmdUnnecessary, Tag: tag, ObjectRef: ref, Value: fmt.Sprint(value),
			Fields: fields, Detail: "the supervised point already was in the commanded state"})
	}()
}

// Stale reports a queued command older than its expiry: delayed, or replayed.
func (c *Client) Stale(doc bson.M, ts time.Time, typeID, ca, ioa int) {
	if c == nil {
		return
	}
	c.Emit(jssecevents.Event{Kind: KindCmdStaleTime, Action: "rejected", Tag: jsmongo.GetString(doc, "tag", ""),
		ObjectRef: ObjectRef(ca, ioa), Fields: cmdFields(doc, typeID, ca, ioa),
		Value: ts.UTC().Format(time.RFC3339), Detail: "command older than its expiry, not sent"})
}

// Refused reports a command the driver did not send (commands disabled, a
// type it does not implement).
func (c *Client) Refused(doc bson.M, why string, typeID, ca, ioa int) {
	if c == nil {
		return
	}
	c.Emit(jssecevents.Event{Kind: string(secevent.KindCmdRejected), Action: "rejected",
		Tag: jsmongo.GetString(doc, "tag", ""), ObjectRef: ObjectRef(ca, ioa),
		Fields: cmdFields(doc, typeID, ca, ioa), Detail: "command not sent: " + why})
}

func (c *Client) sentTag(ca, ioa int) (string, string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.sent[[2]int{ca, ioa}]
	if s.ref == "" {
		s.ref = ObjectRef(ca, ioa)
	}
	return s.tag, s.ref
}

// Answer reports the device refusing a command or parameter (a negative
// confirmation, or an unknown type, cause, common address or object).
func (c *Client) Answer(pack *asdu.ASDU) {
	if c == nil || !(asdu.IsProcessCommand(pack.Type) || asdu.IsParameter(pack.Type)) {
		return
	}
	ioa := -1
	if a, ok := asdu.FirstIOA(pack); ok {
		ioa = int(a)
	}
	e := jssecevents.Event{Kind: string(secevent.KindCmdRejected), Action: "rejected", Fields: PackFields(pack)}
	switch cause := pack.Coa.Cause; {
	case cause == asdu.UnknownIOA:
		e.Kind, e.Detail = KindCmdObjectUnknown, "the device does not have the commanded object (unknown IOA)"
	case cause == asdu.UnknownCA:
		e.Kind, e.Detail = KindCmdObjectUnknown, "the device does not have the commanded common address (unknown CA)"
	case cause == asdu.UnknownTypeID:
		e.Detail = "the device does not serve the command type (unknown type)"
	case cause == asdu.UnknownCOT:
		e.Detail = "the device refused the cause of transmission (unknown cause)"
	case (cause == asdu.ActivationCon || cause == asdu.DeactivationCon) && pack.Coa.IsNegative:
		e.Detail = "the device refused the command (negative confirmation)"
	default:
		return
	}
	e.Tag, e.ObjectRef = c.sentTag(int(pack.CommonAddr), ioa)
	if asdu.IsProcessCommand(pack.Type) {
		e.Value = asdu.DescribeCommand(pack)
	}
	c.Emit(e)
}

// Refused103 reports an IEC 103 general command the device refused
// (cause 21, negative acknowledgement).
func (c *Client) Refused103(ca, objAddr int) {
	if c == nil {
		return
	}
	tag, ref := c.sentTag(ca, objAddr)
	c.Emit(jssecevents.Event{Kind: string(secevent.KindCmdRejected), Action: "rejected", Tag: tag, ObjectRef: ref,
		Fields: bson.M{"commonAddress": ca, "objectAddress": objAddr, "cause": 21},
		Detail: "the device refused the general command (negative acknowledgement)"})
}

// Restarted reports the device telling it (re)initialised: IEC 101 end of
// initialisation, IEC 103 start/restart or power on.
func (c *Client) Restarted(ca int, value, detail string, fields bson.M) {
	if c == nil {
		return
	}
	c.Emit(jssecevents.Event{Kind: string(secevent.KindCtrlEndOfInit), ObjectRef: ObjectRef(ca, -1), Value: value,
		Fields: fields, Detail: detail})
}

// CertRefused reports a TLS handshake that failed on the device's
// certificate (101/103 TCP transport; go-iecp5 reports the 104 ones).
func (c *Client) CertRefused(addr string, err error) bool {
	e, ok := CertRejected(err)
	if !ok || c == nil {
		return ok
	}
	e.PeerAddress = addr
	if ip := jssecevents.SplitAddr(addr); ip.IsValid() {
		e.PeerIP = ip.String()
	}
	c.Rep.Emit(e)
	return true
}

// PeerBase is the identity of a device go-iecp5 describes (104): its
// address, MAC address (from the admission's neighbour table lookup) and TLS
// session, with the keys compared between connections.
func PeerBase(addr string, p secevent.PeerInfo) (bson.M, []Key) {
	id := bson.M{"address": addr, "connectedAt": time.Now()}
	var keys []Key
	if p.RemoteAddr.IsValid() {
		id["ip"] = p.RemoteAddr.Addr().Unmap().String()
	}
	if p.MAC != nil && p.MACSource != secevent.MACGateway {
		id["mac"] = p.MAC.String()
		keys = append(keys, Key{K: "mac", V: p.MAC.String(), Label: "MAC address"})
	}
	if p.TLS != nil {
		id["tls"] = TLSDoc(p.TLS)
		keys = append(keys, Key{K: "cert", V: p.TLS.FingerprintSHA256, Label: "certificate"})
	}
	return id, keys
}

// ConnBase is the identity of a device behind a TCP transport (101/103 over
// a terminal server): its address, the MAC address of its IP from the
// operating system's neighbour table and its TLS certificate. It may run a
// command (the neighbour table lookup): call it off the protocol goroutines.
// Without a TCP connection (serial) it names the port only.
func ConnBase(portName string, linkAddress int, conn net.Conn) (bson.M, []Key) {
	id, keys, ip := connBase(portName, linkAddress, conn)
	if ip.IsValid() {
		if mac := jssecevents.LookupMAC(ip); mac != "" {
			id["mac"] = mac
			keys = append(keys, Key{K: "mac", V: mac, Label: "MAC address"})
		}
	}
	return id, keys
}

// connBase is ConnBase without the neighbour table lookup; it returns the
// peer's IP for it.
func connBase(portName string, linkAddress int, conn net.Conn) (bson.M, []Key, netip.Addr) {
	id := bson.M{"address": portName, "linkAddress": linkAddress, "connectedAt": time.Now()}
	if conn == nil {
		id["address"] = "serial://" + portName
		return id, nil, netip.Addr{}
	}
	var keys []Key
	ip := jssecevents.SplitAddr(conn.RemoteAddr().String())
	if ip.IsValid() {
		id["address"], id["ip"] = conn.RemoteAddr().String(), ip.String()
	}
	if tc, ok := conn.(*tls.Conn); ok {
		cs := tc.ConnectionState()
		if t := jssecevents.TLSStateDoc(&cs); t != nil {
			id["tls"] = t
			if fp, _ := t["sha256"].(string); fp != "" {
				keys = append(keys, Key{K: "cert", V: fp, Label: "certificate"})
			}
		}
	}
	return id, keys, ip
}

// ConnOf is the network connection behind a go-iecp5 101/103 client's port,
// nil for a serial port.
func ConnOf(port io.ReadWriteCloser) net.Conn {
	if c, ok := port.(net.Conn); ok {
		return c
	}
	return nil
}

// PeerFromConn describes the peer behind a 101/103 transport: the TCP peer,
// or the serial port.
func PeerFromConn(portName string, conn net.Conn) secevent.PeerInfo {
	if conn == nil {
		return secevent.PeerInfo{Network: "serial", SerialPort: portName}
	}
	p := secevent.PeerInfo{Network: "tcp"}
	if a, err := netip.ParseAddrPort(conn.RemoteAddr().String()); err == nil {
		p.RemoteAddr = netip.AddrPortFrom(a.Addr().Unmap(), a.Port())
	}
	if a, err := netip.ParseAddrPort(conn.LocalAddr().String()); err == nil {
		p.LocalAddr = a
	}
	return p
}

// Connected identifies the device of a 101/103 connection that just came up:
// the connection's identity is set at once (a device's identification may
// follow immediately), the neighbour table lookup of its MAC address runs in
// the background and is added if the connection is still the current one.
func (c *Client) Connected(portName string, linkAddress int, conn net.Conn) {
	if c == nil {
		return
	}
	c.SetPeer(PeerFromConn(portName, conn)) // events name the peer even without identification
	if !c.ident() {
		return
	}
	base, keys, ip := connBase(portName, linkAddress, conn)
	gen := c.begin(base, keys)
	if !ip.IsValid() {
		return
	}
	go func() {
		if mac := jssecevents.LookupMAC(ip); mac != "" {
			c.addToSession(gen, "mac", mac, Key{K: "mac", V: mac, Label: "MAC address"})
		}
	}()
}
