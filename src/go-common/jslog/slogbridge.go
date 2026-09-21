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

// Bridges a protocol library's *slog.Logger onto the driver's own logger, so
// both streams share one mutex and one timestamp format and cannot interleave
// mid-line. Promoted from dnp3-go/internal/jscfg/slogbridge.go, which
// replaced the DNP3LogBridge : ILogHandler of the C++ client.

package jslog

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

// A library logs protocol traffic at Debug and session milestones at Info.
// The mapping mirrors mapLogLevel() of the C++ client: warnings and errors are
// visible at the basic level, session events from detailed, traffic only at
// debug.
func driverLevelFor(level slog.Level) int {
	switch {
	case level >= slog.LevelWarn:
		return LevelBasic
	case level >= slog.LevelInfo:
		return LevelDetailed
	default:
		return LevelDebug
	}
}

type slogHandler struct {
	prefix string
	attrs  []slog.Attr
	groups []string
}

// NewStackLogger returns an *slog.Logger that writes through this package,
// so library lines are indistinguishable from driver lines. prefix is
// normally the connection name.
func NewStackLogger(prefix string) *slog.Logger {
	return slog.New(&slogHandler{prefix: prefix})
}

// NewTextSlogger returns a plain slog text logger on stdout at debug level,
// or nil below LevelDebug.
//
// parity: this is what iec61850_client/log.go:libLogger() hands to
// go-iec61850, and its output shape (key=value, slog's own timestamp) differs
// from NewStackLogger's. Kept as a separate constructor so that driver's
// debug output does not change.
func NewTextSlogger() *slog.Logger {
	if Level() < LevelDebug {
		return nil
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func (h *slogHandler) Enabled(_ context.Context, level slog.Level) bool {
	return Level() >= driverLevelFor(level)
}

func (h *slogHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	if h.prefix != "" {
		b.WriteString(h.prefix)
		b.WriteString(" - ")
	}
	b.WriteString(r.Message)
	for _, a := range h.attrs {
		appendAttr(&b, h.groups, a)
	}
	r.Attrs(func(a slog.Attr) bool {
		appendAttr(&b, h.groups, a)
		return true
	})
	Log(driverLevelFor(r.Level), "%s", b.String())
	return nil
}

func appendAttr(b *strings.Builder, groups []string, a slog.Attr) {
	if a.Equal(slog.Attr{}) {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		for _, sub := range a.Value.Group() {
			appendAttr(b, append(groups, a.Key), sub)
		}
		return
	}
	b.WriteByte(' ')
	for _, g := range groups {
		b.WriteString(g)
		b.WriteByte('.')
	}
	b.WriteString(a.Key)
	b.WriteByte('=')
	fmt.Fprintf(b, "%v", a.Value.Resolve().Any())
}

func (h *slogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	n := *h
	n.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &n
}

func (h *slogHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	n := *h
	n.groups = append(append([]string(nil), h.groups...), name)
	return &n
}
