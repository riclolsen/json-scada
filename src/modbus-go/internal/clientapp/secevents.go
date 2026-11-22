/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package clientapp

// Security events (secEvents collection) and the identification of the
// device.
//
// go-modbus's client reports no security events, so everything here is the
// driver's: the device's identity (Read Device Identification, FC43/14, or
// Report Server ID, FC17, per polled unit), its MAC address and TLS
// certificate on every connection, and their changes; refused certificates
// and failed handshakes; reads the device refuses (an address it does not
// have, a function it does not serve); malformed responses, responses from
// another unit and unexpected transaction ids; commands refused by the
// driver or the device, commands to addresses the device does not have,
// expired (replayed) commands, command floods and commands to the state the
// point already is in.

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"modbus-go/internal/drv"
	"modbus-go/internal/mbcore"
	"modbus-go/internal/mbsec"

	"github.com/otfabric/go-modbus"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Detection limits.
const (
	// commandFloodMax commands within commandFloodWindow are a flood.
	commandFloodMax    = 10
	commandFloodWindow = 10 * time.Second
	// repeatEvery is how often a repeating condition (a noisy line, a link
	// that keeps refusing the certificate) is reported; the repeats in
	// between are counted.
	repeatEvery = 10 * time.Minute
	// maxIdentifiedUnits bounds the units identified per connection.
	maxIdentifiedUnits = 8
)

// clientSec is the security event state of one connection.
type clientSec struct {
	mu       sync.Mutex
	rep      *mbsec.Reporter
	rt       *mongo.Collection
	changes  jssecevents.Tracker // device identity, MAC, certificate
	flood    jssecevents.RateLimit
	repeat   jssecevents.Throttle
	once     map[string]bool // per connection: what was already reported
	identity bson.M          // stats.peerIdentity
	peerTLS  bson.M
	addr     string // the device, host:port or serial://port
	peerIP   netip.Addr
	session  int64
	// lastTimeout is when a request last timed out: a response with an
	// unexpected transaction id soon after is a late answer to it.
	lastTimeout time.Time
}

// noteTimeout records a request that timed out.
func (c *Connection) noteTimeout() {
	c.sec.mu.Lock()
	c.sec.lastTimeout = time.Now()
	c.sec.mu.Unlock()
}

// lateAnswerWindow is how long after a timeout a mismatched response is
// taken for a late answer.
func (c *Connection) lateAnswerWindow() time.Duration {
	return max(5*time.Second, 3*(1+time.Duration(c.Cfg.MaxRetries))*max(msDur(c.Cfg.TimeoutMs), time.Second))
}

var sessionSeq struct {
	sync.Mutex
	n int64
}

func nextSession() int64 {
	sessionSeq.Lock()
	defer sessionSeq.Unlock()
	sessionSeq.n++
	return sessionSeq.n
}

// SetSecurity sets, before Start, where the security events go (nil: only
// logged) and the realtimeData collection the command checks read.
func (c *Connection) SetSecurity(rep *mbsec.Reporter, rt *mongo.Collection) {
	c.sec.mu.Lock()
	c.sec.rep, c.sec.rt = rep, rt
	c.sec.mu.Unlock()
}

// seedIdentity starts the change detection from the identity stored in the
// connection's statistics, so a change across a driver restart is seen.
func (c *Connection) seedIdentity(stored bson.M) {
	if stored == nil {
		return
	}
	c.sec.mu.Lock()
	c.sec.identity = stored
	c.sec.mu.Unlock()
	for _, k := range jsmongo.GetDocArray(stored, "keys") {
		c.sec.changes.Seed(jsmongo.GetString(k, "k", ""), jsmongo.GetString(k, "v", ""))
	}
}

// PeerIdentity is stats.peerIdentity, nil when never identified or when
// peerIdentification is off.
func (c *Connection) PeerIdentity() bson.M {
	if !c.Cfg.PeerIdentification {
		return nil
	}
	c.sec.mu.Lock()
	defer c.sec.mu.Unlock()
	return c.sec.identity
}

// secEmit stores an event about this connection's device.
func (c *Connection) secEmit(e jssecevents.Event) {
	s := &c.sec
	s.mu.Lock()
	rep := s.rep
	if e.PeerAddress == "" && s.addr != "" {
		mbsec.Peer(&e, s.addr)
		if s.peerIP.IsValid() {
			e.PeerIP = s.peerIP.String()
		}
	}
	if e.Peer == nil && s.peerTLS != nil {
		e.Peer = bson.M{"tls": s.peerTLS}
	}
	if e.SessionID == 0 {
		e.SessionID = s.session
	}
	s.mu.Unlock()
	e.Role = "client"
	rep.Emit(e)
}

// secOnce reports e only the first time key is seen in this connection.
func (c *Connection) secOnce(key string, e jssecevents.Event) {
	s := &c.sec
	s.mu.Lock()
	if s.once == nil {
		s.once = map[string]bool{}
	}
	if s.once[key] || len(s.once) > 10000 {
		s.mu.Unlock()
		return
	}
	s.once[key] = true
	s.mu.Unlock()
	c.secEmit(e)
}

// secRepeat reports e at most once per repeatEvery for key, across
// connections, with the count of the occurrences held back.
func (c *Connection) secRepeat(key string, e jssecevents.Event) {
	ok, n := c.sec.repeat.Allow(key, time.Now())
	if !ok {
		return
	}
	e.Count = n
	c.secEmit(e)
}

// secNewLink starts the state of a connection attempt to addr: "host:port"
// (the host resolved to its IP for the MAC lookup) or "serial://port".
func (c *Connection) secNewLink(addr string) {
	ip := jssecevents.SplitAddr(addr)
	if !ip.IsValid() && !strings.HasPrefix(addr, "serial://") {
		if host, _, err := net.SplitHostPort(addr); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host); err == nil && len(ips) > 0 {
				ip = ips[0].Unmap()
			}
			cancel()
		}
	}
	s := &c.sec
	s.mu.Lock()
	s.once = map[string]bool{}
	s.session = nextSession()
	s.addr, s.peerIP, s.peerTLS = addr, ip, nil
	s.mu.Unlock()
}

// --- TLS --------------------------------------------------------------------

// certCapture returns cfg, or a copy of it whose verification callback also
// keeps the chain the device presented (raw), so a refused certificate can
// be described.
func certCapture(cfg *tls.Config) (*tls.Config, *[][]byte) {
	raw := new([][]byte)
	if cfg.VerifyPeerCertificate == nil {
		return cfg, raw
	}
	cc := cfg.Clone()
	verify := cfg.VerifyPeerCertificate
	cc.VerifyPeerCertificate = func(rc [][]byte, chains [][]*x509.Certificate) error {
		*raw = rc
		return verify(rc, chains)
	}
	return cc, raw
}

// secHandshake records the device's TLS session, or reports a failed
// handshake: a refused certificate, with its details when known, or another
// failure. A link that keeps failing is reported once per repeatEvery.
func (c *Connection) secHandshake(cs *tls.ConnectionState, raw [][]byte, err error) {
	if err == nil {
		t := jssecevents.TLSStateDoc(cs)
		c.sec.mu.Lock()
		c.sec.peerTLS = t
		c.sec.mu.Unlock()
		return
	}
	e := jssecevents.Event{Kind: mbsec.KindTLSHandshakeFailed, Protocol: "tls", Action: "disconnected",
		Error: err.Error(), Detail: "TLS handshake with the device failed"}
	if jssecevents.IsCertError(err) || len(raw) > 0 {
		e.Kind, e.Detail = mbsec.KindTLSCertRejected, "device certificate refused"
		cert := jssecevents.RawCertDoc(raw)
		var ve *tls.CertificateVerificationError
		if cert == nil && errors.As(err, &ve) && len(ve.UnverifiedCertificates) > 0 {
			cert = jssecevents.CertDoc(ve.UnverifiedCertificates[0])
		}
		if cert != nil {
			e.Peer = bson.M{"tls": cert}
		}
	}
	c.secRepeat(e.Kind+"|"+e.Error, e)
}

// --- identification ---------------------------------------------------------

// identityKey is one compared part of the identity.
type identityKey struct{ k, v, label string }

// identifyPeer reads the device identification of the polled units,
// publishes the identity in the statistics and reports what changed since
// the previous connection. It runs once per connection, when the device
// first answers.
func (c *Connection) identifyPeer(ctx context.Context, cl *modbus.Client) {
	c.sec.mu.Lock()
	addr, ip, peerTLS := c.sec.addr, c.sec.peerIP, c.sec.peerTLS
	c.sec.mu.Unlock()
	id := bson.M{"collectedAt": time.Now(), "address": addr}
	var keys []identityKey
	add := func(k, v, label string) {
		if v != "" {
			keys = append(keys, identityKey{k, v, label})
		}
	}

	units := bson.A{}
	for _, u := range c.pollUnits() {
		ud, key, timedOut := c.identifyUnit(ctx, cl, u)
		units = append(units, ud)
		add(fmt.Sprintf("unit:%d", u), key, fmt.Sprintf("unit %d device identification", u))
		if timedOut || ctx.Err() != nil {
			break // a device silent to identification: do not wait for every unit
		}
	}
	if len(units) > 0 {
		id["units"] = units
	}
	if ip.IsValid() {
		id["ip"] = ip.String()
		if mac := jssecevents.LookupMAC(ip); mac != "" {
			id["mac"] = mac
			add("mac", mac, "MAC address")
		}
	}
	if peerTLS != nil {
		id["tls"] = peerTLS
		fp, _ := peerTLS["sha256"].(string)
		add("cert", fp, "certificate")
	}

	c.sec.mu.Lock()
	first, previous := c.sec.identity == nil, c.sec.identity
	c.sec.mu.Unlock()
	stored := bson.A{}
	now := map[string]bool{}
	for _, k := range keys {
		now[k.k] = true
	}
	// Keep what this connection could not read (a unit that did not answer),
	// so a later change of it is still seen across restarts.
	for _, k := range jsmongo.GetDocArray(previous, "keys") {
		if kk := jsmongo.GetString(k, "k", ""); kk != "" && !now[kk] {
			stored = append(stored, bson.M{"k": kk, "v": jsmongo.GetString(k, "v", "")})
		}
	}
	for _, k := range keys {
		stored = append(stored, bson.M{"k": k.k, "v": k.v})
		prev, changed := c.sec.changes.Observe(k.k, k.v)
		if !changed {
			continue
		}
		kind := mbsec.KindPeerIdentityChanged
		switch k.k {
		case "mac":
			kind = mbsec.KindPeerMACChanged
		case "cert":
			kind = mbsec.KindTLSCertChanged
		}
		c.secEmit(jssecevents.Event{Kind: kind, ObjectRef: k.k, Value: k.v, Previous: prev,
			Detail: k.label + " changed"})
	}
	id["keys"] = stored
	if first {
		c.secEmit(jssecevents.Event{Kind: mbsec.KindPeerIdentified, Value: identitySummary(id),
			Detail: "first identification of the device"})
	}
	c.sec.mu.Lock()
	c.sec.identity = id
	c.sec.mu.Unlock()
	jslog.Log(jslog.LevelBasic, "%s: peer identity: %s", c.Cfg.Name, identitySummary(id))
}

// pollUnits are the unit ids the connection polls, at most
// maxIdentifiedUnits of them (broadcast excluded).
func (c *Connection) pollUnits() []int {
	c.mu.Lock()
	seen := map[int]bool{}
	for _, b := range c.blocks {
		if b.Unit > 0 && b.Unit <= 255 {
			seen[b.Unit] = true
		}
	}
	c.mu.Unlock()
	units := make([]int, 0, len(seen))
	for u := range seen {
		units = append(units, u)
	}
	sort.Ints(units)
	if len(units) > maxIdentifiedUnits {
		units = units[:maxIdentifiedUnits]
	}
	return units
}

// identifyUnit reads the identification of one unit: Read Device
// Identification (FC43/14, regular category), or Report Server ID (FC17)
// when the device refuses it. key is what is compared for changes.
func (c *Connection) identifyUnit(ctx context.Context, cl *modbus.Client, unit int) (doc bson.M, key string, timedOut bool) {
	doc = bson.M{"unitId": unit}
	u8 := uint8(unit)
	var di *modbus.DeviceIdentification
	err := c.identRequest(ctx, func(rctx context.Context) error {
		var e error
		di, e = cl.ReadDeviceIdentification(rctx, u8, modbus.DeviceIDRegular, 0)
		return e
	})
	if err == nil && di != nil {
		objs := bson.M{}
		for _, o := range di.Objects {
			name := o.Name
			if name == "" || name == "Reserved" || name == "Extended" {
				name = fmt.Sprintf("object0x%02X", uint8(o.ID))
			}
			objs[name] = printable(o.Value)
		}
		doc["method"] = "FC43/14"
		doc["conformityLevel"] = int(di.ConformityLevel)
		doc["objects"] = objs
		var parts []string
		for _, n := range []string{"VendorName", "ProductCode", "MajorMinorRevision", "ProductName", "ModelName"} {
			if v, _ := objs[n].(string); v != "" {
				parts = append(parts, v)
			}
		}
		key = strings.Join(parts, " | ")
		return doc, key, false
	}
	if isTimeout(err) {
		doc["error"] = ErrText(err)
		return doc, "", true
	}
	doc["fc43Error"] = ErrText(err)
	var rs *modbus.ReportServerIDResponse
	err = c.identRequest(ctx, func(rctx context.Context) error {
		var e error
		rs, e = cl.ReportServerID(rctx, u8)
		return e
	})
	if err != nil || rs == nil {
		doc["error"] = ErrText(err)
		return doc, "", isTimeout(err)
	}
	doc["method"] = "FC17"
	doc["serverId"] = printable(string(rs.Data))
	doc["serverIdHex"] = fmt.Sprintf("%X", rs.Data)
	if rs.RunIndicatorStatus != nil {
		doc["running"] = *rs.RunIndicatorStatus
	}
	return doc, serverIDKey(rs.Data), false
}

// serverIDKey is the part of an FC17 answer compared between connections:
// the data without the run indicator status, which flips between 0x00 and
// 0xFF as the device stops and runs. The standard puts it after the
// device-specific server id, which devices make one byte (the indicator is
// then the second) or the whole answer (the indicator is then the last), so
// both positions are masked when they hold an indicator value.
func serverIDKey(data []byte) string {
	k := []byte(fmt.Sprintf("%X", data))
	for _, i := range []int{1, len(data) - 1} {
		if i >= 1 && i < len(data) && (data[i] == 0x00 || data[i] == 0xFF) {
			k[2*i], k[2*i+1] = '?', '?'
		}
	}
	return string(k)
}

// identRequest runs one identification request through the gate, with the
// request timeout and no retry; it does not count in the statistics.
func (c *Connection) identRequest(ctx context.Context, fn func(ctx context.Context) error) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return ctx.Err()
	}
	c.gate.acquire(false)
	defer func() {
		sleepCtx(ctx, msDur(c.Cfg.InterRequestDelayMs))
		c.gate.release()
	}()
	t := msDur(c.Cfg.TimeoutMs)
	if t <= 0 {
		t = time.Second
	}
	rctx, cancel := context.WithTimeout(ctx, 2*t)
	defer cancel()
	err := fn(rctx)
	if isTimeout(err) {
		c.noteTimeout()
	}
	return err
}

func isTimeout(err error) bool {
	return errors.Is(err, modbus.ErrRequestTimedOut) || errors.Is(err, context.DeadlineExceeded)
}

// printable keeps the printable characters of a device string.
func printable(s string) string {
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, s))
}

func identitySummary(id bson.M) string {
	var parts []string
	if units, ok := id["units"].(bson.A); ok {
		for _, u := range units {
			ud, _ := u.(bson.M)
			if objs, ok := ud["objects"].(bson.M); ok {
				var p []string
				for _, n := range []string{"VendorName", "ProductCode", "MajorMinorRevision"} {
					if v, _ := objs[n].(string); v != "" {
						p = append(p, v)
					}
				}
				parts = append(parts, fmt.Sprintf("unit%v=%s", ud["unitId"], strings.Join(p, "/")))
			} else if sid, ok := ud["serverId"].(string); ok {
				parts = append(parts, fmt.Sprintf("unit%v=%s", ud["unitId"], sid))
			}
		}
	}
	if v, ok := id["mac"].(string); ok {
		parts = append(parts, "mac="+v)
	}
	if t, ok := id["tls"].(bson.M); ok {
		if v, ok := t["subject"].(string); ok && v != "" {
			parts = append(parts, "tls="+v)
		}
	}
	if len(parts) == 0 {
		return "(no identification)"
	}
	return strings.Join(parts, " ")
}

// --- reads and responses ----------------------------------------------------

// secReadError reports a read the device refused for a lasting reason (an
// address it does not have, a function it does not serve) once per
// connection, and a response that violates the protocol.
func (c *Connection) secReadError(b *Block, err error) {
	ref := fmt.Sprintf("unit %d %s:%d x%d", b.Unit, b.Area, b.Start, b.Count)
	if code, ok := mbsec.Exception(err); ok {
		kind := ""
		switch code {
		case mbcore.ExIllegalDataAddress:
			kind = mbsec.KindAccessObjectNonexistent
		case mbcore.ExIllegalFunction:
			kind = mbsec.KindAccessUnrecognizedService
		default:
			return
		}
		tags := make([]string, 0, 3)
		for i, s := range b.Slices {
			if i == 3 {
				tags = append(tags, "...")
				break
			}
			tags = append(tags, s.B.Tag)
		}
		c.secOnce("read|"+ref, jssecevents.Event{Kind: kind, Action: "rejected", ObjectRef: ref,
			Tag: strings.Join(tags, ","), Fields: mbsec.Fields(b.Unit, b.FC, b.Start, b.Count, code), Error: ErrText(err),
			Detail: "the device refused a configured read"})
		return
	}
	c.secResponseError(b.Unit, b.FC, err)
}

// secResponseError reports a response that violates the protocol.
func (c *Connection) secResponseError(unit, fc int, err error) {
	kind := ""
	switch {
	case errors.Is(err, modbus.ErrBadUnitID):
		kind = mbsec.KindProtoUnitMismatch
	case errors.Is(err, modbus.ErrBadTransactionID):
		kind = mbsec.KindProtoTxnMismatch
	case errors.Is(err, modbus.ErrBadCRC), errors.Is(err, modbus.ErrBadLRC), errors.Is(err, modbus.ErrShortFrame):
		kind = mbsec.KindProtoBadFrame
	case errors.Is(err, modbus.ErrUnknownProtocolID), errors.Is(err, modbus.ErrInvalidMBAPLength),
		errors.Is(err, modbus.ErrProtocolError):
		kind = mbsec.KindProtoBadResponse
	default:
		return
	}
	c.secRepeat(kind+"|"+err.Error(), jssecevents.Event{Kind: kind, Action: "blocked", Error: err.Error(),
		Fields: mbsec.Fields(unit, fc, -1, -1, 0), Detail: "invalid response from the device"})
}

// libLogger adds go-modbus's warnings about a response with another
// transaction id or protocol id than expected (a late answer, or an injected
// or replayed one) to the security events.
type libLogger struct {
	drv.LibLogger
	c *Connection
}

func (l *libLogger) Warnf(f string, a ...any) {
	l.LibLogger.Warnf(f, a...)
	msg := fmt.Sprintf(f, a...)
	kind := ""
	switch {
	case strings.Contains(msg, "unexpected transaction id"):
		l.c.sec.mu.Lock()
		late := time.Since(l.c.sec.lastTimeout) < l.c.lateAnswerWindow()
		l.c.sec.mu.Unlock()
		if late {
			return // a late answer to a request that timed out
		}
		kind = mbsec.KindProtoTxnMismatch
	case strings.Contains(msg, "unexpected protocol id"), strings.Contains(msg, "invalid MBAP length"):
		kind = mbsec.KindProtoBadResponse
	default:
		return
	}
	l.c.secRepeat("lib|"+kind, jssecevents.Event{Kind: kind, Source: mbsec.SourceLibrary,
		Action: "blocked", Detail: msg})
}

// --- commands ---------------------------------------------------------------

// commandFields are the Modbus fields of a command document.
func commandFields(doc bson.M) bson.M {
	f := bson.M{}
	for _, k := range []string{"protocolSourceCommonAddress", "protocolSourceObjectAddress", "protocolSourceASDU"} {
		if v := fieldStr(doc, k); v != "" {
			f[k] = v
		}
	}
	return f
}

// secCommandRefused reports a command refused by the driver or the device:
// an address the device does not have is cmd.object_unknown, any other
// refusal cmd.rejected; a timeout or a link failure is not reported.
func (c *Connection) secCommandRefused(doc bson.M, kind, desc string, err error) {
	e := jssecevents.Event{Kind: kind, Action: "rejected", Tag: fieldStr(doc, "tag"),
		ObjectRef: fieldStr(doc, "protocolSourceObjectAddress"), Fields: commandFields(doc), Detail: desc}
	if err != nil {
		code, ok := mbsec.Exception(err)
		if !ok {
			c.secResponseError(-1, -1, err)
			return
		}
		e.Fields["exception"], e.Fields["exceptionName"] = code, mbcore.ExceptionName(code)
		e.Kind, e.Detail = mbsec.KindCmdRejected, "the device refused the command: "+ErrText(err)
		if code == mbcore.ExIllegalDataAddress {
			e.Kind = mbsec.KindCmdObjectUnknown
		}
	}
	c.secEmit(e)
}

// secCommandStale reports a command older than its expiry: delayed in the
// queue, or replayed.
func (c *Connection) secCommandStale(doc bson.M, ts time.Time) {
	c.secEmit(jssecevents.Event{Kind: mbsec.KindCmdStaleTime, Action: "rejected", Tag: fieldStr(doc, "tag"),
		ObjectRef: fieldStr(doc, "protocolSourceObjectAddress"), Fields: commandFields(doc),
		Value: ts.UTC().Format(time.RFC3339), Detail: "command older than its expiry, not executed"})
}

// secCommand counts a command for the flood detector and checks, in the
// background, a digital one against the state its point already is in.
func (c *Connection) secCommand(ctx context.Context, doc bson.M, value float64, digital bool) {
	c.sec.mu.Lock()
	if c.sec.flood.Max == 0 {
		c.sec.flood = jssecevents.RateLimit{Max: commandFloodMax, Window: commandFloodWindow}
	}
	rt := c.sec.rt
	c.sec.mu.Unlock()
	if over, n := c.sec.flood.Hit("cmd", time.Now()); over {
		c.secEmit(jssecevents.Event{Kind: mbsec.KindFloodControls, Tag: fieldStr(doc, "tag"), Value: fmt.Sprint(n),
			Detail: fmt.Sprintf("more than %d commands to the device within %s", commandFloodMax, commandFloodWindow)})
	}
	if rt == nil || !digital || (value != 0 && value != 1) {
		return
	}
	key, before := jsmongo.GetDouble(doc, "pointKey", 0), time.Now()
	go func() {
		state, ok, err := jssecevents.SupervisedState(context.WithoutCancel(ctx), rt, key, before)
		if err != nil || !ok {
			return
		}
		if (state != 0) == (value != 0) {
			c.secEmit(jssecevents.Event{Kind: mbsec.KindCmdUnnecessary, Tag: fieldStr(doc, "tag"),
				ObjectRef: fieldStr(doc, "protocolSourceObjectAddress"), Value: fmt.Sprint(value),
				Fields: commandFields(doc), Detail: "the supervised point already was in the commanded state"})
		}
	}()
}
