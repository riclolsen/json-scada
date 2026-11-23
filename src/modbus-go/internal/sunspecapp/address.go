/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package sunspecapp

// Point addresses (protocolSourceObjectAddress):
//
//	<modelId>[#<occurrence>]/[<group>[<index>]/...]<point>
//
// e.g. "103/W", "160/module[2]/DCW", "160#2/module[1]/DCA", "705/curve[1]/pt[3]/V".
// The occurrence counts the instances of the same model id in the device's
// chain from 1 and is left out for the first; a group index counts the
// instances of the group within its parent from 1, as go-sunspec does.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/otfabric/go-sunspec"
)

// GroupRef is one group step of an address.
type GroupRef struct {
	Name  string
	Index int
}

// Address is a parsed point address.
type Address struct {
	ModelID    uint16
	Occurrence int // 1-based
	Groups     []GroupRef
	Point      string
}

// String renders the canonical form.
func (a Address) String() string {
	var sb strings.Builder
	sb.WriteString(strconv.Itoa(int(a.ModelID)))
	if a.Occurrence > 1 {
		sb.WriteString("#" + strconv.Itoa(a.Occurrence))
	}
	for _, g := range a.Groups {
		fmt.Fprintf(&sb, "/%s[%d]", g.Name, g.Index)
	}
	sb.WriteString("/" + a.Point)
	return sb.String()
}

// TopLevel reports a point of the model's top-level group, the only ones
// go-sunspec writes.
func (a Address) TopLevel() bool { return len(a.Groups) == 0 }

// ParseAddress parses a point address.
func ParseAddress(s string) (Address, error) {
	s = strings.TrimSpace(s)
	parts := strings.Split(s, "/")
	if len(parts) < 2 {
		return Address{}, fmt.Errorf("invalid SunSpec address %q: expected <model>/<point>", s)
	}
	var a Address
	model := parts[0]
	a.Occurrence = 1
	if i := strings.IndexByte(model, '#'); i >= 0 {
		n, err := strconv.Atoi(model[i+1:])
		if err != nil || n < 1 {
			return Address{}, fmt.Errorf("invalid model occurrence in %q", s)
		}
		a.Occurrence, model = n, model[:i]
	}
	id, err := strconv.Atoi(model)
	if err != nil || id < 1 || id >= 65535 {
		return Address{}, fmt.Errorf("invalid model id in %q", s)
	}
	a.ModelID = uint16(id)
	for _, g := range parts[1 : len(parts)-1] {
		open := strings.IndexByte(g, '[')
		if open < 1 || !strings.HasSuffix(g, "]") {
			return Address{}, fmt.Errorf("invalid group %q in %q: expected <name>[<index>]", g, s)
		}
		n, err := strconv.Atoi(g[open+1 : len(g)-1])
		if err != nil || n < 1 {
			return Address{}, fmt.Errorf("invalid group index %q in %q", g, s)
		}
		a.Groups = append(a.Groups, GroupRef{Name: g[:open], Index: n})
	}
	a.Point = parts[len(parts)-1]
	if a.Point == "" {
		return Address{}, fmt.Errorf("missing point name in %q", s)
	}
	return a, nil
}

// instanceOf finds the model instance an address names in a discovery.
func instanceOf(d *sunspec.DiscoveryResult, a Address) (sunspec.ModelInstance, bool) {
	if d == nil {
		return sunspec.ModelInstance{}, false
	}
	n := 0
	for _, m := range d.Models {
		if m.Header.ID != a.ModelID {
			continue
		}
		n++
		if n == a.Occurrence {
			return m, true
		}
	}
	return sunspec.ModelInstance{}, false
}

// occurrences numbers the instances of each model id of a discovery: the
// result is parallel to d.Models.
func occurrences(d *sunspec.DiscoveryResult) []int {
	seen := map[uint16]int{}
	out := make([]int, len(d.Models))
	for i, m := range d.Models {
		seen[m.Header.ID]++
		out[i] = seen[m.Header.ID]
	}
	return out
}

// pointOf finds the decoded point an address names.
func pointOf(m *sunspec.DecodedModel, a Address) *sunspec.DecodedPoint {
	if m == nil || m.Group == nil {
		return nil
	}
	g := m.Group
	for _, ref := range a.Groups {
		var next *sunspec.DecodedGroup
		for _, c := range g.Groups {
			if c.Name == ref.Name && c.Index == ref.Index {
				next = c
				break
			}
		}
		if next == nil {
			return nil
		}
		g = next
	}
	return g.Point(a.Point)
}
