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

// Assigning DNP3 object addresses to tags not yet distributed on a connection.
// Port of AutoCreateDestinations() of the corrected C++ server v0.1.1.

package serverapp

import (
	"context"
	"strconv"

	"dnp3-go/internal/dnp3util"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// MaxObjectAddress is the highest DNP3 object address.
const MaxObjectAddress = 65535

// autoCreatePass is one group's worth of automatic distribution.
type autoCreatePass struct {
	tagType         string
	origin          string
	group           int
	asdu            float64
	commandDuration float64
	whatOverflows   string

	// statusGroup, when set, is the output status group that mirrors this
	// command group: 10 for a CROB, 40 for an analog output block. The status
	// destination goes on the command's supervised twin — the tag the schema
	// calls the one "where the command feedback manifests" — at the same
	// object address, because that is what the protocol means. A CROB at index
	// N operates binary output N, and group 10 index N is that output's state.
	statusGroup int
	statusASDU  float64
}

// autoCreateAll runs the four passes of the C++ server, in its order: commands
// first when they are enabled, then supervised points.
func (e *Engine) autoCreateAll(ctx context.Context, db *mongo.Database, conn *Connection) error {
	if !conn.AutoCreateTags {
		return nil
	}
	jslog.Log(jslog.LevelBasic, "Auto Create Tags is enabled")

	for _, p := range autoCreatePasses(conn) {
		if err := e.autoCreatePass(ctx, db, conn, p); err != nil {
			return err
		}
	}
	return nil
}

// autoCreatePasses is the order the C++ server uses: commands first when they
// are enabled, then supervised points.
func autoCreatePasses(conn *Connection) []autoCreatePass {
	passes := []autoCreatePass{}
	if conn.CommandsEnabled {
		// Digital commands are distributed as group 12 variation 1, analog
		// commands as group 41 variation 3.
		passes = append(passes,
			autoCreatePass{
				tagType: "digital", origin: "command",
				group: dnp3util.GroupCROBCommand, asdu: 1.0, commandDuration: 11.0,
				whatOverflows: "crob commands",
				// Binary output status, g10v2, so a master can read back what
				// it operated.
				statusGroup: dnp3util.GroupBinaryOutputStatus, statusASDU: 2.0,
			},
			autoCreatePass{
				tagType: "analog", origin: "command",
				group: dnp3util.GroupAnalogOutputBlock, asdu: 3.0, commandDuration: 0.0,
				whatOverflows: "analog outputs",
				// Analog output status, g40v3, single precision to match the
				// family default the outstation configures.
				statusGroup: dnp3util.GroupAnalogOutputStatus, statusASDU: 3.0,
			},
		)
	}
	passes = append(passes,
		// Digital points as group 1 variation 2, analog points as group 30
		// variation 6 (double precision).
		//
		// These run after the command passes, and their candidate query skips
		// a tag already distributed on this connection. A command's supervised
		// twin has just been given its output status destination, so it is not
		// also published as a binary or analog input: a controllable point is
		// an output, and one representation of it is what a master wants.
		autoCreatePass{
			tagType: "digital", origin: "supervised",
			group: dnp3util.GroupBinaryInput, asdu: 2.0, whatOverflows: "digitals",
		},
		autoCreatePass{
			tagType: "analog", origin: "supervised",
			group: dnp3util.GroupAnalogInput, asdu: 6.0, whatOverflows: "analogs",
		},
	)
	return passes
}

// autoCreatePass assigns addresses for one group.
//
// Addresses continue from the highest already assigned on this connection *for
// this group*: every group has its own address space, so a tag distributed at
// group 30 must not push the next group 12 address along.
func (e *Engine) autoCreatePass(ctx context.Context, db *mongo.Database, conn *Connection, p autoCreatePass) error {
	rtd := db.Collection(jsmongo.RealtimeDataCollectionName)

	// The conditions are combined with $elemMatch so they hold for one entry of
	// the array, and the entry is checked again when walking it: as separate
	// dotted paths, a destination of another group or another connection would
	// contribute its address to this group's maximum.
	assigned, err := jsmongo.FindAll(ctx, rtd, bson.M{
		"type":   p.tagType,
		"origin": p.origin,
		"protocolDestinations": bson.M{"$elemMatch": bson.M{
			"protocolDestinationConnectionNumber": conn.ProtocolConnectionNumber,
			"protocolDestinationCommonAddress":    p.group,
		}},
	})
	if err != nil {
		return err
	}

	lastAddr := -1
	for _, doc := range assigned {
		for _, d := range DestinationsOf(doc) {
			if d.ConnectionNumber != conn.ProtocolConnectionNumber || d.CommonAddress != p.group {
				continue
			}
			if d.ObjectAddress > lastAddr {
				lastAddr = d.ObjectAddress
			}
		}
	}
	jslog.Log(jslog.LevelBasic, "%s - Last Group %d Address: %d", conn.Name, p.group, lastAddr)

	// Tags of this kind with no destination on this connection yet. $ne on an
	// array field matches a document when no element of it equals the value,
	// which is also true of a document with no destinations at all.
	candidates, err := jsmongo.FindAll(ctx, rtd, bson.M{
		"type":   p.tagType,
		"origin": p.origin,
		"protocolDestinations.protocolDestinationConnectionNumber": bson.M{
			"$ne": conn.ProtocolConnectionNumber,
		},
	}, options.Find().SetSort(bson.M{"_id": 1}))
	if err != nil {
		return err
	}

	for _, doc := range candidates {
		if !conn.MatchesTopic(jsmongo.GetString(doc, "group1", "")) {
			continue
		}
		lastAddr++
		if lastAddr > MaxObjectAddress {
			jslog.Log(jslog.LevelBasic, "%s - Object address for %s exceeds 65535!",
				conn.Name, p.whatOverflows)
			break
		}

		id := jsmongo.GetDouble(doc, "_id", 0)
		jslog.Log(jslog.LevelBasic, "%s - Creating destination for tag: %s %s Dnp3Address: %s",
			conn.Name, jsmongo.FormatID(id), jsmongo.GetString(doc, "tag", ""),
			strconv.Itoa(lastAddr))

		if err := pushDestination(ctx, rtd, id, destination{
			connectionNumber: conn.ProtocolConnectionNumber,
			commonAddress:    p.group,
			objectAddress:    lastAddr,
			asdu:             p.asdu,
			commandDuration:  p.commandDuration,
		}); err != nil {
			return err
		}

		if p.statusGroup != 0 {
			if err := e.createOutputStatus(ctx, rtd, conn, p, doc, lastAddr); err != nil {
				return err
			}
		}
	}
	return nil
}

// destination is one protocolDestinations entry to append.
type destination struct {
	connectionNumber int
	commonAddress    int
	objectAddress    int
	asdu             float64
	commandDuration  float64
}

// pushDestination appends a destination to a tag, creating the array first
// when the tag has none.
func pushDestination(ctx context.Context, rtd *mongo.Collection, id float64, d destination) error {
	var doc bson.M
	if err := rtd.FindOne(ctx, bson.M{"_id": id}).Decode(&doc); err == nil {
		if v, ok := doc["protocolDestinations"]; !ok || v == nil {
			if _, err := rtd.UpdateOne(ctx, bson.M{"_id": id},
				bson.M{"$set": bson.M{"protocolDestinations": bson.A{}}}); err != nil {
				return err
			}
		}
	}
	_, err := rtd.UpdateOne(ctx, bson.M{"_id": id},
		bson.M{"$push": bson.M{"protocolDestinations": bson.M{
			"protocolDestinationConnectionNumber": float64(d.connectionNumber),
			"protocolDestinationCommonAddress":    float64(d.commonAddress),
			"protocolDestinationObjectAddress":    float64(d.objectAddress),
			"protocolDestinationASDU":             d.asdu,
			"protocolDestinationCommandDuration":  d.commandDuration,
			"protocolDestinationCommandUseSBO":    false,
			"protocolDestinationKConv1":           1.0,
			"protocolDestinationKConv2":           0.0,
			"protocolDestinationGroup":            0.0,
			"protocolDestinationHoursShift":       0.0,
		}}})
	return err
}

// createOutputStatus gives a command's supervised twin the output status
// destination that mirrors the command, so a master can read back the state of
// what it operated.
//
// The address is the command's own: DNP3 ties them together, a CROB at index N
// operating binary output N whose state is group 10 index N. That is why this
// cannot pick a free address of its own, and why a clash is reported rather
// than worked around.
func (e *Engine) createOutputStatus(ctx context.Context, rtd *mongo.Collection,
	conn *Connection, p autoCreatePass, cmdDoc bson.M, address int) error {

	cmdTag := jsmongo.GetString(cmdDoc, "tag", "")
	twinID := jsmongo.GetDouble(cmdDoc, "supervisedOfCommand", 0)
	if twinID == 0 {
		// A command with no feedback point. The schema calls this a blind
		// command and discourages it, but it is legal and there is simply
		// nothing whose state could be reported.
		jslog.Log(jslog.LevelDetailed,
			"%s - No supervised twin for command %s; no group %d status created.",
			conn.Name, cmdTag, p.statusGroup)
		return nil
	}

	var twin bson.M
	if err := rtd.FindOne(ctx, bson.M{"_id": twinID}).Decode(&twin); err != nil {
		jslog.Log(jslog.LevelBasic,
			"%s - Command %s names supervised tag %s, which does not exist; no group %d status created.",
			conn.Name, cmdTag, jsmongo.FormatID(twinID), p.statusGroup)
		return nil
	}

	// Already published there by an earlier run or by hand.
	for _, d := range DestinationsOf(twin) {
		if d.ConnectionNumber == conn.ProtocolConnectionNumber && d.CommonAddress == p.statusGroup {
			return nil
		}
	}

	// The address is fixed by the command, so a tag already sitting on it is a
	// clash this pass cannot resolve. Report it and leave both alone: the
	// command still works, and silently moving somebody's configured point
	// would be worse than an unreported status.
	var clash bson.M
	err := rtd.FindOne(ctx, bson.M{
		"protocolDestinations": bson.M{"$elemMatch": bson.M{
			"protocolDestinationConnectionNumber": conn.ProtocolConnectionNumber,
			"protocolDestinationCommonAddress":    p.statusGroup,
			"protocolDestinationObjectAddress":    address,
		}},
	}).Decode(&clash)
	if err == nil {
		jslog.Log(jslog.LevelBasic,
			"%s - Group %d address %d is already taken by tag %s; no status created for command %s.",
			conn.Name, p.statusGroup, address, jsmongo.GetString(clash, "tag", ""), cmdTag)
		return nil
	}

	jslog.Log(jslog.LevelBasic,
		"%s - Creating group %d status for command %s on tag: %s %s Dnp3Address: %d",
		conn.Name, p.statusGroup, cmdTag, jsmongo.FormatID(twinID),
		jsmongo.GetString(twin, "tag", ""), address)

	return pushDestination(ctx, rtd, twinID, destination{
		connectionNumber: conn.ProtocolConnectionNumber,
		commonAddress:    p.statusGroup,
		objectAddress:    address,
		asdu:             p.statusASDU,
	})
}
