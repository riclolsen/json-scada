/*
 * DNP3 Client Protocol driver for {json:scada}, in Go.
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

// Automatic tag creation. Port of autocreate.cpp.

package clientapp

import (
	"context"
	"maps"
	"strconv"

	"dnp3-go/internal/dnp3util"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jstags"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// newTagDoc builds a realtimeData document for an address seen for the first
// time. Port of newRealtimeTagDoc(); every field is reproduced, because the
// rest of JSON-SCADA reads most of them and a missing one shows up as a blank
// column in a viewer rather than as an error.
func newTagDoc(iv Dnp3Value, connName string, id float64, isCommand bool,
	srcCommonAddress int, asdu, commandDuration, commandOfSupervised, supervisedOfCommand float64) bson.M {

	tagType := dnp3util.TypeFromBaseGroup(srcCommonAddress)
	grpDesc := dnp3util.GroupDescription(srcCommonAddress)
	addrStr := strconv.Itoa(iv.Address)
	tag := connName + ";" + strconv.Itoa(srcCommonAddress) + ";" + addrStr
	desc := connName + "~" + grpDesc + "~" + addrStr
	if isCommand {
		desc += "-Command"
	}
	isDigital := tagType == "digital"

	origin := "supervised"
	if isCommand {
		origin = "command"
	}
	alarmState := -1.0
	stateFalse, stateTrue := "", ""
	if isDigital {
		alarmState = 2.0
		stateFalse, stateTrue = "FALSE", "TRUE"
	}
	value, valueString := iv.Value, iv.ValueString
	if isCommand {
		value, valueString = 0.0, ""
	}

	doc := jstags.BaseDoc()
	maps.Copy(doc, bson.M{
		"_id":                            id,
		"tag":                            tag,
		"type":                           tagType,
		"origin":                         origin,
		"description":                    desc,
		"ungroupedDescription":           grpDesc + " " + addrStr,
		"group1":                         connName,
		"group2":                         grpDesc,
		"group3":                         "",
		"protocolSourceConnectionNumber": float64(iv.ConnNumber),
		"protocolSourceCommonAddress":    float64(srcCommonAddress),
		"protocolSourceObjectAddress":    float64(iv.Address),
		"protocolSourceASDU":             asdu,
		"protocolSourceCommandDuration":  commandDuration,
		"protocolSourceCommandUseSBO":    false,
		"commandOfSupervised":            commandOfSupervised,
		"supervisedOfCommand":            supervisedOfCommand,
		"alarmState":                     alarmState,
		"stateTextFalse":                 stateFalse,
		"stateTextTrue":                  stateTrue,
		"eventTextFalse":                 stateFalse,
		"eventTextTrue":                  stateTrue,
		"value":                          value,
		"valueString":                    valueString,
		"invalid":                        true,
		"invalidDetectTimeout":           60000.0,
		"protocolDestinations":           bson.A{},
	})
	return doc
}

// autoCreateFor returns the documents to insert for a value whose address has
// not been seen before, or nil when nothing is to be created.
//
// A supervised point of a controllable family gets a command twin as well, so
// that an operator can act on what the outstation reports. The twin is
// allocated first so that its id is the lower of the two, as in the C++ driver.
func autoCreateFor(ctx context.Context, conn *Connection, rtd *mongo.Collection, iv Dnp3Value) []any {
	if !conn.AutoCreateTags {
		return nil
	}
	key := [2]int{iv.BaseGroup, iv.Address}
	if conn.insertedAddresses[key] {
		return nil
	}
	conn.insertedAddresses[key] = true

	var docs []any
	commandID := 0.0

	if conn.CommandsEnabled &&
		(iv.BaseGroup == dnp3util.GroupBinaryOutputStatus || iv.BaseGroup == dnp3util.GroupAnalogOutputStatus) {

		commandID = conn.tagKeys.Next(ctx, rtd, conn.ProtocolConnectionNumber)
		cmdGroup := dnp3util.GroupAnalogOutputBlock
		asdu := 3.0
		duration := 0.0
		if iv.BaseGroup == dnp3util.GroupBinaryOutputStatus {
			cmdGroup = dnp3util.GroupCROBCommand
			asdu = 1.0
			duration = 3.0 // LATCH 1=ON 0=OFF
		}
		docs = append(docs, newTagDoc(iv, conn.Name, commandID, true, cmdGroup, asdu, duration,
			0.0, commandID+1.0))
		conn.insertedAddresses[[2]int{cmdGroup, iv.Address}] = true
		jslog.Log(jslog.LevelBasic, "%s - INSERT NEW COMMAND TAG: %s;%d;%d",
			conn.Name, conn.Name, cmdGroup, iv.Address)
	}

	newID := conn.tagKeys.Next(ctx, rtd, conn.ProtocolConnectionNumber)
	docs = append(docs, newTagDoc(iv, conn.Name, newID, false, iv.BaseGroup, 0.0, 0.0, commandID, 0.0))
	jslog.Log(jslog.LevelBasic, "%s - INSERT NEW TAG: %s;%d;%d",
		conn.Name, conn.Name, iv.BaseGroup, iv.Address)

	return docs
}
