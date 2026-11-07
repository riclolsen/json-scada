# CIM-EMS driver

Python protocol driver (`CIM-EMS`) that brings CIM/CGMES model exchange and EMS network analysis to JSON-SCADA.
Status: load flow cycle with result publication works end to end (phases 1 to 3 core). The design is in `CIM_EMS_PLAN.md`.

## Offline model check

```
pip install -e .[test]
python tools/import_model.py model.zip --round-trip
python -m pytest
```

The tool imports a CGMES 2.4.15 package (node-breaker topology kept), prints a JSON validation report (counts,
unsupported classes, islands, base case load flow) and, with `--round-trip`, exports and re-imports the model and
compares solved values. Exit code 1 means the model is rejected.

## Running the driver

Register an instance and a connection (Admin UI support for the connection fields is still to do), import and
activate a model, save approved bindings, then start the driver like the others.

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
are listed in the run record (`cimRuns`, `summary.estimation.badData`). The network transfer is checked against
the load flow once per topology and refused beyond 1e-4 pu or 0.01 degrees.

Contingency analysis is off by default. With `runContingency: true` the pypowsybl AC security analysis runs after a
converged load flow on the list `contingencyListId` (`AUTO_N1` for every in-service line and two-winding
transformer from `contingencyMinKv`, or a `cimContingencyLists` document
`{"listId": "L1", "contingencies": [{"id": "c1", "name": "Line 1", "elements": ["<mRID>", ...]}]}`).
It runs when the topology changed (at least `contingencyMinGapSeconds`, default 10, after the last run) or every
`contingencyIntervalSeconds` (default 300); other cycles are skipped and counted, and the last results are
republished so their points do not expire. Points: `<connection>;CA;<contingency id>;STATUS|VIOL|WORST` for the first
`contingencyPublishLimit` (200) entries, `<connection>;CA;SYSTEM;VIOL|WORST|INSECURE|COUNT`, and
`<connection>;SYS;CA;STATUS|SKIPPED|DURATION`. STATUS: 0 secure, 1 violation, 2 islanding, 3 diverged, 4 unsupported.
Violations already present in the base case are not counted. Full results go to the `cimArtifacts` GridFS bucket
(`ca_<runId>.json`, referenced by `summary.contingency.artifactId` in `cimRuns`).

## Public test models

```
python tools/fetch_test_models.py
python tools/import_model.py tests/data/conformity/smallgrid_nodebreaker.zip --round-trip
```

Downloads the ENTSO-E SmallGrid (node-breaker, 1,107 switches) and MicroGrid conformity models distributed with PowSyBl. With `--relicap` it also builds one CGMES 3.0 package per region of ENTSO-E ReliCapGrid (Svedala: 1,470 switches; Espheim: 1,296). They are not committed.

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
