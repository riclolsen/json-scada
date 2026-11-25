/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

// Command sunspecclient is the {json:scada} SunSpec client driver.
// Usage: sunspec-client [instance] [logLevel] [configFile]
package main

import (
	"os"

	"modbus-go/internal/sunspecapp"
)

func main() { os.Exit(sunspecapp.Main(os.Args)) }
