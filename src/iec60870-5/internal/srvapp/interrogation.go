/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - interrogation
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Port of InterrogationHandler.cs: answers general (QOI 20) and group
 * (QOI 21..36) interrogations from the realtimeData collection.
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

package srvapp

import (
	"context"
	"sort"
	"strconv"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"go.mongodb.org/mongo-driver/v2/bson"

	"iec60870-5/internal/conv"
	"iec60870-5/internal/model"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
)

func formatValue(v float64) string {
	return strconv.FormatFloat(v, 'G', -1, 64)
}

// destASDUToBase maps a destination ASDU (possibly a time-tagged variant)
// to the untagged base type sent in interrogation responses.
func destASDUToBase(a int) int {
	switch a {
	case 1, 30:
		return 1
	case 3, 31:
		return 3
	case 5, 32:
		return 5
	case 9, 34:
		return 9
	case 11, 35:
		return 11
	case 13, 36:
		return 13
	}
	return 0 // unsupported for interrogation (C# parity: skipped)
}

// groupMatches reports whether a destination belongs to the interrogated
// group: QOI 20 (station) takes every destination but those marked group -1,
// QOI 21..36 those marked with the QOI or with QOI-20.
func groupMatches(q int, g float64) bool {
	if q == 20 {
		return g != -1
	}
	return int(g) == q || int(g) == q-20
}

// Interrogation answers a station or group interrogation request.
// replyTo is the session (cs104) or the server itself (cs101).
func (e *Engine) Interrogation(srv *Conn, replyTo asdu.Connect, pack *asdu.ASDU, qoi asdu.QualifierOfInterrogation) {
	conName := srv.Cfg.Name + " - "
	q := int(qoi)
	out := e.reply(replyTo)
	jslog.Log(jslog.LevelBasic, "%s[%d] Group interrogation BEGIN", conName, q)
	// a station interrogation answers the points of its common address; only
	// the global address takes every one of the connection
	global := isGlobalCA(replyTo, pack.CommonAddr)
	if !global && !e.knownCA(srv, int(pack.CommonAddr)) {
		srv.Sec.UnknownCA(replyTo, pack, "interrogation")
		e.negativeCon(replyTo, pack)
		jslog.Log(jslog.LevelBasic, "%s[%d] Group interrogation of unknown common address %d", conName, q, int(pack.CommonAddr))
		return
	}
	ca := int(pack.CommonAddr)

	// connection and group must hold on the same destination entry
	var groupCond interface{}
	if q == 20 {
		groupCond = bson.M{"$ne": -1}
	} else {
		groupCond = bson.M{"$in": bson.A{q, q - 20}}
	}
	elem := bson.M{
		"protocolDestinationConnectionNumber": srv.Cfg.ProtocolConnectionNumber,
		"protocolDestinationGroup":            groupCond,
	}
	if !global {
		elem["protocolDestinationCommonAddress"] = jsmongo.AddrMatch(ca)
	}
	filter := bson.M{
		"origin":               bson.M{"$ne": "command"},
		"protocolDestinations": bson.M{"$elemMatch": elem},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cur, err := e.DB().Collection(jsmongo.RealtimeDataCollectionName).Find(ctx, filter)
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s", "Exception on Interrogation: "+err.Error())
		e.negativeCon(replyTo, pack)
		return
	}
	var list []model.RtDataPoint
	if err := cur.All(ctx, &list); err != nil {
		jslog.Log(jslog.LevelBasic, "%s", "Exception on Interrogation: "+err.Error())
		e.negativeCon(replyTo, pack)
		return
	}

	// one entry per matching destination: a point may be mapped more than
	// once on the same connection (several addresses)
	type item struct {
		p   *model.RtDataPoint
		dst *model.ProtocolDestination
	}
	var items []item
	for i := range list {
		p := &list[i]
		for j := range p.ProtocolDestinations {
			d := &p.ProtocolDestinations[j]
			if int(d.ConnectionNumber) == int(srv.Cfg.ProtocolConnectionNumber) && groupMatches(q, d.Group) &&
				(global || int(d.CommonAddress) == ca) {
				items = append(items, item{p, d})
			}
		}
	}
	// order by destination ASDU, then common address, so that ASDUs carry
	// as many objects as possible
	sort.SliceStable(items, func(i, j int) bool {
		di, dj := items[i].dst, items[j].dst
		if di.ASDU != dj.ASDU {
			return di.ASDU < dj.ASDU
		}
		return di.CommonAddress < dj.CommonAddress
	})

	jslog.Log(jslog.LevelBasic, "%s[%d] Group request, %d objects to send.", conName, q, len(items))
	e.positiveCon(replyTo, pack)

	coa := asdu.CauseOfTransmission{Cause: asdu.Cause(q)}
	var batch []*conv.InfoObject
	lastType := asdu.TypeID(0)
	lastCa := -1

	flush := func() {
		if len(batch) == 0 {
			return
		}
		jslog.Log(jslog.LevelBasic, "%s[%d] Send ASDU TI:%d CA:%d with %d objects.",
			conName, q, lastType, lastCa, len(batch))
		if err := conv.SendInfoBatch(out, coa, asdu.CommonAddr(lastCa), lastType, batch); err != nil {
			jslog.Log(jslog.LevelBasic, "%s[%d] Error sending ASDU: %s", conName, q, err.Error())
		}
		batch = nil
	}

	for _, it := range items {
		jslog.Log(jslog.LevelDetailed, "%s[%d] %s %s Key %v", conName, q, it.p.Tag, formatValue(it.p.Value), it.p.ID)
		base := destASDUToBase(int(it.dst.ASDU))
		if base == 0 {
			continue // unsupported destination type for interrogation
		}
		obj := destInfoObj(it.p, it.dst, base, nil)
		if obj == nil {
			continue
		}
		// batch break on type or CA change, or 30 objects
		if len(batch) > 0 && (obj.TypeID != lastType || int(it.dst.CommonAddress) != lastCa || len(batch) >= 30) {
			flush()
		}
		lastType = obj.TypeID
		lastCa = int(it.dst.CommonAddress)
		batch = append(batch, obj)
	}
	flush()
	_ = pack.SendReplyMirror(out, asdu.ActivationTerm)
	jslog.Log(jslog.LevelBasic, "%s[%d] Group interrogation END", conName, q)
}
