/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package mbnet

import (
	"io"
	"strings"
	"time"

	"github.com/otfabric/go-serial"
)

// SerialOptions are the serial fields of a protocolConnections document, with
// the AdminUI enumerations (parity None/Even/Odd/Mark/Space, stopBits
// One/One5/Two, handshake None/Rts/Xon/RtsXon).
type SerialOptions struct {
	PortName  string
	BaudRate  int
	Parity    string
	StopBits  string
	Handshake string
}

// Parity maps the parity name to N/E/O. Mark and space parity are not
// supported by go-serial; they fall back to none and are reported in warn.
func Parity(s string) (p string, warn string) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "even":
		return "E", ""
	case "odd":
		return "O", ""
	case "mark", "space":
		return "N", "parity " + s + " is not supported, using None"
	}
	return "N", ""
}

// StopBits maps One/One5/Two to 1 or 2. Modbus RTU uses whole stop bits, so
// One5 becomes 2, as in the Node.js driver.
func StopBits(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "two", "one5":
		return 2
	}
	return 1
}

// HandshakeWarning reports a flow control setting that is ignored: neither
// go-modbus nor go-serial does RTS/CTS or XON/XOFF flow control.
func HandshakeWarning(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "none":
		return ""
	}
	return "handshake " + s + " is not supported, flow control is off"
}

// OpenSerial opens a port with 8 data bits and the given read timeout.
func OpenSerial(o SerialOptions, readTimeout time.Duration) (io.ReadWriteCloser, error) {
	p, _ := Parity(o.Parity)
	sb := serial.StopBits1
	if StopBits(o.StopBits) == 2 {
		sb = serial.StopBits2
	}
	return serial.Open(&serial.Config{
		Address:  o.PortName,
		BaudRate: serial.BaudRate(o.BaudRate),
		DataBits: serial.DataBits8,
		StopBits: sb,
		Parity:   serial.Parity(p),
		Timeout:  readTimeout,
	})
}
