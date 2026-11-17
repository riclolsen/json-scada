package main

import (
	"net"
	"testing"
	"time"

	"github.com/riclolsen/tase2/tase2"
)

func TestGetICCPType(t *testing.T) {
	cases := []struct {
		name string
		tag  rtData
		want tase2.ICCPType
	}{
		{"digital", rtData{Type: "digital"}, tase2.ICCPTypeStateQTimeTagExtended},
		{"analog float", rtData{Type: "analog"}, tase2.ICCPTypeRealQTimeTagExtended},
		{"analog float32 asdu", rtData{Type: "analog", ProtocolSourceASDU: "float32"}, tase2.ICCPTypeRealQTimeTagExtended},
		{"analog int32", rtData{Type: "analog", ProtocolSourceASDU: "int32"}, tase2.ICCPTypeDiscreteQTimeTagExtended},
		{"analog uint16", rtData{Type: "analog", ProtocolSourceASDU: "uint16"}, tase2.ICCPTypeDiscreteQTimeTagExtended},
		{"analog int64", rtData{Type: "analog", ProtocolSourceASDU: "int64"}, tase2.ICCPTypeDiscreteQTimeTagExtended},
		{"string", rtData{Type: "string"}, tase2.ICCPTypeUnknown},
		{"json", rtData{Type: "json"}, tase2.ICCPTypeUnknown},
	}
	for _, c := range cases {
		if got := getICCPType(c.tag); got != c.want {
			t.Errorf("%s: getICCPType = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestConvertToICCPValueRoundTrip(t *testing.T) {
	// Source time with a sub-second part: the millisecond must survive, the
	// microseconds are truncated.
	ts := time.Date(2026, 7, 5, 11, 30, 0, 123456789, time.UTC)
	wantTS := time.Date(2026, 7, 5, 11, 30, 0, 123000000, time.UTC)

	// Analog with source timestamp and invalid quality
	tag := rtData{Type: "analog", Value: 123.25, Invalid: true, TimeTagAtSource: &ts, TimeTagAtSourceOk: true}
	dp := tase2.DecodeICCP(convertToICCPValue(tag, getICCPType(tag)))
	if dp.Type != tase2.ICCPTypeRealQTimeTagExtended {
		t.Fatalf("decoded type = %v, want RealQTimeTagExtended", dp.Type)
	}
	if dp.Real == nil || *dp.Real < 123.2 || *dp.Real > 123.3 {
		t.Errorf("decoded real = %v, want ~123.25", dp.Real)
	}
	if dp.Quality == nil || dp.Quality.Validity != tase2.QualityInvalid {
		t.Errorf("decoded quality = %+v, want invalid", dp.Quality)
	}
	if got, ok := dp.Time(); !ok || !got.Equal(wantTS) {
		t.Errorf("decoded time = %v (ok=%v), want %v", got, ok, wantTS)
	}

	// Digital on, stamped with the source time to the millisecond
	tag = rtData{Type: "digital", Value: 1, TimeTagAtSource: &ts, TimeTagAtSourceOk: true}
	dp = tase2.DecodeICCP(convertToICCPValue(tag, getICCPType(tag)))
	if dp.Type != tase2.ICCPTypeStateQTimeTagExtended {
		t.Fatalf("digital: decoded type = %v, want StateQTimeTagExtended", dp.Type)
	}
	if dp.State == nil || *dp.State != tase2.StateOn {
		t.Errorf("digital 1: decoded state = %v, want StateOn", dp.State)
	}
	if dp.Milliseconds == nil || *dp.Milliseconds != 123 {
		t.Errorf("digital: decoded milliseconds = %v, want 123", dp.Milliseconds)
	}

	// No source time: stamped with the current time (GMT based, not a
	// time-of-day value).
	tag = rtData{Type: "analog", Value: 1234567, ProtocolSourceASDU: "int32"}
	before := time.Now().Truncate(time.Millisecond)
	dp = tase2.DecodeICCP(convertToICCPValue(tag, getICCPType(tag)))
	if dp.Type != tase2.ICCPTypeDiscreteQTimeTagExtended {
		t.Fatalf("int32 analog: decoded type = %v, want DiscreteQTimeTagExtended", dp.Type)
	}
	if dp.Discrete == nil || *dp.Discrete != 1234567 {
		t.Errorf("int32 analog: decoded discrete = %v, want 1234567 (exact)", dp.Discrete)
	}
	if got, ok := dp.Time(); !ok || got.Before(before) || got.After(time.Now()) {
		t.Errorf("int32 analog: decoded time = %v (ok=%v), want between %v and now", got, ok, before)
	}

	// Legacy second-resolution types now carry GMTBasedS seconds.
	dp = tase2.DecodeICCP(convertToICCPValue(rtData{Type: "analog", Value: 1, TimeTagAtSource: &ts, TimeTagAtSourceOk: true}, tase2.ICCPTypeRealQTimeTag))
	if dp.TimeTag == nil || *dp.TimeTag != ts.Unix() {
		t.Errorf("RealQTimeTag: decoded timetag = %v, want %d", dp.TimeTag, ts.Unix())
	}
}

func TestApplyCommandConversion(t *testing.T) {
	cases := []struct {
		name   string
		value  float64
		doc    rtData
		want   float64
		wantSt string
		origSt string
	}{
		{"digital direct on", 1, rtData{Type: "digital", Kconv1: 1.0}, 1, "true", "true"},
		{"digital direct off", 0, rtData{Type: "digital", Kconv1: 1.0}, 0, "false", "false"},
		{"digital direct nonzero 0.6", 0.6, rtData{Type: "digital"}, 1, "1", "0.600000"},
		{"digital direct nonzero 0.4", 0.4, rtData{Type: "digital"}, 1, "1", "0.400000"},
		{"digital direct non-boolean", 3, rtData{Type: "digital"}, 1, "1", "3"},
		{"digital inverted on", 1, rtData{Type: "digital", Kconv1: -1.0}, 0, "false", "true"},
		{"digital inverted off", 0, rtData{Type: "digital", Kconv1: -1.0}, 1, "true", "false"},
		{"digital inverted int kconv", 1, rtData{Type: "digital", Kconv1: int32(-1)}, 0, "0", "1"},
		{"analog scaled", 10, rtData{Type: "analog", Kconv1: 2.0, Kconv2: 5.0}, 25, "25", "10.000000"},
		{"analog defaults", 12.5, rtData{Type: "analog"}, 12.5, "12.500000", "12.500000"},
		{"analog offset only", 3, rtData{Type: "analog", Kconv2: 1.5}, 4.5, "4.5", "3"},
		{"string type untouched", 7, rtData{Type: "string", Kconv1: -1.0}, 7, "7", "7"},
	}

	for _, c := range cases {
		got := applyCommandConversion(c.value, c.doc)
		if got != c.want {
			t.Errorf("%s: applyCommandConversion(%v) = %v, want %v", c.name, c.value, got, c.want)
		}
		st := c.origSt
		if got != c.value {
			st = commandValueString(got, c.origSt)
		}
		if st != c.wantSt {
			t.Errorf("%s: valueString = %q, want %q", c.name, st, c.wantSt)
		}
	}
}

func TestSanitizePointName(t *testing.T) {
	cases := map[string]string{
		"KAW2AL-21XCBR5217----K": "KAW2AL_21XCBR5217____K",
		"9starts_with_digit":     "X9starts_with_digit",
		"":                       "X_unnamed_",
	}
	for in, want := range cases {
		if got := sanitizePointName(in); got != want {
			t.Errorf("sanitizePointName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseLocalBind(t *testing.T) {
	cases := []struct {
		in   string
		host string
		port int
		bad  bool
	}{
		{"", "0.0.0.0", 102, false},
		{":10102", "0.0.0.0", 10102, false},
		{"0.0.0.0:102", "0.0.0.0", 102, false},
		{"192.168.1.10:2102", "192.168.1.10", 2102, false},
		{"127.0.0.1", "127.0.0.1", 102, false},
		{"[::1]:102", "::1", 102, false},
		{"::", "::", 102, false},
		{" 10.0.0.5:102 ", "10.0.0.5", 102, false},
		{"0.0.0.0:abc", "", 0, true},
		{"0.0.0.0:70000", "", 0, true},
	}
	for _, c := range cases {
		host, port, err := parseLocalBind(c.in)
		if c.bad {
			if err == nil {
				t.Errorf("parseLocalBind(%q) = %q, %d; want error", c.in, host, port)
			}
			continue
		}
		if err != nil || host != c.host || port != c.port {
			t.Errorf("parseLocalBind(%q) = %q, %d, %v; want %q, %d", c.in, host, port, err, c.host, c.port)
		}
	}
}

// The library binds passive endpoints to loopback by default; the driver's
// default must still listen on all interfaces.
func TestListenAllInterfacesByDefault(t *testing.T) {
	host, _, err := parseLocalBind("")
	if err != nil {
		t.Fatal(err)
	}
	ep := tase2.NewEndpoint(tase2.EndpointPassive)
	if err := ep.SetListenAddress(host); err != nil {
		t.Fatalf("SetListenAddress(%q): %v", host, err)
	}
	if err := ep.Listen(0); err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ep.Disconnect()
	addr, ok := ep.ListenAddr().(*net.TCPAddr)
	if !ok || !addr.IP.IsUnspecified() {
		t.Errorf("listening on %v, want the unspecified (all interfaces) address", ep.ListenAddr())
	}

	// A host name other than localhost is rejected rather than resolved.
	if err := tase2.NewEndpoint(tase2.EndpointPassive).SetListenAddress("scada-server"); err == nil {
		t.Error("SetListenAddress(host name) succeeded, want error")
	}
}

func TestBuildModel(t *testing.T) {
	conns := []protocolConnection{
		{ProtocolConnectionNumber: 50, CommandsEnabled: true, Topics: []string{"Sub Station 1"}},
	}
	tags := []rtData{
		// group1 with spaces is sanitized into a valid domain name
		{ID: 1, Tag: "B-POINT", Type: "analog", Group1: "Sub Station 1"},
		{ID: 2, Tag: "A-POINT", Type: "digital", Group1: "Sub Station 1"},
		// collides with A-POINT after sanitizing: not exposed
		{ID: 3, Tag: "A_POINT", Type: "analog", Group1: "Sub Station 1"},
		// command tag in a commandable domain: control point, even if blocked
		{ID: 4, Tag: "CMD-1", Type: "digital", Origin: "command", Group1: "Sub Station 1", CommandBlocked: true},
		// command tag no connection may operate: not exposed at all
		{ID: 5, Tag: "CMD-2", Type: "digital", Origin: "command", Group1: "OTHER"},
		// acquired by this instance's own connection: not served back
		{ID: 6, Tag: "OWN", Type: "analog", Group1: "OTHER", ProtocolSourceConnectionNumber: 50},
		// not a standard ICCP type
		{ID: 7, Tag: "TEXT", Type: "string", Group1: "OTHER"},
	}
	m := buildModel(tags, conns, 0)

	if got := m.dataModel.GetDomains(); len(got) != 1 || got[0] != "Sub_Station_1" {
		t.Errorf("domains = %v, want [Sub_Station_1]", got)
	}
	if len(m.ipByTag) != 2 || m.ipByTag["A-POINT"] == nil || m.ipByTag["B-POINT"] == nil {
		t.Errorf("indication points = %v, want A-POINT and B-POINT", m.ipByTag)
	}
	if m.ipByTag["A_POINT"] != nil || m.tagByKey["Sub_Station_1/A_POINT"] != "A-POINT" {
		t.Errorf("collision: A_POINT exposed or key owner %q, want A-POINT (first by name)", m.tagByKey["Sub_Station_1/A_POINT"])
	}
	if ref := m.cpRefByTag["CMD-1"]; ref.Domain != "Sub_Station_1" || ref.Item != "CMD_1" {
		t.Errorf("control point = %+v, want Sub_Station_1/CMD_1", ref)
	}
	if _, ok := m.cpRefByTag["CMD-2"]; ok {
		t.Error("CMD-2 exposed, want hidden (no connection may operate it)")
	}
	if len(m.datasetDefs) != 1 {
		t.Fatalf("datasets = %+v, want 1", m.datasetDefs)
	}
	ds := m.datasetDefs[0]
	if ds.domain != "Sub_Station_1" || ds.name != "Sub_Station_1/Sub_Station_1_DataSet" ||
		len(ds.items) != 2 || ds.items[0].Item != "A_POINT" || ds.items[1].Item != "B_POINT" {
		t.Errorf("dataset = %+v, want sorted A_POINT, B_POINT", ds)
	}
}

func TestTopicAllowsDomain(t *testing.T) {
	if !topicAllowsDomain(nil, "ANY") || !topicAllowsDomain([]string{" "}, "ANY") {
		t.Error("no topics must allow every domain")
	}
	if !topicAllowsDomain([]string{"Sub Station 1"}, "Sub_Station_1") {
		t.Error("topic must match its sanitized domain name")
	}
	if topicAllowsDomain([]string{"SUB_A"}, "SUB_B") {
		t.Error("SUB_B allowed by topic SUB_A")
	}
}

func TestTopicsOf(t *testing.T) {
	got := topicsOf([]protocolConnection{{Topics: []string{"B", "A"}}, {Topics: []string{"A"}}})
	if len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Errorf("topicsOf = %v, want [A B]", got)
	}
	if got := topicsOf([]protocolConnection{{Topics: []string{"A"}}, {}}); got != nil {
		t.Errorf("topicsOf with an unrestricted connection = %v, want nil (no filter)", got)
	}
}

func TestTagQuality(t *testing.T) {
	cases := []struct {
		tag              rtData
		validity, source string
	}{
		{rtData{}, tase2.QualityValid, tase2.SourceTelemetered},
		{rtData{Invalid: true, Transient: true}, tase2.QualityInvalid, tase2.SourceTelemetered},
		{rtData{Transient: true, Frozen: true}, tase2.QualitySuspect, tase2.SourceTelemetered},
		{rtData{Frozen: true}, tase2.QualityHeld, tase2.SourceTelemetered},
		{rtData{Substituted: true}, tase2.QualityValid, tase2.SourceEntered},
		{rtData{Origin: "manual"}, tase2.QualityValid, tase2.SourceEntered},
		{rtData{Origin: "calculated"}, tase2.QualityValid, tase2.SourceCalculated},
	}
	for i, c := range cases {
		q := tagQuality(c.tag)
		if q.Validity != c.validity || q.Source != c.source {
			t.Errorf("case %d: quality = %s/%s, want %s/%s", i, q.Validity, q.Source, c.validity, c.source)
		}
	}
}

func TestCommandTarget(t *testing.T) {
	tags := []rtData{
		{ID: 1, Tag: "CMD-OK", Type: "digital", Origin: "command", Group1: "SUB_A"},
		{ID: 2, Tag: "CMD-BLOCKED", Type: "digital", Origin: "command", Group1: "SUB_A", CommandBlocked: true},
	}
	conn := protocolConnection{Name: "C", ProtocolConnectionNumber: 60, CommandsEnabled: true, Topics: []string{"SUB_A"}}
	m := buildModel(tags, []protocolConnection{conn}, 0)
	populateRtDataCache(tags, m.tagByKey)

	isActive.Store(true)
	defer isActive.Store(false)

	if tag, ok := commandTarget(conn, "Operate", "SUB_A", "CMD_OK"); !ok || tag != "CMD-OK" {
		t.Errorf("CMD_OK = %q, %v; want CMD-OK allowed", tag, ok)
	}
	if _, ok := commandTarget(conn, "Operate", "SUB_A", "CMD_BLOCKED"); ok {
		t.Error("blocked command allowed")
	}
	if _, ok := commandTarget(protocolConnection{Name: "D"}, "Operate", "SUB_A", "CMD_OK"); ok {
		t.Error("command allowed on a connection with commands disabled")
	}
	if _, ok := commandTarget(protocolConnection{Name: "E", CommandsEnabled: true, Topics: []string{"SUB_B"}},
		"Operate", "SUB_A", "CMD_OK"); ok {
		t.Error("command allowed outside the connection topics")
	}

	// commandBlocked set at runtime (change stream) is honoured.
	blocked := tags[0]
	blocked.CommandBlocked = true
	updateRtDataCache(blocked)
	if _, ok := commandTarget(conn, "Operate", "SUB_A", "CMD_OK"); ok {
		t.Error("command blocked at runtime still allowed")
	}

	isActive.Store(false)
	if _, ok := commandTarget(conn, "Operate", "SUB_A", "CMD_BLOCKED"); ok {
		t.Error("command allowed on an inactive node")
	}
}
