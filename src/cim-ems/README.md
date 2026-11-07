# CIM-EMS driver

Python protocol driver (`CIM-EMS`) that brings CIM/CGMES model exchange and EMS network analysis to JSON-SCADA.
Status: phase 1 in progress (offline model exchange). The design is in `CIM_EMS_PLAN.md`.

## Offline model check

```
pip install -e .[test]
python tools/import_model.py model.zip --round-trip
python -m pytest
```

The tool imports a CGMES 2.4.15 package (node-breaker topology kept), prints a JSON validation report (counts,
unsupported classes, islands, base case load flow) and, with `--round-trip`, exports and re-imports the model and
compares solved values. Exit code 1 means the model is rejected.

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
