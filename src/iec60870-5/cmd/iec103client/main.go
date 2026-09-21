/*
 * IEC 60870-5-103 Client Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Master (primary station) driver for IEC 60870-5-103, the informative
 * interface of protection equipment. Built on the go-iecp5 cs103 package.
 * Serial (FT1.2) or TCP-encapsulated transport, selected by portName.
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
	"strings"
	"sync"
	"time"

	"github.com/riclolsen/go-iecp5/cs101"
	"github.com/riclolsen/go-iecp5/cs103"
	"go.bug.st/serial"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"iec60870-5/internal/cliapp"
	"iec60870-5/internal/conv103"
	"iec60870-5/internal/jscfg"
	"iec60870-5/internal/model"
	"iec60870-5/internal/mongoutil"
	"iec60870-5/internal/tlsutil"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jsredundancy"
)

const (
	protocolDriverName = "IEC60870-5-103"
	driverVersion      = "0.4.0"
)

// redundancy arbitrates the active node; command execution gates on it.
var redundancy = &jsredundancy.Controller{}

// conn103 holds the runtime state of one IEC 103 client connection.
type conn103 struct {
	cfg     model.ConnCfg
	engConn *cliapp.Conn
	config  cs103.Config

	mu  sync.Mutex
	cli *cs103.Client

	cntGI       int
	cntTimeSync int
	scanNumber  byte
}

func (c *conn103) client() *cs103.Client {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cli
}

// cliHandler adapts the go-iecp5 cs103 client callbacks to the shared engine.
type cliHandler struct {
	c *conn103
	e *cliapp.Engine
}

func (h *cliHandler) ingest(a *cs103.ASDU) {
	res := conv103.Decode(a, int(h.c.cfg.ProtocolConnectionNumber))
	h.e.EnqueueValues(res.Values)
	h.e.EnqueueAcks(res.Acks)
}

func (h *cliHandler) TimeTaggedHandler(a *cs103.ASDU, _ cs103.TimeTaggedInfo) error {
	h.ingest(a)
	return nil
}
func (h *cliHandler) MeasurandsHandler(a *cs103.ASDU, _ cs103.MeasurandsInfo) error {
	h.ingest(a)
	return nil
}
func (h *cliHandler) IdentificationHandler(a *cs103.ASDU, info cs103.IdentificationInfo) error {
	jslog.Log(jslog.LevelBasic, "%s - Device %d identification: %q (COL %d)",
		h.c.cfg.Name, int(a.CommonAddr), info.ASCII, int(info.Col))
	return nil
}
func (h *cliHandler) GITerminationHandler(a *cs103.ASDU, scn byte) error {
	jslog.Log(jslog.LevelDetailed, "%s - Device %d GI termination, scan %d",
		h.c.cfg.Name, int(a.CommonAddr), int(scn))
	return nil
}
func (h *cliHandler) ASDUHandler(a *cs103.ASDU) error {
	jslog.Log(jslog.LevelDetailed, "%s - ASDU type %d cause %d from device %d",
		h.c.cfg.Name, int(a.Type), int(a.Coa), int(a.CommonAddr))
	return nil
}
func (h *cliHandler) ASDUHandlerAll(a *cs103.ASDU) error { return nil }

func buildConfig103(cc *model.ConnCfg) (cs103.Config, error) {
	cfg := cs103.DefaultConfig()
	cfg.LinkAddress = byte(defInt(int(cc.RemoteLinkAddress), 1))
	cfg.TimeoutResponseT1 = msToSeconds(cc.TimeoutForACK, 10, 2, 255)
	cfg.TimeoutRepeatT2 = msToSeconds(cc.TimeoutRepeat, 5, 1, 254)
	if cfg.TimeoutRepeatT2 >= cfg.TimeoutResponseT1 {
		cfg.TimeoutRepeatT2 = cfg.TimeoutResponseT1 - time.Second
	}
	if q := int(cc.MaxQueueSize); q > 0 {
		cfg.MaxSendQueueSize = q
	}
	// automatic time sync + GI on link activation is driven by the supervise
	// loop below (giInterval / timeSyncInterval), matching the 101/104 clients
	cfg.AutoInit = false

	portName := cc.PortName
	if portName == "" {
		portName = "COM1"
	}
	if strings.Contains(portName, ":") {
		cfg.Transport = cs103.TransportTCPClient
		cfg.TCP = cs101.TCPConfig{
			Address:        portName,
			ConnectTimeout: time.Duration(defInt(int(cc.T0), 30)) * time.Second,
		}
		if cc.LocalCertFilePath != "" {
			tlsCfg, err := tlsutil.BuildTLSConfig(cc, false)
			if err != nil {
				return cfg, err
			}
			cfg.TCP.TLSConfig = tlsCfg
		}
	} else {
		baud := int(cc.BaudRate)
		if baud == 0 {
			baud = 9600
		}
		parity := serial.EvenParity
		switch strings.ToLower(cc.Parity) {
		case "none":
			parity = serial.NoParity
		case "odd":
			parity = serial.OddParity
		}
		stopBits := serial.OneStopBit
		if strings.ToLower(cc.StopBits) == "two" {
			stopBits = serial.TwoStopBits
		}
		cfg.Serial = cs103.SerialConfig{
			Address:  portName,
			BaudRate: baud,
			DataBits: 8,
			StopBits: stopBits,
			Parity:   parity,
			Timeout:  5 * time.Second,
		}
	}
	return cfg, nil
}

func startClient(c *conn103, e *cliapp.Engine) error {
	opt := cs103.NewOption()
	if err := opt.SetConfig(c.config); err != nil {
		return err
	}
	opt.SetAutoReconnect(true)
	opt.SetReconnectInterval(10 * time.Second)

	cli := cs103.NewClient(&cliHandler{c: c, e: e}, opt)
	if cli == nil {
		return fmt.Errorf("invalid transport configuration")
	}
	if jslog.Level() >= jslog.LevelDebug {
		cli.SetLogMode(true)
	}
	cli.SetOnConnectHandler(func(cl *cs103.Client) {
		jslog.Log(jslog.LevelBasic, "%s", c.cfg.Name+" - Connected (link alive)")
	})
	cli.SetConnectionLostHandler(func(cl *cs103.Client, err error) {
		jslog.Log(jslog.LevelBasic, "%s - Connection lost: %v", c.cfg.Name, err)
		go e.InvalidateConnPoints(int(c.cfg.ProtocolConnectionNumber))
	})
	cli.SetConnectErrorHandler(func(cl *cs103.Client, err error) {
		jslog.Log(jslog.LevelBasic, "%s - Connect error: %v", c.cfg.Name, err)
	})
	cli.SetOnDeviceActiveHandler(func(cl *cs103.Client, addr byte) {
		jslog.Log(jslog.LevelBasic, "%s - Device %d link active", c.cfg.Name, int(addr))
	})
	c.mu.Lock()
	c.cli = cli
	c.mu.Unlock()
	return cli.Start()
}

func stopClient(c *conn103) {
	c.mu.Lock()
	cli := c.cli
	c.cli = nil
	c.mu.Unlock()
	if cli != nil {
		_ = cli.Close()
	}
}

// supervise runs the per-connection 1 s loop: link management, periodic
// general interrogation and time synchronization.
func supervise(c *conn103, e *cliapp.Engine) {
	giInterval := c.cfg.GiIntervalVal()
	tsInterval := int(c.cfg.TimeSyncInterval)
	devAddr := byte(defInt(int(c.cfg.RemoteLinkAddress), 1))

	c.cntGI = giInterval - 5
	c.cntTimeSync = tsInterval

	for {
		time.Sleep(1 * time.Second)
		if !redundancy.Active() {
			if c.client() != nil {
				c.cntGI = 0
				c.cntTimeSync = 0
				stopClient(c)
			}
			continue
		}

		cli := c.client()
		if cli == nil {
			c.cntGI = giInterval - 5
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

		if tsInterval > 0 {
			c.cntTimeSync++
		}
		if giInterval > 0 {
			c.cntGI++
		}

		if tsInterval > 0 && c.cntTimeSync >= tsInterval {
			jslog.Log(jslog.LevelDetailed, "%s", c.cfg.Name+" - Send Time Sync")
			c.cntTimeSync = 0
			if err := cli.TimeSync(devAddr); err != nil {
				jslog.Log(jslog.LevelBasic, "%s", c.cfg.Name+" - Link layer busy or not ready")
			}
		}
		if giInterval > 0 && c.cntGI >= giInterval {
			jslog.Log(jslog.LevelDetailed, "%s - General Interrogation device %d", c.cfg.Name, int(devAddr))
			c.cntGI = 0
			c.scanNumber++
			if err := cli.GeneralInterrogation(devAddr, c.scanNumber); err != nil {
				jslog.Log(jslog.LevelBasic, "%s", c.cfg.Name+" - Link layer busy or not ready")
			}
		}
	}
}

func main() {
	jslog.Log(jslog.LevelBasic, "{json:scada} IEC60870-5-103 Driver - Copyright 2020-2026 RLO")
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
	var conns []*conn103
	for _, cc := range connCfgs {
		c := &conn103{cfg: cc}
		c.config, err = buildConfig103(&c.cfg)
		if err != nil {
			jslog.Log(jslog.LevelBasic, "%s", cc.Name+" - "+err.Error())
			os.Exit(-1)
		}
		engConn := &cliapp.Conn{Cfg: cc}
		engConn.IsConnected = func() bool {
			cli := c.client()
			return cli != nil && cli.IsLinkActive()
		}
		engConn.RawCmd = func(asduNum, objAddr, commonAddr int, value float64, duration int) error {
			cli := c.client()
			if cli == nil {
				return fmt.Errorf("not connected")
			}
			fun, inf, dco := conv103.GeneralCommandParams(objAddr, value)
			return cli.GeneralCommand(byte(commonAddr), fun, inf, dco, byte(duration))
		}
		c.engConn = engConn
		engine.Conns = append(engine.Conns, engConn)
		conns = append(conns, c)
		jslog.Log(jslog.LevelBasic, "%s", cc.Name+" - New Connection, port: "+cc.PortName)
	}

	engine.PreloadInsertedAddresses(db)
	_ = client.Disconnect(context.Background())

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

	select {}
}

func defInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func msToSeconds(ms float64, def, min, max int) time.Duration {
	s := def
	if ms > 0 {
		s = int((ms + 999) / 1000)
	}
	if s < min {
		s = min
	}
	if s > max {
		s = max
	}
	return time.Duration(s) * time.Second
}
