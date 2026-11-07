# CIM-EMS Driver: Implementation Plan

Status: phase 0 done (section 20); phase 1 code done, exit gate waits for the user model; phase 2 and phase 3 cores done on public data (sections 21 to 23); branch `cim-ems`. This file is the single source of truth for the implementation until the component has its own README and AGENTS.md. It is working material: do not link it from committed docs, and delete it (as was done for the IEC 61850 server plan) once its content has moved into the component docs.

## 1. Goal

Add CIM/CGMES model exchange and EMS network analysis to JSON-SCADA as a standard protocol driver named `CIM-EMS`:

- Import and validate CGMES network models.
- Build the operating state from live JSON-SCADA telemetry (switches, taps, analogs).
- Run load flow, state estimation and contingency analysis.
- Publish selected results as ordinary JSON-SCADA points (alarms, SOE, historian, displays for free).
- Generate SVG displays in JSON-SCADA markup, bound to live and calculated points.
- Export the solved or estimated state as CGMES (EQ, SSH, TP, SV).

### Non-goals for this plan

- Automatic control or command execution from study results. Results are operator decision support only.
- Interactive CIM model authoring. Model changes happen by importing a new revision.
- DMS (unbalanced, phase-level distribution analysis). It gets its own plan after phase 6, reusing the driver shell, bindings and diagram tooling (see section 15).
- Byte-exact CGMES round trips. Round trips are checked on identity, connectivity, parameters and solved values.

## 2. Accepted decisions

| Topic | Decision |
|---|---|
| Integration shape | Standard JSON-SCADA protocol driver (`CIM-EMS`), not a bespoke sidecar |
| Redundancy | Existing `activeNodeName` / `activeNodeKeepAliveTimeTag` contract; only the active node publishes |
| Result publication | `origin: "supervised"` points owned by a CIM-EMS connection, written through `sourceDataUpdate` |
| Primary engine | PowSyBl through `pypowsybl`: CGMES import/export, load flow (OpenLoadFlow), security analysis, diagrams |
| State estimator | Second engine, pandapower WLS estimator, behind an adapter; final choice confirmed in phase 4 qualification |
| Language | Python 3 (pypowsybl and pandapower are Python-first); `pymongo` for MongoDB |
| CGMES version | **CGMES 2.4.15 (CIM16)** for import and export, the version best supported by PowSyBl (mature importer and exporter, ENTSO-E conformity cases) and the only one pandapower's CIM converter handles (its converter classes are CIM16). CGMES 3.0 (CIM100) is a connection option (`cgmesVersion`), exported with `iidm.export.cgmes.cim-version=100`, validated in phase 6 against an actual partner need |
| Analysis scope | Balanced, positive-sequence AC, transmission/sub-transmission |
| Diagrams | Generated from PowSyBl diagram output, converted to JSON-SCADA `inkscape:label` markup |
| Contingencies | User-defined contingency lists first; automatic N-1 later |
| Delivery | Docker first; Linux and Windows installers after the Python dependency set is frozen |

## 3. Architecture

```
            CGMES zip / XML (EQ, SSH, TP, SV, DL, BD)
                          |
                    [Model manager] ---- GridFS: original packages, diagrams, results
                          |
                   cimModels (revisions, validation, activation)
                          |
realtimeData ---> [Snapshot + topology] <--- cimMeasurementBindings
(processed values)        |
                   operating state (pypowsybl Network)
                          |
          +---------------+----------------+
          |               |                |
     [Load flow]   [State estimation]  [Contingency]
     pypowsybl      pandapower adapter   pypowsybl SA
          |               |                |
          +-------+-------+--------+-------+
                  |                |
            cimRuns (inputs,   [Publisher] --> realtimeData.sourceDataUpdate
            outputs, status)       |               (cs_data_processor: alarms,
                  |                |                SOE, historian)
            [Exporter]        [Diagram generator] --> svg/ displays
            CGMES SSH/SV
```

All boxes are modules of one driver process. Long studies run in worker processes (section 10.4).

## 4. JSON-SCADA integration contract

### 4.1 Driver registration

- `protocolDriver`: `CIM-EMS`.
- Executable: `bin/cim-ems.sh` (Linux) and `bin\cim-ems.bat` (Windows), wrapping `python -m cim_ems <instance> <logLevel> <configFile>`, following the argument order of the other drivers (see `docs/DEVELOPER_GUIDE.md`).
- Reads `conf/json-scada.json` for MongoDB URI, database name and `nodeName`.
- Exits when its `protocolDriverInstances` document is missing or disabled.
- Redundancy exactly as in the developer guide: become active when `activeNodeName` is empty or its keepalive is stale, refresh the keepalive while active, stop publishing (keep computing optional, default off) while inactive.
- Every publication checks that this node is still active immediately before the write. A node that lost activity mid-run discards its result.

### 4.2 `protocolConnections` (one connection = one network study context)

Standard fields plus CIM-EMS fields. Example (strict JSON):

```json
{
  "protocolDriver": "CIM-EMS",
  "protocolDriverInstanceNumber": 1,
  "protocolConnectionNumber": 9001,
  "name": "EMS_MAIN",
  "description": "Main transmission network analysis",
  "enabled": true,
  "commandsEnabled": false,
  "autoCreateTags": true,
  "cgmesVersion": "2.4.15",
  "cimModelId": "TSO_2026_11",
  "cimModelRevision": 3,
  "cimBindingRevision": 5,
  "cycleSeconds": 60,
  "runLoadFlow": true,
  "runStateEstimation": false,
  "runContingency": false,
  "contingencyListId": "N1_CRITICAL",
  "snapshotMaxAgeSeconds": 30,
  "snapshotMaxSkewSeconds": 10,
  "resultMaxAgeSeconds": 180,
  "loadFlowParameters": {
    "distributedSlack": true,
    "balanceType": "PROPORTIONAL_TO_GENERATION_P_MAX",
    "voltageInitMode": "DC_VALUES",
    "transformerVoltageControlOn": false
  },
  "estimatorParameters": {
    "algorithm": "wls",
    "maxIterations": 50,
    "tolerance": 0.000001,
    "badDataChiSquareConfidence": 0.99
  },
  "publishFilter": {
    "busVoltages": true,
    "branchFlows": true,
    "branchLoading": true,
    "minNominalKv": 69
  }
}
```

- `commandsEnabled` stays `false`; the driver ignores `commandsQueue` in this plan.
- A run is triggered by the cycle timer, by a topology change (switch or tap state change on a bound point), or by an on-demand request (section 11).

### 4.3 Result points in `realtimeData`

Result points are `origin: "supervised"`, `protocolSourceConnectionNumber` = the CIM-EMS connection. The binding lives in the protocol address fields:

| Field | Content |
|---|---|
| `protocolSourceCommonAddress` | Study kind: `LF`, `SE`, `CA`, `SYS` |
| `protocolSourceObjectAddress` | `<mRID>|<side>|<quantity>`; `side` is `1`/`2`/`3` for branch ends, `0` for buses and system points |
| `protocolSourceASDU` | `analog` or `digital` |

Quantities and units:

| Quantity | Applies to | Unit |
|---|---|---|
| `V` | bus (topological node / bus view) | kV |
| `VPU` | bus | pu |
| `ANG` | bus | deg |
| `P`, `Q` | branch end, injection | MW, Mvar |
| `I` | branch end | A |
| `LOAD` | branch (max of ends against the active limit) | % |
| `VIOL` | CA: number of post-contingency violations | count |
| `WORST` | CA: worst post-contingency loading | % |
| `STATUS`, `CONVERGED`, `ITER`, `AGE`, `BADDATA`, `OBSERV`, `JOBJ` | `SYS` points for each study kind | various |

Auto-created tag names (`autoCreateTags: true`): `<connectionName>;<studyKind>;<equipmentName>;<quantity>[;<side>]`, falling back to the mRID when the CIM name is empty or ambiguous. This follows the generated-name convention documented in `docs/schema.md`; group1/group2 come from the CIM Substation / VoltageLevel names for navigation only.

Publication rules:

- Write `sourceDataUpdate` with `valueAtSource`, `timeTagAtSource` = snapshot time, `timeTagAtSourceOk: true`, `invalidAtSource`, `originator: "CIM-EMS|<connectionNumber>|<runId>"`.
- Failed, non-converged or superseded run: do not overwrite values; set `invalidAtSource: true` on the affected study points once `resultMaxAgeSeconds` is exceeded.
- Never publish a run older than the last published run of the same connection and study kind (monotonic `runId`).
- Limits and alarms are configured on the points as usual (`hiLimit`, etc.); the data processor raises the alarms. The driver does not write `soeData` or alarms directly.
- Estimated values are never used as estimator inputs. Bindings refuse a CIM-EMS point as an input source.

### 4.4 New collections

All created with indexes by `mongo_seed` (and by the driver at startup if missing).

`cimModels`, one document per model revision:

```json
{
  "modelId": "TSO_2026_11",
  "revision": 3,
  "status": "active",
  "cgmesVersion": "2.4.15",
  "profiles": ["EQ", "SSH", "TP", "SV", "DL"],
  "boundaryRef": "BD_ENTSOE_2026_10",
  "sourceArtifactId": "6730f2a1c2b9a0a1b2c3d4e5",
  "sourceSha256": "4f1c0c3e0d5b7a9e2f6a1b3c5d7e9f00112233445566778899aabbccddeeff00",
  "importedBy": "admin",
  "importedAt": "2026-11-07T12:00:00Z",
  "engine": {"pypowsybl": "pinned version", "importParameters": {"iidm.import.cgmes.convert-boundary": "false"}},
  "validation": {"errors": 0, "warnings": 12, "unsupported": ["EquivalentBranch with phase shift"], "reportArtifactId": "6730f2a1c2b9a0a1b2c3d4e6"},
  "counts": {"substations": 120, "lines": 310, "transformers": 95, "switches": 2400, "generators": 60, "loads": 400}
}
```

- `status`: `imported`, `validated`, `rejected`, `active`, `retired`. Exactly one `active` revision per `modelId`.
- Activation is an explicit operator action, never automatic on import.

`cimMeasurementBindings`, one document per bound input, versioned by `bindingRevision`:

```json
{
  "modelId": "TSO_2026_11",
  "bindingRevision": 5,
  "kind": "analog",
  "cimMeasurementMrid": "_9b2e7c1a-0f3d-4e6b-9a51-1c2d3e4f5a6b",
  "equipmentMrid": "_c1d2e3f4-a5b6-4c7d-8e9f-0a1b2c3d4e5f",
  "terminalMrid": "_d1e2f3a4-b5c6-4d7e-8f9a-0b1c2d3e4f5a",
  "side": 1,
  "quantity": "P",
  "tag": "KAW2_LT_KOR1_MW",
  "pointKey": 3012,
  "sign": 1,
  "unitCheck": "MW",
  "sigma": 2.0,
  "maxAgeSeconds": 30,
  "approved": true,
  "approvedBy": "engineer1",
  "proposedFrom": "cgmes-measurement"
}
```

- `kind`: `analog` (V, P, Q, I), `switch` (open/closed), `tap` (step position).
- `quantity` for analogs: `V`, `P`, `Q`, `I`.
- `sign` converts the SCADA convention to the CGMES terminal convention (positive = power flowing from the node into the equipment through the terminal).
- `proposedFrom`: `cgmes-measurement` (from CGMES Analog/Discrete objects), `name-match`, `manual`.
- Only `approved: true` bindings are used.

`cimRuns`, one document per study execution (TTL index for retention, default 7 days; detailed outputs in GridFS):

```json
{
  "runId": 1052331,
  "connectionNumber": 9001,
  "kind": "SE",
  "trigger": "cycle",
  "nodeName": "mainNode",
  "snapshotTime": "2026-11-07T12:01:00Z",
  "modelId": "TSO_2026_11",
  "modelRevision": 3,
  "bindingRevision": 5,
  "topologyHash": "a91f03c7",
  "engineVersions": {"pypowsybl": "pinned version", "pandapower": "pinned version"},
  "parametersHash": "77d0e1b2",
  "status": "converged",
  "durationMs": 840,
  "summary": {"iterations": 6, "objective": 312.4, "badData": 1, "observable": true, "islands": 1},
  "published": true,
  "resultArtifactId": "6730f2a1c2b9a0a1b2c3d4e7"
}
```

- `status`: `converged`, `diverged`, `unobservable`, `rejected-snapshot`, `unsupported`, `superseded`, `failed`.
- The tuple (model revision, binding revision, topology hash, snapshot time, engine versions, parameters hash) must reproduce the run offline (section 12.3).

`cimContingencyLists`: named lists of outages (element mRIDs, single or grouped), monitored element filter and limit set. Schema finalized in phase 5.

Artifacts (original packages, validation reports, generated SVG, detailed results, exports) go to a GridFS bucket `cimArtifacts`, so redundant nodes sharing MongoDB see the same files.

### 4.5 What the driver reads from `realtimeData`

- Processed values: `value`, `invalid`, `transient`, `substituted`, `timeTag`, `timeTagAtSource`. Never `sourceDataUpdate.valueAtSource`, so `kconv1`/`kconv2` are not applied twice.
- Access through a change stream on bound point keys, with a full read at startup and after stream resume failures; the snapshot is assembled from the in-memory cache at trigger time.

## 5. Component layout (`src/cim-ems/`)

```
src/cim-ems/
  AGENTS.md                 component DOX contract (created in phase 1)
  README.md                 user documentation
  pyproject.toml            pinned dependencies
  requirements.lock         fully pinned lock file used by Docker and installers
  cim_ems/
    __main__.py             CLI entry, args, logging
    config.py               json-scada.json, driver instance, connections
    redundancy.py           active-node keepalive
    mongo.py                collections, indexes, GridFS
    models/                 import, validation report, activation, revision store
    bindings/               proposal from CGMES measurements and name matching, approval checks
    snapshot.py             realtimeData cache, snapshot policy
    topology.py             switch/tap application, island detection, topology hash
    studies/
      loadflow.py           pypowsybl OpenLoadFlow
      estimation/           adapter interface, pandapower implementation, network transfer
      contingency.py        pypowsybl security analysis
    publish.py              result points, autoCreateTags, sourceDataUpdate, invalidation
    diagrams/               PowSyBl SVG + metadata to JSON-SCADA markup, layout overrides
    export.py               CGMES SSH/SV/TP/EQ export
    workers.py              process pool, limits, cancellation
    api.py                  study requests from cimStudyRequests collection (section 11)
  tests/
    unit/
    integration/            MongoDB-backed tests (replica set required for change streams)
    data/                   small public test cases only, no customer models
  tools/
    import_model.py         offline import and validation report
    replay.py               historian replay into a test database
    compare_engines.py      bus-by-bus LF comparison between pypowsybl and pandapower imports
```

## 6. Model management

1. Upload a CGMES package (zip of profile files) plus its boundary set. Store the original bytes in GridFS with SHA-256.
2. Import with pypowsybl using pinned import parameters. Record the parameters in `cimModels.engine`.
3. Validation report:
   - Parser errors and warnings from PowSyBl import reports.
   - Unsupported or approximated classes (e.g. equivalents, HVDC, phase shifters with unusual tables), each flagged reject or approximate.
   - Connectivity: isolated nodes, islands without slack-capable generation, dangling boundary lines.
   - Parameter sanity: zero or negative impedances, nominal voltage mismatch across a transformer, missing ratings.
   - Base-case load flow on the SSH/SV in the package; compare with the package SV (voltages, flows) within the gate 1 tolerances.
4. `validated` or `rejected`. An engineer reviews the report and activates a validated revision.
5. Activation of a new revision requires a binding revision compatible with it: bindings whose mRIDs no longer exist are reported, and the connection keeps the previous model until a binding revision for the new model is approved.
6. Rollback = activate a previous revision. Revisions are never edited in place.

## 7. Bindings

1. Proposal from CGMES measurement objects (Analog, Discrete, with Terminal and measurementType). PowSyBl can import these into network extensions; confirm on the user's files in phase 0. If absent, propose by name matching between CIM names and JSON-SCADA tags/descriptions, with a confidence score.
2. Engineer review in the Admin UI (phase 3) or by spreadsheet export/import (phase 2, faster to deliver): accept, edit sign, set sigma, reject.
3. Approval creates a new `bindingRevision`. Runs record the revision they used.
4. Coverage report: per voltage level, the share of switches, taps and branch flows bound; unbound switches are listed because they block correct topology.

## 8. Snapshot and topology

### 8.1 Snapshot policy

A snapshot is accepted only if:

- Every bound switch has a usable state (rules in 8.2).
- Analog values are not `invalid`, are younger than their `maxAgeSeconds`, and their `timeTagAtSource` spread is within `snapshotMaxSkewSeconds`. Values that fail are dropped as measurements (estimator) or flagged (load flow), not silently used.
- `substituted` values are used and flagged in the run summary.

A rejected snapshot produces a `cimRuns` entry with `rejected-snapshot` and the reasons; points age out to invalid per 4.3.

### 8.2 Switch and tap rules

| SCADA state | Applied state | Run flag |
|---|---|---|
| valid open / closed | open / closed | none |
| `transient` (intermediate) | last valid state | `topologyUncertain` |
| `invalid` | last valid state, if younger than a configured limit, else reject snapshot | `topologyUncertain` |
| unbound switch | model state from SSH | counted in coverage report |

- Never assume closed.
- Tap positions: valid integer steps within the tap changer range; out-of-range is invalid.
- Apply through pypowsybl `update_switches` and tap changer updates on a working copy of the active model network.
- After applying: bus view topology, island detection, energized/de-energized equipment, topology hash. A hash change versus the previous run triggers a run.

## 9. Studies

### 9.1 Load flow

- pypowsybl OpenLoadFlow, AC, parameters from the connection.
- Injections: SSH values by default; optional "telemetry-driven" mode replaces load and generator P/Q with bound measurements where available.
- Results per bus and branch end; loading against the active operational limit set (PATL first, TATL later).

### 9.2 State estimation

- Adapter interface: `estimate(network_state, measurements, parameters) -> EstimationResult` with voltages, flows, residuals, normalized residuals, bad data list, observability verdict and convergence data.
- First implementation: pandapower WLS. Network transfer options, qualified in phase 4:
  - A: import the same CGMES package with pandapower's CIM converter and apply the same topology.
  - B: build the pandapower net from the solved pypowsybl network.
  - Choose by `tools/compare_engines.py`: load flow results must match pypowsybl bus by bus within gate 1 tolerances before any estimator result is trusted.
- Measurements: bound analogs plus pseudo-measurements (SSH injections with large sigma) and virtual measurements (zero injection buses, low sigma), each typed in the result.
- Bad data: largest normalized residual test with chi-square pre-check; iterate removal up to a configured count; report removed measurements.
- Observability: report unobservable islands; publish their buses as invalid instead of guessing.
- Publication: `SE` study points plus `SYS` health points (`CONVERGED`, `OBSERV`, `BADDATA`, `JOBJ`, `AGE`).

### 9.3 Contingency analysis

- Runs only after a converged LF or SE of the same snapshot (base case taken from SE when enabled).
- pypowsybl security analysis with a user-defined `cimContingencyLists` list.
- Outcome per contingency: `secure`, `violation`, `islanding`, `diverged`, `unsupported`. Violations: branch current/loading, bus voltage high/low.
- Publish `CA` summary points (`VIOL`, `WORST`, per-contingency status for a configured top list); full results to GridFS.
- If the previous CA run is still running, skip this cycle and count the skip.

### 9.4 Export

- From the solved or estimated state: SSH and SV (and TP) referencing the active EQ, plus EQ re-export.
- Check: re-import the export, compare identity, connectivity, parameters and solved values with the source state.
- Export on demand and optionally on a schedule; files to GridFS with download from the Admin UI.

## 10. Diagrams and displays

### 10.1 Generation

- PowSyBl single-line diagrams per voltage level or substation, and network-area diagrams, with their metadata (element IDs per SVG node).
- If the package has a DL profile, use its layout where pypowsybl supports it; otherwise use generated layout.

### 10.2 Conversion to JSON-SCADA markup

- Converter adds `inkscape:label` JSON markup per the `svg-scada` skill (`.agents/skills/svg-scada/SKILL.md`): breakers and disconnectors get color/state bindings to their bound switch tags; branch ends and buses get text bindings to SE/LF result tags and measured tags; invalid and stale states use the existing quality colors.
- Tag references use tag names from bindings and auto-created result points; the converter fails if a referenced tag does not exist.
- Output SVGs are written to the display folder and added to the screen list (`conf-templates/screen_list.js` pattern) under a `CIM` group.
- Generated files must pass the SVG checks in `svg/AGENTS.md` (XML parses, every label JSON parses, tags exist).

### 10.3 Layout preservation

- Operator layout edits are stored as overrides keyed by equipment ID (position, rotation, label offset), in a sidecar JSON file per display in GridFS.
- Regeneration after a model revision reapplies overrides and lists overrides whose equipment disappeared.
- Values update live through normal display bindings; regeneration is needed only for structural model changes.

### 10.4 Runtime isolation

- Studies and diagram generation run in a process pool with per-job CPU time and memory limits and cancellation.
- The main process keeps the change stream, keepalive and publication responsive; a stuck job is killed and recorded as `failed`.

## 11. Operator and engineer interfaces

- Phase 2: CLI tools (`tools/`) for import, validation report, binding proposal (CSV out/in), replay.
- Phase 3: Admin UI additions:
  - `CIM-EMS` in the driver name lists (`ProtocolDriverInstancesTab.vue`, `ProtocolConnectionsTab.vue`) and the connection fields of 4.2.
  - Model page: upload, validation report, activate, rollback.
  - Binding review page: proposals, accept/edit/reject, coverage report.
  - Run history: last runs with status and summary, links to artifacts.
- On-demand requests (rerun LF/SE/CA, export, regenerate diagrams) are inserted into a `cimStudyRequests` collection by the Admin UI backend; the driver watches it. This keeps `commandsQueue` free of non-device semantics.
- Phase 4: estimator view: measurement residuals and the bad data list per run (operators trust the estimator when they can see why a point was rejected).

## 12. Testing

### 12.1 Datasets

- Public: ENTSO-E CGMES conformity test configurations (MicroGrid BE/NL are bundled with pypowsybl; MiniGrid, SmallGrid and RealGrid where licensing allows), IEEE cases from pypowsybl.
- User: one sanitized representative model with switches, tap-changing transformers, loads, generators and shunts, kept outside the repository.
- Recorded telemetry: exported from the TimescaleDB `hist` table for the bound points over representative periods (normal operation, switching events, a disturbance), replayed by `tools/replay.py` into a test database.

### 12.2 Tolerances (fixed before evaluation, review per project)

| Check | Tolerance |
|---|---|
| LF voltage magnitude vs reference | 0.001 pu |
| LF voltage angle vs reference | 0.05 deg |
| LF branch P/Q vs reference | 0.5 % of rating or 0.5 MW/Mvar |
| Engine-to-engine LF comparison | same as above |
| SE on synthetic noise-free measurements vs LF truth | 0.001 pu, 0.1 deg |
| Round-trip export/import solved values | same as LF reference |

### 12.3 Test layers

- Unit: address parsing, tag naming, sign handling, snapshot rules, switch rules table, run monotonicity, publication invalidation.
- Integration (MongoDB replica set): change stream cache, redundancy failover (two driver instances, kill active, check single publisher), publication through `cs_data_processor` to alarms and historian.
- Engineering: LF reference comparisons, engine comparison, SE cases (observable, unobservable, noisy, missing, single and multiple bad data, wrong switch status), CA classification, round trip.
- Reproducibility: rerun a stored `cimRuns` tuple offline and match its results exactly.
- Performance: at the expected network size and contingency count; record LF, SE and CA times per cycle on target hardware.

## 13. Phases and exit criteria

### Phase 0: feasibility spike

- Throwaway scripts under `src/cim-ems/spike/` (not shipped).
- pypowsybl: import MicroGrid BE and one user model; LF; SLD and NAD SVG with metadata; apply switch states from a sample CSV; export SV; re-import.
- Check CGMES measurement import on the user model.
- pandapower: import the same package; LF; compare with `compare_engines.py` prototype.
- Check pypowsybl and pandapower wheels on Linux x86_64, Linux ARM64 and Windows x64.
- Exit: written go/no-go per item, chosen CGMES version, chosen network transfer option candidate, list of unsupported features in the user model.

### Phase 1: driver shell and offline model exchange

- Component skeleton, `AGENTS.md`, `README.md`, pinned dependencies.
- Config, redundancy, collections and indexes, GridFS.
- Model import, validation report, activation, rollback (CLI).
- Base-case LF against package SV.
- Export and re-import.
- Exit: gate 1 tolerances met on public cases and the user model; round-trip check passes; redundancy unit tests pass.

### Phase 2: read-only telemetry and topology

- Binding proposal and CSV review flow; binding revisions.
- Change stream cache, snapshot policy, switch and tap rules, island detection, topology hash.
- Replay tool.
- Exit: replayed switching events produce the expected topology at every step; rejected snapshots carry correct reasons; deterministic topology hash.

### Phase 3: load flow cycle, publication and displays

- Cyclic LF, result points with `autoCreateTags`, invalidation, monotonic runs.
- Diagram generation and markup conversion, layout overrides.
- Admin UI driver registration, model and binding pages, run history.
- Packaging into Docker (`Dockerfile`, `compile-docker`, `demo-docker` service set kept in step).
- Exit: live displays show measured and LF values with correct quality; alarms and historian work for result points; failover leaves exactly one publisher; restart recovers without stale overwrites.

### Phase 4: state estimator qualification

- Adapter, pandapower implementation, network transfer option fixed by engine comparison.
- Pseudo and virtual measurements, bad data processing, observability.
- Residual and bad data view.
- Exit: all SE engineering cases pass with correct verdicts (not only convergence); replayed real telemetry gives stable estimates and plausible bad data flags reviewed by an engineer.

### Phase 5: contingency analysis

- Contingency lists, CA after converged base case, classification, summary points, skip policy.
- Exit: classification correct on constructed cases (violation, islanding, divergence); CA cycle time within the target on the user model.

### Phase 6: operational hardening and installers

- Sustained run (at least one week on replayed or live data) with memory and latency tracking.
- Model rollback drill, failover drill, backup/restore of new collections and GridFS.
- Linux installers (`platform-ubuntu-2404`, `platform-ubuntu-2604`, `platform-rhel9`, `platform-rhel10`): venv or bundled Python, supervisor ini `cim-ems.ini` (`autostart=false`, same layout as `platform-ubuntu-2404/plc4jclient.ini`).
- Windows: embedded Python distribution, service creation in `platform-windows` scripts and `json-scada.nsi`.
- Exit: install, upgrade and uninstall tested on each platform; documentation complete.

## 14. Repository touch points

| Area | Change |
|---|---|
| `src/cim-ems/` | New component, AGENTS.md, README.md |
| `src/AGENTS.md`, root `AGENTS.md` | Child DOX Index entries for `src/cim-ems/` |
| `README.md` (and generated `index.md`) | Driver list entry; run `node docs/sync-index.mjs` |
| `docs/schema.md` | `CIM-EMS` connection fields, new collections, generated tag name row |
| `docs/DEVELOPER_GUIDE.md` | Only if the Python driver pattern is worth documenting as a template |
| Architecture diagrams in `docs/` | Add the CIM-EMS driver |
| `src/AdminUI` | Driver name lists, connection fields, model/binding/run pages, i18n strings |
| `src/server_realtime_auth` | Backend endpoints for model upload, binding review, study requests, artifact download, with role checks |
| `src/mcp-json-scada-db` | `CIM-EMS` in `ProtocolDriverName` (`src/jsonscada/types.ts`) and driver catalog (`src/jsonscada/drivers.ts`); no model content exposed |
| `mongo_seed/` | Collections and indexes for `cimModels`, `cimMeasurementBindings`, `cimRuns`, `cimContingencyLists`, `cimStudyRequests`, GridFS bucket |
| `Dockerfile`, `compile-docker/`, `demo-docker/` | Python stage, service, demo connection with MicroGrid model |
| `platform-*` | Installers and supervisor/Windows service definitions (phase 6) |
| `svg/` | Generated demo display for the MicroGrid model (optional) |

## 15. DMS follow-up (separate plan)

Reused: driver shell, redundancy, bindings, snapshot and topology, publication, diagrams. New: phase-level model (CGMES or other distribution formats), unbalanced load flow and state estimation (Power Grid Model is the primary candidate), sparse measurement handling, DER models, feeder diagrams. Its acceptance dataset must exercise phase connectivity, unbalance, transformer connections, regulators and DER.

## 16. Security and confidentiality

- Network models and study results are treated as critical infrastructure information.
- Upload, activation, binding approval and export require dedicated Admin UI rights; every such action is written to the existing user actions log.
- Artifacts are not served without authentication; exports are downloadable only by authorized roles.
- The MCP server exposes driver status and result points only, never model packages or exports.
- No customer model, binding list or telemetry is committed to the repository; tests use public cases.

## 17. Packaging and licensing

- Pin exact versions of pypowsybl, pandapower, numpy, scipy, pymongo in `requirements.lock`; record engine versions per run.
- pypowsybl ships native wheels with an embedded runtime, no separate JVM; confirm platform coverage in phase 0.
- Licenses: JSON-SCADA GPL-3.0; PowSyBl/pypowsybl MPL-2.0; pandapower BSD-3-Clause; Power Grid Model MPL-2.0. All are compatible with distribution under GPL-3.0. Check any additional engine (for example VeraGrid) before adoption and list third-party licenses in the component README.

## 18. Risks

| Risk | Mitigation |
|---|---|
| User model uses features PowSyBl imports with approximations | Validation report with reject/approximate per class; phase 0 inventory |
| Importer differences between engines look like estimator errors | Engine comparison gate before any SE result is trusted |
| Poor switch/tap binding coverage gives wrong topology | Coverage report; never-assume-closed rule; `topologyUncertain` flag |
| Measurement time skew | Snapshot policy with skew limit; rejected snapshots are visible |
| Study load affects realtime services | Separate worker processes with limits; skip policy |
| Two publishers after failover | Active check before every write; failover integration test |
| Python packaging on Windows | Docker first; embedded Python in phase 6 |
| Engine API changes | Pinned versions; reproducibility tests on upgrade |

## 19. Open questions

1. ~~CGMES version~~: decided, 2.4.15 (CIM16), see section 2. Still open: which boundary set do the exchange partners require, and does any partner need CGMES 3.0?
2. Expected network size (buses, switches, contingencies) and target cycle times for LF, SE and CA?
3. Which representative user model and telemetry period can be used for qualification?
4. Is the SE result needed for downstream exchange (IGM/SV export to a TSO or RSC) in the first release?
5. Which roles in the existing Admin UI should own model activation and binding approval?

## 20. Phase 0 results (2026-11-07)

Environment: Python 3.14.9, pypowsybl 1.16.1, pandapower 3.5.6, pymongo 4.19.0, Linux x86_64. Scripts: `spike/01` to `spike/04` (throwaway, not shipped).

| Item | Result | Verdict |
|---|---|---|
| Install | pypowsybl, pandapower, pymongo install in a plain venv; pandapower CIM import also needs `lxml` and `xmltodict` | go |
| Wheels | pypowsybl 1.16.1: Linux x86_64, Windows x64, macOS (cp313, cp314). **No Linux ARM64 wheel.** pandapower is pure Python | ARM64 unsupported for now (Docker and installers x86_64 only) |
| License | pypowsybl MPL-2.0; pandapower declares none in PyPI metadata, check its repository license file before packaging | verify |
| Import | `create_micro_grid_be_network` converges (3 iterations). Default CGMES import is **bus-branch**; node-breaker needs the import parameter set explicitly. MicroGrid raw files are not shipped in the wheel | go, take raw files from ENTSO-E |
| Switches and topology | On the node-breaker test network, `update_switches` plus bus-view recalculation works (opening one breaker changed the bus-view bus count) and LF reconverged | go |
| Tap changer update | `update_ratio_tap_changers` works | go |
| Diagrams | Single-line diagram SVG plus JSON metadata (component types, anchor points, style classes), network-area diagram SVG | go |
| LF speed | ~10 ms on the small test network | not representative; benchmark on user model |
| CGMES export | Profiles EQ, SSH, SV, TP exported and re-imported with the same counts of switches, lines, transformers, generators, shunts, converters | go with caveat below |
| **Round-trip loss** | Active power matches (1e-10). **HVDC VSC converter reactive setpoint is lost (500 to 0) and sign flipped (120 to -120)**, causing a 240 Mvar difference on a connected line | confirmed limitation: round-trip check must compare solved Q and flag HVDC converters as unsupported |
| Measurements | pypowsybl exposes `measurements` and `discreteMeasurements` extensions; not tested on a real CGMES file with Analog/Discrete objects | open, test on the user model |
| pandapower CIM import | Imports exported IEEE 14 and 57 (EQ, SSH, SV, TP). **No slack/ext_grid is created**: adapter must set a slack generator explicitly. 3-winding transformers with zero magnetizing branch crash `runpp` (division by zero) even with placeholder values | adapter needs model fixups; reject or convert 3-winding transformers in the SE path |
| Engine comparison | IEEE 14 after fixing the slack: voltage magnitudes equal at the 3 decimal places printed, angle differences 0.0025 to 0.04 deg (near the 0.05 deg tolerance). IEEE 57 not comparable: pandapower slack differs from pypowsybl slack | go, but tolerances are tight; the adapter must copy slack, voltage controls and Q limits from the pypowsybl network, not rely on import |

Decisions taken from the results:

- The SE network transfer is option B (build the pandapower net from the solved pypowsybl network through an explicit mapping), not the pandapower CIM import: option A drops the slack and fails on 3-winding transformers. Keep option A only as a cross-check in `compare_engines.py`.
- The validation report must list HVDC and VSC converters, 3-winding transformers and similar classes as `approximate` or `unsupported` per engine.
- Phase 1 targets Python 3.13 or 3.14 on Linux x86_64 and Windows x64. ARM64 waits for a pypowsybl wheel; revisit at phase 6.

CGMES version check (2026-11-07): this pypowsybl exports both `cim-version=16` (CIM16 namespace) and `100` (CIM100 namespace) with EQ, SSH, SV and TP; both re-import with identical equipment counts and a converged load flow on the four-substations test network.

Round-trip fixtures (phase 1): the Eurostag tutorial network round-trips with differences near 1e-12 MW and Mvar. IEEE 14 does not, because pypowsybl's test case has lines between voltage levels of different nominal voltage, which CGMES cannot express. MicroGrid BE exports but cannot be re-imported without its boundary set (`nominalVoltage not found for BaseVoltage`), so boundary datasets are mandatory input for models with boundary lines.

Public conformity models (2026-11-07, `tools/fetch_test_models.py`, from powsybl-core test resources, CGMES 2.4.15 with boundary sets):

| Model | Result |
|---|---|
| SmallGrid NodeBreaker | 105 substations, 115 voltage levels, 173 lines, 10 transformers, 1,107 switches (369 breakers), 19 generators, 103 loads. Import about 1 s, base case load flow converges in 4 iterations (about 14 ms), single synchronous component, 588 operational limits. Round trip with the boundary files re-attached: differences 0.0 MW and Mvar. Opening one breaker re-solves in about 8 ms; 19 of 20 random single-breaker outages converge. SLD in about 40 ms (19 kB SVG), NAD in about 100 ms (229 kB SVG) |
| MicroGrid BaseCase (assembled) | 10 dangling lines and one 3-winding transformer flagged; load flow converges in 7 iterations; round trip within 0.16 MW and 0.02 Mvar |

Consequences: the round-trip check must re-attach the boundary set of the source package (done in `cim_ems/export.py`); neither conformity model contains CGMES Analog/Discrete objects (the pypowsybl `measurements` and `discreteMeasurements` extensions are empty), so binding proposals from CGMES measurements stay untested until the user model is available and name matching is the only proposal method testable on public data. The small-network load flow figures say nothing about the user model size: benchmark there.

Still open from phase 0 (to close during phase 1, they need the user model): CGMES measurement import on a model that contains measurements, unsupported-feature inventory of the user model, and the questions in section 19.

## 21. Phase 2 progress (2026-11-07)

Implemented in `cim_ems/bindings`, `snapshot.py`, `topology.py`, `synthetic.py`, `tools/replay.py`; tested on SmallGrid NodeBreaker (1,107 switches).

| Item | Status |
|---|---|
| Binding model and CSV review format | done |
| Name-match proposals for switches | done. Exact synthetic tags match 1,107 of 1,107 uniquely. Needed whole-word, in-order matching (plain token overlap confused `BREAKER132` with `BREAKER339`, and `bbs1` with `bbs2`) and global best-score assignment (a switch whose own tag is missing no longer takes a neighbour's tag) |
| Tap and analog proposals | not done: no public model has measurement objects; tap proposals follow the same matcher once tap tags exist in a real tag list |
| Snapshot policy | done: valid, transient (hold, flag), invalid (hold up to `max_hold_seconds`, then reject), no history (model state, flag), analog age, invalidity, skew against the newest-biased median, sign, substituted flag |
| Topology processor | done: switch and tap updates, bus and island counts, deterministic hash that returns to its original value after reverting switches |
| Replay | done: deterministic hash sequence for the same events; load flow status per step |
| Coverage report | per voltage level in `Proposal.coverage`; Admin UI view is phase 3 |
| Change stream cache from `realtimeData` | not done (next) |

Still to do for the phase 2 exit gate: the live `realtimeData` cache (change stream with full resync), replay from historian data, and a run on the user model with real tags.

## 22. ReliCapGrid results (2026-11-07)

Source: ENTSO-E ReliCapGrid, branch `cgmes-3.0_ncp-2.5_tc-2.1` (CGMES 3.0, Network Code profiles, nine fictional realms). Fetched with `tools/fetch_test_models.py --relicap`, one package per region with all boundary files. This exercises the opt-in CGMES 3.0 path; the default stays 2.4.15.

| Region | Result |
|---|---|
| Belgovia | 16 voltage levels, 20 switches, one 3-winding transformer, one SVC. Converges (6 iterations), exact round trip |
| Galia | 12 voltage levels, 15 switches. Converges, exact round trip |
| Portheim | 3 voltage levels, 52 switches. Converges, exact round trip |
| Svedala | 99 voltage levels, 1,470 switches, 73 loads, 6 synchronous components. Import about 1 s, converges (28 iterations); only the main component is calculated. 433 switches are open in the base state. Exact round trip |
| Espheim | 129 voltage levels, 1,296 switches, 104 loads. Converges (5 iterations). Exact round trip |
| DC-Nordheim-Galia | 4 VSC converters, 2 HVDC lines. Converges, round trip exact |
| Britheim | VSC HVDC link, 2 components. Converges; round trip loses 59.8 Mvar (VSC reactive setpoint, as in phase 0) and is correctly reported as failed |
| DC-Espheim-Svedala | classic (LCC) HVDC only: load flow returns NO_CALCULATION; reported as unsupported |
| Nordheim | imports but has no calculable component; reported as an error instead of crashing |
| Jotunheim | no equipment (EQ) file in the folder; not a stand-alone model |

Findings:

- **Common data file:** pypowsybl 1.16.1 cannot import a region when BaseVoltage and region definitions live in a separate CoreEquipment-EU "common data" file; it reports `VoltageLevel not found` or `nominalVoltage not found for BaseVoltage`. Merging those objects into each EQ file (done by the fetch tool) makes the import work. A model exchange partner who delivers this layout needs the same preprocessing; the validation report should explain the symptom when the driver meets it.
- **SvInjection on re-import:** the exporter writes an SvInjection for the slack mismatch. The importer converts it into a load without a node, which fails on node-breaker models (`Load ...: node is not set`). The round trip now re-imports with `iidm.import.cgmes.convert-sv-injections=false`. For real imports of models whose SV contains SvInjections on node-breaker networks, check this parameter; converting them would change the load flow.
- **Extra switches on re-import:** the re-imported network can contain a few extra switches (Espheim 1,298 against 1,296, Svedala 1,515 against 1,470). Flows are identical; the cause is not yet explained.
- **Switch names:** Svedala names like `FT63_G1-S` and `FT63_G1-S(1)` need exact-over-suffix ranking in the matcher; Espheim has distinct switches with identical names, which stay ambiguous on purpose (21 of 1,296). Real tag lists will need their own review.
- **Islands:** a model with several synchronous components solves only the main one; the report shows the island count but results for the others are missing until a slack is defined there.
- **Scale:** the largest models (about 1,300 to 1,500 switches, about 130 voltage levels) import in about 1 s and solve in tens of milliseconds, comparable to SmallGrid.

## 23. Phase 3 progress (2026-11-07)

Implemented: `cache.py`, `cycle.py`, `results.py`, `publish.py`, `bindings/store.py`, the driver loop in `__main__.py`.

| Phase 3 exit item | Status |
|---|---|
| Cyclic load flow with result points, `autoCreateTags`, monotonic runs | done. Verified with the real driver process on a MongoDB replica set |
| Live follow of a breaker in SCADA data | done: changing a bound tag triggers a `topology` run within about a second; the result point of the opened line is published as invalid (not calculated) |
| Alarms and historian path | alarms verified with the real `cs_data_processor`: a high limit set on a result point raises `alarmRange = 1` and the processed `value`, quality and time tags are written. Historian output not checked |
| Failover with exactly one publisher | done: two nodes, only the active one publishes; after killing it the standby takes over (keep-alive timeout shortened to 4 s in the test) and runs never interleave afterwards |
| Restart recovery without stale overwrites | partly: run ids are time-based and the publisher refuses non-increasing ids per kind; points written by a previous process are reused. A restart test with a clock step back is not written |
| Displays from generated SVG | partly: `diagrams/sld.py` converts PowSyBl single-line diagrams into svg-scada markup (switch open/closed drawings from the bound point state, feeder P/Q/I `get` text from LF result tags, busbar V/angle tooltips); `tools/gen_displays.py` writes one SVG per voltage level. Tested on the four-substations network (XML and every label parse, bound tags exist) and on Svedala (99 displays in 2 s, 8 MB, all 1,470 switches bound from proposed bindings). Files are named `cim_<voltage level name>_<id prefix>.svg`. Not yet opened in the JSON-SCADA viewer; operator layout overrides and screen list `CIM` group not done |
| Admin UI pages (connection fields, models, bindings, run history) | not started; only the driver name lists were registered |
| Docker/compile-docker/installer packaging | not started |

Design points settled in code:

- The first run after start is labelled `start`, topology-triggered runs `topology` (debounced to one per second), timer runs `cycle`.
- `invalidDetectTimeout` of every result point equals `resultMaxAgeSeconds` and all points are rewritten each run, so a stopped driver makes the data processor mark its results invalid without extra logic. The driver also calls `invalidate` for load flow points when no run succeeded within that time.
- `SYS` points (STATUS code 0 converged, 1 diverged, 2 rejected snapshot, 3 unsupported, 4 failed; CONVERGED; ITER; DURATION in ms; UNCERTAIN for held/transient switch states) are published after every run.
- Results for de-energized or disconnected ends are "not calculated" (invalid at source), never stale numbers.
- Only load flow is wired. State estimation and contingency analysis (phases 4 and 5) will use the same cycle and publisher.
