# DOX: src/calculations — Cyclic Calculations Engine

## Purpose

Compiled cyclic calculations engine written in Go. Computes calculated points (`origin: "calculated"`, a `formula` code and its `parcels`) from predefined formulas over MongoDB realtime data on a configurable cycle.

## Ownership

- calculations owns the calculations engine and the formula catalog

## Local Contracts

- **Language:** Go 1.26+, single `main` package: `calculations.go` (config, redundancy, cycle, MongoDB access), `formulas.go` (formula table), `formulas_test.go`
- **Dependency management:** `go.mod` / `go.sum`
- **Build output:** `calculations` binary in the installation `bin/` (`calculations.exe` on Windows); config from `../conf/json-scada.json` or the 4th argument
- Formulas live only in the `formulas` table (`formulas.go`): name/expression, parcel count rule, evaluation with the invalid/transient rules; `calculate` applies the count rule and turns NaN/Inf into 0, invalid. Never add a formula as code outside the table
- Formula codes are a contract with `README.md` (Formulas tables) and `src/mcp-json-scada-db/src/jsonscada/calculations.ts`, which mirrors each entry to check and preview calculated points (the README tables are the MCP `formulaTable` rows). Change all three together; codes >= 100000 stay reserved for user processes
- Only points with `origin: "calculated"` and `formula > 0` are computed; they are reloaded every `reloadPeriod` (30 s), and points whose parcel count does not fit or whose formula is unknown are logged and skipped, never computed
- Missing parcels count as 0, invalid
- Results go to `sourceDataUpdate` (`valueAtSource`, `invalidAtSource`, `transientAtSource`, `timeTag`) for the data processor, written only when they differ from the stored `sourceDataUpdate`

## Work Guidance

- Keep the process writing only `sourceDataUpdate` of calculated points and its `processInstances` entry (redundancy)
- Behaviour changes to existing formulas change values in running projects: list them in `README.md` (Changes section) with the version

## Verification

- `go build .`, `go vet .`, `gofmt -l .` (no output)
- `go test ./...` — formula table: parcel count rules (no panics), reference values, NaN handling
- Parity with the MCP catalog and scenario checks: the "Calculations parity and e2e" recipe in [src/mcp-json-scada-db/AGENTS.md](../mcp-json-scada-db/AGENTS.md)
