/*
 * Shared {json:scada} driver support library, in Go.
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

// Package jslog is the levelled logger shared by the Go drivers. It is a
// merge of the per-driver log.go files, which were near-identical apart from
// their timestamp layout.
//
// Timestamps are unified. Five different layouts were in use before this
// package existed; every driver now emits TimeFormat, the C# "o" round-trip
// form that the OPC-UA and both IEC 61850 drivers already used and that the
// C# originals emit. dnp3-go moved from 3 fractional digits and iec60870-5
// from RFC3339Nano (which stripped trailing zeros, so its field width varied
// line to line). Anything that parsed those two drivers' logs by column
// position needs updating; anything parsing the bracketed timestamp as
// RFC 3339 does not.
package jslog

import (
	"bufio"
	"fmt"
	"os"
	"sync"
	"time"
)

// Log levels, same numbering as the C#/C++ drivers and the instance document.
const (
	LevelNoLog    = 0
	LevelBasic    = 1
	LevelDetailed = 2
	LevelDebug    = 3
)

// TimeFormat is the one timestamp layout every driver emits: C#'s "o"
// round-trip format, seven fractional digits always present, with the local
// UTC offset. Fixed width, so a log line can be split on column as well as
// parsed.
const TimeFormat = "2006-01-02T15:04:05.0000000Z07:00"

var (
	mu         sync.Mutex
	level      = LevelBasic
	timeFormat = TimeFormat
	useUTC     bool
	writer     = bufio.NewWriterSize(os.Stdout, 32*1024)
	dirty      bool
)

func init() {
	// Flush regularly so a log under a service manager stays live without
	// paying a syscall per line during a burst of data.
	go func() {
		t := time.NewTicker(200 * time.Millisecond)
		for range t.C {
			Flush()
		}
	}()
}

// SetTimeFormat overrides the timestamp layout.
//
// Drivers must not call this: the layout is unified, and a driver that sets
// its own would reintroduce the divergence this package removed. It exists
// for tests that need a deterministic timestamp.
func SetTimeFormat(layout string) {
	mu.Lock()
	timeFormat = layout
	mu.Unlock()
}

// SetUTC makes timestamps render in UTC rather than local time.
// Only cs_data_processor-go does this.
func SetUTC(v bool) {
	mu.Lock()
	useUTC = v
	mu.Unlock()
}

// SetLevel sets the effective verbosity.
func SetLevel(n int) {
	mu.Lock()
	level = n
	mu.Unlock()
}

// Level returns the effective verbosity.
func Level() int {
	mu.Lock()
	defer mu.Unlock()
	return level
}

// Log writes one timestamped line when the level admits it.
//
// format is used as a literal when no arguments are given, so a caller that
// passes a message containing a percent sign is safe.
func Log(lvl int, format string, a ...any) {
	mu.Lock()
	if level < lvl {
		mu.Unlock()
		return
	}
	msg := format
	if len(a) > 0 {
		msg = fmt.Sprintf(format, a...)
	}
	now := time.Now()
	if useUTC {
		now = now.UTC()
	}
	fmt.Fprintf(writer, "[%s] %s\n", now.Format(timeFormat), msg)
	dirty = true
	mu.Unlock()
}

// Flush drains the buffered writer. Called by the background ticker, and
// before any exit path.
func Flush() {
	mu.Lock()
	if dirty {
		_ = writer.Flush()
		dirty = false
	}
	mu.Unlock()
}

// Fatal logs, flushes and terminates with the exit code the C#/C++ drivers
// use for a configuration error.
func Fatal(format string, a ...any) {
	Log(LevelNoLog, format, a...)
	Flush()
	os.Exit(-1)
}
