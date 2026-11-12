/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package srvapp

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// pathNotViable is MongoDB's error for a dotted $set below a null element.
const pathNotViable = 28

// updateConnectionStats applies a $set of dotted stats.* paths to a
// protocolConnections document. Connection documents are created with
// stats: null, where such a $set fails with error 28: the null is replaced
// with an empty document and the update retried once.
func updateConnectionStats(ctx context.Context, coll *mongo.Collection, connNumber any, set bson.M) error {
	filter := bson.M{"protocolConnectionNumber": connNumber}
	_, err := coll.UpdateOne(ctx, filter, bson.M{"$set": set})
	if !isPathNotViable(err) {
		return err
	}
	if _, err := coll.UpdateOne(ctx,
		bson.M{"protocolConnectionNumber": connNumber, "stats": nil},
		bson.M{"$set": bson.M{"stats": bson.M{}}}); err != nil {
		return err
	}
	_, err = coll.UpdateOne(ctx, filter, bson.M{"$set": set})
	return err
}

func isPathNotViable(err error) bool {
	var se mongo.ServerError
	return errors.As(err, &se) && se.HasErrorCode(pathNotViable)
}
