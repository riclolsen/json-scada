/*
 * DNP3 Outstation Server Protocol driver for {json:scada}, in Go.
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

// Reading the driver instance and its connections.

package serverapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// loadInstance validates that this instance is enabled and may run on this
// node. The server has no redundancy: it runs on every enabled node.
func loadInstance(ctx context.Context, db *mongo.Database, instanceNumber int, nodeName string, applyLogLevel bool) error {
	tctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var doc bson.M
	err := db.Collection(jsmongo.ProtocolDriverInstancesCollectionName).FindOne(tctx, bson.M{
		"protocolDriver":               ProtocolDriverName,
		"protocolDriverInstanceNumber": instanceNumber,
	}).Decode(&doc)
	if err != nil {
		return fmt.Errorf("protocol driver instance not found in the database")
	}
	if !jsmongo.GetBool(doc, "enabled", false) {
		return fmt.Errorf("protocol driver instance is disabled in the database")
	}
	if applyLogLevel {
		if _, ok := doc["logLevel"]; ok {
			jslog.SetLevel(jsmongo.GetInt(doc, "logLevel", jslog.LevelBasic))
		}
	}

	// An empty or absent nodeNames array means any node may run it, which is
	// what the C++ server's cnt > 0 test amounts to.
	names := jsmongo.GetStringArray(doc, "nodeNames")
	if len(names) == 0 {
		return nil
	}
	for _, n := range names {
		if n == nodeName {
			jslog.Log(jslog.LevelBasic, "Node Name: %s", n)
			return nil
		}
	}
	return fmt.Errorf("node name not found in the protocol driver instance configuration")
}

// loadConnections reads the enabled connections of the instance.
func loadConnections(ctx context.Context, db *mongo.Database, instanceNumber int) ([]*Connection, error) {
	tctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	docs, err := jsmongo.FindAll(tctx, db.Collection(jsmongo.ProtocolConnectionsCollectionName), bson.M{
		"protocolDriver":               ProtocolDriverName,
		"protocolDriverInstanceNumber": instanceNumber,
		"enabled":                      true,
	})
	if err != nil {
		return nil, err
	}
	if len(docs) == 0 {
		return nil, fmt.Errorf("no protocol connections found for the protocol driver instance")
	}

	conns := make([]*Connection, 0, len(docs))
	for _, doc := range docs {
		c := connectionFromDoc(doc)
		jslog.Log(jslog.LevelBasic, "%s - Connection Number: %d", c.Name, c.ProtocolConnectionNumber)
		conns = append(conns, c)
	}
	return conns, nil
}

// connectionFromDoc maps a protocolConnections document onto a Connection, with
// the defaults the C++ server applies.
func connectionFromDoc(doc bson.M) *Connection {
	return &Connection{
		ProtocolConnectionNumber: jsmongo.GetInt(doc, "protocolConnectionNumber", 0),
		Name:                     jsmongo.GetString(doc, "name", ""),
		Description:              jsmongo.GetString(doc, "description", ""),
		Enabled:                  jsmongo.GetBool(doc, "enabled", false),
		CommandsEnabled:          jsmongo.GetBool(doc, "commandsEnabled", false),
		AutoCreateTags:           jsmongo.GetBool(doc, "autoCreateTags", false),
		Topics:                   jsmongo.GetStringArray(doc, "topics"),
		ConnectionMode:           strings.ToUpper(jsmongo.GetString(doc, "connectionMode", "TCP PASSIVE")),
		IPAddressLocalBind:       jsmongo.GetString(doc, "ipAddressLocalBind", ""),
		IPAddresses:              jsmongo.GetStringArray(doc, "ipAddresses"),
		PortName:                 jsmongo.GetString(doc, "portName", ""),
		BaudRate:                 jsmongo.GetInt(doc, "baudRate", 9600),
		Parity:                   strings.ToUpper(jsmongo.GetString(doc, "parity", "None")),
		StopBits:                 strings.ToUpper(jsmongo.GetString(doc, "stopBits", "One")),
		Handshake:                strings.ToUpper(jsmongo.GetString(doc, "handshake", "None")),
		AsyncOpenDelay:           jsmongo.GetInt(doc, "asyncOpenDelay", 0),
		LocalLinkAddress:         jsmongo.GetInt(doc, "localLinkAddress", 1),
		RemoteLinkAddress:        jsmongo.GetInt(doc, "remoteLinkAddress", 1),
		TimeSyncInterval:         jsmongo.GetInt(doc, "timeSyncInterval", 0),
		TimeSyncMode:             jsmongo.GetInt(doc, "timeSyncMode", 0),
		HoursShift:               jsmongo.GetDouble(doc, "hoursShift", 0),
		EnableUnsolicited:        jsmongo.GetBool(doc, "enableUnsolicited", true),
		ServerQueueSize:          jsmongo.GetInt(doc, "serverQueueSize", 1000),
		LocalCertFilePath:        jsmongo.GetString(doc, "localCertFilePath", ""),
		PrivateKeyFilePath:       jsmongo.GetString(doc, "privateKeyFilePath", ""),
		PeerCertFilePath:         jsmongo.GetString(doc, "peerCertFilePath", ""),
		CipherList:               jsmongo.GetString(doc, "cipherList", ""),
		AllowTLSv10:              jsmongo.GetBool(doc, "allowTLSv10", false),
		AllowTLSv11:              jsmongo.GetBool(doc, "allowTLSv11", false),
		AllowTLSv12:              jsmongo.GetBool(doc, "allowTLSv12", true),
		AllowTLSv13:              jsmongo.GetBool(doc, "allowTLSv13", true),
	}
}
