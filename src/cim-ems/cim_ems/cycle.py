"""One study connection: snapshot -> topology -> load flow -> publication -> run record."""
from __future__ import annotations

import hashlib
import json
import logging
import time
from datetime import datetime, timedelta, timezone
from importlib.metadata import version

import pypowsybl.loadflow as lf
from pymongo.database import Database

from .bindings.model import Binding
from .cache import PointCache
from . import contingency as ca
from .mongo import ARTIFACT_BUCKET, CONTINGENCY_LISTS, RUNS, RUNS_TTL_SECONDS, artifacts
from .publish import ResultPublisher
from .redundancy import Redundancy
from .estimation import adapter as est_adapter
from .estimation import transfer as est_transfer
from .results import ResultValue, extract_estimation, extract_loadflow
from .snapshot import SnapshotBuilder, SnapshotPolicy
from .topology import apply_snapshot

log = logging.getLogger("CIM-EMS.cycle")

STATUS_CODES = {"converged": 0, "diverged": 1, "rejected-snapshot": 2, "unsupported": 3, "failed": 4}
# estimation status: 0 converged, 1 not observable, 2 did not converge, 3 unsupported network, 4 failed, 5 no load flow
SE_CODES = {"converged": 0, "not-observable": 1, "diverged": 2, "unsupported": 3, "failed": 4, "no-load-flow": 5}
TRANSFER_GATE_PU, TRANSFER_GATE_DEG = 1e-4, 0.01


def lf_parameters(cfg: dict) -> lf.Parameters:
    p = cfg or {}
    kw = {}
    if "distributedSlack" in p:
        kw["distributed_slack"] = bool(p["distributedSlack"])
    if p.get("balanceType") in lf.BalanceType.__members__:
        kw["balance_type"] = getattr(lf.BalanceType, p["balanceType"])   # pybind enum: no subscript
    if p.get("voltageInitMode") in lf.VoltageInitMode.__members__:
        kw["voltage_init_mode"] = getattr(lf.VoltageInitMode, p["voltageInitMode"])
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
        self.run_se = bool(connection.get("runStateEstimation", False))
        self.se_cfg = connection.get("estimatorParameters") or {}
        self.se_rn_max = float(self.se_cfg.get("badDataThreshold", 4.0))
        self._transfer, self._transfer_hash, self._transfer_error = None, None, ""
        self.last_se_success = 0.0
        self.run_ca = bool(connection.get("runContingency", False))
        self.ca_list_id = connection.get("contingencyListId", "AUTO_N1")
        self.ca_interval = float(connection.get("contingencyIntervalSeconds", 300))
        self.ca_min_gap = float(connection.get("contingencyMinGapSeconds", 10))
        self.ca_min_kv = float(connection.get("contingencyMinKv", 0))
        self.ca_publish_limit = int(connection.get("contingencyPublishLimit", 200))
        self.ca_threads = int(connection.get("contingencyThreads", 0)) or None
        self._ca_hash, self._ca_last, self._ca_skipped, self.last_ca_success = None, 0.0, 0, 0.0
        self._ca_values: list[ResultValue] = []
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
        self._se_pending: list[ResultValue] = []
        self._ca_pending: list[ResultValue] = []
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
                        if self.run_se:
                            summary["estimation"] = self._estimate(run_id, snap, topo_hash, est_transfer.slack_of(results))
                        if self.run_ca:
                            summary["contingency"] = self._contingency(run_id, snap, topo_hash)
                    else:
                        status = "diverged"
        except Exception as e:                                        # never let a study kill the driver loop
            log.exception("study failed")
            status, summary["error"] = "failed", str(e)[:300]
        if self.run_se and "estimation" not in summary:
            summary["estimation"] = self._se_failed(run_id, snap, "no-load-flow", "no converged load flow")
        self.publisher.publish(run_id, snap.time, self._sys(status, sys_vals, t0, snap) + self._se_pending + self._ca_pending)
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

    def _ca_sys(self, status: str, duration_ms: float = 0.0, **extra) -> list[ResultValue]:
        vals = {"STATUS": float({"done": 0, "skipped": 1, "failed": 2, "no-load-flow": 3}[status]),
                "SKIPPED": float(self._ca_skipped), "DURATION": duration_ms, **extra}
        units = {"STATUS": "", "SKIPPED": "", "DURATION": "ms"}
        return [ResultValue("SYS", "CA", "CA", 0, q, v, units[q]) for q, v in vals.items()]

    def _contingency(self, run_id, snap, topo_hash) -> dict:
        """Run the analysis when the topology changed (after a minimum gap) or the interval elapsed; else skip."""
        now = time.monotonic()
        due = (topo_hash != self._ca_hash and now - self._ca_last >= self.ca_min_gap) or \
              now - self._ca_last >= self.ca_interval
        if not due:
            self._ca_skipped += 1
            self._ca_pending = self._ca_sys("skipped")
            if self._ca_values:      # keep the last results alive: their points expire after resultMaxAgeSeconds
                if self.publisher.publish(run_id, snap.time, self._ca_values):
                    self.last_ca_success = time.monotonic()
            return {"status": "skipped"}
        try:
            if self.ca_list_id == "AUTO_N1":
                cons = ca.auto_n1(self.net, self.ca_min_kv)
            else:
                doc = self.db[CONTINGENCY_LISTS].find_one({"listId": self.ca_list_id})
                if doc is None:
                    raise ValueError(f"contingency list {self.ca_list_id} not found")
                cons = ca.from_document(doc)
            res = ca.analyse(self.net, cons, self.params, self.ca_threads)
        except Exception as e:
            log.exception("contingency analysis failed")
            self._ca_pending = self._ca_sys("failed")
            if time.monotonic() - self.last_ca_success > self.result_max_age:
                self.publisher.invalidate("CA")
            return {"status": "failed", "error": str(e)[:300]}
        self._ca_hash, self._ca_last = topo_hash, time.monotonic()
        self._ca_values = ca.to_result_values(res, self.ca_publish_limit)
        n = self.publisher.publish(run_id, snap.time, self._ca_values)
        if n:
            self.last_ca_success = time.monotonic()
        self._ca_pending = self._ca_sys("done", res.duration_ms)
        fs = artifacts(self.db)
        artifact = fs.put(json.dumps(ca.to_document(res), default=str).encode(), filename=f"ca_{run_id}.json",
                          kind="contingency-result", runId=run_id, connectionNumber=self.number)
        self._prune_ca_artifacts(fs)
        insecure = [o.id for o in res.outcomes if o.status in ("violation", "islanding", "diverged")]
        return {"status": "done", "published": n > 0, "contingencies": len(res.outcomes), "insecure": insecure[:50],
                "unsupported": sum(o.status == "unsupported" for o in res.outcomes),
                "baseViolations": len(res.base_violations), "durationMs": round(res.duration_ms),
                "artifactId": str(artifact)}

    def _prune_ca_artifacts(self, fs) -> None:
        """Contingency result files live as long as the run records (cimRuns TTL); GridFS has no TTL of its own."""
        cutoff = datetime.now(timezone.utc) - timedelta(seconds=RUNS_TTL_SECONDS)
        try:
            for d in self.db[f"{ARTIFACT_BUCKET}.files"].find(
                    {"kind": "contingency-result", "connectionNumber": self.number, "uploadDate": {"$lt": cutoff}},
                    {"_id": 1}).limit(100):
                fs.delete(d["_id"])
        except Exception as e:
            log.warning("contingency artifact cleanup failed: %s", e)

    def _se_failed(self, run_id, snap, status, reason) -> dict:
        self._se_pending = self._se_sys(status)
        if time.monotonic() - self.last_se_success > self.result_max_age:
            self.publisher.invalidate("SE")
        return {"status": status, "reason": reason}

    @staticmethod
    def _se_sys(status, **extra) -> list[ResultValue]:
        vals = {"STATUS": float(SE_CODES[status]), "CONVERGED": 1.0 if status == "converged" else 0.0, **extra}
        units = {"STATUS": "", "CONVERGED": "", "BADDATA": "", "MEAS": "", "CHI2": ""}
        return [ResultValue("SYS", "SE", "SE", 0, q, v, units[q]) for q, v in vals.items()]

    def _transfer_for(self, topo_hash: str, slack: str | None = None):
        """pandapower network for the current topology; checked against the load flow once per topology."""
        if self._transfer_hash == topo_hash:
            return self._transfer, self._transfer_error
        self._transfer, self._transfer_hash, self._transfer_error = None, topo_hash, ""
        try:
            tr = est_transfer.to_pandapower(self.net, slack)
            cmp = est_transfer.compare_load_flow(self.net, tr)
            if cmp["max_dv_pu"] > TRANSFER_GATE_PU or cmp["max_dangle_deg"] > TRANSFER_GATE_DEG:
                self._transfer_error = (f"engine comparison failed: {cmp['max_dv_pu']:.4f} pu, "
                                        f"{cmp['max_dangle_deg']:.3f} deg")
            else:
                self._transfer = est_transfer.to_pandapower(self.net, slack)   # fresh copy, the comparison solved the first
        except est_transfer.UnsupportedForEstimation as e:
            self._transfer_error = str(e)
        except Exception as e:
            self._transfer_error = f"transfer failed: {type(e).__name__}: {e}"
        return self._transfer, self._transfer_error

    def _resolve_buses(self, measurements) -> list[est_adapter.Measurement]:
        """Voltage measurements may name a calculated bus, a bus-breaker view bus or a busbar section; the estimator
        wants calculated bus ids."""
        bbs: dict[str, str] = {}
        for getter in (self.net.get_bus_breaker_view_buses, self.net.get_busbar_sections):
            try:
                bbs.update({k: v for k, v in getter()["bus_id"].items() if v})
            except Exception:
                pass
        out = []
        for m in measurements:
            eid = m.element_id
            if m.quantity == "V" and eid in bbs:
                eid = bbs[eid]
            out.append(est_adapter.Measurement(eid, m.quantity, m.side, m.value, m.sigma, tag=m.element_id))
        return out

    def _estimate(self, run_id, snap, topo_hash, slack=None) -> dict:
        tr, err = self._transfer_for(topo_hash, slack)
        if tr is None:
            return self._se_failed(run_id, snap, "unsupported", err)
        ms = self._resolve_buses(snap.measurements)
        if not ms:
            return self._se_failed(run_id, snap, "not-observable", "no valid analog measurements")
        res = est_adapter.estimate(tr, ms, rn_max_threshold=self.se_rn_max)
        if not res.converged:
            status = "diverged" if res.observable else "not-observable"
            return self._se_failed(run_id, snap, status, res.message)
        values = extract_estimation(self.net, res, self.publish_filter)
        thr = res.chi2_threshold
        self._se_pending = self._se_sys("converged", BADDATA=float(len(res.bad_data)),
                                        MEAS=float(res.measurements_used),
                                        CHI2=res.chi2_statistic / thr if thr == thr and thr else 0.0)
        n = self.publisher.publish(run_id, snap.time, values)
        if n:
            self.last_se_success = time.monotonic()
        return {"status": "converged", "published": n > 0, "measurements": res.measurements_used,
                "degreesOfFreedom": res.degrees_of_freedom, "chi2Passed": res.chi2_passed,
                "badData": [{"element": d.measurement.element_id, "quantity": d.measurement.quantity,
                             "side": d.measurement.side, "normalized": round(d.normalized, 2)}
                            for d in res.bad_data[:20]],
                "durationMs": round(res.duration_ms)}

    def _sys(self, status, sys_vals, t0, snap) -> list[ResultValue]:
        vals = {"STATUS": float(STATUS_CODES[status]), "CONVERGED": 1.0 if status == "converged" else 0.0,
                "DURATION": (time.monotonic() - t0) * 1000.0,
                "UNCERTAIN": 1.0 if "topologyUncertain" in snap.flags else 0.0, **sys_vals}
        units = {"STATUS": "", "CONVERGED": "", "DURATION": "ms", "UNCERTAIN": "", "ITER": ""}
        return [ResultValue("SYS", "LF", "LF", 0, q, v, units[q]) for q, v in vals.items()]
