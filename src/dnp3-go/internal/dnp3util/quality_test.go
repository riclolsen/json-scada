package dnp3util

import (
	"testing"

	dnp3 "github.com/dscsystems/go-dnp3"
)

// TestQualityFields checks the sourceDataUpdate booleans against the
// expressions in processMongo() of the C++ client.
func TestQualityFields(t *testing.T) {
	cases := []struct {
		name string
		q    Quality
		// expected sourceDataUpdate fields
		notTopical, invalid, overflow, blocked, substituted, carry bool
	}{
		{
			name:    "healthy",
			q:       Quality{Online: true},
			invalid: false,
		},
		{
			name:       "offline",
			q:          Quality{Online: false},
			invalid:    true, // !online
			blocked:    true,
			notTopical: false,
		},
		{
			name:       "comm lost",
			q:          Quality{Online: true, CommLost: true},
			notTopical: true,
			invalid:    true,
		},
		{
			name:    "reference error",
			q:       Quality{Online: true, ReferenceError: true},
			invalid: true,
		},
		{
			name:        "remote forced",
			q:           Quality{Online: true, RemoteForced: true},
			substituted: true,
		},
		{
			name:        "local forced",
			q:           Quality{Online: true, LocalForced: true},
			substituted: true,
		},
		{
			name:     "overrange",
			q:        Quality{Online: true, Overrange: true},
			overflow: true,
		},
		{
			name:  "rollover",
			q:     Quality{Online: true, Rollover: true},
			carry: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			check := func(what string, got, want bool) {
				if got != want {
					t.Errorf("%s = %v, want %v", what, got, want)
				}
			}
			check("notTopicalAtSource", c.q.NotTopical(), c.notTopical)
			check("invalidAtSource", c.q.Invalid(), c.invalid)
			check("overflowAtSource", c.q.Overflow(), c.overflow)
			check("blockedAtSource", c.q.Blocked(), c.blocked)
			check("substitutedAtSource", c.q.Substituted(), c.substituted)
			check("carryAtSource", c.q.Carry(), c.carry)
		})
	}
}

// TestQualityDecoding checks that the type-specific upper bits are read for the
// right measurement types.
func TestQualityDecoding(t *testing.T) {
	// OverRange and Rollover are the same bit (0x20) with different meanings.
	flags := dnp3.Online | dnp3.OverRange

	if q := AnalogQuality(flags); !q.Overrange || q.Rollover {
		t.Errorf("analog: over-range not decoded, got %+v", q)
	}
	if q := CounterQuality(flags); !q.Rollover || q.Overrange {
		t.Errorf("counter: rollover not decoded, got %+v", q)
	}
	if q := CommonQuality(flags); q.Overrange || q.Rollover {
		t.Errorf("binary: upper bits must not be decoded, got %+v", q)
	}

	all := dnp3.Online | dnp3.CommLost | dnp3.RemoteForced | dnp3.LocalForced
	q := CommonQuality(all)
	if !q.Online || !q.CommLost || !q.RemoteForced || !q.LocalForced {
		t.Errorf("common bits not decoded, got %+v", q)
	}
}

// TestTagQualityFlags checks the server-side direction: a tag's quality into a
// DNP3 flags octet, as ConvertValue() builds it.
func TestTagQualityFlags(t *testing.T) {
	cases := []struct {
		name            string
		q               TagQuality
		countsTransient bool
		overflowBit     dnp3.Flags
		want            dnp3.Flags
	}{
		{
			name: "healthy",
			q:    TagQuality{},
			want: dnp3.Online,
		},
		{
			name: "invalid",
			q:    TagQuality{Invalid: true},
			want: dnp3.CommLost,
		},
		{
			name:            "transient counts for a binary",
			q:               TagQuality{Transient: true},
			countsTransient: true,
			want:            dnp3.CommLost,
		},
		{
			name:            "transient does not count for a double bit",
			q:               TagQuality{Transient: true},
			countsTransient: false,
			want:            dnp3.Online,
		},
		{
			name: "substituted",
			q:    TagQuality{Substituted: true},
			want: dnp3.Online | dnp3.LocalForced,
		},
		{
			name:        "overflow on an analog",
			q:           TagQuality{Overflow: true},
			overflowBit: dnp3.OverRange,
			want:        dnp3.Online | dnp3.OverRange,
		},
		{
			name:        "overflow on a counter",
			q:           TagQuality{Overflow: true},
			overflowBit: dnp3.Rollover,
			want:        dnp3.Online | dnp3.Rollover,
		},
		{
			name:        "overflow ignored where the family has no bit",
			q:           TagQuality{Overflow: true},
			overflowBit: 0,
			want:        dnp3.Online,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.q.Flags(c.countsTransient, c.overflowBit); got != c.want {
				t.Errorf("Flags = %s (%#x), want %s (%#x)", got, uint8(got), c.want, uint8(c.want))
			}
		})
	}
}
