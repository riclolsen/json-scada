/*
 * IEC 61850 MMS Server driver (IEC61850-90-2 gateway) for {json:scada}, in Go.
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

// Selection of the realtimeData points this server exposes, with the same
// filter semantics as the C# driver and the OPC server drivers.

package main

import (
	"context"
	"sort"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Point is one realtimeData document, reduced to what the driver needs.
type Point struct {
	ID          float64
	Tag         string
	Type        string
	Origin      string
	Group1      string
	Description string

	Value       float64
	ValueString string

	Invalid            bool
	Substituted        bool
	Overflow           bool
	Transient          bool
	TimeTagAtSource    time.Time
	HasTimeTagAtSource bool
	TimeTagAtSourceOk  bool

	// Conversion factors applied to a routed command value, as in the
	// lib60870 server drivers.
	Kconv1 float64
	Kconv2 float64

	// Command routing, kept in the BSON type the source document had so the
	// commandsQueue entries look exactly like the C# driver's.
	SrcConnectionNumber float64
	SrcCommonAddress    any
	SrcObjectAddress    any
	SrcASDU             any
	SrcCommandDuration  float64
	SrcCommandUseSBO    bool
}

// IsCommand reports whether the point is a control point.
func (p *Point) IsCommand() bool { return p.Origin == "command" }

// pointFromDoc reduces a realtimeData document to a Point.
func pointFromDoc(doc bson.M) *Point {
	p := &Point{
		ID:          jsmongo.GetDouble(doc, "_id", 0),
		Tag:         jsmongo.GetString(doc, "tag", ""),
		Type:        jsmongo.GetString(doc, "type", "digital"),
		Origin:      jsmongo.GetString(doc, "origin", ""),
		Group1:      jsmongo.GetString(doc, "group1", ""),
		Description: jsmongo.GetString(doc, "description", ""),

		Value:       jsmongo.GetDouble(doc, "value", 0),
		ValueString: jsmongo.GetString(doc, "valueString", ""),

		// An absent quality flag means unknown, which is invalid.
		Invalid:           jsmongo.GetBool(doc, "invalid", true),
		Substituted:       jsmongo.GetBool(doc, "substituted", false),
		Overflow:          jsmongo.GetBool(doc, "overflow", false),
		Transient:         jsmongo.GetBool(doc, "transient", false),
		TimeTagAtSourceOk: jsmongo.GetBool(doc, "timeTagAtSourceOk", false),

		Kconv1: jsmongo.GetDouble(doc, "kconv1", 1),
		Kconv2: jsmongo.GetDouble(doc, "kconv2", 0),

		SrcConnectionNumber: jsmongo.GetDouble(doc, "protocolSourceConnectionNumber", 0),
		SrcCommonAddress:    mRaw(doc, "protocolSourceCommonAddress", ""),
		SrcObjectAddress:    mRaw(doc, "protocolSourceObjectAddress", ""),
		SrcASDU:             mRaw(doc, "protocolSourceASDU", ""),
		SrcCommandDuration:  jsmongo.GetDouble(doc, "protocolSourceCommandDuration", 0),
		SrcCommandUseSBO:    jsmongo.GetBool(doc, "protocolSourceCommandUseSBO", false),
	}
	if t := jsmongo.GetTime(doc, "timeTagAtSource"); !t.IsZero() {
		p.TimeTagAtSource = t.UTC()
		p.HasTimeTagAtSource = true
	}
	return p
}

// selectPoints queries the points this connection exposes: everything but
// internal points and the ones this very connection is the source of (loop
// prevention), restricted to the topics when any are configured, and
// without command points when commands are disabled.
func selectPoints(ctx context.Context, collRTD *mongo.Collection, conn *ServerConnection) []*Point {
	filter := bson.M{
		"_id":                            bson.M{"$gt": 0},
		"protocolSourceConnectionNumber": bson.M{"$ne": conn.ProtocolConnectionNumber},
	}
	if len(conn.Topics) > 0 {
		filter["group1"] = bson.M{"$in": conn.Topics}
	}
	if !conn.CommandsEnabled {
		filter["origin"] = bson.M{"$ne": "command"}
	}

	cur, err := collRTD.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		jslog.Fatal("Error reading realtime data - %v", err)
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		jslog.Fatal("Error reading realtime data - %v", err)
	}

	points := make([]*Point, 0, len(docs))
	for _, d := range docs {
		p := pointFromDoc(d)
		if p.Tag == "" {
			continue
		}
		points = append(points, p)
	}
	// The model layout depends on the order, and object references have to
	// be stable across restarts.
	sort.SliceStable(points, func(i, j int) bool { return points[i].ID < points[j].ID })

	jslog.Log(jslog.LevelNoLog, "Points selected from realtimeData: %d", len(points))
	if len(points) == 0 {
		jslog.Log(jslog.LevelNoLog, "WARNING: no points matched the topics filter - server model will be empty.")
	}
	return points
}
