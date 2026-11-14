/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package serverapp

// Feeds realtimeData values into the PointMap: a full load, a change stream
// for live updates and a full reload every 5 minutes. Ported from rt-watcher.ts.

import (
	"context"
	"reflect"
	"sync"
	"time"

	"modbus-go/internal/drv"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// FullReloadPeriod is how often the whole map is rebuilt.
var FullReloadPeriod = 5 * time.Minute

// Watcher keeps a PointMap in step with realtimeData.
type Watcher struct {
	Cfg  Config
	Coll *mongo.Collection
	PM   *PointMap

	reload  chan struct{}
	restart chan struct{} // the tag set changed: reopen the change stream

	mu  sync.Mutex
	ids bson.A // _ids of the mapped tags, in _id order
}

// ExtractTagDoc picks the fields the map needs and this connection's
// protocolDestinations entry; ok is false when the tag has none.
func ExtractTagDoc(d bson.M, connNumber float64) (TagDoc, bool) {
	var dest map[string]any
	for _, x := range jsmongo.GetDocArray(d, "protocolDestinations") {
		v, has := x["protocolDestinationConnectionNumber"]
		if drv.JSNumber(v, has) == connNumber {
			dest = x
			break
		}
	}
	if dest == nil {
		return TagDoc{}, false
	}
	value, hasValue := d["value"]
	k1, h1 := d["kconv1"]
	k2, h2 := d["kconv2"]
	return TagDoc{
		ID: d["_id"], Tag: anyStr(d["tag"]), Type: d["type"],
		Value: value, HasValue: hasValue, ValueString: d["valueString"], Invalid: d["invalid"],
		Origin: d["origin"], Kconv1: k1, Kconv2: k2, HasKconv1: h1, HasKconv2: h2,
		ProtocolSourceConnectionNumber: d["protocolSourceConnectionNumber"],
		ProtocolSourceCommonAddress:    d["protocolSourceCommonAddress"],
		ProtocolSourceObjectAddress:    d["protocolSourceObjectAddress"],
		ProtocolSourceASDU:             d["protocolSourceASDU"],
		Dest:                           dest,
	}, true
}

func (w *Watcher) filter() bson.M {
	return bson.M{"protocolDestinations.protocolDestinationConnectionNumber": w.Cfg.ProtocolConnectionNumber}
}

// FullReload rebuilds the map from realtimeData, in _id order.
func (w *Watcher) FullReload(ctx context.Context) error {
	fctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	docs, err := jsmongo.FindAll(fctx, w.Coll, w.filter(), options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return err
	}
	tags := make([]TagDoc, 0, len(docs))
	for _, d := range docs {
		if t, ok := ExtractTagDoc(d, w.Cfg.ProtocolConnectionNumber); ok {
			tags = append(tags, t)
		}
	}
	w.PM.Rebuild(tags)
	w.setIDs(tags)
	return nil
}

// setIDs records the _ids of the mapped tags and, when the set changed, asks
// the change stream to reopen with the new pre-lookup filter.
func (w *Watcher) setIDs(tags []TagDoc) {
	ids := make(bson.A, 0, len(tags))
	for _, t := range tags {
		ids = append(ids, t.ID)
	}
	w.mu.Lock()
	changed := !reflect.DeepEqual(w.ids, ids)
	w.ids = ids
	w.mu.Unlock()
	if changed {
		select {
		case w.restart <- struct{}{}:
		default:
		}
	}
}

// pipeline filters the realtimeData change stream.
//
// The first stage does not reference fullDocument, so the server applies it
// before the updateLookup: an update is only looked up when it is a processed
// value (no sourceDataUpdate) of a mapped tag, or when it changes
// protocolDestinations (a tag may have been mapped to this connection). The
// raw driver writes and the updates of every other tag are dropped without
// reading the document. Inserts and replaces always pass.
func (w *Watcher) pipeline() mongo.Pipeline {
	w.mu.Lock()
	ids := w.ids
	w.mu.Unlock()
	if ids == nil {
		ids = bson.A{}
	}
	destinationsChanged := bson.M{"$gt": bson.A{
		bson.M{"$size": bson.M{"$filter": bson.M{
			"input": bson.M{"$objectToArray": "$updateDescription.updatedFields"},
			"cond":  bson.M{"$eq": bson.A{bson.M{"$indexOfCP": bson.A{"$$this.k", "protocolDestinations"}}, 0}},
		}}},
		0,
	}}
	return mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"$or": bson.A{
			bson.M{"operationType": bson.M{"$in": bson.A{"insert", "replace"}}},
			bson.M{"$and": bson.A{
				bson.M{"operationType": "update"},
				bson.M{"updateDescription.updatedFields.sourceDataUpdate": bson.M{"$exists": false}},
				bson.M{"$or": bson.A{
					bson.M{"documentKey._id": bson.M{"$in": ids}},
					bson.M{"$expr": destinationsChanged},
				}},
			}},
		}}}},
		bson.D{{Key: "$match", Value: bson.M{
			"fullDocument.protocolDestinations.protocolDestinationConnectionNumber": w.Cfg.ProtocolConnectionNumber,
		}}},
	}
}

// Run follows changes until ctx ends; call FullReload first.
func (w *Watcher) Run(ctx context.Context) {
	w.reload = make(chan struct{}, 1)
	w.restart = make(chan struct{}, 1)
	go w.watch(ctx)
	t := time.NewTicker(FullReloadPeriod)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-w.reload:
		}
		if err := w.FullReload(ctx); err != nil && ctx.Err() == nil {
			jslog.Log(jslog.LevelBasic, "%s: full reload error: %v", w.Cfg.Name, err)
		}
	}
}

func (w *Watcher) watch(ctx context.Context) {
	// resume is the token of a stream closed to change its filter: the next
	// one starts right after it, so no change falls in between.
	var resume bson.Raw
	for ctx.Err() == nil {
		opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)
		if resume != nil {
			opts.SetResumeAfter(resume)
		}
		sctx, cancel := context.WithCancel(ctx)
		cs, err := w.Coll.Watch(sctx, w.pipeline(), opts)
		resume = nil
		restarted := false
		if err != nil {
			if ctx.Err() == nil {
				jslog.Log(jslog.LevelBasic, "%s: rt watch failed: %v", w.Cfg.Name, err)
			}
		} else {
			stop := make(chan struct{})
			go func() {
				select {
				case <-w.restart:
					cancel()
				case <-stop:
				}
			}()
			for cs.Next(sctx) {
				var ev struct {
					FullDocument bson.M `bson:"fullDocument"`
				}
				if err := cs.Decode(&ev); err == nil && ev.FullDocument != nil {
					w.apply(ev.FullDocument)
				}
			}
			close(stop)
			restarted = sctx.Err() != nil && ctx.Err() == nil
			if restarted {
				resume = cs.ResumeToken()
			} else if err := cs.Err(); err != nil && ctx.Err() == nil {
				jslog.Log(jslog.LevelBasic, "%s: rt change stream error: %v", w.Cfg.Name, err)
			}
			_ = cs.Close(context.Background())
		}
		cancel()
		if restarted {
			jslog.Log(jslog.LevelDetailed, "%s: rt change stream reopened for the new tag set", w.Cfg.Name)
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (w *Watcher) apply(d bson.M) {
	t, ok := ExtractTagDoc(d, w.Cfg.ProtocolConnectionNumber)
	if !ok {
		return
	}
	if !w.PM.Update(t.ID, drv.JSNumber(t.Value, t.HasValue), anyStr(t.ValueString), truthy(t.Invalid)) {
		// A newly relevant tag: rebuild to map it.
		select {
		case w.reload <- struct{}{}:
		default:
		}
	}
}
