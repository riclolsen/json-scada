/*
 * IEC 61850 MMS Server driver (IEC61850-90-2 gateway) for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
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

// Self test: build and serve a synthetic model without MongoDB, so the
// model can be browsed with any IEC 61850 client.
//
//	iec61850-server selftest [port [bulkPoints]]

package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
)

func runSelfTest(args []string) {
	jslog.SetLevel(jslog.LevelDetailed)
	port := 10102
	if len(args) > 2 {
		if p, err := strconv.Atoi(args[2]); err == nil {
			port = p
		}
	}
	bulk := 0
	if len(args) > 3 {
		if n, err := strconv.Atoi(args[3]); err == nil {
			bulk = n
		}
	}

	jslog.Log(jslog.LevelNoLog, "=== IEC61850_SERVER SELF TEST (no MongoDB) ===")

	conn := &ServerConnection{
		ProtocolDriver:               ProtocolDriverName,
		ProtocolDriverInstanceNumber: 1,
		ProtocolConnectionNumber:     8001,
		Name:                         "IEC61850SRV",
		Description:                  "self test",
		Enabled:                      true,
		CommandsEnabled:              true,
		IPAddressLocalBind:           "0.0.0.0:" + strconv.Itoa(port),
		ServerModeMultiActive:        true,
		MaxClientConnections:         2,
		MaxQueueSize:                 1000,
	}

	points := syntheticPoints()
	if bulk > 0 {
		points = append(points, bulkPoints(bulk)...)
		jslog.Log(jslog.LevelNoLog, "Bulk points added: %d", bulk)
	}
	jslog.Log(jslog.LevelNoLog, "Synthetic points: %d", len(points))

	built := BuildModel(points, conn)
	exportManifest(built, conn)

	gw, err := NewGateway(conn, built)
	if err != nil {
		jslog.Log(jslog.LevelNoLog, "SELF TEST: server FAILED to start: %v", err)
		jslog.Flush()
		os.Exit(-1)
	}
	installControlHandlers(gw)

	gw.Start()
	if !gw.Serving() {
		jslog.Log(jslog.LevelNoLog, "SELF TEST: server FAILED to start.")
		jslog.Flush()
		os.Exit(-1)
	}
	jslog.Log(jslog.LevelNoLog, "SELF TEST: server RUNNING on %s - browse it with an IEC 61850 client.", gw.Addr())

	applyInitialValues(gw, points)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go updateLoop(ctx, gw)

	// Drive a few updates through the normal path.
	for tick := 0; tick < 5; tick++ {
		for _, p := range points {
			mp := built.ByTag[p.Tag]
			if mp == nil || mp.IsCommand {
				continue
			}
			switch mp.Kind {
			case KindMV:
				p.Value = 100 * math.Sin(float64(tick)/3)
			case KindSPS:
				p.Value = float64(tick % 2)
			case KindVSS:
				p.ValueString = fmt.Sprintf("tick %d", tick)
			}
			p.Invalid = false
			p.HasTimeTagAtSource = true
			p.TimeTagAtSource = time.Now().UTC()
			p.TimeTagAtSourceOk = true
			enqueueUpdate(updateFromPoint(mp, p))
		}
		time.Sleep(300 * time.Millisecond)
	}
	jslog.Log(jslog.LevelNoLog, "SELF TEST: updates applied without error. Server stays up until interrupted...")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)
	<-sigs
	gw.Stop()
	jslog.Log(jslog.LevelNoLog, "SELF TEST: done.")
	jslog.Flush()
}

// syntheticPoints covers every mapped class, plus a command of each kind.
func syntheticPoints() []*Point {
	mk := func(id float64, tag, typ, origin, group1, desc string) *Point {
		return &Point{
			ID: id, Tag: tag, Type: typ, Origin: origin, Group1: group1,
			Description: desc, Invalid: true,
			Kconv1: 1, Kconv2: 0,
			SrcConnectionNumber: 91, SrcCommonAddress: "1", SrcObjectAddress: tag + "_addr",
			SrcASDU: "", SrcCommandDuration: 0,
		}
	}
	pts := []*Point{
		mk(1, "SELFTEST_DIGITAL_1", "digital", "supervised", "KAW2", "breaker position"),
		mk(2, "SELFTEST_DIGITAL_2", "digital", "supervised", "KAW2", "disconnector position"),
		mk(3, "SELFTEST_ANALOG_1", "analog", "supervised", "KAW2", "active power"),
		mk(4, "SELFTEST_ANALOG_2", "analog", "supervised", "KAW2", "voltage"),
		mk(5, "SELFTEST_STRING_1", "string", "supervised", "KAW2", "device status text"),
		mk(6, "SELFTEST_CMD_DIGITAL", "digital", "command", "KAW2", "breaker command"),
		mk(7, "SELFTEST_CMD_ANALOG", "analog", "command", "KAW2", "setpoint command"),
		mk(8, "SELFTEST_OTHER_TOPIC", "digital", "supervised", "KIK3", "other topic point"),
	}
	pts[6].SrcCommandUseSBO = true // exercise select-before-operate
	return pts
}

// bulkPoints generates points in one topic to exercise the layout at
// realistic scale.
func bulkPoints(n int) []*Point {
	out := make([]*Point, 0, n)
	for i := 1; i <= n; i++ {
		typ := "digital"
		if i%3 == 0 {
			typ = "analog"
		}
		out = append(out, &Point{
			ID: float64(1000 + i), Tag: fmt.Sprintf("SELFTEST_BULK_%05d", i),
			Type: typ, Origin: "supervised", Group1: "BULK",
			Description: fmt.Sprintf("bulk point %d with a deliberately long description to exercise truncation", i),
			Invalid:     true, Kconv1: 1, SrcConnectionNumber: 91,
			SrcCommonAddress: "1", SrcObjectAddress: fmt.Sprintf("bulk_%d", i), SrcASDU: "",
		})
	}
	return out
}
