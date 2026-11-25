/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - command handling
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Port of the server AsduReceiveHandler.cs: validates commands received
 * from masters against the realtimeData mapping and forwards them to the
 * commandsQueue collection. The activation confirmation is sent only after
 * every check passed and the command was queued.
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
	"errors"
	"fmt"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"iec60870-5/internal/conv"
	"iec60870-5/internal/model"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
)

// selectTimeout is how long a confirmed select stays valid for its execute.
const selectTimeout = 30 * time.Second

// selectPoint records a confirmed select of a point by one link (session).
func (c *Conn) selectPoint(link asdu.Connect, key float64) {
	c.selMu.Lock()
	c.selLink, c.selKey, c.selAt = link, key, time.Now()
	c.selMu.Unlock()
}

// clearSelect drops the selection made by link, if any.
func (c *Conn) clearSelect(link asdu.Connect) {
	c.selMu.Lock()
	if c.selLink == link {
		c.selLink, c.selKey = nil, 0
	}
	c.selMu.Unlock()
}

// takeSelect reports whether link selected the point key within
// selectTimeout, and consumes that selection.
func (c *Conn) takeSelect(link asdu.Connect, key float64) bool {
	c.selMu.Lock()
	defer c.selMu.Unlock()
	ok := c.selLink == link && c.selKey == key && key != 0 && time.Since(c.selAt) < selectTimeout
	if c.selLink == link {
		c.selLink, c.selKey = nil, 0
	}
	return ok
}

// cmdRequest carries the fields extracted from a received command ASDU.
type cmdRequest struct {
	val        float64
	objAddr    int
	dur        int // QU/QL/QCC/QRP/QPM/QPA qualifier
	isSelect   bool
	cmdHasTime bool
	cmdTime    time.Time
}

// negativeCon sends a negative activation confirmation mirror.
func (e *Engine) negativeCon(replyTo asdu.Connect, pack *asdu.ASDU) {
	pack.Coa.IsNegative = true
	_ = pack.SendReplyMirror(e.reply(replyTo), asdu.ActivationCon)
	pack.Coa.IsNegative = false
}

func (e *Engine) positiveCon(replyTo asdu.Connect, pack *asdu.ASDU) {
	_ = pack.SendReplyMirror(e.reply(replyTo), asdu.ActivationCon)
}

// HandleCommandASDU processes control-direction ASDUs routed by the library
// to ASDUHandler (C_SC/C_DC/C_RC/C_SE/C_BO and parameter commands).
// Returns true when the type was recognized.
func (e *Engine) HandleCommandASDU(srv *Conn, replyTo asdu.Connect, pack *asdu.ASDU) bool {
	conName := srv.Cfg.Name + " - "
	var req cmdRequest

	switch pack.Type {
	case asdu.C_SC_NA_1, asdu.C_SC_TA_1: // 45, 58
		cmd := pack.GetSingleCmd()
		req.isSelect = cmd.Qoc.InSelect
		req.dur = int(cmd.Qoc.Qual)
		req.objAddr = int(cmd.Ioa)
		if cmd.Value {
			req.val = 1
		}
		if pack.Type == asdu.C_SC_TA_1 {
			req.cmdTime, req.cmdHasTime = cmd.Time, true
		}
	case asdu.C_DC_NA_1, asdu.C_DC_TA_1: // 46, 59
		cmd := pack.GetDoubleCmd()
		req.isSelect = cmd.Qoc.InSelect
		req.dur = int(cmd.Qoc.Qual)
		req.objAddr = int(cmd.Ioa)
		if cmd.Value != asdu.DCOOn && cmd.Value != asdu.DCOOff {
			e.negativeCon(replyTo, pack)
			jslog.Log(jslog.LevelBasic, "%s  Invalid double state command %d", conName, cmd.Value)
			srv.Sec.Refused(replyTo, pack, int(cmd.Ioa), "", "invalid double command state")
			srv.clearSelect(replyTo)
			return true
		}
		if cmd.Value == asdu.DCOOn {
			req.val = 1
		}
		if pack.Type == asdu.C_DC_TA_1 {
			req.cmdTime, req.cmdHasTime = cmd.Time, true
		}
	case asdu.C_RC_NA_1, asdu.C_RC_TA_1: // 47, 60
		cmd := pack.GetStepCmd()
		req.isSelect = cmd.Qoc.InSelect
		req.dur = int(cmd.Qoc.Qual)
		req.objAddr = int(cmd.Ioa)
		if cmd.Value != asdu.SCOStepUP && cmd.Value != asdu.SCOStepDown {
			e.negativeCon(replyTo, pack)
			jslog.Log(jslog.LevelBasic, "%s  Invalid step state command %d", conName, cmd.Value)
			srv.Sec.Refused(replyTo, pack, int(cmd.Ioa), "", "invalid step command state")
			srv.clearSelect(replyTo)
			return true
		}
		if cmd.Value == asdu.SCOStepUP {
			req.val = 1
		}
		if pack.Type == asdu.C_RC_TA_1 {
			req.cmdTime, req.cmdHasTime = cmd.Time, true
		}
	case asdu.C_SE_NA_1, asdu.C_SE_TA_1: // 48, 61
		cmd := pack.GetSetpointNormalCmd()
		req.isSelect = cmd.Qos.InSelect
		req.dur = int(cmd.Qos.Qual)
		req.objAddr = int(cmd.Ioa)
		req.val = cmd.Value.Float64()
		if pack.Type == asdu.C_SE_TA_1 {
			req.cmdTime, req.cmdHasTime = cmd.Time, true
		}
	case asdu.C_SE_NB_1, asdu.C_SE_TB_1: // 49, 62
		cmd := pack.GetSetpointCmdScaled()
		req.isSelect = cmd.Qos.InSelect
		req.dur = int(cmd.Qos.Qual)
		req.objAddr = int(cmd.Ioa)
		req.val = float64(cmd.Value)
		if pack.Type == asdu.C_SE_TB_1 {
			req.cmdTime, req.cmdHasTime = cmd.Time, true
		}
	case asdu.C_SE_NC_1, asdu.C_SE_TC_1: // 50, 63
		cmd := pack.GetSetpointFloatCmd()
		req.isSelect = cmd.Qos.InSelect
		req.dur = int(cmd.Qos.Qual)
		req.objAddr = int(cmd.Ioa)
		req.val = float64(cmd.Value)
		if pack.Type == asdu.C_SE_TC_1 {
			req.cmdTime, req.cmdHasTime = cmd.Time, true
		}
	case asdu.C_BO_NA_1, asdu.C_BO_TA_1: // 51, 64
		cmd := pack.GetBitsString32Cmd()
		req.objAddr = int(cmd.Ioa)
		req.val = float64(cmd.Value)
		if pack.Type == asdu.C_BO_TA_1 {
			req.cmdTime, req.cmdHasTime = cmd.Time, true
		}
	case asdu.P_ME_NA_1: // 110
		cmd := pack.GetParameterNormal()
		req.objAddr = int(cmd.Ioa)
		req.val = cmd.Value.Float64()
		req.dur = int(cmd.Qpm.Category)
	case asdu.P_ME_NB_1: // 111
		cmd := pack.GetParameterScaled()
		req.objAddr = int(cmd.Ioa)
		req.val = float64(cmd.Value)
		req.dur = int(cmd.Qpm.Category)
	case asdu.P_ME_NC_1: // 112
		cmd := pack.GetParameterFloat()
		req.objAddr = int(cmd.Ioa)
		req.val = float64(cmd.Value)
		req.dur = int(cmd.Qpm.Category)
	case asdu.P_AC_NA_1: // 113
		cmd := pack.GetParameterActivation()
		req.objAddr = int(cmd.Ioa)
		req.dur = int(cmd.Qpa)
	default:
		return false
	}
	jslog.Log(jslog.LevelBasic, "%s  %s Obj Address %d", conName, pack.Type.String(), req.objAddr)
	e.forwardCommand(srv, replyTo, pack, req)
	return true
}

// HandleResetProcess processes C_RP_NA_1 (105) — forwarded like a command.
func (e *Engine) HandleResetProcess(srv *Conn, replyTo asdu.Connect, pack *asdu.ASDU, qrp asdu.QualifierOfResetProcessCmd) {
	req := cmdRequest{dur: int(qrp)}
	jslog.Log(jslog.LevelBasic, "%s -   Reset process command QRP %d", srv.Cfg.Name, int(qrp))
	e.forwardCommand(srv, replyTo, pack, req)
}

// HandleCounterInterrogation processes C_CI_NA_1 (101) — forwarded like a
// command when a matching destination exists (C# parity).
func (e *Engine) HandleCounterInterrogation(srv *Conn, replyTo asdu.Connect, pack *asdu.ASDU, qcc asdu.QualifierCountCall) {
	req := cmdRequest{dur: int(qcc.Request) | int(qcc.Freeze)<<6}
	jslog.Log(jslog.LevelBasic, "%s -   Counter interrogation command", srv.Cfg.Name)
	e.forwardCommand(srv, replyTo, pack, req)
}

// HandleClockSync processes C_CS_NA_1 (103): confirm positive and log.
func (e *Engine) HandleClockSync(srv *Conn, replyTo asdu.Connect, pack *asdu.ASDU, t time.Time) {
	e.positiveCon(replyTo, pack)
	jslog.Log(jslog.LevelBasic, "%s -   Received clock sync command with time %s", srv.Cfg.Name, t.String())
	srv.clearSelect(replyTo)
}

// HandleRead processes C_RD_NA_1 (102): look up the object mapped to this
// connection/CA/IOA (any type) and send its current value with cause Request.
func (e *Engine) HandleRead(srv *Conn, replyTo asdu.Connect, pack *asdu.ASDU, ioa asdu.InfoObjAddr) {
	conName := srv.Cfg.Name + " - "
	if !e.IsMongoLive.Load() {
		return
	}
	point, dst, ferr := e.mappedPoint(srv, int(pack.CommonAddr), int(ioa), 0)
	if point == nil {
		e.negativeCon(replyTo, pack)
		jslog.Log(jslog.LevelBasic, "%s  Request to read object not found, address: %d", conName, int(ioa))
		srv.clearSelect(replyTo)
		if ferr == nil { // nothing maps it (not a database failure)
			if e.knownCA(srv, int(pack.CommonAddr)) {
				srv.Sec.UnknownRead(replyTo, pack, int(ioa))
			} else {
				srv.Sec.UnknownCA(replyTo, pack, "read")
			}
		}
		return
	}
	e.positiveCon(replyTo, pack)
	obj := destInfoObj(point, dst, int(dst.ASDU), nil)
	if obj != nil {
		coa := asdu.CauseOfTransmission{Cause: asdu.Request}
		if err := conv.SendInfoBatch(e.reply(replyTo), coa, asdu.CommonAddr(int(dst.CommonAddress)), obj.TypeID,
			[]*conv.InfoObject{obj}); err != nil {
			jslog.Log(jslog.LevelBasic, "%s  Error sending read reply: %s", conName, err.Error())
		}
	}
}

// findMappedPoint finds the realtimeData point with a destination on this
// connection at this common address and object address, and returns that
// destination; asduType 0 matches any type (C_RD). All conditions must hold
// on the same protocolDestinations entry.
func (e *Engine) findMappedPoint(srv *Conn, ca, ioa, asduType int) (*model.RtDataPoint, *model.ProtocolDestination) {
	p, d, _ := e.mappedPoint(srv, ca, ioa, asduType)
	return p, d
}

// mappedPoint is findMappedPoint; the error is set when the database could
// not tell (nil with no point: nothing maps the address).
func (e *Engine) mappedPoint(srv *Conn, ca, ioa, asduType int) (*model.RtDataPoint, *model.ProtocolDestination, error) {
	// address fields are matched as number or string (they may be configured
	// with either representation)
	elem := bson.M{
		"protocolDestinationConnectionNumber": srv.Cfg.ProtocolConnectionNumber,
		"protocolDestinationCommonAddress":    jsmongo.AddrMatch(ca),
		"protocolDestinationObjectAddress":    jsmongo.AddrMatch(ioa),
	}
	if asduType != 0 {
		elem["protocolDestinationASDU"] = jsmongo.AddrMatch(asduType)
	}
	filter := bson.M{"protocolDestinations": bson.M{"$elemMatch": elem}}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var point model.RtDataPoint
	if err := e.DB().Collection(jsmongo.RealtimeDataCollectionName).FindOne(ctx, filter).Decode(&point); err != nil {
		conName := srv.Cfg.Name + " - "
		jslog.Log(jslog.LevelBasic, "%s  Error finding command point for CA %d, IOA %d, ASDU %d: %s", conName, ca, ioa, asduType, err.Error())
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	for i := range point.ProtocolDestinations {
		d := &point.ProtocolDestinations[i]
		if int(d.ConnectionNumber) == int(srv.Cfg.ProtocolConnectionNumber) &&
			int(d.CommonAddress) == ca && int(d.ObjectAddress) == ioa &&
			(asduType == 0 || int(d.ASDU) == asduType) {
			return &point, d, nil
		}
	}
	return nil, nil, nil
}

// knownCA reports whether a tag is mapped to the common address on this
// connection; true when the database cannot tell.
func (e *Engine) knownCA(srv *Conn, ca int) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	n, err := e.DB().Collection(jsmongo.RealtimeDataCollectionName).CountDocuments(ctx,
		bson.M{"protocolDestinations": bson.M{"$elemMatch": bson.M{
			"protocolDestinationConnectionNumber": srv.Cfg.ProtocolConnectionNumber,
			"protocolDestinationCommonAddress":    jsmongo.AddrMatch(ca),
		}}}, options.Count().SetLimit(1))
	return err != nil || n > 0
}

// isGlobalCA reports the broadcast common address. go-iecp5 decodes the
// 1-octet broadcast address 255 as GlobalCommonAddr too.
func isGlobalCA(replyTo asdu.Connect, ca asdu.CommonAddr) bool {
	if ca == asdu.GlobalCommonAddr {
		return true
	}
	p := replyTo.Params()
	return p != nil && p.CommonAddrSize == 1 && ca == 0xFF
}

// forwardCommand runs the full validation flow and inserts the command
// into commandsQueue (port of the C# server AsduReceiveHandler).
func (e *Engine) forwardCommand(srv *Conn, replyTo asdu.Connect, pack *asdu.ASDU, req cmdRequest) {
	conName := srv.Cfg.Name + " - "
	received := time.Now() // the unnecessary-command check compares with the state before this
	// commandsEnabled and the test bit concern what operates the process
	// (C_CI counter interrogation and C_RP reset process go on as before)
	operates := asdu.IsProcessCommand(pack.Type) || asdu.IsParameter(pack.Type)
	if operates && !srv.Cfg.CommandsEnabledVal() {
		e.negativeCon(replyTo, pack)
		jslog.Log(jslog.LevelBasic, "%s  Command rejected, commands are disabled for this connection", conName)
		srv.clearSelect(replyTo)
		srv.Sec.Refused(replyTo, pack, req.objAddr, "", "commands are disabled for this connection")
		return
	}
	if operates && pack.Coa.IsTest {
		// a test command must not operate the process: not forwarded
		e.negativeCon(replyTo, pack)
		jslog.Log(jslog.LevelBasic, "%s  Command rejected, test bit set", conName)
		srv.clearSelect(replyTo)
		srv.Sec.Refused(replyTo, pack, req.objAddr, "", "command with the test bit set")
		return
	}
	if !e.IsMongoLive.Load() {
		e.negativeCon(replyTo, pack)
		jslog.Log(jslog.LevelBasic, "%s  Command rejected, MongoDB not available", conName)
		srv.Sec.Refused(replyTo, pack, req.objAddr, "", "database not available")
		return
	}

	point, dst, ferr := e.mappedPoint(srv, int(pack.CommonAddr), req.objAddr, int(pack.Type))
	if point == nil {
		e.negativeCon(replyTo, pack)
		jslog.Log(jslog.LevelBasic, "%s  Command not found!", conName)
		srv.clearSelect(replyTo)
		if ferr != nil {
			srv.Sec.Refused(replyTo, pack, req.objAddr, "", "database error looking for the command's point")
		} else if asdu.IsProcessCommand(pack.Type) || asdu.IsParameter(pack.Type) {
			if e.knownCA(srv, int(pack.CommonAddr)) {
				srv.Sec.UnknownCommand(replyTo, pack, req.objAddr)
			} else {
				srv.Sec.UnknownCA(replyTo, pack, "command")
			}
		}
		return
	}
	jslog.Log(jslog.LevelBasic, "%s  Command found.", conName)

	dstkconv1 := dst.KConv1
	if dstkconv1 == 0 {
		dstkconv1 = 1
	}
	dstkconv2 := dst.KConv2
	dstsbo := dst.CommandUseSBO
	dstdur := int(dst.CommandDuration)

	if req.isSelect && !dstsbo {
		// tried a select when there is no select expected
		e.negativeCon(replyTo, pack)
		jslog.Log(jslog.LevelBasic, "%s  Select tried but not expected!", conName)
		srv.clearSelect(replyTo)
		srv.Sec.Refused(replyTo, pack, req.objAddr, point.Tag, "select of a command configured without select-before-operate")
		return
	}

	if req.dur != dstdur {
		// duration spec different than expected, reject command
		e.negativeCon(replyTo, pack)
		jslog.Log(jslog.LevelBasic, "%s  QU/QL command qualifier not expected: %d, %d wanted", conName, req.dur, dstdur)
		srv.clearSelect(replyTo)
		srv.Sec.Refused(replyTo, pack, req.objAddr, point.Tag, fmt.Sprintf("command qualifier %d, %d expected", req.dur, dstdur))
		return
	}

	srcconn := int(point.ProtocolSourceConnectionNumber)
	srcdur := int(point.ProtocolSourceCommandDuration)
	srcobjaddr := int(point.ProtocolSourceObjectAddress)
	srcasdu := int(point.ProtocolSourceASDU)
	srcca := int(point.ProtocolSourceCommonAddress)
	srckconv1 := point.KConv1
	if srckconv1 == 0 {
		srckconv1 = 1
	}
	srckconv2 := point.KConv2
	srcsbo := point.ProtocolSourceCommandUseSBO
	srcpointkey := point.ID
	srctag := point.Tag
	srctype := point.Type

	if req.cmdHasTime {
		// the window applies both ways: a time in the future is as
		// suspect as a stale one
		if age := time.Since(req.cmdTime); age > TimeToExpireCommandsWithTime || age < -TimeToExpireCommandsWithTime {
			jslog.Log(jslog.LevelBasic, "%s  Command with time outside the %v window (age %v)", conName, TimeToExpireCommandsWithTime, age)
			e.negativeCon(replyTo, pack)
			srv.clearSelect(replyTo)
			srv.Sec.StaleTime(replyTo, pack, req.objAddr, srctag, req.cmdTime, TimeToExpireCommandsWithTime)
			return
		}
	}

	if req.isSelect {
		srv.selectPoint(replyTo, srcpointkey) // flag selected point
		e.positiveCon(replyTo, pack)
		jslog.Log(jslog.LevelBasic, "%s  Select!", conName)
		return // do not forward a select
	}

	if dstsbo && !srv.takeSelect(replyTo, srcpointkey) {
		// execute without a valid select of this point by this master
		e.negativeCon(replyTo, pack)
		jslog.Log(jslog.LevelBasic, "%s  Tried execute without select first!", conName)
		srv.Sec.ExecuteWithoutSelect(replyTo, pack, req.objAddr, srctag)
		return
	}
	srv.clearSelect(replyTo)
	jslog.Log(jslog.LevelBasic, "%s  Execute (forward to queue)!", conName)

	val := req.val
	// destination conversion (received value -> engineering value)
	switch pack.Type {
	case asdu.C_SC_NA_1, asdu.C_SC_TA_1, asdu.C_DC_NA_1, asdu.C_DC_TA_1,
		asdu.C_RC_NA_1, asdu.C_RC_TA_1:
		if dstkconv1 == -1 { // invert digital for kconv1 -1
			if val == 0 {
				val = 1
			} else {
				val = 0
			}
		}
	case asdu.C_SE_NA_1, asdu.C_SE_TA_1, asdu.C_SE_NB_1, asdu.C_SE_TB_1,
		asdu.C_SE_NC_1, asdu.C_SE_TC_1, asdu.P_ME_NA_1, asdu.P_ME_NB_1,
		asdu.P_ME_NC_1, asdu.P_AC_NA_1, asdu.C_RP_NA_1:
		val = val*dstkconv1 + dstkconv2
	case asdu.C_BO_NA_1, asdu.C_BO_TA_1:
		if dstkconv1 == -1 {
			val = float64(conv.Bits32(val)) // truncation only
		}
	}

	// source conversion (engineering value -> source protocol value)
	srcval := val
	switch asdu.TypeID(srcasdu) {
	case asdu.C_SC_NA_1, asdu.C_SC_TA_1, asdu.C_DC_NA_1, asdu.C_DC_TA_1,
		asdu.C_RC_NA_1, asdu.C_RC_TA_1:
		if srckconv1 == -1 { // invert digital for kconv1 -1
			if val == 0 {
				srcval = 1
			} else {
				srcval = 0
			}
		}
	case asdu.C_SE_NA_1, asdu.C_SE_TA_1, asdu.C_SE_NB_1, asdu.C_SE_TB_1,
		asdu.C_SE_NC_1, asdu.C_SE_TC_1, asdu.P_ME_NA_1, asdu.P_ME_NB_1,
		asdu.P_ME_NC_1, asdu.P_AC_NA_1, asdu.C_RP_NA_1:
		srcval = val*srckconv1 + srckconv2
	case asdu.C_BO_NA_1, asdu.C_BO_TA_1:
		if srckconv1 == -1 { // invert digital bits for kconv1 -1
			srcval = float64(^conv.Bits32(val))
		} else {
			srcval = float64(conv.Bits32(val))
		}
	default:
		if srctype == "digital" {
			if srckconv1 == -1 { // invert digital bits for kconv1 -1
				if val == 0 {
					srcval = 1
				} else {
					srcval = 0
				}
			}
		}
		if srctype == "analog" {
			srcval = val*srckconv1 + srckconv2
		}
	}

	orgip := ""
	if srv.OriginatorIP != nil {
		orgip = srv.OriginatorIP()
	}

	cmdDoc := bson.D{
		{Key: "protocolSourceConnectionNumber", Value: float64(srcconn)},
		{Key: "protocolSourceCommonAddress", Value: float64(srcca)},
		{Key: "protocolSourceObjectAddress", Value: float64(srcobjaddr)},
		{Key: "protocolSourceASDU", Value: float64(srcasdu)},
		{Key: "protocolSourceCommandDuration", Value: float64(srcdur)},
		{Key: "protocolSourceCommandUseSBO", Value: srcsbo},
		{Key: "pointKey", Value: srcpointkey},
		{Key: "tag", Value: srctag},
		{Key: "value", Value: srcval},
		{Key: "valueString", Value: formatValue(srcval)},
		{Key: "originatorUserName", Value: "Protocol connection: " + srv.Cfg.Name},
		{Key: "originatorIpAddress", Value: orgip},
		{Key: "timeTag", Value: time.Now()},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := e.DB().Collection(jsmongo.CommandsQueueCollectionName).InsertOne(ctx, cmdDoc); err != nil {
		jslog.Log(jslog.LevelBasic, "%s", "  Exception Mongo: "+err.Error())
		e.negativeCon(replyTo, pack) // not forwarded
		srv.Sec.Refused(replyTo, pack, req.objAddr, srctag, "the command could not be queued: database error")
		return
	}
	e.positiveCon(replyTo, pack)
	srv.Sec.Forwarded(replyTo, pack, req.objAddr, srctag, srcpointkey, val, received)
}
