/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package clientapp

// Auto tag creation from a connection's topics list. Modbus cannot be browsed,
// so discovery is driven by operator-supplied descriptors:
//
//	"TAG_NAME|<area>:<offset>:<type[_byteorder]>[count]"
//
// e.g. "PLC1_FLOW|hr:4:float32_cdab" or "PLC1_STAT|hr:20:uint16[10]".
// Ported from src/modbus/src/client/auto-tag.ts, documents field for field.

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"modbus-go/internal/mbcore"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// TopicSpec is one parsed topic.
type TopicSpec struct {
	Tag           string
	ObjectAddress string
	Asdu          string
	Count         int
}

var (
	reCount = regexp.MustCompile(`\[(\d+)\]\s*$`)
	reBody  = regexp.MustCompile(`^([a-zA-Z-]+:\d+):(.+)$`)
)

// ParseTopic parses "TAG|area:offset:type[count]" (optionally "|byteorder").
func ParseTopic(topic string) (TopicSpec, bool) {
	parts := strings.Split(topic, "|")
	if len(parts) < 2 {
		return TopicSpec{}, false
	}
	tag := strings.TrimSpace(parts[0])
	rest := strings.TrimSpace(strings.Join(parts[1:], "|"))
	bar := strings.Split(rest, "|")
	body := strings.TrimSpace(bar[0])
	orderExtra := ""
	if len(bar) > 1 {
		orderExtra = strings.TrimSpace(bar[1])
	}
	count := 1
	if loc := reCount.FindStringSubmatchIndex(body); loc != nil {
		count, _ = strconv.Atoi(body[loc[2]:loc[3]])
		body = strings.TrimSpace(body[:loc[0]])
	}
	m := reBody.FindStringSubmatch(body)
	if m == nil {
		return TopicSpec{}, false
	}
	asdu := strings.TrimSpace(m[2])
	if orderExtra != "" && !strings.Contains(asdu, "_") {
		asdu = asdu + "_" + orderExtra
	}
	return TopicSpec{Tag: tag, ObjectAddress: m[1], Asdu: asdu, Count: count}, true
}

// TagStore is what auto creation needs from realtimeData.
type TagStore interface {
	Exists(ctx context.Context, tag string) (bool, error)
	NextKey(ctx context.Context) (float64, error)
	Insert(ctx context.Context, doc bson.D) error
}

// MongoTags is the realtimeData collection.
type MongoTags struct{ Coll *mongo.Collection }

// Exists implements TagStore.
func (m MongoTags) Exists(ctx context.Context, tag string) (bool, error) {
	err := m.Coll.FindOne(ctx, bson.M{"tag": tag}).Err()
	if errors.Is(err, mongo.ErrNoDocuments) {
		return false, nil
	}
	return err == nil, err
}

// NextKey is the highest _id plus one, as the Node.js driver allocated it.
func (m MongoTags) NextKey(ctx context.Context) (float64, error) {
	var doc bson.M
	err := m.Coll.FindOne(ctx, bson.M{}, options.FindOne().SetSort(bson.D{{Key: "_id", Value: -1}})).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 1, nil
	}
	if err != nil {
		return 0, err
	}
	return jsmongo.ToFloat(doc["_id"], 0) + 1, nil
}

// Insert implements TagStore.
func (m MongoTags) Insert(ctx context.Context, doc bson.D) error {
	_, err := m.Coll.InsertOne(ctx, doc)
	return err
}

// AutoCreateTags creates the missing tags named by cfg.Topics.
func AutoCreateTags(ctx context.Context, cfg Config, store TagStore) error {
	d := cfg.Defaults()
	created := 0
	for _, topic := range cfg.Topics {
		spec, ok := ParseTopic(topic)
		if !ok {
			jslog.Log(jslog.LevelBasic, "%s: cannot parse topic %q", cfg.Name, topic)
			continue
		}
		for i := 0; i < spec.Count; i++ {
			name := spec.Tag
			if spec.Count > 1 {
				name = fmt.Sprintf("%s[%d]", spec.Tag, i)
			}
			tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			exists, err := store.Exists(tctx, name)
			cancel()
			if err != nil {
				return err
			}
			if exists {
				continue
			}
			addr, err := mbcore.ParseObjectAddress(spec.ObjectAddress, cfg.UseModiconAddresses)
			var asdu mbcore.Asdu
			if err == nil {
				asdu, err = mbcore.ParseAsdu(spec.Asdu, addr.Area, d)
			}
			if err != nil {
				jslog.Log(jslog.LevelBasic, "%s: invalid topic %q: %v", cfg.Name, topic, err)
				break
			}
			step := 1
			if !mbcore.IsBitArea(addr.Area) && asdu.RegCount > 1 {
				step = asdu.RegCount
			}
			objectAddress := fmt.Sprintf("%s:%d", addr.Area, addr.Offset+i*step)
			typ := "analog"
			switch {
			case mbcore.IsBitArea(addr.Area) || asdu.Type == mbcore.TypeBool:
				typ = "digital"
			case asdu.Type == mbcore.TypeString:
				typ = "string"
			}
			kctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			key, err := store.NextKey(kctx)
			if err == nil {
				err = store.Insert(kctx, TagDoc(key, name, cfg.ProtocolConnectionNumber, cfg.Name, objectAddress, spec.Asdu, 1, typ))
			}
			cancel()
			if err != nil {
				return err
			}
			created++
		}
	}
	if created > 0 {
		jslog.Log(jslog.LevelBasic, "%s: auto-created %d tags", cfg.Name, created)
	}
	return nil
}

// TagDoc is the realtimeData document an auto-created tag gets. Digital tags
// get the state and event texts TRUE and FALSE, as in the other Go client
// drivers, so their value is shown; other types get empty texts.
func TagDoc(key float64, tag string, connNumber float64, connName, objectAddress, asdu string, commonAddress float64, typ string) bson.D {
	textTrue, textFalse := "", ""
	if typ == "digital" {
		textTrue, textFalse = "TRUE", "FALSE"
	}
	return bson.D{
		{Key: "_id", Value: key},
		{Key: "tag", Value: tag},
		{Key: "type", Value: typ},
		{Key: "origin", Value: "supervised"},
		{Key: "description", Value: connName + "~" + tag},
		{Key: "ungroupedDescription", Value: tag},
		{Key: "group1", Value: connName},
		{Key: "group2", Value: ""},
		{Key: "group3", Value: ""},
		{Key: "protocolSourceConnectionNumber", Value: connNumber},
		{Key: "protocolSourceCommonAddress", Value: commonAddress},
		{Key: "protocolSourceObjectAddress", Value: objectAddress},
		{Key: "protocolSourceASDU", Value: asdu},
		{Key: "protocolSourceCommandDuration", Value: 0.0},
		{Key: "protocolSourceCommandUseSBO", Value: false},
		{Key: "kconv1", Value: 1.0},
		{Key: "kconv2", Value: 0.0},
		{Key: "value", Value: 0.0},
		{Key: "valueString", Value: ""},
		{Key: "valueDefault", Value: 0.0},
		{Key: "invalid", Value: true},
		{Key: "isEvent", Value: typ == "digital"},
		{Key: "enabled", Value: true},
		{Key: "commandOfSupervised", Value: 0.0},
		{Key: "supervisedOfCommand", Value: 0.0},
		{Key: "timeTag", Value: jsmongo.Now()},
		{Key: "alarmDisabled", Value: false},
		{Key: "hiLimit", Value: nil},
		{Key: "loLimit", Value: nil},
		{Key: "stateTextFalse", Value: textFalse},
		{Key: "stateTextTrue", Value: textTrue},
		{Key: "eventTextFalse", Value: textFalse},
		{Key: "eventTextTrue", Value: textTrue},
		{Key: "unit", Value: ""},
		{Key: "priority", Value: 0.0},
	}
}
