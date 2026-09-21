/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - configuration entry
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
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

// Package jscfg is what remains of this module's own configuration library
// after the shared parts moved to go-common: one function, holding the
// stricter command-line contract of the five IEC 60870-5 binaries.
//
// parity: these drivers exit with a diagnostic when the instance number or
// the log level is not a number, where dnp3-go and the flat drivers log the
// problem and carry on with the default. That difference is deliberate and is
// the only reason this package still exists — everything else it used to hold
// is now jsconfig, jslog or jsmongo.
package jscfg

import (
	"fmt"
	"os"
	"strconv"

	"github.com/riclolsen/json-scada/src/go-common/jsconfig"
	"github.com/riclolsen/json-scada/src/go-common/jslog"
)

// Read parses the CLI arguments (instance number, log level, config file
// path) and loads conf/json-scada.json, failing on anything malformed.
func Read() (cfg jsconfig.Config, instanceNumber int, err error) {
	instanceNumber = 1
	if len(os.Args) > 1 {
		i, cerr := strconv.Atoi(os.Args[1])
		if cerr != nil {
			return cfg, 0, fmt.Errorf("instance parameter should be a number: %v", cerr)
		}
		instanceNumber = i
	}
	if len(os.Args) > 2 {
		i, cerr := strconv.Atoi(os.Args[2])
		if cerr != nil {
			return cfg, 0, fmt.Errorf("log level parameter should be a number: %v", cerr)
		}
		jslog.SetLevel(i)
	}

	argPath := ""
	if len(os.Args) > 3 {
		argPath = os.Args[3]
	}
	path := jsconfig.ResolvePath(
		jsconfig.DefaultConfigFilePath, jsconfig.WindowsFallbackPath, argPath)

	jslog.Log(jslog.LevelBasic, "%s", "Reading config file "+path)
	cfg, err = jsconfig.Load(path)
	if err != nil {
		return cfg, 0, err
	}
	if err = jsconfig.Validate(cfg, path); err != nil {
		return cfg, 0, err
	}
	return cfg, instanceNumber, nil
}
