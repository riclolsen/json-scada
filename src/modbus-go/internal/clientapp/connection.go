/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package clientapp

// One MODBUS client connection: link lifecycle with endpoint failover, the
// poll loop, value publishing and the request gate shared with commands.
// Ported from src/modbus/src/client/connection.ts; the protocol (framing,
// transaction matching, timeouts, CRC) is go-modbus's.

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"modbus-go/internal/drv"
	"modbus-go/internal/mbcore"
	"modbus-go/internal/mbnet"

	"github.com/otfabric/go-modbus"
	"github.com/riclolsen/json-scada/src/go-common/jslog"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Stats are the counters published into protocolConnections.stats.
type Stats struct {
	Status      string
	ConnectedTo string
	Requests    int64
	Errors      int64
	Timeouts    int64
	Exceptions  int64
	LastError   string
	GiCount     int64
}

// BSON renders the stats sub-document (nodeName and timeTag are added by the
// writer).
func (s Stats) BSON() bson.M {
	return bson.M{
		"status": s.Status, "connectedTo": s.ConnectedTo,
		"requests": s.Requests, "errors": s.Errors, "timeouts": s.Timeouts,
		"exceptions": s.Exceptions, "lastError": s.LastError, "giCount": s.GiCount,
	}
}

// Connection is one MODBUS client connection.
type Connection struct {
	Cfg  Config
	log  drv.ConnLogger
	sink Sink

	// Commands, when set, is watched for this connection's commands while it runs.
	Commands CommandStore
	// Active gates command execution on the redundancy state; nil means active.
	Active func() bool

	mu       sync.Mutex
	bindings []*Binding
	byID     map[string]*Binding
	blocks   []*Block

	clientMu  sync.RWMutex
	client    *modbus.Client
	connected atomic.Bool

	gate gate

	statsMu sync.Mutex
	stats   Stats

	runMu     sync.Mutex
	runCancel context.CancelFunc
	runDone   chan struct{}

	endpointIndex int
	lastGi        time.Time
}

// NewConnection prepares a connection; Start runs it.
func NewConnection(cfg Config, sink Sink) *Connection {
	c := &Connection{Cfg: cfg, log: drv.ConnLogger{Prefix: cfg.Name}, sink: sink, byID: map[string]*Binding{}}
	c.gate.cond = sync.NewCond(&c.gate.mu)
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
// (protocolSourceObjectAddress, protocolSourceASDU, protocolSourceCommonAddress,
// kconv1, kconv2). Every block's change cache restarts, so the next cycle
// publishes every value.
func (c *Connection) SetBindings(docs []bson.M) {
	d := c.Cfg.Defaults()
	var bindings []*Binding
	byID := map[string]*Binding{}
	for _, doc := range docs {
		ca, hasCA := doc["protocolSourceCommonAddress"]
		k1, hasK1 := doc["kconv1"]
		k2, hasK2 := doc["kconv2"]
		oa := ""
		if v, ok := doc["protocolSourceObjectAddress"]; ok && v != nil {
			oa = drv.AnyString(v)
		}
		asdu, _ := doc["protocolSourceASDU"].(string)
		tag, _ := doc["tag"].(string)
		b, err := BuildBinding(RawBinding{
			ID: doc["_id"], Tag: tag, ObjectAddress: oa, Asdu: asdu,
			CommonAddress: ca, HasCommon: hasCA, Kconv1: k1, HasKconv1: hasK1, Kconv2: k2, HasKconv2: hasK2,
		}, d, c.Cfg.UseModiconAddresses)
		if err != nil {
			c.log.Info(fmt.Sprintf("skipping tag %s - %v", drv.AnyString(doc["tag"]), err))
			continue
		}
		bindings = append(bindings, b)
		byID[drv.AnyString(doc["_id"])] = b
	}
	blocks := PlanReads(bindings, Limits{
		MaxReadRegisters: c.Cfg.MaxReadRegisters, MaxReadCoils: c.Cfg.MaxReadCoils, MaxAddressGap: c.Cfg.MaxAddressGap,
	})
	c.mu.Lock()
	c.bindings, c.byID, c.blocks = bindings, byID, blocks
	c.mu.Unlock()
	jslog.Log(jslog.LevelBasic, "%s: %d tags in %d read blocks", c.Cfg.Name, len(bindings), len(blocks))
	if jslog.Level() >= jslog.LevelDetailed {
		for _, b := range blocks {
			jslog.Log(jslog.LevelDetailed, "  block unit=%d %s:%d x%d (%d tags)", b.Unit, b.Area, b.Start, b.Count, len(b.Slices))
		}
	}
}

// BindingByKey finds a supervised binding by its _id rendered as a string.
func (c *Connection) BindingByKey(key string) (*Binding, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, ok := c.byID[key]
	return b, ok
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
		if c.Cfg.CommandsEnabled && c.Commands != nil {
			wg.Add(1)
			go func() {
				defer wg.Done()
				c.Commands.Watch(ctx, c.Cfg.ProtocolConnectionNumber, func(doc bson.M) {
					go c.HandleCommand(ctx, doc)
				})
			}()
		}
		c.run(ctx)
		wg.Wait()
	}()
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

// Running reports whether Start was called without a Stop.
func (c *Connection) Running() bool {
	c.runMu.Lock()
	defer c.runMu.Unlock()
	return c.runCancel != nil
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

// run is the link lifecycle: connect, poll until the link fails, back off
// (1 s doubling to 30 s), move to the next address of ipAddresses, repeat.
func (c *Connection) run(ctx context.Context) {
	backoff := time.Second
	invalidated := false
	for ctx.Err() == nil {
		c.stat(func(s *Stats) { s.Status = "connecting" })
		cl, br, desc, err := c.open(ctx)
		c.stat(func(s *Stats) { s.ConnectedTo = desc })
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			jslog.Log(jslog.LevelBasic, "%s: connect failed (%s): %v", c.Cfg.Name, desc, err)
			c.stat(func(s *Stats) { s.LastError = err.Error() })
			c.linkDown(ctx, &invalidated)
			if !sleepCtx(ctx, backoff) {
				break
			}
			backoff = min(30*time.Second, backoff*2)
			continue
		}
		jslog.Log(jslog.LevelBasic, "%s: connected to %s", c.Cfg.Name, desc)
		c.stat(func(s *Stats) { s.Status = "connected" })
		backoff = time.Second
		invalidated = false
		c.lastGi = time.Time{}
		c.clientMu.Lock()
		c.client = cl
		c.clientMu.Unlock()
		c.connected.Store(true)

		reason := c.pollLoop(ctx, cl)

		c.connected.Store(false)
		c.clientMu.Lock()
		c.client = nil
		c.clientMu.Unlock()
		_ = cl.Close()
		if br != nil {
			br.Close()
		}
		if ctx.Err() != nil {
			break
		}
		jslog.Log(jslog.LevelBasic, "%s: link down (%s): %s", c.Cfg.Name, desc, reason)
		c.linkDown(ctx, &invalidated)
		if !sleepCtx(ctx, backoff) {
			break
		}
		backoff = min(30*time.Second, backoff*2)
	}
	c.stat(func(s *Stats) { s.Status = "stopped"; s.ConnectedTo = "" })
}

// linkDown marks every tag invalid once per outage and moves to the next
// endpoint.
func (c *Connection) linkDown(ctx context.Context, invalidated *bool) {
	c.stat(func(s *Stats) { s.Status = "disconnected" })
	if !*invalidated {
		*invalidated = true
		c.publishAllInvalid(ctx)
	}
	n := len(c.Cfg.IPAddresses)
	if n < 1 {
		n = 1
	}
	c.endpointIndex = (c.endpointIndex + 1) % n
}

func (c *Connection) publishAllInvalid(ctx context.Context) {
	c.mu.Lock()
	ids := make([]any, 0, len(c.bindings))
	for _, b := range c.bindings {
		ids = append(ids, b.ID)
	}
	c.mu.Unlock()
	if len(ids) == 0 {
		return
	}
	u := NewUpdater(c.Cfg.ProtocolConnectionNumber)
	u.QueueInvalid(ids)
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := u.Flush(wctx, c.sink); err != nil {
		jslog.Log(jslog.LevelBasic, "%s: invalidate error: %v", c.Cfg.Name, err)
	}
}

// Endpoint describes where the next connection attempt goes.
func (c *Connection) endpoint() (host string, port int) {
	eps := c.Cfg.IPAddresses
	if len(eps) == 0 {
		eps = []string{"127.0.0.1"}
	}
	return mbnet.ParseHostPort(eps[c.endpointIndex%len(eps)], c.Cfg.DefaultPort())
}

// open builds and opens the go-modbus client for the current endpoint. TLS
// modes and a local bind address go through a loopback bridge so the socket
// is opened with the connection's own options.
func (c *Connection) open(ctx context.Context) (*modbus.Client, *mbnet.Bridge, string, error) {
	cfg := c.Cfg
	timeout := msDur(cfg.TimeoutMs)
	if timeout <= 0 {
		timeout = time.Second
	}
	dialTimeout := max(3*time.Second, 3*timeout)
	mc := modbus.Config{
		Timeout:     timeout,
		DialTimeout: dialTimeout,
		Logger:      drv.LibLogger{Prefix: cfg.Name},
		PDUTap:      modbus.PDUTapFunc(c.tap),
		MaxConns:    1,
	}
	baud := cfg.BaudRate
	if baud < 1 {
		baud = 9600
	}
	var br *mbnet.Bridge
	var desc string
	if cfg.IsSerial() {
		desc = fmt.Sprintf("serial://%s@%s", cfg.PortName, mbcore.JSNumberString(cfg.BaudRate))
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
		scheme := "tcp"
		if cfg.IsRTU() {
			scheme = "rtuovertcp"
			mc.Speed = uint(baud)
		}
		if cfg.IsTLS() {
			desc = "tls://" + target
		} else {
			desc = "tcp://" + target
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
				t := tls.Client(conn, tlsCfg)
				hctx, cancel := context.WithTimeout(ctx, dialTimeout)
				defer cancel()
				if err := t.HandshakeContext(hctx); err != nil {
					_ = conn.Close()
					return nil, fmt.Errorf("TLS handshake: %w", err)
				}
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
	if cfg.InterFrameDelayMs > 0 && cfg.IsRTU() {
		c.log.Info("interFrameDelayMs is not used by the client: go-modbus derives the RTU frame timing from baudRate")
	}
	jslog.Log(jslog.LevelDebug, "%s: opening %s framing=%s timeout=%sms retries=%s", cfg.Name, desc,
		map[bool]string{true: "rtu", false: "mbap"}[cfg.IsRTU()], mbcore.JSNumberString(cfg.TimeoutMs), mbcore.JSNumberString(cfg.MaxRetries))
	cl, err := modbus.New(mc)
	if err != nil {
		if br != nil {
			br.Close()
		}
		return nil, nil, desc, err
	}
	if err := cl.Open(); err != nil {
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
		c.log.Debug(fmt.Sprintf("RX unit=%d %s | PDU: %s", ex.UnitID, mbcore.DescribeResponse(ex.Response), mbcore.Hex(ex.Response, 0)))
	}
}

// Connected reports whether the link is up.
func (c *Connection) Connected() bool { return c.connected.Load() }

func (c *Connection) currentClient() *modbus.Client {
	c.clientMu.RLock()
	defer c.clientMu.RUnlock()
	return c.client
}

// gate serializes requests on the link and lets commands overtake polling,
// as the Node.js driver queued commands ahead of poll requests.
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

// exec runs one request through the gate, retrying a timed-out request up to
// maxRetries times. interRequestDelayMs is observed after every request.
func (c *Connection) exec(ctx context.Context, prio bool, unit int, pdu []byte, fn func(ctx context.Context) error) error {
	c.gate.acquire(prio)
	defer func() {
		sleepCtx(ctx, msDur(c.Cfg.InterRequestDelayMs))
		c.gate.release()
	}()
	retries := int(c.Cfg.MaxRetries)
	if retries < 0 {
		retries = 0
	}
	for left := retries; ; left-- {
		if c.log.DebugEnabled() {
			retry := ""
			if left < retries {
				retry = " retry=" + strconv.Itoa(retries-left)
			}
			c.log.Debug(fmt.Sprintf("TX unit=%d%s %s | PDU: %s", unit, retry, mbcore.DescribeRequest(pdu), mbcore.Hex(pdu, 0)))
		}
		err := fn(ctx)
		if err == nil || !errors.Is(err, modbus.ErrRequestTimedOut) || ctx.Err() != nil {
			return err
		}
		if left <= 0 {
			c.log.Debug(fmt.Sprintf("response timeout after %s ms, unit=%d %s - no retries left",
				mbcore.JSNumberString(c.Cfg.TimeoutMs), unit, mbcore.DescribeRequest(pdu)))
			return err
		}
		c.stat(func(s *Stats) { s.Timeouts++ })
		c.log.Debug(fmt.Sprintf("response timeout after %s ms, unit=%d %s - retrying (%d left)",
			mbcore.JSNumberString(c.Cfg.TimeoutMs), unit, mbcore.DescribeRequest(pdu), left-1))
	}
}

// ErrText renders a request error as the Node.js driver did, for lastError
// and for a command's resultDescription.
func ErrText(err error) string {
	var ex *modbus.ExceptionError
	switch {
	case errors.As(err, &ex):
		return mbcore.ExceptionText(int(ex.FunctionCode), int(ex.ExceptionCode))
	case errors.Is(err, modbus.ErrRequestTimedOut):
		return "response timeout"
	}
	return err.Error()
}

func isException(err error) bool {
	var ex *modbus.ExceptionError
	return errors.As(err, &ex)
}

// pollLoop polls until ctx ends or a whole cycle fails at the transport level
// (no answer at all, and at least one link error), which is how a lost link
// shows through go-modbus.
func (c *Connection) pollLoop(ctx context.Context, cl *modbus.Client) string {
	interval := msDur(c.Cfg.PollingIntervalMs)
	for ctx.Err() == nil {
		start := time.Now()
		answered, linkErrs, lastLinkErr := c.pollCycle(ctx, cl)
		if ctx.Err() != nil {
			return "stopped"
		}
		if answered == 0 && linkErrs > 0 {
			return lastLinkErr
		}
		if !sleepCtx(ctx, interval-time.Since(start)) {
			return "stopped"
		}
	}
	return "stopped"
}

func (c *Connection) pollCycle(ctx context.Context, cl *modbus.Client) (answered, linkErrs int, lastLinkErr string) {
	c.mu.Lock()
	blocks := c.blocks
	c.mu.Unlock()
	start := time.Now()
	jslog.Log(jslog.LevelDebug, "%s: poll cycle start (%d blocks)", c.Cfg.Name, len(blocks))
	isGi := c.lastGi.IsZero() || time.Since(c.lastGi) >= time.Duration(c.Cfg.GiIntervalS*float64(time.Second))
	if isGi {
		c.lastGi = time.Now()
		c.stat(func(s *Stats) { s.GiCount++ })
	}
	u := NewUpdater(c.Cfg.ProtocolConnectionNumber)
	for _, b := range blocks {
		if ctx.Err() != nil {
			return
		}
		ok, linkErr := c.readBlock(ctx, cl, b, u, isGi)
		if ok {
			answered++
		} else if linkErr != "" {
			linkErrs++
			lastLinkErr = linkErr
			if answered == 0 {
				// Nothing answered yet and the link failed: every further block
				// would only redial and fail the same way. End the cycle so the
				// connection fails over now.
				break
			}
		}
	}
	jslog.Log(jslog.LevelDebug, "%s: poll cycle done in %d ms, %d tag update(s)%s", c.Cfg.Name,
		time.Since(start).Milliseconds(), u.Pending(), map[bool]string{true: " [integrity]", false: ""}[isGi])
	wctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancel()
	if err := u.Flush(wctx, c.sink); err != nil {
		jslog.Log(jslog.LevelDetailed, "%s: poll error: %v", c.Cfg.Name, err)
	}
	return
}

// readRequestPDU is the request a block read sends, for traces.
func readRequestPDU(fc, start, count int) []byte {
	return []byte{byte(fc), byte(start >> 8), byte(start), byte(count >> 8), byte(count)}
}

// readBlock reads one block. ok means the device answered (an exception
// counts: the link is alive); linkErr names a transport failure.
func (c *Connection) readBlock(ctx context.Context, cl *modbus.Client, b *Block, u *Updater, isGi bool) (ok bool, linkErr string) {
	var raw []byte
	unit := uint8(b.Unit)
	addr, qty := uint16(b.Start), uint16(b.Count)
	c.stat(func(s *Stats) { s.Requests++ })
	err := c.exec(ctx, false, b.Unit, readRequestPDU(b.FC, b.Start, b.Count), func(ctx context.Context) error {
		switch b.Area {
		case mbcore.AreaCoil, mbcore.AreaDiscrete:
			var bits []bool
			var err error
			if b.Area == mbcore.AreaCoil {
				bits, err = cl.ReadCoils(ctx, unit, addr, qty)
			} else {
				bits, err = cl.ReadDiscreteInputs(ctx, unit, addr, qty)
			}
			if err != nil {
				return err
			}
			raw = make([]byte, (b.Count+7)/8)
			for i, v := range bits {
				if v {
					raw[i>>3] |= 1 << (i & 7)
				}
			}
		default:
			rt := modbus.HoldingRegister
			if b.Area == mbcore.AreaInput {
				rt = modbus.InputRegister
			}
			regs, err := cl.ReadRegisters(ctx, unit, addr, qty, rt)
			if err != nil {
				return err
			}
			raw = make([]byte, 2*len(regs))
			for i, r := range regs {
				raw[2*i], raw[2*i+1] = byte(r>>8), byte(r)
			}
		}
		return nil
	})
	if err != nil {
		if ctx.Err() != nil {
			return false, ""
		}
		msg := ErrText(err)
		switch {
		case isException(err):
			c.stat(func(s *Stats) { s.Exceptions++ })
			ok = true
		case errors.Is(err, modbus.ErrRequestTimedOut):
			c.stat(func(s *Stats) { s.Timeouts++ })
		default:
			c.stat(func(s *Stats) { s.Errors++ })
			linkErr = msg
		}
		c.stat(func(s *Stats) { s.LastError = msg })
		jslog.Log(jslog.LevelDetailed, "%s: read failed unit=%d %s:%d x%d - %s", c.Cfg.Name, b.Unit, b.Area, b.Start, b.Count, msg)
		if !b.invalidPublished {
			ids := make([]any, len(b.Slices))
			for i, s := range b.Slices {
				ids[i] = s.B.ID
			}
			u.QueueInvalid(ids)
			b.invalidPublished = true
			b.lastRaw = nil
		}
		return ok, linkErr
	}
	changed := b.lastRaw == nil || !bytesEqual(raw, b.lastRaw) || isGi
	if c.log.DebugEnabled() {
		skip := ""
		if !changed {
			skip = " (unchanged, skipped)"
		}
		c.log.Debug(fmt.Sprintf("block unit=%d %s:%d x%d -> %d bytes%s", b.Unit, b.Area, b.Start, b.Count, len(raw), skip))
	}
	b.invalidPublished = false
	if changed {
		c.decodeAndQueue(b, raw, u, isGi)
		b.lastRaw = raw
	}
	return true, ""
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (c *Connection) decodeAndQueue(b *Block, raw []byte, u *Updater, isGi bool) {
	bitArea := mbcore.IsBitArea(b.Area)
	cot := "3"
	if isGi {
		cot = "20"
	}
	for _, s := range b.Slices {
		bd := s.B
		var v mbcore.Value
		invalid := false
		var js []int
		var err error
		switch {
		case bitArea:
			bit := 0
			if idx := s.RelStart >> 3; idx < len(raw) {
				bit = int(raw[idx]>>(s.RelStart&7)) & 1
			}
			if bd.Kconv1 == -1 {
				v = mbcore.Bool(bit == 0)
			} else {
				v = mbcore.Bool(bit == 1)
			}
		case bd.Bit >= 0:
			var dec mbcore.DecodeResult
			dec, err = mbcore.Decode(mbcore.TypeUint16, raw, s.RelStart, bd.Asdu.Perm, nil)
			word := int(dec.Value.Num)
			v = mbcore.Bool((word>>bd.Bit)&1 == 1)
		default:
			var dec mbcore.DecodeResult
			dec, err = mbcore.Decode(bd.Asdu.Type, raw, s.RelStart, bd.Asdu.Perm, bd.Asdu.Str)
			invalid = dec.Invalid
			js = dec.JSON
			if dec.Value.Kind == mbcore.KindNumber && bd.Asdu.Type != mbcore.TypeString {
				v = mbcore.Num(dec.Value.Num*bd.Kconv1 + bd.Kconv2)
			} else {
				v = dec.Value
			}
		}
		if err != nil {
			jslog.Log(jslog.LevelDetailed, "%s: decode error tag %s: %v", c.Cfg.Name, bd.Tag, err)
			u.QueueInvalid([]any{bd.ID})
			continue
		}
		if c.log.DebugEnabled() {
			bitS := ""
			if bd.Bit >= 0 {
				bitS = ".b" + strconv.Itoa(bd.Bit)
			}
			val := v.String()
			if v.Kind == mbcore.KindString {
				val = strconv.Quote(val)
			}
			inv := ""
			if invalid {
				inv = " (invalid)"
			}
			c.log.Debug(fmt.Sprintf("  %s [%s:%d%s %s] = %s%s", bd.Tag, bd.Area, bd.Offset, bitS, bd.Asdu.Label(), val, inv))
		}
		u.Queue(TagUpdate{ID: bd.ID, Value: v, JSON: js, AsduLabel: bd.Asdu.Label(), Invalid: invalid, Cot: cot})
	}
}
