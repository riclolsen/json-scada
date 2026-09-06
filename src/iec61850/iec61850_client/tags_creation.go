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

// Automatic tag creation. Port of TagsCreation.cs: the documents inserted
// here must match what the C# driver inserts, field for field, so that a
// database seeded by one driver works with the other.

package main

import (
	"maps"
	"strings"

	"github.com/riclolsen/json-scada/src/go-common/jstags"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// TagFromParameters builds the tag name of an automatically created point.
func TagFromParameters(iv IECValue) string {
	return "IEC61850;" + iv.ConnName + ";" + iv.Address + "[" + iv.CommonAddress + "]"
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
	Attempts   int    // times the supervised twin was looked for
}

// Tag is the name of the command tag.
func (c CommandTag) Tag() string {
	return "IEC61850;" + c.ConnName + ";" + c.Ref + "[CO]"
}

// newCommandDoc builds the realtimeData document of a command point.
// supervisedID is the key of the point where the command's effect shows,
// zero when the device exposes no status for the controllable object.
func newCommandDoc(ct CommandTag, id, supervisedID float64) bson.M {
	const group1 = "IEC61850"

	doc := jstags.BaseDoc()
	maps.Copy(doc, bson.M{
		"_id":                            id,
		"protocolSourceASDU":             ct.Asdu,
		"protocolSourceCommonAddress":    "CO",
		"protocolSourceConnectionNumber": float64(ct.ConnNumber),
		"protocolSourceObjectAddress":    ct.Ref,
		"protocolSourceCommandUseSBO":    ct.UseSBO,
		"protocolSourceCommandDuration":  0.0,
		"description":                    group1 + "~" + ct.ConnName + "~" + ct.Ref + " command",
		"ungroupedDescription":           ct.Ref + " command",
		"group1":                         group1,
		"group2":                         ct.ConnName,
		"group3":                         "CO",
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
	const group1 = "IEC61850"

	doc := jstags.BaseDoc()
	maps.Copy(doc, bson.M{
		"_id":                            id,
		"protocolSourceASDU":             iv.Asdu,
		"protocolSourceCommonAddress":    strings.ToUpper(iv.CommonAddress),
		"protocolSourceConnectionNumber": float64(iv.ConnNumber),
		"protocolSourceObjectAddress":    iv.Address,
		"protocolSourceCommandUseSBO":    false,
		"protocolSourceCommandDuration":  0.0,
		"description":                    group1 + "~" + iv.ConnName + "~" + iv.DisplayName,
		"ungroupedDescription":           iv.DisplayName,
		"group1":                         group1,
		"group2":                         iv.ConnName,
		"group3":                         iv.CommonAddress,
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
