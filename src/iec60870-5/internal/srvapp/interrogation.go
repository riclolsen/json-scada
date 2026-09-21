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

// Interrogation answers a station or group interrogation request.
// replyTo is the session (cs104) or the server itself (cs101).
func (e *Engine) Interrogation(srv *Conn, replyTo asdu.Connect, pack *asdu.ASDU, qoi asdu.QualifierOfInterrogation) {
	conName := srv.Cfg.Name + " - "
	q := int(qoi)
	jslog.Log(jslog.LevelBasic, "%s[%d] Group interrogation BEGIN", conName, q)

	// for QOI 20 (general interrogation) filter by all in destination
	// connection but those marked with group -1; for other groups filter
	// by those marked with this same group (value qoi or qoi-20)
	filterConn := bson.M{"protocolDestinations.protocolDestinationConnectionNumber": srv.Cfg.ProtocolConnectionNumber}
	filterCmd := bson.M{"origin": bson.M{"$ne": "command"}}
	var filter bson.M
	if q == 20 {
		filter = bson.M{"$and": bson.A{filterConn, filterCmd,
			bson.M{"protocolDestinations.protocolDestinationGroup": bson.M{"$ne": -1}}}}
	} else {
		filter = bson.M{"$and": bson.A{filterConn, filterCmd,
			bson.M{"$or": bson.A{
				bson.M{"protocolDestinations.protocolDestinationGroup": q},
				bson.M{"protocolDestinations.protocolDestinationGroup": q - 20},
			}}}}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cur, err := e.DB().Collection(jsmongo.RealtimeDataCollectionName).Find(ctx, filter)
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s", "Exception on Interrogation: "+err.Error())
		pack.Coa.IsNegative = true
		_ = pack.SendReplyMirror(replyTo, asdu.ActivationCon) // negative confirm
		return
	}
	var list []model.RtDataPoint
	if err := cur.All(ctx, &list); err != nil {
		jslog.Log(jslog.LevelBasic, "%s", "Exception on Interrogation: "+err.Error())
		pack.Coa.IsNegative = true
		_ = pack.SendReplyMirror(replyTo, asdu.ActivationCon)
		return
	}

	// destination entry for this connection (fallback: first destination)
	dstFor := func(p *model.RtDataPoint) *model.ProtocolDestination {
		if len(p.ProtocolDestinations) == 0 {
			return nil
		}
		d := &p.ProtocolDestinations[0]
		for i := range p.ProtocolDestinations {
			if int(p.ProtocolDestinations[i].ConnectionNumber) == int(srv.Cfg.ProtocolConnectionNumber) {
				d = &p.ProtocolDestinations[i]
			}
		}
		return d
	}

	// order by destination ASDU (C# parity)
	sort.SliceStable(list, func(i, j int) bool {
		di, dj := dstFor(&list[i]), dstFor(&list[j])
		if di == nil || dj == nil {
			return false
		}
		return di.ASDU < dj.ASDU
	})

	jslog.Log(jslog.LevelBasic, "%s[%d] Group request, %d objects to send.", conName, q, len(list))
	_ = pack.SendReplyMirror(replyTo, asdu.ActivationCon) // confirm positive

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
		if err := conv.SendInfoBatch(replyTo, coa, asdu.CommonAddr(lastCa), lastType, batch); err != nil {
			jslog.Log(jslog.LevelBasic, "%s[%d] Error sending ASDU: %s", conName, q, err.Error())
		}
		batch = nil
	}

	for i := range list {
		entry := &list[i]
		jslog.Log(jslog.LevelDetailed, "%s[%d] %s %s Key %v", conName, q, entry.Tag, formatValue(entry.Value), entry.ID)
		for _, dst := range entry.ProtocolDestinations {
			if int(dst.ConnectionNumber) != int(srv.Cfg.ProtocolConnectionNumber) {
				continue
			}
			base := destASDUToBase(int(dst.ASDU))
			if base == 0 {
				break // unsupported destination type: skipped (C# parity)
			}
			quality := conv.Quality{
				Invalid:     entry.Invalid,
				Substituted: entry.Substituted,
				Overflow:    entry.Overflow,
			}
			kconv1 := dst.KConv1
			if kconv1 == 0 {
				kconv1 = 1 // BsonDefaultValue(1) parity for missing field
			}
			obj := conv.BuildInfoObj(base, int(dst.ObjectAddress), entry.Value,
				false, 0, quality, nil, kconv1, dst.KConv2, entry.Transient)
			if obj == nil {
				break
			}
			// batch break on type or CA change, or 30 objects
			if len(batch) > 0 && (obj.TypeID != lastType || int(dst.CommonAddress) != lastCa || len(batch) >= 30) {
				flush()
			}
			lastType = obj.TypeID
			lastCa = int(dst.CommonAddress)
			batch = append(batch, obj)
			break
		}
	}
	flush()
	_ = pack.SendReplyMirror(replyTo, asdu.ActivationTerm)
	jslog.Log(jslog.LevelBasic, "%s[%d] Group interrogation END", conName, q)
}
