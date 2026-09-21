/*
 * DNP3 Client Protocol driver for {json:scada}, in Go.
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

// Reading the driver instance and its connections. Port of loadConnections().

package clientapp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/riclolsen/json-scada/src/go-common/jslog"
	"github.com/riclolsen/json-scada/src/go-common/jsmongo"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// loadInstance reads the enabled driver instance document, applying its log
// level unless the command line already set one.
func loadInstance(ctx context.Context, db *mongo.Database, instanceNumber int, applyLogLevel bool) error {
	tctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var doc bson.M
	err := db.Collection(jsmongo.ProtocolDriverInstancesCollectionName).FindOne(tctx, bson.M{
		"protocolDriver":               ProtocolDriverName,
		"protocolDriverInstanceNumber": instanceNumber,
		"enabled":                      true,
	}).Decode(&doc)
	if err != nil {
		return fmt.Errorf("driver instance not found")
	}
	if applyLogLevel {
		if _, ok := doc["logLevel"]; ok {
			jslog.SetLevel(jsmongo.GetInt(doc, "logLevel", jslog.LevelBasic))
		}
	}
	return nil
}

// loadConnections reads the enabled connections of the instance and preloads,
// for every connection with autoCreateTags, the addresses already in
// realtimeData so a restart does not create them a second time.
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

	conns := make([]*Connection, 0, len(docs))
	for _, doc := range docs {
		conns = append(conns, connectionFromDoc(doc))
	}
	if len(conns) == 0 {
		return nil, fmt.Errorf("no DNP3 connections found")
	}

	rtd := db.Collection(jsmongo.RealtimeDataCollectionName)
	for _, conn := range conns {
		conn.insertedAddresses = map[[2]int]bool{}
		if !conn.AutoCreateTags {
			continue
		}
		jslog.Log(jslog.LevelBasic, "%s - Auto creating tags for connection.", conn.Name)
		if err := preloadAddresses(tctx, rtd, conn); err != nil {
			return nil, err
		}
	}
	return conns, nil
}

// preloadAddresses records the (commonAddress, objectAddress) pairs already
// configured for a connection.
func preloadAddresses(ctx context.Context, rtd *mongo.Collection, conn *Connection) error {
	opts := options.Find().SetProjection(bson.M{
		"protocolSourceCommonAddress": 1,
		"protocolSourceObjectAddress": 1,
	})
	docs, err := jsmongo.FindAll(ctx, rtd,
		bson.M{"protocolSourceConnectionNumber": conn.ProtocolConnectionNumber}, opts)
	if err != nil {
		return err
	}
	for _, d := range docs {
		key := [2]int{
			jsmongo.GetInt(d, "protocolSourceCommonAddress", -1),
			jsmongo.GetInt(d, "protocolSourceObjectAddress", -1),
		}
		conn.insertedAddresses[key] = true
	}
	jslog.Log(jslog.LevelBasic, "%s - Found %d tags in database.", conn.Name, len(docs))
	return nil
}

// connectionFromDoc maps a protocolConnections document onto a Connection,
// with exactly the defaults the C++ driver applies.
func connectionFromDoc(doc bson.M) *Connection {
	c := &Connection{
		ProtocolDriverInstanceNumber: jsmongo.GetInt(doc, "protocolDriverInstanceNumber", 1),
		ProtocolConnectionNumber:     jsmongo.GetInt(doc, "protocolConnectionNumber", 1),
		Name:                         jsmongo.GetString(doc, "name", "NO NAME"),
		Enabled:                      jsmongo.GetBool(doc, "enabled", true),
		CommandsEnabled:              jsmongo.GetBool(doc, "commandsEnabled", true),
		ConnectionMode:               strings.ToUpper(jsmongo.GetString(doc, "connectionMode", "TCP ACTIVE")),
		IPAddressLocalBind:           jsmongo.GetString(doc, "ipAddressLocalBind", ""),
		IPAddresses:                  jsmongo.GetStringArray(doc, "ipAddresses"),
		PortName:                     jsmongo.GetString(doc, "portName", ""),
		BaudRate:                     jsmongo.GetInt(doc, "baudRate", 9600),
		Parity:                       jsmongo.GetString(doc, "parity", "None"),
		StopBits:                     jsmongo.GetString(doc, "stopBits", "One"),
		Handshake:                    jsmongo.GetString(doc, "handshake", "None"),
		AsyncOpenDelay:               jsmongo.GetInt(doc, "asyncOpenDelay", 0),
		AllowTLSv10:                  jsmongo.GetBool(doc, "allowTLSv10", false),
		AllowTLSv11:                  jsmongo.GetBool(doc, "allowTLSv11", false),
		AllowTLSv12:                  jsmongo.GetBool(doc, "allowTLSv12", true),
		AllowTLSv13:                  jsmongo.GetBool(doc, "allowTLSv13", true),
		CipherList:                   jsmongo.GetString(doc, "cipherList", ""),
		LocalCertFilePath:            jsmongo.GetString(doc, "localCertFilePath", ""),
		PeerCertFilePath:             jsmongo.GetString(doc, "peerCertFilePath", ""),
		PrivateKeyFilePath:           jsmongo.GetString(doc, "privateKeyFilePath", ""),
		LocalLinkAddress:             jsmongo.GetInt(doc, "localLinkAddress", 1),
		RemoteLinkAddress:            jsmongo.GetInt(doc, "remoteLinkAddress", 1),
		GIInterval:                   jsmongo.GetInt(doc, "giInterval", 300),
		Class0ScanInterval:           jsmongo.GetInt(doc, "class0ScanInterval", 0),
		Class1ScanInterval:           jsmongo.GetInt(doc, "class1ScanInterval", 0),
		Class2ScanInterval:           jsmongo.GetInt(doc, "class2ScanInterval", 0),
		Class3ScanInterval:           jsmongo.GetInt(doc, "class3ScanInterval", 0),
		TimeSyncMode:                 jsmongo.GetInt(doc, "timeSyncMode", 0),
		EnableUnsolicited:            jsmongo.GetBool(doc, "enableUnsolicited", true),
		AutoCreateTags:               jsmongo.GetBool(doc, "autoCreateTags", false),
	}
	for _, rs := range jsmongo.GetDocArray(doc, "rangeScans") {
		c.RangeScans = append(c.RangeScans, RangeScan{
			Group:        jsmongo.GetInt(rs, "group", 1),
			Variation:    jsmongo.GetInt(rs, "variation", 1),
			StartAddress: jsmongo.GetInt(rs, "startAddress", 0),
			StopAddress:  jsmongo.GetInt(rs, "stopAddress", 0),
			Period:       jsmongo.GetInt(rs, "period", 0),
		})
	}
	return c
}
