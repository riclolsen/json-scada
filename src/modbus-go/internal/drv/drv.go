/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

// Package drv is the start-up scaffolding both Modbus drivers share: command
// line and environment handling (the Node.js driver's contract), the config
// file, logging adapters and the driver instance document.
package drv

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"modbus-go/internal/mbcore"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmodel"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// AppDefs identify a driver.
type AppDefs struct {
	Name      string // protocolDriver: MODBUS or MODBUS_SERVER
	EnvPrefix string // JS_MODBUS_ or JS_MODBUSSRV_
	Version   string
	Msg       string
}

// Setup is the outcome of start-up.
type Setup struct {
	Config     jsconfig.Config
	ConfigPath string
	Instance   int
	LogLevel   int
}

// configCandidates are tried, in order, when neither the command line nor the
// environment names the config file: ../conf from bin/ (where the services
// run the binaries), ../../conf from src/modbus-go/<dir> or src/modbus (the
// Node.js driver's default), and the Windows install path.
var configCandidates = []string{
	"../conf/json-scada.json",
	"../../conf/json-scada.json",
	"c:/json-scada/conf/json-scada.json",
}

// Start parses `<instance> <logLevel> <configFile>` (each also settable as
// <EnvPrefix>INSTANCE, <EnvPrefix>LOGLEVEL, <EnvPrefix>CONFIG_FILE, and
// JS_CONFIG_FILE for the file), loads the config file and logs the banner.
// A missing or invalid config file is fatal.
func Start(app AppDefs, argv []string) Setup {
	args := argv[1:]
	getenv := func(k string) string { return strings.TrimSpace(os.Getenv(app.EnvPrefix + k)) }

	level := jslog.LevelBasic
	if v := getenv("LOGLEVEL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			level = n
		}
	}
	if len(args) > 1 {
		if n, err := strconv.Atoi(strings.TrimSpace(args[1])); err == nil {
			level = n
		}
	}
	jslog.SetLevel(level)

	instance := 0
	if len(args) > 0 {
		instance, _ = strconv.Atoi(strings.TrimSpace(args[0]))
	}
	if instance == 0 {
		instance, _ = strconv.Atoi(getenv("INSTANCE"))
	}
	if instance == 0 {
		instance = 1
	}

	path := ""
	switch {
	case len(args) > 2 && strings.TrimSpace(args[2]) != "":
		path = strings.TrimSpace(args[2])
	case getenv("CONFIG_FILE") != "":
		path = getenv("CONFIG_FILE")
	case strings.TrimSpace(os.Getenv(jsconfig.ConfigFileEnvVar)) != "":
		path = strings.TrimSpace(os.Getenv(jsconfig.ConfigFileEnvVar))
	default:
		path = configCandidates[0]
		for _, c := range configCandidates {
			if jsconfig.FileExists(c) {
				path = c
				break
			}
		}
	}
	path = jsconfig.ExpandHome(path)
	jslog.Log(jslog.LevelNoLog, "Config - Config File: %s", path)
	if !jsconfig.FileExists(path) {
		jslog.Fatal("Config - Error: config file not found!")
	}
	cfg, err := jsconfig.Load(path)
	if err != nil {
		jslog.Fatal("Error reading config file. %v", err)
	}
	if err := jsconfig.Validate(cfg, path); err != nil {
		jslog.Fatal("Error reading config file. %v", err)
	}
	jslog.Log(jslog.LevelNoLog, "Config - %s Version %s", app.Msg, app.Version)
	jslog.Log(jslog.LevelNoLog, "Config - Instance: %d", instance)
	jslog.Log(jslog.LevelNoLog, "Config - Log level: %d", level)
	return Setup{Config: cfg, ConfigPath: path, Instance: instance, LogLevel: level}
}

// EnsureInstance creates the protocolDriverInstances document when it does
// not exist, with this node as the active one, as the Node.js driver did on
// first start. It reports whether a document was created.
func EnsureInstance(ctx context.Context, db *mongo.Database, app AppDefs, s Setup) (bool, error) {
	coll := db.Collection(jsmongo.ProtocolDriverInstancesCollectionName)
	fctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	n, err := coll.CountDocuments(fctx, jsmodel.InstanceFilter(app.Name, s.Instance))
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	jslog.Log(jslog.LevelNoLog, "Redundancy - Instance config not found, creating one...")
	_, err = coll.InsertOne(fctx, bson.M{
		"protocolDriver":               app.Name,
		"protocolDriverInstanceNumber": float64(s.Instance),
		"enabled":                      true,
		"logLevel":                     float64(s.LogLevel),
		"nodeNames":                    bson.A{},
		"activeNodeName":               s.Config.NodeName,
		"activeNodeKeepAliveTimeTag":   jsmongo.Now(),
	})
	return err == nil, err
}

// InstanceDisabled reports whether the instance document says enabled: false;
// the Node.js driver exited in that case.
func InstanceDisabled(ctx context.Context, db *mongo.Database, app AppDefs, instance int) bool {
	fctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var doc bson.M
	if err := db.Collection(jsmongo.ProtocolDriverInstancesCollectionName).
		FindOne(fctx, jsmodel.InstanceFilter(app.Name, instance)).Decode(&doc); err != nil {
		return false
	}
	return !jsmodel.InstanceFromDoc(doc).Enabled
}

// ConnLogger prefixes every line with a connection name and maps the three
// severities to log levels 3 (debug), 2 (info) and 1 (error), as the Node.js
// driver's stack logger did. It also satisfies go-modbus's Logger.
type ConnLogger struct{ Prefix string }

// Debug logs at level 3.
func (l ConnLogger) Debug(msg string) { jslog.Log(jslog.LevelDebug, "%s: %s", l.Prefix, msg) }

// Info logs at level 2.
func (l ConnLogger) Info(msg string) { jslog.Log(jslog.LevelDetailed, "%s: %s", l.Prefix, msg) }

// Error logs at level 1.
func (l ConnLogger) Error(msg string) { jslog.Log(jslog.LevelBasic, "%s: %s", l.Prefix, msg) }

// DebugEnabled is true at log level 3, so callers can skip building traces.
func (l ConnLogger) DebugEnabled() bool { return jslog.Level() >= jslog.LevelDebug }

// LibLogger adapts go-modbus's internal log lines: debug and info at level 3,
// warnings at 2, errors at 1.
type LibLogger struct{ Prefix string }

// Debugf implements modbus.Logger.
func (l LibLogger) Debugf(f string, a ...any) {
	jslog.Log(jslog.LevelDebug, "%s: go-modbus: %s", l.Prefix, fmt.Sprintf(f, a...))
}

// Infof implements modbus.Logger.
func (l LibLogger) Infof(f string, a ...any) {
	jslog.Log(jslog.LevelDebug, "%s: go-modbus: %s", l.Prefix, fmt.Sprintf(f, a...))
}

// Warnf implements modbus.Logger.
func (l LibLogger) Warnf(f string, a ...any) {
	jslog.Log(jslog.LevelDetailed, "%s: go-modbus: %s", l.Prefix, fmt.Sprintf(f, a...))
}

// Errorf implements modbus.Logger.
func (l LibLogger) Errorf(f string, a ...any) {
	jslog.Log(jslog.LevelBasic, "%s: go-modbus: %s", l.Prefix, fmt.Sprintf(f, a...))
}

// Conversions shared by both drivers' config readers: permissive, as the
// Node.js driver's num/str/bool helpers were.

// Num is a finite number, or def for null, missing, "" and non-numbers.
func Num(doc bson.M, key string, def float64) float64 {
	v, ok := doc[key]
	if !ok || v == nil {
		return def
	}
	if s, ok := v.(string); ok && s == "" {
		return def
	}
	f := jsmongo.ToFloat(v, def)
	if f != f || f > 1.7e308 || f < -1.7e308 {
		return def
	}
	return f
}

// Str is a non-empty string, or def.
func Str(doc bson.M, key string, def string) string {
	if s, ok := doc[key].(string); ok && s != "" {
		return s
	}
	return def
}

// Bool is a boolean, or def for anything that is not one.
func Bool(doc bson.M, key string, def bool) bool {
	if b, ok := doc[key].(bool); ok {
		return b
	}
	return def
}

// StrArr is an array rendered as strings, empty when not an array.
func StrArr(doc bson.M, key string) []string {
	var out []string
	switch a := doc[key].(type) {
	case bson.A:
		for _, x := range a {
			out = append(out, AnyString(x))
		}
	case []any:
		for _, x := range a {
			out = append(out, AnyString(x))
		}
	case []string:
		out = append(out, a...)
	}
	return out
}

// AnyString is JavaScript's String(x) for the BSON values a document holds.
func AnyString(x any) string {
	switch v := x.(type) {
	case nil:
		return "null"
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64, float32, int, int32, int64:
		return formatNum(jsmongo.ToFloat(v, 0))
	}
	return fmt.Sprint(x)
}

func formatNum(f float64) string { return mbcore.JSNumberString(f) }

// InstanceCheckPeriod is how often WatchInstanceEnabled re-reads the instance.
var InstanceCheckPeriod = 30 * time.Second

// WatchInstanceEnabled calls stop when the instance document becomes
// enabled: false, so the process exits as the Node.js driver did.
func WatchInstanceEnabled(ctx context.Context, db *mongo.Database, app AppDefs, instance int, stop func()) {
	t := time.NewTicker(InstanceCheckPeriod)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if InstanceDisabled(ctx, db, app, instance) {
				jslog.Log(jslog.LevelNoLog, "Instance %d of %s is disabled. Exiting.", instance, app.Name)
				stop()
				return
			}
		}
	}
}

// ToNum is the Node.js toNum: null, missing and "" give def, as do values
// that are not finite numbers.
func ToNum(v any, present bool, def float64) float64 {
	if !present || v == nil {
		return def
	}
	if s, ok := v.(string); ok && s == "" {
		return def
	}
	f := JSNumber(v, true)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return def
	}
	return f
}

// JSNumber is JavaScript's Number(v) of a BSON value; a missing field is
// undefined, which is NaN.
func JSNumber(v any, present bool) float64 {
	if !present {
		return math.NaN()
	}
	switch x := v.(type) {
	case nil:
		return 0
	case string:
		return mbcore.JSNumberFromString(x)
	case bool:
		if x {
			return 1
		}
		return 0
	case float64:
		return x
	case float32:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case int:
		return float64(x)
	}
	return math.NaN()
}
