/*
 * DNP3 Outstation Server Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * A drop-in alternative to the C++ driver of src/dnp3/Dnp3Server: same protocol
 * driver name, same configuration documents, same MongoDB semantics, with no
 * opendnp3, mongo-cxx-driver or OpenSSL build.
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

package main

import (
	"os"

	"dnp3-go/internal/serverapp"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
)

// The server's own fallback, as in Dnp3Server.
const altConfigFilePath = "/json-scada/conf/json-scada.json"

func main() {
	jslog.Log(jslog.LevelNoLog, "%s", serverapp.DriverMessage)
	jslog.Log(jslog.LevelNoLog, "Driver version: %s", serverapp.DriverVersion)
	jslog.Log(jslog.LevelNoLog,
		"Usage: %s [ProtocolDriverInstanceNumber] [LogLevel] [ConfigurationFile]", os.Args[0])

	args := jsconfig.ParseArgs(os.Args)
	jslog.Log(jslog.LevelNoLog, "ProtocolDriverInstanceNumber: %d", args.InstanceNumber)
	jslog.Log(jslog.LevelNoLog, "LogLevel: %d", jslog.Level())

	path := jsconfig.ResolvePath(jsconfig.DefaultConfigFilePath, altConfigFilePath, args.ConfigFilePath)
	cfg, err := jsconfig.Load(path)
	if err != nil {
		jslog.Fatal("Could not open the configuration file: %s - %v", path, err)
	}
	jslog.Log(jslog.LevelNoLog, "ConfigurationFile: %s", path)

	if cfg.MongoConnectionString == "" || cfg.MongoDatabaseName == "" {
		jslog.Fatal("MongoDB connection string or database name is empty in %s", path)
	}

	serverapp.Run(args, cfg)
}
