/*
 * DNP3 Outstation Server Protocol driver for {json:scada}, in Go.
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

// Device attributes, group 0: the outstation's answer to "what are you?".
//
// A master commissioning an unfamiliar panel reads these instead of trusting a
// drawing, and an engineer with a protocol analyser reads them to find out
// which of several identical-looking gateways they are talking to. The C++
// server answers none of it — opendnp3 has no group 0 support — so this is new
// (deviation D24).
//
// The library derives the point counts and the fragment sizes from the session
// it built, so those cannot drift from the database they describe and are not
// repeated here. What is left is identity, which only the application knows.

package serverapp

import (
	"runtime"
	"strconv"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/objects"
)

// The standard set 0 variations this driver reports. They are named here
// rather than taken from the library's display table because these are numbers
// used to answer a request, not labels: a wrong entry in a display table
// mislabels a row, a wrong entry here answers the wrong question.
const (
	attrSoftwareVersion uint8 = 242
	attrHardwareVersion uint8 = 243
	attrLocation        uint8 = 245
	attrIDCode          uint8 = 246
	attrDeviceName      uint8 = 247
	attrProductName     uint8 = 250
	attrManufacturer    uint8 = 252
)

// Identity reported by every connection of this driver.
const (
	manufacturerName = "{json:scada}"
	productName      = "JSON-SCADA DNP3 Outstation Server (Go)"
)

// deviceAttributes describes one connection to a master that asks.
//
// Deliberately absent:
//
//   - Subset level and conformance (249). Nothing in this repository has been
//     through certified conformance testing, and the library's own device
//     profile says as much. Answering it would be a claim, not a fact.
//   - Serial number (248). A gateway has no serial number to give, and
//     inventing one from a connection number invites somebody to key an asset
//     register off it.
//
// Both are better left unanswered than answered wrongly: a master reading an
// attribute the outstation does not report learns that it does not report it,
// which is true, whereas a plausible wrong value propagates.
func deviceAttributes(conn *Connection) []dnp3.Attribute {
	attrs := []dnp3.Attribute{
		objects.StringAttribute(attrManufacturer, manufacturerName),
		objects.StringAttribute(attrProductName, productName),
		objects.StringAttribute(attrSoftwareVersion, DriverVersion),
		// The host platform is the nearest honest thing to hardware for a
		// software outstation, and it is what an engineer wants when a
		// gateway misbehaves on one machine and not another.
		objects.StringAttribute(attrHardwareVersion, runtime.GOOS+"/"+runtime.GOARCH),
	}

	// The connection name is what the tag names of this driver are built from
	// and what every log line is prefixed with, so it is the name that
	// identifies this outstation everywhere else in the system.
	if conn.Name != "" {
		attrs = append(attrs, objects.StringAttribute(attrDeviceName, conn.Name))
	}
	// The description exists in protocolConnections and is documented as
	// purely documental; reporting it is how it reaches the field.
	if conn.Description != "" {
		attrs = append(attrs, objects.StringAttribute(attrLocation, conn.Description))
	}
	// The connection number is unique across every driver of an installation,
	// which makes it the one identifier that tells two otherwise identical
	// outstations apart.
	attrs = append(attrs, objects.StringAttribute(attrIDCode,
		strconv.Itoa(conn.ProtocolConnectionNumber)))

	return attrs
}
