/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
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

// Startup: the driver's own banner and configuration policy. Everything
// generic (parsing, path resolution, validation, logging) is in go-common.

package main

import (
	"os"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
)

// readConfigFile parses the command line and loads conf/json-scada.json.
//
// parity: the log level comes from the command line only. The C# driver
// reads logLevel into the instance document type but never assigns it to
// MainClass.LogLevel, so both binaries answer to the same knob.
func readConfigFile() (cfg jsconfig.Config, instanceNumber int) {
	args := jsconfig.ParseArgs(os.Args)
	instanceNumber = args.InstanceNumber

	jslog.Log(jslog.LevelNoLog, "%s", CopyrightMessage)
	jslog.Log(jslog.LevelNoLog, "Driver version %s", DriverVersion)
	jslog.Log(jslog.LevelNoLog, "Using the gopcua library, %s.", LibraryVersion)
	jslog.Log(jslog.LevelNoLog, "Log level: %d", jslog.Level())

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
	jslog.Log(jslog.LevelNoLog, "MongoDB database name: %s", cfg.MongoDatabaseName)
	jslog.Log(jslog.LevelNoLog, "Node name: %s", cfg.NodeName)

	return cfg, instanceNumber
}
