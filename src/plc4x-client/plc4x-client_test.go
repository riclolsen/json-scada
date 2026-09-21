package main

import (
	"context"
	"net"
	"testing"
	"time"

	plc4go "github.com/apache/plc4x/plc4go/pkg/api"
	"github.com/apache/plc4x/plc4go/pkg/api/config"
	"github.com/apache/plc4x/plc4go/pkg/api/drivers"
	"github.com/apache/plc4x/plc4go/pkg/api/transports"
	"github.com/rs/zerolog"
)

func TestMigrateLegacyModbusOptions(t *testing.T) {
	cases := []struct{ in, want string }{
		{"modbus-tcp://host:502?unit-identifier=3", "modbus-tcp://host:502?default-unit-identifier=3"},
		{"modbus-rtu://COM1?baud-rate=9600&unit-identifier=7", "modbus-rtu://COM1?baud-rate=9600&default-unit-identifier=7"},
		{"modbus-tcp://host:502?default-unit-identifier=4", "modbus-tcp://host:502?default-unit-identifier=4"},
		{"modbus-tcp://host:502", "modbus-tcp://host:502"},
		{"opcua://host:4840?unit-identifier=3", "opcua://host:4840?unit-identifier=3"},
	}
	for _, c := range cases {
		if got := migrateLegacyModbusOptions("T", c.in); got != c.want {
			t.Errorf("migrateLegacyModbusOptions(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestUnitIdentifierReachesTheWire connects to a stub listener and checks the unit id in the MBAP
// header of the request Ping sends, which is where a wrongly-named option would show up.
func TestUnitIdentifierReachesTheWire(t *testing.T) {
	zerolog.SetGlobalLevel(zerolog.Disabled)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	firstRequest := make(chan []byte, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		buf := make([]byte, 256)
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		firstRequest <- buf[:n]
	}()

	driverManager := plc4go.NewPlcDriverManager(config.WithCustomLogger(zerolog.Logger{}))
	transports.RegisterTcpTransport(driverManager)
	drivers.RegisterModbusTcpDriver(driverManager)

	url := migrateLegacyModbusOptions("T", "modbus-tcp://"+listener.Addr().String()+"?unit-identifier=3")
	connection, err := driverManager.GetConnection(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer connection.Close()
	go connection.Ping(context.Background())

	select {
	case req := <-firstRequest:
		if len(req) < 8 {
			t.Fatalf("short request: % x", req)
		}
		// MBAP header: transaction id (2), protocol id (2), length (2), unit id (1), function code (1)
		if req[6] != 3 {
			t.Errorf("unit id on the wire = %d, want 3 (request: % x)", req[6], req)
		}
		if req[7] != 0x03 {
			t.Errorf("function code = %#x, want 0x03 (read holding registers)", req[7])
		}
	case <-time.After(15 * time.Second):
		t.Fatal("no request reached the listener")
	}
}
