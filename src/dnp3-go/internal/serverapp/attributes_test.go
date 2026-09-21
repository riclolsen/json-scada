package serverapp

import (
	"context"
	"testing"
	"time"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/channel"
	"github.com/dscsystems/go-dnp3/master"
)

// TestDeviceAttributes checks the identity a connection reports.
func TestDeviceAttributes(t *testing.T) {
	conn := &Connection{
		ProtocolConnectionNumber: 34,
		Name:                     "DNP3SRV",
		Description:              "KAW2 substation gateway",
	}

	byVariation := map[uint8]dnp3.Attribute{}
	for _, a := range deviceAttributes(conn) {
		if a.Set != dnp3.AttrSetStandard {
			t.Errorf("attribute %d is in set %d, want the standard set", a.Variation, a.Set)
		}
		if _, dup := byVariation[a.Variation]; dup {
			t.Errorf("attribute %d reported twice", a.Variation)
		}
		byVariation[a.Variation] = a
	}

	want := map[uint8]string{
		attrManufacturer:    manufacturerName,
		attrProductName:     productName,
		attrSoftwareVersion: DriverVersion,
		attrDeviceName:      "DNP3SRV",
		attrLocation:        "KAW2 substation gateway",
		attrIDCode:          "34",
	}
	for variation, value := range want {
		a, ok := byVariation[variation]
		if !ok {
			t.Errorf("attribute %d (%s) not reported", variation, nameOf(variation))
			continue
		}
		if a.Value() != value {
			t.Errorf("attribute %d (%s) = %q, want %q", variation, nameOf(variation), a.Value(), value)
		}
	}

	if a, ok := byVariation[attrHardwareVersion]; !ok || a.Value() == "" {
		t.Error("the host platform should be reported as the hardware version")
	}

	// Two attributes are deliberately not answered: a conformance level nobody
	// has certified, and a serial number a gateway does not have. A plausible
	// wrong value propagates further than a missing one.
	for _, variation := range []uint8{248, 249} {
		if a, reported := byVariation[variation]; reported {
			t.Errorf("attribute %d must not be reported, got %q", variation, a.Value())
		}
	}
}

// TestDeviceAttributesOptionalFields checks that a connection with no
// description does not report an empty location: saying nothing and saying
// "" are different answers.
func TestDeviceAttributesOptionalFields(t *testing.T) {
	conn := &Connection{ProtocolConnectionNumber: 1, Name: "SRV"}
	for _, a := range deviceAttributes(conn) {
		if a.Variation == attrLocation {
			t.Errorf("location reported as %q for a connection with no description", a.Value())
		}
		if a.Value() == "" {
			t.Errorf("attribute %d reported with an empty value", a.Variation)
		}
	}
}

// TestDeviceAttributesOverTheWire reads the attributes back through a real
// master, which is the only way to know the outstation actually serves them.
//
// It also covers what the library derives rather than what this driver
// configures: the point counts a master reads must match the database it is
// about to poll.
func TestDeviceAttributesOverTheWire(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connection number 1, because the tag fixtures are written for it: the
	// destinations have to match or the database sizes to nothing.
	conn := &Connection{
		ProtocolConnectionNumber: 1,
		Name:                     "DNP3SRV",
		Description:              "KAW2 substation gateway",
		LocalLinkAddress:         10,
		RemoteLinkAddress:        1,
		ServerQueueSize:          100,
	}
	// Three binaries and one analog, so the derived counts have something to
	// be wrong about.
	tags := []map[string]any{
		tag(1, "B0", true, 1, 0, 2, nil),
		tag(2, "B1", true, 1, 1, 2, nil),
		tag(3, "B2", true, 1, 2, 2, nil),
		tag(4, "A0", 1.5, 30, 0, 5, nil),
	}

	e := &Engine{byNum: map[int]*Connection{1: conn}}
	station := e.newOutstation(conn, toBsonSlice(tags))
	conn.setStation(station)

	mch, och := channel.Pipe()
	go func() { _ = station.Run(ctx, och) }()

	m := master.New(master.Config{
		LocalAddr: 1, RemoteAddr: 10,
		ResponseTimeout: 5 * time.Second,
	}, master.NopHandler{})
	go func() { _ = m.Run(ctx, mch) }()

	waitFor(t, "the master to connect", m.Connected)

	readCtx, readCancel := context.WithTimeout(ctx, 10*time.Second)
	defer readCancel()
	attrs, err := m.ReadAttributes(readCtx)
	if err != nil {
		t.Fatalf("ReadAttributes: %v", err)
	}
	if len(attrs) == 0 {
		t.Fatal("the outstation reported no device attributes")
	}

	got := map[uint8]string{}
	for _, a := range attrs {
		got[a.Variation] = a.Value()
		t.Logf("  %3d %-40s %s", a.Variation, a.Name(), a.Value())
	}

	// What this driver configures.
	for variation, want := range map[uint8]string{
		attrManufacturer:    manufacturerName,
		attrProductName:     productName,
		attrSoftwareVersion: DriverVersion,
		attrDeviceName:      "DNP3SRV",
		attrLocation:        "KAW2 substation gateway",
		attrIDCode:          "1",
	} {
		if got[variation] != want {
			t.Errorf("attribute %d over the wire = %q, want %q", variation, got[variation], want)
		}
	}

	// What the library derives, which must agree with the database that was
	// actually built: three binary inputs and one analog input.
	if got[226] != "3" {
		t.Errorf("binary input count = %q, want 3", got[226])
	}
	if got[220] != "1" {
		t.Errorf("analog input count = %q, want 1", got[220])
	}
	// A point type this connection does not have is left unreported rather
	// than reported as zero.
	if v, reported := got[216]; reported {
		t.Errorf("counter count reported as %q for a connection with no counters", v)
	}
}

// nameOf labels a variation for a failure message.
func nameOf(variation uint8) string {
	if n, ok := dnp3.AttributeName(variation); ok {
		return n
	}
	return "?"
}

// TestCommandPassesCarryOutputStatus pins the readback mapping the auto-create
// pass applies: a CROB command is mirrored by a binary output status and an
// analog output block by an analog output status, at the same object address.
//
// The address is not a choice. DNP3 ties them together — a CROB at index N
// operates binary output N, whose state is group 10 index N — so a pass that
// assigned the status its own address would describe a different point.
func TestCommandPassesCarryOutputStatus(t *testing.T) {
	conn := &Connection{CommandsEnabled: true}
	passes := autoCreatePasses(conn)

	byGroup := map[int]autoCreatePass{}
	for _, p := range passes {
		byGroup[p.group] = p
	}

	crob, ok := byGroup[12]
	if !ok {
		t.Fatal("no CROB pass")
	}
	if crob.statusGroup != 10 {
		t.Errorf("CROB status group = %d, want 10 (binary output status)", crob.statusGroup)
	}
	if crob.statusASDU != 2 {
		t.Errorf("CROB status ASDU = %v, want 2 (g10v2)", crob.statusASDU)
	}

	analog, ok := byGroup[41]
	if !ok {
		t.Fatal("no analog output pass")
	}
	if analog.statusGroup != 40 {
		t.Errorf("analog command status group = %d, want 40 (analog output status)", analog.statusGroup)
	}
	if analog.statusASDU != 3 {
		t.Errorf("analog command status ASDU = %v, want 3 (g40v3)", analog.statusASDU)
	}

	// A supervised pass has no status companion: it is not a command, and
	// giving it one would publish the same point twice.
	for _, group := range []int{1, 30} {
		if p, ok := byGroup[group]; ok && p.statusGroup != 0 {
			t.Errorf("supervised pass for group %d must have no status group, got %d",
				group, p.statusGroup)
		}
	}

	// With commands disabled there are no command passes at all, so no status
	// destinations either.
	for _, p := range autoCreatePasses(&Connection{CommandsEnabled: false}) {
		if p.origin == "command" {
			t.Errorf("group %d command pass present with commands disabled", p.group)
		}
	}
}
