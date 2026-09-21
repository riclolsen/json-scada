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
	"time"

	"dnp3-go/internal/dnp3util"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/outstation"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// commandHandler answers the control function codes for one connection.
//
// Its methods run on the outstation's session goroutine, so each does one
// indexed MongoDB lookup and one insert and returns; nothing here waits on the
// field.
type commandHandler struct {
	engine *Engine
	conn   *Connection
}

// SelectCROB answers whether the outstation would accept the control. It must
// not operate anything.
func (h *commandHandler) SelectCROB(index uint16, _ dnp3.ControlRelayOutputBlock) dnp3.CommandStatus {
	_, status := h.lookup(true, index, "ControlRelayOutputBlock")
	return status
}

// OperateCROB queues a digital command for the connection that owns the tag.
func (h *commandHandler) OperateCROB(index uint16, c dnp3.ControlRelayOutputBlock, _ outstation.OperateType) dnp3.CommandStatus {
	tag, status := h.lookup(true, index, "ControlRelayOutputBlock")
	if status != dnp3.CommandSuccess {
		return status
	}
	if c.Code.OpType() == dnp3.ControlNUL {
		jslog.Log(jslog.LevelBasic, "%s - ControlRelayOutputBlock index: %d - OperationType: NUL",
			h.conn.Name, index)
		return dnp3.CommandFormatError
	}
	// Both coil bits set is the reserved combination and means nothing.
	if c.Code&(dnp3.ControlTrip|dnp3.ControlClose) == dnp3.ControlTrip|dnp3.ControlClose {
		jslog.Log(jslog.LevelBasic, "%s - ControlRelayOutputBlock index: %d - Invalid TripCloseCode!",
			h.conn.Name, index)
		return dnp3.CommandFormatError
	}

	dest, ok := h.destinationOf(tag, dnp3util.GroupCROBCommand, index)
	if !ok {
		return dnp3.CommandNotSupported
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
	value := 0.0
	switch {
	case c.Code.IsClose():
		value = 1.0
	case c.Code.IsTrip():
		value = 0.0
	default:
		switch c.Code.OpType() {
		case dnp3.ControlLatchOn, dnp3.ControlPulseOn:
			value = 1.0
		case dnp3.ControlLatchOff, dnp3.ControlPulseOff:
			value = 0.0
		}
	}
	if dest.KConv1 == -1.0 {
		value = 1.0 - value
	}

	if !h.queue(tag, value, "") {
		return dnp3.CommandDownstreamFail
	}
	return dnp3.CommandSuccess
}

// SelectAnalog answers whether the outstation would accept the setpoint.
func (h *commandHandler) SelectAnalog(index uint16, _ outstation.AnalogOutput) dnp3.CommandStatus {
	_, status := h.lookup(false, index, "AnalogOutput")
	return status
}

// OperateAnalog queues an analog command for the connection that owns the tag.
func (h *commandHandler) OperateAnalog(index uint16, v outstation.AnalogOutput, _ outstation.OperateType) dnp3.CommandStatus {
	tag, status := h.lookup(false, index, "AnalogOutput")
	if status != dnp3.CommandSuccess {
		return status
	}
	dest, ok := h.destinationOf(tag, dnp3util.GroupAnalogOutputBlock, index)
	if !ok {
		return dnp3.CommandNotSupported
	}

	value := scaled(v.Value, dest)
	if !h.queue(tag, value, formatValue(value)) {
		return dnp3.CommandDownstreamFail
	}
	return dnp3.CommandSuccess
}

// lookup finds the command tag distributed at this object address.
//
// Digital commands are distributed as CROB (group 12) and analog commands as
// analog output blocks (group 41), so the group and the tag type must agree.
// The conditions are combined with $elemMatch so that they hold for one entry
// of the array rather than across entries.
func (h *commandHandler) lookup(digital bool, index uint16, what string) (bson.M, dnp3.CommandStatus) {
	if h.engine == nil {
		return nil, dnp3.CommandDownstreamFail
	}
	db := h.engine.commandDB()
	if db == nil {
		return nil, dnp3.CommandDownstreamFail
	}

	tagType, group := "analog", dnp3util.GroupAnalogOutputBlock
	if digital {
		tagType, group = "digital", dnp3util.GroupCROBCommand
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

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
	if err != nil {
		jslog.Log(jslog.LevelBasic, "%s - Tag not found in the database for %s index: %d",
			h.conn.Name, what, index)
		return nil, dnp3.CommandNotSupported
	}
	if !jsmongo.GetBool(doc, "enabled", true) {
		jslog.Log(jslog.LevelBasic, "%s - Tag disabled in the database for %s index: %d",
			h.conn.Name, what, index)
		return nil, dnp3.CommandBlocked
	}
	return doc, dnp3.CommandSuccess
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
