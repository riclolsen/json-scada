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
    bus_index: dict[str, int] = {}
    for bid, row in main.iterrows():
        vn = float(vls[row.voltage_level_id])
        va = _clean(row.v_angle)
        vm = _clean(row.v_mag, vn)
        bus_index[bid] = pp.create_bus(net, vn_kv=vn, name=bid, index=len(bus_index))
        net.bus.loc[bus_index[bid], ["vm_init", "va_init"]] = vm / vn, va

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
    pp.create_ext_grid(net, bus=sb, vm_pu=main.loc[slack_bus, "v_mag"] / net.bus.vn_kv[sb],
                       va_degree=_clean(main.loc[slack_bus, "v_angle"]), name=slack_gen or slack_bus)
    for gid, g in gens.iterrows():
        if gid == slack_gen:
            continue
        pp.create_sgen(net, bus=bus_index[g.bus_id], p_mw=-_clean(g.p), q_mvar=-_clean(g.q), name=gid)

    for table in (src.get_loads(), src.get_dangling_lines()):
        t = table[table.bus_id.isin(bus_index)]
        for lid, r in t.iterrows():
            pp.create_load(net, bus=bus_index[r.bus_id], p_mw=_clean(r.p), q_mvar=_clean(r.q), name=lid)
    sh = src.get_shunt_compensators()
    for sid, r in sh[sh.bus_id.isin(bus_index)].iterrows():
        vn = net.bus.vn_kv[bus_index[r.bus_id]]
        # fixed admittance: p and q at nominal voltage, shunt convention (q negative = capacitive in pandapower sign)
        pp.create_shunt(net, bus=bus_index[r.bus_id], q_mvar=-r.b * vn**2,
                        p_mw=r.g * vn**2, name=sid)
    line_index, trafo_index, skipped = {}, {}, []

    lines = src.get_lines()
    for lid, r in lines.iterrows():
        if r.bus1_id not in bus_index or r.bus2_id not in bus_index:
            if r.bus1_id in bus_index or r.bus2_id in bus_index:
                # open at one end: the line is a shunt at the connected end (its own shunt plus the series
                # impedance in series with the far-end shunt)
                near, far = ((1, 2) if r.bus1_id in bus_index else (2, 1))
                ys = 1 / complex(r.r, r.x) if (r.r or r.x) else 0j
                y_near = complex(_clean(r[f"g{near}"]), _clean(r[f"b{near}"]))
                y_far = complex(_clean(r[f"g{far}"]), _clean(r[f"b{far}"]))
                y_eq = y_near + (ys * y_far / (ys + y_far) if ys and (ys + y_far) else 0j)
                b = bus_index[r[f"bus{near}_id"]]
                vn = net.bus.vn_kv[b]
                if y_eq:
                    pp.create_shunt(net, bus=b, p_mw=y_eq.real * vn**2, q_mvar=-y_eq.imag * vn**2, name=f"{lid}.open")
                skipped.append(lid)      # not a branch of the estimator network, flows are not estimated
            continue
        vn = net.bus.vn_kv[bus_index[r.bus1_id]]
        if abs(vn - net.bus.vn_kv[bus_index[r.bus2_id]]) > 1e-6 * vn:
            raise UnsupportedForEstimation(f"line {lid} joins buses of different nominal voltage")
        b_tot = _clean(r.b1) + _clean(r.b2)
        pp.create_line_from_parameters(
            net, bus_index[r.bus1_id], bus_index[r.bus2_id], length_km=1.0, r_ohm_per_km=r.r, x_ohm_per_km=r.x,
            c_nf_per_km=b_tot / (2 * math.pi * FREQ) * 1e9, g_us_per_km=(_clean(r.g1) + _clean(r.g2)) * 1e6,
            max_i_ka=1e3, name=lid)
        line_index[lid] = int(net.line.index[-1])

    steps = src.get_ratio_tap_changer_steps()
    rtc = src.get_ratio_tap_changers()
    ptc = src.get_phase_tap_changers()
    psteps = src.get_phase_tap_changer_steps()
    for tid, r in src.get_2_windings_transformers().iterrows():
        if r.bus1_id not in bus_index or r.bus2_id not in bus_index:
            continue
        rho, dr, dx, dg, db = 1.0, 0.0, 0.0, 0.0, 0.0
        if tid in rtc.index:
            s = steps.loc[(tid, int(rtc.loc[tid, "tap"]))]
            rho, dr, dx, dg, db = s.rho, s.r, s.x, s.g, s.b
        shift = 0.0
        if tid in ptc.index:
            s = psteps.loc[(tid, int(ptc.loc[tid, "tap"]))]
            rho, dr, dx, dg, db, shift = rho * s.rho, dr + s.r, dx + s.x, dg + s.g, db + s.b, s.alpha
        rr, xx = r.r * (1 + dr / 100), r.x * (1 + dx / 100)
        gg, bb = _clean(r.g) * (1 + dg / 100), _clean(r.b) * (1 + db / 100)
        zb = r.rated_u2**2 / S_BASE
        z = abs(complex(rr, xx)) / zb
        pp.create_transformer_from_parameters(
            net, hv_bus=bus_index[r.bus1_id], lv_bus=bus_index[r.bus2_id], sn_mva=S_BASE,
            vn_hv_kv=r.rated_u1 / rho, vn_lv_kv=r.rated_u2, vk_percent=z * 100, vkr_percent=rr / zb * 100,
            pfe_kw=0.0, i0_percent=0.0, shift_degree=SHIFT_SIGN * shift, name=tid)
        trafo_index[tid] = int(net.trafo.index[-1])
        if gg or bb:
            # IIDM: V1 -> ideal ratio rho*rated_u2/rated_u1 -> magnetizing admittance (side 2 units) -> series
            # impedance -> V2. As a fixed shunt at bus 1: Q = -b (V1 ratio)^2, so at V1 = vn1 the shunt power is
            # -b (vn1 ratio)^2. The sign of b is free (CGMES models carry capacitive values too).
            v_mag = net.bus.vn_kv[bus_index[r.bus1_id]] * rho * r.rated_u2 / r.rated_u1
            pp.create_shunt(net, bus=bus_index[r.bus1_id], p_mw=gg * v_mag**2, q_mvar=-bb * v_mag**2,
                            name=f"{tid}.mag")
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
