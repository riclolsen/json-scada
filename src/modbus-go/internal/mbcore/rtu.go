/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package mbcore

// RTU request framing for the server side. go-modbus serves Modbus TCP and TLS
// only, so the RTU server modes (Serial, RTU over TCP/TLS Passive) cut RTU
// frames here and hand the PDUs to the go-modbus server as MBAP requests.

var crcTable = func() [256]uint16 {
	var t [256]uint16
	for i := 0; i < 256; i++ {
		crc := uint16(i)
		for j := 0; j < 8; j++ {
			if crc&1 != 0 {
				crc = crc>>1 ^ 0xa001
			} else {
				crc >>= 1
			}
		}
		t[i] = crc
	}
	return t
}()

// CRC16 is the Modbus CRC-16 (polynomial 0xA001, reflected, init 0xFFFF).
func CRC16(b []byte) uint16 {
	crc := uint16(0xffff)
	for _, c := range b {
		crc = crc>>8 ^ crcTable[(crc^uint16(c))&0xff]
	}
	return crc
}

// AppendCRC appends the CRC low byte first.
func AppendCRC(b []byte) []byte {
	crc := CRC16(b)
	return append(b, byte(crc), byte(crc>>8))
}

// CheckCRC verifies the two trailing CRC bytes of a frame.
func CheckCRC(frame []byte) bool {
	if len(frame) < 3 {
		return false
	}
	n := len(frame) - 2
	return CRC16(frame[:n]) == uint16(frame[n])|uint16(frame[n+1])<<8
}

// EncodeRTU builds unit + PDU + CRC.
func EncodeRTU(unit byte, pdu []byte) []byte {
	b := make([]byte, 0, len(pdu)+3)
	b = append(b, unit)
	b = append(b, pdu...)
	return AppendCRC(b)
}

// RTUFrame is one decoded RTU frame.
type RTUFrame struct {
	Unit byte
	PDU  []byte
}

// RTURequestDecoder cuts RTU request frames out of a byte stream. A request's
// length is predictable from its function code and counted fields; anything
// else is delimited by the inter-frame idle gap (OnIdle).
type RTURequestDecoder struct {
	buf []byte
}

// Reset drops anything buffered.
func (d *RTURequestDecoder) Reset() { d.buf = d.buf[:0] }

// Buffered reports how many bytes wait for a frame boundary.
func (d *RTURequestDecoder) Buffered() int { return len(d.buf) }

// Push appends received bytes and returns every complete request.
func (d *RTURequestDecoder) Push(chunk []byte) []RTUFrame {
	d.buf = append(d.buf, chunk...)
	var out []RTUFrame
	for d.tryOne(&out) {
	}
	return out
}

// OnIdle treats whatever is buffered as one frame when its CRC checks out,
// and discards it otherwise.
func (d *RTURequestDecoder) OnIdle() []RTUFrame {
	var out []RTUFrame
	if len(d.buf) >= 4 && CheckCRC(d.buf) {
		out = append(out, RTUFrame{Unit: d.buf[0], PDU: append([]byte(nil), d.buf[1:len(d.buf)-2]...)})
	}
	d.buf = d.buf[:0]
	return out
}

func (d *RTURequestDecoder) tryOne(out *[]RTUFrame) bool {
	n := predictRequestLength(d.buf)
	if n < 0 || len(d.buf) < n {
		return false
	}
	frame := d.buf[:n]
	if CheckCRC(frame) {
		*out = append(*out, RTUFrame{Unit: frame[0], PDU: append([]byte(nil), frame[1:n-2]...)})
		d.buf = append(d.buf[:0], d.buf[n:]...)
		return true
	}
	// CRC failure: shift one byte and retry to resynchronize.
	d.buf = append(d.buf[:0], d.buf[1:]...)
	return len(d.buf) >= 4
}

// predictRequestLength is the total RTU request length (unit + PDU + CRC), or
// -1 when it cannot be known yet.
func predictRequestLength(b []byte) int {
	if len(b) < 2 {
		return -1
	}
	switch b[1] {
	case FCReadCoils, FCReadDiscreteInputs, FCReadHoldingRegisters, FCReadInputRegisters,
		FCWriteSingleCoil, FCWriteSingleRegister:
		return 1 + 5 + 2
	case FCMaskWriteRegister:
		return 1 + 7 + 2
	case FCWriteMultipleCoils, FCWriteMultipleRegisters:
		if len(b) < 7 {
			return -1
		}
		return 1 + 6 + int(b[6]) + 2
	}
	return -1
}

// RTUIdleGap is the default inter-frame gap: 3.5 character times at the baud
// rate (11 bits per character, worst case), at least 2 ms. override > 0 wins.
func RTUIdleGapMs(baudRate, override float64) float64 {
	if override > 0 {
		return override
	}
	if baudRate < 1 {
		baudRate = 1
	}
	ms := 3.5 * 11 * 1000 / baudRate
	c := float64(int(ms))
	if ms > c {
		c++
	}
	if c < 2 {
		c = 2
	}
	return c
}
