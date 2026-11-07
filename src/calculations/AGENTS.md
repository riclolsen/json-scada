# DOX: src/calculations — Cyclic Calculations Engine

## Purpose

Compiled cyclic calculations engine written in Go. Executes user-defined calculation rules against MongoDB real-time data on a configurable cycle.

## Ownership

- calculations owns the calculations engine implementation

## Local Contracts

- **Language:** Go 1.26+
- **Single file:** `calculations.go` (single main package)
- **Dependency management:** `go.mod` / `go.sum`
- **Build output:** `calculations.exe`
- **Config:** INI file via Supervisor

## Work Guidance

- Calculation rules are stored in MongoDB and loaded at startup
- The engine runs on a configurable cycle (e.g., every N seconds)
- Supports arithmetic operations, logical operations, and data lookup
- Results are written back to the real-time data collection in MongoDB
- Follow Go idioms: `error` returns, `context.Context`, clean shutdown via signal handling

## Verification

- `go build .` — compiles without errors
- `go vet .` — no issues
- Test with calculation rules defined in MongoDB
