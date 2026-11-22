package serverapp

import (
	"testing"

	"dnp3-go/internal/dnp3sec"

	"github.com/riclolsen/json-scada/src/go-common/jssecevents"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/outstation"
)

// TestRefusedControlsAreReported: a control the handler refuses is answered
// as before and reported with the reason, once per period for one object.
func TestRefusedControlsAreReported(t *testing.T) {
	var got []jssecevents.Event
	rep := dnp3sec.NewReporter(nil, jssecevents.Origin{}, "server", true)
	rep.Capture = func(e jssecevents.Event) { got = append(got, e) }
	conn := &Connection{Name: "SRV", ProtocolConnectionNumber: 3}
	conn.Sec = dnp3sec.NewServer(rep, nil, 1, nil)

	h := &commandHandler{conn: conn, disabled: true}
	crob := dnp3.ControlRelayOutputBlock{Code: dnp3.ControlLatchOn, Count: 1}
	if st := h.SelectCROB(4, crob); st != dnp3.CommandNotSupported {
		t.Errorf("SelectCROB with commands disabled = %v, want NOT_SUPPORTED", st)
	}
	if st := h.OperateCROB(4, crob, outstation.OperateDirect); st != dnp3.CommandNotSupported {
		t.Errorf("OperateCROB with commands disabled = %v, want NOT_SUPPORTED", st)
	}
	if len(got) != 1 {
		t.Fatalf("stored %d events, want 1 (the same refusal of one object once per period)", len(got))
	}
	e := got[0]
	if e.Kind != "cmd.rejected" || e.FC != "SELECT" || e.ObjectRef != "g12 #4" ||
		e.Detail != "commands are disabled on the connection" || e.Fields["commandStatus"] != "NOT_SUPPORTED" {
		t.Errorf("refusal = %+v", e)
	}

	// Without a database the control is refused, not acknowledged.
	h = &commandHandler{conn: conn}
	if st := h.OperateAnalog(2, outstation.AnalogOutput{Value: 1.5}, outstation.OperateDirect); st != dnp3.CommandDownstreamFail {
		t.Errorf("OperateAnalog without a database = %v, want DOWNSTREAM_FAIL", st)
	}
	if e := got[len(got)-1]; e.ObjectRef != "g41 #2" || e.FC != "DIRECT_OPERATE" || e.Value != "1.5" {
		t.Errorf("analog refusal = %+v", e)
	}
}
