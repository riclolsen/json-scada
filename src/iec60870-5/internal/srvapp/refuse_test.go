package srvapp

import (
	"testing"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"

	"iec60870-5/internal/model"
)

// A refusal that does not need the database: nothing is queued, and the
// master gets one negative confirmation.
func TestCommandRefusedBeforeDatabase(t *testing.T) {
	off := false
	cases := []struct {
		name string
		cfg  model.ConnCfg
		pack func() *asdu.ASDU
	}{
		{"commands disabled", model.ConnCfg{CommandsEnabled: &off},
			func() *asdu.ASDU { return singleCmd(t, 500, false, asdu.C_SC_NA_1, time.Time{}) }},
		{"test bit", model.ConnCfg{},
			func() *asdu.ASDU {
				p := singleCmd(t, 500, false, asdu.C_SC_NA_1, time.Time{})
				p.Coa.IsTest = true
				return p
			}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := &Engine{}
			e.IsMongoLive.Store(true) // the refusal must come before any query
			srv := NewConn(c.cfg)
			l := &fakeLink{}
			if !e.HandleCommandASDU(srv, l, c.pack()) {
				t.Fatal("command type not handled")
			}
			if got := confirmations(l); len(got) != 1 || got[0] {
				t.Fatalf("confirmations %v, want [false]", got)
			}
		})
	}
}

type params1 struct{ fakeLink }

var testParams1 = asdu.Params{CauseSize: 1, CommonAddrSize: 1, InfoObjAddrSize: 2, InfoObjTimeZone: time.UTC}

func (*params1) Params() *asdu.Params { return &testParams1 }

func TestIsGlobalCA(t *testing.T) {
	l2, l1 := &fakeLink{}, &params1{}
	for _, c := range []struct {
		l    asdu.Connect
		ca   asdu.CommonAddr
		want bool
	}{
		{l2, asdu.GlobalCommonAddr, true},
		{l2, 0xFF, false},
		{l1, asdu.GlobalCommonAddr, true}, // 255 as go-iecp5 decodes it
		{l1, 0xFF, true},
		{l1, 1, false},
	} {
		if got := isGlobalCA(c.l, c.ca); got != c.want {
			t.Errorf("isGlobalCA(size %d, %#x) = %v", c.l.Params().CommonAddrSize, c.ca, got)
		}
	}
}
