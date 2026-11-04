/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

// Command modbusserver is the {json:scada} Modbus server (slave) driver.
// Usage: modbus-server [instance] [logLevel] [configFile]
package main

import (
	"os"

	"modbus-go/internal/serverapp"
)

func main() { os.Exit(serverapp.Main(os.Args)) }
