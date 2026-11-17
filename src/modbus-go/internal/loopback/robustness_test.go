/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

package loopback

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"modbus-go/internal/clientapp"
	"modbus-go/internal/serverapp"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func startServer(t *testing.T, cmds cmdSink) *serverapp.Server {
	t.Helper()
	srv := serverapp.NewServer(serverapp.Normalize(bson.M{
		"protocolConnectionNumber": srvConn, "name": "SRV", "ipAddressLocalBind": "127.0.0.1:0",
	}), cmds, nil)
	srv.PM.Rebuild(serverTags())
	srv.Start()
	t.Cleanup(srv.Stop)
	waitFor(t, "server listening", 5*time.Second, func() bool { return srv.Addr() != "" })
	return srv
}

// failingSink fails its first writes, then records like rtSink.
type failingSink struct {
	rtSink
	mu    sync.Mutex
	fails int
}

func (s *failingSink) Apply(ctx context.Context, ops []clientapp.RtOp) error {
	s.mu.Lock()
	if s.fails > 0 {
		s.fails--
		s.mu.Unlock()
		return errors.New("mongo down")
	}
	s.mu.Unlock()
	return s.rtSink.Apply(ctx, ops)
}

// Values whose write failed are written again by the next cycle, although
// they did not change and no integrity cycle is due.
func TestFailedWriteIsRetried(t *testing.T) {
	srv := startServer(t, cmdSink{ch: make(chan bson.D, 4)})
	sink := &failingSink{fails: 2}
	cli := clientapp.NewConnection(clientapp.Normalize(bson.M{
		"protocolConnectionNumber": 60.0, "ipAddresses": bson.A{srv.Addr()}, "pollingInterval": 100.0,
	}), sink)
	cli.SetBindings([]bson.M{{"_id": "a", "tag": "AN1", "protocolSourceObjectAddress": "hr:0", "protocolSourceASDU": "float32"}})
	cli.Start()
	defer cli.Stop()
	waitFor(t, "AN1 written after the failures", 5*time.Second, func() bool {
		s, ok := sink.get("a")
		return ok && s["valueAtSource"] == 123.5
	})
	if st := cli.Stats(); st.Status != "connected" {
		t.Fatalf("status %+v", st)
	}
}

type queueStore struct {
	docs []bson.M
	mu   sync.Mutex
	acks []string
}

func (q *queueStore) Watch(ctx context.Context, _ float64, onInsert func(bson.M)) {
	for _, d := range q.docs {
		onInsert(d)
	}
	<-ctx.Done()
}

func (q *queueStore) Ack(_ context.Context, id any, ok bool, desc string) error {
	q.mu.Lock()
	q.acks = append(q.acks, fmt.Sprintf("%v:%v", id, ok))
	q.mu.Unlock()
	return nil
}

func (q *queueStore) done() []string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]string(nil), q.acks...)
}

// Commands are executed one at a time, in the order they were inserted.
func TestCommandsRunInOrder(t *testing.T) {
	cmds := cmdSink{ch: make(chan bson.D, 64)}
	srv := startServer(t, cmds)
	const n = 20
	store := &queueStore{}
	for i := 0; i < n; i++ {
		store.docs = append(store.docs, bson.M{
			"_id": i, "tag": "CMD1", "protocolSourceObjectAddress": "hr:10", "protocolSourceASDU": "int16",
			"protocolSourceCommonAddress": 1.0, "value": float64(i), "timeTag": time.Now(),
		})
	}
	cli := clientapp.NewConnection(clientapp.Normalize(bson.M{
		"protocolConnectionNumber": 60.0, "ipAddresses": bson.A{srv.Addr()}, "pollingInterval": 100.0,
	}), &rtSink{})
	cli.SetBindings([]bson.M{{"_id": "a", "tag": "AN1", "protocolSourceObjectAddress": "hr:0", "protocolSourceASDU": "float32"}})
	cli.Commands = store
	// The link must be up before the commands arrive, or they are refused.
	cli.Start()
	defer cli.Stop()
	waitFor(t, "every command acknowledged", 10*time.Second, func() bool { return len(store.done()) == n })
	for i, a := range store.done() {
		if a != fmt.Sprintf("%d:true", i) && a != fmt.Sprintf("%d:false", i) {
			t.Fatalf("ack %d is %q: out of order (%v)", i, a, store.done())
		}
	}
}
