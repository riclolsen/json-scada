"""Element catalog: every model value a JSON-SCADA point can be bound to, with readable names.

One entry per binding slot, keyed like a `Binding` (kind, element_id, side, quantity):

- `switch`: every switch (breaker, disconnector, ...), state from a digital point;
- `tap`: every two-winding transformer tap changer (ratio or phase), position from an analog point;
- `analog`: P and Q at each end of lines and two-winding transformers, V at the first busbar section of each voltage
  level (first bus in a bus-breaker model), the same slots the name matcher proposes for.

`label` is the text the name matcher compares with tag names (`proposal.switch_labels` / `analog_labels`), so a
reviewer or an AI assistant sees the model as the matcher does. Entries whose label is shared by another slot of the
same quantity (parallel circuits with one name) carry `ambiguousLabel`.
"""
from __future__ import annotations

import math

import pypowsybl.network as pn

from .proposal import analog_labels, switch_parts

UNITS = {"V": "kV", "P": "MW", "Q": "Mvar", "I": "A"}


def _num(v) -> float | None:
    try:
        f = float(v)
    except (TypeError, ValueError):
        return None
    return None if math.isnan(f) else f


def element_catalog(net: pn.Network) -> list[dict]:
    vls = net.get_voltage_levels()[["name", "substation_id", "nominal_v"]]
    subs = net.get_substations()["name"]

    def sub_of(vl: str) -> str:
        return subs.get(vls.loc[vl, "substation_id"]) or vls.loc[vl, "substation_id"] or ""

    def vl_name(vl: str) -> str:
        return vls.loc[vl, "name"] or vl

    def kv(vl: str) -> float | None:
        return _num(vls.loc[vl, "nominal_v"])

    out: list[dict] = []
    sw = net.get_switches()
    for sid, (sub, vl, name) in switch_parts(net).items():
        r = sw.loc[sid]
        out.append({"kind": "switch", "element_id": sid, "side": 0, "quantity": "", "elementType": str(r.kind),
                    "name": name, "substation": sub, "voltageLevel": vl, "nominalKv": kv(r.voltage_level_id),
                    "label": f"{sub} {vl} {name}", "normallyOpen": bool(r.open)})

    t2 = net.get_2_windings_transformers()
    for df, ttype in ((net.get_ratio_tap_changers(), "ratio-tap"), (net.get_phase_tap_changers(), "phase-tap")):
        for tid, r in df.iterrows():
            if tid not in t2.index:
                continue
            t = t2.loc[tid]
            vl1 = t.voltage_level1_id
            name = t["name"] or tid
            out.append({"kind": "tap", "element_id": tid, "side": 0, "quantity": "", "elementType": ttype,
                        "name": name, "substation": sub_of(vl1), "voltageLevel": vl_name(vl1), "nominalKv": kv(vl1),
                        "label": f"{sub_of(vl1)} {vl_name(vl1)} {name}",
                        "tapRange": [int(r.low_tap), int(r.high_tap)], "tap": int(r.tap)})

    branch: dict[str, tuple[str, object]] = {}
    for df, btype in ((net.get_lines(), "line"), (net.get_2_windings_transformers(), "transformer")):
        for eid, r in df.iterrows():
            branch[eid] = (btype, r)
    bus_vl: dict[str, tuple[str, str]] = {}
    for df, btype in ((net.get_busbar_sections(), "busbar"), (net.get_bus_breaker_view_buses(), "bus")):
        for eid, r in df.iterrows():
            bus_vl.setdefault(eid, (btype, r.voltage_level_id))

    labels = analog_labels(net)
    count: dict[tuple[str, str], int] = {}
    for (_, _, q), label in labels.items():
        count[(q, label)] = count.get((q, label), 0) + 1
    for (eid, side, q), label in labels.items():
        e = {"kind": "analog", "element_id": eid, "side": side, "quantity": q, "unit": UNITS[q], "label": label}
        if eid in branch:
            btype, r = branch[eid]
            vl, other = (r.voltage_level1_id, r.voltage_level2_id) if side == 1 else \
                (r.voltage_level2_id, r.voltage_level1_id)
            e.update(elementType=btype, name=r["name"] or eid, substation=sub_of(vl), voltageLevel=vl_name(vl),
                     nominalKv=kv(vl), otherEnd={"substation": sub_of(other), "voltageLevel": vl_name(other)})
        else:
            btype, vl = bus_vl.get(eid, ("bus", ""))
            e.update(elementType=btype, name=vl_name(vl) if vl else eid, substation=sub_of(vl) if vl else "",
                     voltageLevel=vl_name(vl) if vl else "", nominalKv=kv(vl) if vl else None)
        if count[(q, label)] > 1:
            e["ambiguousLabel"] = True
        out.append(e)
    return out
