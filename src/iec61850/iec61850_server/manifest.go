/*
 * IEC 61850 MMS Server driver (IEC61850-90-2 gateway) for {json:scada}, in Go.
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

// The tag -> object reference mapping manifest: the IEC 61850-90-2
// name-mapping deliverable, written at startup.

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
)

type manifestEntry struct {
	Tag             string  `json:"tag"`
	PointKey        float64 `json:"pointKey"`
	ObjectReference string  `json:"objectReference"`
	CDC             string  `json:"cdc"`
	IsCommand       bool    `json:"isCommand"`
}

// exportManifest writes log/iec61850_server_map_<conn>.json. Entries are
// sorted by point key so the file is stable between runs and diffable.
func exportManifest(built *BuiltModel, conn *ServerConnection) {
	entries := make([]manifestEntry, 0, len(built.Order))
	for _, mp := range built.Order {
		entries = append(entries, manifestEntry{
			Tag:             mp.Tag,
			PointKey:        mp.PointKey,
			ObjectReference: string(mp.ObjRef),
			CDC:             mp.Kind.String(),
			IsCommand:       mp.IsCommand,
		})
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].PointKey < entries[j].PointKey })

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		jslog.Log(jslog.LevelNoLog, "Could not write mapping manifest: %v", err)
		return
	}

	fname := "iec61850_server_map_" + itoa(conn.ProtocolConnectionNumber) + ".json"
	path := fname
	if st, err := os.Stat("../log"); err == nil && st.IsDir() {
		path = filepath.Join("../log", fname)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		jslog.Log(jslog.LevelNoLog, "Could not write mapping manifest: %v", err)
		return
	}
	jslog.Log(jslog.LevelBasic, "Mapping manifest written: %s (%d points)", path, len(entries))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
