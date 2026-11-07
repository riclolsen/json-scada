"""Build svg-scada displays for the voltage levels of a model, bound to switch tags and load flow result tags."""
from __future__ import annotations

from collections import Counter
from pathlib import Path

import pypowsybl.network as pn

from ..bindings.model import Binding
from ..publish import tag_name
from ..results import ResultValue
from .sld import Report, TagRef, convert


def make_refs(connection: str, bindings: list[Binding], values: list[ResultValue]):
    """Reference callbacks for convert(), using the same tag naming as the result publisher."""
    sw = {b.element_id: b for b in bindings if b.kind == "switch" and b.approved}
    counts = Counter((v.kind, v.name, v.quantity, v.side) for v in values)
    res = {(v.element_id, v.side, v.quantity): TagRef(tag_name(connection, v, counts[(v.kind, v.name, v.quantity, v.side)] > 1)) for v in values}

    def switch_ref(sid):
        b = sw.get(sid)
        return (TagRef(b.tag, b.point_key), b.closed_value >= 0.5) if b else None

    def result_ref(eid, side, q):
        return res.get((eid, side, q))

    def bus_ref(bus_id, q):
        return res.get((bus_id, 0, q))

    return switch_ref, result_ref, bus_ref


def generate_all(net: pn.Network, connection: str, bindings: list[Binding], values: list[ResultValue],
                 out_dir: str | Path, only: list[str] | None = None) -> list[Report]:
    sref, rref, bref = make_refs(connection, bindings, values)
    out = Path(out_dir)
    out.mkdir(parents=True, exist_ok=True)
    reports = []
    for vl in net.get_voltage_levels().index:
        if only and vl not in only:
            continue
        svg, rep = convert(net, vl, sref, rref, bref)
        safe = "".join(c if c.isalnum() or c in "-_" else "_" for c in vl)
        (out / f"cim_{safe}.svg").write_text(svg, encoding="utf-8")
        reports.append(rep)
    return reports
