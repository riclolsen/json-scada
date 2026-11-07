# DOX: src/modbus-go — Modbus Client and Server Drivers (Go)

## Purpose

Go Modbus master and slave drivers for JSON-SCADA, built on `github.com/otfabric/go-modbus`.
Drop-in replacements for the former Node.js drivers in `src/modbus` (sources removed in 0.69):
same `protocolDriver` names (`MODBUS`, `MODBUS_SERVER`), same configuration documents and
defaults, same CLI and environment variables (`JS_MODBUS_`, `JS_MODBUSSRV_`), same service names.

## Ownership

- modbus-go owns the Go implementation of both Modbus drivers
- The Node.js sources this implementation was matched against are gone; `README.md` keeps the
  numbered behavioural differences for reference

## Local Contracts

- **Language:** Go 1.26, module `modbus-go`, `cmd/` + `internal/` layout, listed in `src/go.work`
- **Library:** `github.com/otfabric/go-modbus` v1.10.7 (MIT) and `github.com/otfabric/go-serial`.
  Pin the version; diff the API of what this module uses before bumping.
- **Binaries:** `modbus-client(.exe)` and `modbus-server(.exe)` in `bin/`.
- **Service names do not change:** Supervisor programs `modbusclient` / `modbusserver`, Windows
  services `JSON_SCADA_modbusclient` / `JSON_SCADA_modbusserver`, process-manager keys
  `MODBUS` / `MODBUS_SERVER`.
- **Packages:**
  - `cmd/modbusclient`, `cmd/modbusserver` — entry points; `Main` takes the full `os.Args`
  - `internal/mbcore` — codec, byte orders, addresses/ASDUs, CRC/RTU framing, request
    descriptions, JavaScript number semantics. No MongoDB or driver imports.
  - `internal/mbnet` — TLS configs, client loopback `Bridge`, server `Front` and RTU gateway,
    serial ports
  - `internal/drv` — CLI/config start-up, instance document, loggers, BSON/JS conversions
  - `internal/clientapp` — `MODBUS` driver; `internal/serverapp` — `MODBUS_SERVER` driver
  - `internal/loopback` — client ↔ server tests over every network transport
  - `src/go-common` via a `replace` directive — `jsconfig`, `jslog`, `jsmongo`,
    `jsredundancy`, `jsstats`, `jsrtdata`, `jscommands`, `jstls`, `jsmodel`
- **Build:** `build.bat`, or `go build -ldflags="-s -w" -o ../../bin/modbus-client ./cmd/modbusclient`
  (and `modbus-server` from `./cmd/modbusserver`)
- **Documentation:**
  - `cmd/modbusclient/README.md` and `cmd/modbusserver/README.md` are the standalone operator
    references; the Node.js sources and their docs are no longer in the repository.
  - `README.md` keeps what both share (transports, types, byte orders, addressing, logging),
    build, architecture, the numbered differences from the former Node.js drivers, migration
    and testing.
  - Every key `config.go` reads must be in the driver README (`TestReadmeDocumentsEveryConfigKey`).
    A changed default, statistic, command result or log line is a README change in the same commit.

## Work Guidance

- The former Node.js drivers are the behavioural specification. Intentional differences are
  numbered in `README.md` ("Differences from the Node.js drivers"); add to that list rather
  than diverging silently.
- Read configuration with the `drv` helpers (`Num`, `Str`, `Bool`, `StrArr`, `ToNum`, `JSNumber`,
  `AnyString`) so missing, `null`, `""` and wrong-typed values behave as in JavaScript.
- Client data goes only to `sourceDataUpdate`; numbers written to MongoDB are `float64`, except
  the `stats` counters (`int64`).
- go-modbus gaps are bridged, not patched: client TLS and local bind go through `mbnet.Bridge`;
  the server's go-modbus instance listens on a private `127.0.0.1` port behind `mbnet.Front`,
  which owns the allow-list, client limit, idle timeout, TLS and RTU framing. Handlers must
  resolve `ClientAddr` through `AddrMap` to name the real master.
- Server handler exceptions are wrapped in `handlerError` (logged and counted once); the
  `metrics` hook logs only exceptions the library sends itself. Keep that split.
- Do not set a client-level timeout on the MongoDB client; give each call its own deadline
  (change streams would be torn down otherwise).

## Verification

- `go vet ./...` and `gofmt -l .` clean; `go test ./...` and `go test -race ./...` pass.
  No MongoDB or hardware needed; the TLS loopback tests generate certificates in `t.TempDir()`.
- Interop by hand: `diagslave` / `modpoll` (see `README.md`).

## Child DOX Index

None.
