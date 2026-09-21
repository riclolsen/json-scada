/*
 * DNP3 Client and Server Protocol drivers for {json:scada}, in Go.
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

// protocolSourceCommandDuration to control relay output block. Port of the
// switch in executeCommand() of the C++ client.

package dnp3util

import dnp3 "github.com/dscsystems/go-dnp3"

// Pulse times, fixed in the C++ driver and kept fixed here.
const (
	CROBPulseOnTime  = 100
	CROBPulseOffTime = 100
)

// CROBFor builds the control relay output block for a command duration code
// and a command value.
//
// The duration codes are the ones documented in the driver README. Note that
// 10, 12, 20 and 22 appear in that table but were never implemented by the C++
// driver's switch: they fall through to an all-zero block, which operates
// nothing. Reproduced deliberately (quirk Q3) rather than guessed at, because a
// wrong guess here operates the wrong coil of a breaker.
func CROBFor(duration int, value float64) dnp3.ControlRelayOutputBlock {
	on := value != 0
	crob := dnp3.ControlRelayOutputBlock{Count: 1}

	pulse := func(onCode, offCode dnp3.ControlCode) {
		crob.OnTime = CROBPulseOnTime
		crob.OffTime = CROBPulseOffTime
		if on {
			crob.Code = onCode
		} else {
			crob.Code = offCode
		}
	}
	latch := func(onCode, offCode dnp3.ControlCode) {
		if on {
			crob.Code = onCode
		} else {
			crob.Code = offCode
		}
	}

	switch duration {
	case 1:
		pulse(dnp3.ControlPulseOn, dnp3.ControlPulseOff)
	case 2:
		pulse(dnp3.ControlPulseOff, dnp3.ControlPulseOn)
	case 3:
		latch(dnp3.ControlLatchOn, dnp3.ControlLatchOff)
	case 4:
		latch(dnp3.ControlLatchOff, dnp3.ControlLatchOn)
	case 11:
		pulse(dnp3.ControlPulseOn|dnp3.ControlClose, dnp3.ControlPulseOff|dnp3.ControlTrip)
	case 13:
		latch(dnp3.ControlLatchOn|dnp3.ControlClose, dnp3.ControlLatchOff|dnp3.ControlTrip)
	case 21:
		pulse(dnp3.ControlPulseOn|dnp3.ControlTrip, dnp3.ControlPulseOff|dnp3.ControlClose)
	case 23:
		latch(dnp3.ControlLatchOn|dnp3.ControlTrip, dnp3.ControlLatchOff|dnp3.ControlClose)
	default:
		// Including 0 and the documented-but-unimplemented 10, 12, 20, 22.
		crob.Code = dnp3.ControlNUL
	}
	return crob
}
