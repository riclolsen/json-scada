/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
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

// Automatically created tag documents. Port of TagsCreation.cs.
//
// Every field of realtimeData is written out, at the same default the C#
// rtData class declares: the AdminUI and cs_data_processor both expect
// complete documents.

package main

import (
	"maps"
	"strconv"
	"strings"

	"github.com/riclolsen/json-scada/src/go-common/jstags"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// tagTypeFor maps an OPC UA type name onto the json-scada tag type.
func tagTypeFor(asdu string, isArray bool) string {
	t := "analog"
	switch strings.ToLower(asdu) {
	case "boolean":
		t = "digital"
	case "string", "bytestring", "localeid", "localizedtext",
		"xmlelement", "qualifiedname", "guid":
		t = "string"
	case "nodeid", "expandednodeid", "extensionobject":
		t = "json"
	}
	if isArray {
		t = "json"
	}
	return t
}

// tagFromOPCParameters is the tag name of a supervised point.
func tagFromOPCParameters(ov OPCValue) string {
	return ov.ConnName + ";" + ov.Address
}

// newRealtimeDoc builds the realtimeData document for a discovered point.
//
// The C# version is five near-identical object literals; the differences
// between them are small enough to express as a base document plus the
// per-shape overrides below. Compare against TagsCreation.cs when changing
// anything here.
func newRealtimeDoc(ov OPCValue, id float64, commandOfSupervised float64) bson.M {
	typ := tagTypeFor(ov.Asdu, ov.IsArray)
	isCommand := ov.CreateCommandForMethod || ov.CreateCommandForSupervised

	doc := jstags.BaseDoc()
	maps.Copy(doc, bson.M{
		"_id":                              id,
		"protocolSourceBrowsePath":         ov.Path,
		"protocolSourceAccessLevel":        strconv.Itoa(int(ov.AccessLevels)),
		"protocolSourceASDU":               ov.Asdu,
		"protocolSourceCommonAddress":      ov.CommonAddress,
		"protocolSourceConnectionNumber":   float64(ov.ConnNumber),
		"protocolSourceObjectAddress":      ov.Address,
		"protocolSourceCommandUseSBO":      false,
		"protocolSourceCommandDuration":    0.0,
		"protocolSourcePublishingInterval": 5.0,
		"protocolSourceSamplingInterval":   2.0,
		"protocolSourceQueueSize":          10.0,
		"protocolSourceDiscardOldest":      true,

		"alarmState":           -1.0,
		"description":          ov.ConnName + "~" + ov.Path + "~" + ov.DisplayName,
		"ungroupedDescription": ov.DisplayName,
		"group1":               ov.ConnName,
		"group2":               ov.Path,
		"group3":               "",
		"stateTextFalse":       "",
		"stateTextTrue":        "",
		"eventTextFalse":       "",
		"eventTextTrue":        "",
		"origin":               "supervised",
		"tag":                  tagFromOPCParameters(ov),
		"type":                 typ,
		"value":                0.0,
		"valueString":          "",
		"valueJson":            "",

		"commandOfSupervised":  commandOfSupervised,
		"invalid":              true,
		"invalidDetectTimeout": 60000.0,
		"protocolDestinations": bson.A{},
		"supervisedOfCommand":  0.0,
	})

	switch {
	case isCommand:
		doc["origin"] = "command"
		doc["tag"] = tagFromOPCParameters(ov) + ";cmd"
		doc["description"] = ov.ConnName + "~" + ov.Path + "~" + ov.DisplayName + "-Command"
		doc["alarmState"] = 2.0
		doc["protocolSourcePublishingInterval"] = 0.0
		doc["protocolSourceSamplingInterval"] = 0.0
		doc["protocolSourceQueueSize"] = 0.0
		// parity: a command document carries no link back to the point it
		// drives unless it was created for a writable variable, and then
		// the supervised twin is the very next key.
		doc["commandOfSupervised"] = 0.0
		if ov.CreateCommandForSupervised {
			doc["supervisedOfCommand"] = id + 1
		}
		if typ == "digital" {
			doc["stateTextFalse"] = "FALSE"
			doc["stateTextTrue"] = "TRUE"
			doc["eventTextFalse"] = "FALSE"
			doc["eventTextTrue"] = "TRUE"
		}

	case typ == "digital":
		doc["alarmState"] = 2.0
		doc["stateTextFalse"] = "FALSE"
		doc["stateTextTrue"] = "TRUE"
		doc["eventTextFalse"] = "FALSE"
		doc["eventTextTrue"] = "TRUE"
		doc["value"] = ov.Value

	case typ == "string":
		doc["valueString"] = ov.ValueString

	case typ == "json":
		doc["valueString"] = ov.ValueString
		doc["valueJson"] = ov.ValueJSON

	default: // analog
		doc["value"] = ov.Value
		// parity: the analog literal in TagsCreation.cs is the only one
		// that never assigns valueJson, so the C# field keeps its null
		// default and the stored document gets null rather than "".
		doc["valueJson"] = nil
	}

	if isCommand {
		doc["value"] = 0.0
		doc["valueString"] = ""
		doc["valueJson"] = ""
	}

	return doc
}
