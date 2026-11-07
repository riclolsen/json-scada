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
  `cimArtifacts` GridFS bucket belong to this driver; indexes in `cim_ems/mongo.py` and `mongo_seed/b_create-db.js`
  (keep both in step), collection table in `docs/schema.md`
- The Admin UI page (`src/AdminUI/src/components/CimEmsTab.vue`) and its backend
  (`src/server_realtime_auth/app/controllers/cimEms.controller.js`) only queue `cimStudyRequests`; this driver
  executes them (`cim_ems/api.py`)

## Local Contracts

- **Language:** Python 3.10 to 3.14, versions pinned in `pyproject.toml`; Linux x86_64 and Windows x64 only
  (no pypowsybl ARM64 wheel)
- **Packaging:** `install.sh` creates `.venv` here (`pip install .`) and exits 0 with a notice on non-x86_64.
  `install.bat` installs into `platform-windows/python-runtime` when it exists (shipped by the Windows installer),
  else creates `.venv`. Callers: `platform-linux/build.sh`, `platform-windows/build.bat`/`buildupd.bat`, root
  `Dockerfile` (amd64). Supervisor program `cim_ems` (`platform-*/cim_ems.ini`, `autostart=false`) runs
  `.venv/bin/python -m cim_ems 1 1 <conf>/json-scada.json`; Windows service `JSON_SCADA_cim_ems` (demand start) runs
  `python-runtime\python.exe -m cim_ems`. Process manager entry `CIM-EMS` (type `python`) in
  `src/server_realtime_auth/app/services/process-manager/driver-catalog.js` uses the same names
  New subpackages are picked up by `[tool.setuptools.packages.find]`
- **CGMES version:** 2.4.15 (CIM16) only; export with `iidm.export.cgmes.cim-version=16`. CGMES 3.0 is opt-in later
- **CLI:** `python -m cim_ems <instance> <logLevel> <configFile>`, like the other drivers
- **Files:**
  - `cim_ems/config.py` — `json-scada.json` and CLI args
  - `cim_ems/models/importer.py` — pinned import parameters, validation report, unsupported-class list
  - `cim_ems/export.py` — CGMES export and the round-trip check (compares solved values, never XML bytes)
  - `cim_ems/__main__.py` — driver loop: instance check, keep-alive, one `StudyCycle` per enabled connection, reload on connection change; runs are `start`, `topology` (switch/tap change, debounced 1 s), `request` or `cycle`
  - `cim_ems/api.py` — study requests on the active node: atomic claim (`pending` → `running`), `import`, `activate`, `rollback`, `proposeBindings`, `saveBindings`, `rerun`; result in `result` or `error`; `running` older than 1 h is failed as interrupted. Activation and binding saves rebuild the affected cycles. Requests run synchronously in the loop (an import blocks the studies while it runs)
  - `cim_ems/cache.py` — `PointCache`: processed `realtimeData` values of bound tags via change stream opened before the full load, resync on any error or request
  - `cim_ems/cycle.py` — `StudyCycle.run_once`: snapshot, topology, load flow, publication, `cimRuns` record; failures never stop the loop
  - `cim_ems/results.py` — load flow result extraction (bus V/VPU/ANG, branch P/Q/I per end, LOAD against the permanent current limit); disconnected ends and NaN become "not calculated"
  - `cim_ems/publish.py` — result point registry, auto-created supervised points, `sourceDataUpdate` publication, per-kind monotonic run guard, `still_active()` check before every write, `invalidate`
  - `cim_ems/bindings/store.py` — versioned binding sets in `cimMeasurementBindings`
  - `cim_ems/mongo.py` — collection names, indexes (one active revision per model, TTL on runs), GridFS bucket
  - `cim_ems/redundancy.py` — active-node arbitration; `still_active()` must be checked right before every publication
  - `cim_ems/models/store.py` — immutable revisions, explicit activation, rollback, rebuild from the stored package
  - `cim_ems/bindings/` — `Binding` model with CSV review format; `proposal.py` name-matching proposals (never approved automatically; best score wins a tag, weaker claimants stay unmatched)
  - `cim_ems/snapshot.py` — snapshot policy: switch/tap/analog rules, hold of last valid state, rejection, skew and age
  - `cim_ems/topology.py` — applies a snapshot to the network, island counts, deterministic topology hash
  - `cim_ems/synthetic.py` — synthetic tags and switching events for tests and demos
  - `cim_ems/contingency.py` — pypowsybl AC security analysis, classification (secure/violation/islanding/diverged/unsupported), CA result values; runs synchronously in the study cycle with a topology-change/interval skip policy
  - `cim_ems/estimation/` — pandapower network transfer from a solved pypowsybl net (`transfer.py`), WLS estimator adapter with bad data removal (`adapter.py`), synthetic measurements (`synthetic.py`); used by the study cycle when the connection sets `runStateEstimation`; SE status points share the SYS publication with the load flow ones (one monotonic run guard per kind)
  - `cim_ems/diagrams/` — PowSyBl single-line diagram to svg-scada markup; `tools/gen_displays.py` generates one SVG per voltage level (switch tags from approved bindings, result tags named as `publish.tag_name`)
  - `tools/replay.py` — replays switching events through snapshot and topology (`--synthetic N` for a smoke run)
  - `tools/import_model.py` — offline import, validation report and optional round trip
  - `tools/fetch_test_models.py` — downloads public CGMES 2.4.15 conformity models (and with `--relicap` the CGMES 3.0 ReliCapGrid regions, common data merged into EQ) into the gitignored `tests/data/conformity/`
  - `tools/bindings_admin.py` — propose (switch and analog name matching), save a reviewed CSV as a binding revision, export a revision
  - `tools/model_admin.py` — import, list, activate, rollback against MongoDB
- Switch values are never aged (a stable breaker has an old timestamp); invalid ones are held for `max_hold_seconds`, never assumed closed
- Results go through `sourceDataUpdate` of supervised points; the driver never writes `soeData` or alarms
- Result points are keyed by (study kind, `<mRID>|<side>|<quantity>`), keys in `connection * 1,000,000 + n`, `invalidDetectTimeout` = `resultMaxAgeSeconds` and every point is rewritten each run, so a dead driver expires its results in the data processor
- System health points (`SYS`: STATUS, CONVERGED, ITER, DURATION, UNCERTAIN) are published after every run, whatever its outcome
- `CIM_EMS_KEEPALIVE_TIMEOUT` (seconds, default 30) shortens the failover time for tests
- The estimator network is built from the solved pypowsybl network, not from pandapower's CIM import
- Customer models, bindings and telemetry are never committed; tests use public or synthetic cases
- Request contents are re-checked here (validation, own-point refusal, binding fields) even though the backend checks rights

## Work Guidance

- Follow the phases and exit criteria in `CIM_EMS_PLAN.md`; record new engine findings in its section 20
- Round-trip re-import turns off `convert-sv-injections` (exporter SvInjection becomes a nodeless load on node-breaker models)
- Public models (SmallGrid NodeBreaker, MicroGrid BaseCase, ReliCapGrid regions) are fetched, never committed; `tests/integration/` skips without them. Their boundary set must be re-attached on round trip
- Fixtures: the Eurostag tutorial network round-trips exactly; IEEE 14 and MicroGrid BE are not valid
  round-trip fixtures (nominal-voltage mismatch, missing boundary set); HVDC/VSC networks lose the VSC reactive setpoint

## Verification

- `./install.sh` in a clean `ubuntu:24.04` container (Python 3.12) followed by `pytest tests/unit` checks packaging
- `python -m pytest` from this folder, in a venv with the pinned dependencies (uses `mongomock`; `tests/conftest.py` patches its bulk API for current pymongo)
- `CIM_EMS_TEST_MONGO=mongodb://127.0.0.1:27018 python -m pytest` runs the same tests on a real MongoDB (needed for the partial unique index), e.g. `docker run -d --rm -p 127.0.0.1:27018:27017 mongo:7`
- `CIM_EMS_TEST_MONGO_RS="mongodb://127.0.0.1:27019/?directConnection=true"` enables the change-stream and end-to-end tests (driver process, `cs_data_processor`, failover); start `mongo:7 --replSet rs0 --bind_ip_all` and run `replSetInitiate` with host `127.0.0.1:27017`

## Child DOX Index

None.
