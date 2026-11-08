# DOX: src/demo_simul — Demo Process Simulator

## Purpose

Node.js service that simulates the electrical process behind the demo database:
it publishes physically coherent analog/digital values into MongoDB, answers
supervisory commands and generates protection events.

## Ownership

- demo_simul owns the demo process simulation (values, switching, faults)
- it does NOT own calculated points (`origin: 'calculated'`) — those belong to
  the `calculations` driver — nor manual points (`origin: 'manual'`)
- it does NOT touch points or commands of `CIM-EMS` and `UNDEFINED` connections
  (`FOREIGN_DRIVERS` in `index.js`): CIM-EMS results (invalid means "not
  calculated", never revalidate) and externally simulated points (Svedala demo
  field, `cim_ems.demo_field`)

## Local Contracts

- **Language:** Node.js
- **Main entry:** `index.js` (MongoDB plumbing only)
- **Model:** `power-model.js` (topology discovery + electrical physics)
- **Tunables:** `sim-config.js`, overridable by `JS_DEMO_SIMUL_*` env vars
- **Pattern:** standard Node.js app (`app-defs.js`, `load-config.js`, `simple-logger.js`)
- **Config:** INI file via Supervisor / `conf/json-scada.json`
- **Integration point:** writes the `sourceDataUpdate` subdocument of
  `realtimeData` exactly like a protocol driver, so `cs_data_processor` handles
  conversion, alarms and SOE; reads `commandsQueue` and acknowledges commands

## Work Guidance

- The topology is derived from the database at runtime, never hard-coded: tag
  convention `SSSSMMMMMPPPP…` (4 station + 5 module + point code), `group1` as
  station, `group2` as bay (nominal kV parsed from its text), `unit` as fallback
  for the quantity classification. New demo tags are picked up automatically.
- Everything is anchored on `valueDefault` so the demo keeps its familiar look;
  physics governs how values move, not their magnitude.
- Derived quantities must stay consistent: `I = 1000/sqrt(3)·S/V` (same constant
  as the calculations driver), `S = hypot(P,Q)`, open breaker ⇒ zero flow,
  transformer outgoing winding = incoming minus losses.
- A `valueDefault` of zero means "not in service" for frequency, temperature and
  auxiliary voltage points — keep them at zero.
- Publishing is deadband-based with a periodic forced refresh; do not publish
  every point on every step.

## Verification

- `npm install` — dependencies install cleanly
- Model-level: build `PowerModel` from `demo-docker/mongo_seed/files/demo_data.json`
  and step it; assert currents match `1000/sqrt(3)·S/V`, `S = hypot(P,Q)`, open
  bays carry no power, transformer losses are positive, no NaN, and points with
  a zero default stay at zero
- End-to-end: seed the demo data into a replica-set mongod, run `index.js`, then
  check `sourceDataUpdate` coherence in `realtimeData`, insert a breaker and a
  tap command into `commandsQueue` and confirm the status change plus the `ack`
