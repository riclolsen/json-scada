"""One study connection: snapshot -> topology -> load flow -> publication -> run record."""
from __future__ import annotations

import hashlib
import json
import logging
import time
from datetime import datetime, timezone
from importlib.metadata import version

import pypowsybl.loadflow as lf
from pymongo.database import Database

from .bindings.model import Binding
from .cache import PointCache
from .mongo import RUNS
from .publish import ResultPublisher
from .redundancy import Redundancy
from .results import ResultValue, extract_loadflow
from .snapshot import SnapshotBuilder, SnapshotPolicy
from .topology import apply_snapshot

log = logging.getLogger("CIM-EMS.cycle")

STATUS_CODES = {"converged": 0, "diverged": 1, "rejected-snapshot": 2, "unsupported": 3, "failed": 4}


def lf_parameters(cfg: dict) -> lf.Parameters:
    p = cfg or {}
    kw = {}
    if "distributedSlack" in p:
        kw["distributed_slack"] = bool(p["distributedSlack"])
    if p.get("balanceType") in lf.BalanceType.__members__:
        kw["balance_type"] = lf.BalanceType[p["balanceType"]]
    if p.get("voltageInitMode") in lf.VoltageInitMode.__members__:
        kw["voltage_init_mode"] = lf.VoltageInitMode[p["voltageInitMode"]]
    if "transformerVoltageControlOn" in p:
        kw["transformer_voltage_control_on"] = bool(p["transformerVoltageControlOn"])
    return lf.Parameters(**kw)


class StudyCycle:
    def __init__(self, db: Database, connection: dict, model: dict, net, bindings: list[Binding],
                 cache: PointCache, publisher: ResultPublisher, redundancy: Redundancy, node_name: str):
        self.db, self.conn, self.model, self.net = db, connection, model, net
        self.cache, self.publisher, self.red, self.node = cache, publisher, redundancy, node_name
        self.number = int(connection["protocolConnectionNumber"])
        self.builder = SnapshotBuilder(
            bindings,
            SnapshotPolicy(max_hold_seconds=float(connection.get("switchHoldSeconds", 300)),
                           max_skew_seconds=float(connection.get("snapshotMaxSkewSeconds", 10))),
        )
        self.binding_revision = connection.get("cimBindingRevision")
        self.params_cfg = connection.get("loadFlowParameters") or {}
        self.params = lf_parameters(self.params_cfg)
        self.publish_filter = connection.get("publishFilter") or {}
        self.result_max_age = float(connection.get("resultMaxAgeSeconds", 180))
        self.last_success = 0.0                  # monotonic seconds of the last published converged run
        self._last_run_id = 0
        self.parameters_hash = hashlib.sha256(json.dumps(self.params_cfg, sort_keys=True).encode()).hexdigest()[:8]

    def next_run_id(self) -> int:
        self._last_run_id = max(int(time.time() * 1000), self._last_run_id + 1)
        return self._last_run_id

    def run_once(self, trigger: str = "cycle") -> dict:
        t0 = time.monotonic()
        now = datetime.now(timezone.utc)
        run_id = self.next_run_id()
        snap = self.builder.build(self.cache.points(), now)
        status, summary, published, topo_hash = "failed", {}, False, ""
        sys_vals: dict[str, float] = {}
        try:
            if snap.rejected:
                status = "rejected-snapshot"
            else:
                info = apply_snapshot(self.net, snap)
                topo_hash = info.hash
                summary.update(buses=info.buses, islands=info.connected_components, switchesOpen=info.switches_open)
                results = lf.run_ac(self.net, self.params)
                if not results:
                    status = "unsupported"
                    summary["reason"] = "no calculable network component"
                else:
                    main = results[0]
                    summary.update(iterations=int(main.iteration_count), loadFlowStatus=main.status.name)
                    sys_vals["ITER"] = float(main.iteration_count)
                    if main.status.name == "CONVERGED":
                        status = "converged"
                        values = extract_loadflow(self.net, self.publish_filter)
                        n = self.publisher.publish(run_id, snap.time, values)
                        published = n > 0
                        if published:
                            self.last_success = time.monotonic()
                    else:
                        status = "diverged"
        except Exception as e:                                        # never let a study kill the driver loop
            log.exception("study failed")
            status, summary["error"] = "failed", str(e)[:300]
        self.publisher.publish(run_id, snap.time, self._sys(status, sys_vals, t0, snap))
        if not published and time.monotonic() - self.last_success > self.result_max_age:
            self.publisher.invalidate("LF")
        summary.update(flags=sorted(snap.flags), reasons=snap.reasons[:20])
        doc = {
            "runId": run_id, "connectionNumber": self.number, "kind": "LF", "trigger": trigger,
            "nodeName": self.node, "snapshotTime": snap.time, "modelId": self.model["modelId"],
            "modelRevision": self.model["revision"], "bindingRevision": self.binding_revision,
            "topologyHash": topo_hash,
            "engineVersions": {"pypowsybl": version("pypowsybl")}, "parametersHash": self.parameters_hash,
            "status": status, "durationMs": int((time.monotonic() - t0) * 1000), "summary": summary,
            "published": published, "createdAt": datetime.now(timezone.utc),
        }
        self.db[RUNS].insert_one(doc)
        log.info("run %d %s hash=%s %d ms published=%s", run_id, status, topo_hash, doc["durationMs"], published)
        return doc

    def _sys(self, status, sys_vals, t0, snap) -> list[ResultValue]:
        vals = {"STATUS": float(STATUS_CODES[status]), "CONVERGED": 1.0 if status == "converged" else 0.0,
                "DURATION": (time.monotonic() - t0) * 1000.0,
                "UNCERTAIN": 1.0 if "topologyUncertain" in snap.flags else 0.0, **sys_vals}
        units = {"STATUS": "", "CONVERGED": "", "DURATION": "ms", "UNCERTAIN": "", "ITER": ""}
        return [ResultValue("SYS", "LF", "LF", 0, q, v, units[q]) for q, v in vals.items()]
