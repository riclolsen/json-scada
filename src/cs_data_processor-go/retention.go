package main

// Data retention: once a day removes the soeData, alarmsData and secEvents
// documents older than their retention and keeps the expiry of the hist time series
// collection equal to its retention.
//
// Port of ../cs_data_processor/retention.js: change both together and keep
// retention_test.go and retention.test.js with the same expected values.
// The retentions (days) are the soeRetentionDays, alarmsRetentionDays,
// secEventsRetentionDays and histRetentionDays fields of the systemSettings document { key: 'global' },
// edited in the AdminUI (Admin / System) through server_realtime_auth.
// Contract in docs/schema.md (systemSettings).
//
// Instance 1 on the active node only. Also creates the alarmsData and
// secEvents indexes
// and turns an old capped soeData into a normal collection.

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	retentionDay            = 24 * time.Hour
	defaultRetentionDays    = 60
	maxRetentionDays        = 36500
	retentionFirstRunDelay  = time.Minute  // after the start
	retentionCheckEvery     = time.Hour    // a node that became active runs within an hour
	retentionRunEvery       = retentionDay // once a day
	retentionCommandTimeout = 30 * time.Minute
)

type indexSpec struct {
	keys bson.D
	name string // empty: default name
}

// alarmsData indexes (also in mongo_seed/b_create-db.js)
var alarmsIndexes = []indexSpec{
	{bson.D{{Key: "timeTag", Value: 1}}, "alarmsTimeTagIndex"},
	{bson.D{{Key: "pointKey", Value: 1}, {Key: "active", Value: 1}}, "alarmsPointActiveIndex"},
	{bson.D{{Key: "ack", Value: 1}, {Key: "timeTag", Value: -1}}, "alarmsAckIndex"},
	{bson.D{{Key: "active", Value: 1}, {Key: "timeTag", Value: -1}}, "alarmsActiveIndex"},
	{bson.D{{Key: "group1", Value: 1}}, "alarmsGroup1Index"},
}

// secEvents indexes (also in mongo_seed/b_create-db.js)
var secEventsIndexes = []indexSpec{
	{bson.D{{Key: "timeTag", Value: -1}}, "secEventsTimeTagIndex"},
	{bson.D{{Key: "severityLevel", Value: 1}, {Key: "timeTag", Value: -1}}, "secEventsSeverityIndex"},
	{bson.D{{Key: "kind", Value: 1}, {Key: "timeTag", Value: -1}}, "secEventsKindIndex"},
	{bson.D{{Key: "protocolConnectionNumber", Value: 1}, {Key: "timeTag", Value: -1}}, "secEventsConnectionIndex"},
}

// soeData indexes (also in mongo_seed/b_create-db.js)
var soeIndexes = []indexSpec{
	{bson.D{{Key: "timeTag", Value: 1}}, ""},
	{bson.D{{Key: "timeTagAtSource", Value: 1}}, ""},
	{bson.D{{Key: "group1", Value: 1}}, ""},
	{bson.D{{Key: "ack", Value: 1}}, ""},
}

// retentionDays is a retention in whole days: a missing or invalid value is
// the default.
func retentionDays(v float64, ok bool) int {
	if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < 1 {
		return defaultRetentionDays
	}
	return int(math.Min(math.Trunc(v), maxRetentionDays))
}

type retentionConfig struct{ soe, alarms, hist, secEvents int }

// retentionSettings are the retentions of the systemSettings document
// (invalid when absent).
func retentionSettings(d rawDoc) retentionConfig {
	get := func(k string) int {
		if !d.valid() {
			return defaultRetentionDays
		}
		return retentionDays(d.num(k))
	}
	return retentionConfig{
		soe:       get("soeRetentionDays"),
		alarms:    get("alarmsRetentionDays"),
		hist:      get("histRetentionDays"),
		secEvents: get("secEventsRetentionDays"),
	}
}

// retentionCutoff: documents with a timeTag older than this are removed.
func retentionCutoff(now time.Time, days int) time.Time {
	return now.Add(-time.Duration(days) * retentionDay)
}

// alarmsCleanupFilter selects the alarms removed: older than the cutoff,
// acknowledged and inactive.
func alarmsCleanupFilter(cut time.Time) bson.D {
	return bson.D{
		{Key: "timeTag", Value: bson.D{{Key: "$lt", Value: cut}}},
		{Key: "ack", Value: bson.D{{Key: "$ne", Value: 0}}},
		{Key: "active", Value: bson.D{{Key: "$ne", Value: true}}},
	}
}

// StartRetention launches the daily cleanup.
func StartRetention(ctx context.Context, cfg Config) {
	if cfg.Instance != 1 {
		Log(LogLevelNormal, "Retention - done by instance 1 only")
		return
	}
	go func() {
		var lastRun time.Time
		tick := func() {
			if !lastRun.IsZero() && time.Since(lastRun) < retentionRunEvery {
				return
			}
			if !mongoIsConnected() || !ProcessStateIsActive() {
				return
			}
			_, _, _, _, db, ok := Mongo.Handles()
			if !ok || db == nil {
				return
			}
			defer func() {
				if r := recover(); r != nil {
					M.Inc(CntErrors, 1)
					Log(LogLevelMin, "Retention - error: %v", r)
				}
			}()
			opCtx, cancel := context.WithTimeout(ctx, retentionCommandTimeout)
			defer cancel()
			if err := retentionCycle(opCtx, db, time.Now()); err != nil {
				M.Inc(CntErrors, 1)
				Log(LogLevelMin, "Retention - error: %v", err)
				return
			}
			lastRun = time.Now()
		}
		first := time.NewTimer(retentionFirstRunDelay)
		defer first.Stop()
		t := time.NewTicker(retentionCheckEvery)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-first.C:
				tick()
			case <-t.C:
				tick()
			}
		}
	}()
}

func collectionOptions(ctx context.Context, db *mongo.Database, name string) (bson.Raw, bool, error) {
	cur, err := db.ListCollections(ctx, bson.D{{Key: "name", Value: name}})
	if err != nil {
		return nil, false, err
	}
	defer cur.Close(ctx)
	if !cur.Next(ctx) {
		return nil, false, cur.Err()
	}
	opts, _ := cur.Current.Lookup("options").DocumentOK()
	return opts, true, nil
}

func createIndexes(ctx context.Context, db *mongo.Database, coll string, specs []indexSpec) {
	for _, s := range specs {
		m := mongo.IndexModel{Keys: s.keys}
		if s.name != "" {
			m.Options = options.Index().SetName(s.name)
		}
		if _, err := db.Collection(coll).Indexes().CreateOne(ctx, m); err != nil {
			Log(LogLevelMin, "Retention - index of %s: %v", coll, err)
		}
	}
}

// uncapSoe turns a capped soeData (older installs) into a normal collection:
// renamed first, so new events create the normal collection at once, then
// the events still within the retention are copied back.
func uncapSoe(ctx context.Context, db *mongo.Database, keepFrom time.Time) error {
	opts, found, err := collectionOptions(ctx, db, SoeDataCollectionName)
	if err != nil || !found {
		return err
	}
	if capped, _ := opts.Lookup("capped").BooleanOK(); !capped {
		return nil
	}
	oldName := fmt.Sprintf("%s_capped_%d", SoeDataCollectionName, time.Now().UnixMilli())
	Log(LogLevelMin, "Retention - converting capped %s to a normal collection", SoeDataCollectionName)
	dbName := db.Name()
	if err := db.Client().Database("admin").RunCommand(ctx, bson.D{
		{Key: "renameCollection", Value: dbName + "." + SoeDataCollectionName},
		{Key: "to", Value: dbName + "." + oldName},
	}).Err(); err != nil {
		return err
	}
	if err := db.CreateCollection(ctx, SoeDataCollectionName); err != nil {
		var ce mongo.CommandError
		if !errors.As(err, &ce) || ce.Code != 48 { // 48: created meanwhile by a new event
			return err
		}
	}
	createIndexes(ctx, db, SoeDataCollectionName, soeIndexes)
	cur, err := db.Collection(oldName).Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: "timeTag", Value: bson.D{{Key: "$gte", Value: keepFrom}}}}}},
		{{Key: "$merge", Value: bson.D{
			{Key: "into", Value: SoeDataCollectionName},
			{Key: "whenMatched", Value: "keepExisting"},
			{Key: "whenNotMatched", Value: "insert"},
		}}},
	})
	if err != nil {
		return err
	}
	cur.Close(ctx)
	if err := db.Collection(oldName).Drop(ctx); err != nil {
		return err
	}
	Log(LogLevelMin, "Retention - %s converted", SoeDataCollectionName)
	return nil
}

// histExpiry keeps the hist time series expiry equal to the retention.
func histExpiry(ctx context.Context, db *mongo.Database, days int) error {
	opts, found, err := collectionOptions(ctx, db, HistCollectionName)
	if err != nil || !found {
		return err
	}
	if _, ok := opts.Lookup("timeseries").DocumentOK(); !ok {
		return nil
	}
	secs := float64(days) * 24 * 60 * 60
	if cur, ok := rawValueNum(opts.Lookup("expireAfterSeconds")); ok && cur == secs {
		return nil
	}
	if err := db.RunCommand(ctx, bson.D{
		{Key: "collMod", Value: HistCollectionName},
		{Key: "expireAfterSeconds", Value: int64(secs)},
	}).Err(); err != nil {
		return err
	}
	Log(LogLevelMin, "Retention - %s expiry set to %d days", HistCollectionName, days)
	return nil
}

func retentionCycle(ctx context.Context, db *mongo.Database, now time.Time) error {
	raw, err := db.Collection(SystemSettingsCollectionName).
		FindOne(ctx, bson.D{{Key: "key", Value: "global"}}).Raw()
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return err
	}
	settings := retentionSettings(rawDoc{raw: raw})
	soeCut := retentionCutoff(now, settings.soe)

	createIndexes(ctx, db, AlarmsDataCollectionName, alarmsIndexes)
	createIndexes(ctx, db, SecEventsCollectionName, secEventsIndexes)
	if err := uncapSoe(ctx, db, soeCut); err != nil {
		return err
	}
	soe, err := db.Collection(SoeDataCollectionName).DeleteMany(ctx,
		bson.D{{Key: "timeTag", Value: bson.D{{Key: "$lt", Value: soeCut}}}})
	if err != nil {
		return err
	}
	alm, err := db.Collection(AlarmsDataCollectionName).DeleteMany(ctx,
		alarmsCleanupFilter(retentionCutoff(now, settings.alarms)))
	if err != nil {
		return err
	}
	sec, err := db.Collection(SecEventsCollectionName).DeleteMany(ctx,
		bson.D{{Key: "timeTag", Value: bson.D{{Key: "$lt", Value: retentionCutoff(now, settings.secEvents)}}}})
	if err != nil {
		return err
	}
	if err := histExpiry(ctx, db, settings.hist); err != nil {
		return err
	}
	Log(LogLevelMin, "Retention - removed %d events (%d days), %d alarms (%d days), %d security events (%d days); hist %d days",
		soe.DeletedCount, settings.soe, alm.DeletedCount, settings.alarms, sec.DeletedCount, settings.secEvents, settings.hist)
	return nil
}
