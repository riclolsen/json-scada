"""Estimator adapter: measurements in, voltages, residuals and bad data out. Engine: pandapower WLS."""
from __future__ import annotations

import copy
import logging
import math
import time
from dataclasses import dataclass, field

import pandapower as pp
import pandas as pd
import pandapower.estimation as est
from scipy.stats import chi2

from .transfer import Transfer

log = logging.getLogger("CIM-EMS.estimation")


@dataclass
class Measurement:
    element_id: str      # IIDM bus, line or two-winding transformer id
    quantity: str        # V (kV, bus), P, Q (MW, MVAr, branch end, power into the branch)
    side: int            # 1 or 2 for branches, 0 for buses
    value: float
    sigma: float         # same unit as the value
    tag: str = ""


@dataclass
class Residual:
    index: int
    measurement: Measurement
    estimated: float
    normalized: float    # (measured - estimated) / sigma
    bad: bool = False


@dataclass
class EstimationResult:
    converged: bool
    observable: bool
    message: str = ""
    iterations: int = 0
    duration_ms: float = 0.0
    bus_v_pu: dict[str, float] = field(default_factory=dict)
    bus_angle_deg: dict[str, float] = field(default_factory=dict)
    residuals: list[Residual] = field(default_factory=list)
    branch_flows: dict[tuple[str, int, str], float] = field(default_factory=dict)   # (id, side, P|Q) -> value
    measurements_used: int = 0
    bad_data: list[Residual] = field(default_factory=list)   # removed by the largest normalized residual loop
    chi2_statistic: float = float("nan")
    chi2_threshold: float = float("nan")
    chi2_passed: bool | None = None
    degrees_of_freedom: int = 0


def _row(net: pp.pandapowerNet, tr: Transfer, m: Measurement) -> tuple | None:
    """pandapower measurement row (type, element type, element, value, std, side), None when the element is not in
    the transferred network."""
    if m.quantity == "V" and m.element_id in tr.bus_index:
        b = tr.bus_index[m.element_id]
        vn = float(net.bus.vn_kv.at[b])
        return "v", "bus", b, m.value / vn, m.sigma / vn, None
    kind = {"P": "p", "Q": "q"}.get(m.quantity)
    if kind is None:
        return None
    if m.element_id in tr.line_index:
        return kind, "line", tr.line_index[m.element_id], m.value, m.sigma, "from" if m.side == 1 else "to"
    if m.element_id in tr.trafo_index:
        return kind, "trafo", tr.trafo_index[m.element_id], m.value, m.sigma, "hv" if m.side == 1 else "lv"
    return None


def _set_measurements(net: pp.pandapowerNet, rows: list[tuple]) -> None:
    """Replace the measurement table in one step (create_measurement per row costs seconds for 1000 rows)."""
    cols = ["name", "measurement_type", "element_type", "element", "value", "std_dev", "side"]
    df = pd.DataFrame([(None, *r) for r in rows], columns=cols)
    net.measurement = df.astype(net.measurement.dtypes.to_dict())


def _estimated(net: pp.pandapowerNet, tr: Transfer, m: Measurement) -> float | None:
    if m.quantity == "V":
        b = tr.bus_index[m.element_id]
        return float(net.res_bus_est.vm_pu[b] * net.bus.vn_kv[b])
    col = {("line", "P", 1): "p_from_mw", ("line", "P", 2): "p_to_mw", ("line", "Q", 1): "q_from_mvar",
           ("line", "Q", 2): "q_to_mvar", ("trafo", "P", 1): "p_hv_mw", ("trafo", "P", 2): "p_lv_mw",
           ("trafo", "Q", 1): "q_hv_mvar", ("trafo", "Q", 2): "q_lv_mvar"}
    if m.element_id in tr.line_index:
        return float(net.res_line_est[col[("line", m.quantity, m.side)]][tr.line_index[m.element_id]])
    if m.element_id in tr.trafo_index:
        return float(net.res_trafo_est[col[("trafo", m.quantity, m.side)]][tr.trafo_index[m.element_id]])
    return None


def estimate(tr: Transfer, measurements: list[Measurement], remove_bad_data: bool = True,
             rn_max_threshold: float = 4.0, max_removals: int = 10, alpha: float = 0.01) -> EstimationResult:
    """Weighted least squares estimate of the transferred network from the measurements.

    Bad data: while the largest normalized residual exceeds `rn_max_threshold`, drop that measurement (at most
    `max_removals`) and estimate again. Dropped measurements are reported in `bad_data`.
    """
    t0 = time.perf_counter()
    active = list(measurements)
    dropped: list[Residual] = []
    rows = {id(m): _row(tr.net, tr, m) for m in active}
    active = [m for m in active if rows[id(m)] is not None]
    for _ in range(max_removals + 1):
        net = copy.deepcopy(tr.net)
        kept = list(active)
        _set_measurements(net, [rows[id(m)] for m in kept])
        if not kept:
            return EstimationResult(False, False, "no usable measurements", duration_ms=(time.perf_counter() - t0) * 1e3)
        try:
            ok = est.estimate(net, algorithm="wls", init="flat",
                              zero_injection="aux_bus", tolerance=1e-6, maximum_iterations=30)
        except Exception as e:   # unobservable systems raise from the gain matrix factorization
            return EstimationResult(False, False, f"estimator failed: {type(e).__name__}: {e}",
                                    bad_data=dropped, duration_ms=(time.perf_counter() - t0) * 1e3)
        if not ok:
            return EstimationResult(False, True, "estimator did not converge", bad_data=dropped,
                                    duration_ms=(time.perf_counter() - t0) * 1e3)
        res = []
        try:
            for i, m in enumerate(kept):
                e = _estimated(net, tr, m)
                res.append(Residual(i, m, e, (m.value - e) / m.sigma))
            complete = all(math.isfinite(r.normalized) for r in res)
        except (KeyError, IndexError):
            complete = False
        if not complete:   # pandapower may report success with a singular gain matrix and partial result tables
            return EstimationResult(False, False, "estimator result incomplete (singular gain matrix, not observable)",
                                    bad_data=dropped, duration_ms=(time.perf_counter() - t0) * 1e3)
        worst = max(res, key=lambda r: abs(r.normalized))
        if remove_bad_data and abs(worst.normalized) > rn_max_threshold and len(dropped) < max_removals:
            worst.bad = True
            dropped.append(worst)
            active = [m for m in kept if m is not worst.measurement]
            continue
        n_state = 2 * len(net.bus) - 1
        dof = max(len(kept) - n_state, 0)
        stat = float(sum(r.normalized**2 for r in res))
        thr = float(chi2.ppf(1 - alpha, dof)) if dof else float("nan")
        flows = {}
        for eid, i in tr.line_index.items():
            for side, (pc, qc) in ((1, ("p_from_mw", "q_from_mvar")), (2, ("p_to_mw", "q_to_mvar"))):
                flows[(eid, side, "P")], flows[(eid, side, "Q")] = float(net.res_line_est[pc][i]), float(net.res_line_est[qc][i])
        for eid, i in tr.trafo_index.items():
            for side, (pc, qc) in ((1, ("p_hv_mw", "q_hv_mvar")), (2, ("p_lv_mw", "q_lv_mvar"))):
                flows[(eid, side, "P")], flows[(eid, side, "Q")] = float(net.res_trafo_est[pc][i]), float(net.res_trafo_est[qc][i])
        return EstimationResult(
            True, True, "", iterations=0, branch_flows=flows, measurements_used=len(kept), duration_ms=(time.perf_counter() - t0) * 1e3,
            bus_v_pu={bid: float(net.res_bus_est.vm_pu[i]) for bid, i in tr.bus_index.items()},
            bus_angle_deg={bid: float(net.res_bus_est.va_degree[i]) for bid, i in tr.bus_index.items()},
            residuals=res, bad_data=dropped, chi2_statistic=stat, chi2_threshold=thr,
            chi2_passed=(stat <= thr) if dof else None, degrees_of_freedom=dof)
    return EstimationResult(False, True, "bad data removal limit reached", bad_data=dropped)
