/*
 * ICCP/TASE.2 Server Driver for JSON-SCADA
 * {json:scada} - Copyright (c) 2020-2025 - Ricardo L. Olsen
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

package main

import (
	"fmt"
	"log"
	"time"
)

const (
	LogLevelMin      = 0
	LogLevelNormal   = 1
	LogLevelDetailed = 2
	LogLevelDebug    = 3
)

var currentLogLevel = LogLevelNormal

// LogMsg logs a message at the given level if the current log level allows it.
func LogMsg(level int, format string, v ...interface{}) {
	if level <= currentLogLevel {
		msg := fmt.Sprintf(format, v...)
		log.Printf("%s - %s", time.Now().Format("2006-01-02T15:04:05.000Z07:00"), msg)
	}
}

// CheckFatalError logs and exits if there is an error.
func CheckFatalError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
