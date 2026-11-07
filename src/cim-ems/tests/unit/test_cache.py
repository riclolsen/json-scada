import os
import time
from datetime import datetime, timezone

import mongomock
import pytest

from cim_ems.cache import PointCache, to_state
from cim_ems.snapshot import PointState

T = datetime(2026, 1, 1, tzinfo=timezone.utc)


def test_to_state_maps_processed_fields():
    s = to_state({"tag": "A", "value": 1.0, "invalid": True, "transient": False, "substituted": True,
                  "frozen": True, "timeTag": T, "timeTagAtSource": T, "timeTagAtSourceOk": True})
    assert s.value == 1.0 and s.invalid and s.substituted and s.frozen and s.time_tag_at_source_ok
    assert to_state({"tag": "B", "value": None}).value is None
    assert to_state({"tag": "C", "value": True}).value == 1.0     # digital values may arrive as booleans


def test_naive_mongo_dates_become_utc():
    assert to_state({"tag": "A", "value": 1, "timeTag": datetime(2026, 1, 1)}).time_tag.tzinfo is not None


def test_full_load_and_trigger_on_change():
    db = mongomock.MongoClient()["t"]
    db.realtimeData.insert_many([{"_id": 1.0, "tag": "SW1", "value": 1.0, "invalid": False},
                                 {"_id": 2.0, "tag": "P1", "value": 5.0}, {"_id": 3.0, "tag": "OTHER", "value": 9.0}])
    c = PointCache(db, ["SW1", "P1", "MISSING"], trigger_tags={"SW1"})
    c.full_load()
    assert set(c.points()) == {"SW1", "P1"} and c.points()["P1"].value == 5.0
    c.trigger.clear()
    c.full_load()
    assert not c.trigger.is_set()                  # nothing changed
    db.realtimeData.update_one({"tag": "SW1"}, {"$set": {"value": 0.0}})
    c.full_load()
    assert c.trigger.is_set() and c.points()["SW1"].value == 0.0


def test_apply_triggers_only_for_trigger_tags_and_real_changes():
    c = PointCache(mongomock.MongoClient()["t"], ["SW1", "P1"], trigger_tags={"SW1"})
    c.apply({"tag": "SW1", "value": 1.0})
    assert c.trigger.is_set()
    c.trigger.clear()
    c.apply({"tag": "SW1", "value": 1.0, "timeTag": T})      # same value and flags: no trigger
    c.apply({"tag": "P1", "value": 3.0})
    assert not c.trigger.is_set()
    c.apply({"tag": "SW1", "value": 1.0, "invalid": True})
    assert c.trigger.is_set()


@pytest.mark.skipif(not os.environ.get("CIM_EMS_TEST_MONGO_RS"), reason="needs a MongoDB replica set")
def test_change_stream_follows_updates_and_recovers_by_resync():
    import pymongo

    client = pymongo.MongoClient(os.environ["CIM_EMS_TEST_MONGO_RS"])
    client.drop_database("cim_ems_cache")
    db = client["cim_ems_cache"]
    db.realtimeData.insert_many([{"_id": 1.0, "tag": "SW1", "value": 1.0}, {"_id": 2.0, "tag": "P1", "value": 5.0},
                                 {"_id": 3.0, "tag": "OTHER", "value": 0.0}])
    c = PointCache(db, ["SW1", "P1"], trigger_tags={"SW1"})
    c.start()
    try:
        assert c.ready.wait(10)
        c.trigger.clear()
        db.realtimeData.update_one({"tag": "SW1"}, {"$set": {"value": 0.0, "transient": True}})
        db.realtimeData.update_one({"tag": "OTHER"}, {"$set": {"value": 7.0}})
        deadline = time.time() + 10
        while time.time() < deadline and not (c.points()["SW1"].value == 0.0):
            time.sleep(0.05)
        assert c.points()["SW1"].value == 0.0 and c.points()["SW1"].transient
        assert c.trigger.is_set() and "OTHER" not in c.points()
        # an update lands while the stream is being re-established: the full load must pick it up
        loads = c.stats["full_loads"]
        c.request_resync()
        db.realtimeData.update_one({"tag": "P1"}, {"$set": {"value": 8.0}})
        deadline = time.time() + 10
        while time.time() < deadline and not (c.stats["full_loads"] > loads and c.points()["P1"].value == 8.0):
            time.sleep(0.05)
        assert c.points()["P1"].value == 8.0 and c.stats["full_loads"] > loads
    finally:
        c.stop()
        client.drop_database("cim_ems_cache")
