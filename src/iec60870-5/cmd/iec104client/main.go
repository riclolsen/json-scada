/*
 * IEC 60870-5-104 Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Go reimplementation of src/lib60870.netcore/iec104client using the
 * go-iecp5 library. Drop-in replacement: same protocol driver name,
 * MongoDB semantics and CLI contract.
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
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/go-iecp5/cs104"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"iec60870-5/internal/cliapp"
	"iec60870-5/internal/conv"
	"iec60870-5/internal/jscfg"
	"iec60870-5/internal/model"
	"iec60870-5/internal/mongoutil"
	"iec60870-5/internal/tlsutil"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jsredundancy"
)

const (
	protocolDriverName = "IEC60870-5-104"
	driverVersion      = "0.3.0"
)

// redundancy arbitrates the active node; command execution gates on it.
var redundancy = &jsredundancy.Controller{}

// conn104 holds the runtime state of one IEC 104 client connection.
type conn104 struct {
	cfg     model.ConnCfg
	engConn *cliapp.Conn
	tlsCfg  *tls.Config
	params  asdu.Params
	config  cs104.Config

	mu              sync.Mutex
	cli             *cs104.Client
	connected       atomic.Bool
	addrIdx         int // index into cfg.IPAddresses (dual-server failover)
	lastConnAttempt time.Time

	cntGI          int
	cntTestCommand int
	cntTimeSync    int
	cntTestSeq     uint16
}

func (c *conn104) client() *cs104.Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cli
}

// cliHandler adapts the go-iecp5 client callbacks to the shared engine.
type cliHandler struct {
	c *conn104
	e *cliapp.Engine
}

func (h *cliHandler) logConfirmation(kind string, pack *asdu.ASDU) error {
	neg := "Positive"
	if pack.Coa.IsNegative {
		neg = "Negative"
	}
	jslog.Log(jslog.LevelDetailed, "%s - %s %s", h.c.cfg.Name, neg, kind)
	return nil
}

func (h *cliHandler) InterrogationHandler(_ asdu.Connect, pack *asdu.ASDU) error {
	return h.logConfirmation("confirmation for interrogation command", pack)
}
func (h *cliHandler) CounterInterrogationHandler(_ asdu.Connect, pack *asdu.ASDU) error {
	return h.logConfirmation("counter interrogation command", pack)
}
func (h *cliHandler) ReadHandler(_ asdu.Connect, pack *asdu.ASDU) error {
	return h.logConfirmation("read command", pack)
}
func (h *cliHandler) TestCommandHandler(_ asdu.Connect, pack *asdu.ASDU) error {
	return h.logConfirmation("confirmation for test command", pack)
}
func (h *cliHandler) ClockSyncHandler(_ asdu.Connect, pack *asdu.ASDU) error {
	return h.logConfirmation("clock synchronization command", pack)
}
func (h *cliHandler) ResetProcessHandler(_ asdu.Connect, pack *asdu.ASDU) error {
	return h.logConfirmation("reset process command", pack)
}
func (h *cliHandler) DelayAcquisitionHandler(_ asdu.Connect, pack *asdu.ASDU) error {
	return h.logConfirmation("delay acquisition command", pack)
}

func (h *cliHandler) ASDUHandler(cn asdu.Connect, pack *asdu.ASDU, _ *cs104.Server, _ int) error {
	connNumber := int(h.c.cfg.ProtocolConnectionNumber)
	jslog.Log(jslog.LevelDetailed, "%s - %s", h.c.cfg.Name, pack.String())
	res := conv.Decode(pack, connNumber)
	h.e.EnqueueValues(res.Values)
	h.e.EnqueueAcks(res.Acks)
	// SBO: send the stored execute twin when the select is confirmed
	for _, sel := range res.Selects {
		if exec := h.c.engConn.SelectedFor(sel.Ioa); exec != nil {
			coa := asdu.CauseOfTransmission{Cause: asdu.Activation}
			if err := conv.SendCommand(cn, coa, asdu.CommonAddr(sel.Ca), exec); err != nil {
				jslog.Log(jslog.LevelBasic, "%s - Error sending execute after select: %s", h.c.cfg.Name, err.Error())
			} else {
				jslog.Log(jslog.LevelDetailed, "%s - Sending command execute after select confirmed, Object Address %d",
					h.c.cfg.Name, sel.Ioa)
			}
		}
	}
	return nil
}

func (h *cliHandler) ASDUHandlerAll(asdu.Connect, *asdu.ASDU, *cs104.Server, int) error { return nil }

// startClient creates and starts a cs104 client for the current server address.
func startClient(c *conn104, e *cliapp.Engine) error {
	addr := c.cfg.IPAddresses[c.addrIdx]
	if !strings.Contains(addr, ":") {
		addr += ":2404"
	}
	scheme := "tcp://"
	if c.tlsCfg != nil {
		scheme = "tls://"
	}
	opt := cs104.NewOption()
	if err := opt.AddRemoteServer(scheme + addr); err != nil {
		return err
	}
	_ = opt.SetConfig(c.config)
	_ = opt.SetParams(&c.params)
	opt.SetAutoReconnect(false)
	if c.tlsCfg != nil {
		opt.SetTLSConfig(c.tlsCfg)
	}

	cli := cs104.NewClient(&cliHandler{c: c, e: e}, opt)
	if jslog.Level() >= jslog.LevelDebug {
		cli.LogMode(true)
	}
	cli.SetOnConnectHandler(func(cl *cs104.Client) {
		jslog.Log(jslog.LevelBasic, "%s", c.cfg.Name+" - Connected")
		cl.SendStartDt() // required to activate the 104 link
	})
	cli.SetOnActivatedHandler(func(cl *cs104.Client) {
		jslog.Log(jslog.LevelDetailed, "%s", c.cfg.Name+" - STARTDT CON received")
		c.connected.Store(true)
	})
	cli.SetConnectionLostHandler(func(cl *cs104.Client) {
		wasConnected := c.connected.Swap(false)
		jslog.Log(jslog.LevelBasic, "%s", c.cfg.Name+" - Connection closed")
		if wasConnected {
			go e.InvalidateConnPoints(int(c.cfg.ProtocolConnectionNumber))
		}
	})
	c.mu.Lock()
	c.cli = cli
	c.mu.Unlock()
	c.lastConnAttempt = time.Now()
	jslog.Log(jslog.LevelBasic, "%s - Connecting to %s", c.cfg.Name, addr)
	return cli.Start()
}

func stopClient(c *conn104) {
	c.mu.Lock()
	cli := c.cli
	c.cli = nil
	c.mu.Unlock()
	c.connected.Store(false)
	if cli != nil {
		_ = cli.Close()
	}
}

// supervise runs the per-connection 1 s loop: connection management and
// periodic GI / test command / clock sync (port of the C# main loop).
func supervise(c *conn104, e *cliapp.Engine) {
	giInterval := c.cfg.GiIntervalVal()
	testInterval := int(c.cfg.TestCommandInterval)
	tsInterval := int(c.cfg.TimeSyncInterval)
	remoteCA := asdu.CommonAddr(c.cfg.RemoteLinkAddress)

	// C# parity: initial counter offsets
	c.cntGI = giInterval - 3
	c.cntTestCommand = testInterval - 1
	c.cntTimeSync = 0

	resetCounters := func() {
		c.cntGI = giInterval - 2
		c.cntTestCommand = testInterval - 1
		c.cntTimeSync = tsInterval
		c.cntTestSeq = 0
	}

	for {
		time.Sleep(1 * time.Second)
		if !redundancy.Active() {
			if c.client() != nil {
				resetCounters()
				stopClient(c)
			}
			continue
		}

		cli := c.client()
		if cli == nil || !cli.IsConnected() {
			if cli != nil && c.connected.Load() {
				c.connected.Store(false)
			}
			// rate-limit reconnection attempts
			if time.Since(c.lastConnAttempt) < 3*time.Second {
				continue
			}
			resetCounters()
			stopClient(c)
			// swap server when a secondary is configured
			if len(c.cfg.IPAddresses) > 1 {
				c.addrIdx = (c.addrIdx + 1) % len(c.cfg.IPAddresses)
				jslog.Log(jslog.LevelBasic, "%s - Trying server %s", c.cfg.Name, c.cfg.IPAddresses[c.addrIdx])
			}
			if err := startClient(c, e); err != nil {
				jslog.Log(jslog.LevelBasic, "%s - Error connecting! %s", c.cfg.Name, err.Error())
			}
			continue
		}

		if !cli.IsActive() {
			continue // connected but STARTDT not confirmed yet
		}

		if testInterval > 0 {
			c.cntTestCommand++
		}
		if giInterval > 0 {
			c.cntGI++
		}
		if tsInterval > 0 {
			c.cntTimeSync++
		}

		if giInterval > 0 && c.cntGI >= giInterval {
			jslog.Log(jslog.LevelDetailed, "%s", c.cfg.Name+" - Send Interrogation Request")
			c.cntGI = 0
			if err := cli.InterrogationCmd(asdu.CauseOfTransmission{Cause: asdu.Activation},
				remoteCA, asdu.QOIStation); err != nil {
				jslog.Log(jslog.LevelDetailed, "%s", c.cfg.Name+" - GI error: "+err.Error())
			}
		}
		if testInterval > 0 && c.cntTestCommand >= testInterval {
			jslog.Log(jslog.LevelDetailed, "%s", c.cfg.Name+" - Send Test Command")
			c.cntTestCommand = 0
			c.cntTestSeq++
			if err := asdu.TestCommandCP56Time2a(cli,
				asdu.CauseOfTransmission{Cause: asdu.Activation}, remoteCA, time.Now()); err != nil {
				jslog.Log(jslog.LevelDetailed, "%s", c.cfg.Name+" - Test command error: "+err.Error())
			}
		}
		if tsInterval > 0 && c.cntTimeSync >= tsInterval {
			jslog.Log(jslog.LevelDetailed, "%s", c.cfg.Name+" - Send Clock Sync")
			c.cntTimeSync = 0
			if err := cli.ClockSynchronizationCmd(asdu.CauseOfTransmission{Cause: asdu.Activation},
				remoteCA, time.Now()); err != nil {
				jslog.Log(jslog.LevelDetailed, "%s", c.cfg.Name+" - Clock sync error: "+err.Error())
			}
		}
	}
}

func main() {
	jslog.Log(jslog.LevelBasic, "{json:scada} IEC60870-5-104 Driver - Copyright 2020-2026 RLO")
	jslog.Log(jslog.LevelBasic, "Driver version "+driverVersion+" (Go/go-iecp5)")

	cfg, instanceNumber, err := jscfg.Read()
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s", err.Error())
		os.Exit(-1)
	}
	jslog.Log(jslog.LevelBasic, "%s", "MongoDB database name: "+cfg.MongoDatabaseName)
	jslog.Log(jslog.LevelBasic, "%s", "Node name: "+cfg.NodeName)

	client, db, err := jsmongo.ConnectAndPing(cfg)
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s", "Error connecting to MongoDB: "+err.Error())
		os.Exit(-1)
	}

	if _, err := mongoutil.LoadInstance(db, protocolDriverName, instanceNumber, cfg.NodeName); err != nil {
		jslog.Log(jslog.LevelBasic, "%s", err.Error())
		os.Exit(-1)
	}
	jslog.Log(jslog.LevelBasic, "Instance: %d", instanceNumber)

	connCfgs, err := mongoutil.LoadConns(db, protocolDriverName, instanceNumber)
	if err != nil || len(connCfgs) == 0 {
		jslog.Log(jslog.LevelBasic, "No connections found!")
		os.Exit(-1)
	}

	engine := cliapp.New(cfg, protocolDriverName, instanceNumber, redundancy.Active)
	var conns []*conn104
	for _, cc := range connCfgs {
		if len(cc.IPAddresses) < 1 || cc.IPAddresses[0] == "" {
			jslog.Log(jslog.LevelBasic, "%s", cc.Name+" - Missing ipAddresses list!")
			os.Exit(-1)
		}
		c := &conn104{cfg: cc}
		// defaults (C# BsonDefaultValue parity)
		c.params = asdu.Params{
			CauseSize:       defInt(int(cc.SizeOfCOT), 2),
			CommonAddrSize:  defInt(int(cc.SizeOfCA), 2),
			InfoObjAddrSize: defInt(int(cc.SizeOfIOA), 3),
			OrigAddress:     asdu.OriginAddr(cc.LocalLinkAddress),
			InfoObjTimeZone: time.Local,
		}
		c.config = cs104.Config{
			ConnectTimeout0:   time.Duration(defInt(int(cc.T0), 10)) * time.Second,
			SendUnAckLimitK:   uint16(defInt(int(cc.K), 12)),
			SendUnAckTimeout1: time.Duration(defInt(int(cc.T1), 15)) * time.Second,
			RecvUnAckLimitW:   uint16(defInt(int(cc.W), 8)),
			RecvUnAckTimeout2: time.Duration(defInt(int(cc.T2), 10)) * time.Second,
			IdleTimeout3:      time.Duration(defInt(int(cc.T3), 20)) * time.Second,
		}
		tlsCfg, err := tlsutil.BuildTLSConfig(&c.cfg, false)
		if err != nil {
			jslog.Log(jslog.LevelBasic, "%s", cc.Name+" - Error configuring TLS certificates.")
			jslog.Log(jslog.LevelBasic, "%s", cc.Name+" - "+err.Error())
			os.Exit(1)
		}
		c.tlsCfg = tlsCfg

		engConn := &cliapp.Conn{Cfg: cc}
		engConn.IsConnected = func() bool {
			cli := c.client()
			return cli != nil && cli.IsActive()
		}
		engConn.SendCmd = func(obj *conv.InfoObject, ca int) error {
			cli := c.client()
			if cli == nil {
				return fmt.Errorf("not connected")
			}
			return conv.SendCommand(cli, asdu.CauseOfTransmission{Cause: asdu.Activation}, asdu.CommonAddr(ca), obj)
		}
		c.engConn = engConn
		engine.Conns = append(engine.Conns, engConn)
		conns = append(conns, c)
		jslog.Log(jslog.LevelBasic, "%s", cc.Name+" - New Connection")
	}

	// Preload existing tag addresses so autoCreateTags never re-creates them
	engine.PreloadInsertedAddresses(db)
	_ = client.Disconnect(context.Background())

	// redundancy control with connection stats updates
	statsFn := func(db *mongo.Database) {
		collConns := db.Collection(jsmongo.ProtocolConnectionsCollectionName)
		for _, c := range conns {
			isConnected := false
			if cli := c.client(); cli != nil {
				isConnected = cli.IsConnected()
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, _ = collConns.UpdateOne(ctx,
				bson.M{"protocolConnectionNumber": c.cfg.ProtocolConnectionNumber},
				bson.M{"$set": bson.M{"stats": bson.M{
					"nodeName":    cfg.NodeName,
					"timeTag":     time.Now(),
					"isConnected": isConnected,
				}}})
			cancel()
		}
	}
	redundancy.Config = cfg
	redundancy.DriverName = protocolDriverName
	redundancy.InstanceNumber = instanceNumber
	// No OnActivate/OnDeactivate: acquisition runs on both nodes, and only
	// command execution gates on Active().
	redundancy.OnTick = statsFn
	go redundancy.Run(context.Background())

	// data/ack writer and commands change stream
	go engine.RunMongoWriter()
	go engine.RunCommandsStream()

	jslog.Log(jslog.LevelBasic, "Setting up IEC Connections & ASDU handlers...")
	for _, c := range conns {
		go supervise(c, engine)
	}

	select {} // run forever
}

func defInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
