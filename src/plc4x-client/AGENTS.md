# DOX: src/plc4x-client — PLC4X Modbus Client Driver

## Purpose

Go-based Modbus client driver that uses the Apache PLC4X library to connect to Modbus devices. Bridges Modbus register/coil data to JSON-SCADA MongoDB.

## Ownership

- plc4x-client owns the Modbus client protocol driver via PLC4X

## Local Contracts

- **Language:** Go 1.21+
- **Structure:**
  - `plc4x-client.go` — main application
  - `autotag.go` — automatic tag creation from device configuration
  - `config.go` — configuration parsing
- **Dependency management:** `go.mod` / `go.sum`
- **Config:** INI file via Supervisor or environment variables
- **Build:** `go build`

## Work Guidance

- Uses Apache PLC4X Go library for Modbus TCP/RTU connectivity
- Supports Modbus coils, discrete inputs, input registers, and holding registers
- Auto-tag feature creates MongoDB tags from device register maps
- Handles reconnection with backoff
- Follows standard JSON-SCADA Go driver pattern

## Verification

- `go build .` — compiles without errors
- `go vet .` — no issues
- Test with a Modbus device simulator or real PLC
