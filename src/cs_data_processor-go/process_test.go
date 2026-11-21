package main

import (
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func newTestProcessor(t *testing.T) *Processor {
	t.Helper()
	cfg := Config{
		Workers:         1,
		ChangeQueueSize: 64,
		WriteQueueSize:  64,
		Instance:        1,
	}
	processActive.Store(true)
	return NewProcessor(cfg)
}

func mkChange(op string, fullDocument bson.D, sourceDataUpdate bson.D) changeEvent {
	ev := bson.D{
		{Key: "operationType", Value: op},
		{Key: "fullDocument", Value: fullDocument},
	}
	if sourceDataUpdate != nil {
		ev = append(ev, bson.E{Key: "updateDescription", Value: bson.D{
			{Key: "updatedFields", Value: bson.D{
				{Key: "sourceDataUpdate", Value: sourceDataUpdate},
			}},
		}})
	}
	raw, err := bson.Marshal(ev)
	if err != nil {
		panic(err)
	}
	return changeEvent{raw: bson.Raw(raw), recvAt: time.Now()}
}

func drainRt(p *Processor) []*rtUpdate {
	var out []*rtUpdate
	for {
		select {
		case u := <-p.rtCh:
			out = append(out, u)
		default:
			return out
		}
	}
}

func drainSoe(p *Processor) []bson.D {
	var out []bson.D
	for {
		select {
		case d := <-p.soeCh:
			out = append(out, d)
		default:
			return out
		}
	}
}

func drainHist(p *Processor) []histEntry {
	var out []histEntry
	for {
		select {
		case e := <-p.histCh:
			out = append(out, e)
		default:
			return out
		}
	}
}

func drainSQL(p *Processor) []string {
	var out []string
	for {
		select {
		case s := <-p.sqlRtCh:
			out = append(out, s)
		default:
			return out
		}
	}
}

func setField(d bson.D, key string) (any, bool) {
	for _, e := range d {
		if e.Key == key {
			return e.Value, true
		}
	}
	return nil, false
}

func baseDigital() bson.D {
	return bson.D{
		{Key: "_id", Value: 101.0},
		{Key: "tag", Value: "KAW2~CB~Status"},
		{Key: "type", Value: "digital"},
		{Key: "value", Value: 0.0},
		{Key: "valueString", Value: "OPEN"},
		{Key: "valueJson", Value: ""},
		{Key: "alarmed", Value: false},
		{Key: "alarmDisabled", Value: false},
		{Key: "alarmRange", Value: 0.0},
		{Key: "isEvent", Value: false},
		{Key: "invalid", Value: false},
		{Key: "kconv1", Value: 1.0},
		{Key: "kconv2", Value: 0.0},
		{Key: "unit", Value: ""},
		{Key: "stateTextTrue", Value: "CLOSED"},
		{Key: "stateTextFalse", Value: "OPEN"},
		{Key: "eventTextTrue", Value: "CLOSED"},
		{Key: "eventTextFalse", Value: "OPEN"},
		{Key: "group1", Value: "KAW2"},
		{Key: "description", Value: "KAW2~CB~Status"},
		{Key: "priority", Value: 1.0},
		{Key: "historianPeriod", Value: 0.0},
		{Key: "updatesCnt", Value: 7.0},
		{Key: "timeTag", Value: time.Now().Add(-time.Minute)},
	}
}

func TestDigitalChangeProducesUpdateSoeAndHist(t *testing.T) {
	p := newTestProcessor(t)
	now := time.Now()
	sdu := bson.D{
		{Key: "valueAtSource", Value: 1.0},
		{Key: "valueStringAtSource", Value: ""},
		{Key: "asduAtSource", Value: "M_SP_TB_1"},
		{Key: "causeOfTransmissionAtSource", Value: "3"},
		{Key: "timeTag", Value: now},
		{Key: "timeTagAtSource", Value: now.Add(-50 * time.Millisecond)},
		{Key: "timeTagAtSourceOk", Value: true},
		{Key: "invalidAtSource", Value: false},
	}
	p.handleChange(mkChange("update", baseDigital(), sdu))

	ups := drainRt(p)
	if len(ups) != 3 {
		t.Fatalf("expected the beep, counter and point updates, got %d", len(ups))
	}
	// the counter point is queued first, the point update last
	pointUpd := ups[len(ups)-1]
	if v, _ := setField(pointUpd.set, "value"); v != 1.0 {
		t.Errorf("value = %v, want 1", v)
	}
	if v, _ := setField(pointUpd.set, "valueString"); v != "CLOSED" {
		t.Errorf("valueString = %v, want CLOSED", v)
	}
	if v, _ := setField(pointUpd.set, "alarmed"); v != true {
		t.Errorf("alarmed = %v, want true", v)
	}
	if v, _ := setField(pointUpd.set, "updatesCnt"); v != 8.0 {
		t.Errorf("updatesCnt = %v, want 8", v)
	}
	if _, ok := setField(pointUpd.set, "timeTagAlarm"); !ok {
		t.Error("timeTagAlarm missing on a new alarm")
	}

	soe := drainSoe(p)
	if len(soe) != 1 {
		t.Fatalf("expected 1 SOE entry, got %d", len(soe))
	}
	if v, _ := setField(soe[0], "eventText"); v != "CLOSED" {
		t.Errorf("eventText = %v", v)
	}
	if v, _ := setField(soe[0], "ack"); v != 0 {
		t.Errorf("ack = %v, want 0", v)
	}

	hist := drainHist(p)
	if len(hist) != 1 {
		t.Fatalf("expected 1 hist entry, got %d", len(hist))
	}
	if !strings.HasPrefix(hist[0].sql, "'KAW2~CB~Status',") {
		t.Errorf("hist sql = %s", hist[0].sql)
	}
	// flags: valid value, valid source time, digital, not integrity
	if !strings.HasSuffix(hist[0].sql, ",B'00000000'") {
		t.Errorf("hist flags wrong: %s", hist[0].sql)
	}

	sql := drainSQL(p)
	if len(sql) != 1 {
		t.Fatalf("expected 1 realtime SQL row, got %d", len(sql))
	}
	if !strings.Contains(sql[0], `\"valueString\":\"CLOSED\"`) &&
		!strings.Contains(sql[0], `"valueString":"CLOSED"`) {
		t.Errorf("realtime SQL row does not carry the new value: %s", sql[0])
	}
}

func TestDigitalNoChangeIsSkipped(t *testing.T) {
	p := newTestProcessor(t)
	fd := baseDigital()
	sdu := bson.D{
		{Key: "valueAtSource", Value: 0.0},
		{Key: "timeTag", Value: time.Now()},
	}
	p.handleChange(mkChange("update", fd, sdu))
	if ups := drainRt(p); len(ups) != 0 {
		t.Fatalf("expected no update for an unchanged value, got %d", len(ups))
	}
	if sql := drainSQL(p); len(sql) != 0 {
		t.Fatalf("expected no SQL row, got %d", len(sql))
	}
}

func TestDoublePointTransientIsInvalidated(t *testing.T) {
	p := newTestProcessor(t)
	fd := baseDigital()
	sdu := bson.D{
		{Key: "valueAtSource", Value: 0.0}, // indeterminate double point state
		{Key: "asduAtSource", Value: "M_DP_TB_1"},
		{Key: "timeTag", Value: time.Now()},
	}
	p.handleChange(mkChange("update", fd, sdu))
	ups := drainRt(p)
	if len(ups) == 0 {
		t.Fatal("expected an update")
	}
	u := ups[len(ups)-1]
	if v, _ := setField(u.set, "invalid"); v != true {
		t.Errorf("invalid = %v, want true", v)
	}
	if v, _ := setField(u.set, "transient"); v != true {
		t.Errorf("transient = %v, want true", v)
	}
	// value 0 with M_DP_ maps to 1
	if v, _ := setField(u.set, "value"); v != 1.0 {
		t.Errorf("value = %v, want 1", v)
	}
	if v, _ := setField(u.set, "valueString"); !strings.Contains(v.(string), "[IV]") {
		t.Errorf("valueString = %v, want the [IV] qualifier", v)
	}
}

func baseAnalog() bson.D {
	return bson.D{
		{Key: "_id", Value: 202.0},
		{Key: "tag", Value: "KAW2~TR1~MW"},
		{Key: "type", Value: "analog"},
		{Key: "value", Value: 10.0},
		{Key: "valueString", Value: "10 MW"},
		{Key: "valueJson", Value: ""},
		{Key: "alarmed", Value: false},
		{Key: "alarmDisabled", Value: false},
		{Key: "alarmRange", Value: 0.0},
		{Key: "isEvent", Value: false},
		{Key: "invalid", Value: false},
		{Key: "kconv1", Value: 2.0},
		{Key: "kconv2", Value: 1.0},
		{Key: "unit", Value: "MW"},
		{Key: "hiLimit", Value: 50.0},
		{Key: "loLimit", Value: -50.0},
		{Key: "hysteresis", Value: 1.0},
		{Key: "group1", Value: "KAW2"},
		{Key: "description", Value: "KAW2~TR1~MW"},
		{Key: "priority", Value: 2.0},
		{Key: "historianPeriod", Value: 0.0},
		{Key: "historianDeadBand", Value: 0.0},
		{Key: "updatesCnt", Value: 3.0},
		{Key: "timeTag", Value: time.Now().Add(-time.Minute)},
	}
}

func TestAnalogConversionAndHighLimitAlarm(t *testing.T) {
	p := newTestProcessor(t)
	sdu := bson.D{
		{Key: "valueAtSource", Value: 30.0}, // 30*2 + 1 = 61 > 50 + 1
		{Key: "timeTag", Value: time.Now()},
	}
	p.handleChange(mkChange("update", baseAnalog(), sdu))

	ups := drainRt(p)
	if len(ups) != 1 {
		t.Fatalf("expected 1 update, got %d", len(ups))
	}
	if v, _ := setField(ups[0].set, "value"); v != 61.0 {
		t.Errorf("value = %v, want 61", v)
	}
	if v, _ := setField(ups[0].set, "valueString"); v != "61 MW" {
		t.Errorf("valueString = %q, want %q", v, "61 MW")
	}
	if v, _ := setField(ups[0].set, "alarmRange"); v != 1.0 {
		t.Errorf("alarmRange = %v, want 1", v)
	}
	if v, _ := setField(ups[0].set, "alarmed"); v != true {
		t.Errorf("alarmed = %v, want true", v)
	}
	if v, _ := setField(ups[0].set, "historianLastValue"); v != 61.0 {
		t.Errorf("historianLastValue = %v, want 61", v)
	}

	soe := drainSoe(p)
	if len(soe) != 1 {
		t.Fatalf("expected the limit alarm SOE, got %d", len(soe))
	}
	txt, _ := setField(soe[0], "eventText")
	if !strings.HasPrefix(txt.(string), "61 MW") || !strings.Contains(txt.(string), "\U0001F6A9") {
		t.Errorf("eventText = %q", txt)
	}
}

// An unacknowledged HI alarm that goes to HIHI is a new alarm: it beeps
// again, gets a new alarm time and a HIHI event.
func TestAnalogLimitEscalation(t *testing.T) {
	p := newTestProcessor(t)
	fd := baseAnalog()
	for i := range fd {
		switch fd[i].Key {
		case "alarmed":
			fd[i].Value = true
		case "alarmRange":
			fd[i].Value = 1.0
		case "priority":
			fd[i].Value = 1.0
		case "value":
			fd[i].Value = 52.0
		}
	}
	fd = append(fd, bson.E{Key: "hihiLimit", Value: 55.0}, bson.E{Key: "hihihiLimit", Value: nil})
	sdu := bson.D{
		{Key: "valueAtSource", Value: 30.0}, // 30*2 + 1 = 61 > 55 + 1
		{Key: "timeTag", Value: time.Now()},
	}
	p.handleChange(mkChange("update", fd, sdu))

	ups := drainRt(p)
	var point, beep *rtUpdate
	for _, u := range ups {
		if v, _ := setField(u.set, "valueString"); v == "Beep Active" {
			beep = u
		} else if _, ok := setField(u.set, "alarmRange"); ok {
			point = u
		}
	}
	if point == nil {
		t.Fatalf("no point update in %d updates", len(ups))
	}
	if v, _ := setField(point.set, "alarmRange"); v != 2.0 {
		t.Errorf("alarmRange = %v, want 2", v)
	}
	if v, _ := setField(point.set, "alarmed"); v != true {
		t.Errorf("alarmed = %v, want true", v)
	}
	if _, ok := setField(point.set, "timeTagAlarm"); !ok {
		t.Error("timeTagAlarm not set on escalation")
	}
	if beep == nil {
		t.Error("no beep on escalation")
	}
	soe := drainSoe(p)
	if len(soe) != 1 {
		t.Fatalf("expected the limit alarm SOE, got %d", len(soe))
	}
	if txt, _ := setField(soe[0], "eventText"); txt != "61 MW \u2909 \U0001F6A9 HIHI" {
		t.Errorf("eventText = %q", txt)
	}

	// back between hiLimit and hihiLimit (out of the band): HI, no new beep
	fd2 := baseAnalog()
	for i := range fd2 {
		switch fd2[i].Key {
		case "alarmed":
			fd2[i].Value = true
		case "alarmRange":
			fd2[i].Value = 2.0
		case "priority":
			fd2[i].Value = 1.0
		case "value":
			fd2[i].Value = 61.0
		}
	}
	fd2 = append(fd2, bson.E{Key: "hihiLimit", Value: 55.0})
	sdu2 := bson.D{
		{Key: "valueAtSource", Value: 26.0}, // 26*2 + 1 = 53 < 55 - 1
		{Key: "timeTag", Value: time.Now()},
	}
	p.handleChange(mkChange("update", fd2, sdu2))
	for _, u := range drainRt(p) {
		if v, _ := setField(u.set, "valueString"); v == "Beep Active" {
			t.Error("beep on de-escalation")
		} else if v, ok := setField(u.set, "alarmRange"); ok && v != 1.0 {
			t.Errorf("alarmRange = %v, want 1", v)
		}
	}
	soe = drainSoe(p)
	if len(soe) != 1 {
		t.Fatalf("expected the limit SOE, got %d", len(soe))
	}
	if txt, _ := setField(soe[0], "eventText"); txt != "53 MW \u2908 \U0001F6A9 HI" {
		t.Errorf("eventText = %q", txt)
	}
}

func TestAnalogQualifierAndDeadBand(t *testing.T) {
	p := newTestProcessor(t)
	fd := baseAnalog()
	for i := range fd {
		if fd[i].Key == "historianDeadBand" {
			fd[i].Value = 100.0
		}
	}
	fd = append(fd, bson.E{Key: "historianLastValue", Value: 21.0})
	sdu := bson.D{
		{Key: "valueAtSource", Value: 10.0}, // 10*2+1 = 21
		{Key: "invalidAtSource", Value: true},
		{Key: "timeTag", Value: time.Now()},
	}
	p.handleChange(mkChange("update", fd, sdu))

	ups := drainRt(p)
	if len(ups) != 1 {
		t.Fatalf("expected 1 update, got %d", len(ups))
	}
	if v, _ := setField(ups[0].set, "valueString"); v != "21 MW [IV]" {
		t.Errorf("valueString = %q, want %q", v, "21 MW [IV]")
	}
	if _, ok := setField(ups[0].set, "historianLastValue"); ok {
		t.Error("historianLastValue must not be updated when the dead band filters the sample")
	}
	if h := drainHist(p); len(h) != 0 {
		t.Errorf("dead band should have suppressed the historical sample, got %d", len(h))
	}
}

func TestBitstringValueString(t *testing.T) {
	p := newTestProcessor(t)
	fd := baseAnalog()
	// identity conversion for the bitstring
	for i := range fd {
		if fd[i].Key == "kconv1" {
			fd[i].Value = 1.0
		}
		if fd[i].Key == "kconv2" {
			fd[i].Value = 0.0
		}
		if fd[i].Key == "unit" {
			fd[i].Value = "Bits"
		}
	}
	sdu := bson.D{
		{Key: "valueAtSource", Value: 5.0},
		{Key: "asduAtSource", Value: "M_BO_NA_1"},
		{Key: "timeTag", Value: time.Now()},
	}
	p.handleChange(mkChange("update", fd, sdu))
	ups := drainRt(p)
	if len(ups) != 1 {
		t.Fatalf("expected 1 update, got %d", len(ups))
	}
	if v, _ := setField(ups[0].set, "valueString"); v != "101 Bits" {
		t.Errorf("valueString = %q, want %q", v, "101 Bits")
	}
}

func TestInactiveNodeIgnoresChanges(t *testing.T) {
	p := newTestProcessor(t)
	processActive.Store(false)
	defer processActive.Store(true)
	sdu := bson.D{
		{Key: "valueAtSource", Value: 1.0},
		{Key: "timeTag", Value: time.Now()},
	}
	p.handleChange(mkChange("update", baseDigital(), sdu))
	if ups := drainRt(p); len(ups) != 0 {
		t.Fatalf("a standby node must not write, got %d updates", len(ups))
	}
}

func TestHistoricalBackfillDoesNotUpdateRealtime(t *testing.T) {
	p := newTestProcessor(t)
	sdu := bson.D{
		{Key: "valueAtSource", Value: 1.0},
		{Key: "isHistorical", Value: true},
		{Key: "timeTag", Value: time.Now()},
		{Key: "timeTagAtSource", Value: time.Now().Add(-time.Hour)},
		{Key: "timeTagAtSourceOk", Value: true},
	}
	p.handleChange(mkChange("update", baseDigital(), sdu))
	if ups := drainRt(p); len(ups) != 0 {
		t.Fatalf("historical backfill must not touch realtimeData, got %d", len(ups))
	}
	// but it still produces the SOE entry
	if soe := drainSoe(p); len(soe) != 1 {
		t.Fatalf("expected the SOE entry, got %d", len(soe))
	}
}

func TestBeepOnPriorityZeroAlarm(t *testing.T) {
	p := newTestProcessor(t)
	fd := baseDigital()
	for i := range fd {
		if fd[i].Key == "priority" {
			fd[i].Value = 0.0
		}
	}
	sdu := bson.D{
		{Key: "valueAtSource", Value: 1.0},
		{Key: "timeTag", Value: time.Now()},
	}
	p.handleChange(mkChange("update", fd, sdu))
	ups := drainRt(p)
	if len(ups) != 3 {
		t.Fatalf("expected beep, counter and point updates, got %d", len(ups))
	}
	beep := ups[0]
	if v, _ := setField(beep.set, "beepType"); v != 2.0 {
		t.Errorf("beepType = %v, want 2", v)
	}
	if v, _ := setField(beep.set, "valueString"); v != "Beep Active" {
		t.Errorf("beep valueString = %v", v)
	}
	if len(beep.addToSet) != 1 || beep.addToSet[0].Key != "beepGroup1List" {
		t.Errorf("beepGroup1List not added: %v", beep.addToSet)
	}
}

func TestInsertProducesRealtimeSQLOnly(t *testing.T) {
	p := newTestProcessor(t)
	p.handleChange(mkChange("insert", baseDigital(), nil))
	if ups := drainRt(p); len(ups) != 0 {
		t.Fatalf("insert must not update, got %d", len(ups))
	}
	sql := drainSQL(p)
	if len(sql) != 1 {
		t.Fatalf("expected 1 SQL row, got %d", len(sql))
	}
	if !strings.Contains(sql[0], "to_json(") {
		t.Errorf("insert SQL row should wrap the document in to_json: %s", sql[0])
	}
}
