"""Svedala demo: field points of the ReliCapGrid Svedala model, shared by the seed builder and the field simulator.

The demo database (`demo-docker/mongo_seed/files/demo_cim_*.json`) holds the model, an approved binding revision,
the field points of connection SVEDALA_FIELD and the result points of connection SVEDALA_EMS. The field simulator
(`python -m cim_ems.demo_field`) plays the substations: it solves its own copy of the network under a load curve and
writes noisy measurements and switch states to the field points, and executes breaker commands.

Source model: ENTSO-E ReliCapGrid (https://github.com/entsoe/ReliCapGrid), Svedala region by Svenska kraftnät,
CC BY-SA 4.0 (https://creativecommons.org/licenses/by-sa/4.0/). Changed: the common data is merged into the EQ profile.
"""
from __future__ import annotations

import math
from dataclasses import dataclass

import pypowsybl.network as pn

from .bindings.model import Binding
from .bindings.proposal import analog_labels, switch_parts

MODEL_ID = "SVEDALA"
EMS_CONNECTION = (90, "SVEDALA_EMS")
FIELD_CONNECTION = (91, "SVEDALA_FIELD")
SWITCH_KEYS, ANALOG_KEYS, COMMAND_KEYS = 91_000_000, 91_100_000, 91_200_000
UNITS = {"P": "MW", "Q": "Mvar", "V": "kV"}
SIGMA_V_REL, SIGMA_PQ = 0.004, 1.0      # binding standard deviations (same defaults as the proposal)
ATTRIBUTION = ("Model: ENTSO-E ReliCapGrid, Svedala region by Svenska kraftnät, CC BY-SA 4.0 "
               "(common data merged into EQ)")
LICENSE_URL = "https://creativecommons.org/licenses/by-sa/4.0/"
SOURCE_URL = "https://github.com/entsoe/ReliCapGrid"


@dataclass
class FieldPoint:
    key: float
    tag: str
    kind: str              # switch | analog | command
    element_id: str        # switch id, or branch/bus id for analogs
    side: int = 0
    quantity: str = ""
    group1: str = ""       # substation
    group2: str = ""       # voltage level
    name: str = ""         # switch or branch name
    switch_kind: str = ""  # BREAKER, DISCONNECTOR, ...
    supervised_key: float = 0.0   # command: its status point
    command_key: float = 0.0      # breaker status: its command point


def _safe(s: str) -> str:
    return " ".join(str(s).split())


def field_points(net: pn.Network) -> list[FieldPoint]:
    """Switch status, analog and breaker command points, ordered and keyed deterministically."""
    sw = net.get_switches()
    vls = net.get_voltage_levels()[["name", "substation_id"]]
    subs = net.get_substations()["name"]
    parts = switch_parts(net)
    out: list[FieldPoint] = []
    switches = sorted(((f"{sub};{vl};{name}", sid, sub, vl, name) for sid, (sub, vl, name) in parts.items()))
    breakers = []
    for i, (tag, sid, sub, vl, name) in enumerate(switches, start=1):
        fp = FieldPoint(float(SWITCH_KEYS + i), tag, "switch", sid, group1=sub, group2=vl, name=name,
                        switch_kind=str(sw.loc[sid, "kind"]))
        out.append(fp)
        if fp.switch_kind == "BREAKER":
            breakers.append(fp)
    for i, fp in enumerate(breakers, start=1):
        fp.command_key = float(COMMAND_KEYS + i)
        out.append(FieldPoint(fp.command_key, fp.tag + ";K", "command", fp.element_id, group1=fp.group1,
                              group2=fp.group2, name=fp.name, switch_kind=fp.switch_kind, supervised_key=fp.key))

    def where(vl_id: str) -> tuple[str, str]:
        sub = subs.get(vls.loc[vl_id, "substation_id"]) or vls.loc[vl_id, "substation_id"] or ""
        return _safe(sub) or _safe(vls.loc[vl_id, "name"] or vl_id), _safe(vls.loc[vl_id, "name"] or vl_id)

    branch_vl = {}
    for df in (net.get_lines(), net.get_2_windings_transformers()):
        for eid, r in df.iterrows():
            branch_vl[(eid, 1)] = (r.voltage_level1_id, r["name"] or eid)
            branch_vl[(eid, 2)] = (r.voltage_level2_id, r["name"] or eid)
    bus_vl = {}
    for getter in (net.get_busbar_sections, net.get_bus_breaker_view_buses):
        for bid, r in getter().iterrows():
            bus_vl.setdefault(bid, r.voltage_level_id)
    analogs, used = [], set()
    for (eid, side, q) in sorted(analog_labels(net)):
        if q == "V":
            sub, vl = where(bus_vl[eid])
            name, base = "", f"{sub};{vl};V"
        else:
            vl_id, name = branch_vl[(eid, side)]
            sub, vl = where(vl_id)
            name = _safe(name)
            base = f"{sub};{vl};{name};{q}"
        tag, n = base, 1
        while tag in used:
            n += 1
            tag = f"{base};{n}"
        used.add(tag)
        analogs.append((tag, eid, side, q, sub, vl, name))
    for i, (tag, eid, side, q, sub, vl, name) in enumerate(sorted(analogs), start=1):
        out.append(FieldPoint(float(ANALOG_KEYS + i), tag, "analog", eid, side, q, sub, vl, name))
    return out


def bindings_for(net: pn.Network, points: list[FieldPoint]) -> list[Binding]:
    """Approved bindings of the field points (the demo plays an engineer who reviewed a proposal)."""
    nominal = {}
    vls = net.get_voltage_levels()["nominal_v"]
    for getter in (net.get_busbar_sections, net.get_bus_breaker_view_buses):
        for bid, r in getter().iterrows():
            nominal.setdefault(bid, float(vls[r.voltage_level_id]))
    out = []
    for p in points:
        if p.kind == "switch":
            out.append(Binding("switch", p.element_id, p.tag, point_key=p.key, approved=True, confidence=1.0,
                               proposed_from="manual"))
        elif p.kind == "analog":
            sigma = SIGMA_V_REL * nominal.get(p.element_id, 1.0) if p.quantity == "V" else SIGMA_PQ
            out.append(Binding("analog", p.element_id, p.tag, p.quantity, p.side, point_key=p.key,
                               sigma=round(sigma, 4), max_age_seconds=60.0, approved=True, confidence=1.0,
                               proposed_from="manual"))
    return out


class Measurer:
    """True values of analog points from a solved network (None when not calculated)."""

    def __init__(self, net: pn.Network):
        self.net = net
        self.refresh()

    def refresh(self) -> None:
        n = self.net
        self.bus_of: dict[str, str] = {}
        for getter in (n.get_bus_breaker_view_buses, n.get_busbar_sections):
            self.bus_of.update({k: v for k, v in getter()["bus_id"].items() if v})
        self.v = n.get_buses()["v_mag"].to_dict()
        self.flows = {}
        for df in (n.get_lines(), n.get_2_windings_transformers()):
            for col in ("p1", "q1", "p2", "q2"):
                for eid, val in df[col].items():
                    self.flows[(eid, int(col[1]), col[0].upper())] = val

    def value(self, element_id: str, side: int, quantity: str) -> float | None:
        if quantity == "V":
            v = self.v.get(self.bus_of.get(element_id, element_id))
        else:
            v = self.flows.get((element_id, side, quantity))
        return None if v is None or not math.isfinite(v) else float(v)


_D = float
_MAX = 1.7976931348623157e308


def point_document(p: FieldPoint, connection_number: int) -> dict:
    """realtimeData document of a field point, same field set as the demo points of the other connections."""
    digital = p.kind in ("switch", "command")
    if p.kind == "analog":
        what = f"{p.name} {p.quantity}" + (f" side {p.side}" if p.side and p.name else "") if p.name else "Voltage"
    else:
        what = f"{p.name} {p.switch_kind.lower().replace('_', ' ')}" + (" command" if p.kind == "command" else "")
    breaker = p.switch_kind == "BREAKER"
    return {
        "_id": p.key, "tag": p.tag, "type": "digital" if digital else "analog",
        "origin": "command" if p.kind == "command" else "supervised",
        "description": f"{p.group1}~{p.group2}~{what}", "ungroupedDescription": what,
        "group1": p.group1, "group2": p.group2, "group3": "",
        "protocolSourceConnectionNumber": _D(connection_number), "protocolSourceCommonAddress": p.kind,
        "protocolSourceObjectAddress": p.element_id + (f"|{p.side}|{p.quantity}" if p.kind == "analog" else ""),
        "protocolSourceASDU": "command" if p.kind == "command" else ("digital" if digital else "analog"),
        "protocolSourceCommandDuration": _D(0), "protocolSourceCommandUseSBO": False,
        "alarmState": _D(2 if breaker and p.kind == "switch" else -1), "alarmRange": _D(0),
        "priority": _D(1 if breaker else 3), "isEvent": False, "alarmDisabled": False,
        "alarmed": False, "alerted": False, "alertState": "", "annotation": "", "notes": "",
        "stateTextTrue": "CLOSE" if p.kind == "command" else "CLOSED" if digital else "",
        "stateTextFalse": "OPEN" if digital else "",
        "eventTextTrue": "CLOSED" if p.kind == "switch" else "", "eventTextFalse": "OPEN" if p.kind == "switch" else "",
        "unit": UNITS.get(p.quantity, ""),
        "commandBlocked": False, "commandOfSupervised": _D(p.command_key), "supervisedOfCommand": _D(p.supervised_key),
        "commissioningRemarks": "Svedala demo (ReliCapGrid, CC BY-SA 4.0)", "formula": _D(0),
        "frozen": False, "frozenDetectTimeout": _D(0),
        "hiLimit": _MAX, "hihiLimit": _MAX, "hihihiLimit": _MAX,
        "loLimit": -_MAX, "loloLimit": -_MAX, "lololoLimit": -_MAX,
        "historianDeadBand": _D(0), "historianPeriod": _D(0), "hysteresis": _D(0),
        "invalid": p.kind != "command", "invalidDetectTimeout": _D(0 if p.kind == "command" else 300),
        "kconv1": _D(1), "kconv2": _D(0), "location": None, "overflow": False, "parcels": None,
        "protocolDestinations": None, "sourceDataUpdate": None, "substituted": False,
        "timeTag": None, "timeTagAlarm": None, "timeTagAtSource": None, "timeTagAtSourceOk": False,
        "transient": False, "updatesCnt": _D(0), "value": _D(0), "valueDefault": _D(0),
        "valueString": "OPEN" if digital and p.kind == "switch" else "0", "zeroDeadband": _D(0),
    }
