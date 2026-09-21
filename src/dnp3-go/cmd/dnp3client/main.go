/*
 * DNP3 Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * A drop-in alternative to the C++ driver of src/dnp3/Dnp3ClientCpp: same
 * protocol driver name, same configuration documents, same MongoDB semantics,
 * with no opendnp3, mongo-cxx-driver or OpenSSL build.
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

	"dnp3-go/internal/clientapp"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
)

// The client's own fallback, as in Dnp3ClientCpp.
const altConfigFilePath = "~/json-scada/conf/json-scada.json"

func main() {
	jslog.Log(jslog.LevelNoLog, "%s", clientapp.DriverMessage)
	jslog.Log(jslog.LevelNoLog, "Driver version %s", clientapp.DriverVersion)
	jslog.Log(jslog.LevelDetailed, "Main: Starting driver...")

	args := jsconfig.ParseArgs(os.Args)
	if args.LogLevelFromCLI {
		jslog.Log(jslog.LevelDetailed, "Main: Log level set to %d", jslog.Level())
	}
	jslog.Log(jslog.LevelBasic, "ProtocolDriverInstanceNumber: %d", args.InstanceNumber)

	path := jsconfig.ResolvePath(jsconfig.DefaultConfigFilePath, altConfigFilePath, args.ConfigFilePath)
	cfg, err := jsconfig.Load(path)
	if err != nil {
		jslog.Fatal("Could not open the configuration file: %s - %v", path, err)
	}
	jslog.Log(jslog.LevelBasic, "ConfigurationFile: %s", path)

	if cfg.MongoConnectionString == "" || cfg.MongoDatabaseName == "" || cfg.NodeName == "" {
		jslog.Fatal("Invalid JSON-SCADA configuration")
	}

	clientapp.Run(args, cfg)
}
