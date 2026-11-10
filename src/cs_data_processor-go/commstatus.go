/*
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

// Communication status tags: the health of every protocol driver instance
// and protocol connection, materialized as ordinary realtimeData tags under
// the _System.Driver., _System.Conn. and _System.Comm. name spaces, so SVG
// displays, the tabular and alarm viewers, the historian and every API see
// it like any other point.
//
// Liveness is judged from outside the driver (a dead driver cannot report
// itself dead) with the same skew-immune rule the redundancy uses: the
// keep-alive is read every cycle and the driver is declared stopped after
// more than commStatusStallLimit consecutive unchanged readings.
//
// Values are written as sourceDataUpdate, so this processor's own change
// stream converts them and produces alarms, SOE, beep and history.
//
// Only processor instance 1, while active, maintains the tags. The
// processInstances entry field commStatusTags (default true) turns it off;
// commStatusMirrorStats (default false) also mirrors each numeric
// protocolConnections stats counter.

package main

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	commStatusCycle = 5 * time.Second
	// tag counts are aggregated every commStatusCountEvery cycles (15 s)
	commStatusCountEvery = 3
	// stopped after more than this many consecutive unchanged keep-alives
	// (about 25-30 s), same limit as the redundancy takeover
	commStatusStallLimit = 4
	// a keep-alive seen for the first time counts as alive when newer than
	// this, by the local clock (the only reading without a history)
	commStatusFirstSeenGrace = 30 * time.Second
	// keys of generated tags are allocated below this one
	commStatusKeyCeiling = -1000.0
	commStatusField      = "commStatus"
)

const (
	csKindDriver  = "driver"
	csKindConn    = "conn"
	csKindSummary = "summary"
)

// ---------------------------------------------------------------------
// snapshot of the configuration collections
// ---------------------------------------------------------------------

type csInstance struct {
	ID           string
	Driver       string
	Number       float64
	Enabled      bool
	ActiveNode   string
	Version      string
	KeepAlive    time.Time
	HasKeepAlive bool
}

type csConnection struct {
	ID           string
	Number       float64
	Name         string
	Driver       string
	InstNumber   float64
	Enabled      bool
	StatsTime    time.Time
	HasStatsTime bool
	StatsNode    string
	IsConnected  *bool
	NumStats     map[string]float64
}

type csTagCount struct {
	Total   float64
	Invalid float64
}

type csSnapshot struct {
	Instances   []csInstance
	Connections []csConnection
	// tag counts by protocolSourceConnectionNumber; nil when not refreshed
	Counts map[float64]csTagCount
}

// ---------------------------------------------------------------------
// desired points
// ---------------------------------------------------------------------

type csSpec struct {
	Kind, Ref, Point string
	Tag              string
	Type             string // digital | analog | string
	Group2, Group3   string
	Text             string
	StateTrue        string
	StateFalse       string
	Unit             string
	Alarm            bool
	Priority         float64
	NoHistory        bool
	Deadband         float64
}

type csValue struct {
	Value   float64
	Str     string
	Invalid bool
}

type csPoint struct {
	Spec csSpec
	Val  csValue
}

func (s csSpec) key() string { return s.Kind + "|" + s.Ref + "|" + s.Point }

var csNameSanitizer = regexp.MustCompile(`[^A-Za-z0-9_\-]`)

// csSanitize makes a name usable as one dot-separated tag segment.
func csSanitize(s string) string {
	s = csNameSanitizer.ReplaceAllString(strings.TrimSpace(s), "_")
	if s == "" {
		return "_"
	}
	return s
}

// ---------------------------------------------------------------------
// keep-alive stall detection
// ---------------------------------------------------------------------

type csStall struct {
	seen        bool
	last        time.Time
	unchanged   int
	lastAdvance time.Time
}

// observe records one reading and reports whether the source is alive and
// how long ago (local clock) it last advanced.
func (s *csStall) observe(v time.Time, has bool, now time.Time) (alive bool, age float64) {
	if !has {
		s.seen = false
		return false, 0
	}
	if !s.seen {
		s.seen = true
		s.last = v
		s.unchanged = 0
		s.lastAdvance = v
		if s.lastAdvance.After(now) {
			s.lastAdvance = now
		}
		alive = now.Sub(v) < commStatusFirstSeenGrace
		if !alive {
			// already stale at start: count it as stalled
			s.unchanged = commStatusStallLimit + 1
		}
		return alive, math.Max(0, now.Sub(s.lastAdvance).Seconds())
	}
	if v.Equal(s.last) {
		s.unchanged++
	} else {
		s.unchanged = 0
		s.last = v
		s.lastAdvance = now
	}
	return s.unchanged <= commStatusStallLimit, math.Max(0, now.Sub(s.lastAdvance).Seconds())
}

// ---------------------------------------------------------------------
// the monitor
// ---------------------------------------------------------------------

type commStatus struct {
	cfg        Config
	instStall  map[string]*csStall
	statsStall map[string]*csStall
	counts     map[float64]csTagCount
	// last value written (or read back) per spec key
	written    map[string]csValue
	cycleCount int
	nextKey    float64
	disabled   bool
}

func newCommStatus(cfg Config) *commStatus {
	return &commStatus{
		cfg:        cfg,
		instStall:  map[string]*csStall{},
		statsStall: map[string]*csStall{},
		counts:     map[float64]csTagCount{},
		written:    map[string]csValue{},
	}
}

// StartCommStatus launches the communication status tags maintenance.
func StartCommStatus(ctx context.Context, cfg Config) {
	if cfg.Instance != 1 {
		Log(LogLevelNormal, "Comm status tags - maintained by instance 1 only")
		return
	}
	cs := newCommStatus(cfg)
	go func() {
		t := time.NewTicker(commStatusCycle)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if !mongoIsConnected() || !ProcessStateIsActive() {
					continue
				}
				func() {
					defer func() {
						if r := recover(); r != nil {
							M.Inc(CntErrors, 1)
							Log(LogLevelMin, "Comm status tags - error: %v", r)
						}
					}()
					cs.cycle(ctx)
				}()
			}
		}
	}()
}

func (cs *commStatus) cycle(ctx context.Context) {
	rt, _, _, _, db, ok := Mongo.Handles()
	if !ok || db == nil {
		return
	}
	opCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	enabled, mirror := cs.readFlags(opCtx, db)
	if !enabled {
		if !cs.disabled {
			cs.disabled = true
			Log(LogLevelMin, "Comm status tags - disabled, invalidating existing tags")
			cs.invalidateAll(opCtx, rt)
		}
		return
	}
	if cs.disabled {
		Log(LogLevelMin, "Comm status tags - enabled")
		cs.disabled = false
		cs.written = map[string]csValue{}
	}

	snap, err := cs.readSnapshot(opCtx, db, rt, cs.cycleCount%commStatusCountEvery == 0)
	cs.cycleCount++
	if err != nil {
		M.Inc(CntErrors, 1)
		Log(LogLevelMin, "Comm status tags - error reading configuration: %v", err)
		return
	}
	desired := cs.build(snap, mirror, time.Now())
	cs.apply(opCtx, rt, desired)
}

func (cs *commStatus) readFlags(ctx context.Context, db *mongo.Database) (enabled, mirror bool) {
	enabled = true
	var doc bson.M
	err := db.Collection(ProcessInstancesCollectionName).FindOne(ctx, bson.D{
		{Key: "processName", Value: AppName},
		{Key: "processInstanceNumber", Value: float64(cs.cfg.Instance)},
	}).Decode(&doc)
	if err != nil {
		return
	}
	if b, ok := doc["commStatusTags"].(bool); ok {
		enabled = b
	}
	if b, ok := doc["commStatusMirrorStats"].(bool); ok {
		mirror = b
	}
	return
}

func (cs *commStatus) readSnapshot(ctx context.Context, db *mongo.Database,
	rt *mongo.Collection, withCounts bool) (csSnapshot, error) {
	var snap csSnapshot

	var insts []bson.M
	cur, err := db.Collection(ProtocolDriverInstancesCollectionName).Find(ctx, bson.D{})
	if err != nil {
		return snap, err
	}
	if err := cur.All(ctx, &insts); err != nil {
		return snap, err
	}
	for _, d := range insts {
		ka, hasKa := csTime(d["activeNodeKeepAliveTimeTag"])
		snap.Instances = append(snap.Instances, csInstance{
			ID:           csID(d["_id"]),
			Driver:       csStr(d["protocolDriver"]),
			Number:       csNum(d["protocolDriverInstanceNumber"]),
			Enabled:      csBool(d["enabled"], true),
			ActiveNode:   csStr(d["activeNodeName"]),
			Version:      csStr(d["softwareVersion"]),
			KeepAlive:    ka,
			HasKeepAlive: hasKa,
		})
	}

	var conns []bson.M
	cur, err = db.Collection(ProtocolConnectionsCollectionName).Find(ctx, bson.D{},
		options.Find().SetProjection(bson.D{
			{Key: "protocolConnectionNumber", Value: 1},
			{Key: "name", Value: 1},
			{Key: "protocolDriver", Value: 1},
			{Key: "protocolDriverInstanceNumber", Value: 1},
			{Key: "enabled", Value: 1},
			{Key: "stats", Value: 1},
		}))
	if err != nil {
		return snap, err
	}
	if err := cur.All(ctx, &conns); err != nil {
		return snap, err
	}
	for _, d := range conns {
		c := csConnection{
			ID:         csID(d["_id"]),
			Number:     csNum(d["protocolConnectionNumber"]),
			Name:       csStr(d["name"]),
			Driver:     csStr(d["protocolDriver"]),
			InstNumber: csNum(d["protocolDriverInstanceNumber"]),
			Enabled:    csBool(d["enabled"], true),
			NumStats:   map[string]float64{},
		}
		if c.Name == "" {
			c.Name = "Connection" + jsNumberToString(c.Number)
		}
		for k, v := range csSubDoc(d["stats"]) {
			switch k {
			case "timeTag":
				c.StatsTime, c.HasStatsTime = csTime(v)
			case "nodeName":
				c.StatsNode = csStr(v)
			case "isConnected":
				if b, ok := v.(bool); ok {
					c.IsConnected = &b
				}
			default:
				if f, ok := csNumOK(v); ok {
					c.NumStats[k] = f
				}
			}
		}
		snap.Connections = append(snap.Connections, c)
	}

	if withCounts {
		counts, err := readTagCounts(ctx, rt)
		if err != nil {
			M.Inc(CntErrors, 1)
			Log(LogLevelMin, "Comm status tags - error counting tags: %v", err)
		} else {
			snap.Counts = counts
		}
	}
	return snap, nil
}

func readTagCounts(ctx context.Context, rt *mongo.Collection) (map[float64]csTagCount, error) {
	cur, err := rt.Aggregate(ctx, mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{{Key: "origin", Value: "supervised"}}}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$protocolSourceConnectionNumber"},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "invalid", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: bson.A{bson.D{{Key: "$eq", Value: bson.A{"$invalid", true}}}, 1, 0}},
			}}}},
		}}},
	})
	if err != nil {
		return nil, err
	}
	var rows []bson.M
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := map[float64]csTagCount{}
	for _, r := range rows {
		n, ok := csNumOK(r["_id"])
		if !ok {
			continue
		}
		out[n] = csTagCount{Total: csNum(r["total"]), Invalid: csNum(r["invalid"])}
	}
	return out, nil
}

// build computes every desired point and its value from one snapshot.
func (cs *commStatus) build(snap csSnapshot, mirror bool, now time.Time) []csPoint {
	if snap.Counts != nil {
		cs.counts = snap.Counts
	}
	var pts []csPoint
	add := func(s csSpec, v csValue) { pts = append(pts, csPoint{Spec: s, Val: v}) }

	type instState struct {
		alive, enabled bool
		activeNode     string
	}
	instByKey := map[string]instState{}
	seenInst := map[string]bool{}
	driversDown := 0.0

	for _, in := range snap.Instances {
		seenInst[in.ID] = true
		st := cs.instStall[in.ID]
		if st == nil {
			st = &csStall{}
			cs.instStall[in.ID] = st
		}
		alive, age := st.observe(in.KeepAlive, in.HasKeepAlive, now)
		instByKey[in.Driver+"|"+jsNumberToString(in.Number)] = instState{alive, in.Enabled, in.ActiveNode}
		if in.Enabled && !alive {
			driversDown++
		}

		base := "_System.Driver." + csSanitize(in.Driver) + "." + csSanitize(jsNumberToString(in.Number)) + "."
		g3 := in.Driver + " #" + jsNumberToString(in.Number)
		mk := func(point, typ, text string) csSpec {
			return csSpec{Kind: csKindDriver, Ref: in.ID, Point: point, Tag: base + point,
				Type: typ, Group2: "Drivers", Group3: g3, Text: text}
		}
		s := mk("Alive", "digital", "Running")
		s.StateTrue, s.StateFalse, s.Alarm, s.Priority = "Running", "Stopped", true, 2
		add(s, csValue{Value: b2f(alive), Invalid: !in.HasKeepAlive})
		s = mk("Enabled", "digital", "Enabled")
		s.StateTrue, s.StateFalse = "Enabled", "Disabled"
		add(s, csValue{Value: b2f(in.Enabled)})
		add(mk("ActiveNode", "string", "Active Node"), csValue{Str: in.ActiveNode, Invalid: !in.HasKeepAlive})
		add(mk("Version", "string", "Software Version"), csValue{Str: in.Version, Invalid: in.Version == ""})
		s = mk("KeepAliveAge", "analog", "Keep-Alive Age")
		s.Unit, s.NoHistory, s.Deadband = "s", true, 5
		add(s, csValue{Value: math.Round(age), Invalid: !in.HasKeepAlive})
	}
	for id := range cs.instStall {
		if !seenInst[id] {
			delete(cs.instStall, id)
		}
	}

	connsDown := 0.0
	seenConn := map[string]bool{}
	for _, c := range snap.Connections {
		seenConn[c.ID] = true
		st := cs.statsStall[c.ID]
		if st == nil {
			st = &csStall{}
			cs.statsStall[c.ID] = st
		}
		_, statsAge := st.observe(c.StatsTime, c.HasStatsTime, now)
		inst, hasInst := instByKey[c.Driver+"|"+jsNumberToString(c.InstNumber)]
		cnt, hasCnt := cs.counts[c.Number]
		health := 0.0
		if hasCnt && cnt.Total > 0 {
			health = math.Round(1000*(cnt.Total-cnt.Invalid)/cnt.Total) / 10
		}
		status := c.Enabled && hasInst && inst.enabled && inst.alive &&
			(c.IsConnected == nil || *c.IsConnected) &&
			(!hasCnt || cnt.Total == 0 || health > 0)
		if c.Enabled && !status {
			connsDown++
		}
		activeNode := c.StatsNode
		if activeNode == "" && hasInst {
			activeNode = inst.activeNode
		}

		base := "_System.Conn." + csSanitize(c.Name) + "."
		mk := func(point, typ, text string) csSpec {
			return csSpec{Kind: csKindConn, Ref: c.ID, Point: point, Tag: base + point,
				Type: typ, Group2: "Connections", Group3: c.Name, Text: text}
		}
		s := mk("Status", "digital", "Status")
		s.StateTrue, s.StateFalse, s.Alarm, s.Priority = "Online", "Offline", true, 1
		add(s, csValue{Value: b2f(status)})
		s = mk("Enabled", "digital", "Enabled")
		s.StateTrue, s.StateFalse = "Enabled", "Disabled"
		add(s, csValue{Value: b2f(c.Enabled)})
		s = mk("LinkUp", "digital", "Link")
		s.StateTrue, s.StateFalse = "Link Up", "Link Down"
		if c.IsConnected != nil {
			add(s, csValue{Value: b2f(*c.IsConnected)})
		} else {
			add(s, csValue{Invalid: true})
		}
		s = mk("StatsAge", "analog", "Statistics Age")
		s.Unit, s.NoHistory, s.Deadband = "s", true, 5
		add(s, csValue{Value: math.Round(statsAge), Invalid: !c.HasStatsTime})
		add(mk("ActiveNode", "string", "Active Node"), csValue{Str: activeNode, Invalid: activeNode == ""})
		s = mk("TagCount", "analog", "Tag Count")
		s.Unit = "Tags"
		add(s, csValue{Value: cnt.Total, Invalid: !hasCnt && snap.Counts != nil})
		s = mk("InvalidTags", "analog", "Invalid Tags")
		s.Unit = "Tags"
		add(s, csValue{Value: cnt.Invalid, Invalid: !hasCnt && snap.Counts != nil})
		s = mk("Health", "analog", "Health")
		s.Unit, s.Deadband = "%", 1
		add(s, csValue{Value: health, Invalid: !hasCnt || cnt.Total == 0})

		if mirror {
			names := make([]string, 0, len(c.NumStats))
			for k := range c.NumStats {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, k := range names {
				s = mk("Stats."+csSanitize(k), "analog", "Stats "+k)
				s.NoHistory = true
				add(s, csValue{Value: c.NumStats[k]})
			}
		}
	}
	for id := range cs.statsStall {
		if !seenConn[id] {
			delete(cs.statsStall, id)
		}
	}

	sum := func(point, text, unit string, v float64) {
		add(csSpec{Kind: csKindSummary, Ref: "", Point: point, Tag: "_System.Comm." + point,
			Type: "analog", Group2: "Comm", Group3: "", Text: text, Unit: unit}, csValue{Value: v})
	}
	sum("DriversDown", "Drivers Down", "Drivers", driversDown)
	sum("ConnectionsDown", "Connections Down", "Connections", connsDown)
	sum("ConnectionsTotal", "Connections Total", "Connections", float64(len(snap.Connections)))
	return pts
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// ---------------------------------------------------------------------
// writing
// ---------------------------------------------------------------------

type csExisting struct {
	ID    any
	Tag   string
	Value csValue
}

func (cs *commStatus) readExisting(ctx context.Context, rt *mongo.Collection) (map[string]csExisting, error) {
	cur, err := rt.Find(ctx, bson.D{{Key: commStatusField, Value: bson.D{{Key: "$exists", Value: true}}}},
		options.Find().SetProjection(bson.D{
			{Key: "_id", Value: 1}, {Key: "tag", Value: 1}, {Key: commStatusField, Value: 1},
			{Key: "value", Value: 1}, {Key: "valueString", Value: 1}, {Key: "invalid", Value: 1},
		}))
	if err != nil {
		return nil, err
	}
	var docs []bson.M
	if err := cur.All(ctx, &docs); err != nil {
		return nil, err
	}
	out := map[string]csExisting{}
	for _, d := range docs {
		m := csSubDoc(d[commStatusField])
		k := csStr(m["kind"]) + "|" + csStr(m["ref"]) + "|" + csStr(m["point"])
		out[k] = csExisting{
			ID:  d["_id"],
			Tag: csStr(d["tag"]),
			Value: csValue{
				Value:   csNum(d["value"]),
				Str:     csStr(d["valueString"]),
				Invalid: csBool(d["invalid"], false),
			},
		}
	}
	return out, nil
}

func (cs *commStatus) apply(ctx context.Context, rt *mongo.Collection, pts []csPoint) {
	existing, err := cs.readExisting(ctx, rt)
	if err != nil {
		M.Inc(CntErrors, 1)
		Log(LogLevelMin, "Comm status tags - error reading tags: %v", err)
		return
	}
	now := time.Now()
	wanted := map[string]bool{}
	created := 0

	for _, p := range pts {
		k := p.Spec.key()
		wanted[k] = true
		ex, found := existing[k]
		if !found {
			id, err := cs.create(ctx, rt, p.Spec)
			if err != nil {
				M.Inc(CntErrors, 1)
				Log(LogLevelMin, "Comm status tags - error creating %s: %v", p.Spec.Tag, err)
				continue
			}
			created++
			ex = csExisting{ID: id, Tag: p.Spec.Tag, Value: csValue{Invalid: true}}
		}
		if ex.Tag != p.Spec.Tag {
			if _, err := rt.UpdateOne(ctx, bson.D{{Key: "_id", Value: ex.ID}}, bson.D{{Key: "$set",
				Value: csNamingFields(p.Spec)}}); err != nil {
				M.Inc(CntErrors, 1)
				Log(LogLevelMin, "Comm status tags - error renaming %s to %s: %v", ex.Tag, p.Spec.Tag, err)
			} else {
				Log(LogLevelMin, "Comm status tags - renamed %s to %s", ex.Tag, p.Spec.Tag)
			}
		}

		// a new tag is always written once, so it leaves its creation state
		last, known := cs.written[k]
		if found && known && !csChanged(p.Spec, last, p.Val) {
			continue
		}
		if found && !known && !csChanged(p.Spec, ex.Value, p.Val) {
			cs.written[k] = ex.Value
			continue
		}
		if err := cs.write(ctx, rt, ex.ID, p, now); err != nil {
			M.Inc(CntErrors, 1)
			Log(LogLevelMin, "Comm status tags - error updating %s: %v", p.Spec.Tag, err)
			continue
		}
		cs.written[k] = p.Val
	}
	if created > 0 {
		Log(LogLevelMin, "Comm status tags - created %d tags", created)
	}

	var stale bson.A
	for k, ex := range existing {
		if !wanted[k] {
			stale = append(stale, ex.ID)
			delete(cs.written, k)
			Log(LogLevelMin, "Comm status tags - removing %s", ex.Tag)
		}
	}
	if len(stale) > 0 {
		if _, err := rt.DeleteMany(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: stale}}}}); err != nil {
			M.Inc(CntErrors, 1)
			Log(LogLevelMin, "Comm status tags - error removing tags: %v", err)
		}
	}
}

// csChanged reports whether v must be written over last.
func csChanged(s csSpec, last, v csValue) bool {
	if last.Invalid != v.Invalid {
		return true
	}
	switch s.Type {
	case "string":
		return last.Str != v.Str
	case "analog":
		if s.Deadband > 0 {
			return math.Abs(last.Value-v.Value) >= s.Deadband
		}
	}
	return last.Value != v.Value
}

func (cs *commStatus) write(ctx context.Context, rt *mongo.Collection, id any, p csPoint, now time.Time) error {
	sdu := bson.D{
		{Key: "valueAtSource", Value: p.Val.Value},
		{Key: "valueStringAtSource", Value: p.Val.Str},
		{Key: "invalidAtSource", Value: p.Val.Invalid},
		{Key: "timeTag", Value: now},
	}
	if p.Spec.Type == "digital" && p.Spec.Alarm {
		// a source time tag makes the change an SOE event
		sdu = append(sdu,
			bson.E{Key: "timeTagAtSource", Value: now},
			bson.E{Key: "timeTagAtSourceOk", Value: true})
	} else {
		sdu = append(sdu,
			bson.E{Key: "timeTagAtSource", Value: nil},
			bson.E{Key: "timeTagAtSourceOk", Value: false})
	}
	_, err := rt.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "sourceDataUpdate", Value: sdu}}}})
	return err
}

func csNamingFields(s csSpec) bson.D {
	desc := "_System~" + s.Group2
	if s.Group3 != "" {
		desc += "~" + s.Group3
	}
	desc += "~" + s.Text
	return bson.D{
		{Key: "tag", Value: s.Tag},
		{Key: "description", Value: desc},
		{Key: "ungroupedDescription", Value: s.Text},
		{Key: "group3", Value: s.Group3},
	}
}

// csTagDocument renders a new generated tag, on the same base as the
// special system tags.
func csTagDocument(id float64, s csSpec) bson.D {
	doc := specialTagBase(id, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	set := func(k string, v any) {
		for i := range doc {
			if doc[i].Key == k {
				doc[i].Value = v
				return
			}
		}
		doc = append(doc, bson.E{Key: k, Value: v})
	}
	for _, e := range csNamingFields(s) {
		set(e.Key, e.Value)
	}
	set("group2", s.Group2)
	set("type", s.Type)
	set("unit", s.Unit)
	set("alarmDisabled", !s.Alarm)
	if s.Priority > 0 {
		set("priority", s.Priority)
	}
	if s.NoHistory {
		set("historianPeriod", -1.0)
	}
	if s.Type == "analog" {
		set("historianDeadBand", s.Deadband)
		set("frozenDetectTimeout", 0.0)
	}
	set("stateTextTrue", s.StateTrue)
	set("stateTextFalse", s.StateFalse)
	set("eventTextTrue", s.StateTrue)
	set("eventTextFalse", s.StateFalse)
	set("valueString", "")
	// a null timeTag makes the processor accept the first update even when
	// it carries the creation value and quality
	set("timeTag", nil)
	set(commStatusField, bson.D{
		{Key: "kind", Value: s.Kind},
		{Key: "ref", Value: s.Ref},
		{Key: "point", Value: s.Point},
	})
	return doc
}

func (cs *commStatus) create(ctx context.Context, rt *mongo.Collection, s csSpec) (float64, error) {
	if cs.nextKey == 0 {
		if err := cs.loadNextKey(ctx, rt); err != nil {
			return 0, err
		}
	}
	id := cs.nextKey
	_, err := rt.InsertOne(ctx, csTagDocument(id, s))
	if err != nil {
		// a key or tag name clash: re-read the lowest key next time
		cs.nextKey = 0
		return 0, err
	}
	cs.nextKey--
	return id, nil
}

func (cs *commStatus) loadNextKey(ctx context.Context, rt *mongo.Collection) error {
	var doc bson.M
	err := rt.FindOne(ctx, bson.D{{Key: "_id", Value: bson.D{{Key: "$lte", Value: commStatusKeyCeiling}}}},
		options.FindOne().SetSort(bson.D{{Key: "_id", Value: 1}}).
			SetProjection(bson.D{{Key: "_id", Value: 1}})).Decode(&doc)
	lowest := commStatusKeyCeiling
	if err == nil {
		if f, ok := csNumOK(doc["_id"]); ok && f < lowest {
			lowest = f
		}
	} else if err != mongo.ErrNoDocuments {
		return err
	}
	cs.nextKey = lowest - 1
	return nil
}

func (cs *commStatus) invalidateAll(ctx context.Context, rt *mongo.Collection) {
	existing, err := cs.readExisting(ctx, rt)
	if err != nil {
		M.Inc(CntErrors, 1)
		Log(LogLevelMin, "Comm status tags - error reading tags: %v", err)
		return
	}
	now := time.Now()
	for _, ex := range existing {
		p := csPoint{Spec: csSpec{Type: "analog"}, Val: csValue{Value: ex.Value.Value, Str: ex.Value.Str, Invalid: true}}
		if err := cs.write(ctx, rt, ex.ID, p, now); err != nil {
			M.Inc(CntErrors, 1)
			Log(LogLevelMin, "Comm status tags - error invalidating %s: %v", ex.Tag, err)
		}
	}
	cs.written = map[string]csValue{}
}

// ---------------------------------------------------------------------
// loose BSON decoding helpers
// ---------------------------------------------------------------------

func csNumOK(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	case bson.Decimal128:
		f, err := csParseDecimal(x)
		return f, err == nil
	}
	return 0, false
}

func csParseDecimal(d bson.Decimal128) (float64, error) {
	var f float64
	_, err := fmt.Sscan(d.String(), &f)
	return f, err
}

func csNum(v any) float64 {
	f, _ := csNumOK(v)
	return f
}

func csStr(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func csBool(v any, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

func csTime(v any) (time.Time, bool) {
	switch x := v.(type) {
	case bson.DateTime:
		return x.Time(), true
	case time.Time:
		return x, true
	}
	return time.Time{}, false
}

func csID(v any) string {
	switch x := v.(type) {
	case bson.ObjectID:
		return x.Hex()
	case string:
		return x
	}
	if f, ok := csNumOK(v); ok {
		return jsNumberToString(f)
	}
	return fmt.Sprint(v)
}

func csSubDoc(v any) map[string]any {
	switch x := v.(type) {
	case bson.M:
		return x
	case map[string]any:
		return x
	case bson.D:
		m := make(map[string]any, len(x))
		for _, e := range x {
			m[e.Key] = e.Value
		}
		return m
	}
	return nil
}
