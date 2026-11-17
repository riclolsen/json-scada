/*
 * IEC 61850 MMS Client driver for {json:scada}, in Go.
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

// Automatic tag creation. Port of TagsCreation.cs, with one deliberate
// difference: tags are named and grouped by the connection and the IEC 61850
// object hierarchy (group1 connection, group2 logical device, group3 logical
// node) instead of under a fixed "IEC61850" group, so they sort and filter
// like the device they come from. The C# driver named them
// "IEC61850;<connection>;..." with group1 "IEC61850", group2 the connection
// and group3 the functional constraint.

package main

import (
	"maps"
	"strings"

	"github.com/riclolsen/json-scada/src/go-common/jstags"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// TagFromParameters builds the tag name of an automatically created point:
// "<connection>;<object reference>[<FC>]".
func TagFromParameters(iv IECValue) string {
	return iv.ConnName + ";" + iv.Address + "[" + iv.CommonAddress + "]"
}

// pointKey identifies a point by object reference and functional
// constraint, the same identity entryKey gives a configured point. Tags are
// matched on it, never on their name, so a point keeps one tag whatever
// naming scheme created it.
func pointKey(ref, fc string) string {
	return strings.TrimSpace(ref) + strings.ToUpper(strings.TrimSpace(fc))
}

// splitRef splits an object reference "LD/LN.DO.DA" into the logical device,
// the logical node and the rest ("DO.DA"): group2, group3 and the ungrouped
// description of the tag. Whatever cannot be split stays in rest.
func splitRef(ref string) (ld, ln, rest string) {
	ld, path, ok := strings.Cut(ref, "/")
	if !ok {
		return "", "", ref
	}
	ln, rest, ok = strings.Cut(path, ".")
	if !ok {
		return ld, "", path
	}
	return ld, ln, rest
}

// CommandTag describes a controllable object found while browsing, waiting
// for its tag to be created.
type CommandTag struct {
	ConnNumber int
	ConnName   string
	Ref        string // IEC 61850 object reference of the control object
	IsDigital  bool
	UseSBO     bool
	Asdu       string // MMS type of the control value
	Attempts   int    // failed insert attempts
}

// Tag is the name of the command tag.
func (c CommandTag) Tag() string {
	return c.ConnName + ";" + c.Ref + "[CO]"
}

// newCommandDoc builds the realtimeData document of a command point.
// supervisedID is the key of the point where the command's effect shows,
// zero when the device exposes no status for the controllable object.
func newCommandDoc(ct CommandTag, id, supervisedID float64) bson.M {
	ld, ln, rest := splitRef(ct.Ref)

	doc := jstags.BaseDoc()
	maps.Copy(doc, bson.M{
		"_id":                            id,
		"protocolSourceASDU":             ct.Asdu,
		"protocolSourceCommonAddress":    "CO",
		"protocolSourceConnectionNumber": float64(ct.ConnNumber),
		"protocolSourceObjectAddress":    ct.Ref,
		"protocolSourceCommandUseSBO":    ct.UseSBO,
		"protocolSourceCommandDuration":  0.0,
		"description":                    ct.ConnName + "~" + ld + "~" + ln + "~" + rest + " command",
		"ungroupedDescription":           rest + " command",
		"group1":                         ct.ConnName,
		"group2":                         ld,
		"group3":                         ln,
		"origin":                         "command",
		"tag":                            ct.Tag(),
		// The two ends of the pair: this command acts on that supervised
		// point, which is where its feedback appears.
		"supervisedOfCommand":  supervisedID,
		"commandOfSupervised":  0.0,
		"invalid":              false,
		"invalidDetectTimeout": 0.0,
		"protocolDestinations": nil,
		"value":                0.0,
		"valueString":          "",
	})

	if ct.IsDigital {
		doc["type"] = "digital"
		doc["alarmState"] = -1.0
		doc["stateTextFalse"] = "FALSE"
		doc["stateTextTrue"] = "TRUE"
		doc["eventTextFalse"] = "FALSE"
		doc["eventTextTrue"] = "TRUE"
	} else {
		doc["type"] = "analog"
		doc["alarmState"] = -1.0
		doc["stateTextFalse"] = ""
		doc["stateTextTrue"] = ""
		doc["eventTextFalse"] = ""
		doc["eventTextTrue"] = ""
	}
	return doc
}

// newRealtimeDoc builds the realtimeData document for a discovered point.
func newRealtimeDoc(iv IECValue, id float64) bson.M {
	// The groups come from the object reference, as the tag name does. A
	// display name of its own, distinct from the reference, is kept as the
	// ungrouped description.
	ld, ln, rest := splitRef(iv.Address)
	if iv.DisplayName != "" && iv.DisplayName != iv.Address {
		rest = iv.DisplayName
	}

	doc := jstags.BaseDoc()
	maps.Copy(doc, bson.M{
		"_id":                            id,
		"protocolSourceASDU":             iv.Asdu,
		"protocolSourceCommonAddress":    strings.ToUpper(iv.CommonAddress),
		"protocolSourceConnectionNumber": float64(iv.ConnNumber),
		"protocolSourceObjectAddress":    iv.Address,
		"protocolSourceCommandUseSBO":    false,
		"protocolSourceCommandDuration":  0.0,
		"description":                    iv.ConnName + "~" + ld + "~" + ln + "~" + rest,
		"ungroupedDescription":           rest,
		"group1":                         iv.ConnName,
		"group2":                         ld,
		"group3":                         ln,
		"origin":                         "supervised",
		"tag":                            TagFromParameters(iv),
		"commandOfSupervised":            0.0,
		"invalid":                        true,
		"invalidDetectTimeout":           60000.0,
		"protocolDestinations":           nil,
		"supervisedOfCommand":            0.0,
	})

	switch {
	case strings.EqualFold(iv.Asdu, "boolean") || iv.IsDigital:
		doc["alarmState"] = 2.0
		doc["stateTextFalse"] = "FALSE"
		doc["stateTextTrue"] = "TRUE"
		doc["eventTextFalse"] = "FALSE"
		doc["eventTextTrue"] = "TRUE"
		doc["type"] = "digital"
		doc["value"] = iv.Value
		doc["valueString"] = "????"
	case strings.EqualFold(iv.Asdu, "string") || strings.EqualFold(iv.Asdu, "extensionobject"):
		doc["alarmState"] = -1.0
		doc["stateTextFalse"] = ""
		doc["stateTextTrue"] = ""
		doc["eventTextFalse"] = ""
		doc["eventTextTrue"] = ""
		doc["type"] = "string"
		doc["value"] = 0.0
		doc["valueString"] = iv.ValueString
	default:
		doc["alarmState"] = -1.0
		doc["stateTextFalse"] = ""
		doc["stateTextTrue"] = ""
		doc["eventTextFalse"] = ""
		doc["eventTextTrue"] = ""
		doc["type"] = "analog"
		doc["value"] = iv.Value
		doc["valueString"] = "????"
	}

	return doc
}
