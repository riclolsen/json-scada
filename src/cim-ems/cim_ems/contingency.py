"""Contingency analysis with the pypowsybl AC security analysis.

Outcome per contingency: secure, violation, islanding, diverged, unsupported. A violation counts only when the base
case does not already have it, so VIOL says what the outage causes.
"""
from __future__ import annotations

import time
from dataclasses import dataclass, field

import pypowsybl.loadflow as lf
import pypowsybl.network as pn
import pypowsybl.security as sa

from .results import ResultValue, _num

STATUS_CODES = {"secure": 0, "violation": 1, "islanding": 2, "diverged": 3, "unsupported": 4}


@dataclass
class Contingency:
    id: str
    name: str
    elements: list[str]


@dataclass
class Outcome:
    id: str
    name: str
    status: str
    violations: list[dict] = field(default_factory=list)
    worst_loading: float | None = None
    disconnected: list[str] = field(default_factory=list)   # elements lost besides the outage itself


@dataclass
class CaResult:
    outcomes: list[Outcome]
    base_violations: list[dict]
    duration_ms: float
    base_status: str = ""


def from_document(doc: dict) -> list[Contingency]:
    """List document: {listId, contingencies: [{id, name?, elements: [id, ...]}]}."""
    return [Contingency(c["id"], c.get("name") or c["id"], list(c["elements"])) for c in doc.get("contingencies", [])]


def auto_n1(net: pn.Network, min_nominal_kv: float = 0.0) -> list[Contingency]:
    """Every line and two-winding transformer outage, one at a time."""
    vls = net.get_voltage_levels()["nominal_v"]
    out = []
    for df in (net.get_lines(), net.get_2_windings_transformers()):
        for eid, r in df.iterrows():
            if vls[r.voltage_level1_id] >= min_nominal_kv and r.connected1 and r.connected2:
                out.append(Contingency(eid, r["name"] or eid, [eid]))
    return out


def permanent_limits(net: pn.Network) -> dict[tuple[str, str], float]:
    limits = net.get_operational_limits()
    if not len(limits):
        return {}
    lim = limits.reset_index()
    lim = lim[(lim["type"] == "CURRENT") & (lim["acceptable_duration"] == -1)]
    return {(r.element_id, r.side): r.value for r in lim.itertuples()}


def _violations(df, cid: str) -> list[dict]:
    if df is None or not len(df) or cid not in df.index.get_level_values(0):
        return []
    out = []
    for (c, subject), r in df.loc[[cid]].reset_index().set_index(["contingency_id", "subject_id"]).iterrows():
        out.append({"subject": subject, "type": r["limit_type"], "side": r.get("side", "") or "",
                    "limit": _num(r["limit"]), "value": _num(r["value"])})
    return out


def _key(v: dict) -> tuple:
    return v["subject"], v["type"], v["side"]


def analyse(net: pn.Network, contingencies: list[Contingency], lf_params: lf.Parameters | None = None,
            ) -> CaResult:
    t0 = time.perf_counter()
    known = set(net.get_identifiables().index)
    outcomes: dict[str, Outcome] = {}
    valid: list[Contingency] = []
    for c in contingencies:
        missing = [e for e in c.elements if e not in known]
        if not c.elements or missing:
            outcomes[c.id] = Outcome(c.id, c.name, "unsupported", disconnected=missing)
        else:
            valid.append(c)
    base: list[dict] = []
    base_status = ""
    if valid:
        analysis = sa.create_analysis()
        for c in valid:
            if len(c.elements) == 1:
                analysis.add_single_element_contingency(c.elements[0], c.id)
            else:
                analysis.add_multiple_elements_contingency(c.elements, c.id)
        branch_ids = list(net.get_lines().index) + list(net.get_2_windings_transformers().index)
        analysis.add_monitored_elements(branch_ids=branch_ids)
        params = sa.Parameters(load_flow_parameters=lf_params) if lf_params is not None else None
        res = analysis.run_ac(net, parameters=params) if params else analysis.run_ac(net)
        base_status = res.pre_contingency_result.status.name
        viol = res.limit_violations
        base = _violations(viol, "")
        base_keys = {_key(v) for v in base}
        perm = permanent_limits(net)
        br = res.branch_results
        for c in valid:
            post = res.post_contingency_results.get(c.id)
            if post is None or post.status.name != "CONVERGED":
                outcomes[c.id] = Outcome(c.id, c.name, "diverged")
                continue
            new = [v for v in _violations(viol, c.id) if _key(v) not in base_keys]
            lost = [e for e in post.disconnected_elements if e not in c.elements]
            worst = None
            if len(br) and c.id in br.index.get_level_values(0):
                for (_, _, bid), r in br.loc[[c.id]].reset_index().set_index(["contingency_id", "operator_strategy_id", "branch_id"]).iterrows():
                    for side, tag in ((1, "ONE"), (2, "TWO")):
                        lim, cur = perm.get((bid, tag)), _num(r[f"i{side}"])
                        if lim and cur is not None:
                            worst = max(worst or 0.0, 100.0 * cur / lim)
            status = "islanding" if lost else ("violation" if new else "secure")
            outcomes[c.id] = Outcome(c.id, c.name, status, new, worst, lost)
    ordered = [outcomes[c.id] for c in contingencies]
    return CaResult(ordered, base, (time.perf_counter() - t0) * 1000.0, base_status)


def to_result_values(res: CaResult, publish_limit: int = 200) -> list[ResultValue]:
    """Per contingency STATUS, VIOL, WORST (list order, capped) and the SYSTEM aggregates."""
    out: list[ResultValue] = []
    for o in res.outcomes[:publish_limit]:
        out += [ResultValue("CA", o.id, o.name, 0, "STATUS", float(STATUS_CODES[o.status]), ""),
                ResultValue("CA", o.id, o.name, 0, "VIOL", float(len(o.violations)), ""),
                ResultValue("CA", o.id, o.name, 0, "WORST", o.worst_loading, "%")]
    worst = [o.worst_loading for o in res.outcomes if o.worst_loading is not None]
    out += [
        ResultValue("CA", "SYSTEM", "SYSTEM", 0, "VIOL", float(sum(len(o.violations) for o in res.outcomes)), ""),
        ResultValue("CA", "SYSTEM", "SYSTEM", 0, "WORST", max(worst) if worst else None, "%"),
        ResultValue("CA", "SYSTEM", "SYSTEM", 0, "INSECURE",
                    float(sum(o.status in ("violation", "islanding", "diverged") for o in res.outcomes)), ""),
        ResultValue("CA", "SYSTEM", "SYSTEM", 0, "COUNT", float(len(res.outcomes)), ""),
    ]
    return out


def to_document(res: CaResult) -> dict:
    return {"durationMs": round(res.duration_ms), "baseStatus": res.base_status, "baseViolations": res.base_violations,
            "outcomes": [{"id": o.id, "name": o.name, "status": o.status, "violations": o.violations,
                          "worstLoading": o.worst_loading, "disconnected": o.disconnected} for o in res.outcomes]}
