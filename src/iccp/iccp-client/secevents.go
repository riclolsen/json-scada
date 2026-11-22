/*
 * ICCP/TASE.2 Client Driver for JSON-SCADA
 * {json:scada} - Copyright (c) 2020-present - Ricardo L. Olsen
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

// Security events (secEvents collection) and identification of the server.
//
// The library reports through one security state per connection, shared by
// its successive associations: dial and TLS failures (rejected server
// certificates), MAC and certificate changes, malformed PDUs, unsolicited
// responses and reports, report time stamps going backwards. The driver
// adds the server's identity (MMS Identify, TASE2_Version,
// Supported_Features, Bilateral_Table_ID) on every association and its
// changes, reads of points the server does not have, commands for unknown
// tags or objects, of the wrong type, refused, flooding or unnecessary.

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	"github.com/riclolsen/tase2/iso"
	"github.com/riclolsen/tase2/mms"
	"github.com/riclolsen/tase2/netpeer"
	"github.com/riclolsen/tase2/secevent"
	"github.com/riclolsen/tase2/tase2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	// Command flooding: more than commandFloodMax commands to the server
	// within commandFloodWindow.
	commandFloodMax    = 10
	commandFloodWindow = 10 * time.Second
	// maxIdentifiedDomains bounds the Bilateral_Table_ID reads.
	maxIdentifiedDomains = 32
	// A server that stays unreachable gives one conn.dial_failed event per
	// dialFailedEvery (with the count), not one per reconnect attempt.
	dialFailedEvery = 10 * time.Minute
)

// secWriter delivers the events of every connection; nil (tests) keeps
// them in no collection.
var (
	secWriter   *jssecevents.Writer
	secNodeName string
)

// connSec is the security event state of one connection.
type connSec struct {
	name     string
	origin   jssecevents.Origin
	ident    bool          // peerIdentification
	security *iso.Security // shared by the connection's successive endpoints

	changes jssecevents.Tracker // identity keys
	flood   jssecevents.RateLimit
	dials   jssecevents.Throttle

	mu       sync.Mutex
	ep       *tase2.Endpoint // the current association's endpoint
	reported map[string]bool // per association: what was already reported once
	identity bson.M          // stats.peerIdentity
}

// newConnSec creates the state of a connection, seeds the change detection
// with the identity stored in its statistics, and creates the library's
// report-only security policy (nil when it cannot).
func newConnSec(conn protocolConnection) *connSec {
	s := &connSec{
		name: conn.Name,
		origin: jssecevents.Origin{ProtocolDriver: DriverName,
			ProtocolDriverInstanceNumber: conn.ProtocolDriverInstanceNumber,
			ProtocolConnectionNumber:     conn.ProtocolConnectionNumber,
			ConnectionName:               conn.Name, NodeName: secNodeName},
		ident:    conn.peerIdentificationOn(),
		flood:    jssecevents.RateLimit{Max: commandFloodMax, Window: commandFloodWindow},
		dials:    jssecevents.Throttle{Every: dialFailedEvery},
		reported: map[string]bool{},
	}
	s.seed(conn.Stats.PeerIdentity)
	p := tase2.DefaultSecurityPolicy()
	p.Sink = secevent.SinkFunc(s.libraryEvent)
	sec, err := iso.NewSecurity(p)
	if err != nil {
		LogMsg(LogLevelMin, "ICCP %s - Security events disabled: %v", conn.Name, err)
		return s
	}
	s.security = sec
	return s
}

// storedIdentity is the part of stats.peerIdentity read back at startup.
type storedIdentity struct {
	IP          string    `bson:"ip"`
	MAC         string    `bson:"mac"`
	MACSource   string    `bson:"macSource"`
	CollectedAt time.Time `bson:"collectedAt"`
	TLS         struct {
		SHA256 string `bson:"sha256"`
	} `bson:"tls"`
	Keys []struct {
		K string `bson:"k"`
		V string `bson:"v"`
	} `bson:"keys"`
}

// seed starts the change detection from the stored identity, so a change
// across a driver restart is seen: the identity keys here, the MAC and the
// certificate in the library's peer history (which then reports them).
func (s *connSec) seed(raw bson.Raw) {
	if len(raw) == 0 {
		return
	}
	var st storedIdentity
	if err := bson.Unmarshal(raw, &st); err != nil {
		return
	}
	var stored bson.M
	if bson.Unmarshal(raw, &stored) == nil {
		s.identity = stored
	}
	for _, k := range st.Keys {
		s.changes.Seed(k.K, k.V)
	}
	ip, err := netip.ParseAddr(st.IP)
	if err != nil {
		return
	}
	at := st.CollectedAt
	if at.IsZero() || at.After(time.Now()) {
		at = time.Now()
	}
	if mac, err := net.ParseMAC(st.MAC); err == nil && st.MACSource == string(secevent.MACNeighbor) {
		netpeer.DefaultCache().Observe(netpeer.Observation{IP: ip, MAC: mac, Source: netpeer.SourceNeighbor, Time: at})
	}
	if st.TLS.SHA256 != "" {
		// A client observes the server's certificate before the association,
		// with no AP title: keyed by IP.
		netpeer.DefaultCache().Observe(netpeer.Observation{IP: ip, CertFingerprint: st.TLS.SHA256, Time: at})
	}
}

// libraryEvent stores an event of the library. The routine informational
// ones (connection accepted, association established, closed) are not
// stored, and an unreachable server reports once per dialFailedEvery.
func (s *connSec) libraryEvent(e secevent.Event) {
	if secWriter == nil {
		LogMsg(LogLevelDetailed, "ICCP %s - Security event %s", s.name, e.String())
		return
	}
	if e.Severity == secevent.SeverityInfo && kindInfo(e.Kind).Severity == secevent.SeverityInfo {
		return
	}
	if e.Kind == secevent.KindConnDialFailed {
		ok, n := s.dials.Allow(e.Reason, time.Now())
		if !ok {
			return
		}
		if n > 1 {
			e.Count = n
			e.Detail = strings.TrimSpace(fmt.Sprintf("%s (%d attempts since the last report)", e.Detail, n))
		}
	}
	if e.Role == "" {
		e.Role = "client"
	}
	secWriter.Emit(s.origin, toSecDoc(e, secSourceLibrary))
}

// attach prepares a new association on ep: it shares the connection's
// security state and starts a new set of once-only reports.
func (s *connSec) attach(ep *tase2.Endpoint) {
	if s == nil {
		return
	}
	if s.security != nil {
		ep.SetSecurity(s.security)
	}
	s.mu.Lock()
	s.ep = ep
	s.reported = map[string]bool{}
	s.mu.Unlock()
}

// connected reports that a connect attempt ended (ok false: failed): a
// working association resets the dial failure throttle, so the next outage
// is reported at once.
func (s *connSec) connected(ok bool) {
	if s != nil && ok {
		s.dials.Reset()
	}
}

// emit stores a driver event about the current association's server.
func (s *connSec) emit(e secevent.Event) {
	if s == nil {
		return
	}
	s.mu.Lock()
	ep := s.ep
	s.mu.Unlock()
	if ep != nil && !e.Peer.RemoteAddr.IsValid() {
		e.Peer = ep.PeerInfo()
	}
	e.Protocol, e.Role = "iccp", "client"
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	if secWriter == nil {
		LogMsg(LogLevelDetailed, "ICCP %s - Security event %s", s.name, e.String())
		return
	}
	secWriter.Emit(s.origin, toSecDoc(e, secSourceDriver))
}

// emitOnce reports e only the first time key is seen in this association.
func (s *connSec) emitOnce(key string, e secevent.Event) {
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.reported[key] || len(s.reported) > 10000 {
		s.mu.Unlock()
		return
	}
	s.reported[key] = true
	s.mu.Unlock()
	s.emit(e)
}

// close flushes and releases the library's security state.
func (s *connSec) close() {
	if s != nil && s.security != nil {
		s.security.Close()
	}
}

// PeerIdentityDoc is stats.peerIdentity, nil when never identified or when
// identification is off.
func (s *connSec) PeerIdentityDoc() bson.M {
	if s == nil || !s.ident {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.identity
}

// --- identification -----------------------------------------------------

// identityKey is one compared value of the identity.
type identityKey struct{ k, v, label string }

// identify reads the server's identity with the standard services and
// objects, publishes it in the statistics and reports what changed since
// the previous association. domains are the server's domains that hold
// mapped points (their Bilateral_Table_ID is read).
func (s *connSec) identify(cl *tase2.Client, ep *tase2.Endpoint, address string, domains []string) {
	if s == nil || !s.ident {
		return
	}
	id := bson.M{"collectedAt": time.Now(), "address": address}
	var keys []identityKey
	add := func(k, v, label string) {
		if v != "" {
			keys = append(keys, identityKey{k, v, label})
		}
	}

	// MMS Identify (ISO 9506) is the standard service; servers without it
	// may still have the VMD VendorName/ModelName/Revision variables.
	if vendor, model, rev, err := cl.Identify(); err == nil {
		id["vendor"], id["model"], id["revision"], id["method"] = vendor, model, rev, "identify"
		add("identify", vendor+" | "+model+" | "+rev, "MMS identity")
	} else if vendor, model, rev, err2 := cl.ReadPeerIdentity(); err2 == nil && vendor+model+rev != "" {
		id["vendor"], id["model"], id["revision"], id["method"] = vendor, model, rev, "vmdVariables"
		add("identify", vendor+" | "+model+" | "+rev, "MMS identity")
	} else {
		id["identifyError"] = err.Error()
	}

	// TASE.2 system variables: the VCC's TASE2_Version and
	// Supported_Features, and each domain's Bilateral_Table_ID.
	sort.Strings(domains)
	if len(domains) > maxIdentifiedDomains {
		domains = domains[:maxIdentifiedDomains]
	}
	refs := []tase2.ObjectRef{{Item: "TASE2_Version"}, {Item: "Supported_Features"}}
	for _, d := range domains {
		refs = append(refs, tase2.ObjectRef{Domain: d, Item: "Bilateral_Table_ID"})
	}
	if res, err := cl.ReadMultipleResults(refs); err == nil && len(res) == len(refs) {
		if v := tase2VersionText(res[0].Value); v != "" {
			id["tase2Version"] = v
			add("tase2Version", v, "TASE.2 version")
		}
		if f, ok := supportedFeatures(res[1].Value); ok {
			id["supportedFeatures"] = f
			add("supportedFeatures", strings.Join(f, ","), "supported conformance blocks")
		}
		blts := bson.A{}
		for i, d := range domains {
			v := res[i+2].Value
			if v == nil || v.VisibleStringVal == "" {
				continue
			}
			blts = append(blts, bson.M{"domain": d, "id": v.VisibleStringVal})
			add("blt:"+d, v.VisibleStringVal, "bilateral table of domain "+d)
		}
		if len(blts) > 0 {
			id["bilateralTables"] = blts
		}
	}

	pi := ep.PeerInfo()
	if pi.RemoteAddr.IsValid() {
		id["ip"] = pi.RemoteAddr.Addr().Unmap().String()
	}
	if len(pi.MAC) > 0 {
		id["mac"], id["macSource"] = pi.MAC.String(), string(pi.MACSource)
	}
	if pi.TLS != nil {
		id["tls"] = tlsDoc(pi.TLS)
	}
	if pi.APTitle != "" {
		id["apTitle"], id["aeQualifier"] = pi.APTitle, pi.AEQualifier
	}

	s.mu.Lock()
	previous := s.identity
	s.mu.Unlock()
	now := map[string]bool{}
	for _, k := range keys {
		now[k.k] = true
	}
	// Keep what this association could not read, so a later change of it
	// is still seen across restarts.
	stored := bson.A{}
	if prevKeys, ok := previous["keys"].(bson.A); ok {
		for _, pk := range prevKeys {
			k, v := keyValue(pk)
			if k != "" && !now[k] {
				stored = append(stored, bson.M{"k": k, "v": v})
			}
		}
	}
	for _, k := range keys {
		stored = append(stored, bson.M{"k": k.k, "v": k.v})
		prev, changed := s.changes.Observe(k.k, k.v)
		if !changed {
			continue
		}
		obj := ""
		if d, ok := strings.CutPrefix(k.k, "blt:"); ok {
			obj = d + "/Bilateral_Table_ID"
		}
		s.emit(secevent.Event{Kind: kindPeerIdentityChanged, Item: obj, Value: k.v,
			Detail: k.label + " changed, was " + prev})
	}
	id["keys"] = stored
	if previous == nil {
		s.emit(secevent.Event{Kind: kindPeerIdentified, Value: identitySummary(id),
			Detail: "first identification of the server"})
	}
	s.mu.Lock()
	s.identity = id
	s.mu.Unlock()
	LogMsg(LogLevelNormal, "ICCP %s - Peer identity: %s", s.name, identitySummary(id))
}

// keyValue reads a stored {k, v} entry, whichever document type it decoded to.
func keyValue(e any) (string, string) {
	switch d := e.(type) {
	case bson.M:
		k, _ := d["k"].(string)
		v, _ := d["v"].(string)
		return k, v
	case bson.D:
		var k, v string
		for _, f := range d {
			switch f.Key {
			case "k":
				k, _ = f.Value.(string)
			case "v":
				v, _ = f.Value.(string)
			}
		}
		return k, v
	}
	return "", ""
}

func identitySummary(id bson.M) string {
	var parts []string
	for _, f := range []string{"vendor", "model", "revision", "tase2Version", "ip", "mac"} {
		if v, ok := id[f].(string); ok && v != "" {
			parts = append(parts, f+"="+v)
		}
	}
	return strings.Join(parts, " ")
}

// structComponents returns the components of a structure in order: a
// structure read from the wire has them keyed by index ("0", "1", ...).
func structComponents(v *tase2.DataValue) []*tase2.DataValue {
	if v == nil {
		return nil
	}
	if len(v.StructFields) > 0 {
		out := make([]*tase2.DataValue, len(v.StructFields))
		for i, f := range v.StructFields {
			out[i] = f.Value
		}
		return out
	}
	var out []*tase2.DataValue
	for i := 0; i < len(v.StructVal); i++ {
		c, ok := v.StructVal[strconv.Itoa(i)]
		if !ok {
			break
		}
		out = append(out, c)
	}
	return out
}

// tase2VersionText renders TASE2_Version {MajorVersionNumber,
// MinorVersionNumber} as "major.minor", "" when it is not that structure.
func tase2VersionText(v *tase2.DataValue) string {
	c := structComponents(v)
	if len(c) < 2 {
		return ""
	}
	num := func(d *tase2.DataValue) (int64, bool) {
		switch {
		case d == nil:
		case d.IntVal != nil:
			return *d.IntVal, true
		case d.UnsignedVal != nil:
			return int64(*d.UnsignedVal), true
		}
		return 0, false
	}
	major, ok1 := num(c[0])
	minor, ok2 := num(c[1])
	if !ok1 || !ok2 {
		return ""
	}
	return fmt.Sprintf("%d.%d", major, minor)
}

// supportedFeatures decodes the Supported_Features bit string (bit n-1, MSB
// first, is conformance block n) into the names of the blocks it sets.
func supportedFeatures(v *tase2.DataValue) ([]string, bool) {
	if v == nil || v.BitStringVal == nil {
		return nil, false
	}
	out := []string{}
	for i := 0; i < 12 && i/8 < len(v.BitStringVal); i++ {
		if v.BitStringVal[i/8]&(0x80>>(i%8)) != 0 {
			out = append(out, fmt.Sprintf("CBB%d", i+1))
		}
	}
	return out, true
}

// --- reads --------------------------------------------------------------

// readRefused reports a polled point the server refused for a reason that
// is not a transient one: an object it does not have, or one it denies.
func (s *connSec) readRefused(m tagMapping, err error) {
	var dae *mms.DataAccessError
	if s == nil || !errors.As(err, &dae) {
		return
	}
	kind := secevent.KindAccessUnknownObject
	switch dae.ErrorCode {
	case mms.DataAccessObjectNonExistent, mms.DataAccessObjectUndefined:
	case mms.DataAccessObjectAccessDenied:
		kind = secevent.KindAccessDenied
	default:
		return
	}
	s.emitOnce("read:"+refKey(m.ref), secevent.Event{Kind: kind, Service: "read", Domain: m.ref.Domain,
		Item: m.ref.Item, ErrorCode: dae.ErrorCode, Err: err,
		Detail: "configured point refused by the server (tag " + m.tag.Tag + ")"})
}

// --- commands -----------------------------------------------------------

// commandRefused reports a command refused by the driver or the server.
// kind is kindAccessTagUnknown, kindCmdObjectUnknown, kindCmdWrongType or
// secevent.KindCmdRejected.
func (s *connSec) commandRefused(kind secevent.Kind, cmd commandQueueEntry, domain, item string, err error, detail string) {
	if s == nil {
		return
	}
	e := secevent.Event{Kind: kind, Action: secevent.ActionRejected, Service: "write", Domain: domain,
		Item: item, Value: commandValueText(cmd), Err: err, Detail: detail + " (tag " + cmd.Tag + ")"}
	var dae *mms.DataAccessError
	var se *mms.ServiceError
	switch {
	case errors.As(err, &dae):
		e.ErrorCode = dae.ErrorCode
	case errors.As(err, &se):
		e.ErrorClass, e.ErrorCode = se.Class, se.Code
	}
	s.emit(e)
}

func commandValueText(cmd commandQueueEntry) string {
	if cmd.ValueString != "" {
		return cmd.ValueString
	}
	return fmt.Sprint(cmd.Value)
}

// commandSent counts a command for the flood detector and reports one that
// would not change the state it commands. The check runs in the background,
// against the state before the dispatch: it never delays the command.
func (s *connSec) commandSent(rt *mongo.Collection, cmd commandQueueEntry, cmdTag rtData, domain, item string) {
	if s == nil {
		return
	}
	if over, n := s.flood.Hit("cmd", time.Now()); over {
		s.emit(secevent.Event{Kind: secevent.KindFloodControls, Domain: domain, Item: item, Value: fmt.Sprint(n),
			Detail: fmt.Sprintf("more than %d commands to the server within %s", commandFloodMax, commandFloodWindow)})
	}
	if rt == nil || cmdTag.Type != "digital" || (cmd.Value != 0 && cmd.Value != 1) {
		return
	}
	before, key := time.Now(), cmd.PointKey
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		state, ok, err := jssecevents.SupervisedState(ctx, rt, key, before)
		if err != nil || !ok {
			return
		}
		if (state != 0) == (cmd.Value != 0) {
			s.emit(secevent.Event{Kind: kindCmdUnnecessary, Domain: domain, Item: item, Value: fmt.Sprint(cmd.Value),
				Detail: "the supervised point already was in the commanded state (tag " + cmd.Tag + ")"})
		}
	}()
}

// writeCommand writes a command value and returns the server's refusal as
// a *mms.DataAccessError (or *mms.ServiceError) error when it refused it.
func writeCommand(cl *tase2.Client, domain, item string, value *tase2.DataValue) error {
	resp, err := cl.MMSClient.Write([]mms.ObjectRef{{Domain: domain, Item: item}}, []*mms.DataValue{value})
	if err != nil {
		return err
	}
	return writeResult(resp)
}

// selectControl selects an SBO control point by reading its "<item>_SBO"
// object (IEC 60870-6-802), as tase2 Client.Select does, but returns the
// server's refusal as a *mms.DataAccessError error.
func selectControl(cl *tase2.Client, domain, item string) error {
	resp, err := cl.MMSClient.Read([]mms.ObjectRef{{Domain: domain, Item: item + "_SBO"}}, false)
	if err != nil {
		return err
	}
	if resp == nil || len(resp.ListOfAccessResult) != 1 || resp.ListOfAccessResult[0] == nil {
		return errors.New("indeterminate select response")
	}
	if f := resp.ListOfAccessResult[0].Failure; f != nil {
		return fmt.Errorf("server refused the select (dataAccessError code=%d): %w", f.ErrorCode, f)
	}
	if resp.ListOfAccessResult[0].Success == nil {
		return errors.New("indeterminate select response")
	}
	return nil
}

// errWriteIndeterminate is a Write-Response that neither confirms nor
// refuses the single write: the outcome is unknown, never a success.
var errWriteIndeterminate = errors.New("indeterminate write response")

// writeResult checks the response of a single-variable write: exactly one
// result, and it must be the success marker.
func writeResult(resp *mms.WriteResponse) error {
	if resp == nil || len(resp.Results) != 1 {
		n := 0
		if resp != nil {
			n = len(resp.Results)
		}
		return fmt.Errorf("%w: %d results for one variable", errWriteIndeterminate, n)
	}
	switch r := resp.Results[0].(type) {
	case nil:
		return errors.New("server denied the write (no handler or access violation)")
	case *mms.DataAccessError:
		return fmt.Errorf("server denied the write (dataAccessError code=%d): %w", r.ErrorCode, r)
	case *mms.DataValue:
		if r != nil {
			return nil
		}
	}
	return fmt.Errorf("%w: result %T", errWriteIndeterminate, resp.Results[0])
}

// classifyCommandError maps a command write failure to the event kind it
// reports; ok is false for a failure that is not a refusal (a link error).
func classifyCommandError(err error) (kind secevent.Kind, detail string, ok bool) {
	var dae *mms.DataAccessError
	var se *mms.ServiceError
	switch {
	case errors.As(err, &dae):
		switch dae.ErrorCode {
		case mms.DataAccessObjectNonExistent, mms.DataAccessObjectUndefined:
			return kindCmdObjectUnknown, "the server has no such control point", true
		case mms.DataAccessTypeInconsistent, mms.DataAccessTypeUnsupported:
			return kindCmdWrongType, "the server refused the value type for the control point", true
		}
		return secevent.KindCmdRejected, "command refused by the server", true
	case errors.As(err, &se):
		if (se.Class == mms.SvcErrClassAccess && se.Code == mms.SvcErrAccessObjectNonExistent) ||
			(se.Class == mms.SvcErrClassDefinition && se.Code == mms.SvcErrDefinitionObjectUndefined) {
			return kindCmdObjectUnknown, "the server has no such control point", true
		}
		return secevent.KindCmdRejected, "command refused by the server", true
	case err != nil && strings.Contains(err.Error(), "server denied the write"):
		return secevent.KindCmdRejected, "command refused by the server", true
	}
	return "", "", false
}
