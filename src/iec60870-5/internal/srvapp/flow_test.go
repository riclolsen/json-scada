/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package srvapp

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"
	"github.com/riclolsen/go-iecp5/cs104"

	"iec60870-5/internal/conv"
	"iec60870-5/internal/model"
)

func TestFlowWaitsOnDrainingLink(t *testing.T) {
	e := &Engine{}
	l := &fakeLink{}
	out := e.reply(l)
	single := func() error {
		return asdu.Single(out, false, asdu.CauseOfTransmission{Cause: asdu.Spontaneous}, 1,
			asdu.SinglePointInfo{Ioa: 1, Value: true})
	}
	if err := single(); err != nil { // link accepted something: draining
		t.Fatal(err)
	}
	l.mu.Lock()
	l.fullFor = l.calls + 50 // the next 50 attempts find the buffer full
	l.mu.Unlock()
	if err := single(); err != nil {
		t.Fatalf("send on a draining link not retried: %v", err)
	}
	if n := len(l.all()); n != 2 {
		t.Errorf("sent %d ASDUs, want 2", n)
	}
}

// A link that never accepted anything (a 104 session without STARTDT) is
// not waited on.
func TestFlowDoesNotWaitOnStalledLink(t *testing.T) {
	e := &Engine{}
	l := &fakeLink{fullFor: 1 << 30}
	start := time.Now()
	err := asdu.Single(e.reply(l), false, asdu.CauseOfTransmission{Cause: asdu.Spontaneous}, 1,
		asdu.SinglePointInfo{Ioa: 1, Value: true})
	if !errors.Is(err, cs104.ErrBufferFulled) {
		t.Fatalf("err = %v, want buffer full", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("waited %v on a stalled link", d)
	}
}

func TestOutQueueBounded(t *testing.T) {
	c := NewConn(model.ConnCfg{Name: "Q", MaxQueueSize: 10})
	for i := 0; i < 25; i++ {
		c.EnqueueInfo(conv.BuildInfoObj(1, i, 1, false, 0, conv.Quality{}, nil, 1, 0, false), 1)
	}
	b := c.outQ.popBatch(100)
	if len(b) != 10 || b[0].obj.Ioa != 15 {
		t.Fatalf("queue kept %d objects starting at IOA %d, want 10 from 15", len(b), b[0].obj.Ioa)
	}
	if c.dropped.Load() != 15 {
		t.Errorf("dropped = %d, want 15", c.dropped.Load())
	}
	if NewConn(model.ConnCfg{}).outQ.limit != DefaultMaxQueueSize {
		t.Error("default queue limit not applied")
	}
}

// bigGIHandler answers interrogation with many points through the engine's
// flow control, as Engine.Interrogation does.
type bigGIHandler struct {
	e      *Engine
	points int
	errs   int
}

func (h *bigGIHandler) InterrogationHandler(c asdu.Connect, pack *asdu.ASDU, _ asdu.QualifierOfInterrogation) error {
	out := h.e.reply(c)
	_ = pack.SendReplyMirror(out, asdu.ActivationCon)
	coa := asdu.CauseOfTransmission{Cause: asdu.InterrogatedByStation}
	var batch []*conv.InfoObject
	for i := 1; i <= h.points; i++ {
		batch = append(batch, conv.BuildInfoObj(13, i, float64(i), false, 0, conv.Quality{}, nil, 1, 0, false))
		if len(batch) == 30 || i == h.points {
			if err := conv.SendInfoBatch(out, coa, 1, asdu.M_ME_NC_1, batch); err != nil {
				h.errs++
			}
			batch = nil
		}
	}
	return pack.SendReplyMirror(out, asdu.ActivationTerm)
}
func (h *bigGIHandler) CounterInterrogationHandler(asdu.Connect, *asdu.ASDU, asdu.QualifierCountCall) error {
	return nil
}
func (h *bigGIHandler) ReadHandler(asdu.Connect, *asdu.ASDU, asdu.InfoObjAddr) error { return nil }
func (h *bigGIHandler) ClockSyncHandler(asdu.Connect, *asdu.ASDU, time.Time) error   { return nil }
func (h *bigGIHandler) ResetProcessHandler(asdu.Connect, *asdu.ASDU, asdu.QualifierOfResetProcessCmd) error {
	return nil
}
func (h *bigGIHandler) DelayAcquisitionHandler(asdu.Connect, *asdu.ASDU, uint16) error { return nil }
func (h *bigGIHandler) ASDUHandler(asdu.Connect, *asdu.ASDU) error                     { return nil }
func (h *bigGIHandler) ASDUHandlerAll(asdu.Connect, *asdu.ASDU, int) error             { return nil }

type countingClient struct {
	mu   sync.Mutex
	objs map[int]bool
	term chan struct{}
}

func (h *countingClient) InterrogationHandler(_ asdu.Connect, pack *asdu.ASDU) error {
	if pack.Coa.Cause == asdu.ActivationTerm {
		close(h.term)
	}
	return nil
}
func (h *countingClient) CounterInterrogationHandler(asdu.Connect, *asdu.ASDU) error { return nil }
func (h *countingClient) ReadHandler(asdu.Connect, *asdu.ASDU) error                 { return nil }
func (h *countingClient) TestCommandHandler(asdu.Connect, *asdu.ASDU) error          { return nil }
func (h *countingClient) ClockSyncHandler(asdu.Connect, *asdu.ASDU) error            { return nil }
func (h *countingClient) ResetProcessHandler(asdu.Connect, *asdu.ASDU) error         { return nil }
func (h *countingClient) DelayAcquisitionHandler(asdu.Connect, *asdu.ASDU) error     { return nil }
func (h *countingClient) ASDUHandler(_ asdu.Connect, pack *asdu.ASDU, _ *cs104.Server, _ int) error {
	h.mu.Lock()
	for _, v := range conv.Decode(pack, 1).Values {
		h.objs[v.Address] = true
	}
	h.mu.Unlock()
	return nil
}
func (h *countingClient) ASDUHandlerAll(asdu.Connect, *asdu.ASDU, *cs104.Server, int) error {
	return nil
}

// A station interrogation larger than the session send buffer (k*16 ASDUs)
// arrives complete, ActTerm included, over a real 104 link.
func TestBigInterrogationLoopback(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()

	sh := &bigGIHandler{e: &Engine{}, points: 18000}
	srv := cs104.NewServer(sh)
	go func() { _ = srv.ListenAndServer(addr) }()
	defer srv.Close()
	time.Sleep(300 * time.Millisecond)

	ch := &countingClient{objs: map[int]bool{}, term: make(chan struct{})}
	opt := cs104.NewOption()
	if err := opt.AddRemoteServer("tcp://" + addr); err != nil {
		t.Fatal(err)
	}
	cli := cs104.NewClient(ch, opt)
	activated := make(chan struct{}, 1)
	cli.SetOnConnectHandler(func(c *cs104.Client) { c.SendStartDt() })
	cli.SetOnActivatedHandler(func(*cs104.Client) {
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
	if err := cli.InterrogationCmd(asdu.CauseOfTransmission{Cause: asdu.Activation}, 1, asdu.QOIStation); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch.term:
	case <-time.After(30 * time.Second):
		t.Fatal("no ActTerm")
	}
	ch.mu.Lock()
	n := len(ch.objs)
	ch.mu.Unlock()
	if n != sh.points || sh.errs != 0 {
		t.Errorf("received %d of %d points, %d send errors", n, sh.points, sh.errs)
	}
}
