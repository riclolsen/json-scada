/*
 * DNP3 Client Protocol driver for {json:scada}, in Go.
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

// The commandsQueue watcher and command dispatch. Ports of processMongoCmd()
// and executeCommand().

package clientapp

import (
	"context"
	"errors"
	"time"

	"dnp3-go/internal/dnp3util"

	"github.com/riclolsen/json-scada/src/go-common/jscommands"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/master"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// CommandExpiry is how old a queued command may be and still be executed.
const CommandExpiry = jscommands.DefaultExpiry

// commandsLoop watches commandsQueue for inserts and dispatches each one.
//
// The resume token is kept across reconnections so that a command inserted
// during a MongoDB outage is still delivered; the C++ driver restarts the
// stream from "now" and loses it (deviation D9).
func (e *Engine) commandsLoop(ctx context.Context) {
	var resumeToken bson.Raw

	for ctx.Err() == nil {
		cli, err := jsmongo.Connect(e.cfg)
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "Exception Mongo CMD: %v", err)
			sleepCtx(ctx, 3*time.Second)
			continue
		}
		coll := cli.Database(e.cfg.MongoDatabaseName).Collection(jsmongo.CommandsQueueCollectionName)

		pipeline := jscommands.InsertOnlyPipeline()
		opts := options.ChangeStream()
		if len(resumeToken) > 0 {
			opts.SetResumeAfter(resumeToken)
		}

		stream, err := coll.Watch(ctx, pipeline, opts)
		if err != nil {
			jslog.Log(jslog.LevelNoLog, "Exception Mongo CMD: %v", err)
			_ = cli.Disconnect(context.Background())
			// A resume token can go stale once its oplog entry ages out; drop
			// it so the next attempt starts fresh rather than failing forever.
			resumeToken = nil
			sleepCtx(ctx, 3*time.Second)
			continue
		}

		for stream.Next(ctx) {
			resumeToken = stream.ResumeToken()
			var change struct {
				FullDocument bson.M `bson:"fullDocument"`
			}
			if err := stream.Decode(&change); err != nil {
				jslog.Log(jslog.LevelDetailed, "Mongo CMD: cannot decode change: %v", err)
				continue
			}
			if change.FullDocument != nil {
				e.executeCommand(ctx, coll, change.FullDocument)
			}
		}
		if err := stream.Err(); err != nil && ctx.Err() == nil {
			jslog.Log(jslog.LevelNoLog, "Exception Mongo CMD: %v", err)
		}
		_ = stream.Close(context.Background())
		_ = cli.Disconnect(context.Background())
		sleepCtx(ctx, 3*time.Second)
	}
}

// executeCommand validates a queued command and issues it.
func (e *Engine) executeCommand(ctx context.Context, coll *mongo.Collection, cmd bson.M) {
	if !e.redundancy.Active() {
		return
	}
	id, ok := cmd["_id"]
	if !ok {
		return
	}

	connNumber := jsmongo.GetInt(cmd, "protocolSourceConnectionNumber", 0)
	conn := e.connByNumber(connNumber)
	if conn == nil {
		// The change stream sees every command in the queue, including those of other
		// drivers and instances. A connection this driver does not own is not ours to
		// cancel: just log it, as the other {json:scada} drivers do.
		jslog.Log(jslog.LevelDetailed, "Mongo CMD: connection %d not handled by this driver, ignoring command", connNumber)
		return
	}
	session := conn.Session()
	switch {
	case session == nil:
		cancelCommand(ctx, coll, id, "connection_not_found")
		return
	case !conn.Connected():
		cancelCommand(ctx, coll, id, "not_connected")
		return
	case !conn.CommandsEnabled:
		cancelCommand(ctx, coll, id, "cmds_disabled")
		return
	}
	if time.Since(time.UnixMilli(jsmongo.GetDateMs(cmd, "timeTag", time.Now().UnixMilli()))) > CommandExpiry {
		cancelCommand(ctx, coll, id, "expired")
		return
	}

	group := jsmongo.GetInt(cmd, "protocolSourceCommonAddress", 0)
	variation := jsmongo.GetInt(cmd, "protocolSourceASDU", 0)
	index := uint16(jsmongo.GetInt(cmd, "protocolSourceObjectAddress", 0))
	useSBO := jsmongo.GetBool(cmd, "protocolSourceCommandUseSBO", false)
	value := jsmongo.GetDouble(cmd, "value", 0)
	duration := jsmongo.GetInt(cmd, "protocolSourceCommandDuration", 0)

	var command master.Command
	switch group {
	case dnp3util.GroupCROBCommand:
		command = master.CROB(index, dnp3util.CROBFor(duration, value))
	case dnp3util.GroupAnalogOutputBlock:
		switch variation {
		case 1:
			command = master.AnalogOutputInt32(index, int32(value))
		case 2:
			command = master.AnalogOutputInt16(index, int16(value))
		case 4:
			command = master.AnalogOutputFloat64(index, value)
		default:
			// Variation 3 and anything unrecognised: single precision, as the
			// C++ driver's final fall-through does.
			command = master.AnalogOutputFloat32(index, float32(value))
		}
	default:
		cancelCommand(ctx, coll, id, "unsupported_group")
		return
	}

	name := conn.Name
	jslog.Log(jslog.LevelBasic, "%s - Issuing command %s useSBO=%v", name, command, useSBO)

	// The request methods block until the outstation answers, so each command
	// gets its own goroutine and the change stream is never held up.
	go func() {
		cctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		var (
			res master.CommandResult
			err error
		)
		if useSBO {
			res, err = session.SelectAndOperate(cctx, command)
		} else {
			res, err = session.DirectOperate(cctx, command)
		}

		ok, description := describeResult(res, err)
		jslog.Log(jslog.LevelBasic, "%s - Command result: %s", name, description)
		ackCommand(context.Background(), e, id, ok, description)
	}()
}

// describeResult maps a command outcome onto the result vocabulary the C++
// driver writes, so operator screens matching those strings keep matching.
//
// A rejected command additionally carries the DNP3 status in parentheses after
// the legacy prefix (deviation D10).
func describeResult(res master.CommandResult, err error) (bool, string) {
	if err == nil {
		return true, "SUCCESS"
	}
	switch {
	case errors.Is(err, dnp3.ErrTimeout):
		return false, "FAILURE_RESPONSE_TIMEOUT"
	case errors.Is(err, dnp3.ErrTaskFailed):
		return false, "FAILURE_START_TIMEOUT"
	case errors.Is(err, dnp3.ErrNoConnection), errors.Is(err, dnp3.ErrClosed):
		return false, "FAILURE_NO_COMMS"
	case errors.Is(err, dnp3.ErrMalformed), errors.Is(err, dnp3.ErrBadConfig):
		return false, "FAILURE_MESSAGE_FORMAT_ERROR"
	}
	// A non-zero command status is reported by CommandResult.Err(), which is
	// the error returned when the exchange itself succeeded.
	for _, s := range res.Statuses {
		if !s.OK() {
			return false, "FAILURE_BAD_RESPONSE (" + s.String() + ")"
		}
	}
	if len(res.Statuses) > 0 {
		return false, "FAILURE_BAD_RESPONSE"
	}
	return false, "UNKNOWN"
}

// cancelCommand records why a command was not issued.
func cancelCommand(ctx context.Context, coll *mongo.Collection, id any, reason string) {
	if err := jscommands.Cancel(ctx, coll, id, reason); err != nil {
		jslog.Log(jslog.LevelDetailed, "cancelCommand error: %v", err)
	}
}

// ackCommand records the outcome of a command that was issued.
//
// It opens its own client: the change-stream cursor blocks the one the watcher
// owns, and a mongo.Client is cheap to create next to the round trip a DNP3
// command has just taken.
func ackCommand(ctx context.Context, e *Engine, id any, ok bool, description string) {
	cli, err := jsmongo.Connect(e.cfg)
	if err != nil {
		jslog.Log(jslog.LevelNoLog, "ackCommand error: %v", err)
		return
	}
	defer func() { _ = cli.Disconnect(context.Background()) }()

	coll := cli.Database(e.cfg.MongoDatabaseName).
		Collection(jsmongo.CommandsQueueCollectionName)
	if err := jscommands.Ack(ctx, coll, id, ok, description); err != nil {
		jslog.Log(jslog.LevelNoLog, "ackCommand error: %v", err)
	}
}
