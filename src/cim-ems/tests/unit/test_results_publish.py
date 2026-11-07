import math
from datetime import datetime, timezone

import mongomock
import pypowsybl.loadflow as lf
import pypowsybl.network as pn
import pytest

from cim_ems.publish import ResultPublisher, point_document, tag_name
from cim_ems.redundancy import Redundancy
from cim_ems.results import ResultValue, extract_loadflow

T = datetime(2026, 1, 1, tzinfo=timezone.utc)


@pytest.fixture
def db():
    d = mongomock.MongoClient()["t"]
    d["protocolDriverInstances"].insert_one({"protocolDriver": "CIM-EMS", "protocolDriverInstanceNumber": 1,
                                             "enabled": True, "nodeNames": ["a"], "activeNodeName": "",
                                             "activeNodeKeepAliveTimeTag": None, "logLevel": 1})
    return d


@pytest.fixture
def publisher(db):
    red = Redundancy(db, "a", 1)
    assert red.tick()
    return ResultPublisher(db, red, 9001, "EMS_MAIN", result_max_age=180)


def rv(eid="L1", name="Line 1", side=1, q="P", value=10.0, kind="LF"):
    return ResultValue(kind, eid, name, side, q, value, "MW", "SubA", "VL400")


def test_extract_eurostag_has_buses_and_branches_with_units():
    net = pn.create_eurostag_tutorial_example1_network()
    lf.run_ac(net)
    vals = extract_loadflow(net)
    q = {(v.element_id, v.side, v.quantity): v for v in vals}
    assert sum(1 for v in vals if v.quantity == "V") == len(net.get_buses())
    assert all(v.value is not None for v in vals if v.quantity in ("V", "P"))
    bus = next(v for v in vals if v.quantity == "VPU")
    assert 0.9 < bus.value < 1.1 and bus.unit == "pu"
    assert any(v.quantity == "LOAD" for v in vals)
    line = net.get_lines().index[0]
    assert math.isclose(q[(line, 1, "P")].value, float(net.get_lines().loc[line, "p1"]))


def test_min_nominal_filter_and_switches_off():
    net = pn.create_eurostag_tutorial_example1_network()
    lf.run_ac(net)
    assert not extract_loadflow(net, {"minNominalKv": 1e6})
    only_v = extract_loadflow(net, {"branchFlows": False, "branchLoading": False})
    assert {v.quantity for v in only_v} == {"V", "VPU", "ANG"}


def test_open_line_gives_not_calculated_values():
    net = pn.create_four_substations_node_breaker_network()
    net.update_switches(id=["S3VL1_LINES3S4_BREAKER"], open=[True])
    lf.run_ac(net)
    vals = extract_loadflow(net)
    line = [v for v in vals if v.element_id == "LINE_S3S4"]
    assert line and all(v.value is None for v in line)
    assert all(v.value is not None for v in vals if v.element_id == "LINE_S2S3" and v.quantity == "P")


def test_tag_names_follow_documented_convention():
    assert tag_name("EMS", rv(), False) == "EMS;LF;Line 1;P;1"
    assert tag_name("EMS", rv(side=0, q="V", eid="B1", name=""), False) == "EMS;LF;B1;V"
    assert tag_name("EMS", rv(), True) == "EMS;LF;L1;P;1"      # ambiguous name falls back to the mRID


def test_point_document_is_a_supervised_point_with_expiry():
    d = point_document(9001000001.0, "T", rv(), 9001, 180, T)
    assert d["origin"] == "supervised" and d["protocolSourceCommonAddress"] == "LF"
    assert d["protocolSourceObjectAddress"] == "L1|1|P" and d["invalidDetectTimeout"] == 180.0
    assert d["unit"] == "MW" and d["kconv1"] == 1.0


def test_ensure_points_creates_once_with_keys_in_connection_range(publisher, db):
    vals = [rv(side=1), rv(side=2), rv(q="Q")]
    assert publisher.ensure_points(vals) == 3
    assert publisher.ensure_points(vals) == 0
    keys = sorted(d["_id"] for d in db.realtimeData.find())
    assert keys == [9001000001.0, 9001000002.0, 9001000003.0]
    # a second publisher instance (e.g. after restart) reuses and extends them
    other = ResultPublisher(db, publisher.red, 9001, "EMS_MAIN")
    assert other.ensure_points(vals + [rv(eid="L2")]) == 1
    assert db.realtimeData.count_documents({}) == 4


def test_duplicate_names_get_unique_tags(publisher, db):
    publisher.ensure_points([rv(eid="A", name="Same"), rv(eid="B", name="Same")])
    tags = {d["tag"] for d in db.realtimeData.find()}
    assert tags == {"EMS_MAIN;LF;A;P;1", "EMS_MAIN;LF;B;P;1"}


def test_publish_writes_source_data_update_and_marks_missing_invalid(publisher, db):
    n = publisher.publish(1, T, [rv(value=12.5), rv(eid="L2", value=None)])
    assert n == 2
    ok = db.realtimeData.find_one({"protocolSourceObjectAddress": "L1|1|P"})["sourceDataUpdate"]
    bad = db.realtimeData.find_one({"protocolSourceObjectAddress": "L2|1|P"})["sourceDataUpdate"]
    assert ok["valueAtSource"] == 12.5 and ok["invalidAtSource"] is False and ok["timeTagAtSourceOk"] is True
    assert ok["timeTagAtSource"].replace(tzinfo=None) == T.replace(tzinfo=None) and ok["originator"] == "CIM-EMS|9001|1"
    assert bad["invalidAtSource"] is True


def test_older_or_equal_run_is_refused(publisher, db):
    assert publisher.publish(5, T, [rv(value=1.0)]) == 1
    assert publisher.publish(5, T, [rv(value=2.0)]) == 0
    assert publisher.publish(4, T, [rv(value=3.0)]) == 0
    assert db.realtimeData.find_one({})["sourceDataUpdate"]["valueAtSource"] == 1.0
    assert publisher.publish(6, T, [rv(value=4.0)]) == 1


def test_runs_are_tracked_per_study_kind(publisher):
    assert publisher.publish(5, T, [rv(kind="LF")]) == 1
    assert publisher.publish(1, T, [rv(kind="SE")]) == 1


def test_inactive_node_never_publishes(publisher, db):
    db["protocolDriverInstances"].update_one({}, {"$set": {"activeNodeName": "b"}})
    assert publisher.publish(1, T, [rv()]) == 0
    assert db.realtimeData.count_documents({}) == 0 or all(d["sourceDataUpdate"] is None for d in db.realtimeData.find())
    assert publisher.invalidate("LF") == 0


def test_invalidate_marks_kind_only(publisher, db):
    publisher.publish(1, T, [rv(kind="LF", value=1.0), rv(kind="SE", value=2.0)])
    assert publisher.invalidate("LF") == 1
    lf_pt = db.realtimeData.find_one({"protocolSourceCommonAddress": "LF"})["sourceDataUpdate"]
    se_pt = db.realtimeData.find_one({"protocolSourceCommonAddress": "SE"})["sourceDataUpdate"]
    assert lf_pt["invalidAtSource"] is True and se_pt["invalidAtSource"] is False
