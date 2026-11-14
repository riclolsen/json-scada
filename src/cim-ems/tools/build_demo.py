"""Build the Svedala demo: seed files for the demo database and the svg-scada displays.

  python tools/build_demo.py [--package tests/data/conformity/relicap_Svedala.zip]
                             [--seed ../../demo-docker/mongo_seed/files] [--svg ../../svg]

Needs the test dependencies (`pip install .[test]`, for mongomock) and the package fetched with
`python tools/fetch_test_models.py --relicap`. The output is deterministic (fixed dates and ids), so
rebuilding with the same package and engine versions changes no file.

The driver's own code builds the content in an in-memory database: the model is imported, validated and
activated, a binding revision is saved, the field points are created and one study cycle (load flow, state
estimation, N-1) creates the result points. Those collections are then written as mongoimport files:

  demo_cim_models.json, demo_cim_bindings.json, demo_cim_data.json (realtimeData),
  demo_cim_artifacts_files.json, demo_cim_artifacts_chunks.json (GridFS bucket cimArtifacts),
  plus the CIM-EMS instance and the two connections appended to demo_instances.json and
  demo_connections*.json (earlier Svedala lines are replaced).

Displays: svg/svedala-overview.svg (network-area diagram) and one svg/svedala_<substation>.svg per substation.
"""
from __future__ import annotations

import argparse
import copy
import hashlib
import json
import re
import sys
import unicodedata
import xml.etree.ElementTree as ET
from datetime import datetime, timezone
from pathlib import Path

HERE = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(HERE))

import mongomock  # noqa: E402
import mongomock.gridfs  # noqa: E402
from bson import ObjectId, json_util  # noqa: E402
from mongomock.collection import BulkOperationBuilder  # noqa: E402

mongomock.gridfs.enable_gridfs_integration()
_orig_add_update = BulkOperationBuilder.add_update
BulkOperationBuilder.add_update = lambda self, *a, sort=None, **k: _orig_add_update(self, *a, **k)

import pypowsybl.loadflow as lf  # noqa: E402

from cim_ems import demo  # noqa: E402
from cim_ems.bindings import store as bstore  # noqa: E402
from cim_ems.cycle import StudyCycle  # noqa: E402
from cim_ems.diagrams.generate import make_refs  # noqa: E402
from cim_ems.diagrams.sld import INK, LABEL, SVG, TagRef, _dump, _value_labels, convert  # noqa: E402
from cim_ems.models import store as mstore  # noqa: E402
from cim_ems.models.importer import import_cgmes, validate  # noqa: E402
from cim_ems.mongo import ARTIFACT_BUCKET, BINDINGS, MODELS, ensure_indexes  # noqa: E402
from cim_ems.publish import ResultPublisher  # noqa: E402
from cim_ems.redundancy import Redundancy  # noqa: E402
from cim_ems.results import extract_loadflow  # noqa: E402
from cim_ems.snapshot import PointState  # noqa: E402

DEMO_DATE = datetime(2026, 11, 1, tzinfo=timezone.utc)
EMS_NUM, EMS_NAME = demo.EMS_CONNECTION
FIELD_NUM, FIELD_NAME = demo.FIELD_CONNECTION
OVERVIEW = "svedala-overview"

EMS_CONNECTION_DOC = {
    "protocolDriver": "CIM-EMS", "protocolDriverInstanceNumber": 1.0, "protocolConnectionNumber": float(EMS_NUM),
    "name": EMS_NAME, "description": "Svedala demo EMS: load flow, state estimation and N-1 (ReliCapGrid model)",
    "enabled": True, "commandsEnabled": False, "stats": None,
    "cimModelId": demo.MODEL_ID, "cimModelRevision": None, "cimBindingRevision": None,
    "cycleSeconds": 30.0, "resultMaxAgeSeconds": 180.0, "switchHoldSeconds": 300.0, "snapshotMaxSkewSeconds": 10.0,
    "loadFlowParameters": {"distributedSlack": True, "balanceType": "PROPORTIONAL_TO_GENERATION_P_MAX",
                           "voltageInitMode": "UNIFORM_VALUES", "transformerVoltageControlOn": False},
    "publishFilter": {"busVoltages": True, "branchFlows": True, "branchLoading": True, "minNominalKv": 0.0},
    "runStateEstimation": True, "estimatorParameters": {"badDataThreshold": 4.0},
    "runContingency": True, "contingencyListId": "AUTO_N1", "contingencyIntervalSeconds": 300.0,
    "contingencyMinGapSeconds": 10.0, "contingencyMinKv": 100.0, "contingencyPublishLimit": 200.0,
    "contingencyThreads": 1.0, "autoCreateTags": True,
}
FIELD_CONNECTION_DOC = {
    "protocolDriver": "UNDEFINED", "protocolDriverInstanceNumber": 1.0, "protocolConnectionNumber": float(FIELD_NUM),
    "name": FIELD_NAME, "description": "Svedala demo field points, written by the simulator python -m cim_ems.demo_field",
    "enabled": True, "commandsEnabled": True, "stats": None,
}
INSTANCE_DOC = {
    "protocolDriver": "CIM-EMS", "protocolDriverInstanceNumber": 1.0, "enabled": True, "logLevel": 1.0,
    "nodeNames": [], "activeNodeName": "", "activeNodeKeepAliveTimeTag": DEMO_DATE,
    "keepProtocolRunningWhileInactive": False,
}


def oid(name: str) -> ObjectId:
    return ObjectId(hashlib.sha256(f"svedala-demo:{name}".encode()).hexdigest()[:24])


class _Cache:
    def __init__(self, points):
        self._p = points

    def points(self):
        return self._p


# ---------------------------------------------------------------- database content

def build_database(package: Path):
    db = mongomock.MongoClient()["json_scada"]
    ensure_indexes(db)
    net = import_cgmes(package)
    report = validate(net)            # solves the base case
    if not report.ok:
        raise SystemExit(f"model not valid: {report.errors}")
    model = mstore.register_import(db, package, demo.MODEL_ID, report, user="demo")
    mstore.activate(db, demo.MODEL_ID, model["revision"])
    model = mstore.get_active(db, demo.MODEL_ID)

    points = demo.field_points(net)
    bindings = demo.bindings_for(net, points)
    bstore.save_revision(db, demo.MODEL_ID, bindings, user="demo")
    db["realtimeData"].insert_many([demo.point_document(p, FIELD_NUM) for p in points])

    db["protocolDriverInstances"].insert_one({**INSTANCE_DOC, "nodeNames": ["build"]})
    red = Redundancy(db, "build", 1)
    red.tick()
    pub = ResultPublisher(db, red, EMS_NUM, EMS_NAME, EMS_CONNECTION_DOC["resultMaxAgeSeconds"])
    meas = demo.Measurer(net)
    sw = net.get_switches()
    now = datetime.now(timezone.utc)
    states = {}
    for b in bindings:
        if b.kind == "switch":
            v = 0.0 if sw.loc[b.element_id, "open"] else 1.0
        else:
            v = meas.value(b.element_id, b.side, b.quantity)
            if v is None:
                continue
        states[b.tag] = PointState(v, time_tag=now, time_tag_at_source=now)
    cycle = StudyCycle(db, dict(EMS_CONNECTION_DOC), model, mstore.load_network(db, model), bindings,
                       _Cache(states), pub, red, "build")
    run = cycle.run_once("start")
    if run["status"] != "converged" or run["summary"].get("estimation", {}).get("status") != "converged":
        raise SystemExit(f"demo study did not converge: {run['status']} {run['summary']}")
    return db, net, bindings, run


def _normalize(doc, ids: dict):
    """Fixed dates and deterministic ObjectIds (mapped in order of appearance)."""
    if isinstance(doc, dict):
        return {k: _normalize(v, ids) for k, v in doc.items()}
    if isinstance(doc, list):
        return [_normalize(v, ids) for v in doc]
    if isinstance(doc, datetime):
        return DEMO_DATE
    if isinstance(doc, ObjectId):
        if doc not in ids:
            ids[doc] = oid(f"id{len(ids)}")
        return ids[doc]
    return doc


def _write_lines(path: Path, docs: list[dict]) -> None:
    opts = json_util.JSONOptions(json_mode=json_util.JSONMode.RELAXED, tz_aware=True, tzinfo=timezone.utc)
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        for d in docs:
            f.write(json_util.dumps(d, json_options=opts, ensure_ascii=False, separators=(",", ":")) + "\n")


def _replace_lines(path: Path, docs: list[dict], is_ours) -> None:
    """Rewrite a shared seed file: keep foreign lines, replace the Svedala ones."""
    keep = [ln for ln in path.read_text(encoding="utf-8").splitlines() if ln.strip() and not is_ours(json.loads(ln))]
    path.write_text("\n".join(keep) + "\n", encoding="utf-8", newline="\n")
    tmp = path.with_suffix(".tmp")
    _write_lines(tmp, docs)
    with open(path, "a", encoding="utf-8", newline="\n") as f:
        f.write(tmp.read_text(encoding="utf-8"))
    tmp.unlink()


def write_seed(db, out: Path) -> dict:
    ids: dict = {}
    models = [_normalize(d, ids) for d in db[MODELS].find({}, sort=[("revision", 1)])]
    files = [_normalize(d, ids) for d in db[f"{ARTIFACT_BUCKET}.files"].find({"kind": "cgmes-package"})]
    pkg_ids = [d["_id"] for d in db[f"{ARTIFACT_BUCKET}.files"].find({"kind": "cgmes-package"})]
    chunks = [_normalize(d, ids) for d in db[f"{ARTIFACT_BUCKET}.chunks"].find(
        {"files_id": {"$in": pkg_ids}}, sort=[("files_id", 1), ("n", 1)])]
    bindings = [_normalize(d, ids) for d in db[BINDINGS].find({}, sort=[("bindingRevision", 1), ("tag", 1)])]
    rtd = []
    for d in db["realtimeData"].find({}, sort=[("_id", 1)]):
        d.update(sourceDataUpdate=None, timeTag=None, timeTagAtSource=None, timeTagAtSourceOk=False,
                 invalid=d.get("origin") != "command", value=0.0, valueString=d.get("valueString", "0"))
        rtd.append(_normalize(d, ids))
    _write_lines(out / "demo_cim_models.json", models)
    _write_lines(out / "demo_cim_artifacts_files.json", files)
    _write_lines(out / "demo_cim_artifacts_chunks.json", chunks)
    _write_lines(out / "demo_cim_bindings.json", bindings)
    _write_lines(out / "demo_cim_data.json", rtd)

    ours_conn = lambda d: d.get("protocolConnectionNumber") in (EMS_NUM, FIELD_NUM)   # noqa: E731
    conns = [{"_id": oid("connection-ems"), **EMS_CONNECTION_DOC}, {"_id": oid("connection-field"), **FIELD_CONNECTION_DOC}]
    for name in ("demo_connections.json", "demo_connections_linux.json"):
        _replace_lines(out / name, conns, ours_conn)
    _replace_lines(out / "demo_instances.json", [{"_id": oid("instance-1"), **INSTANCE_DOC}],
                   lambda d: d.get("protocolDriver") == "CIM-EMS")
    return {"models": len(models), "chunks": len(chunks), "bindings": len(bindings), "realtimeData": len(rtd)}


# ---------------------------------------------------------------- displays

def ascii_name(s: str) -> str:
    s = unicodedata.normalize("NFKD", s).encode("ascii", "ignore").decode()
    return re.sub(r"[^A-Za-z0-9]+", "_", s).strip("_")


def _text(parent, x, y, text, size=16, anchor="start", fill="#1a1a1a", weight="normal", label=None):
    t = ET.SubElement(parent, f"{{{SVG}}}text", {
        "x": f"{x:.1f}", "y": f"{y:.1f}",
        "style": f"font-family:sans-serif;font-size:{size}px;font-weight:{weight};fill:{fill};text-anchor:{anchor}"})
    if label:
        t.set(LABEL, label)
    ET.SubElement(t, f"{{{SVG}}}tspan").text = text
    return t


def _link(screen: str) -> str:
    return _dump([{"attr": "open", "istag": 0, "src": screen, "type": "_self", "height": 0, "width": 0, "x": 0, "y": 0}])


def _frame(root, title: str, top: float, bottom: float, links: list[tuple[str, str]], size=16):
    """Title and screen links above, attribution below the drawing; grows the view box. `top`/`bottom` in rows."""
    x, y, w, h = (float(v) for v in root.get("viewBox").split())
    row = size * 1.8
    x0, y0, h0 = x, y - row * top, h + row * (top + bottom)
    root.set("viewBox", f"{x0:.2f} {y0:.2f} {w:.2f} {h0:.2f}")
    if root.get("height") and root.get("width"):
        root.set("height", f"{float(root.get('width')) * h0 / w:.1f}")
    root.insert(1, ET.Element(f"{{{SVG}}}rect", {"x": f"{x0:.2f}", "y": f"{y0:.2f}", "width": f"{w:.2f}",
                                                    "height": f"{h0:.2f}", "style": "fill:#ffffff;stroke:none"}))
    g = ET.SubElement(root, f"{{{SVG}}}g", {"id": "svedala_frame"})
    _text(g, x0 + size, y0 + row, title, size * 1.3, weight="bold")
    cx = x0 + size
    for caption, screen in links:
        _text(g, cx, y0 + row * 2, caption, size, fill="#0b62a8", label=_link(screen))
        cx += size * 0.62 * (len(caption) + 3)
    note = f"{demo.ATTRIBUTION}. {demo.SOURCE_URL} - {demo.LICENSE_URL}"
    small = min(size * 0.7, (w - 2 * size) / (0.55 * len(note)))   # fit the width of narrow displays
    _text(g, x0 + size, y0 + h0 - small, note, small, fill="#666666")
    return g


def substation_displays(net, bindings, values, svg_dir: Path) -> tuple[dict[str, str], list]:
    """One display per substation, and one per voltage level without substation. Returns vl id -> screen name."""
    sref, rref, bref = make_refs(EMS_NAME, bindings, values)
    subs = net.get_substations()
    vls = net.get_voltage_levels()
    screens, reports, used = {}, [], set()
    targets = [(sid, subs.loc[sid, "name"] or sid, list(vls.index[vls.substation_id == sid])) for sid in subs.index]
    targets += [(vl, vls.loc[vl, "name"] or vl, [vl]) for vl in vls.index[vls.substation_id.fillna("") == ""]]
    for cid, name, members in sorted(targets, key=lambda t: t[1]):
        screen = f"svedala_{ascii_name(name)}"
        while screen in used:
            screen += "_"
        used.add(screen)
        svg, rep = convert(net, cid, sref, rref, bref)
        root = ET.fromstring(svg)
        _frame(root, f"Svedala - {name}", 2.4, 1.2, [("<< Network overview", OVERVIEW)], size=16)
        (svg_dir / f"{screen}.svg").write_text(_serialize(root), encoding="utf-8", newline="\n")
        for vl in members:
            screens[vl] = screen
        reports.append((screen, rep))
    return screens, reports


def _serialize(root) -> str:
    ET.register_namespace("inkscape", INK)
    for e in root.iter():          # drop the indentation of the generated drawing (about a third of the size)
        if e.tail is not None and not e.tail.strip():
            e.tail = None
        if e.text is not None and not e.text.strip() and len(e):
            e.text = None
    return '<?xml version="1.0" encoding="UTF-8"?>\n' + ET.tostring(root, encoding="unicode") + "\n"


def overview_display(net, bindings, values, screens: dict[str, str], svg_dir: Path) -> dict:
    """Network-area diagram: live flows on the branch ends, live voltages per bus, links to the substations."""
    _, rref, bref = make_refs(EMS_NAME, bindings, values)
    lines, t2 = net.get_lines(), net.get_2_windings_transformers()
    linked = set(lines.voltage_level1_id) | set(lines.voltage_level2_id) | set(t2.voltage_level1_id) | set(t2.voltage_level2_id)
    # voltage levels without branches (boundary points) would be laid out far away from the grid
    diagram = net.get_network_area_diagram(sorted(linked), depth=0)
    meta = json.loads(diagram.metadata)
    root = ET.fromstring(diagram.svg)
    parent = {c: p for p in root.iter() for c in p}
    by_id = {e.get("id"): e for e in root.iter() if e.get("id")}
    vls = net.get_voltage_levels()
    stats = {"flows": 0, "voltages": 0, "links": 0}

    for edge in meta["edges"]:
        for side, key in ((1, "edgeInfo1"), (2, "edgeInfo2")):
            info = edge.get(key)
            g = by_id.get(info["svgId"]) if info else None
            ref = rref(edge.get("equipmentId", ""), side, "P") if g is not None else None
            if ref is None:
                continue
            for arrow in [c for c in g if c.tag == f"{{{SVG}}}path"]:   # static direction: the sign tells it
                g.remove(arrow)
            text = next((t for t in g if t.tag == f"{{{SVG}}}text"), None)
            if text is None:
                continue
            text.set(LABEL, _value_labels(ref.text()))
            text.set("style", (text.get("style", "") + ";font-size:55px;font-family:sans-serif").lstrip(";"))
            text.text = None
            for c in list(text):
                text.remove(c)
            ET.SubElement(text, f"{{{SVG}}}tspan").text = "%.0f"
            stats["flows"] += 1

    buses_of: dict[str, list] = {}
    for bn in meta["busNodes"]:
        buses_of.setdefault(bn["vlNode"], []).append(bn)
        circle = by_id.get(bn["svgId"])
        v, a = bref(bn["equipmentId"], "V"), bref(bn["equipmentId"], "ANG")
        if circle is not None and v is not None:
            lines = [bn["equipmentId"], f"V (kV): !EVAL $V('{v.text()}').toFixed(1) !END"]
            if a is not None:
                lines.append(f"Angle (deg): !EVAL $V('{a.text()}').toFixed(2) !END")
            circle.set(LABEL, _dump([{"attr": "tooltips", "param": lines + [""] * (5 - len(lines)), "size": 12, "style": ""}]))

    text_of = {t["vlNode"]: t for t in meta["textNodes"]}
    for node in meta["nodes"]:
        vl = node["equipmentId"]
        screen = screens.get(vl)
        g = by_id.get(node["svgId"])
        if g is not None and screen:
            g.set(LABEL, _link(screen))
            stats["links"] += 1
        tn = text_of.get(node["svgId"])
        fo = by_id.get(tn["svgId"]) if tn else None
        if fo is None:
            continue
        x, y = float(fo.get("x")), float(fo.get("y"))
        box = ET.Element(f"{{{SVG}}}g", {"id": fo.get("id")})
        p = parent[fo]
        p.insert(list(p).index(fo), box)
        p.remove(fo)
        name = vls.loc[vl, "name"] or vl
        _text(box, x, y + 70, name, 70, weight="bold", label=_link(screen) if screen else None)
        for i, bn in enumerate(sorted(buses_of.get(node["svgId"], []), key=lambda b: b["index"])):
            ref = bref(bn["equipmentId"], "V")
            if ref is None:
                continue
            t = _text(box, x + 10, y + 140 + 65 * i, "%.1f kV", 58, fill="#37474f", label=_value_labels(ref.text()))
            t.find(f"{{{SVG}}}tspan").text = "%.1f kV"
            stats["voltages"] += 1

    status = [("LF", f"{EMS_NAME};SYS;LF;STATUS"), ("SE", f"{EMS_NAME};SYS;SE;STATUS"), ("CA", f"{EMS_NAME};SYS;CA;STATUS")]
    size = 120
    frame = _frame(root, "Svedala - network overview (load flow, MW at branch ends, kV at buses)", 2.4, 1.2, [],
                   size=size)
    x, y, _w, _h = (float(v) for v in root.get("viewBox").split())
    row_y = y + size * 1.8 * 2
    for i, (caption, tag) in enumerate(status):
        _text(frame, x + size + 1500 * i, row_y, f"{caption} status:", 90, fill="#37474f")
        t = _text(frame, x + size + 1500 * i + 820, row_y, "", 90, weight="bold",
                  label=_dump([{"attr": "get", "tag": tag, "align": "Left", "type": "Good"}]))
        t.find(f"{{{SVG}}}tspan").text = "%.0f"
    _text(frame, x + size + 4500, row_y, "0 = converged / done. Click a substation to open its diagram.",
          70, fill="#666666")
    (svg_dir / f"{OVERVIEW}.svg").write_text(_serialize(root), encoding="utf-8", newline="\n")
    return stats


def check_tags(svg_dir: Path, known: set[str]) -> list[str]:
    """Tag references of the generated displays that are not in the seed."""
    missing = set()
    for f in sorted(svg_dir.glob("svedala*.svg")):
        for m in re.finditer(r'inkscape:label="([^"]*)"', f.read_text(encoding="utf-8")):
            lab = m.group(1).replace("&quot;", '"').replace("&amp;", "&")
            for t in re.findall(r'"tag":"([^"]+)"', lab) + re.findall(r"\$V\('([^']+)'\)", lab):
                if t not in known and not t.replace(".", "").isdigit() and not t.startswith("%"):
                    missing.add(t)
    return sorted(missing)


def main(argv=None):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--package", default=str(HERE / "tests/data/conformity/relicap_Svedala.zip"))
    ap.add_argument("--seed", default=str(HERE.parents[1] / "demo-docker/mongo_seed/files"))
    ap.add_argument("--svg", default=str(HERE.parents[1] / "svg"))
    a = ap.parse_args(argv)
    package = Path(a.package)
    if not package.exists():
        raise SystemExit(f"{package} not found: run python tools/fetch_test_models.py --relicap")
    db, net, bindings, run = build_database(package)
    print("study:", run["status"], {k: run["summary"].get(k) for k in ("buses", "islands", "iterations")},
          "SE", run["summary"]["estimation"].get("status"), "CA", run["summary"]["contingency"].get("status"))
    print("seed:", write_seed(db, Path(a.seed)))

    svg_dir = Path(a.svg)
    for old in svg_dir.glob("svedala*.svg"):
        old.unlink()
    lf.run_ac(net, lf.Parameters(distributed_slack=True))
    values = extract_loadflow(net)
    screens, reports = substation_displays(net, bindings, values, svg_dir)
    unbound = sum(len(r.switches_unbound) for _, r in reports)
    print(f"substation displays: {len(reports)}, switches bound {sum(r.switches_bound for _, r in reports)}, "
          f"unbound {unbound}, values {sum(r.values_bound for _, r in reports)}")
    print("overview:", overview_display(net, bindings, values, screens, svg_dir))
    known = {d["tag"] for d in db["realtimeData"].find({}, {"tag": 1})}
    missing = check_tags(svg_dir, known)
    if missing:
        raise SystemExit(f"{len(missing)} display tag(s) not in the seed, e.g. {missing[:5]}")


if __name__ == "__main__":
    main()
