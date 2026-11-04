/*
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * Licensed under the GNU General Public License v3. See LICENSE in the repo root.
 */

// Package loopback runs the Go client against the Go server in every network
// transport, with in-memory sinks instead of MongoDB.
package loopback

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"modbus-go/internal/clientapp"
	"modbus-go/internal/mbcore"
	"modbus-go/internal/serverapp"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type rtSink struct {
	mu   sync.Mutex
	last map[string]bson.M
}

func (s *rtSink) Apply(_ context.Context, ops []clientapp.RtOp) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil {
		s.last = map[string]bson.M{}
	}
	for _, op := range ops {
		if sdu, ok := op.Set["sourceDataUpdate"].(bson.M); ok {
			s.last[op.ID.(string)] = sdu
		} else if inv, ok := op.Set["sourceDataUpdate.invalidAtSource"]; ok {
			s.last[op.ID.(string)] = bson.M{"invalidAtSource": inv}
		}
	}
	return nil
}

func (s *rtSink) get(id string) (bson.M, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.last[id]
	return v, ok
}

type cmdSink struct{ ch chan bson.D }

func (c cmdSink) Insert(_ context.Context, doc bson.D) error { c.ch <- doc; return nil }

type ackStore struct{ ch chan string }

func (a ackStore) Watch(ctx context.Context, _ float64, _ func(bson.M)) { <-ctx.Done() }
func (a ackStore) Ack(_ context.Context, _ any, ok bool, desc string) error {
	if !ok {
		desc = "NAK " + desc
	}
	a.ch <- desc
	return nil
}

const srvConn = 50.0

func dest(addr, asdu string) bson.A {
	return bson.A{bson.M{
		"protocolDestinationConnectionNumber": srvConn,
		"protocolDestinationObjectAddress":    addr,
		"protocolDestinationASDU":             asdu,
		"protocolDestinationCommonAddress":    1.0,
	}}
}

func serverTags() []serverapp.TagDoc {
	docs := []bson.M{
		{"_id": 1.0, "tag": "AN1", "type": "analog", "value": 123.5, "protocolDestinations": dest("hr:0", "float32")},
		{"_id": 2.0, "tag": "DI1", "type": "digital", "value": 1.0, "protocolDestinations": dest("co:5", "")},
		{"_id": 3.0, "tag": "CMD1", "type": "analog", "origin": "command",
			"protocolSourceConnectionNumber": 99.0, "protocolSourceCommonAddress": 7.0,
			"protocolSourceObjectAddress": "hr:300", "protocolSourceASDU": "int16",
			"value": 0.0, "protocolDestinations": dest("hr:10", "int16")},
	}
	var out []serverapp.TagDoc
	for _, d := range docs {
		t, ok := serverapp.ExtractTagDoc(d, srvConn)
		if !ok {
			panic("no destination")
		}
		out = append(out, t)
	}
	return out
}

type certs struct{ ca, srvCert, srvKey, cliCert, cliKey string }

func makeCerts(t *testing.T) certs {
	t.Helper()
	dir := t.TempDir()
	write := func(name, typ string, der []byte) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	caKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	caTpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, _ := x509.CreateCertificate(rand.Reader, caTpl, caTpl, &caKey.PublicKey, caKey)
	caCert, _ := x509.ParseCertificate(caDER)
	leaf := func(serial int64, cn string, usage x509.ExtKeyUsage) ([]byte, []byte) {
		k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		tpl := &x509.Certificate{
			SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: cn},
			NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
			KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage},
			IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		}
		der, _ := x509.CreateCertificate(rand.Reader, tpl, caCert, &k.PublicKey, caKey)
		kd, _ := x509.MarshalPKCS8PrivateKey(k)
		return der, kd
	}
	sc, sk := leaf(2, "server", x509.ExtKeyUsageServerAuth)
	cc, ck := leaf(3, "client", x509.ExtKeyUsageClientAuth)
	return certs{
		ca:      write("ca.pem", "CERTIFICATE", caDER),
		srvCert: write("srv.pem", "CERTIFICATE", sc), srvKey: write("srv.key", "PRIVATE KEY", sk),
		cliCert: write("cli.pem", "CERTIFICATE", cc), cliKey: write("cli.key", "PRIVATE KEY", ck),
	}
}

func waitFor(t *testing.T, what string, d time.Duration, f func() bool) {
	t.Helper()
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if f() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestLoopback(t *testing.T) {
	cases := []struct {
		name, srvMode, cliMode string
		tls                    bool
	}{
		{"tcp", serverapp.ModeTCP, clientapp.ModeTCP, false},
		{"rtu-over-tcp", serverapp.ModeRTUOverTCP, clientapp.ModeRTUOverTCP, false},
		{"tls", serverapp.ModeTLS, clientapp.ModeTLS, true},
		{"rtu-over-tls", serverapp.ModeRTUOverTLS, clientapp.ModeRTUOverTLS, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srvDoc := bson.M{
				"protocolConnectionNumber": srvConn, "name": "SRV", "connectionMode": tc.srvMode,
				"ipAddressLocalBind": "127.0.0.1:0",
			}
			cliDoc := bson.M{
				"protocolConnectionNumber": 60.0, "name": "CLI", "connectionMode": tc.cliMode,
				"pollingInterval": 200.0, "timeoutMs": 1000.0,
			}
			if tc.tls {
				c := makeCerts(t)
				srvDoc["localCertFilePath"], srvDoc["privateKeyFilePath"], srvDoc["peerCertFilePath"] = c.srvCert, c.srvKey, c.ca
				cliDoc["localCertFilePath"], cliDoc["privateKeyFilePath"], cliDoc["peerCertFilePath"] = c.cliCert, c.cliKey, c.ca
			}
			cmds := cmdSink{ch: make(chan bson.D, 4)}
			srv := serverapp.NewServer(serverapp.Normalize(srvDoc), cmds, nil)
			srv.PM.Rebuild(serverTags())
			srv.Start()
			defer srv.Stop()
			waitFor(t, "server listening", 5*time.Second, func() bool { return srv.Addr() != "" })

			cliDoc["ipAddresses"] = bson.A{srv.Addr()}
			sink := &rtSink{}
			cli := clientapp.NewConnection(clientapp.Normalize(cliDoc), sink)
			acks := ackStore{ch: make(chan string, 4)}
			cli.Commands = acks
			cli.SetBindings([]bson.M{
				{"_id": "a", "tag": "AN1", "protocolSourceObjectAddress": "hr:0", "protocolSourceASDU": "float32", "protocolSourceCommonAddress": 1.0},
				{"_id": "b", "tag": "DI1", "protocolSourceObjectAddress": "co:5", "protocolSourceCommonAddress": 1.0},
				{"_id": "c", "tag": "MISSING", "protocolSourceObjectAddress": "hr:900", "protocolSourceASDU": "int16", "protocolSourceCommonAddress": 1.0},
			})
			cli.Start()
			defer cli.Stop()

			waitFor(t, "AN1 value", 10*time.Second, func() bool {
				s, ok := sink.get("a")
				return ok && s["valueAtSource"] == 123.5 && s["invalidAtSource"] == false
			})
			waitFor(t, "DI1 value", 5*time.Second, func() bool {
				s, ok := sink.get("b")
				return ok && s["valueAtSource"] == 1.0
			})
			waitFor(t, "MISSING invalid", 5*time.Second, func() bool {
				s, ok := sink.get("c")
				return ok && s["invalidAtSource"] == true
			})

			// A served value change reaches the client.
			srv.PM.Update(1.0, -7.25, "", false)
			waitFor(t, "AN1 update", 5*time.Second, func() bool {
				s, ok := sink.get("a")
				return ok && s["valueAtSource"] == -7.25
			})

			// A client command to the server's command point is relayed.
			go cli.HandleCommand(context.Background(), bson.M{
				"_id": "k1", "tag": "CMD1", "protocolSourceObjectAddress": "hr:10",
				"protocolSourceASDU": "int16", "protocolSourceCommonAddress": 1.0, "value": 42.0,
				"timeTag": time.Now(),
			})
			select {
			case doc := <-cmds.ch:
				m := map[string]any{}
				for _, e := range doc {
					m[e.Key] = e.Value
				}
				if m["value"] != 42.0 || m["protocolSourceConnectionNumber"] != 99.0 || m["protocolSourceObjectAddress"] != "hr:300" ||
					m["tag"] != "CMD1" || m["originatorUserName"] != "Protocol connection: SRV" {
					t.Fatalf("relayed command %v", m)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("no command relayed")
			}
			if a := <-acks.ch; a != "OK" {
				t.Fatalf("ack %q", a)
			}

			// A write to a supervised point is refused with an exception.
			go cli.HandleCommand(context.Background(), bson.M{
				"_id": "k2", "tag": "AN1", "protocolSourceObjectAddress": "hr:0",
				"protocolSourceASDU": "float32", "protocolSourceCommonAddress": 1.0, "value": 1.0,
			})
			if a := <-acks.ch; a == "OK" {
				t.Fatalf("write to supervised point accepted")
			}
			st := srv.Stats()
			if st.Reads == 0 || st.Writes < 2 || st.Exceptions == 0 || st.Clients != 1 {
				t.Fatalf("server stats %+v", st)
			}
			if cs := cli.Stats(); cs.Requests == 0 || cs.Exceptions == 0 {
				t.Fatalf("client stats %+v", cs)
			}
		})
	}
}

func TestRouteCommandValue(t *testing.T) {
	p := &serverapp.Point{TagType: serverapp.TagAnalog, Kconv1: 2, Kconv2: 1, TagKconv1: 10, TagKconv2: -5}
	v, s := serverapp.RouteCommandValue(p, mbcore.Num(3))
	if v != 65 || s != "65" {
		t.Fatalf("analog %v %q", v, s)
	}
	p = &serverapp.Point{TagType: serverapp.TagDigital, Kconv1: -1, TagKconv1: 1}
	if v, _ := serverapp.RouteCommandValue(p, mbcore.Num(1)); v != 0 {
		t.Fatalf("digital inverted %v", v)
	}
	p.TagKconv1 = -1 // two inversions cancel
	if v, _ := serverapp.RouteCommandValue(p, mbcore.Num(1)); v != 1 {
		t.Fatalf("digital double inversion %v", v)
	}
}
