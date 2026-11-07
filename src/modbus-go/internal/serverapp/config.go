/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

// Package serverapp is the MODBUS_SERVER driver: it serves realtimeData
// values to Modbus masters and relays their writes as JSON-SCADA commands.
package serverapp

// Parsing and normalization of a MODBUS_SERVER protocolConnections document.
// Ported from src/modbus/src/server/conn-config.ts: same keys, same defaults.

import (
	"fmt"
	"math"

	"modbus-go/internal/drv"
	"modbus-go/internal/mbcore"
	"modbus-go/internal/mbnet"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// The connection modes.
const (
	ModeTCP        = "TCP Passive"
	ModeTLS        = "TLS Passive"
	ModeSerial     = "Serial"
	ModeRTUOverTCP = "RTU over TCP Passive"
	ModeRTUOverTLS = "RTU over TLS Passive"
)

// Config is one normalized MODBUS_SERVER connection.
type Config struct {
	ProtocolConnectionNumber float64
	Name                     string
	Enabled                  bool
	CommandsEnabled          bool

	ConnectionMode       string
	IPAddressLocalBind   string
	MaxClientConnections float64
	ClientIdleTimeoutMs  float64
	IPAddresses          []string // client allow-list, empty = any

	PortName          string
	BaudRate          float64
	Parity            string
	StopBits          string
	Handshake         string
	InterFrameDelayMs float64

	ServerUnitIDs       []int
	StrictUnitID        bool
	ServeUnmappedAsZero bool

	ByteOrder16             string
	ByteOrder32             string
	ByteOrder64             string
	ByteOrderStr            string
	StringEncoding          string
	UseModiconAddresses     bool
	InvalidValuePolicy      string // last | zero
	AllowWritesToSupervised bool

	TLS mbnet.TLSOptions

	KeepProtocolRunningWhileInactive bool
}

// numArr is a list of rounded finite numbers, or def when empty or not a list.
func numArr(doc bson.M, key string, def []int) []int {
	var items []any
	switch a := doc[key].(type) {
	case bson.A:
		items = a
	case []any:
		items = a
	default:
		return def
	}
	var out []int
	for _, x := range items {
		f := mbcore.JSNumberFromString(drv.AnyString(x))
		if _, isBool := x.(bool); isBool {
			f = 0
			if x.(bool) {
				f = 1
			}
		} else if x == nil {
			f = 0
		}
		if math.IsNaN(f) || math.IsInf(f, 0) {
			continue
		}
		out = append(out, int(mbcore.JSRound(f)))
	}
	if len(out) == 0 {
		return def
	}
	return out
}

// Normalize reads a protocolConnections document permissively.
func Normalize(doc bson.M) Config {
	connNum := drv.Num(doc, "protocolConnectionNumber", 0)
	return Config{
		ProtocolConnectionNumber: connNum,
		Name:                     drv.Str(doc, "name", "MODBUS_SERVER-"+mbcore.JSNumberString(connNum)),
		Enabled:                  drv.Bool(doc, "enabled", true),
		CommandsEnabled:          drv.Bool(doc, "commandsEnabled", true),

		ConnectionMode:       drv.Str(doc, "connectionMode", ModeTCP),
		IPAddressLocalBind:   drv.Str(doc, "ipAddressLocalBind", "0.0.0.0:502"),
		MaxClientConnections: drv.Num(doc, "maxClientConnections", 8),
		ClientIdleTimeoutMs:  drv.Num(doc, "clientIdleTimeoutMs", 60000),
		IPAddresses:          drv.StrArr(doc, "ipAddresses"),

		PortName:          drv.Str(doc, "portName", ""),
		BaudRate:          drv.Num(doc, "baudRate", 9600),
		Parity:            drv.Str(doc, "parity", "Even"),
		StopBits:          drv.Str(doc, "stopBits", "One"),
		Handshake:         drv.Str(doc, "handshake", "None"),
		InterFrameDelayMs: drv.Num(doc, "interFrameDelayMs", 0),

		ServerUnitIDs:       numArr(doc, "serverUnitIds", []int{1}),
		StrictUnitID:        drv.Bool(doc, "strictUnitId", false),
		ServeUnmappedAsZero: drv.Bool(doc, "serveUnmappedAsZero", false),

		ByteOrder16:             drv.Str(doc, "byteOrder16", "AB"),
		ByteOrder32:             drv.Str(doc, "byteOrder32", "ABCD"),
		ByteOrder64:             drv.Str(doc, "byteOrder64", "ABCDEFGH"),
		ByteOrderStr:            drv.Str(doc, "byteOrderStr", "AB"),
		StringEncoding:          drv.Str(doc, "stringEncoding", "latin1"),
		UseModiconAddresses:     drv.Bool(doc, "useModiconAddresses", false),
		InvalidValuePolicy:      drv.Str(doc, "invalidValuePolicy", "last"),
		AllowWritesToSupervised: drv.Bool(doc, "allowWritesToSupervised", false),

		TLS: mbnet.TLSOptions{
			LocalCertFilePath:             drv.Str(doc, "localCertFilePath", ""),
			PrivateKeyFilePath:            drv.Str(doc, "privateKeyFilePath", ""),
			PrivateKeyPassphrase:          drv.Str(doc, "privateKeyPassphrase", ""),
			PeerCertFilePath:              drv.Str(doc, "peerCertFilePath", ""),
			ChainValidation:               drv.Bool(doc, "chainValidation", true),
			AllowOnlySpecificCertificates: drv.Bool(doc, "allowOnlySpecificCertificates", false),
			CipherList:                    drv.Str(doc, "cipherList", ""),
			AllowTLSv12:                   drv.Bool(doc, "allowTLSv12", true),
			AllowTLSv13:                   drv.Bool(doc, "allowTLSv13", true),
		},

		KeepProtocolRunningWhileInactive: drv.Bool(doc, "keepProtocolRunningWhileInactive", false),
	}
}

// Defaults are the connection-level ASDU defaults.
func (c Config) Defaults() mbcore.AsduDefaults {
	return mbcore.AsduDefaults{
		ByteOrder16: c.ByteOrder16, ByteOrder32: c.ByteOrder32, ByteOrder64: c.ByteOrder64,
		ByteOrderStr: c.ByteOrderStr, StringEncoding: c.StringEncoding,
	}
}

// IsTLS reports a TLS listening mode.
func (c Config) IsTLS() bool {
	return c.ConnectionMode == ModeTLS || c.ConnectionMode == ModeRTUOverTLS
}

// IsSerial reports the serial mode.
func (c Config) IsSerial() bool { return c.ConnectionMode == ModeSerial }

// IsRTU reports RTU framing.
func (c Config) IsRTU() bool {
	return c.ConnectionMode == ModeSerial || c.ConnectionMode == ModeRTUOverTCP || c.ConnectionMode == ModeRTUOverTLS
}

// DefaultPort is 802 for TLS modes and 502 otherwise.
func (c Config) DefaultPort() int {
	if c.IsTLS() {
		return 802
	}
	return 502
}

// Validate reports an unknown connection mode; the driver then serves plain TCP,
// as the Node.js driver did.
func (c Config) Validate() error {
	switch c.ConnectionMode {
	case ModeTCP, ModeTLS, ModeSerial, ModeRTUOverTCP, ModeRTUOverTLS:
		return nil
	}
	return fmt.Errorf("unknown connectionMode %q (expected %q, %q, %q, %q or %q)",
		c.ConnectionMode, ModeTCP, ModeTLS, ModeSerial, ModeRTUOverTCP, ModeRTUOverTLS)
}
