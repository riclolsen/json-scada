/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - cs101 config map
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Maps the protocolConnections serial/link fields (portName, baudRate,
 * parity, stopBits, timeoutForACK, ...) to a cs101.Config.
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

package cs101util

import (
	"strings"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/go-iecp5/cs101"
	"go.bug.st/serial"

	"iec60870-5/internal/model"
	"iec60870-5/internal/tlsutil"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
)

func msToSeconds(ms float64, def, min, max int) time.Duration {
	s := def
	if ms > 0 {
		s = int((ms + 999) / 1000) // round up
	}
	if s < min {
		s = min
	}
	if s > max {
		s = max
	}
	return time.Duration(s) * time.Second
}

// BuildConfig maps a protocolConnections document to a cs101.Config.
// isServer selects the local (server) or remote (client) link address.
func BuildConfig(cc *model.ConnCfg, isServer bool) (cs101.Config, error) {
	cfg := cs101.DefaultConfig()

	portName := cc.PortName
	if portName == "" {
		portName = "COM1" // C# BsonDefaultValue parity
	}
	cfg.Mode = cs101.ModeUnbalanced

	// portName "host:port" is TCP-encapsulated FT1.2 (the C# driver used a
	// TcpClientVirtualSerialPort for this). go-iecp5 carries the frames over
	// a TCP client connection to the terminal/serial-device server.
	if strings.Contains(portName, ":") {
		cfg.Transport = cs101.TransportTCPClient
		cfg.TCP = cs101.TCPConfig{
			Address:        portName,
			ConnectTimeout: time.Duration(defInt(int(cc.T0), 30)) * time.Second,
		}
		// optional TLS on the TCP transport when a local certificate is set
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
		parity := serial.EvenParity // Even is the standard parity for 101
		switch strings.ToLower(cc.Parity) {
		case "none":
			parity = serial.NoParity
		case "odd":
			parity = serial.OddParity
		case "mark":
			parity = serial.MarkParity
		case "space":
			parity = serial.SpaceParity
		}
		stopBits := serial.OneStopBit
		switch strings.ToLower(cc.StopBits) {
		case "one5", "onepointfive":
			stopBits = serial.OnePointFiveStopBits
		case "two":
			stopBits = serial.TwoStopBits
		}
		if h := strings.ToLower(cc.Handshake); h != "" && h != "none" {
			jslog.Log(jslog.LevelBasic, "%s", cc.Name+" - Warning: serial handshake '"+cc.Handshake+"' not supported, using none.")
		}
		cfg.Serial = cs101.SerialConfig{
			Address:  portName,
			BaudRate: baud,
			DataBits: 8,
			StopBits: stopBits,
			Parity:   parity,
			Timeout:  5 * time.Second,
		}
	}

	if isServer {
		cfg.LinkAddress = uint16(defInt(int(cc.LocalLinkAddress), 1))
	} else {
		cfg.LinkAddress = uint16(defInt(int(cc.RemoteLinkAddress), 1))
	}
	cfg.LinkAddrSize = byte(defInt(int(cc.SizeOfLinkAddress), 1))

	// timeoutForACK/timeoutRepeat are configured in ms (C# link layer);
	// go-iecp5 uses whole seconds with T2 < T1
	cfg.TimeoutResponseT1 = msToSeconds(cc.TimeoutForACK, 2, 2, 255)
	cfg.TimeoutRepeatT2 = msToSeconds(cc.TimeoutRepeat, 1, 1, 254)
	if cfg.TimeoutRepeatT2 >= cfg.TimeoutResponseT1 {
		cfg.TimeoutRepeatT2 = cfg.TimeoutResponseT1 - time.Second
	}
	if q := int(cc.MaxQueueSize); q > 0 {
		cfg.MaxSendQueueSize = q
	}
	// timeoutMessage/timeoutCharacter have no direct equivalent; the poll
	// pacing (TimeoutSendLinkMsg) is the closest knob
	if cc.TimeoutMessage > 0 {
		cfg.TimeoutSendLinkMsg = time.Duration(cc.TimeoutMessage) * time.Millisecond
	}
	if cc.UseSingleCharACK {
		jslog.Log(jslog.LevelBasic, "%s", cc.Name+" - Note: useSingleCharACK accepted on receive; sending E5 not configurable.")
	}
	return cfg, nil
}

// BuildParams maps the ASDU size fields with the IEC 101 defaults.
func BuildParams(cc *model.ConnCfg) asdu.Params {
	return asdu.Params{
		CauseSize:       defInt(int(cc.SizeOfCOT), 1),
		CommonAddrSize:  defInt(int(cc.SizeOfCA), 1),
		InfoObjAddrSize: defInt(int(cc.SizeOfIOA), 2),
		OrigAddress:     asdu.OriginAddr(cc.LocalLinkAddress),
		InfoObjTimeZone: time.Local,
	}
}

func defInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
