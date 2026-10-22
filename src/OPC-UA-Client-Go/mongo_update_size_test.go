/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 */

package main

import (
	"strings"
	"testing"
	"time"
)

// An update over maxUpdateBytes is discarded; one under it is written.
func TestUpdateSizeLimit(t *testing.T) {
	base := OPCValue{Address: "ns=1;s=Big", Asdu: "string", ConnNumber: 81, ConnName: "PLC1",
		Quality: true, ServerTimestamp: time.Now()}

	small := base
	small.ValueString = strings.Repeat("x", 1<<20)
	if updateModel(small) == nil {
		t.Error("a 1 MB value must be written")
	}

	big := base
	big.ValueString = strings.Repeat("x", maxUpdateBytes)
	if updateModel(big) != nil {
		t.Error("a value over the limit must be discarded")
	}

	// A numeric array whose JSON is far below the limit but whose BSON is
	// above it: the pre-filter must not let it through.
	arr := base
	arr.Asdu = "double[]"
	arr.IsArray = true
	arr.ValueJSON = "[" + strings.Repeat("1,", 400_000) + "1]"
	if updateModel(arr) != nil {
		t.Errorf("a %d-byte JSON array that encodes over the limit must be discarded", len(arr.ValueJSON))
	}
}
