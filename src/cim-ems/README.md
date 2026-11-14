# CIM-EMS driver

Python protocol driver (`CIM-EMS`) that brings CIM/CGMES model exchange and EMS network analysis to JSON-SCADA.
Status: load flow, state estimation and contingency analysis run in the driver cycle and publish results (phases 1
to 5 core, tested on public models). The design and progress are in `CIM_EMS_PLAN.md`.

## Installation

Linux x86_64 and Windows x64 only: pypowsybl publishes no Linux ARM64 wheel. Python 3.10 to 3.14.

```bash
./install.sh      # Linux: creates .venv here with the pinned dependencies (skips on non-x86_64)
install.bat       # Windows: installs into platform-windows\python-runtime, else creates .venv (py launcher or PATH)
```

`platform-linux/build.sh` runs `install.sh`, the Ubuntu and RHEL installers add the Python packages
(`python3-venv`; RHEL 9 `python3.12`) and copy `cim_ems.ini`, and the Docker image builds the environment on
amd64. The supervisor program `cim_ems` is `autostart=false`: enable it after a model and bindings are in place.
On Windows the installer ships `platform-windows\python-runtime` (Python 3.14 with this package installed by
`build.bat`, see `docs/build_windows.md`) and registers the service `JSON_SCADA_cim_ems` with manual start; set it
to automatic once a model and bindings are in place. The Admin UI process manager starts and stops the driver like
the others (instances above 1 get `cim_ems_<n>` / `JSON_SCADA_cim_ems_<n>`); it uses `.venv` when present, else the
bundled runtime.

## Offline model check

```
.venv/bin/python -m pip install -e .[test]   # development: editable install with the test tools
python tools/import_model.py model.zip --round-trip
python -m pytest
```

The tool imports a CGMES package (2.4.15; 3.0 is detected from the namespace and recorded as `cgmesVersion`) (node-breaker topology kept), prints a JSON validation report (counts,
unsupported classes, islands, base case load flow) and, with `--round-trip`, exports and re-imports the model and
compares solved values. Exit code 1 means the model is rejected.

## Running the driver

Register an instance and a connection in the Admin UI (driver `CIM-EMS`; the connection dialog has a CIM-EMS
section for the model, cycle, load flow, publication, estimation and contingency settings), import and activate a
model, save approved bindings, then start the driver like the others.

## Svedala demo

The demo database (Docker demo, Linux x86_64 and Windows installers) holds a ready-to-run case built from the
Svedala region of ENTSO-E [ReliCapGrid](https://github.com/entsoe/ReliCapGrid) (58 substations, 1,470 switches,
CGMES 3.0):

- model `SVEDALA` revision 1 (active; the original package is in the `cimArtifacts` bucket) and an approved binding
  revision with 1,470 switch and 698 measurement bindings;
- connection `SVEDALA_EMS` (number 90, driver `CIM-EMS` instance 1): load flow and state estimation every 30 s, N-1
  of the branches from 100 kV every 5 minutes or on a topology change; its result points are already in the seed;
- connection `SVEDALA_FIELD` (number 91, driver `UNDEFINED`): the field points (switch states, V/P/Q
  measurements, 439 breaker commands), written by the simulator below;
- displays `svedala-overview.svg` (network-area diagram: live MW at the branch ends, kV at the buses, links to the
  substations) and one `svedala_<substation>.svg` single-line diagram per substation.

Run the driver and the field simulator (both manual start; Docker demo: services `cim_ems` and
`cim_ems_demo_field`, Linux supervisor programs `cim_ems` and `cim_ems_demo_field`, Windows services
`JSON_SCADA_cim_ems` and `JSON_SCADA_cim_ems_demo_field`):

```
.venv/bin/python -m cim_ems 1 1 ../../conf/json-scada.json
.venv/bin/python -m cim_ems.demo_field ../../conf/json-scada.json [--period 5]
```

The simulator solves its own copy of the network under a slow load curve with noise, writes the measurements and
switch states like a protocol driver, and executes breaker commands sent to the `SVEDALA_FIELD` points (1 closes,
0 opens). `demo_simul` leaves the points of `CIM-EMS` and `UNDEFINED` connections alone.

The seed and the displays are generated, never edited by hand:

```
python tools/fetch_test_models.py --relicap      # relicap_Svedala.zip into tests/data/conformity
.venv/bin/python -m pip install .[test]          # mongomock
python tools/build_demo.py                       # demo-docker/mongo_seed/files/demo_cim_*.json and svg/svedala*.svg
```

The build runs the driver code (import, validation, activation, binding revision, one study cycle) in an in-memory
database and writes the result with fixed dates and ids, so an unchanged package gives unchanged files. It also
replaces the Svedala lines of `demo_instances.json` and `demo_connections*.json`.

Attribution: the model is ReliCapGrid by ENTSO-E, Svedala region by Svenska kraftnät, licensed under
[CC BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/). Changes: the common data is merged into the EQ
profile. The seed files and the displays derived from it are shared under the same license; every display carries
this attribution.

## Admin UI

The **CIM-EMS** tab of the Admin page has four views:

- **Models**: upload a CGMES zip (2.4.15, or 3.0 such as ReliCapGrid) as a new revision of a model ID, read its validation report (errors,
  warnings, unsupported classes, base case load flow), activate a revision, roll back, download the stored package.
- **Bindings**: show a binding revision, propose bindings from the current supervised tags, approve, edit (tag,
  sigma) or delete entries, and save the list as a new binding revision. Coverage chips show approved/total per kind.
- **Runs**: the latest `cimRuns` with load flow, estimation (bad data with normalized residuals) and contingency
  outcome (insecure contingencies, result download); **Run now** requests an immediate study for a connection.
- **Requests**: the `cimStudyRequests` queue with results and errors.

Every view needs an Admin role. Upload, activation, rollback, binding proposals and saves, and artifact downloads
also need a role with **CIM-EMS Engineering** (`cimEngineering`, off by default, set it in the Roles tab). Every
request and download is written to the user actions log. The web server only stores the upload and queues the
request; the active CIM-EMS driver node validates, imports and saves, so the instance must be running.
Activating a revision or saving bindings rebuilds the affected connections at once. Model packages and study
results are never served by `/GetFile`.

The MCP server ([`cim_ems_*` tools](../mcp-json-scada-db/README.md#cim-ems-tools)) uses the same backend and rights; for AI-assisted binding it also queues `listElements` requests, which store an element catalog (every switch, tap changer and P/Q/V slot with names, nominal kV and the matcher label) as a `cimArtifacts` file. The command line tools below do the same against MongoDB directly.

Bindings are proposed by name matching, reviewed as CSV and saved as a revision:

```bash
python tools/bindings_admin.py propose model.zip --out bindings.csv --report report.json   # tags from MongoDB
python tools/bindings_admin.py save <modelId> bindings.csv                                  # after setting approved
python tools/bindings_admin.py export <modelId> --out current.csv
```

Switch tags are matched to `<substation> <voltage level> <switch>`. Analog tags are matched by quantity (from the
unit kV/MW/Mvar, else the last P/Q/V/MW/MVAR/KV word of the tag, else the description) to
`<substation> <branch>` for each branch end (`<substation> <voltage level> <branch>` when both ends are in one
substation) and to `<substation> <voltage level>` for voltages, bound to the first busbar section of the voltage
level (or its first bus in a bus-breaker model). Elements with identical labels, such as parallel circuits with the
same name, are reported as ambiguous and never guessed. Points published by a CIM-EMS connection are never
candidates, `save` refuses them and the driver ignores them.


```
python -m cim_ems <instance> <logLevel> <path to json-scada.json>
```

Minimal `protocolConnections` document: `protocolDriver: "CIM-EMS"`, `protocolDriverInstanceNumber`,
`protocolConnectionNumber`, `name`, `description`, `enabled: true`, `commandsEnabled: false`, `cimModelId`, and
optionally `cimBindingRevision`, `cycleSeconds` (default 60), `resultMaxAgeSeconds` (default 180),
`autoCreateTags` (default true), `loadFlowParameters` and `publishFilter` (see the plan, section 4.2).
Results appear as supervised points `<connection>;LF;<name>;<quantity>[;<side>]`, with the system health points
`<connection>;SYS;LF;STATUS` and others.

State estimation is off by default. With `runStateEstimation: true` (and `estimatorParameters.badDataThreshold`,
default 4) every converged load flow is followed by a weighted least squares estimate from the approved analog
bindings (`V` on a bus or busbar section in kV, `P`/`Q` on a line or transformer end in MW/Mvar). Results are
`<connection>;SE;<name>;<quantity>[;<side>]` and `<connection>;SYS;SE;STATUS|CONVERGED|BADDATA|MEAS|CHI2`
(STATUS 0 converged, 1 not observable, 2 not converged, 3 unsupported network, 4 failed, 5 no load flow). Bad data
and the largest residuals are listed with their tags in the run record (`cimRuns`, `summary.estimation.badData` and
`largestResiduals`; a removed measurement is compared with the final estimate made without it). The full residual
report of the latest estimate (every measurement: tag, measured, estimated, sigma, normalized residual, and the
analogs dropped from the snapshot) is a `cimArtifacts` file (`kind: "se-residuals"`, one per connection). Every
converged run also records the system balance in `summary.totals` (generation, load, losses in MW). The network transfer is checked against
the load flow once per topology and refused beyond 1e-4 pu or 0.01 degrees.

Contingency analysis is off by default. With `runContingency: true` the pypowsybl AC security analysis runs after a
converged load flow on the list `contingencyListId` (`AUTO_N1` for every in-service line and two-winding
transformer from `contingencyMinKv`, or a `cimContingencyLists` document
`{"listId": "L1", "contingencies": [{"id": "c1", "name": "Line 1", "elements": ["<mRID>", ...]}]}`).
It runs when the topology changed (at least `contingencyMinGapSeconds`, default 10, after the last run) or every
`contingencyIntervalSeconds` (default 300); other cycles are skipped and counted, and the last results are
republished so their points do not expire. Points: `<connection>;CA;<contingency id>;STATUS|VIOL|WORST` for the first
`contingencyPublishLimit` (200) entries (`contingencyThreads`, default 1, sets the OpenLoadFlow thread count;
more threads are faster but not bit-reproducible near limits), `<connection>;CA;SYSTEM;VIOL|WORST|INSECURE|COUNT`, and
`<connection>;SYS;CA;STATUS|SKIPPED|DURATION`. STATUS: 0 secure, 1 violation, 2 islanding, 3 diverged, 4 unsupported.
Violations already present in the base case are not counted. Full results go to the `cimArtifacts` GridFS bucket
(`ca_<runId>.json`, referenced by `summary.contingency.artifactId` in `cimRuns`).

## Public test models

```
python tools/fetch_test_models.py
python tools/import_model.py tests/data/conformity/smallgrid_nodebreaker.zip --round-trip
```

Downloads the ENTSO-E SmallGrid (node-breaker, 1,107 switches) and MicroGrid conformity models distributed with PowSyBl. With `--relicap` it also builds one CGMES 3.0 package per region of ENTSO-E ReliCapGrid (Svedala: 1,470 switches; Espheim: 1,296). They are not committed as test data; the Svedala package is part of the demo seed (see [Svedala demo](#svedala-demo)).

## Replay

```
python tools/replay.py tests/data/conformity/smallgrid_nodebreaker.zip --synthetic 10
python tools/replay.py model.zip bindings.csv events.csv
python tools/gen_displays.py model.zip bindings.csv --connection CIM-EMS1 --out ../../svg/cim
```

Generates (or reads) switching events, applies the snapshot policy and topology processor, and prints the topology
hash, bus and island counts and load flow status after each event. `bindings.csv` is the review format of
`cim_ems.bindings`: proposals come with `approved=false` and a confidence; only approved bindings are used.

## Model revisions

```
python tools/model_admin.py import model.zip TSO_MODEL
python tools/model_admin.py activate TSO_MODEL 1
python tools/model_admin.py rollback TSO_MODEL
```

Imports are immutable revisions with the original package kept in GridFS. A rejected revision cannot be activated.

## Third-party engines

- pypowsybl (MPL-2.0): CGMES import/export, load flow, diagrams
- pandapower: state estimation (check its license file before packaging)
