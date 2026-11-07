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
activate a model, save approved bindings, then start the driver like the others:

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
the load flow once per topology and refused beyond 2e-3 pu or 0.1 degrees.

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
