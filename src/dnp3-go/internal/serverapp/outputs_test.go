package serverapp

import (
	"context"
	"testing"
	"time"

	dnp3 "github.com/dscsystems/go-dnp3"
	"github.com/dscsystems/go-dnp3/channel"
	"github.com/dscsystems/go-dnp3/master"
	"github.com/dscsystems/go-dnp3/outstation"

	"dnp3-go/internal/dnp3util"
)

// TestCommandsReachTheHandlerWhateverTheirIndex checks that the library does
// not refuse a command before the driver's handler has looked for its tag.
//
// Controls and output statuses are separate address spaces in the
// configuration: a connection may publish four binary output statuses and have
// control points far beyond them. go-dnp3 v0.8.0 refuses a control whose index
// is past the database's output status count — NOT_SUPPORTED, never reaching
// the handler — unless the handler implements outstation.OutputPoints. The
// handler is the one that knows which command tags exist, so it has to be asked.
//
// The engine here has no database, so a command that reaches the handler is
// answered DOWNSTREAM_FAIL; one the library refused is answered NOT_SUPPORTED.
func TestCommandsReachTheHandlerWhateverTheirIndex(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn := &Connection{
		ProtocolConnectionNumber: 1, Name: "SRV", CommandsEnabled: true,
		LocalLinkAddress: 10, RemoteLinkAddress: 1, ServerQueueSize: 100,
	}
	// Four binary and two analog output statuses.
	var tags []map[string]any
	for i := 0; i < 4; i++ {
		tags = append(tags, tag(float64(i+1), "BOS", true, 10, i, 2, nil))
	}
	for i := 0; i < 2; i++ {
		tags = append(tags, tag(float64(i+10), "AOS", 1.5, 40, i, 3, nil))
	}

	e := &Engine{byNum: map[int]*Connection{1: conn}}
	station := e.newOutstation(conn, toBsonSlice(tags))
	conn.setStation(station)

	mch, och := channel.Pipe()
	go func() { _ = station.Run(ctx, och) }()
	m := master.New(master.Config{
		LocalAddr: 1, RemoteAddr: 10, ResponseTimeout: 3 * time.Second,
	}, master.NopHandler{})
	go func() { _ = m.Run(ctx, mch) }()
	waitFor(t, "the master to connect", m.Connected)

	rctx, rcancel := context.WithTimeout(ctx, 10*time.Second)
	defer rcancel()

	status := func(cmd master.Command) dnp3.CommandStatus {
		t.Helper()
		res, _ := m.DirectOperate(rctx, cmd)
		if len(res.Statuses) == 0 {
			t.Fatalf("no command status for %v", cmd)
		}
		return res.Statuses[0]
	}

	for _, index := range []uint16{1, 3, 4, 10, 500} {
		if got := status(master.CROB(index, dnp3util.CROBFor(3, 1))); got != dnp3.CommandDownstreamFail {
			t.Errorf("CROB at index %d answered %v, want the handler's DOWNSTREAM_FAIL", index, got)
		}
	}
	for _, index := range []uint16{0, 1, 2, 10, 500} {
		if got := status(master.AnalogOutputFloat32(index, 1.5)); got != dnp3.CommandDownstreamFail {
			t.Errorf("analog output at index %d answered %v, want the handler's DOWNSTREAM_FAIL", index, got)
		}
	}
}

// TestCommandHandlerSaysItHasEveryOutput pins the interface itself, so a
// library that stops asking for it fails to compile here rather than silently
// going back to refusing commands by index.
func TestCommandHandlerSaysItHasEveryOutput(t *testing.T) {
	var h outstation.CommandHandler = &commandHandler{}
	op, ok := h.(outstation.OutputPoints)
	if !ok {
		t.Fatal("commandHandler does not implement outstation.OutputPoints")
	}
	for _, pt := range []dnp3.PointType{dnp3.TypeBinaryOutputStatus, dnp3.TypeAnalogOutputStatus} {
		if !op.HasOutput(pt, 60000) {
			t.Errorf("HasOutput(%v, 60000) = false: the handler decides which outputs exist", pt)
		}
	}
}
