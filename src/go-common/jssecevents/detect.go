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

package jssecevents

import (
	"bufio"
	"context"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Tracker remembers the last value seen per key (a peer address, an
// identity field) to detect a change of it.
type Tracker struct {
	mu   sync.Mutex
	last map[string]string
}

// Seed records a value known from before (e.g. stored statistics) without
// reporting it.
func (t *Tracker) Seed(key, value string) {
	if value == "" {
		return
	}
	t.mu.Lock()
	if t.last == nil {
		t.last = map[string]string{}
	}
	if _, ok := t.last[key]; !ok {
		t.last[key] = value
	}
	t.mu.Unlock()
}

// Observe records value for key and returns the previous one when it
// differs. An empty value is ignored: unknown is not a change.
func (t *Tracker) Observe(key, value string) (previous string, changed bool) {
	if value == "" {
		return "", false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.last == nil {
		t.last = map[string]string{}
	}
	prev, ok := t.last[key]
	t.last[key] = value
	return prev, ok && prev != value
}

// RateLimit detects more than Max hits of a key within Window. Exceeded
// reports true once per window in which the limit was passed, so a flood
// gives one event per window, not one per hit.
type RateLimit struct {
	Max    int
	Window time.Duration

	mu    sync.Mutex
	state map[string]*rateState
}

type rateState struct {
	start    time.Time
	hits     int
	reported bool
}

// Hit counts one occurrence; it returns true, and the count, the first time
// the limit is passed in the current window.
func (r *RateLimit) Hit(key string, now time.Time) (bool, int) {
	if r.Max <= 0 || r.Window <= 0 {
		return false, 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.state == nil {
		r.state = map[string]*rateState{}
	}
	s := r.state[key]
	if s == nil || now.Sub(s.start) >= r.Window {
		if len(r.state) > 4096 { // bound the state: forget old windows
			for k, v := range r.state {
				if now.Sub(v.start) >= r.Window {
					delete(r.state, k)
				}
			}
		}
		s = &rateState{start: now}
		r.state[key] = s
	}
	s.hits++
	if s.hits > r.Max && !s.reported {
		s.reported = true
		return true, s.hits
	}
	return false, s.hits
}

// MACCache caches LookupMAC results for a short time.
type MACCache struct {
	mu sync.Mutex
	m  map[netip.Addr]macEntry
}

type macEntry struct {
	mac string
	at  time.Time
}

const macCacheTTL = 30 * time.Second

// Lookup is LookupMAC through the cache.
func (c *MACCache) Lookup(ip netip.Addr) string {
	if !ip.IsValid() {
		return ""
	}
	now := time.Now()
	c.mu.Lock()
	if e, ok := c.m[ip]; ok && now.Sub(e.at) < macCacheTTL {
		c.mu.Unlock()
		return e.mac
	}
	c.mu.Unlock()
	mac := LookupMAC(ip)
	c.mu.Lock()
	if c.m == nil || len(c.m) > 4096 {
		c.m = map[netip.Addr]macEntry{}
	}
	c.m[ip] = macEntry{mac, now}
	c.mu.Unlock()
	return mac
}

// LookupMAC returns the MAC address the OS neighbour table holds for ip,
// or "" when unknown: a peer behind a router shows the router's MAC, and a
// peer the host has not talked to recently may not be listed. It reads
// /proc/net/arp on Linux (IPv4), and runs "arp -a" on Windows and macOS.
func LookupMAC(ip netip.Addr) string {
	if !ip.IsValid() || ip.IsLoopback() {
		return ""
	}
	ip = ip.Unmap()
	switch runtime.GOOS {
	case "linux":
		return lookupProcARP(ip, "/proc/net/arp")
	case "windows", "darwin", "freebsd":
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "arp", "-a", ip.String()).Output()
		if err != nil {
			return ""
		}
		return parseARPOutput(ip, string(out))
	}
	return ""
}

func lookupProcARP(ip netip.Addr, path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		// IP address  HW type  Flags  HW address  Mask  Device
		fs := strings.Fields(sc.Text())
		if len(fs) < 4 || fs[0] != ip.String() {
			continue
		}
		if fs[2] == "0x0" { // incomplete entry
			return ""
		}
		return normalizeMAC(fs[3])
	}
	return ""
}

var macRE = regexp.MustCompile(`(?i)\b([0-9a-f]{1,2}[:-]){5}[0-9a-f]{1,2}\b`)

// parseARPOutput finds the MAC on the line of "arp -a" that names ip.
func parseARPOutput(ip netip.Addr, out string) string {
	want := ip.String()
	for _, line := range strings.Split(out, "\n") {
		hit := false
		for _, f := range strings.FieldsFunc(line, func(r rune) bool { return r == ' ' || r == '(' || r == ')' || r == '\t' }) {
			if f == want {
				hit = true
				break
			}
		}
		if !hit {
			continue
		}
		if m := macRE.FindString(line); m != "" {
			return normalizeMAC(m)
		}
	}
	return ""
}

// normalizeMAC writes a MAC as lower-case, colon separated, two digits per
// octet; "" for the all-zero address or an unparsable one.
func normalizeMAC(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == ':' || r == '-' })
	if len(parts) != 6 {
		return ""
	}
	for i, p := range parts {
		if len(p) == 1 {
			parts[i] = "0" + p
		}
	}
	hw, err := net.ParseMAC(strings.Join(parts, ":"))
	if err != nil || hw.String() == "00:00:00:00:00:00" {
		return ""
	}
	return hw.String()
}

// SplitAddr returns the IP of an "ip:port" address ("" when it has none).
func SplitAddr(addr string) netip.Addr {
	if ap, err := netip.ParseAddrPort(addr); err == nil {
		return ap.Addr().Unmap()
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		addr = host
	}
	a, _ := netip.ParseAddr(strings.Trim(addr, "[]"))
	return a.Unmap()
}
