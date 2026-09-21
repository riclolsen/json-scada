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

// Package jscommands holds the commandsQueue plumbing shared by the client
// drivers: the change-stream pipeline, the fields every driver reads out of a
// command document, the expiry test, and the ack/cancel writeback.
//
// Scope note: the watch loop itself is NOT here. dnp3-go keeps a resume token
// across reconnects and gates on the active node inside its dispatcher, while
// the others start the stream from "now" and gate in the loop; the retry
// pauses and log wording differ too. Those differences are deliberate, so
// each driver keeps its own loop and calls into this package for the parts
// that were identical.
package jscommands

import (
	"context"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// DefaultExpiry is how old a queued command may be before it is refused. The
// operator's intent is stale by then, so acting on it would be wrong.
const DefaultExpiry = 10 * time.Second

// WriteTimeout bounds an ack or cancel writeback.
const WriteTimeout = 10 * time.Second

// InsertOnlyPipeline is the change-stream pipeline every driver uses: a
// command is only ever acted on when it is inserted.
func InsertOnlyPipeline() mongo.Pipeline {
	return mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "operationType", Value: "insert"}}}},
	}
}

// Fields are the parts of a commandsQueue document every driver reads.
// Protocol-specific fields stay with the driver.
type Fields struct {
	ID          any
	ConnNumber  int
	Address     string
	Asdu        string
	Value       float64
	ValueString string
	UseSBO      bool
	Duration    int
	TimeTag     time.Time
}

// FieldsFromDoc reads the common fields, permissively, the way every driver
// reads them.
//
// Address and Asdu are returned as strings because that is how the OPC-UA and
// IEC 61850 clients use them. A driver whose addresses are numeric reads them
// itself with jsmongo.GetU32 rather than parsing these back.
func FieldsFromDoc(doc bson.M) Fields {
	return Fields{
		ID:          doc["_id"],
		ConnNumber:  jsmongo.GetInt(doc, "protocolSourceConnectionNumber", 0),
		Address:     jsmongo.GetString(doc, "protocolSourceObjectAddress", ""),
		Asdu:        jsmongo.GetString(doc, "protocolSourceASDU", ""),
		Value:       jsmongo.GetDouble(doc, "value", 0),
		ValueString: jsmongo.GetString(doc, "valueString", ""),
		UseSBO:      jsmongo.GetBool(doc, "protocolSourceCommandUseSBO", false),
		Duration:    jsmongo.GetInt(doc, "protocolSourceCommandDuration", 0),
		TimeTag:     jsmongo.GetTime(doc, "timeTag"),
	}
}

// Age is how long ago the command was queued.
func (f Fields) Age() time.Duration { return time.Since(f.TimeTag) }

// Expired reports whether the command is too old to act on.
func (f Fields) Expired(expiry time.Duration) bool { return f.Age() > expiry }

// CancelDoc is the update that marks a command refused before it reached the
// device.
func CancelDoc(reason string) bson.M {
	return bson.M{"$set": bson.M{"cancelReason": reason}}
}

// AckDoc is the update that records the outcome of a command that reached the
// device.
func AckDoc(ok bool, resultDescription string) bson.M {
	return bson.M{"$set": bson.M{
		"delivered":         true,
		"ack":               ok,
		"ackTimeTag":        jsmongo.Now(),
		"resultDescription": resultDescription,
	}}
}

// Cancel writes the refusal. The error is returned rather than logged, so
// each driver keeps its own wording and level.
func Cancel(ctx context.Context, coll *mongo.Collection, id any, reason string) error {
	return apply(ctx, coll, id, CancelDoc(reason))
}

// Ack writes the outcome. The error is returned rather than logged, so each
// driver keeps its own wording and level.
func Ack(ctx context.Context, coll *mongo.Collection, id any, ok bool, resultDescription string) error {
	return apply(ctx, coll, id, AckDoc(ok, resultDescription))
}

func apply(ctx context.Context, coll *mongo.Collection, id any, update bson.M) error {
	tctx, cancel := context.WithTimeout(ctx, WriteTimeout)
	defer cancel()
	_, err := coll.UpdateOne(tctx, bson.M{"_id": id}, update)
	return err
}
