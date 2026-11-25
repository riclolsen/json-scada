/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package sunspecapp

// Automatic tag creation from the discovered model chain: every point of
// every decoded model instance (repeating groups included) becomes a
// supervised tag, and every writable top-level point also a command tag.
// Scale factors and padding get no tag, nor, unless autoCreateUnimplemented
// is set, the points the device reports as not implemented. Existing tags
// (same connection, unit and address) are left alone.

import (
	"context"
	"fmt"
	"maps"
	"modbus-go/internal/drv"
	"strconv"
	"strings"
	"time"

	"github.com/otfabric/go-sunspec"
	"github.com/otfabric/go-sunspec/registry"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
	"github.com/riclolsen/json-scada/src/go-common/jstags"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ExistingTag is a tag the connection already has.
type ExistingTag struct {
	ID                  any
	CommandOfSupervised float64
}

// TagStore is what auto creation needs from realtimeData.
type TagStore interface {
	// Existing returns the connection's tags keyed by tagKey, and the tag
	// names in use.
	Existing(ctx context.Context, connNumber float64) (map[string]ExistingTag, map[string]bool, error)
	NextKey(ctx context.Context) float64
	Insert(ctx context.Context, doc bson.M) error
	// SetCommandOf links a supervised tag to its command tag.
	SetCommandOf(ctx context.Context, id any, cmdID float64) error
}

// tagKey identifies a tag by unit, address and whether it is a command.
func tagKey(unit int, addr string, command bool) string {
	return fmt.Sprintf("%d|%s|%v", unit, addr, command)
}

// MongoTags is the realtimeData collection; keys come from the connection's
// _id range.
type MongoTags struct {
	Coll       *mongo.Collection
	ConnNumber int
	Keys       *jstags.KeyAllocator
}

// Existing implements TagStore.
func (m MongoTags) Existing(ctx context.Context, connNumber float64) (map[string]ExistingTag, map[string]bool, error) {
	docs, err := jsmongo.FindAll(ctx, m.Coll, bson.M{"protocolSourceConnectionNumber": connNumber})
	if err != nil {
		return nil, nil, err
	}
	byKey, names := map[string]ExistingTag{}, map[string]bool{}
	for _, d := range docs {
		names[jsmongo.GetString(d, "tag", "")] = true
		unit := int(jsmongo.GetDouble(d, "protocolSourceCommonAddress", 1))
		addr := jsmongo.GetString(d, "protocolSourceObjectAddress", "")
		if a, err := ParseAddress(addr); err == nil {
			addr = a.String()
		}
		byKey[tagKey(unit, addr, jsmongo.GetString(d, "origin", "") == "command")] = ExistingTag{
			ID: d["_id"], CommandOfSupervised: jsmongo.GetDouble(d, "commandOfSupervised", 0)}
	}
	return byKey, names, nil
}

// NextKey implements TagStore.
func (m MongoTags) NextKey(ctx context.Context) float64 {
	return m.Keys.Next(ctx, m.Coll, m.ConnNumber)
}

// Insert implements TagStore.
func (m MongoTags) Insert(ctx context.Context, doc bson.M) error {
	_, err := m.Coll.InsertOne(ctx, doc)
	return err
}

// SetCommandOf implements TagStore.
func (m MongoTags) SetCommandOf(ctx context.Context, id any, cmdID float64) error {
	_, err := m.Coll.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"commandOfSupervised": cmdID}})
	return err
}

// discoveredPoint is one point found in a decoded model.
type discoveredPoint struct {
	addr      Address
	point     *sunspec.DecodedPoint
	meta      *registry.PointMeta
	groupPath string // "module[2]", "" at the top level
}

// walkPoints lists the points of a decoded model with their schema.
func walkPoints(m *sunspec.DecodedModel, occurrence int) []discoveredPoint {
	if m == nil || m.Group == nil || m.Schema == nil || m.Schema.Group == nil {
		return nil
	}
	var out []discoveredPoint
	var walk func(g *sunspec.DecodedGroup, gm *registry.GroupMeta, refs []GroupRef)
	walk = func(g *sunspec.DecodedGroup, gm *registry.GroupMeta, refs []GroupRef) {
		path := make([]string, len(refs))
		for i, r := range refs {
			path[i] = fmt.Sprintf("%s[%d]", r.Name, r.Index)
		}
		for i := range g.Points {
			p := &g.Points[i]
			if len(refs) == 0 && (p.Name == "ID" || p.Name == "L") {
				continue // the model header
			}
			var pm *registry.PointMeta
			if gm != nil {
				pm = gm.Point(p.Name)
			}
			out = append(out, discoveredPoint{
				addr:  Address{ModelID: m.ModelID, Occurrence: occurrence, Groups: append([]GroupRef(nil), refs...), Point: p.Name},
				point: p, meta: pm, groupPath: strings.Join(path, "/"),
			})
		}
		for _, c := range g.Groups {
			var cm *registry.GroupMeta
			if gm != nil {
				cm = gm.Group(c.Name)
			}
			walk(c, cm, append(refs, GroupRef{Name: c.Name, Index: c.Index}))
		}
	}
	walk(m.Group, m.Schema.Group, nil)
	return out
}

// countPoint reports a point that counts the instances of a group of the
// model, which go-sunspec refuses to write.
func countPoint(gm *registry.GroupMeta, name string) bool {
	if gm == nil {
		return false
	}
	for _, c := range gm.Groups {
		if c.CountPoint == name || countPoint(c, name) {
			return true
		}
	}
	return false
}

// UnitModels is what discovery and the first read found at one unit.
type UnitModels struct {
	Unit      int
	Discovery *sunspec.DiscoveryResult
	Models    []*sunspec.DecodedModel // parallel to Discovery.Models; nil when not decoded
	MultiUnit bool                    // the connection has more than one unit
}

// AutoCreateTags creates the missing tags of the points found at a unit and
// returns how many it created.
func AutoCreateTags(ctx context.Context, cfg Config, um UnitModels, store TagStore) (int, error) {
	if um.Discovery == nil {
		return 0, nil
	}
	ectx, cancel := context.WithTimeout(ctx, 30*time.Second)
	existing, names, err := store.Existing(ectx, cfg.ConnNumber())
	cancel()
	if err != nil {
		return 0, err
	}
	occ := occurrences(um.Discovery)
	created := 0
	for i, inst := range um.Discovery.Models {
		if !cfg.wantsModel(inst.Header.ID) {
			continue
		}
		var m *sunspec.DecodedModel
		if i < len(um.Models) {
			m = um.Models[i]
		}
		if m == nil || m.Group == nil {
			jslog.Log(jslog.LevelDetailed, "%s: unit %d model %d (%s) not decoded, no tags created", cfg.Name(), um.Unit, inst.Header.ID, inst.Name)
			continue
		}
		modelLabel := inst.Name
		if occ[i] > 1 {
			modelLabel += " #" + strconv.Itoa(occ[i])
		}
		if um.MultiUnit {
			modelLabel = fmt.Sprintf("unit %d %s", um.Unit, modelLabel)
		}
		for _, dp := range walkPoints(m, occ[i]) {
			if skippedType(dp.point.Type) || (!dp.point.Implemented && !cfg.AutoCreateUnimplemented) {
				continue
			}
			addr := dp.addr.String()
			sup, hasSup := existing[tagKey(um.Unit, addr, false)]
			writable := cfg.AutoCreateCommandTags && dp.addr.TopLevel() && dp.meta != nil && dp.meta.Access == "RW" &&
				cfg.RegisterType == RegHolding && !countPoint(inst.Schema.Group, dp.point.Name)
			cmd, hasCmd := existing[tagKey(um.Unit, addr, true)]
			name := fmt.Sprintf("%s;%d;%s", cfg.Name(), um.Unit, addr)
			kctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			if !hasSup {
				id := store.NextKey(kctx)
				err = insertUnique(kctx, store, names, name, func(tag string) bson.M {
					return newTagDoc(cfg, um.Unit, dp, modelLabel, tag, id, false)
				})
				if err == nil {
					sup.ID = id
					created++
				}
			}
			if err == nil && writable && !hasCmd {
				id := store.NextKey(kctx)
				err = insertUnique(kctx, store, names, name+";cmd", func(tag string) bson.M {
					doc := newTagDoc(cfg, um.Unit, dp, modelLabel, tag, id, true)
					doc["supervisedOfCommand"] = sup.ID
					return doc
				})
				if err == nil {
					cmd.ID = id
					created++
				}
			}
			if err == nil && writable && cmd.ID != nil {
				id := drv.ToNum(cmd.ID, true, 0)
				if sup.CommandOfSupervised != id {
					err = store.SetCommandOf(kctx, sup.ID, id)
				}
			}
			cancel()
			if err != nil {
				return created, err
			}
		}
	}
	if created > 0 {
		jslog.Log(jslog.LevelBasic, "%s: unit %d: auto-created %d tags", cfg.Name(), um.Unit, created)
	}
	return created, nil
}

// insertUnique inserts the tag document doc(name), with a "~N" suffix when
// the name is taken: names has the connection's own tags, and a tag of
// another connection with the name fails the unique index on tag.
func insertUnique(ctx context.Context, store TagStore, names map[string]bool, name string, doc func(tag string) bson.M) error {
	for range maxNameTries {
		tag := uniqueName(names, name)
		err := store.Insert(ctx, doc(tag))
		names[tag] = true
		if !mongo.IsDuplicateKeyError(err) {
			return err
		}
		jslog.Log(jslog.LevelDetailed, "tag name %s is in use, trying another", tag)
	}
	return fmt.Errorf("no free tag name for %s", name)
}

// maxNameTries bounds the names insertUnique tries.
const maxNameTries = 50

// uniqueName makes a tag name unique among names.
func uniqueName(names map[string]bool, tag string) string {
	if !names[tag] {
		return tag
	}
	for i := 2; ; i++ {
		if t := fmt.Sprintf("%s~%d", tag, i); !names[t] {
			return t
		}
	}
}

// newTagDoc is the realtimeData document of an auto-created tag.
func newTagDoc(cfg Config, unit int, dp discoveredPoint, modelLabel, tag string, id float64, command bool) bson.M {
	label, desc := dp.point.Name, ""
	if dp.meta != nil {
		if dp.meta.Label != "" {
			label = dp.meta.Label
		}
		desc = dp.meta.Desc
	}
	ungrouped := label
	if dp.groupPath != "" {
		ungrouped = dp.groupPath + " " + label
	}
	description := cfg.Name() + "~" + modelLabel + "~" + ungrouped
	origin := "supervised"
	if command {
		origin = "command"
		description += "-Command"
	}
	doc := jstags.BaseDoc()
	maps.Copy(doc, bson.M{
		"_id":                            id,
		"tag":                            tag,
		"type":                           tagType(dp.point.Type),
		"origin":                         origin,
		"description":                    description,
		"ungroupedDescription":           ungrouped,
		"group1":                         cfg.Name(),
		"group2":                         modelLabel,
		"group3":                         dp.groupPath,
		"notes":                          desc,
		"unit":                           dp.point.Units,
		"protocolSourceConnectionNumber": cfg.ConnNumber(),
		"protocolSourceCommonAddress":    float64(unit),
		"protocolSourceObjectAddress":    dp.addr.String(),
		"protocolSourceASDU":             dp.point.Type,
		"protocolSourceCommandDuration":  0.0,
		"protocolSourceCommandUseSBO":    false,
		"alarmState":                     -1.0,
		"stateTextFalse":                 "",
		"stateTextTrue":                  "",
		"eventTextFalse":                 "",
		"eventTextTrue":                  "",
		"value":                          0.0,
		"valueString":                    "",
		"valueJson":                      nil,
		"commandOfSupervised":            0.0,
		"supervisedOfCommand":            0.0,
		"invalid":                        true,
		"invalidDetectTimeout":           60000.0,
		"protocolDestinations":           bson.A{},
		"timeTag":                        jsmongo.Now(),
	})
	if command {
		doc["invalid"] = false
		doc["invalidDetectTimeout"] = 0.0
	}
	return doc
}
