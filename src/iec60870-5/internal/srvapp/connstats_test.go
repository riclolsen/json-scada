/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package srvapp

import (
	"errors"
	"fmt"
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// A dotted $set below stats: null comes back as a write error with code 28;
// only that error triggers the null replacement.
func TestIsPathNotViable(t *testing.T) {
	we := mongo.WriteException{WriteErrors: []mongo.WriteError{{Code: 28, Message: "Cannot create field 'isConnected' in element {stats: null}"}}}
	if !isPathNotViable(we) || !isPathNotViable(fmt.Errorf("wrapped: %w", we)) {
		t.Error("code 28 write error not recognised")
	}
	other := mongo.WriteException{WriteErrors: []mongo.WriteError{{Code: 11000}}}
	if isPathNotViable(other) || isPathNotViable(errors.New("x")) || isPathNotViable(nil) {
		t.Error("only code 28 must match")
	}
}
