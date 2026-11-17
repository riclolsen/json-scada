/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package cs101util

import "testing"

func TestIsTCPAddress(t *testing.T) {
	cases := map[string]bool{
		"192.168.0.10:2404": true,
		"rtu-gw.local:4001": true,
		"[::1]:2404":        true,
		"COM1":              false,
		"/dev/ttyUSB0":      false,
		"/dev/serial/by-path/pci-0000:00:14.0-usb-0:1:1.0-port0": false,
		`\\.\COM10`:     false,
		"host:notaport": false,
		":2404":         false,
		"host:70000":    false,
	}
	for in, want := range cases {
		if got := IsTCPAddress(in); got != want {
			t.Errorf("IsTCPAddress(%q) = %v, want %v", in, got, want)
		}
	}
}
