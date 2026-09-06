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

package main

import (
	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Driver identity, matching the C# driver so the same instance and
// connection documents drive either binary.
const (
	CopyrightMessage   = "{json:scada} IEC61850 Server Driver (IEC61850-90-2, Go) - Copyright 2020-2026 Ricardo Olsen"
	ProtocolDriverName = "IEC61850_SERVER"
	DriverVersion      = "0.1.2"
	LibraryVersion     = "v0.2.5"
)

// Default config file locations, in the C# driver's resolution order.
const (
	JSONConfigFilePath    = "../conf/json-scada.json"
	JSONConfigFilePathAlt = "c:/json-scada/conf/json-scada.json"
)

// ServerConnection is the single protocolConnections document this driver
// instance serves.
type ServerConnection struct {
	ID                            bson.ObjectID
	ProtocolDriver                string
	ProtocolDriverInstanceNumber  int
	ProtocolConnectionNumber      int
	Name                          string
	Description                   string
	Enabled                       bool
	CommandsEnabled               bool
	IPAddressLocalBind            string
	IPAddresses                   []string
	Topics                        []string
	ServerModeMultiActive         bool
	MaxClientConnections          float64
	MaxQueueSize                  float64
	UseSecurity                   bool
	LocalCertFilePath             string
	PrivateKeyFilePath            string
	PeerCertFilesPaths            []string
	RootCertFilePath              string
	ChainValidation               bool
	AllowOnlySpecificCertificates bool
	AllowTLSv10                   bool
	AllowTLSv11                   bool
	AllowTLSv12                   bool
	AllowTLSv13                   bool
	CipherList                    string
	Username                      string
	Password                      string
}

func logBanner() {
	jslog.Log(jslog.LevelNoLog, "%s", CopyrightMessage)
	jslog.Log(jslog.LevelNoLog, "Driver version %s", DriverVersion)
	jslog.Log(jslog.LevelNoLog, "Using go-iec61850 version %s", LibraryVersion)
	jslog.Log(jslog.LevelNoLog, "Log level: %d", jslog.Level())
}

// --- permissive BSON accessors -------------------------------------------
//
// Configuration numbers are BSON doubles by convention but hand-edited
// documents carry int32/int64/strings. These mirror the C# driver's
// permissive deserializers: read almost anything, produce a number.

// mRaw returns a field exactly as it was stored, so a value copied into a
// command document keeps the BSON type the source tag had.
func mRaw(m bson.M, key string, def any) any {
	if v, ok := m[key]; ok && v != nil {
		return v
	}
	return def
}

// connectionFromDoc maps a protocolConnections document onto the runtime
// struct, applying the same defaults as the C# BsonDefaultValue attributes.
func connectionFromDoc(doc bson.M) *ServerConnection {
	c := &ServerConnection{
		ProtocolDriver:                jsmongo.GetString(doc, "protocolDriver", ""),
		ProtocolDriverInstanceNumber:  jsmongo.GetInt(doc, "protocolDriverInstanceNumber", 1),
		ProtocolConnectionNumber:      jsmongo.GetInt(doc, "protocolConnectionNumber", 1),
		Name:                          jsmongo.GetString(doc, "name", "NO NAME"),
		Description:                   jsmongo.GetString(doc, "description", "SERVER NOT DESCRIPTED"),
		Enabled:                       jsmongo.GetBool(doc, "enabled", true),
		CommandsEnabled:               jsmongo.GetBool(doc, "commandsEnabled", true),
		IPAddressLocalBind:            jsmongo.GetString(doc, "ipAddressLocalBind", "0.0.0.0:102"),
		IPAddresses:                   jsmongo.GetStringArray(doc, "ipAddresses"),
		Topics:                        jsmongo.GetStringArray(doc, "topics"),
		ServerModeMultiActive:         jsmongo.GetBool(doc, "serverModeMultiActive", true),
		MaxClientConnections:          jsmongo.GetDouble(doc, "maxClientConnections", 1),
		MaxQueueSize:                  jsmongo.GetDouble(doc, "maxQueueSize", 5000),
		UseSecurity:                   jsmongo.GetBool(doc, "useSecurity", false),
		LocalCertFilePath:             jsmongo.GetString(doc, "localCertFilePath", ""),
		PrivateKeyFilePath:            jsmongo.GetString(doc, "privateKeyFilePath", ""),
		PeerCertFilesPaths:            jsmongo.GetStringArray(doc, "peerCertFilesPaths"),
		RootCertFilePath:              jsmongo.GetString(doc, "rootCertFilePath", ""),
		ChainValidation:               jsmongo.GetBool(doc, "chainValidation", false),
		AllowOnlySpecificCertificates: jsmongo.GetBool(doc, "allowOnlySpecificCertificates", false),
		AllowTLSv10:                   jsmongo.GetBool(doc, "allowTLSv10", false),
		AllowTLSv11:                   jsmongo.GetBool(doc, "allowTLSv11", false),
		AllowTLSv12:                   jsmongo.GetBool(doc, "allowTLSv12", true),
		AllowTLSv13:                   jsmongo.GetBool(doc, "allowTLSv13", true),
		CipherList:                    jsmongo.GetString(doc, "cipherList", ""),
		Username:                      jsmongo.GetString(doc, "username", ""),
		Password:                      jsmongo.GetString(doc, "password", ""),
	}
	if id, ok := doc["_id"].(bson.ObjectID); ok {
		c.ID = id
	}
	return c
}
