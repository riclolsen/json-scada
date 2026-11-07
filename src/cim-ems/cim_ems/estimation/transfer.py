"""Build a pandapower network from a solved pypowsybl network (option B of the plan).

Only the largest synchronous component is transferred. Generation and loads are fixed injections taken from the
solved state; one external grid at the slack bus takes the rest. The resulting network reproduces the pypowsybl load
flow, which is checked by `compare_load_flow` before any estimate is trusted.
"""
from __future__ import annotations

import math
from dataclasses import dataclass, field

import numpy as np
import pandapower as pp
import pypowsybl.network as pn

S_BASE = 100.0
FREQ = 50.0
SHIFT_SIGN = -1.0   # pypowsybl alpha: v2 leads v1 by alpha; pandapower shift_degree: lv lags hv by the angle


class UnsupportedForEstimation(Exception):
    pass


@dataclass
class Transfer:
    net: pp.pandapowerNet
    bus_index: dict[str, int]                 # IIDM bus id -> pandapower bus
    line_index: dict[str, int]
    trafo_index: dict[str, int]
    slack_bus: str
    skipped: list[str] = field(default_factory=list)   # lines open at one end (kept as shunts, no flow)


def _clean(v, default=0.0):
    return default if v is None or (isinstance(v, float) and math.isnan(v)) else float(v)


def to_pandapower(src: pn.Network, slack_bus: str | None = None) -> Transfer:
    """`slack_bus`: the load flow's slack bus (`ComponentResult.slack_bus_results[0].id`). The external grid goes
    there, so the residual slack mismatch of the load flow is absorbed where the load flow left it. Without it, the
    bus of the largest voltage-regulating generator is used."""
    buses = src.get_buses()
    if buses.empty:
        raise UnsupportedForEstimation("no calculated buses")
    if len(src.get_3_windings_transformers()):
        raise UnsupportedForEstimation("three-winding transformers are not supported by the estimator")
    if len(src.get_hvdc_lines()):
        raise UnsupportedForEstimation("HVDC lines are not supported by the estimator")
    comp = buses["synchronous_component"].value_counts().idxmax()
    main = buses[buses["synchronous_component"] == comp]
    vls = src.get_voltage_levels()["nominal_v"]
    net = pp.create_empty_network(sn_mva=S_BASE, f_hz=FREQ)
    # Rows are collected first and created in bulk: element-by-element creation costs seconds on 100+ buses.
    ids = list(main.index)
    vn_of = {bid: float(vls[main.loc[bid, "voltage_level_id"]]) for bid in ids}
    bus_index: dict[str, int] = {bid: i for i, bid in enumerate(ids)}
    pp.create_buses(net, len(ids), vn_kv=[vn_of[b] for b in ids], name=ids, index=list(range(len(ids))))
    net.bus["vm_init"] = [_clean(main.loc[b, "v_mag"], vn_of[b]) / vn_of[b] for b in ids]
    net.bus["va_init"] = [_clean(main.loc[b, "v_angle"]) for b in ids]

    gens = src.get_generators()
    gens = gens[gens.bus_id.isin(bus_index)]
    slack_candidates = gens[gens.voltage_regulator_on] if gens.voltage_regulator_on.any() else gens
    if slack_candidates.empty and slack_bus not in bus_index:
        raise UnsupportedForEstimation("no generator to hold the slack")
    if slack_bus in bus_index:
        slack_gen = None
    else:
        slack_gen = slack_candidates.sort_values("max_p", ascending=False).index[0]
        slack_bus = gens.loc[slack_gen, "bus_id"]
    sb = bus_index[slack_bus]
    pp.create_ext_grid(net, bus=sb, vm_pu=main.loc[slack_bus, "v_mag"] / vn_of[slack_bus],
                       va_degree=_clean(main.loc[slack_bus, "v_angle"]), name=slack_gen or slack_bus)
    g = gens.drop(index=[slack_gen]) if slack_gen is not None else gens
    if len(g):
        pp.create_sgens(net, [bus_index[x] for x in g.bus_id], p_mw=[-_clean(x) for x in g.p],
                        q_mvar=[-_clean(x) for x in g.q], name=list(g.index))

    loads = {"bus": [], "p": [], "q": [], "name": []}
    for table in (src.get_loads(), src.get_dangling_lines()):
        t = table[table.bus_id.isin(bus_index)]
        for lid, r in t.iterrows():
            loads["bus"].append(bus_index[r.bus_id]); loads["p"].append(_clean(r.p))
            loads["q"].append(_clean(r.q)); loads["name"].append(lid)
    if loads["bus"]:
        pp.create_loads(net, loads["bus"], p_mw=loads["p"], q_mvar=loads["q"], name=loads["name"])

    shunts = {"bus": [], "p": [], "q": [], "name": []}

    def shunt(bus: int, y: complex, v_kv: float, name: str) -> None:
        """Fixed admittance y (S) at voltage v_kv: power at the bus nominal voltage, pandapower load sign."""
        shunts["bus"].append(bus); shunts["p"].append(y.real * v_kv**2)
        shunts["q"].append(-y.imag * v_kv**2); shunts["name"].append(name)

    sh = src.get_shunt_compensators()
    for sid, r in sh[sh.bus_id.isin(bus_index)].iterrows():
        shunt(bus_index[r.bus_id], complex(_clean(r.g), _clean(r.b)), vn_of[r.bus_id], sid)

    line_index, trafo_index, skipped = {}, {}, []
    rows = {k: [] for k in ("f", "t", "r", "x", "c", "g", "name")}
    for lid, r in src.get_lines().iterrows():
        if r.bus1_id not in bus_index or r.bus2_id not in bus_index:
            if r.bus1_id in bus_index or r.bus2_id in bus_index:
                # open at one end: the line is a shunt at the connected end (its own shunt plus the series
                # impedance in series with the far-end shunt)
                near, far = ((1, 2) if r.bus1_id in bus_index else (2, 1))
                ys = 1 / complex(r.r, r.x) if (r.r or r.x) else 0j
                y_near = complex(_clean(r[f"g{near}"]), _clean(r[f"b{near}"]))
                y_far = complex(_clean(r[f"g{far}"]), _clean(r[f"b{far}"]))
                y_eq = y_near + (ys * y_far / (ys + y_far) if ys and (ys + y_far) else 0j)
                bid = r[f"bus{near}_id"]
                if y_eq:
                    shunt(bus_index[bid], y_eq, vn_of[bid], f"{lid}.open")
                skipped.append(lid)      # not a branch of the estimator network, flows are not estimated
            continue
        if abs(vn_of[r.bus1_id] - vn_of[r.bus2_id]) > 1e-6 * vn_of[r.bus1_id]:
            raise UnsupportedForEstimation(f"line {lid} joins buses of different nominal voltage")
        rows["f"].append(bus_index[r.bus1_id]); rows["t"].append(bus_index[r.bus2_id])
        rows["r"].append(float(r.r)); rows["x"].append(float(r.x))
        rows["c"].append((_clean(r.b1) + _clean(r.b2)) / (2 * math.pi * FREQ) * 1e9)
        rows["g"].append((_clean(r.g1) + _clean(r.g2)) * 1e6); rows["name"].append(lid)
    if rows["f"]:
        idx = pp.create_lines_from_parameters(
            net, rows["f"], rows["t"], length_km=1.0, r_ohm_per_km=rows["r"], x_ohm_per_km=rows["x"],
            c_nf_per_km=rows["c"], g_us_per_km=rows["g"], max_i_ka=1e3, name=rows["name"])
        line_index = {lid: int(i) for lid, i in zip(rows["name"], idx)}

    steps = src.get_ratio_tap_changer_steps()
    rtc = src.get_ratio_tap_changers()
    ptc = src.get_phase_tap_changers()
    psteps = src.get_phase_tap_changer_steps()
    tr = {k: [] for k in ("hv", "lv", "vhv", "vlv", "vk", "vkr", "shift", "name")}
    for tid, r in src.get_2_windings_transformers().iterrows():
        if r.bus1_id not in bus_index or r.bus2_id not in bus_index:
            continue
        rho, dr, dx, dg, db = 1.0, 0.0, 0.0, 0.0, 0.0
        if tid in rtc.index:
            st = steps.loc[(tid, int(rtc.loc[tid, "tap"]))]
            rho, dr, dx, dg, db = st.rho, st.r, st.x, st.g, st.b
        shift = 0.0
        if tid in ptc.index:
            st = psteps.loc[(tid, int(ptc.loc[tid, "tap"]))]
            rho, dr, dx, dg, db, shift = rho * st.rho, dr + st.r, dx + st.x, dg + st.g, db + st.b, st.alpha
        rr, xx = r.r * (1 + dr / 100), r.x * (1 + dx / 100)
        gg, bb = _clean(r.g) * (1 + dg / 100), _clean(r.b) * (1 + db / 100)
        zb = r.rated_u2**2 / S_BASE
        tr["hv"].append(bus_index[r.bus1_id]); tr["lv"].append(bus_index[r.bus2_id])
        tr["vhv"].append(r.rated_u1 / rho); tr["vlv"].append(float(r.rated_u2))
        tr["vk"].append(abs(complex(rr, xx)) / zb * 100); tr["vkr"].append(rr / zb * 100)
        tr["shift"].append(SHIFT_SIGN * shift); tr["name"].append(tid)
        if gg or bb:
            # IIDM: V1 -> ideal ratio rho*rated_u2/rated_u1 -> magnetizing admittance (side 2 units) -> series
            # impedance -> V2. As a fixed shunt at bus 1 its voltage at V1 = vn1 is vn1 times the ratio.
            # The sign of b is free (CGMES models carry capacitive values too).
            shunt(bus_index[r.bus1_id], complex(gg, bb), vn_of[r.bus1_id] * rho * r.rated_u2 / r.rated_u1,
                  f"{tid}.mag")
    if tr["hv"]:
        idx = pp.create_transformers_from_parameters(
            net, tr["hv"], tr["lv"], sn_mva=S_BASE, vn_hv_kv=tr["vhv"], vn_lv_kv=tr["vlv"], vkr_percent=tr["vkr"],
            vk_percent=tr["vk"], pfe_kw=0.0, i0_percent=0.0, shift_degree=tr["shift"], name=tr["name"])
        trafo_index = {tid: int(i) for tid, i in zip(tr["name"], idx)}
    if shunts["bus"]:
        # vn_kv given explicitly: pandapower's default lookup fails when two shunts share a bus
        pp.create_shunts(net, shunts["bus"], q_mvar=shunts["q"], p_mw=shunts["p"], name=shunts["name"],
                         vn_kv=[float(net.bus.vn_kv.at[b]) for b in shunts["bus"]])
    return Transfer(net, bus_index, line_index, trafo_index, slack_bus, skipped)


def slack_of(results) -> str | None:
    """Slack bus id of the main component from `pypowsybl.loadflow.run_ac` results."""
    try:
        return results[0].slack_bus_results[0].id
    except (IndexError, AttributeError, TypeError):
        return None


def compare_load_flow(src: pn.Network, tr: Transfer, init: str = "results") -> dict:
    """Run pandapower and compare bus voltages with the solved pypowsybl state.

    With init="results" the solved voltages are the start point, so the check says whether the pypowsybl solution is
    a fixed point of the transferred network (a start from flat voltages may not converge on large networks).
    """
    net, buses = tr.net, src.get_buses()
    if init == "results":
        net.res_bus = net.res_bus.reindex(net.bus.index)
        for bid, i in tr.bus_index.items():
            net.res_bus.loc[i, ["vm_pu", "va_degree"]] = [buses.loc[bid, "v_mag"] / net.bus.vn_kv[i],
                                                          buses.loc[bid, "v_angle"]]
    pp.runpp(net, calculate_voltage_angles=True, init=init, max_iteration=30, numba=False)
    dv, da = [], []
    for bid, i in tr.bus_index.items():
        dv.append(abs(net.res_bus.vm_pu[i] - buses.loc[bid, "v_mag"] / net.bus.vn_kv[i]))
        da.append(abs(net.res_bus.va_degree[i] - buses.loc[bid, "v_angle"]))
    return {"max_dv_pu": float(np.max(dv)), "max_dangle_deg": float(np.max(da)), "buses": len(dv)}
