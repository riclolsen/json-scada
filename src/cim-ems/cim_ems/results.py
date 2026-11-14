"""Turn a solved pypowsybl network into result values for publication."""
from __future__ import annotations

import math
from dataclasses import dataclass

import pypowsybl.network as pn

DEFAULT_FILTER = {"busVoltages": True, "branchFlows": True, "branchLoading": True, "minNominalKv": 0.0}


@dataclass
class ResultValue:
    kind: str            # study kind: LF, SE, CA, SYS
    element_id: str      # mRID / IIDM id; "SYSTEM" for SYS points
    name: str            # CIM name (may be empty or ambiguous)
    side: int            # branch end 1/2, 0 for buses and system points
    quantity: str        # V, VPU, ANG, P, Q, I, LOAD, STATUS, ...
    value: float | None  # None: not calculated (de-energized, NaN)
    unit: str
    group1: str = ""     # substation name
    group2: str = ""     # voltage level name

    @property
    def address(self) -> str:
        return f"{self.element_id}|{self.side}|{self.quantity}"


def _num(x) -> float | None:
    try:
        x = float(x)
    except (TypeError, ValueError):
        return None
    return None if math.isnan(x) or math.isinf(x) else x


def extract_loadflow(net: pn.Network, flt: dict | None = None, kind: str = "LF") -> list[ResultValue]:
    flt = {**DEFAULT_FILTER, **(flt or {})}
    vls = net.get_voltage_levels()[["name", "substation_id", "nominal_v"]]
    subs = net.get_substations()["name"]
    out: list[ResultValue] = []

    def groups(vl_id: str) -> tuple[str, str]:
        v = vls.loc[vl_id]
        return (subs.get(v.substation_id) or v.substation_id or "", v["name"] or vl_id)

    if flt["busVoltages"]:
        for bid, b in net.get_buses().iterrows():
            nominal = vls.loc[b.voltage_level_id, "nominal_v"]
            if nominal < flt["minNominalKv"]:
                continue
            g1, g2 = groups(b.voltage_level_id)
            v = _num(b.v_mag)
            out += [
                ResultValue(kind, bid, b.get("name", "") or "", 0, "V", v, "kV", g1, g2),
                ResultValue(kind, bid, b.get("name", "") or "", 0, "VPU", v / nominal if v is not None else None, "pu", g1, g2),
                ResultValue(kind, bid, b.get("name", "") or "", 0, "ANG", _num(b.v_angle), "deg", g1, g2),
            ]

    if flt["branchFlows"] or flt["branchLoading"]:
        limits = net.get_operational_limits()
        perm = {}
        if len(limits):
            lim = limits.reset_index()
            lim = lim[(lim["type"] == "CURRENT") & (lim["acceptable_duration"] == -1)]
            perm = {(r.element_id, r.side): r.value for r in lim.itertuples()}
        for df in (net.get_lines(), net.get_2_windings_transformers()):
            for eid, br in df.iterrows():
                if vls.loc[br.voltage_level1_id, "nominal_v"] < flt["minNominalKv"]:
                    continue
                g1, g2 = groups(br.voltage_level1_id)
                loads = []
                for side, tag in ((1, "ONE"), (2, "TWO")):
                    connected = bool(br[f"connected{side}"])
                    if flt["branchFlows"]:
                        for q, unit in (("p", "MW"), ("q", "Mvar"), ("i", "A")):
                            out.append(ResultValue(kind, eid, br["name"] or "", side, q.upper(),
                                                   _num(br[f"{q}{side}"]) if connected else None, unit, g1, g2))
                    cur, limit = (_num(br[f"i{side}"]) if connected else None), perm.get((eid, tag))
                    if cur is not None and limit:
                        loads.append(100.0 * cur / limit)
                if flt["branchLoading"] and (loads or (eid, "ONE") in perm or (eid, "TWO") in perm):
                    out.append(ResultValue(kind, eid, br["name"] or "", 0, "LOAD", max(loads) if loads else None, "%", g1, g2))
    return out


def extract_estimation(net: pn.Network, res, flt: dict | None = None) -> list[ResultValue]:
    """Result values (kind SE) of a converged estimate: bus V/VPU/ANG and branch P/Q per end."""
    flt = {**DEFAULT_FILTER, **(flt or {})}
    vls = net.get_voltage_levels()[["name", "substation_id", "nominal_v"]]
    subs = net.get_substations()["name"]
    buses = net.get_buses()
    out: list[ResultValue] = []

    def groups(vl_id):
        v = vls.loc[vl_id]
        return (subs.get(v.substation_id) or v.substation_id or "", v["name"] or vl_id)

    if flt["busVoltages"]:
        for bid, vpu in res.bus_v_pu.items():
            b = buses.loc[bid]
            nominal = vls.loc[b.voltage_level_id, "nominal_v"]
            if nominal < flt["minNominalKv"]:
                continue
            g1, g2 = groups(b.voltage_level_id)
            name = b.get("name", "") or ""
            out += [ResultValue("SE", bid, name, 0, "V", _num(vpu * nominal), "kV", g1, g2),
                    ResultValue("SE", bid, name, 0, "VPU", _num(vpu), "pu", g1, g2),
                    ResultValue("SE", bid, name, 0, "ANG", _num(res.bus_angle_deg[bid]), "deg", g1, g2)]
    if flt["branchFlows"]:
        branches = {**{i: r for i, r in net.get_lines().iterrows()},
                    **{i: r for i, r in net.get_2_windings_transformers().iterrows()}}
        for (eid, side, q), v in res.branch_flows.items():
            br = branches[eid]
            if vls.loc[br.voltage_level1_id, "nominal_v"] < flt["minNominalKv"]:
                continue
            g1, g2 = groups(br.voltage_level1_id)
            out.append(ResultValue("SE", eid, br["name"] or "", side, q, _num(v), "MW" if q == "P" else "Mvar", g1, g2))
    return out


def _sum(df, *cols) -> float:
    if df is None or not len(df):
        return 0.0
    total = 0.0
    for c in cols:
        if c in df:
            v = df[c].sum(skipna=True)
            total += float(v) if v == v else 0.0
    return total


def system_totals(net: pn.Network) -> dict:
    """Active/reactive balance of a solved network (MW, Mvar), for the run record.

    pypowsybl uses the load sign convention: generators show negative p when producing. `otherInjectionsMW` is the
    net injection of HVDC converters, batteries, dangling lines (boundary exchange) and static var compensators;
    `lossesMW` is the balance (generation - load + other injections) and `branchLossesMW` the sum over lines and
    transformers of the power entering at their ends.
    """
    gens, loads = net.get_generators(), net.get_loads()
    generation = -_sum(gens, "p")
    load = _sum(loads, "p")
    other = -sum(_sum(df, "p") for df in (net.get_vsc_converter_stations(), net.get_lcc_converter_stations(),
                                          net.get_batteries(), net.get_dangling_lines(),
                                          net.get_static_var_compensators()))
    branch = _sum(net.get_lines(), "p1", "p2") + _sum(net.get_2_windings_transformers(), "p1", "p2") + \
        _sum(net.get_3_windings_transformers(), "p1", "p2", "p3")
    losses = generation - load + other

    def r(v: float) -> float:
        return round(v, 3)

    return {"generationMW": r(generation), "generationMvar": r(-_sum(gens, "q")), "loadMW": r(load),
            "loadMvar": r(_sum(loads, "q")), "shuntMvar": r(-_sum(net.get_shunt_compensators(), "q")),
            "otherInjectionsMW": r(other), "lossesMW": r(losses), "branchLossesMW": r(branch),
            "lossesPct": r(100.0 * losses / generation) if generation > 0 else None}
