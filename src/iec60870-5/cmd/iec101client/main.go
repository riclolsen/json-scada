/*
 * IEC 60870-5-101 Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Go reimplementation of src/lib60870.netcore/iec101client using the
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
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/go-iecp5/cs101"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"iec60870-5/internal/cliapp"
	"iec60870-5/internal/conv"
	"iec60870-5/internal/cs101util"
	"iec60870-5/internal/jscfg"
	"iec60870-5/internal/model"
	"iec60870-5/internal/mongoutil"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jsredundancy"
)

const (
	protocolDriverName = "IEC60870-5-101"
	driverVersion      = "0.3.0"
)

// redundancy arbitrates the active node; command execution gates on it.
var redundancy = &jsredundancy.Controller{}

// conn101 holds the runtime state of one IEC 101 client connection.
type conn101 struct {
	cfg     model.ConnCfg
	engConn *cliapp.Conn
	params  asdu.Params
	config  cs101.Config

	mu  sync.Mutex
	cli *cs101.Client

	cntGI          int
	cntTestCommand int
	cntTimeSync    int
}

func (c *conn101) client() *cs101.Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cli
}

// cliHandler adapts the go-iecp5 cs101 client callbacks to the shared engine.
// The cs101 client routes by cause of transmission: interrogation-caused
// data lands in InterrogationHandler, counter data in
// CounterInterrogationHandler, everything else in ASDUHandler.
type cliHandler struct {
	c *conn101
	e *cliapp.Engine
}

func (h *cliHandler) decode(pack *asdu.ASDU) error {
	connNumber := int(h.c.cfg.ProtocolConnectionNumber)
	jslog.Log(jslog.LevelDetailed, "%s - %s", h.c.cfg.Name, pack.String())
	res := conv.Decode(pack, connNumber)
	h.e.EnqueueValues(res.Values)
	h.e.EnqueueAcks(res.Acks)
	for _, sel := range res.Selects {
		if exec := h.c.engConn.SelectedFor(sel.Ioa); exec != nil {
			cli := h.c.client()
			if cli == nil {
				continue
			}
			coa := asdu.CauseOfTransmission{Cause: asdu.Activation}
			if err := conv.SendCommand(cli, coa, asdu.CommonAddr(sel.Ca), exec); err != nil {
				jslog.Log(jslog.LevelBasic, "%s - Error sending execute after select: %s", h.c.cfg.Name, err.Error())
			} else {
				jslog.Log(jslog.LevelDetailed, "%s - Sending command execute after select confirmed, Object Address %d",
					h.c.cfg.Name, sel.Ioa)
			}
		}
	}
	return nil
}

func (h *cliHandler) InterrogationHandler(pack *asdu.ASDU) error        { return h.decode(pack) }
func (h *cliHandler) CounterInterrogationHandler(pack *asdu.ASDU) error { return h.decode(pack) }
func (h *cliHandler) ReadHandler(pack *asdu.ASDU) error                 { return h.decode(pack) }
func (h *cliHandler) TestCommandHandler(pack *asdu.ASDU) error {
	jslog.Log(jslog.LevelDetailed, "%s - confirmation for test command", h.c.cfg.Name)
	return nil
}
func (h *cliHandler) ClockSyncHandler(pack *asdu.ASDU) error {
	jslog.Log(jslog.LevelDetailed, "%s - clock synchronization command", h.c.cfg.Name)
	return nil
}
func (h *cliHandler) ResetProcessHandler(pack *asdu.ASDU) error {
	jslog.Log(jslog.LevelDetailed, "%s - reset process command", h.c.cfg.Name)
	return nil
}
func (h *cliHandler) DelayAcquisitionHandler(pack *asdu.ASDU) error {
	jslog.Log(jslog.LevelDetailed, "%s - delay acquisition command", h.c.cfg.Name)
	return nil
}
func (h *cliHandler) ASDUHandler(pack *asdu.ASDU, _ int) error { return h.decode(pack) }
func (h *cliHandler) ASDUHandlerAll(*asdu.ASDU, int) error     { return nil }

func startClient(c *conn101, e *cliapp.Engine) error {
	opt := cs101.NewOption()
	if err := opt.SetConfig(c.config); err != nil {
		return err
	}
	if err := opt.SetParams(&c.params); err != nil {
		return err
	}
	opt.SetAutoReconnect(true)
	opt.SetReconnectInterval(10 * time.Second)

	cli := cs101.NewClient(&cliHandler{c: c, e: e}, opt)
	if cli == nil {
		return fmt.Errorf("invalid serial configuration")
	}
	if jslog.Level() >= jslog.LevelDebug {
		cli.SetLogMode(true)
	}
	cli.SetOnConnectHandler(func(cl *cs101.Client) {
		jslog.Log(jslog.LevelBasic, "%s", c.cfg.Name+" - Connected (link alive)")
	})
	cli.SetConnectionLostHandler(func(cl *cs101.Client, err error) {
		jslog.Log(jslog.LevelBasic, "%s - Connection lost: %v", c.cfg.Name, err)
		go e.InvalidateConnPoints(int(c.cfg.ProtocolConnectionNumber))
	})
	cli.SetConnectErrorHandler(func(cl *cs101.Client, err error) {
		jslog.Log(jslog.LevelBasic, "%s - Connect error: %v", c.cfg.Name, err)
	})
	c.mu.Lock()
	c.cli = cli
	c.mu.Unlock()
	return cli.Start()
}

func stopClient(c *conn101) {
	c.mu.Lock()
	cli := c.cli
	c.cli = nil
	c.mu.Unlock()
	if cli != nil {
		_ = cli.Close()
	}
}

// supervise runs the per-connection 1 s loop: link management and periodic
// GI / test command / clock sync (port of the C# 101 client main loop).
func supervise(c *conn101, e *cliapp.Engine) {
	giInterval := c.cfg.GiIntervalVal()
	testInterval := int(c.cfg.TestCommandInterval)
	tsInterval := int(c.cfg.TimeSyncInterval)
	remoteCA := asdu.CommonAddr(c.cfg.RemoteLinkAddress)

	// C# parity: initial counter offsets (101 variant)
	c.cntGI = giInterval - 5
	c.cntTestCommand = testInterval - 2
	c.cntTimeSync = tsInterval

	for {
		time.Sleep(1 * time.Second)
		if !redundancy.Active() {
			if c.client() != nil {
				c.cntGI = 0
				c.cntTimeSync = 0
				c.cntTestCommand = 0
				stopClient(c)
			}
			continue
		}

		cli := c.client()
		if cli == nil {
			c.cntGI = giInterval - 5
			c.cntTestCommand = testInterval - 2
			c.cntTimeSync = tsInterval
			if err := startClient(c, e); err != nil {
				jslog.Log(jslog.LevelBasic, "%s - Error connecting! %s", c.cfg.Name, err.Error())
				stopClient(c)
			}
			continue
		}

		if !cli.IsLinkActive() {
			continue
		}

		if giInterval > 0 {
			c.cntGI++
		}
		if testInterval > 0 {
			c.cntTestCommand++
		}
		if tsInterval > 0 {
			c.cntTimeSync++
		}

		if giInterval > 0 && c.cntGI >= giInterval {
			jslog.Log(jslog.LevelDetailed, "%s - Interrogation %d", c.cfg.Name, int(c.cfg.RemoteLinkAddress))
			c.cntGI = 0
			if err := cli.InterrogationCmd(asdu.CauseOfTransmission{Cause: asdu.Activation},
				remoteCA, asdu.QOIStation); err != nil {
				jslog.Log(jslog.LevelBasic, "%s", c.cfg.Name+" - Link layer busy or not ready")
			}
		}
		if testInterval > 0 && c.cntTestCommand >= testInterval {
			jslog.Log(jslog.LevelDetailed, "%s", c.cfg.Name+" - Test Command")
			c.cntTestCommand = 0
			if err := cli.TestCommand(asdu.CauseOfTransmission{Cause: asdu.Activation}, remoteCA); err != nil {
				jslog.Log(jslog.LevelBasic, "%s", c.cfg.Name+" - Link layer busy or not ready")
			}
		}
		if tsInterval > 0 && c.cntTimeSync >= tsInterval {
			jslog.Log(jslog.LevelDetailed, "%s", c.cfg.Name+" - Send Clock Sync")
			c.cntTimeSync = 0
			if err := cli.ClockSynchronizationCmd(asdu.CauseOfTransmission{Cause: asdu.Activation},
				remoteCA, time.Now()); err != nil {
				jslog.Log(jslog.LevelBasic, "%s", c.cfg.Name+" - Link layer busy or not ready")
			}
		}
	}
}

func main() {
	jslog.Log(jslog.LevelBasic, "{json:scada} IEC60870-5-101 Driver - Copyright 2020-2026 RLO")
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
	var conns []*conn101
	for _, cc := range connCfgs {
		c := &conn101{cfg: cc}
		c.params = cs101util.BuildParams(&c.cfg)
		c.config, err = cs101util.BuildConfig(&c.cfg, false)
		if err != nil {
			jslog.Log(jslog.LevelBasic, "%s", cc.Name+" - "+err.Error())
			os.Exit(-1)
		}
		engConn := &cliapp.Conn{Cfg: cc}
		engConn.IsConnected = func() bool {
			cli := c.client()
			return cli != nil && cli.IsLinkActive()
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
		jslog.Log(jslog.LevelBasic, "%s", cc.Name+" - New Connection, serial port: "+cc.PortName)
	}

	engine.PreloadInsertedAddresses(db)
	_ = client.Disconnect(context.Background())

	// redundancy control with link state stats (C# 101 parity)
	statsFn := func(db *mongo.Database) {
		collConns := db.Collection(jsmongo.ProtocolConnectionsCollectionName)
		for _, c := range conns {
			state := "ERROR"
			if cli := c.client(); cli != nil && cli.IsLinkActive() {
				state = "AVAILABLE"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			_, _ = collConns.UpdateOne(ctx,
				bson.M{"protocolConnectionNumber": c.cfg.ProtocolConnectionNumber},
				bson.M{"$set": bson.M{"stats": bson.M{
					"nodeName":       cfg.NodeName,
					"timeTag":        time.Now(),
					"linkLayerState": state,
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

	go engine.RunMongoWriter()
	go engine.RunCommandsStream()

	jslog.Log(jslog.LevelBasic, "Setting up IEC Connections & ASDU handlers...")
	for _, c := range conns {
		go supervise(c, engine)
	}

	select {} // run forever
}
