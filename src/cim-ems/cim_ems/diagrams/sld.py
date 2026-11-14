"""Single-line diagrams from PowSyBl, converted to JSON-SCADA SVG markup (`inkscape:label` JSON).

Conventions (see .agents/skills/svg-scada/SKILL.md and src/svg-display-editor/README.md):
- Switches: the closed and open drawings are shown or hidden from the digital state of the bound point
  (1 = off, 2 = on, 0 invalid, 3 transit, +128 bad quality). `closed_value` of the binding says whether closed is on or off.
- Feeder power/current labels: `get` bindings to the published load flow result points, with a failed-quality colour;
  labels without a result point (generators, loads) are removed instead of showing the static base-case value.
- Labels show element names (`SldParameters(use_name=True)`); bindings use the ids from the diagram metadata.
- Busbars: tooltips with the bus voltage and angle.
Tag references that start with a digit are written as point keys, because the viewer reads them as keys.
"""
from __future__ import annotations

import json
import re
import xml.etree.ElementTree as ET
from dataclasses import dataclass, field
from typing import Callable

import pypowsybl.network as pn

SVG = "http://www.w3.org/2000/svg"
INK = "http://www.inkscape.org/namespaces/inkscape"
ET.register_namespace("", SVG)
ET.register_namespace("inkscape", INK)
LABEL = f"{{{INK}}}label"

# digital state codes -> (closed drawing visible, open drawing visible, uncertain)
CLOSED_ON = {2: (True, False, False), 130: (True, False, True)}
OPEN_ON = {1: (False, True, False), 129: (False, True, True)}
UNKNOWN = {0: (False, True, True), 3: (False, True, True), 128: (False, True, True), 131: (False, True, True)}

SWITCH_CLASSES = ("sld-breaker", "sld-disconnector", "sld-load-break-switch")
FEEDER_QUANTITY = {"ARROW_ACTIVE": ("P", "%.1f"), "ARROW_REACTIVE": ("Q", "%.1f"), "CURRENT": ("I", "%.0f")}
SIDES = {"ONE": 1, "TWO": 2}
AMBER = "#f0ad00"


def decode_id(svg_id: str) -> str:
    """PowSyBl SVG ids: 'id' prefix, '-' written as '_45_'."""
    s = svg_id[2:] if svg_id.startswith("id") else svg_id
    return re.sub(r"_(\d+)_", lambda m: chr(int(m.group(1))), s)


@dataclass
class Report:
    voltage_level: str
    switches_bound: int = 0
    switches_unbound: list[str] = field(default_factory=list)
    values_bound: int = 0
    busbars_bound: int = 0


@dataclass
class TagRef:
    tag: str
    key: float | None = None

    def text(self) -> str:
        """What goes into the tag field: the name, or the point key when the name starts with a digit."""
        if self.tag[:1].isdigit() and self.key is not None:
            return str(int(self.key))
        return self.tag


def _dump(objs: list[dict]) -> str:
    """Label format: the objects of one element as comma separated JSON objects, without brackets."""
    return ",".join(json.dumps(o, separators=(",", ":"), ensure_ascii=False) for o in objs)


def _vis(visible: bool, amber: bool = False) -> str:
    style = "visibility:visible" if visible else "visibility:hidden"
    return "attrib: style=" + style + (f";stroke:{AMBER}" if visible and amber else "")


def switch_labels(ref: str, closed_is_on: bool) -> tuple[str, str]:
    """(label for the closed drawing, label for the open drawing)."""
    closed_codes, open_codes = (CLOSED_ON, OPEN_ON) if closed_is_on else (OPEN_ON, CLOSED_ON)
    table = {}
    for codes, kind in ((closed_codes, "closed"), (open_codes, "open"), (UNKNOWN, "unknown")):
        for code, (_c, _o, amber) in codes.items():
            # codes were written for "closed is on"; swap the roles when closed is off
            table[code] = (kind, amber)
    out = []
    for drawing in ("closed", "open"):
        entries = []
        for code in sorted(table):
            kind, amber = table[code]
            if kind == "unknown":
                visible = drawing == "open"
                amber = True
            else:
                visible = kind == drawing
            entries.append({"data": str(code), "param": _vis(visible, amber), "tag": ref})
        out.append(_dump([{"attr": "color", "list": entries}]))
    return out[0], out[1]


def _value_labels(ref: str) -> str:
    return _dump([
        {"attr": "color", "list": [{"data": "-99999", "param": "#1a1a1a|", "tag": ref},
                                   {"data": "f", "param": "#9a9a9a|", "tag": ref}]},
        {"align": "Right", "attr": "get", "tag": ref, "type": "Good"},
    ])


def convert(
    net: pn.Network,
    voltage_level_id: str,
    switch_ref: Callable[[str], tuple[TagRef, bool] | None],
    result_ref: Callable[[str, int, str], TagRef | None],
    bus_ref: Callable[[str, str], TagRef | None] | None = None,
) -> tuple[str, Report]:
    """Build the display for one voltage level.

    switch_ref(switch_id) -> (TagRef, closed_is_on) or None
    result_ref(equipment_id, side, quantity) -> TagRef or None   (load flow result points)
    bus_ref(bus_id, quantity) -> TagRef or None                  (quantity V or ANG)
    """
    diagram = net.get_single_line_diagram(voltage_level_id, pn.SldParameters(use_name=True))
    meta = json.loads(diagram.metadata)
    root = ET.fromstring(diagram.svg)
    rep = Report(voltage_level_id)
    by_id = {e.get("id"): e for e in root.iter() if e.get("id")}

    for node in meta["nodes"]:
        if node.get("componentType") not in ("BREAKER", "DISCONNECTOR", "LOAD_BREAK_SWITCH"):
            continue
        g = by_id.get(node["id"])
        eid = node.get("equipmentId")
        bound = switch_ref(eid) if eid else None
        if g is None or bound is None:
            rep.switches_unbound.append(eid or node["id"])
            continue
        ref, closed_is_on = bound
        closed_label, open_label = switch_labels(ref.text(), closed_is_on)
        for path in g:
            cls = path.get("class", "")
            if "sld-sw-closed" in cls:
                path.set(LABEL, closed_label)
            elif "sld-sw-open" in cls:
                path.set(LABEL, open_label)
        rep.switches_bound += 1

    for info in meta["feederInfos"]:
        quantity = FEEDER_QUANTITY.get(info["componentType"])
        g = by_id.get(info["id"])
        if quantity is None or g is None:
            continue
        q, fmt = quantity
        side = SIDES.get(info.get("side", ""), 1 if info.get("side") is None else 0)
        ref = result_ref(info["equipmentId"], side, q)
        text = next((t for t in g if t.tag == f"{{{SVG}}}text"), None)
        if ref is None:          # no result point (generator, load): a static number would look live
            for child in list(g):
                g.remove(child)
            continue
        if text is None:
            continue
        text.set(LABEL, _value_labels(ref.text()))
        text.text = None
        for child in list(text):
            text.remove(child)
        ET.SubElement(text, f"{{{SVG}}}tspan").text = fmt
        rep.values_bound += 1

    if bus_ref is not None:
        busbars = net.get_busbar_sections()
        for node in meta["nodes"]:
            if node.get("componentType") != "BUSBAR_SECTION":
                continue
            eid = node.get("equipmentId")
            g = by_id.get(node["id"])
            if g is None or eid not in busbars.index or not busbars.loc[eid, "bus_id"]:
                continue
            bus = busbars.loc[eid, "bus_id"]
            v, a = bus_ref(bus, "V"), bus_ref(bus, "ANG")
            if v is None:
                continue
            lines = [busbars.loc[eid, "name"] or eid, f"V (kV): !EVAL $V('{v.text()}').toFixed(1) !END"]
            if a is not None:
                lines.append(f"Angle (deg): !EVAL $V('{a.text()}').toFixed(2) !END")
            lines += [""] * (5 - len(lines))
            g.set(LABEL, _dump([{"attr": "tooltips", "param": lines, "size": 12, "style": ""}]))
            rep.busbars_bound += 1
    return ET.tostring(root, encoding="unicode", xml_declaration=False), rep
