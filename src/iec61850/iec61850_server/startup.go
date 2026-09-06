/*
 * IEC 61850 MMS Server driver for {json:scada}, in Go.
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

// Startup: the driver's own banner, configuration policy and library
// diagnostics. Everything generic is in go-common.

package main

import (
	"log/slog"
	"os"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
)

// libLogger returns the diagnostic logger handed to the go-iec61850 server.
// Only at debug level, where it dumps protocol PDUs.
//
// parity: this is slog's own text format, not the driver's line format.
// Unchanged from before the go-common migration.
func libLogger() *slog.Logger {
	return jslog.NewTextSlogger()
}

// readConfigFile parses the command line and loads conf/json-scada.json.
// The log level comes from the command line and overrides the instance
// document, as in the C# driver.
func readConfigFile() (cfg jsconfig.Config, instanceNumber int) {
	args := jsconfig.ParseArgs(os.Args)
	instanceNumber = args.InstanceNumber

	logBanner()

	path := jsconfig.ResolvePath(JSONConfigFilePath, JSONConfigFilePathAlt, args.ConfigFilePath)
	if !jsconfig.FileExists(path) {
		jslog.Fatal("Missing config file %s", JSONConfigFilePath)
	}

	jslog.Log(jslog.LevelNoLog, "Reading config file %s", path)
	cfg, err := jsconfig.Load(path)
	if err != nil {
		jslog.Fatal("%v", err)
	}
	if err := jsconfig.Validate(cfg, path); err != nil {
		jslog.Fatal("%v", err)
	}
	// parity: this driver reports both names only after every check passes,
	// where the client driver reports the database name before the nodeName
	// check. Kept as it was.
	jslog.Log(jslog.LevelNoLog, "MongoDB database name: %s", cfg.MongoDatabaseName)
	jslog.Log(jslog.LevelNoLog, "Node name: %s", cfg.NodeName)

	return cfg, instanceNumber
}
