/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

// Package clientapp is the MODBUS client (master) driver: it polls devices,
// writes sourceDataUpdate into realtimeData and dispatches commandsQueue
// inserts as Modbus writes, using go-modbus for the protocol.
package clientapp

import (
	"fmt"
	"math"

	"modbus-go/internal/drv"
	"modbus-go/internal/mbcore"
	"modbus-go/internal/mbnet"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Connection modes.
const (
	ModeTCP        = "TCP Active"
	ModeTLS        = "TLS Active"
	ModeSerial     = "Serial"
	ModeRTUOverTCP = "RTU over TCP"
	ModeRTUOverTLS = "RTU over TLS"
)

// Config is one normalized MODBUS protocolConnections document. Every key and
// default is the Node.js driver's (src/modbus/src/client/conn-config.ts).
type Config struct {
	ProtocolConnectionNumber float64
	Name                     string
	Enabled                  bool
	CommandsEnabled          bool

	ConnectionMode     string
	IPAddresses        []string
	IPAddressLocalBind string
	PortName           string
	BaudRate           float64
	Parity             string
	StopBits           string
	Handshake          string

	TimeoutMs           float64
	MaxRetries          float64
	PollingIntervalMs   float64
	GiIntervalS         float64
	InterRequestDelayMs float64
	InterFrameDelayMs   float64

	MaxReadRegisters float64
	MaxReadCoils     float64
	MaxAddressGap    float64

	ByteOrder16         string
	ByteOrder32         string
	ByteOrder64         string
	ByteOrderStr        string
	StringEncoding      string
	UseModiconAddresses bool
	UseMaskWrite        bool

	TLS mbnet.TLSOptions

	AutoCreateTags bool
	Topics         []string

	KeepProtocolRunningWhileInactive bool
}

// Normalize reads a protocolConnections document permissively.
func Normalize(doc bson.M) Config {
	connNum := drv.Num(doc, "protocolConnectionNumber", 0)
	return Config{
		ProtocolConnectionNumber: connNum,
		Name:                     drv.Str(doc, "name", "MODBUS-"+mbcore.JSNumberString(connNum)),
		Enabled:                  drv.Bool(doc, "enabled", true),
		CommandsEnabled:          drv.Bool(doc, "commandsEnabled", true),

		ConnectionMode:     drv.Str(doc, "connectionMode", ModeTCP),
		IPAddresses:        drv.StrArr(doc, "ipAddresses"),
		IPAddressLocalBind: drv.Str(doc, "ipAddressLocalBind", ""),
		PortName:           drv.Str(doc, "portName", ""),
		BaudRate:           drv.Num(doc, "baudRate", 9600),
		Parity:             drv.Str(doc, "parity", "Even"),
		StopBits:           drv.Str(doc, "stopBits", "One"),
		Handshake:          drv.Str(doc, "handshake", "None"),

		TimeoutMs:           drv.Num(doc, "timeoutMs", 1000),
		MaxRetries:          drv.Num(doc, "maxRetries", 2),
		PollingIntervalMs:   drv.Num(doc, "pollingInterval", 1000),
		GiIntervalS:         drv.Num(doc, "giInterval", 300),
		InterRequestDelayMs: drv.Num(doc, "interRequestDelayMs", 0),
		InterFrameDelayMs:   drv.Num(doc, "interFrameDelayMs", 0),

		MaxReadRegisters: math.Min(125, drv.Num(doc, "maxReadRegisters", 125)),
		MaxReadCoils:     math.Min(2000, drv.Num(doc, "maxReadCoils", 2000)),
		MaxAddressGap:    drv.Num(doc, "maxAddressGap", 8),

		ByteOrder16:         drv.Str(doc, "byteOrder16", "AB"),
		ByteOrder32:         drv.Str(doc, "byteOrder32", "ABCD"),
		ByteOrder64:         drv.Str(doc, "byteOrder64", "ABCDEFGH"),
		ByteOrderStr:        drv.Str(doc, "byteOrderStr", "AB"),
		StringEncoding:      drv.Str(doc, "stringEncoding", "latin1"),
		UseModiconAddresses: drv.Bool(doc, "useModiconAddresses", false),
		UseMaskWrite:        drv.Bool(doc, "useMaskWrite", true),

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

		AutoCreateTags: drv.Bool(doc, "autoCreateTags", false),
		Topics:         drv.StrArr(doc, "topics"),

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

// IsTLS reports a TLS connection mode.
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

// Validate reports a connection mode the driver does not know.
func (c Config) Validate() error {
	switch c.ConnectionMode {
	case ModeTCP, ModeTLS, ModeSerial, ModeRTUOverTCP, ModeRTUOverTLS:
		return nil
	}
	return fmt.Errorf("unknown connectionMode %q (expected %q, %q, %q, %q or %q)",
		c.ConnectionMode, ModeTCP, ModeTLS, ModeSerial, ModeRTUOverTCP, ModeRTUOverTLS)
}
