/*
 * ICCP/TASE.2 Client Driver for JSON-SCADA
 * {json:scada} - Copyright (c) 2020-present - Ricardo L. Olsen
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
	"strings"
	"sync/atomic"
	"time"

	"github.com/riclolsen/tase2/tase2"
)

const (
	LogLevelMin      = 0
	LogLevelNormal   = 1
	LogLevelDetailed = 2
	LogLevelDebug    = 3
)

// logLevel is read from library goroutines (log handlers, report callbacks),
// so it is atomic.
var logLevel atomic.Int32

func init() { logLevel.Store(LogLevelNormal) }

func currentLogLevel() int { return int(logLevel.Load()) }

func setLogLevel(level int) { logLevel.Store(int32(level)) }

// LogMsg logs a message at the given level if the current log level allows it.
func LogMsg(level int, format string, v ...interface{}) {
	if level <= currentLogLevel() {
		logLine(time.Now(), fmt.Sprintf(format, v...))
	}
}

func logLine(t time.Time, msg string) {
	log.Printf("%s - %s", t.Format("2006-01-02T15:04:05.000Z07:00"), msg)
}

// configureTASE2Logging sets the library's level filter from the driver log
// level and routes process-level library messages (iso, mms and tase2)
// through the driver log. Endpoints add connection attribution with
// tase2LogHandler.
func configureTASE2Logging(level int) {
	switch {
	case level >= LogLevelDebug:
		tase2.SetLogLevel(tase2.LogLevelDebug)
	case level >= LogLevelDetailed:
		tase2.SetLogLevel(tase2.LogLevelInfo)
	default:
		tase2.SetLogLevel(tase2.LogLevelError)
	}
	tase2.SetLogHandler(tase2LogHandler(""))
}

// tase2LogHandler returns a library log handler that writes through the
// driver log, prefixed with the connection name (label) and, for messages
// about one transport connection, its id, remote address and peer AP title.
func tase2LogHandler(label string) func(tase2.LogRecord) {
	return func(r tase2.LogRecord) {
		level := LogLevelDebug
		switch r.Level {
		case tase2.LogLevelError:
			level = LogLevelMin
		case tase2.LogLevelInfo:
			level = LogLevelDetailed
		}
		if level <= currentLogLevel() {
			logLine(r.Time, formatTASE2LogRecord(label, r))
		}
	}
}

func formatTASE2LogRecord(label string, r tase2.LogRecord) string {
	var b strings.Builder
	b.WriteString("TASE2")
	if label != "" {
		b.WriteString(" " + label)
	}
	if r.ConnID != 0 {
		fmt.Fprintf(&b, " [conn %d %s", r.ConnID, r.RemoteAddr)
		if r.PeerAPTitle != "" {
			fmt.Fprintf(&b, " %s/%d", r.PeerAPTitle, r.PeerAEQualifier)
		}
		b.WriteString("]")
	}
	fmt.Fprintf(&b, " %s - %s", r.Package, r.Message)
	return b.String()
}

// CheckFatalError logs and exits if there is an error.
func CheckFatalError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
