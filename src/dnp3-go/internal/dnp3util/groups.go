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

// DNP3 object group tables shared by both drivers.

package dnp3util

import "strconv"

// The common addresses JSON-SCADA uses to name a DNP3 object family. These are
// the values that go in protocolSourceCommonAddress and
// protocolDestinationCommonAddress.
const (
	GroupBinaryInput        = 1
	GroupDoubleBinaryInput  = 3
	GroupBinaryOutputStatus = 10
	GroupCROBCommand        = 12
	GroupCounter            = 20
	GroupFrozenCounter      = 21
	GroupFrozenCounterEvent = 23
	GroupAnalogInput        = 30
	GroupAnalogOutputStatus = 40
	GroupAnalogOutputBlock  = 41
	GroupOctetString        = 110
	GroupTimeAndInterval    = 50
)

// TypeFromBaseGroup classifies a group as a JSON-SCADA tag type. Port of
// dnp3TypeFromBaseGroup().
func TypeFromBaseGroup(g int) string {
	switch g {
	case GroupBinaryInput, GroupDoubleBinaryInput, GroupBinaryOutputStatus, GroupCROBCommand:
		return "digital"
	default:
		return "analog"
	}
}

// GroupDescription is the human label used in auto-created tag names and
// descriptions. Port of dnp3GroupDescription().
func GroupDescription(g int) string {
	switch g {
	case GroupBinaryInput:
		return "Binary Input"
	case GroupDoubleBinaryInput:
		return "Double Binary Input"
	case GroupBinaryOutputStatus:
		return "Binary Output Status"
	case GroupCounter:
		return "Counter"
	case GroupFrozenCounterEvent:
		return "Frozen Counter"
	case GroupAnalogInput:
		return "Analog Input"
	case GroupAnalogOutputStatus:
		return "Analog Output Status"
	case GroupCROBCommand:
		return "CROB Command"
	case GroupAnalogOutputBlock:
		return "Analog Output Command"
	default:
		return "Group " + strconv.Itoa(g)
	}
}
