"""Runs the real driver process against a MongoDB replica set (CIM_EMS_TEST_MONGO_RS)."""
import json
import os
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path

import pymongo
import pypowsybl.network as pn
import pytest

from cim_ems import synthetic
from cim_ems.bindings import store as bstore
from cim_ems.bindings.proposal import propose_switches
from cim_ems.export import export_cgmes
from cim_ems.models import store as mstore
from cim_ems.models.importer import import_cgmes, validate

URI = os.environ.get("CIM_EMS_TEST_MONGO_RS")
pytestmark = pytest.mark.skipif(not URI, reason="needs a MongoDB replica set")
ROOT = Path(__file__).resolve().parents[2]


def wait_for(fn, timeout=40, step=0.25):
    end = time.time() + timeout
    while time.time() < end:
        v = fn()
        if v:
            return v
        time.sleep(step)
    return None


def seed(tmp_path):
    client = pymongo.MongoClient(URI)
    client.drop_database("cim_ems_e2e")
    db = client["cim_ems_e2e"]
    mongo_db_uri = URI.split("/?")[0].rstrip("/") + "/cim_ems_e2e?" + (URI.split("/?")[1] if "/?" in URI else "")

    # model: node-breaker network without HVDC, exported as CGMES 2.4.15
    net = pn.create_four_substations_node_breaker_network()
    pkg = tmp_path / "m.zip"
    export_cgmes(net, pkg)
    model_net = import_cgmes(pkg)
    doc = mstore.register_import(db, pkg, "M1", validate(model_net))
    mstore.activate(db, "M1", doc["revision"])

    # tags and bindings
    tags = synthetic.switch_tags(model_net)
    prop = propose_switches(model_net, tags)
    for b in prop.bindings:
        b.approved = True
    bstore.save_revision(db, "M1", prop.bindings)
    sw = model_net.get_switches()
    now = datetime.now(timezone.utc)
    db.realtimeData.insert_many([{"_id": t["_id"], "tag": t["tag"], "type": "digital", "origin": "supervised",
                                  "value": 0.0 if sw.loc[b.element_id, "open"] else 1.0, "invalid": False,
                                  "transient": False, "timeTag": now, "timeTagAtSource": now, "timeTagAtSourceOk": True}
                                 for t in tags for b in prop.bindings if b.tag == t["tag"]])

    db.protocolDriverInstances.insert_one({"protocolDriver": "CIM-EMS", "protocolDriverInstanceNumber": 1, "enabled": True,
                                           "logLevel": 1, "nodeNames": ["n1", "n2"], "activeNodeName": "",
                                           "activeNodeKeepAliveTimeTag": None, "keepProtocolRunningWhileInactive": False})
    db.protocolConnections.insert_one({"protocolDriver": "CIM-EMS", "protocolDriverInstanceNumber": 1,
                                       "protocolConnectionNumber": 9001.0, "name": "EMS_T", "description": "t",
                                       "enabled": True, "commandsEnabled": False, "autoCreateTags": True,
                                       "cimModelId": "M1", "cycleSeconds": 3600, "resultMaxAgeSeconds": 120,
                                       "publishFilter": {"minNominalKv": 0}})
    cfg = tmp_path / "c.json"
    cfg.write_text(json.dumps({"nodeName": "n1", "mongoConnectionString": mongo_db_uri, "mongoDatabaseName": "cim_ems_e2e"}))
    return client, db, prop, cfg


def test_driver_publishes_load_flow_and_follows_a_breaker(tmp_path):
    client, db, prop, cfg = seed(tmp_path)
    proc = subprocess.Popen([sys.executable, "-m", "cim_ems", "1", "1", str(cfg)], cwd=ROOT,
                            stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    try:
        run1 = wait_for(lambda: db.cimRuns.find_one({"status": "converged", "published": True}))
        assert run1, "no converged run: " + (proc.stdout.read() if proc.poll() is not None else "driver still running")
        assert run1["modelId"] == "M1" and run1["trigger"] == "start" and run1["bindingRevision"] == 1

        # result points exist as supervised points with sourceDataUpdate
        pts = list(db.realtimeData.find({"protocolSourceConnectionNumber": 9001.0}))
        assert len(pts) > 20 and all(p["origin"] == "supervised" for p in pts)
        line_p = db.realtimeData.find_one({"protocolSourceObjectAddress": "LINE_S3S4|1|P"})
        assert line_p["tag"] == "EMS_T;LF;LINE_S3S4;P;1" or line_p["tag"].startswith("EMS_T;LF;")
        assert wait_for(lambda: db.realtimeData.find_one({"_id": line_p["_id"]})["sourceDataUpdate"])
        v1 = db.realtimeData.find_one({"_id": line_p["_id"]})["sourceDataUpdate"]
        assert v1["invalidAtSource"] is False and abs(v1["valueAtSource"]) > 100
        status = db.realtimeData.find_one({"protocolSourceObjectAddress": "LF|0|CONVERGED"})["sourceDataUpdate"]
        assert status["valueAtSource"] == 1.0
        keep = db.protocolDriverInstances.find_one({})
        assert keep["activeNodeName"] == "n1"

        # open the breaker of line S3-S4 in the SCADA data: topology-triggered run, line becomes not calculated
        tag = next(b.tag for b in prop.bindings if b.element_id == "S3VL1_LINES3S4_BREAKER")
        db.realtimeData.update_one({"tag": tag}, {"$set": {"value": 0.0, "timeTag": datetime.now(timezone.utc),
                                                           "timeTagAtSource": datetime.now(timezone.utc)}})
        run2 = wait_for(lambda: db.cimRuns.find_one({"trigger": "topology", "status": "converged"}))
        assert run2, "no topology-triggered run"
        assert run2["topologyHash"] != run1["topologyHash"] and run2["runId"] > run1["runId"]
        v2 = wait_for(lambda: (lambda u: u if u and u["invalidAtSource"] else None)(
            db.realtimeData.find_one({"_id": line_p["_id"]})["sourceDataUpdate"]))
        assert v2, "open line must be published as invalid"
        assert v2["timeTag"] > v1["timeTag"]
    finally:
        proc.terminate()
        try:
            proc.wait(10)
        except subprocess.TimeoutExpired:
            proc.kill()
        client.drop_database("cim_ems_e2e")


def test_data_processor_turns_results_into_values_and_alarms(tmp_path):
    """The real cs_data_processor must consume the driver's sourceDataUpdate like any other source."""
    client, db, prop, cfg = seed(tmp_path)
    procs = [subprocess.Popen([sys.executable, "-m", "cim_ems", "1", "1", str(cfg)], cwd=ROOT,
                              stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)]
    try:
        assert wait_for(lambda: db.realtimeData.find_one({"protocolSourceObjectAddress": "LINE_S3S4|1|P"}))
        # set a limit below the flow (about 240 MW) on this point, then start the processor
        db.realtimeData.update_one({"protocolSourceObjectAddress": "LINE_S3S4|1|P"}, {"$set": {"hiLimit": 100.0}})
        procs.append(subprocess.Popen(["node", "cs_data_processor.js", "1", "1", str(cfg)], cwd=ROOT.parent / "cs_data_processor",
                                      stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True))
        time.sleep(2)
        # a new run after the processor is up: flip a breaker tag so the driver republishes
        tag = next(b.tag for b in prop.bindings if b.element_id == "S1VL1_LD1_BREAKER")
        db.realtimeData.update_one({"tag": tag}, {"$set": {"value": 0.0, "timeTag": datetime.now(timezone.utc),
                                                           "timeTagAtSource": datetime.now(timezone.utc)}})
        def processed():
            d = db.realtimeData.find_one({"protocolSourceObjectAddress": "LINE_S3S4|1|P"})
            return d if d and d.get("timeTag") and d.get("value") else None
        d = wait_for(processed, 40)
        assert d, "processor did not update the result point"
        assert d["invalid"] is False and abs(d["value"] - 240) < 30
        assert d["alarmRange"] == 1.0, f"hiLimit violation not detected: {d.get('alarmRange')}"
        assert d["sourceDataUpdate"]["originator"].startswith("CIM-EMS|9001|")
    finally:
        for p in procs:
            p.terminate()
        for p in procs:
            try:
                p.wait(10)
            except subprocess.TimeoutExpired:
                p.kill()
        client.drop_database("cim_ems_e2e")


def test_failover_leaves_exactly_one_publisher(tmp_path):
    client, db, prop, cfg1 = seed(tmp_path)
    cfg2 = tmp_path / "c2.json"
    c = json.loads(cfg1.read_text())
    cfg2.write_text(json.dumps({**c, "nodeName": "n2"}))
    db.protocolConnections.update_one({}, {"$set": {"cycleSeconds": 2}})
    env = {**os.environ, "CIM_EMS_KEEPALIVE_TIMEOUT": "4"}
    p1 = subprocess.Popen([sys.executable, "-m", "cim_ems", "1", "1", str(cfg1)], cwd=ROOT, env=env,
                          stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        assert wait_for(lambda: db.cimRuns.find_one({"published": True, "nodeName": "n1"}))
        p2 = subprocess.Popen([sys.executable, "-m", "cim_ems", "1", "1", str(cfg2)], cwd=ROOT, env=env,
                              stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        try:
            time.sleep(8)                                    # both alive: only n1 may publish
            assert {r["nodeName"] for r in db.cimRuns.find()} == {"n1"}
            assert db.protocolDriverInstances.find_one({})["activeNodeName"] == "n1"
            p1.kill()                                        # crash, no clean shutdown
            p1.wait()
            killed_at = datetime.now(timezone.utc)
            takeover = wait_for(lambda: db.cimRuns.find_one({"nodeName": "n2", "published": True}), 30)
            assert takeover, "standby node did not take over"
            assert db.protocolDriverInstances.find_one({})["activeNodeName"] == "n2"
            assert takeover["runId"] > max(r["runId"] for r in db.cimRuns.find({"nodeName": "n1"}))
            # runs of the dead node stop; no interleaving after the take-over
            ids = [(r["runId"], r["nodeName"]) for r in db.cimRuns.find().sort("runId", 1)]
            first_n2 = next(i for i, (_, n) in enumerate(ids) if n == "n2")
            assert all(n == "n2" for _, n in ids[first_n2:])
        finally:
            p2.kill()
    finally:
        p1.kill()
        client.drop_database("cim_ems_e2e")
