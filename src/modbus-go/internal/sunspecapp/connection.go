/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package sunspecapp

// One SUNSPEC client connection: the link lifecycle with endpoint failover,
// the discovery of each unit's model chain, automatic tag creation, the poll
// loop and the request gate shared with commands.

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"modbus-go/internal/clientapp"
	"modbus-go/internal/drv"
	"modbus-go/internal/mbcore"
	"modbus-go/internal/mbnet"

	"github.com/otfabric/go-modbus"
	"github.com/otfabric/go-sunspec"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jsrtdata"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Stats are the counters published into protocolConnections.stats.
type Stats struct {
	Status          string
	ConnectedTo     string
	Reads           int64
	Writes          int64
	Errors          int64
	Timeouts        int64
	Exceptions      int64
	LastError       string
	GiCount         int64
	Discoveries     int64
	DiscoveredUnits int64
	Models          int64
}

// BSON renders the stats sub-document (nodeName and timeTag are added by the
// writer).
func (s Stats) BSON() bson.M {
	return bson.M{
		"status": s.Status, "connectedTo": s.ConnectedTo,
		"reads": s.Reads, "writes": s.Writes, "errors": s.Errors, "timeouts": s.Timeouts,
		"exceptions": s.Exceptions, "lastError": s.LastError, "giCount": s.GiCount,
		"discoveries": s.Discoveries, "discoveredUnits": s.DiscoveredUnits, "models": s.Models,
	}
}

// binding is one supervised tag.
type binding struct {
	ID   any
	Tag  string
	Unit int
	Addr Address
	// last is the value last written to realtimeData; nil forces the next
	// publish. seq is the read sequence of that value: an older read is not
	// written over it.
	last *Value
	seq  uint64
}

// unitState is the discovered device behind one unit id.
type unitState struct {
	unit       int
	dev        *sunspec.Device
	rediscover bool // the model chain changed: discover again
}

// Connection is one SUNSPEC client connection.
type Connection struct {
	Cfg  Config
	log  drv.ConnLogger
	sink clientapp.Sink

	// Commands, when set, is watched for this connection's commands while it runs.
	Commands clientapp.CommandStore
	// Active gates command execution on the redundancy state; nil means active.
	Active func() bool
	// Tags, when set and autoCreateTags is on, receives the tags of the
	// discovered points.
	Tags TagStore
	// LoadBindings, when set, reads the connection's tags again (after tags
	// were created).
	LoadBindings func(ctx context.Context) ([]bson.M, error)

	mu       sync.Mutex
	bindings []*binding
	byID     map[string]*binding
	units    map[int]*unitState

	clientMu  sync.RWMutex
	client    *modbus.Client
	connected atomic.Bool

	gate gate

	statsMu sync.Mutex
	stats   Stats

	runMu     sync.Mutex
	runCancel context.CancelFunc
	runDone   chan struct{}

	// readSeq orders the device reads (taken inside the gate); pubMu
	// serializes the realtimeData writes.
	readSeq atomic.Uint64
	pubMu   sync.Mutex

	endpointIndex int
	lastGi        time.Time
	sec           clientSec
}

// commandQueueSize bounds the commands waiting for execution.
const commandQueueSize = 256

// NewConnection prepares a connection; Start runs it.
func NewConnection(cfg Config, sink clientapp.Sink) *Connection {
	c := &Connection{Cfg: cfg, log: drv.ConnLogger{Prefix: cfg.Name()}, sink: sink,
		byID: map[string]*binding{}, units: map[int]*unitState{}}
	c.gate.cond = sync.NewCond(&c.gate.mu)
	c.sec.repeat.Every = repeatEvery
	c.seedIdentity(cfg.Link.StoredIdentity)
	c.stats.Status = "stopped"
	return c
}

// Stats returns a snapshot of the counters.
func (c *Connection) Stats() Stats {
	c.statsMu.Lock()
	defer c.statsMu.Unlock()
	return c.stats
}

func (c *Connection) stat(f func(s *Stats)) {
	c.statsMu.Lock()
	f(&c.stats)
	c.statsMu.Unlock()
}

// SetBindings replaces the tag bindings from realtimeData documents
// (protocolSourceCommonAddress, protocolSourceObjectAddress). A tag whose
// address did not change keeps its last published value.
func (c *Connection) SetBindings(docs []bson.M) {
	c.mu.Lock()
	defer c.mu.Unlock()
	old := c.byID
	var bindings []*binding
	byID := map[string]*binding{}
	for _, doc := range docs {
		tag := jsmongo.GetString(doc, "tag", "")
		addr, err := ParseAddress(drv.AnyString(doc["protocolSourceObjectAddress"]))
		if err != nil {
			c.log.Info(fmt.Sprintf("skipping tag %s - %v", tag, err))
			continue
		}
		unit := drv.ToNum(doc["protocolSourceCommonAddress"], doc["protocolSourceCommonAddress"] != nil, 1)
		if math.IsNaN(unit) || unit < 1 || unit > 255 || unit != math.Trunc(unit) {
			c.log.Info(fmt.Sprintf("skipping tag %s - invalid unit id (protocolSourceCommonAddress)", tag))
			continue
		}
		key := drv.AnyString(doc["_id"])
		b := &binding{ID: doc["_id"], Tag: tag, Unit: int(unit), Addr: addr}
		if o := old[key]; o != nil && o.Unit == b.Unit && o.Addr.String() == b.Addr.String() {
			b.last, b.seq = o.last, o.seq
		}
		bindings = append(bindings, b)
		byID[key] = b
	}
	c.bindings, c.byID = bindings, byID
	jslog.Log(jslog.LevelBasic, "%s: %d tags", c.Cfg.Name(), len(bindings))
}

// ReloadBindings reads the bindings again through LoadBindings.
func (c *Connection) ReloadBindings(ctx context.Context) error {
	if c.LoadBindings == nil {
		return nil
	}
	docs, err := c.LoadBindings(ctx)
	if err != nil {
		return err
	}
	c.SetBindings(docs)
	return nil
}

// Start runs the connection in the background; a running one is left alone.
func (c *Connection) Start() {
	c.runMu.Lock()
	defer c.runMu.Unlock()
	if c.runCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.runCancel = cancel
	c.runDone = make(chan struct{})
	done := c.runDone
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		if c.Cfg.Link.CommandsEnabled && c.Commands != nil {
			queue := make(chan bson.M, commandQueueSize)
			wg.Add(2)
			go func() {
				defer wg.Done()
				c.Commands.Watch(ctx, c.Cfg.ConnNumber(), func(doc bson.M) {
					select {
					case queue <- doc:
					case <-ctx.Done():
					}
				})
			}()
			go func() {
				defer wg.Done()
				c.runCommands(ctx, queue)
			}()
		}
		c.run(ctx)
		wg.Wait()
	}()
}

// runCommands executes queued commands one at a time, in arrival order,
// until ctx ends; the ones still queued then are acknowledged "connection down".
func (c *Connection) runCommands(ctx context.Context, queue chan bson.M) {
	for {
		select {
		case doc := <-queue:
			c.HandleCommand(ctx, doc)
			continue
		case <-ctx.Done():
		}
		for {
			select {
			case doc := <-queue:
				c.ack(ctx, doc, false, "connection down")
			default:
				return
			}
		}
	}
}

// Stop ends the connection and waits for it.
func (c *Connection) Stop() {
	c.runMu.Lock()
	cancel, done := c.runCancel, c.runDone
	c.runCancel, c.runDone = nil, nil
	c.runMu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
	c.stat(func(s *Stats) { s.Status = "stopped"; s.ConnectedTo = "" })
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

func msDur(ms float64) time.Duration {
	if ms <= 0 || math.IsNaN(ms) {
		return 0
	}
	return time.Duration(ms * float64(time.Millisecond))
}

// run is the link lifecycle: connect, discover, poll until the link fails,
// back off (1 s doubling to 30 s), move to the next address of ipAddresses,
// repeat. The status turns "connected", and the back-off resets, only once
// a SunSpec device was discovered.
func (c *Connection) run(ctx context.Context) {
	backoff := time.Second
	invalidated := false
	for ctx.Err() == nil {
		c.stat(func(s *Stats) { s.Status = "connecting" })
		cl, br, desc, err := c.open(ctx)
		c.stat(func(s *Stats) { s.ConnectedTo = desc })
		reason, answered := "", false
		if err == nil {
			jslog.Log(jslog.LevelBasic, "%s: connected to %s", c.Cfg.Name(), desc)
			c.clientMu.Lock()
			c.client = cl
			c.clientMu.Unlock()
			reason, answered = c.session(ctx, cl)
			c.connected.Store(false)
			c.clientMu.Lock()
			c.client = nil
			c.clientMu.Unlock()
			_ = cl.Close()
			if br != nil {
				br.Close()
			}
		} else {
			reason = err.Error()
			c.stat(func(s *Stats) { s.LastError = reason })
		}
		if ctx.Err() != nil {
			break
		}
		if answered {
			backoff, invalidated = time.Second, false
		}
		jslog.Log(jslog.LevelBasic, "%s: link down (%s): %s", c.Cfg.Name(), desc, reason)
		c.linkDown(ctx, &invalidated)
		if !sleepCtx(ctx, backoff) {
			break
		}
		backoff = min(30*time.Second, backoff*2)
	}
	c.stat(func(s *Stats) { s.Status = "stopped"; s.ConnectedTo = "" })
}

// session discovers the units and polls them until the link fails.
func (c *Connection) session(ctx context.Context, cl *modbus.Client) (reason string, answered bool) {
	c.mu.Lock()
	c.units = map[int]*unitState{}
	c.mu.Unlock()
	c.lastGi = time.Time{}
	found, reason := c.discoverUnits(ctx, cl, c.Cfg.UnitIDs)
	if found == 0 {
		if reason == "" {
			reason = "no SunSpec device found"
		}
		return reason, false
	}
	c.connected.Store(true)
	c.stat(func(s *Stats) { s.Status = "connected" })
	c.afterDiscovery(ctx, true)
	return c.pollLoop(ctx), true
}

// linkDown marks every tag invalid once per outage and moves to the next
// endpoint.
func (c *Connection) linkDown(ctx context.Context, invalidated *bool) {
	c.stat(func(s *Stats) { s.Status = "disconnected" })
	if !*invalidated {
		*invalidated = c.publishAllInvalid(ctx)
	}
	n := max(1, len(c.Cfg.Link.IPAddresses))
	c.endpointIndex = (c.endpointIndex + 1) % n
}

// publishAllInvalid marks every tag invalid, keeping its last value; false
// when the write failed.
func (c *Connection) publishAllInvalid(ctx context.Context) bool {
	c.mu.Lock()
	bindings := c.bindings
	c.mu.Unlock()
	if len(bindings) == 0 {
		return true
	}
	u := newUpdater(c.Cfg.ConnNumber())
	seq := c.nextSeq()
	for _, b := range bindings {
		c.publish(u, b, Value{Invalid: true}, "3", true, seq)
	}
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := c.flush(wctx, u); err != nil {
		jslog.Log(jslog.LevelBasic, "%s: invalidate error: %v", c.Cfg.Name(), err)
		return false
	}
	return true
}

// endpoint is where the next connection attempt goes.
func (c *Connection) endpoint() (host string, port int) {
	eps := c.Cfg.Link.IPAddresses
	if len(eps) == 0 {
		eps = []string{"127.0.0.1"}
	}
	return mbnet.ParseHostPort(eps[c.endpointIndex%len(eps)], c.Cfg.Link.DefaultPort())
}

// open builds and opens the go-modbus client for the current endpoint, as
// the MODBUS client does: TLS modes and a local bind address go through a
// loopback bridge so the socket is opened with the connection's options.
func (c *Connection) open(ctx context.Context) (*modbus.Client, *mbnet.Bridge, string, error) {
	cfg := c.Cfg.Link
	timeout := msDur(cfg.TimeoutMs)
	if timeout <= 0 {
		timeout = time.Second
	}
	dialTimeout := max(3*time.Second, 3*timeout)
	mc := modbus.Config{
		Timeout:     timeout,
		DialTimeout: dialTimeout,
		Logger:      &libLogger{LibLogger: drv.LibLogger{Prefix: cfg.Name}, c: c},
		PDUTap:      modbus.PDUTapFunc(c.tap),
		MaxConns:    1,
	}
	if !cfg.CommandsEnabled || c.Cfg.RegisterType == RegInput {
		mc.AllowedFunctionCodes = sunspec.ReadFunctionCodes()
	}
	baud := cfg.BaudRate
	if baud < 1 {
		baud = 9600
	}
	var br *mbnet.Bridge
	var desc string
	if cfg.IsSerial() {
		desc = fmt.Sprintf("serial://%s@%s", cfg.PortName, mbcore.JSNumberString(cfg.BaudRate))
		c.secNewLink("serial://" + cfg.PortName)
		par, warn := mbnet.Parity(cfg.Parity)
		if warn != "" {
			c.log.Error(warn)
		}
		if w := mbnet.HandshakeWarning(cfg.Handshake); w != "" {
			c.log.Error(w)
		}
		mc.URL = "rtu://" + cfg.PortName
		mc.Speed = uint(baud)
		mc.DataBits = 8
		mc.StopBits = uint(mbnet.StopBits(cfg.StopBits))
		switch par {
		case "E":
			mc.Parity = modbus.ParityEven
		case "O":
			mc.Parity = modbus.ParityOdd
		default:
			mc.Parity = modbus.ParityNone
		}
	} else {
		host, port := c.endpoint()
		target := mbnet.JoinHostPort(host, port)
		c.secNewLink(target)
		scheme := "tcp"
		if cfg.IsRTU() {
			scheme = "rtuovertcp"
			mc.Speed = uint(baud)
		}
		desc = "tcp://" + target
		if cfg.IsTLS() {
			desc = "tls://" + target
		}
		if cfg.IsTLS() || cfg.IPAddressLocalBind != "" {
			var tlsCfg *tls.Config
			if cfg.IsTLS() {
				var err error
				if tlsCfg, err = mbnet.ClientTLSConfig(cfg.TLS, host); err != nil {
					return nil, nil, desc, err
				}
			}
			dialer := &net.Dialer{Timeout: dialTimeout}
			if cfg.IPAddressLocalBind != "" {
				lh, lp := mbnet.ParseHostPort(cfg.IPAddressLocalBind, 0)
				la, err := net.ResolveTCPAddr("tcp", mbnet.JoinHostPort(lh, lp))
				if err != nil {
					return nil, nil, desc, fmt.Errorf("ipAddressLocalBind %q: %w", cfg.IPAddressLocalBind, err)
				}
				dialer.LocalAddr = la
			}
			dial := func(ctx context.Context) (net.Conn, error) {
				conn, err := dialer.DialContext(ctx, "tcp", target)
				if err != nil {
					return nil, err
				}
				if tc, ok := conn.(*net.TCPConn); ok {
					_ = tc.SetNoDelay(true)
				}
				if tlsCfg == nil {
					return conn, nil
				}
				tcfg, raw := certCapture(tlsCfg)
				t := tls.Client(conn, tcfg)
				hctx, cancel := context.WithTimeout(ctx, dialTimeout)
				defer cancel()
				if err := t.HandshakeContext(hctx); err != nil {
					_ = conn.Close()
					if ctx.Err() == nil {
						c.secHandshake(nil, *raw, err)
					}
					return nil, fmt.Errorf("TLS handshake: %w", err)
				}
				cs := t.ConnectionState()
				c.secHandshake(&cs, nil, nil)
				return t, nil
			}
			var err error
			if br, err = mbnet.NewBridge(dial); err != nil {
				return nil, nil, desc, err
			}
			pctx, cancel := context.WithTimeout(ctx, dialTimeout)
			err = br.Prime(pctx)
			cancel()
			if err != nil {
				br.Close()
				return nil, nil, desc, err
			}
			target = br.Addr()
		}
		mc.URL = scheme + "://" + target
	}
	cl, err := modbus.New(mc)
	if err == nil {
		err = cl.OpenContext(ctx)
	}
	if err != nil {
		if br != nil {
			br.Close()
		}
		return nil, nil, desc, err
	}
	return cl, br, desc, nil
}

// tap traces every answered request at log level 3.
func (c *Connection) tap(_ context.Context, ex modbus.PDUExchange) {
	if c.log.DebugEnabled() {
		c.log.Debug(fmt.Sprintf("unit=%d TX %s | RX %s", ex.UnitID, mbcore.DescribeRequest(ex.Request), mbcore.DescribeResponse(ex.Response)))
	}
}

// Connected reports whether a SunSpec device was discovered on the link.
func (c *Connection) Connected() bool { return c.connected.Load() }

func (c *Connection) currentClient() *modbus.Client {
	c.clientMu.RLock()
	defer c.clientMu.RUnlock()
	return c.client
}

// gate serializes requests on the link and lets commands overtake polling.
type gate struct {
	mu   sync.Mutex
	cond *sync.Cond
	busy bool
	prio int
}

func (g *gate) acquire(prio bool) {
	g.mu.Lock()
	if prio {
		g.prio++
	}
	for g.busy || (!prio && g.prio > 0) {
		g.cond.Wait()
	}
	if prio {
		g.prio--
	}
	g.busy = true
	g.mu.Unlock()
}

func (g *gate) release() {
	g.mu.Lock()
	g.busy = false
	g.cond.Broadcast()
	g.mu.Unlock()
}

// exec runs fn through the gate, retrying it up to maxRetries times while it
// fails with a timeout (retry false: never). interRequestDelayMs is observed
// after it.
func (c *Connection) exec(ctx context.Context, prio, retry bool, fn func(ctx context.Context) error) error {
	c.gate.acquire(prio)
	defer func() {
		sleepCtx(ctx, msDur(c.Cfg.Link.InterRequestDelayMs))
		c.gate.release()
	}()
	retries := 0
	if retry {
		retries = max(0, int(c.Cfg.Link.MaxRetries))
	}
	for left := retries; ; left-- {
		err := fn(ctx)
		if isTimeout(err) {
			c.noteTimeout()
		}
		if !isTimeout(err) || ctx.Err() != nil || left <= 0 {
			return err
		}
		c.stat(func(s *Stats) { s.Timeouts++ })
		c.log.Debug(fmt.Sprintf("response timeout - retrying (%d left)", left-1))
	}
}

func isTimeout(err error) bool {
	return errors.Is(err, modbus.ErrRequestTimedOut) || errors.Is(err, context.DeadlineExceeded)
}

func isException(err error) bool {
	var ex *modbus.ExceptionError
	return errors.As(err, &ex)
}

// linkError reports an error that says nothing came back from the device: a
// timeout or a transport failure, not an exception nor a SunSpec layout or
// decoding problem.
func linkError(err error) bool {
	if err == nil || isException(err) {
		return false
	}
	if isTimeout(err) {
		return true
	}
	var de *sunspec.DecodeError
	var ee *sunspec.EncodeError
	switch {
	case errors.Is(err, sunspec.ErrModelChanged), errors.Is(err, sunspec.ErrNotSunSpec),
		errors.Is(err, sunspec.ErrModelLayout), errors.Is(err, sunspec.ErrModelChainInvalid),
		errors.Is(err, sunspec.ErrModelChainLimitExceeded), errors.Is(err, sunspec.ErrDecode),
		errors.Is(err, sunspec.ErrPointNotWritable), errors.Is(err, sunspec.ErrPointNotFound),
		errors.Is(err, sunspec.ErrEncode), errors.Is(err, sunspec.ErrValueOutOfRange),
		errors.Is(err, modbus.ErrFunctionCodeNotAllowed), errors.As(err, &ee),
		errors.As(err, &de), errors.Is(err, context.Canceled):
		return false
	}
	return true
}

// discoverUnits discovers the model chain of each unit. found counts the
// units with a SunSpec device; reason names why none was found.
func (c *Connection) discoverUnits(ctx context.Context, cl *modbus.Client, units []int) (found int, reason string) {
	for _, u := range units {
		if ctx.Err() != nil {
			return found, "stopped"
		}
		var dev *sunspec.Device
		c.stat(func(s *Stats) { s.Discoveries++ })
		err := c.exec(ctx, false, true, func(ctx context.Context) error {
			var e error
			dev, e = sunspec.Discover(ctx, cl, c.Cfg.DiscoverOptions(u))
			return e
		})
		if err != nil {
			if ctx.Err() != nil {
				return found, "stopped"
			}
			reason = fmt.Sprintf("unit %d: %s", u, clientapp.ErrText(err))
			c.stat(func(s *Stats) { s.LastError = reason })
			jslog.Log(jslog.LevelBasic, "%s: discovery failed, %s", c.Cfg.Name(), reason)
			c.secDiscoveryError(u, err)
			c.mu.Lock()
			delete(c.units, u)
			c.mu.Unlock()

			continue
		}
		found++
		jslog.Log(jslog.LevelBasic, "%s: unit %d: SunSpec map at %d (%s), %d models: %s", c.Cfg.Name(), u,
			dev.Discovery.BaseAddress, c.Cfg.RegisterType, len(dev.Discovery.Models), chainString(dev.Discovery))
		for _, w := range dev.Discovery.Warnings {
			jslog.Log(jslog.LevelDetailed, "%s: unit %d: %s", c.Cfg.Name(), u, w)
		}
		c.mu.Lock()
		c.units[u] = &unitState{unit: u, dev: dev}
		c.mu.Unlock()
	}
	c.mu.Lock()
	nu, nm := int64(len(c.units)), int64(0)
	for _, us := range c.units {
		nm += int64(len(us.dev.Discovery.Models))
	}
	c.mu.Unlock()
	c.stat(func(s *Stats) { s.DiscoveredUnits, s.Models = nu, nm })
	return found, reason
}

// chainString lists the model ids of a discovery.
func chainString(d *sunspec.DiscoveryResult) string {
	ids := make([]string, len(d.Models))
	for i, m := range d.Models {
		ids[i] = fmt.Sprint(m.Header.ID)
	}
	return strings.Join(ids, ",")
}

// unitList is the discovered units in order.
func (c *Connection) unitList() []*unitState {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*unitState, 0, len(c.units))
	for _, us := range c.units {
		out = append(out, us)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].unit < out[j].unit })
	return out
}

// afterDiscovery identifies the device and creates the tags of the
// discovered points.
func (c *Connection) afterDiscovery(ctx context.Context, first bool) {
	if c.Cfg.Link.PeerIdentification {
		c.identifyPeer(ctx)
	}
	if !c.Cfg.Link.AutoCreateTags || c.Tags == nil {
		return
	}
	units := c.unitList()
	created := 0
	for _, us := range units {
		insts := us.dev.Discovery.Models
		var res []sunspec.ModelResult
		err := c.exec(ctx, false, true, func(ctx context.Context) error {
			var e error
			res, e = us.dev.ReadModels(ctx, insts)
			return e
		})
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			jslog.Log(jslog.LevelDetailed, "%s: unit %d: reading the models for tag creation: %v", c.Cfg.Name(), us.unit, err)
		}
		models := make([]*sunspec.DecodedModel, len(insts))
		for i := range res {
			if res[i].Err == nil || (res[i].Model != nil && res[i].Model.Group != nil) {
				models[i] = res[i].Model
			}
		}
		n, err := AutoCreateTags(ctx, c.Cfg, UnitModels{Unit: us.unit, Discovery: us.dev.Discovery, Models: models,
			MultiUnit: len(c.Cfg.UnitIDs) > 1}, c.Tags)
		created += n
		if err != nil {
			jslog.Log(jslog.LevelBasic, "%s: unit %d: auto-tag error: %v", c.Cfg.Name(), us.unit, err)
		}
	}
	if created > 0 || first {
		if err := c.ReloadBindings(ctx); err != nil {
			jslog.Log(jslog.LevelBasic, "%s: bindings load error: %v", c.Cfg.Name(), err)
		}
	}
}

// pollLoop polls until ctx ends or a whole cycle gets no answer.
func (c *Connection) pollLoop(ctx context.Context) string {
	interval := msDur(c.Cfg.Link.PollingIntervalMs)
	for ctx.Err() == nil {
		start := time.Now()
		if reason, lost := c.rediscoverUnits(ctx); lost {
			return reason
		}
		answered, failed, lastErr, empty := c.pollCycle(ctx)
		if ctx.Err() != nil {
			return "stopped"
		}
		if answered == 0 && failed > 0 {
			return lastErr
		}
		wait := interval - time.Since(start)
		if empty && wait < time.Second {
			wait = time.Second // nothing to poll: do not spin
		}
		if !sleepCtx(ctx, wait) {
			return "stopped"
		}
	}
	return "stopped"
}

// rediscoverUnits discovers again the units whose model chain changed, and,
// on an integrity cycle, the configured units without a SunSpec device.
// lost reports a link that no longer answers.
func (c *Connection) rediscoverUnits(ctx context.Context) (reason string, lost bool) {
	gi := c.giDue()
	c.mu.Lock()
	var units []int
	for _, u := range c.Cfg.UnitIDs {
		us := c.units[u]
		if (gi && !c.lastGi.IsZero()) || (us != nil && us.rediscover) {
			units = append(units, u)
		}
	}
	c.mu.Unlock()
	if len(units) == 0 {
		return "", false
	}
	found, reason := c.discoverUnits(ctx, c.currentClient(), units)
	if found == 0 && len(c.unitList()) == 0 {
		return reason, true
	}
	if found > 0 {
		c.afterDiscovery(ctx, false)
	}
	return "", false
}

func (c *Connection) giDue() bool {
	return c.lastGi.IsZero() || time.Since(c.lastGi) >= time.Duration(c.Cfg.Link.GiIntervalS*float64(time.Second))
}

// pollCycle reads, per unit, the model instances the tags use. answered
// counts the units that answered (an exception included), failed the ones
// that timed out or hit a transport error; empty means nothing to read.
func (c *Connection) pollCycle(ctx context.Context) (answered, failed int, lastErr string, empty bool) {
	isGi := c.giDue()
	if isGi {
		c.lastGi = time.Now()
		c.stat(func(s *Stats) { s.GiCount++ })
	}
	cot := "3"
	if isGi {
		cot = "20"
	}
	c.mu.Lock()
	bindings := c.bindings
	units := map[int]*unitState{}
	for k, v := range c.units {
		units[k] = v
	}
	c.mu.Unlock()
	empty = len(bindings) == 0
	byUnit := map[int][]*binding{}
	var unitIDs []int
	for _, b := range bindings {
		if _, ok := byUnit[b.Unit]; !ok {
			unitIDs = append(unitIDs, b.Unit)
		}
		byUnit[b.Unit] = append(byUnit[b.Unit], b)
	}
	sort.Ints(unitIDs)
	u := newUpdater(c.Cfg.ConnNumber())
	for _, unit := range unitIDs {
		if ctx.Err() != nil {
			return
		}
		list := byUnit[unit]
		us := units[unit]
		if us == nil {
			seq := c.nextSeq()
			for _, b := range list {
				c.secMissingPoint(b, "unit not discovered")
				c.publish(u, b, Value{Invalid: true}, cot, isGi, seq)
			}
			continue
		}
		var insts []sunspec.ModelInstance
		idx := map[uint16]int{} // header address -> index in insts
		where := make([]int, len(list))
		for i, b := range list {
			inst, ok := instanceOf(us.dev.Discovery, b.Addr)
			if !ok {
				where[i] = -1
				continue
			}
			j, ok := idx[inst.Header.StartAddress]
			if !ok {
				j = len(insts)
				idx[inst.Header.StartAddress] = j
				insts = append(insts, inst)
			}
			where[i] = j
		}
		var res []sunspec.ModelResult
		var err error
		var seq uint64
		if len(insts) > 0 {
			c.stat(func(s *Stats) { s.Reads++ })
			err = c.exec(ctx, false, true, func(ctx context.Context) error {
				var e error
				seq = c.nextSeq()
				res, e = us.dev.ReadModels(ctx, insts)
				return e
			})
			if ctx.Err() != nil {
				return
			}
		} else {
			seq = c.nextSeq()
		}
		alive := err == nil
		for i := range res {
			if e := res[i].Err; e == nil || !linkError(e) {
				alive = true
			}
		}
		if err != nil {
			c.countError(err)
			lastErr = clientapp.ErrText(err)
			jslog.Log(jslog.LevelDetailed, "%s: unit %d read failed: %s", c.Cfg.Name(), unit, lastErr)
		}
		if len(insts) == 0 {
			// No device response in this cycle. Discovery checks liveness at the integrity interval.
		} else if alive {
			answered++
		} else {
			failed++
		}
		reported := map[int]bool{}
		for i, b := range list {
			if where[i] < 0 {
				c.secMissingPoint(b, "model not in the device's chain")
				c.publish(u, b, Value{Invalid: true}, cot, isGi, seq)
				continue
			}
			r := res[where[i]]
			if r.Err != nil && !reported[where[i]] {
				reported[where[i]] = true
				if errors.Is(r.Err, sunspec.ErrModelChanged) {
					jslog.Log(jslog.LevelBasic, "%s: unit %d: model chain changed, discovering again", c.Cfg.Name(), unit)
					c.mu.Lock()
					us.rediscover = true
					c.mu.Unlock()
				}
				c.secReadError(unit, r.Instance, r.Err)
			}
			var p *sunspec.DecodedPoint
			if r.Model != nil && (r.Err == nil || !errors.Is(r.Err, sunspec.ErrModelChanged)) {
				p = pointOf(r.Model, b.Addr)
				if p == nil && r.Err == nil {
					c.secMissingPoint(b, "point not in the decoded model")
				}
			}
			c.publish(u, b, PointValue(p), cot, isGi, seq)
		}

	}
	jslog.Log(jslog.LevelDebug, "%s: poll cycle done, %d tag update(s)%s", c.Cfg.Name(), u.pending(),
		map[bool]string{true: " [integrity]", false: ""}[isGi])
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := c.flush(wctx, u); err != nil {
		jslog.Log(jslog.LevelBasic, "%s: realtimeData write error, values will be written again: %v", c.Cfg.Name(), err)
	}
	return
}

func (c *Connection) countError(err error) {
	c.stat(func(s *Stats) {
		switch {
		case isException(err):
			s.Exceptions++
		case isTimeout(err):
			s.Timeouts++
		default:
			s.Errors++
		}
		s.LastError = clientapp.ErrText(err)
	})
}

// nextSeq numbers a device read; reads are numbered inside the gate, so
// the order is the order on the link.
func (c *Connection) nextSeq() uint64 { return c.readSeq.Add(1) }

// publish queues a value read at seq when it differs from the value last
// written, or on an integrity cycle. An invalid value without data keeps
// the last value. The cache moves only when flush wrote the value.
func (c *Connection) publish(u *updater, b *binding, v Value, cot string, isGi bool, seq uint64) {
	c.mu.Lock()
	if v == (Value{Invalid: true}) && b.last != nil {
		v.Num, v.Str = b.last.Num, b.last.Str
	}
	changed := seq > b.seq && (b.last == nil || *b.last != v || isGi)
	c.mu.Unlock()
	if !changed {
		return
	}
	if c.log.DebugEnabled() {
		inv := ""
		if v.Invalid {
			inv = " (invalid)"
		}
		c.log.Debug(fmt.Sprintf("  %s [unit %d %s] = %s%s", b.Tag, b.Unit, b.Addr, v.Str, inv))
	}
	u.queue(b, v, cot, seq)
}

// flush writes the queued values, one flush at a time, leaving out a value
// older than the one a later read already wrote; the written values become
// the bindings' last values, the ones not written are published again.
func (c *Connection) flush(ctx context.Context, u *updater) error {
	if len(u.entries) == 0 {
		return nil
	}
	entries := u.entries
	u.entries = nil
	c.pubMu.Lock()
	defer c.pubMu.Unlock()
	c.mu.Lock()
	ops := make([]clientapp.RtOp, 0, len(entries))
	kept := entries[:0]
	for _, e := range entries {
		if e.seq <= c.current(e.b).seq {
			continue
		}
		ops = append(ops, clientapp.RtOp{ID: e.b.ID, Set: e.set})
		kept = append(kept, e)
	}
	c.mu.Unlock()
	if len(ops) == 0 {
		return nil
	}
	err := c.sink.Apply(ctx, ops)
	c.mu.Lock()
	for _, e := range kept {
		b := c.current(e.b)
		if err != nil {
			b.last = nil
			continue
		}
		v := e.v
		b.last, b.seq = &v, e.seq
	}
	c.mu.Unlock()
	return err
}

// current is the binding that replaced b after a bindings reload (same tag
// and address), b when there is none. c.mu is held.
func (c *Connection) current(b *binding) *binding {
	if n := c.byID[drv.AnyString(b.ID)]; n != nil && n.Unit == b.Unit && n.Addr.String() == b.Addr.String() {
		return n
	}
	return b
}

// updater accumulates the sourceDataUpdate documents of a cycle.
type updater struct {
	originator string
	entries    []queued
}

// queued is one sourceDataUpdate waiting for flush.
type queued struct {
	b   *binding
	v   Value
	seq uint64
	set bson.M
}

func newUpdater(connNumber float64) *updater {
	return &updater{originator: "SUNSPEC|" + mbcore.JSNumberString(connNumber)}
}

// queue adds the whole sourceDataUpdate document (the data processors
// watch for it being replaced, an invalid value included).
func (u *updater) queue(b *binding, v Value, cot string, seq uint64) {
	sdu := jsrtdata.SourceDataUpdate{
		ValueAtSource:               v.Num,
		ValueStringAtSource:         v.Str,
		CauseOfTransmissionAtSource: cot,
		TimeTag:                     jsmongo.Now(),
		InvalidAtSource:             v.Invalid,
		Extra:                       bson.M{"transientAtSource": false, "originator": u.originator},
	}
	u.entries = append(u.entries, queued{b: b, v: v, seq: seq, set: bson.M{"sourceDataUpdate": sdu.BSON()}})
}

func (u *updater) pending() int { return len(u.entries) }
