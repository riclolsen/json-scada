/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package clientapp

import (
	"context"
	"fmt"

	"modbus-go/internal/mbcore"

	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jsrtdata"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// RtOp is one realtimeData update: a $set on the document with this _id.
type RtOp struct {
	ID  any
	Set bson.M
}

// Sink receives the updates of one poll cycle.
type Sink interface {
	Apply(ctx context.Context, ops []RtOp) error
}

// MongoSink writes to realtimeData with one unordered bulk write per cycle.
type MongoSink struct{ Coll *mongo.Collection }

// Apply implements Sink.
func (s MongoSink) Apply(ctx context.Context, ops []RtOp) error {
	if len(ops) == 0 {
		return nil
	}
	models := make([]mongo.WriteModel, len(ops))
	for i, op := range ops {
		models[i] = mongo.NewUpdateOneModel().SetFilter(bson.M{"_id": op.ID}).SetUpdate(bson.M{"$set": op.Set})
	}
	_, err := s.Coll.BulkWrite(ctx, models, options.BulkWrite().SetOrdered(false))
	return err
}

// TagUpdate is one decoded value.
type TagUpdate struct {
	ID        any
	Value     mbcore.Value
	JSON      []int
	AsduLabel string
	Invalid   bool
	Cot       string // "3" spontaneous, "20" integrity
}

// Updater accumulates the sourceDataUpdate documents of a cycle.
type Updater struct {
	connNumber float64
	ops        []RtOp
}

// NewUpdater starts an empty batch.
func NewUpdater(connNumber float64) *Updater { return &Updater{connNumber: connNumber} }

func (u *Updater) originator() string {
	return fmt.Sprintf("MODBUS|%s", mbcore.JSNumberString(u.connNumber))
}

// Queue adds a value. The document is the Node.js driver's, field for field.
func (u *Updater) Queue(t TagUpdate) {
	var valueAtSource float64
	switch t.Value.Kind {
	case mbcore.KindString:
		valueAtSource = 0
	default:
		valueAtSource = t.Value.Float()
	}
	var vjson any
	if t.JSON != nil {
		a := bson.A{}
		for _, b := range t.JSON {
			a = append(a, int32(b))
		}
		vjson = a
	}
	sdu := jsrtdata.SourceDataUpdate{
		ValueAtSource:               valueAtSource,
		ValueStringAtSource:         t.Value.String(),
		AsduAtSource:                t.AsduLabel,
		CauseOfTransmissionAtSource: t.Cot,
		TimeTagAtSource:             nil,
		TimeTagAtSourceOk:           false,
		TimeTag:                     jsmongo.Now(),
		InvalidAtSource:             t.Invalid,
		Extra: bson.M{
			"valueJsonAtSource": vjson,
			"transientAtSource": false,
			"originator":        u.originator(),
		},
	}
	u.ops = append(u.ops, RtOp{ID: t.ID, Set: bson.M{"sourceDataUpdate": sdu.BSON()}})
}

// QueueInvalid marks tags invalid (link down, a failed block read).
func (u *Updater) QueueInvalid(ids []any) {
	for _, id := range ids {
		u.ops = append(u.ops, RtOp{ID: id, Set: bson.M{
			"sourceDataUpdate.invalidAtSource": true,
			"sourceDataUpdate.timeTag":         jsmongo.Now(),
			"sourceDataUpdate.originator":      u.originator(),
		}})
	}
}

// Pending is the number of queued updates.
func (u *Updater) Pending() int { return len(u.ops) }

// Flush hands the batch to the sink.
func (u *Updater) Flush(ctx context.Context, s Sink) error {
	if len(u.ops) == 0 {
		return nil
	}
	batch := u.ops
	u.ops = nil
	return s.Apply(ctx, batch)
}
