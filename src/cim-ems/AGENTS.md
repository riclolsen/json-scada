# DOX: src/cim-ems — CIM/CGMES EMS Driver (Python)

## Purpose

`CIM-EMS` protocol driver: CGMES 2.4.15 model import/export and validation, operating-state assembly from
JSON-SCADA telemetry, load flow, state estimation and contingency analysis, results published as ordinary
supervised points. `CIM_EMS_PLAN.md` is the working plan (phases, data contracts, acceptance gates); delete it
once its content has moved into this README and `docs/schema.md`.

## Ownership

- cim-ems owns the Python package `cim_ems`, its tools, tests and the throwaway phase 0 `spike/` scripts
- Engines: `pypowsybl` (CGMES, load flow, security analysis, diagrams), `pandapower` (state estimator adapter)
- Collections `cimModels`, `cimMeasurementBindings`, `cimRuns`, `cimContingencyLists`, `cimStudyRequests` and the
  `cimArtifacts` GridFS bucket belong to this driver (documented in the plan, to move to `docs/schema.md`)

## Local Contracts

- **Language:** Python 3.13 or 3.14, versions pinned in `pyproject.toml`; Linux x86_64 and Windows x64 only
  (no pypowsybl ARM64 wheel)
- **CGMES version:** 2.4.15 (CIM16) only; export with `iidm.export.cgmes.cim-version=16`. CGMES 3.0 is opt-in later
- **CLI:** `python -m cim_ems <instance> <logLevel> <configFile>`, like the other drivers
- **Files:**
  - `cim_ems/config.py` — `json-scada.json` and CLI args
  - `cim_ems/models/importer.py` — pinned import parameters, validation report, unsupported-class list
  - `cim_ems/export.py` — CGMES export and the round-trip check (compares solved values, never XML bytes)
  - `cim_ems/__main__.py` — driver entry: config, instance check, keep-alive loop (study cycle comes later)
  - `cim_ems/mongo.py` — collection names, indexes (one active revision per model, TTL on runs), GridFS bucket
  - `cim_ems/redundancy.py` — active-node arbitration; `still_active()` must be checked right before every publication
  - `cim_ems/models/store.py` — immutable revisions, explicit activation, rollback, rebuild from the stored package
  - `tools/import_model.py` — offline import, validation report and optional round trip
  - `tools/model_admin.py` — import, list, activate, rollback against MongoDB
- Results go through `sourceDataUpdate` of supervised points; the driver never writes `soeData` or alarms
- The estimator network is built from the solved pypowsybl network, not from pandapower's CIM import
- Customer models, bindings and telemetry are never committed; tests use public or synthetic cases

## Work Guidance

- Follow the phases and exit criteria in `CIM_EMS_PLAN.md`; record new engine findings in its section 20
- Fixtures: the Eurostag tutorial network round-trips exactly; IEEE 14 and MicroGrid BE are not valid
  round-trip fixtures (nominal-voltage mismatch, missing boundary set); HVDC/VSC networks lose the VSC reactive setpoint

## Verification

- `python -m pytest` from this folder, in a venv with the pinned dependencies (uses `mongomock`)
- `CIM_EMS_TEST_MONGO=mongodb://127.0.0.1:27018 python -m pytest` runs the same tests on a real MongoDB (needed for the partial unique index), e.g. `docker run -d --rm -p 127.0.0.1:27018:27017 mongo:7`

## Child DOX Index

None.
