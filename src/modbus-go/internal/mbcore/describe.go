/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package mbcore

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// Function codes used by the drivers.
const (
	FCReadCoils              = 1
	FCReadDiscreteInputs     = 2
	FCReadHoldingRegisters   = 3
	FCReadInputRegisters     = 4
	FCWriteSingleCoil        = 5
	FCWriteSingleRegister    = 6
	FCWriteMultipleCoils     = 15
	FCWriteMultipleRegisters = 16
	FCMaskWriteRegister      = 22
	FCReadWriteMultipleRegs  = 23
)

// Exception codes.
const (
	ExIllegalFunction     = 0x01
	ExIllegalDataAddress  = 0x02
	ExIllegalDataValue    = 0x03
	ExServerDeviceFailure = 0x04
)

var fcNames = map[int]string{
	FCReadCoils:              "READ_COILS",
	FCReadDiscreteInputs:     "READ_DISCRETE_INPUTS",
	FCReadHoldingRegisters:   "READ_HOLDING_REGISTERS",
	FCReadInputRegisters:     "READ_INPUT_REGISTERS",
	FCWriteSingleCoil:        "WRITE_SINGLE_COIL",
	FCWriteSingleRegister:    "WRITE_SINGLE_REGISTER",
	FCWriteMultipleCoils:     "WRITE_MULTIPLE_COILS",
	FCWriteMultipleRegisters: "WRITE_MULTIPLE_REGISTERS",
	FCMaskWriteRegister:      "MASK_WRITE_REGISTER",
	FCReadWriteMultipleRegs:  "READ_WRITE_MULTIPLE_REGISTERS",
}

// ExceptionNames are the Modbus exception names.
var ExceptionNames = map[int]string{
	0x01: "ILLEGAL FUNCTION",
	0x02: "ILLEGAL DATA ADDRESS",
	0x03: "ILLEGAL DATA VALUE",
	0x04: "SERVER DEVICE FAILURE",
	0x05: "ACKNOWLEDGE",
	0x06: "SERVER DEVICE BUSY",
	0x08: "MEMORY PARITY ERROR",
	0x0a: "GATEWAY PATH UNAVAILABLE",
	0x0b: "GATEWAY TARGET DEVICE FAILED TO RESPOND",
}

// FCName names a function code; an exception response gets "(exception)".
func FCName(fc int) string {
	base := fc & 0x7f
	name, ok := fcNames[base]
	if !ok {
		name = fmt.Sprintf("FC%d", base)
	}
	if fc&0x80 != 0 {
		name += "(exception)"
	}
	return name
}

// ExceptionName renders "0x02 ILLEGAL DATA ADDRESS".
func ExceptionName(code int) string {
	name, ok := ExceptionNames[code]
	if !ok {
		name = "UNKNOWN"
	}
	return fmt.Sprintf("0x%02x %s", code, name)
}

// ExceptionText is the error text of an exception, as the Node.js driver
// reported it in lastError and in a command's resultDescription.
func ExceptionText(fc, code int) string {
	name, ok := ExceptionNames[code]
	if !ok {
		name = "UNKNOWN"
	}
	return fmt.Sprintf("Modbus exception %02x %s (FC %d)", code, name, fc&0x7f)
}

// Hex is a space-separated upper-case hex dump, capped at max bytes.
func Hex(b []byte, max int) string {
	if max <= 0 {
		max = 300
	}
	n := len(b)
	if n > max {
		n = max
	}
	var sb strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			sb.WriteByte(' ')
		}
		fmt.Fprintf(&sb, "%02X", b[i])
	}
	if len(b) > max {
		fmt.Fprintf(&sb, " ... (+%d bytes)", len(b)-max)
	}
	return sb.String()
}

func u16at(b []byte, i int) (int, bool) {
	if len(b) < i+2 {
		return 0, false
	}
	return int(binary.BigEndian.Uint16(b[i:])), true
}

// DescribeRequest is a one-line summary of a request PDU, e.g.
// "fc=3 READ_HOLDING_REGISTERS addr=10 qty=2".
func DescribeRequest(pdu []byte) string {
	if len(pdu) < 1 {
		return "empty pdu"
	}
	fc := int(pdu[0])
	head := fmt.Sprintf("fc=%d %s", fc, FCName(fc))
	a, ok1 := u16at(pdu, 1)
	q, ok2 := u16at(pdu, 3)
	switch fc {
	case FCReadCoils, FCReadDiscreteInputs, FCReadHoldingRegisters, FCReadInputRegisters:
		if ok1 && ok2 {
			return fmt.Sprintf("%s addr=%d qty=%d", head, a, q)
		}
	case FCWriteSingleCoil:
		if ok1 && ok2 {
			v := "OFF"
			if q == 0xff00 {
				v = "ON"
			}
			return fmt.Sprintf("%s addr=%d value=%s", head, a, v)
		}
	case FCWriteSingleRegister:
		if ok1 && ok2 {
			return fmt.Sprintf("%s addr=%d value=%d", head, a, q)
		}
	case FCWriteMultipleCoils, FCWriteMultipleRegisters:
		if ok1 && ok2 && len(pdu) > 5 {
			return fmt.Sprintf("%s addr=%d qty=%d bytes=%d", head, a, q, pdu[5])
		}
	case FCMaskWriteRegister:
		o, ok3 := u16at(pdu, 5)
		if ok1 && ok2 && ok3 {
			return fmt.Sprintf("%s addr=%d and=0x%x or=0x%x", head, a, q, o)
		}
	default:
		return head
	}
	return head + " (truncated)"
}

// DescribeResponse is a one-line summary of a response PDU.
func DescribeResponse(pdu []byte) string {
	if len(pdu) < 1 {
		return "empty pdu"
	}
	fc := int(pdu[0])
	if fc&0x80 != 0 {
		code := 0
		if len(pdu) > 1 {
			code = int(pdu[1])
		}
		name, ok := ExceptionNames[code]
		if !ok {
			name = "UNKNOWN"
		}
		return fmt.Sprintf("fc=%d %s code=0x%02x %s", fc&0x7f, FCName(fc), code, name)
	}
	head := fmt.Sprintf("fc=%d %s", fc, FCName(fc))
	switch fc {
	case FCReadCoils, FCReadDiscreteInputs, FCReadHoldingRegisters, FCReadInputRegisters:
		if len(pdu) > 1 {
			return fmt.Sprintf("%s bytes=%d", head, pdu[1])
		}
	case FCWriteSingleCoil, FCWriteSingleRegister, FCWriteMultipleCoils, FCWriteMultipleRegisters:
		a, ok1 := u16at(pdu, 1)
		e, ok2 := u16at(pdu, 3)
		if ok1 && ok2 {
			return fmt.Sprintf("%s addr=%d echo=%d", head, a, e)
		}
	default:
		return head
	}
	return head + " (truncated)"
}
