/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

// Command modbusclient is the {json:scada} Modbus client (master) driver.
// Usage: modbus-client [instance] [logLevel] [configFile]
package main

import (
	"os"

	"modbus-go/internal/clientapp"
)

func main() { os.Exit(clientapp.Main(os.Args)) }
