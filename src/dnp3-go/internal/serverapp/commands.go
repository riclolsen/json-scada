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

// Controls received from a master. Port of MyCommandHandler, matching the
// corrected C++ server of v0.1.1: the command is queued on commandsQueue and
// routed by the tag's own protocolSource* fields, because this server is the
// command's destination, not the connection that delivers it to the field.

package serverapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dnp3-go/internal/dnp3sec"
	"dnp3-go/internal/dnp3util"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/outstation"
	"github.com/dscsystems/go-dnp3/secevent"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// commandHandler answers the control function codes for one connection.
//
// Its methods run on the outstation's session goroutine, so each does one
// indexed MongoDB lookup and one insert and returns; nothing here waits on the
// field.
//
// Every refusal is reported as a security event with its reason (refuse); a
// connection with commands disabled refuses every control here with
// NOT_SUPPORTED, the answer of the library's RejectingCommandHandler.
type commandHandler struct {
	engine *Engine
	conn   *Connection
	// disabled is commandsEnabled false.
	disabled bool
}

// operateName names an operate for the security events.
func operateName(t outstation.OperateType) string {
	switch t {
	case outstation.OperateDirectNoAck:
		return "DIRECT_OPERATE_NR"
	case outstation.OperateSelected:
		return "OPERATE"
	default:
		return "DIRECT_OPERATE"
	}
}

// peer is the master connected now, for the security events.
func (h *commandHandler) peer() secevent.PeerInfo {
	if h.conn == nil {
		return secevent.PeerInfo{}
	}
	if st := h.conn.Station(); st != nil {
		p, _ := st.PeerInfo()
		return p
	}
	return secevent.PeerInfo{}
}

// refusal is why a control is refused: its status, event kind and reason.
type refusal struct {
	status dnp3.CommandStatus
	kind   string
	why    string
}

var accepted = refusal{status: dnp3.CommandSuccess}

// refuse reports a refused control and returns its status.
func (h *commandHandler) refuse(service string, digital bool, index uint16, value string, tag bson.M, r refusal) dnp3.CommandStatus {
	if h.conn == nil || h.conn.Sec == nil {
		return r.status
	}
	group := dnp3util.GroupAnalogOutputBlock
	if digital {
		group = dnp3util.GroupCROBCommand
	}
	h.conn.Sec.Refused(h.peer(), dnp3sec.Refusal{Kind: r.kind, Service: service, Group: group, Index: int(index),
		Tag: jsmongo.GetString(tag, "tag", ""), Value: value, Status: r.status.String(), Why: r.why})
	return r.status
}

func crobText(c dnp3.ControlRelayOutputBlock) string {
	return fmt.Sprintf("code=0x%02X count=%d on=%dms off=%dms", uint8(c.Code), c.Count, c.OnTime, c.OffTime)
}

// HasOutput implements outstation.OutputPoints: the handler says every output
// exists, and answers for the ones that do not itself.
//
// Controls and output statuses are separate address spaces here. A connection
// may publish four binary output statuses and have control points far beyond
// them, and the outstation database knows only about the statuses. Left to
// itself, go-dnp3 v0.8.0 refuses any control whose index is past the status
// count, answering NOT_SUPPORTED before the handler has looked for the command
// tag. The tag lookup (lookup) is what decides whether an output exists, and
// it gives the same NOT_SUPPORTED when it does not.
func (h *commandHandler) HasOutput(_ dnp3.PointType, _ uint16) bool { return true }

// SelectCROB answers whether the outstation would accept the control. It must
// not operate anything.
func (h *commandHandler) SelectCROB(index uint16, c dnp3.ControlRelayOutputBlock) dnp3.CommandStatus {
	tag, r := h.lookup(true, index, "ControlRelayOutputBlock")
	if r.status != dnp3.CommandSuccess {
		return h.refuse("SELECT", true, index, crobText(c), tag, r)
	}
	return dnp3.CommandSuccess
}

// crobValue is the command value a control code asks for: close is 1 and trip
// is 0, and without a trip/close code the operation type decides.
//
// The trip/close code is bits 7-6 of the control code, 1 for close (0x40) and
// 2 for trip (0x80). go-dnp3 before v0.5.3 had the two constants transposed,
// so a close sent by any other implementation was read here as a trip;
// TestCROBValueWireCodes pins the raw octets.
func crobValue(code dnp3.ControlCode) float64 {
	switch {
	case code.IsClose():
		return 1.0
	case code.IsTrip():
		return 0.0
	}
	switch code.OpType() {
	case dnp3.ControlLatchOn, dnp3.ControlPulseOn:
		return 1.0
	default:
		return 0.0
	}
}

// OperateCROB queues a digital command for the connection that owns the tag.
func (h *commandHandler) OperateCROB(index uint16, c dnp3.ControlRelayOutputBlock, op outstation.OperateType) dnp3.CommandStatus {
	received := time.Now() // the unnecessary-command check compares with the state before this
	service, text := operateName(op), crobText(c)
	tag, r := h.lookup(true, index, "ControlRelayOutputBlock")
	if r.status != dnp3.CommandSuccess {
		return h.refuse(service, true, index, text, tag, r)
	}
	if c.Code.OpType() == dnp3.ControlNUL {
		jslog.Log(jslog.LevelBasic, "%s - ControlRelayOutputBlock index: %d - OperationType: NUL",
			h.conn.Name, index)
		return h.refuse(service, true, index, text, tag, refusal{dnp3.CommandFormatError, dnp3sec.KindCmdRejected,
			"control with operation type NUL"})
	}
	// Both coil bits set is the reserved combination and means nothing.
	if c.Code&(dnp3.ControlTrip|dnp3.ControlClose) == dnp3.ControlTrip|dnp3.ControlClose {
		jslog.Log(jslog.LevelBasic, "%s - ControlRelayOutputBlock index: %d - Invalid TripCloseCode!",
			h.conn.Name, index)
		return h.refuse(service, true, index, text, tag, refusal{dnp3.CommandFormatError, dnp3sec.KindCmdRejected,
			"control with the reserved trip/close code (both bits set)"})
	}

	dest, ok := h.destinationOf(tag, dnp3util.GroupCROBCommand, index)
	if !ok {
		return h.refuse(service, true, index, text, tag, refusal{dnp3.CommandNotSupported, dnp3sec.KindCmdObjectUnknown,
			"the command tag has no destination at this index"})
	}

	// Close means on and trip means off. A CROB carrying neither coil is
	// perfectly ordinary — a plain LATCH_ON or PULSE_ON operating a single
	// output — so the operation type decides when the trip/close field is
	// absent (deviation D23).
	//
	// The C++ server reads the value from the trip/close field alone and
	// rejects anything else as a format error, which means a latch command
	// never reaches the field. Its own client driver auto-creates command tags
	// with duration 3, LATCH 1=ON 0=OFF, so the two halves of the product could
	// not operate a point through each other on their default settings.
	value := crobValue(c.Code)
	if dest.KConv1 == -1.0 {
		value = 1.0 - value
	}

	if !h.queue(tag, value, "") {
		return h.refuse(service, true, index, text, tag, refusal{dnp3.CommandDownstreamFail, dnp3sec.KindCmdRejected,
			"the command could not be queued"})
	}
	h.conn.Sec.Forwarded(h.peer(), jsmongo.GetString(tag, "tag", ""), jsmongo.GetDouble(tag, "_id", 0), value,
		dnp3util.GroupCROBCommand, int(index), received)
	return dnp3.CommandSuccess
}

// SelectAnalog answers whether the outstation would accept the setpoint.
func (h *commandHandler) SelectAnalog(index uint16, v outstation.AnalogOutput) dnp3.CommandStatus {
	tag, r := h.lookup(false, index, "AnalogOutput")
	if r.status != dnp3.CommandSuccess {
		return h.refuse("SELECT", false, index, trimFloat(v.Value), tag, r)
	}
	return dnp3.CommandSuccess
}

// OperateAnalog queues an analog command for the connection that owns the tag.
func (h *commandHandler) OperateAnalog(index uint16, v outstation.AnalogOutput, op outstation.OperateType) dnp3.CommandStatus {
	service, text := operateName(op), trimFloat(v.Value)
	tag, r := h.lookup(false, index, "AnalogOutput")
	if r.status != dnp3.CommandSuccess {
		return h.refuse(service, false, index, text, tag, r)
	}
	dest, ok := h.destinationOf(tag, dnp3util.GroupAnalogOutputBlock, index)
	if !ok {
		return h.refuse(service, false, index, text, tag, refusal{dnp3.CommandNotSupported, dnp3sec.KindCmdObjectUnknown,
			"the command tag has no destination at this index"})
	}

	value := scaled(v.Value, dest)
	if !h.queue(tag, value, formatValue(value)) {
		return h.refuse(service, false, index, text, tag, refusal{dnp3.CommandDownstreamFail, dnp3sec.KindCmdRejected,
			"the command could not be queued"})
	}
	return dnp3.CommandSuccess
}

// lookup finds the command tag distributed at this object address.
//
// Digital commands are distributed as CROB (group 12) and analog commands as
// analog output blocks (group 41), so the group and the tag type must agree.
// The conditions are combined with $elemMatch so that they hold for one entry
// of the array rather than across entries.
//
// A refused control comes back with its status and the reason, for the
// security event: an index with no command tag is an unknown object, or a
// control of the wrong type when the other family has a command tag there.
func (h *commandHandler) lookup(digital bool, index uint16, what string) (bson.M, refusal) {
	if h.disabled {
		jslog.Log(jslog.LevelBasic, "%s - %s index: %d refused: commands are disabled", h.conn.Name, what, index)
		return nil, refusal{dnp3.CommandNotSupported, dnp3sec.KindCmdRejected, "commands are disabled on the connection"}
	}
	unavailable := refusal{dnp3.CommandDownstreamFail, dnp3sec.KindCmdRejected, "the database is not available"}
	if h.engine == nil {
		return nil, unavailable
	}
	db := h.engine.commandDB()
	if db == nil {
		return nil, unavailable
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	doc, err := h.findCommandTag(ctx, db, digital, index)
	if errors.Is(err, mongo.ErrNoDocuments) {
		jslog.Log(jslog.LevelBasic, "%s - Tag not found in the database for %s index: %d",
			h.conn.Name, what, index)
		r := refusal{dnp3.CommandNotSupported, dnp3sec.KindCmdObjectUnknown, "no command tag at this index"}
		if other, oerr := h.findCommandTag(ctx, db, !digital, index); oerr == nil {
			r.kind = dnp3sec.KindCmdWrongType
			r.why = fmt.Sprintf("%s to an index where the command tag %s takes the other control type",
				what, jsmongo.GetString(other, "tag", ""))
		}
		return nil, r
	}
	if err != nil {
		// The database could not answer, which says nothing about whether the
		// output exists. NOT_SUPPORTED would tell the master it never will.
		jslog.Log(jslog.LevelBasic, "%s - Cannot look up the tag for %s index: %d - %v",
			h.conn.Name, what, index, err)
		return nil, unavailable
	}
	if !jsmongo.GetBool(doc, "enabled", true) {
		jslog.Log(jslog.LevelBasic, "%s - Tag disabled in the database for %s index: %d",
			h.conn.Name, what, index)
		return doc, refusal{dnp3.CommandBlocked, dnp3sec.KindCmdRejected, "the command tag is disabled"}
	}
	return doc, accepted
}

// findCommandTag reads the command tag of a control family at an index:
// digital (CROB, group 12) or analog (analog output, group 41).
func (h *commandHandler) findCommandTag(ctx context.Context, db *mongo.Database, digital bool, index uint16) (bson.M, error) {
	tagType, group := "analog", dnp3util.GroupAnalogOutputBlock
	if digital {
		tagType, group = "digital", dnp3util.GroupCROBCommand
	}
	var doc bson.M
	err := db.Collection(jsmongo.RealtimeDataCollectionName).FindOne(ctx, bson.M{
		"origin": "command",
		"type":   tagType,
		"protocolDestinations": bson.M{"$elemMatch": bson.M{
			"protocolDestinationConnectionNumber": h.conn.ProtocolConnectionNumber,
			"protocolDestinationCommonAddress":    group,
			"protocolDestinationObjectAddress":    int(index),
		}},
	}).Decode(&doc)
	return doc, err
}

// destinationOf picks the destination entry the command arrived on, so that the
// conversion factors applied are the ones configured for this address.
func (h *commandHandler) destinationOf(tag bson.M, group int, index uint16) (Destination, bool) {
	for _, d := range DestinationsOf(tag) {
		if d.ConnectionNumber == h.conn.ProtocolConnectionNumber &&
			d.CommonAddress == group &&
			d.ObjectAddress == int(index) {
			return d, true
		}
	}
	return Destination{}, false
}

// queue inserts the command on commandsQueue, routed by the tag's own
// protocolSource* fields.
func (h *commandHandler) queue(tag bson.M, value float64, valueString string) bool {
	db := h.engine.commandDB()
	if db == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := db.Collection(jsmongo.CommandsQueueCollectionName).InsertOne(ctx, bson.M{
		"protocolSourceConnectionNumber": jsmongo.GetDouble(tag, "protocolSourceConnectionNumber", 0),
		"protocolSourceCommonAddress":    jsmongo.GetDouble(tag, "protocolSourceCommonAddress", 0),
		"protocolSourceObjectAddress":    jsmongo.GetDouble(tag, "protocolSourceObjectAddress", 0),
		"protocolSourceASDU":             jsmongo.GetDouble(tag, "protocolSourceASDU", 0),
		"protocolSourceCommandDuration":  jsmongo.GetDouble(tag, "protocolSourceCommandDuration", 0),
		"protocolSourceCommandUseSBO":    jsmongo.GetBool(tag, "protocolSourceCommandUseSBO", false),
		"pointKey":                       jsmongo.GetDouble(tag, "_id", 0),
		"tag":                            jsmongo.GetString(tag, "tag", ""),
		"value":                          value,
		"valueString":                    valueString,
		"originatorUserName":             "DNP3 Server Driver",
		"originatorIpAddress":            "",
		"timeTag":                        jsmongo.Now(),
	})
	if err != nil {
		jslog.Log(jslog.LevelNoLog, "%s - Error queueing command for tag: %s - %v",
			h.conn.Name, jsmongo.GetString(tag, "tag", ""), err)
		return false
	}
	jslog.Log(jslog.LevelBasic, "%s - Command queued for tag: %s Value: %v",
		h.conn.Name, jsmongo.GetString(tag, "tag", ""), value)
	return true
}

// formatValue renders a setpoint for valueString.
func formatValue(v float64) string {
	return trimFloat(v)
}
