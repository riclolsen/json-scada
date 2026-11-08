"""Study cycle with state estimation on Galia: analog bindings from synthetic telemetry, SE result points."""
import os
from datetime import datetime, timezone
from pathlib import Path

import mongomock
import numpy as np
import pypowsybl.loadflow as lf
import pytest

from cim_ems.bindings.model import Binding
from cim_ems.cycle import StudyCycle
from cim_ems.estimation import synthetic as S
from cim_ems.estimation import transfer as T
from cim_ems.models.importer import import_cgmes
from cim_ems.publish import ResultPublisher
from cim_ems.redundancy import Redundancy
from cim_ems.snapshot import PointState

DATA = Path(os.environ.get("CIM_EMS_TEST_DATA", Path(__file__).resolve().parents[1] / "data/conformity"))
GALIA = DATA / "relicap_Galia.zip"
needs = pytest.mark.skipif(not GALIA.exists(), reason="run tools/fetch_test_models.py --relicap")


class FakeCache:
    def __init__(self, points):
        self._p = points

    def points(self):
        return self._p


def build(runse=True, drop=lambda m: False, bad=None):
    net = import_cgmes(GALIA)
    tr = T.to_pandapower(net, T.slack_of(lf.run_ac(net)))
    ms = [m for m in S.from_load_flow(net, tr, np.random.default_rng(5)) if not drop(m)]
    if bad:
        for m in ms:
            if m.tag == bad:
                m.value += 80.0
    now = datetime.now(timezone.utc)
    bindings, points = [], {}
    for i, m in enumerate(ms):
        tag = f"A.{i}"
        bindings.append(Binding(kind="analog", element_id=m.element_id, tag=tag, quantity=m.quantity, side=m.side,
                                sigma=m.sigma, max_age_seconds=60, approved=True))
        points[tag] = PointState(m.value, time_tag=now, time_tag_at_source=now)
    db = mongomock.MongoClient()["t"]
    db["protocolDriverInstances"].insert_one({"protocolDriver": "CIM-EMS", "protocolDriverInstanceNumber": 1,
                                              "enabled": True, "nodeNames": ["a"], "activeNodeName": "",
                                              "activeNodeKeepAliveTimeTag": None, "logLevel": 1})
    red = Redundancy(db, "a", 1)
    assert red.tick()
    pub = ResultPublisher(db, red, 9001, "EMS", result_max_age=180)
    conn = {"protocolConnectionNumber": 9001, "name": "EMS", "runStateEstimation": runse}
    cyc = StudyCycle(db, conn, {"modelId": "m", "revision": 1}, net, bindings, FakeCache(points), pub, red, "a")
    return cyc, db, ms


def value(db, tag):
    d = db["realtimeData"].find_one({"tag": tag})
    return d["sourceDataUpdate"] if d else None


@needs
def test_cycle_publishes_estimate_and_status():
    cyc, db, ms = build()
    run = cyc.run_once()
    assert run["status"] == "converged"
    est = run["summary"]["estimation"]
    assert est["status"] == "converged" and est["published"]
    assert value(db, "EMS;SYS;SE;STATUS")["valueAtSource"] == 0.0
    # a few clean values may be dropped as bad data: the transferred model is only accurate to about 1e-3 pu
    assert len(ms) - 4 <= value(db, "EMS;SYS;SE;MEAS")["valueAtSource"] <= len(ms)
    se_pts = db["realtimeData"].count_documents({"tag": {"$regex": "^EMS;SE;"}})
    assert se_pts > 20
    assert db["realtimeData"].count_documents({"tag": {"$regex": "^EMS;SE;.*;VPU$"}}) == 11


@needs
def test_cycle_reports_bad_data_in_run_record():
    _, _, ms = build()
    victim = next(m for m in ms if m.quantity == "P" and abs(m.value) > 20)
    cyc2, db2, ms2 = build(bad=victim.tag)
    run = cyc2.run_once()
    est = run["summary"]["estimation"]
    assert est["status"] == "converged"
    assert any(b["element"] == victim.element_id and b["quantity"] == "P" for b in est["badData"])
    assert value(db2, "EMS;SYS;SE;BADDATA")["valueAtSource"] >= 1


@needs
def test_cycle_marks_unobservable_and_keeps_load_flow():
    cyc, db, ms = build(drop=lambda m: m.quantity != "V")
    run = cyc.run_once()
    assert run["status"] == "converged"
    assert run["summary"]["estimation"]["status"] == "not-observable"
    assert value(db, "EMS;SYS;SE;STATUS")["valueAtSource"] == 1.0
    assert value(db, "EMS;SYS;LF;STATUS")["valueAtSource"] == 0.0


@needs
def test_estimation_off_by_default_publishes_nothing():
    cyc, db, _ = build(runse=False)
    run = cyc.run_once()
    assert "estimation" not in run["summary"]
    assert db["realtimeData"].count_documents({"tag": {"$regex": "^EMS;(SE|SYS;SE)"}}) == 0


def _ca_cycle(**conn):
    cyc, db, ms = build(runse=False)
    cyc.run_ca = True
    cyc.ca_list_id = conn.get("list", "AUTO_N1")
    cyc.ca_min_kv = 100
    return cyc, db


@needs
def test_cycle_runs_n1_publishes_ca_points_and_artifact():
    cyc, db = _ca_cycle()
    run = cyc.run_once()
    ca = run["summary"]["contingency"]
    assert ca["status"] == "done" and ca["published"] and ca["contingencies"] > 0
    assert value(db, "EMS;SYS;CA;STATUS")["valueAtSource"] == 0.0
    assert db["realtimeData"].count_documents({"tag": {"$regex": "^EMS;CA;.*;STATUS$"}}) == ca["contingencies"] + 0
    assert value(db, "EMS;CA;SYSTEM;COUNT")["valueAtSource"] == ca["contingencies"]
    import gridfs
    import json as js
    fs = gridfs.GridFS(db, collection="cimArtifacts")
    doc = js.loads(fs.find_one({"filename": f"ca_{run['runId']}.json"}).read())
    assert len(doc["outcomes"]) == ca["contingencies"]


@needs
def test_old_contingency_artifacts_are_pruned():
    import gridfs
    from datetime import timedelta
    cyc, db = _ca_cycle()
    fs = gridfs.GridFS(db, collection="cimArtifacts")
    old = fs.put(b"{}", filename="ca_0.json", kind="contingency-result", connectionNumber=cyc.number)
    keep = fs.put(b"{}", filename="model.zip", kind="cgmes-package")
    db["cimArtifacts.files"].update_many({}, {"$set": {"uploadDate": datetime.now(timezone.utc) - timedelta(days=8)}})
    cyc.run_once()
    assert not fs.exists(old) and fs.exists(keep)
    assert db["cimArtifacts.files"].count_documents({"kind": "contingency-result"}) == 1


@needs
def test_second_cycle_skips_but_keeps_points_alive():
    cyc, db = _ca_cycle()
    cyc.run_once()
    first = value(db, "EMS;CA;SYSTEM;COUNT")["timeTag"]
    run = cyc.run_once()
    assert run["summary"]["contingency"]["status"] == "skipped"
    assert value(db, "EMS;SYS;CA;SKIPPED")["valueAtSource"] == 1.0
    assert value(db, "EMS;CA;SYSTEM;COUNT")["timeTag"] > first


@needs
def test_missing_list_is_reported_not_fatal():
    cyc, db = _ca_cycle(list="NOPE")
    run = cyc.run_once()
    assert run["status"] == "converged"
    assert run["summary"]["contingency"]["status"] == "failed"
    assert value(db, "EMS;SYS;CA;STATUS")["valueAtSource"] == 2.0


@needs
def test_user_list_with_unknown_element_and_double_outage():
    cyc, db = _ca_cycle(list="L1")
    lines = list(cyc.net.get_lines().index)[:2]
    db["cimContingencyLists"].insert_one({"listId": "L1", "contingencies": [
        {"id": "ok", "elements": [lines[0]]}, {"id": "bad", "elements": ["NOPE"]},
        {"id": "dbl", "name": "two lines", "elements": lines}]})
    run = cyc.run_once()
    ca = run["summary"]["contingency"]
    assert ca["status"] == "done" and ca["contingencies"] == 3 and ca["unsupported"] == 1
    assert value(db, "EMS;CA;bad;STATUS")["valueAtSource"] == 4.0


@needs
def test_estimate_from_name_matched_bindings():
    """Synthetic SCADA tags -> analog proposals -> approval -> estimate. Voltages are bound to busbar sections."""
    from cim_ems import synthetic
    from cim_ems.bindings.proposal import propose_analogs

    cyc, db, _ = build()
    net = cyc.net
    tags, truth = synthetic.analog_tags(net)
    prop = propose_analogs(net, tags)
    rng = np.random.default_rng(7)
    bbs = {**net.get_bus_breaker_view_buses()["bus_id"].to_dict(), **net.get_busbar_sections()["bus_id"].to_dict()}
    buses = net.get_buses()
    branches = {**{i: r for i, r in net.get_lines().iterrows()},
                **{i: r for i, r in net.get_2_windings_transformers().iterrows()}}
    now = datetime.now(timezone.utc)
    points = {}
    used = []
    for b in prop.bindings:
        if b.quantity == "V" and not bbs[b.element_id]:
            continue          # busbar section out of service: the cycle drops such a measurement
        b.approved = True
        used.append(b)
        true = buses.loc[bbs[b.element_id], "v_mag"] if b.quantity == "V" else \
            branches[b.element_id][f"{b.quantity.lower()}{b.side}"]
        points[b.tag] = PointState(float(true + rng.normal(0, b.sigma)), time_tag=now, time_tag_at_source=now)
    assert sum(b.quantity == "V" for b in prop.bindings) > 0
    cyc2 = StudyCycle(db, {"protocolConnectionNumber": 9001, "name": "EMS", "runStateEstimation": True},
                      {"modelId": "m", "revision": 1}, net, used, FakeCache(points), cyc.publisher, cyc.red, "a")
    run = cyc2.run_once()
    est = run["summary"]["estimation"]
    assert est["status"] == "converged", est
    assert est["measurements"] >= len(used) - 4
