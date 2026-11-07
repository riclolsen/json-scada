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
  - `cim_ems/bindings/` — `Binding` model with CSV review format; `proposal.py` name-matching proposals (never approved automatically; best score wins a tag, weaker claimants stay unmatched)
  - `cim_ems/snapshot.py` — snapshot policy: switch/tap/analog rules, hold of last valid state, rejection, skew and age
  - `cim_ems/topology.py` — applies a snapshot to the network, island counts, deterministic topology hash
  - `cim_ems/synthetic.py` — synthetic tags and switching events for tests and demos
  - `tools/replay.py` — replays switching events through snapshot and topology (`--synthetic N` for a smoke run)
  - `tools/import_model.py` — offline import, validation report and optional round trip
  - `tools/fetch_test_models.py` — downloads public CGMES 2.4.15 conformity models (and with `--relicap` the CGMES 3.0 ReliCapGrid regions, common data merged into EQ) into the gitignored `tests/data/conformity/`
  - `tools/model_admin.py` — import, list, activate, rollback against MongoDB
- Switch values are never aged (a stable breaker has an old timestamp); invalid ones are held for `max_hold_seconds`, never assumed closed
- Results go through `sourceDataUpdate` of supervised points; the driver never writes `soeData` or alarms
- The estimator network is built from the solved pypowsybl network, not from pandapower's CIM import
- Customer models, bindings and telemetry are never committed; tests use public or synthetic cases

## Work Guidance

- Follow the phases and exit criteria in `CIM_EMS_PLAN.md`; record new engine findings in its section 20
- Round-trip re-import turns off `convert-sv-injections` (exporter SvInjection becomes a nodeless load on node-breaker models)
- Public models (SmallGrid NodeBreaker, MicroGrid BaseCase, ReliCapGrid regions) are fetched, never committed; `tests/integration/` skips without them. Their boundary set must be re-attached on round trip
- Fixtures: the Eurostag tutorial network round-trips exactly; IEEE 14 and MicroGrid BE are not valid
  round-trip fixtures (nominal-voltage mismatch, missing boundary set); HVDC/VSC networks lose the VSC reactive setpoint

## Verification

- `python -m pytest` from this folder, in a venv with the pinned dependencies (uses `mongomock`)
- `CIM_EMS_TEST_MONGO=mongodb://127.0.0.1:27018 python -m pytest` runs the same tests on a real MongoDB (needed for the partial unique index), e.g. `docker run -d --rm -p 127.0.0.1:27018:27017 mongo:7`

## Child DOX Index

None.
