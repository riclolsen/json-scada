/*
 * IEC 60870-5-101 Server Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Go reimplementation of src/lib60870.netcore/iec101server using the
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
	"os"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/go-iecp5/cs101"

	"iec60870-5/internal/cs101util"
	"iec60870-5/internal/jscfg"
	"iec60870-5/internal/model"
	"iec60870-5/internal/mongoutil"
	"iec60870-5/internal/srvapp"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
)

const (
	protocolDriverName = "IEC60870-5-101_SERVER"
	driverVersion      = "0.3.0"
)

// srv101 holds the runtime state of one served IEC 101 connection.
type srv101 struct {
	cfg     model.ConnCfg
	engConn *srvapp.Conn
	server  *cs101.Server
	engine  *srvapp.Engine
}

// srvHandler adapts go-iecp5 cs101 server callbacks to the shared engine.
// cs101 handlers have no per-session Connect parameter: replies go through
// the captured server object (buffered into the class 1/2 queues).
type srvHandler struct {
	s *srv101
}

func (h *srvHandler) InterrogationHandler(pack *asdu.ASDU, qoi asdu.QualifierOfInterrogation) error {
	h.s.engine.Interrogation(h.s.engConn, h.s.server, pack, qoi)
	return nil
}
func (h *srvHandler) CounterInterrogationHandler(pack *asdu.ASDU, qcc asdu.QualifierCountCall) error {
	h.s.engine.HandleCounterInterrogation(h.s.engConn, h.s.server, pack, qcc)
	return nil
}
func (h *srvHandler) ReadHandler(pack *asdu.ASDU, ioa asdu.InfoObjAddr) error {
	h.s.engine.HandleRead(h.s.engConn, h.s.server, pack, ioa)
	return nil
}
func (h *srvHandler) ClockSyncHandler(pack *asdu.ASDU, t time.Time) error {
	h.s.engine.HandleClockSync(h.s.engConn, h.s.server, pack, t)
	return nil
}
func (h *srvHandler) ResetProcessHandler(pack *asdu.ASDU, qrp asdu.QualifierOfResetProcessCmd) error {
	h.s.engine.HandleResetProcess(h.s.engConn, h.s.server, pack, qrp)
	return nil
}
func (h *srvHandler) DelayAcquisitionHandler(pack *asdu.ASDU, _ uint16) error {
	_ = pack.SendReplyMirror(h.s.server, asdu.ActivationCon)
	return nil
}
func (h *srvHandler) ASDUHandler(pack *asdu.ASDU) error {
	jslog.Log(jslog.LevelDetailed, "%s - %s", h.s.cfg.Name, pack.String())
	if !h.s.engine.HandleCommandASDU(h.s.engConn, h.s.server, pack) {
		jslog.Log(jslog.LevelBasic, "%s -   Not implemented type of ASDU received: %d", h.s.cfg.Name, pack.Type)
		pack.Coa.IsNegative = true
		_ = pack.SendReplyMirror(h.s.server, asdu.ActivationCon)
	}
	return nil
}
func (h *srvHandler) ASDUHandlerAll(*asdu.ASDU, int) error { return nil }

func main() {
	jslog.Log(jslog.LevelBasic, "{json:scada} IEC60870-5-101 Server Driver - Copyright 2020-2026 RLO")
	jslog.Log(jslog.LevelBasic, "Driver version "+driverVersion+" (Go/go-iecp5)")

	cfg, instanceNumber, err := jscfg.Read()
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s", err.Error())
		os.Exit(-1)
	}
	jslog.Log(jslog.LevelBasic, "%s", "MongoDB database name: "+cfg.MongoDatabaseName)
	jslog.Log(jslog.LevelBasic, "%s", "Node name: "+cfg.NodeName)

	engine, err := srvapp.New(cfg, protocolDriverName)
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s", "Error connecting to MongoDB: "+err.Error())
		os.Exit(-1)
	}

	if _, err := mongoutil.LoadInstance(engine.DB(), protocolDriverName, instanceNumber, cfg.NodeName); err != nil {
		jslog.Log(jslog.LevelBasic, "%s", err.Error())
		os.Exit(-1)
	}
	jslog.Log(jslog.LevelBasic, "Instance: %d", instanceNumber)

	connCfgs, err := mongoutil.LoadConns(engine.DB(), protocolDriverName, instanceNumber)
	if err != nil || len(connCfgs) == 0 {
		jslog.Log(jslog.LevelBasic, "No connections found!")
		os.Exit(-1)
	}

	var servers []*srv101
	for _, cc := range connCfgs {
		s := &srv101{cfg: cc, engine: engine}
		engConn := &srvapp.Conn{Cfg: cc}
		s.engConn = engConn
		engine.Conns = append(engine.Conns, engConn)
		servers = append(servers, s)
		jslog.Log(jslog.LevelBasic, "%s", cc.Name)
	}

	engine.DistributeAutoTags()

	jslog.Log(jslog.LevelBasic, "Setting up IEC Connections & ASDU handlers...")
	for _, s := range servers {
		cc := s.cfg
		server := cs101.NewServer(&srvHandler{s: s})
		s.server = server
		s.engConn.Endpoint = server
		srvRef := server
		s.engConn.HasSession = func() bool { return srvRef.IsConnected() }

		config, err := cs101util.BuildConfig(&s.cfg, true)
		if err != nil {
			jslog.Log(jslog.LevelBasic, "%s", cc.Name+" - "+err.Error())
			os.Exit(-1)
		}
		server.SetConfig(config)
		params := cs101util.BuildParams(&s.cfg)
		server.SetParams(&params)
		if jslog.Level() >= jslog.LevelDebug {
			server.SetLogMode(true)
		}
		if err := server.Start(); err != nil {
			jslog.Log(jslog.LevelBasic, "%s", cc.Name+" - Error starting server: "+err.Error())
			os.Exit(-1)
		}
		jslog.Log(jslog.LevelBasic, "%s", cc.Name+" - New server listening on "+cc.PortName)
	}

	go engine.RunDequeueLoop()
	go engine.RunRealtimeStream()

	// main loop: keep MongoDB connection alive (blocks forever)
	engine.MaintainMongo()
}
