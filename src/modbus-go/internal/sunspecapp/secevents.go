/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package sunspecapp

// Security events (secEvents collection) and the identification of the
// device, as in the MODBUS client: go-sunspec's secevent package (the
// go-modbus security event API) only reports on servers, so everything here
// is detected by the driver and stored through mbsec.Reporter (protocol
// "sunspec").
//
// Detected: the device identity (Common model 1: Mn, Md, Opt, Vr, SN, DA,
// and the model chain, per unit), its MAC address and TLS certificate on
// every discovery, and their changes; refused certificates and failed
// handshakes; units without a SunSpec map; invalid model chains; reads the
// device refuses and configured points it does not expose; malformed
// responses and unexpected transaction ids; commands refused by the driver
// or the device, commands to points that do not exist, expired (replayed)
// commands, command floods and commands to the value the point already has.

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"
	"unicode"

	"modbus-go/internal/clientapp"
	"modbus-go/internal/drv"
	"modbus-go/internal/mbcore"
	"modbus-go/internal/mbsec"

	"github.com/otfabric/go-modbus"
	"github.com/otfabric/go-sunspec"
	"github.com/otfabric/go-sunspec/secevent"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Protocol is the secEvents protocol field of this driver's events.
const Protocol = "sunspec"

// Detection limits.
const (
	commandFloodMax    = 10
	commandFloodWindow = 10 * time.Second
	repeatEvery        = 10 * time.Minute
)

// clientSec is the security event state of one connection.
type clientSec struct {
	mu          sync.Mutex
	rep         *mbsec.Reporter
	rt          *mongo.Collection
	changes     jssecevents.Tracker
	flood       jssecevents.RateLimit
	repeat      jssecevents.Throttle
	once        map[string]bool
	identity    bson.M
	peerTLS     bson.M
	addr        string
	peerIP      netip.Addr
	session     int64
	lastTimeout time.Time
}

func (c *Connection) noteTimeout() {
	c.sec.mu.Lock()
	c.sec.lastTimeout = time.Now()
	c.sec.mu.Unlock()
}

func (c *Connection) lateAnswerWindow() time.Duration {
	return max(5*time.Second, 3*(1+time.Duration(c.Cfg.Link.MaxRetries))*max(msDur(c.Cfg.Link.TimeoutMs), time.Second))
}

// SetSecurity sets, before Start, where the security events go (nil: only
// logged) and the realtimeData collection the command checks read.
func (c *Connection) SetSecurity(rep *mbsec.Reporter, rt *mongo.Collection) {
	c.sec.mu.Lock()
	c.sec.rep, c.sec.rt = rep, rt
	c.sec.mu.Unlock()
}

// seedIdentity starts the change detection from the stored identity.
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
	if !c.Cfg.Link.PeerIdentification {
		return nil
	}
	c.sec.mu.Lock()
	defer c.sec.mu.Unlock()
	return c.sec.identity
}

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
	if e.Protocol == "" {
		e.Protocol = Protocol
	}
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

// secRepeat reports e at most once per repeatEvery for key.
func (c *Connection) secRepeat(key string, e jssecevents.Event) {
	ok, n := c.sec.repeat.Allow(key, time.Now())
	if !ok {
		return
	}
	e.Count = n
	c.secEmit(e)
}

// secNewLink starts the state of a connection attempt to addr.
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
	s.session = int64(secevent.NextConnID())
	s.addr, s.peerIP, s.peerTLS = addr, ip, nil
	s.mu.Unlock()
}

// --- TLS --------------------------------------------------------------------

// certCapture returns cfg, or a copy whose verification callback keeps the
// chain the device presented.
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

// secHandshake records the device's TLS session, or reports a failed one.
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

type identityKey struct{ k, v, label string }

// commonPoints are the Common model (1) points of the identification, with
// the names stored in stats.peerIdentity.
var commonPoints = []struct{ point, field string }{
	{"Mn", "manufacturer"}, {"Md", "model"}, {"Opt", "options"}, {"Vr", "version"},
	{"SN", "serialNumber"}, {"DA", "deviceAddress"},
}

// identifyPeer reads the Common model of every discovered unit, publishes
// the identity in the statistics and reports what changed since the
// previous discovery.
func (c *Connection) identifyPeer(ctx context.Context) {
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
	for _, us := range c.unitList() {
		ud := bson.M{"unitId": us.unit, "baseAddress": int(us.dev.Discovery.BaseAddress),
			"registerType": c.Cfg.RegisterType, "models": chainString(us.dev.Discovery)}
		add(fmt.Sprintf("models:%d", us.unit), chainString(us.dev.Discovery), fmt.Sprintf("unit %d model chain", us.unit))
		if inst := us.dev.ModelByID(1); inst != nil {
			var m *sunspec.DecodedModel
			err := c.exec(ctx, false, false, func(ctx context.Context) error {
				var e error
				m, e = us.dev.ReadModel(ctx, *inst)
				return e
			})
			if err != nil {
				ud["error"] = clientapp.ErrText(err)
			} else {
				var parts []string
				for _, cp := range commonPoints {
					p := m.Point(cp.point)
					if p == nil || !p.Implemented {
						continue
					}
					v := printable(PointValue(p).Str)
					if v == "" {
						continue
					}
					ud[cp.field] = v
					if cp.point != "DA" {
						parts = append(parts, v)
					}
				}
				add(fmt.Sprintf("unit:%d", us.unit), strings.Join(parts, " | "), fmt.Sprintf("unit %d device identification", us.unit))
			}
		} else {
			ud["error"] = "no Common model (1)"
		}
		units = append(units, ud)
		if ctx.Err() != nil {
			return
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
		c.secEmit(jssecevents.Event{Kind: kind, ObjectRef: k.k, Value: k.v, Previous: prev, Detail: k.label + " changed"})
	}
	id["keys"] = stored
	if first {
		c.secEmit(jssecevents.Event{Kind: mbsec.KindPeerIdentified, Value: identitySummary(id),
			Detail: "first identification of the device"})
	}
	c.sec.mu.Lock()
	c.sec.identity = id
	c.sec.mu.Unlock()
	jslog.Log(jslog.LevelBasic, "%s: peer identity: %s", c.Cfg.Name(), identitySummary(id))
}

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
			var p []string
			for _, n := range []string{"manufacturer", "model", "version", "serialNumber"} {
				if v, _ := ud[n].(string); v != "" {
					p = append(p, v)
				}
			}
			parts = append(parts, fmt.Sprintf("unit%v=%s models=%v", ud["unitId"], strings.Join(p, "/"), ud["models"]))
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

// --- discovery and reads ----------------------------------------------------

// secDiscoveryError reports a unit without a SunSpec map, an invalid model
// chain or a response that violates the protocol.
func (c *Connection) secDiscoveryError(unit int, err error) {
	f := mbsec.Fields(unit, -1, -1, -1, 0)
	switch {
	case errors.Is(err, sunspec.ErrNotSunSpec):
		c.secOnce(fmt.Sprintf("nosunspec|%d", unit), jssecevents.Event{Kind: mbsec.KindAccessUnitUnknown,
			Action: "rejected", ObjectRef: fmt.Sprintf("unit %d", unit), Fields: f, Error: err.Error(),
			Detail: "no SunSpec map at the configured unit"})
	case errors.Is(err, sunspec.ErrModelChainInvalid), errors.Is(err, sunspec.ErrModelChainLimitExceeded),
		errors.Is(err, sunspec.ErrModelLayout):
		c.secRepeat(fmt.Sprintf("chain|%d|%s", unit, err), jssecevents.Event{Kind: mbsec.KindProtoBadResponse,
			Action: "blocked", ObjectRef: fmt.Sprintf("unit %d", unit), Fields: f, Error: err.Error(),
			Detail: "invalid SunSpec model chain"})
	case isException(err):
		code, _ := mbsec.Exception(err)
		switch code {
		case mbcore.ExIllegalFunction:
			c.secOnce(fmt.Sprintf("discfc|%d", unit), jssecevents.Event{Kind: mbsec.KindAccessUnrecognizedService,
				Action: "rejected", ObjectRef: fmt.Sprintf("unit %d", unit), Fields: mbsec.Fields(unit, -1, -1, -1, code),
				Error: clientapp.ErrText(err), Detail: "the device refused the SunSpec discovery reads"})
		case 0x0A, 0x0B: // gateway path unavailable, gateway target failed to respond
			c.secOnce(fmt.Sprintf("nosunspec|%d", unit), jssecevents.Event{Kind: mbsec.KindAccessUnitUnknown,
				Action: "rejected", ObjectRef: fmt.Sprintf("unit %d", unit), Fields: mbsec.Fields(unit, -1, -1, -1, code),
				Error: clientapp.ErrText(err), Detail: "the configured unit does not answer behind the gateway"})
		}
	default:
		c.secResponseError(unit, err)
	}
}

// secReadError reports a model read the device refused for a lasting reason
// once per connection, and a response that violates the protocol.
func (c *Connection) secReadError(unit int, inst sunspec.ModelInstance, err error) {
	ref := fmt.Sprintf("unit %d model %d @%d", unit, inst.Header.ID, inst.Header.StartAddress)
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
		c.secOnce("read|"+ref, jssecevents.Event{Kind: kind, Action: "rejected", ObjectRef: ref,
			Fields: mbsec.Fields(unit, -1, int(inst.Header.StartAddress), int(inst.Header.Length)+2, code),
			Error:  clientapp.ErrText(err), Detail: "the device refused a model read"})
		return
	}
	var de *sunspec.DecodeError
	if errors.As(err, &de) {
		c.secRepeat("decode|"+ref, jssecevents.Event{Kind: mbsec.KindProtoBadResponse, Action: "blocked",
			ObjectRef: ref, Fields: mbsec.Fields(unit, -1, -1, -1, 0), Error: err.Error(),
			Detail: "model data the decoder refused"})
		return
	}
	c.secResponseError(unit, err)
}

// secMissingPoint reports, once per connection, a configured tag whose
// point the device does not expose.
func (c *Connection) secMissingPoint(b *binding, why string) {
	c.secOnce(fmt.Sprintf("missing|%d|%s", b.Unit, b.Addr), jssecevents.Event{Kind: mbsec.KindAccessObjectNonexistent,
		Action: "observed", ObjectRef: b.Addr.String(), Tag: b.Tag, Fields: mbsec.Fields(b.Unit, -1, -1, -1, 0),
		Detail: "configured point not exposed by the device: " + why})
}

// secResponseError reports a response that violates the Modbus protocol.
func (c *Connection) secResponseError(unit int, err error) {
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
		Fields: mbsec.Fields(unit, -1, -1, -1, 0), Detail: "invalid response from the device"})
}

// libLogger adds go-modbus's warnings about a response with another
// transaction id or protocol id than expected to the security events.
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
			return
		}
		kind = mbsec.KindProtoTxnMismatch
	case strings.Contains(msg, "unexpected protocol id"), strings.Contains(msg, "invalid MBAP length"):
		kind = mbsec.KindProtoBadResponse
	default:
		return
	}
	l.c.secRepeat("lib|"+kind, jssecevents.Event{Kind: kind, Source: mbsec.SourceLibrary, Action: "blocked", Detail: msg})
}

// --- commands ---------------------------------------------------------------

func commandFields(doc bson.M) bson.M {
	f := bson.M{}
	for _, k := range []string{"protocolSourceCommonAddress", "protocolSourceObjectAddress", "protocolSourceASDU"} {
		if v := fieldStr(doc, k); v != "" {
			f[k] = v
		}
	}
	return f
}

// secCommandRefused reports a command refused by the driver (kind given) or
// by the device (err); a timeout or a link failure is not reported.
func (c *Connection) secCommandRefused(doc bson.M, kind, desc string, err error) {
	e := jssecevents.Event{Kind: kind, Action: "rejected", Tag: fieldStr(doc, "tag"),
		ObjectRef: fieldStr(doc, "protocolSourceObjectAddress"), Fields: commandFields(doc), Detail: desc}
	if err != nil {
		code, ok := mbsec.Exception(err)
		if !ok {
			c.secResponseError(-1, err)
			return
		}
		e.Fields["exception"], e.Fields["exceptionName"] = code, mbcore.ExceptionName(code)
		e.Kind, e.Detail = mbsec.KindCmdRejected, "the device refused the command: "+clientapp.ErrText(err)
		if code == mbcore.ExIllegalDataAddress {
			e.Kind = mbsec.KindCmdObjectUnknown
		}
	}
	c.secEmit(e)
}

func (c *Connection) secCommandStale(doc bson.M, ts time.Time) {
	c.secEmit(jssecevents.Event{Kind: mbsec.KindCmdStaleTime, Action: "rejected", Tag: fieldStr(doc, "tag"),
		ObjectRef: fieldStr(doc, "protocolSourceObjectAddress"), Fields: commandFields(doc),
		Value: ts.UTC().Format(time.RFC3339), Detail: "command older than its expiry, not executed"})
}

// secCommand counts a command for the flood detector and checks, in the
// background, a numeric one against the value its supervised point already
// has.
func (c *Connection) secCommand(ctx context.Context, doc bson.M, value float64, numeric bool) {
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
	if rt == nil || !numeric || math.IsNaN(value) {
		return
	}
	key, before := jsmongo.GetDouble(doc, "pointKey", 0), time.Now()
	go func() {
		state, ok, err := jssecevents.SupervisedState(context.WithoutCancel(ctx), rt, key, before)
		if err != nil || !ok {
			return
		}
		if sameValue(state, value) {
			c.secEmit(jssecevents.Event{Kind: mbsec.KindCmdUnnecessary, Tag: fieldStr(doc, "tag"),
				ObjectRef: fieldStr(doc, "protocolSourceObjectAddress"), Value: fmt.Sprint(value),
				Fields: commandFields(doc), Detail: "the supervised point already had the commanded value"})
		}
	}()
}

// sameValue compares a supervised value with a commanded one, allowing for
// the rounding of a scale factor.
func sameValue(a, b float64) bool {
	return math.Abs(a-b) <= 1e-9*math.Max(1, math.Max(math.Abs(a), math.Abs(b)))
}
