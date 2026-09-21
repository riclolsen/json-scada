/*
 * DNP3 Client and Server Protocol drivers for {json:scada}, in Go.
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

// Endpoint string handling. Port of parseEndpoint() of the C++ client.

package dnp3util

import (
	"net"
	"strconv"
	"strings"
)

// DefaultDNP3Port is the port assumed when an address carries none.
const DefaultDNP3Port = 20000

// ParseEndpoint splits "host:port" into its parts, defaulting the port. An
// empty host becomes 0.0.0.0, which is what a bind address without one means.
func ParseEndpoint(text string, defaultHost string) (host string, port int) {
	host, port = defaultHost, DefaultDNP3Port
	text = strings.TrimSpace(text)
	if text == "" {
		return host, port
	}
	// Split on the last colon so that a bare IPv6 address is not mangled; an
	// IPv6 address with a port has to be bracketed, which net.SplitHostPort
	// handles.
	if h, p, err := net.SplitHostPort(text); err == nil {
		if n, err := strconv.Atoi(p); err == nil {
			if h == "" {
				h = defaultHost
			}
			return h, n
		}
	}
	return text, port
}

// JoinEndpoint renders an address for the channel constructors.
func JoinEndpoint(host string, port int) string {
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// NormalizeEndpoint parses and re-renders an address, applying the default
// host and port. Used for both dialling and for grouping connections that
// share a channel, so that "0.0.0.0:20000" and ":20000" are one endpoint.
func NormalizeEndpoint(text, defaultHost string) string {
	host, port := ParseEndpoint(text, defaultHost)
	return JoinEndpoint(host, port)
}

// HostOf returns just the host part of an address, for the local bind address
// of an active connection, where a port makes no sense.
func HostOf(text, defaultHost string) string {
	host, _ := ParseEndpoint(text, defaultHost)
	return host
}
