/*
 * IEC 60870-5-101/104 protocol drivers for {json:scada} - loopback test
 * {json:scada} - Copyright (c) 2020 - 2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * Protocol-level loopback test: a go-iecp5 cs104 server and client exchange
 * monitor data (through conv.BuildInfoObj/SendInfoBatch on the server and
 * conv.Decode on the client) and a control command. This validates the
 * encode/decode paths that the drivers depend on, without requiring MongoDB.
 */

package test

import (
	"sync"
	"testing"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/go-iecp5/cs104"

	"iec60870-5/internal/conv"
)

// serverHandler answers interrogation with a fixed process image and records
// received commands.
type serverHandler struct {
	mu       sync.Mutex
	gotCmd   bool
	cmdValue bool
}

func (h *serverHandler) InterrogationHandler(c asdu.Connect, pack *asdu.ASDU, qoi asdu.QualifierOfInterrogation) error {
	_ = pack.SendReplyMirror(c, asdu.ActivationCon)
	coa := asdu.CauseOfTransmission{Cause: asdu.InterrogatedByStation}
	// single point true, and a float measurement, built via conv
	sp := conv.BuildInfoObj(1, 100, 1, false, 0, conv.Quality{}, nil, 1, 0, false)
	_ = conv.SendInfoBatch(c, coa, 1, sp.TypeID, []*conv.InfoObject{sp})
	mv := conv.BuildInfoObj(13, 400, 22.5, false, 0, conv.Quality{}, nil, 1, 0, false)
	_ = conv.SendInfoBatch(c, coa, 1, mv.TypeID, []*conv.InfoObject{mv})
	return pack.SendReplyMirror(c, asdu.ActivationTerm)
}
func (h *serverHandler) CounterInterrogationHandler(c asdu.Connect, p *asdu.ASDU, q asdu.QualifierCountCall) error {
	return p.SendReplyMirror(c, asdu.ActivationCon)
}
func (h *serverHandler) ReadHandler(c asdu.Connect, p *asdu.ASDU, ioa asdu.InfoObjAddr) error {
	return nil
}
func (h *serverHandler) ClockSyncHandler(c asdu.Connect, p *asdu.ASDU, t time.Time) error {
	return p.SendReplyMirror(c, asdu.ActivationCon)
}
func (h *serverHandler) ResetProcessHandler(c asdu.Connect, p *asdu.ASDU, q asdu.QualifierOfResetProcessCmd) error {
	return nil
}
func (h *serverHandler) DelayAcquisitionHandler(c asdu.Connect, p *asdu.ASDU, m uint16) error {
	return nil
}
func (h *serverHandler) ASDUHandler(c asdu.Connect, pack *asdu.ASDU) error {
	if pack.Type == asdu.C_SC_NA_1 || pack.Type == asdu.C_SC_TA_1 {
		cmd := pack.GetSingleCmd()
		h.mu.Lock()
		h.gotCmd = true
		h.cmdValue = cmd.Value
		h.mu.Unlock()
		_ = pack.SendReplyMirror(c, asdu.ActivationCon)
		return pack.SendReplyMirror(c, asdu.ActivationTerm)
	}
	return nil
}
func (h *serverHandler) ASDUHandlerAll(c asdu.Connect, p *asdu.ASDU, n int) error { return nil }

// clientHandler collects decoded values via conv.Decode.
type clientHandler struct {
	mu     sync.Mutex
	values map[int]conv.IecValue
	acks   []conv.IecCmdAck
}

func (h *clientHandler) InterrogationHandler(asdu.Connect, *asdu.ASDU) error        { return nil }
func (h *clientHandler) CounterInterrogationHandler(asdu.Connect, *asdu.ASDU) error { return nil }
func (h *clientHandler) ReadHandler(asdu.Connect, *asdu.ASDU) error                 { return nil }
func (h *clientHandler) TestCommandHandler(asdu.Connect, *asdu.ASDU) error          { return nil }
func (h *clientHandler) ClockSyncHandler(asdu.Connect, *asdu.ASDU) error            { return nil }
func (h *clientHandler) ResetProcessHandler(asdu.Connect, *asdu.ASDU) error         { return nil }
func (h *clientHandler) DelayAcquisitionHandler(asdu.Connect, *asdu.ASDU) error     { return nil }
func (h *clientHandler) ASDUHandler(c asdu.Connect, pack *asdu.ASDU, _ *cs104.Server, _ int) error {
	res := conv.Decode(pack, 1)
	h.mu.Lock()
	for _, v := range res.Values {
		h.values[v.Address] = v
	}
	h.acks = append(h.acks, res.Acks...)
	h.mu.Unlock()
	return nil
}
func (h *clientHandler) ASDUHandlerAll(asdu.Connect, *asdu.ASDU, *cs104.Server, int) error {
	return nil
}

func TestLoopback104(t *testing.T) {
	addr := "127.0.0.1:24040"
	sh := &serverHandler{}
	srv := cs104.NewServer(sh)
	go func() { _ = srv.ListenAndServer(addr) }()
	defer srv.Close()
	time.Sleep(300 * time.Millisecond)

	ch := &clientHandler{values: map[int]conv.IecValue{}}
	opt := cs104.NewOption()
	if err := opt.AddRemoteServer("tcp://" + addr); err != nil {
		t.Fatal(err)
	}
	cli := cs104.NewClient(ch, opt)
	activated := make(chan struct{}, 1)
	cli.SetOnConnectHandler(func(c *cs104.Client) { c.SendStartDt() })
	cli.SetOnActivatedHandler(func(c *cs104.Client) {
		select {
		case activated <- struct{}{}:
		default:
		}
	})
	if err := cli.Start(); err != nil {
		t.Fatal(err)
	}
	defer cli.Close()

	select {
	case <-activated:
	case <-time.After(5 * time.Second):
		t.Fatal("client did not activate")
	}

	// interrogation
	if err := cli.InterrogationCmd(asdu.CauseOfTransmission{Cause: asdu.Activation}, 1, asdu.QOIStation); err != nil {
		t.Fatal(err)
	}

	// wait for data
	deadline := time.After(5 * time.Second)
	for {
		ch.mu.Lock()
		n := len(ch.values)
		ch.mu.Unlock()
		if n >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for interrogation data, got %d values", n)
		case <-time.After(50 * time.Millisecond):
		}
	}

	ch.mu.Lock()
	sp, okSP := ch.values[100]
	mv, okMV := ch.values[400]
	ch.mu.Unlock()
	if !okSP || sp.Value != 1 || !sp.IsDigital {
		t.Errorf("single point mismatch: %+v", sp)
	}
	if !okMV || mv.Value < 22.4 || mv.Value > 22.6 {
		t.Errorf("measured value mismatch: %+v", mv)
	}

	// send a single command through conv and verify the server receives it
	cmd := conv.BuildInfoObj(45, 6000, 1, false, 0, conv.Quality{}, nil, 1, 0, false)
	if err := conv.SendCommand(cli, asdu.CauseOfTransmission{Cause: asdu.Activation}, 1, cmd); err != nil {
		t.Fatal(err)
	}
	deadline = time.After(5 * time.Second)
	for {
		sh.mu.Lock()
		got := sh.gotCmd
		val := sh.cmdValue
		sh.mu.Unlock()
		if got {
			if !val {
				t.Errorf("command value mismatch: got %v want true", val)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("server did not receive command")
		case <-time.After(50 * time.Millisecond):
		}
	}

	// command confirmation should have produced an ack on the client
	time.Sleep(300 * time.Millisecond)
	ch.mu.Lock()
	nacks := len(ch.acks)
	ch.mu.Unlock()
	if nacks == 0 {
		t.Errorf("expected at least one command ack, got none")
	}
}
