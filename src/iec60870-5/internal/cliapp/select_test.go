package cliapp

import (
	"testing"
	"time"

	"github.com/riclolsen/go-iecp5/asdu"

	"iec60870-5/internal/conv"
)

func TestTakeSelected(t *testing.T) {
	exec := &conv.InfoObject{TypeID: asdu.C_SC_NA_1, Ioa: 500}
	sel := conv.SelectConfirm{Ca: 1, Ioa: 500, Type: asdu.C_SC_NA_1}
	c := &Conn{}
	c.SetLastSelected(exec, 1)

	for name, bad := range map[string]conv.SelectConfirm{
		"other station": {Ca: 2, Ioa: 500, Type: asdu.C_SC_NA_1},
		"other object":  {Ca: 1, Ioa: 501, Type: asdu.C_SC_NA_1},
		"other type":    {Ca: 1, Ioa: 500, Type: asdu.C_DC_NA_1},
	} {
		if c.TakeSelected(bad) != nil {
			t.Errorf("%s matched", name)
		}
	}
	if c.TakeSelected(sel) != exec {
		t.Fatal("matching confirmation did not take the execute")
	}
	if c.TakeSelected(sel) != nil {
		t.Fatal("a repeated confirmation executed again")
	}

	c.SetLastSelected(exec, 1)
	c.ClearSelected()
	if c.TakeSelected(sel) != nil {
		t.Fatal("executed after the connection was lost")
	}

	c.SetLastSelected(exec, 1)
	c.mu.Lock()
	c.lastSelectedAt = time.Now().Add(-2 * SelectTimeout)
	c.mu.Unlock()
	if c.TakeSelected(sel) != nil {
		t.Fatal("expired select executed")
	}
}
