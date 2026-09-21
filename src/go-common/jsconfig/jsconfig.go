/*
 * Shared {json:scada} driver support library, in Go.
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

// Package jsconfig reads conf/json-scada.json and the drivers' common command
// line: <instanceNumber> <logLevel> <configFilePath>, all optional, in that
// order.
//
// Promoted from dnp3-go/internal/jscfg/config.go, reconciled with
// iec60870-5/internal/jscfg/jscfg.go and the readConfigFile() of the flat
// drivers.
package jsconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
)

// DefaultConfigFilePath is the path every driver tries first.
const DefaultConfigFilePath = "../conf/json-scada.json"

// WindowsFallbackPath is the fallback the C# drivers and iec60870-5 use.
const WindowsFallbackPath = "c:/json-scada/conf/json-scada.json"

// ConfigFileEnvVar overrides the config file path when set and readable.
const ConfigFileEnvVar = "JS_CONFIG_FILE"

// Config is the subset of json-scada.json the drivers use. It is the union of
// the per-driver structs; a driver that never reads a field simply ignores it.
type Config struct {
	NodeName                 string `json:"nodeName"`
	MongoConnectionString    string `json:"mongoConnectionString"`
	MongoDatabaseName        string `json:"mongoDatabaseName"`
	TLSCaPemFile             string `json:"tlsCaPemFile"`
	TLSClientPemFile         string `json:"tlsClientPemFile"`
	TLSClientPfxFile         string `json:"tlsClientPfxFile"`
	TLSClientKeyPassword     string `json:"tlsClientKeyPassword"`
	TLSAllowInvalidHostnames bool   `json:"tlsAllowInvalidHostnames"`
	TLSAllowChainErrors      bool   `json:"tlsAllowChainErrors"`
	TLSInsecure              bool   `json:"tlsInsecure"`
}

// Args is the parsed command line.
type Args struct {
	InstanceNumber int
	// ConfigFilePath is argv[3], empty when not given.
	ConfigFilePath string
	// LogLevelFromCLI records whether the log level came from the command
	// line. The C#/C++ drivers keep a command-line level in preference to the
	// one in the instance document; every Go driver does the same.
	LogLevelFromCLI bool
}

// ParseArgs reads the three positional arguments and applies a log level given
// on the command line immediately, so anything logged during startup already
// honours it.
//
// parity: malformed numbers are logged and ignored rather than being fatal,
// which is the dnp3-go and OPC-UA behaviour. iec60870-5 used to exit on a
// malformed argument; callers that want that check Args themselves.
func ParseArgs(argv []string) Args {
	args := Args{InstanceNumber: 1}

	if len(argv) > 1 && strings.TrimSpace(argv[1]) != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(argv[1])); err == nil {
			args.InstanceNumber = n
		} else {
			jslog.Log(jslog.LevelNoLog,
				"Conversion error: Invalid integer value for ProtocolDriverInstanceNumber")
		}
	}
	if len(argv) > 2 && strings.TrimSpace(argv[2]) != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(argv[2])); err == nil {
			jslog.SetLevel(n)
			args.LogLevelFromCLI = true
		} else {
			jslog.Log(jslog.LevelNoLog,
				"Conversion error: Invalid integer value for LogLevel")
		}
	}
	if len(argv) > 3 && argv[3] != "" {
		args.ConfigFilePath = argv[3]
	}
	return args
}

// ExpandHome expands a leading ~/ using HOME, then USERPROFILE.
func ExpandHome(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	if home == "" {
		return path
	}
	return home + path[1:]
}

// FileExists reports whether the path can be opened for reading.
func FileExists(path string) bool {
	f, err := os.Open(ExpandHome(path))
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// ResolvePath picks the config file, in this precedence:
//
//	argPath (argv[3])        when it exists
//	$JS_CONFIG_FILE          when it exists
//	primary                  when it exists
//	fallback                 otherwise
//
// This is the order the flat drivers already used. Two small reconciliations:
// dnp3-go gains $JS_CONFIG_FILE support (it had none), and iec60870-5 now
// requires $JS_CONFIG_FILE to name a readable file before honouring it, where
// before it took the variable unconditionally and failed later.
//
// The C++ client and server use different fallbacks (~/json-scada/conf/... vs
// /json-scada/conf/...), so the fallback stays a parameter.
func ResolvePath(primary, fallback, argPath string) string {
	if argPath != "" && FileExists(argPath) {
		return ExpandHome(argPath)
	}
	if env := os.Getenv(ConfigFileEnvVar); env != "" && FileExists(env) {
		return ExpandHome(env)
	}
	if primary != "" && FileExists(primary) {
		return ExpandHome(primary)
	}
	if fallback != "" {
		return ExpandHome(fallback)
	}
	return ExpandHome(primary)
}

// Load reads and parses the config file, trimming the three string fields
// every driver trims.
func Load(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(ExpandHome(path))
	if err != nil {
		return cfg, fmt.Errorf("failed to read config file %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse JSON config file %s: %w", path, err)
	}
	cfg.MongoConnectionString = strings.TrimSpace(cfg.MongoConnectionString)
	cfg.MongoDatabaseName = strings.TrimSpace(cfg.MongoDatabaseName)
	cfg.NodeName = strings.TrimSpace(cfg.NodeName)
	return cfg, nil
}

// Validate reports the missing mandatory fields, in the order the drivers
// check them so the message a user sees does not change.
func Validate(cfg Config, path string) error {
	if cfg.MongoConnectionString == "" {
		return fmt.Errorf("missing MongoDB connection string in JSON config file %s", path)
	}
	if cfg.MongoDatabaseName == "" {
		return fmt.Errorf("missing MongoDB database name in JSON config file %s", path)
	}
	if cfg.NodeName == "" {
		return fmt.Errorf("missing nodeName parameter in JSON config file %s", path)
	}
	return nil
}

// LoadResolved is ResolvePath + Load + Validate, which is what most drivers
// do. It returns the resolved path so the caller can name it in its log.
func LoadResolved(primary, fallback, argPath string) (Config, string, error) {
	path := ResolvePath(primary, fallback, argPath)
	cfg, err := Load(path)
	if err != nil {
		return cfg, path, err
	}
	return cfg, path, Validate(cfg, path)
}
