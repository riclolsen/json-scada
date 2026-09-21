/*
 * Shared {json:scada} driver support library, in Go.
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

// Package jsmodel holds the {json:scada} documents the drivers share.
//
// Scope note: these are the bson.M-decoding forms, which is how the flat
// drivers and dnp3-go read their configuration. iec60870-5 decodes
// protocolDriverInstances and protocolConnections into bson-tagged structs
// instead, with float64 numeric fields so any BSON numeric type decodes; that
// path stays in iec60870-5/internal/model, because moving it would change the
// decoder in use rather than just where the code lives.
package jsmodel

import (
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// DriverInstance is a protocolDriverInstances document.
type DriverInstance struct {
	ID                               bson.ObjectID
	ProtocolDriver                   string
	ProtocolDriverInstanceNumber     int
	Enabled                          bool
	LogLevel                         int
	NodeNames                        []string
	ActiveNodeName                   string
	ActiveNodeKeepAliveTimeTag       time.Time
	KeepProtocolRunningWhileInactive bool
}

// InstanceFromDoc decodes a protocolDriverInstances document.
func InstanceFromDoc(doc bson.M) *DriverInstance {
	inst := &DriverInstance{
		ProtocolDriver:                   jsmongo.GetString(doc, "protocolDriver", ""),
		ProtocolDriverInstanceNumber:     jsmongo.GetInt(doc, "protocolDriverInstanceNumber", 1),
		Enabled:                          jsmongo.GetBool(doc, "enabled", true),
		LogLevel:                         jsmongo.GetInt(doc, "logLevel", 1),
		NodeNames:                        jsmongo.GetStringArray(doc, "nodeNames"),
		ActiveNodeName:                   jsmongo.GetString(doc, "activeNodeName", ""),
		ActiveNodeKeepAliveTimeTag:       jsmongo.GetTime(doc, "activeNodeKeepAliveTimeTag"),
		KeepProtocolRunningWhileInactive: jsmongo.GetBool(doc, "keepProtocolRunningWhileInactive", false),
	}
	if id, ok := doc["_id"].(bson.ObjectID); ok {
		inst.ID = id
	}
	return inst
}

// NodeAllowed reports whether this node may run the instance: an empty
// nodeNames list means any node.
func NodeAllowed(inst *DriverInstance, nodeName string) bool {
	if len(inst.NodeNames) == 0 {
		return true
	}
	for _, n := range inst.NodeNames {
		if n == nodeName {
			return true
		}
	}
	return false
}

// InstanceFilter is the query that selects a driver's own instance document.
func InstanceFilter(driverName string, instanceNumber int) bson.M {
	return bson.M{
		"protocolDriver":               driverName,
		"protocolDriverInstanceNumber": instanceNumber,
	}
}
