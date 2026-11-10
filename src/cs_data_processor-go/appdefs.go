/*
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

// Application level definitions, kept aligned with the Node.js
// implementation's app-defs.js so both register under the same process name.

package main

const (
	// AppName is the processName used in the processInstances collection.
	// It matches the Node.js implementation so this binary is a drop-in
	// replacement (never run both at once on the same instance number).
	AppName = "CS_DATA_PROCESSOR"
	// EnvPrefix prefixes every environment variable read by this process.
	EnvPrefix = "JS_CSDATAPROC_"
	// AppMsg is the banner printed at startup.
	AppMsg = "{json:scada} - Change Stream Data Processor (Go)"
	// AppVersion of this implementation.
	AppVersion = "0.1.8-go.2"
	// AppImplementation identifies this build in the latency metrics so the
	// Node.js and Go numbers can be told apart when compared side by side.
	AppImplementation = "go"
)

// Collection names, same defaults as load-config.js.
const (
	RealtimeDataCollectionName            = "realtimeData"
	HistCollectionName                    = "hist"
	SoeDataCollectionName                 = "soeData"
	ProtocolDriverInstancesCollectionName = "protocolDriverInstances"
	ProtocolConnectionsCollectionName     = "protocolConnections"
	ProcessInstancesCollectionName        = "processInstances"
)

const (
	// beepPointKey and cntUpdatesPointKey are the reserved system point keys.
	beepPointKey       = -1.0
	cntUpdatesPointKey = -2.0
	// lowestPriorityThatBeeps: alarms of priority 0 and 1 raise the beep.
	lowestPriorityThatBeeps = 1.0
)
