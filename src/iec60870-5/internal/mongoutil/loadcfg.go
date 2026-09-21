/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - config loading
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
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

package mongoutil

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"iec60870-5/internal/model"

	"github.com/riclolsen/json-scada/src/go-common/jsmongo"
)

// LoadInstance finds the enabled driver instance and validates the node name
// (port of the C# startup instance processing).
func LoadInstance(db *mongo.Database, driverName string, instanceNumber int, nodeName string) (*model.DriverInstance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var inst model.DriverInstance
	err := db.Collection(jsmongo.ProtocolDriverInstancesCollectionName).FindOne(ctx, bson.M{
		"protocolDriver":               driverName,
		"protocolDriverInstanceNumber": instanceNumber,
		"enabled":                      true,
	}).Decode(&inst)
	if err != nil {
		return nil, fmt.Errorf("driver instance [%d] not found in configuration", instanceNumber)
	}
	nodeFound := len(inst.NodeNames) == 0
	for _, name := range inst.NodeNames {
		if nodeName == name {
			nodeFound = true
		}
	}
	if !nodeFound {
		return nil, fmt.Errorf("node '%s' not found in instances configuration", nodeName)
	}
	return &inst, nil
}

// LoadConns loads the enabled connections of the driver instance.
func LoadConns(db *mongo.Database, driverName string, instanceNumber int) ([]model.ConnCfg, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cur, err := db.Collection(jsmongo.ProtocolConnectionsCollectionName).Find(ctx, bson.M{
		"protocolDriver":               driverName,
		"protocolDriverInstanceNumber": instanceNumber,
		"enabled":                      true,
	})
	if err != nil {
		return nil, err
	}
	var conns []model.ConnCfg
	if err := cur.All(ctx, &conns); err != nil {
		return nil, err
	}
	return conns, nil
}
